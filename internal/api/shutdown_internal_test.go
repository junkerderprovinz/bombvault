package api

// What a clean stop has to get right ([375]).
//
// Every one of these fails against the code before it: there was no shutdown
// path at all, so a backup interrupted by `docker stop` left its row 'running'
// for the next boot's reaper to call "interrupted", and a restore had no
// protection from being cancelled because nothing was cancelling anything.

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestShutdownRelabelsOnlyAFailedRunAndOnlyWhileLeaving(t *testing.T) {
	s := &Service{}

	// Business as usual: nothing is touched, whatever the status.
	for _, st := range []string{"failed", "success", "cancelled", "skipped"} {
		if got, _, changed := s.shutdownStatus(st); changed || got != st {
			t.Errorf("before shutdown, %q became %q (changed=%v)", st, got, changed)
		}
	}

	s.shuttingDown.Store(true)

	// A failure during shutdown is an abort, and says so.
	got, msg, changed := s.shutdownStatus("failed")
	if !changed || got != "cancelled" {
		t.Errorf("failed during shutdown = %q (changed=%v), want cancelled", got, changed)
	}
	if msg == "" {
		t.Error("the relabelled run carries no reason, so the row cannot explain itself")
	}

	// Everything else keeps its meaning. A run that SUCCEEDED while the server
	// was on its way out still succeeded.
	for _, st := range []string{"success", "cancelled", "skipped"} {
		if out, _, ch := s.shutdownStatus(st); ch || out != st {
			t.Errorf("during shutdown, %q became %q (changed=%v)", st, out, ch)
		}
	}
}

func TestBeginShutdownCancelsBackupsAndSparesRestores(t *testing.T) {
	s := &Service{}

	backupCtx, backupCancel := context.WithCancel(context.Background())
	restoreCtx, restoreCancel := context.WithCancel(context.Background())
	defer restoreCancel()

	s.registerBackupCancel("container:plex", backupCancel)
	s.registerCancel("container:plex", restoreCancel) // the RESTORE registry

	// A real backup unwinds and unregisters; without that BeginShutdown would
	// (correctly) wait out its grace, which would make this test slow rather
	// than wrong.
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		<-backupCtx.Done()
		s.unregisterBackupCancel("container:plex")
	}()

	start := time.Now()
	s.BeginShutdown()
	wg.Wait()

	if backupCtx.Err() == nil {
		t.Error("the backup was not cancelled, so it dies with the process instead of unwinding")
	}
	// The point of two maps: a restore that is interrupted has already removed
	// the container and half-written its appdata.
	if restoreCtx.Err() != nil {
		t.Error("the restore was cancelled - that is destructive and must never happen on shutdown")
	}
	if !s.IsShuttingDown() {
		t.Error("IsShuttingDown is false after BeginShutdown")
	}
	if took := time.Since(start); took > shutdownGrace {
		t.Errorf("BeginShutdown waited %s despite the backup unwinding, cap is %s", took, shutdownGrace)
	}
}

func TestBeginShutdownGivesUpRatherThanHanging(t *testing.T) {
	// A backup that never notices its context must not hold the process open
	// past the grace: Docker sends SIGKILL 10s after SIGTERM, so an unbounded
	// wait here just means being killed mid-wait with nothing written.
	s := &Service{}
	_, cancel := context.WithCancel(context.Background())
	defer cancel()
	s.registerBackupCancel("container:wedged", cancel) // never unregistered

	start := time.Now()
	s.BeginShutdown()
	took := time.Since(start)

	if took < shutdownGrace {
		t.Errorf("gave up after %s, before the %s grace", took, shutdownGrace)
	}
	if took > shutdownGrace+2*time.Second {
		t.Errorf("waited %s, well past the %s grace", took, shutdownGrace)
	}
}

func TestBeginShutdownIsSafeTwiceAndWithNothingRunning(t *testing.T) {
	s := &Service{}
	s.BeginShutdown()
	s.BeginShutdown() // must not panic on the nil/empty map
	if !s.IsShuttingDown() {
		t.Error("IsShuttingDown is false")
	}
}

func TestRunsAdapterWithoutAServiceStillWorks(t *testing.T) {
	// The bookkeeping-only call sites pass no Service. That must stay a
	// no-relabel rather than a nil dereference, or a shutdown during an
	// unrelated update crashes the process it was trying to stop cleanly.
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("runsAdapter with a nil svc panicked: %v", r)
		}
	}()
	var r runsAdapter // zero value: st nil, svc nil
	if r.svc != nil {
		t.Fatal("zero-value runsAdapter unexpectedly carries a Service")
	}
}
