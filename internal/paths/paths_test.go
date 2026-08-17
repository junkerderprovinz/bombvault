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

// TestResolveAndWithinIdentityRoot proves Resolve/Within behave identically
// under a generic/TrueNAS "identity bind" config (HostSourceRoot ==
// HostMountRoot, e.g. both "/data" — no path translation) as they do under
// Unraid's split-root config (HostSourceRoot=/mnt, HostMountRoot=/host/user).
// Both configs ultimately feed the SAME single "root" value into
// Resolve/Within — neither function ever reads HostSourceRoot, only the
// caller's chosen root string — so an identity root is not a special case,
// just another root value. This test exists to prove that property rather
// than leave it implicit, per the design spec's platform-expansion audit.
func TestResolveAndWithinIdentityRoot(t *testing.T) {
	for _, tc := range []struct {
		name string
		root string
	}{
		{"split-root (Unraid default: HostSourceRoot=/mnt, HostMountRoot=/host/user)", "/host/user"},
		{"identity-root (generic/TrueNAS default: HostSourceRoot==HostMountRoot)", "/data"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := paths.Resolve(tc.root, "backups/bombvault/containers")
			if err != nil {
				t.Fatalf("Resolve: unexpected error: %v", err)
			}
			want := tc.root + "/backups/bombvault/containers"
			if got != want {
				t.Fatalf("Resolve: expected %s, got %s", want, got)
			}

			if _, err := paths.Resolve(tc.root, "../etc"); err == nil {
				t.Fatal("Resolve: must still reject traversal under this root")
			}
			if _, err := paths.Resolve(tc.root, "/etc/passwd"); err == nil {
				t.Fatal("Resolve: must still reject an absolute sub path under this root")
			}

			if !paths.Within(tc.root, want) {
				t.Fatalf("Within: expected %s to be contained within %s", want, tc.root)
			}
			if paths.Within(tc.root, tc.root) {
				t.Fatal("Within: root itself must not be considered a strict child of root")
			}
			if paths.Within(tc.root, tc.root+"2/other") {
				t.Fatal("Within: a sibling path sharing the root as a string prefix must not count as contained")
			}
		})
	}
}
