package restic_test

import (
	"archive/zip"
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
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

// TestBackupStdinRoundtrip exercises BackupStdin → DumpRaw against the real
// restic binary: content piped in via stdin (standing in for a zvol's
// `zfs send` stream, Task 10 of the v8.0.0 TrueNAS platform expansion) must
// come back byte-identical, with no local staging file at any point. This
// verifies the restic-level mechanism genuinely works (not just its argv
// shape) — it does NOT verify anything upstream of it: a real `zfs send`
// stream over a real SSH connection from a real TrueNAS Scale host has never
// been exercised (no test hardware available; see
// internal/virshcli/zvol.go's package doc comment for the full caveat).
// Skipped when restic is not on PATH, same as TestRoundtrip.
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

	// Content large enough to exercise more than a single read() from the
	// pipe, standing in for a zfs send stream's bytes.
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

	// The recorded snapshot path must be exactly the stdin-filename given — no
	// synthesized "/stdin/..." prefix — which is what BackupStdinArgs's doc
	// comment claims and what DumpRaw's caller (the zvol restore path) depends
	// on. Only asserted on the actual deployment target (Linux, where an
	// already-absolute path is stored verbatim); restic resolves a "/..."
	// --stdin-filename through the OS's own absolute-path rules, so on Windows
	// (this repo's dev sandbox only — BombVault never runs there) restic
	// rewrites it under the current drive (e.g. "D:\vm-disks\..."), which is a
	// platform quirk of local dev, not a behavior BombVault's container ever
	// exercises. The byte-identity round-trip above (the property that
	// actually matters) already passed on every OS.
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
