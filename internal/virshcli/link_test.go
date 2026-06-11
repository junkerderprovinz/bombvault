package virshcli

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// symlinkOrSkip skips a subtest on Windows, where creating a symlink needs an
// elevated privilege the test runner usually lacks. The behavior is exercised on
// Linux CI (and the production target is a Linux container).
func symlinkOrSkip(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation requires elevated privilege on Windows; verified on Linux CI")
	}
}

func TestLinkSocket(t *testing.T) {
	t.Run("creates symlink to runRoot/libvirt", func(t *testing.T) {
		symlinkOrSkip(t)
		dir := t.TempDir()
		runRoot := filepath.Join(dir, "host", "run")
		if err := os.MkdirAll(filepath.Join(runRoot, "libvirt"), 0o750); err != nil {
			t.Fatal(err)
		}
		link := filepath.Join(dir, "var", "run", "libvirt")

		if err := LinkSocket(runRoot, link); err != nil {
			t.Fatalf("LinkSocket: %v", err)
		}
		dst, err := os.Readlink(link)
		if err != nil {
			t.Fatalf("expected a symlink at %s: %v", link, err)
		}
		if want := filepath.Join(runRoot, "libvirt"); dst != want {
			t.Fatalf("symlink -> %q, want %q", dst, want)
		}
	})

	t.Run("no-op when runRoot is empty", func(t *testing.T) {
		if err := LinkSocket("", filepath.Join(t.TempDir(), "link")); err != nil {
			t.Fatalf("expected nil, got %v", err)
		}
	})

	t.Run("no-op when runRoot/libvirt is absent", func(t *testing.T) {
		dir := t.TempDir()
		link := filepath.Join(dir, "link")
		if err := LinkSocket(filepath.Join(dir, "empty-run"), link); err != nil {
			t.Fatalf("expected nil, got %v", err)
		}
		if _, err := os.Lstat(link); !os.IsNotExist(err) {
			t.Fatalf("expected no link created, got err=%v", err)
		}
	})

	t.Run("replaces a stale entry at linkPath", func(t *testing.T) {
		symlinkOrSkip(t)
		dir := t.TempDir()
		runRoot := filepath.Join(dir, "run")
		if err := os.MkdirAll(filepath.Join(runRoot, "libvirt"), 0o750); err != nil {
			t.Fatal(err)
		}
		link := filepath.Join(dir, "var", "run", "libvirt")
		// Pre-create a stale DIRECTORY at linkPath (the phantom Docker would leave).
		if err := os.MkdirAll(link, 0o750); err != nil {
			t.Fatal(err)
		}
		if err := LinkSocket(runRoot, link); err != nil {
			t.Fatalf("LinkSocket: %v", err)
		}
		dst, err := os.Readlink(link)
		if err != nil {
			t.Fatalf("expected stale dir replaced by symlink: %v", err)
		}
		if want := filepath.Join(runRoot, "libvirt"); dst != want {
			t.Fatalf("symlink -> %q, want %q", dst, want)
		}
	})

	t.Run("idempotent on an already-correct symlink", func(t *testing.T) {
		symlinkOrSkip(t)
		dir := t.TempDir()
		runRoot := filepath.Join(dir, "run")
		if err := os.MkdirAll(filepath.Join(runRoot, "libvirt"), 0o750); err != nil {
			t.Fatal(err)
		}
		link := filepath.Join(dir, "link")
		if err := LinkSocket(runRoot, link); err != nil {
			t.Fatal(err)
		}
		if err := LinkSocket(runRoot, link); err != nil {
			t.Fatalf("second call should be a no-op, got %v", err)
		}
		dst, _ := os.Readlink(link)
		if want := filepath.Join(runRoot, "libvirt"); dst != want {
			t.Fatalf("symlink -> %q, want %q", dst, want)
		}
	})
}
