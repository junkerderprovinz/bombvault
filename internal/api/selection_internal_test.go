package api

import (
	"reflect"
	"testing"
)

func TestMapRestorePaths(t *testing.T) {
	t.Run("descendant clause: a snapshot path strictly below a stored path restores as-is", func(t *testing.T) {
		gotMapped, gotSkipped, _ := mapRestorePaths([]string{"/a/b"}, []string{"/a/b/c"})
		wantMapped := []string{"/a/b/c"}
		if !reflect.DeepEqual(gotMapped, wantMapped) {
			t.Fatalf("mapped = %v, want %v", gotMapped, wantMapped)
		}
		if len(gotSkipped) != 0 {
			t.Fatalf("skipped = %v, want empty", gotSkipped)
		}
	})

	t.Run("a stored path below a recorded root restores itself, not the root", func(t *testing.T) {
		// Restoring the ancestor would overwrite deselected siblings with old
		// snapshot data.
		gotMapped, gotSkipped, _ := mapRestorePaths([]string{"/a/b/c"}, []string{"/a", "/a/b"})
		wantMapped := []string{"/a/b/c"}
		if !reflect.DeepEqual(gotMapped, wantMapped) {
			t.Fatalf("mapped = %v, want %v", gotMapped, wantMapped)
		}
		if len(gotSkipped) != 0 {
			t.Fatalf("skipped = %v, want empty", gotSkipped)
		}
	})

	t.Run("exact equality matches before any prefix logic", func(t *testing.T) {
		gotMapped, gotSkipped, _ := mapRestorePaths([]string{"/x"}, []string{"/x"})
		wantMapped := []string{"/x"}
		if !reflect.DeepEqual(gotMapped, wantMapped) {
			t.Fatalf("mapped = %v, want %v", gotMapped, wantMapped)
		}
		if len(gotSkipped) != 0 {
			t.Fatalf("skipped = %v, want empty", gotSkipped)
		}
	})

	t.Run("stored path absent from the snapshot is skipped, not mapped", func(t *testing.T) {
		gotMapped, gotSkipped, _ := mapRestorePaths([]string{"/gone"}, []string{"/other"})
		if len(gotMapped) != 0 {
			t.Fatalf("mapped = %v, want empty", gotMapped)
		}
		wantSkipped := []string{"/gone"}
		if !reflect.DeepEqual(gotSkipped, wantSkipped) {
			t.Fatalf("skipped = %v, want %v", gotSkipped, wantSkipped)
		}
	})

	t.Run("prefix test is segment-aligned: /a is not an ancestor of /ab", func(t *testing.T) {
		gotMapped, gotSkipped, _ := mapRestorePaths([]string{"/ab"}, []string{"/a"})
		if len(gotMapped) != 0 {
			t.Fatalf("mapped = %v, want empty", gotMapped)
		}
		wantSkipped := []string{"/ab"}
		if !reflect.DeepEqual(gotSkipped, wantSkipped) {
			t.Fatalf("skipped = %v, want %v", gotSkipped, wantSkipped)
		}
	})

	t.Run("mapped paths keep snapshot order; unmapped stored paths skip in stored order", func(t *testing.T) {
		gotMapped, gotSkipped, _ := mapRestorePaths(
			[]string{"/sel", "/gone1", "/gone2"},
			[]string{"/zzz/under", "/sel/sub", "/aaa/under"},
		)
		// /sel/sub is the only snapshot path at or below a stored path. The two
		// others have no recorded ancestor and skip in stored order.
		wantMapped := []string{"/sel/sub"}
		if !reflect.DeepEqual(gotMapped, wantMapped) {
			t.Fatalf("mapped = %v, want %v", gotMapped, wantMapped)
		}
		wantSkipped := []string{"/gone1", "/gone2"}
		if !reflect.DeepEqual(gotSkipped, wantSkipped) {
			t.Fatalf("skipped = %v, want %v", gotSkipped, wantSkipped)
		}
	})

	t.Run("mixed: one stored path matches a snapshot path, the other maps to itself", func(t *testing.T) {
		gotMapped, gotSkipped, _ := mapRestorePaths(
			[]string{"/sel/deep", "/other"},
			[]string{"/other/kid", "/sel"},
		)
		// /other/kid is below stored /other. /sel is recorded above /sel/deep,
		// so /sel/deep restores as itself.
		wantMapped := []string{"/other/kid", "/sel/deep"}
		if !reflect.DeepEqual(gotMapped, wantMapped) {
			t.Fatalf("mapped = %v, want %v", gotMapped, wantMapped)
		}
		if len(gotSkipped) != 0 {
			t.Fatalf("skipped = %v, want empty", gotSkipped)
		}
	})

	t.Run("two selected siblings restore as two paths, not as their shared parent", func(t *testing.T) {
		gotMapped, gotSkipped, _ := mapRestorePaths(
			[]string{"/sel/a", "/sel/b"},
			[]string{"/sel"},
		)
		// If /sel also holds a deselected c, restoring /sel would overwrite
		// its live contents.
		wantMapped := []string{"/sel/a", "/sel/b"}
		if !reflect.DeepEqual(gotMapped, wantMapped) {
			t.Fatalf("mapped = %v, want %v", gotMapped, wantMapped)
		}
		if len(gotSkipped) != 0 {
			t.Fatalf("skipped = %v, want empty", gotSkipped)
		}
	})
}

// An exclusion equal to an included root and an exclusion under no included
// root produce no --exclude pattern.
func TestExcludedBranches(t *testing.T) {
	cases := []struct {
		name    string
		entries []string
		want    []string
	}{
		{"both branches qualify, stored order preserved", []string{"/c/a", "!/c/a/b", "!/c/a/z"}, []string{"/c/a/b", "/c/a/z"}},
		{"orphan exclusion under no include is dropped", []string{"/c/a", "!/c/a/b", "!/c/other"}, []string{"/c/a/b"}},
		{"exclusions-only: no include to qualify against", []string{"!/c/a"}, []string{}},
		{"equality is not a strict descendant", []string{"/c/a", "!/c/a"}, []string{}},
		{"includes only: nothing to enforce", []string{"/c/a"}, []string{}},
		{"empty list: non-nil empty", nil, []string{}},
		{"deep branch below one qualifier wins", []string{"/c/a", "!/c/a/b/c"}, []string{"/c/a/b/c"}},
	}
	for _, tc := range cases {
		if got := excludedBranches(tc.entries); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%s: excludedBranches(%v) = %v, want %v", tc.name, tc.entries, got, tc.want)
		}
	}
}
