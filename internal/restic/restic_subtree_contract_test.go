package restic

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

// TestRestoreSubtreeBelowRecordedPath checks that a `<id>:<path>` selector can
// name any directory inside the snapshot, not only a recorded path. That lets
// mapRestorePaths restore /a/b/c from a snapshot that records only /a, instead
// of restoring all of /a and overwriting siblings the user deselected and never
// backed up. Requiring subtreePath to be one of the snapshot's own paths is a
// caller-side safety rule (a recomputed path could miss after a HostMountRoot
// change), not a restic limit.
func TestRestoreSubtreeBelowRecordedPath(t *testing.T) {
	if _, err := exec.LookPath("restic"); err != nil {
		t.Skip("restic not on PATH")
	}
	// A drive letter puts a second colon into "<id>:<path>" and restic takes
	// "C" as the id.
	if runtime.GOOS == "windows" {
		t.Skip("the <id>:<path> selector cannot carry a Windows drive letter")
	}
	tmp := t.TempDir()
	repo := filepath.Join(tmp, "repo")
	src := filepath.Join(tmp, "src")
	// Only "keep" may come back; "other" stands for a deselected sibling.
	keep := filepath.Join(src, "keep")
	other := filepath.Join(src, "other")
	for _, d := range []string{keep, other} {
		if err := os.MkdirAll(d, 0o755); err != nil { //nolint:gosec // G301: test temp dir
			t.Fatalf("mkdir %s: %v", d, err)
		}
	}
	if err := os.WriteFile(filepath.Join(keep, "wanted.txt"), []byte("wanted"), 0o644); err != nil { //nolint:gosec // G306: test file
		t.Fatalf("write wanted: %v", err)
	}
	if err := os.WriteFile(filepath.Join(other, "unwanted.txt"), []byte("unwanted"), 0o644); err != nil { //nolint:gosec // G306: test file
		t.Fatalf("write unwanted: %v", err)
	}

	ctx := context.Background()
	env := append(os.Environ(), "RESTIC_PASSWORD=contract")
	run := func(args ...string) (string, error) {
		cmd := exec.CommandContext(ctx, "restic", args...) //nolint:gosec // fixed args, test-local paths
		cmd.Env = env
		out, err := cmd.CombinedOutput()
		return string(out), err
	}

	if out, err := run("-r", repo, "init"); err != nil {
		t.Fatalf("init: %v\n%s", err, out)
	}
	// The snapshot records only the root.
	if out, err := run("-r", repo, "backup", "--no-scan", src); err != nil {
		t.Fatalf("backup: %v\n%s", err, out)
	}

	target := filepath.Join(tmp, "restored")
	out, err := run("-r", repo, "restore", "latest:"+filepath.ToSlash(keep), "--target", target)
	if err != nil {
		t.Fatalf("restore of a subtree below the recorded path failed: %v\n%s", err, out)
	}

	if _, err := os.Stat(filepath.Join(target, "wanted.txt")); err != nil {
		t.Fatalf("wanted.txt not restored (so the subtree selector did not work): %v\n%s", err, out)
	}
	if _, err := os.Stat(filepath.Join(target, "unwanted.txt")); err == nil {
		t.Fatal("the deselected sibling was restored too: the selector widened to the recorded root")
	}
	if _, err := os.Stat(filepath.Join(target, "other")); err == nil {
		t.Fatal("the deselected sibling directory was restored too")
	}
}

// TestRestoreCommonAncestorOfRecordedRoots checks that a node above the recorded
// paths is addressable too and restores exactly the recorded roots. A file set
// with two selected sub-folders records two paths, and the to-folder restore
// brings both back through their deepest common ancestor. That is only safe
// because a snapshot tree holds nothing but what was backed up, so a
// deselected sibling cannot come back.
func TestRestoreCommonAncestorOfRecordedRoots(t *testing.T) {
	if _, err := exec.LookPath("restic"); err != nil {
		t.Skip("restic not on PATH")
	}
	if runtime.GOOS == "windows" {
		t.Skip("the <id>:<path> selector cannot carry a Windows drive letter")
	}
	tmp := t.TempDir()
	repo := filepath.Join(tmp, "repo")
	docs := filepath.Join(tmp, "src", "docs")
	// Only two of the three siblings are backed up. The third is a deselected
	// folder, present on disk and absent from the snapshot.
	for _, d := range []string{"keep-a", "keep-b", "never-backed-up"} {
		if err := os.MkdirAll(filepath.Join(docs, d), 0o755); err != nil { //nolint:gosec // G301: test temp dir
			t.Fatalf("mkdir %s: %v", d, err)
		}
		if err := os.WriteFile(filepath.Join(docs, d, "f.txt"), []byte(d), 0o644); err != nil { //nolint:gosec // G306: test file
			t.Fatalf("write %s: %v", d, err)
		}
	}

	env := append(os.Environ(), "RESTIC_PASSWORD=contract")
	run := func(args ...string) (string, error) {
		cmd := exec.CommandContext(context.Background(), "restic", args...) //nolint:gosec // fixed args, test-local paths
		cmd.Env = env
		out, err := cmd.CombinedOutput()
		return string(out), err
	}
	if out, err := run("-r", repo, "init"); err != nil {
		t.Fatalf("init: %v\n%s", err, out)
	}
	// Two recorded roots, as fileSetPositionals produces them.
	if out, err := run("-r", repo, "backup", "--no-scan",
		filepath.Join(docs, "keep-a"), filepath.Join(docs, "keep-b")); err != nil {
		t.Fatalf("backup: %v\n%s", err, out)
	}

	target := filepath.Join(tmp, "restored")
	// Their common ancestor is not a recorded path.
	out, err := run("-r", repo, "restore", "latest:"+filepath.ToSlash(docs), "--target", target)
	if err != nil {
		t.Fatalf("restore of the common ancestor failed: %v\n%s", err, out)
	}

	for _, d := range []string{"keep-a", "keep-b"} {
		if _, err := os.Stat(filepath.Join(target, d, "f.txt")); err != nil {
			t.Fatalf("%s/f.txt not restored, so one recorded root was dropped: %v\n%s", d, err, out)
		}
	}
	if _, err := os.Stat(filepath.Join(target, "never-backed-up")); err == nil {
		t.Fatal("a never-backed-up sibling came back: restoring the common ancestor widens, and the to-folder fix would be unsafe")
	}
}
