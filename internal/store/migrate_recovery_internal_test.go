package store

// The migrations that repair databases broken by the migration renumbering,
// run against databases in those broken states.

import (
	"database/sql"
	"testing"
)

func openMigrated(t *testing.T) *sql.DB {
	t.Helper()
	db, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := Migrate(db); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	return db
}

// TestMigrateRecoversSettingsEverythingColumns covers a database that recorded
// version 90 under the other numbering and so skips this build's
// settings_everything migration for good. getSettings selects those columns by
// name, so without the repair the settings cannot be read at all.
func TestMigrateRecoversSettingsEverythingColumns(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close() //nolint:errcheck // test cleanup

	if err := Migrate(db); err != nil {
		t.Fatalf("first Migrate: %v", err)
	}
	// The columns are gone while version 90 stays recorded under the other
	// numbering's name. Deleting the 90 row would let v90 rerun and repair
	// itself, which is not what happens on real installs. Only 94 is
	// unrecorded, as in a database that predates the repair.
	for _, col := range []string{"everything_schedule", "everything_pre_hook", "everything_post_hook"} {
		if _, err := db.Exec(`ALTER TABLE settings DROP COLUMN ` + col); err != nil {
			t.Fatalf("drop %s: %v", col, err)
		}
	}
	if _, err := db.Exec(`UPDATE schema_migrations SET name = 'runs_group_id' WHERE version = 90`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`DELETE FROM schema_migrations WHERE version = 94`); err != nil {
		t.Fatal(err)
	}

	if err := Migrate(db); err != nil {
		t.Fatalf("recovery Migrate: %v", err)
	}
	for _, col := range []string{"everything_schedule", "everything_pre_hook", "everything_post_hook"} {
		var n int
		if err := db.QueryRow(`SELECT count(*) FROM pragma_table_info('settings') WHERE name = ?`, col).Scan(&n); err != nil {
			t.Fatal(err)
		}
		if n == 0 {
			t.Fatalf("settings.%s missing after recovery; getSettings selects it by name", col)
		}
	}
	if _, err := New(db).GetSettings(); err != nil {
		t.Fatalf("GetSettings after recovery: %v", err)
	}
}

// TestMigrateRecoveryIsANoOpWhenNothingIsBroken checks that the repair, which
// runs once on every healthy database, leaves it intact.
func TestMigrateRecoveryIsANoOpWhenNothingIsBroken(t *testing.T) {
	db := openMigrated(t)
	if err := Migrate(db); err != nil {
		t.Fatalf("second Migrate must be idempotent: %v", err)
	}
	if _, err := New(db).GetSettings(); err != nil {
		t.Fatalf("GetSettings: %v", err)
	}
}

// TestPanicClosedRunIsNotCompleted covers v93's backfill, which excluded only
// the reap marker and so marked a run closed by the panic path as completed.
// LastEverythingPass reads that flag to decide whether a whole-server pass ran
// and would skip the next everyN interval.
func TestPanicClosedRunIsNotCompleted(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close() //nolint:errcheck // test cleanup
	if err := Migrate(db); err != nil {
		t.Fatal(err)
	}

	// A row in the shape v93's backfill produced for a panic-closed run.
	if _, err := db.Exec(`INSERT INTO runs (id, target_id, kind, status, started_at, finished_at, error, completed)
		VALUES ('r1', 'everything', 'backup', 'failed', 1, 2, 'internal error (recovered panic): boom', 1)`); err != nil {
		t.Fatal(err)
	}
	// A completed failure, which must stay completed.
	if _, err := db.Exec(`INSERT INTO runs (id, target_id, kind, status, started_at, finished_at, error, completed)
		VALUES ('r2', 'everything', 'backup', 'failed', 1, 2, 'containers: 1/2 ok (plex: boom)', 1)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`DELETE FROM schema_migrations WHERE version = 95`); err != nil {
		t.Fatal(err)
	}
	if err := Migrate(db); err != nil {
		t.Fatalf("correction Migrate: %v", err)
	}

	var panicked, real int
	if err := db.QueryRow(`SELECT completed FROM runs WHERE id = 'r1'`).Scan(&panicked); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT completed FROM runs WHERE id = 'r2'`).Scan(&real); err != nil {
		t.Fatal(err)
	}
	if panicked != 0 {
		t.Fatal("a panic-closed run never finished and must not count as completed")
	}
	if real != 1 {
		t.Fatal("a run that genuinely finished (and failed) must stay completed")
	}
}
