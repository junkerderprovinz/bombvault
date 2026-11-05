package api

import (
	"context"
	"errors"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/restic"
)

// dockhandDB is the file of the dockhand rig a restore in place picks.
func (r *restoreRig) dockhandDB() (rel, abs string) {
	rel = "appdata/dockhand/dockhand.db"
	return rel, filepath.ToSlash(filepath.Join(r.root, rel))
}

func TestAPickedFileInPlaceStopsTheContainerWhileItIsWritten(t *testing.T) {
	r := newRestoreRig(t)
	tg, snap := r.seedDockhand(t)
	rel, file := r.dockhandDB()
	r.write(t, map[string]string{rel: "broken"})
	r.docker.calls = nil

	if _, started, err := r.svc.StartRestoreFiles(context.Background(), "dockhand", "local", snap, []string{file}, "", true, RestoreOptions{}); err != nil || !started {
		t.Fatalf("StartRestoreFiles: started=%v err=%v", started, err)
	}
	r.wait(t)

	if run := r.lastRun(t, tg.ID, "restore"); run.Status != "success" {
		t.Fatalf("restore run = %s: %s", run.Status, run.Error)
	}
	if got := read(t, filepath.Join(r.root, rel)); got != "db" {
		t.Errorf("the file holds %q, want it restored", got)
	}
	if got := r.docker.did("stop", "start"); !slices.Equal(got, []string{"stop:dockhand", "start:dockhand"}) {
		t.Errorf("Docker was told %v, want a stop and a start", got)
	}
}

func TestAPickedFileInAFolderLeavesTheContainerRunning(t *testing.T) {
	r := newRestoreRig(t)
	_, snap := r.seedDockhand(t)
	_, file := r.dockhandDB()
	r.docker.calls = nil

	if _, started, err := r.svc.StartRestoreFiles(context.Background(), "dockhand", "local", snap, []string{file}, "temp/restores", true, RestoreOptions{}); err != nil || !started {
		t.Fatalf("StartRestoreFiles: started=%v err=%v", started, err)
	}
	r.wait(t)

	if got := r.docker.did("stop", "start"); len(got) != 0 {
		t.Errorf("Docker was told %v", got)
	}
}

func TestAPickedFileInPlaceBacksUpTheCurrentStateFirstWhenAsked(t *testing.T) {
	r := newRestoreRig(t)
	tg, snap := r.seedDockhand(t)
	rel, file := r.dockhandDB()
	r.write(t, map[string]string{rel: "broken"})

	res := r.check(t, RestoreCheckRequest{Kind: checkContainerFiles, Name: "dockhand", SnapshotID: snap, Paths: []string{file}, BackupFirst: true})
	if b := backupFirstOf(t, res); b.Status != lineOK || !res.Ready {
		t.Fatalf("backup first = %+v ready=%v", b, res.Ready)
	}

	if _, started, err := r.svc.StartRestoreFiles(context.Background(), "dockhand", "local", snap, []string{file}, "", true, RestoreOptions{BackupFirst: true}); err != nil || !started {
		t.Fatalf("StartRestoreFiles: started=%v err=%v", started, err)
	}
	r.wait(t)

	if run := r.lastRun(t, tg.ID, "restore"); run.Status != "success" {
		t.Fatalf("restore run = %s: %s", run.Status, run.Error)
	}
	before := r.stateBefore(t, "containers", "container:dockhand", snap)
	if err := r.svc.engine.RestoreInclude(context.Background(), r.containersRepo(t), before.ID, restic.EscapeGlob(file), filepath.ToSlash(filepath.Join(r.root, "out")), restic.Mode{}); err != nil {
		t.Fatal(err)
	}
	if got := read(t, filepath.Join(r.root, "out", file)); got != "broken" {
		t.Errorf("the backup taken first holds %q, want the state the restore replaced", got)
	}
}

func TestAPickedFileThatFailsLeavesTheContainerStoppedAndSaysSo(t *testing.T) {
	r := newRestoreRig(t)
	tg, snap := r.seedDockhand(t)
	_, file := r.dockhandDB()
	r.docker.calls = nil
	r.svc.engine = failingRestore{ResticEngine: r.svc.engine, listing: []restic.Snapshot{{ID: snap, Tags: []string{"container:dockhand"}, Paths: []string{filepath.Dir(file)}}}}

	if _, started, err := r.svc.StartRestoreFiles(context.Background(), "dockhand", "local", snap, []string{file}, "", true, RestoreOptions{}); err != nil || !started {
		t.Fatalf("StartRestoreFiles: started=%v err=%v", started, err)
	}
	r.wait(t)

	run := r.lastRun(t, tg.ID, "restore")
	if run.Status != "failed" || !strings.HasPrefix(run.Error, "restore did not finish: the container was left stopped") {
		t.Errorf("run = %s %q", run.Status, run.Error)
	}
	if got := r.docker.did("stop", "start"); !slices.Equal(got, []string{"stop:dockhand"}) {
		t.Errorf("Docker was told %v, want only the stop", got)
	}
}

// failingRestore answers the listing and fails every picked restore.
type failingRestore struct {
	ResticEngine
	listing []restic.Snapshot
}

func (f failingRestore) Snapshots(context.Context, string, restic.Mode) ([]restic.Snapshot, error) {
	return f.listing, nil
}

func (failingRestore) RestoreInclude(context.Context, string, string, string, string, restic.Mode) error {
	return errors.New("restic restore failed: no space left on device")
}

func (failingRestore) RestoreSubtreeInclude(context.Context, string, string, string, string, string, restic.Mode) error {
	return errors.New("restic restore failed: no space left on device")
}

func (r *restoreRig) containersRepo(t *testing.T) string {
	t.Helper()
	settings, err := r.st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	repo, err := r.svc.repoFor(settings, "containers", "local")
	if err != nil {
		t.Fatal(err)
	}
	return repo
}

func TestARestoreThatNamesNoSnapshotPassesOverTheBackupTakenBeforeARestore(t *testing.T) {
	good := restic.Snapshot{ID: "good", Tags: []string{"container:plex"}}
	before := restic.Snapshot{ID: "before", Tags: []string{"container:plex", "bv:before-restore:abcd1234"}}
	bare := restic.Snapshot{ID: "bare", Tags: []string{"bv:before-restore"}}

	if got := newestToRestore([]restic.Snapshot{good, before, bare}); got.ID != "good" {
		t.Errorf("newest = %s, want the backup before them", got.ID)
	}
	if got := newestToRestore([]restic.Snapshot{before, bare}); got.ID != "bare" {
		t.Errorf("newest = %s, want the newest when there is no other", got.ID)
	}
	later := restic.Snapshot{ID: "later", Tags: []string{"container:plex"}}
	if got := newestToRestore([]restic.Snapshot{good, before, later}); got.ID != "later" {
		t.Errorf("newest = %s, want the ordinary backup after it", got.ID)
	}
}

func TestLatestOfAContainerIsNotTheBackupTakenBeforeARestore(t *testing.T) {
	r := newRestoreRig(t)
	tg, first := r.seedPlex(t)
	r.write(t, map[string]string{"appdata/plex/library.db": "the state a restore replaced"})
	if started, err := r.svc.StartRestore(context.Background(), "plex", first, "local", RestoreOptions{BackupFirst: true}); err != nil || !started {
		t.Fatalf("StartRestore: started=%v err=%v", started, err)
	}
	r.wait(t)
	if run := r.lastRun(t, tg.ID, "restore"); run.Status != "success" {
		t.Fatalf("restore run = %s: %s", run.Status, run.Error)
	}

	plan, err := r.svc.prepareRestore(context.Background(), "plex", "latest", true, "local")
	if err != nil {
		t.Fatal(err)
	}
	if plan.snapshotID != first {
		t.Errorf("latest = %s, want %s, the backup before the one taken for the restore", shortID(plan.snapshotID), shortID(first))
	}
}
