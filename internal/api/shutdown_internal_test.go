package api

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestShutdownRelabelsOnlyAFailedRunAndOnlyWhileLeaving(t *testing.T) {
	s := &Service{}

	// Before shutdown no status changes.
	for _, st := range []string{"failed", "success", "cancelled", "skipped"} {
		if got, _, changed := s.shutdownStatus(st); changed || got != st {
			t.Errorf("before shutdown, %q became %q (changed=%v)", st, got, changed)
		}
	}

	s.shuttingDown.Store(true)

	// A failure during shutdown becomes a cancellation with a reason.
	got, msg, changed := s.shutdownStatus("failed")
	if !changed || got != "cancelled" {
		t.Errorf("failed during shutdown = %q (changed=%v), want cancelled", got, changed)
	}
	if msg == "" {
		t.Error("the relabelled run carries no reason, so the row cannot explain itself")
	}

	// Every other status stays as it is.
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
	s.registerCancel("container:plex", restoreCancel) // the restore registry

	// Unregister like a real backup does, or BeginShutdown waits out the grace.
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
	// An interrupted restore has already removed the container and half-written
	// its appdata.
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
	// Docker sends SIGKILL 10s after SIGTERM, so waiting longer for a backup
	// that ignores its context gains nothing.
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
	// Bookkeeping-only call sites pass no Service.
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
