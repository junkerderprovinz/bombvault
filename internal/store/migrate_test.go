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

func TestDBDumpColumnsMigrate(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO targets (id, container_name, appdata_paths, include_in_schedule, created_at)
		VALUES ('t1', 'pg', '[]', 0, 1)`); err != nil {
		t.Fatalf("insert target: %v", err)
	}
	var off int
	var engine string
	if err := db.QueryRow(`SELECT db_dump_off, db_dump_engine FROM targets WHERE id = 't1'`).Scan(&off, &engine); err != nil {
		t.Fatalf("read dump columns: %v", err)
	}
	if off != 0 || engine != "" {
		t.Fatalf("defaults are db_dump_off=%d db_dump_engine=%q, want 0 and empty", off, engine)
	}
	var enabled int
	if err := db.QueryRow(`SELECT db_dumps_enabled FROM settings WHERE id = 1`).Scan(&enabled); err != nil {
		t.Fatalf("read settings column: %v", err)
	}
	if enabled != 1 {
		t.Fatalf("db_dumps_enabled default = %d, want 1", enabled)
	}
}

// A database born under a different numbering already carries the columns, and
// SQLite has no idempotent ADD COLUMN, so the guards have to recognise them.
func TestDBDumpColumnsMigrateOverExistingColumns(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if _, err := db.Exec(`DELETE FROM schema_migrations WHERE name IN
		('targets_db_dump_off', 'settings_db_dumps_enabled', 'targets_db_dump_engine')`); err != nil {
		t.Fatalf("forget the dump migrations: %v", err)
	}
	if err := store.Migrate(db); err != nil {
		t.Fatalf("migrate over existing columns: %v", err)
	}
}

func TestMigrateCreatesVMsTable(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
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
