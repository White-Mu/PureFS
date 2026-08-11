package service

import (
	"context"
	"fmt"
	"log"

	"github.com/purefs/purefs/internal/model"
	"github.com/purefs/purefs/internal/repository"
	"github.com/purefs/purefs/internal/search"
	"github.com/purefs/purefs/internal/storage"
	"github.com/purefs/purefs/internal/task"
)

// IndexTask is the payload for an async indexing task.
type IndexTask struct {
	FileID int64
	UserID int64
}

// DeIndexTask is the payload for an async de-indexing task.
type DeIndexTask struct {
	Path string
}

// SearchService ties together the search engine, text extractor, file
// repository, and task queue for async indexing.
type SearchService struct {
	engine   search.SearchEngine
	fileRepo *repository.FileRepo
	store    storage.Storage
	queue    *task.TaskQueue
}

// NewSearchService creates a new SearchService. The task queue handles async
// index and de-index tasks.
func NewSearchService(
	engine search.SearchEngine,
	fileRepo *repository.FileRepo,
	store storage.Storage,
) *SearchService {
	svc := &SearchService{
		engine:   engine,
		fileRepo: fileRepo,
		store:    store,
	}

	svc.queue = task.NewTaskQueue(2, svc.handleTask)
	return svc
}

// StartQueue starts the background task workers.
func (s *SearchService) StartQueue() {
	s.queue.Start()
}

// StopQueue stops the background task workers and waits for them to drain.
func (s *SearchService) StopQueue() {
	s.queue.Stop()
}

// handleTask routes a task based on its type.
func (s *SearchService) handleTask(t task.Task) {
	switch t.Type {
	case "index":
		payload, ok := t.Payload.(IndexTask)
		if !ok {
			log.Printf("search: bad index task payload")
			return
		}
		if err := s.indexFile(payload.FileID, payload.UserID); err != nil {
			log.Printf("search: index file %d: %v", payload.FileID, err)
		}
	case "deindex":
		payload, ok := t.Payload.(DeIndexTask)
		if !ok {
			log.Printf("search: bad deindex task payload")
			return
		}
		if err := s.engine.Remove(payload.Path); err != nil {
			log.Printf("search: deindex %s: %v", payload.Path, err)
		}
	default:
		log.Printf("search: unknown task type %q", t.Type)
	}
}

// IndexFileAsync submits an async indexing task. Safe to call from request
// handlers without blocking.
func (s *SearchService) IndexFileAsync(fileID, userID int64) {
	s.queue.Submit(task.Task{
		Type:    "index",
		Payload: IndexTask{FileID: fileID, UserID: userID},
	})
}

// RemoveFromIndexAsync submits an async de-indexing task.
func (s *SearchService) RemoveFromIndexAsync(path string) {
	s.queue.Submit(task.Task{
		Type:    "deindex",
		Payload: DeIndexTask{Path: path},
	})
}

// indexFile reads a file from storage, extracts its text, and indexes it.
func (s *SearchService) indexFile(fileID, userID int64) error {
	f, err := s.fileRepo.GetByID(fileID)
	if err != nil {
		return fmt.Errorf("get file: %w", err)
	}

	// E2EE files are stored as ciphertext; skip content indexing entirely.
	if f.IsE2EE {
		return nil
	}

	reader, err := s.store.Open(f.Path)
	if err != nil {
		return fmt.Errorf("open file for indexing: %w", err)
	}
	defer reader.Close()

	content, err := search.ExtractText(f.Name, f.MimeType, reader)
	if err != nil {
		return fmt.Errorf("extract text: %w", err)
	}

	doc := search.SearchDocument{
		ID:        f.ID,
		Path:      f.Path,
		Name:      f.Name,
		Content:   content,
		Extension: extensionFromName(f.Name),
		FileSize:  f.Size,
		ModTime:   f.UpdatedAt,
		UserID:    f.UserID,
	}

	return s.engine.Index(doc)
}

// Search performs a full-text search and converts results to model.File
// objects by looking them up in the database.
func (s *SearchService) Search(ctx context.Context, query string, userID int64, opts search.SearchOptions) ([]*model.File, int, error) {
	opts.UserID = userID

	results, err := s.engine.Search(query, opts)
	if err != nil {
		return nil, 0, fmt.Errorf("search: %w", err)
	}

	files := make([]*model.File, 0, len(results))
	for _, r := range results {
		f, err := s.fileRepo.GetByID(r.ID)
		if err != nil {
			// File may have been deleted between index and search; skip.
			continue
		}
		files = append(files, f)
	}

	return files, len(files), nil
}

// RebuildIndex walks all files and re-indexes them synchronously. This can be
// expensive for large file collections.
func (s *SearchService) RebuildIndex(ctx context.Context) error {
	// Clear existing index by dropping and recreating the FTS5 table.
	// FTS5Engine manages the table, but we'll just re-index everything.
	const pageSize = 200
	offset := 0

	for {
		q := model.FileListQuery{
			Offset: offset,
			Limit:  pageSize,
			SortBy: "id",
		}
		files, count, err := s.fileRepo.List(q)
		if err != nil {
			return fmt.Errorf("list files: %w", err)
		}

		for _, f := range files {
			if f.FileType == model.FileTypeDir || f.IsE2EE {
				continue
			}
			if err := s.indexFile(f.ID, f.UserID); err != nil {
				log.Printf("search: rebuild index: file %d (%s): %v", f.ID, f.Path, err)
			}
		}

		offset += pageSize
		if int64(offset) >= count {
			break
		}
	}

	return nil
}

// extensionFromName returns the lowercase extension including the dot.
func extensionFromName(name string) string {
	for i := len(name) - 1; i >= 0; i-- {
		if name[i] == '.' {
			return toLower(name[i:])
		}
	}
	return ""
}

func toLower(s string) string {
	b := make([]byte, len(s))
	for i := range s {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 32
		}
		b[i] = c
	}
	return string(b)
}
