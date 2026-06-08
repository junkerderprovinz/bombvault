package template_test

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/template"
)

func TestFileName(t *testing.T) {
	if got := template.FileName("Plex"); got != "my-Plex.xml" {
		t.Fatalf("FileName = %q want my-Plex.xml", got)
	}
	// Casing must be preserved verbatim.
	if got := template.FileName("nginx-Proxy"); got != "my-nginx-Proxy.xml" {
		t.Fatalf("FileName = %q want my-nginx-Proxy.xml", got)
	}
}

func TestReadAbsent(t *testing.T) {
	dir := t.TempDir()
	got, ok, err := template.Read(dir, "DoesNotExist")
	if err != nil {
		t.Fatalf("Read of absent template: unexpected error %v", err)
	}
	if ok {
		t.Fatalf("Read of absent template: ok = true, want false")
	}
	if got != "" {
		t.Fatalf("Read of absent template: got %q, want empty", got)
	}
}

// TestReadReturnsRealError verifies a real I/O error (not a missing file) is
// surfaced rather than swallowed as "absent". Here the expected file path is
// actually a directory, so os.ReadFile fails with something other than
// fs.ErrNotExist.
func TestReadReturnsRealError(t *testing.T) {
	dir := t.TempDir()
	// Create a directory at the exact path Read will try to open.
	clash := filepath.Join(dir, template.FileName("App"))
	if err := os.Mkdir(clash, 0o750); err != nil {
		t.Fatalf("setup mkdir: %v", err)
	}
	got, ok, err := template.Read(dir, "App")
	if err == nil {
		t.Fatalf("Read of a path that is a directory: err = nil, want a real I/O error (ok=%v got=%q)", ok, got)
	}
	if ok {
		t.Fatalf("Read with a real error: ok = true, want false")
	}
	if got != "" {
		t.Fatalf("Read with a real error: got %q, want empty", got)
	}
	if errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("Read with a real error: err must NOT be fs.ErrNotExist, got %v", err)
	}
}

func TestWriteReadRoundtrip(t *testing.T) {
	dir := t.TempDir()
	xml := "<Container><Name>Plex</Name></Container>"

	if err := template.Write(dir, "Plex", xml); err != nil {
		t.Fatalf("Write: %v", err)
	}

	got, ok, err := template.Read(dir, "Plex")
	if err != nil {
		t.Fatalf("Read after Write: unexpected error %v", err)
	}
	if !ok {
		t.Fatalf("Read after Write: ok = false, want true")
	}
	if got != xml {
		t.Fatalf("Read after Write: got %q, want %q", got, xml)
	}
}

func TestWriteCreatesDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "templates")
	if err := template.Write(dir, "App", "<x/>"); err != nil {
		t.Fatalf("Write into nonexistent dir: %v", err)
	}
	got, ok, err := template.Read(dir, "App")
	if err != nil {
		t.Fatalf("Read after Write into created dir: unexpected error %v", err)
	}
	if !ok || got != "<x/>" {
		t.Fatalf("Read after Write into created dir: ok=%v got=%q", ok, got)
	}
}

func TestWriteOverwrites(t *testing.T) {
	dir := t.TempDir()
	if err := template.Write(dir, "App", "<old/>"); err != nil {
		t.Fatalf("Write old: %v", err)
	}
	if err := template.Write(dir, "App", "<new/>"); err != nil {
		t.Fatalf("Write new: %v", err)
	}
	got, _, err := template.Read(dir, "App")
	if err != nil {
		t.Fatalf("overwrite read: unexpected error %v", err)
	}
	if got != "<new/>" {
		t.Fatalf("overwrite: got %q want <new/>", got)
	}
}
