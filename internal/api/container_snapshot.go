package api

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/backup"
	"github.com/junkerderprovinz/bombvault/internal/dockercli"
	"github.com/junkerderprovinz/bombvault/internal/model"
	"github.com/junkerderprovinz/bombvault/internal/notify"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
	"github.com/junkerderprovinz/bombvault/internal/zfs"
)

// snapNames hands out the names of the snapshots runs take. A name that a run
// still uses is never given out again and never swept, so a container backup
// and a dataset backup over the same tree cannot destroy each other's
// snapshot, and two runs in one second do not collide.
type snapNames struct {
	mu    sync.Mutex
	inUse map[string]bool
}

// reserve returns the first free name at or after the given instant and the
// function that frees it again, which the run calls once its snapshot is
// destroyed.
func (n *snapNames) reserve(now time.Time) (string, func()) {
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.inUse == nil {
		n.inUse = map[string]bool{}
	}
	name := zfs.SnapshotName(now)
	for n.inUse[name] {
		now = now.Add(time.Second)
		name = zfs.SnapshotName(now)
	}
	n.inUse[name] = true
	return name, sync.OnceFunc(func() {
		n.mu.Lock()
		defer n.mu.Unlock()
		delete(n.inUse, name)
	})
}

// busy reports whether a run is still using the snapshot name.
func (n *snapNames) busy(name string) bool {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.inUse[name]
}

// Reasons a container that could have been read from a snapshot is stopped
// for the whole backup instead. Where its data lies has codes of its own in
// internal/zfs.
const (
	snapFallbackNoHost      = "ssh-missing"
	snapFallbackHost        = "zfs-unreachable"
	snapFallbackView        = "view-unavailable"
	snapFallbackNotInListed = "dataset-unknown"
	snapFallbackRepo        = "repo-in-container"
)

// containerSnapPlan is where a container's data lies on ZFS: what the backup
// binds where, and the host mountpoint of every dataset involved.
type containerSnapPlan struct {
	bindings   []zfs.Binding
	mountpoint map[string]string
}

// datasets are the datasets the plan snapshots, in the order they appear.
func (p *containerSnapPlan) datasets() []string {
	var out []string
	for _, b := range p.bindings {
		if !slices.Contains(out, b.Dataset) {
			out = append(out, b.Dataset)
		}
	}
	return out
}

// planContainerSnapshot decides how a container backup reads its data. A plan
// means it is read from a snapshot. Without one, fallback says why a container
// with data on ZFS is stopped for the whole backup; it is empty when nothing
// suggested otherwise: the read is switched off, the container is not running,
// or none of its data is on ZFS.
func (s *Service) planContainerSnapshot(ctx context.Context, settings store.Settings, tg store.Target, in model.Inspect, paths []string, repo string) (*containerSnapPlan, string) {
	if !in.Running || len(paths) == 0 || !tg.SnapshotsOnZFS(settings.ContainerSnapshots) {
		return nil, ""
	}
	recs := zfsMountRecords()
	lookup := zfs.Lookup{
		Exists: func(p string) bool {
			_, err := zfsStat(p)
			return err == nil
		},
		Real: zfsRealPath,
	}
	bindings, unplaced := zfs.PlaceOnZFS(recs, paths, s.cfg.HostMountRoot, lookup)
	if unplaced != nil {
		if !anyOnZFS(recs, paths, s.cfg.HostMountRoot, lookup) {
			return nil, ""
		}
		return nil, unplaced.Code + ": " + s.toHostPath(unplaced.Path)
	}
	// restic reads the snapshot inside a view that shares only what is mounted
	// into this container, so a repository in its own layer is not there.
	if !restic.IsRemoteRepo(repo) && zfs.InOwnLayer(recs, repo) {
		return nil, snapFallbackRepo
	}

	if s.zfs == nil {
		return nil, snapFallbackNoHost
	}
	lctx, cancel := context.WithTimeout(ctx, zfsListTimeout)
	defer cancel()
	listed, err := s.zfs.List(lctx)
	if err != nil {
		return nil, snapFallbackHost + ": " + zfsErrCode(err)
	}
	plan := &containerSnapPlan{bindings: bindings, mountpoint: map[string]string{}}
	for _, ds := range plan.datasets() {
		i := slices.IndexFunc(listed, func(e zfs.ListEntry) bool { return e.Name == ds })
		if i < 0 {
			return nil, snapFallbackNotInListed + ": " + ds
		}
		if code := zfs.MemberCode(listed[i], nil); code != "" {
			return nil, code + ": " + ds
		}
		plan.mountpoint[ds] = listed[i].Mountpoint
	}

	if err := s.probeSnapView(ctx); err != nil {
		log.Printf("api: a snapshot view cannot be used on this host, containers are stopped for their whole backup: %v", err)
		return nil, snapFallbackView
	}
	s.sweepContainerSnapshots(ctx)
	return plan, ""
}

// anyOnZFS reports whether at least one of paths lies on ZFS, which is when
// it is worth saying why the container was stopped the ordinary way.
func anyOnZFS(recs []zfs.MountRecord, paths []string, mountRoot string, lookup zfs.Lookup) bool {
	for _, p := range paths {
		if _, u := zfs.PlaceOnZFS(recs, []string{p}, mountRoot, lookup); u == nil || u.Code != zfs.CodeNotOnZFS && u.Code != zfs.CodeNoBranch {
			return true
		}
	}
	return false
}

// snapViewHost is the Docker side of a snapshot view, nil when the Docker
// adapter has none.
func (s *Service) snapViewHost() dockercli.SnapViewHost {
	h, _ := s.docker.(dockercli.SnapViewHost)
	return h
}

// probeSnapView enters an empty view once per process. A host that lets this
// container create a view but not enter it, through its security profile or
// missing capabilities, is found out here, before any container goes down.
func (s *Service) probeSnapView(ctx context.Context) error {
	if s.snapViewOK.Load() {
		return nil
	}
	host := s.snapViewHost()
	if host == nil {
		return errors.New("the Docker adapter cannot start one")
	}
	name := snapViewName(s.instanceID(), "probe-"+strconv.FormatInt(time.Now().UnixNano(), 36))
	if err := host.StartSnapView(ctx, name, s.instanceID(), nil); err != nil {
		return err
	}
	defer s.removeSnapView(ctx, name)
	pid, err := findViewPID(ctx, name)
	if err != nil {
		return err
	}
	if err := snapViewEnter(viewRoot(pid)); err != nil {
		return err
	}
	s.snapViewOK.Store(true)
	return nil
}

func (s *Service) removeSnapView(ctx context.Context, name string) {
	rctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
	defer cancel()
	if err := s.snapViewHost().RemoveSnapView(rctx, name); err != nil {
		log.Printf("api: removing the snapshot view %s failed: %v", name, err)
	}
}

// snapViewName names a view after the instance and the run, so whoever looks
// at the Docker host can tell whose it is.
func snapViewName(instance, run string) string {
	return dockercli.SnapViewPrefix + truncateName(instance, 8) + "-" + run
}

// zfsRealPath is where a path lies once its symlinks are followed, or the path
// itself when they cannot be. A var so a test can stand in for the links.
var zfsRealPath = func(p string) string {
	if real, err := filepath.EvalSymlinks(p); err == nil {
		return real
	}
	return p
}

// viewPIDWait bounds the wait for a view's shell to appear, and snapViewEnter
// is the probe's way in. Both are vars so a test can run without a kernel
// that lets it chroot.
var (
	viewPIDWait   = 10 * time.Second
	snapViewEnter = enterView
)

// findViewPID finds the view's shell among this container's processes by the
// name on its command line.
func findViewPID(ctx context.Context, name string) (int, error) {
	deadline := time.Now().Add(viewPIDWait)
	for {
		entries, _ := os.ReadDir(procDir)
		for _, e := range entries {
			pid, err := strconv.Atoi(e.Name())
			if err != nil {
				continue
			}
			cmdline, err := os.ReadFile(filepath.Join(procDir, e.Name(), "cmdline")) //nolint:gosec // G304: a numbered entry of /proc
			if err == nil && slices.ContainsFunc(bytes.Split(cmdline, []byte{0}), func(arg []byte) bool { return string(arg) == name }) {
				return pid, nil
			}
		}
		if time.Now().After(deadline) {
			return 0, fmt.Errorf("the snapshot view %s did not show up among this container's processes", name)
		}
		select {
		case <-ctx.Done():
			return 0, ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
}

func viewRoot(pid int) string { return procDir + "/" + strconv.Itoa(pid) + "/root" }

// containerSnapshot is one container backup's snapshot: taken while the
// container is down, read through a view, destroyed at the end.
type containerSnapshot struct {
	svc  *Service
	plan *containerSnapPlan
	// read builds the restic seam that runs inside root.
	read func(root string) backup.Restic

	snap  string
	free  func()
	roots []string
	view  string
	root  string
}

var (
	_ backup.ContainerSnapshot = (*containerSnapshot)(nil)
	_ backup.SnapshotRead      = (*containerSnapshot)(nil)
)

// Take snapshots the plan's datasets, one atomic call per pool. It does
// nothing else, since the containers stay down until it returns.
func (c *containerSnapshot) Take(ctx context.Context) (backup.SnapshotRead, error) {
	s := c.svc
	datasets := c.plan.datasets()
	c.roots = outermost(datasets)
	c.snap, c.free = s.snapNames.reserve(time.Now())
	for _, root := range c.roots {
		if err := s.store.AddContainerSnapshotDataset(root, time.Now().Unix()); err != nil {
			log.Printf("api: recording %s for the sweep failed: %v", root, err)
		}
	}
	for _, group := range byPool(datasets) {
		sctx, cancel := context.WithTimeout(ctx, zfsSnapshotTimeout)
		err := s.zfs.Snapshot(sctx, group, c.snap)
		cancel()
		if err != nil {
			c.destroy(context.WithoutCancel(ctx))
			return nil, &backup.ZFSRefusal{Code: "snapshot-failed", Detail: strings.Join(group, ", ") + ": " + zfsDetail(err.Error())}
		}
	}
	return c, nil
}

// Backup opens the view on its first call and reads the snapshot through it.
func (c *containerSnapshot) Backup(ctx context.Context, repo string, paths, tags []string, excludes ...string) (backup.Summary, error) {
	if c.root == "" {
		if err := c.openView(ctx); err != nil {
			return backup.Summary{}, err
		}
	}
	return c.read(c.root).Backup(ctx, repo, paths, tags, excludes...)
}

// openView mounts the snapshots on the host and starts the view that lays them
// over the container's paths.
func (c *containerSnapshot) openView(ctx context.Context) error {
	s := c.svc
	var binds []dockercli.SnapViewBind
	primed := map[string]bool{}
	for _, b := range c.plan.bindings {
		mp := c.plan.mountpoint[b.Dataset]
		if !primed[b.Dataset] {
			primed[b.Dataset] = true
			pctx, cancel := context.WithTimeout(ctx, zfsPrimeTimeout)
			if err := s.zfs.Prime(pctx, mp, c.snap); err != nil {
				log.Printf("api: priming %s@%s on the host failed, leaving it to Docker: %v", b.Dataset, c.snap, err)
			}
			cancel()
		}
		binds = append(binds, dockercli.SnapViewBind{
			Source: path.Join(mp, ".zfs", "snapshot", c.snap, b.Rel),
			Target: b.Target,
		})
	}
	name := snapViewName(s.instanceID(), strings.TrimPrefix(c.snap, zfs.SnapshotPrefix))
	if err := s.snapViewHost().StartSnapView(ctx, name, s.instanceID(), binds); err != nil {
		return &backup.ZFSRefusal{Code: "view-failed", Detail: zfsDetail(err.Error())}
	}
	c.view = name
	pid, err := findViewPID(ctx, name)
	if err != nil {
		return &backup.ZFSRefusal{Code: "view-failed", Detail: zfsDetail(err.Error())}
	}
	root := viewRoot(pid)
	for _, b := range c.plan.bindings {
		if _, err := os.Stat(root + b.Target); err != nil {
			return &backup.ZFSRefusal{Code: "view-failed", Detail: s.toHostPath(b.Target) + " is not in the snapshot"}
		}
	}
	c.root = root
	return nil
}

// Release removes the view, which holds the snapshot busy, then the snapshot.
func (c *containerSnapshot) Release(ctx context.Context) {
	if c.view != "" {
		c.svc.removeSnapView(ctx, c.view)
	}
	c.destroy(ctx)
}

func (c *containerSnapshot) destroy(ctx context.Context) {
	s := c.svc
	defer c.free()
	for _, root := range c.roots {
		err := backup.DestroyRecursiveWithRetry(ctx, s.zfs, root, c.snap, zfsDestroyBudget, time.Now, sleepFor, s.IsShuttingDown())
		if err != nil {
			log.Printf("api: %s@%s could not be removed, the next sweep retries it: %v", root, c.snap, err)
			s.notifyZFSUnsuppressed(notify.Event{
				Title:   "BombVault",
				Message: "The snapshot " + root + "@" + c.snap + " could not be removed. BombVault will retry before the next backup.",
				OK:      false,
			})
			continue
		}
		if err := s.store.RemoveContainerSnapshotDataset(root); err != nil {
			log.Printf("api: forgetting %s for the sweep failed: %v", root, err)
		}
	}
}

// outermost drops every dataset another one in the list contains, since a
// recursive destroy of that one reaches it.
func outermost(datasets []string) []string {
	var out []string
	for _, d := range datasets {
		if !slices.ContainsFunc(datasets, func(o string) bool { return zfs.DescendantOf(d, o) }) {
			out = append(out, d)
		}
	}
	return out
}

// byPool groups datasets by pool, because one zfs snapshot call cannot span
// two pools.
func byPool(datasets []string) [][]string {
	var pools []string
	groups := map[string][]string{}
	for _, d := range datasets {
		pool, _, _ := strings.Cut(d, "/")
		if _, ok := groups[pool]; !ok {
			pools = append(pools, pool)
		}
		groups[pool] = append(groups[pool], d)
	}
	out := make([][]string, 0, len(pools))
	for _, p := range pools {
		out = append(out, groups[p])
	}
	return out
}

// sweepContainerSnapshots removes what an interrupted container backup left:
// views first, which hold their snapshots busy, then the snapshots on every
// dataset such a backup took one of. A dataset found clean is forgotten. It
// runs before a container backup snapshots anything, when no other one can be
// running, and at boot.
func (s *Service) sweepContainerSnapshots(ctx context.Context) {
	if host := s.snapViewHost(); host != nil {
		views, err := host.SnapViewLeftovers(ctx, s.instanceID())
		if err != nil {
			log.Printf("api: listing leftover snapshot views failed: %v", err)
		}
		for _, v := range views {
			log.Printf("api: removing the snapshot view %s an interrupted backup left", v)
			s.removeSnapView(ctx, v)
		}
	}
	if s.zfs == nil {
		return
	}
	datasets, err := s.store.ListContainerSnapshotDatasets()
	if err != nil {
		log.Printf("api: listing the datasets container backups snapshotted failed: %v", err)
		return
	}
	for _, ds := range datasets {
		remaining, err := s.sweepStamps(ctx, ds)
		switch {
		case zfs.IsNotFound(err):
		case err != nil:
			log.Printf("api: sweeping %s failed: %v", ds, err)
			continue
		case remaining > 0:
			continue
		}
		if err := s.store.RemoveContainerSnapshotDataset(ds); err != nil {
			log.Printf("api: forgetting %s for the sweep failed: %v", ds, err)
		}
	}
}

// recordSnapshotRead notes on a container backup's run how it read the data.
func (s *Service) recordSnapshotRead(runID string, window *time.Duration, fallback string) {
	if runID == "" || window == nil && fallback == "" {
		return
	}
	var ms *int64
	if window != nil {
		v := window.Milliseconds()
		ms = &v
	}
	if err := s.store.SetRunSnapshotRead(runID, ms, fallback); err != nil {
		log.Printf("api: recording how run %s read its data failed: %v", runID, err)
	}
}

// readViewChanged reports whether this run reads the data through another
// view than the target's last good backup did. restic then sees other inode
// numbers for unchanged files, so this one run compares them without.
func (s *Service) readViewChanged(targetID string, fromSnapshot bool) bool {
	last, found, err := s.store.LastBackupReadSnapshot(targetID)
	if err != nil {
		log.Printf("api: reading how the last backup of %s read its data failed: %v", targetID, err)
		return false
	}
	return found && last != fromSnapshot
}

// containerRestarts keeps the containers a backup holds down in the database.
type containerRestarts struct{ st *store.Repo }

func (r containerRestarts) Mark(names []string) error {
	return r.st.MarkContainersStopped(names, time.Now().Unix())
}

func (r containerRestarts) Clear(names []string) error { return r.st.ClearContainersStopped(names) }

// RecoverContainerRestarts starts the containers a backup left stopped when
// BombVault stopped in the middle of it. It runs at boot before the HTTP
// server and the scheduler, like RecoverZFSRestarts. A container that is gone
// is forgotten; one that does not start stays recorded for the next boot.
func (s *Service) RecoverContainerRestarts(ctx context.Context) {
	names, err := s.store.ListContainersStopped()
	if err != nil {
		log.Printf("api: could not read which containers a backup left stopped: %v", err)
		return
	}
	if len(names) == 0 {
		return
	}
	var deps []backup.StopContainer
	var gone []string
	for _, name := range names {
		in, err := s.inspectNamed(ctx, name)
		if err != nil {
			log.Printf("api: %q was left stopped by a backup and cannot be inspected, forgetting it: %v", name, err) //nolint:gosec // G706: %q-quoted
			gone = append(gone, name)
			continue
		}
		log.Printf("api: starting %q again, an interrupted backup left it stopped", name) //nolint:gosec // G706: %q-quoted
		deps = append(deps, backup.StopContainer{
			Name:       name,
			ID:         in.ID,
			WasRunning: true,
			Service:    composeService(in.Config.Labels),
			DependsOn:  parseDependsOn(in.Config.Labels),
		})
	}
	backup.RestartInOrder(ctx, s.docker, deps, false, 0)

	started := 0
	running := gone
	for _, dep := range deps {
		if in, err := s.docker.Inspect(ctx, dep.ID); err == nil && in.Running {
			running = append(running, dep.Name)
			started++
		}
	}
	if err := s.store.ClearContainersStopped(running); err != nil {
		log.Printf("api: clearing the record of the restarted containers failed: %v", err)
	}
	if started == 0 {
		return
	}
	s.notifyZFSUnsuppressed(notify.Event{
		Title:   "BombVault",
		Message: strconv.Itoa(started) + " containers were left stopped by an interrupted backup and have been started again",
		OK:      true,
	})
}

// runIDRecorder keeps the id of the backup run a container backup started, so
// the service can add to that row afterwards. The database dump starts a run
// of its own on the same seam.
type runIDRecorder struct {
	backup.Runs
	id string
}

func (r *runIDRecorder) Start(targetID, kind string) (string, error) {
	id, err := r.Runs.Start(targetID, kind)
	if err == nil && kind == "backup" {
		r.id = id
	}
	return id, err
}
