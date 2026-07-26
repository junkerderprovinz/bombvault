package store_test

import (
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

// TestWatchdogStateRoundTrip pins the watchdog_state CRUD: absent → upsert →
// read-back → refresh (same domain, new episode) → delete → absent again.
func TestWatchdogStateRoundTrip(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	r := store.New(db)

	if _, found, err := r.GetWatchdogState("containers"); err != nil || found {
		t.Fatalf("expected no state before upsert, found=%v err=%v", found, err)
	}

	if err := r.UpsertWatchdogState(store.WatchdogState{Domain: "containers", NotifiedAt: 100, LastSuccessAt: 50}); err != nil {
		t.Fatal(err)
	}
	ws, found, err := r.GetWatchdogState("containers")
	if err != nil || !found {
		t.Fatalf("expected state after upsert, found=%v err=%v", found, err)
	}
	if ws.NotifiedAt != 100 || ws.LastSuccessAt != 50 {
		t.Fatalf("state = %+v, want NotifiedAt=100 LastSuccessAt=50", ws)
	}

	// Upsert on the same domain replaces the episode (conflict path).
	if err := r.UpsertWatchdogState(store.WatchdogState{Domain: "containers", NotifiedAt: 200, LastSuccessAt: 150}); err != nil {
		t.Fatal(err)
	}
	ws, _, err = r.GetWatchdogState("containers")
	if err != nil {
		t.Fatal(err)
	}
	if ws.NotifiedAt != 200 || ws.LastSuccessAt != 150 {
		t.Fatalf("refreshed state = %+v, want NotifiedAt=200 LastSuccessAt=150", ws)
	}

	// Delete → absent; deleting again stays a no-op (no error).
	if err := r.DeleteWatchdogState("containers"); err != nil {
		t.Fatal(err)
	}
	if _, found, err := r.GetWatchdogState("containers"); err != nil || found {
		t.Fatalf("expected no state after delete, found=%v err=%v", found, err)
	}
	if err := r.DeleteWatchdogState("containers"); err != nil {
		t.Fatalf("deleting a missing row must be a no-op, got %v", err)
	}
}
