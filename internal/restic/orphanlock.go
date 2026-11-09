package restic

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// processStart divides the locks a restic child of this process may hold from
// the locks of an earlier run.
var processStart = time.Now()

// lockFile is what `restic cat lock` prints.
type lockFile struct {
	Time     time.Time `json:"time"`
	Hostname string    `json:"hostname"`
	PID      int       `json:"pid"`
}

var lockIDRe = regexp.MustCompile(`^[0-9a-f]{64}$`)

// lockRefreshSilence is how long a lock has to go without a refresh before it
// counts as left behind. restic 0.17 writes a held lock again with a new time
// every five minutes (defaultRefreshInterval in internal/repository/lock.go);
// the rest is room for clocks that drift apart.
const lockRefreshSilence = 10 * time.Minute

// staleLockTimeout is restic's StaleLockTimeout. restic stops an operation
// whose lock it could not refresh for that long, so a lock this old protects
// nothing, whichever host wrote it.
const staleLockTimeout = 30 * time.Minute

// ErrLockHeld is what Unlock answers a request to remove every lock with while
// a lock in the repository is still being refreshed.
var ErrLockHeld = errors.New("the repository holds a lock that is still being refreshed, so a backup or prune may be running there")

// orphanLock reports whether a lock was left by a restic of an earlier run on
// this host whose process is gone. restic's own stale test asks only whether
// the PID is alive, and in a container that has just restarted the few PIDs
// in use are handed out again at once, so a dead run's lock often looks held.
// Every install runs under the hostname bombvault, so the lock of a second
// install on the same repository looks like this host's with its owner gone;
// that restic still refreshes it, and a dead one does not.
func orphanLock(l lockFile, host string, started, now time.Time, ownerMayLive func(pid int) bool) bool {
	return l.Hostname == host && l.Time.Before(started) && now.Sub(l.Time) > lockRefreshSilence && !ownerMayLive(l.PID)
}

// staleLock reports whether no running restic can rely on l any more.
func staleLock(l lockFile, host string, started, now time.Time) bool {
	return now.Sub(l.Time) > staleLockTimeout || orphanLock(l, host, started, now, ownerMayLive)
}

// clearLocks removes the stale locks of repo and leaves every other one alone.
//
// restic is not left to decide which lock is stale. It also calls a lock stale
// when it comes from its own hostname and no process with the lock's PID runs,
// and a second install on the same repository has that hostname and runs its
// restic where this one cannot see it. Its prune would lose its lock to that
// rule while it runs. So the locks of a local repository are removed as files,
// one by one, and a remote repository, whose files only restic reaches, gets
// `restic unlock` only when every lock in it is stale by the rules here. restic
// reads the locks again for that, so a lock taken in between can still go.
//
// removeAll asks for every lock to go. A lock still being refreshed stays all
// the same, and the answer is ErrLockHeld.
func (r Restic) clearLocks(ctx context.Context, repo string, removeAll bool, m Mode, started, now time.Time) error {
	if m.NoLock {
		return nil
	}
	locks, err := r.readLocks(ctx, repo, m)
	if err != nil {
		return err
	}
	host, _ := os.Hostname()
	var stale []string
	var held *repoLock
	for i, l := range locks {
		switch {
		case staleLock(l.lockFile, host, started, now):
			stale = append(stale, l.id)
		case held == nil || l.Time.After(held.Time):
			held = &locks[i]
		}
	}
	if IsRemoteRepo(repo) {
		if held == nil && len(stale) > 0 {
			_, err = r.run(ctx, UnlockArgs(repo, false, m), m)
		}
	} else {
		err = removeLocks(repo, stale)
	}
	if held != nil && removeAll {
		err = errors.Join(err, fmt.Errorf("%w (restic on %s refreshed it %s ago). It stays until it has gone 30 minutes without a refresh",
			ErrLockHeld, held.Hostname, now.Sub(held.Time).Round(time.Second)))
	}
	return err
}

// repoLock is one lock of a repository and its id.
type repoLock struct {
	id string
	lockFile
}

// readLocks reads the finished locks of repo without taking one. A lock that
// is released before it is read, or does not parse, is left out.
func (r Restic) readLocks(ctx context.Context, repo string, m Mode) ([]repoLock, error) {
	out, err := r.run(ctx, lockArgs(repo, m, "list", "locks"), m)
	if err != nil {
		return nil, fmt.Errorf("list locks: %w", err)
	}
	var locks []repoLock
	for _, id := range strings.Fields(string(out)) {
		if !lockIDRe.MatchString(id) {
			continue
		}
		raw, err := r.run(ctx, lockArgs(repo, m, "cat", "lock", id), m)
		if err != nil {
			continue
		}
		l := repoLock{id: id}
		if json.Unmarshal(bytes.TrimSpace(raw), &l.lockFile) == nil {
			locks = append(locks, l)
		}
	}
	return locks, nil
}

// removeLocks deletes the lock files ids of a local repository, the files
// `restic unlock` would delete.
func removeLocks(repo string, ids []string) error {
	var errs []error
	for _, id := range ids {
		if err := os.Remove(filepath.Join(strings.TrimPrefix(repo, "local:"), "locks", id)); err != nil && !errors.Is(err, fs.ErrNotExist) {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// lockArgs reads the lock list without taking a lock of its own.
func lockArgs(repo string, m Mode, cmd ...string) []string {
	args := repoFlag(repo, m)
	args = append(args, "--no-lock")
	args = append(args, cmd...)
	if !m.Encrypted {
		args = append(args, insecureFlag)
	}
	return args
}
