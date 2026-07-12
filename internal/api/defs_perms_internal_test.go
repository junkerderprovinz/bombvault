package api

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// The disaster-recovery defs directory lives on the operator's backup share, which
// is typically a network path the operator also copies off-box. An older BombVault
// created it at 0700 (root-only), which locked non-root SMB users out of the WHOLE
// backup folder and broke their second-copy sync. ensureDefsDir must create it
// world-traversable AND heal an existing 0700 directory to 0755.
func TestEnsureDefsDirHealsTo0755(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix permission bits are not modelled on windows")
	}
	dir := filepath.Join(t.TempDir(), "bombvault-defs")

	// A fresh create is world-traversable.
	if err := ensureDefsDir(dir); err != nil {
		t.Fatalf("ensureDefsDir (fresh): %v", err)
	}
	if perm := statPerm(t, dir); perm != 0o755 {
		t.Fatalf("fresh defs dir perm = %o, want 0755", perm)
	}

	// A directory an older version locked down to 0700 is healed to 0755.
	if err := os.Chmod(dir, 0o700); err != nil { //nolint:gosec // G302: deliberately simulating the old locked-down dir this fix heals
		t.Fatal(err)
	}
	if err := ensureDefsDir(dir); err != nil {
		t.Fatalf("ensureDefsDir (heal): %v", err)
	}
	if perm := statPerm(t, dir); perm != 0o755 {
		t.Fatalf("healed defs dir perm = %o, want 0755 (must relax a 0700 dir so the off-server sync tool can read it)", perm)
	}
}

// os.WriteFile keeps an existing file's mode, so a .def an older version wrote at
// 0600 would stay unreadable to the SMB sync user even after a fresh backup rewrote
// it. writeDef must heal it to 0644 (and still write the new encrypted bytes).
func TestWriteDefHeals0600FileTo0644(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix permission bits are not modelled on windows")
	}
	dir := t.TempDir()
	const fn = "plex.def"
	if err := os.WriteFile(filepath.Join(dir, fn), []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := writeDef(dir, fn, []byte("new-encrypted")); err != nil {
		t.Fatalf("writeDef: %v", err)
	}
	if perm := statPerm(t, filepath.Join(dir, fn)); perm != 0o644 {
		t.Fatalf("def file perm = %o, want 0644 (must heal a pre-existing 0600 file)", perm)
	}
	if b, err := os.ReadFile(filepath.Join(dir, fn)); err != nil || string(b) != "new-encrypted" { //nolint:gosec // G304: fn is a test constant under a t.TempDir()
		t.Fatalf("content = %q (err %v), want the new bytes", b, err)
	}
}

func statPerm(t *testing.T, path string) os.FileMode {
	t.Helper()
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	return fi.Mode().Perm()
}
