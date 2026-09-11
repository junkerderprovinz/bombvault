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
//  3. An ABSOLUTE SUBDIR pattern filters that subtree WITHIN a positional
//     source without dropping the positional — the exact pattern form the
//     stored-selection exclusion derivation emits on the backup argv (plan
//     01-05), and what makes WR-01's exclude-based enforcement of mixed
//     selections work on the engine floor.
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

// TestPositionalExcludeAbsoluteSubdirPattern pins claim 3: backing up srcDir
// (holding file.txt and sub/inner.txt) with the absolute pattern srcDir+"/sub"
// keeps srcDir verbatim in snapshot Paths AND proves via ls that file.txt is
// recoverable while inner.txt is filtered — the EXACT absolute-subdir pattern
// form the production derivation emits (stored selection entries are
// container-form POSIX paths). The pattern is built by POSIX concatenation to
// mirror that namespace; on the Windows dev box the LookPath skip fires before
// path form could matter, and CI (restic 0.17.3 on Linux) is the arbiter.
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

	// POSIX concatenation — mirrors the production derivation's namespace
	// (stored container-form POSIX paths), never filepath.Join, so the pattern
	// form proven here is byte-identical to what a mixed selection emits.
	excludePattern := srcDir + "/sub"

	sum, err := r.Backup(ctx, repo, []string{srcDir}, []string{"t"}, m, excludePattern)
	if err != nil {
		t.Fatal("Backup:", err)
	}
	if sum.SnapshotID == "" {
		t.Fatal("expected non-empty snapshot ID")
	}

	// Half one: the excluded SUBTREE must not drop the positional SOURCE —
	// snapshot Paths keep srcDir verbatim, which is what keeps the
	// restore-selector contract intact while the subtree is filtered.
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

	// Half two: within that snapshot, file.txt is recoverable and inner.txt is
	// genuinely filtered by the absolute subdir pattern.
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

// TestMultiPositionalPathsMirrorSelection pins the file-sets-parity engine
// floor (Phase 4, INTEG-02 success criterion 2): a backup invoked with TWO
// positional sources — a shallower root plus a disjoint deeper root in another
// branch, exactly the shape fileSetPositionals emits for a narrowed set — and
// one derived absolute-subdir --exclude records snapshot Paths EXACTLY equal
// to the positional list (verbatim, absolute, input order preserved), with the
// excluded branch's file filtered while both positionals survive as restore
// selectors. The argv-shape pins above prove shape; this proves the ENGINE
// behavior success criterion 2 is built on: snapshot Paths == the ticked
// roots, so the stored compiled list stays a valid mapRestorePaths input.
func TestMultiPositionalPathsMirrorSelection(t *testing.T) {
	if _, err := exec.LookPath("restic"); err != nil {
		t.Skip("no restic")
	}

	ctx := context.Background()
	dir := t.TempDir()
	repo := filepath.Join(dir, "repo")
	// The set root itself is NOT a positional; the two positionals are two
	// disjoint branches under it, one shallower and one deeper — the narrowed
	// multi-root compile output (fileSetPositionals never emits a positional
	// nested inside another one, so disjoint branches are the maximal shape).
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

	// The derived exclude uses POSIX concatenation (the production
	// derivation's namespace — stored entries are POSIX paths), never
	// filepath.Join, mirroring TestPositionalExcludeAbsoluteSubdirPattern.
	// Input order [docs, photos] is asserted below as-is: Paths must preserve
	// the positional order, not sort or dedupe it.
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

	// Both positionals remain live selectors — each has a recoverable file —
	// and the excluded branch's file is genuinely filtered out of the
	// snapshot content.
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
