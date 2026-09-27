package api

import (
	"context"
	"strings"
	"sync"
	"testing"
)

// A VM batch is refused up front while another operation holds the vms domain,
// rather than blocking on the lock in a goroutine, and batchActive is released.
func TestStartBackupVMsAllDomainBusy(t *testing.T) {
	svc := &Service{
		repoMu:         map[string]*sync.Mutex{"containers": {}, "vms": {}, "flash": {}, "config": {}, "files": {}},
		domainActivity: map[string]string{},
	}
	unlock := svc.lockDomainFor("vms", "restore")
	defer unlock()

	started, err := svc.StartBackupVMsAll(context.Background(), []string{"alpha"})
	if err == nil || started {
		t.Fatalf("expected StartBackupVMsAll to refuse a busy domain, got started=%v err=%v", started, err)
	}
	if got := err.Error(); !strings.Contains(got, "restore") || !strings.Contains(got, "vms") {
		t.Fatalf("busy error should name the op and domain, got %q", got)
	}
	if svc.batchActive.Load() {
		t.Fatal("batchActive must be cleared after a refused start")
	}
}
