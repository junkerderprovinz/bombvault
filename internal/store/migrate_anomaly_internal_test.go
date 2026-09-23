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

	var n int
	err := db.QueryRow(
		`SELECT count(*) FROM sqlite_master WHERE type = 'index' AND name = 'idx_runs_target_kind_started'`,
	).Scan(&n)
	if err != nil {
		t.Fatalf("probe index: %v", err)
	}
	if n != 1 {
		t.Fatal("idx_runs_target_kind_started is missing")
	}
}
