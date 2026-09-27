package api

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/backup"
	"github.com/junkerderprovinz/bombvault/internal/notify"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// zfsStopTimeout is what one container gets to shut down gracefully, the same
// grace a container backup gives.
const zfsStopTimeout = 30 * time.Second

// zfsStopKillMargin is what the stop call gets beyond the grace. The daemon
// answers only once the container has exited, which for an app that ignores
// SIGTERM is after the kill at the end of the grace.
const zfsStopKillMargin = 15 * time.Second

// zfsManualLockWait is how long a run a user started waits for the containers
// domain. It is a var so tests can reach the timeout without waiting it out.
var zfsManualLockWait = 30 * time.Minute

// zfsLockWait is how long a run may wait for the containers domain. A scheduled
// run waits as long as it may itself take, so a nightly pass over both domains
// does not fail only because a long container backup started in the same
// minute; a user watching a manual run gets an answer sooner.
func zfsLockWait(ctx context.Context) time.Duration {
	if notify.MessagesSuppressed(ctx) {
		return backupHardCap()
	}
	return zfsManualLockWait
}

// lockWithin waits up to wait for a domain's lock and gives up cleanly, also
// when ctx ends first. Go hands a mutex over to a waiter that has been queued
// for more than a millisecond before it hands it to a fresh contender, so the
// window slots in between two container backups instead of waiting out the
// whole batch. A wait of zero or less blocks until the lock is free.
func (s *Service) lockWithin(ctx context.Context, domain, reason string, wait time.Duration) (func(), bool) {
	if wait <= 0 {
		return s.lockDomainFor(domain, reason), true
	}
	mu := s.repoMu[domain]
	if mu == nil {
		return func() {}, true
	}
	got := make(chan struct{})
	go func() {
		mu.Lock()
		close(got)
	}()
	timer := time.NewTimer(wait)
	defer timer.Stop()
	select {
	case <-got:
		s.setDomainActivity(domain, reason)
		return func() {
			s.clearDomainActivity(domain)
			mu.Unlock()
		}, true
	case <-timer.C:
	case <-ctx.Done():
	}
	go func() {
		<-got
		mu.Unlock()
	}()
	return nil, false
}

// zfsConsistency holds an item's containers down for the snapshot instant, so
// the snapshot catches the applications between writes rather than mid-write.
type zfsConsistency struct {
	svc      *Service
	item     store.ZFSDataset
	settings store.Settings
	wait     time.Duration

	unlock    func()
	stopped   []backup.StopContainer
	firstStop time.Time
	window    time.Duration
}

// newZFSConsistency builds the consistency window of an item that stops
// containers. An item that stops nothing gets none, and its snapshot is taken
// without Docker being touched at all.
func (s *Service) newZFSConsistency(d store.ZFSDataset, settings store.Settings, wait time.Duration) backup.ZFSConsistency {
	if len(d.StopContainers) == 0 {
		return nil
	}
	return &zfsConsistency{svc: s, item: d, settings: settings, wait: wait}
}

func (c *zfsConsistency) Freeze(ctx context.Context) (func(context.Context), func() time.Duration, error) {
	// The lock comes before the look at the stop list: a container backup
	// stops its container under this lock and starts it again before letting
	// go, so a container seen down from outside may be running by the time the
	// snapshot is taken.
	unlock, ok := c.svc.lockWithin(ctx, "containers", "zfs-consistency", c.wait)
	if !ok {
		if ctx.Err() != nil {
			return nil, nil, ctx.Err()
		}
		return nil, nil, &backup.ZFSRefusal{Code: "containers-busy", Detail: c.item.Dataset}
	}
	deps, err := c.running(ctx)
	if err != nil {
		unlock()
		return nil, nil, err
	}
	if len(deps) == 0 {
		unlock()
		return func(context.Context) {}, func() time.Duration { return 0 }, nil
	}
	c.unlock = unlock

	// Whatever happens from here on, the row says which containers are down, so
	// a process that dies inside the window can start them again at boot.
	names := make([]string, len(deps))
	for i, dep := range deps {
		names[i] = dep.Name
	}
	if err := c.svc.store.SetZFSRestartPending(c.item.ID, names); err != nil {
		c.unlock()
		return nil, nil, &backup.ZFSRefusal{Code: "consistency-stop-failed", Detail: err.Error()}
	}

	if stopErr := c.stop(ctx, deps); stopErr != nil {
		c.thaw(context.WithoutCancel(ctx))
		return nil, nil, stopErr
	}
	return c.thaw, func() time.Duration { return c.window }, nil
}

// running inspects the stop list and keeps the containers that are up. A
// container the user stopped stays stopped.
func (c *zfsConsistency) running(ctx context.Context) ([]backup.StopContainer, error) {
	self := c.svc.selfContainerName(ctx)
	var deps []backup.StopContainer
	for _, name := range c.item.StopContainers {
		if name == self {
			return nil, &backup.ZFSRefusal{Code: "container-is-self", Detail: name}
		}
		in, err := c.svc.inspectNamed(ctx, name)
		if err != nil {
			return nil, &backup.ZFSRefusal{Code: "container-unknown", Detail: name}
		}
		if !in.Running {
			continue
		}
		deps = append(deps, backup.StopContainer{
			Name:       name,
			ID:         in.ID,
			WasRunning: true,
			Service:    composeService(in.Config.Labels),
			DependsOn:  parseDependsOn(in.Config.Labels),
		})
	}
	return deps, nil
}

// stop takes the containers down one dependency level at a time, dependents
// first, and records what went down so a failure can put it back.
func (c *zfsConsistency) stop(ctx context.Context, deps []backup.StopContainer) error {
	c.firstStop = time.Now()
	for _, level := range backup.StopLevels(deps) {
		var mu sync.Mutex
		var wg sync.WaitGroup
		var failed []string
		for _, i := range level {
			wg.Add(1)
			go func(dep backup.StopContainer) {
				defer wg.Done()
				down, err := c.stopOne(ctx, dep)
				mu.Lock()
				defer mu.Unlock()
				if down {
					c.stopped = append(c.stopped, dep)
				}
				if err != nil {
					log.Printf("api: zfs: stopping %q for the snapshot of %s failed: %v", dep.Name, c.item.Dataset, err) //nolint:gosec // G706: %q-quoted
					failed = append(failed, dep.Name)
				}
			}(deps[i])
		}
		wg.Wait()
		if len(failed) > 0 {
			return &backup.ZFSRefusal{Code: "consistency-stop-failed", Detail: strings.Join(failed, ", ")}
		}
	}
	return nil
}

// stopOne stops one container and reports whether it may be down, which is
// what thaw has to start again. The stop runs on a context no cancel reaches,
// because the daemon carries on with a stop its client gave up on; a failed
// call is therefore checked against the container itself.
func (c *zfsConsistency) stopOne(ctx context.Context, dep backup.StopContainer) (bool, error) {
	detached := context.WithoutCancel(ctx)
	sctx, cancel := context.WithTimeout(detached, zfsStopTimeout+zfsStopKillMargin)
	defer cancel()
	err := c.svc.docker.Stop(sctx, dep.ID, zfsStopTimeout)
	if err == nil {
		return true, nil
	}
	in, iErr := c.svc.docker.Inspect(detached, dep.ID)
	if iErr != nil {
		return true, err
	}
	if !in.Running {
		log.Printf("api: zfs: %q is stopped although the stop call reported %v", dep.Name, err) //nolint:gosec // G706: %q-quoted
		return true, nil
	}
	return false, err
}

// thaw starts the containers again in dependency order and releases the
// containers domain. It runs on a context nothing cancels, so neither a user
// cancel nor a shutdown can leave a user's applications down.
func (c *zfsConsistency) thaw(ctx context.Context) {
	defer c.unlock()
	backup.RestartInOrder(ctx, c.svc.docker, c.stopped, c.settings.RestartHealthWait,
		time.Duration(c.settings.RestartHealthTimeoutSec)*time.Second)
	c.window = time.Since(c.firstStop)

	var down []string
	for _, dep := range c.stopped {
		in, err := c.svc.docker.Inspect(ctx, dep.ID)
		if err != nil || !in.Running {
			down = append(down, dep.Name)
		}
	}
	if len(down) == 0 {
		if err := c.svc.store.ClearZFSRestartPending(c.item.ID); err != nil {
			log.Printf("api: zfs: clearing the restart marker of %s failed: %v", c.item.Dataset, err)
		}
		return
	}
	if err := c.svc.store.SetZFSRestartPending(c.item.ID, down); err != nil {
		log.Printf("api: zfs: recording the containers still down after %s failed: %v", c.item.Dataset, err)
	}
	log.Printf("api: zfs: %s did not start again after the snapshot of %s", strings.Join(down, ", "), c.item.Dataset)
	c.svc.notifyZFSUnsuppressed(notify.Event{
		Title:   "BombVault",
		Message: "These containers did not start again after the snapshot of " + c.item.Dataset + ": " + strings.Join(down, ", "),
		OK:      false,
	})
}

// zfsHooks runs an item's pre- and post-snapshot commands in its hook
// container, the way the container domain runs its own hooks.
type zfsHooks struct {
	svc       *Service
	container string
	pre, post string
}

// newZFSHooks builds the hook pair of an item that has a command to run.
func (s *Service) newZFSHooks(d store.ZFSDataset) backup.ZFSHooks {
	if d.HookContainer == "" || (d.PreSnapshot == "" && d.PostSnapshot == "") {
		return nil
	}
	return &zfsHooks{svc: s, container: d.HookContainer, pre: d.PreSnapshot, post: d.PostSnapshot}
}

func (h *zfsHooks) Pre(ctx context.Context) error { return h.exec(ctx, h.pre) }

func (h *zfsHooks) Post(ctx context.Context) error { return h.exec(ctx, h.post) }

func (h *zfsHooks) exec(ctx context.Context, cmd string) error {
	if cmd == "" {
		return nil
	}
	in, err := h.svc.inspectNamed(ctx, h.container)
	if err != nil {
		return fmt.Errorf("%s: %w", h.container, err)
	}
	if !in.Running {
		return fmt.Errorf("%s is not running", h.container)
	}
	return h.svc.docker.Exec(ctx, in.ID, []string{"sh", "-c", cmd})
}

// RecoverZFSRestarts starts the containers a consistency window left stopped.
// It runs at boot before the HTTP server and the scheduler, because a user's
// applications being down outranks everything else this process has to do.
func (s *Service) RecoverZFSRestarts(ctx context.Context) {
	rows, err := s.store.ListZFSRestartPending()
	if err != nil {
		log.Printf("api: zfs: could not read which containers a snapshot left stopped: %v", err)
		return
	}
	if len(rows) == 0 {
		log.Print("api: zfs: no containers were left stopped by a dataset snapshot")
		return
	}
	recovered := 0
	for _, d := range rows {
		recovered += s.recoverItemRestarts(ctx, d)
	}
	if recovered == 0 {
		return
	}
	s.notifyZFSUnsuppressed(notify.Event{
		Title: "BombVault",
		Message: strconv.Itoa(recovered) +
			" containers were left stopped by an interrupted dataset snapshot and have been started again",
		OK: true,
	})
}

// recoverItemRestarts starts one item's marked containers and returns how many
// are running again. A name that does not come up stays on the marker, so the
// page keeps showing it and the next boot tries again.
func (s *Service) recoverItemRestarts(ctx context.Context, d store.ZFSDataset) int {
	var deps []backup.StopContainer
	var down []string
	for _, name := range d.RestartPending {
		in, err := s.inspectNamed(ctx, name)
		if err != nil {
			log.Printf("api: zfs: %q was left stopped by a snapshot of %s and cannot be inspected: %v", name, d.Dataset, err) //nolint:gosec // G706: %q-quoted
			down = append(down, name)
			continue
		}
		log.Printf("api: zfs: starting %q again, an interrupted snapshot of %s left it stopped", name, d.Dataset) //nolint:gosec // G706: %q-quoted
		deps = append(deps, backup.StopContainer{
			Name:       name,
			ID:         in.ID,
			WasRunning: true,
			Service:    composeService(in.Config.Labels),
			DependsOn:  parseDependsOn(in.Config.Labels),
		})
	}
	backup.RestartInOrder(ctx, s.docker, deps, false, 0)

	for _, dep := range deps {
		in, err := s.docker.Inspect(ctx, dep.ID)
		if err != nil || !in.Running {
			down = append(down, dep.Name)
		}
	}
	if len(down) == 0 {
		if err := s.store.ClearZFSRestartPending(d.ID); err != nil {
			log.Printf("api: zfs: clearing the restart marker of %s failed: %v", d.Dataset, err)
		}
	} else if err := s.store.SetZFSRestartPending(d.ID, down); err != nil {
		log.Printf("api: zfs: recording the containers still down on %s failed: %v", d.Dataset, err)
	}
	return len(d.RestartPending) - len(down)
}
