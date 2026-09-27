package store

import (
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
