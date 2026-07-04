package store_test

import (
	"path/filepath"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

// TestVacuumIntoProducesConsistentSnapshot verifies VacuumInto writes a
// standalone, fully-consistent copy of the live DB: the snapshot opens as an
// independent database and reads back a value written to the source. VACUUM INTO
// is only meaningful from an on-disk source, so this opens a real temp DB rather
// than the in-memory OpenMem helper the other store tests use.
func TestVacuumIntoProducesConsistentSnapshot(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "src.sqlite"))
	if err != nil {
		t.Fatalf("open source db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := store.Migrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	r := store.New(db)

	s, err := r.GetSettings()
	if err != nil {
		t.Fatalf("GetSettings: %v", err)
	}
	s.ContainersPath = "marker/path"
	if err := r.UpdateSettings(s); err != nil {
		t.Fatalf("UpdateSettings: %v", err)
	}

	dst := filepath.Join(t.TempDir(), "snap.sqlite")
	if err := r.VacuumInto(dst); err != nil {
		t.Fatalf("VacuumInto: %v", err)
	}

	// Open the snapshot as an independent DB and read the marker back.
	snapDB, err := store.Open(dst)
	if err != nil {
		t.Fatalf("open snapshot: %v", err)
	}
	defer snapDB.Close() //nolint:errcheck // test cleanup; error not actionable
	var path string
	if err := snapDB.QueryRow("SELECT containers_path FROM settings WHERE id = 1").Scan(&path); err != nil {
		t.Fatalf("read marker from snapshot: %v", err)
	}
	if path != "marker/path" {
		t.Fatalf("snapshot inconsistent: got %q", path)
	}
}
