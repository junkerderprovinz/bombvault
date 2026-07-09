package releasenotes

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTagNormalizes(t *testing.T) {
	cases := map[string]string{
		"v5.2.1":           "v5.2.1",
		"5.2.1":            "v5.2.1",
		"v5.2.1+main.abc1": "v5.2.1", // :latest build metadata
		"dev":              "",
		"0.0.0+main.abc":   "",
		"":                 "",
	}
	for in, want := range cases {
		if got := Tag(in); got != want {
			t.Errorf("Tag(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestNotesForKnownVersion(t *testing.T) {
	// v5.0.0 is embedded and non-empty; the +main.<sha> form must resolve to it.
	body, ok := Notes("v5.0.0+main.deadbee")
	if !ok || body == "" {
		t.Fatalf("Notes for v5.0.0 build should be found + non-empty; ok=%v len=%d", ok, len(body))
	}
	if _, ok := Notes("dev"); ok {
		t.Error("Notes(dev) must not be found")
	}
}

// TestNotesInSyncWithReleaseNotes guarantees every .github/release-notes/*.md is
// embedded (copied into internal/releasenotes/notes/), so a newly-released
// version never 404s the What's-new dialog (#54). If this fails after adding a
// release note, run: cp .github/release-notes/*.md internal/releasenotes/notes/
func TestNotesInSyncWithReleaseNotes(t *testing.T) {
	srcDir := filepath.Join("..", "..", ".github", "release-notes")
	entries, err := os.ReadDir(srcDir)
	if err != nil {
		t.Skipf("release-notes source dir not available (%v) — skipping sync check", err)
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		want, err := os.ReadFile(filepath.Join(srcDir, e.Name())) //nolint:gosec // G304: test reads repo-local .github/release-notes files
		if err != nil {
			t.Fatalf("read %s: %v", e.Name(), err)
		}
		got, err := notesFS.ReadFile("notes/" + e.Name())
		if err != nil {
			t.Errorf("release note %s is not embedded — copy it into internal/releasenotes/notes/", e.Name())
			continue
		}
		if strings.TrimSpace(string(want)) != strings.TrimSpace(string(got)) {
			t.Errorf("embedded note %s differs from .github/release-notes/%s — re-copy it", e.Name(), e.Name())
		}
	}
}
