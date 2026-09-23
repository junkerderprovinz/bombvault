package store

import (
	"database/sql"
	"testing"
)

// ---------------------------------------------------------------------------
// The second numbering collision, and the one that actually reached a machine.
//
// main published `100 = passkeys` through :latest. This branch had independently
// taken `100 = target_exclude_caches` and stacked 101..105 on it. Migrate keys
// "applied" on the version NUMBER and skips a recorded version BEFORE it asks
// alreadySatisfied, so the two numberings cannot simply be merged:
//
//   - a :latest database has 100 = passkeys recorded. Renumbering the branch's
//     six to 101..106 is enough for it: those numbers are free there.
//   - a database born under a BUILD OF THIS BRANCH already has 100 recorded for a
//     completely different body, so main's passkeys migration is skipped on it
//     forever and every passkey route meets a missing table. Migration 107
//     repairs that.
//   - and a database born under a PARTIAL branch build - 100 recorded, the rest
//     not - is the one that needs the alreadySatisfied guards: the renumbered
//     101 would re-issue an ALTER for a column that is already there, and SQLite
//     has no ADD COLUMN IF NOT EXISTS, so the boot aborts.
//
// The 89/90 collision was solved the same way and pinned only from the :latest
// side. All three directions are pinned here.
// ---------------------------------------------------------------------------

// applied reports whether schema_migrations carries this version.
func applied(t *testing.T, db *sql.DB, version int) bool {
	t.Helper()
	var n int
	if err := db.QueryRow(`SELECT count(*) FROM schema_migrations WHERE version = ?`, version).Scan(&n); err != nil {
		t.Fatalf("read schema_migrations: %v", err)
	}
	return n > 0
}

// tableExists is the assertion this whole file is about.
func tableExists(t *testing.T, db *sql.DB, name string) bool {
	t.Helper()
	var got string
	err := db.QueryRow(`SELECT name FROM sqlite_master WHERE type = 'table' AND name = ?`, name).Scan(&got)
	switch {
	case err == sql.ErrNoRows:
		return false
	case err != nil:
		t.Fatalf("read sqlite_master: %v", err)
	}
	return got == name
}

// seedBranchNumbering rewinds a fully migrated database to the state a build of
// THIS BRANCH left behind, recording `through` as the highest old branch version.
// The columns the old numbering created below that point are kept; everything
// above it is removed, so the merged build meets exactly what such a machine has.
func seedBranchNumbering(t *testing.T, db *sql.DB, through int) {
	t.Helper()
	// The branch's old numbering, in order, with the column each body created.
	old := []struct {
		v            int
		name         string
		table, colum string
	}{
		{100, "target_exclude_caches", "targets", "exclude_caches"},
		{101, "file_set_selected_paths", "file_sets", "selected_paths"},
		{102, "file_set_repo", "file_sets", "repo"},
		{103, "target_repo", "targets", "repo"},
		{104, "vm_repo", "vms", "repo"},
		{105, "file_set_repo_is_an_id", "", ""}, // an UPDATE, no column of its own
	}
	if _, err := db.Exec(`DELETE FROM schema_migrations WHERE version >= 100`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`DROP TABLE IF EXISTS passkeys`); err != nil {
		t.Fatal(err)
	}
	// v120 (target_aliases) is a fresh migration too; dropping the table also
	// takes its v122 column (prev_definition) with it.
	if _, err := db.Exec(`DROP TABLE IF EXISTS target_aliases`); err != nil {
		t.Fatal(err)
	}
	// v121 (vm_uuid) is a fresh migration too; dropping its column lets Migrate
	// apply it again without "duplicate column name".
	if _, err := db.Exec(`ALTER TABLE vms DROP COLUMN uuid`); err != nil {
		t.Fatal(err)
	}
	for _, m := range old {
		if m.v <= through {
			if _, err := db.Exec(`INSERT INTO schema_migrations (version, name, applied_at) VALUES (?, ?, 0)`, m.v, m.name); err != nil {
				t.Fatalf("seed v%d: %v", m.v, err)
			}
			continue
		}
		// Above the cut: this body never ran on such a machine, so its column has
		// to go back.
		if m.table == "" {
			continue
		}
		if _, err := db.Exec(`ALTER TABLE ` + m.table + ` DROP COLUMN ` + m.colum); err != nil {
			t.Fatalf("unseed %s.%s: %v", m.table, m.colum, err)
		}
	}
}

// TestADatabaseBornUnderTheBranchNumberingStillGetsPasskeys is the direction that
// would otherwise fail silently: version 100 is recorded for the branch's own
// body, so main's passkeys migration never runs, and nothing says so.
func TestADatabaseBornUnderTheBranchNumberingStillGetsPasskeys(t *testing.T) {
	db := OpenMem(t)
	if err := Migrate(db); err != nil {
		t.Fatalf("first migrate: %v", err)
	}
	seedBranchNumbering(t, db, 105) // a complete branch build
	if tableExists(t, db, "passkeys") {
		t.Fatal("fixture is wrong: passkeys must be absent before the second migrate")
	}

	// This is the upgrade an operator gets by pulling the merged build.
	if err := Migrate(db); err != nil {
		t.Fatalf("upgrade migrate must not fail on a branch-numbered database: %v", err)
	}

	if !tableExists(t, db, "passkeys") {
		t.Error("the passkeys table is missing after the upgrade.\n" +
			"Version 100 is recorded on this database for a different body, and Migrate skips a\n" +
			"recorded version before it ever asks alreadySatisfied - so main's passkeys migration\n" +
			"never runs here. Migration 107 exists to repair exactly this, and it did not.")
	}
	if !applied(t, db, 107) {
		t.Error("migration 107 was not recorded")
	}
}

// TestAPartialBranchDatabaseSurvivesTheRenumbering is the direction the guards
// are for, and the one the first fixture cannot reach: with only 100 recorded,
// the renumbered 101 is NOT skipped by version and actually runs its body against
// a column that is already there.
func TestAPartialBranchDatabaseSurvivesTheRenumbering(t *testing.T) {
	db := OpenMem(t)
	if err := Migrate(db); err != nil {
		t.Fatalf("first migrate: %v", err)
	}
	seedBranchNumbering(t, db, 100) // an early branch build: only the first one ran
	if !hasColumn(t, db, "targets", "exclude_caches") {
		t.Fatal("fixture is wrong: targets.exclude_caches must already exist")
	}
	if hasColumn(t, db, "file_sets", "selected_paths") {
		t.Fatal("fixture is wrong: file_sets.selected_paths must NOT exist yet")
	}

	if err := Migrate(db); err != nil {
		t.Fatalf("the upgrade aborted on a partially migrated branch database: %v\n"+
			"Version 101 is free here, so the renumbered target_exclude_caches runs its ALTER\n"+
			"against a column that already exists. SQLite has no ADD COLUMN IF NOT EXISTS, so\n"+
			"without alreadySatisfied the boot dies and the instance never comes back up.", err)
	}

	// …and everything above the cut still landed.
	for _, c := range []struct{ table, column string }{
		{"file_sets", "selected_paths"}, {"file_sets", "repo"},
		{"targets", "repo"}, {"vms", "repo"},
	} {
		if !hasColumn(t, db, c.table, c.column) {
			t.Errorf("%s.%s is missing after the upgrade", c.table, c.column)
		}
	}
	if !tableExists(t, db, "passkeys") {
		t.Error("the passkeys table is missing after the upgrade")
	}
}

// TestAFreshDatabaseGetsPasskeysExactlyOnce is the ordinary direction, kept so a
// later simplification of migration 107 cannot break the common case.
func TestAFreshDatabaseGetsPasskeysExactlyOnce(t *testing.T) {
	db := OpenMem(t)
	if err := Migrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if !tableExists(t, db, "passkeys") {
		t.Fatal("a fresh database has no passkeys table")
	}
	if err := Migrate(db); err != nil {
		t.Fatalf("second migrate must be a no-op: %v", err)
	}
}

// TestThePasskeysRecoveryBodyIsIdempotentInPlainSQL pins the property that lets
// migration 107 carry no alreadySatisfied guard at all - the same property v92
// relies on.
func TestThePasskeysRecoveryBodyIsIdempotentInPlainSQL(t *testing.T) {
	db := OpenMem(t)
	if err := Migrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	body := bodyOf(t, "passkeys_renumbering_recovery")
	for i := range 3 {
		if _, err := db.Exec(body); err != nil {
			t.Fatalf("passkeys_renumbering_recovery is not idempotent (run %d): %v", i+2, err)
		}
	}
}
