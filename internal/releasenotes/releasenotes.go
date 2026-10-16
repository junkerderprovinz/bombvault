// Package releasenotes embeds BombVault's release notes so the "What's new"
// dialog is served from the same origin. The app's Content-Security-Policy
// (connect-src 'self') blocks fetching them from api.github.com, and a local
// copy also works offline and without rate limits.
package releasenotes

import (
	"embed"
	"regexp"
	"sort"
	"strconv"
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

// Between returns the tags of the embedded notes newer than from and not newer
// than to, newest first. An empty bound leaves that side open.
func Between(from, to string) []string {
	lo, hasLo := parse(Tag(from))
	hi, hasHi := parse(Tag(to))
	entries, err := notesFS.ReadDir("notes")
	if err != nil {
		return nil
	}
	type note struct {
		tag string
		n   [3]int
	}
	var notes []note
	for _, e := range entries {
		tag := strings.TrimSuffix(e.Name(), ".md")
		n, ok := parse(tag)
		if !ok || (hasLo && !newer(n, lo)) || (hasHi && newer(n, hi)) {
			continue
		}
		notes = append(notes, note{tag, n})
	}
	sort.Slice(notes, func(i, j int) bool { return newer(notes[i].n, notes[j].n) })
	tags := make([]string, len(notes))
	for i, n := range notes {
		tags[i] = n.tag
	}
	return tags
}

func parse(tag string) ([3]int, bool) {
	var n [3]int
	m := verRe.FindString(tag)
	if m == "" {
		return n, false
	}
	for i, part := range strings.SplitN(m, ".", 3) {
		n[i], _ = strconv.Atoi(part)
	}
	return n, true
}

func newer(a, b [3]int) bool {
	for i := range a {
		if a[i] != b[i] {
			return a[i] > b[i]
		}
	}
	return false
}
