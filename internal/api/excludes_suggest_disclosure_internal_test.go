package api

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/restic"
)

// ---------------------------------------------------------------------------
// Issue #175, second round. The first round closed two silent-drop sites and
// taught the live walk to label a lower bound. Reviewing it found that the fix
// had opened new instances of the SAME defect class it exists to remove: the
// user shown something false, or denied the answer without being told.
//
// Every test here pins one of those. They are grouped rather than scattered
// because they share one claim: this panel may report LESS than it knows, but
// it may never report less than it knows WITHOUT SAYING SO.
// ---------------------------------------------------------------------------

// TestCapKeepsExactlyMeasuredLargeFolder: the results cap used to seat every
// incomplete candidate before any complete one, unconditionally. Unreadable
// subtrees are unbounded, so a messy appdata tree spent the whole budget on
// zero-byte lower bounds and evicted the one exactly measured 55 GiB folder the
// user came for — the original bug pointed the other way.
func TestCapKeepsExactlyMeasuredLargeFolder(t *testing.T) {
	cands := []suggestCandidate{{rel: "Plex/Media", size: 55 << 30, complete: true}}
	for i := 0; i < 25; i++ {
		cands = append(cands, suggestCandidate{rel: "locked" + string(rune('a'+i)), size: 0, complete: false})
	}
	sortSuggestCandidates(cands)

	capped := capSuggestions(cands, suggestMaxResults)
	if len(capped) != suggestMaxResults {
		t.Fatalf("cap returned %d candidates, want %d", len(capped), suggestMaxResults)
	}
	kept := candByRel(capped)
	if _, ok := kept["Plex/Media"]; !ok {
		t.Fatalf("the exactly measured 55 GiB folder was cut by the cap in favour of zero-byte lower bounds; kept %+v", capped)
	}
	// The reservation still exists: a truncated walk's ancestor chain keeps its
	// seats rather than being sorted off the bottom of the list.
	partials := 0
	for _, c := range capped {
		if !c.complete {
			partials++
		}
	}
	if partials < suggestPartialSeats {
		t.Fatalf("only %d lower bound(s) kept, want at least the %d reserved seats", partials, suggestPartialSeats)
	}
}

// TestUnreachableRootsStillReadTheSnapshot: an unmounted array makes every
// configured path fail its stat. The request used to return an empty list at
// that point, BEFORE the snapshot feeder — even though the newest snapshot
// holds every size exactly and needs no filesystem at all. The panel then said
// "nothing left to exclude" about a container with a healthy 55 GB backup.
func TestUnreachableRootsStillReadTheSnapshot(t *testing.T) {
	svc, eng, root := suggestFixture(t)
	// The share goes away. The selection stays configured, which is the whole
	// point: the container is not stateless, its folders are unreachable.
	if err := os.RemoveAll(filepath.FromSlash(root)); err != nil {
		t.Fatal(err)
	}
	eng.snaps = []restic.Snapshot{{ID: "abc123", Time: "2026-08-01T10:00:00Z", Paths: []string{root}, Tags: []string{"container:plex"}}}
	eng.entries = []restic.FileEntry{
		{Path: root + "/Media", Type: "dir"},
		{Path: root + "/Media/big.bin", Type: "file", Size: 55 << 30},
	}

	res, err := svc.SuggestExcludes(context.Background(), "plex", "")
	if err != nil {
		t.Fatal(err)
	}
	if eng.streamCalls != 1 {
		t.Fatalf("the backup index was never opened (LsStream calls=%d); it needs no filesystem, so an unmounted share is no reason to withhold the answer it holds", eng.streamCalls)
	}
	if res.Source != suggestSourceSnapshot {
		t.Fatalf("source = %q, want %q", res.Source, suggestSourceSnapshot)
	}
	if len(res.Suggestions) != 1 || res.Suggestions[0].Path != "Media" {
		t.Fatalf("suggestions = %+v, want the 55 GiB folder the snapshot records", res.Suggestions)
	}
	if res.PathsUnavailable {
		t.Fatal("the snapshot answered, so nothing is unavailable to the user")
	}
}

// TestUnreachableRootsWithoutSnapshotSayNothingCouldBeRead: the same unmounted
// share, but with no snapshot to fall back on. There is genuinely no answer
// here, and "nothing left to exclude" is the loudest possible lie about it.
func TestUnreachableRootsWithoutSnapshotSayNothingCouldBeRead(t *testing.T) {
	svc, eng, root := suggestFixture(t)
	if err := os.RemoveAll(filepath.FromSlash(root)); err != nil {
		t.Fatal(err)
	}
	eng.snaps = nil // never backed up, or an off-site-only repo

	res, err := svc.SuggestExcludes(context.Background(), "plex", "")
	if err != nil {
		t.Fatal(err)
	}
	if !res.PathsUnavailable {
		t.Fatalf("an unreachable folder set with no backup must say so, got %+v", res)
	}
	if len(res.Suggestions) != 0 {
		t.Fatalf("suggestions = %+v, want none", res.Suggestions)
	}
}

// TestStatelessContainerStillReportsNothingFound guards the other side of the
// line above: a container with NO configured folders at all is genuinely
// stateless, and an empty list is the true answer, not a disclosure failure.
func TestStatelessContainerStillReportsNothingFound(t *testing.T) {
	svc, eng, _ := suggestFixture(t)
	if err := svc.store.SetBackupPaths("plex", nil); err != nil {
		t.Fatal(err)
	}
	res, err := svc.SuggestExcludes(context.Background(), "plex", "")
	if err != nil {
		t.Fatal(err)
	}
	if res.PathsUnavailable {
		t.Fatal("a stateless container's empty list is correct; it must not be dressed up as a failure")
	}
	if eng.streamCalls != 0 {
		t.Fatalf("nothing is configured, so the index must not be read (calls=%d)", eng.streamCalls)
	}
}

// TestLsStreamSelfHealNeverReplaysIntoFedCollectors: the stale-lock retry hands
// the SAME onEntry closure to a second listing. A first attempt that emits nodes
// and then fails a lock check re-fed every one of them, so the collector emitted
// the directory twice at double the size, under a duplicate React key. restic
// takes the lock before it emits, so a lock error after the first node is not
// the case the self-heal exists for.
func TestLsStreamSelfHealNeverReplaysIntoFedCollectors(t *testing.T) {
	svc, eng, root := suggestFixture(t)
	eng.snaps = []restic.Snapshot{{ID: "abc123", Time: "2026-08-01T10:00:00Z", Paths: []string{root}, Tags: []string{"container:plex"}}}
	eng.streamHook = func(onEntry func(restic.FileEntry)) error {
		onEntry(restic.FileEntry{Path: root + "/Media", Type: "dir"})
		onEntry(restic.FileEntry{Path: root + "/Media/big.bin", Type: "file", Size: 500 << 20})
		return errors.New("repository is already locked exclusively by PID 123")
	}

	res, err := svc.SuggestExcludes(context.Background(), "plex", "")
	if !errors.Is(err, errSuggestIndexRead) {
		t.Fatalf("err = %v, want an index-read failure: a lock error after nodes were emitted cannot be retried onto collectors that already hold them", err)
	}
	if eng.streamCalls != 1 {
		t.Fatalf("LsStream calls = %d, want 1 — replaying a partially consumed stream double-counts every node it had already emitted", eng.streamCalls)
	}
	if len(res.Suggestions) != 0 {
		t.Fatalf("suggestions = %+v, want none (all-or-nothing)", res.Suggestions)
	}
}

// TestUnreadableRootIsReported: when the walk cannot read the ROOT, there are
// no ancestors to mark and no rows to flag, so markIncomplete has nothing to
// say and truncated stays false. The result was indistinguishable from an empty
// folder — and with several roots, one unreadable root read as a finished scan
// of all of them.
func TestUnreadableRootIsReported(t *testing.T) {
	root := t.TempDir()
	writeSized(t, filepath.Join(root, "sub", "f.bin"), 3000)
	rootSlash := filepath.ToSlash(root)

	real := suggestWalkDir
	t.Cleanup(func() { suggestWalkDir = real })
	suggestWalkDir = func(r string, fn fs.WalkDirFunc) error {
		// The shape filepath.WalkDir produces when the root's own ReadDir fails:
		// announced once, re-announced with the error, contents never visited.
		return real(r, func(p string, d fs.DirEntry, err error) error {
			if filepath.ToSlash(p) != rootSlash {
				return fn(p, d, err)
			}
			if e := fn(p, d, nil); e != nil {
				return e
			}
			if e := fn(p, d, errors.New("permission denied")); e != nil {
				return e
			}
			return fs.SkipDir
		})
	}

	sc := scanExcludeCandidates(context.Background(), root, nil, suggestTestOpts())
	if len(sc.cands) != 0 {
		t.Fatalf("fixture drifted: an unreadable root yields no candidates, got %+v", sc.cands)
	}
	if !sc.rootUnreadable {
		t.Fatal("a root the walk could not read at all is indistinguishable from an empty folder unless the scan says so")
	}
}

// TestSnapshotCacheKeyIncludesRoots: the key pinned repo, snapshot ID and
// excludes but not the ROOT SET, while the roots are a live input to the
// aggregate. Deselect a folder without touching the excludes and every rescan
// until the next backup replayed candidates for folders no longer backed up.
func TestSnapshotCacheKeyIncludesRoots(t *testing.T) {
	a := suggestExcludesKey("repo", "snap1", []string{"/a", "/b"}, nil)
	b := suggestExcludesKey("repo", "snap1", []string{"/a"}, nil)
	if a == b {
		t.Fatal("narrowing the folder selection must MISS the cache; it changes what the aggregate covers")
	}
	// Merely reordering the same selection is not a change, and must still hit.
	if suggestExcludesKey("repo", "snap1", []string{"/b", "/a"}, nil) != a {
		t.Fatal("reordering the same roots must still hit the cache")
	}

	// End to end: the same snapshot, one folder deselected, must not serve the
	// deselected folder's candidates back.
	svc, eng, root := suggestFixture(t)
	other := filepath.ToSlash(filepath.Join(filepath.Dir(filepath.FromSlash(root)), "other"))
	if err := os.MkdirAll(filepath.FromSlash(other), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := svc.store.SetBackupPaths("plex", []string{root, other}); err != nil {
		t.Fatal(err)
	}
	eng.snaps = []restic.Snapshot{{ID: "abc123", Time: "2026-08-01T10:00:00Z", Paths: []string{root, other}, Tags: []string{"container:plex"}}}
	eng.entries = []restic.FileEntry{
		{Path: root + "/Keep", Type: "dir"},
		{Path: root + "/Keep/f.bin", Type: "file", Size: 900 << 20},
		{Path: other + "/Gone", Type: "dir"},
		{Path: other + "/Gone/f.bin", Type: "file", Size: 900 << 20},
	}
	if _, err := svc.SuggestExcludes(context.Background(), "plex", ""); err != nil {
		t.Fatal(err)
	}
	if err := svc.store.SetBackupPaths("plex", []string{root}); err != nil {
		t.Fatal(err)
	}
	after, err := svc.SuggestExcludes(context.Background(), "plex", "")
	if err != nil {
		t.Fatal(err)
	}
	for _, sg := range after.Suggestions {
		if strings.Contains(sg.Path, "Gone") || strings.Contains(sg.Line, "Gone") {
			t.Fatalf("a folder the user just deselected is still being offered as an exclude: %+v", after.Suggestions)
		}
	}
	if len(after.Suggestions) != 1 || after.Suggestions[0].Path != "Keep" {
		t.Fatalf("suggestions after narrowing = %+v, want only Keep", after.Suggestions)
	}
}

// TestUnexaminedRootsAreNamed: the live walk's context is shared across roots,
// so once it expires every remaining root aborts on its own first callback and
// contributes nothing. The banner still named the FIRST root's stop point, which
// says nothing about the backup folders that were never opened at all.
func TestUnexaminedRootsAreNamed(t *testing.T) {
	base := t.TempDir()
	first := filepath.Join(base, "first")
	second := filepath.Join(base, "second")
	third := filepath.Join(base, "third")
	// Walk order in `first`: 1 first  2 first/aaa  3 first/aaa/f.bin  4 first/bbb …
	writeSized(t, filepath.Join(first, "aaa", "f.bin"), 3000)
	writeSized(t, filepath.Join(first, "bbb", "f.bin"), 4000)
	writeSized(t, filepath.Join(second, "ccc", "f.bin"), 5000)
	writeSized(t, filepath.Join(third, "ddd", "f.bin"), 6000)
	roots := []string{filepath.ToSlash(first), filepath.ToSlash(second), filepath.ToSlash(third)}

	// Three callbacks get through, all inside the FIRST root, so the budget is
	// gone before the other two are ever opened.
	var res SuggestResult
	suggestLivePass(abortAfter(3), roots, nil, suggestOpts{maxDepth: suggestMaxDepth, largeBytes: 0}, &res)

	if !res.Truncated {
		t.Fatalf("a run that never opened two of three backup folders is truncated: %+v", res)
	}
	if res.StoppedAt == "" {
		t.Fatalf("the first root's stop point is still worth naming: %+v", res)
	}
	got := strings.Join(res.UnexaminedRoots, ",")
	want := roots[1] + "," + roots[2]
	if got != want {
		t.Fatalf("unexamined roots = [%s], want [%s] — StoppedAt names ONE place and claims nothing about the backup folders that were never opened at all", got, want)
	}
}

// TestSnapshotAggregateRejectsOutOfOrderNodes: the aggregate credits a file's
// bytes only to ancestors it has already seen. That is sound for restic's
// depth-first emission and silently catastrophic without it — every directory
// would read 0, fall under the threshold, and the panel would report "nothing
// left to exclude" for a 55 GB tree while flagging nothing. Snapshot candidates
// are always complete:true, so nothing downstream could ever notice.
func TestSnapshotAggregateRejectsOutOfOrderNodes(t *testing.T) {
	root := "/host/user/appdata/plex"
	entries := []restic.FileEntry{
		// The file arrives before the folder holding it.
		{Path: root + "/Media/big.bin", Type: "file", Size: 55 << 30},
		{Path: root + "/Media", Type: "dir"},
	}
	_, err := aggregateSnapshotCandidates(streamOf(entries), []string{root}, nil, suggestTestOpts())
	if !errors.Is(err, errSuggestNodeOrder) {
		t.Fatalf("err = %v, want errSuggestNodeOrder: those bytes were dropped on the floor and every candidate would still be reported as exact", err)
	}

	// And the ordinary depth-first stream is unaffected.
	ordered := []restic.FileEntry{
		{Path: root + "/Media", Type: "dir"},
		{Path: root + "/Media/big.bin", Type: "file", Size: 55 << 30},
	}
	cands, err := aggregateSnapshotCandidates(streamOf(ordered), []string{root}, nil, suggestTestOpts())
	if err != nil {
		t.Fatal(err)
	}
	if len(cands) != 1 || cands[0].size != 55<<30 {
		t.Fatalf("a correctly ordered stream must aggregate normally, got %+v", cands)
	}
}

// TestSuggestFlightJoinsInsteadOfStartingASecond pins the seam: for one key
// there is exactly one leader at a time, and the flight is retired afterwards so
// the next miss can lead.
func TestSuggestFlightJoinsInsteadOfStartingASecond(t *testing.T) {
	svc := &Service{}
	first, leader := svc.suggestFlightFor("k")
	if !leader {
		t.Fatal("the first caller for a key must lead")
	}
	second, alsoLeader := svc.suggestFlightFor("k")
	if alsoLeader {
		t.Fatal("a second caller for the same key must join, not start its own listing")
	}
	if second != first {
		t.Fatal("the joiner must wait on the leader's flight, not a fresh one")
	}
	if _, other := svc.suggestFlightFor("other"); !other {
		t.Fatal("a different key is a different listing and must lead on its own")
	}
	svc.suggestFlightDone("k", first)
	select {
	case <-first.done:
	default:
		t.Fatal("retiring a flight must release its waiters")
	}
	if _, again := svc.suggestFlightFor("k"); !again {
		t.Fatal("after the flight is retired the next caller leads")
	}
}

// TestSuggestSingleflightSharesOneListing: the panel auto-scans on open, so
// refreshes, second tabs and second viewers arrive in bursts. Without a
// singleflight each miss starts its own `restic ls` against the same repo,
// inside a container that typically runs under a memory cap.
//
// Deterministic by construction, not by timing: every caller is held inside
// Snapshots (the last engine call before the decision to list) until all of them
// have arrived, so the leader cannot finish and seed the cache first. Whatever
// order they then resume in, exactly one may reach LsStream.
func TestSuggestSingleflightSharesOneListing(t *testing.T) {
	svc, eng, root := suggestFixture(t)
	eng.snaps = []restic.Snapshot{{ID: "abc123", Time: "2026-08-01T10:00:00Z", Paths: []string{root}, Tags: []string{"container:plex"}}}
	eng.entries = []restic.FileEntry{
		{Path: root + "/Cache", Type: "dir"},
		{Path: root + "/Cache/f.bin", Type: "file", Size: 10},
	}

	const callers = 6
	var arrived sync.WaitGroup
	arrived.Add(callers)
	allArrived := make(chan struct{})
	eng.snapsHook = func() {
		arrived.Done()
		<-allArrived
	}
	go func() {
		arrived.Wait()
		close(allArrived)
	}()

	var wg sync.WaitGroup
	results := make([]SuggestResult, callers)
	errs := make([]error, callers)
	for i := 0; i < callers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			results[i], errs[i] = svc.SuggestExcludes(context.Background(), "plex", "")
		}(i)
	}
	wg.Wait()

	for i := range errs {
		if errs[i] != nil {
			t.Fatalf("caller %d: %v", i, errs[i])
		}
		if len(results[i].Suggestions) != 1 || results[i].Suggestions[0].Path != "Cache" {
			t.Fatalf("caller %d got %+v, want the same one suggestion as everyone else", i, results[i].Suggestions)
		}
	}
	if got := eng.streamCallCount(); got != 1 {
		t.Fatalf("%d concurrent scans started %d listings; a refresh must not multiply restic processes inside a memory-capped container", callers, got)
	}
}
