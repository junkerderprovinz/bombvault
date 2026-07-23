package api_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/notify"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
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

// latestRunOfKind returns the newest run of the given kind from the shared runs
// table, failing the test when none was recorded.
func latestRunOfKind(t *testing.T, st *store.Repo, kind string) store.Run {
	t.Helper()
	runs, err := st.ListRuns(50)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range runs {
		if r.Kind == kind {
			return r
		}
	}
	t.Fatalf("no %q run recorded, got %+v", kind, runs)
	return store.Run{}
}

// TestReplicateOffsiteRecordsActivityRun pins the Activity Log feed (G2): a
// replication ALSO lands a kind="offsite" row in the SHARED runs table, on the
// reserved domain target id (the prune/verify pattern) — additive to the
// offsite_runs bookkeeping the scorecard's currency checks rely on, which must
// keep being recorded exactly as before.
func TestReplicateOffsiteRecordsActivityRun(t *testing.T) {
	eng := &fakeResticEngine{}
	svc, st := offsiteReplTestService(t, eng)
	if err := svc.ReplicateOffsite(context.Background(), "flash"); err != nil {
		t.Fatalf("ReplicateOffsite: %v", err)
	}

	run := latestRunOfKind(t, st, "offsite")
	if run.TargetID != store.FlashTargetID {
		t.Fatalf("offsite run target = %q, want %q", run.TargetID, store.FlashTargetID)
	}
	if run.Status != "success" || run.FinishedAt == nil {
		t.Fatalf("a successful replication must record a finished success run, got %+v", run)
	}
	if run.Error != "" {
		t.Fatalf("a successful offsite run must carry no error, got %q", run.Error)
	}

	// The offsite_runs history (currency source) is unchanged and still recorded.
	if _, found, err := st.LatestSuccessfulOffsiteRun("flash"); err != nil || !found {
		t.Fatalf("offsite_runs bookkeeping must still be recorded, found=%v err=%v", found, err)
	}
}

// TestReplicateOffsiteFailureRecordsFailedActivityRun pins the failure side of
// the same feed: a failed copy records the kind="offsite" run as failed with
// the (truncated) error text.
func TestReplicateOffsiteFailureRecordsFailedActivityRun(t *testing.T) {
	eng := &fakeResticEngine{copyErr: errors.New("copy exploded")}
	svc, st := offsiteReplTestService(t, eng)
	if err := svc.ReplicateOffsite(context.Background(), "flash"); err == nil {
		t.Fatal("a failed copy must surface the error")
	}

	run := latestRunOfKind(t, st, "offsite")
	if run.Status != "failed" {
		t.Fatalf("a failed replication must record a failed offsite run, got %+v", run)
	}
	if !strings.Contains(run.Error, "copy exploded") {
		t.Fatalf("the failed offsite run must carry the error text, got %q", run.Error)
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
