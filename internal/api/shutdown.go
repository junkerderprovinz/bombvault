package api

// Stopping on purpose ([375]).
//
// Until this file, BombVault caught no signal at all. `signal.Notify` did not
// appear anywhere in the project, which is easy to miss because the care is
// visible everywhere else: restic gets a SIGTERM and a documented clean-abort
// window (internal/restic/proc_unix.go), a VM gets an ACPI shutdown
// (internal/virshcli). Only the process itself got nothing, so `docker stop`
// killed it outright, mid-backup, with restic's child dying alongside it.
//
// The evidence was in the live data before it was in the code. Of 500 runs
// between 2026-07-24 and 2026-08-31, three had been swept up by
// ReapInterruptedRuns, and two of those started within THIRTY SECONDS of the
// 04:00 schedule on consecutive backup days. That sweep is what writes
// "interrupted (BombVault restarted mid-run)", and its problem is not that it
// is wrong. Its problem is that it cannot tell an update from a crash: it runs
// at startup and infers, so every abrupt ending looks the same afterwards.
//
// A shutdown the process TOOK PART IN needs no inference. It knows it is
// leaving, so it can say so, and then "interrupted" gets its meaning back: it
// means nobody asked, which is the case actually worth investigating.

import (
	"context"
	"log"
	"time"
)

// shutdownGrace bounds how long BeginShutdown waits for in-flight backups to
// notice their cancelled context and unwind.
//
// Ten seconds, matched to the outside rather than to the work: Docker's default
// `docker stop` timeout is 10s and Unraid's container update uses it, so a
// longer wait here would simply be overtaken by SIGKILL and the run would land
// in the reaper anyway. This is not a wait for the BACKUP to finish (that can
// take hours), only for the cancellation to propagate and the run row to be
// written, which is a database write behind a returning restic.
const shutdownGrace = 10 * time.Second

// registerBackupCancel records a running backup's cancel func under its
// progress key so shutdown can reach it. Paired with a deferred
// unregisterBackupCancel, exactly like the restore side.
func (s *Service) registerBackupCancel(key string, cancel context.CancelFunc) {
	s.cancelMu.Lock()
	if s.backupCancels == nil {
		s.backupCancels = map[string]context.CancelFunc{}
	}
	s.backupCancels[key] = cancel
	s.cancelMu.Unlock()
}

// unregisterBackupCancel drops a backup's entry once it has finished, along
// with any user-cancellation mark against the same key: the next backup under
// that key must start from a clean slate, or it would relabel its own genuine
// failure as somebody's cancellation.
func (s *Service) unregisterBackupCancel(key string) {
	s.cancelMu.Lock()
	delete(s.backupCancels, key)
	delete(s.cancelledBackups, key)
	s.cancelMu.Unlock()
}

// CancelBackupRun cancels an in-flight backup by its progress key (#200,
// scooterscott1: "Is there a way to cancel an in flight backup of folders?").
//
// Every running backup has held its own cancel func since [375]; until now the
// only caller was shutdown. This is the same mechanism with a second door, and
// the reason it is safe to open is written out at backupCancels' own
// declaration: restic writes its snapshot as the last act of a run, so an
// aborted backup leaves unreferenced data and no snapshot, which the next prune
// collects. Nothing on the host is touched. That is why backups may be
// cancelled and restores may not, and why this deliberately reaches only
// backupCancels.
//
// Returns false for a key that is not running - already finished, never
// started, or misspelled - so the call is idempotent and a stale button in a
// browser tab cannot produce an error.
//
// The mark is what turns the resulting failure into a "cancelled" run rather
// than a red row nobody asked for; runsAdapter.Finish reads it, and
// unregisterBackupCancel clears it.
func (s *Service) CancelBackupRun(key string) bool {
	s.cancelMu.Lock()
	cancel, ok := s.backupCancels[key]
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
// under this key. Read once, while the run is being finished; the mark itself
// is cleared by unregisterBackupCancel a moment later.
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

// IsShuttingDown reports whether BeginShutdown has run. It exists so the run
// bookkeeping can label an abort correctly; it is not a general "are we healthy"
// flag and nothing should gate new work on it alone.
func (s *Service) IsShuttingDown() bool { return s.shuttingDown.Load() }

// BeginShutdown marks the process as leaving, cancels every in-flight BACKUP,
// and waits up to shutdownGrace for them to unwind. It returns when the last
// one has gone or the grace expires, whichever comes first.
//
// Restores are deliberately NOT cancelled — see backupCancels' comment. A
// restore that is interrupted has already removed the container and half-written
// its appdata, so the least-bad thing an exiting process can do is leave it
// alone and let it be reaped, loudly, as interrupted. That is a true statement
// about a restore nobody should trust.
//
// Safe to call more than once; the second call finds an empty map.
func (s *Service) BeginShutdown() {
	s.shuttingDown.Store(true)

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

	// Poll rather than use a WaitGroup: the cancel entries are removed by the
	// backups' own deferred unregister, so the map emptying IS the signal that
	// every run row has been written. A WaitGroup would need every call site to
	// remember to Add/Done, and the map already exists.
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
