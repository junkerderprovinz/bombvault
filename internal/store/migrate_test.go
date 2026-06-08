package store_test

import (
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

func TestMigrateIdempotent(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	if err := store.Migrate(db); err != nil {
		t.Fatalf("second migrate: %v", err)
	}
	for _, tbl := range []string{"settings", "targets", "runs", "schema_migrations"} {
		var n int
		row := db.QueryRow("SELECT count(*) FROM sqlite_master WHERE type='table' AND name=?", tbl)
		if err := row.Scan(&n); err != nil || n != 1 {
			t.Fatalf("table %s missing", tbl)
		}
	}
}
