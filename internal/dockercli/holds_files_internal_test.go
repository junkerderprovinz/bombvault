package dockercli

import (
	"archive/tar"
	"bytes"
	"testing"
)

func tarOf(t *testing.T, entries ...*tar.Header) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	w := tar.NewWriter(&buf)
	for _, h := range entries {
		if err := w.WriteHeader(h); err != nil {
			t.Fatal(err)
		}
		if h.Size > 0 {
			if _, err := w.Write(bytes.Repeat([]byte("x"), int(h.Size))); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return &buf
}

func TestAnArchiveOfFoldersOnlyHoldsNoFiles(t *testing.T) {
	buf := tarOf(t,
		&tar.Header{Name: "data/", Typeflag: tar.TypeDir, Mode: 0o755},
		&tar.Header{Name: "data/sub/", Typeflag: tar.TypeDir, Mode: 0o755},
	)
	got, err := tarHoldsFiles(buf)
	if err != nil || got {
		t.Fatalf("tarHoldsFiles = %v, %v, want false", got, err)
	}
}

func TestAnArchiveWithAFileBelowAFolderHoldsFiles(t *testing.T) {
	buf := tarOf(t,
		&tar.Header{Name: "data/", Typeflag: tar.TypeDir, Mode: 0o755},
		&tar.Header{Name: "data/sub/", Typeflag: tar.TypeDir, Mode: 0o755},
		&tar.Header{Name: "data/sub/c.db", Typeflag: tar.TypeReg, Mode: 0o644, Size: 3},
	)
	got, err := tarHoldsFiles(buf)
	if err != nil || !got {
		t.Fatalf("tarHoldsFiles = %v, %v, want true", got, err)
	}
}

func TestAnEmptyArchiveHoldsNoFiles(t *testing.T) {
	got, err := tarHoldsFiles(tarOf(t))
	if err != nil || got {
		t.Fatalf("tarHoldsFiles = %v, %v, want false", got, err)
	}
}
