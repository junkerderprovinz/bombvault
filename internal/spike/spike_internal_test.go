package spike

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// TestProbePathWritableDoesNotCreateDir checks that the probe tests an existing
// ancestor instead of creating the path, because on Unraid a new top-level
// directory under /mnt/user becomes a share.
func TestProbePathWritableDoesNotCreateDir(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "bombvault", "container")

	msg, err := probePathWritable(Deps{ContainerPath: target})
	if err != nil {
		t.Fatalf("probePathWritable: %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(root, "bombvault")); statErr == nil {
		t.Fatalf("probe must NOT create the backup dir, but %q exists", filepath.Join(root, "bombvault"))
	}
	if msg == "" {
		t.Fatal("expected a writability message")
	}
}

func TestProbePathWritableExistingPath(t *testing.T) {
	dir := t.TempDir()
	msg, err := probePathWritable(Deps{ContainerPath: dir})
	if err != nil {
		t.Fatalf("probePathWritable: %v", err)
	}
	if msg == "" {
		t.Fatal("expected a writability message for an existing dir")
	}
}

func TestProbePathWritableEmpty(t *testing.T) {
	if _, err := probePathWritable(Deps{ContainerPath: ""}); err != nil {
		t.Fatalf("empty path should be a clean skip, got %v", err)
	}
}

func TestProbeZFSWithoutDepsSaysNotConfigured(t *testing.T) {
	detail, err := probeZFS(Deps{})
	if err != nil {
		t.Fatalf("a nil ZFSTest is not a failure, got %v", err)
	}
	if detail == "" {
		t.Fatal("the probe must say why it did not run")
	}
}

func TestProbeZFSReportsWhatTheHostAnswered(t *testing.T) {
	detail, err := probeZFS(Deps{ZFSTest: func() (string, error) { return "zfs-2.3.4 on root@tower:22", nil }})
	if err != nil {
		t.Fatalf("probeZFS: %v", err)
	}
	if detail != "zfs-2.3.4 on root@tower:22" {
		t.Fatalf("detail = %q, want the host's own answer", detail)
	}
	if _, err := probeZFS(Deps{ZFSTest: func() (string, error) { return "", errors.New("ssh-auth") }}); err == nil {
		t.Fatal("a failing connection test must fail the probe")
	}
}
