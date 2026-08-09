package search

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// FTS5Engine implements SearchEngine using SQLite FTS5 full-text search.
// This uses the standard database/sql connection shared with the main database.
type FTS5Engine struct {
	db *sql.DB
}

// NewFTS5Engine creates the FTS5 virtual table if it does not exist and returns
// a new engine. The caller is responsible for providing the same *sql.DB used by
// the rest of the application.
func NewFTS5Engine(db *sql.DB) (*FTS5Engine, error) {
	e := &FTS5Engine{db: db}
	if err := e.init(); err != nil {
		return nil, err
	}
	return e, nil
}

func (e *FTS5Engine) init() error {
	_, err := e.db.Exec(`
		CREATE VIRTUAL TABLE IF NOT EXISTS search_index USING fts5(
			doc_id UNINDEXED,
			path UNINDEXED,
			name,
			content,
			extension UNINDEXED,
			file_size UNINDEXED,
			mod_time UNINDEXED,
			user_id UNINDEXED,
			tokenize='porter unicode61'
		)
	`)
	return err
}

// Index adds or updates a document in the FTS5 index. If a document with the
// same path already exists it is replaced.
func (e *FTS5Engine) Index(doc SearchDocument) error {
	// Remove existing entry first (path column is a logical unique key here)
	_, err := e.db.Exec(`DELETE FROM search_index WHERE path = ?`, doc.Path)
	if err != nil {
		return fmt.Errorf("fts5: delete old entry: %w", err)
	}

	_, err = e.db.Exec(
		`INSERT INTO search_index (doc_id, path, name, content, extension, file_size, mod_time, user_id)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		doc.ID, doc.Path, doc.Name, doc.Content, doc.Extension, doc.FileSize,
		doc.ModTime.Format(time.RFC3339), doc.UserID,
	)
	return err
}

// Remove deletes a document from the index by its path.
func (e *FTS5Engine) Remove(path string) error {
	_, err := e.db.Exec(`DELETE FROM search_index WHERE path = ?`, path)
	return err
}

// Search performs a full-text search against the FTS5 index.
func (e *FTS5Engine) Search(query string, opts SearchOptions) ([]SearchResult, error) {
	if opts.Limit <= 0 || opts.Limit > 200 {
		opts.Limit = 50
	}

	query = sanitizeFTS5(query)
	if query == "" {
		return nil, nil
	}

	where := []string{"search_index MATCH ?"}
	args := []interface{}{query}

	if opts.UserID > 0 {
		where = append(where, "user_id = ?")
		args = append(args, fmt.Sprintf("%d", opts.UserID))
	}

	if opts.Type != "" && opts.Type != "all" {
		switch opts.Type {
		case "file":
			// "file" = everything that's not an image
			where = append(where, "extension NOT IN ('jpg','jpeg','png','gif','webp','svg','bmp','ico')")
		case "document":
			where = append(where, "extension IN ('txt','md','csv','json','pdf','doc','docx','xls','xlsx','ppt','pptx','html','css','js','xml')")
		case "image":
			where = append(where, "extension IN ('jpg','jpeg','png','gif','webp','svg','bmp','ico')")
		}
	}

	whereClause := strings.Join(where, " AND ")

	querySQL := fmt.Sprintf(
		`SELECT doc_id, path, name, snippet(search_index, 2, '<b>', '</b>', '...', 64) AS snippet, rank
		 FROM search_index WHERE %s ORDER BY rank LIMIT ? OFFSET ?`,
		whereClause,
	)
	args = append(args, opts.Limit, opts.Offset)

	rows, err := e.db.Query(querySQL, args...)
	if err != nil {
		return nil, fmt.Errorf("fts5 search: %w", err)
	}
	defer rows.Close()

	var results []SearchResult
	for rows.Next() {
		var (
			r       SearchResult
			snippet sql.NullString
			rank    float64
		)
		if err := rows.Scan(&r.ID, &r.Path, &r.Name, &snippet, &rank); err != nil {
			return nil, fmt.Errorf("fts5 scan: %w", err)
		}
		r.Score = -rank // FTS5 returns negative rank (lower is better); invert for intuitive higher-is-better
		if snippet.Valid {
			r.Snippet = snippet.String
		}
		results = append(results, r)
	}

	return results, nil
}

// Close is a no-op for FTS5Engine (the *sql.DB is managed externally).
func (e *FTS5Engine) Close() error {
	return nil
}

// sanitizeFTS5 escapes special FTS5 characters and prepares the query string.
func sanitizeFTS5(q string) string {
	// Remove special FTS5 syntax characters that users are unlikely to mean literally.
	q = strings.ReplaceAll(q, "*", "")
	q = strings.ReplaceAll(q, "^", "")
	q = strings.ReplaceAll(q, "\"", "")
	q = strings.TrimSpace(q)
	if q == "" {
		return ""
	}
	// Add prefix matching on the last term for better UX.
	terms := strings.Fields(q)
	if len(terms) > 0 {
		terms[len(terms)-1] += "*"
	}
	return strings.Join(terms, " ")
}
