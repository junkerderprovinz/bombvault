package api

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

// newTestStore is an in-memory, migrated store, the same shape
// newAuthGateHandler builds for the auth tests.
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

// Cancelling a running backup (#200, scooterscott1: "Is there a way to cancel
// an in flight backup of folders?").
//
// The machinery for this existed before the feature did: every backup has held
// its own cancel func since [375], reachable only by shutdown. What these tests
// pin is the second door and, more importantly, the bookkeeping behind it. A
// cancelled backup must not read as a failure - a red row in Run History, a
// count on the dashboard and an alert for something somebody asked for on
// purpose is the exact outcome this feature would otherwise buy.

func TestCancelBackupRunCancelsAndReportsIt(t *testing.T) {
	s := &Service{}
	_, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	s.registerBackupCancel("files:abc", func() { close(done); cancel() })

	if !s.CancelBackupRun("files:abc") {
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
	if s.CancelBackupRun("files:gone") {
		t.Fatal("an unknown key must report false, not true")
	}
	if s.backupWasCancelled("files:gone") {
		t.Fatal("an unknown key must not leave a mark behind")
	}
}

func TestUnregisterClearsTheCancellationMark(t *testing.T) {
	// Without this, the NEXT backup under the same key would relabel its own
	// genuine failure as somebody's cancellation.
	s := &Service{}
	s.registerBackupCancel("files:abc", func() {})
	s.CancelBackupRun("files:abc")
	s.unregisterBackupCancel("files:abc")
	if s.backupWasCancelled("files:abc") {
		t.Fatal("the mark outlived the run it belonged to")
	}
}

func TestCancelBackupRunLeavesRestoresAlone(t *testing.T) {
	// The whole reason there are two maps: interrupting a restore is
	// destructive, and a caller must not be able to reach it through the
	// backup door by getting a key prefix wrong.
	s := &Service{}
	reached := false
	s.registerCancel("container:plex", func() { reached = true })

	if s.CancelBackupRun("container:plex") {
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
	s.CancelBackupRun("")
	if s.backupWasCancelled("") {
		t.Fatal("the empty key must never count as cancelled")
	}
}

func TestCancelledBackupRecordsCancelledNotFailed(t *testing.T) {
	st := newTestStore(t)
	s := &Service{store: st}
	s.registerBackupCancel("files:set1", func() {})
	s.CancelBackupRun("files:set1")

	runID, err := st.StartRun("set1", "backup")
	if err != nil {
		t.Fatalf("start run: %v", err)
	}
	// This is what the backup package calls when restic returns the context
	// error that the cancellation caused.
	a := runsAdapter{st: st, ctx: context.Background(), svc: s, cancelKey: "files:set1"}
	if err := a.Finish(runID, "failed", "", 0, "context canceled"); err != nil {
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
	// The relabel is deliberately narrow: only a FAILED run, and only one whose
	// key was actually marked. Break either half and a real failure disappears
	// from the dashboard.
	st := newTestStore(t)
	s := &Service{store: st}

	runID, err := st.StartRun("set2", "backup")
	if err != nil {
		t.Fatalf("start run: %v", err)
	}
	a := runsAdapter{st: st, ctx: context.Background(), svc: s, cancelKey: "files:set2"}
	if err := a.Finish(runID, "failed", "", 0, "repository is locked"); err != nil {
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
	s.CancelBackupRun("files:set3")

	runID, err := st.StartRun("set3", "backup")
	if err != nil {
		t.Fatalf("start run: %v", err)
	}
	a := runsAdapter{st: st, ctx: context.Background(), svc: s, cancelKey: "files:set3"}
	if err := a.Finish(runID, "success", "snap1", 42, ""); err != nil {
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

// TestFilesCancelKeyMatchesTheProgressKey pins the mismatch that #200 uncovered.
//
// Files was the one domain where the cancel entry and the progress stream
// disagreed: "files:<id>" against "files:<name>". Nothing noticed, because the
// only caller was shutdown, which walks the whole map without reading keys. The
// moment a user can press Cancel, the interface has exactly one key in hand -
// the one the progress stream gave it - and a mismatch means a button that
// answers "cancelled: false" forever with nothing anywhere to explain it.
//
// A source scan rather than a behavioural test, deliberately: reaching the real
// registration means running a real restic backup, and the thing worth guarding
// is the LITERAL agreement between two lines that sit 80 lines apart in one
// function and are edited by different people for different reasons.
func TestFilesCancelKeyMatchesTheProgressKey(t *testing.T) {
	raw, err := os.ReadFile("service.go")
	if err != nil {
		t.Fatalf("read service.go: %v", err)
	}
	src := string(raw)

	for _, want := range []string{
		`s.registerBackupCancel("files:"+set.Name, cancel)`,
		`defer s.unregisterBackupCancel("files:" + set.Name)`,
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
	// The old shape, which looked harmless and was not.
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
