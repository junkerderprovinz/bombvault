package schedule

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

// TestScheduledJobSkipsOverlappingRun pins the cron.SkipIfStillRunning guard New
// wires around every registered entry (cron.WithChain(cron.SkipIfStillRunning(...),
// cron.Recover(...))): if a scheduled job's previous invocation is still in
// flight when the same entry fires again, the second invocation must return
// immediately WITHOUT ever entering backupFn a second time — the guard against
// an overrunning nightly run spawning a second, overlapping pass over the same
// repo (#95).
//
// robfig/cron builds the guard fresh per AddFunc call (SkipIfStillRunning's
// token channel is created inside the JobWrapper closure invoked by
// Chain.Then, and Then runs once per registered entry), so the guard is scoped
// PER CRON ENTRY rather than globally across the whole Cron — this test proves
// that scoping by firing the SAME entry's WrappedJob concurrently rather than
// relying on wall-clock cron ticks.
//
// White-box (package schedule): it reaches the unexported entries slice and
// fires the registered entry's WrappedJob directly, the same synchronous path
// TestContainersJobBatchedOffsiteRunsAfterAllBackups uses.
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
	s := store.Settings{ContainersSchedule: "daily 03:00"}
	if err := sc.ReloadWithDueChecks(s, nil, nil, nil, nil, nil); err != nil {
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

	// Wait until the first run has actually entered backupFn — at that point it
	// is holding SkipIfStillRunning's per-entry token, so firing the same entry
	// again is guaranteed to race against a genuinely in-flight run rather than
	// a run that has not started yet.
	select {
	case <-enteredCh:
	case <-time.After(5 * time.Second):
		t.Fatal("first run never entered backupFn")
	}

	// Second concurrent invocation of the SAME entry while the first is still
	// blocked inside backupFn: SkipIfStillRunning must return promptly without
	// invoking backupFn again.
	secondDone := make(chan struct{})
	go func() {
		wrapped.Run()
		close(secondDone)
	}()
	select {
	case <-secondDone:
	case <-time.After(5 * time.Second):
		t.Fatal("second (overlapping) Run() did not return promptly — SkipIfStillRunning did not skip it")
	}

	if got := atomic.LoadInt32(&entered); got != 1 {
		t.Fatalf("backupFn entered %d times while the first run was still in flight, want 1 (skip failed)", got)
	}

	// Release the first run and make sure it completes cleanly.
	close(release)
	wg.Wait()

	if got := atomic.LoadInt32(&entered); got != 1 {
		t.Fatalf("backupFn entered %d times in total, want exactly 1", got)
	}
}
