package api

import (
	"context"
	"fmt"
	"log"
	"maps"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

// The worker behind anomaly detection. A finished run marks its series dirty
// and wakes it; one pass judges every dirty series, writes what changed and
// rebuilds the small figure set the SPA polls.
//
// Three rules hold the whole thing together, because the store runs on one
// connection and FinishRun calls back into the engine from the goroutine that
// holds it:
//
//   - e.mu is never held across a store call.
//   - a scope's rows are read, decided and written before the next scope starts,
//     with every result set drained.
//   - a scope lock is taken before e.mu is needed, never while it is held.

const (
	anomalyScopeItem   = "item"
	anomalyScopeDump   = "dump"
	anomalyScopeZFSDS  = "zfsds"
	anomalyScopeDomain = "domain"
	anomalyScopeVolume = "volume"
)

const (
	// anomalyDebounce lets a night of backups arrive as one pass.
	anomalyDebounce = 20 * time.Second
	// anomalyIdleTick is what the once-a-day housekeeping runs on when no
	// backup happens at all.
	anomalyIdleTick = 15 * time.Minute
	// anomalyStartupDelay keeps the first full pass out of the way of boot.
	anomalyStartupDelay = 60 * time.Second
)

const (
	anomalyPruneEvery     = int64(24 * 3600)
	anomalyKeepClosedDays = 90
	anomalySeriesRuns     = 90
	anomalyNewDataMaxRows = 1000
	anomalyDrillChecks    = 30
	anomalyOpenRowLimit   = 500
)

var anomalyPresets = []Sensitivity{sensStrict, sensBalanced, sensPermissive}

// anomalyNotifyLevels is the closed set a notification minimum comes from, from
// quietest to loudest plus the off switch.
var anomalyNotifyLevels = []string{"info", "warning", "critical", "off"}

// anomalyScope names one evaluated series: an item's backups, a container's
// database dumps, one dataset of a ZFS tree, or one series of restore checks.
// Its string form is the key both the pass and a user's action lock on, so a
// scope without a target id is locked like an item's rows.
type anomalyScope struct{ Kind, ID string }

func (sc anomalyScope) String() string { return sc.Kind + ":" + sc.ID }

// anomalyItemRef is one backed-up item as a pass sees it.
type anomalyItemRef struct {
	TargetID, Domain, Name string
	Scheduled              bool
}

// anomalyScopeResult is what the last pass learned about one series, kept so
// the Items tab can be served without touching the database.
type anomalyScopeResult struct {
	Learning  learningInfo
	Typical   typicalValues
	Runs      int
	NewestAt  int64
	Selection int64
}

// anomalyCache is the whole read side, replaced in one go at the end of a pass
// and after every user action.
type anomalyCache struct {
	Ready      bool
	Generation int64
	Summary    AnomalySummary
	Items      []AnomalyItem
}

type anomalyEngine struct {
	svc *Service
	now func() time.Time

	mu           sync.Mutex
	dirtyRuns    map[string]struct{}
	dirtyScopes  map[anomalyScope]struct{}
	dirtyDomains map[string]struct{}
	volumeDirty  bool
	fullDirty    bool
	evaluatedTo  map[anomalyScope]int64
	results      map[anomalyScope]anomalyScopeResult
	lastPrune    int64
	evalErrors   int
	ready        bool
	unmeasured   map[string][]string

	signal     chan struct{}
	scopeLocks sync.Map
	cache      atomic.Pointer[anomalyCache]

	debounce     time.Duration
	idleTick     time.Duration
	startupDelay time.Duration

	// items and runTargets are the two store reads a pass cannot continue
	// without. They are fields so a test can make them fail.
	items      func(store.Settings) (map[string]anomalyItemRef, error)
	runTargets func([]string) (map[string]store.RunTargetKind, error)
	// beforeWrite runs inside a scope's lock, after its reads and before its
	// write. Nothing in production sets it; a test interrupts a pass there.
	beforeWrite func(anomalyScope)
}

func newAnomalyEngine(s *Service, now func() time.Time) *anomalyEngine {
	e := &anomalyEngine{
		svc: s, now: now,
		dirtyRuns:    map[string]struct{}{},
		dirtyScopes:  map[anomalyScope]struct{}{},
		dirtyDomains: map[string]struct{}{},
		evaluatedTo:  map[anomalyScope]int64{},
		results:      map[anomalyScope]anomalyScopeResult{},
		unmeasured:   map[string][]string{},
		signal:       make(chan struct{}, 1),
		debounce:     anomalyDebounce,
		idleTick:     anomalyIdleTick,
		startupDelay: anomalyStartupDelay,
		runTargets:   s.store.RunTargets,
	}
	e.items = func(settings store.Settings) (map[string]anomalyItemRef, error) {
		return s.readAnomalyItems(settings)
	}
	return e
}

// Start fills the cache from the table, runs the worker until ctx is cancelled
// and schedules the first full pass.
func (e *anomalyEngine) Start(ctx context.Context) {
	if e == nil {
		return
	}
	e.svc.store.SetRunFinishedHook(e.runFinished)
	if err := e.rebuildCache(); err != nil {
		log.Printf("anomaly: read the findings at startup: %v", err)
	}
	go e.run(ctx)
	go func() {
		select {
		case <-ctx.Done():
			return
		case <-time.After(e.startupDelay):
		}
		if err := e.backfillRunMetrics(ctx); err != nil {
			log.Printf("anomaly: read the repositories' own history: %v", err)
		}
		e.MarkAllDirty()
	}()
}

func (e *anomalyEngine) run(ctx context.Context) {
	tick := time.NewTicker(e.idleTick)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-e.signal:
			if !e.settle(ctx) {
				return
			}
		case <-tick.C:
		}
		if err := e.passOnce(ctx); err != nil {
			log.Printf("anomaly: pass: %v", err)
		}
	}
}

// settle waits out the debounce and swallows the signals that arrived during
// it, so a whole night of backups becomes one pass.
func (e *anomalyEngine) settle(ctx context.Context) bool {
	if e.debounce <= 0 {
		return true
	}
	select {
	case <-ctx.Done():
		return false
	case <-time.After(e.debounce):
	}
	select {
	case <-e.signal:
	default:
	}
	return true
}

// runFinished is the store's hook. It only marks dirty: it runs on the
// goroutine that finished the run, which holds the database connection.
func (e *anomalyEngine) runFinished(ev store.RunFinished) {
	if e == nil {
		return
	}
	e.mu.Lock()
	switch {
	case ev.RunID != "":
		e.dirtyRuns[ev.RunID] = struct{}{}
	case ev.TargetID != "":
		// A run failed from under its own goroutine, so its kind is gone with
		// it; both of an item's series are cheap to judge again.
		e.dirtyScopes[anomalyScope{Kind: anomalyScopeItem, ID: ev.TargetID}] = struct{}{}
		e.dirtyScopes[anomalyScope{Kind: anomalyScopeDump, ID: ev.TargetID}] = struct{}{}
	}
	e.mu.Unlock()
	e.wake()
}

// MarkAllDirty puts every series and every domain up for the next pass.
func (e *anomalyEngine) MarkAllDirty() {
	if e == nil {
		return
	}
	e.mu.Lock()
	e.fullDirty = true
	e.mu.Unlock()
	e.wake()
}

// MarkVolumeDirty puts the free-space rules up for the next pass, which is
// what a fresh reading asks for.
func (e *anomalyEngine) MarkVolumeDirty() {
	if e == nil {
		return
	}
	e.mu.Lock()
	e.volumeDirty = true
	e.mu.Unlock()
	e.wake()
}

// noteUnmeasuredVolumes records the repositories of one domain whose backend
// answers no capacity question, so the page can name them instead of leaving
// them out of the picture.
func (e *anomalyEngine) noteUnmeasuredVolumes(domain string, names []string) {
	if e == nil {
		return
	}
	e.mu.Lock()
	if len(names) == 0 {
		delete(e.unmeasured, domain)
	} else {
		e.unmeasured[domain] = names
	}
	e.mu.Unlock()
}

func (e *anomalyEngine) markScopesDirty(scopes []anomalyScope) {
	if e == nil || len(scopes) == 0 {
		return
	}
	e.mu.Lock()
	for _, sc := range scopes {
		if sc.Kind == anomalyScopeDomain {
			e.dirtyDomains[domainOfDrillScope(sc)] = struct{}{}
			continue
		}
		e.dirtyScopes[sc] = struct{}{}
	}
	e.mu.Unlock()
	e.wake()
}

func (e *anomalyEngine) wake() {
	select {
	case e.signal <- struct{}{}:
	default:
	}
}

// nowUnix is the engine's clock, so a service built without an engine still
// answers.
func (e *anomalyEngine) nowUnix() int64 {
	if e == nil {
		return time.Now().Unix()
	}
	return e.now().Unix()
}

// lockScopes takes the locks of every scope a user action touches, in key
// order, so two actions on overlapping scopes can never deadlock.
func (e *anomalyEngine) lockScopes(scopes []anomalyScope) func() {
	if e == nil || len(scopes) == 0 {
		return func() {}
	}
	keys := make([]string, 0, len(scopes))
	for _, sc := range scopes {
		keys = append(keys, sc.String())
	}
	slices.Sort(keys)
	keys = slices.Compact(keys)

	locks := make([]*sync.Mutex, 0, len(keys))
	for _, key := range keys {
		mu := e.lockOf(key)
		mu.Lock()
		locks = append(locks, mu)
	}
	return func() {
		for i := len(locks) - 1; i >= 0; i-- {
			locks[i].Unlock()
		}
	}
}

func (e *anomalyEngine) lockOf(key string) *sync.Mutex {
	mu, _ := e.scopeLocks.LoadOrStore(key, &sync.Mutex{})
	return mu.(*sync.Mutex)
}

// refresh rebuilds the read side after a user action, so the page shows what
// the user just did without waiting for a pass.
func (e *anomalyEngine) refresh() {
	if e == nil {
		return
	}
	if err := e.rebuildCache(); err != nil {
		log.Printf("anomaly: rebuild the summary: %v", err)
	}
}

func (e *anomalyEngine) summary() AnomalySummary {
	if e == nil {
		return AnomalySummary{}
	}
	if c := e.cache.Load(); c != nil {
		return c.Summary
	}
	return AnomalySummary{}
}

func (e *anomalyEngine) itemViews() []AnomalyItem {
	if e == nil {
		return nil
	}
	if c := e.cache.Load(); c != nil {
		return c.Items
	}
	return nil
}

// anomalyPass is what one pass reads once and every scope of it then shares.
type anomalyPass struct {
	settings store.Settings
	now      int64
	items    map[string]anomalyItemRef
	prefs    map[string]store.ItemPrefs
	errs     int

	drills     []store.DrillKey
	drillsRead bool

	volumes     map[string]*volumeHistory
	volumesRead bool
	growth      map[string]int64
	growthRead  bool
}

// volumeHistory is one volume's readings and the domains that keep a
// repository on it.
type volumeHistory struct {
	samples []store.VolumeSample
	domains []string
}

// drillKeys lists the restore-check series of this installation, once per pass.
func (e *anomalyEngine) drillKeys(p *anomalyPass) ([]store.DrillKey, error) {
	if p.drillsRead {
		return p.drills, nil
	}
	keys, err := e.svc.store.ListRestoreDrillKeys()
	if err != nil {
		return nil, err
	}
	p.drills, p.drillsRead = keys, true
	return keys, nil
}

// passOnce judges everything that is dirty and leaves nothing behind: a scope
// whose evaluation failed goes back into the dirty set for the next pass.
func (e *anomalyEngine) passOnce(ctx context.Context) error {
	if e == nil {
		return nil
	}
	now := e.now().Unix()
	dirty, full := e.takeDirty()
	pruneDue := e.pruneDue(now)
	if !full && !pruneDue && dirty.empty() {
		return nil
	}

	settings, err := e.svc.store.GetSettings()
	if err != nil {
		e.restoreDirty(dirty, full)
		return err
	}
	if pruneDue {
		e.prune(now)
		if bErr := e.backfillRunMetrics(ctx); bErr != nil {
			log.Printf("anomaly: read the repositories' own history: %v", bErr)
		}
	}
	if !settings.AnomalyEnabled {
		e.setEvalErrors(0)
		return e.rebuildCache()
	}

	p := &anomalyPass{settings: settings, now: now}
	if p.items, err = e.items(settings); err != nil {
		e.restoreDirty(dirty, full)
		e.setEvalErrors(1)
		e.refresh()
		return err
	}
	if p.prefs, err = e.svc.store.ListItemPrefs(); err != nil {
		e.restoreDirty(dirty, full)
		e.setEvalErrors(1)
		e.refresh()
		return err
	}

	for _, sc := range e.scopesToEvaluate(dirty, full, p) {
		if ctx.Err() != nil {
			break
		}
		if evErr := e.evaluateGuarded(ctx, sc, p); evErr != nil {
			log.Printf("anomaly: %s: %v", sc, evErr)
			p.errs++
			e.markScopesDirty([]anomalyScope{sc})
		}
	}

	e.setEvalErrors(p.errs)
	if full {
		e.markReady()
	}
	if nErr := e.sendNotifications(ctx, settings); nErr != nil {
		log.Printf("anomaly: send the findings of this pass: %v", nErr)
	}
	return e.rebuildCache()
}

// scopesToEvaluate turns the dirty sets into the series of this pass, in a
// stable order.
func (e *anomalyEngine) scopesToEvaluate(dirty dirtySets, full bool, p *anomalyPass) []anomalyScope {
	runs := dirty.runs
	want := maps.Clone(dirty.scopes)
	dirtyDomains := maps.Clone(dirty.domains)
	if dirty.volumes || full {
		for _, sc := range e.volumeScopes(p) {
			want[sc] = struct{}{}
		}
	}
	if full {
		for id, ref := range p.items {
			want[anomalyScope{Kind: anomalyScopeItem, ID: id}] = struct{}{}
			if ref.Domain == anomalyDomainContainer {
				want[anomalyScope{Kind: anomalyScopeDump, ID: id}] = struct{}{}
			}
		}
		keys, err := e.drillKeys(p)
		if err != nil {
			log.Printf("anomaly: list the restore checks: %v", err)
			p.errs++
		}
		for _, k := range keys {
			dirtyDomains[k.Domain] = struct{}{}
		}
	}

	if len(runs) > 0 {
		ids := slices.Sorted(maps.Keys(runs))
		found, err := e.runTargets(ids)
		if err != nil {
			log.Printf("anomaly: resolve %d finished run(s): %v", len(ids), err)
			p.errs++
			e.restoreRuns(runs)
		}
		for _, tk := range found {
			switch tk.Kind {
			case "backup":
				if _, known := p.items[tk.TargetID]; known {
					want[anomalyScope{Kind: anomalyScopeItem, ID: tk.TargetID}] = struct{}{}
				}
			case "dbdump":
				if _, known := p.items[tk.TargetID]; known {
					want[anomalyScope{Kind: anomalyScopeDump, ID: tk.TargetID}] = struct{}{}
				}
			case "drill", "drdrill":
				if domain := anomalyRunDomain(tk.TargetID); domain != "" {
					dirtyDomains[domain] = struct{}{}
				}
			}
		}
	}

	if len(dirtyDomains) > 0 {
		keys, err := e.drillKeys(p)
		if err != nil {
			log.Printf("anomaly: list the restore checks: %v", err)
			p.errs++
		}
		for _, k := range keys {
			if _, dirty := dirtyDomains[k.Domain]; dirty {
				want[drillScope(k)] = struct{}{}
			}
		}
	}

	out := slices.Collect(maps.Keys(want))
	slices.SortFunc(out, func(a, b anomalyScope) int { return strings.Compare(a.String(), b.String()) })
	return out
}

// evaluateGuarded keeps one broken series from taking the pass with it.
func (e *anomalyEngine) evaluateGuarded(ctx context.Context, sc anomalyScope, p *anomalyPass) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("evaluation panicked: %v", r)
		}
	}()
	return e.evaluateScope(ctx, sc, p)
}

func (e *anomalyEngine) evaluateScope(ctx context.Context, sc anomalyScope, p *anomalyPass) error {
	release := e.lockScopes([]anomalyScope{sc})
	defer release()

	switch sc.Kind {
	case anomalyScopeItem, anomalyScopeDump:
		return e.evaluateSeries(ctx, sc, p)
	case anomalyScopeDomain:
		return e.evaluateDrillSeries(ctx, sc, p)
	case anomalyScopeVolume:
		return e.evaluateVolume(ctx, sc, p)
	}
	return nil
}

// volumeScopes are the volumes this installation has readings for.
func (e *anomalyEngine) volumeScopes(p *anomalyPass) []anomalyScope {
	histories, err := e.volumeHistories(p)
	if err != nil {
		log.Printf("anomaly: read the free-space samples: %v", err)
		p.errs++
		return nil
	}
	out := make([]anomalyScope, 0, len(histories))
	for volume := range histories {
		out = append(out, anomalyScope{Kind: anomalyScopeVolume, ID: volume})
	}
	return out
}

// volumeHistories groups this pass's readings by volume, once.
func (e *anomalyEngine) volumeHistories(p *anomalyPass) (map[string]*volumeHistory, error) {
	if p.volumesRead {
		return p.volumes, nil
	}
	samples, err := e.svc.store.ListVolumeSamples(p.now - capacityWindowDays*86400)
	if err != nil {
		return nil, err
	}
	out := map[string]*volumeHistory{}
	for _, sample := range samples {
		history := out[sample.Volume]
		if history == nil {
			history = &volumeHistory{}
			out[sample.Volume] = history
		}
		history.samples = append(history.samples, sample)
		for _, domain := range sample.Domains {
			if !slices.Contains(history.domains, domain) {
				history.domains = append(history.domains, domain)
			}
		}
	}
	p.volumes, p.volumesRead = out, true
	return out, nil
}

// repoGrowth is how fast each domain's own repository grows, from the same
// measurement the storage forecast shows, so the card and the finding cannot
// give different answers.
func (e *anomalyEngine) repoGrowth(p *anomalyPass) map[string]int64 {
	if p.growthRead {
		return p.growth
	}
	out := map[string]int64{}
	for _, domain := range enabledDomains(p.settings) {
		stats, err := e.svc.store.ListRepoStats(domain, "local", 0)
		if err != nil {
			log.Printf("anomaly: read the size samples of %s: %v", domain, err) //nolint:gosec // G706: domain is a fixed literal
			continue
		}
		if week, ok := growthBytesPerWeek(stats, time.Unix(p.now, 0)); ok {
			out[domain] = week
		}
	}
	p.growth, p.growthRead = out, true
	return out
}

// evaluateVolume judges how much room one volume has left. It is a scope like
// any other, so an acknowledge of a capacity finding takes the same lock a pass
// does.
func (e *anomalyEngine) evaluateVolume(ctx context.Context, sc anomalyScope, p *anomalyPass) error {
	histories, err := e.volumeHistories(p)
	if err != nil {
		return err
	}
	history := histories[sc.ID]
	if history == nil {
		return nil
	}
	state, err := e.svc.store.AnomalyScopeState(sc.Kind, sc.ID)
	if err != nil {
		return err
	}
	growth := e.repoGrowth(p)
	sens := resolveSensitivity("", p.settings.AnomalySensitivity)
	if e.beforeWrite != nil {
		e.beforeWrite(sc)
	}
	found, absent := detectCapacity(volumeInput{
		Volume: sc.ID, Domains: history.domains, Samples: history.samples,
		Growth: growth, Open: openByMetric(state.Open), Sens: sens, Now: p.now,
	})
	changes := applyFindings(scopeRef{Kind: sc.Kind, ID: sc.ID, Sensitivity: string(sens)},
		found, absent, state, p.now)
	if err := e.apply(sc, changes); err != nil {
		return err
	}
	return ctx.Err()
}

// evaluateSeries judges one item's backups or one container's database dumps.
func (e *anomalyEngine) evaluateSeries(ctx context.Context, sc anomalyScope, p *anomalyPass) error {
	ref, known := p.items[sc.ID]
	if !known {
		return nil
	}
	kind, series := "backup", seriesItem
	if sc.Kind == anomalyScopeDump {
		kind, series = "dbdump", seriesDump
	}
	from := e.evaluatedFrom(sc, p.now)

	rows, err := e.svc.store.ItemSeries(sc.ID, kind, p.now, anomalySeriesRuns)
	if err != nil {
		return err
	}
	window, err := e.svc.store.NewDataWindow(sc.ID, kind,
		from-anomalyNewDataDays*86400, p.now, anomalyNewDataMaxRows)
	if err != nil {
		return err
	}
	state, err := e.svc.store.AnomalyScopeState(sc.Kind, sc.ID)
	if err != nil {
		return err
	}
	expectations, err := e.svc.store.ListAnomalyExpectations(sc.Kind, sc.ID)
	if err != nil {
		return err
	}

	sens := resolveSensitivity(p.prefs[ref.TargetID].Sensitivity, p.settings.AnomalySensitivity)
	if e.beforeWrite != nil {
		e.beforeWrite(sc)
	}
	res := evaluateItem(itemInput{
		Kind: series, Domain: ref.Domain, Series: rows, NewData: window,
		Open: openByMetric(state.Open), Expectations: byFamily(expectations),
		Sens: sens, EvaluateFrom: from,
	})
	changes := applyFindings(scopeRef{
		Kind: sc.Kind, ID: sc.ID, TargetID: ref.TargetID, Domain: ref.Domain,
		Sensitivity: string(sens),
	}, res.Findings, res.Absent, state, p.now)

	if err := e.apply(sc, changes); err != nil {
		return err
	}
	e.recordScope(sc, anomalyScopeResult{
		Learning: res.Learning, Typical: res.Typical, Runs: len(rows),
		NewestAt: newestEligibleAt(rows), Selection: selectionRebaseAt(rows),
	})
	return ctx.Err()
}

// evaluateDrillSeries judges the restore checks of one domain and source. Both
// flavours of check share the scope, because a fingerprint carries the metric.
func (e *anomalyEngine) evaluateDrillSeries(ctx context.Context, sc anomalyScope, p *anomalyPass) error {
	keys, err := e.drillKeys(p)
	if err != nil {
		return err
	}
	var found []finding
	var absent []absence
	for _, k := range keys {
		if drillScope(k) != sc {
			continue
		}
		checks, cErr := e.svc.store.ListRestoreDrillsKind(k.Domain, k.Source, k.TargetID, k.Kind, anomalyDrillChecks)
		if cErr != nil {
			return cErr
		}
		keyFound, keyAbsent := detectIntegrity(k, checks)
		found = append(found, keyFound...)
		absent = append(absent, keyAbsent...)
	}

	state, err := e.svc.store.AnomalyScopeState(sc.Kind, sc.ID)
	if err != nil {
		return err
	}
	if e.beforeWrite != nil {
		e.beforeWrite(sc)
	}
	changes := applyFindings(scopeRef{
		Kind: sc.Kind, ID: sc.ID, Domain: domainOfDrillScope(sc),
		Sensitivity: string(resolveSensitivity("", p.settings.AnomalySensitivity)),
	}, found, absent, state, p.now)
	if err := e.apply(sc, changes); err != nil {
		return err
	}
	return ctx.Err()
}

func (e *anomalyEngine) apply(sc anomalyScope, changes store.AnomalyChanges) error {
	applied, err := e.svc.store.ApplyAnomalyChanges(changes)
	if err != nil {
		return err
	}
	if len(applied.Stale) > 0 {
		log.Printf("anomaly: %s: %d finding(s) were changed while the pass ran: %v",
			sc, len(applied.Stale), applied.Stale)
	}
	return nil
}

// EvaluateNow judges one series between passes, which is what the retention
// hold asks for before it deletes anything.
func (e *anomalyEngine) EvaluateNow(ctx context.Context, sc anomalyScope) error {
	if e == nil {
		return nil
	}
	settings, err := e.svc.store.GetSettings()
	if err != nil {
		return err
	}
	if !settings.AnomalyEnabled {
		return nil
	}
	p := &anomalyPass{settings: settings, now: e.now().Unix()}
	if p.items, err = e.items(settings); err != nil {
		return err
	}
	if p.prefs, err = e.svc.store.ListItemPrefs(); err != nil {
		return err
	}
	if err := e.evaluateGuarded(ctx, sc, p); err != nil {
		return err
	}
	return e.rebuildCache()
}

// RetentionHeld reports whether deleting old backups of this series has to
// wait, and why. It judges the series first, so a rewrite found in the running
// backup is already on record when that backup's own retention step asks.
func (e *anomalyEngine) RetentionHeld(ctx context.Context, sc anomalyScope) (bool, string, error) {
	if e == nil || sc.Kind == "" {
		return false, "", nil
	}
	settings, err := e.svc.store.GetSettings()
	if err != nil {
		e.noteEvalError()
		return false, "", err
	}
	if !settings.AnomalyEnabled || !settings.AnomalyRetentionHold {
		return false, "", nil
	}
	if err := e.EvaluateNow(ctx, sc); err != nil {
		e.noteEvalError()
		return false, "", err
	}
	rows, _, err := e.svc.store.ListAnomalies(store.AnomalyFilter{
		ScopeKind: sc.Kind, ScopeID: sc.ID, Limit: anomalyOpenRowLimit,
	})
	if err != nil {
		return false, "", err
	}
	for _, row := range rows {
		if !anomalyHolds(row) {
			continue
		}
		if row.Metric == metricNewDataRewrite {
			return true, "most of its data was rewritten", nil
		}
		return true, "its source shrank sharply", nil
	}
	return false, "", nil
}

// HeldIdentityTags is every identity tag whose old backups a finding is
// keeping, across every domain. A pass that forgets by tag without knowing
// which domain a repository holds asks this before it deletes anything.
func (e *anomalyEngine) HeldIdentityTags() (heldIdentityTags, error) {
	if e == nil {
		return nil, nil
	}
	settings, err := e.svc.store.GetSettings()
	if err != nil {
		return nil, err
	}
	if !settings.AnomalyEnabled || !settings.AnomalyRetentionHold {
		return nil, nil
	}
	rows, _, err := e.svc.store.ListAnomalies(store.AnomalyFilter{Limit: anomalyOpenRowLimit})
	if err != nil {
		return nil, err
	}
	items, err := e.svc.readAnomalyItems(settings)
	if err != nil {
		return nil, err
	}
	out := heldIdentityTags{}
	for _, row := range rows {
		if !anomalyHolds(row) {
			continue
		}
		for _, tag := range e.svc.heldTagsOf(row, items) {
			out[tag] = struct{}{}
		}
	}
	return out, nil
}

// heldIdentityTags is the set of tags a retention pass has to leave alone.
type heldIdentityTags map[string]struct{}

// holds reports whether tag belongs to a held series. A VM's block disks carry
// a tag of their own and are kept with the VM they belong to.
func (h heldIdentityTags) holds(tag string) bool {
	if _, exact := h[tag]; exact {
		return true
	}
	for candidate := range h {
		if strings.HasPrefix(tag, candidate+":zvol:") {
			return true
		}
	}
	return false
}

func (h heldIdentityTags) holdsAny(tags []string) bool {
	return slices.ContainsFunc(tags, h.holds)
}

func (h heldIdentityTags) names() []string {
	return slices.Sorted(maps.Keys(h))
}

// heldTagsOf is every tag the snapshots of one held series can carry: the
// series' own tag and, after a rename, the old names that are still on the
// snapshots already written.
func (s *Service) heldTagsOf(row store.Anomaly, items map[string]anomalyItemRef) []string {
	ref, known := items[row.TargetID]
	if !known {
		return nil
	}
	if row.ScopeKind == anomalyScopeDump {
		return s.containerDumpIdentity(ref.Name).listTags()
	}
	if row.ScopeKind != anomalyScopeItem {
		return nil
	}
	switch ref.Domain {
	case anomalyDomainContainer:
		return s.containerIdentity(ref.Name).listTags()
	case anomalyDomainVM:
		return s.vmIdentity(ref.Name).listTags()
	case "files":
		return []string{"fileset:" + ref.Name}
	case "flash", "config":
		return []string{ref.Domain}
	}
	return nil
}

// errRetentionPaused is what the repository-wide pass returns instead of
// running: it selects by path rather than by item, so it cannot spare the items
// a finding is holding.
func errRetentionPaused(n int) error {
	return fmt.Errorf("retention over the whole repository was skipped: deleting old backups is paused for %d item(s)", n)
}

func (e *anomalyEngine) noteEvalError() {
	e.mu.Lock()
	e.evalErrors++
	e.mu.Unlock()
}

func (e *anomalyEngine) takeDirty() (dirtySets, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	d := dirtySets{runs: e.dirtyRuns, scopes: e.dirtyScopes, domains: e.dirtyDomains, volumes: e.volumeDirty}
	full := e.fullDirty
	e.dirtyRuns = map[string]struct{}{}
	e.dirtyScopes = map[anomalyScope]struct{}{}
	e.dirtyDomains = map[string]struct{}{}
	e.volumeDirty = false
	e.fullDirty = false
	return d, full
}

// dirtySets is what one pass took off the queue, kept together so a pass that
// cannot finish can put all of it back.
type dirtySets struct {
	runs    map[string]struct{}
	scopes  map[anomalyScope]struct{}
	domains map[string]struct{}
	volumes bool
}

func (d dirtySets) empty() bool {
	return len(d.runs)+len(d.scopes)+len(d.domains) == 0 && !d.volumes
}

func (e *anomalyEngine) restoreDirty(d dirtySets, full bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	maps.Copy(e.dirtyRuns, d.runs)
	maps.Copy(e.dirtyScopes, d.scopes)
	maps.Copy(e.dirtyDomains, d.domains)
	e.volumeDirty = e.volumeDirty || d.volumes
	e.fullDirty = e.fullDirty || full
}

func (e *anomalyEngine) restoreRuns(runs map[string]struct{}) {
	e.mu.Lock()
	maps.Copy(e.dirtyRuns, runs)
	e.mu.Unlock()
}

func (e *anomalyEngine) evaluatedFrom(sc anomalyScope, now int64) int64 {
	e.mu.Lock()
	defer e.mu.Unlock()
	if at, seen := e.evaluatedTo[sc]; seen {
		return at
	}
	return now - anomalyEventOpenDays*86400
}

func (e *anomalyEngine) recordScope(sc anomalyScope, res anomalyScopeResult) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.results[sc] = res
	if res.NewestAt > e.evaluatedTo[sc] {
		e.evaluatedTo[sc] = res.NewestAt
	}
}

func (e *anomalyEngine) setEvalErrors(n int) {
	e.mu.Lock()
	e.evalErrors = n
	e.mu.Unlock()
}

func (e *anomalyEngine) markReady() {
	e.mu.Lock()
	e.ready = true
	e.mu.Unlock()
}

func (e *anomalyEngine) pruneDue(now int64) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return now-e.lastPrune >= anomalyPruneEvery
}

// prune drops the closed findings nobody needs any more. It runs whether
// detection is on or off, so switching it off does not freeze the history.
func (e *anomalyEngine) prune(now int64) {
	e.mu.Lock()
	e.lastPrune = now
	e.mu.Unlock()
	if _, err := e.svc.store.PruneAnomalies(now - anomalyKeepClosedDays*86400); err != nil {
		log.Printf("anomaly: prune the closed findings: %v", err)
	}
}

// rebuildCache replaces the whole read side in one go, one generation further
// on, so the SPA can tell that something moved with a single number.
func (e *anomalyEngine) rebuildCache() error {
	settings, err := e.svc.store.GetSettings()
	if err != nil {
		return err
	}
	open, _, err := e.svc.store.ListAnomalies(store.AnomalyFilter{Limit: anomalyOpenRowLimit})
	if err != nil {
		return err
	}
	prefs, err := e.svc.store.ListItemPrefs()
	if err != nil {
		return err
	}
	items, err := e.items(settings)
	if err != nil {
		return err
	}
	backfill, err := e.svc.store.ListAnomalyBackfill()
	if err != nil {
		return err
	}

	e.mu.Lock()
	results := maps.Clone(e.results)
	errs, ready := e.evalErrors, e.ready
	unmeasured := unmeasuredNames(e.unmeasured)
	e.mu.Unlock()

	next := &anomalyCache{Ready: ready}
	next.Summary = AnomalySummary{
		Enabled: settings.AnomalyEnabled, Ready: ready,
		EvalErrors: errs, Backfill: backfillSummary(backfill),
		UnmeasuredVolumes: unmeasured,
	}
	for _, row := range open {
		switch row.Severity {
		case "critical":
			next.Summary.Open.Critical++
			if row.RecoveredAt > 0 {
				next.Summary.RecoveredCritical++
			}
		case "warning":
			next.Summary.Open.Warning++
		case "info":
			next.Summary.Open.Info++
		}
		if anomalyHolds(row) {
			next.Summary.RetentionHeld++
		}
	}

	now := e.now().Unix()
	for _, ref := range sortedItemRefs(items) {
		item, listed, iErr := e.itemView(ref, prefs[ref.TargetID], settings, results, open, now)
		if iErr != nil {
			return iErr
		}
		if !listed {
			continue
		}
		if item.Scheduled && !item.Learning.NoData && item.Learning.Samples < item.Learning.Needed {
			next.Summary.LearningItems++
		}
		next.Items = append(next.Items, item)
	}

	if previous := e.cache.Load(); previous != nil {
		next.Generation = previous.Generation + 1
	} else {
		next.Generation = 1
	}
	next.Summary.Generation = next.Generation
	e.cache.Store(next)
	return nil
}

// itemView assembles one row of the Items tab. An item that is neither
// scheduled nor backed up in the last ninety days is not listed at all.
func (e *anomalyEngine) itemView(ref anomalyItemRef, prefs store.ItemPrefs, settings store.Settings,
	results map[anomalyScope]anomalyScopeResult, open []store.Anomaly, now int64) (AnomalyItem, bool, error) {

	itemScope := anomalyScope{Kind: anomalyScopeItem, ID: ref.TargetID}
	res, judged := results[itemScope]
	recent := judged && res.NewestAt > 0 && res.NewestAt >= now-anomalySeriesRuns*86400
	if !ref.Scheduled && !recent {
		return AnomalyItem{}, false, nil
	}

	item := AnomalyItem{
		TargetID: ref.TargetID, Domain: ref.Domain, Name: ref.Name, Scheduled: ref.Scheduled,
		Sensitivity: prefs.Sensitivity, NotifyMin: prefs.NotifyMin,
		Effective:          string(resolveSensitivity(prefs.Sensitivity, settings.AnomalySensitivity)),
		EffectiveNotifyMin: effectiveNotifyMin(prefs.NotifyMin, settings.AnomalyNotifyMin),
		Datasets:           []AnomalySeriesInfo{},
		SelectionSince:     res.Selection,
	}
	if ref.Scheduled {
		item.Learning = AnomalyLearning{
			Samples: min(res.Learning.NewData, res.Learning.Source, res.Learning.Duration),
			Needed:  anomalyMinSamples, NewData: res.Learning.NewData,
			Source: res.Learning.Source, Duration: res.Learning.Duration, NoData: res.Learning.NoData,
		}
		item.Typical = AnomalyTypical{
			SourceBytes: res.Typical.SourceBytes, NewDataBytes: res.Typical.NewDataBytes,
			ResticMS: res.Typical.ResticMS,
		}
	}
	item.Open, item.RetentionHeld = scopeCounts(open, itemScope)

	dumpScope := anomalyScope{Kind: anomalyScopeDump, ID: ref.TargetID}
	if dump, hasDumps := results[dumpScope]; hasDumps && dump.Runs > 0 {
		series := AnomalySeriesInfo{
			Learning: AnomalySeriesLearning{
				Samples: min(dump.Learning.Source, dump.Learning.Duration), Needed: anomalyMinSamples,
			},
			Typical: AnomalySeriesTypical{SourceBytes: dump.Typical.SourceBytes, ResticMS: dump.Typical.ResticMS},
		}
		series.Open, series.RetentionHeld = scopeCounts(open, dumpScope)
		item.Dump = &series
		item.RetentionHeld = item.RetentionHeld || series.RetentionHeld
	}

	expectations, err := e.svc.store.ListAnomalyExpectationsForTarget(ref.TargetID)
	if err != nil {
		return AnomalyItem{}, false, err
	}
	item.Expectations = make([]AnomalyExpectationView, 0, len(expectations))
	for _, x := range expectations {
		view := AnomalyExpectationView{
			Family: x.Family, ScopeKind: x.ScopeKind,
			SinceAt: x.SinceAt, Ceiling: x.Ceiling, UpdatedAt: x.UpdatedAt,
		}
		if x.ScopeKind == anomalyScopeZFSDS {
			view.Part = x.ScopeID
		}
		item.Expectations = append(item.Expectations, view)
	}
	return item, true, nil
}

// readAnomalyItems is the domain map of one pass. It reads the item tables
// itself and reports a failure, because a swallowed list error would silently
// leave a whole domain unwatched.
func (s *Service) readAnomalyItems(settings store.Settings) (map[string]anomalyItemRef, error) {
	out := map[string]anomalyItemRef{}
	targets, err := s.store.ListTargets()
	if err != nil {
		return nil, err
	}
	for _, t := range targets {
		out[t.ID] = anomalyItemRef{
			TargetID: t.ID, Domain: anomalyDomainContainer, Name: t.ContainerName,
			Scheduled: settings.ContainersEnabled && t.IncludeInSchedule,
		}
	}
	vms, err := s.store.ListVMTargets()
	if err != nil {
		return nil, err
	}
	for _, vm := range vms {
		out[vm.ID] = anomalyItemRef{
			TargetID: vm.ID, Domain: anomalyDomainVM, Name: vm.Name,
			Scheduled: settings.VMsEnabled && vm.IncludeInSchedule,
		}
	}
	sets, err := s.store.ListFileSets()
	if err != nil {
		return nil, err
	}
	for _, fs := range sets {
		out[fs.ID] = anomalyItemRef{
			TargetID: fs.ID, Domain: "files", Name: fs.Name,
			Scheduled: settings.FilesEnabled && fs.Enabled,
		}
	}
	out[store.FlashTargetID] = anomalyItemRef{
		TargetID: store.FlashTargetID, Domain: "flash", Scheduled: settings.FlashEnabled,
	}
	out[store.ConfigTargetID] = anomalyItemRef{
		TargetID: store.ConfigTargetID, Domain: "config", Scheduled: settings.ConfigEnabled,
	}
	return out, nil
}

// anomalyItemRefs reads the items for a caller that has no pass of its own.
func (s *Service) anomalyItemRefs() (map[string]anomalyItemRef, error) {
	settings, err := s.store.GetSettings()
	if err != nil {
		return nil, err
	}
	return s.readAnomalyItems(settings)
}

// anomalyRunDomain inverts domainRunTargetID: a maintenance or drill run is
// recorded on a reserved id for flash and config and on the domain's own name
// for the rest.
func anomalyRunDomain(targetID string) string {
	switch targetID {
	case store.FlashTargetID, store.ConfigTargetID, "containers", "vms", "files", "zfs":
		return targetID
	}
	return ""
}

// drillScope is the series one restore check belongs to. A named off-site
// target is a series of its own, so one bad copy is a finding rather than a gap
// in the domain's history.
func drillScope(k store.DrillKey) anomalyScope {
	id := k.Domain + ":" + k.Source
	if k.TargetID != "" {
		id = k.Domain + ":offsite:" + k.TargetID
	}
	return anomalyScope{Kind: anomalyScopeDomain, ID: id}
}

func domainOfDrillScope(sc anomalyScope) string {
	domain, _, _ := strings.Cut(sc.ID, ":")
	return domain
}

func openByMetric(rows map[string]store.Anomaly) map[string]store.Anomaly {
	out := make(map[string]store.Anomaly, len(rows))
	for _, row := range rows {
		out[row.Metric] = row
	}
	return out
}

func byFamily(expectations []store.AnomalyExpectation) map[string]store.AnomalyExpectation {
	out := make(map[string]store.AnomalyExpectation, len(expectations))
	for _, x := range expectations {
		out[x.Family] = x
	}
	return out
}

func newestEligibleAt(rows []store.SeriesRun) int64 {
	var newest int64
	for _, run := range eligibleRuns(rows) {
		if run.StartedAt > newest {
			newest = run.StartedAt
		}
	}
	return newest
}

func scopeCounts(open []store.Anomaly, sc anomalyScope) (AnomalyOpenCounts, bool) {
	var counts AnomalyOpenCounts
	held := false
	for _, row := range open {
		if row.ScopeKind != sc.Kind || row.ScopeID != sc.ID {
			continue
		}
		switch row.Severity {
		case "critical":
			counts.Critical++
		case "warning":
			counts.Warning++
		case "info":
			counts.Info++
		}
		held = held || anomalyHolds(row)
	}
	return counts, held
}

// unmeasuredNames is every repository no backend can measure, each named once
// however many domains write to it.
func unmeasuredNames(byDomain map[string][]string) []string {
	var out []string
	for _, names := range byDomain {
		for _, name := range names {
			if !slices.Contains(out, name) {
				out = append(out, name)
			}
		}
	}
	slices.Sort(out)
	return out
}

func backfillSummary(slots []store.AnomalyBackfillSlot) AnomalyBackfillSummary {
	out := AnomalyBackfillSummary{Slots: len(slots)}
	for _, slot := range slots {
		switch {
		case slot.Done:
			out.Done++
		case slot.Error != "":
			out.Failed++
		}
		out.Filled += slot.Filled
		out.WithoutSummary += slot.WithoutSummary
	}
	return out
}

// effectiveNotifyMin is the severity from which an item's findings are pushed:
// its own override, else the global minimum.
func effectiveNotifyMin(item, global string) string {
	for _, candidate := range []string{item, global} {
		if slices.Contains(anomalyNotifyLevels, candidate) {
			return candidate
		}
	}
	return "critical"
}

func sortedItemRefs(items map[string]anomalyItemRef) []anomalyItemRef {
	out := slices.Collect(maps.Values(items))
	slices.SortFunc(out, func(a, b anomalyItemRef) int {
		if a.Domain != b.Domain {
			return strings.Compare(a.Domain, b.Domain)
		}
		if a.Name != b.Name {
			return strings.Compare(a.Name, b.Name)
		}
		return strings.Compare(a.TargetID, b.TargetID)
	})
	return out
}
