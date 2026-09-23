package api

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/restic"
)

// buildTestStoredZip builds a zip with every entry stored uncompressed, the
// way restic's `dump -a zip` writes it. A name ending in "/" becomes a
// directory entry and its content is ignored.
func buildTestStoredZip(t *testing.T, files map[string][]byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, content := range files {
		hdr := &zip.FileHeader{Name: name, Method: zip.Store}
		if strings.HasSuffix(name, "/") {
			hdr.SetMode(os.ModeDir | 0o700)
			content = nil
		} else {
			hdr.SetMode(0o600)
		}
		w, err := zw.CreateHeader(hdr)
		if err != nil {
			t.Fatalf("create header %s: %v", name, err)
		}
		if _, err := w.Write(content); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close zip writer: %v", err)
	}
	return buf.Bytes()
}

func TestFlashZipJunkEntry(t *testing.T) {
	cases := []struct {
		name string
		size uint64
		want bool
	}{
		{".git", 0, true},
		{".git/config", 475, true},
		{".git/objects/ab/cdef", 100, true},
		{".gitattributes", 180, true},
		{"System Volume Information", 0, true},
		{"System Volume Information/IndexerVolumeGuid", 76, true},
		{"FSCK0000.REC", 32768, true},
		{"FSCK123456.REC", 100, true},
		{"260325124039.BMP", 0, true},
		{"previous", 0, true},
		{"previous/bzimage", 9821184, true},
		{"previous/bzmodules", 771751936, true},
		{"config/plugins/gitkeep-note.txt", 12, false},
		{"notFSCK.REC", 10, false},
		{"syslinux/splash.bmp", 45678, false},
		{"bzfirmware", 323293184, false},
		{"EFI/boot/bootx64.efi", 199952, false},
		{"config/previous-attempt.log", 40, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := flashZipJunkEntry(tc.name, tc.size); got != tc.want {
				t.Errorf("flashZipJunkEntry(%q, %d) = %v, want %v", tc.name, tc.size, got, tc.want)
			}
		})
	}
}

func TestRecompressFlashZipConvertsStoreToDeflate(t *testing.T) {
	content := bytes.Repeat([]byte("compressible-content-"), 500)
	raw := buildTestStoredZip(t, map[string][]byte{"bzfirmware": content})

	zrIn, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		t.Fatal(err)
	}
	if zrIn.File[0].Method != zip.Store {
		t.Fatalf("fixture precondition failed: input method = %v, want Store", zrIn.File[0].Method)
	}

	var out bytes.Buffer
	if err := recompressFlashZip(bytes.NewReader(raw), int64(len(raw)), &out); err != nil {
		t.Fatalf("recompressFlashZip: %v", err)
	}

	zrOut, err := zip.NewReader(bytes.NewReader(out.Bytes()), int64(out.Len()))
	if err != nil {
		t.Fatalf("output is not a valid zip: %v", err)
	}
	if len(zrOut.File) != 1 {
		t.Fatalf("output entry count = %d, want 1", len(zrOut.File))
	}
	f := zrOut.File[0]
	if f.Method != zip.Deflate {
		t.Fatalf("output method = %v, want Deflate", f.Method)
	}
	rc, err := f.Open()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rc.Close() }()
	got, err := io.ReadAll(rc)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, content) {
		t.Fatal("recompressed content does not round-trip to the original bytes")
	}
}

func TestRecompressFlashZipDropsJunkEntries(t *testing.T) {
	raw := buildTestStoredZip(t, map[string][]byte{
		"bzfirmware":                 []byte("real boot content"),
		"EFI/":                       nil,
		"EFI/boot/bootx64.efi":       []byte("real efi content"),
		".git/":                      nil,
		".git/config":                []byte("[core]"),
		".gitattributes":             []byte("* -text"),
		"System Volume Information/": nil,
		"System Volume Information/IndexerVolumeGuid": []byte("guid"),
		"FSCK0000.REC":       bytes.Repeat([]byte{0}, 100),
		"260325124039.BMP":   nil,
		"previous/":          nil,
		"previous/bzimage":   []byte("stale prior-version kernel"),
		"previous/bzmodules": []byte("stale prior-version modules"),
	})

	var out bytes.Buffer
	if err := recompressFlashZip(bytes.NewReader(raw), int64(len(raw)), &out); err != nil {
		t.Fatalf("recompressFlashZip: %v", err)
	}

	zrOut, err := zip.NewReader(bytes.NewReader(out.Bytes()), int64(out.Len()))
	if err != nil {
		t.Fatal(err)
	}
	survivors := map[string]bool{}
	for _, f := range zrOut.File {
		survivors[f.Name] = true
	}
	want := map[string]bool{"bzfirmware": true, "EFI/": true, "EFI/boot/bootx64.efi": true}
	if len(survivors) != len(want) {
		t.Fatalf("survivors = %v, want exactly %v", survivors, want)
	}
	for name := range want {
		if !survivors[name] {
			t.Errorf("%s should have survived, survivors = %v", name, survivors)
		}
	}
}

func TestRecompressFlashZipPreservesNonEmptyBMP(t *testing.T) {
	raw := buildTestStoredZip(t, map[string][]byte{
		"syslinux/splash.bmp": bytes.Repeat([]byte{0x42, 0x4d}, 200),
	})

	var out bytes.Buffer
	if err := recompressFlashZip(bytes.NewReader(raw), int64(len(raw)), &out); err != nil {
		t.Fatalf("recompressFlashZip: %v", err)
	}
	zrOut, err := zip.NewReader(bytes.NewReader(out.Bytes()), int64(out.Len()))
	if err != nil {
		t.Fatal(err)
	}
	if len(zrOut.File) != 1 || zrOut.File[0].Name != "syslinux/splash.bmp" {
		t.Fatalf("a non-empty .bmp must survive recompression, got %v", zrOut.File)
	}
}

func TestRecompressFlashZipPreservesDirectoryEntries(t *testing.T) {
	raw := buildTestStoredZip(t, map[string][]byte{
		"EFI/":      nil,
		"EFI/boot/": nil,
	})

	var out bytes.Buffer
	if err := recompressFlashZip(bytes.NewReader(raw), int64(len(raw)), &out); err != nil {
		t.Fatalf("recompressFlashZip: %v", err)
	}
	zrOut, err := zip.NewReader(bytes.NewReader(out.Bytes()), int64(out.Len()))
	if err != nil {
		t.Fatal(err)
	}
	if len(zrOut.File) != 2 {
		t.Fatalf("output entry count = %d, want 2", len(zrOut.File))
	}
	for _, f := range zrOut.File {
		if !f.FileInfo().IsDir() {
			t.Errorf("%s should still be a directory entry after recompression", f.Name)
		}
	}
}

func TestRecompressFlashZipRejectsInvalidInput(t *testing.T) {
	garbage := []byte("not a zip file at all")
	var out bytes.Buffer
	if err := recompressFlashZip(bytes.NewReader(garbage), int64(len(garbage)), &out); err == nil {
		t.Fatal("expected an error for non-zip input")
	}
}

// compatFakeEngine implements only DumpZip, the one engine method
// dumpFlashZipCompat calls.
type compatFakeEngine struct {
	ResticEngine
	dumpBytes []byte
	dumpErr   error
}

func (f *compatFakeEngine) DumpZip(_ context.Context, _, _, _ string, w io.Writer, _ restic.Mode) error {
	if f.dumpErr != nil {
		return f.dumpErr
	}
	_, err := w.Write(f.dumpBytes)
	return err
}

func TestDumpFlashZipCompatRecompressesAndFilters(t *testing.T) {
	raw := buildTestStoredZip(t, map[string][]byte{
		"bzfirmware":     bytes.Repeat([]byte("firmware-bytes-"), 100),
		".gitattributes": []byte("* -text"),
		".git/config":    []byte("[core]"),
	})
	dataDir := t.TempDir()
	svc := &Service{
		cfg:    config.Config{DataDir: dataDir},
		engine: &compatFakeEngine{dumpBytes: raw},
	}

	var out bytes.Buffer
	if err := svc.dumpFlashZipCompat(context.Background(), "repo", "snap", "/host/boot", &out, restic.Mode{}); err != nil {
		t.Fatalf("dumpFlashZipCompat: %v", err)
	}

	zr, err := zip.NewReader(bytes.NewReader(out.Bytes()), int64(out.Len()))
	if err != nil {
		t.Fatalf("output is not a valid zip: %v", err)
	}
	if len(zr.File) != 1 || zr.File[0].Name != "bzfirmware" {
		t.Fatalf("output entries = %v, want only [bzfirmware]", zr.File)
	}
	if zr.File[0].Method != zip.Deflate {
		t.Fatalf("bzfirmware method = %v, want Deflate", zr.File[0].Method)
	}

	if _, statErr := os.Stat(filepath.Join(dataDir, ".flash-download.tmp.zip")); !os.IsNotExist(statErr) {
		t.Fatalf("scratch file must be removed, stat err = %v", statErr)
	}
}

func TestDumpFlashZipCompatDumpErrorCleansUpTempFile(t *testing.T) {
	dataDir := t.TempDir()
	svc := &Service{
		cfg:    config.Config{DataDir: dataDir},
		engine: &compatFakeEngine{dumpErr: errors.New("boom")},
	}

	var out bytes.Buffer
	err := svc.dumpFlashZipCompat(context.Background(), "repo", "snap", "/host/boot", &out, restic.Mode{})
	if err == nil {
		t.Fatal("expected an error when DumpZip fails")
	}
	if out.Len() != 0 {
		t.Fatal("no bytes may be written to dst when the dump fails")
	}
	if _, statErr := os.Stat(filepath.Join(dataDir, ".flash-download.tmp.zip")); !os.IsNotExist(statErr) {
		t.Fatalf("scratch file must be removed even on dump error, stat err = %v", statErr)
	}
}
