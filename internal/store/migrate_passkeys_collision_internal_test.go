package store

import (
	"database/sql"
	"testing"
)

// Two builds numbered migrations differently: :latest shipped 100 = passkeys,
// while another build used 100 = target_exclude_caches with 101..105 on top.
// Migrate skips a recorded version before it asks alreadySatisfied, so:
//
//   - a :latest database only needs those six renumbered to 101..106.
//   - a database from the other build has 100 recorded for a different body,
//     so the passkeys migration never runs there. Migration 107 repairs that.
//   - a database from a partial run of the other build (only 100 recorded)
//     needs the alreadySatisfied guards: the renumbered 101 would repeat an
//     ALTER for an existing column, and SQLite has no ADD COLUMN IF NOT
//     EXISTS, so the boot would abort.

func applied(t *testing.T, db *sql.DB, version int) bool {
	t.Helper()
	var n int
	if err := db.QueryRow(`SELECT count(*) FROM schema_migrations WHERE version = ?`, version).Scan(&n); err != nil {
		t.Fatalf("read schema_migrations: %v", err)
	}
	return n > 0
}

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

// seedBranchNumbering rewinds a fully migrated database to what a build with
// the other numbering left behind, with `through` as its highest version.
// Columns created up to that point stay; everything above it is removed.
func seedBranchNumbering(t *testing.T, db *sql.DB, through int) {
	t.Helper()
	// The other numbering, in order, with the column each body created.
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

// Without migration 107 this case fails silently: version 100 is recorded for
// another body, so the passkeys migration never runs.
func TestADatabaseBornUnderTheBranchNumberingStillGetsPasskeys(t *testing.T) {
	db := OpenMem(t)
	if err := Migrate(db); err != nil {
		t.Fatalf("first migrate: %v", err)
	}
	seedBranchNumbering(t, db, 105)
	if tableExists(t, db, "passkeys") {
		t.Fatal("fixture is wrong: passkeys must be absent before the second migrate")
	}

	if err := Migrate(db); err != nil {
		t.Fatalf("upgrade migrate must not fail on a branch-numbered database: %v", err)
	}

	if !tableExists(t, db, "passkeys") {
		t.Error("the passkeys table is missing after the upgrade.\n" +
			"Version 100 is recorded on this database for a different body, and Migrate skips a\n" +
			"recorded version before it ever asks alreadySatisfied, so the passkeys migration\n" +
			"never runs here. Migration 107 exists to repair exactly this, and it did not.")
	}
	if !applied(t, db, 107) {
		t.Error("migration 107 was not recorded")
	}
}

// With only 100 recorded, the renumbered 101 is not skipped by version and
// runs its body against a column that already exists.
func TestAPartialBranchDatabaseSurvivesTheRenumbering(t *testing.T) {
	db := OpenMem(t)
	if err := Migrate(db); err != nil {
		t.Fatalf("first migrate: %v", err)
	}
	seedBranchNumbering(t, db, 100)
	if !hasColumn(t, db, "targets", "exclude_caches") {
		t.Fatal("fixture is wrong: targets.exclude_caches must already exist")
	}
	if hasColumn(t, db, "file_sets", "selected_paths") {
		t.Fatal("fixture is wrong: file_sets.selected_paths must not exist yet")
	}

	if err := Migrate(db); err != nil {
		t.Fatalf("the upgrade aborted on a partially migrated branch database: %v\n"+
			"Version 101 is free here, so the renumbered target_exclude_caches runs its ALTER\n"+
			"against a column that already exists. SQLite has no ADD COLUMN IF NOT EXISTS, so\n"+
			"without alreadySatisfied the boot dies and the instance never comes back up.", err)
	}

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

// Migration 107 needs no alreadySatisfied guard because its body is idempotent,
// as v92's is.
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
