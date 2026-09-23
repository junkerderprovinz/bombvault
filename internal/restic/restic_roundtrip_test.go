package restic_test

import (
	"archive/zip"
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/restic"
)

// resolved returns path as restic will record it: symlinks followed and, on
// Windows, 8.3 short names expanded.
func resolved(t *testing.T, path string) string {
	t.Helper()
	real, err := filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatalf("resolve %q: %v", path, err)
	}
	return real
}

// inSnapshot returns the spelling restic uses for path inside a snapshot, which
// is what the `<id>:<path>` selector of `restic dump` is matched against. On
// Linux that is the path itself. On Windows restic drops the colon and makes
// the drive letter the first component, so C:\Users\x\src is stored as
// /C/Users/x/src. DumpZip callers run in the Linux container and never need
// this.
func inSnapshot(path string) string {
	if runtime.GOOS != "windows" {
		return path
	}
	vol := filepath.VolumeName(path) // "C:"
	rest := filepath.ToSlash(strings.TrimPrefix(path, vol))
	return "/" + strings.TrimSuffix(vol, ":") + rest
}

// TestRoundtrip runs init, backup and restore against the real restic binary.
// It skips when restic is not on PATH.
func TestRoundtrip(t *testing.T) {
	if _, err := exec.LookPath("restic"); err != nil {
		t.Skip("no restic")
	}

	ctx := context.Background()
	// restic records the resolved path and matches every later command against
	// it. On Windows a profile name with spaces gives t.TempDir() an 8.3 short
	// form that restic expands, and on macOS /var is a symlink to /private/var.
	dir := resolved(t, t.TempDir())
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
	if err := r.DumpZip(ctx, repo, sum.SnapshotID, inSnapshot(src), &buf, m); err != nil {
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

	snaps, err := r.Snapshots(ctx, repo, m)
	if err != nil {
		t.Fatal("Snapshots:", err)
	}
	if len(snaps) != 1 {
		t.Fatalf("expected 1 snapshot, got %d", len(snaps))
	}
}

// TestBackupStdinRoundtrip pipes content through BackupStdin and back out of
// DumpRaw, as a zvol's `zfs send` stream is backed up, and expects the same
// bytes without a staging file in between. It covers restic only; a real zfs
// send over SSH from a TrueNAS host is untested (see the package doc in
// internal/virshcli/zvol.go).
func TestBackupStdinRoundtrip(t *testing.T) {
	if _, err := exec.LookPath("restic"); err != nil {
		t.Skip("no restic")
	}

	ctx := context.Background()
	dir := t.TempDir()
	repo := filepath.Join(dir, "repo")

	r := restic.Restic{Bin: "restic"}
	m := restic.Mode{Encrypted: false}

	if err := r.Init(ctx, repo, m); err != nil {
		t.Fatal("Init:", err)
	}

	// Large enough to take more than one read from the pipe.
	want := bytes.Repeat([]byte("zfs-send-stream-bytes-"), 4096)
	const stdinPath = "/vm-disks/tank/vm-disk1@bombvault-snap"

	sum, err := r.BackupStdin(ctx, repo, bytes.NewReader(want), stdinPath, []string{"vm:truenasvm"}, m)
	if err != nil {
		t.Fatal("BackupStdin:", err)
	}
	if sum.SnapshotID == "" {
		t.Fatal("expected non-empty snapshot ID")
	}

	var got bytes.Buffer
	if err := r.DumpRaw(ctx, repo, sum.SnapshotID, stdinPath, &got, m); err != nil {
		t.Fatal("DumpRaw:", err)
	}
	if !bytes.Equal(got.Bytes(), want) {
		t.Fatalf("DumpRaw returned %d bytes, want %d bytes identical to what BackupStdin was given", got.Len(), len(want))
	}

	// The snapshot path must be the stdin filename exactly, without a
	// "/stdin/" prefix, because the zvol restore hands it to DumpRaw. Checked
	// on Linux only: on Windows restic resolves the absolute --stdin-filename
	// against the current drive ("D:\vm-disks\..."), and BombVault never runs
	// there.
	snaps, err := r.Snapshots(ctx, repo, m)
	if err != nil {
		t.Fatal("Snapshots:", err)
	}
	found := false
	for _, s := range snaps {
		if s.ID == sum.SnapshotID {
			found = true
			if runtime.GOOS != "windows" {
				if len(s.Paths) != 1 || s.Paths[0] != stdinPath {
					t.Fatalf("snapshot Paths = %v, want exactly [%q]", s.Paths, stdinPath)
				}
			}
		}
	}
	if !found {
		t.Fatal("backed-up snapshot not found in listing")
	}
}
