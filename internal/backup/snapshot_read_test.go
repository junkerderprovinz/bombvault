package backup_test

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/backup"
	"github.com/junkerderprovinz/bombvault/internal/model"
)

// fakeSnapshot writes what it is asked to do into the Docker fake's log, so a
// test sees it in order with the stops and starts.
type fakeSnapshot struct {
	d       *fakeDocker
	takeErr error
	readErr error
	// onTake runs inside Take, for a test that cancels the run there.
	onTake   func()
	paths    []string
	tags     []string
	released int
}

func (s *fakeSnapshot) Take(context.Context) (backup.SnapshotRead, error) {
	s.d.log = append(s.d.log, "take")
	if s.onTake != nil {
		s.onTake()
	}
	if s.takeErr != nil {
		return nil, s.takeErr
	}
	return s, nil
}

func (s *fakeSnapshot) Backup(_ context.Context, _ string, paths, tags []string, _ ...string) (backup.Summary, error) {
	s.d.log = append(s.d.log, "read")
	s.paths, s.tags = paths, tags
	if s.readErr != nil {
		return backup.Summary{}, s.readErr
	}
	return backup.Summary{SnapshotID: "cafe1234cafe1234", Bytes: 10, Measured: true}, nil
}

func (s *fakeSnapshot) Release(ctx context.Context) {
	s.d.log = append(s.d.log, "release")
	if ctx.Err() != nil {
		s.d.log = append(s.d.log, "release-on-done-context")
	}
	s.released++
}

type snapshotRun struct {
	docker  *fakeDocker
	restic  *fakeRestic
	snap    *fakeSnapshot
	runs    *fakeRuns
	window  time.Duration
	windows int
	sum     backup.Summary
	err     error
}

func runSnapshotBackup(ctx context.Context, t *testing.T, configure func(*backup.BackupDeps, *fakeSnapshot)) snapshotRun {
	t.Helper()
	r := snapshotRun{docker: &fakeDocker{}, restic: &fakeRestic{summary: backup.Summary{SnapshotID: "deadbeef12345678"}}, runs: &fakeRuns{}}
	r.snap = &fakeSnapshot{d: r.docker}
	now := time.Unix(1000, 0)
	deps := backup.BackupDeps{
		ContainerRef:         "app",
		ContainerName:        "app",
		RepoPath:             "/repo",
		AppdataPaths:         []string{"/host/user/user/appdata/app"},
		TargetID:             "target-1",
		WasRunning:           true,
		PostHook:             "post",
		StopContainers:       []backup.StopContainer{{Name: "db", WasRunning: true}, {Name: "cache", WasRunning: false}},
		SnapshotTemplatesDir: "/data/templates",
		FlashTemplatesDir:    "/boot/templates",
		Snapshot:             r.snap,
		OnWindow:             func(w time.Duration) { r.window, r.windows = w, r.windows+1 },
		Clock: func() time.Time {
			now = now.Add(3 * time.Second)
			return now
		},
		Committed: func() { r.docker.log = append(r.docker.log, "committed") },
		Docker:    r.docker,
		Restic:    r.restic,
		Templates: &fakeTemplates{},
		Runs:      r.runs,
	}
	if configure != nil {
		configure(&deps, r.snap)
	}
	r.sum, r.err = backup.BackupContainer(ctx, deps)
	return r
}

// before reports whether a comes before b in log, and both are there.
func before(log []string, a, b string) bool {
	ia, ib := idxOf(log, a), idxOf(log, b)
	return ia >= 0 && ib >= 0 && ia < ib
}

func TestSnapshotBackupHoldsTheContainersDownOnlyForTheSnapshot(t *testing.T) {
	r := runSnapshotBackup(t.Context(), t, nil)
	if r.err != nil {
		t.Fatal(r.err)
	}
	log := r.docker.log
	for _, pair := range [][2]string{
		{"stop:app", "stop:db"},
		{"stop:db", "take"},
		{"take", "start:app"},
		{"start:app", "start:db"},
		{"start:db", "exec:app:sh -c post"},
		{"exec:app:sh -c post", "read"},
		{"read", "release"},
		{"release", "committed"},
	} {
		if !before(log, pair[0], pair[1]) {
			t.Errorf("%s has to come before %s: %v", pair[0], pair[1], log)
		}
	}
	if slices.Contains(log, "stop:cache") || slices.Contains(log, "start:cache") {
		t.Errorf("a dependency that was already stopped was touched: %v", log)
	}
	if countOf(log, "exec:app:sh -c post") != 1 {
		t.Errorf("the post-hook has to run exactly once: %v", log)
	}
	if len(r.restic.log) != 0 {
		t.Errorf("the live data was read as well: %v", r.restic.log)
	}
	if r.sum.SnapshotID != "cafe1234cafe1234" || r.runs.finishes[0] != "success" {
		t.Errorf("summary %+v, finishes %v", r.sum, r.runs.finishes)
	}
	if !slices.Equal(r.snap.paths, []string{"/host/user/user/appdata/app"}) || !slices.Contains(r.snap.tags, "container:app") {
		t.Errorf("the snapshot was read with paths %v and tags %v, not the container's own", r.snap.paths, r.snap.tags)
	}
	if r.windows != 1 || r.window <= 0 {
		t.Errorf("window reported %d times as %s", r.windows, r.window)
	}
}

func TestSnapshotBackupStartsTheContainersAgainWhenTheSnapshotFails(t *testing.T) {
	r := runSnapshotBackup(t.Context(), t, func(_ *backup.BackupDeps, s *fakeSnapshot) {
		s.takeErr = errors.New("zfs: busy")
	})
	if r.err == nil {
		t.Fatal("a failed snapshot has to fail the backup")
	}
	log := r.docker.log
	if !before(log, "take", "start:app") || !before(log, "take", "start:db") {
		t.Errorf("the containers have to come back: %v", log)
	}
	if slices.Contains(log, "read") || slices.Contains(log, "release") || slices.Contains(log, "exec:app:sh -c post") {
		t.Errorf("nothing may be read, released or hooked without a snapshot: %v", log)
	}
	if countOf(log, "stop:app") != 1 {
		t.Errorf("the container must not be stopped a second time: %v", log)
	}
	if r.runs.finishes[0] != "failed" {
		t.Errorf("finishes = %v", r.runs.finishes)
	}
}

func TestSnapshotBackupReleasesTheSnapshotWhenTheReadFails(t *testing.T) {
	r := runSnapshotBackup(t.Context(), t, func(_ *backup.BackupDeps, s *fakeSnapshot) {
		s.readErr = errors.New("restic: repository locked")
	})
	if r.err == nil || r.runs.finishes[0] != "failed" {
		t.Fatalf("err %v, finishes %v", r.err, r.runs.finishes)
	}
	if r.snap.released != 1 {
		t.Errorf("released %d times, want once", r.snap.released)
	}
	if slices.Contains(r.docker.log, "committed") {
		t.Errorf("a failed read was committed: %v", r.docker.log)
	}
}

func TestSnapshotBackupReleasesOnAContextNothingCancels(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	r := runSnapshotBackup(ctx, t, func(_ *backup.BackupDeps, s *fakeSnapshot) {
		s.readErr = context.Canceled
		s.onTake = cancel
	})
	if r.err == nil {
		t.Fatal("a cancelled read has to end the backup")
	}
	if slices.Contains(r.docker.log, "release-on-done-context") {
		t.Errorf("the snapshot was released on the cancelled context: %v", r.docker.log)
	}
	if !slices.Contains(r.docker.log, "start:app") || !slices.Contains(r.docker.log, "start:db") {
		t.Errorf("a cancel left the containers down: %v", r.docker.log)
	}
}

func TestSnapshotBackupTakesNoSnapshotAfterACancelDuringTheStop(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	r := runSnapshotBackup(ctx, t, func(d *backup.BackupDeps, _ *fakeSnapshot) {
		d.StopContainers = []backup.StopContainer{{Name: "db", WasRunning: true}}
		cancel()
	})
	if r.err == nil {
		t.Fatal("a cancel has to end the backup")
	}
	if slices.Contains(r.docker.log, "take") {
		t.Errorf("a snapshot was taken after the cancel: %v", r.docker.log)
	}
	if !slices.Contains(r.docker.log, "start:app") || !slices.Contains(r.docker.log, "start:db") {
		t.Errorf("a cancel left the containers down: %v", r.docker.log)
	}
}

func TestSnapshotBackupReadsLiveDataOfAStoppedContainer(t *testing.T) {
	r := runSnapshotBackup(t.Context(), t, func(d *backup.BackupDeps, _ *fakeSnapshot) {
		d.WasRunning = false
	})
	if r.err != nil {
		t.Fatal(r.err)
	}
	if slices.Contains(r.docker.log, "take") || len(r.restic.log) != 1 {
		t.Errorf("a stopped container has nothing to hold down: log %v, restic %v", r.docker.log, r.restic.log)
	}
}

func TestSnapshotBackupWithoutAnUpdateStopsTheDependentsOnce(t *testing.T) {
	checked := false
	r := runSnapshotBackup(t.Context(), t, func(d *backup.BackupDeps, _ *fakeSnapshot) {
		d.UpdateAfterBackup = func() func() {
			checked = true
			return nil
		}
	})
	if r.err != nil {
		t.Fatal(r.err)
	}
	if !checked {
		t.Error("the update check did not run")
	}
	if countOf(r.docker.log, "stop:db") != 1 {
		t.Errorf("no update arrived, yet the dependents went down again: %v", r.docker.log)
	}
}

func TestSnapshotBackupRecreatesWithTheDependentsDown(t *testing.T) {
	var d *fakeDocker
	r := runSnapshotBackup(t.Context(), t, func(deps *backup.BackupDeps, s *fakeSnapshot) {
		d = s.d
		deps.UpdateAfterBackup = func() func() {
			return func() { _ = d.CreateAndStart(context.Background(), model.Inspect{Name: "/app"}, true) }
		}
	})
	if r.err != nil {
		t.Fatal(r.err)
	}
	log := r.docker.log
	recreate := idxOf(log, "createAndStart:/app")
	if recreate < 0 || !before(log, "read", "createAndStart:/app") {
		t.Fatalf("the update has to come after the read: %v", log)
	}
	stops := []int{}
	starts := []int{}
	for i, e := range log {
		switch e {
		case "stop:db":
			stops = append(stops, i)
		case "start:db":
			starts = append(starts, i)
		}
	}
	if len(stops) != 2 || len(starts) != 2 || stops[1] > recreate || starts[1] < recreate {
		t.Errorf("db has to be down around the recreate: %v", log)
	}
}

func TestSnapshotBackupRunsNoUpdateWhenTheContainerDidNotComeBack(t *testing.T) {
	checked := false
	r := runSnapshotBackup(t.Context(), t, func(d *backup.BackupDeps, s *fakeSnapshot) {
		s.d.waitRunningErr = errors.New("not running after 60s")
		d.UpdateAfterBackup = func() func() {
			checked = true
			return nil
		}
	})
	if r.err == nil || r.runs.finishes[0] != "failed" {
		t.Fatalf("err %v, finishes %v; a container that did not come back fails the backup", r.err, r.runs.finishes)
	}
	if !slices.Contains(r.docker.log, "read") {
		t.Errorf("the snapshot was not read although it was taken: %v", r.docker.log)
	}
	if checked {
		t.Error("an update ran on a container that did not come back")
	}
}

func TestSnapshotBackupRunsNoUpdateAfterAFailedRead(t *testing.T) {
	checked := false
	r := runSnapshotBackup(t.Context(), t, func(d *backup.BackupDeps, s *fakeSnapshot) {
		s.readErr = errors.New("restic failed")
		d.UpdateAfterBackup = func() func() {
			checked = true
			return nil
		}
	})
	if r.err == nil || checked {
		t.Fatalf("err %v, update checked %v", r.err, checked)
	}
}
