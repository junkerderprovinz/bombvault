package restic

import (
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
)

// HoldsFile reports whether a snapshot holds p as a file, a link or any other
// node that is not a folder. restic's <id>:<path> selector only takes a
// folder, so such a node is restored through the folder it lies in. It lists
// that folder rather than p, whose listing could run to every entry of a large
// folder, and takes no lock.
func (r Restic) HoldsFile(ctx context.Context, repo, snapshotID, p string, m Mode) (bool, error) {
	p = path.Clean(p)
	args := repoFlag(repo)
	args = append(args, "ls", "--json", "--no-lock")
	if !m.Encrypted {
		args = append(args, insecureFlag)
	}
	args = append(args, "--", snapshotID, path.Dir(p))
	out, err := r.run(ctx, args, m)
	if err != nil {
		return false, err
	}
	for _, e := range parseFileEntries(out) {
		if e.Path == p {
			return e.Type != "dir", nil
		}
	}
	return false, nil
}

// fileStep is a restore step that brings back the single file p from its
// folder in the snapshot into the folder dir, as the only include.
func fileStep(snapshotID, p, dir string) PreviewStep {
	return PreviewStep{SnapshotID: snapshotID, Subtree: path.Dir(p), Target: dir, Include: "/" + EscapeGlob(path.Base(p))}
}

// restoreFileTo restores the file p of a snapshot as target. Under its own
// name it is restored straight into target's folder, which leaves the rest of
// that folder alone. Under another name it goes through a scratch folder next
// to target and is renamed into place.
func (r Restic) restoreFileTo(ctx context.Context, repo, snapshotID, p, target string, m Mode) error {
	st := fileStep(snapshotID, p, path.Dir(target))
	if path.Base(target) == path.Base(p) {
		_, err := r.run(ctx, RestoreSubtreeIncludeArgs(repo, snapshotID, st.Subtree, st.Include, st.Target, m), m)
		return err
	}
	dir := filepath.FromSlash(st.Target)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	scratch, err := os.MkdirTemp(dir, ".bombvault-restore-")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(scratch) }()
	if _, err := r.run(ctx, RestoreSubtreeIncludeArgs(repo, snapshotID, st.Subtree, st.Include, filepath.ToSlash(scratch), m), m); err != nil {
		return err
	}
	// restic puts a file in the place of an empty folder and refuses a folder
	// with something in it. The rename does the same.
	to := filepath.FromSlash(target)
	if info, err := os.Lstat(to); err == nil && info.IsDir() {
		if err := os.Remove(to); err != nil {
			return fmt.Errorf("restore %s: a folder with files in it is in the way: %w", path.Base(target), err)
		}
	}
	return os.Rename(filepath.Join(scratch, path.Base(p)), to)
}

// previewFile is the dry run of restoreFileTo. It reports the file as the
// step's target itself, the item "", the way a folder's items are reported
// relative to it. A file restored under another name is compared with an empty
// folder, so it counts as added.
func (r Restic) previewFile(ctx context.Context, repo string, st PreviewStep, m Mode, onItem func(PreviewItem)) error {
	file := fileStep(st.SnapshotID, st.Subtree, path.Dir(st.Target))
	name := path.Base(st.Subtree)
	if path.Base(st.Target) != name {
		empty, err := os.MkdirTemp("", "bombvault-preview-")
		if err != nil {
			return err
		}
		defer func() { _ = os.RemoveAll(empty) }()
		file.Target = filepath.ToSlash(empty)
	}
	return r.preview(ctx, repo, file, m, func(it PreviewItem) {
		if it.Item == "/"+name {
			it.Item = ""
			onItem(it)
		}
	})
}
