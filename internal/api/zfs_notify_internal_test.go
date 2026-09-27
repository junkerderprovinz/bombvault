package api

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/notify"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// zfsSummaryNotifications points the service at a local channel and turns the
// scheduled summary on, the setting that replaces a run's per-item messages
// with one line.
func zfsSummaryNotifications(t *testing.T, s *Service) func() []string {
	t.Helper()
	bodies := zfsCaptureNotifications(t, s)
	c, err := s.NotifyConfig()
	if err != nil {
		t.Fatalf("read notify config: %v", err)
	}
	c.ScheduledSummary = true
	if err := s.SetNotifyConfig(c); err != nil {
		t.Fatalf("configure notifications: %v", err)
	}
	return bodies
}

// A scheduled run reports one summary line instead of a message per item. The
// tree changing, a snapshot left on the pool and applications started again
// after an interrupted run are not part of that count, and a user who never
// hears them cannot act on them.
func TestZFSChangeAndLeftoverNotificationsSurviveScheduledSummary(t *testing.T) {
	s, st, host, _ := zfsRunFixture(t, zfsTwoDatasetTree())
	bodies := zfsSummaryNotifications(t, s)
	host.destroyErr = errors.New("dataset is busy")

	d := zfsSeedItem(t, st, zfsRoot)
	if err := st.ReplaceZFSMembers(d.ID, []store.ZFSMember{
		{ItemID: d.ID, Dataset: zfsRoot, Outcome: "backed-up"},
	}); err != nil {
		t.Fatalf("seed the previous tree: %v", err)
	}

	scheduled := notify.WithMessagesSuppressed(context.Background())
	if _, err := s.BackupZFSDataset(scheduled, d.ID); err != nil {
		t.Fatalf("backup: %v", err)
	}

	sent := bodies()
	if !zfsAnyMessageContains(sent, zfsChild+" is new") {
		t.Fatalf("the new dataset was not reported: %v", sent)
	}
	if !zfsAnyMessageContains(sent, "could not be removed") {
		t.Fatalf("the leftover snapshot was not reported: %v", sent)
	}
	if zfsAnyMessageContains(sent, "Backup of zfs") {
		t.Fatalf("the per-item backup message must stay inside the summary: %v", sent)
	}
}

// A channel set up with the policy left unset reports failures, the way every
// other failure message and the anomaly sender read that setting.
func TestZFSLeftoverIsReportedWithThePolicyUnset(t *testing.T) {
	s, st, host, _ := zfsRunFixture(t, zfsTwoDatasetTree())
	bodies := zfsCaptureNotifications(t, s)
	c, err := s.NotifyConfig()
	if err != nil {
		t.Fatal(err)
	}
	c.On = ""
	if err := s.SetNotifyConfig(c); err != nil {
		t.Fatal(err)
	}
	host.destroyErr = errors.New("dataset is busy")
	d := zfsSeedItem(t, st, zfsRoot)

	_, _ = s.BackupZFSDataset(context.Background(), d.ID)

	if sent := bodies(); !zfsAnyMessageContains(sent, "could not be removed") {
		t.Fatalf("the leftover snapshot was not reported: %v", sent)
	}
}

// A cancel ends the run's own context, and whatever follows the run on that
// context fails at once: the message about the run went nowhere and the
// off-site copy was recorded as a failed run of its own.
func TestCancelledZFSRunNotifiesAndCopiesNothingOffSite(t *testing.T) {
	s, st, _, eng := zfsRunFixture(t, zfsTwoDatasetTree())
	bodies := zfsCaptureNotifications(t, s)
	if _, err := st.UpsertOffsiteTarget(store.OffsiteTarget{
		Domain: "zfs", Repo: "b2:bucket/offsite", Enabled: true,
	}); err != nil {
		t.Fatalf("create off-site target: %v", err)
	}
	d := zfsSeedItem(t, st, zfsRoot)
	eng.onBackup = func() { s.CancelBackupRun(zfsDomain+":"+zfsRoot, "") }

	if _, err := s.BackupZFSDataset(context.Background(), d.ID); err == nil {
		t.Fatal("a cancelled run must not report success")
	}

	if run := zfsLastRun(t, st, d.ID); run.Status != "cancelled" {
		t.Fatalf("run status = %q, want cancelled", run.Status)
	}
	if sent := bodies(); !zfsAnyMessageContains(sent, "Backup of zfs") {
		t.Fatalf("the cancelled run was not reported: %v", sent)
	}
	if len(eng.copies) != 0 {
		t.Fatalf("a cancelled run was copied off site: %v", eng.copies)
	}
	offsite, err := st.RecentRunsOfKind(domainRunTargetID(zfsDomain), "offsite", 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(offsite) != 0 {
		t.Fatalf("a cancelled run recorded off-site runs: %+v", offsite)
	}
}

// A cancel is no fault of the dataset restic was reading. The run says the
// dataset was not reached, and the item's tree, which the row counts skipped
// datasets from, keeps what the preflight found.
func TestCancelledZFSRunMarksNoDatasetAsFailed(t *testing.T) {
	s, st, _, eng := zfsRunFixture(t, zfsTwoDatasetTree())
	d := zfsSeedItem(t, st, zfsRoot)
	eng.onBackup = func() { s.CancelBackupRun(zfsDomain+":"+zfsRoot, "") }

	_, _ = s.BackupZFSDataset(context.Background(), d.ID)

	run := zfsLastRun(t, st, d.ID)
	members, err := st.ListZFSRunMembers(run.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range members {
		if m.Outcome != "not-reached" {
			t.Fatalf("run member %s = %q, want not-reached", m.Dataset, m.Outcome)
		}
	}
	tree, err := st.ListZFSMembers(d.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range tree {
		if m.Outcome != "" {
			t.Fatalf("tree member %s = %q, want the preflight's verdict kept", m.Dataset, m.Outcome)
		}
	}
}

func TestZFSRestartRecoveryNotificationSurvivesScheduledSummary(t *testing.T) {
	s, st, _, _ := zfsRunFixture(t, zfsTwoDatasetTree())
	s.docker = newZFSFakeDocker("plex")
	bodies := zfsSummaryNotifications(t, s)

	d := zfsSeedItem(t, st, zfsRoot)
	if err := st.SetZFSRestartPending(d.ID, []string{"plex"}); err != nil {
		t.Fatalf("mark the stopped container: %v", err)
	}

	s.RecoverZFSRestarts(notify.WithMessagesSuppressed(context.Background()))

	if sent := bodies(); !zfsAnyMessageContains(sent, "have been started again") {
		t.Fatalf("the recovered restart was not reported: %v", sent)
	}
}

// The tree changing and a snapshot left on the pool say nothing about how the
// run went, so they must not move the Healthchecks check the run itself ends.
func TestZFSSideNotificationsLeaveHealthchecksAlone(t *testing.T) {
	s, st, host, _ := zfsRunFixture(t, zfsTwoDatasetTree())
	bodies := zfsCaptureNotifications(t, s)
	var mu sync.Mutex
	var pings []string
	hc := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		mu.Lock()
		pings = append(pings, r.URL.Path)
		mu.Unlock()
	}))
	defer hc.Close()
	c, err := s.NotifyConfig()
	if err != nil {
		t.Fatal(err)
	}
	c.HealthchecksURL = hc.URL
	if err := s.SetNotifyConfig(c); err != nil {
		t.Fatal(err)
	}
	host.destroyErr = errors.New("dataset is busy")
	d := zfsSeedItem(t, st, zfsRoot)
	if err := st.ReplaceZFSMembers(d.ID, []store.ZFSMember{
		{ItemID: d.ID, Dataset: zfsRoot, Outcome: "backed-up"},
	}); err != nil {
		t.Fatalf("seed the previous tree: %v", err)
	}

	if _, err := s.BackupZFSDataset(context.Background(), d.ID); err != nil {
		t.Fatalf("backup: %v", err)
	}

	sent := bodies()
	if !zfsAnyMessageContains(sent, zfsChild+" is new") || !zfsAnyMessageContains(sent, "could not be removed") {
		t.Fatalf("the side messages were not sent: %v", sent)
	}
	mu.Lock()
	defer mu.Unlock()
	var ends []string
	for _, p := range pings {
		if p != "/start" {
			ends = append(ends, p)
		}
	}
	if len(ends) != 1 || ends[0] != "/" {
		t.Fatalf("a successful run ended its check with %v, want one success ping", ends)
	}
}

// Restarting the applications after an interrupted run is news for the user,
// not the result of a backup, and must not turn a red check green.
func TestZFSRestartRecoveryLeavesHealthchecksAlone(t *testing.T) {
	s, st, _, _ := zfsRunFixture(t, zfsTwoDatasetTree())
	s.docker = newZFSFakeDocker("plex")
	bodies := zfsCaptureNotifications(t, s)
	var hits int
	var mu sync.Mutex
	hc := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		mu.Lock()
		hits++
		mu.Unlock()
	}))
	defer hc.Close()
	c, err := s.NotifyConfig()
	if err != nil {
		t.Fatal(err)
	}
	c.HealthchecksURL = hc.URL
	if err := s.SetNotifyConfig(c); err != nil {
		t.Fatal(err)
	}
	d := zfsSeedItem(t, st, zfsRoot)
	if err := st.SetZFSRestartPending(d.ID, []string{"plex"}); err != nil {
		t.Fatalf("mark the stopped container: %v", err)
	}

	s.RecoverZFSRestarts(context.Background())

	if sent := bodies(); !zfsAnyMessageContains(sent, "have been started again") {
		t.Fatalf("the recovered restart was not reported: %v", sent)
	}
	mu.Lock()
	defer mu.Unlock()
	if hits != 0 {
		t.Fatalf("the restart notice pinged Healthchecks %d times", hits)
	}
}

// An item the user cancelled inside a Backup Everything pass is no failure of
// the pass: its own run says cancelled, and the round must not ping a failure
// or list it among the failed items.
func TestEverythingPassDoesNotCountACancelledItemAsFailed(t *testing.T) {
	s, st, _, eng := zfsRunFixture(t, zfsTwoDatasetTree())
	d := zfsSeedItem(t, st, zfsRoot)
	eng.onBackup = func() { s.CancelBackupRun(zfsDomain+":"+zfsRoot, "") }
	settings, err := st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}

	res := s.everythingRunZFS(context.Background(), "", settings)

	if res.Failed != 0 || len(res.Failures) != 0 {
		t.Fatalf("the cancelled item counts as failed: %+v", res)
	}
	if run := zfsLastRun(t, st, d.ID); run.Status != "cancelled" {
		t.Fatalf("run status = %q, want cancelled", run.Status)
	}
}

func TestIsBackupCancelledTellsACancelFromAFailure(t *testing.T) {
	s, st, _, eng := zfsRunFixture(t, zfsTwoDatasetTree())
	d := zfsSeedItem(t, st, zfsRoot)
	eng.onBackup = func() { s.CancelBackupRun(zfsDomain+":"+zfsRoot, "") }
	_, err := s.BackupZFSDataset(context.Background(), d.ID)
	if !IsBackupCancelled(err) {
		t.Fatalf("a cancelled backup's error %v does not read as cancelled", err)
	}
	if IsBackupCancelled(errors.New("restic: exit status 1")) {
		t.Fatal("a plain failure reads as cancelled")
	}
}
