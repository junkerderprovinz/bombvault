// A backup selection is a flat list of paths: a bare entry is an included root,
// an entry prefixed with "!" is a deselected branch. The store keeps entries as
// opaque strings and this file owns their meaning. The entries are container
// paths, so plain prefix checks on "/" are correct on any build OS.

package api

import (
	"path"
	"sort"
	"strings"
)

// ExclusionPrefix marks a deselected branch in the flat selection, as in
// "!/c/appdata/plex/transcoding". Entries are absolute paths, so the first byte
// is enough to classify one.
const ExclusionPrefix = "!"

// SplitExclusion returns entry without the exclusion prefix and whether it had
// one. A lone "!" yields ("", true); SetBackupPaths rejects it.
func SplitExclusion(entry string) (bare string, excluded bool) {
	if strings.HasPrefix(entry, ExclusionPrefix) {
		return entry[len(ExclusionPrefix):], true
	}
	return entry, false
}

// isStrictDescendant reports whether child lies strictly below ancestor.
// Comparing against ancestor+"/" keeps /c/plex from matching /c/plex2/x.
func isStrictDescendant(child, ancestor string) bool {
	c, a := path.Clean(child), path.Clean(ancestor)
	if c == a {
		return false
	}
	return strings.HasPrefix(c, a+"/")
}

// PruneMaximal drops every path that lies below another path in the list and
// keeps the order of the rest. Callers run it per class, because an included
// root with an excluded branch below it is an ordinary selection. Duplicates
// survive; NormalizeSelection removes them.
func PruneMaximal(paths []string) []string {
	out := make([]string, 0, len(paths))
	for _, p := range paths {
		maximal := true
		for _, q := range paths {
			if isStrictDescendant(p, q) {
				maximal = false
				break
			}
		}
		if maximal {
			out = append(out, p)
		}
	}
	return out
}

// NormalizeSelection brings an entry list into its stored form: cleaned,
// deduplicated and pruned per class, with the sorted includes first and the
// sorted exclusions after. Equal selections are stored identically whatever
// order the client sent, and normalizing a normalized list changes nothing.
//
// An exclusion with no included ancestor is kept. An exclusions-only list marks
// an item as explicitly deselected, which an empty list (auto-detect) cannot.
func NormalizeSelection(entries []string) []string {
	// An empty selection is stored as [], not nil (see store.SetBackupPaths).
	out := make([]string, 0, len(entries))
	var includes, excludes []string
	for _, e := range entries {
		bare, excluded := SplitExclusion(e)
		if bare == "" {
			continue
		}
		bare = path.Clean(bare)
		if excluded {
			excludes = append(excludes, bare)
		} else {
			includes = append(includes, bare)
		}
	}
	includes = dedupe(includes)
	excludes = dedupe(excludes)
	// An include below an exclusion wins and the exclusion is dropped. The flat
	// encoding cannot say "exclude this branch but keep that folder inside it",
	// since a restic --exclude swallows everything below it, and backing up too
	// much is the safer error. The tree does the same when an excluded node is
	// ticked (applyToggle). This has to run before pruning, which would drop the
	// include as redundant to the root above it and keep the exclusion.
	kept := make([]string, 0, len(excludes))
	for _, e := range excludes {
		contradicted := false
		for _, i := range includes {
			if isStrictDescendant(i, e) {
				contradicted = true
				break
			}
		}
		if !contradicted {
			kept = append(kept, e)
		}
	}
	excludes = kept
	includes = PruneMaximal(includes)
	excludes = PruneMaximal(excludes)
	sort.Strings(includes)
	sort.Strings(excludes)
	out = append(out, includes...)
	for _, p := range excludes {
		out = append(out, ExclusionPrefix+p)
	}
	return out
}

// includesOnly returns the included paths of a stored selection, which become
// the restic positionals. Exclusions are applied separately as --exclude
// patterns (excludedBranches), so snapshot Paths stay the included roots that
// restores select by.
func includesOnly(entries []string) []string {
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		if bare, excluded := SplitExclusion(e); !excluded && bare != "" {
			out = append(out, bare)
		}
	}
	return out
}

// excludedBranches returns the exclusions that lie strictly below an included
// root, escaped as restic --exclude patterns for the backup. They end up in the
// snapshot's Excludes metadata next to the user's own patterns, which is better
// than backing up a branch the user deselected.
//
// Two shapes produce no pattern (see TestExcludedBranches):
//
//   - an exclusion equal to an included root, which would filter the source
//     root itself and cannot be built in the tree UI;
//   - an exclusion with no included ancestor, which has nothing to carve from.
//
// The result keeps the input order, so the argv is deterministic.
func excludedBranches(entries []string) []string {
	out := make([]string, 0, len(entries))
	var includes []string
	for _, e := range entries {
		if bare, excluded := SplitExclusion(e); !excluded && bare != "" {
			includes = append(includes, bare)
		}
	}
	for _, e := range entries {
		bare, excluded := SplitExclusion(e)
		if !excluded || bare == "" {
			continue
		}
		for _, inc := range includes {
			if isStrictDescendant(bare, inc) {
				out = append(out, escapeGlobLiteral(bare))
				break
			}
		}
	}
	return out
}

// escapeGlobLiteral backslash-escapes restic's glob characters so a pattern
// derived from a folder matches exactly that folder. Unescaped, a folder name
// can miss itself ("[1080p]" is a character class), exclude its siblings
// ("Season [01]" matches "Season 0" and "Season 1"), or fail the whole backup
// as an invalid pattern ("Movies [2024"). TestDerivedExcludePatternsAreLiteral
// in internal/restic checks the escaped forms against the real engine.
//
// The backslash is escaped as well: "back\slash" would read "\s" as a literal
// "s" and miss the folder. Patterns the user writes in the exclusions editor
// are globs on purpose and never pass through here.
func escapeGlobLiteral(p string) string {
	var b strings.Builder
	b.Grow(len(p))
	for _, r := range p {
		switch r {
		case '\\', '*', '?', '[', ']':
			b.WriteByte('\\')
		}
		b.WriteRune(r)
	}
	return b.String()
}

// fileSetPositionals turns a stored file-set selection into restic positionals.
// A nil selection backs up the set root src as a whole. Otherwise the selection
// is normalized again and only includes at or below src are kept: the set's
// path can change after the selection was saved, and entries under the old
// root must not widen the backup. If nothing is left, the result is src alone.
func fileSetPositionals(selected []string, src string) []string {
	if selected == nil {
		return []string{src}
	}
	positionals := make([]string, 0, len(selected))
	for _, p := range includesOnly(NormalizeSelection(selected)) {
		if p == src || isStrictDescendant(p, src) {
			positionals = append(positionals, p)
		}
	}
	if len(positionals) == 0 {
		return []string{src}
	}
	return positionals
}

// mapRestorePaths maps the stored selection onto the paths the chosen snapshot
// recorded, so a selection changed since the backup cannot make the restore
// fail after the container was already stopped and removed.
//
// A snapshot path at or below a stored path is used as is. A stored path below
// a recorded path is used itself and also returned in narrowed; the caller has
// to check those against the snapshot's tree, since an --exclude may have
// carved them out. Every other stored path is skipped. Skips do not abort the
// restore; the caller aborts on an empty mapped list.
func mapRestorePaths(stored, snapshotPaths []string) (mapped, skipped, narrowed []string) {
	mapped = make([]string, 0, len(snapshotPaths))
	skipped = make([]string, 0, len(stored))
	narrowed = make([]string, 0, len(stored))
	covered := make(map[string]bool, len(stored))
	for _, q := range snapshotPaths {
		for _, p := range stored {
			if q == p || isStrictDescendant(q, p) {
				mapped = append(mapped, q)
				covered[p] = true
				break
			}
		}
	}
	for _, p := range stored {
		if covered[p] {
			continue
		}
		// Restore p itself, not its recorded ancestor. The ancestor would bring
		// back every sibling too, including branches the user deselected and
		// never backed up, and overwrite their live contents. restic's
		// "<id>:<path>" selector reaches any directory inside a snapshot
		// (TestRestoreSubtreeBelowRecordedPath in internal/restic).
		hasAncestor := false
		for _, q := range snapshotPaths {
			if isStrictDescendant(p, q) {
				hasAncestor = true
				break
			}
		}
		if !hasAncestor {
			skipped = append(skipped, p)
			continue
		}
		seen := false
		for _, m := range mapped {
			if m == p {
				seen = true
				break
			}
		}
		if !seen {
			mapped = append(mapped, p)
			narrowed = append(narrowed, p)
		}
	}
	return mapped, skipped, narrowed
}

// dedupe removes exact duplicates, preserving first-occurrence order.
func dedupe(xs []string) []string {
	seen := make(map[string]bool, len(xs))
	out := make([]string, 0, len(xs))
	for _, x := range xs {
		if !seen[x] {
			seen[x] = true
			out = append(out, x)
		}
	}
	return out
}
