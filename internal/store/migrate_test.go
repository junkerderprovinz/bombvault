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
	for _, tbl := range []string{"settings", "targets", "runs", "schema_migrations", "vms"} {
		var n int
		row := db.QueryRow("SELECT count(*) FROM sqlite_master WHERE type='table' AND name=?", tbl)
		if err := row.Scan(&n); err != nil || n != 1 {
			t.Fatalf("table %s missing", tbl)
		}
	}
}

func TestMigrateV4VMsTable(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	// Verify vms table exists with the expected columns.
	_, err := db.Exec(`INSERT INTO vms (id, name, method, include_in_schedule, definition, created_at)
		VALUES ('test-id', 'testvm', 'graceful', 0, '', 1234567890)`)
	if err != nil {
		t.Fatalf("vms table not created or wrong schema: %v", err)
	}
	var name string
	if err := db.QueryRow(`SELECT name FROM vms WHERE id = 'test-id'`).Scan(&name); err != nil {
		t.Fatalf("cannot read back: %v", err)
	}
	if name != "testvm" {
		t.Fatalf("name = %q, want testvm", name)
	}
}
