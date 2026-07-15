package paths_test

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/paths"
)

// TestEnsureDirReadable pins Part 2: a restore TARGET on a user-visible share must
// be created 0o755 (readable by the operator's non-root SMB user), and an existing
// locked-down 0o700 target must be healed to 0o755 — mirroring how ensureDefsDir/
// makeRepoReadable relax perms on the backup share.
func TestEnsureDirReadable(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix permission bits are not modelled on windows")
	}
	root := t.TempDir()
	target := filepath.Join(root, "restore", "docs")

	if err := paths.EnsureDirReadable(target); err != nil {
		t.Fatalf("EnsureDirReadable (fresh): %v", err)
	}
	if perm := statPerm(t, target); perm != 0o755 {
		t.Fatalf("fresh restore target must be 0o755, got %o", perm)
	}

	// Heal an existing 0o700 target (an older version's mode) up to 0o755.
	if err := os.Chmod(target, 0o700); err != nil { //nolint:gosec // G302: deliberately simulating the old locked-down dir this fix heals
		t.Fatalf("chmod setup: %v", err)
	}
	if err := paths.EnsureDirReadable(target); err != nil {
		t.Fatalf("EnsureDirReadable (heal): %v", err)
	}
	if perm := statPerm(t, target); perm != 0o755 {
		t.Fatalf("EnsureDirReadable must heal 0o700 → 0o755, got %o", perm)
	}
}

func statPerm(t *testing.T, p string) os.FileMode {
	t.Helper()
	fi, err := os.Stat(p)
	if err != nil {
		t.Fatalf("stat %s: %v", p, err)
	}
	return fi.Mode().Perm()
}

func TestResolveHappyPath(t *testing.T) {
	got, err := paths.Resolve("/host/user", "backups/x")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "/host/user/backups/x" {
		t.Fatalf("expected /host/user/backups/x, got %s", got)
	}
}

func TestResolveRejectsTraversal(t *testing.T) {
	if _, err := paths.Resolve("/host/user", "../etc"); err == nil {
		t.Fatal("must reject .. traversal")
	}
	got, err := paths.Resolve("/host/user", "backups/x")
	if err != nil || got != "/host/user/backups/x" {
		t.Fatalf("expected /host/user/backups/x, got %s, err: %v", got, err)
	}
}

func TestResolveRejectsAbsoluteSub(t *testing.T) {
	if _, err := paths.Resolve("/host/user", "/etc/passwd"); err == nil {
		t.Fatal("must reject absolute sub path")
	}
}

func TestResolveRejectsHiddenTraversal(t *testing.T) {
	// sub that after cleaning resolves outside root
	if _, err := paths.Resolve("/host/user", "a/../../etc"); err == nil {
		t.Fatal("must reject traversal via a/../../etc")
	}
}

func TestResolveDeepPath(t *testing.T) {
	got, err := paths.Resolve("/host/user", "backups/bombvault/containers")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "/host/user/backups/bombvault/containers" {
		t.Fatalf("unexpected result: %s", got)
	}
}

func TestResolveRejectsEmptySub(t *testing.T) {
	// sub="" cleans to root itself — must be rejected (not a strict child).
	_, err := paths.Resolve("/host/user", "")
	if err == nil {
		t.Fatal("must reject empty sub (resolves to root, not a strict child)")
	}
}

func TestResolveRejectsDotSub(t *testing.T) {
	// sub="." cleans to root itself — must be rejected (not a strict child).
	_, err := paths.Resolve("/host/user", ".")
	if err == nil {
		t.Fatal("must reject sub='.' (resolves to root, not a strict child)")
	}
}
