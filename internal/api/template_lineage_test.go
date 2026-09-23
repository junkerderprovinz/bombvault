package api

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/config"
)

// A container whose my-<name>.xml exists on the flash shows up in both
// Present and XMLByName. A name without a file, like the old template Unraid
// deletes on a rename, is absent from Present and not an error.
func TestReadTemplateLineage(t *testing.T) {
	dir := t.TempDir()
	xml := `<Container version="2"><Name>radarr-movies</Name></Container>`
	if err := os.WriteFile(filepath.Join(dir, "my-radarr-movies.xml"), []byte(xml), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	// No my-radarr.xml: Unraid deletes the old name's template on a rename.

	s := &Service{cfg: config.Config{FlashTemplatesDir: dir}}

	got := s.readTemplateLineage()

	if !got.Present["radarr-movies"] {
		t.Errorf("Present[%q] = false, want true (my-radarr-movies.xml exists)", "radarr-movies")
	}
	if got.XMLByName["radarr-movies"] != xml {
		t.Errorf("XMLByName[%q] = %q, want %q", "radarr-movies", got.XMLByName["radarr-movies"], xml)
	}
	if got.Present["radarr"] {
		t.Errorf("Present[%q] = true, want false (no my-radarr.xml on the flash)", "radarr")
	}
	if _, ok := got.XMLByName["radarr"]; ok {
		t.Errorf("XMLByName[%q] present, want absent (no file for it)", "radarr")
	}
}

// A templates directory that does not exist (not Unraid, or the flash not
// mounted yet) yields an empty lineage instead of an error that would break
// the container list.
func TestReadTemplateLineageMissingDir(t *testing.T) {
	s := &Service{cfg: config.Config{FlashTemplatesDir: filepath.Join(t.TempDir(), "does-not-exist")}}

	got := s.readTemplateLineage()

	if len(got.Present) != 0 {
		t.Errorf("Present = %+v, want empty for a missing templates dir", got.Present)
	}
	if len(got.XMLByName) != 0 {
		t.Errorf("XMLByName = %+v, want empty for a missing templates dir", got.XMLByName)
	}
}

// A stray file on the flash that does not fit the my-<name>.xml convention is
// neither an error nor mistaken for a container's template.
func TestReadTemplateLineageIgnoresNonTemplateFiles(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("hello"), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	if err := os.Mkdir(filepath.Join(dir, "my-subdir.xml"), 0o750); err != nil {
		t.Fatalf("mkdir fixture: %v", err)
	}

	s := &Service{cfg: config.Config{FlashTemplatesDir: dir}}

	got := s.readTemplateLineage()

	if len(got.Present) != 0 {
		t.Errorf("Present = %+v, want empty (only non-template entries on the flash)", got.Present)
	}
}
