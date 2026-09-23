package schedule

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

// TestScheduledJobSkipsOverlappingRun checks the cron.SkipIfStillRunning guard
// New wraps around every entry: when an entry fires while its previous run is
// still going, the second run returns at once without calling backupFn, so an
// overrunning nightly pass cannot start a second one over the same repo.
// robfig/cron builds the guard once per registered entry, so the test fires
// the same entry's WrappedJob twice instead of waiting for cron ticks.
func TestScheduledJobSkipsOverlappingRun(t *testing.T) {
	var entered int32
	enteredCh := make(chan struct{}) // closed once backupFn is actually entered
	release := make(chan struct{})   // closed by the test to let the blocked run finish

	backupFn := func(name string) error {
		atomic.AddInt32(&entered, 1)
		close(enteredCh)
		<-release
		return nil
	}
	listFn := func() ([]store.Target, error) {
		return []store.Target{{ContainerName: "plex", IncludeInSchedule: true}}, nil
	}

	sc := New(backupFn, listFn)
	s := store.Settings{ContainersEnabled: true, ContainersSchedule: "daily 03:00"}
	if err := sc.ReloadWithDueChecks(s, nil, nil, nil, nil, nil, nil); err != nil {
		t.Fatalf("ReloadWithDueChecks: %v", err)
	}

	var entry *scheduledEntry
	for i := range sc.entries {
		if sc.entries[i].domain == "containers" {
			entry = &sc.entries[i]
		}
	}
	if entry == nil {
		t.Fatal("no containers entry registered")
	}
	wrapped := sc.c.Entry(entry.id).WrappedJob

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		wrapped.Run() // blocks inside backupFn until the test closes release
	}()

	// Inside backupFn the first run holds the entry's token, so the second run
	// below meets a run that is really in flight.
	select {
	case <-enteredCh:
	case <-time.After(5 * time.Second):
		t.Fatal("first run never entered backupFn")
	}

	secondDone := make(chan struct{})
	go func() {
		wrapped.Run()
		close(secondDone)
	}()
	select {
	case <-secondDone:
	case <-time.After(5 * time.Second):
		t.Fatal("second (overlapping) Run() did not return promptly; SkipIfStillRunning did not skip it")
	}

	if got := atomic.LoadInt32(&entered); got != 1 {
		t.Fatalf("backupFn entered %d times while the first run was still in flight, want 1 (skip failed)", got)
	}

	close(release)
	wg.Wait()

	if got := atomic.LoadInt32(&entered); got != 1 {
		t.Fatalf("backupFn entered %d times in total, want exactly 1", got)
	}
}
