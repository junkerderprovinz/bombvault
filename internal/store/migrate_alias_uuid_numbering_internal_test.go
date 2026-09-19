package store

import (
	"database/sql"
	"testing"
)

// Other builds record offsite_targets_primary_slot or
// named_repo_already_offsite as 109, so the alias schema sits at 120 to 122.
// These tests migrate a database that carries either 109, and one that has
// the alias schema without its version rows, which the alreadySatisfied
// guards record without running the bodies again.

// seedWith109As builds a database that ran another build first: a full
// install without the alias schema or its version rows, and 109 recorded
// under name.
func seedWith109As(t *testing.T, name string) *sql.DB {
	t.Helper()
	db := OpenMem(t)
	if err := Migrate(db); err != nil {
		t.Fatalf("seed install: %v", err)
	}
	if _, err := db.Exec(`DELETE FROM schema_migrations WHERE version IN (120, 121, 122)`); err != nil {
		t.Fatalf("unseed the version rows: %v", err)
	}
	// Dropping the table also takes prev_definition (122) with it.
	if _, err := db.Exec(`DROP TABLE IF EXISTS target_aliases`); err != nil {
		t.Fatalf("unseed target_aliases: %v", err)
	}
	if _, err := db.Exec(`ALTER TABLE vms DROP COLUMN uuid`); err != nil {
		t.Fatalf("unseed vms.uuid: %v", err)
	}
	record(t, db, 109, name)
	return db
}

// Here 109 is a data-only body, recorded under its own name.
func TestADatabaseWithOffsiteTargetsPrimarySlotAt109StillGetsTheAliasWork(t *testing.T) {
	db := seedWith109As(t, "offsite_targets_primary_slot")

	if err := Migrate(db); err != nil {
		t.Fatalf("migrate on a database with 109=offsite_targets_primary_slot failed: %v", err)
	}

	if !hasTable(t, db, "target_aliases") {
		t.Error("target_aliases is missing")
	}
	if !hasColumn(t, db, "vms", "uuid") {
		t.Error("vms.uuid is missing")
	}
	if !hasColumn(t, db, "target_aliases", "prev_definition") {
		t.Error("target_aliases.prev_definition is missing")
	}
	for _, v := range []int{120, 121, 122} {
		if !applied(t, db, v) {
			t.Errorf("v%d was not recorded", v)
		}
	}
}

// Here 109 adds a column, so the seed adds it along with the version row.
func TestADatabaseWithNamedRepoAlreadyOffsiteAt109StillGetsTheAliasWork(t *testing.T) {
	db := seedWith109As(t, "named_repo_already_offsite")
	if _, err := db.Exec(`ALTER TABLE offsite_targets ADD COLUMN already_offsite INTEGER NOT NULL DEFAULT 0;`); err != nil {
		t.Fatalf("seed the already_offsite column: %v", err)
	}

	if err := Migrate(db); err != nil {
		t.Fatalf("migrate on a database with 109=named_repo_already_offsite failed: %v", err)
	}

	if !hasTable(t, db, "target_aliases") {
		t.Error("target_aliases is missing")
	}
	if !hasColumn(t, db, "vms", "uuid") {
		t.Error("vms.uuid is missing")
	}
	if !hasColumn(t, db, "target_aliases", "prev_definition") {
		t.Error("target_aliases.prev_definition is missing")
	}
	for _, v := range []int{120, 121, 122} {
		if !applied(t, db, v) {
			t.Errorf("v%d was not recorded", v)
		}
	}
}

// TestATargetAliasesDatabaseMissingItsVersionRowsMigratesWithoutDuplicateErrors
// is the guard's own proof: a database that already has the table and both
// columns, through whatever path, but never recorded 120/121/122, must not have
// Migrate reissue the CREATE TABLE or either ALTER against them.
func TestATargetAliasesDatabaseMissingItsVersionRowsMigratesWithoutDuplicateErrors(t *testing.T) {
	db := OpenMem(t)
	if err := Migrate(db); err != nil {
		t.Fatalf("fresh migrate: %v", err)
	}
	if _, err := db.Exec(`DELETE FROM schema_migrations WHERE version IN (120, 121, 122)`); err != nil {
		t.Fatalf("unseed the version rows: %v", err)
	}

	if err := Migrate(db); err != nil {
		t.Fatalf("migrate on a database that already has the table and columns failed: %v", err)
	}

	for _, v := range []int{120, 121, 122} {
		if !applied(t, db, v) {
			t.Errorf("v%d was not recorded", v)
		}
	}
}

// TestAFreshDatabaseGetsTargetAliasesAndVMUUID is the ordinary path, kept so a
// later change to the guards cannot break the common case.
func TestAFreshDatabaseGetsTargetAliasesAndVMUUID(t *testing.T) {
	db := OpenMem(t)
	if err := Migrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	if !hasTable(t, db, "target_aliases") {
		t.Fatal("target_aliases is missing")
	}
	if !hasColumn(t, db, "vms", "uuid") {
		t.Fatal("vms.uuid is missing")
	}
	if !hasColumn(t, db, "target_aliases", "prev_definition") {
		t.Fatal("target_aliases.prev_definition is missing")
	}
}
