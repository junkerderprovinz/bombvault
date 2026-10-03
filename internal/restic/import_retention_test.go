package restic_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/restic"
)

// An imported restore point is older than the item's own backups, so a keep
// rule would forget it on the next pass, and nothing would bring it back.
func TestRetentionKeepsImportedSnapshots(t *testing.T) {
	if _, err := exec.LookPath("restic"); err != nil {
		t.Skip("no restic")
	}
	ctx := context.Background()
	dir := resolved(t, t.TempDir())
	repo := filepath.Join(dir, "repo")
	src := filepath.Join(dir, "src")
	if err := os.MkdirAll(src, 0o755); err != nil { //nolint:gosec // G301: test temp dir
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "f"), []byte("x"), 0o644); err != nil { //nolint:gosec // G306: test file
		t.Fatal(err)
	}
	r := restic.Restic{Bin: "restic"}
	m := restic.Mode{}
	if err := r.Init(ctx, repo, m); err != nil {
		t.Fatal(err)
	}
	tags := []string{"container:plex"}
	at := time.Date(2024, 1, 1, 2, 0, 0, 0, time.Local)
	imported, err := r.ImportDir(ctx, repo, src, []string{"container:plex", "import:ab_20240101_020000"}, at, m)
	if err != nil {
		t.Fatal(err)
	}
	var newest restic.Summary
	for range 2 {
		if newest, err = r.Backup(ctx, repo, []string{src}, tags, m); err != nil {
			t.Fatal(err)
		}
	}
	if err := r.ForgetPolicy(ctx, repo, restic.RetentionPolicy{KeepLast: 1}, m, tags, false); err != nil {
		t.Fatal(err)
	}
	snaps, err := r.Snapshots(ctx, repo, m)
	if err != nil {
		t.Fatal(err)
	}
	var left []string
	for _, sn := range snaps {
		left = append(left, sn.ID)
	}
	if len(left) != 2 || !slices.Contains(left, imported.SnapshotID) || !slices.Contains(left, newest.SnapshotID) {
		t.Fatalf("left %v, want the newest backup %s and the imported %s", left, newest.SnapshotID, imported.SnapshotID)
	}
}
