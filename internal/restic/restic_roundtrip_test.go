package restic_test

import (
	"archive/zip"
	"bytes"
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

	// Flash-style restore: stream the snapshot subtree as a zip. Rooting at src
	// puts its contents (f.txt) at the archive root.
	var buf bytes.Buffer
	if err := r.DumpZip(ctx, repo, sum.SnapshotID, src, &buf, m); err != nil {
		t.Fatal("DumpZip:", err)
	}
	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatal("zip open:", err)
	}
	found := false
	for _, f := range zr.File {
		if filepath.Base(f.Name) == "f.txt" {
			found = true
		}
	}
	if !found {
		t.Fatal("f.txt not found in dumped zip")
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
