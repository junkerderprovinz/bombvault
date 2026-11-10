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
	"slices"
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
	args := repoFlag(repo, m)
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

// ReasonFilesNotWritten begins the failure of a restore that left files of the
// snapshot unwritten. store.ReasonRestoreFilesNotWritten is the same sentence.
const ReasonFilesNotWritten = "restore failed: some files could not be written"

// runRestore runs a restore into target. restic counts a file it cannot write as
// restored and reports only the chmod and lchown that follow, so a run with
// nothing but such errors may have written all of its data, as on a share that
// refuses ownership, or none of it. The same restore as a dry run tells them
// apart, because it compares the target with the snapshot by content.
func (r Restic) runRestore(ctx context.Context, args []string, target string, m Mode) error {
	_, err := r.run(ctx, args, m)
	if !errors.Is(err, ErrRestoreMetadataOnly) {
		return err
	}
	unwritten, checkErr := r.unwrittenFiles(ctx, args, target, m)
	switch {
	case checkErr != nil:
		return fmt.Errorf("%s: the target could not be compared with the backup: %w", ReasonFilesNotWritten, checkErr)
	case unwritten > 0:
		return fmt.Errorf("%s: %d still hold other content or are missing", ReasonFilesNotWritten, unwritten)
	}
	return err
}

// unwrittenFiles counts the files a dry run of the restore args would still
// write. restic says "restored" for every directory and link as well, whether it
// is there or not, so that word only counts for a path that is missing.
func (r Restic) unwrittenFiles(ctx context.Context, args []string, target string, m Mode) (int, error) {
	at := slices.Index(args, "restore") + 1
	dry := slices.Insert(slices.Clone(args), at, "--dry-run", "--no-lock", "--verbose=2")
	ctx, cmd, done := r.command(ctx, dry...)
	defer done()
	cmd.Env = r.authEnv(m)
	unwritten := 0
	err := streamLines(cmd, dry, func(line []byte) {
		var msg struct {
			MessageType string `json:"message_type"`
			PreviewItem
		}
		if json.Unmarshal(bytes.TrimSpace(line), &msg) != nil || msg.MessageType != "verbose_status" {
			return
		}
		switch msg.Action {
		case "updated":
			unwritten++
		case "restored":
			if _, err := os.Lstat(filepath.Join(filepath.FromSlash(target), filepath.FromSlash(msg.Item))); errors.Is(err, fs.ErrNotExist) {
				unwritten++
			}
		}
	})
	return unwritten, ctxCancelErr(ctx, dry, err)
}

func (r Restic) preview(ctx context.Context, repo string, st PreviewStep, m Mode, onItem func(PreviewItem)) error {
	args := RestorePreviewArgs(repo, st, m)
	if err := checkRepoArgs(args); err != nil {
		return err
	}
	ctx, cmd, done := r.command(ctx, args...)
	defer done()
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
