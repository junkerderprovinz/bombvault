package restic

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"
)

// The two contract tests below pin the real-restic-0.17 behaviors the whole
// restore-selector design (RESTORE-01, 01-CONTEXT restore Q4, 01-RESEARCH R7)
// is built on:
//
//  1. --exclude patterns filter FILES but never drop a positional SOURCE dir:
//     the snapshot's Paths still record every positional verbatim, so the
//     stored selection's maximal roots stay valid `restore <id>:<path>`
//     selectors even when parts of the tree were excluded at backup time.
//  2. Snapshot Paths preserve ABSOLUTE paths exactly as given — no
//     relativization, no basename collapse — which is what makes
//     longest-prefix mapping against Paths meaningful and lets two positional
//     sources sharing a basename (…/a/end, …/b/end) stay distinct.
//
// They follow restic_roundtrip_test.go verbatim (package restic rather than
// restic_test, same as restic_args_test.go — the engine identifiers are in
// scope without an import): skipped when restic is not on PATH (the Windows
// dev box), proven on CI against the pinned restic 0.17.3. A unit test of
// BackupArgs can only prove argv SHAPE; these prove the ENGINE BEHAVIOR the
// shape was designed around.

// TestPositionalExcludesKeepSourceDir backs up a dir containing file.txt and
// ex.txt with exclude pattern "ex.txt", then asserts BOTH halves of the doc
// claim on the 0.17 floor: the snapshot's Paths still equal [srcDir] (the
// positional survived), and an ls of the snapshot proves file.txt is
// recoverable while ex.txt is genuinely filtered.
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

	// Half one: the excluded FILE must not drop the positional SOURCE — the
	// snapshot's Paths record srcDir exactly as passed, which is what keeps it
	// a valid restore selector after the selection was stored.
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

	// Half two: within that snapshot, file.txt is recoverable and ex.txt is
	// genuinely filtered — proving the exclude did its file-level job WITHOUT
	// its source-level side effect.
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

// TestPositionalAbsolutePathPreserved backs up a directory given as an
// absolute path and asserts the snapshot's Paths equal that absolute path
// VERBATIM — no relativization. The maximal-roots stored-selection form and
// the longest-prefix restore mapping both assume snapshot Paths are absolute;
// this pins that assumption on the 0.17 floor.
func TestPositionalAbsolutePathPreserved(t *testing.T) {
	if _, err := exec.LookPath("restic"); err != nil {
		t.Skip("no restic")
	}

	ctx := context.Background()
	dir := t.TempDir()
	repo := filepath.Join(dir, "repo")
	// t.TempDir() already yields an absolute path; the extra level mirrors a
	// real appdata subtree (never the repo or temp root itself).
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

// entryPaths flattens an Ls listing for failure messages.
func entryPaths(entries []FileEntry) []string {
	paths := make([]string, 0, len(entries))
	for _, e := range entries {
		paths = append(paths, e.Path)
	}
	return paths
}
