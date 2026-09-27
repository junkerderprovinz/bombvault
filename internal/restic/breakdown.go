package restic

import (
	"bytes"
	"context"
	"encoding/json"
	"os/exec"
)

// LsNoLockArgs is LsArgs without a repository lock, for a read that runs
// beside backups and prunes and must not hold them up. A prune that removes a
// pack under it makes the listing fail, never return wrong data.
func LsNoLockArgs(repo, snapshotID string, m Mode) []string {
	args := repoFlag(repo)
	args = append(args, "ls", "--no-lock", "--json")
	if !m.Encrypted {
		args = append(args, insecureFlag)
	}
	return append(args, "--", snapshotID)
}

// LsStreamNoLock is LsStream without a repository lock.
func (r Restic) LsStreamNoLock(ctx context.Context, repo, snapshotID string, m Mode, onEntry func(FileEntry)) error {
	args := LsNoLockArgs(repo, snapshotID, m)
	cmd := exec.CommandContext(ctx, r.bin(), args...) //nolint:gosec // G204: argv is constructed by typed builders in this package; no user input reaches here
	configureProcGroup(cmd)
	cmd.Env = r.authEnv(m)
	err := streamLines(cmd, args, func(line []byte) {
		var e FileEntry
		if json.Unmarshal(bytes.TrimSpace(line), &e) != nil || e.Path == "" || e.Path == "/" {
			return
		}
		onEntry(e)
	})
	return ctxCancelErr(ctx, args, err)
}

// DiffChange is one path restic diff reports. Modifier is "+" for added,
// "-" for removed, and for a path in both a combination of "T" (type), "M"
// (content) and "U" (metadata only). A directory's path ends in a slash.
type DiffChange struct {
	Path     string `json:"path"`
	Modifier string `json:"modifier"`
}

// DiffStream runs restic diff between two snapshots and hands each change to
// onChange as it is read. restic only descends into trees that differ, so a
// small change costs little however large the snapshots are.
func (r Restic) DiffStream(ctx context.Context, repo, snap1, snap2 string, m Mode, onChange func(DiffChange)) error {
	args := DiffArgs(repo, snap1, snap2, m)
	cmd := exec.CommandContext(ctx, r.bin(), args...) //nolint:gosec // G204: argv is constructed by typed builders in this package; no user input reaches here
	configureProcGroup(cmd)
	cmd.Env = r.authEnv(m)
	err := streamLines(cmd, args, func(line []byte) {
		var c struct {
			MessageType string `json:"message_type"`
			DiffChange
		}
		if json.Unmarshal(bytes.TrimSpace(line), &c) != nil || c.MessageType != "change" {
			return
		}
		onChange(c.DiffChange)
	})
	return ctxCancelErr(ctx, args, err)
}
