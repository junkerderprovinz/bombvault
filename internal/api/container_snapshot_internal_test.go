package api

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/backup"
	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/dockercli"
	"github.com/junkerderprovinz/bombvault/internal/model"
	"github.com/junkerderprovinz/bombvault/internal/store"
	"github.com/junkerderprovinz/bombvault/internal/zfs"
)

// The mount table of an Unraid box with a ZFS pool "cache", an XFS array disk
// and the user share over both.
const snapMountinfo = `
30 1 0:28 / / rw,relatime - overlay overlay rw
40 30 0:29 /mnt /host/user rw,relatime master:5 - rootfs rootfs rw
41 40 0:31 / /host/user/cache rw,relatime master:6 - zfs cache rw,xattr
42 41 0:32 / /host/user/cache/appdata rw,relatime master:7 - zfs cache/appdata rw,xattr
43 42 0:33 / /host/user/cache/appdata/plex rw,relatime master:8 - zfs cache/appdata/plex rw,xattr
44 40 0:34 / /host/user/disk1 rw,relatime master:9 - xfs /dev/md1p1 rw
45 40 0:35 / /host/user/user rw,relatime master:10 - fuse.shfs shfs rw
`

// snapViewDocker answers the snapshot views a backup starts: each one appears
// as a process under the fake /proc, with every bound path inside its root.
type snapViewDocker struct {
	dockercli.Docker
	proc     string
	startErr error
	// empty starts views whose root holds none of the bound paths.
	empty    bool
	mu       sync.Mutex
	started  []string
	binds    [][]dockercli.SnapViewBind
	removed  []string
	leftover []string
	nextPID  int
}

func (d *snapViewDocker) StartSnapView(_ context.Context, name, _ string, binds []dockercli.SnapViewBind) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.startErr != nil {
		return d.startErr
	}
	d.started = append(d.started, name)
	d.binds = append(d.binds, binds)
	d.nextPID++
	pid := strconv.Itoa(100 + d.nextPID)
	if err := os.MkdirAll(filepath.Join(d.proc, pid, "root"), 0o750); err != nil {
		return err
	}
	cmdline := strings.Join([]string{"sh", "-c", "while :; do sleep 3600; done", name}, "\x00") + "\x00"
	if err := os.WriteFile(filepath.Join(d.proc, pid, "cmdline"), []byte(cmdline), 0o600); err != nil {
		return err
	}
	for _, b := range binds {
		if d.empty {
			break
		}
		if err := os.MkdirAll(filepath.Join(d.proc, pid, "root", filepath.FromSlash(b.Target)), 0o750); err != nil {
			return err
		}
	}
	return nil
}

func (d *snapViewDocker) RemoveSnapView(_ context.Context, name string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.removed = append(d.removed, name)
	return nil
}

func (d *snapViewDocker) SnapViewLeftovers(context.Context, string) ([]string, error) {
	return d.leftover, nil
}

type snapFixture struct {
	svc    *Service
	st     *store.Repo
	host   *fakeZFSHost
	docker *snapViewDocker
	target store.Target
	in     model.Inspect
	repo   string
}

func newSnapFixture(t *testing.T) *snapFixture {
	t.Helper()
	st := newTestStore(t)
	host := &fakeZFSHost{tree: []zfs.ListEntry{
		zfsEntry("cache", "/mnt/cache"),
		zfsEntry("cache/appdata", "/mnt/cache/appdata"),
		zfsEntry("cache/appdata/plex", "/mnt/cache/appdata/plex"),
	}}
	proc := t.TempDir()
	docker := &snapViewDocker{proc: proc}
	s := &Service{
		store:  st,
		zfs:    host,
		docker: docker,
		cfg:    config.Config{HostMountRoot: "/host/user", HostSourceRoot: "/mnt", DataDir: t.TempDir()},
		repoMu: map[string]*sync.Mutex{"zfs": {}, "containers": {}},
	}
	tg, err := st.UpsertTarget(store.Target{ContainerName: "plex"})
	if err != nil {
		t.Fatal(err)
	}

	previousRecs, previousStat, previousReal := zfsMountRecords, zfsStat, zfsRealPath
	previousProc, previousWait, previousEnter := procDir, viewPIDWait, snapViewEnter
	t.Cleanup(func() {
		zfsMountRecords, zfsStat, zfsRealPath = previousRecs, previousStat, previousReal
		procDir, viewPIDWait, snapViewEnter = previousProc, previousWait, previousEnter
	})
	recs := zfsTestRecords(t, snapMountinfo)
	zfsMountRecords = func() []zfs.MountRecord { return recs }
	present := map[string]bool{"/host/user/cache/appdata/sonarr": true, "/host/user/cache/appdata/plex": true, "/host/user/disk1/media": true}
	zfsStat = func(p string) (fs.FileInfo, error) {
		if present[p] {
			return nil, nil
		}
		return nil, fs.ErrNotExist
	}
	zfsRealPath = func(p string) string { return p }
	procDir, viewPIDWait = proc, 50*time.Millisecond
	snapViewEnter = func(string) error { return nil }
	return &snapFixture{svc: s, st: st, host: host, docker: docker, target: tg, in: model.Inspect{Running: true}, repo: "/host/user/user/bombvault/container"}
}

func (f *snapFixture) plan(t *testing.T, paths ...string) (*containerSnapPlan, string) {
	t.Helper()
	settings, err := f.st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	return f.svc.planContainerSnapshot(t.Context(), settings, f.target, f.in, paths, f.repo)
}

func TestSnapNamesNeverHandOutANameInUse(t *testing.T) {
	var n snapNames
	at := time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC)
	first, freeFirst := n.reserve(at)
	second, _ := n.reserve(at)
	if first != "bombvault-20261015120000" || second != "bombvault-20261015120001" {
		t.Fatalf("names %s and %s, want the stamp and the next second", first, second)
	}
	if !n.busy(first) || !n.busy(second) {
		t.Fatal("a reserved name is not busy")
	}
	freeFirst()
	freeFirst()
	if n.busy(first) || !n.busy(second) {
		t.Fatal("freeing one name has to free exactly that one")
	}
	if again, _ := n.reserve(at); again != first {
		t.Fatalf("a freed name was not handed out again: %s", again)
	}
}

func TestPlanReadsAShareOnAZFSPoolFromASnapshot(t *testing.T) {
	f := newSnapFixture(t)
	plan, fallback := f.plan(t, "/host/user/user/appdata/sonarr")
	if plan == nil {
		t.Fatalf("no plan, fallback %q", fallback)
	}
	want := []zfs.Binding{{Dataset: "cache/appdata", Rel: "sonarr", Target: "/host/user/user/appdata/sonarr"}}
	if !slices.Equal(plan.bindings, want) || plan.mountpoint["cache/appdata"] != "/mnt/cache/appdata" {
		t.Fatalf("plan %+v", plan)
	}
	if len(f.docker.started) != 1 || len(f.docker.removed) != 1 || f.docker.started[0] != f.docker.removed[0] {
		t.Errorf("the probe has to start one view and remove it again: started %v, removed %v", f.docker.started, f.docker.removed)
	}
	if !f.svc.snapViewOK.Load() {
		t.Error("a probe that worked is not remembered")
	}
}

func TestPlanStaysQuietForAContainerWithNothingOnZFS(t *testing.T) {
	f := newSnapFixture(t)
	if plan, fallback := f.plan(t, "/host/user/user/media"); plan != nil || fallback != "" {
		t.Fatalf("plan %+v, fallback %q; a container on the array is stopped as before without a word", plan, fallback)
	}
	if len(f.host.recorded()) != 0 {
		t.Errorf("the host was asked about a container that has nothing on ZFS: %v", f.host.recorded())
	}
}

func TestPlanStaysQuietWhenItMustNotRead(t *testing.T) {
	cases := map[string]func(*snapFixture, *testing.T){
		"not running": func(f *snapFixture, _ *testing.T) { f.in.Running = false },
		"switched off for every container": func(f *snapFixture, t *testing.T) {
			if _, err := f.st.MutateSettings(func(s *store.Settings) error { s.ContainerSnapshots = false; return nil }); err != nil {
				t.Fatal(err)
			}
		},
		"switched off for this one": func(f *snapFixture, t *testing.T) {
			if err := f.st.SetZFSSnapshot("plex", false); err != nil {
				t.Fatal(err)
			}
			f.target, _ = f.st.GetTargetByContainer("plex")
		},
	}
	for name, setup := range cases {
		t.Run(name, func(t *testing.T) {
			f := newSnapFixture(t)
			setup(f, t)
			if plan, fallback := f.plan(t, "/host/user/user/appdata/sonarr"); plan != nil || fallback != "" {
				t.Fatalf("plan %+v, fallback %q", plan, fallback)
			}
		})
	}
}

func TestPlanReadsAContainerThatChoseItAlthoughEveryOtherDoesNot(t *testing.T) {
	f := newSnapFixture(t)
	if _, err := f.st.MutateSettings(func(s *store.Settings) error { s.ContainerSnapshots = false; return nil }); err != nil {
		t.Fatal(err)
	}
	if err := f.st.SetZFSSnapshot("plex", true); err != nil {
		t.Fatal(err)
	}
	f.target, _ = f.st.GetTargetByContainer("plex")
	if plan, fallback := f.plan(t, "/host/user/user/appdata/sonarr"); plan == nil {
		t.Fatalf("no plan, fallback %q", fallback)
	}
}

func TestPlanSaysWhyAContainerOnZFSIsStoppedAsBefore(t *testing.T) {
	cases := []struct {
		name  string
		setup func(*snapFixture)
		paths []string
		want  string
	}{
		{"part of it on the array", func(*snapFixture) {}, []string{"/host/user/user/appdata/sonarr", "/host/user/user/media"}, "not-on-zfs: /mnt/user/media"},
		{"no host", func(f *snapFixture) { f.svc.zfs = nil }, nil, snapFallbackNoHost},
		{"host unreachable", func(f *snapFixture) {
			f.host.listErr = &zfs.CmdError{Code: "ssh-unreachable"}
		}, nil, "zfs-unreachable: ssh-unreachable"},
		{"snapdir disabled", func(f *snapFixture) { f.host.tree[1].Snapdir = "disabled" }, nil, "snapdir-disabled: cache/appdata"},
		{"repository in the container's own layer", func(f *snapFixture) { f.repo = "/var/lib/bombvault/repo" }, nil, snapFallbackRepo},
		{"repository linked into the container's own layer", func(f *snapFixture) {
			zfsRealPath = func(p string) string {
				if p == f.repo {
					return "/var/lib/bombvault/repo"
				}
				return p
			}
		}, nil, snapFallbackRepo},
		{"remote repository", func(f *snapFixture) { f.repo = "s3:https://s3.example.com/bucket"; f.svc.zfs = nil }, nil, snapFallbackNoHost},
		{"no view", func(f *snapFixture) { f.docker.startErr = errors.New("no such image") }, nil, snapFallbackView},
		{"view cannot be entered", func(*snapFixture) {
			snapViewEnter = func(string) error { return errors.New("operation not permitted") }
		}, nil, snapFallbackView},
		{"chroot not permitted", func(*snapFixture) {
			snapViewEnter = func(string) error {
				return &fs.PathError{Op: "fork/exec", Path: "/bin/true", Err: syscall.EPERM}
			}
		}, nil, snapFallbackChroot},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := newSnapFixture(t)
			c.setup(f)
			paths := c.paths
			if paths == nil {
				paths = []string{"/host/user/user/appdata/sonarr"}
			}
			plan, fallback := f.plan(t, paths...)
			if plan != nil || fallback != c.want {
				t.Fatalf("plan %+v, fallback %q, want %q", plan, fallback, c.want)
			}
		})
	}
}

func snapshotFor(f *snapFixture, plan *containerSnapPlan, read func(root string) backup.Restic) *containerSnapshot {
	return &containerSnapshot{svc: f.svc, plan: plan, read: read}
}

type rootRecordingRestic struct {
	backup.Restic
	roots *[]string
	root  string
}

func (r rootRecordingRestic) Backup(context.Context, string, []string, []string, ...string) (backup.Summary, error) {
	*r.roots = append(*r.roots, r.root)
	return backup.Summary{SnapshotID: "cafe1234cafe1234"}, nil
}

func TestContainerSnapshotReadsThroughTheViewAndCleansUp(t *testing.T) {
	f := newSnapFixture(t)
	f.host.tree = append(f.host.tree, zfsEntry("fast/db", "/mnt/fast/db"))
	plan := &containerSnapPlan{
		bindings: []zfs.Binding{
			{Dataset: "cache/appdata/plex", Target: "/host/user/user/appdata/plex"},
			{Dataset: "cache/appdata/plex/db", Target: "/host/user/user/appdata/plex/db"},
			{Dataset: "fast/db", Rel: "x", Target: "/host/user/fast/db/x"},
		},
		mountpoint: map[string]string{
			"cache/appdata/plex": "/mnt/cache/appdata/plex", "cache/appdata/plex/db": "/mnt/cache/appdata/plex/db",
			"fast/db": "/mnt/fast/db",
		},
	}
	var roots []string
	c := snapshotFor(f, plan, func(root string) backup.Restic { return rootRecordingRestic{roots: &roots, root: root} })

	read, err := c.Take(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	calls := f.host.recorded()
	wantSnaps := []string{
		"snapshot cache/appdata/plex cache/appdata/plex/db@" + c.snap,
		"snapshot fast/db@" + c.snap,
	}
	for _, w := range wantSnaps {
		if !slices.Contains(calls, w) {
			t.Errorf("missing %q in %v", w, calls)
		}
	}
	if len(f.docker.started) != 0 {
		t.Error("Take must do nothing but snapshot while the containers are down")
	}
	recorded, _ := f.st.ListContainerSnapshotDatasets()
	if !slices.Equal(recorded, []string{"cache/appdata/plex", "fast/db"}) {
		t.Errorf("recorded for the sweep: %v", recorded)
	}

	if _, err := read.Backup(t.Context(), "/repo", []string{"/host/user/user/appdata/plex"}, nil); err != nil {
		t.Fatal(err)
	}
	if len(roots) != 1 || !strings.HasSuffix(roots[0], "/root") {
		t.Fatalf("restic ran in %v, want the view's root", roots)
	}
	stamp := strings.TrimPrefix(c.snap, zfs.SnapshotPrefix)
	if len(f.docker.started) != 1 || !strings.HasSuffix(f.docker.started[0], stamp) {
		t.Fatalf("views %v", f.docker.started)
	}
	wantBind := dockercli.SnapViewBind{Source: "/mnt/fast/db/.zfs/snapshot/" + c.snap + "/x", Target: "/host/user/fast/db/x"}
	if !slices.Contains(f.docker.binds[0], wantBind) {
		t.Errorf("binds %v lack %v", f.docker.binds[0], wantBind)
	}
	if !f.svc.snapNames.busy(c.snap) {
		t.Error("the name is free while the snapshot is still read")
	}

	read.Release(context.Background())
	calls = f.host.recorded()
	view := slices.Index(calls, "destroy -r cache/appdata/plex@"+c.snap)
	if view < 0 || !slices.Contains(calls, "destroy -r fast/db@"+c.snap) || slices.Contains(calls, "destroy -r cache/appdata/plex/db@"+c.snap) {
		t.Errorf("destroy has to cover each outermost dataset once: %v", calls)
	}
	if !slices.Equal(f.docker.removed, f.docker.started) {
		t.Errorf("the view was not removed: %v", f.docker.removed)
	}
	if f.svc.snapNames.busy(c.snap) {
		t.Error("the name stays reserved after the release")
	}
	if recorded, _ := f.st.ListContainerSnapshotDatasets(); len(recorded) != 0 {
		t.Errorf("destroyed datasets are still recorded for the sweep: %v", recorded)
	}
}

func TestContainerSnapshotUndoesAFailedSnapshot(t *testing.T) {
	f := newSnapFixture(t)
	f.host.snapshotErr = &zfs.CmdError{Code: "zfs-error", Stderr: "out of space"}
	plan := &containerSnapPlan{
		bindings:   []zfs.Binding{{Dataset: "cache/appdata", Rel: "sonarr", Target: "/host/user/user/appdata/sonarr"}},
		mountpoint: map[string]string{"cache/appdata": "/mnt/cache/appdata"},
	}
	c := snapshotFor(f, plan, nil)
	_, err := c.Take(t.Context())
	var refusal *backup.ZFSRefusal
	if !errors.As(err, &refusal) || refusal.Code != "snapshot-failed" || !strings.Contains(refusal.Detail, "cache/appdata") {
		t.Fatalf("err %v, want a snapshot-failed refusal naming the dataset", err)
	}
	if !slices.Contains(f.host.recorded(), "destroy -r cache/appdata@"+c.snap) {
		t.Errorf("a half-taken snapshot was left: %v", f.host.recorded())
	}
	if f.svc.snapNames.busy(c.snap) {
		t.Error("a failed snapshot keeps its name")
	}
}

func TestContainerSnapshotRefusesAViewWithoutTheData(t *testing.T) {
	f := newSnapFixture(t)
	plan := &containerSnapPlan{
		bindings:   []zfs.Binding{{Dataset: "cache/appdata", Rel: "sonarr", Target: "/host/user/user/appdata/sonarr"}},
		mountpoint: map[string]string{"cache/appdata": "/mnt/cache/appdata"},
	}
	f.docker.empty = true
	called := false
	c := snapshotFor(f, plan, func(string) backup.Restic { called = true; return nil })
	read, err := c.Take(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := read.Backup(t.Context(), "/repo", []string{"/host/user/user/appdata/sonarr"}, nil); err == nil || called {
		t.Fatalf("err %v, restic called %v; a view without the data must not be read", err, called)
	}
	read.Release(context.Background())
	if len(f.docker.removed) != 1 {
		t.Errorf("the broken view was not removed: %v", f.docker.removed)
	}
}

func TestSweepLeavesASnapshotInUseAlone(t *testing.T) {
	f := newSnapFixture(t)
	inUse, free := f.svc.snapNames.reserve(time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC))
	defer free()
	leftover := "bombvault-20261014030000"
	f.host.snaps = []zfs.SnapshotEntry{
		{Dataset: "cache/appdata", Name: inUse},
		{Dataset: "cache/appdata", Name: leftover},
	}
	remaining, err := f.svc.sweepStamps(t.Context(), "cache/appdata")
	if err != nil || remaining != 0 {
		t.Fatalf("remaining %d, err %v", remaining, err)
	}
	calls := f.host.recorded()
	if slices.Contains(calls, "destroy -r cache/appdata@"+inUse) {
		t.Errorf("the sweep destroyed a snapshot another run is reading: %v", calls)
	}
	if !slices.Contains(calls, "destroy -r cache/appdata@"+leftover) {
		t.Errorf("the leftover survived: %v", calls)
	}
}

func TestSweepContainerSnapshotsRemovesViewsFirstAndForgetsCleanDatasets(t *testing.T) {
	f := newSnapFixture(t)
	f.docker.leftover = []string{dockercli.SnapViewPrefix + "x-20261014030000"}
	if err := f.st.AddContainerSnapshotDataset("cache/appdata", 1); err != nil {
		t.Fatal(err)
	}
	f.host.snaps = []zfs.SnapshotEntry{{Dataset: "cache/appdata", Name: "bombvault-20261014030000"}}
	f.svc.sweepContainerSnapshots(t.Context())
	if !slices.Equal(f.docker.removed, f.docker.leftover) {
		t.Errorf("views removed: %v", f.docker.removed)
	}
	if !slices.Contains(f.host.recorded(), "destroy -r cache/appdata@bombvault-20261014030000") {
		t.Errorf("the leftover snapshot was not destroyed: %v", f.host.recorded())
	}
	if left, _ := f.st.ListContainerSnapshotDatasets(); len(left) != 0 {
		t.Errorf("a clean dataset is still recorded: %v", left)
	}
}

func TestSweepContainerSnapshotsKeepsADatasetWithASurvivor(t *testing.T) {
	f := newSnapFixture(t)
	if err := f.st.AddContainerSnapshotDataset("cache/appdata", 1); err != nil {
		t.Fatal(err)
	}
	f.host.snaps = []zfs.SnapshotEntry{{Dataset: "cache/appdata", Name: "bombvault-20261014030000"}}
	f.host.destroyErr = &zfs.CmdError{Code: "zfs-error"}
	f.svc.sweepContainerSnapshots(t.Context())
	if left, _ := f.st.ListContainerSnapshotDatasets(); !slices.Equal(left, []string{"cache/appdata"}) {
		t.Errorf("a dataset whose leftover survived was forgotten: %v", left)
	}
}

func TestReadViewChangedOnlyWhenTheLastGoodBackupReadTheOtherWay(t *testing.T) {
	f := newSnapFixture(t)
	if f.svc.readViewChanged(f.target.ID, true) {
		t.Fatal("a first backup has nothing to compare with")
	}
	runID, err := f.st.StartRun(f.target.ID, "backup")
	if err != nil {
		t.Fatal(err)
	}
	if err := f.st.FinishRun(runID, "success", "cafe1234", 1, ""); err != nil {
		t.Fatal(err)
	}
	window := 4 * time.Second
	f.svc.recordSnapshotRead(runID, &window, "")
	if f.svc.readViewChanged(f.target.ID, true) {
		t.Error("two snapshot reads in a row are no change")
	}
	if !f.svc.readViewChanged(f.target.ID, false) {
		t.Error("a live read after a snapshot read is a change")
	}
	run, err := f.st.GetRun(runID)
	if err != nil || run.SnapshotWindowMS == nil || *run.SnapshotWindowMS != 4000 {
		t.Fatalf("run %+v, err %v", run, err)
	}
}

func TestRecordSnapshotReadKeepsTheFallbackReason(t *testing.T) {
	f := newSnapFixture(t)
	runID, err := f.st.StartRun(f.target.ID, "backup")
	if err != nil {
		t.Fatal(err)
	}
	f.svc.recordSnapshotRead(runID, nil, "split-share: /mnt/user/appdata/x")
	run, err := f.st.GetRun(runID)
	if err != nil || run.SnapshotWindowMS != nil || run.SnapshotFallback != "split-share: /mnt/user/appdata/x" {
		t.Fatalf("run %+v, err %v", run, err)
	}
}

func restartFixture(t *testing.T, dock *zfsFakeDocker) (*Service, *store.Repo) {
	t.Helper()
	st := newTestStore(t)
	s := &Service{store: st, docker: dock, repoMu: map[string]*sync.Mutex{"containers": {}}}
	return s, st
}

func TestRecoverContainerRestartsStartsWhatABackupLeftStopped(t *testing.T) {
	dock := newZFSFakeDocker("db", "web")
	dock.containers["db"].running = false
	dock.containers["web"].running = false
	s, st := restartFixture(t, dock)
	messages := zfsCaptureNotifications(t, s)
	if err := st.MarkContainersStopped([]string{"web", "db", "gone"}, 1); err != nil {
		t.Fatal(err)
	}

	s.RecoverContainerRestarts(context.Background())

	if !dock.containers["db"].running || !dock.containers["web"].running {
		t.Fatalf("calls = %v, want both containers running", dock.recorded())
	}
	if left, _ := st.ListContainersStopped(); len(left) != 0 {
		t.Fatalf("still recorded: %v; a started or vanished container is forgotten", left)
	}
	if !zfsAnyMessageContains(messages(), "2 containers") {
		t.Fatalf("notifications = %v, want one naming how many came back", messages())
	}
}

// One attempt at boot, as after a backup: kept, a container that does not
// come up would be started at every later boot, after somebody left it off.
func TestRecoverContainerRestartsTriesOnceAndSaysWhatStayedDown(t *testing.T) {
	dock := newZFSFakeDocker("db", "web")
	dock.containers["db"].running = false
	dock.containers["web"].running = false
	dock.containers["web"].startLeavesDown = true
	s, st := restartFixture(t, dock)
	messages := zfsCaptureNotifications(t, s)
	if err := st.MarkContainersStopped([]string{"db", "web"}, 1); err != nil {
		t.Fatal(err)
	}

	s.RecoverContainerRestarts(context.Background())

	if left, _ := st.ListContainersStopped(); len(left) != 0 {
		t.Fatalf("recorded %v, want nothing kept for the next boot", left)
	}
	if !zfsAnyMessageContains(messages(), "did not start again: web") {
		t.Fatalf("notifications = %v, want web named as still down", messages())
	}
}

func TestRecoverContainerRestartsForgetsAContainerThatRunsAlready(t *testing.T) {
	dock := newZFSFakeDocker("web")
	s, st := restartFixture(t, dock)
	messages := zfsCaptureNotifications(t, s)
	if err := st.MarkContainersStopped([]string{"web"}, 1); err != nil {
		t.Fatal(err)
	}

	s.RecoverContainerRestarts(context.Background())

	if slices.Contains(dock.recorded(), "start:web") {
		t.Fatalf("calls = %v, a running container was started again", dock.recorded())
	}
	if left, _ := st.ListContainersStopped(); len(left) != 0 {
		t.Fatalf("still recorded: %v", left)
	}
	if got := messages(); len(got) != 0 {
		t.Fatalf("notifications = %v, nothing was left stopped", got)
	}
}

type startOnlyRuns struct {
	backup.Runs
	n int
}

func (r *startOnlyRuns) Start(_, kind string) (string, error) {
	r.n++
	return kind + "-" + strconv.Itoa(r.n), nil
}

func TestRunIDRecorderKeepsTheBackupRunAlthoughTheDumpStartsLater(t *testing.T) {
	r := &runIDRecorder{Runs: &startOnlyRuns{}}
	if _, err := r.Start("t1", "backup"); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Start("t1", "dbdump"); err != nil {
		t.Fatal(err)
	}
	if r.id != "backup-1" {
		t.Fatalf("id = %q, want the backup run; the window belongs on it", r.id)
	}
}
