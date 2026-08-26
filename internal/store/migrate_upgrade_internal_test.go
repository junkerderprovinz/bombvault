package store

import (
	"database/sql"
	"fmt"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// Migration numbering + the upgrade path across the contested 89/90 numbers.
//
// Two lines of work each added a migration numbered 89 at the same time: the
// feature branch's `schedule_job_runs` (the last-run record that makes an everyN
// cadence enforceable on the drill/tamper/digest passes, #166) and main's
// `settings_everything` (with `runs_group_id` right behind it at 90) for the
// Backup Everything pass. The merge kept schedule_job_runs at 89 and shifted
// main's pair to 90/91.
//
// The merge justified that with "neither had shipped — the newest tag, v7.12.1,
// tops out at v88 — so no released database carries either". THAT PREMISE WAS
// WRONG, and this file exists because of it: .github/workflows/build.yml
// publishes :latest on every push to main, and the Unraid CA template installs
// :latest, so main's numbering (89 = settings_everything, 90 = runs_group_id)
// has been in ordinary users' databases since main commit 9ab1401b. Both
// numberings are in the field. No assignment of 89/90/91 is correct for every
// database, so correctness cannot come from the numbers — it comes from the
// migrations at those numbers detecting what is already there (alreadySatisfied)
// plus a fresh-numbered, plain-SQL-idempotent recovery migration (v92) that
// re-establishes whatever a contested number swallowed.
//
// These tests construct every reachable database state FOR REAL — the actual
// migration bodies, recorded under the actual numbers each historical build
// used — and drive Migrate over each one. The bar is convergence: every state
// must end with the same schema a fresh install gets, and must survive a second
// Migrate unchanged.
// ---------------------------------------------------------------------------

// bodyOf returns the SQL of the migration with the given name, so the historical
// seeds below reproduce real builds rather than a hand-copied approximation that
// can drift away from them.
func bodyOf(t *testing.T, name string) string {
	t.Helper()
	for _, m := range migrations {
		if m.name == name {
			return m.sql
		}
	}
	t.Fatalf("no migration named %q", name)
	return ""
}

func ensureMigrationsTable(t *testing.T, db *sql.DB) {
	t.Helper()
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
		version    INTEGER PRIMARY KEY,
		name       TEXT NOT NULL,
		applied_at INTEGER NOT NULL
	)`); err != nil {
		t.Fatalf("create schema_migrations: %v", err)
	}
}

// record marks version/name applied exactly as Migrate does.
func record(t *testing.T, db *sql.DB, version int, name string) {
	t.Helper()
	if _, err := db.Exec(
		`INSERT INTO schema_migrations (version, name, applied_at) VALUES (?, ?, ?)`,
		version, name, time.Now().Unix(),
	); err != nil {
		t.Fatalf("record v%d (%s): %v", version, name, err)
	}
}

// applyAs runs the body of the named migration and records it under version —
// which is NOT necessarily the number this build gives it. That mismatch is the
// whole point: main shipped settings_everything as 89 and runs_group_id as 90.
func applyAs(t *testing.T, db *sql.DB, version int, name string) {
	t.Helper()
	if _, err := db.Exec(bodyOf(t, name)); err != nil {
		t.Fatalf("seed v%d (%s): %v", version, name, err)
	}
	record(t, db, version, name)
}

// applyThrough runs the real migration bodies for every version <= maxVersion
// under THIS build's numbering, recording them exactly as Migrate does. Versions
// 1..88 are byte-identical on main and on this branch (verified: the two files
// diff clean up to v88), so applyThrough(db, 88) reproduces a genuine v7.12.1-era
// database and applyThrough(db, 89) a genuine pre-merge feature-branch one.
func applyThrough(t *testing.T, db *sql.DB, maxVersion int) {
	t.Helper()
	ensureMigrationsTable(t, db)
	for _, m := range migrations {
		if m.version > maxVersion {
			continue
		}
		if _, err := db.Exec(m.sql); err != nil {
			t.Fatalf("seed v%d (%s): %v", m.version, m.name, err)
		}
		record(t, db, m.version, m.name)
	}
}

// seedMainLatest builds the database an ordinary user has: they installed or
// updated the container from the Unraid CA template, which pulls
// ghcr.io/junkerderprovinz/bombvault:latest, at any point after main commit
// 9ab1401b. Its schema_migrations holds 89 = settings_everything and
// 90 = runs_group_id; schedule_job_runs does not exist.
func seedMainLatest(t *testing.T, db *sql.DB) {
	t.Helper()
	applyThrough(t, db, 88)
	applyAs(t, db, 89, "settings_everything")
	applyAs(t, db, 90, "runs_group_id")
}

// seedMainLatestInterrupted builds the same database caught between main's two
// migrations. They commit in separate transactions, so a power cut or a killed
// container on the very first boot after that update leaves 89 recorded and 90
// not: the everything_* columns exist, runs.group_id does not.
func seedMainLatestInterrupted(t *testing.T, db *sql.DB) {
	t.Helper()
	applyThrough(t, db, 88)
	applyAs(t, db, 89, "settings_everything")
}

// seedMergedBranch builds a database that already booted a build cut from this
// branch AFTER the merge but BEFORE v92 existed — i.e. one that came up the
// pre-merge branch path (89 = schedule_job_runs) and then took main's pair at
// 90/91. It is the one state the pre-fix code did handle, and it must keep
// working.
func seedMergedBranch(t *testing.T, db *sql.DB) {
	t.Helper()
	applyThrough(t, db, 91)
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

func hasTable(t *testing.T, db *sql.DB, table string) bool {
	t.Helper()
	var n int
	if err := db.QueryRow(
		`SELECT count(*) FROM sqlite_master WHERE type='table' AND name = ?`, table,
	).Scan(&n); err != nil {
		t.Fatalf("look up table %s: %v", table, err)
	}
	return n == 1
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

// TestMigrationVersionsAreStrictlySequential is the guard the 89 collision would
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

// TestContestedMigrationsCarryTheirGuard pins the rule the numbering hazard
// forces, so a later edit cannot quietly drop it: a migration whose BODY already
// shipped under a different version number will run a second time on the
// databases that took it under the old number, so it must detect that and skip
// rather than blindly re-issue an ALTER that SQLite cannot make idempotent.
func TestContestedMigrationsCarryTheirGuard(t *testing.T) {
	// name -> the number some SHIPPED build already recorded this body under.
	shippedUnderAnotherNumber := map[string]int{
		"settings_everything": 89, // main's v89, published as :latest
		"runs_group_id":       90, // main's v90, published as :latest
	}
	for _, m := range migrations {
		old, contested := shippedUnderAnotherNumber[m.name]
		if !contested {
			continue
		}
		if m.version == old {
			t.Fatalf("v%d (%s) no longer differs from the number it shipped under — update this test's premise", m.version, m.name)
		}
		if m.alreadySatisfied == nil {
			t.Fatalf("v%d (%s) shipped under v%d as well and has no alreadySatisfied guard: it will re-run its ALTER on every :latest database and abort the boot",
				m.version, m.name, old)
		}
	}
}

// TestRenumberingRecoveryIsIdempotent pins the other half: the recovery
// migration must be safe to run on a database that already has everything it
// creates, because that is the common case (only :latest databases are missing
// anything). Plain-SQL idempotence is what lets it carry no guard at all.
func TestRenumberingRecoveryIsIdempotent(t *testing.T) {
	db := OpenMem(t)
	if err := Migrate(db); err != nil {
		t.Fatalf("fresh migrate: %v", err)
	}
	body := bodyOf(t, "renumbering_recovery")
	for i := range 3 {
		if _, err := db.Exec(body); err != nil {
			t.Fatalf("renumbering_recovery is not idempotent (run %d): %v", i+2, err)
		}
	}
	// And the table it recreates must be the same table v89 creates, or an
	// upgraded database would end up with a different schedule_job_runs than a
	// fresh one.
	fresh := OpenMem(t)
	ensureMigrationsTable(t, fresh)
	if _, err := fresh.Exec(bodyOf(t, "schedule_job_runs")); err != nil {
		t.Fatalf("apply schedule_job_runs body: %v", err)
	}
	recovered := OpenMem(t)
	if _, err := recovered.Exec(`CREATE TABLE runs (id TEXT PRIMARY KEY, group_id TEXT NOT NULL DEFAULT '')`); err != nil {
		t.Fatalf("stub runs: %v", err)
	}
	if _, err := recovered.Exec(body); err != nil {
		t.Fatalf("apply renumbering_recovery body: %v", err)
	}
	if got, want := tableDDL(t, recovered, "schedule_job_runs"), tableDDL(t, fresh, "schedule_job_runs"); got != want {
		t.Fatalf("recovery creates a different schedule_job_runs than v89:\nrecovery: %s\nv89:      %s", got, want)
	}
}

// TestUpgradeConvergesFromEveryShippedDatabase is the release gate for the
// contested numbering. Every database state a real user can be holding is built
// for real and migrated; each must succeed, and each must land on exactly the
// schema a fresh install has.
func TestUpgradeConvergesFromEveryShippedDatabase(t *testing.T) {
	fresh := OpenMem(t)
	if err := Migrate(fresh); err != nil {
		t.Fatalf("fresh install migrate: %v", err)
	}
	want := schemaFingerprint(t, fresh)

	cases := []struct {
		name string
		seed func(*testing.T, *sql.DB)
	}{
		{
			// The release-blocker: `latest` is republished on every push to main,
			// and the CA template installs `latest`, so this is what most users
			// actually hold. Before the fix, Migrate died here with
			// "duplicate column name: group_id" and the container did not boot.
			name: "main :latest (89=settings_everything, 90=runs_group_id)",
			seed: seedMainLatest,
		},
		{
			name: "main :latest interrupted between its two migrations (89 only)",
			seed: seedMainLatestInterrupted,
		},
		{
			name: "pre-merge feature branch (89=schedule_job_runs)",
			seed: func(t *testing.T, db *sql.DB) { applyThrough(t, db, 89) },
		},
		{
			name: "released v7.12.1 or older (tops out at v88)",
			seed: func(t *testing.T, db *sql.DB) { applyThrough(t, db, 88) },
		},
		{
			name: "already ran a post-merge branch build (89/90/91)",
			seed: seedMergedBranch,
		},
		{
			name: "fresh install",
			seed: func(*testing.T, *sql.DB) {},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			db := OpenMem(t)
			tc.seed(t, db)

			if err := Migrate(db); err != nil {
				t.Fatalf("Migrate on %s failed — this is a container that does not boot: %v", tc.name, err)
			}

			// Checked BEFORE the fingerprint on purpose: the fingerprint proves
			// convergence but reports it as a diff of two long strings, so the
			// named checks below get first refusal and a failure says which
			// feature is missing.
			if !hasTable(t, db, "schedule_job_runs") {
				t.Fatal("schedule_job_runs missing: LastScheduleJobRun errors and every everyN drills/tamper/digest pass is silently skipped (#166)")
			}
			for _, col := range []string{"everything_schedule", "everything_pre_hook", "everything_post_hook"} {
				if !hasColumn(t, db, "settings", col) {
					t.Fatalf("settings.%s missing: the Backup Everything pass cannot be configured", col)
				}
			}
			if !hasColumn(t, db, "runs", "group_id") {
				t.Fatal("runs.group_id missing: a Backup Everything pass cannot group its child runs")
			}

			if got := schemaFingerprint(t, db); got != want {
				t.Fatalf("schema did not converge on a fresh install's:\ngot:  %s\nwant: %s", got, want)
			}

			// Every version this build knows about must be recorded, or the next
			// boot retries a body that has already had its effect.
			applied := appliedVersions(t, db)
			for _, m := range migrations {
				if _, ok := applied[m.version]; !ok {
					t.Fatalf("v%d (%s) is not recorded after Migrate", m.version, m.name)
				}
			}

			// The tables the recovered schema promises must actually work.
			repo := New(db)
			at := time.Unix(1700000000, 0)
			if err := repo.RecordScheduleJobRun(ScheduleJobDrills, at); err != nil {
				t.Fatalf("RecordScheduleJobRun: %v", err)
			}
			back, err := repo.LastScheduleJobRun(ScheduleJobDrills)
			if err != nil {
				t.Fatalf("LastScheduleJobRun: %v", err)
			}
			if !back.Equal(at) {
				t.Fatalf("LastScheduleJobRun = %v, want %v", back, at)
			}

			// Re-running is a no-op on every one of these, as for any database.
			if err := Migrate(db); err != nil {
				t.Fatalf("second Migrate on %s: %v", tc.name, err)
			}
			if got := schemaFingerprint(t, db); got != want {
				t.Fatalf("second Migrate changed the schema of %s", tc.name)
			}
		})
	}
}

// TestUpgradeFromMainLatestKeepsUserData is the data half of the same upgrade:
// a :latest user's real rows must come through the recovery untouched.
func TestUpgradeFromMainLatestKeepsUserData(t *testing.T) {
	db := OpenMem(t)
	seedMainLatest(t, db)

	// Preconditions: exactly main's numbering, and the branch's table absent.
	before := appliedVersions(t, db)
	if before[89] != "settings_everything" || before[90] != "runs_group_id" {
		t.Fatalf("seed is not a main :latest database: 89=%q 90=%q", before[89], before[90])
	}
	if hasTable(t, db, "schedule_job_runs") {
		t.Fatal("seed must not already carry the branch's schedule_job_runs table")
	}
	if _, ok := before[91]; ok {
		t.Fatal("seed must not carry v91")
	}

	if _, err := db.Exec(`INSERT INTO targets (id, container_name, appdata_paths, created_at) VALUES ('t1', 'sonarr', '[]', 1700000000)`); err != nil {
		t.Fatalf("seed a target: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO runs (id, target_id, kind, status, started_at, group_id) VALUES ('r1', 't1', 'backup', 'success', 1700000000, 'grp-1')`); err != nil {
		t.Fatalf("seed a run: %v", err)
	}
	if _, err := db.Exec(`UPDATE settings SET everything_schedule = '0 3 * * *' WHERE id = 1`); err != nil {
		t.Fatalf("seed the everything schedule: %v", err)
	}

	if err := Migrate(db); err != nil {
		t.Fatalf("upgrade from a main :latest database failed: %v", err)
	}

	var group string
	if err := db.QueryRow(`SELECT group_id FROM runs WHERE id = 'r1'`).Scan(&group); err != nil {
		t.Fatalf("run lost in the upgrade: %v", err)
	}
	if group != "grp-1" {
		t.Fatalf("runs.group_id = %q, want grp-1 — the guard must skip the ALTER, never rebuild the table", group)
	}
	var sched string
	if err := db.QueryRow(`SELECT everything_schedule FROM settings WHERE id = 1`).Scan(&sched); err != nil {
		t.Fatalf("settings lost in the upgrade: %v", err)
	}
	if sched != "0 3 * * *" {
		t.Fatalf("everything_schedule = %q, want the configured cron — the guard must not reset the column to its default", sched)
	}

	after := appliedVersions(t, db)
	if after[89] != "settings_everything" {
		t.Fatalf("v89 changed identity to %q — an already-recorded version must never be rewritten", after[89])
	}
	if after[92] != "renumbering_recovery" {
		t.Fatalf("v92 = %q, want renumbering_recovery", after[92])
	}
}

// TestUpgradeFromPreMergeBranchDatabase is the other real upgrade proof: a
// database that already ran the feature branch (89 = schedule_job_runs, main's
// two migrations never seen) must take main's work cleanly and keep its data.
func TestUpgradeFromPreMergeBranchDatabase(t *testing.T) {
	db := OpenMem(t)
	applyThrough(t, db, 89)

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

	// The branch's own table survived untouched — not dropped, not re-created
	// empty by the recovery migration.
	var at int64
	if err := db.QueryRow(`SELECT at FROM schedule_job_runs WHERE job = 'drills'`).Scan(&at); err != nil {
		t.Fatalf("schedule_job_runs row lost in the upgrade: %v", err)
	}
	if at != 1700000000 {
		t.Fatalf("schedule_job_runs.at = %d, want 1700000000", at)
	}

	if err := Migrate(db); err != nil {
		t.Fatalf("second migrate after upgrade: %v", err)
	}
}

// tableDDL returns a table's columns with the details a plain name:type
// comparison would miss (nullability, default, primary key).
// TestRunsCompletedBackfill covers the one-off reconstruction v93 has to do for
// rows written before runs.completed existed: they carry no structural signal, so
// the backfill reads the reap marker instead. A run FinishRun concluded must come
// out completed (or every existing installation is granted one extra whole-server
// pass), and a run ReapInterruptedRuns closed out must not (or the abandoned pass
// keeps shutting the everyN gate for a whole interval, which is the defect v93
// exists to close).
func TestRunsCompletedBackfill(t *testing.T) {
	db := OpenMem(t)
	applyThrough(t, db, 92)

	if hasColumn(t, db, "runs", "completed") {
		t.Fatal("seed must predate runs.completed")
	}
	if _, err := db.Exec(`
		INSERT INTO runs (id, target_id, kind, status, started_at, finished_at, error) VALUES
		  ('concluded', 'everything', 'backup', 'failed',  1700000000, 1700003600, 'flash: not mounted'),
		  ('clean',     'everything', 'backup', 'success', 1700000000, 1700003600, NULL),
		  ('reaped',    'everything', 'backup', 'failed',  1700000000, 1700003600, 'interrupted (BombVault restarted mid-run)'),
		  ('running',   'everything', 'backup', 'running', 1700000000, NULL,       NULL)`,
	); err != nil {
		t.Fatalf("seed runs: %v", err)
	}

	if err := Migrate(db); err != nil {
		t.Fatalf("upgrade: %v", err)
	}

	want := map[string]int{"concluded": 1, "clean": 1, "reaped": 0, "running": 0}
	for id, w := range want {
		var got int
		if err := db.QueryRow(`SELECT completed FROM runs WHERE id = ?`, id).Scan(&got); err != nil {
			t.Fatalf("read %s: %v", id, err)
		}
		if got != w {
			t.Fatalf("run %q: completed = %d, want %d", id, got, w)
		}
	}

	// And the gate reads the backfill, not the reap stamp.
	ts, err := New(db).LastEverythingPass()
	if err != nil {
		t.Fatal(err)
	}
	if ts.IsZero() {
		t.Fatal("the concluded pass must still satisfy the everyN gate after the upgrade")
	}
}

func tableDDL(t *testing.T, db *sql.DB, table string) string {
	t.Helper()
	rows, err := db.Query(`SELECT name, type, "notnull", ifnull(dflt_value, ''), pk FROM pragma_table_info(?) ORDER BY name`, table)
	if err != nil {
		t.Fatalf("columns of %s: %v", table, err)
	}
	defer rows.Close() //nolint:errcheck // test cleanup
	out := ""
	for rows.Next() {
		var name, ty, dflt string
		var notNull, pk int
		if err := rows.Scan(&name, &ty, &notNull, &dflt, &pk); err != nil {
			t.Fatalf("scan column: %v", err)
		}
		out += fmt.Sprintf("%s:%s:nn=%d:def=%s:pk=%d,", name, ty, notNull, dflt, pk)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("cols: %v", err)
	}
	return out
}

// schemaFingerprint renders every table with its full column definitions and
// every named index with its DDL, in a stable order. Indexes are in deliberately:
// v91's guard skips the whole body on a database that already has runs.group_id,
// including its CREATE INDEX, and this is what proves v92 puts idx_runs_group
// back.
func schemaFingerprint(t *testing.T, db *sql.DB) string {
	t.Helper()
	names := func(kind string) []string {
		rows, err := db.Query(
			`SELECT name FROM sqlite_master WHERE type = ? AND name NOT LIKE 'sqlite_%' ORDER BY name`, kind,
		)
		if err != nil {
			t.Fatalf("list %ss: %v", kind, err)
		}
		defer rows.Close() //nolint:errcheck // test cleanup
		var out []string
		for rows.Next() {
			var n string
			if err := rows.Scan(&n); err != nil {
				t.Fatalf("scan %s: %v", kind, err)
			}
			out = append(out, n)
		}
		if err := rows.Err(); err != nil {
			t.Fatalf("rows: %v", err)
		}
		return out
	}

	out := ""
	for _, tbl := range names("table") {
		out += tbl + "(" + tableDDL(t, db, tbl) + ") "
	}
	for _, idx := range names("index") {
		var ddl sql.NullString
		if err := db.QueryRow(`SELECT sql FROM sqlite_master WHERE type='index' AND name = ?`, idx).Scan(&ddl); err != nil {
			t.Fatalf("index ddl %s: %v", idx, err)
		}
		out += "INDEX " + idx + "[" + ddl.String + "] "
	}
	return out
}
