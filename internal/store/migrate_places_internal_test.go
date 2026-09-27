package store

import (
	"fmt"
	"slices"
	"testing"
)

// placeMigrations are the storage place migrations in order.
var placeMigrations = []string{
	"storage_places",
	"storage_domain_places",
	"offsite_targets_place",
	"settings_places_migrated",
	"offsite_targets_off_with_place",
}

// v900Schema is the last migration a v9.0.0 database has recorded.
const v900Schema = 153

func TestAFreshDatabaseHasTheStoragePlaceSchema(t *testing.T) {
	db := OpenMem(t)
	if err := Migrate(db); err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"storage_places", "storage_domain_places"} {
		if !hasTable(t, db, table) {
			t.Errorf("table %s is missing", table)
		}
	}
	for _, column := range []string{"place_id", "place_domain", "place_suffix", "off_with_place"} {
		if !hasColumn(t, db, "offsite_targets", column) {
			t.Errorf("offsite_targets.%s is missing", column)
		}
	}
	if !hasColumn(t, db, "settings", "places_migrated") {
		t.Error("settings.places_migrated is missing")
	}
}

func TestRowsFromBeforeThePlacesStartAtNoPlace(t *testing.T) {
	db := OpenMem(t)
	migrateThrough(t, db, 122)
	if _, err := db.Exec(`INSERT INTO offsite_targets (id, domain, name, repo, role, enabled, created_at, sort_order)
		VALUES ('c-b2', 'containers', 'B2', 's3:b2/containers', 'offsite', 1, 1000, 0)`); err != nil {
		t.Fatal(err)
	}
	if err := Migrate(db); err != nil {
		t.Fatal(err)
	}
	var placeID, domain, suffix string
	if err := db.QueryRow(`SELECT place_id, place_domain, place_suffix FROM offsite_targets WHERE id = 'c-b2'`).
		Scan(&placeID, &domain, &suffix); err != nil {
		t.Fatal(err)
	}
	if placeID != "" || domain != "" || suffix != "" {
		t.Errorf("an existing row got place %q, domain %q, suffix %q, want none", placeID, domain, suffix)
	}
	var migrated int64
	if err := db.QueryRow(`SELECT places_migrated FROM settings WHERE id = 1`).Scan(&migrated); err != nil {
		t.Fatal(err)
	}
	if migrated != 0 {
		t.Errorf("places_migrated = %d, want 0 until the places are built", migrated)
	}
}

func TestThePlaceSchemaIsRecordedWhereItAlreadyExists(t *testing.T) {
	db := OpenMem(t)
	if err := Migrate(db); err != nil {
		t.Fatal(err)
	}
	for _, name := range placeMigrations {
		if _, err := db.Exec(`DELETE FROM schema_migrations WHERE name = ?`, name); err != nil {
			t.Fatal(err)
		}
	}
	if err := Migrate(db); err != nil {
		t.Fatalf("migrate over a database that already has the place schema: %v", err)
	}
	for _, name := range placeMigrations {
		if v := migrationNamed(t, name).version; !applied(t, db, v) {
			t.Errorf("v%d (%s) was not recorded", v, name)
		}
	}
}

func TestAv900DatabaseTakesThePlaceSchema(t *testing.T) {
	db := OpenMem(t)
	migrateThrough(t, db, 122)
	for v := 123; v <= v900Schema; v++ {
		if _, err := db.Exec(`INSERT INTO schema_migrations (version, name, applied_at) VALUES (?, ?, 0)`,
			v, fmt.Sprintf("v900_%d", v)); err != nil {
			t.Fatal(err)
		}
	}
	if err := Migrate(db); err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"storage_places", "storage_domain_places"} {
		if !hasTable(t, db, table) {
			t.Errorf("table %s is missing", table)
		}
	}
	for _, column := range []string{"place_id", "off_with_place"} {
		if !hasColumn(t, db, "offsite_targets", column) {
			t.Errorf("offsite_targets.%s is missing", column)
		}
	}
	if _, err := New(db).GetSettings(); err != nil {
		t.Fatalf("GetSettings: %v", err)
	}
}

// Test builds recorded the place migrations as 123 to 127. Those numbers
// belong to other migrations, which have to run there all the same.
func TestPlaceRecordsUnderEarlierNumbersMakeWayForTheirOwners(t *testing.T) {
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
	owner := migration{version: 123, name: "targets_db_dump_off",
		sql: `ALTER TABLE targets ADD COLUMN db_dump_off INTEGER NOT NULL DEFAULT 0;`}
	at := slices.IndexFunc(migrations, func(m migration) bool { return m.version > 122 })
	withMigrations(t, slices.Insert(slices.Clone(migrations), at, owner))
	if err := Migrate(db); err != nil {
		t.Fatal(err)
	}

	got := appliedVersions(t, db)
	if got[123] != owner.name || !hasColumn(t, db, "targets", "db_dump_off") {
		t.Errorf("v123 recorded as %q, want %s with its column", got[123], owner.name)
	}
	for v := 124; v <= 127; v++ {
		if name, ok := got[v]; ok {
			t.Errorf("v%d still recorded as %q", v, name)
		}
	}
	for _, name := range placeMigrations {
		if v := migrationNamed(t, name).version; got[v] != name {
			t.Errorf("v%d recorded as %q, want %s", v, got[v], name)
		}
	}
}

func TestRowsOffAtASwitchedOffPlaceComeBackWithItAfterTheUpgrade(t *testing.T) {
	db := OpenMem(t)
	migrateThrough(t, db, migrationNamed(t, "settings_places_migrated").version)
	if _, err := db.Exec(`
INSERT INTO storage_places (id, name, provider, kind, base, folders, enabled) VALUES
  ('p-off', 'B2',  's3-other', 's3',    's3:b2',       '{"containers":"containers","flash":"flash"}', 0),
  ('p-on',  'NAS', 'share',    'local', 'remotes/nas', '{"containers":"containers"}',                 1);
INSERT INTO offsite_targets (id, domain, name, repo, role, enabled, created_at, sort_order, place_id, place_domain) VALUES
  ('held',  'containers', 'B2',    's3:b2/containers',       'offsite', 0, 1000, 0, 'p-off', 'containers'),
  ('on',    'flash',      'B2',    's3:b2/flash',            'offsite', 1, 1001, 0, 'p-off', 'flash'),
  ('own',   'containers', 'NAS',   'remotes/nas/containers', 'offsite', 0, 1002, 1, 'p-on',  'containers'),
  ('loose', 'vms',        'Loose', 's3:elsewhere/vms',       'offsite', 0, 1003, 0, '',      '');`); err != nil {
		t.Fatal(err)
	}
	if err := Migrate(db); err != nil {
		t.Fatal(err)
	}
	marked, err := stringsQ(db, `SELECT id FROM offsite_targets WHERE off_with_place = 1 ORDER BY id`)
	if err != nil {
		t.Fatal(err)
	}
	if len(marked) != 1 || marked[0] != "held" {
		t.Fatalf("rows switched off by their place = %v, want only the one off at the place that is off", marked)
	}
}
