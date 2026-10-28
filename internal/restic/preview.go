package restic

import (
	"bytes"
	"context"
	"encoding/json"
	"os/exec"
	"path/filepath"
	"strings"
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
// caller caps a preview of a very large tree. A step whose subtree is a file
// is previewed the way RestoreSubtreeTo restores it.
func (r Restic) RestorePreview(ctx context.Context, repo string, st PreviewStep, m Mode, onItem func(PreviewItem)) error {
	if st.Subtree != "" && st.Include == "" {
		file, err := r.HoldsFile(ctx, repo, st.SnapshotID, st.Subtree, m)
		if err != nil {
			return err
		}
		if file {
			return r.previewFile(ctx, repo, st, m, onItem)
		}
	}
	return r.preview(ctx, repo, st, m, onItem)
}

func (r Restic) preview(ctx context.Context, repo string, st PreviewStep, m Mode, onItem func(PreviewItem)) error {
	args := RestorePreviewArgs(repo, st, m)
	cmd := exec.CommandContext(ctx, r.bin(), args...) //nolint:gosec // G204: argv is constructed by typed builders in this package; no user input reaches here
	configureProcGroup(cmd)
	cmd.Env = r.authEnv(m)
	// restic names a deleted item by its path on disk, and reports what a
	// deleted directory holds just before the directory itself, deepest first.
	// Only the directory is passed on, relative to the target like every other
	// item, so deleted items are held back until restic is done.
	var deleted []PreviewItem
	err := streamLines(cmd, args, func(line []byte) {
		var msg struct {
			MessageType string `json:"message_type"`
			PreviewItem
		}
		if json.Unmarshal(bytes.TrimSpace(line), &msg) != nil || msg.MessageType != "verbose_status" {
			return
		}
		it := msg.PreviewItem
		if it.Action != "deleted" {
			onItem(it)
			return
		}
		if rel, err := filepath.Rel(st.Target, it.Item); err == nil && filepath.IsLocal(rel) {
			it.Item = string(filepath.Separator) + rel
		}
		inside := it.Item + string(filepath.Separator)
		for len(deleted) > 0 && strings.HasPrefix(deleted[len(deleted)-1].Item, inside) {
			deleted = deleted[:len(deleted)-1]
		}
		deleted = append(deleted, it)
	})
	for _, it := range deleted {
		onItem(it)
	}
	return ctxCancelErr(ctx, args, err)
}
