package api

import (
	"context"
	"database/sql"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

// The engine is the part that reacts: a finished run marks its series dirty,
// one pass judges every dirty series, and the summary the UI polls is rebuilt
// from what the pass wrote. Everything below drives it through the real store
// on its single connection, because the lock order between the pass, the store
// hook and a user's acknowledge is what these tests are about.

const (
	gib     = 1 << 30
	mib     = 1 << 20
	testNow = int64(1_800_000_000)
	// seriesFiles is a file count small enough that the file rules stay out of
	// the way, so a fixture about sizes raises the size rule and nothing else.
	seriesFiles = 40
	waitLimit   = 10 * time.Second
)

type engineFixture struct {
	svc *Service
	st  *store.Repo
	db  *sql.DB
	e   *anomalyEngine
	now int64
}

func newEngineFixture(t *testing.T) *engineFixture {
	t.Helper()
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	f := &engineFixture{st: store.New(db), db: db, now: testNow}
	f.svc = &Service{store: f.st}
	f.e = newAnomalyEngine(f.svc, func() time.Time { return time.Unix(f.now, 0) })
	f.e.debounce = 0
	f.svc.anomalies = f.e
	f.st.SetRunFinishedHook(f.e.runFinished)
	return f
}

func (f *engineFixture) pass(t *testing.T) {
	t.Helper()
	if err := f.e.passOnce(context.Background()); err != nil {
		t.Fatalf("pass: %v", err)
	}
}

// container registers a scheduled container and returns its target id.
func (f *engineFixture) container(t *testing.T, name string) string {
	t.Helper()
	target, err := f.st.UpsertTarget(store.Target{ContainerName: name, IncludeInSchedule: true})
	if err != nil {
		t.Fatal(err)
	}
	return target.ID
}

// run records a finished run of the given series and backdates it, so a series
// can be built in the order a detector reads it.
func (f *engineFixture) run(t *testing.T, targetID, kind string, at, sourceBytes int64, opts ...func(*store.RunMetrics)) string {
	t.Helper()
	m := store.RunMetrics{SourceBytes: sourceBytes, SourceFiles: seriesFiles, ResticMS: 60_000}
	hasParent := true
	m.HasParent = &hasParent
	for _, opt := range opts {
		opt(&m)
	}
	id, err := f.st.StartRun(targetID, kind)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.st.FinishRunMeasured(id, "success", "snap-"+id, sourceBytes/100, "", &m, "sel-1"); err != nil {
		t.Fatal(err)
	}
	f.backdate(t, id, at)
	return id
}

func withMS(ms int64) func(*store.RunMetrics) {
	return func(m *store.RunMetrics) { m.ResticMS = ms }
}

func (f *engineFixture) backdate(t *testing.T, runID string, at int64) {
	t.Helper()
	if _, err := f.db.Exec(`UPDATE runs SET started_at = ?, finished_at = ? WHERE id = ?`, at, at+60, runID); err != nil {
		t.Fatal(err)
	}
}

// steadySeries fills a series with n daily runs of the same size, the newest
// one day before the fixture's clock.
func (f *engineFixture) steadySeries(t *testing.T, targetID, kind string, n int, sourceBytes int64) {
	t.Helper()
	for i := n; i >= 1; i-- {
		f.run(t, targetID, kind, f.now-int64(i)*86400, sourceBytes)
	}
}

func (f *engineFixture) openRows(t *testing.T) []store.Anomaly {
	t.Helper()
	rows, _, err := f.st.ListAnomalies(store.AnomalyFilter{})
	if err != nil {
		t.Fatal(err)
	}
	return rows
}

func onlyRow(t *testing.T, rows []store.Anomaly) store.Anomaly {
	t.Helper()
	if len(rows) != 1 {
		t.Fatalf("want one open finding, got %d: %+v", len(rows), rows)
	}
	return rows[0]
}

// A collapsed source is raised as soon as the run that found it is finished,
// and the summary the dashboard polls carries it right after the pass.
func TestEngineRaisesAfterFinishRun(t *testing.T) {
	f := newEngineFixture(t)
	id := f.container(t, "nextcloud")
	f.steadySeries(t, id, "backup", 11, 40*gib)
	lastGood, _, err := f.st.ListAnomalies(store.AnomalyFilter{})
	if err != nil || len(lastGood) != 0 {
		t.Fatalf("seeding must raise nothing: %v %+v", err, lastGood)
	}
	before := f.e.summary().Generation
	good := f.run(t, id, "backup", f.now-86400/2, 40*gib)
	f.run(t, id, "backup", f.now-60, 20*mib)

	f.pass(t)

	row := onlyRow(t, f.openRows(t))
	if row.Metric != metricSourceBytesShrink || row.Severity != "critical" {
		t.Fatalf("want a critical source_bytes_shrink, got %s/%s", row.Metric, row.Severity)
	}
	if row.LastGoodRunID != good {
		t.Fatalf("last good run = %q, want %q", row.LastGoodRunID, good)
	}
	if row.TargetID != id || row.Domain != "container" || row.ScopeKind != anomalyScopeItem {
		t.Fatalf("scope fields wrong: %+v", row)
	}
	sum := f.e.summary()
	if sum.Open.Critical != 1 {
		t.Fatalf("summary critical = %d, want 1", sum.Open.Critical)
	}
	if sum.Generation <= before {
		t.Fatalf("generation did not move: %d <= %d", sum.Generation, before)
	}
}

// A database dump is a series of its own. Its few hundred megabytes must not
// read as the container's source shrinking, and a collapsed dump holds only the
// dump series.
func TestDumpRunsNeverEnterTheContainerSeries(t *testing.T) {
	f := newEngineFixture(t)
	id := f.container(t, "nextcloud")
	for i := 12; i >= 1; i-- {
		f.run(t, id, "backup", f.now-int64(i)*86400, 40*gib)
		f.run(t, id, "dbdump", f.now-int64(i)*86400+600, 300*mib)
	}
	f.run(t, id, "dbdump", f.now-600, 100<<10)

	f.pass(t)

	row := onlyRow(t, f.openRows(t))
	if row.Metric != metricDumpBytesShrink || row.ScopeKind != anomalyScopeDump {
		t.Fatalf("want a dump_bytes_shrink on the dump scope, got %s on %s", row.Metric, row.ScopeKind)
	}
	if row.Severity != "critical" {
		t.Fatalf("a collapsed dump is critical, got %s", row.Severity)
	}
}

// Runs the engine cannot attribute to a series raise nothing: an id with no
// item row, the "Backup Everything" id, and the maintenance kinds recorded on a
// domain's literal target id.
func TestEngineIgnoresUnknownTargetsAndEverything(t *testing.T) {
	f := newEngineFixture(t)
	known := f.container(t, "nextcloud")
	f.steadySeries(t, known, "backup", 12, 40*gib)

	f.steadySeries(t, "gone-with-the-target", "backup", 12, 40*gib)
	f.run(t, "gone-with-the-target", "backup", f.now-60, 1*mib)
	f.run(t, "everything", "backup", f.now-120, 1*mib)
	f.run(t, "containers", "prune", f.now-180, 1*mib)
	f.run(t, "containers", "verify", f.now-240, 1*mib)

	f.pass(t)

	if rows := f.openRows(t); len(rows) != 0 {
		t.Fatalf("want no findings, got %+v", rows)
	}
	items, err := f.svc.AnomalyItems(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range items {
		switch item.TargetID {
		case "gone-with-the-target", "everything", "containers":
			t.Fatalf("%q is not an item", item.TargetID)
		}
	}
	if itemByID(t, items, known).Learning.Samples == 0 {
		t.Fatal("the known container was not judged")
	}
}

// A pass that cannot read the item tables raises nothing and leaves the work
// for the next one, instead of silently skipping a whole domain.
func TestFailedDomainMapKeepsScopesDirty(t *testing.T) {
	f := newEngineFixture(t)
	id := f.container(t, "nextcloud")
	f.steadySeries(t, id, "backup", 12, 40*gib)
	f.run(t, id, "backup", f.now-60, 20*mib)

	real := f.e.items
	failures := 0
	f.e.items = func(s store.Settings) (map[string]anomalyItemRef, error) {
		if failures == 0 {
			failures++
			return nil, errors.New("database is locked")
		}
		return real(s)
	}

	if err := f.e.passOnce(context.Background()); err == nil {
		t.Fatal("a failed item read must report")
	}
	if rows := f.openRows(t); len(rows) != 0 {
		t.Fatalf("nothing may be raised from a failed pass, got %+v", rows)
	}
	if got := f.e.summary().EvalErrors; got != 1 {
		t.Fatalf("evalErrors = %d, want 1", got)
	}

	f.pass(t)
	if row := onlyRow(t, f.openRows(t)); row.Metric != metricSourceBytesShrink {
		t.Fatalf("the next pass must judge the series, got %s", row.Metric)
	}
	if got := f.e.summary().EvalErrors; got != 0 {
		t.Fatalf("evalErrors = %d after a clean pass, want 0", got)
	}
}

// A restore drill is recorded on its domain's literal target id, so a
// regression shows in the pass that follows the drill rather than after the
// next backup or a restart.
func TestDrillRunMakesItsDomainDirty(t *testing.T) {
	f := newEngineFixture(t)
	for _, c := range []struct {
		domain, kind string
		ok           bool
		at           int64
	}{
		{"containers", "subset", true, f.now - 3*86400},
		{"containers", "subset", false, f.now - 86400},
		{"flash", "dr", true, f.now - 3*86400},
		{"flash", "dr", false, f.now - 86400},
	} {
		err := f.st.AddRestoreDrill(store.RestoreDrill{
			Domain: c.domain, Source: c.domain, At: c.at, OK: c.ok, Kind: c.kind,
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	f.run(t, "containers", "drill", f.now-600, 0)
	f.run(t, store.FlashTargetID, "drdrill", f.now-500, 0)
	f.run(t, "containers", "prune", f.now-400, 0)

	f.pass(t)

	byMetricName := map[string]store.Anomaly{}
	for _, row := range f.openRows(t) {
		byMetricName[row.Metric] = row
	}
	if len(byMetricName) != 2 {
		t.Fatalf("want a subset and a dr finding, got %+v", byMetricName)
	}
	subset := byMetricName[metricDrillSubset]
	if subset.ScopeKind != anomalyScopeDomain || subset.ScopeID != "containers:containers" {
		t.Fatalf("subset scope = %s/%s", subset.ScopeKind, subset.ScopeID)
	}
	if dr := byMetricName[metricDrillDR]; dr.Domain != "flash" {
		t.Fatalf("dr finding domain = %q, want flash", dr.Domain)
	}
}

// The once-a-day housekeeping does not wait for a backup, and a tick with
// nothing dirty and nothing due touches no series at all.
func TestIdleTickPrunesAndRetriesWithoutABackup(t *testing.T) {
	f := newEngineFixture(t)
	id := f.container(t, "nextcloud")
	f.steadySeries(t, id, "backup", 12, 40*gib)
	f.run(t, id, "backup", f.now-60, 20*mib)
	f.pass(t)
	row := onlyRow(t, f.openRows(t))
	if _, err := f.st.AcknowledgeAnomalies([]string{row.ID}, "planned", f.now); err != nil {
		t.Fatal(err)
	}
	// An episode the user settled while its condition was present lives as long
	// as the condition; only a cleared one ages out.
	if _, err := f.db.Exec(`UPDATE anomalies SET cleared_at = ?, last_seen_at = ?, acked_at = ? WHERE id = ?`,
		f.now-200*86400, f.now-200*86400, f.now-200*86400, row.ID); err != nil {
		t.Fatal(err)
	}

	f.now += anomalyPruneEvery
	var evaluated []anomalyScope
	f.e.beforeWrite = func(sc anomalyScope) { evaluated = append(evaluated, sc) }
	f.pass(t)

	if len(evaluated) != 0 {
		t.Fatalf("an idle tick must judge no series, judged %v", evaluated)
	}
	var left int
	if err := f.db.QueryRow(`SELECT count(*) FROM anomalies`).Scan(&left); err != nil {
		t.Fatal(err)
	}
	if left != 0 {
		t.Fatalf("the closed finding should be pruned, %d left", left)
	}

	f.now += 60
	generation := f.e.summary().Generation
	f.pass(t)
	if len(evaluated) != 0 {
		t.Fatalf("a tick with nothing due must do nothing, judged %v", evaluated)
	}
	if now := f.e.summary().Generation; now != generation {
		t.Fatalf("generation moved to %d without anything happening", now)
	}
}

// A night of backups arrives as a burst of finished runs; one pass judges each
// series once.
func TestEngineCoalescesBurst(t *testing.T) {
	f := newEngineFixture(t)
	ids := []string{f.container(t, "a"), f.container(t, "b"), f.container(t, "c")}
	for _, id := range ids {
		f.steadySeries(t, id, "backup", 12, 40*gib)
	}
	var evaluated []anomalyScope
	f.e.beforeWrite = func(sc anomalyScope) { evaluated = append(evaluated, sc) }
	for i := range 50 {
		f.run(t, ids[i%len(ids)], "backup", f.now-int64(50-i), 40*gib)
	}

	f.pass(t)

	seen := map[anomalyScope]int{}
	for _, sc := range evaluated {
		seen[sc]++
	}
	for _, id := range ids {
		sc := anomalyScope{Kind: anomalyScopeItem, ID: id}
		if seen[sc] != 1 {
			t.Fatalf("%s judged %d times, want once", sc, seen[sc])
		}
	}
	if len(seen) != len(ids) {
		t.Fatalf("judged %d scopes, want %d: %v", len(seen), len(ids), seen)
	}
}

// A pass that cannot resolve its run ids keeps them for the next one.
func TestUnresolvedRunsStayDirty(t *testing.T) {
	f := newEngineFixture(t)
	id := f.container(t, "nextcloud")
	f.steadySeries(t, id, "backup", 12, 40*gib)
	f.run(t, id, "backup", f.now-60, 20*mib)

	f.e.runTargets = func([]string) (map[string]store.RunTargetKind, error) {
		return nil, errors.New("database is locked")
	}
	if err := f.e.passOnce(context.Background()); err != nil {
		t.Fatalf("a failed id lookup must not fail the pass: %v", err)
	}
	if rows := f.openRows(t); len(rows) != 0 {
		t.Fatalf("want no findings, got %+v", rows)
	}
	if got := f.e.summary().EvalErrors; got != 1 {
		t.Fatalf("evalErrors = %d, want 1", got)
	}

	f.e.runTargets = f.st.RunTargets
	f.pass(t)
	if row := onlyRow(t, f.openRows(t)); row.Metric != metricSourceBytesShrink {
		t.Fatalf("the retried run must be judged, got %s", row.Metric)
	}
}

// Detection off stops evaluation entirely; switching it on again judges
// everything that happened while it was off.
func TestEngineDisabledDoesNothing(t *testing.T) {
	f := newEngineFixture(t)
	id := f.container(t, "nextcloud")
	f.steadySeries(t, id, "backup", 12, 40*gib)
	f.run(t, id, "backup", f.now-60, 20*mib)
	if _, err := f.st.MutateSettings(func(s *store.Settings) error {
		s.AnomalyEnabled = false
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	f.pass(t)

	if rows := f.openRows(t); len(rows) != 0 {
		t.Fatalf("detection is off, got %+v", rows)
	}
	if sum := f.e.summary(); sum.Enabled {
		t.Fatal("summary should report detection as off")
	}
	held, why, err := f.e.RetentionHeld(context.Background(), anomalyScope{Kind: anomalyScopeItem, ID: id})
	if held || why != "" || err != nil {
		t.Fatalf("RetentionHeld with detection off = %v %q %v", held, why, err)
	}

	if _, err := f.st.MutateSettings(func(s *store.Settings) error {
		s.AnomalyEnabled = true
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	f.e.MarkAllDirty()
	f.pass(t)
	if row := onlyRow(t, f.openRows(t)); row.Metric != metricSourceBytesShrink {
		t.Fatalf("want the collapse after switching on, got %s", row.Metric)
	}
}

// One item that panics must not take the pass with it.
func TestEnginePanicInOneItemIsCountedNotFatal(t *testing.T) {
	f := newEngineFixture(t)
	bad := f.container(t, "aaa-breaks")
	good := f.container(t, "zzz-works")
	for _, id := range []string{bad, good} {
		f.steadySeries(t, id, "backup", 12, 40*gib)
		f.run(t, id, "backup", f.now-60, 20*mib)
	}
	f.e.beforeWrite = func(sc anomalyScope) {
		if sc.ID == bad {
			panic("detector fell over")
		}
	}

	f.pass(t)

	row := onlyRow(t, f.openRows(t))
	if row.TargetID != good {
		t.Fatalf("the healthy item was not judged: %+v", row)
	}
	if got := f.e.summary().EvalErrors; got != 1 {
		t.Fatalf("evalErrors = %d, want 1", got)
	}
}

// The store runs on one connection, so a pass that started another query while
// a result set was still open would never finish.
func TestEngineSingleConnectionNoDeadlock(t *testing.T) {
	f := newEngineFixture(t)
	for i := range 20 {
		id := f.container(t, string(rune('a'+i)))
		f.steadySeries(t, id, "backup", 90, 40*gib)
	}
	f.e.MarkAllDirty()

	done := make(chan error, 1)
	go func() { done <- f.e.passOnce(context.Background()) }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("pass: %v", err)
		}
	case <-time.After(waitLimit):
		t.Fatal("the pass never finished: a store call inside an open result set")
	}
}

// FinishRun calls the engine's hook on the goroutine that holds the database
// connection, so a pass that held its own mutex across a store call would
// deadlock against every backup that finishes during it.
func TestHookDuringPassDoesNotDeadlock(t *testing.T) {
	f := newEngineFixture(t)
	for i := range 5 {
		id := f.container(t, string(rune('a'+i)))
		f.steadySeries(t, id, "backup", 30, 40*gib)
	}
	busy := f.container(t, "busy")

	f.e.MarkAllDirty()
	stop := make(chan struct{})
	var finishing sync.WaitGroup
	finishing.Add(1)
	go func() {
		defer finishing.Done()
		for i := 0; i < 100; i++ {
			select {
			case <-stop:
				return
			default:
			}
			id, err := f.st.StartRun(busy, "backup")
			if err != nil {
				return
			}
			_ = f.st.FinishRun(id, "success", "snap", 1, "")
		}
	}()

	done := make(chan error, 1)
	go func() { done <- f.e.passOnce(context.Background()) }()
	select {
	case err := <-done:
		close(stop)
		finishing.Wait()
		if err != nil {
			t.Fatalf("pass: %v", err)
		}
	case <-time.After(waitLimit):
		close(stop)
		t.Fatal("the pass deadlocked against the run-finished hook")
	}
}

// A pass and a user's acknowledge take the same per-scope lock, so the
// acknowledge never lands between the pass's read and its write. It holds for a
// scope with no target id too.
func TestAcknowledgeDuringPassKeepsAcknowledged(t *testing.T) {
	for _, tc := range []struct {
		name  string
		build func(*engineFixture, *testing.T) anomalyScope
	}{
		{"item", func(f *engineFixture, t *testing.T) anomalyScope {
			id := f.container(t, "nextcloud")
			f.steadySeries(t, id, "backup", 12, 40*gib)
			f.run(t, id, "backup", f.now-60, 20*mib)
			return anomalyScope{Kind: anomalyScopeItem, ID: id}
		}},
		{"domain", func(f *engineFixture, t *testing.T) anomalyScope {
			for i, ok := range []bool{true, false} {
				err := f.st.AddRestoreDrill(store.RestoreDrill{
					Domain: "containers", Source: "containers", Kind: "subset",
					At: f.now - int64(3-i)*86400, OK: ok,
				})
				if err != nil {
					t.Fatal(err)
				}
			}
			f.run(t, "containers", "drill", f.now-600, 0)
			return anomalyScope{Kind: anomalyScopeDomain, ID: "containers:containers"}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newEngineFixture(t)
			scope := tc.build(f, t)
			f.pass(t)
			row := onlyRow(t, f.openRows(t))

			f.e.MarkAllDirty()
			paused := make(chan struct{})
			acked := make(chan error, 1)
			f.e.beforeWrite = func(sc anomalyScope) {
				if sc != scope {
					return
				}
				f.e.beforeWrite = nil
				close(paused)
				// The acknowledge must wait for this scope's lock, which the
				// pass is holding.
				select {
				case <-acked:
					t.Error("the acknowledge did not wait for the pass")
				case <-time.After(50 * time.Millisecond):
				}
			}
			go func() {
				<-paused
				_, _, err := f.svc.AcknowledgeAnomalies(context.Background(), []string{row.ID}, "seen it")
				acked <- err
			}()

			f.pass(t)
			select {
			case err := <-acked:
				if err != nil {
					t.Fatalf("acknowledge: %v", err)
				}
			case <-time.After(waitLimit):
				t.Fatal("the acknowledge never got the scope lock")
			}

			after, found, err := f.st.GetAnomaly(row.ID)
			if err != nil || !found {
				t.Fatalf("read back: %v %v", err, found)
			}
			if after.State != "acknowledged" || after.AckNote != "seen it" {
				t.Fatalf("state = %s note = %q, want the acknowledge to stand", after.State, after.AckNote)
			}
		})
	}
}

// Most of this package's tests build a bare &Service{} with no engine, so every
// entry point has to survive a nil one.
func TestAnomalyEngineNilReceiverIsInert(t *testing.T) {
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close() //nolint:errcheck // closing an in-memory store cannot fail the test
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	s := &Service{store: store.New(db)}

	s.anomalies.MarkAllDirty()
	s.anomalies.runFinished(store.RunFinished{RunID: "r1"})
	s.StartAnomalyEngine(context.Background())
	if err := s.anomalies.passOnce(context.Background()); err != nil {
		t.Fatalf("passOnce on a nil engine: %v", err)
	}
	held, why, err := s.anomalies.RetentionHeld(context.Background(), anomalyScope{Kind: anomalyScopeItem, ID: "x"})
	if held || why != "" || err != nil {
		t.Fatalf("RetentionHeld on a nil engine = %v %q %v", held, why, err)
	}
	if sum := s.AnomalySummary(context.Background()); sum.Ready {
		t.Fatal("a service without an engine is never ready")
	}
	if items, iErr := s.AnomalyItems(context.Background()); iErr != nil || len(items) != 0 {
		t.Fatalf("AnomalyItems on a nil engine = %v %v", items, iErr)
	}
}

// A stricter preset has to reach the series that were already judged under the
// old one.
func TestEngineSettingsChangeMarksEverythingDirty(t *testing.T) {
	f := newEngineFixture(t)
	id := f.container(t, "nextcloud")
	for i := 12; i >= 1; i-- {
		f.run(t, id, "backup", f.now-int64(i)*86400, 40*gib, withMS(10*60*1000))
	}
	// Far enough over the usual ten minutes for strict, well under what balanced
	// asks for.
	f.run(t, id, "backup", f.now-60, 40*gib, withMS(23*60*1000))
	f.pass(t)
	if rows := f.openRows(t); len(rows) != 0 {
		t.Fatalf("balanced raises nothing here, got %+v", rows)
	}

	if _, err := f.st.MutateSettings(func(s *store.Settings) error {
		s.AnomalySensitivity = "strict"
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	f.e.MarkAllDirty()
	f.pass(t)

	if row := onlyRow(t, f.openRows(t)); row.Metric != metricDurationSlower {
		t.Fatalf("want the slower run under strict, got %s", row.Metric)
	}
}

// The run-finished hook takes the engine's mutex while the goroutine that
// finished the run holds the database connection. A pass that held that mutex
// across a store call would stop both of them for good.
func TestEngineNeverHoldsItsMutexAcrossTheStore(t *testing.T) {
	f := newEngineFixture(t)
	id := f.container(t, "nextcloud")
	f.steadySeries(t, id, "backup", 12, 40*gib)
	f.run(t, id, "backup", f.now-60, 20*mib)
	f.e.beforeWrite = func(anomalyScope) {
		f.e.runFinished(store.RunFinished{RunID: "another-run"})
	}

	done := make(chan error, 1)
	go func() { done <- f.e.passOnce(context.Background()) }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("pass: %v", err)
		}
	case <-time.After(waitLimit):
		t.Fatal("the pass holds its mutex while it works on a scope")
	}
}
