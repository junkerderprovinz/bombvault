// Selection encoding for the flat backupPaths set: a bare entry is an included
// root, an entry prefixed with "!" is an explicitly deselected sub-branch.
//
// Everything in this file is PURE (no receiver, no store access, no config) so
// the prefix semantics stay table-testable in isolation — the store treats
// entries as opaque strings and every reader decodes through SplitExclusion
// (01-CONTEXT.md encoding Q2). This file owns the meaning of "!"; the store's
// doc comment only points here, and internal/store never interprets entries.
//
// Paths are always Linux paths (container-translated before they get here), so
// plain strings.HasPrefix on "/"-joined forms is correct regardless of the
// build OS — the same POSIX-only reasoning as internal/paths (paths.go:31-32),
// and what makes the selection tables safe on the Windows dev box.

package api

import (
	"path"
	"sort"
	"strings"
)

// ExclusionPrefix marks a deselected sub-branch inside the flat backupPaths
// set: "!/c/appdata/plex/transcoding". One list, two entry classes — zero
// schema or wire change to the stored selection (SELECT-02; 01-CONTEXT.md
// encoding Q1). Absolute paths make the prefix unambiguous: no legitimate path
// starts with "!", so the first byte classifies the entry.
const ExclusionPrefix = "!"

// SplitExclusion splits one flat-set entry into its bare path and its class:
// "!/mnt/x" → ("/mnt/x", true); "/mnt/x" → ("/mnt/x", false). A bare "!" (no
// path) parses to ("", true) — rejecting that is the CALLER's job
// (service.SetBackupPaths), so this stays a pure parser with no error path.
func SplitExclusion(entry string) (bare string, excluded bool) {
	if strings.HasPrefix(entry, ExclusionPrefix) {
		return entry[len(ExclusionPrefix):], true
	}
	return entry, false
}

// isStrictDescendant reports whether child lies strictly below ancestor, on
// cleaned POSIX paths. Appending "/" to the ancestor is what makes the test
// strict — /c/plex never matches /c/plex2/x — the exact primitive of
// internal/paths.Resolve (paths.go:44-48).
func isStrictDescendant(child, ancestor string) bool {
	c, a := path.Clean(child), path.Clean(ancestor)
	if c == a {
		return false
	}
	return strings.HasPrefix(c, a+"/")
}

// PruneMaximal drops every path that is a strict descendant of another path in
// the list, preserving the first-occurrence order of the survivors. The caller
// runs it PER CLASS (includes and exclusions never prune each other): an
// included root and an excluded branch deliberately coexist — that pair IS the
// "mount kept, volatile subfolder deselected" selection (01-CONTEXT.md
// encoding Q4). Duplicates are not descendants of each other and survive here;
// NormalizeSelection dedupes.
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

// NormalizeSelection canonicalizes a mixed, already container-translated entry
// list into the stored form: split classes → dedupe → per-class maximal-root
// pruning → canonical order (includes sorted lexically, then exclusions sorted
// lexically, each re-prefixed). Equal selections therefore store byte-identical
// sets no matter what order the client sent, and normalizing an
// already-normalized list is the identity — readers and re-saves can never
// drift the stored form.
//
// An orphan exclusion (no included ancestor) is PRESERVED, not dropped: an
// exclusions-only list is the "item explicitly deselected" carrier that keeps
// it distinct from an empty list = auto-detection (01-CONTEXT.md encoding Q3).
func NormalizeSelection(entries []string) []string {
	// Never nil: an empty selection must persist as [] — the auto-detection
	// boundary in store.SetBackupPaths.
	out := make([]string, 0, len(entries))
	var includes, excludes []string
	for _, e := range entries {
		bare, excluded := SplitExclusion(e)
		if bare == "" {
			continue // a bare "!" carries no path; the setter rejects it, normalization just skips
		}
		bare = path.Clean(bare)
		if excluded {
			excludes = append(excludes, bare)
		} else {
			includes = append(includes, bare)
		}
	}
	includes = PruneMaximal(dedupe(includes))
	excludes = PruneMaximal(dedupe(excludes))
	sort.Strings(includes)
	sort.Strings(excludes)
	for _, p := range includes {
		out = append(out, p)
	}
	for _, p := range excludes {
		out = append(out, ExclusionPrefix+p)
	}
	return out
}

// includesOnly returns the bare (included) half of a stored flat selection —
// the list a backup is actually built from. Exclusion entries are dropped, not
// transformed: exclusions never become restic positionals and never derive
// --exclude flags (locked positions L1/L14 — restic excludes do not apply to
// positional sources, so exclude-encoding a deselection would silently no-op).
// Readers that need the whole picture call SplitExclusion themselves.
func includesOnly(entries []string) []string {
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		if bare, excluded := SplitExclusion(e); !excluded && bare != "" {
			out = append(out, bare)
		}
	}
	return out
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
