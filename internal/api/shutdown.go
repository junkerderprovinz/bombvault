package api

import (
	"context"
	"log"
	"time"
)

// shutdownGrace bounds how long BeginShutdown waits for cancelled backups to
// unwind and write their run rows. It is not a wait for the backup itself.
// docker stop sends SIGKILL after 10s by default and Unraid's container update
// uses that default, so waiting longer would be cut short anyway.
const shutdownGrace = 10 * time.Second

// registerBackupCancel records a running backup's cancel func under its
// progress key so shutdown and CancelBackupRun can reach it. Pair it with a
// deferred unregisterBackupCancel.
func (s *Service) registerBackupCancel(key string, cancel context.CancelFunc) {
	s.cancelMu.Lock()
	if s.backupCancels == nil {
		s.backupCancels = map[string]context.CancelFunc{}
	}
	s.backupCancels[key] = cancel
	s.cancelMu.Unlock()
}

// bindBackupRun records the run the backup under key writes, so a cancel aimed
// at that run cannot reach the next backup that registers the same key.
func (s *Service) bindBackupRun(key, runID string) {
	s.cancelMu.Lock()
	if s.backupRuns == nil {
		s.backupRuns = map[string]string{}
	}
	s.backupRuns[key] = runID
	s.cancelMu.Unlock()
}

// unregisterBackupCancel drops a finished backup's entry and its cancellation
// mark, so the next backup under the same key does not report its own failure
// as cancelled.
func (s *Service) unregisterBackupCancel(key string) {
	s.cancelMu.Lock()
	delete(s.backupCancels, key)
	delete(s.backupRuns, key)
	delete(s.cancelledBackups, key)
	s.cancelMu.Unlock()
}

// CancelBackupRun cancels the in-flight backup under a progress key and reports
// whether one was running, so a stale button in another tab is harmless.
// restic writes the snapshot last, so an aborted backup leaves only unreferenced
// data for the next prune. It only looks in backupCancels, so it can never
// reach a restore.
// The mark it sets makes runsAdapter.Finish record the run as cancelled rather
// than failed.
//
// A non-empty runID cancels only while the backup under key still writes that
// run, checked under the same lock as the cancel itself, so a caller that
// looked the run up a moment earlier cannot reach the next backup of the item.
// The web interface's button passes none: it means whatever runs there now.
func (s *Service) CancelBackupRun(key, runID string) bool {
	s.cancelMu.Lock()
	cancel, ok := s.backupCancels[key]
	if runID != "" && s.backupRuns[key] != runID {
		ok = false
	}
	if ok {
		if s.cancelledBackups == nil {
			s.cancelledBackups = map[string]bool{}
		}
		s.cancelledBackups[key] = true
	}
	s.cancelMu.Unlock()
	if ok {
		cancel()
	}
	return ok
}

// backupWasCancelled reports whether the user cancelled the backup running
// under key.
func (s *Service) backupWasCancelled(key string) bool {
	if key == "" {
		return false
	}
	s.cancelMu.Lock()
	defer s.cancelMu.Unlock()
	return s.cancelledBackups[key]
}

// inFlightBackups reports how many backups currently hold a cancel entry.
func (s *Service) inFlightBackups() int {
	s.cancelMu.Lock()
	defer s.cancelMu.Unlock()
	return len(s.backupCancels)
}

// IsShuttingDown reports whether BeginShutdown has run. The run bookkeeping
// uses it to label aborts; it is not a health flag.
func (s *Service) IsShuttingDown() bool { return s.shuttingDown.Load() }

// StopContext returns a context that BeginShutdown cancels.
func (s *Service) StopContext() context.Context {
	s.stopOnce.Do(s.openStopContext)
	return s.stopCtx
}

func (s *Service) openStopContext() {
	s.stopCtx, s.stopCancel = context.WithCancel(context.Background())
}

// BeginShutdown marks the process as leaving, cancels every in-flight backup
// and waits up to shutdownGrace for them to unwind. Backups that fail meanwhile
// are recorded as cancelled, which leaves the startup reaper's "interrupted"
// for stops nobody asked for.
//
// Restores are not cancelled: an interrupted restore has already removed the
// container and half-written its appdata, so it is left to be reaped as
// interrupted. BeginShutdown is safe to call more than once.
func (s *Service) BeginShutdown() {
	s.shuttingDown.Store(true)
	s.stopOnce.Do(s.openStopContext)
	s.stopCancel()

	s.cancelMu.Lock()
	cancels := make([]context.CancelFunc, 0, len(s.backupCancels))
	for _, c := range s.backupCancels {
		cancels = append(cancels, c)
	}
	s.cancelMu.Unlock()

	if len(cancels) == 0 {
		return
	}
	log.Printf("shutdown: cancelling %d in-flight backup(s)", len(cancels))
	for _, c := range cancels {
		c()
	}

	// Each backup's deferred unregister removes its entry after the run row is
	// written, so an empty map means every row is in.
	deadline := time.Now().Add(shutdownGrace)
	for time.Now().Before(deadline) {
		if s.inFlightBackups() == 0 {
			log.Printf("shutdown: all backups unwound cleanly")
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	log.Printf("shutdown: %d backup(s) still unwinding after %s, leaving them to the reaper",
		s.inFlightBackups(), shutdownGrace)
}
