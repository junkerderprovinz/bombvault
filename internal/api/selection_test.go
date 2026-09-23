package api_test

import (
	"reflect"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/api"
)

func TestSplitExclusion(t *testing.T) {
	cases := []struct {
		name     string
		entry    string
		wantBare string
		wantExcl bool
	}{
		{"exclusion entry", "!/mnt/x", "/mnt/x", true},
		{"include entry", "/mnt/x", "/mnt/x", false},
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
			name:    "mixed selection collapses to maximal roots + branch exclusions",
			entries: []string{"/c/appdata/plex", "/c/appdata/plex/config", "!/c/appdata/plex/transcoding", "!/c/appdata/plex/transcoding/cache"},
			want:    []string{"/c/appdata/plex", "!/c/appdata/plex/transcoding"},
		},
		{
			// An exclusions-only list marks an explicitly deselected item; an
			// empty list means auto-detect.
			name:    "orphan exclusion preserved (explicit-none carrier)",
			entries: []string{"!/c/appdata/plex/transcoding"},
			want:    []string{"!/c/appdata/plex/transcoding"},
		},
		{
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
			again := api.NormalizeSelection(got)
			if !reflect.DeepEqual(again, tc.want) {
				t.Fatalf("NormalizeSelection is not idempotent: %v -> %v", got, again)
			}
		})
	}
}
