package api

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/model"
)

// writeSized creates a file of exactly n bytes, parents included.
func writeSized(t *testing.T, p string, n int) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, bytes.Repeat([]byte("x"), n), 0o600); err != nil {
		t.Fatal(err)
	}
}

// suggestTestOpts shrinks the size threshold so tests don't need 200 MiB trees.
func suggestTestOpts() suggestOpts {
	return suggestOpts{maxDepth: suggestMaxDepth, largeBytes: 1000}
}

func TestScanExcludeCandidatesKnownAndLarge(t *testing.T) {
	root := t.TempDir()
	writeSized(t, filepath.Join(root, "Cache", "tiny.bin"), 10)         // junk by name, size irrelevant
	writeSized(t, filepath.Join(root, "data", "deep", "blob.bin"), 2e3) // large by size, attributed from depth 2
	writeSized(t, filepath.Join(root, "small", "f.bin"), 10)            // neither

	sc := scanExcludeCandidates(context.Background(), root, nil, suggestTestOpts())
	cands := sc.cands
	if sc.truncated {
		t.Fatal("scan should not be truncated")
	}
	// data (2000, large) sorts before Cache (10, known). small never qualifies.
	// data/deep (2000, large child of a merely-large parent) stays visible.
	want := map[string]struct {
		size  int64
		known bool
	}{
		"data":      {2000, false},
		"data/deep": {2000, false},
		"Cache":     {10, true},
	}
	if len(cands) != len(want) {
		t.Fatalf("expected %d candidates, got %d: %+v", len(want), len(cands), cands)
	}
	if cands[0].rel != "data" || cands[1].rel != "data/deep" || cands[2].rel != "Cache" {
		t.Fatalf("wrong order (size desc, rel asc on tie): %+v", cands)
	}
	for _, c := range cands {
		w, ok := want[c.rel]
		if !ok || c.size != w.size || c.known != w.known {
			t.Fatalf("candidate %q = {size:%d known:%v}, want %+v", c.rel, c.size, c.known, w)
		}
	}
}

func TestScanExcludeCandidatesSkipsExcluded(t *testing.T) {
	root := t.TempDir()
	writeSized(t, filepath.Join(root, "app", "logs", "big.log"), 5e3) // excluded by a basename pattern
	writeSized(t, filepath.Join(root, "skipme", "big.bin"), 5e3)      // excluded by an anchored pattern
	writeSized(t, filepath.Join(root, "keep", "f.bin"), 2e3)

	patterns := []string{
		"logs", // basename, matches at any depth like restic
		filepath.ToSlash(filepath.Join(root, "skipme")),
	}
	cands := scanExcludeCandidates(context.Background(), root, patterns, suggestTestOpts()).cands
	// Excluded subtrees are pruned and not counted: "app" holds only the pruned
	// logs, so it stays under the threshold too.
	if len(cands) != 1 || cands[0].rel != "keep" || cands[0].size != 2000 {
		t.Fatalf("expected only keep(2000), got %+v", cands)
	}
}

func TestScanExcludeCandidatesDepthBound(t *testing.T) {
	root := t.TempDir()
	// Cache sits at depth 5, beyond the bound, so it is not suggested itself,
	// but its bytes surface through every ancestor within the bound.
	writeSized(t, filepath.Join(root, "a", "b", "c", "d", "Cache", "f.bin"), 2e3)

	cands := scanExcludeCandidates(context.Background(), root, nil, suggestTestOpts()).cands
	rels := map[string]int64{}
	for _, c := range cands {
		rels[c.rel] = c.size
	}
	if _, ok := rels["a/b/c/d/Cache"]; ok {
		t.Fatalf("depth-5 dir must not be suggested: %+v", cands)
	}
	for _, want := range []string{"a", "a/b", "a/b/c", "a/b/c/d"} {
		if rels[want] != 2000 {
			t.Fatalf("ancestor %q should carry the deep size (2000), got %+v", want, cands)
		}
	}
}

func TestScanExcludeCandidatesKnownSuppressesChildren(t *testing.T) {
	root := t.TempDir()
	// Cache qualifies by name and its child by size, but excluding Cache already
	// covers the child.
	writeSized(t, filepath.Join(root, "Cache", "sub", "big.bin"), 2e3)

	cands := scanExcludeCandidates(context.Background(), root, nil, suggestTestOpts()).cands
	if len(cands) != 1 || cands[0].rel != "Cache" || cands[0].size != 2000 || !cands[0].known {
		t.Fatalf("expected only Cache(2000,known), got %+v", cands)
	}
}

func TestScanExcludeCandidatesTimeBound(t *testing.T) {
	root := t.TempDir()
	writeSized(t, filepath.Join(root, "Cache", "f.bin"), 10)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // the walk must stop at once and say so
	sc := scanExcludeCandidates(ctx, root, nil, suggestTestOpts())
	if !sc.truncated {
		t.Fatal("expired context must report truncated=true")
	}
	if len(sc.cands) != 0 {
		t.Fatalf("expired context must return no candidates, got %+v", sc.cands)
	}
	// The walk never got past the root, so the truncation banner names the root.
	if sc.stoppedAt != filepath.ToSlash(root) {
		t.Fatalf("stoppedAt = %q, want the root %q", sc.stoppedAt, filepath.ToSlash(root))
	}
}

func TestExcludeLineFor(t *testing.T) {
	s := excludeSvc()
	in := model.Inspect{Mounts: []model.Mount{
		{Type: "bind", Source: "/mnt/user/appdata/plex", Destination: "/config"},
	}}

	// A scanned dir under a mounted folder maps back to the path inside the
	// container, the inverse of resolveExcludeLine.
	full := "/host/user/user/appdata/plex/Cache"
	line := s.excludeLineFor(full, in)
	if line != "/config/Cache" {
		t.Fatalf("line = %q, want /config/Cache", line)
	}
	// Round-trip: storing that line resolves to the exact scanned path.
	if pattern, status := s.resolveExcludeLine(line, in); pattern != full || status != "translated" {
		t.Fatalf("round-trip: got (%q,%q), want (%q,translated)", pattern, status, full)
	}

	// A dir no mount covers keeps the scanned path, which resolves to itself as a
	// passthrough.
	orphan := "/host/user/user/media/movies"
	if line := s.excludeLineFor(orphan, in); line != orphan {
		t.Fatalf("orphan line = %q, want %q", line, orphan)
	}
	if pattern, status := s.resolveExcludeLine(orphan, in); pattern != orphan || status != "passthrough" {
		t.Fatalf("orphan round-trip: got (%q,%q), want (%q,passthrough)", pattern, status, orphan)
	}
}
