package store

import "testing"

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
	for _, column := range []string{"place_id", "place_domain", "place_suffix"} {
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
	if _, err := db.Exec(`DELETE FROM schema_migrations WHERE version IN (123, 124, 125, 126)`); err != nil {
		t.Fatal(err)
	}
	if err := Migrate(db); err != nil {
		t.Fatalf("migrate over a database that already has the place schema: %v", err)
	}
	for _, v := range []int{123, 124, 125, 126} {
		if !applied(t, db, v) {
			t.Errorf("v%d was not recorded", v)
		}
	}
}
