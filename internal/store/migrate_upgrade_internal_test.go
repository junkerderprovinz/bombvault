package store

import (
	"database/sql"
	"fmt"
	"testing"
	"time"
)

// Two lines of work each added a migration numbered 89: schedule_job_runs on a
// feature branch, and settings_everything (with runs_group_id at 90) on main.
// The merge kept schedule_job_runs at 89 and moved main's pair to 90/91, but
// every push to main publishes :latest and the CA template installs :latest,
// so both numberings exist in real databases. No assignment of 89 to 91 is
// right for all of them. The contested migrations therefore detect what is
// already there (alreadySatisfied), and the idempotent recovery migration v92
// restores whatever a contested number swallowed.
//
// The tests below build each reachable database state from the real migration
// bodies, recorded under the numbers each build used, and check that Migrate
// ends on a fresh install's schema and leaves it unchanged on a second run.

// bodyOf returns the SQL of the named migration, so the seeds below use the
// real bodies rather than copies that can drift.
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

// applyAs runs the body of the named migration and records it under version,
// which need not be the number this build gives it: main shipped
// settings_everything as 89 and runs_group_id as 90.
func applyAs(t *testing.T, db *sql.DB, version int, name string) {
	t.Helper()
	if _, err := db.Exec(bodyOf(t, name)); err != nil {
		t.Fatalf("seed v%d (%s): %v", version, name, err)
	}
	record(t, db, version, name)
}

// applyThrough runs and records every migration up to maxVersion under this
// build's numbering, as Migrate does. Versions 1 to 88 are identical on both
// lines, so applyThrough(db, 88) gives a v7.12.1-era database and
// applyThrough(db, 89) a pre-merge feature-branch one.
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

// seedMainLatest builds the database of a user who installed :latest from the
// CA template: 89 = settings_everything, 90 = runs_group_id, and no
// schedule_job_runs table.
func seedMainLatest(t *testing.T, db *sql.DB) {
	t.Helper()
	applyThrough(t, db, 88)
	applyAs(t, db, 89, "settings_everything")
	applyAs(t, db, 90, "runs_group_id")
}

// seedMainLatestInterrupted builds the same database stopped between main's two
// migrations. They commit separately, so a power cut on the first boot after
// the update leaves 89 recorded and 90 not: the everything_* columns exist,
// runs.group_id does not.
func seedMainLatestInterrupted(t *testing.T, db *sql.DB) {
	t.Helper()
	applyThrough(t, db, 88)
	applyAs(t, db, 89, "settings_everything")
}

// seedMergedBranch builds a database that booted a post-merge build from before
// v92: 89 = schedule_job_runs, then main's pair at 90/91.
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

// TestMigrationVersionsAreUniqueAndAscending requires every version once, in
// ascending order. Migrate skips a version whose row already exists, so the
// second of two migrations sharing a number would never run on a database that
// applied the first. Gaps are allowed: 109 to 119 stay free because other
// builds record unrelated migrations as 109.
func TestMigrationVersionsAreUniqueAndAscending(t *testing.T) {
	seen := map[int]string{}
	last := 0
	for _, m := range migrations {
		if prev, dup := seen[m.version]; dup {
			t.Fatalf("duplicate migration version %d: %q and %q; Migrate would skip the second", m.version, prev, m.name)
		}
		seen[m.version] = m.name
		if m.version <= last {
			t.Fatalf("migration %s is out of order: version %d does not come after %d", m.name, m.version, last)
		}
		last = m.version
		if m.name == "" {
			t.Fatalf("migration v%d has no name", m.version)
		}
	}
	if len(migrations) == 0 {
		t.Fatal("no migrations defined")
	}
}

// TestContestedMigrationsCarryTheirGuard requires an alreadySatisfied guard on
// every migration whose body shipped under another version number. Such a
// migration runs again on databases that recorded it under the old one, and
// SQLite cannot make an ALTER idempotent.
func TestContestedMigrationsCarryTheirGuard(t *testing.T) {
	// name -> the version a shipped build recorded this body under
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
			t.Fatalf("v%d (%s) no longer differs from the number it shipped under; update this test's premise", m.version, m.name)
		}
		if m.alreadySatisfied == nil {
			t.Fatalf("v%d (%s) shipped under v%d as well and has no alreadySatisfied guard: it will re-run its ALTER on every :latest database and abort the boot",
				m.version, m.name, old)
		}
	}
}

// TestRenumberingRecoveryIsIdempotent reruns the recovery migration on a
// database that already has everything it creates, the common case. It has no
// guard, so it must be idempotent in plain SQL.
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
	// The table it recreates must match the one v89 creates, or an upgraded
	// database would differ from a fresh one.
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

// TestUpgradeConvergesFromEveryShippedDatabase migrates every database state a
// real user can hold and expects exactly the schema a fresh install has.
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
			// What most users hold. Without the guards Migrate fails here with
			// "duplicate column name: group_id" and the container does not boot.
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
				t.Fatalf("Migrate on %s failed, so this container would not boot: %v", tc.name, err)
			}

			// Named checks before the fingerprint, so a failure says which
			// feature is missing instead of printing two long strings.
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

			if err := Migrate(db); err != nil {
				t.Fatalf("second Migrate on %s: %v", tc.name, err)
			}
			if got := schemaFingerprint(t, db); got != want {
				t.Fatalf("second Migrate changed the schema of %s", tc.name)
			}
		})
	}
}

func TestUpgradeFromMainLatestKeepsUserData(t *testing.T) {
	db := OpenMem(t)
	seedMainLatest(t, db)

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
		t.Fatalf("runs.group_id = %q, want grp-1; the guard must skip the ALTER, not rebuild the table", group)
	}
	var sched string
	if err := db.QueryRow(`SELECT everything_schedule FROM settings WHERE id = 1`).Scan(&sched); err != nil {
		t.Fatalf("settings lost in the upgrade: %v", err)
	}
	if sched != "0 3 * * *" {
		t.Fatalf("everything_schedule = %q, want the configured cron; the guard must not reset the column to its default", sched)
	}

	after := appliedVersions(t, db)
	if after[89] != "settings_everything" {
		t.Fatalf("v89 changed identity to %q; an already-recorded version must not be rewritten", after[89])
	}
	if after[92] != "renumbering_recovery" {
		t.Fatalf("v92 = %q, want renumbering_recovery", after[92])
	}
}

// TestUpgradeFromPreMergeBranchDatabase starts from a database that ran the
// feature branch (89 = schedule_job_runs), which must take main's two
// migrations and keep its data.
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

	// A recorded scheduled-job run, which an everyN due-gate reads back.
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
		t.Fatalf("v89 changed identity to %q; an already-applied version must not be reused", after[89])
	}

	// The recovery migration must not drop the branch's table or recreate it
	// empty.
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

// TestRunsCompletedBackfill covers v93, which backfills runs.completed for rows
// written before the column existed, with the reap marker as the only signal. A
// run FinishRun concluded must come out completed, or every install gets one
// extra whole-server pass. A run ReapInterruptedRuns closed must not, or the
// abandoned pass holds the everyN gate shut for a whole interval.
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

	// The gate reads the backfill, not the reap stamp.
	ts, err := New(db).LastEverythingPass()
	if err != nil {
		t.Fatal(err)
	}
	if ts.IsZero() {
		t.Fatal("the concluded pass must still satisfy the everyN gate after the upgrade")
	}
}

// TestZFSMigrationsAreSatisfiedWhenAlreadyApplied builds a database that took
// the ZFS bodies under numbers this build does not use, which is what a
// renumbering at merge time leaves behind, and expects the guards to record the
// versions without re-running an ALTER SQLite cannot repeat.
func TestZFSMigrationsAreSatisfiedWhenAlreadyApplied(t *testing.T) {
	db := OpenMem(t)
	if err := Migrate(db); err != nil {
		t.Fatalf("first migrate: %v", err)
	}

	var zfsNames []string
	for _, m := range migrations {
		if m.version >= zfsMigrationBase && m.version < anomalyMigrationBase {
			zfsNames = append(zfsNames, m.name)
		}
	}
	if len(zfsNames) != 12 {
		t.Fatalf("found %d migrations from v%d to v%d, want the 12 of the ZFS domain", len(zfsNames), zfsMigrationBase, anomalyMigrationBase-1)
	}
	for i, name := range zfsNames {
		if _, err := db.Exec(`DELETE FROM schema_migrations WHERE name = ?`, name); err != nil {
			t.Fatalf("forget %s: %v", name, err)
		}
		record(t, db, 900+i, name)
	}
	if _, err := db.Exec(`UPDATE settings SET zfs_path = 'user/tank/zfs', zfs_schedule = 'daily 03:00' WHERE id = 1`); err != nil {
		t.Fatalf("configure the domain: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO zfs_datasets (id, dataset) VALUES ('z1', 'cache/appdata')`); err != nil {
		t.Fatalf("seed an item: %v", err)
	}

	if err := Migrate(db); err != nil {
		t.Fatalf("migrate over a database that already has the ZFS schema: %v", err)
	}

	applied := appliedVersions(t, db)
	for _, m := range migrations {
		if m.version < zfsMigrationBase || m.version >= anomalyMigrationBase {
			continue
		}
		if applied[m.version] != m.name {
			t.Fatalf("v%d = %q, want %q recorded so the next boot does not retry it", m.version, applied[m.version], m.name)
		}
	}

	var path, schedule string
	if err := db.QueryRow(`SELECT zfs_path, zfs_schedule FROM settings WHERE id = 1`).Scan(&path, &schedule); err != nil {
		t.Fatalf("settings lost in the upgrade: %v", err)
	}
	if path != "user/tank/zfs" || schedule != "daily 03:00" {
		t.Fatalf("zfs_path = %q, zfs_schedule = %q; a skipped body must not reset the columns", path, schedule)
	}
	var dataset string
	if err := db.QueryRow(`SELECT dataset FROM zfs_datasets WHERE id = 'z1'`).Scan(&dataset); err != nil {
		t.Fatalf("item lost in the upgrade: %v", err)
	}
	if dataset != "cache/appdata" {
		t.Fatalf("dataset = %q, want cache/appdata", dataset)
	}
}

// tableDDL returns a table's columns with the details a plain name:type
// comparison would miss (nullability, default, primary key).
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
// every named index with its DDL, in a stable order. The indexes matter: v91's
// guard skips its whole body, CREATE INDEX included, on a database that
// already has runs.group_id, and v92 has to put idx_runs_group back.
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

// TestZFSScheduleJoinsSchedulesThatWereInStep upgrades a database whose
// Containers, VMs, Flash and Folders schedules were kept in step. The ZFS
// schedule arrives as "off", and unless it joins them the page reads the
// shared schedule as switched off.
func TestZFSScheduleJoinsSchedulesThatWereInStep(t *testing.T) {
	for name, tc := range map[string]struct {
		containers, vms, want string
	}{
		"in step":          {containers: "daily 03:00", vms: "daily 03:00", want: "daily 03:00"},
		"one differs":      {containers: "daily 03:00", vms: "weekly 0 04:00", want: "off"},
		"all switched off": {containers: "off", vms: "off", want: "off"},
	} {
		t.Run(name, func(t *testing.T) {
			db := OpenMem(t)
			if err := Migrate(db); err != nil {
				t.Fatalf("first migrate: %v", err)
			}
			if _, err := db.Exec(`DELETE FROM schema_migrations WHERE name = 'settings_zfs_schedule_joins_sync'`); err != nil {
				t.Fatalf("forget the backfill: %v", err)
			}
			if _, err := db.Exec(`UPDATE settings SET containers_schedule = ?, vms_schedule = ?, flash_schedule = ?,
				files_schedule = ?, zfs_schedule = 'off' WHERE id = 1`, tc.containers, tc.vms, tc.containers, tc.containers); err != nil {
				t.Fatalf("set the schedules: %v", err)
			}
			if err := Migrate(db); err != nil {
				t.Fatalf("upgrade: %v", err)
			}
			var got string
			if err := db.QueryRow(`SELECT zfs_schedule FROM settings WHERE id = 1`).Scan(&got); err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Fatalf("zfs_schedule = %q, want %q", got, tc.want)
			}
		})
	}
}

// The published build of the anomaly branch numbered its migrations from 136,
// where this build keeps the last two ZFS steps. A database that ran it has
// 136 and 137 recorded under other names, so the upgrade skips both bodies.
func TestAnomalyBranchDatabaseGetsTheZFSStepsItsNumbersHid(t *testing.T) {
	db := OpenMem(t)
	if err := Migrate(db); err != nil {
		t.Fatalf("first migrate: %v", err)
	}
	if _, err := db.Exec(`DROP TABLE zfs_safety_snapshots`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`DELETE FROM schema_migrations WHERE version >= ?`, zfsMigrationBase+10); err != nil {
		t.Fatal(err)
	}
	record(t, db, zfsMigrationBase+10, "runs_source_metrics")
	record(t, db, zfsMigrationBase+11, "anomalies")
	record(t, db, anomalyMigrationBase, "settings_anomaly")
	if _, err := db.Exec(`UPDATE settings SET zfs_schedule = 'off', containers_schedule = 'daily 03:00',
		vms_schedule = 'daily 03:00', flash_schedule = 'daily 03:00', files_schedule = 'daily 03:00' WHERE id = 1`); err != nil {
		t.Fatal(err)
	}

	if err := Migrate(db); err != nil {
		t.Fatalf("migrate over the anomaly branch's numbering: %v", err)
	}
	if !hasTable(t, db, "zfs_safety_snapshots") {
		t.Fatal("zfs_safety_snapshots is missing, so every in-place ZFS restore fails")
	}
	var schedule string
	if err := db.QueryRow(`SELECT zfs_schedule FROM settings WHERE id = 1`).Scan(&schedule); err != nil {
		t.Fatal(err)
	}
	if schedule != "daily 03:00" {
		t.Fatalf("zfs_schedule = %q, want it joined to the shared schedule", schedule)
	}
}

// The published build of the MCP branch recorded its two migrations as 146
// and 147, and a database that ran it would skip whatever else took them.
func TestNoMigrationTakesTheNumbersTheMCPBranchRecorded(t *testing.T) {
	for _, m := range migrations {
		if m.version == 146 || m.version == 147 {
			t.Fatalf("v%d (%s) would be skipped on a database that ran the MCP branch build", m.version, m.name)
		}
	}
}
