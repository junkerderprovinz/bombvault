package restic

import (
	"bytes"
	"context"
	"encoding/json"
	"os/exec"
)

// PreviewStep is one restore call as its dry run replays it: the same
// snapshot, subtree root, target, include and excludes the real call uses.
type PreviewStep struct {
	SnapshotID string
	// Subtree roots the restore at <id>:<subtree>; empty restores from the
	// snapshot's own root.
	Subtree  string
	Target   string
	Include  string
	Excludes []string
}

// PreviewItem is one line of a restore dry run. Item is relative to the
// target. Action is one of restored, updated, unchanged or deleted; a
// deleted item exists in the target and not in the snapshot.
type PreviewItem struct {
	Action string `json:"action"`
	Item   string `json:"item"`
	Size   int64  `json:"size"`
}

// RestorePreviewArgs builds a restore that only reports. --dry-run comes first
// and is unconditional: --delete makes restic list the files the target holds
// beyond the snapshot, which is what a preview wants and what a real restore
// here never does. --overwrite if-changed compares size and mtime, so the dry
// run stats the live files and never reads them. It takes no lock, like
// every other read of a repository that may be in use.
func RestorePreviewArgs(repo string, st PreviewStep, m Mode) []string {
	args := repoFlag(repo)
	args = append(args, "restore", "--dry-run", "--delete", "--no-lock")
	if !m.Encrypted {
		args = append(args, insecureFlag)
	}
	args = append(args, "--json", "--verbose=2", "--overwrite", "if-changed", "--target", st.Target)
	if st.Include != "" {
		args = append(args, "--include", st.Include)
	}
	for _, p := range st.Excludes {
		args = append(args, "--exclude", p)
	}
	sel := st.SnapshotID
	if st.Subtree != "" {
		sel += ":" + st.Subtree
	}
	return append(args, "--", sel)
}

// RestorePreview runs the dry run of one restore step and hands each item to
// onItem as restic reports it. Cancelling ctx stops restic, which is how a
// caller caps a preview of a very large tree.
func (r Restic) RestorePreview(ctx context.Context, repo string, st PreviewStep, m Mode, onItem func(PreviewItem)) error {
	args := RestorePreviewArgs(repo, st, m)
	cmd := exec.CommandContext(ctx, r.bin(), args...) //nolint:gosec // G204: argv is constructed by typed builders in this package; no user input reaches here
	configureProcGroup(cmd)
	cmd.Env = r.authEnv(m)
	err := streamLines(cmd, args, func(line []byte) {
		var msg struct {
			MessageType string `json:"message_type"`
			PreviewItem
		}
		if json.Unmarshal(bytes.TrimSpace(line), &msg) != nil || msg.MessageType != "verbose_status" {
			return
		}
		onItem(msg.PreviewItem)
	})
	return ctxCancelErr(ctx, args, err)
}
