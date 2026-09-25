package api

import (
	"context"
	"errors"
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
