package template_test

import (
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
	got, ok := template.Read(dir, "DoesNotExist")
	if ok {
		t.Fatalf("Read of absent template: ok = true, want false")
	}
	if got != "" {
		t.Fatalf("Read of absent template: got %q, want empty", got)
	}
}

func TestWriteReadRoundtrip(t *testing.T) {
	dir := t.TempDir()
	xml := "<Container><Name>Plex</Name></Container>"

	if err := template.Write(dir, "Plex", xml); err != nil {
		t.Fatalf("Write: %v", err)
	}

	got, ok := template.Read(dir, "Plex")
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
	got, ok := template.Read(dir, "App")
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
	got, _ := template.Read(dir, "App")
	if got != "<new/>" {
		t.Fatalf("overwrite: got %q want <new/>", got)
	}
}
