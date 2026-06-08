// Package store manages the SQLite database for bombvault.
package store

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite" // register "sqlite" driver
)

// Open opens (or creates) the SQLite database at path and configures it.
func Open(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("store.Open: %w", err)
	}
	// Single writer to avoid SQLITE_BUSY; WAL for concurrent reads.
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(`PRAGMA journal_mode=WAL`); err != nil {
		db.Close() //nolint:errcheck,gosec // cleanup on error path; original error takes priority
		return nil, fmt.Errorf("store.Open WAL: %w", err)
	}
	if _, err := db.Exec(`PRAGMA foreign_keys=ON`); err != nil {
		db.Close() //nolint:errcheck,gosec // cleanup on error path; original error takes priority
		return nil, fmt.Errorf("store.Open foreign_keys: %w", err)
	}
	return db, nil
}
