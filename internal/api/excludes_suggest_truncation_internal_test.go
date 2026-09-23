package api

import (
	"context"
	"errors"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// A scan that cannot finish must not present a fraction as a total, and must
// not drop the folder the user is looking for. These tests abort in the middle
// of the walk to reach the code behind both.

// countdownCtx expires after n Err() calls. scanExcludeCandidates checks
// ctx.Err() once per walk callback, so n picks the entry the walk stops on
// without racing a clock.
type countdownCtx struct {
	context.Context
	left *int
}

func (c countdownCtx) Err() error {
	if *c.left <= 0 {
		return context.DeadlineExceeded
	}
	*c.left--
	return nil
}

// abortAfter returns a context that lets exactly n walk callbacks through.
func abortAfter(n int) context.Context {
	left := n
	return countdownCtx{Context: context.Background(), left: &left}
}

// candByRel indexes a candidate slice for assertions.
func candByRel(cands []suggestCandidate) map[string]suggestCandidate {
	out := make(map[string]suggestCandidate, len(cands))
	for _, c := range cands {
		out[c.rel] = c
	}
	return out
}

// midWalkTree writes the fixture of the truncation tests. The walk is lexical
// depth-first:
//
//	1 root  2 aaa  3 aaa/f.bin  4 bbb  5 bbb/g.bin  6 bbb/sub  7 bbb/sub/f.bin
//	8 ccc  9 ccc/f.bin
func midWalkTree(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeSized(t, filepath.Join(root, "aaa", "f.bin"), 3000)
	writeSized(t, filepath.Join(root, "bbb", "g.bin"), 1000)
	writeSized(t, filepath.Join(root, "bbb", "sub", "f.bin"), 5000)
	writeSized(t, filepath.Join(root, "ccc", "f.bin"), 7000)
	return root
}

// After an abort mid-walk every candidate marked complete carries its full
// size, and anything cut short says so.
func TestScanExcludeCandidatesMidWalkTruncation(t *testing.T) {
	root := midWalkTree(t)
	o := suggestOpts{maxDepth: suggestMaxDepth, largeBytes: 1000}

	// Ground truth: the same tree, walked to the end.
	truth := candByRel(scanExcludeCandidates(context.Background(), root, nil, o).cands)

	// Stop on entry 7 (bbb/sub/f.bin), i.e. after 6 callbacks got through.
	sc := scanExcludeCandidates(abortAfter(6), root, nil, o)
	if !sc.truncated {
		t.Fatal("a mid-walk abort must report truncated=true")
	}
	got := candByRel(sc.cands)

	for rel, c := range got {
		if !c.complete {
			continue
		}
		want, ok := truth[rel]
		if !ok {
			t.Fatalf("candidate %q is not in the full-walk result at all: %+v", rel, sc.cands)
		}
		if c.size != want.size {
			t.Fatalf("candidate %q is marked complete with size %d, but its real size is %d; "+
				"a complete flag on a partial number is exactly defect #175", rel, c.size, want.size)
		}
	}

	// aaa finished before the abort.
	if c, ok := got["aaa"]; !ok || !c.complete || c.size != 3000 {
		t.Fatalf("aaa should be complete at 3000, got %+v (present=%v)", c, ok)
	}
	// bbb was cut short at 1000 of its 6000 and must not claim that as a total.
	if c, ok := got["bbb"]; !ok || c.complete || c.size != 1000 {
		t.Fatalf("bbb should be present as an incomplete 1000, got %+v (present=%v)", c, ok)
	}
	if truth["bbb"].size != 6000 {
		t.Fatalf("fixture drifted: bbb's real size is %d, want 6000", truth["bbb"].size)
	}
}

// On abort the incomplete set is exactly the ancestor chain of the stop point.
// That follows from the lexical depth-first walk; a parallel walk would break
// it and has to fail here.
func TestWalkDirLexicalDFSInvariant(t *testing.T) {
	root := midWalkTree(t)
	// largeBytes 0 keeps every collected directory, so the assertion covers the
	// whole set rather than the qualifying subset.
	o := suggestOpts{maxDepth: suggestMaxDepth, largeBytes: 0}

	sc := scanExcludeCandidates(abortAfter(6), root, nil, o) // stops on bbb/sub/f.bin
	var incomplete, complete []string
	for _, c := range sc.cands {
		if c.complete {
			complete = append(complete, c.rel)
		} else {
			incomplete = append(incomplete, c.rel)
		}
	}
	sort.Strings(incomplete)
	sort.Strings(complete)

	// The stop point is a file, so the walk stopped inside bbb/sub, whose chain
	// within the root is bbb/sub and bbb.
	if want := []string{"bbb", "bbb/sub"}; strings.Join(incomplete, ",") != strings.Join(want, ",") {
		t.Fatalf("incomplete set = %v, want exactly the stop point's ancestor chain %v", incomplete, want)
	}
	// Everything the walk finished is exact.
	if want := []string{"aaa"}; strings.Join(complete, ",") != strings.Join(want, ",") {
		t.Fatalf("complete set = %v, want %v (ccc was never reached, so it has no row at all)", complete, want)
	}
	if sc.stoppedAt != filepath.ToSlash(filepath.Join(root, "bbb", "sub")) {
		t.Fatalf("stoppedAt = %q, want the folder the walk stopped inside", sc.stoppedAt)
	}
}

// A lexically late large directory whose partial size falls under the
// threshold stays in a truncated scan, instead of leaving only a small cache.
func TestTruncatedLargeDirNotDropped(t *testing.T) {
	root := t.TempDir()
	writeSized(t, filepath.Join(root, "Plex", "Cache", "c.bin"), 500)
	writeSized(t, filepath.Join(root, "Plex", "Media", "aa.bin"), 40000)
	writeSized(t, filepath.Join(root, "Plex", "Media", "zz", "big.bin"), 900000)
	o := suggestOpts{maxDepth: suggestMaxDepth, largeBytes: 100000}

	// Full walk: Plex 940500 / Plex/Media 940000 / Plex/Media/zz 900000 /
	// Plex/Cache 500. Media is far over the threshold.
	full := candByRel(scanExcludeCandidates(context.Background(), root, nil, o).cands)
	if full["Plex/Media"].size != 940000 {
		t.Fatalf("fixture drifted: Plex/Media's real size is %d, want 940000", full["Plex/Media"].size)
	}

	// Walk order: 1 root  2 Plex  3 Plex/Cache  4 Plex/Cache/c.bin  5 Plex/Media
	// 6 Plex/Media/aa.bin  7 Plex/Media/zz  8 Plex/Media/zz/big.bin. Stopping on
	// entry 8 leaves Media holding 40000 of its 940000, under the threshold.
	sc := scanExcludeCandidates(abortAfter(7), root, nil, o)
	got := candByRel(sc.cands)

	media, ok := got["Plex/Media"]
	if !ok {
		t.Fatalf("Plex/Media vanished from a truncated scan (#175: the biggest offender was the one that got hidden); got %+v", sc.cands)
	}
	if media.complete {
		t.Fatalf("Plex/Media is marked complete at %d, but its real size is 940000", media.size)
	}
	if media.size != 40000 {
		t.Fatalf("Plex/Media lower bound = %d, want the 40000 the walk actually reached", media.size)
	}
	// The finished sibling keeps its exact number and is still presented as final.
	if c, ok := got["Plex/Cache"]; !ok || !c.complete || c.size != 500 {
		t.Fatalf("Plex/Cache should be a complete 500, got %+v (present=%v)", c, ok)
	}
}

// The list mixes exact sizes with lower bounds, so a plain cut could drop a
// partly measured 55 GB folder for a fully measured small one.
func TestMaxResultsCapKeepsPartials(t *testing.T) {
	var cands []suggestCandidate
	for i := 0; i < 25; i++ {
		cands = append(cands, suggestCandidate{rel: "big" + string(rune('a'+i)), size: int64(1_000_000 - i), complete: true})
	}
	// Two lower bounds that sort to the very bottom by their (partial) size.
	cands = append(cands,
		suggestCandidate{rel: "partial-one", size: 12, complete: false},
		suggestCandidate{rel: "partial-two", size: 7, complete: false},
	)
	sortSuggestCandidates(cands)

	capped := capSuggestions(cands, suggestMaxResults)
	if len(capped) != suggestMaxResults {
		t.Fatalf("cap returned %d candidates, want %d", len(capped), suggestMaxResults)
	}
	kept := candByRel(capped)
	for _, rel := range []string{"partial-one", "partial-two"} {
		if _, ok := kept[rel]; !ok {
			t.Fatalf("%q was cut by the results cap; a lower bound may never lose its slot to a known-small folder", rel)
		}
	}
	// The rest of the budget still goes to the biggest complete candidates, in order.
	if capped[0].rel != "biga" || capped[0].size != 1_000_000 {
		t.Fatalf("cap reordered the list: first entry = %+v", capped[0])
	}
}

// An unreadable subtree leaves its ancestors short by whatever it held, which
// happens a lot in messy Unraid appdata trees. The walk seam injects the error
// a permission failure delivers, since chmod 0 does not stop a directory
// listing on Windows.
func TestUnreadableSubtreeMarksAncestorsIncomplete(t *testing.T) {
	root := t.TempDir()
	writeSized(t, filepath.Join(root, "readable", "f.bin"), 3000)
	writeSized(t, filepath.Join(root, "locked", "hidden.bin"), 9000)
	bad := filepath.ToSlash(filepath.Join(root, "locked"))

	real := suggestWalkDir
	t.Cleanup(func() { suggestWalkDir = real })
	suggestWalkDir = func(r string, fn fs.WalkDirFunc) error {
		return real(r, func(p string, d fs.DirEntry, err error) error {
			if filepath.ToSlash(p) != bad {
				return fn(p, d, err)
			}
			// The exact sequence filepath.WalkDir produces when ReadDir fails: the
			// directory is announced first, then re-announced with the error, and
			// its contents are never visited.
			if e := fn(p, d, nil); e != nil {
				return e
			}
			if e := fn(p, d, errors.New("permission denied")); e != nil {
				return e
			}
			return fs.SkipDir
		})
	}

	sc := scanExcludeCandidates(context.Background(), root, nil, suggestOpts{maxDepth: suggestMaxDepth, largeBytes: 0})
	if sc.truncated {
		t.Fatal("an unreadable subtree is not a time-out; truncated must stay false")
	}
	got := candByRel(sc.cands)
	locked, ok := got["locked"]
	if !ok {
		t.Fatalf("the unreadable directory must still be offered as a candidate, got %+v", sc.cands)
	}
	if locked.complete {
		t.Fatalf("locked is marked complete at %d bytes, but its contents were never read", locked.size)
	}
	if c := got["readable"]; !c.complete || c.size != 3000 {
		t.Fatalf("the readable sibling must stay exact and final, got %+v", c)
	}
}

// A walk that reaches the end marks every candidate complete and keeps the
// usual order, sizes, reasons and suppression.
func TestCompleteScanCandidatesAreExact(t *testing.T) {
	root := t.TempDir()
	writeSized(t, filepath.Join(root, "Cache", "tiny.bin"), 10)         // junk by name
	writeSized(t, filepath.Join(root, "Cache", "sub", "more.bin"), 900) // child of junk: suppressed
	writeSized(t, filepath.Join(root, "data", "deep", "blob.bin"), 2000)
	writeSized(t, filepath.Join(root, "small", "f.bin"), 10) // under the threshold

	sc := scanExcludeCandidates(context.Background(), root, nil, suggestTestOpts())
	if sc.truncated || sc.stoppedAt != "" {
		t.Fatalf("a finished walk reports neither truncation nor a stop point: %+v", sc)
	}
	want := []struct {
		rel   string
		size  int64
		known bool
	}{
		{"data", 2000, false},
		{"data/deep", 2000, false},
		{"Cache", 910, true},
	}
	if len(sc.cands) != len(want) {
		t.Fatalf("expected %d candidates, got %d: %+v", len(want), len(sc.cands), sc.cands)
	}
	for i, w := range want {
		c := sc.cands[i]
		if c.rel != w.rel || c.size != w.size || c.known != w.known || !c.complete {
			t.Fatalf("candidate %d = %+v, want {rel:%s size:%d known:%v complete:true}", i, c, w.rel, w.size, w.known)
		}
	}
}
