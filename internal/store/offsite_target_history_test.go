package store_test

import (
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

// TestRecordTamperTestForTarget covers the per-destination tamper history: an
// empty targetID delegates to the domain-wide record/read (byte-identical N=1),
// while a real targetID stamps offsite_target_id so per-destination reads isolate.
func TestRecordTamperTestForTarget(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	r := store.New(db)

	// Empty targetID delegates to the domain path: the row is readable both
	// domain-wide and via the empty-target read (which itself delegates).
	if err := r.RecordTamperTestForTarget("containers", "", true, ""); err != nil {
		t.Fatalf("RecordTamperTestForTarget(\"\"): %v", err)
	}
	if tt, found, err := r.LatestTamperTest("containers"); err != nil || !found || !tt.Protected {
		t.Fatalf("domain read after empty-target record: found=%v protected=%v err=%v", found, tt.Protected, err)
	}
	if tt, found, err := r.LatestTamperTestForTarget("containers", ""); err != nil || !found || !tt.Protected {
		t.Fatalf("empty-target read must delegate to domain: found=%v protected=%v err=%v", found, tt.Protected, err)
	}
	// A specific targetID must NOT match the ""-stamped row.
	if _, found, err := r.LatestTamperTestForTarget("containers", "aaaa1111"); err != nil {
		t.Fatalf("LatestTamperTestForTarget(id): %v", err)
	} else if found {
		t.Fatal("a specific target must not see the domain-default (\"\") row")
	}

	// Two destinations for the same domain record independent verdicts.
	if err := r.RecordTamperTestForTarget("containers", "t1", true, ""); err != nil {
		t.Fatalf("RecordTamperTestForTarget(t1): %v", err)
	}
	if err := r.RecordTamperTestForTarget("containers", "t2", false, "server accepted a delete"); err != nil {
		t.Fatalf("RecordTamperTestForTarget(t2): %v", err)
	}
	t1, found, err := r.LatestTamperTestForTarget("containers", "t1")
	if err != nil || !found || !t1.Protected {
		t.Fatalf("t1 must read protected: found=%v protected=%v err=%v", found, t1.Protected, err)
	}
	t2, found, err := r.LatestTamperTestForTarget("containers", "t2")
	if err != nil || !found || t2.Protected {
		t.Fatalf("t2 must read UNprotected: found=%v protected=%v err=%v", found, t2.Protected, err)
	}
	if t2.Detail != "server accepted a delete" {
		t.Fatalf("t2 detail round-trip: got %q", t2.Detail)
	}
}

// TestLatestSuccessfulOffsiteRunForTarget covers the per-destination currency
// source used by the worst-of scorecard aggregation.
func TestLatestSuccessfulOffsiteRunForTarget(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	r := store.New(db)

	// A successful run for t1, a still-open (unfinished) run for t2.
	id1, err := r.RecordOffsiteRunForTarget("containers", "t1", 1000)
	if err != nil {
		t.Fatalf("RecordOffsiteRunForTarget(t1): %v", err)
	}
	if err := r.FinishOffsiteRun(id1, true, ""); err != nil {
		t.Fatalf("FinishOffsiteRun(t1): %v", err)
	}
	if _, err := r.RecordOffsiteRunForTarget("containers", "t2", 2000); err != nil {
		t.Fatalf("RecordOffsiteRunForTarget(t2): %v", err)
	}

	run, found, err := r.LatestSuccessfulOffsiteRunForTarget("containers", "t1")
	if err != nil || !found {
		t.Fatalf("t1 must have a successful run: found=%v err=%v", found, err)
	}
	if run.StartedAt != 1000 || !run.OK {
		t.Fatalf("t1 run = %+v, want StartedAt=1000 ok=true", run)
	}
	// t2 has only an unfinished (not-yet-successful) run → no success.
	if _, found, err := r.LatestSuccessfulOffsiteRunForTarget("containers", "t2"); err != nil {
		t.Fatalf("LatestSuccessfulOffsiteRunForTarget(t2): %v", err)
	} else if found {
		t.Fatal("t2 has no SUCCESSFUL run yet — must not be found")
	}
	// Empty targetID delegates to the domain-wide query (finds t1's success).
	if _, found, err := r.LatestSuccessfulOffsiteRunForTarget("containers", ""); err != nil || !found {
		t.Fatalf("empty-target must delegate to domain-wide success: found=%v err=%v", found, err)
	}
}
