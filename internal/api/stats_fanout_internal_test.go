package api

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

// statsFakeEngine records the restic calls and can hold a caller inside
// Snapshots. That is CollectStats' first engine call, made after the in-flight
// slot is taken.
type statsFakeEngine struct {
	ResticEngine
	entered chan struct{} // signalled once per Snapshots call
	release chan struct{} // only the first Snapshots call waits on this

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
	// Hold only the first caller, so a broken guard fails the test instead of
	// deadlocking it.
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

// The throttle reads the newest repo_stats row, which is written only when a
// sample finishes, so it cannot stop a second sample while one is running; the
// in-flight guard does. Holding the first caller inside Snapshots makes the
// test deterministic.
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

	// Further callers, like the other items of a round, return without running
	// restic.
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

// Once a sample has landed the throttle takes over, which also shows the guard
// was released.
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

// The growth-budget check needs one number, so it runs restic once and writes
// no repo_stats row. A row per container would clutter the Storage card and
// hold off the daily sample.
func TestPrimaryRemoteBudgetMeasuresWithoutSampling(t *testing.T) {
	eng := &statsFakeEngine{}
	svc, st := statsTestService(t, eng)
	svc.offsiteOverBudget = map[string]bool{}

	settings, err := st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	// The domain's own repository is the remote one. An item can use another
	// named repository, but the budget, alarm and latch key belong to the primary.
	settings.ContainersPath = "s3:example.com/bucket/repo"
	if err := st.UpdateSettings(settings); err != nil {
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

// Call sites pass the repository of the item they just backed up, which can be
// a named repository on another account. The budget, the alarm text and the
// "primary:"+domain latch belong to the primary, so any other repository is not
// measured against them.
func TestPrimaryRemoteBudgetIgnoresAnItemsOwnRepository(t *testing.T) {
	eng := &statsFakeEngine{}
	svc, st := statsTestService(t, eng)
	svc.offsiteOverBudget = map[string]bool{}

	settings, err := st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	settings.ContainersPath = "s3:example.com/bucket/primary"
	if err := st.UpdateSettings(settings); err != nil {
		t.Fatal(err)
	}
	if _, err := st.UpsertPrimaryRemoteTarget("containers", store.OffsiteTarget{
		Repo: "s3:example.com/bucket/primary", Enabled: true, GrowthBudgetGB: 1,
	}); err != nil {
		t.Fatal(err)
	}

	// A container on a named remote repository, which is not the primary.
	svc.checkPrimaryRemoteBudget(context.Background(), "containers", "b2:bucket/cold", settings)

	if got := strings.Join(eng.recorded(), ","); got != "" {
		t.Fatalf("the primary's growth budget measured %q against a repository that is not the primary", got)
	}
}

// A domain wired straight to maybeCollectStats would sample once per item of a
// round, with no error and no log line to show for it.
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
				"once per item during a round; use collectStatsAfterItem", domain)
		}
	}
	// The round itself samples once at the end, with the batch context.
	if !strings.Contains(string(src), `s.maybeCollectStats(bctx, "containers")`) {
		t.Error("a container round must still sample once, at the end")
	}
}
