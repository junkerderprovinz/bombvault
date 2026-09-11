package api_test

import (
	"reflect"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/api"
)

// Table style mirrors internal/restic/restic_args_test.go: fully literal want
// slices + one reflect.DeepEqual per case, so a mismatch prints the exact
// got/want lists. These tables ARE the locked normalization invariants
// (01-CONTEXT.md encoding Q1-Q4; 01-RESEARCH.md R3):
//
//   - maximal roots only (an include elides its include-descendants)
//   - per-class pruning (an exclusion elides exclusion-descendants; the two
//     classes deliberately coexist so an included root can carry an excluded
//     branch)
//   - orphan exclusions are preserved (the exclusions-only "explicitly
//     deselected" carrier — distinct from [] = auto-detection)
//   - a pure include list (the whitelist start-state) passes through the same
//     normalizer with no special case
//   - canonical output order: sorted includes, then sorted exclusions
//
// All paths below are container-form POSIX paths, so the string prefix tests
// are build-OS independent (same POSIX-only note as internal/paths).

func TestSplitExclusion(t *testing.T) {
	cases := []struct {
		name     string
		entry    string
		wantBare string
		wantExcl bool
	}{
		{"exclusion entry", "!/mnt/x", "/mnt/x", true},
		{"include entry", "/mnt/x", "/mnt/x", false},
		// A bare "!" parses to an empty path; rejecting it is the SETTER's job
		// (service.SetBackupPaths) so the parser itself stays pure.
		{"bare prefix parses empty", "!", "", true},
		{"deep exclusion", "!/c/appdata/plex/transcoding/cache", "/c/appdata/plex/transcoding/cache", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			bare, excluded := api.SplitExclusion(tc.entry)
			if bare != tc.wantBare || excluded != tc.wantExcl {
				t.Fatalf("SplitExclusion(%q) = (%q, %v), want (%q, %v)",
					tc.entry, bare, excluded, tc.wantBare, tc.wantExcl)
			}
		})
	}
}

func TestPruneMaximal(t *testing.T) {
	cases := []struct {
		name  string
		paths []string
		want  []string
	}{
		{
			// The paths.go:44-48 strictness primitive: /c/plex must not swallow
			// its SIBLING /c/plex2 — only strict descendants are elided.
			name:  "sibling prefix is not an ancestor",
			paths: []string{"/c/plex", "/c/plex/config", "/c/plex2/config"},
			want:  []string{"/c/plex", "/c/plex2/config"},
		},
		{
			name:  "deep descendant elided regardless of input order",
			paths: []string{"/c/plex/config/cache", "/c/plex"},
			want:  []string{"/c/plex"},
		},
		{
			name:  "no ancestors at all keeps everything in order",
			paths: []string{"/c/b", "/c/a", "/host/other"},
			want:  []string{"/c/b", "/c/a", "/host/other"},
		},
		{
			name:  "empty input stays empty",
			paths: []string{},
			want:  []string{},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := api.PruneMaximal(tc.paths)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("PruneMaximal(%v) = %v, want %v", tc.paths, got, tc.want)
			}
		})
	}
}

func TestNormalizeSelection(t *testing.T) {
	cases := []struct {
		name    string
		entries []string
		want    []string
	}{
		{
			// The canonical mixed case (RESEARCH "Normalization invariant"):
			// include elides include-descendant, exclusion elides
			// exclusion-descendant, and the classes coexist (included root +
			// excluded branch).
			name:    "mixed selection collapses to maximal roots + branch exclusions",
			entries: []string{"/c/appdata/plex", "/c/appdata/plex/config", "!/c/appdata/plex/transcoding", "!/c/appdata/plex/transcoding/cache"},
			want:    []string{"/c/appdata/plex", "!/c/appdata/plex/transcoding"},
		},
		{
			// Preserved, not dropped: an exclusions-only list is the "item
			// explicitly deselected" carrier that separates it from an empty
			// list = auto-detection (encoding Q3).
			name:    "orphan exclusion preserved (explicit-none carrier)",
			entries: []string{"!/c/appdata/plex/transcoding"},
			want:    []string{"!/c/appdata/plex/transcoding"},
		},
		{
			// The whitelist start-state goes through the SAME normalizer with no
			// special case (SELECT-04).
			name:    "pure include list passes through",
			entries: []string{"/c/appdata/plex/config", "/c/appdata/plex"},
			want:    []string{"/c/appdata/plex"},
		},
		{
			name:    "empty input yields empty",
			entries: []string{},
			want:    []string{},
		},
		{
			// Adjacent, non-nested keep-lists all survive (only strict
			// descendants are elided).
			name:    "siblings both survive",
			entries: []string{"/c/appdata/plex", "/c/appdata/media"},
			want:    []string{"/c/appdata/media", "/c/appdata/plex"},
		},
		{
			name:    "duplicates collapse",
			entries: []string{"/c/a", "/c/a", "!/c/b", "!/c/b"},
			want:    []string{"/c/a", "!/c/b"},
		},
		{
			// Canonical order: sorted includes first, then sorted exclusions,
			// regardless of the order the client sent — equal selections must
			// produce byte-identical stored sets.
			name:    "output order is canonical, not input order",
			entries: []string{"!/c/appdata/plex/transcoding", "/c/appdata/plex", "!/c/appdata/aaa", "/c/appdata/media"},
			want:    []string{"/c/appdata/media", "/c/appdata/plex", "!/c/appdata/aaa", "!/c/appdata/plex/transcoding"},
		},
		{
			name:    "exclusion class prunes itself (exclusion elides exclusion-descendant)",
			entries: []string{"!/c/appdata/plex", "!/c/appdata/plex/transcoding"},
			want:    []string{"!/c/appdata/plex"},
		},
		{
			// Cleaning on the bare path (the setter already translates through
			// toContainerPath, which Cleans — this keeps the pure function
			// self-consistent and idempotent).
			name:    "trailing slashes and dot segments normalize away",
			entries: []string{"/c/appdata/plex/", "/c/appdata/plex/./config"},
			want:    []string{"/c/appdata/plex"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := api.NormalizeSelection(tc.entries)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("NormalizeSelection(%v) = %v, want %v", tc.entries, got, tc.want)
			}
			// Idempotency: normalizing an already-normalized set is the identity,
			// so re-saves and readers can never drift the stored form.
			again := api.NormalizeSelection(got)
			if !reflect.DeepEqual(again, tc.want) {
				t.Fatalf("NormalizeSelection is not idempotent: %v -> %v", got, again)
			}
		})
	}
}
