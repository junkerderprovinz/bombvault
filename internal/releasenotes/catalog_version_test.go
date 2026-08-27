package releasenotes

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// TestTrueNASCatalogTracksLatestRelease pins truenas-apps/'s two version fields
// to the newest release note, so the prepared catalog entry cannot silently
// describe an older BombVault than the one actually published.
//
// This guards a drift that already happened once: app_version went to 8.0.0
// while the image tag stayed on 7.11.1, and nothing caught it — the directory's
// own README said so ("Nothing enforces it"). A catalog listing that installs a
// stale image is worse than no listing, because the version shown in the TrueNAS
// UI would not be the version running.
//
// The newest file in .github/release-notes/ is the source of truth rather than a
// constant in the code: internal/api.Version is injected at build time (it reads
// "dev" in a test binary), so there is no in-repo version string to compare
// against. Release notes are written for every release by convention and are
// already the input to TestNotesInSyncWithReleaseNotes above.
//
// Note the two fields differ by a "v": git tags are v-prefixed (v8.1.0), the
// published container image tag is bare (8.1.0), and app_version follows the
// image. See ix_values.yaml's own comment.
func TestTrueNASCatalogTracksLatestRelease(t *testing.T) {
	repoRoot := filepath.Join("..", "..")

	latest, err := latestReleaseNoteVersion(filepath.Join(repoRoot, ".github", "release-notes"))
	if err != nil {
		t.Skipf("cannot determine latest release (%v) — skipping catalog version check", err)
	}

	for _, c := range []struct {
		file    string
		pattern *regexp.Regexp
		field   string
	}{
		{filepath.Join("truenas-apps", "app.yaml"), regexp.MustCompile(`(?m)^app_version:\s*(\S+)\s*$`), "app_version"},
		{filepath.Join("truenas-apps", "ix_values.yaml"), regexp.MustCompile(`(?m)^\s+tag:\s*(\S+)\s*$`), "image tag"},
	} {
		path := filepath.Join(repoRoot, c.file)
		raw, err := os.ReadFile(path) //nolint:gosec // G304: test reads a repo-local file at a fixed relative path
		if err != nil {
			t.Skipf("catalog file %s not available (%v) — skipping", c.file, err)
		}
		m := c.pattern.FindSubmatch(raw)
		if m == nil {
			t.Errorf("%s: could not find a %s line — did the file's shape change?", c.file, c.field)
			continue
		}
		if got := string(m[1]); got != latest {
			t.Errorf("%s: %s is %q but the newest release note is v%s.\n"+
				"Bump BOTH truenas-apps/app.yaml (app_version) and truenas-apps/ix_values.yaml (tag) to %s, "+
				"without the leading v.", c.file, c.field, got, latest, latest)
		}
	}
}

var releaseNoteRe = regexp.MustCompile(`^v(\d+)\.(\d+)\.(\d+)\.md$`)

// latestReleaseNoteVersion returns the highest vX.Y.Z release note in dir as a
// bare "X.Y.Z". It compares numerically per segment, not lexically: a string
// compare puts v8.9.0 above v8.10.0, which is the same class of bug the Unraid
// plugin version check has.
func latestReleaseNoteVersion(dir string) (string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}
	var best [3]int
	found := false
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		m := releaseNoteRe.FindStringSubmatch(e.Name())
		if m == nil {
			continue
		}
		var v [3]int
		for i := 0; i < 3; i++ {
			n, convErr := strconv.Atoi(m[i+1])
			if convErr != nil { // unreachable: the regexp only matches digits
				return "", fmt.Errorf("parse %s: %w", e.Name(), convErr)
			}
			v[i] = n
		}
		if !found || newer(v, best) {
			best, found = v, true
		}
	}
	if !found {
		return "", fmt.Errorf("no vX.Y.Z release notes in %s", dir)
	}
	return strings.Join([]string{
		strconv.Itoa(best[0]), strconv.Itoa(best[1]), strconv.Itoa(best[2]),
	}, "."), nil
}

func newer(a, b [3]int) bool {
	for i := 0; i < 3; i++ {
		if a[i] != b[i] {
			return a[i] > b[i]
		}
	}
	return false
}
