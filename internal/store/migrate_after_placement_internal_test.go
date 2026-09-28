package store

import (
	"database/sql"
	"slices"
	"testing"
)

// Builds of the placement release carry every migration up to 153, the
// placement ones at 109 to 119 among them. The storage place migrations follow
// them, so such a database applies exactly those on its next start. The
// released 9.0.0 and 9.1.0 lack the placement migrations and take both sets.

func isPlaceMigration(version int) bool { return version >= placesMigrationBase }

// bootWithout migrates db with every migration skip leaves in, the way a
// release that lacked the others did.
func bootWithout(t *testing.T, db *sql.DB, skip func(version int) bool) {
	t.Helper()
	var list []migration
	for _, m := range migrations {
		if !skip(m.version) {
			list = append(list, m)
		}
	}
	saved := migrations
	migrations = list
	defer func() { migrations = saved }()
	if err := Migrate(db); err != nil {
		t.Fatalf("migrate as an older release: %v", err)
	}
}

// seedPlacementBuild adds to seedV900 what a placement build records of its
// own: a placement default and a copy rule.
func seedPlacementBuild(t *testing.T, db *sql.DB) {
	t.Helper()
	seedV900(t, db)
	if _, err := db.Exec(`
INSERT INTO placement_defaults (domain, home, skip, confirmed_at, updated_at) VALUES ('vms', 'r-box', '["c-extra"]', 3000, 3000);
INSERT INTO offsite_copy_rules (domain, identity, skip, updated_at) VALUES ('containers', 'container:postgres', '["c-extra"]', 3000);`); err != nil {
		t.Fatalf("seed a placement build: %v", err)
	}
}

var placementBuildTables = append(slices.Clone(v900Tables), "placement_defaults", "offsite_copy_rules")

// snapshotTables renders every row of tables over the columns they have now.
func snapshotTables(t *testing.T, db *sql.DB, tables []string) (map[string][]string, map[string][]string) {
	t.Helper()
	cols := map[string][]string{}
	rows := map[string][]string{}
	for _, table := range tables {
		cols[table] = columnsOf(t, db, table)
		rows[table] = rowsOf(t, db, table, cols[table])
	}
	return cols, rows
}

// checkUpgrade migrates db and checks that every migration is recorded under
// its own number, that no row lost a value it had, that the schema is a fresh
// install's, and that a second run changes nothing.
func checkUpgrade(t *testing.T, db *sql.DB, tables []string) {
	t.Helper()
	cols, before := snapshotTables(t, db, tables)
	if err := Migrate(db); err != nil {
		t.Fatalf("upgrade: %v", err)
	}
	applied := appliedVersions(t, db)
	for _, m := range migrations {
		if applied[m.version] != m.name {
			t.Errorf("v%d = %q, want %q recorded", m.version, applied[m.version], m.name)
		}
	}
	if len(applied) != len(migrations) {
		t.Errorf("%d migrations recorded, want %d", len(applied), len(migrations))
	}
	for _, table := range tables {
		if after := rowsOf(t, db, table, cols[table]); !slices.Equal(after, before[table]) {
			t.Errorf("%s changed in the upgrade:\nbefore %v\nafter  %v", table, before[table], after)
		}
	}

	fresh := OpenMem(t)
	if err := Migrate(fresh); err != nil {
		t.Fatalf("migrate a fresh database: %v", err)
	}
	if got, want := schemaFingerprint(t, db), schemaFingerprint(t, fresh); got != want {
		t.Errorf("the upgraded schema differs from a fresh install:\n got %s\nwant %s", got, want)
	}

	allTables := append(slices.Clone(tables), "schema_migrations", "storage_places", "storage_domain_places")
	allCols, once := snapshotTables(t, db, allTables)
	if err := Migrate(db); err != nil {
		t.Fatalf("second migrate: %v", err)
	}
	for _, table := range allTables {
		if twice := rowsOf(t, db, table, allCols[table]); !slices.Equal(twice, once[table]) {
			t.Errorf("a second run changed %s:\nbefore %v\nafter  %v", table, once[table], twice)
		}
	}
}

func TestPlaceMigrationsApplyToAPlacementBuildDatabase(t *testing.T) {
	db := OpenMem(t)
	bootWithout(t, db, isPlaceMigration)
	seedPlacementBuild(t, db)
	checkUpgrade(t, db, placementBuildTables)

	rows, err := stringsQ(db, `SELECT id FROM offsite_targets WHERE place_id <> '' OR place_domain <> '' OR off_with_place <> 0`)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 0 {
		t.Errorf("rows %v start at a place; the move onto places is the start's job", rows)
	}
	if got := intOf(t, db, `SELECT places_migrated FROM settings WHERE id = 1`); got != 0 {
		t.Errorf("places_migrated = %d, want 0 so the start builds the places", got)
	}
}

// 9.1.0 added no migrations, so its database is the 9.0.0 one.
func TestPlacementAndPlaceMigrationsApplyToA900Database(t *testing.T) {
	db := OpenMem(t)
	bootWithout(t, db, func(v int) bool { return isPlacementMigration(v) || isPlaceMigration(v) })
	seedV900(t, db)
	checkUpgrade(t, db, v900Tables)
}

// Early builds of the storage places recorded their five migrations as 123 to
// 127, the numbers v9.0.0 gives to its database dump and ZFS columns.
func TestAnEarlyPlacesBuildTakesTheDumpAndZFSMigrations(t *testing.T) {
	db := OpenMem(t)
	migrateThrough(t, db, 122)
	for i, name := range placeMigrations {
		if _, err := db.Exec(migrationNamed(t, name).sql); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if _, err := db.Exec(`INSERT INTO schema_migrations (version, name, applied_at) VALUES (?, ?, 0)`,
			123+i, name); err != nil {
			t.Fatal(err)
		}
	}
	checkUpgrade(t, db, []string{"settings", "offsite_targets"})
	for _, column := range []string{"db_dump_off", "db_dump_engine"} {
		if !hasColumn(t, db, "targets", column) {
			t.Errorf("targets.%s is missing", column)
		}
	}
	if !hasTable(t, db, "zfs_datasets") {
		t.Error("zfs_datasets is missing")
	}
}
