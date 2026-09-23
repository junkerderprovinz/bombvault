package restic

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
)

// zfsdirRepo prepares a throwaway repository for the contract tests below and
// returns the engine, the repository location and the unencrypted mode they all
// use. It skips when restic is not installed.
func zfsdirRepo(t *testing.T) (Restic, string, Mode) {
	t.Helper()
	if _, err := exec.LookPath("restic"); err != nil {
		t.Skip("restic not on PATH")
	}
	r := Restic{Bin: "restic"}
	m := Mode{}
	repo := filepath.Join(t.TempDir(), "repo")
	if err := r.Init(context.Background(), repo, m); err != nil {
		t.Fatalf("init repository: %v", err)
	}
	return r, repo, m
}

// zfsdirTree writes a small tree of files below dir, creating the parents.
func zfsdirTree(t *testing.T, dir string, files ...string) {
	t.Helper()
	for _, f := range files {
		p := filepath.Join(dir, filepath.FromSlash(f))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil { //nolint:gosec // G301: test temp dir
			t.Fatalf("mkdir %s: %v", filepath.Dir(p), err)
		}
		if err := os.WriteFile(p, []byte(f), 0o644); err != nil { //nolint:gosec // G306: test file
			t.Fatalf("write %s: %v", p, err)
		}
	}
}

// zfsdirEntries lists the file paths a snapshot holds, sorted.
func zfsdirEntries(t *testing.T, r Restic, repo, snapshotID string, m Mode) []string {
	t.Helper()
	entries, err := r.Ls(context.Background(), repo, snapshotID, m)
	if err != nil {
		t.Fatalf("ls %s: %v", snapshotID, err)
	}
	var paths []string
	for _, e := range entries {
		if e.Type == "file" {
			paths = append(paths, e.Path)
		}
	}
	sort.Strings(paths)
	return paths
}

// TestBackupDirStoresEntriesAtSnapshotRoot checks that a backup taken inside a
// directory on the positional "." puts that directory's entries at the snapshot
// root. That is what keeps a member's tree root stable although the snapshot
// directory carries a different name every run.
func TestBackupDirStoresEntriesAtSnapshotRoot(t *testing.T) {
	r, repo, m := zfsdirRepo(t)
	dir := t.TempDir()
	zfsdirTree(t, dir, "a.txt", "sub/big.bin")

	sum, err := r.BackupDir(context.Background(), repo, dir, []string{"zfs:tank/data"}, m)
	if err != nil {
		t.Fatalf("BackupDir: %v", err)
	}
	got := zfsdirEntries(t, r, repo, sum.SnapshotID, m)
	want := []string{"/a.txt", "/sub/big.bin"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("snapshot holds %v, want %v", got, want)
	}
}

// TestBackupDirFindsParentByTagAcrossPaths checks that the second run of a
// dataset reads nothing although its absolute path differs, the situation every
// night because the snapshot directory carries the run's timestamp. Two
// symlinks to one directory reproduce it, identical inodes and ctimes included;
// the control run at the end shows that without the tag grouping restic finds
// no parent at all.
func TestBackupDirFindsParentByTagAcrossPaths(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("needs symlinks to reach one directory through two paths")
	}
	r, repo, m := zfsdirRepo(t)
	tmp := t.TempDir()
	src := filepath.Join(tmp, "src")
	zfsdirTree(t, src, "a.txt", "sub/big.bin")
	links := make([]string, 3)
	for i, name := range []string{"first", "second", "third"} {
		links[i] = filepath.Join(tmp, name)
		if err := os.Symlink(src, links[i]); err != nil {
			t.Fatalf("symlink %s: %v", name, err)
		}
	}

	ctx := context.Background()
	tags := []string{"zfs:tank/data"}
	first, err := r.BackupDir(ctx, repo, links[0], tags, m)
	if err != nil {
		t.Fatalf("first BackupDir: %v", err)
	}
	if first.FilesNew != 2 {
		t.Fatalf("the first run stored %d new files, want both", first.FilesNew)
	}

	second, err := r.BackupDir(ctx, repo, links[1], tags, m)
	if err != nil {
		t.Fatalf("second BackupDir: %v", err)
	}
	if second.FilesNew != 0 || second.FilesChanged != 0 {
		t.Fatalf("the second run re-read files (%d new, %d changed), so it found no parent", second.FilesNew, second.FilesChanged)
	}
	if second.FilesUnmodified < 2 || second.BytesAdded != 0 {
		t.Fatalf("the second run reports %d unmodified files and %v bytes added, want both files unmodified and nothing stored",
			second.FilesUnmodified, second.BytesAdded)
	}

	snaps, err := r.Snapshots(ctx, repo, m)
	if err != nil {
		t.Fatalf("snapshots: %v", err)
	}
	pathOf := func(id string) string {
		for _, s := range snaps {
			if strings.HasPrefix(s.ID, id) && len(s.Paths) > 0 {
				return s.Paths[0]
			}
		}
		t.Fatalf("snapshot %s is not in the repository listing", id)
		return ""
	}
	if pathOf(first.SnapshotID) == pathOf(second.SnapshotID) {
		t.Fatalf("both runs recorded the path %q, so the parent was not found across differing paths", pathOf(first.SnapshotID))
	}

	out, err := r.runIn(ctx, links[2], BackupArgs(repo, []string{"."}, tags, m), m)
	if err != nil {
		t.Fatalf("control BackupDir without the tag grouping: %v\n%s", err, out)
	}
	control, err := ParseBackupSummary(out)
	if err != nil {
		t.Fatalf("parse control summary: %v\n%s", err, out)
	}
	if control.FilesNew == 0 {
		t.Fatal("a run without --group-by found a parent, so the flag is not what makes parent detection work")
	}
}

// TestBackupDirParentIsPerDatasetTag checks that a snapshot of another dataset
// never becomes the parent of this one. Members of one item share a repository
// and a host, so only the identity tag keeps their histories apart.
func TestBackupDirParentIsPerDatasetTag(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("needs symlinks to reach one directory through two paths")
	}
	r, repo, m := zfsdirRepo(t)
	tmp := t.TempDir()
	src := filepath.Join(tmp, "src")
	zfsdirTree(t, src, "a.txt", "sub/big.bin")
	first := filepath.Join(tmp, "first")
	second := filepath.Join(tmp, "second")
	for _, l := range []string{first, second} {
		if err := os.Symlink(src, l); err != nil {
			t.Fatalf("symlink %s: %v", l, err)
		}
	}

	ctx := context.Background()
	if _, err := r.BackupDir(ctx, repo, first, []string{"zfs:tank/data"}, m); err != nil {
		t.Fatalf("BackupDir of the first dataset: %v", err)
	}
	other, err := r.BackupDir(ctx, repo, second, []string{"zfs:tank/other"}, m)
	if err != nil {
		t.Fatalf("BackupDir of the second dataset: %v", err)
	}
	if other.FilesNew != 2 {
		t.Fatalf("the second dataset stored %d new files, want both: it took the first dataset's snapshot as its parent", other.FilesNew)
	}
}

// zfsdirGlobEscape escapes what restic's pattern matcher reads as glob syntax,
// so a mountpoint such as "/mnt/cache/Media [old]" still anchors a pattern.
var zfsdirGlobEscape = strings.NewReplacer(`\`, `\\`, `*`, `\*`, `?`, `\?`, `[`, `\[`)

// TestBackupDirAnchoredExcludeIsRelativeToCwd checks that an exclude anchored at
// the snapshot directory hits only the entry at the tree root, not the same name
// further down. restic matches excludes against absolute paths, so an anchored
// pattern has to carry the snapshot directory itself.
func TestBackupDirAnchoredExcludeIsRelativeToCwd(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("an anchored pattern is a posix path; restic reads the backslash as an escape")
	}
	r, repo, m := zfsdirRepo(t)
	dir := t.TempDir()
	zfsdirTree(t, dir, "sub/x.txt", "other/sub/y.txt")

	sum, err := r.BackupDir(context.Background(), repo, dir, []string{"zfs:tank/data"}, m, dir+"/sub")
	if err != nil {
		t.Fatalf("BackupDir: %v", err)
	}
	got := zfsdirEntries(t, r, repo, sum.SnapshotID, m)
	want := []string{"/other/sub/y.txt"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("snapshot holds %v, want %v", got, want)
	}
}

// TestBackupDirAnchoredExcludeWithGlobCharsInDir checks that a snapshot
// directory whose name carries glob characters still anchors once they are
// escaped. An Unraid share may well be called "Media [old]".
func TestBackupDirAnchoredExcludeWithGlobCharsInDir(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("an anchored pattern is a posix path; restic reads the backslash as an escape")
	}
	r, repo, m := zfsdirRepo(t)
	dir := filepath.Join(t.TempDir(), "x [old]")
	zfsdirTree(t, dir, "sub/x.txt", "keep.txt")

	sum, err := r.BackupDir(context.Background(), repo, dir, []string{"zfs:tank/data"}, m,
		zfsdirGlobEscape.Replace(dir)+"/sub")
	if err != nil {
		t.Fatalf("BackupDir: %v", err)
	}
	got := zfsdirEntries(t, r, repo, sum.SnapshotID, m)
	want := []string{"/keep.txt"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("snapshot holds %v, want %v", got, want)
	}
}

// TestBackupDirUnanchoredExcludeMatchesAnywhere checks that a bare name hits
// that name at every depth, which is what a pattern without a slash promises in
// the excludes editor.
func TestBackupDirUnanchoredExcludeMatchesAnywhere(t *testing.T) {
	r, repo, m := zfsdirRepo(t)
	dir := t.TempDir()
	zfsdirTree(t, dir, "big.bin", "sub/big.bin", "sub/deeper/big.bin", "keep.txt")

	sum, err := r.BackupDir(context.Background(), repo, dir, []string{"zfs:tank/data"}, m, "big.bin")
	if err != nil {
		t.Fatalf("BackupDir: %v", err)
	}
	got := zfsdirEntries(t, r, repo, sum.SnapshotID, m)
	want := []string{"/keep.txt"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("snapshot holds %v, want %v", got, want)
	}
}

// TestBackupDirMiddleSlashExcludeMatchesAtAnyDepth checks that a pattern with a
// slash inside but none in front matches that path sequence at every depth, so
// "Library/Cache" reaches an app's cache wherever it sits.
func TestBackupDirMiddleSlashExcludeMatchesAtAnyDepth(t *testing.T) {
	r, repo, m := zfsdirRepo(t)
	dir := t.TempDir()
	zfsdirTree(t, dir, "Library/Cache/c.bin", "a/Library/Cache/c.bin", "Library/Preferences/p.xml")

	sum, err := r.BackupDir(context.Background(), repo, dir, []string{"zfs:tank/data"}, m, "Library/Cache")
	if err != nil {
		t.Fatalf("BackupDir: %v", err)
	}
	got := zfsdirEntries(t, r, repo, sum.SnapshotID, m)
	want := []string{"/Library/Preferences/p.xml"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("snapshot holds %v, want %v", got, want)
	}
}

// TestBackupDirTrailingSlashExclude checks that a trailing slash changes
// nothing, so the editor does not have to strip what a user types.
func TestBackupDirTrailingSlashExclude(t *testing.T) {
	r, repo, m := zfsdirRepo(t)
	dir := t.TempDir()
	zfsdirTree(t, dir, "cache/c.bin", "a/cache/c.bin", "keep.txt")

	sum, err := r.BackupDir(context.Background(), repo, dir, []string{"zfs:tank/data"}, m, "cache/")
	if err != nil {
		t.Fatalf("BackupDir: %v", err)
	}
	got := zfsdirEntries(t, r, repo, sum.SnapshotID, m)
	want := []string{"/keep.txt"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("snapshot holds %v, want %v", got, want)
	}
}

// TestBackupDirEmptyDirectoryFails records that restic refuses an empty
// directory. A dataset can be empty at the snapshot instant, so the caller has
// to look before it runs restic at all.
func TestBackupDirEmptyDirectoryFails(t *testing.T) {
	r, repo, m := zfsdirRepo(t)
	_, err := r.BackupDir(context.Background(), repo, t.TempDir(), []string{"zfs:tank/data"}, m)
	if err == nil {
		t.Fatal("restic accepted an empty directory")
	}
	if !strings.Contains(err.Error(), "snapshot is empty") {
		t.Fatalf("restic failed for another reason than an empty snapshot: %v", err)
	}
}

// TestRestoreAllLandsTreeRootInTarget checks that restoring a whole snapshot
// puts the dataset's own files directly into the target, with nothing of the
// snapshot directory's absolute path around them.
func TestRestoreAllLandsTreeRootInTarget(t *testing.T) {
	r, repo, m := zfsdirRepo(t)
	dir := t.TempDir()
	zfsdirTree(t, dir, "a.txt", "sub/big.bin")

	ctx := context.Background()
	sum, err := r.BackupDir(ctx, repo, dir, []string{"zfs:tank/data"}, m)
	if err != nil {
		t.Fatalf("BackupDir: %v", err)
	}
	target := filepath.Join(t.TempDir(), "restored")
	if err := r.RestoreAll(ctx, repo, sum.SnapshotID, target, m); err != nil {
		t.Fatalf("RestoreAll: %v", err)
	}
	for _, p := range []string{"a.txt", filepath.Join("sub", "big.bin")} {
		if _, err := os.Stat(filepath.Join(target, p)); err != nil {
			t.Fatalf("%s did not land at the root of the target: %v", p, err)
		}
	}
}

// TestRestoreIncludeSlashLandsTreeRootInTarget checks the same for the include
// form the drill uses to verify one member into a sandbox.
func TestRestoreIncludeSlashLandsTreeRootInTarget(t *testing.T) {
	r, repo, m := zfsdirRepo(t)
	dir := t.TempDir()
	zfsdirTree(t, dir, "a.txt", "sub/big.bin")

	ctx := context.Background()
	sum, err := r.BackupDir(ctx, repo, dir, []string{"zfs:tank/data"}, m)
	if err != nil {
		t.Fatalf("BackupDir: %v", err)
	}
	target := filepath.Join(t.TempDir(), "restored")
	if err := r.RestoreInclude(ctx, repo, sum.SnapshotID, "/", target, m); err != nil {
		t.Fatalf("RestoreInclude: %v", err)
	}
	if _, err := os.Stat(filepath.Join(target, "a.txt")); err != nil {
		t.Fatalf("a.txt did not land at the root of the target: %v", err)
	}
}
