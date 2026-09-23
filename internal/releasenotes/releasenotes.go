// Package releasenotes embeds BombVault's release notes so the "What's new"
// dialog is served from the same origin. The app's Content-Security-Policy
// (connect-src 'self') blocks fetching them from api.github.com, and a local
// copy also works offline and without rate limits.
package releasenotes

import (
	"embed"
	"regexp"
	"strings"
)

//go:embed notes/*.md
var notesFS embed.FS

var verRe = regexp.MustCompile(`\d+\.\d+\.\d+`)

// Tag turns a build version into its release tag, so "v5.2.1+main.<sha>"
// becomes "v5.2.1". It returns "" for "dev", "0.0.0" and anything without an
// x.y.z core, matching releaseTag() in the frontend.
func Tag(version string) string {
	m := verRe.FindString(version)
	if m == "" || m == "0.0.0" {
		return ""
	}
	return "v" + m
}

// Notes returns the embedded release notes for version's tag. It reports false
// for dev builds and versions without a note; the dialog then links to GitHub.
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
