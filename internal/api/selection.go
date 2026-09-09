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
// --exclude flags (locked positions L1/L14). That lock is a PRODUCT choice,
// not a restic limitation — restic excludes DO filter content within positional
// sources (TestPositionalExcludesKeepSourceDir proves it against real restic:
// the positional source survives in Paths while the excluded file is filtered
// from the snapshot), so encoding "!" entries as --exclude patterns would
// enforce a deselection content-wise. It stays locked anyway because
// engine-derived patterns would land in the snapshot's restic Excludes
// metadata, which is user-owned surface (the exclusions editor previews
// exactly the patterns the user wrote — STACK.md "What NOT to Use"), and
// machine-generated entries would pollute that round-trip. Readers that need
// the whole picture call SplitExclusion themselves.
func includesOnly(entries []string) []string {
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		if bare, excluded := SplitExclusion(e); !excluded && bare != "" {
			out = append(out, bare)
		}
	}
	return out
}

// mapRestorePaths intersects the stored selection (the positional truth
// recorded at backup time, tg.AppdataPaths) with the CHOSEN snapshot's
// recorded Paths, producing the restore selector list plus the stored paths
// that could not be mapped. RESTORE-01: the stored list must never be replayed
// verbatim as restore selectors — restic's `restore <id>:<path>` selector must
// come from the snapshot's Paths (restic.go RestoreSubtreeToArgs doc), so a
// selection reshaped since the snapshot was taken would miss and fail the
// restore mid-loop AFTER the container has been stopped and removed. Mapping
// resolves that here, synchronously, before anything destructive.
//
// The returned mapped list is in SNAPSHOT-path form. Deterministic two-pass
// semantics (01-RESEARCH R5):
//
//	pass 1 — every snapshot path q (in the snapshot's own Paths order) that
//	         equals or lies strictly below some stored path p is restored
//	         as-is: a snapshot path inside a stored root is exactly what the
//	         user backed up, and is a valid selector;
//	pass 2 — every stored path p not already covered by pass 1 falls back to
//	         the LONGEST snapshot path q that is a strict ancestor of p
//	         (restoring q's subtree covers p) — longest, never
//	         first-component (RESTORE-01), appended only if not already
//	         present; a stored path matching neither clause lands in skipped.
//
// skipped is reported to the caller (scrubbed log + run-record note); a skip
// never aborts the restore — only an empty intersection does, and that check
// is the caller's (it needs the explicit nothing-to-restore error shape).
//
// Pure: no receiver, no store access, no cfg. Containers call it today;
// File Sets reuse it in Phase 4 (01-CONTEXT.md restore Q1/D-13). The
// strict-prefix primitive is isStrictDescendant — the same segment-aligned
// shape as internal/paths.Resolve (paths.go:44-48), so /a never matches /ab.
func mapRestorePaths(stored, snapshotPaths []string) (mapped, skipped []string) {
	mapped = make([]string, 0, len(snapshotPaths))
	skipped = make([]string, 0, len(stored))
	covered := make(map[string]bool, len(stored)) // stored paths pass 1 already satisfied
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
		best := ""
		for _, q := range snapshotPaths {
			// Strict ancestors of p form a prefix chain, so "longest" is also a
			// tiebreak-free total order — first-strictly-longer always wins.
			if isStrictDescendant(p, q) && len(q) > len(best) {
				best = q
			}
		}
		if best == "" {
			skipped = append(skipped, p)
			continue
		}
		seen := false
		for _, m := range mapped {
			if m == best {
				seen = true
				break
			}
		}
		if !seen {
			mapped = append(mapped, best)
		}
	}
	return mapped, skipped
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
