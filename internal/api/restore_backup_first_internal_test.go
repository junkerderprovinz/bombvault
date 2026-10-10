package api

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/progress"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

func TestBeforeRestoreTagsMarkTheStateBeforeARestore(t *testing.T) {
	if got := beforeRestoreTags(context.Background()); got != nil {
		t.Fatalf("an ordinary backup got the tags %v", got)
	}
	ctx := withBeforeRestore(context.Background(), "abcd1234ef567890")
	if got := beforeRestoreTags(ctx); !slices.Equal(got, []string{"bv:before-restore:abcd1234"}) {
		t.Fatalf("tags = %v, want the marker with the short id of the restored snapshot", got)
	}
	// A container that is only recreated from its definition has no snapshot.
	if got := beforeRestoreTags(withBeforeRestore(context.Background(), "")); !slices.Equal(got, []string{"bv:before-restore"}) {
		t.Fatalf("tags = %v, want the bare marker", got)
	}
}

// firstFixture is a Service with a store, a progress stream and one container
// target, enough for backupBeforeRestore to record what it decides.
func firstFixture(t *testing.T) (*Service, store.Target, *progress.Store) {
	t.Helper()
	st := newTestStore(t)
	tg, err := st.UpsertTarget(store.Target{ContainerName: "plex"})
	if err != nil {
		t.Fatal(err)
	}
	p := progress.NewStore()
	s := &Service{store: st}
	s.SetProgress(p)
	return s, tg, p
}

func TestBackupBeforeRestoreLetsTheRestoreStartAfterAGoodBackup(t *testing.T) {
	s, tg, _ := firstFixture(t)
	var marked, deferred bool

	err := s.backupBeforeRestore(context.Background(), "container:plex", tg.ID, "abcd1234", func(ctx context.Context) error {
		marked, deferred = isBeforeRestore(ctx), bulkReplicateSuppressed(ctx)
		return nil
	})

	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if !marked || !deferred {
		t.Fatalf("marked=%v, off-site copy deferred=%v, want both", marked, deferred)
	}
	if run, _ := s.store.LastRunOfKind(tg.ID, "restore"); run != nil {
		t.Fatalf("the restore records its own run, not the backup before it: %+v", run)
	}
}

func TestBackupBeforeRestoreKeepsTheRestoreFromStartingAndSaysWhy(t *testing.T) {
	s, tg, p := firstFixture(t)
	events, stop := p.Subscribe()
	defer stop()

	err := s.backupBeforeRestore(context.Background(), "container:plex", tg.ID, "abcd1234", func(context.Context) error {
		return errors.New("repository is locked")
	})

	if err == nil || !strings.Contains(err.Error(), "nothing was restored") {
		t.Fatalf("err = %v, want it to say that nothing was restored", err)
	}
	run, _ := s.store.LastRunOfKind(tg.ID, "restore")
	if run == nil || run.Status != "failed" || !strings.Contains(run.Error, "backup of the current state failed: repository is locked") {
		t.Fatalf("run = %+v, want a failed restore that names the backup's failure", run)
	}
	select {
	case e := <-events:
		if e.Key != "container:plex" || e.Phase != "restore" || e.Active {
			t.Fatalf("event = %+v, want the end of the restore", e)
		}
	default:
		t.Fatal("whoever watches the restore is never told that it ended")
	}
}

func TestBackupBeforeRestoreRecordsACancelAsCancelled(t *testing.T) {
	for name, save := range map[string]func(cancel context.CancelFunc) error{
		"the restore is cancelled during the backup": func(cancel context.CancelFunc) error {
			cancel()
			return nil
		},
		"the backup itself is cancelled": func(context.CancelFunc) error {
			return cancelledBackupError{errors.New("signal: killed")}
		},
	} {
		t.Run(name, func(t *testing.T) {
			s, tg, _ := firstFixture(t)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			err := s.backupBeforeRestore(ctx, "container:plex", tg.ID, "abcd1234", func(context.Context) error { return save(cancel) })

			if !errors.Is(err, context.Canceled) {
				t.Fatalf("err = %v, want a cancellation", err)
			}
			if run, _ := s.store.LastRunOfKind(tg.ID, "restore"); run == nil || run.Status != "cancelled" {
				t.Fatalf("run = %+v, want cancelled", run)
			}
		})
	}
}

func TestCheckRestoreSaysWhetherTheCurrentStateIsBackedUpFirst(t *testing.T) {
	f := newCheckFixture(t)
	tg, err := f.svc.store.GetTargetByContainer("plex")
	if err != nil {
		t.Fatal(err)
	}
	runID, err := f.svc.store.StartRun(tg.ID, "backup")
	if err != nil {
		t.Fatal(err)
	}
	if err := f.svc.store.FinishRun(runID, "success", "cccc3333", 4096, ""); err != nil {
		t.Fatal(err)
	}
	req := RestoreCheckRequest{Kind: checkContainer, Name: "plex", SnapshotID: "aaaa1111", BackupFirst: true}

	res := f.check(t, req)
	if b := backupFirstOf(t, res); b.Status != lineOK || b.Bytes != 4096 || b.Seconds < 0 || !res.Ready {
		t.Fatalf("backup first = %+v ready=%v", b, res.Ready)
	}

	// A container that is gone has no state to save, and the restore goes on.
	f.docker.list = nil
	res = f.check(t, req)
	if b := backupFirstOf(t, res); b.Status != lineSkip || b.Reason != reasonGone || !res.Ready {
		t.Fatalf("backup first = %+v ready=%v", b, res.Ready)
	}
}

func TestCheckRestoreHasNoBackupLineWhenNothingIsReplaced(t *testing.T) {
	f, _ := newBesideCheckFixture(t)
	for name, req := range map[string]RestoreCheckRequest{
		"not asked for":        {Kind: checkContainer, Name: "plex", SnapshotID: "aaaa1111"},
		"next to the original": {Kind: checkContainer, Name: "plex", SnapshotID: "aaaa1111", Target: targetBeside, BackupFirst: true},
		"into a folder":        {Kind: checkContainerTo, Name: "plex", SnapshotID: "aaaa1111", TargetPath: "restore/plex", BackupFirst: true},
	} {
		res := f.check(t, req)
		if res.Plan.BackupFirst != nil {
			t.Errorf("%s: backupFirst = %+v", name, res.Plan.BackupFirst)
		}
	}
}

func TestCheckRestoreRefusesToBackUpBombVaultsOwnContainerFirst(t *testing.T) {
	f := newCheckFixture(t)
	f.docker.self = "plex"

	res := f.check(t, RestoreCheckRequest{Kind: checkContainer, Name: "plex", SnapshotID: "aaaa1111", BackupFirst: true})

	if b := backupFirstOf(t, res); b.Status != lineFail || res.Ready {
		t.Fatalf("backup first = %+v ready=%v", b, res.Ready)
	}
}

// snapshotsOf lists what a domain's repository on the rig holds.
func (r *restoreRig) snapshotsOf(t *testing.T, domain string) []restic.Snapshot {
	t.Helper()
	settings, err := r.st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	repo, err := r.svc.repoFor(settings, domain, "local")
	if err != nil {
		t.Fatal(err)
	}
	snaps, err := r.svc.engine.Snapshots(context.Background(), repo, r.svc.repoModeFor(settings, domain, "local", repo))
	if err != nil {
		t.Fatal(err)
	}
	return snaps
}

// stateBefore finds the snapshot tagged as the state before the restore of
// restored, which must also carry the entry's own tag.
func (r *restoreRig) stateBefore(t *testing.T, domain, identity, restored string) restic.Snapshot {
	t.Helper()
	want := beforeRestoreTag + ":" + shortID(restored)
	for _, sn := range r.snapshotsOf(t, domain) {
		if slices.Contains(sn.Tags, want) {
			if !slices.Contains(sn.Tags, identity) {
				t.Fatalf("the snapshot %s is tagged %v, want %s too", sn.ID, sn.Tags, identity)
			}
			return sn
		}
	}
	t.Fatalf("no snapshot is tagged %s", want)
	return restic.Snapshot{}
}

func (r *restoreRig) backupRuns(t *testing.T, targetID string) int {
	t.Helper()
	runs, err := r.st.RecentRunsOfKind(targetID, "backup", 20)
	if err != nil {
		t.Fatal(err)
	}
	return len(runs)
}

func read(t *testing.T, p string) string {
	t.Helper()
	b, err := os.ReadFile(p) //nolint:gosec // G304: a file below the test's own folder
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestRestoreContainerBacksUpTheCurrentStateFirstSoTheReplaceCanBeUndone(t *testing.T) {
	r := newRestoreRig(t)
	tg, snap := r.seedPlex(t)
	library := filepath.Join(r.root, "appdata/plex/library.db")
	r.write(t, map[string]string{"appdata/plex/library.db": "a newer library"})
	r.docker.forget()

	res := r.check(t, RestoreCheckRequest{Kind: checkContainer, Name: "plex", SnapshotID: snap, BackupFirst: true})
	if b := backupFirstOf(t, res); b.Status != lineOK || !res.Ready {
		t.Fatalf("backup first = %+v ready=%v", b, res.Ready)
	}
	if r.backupRuns(t, tg.ID) != 1 {
		t.Fatal("a check took a backup")
	}

	// With one backup to keep, a backup that counted at once would cost the
	// restore the snapshot it is about to read.
	settings, err := r.st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	settings.RetentionKeepLast = 1
	if err := r.st.UpdateSettings(settings); err != nil {
		t.Fatal(err)
	}
	started, err := r.svc.StartRestore(context.Background(), "plex", snap, "local", RestoreOptions{BackupFirst: true})
	if err != nil || !started {
		t.Fatalf("StartRestore: started=%v err=%v", started, err)
	}
	r.wait(t)

	if got := read(t, library); got != "library at backup time" {
		t.Fatalf("library.db = %q after the restore", got)
	}
	// The backup stops and starts the container before the restore removes it.
	calls := r.docker.did("stop", "start", "remove", "create")
	if want := []string{"stop:plex", "start:plex", "stop:plex", "remove:plex", "create:plex:start=true"}; !slices.Equal(calls, want) {
		t.Fatalf("docker calls = %v, want %v", calls, want)
	}
	if r.backupRuns(t, tg.ID) != 2 {
		t.Fatalf("backup runs = %d, want the seed and the one before the restore", r.backupRuns(t, tg.ID))
	}
	before := r.stateBefore(t, "containers", "container:plex", snap)

	// The timeline shows the backup with its marker.
	tl, err := r.svc.Timeline(context.Background(), "containers", "plex")
	if err != nil {
		t.Fatal(err)
	}
	marked := slices.ContainsFunc(tl.Rows, func(row timelineRow) bool {
		return row.Key == before.ID && slices.Contains(row.Places[0].Tags, beforeRestoreTag+":"+shortID(snap))
	})
	if !marked {
		t.Fatalf("the timeline does not show %s as the state before the restore: %+v", before.ID, tl.Rows)
	}

	// Restoring that backup undoes the replace, and without the option no
	// further backup is taken.
	if started, err := r.svc.StartRestore(context.Background(), "plex", before.ID, "local", RestoreOptions{}); err != nil || !started {
		t.Fatalf("undo: started=%v err=%v", started, err)
	}
	r.wait(t)
	if got := read(t, library); got != "a newer library" {
		t.Fatalf("library.db = %q after the undo", got)
	}
	if r.backupRuns(t, tg.ID) != 2 {
		t.Fatal("a restore without the option took a backup")
	}

	// The entry's next backup applies the policy to the backup before the
	// restore as to any other.
	if _, err := r.svc.Backup(context.Background(), "plex"); err != nil {
		t.Fatal(err)
	}
	if left := r.snapshotsOf(t, "containers"); len(left) != 1 || left[0].ID == before.ID {
		t.Fatalf("keep-last 1 left %d snapshots, want only the newest", len(left))
	}
}

// brokenBackups is the real engine with a repository that takes no backup.
type brokenBackups struct{ ResticEngine }

func (brokenBackups) Backup(context.Context, string, []string, []string, restic.Mode, ...string) (restic.Summary, error) {
	return restic.Summary{}, errors.New("Fatal: unable to save snapshot: no space left on device")
}

func TestRestoreDoesNotStartWhenTheBackupOfTheCurrentStateFails(t *testing.T) {
	const reason = "nothing was restored, because the backup of the current state failed"

	t.Run("container", func(t *testing.T) {
		r := newRestoreRig(t)
		tg, snap := r.seedPlex(t)
		r.write(t, map[string]string{"appdata/plex/library.db": "a newer library"})
		before := treeSum(t, filepath.Join(r.root, "appdata"), true)
		r.svc.engine = brokenBackups{r.svc.engine}
		r.docker.forget()

		if started, err := r.svc.StartRestore(context.Background(), "plex", snap, "local", RestoreOptions{BackupFirst: true}); err != nil || !started {
			t.Fatalf("StartRestore: started=%v err=%v", started, err)
		}
		r.wait(t)

		if run := r.lastRun(t, tg.ID, "restore"); run.Status != "failed" || !strings.Contains(run.Error, reason) {
			t.Fatalf("run = %+v, want a failed restore saying %q", run, reason)
		}
		if calls := r.docker.did("remove", "create"); len(calls) != 0 {
			t.Fatalf("the restore went on to %v", calls)
		}
		if original, ok := r.docker.container("plex"); !ok || !original.Running {
			t.Fatalf("the container is not back up after the failed backup: %+v", original)
		}
		if treeSum(t, filepath.Join(r.root, "appdata"), true) != before {
			t.Fatal("the appdata changed")
		}
	})

	t.Run("vm", func(t *testing.T) {
		r := newRestoreRig(t)
		tg, snap := r.seedWin(t)
		r.write(t, map[string]string{"domains/win/vdisk2.img": "the data disk moved on"})
		before := treeSum(t, filepath.Join(r.root, "domains"), true)
		r.svc.engine = brokenBackups{r.svc.engine}
		r.virsh.forget()

		if started, err := r.svc.StartRestoreVM(context.Background(), "win", snap, "local", RestoreOptions{BackupFirst: true}); err != nil || !started {
			t.Fatalf("StartRestoreVM: started=%v err=%v", started, err)
		}
		r.wait(t)

		if run := r.lastRun(t, tg.ID, "restore"); run.Status != "failed" || !strings.Contains(run.Error, reason) {
			t.Fatalf("run = %+v, want a failed restore saying %q", run, reason)
		}
		for _, call := range r.virsh.recorded() {
			if call == "undefine" || call == "destroy" || call == "define" {
				t.Fatalf("the restore went on to %v", r.virsh.recorded())
			}
		}
		if treeSum(t, filepath.Join(r.root, "domains"), true) != before {
			t.Fatal("the disks changed")
		}
	})

	t.Run("file set", func(t *testing.T) {
		r := newRestoreRig(t)
		set, snap := r.seedFotos(t)
		r.write(t, map[string]string{"fotos/2025/garten.jpg": "bearbeitet"})
		before := treeSum(t, filepath.Join(r.root, "fotos"), true)
		r.svc.engine = brokenBackups{r.svc.engine}

		if _, started, err := r.svc.StartRestoreFileSet(context.Background(), set.ID, snap, "local", "", true, RestoreOptions{BackupFirst: true}); err != nil || !started {
			t.Fatalf("StartRestoreFileSet: started=%v err=%v", started, err)
		}
		r.wait(t)

		if run := r.lastRun(t, set.ID, "restore"); run.Status != "failed" || !strings.Contains(run.Error, reason) {
			t.Fatalf("run = %+v, want a failed restore saying %q", run, reason)
		}
		if treeSum(t, filepath.Join(r.root, "fotos"), true) != before {
			t.Fatal("the set's folder changed")
		}
	})
}

func TestRestoreOfAContainerThatIsGoneHasNoStateToBackUpFirst(t *testing.T) {
	r := newRestoreRig(t)
	tg, snap := r.seedPlex(t)
	delete(r.docker.containers, "plex")

	res := r.check(t, RestoreCheckRequest{Kind: checkContainer, Name: "plex", SnapshotID: snap, BackupFirst: true})
	if b := backupFirstOf(t, res); b.Status != lineSkip || b.Reason != reasonGone || !res.Ready {
		t.Fatalf("backup first = %+v ready=%v", b, res.Ready)
	}

	if started, err := r.svc.StartRestore(context.Background(), "plex", snap, "local", RestoreOptions{BackupFirst: true}); err != nil || !started {
		t.Fatalf("StartRestore: started=%v err=%v", started, err)
	}
	r.wait(t)

	if run := r.lastRun(t, tg.ID, "restore"); run.Status != "success" {
		t.Fatalf("run = %+v", run)
	}
	if _, back := r.docker.container("plex"); !back || r.backupRuns(t, tg.ID) != 1 {
		t.Fatalf("container back=%v, backup runs=%d, want the container and no second backup", back, r.backupRuns(t, tg.ID))
	}
}

func TestRestoreVMBacksUpTheCurrentStateFirstSoTheReplaceCanBeUndone(t *testing.T) {
	r := newRestoreRig(t)
	tg, snap := r.seedWin(t)
	data := filepath.Join(r.root, "domains/win/vdisk2.img")
	atBackup := read(t, data)
	r.write(t, map[string]string{"domains/win/vdisk2.img": "the data disk moved on"})
	r.virsh.forget()

	res := r.check(t, RestoreCheckRequest{Kind: checkVM, Name: "win", SnapshotID: snap, BackupFirst: true})
	if b := backupFirstOf(t, res); b.Status != lineOK || !res.Ready {
		t.Fatalf("backup first = %+v ready=%v", b, res.Ready)
	}

	if started, err := r.svc.StartRestoreVM(context.Background(), "win", snap, "local", RestoreOptions{BackupFirst: true}); err != nil || !started {
		t.Fatalf("StartRestoreVM: started=%v err=%v", started, err)
	}
	r.wait(t)

	if got := read(t, data); got != atBackup {
		t.Fatal("the data disk is not the backed-up one after the restore")
	}
	calls := r.virsh.recorded()
	if !slices.Contains(calls, "undefine") || !slices.Contains(calls, "define") {
		t.Fatalf("libvirt calls = %v, want the domain replaced", calls)
	}
	if r.backupRuns(t, tg.ID) != 2 {
		t.Fatalf("backup runs = %d, want the seed and the one before the restore", r.backupRuns(t, tg.ID))
	}
	before := r.stateBefore(t, "vms", "vm:win", snap)

	if started, err := r.svc.StartRestoreVM(context.Background(), "win", before.ID, "local", RestoreOptions{}); err != nil || !started {
		t.Fatalf("undo: started=%v err=%v", started, err)
	}
	r.wait(t)
	if got := read(t, data); got != "the data disk moved on" {
		t.Fatalf("vdisk2.img = %q after the undo", got)
	}
	if r.backupRuns(t, tg.ID) != 2 {
		t.Fatal("a restore without the option took a backup")
	}
}

func TestRestoreFileSetBacksUpTheCurrentStateFirstSoTheReplaceCanBeUndone(t *testing.T) {
	r := newRestoreRig(t)
	set, snap := r.seedFotos(t)
	garten := filepath.Join(r.root, "fotos/2025/garten.jpg")
	r.write(t, map[string]string{"fotos/2025/garten.jpg": "bearbeitet"})

	res := r.check(t, RestoreCheckRequest{Kind: checkFileSet, Name: set.ID, SnapshotID: snap, BackupFirst: true})
	if b := backupFirstOf(t, res); b.Status != lineOK || !res.Ready {
		t.Fatalf("backup first = %+v ready=%v", b, res.Ready)
	}

	if _, started, err := r.svc.StartRestoreFileSet(context.Background(), set.ID, snap, "local", "", true, RestoreOptions{BackupFirst: true}); err != nil || !started {
		t.Fatalf("StartRestoreFileSet: started=%v err=%v", started, err)
	}
	r.wait(t)

	if got := read(t, garten); got != "garten" {
		t.Fatalf("garten.jpg = %q after the restore", got)
	}
	if r.backupRuns(t, set.ID) != 2 {
		t.Fatalf("backup runs = %d, want the seed and the one before the restore", r.backupRuns(t, set.ID))
	}
	before := r.stateBefore(t, "files", "fileset:Fotos", snap)

	if _, started, err := r.svc.StartRestoreFileSet(context.Background(), set.ID, before.ID, "local", "", true, RestoreOptions{}); err != nil || !started {
		t.Fatalf("undo: started=%v err=%v", started, err)
	}
	r.wait(t)
	if got := read(t, garten); got != "bearbeitet" {
		t.Fatalf("garten.jpg = %q after the undo", got)
	}
	if r.backupRuns(t, set.ID) != 2 {
		t.Fatal("a restore without the option took a backup")
	}
}

func TestRestoreIntoAFolderTakesNoBackupFirst(t *testing.T) {
	r := newRestoreRig(t)
	set, snap := r.seedFotos(t)

	if _, started, err := r.svc.StartRestoreFileSet(context.Background(), set.ID, snap, "local", "restore/fotos", false, RestoreOptions{BackupFirst: true}); err != nil || !started {
		t.Fatalf("StartRestoreFileSet: started=%v err=%v", started, err)
	}
	r.wait(t)

	if r.backupRuns(t, set.ID) != 1 {
		t.Fatal("a restore that replaces nothing took a backup")
	}
}

// backupFirstOf is what a check says about the backup before the restore.
func backupFirstOf(t *testing.T, res RestoreCheck) BackupFirstPlan {
	t.Helper()
	if res.Plan == nil || res.Plan.BackupFirst == nil {
		t.Fatalf("the check says nothing about a backup first: %+v", res)
	}
	return *res.Plan.BackupFirst
}

func TestBeforeRestoreTagIsTheOneTheFrontendLooksFor(t *testing.T) {
	client := read(t, filepath.Join("..", "..", "web", "src", "lib", "api.ts"))
	if want := `BEFORE_RESTORE_TAG = "` + beforeRestoreTag + `"`; !strings.Contains(client, want) {
		t.Fatalf("api.ts lacks %s, so the timeline cannot recognise the backup before a restore", want)
	}
}
