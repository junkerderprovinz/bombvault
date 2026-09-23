package api_test

// The "Backup Everything" single-flight guard covers both StartBackupEverything
// and the scheduler's direct call to BackupEverything. cron's
// SkipIfStillRunning only keeps a scheduled pass from overlapping itself, and
// lockDomain blocks rather than failing, so two passes would both complete:
// every domain backed up twice and the post-hook fired twice. The fake engine's
// block channel holds the first pass in flight.

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/api"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// waitForEverythingInFlight blocks until a pass has taken the guard.
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

// everythingParentRuns counts the parent run rows of all passes.
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

// A manual "Run now" during the scheduled pass is refused, which the handler
// answers with 409.
func TestScheduledEverythingPassRefusesAManualOne(t *testing.T) {
	eng := &fakeResticEngine{block: make(chan struct{})}
	svc, st, _, _ := everythingTestService(t, eng)

	// What the SetEverythingJob closure in cmd/bombvault/main.go does.
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

// The nightly trigger during a manual pass gets ErrEverythingInFlight, which
// the scheduled closure treats as a skip.
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
