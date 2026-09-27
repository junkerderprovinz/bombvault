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
	"runtime"
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

// orphanLock reports whether a lock was left by a restic of an earlier run on
// this host whose process is gone. restic's own stale test asks only whether
// the PID is alive, and in a container that has just restarted the few PIDs
// in use are handed out again at once, so a dead run's lock often looks held.
func orphanLock(l lockFile, host string, started time.Time, ownerMayLive func(pid int) bool) bool {
	return l.Hostname == host && l.Time.Before(started) && !ownerMayLive(l.PID)
}

// removeOrphanLocks deletes the orphaned locks of a local repository, the
// same files `restic unlock` deletes for a lock it can tell is stale. A remote
// repository, and any repository where the process table cannot be read, is
// left to restic's own rule, which frees such a lock after 30 minutes.
func (r Restic) removeOrphanLocks(ctx context.Context, repo string, m Mode, started time.Time) error {
	if m.NoLock || IsRemoteRepo(repo) || runtime.GOOS != "linux" {
		return nil
	}
	host, err := os.Hostname()
	if err != nil {
		return nil
	}
	out, err := r.run(ctx, lockArgs(repo, m, "list", "locks"), m)
	if err != nil {
		return fmt.Errorf("list locks: %w", err)
	}
	var errs []error
	for _, id := range strings.Fields(string(out)) {
		if !lockIDRe.MatchString(id) {
			continue
		}
		raw, cErr := r.run(ctx, lockArgs(repo, m, "cat", "lock", id), m)
		if cErr != nil {
			// A lock released between the listing and now is no error.
			continue
		}
		var l lockFile
		if json.Unmarshal(bytes.TrimSpace(raw), &l) != nil || !orphanLock(l, host, started, ownerMayLive) {
			continue
		}
		if rmErr := os.Remove(filepath.Join(strings.TrimPrefix(repo, "local:"), "locks", id)); rmErr != nil && !errors.Is(rmErr, fs.ErrNotExist) {
			errs = append(errs, rmErr)
		}
	}
	return errors.Join(errs...)
}

// lockArgs reads the lock list without taking a lock of its own.
func lockArgs(repo string, m Mode, cmd ...string) []string {
	args := repoFlag(repo)
	args = append(args, "--no-lock")
	args = append(args, cmd...)
	if !m.Encrypted {
		args = append(args, insecureFlag)
	}
	return args
}
