package store

import (
	"testing"
)

// TestAnomalyMigrationsCreateSchema pins the columns the anomaly detectors read
// and the index their per-series window relies on. The columns stay NULL on a
// row that predates them, because 0 is a real measurement.
func TestAnomalyMigrationsCreateSchema(t *testing.T) {
	db := OpenMem(t)
	if err := Migrate(db); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	if _, err := db.Exec(`
		INSERT INTO targets (id, container_name, appdata_paths, include_in_schedule, created_at)
		VALUES ('tg', 'sonarr', '[]', 0, 1)`); err != nil {
		t.Fatalf("insert target: %v", err)
	}
	if _, err := db.Exec(`
		INSERT INTO runs (id, target_id, kind, status, started_at, finished_at, snapshot_id, bytes)
		VALUES ('old', 'tg', 'backup', 'success', 100, 101, 'snap', 4096)`); err != nil {
		t.Fatalf("insert run: %v", err)
	}

	for _, col := range []string{"source_bytes", "source_files", "files_new", "restic_ms", "has_parent", "selection_fp"} {
		var isNull bool
		if err := db.QueryRow(`SELECT ` + col + ` IS NULL FROM runs WHERE id = 'old'`).Scan(&isNull); err != nil {
			t.Fatalf("read runs.%s: %v", col, err)
		}
		if !isNull {
			t.Fatalf("runs.%s carries a value on a row written before the migration", col)
		}
	}

	for _, name := range []string{"idx_runs_target_kind_started", "idx_anomalies_open_fingerprint", "idx_anomalies_scope", "idx_anomalies_target"} {
		var n int
		err := db.QueryRow(
			`SELECT count(*) FROM sqlite_master WHERE type = 'index' AND name = ?`, name,
		).Scan(&n)
		if err != nil {
			t.Fatalf("probe index %s: %v", name, err)
		}
		if n != 1 {
			t.Fatalf("index %s is missing", name)
		}
	}

	tables := map[string][]string{
		"anomalies":            {"target_id", "last_good_run_id", "cleared_at", "notified_severity"},
		"anomaly_item_prefs":   {"sensitivity", "notify_min"},
		"anomaly_expectations": {"since_at", "ceiling", "anomaly_id"},
		"volume_samples":       {"free_bytes", "total_bytes", "domains", "source"},
		"anomaly_backfill":     {"attempted_at", "done", "filled", "without_summary", "error"},
	}
	for table, columns := range tables {
		rows, err := db.Query(`SELECT name FROM pragma_table_info(?)`, table)
		if err != nil {
			t.Fatalf("read %s: %v", table, err)
		}
		present := map[string]bool{}
		for rows.Next() {
			var name string
			if err := rows.Scan(&name); err != nil {
				t.Fatalf("read %s: %v", table, err)
			}
			present[name] = true
		}
		rows.Close() //nolint:errcheck,gosec // test cleanup on a completed query
		for _, col := range columns {
			if !present[col] {
				t.Fatalf("%s has no column %s", table, col)
			}
		}
	}

	var enabled, hold int
	var sensitivity, notifyMin string
	err := db.QueryRow(`
		SELECT anomaly_enabled, anomaly_sensitivity, anomaly_notify_min, anomaly_retention_hold
		FROM settings WHERE id = 1`).Scan(&enabled, &sensitivity, &notifyMin, &hold)
	if err != nil {
		t.Fatalf("read anomaly settings: %v", err)
	}
	if enabled != 1 || sensitivity != "balanced" || notifyMin != "critical" || hold != 1 {
		t.Fatalf("anomaly settings default to (%d, %q, %q, %d)", enabled, sensitivity, notifyMin, hold)
	}
}

// TestAnomalyMigrationsCarryGuards pins which of the anomaly bodies may be
// skipped. An ALTER body needs the guard because SQLite cannot add a column
// idempotently; the CREATE body must not have one, or a database that got the
// anomalies table under another number would never see the tables next to it.
func TestAnomalyMigrationsCarryGuards(t *testing.T) {
	guards := map[int]bool{}
	for _, m := range migrations {
		if m.version >= anomalyMigrationBase && m.version <= anomalyMigrationBase+2 {
			guards[m.version] = m.alreadySatisfied != nil
		}
	}
	for version, want := range map[int]bool{
		anomalyMigrationBase:     true,
		anomalyMigrationBase + 1: false,
		anomalyMigrationBase + 2: true,
	} {
		got, ok := guards[version]
		if !ok {
			t.Fatalf("migration v%d is missing", version)
		}
		if got != want {
			t.Fatalf("migration v%d has a guard = %v, want %v", version, got, want)
		}
	}

	db := OpenMem(t)
	if err := Migrate(db); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	if _, err := db.Exec(`DROP TABLE volume_samples`); err != nil {
		t.Fatalf("drop volume_samples: %v", err)
	}
	if _, err := db.Exec(`DELETE FROM schema_migrations WHERE version = ?`, anomalyMigrationBase+1); err != nil {
		t.Fatalf("forget the anomalies migration: %v", err)
	}
	if err := Migrate(db); err != nil {
		t.Fatalf("Migrate over an existing anomalies table: %v", err)
	}
	if _, err := db.Exec(`SELECT volume FROM volume_samples LIMIT 1`); err != nil {
		t.Fatalf("volume_samples was not restored: %v", err)
	}
}

// TestAnomalySettingsMigrationSurvivesRenumbering covers the database that
// reaches this version with the settings columns already added under a number
// of its own.
func TestAnomalySettingsMigrationSurvivesRenumbering(t *testing.T) {
	db := OpenMem(t)
	if err := Migrate(db); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	if _, err := db.Exec(`DELETE FROM schema_migrations WHERE version = ?`, anomalyMigrationBase+2); err != nil {
		t.Fatalf("forget the settings migration: %v", err)
	}
	if err := Migrate(db); err != nil {
		t.Fatalf("Migrate over existing settings columns: %v", err)
	}
}
