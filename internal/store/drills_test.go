package store_test

import (
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

// TestRestoreDrillsRoundTrip covers the restore-drill history: adding results,
// reading the latest, the empty-store "not found" case, that listing returns
// drills newest-first, that the ok flag round-trips, and that a different
// domain/source is isolated.
func TestRestoreDrillsRoundTrip(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	r := store.New(db)

	// Empty store: no latest drill.
	if _, found, err := r.LatestRestoreDrill("containers", "local"); err != nil {
		t.Fatalf("LatestRestoreDrill (empty): %v", err)
	} else if found {
		t.Fatal("expected found=false on an empty store")
	}

	// Add three drills with increasing `at`: two ok, the newest a failure.
	drills := []store.RestoreDrill{
		{Domain: "containers", Source: "local", At: 100, OK: true},
		{Domain: "containers", Source: "local", At: 200, OK: true},
		{Domain: "containers", Source: "local", At: 300, OK: false, Detail: "data corruption"},
	}
	for _, d := range drills {
		if err := r.AddRestoreDrill(d); err != nil {
			t.Fatalf("AddRestoreDrill: %v", err)
		}
	}

	// Latest is the newest (at=300), and the ok flag + detail round-trip.
	latest, found, err := r.LatestRestoreDrill("containers", "local")
	if err != nil {
		t.Fatalf("LatestRestoreDrill: %v", err)
	}
	if !found {
		t.Fatal("expected found=true after adding drills")
	}
	if latest.At != 300 || latest.OK || latest.Detail != "data corruption" {
		t.Fatalf("latest = %+v, want at=300 ok=false detail='data corruption'", latest)
	}

	// List returns them newest first (descending by `at`).
	list, err := r.ListRestoreDrills("containers", "local", 0)
	if err != nil {
		t.Fatalf("ListRestoreDrills: %v", err)
	}
	if len(list) != 3 {
		t.Fatalf("expected 3 drills, got %d", len(list))
	}
	for i := 1; i < len(list); i++ {
		if list[i].At >= list[i-1].At {
			t.Fatalf("drills not descending by at: %+v", list)
		}
	}

	// A different domain/source is isolated.
	if _, found, err := r.LatestRestoreDrill("vms", "local"); err != nil {
		t.Fatalf("LatestRestoreDrill (other domain): %v", err)
	} else if found {
		t.Fatal("a different domain must not see containers drills")
	}
}
