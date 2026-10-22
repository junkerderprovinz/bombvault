package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/notify"
	"github.com/junkerderprovinz/bombvault/internal/schedule"
)

func TestTheZFSDueGateCountsTheReplicaOfAnItemThatOnlyReplicates(t *testing.T) {
	r := replicaOnlyRig(t)
	gate := schedule.ZFSDueGate(r.st, zfsReplicaGate(r.s))
	if last, err := gate(); err != nil || !last.IsZero() {
		t.Fatalf("before any run: %v, %v", last, err)
	}
	if _, err := r.s.BackupZFSDataset(context.Background(), r.item.ID); err != nil {
		t.Fatal(err)
	}
	last, err := gate()
	if err != nil {
		t.Fatal(err)
	}
	if time.Since(last) > time.Minute {
		t.Fatalf("the gate saw %v, so an everyN cadence would be due at every trigger", last)
	}
}

func TestTheWatchdogJudgesAZFSDomainThatOnlyReplicatesByItsReplicas(t *testing.T) {
	r := replicaOnlyRig(t)
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)
	if err := r.s.SetNotifyConfig(notify.Config{On: "failure", WebhookEnabled: true, WebhookURL: srv.URL}); err != nil {
		t.Fatal(err)
	}
	if _, err := r.s.BackupZFSDataset(context.Background(), r.item.ID); err != nil {
		t.Fatal(err)
	}
	const day = int64(86400)
	now := time.Now().Unix()
	if err := r.s.runWatchdogAt(context.Background(), now+day/2); err != nil {
		t.Fatal(err)
	}
	if n := atomic.LoadInt32(&hits); n != 0 {
		t.Fatalf("a current replica was reported overdue %d times", n)
	}
	if err := r.s.runWatchdogAt(context.Background(), now+3*day); err != nil {
		t.Fatal(err)
	}
	if n := atomic.LoadInt32(&hits); n != 1 {
		t.Fatalf("a replica three days old was reported %d times, want once", n)
	}
}
