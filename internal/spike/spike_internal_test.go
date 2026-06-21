package spike

import (
	"os"
	"path/filepath"
	"testing"
)

// TestProbePathWritableDoesNotCreateDir pins the Unraid-share fix: probing a
// not-yet-existing backup path must report writability via an existing ancestor
// WITHOUT creating the path (a new top-level dir under /mnt/user would become a
// share the user can't easily delete).
func TestProbePathWritableDoesNotCreateDir(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "bombvault", "container") // does not exist yet

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

// TestProbePathWritableExistingPath: an already-existing path probes in place.
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

// TestProbePathWritableEmpty: no path configured is a clean skip.
func TestProbePathWritableEmpty(t *testing.T) {
	if _, err := probePathWritable(Deps{ContainerPath: ""}); err != nil {
		t.Fatalf("empty path should be a clean skip, got %v", err)
	}
}
