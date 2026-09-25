package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/backup"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// newTestStore returns a migrated in-memory store.
func newTestStore(t *testing.T) *store.Repo {
	t.Helper()
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatalf("open mem store: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := store.Migrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return store.New(db)
}

func TestCancelBackupRunCancelsAndReportsIt(t *testing.T) {
	s := &Service{}
	_, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	s.registerBackupCancel("files:abc", func() { close(done); cancel() })

	if !s.CancelBackupRun("files:abc", "") {
		t.Fatal("CancelBackupRun on a running key must report true")
	}
	select {
	case <-done:
	default:
		t.Fatal("the registered cancel func was never called")
	}
	if !s.backupWasCancelled("files:abc") {
		t.Fatal("the key must be marked, or the run records itself as failed")
	}
}

func TestCancelBackupRunIsIdempotentOnAnUnknownKey(t *testing.T) {
	s := &Service{}
	// A browser tab still showing the button for a backup that finished a
	// second ago must not produce an error.
	if s.CancelBackupRun("files:gone", "") {
		t.Fatal("an unknown key must report false, not true")
	}
	if s.backupWasCancelled("files:gone") {
		t.Fatal("an unknown key must not leave a mark behind")
	}
}

func TestCancelBackupRunRefusesABackupThatWroteItsRestorePoint(t *testing.T) {
	s := &Service{}
	cancelled := false
	s.registerBackupCancel("container:plex", func() {
		cancelled = true
		s.unregisterBackupCancel("container:plex")
	})
	s.bindBackupRun("container:plex", "run-1")
	s.commitBackup("container:plex")

	if s.CancelBackupRun("container:plex", "run-1") || s.CancelBackupRun("container:plex", "") {
		t.Fatal("a backup that wrote its restore point was reported as cancelled")
	}
	if cancelled || s.backupWasCancelled("container:plex") {
		t.Fatal("the cancel reached a backup that only starts its containers again")
	}
	if !s.BackupCommitted("container:plex", "run-1") {
		t.Fatal("the committed run does not say so")
	}
	if s.BackupCommitted("container:plex", "run-0") {
		t.Fatal("another run of the same item reads as committed")
	}

	s.BeginShutdown()
	if !cancelled {
		t.Fatal("shutdown has to reach a committed backup too")
	}
	if s.BackupCommitted("container:plex", "") {
		t.Fatal("the commit outlived the run it belonged to")
	}
}

func TestUnregisterClearsTheCancellationMark(t *testing.T) {
	// Otherwise the next backup under the same key would report its own
	// failure as a cancellation.
	s := &Service{}
	s.registerBackupCancel("files:abc", func() {})
	s.CancelBackupRun("files:abc", "")
	s.unregisterBackupCancel("files:abc")
	if s.backupWasCancelled("files:abc") {
		t.Fatal("the mark outlived the run it belonged to")
	}
}

func TestOnlyACancelledBackupEndsAsCancelled(t *testing.T) {
	s := &Service{}
	s.registerBackupCancel("files:abc", func() {})
	err := errors.New("repository is locked")
	s.endBackupCancel("files:abc", &err)
	if backupEnding(err) != "failed" || err.Error() != "repository is locked" {
		t.Fatalf("an uncancelled failure ended as %s: %v", backupEnding(err), err)
	}

	s.registerBackupCancel("files:abc", func() {})
	s.CancelBackupRun("files:abc", "")
	err = fmt.Errorf("backup: %w", context.Canceled)
	s.endBackupCancel("files:abc", &err)
	if backupEnding(err) != "cancelled" || !errors.Is(err, context.Canceled) {
		t.Fatalf("a cancelled backup ended as %s: %v", backupEnding(err), err)
	}
	if s.backupWasCancelled("files:abc") {
		t.Fatal("the mark outlived the run it belonged to")
	}
}

// The web button has to tell a stale tab from a backup that is only starting
// its containers again, and the endpoint is where it learns which one it was.
func TestBackupCancelEndpointSaysWhyNothingWasCancelled(t *testing.T) {
	s := &Service{}
	s.registerBackupCancel("files:docs", func() {})
	s.registerBackupCancel("container:plex", func() {})
	s.commitBackup("container:plex")
	h := &Handler{svc: s}

	for key, want := range map[string]struct {
		cancelled bool
		reason    string
	}{
		"files:docs":     {cancelled: true},
		"container:plex": {reason: "committed"},
		"files:gone":     {reason: "not_running"},
	} {
		req := httptest.NewRequest(http.MethodPost, "/api/backup/cancel", strings.NewReader(`{"key":"`+key+`"}`))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		h.handleBackupCancel(w, req)

		var got struct {
			Cancelled bool   `json:"cancelled"`
			Reason    string `json:"reason"`
		}
		if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
			t.Fatalf("%s: decode: %v", key, err)
		}
		if got.Cancelled != want.cancelled || got.Reason != want.reason {
			t.Errorf("%s: cancelled %v, reason %q; want %v, %q", key, got.Cancelled, got.Reason, want.cancelled, want.reason)
		}
	}
}

func TestCancelBackupRunLeavesRestoresAlone(t *testing.T) {
	// Restores have a map of their own because interrupting one is
	// destructive; a wrong key prefix must not reach it through here.
	s := &Service{}
	reached := false
	s.registerCancel("container:plex", func() { reached = true })

	if s.CancelBackupRun("container:plex", "") {
		t.Fatal("CancelBackupRun must not find a RESTORE's cancel entry")
	}
	if reached {
		t.Fatal("CancelBackupRun called a restore's cancel func")
	}
}

func TestBackupWasCancelledIgnoresTheEmptyKey(t *testing.T) {
	// Every runsAdapter that is not a backup's own run carries an empty key.
	// If the empty key could ever match, those runs would start relabelling
	// themselves the moment any backup anywhere was cancelled.
	s := &Service{}
	s.registerBackupCancel("", func() {})
	s.CancelBackupRun("", "")
	if s.backupWasCancelled("") {
		t.Fatal("the empty key must never count as cancelled")
	}
}

// TestCancelledBackupRecordsCancelledNotFailed: a backup the user cancelled
// must not show up as a failure, with a red row, a dashboard count and an
// alert.
func TestCancelledBackupRecordsCancelledNotFailed(t *testing.T) {
	st := newTestStore(t)
	s := &Service{store: st}
	s.registerBackupCancel("files:set1", func() {})
	s.CancelBackupRun("files:set1", "")

	runID, err := st.StartRun("set1", "backup")
	if err != nil {
		t.Fatalf("start run: %v", err)
	}
	// This is what the backup package calls when restic returns the context
	// error that the cancellation caused.
	a := runsAdapter{st: st, ctx: context.Background(), svc: s, cancelKey: "files:set1"}
	if err := a.Finish(runID, "failed", backup.Summary{}, "context canceled"); err != nil {
		t.Fatalf("finish run: %v", err)
	}

	runs, err := st.ListRuns(10)
	if err != nil {
		t.Fatalf("list runs: %v", err)
	}
	if len(runs) == 0 {
		t.Fatal("no run recorded")
	}
	got := runs[0]
	if got.Status != "cancelled" {
		t.Fatalf("a cancelled backup recorded status %q, want \"cancelled\" - a red row for something the user asked for", got.Status)
	}
	if got.Error != store.ReasonCancelled {
		t.Fatalf("reason = %q, want %q", got.Error, store.ReasonCancelled)
	}
}

func TestAGenuineFailureKeepsItsStatus(t *testing.T) {
	// Only a failed run whose key was marked is relabelled. Anything wider
	// hides real failures from the dashboard.
	st := newTestStore(t)
	s := &Service{store: st}

	runID, err := st.StartRun("set2", "backup")
	if err != nil {
		t.Fatalf("start run: %v", err)
	}
	a := runsAdapter{st: st, ctx: context.Background(), svc: s, cancelKey: "files:set2"}
	if err := a.Finish(runID, "failed", backup.Summary{}, "repository is locked"); err != nil {
		t.Fatalf("finish run: %v", err)
	}

	runs, err := st.ListRuns(10)
	if err != nil {
		t.Fatalf("list runs: %v", err)
	}
	if runs[0].Status != "failed" {
		t.Fatalf("an uncancelled failure recorded %q, want \"failed\"", runs[0].Status)
	}
	if runs[0].Error != "repository is locked" {
		t.Fatalf("the real error was replaced: %q", runs[0].Error)
	}
}

func TestASuccessfulBackupIsNeverRelabelled(t *testing.T) {
	// A cancellation that lands after restic already wrote its snapshot must
	// not turn a finished backup into a cancelled one.
	st := newTestStore(t)
	s := &Service{store: st}
	s.registerBackupCancel("files:set3", func() {})
	s.CancelBackupRun("files:set3", "")

	runID, err := st.StartRun("set3", "backup")
	if err != nil {
		t.Fatalf("start run: %v", err)
	}
	a := runsAdapter{st: st, ctx: context.Background(), svc: s, cancelKey: "files:set3"}
	if err := a.Finish(runID, "success", backup.Summary{SnapshotID: "snap1", Bytes: 42}, ""); err != nil {
		t.Fatalf("finish run: %v", err)
	}

	runs, err := st.ListRuns(10)
	if err != nil {
		t.Fatalf("list runs: %v", err)
	}
	if runs[0].Status != "success" {
		t.Fatalf("a finished backup recorded %q, want \"success\"", runs[0].Status)
	}
}

// TestFilesCancelKeyMatchesTheProgressKey: the Cancel button can only send the
// key the progress stream published, so BackupFileSet has to register its
// cancel func under the same "files:" + set.Name. It scans the source because
// reaching the real registration takes a real restic backup.
func TestFilesCancelKeyMatchesTheProgressKey(t *testing.T) {
	raw, err := os.ReadFile("service.go")
	if err != nil {
		t.Fatalf("read service.go: %v", err)
	}
	src := string(raw)

	for _, want := range []string{
		`s.registerBackupCancel("files:"+set.Name, cancel)`,
		`defer s.endBackupCancel("files:"+set.Name, &retErr)`,
		`key := "files:" + set.Name`,
		`cancelKey: "files:" + set.Name`,
	} {
		if !strings.Contains(src, want) {
			t.Errorf("BackupFileSet no longer contains %s\n"+
				"All four must key off the SAME expression, or the Cancel button in the\n"+
				"interface silently does nothing: it can only send the key the progress\n"+
				"stream published.", want)
		}
	}
	// Keyed by the set id, which the progress stream never uses.
	for _, forbidden := range []string{
		`s.registerBackupCancel("files:"+id`,
		`cancelKey: "files:" + id`,
	} {
		if strings.Contains(src, forbidden) {
			t.Errorf("BackupFileSet is back to keying the cancel entry off the set id (%s).\n"+
				"The progress stream publishes under the set NAME, so the two would never meet.", forbidden)
		}
	}
}
