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

func TestMigrateAddsRunsStartedVia(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	// A run written by code that knows nothing of the origin columns.
	if _, err := db.Exec(`INSERT INTO runs (id, target_id, kind, status, started_at)
		VALUES ('r1', 't1', 'backup', 'running', 1700000000)`); err != nil {
		t.Fatalf("insert run: %v", err)
	}
	var via, viaKey string
	if err := db.QueryRow(`SELECT started_via, started_via_key FROM runs WHERE id = 'r1'`).Scan(&via, &viaKey); err != nil {
		t.Fatalf("read origin columns: %v", err)
	}
	if via != "" || viaKey != "" {
		t.Fatalf("origin = %q/%q, want empty", via, viaKey)
	}
	if _, err := db.Exec(`UPDATE runs SET started_via = NULL WHERE id = 'r1'`); err == nil {
		t.Fatal("started_via accepted NULL, want a NOT NULL column")
	}
	if _, err := db.Exec(`UPDATE runs SET started_via_key = NULL WHERE id = 'r1'`); err == nil {
		t.Fatal("started_via_key accepted NULL, want a NOT NULL column")
	}

	var n int
	if err := db.QueryRow(`SELECT count(*) FROM sqlite_master WHERE type = 'index' AND name = 'idx_runs_started_via'`).Scan(&n); err != nil {
		t.Fatalf("look for the index: %v", err)
	}
	if n != 1 {
		t.Fatalf("idx_runs_started_via count = %d, want 1", n)
	}

	if err := store.Migrate(db); err != nil {
		t.Fatalf("second migrate: %v", err)
	}
}

func TestMigrationStartedViaSatisfiedWhenColumnExists(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if _, err := db.Exec(`DELETE FROM schema_migrations WHERE name = 'runs_started_via'`); err != nil {
		t.Fatalf("forget the origin migration: %v", err)
	}
	if err := store.Migrate(db); err != nil {
		t.Fatalf("migrate over existing columns: %v", err)
	}
	var n int
	if err := db.QueryRow(`SELECT count(*) FROM schema_migrations WHERE name = 'runs_started_via'`).Scan(&n); err != nil {
		t.Fatalf("read the version row: %v", err)
	}
	if n != 1 {
		t.Fatalf("version rows for runs_started_via = %d, want 1", n)
	}
}

func TestMigrateCreatesMCPKeys(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	for _, name := range []string{"mcp_keys", "idx_mcp_keys_digest", "idx_mcp_keys_active_label"} {
		var n int
		if err := db.QueryRow(`SELECT count(*) FROM sqlite_master WHERE name = ?`, name).Scan(&n); err != nil {
			t.Fatalf("look for %s: %v", name, err)
		}
		if n != 1 {
			t.Fatalf("%s count = %d, want 1", name, n)
		}
	}
	if _, err := db.Exec(`DELETE FROM schema_migrations WHERE name = 'mcp_keys'`); err != nil {
		t.Fatalf("forget the mcp_keys migration: %v", err)
	}
	if err := store.Migrate(db); err != nil {
		t.Fatalf("migrate over the existing table: %v", err)
	}
}
