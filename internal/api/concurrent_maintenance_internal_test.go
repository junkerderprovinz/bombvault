package api

// Two more guards of the same family as stats_fanout_internal_test.go's, found
// by auditing for the shape rather than by anything failing: a gate that reads
// state its own work writes last, with an expensive restic run behind it.
//
//   - The received-repo integrity check gated on LastCheckAt, whose manual twin
//     (POST /api/receiver/repos/{id}/check) had no gate at all, and whose work is
//     `restic check`, optionally re-reading pack data. Received repos sit outside
//     repoMu, so nothing else serialised them.
//   - The restic cache trim, which rides an after-bulk hook that fires once per
//     per-item cron entry rather than once per night.

import (
	"context"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

// Two callers, one repository: the second is refused, and refused is not a
// verdict. Nothing about the repository's integrity may be inferred from it,
// which is why the guard sits outside receiverCheck rather than inside it.
func TestReceiverCheckRefusesASecondOnTheSameRepo(t *testing.T) {
	svc := &Service{}
	rr := store.ReceivedRepo{ID: "repo-1"}

	if !svc.claimReceiverCheck(rr.ID) {
		t.Fatal("the first claim must succeed")
	}
	if svc.claimReceiverCheck(rr.ID) {
		t.Fatal("a second check on the same repo must be refused while one runs")
	}
	// A different repository is unaffected: the slot is per repo, not global.
	if !svc.claimReceiverCheck("repo-2") {
		t.Fatal("a check on another repo must not be blocked")
	}

	svc.releaseReceiverCheck(rr.ID)
	if !svc.claimReceiverCheck(rr.ID) {
		t.Fatal("the slot must be free again once the check is done")
	}
}

// The refusal reaches the caller as "nothing ran", never as a result. A result
// would be persisted by both call sites and read back as a FAILED integrity
// check, and the scheduled path would alert on it.
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

// Both callers must go through the guarded wrapper. The scheduled sweep alone
// is harmless (it is sequential); the bug was the manual endpoint beside it, and
// a future third caller would be just as invisible.
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
			t.Errorf("%s calls receiverCheck directly — use receiverCheckExclusive", f)
		}
	}
}

// The cache trim admits one caller and turns the rest away. Ten per-item cron
// entries firing in the same minute used to give ten independent measure-and-
// evict passes over one cache directory.
func TestCacheTrimAdmitsOneCaller(t *testing.T) {
	svc := &Service{}
	// No cache dir and no store: a caller that gets past the flag would panic on
	// the settings read, which is exactly the observation this test wants.
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
			svc.TrimResticCache(context.Background()) // must return immediately
		}()
	}
	go func() { wg.Wait(); close(done) }()

	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("a trim that finds one running must return at once, not queue or run")
	}
}
