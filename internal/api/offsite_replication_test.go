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

// webhookCounter starts a server that counts the notifications posted to it.
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

// Nobody watches a scheduled replication, so a failure has to notify.
// ReplicateOffsite does not, because the UI shows its error.
func TestScheduledReplicateOffsiteNotifiesOnFailure(t *testing.T) {
	url, hits := webhookCounter(t)
	eng := &fakeResticEngine{copyErr: errors.New("copy exploded")}
	svc, _ := offsiteReplTestService(t, eng)
	if err := svc.SetNotifyConfig(notify.Config{On: "failure", WebhookEnabled: true, WebhookURL: url}); err != nil {
		t.Fatal(err)
	}

	if err := svc.ScheduledReplicateOffsite(context.Background(), "flash"); err == nil {
		t.Fatal("a failed scheduled replication must surface the error")
	}
	if atomic.LoadInt32(hits) == 0 {
		t.Fatal("a failed scheduled replication must NOTIFY, got no notification")
	}
}

// The budget is checked against a size sampled for this replication, so even
// the first one over it alarms. For an append-only repo, where nothing prunes
// the far side, this is the only check on growth.
func TestReplicateOffsiteFirstOverBudgetAlarms(t *testing.T) {
	url, hits := webhookCounter(t)
	eng := &fakeResticEngine{
		snaps:        []restic.Snapshot{{ID: "aaaa1111bbbb2222"}}, // non-empty so the size sample lands
		rawSizeBytes: 2 * 1024 * 1024 * 1024,
	}
	svc, st := offsiteReplTestService(t, eng)
	s, err := st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	s.OffsiteGrowthBudgetGB = 1
	if err := st.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}
	if err := svc.SetNotifyConfig(notify.Config{On: "failure", WebhookEnabled: true, WebhookURL: url}); err != nil {
		t.Fatal(err)
	}

	if err := svc.ReplicateOffsite(context.Background(), "flash"); err != nil {
		t.Fatalf("ReplicateOffsite: %v", err)
	}
	if atomic.LoadInt32(hits) == 0 {
		t.Fatal("the FIRST over-budget replication must alarm (fresh size, no prior sample), got none")
	}
}

// latestRunOfKind returns the newest run of the given kind and fails the test
// when there is none.
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

// A replication shows up in the activity log as an "offsite" run on the
// domain's reserved target id, like prune and verify. The offsite_runs rows
// the scorecard reads are recorded as well.
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

	if _, found, err := st.LatestSuccessfulOffsiteRun("flash"); err != nil || !found {
		t.Fatalf("offsite_runs bookkeeping must still be recorded, found=%v err=%v", found, err)
	}
}

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

// A panic during the copy must close the run as failed, not as a success.
func TestReplicateOffsitePanicRecordsFailure(t *testing.T) {
	eng := &fakeResticEngine{copyPanic: true}
	svc, st := offsiteReplTestService(t, eng)

	func() {
		defer func() { _ = recover() }()
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
