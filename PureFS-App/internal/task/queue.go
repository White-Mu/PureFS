package task

import (
	"log"
	"sync"
)

// Task represents a unit of asynchronous work.
type Task struct {
	Type    string
	Payload interface{}
}

// TaskQueue provides a simple goroutine+channel pattern for async task
// processing. Submitting a task after the queue is stopped is a no-op.
type TaskQueue struct {
	jobs    chan Task
	handler func(Task)
	workers int
	wg      sync.WaitGroup
	mu      sync.Mutex
	started bool
	stopped bool
}

// NewTaskQueue creates a new task queue. workers controls how many goroutines
// process jobs concurrently. handler is called for each task.
func NewTaskQueue(workers int, handler func(Task)) *TaskQueue {
	if workers < 1 {
		workers = 1
	}
	return &TaskQueue{
		jobs:    make(chan Task, 1024),
		handler: handler,
		workers: workers,
	}
}

// Submit enqueues a task. If the queue has been stopped the task is silently
// dropped.
func (q *TaskQueue) Submit(t Task) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.stopped {
		log.Printf("task: dropping %q task because queue is stopped", t.Type)
		return
	}
	// Non-blocking send: if the buffer is full, skip the task rather than
	// blocking the upload handler.
	select {
	case q.jobs <- t:
	default:
		log.Printf("task: dropping %q task because queue is full", t.Type)
	}
}

// Start launches worker goroutines. Must be called before Submit.
func (q *TaskQueue) Start() {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.started {
		return
	}
	q.started = true

	for i := 0; i < q.workers; i++ {
		q.wg.Add(1)
		go func(workerID int) {
			defer q.wg.Done()
			for task := range q.jobs {
				q.handler(task)
			}
		}(i)
	}
	log.Printf("task: started %d workers", q.workers)
}

// Stop closes the job channel and waits for all workers to finish.
func (q *TaskQueue) Stop() {
	q.mu.Lock()
	if q.stopped {
		q.mu.Unlock()
		return
	}
	q.stopped = true
	q.mu.Unlock()

	close(q.jobs)
	q.wg.Wait()
	log.Println("task: all workers stopped")
}
