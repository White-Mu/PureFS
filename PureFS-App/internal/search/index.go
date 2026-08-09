package search

import "time"

// SearchDocument represents a document to be indexed for full-text search.
type SearchDocument struct {
	ID        int64
	Path      string
	Name      string
	Content   string
	Extension string
	FileSize  int64
	ModTime   time.Time
	UserID    int64
}

// SearchOptions configures a search query.
type SearchOptions struct {
	UserID int64
	Type   string // "all", "file", "document", "image"
	Offset int
	Limit  int
}

// SearchResult represents a single search hit.
type SearchResult struct {
	ID      int64   `json:"id"`
	Path    string  `json:"path"`
	Name    string  `json:"name"`
	Score   float64 `json:"score"`
	Snippet string  `json:"snippet"`
}

// SearchEngine is the interface for full-text search operations.
type SearchEngine interface {
	// Index adds or updates a document in the search index.
	Index(doc SearchDocument) error

	// Remove deletes a document from the search index by its path.
	Remove(path string) error

	// Search performs a full-text search and returns ranked results.
	Search(query string, opts SearchOptions) ([]SearchResult, error)

	// Close releases resources held by the engine.
	Close() error
}
