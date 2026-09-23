package store_test

import (
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

// TestTamperTestsRoundTrip expects the latest verdict to win, so a flip to
// unprotected shows, and domains to stay isolated.
func TestTamperTestsRoundTrip(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	r := store.New(db)

	if _, found, err := r.LatestTamperTest("containers"); err != nil {
		t.Fatalf("LatestTamperTest (empty): %v", err)
	} else if found {
		t.Fatal("expected found=false on an empty store")
	}

	if err := r.RecordTamperTest("containers", true, ""); err != nil {
		t.Fatalf("RecordTamperTest: %v", err)
	}
	if err := r.RecordTamperTest("containers", false, "server accepted a delete"); err != nil {
		t.Fatalf("RecordTamperTest (flip): %v", err)
	}
	latest, found, err := r.LatestTamperTest("containers")
	if err != nil {
		t.Fatalf("LatestTamperTest: %v", err)
	}
	if !found {
		t.Fatal("expected found=true after recording tamper tests")
	}
	if latest.Protected || latest.Detail != "server accepted a delete" {
		t.Fatalf("latest = %+v, want the newest (unprotected) verdict", latest)
	}
	if latest.Domain != "containers" || latest.At == 0 {
		t.Fatalf("latest = %+v, want domain=containers with a timestamp", latest)
	}

	if _, found, err := r.LatestTamperTest("vms"); err != nil {
		t.Fatalf("LatestTamperTest (other domain): %v", err)
	} else if found {
		t.Fatal("a different domain must not see containers tamper tests")
	}
}

// TestOffsiteRunsRoundTrip covers successful, failed and still-running
// replication runs, the latest run winning, and domain isolation.
func TestOffsiteRunsRoundTrip(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	r := store.New(db)

	if _, found, err := r.LatestOffsiteRun("flash"); err != nil {
		t.Fatalf("LatestOffsiteRun (empty): %v", err)
	} else if found {
		t.Fatal("expected found=false on an empty store")
	}

	id1, err := r.RecordOffsiteRun("flash", 100)
	if err != nil {
		t.Fatalf("RecordOffsiteRun: %v", err)
	}
	if id1 == 0 {
		t.Fatal("RecordOffsiteRun must return a non-zero rowid")
	}
	if err := r.FinishOffsiteRun(id1, true, ""); err != nil {
		t.Fatalf("FinishOffsiteRun: %v", err)
	}
	run, found, err := r.LatestOffsiteRun("flash")
	if err != nil {
		t.Fatalf("LatestOffsiteRun: %v", err)
	}
	if !found {
		t.Fatal("expected found=true after recording a run")
	}
	if !run.OK || run.Error != "" || run.StartedAt != 100 || run.FinishedAt == 0 {
		t.Fatalf("run = %+v, want ok=true finished with started_at=100", run)
	}

	// A newer failed run wins.
	id2, err := r.RecordOffsiteRun("flash", 200)
	if err != nil {
		t.Fatalf("RecordOffsiteRun (second): %v", err)
	}
	if id2 == id1 {
		t.Fatalf("expected distinct rowids, got %d twice", id1)
	}
	if err := r.FinishOffsiteRun(id2, false, "copy failed"); err != nil {
		t.Fatalf("FinishOffsiteRun (failure): %v", err)
	}
	run, found, err = r.LatestOffsiteRun("flash")
	if err != nil || !found {
		t.Fatalf("LatestOffsiteRun (after failure): found=%v err=%v", found, err)
	}
	if run.OK || run.Error != "copy failed" || run.StartedAt != 200 {
		t.Fatalf("run = %+v, want the newest failed run", run)
	}

	// An unfinished run wins too.
	if _, err := r.RecordOffsiteRun("flash", 300); err != nil {
		t.Fatalf("RecordOffsiteRun (running): %v", err)
	}
	run, found, err = r.LatestOffsiteRun("flash")
	if err != nil || !found {
		t.Fatalf("LatestOffsiteRun (running): found=%v err=%v", found, err)
	}
	if run.OK || run.FinishedAt != 0 || run.StartedAt != 300 {
		t.Fatalf("run = %+v, want an unfinished run (finishedAt=0)", run)
	}

	if _, found, err := r.LatestOffsiteRun("containers"); err != nil {
		t.Fatalf("LatestOffsiteRun (other domain): %v", err)
	} else if found {
		t.Fatal("a different domain must not see flash runs")
	}
}

// TestLatestSuccessfulOffsiteRun checks that a newer failed or still-running
// run does not hide the last successful copy, so a broken replication reads as
// stale.
func TestLatestSuccessfulOffsiteRun(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	r := store.New(db)

	if _, found, err := r.LatestSuccessfulOffsiteRun("flash"); err != nil {
		t.Fatalf("LatestSuccessfulOffsiteRun (empty): %v", err)
	} else if found {
		t.Fatal("expected found=false on an empty store")
	}

	id1, err := r.RecordOffsiteRun("flash", 100)
	if err != nil {
		t.Fatalf("RecordOffsiteRun: %v", err)
	}
	if err := r.FinishOffsiteRun(id1, true, ""); err != nil {
		t.Fatalf("FinishOffsiteRun: %v", err)
	}
	id2, err := r.RecordOffsiteRun("flash", 200)
	if err != nil {
		t.Fatalf("RecordOffsiteRun (fail): %v", err)
	}
	if err := r.FinishOffsiteRun(id2, false, "copy failed"); err != nil {
		t.Fatalf("FinishOffsiteRun (fail): %v", err)
	}

	run, found, err := r.LatestSuccessfulOffsiteRun("flash")
	if err != nil || !found {
		t.Fatalf("LatestSuccessfulOffsiteRun: found=%v err=%v", found, err)
	}
	if !run.OK || run.StartedAt != 100 {
		t.Fatalf("run = %+v, want the last SUCCESSFUL run (started_at=100), not the newer failure", run)
	}

	if _, err := r.RecordOffsiteRun("flash", 300); err != nil {
		t.Fatalf("RecordOffsiteRun (running): %v", err)
	}
	run, found, err = r.LatestSuccessfulOffsiteRun("flash")
	if err != nil || !found || run.StartedAt != 100 {
		t.Fatalf("a still-running row must not count as success; want started_at=100, got %+v found=%v err=%v", run, found, err)
	}

	if _, found, err := r.LatestSuccessfulOffsiteRun("containers"); err != nil {
		t.Fatalf("LatestSuccessfulOffsiteRun (other domain): %v", err)
	} else if found {
		t.Fatal("a different domain must not see flash runs")
	}
}

// TestRestoreDrillKinds expects a drill without a kind to be stored as
// "subset", LatestRestoreDrillKind to filter by kind and LatestRestoreDrill to
// return the newest drill of any kind.
func TestRestoreDrillKinds(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	r := store.New(db)

	if _, found, err := r.LatestRestoreDrillKind("containers", "offsite", "dr"); err != nil {
		t.Fatalf("LatestRestoreDrillKind (empty): %v", err)
	} else if found {
		t.Fatal("expected found=false on an empty store")
	}

	// A drill without a kind gets the column default, "subset".
	if err := r.AddRestoreDrill(store.RestoreDrill{Domain: "containers", Source: "offsite", At: 100, OK: true}); err != nil {
		t.Fatalf("AddRestoreDrill (subset): %v", err)
	}
	if err := r.AddRestoreDrill(store.RestoreDrill{Domain: "containers", Source: "offsite", At: 200, OK: false, Detail: "restore mismatch", Kind: "dr"}); err != nil {
		t.Fatalf("AddRestoreDrill (dr): %v", err)
	}

	dr, found, err := r.LatestRestoreDrillKind("containers", "offsite", "dr")
	if err != nil || !found {
		t.Fatalf("LatestRestoreDrillKind dr: found=%v err=%v", found, err)
	}
	if dr.At != 200 || dr.OK || dr.Kind != "dr" || dr.Detail != "restore mismatch" {
		t.Fatalf("dr drill = %+v, want at=200 ok=false kind=dr", dr)
	}
	subset, found, err := r.LatestRestoreDrillKind("containers", "offsite", "subset")
	if err != nil || !found {
		t.Fatalf("LatestRestoreDrillKind subset: found=%v err=%v", found, err)
	}
	if subset.At != 100 || !subset.OK || subset.Kind != "subset" {
		t.Fatalf("subset drill = %+v, want at=100 ok=true kind=subset", subset)
	}

	latest, found, err := r.LatestRestoreDrill("containers", "offsite")
	if err != nil || !found {
		t.Fatalf("LatestRestoreDrill: found=%v err=%v", found, err)
	}
	if latest.At != 200 || latest.Kind != "dr" {
		t.Fatalf("latest = %+v, want the newest drill regardless of kind", latest)
	}

	if _, found, err := r.LatestRestoreDrillKind("containers", "offsite", "nope"); err != nil {
		t.Fatalf("LatestRestoreDrillKind (unknown kind): %v", err)
	} else if found {
		t.Fatal("an unknown kind must not match any drill")
	}
}
