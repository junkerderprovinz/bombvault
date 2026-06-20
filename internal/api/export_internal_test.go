package api

import (
	"archive/tar"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestDedupPaths verifies exact duplicates and paths nested under another are
// dropped, so overlapping selections never archive a file twice.
func TestDedupPaths(t *testing.T) {
	in := []string{"/a/b", "/a/b", "/a/b/c", "/a/d", "/a/b/c/d"}
	got := dedupPaths(in)
	want := []string{filepath.Clean("/a/b"), filepath.Clean("/a/d")}
	if len(got) != len(want) {
		t.Fatalf("dedupPaths(%v) = %v, want %v", in, got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("dedupPaths[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

// TestAddToTarNeverEscapes guards the archive-traversal hardening: a source path
// OUTSIDE the mount root must NOT produce a "../"-prefixed tar entry (which would
// write outside the target tree on extraction). Such a path is re-rooted at its
// own base name instead.
func TestAddToTarNeverEscapes(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, "mount") // need not exist
	outside := filepath.Join(dir, "elsewhere", "secret")
	if err := os.MkdirAll(outside, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(outside, "f.txt"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	if err := addToTar(tw, root, outside); err != nil {
		t.Fatalf("addToTar: %v", err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}

	tr := tar.NewReader(&buf)
	var sawFile bool
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		if strings.HasPrefix(hdr.Name, "..") || strings.Contains(hdr.Name, "/../") {
			t.Fatalf("tar entry escapes the tree: %q", hdr.Name)
		}
		if hdr.Name == "secret/f.txt" {
			sawFile = true
		}
	}
	if !sawFile {
		t.Fatal(`expected entry "secret/f.txt" (re-rooted at the source base)`)
	}
}
