package api

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"testing"

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
