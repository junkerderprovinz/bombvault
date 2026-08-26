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

	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/dockercli"
	"github.com/junkerderprovinz/bombvault/internal/model"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// ---------------------------------------------------------------------------
// Issue #175, the snapshot feeder. restic already recorded every file's size
// when it made the backup, so aggregating those per directory answers the
// reporter's question ("what is bloating my backup") exactly and without a
// filesystem walk — and it is all-or-nothing, so it can never produce the
// partial number the live walk had to be taught to label.
// ---------------------------------------------------------------------------

// suggestFakeDocker embeds the Docker interface (left nil) and overrides only
// Inspect — the single method the exclusion assistant reaches. Any other call
// would panic on the nil embed, which is the point: this path must stay small.
type suggestFakeDocker struct {
	dockercli.Docker
	inspect model.Inspect
}

func (f *suggestFakeDocker) Inspect(context.Context, string) (model.Inspect, error) {
	return f.inspect, nil
}

// suggestFakeEngine embeds ResticEngine (left nil) and implements only what the
// snapshot feeder uses. Ls is implemented ON PURPOSE while being forbidden: it
// counts its calls so a test can assert the suggest path never falls back onto
// the buffered listing (measured 1.36 GiB retained on a 672k-node snapshot).
type suggestFakeEngine struct {
	ResticEngine
	snaps      []restic.Snapshot
	entries    []restic.FileEntry
	streamErr  error // fails the FIRST LsStream call only (stale-lock self-heal)
	streamHook func(onEntry func(restic.FileEntry)) error
	// snapsHook runs inside Snapshots, the last engine call before the suggest
	// path decides whether to start its own listing. The singleflight test uses
	// it to hold every caller at that point, so the assertion is about the
	// singleflight and not about goroutine scheduling.
	snapsHook func()
	// mu guards the counters, which the singleflight test increments from
	// several goroutines at once. Everything else here is single-threaded.
	mu          sync.Mutex
	streamCalls int
	lsCalls     int
	unlocks     int
}

func (e *suggestFakeEngine) Snapshots(context.Context, string, restic.Mode) ([]restic.Snapshot, error) {
	if e.snapsHook != nil {
		e.snapsHook()
	}
	return e.snaps, nil
}

func (e *suggestFakeEngine) LsStream(_ context.Context, _, _ string, _ restic.Mode, onEntry func(restic.FileEntry)) error {
	e.mu.Lock()
	e.streamCalls++
	err := e.streamErr
	e.streamErr = nil // fail once, then succeed
	e.mu.Unlock()
	if err != nil {
		return err
	}
	if e.streamHook != nil {
		return e.streamHook(onEntry)
	}
	for _, en := range e.entries {
		onEntry(en)
	}
	return nil
}

// streamCallCount reads the counter under the lock, for tests that assert it
// while other goroutines may still be running.
func (e *suggestFakeEngine) streamCallCount() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.streamCalls
}

func (e *suggestFakeEngine) Ls(context.Context, string, string, restic.Mode) ([]restic.FileEntry, error) {
	e.lsCalls++
	return nil, nil
}

func (e *suggestFakeEngine) Unlock(context.Context, string, bool, restic.Mode) error {
	e.unlocks++
	return nil
}

// suggestFixture builds a Service wired to a real store, a fake Docker and a
// fake engine, with one container ("plex") whose selected backup folder is a
// real directory under the mount root. It returns the service, the engine and
// that folder.
func suggestFixture(t *testing.T) (*Service, *suggestFakeEngine, string) {
	t.Helper()
	dir := t.TempDir()
	db, err := store.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() }) // close before TempDir cleanup (Windows file lock)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	st := store.New(db)

	// A local containers repo that EXISTS (restic's config marker), so the
	// snapshot feeder is reachable at all.
	repo := filepath.Join(dir, "backups", "containers")
	if err := os.MkdirAll(repo, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "config"), []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	settings, err := st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	settings.ContainersPath = "backups/containers"
	if err := st.UpdateSettings(settings); err != nil {
		t.Fatal(err)
	}

	appdata := filepath.Join(dir, "appdata", "plex")
	if err := os.MkdirAll(appdata, 0o750); err != nil {
		t.Fatal(err)
	}
	if _, err := st.UpsertTarget(store.Target{ContainerName: "plex"}); err != nil {
		t.Fatal(err)
	}
	if err := st.SetBackupPaths("plex", []string{filepath.ToSlash(appdata)}); err != nil {
		t.Fatal(err)
	}

	eng := &suggestFakeEngine{}
	svc := &Service{
		cfg:          config.Config{AppKey: strings.Repeat("a", 64), DataDir: dir, HostMountRoot: dir},
		store:        st,
		docker:       &suggestFakeDocker{},
		engine:       eng,
		suggestCache: map[string]suggestCacheEntry{},
	}
	return svc, eng, filepath.ToSlash(appdata)
}

// snapEntries turns a real directory tree into the node stream `restic ls`
// would emit for it: one entry per directory and file, absolute paths, file
// sizes as recorded. This is what makes the equivalence assertion below a claim
// about the two FEEDERS rather than about two hand-written fixtures.
func snapEntries(t *testing.T, root string) []restic.FileEntry {
	t.Helper()
	var out []restic.FileEntry
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		full := filepath.ToSlash(p)
		if d.IsDir() {
			out = append(out, restic.FileEntry{Path: full, Type: "dir"})
			return nil
		}
		info, iErr := d.Info()
		if iErr != nil {
			return iErr
		}
		out = append(out, restic.FileEntry{Path: full, Type: "file", Size: info.Size()})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// streamOf replays a fixed entry list as an aggregateSnapshotCandidates feeder.
func streamOf(entries []restic.FileEntry) func(func(restic.FileEntry)) error {
	return func(onEntry func(restic.FileEntry)) error {
		for _, e := range entries {
			onEntry(e)
		}
		return nil
	}
}

// TestSnapshotAggregateMatchesLiveWalk encodes the equivalence claim the whole
// design rests on: the same tree through both feeders yields the same candidate
// list — sizes, order, reasons and completeness. If this ever diverges, the UI's
// "sizes come from the backup" promise is no longer the same number the walk
// would have shown.
func TestSnapshotAggregateMatchesLiveWalk(t *testing.T) {
	root := t.TempDir()
	writeSized(t, filepath.Join(root, "Cache", "tiny.bin"), 10)
	writeSized(t, filepath.Join(root, "Media", "deep", "blob.bin"), 5000)
	writeSized(t, filepath.Join(root, "Media", "aa.bin"), 2000)
	writeSized(t, filepath.Join(root, "small", "f.bin"), 10)
	o := suggestTestOpts()

	live := scanExcludeCandidates(context.Background(), root, nil, o).cands
	snap, err := aggregateSnapshotCandidates(streamOf(snapEntries(t, root)), []string{filepath.ToSlash(root)}, nil, o)
	if err != nil {
		t.Fatal(err)
	}
	if len(live) == 0 {
		t.Fatal("fixture produced no candidates at all")
	}
	if len(snap) != len(live) {
		t.Fatalf("snapshot feeder produced %d candidates, live walk %d:\n snap=%+v\n live=%+v", len(snap), len(live), snap, live)
	}
	for i := range live {
		l, s := live[i], snap[i]
		if l.rel != s.rel || l.size != s.size || l.known != s.known || l.complete != s.complete {
			t.Fatalf("candidate %d diverges: live=%+v snapshot=%+v", i, l, s)
		}
	}
}

// TestSnapshotTreeRespectsCurrentExcludes: an exclude pattern added SINCE the
// last backup still prunes those nodes. The directories are physically in the
// snapshot (restic stored them before the pattern existed), so the pruning has
// to happen while reading it, not by trusting the backup's own exclusion.
func TestSnapshotTreeRespectsCurrentExcludes(t *testing.T) {
	root := t.TempDir()
	writeSized(t, filepath.Join(root, "app", "logs", "big.log"), 5000)
	writeSized(t, filepath.Join(root, "keep", "f.bin"), 2000)
	entries := snapEntries(t, root)

	cands, err := aggregateSnapshotCandidates(streamOf(entries), []string{filepath.ToSlash(root)}, []string{"logs"}, suggestTestOpts())
	if err != nil {
		t.Fatal(err)
	}
	// logs is pruned AND not counted, so "app" holds nothing and never qualifies —
	// exactly what the live walk does with the same pattern.
	if len(cands) != 1 || cands[0].rel != "keep" || cands[0].size != 2000 {
		t.Fatalf("expected only keep(2000), got %+v", cands)
	}
}

// TestSnapshotRootsIntersection: roots are DERIVED from snap.Paths ∩ the
// container's current selection. A snapshot path the user has since deselected
// contributes nothing, and a newly selected folder that is not in the snapshot
// yet produces no phantom candidates.
func TestSnapshotRootsIntersection(t *testing.T) {
	got := snapshotRoots(
		[]string{"/host/user/appdata/plex", "/host/user/appdata/dropped"},
		[]string{"/host/user/appdata/plex", "/host/user/appdata/brand-new"},
	)
	if len(got) != 1 || got[0] != "/host/user/appdata/plex" {
		t.Fatalf("roots = %v, want only the folder present in BOTH", got)
	}

	// And the aggregate over that intersection ignores nodes outside it.
	root := t.TempDir()
	writeSized(t, filepath.Join(root, "keep", "f.bin"), 2000)
	entries := append(snapEntries(t, root),
		restic.FileEntry{Path: "/elsewhere/dropped", Type: "dir"},
		restic.FileEntry{Path: "/elsewhere/dropped/huge.bin", Type: "file", Size: 9_000_000},
	)
	cands, err := aggregateSnapshotCandidates(streamOf(entries), []string{filepath.ToSlash(root)}, nil, suggestTestOpts())
	if err != nil {
		t.Fatal(err)
	}
	if len(cands) != 1 || cands[0].rel != "keep" {
		t.Fatalf("a node under no selected root must not become a candidate, got %+v", cands)
	}
}

// TestSnapshotPassIsAllOrNothing: a listing that outruns its budget yields ZERO
// suggestions and an index-read failure. Asserted positively — there must be no
// code path that hands back a partially consumed stream, because that is defect
// #175 with a different feeder behind it.
func TestSnapshotPassIsAllOrNothing(t *testing.T) {
	svc, eng, root := suggestFixture(t)
	eng.snaps = []restic.Snapshot{{ID: "abc123", Time: "2026-08-01T10:00:00Z", Paths: []string{root}, Tags: []string{"container:plex"}}}
	// Emits a few real nodes, then dies the way a blown deadline does.
	eng.streamHook = func(onEntry func(restic.FileEntry)) error {
		onEntry(restic.FileEntry{Path: root + "/Cache", Type: "dir"})
		onEntry(restic.FileEntry{Path: root + "/Cache/f.bin", Type: "file", Size: 10})
		return context.DeadlineExceeded
	}

	res, err := svc.SuggestExcludes(context.Background(), "plex", "")
	if !errors.Is(err, errSuggestIndexRead) {
		t.Fatalf("err = %v, want an index-read failure", err)
	}
	if len(res.Suggestions) != 0 {
		t.Fatalf("a snapshot pass that did not finish must return NO suggestions, got %+v", res.Suggestions)
	}
	if res.Source != suggestSourceSnapshot {
		t.Fatalf("source = %q, want %q so the UI can offer the live scan", res.Source, suggestSourceSnapshot)
	}
	// And nothing was cached from the half-read stream.
	if _, ok := svc.suggestCacheGet("plex", suggestExcludesKey("x", "abc123", []string{root}, nil)); ok {
		t.Fatal("a failed pass must not seed the cache")
	}
}

// TestNoSnapshotFallsBackToLive: a container that has never been backed up is
// exactly the user who wants to pre-exclude Plex junk BEFORE the first huge
// backup. Only a walk can serve them, so the absence of a snapshot must fall
// through to the live source rather than fail.
func TestNoSnapshotFallsBackToLive(t *testing.T) {
	svc, eng, root := suggestFixture(t)
	eng.snaps = nil // never backed up
	writeSized(t, filepath.Join(filepath.FromSlash(root), "Cache", "f.bin"), 10)

	res, err := svc.SuggestExcludes(context.Background(), "plex", "")
	if err != nil {
		t.Fatal(err)
	}
	if res.Source != suggestSourceLive {
		t.Fatalf("source = %q, want %q", res.Source, suggestSourceLive)
	}
	if res.SnapshotTime != "" {
		t.Fatalf("a live scan has no snapshot time, got %q", res.SnapshotTime)
	}
	if len(res.Suggestions) != 1 || res.Suggestions[0].Path != "Cache" || !res.Suggestions[0].Complete {
		t.Fatalf("expected one complete Cache suggestion from the walk, got %+v", res.Suggestions)
	}
	if eng.streamCalls != 0 {
		t.Fatalf("no snapshot exists, so the index must not be read at all (calls=%d)", eng.streamCalls)
	}
}

// TestSnapshotCacheKeyIncludesExcludes: the same snapshot plus the same
// resolved excludes reads the index ONCE (a rescan is instant until the next
// backup); editing the excludes changes what gets pruned, so it recomputes.
func TestSnapshotCacheKeyIncludesExcludes(t *testing.T) {
	svc, eng, root := suggestFixture(t)
	eng.snaps = []restic.Snapshot{{ID: "abc123", Time: "2026-08-01T10:00:00Z", Paths: []string{root}, Tags: []string{"container:plex"}}}
	eng.entries = []restic.FileEntry{
		{Path: root, Type: "dir"},
		{Path: root + "/Cache", Type: "dir"},
		{Path: root + "/Cache/f.bin", Type: "file", Size: 10},
	}

	first, err := svc.SuggestExcludes(context.Background(), "plex", "")
	if err != nil {
		t.Fatal(err)
	}
	if first.Source != suggestSourceSnapshot || first.SnapshotTime != "2026-08-01T10:00:00Z" {
		t.Fatalf("first pass = %+v, want the snapshot source with its timestamp", first)
	}
	if len(first.Suggestions) != 1 || first.Suggestions[0].Path != "Cache" || !first.Suggestions[0].Complete {
		t.Fatalf("snapshot suggestions = %+v, want one complete Cache", first.Suggestions)
	}
	if _, err := svc.SuggestExcludes(context.Background(), "plex", ""); err != nil {
		t.Fatal(err)
	}
	if eng.streamCalls != 1 {
		t.Fatalf("a second scan of the same snapshot must be served from cache (LsStream calls=%d)", eng.streamCalls)
	}

	// Same snapshot, different resolved excludes → different pruning → recompute.
	if err := svc.store.SetExcludes("plex", []string{"Cache"}); err != nil {
		t.Fatal(err)
	}
	after, err := svc.SuggestExcludes(context.Background(), "plex", "")
	if err != nil {
		t.Fatal(err)
	}
	if eng.streamCalls != 2 {
		t.Fatalf("changed excludes must recompute (LsStream calls=%d)", eng.streamCalls)
	}
	if len(after.Suggestions) != 0 {
		t.Fatalf("the excluded folder must be gone from the suggestions, got %+v", after.Suggestions)
	}
}

// TestSuggestNeverCallsBufferedLs is the enforceable form of the memory guard:
// the suggest path must reach the index only through LsStream. Ls buffers the
// whole listing and parseFileEntries splits that buffer, which measured 1355 MiB
// retained on a 672k-node snapshot inside a memory-capped container.
func TestSuggestNeverCallsBufferedLs(t *testing.T) {
	svc, eng, root := suggestFixture(t)
	eng.snaps = []restic.Snapshot{{ID: "abc123", Time: "2026-08-01T10:00:00Z", Paths: []string{root}, Tags: []string{"container:plex"}}}
	eng.entries = []restic.FileEntry{
		{Path: root + "/Cache", Type: "dir"},
		{Path: root + "/Cache/f.bin", Type: "file", Size: 10},
	}
	if _, err := svc.SuggestExcludes(context.Background(), "plex", ""); err != nil {
		t.Fatal(err)
	}
	if eng.lsCalls != 0 {
		t.Fatalf("the exclusion assistant called the BUFFERED Ls %d time(s); it must only ever stream", eng.lsCalls)
	}
	if eng.streamCalls != 1 {
		t.Fatalf("LsStream calls = %d, want 1", eng.streamCalls)
	}
}

// TestSuggestLsStaleLockSelfHeal: a stale exclusive lock left by an interrupted
// write elsewhere in the repo blocks even a shared-lock listing. Without the
// self-heal this new path would reintroduce #129 verbatim — clear stale locks,
// retry exactly once.
func TestSuggestLsStaleLockSelfHeal(t *testing.T) {
	svc, eng, root := suggestFixture(t)
	eng.snaps = []restic.Snapshot{{ID: "abc123", Time: "2026-08-01T10:00:00Z", Paths: []string{root}, Tags: []string{"container:plex"}}}
	eng.entries = []restic.FileEntry{
		{Path: root + "/Cache", Type: "dir"},
		{Path: root + "/Cache/f.bin", Type: "file", Size: 10},
	}
	eng.streamErr = errors.New("repository is already locked exclusively by PID 123")

	res, err := svc.SuggestExcludes(context.Background(), "plex", "")
	if err != nil {
		t.Fatalf("a stale lock must self-heal, got %v", err)
	}
	if eng.unlocks != 1 {
		t.Fatalf("unlockStale calls = %d, want exactly 1", eng.unlocks)
	}
	if eng.streamCalls != 2 {
		t.Fatalf("LsStream calls = %d, want 2 (the failure and ONE retry)", eng.streamCalls)
	}
	if len(res.Suggestions) != 1 || res.Suggestions[0].Path != "Cache" {
		t.Fatalf("suggestions after the retry = %+v", res.Suggestions)
	}
}

// TestSuggestLiveSourceForced: the UI's explicit retry after a failed index read
// asks for the live walk by name, and gets it even though a snapshot exists.
func TestSuggestLiveSourceForced(t *testing.T) {
	svc, eng, root := suggestFixture(t)
	eng.snaps = []restic.Snapshot{{ID: "abc123", Time: "2026-08-01T10:00:00Z", Paths: []string{root}, Tags: []string{"container:plex"}}}
	writeSized(t, filepath.Join(filepath.FromSlash(root), "Cache", "f.bin"), 10)

	res, err := svc.SuggestExcludes(context.Background(), "plex", suggestSourceLive)
	if err != nil {
		t.Fatal(err)
	}
	if res.Source != suggestSourceLive || eng.streamCalls != 0 {
		t.Fatalf("source = %q with %d index reads, want a pure live walk", res.Source, eng.streamCalls)
	}
	if len(res.Suggestions) != 1 || res.Suggestions[0].Path != "Cache" {
		t.Fatalf("suggestions = %+v", res.Suggestions)
	}
	if res.Truncated {
		t.Fatalf("a tiny tree cannot truncate: %+v", res)
	}
}
