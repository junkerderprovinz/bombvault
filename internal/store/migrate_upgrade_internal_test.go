package store

import (
	"database/sql"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// Migration numbering + the upgrade path across the v89 collision.
//
// Two lines of work each added a migration numbered 89 at the same time: the
// feature branch's `schedule_job_runs` (the last-run record that makes an everyN
// cadence enforceable on the drill/tamper/digest passes, #166) and main's
// `settings_everything` + `runs_group_id` for the Backup Everything pass.
//
// Neither had shipped — the newest tag at the time, v7.12.1, topped out at v88 —
// so no released database carried either. The collision was resolved by keeping
// schedule_job_runs at 89 and shifting main's pair to 90/91, because Migrate
// keys off the version NUMBER (one schema_migrations row per version, skipped
// forever once present, see Migrate) and databases already running the branch
// build had 89 recorded. Had schedule_job_runs been renumbered instead, those
// databases would have skipped whichever migration inherited 89 and then failed
// on an ALTER TABLE that had already been applied under a different number.
//
// These tests pin both halves of that reasoning so it cannot silently rot.
// ---------------------------------------------------------------------------

// applyThrough runs the real migration bodies for every version <= maxVersion,
// recording them exactly as Migrate does. Versions 1..89 are byte-identical to
// the pre-merge feature branch's list, so applyThrough(db, 89) reproduces a
// genuine pre-merge database rather than an approximation of one.
func applyThrough(t *testing.T, db *sql.DB, maxVersion int) {
	t.Helper()
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
		version    INTEGER PRIMARY KEY,
		name       TEXT NOT NULL,
		applied_at INTEGER NOT NULL
	)`); err != nil {
		t.Fatalf("create schema_migrations: %v", err)
	}
	for _, m := range migrations {
		if m.version > maxVersion {
			continue
		}
		if _, err := db.Exec(m.sql); err != nil {
			t.Fatalf("seed v%d (%s): %v", m.version, m.name, err)
		}
		if _, err := db.Exec(
			`INSERT INTO schema_migrations (version, name, applied_at) VALUES (?, ?, ?)`,
			m.version, m.name, time.Now().Unix(),
		); err != nil {
			t.Fatalf("record v%d: %v", m.version, err)
		}
	}
}

func hasColumn(t *testing.T, db *sql.DB, table, column string) bool {
	t.Helper()
	rows, err := db.Query(`SELECT name FROM pragma_table_info(?)`, table)
	if err != nil {
		t.Fatalf("pragma_table_info(%s): %v", table, err)
	}
	defer rows.Close() //nolint:errcheck // test cleanup
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatalf("scan column: %v", err)
		}
		if name == column {
			return true
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows: %v", err)
	}
	return false
}

func appliedVersions(t *testing.T, db *sql.DB) map[int]string {
	t.Helper()
	rows, err := db.Query(`SELECT version, name FROM schema_migrations`)
	if err != nil {
		t.Fatalf("read schema_migrations: %v", err)
	}
	defer rows.Close() //nolint:errcheck // test cleanup
	got := map[int]string{}
	for rows.Next() {
		var v int
		var n string
		if err := rows.Scan(&v, &n); err != nil {
			t.Fatalf("scan: %v", err)
		}
		got[v] = n
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows: %v", err)
	}
	return got
}

// TestMigrationVersionsAreStrictlySequential is the guard the v89 collision would
// have tripped: every version appears exactly once, in ascending order, with no
// gaps. A duplicate is not a cosmetic problem — Migrate skips a version whose row
// already exists, so the SECOND migration sharing a number would never run on any
// database that had applied the first.
func TestMigrationVersionsAreStrictlySequential(t *testing.T) {
	seen := map[int]string{}
	for i, m := range migrations {
		if prev, dup := seen[m.version]; dup {
			t.Fatalf("duplicate migration version %d: %q and %q — Migrate would silently skip the second", m.version, prev, m.name)
		}
		seen[m.version] = m.name
		if want := i + 1; m.version != want {
			t.Fatalf("migration %d (%s) is out of sequence: want version %d, got %d", i, m.name, want, m.version)
		}
		if m.name == "" {
			t.Fatalf("migration v%d has no name", m.version)
		}
	}
	if len(migrations) == 0 {
		t.Fatal("no migrations defined")
	}
}

// TestUpgradeFromPreMergeBranchDatabase is the real upgrade proof: a database
// that already ran the feature branch (v89 = schedule_job_runs, main's two
// migrations never seen) must take main's work cleanly, keep its own data, and
// end up with the identical schema a fresh database gets.
func TestUpgradeFromPreMergeBranchDatabase(t *testing.T) {
	db := OpenMem(t)
	applyThrough(t, db, 89)

	// Precondition: exactly the pre-merge branch state.
	before := appliedVersions(t, db)
	if before[89] != "schedule_job_runs" {
		t.Fatalf("seeded v89 = %q, want schedule_job_runs", before[89])
	}
	if _, ok := before[90]; ok {
		t.Fatal("seed must not carry v90")
	}
	if hasColumn(t, db, "settings", "everything_schedule") {
		t.Fatal("seed must not already have main's everything_schedule column")
	}
	if hasColumn(t, db, "runs", "group_id") {
		t.Fatal("seed must not already have main's runs.group_id column")
	}

	// Real data the upgrade must not disturb: a recorded scheduled-job run, the
	// exact fact an everyN due-gate reads back.
	if _, err := db.Exec(`INSERT INTO schedule_job_runs (job, at) VALUES ('drills', 1700000000)`); err != nil {
		t.Fatalf("seed a schedule_job_runs row: %v", err)
	}

	// The upgrade itself.
	if err := Migrate(db); err != nil {
		t.Fatalf("upgrade from a pre-merge branch database failed: %v", err)
	}

	after := appliedVersions(t, db)
	if after[90] != "settings_everything" {
		t.Fatalf("v90 = %q, want settings_everything", after[90])
	}
	if after[91] != "runs_group_id" {
		t.Fatalf("v91 = %q, want runs_group_id", after[91])
	}
	if after[89] != "schedule_job_runs" {
		t.Fatalf("v89 changed identity to %q — an already-applied version must never be reused", after[89])
	}
	if !hasColumn(t, db, "settings", "everything_schedule") ||
		!hasColumn(t, db, "settings", "everything_pre_hook") ||
		!hasColumn(t, db, "settings", "everything_post_hook") {
		t.Fatal("Backup Everything settings columns missing after upgrade")
	}
	if !hasColumn(t, db, "runs", "group_id") {
		t.Fatal("runs.group_id missing after upgrade")
	}

	// The branch's own table survived untouched — not dropped, not re-created
	// empty by a renumbered migration re-running over it.
	var at int64
	if err := db.QueryRow(`SELECT at FROM schedule_job_runs WHERE job = 'drills'`).Scan(&at); err != nil {
		t.Fatalf("schedule_job_runs row lost in the upgrade: %v", err)
	}
	if at != 1700000000 {
		t.Fatalf("schedule_job_runs.at = %d, want 1700000000", at)
	}

	// Re-running is a no-op, as for any other database.
	if err := Migrate(db); err != nil {
		t.Fatalf("second migrate after upgrade: %v", err)
	}
}

// TestUpgradedSchemaMatchesFreshInstall proves the upgrade path and the
// fresh-install path converge: same tables, same columns. A renumbering that
// left an old database missing a column would show up here as a difference.
func TestUpgradedSchemaMatchesFreshInstall(t *testing.T) {
	upgraded := OpenMem(t)
	applyThrough(t, upgraded, 89)
	if err := Migrate(upgraded); err != nil {
		t.Fatalf("upgrade: %v", err)
	}

	fresh := OpenMem(t)
	if err := Migrate(fresh); err != nil {
		t.Fatalf("fresh migrate: %v", err)
	}

	if got, want := schemaFingerprint(t, upgraded), schemaFingerprint(t, fresh); got != want {
		t.Fatalf("upgraded schema differs from a fresh install:\nupgraded: %s\nfresh:    %s", got, want)
	}
}

// schemaFingerprint renders every table and its columns in a stable order.
func schemaFingerprint(t *testing.T, db *sql.DB) string {
	t.Helper()
	rows, err := db.Query(`SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY name`)
	if err != nil {
		t.Fatalf("list tables: %v", err)
	}
	var tables []string
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			t.Fatalf("scan table: %v", err)
		}
		tables = append(tables, n)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows: %v", err)
	}
	rows.Close() //nolint:errcheck // done reading

	out := ""
	for _, tbl := range tables {
		out += tbl + "("
		cols, err := db.Query(`SELECT name, type FROM pragma_table_info(?) ORDER BY name`, tbl)
		if err != nil {
			t.Fatalf("columns of %s: %v", tbl, err)
		}
		for cols.Next() {
			var n, ty string
			if err := cols.Scan(&n, &ty); err != nil {
				t.Fatalf("scan column: %v", err)
			}
			out += n + ":" + ty + ","
		}
		if err := cols.Err(); err != nil {
			t.Fatalf("cols: %v", err)
		}
		cols.Close() //nolint:errcheck // done reading
		out += ") "
	}
	return out
}
