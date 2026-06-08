package store

import (
	"database/sql"
	"testing"
)

// OpenMem opens an in-memory SQLite database suitable for tests.
// The database is closed automatically when t finishes.
func OpenMem(t *testing.T) *sql.DB {
	t.Helper()
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("store.OpenMem: %v", err)
	}
	t.Cleanup(func() { db.Close() }) //nolint:errcheck,gosec // test cleanup; error not actionable
	return db
}
