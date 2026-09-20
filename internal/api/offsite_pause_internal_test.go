package api

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"
)

func pausedDefault(t *testing.T, f *placementFixture, domain string) bool {
	t.Helper()
	d, found, err := f.st.PlacementDefaultFor(domain)
	if err != nil {
		t.Fatal(err)
	}
	return found && d.Paused()
}

// waitForListings waits until no target is being listed in the background.
func waitForListings(t *testing.T, f *placementFixture) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		f.svc.listingMu.Lock()
		n := len(f.svc.listing)
		f.svc.listingMu.Unlock()
		if n == 0 {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("a background listing did not finish")
}

func TestAFirstListingThatFindsTheDomainAtTheTargetPauses(t *testing.T) {
	f := newPlacementFixture(t)
	b2 := f.target("containers", "B2", "b2:bucket:containers")
	now := time.Now().Unix()
	f.hold(f.domainPath("containers"), snap("a9", now, "container:nginx"))
	f.hold("b2:bucket:containers", copied("b1", "a1", now-86400, "container:nginx"))
	sent := placementWebhook(t, f)

	for range 2 {
		if err := f.svc.ReplicateOffsite(context.Background(), "containers"); err != nil {
			t.Fatal(err)
		}
	}
	if !pausedDefault(t, f, "containers") {
		t.Fatal("the domain did not pause")
	}
	if len(f.eng.copies) != 0 {
		t.Fatalf("copied %+v to a target holding history this database never wrote", f.eng.copies)
	}
	if _, listed, err := f.st.TargetObservationFor("containers", b2.ID); err != nil || !listed {
		t.Fatalf("the listing that found it was not recorded (listed=%v err=%v)", listed, err)
	}
	if msgs := sent(); len(msgs) != 1 || !strings.Contains(msgs[0], "paused") {
		t.Fatalf("notifications = %q, want one about the pause", msgs)
	}
}

func TestASourceOlderThanTheDatabasePauses(t *testing.T) {
	f := newPlacementFixture(t)
	f.target("containers", "B2", "b2:bucket:containers")
	f.hold(f.domainPath("containers"), snap("a1", 100, "container:nginx"))

	if err := f.svc.ReplicateOffsite(context.Background(), "containers"); err != nil {
		t.Fatal(err)
	}
	if !pausedDefault(t, f, "containers") || len(f.eng.copies) != 0 {
		t.Fatalf("paused=%v copies=%+v, want a pause and no copy", pausedDefault(t, f, "containers"), f.eng.copies)
	}
}

func TestAFailedFirstListingStaysPendingForTheNextPass(t *testing.T) {
	f := newPlacementFixture(t)
	b2 := f.target("containers", "B2", "b2:bucket:containers")
	f.hold(f.domainPath("containers"), snap("a1", time.Now().Unix(), "container:nginx"))
	f.eng.listErr["b2:bucket:containers"] = errors.New("503 service unavailable")

	// B2 is the domain's only target, so a pass that cannot even list it for
	// the first-listing check reaches nothing: that is a failed pass, not a
	// quiet no-op.
	if err := f.svc.ReplicateOffsite(context.Background(), "containers"); err == nil {
		t.Fatal("a pass that could visit no target returned nil, want the failure")
	}
	if len(f.eng.copies) != 0 {
		t.Fatalf("copied %+v to a target whose first listing failed", f.eng.copies)
	}
	if _, listed, err := f.st.TargetObservationFor("containers", b2.ID); err != nil {
		t.Fatal(err)
	} else if listed {
		t.Fatal("a target that could not be listed for its first check must not be recorded as listed")
	}
	if runs := offsiteRuns(t, f, "containers"); len(runs) != 1 || !strings.HasPrefix(runs[0], b2.ID+" ok=0 ") {
		t.Fatalf("runs = %v, want one failed run at B2", runs)
	}
	if status := domainActivityStatus(t, f, "containers"); status != "failed" {
		t.Fatalf("activity status = %q, want failed", status)
	}

	f.eng.listErr = map[string]error{}
	if err := f.svc.ReplicateOffsite(context.Background(), "containers"); err != nil {
		t.Fatal(err)
	}
	if len(f.eng.copies) != 1 {
		t.Fatalf("copies = %+v, want the next pass to try B2 again", f.eng.copies)
	}
}

func TestAPullReceiverPausesOnceAndCopiesAfterTheConfirmation(t *testing.T) {
	f := newPlacementFixture(t)
	f.target("containers", "B2", "b2:bucket:containers")
	// A receiver's domain path holds what restic copy brought in: the original
	// times, older than this database.
	f.hold(f.domainPath("containers"), copied("r1", "x1", 100, "container:nginx"))

	if err := f.svc.ReplicateOffsite(context.Background(), "containers"); err != nil {
		t.Fatal(err)
	}
	if !pausedDefault(t, f, "containers") {
		t.Fatal("the receiver did not pause")
	}
	if err := f.st.ConfirmPlacement("containers", nil); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.ReplicateOffsite(context.Background(), "containers"); err != nil {
		t.Fatal(err)
	}
	if pausedDefault(t, f, "containers") || len(f.eng.copies) != 1 {
		t.Fatalf("after the confirmation: paused=%v copies=%+v, want one copy and no new pause", pausedDefault(t, f, "containers"), f.eng.copies)
	}
}

func TestNewHistoryOnlyDoesNotPause(t *testing.T) {
	f := newPlacementFixture(t)
	f.target("containers", "B2", "b2:bucket:containers")
	f.hold(f.domainPath("containers"), snap("a9", time.Now().Unix(), "container:nginx"))

	if err := f.svc.ReplicateOffsite(context.Background(), "containers"); err != nil {
		t.Fatal(err)
	}
	if pausedDefault(t, f, "containers") || len(f.eng.copies) != 1 {
		t.Fatalf("paused=%v copies=%+v, want a copy and no pause", pausedDefault(t, f, "containers"), f.eng.copies)
	}
}

func TestADomainThatReplicatedBeforeNeverPauses(t *testing.T) {
	f := newPlacementFixture(t)
	f.target("containers", "B2", "b2:bucket:containers")
	f.replicated("containers")
	f.hold(f.domainPath("containers"), snap("a1", 100, "container:nginx"), snap("a2", 200, "container:nginx"))
	f.hold("b2:bucket:containers", copied("b1", "a1", 100, "container:nginx"))

	if err := f.svc.ReplicateOffsite(context.Background(), "containers"); err != nil {
		t.Fatal(err)
	}
	if pausedDefault(t, f, "containers") || len(f.eng.copies) != 1 {
		t.Fatalf("paused=%v copies=%+v, want a copy and no pause", pausedDefault(t, f, "containers"), f.eng.copies)
	}
}

func TestConfirmingWithAnExcludedNameLeavesItOut(t *testing.T) {
	f := newPlacementFixture(t)
	f.target("containers", "B2", "b2:bucket:containers")
	f.hold(f.domainPath("containers"), snap("o1", 100, "container:old-app"), snap("a1", 100, "container:nginx"))
	if err := f.svc.ReplicateOffsite(context.Background(), "containers"); err != nil {
		t.Fatal(err)
	}
	if err := f.st.ConfirmPlacement("containers", []string{"container:old-app"}); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.ReplicateOffsite(context.Background(), "containers"); err != nil {
		t.Fatal(err)
	}
	if len(f.eng.copies) != 1 || !slices.Equal(f.eng.copies[0].IDs, []string{"a1"}) {
		t.Fatalf("copies = %+v, want a1 only", f.eng.copies)
	}
}

func TestTheHookPausesToo(t *testing.T) {
	f := newPlacementFixture(t)
	f.target("containers", "B2", "b2:bucket:containers")
	f.hold("b2:bucket:containers", copied("b1", "a1", 100, "container:nginx"))

	f.svc.replicateOffsite(context.Background(), "containers", settingsOf(t, f.svc), f.domainPath("containers"), "container:nginx")

	if !pausedDefault(t, f, "containers") || len(f.eng.copies) != 0 {
		t.Fatalf("paused=%v copies=%+v, want a pause and no copy", pausedDefault(t, f, "containers"), f.eng.copies)
	}
}

func TestABackgroundListingLooksForHistoryToo(t *testing.T) {
	f := newPlacementFixture(t)
	b2 := f.target("containers", "B2", "b2:bucket:containers")
	f.hold("b2:bucket:containers", copied("b1", "a1", 100, "container:nginx"))

	f.svc.listTargetInBackground("containers", b2.ID)
	waitForListings(t, f)

	if !pausedDefault(t, f, "containers") {
		t.Fatal("the background listing found history and did not pause")
	}
	if _, listed, err := f.st.TargetObservationFor("containers", b2.ID); err != nil || !listed {
		t.Fatalf("the background listing was not recorded (listed=%v err=%v)", listed, err)
	}
}

func TestAConfirmedDomainDoesNotPauseWhenANewTargetIsListedFirst(t *testing.T) {
	f := newPlacementFixture(t)
	if err := f.st.ConfirmPlacement("containers", nil); err != nil {
		t.Fatal(err)
	}
	f.target("containers", "B2", "b2:bucket:containers")
	f.hold(f.domainPath("containers"), snap("a1", 100, "container:nginx"))

	if err := f.svc.ReplicateOffsite(context.Background(), "containers"); err != nil {
		t.Fatal(err)
	}
	if pausedDefault(t, f, "containers") || len(f.eng.copies) != 1 {
		t.Fatalf("paused=%v copies=%+v, want a copy and no pause", pausedDefault(t, f, "containers"), f.eng.copies)
	}
}

func TestAConfirmedDomainDoesNotPauseWhenTheBackgroundListingFindsHistory(t *testing.T) {
	f := newPlacementFixture(t)
	if err := f.st.ConfirmPlacement("containers", nil); err != nil {
		t.Fatal(err)
	}
	b2 := f.target("containers", "B2", "b2:bucket:containers")
	f.hold("b2:bucket:containers", copied("b1", "a1", 100, "container:nginx"))

	f.svc.listTargetInBackground("containers", b2.ID)
	waitForListings(t, f)

	if pausedDefault(t, f, "containers") {
		t.Fatal("a confirmed domain paused again from a background listing")
	}
	if _, listed, err := f.st.TargetObservationFor("containers", b2.ID); err != nil || !listed {
		t.Fatalf("the background listing was not recorded (listed=%v err=%v)", listed, err)
	}
}
