package store

// The two renumbering-recovery repairs, exercised against databases in the
// broken states they exist for.

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

// TestMigrateRecoversSettingsEverythingColumns reproduces the mirror of the
// collision v92 already covers: a database that recorded version 90 with the
// OTHER numbering's body skips this build's settings_everything for good.
// getSettings selects those columns by name, so without the repair the
// installation cannot read its settings at all.
func TestMigrateRecoversSettingsEverythingColumns(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close() //nolint:errcheck // test cleanup

	if err := Migrate(db); err != nil {
		t.Fatalf("first Migrate: %v", err)
	}
	// The damaged shape, exactly: the columns are gone, and version 90 is still
	// RECORDED — under the other numbering's name, which is what makes Migrate
	// skip this build's settings_everything forever. Deleting the 90 row instead
	// would just let v90 rerun and repair itself, which is not the situation the
	// field is in. Only 94 is unrecorded, the way a database that predates the
	// repair has it.
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
			t.Fatalf("settings.%s missing after recovery — getSettings selects it by name", col)
		}
	}
	// And the settings are actually readable again, which is the point.
	if _, err := New(db).GetSettings(); err != nil {
		t.Fatalf("GetSettings after recovery: %v", err)
	}
}

// TestMigrateRecoveryIsANoOpWhenNothingIsBroken: the repair must not disturb a
// healthy database, since it runs on every one of them exactly once.
func TestMigrateRecoveryIsANoOpWhenNothingIsBroken(t *testing.T) {
	db := openMigrated(t)
	if err := Migrate(db); err != nil {
		t.Fatalf("second Migrate must be idempotent: %v", err)
	}
	if _, err := New(db).GetSettings(); err != nil {
		t.Fatalf("GetSettings: %v", err)
	}
}

// TestPanicClosedRunIsNotCompleted: v93's backfill excluded only the reap
// marker, so a run closed by the PANIC path was recorded as completed — the
// exact signal LastEverythingPass reads to decide a whole-server pass already
// ran, which then skipped the next everyN interval.
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
	// A genuinely completed failure, which must be left alone.
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
		t.Fatal("a panic-closed run must not count as completed — it never reached its own conclusion")
	}
	if real != 1 {
		t.Fatal("a run that genuinely finished (and failed) must stay completed")
	}
}
