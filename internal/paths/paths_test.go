package paths_test

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/paths"
)

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

	// An existing 0o700 target is opened up to 0o755.
	if err := os.Chmod(target, 0o700); err != nil { //nolint:gosec // G302: sets up a locked-down target
		t.Fatalf("chmod setup: %v", err)
	}
	if err := paths.EnsureDirReadable(target); err != nil {
		t.Fatalf("EnsureDirReadable (heal): %v", err)
	}
	if perm := statPerm(t, target); perm != 0o755 {
		t.Fatalf("EnsureDirReadable must heal 0o700 → 0o755, got %o", perm)
	}
}

func TestEnsureEmptyDirTakesOnlyAFolderThatHoldsNothing(t *testing.T) {
	root := t.TempDir()
	if err := paths.EnsureEmptyDir(filepath.Join(root, "new", "folder")); err != nil {
		t.Fatalf("a new folder: %v", err)
	}
	empty := filepath.Join(root, "empty")
	if err := os.Mkdir(empty, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := paths.EnsureEmptyDir(empty); err != nil {
		t.Fatalf("an empty folder: %v", err)
	}
	if runtime.GOOS != "windows" && statPerm(t, empty) != 0o700 {
		t.Fatalf("the empty folder's mode changed to %o", statPerm(t, empty))
	}
	full := filepath.Join(root, "full")
	if err := os.Mkdir(full, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(full, "a"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(root, "file")
	if err := os.WriteFile(file, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{full, file} {
		if err := paths.EnsureEmptyDir(p); !errors.Is(err, paths.ErrNotEmpty) {
			t.Errorf("%s: %v, want ErrNotEmpty", p, err)
		}
	}
	if runtime.GOOS != "windows" {
		link := filepath.Join(root, "link")
		if err := os.Symlink(empty, link); err != nil {
			t.Fatal(err)
		}
		if err := paths.EnsureEmptyDir(link); !errors.Is(err, paths.ErrNotEmpty) {
			t.Errorf("a link to an empty folder: %v, want ErrNotEmpty", err)
		}
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
	_, err := paths.Resolve("/host/user", "")
	if err == nil {
		t.Fatal("must reject empty sub (resolves to root, not a strict child)")
	}
}

func TestResolveRejectsDotSub(t *testing.T) {
	_, err := paths.Resolve("/host/user", ".")
	if err == nil {
		t.Fatal("must reject sub='.' (resolves to root, not a strict child)")
	}
}

// TestResolveAndWithinIdentityRoot covers generic and TrueNAS hosts, which
// mount /data at /data instead of translating a path the way Unraid does; for
// Resolve and Within that is only another root.
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
