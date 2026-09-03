package api

// Repo-size sampling must not pile up.
//
// The throttle above every sampling path reads the newest repo_stats row, and
// that row is written only when a sample FINISHES. So while one sample walks the
// repo, the throttle still sees the old timestamp and waves the next caller
// through. On a container round that was one three-command probe per container,
// all against the repo the round was writing to, all with --no-lock so nothing
// blocked and nothing reached the log. Reported as a box pinned at 90-100% CPU
// for a whole round, with `ps` showing nine concurrent restic processes
// (issue #189).
//
// What is pinned here: the in-flight guard, the throttle taking over once a
// sample lands, the budget check measuring instead of sampling, and the per-item
// call sites going through the round-aware hook.

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// statsFakeEngine records what was asked of restic and can hold a caller inside
// Snapshots, which is CollectStats' first engine call — the point at which the
// in-flight slot is already taken and the expensive part has not started.
type statsFakeEngine struct {
	ResticEngine
	entered chan struct{} // signalled once per Snapshots call
	release chan struct{} // ONLY the first Snapshots call waits on this

	mu       sync.Mutex
	calls    []string // "snapshots", then each stats mode
	heldOnce bool
}

func (e *statsFakeEngine) record(what string) {
	e.mu.Lock()
	e.calls = append(e.calls, what)
	e.mu.Unlock()
}

func (e *statsFakeEngine) recorded() []string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([]string(nil), e.calls...)
}

func (e *statsFakeEngine) Snapshots(_ context.Context, _ string, _ restic.Mode) ([]restic.Snapshot, error) {
	e.record("snapshots")
	if e.entered != nil {
		e.entered <- struct{}{}
	}
	// Only the FIRST caller is held. A later one must be free to run to
	// completion, or a broken guard would deadlock this test instead of failing
	// it, and a deadlocked test says nothing about what it was meant to pin.
	e.mu.Lock()
	hold := e.release != nil && !e.heldOnce
	e.heldOnce = true
	e.mu.Unlock()
	if hold {
		<-e.release
	}
	return []restic.Snapshot{{ID: "abc", Time: "2026-09-03T00:00:00Z"}}, nil
}

func (e *statsFakeEngine) Stats(_ context.Context, _, mode string, _ restic.Mode) (restic.StatsResult, error) {
	e.record(mode)
	return restic.StatsResult{TotalSize: 4 * 1024 * 1024 * 1024}, nil // 4 GiB
}

// statsTestService builds the smallest Service that can sample: a real store, a
// repo directory carrying restic's config marker (so localRepoMissing is false)
// and the fake engine.
func statsTestService(t *testing.T, eng ResticEngine) (*Service, *store.Repo) {
	t.Helper()
	dir := t.TempDir()
	db, err := store.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() }) // before TempDir cleanup: Windows holds the file
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	st := store.New(db)
	s, err := st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	s.ContainersPath = "backups/containers"
	if err := st.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}
	repo := filepath.Join(dir, "backups", "containers")
	if err := os.MkdirAll(repo, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "config"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	return &Service{
		cfg:    config.Config{AppKey: strings.Repeat("a", 64), DataDir: dir, HostMountRoot: dir},
		store:  st,
		engine: eng,
	}, st
}

// A second sample cannot start while one is in flight. The first caller is held
// inside Snapshots, which is what makes this deterministic rather than a race
// the test hopes to lose: the slot is provably taken when the assertion runs.
func TestStatsSampleDoesNotStackUp(t *testing.T) {
	eng := &statsFakeEngine{entered: make(chan struct{}, 4), release: make(chan struct{})}
	svc, _ := statsTestService(t, eng)

	done := make(chan error, 1)
	go func() { done <- svc.collectStatsGuarded(context.Background(), "containers", "local") }()

	select {
	case <-eng.entered:
	case <-time.After(10 * time.Second):
		t.Fatal("the first sample never reached the engine")
	}

	// The slot is held. Every further caller must return without touching restic,
	// which is the whole point: these are the other 43 containers of the round.
	for i := 0; i < 3; i++ {
		if err := svc.collectStatsGuarded(context.Background(), "containers", "local"); err != nil {
			t.Fatalf("a sample that finds one in flight must skip quietly, got %v", err)
		}
	}
	if got := eng.recorded(); len(got) != 1 {
		t.Fatalf("restic was run %d times while one sample was in flight (%v), want 1", len(got), got)
	}

	close(eng.release)
	if err := <-done; err != nil {
		t.Fatalf("the in-flight sample: %v", err)
	}
	// It ran to completion: both stats modes, and a row.
	if got := strings.Join(eng.recorded(), ","); got != "snapshots,raw-data,restore-size" {
		t.Fatalf("engine calls %q, want the full sample", got)
	}
}

// Once a sample has landed, the ordinary throttle takes over — and the slot is
// free again, so this also pins that the guard releases.
func TestStatsSampleThrottledAfterOneLands(t *testing.T) {
	eng := &statsFakeEngine{}
	svc, st := statsTestService(t, eng)

	if err := svc.collectStatsGuarded(context.Background(), "containers", "local"); err != nil {
		t.Fatalf("first sample: %v", err)
	}
	rows, err := st.ListRepoStats("containers", "local", 0)
	if err != nil || len(rows) != 1 {
		t.Fatalf("one sample must record one row, got %d rows err=%v", len(rows), err)
	}

	before := len(eng.recorded())
	if err := svc.collectStatsGuarded(context.Background(), "containers", "local"); err != nil {
		t.Fatalf("second sample: %v", err)
	}
	if got := eng.recorded(); len(got) != before {
		t.Fatalf("a sample taken minutes ago must throttle the next one, engine ran again: %v", got)
	}
}

// A failed sample writes no row, so the next backup retries instead of waiting
// out the 20-hour throttle. This is why the guard is an in-flight set and not an
// attempt stamp.
func TestStatsSampleRetriesAfterFailure(t *testing.T) {
	eng := &failingStatsEngine{}
	svc, st := statsTestService(t, eng)

	for i := 0; i < 2; i++ {
		if err := svc.collectStatsGuarded(context.Background(), "containers", "local"); err == nil {
			t.Fatal("a failing engine must surface its error")
		}
	}
	if eng.tries != 2 {
		t.Fatalf("a failed sample must not block the next attempt, tries=%d want 2", eng.tries)
	}
	if rows, err := st.ListRepoStats("containers", "local", 0); err != nil || len(rows) != 0 {
		t.Fatalf("a failed sample must record nothing, got %d rows err=%v", len(rows), err)
	}
}

type failingStatsEngine struct {
	ResticEngine
	tries int
}

func (e *failingStatsEngine) Snapshots(_ context.Context, _ string, _ restic.Mode) ([]restic.Snapshot, error) {
	e.tries++
	return nil, os.ErrDeadlineExceeded
}

// The growth-budget check needs one number, and it now costs one restic run.
// It used to call CollectStats: three runs, two of them computed and discarded,
// plus a repo_stats row per container that both polluted the Storage card's
// series and closed the throttle on the real once-a-day sample.
func TestPrimaryRemoteBudgetMeasuresWithoutSampling(t *testing.T) {
	eng := &statsFakeEngine{}
	svc, st := statsTestService(t, eng)
	svc.offsiteOverBudget = map[string]bool{}

	settings, err := st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.UpsertPrimaryRemoteTarget("containers", store.OffsiteTarget{
		Repo: "s3:example.com/bucket/repo", Enabled: true, GrowthBudgetGB: 1,
	}); err != nil {
		t.Fatal(err)
	}

	svc.checkPrimaryRemoteBudget(context.Background(), "containers", "s3:example.com/bucket/repo", settings)

	if got := strings.Join(eng.recorded(), ","); got != "raw-data" {
		t.Fatalf("the budget check ran %q, want a single raw-data measurement", got)
	}
	if rows, err := st.ListRepoStats("containers", "local", 0); err != nil || len(rows) != 0 {
		t.Fatalf("the budget check must not write a repo_stats row, got %d rows err=%v", len(rows), err)
	}
}

// The per-item success paths must go through the round-aware hook. A new domain
// wired straight to maybeCollectStats would reintroduce the fan-out silently:
// nothing fails, nothing logs, the box just runs hot for the length of a round.
func TestPerItemSuccessPathsUseTheRoundAwareHook(t *testing.T) {
	src, err := os.ReadFile("service.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, domain := range []string{"containers", "vms", "flash", "files", "config"} {
		if !strings.Contains(string(src), `s.collectStatsAfterItem(ctx, "`+domain+`")`) {
			t.Errorf("%s's success path must sample via collectStatsAfterItem", domain)
		}
		if strings.Contains(string(src), `s.maybeCollectStats(ctx, "`+domain+`")`) {
			t.Errorf("%s's success path calls maybeCollectStats(ctx, ...) directly, which samples "+
				"once per item during a round — use collectStatsAfterItem", domain)
		}
	}
	// The round's own sampling point is the exception, and it is deliberately
	// spelled with the batch context so it stays easy to tell apart.
	if !strings.Contains(string(src), `s.maybeCollectStats(bctx, "containers")`) {
		t.Error("a container round must still sample once, at the end")
	}
}
