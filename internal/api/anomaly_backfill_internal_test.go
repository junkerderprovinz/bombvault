package api

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// The backfill reads what the repositories already know about runs that
// predate the measurement. Matching is pure and tested on its own; the pass
// around it is about doing the work once per repository and saying so when a
// repository could not be opened.

func meta(id, parent string, bytes, files, added uint64, start int64, secs int64, tags ...string) restic.SnapshotMeta {
	return restic.SnapshotMeta{
		ID: id, Parent: parent, Tags: tags,
		Summary: &restic.SnapshotMetaSummary{
			BackupStart:         time.Unix(start, 0),
			BackupEnd:           time.Unix(start+secs, 0),
			FilesNew:            files,
			DataAdded:           added,
			TotalFilesProcessed: files,
			TotalBytesProcessed: bytes,
		},
	}
}

func TestMatchSnapshotSummaries(t *testing.T) {
	runs := []store.UnmeasuredRun{
		{ID: "run-c", TargetID: "tg-1", Kind: "backup", SnapshotID: "snapc", StartedAt: anomalyNow},
		{ID: "run-d", TargetID: "tg-1", Kind: "dbdump", SnapshotID: "snapd", StartedAt: anomalyNow},
		{ID: "run-v", TargetID: "tg-2", Kind: "backup", SnapshotID: "snapv", StartedAt: anomalyNow},
	}
	snaps := []restic.SnapshotMeta{
		meta("snapc", "older", 40<<30, 900, 1<<30, anomalyNow, 120),
		meta("snapd", "", 2<<30, 1, 2<<30, anomalyNow, 30, "bvrun:run-c"),
		meta("snapv", "older", 10<<30, 5, 1<<30, anomalyNow, 60, "vmrun:run-v"),
		meta("snapz", "older", 30<<30, 3, 3<<30, anomalyNow, 90, "vmrun:run-v"),
		{ID: "snapold", Tags: []string{"vmrun:run-v"}},
		{ID: "snapforeign", Summary: &restic.SnapshotMetaSummary{TotalBytesProcessed: 99}},
	}

	metrics, withoutSummary := matchSnapshotSummaries(runs, snaps)

	if got := metrics["run-c"].SourceBytes; got != 40<<30 {
		t.Fatalf("the container run read %d source bytes, want %d", got, int64(40)<<30)
	}
	if metrics["run-c"].HasParent == nil || !*metrics["run-c"].HasParent {
		t.Fatalf("the parent link did not reach the run: %+v", metrics["run-c"])
	}
	if got := metrics["run-c"].ResticMS; got != 120_000 {
		t.Fatalf("restic_ms = %d, want 120000", got)
	}
	if metrics["run-c"].Bytes != nil {
		t.Fatalf("a container run had its new data rewritten: %+v", metrics["run-c"])
	}
	if got := metrics["run-d"].SourceBytes; got != 2<<30 {
		t.Fatalf("the dump run read %d source bytes, want %d", got, int64(2)<<30)
	}
	if got := metrics["run-v"].SourceBytes; got != 40<<30 {
		t.Fatalf("the VM run summed %d source bytes, want the disks' %d", got, int64(40)<<30)
	}
	if metrics["run-v"].Bytes == nil || *metrics["run-v"].Bytes != 4<<30 {
		t.Fatalf("a VM run's new data was not corrected: %+v", metrics["run-v"])
	}
	if withoutSummary != 1 {
		t.Fatalf("%d snapshots counted as too old to carry a summary, want 1", withoutSummary)
	}
	if len(metrics) != 3 {
		t.Fatalf("a snapshot from outside this installation was matched: %+v", metrics)
	}
}

func TestMatchSnapshotSummariesKeepsADumpOffItsBackupRun(t *testing.T) {
	runs := []store.UnmeasuredRun{{ID: "run-c", TargetID: "tg-1", Kind: "backup", SnapshotID: "snapc", StartedAt: anomalyNow}}
	snaps := []restic.SnapshotMeta{
		meta("snapc", "older", 40<<30, 900, 1<<30, anomalyNow, 120),
		meta("snapd", "", 2<<30, 1, 2<<30, anomalyNow, 30, "bvrun:run-c"),
	}

	metrics, _ := matchSnapshotSummaries(runs, snaps)
	if got := metrics["run-c"].SourceBytes; got != 40<<30 {
		t.Fatalf("the dump was summed into the backup it belongs to: %d source bytes", got)
	}
}

func TestMatchSnapshotSummariesClampsABackwardsClock(t *testing.T) {
	runs := []store.UnmeasuredRun{{ID: "run-c", TargetID: "tg-1", Kind: "backup", SnapshotID: "snapc"}}
	snaps := []restic.SnapshotMeta{meta("snapc", "older", 1<<30, 5, 1<<20, anomalyNow, -30)}

	metrics, _ := matchSnapshotSummaries(runs, snaps)
	if got := metrics["run-c"].ResticMS; got != 0 {
		t.Fatalf("restic_ms = %d, want 0", got)
	}
}

func TestBackfillSlotsNameEveryRepositoryOnce(t *testing.T) {
	repos := []domainRepoRef{
		ownRef("/mnt/user/backups/containers"),
		namedRef("/mnt/disks/cold/repo", store.OffsiteTarget{ID: "cold", Name: "Cold"}),
		namedRef("rclone:box:bv", store.OffsiteTarget{ID: "box", Name: "Box"}),
	}
	got := backfillSlots("containers", repos)
	want := []backfillSlot{
		{Slot: "domain:containers", Repo: "/mnt/user/backups/containers"},
		{Slot: "repo:cold", Repo: "/mnt/disks/cold/repo"},
		{Slot: "repo:box", Repo: "rclone:box:bv"},
	}
	if len(got) != len(want) {
		t.Fatalf("slots = %+v, want %+v", got, want)
	}
	for i := range want {
		if got[i].Slot != want[i].Slot || got[i].Repo != want[i].Repo {
			t.Fatalf("slot %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}

// backfillFixture is an engine over a real store whose containers repository
// exists on disk, so the slot the backfill builds resolves.
type backfillFixture struct {
	*engineFixture
	eng *backfillEngine
}

type backfillEngine struct {
	ResticEngine
	snaps []restic.SnapshotMeta
	err   error
	lists int
	// before runs inside the listing, where a test can hold one backfill open
	// while it starts another.
	before func()
}

func (e *backfillEngine) SnapshotsMeta(context.Context, string, restic.Mode) ([]restic.SnapshotMeta, error) {
	if e.before != nil {
		e.before()
	}
	e.lists++
	if e.err != nil {
		return nil, e.err
	}
	return e.snaps, nil
}

func newBackfillFixture(t *testing.T) *backfillFixture {
	t.Helper()
	f := newEngineFixture(t)
	dir := t.TempDir()
	repo := filepath.Join(dir, "backups", "containers")
	if err := os.MkdirAll(repo, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "config"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	settings, err := f.st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	settings.ContainersEnabled, settings.VMsEnabled = true, false
	settings.FilesEnabled, settings.FlashEnabled, settings.ConfigEnabled = false, false, false
	settings.ContainersPath = "backups/containers"
	if err := f.st.UpdateSettings(settings); err != nil {
		t.Fatal(err)
	}
	eng := &backfillEngine{}
	f.svc.cfg.DataDir, f.svc.cfg.HostMountRoot = dir, dir
	f.svc.engine = eng
	return &backfillFixture{engineFixture: f, eng: eng}
}

// unmeasuredRun records a finished run that left a snapshot and no figures,
// which is what an installation looks like before this measurement existed.
func (f *backfillFixture) unmeasuredRun(t *testing.T, targetID, snapshotID string, at int64) string {
	t.Helper()
	id, err := f.st.StartRun(targetID, "backup")
	if err != nil {
		t.Fatal(err)
	}
	if err := f.st.FinishRun(id, "success", snapshotID, 1<<20, ""); err != nil {
		t.Fatal(err)
	}
	f.backdate(t, id, at)
	return id
}

func (f *backfillFixture) slot(t *testing.T, name string) store.AnomalyBackfillSlot {
	t.Helper()
	slots, err := f.st.ListAnomalyBackfill()
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range slots {
		if s.Slot == name {
			return s
		}
	}
	t.Fatalf("no slot %q in %+v", name, slots)
	return store.AnomalyBackfillSlot{}
}

func TestBackfillFillsOncePerSlotAndReportsFailure(t *testing.T) {
	f := newBackfillFixture(t)
	targetID := f.container(t, "Nexterm")
	f.unmeasuredRun(t, targetID, "snap1", f.now-10*86400)
	f.eng.snaps = []restic.SnapshotMeta{
		meta("snap1", "older", 40<<30, 900, 1<<30, f.now-10*86400, 120),
		{ID: "snap0", Tags: []string{}},
	}

	if err := f.e.backfillRunMetrics(context.Background()); err != nil {
		t.Fatalf("backfill: %v", err)
	}
	rows, err := f.st.ItemSeries(targetID, "backup", f.now, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].SourceBytes == nil || *rows[0].SourceBytes != 40<<30 {
		t.Fatalf("the run was not filled from its snapshot: %+v", rows)
	}
	slot := f.slot(t, "domain:containers")
	if !slot.Done || slot.Filled != 1 {
		t.Fatalf("slot = %+v, want one filled run and done", slot)
	}

	if err := f.e.backfillRunMetrics(context.Background()); err != nil {
		t.Fatalf("second backfill: %v", err)
	}
	if f.eng.lists != 1 {
		t.Fatalf("the repository was listed %d times, want once", f.eng.lists)
	}

	t.Run("a newer unmeasured run makes the slot read again", func(t *testing.T) {
		f.unmeasuredRun(t, targetID, "snap2", f.now+3600)
		if err := f.e.backfillRunMetrics(context.Background()); err != nil {
			t.Fatal(err)
		}
		if f.eng.lists != 1 {
			t.Fatalf("a done slot was re-read within the day (%d lists)", f.eng.lists)
		}
		f.now += 25 * 3600
		if err := f.e.backfillRunMetrics(context.Background()); err != nil {
			t.Fatal(err)
		}
		if f.eng.lists != 2 {
			t.Fatalf("a done slot with newer runs was not read again (%d lists)", f.eng.lists)
		}
	})
}

func TestBackfillRetriesAFailedRepositoryOnceADay(t *testing.T) {
	f := newBackfillFixture(t)
	targetID := f.container(t, "Nexterm")
	f.unmeasuredRun(t, targetID, "snap1", f.now-10*86400)
	f.eng.err = errors.New("unable to open repository at /mnt/user/backups/containers: locked")

	if err := f.e.backfillRunMetrics(context.Background()); err != nil {
		t.Fatalf("backfill: %v", err)
	}
	slot := f.slot(t, "domain:containers")
	if slot.Done {
		t.Fatalf("a repository that would not open was recorded as done: %+v", slot)
	}
	if slot.Error == "" || strings.Contains(slot.Error, "/mnt/user") {
		t.Fatalf("the recorded error is empty or carries a path: %q", slot.Error)
	}

	if err := f.e.backfillRunMetrics(context.Background()); err != nil {
		t.Fatal(err)
	}
	if f.eng.lists != 1 {
		t.Fatalf("a failed slot was retried within the day (%d lists)", f.eng.lists)
	}

	f.now += 25 * 3600
	f.eng.err = nil
	f.eng.snaps = []restic.SnapshotMeta{meta("snap1", "older", 40<<30, 900, 1<<30, f.now-10*86400, 120)}
	if err := f.e.backfillRunMetrics(context.Background()); err != nil {
		t.Fatal(err)
	}
	if f.eng.lists != 2 {
		t.Fatalf("a failed slot was not retried after a day (%d lists)", f.eng.lists)
	}
	if slot := f.slot(t, "domain:containers"); !slot.Done || slot.Error != "" || slot.Filled != 1 {
		t.Fatalf("the retry did not settle the slot: %+v", slot)
	}
}

// The read scheduled for shortly after boot and the worker's own can meet.
// Only one of them lists the repositories: the other would pay for the same
// listing again and write the same slot counter back over it.
func TestBackfillListsARepositoryOnceWhileOneIsRunning(t *testing.T) {
	f := newBackfillFixture(t)
	targetID := f.container(t, "Nexterm")
	f.unmeasuredRun(t, targetID, "snap1", f.now-10*86400)
	f.eng.snaps = []restic.SnapshotMeta{meta("snap1", "older", 40<<30, 900, 1<<30, f.now-10*86400, 120)}

	var listings atomic.Int64
	listing, release := make(chan struct{}), make(chan struct{})
	f.eng.before = func() {
		if listings.Add(1) == 1 {
			close(listing)
			<-release
		}
	}

	done := make(chan error, 1)
	go func() { done <- f.e.backfillRunMetrics(context.Background()) }()
	<-listing
	if err := f.e.backfillRunMetrics(context.Background()); err != nil {
		t.Fatalf("second backfill: %v", err)
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatalf("backfill: %v", err)
	}

	if got := listings.Load(); got != 1 {
		t.Fatalf("the repository was listed %d times, want once", got)
	}
}

func TestBackfillReportsItsProgressInTheSummary(t *testing.T) {
	f := newBackfillFixture(t)
	targetID := f.container(t, "Nexterm")
	f.unmeasuredRun(t, targetID, "snap1", f.now-10*86400)
	f.eng.snaps = []restic.SnapshotMeta{
		meta("snap1", "older", 40<<30, 900, 1<<30, f.now-10*86400, 120),
		{ID: "snapold", Tags: []string{"bvrun:gone"}},
	}

	if err := f.e.backfillRunMetrics(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := f.e.rebuildCache(); err != nil {
		t.Fatal(err)
	}
	got := f.e.summary().Backfill
	if got.Slots != 1 || got.Done != 1 || got.Failed != 0 || got.Filled != 1 {
		t.Fatalf("backfill summary = %+v", got)
	}
}
