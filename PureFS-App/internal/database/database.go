package database

import (
	"database/sql"
	"embed"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var MigrationsFS embed.FS

func Open(driver, dsn string) (*sql.DB, error) {
	if driver == "sqlite" {
		dir := filepath.Dir(dsn)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("create data dir: %w", err)
		}
	}

	db, err := sql.Open(driver, dsn)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	if driver == "sqlite" {
		pragmas := []string{
			"PRAGMA journal_mode=WAL",
			"PRAGMA busy_timeout=5000",
			"PRAGMA synchronous=NORMAL",
			"PRAGMA foreign_keys=ON",
			"PRAGMA cache_size=-64000",
		}
		for _, p := range pragmas {
			if _, err := db.Exec(p); err != nil {
				return nil, fmt.Errorf("set pragma %s: %w", p, err)
			}
		}
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("ping database: %w", err)
	}

	return db, nil
}
