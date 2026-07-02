package api

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestStartBackupAllRefusesBusyDomain pins the per-domain activity tracker: when
// a maintenance/scheduler op already holds the containers domain (recorded via
// lockDomainFor), a UI-initiated batch backup must be refused up front with a
// clear busy error naming the op and the domain — instead of launching a
// goroutine that then blocks silently on the domain lock. The shared batchActive
// single-flight guard must be released so a later attempt can still run.
func TestStartBackupAllRefusesBusyDomain(t *testing.T) {
	svc := &Service{
		repoMu:         map[string]*sync.Mutex{"containers": {}, "vms": {}, "flash": {}},
		domainActivity: map[string]string{},
	}
	// Simulate a scheduler/maintenance op holding the containers domain.
	unlock := svc.lockDomainFor("containers", "prune")
	defer unlock()

	started, err := svc.StartBackupAll(context.Background(), []string{"plex"})
	if err == nil || started {
		t.Fatalf("expected StartBackupAll to refuse a busy domain, got started=%v err=%v", started, err)
	}
	if got := err.Error(); !strings.Contains(got, "prune") || !strings.Contains(got, "containers") {
		t.Fatalf("busy error should name the op and domain, got %q", got)
	}
	// batchActive must be released so a later attempt can run.
	if svc.batchActive.Load() {
		t.Fatal("batchActive must be cleared after a refused start")
	}
}

// TestCancelRunLifecycle pins the cancel registry: a registered progress key can
// be cancelled (its context is cancelled and CancelRun reports true), and after
// unregister a cancel of the same key is an idempotent no-op reporting false —
// so cancelling an already-finished/unknown restore is harmless.
func TestCancelRunLifecycle(t *testing.T) {
	svc := &Service{runCancels: map[string]context.CancelFunc{}}
	ctx, cancel := context.WithCancel(context.Background())
	svc.registerCancel("container:plex", cancel)
	if !svc.CancelRun("container:plex") {
		t.Fatal("CancelRun should report true for a registered key")
	}
	select {
	case <-ctx.Done():
	case <-time.After(time.Second):
		t.Fatal("CancelRun must cancel the registered context")
	}
	svc.unregisterCancel("container:plex")
	if svc.CancelRun("container:plex") {
		t.Fatal("CancelRun should report false for an unknown/finished key (idempotent no-op)")
	}
}
