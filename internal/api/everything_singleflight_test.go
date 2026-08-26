package api_test

// ---------------------------------------------------------------------------
// The "Backup Everything" single-flight guard covers BOTH entry points.
//
// The guard used to live only in StartBackupEverything — the HTTP one. The
// SCHEDULED closure (cmd/bombvault/main.go's SetEverythingJob) calls
// BackupEverything directly, so it neither set the flag nor tested it: a manual
// "Run now" during the nightly pass found the flag still false, took it, and
// started a second concurrent pass. Two parent run rows on target_id
// "everything", every domain backed up twice (lockDomain BLOCKS rather than
// failing, so the two interleave and both complete), and the post-hook fired
// twice — the dead-man's-switch reporting the whole server protected twice for
// one nightly window. cron's SkipIfStillRunning only stops a scheduled pass
// overlapping ITSELF.
//
// These drive the two entry points against each other in both directions, with
// the fake engine's block channel holding the first pass genuinely in flight.
// ---------------------------------------------------------------------------

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/api"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// waitForEverythingInFlight blocks until a pass has actually taken the guard, so
// the assertions below are about the guard and not about a race with goroutine
// scheduling.
func waitForEverythingInFlight(t *testing.T, svc *api.Service) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if svc.EverythingInProgress() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("timed out waiting for the Backup Everything pass to take the guard")
}

// everythingParentRuns counts the parent run rows one or more passes opened.
// Two of them is the observable damage: one nightly window, two whole-server
// passes.
func everythingParentRuns(t *testing.T, st *store.Repo) int {
	t.Helper()
	runs, err := st.ListRuns(50)
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, r := range runs {
		if r.TargetID == store.EverythingTargetID {
			n++
		}
	}
	return n
}

// TestScheduledEverythingPassRefusesAManualOne is the regression. The scheduled
// closure's own call is in flight; pressing the badge must be refused with the
// 409 the handler documents, not started alongside it.
func TestScheduledEverythingPassRefusesAManualOne(t *testing.T) {
	eng := &fakeResticEngine{block: make(chan struct{})}
	svc, st, _, _ := everythingTestService(t, eng)

	// Exactly what cmd/bombvault/main.go's SetEverythingJob closure does.
	done := make(chan error, 1)
	go func() {
		_, err := svc.BackupEverything(context.Background())
		done <- err
	}()
	waitForEverythingInFlight(t, svc)

	started, err := svc.StartBackupEverything(context.Background())
	if err != nil {
		t.Fatalf("StartBackupEverything: %v", err)
	}
	if started {
		t.Fatal("a manual \"Run now\" during a SCHEDULED pass must be refused (the 409 the handler documents), " +
			"not started as a second concurrent whole-server pass")
	}

	close(eng.block)
	if err := <-done; err != nil {
		t.Fatalf("the scheduled pass itself must complete: %v", err)
	}
	waitForEverythingDone(t, svc)

	if n := everythingParentRuns(t, st); n != 1 {
		t.Fatalf("one nightly window must open exactly one parent run row, got %d", n)
	}
}

// TestManualEverythingPassRefusesTheScheduledOne is the same guard from the
// other side: the nightly trigger arriving on top of a manual pass is refused
// with ErrEverythingInFlight, which the scheduled closure reads as a skip.
func TestManualEverythingPassRefusesTheScheduledOne(t *testing.T) {
	eng := &fakeResticEngine{block: make(chan struct{})}
	svc, st, _, _ := everythingTestService(t, eng)

	started, err := svc.StartBackupEverything(context.Background())
	if err != nil || !started {
		t.Fatalf("the manual pass should start: started=%v err=%v", started, err)
	}
	waitForEverythingInFlight(t, svc)

	if _, err := svc.BackupEverything(context.Background()); !errors.Is(err, api.ErrEverythingInFlight) {
		t.Fatalf("the scheduled pass must be refused while a manual one is in flight, got err=%v", err)
	}

	close(eng.block)
	waitForEverythingDone(t, svc)

	if n := everythingParentRuns(t, st); n != 1 {
		t.Fatalf("a refused pass must not open a parent run row: got %d, want 1", n)
	}
}

// TestEverythingGuardIsReleasedForTheNextPass pins that owning the guard inside
// BackupEverything does not leave it stuck: the next pass must be able to run.
func TestEverythingGuardIsReleasedForTheNextPass(t *testing.T) {
	svc, st, _, _ := everythingTestService(t, &fakeResticEngine{})

	for i := range 2 {
		if _, err := svc.BackupEverything(context.Background()); err != nil {
			t.Fatalf("pass %d: %v", i+1, err)
		}
		if svc.EverythingInProgress() {
			t.Fatalf("pass %d: the guard must be released once the pass returns", i+1)
		}
	}
	if n := everythingParentRuns(t, st); n != 2 {
		t.Fatalf("two sequential passes must open two parent run rows, got %d", n)
	}
}
