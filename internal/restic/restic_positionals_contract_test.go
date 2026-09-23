package restic

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"
)

// These tests run the real restic (pinned to 0.17.3 on CI) and skip where it
// is not on PATH. The BackupArgs tests only show the argv; these show what
// restic does with it. Restore selection relies on two things checked here: an
// --exclude filters files inside a positional source without dropping the
// source from the snapshot's Paths, and Paths keep every positional as the
// absolute path it was given, so two sources sharing a basename stay distinct.

// TestPositionalExcludesKeepSourceDir backs up a directory with the exclude
// "ex.txt" and checks that the snapshot's Paths are still [srcDir] and that
// file.txt is in the snapshot while ex.txt is not.
func TestPositionalExcludesKeepSourceDir(t *testing.T) {
	if _, err := exec.LookPath("restic"); err != nil {
		t.Skip("no restic")
	}

	ctx := context.Background()
	dir := t.TempDir()
	repo := filepath.Join(dir, "repo")
	srcDir := filepath.Join(dir, "src")
	if err := os.MkdirAll(srcDir, 0o755); err != nil { //nolint:gosec // G301: test temp dir, relaxed permissions intentional
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "file.txt"), []byte("kept"), 0o644); err != nil { //nolint:gosec // G306: test file, relaxed permissions intentional
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "ex.txt"), []byte("excluded"), 0o644); err != nil { //nolint:gosec // G306: test file, relaxed permissions intentional
		t.Fatal(err)
	}

	r := Restic{Bin: "restic"}
	m := Mode{Encrypted: false}

	if err := r.Init(ctx, repo, m); err != nil {
		t.Fatal("Init:", err)
	}

	sum, err := r.Backup(ctx, repo, []string{srcDir}, []string{"t"}, m, "ex.txt")
	if err != nil {
		t.Fatal("Backup:", err)
	}
	if sum.SnapshotID == "" {
		t.Fatal("expected non-empty snapshot ID")
	}

	// srcDir has to stay in Paths as passed, or it stops being a valid restore
	// selector for the stored selection.
	snaps, err := r.Snapshots(ctx, repo, m)
	if err != nil {
		t.Fatal("Snapshots:", err)
	}
	if len(snaps) != 1 {
		t.Fatalf("expected 1 snapshot, got %d", len(snaps))
	}
	wantPaths := []string{srcDir}
	if !reflect.DeepEqual(snaps[0].Paths, wantPaths) {
		t.Fatalf("snapshot Paths = %v, want exactly %v (excludes must not drop the positional source)", snaps[0].Paths, wantPaths)
	}

	entries, err := r.Ls(ctx, repo, snaps[0].ID, m)
	if err != nil {
		t.Fatal("Ls:", err)
	}
	var kept, excluded bool
	for _, e := range entries {
		switch filepath.Base(e.Path) {
		case "file.txt":
			kept = true
		case "ex.txt":
			excluded = true
		}
	}
	if !kept {
		t.Fatalf("file.txt not recoverable from the snapshot; entries = %v", entryPaths(entries))
	}
	if excluded {
		t.Fatalf("ex.txt must be filtered by the exclude pattern; entries = %v", entryPaths(entries))
	}
}

// TestPositionalAbsolutePathPreserved checks that an absolute source appears
// unchanged in the snapshot's Paths, which the stored selection and the
// longest-prefix restore mapping both assume.
func TestPositionalAbsolutePathPreserved(t *testing.T) {
	if _, err := exec.LookPath("restic"); err != nil {
		t.Skip("no restic")
	}

	ctx := context.Background()
	dir := t.TempDir()
	repo := filepath.Join(dir, "repo")
	// A subtree like appdata rather than the temp root itself.
	absDir := filepath.Join(dir, "abs", "appdata")
	if err := os.MkdirAll(absDir, 0o755); err != nil { //nolint:gosec // G301: test temp dir, relaxed permissions intentional
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(absDir, "f.txt"), []byte("hi"), 0o644); err != nil { //nolint:gosec // G306: test file, relaxed permissions intentional
		t.Fatal(err)
	}
	if !filepath.IsAbs(absDir) {
		t.Fatalf("precondition: %q must be an absolute path", absDir)
	}

	r := Restic{Bin: "restic"}
	m := Mode{Encrypted: false}

	if err := r.Init(ctx, repo, m); err != nil {
		t.Fatal("Init:", err)
	}

	if _, err := r.Backup(ctx, repo, []string{absDir}, []string{"t"}, m); err != nil {
		t.Fatal("Backup:", err)
	}

	snaps, err := r.Snapshots(ctx, repo, m)
	if err != nil {
		t.Fatal("Snapshots:", err)
	}
	if len(snaps) != 1 {
		t.Fatalf("expected 1 snapshot, got %d", len(snaps))
	}
	wantPaths := []string{absDir}
	if !reflect.DeepEqual(snaps[0].Paths, wantPaths) {
		t.Fatalf("snapshot Paths = %v, want the absolute path verbatim %v (no relativization)", snaps[0].Paths, wantPaths)
	}
}

// TestPositionalExcludeAbsoluteSubdirPattern backs up srcDir with the absolute
// pattern srcDir+"/sub", the form derived from a stored selection, and checks
// that srcDir stays in the snapshot's Paths while sub/inner.txt is filtered.
func TestPositionalExcludeAbsoluteSubdirPattern(t *testing.T) {
	if _, err := exec.LookPath("restic"); err != nil {
		t.Skip("no restic")
	}

	ctx := context.Background()
	dir := t.TempDir()
	repo := filepath.Join(dir, "repo")
	srcDir := filepath.Join(dir, "src")
	if err := os.MkdirAll(filepath.Join(srcDir, "sub"), 0o755); err != nil { //nolint:gosec // G301: test temp dir, relaxed permissions intentional
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "file.txt"), []byte("kept"), 0o644); err != nil { //nolint:gosec // G306: test file, relaxed permissions intentional
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "sub", "inner.txt"), []byte("filtered"), 0o644); err != nil { //nolint:gosec // G306: test file, relaxed permissions intentional
		t.Fatal(err)
	}

	r := Restic{Bin: "restic"}
	m := Mode{Encrypted: false}

	if err := r.Init(ctx, repo, m); err != nil {
		t.Fatal("Init:", err)
	}

	// Stored selections hold POSIX paths, so the pattern is concatenated the
	// same way instead of using filepath.Join.
	excludePattern := srcDir + "/sub"

	sum, err := r.Backup(ctx, repo, []string{srcDir}, []string{"t"}, m, excludePattern)
	if err != nil {
		t.Fatal("Backup:", err)
	}
	if sum.SnapshotID == "" {
		t.Fatal("expected non-empty snapshot ID")
	}

	snaps, err := r.Snapshots(ctx, repo, m)
	if err != nil {
		t.Fatal("Snapshots:", err)
	}
	if len(snaps) != 1 {
		t.Fatalf("expected 1 snapshot, got %d", len(snaps))
	}
	wantPaths := []string{srcDir}
	if !reflect.DeepEqual(snaps[0].Paths, wantPaths) {
		t.Fatalf("snapshot Paths = %v, want exactly %v (an absolute subdir exclude must not drop the positional)", snaps[0].Paths, wantPaths)
	}

	entries, err := r.Ls(ctx, repo, snaps[0].ID, m)
	if err != nil {
		t.Fatal("Ls:", err)
	}
	var kept, filtered bool
	for _, e := range entries {
		switch filepath.Base(e.Path) {
		case "file.txt":
			kept = true
		case "inner.txt":
			filtered = true
		}
	}
	if !kept {
		t.Fatalf("file.txt not recoverable from the snapshot; entries = %v", entryPaths(entries))
	}
	if filtered {
		t.Fatalf("inner.txt must be filtered by the absolute subdir pattern; entries = %v", entryPaths(entries))
	}
}

// TestMultiPositionalPathsMirrorSelection backs up two disjoint roots, the
// shape fileSetPositionals emits for a narrowed file set, with one absolute
// subdir exclude. The snapshot's Paths must equal the positional list as given
// (absolute, same order) so the stored list stays valid input for
// mapRestorePaths, and the excluded branch must be missing from the content.
func TestMultiPositionalPathsMirrorSelection(t *testing.T) {
	if _, err := exec.LookPath("restic"); err != nil {
		t.Skip("no restic")
	}

	ctx := context.Background()
	dir := t.TempDir()
	repo := filepath.Join(dir, "repo")
	// The set root is not a positional itself. fileSetPositionals never nests
	// one positional inside another, so the two are disjoint branches, one
	// deeper than the other.
	setRoot := filepath.Join(dir, "setroot")
	docs := filepath.Join(setRoot, "docs")
	photos := filepath.Join(setRoot, "photos", "2024")
	private := filepath.Join(photos, "private")
	for _, d := range []string{docs, private} {
		if err := os.MkdirAll(d, 0o755); err != nil { //nolint:gosec // G301: test temp dir, relaxed permissions intentional
			t.Fatal(err)
		}
	}
	for path, content := range map[string]string{
		filepath.Join(docs, "keep-doc.txt"):     "doc",
		filepath.Join(photos, "keep-photo.txt"): "photo",
		filepath.Join(private, "secret.txt"):    "secret",
	} {
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil { //nolint:gosec // G306: test file, relaxed permissions intentional
			t.Fatal(err)
		}
	}

	r := Restic{Bin: "restic"}
	m := Mode{Encrypted: false}

	if err := r.Init(ctx, repo, m); err != nil {
		t.Fatal("Init:", err)
	}

	// The exclude is concatenated like a derived one. Paths must keep the
	// positional order as given.
	positionals := []string{docs, photos}
	if _, err := r.Backup(ctx, repo, positionals, []string{"t"}, m, photos+"/private"); err != nil {
		t.Fatal("Backup:", err)
	}

	snaps, err := r.Snapshots(ctx, repo, m)
	if err != nil {
		t.Fatal("Snapshots:", err)
	}
	if len(snaps) != 1 {
		t.Fatalf("expected 1 snapshot, got %d", len(snaps))
	}
	if !reflect.DeepEqual(snaps[0].Paths, positionals) {
		t.Fatalf("snapshot Paths = %v, want exactly the positional list %v (verbatim, absolute, order preserved)", snaps[0].Paths, positionals)
	}

	entries, err := r.Ls(ctx, repo, snaps[0].ID, m)
	if err != nil {
		t.Fatal("Ls:", err)
	}
	var keptDoc, keptPhoto, secret bool
	for _, e := range entries {
		switch filepath.Base(e.Path) {
		case "keep-doc.txt":
			keptDoc = true
		case "keep-photo.txt":
			keptPhoto = true
		case "secret.txt":
			secret = true
		}
	}
	if !keptDoc || !keptPhoto {
		t.Fatalf("both positionals must remain restorable selectors; entries = %v", entryPaths(entries))
	}
	if secret {
		t.Fatalf("the excluded branch's file must be filtered from the snapshot; entries = %v", entryPaths(entries))
	}
}

// entryPaths flattens an Ls listing for failure messages.
func entryPaths(entries []FileEntry) []string {
	paths := make([]string, 0, len(entries))
	for _, e := range entries {
		paths = append(paths, e.Path)
	}
	return paths
}
