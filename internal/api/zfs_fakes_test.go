package api

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/dockercli"
	"github.com/junkerderprovinz/bombvault/internal/model"
	"github.com/junkerderprovinz/bombvault/internal/notify"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/zfs"
)

// fakeZFSHost stands in for the machine that owns the pools. It records every
// call the way the argv would read, so a test can pin the order and the exact
// snapshot name a run used.
type fakeZFSHost struct {
	mu sync.Mutex

	tree     []zfs.ListEntry
	treeErr  error
	snaps    []zfs.SnapshotEntry
	snapsErr error

	snapshotErr error
	destroyErr  error
	primeErr    error

	calls []string
	// taken are the snapshot names currently on the host, so the mount table
	// the container polls can follow what the run actually created.
	taken []string
	// onSnapshot runs after a successful recursive snapshot, for a test that
	// has to change the world mid-run.
	onSnapshot func(snap string)
	// onTree holds a tree listing, so a test can look at the world while a
	// sweep is waiting on the host.
	onTree func()
}

func (h *fakeZFSHost) record(call string) {
	h.mu.Lock()
	h.calls = append(h.calls, call)
	h.mu.Unlock()
}

func (h *fakeZFSHost) recorded() []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]string(nil), h.calls...)
}

func (h *fakeZFSHost) snapshotNames() []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]string(nil), h.taken...)
}

func (h *fakeZFSHost) Version(context.Context) (string, error) { return "zfs-2.3.4", nil }

func (h *fakeZFSHost) Tree(_ context.Context, root string) ([]zfs.ListEntry, error) {
	h.record("list -r " + root)
	if h.onTree != nil {
		h.onTree()
	}
	if h.treeErr != nil {
		return nil, h.treeErr
	}
	return h.tree, nil
}

func (h *fakeZFSHost) List(context.Context) ([]zfs.ListEntry, error) {
	h.record("list")
	return h.tree, nil
}

func (h *fakeZFSHost) Snapshots(_ context.Context, root string) ([]zfs.SnapshotEntry, error) {
	h.record("list -t snapshot " + root)
	if h.snapsErr != nil {
		return nil, h.snapsErr
	}
	return h.snaps, nil
}

func (h *fakeZFSHost) SnapshotRecursive(ctx context.Context, root, snap string) error {
	h.record("snapshot -r " + root + "@" + snap)
	if h.snapshotErr != nil {
		return h.snapshotErr
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	h.mu.Lock()
	h.taken = append(h.taken, snap)
	h.mu.Unlock()
	if h.onSnapshot != nil {
		h.onSnapshot(snap)
	}
	return nil
}

func (h *fakeZFSHost) DestroyRecursive(_ context.Context, root, snap string) error {
	h.record("destroy -r " + root + "@" + snap)
	if h.destroyErr != nil {
		return h.destroyErr
	}
	h.mu.Lock()
	kept := h.taken[:0]
	for _, s := range h.taken {
		if s != snap {
			kept = append(kept, s)
		}
	}
	h.taken = kept
	h.mu.Unlock()
	return nil
}

func (h *fakeZFSHost) SnapshotSafety(_ context.Context, dataset, snap string) error {
	h.record("snapshot " + dataset + "@" + snap)
	return nil
}

func (h *fakeZFSHost) DestroySafety(_ context.Context, dataset, snap string) error {
	h.record("destroy " + dataset + "@" + snap)
	return nil
}

func (h *fakeZFSHost) Prime(_ context.Context, hostMountpoint, snap string) error {
	h.record("stat " + hostMountpoint + "/.zfs/snapshot/" + snap)
	return h.primeErr
}

// zfsFakeEngine answers the restic calls a ZFS run makes and records what it
// was asked to read, forget and copy.
type zfsFakeEngine struct {
	ResticEngine

	mu sync.Mutex

	sum       restic.Summary
	backupErr map[string]error // keyed by the directory restic was pointed at

	backupDirs     []string
	backupTags     [][]string
	backupExcludes [][]string
	backupRepos    []string

	forgetTags   []string
	forgetPruned []bool
	prunes       int
	copies       []string
	snaps        []restic.Snapshot
}

func (e *zfsFakeEngine) RepoOpens(context.Context, string, restic.Mode) bool { return true }

func (e *zfsFakeEngine) Unlock(context.Context, string, bool, restic.Mode) error { return nil }

func (e *zfsFakeEngine) Snapshots(context.Context, string, restic.Mode) ([]restic.Snapshot, error) {
	return e.snaps, nil
}

func (e *zfsFakeEngine) BackupDir(_ context.Context, repo, dir string, tags []string, _ restic.Mode, excludes ...string) (restic.Summary, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.backupRepos = append(e.backupRepos, repo)
	e.backupDirs = append(e.backupDirs, dir)
	e.backupTags = append(e.backupTags, tags)
	e.backupExcludes = append(e.backupExcludes, excludes)
	if err := e.backupErr[dir]; err != nil {
		return restic.Summary{}, err
	}
	sum := e.sum
	if sum.SnapshotID == "" {
		sum.SnapshotID = fmt.Sprintf("zfssnap%09d", len(e.backupDirs))
	}
	return sum, nil
}

func (e *zfsFakeEngine) ForgetPolicy(_ context.Context, _ string, _ restic.RetentionPolicy, _ restic.Mode, tags []string, prune bool) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.forgetTags = append(e.forgetTags, tags...)
	e.forgetPruned = append(e.forgetPruned, prune)
	return nil
}

func (e *zfsFakeEngine) Prune(context.Context, string, restic.Mode) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.prunes++
	return nil
}

func (e *zfsFakeEngine) Copy(_ context.Context, destRepo, srcRepo string, _ []string, _ restic.Limits, _ restic.Mode) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.copies = append(e.copies, srcRepo+" -> "+destRepo)
	return nil
}

func (e *zfsFakeEngine) Init(context.Context, string, restic.Mode) error { return nil }

func (e *zfsFakeEngine) readBackupDirs() []string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([]string(nil), e.backupDirs...)
}

func (e *zfsFakeEngine) readForgetTags() []string {
	e.mu.Lock()
	defer e.mu.Unlock()
	out := append([]string(nil), e.forgetTags...)
	sort.Strings(out)
	return out
}

// zfsMountFixture points the container's mount table at a table built for a
// tree, and lets the snapshot mounts appear as the host takes them.
func zfsMountFixture(t *testing.T, base []zfs.MountRecord, host *fakeZFSHost, mounts map[string]string) {
	t.Helper()
	previous := zfsMountRecords
	t.Cleanup(func() { zfsMountRecords = previous })
	zfsMountRecords = func() []zfs.MountRecord {
		recs := append([]zfs.MountRecord(nil), base...)
		for _, snap := range host.snapshotNames() {
			for dataset, cpath := range mounts {
				recs = append(recs, zfs.MountRecord{
					MountPoint: cpath + "/.zfs/snapshot/" + snap,
					Root:       "/",
					FSType:     "zfs",
					Source:     dataset + "@" + snap,
					Options:    []string{"ro"},
					Optional:   []string{"master:6"},
				})
			}
		}
		return recs
	}
}

// zfsFakeContainer is one container the consistency window works on.
type zfsFakeContainer struct {
	id        string
	running   bool
	service   string
	dependsOn string
	// startLeavesDown makes Start succeed while the container stays down, the
	// way a container that crashes on start behaves.
	startLeavesDown bool
	stopErr         error
	startErr        error
}

// zfsFakeDocker answers the container calls a consistency window makes and
// records them in order, so a test can pin which containers went down and in
// which order they came back.
type zfsFakeDocker struct {
	dockercli.Docker

	mu sync.Mutex

	self       string
	containers map[string]*zfsFakeContainer
	calls      []string
	execCmds   []string
	execErr    error
	// onStop runs while a stop is in flight, so a test can hold two stops
	// against each other.
	onStop func(name string)
}

func newZFSFakeDocker(running ...string) *zfsFakeDocker {
	d := &zfsFakeDocker{containers: map[string]*zfsFakeContainer{}}
	for _, name := range running {
		d.containers[name] = &zfsFakeContainer{id: "id-" + name, running: true, service: name}
	}
	return d
}

func (d *zfsFakeDocker) record(call string) {
	d.mu.Lock()
	d.calls = append(d.calls, call)
	d.mu.Unlock()
}

func (d *zfsFakeDocker) recorded() []string {
	d.mu.Lock()
	defer d.mu.Unlock()
	return append([]string(nil), d.calls...)
}

// byID finds a container by the reference stop and start use.
func (d *zfsFakeDocker) byID(ref string) *zfsFakeContainer {
	for _, c := range d.containers {
		if c.id == ref {
			return c
		}
	}
	return nil
}

func (d *zfsFakeDocker) Self(context.Context) (string, error) { return d.self, nil }

func (d *zfsFakeDocker) Inspect(_ context.Context, ref string) (model.Inspect, error) {
	d.record("inspect:" + ref)
	c, name := d.containers[ref], ref
	if c == nil {
		if c = d.byID(ref); c == nil {
			return model.Inspect{}, fmt.Errorf("no such container %q", ref)
		}
		for n, known := range d.containers {
			if known == c {
				name = n
			}
		}
	}
	labels := map[string]string{"com.docker.compose.service": c.service}
	if c.dependsOn != "" {
		labels["com.docker.compose.depends_on"] = c.dependsOn
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	return model.Inspect{
		ID: c.id, Name: "/" + name, Running: c.running,
		Config: model.Config{Labels: labels},
	}, nil
}

func (d *zfsFakeDocker) Stop(_ context.Context, ref string, _ time.Duration) error {
	c := d.byID(ref)
	if c == nil {
		return fmt.Errorf("no such container %q", ref)
	}
	name := d.nameOf(c)
	d.record("stop:" + name)
	if d.onStop != nil {
		d.onStop(name)
	}
	if c.stopErr != nil {
		return c.stopErr
	}
	d.mu.Lock()
	c.running = false
	d.mu.Unlock()
	return nil
}

func (d *zfsFakeDocker) Start(_ context.Context, ref string) error {
	c := d.byID(ref)
	if c == nil {
		return fmt.Errorf("no such container %q", ref)
	}
	d.record("start:" + d.nameOf(c))
	if c.startErr != nil {
		return c.startErr
	}
	if c.startLeavesDown {
		return nil
	}
	d.mu.Lock()
	c.running = true
	d.mu.Unlock()
	return nil
}

func (d *zfsFakeDocker) Health(_ context.Context, ref string) (model.Health, error) {
	c := d.byID(ref)
	if c == nil {
		return model.Health{}, fmt.Errorf("no such container %q", ref)
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	return model.Health{Running: c.running}, nil
}

func (d *zfsFakeDocker) Exec(_ context.Context, ref string, cmd []string) error {
	d.mu.Lock()
	d.execCmds = append(d.execCmds, strings.Join(cmd, " "))
	d.mu.Unlock()
	d.record("exec:" + ref)
	return d.execErr
}

func (d *zfsFakeDocker) nameOf(c *zfsFakeContainer) string {
	for name, known := range d.containers {
		if known == c {
			return name
		}
	}
	return c.id
}

// zfsCaptureNotifications points the service's only notification channel at a
// local server and returns a reader for the bodies it received.
func zfsCaptureNotifications(t *testing.T, s *Service) func() []string {
	t.Helper()
	var mu sync.Mutex
	var bodies []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		bodies = append(bodies, string(body))
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)
	if err := s.SetNotifyConfig(notify.Config{On: "always", WebhookEnabled: true, WebhookURL: srv.URL}); err != nil {
		t.Fatalf("configure notifications: %v", err)
	}
	return func() []string {
		mu.Lock()
		defer mu.Unlock()
		return append([]string(nil), bodies...)
	}
}

func zfsAnyMessageContains(bodies []string, want string) bool {
	for _, b := range bodies {
		if strings.Contains(b, want) {
			return true
		}
	}
	return false
}
