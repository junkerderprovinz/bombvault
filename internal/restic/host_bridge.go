package restic

import (
	"context"
	"encoding/json"
	"log"
	"slices"
	"strings"
	"sync"
	"time"
)

// hostSettled holds the backups that already have a snapshot under their own
// host, so looking for a parent under LegacyHost costs one listing per item
// and process.
var hostSettled sync.Map

// ParentCandidatesArgs lists the snapshots under the mode's host or
// LegacyHost that restic would pick a backup's parent from: those with all of
// tags, or with all of paths.
func ParentCandidatesArgs(repo string, m Mode, tags, paths []string) []string {
	args := append(SnapshotsArgs(repo, m), "--host", m.backupHost(), "--host", LegacyHost)
	if len(tags) > 0 {
		args = append(args, "--tag", strings.Join(tags, ","))
	}
	for _, p := range paths {
		args = append(args, "--path", p)
	}
	return args
}

// legacyParent names the parent of the first backup under this server's own
// host: the newest snapshot under LegacyHost that restic would have picked
// before. restic looks for a parent under the backup's host only, so without
// it that run would read every file again. tags or paths select the
// candidates as restic's own search does for the backup at hand.
func (r Restic) legacyParent(ctx context.Context, repo string, m Mode, tags, paths []string) string {
	if m.backupHost() == LegacyHost {
		return ""
	}
	key := settledKey(repo, m, tags, paths)
	if _, ok := hostSettled.Load(key); ok {
		return ""
	}
	out, err := r.run(ctx, ParentCandidatesArgs(repo, m, tags, paths), m)
	if err != nil {
		log.Printf("restic: looking for a parent under host %s: %v", LegacyHost, err)
		return ""
	}
	var snaps []Snapshot
	if err := json.Unmarshal(out, &snaps); err != nil {
		log.Printf("restic: looking for a parent under host %s: parse JSON: %v", LegacyHost, err)
		return ""
	}
	parent, settled := bridgeParent(snaps, m.backupHost())
	if settled {
		hostSettled.Store(key, true)
	}
	return parent
}

// bridgeParent picks the newest snapshot under LegacyHost while none under
// host exists. settled reports that one under host does.
func bridgeParent(snaps []Snapshot, host string) (parent string, settled bool) {
	var newest time.Time
	for _, sn := range snaps {
		switch sn.Hostname {
		case host:
			return "", true
		case LegacyHost:
			if at, err := time.Parse(time.RFC3339Nano, sn.Time); err == nil && (parent == "" || at.After(newest)) {
				parent, newest = sn.ID, at
			}
		}
	}
	return parent, false
}

// settleHost records that the backup the key names has a snapshot under its
// own host.
func settleHost(repo string, m Mode, tags, paths []string) {
	if m.backupHost() != LegacyHost {
		hostSettled.Store(settledKey(repo, m, tags, paths), true)
	}
}

func settledKey(repo string, m Mode, tags, paths []string) string {
	return strings.Join([]string{repo, m.backupHost(), strings.Join(tags, ","), strings.Join(paths, "\x00")}, "\x00\x00")
}

// withParent puts --parent in front of the positionals of a backup's argv.
func withParent(args []string, parent string) []string {
	if parent == "" {
		return args
	}
	return slices.Insert(slices.Clone(args), slices.Index(args, "--"), "--parent", parent)
}
