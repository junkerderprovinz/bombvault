package api

// The exclusion assistant must not count what the stored patterns already
// exclude, or a folder of mostly `*.log` files would be suggested at its full
// size on the strength of bytes restic already skips.

import (
	"path"
	"testing"
)

// feed replays a tree parents before children, the order add() expects.
func feed(c *suggestCollector, nodes []struct {
	rel   string
	isDir bool
	size  int64
}) {
	for _, n := range nodes {
		c.add(n.rel, n.isDir, n.size)
	}
}

func TestSuggestCollectorSkipsExcludedFiles(t *testing.T) {
	tree := []struct {
		rel   string
		isDir bool
		size  int64
	}{
		{"appdata", true, 0},
		{"appdata/data.db", false, 10},
		{"appdata/app.log", false, 900},
		{"appdata/old.log", false, 90},
	}

	t.Run("a pattern that covers the files keeps their bytes out", func(t *testing.T) {
		c := newSuggestCollector("/host/user", []string{"*.log"}, suggestOpts{maxDepth: 3, largeBytes: 1})
		feed(c, tree)
		got := c.byRel["appdata"]
		if got == nil {
			t.Fatal("appdata should still be a candidate")
		}
		if got.size != 10 {
			t.Fatalf("appdata size = %d, want 10 (the excluded .log files must not be counted)", got.size)
		}
	})

	t.Run("without the pattern every file still counts", func(t *testing.T) {
		c := newSuggestCollector("/host/user", nil, suggestOpts{maxDepth: 3, largeBytes: 1})
		feed(c, tree)
		if got := c.byRel["appdata"].size; got != 1000 {
			t.Fatalf("appdata size = %d, want 1000; the guard must only drop what a pattern covers", got)
		}
	})

	t.Run("an excluded directory is still pruned whole", func(t *testing.T) {
		c := newSuggestCollector("/host/user", []string{"cache"}, suggestOpts{maxDepth: 3, largeBytes: 1})
		feed(c, []struct {
			rel   string
			isDir bool
			size  int64
		}{
			{"appdata", true, 0},
			{"appdata/cache", true, 0},
			{"appdata/cache/blob", false, 500},
			{"appdata/keep.db", false, 7},
		})
		if _, ok := c.byRel["appdata/cache"]; ok {
			t.Fatal("an excluded directory must not be a candidate")
		}
		if got := c.byRel["appdata"].size; got != 7 {
			t.Fatalf("appdata size = %d, want 7 (the pruned subtree contributes nothing)", got)
		}
	})

	t.Run("the file guard does not latch the out-of-order flag", func(t *testing.T) {
		// A skipped file must look like it was never fed, not like a feeder that
		// broke the parents-before-children contract (errSuggestNodeOrder).
		c := newSuggestCollector("/host/user", []string{"*.log"}, suggestOpts{maxDepth: 3, largeBytes: 1})
		feed(c, tree)
		if c.outOfOrder {
			t.Fatal("excluding a file must not read as a node-order violation")
		}
	})
}

// A wildcard inside an anchored path, the shape the assistant's own suggestions
// take, has to match; otherwise excluded folders are suggested and counted
// again.
func TestAnchoredWildcardPatternsMatch(t *testing.T) {
	pats := []string{"/config/*/Cache"}
	cases := []struct {
		full string
		want bool
	}{
		{"/config/plex/Cache", true},
		{"/config/sonarr/Cache", true},
		// One level only: path.Match does not let "*" cross a separator, which is
		// how restic reads it too.
		{"/config/a/b/Cache", false},
		// Literal segments still have to match.
		{"/config/plex/Media", false},
		{"/config", false},
	}
	for _, c := range cases {
		got := matchesExcludePatterns(c.full, path.Base(c.full), pats)
		if got != c.want {
			t.Errorf("matchesExcludePatterns(%q) = %v, want %v", c.full, got, c.want)
		}
	}
}

// Each root gets its own collector, so overlapping roots would suggest the same
// directory twice.
func TestDropNestedRoots(t *testing.T) {
	got := dropNestedRoots([]string{"/appdata/plex/Media", "/appdata/plex", "/appdata/sonarr"})
	want := []string{"/appdata/plex", "/appdata/sonarr"}
	if len(got) != len(want) {
		t.Fatalf("dropNestedRoots = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("dropNestedRoots = %v, want %v", got, want)
		}
	}

	// A shared prefix is not nesting; only a path boundary counts.
	sibling := dropNestedRoots([]string{"/appdata/plex", "/appdata/plex-extra"})
	if len(sibling) != 2 {
		t.Fatalf("dropNestedRoots kept %v; /appdata/plex-extra is a sibling, not a child", sibling)
	}
}
