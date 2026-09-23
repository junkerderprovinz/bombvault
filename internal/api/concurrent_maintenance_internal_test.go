package api

import (
	"context"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

// TestReceiverCheckRefusesASecondOnTheSameRepo: a second check on a received
// repository is refused while one runs. Received repos sit outside repoMu, so
// nothing else serialises the scheduled and the manual check. The guard sits
// outside receiverCheck because a refusal says nothing about the repository's
// integrity.
func TestReceiverCheckRefusesASecondOnTheSameRepo(t *testing.T) {
	svc := &Service{}
	rr := store.ReceivedRepo{ID: "repo-1"}

	if !svc.claimReceiverCheck(rr.ID) {
		t.Fatal("the first claim must succeed")
	}
	if svc.claimReceiverCheck(rr.ID) {
		t.Fatal("a second check on the same repo must be refused while one runs")
	}
	// The slot is per repository.
	if !svc.claimReceiverCheck("repo-2") {
		t.Fatal("a check on another repo must not be blocked")
	}

	svc.releaseReceiverCheck(rr.ID)
	if !svc.claimReceiverCheck(rr.ID) {
		t.Fatal("the slot must be free again once the check is done")
	}
}

// TestReceiverCheckRefusalIsNotAVerdict: a refusal reaches the caller as
// "nothing ran". A result would be stored as a failed integrity check, and the
// scheduled path would alert on it.
func TestReceiverCheckRefusalIsNotAVerdict(t *testing.T) {
	svc := &Service{}
	rr := store.ReceivedRepo{ID: "repo-1"}
	if !svc.claimReceiverCheck(rr.ID) {
		t.Fatal("setup: claim")
	}
	defer svc.releaseReceiverCheck(rr.ID)

	res, ran := svc.receiverCheckExclusive(context.Background(), rr, false)
	if ran {
		t.Fatal("receiverCheckExclusive must report that it ran nothing")
	}
	if res.At != 0 || res.Error != "" || res.OK {
		t.Fatalf("a refusal must carry no verdict at all, got %+v", res)
	}
}

// TestBothReceiverCheckCallersAreGuarded: both callers must use the guarded
// wrapper. The scheduled sweep is sequential on its own; the manual endpoint
// runs beside it.
func TestBothReceiverCheckCallersAreGuarded(t *testing.T) {
	for _, f := range []string{"receiver_watch.go", "receiver_handlers.go"} {
		src, err := os.ReadFile(f) //nolint:gosec // G304: fixed file name in this package's own directory
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(src), "receiverCheckExclusive(") {
			t.Errorf("%s runs a received-repo check without the in-flight guard", f)
		}
		if strings.Contains(string(src), ".receiverCheck(") {
			t.Errorf("%s calls receiverCheck directly; use receiverCheckExclusive", f)
		}
	}
}

// TestCacheTrimAdmitsOneCaller: the cache trim admits one caller and turns the
// rest away, since per-item cron entries can start it several times in the same
// minute.
func TestCacheTrimAdmitsOneCaller(t *testing.T) {
	// The Service has no engine or store, so a caller that got past the flag
	// would panic.
	svc := &Service{}
	if !svc.cacheTrimming.CompareAndSwap(false, true) {
		t.Fatal("setup: the flag must start clear")
	}
	defer svc.cacheTrimming.Store(false)

	done := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			svc.TrimResticCache(context.Background())
		}()
	}
	go func() { wg.Wait(); close(done) }()

	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("a trim that finds one running must return at once, not queue or run")
	}
}
