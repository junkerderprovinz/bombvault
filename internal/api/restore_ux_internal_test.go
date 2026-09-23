package api

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"
)

// A destination refusal is there to name the folder in the way, so scrubError
// keeps its path. Other errors are still scrubbed.
func TestScrubErrorKeepsRestoreDestinationPath(t *testing.T) {
	refusal := destinationRefusal("restore destination %q already contains data — it may belong to a different container; confirm overwrite to proceed", "/mnt/cache/appdata/SnapOtter")
	if !errors.Is(refusal, errRestoreDestination) {
		t.Fatal("destinationRefusal must satisfy errors.Is(err, errRestoreDestination)")
	}
	got := scrubError(refusal)
	if strings.Contains(got, "[path]") {
		t.Fatalf("destination refusal must not be path-scrubbed, got %q", got)
	}
	if !strings.Contains(got, "/mnt/cache/appdata/SnapOtter") {
		t.Fatalf("destination refusal must name the real destination, got %q", got)
	}
	if other := scrubError(errors.New("open /config/bombvault.db: permission denied")); !strings.Contains(other, "[path]") {
		t.Fatalf("ordinary errors must still be path-scrubbed, got %q", other)
	}
}

// A batch backup is refused up front while another operation holds the domain,
// rather than blocking on the lock in a goroutine, and batchActive is released.
func TestStartBackupAllRefusesBusyDomain(t *testing.T) {
	svc := &Service{
		repoMu:         map[string]*sync.Mutex{"containers": {}, "vms": {}, "flash": {}, "config": {}, "files": {}},
		domainActivity: map[string]string{},
	}
	unlock := svc.lockDomainFor("containers", "prune")
	defer unlock()

	started, err := svc.StartBackupAll(context.Background(), []string{"plex"})
	if err == nil || started {
		t.Fatalf("expected StartBackupAll to refuse a busy domain, got started=%v err=%v", started, err)
	}
	if got := err.Error(); !strings.Contains(got, "prune") || !strings.Contains(got, "containers") {
		t.Fatalf("busy error should name the op and domain, got %q", got)
	}
	if svc.batchActive.Load() {
		t.Fatal("batchActive must be cleared after a refused start")
	}
}

// Cancelling a registered run cancels its context; cancelling it again after
// unregister is a no-op that reports false.
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
