package store_test

import (
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

// TestRepoStatsRoundTrip covers the repo-size history: adding samples, reading
// the latest, the empty-store "not found" case, and that listing returns the
// samples ascending by `at`.
func TestRepoStatsRoundTrip(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	r := store.New(db)

	// Empty store: no latest sample.
	if _, found, err := r.LatestRepoStat("containers", "local"); err != nil {
		t.Fatalf("LatestRepoStat (empty): %v", err)
	} else if found {
		t.Fatal("expected found=false on an empty store")
	}

	// Add three samples with increasing `at`.
	for i, at := range []int64{100, 200, 300} {
		if err := r.AddRepoStat(store.RepoStat{
			Domain:      "containers",
			Source:      "local",
			At:          at,
			RawSize:     int64((i + 1) * 1000),
			RestoreSize: int64((i + 1) * 5000),
			Snapshots:   int64(i + 1),
		}); err != nil {
			t.Fatalf("AddRepoStat: %v", err)
		}
	}

	// Latest is the newest (at=300).
	latest, found, err := r.LatestRepoStat("containers", "local")
	if err != nil {
		t.Fatalf("LatestRepoStat: %v", err)
	}
	if !found {
		t.Fatal("expected found=true after adding samples")
	}
	if latest.At != 300 || latest.Snapshots != 3 {
		t.Fatalf("latest = %+v, want at=300 snapshots=3", latest)
	}

	// List returns them ascending by `at`.
	list, err := r.ListRepoStats("containers", "local", 0)
	if err != nil {
		t.Fatalf("ListRepoStats: %v", err)
	}
	if len(list) != 3 {
		t.Fatalf("expected 3 samples, got %d", len(list))
	}
	for i := 1; i < len(list); i++ {
		if list[i].At <= list[i-1].At {
			t.Fatalf("samples not ascending by at: %+v", list)
		}
	}

	// A different domain/source is isolated.
	if _, found, err := r.LatestRepoStat("vms", "local"); err != nil {
		t.Fatalf("LatestRepoStat (other domain): %v", err)
	} else if found {
		t.Fatal("a different domain must not see containers samples")
	}
}
