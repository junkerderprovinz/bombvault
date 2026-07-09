// Package releasenotes embeds BombVault's own release notes into the binary so
// the "What's new" dialog (#48) can be served locally. The dialog used to fetch
// api.github.com at runtime, but the app's own Content-Security-Policy
// (connect-src 'self') blocks that cross-origin request, so it always failed
// (#54). Serving the notes from the same origin fixes it — offline, and with no
// GitHub rate limits.
package releasenotes

import (
	"embed"
	"regexp"
	"strings"
)

//go:embed notes/*.md
var notesFS embed.FS

var verRe = regexp.MustCompile(`\d+\.\d+\.\d+`)

// Tag normalizes a build version to its release tag "vX.Y.Z". Build versions on
// :latest carry metadata ("v5.2.1+main.<sha>", issue #22); this returns
// "v5.2.1". Returns "" for "dev" / "0.0.0" / anything without an x.y.z core,
// mirroring the frontend releaseTag() so both agree on which release to show.
func Tag(version string) string {
	m := verRe.FindString(version)
	if m == "" || m == "0.0.0" {
		return ""
	}
	return "v" + m
}

// Notes returns the embedded release-notes markdown for a version (normalized to
// its tag) and true when found. Returns "", false for dev builds or a version
// whose note was not embedded, so the dialog degrades to its GitHub-link fallback.
func Notes(version string) (string, bool) {
	tag := Tag(version)
	if tag == "" {
		return "", false
	}
	b, err := notesFS.ReadFile("notes/" + tag + ".md")
	if err != nil {
		return "", false
	}
	return strings.TrimSpace(string(b)), true
}
