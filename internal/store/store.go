// Package store manages the SQLite database for bombvault.
package store

import (
	"database/sql"
	"fmt"
	"testing"

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
		db.Close()
		return nil, fmt.Errorf("store.Open WAL: %w", err)
	}
	if _, err := db.Exec(`PRAGMA foreign_keys=ON`); err != nil {
		db.Close()
		return nil, fmt.Errorf("store.Open foreign_keys: %w", err)
	}
	return db, nil
}

// OpenMem opens an in-memory SQLite database suitable for tests.
// The database is closed automatically when t finishes.
func OpenMem(t *testing.T) *sql.DB {
	t.Helper()
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("store.OpenMem: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}
