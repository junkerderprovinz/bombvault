package api_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/notify"
	"github.com/junkerderprovinz/bombvault/internal/restic"
)

// webhookCounter is an httptest server that counts the notifications posted to it,
// so a test can assert a channel actually fired without a real endpoint.
func webhookCounter(t *testing.T) (url string, hits *int32) {
	t.Helper()
	var n int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&n, 1)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)
	return srv.URL, &n
}

// TestScheduledReplicateOffsiteNotifiesOnFailure pins H3(c): a scheduled off-site
// replication that FAILS must fire a notification (not just log), so a silently
// rotting off-site copy is surfaced. The interactive ReplicateOffsite stays
// notify-free (the UI surfaces its error directly).
func TestScheduledReplicateOffsiteNotifiesOnFailure(t *testing.T) {
	url, hits := webhookCounter(t)
	eng := &fakeResticEngine{copyErr: errors.New("copy exploded")}
	svc, _ := offsiteReplTestService(t, eng)
	if err := svc.SetNotifyConfig(notify.Config{On: "failure", WebhookURL: url}); err != nil {
		t.Fatal(err)
	}

	if err := svc.ScheduledReplicateOffsite(context.Background(), "flash"); err == nil {
		t.Fatal("a failed scheduled replication must surface the error")
	}
	if atomic.LoadInt32(hits) == 0 {
		t.Fatal("a failed scheduled replication must NOTIFY, got no notification")
	}
}

// TestReplicateOffsiteFirstOverBudgetAlarms pins M4: the FIRST replication that
// exceeds the growth budget must alarm — the budget is evaluated against a FRESH
// size sampled for this replication (no prior sample to lag behind), which for an
// immutable repo (no far-side prune) is the only growth backstop.
func TestReplicateOffsiteFirstOverBudgetAlarms(t *testing.T) {
	url, hits := webhookCounter(t)
	eng := &fakeResticEngine{
		snaps:        []restic.Snapshot{{ID: "aaaa1111bbbb2222"}}, // non-empty so the size sample lands
		rawSizeBytes: 2 * 1024 * 1024 * 1024,                      // 2 GiB
	}
	svc, st := offsiteReplTestService(t, eng)
	s, err := st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	s.OffsiteGrowthBudgetGB = 1 // 2 GiB > 1 GiB budget
	if err := st.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}
	if err := svc.SetNotifyConfig(notify.Config{On: "failure", WebhookURL: url}); err != nil {
		t.Fatal(err)
	}

	if err := svc.ReplicateOffsite(context.Background(), "flash"); err != nil {
		t.Fatalf("ReplicateOffsite: %v", err)
	}
	if atomic.LoadInt32(hits) == 0 {
		t.Fatal("the FIRST over-budget replication must alarm (fresh size, no prior sample), got none")
	}
}

// TestReplicateOffsitePanicRecordsFailure pins L5: a panic during the copy must
// NOT stamp a phantom successful replication — the deferred FinishOffsiteRun
// records ok=false because the local success flag was never set.
func TestReplicateOffsitePanicRecordsFailure(t *testing.T) {
	eng := &fakeResticEngine{copyPanic: true}
	svc, st := offsiteReplTestService(t, eng)

	func() {
		defer func() { _ = recover() }() // swallow the propagating panic
		_ = svc.ReplicateOffsite(context.Background(), "flash")
	}()

	run, found, err := st.LatestOffsiteRun("flash")
	if err != nil || !found {
		t.Fatalf("a panic during copy must still record the run, found=%v err=%v", found, err)
	}
	if run.OK {
		t.Fatalf("a panic during copy must record ok=false, not a phantom success, got %+v", run)
	}
	if run.FinishedAt == 0 {
		t.Fatalf("the run must be closed (finish stamped) on the unwind, got %+v", run)
	}
}
