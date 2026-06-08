package restic_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/restic"
)

// TestRoundtrip exercises a full init → backup → restore cycle using the real
// restic binary.  It is skipped when restic is not on PATH (local dev) and
// runs in CI where restic is installed by the workflow.
func TestRoundtrip(t *testing.T) {
	if _, err := exec.LookPath("restic"); err != nil {
		t.Skip("no restic")
	}

	ctx := context.Background()
	dir := t.TempDir()
	repo := filepath.Join(dir, "repo")
	src := filepath.Join(dir, "src")
	if err := os.MkdirAll(src, 0o755); err != nil { //nolint:gosec // G301: test temp dir, relaxed permissions intentional
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "f.txt"), []byte("hi"), 0o644); err != nil { //nolint:gosec // G306: test file, relaxed permissions intentional
		t.Fatal(err)
	}

	r := restic.Restic{Bin: "restic"}
	m := restic.Mode{Encrypted: false}

	if err := r.Init(ctx, repo, m); err != nil {
		t.Fatal("Init:", err)
	}

	sum, err := r.Backup(ctx, repo, []string{src}, []string{"t"}, m)
	if err != nil {
		t.Fatal("Backup:", err)
	}
	if sum.SnapshotID == "" {
		t.Fatal("expected non-empty snapshot ID")
	}

	out := filepath.Join(dir, "out")
	if err := r.Restore(ctx, repo, sum.SnapshotID, out, m); err != nil {
		t.Fatal("Restore:", err)
	}

	// The restored tree is placed under out/ with the full path of src inside.
	// Walk for f.txt anywhere under out to verify restoration succeeded.
	found := false
	err = filepath.Walk(out, func(path string, _ os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if filepath.Base(path) == "f.txt" {
			found = true
		}
		return nil
	})
	if err != nil {
		t.Fatal("Walk:", err)
	}
	if !found {
		t.Fatal("restored f.txt not found under", out)
	}

	// Also verify snapshots listing works.
	snaps, err := r.Snapshots(ctx, repo, m)
	if err != nil {
		t.Fatal("Snapshots:", err)
	}
	if len(snaps) != 1 {
		t.Fatalf("expected 1 snapshot, got %d", len(snaps))
	}
}
