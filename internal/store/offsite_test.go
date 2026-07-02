package store_test

import (
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

// TestTamperTestsRoundTrip covers the tamper-test history: the empty-store
// "not found" case, that the latest verdict wins (a protected→unprotected flip
// is visible), and that domains are isolated.
func TestTamperTestsRoundTrip(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	r := store.New(db)

	// Empty store: no tamper test yet.
	if _, found, err := r.LatestTamperTest("containers"); err != nil {
		t.Fatalf("LatestTamperTest (empty): %v", err)
	} else if found {
		t.Fatal("expected found=false on an empty store")
	}

	// Record protected, then a flip to unprotected — the latest wins.
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

	// A different domain is isolated.
	if _, found, err := r.LatestTamperTest("vms"); err != nil {
		t.Fatalf("LatestTamperTest (other domain): %v", err)
	} else if found {
		t.Fatal("a different domain must not see containers tamper tests")
	}
}

// TestOffsiteRunsRoundTrip covers the replication history: begin/finish for a
// successful and a failed run, the still-running shape (no finished_at), that
// the latest run wins, and domain isolation.
func TestOffsiteRunsRoundTrip(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	r := store.New(db)

	// Empty store: no run yet.
	if _, found, err := r.LatestOffsiteRun("flash"); err != nil {
		t.Fatalf("LatestOffsiteRun (empty): %v", err)
	} else if found {
		t.Fatal("expected found=false on an empty store")
	}

	// A finished, successful run.
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

	// A newer failed run wins, carrying its (scrubbed) error text.
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

	// A still-running (unfinished) run: ok=false, no finish timestamp yet.
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

	// A different domain is isolated.
	if _, found, err := r.LatestOffsiteRun("containers"); err != nil {
		t.Fatalf("LatestOffsiteRun (other domain): %v", err)
	} else if found {
		t.Fatal("a different domain must not see flash runs")
	}
}

// TestLatestSuccessfulOffsiteRun pins that LatestSuccessfulOffsiteRun returns the
// most recent run whose ok=1, IGNORING a newer failed (or still-running) run — so
// a broken replication reads as stale (last real copy) rather than fresh. This is
// the currency source the scorecard uses (mirrors backups' last-SUCCESS).
func TestLatestSuccessfulOffsiteRun(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	r := store.New(db)

	// Empty store: no successful run yet.
	if _, found, err := r.LatestSuccessfulOffsiteRun("flash"); err != nil {
		t.Fatalf("LatestSuccessfulOffsiteRun (empty): %v", err)
	} else if found {
		t.Fatal("expected found=false on an empty store")
	}

	// A successful run at t=100, then a NEWER failed run at t=200.
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

	// The successful (older) run must win over the newer failed one.
	run, found, err := r.LatestSuccessfulOffsiteRun("flash")
	if err != nil || !found {
		t.Fatalf("LatestSuccessfulOffsiteRun: found=%v err=%v", found, err)
	}
	if !run.OK || run.StartedAt != 100 {
		t.Fatalf("run = %+v, want the last SUCCESSFUL run (started_at=100), not the newer failure", run)
	}

	// A still-running (unfinished) row is not a success either.
	if _, err := r.RecordOffsiteRun("flash", 300); err != nil {
		t.Fatalf("RecordOffsiteRun (running): %v", err)
	}
	run, found, err = r.LatestSuccessfulOffsiteRun("flash")
	if err != nil || !found || run.StartedAt != 100 {
		t.Fatalf("a still-running row must not count as success; want started_at=100, got %+v found=%v err=%v", run, found, err)
	}

	// Domain isolation.
	if _, found, err := r.LatestSuccessfulOffsiteRun("containers"); err != nil {
		t.Fatalf("LatestSuccessfulOffsiteRun (other domain): %v", err)
	} else if found {
		t.Fatal("a different domain must not see flash runs")
	}
}

// TestRestoreDrillKinds covers the drill kind column: a kind-less record
// defaults to "subset", a kind="dr" drill is retrievable via
// LatestRestoreDrillKind, and the plain LatestRestoreDrill keeps returning the
// newest drill of ANY kind.
func TestRestoreDrillKinds(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	r := store.New(db)

	// No drills of a kind yet.
	if _, found, err := r.LatestRestoreDrillKind("containers", "offsite", "dr"); err != nil {
		t.Fatalf("LatestRestoreDrillKind (empty): %v", err)
	} else if found {
		t.Fatal("expected found=false on an empty store")
	}

	// A kind-less record defaults to "subset" (mirrors the SQL column default).
	if err := r.AddRestoreDrill(store.RestoreDrill{Domain: "containers", Source: "offsite", At: 100, OK: true}); err != nil {
		t.Fatalf("AddRestoreDrill (subset): %v", err)
	}
	// A newer real-DR drill.
	if err := r.AddRestoreDrill(store.RestoreDrill{Domain: "containers", Source: "offsite", At: 200, OK: false, Detail: "restore mismatch", Kind: "dr"}); err != nil {
		t.Fatalf("AddRestoreDrill (dr): %v", err)
	}

	// Kind-aware lookups see exactly their kind.
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

	// The plain latest keeps returning the newest of ANY kind (here the dr one).
	latest, found, err := r.LatestRestoreDrill("containers", "offsite")
	if err != nil || !found {
		t.Fatalf("LatestRestoreDrill: found=%v err=%v", found, err)
	}
	if latest.At != 200 || latest.Kind != "dr" {
		t.Fatalf("latest = %+v, want the newest drill regardless of kind", latest)
	}

	// An unknown kind finds nothing.
	if _, found, err := r.LatestRestoreDrillKind("containers", "offsite", "nope"); err != nil {
		t.Fatalf("LatestRestoreDrillKind (unknown kind): %v", err)
	} else if found {
		t.Fatal("an unknown kind must not match any drill")
	}
}
