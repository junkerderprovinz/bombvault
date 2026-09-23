package store_test

import (
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

// TestRecordTamperTestForTarget expects an empty targetID to use the
// domain-wide record and read, while a real one stamps offsite_target_id so
// each destination reads only its own results.
func TestRecordTamperTestForTarget(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	r := store.New(db)

	if err := r.RecordTamperTestForTarget("containers", "", true, ""); err != nil {
		t.Fatalf("RecordTamperTestForTarget(\"\"): %v", err)
	}
	if tt, found, err := r.LatestTamperTest("containers"); err != nil || !found || !tt.Protected {
		t.Fatalf("domain read after empty-target record: found=%v protected=%v err=%v", found, tt.Protected, err)
	}
	if tt, found, err := r.LatestTamperTestForTarget("containers", ""); err != nil || !found || !tt.Protected {
		t.Fatalf("empty-target read must delegate to domain: found=%v protected=%v err=%v", found, tt.Protected, err)
	}
	// A specific targetID must not match the ""-stamped row.
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
// source behind the worst-of scorecard.
func TestLatestSuccessfulOffsiteRunForTarget(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	r := store.New(db)

	// A successful run for t1, an unfinished one for t2.
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
	if _, found, err := r.LatestSuccessfulOffsiteRunForTarget("containers", "t2"); err != nil {
		t.Fatalf("LatestSuccessfulOffsiteRunForTarget(t2): %v", err)
	} else if found {
		t.Fatal("t2 has no successful run yet and must not be found")
	}
	// An empty targetID falls back to the domain-wide query and finds t1's run.
	if _, found, err := r.LatestSuccessfulOffsiteRunForTarget("containers", ""); err != nil || !found {
		t.Fatalf("empty-target must delegate to domain-wide success: found=%v err=%v", found, err)
	}
}
