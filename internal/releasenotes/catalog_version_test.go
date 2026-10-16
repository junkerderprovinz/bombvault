package releasenotes

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// TestTrueNASCatalogTracksLatestRelease checks both version fields in
// truenas-apps against the newest release; a mismatch makes the TrueNAS UI show
// a version other than the one it installs. The newest
// release note is the reference because api.Version is injected at build time
// and reads "dev" here. Both fields carry the image tag, which has no leading
// "v".
func TestTrueNASCatalogTracksLatestRelease(t *testing.T) {
	repoRoot := filepath.Join("..", "..")

	latest, err := latestReleaseNoteVersion(filepath.Join(repoRoot, ".github", "release-notes"))
	if err != nil {
		t.Skipf("cannot determine latest release (%v), skipping catalog version check", err)
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
			t.Skipf("catalog file %s not available (%v), skipping", c.file, err)
		}
		m := c.pattern.FindSubmatch(raw)
		if m == nil {
			t.Errorf("%s: could not find a %s line; did the file's shape change?", c.file, c.field)
			continue
		}
		if got := string(m[1]); got != latest {
			t.Errorf("%s: %s is %q but the newest release note is v%s.\n"+
				"Bump both truenas-apps/app.yaml (app_version) and truenas-apps/ix_values.yaml (tag) to %s, "+
				"without the leading v.", c.file, c.field, got, latest, latest)
		}
	}
}

// TestAndroidAppTracksLatestRelease checks the version android/gradle.properties
// carries, which F-Droid reads at the release tag to build the app.
func TestAndroidAppTracksLatestRelease(t *testing.T) {
	repoRoot := filepath.Join("..", "..")
	latest, err := latestReleaseNoteVersion(filepath.Join(repoRoot, ".github", "release-notes"))
	if err != nil {
		t.Skipf("cannot determine latest release (%v), skipping app version check", err)
	}
	raw, err := os.ReadFile(filepath.Join(repoRoot, "android", "gradle.properties")) //nolint:gosec // G304: test reads a repo-local file at a fixed relative path
	if err != nil {
		t.Fatal(err)
	}
	name := regexp.MustCompile(`(?m)^bombvaultVersion=(\S+)\s*$`).FindSubmatch(raw)
	code := regexp.MustCompile(`(?m)^bombvaultVersionCode=(\d+)\s*$`).FindSubmatch(raw)
	if name == nil || code == nil {
		t.Fatal("android/gradle.properties: bombvaultVersion or bombvaultVersionCode line missing")
	}
	if got := string(name[1]); got != latest {
		t.Errorf("android/gradle.properties: bombvaultVersion is %q but the newest release note is v%s", got, latest)
	}
	var want int
	for _, part := range strings.Split(latest, ".") {
		n, _ := strconv.Atoi(part)
		want = want*100 + n
	}
	if got, _ := strconv.Atoi(string(code[1])); got != want {
		t.Errorf("android/gradle.properties: bombvaultVersionCode is %d, %s needs %d", got, latest, want)
	}
}

// TestTrueNASAppGrantsTheCapabilitiesBombVaultUses checks the catalog entry and
// the custom-app YAML of every documentation language against each other and
// against the two the code cannot do without: SETPCAP for libvirt and
// SYS_CHROOT for entering a snapshot view.
func TestTrueNASAppGrantsTheCapabilitiesBombVaultUses(t *testing.T) {
	repoRoot := filepath.Join("..", "..")
	read := func(rel string) string {
		raw, err := os.ReadFile(filepath.Join(repoRoot, rel)) //nolint:gosec // G304: test reads a repo-local file
		if err != nil {
			t.Fatal(err)
		}
		return string(raw)
	}
	split := func(list string) []string {
		var caps []string
		for _, c := range strings.Split(list, ",") {
			caps = append(caps, strings.Trim(strings.TrimSpace(c), `"`))
		}
		slices.Sort(caps)
		return caps
	}

	section := regexp.MustCompile(`(?ms)^capabilities:\n(.*?)^\w`).FindStringSubmatch(read("truenas-apps/app.yaml"))
	if section == nil {
		t.Fatal("truenas-apps/app.yaml: no capabilities list")
	}
	var listed []string
	for _, m := range regexp.MustCompile(`(?m)^  name: (\S+)$`).FindAllStringSubmatch(section[1], -1) {
		listed = append(listed, m[1])
	}
	slices.Sort(listed)
	for _, need := range []string{"SETPCAP", "SYS_CHROOT"} {
		if !slices.Contains(listed, need) {
			t.Errorf("truenas-apps/app.yaml does not list %s", need)
		}
	}

	m := regexp.MustCompile(`add_caps\(\[([^\]]*)\]\)`).FindStringSubmatch(read("truenas-apps/templates/docker-compose.yaml"))
	if m == nil {
		t.Fatal("truenas-apps/templates/docker-compose.yaml: no add_caps line")
	}
	if got := split(m[1]); !slices.Equal(got, listed) {
		t.Errorf("the compose template adds %v, app.yaml lists %v", got, listed)
	}

	docs, err := filepath.Glob(filepath.Join(repoRoot, "docs", "getting-started*.md"))
	if err != nil || len(docs) == 0 {
		t.Fatalf("no getting-started pages: %v", err)
	}
	capAdd := regexp.MustCompile(`(?m)^    cap_add: \[([^\]]*)\]$`)
	for _, doc := range docs {
		rel := filepath.Join("docs", filepath.Base(doc))
		m := capAdd.FindStringSubmatch(read(rel))
		if m == nil {
			t.Errorf("%s: no cap_add line in the TrueNAS YAML", rel)
			continue
		}
		if got := split(m[1]); !slices.Equal(got, listed) {
			t.Errorf("%s adds %v, the catalog entry %v", rel, got, listed)
		}
	}
}

var releaseNoteRe = regexp.MustCompile(`^v(\d+)\.(\d+)\.(\d+)\.md$`)

// latestReleaseNoteVersion returns the highest vX.Y.Z release note in dir as a
// bare "X.Y.Z". It compares each segment as a number, because a string compare
// puts v8.9.0 above v8.10.0.
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
			if convErr != nil { // digits only, so this is an overflow
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
