package api

import (
	"math"
	"slices"
	"strings"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

// Sensitivity is how far a value has to deviate from a series' own history
// before a detector raises it.
type Sensitivity string

const (
	sensStrict     Sensitivity = "strict"
	sensBalanced   Sensitivity = "balanced"
	sensPermissive Sensitivity = "permissive"
)

// seriesKind is which history a scope reads: an item's own backups or the
// database dumps of a container.
type seriesKind string

const (
	seriesItem seriesKind = "item"
	seriesDump seriesKind = "dump"
)

const (
	detectorNewData     = "new_data"
	detectorSource      = "source"
	detectorDuration    = "duration"
	detectorReliability = "reliability"
	detectorIntegrity   = "integrity"
	detectorCapacity    = "capacity"
)

const (
	metricNewData            = "new_data"
	metricNewDataRewrite     = "new_data_rewrite"
	metricNewDataFull        = "new_data_full"
	metricSourceBytesShrink  = "source_bytes_shrink"
	metricSourceBytesGrowth  = "source_bytes_growth"
	metricSourceFilesShrink  = "source_files_shrink"
	metricDumpBytesShrink    = "dump_bytes_shrink"
	metricDumpBytesGrowth    = "dump_bytes_growth"
	metricDurationSlower     = "duration_slower"
	metricDumpDurationSlower = "dump_duration_slower"
	metricFailureStreak      = "failure_streak"
	metricFlaky              = "flaky"
	metricDumpFailureStreak  = "dump_failure_streak"
	metricDumpFlaky          = "dump_flaky"
	metricDrillSubset        = "drill_subset"
	metricDrillDR            = "drill_dr"
	metricCapacityETA        = "capacity_eta"
	metricCapacityLow        = "capacity_low"
)

// anomalyDetectors is the closed catalogue: which rule stands behind every
// metric. A fingerprint is built from it, so the detector of a finding and the
// detector of the row it refreshes can never drift apart.
var anomalyDetectors = map[string]string{
	metricNewData:            detectorNewData,
	metricNewDataRewrite:     detectorNewData,
	metricNewDataFull:        detectorNewData,
	metricSourceBytesShrink:  detectorSource,
	metricSourceBytesGrowth:  detectorSource,
	metricSourceFilesShrink:  detectorSource,
	metricDumpBytesShrink:    detectorSource,
	metricDumpBytesGrowth:    detectorSource,
	metricDurationSlower:     detectorDuration,
	metricDumpDurationSlower: detectorDuration,
	metricFailureStreak:      detectorReliability,
	metricFlaky:              detectorReliability,
	metricDumpFailureStreak:  detectorReliability,
	metricDumpFlaky:          detectorReliability,
	metricDrillSubset:        detectorIntegrity,
	metricDrillDR:            detectorIntegrity,
	metricCapacityETA:        detectorCapacity,
	metricCapacityLow:        detectorCapacity,
}

// The families a user marks as expected, one per rule and direction, so that
// accepting a new level leaves the opposite direction watching.
const (
	familyNewData         = "new_data"
	familySourceBytesDown = "source_bytes_down"
	familySourceBytesUp   = "source_bytes_up"
	familySourceFilesDown = "source_files_down"
	familyDuration        = "duration"
	familyDumpBytesDown   = "dump_bytes_down"
	familyDumpBytesUp     = "dump_bytes_up"
	familyDumpDuration    = "dump_duration"
)

// anomalyDomainContainer and anomalyDomainVM are the item domains that are
// singled out: a container has a dump series next to its backups, and a VM's
// runs only carry the data its disks added once the run was measured.
const (
	anomalyDomainContainer = "container"
	anomalyDomainVM        = "vm"
)

const (
	// anomalyMinSamples is how much history a detector wants before it judges
	// anything. Sensitivity changes how far a value has to deviate, not how
	// much history is needed, so the number is the same for every preset.
	anomalyMinSamples  = 10
	anomalyNewDataDays = 30
	// newDataRefFloor keeps a series that usually adds almost nothing from
	// raising on the first backup that stores a few hundred megabytes.
	newDataRefFloor = 64 << 20
	// newDataMinGap and newDataMaxGap bound the time a run had to collect its
	// data, so that a burst minutes apart and a machine that was off for
	// months stay comparable.
	newDataMinGap = 300
	newDataMaxGap = 30 * 86400
	rewriteShare  = 0.5
	rewriteFloor  = 64 << 20
	// anomalyWindow is how many of a series' newest samples the size and
	// duration rules take their level from.
	anomalyWindow = 30
	// collapseRefSamples keeps the collapse reference close to the present, so
	// that a source which was larger years ago cannot make today's emptying
	// look mild.
	collapseRefSamples = 5
	anomalyDrainDays   = 30
	collapseFloorItem  = 64 << 20
	collapseFloorDump  = 1 << 20
	collapseFloorFiles = 100
	// dumpCollapseMinRef keeps the dump of a nearly empty database out of the
	// collapse rule, where a few kilobytes of noise are a large share.
	dumpCollapseMinRef  = 4 << 20
	strongShrinkFrac    = 0.5
	shrinkClearedFrac   = 0.9
	growthClearedFrac   = 1.1
	durationClearedFrac = 1.2
	// The spread floors below which a series counts as noise-free, absolute and
	// relative to its own level.
	sizeMADFloorBytes        = 1 << 20
	sizeMADFloorFiles        = 10
	sizeRelativeMADFloor     = 0.02
	durationMADFloorMS       = 1000
	durationRelativeMADFloor = 0.05
	// reliabilityRuns is how far back a failure streak and a flaky series are
	// counted, and flakyMinRuns how much history that judgement needs.
	reliabilityRuns = 10
	flakyMinRuns    = 5
)

// sensParams are the thresholds one preset hands every detector.
type sensParams struct {
	K            float64
	NewDataMult  float64
	NewDataFloor int64
	CollapseFrac float64
	ShrinkRatio  float64
	ShrinkFloor  int64
	FilesFloor   int64
	GrowthRatio  float64
	GrowthFloor  int64
	DurRatio     float64
	DurFloorMS   int64
	StreakWarn   int
	FlakyK       int
	EtaWarnDays  float64
	EtaCritDays  float64
}

func paramsFor(s Sensitivity) sensParams {
	switch s {
	case sensStrict:
		return sensParams{
			K: 2.5, NewDataMult: 3, NewDataFloor: 256 << 20,
			CollapseFrac: 0.25, ShrinkRatio: 0.85, ShrinkFloor: 256 << 20, FilesFloor: 200,
			GrowthRatio: 1.5, GrowthFloor: 1 << 30,
			DurRatio: 2, DurFloorMS: 6 * 60 * 1000,
			StreakWarn: 2, FlakyK: 2, EtaWarnDays: 56, EtaCritDays: 14,
		}
	case sensPermissive:
		return sensParams{
			K: 5.0, NewDataMult: 10, NewDataFloor: 4 << 30,
			CollapseFrac: 0.05, ShrinkRatio: 0.50, ShrinkFloor: 5 << 30, FilesFloor: 10000,
			GrowthRatio: 4.0, GrowthFloor: 20 << 30,
			DurRatio: 5, DurFloorMS: 30 * 60 * 1000,
			StreakWarn: 5, FlakyK: 5, EtaWarnDays: 14, EtaCritDays: 3,
		}
	default:
		return sensParams{
			K: 3.5, NewDataMult: 5, NewDataFloor: 1 << 30,
			CollapseFrac: 0.10, ShrinkRatio: 0.70, ShrinkFloor: 1 << 30, FilesFloor: 1000,
			GrowthRatio: 2.0, GrowthFloor: 5 << 30,
			DurRatio: 3, DurFloorMS: 10 * 60 * 1000,
			StreakWarn: 3, FlakyK: 3, EtaWarnDays: 28, EtaCritDays: 7,
		}
	}
}

// resolveSensitivity lets an item's own preset win over the global one, and
// falls back to balanced for anything it does not know.
func resolveSensitivity(item, global string) Sensitivity {
	for _, candidate := range []Sensitivity{Sensitivity(item), Sensitivity(global)} {
		switch candidate {
		case sensStrict, sensBalanced, sensPermissive:
			return candidate
		}
	}
	return sensBalanced
}

// itemInput is one series and everything a pass knows about it. The detectors
// are pure: no store, no clock, no network call.
type itemInput struct {
	Kind   seriesKind
	Domain string
	// Series is the item's own history, newest first, as the store returns it.
	Series []store.SeriesRun
	// NewData is the new-data window, newest first, with enough history around
	// it for every evaluated run to find its own samples.
	NewData []store.SeriesRun
	// Open are the rows a previous pass left open, by metric. A condition's
	// level is frozen on them.
	Open map[string]store.Anomaly
	// Expectations are what a user marked as expected, by family.
	Expectations map[string]store.AnomalyExpectation
	Sens         Sensitivity
	// EvaluateFrom is the newest run start a previous pass judged. Events on
	// newer runs are judged now, so two runs coalesced into one pass are both
	// seen.
	EvaluateFrom int64
}

// finding is one raised event or condition, in the shape a row is written from.
type finding struct {
	Metric, Severity              string
	RunID, LastGoodRunID          string
	RunAt                         int64
	Observed, Expected, Threshold float64
	Samples                       int
	Details                       map[string]any
	Event                         bool
}

// absence is a metric that was evaluated and whose condition is not there, so
// that an open row can resolve and a closed episode can end.
type absence struct {
	Metric   string
	Observed float64
}

// detectNewData raises the three rules around the data one backup added: an
// unusual amount against the series' own history, a backup that stored most of
// the source again, and a backup that found no parent at all. The findings come
// back in run order, next to how many samples the newest run could learn from.
func detectNewData(in itemInput, p sensParams) ([]finding, int) {
	// A dump is a full logical export every time, so there is no such thing as
	// an unusual amount of new data in one.
	if in.Kind == seriesDump {
		return nil, 0
	}
	rows := oldestFirst(in.NewData)
	if len(rows) == 0 {
		return nil, 0
	}

	measuredOnly := in.Domain == anomalyDomainVM
	isSample := make([]bool, len(rows))
	for i, run := range rows {
		// The oldest row has nothing before it to measure a rate against, which
		// is also why the series' first backup is never a sample.
		isSample[i] = i > 0 &&
			!freshUpload(run) &&
			!selectionChangedAt(rows, i) &&
			(!measuredOnly || run.SourceBytes != nil)
	}
	ceiling := in.Expectations[familyNewData].Ceiling

	var found []finding
	for i, run := range rows {
		if i == 0 || run.StartedAt <= in.EvaluateFrom || selectionChangedAt(rows, i) {
			continue
		}
		samples := newDataSamples(rows, isSample, i)
		switch {
		case rewroteMostOfTheSource(run, len(samples), ceiling):
			found = append(found, rewriteFinding(run, rows[i-1], len(samples)))
		case freshUpload(run) && allFilesNew(run):
			found = append(found, fullUploadFinding(run, len(samples)))
		default:
			if spike, ok := newDataSpike(rows, samples, i, p, ceiling); ok {
				found = append(found, spike)
			}
		}
	}
	return found, min(len(newDataSamples(rows, isSample, len(rows)-1)), anomalyMinSamples)
}

// newDataSamples picks what run i is judged against: the eligible samples of
// the last thirty days, and when the series is too young or too slow for that,
// the newest ten of any age.
func newDataSamples(rows []store.SeriesRun, isSample []bool, i int) []int {
	from := rows[i].StartedAt - anomalyNewDataDays*86400
	var recent, all []int
	for j := range i {
		if !isSample[j] {
			continue
		}
		all = append(all, j)
		if rows[j].StartedAt >= from {
			recent = append(recent, j)
		}
	}
	if len(recent) >= anomalyMinSamples {
		return recent
	}
	if len(all) > anomalyMinSamples {
		return all[len(all)-anomalyMinSamples:]
	}
	return all
}

// newDataSpike raises a run that stored far more than this series ever stores,
// both in total and per second of the time it had to collect it. A modified
// z-score would be the obvious gate and does not work here: on a series with two
// modes, hourly runs adding almost nothing and one nightly run adding a
// gigabyte, the spread sits between the modes and a real outlier scores below
// one.
func newDataSpike(rows []store.SeriesRun, samples []int, i int, p sensParams, ceiling float64) (finding, bool) {
	if len(samples) < anomalyMinSamples {
		return finding{}, false
	}
	bytes := make([]float64, len(samples))
	rates := make([]float64, len(samples))
	for n, j := range samples {
		bytes[n] = float64(rows[j].Bytes)
		rates[n] = float64(rows[j].Bytes) / float64(elapsedBefore(rows, j))
	}
	refBytes, refRate := newDataReference(bytes), newDataReference(rates)

	run := rows[i]
	observed := float64(run.Bytes)
	elapsed := elapsedBefore(rows, i)
	rate := observed / float64(elapsed)
	threshold := p.NewDataMult * math.Max(refBytes, newDataRefFloor)
	rateThreshold := p.NewDataMult * math.Max(refRate, float64(newDataRefFloor)/86400)
	if observed < float64(p.NewDataFloor) || observed < threshold ||
		rate < rateThreshold || observed <= ceiling {
		return finding{}, false
	}
	return finding{
		Metric: metricNewData, Severity: "warning",
		RunID: run.ID, RunAt: run.StartedAt,
		Observed: observed, Expected: refBytes, Threshold: threshold,
		Samples: len(samples), Event: true,
		Details: finiteDetails(map[string]any{
			"refBytes":   refBytes,
			"refRate":    refRate,
			"rate":       rate,
			"elapsedSec": elapsed,
		}),
	}, true
}

// rewroteMostOfTheSource is true when at least half of everything the item backs
// up was stored again although an earlier backup existed, the trace ransomware
// leaves when it encrypts files in place. An unknown parent counts as one, so a
// failed parent probe cannot silence the rule.
func rewroteMostOfTheSource(run store.SeriesRun, samples int, ceiling float64) bool {
	if freshUpload(run) || run.SourceBytes == nil || samples == 0 {
		return false
	}
	observed := float64(run.Bytes)
	return observed >= rewriteShare*float64(*run.SourceBytes) &&
		observed >= rewriteFloor && observed > ceiling
}

func rewriteFinding(run, previous store.SeriesRun, samples int) finding {
	source := float64(*run.SourceBytes)
	observed := float64(run.Bytes)
	details := map[string]any{
		"sourceBytes": source,
		"ratio":       observed / source,
		"lastGoodAt":  previous.StartedAt,
	}
	if allFilesNew(run) {
		details["allFilesNew"] = true
	}
	if run.HasParent != nil {
		details["hasParent"] = *run.HasParent
	}
	return finding{
		Metric: metricNewDataRewrite, Severity: "critical",
		RunID: run.ID, LastGoodRunID: previous.ID, RunAt: run.StartedAt,
		Observed: observed, Expected: source, Threshold: rewriteShare * source,
		Samples: samples, Event: true, Details: finiteDetails(details),
	}
}

// fullUploadFinding records that restic matched no parent and read every file
// again. A repository move, a changed encryption or a lost parent explains it,
// so it is information and never an alarm.
func fullUploadFinding(run store.SeriesRun, samples int) finding {
	return finding{
		Metric: metricNewDataFull, Severity: "info",
		RunID: run.ID, RunAt: run.StartedAt,
		Observed: float64(run.Bytes), Samples: samples, Event: true,
	}
}

// newDataReference is the k-th largest of xs, the second largest below two
// hundred samples and about the 99th percentile above. One earlier spike is
// ignored, while a rarer mode of more than one percent of the runs, such as a
// nightly dump among hourly backups, stays inside the reference.
func newDataReference(xs []float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	sorted := slices.Clone(xs)
	slices.Sort(sorted)
	k := min(max(2, int(math.Ceil(float64(len(sorted))/100))), len(sorted))
	return sorted[len(sorted)-k]
}

// elapsedBefore is the time run i had to collect its data, counted from the
// previous eligible run of the series.
func elapsedBefore(rows []store.SeriesRun, i int) int64 {
	return min(max(rows[i].StartedAt-rows[i-1].StartedAt, newDataMinGap), newDataMaxGap)
}

// selectionChangedAt says whether run i is the first one under a changed
// selection. Its backup covers different data by configuration, so it is
// neither judged nor learned from.
func selectionChangedAt(rows []store.SeriesRun, i int) bool {
	if i == 0 || rows[i].SelectionFP == nil || rows[i-1].SelectionFP == nil {
		return false
	}
	return *rows[i].SelectionFP != *rows[i-1].SelectionFP
}

func freshUpload(run store.SeriesRun) bool {
	return run.HasParent != nil && *run.HasParent == 0
}

func allFilesNew(run store.SeriesRun) bool {
	return run.FilesNew != nil && run.SourceFiles != nil &&
		*run.SourceFiles > 0 && *run.FilesNew == *run.SourceFiles
}

// oldestFirst turns a series the store returned newest first around, so that a
// run's predecessor is the element before it.
func oldestFirst(runs []store.SeriesRun) []store.SeriesRun {
	out := make([]store.SeriesRun, len(runs))
	for i, run := range runs {
		out[len(runs)-1-i] = run
	}
	return out
}

// detectSource watches how much a series backs up: a source that collapsed or
// drained away, one that shrank or grew far beyond its own spread, and the same
// rules on the file count. The number that comes back is how much history the
// size rules could learn from.
func detectSource(in itemInput, p sensParams) ([]finding, []absence, int) {
	rows := oldestFirst(eligibleRuns(in.Series))
	rebase := selectionRebaseAt(in.Series)

	var found []finding
	var absent []absence
	learning := 0
	for i, rule := range sourceRules(in.Kind, p) {
		ruleFound, ruleAbsent, samples := rule.evaluate(in.Open, in.Expectations, rows, rebase, p)
		found = append(found, ruleFound...)
		absent = append(absent, ruleAbsent...)
		if i == 0 {
			learning = min(samples, anomalyMinSamples)
		}
	}
	return found, absent, learning
}

// sourceRules are the size metrics a series is watched on. A dump is one
// logical export written to standard input, so it has its own floors and no
// file count to speak of.
func sourceRules(kind seriesKind, p sensParams) []sizeRule {
	bytes := sizeRule{
		shrinkMetric: metricSourceBytesShrink, growthMetric: metricSourceBytesGrowth,
		downFamily: familySourceBytesDown, upFamily: familySourceBytesUp,
		value:         func(run store.SeriesRun) *int64 { return run.SourceBytes },
		collapseFloor: collapseFloorItem,
		shrinkFloor:   float64(p.ShrinkFloor),
		growthFloor:   float64(p.GrowthFloor),
		madFloor:      sizeMADFloorBytes,
	}
	if kind == seriesDump {
		bytes.shrinkMetric, bytes.growthMetric = metricDumpBytesShrink, metricDumpBytesGrowth
		bytes.downFamily, bytes.upFamily = familyDumpBytesDown, familyDumpBytesUp
		bytes.collapseFloor, bytes.minCollapseRef = collapseFloorDump, dumpCollapseMinRef
		return []sizeRule{bytes}
	}
	files := sizeRule{
		shrinkMetric:  metricSourceFilesShrink,
		downFamily:    familySourceFilesDown,
		value:         func(run store.SeriesRun) *int64 { return run.SourceFiles },
		collapseFloor: collapseFloorFiles,
		shrinkFloor:   float64(p.FilesFloor),
		madFloor:      sizeMADFloorFiles,
	}
	return []sizeRule{bytes, files}
}

// sizeRule is one metric of a series and the floors below which a change on it
// is too small to mean anything. A rule without a growth metric watches one
// direction only.
type sizeRule struct {
	shrinkMetric, growthMetric string
	downFamily, upFamily       string
	value                      func(store.SeriesRun) *int64
	collapseFloor              float64
	minCollapseRef             float64
	shrinkFloor                float64
	growthFloor                float64
	madFloor                   float64
}

func (r sizeRule) evaluate(open map[string]store.Anomaly, expectations map[string]store.AnomalyExpectation,
	rows []store.SeriesRun, rebase int64, p sensParams) ([]finding, []absence, int) {
	samples := measurements(rows, r.value)
	if len(samples) == 0 {
		return nil, nil, 0
	}
	current, prior := samples[len(samples)-1], samples[:len(samples)-1]

	// Both downward rules write the same fingerprint, and a collapse is the one
	// that holds retention, so it wins where they overlap. It also reads the
	// samples a user's expectation cut away, because accepting a smaller source
	// must not switch the data-loss guard off.
	down := newestSamples(samplesFrom(prior, max(rebase, expectations[r.downFamily].SinceAt)), anomalyWindow)
	raised := r.collapsed(current, samplesFrom(prior, rebase), p)
	if raised == nil {
		raised = r.shrank(current, down, p)
	}
	found, absent := verdict(open, r.shrinkMetric, current, raised, func(expected float64) bool {
		return current.value >= shrinkClearedFrac*expected
	})

	if r.growthMetric != "" {
		up := newestSamples(samplesFrom(prior, max(rebase, expectations[r.upFamily].SinceAt)), anomalyWindow)
		grownFound, grownAbsent := verdict(open, r.growthMetric, current, r.grew(current, up, p),
			func(expected float64) bool { return current.value <= growthClearedFrac*expected })
		found = append(found, grownFound...)
		absent = append(absent, grownAbsent...)
	}
	return found, absent, len(down)
}

// collapsed is the data-loss rule: what the item backs up is a fraction of what
// it backed up before, measured both against the level of the last few runs and
// against the highest level of the last month. The second reference catches a
// source emptied in steps that each stayed under the per-run rules and were
// absorbed into the first.
func (r sizeRule) collapsed(current measurement, prior []measurement, p sensParams) *finding {
	if len(prior) == 0 {
		return nil
	}
	reference := newestSamples(prior, collapseRefSamples)
	month := samplesFrom(prior, current.at-anomalyDrainDays*86400)
	level := median(sampleValues(reference))
	peak := 0.0
	if len(month) > 0 {
		peak = slices.Max(sampleValues(month))
	}

	collapse, drain := r.lost(current.value, level, p), r.lost(current.value, peak, p)
	if !collapse && !drain {
		return nil
	}
	expected, samples := level, len(reference)
	if !collapse {
		expected, samples = peak, len(month)
	}
	details := map[string]any{"ratio": current.value / expected}
	if collapse {
		details["collapse"] = true
	}
	if drain {
		details["drain"] = true
	}
	return &finding{
		Metric: r.shrinkMetric, Severity: "critical",
		RunID: current.runID, RunAt: current.at, LastGoodRunID: prior[len(prior)-1].runID,
		Observed: current.value, Expected: expected, Threshold: p.CollapseFrac * expected,
		Samples: samples, Details: finiteDetails(details),
	}
}

// lost is how much of a reference level has to be gone before it counts as data
// loss rather than as a source that simply changed.
func (r sizeRule) lost(value, reference float64, p sensParams) bool {
	return reference >= r.minCollapseRef &&
		value <= p.CollapseFrac*reference &&
		reference-value >= r.collapseFloor
}

// shrank is the rule for a source that lost a large share of itself without
// collapsing: far under the level of its window, far enough in absolute terms
// to matter, and outside the spread the series usually has.
func (r sizeRule) shrank(current measurement, samples []measurement, p sensParams) *finding {
	if len(samples) < anomalyMinSamples {
		return nil
	}
	xs := sampleValues(samples)
	level := median(xs)
	spread := r.spread(xs, level)
	z := modifiedZ(current.value, level, spread)
	if current.value >= p.ShrinkRatio*level || level-current.value < r.shrinkFloor || z > -p.K {
		return nil
	}
	severity := "warning"
	if current.value < strongShrinkFrac*level {
		severity = "critical"
	}
	return &finding{
		Metric: r.shrinkMetric, Severity: severity,
		RunID: current.runID, RunAt: current.at, LastGoodRunID: samples[len(samples)-1].runID,
		Observed: current.value, Expected: level, Threshold: p.ShrinkRatio * level,
		Samples: len(samples), Details: levelDetails(current.value, level, spread, z),
	}
}

// grew is the same rule in the other direction. A source that gained a lot is
// worth a look and never an alarm: nothing is lost.
func (r sizeRule) grew(current measurement, samples []measurement, p sensParams) *finding {
	if len(samples) < anomalyMinSamples {
		return nil
	}
	xs := sampleValues(samples)
	level := median(xs)
	spread := r.spread(xs, level)
	z := modifiedZ(current.value, level, spread)
	if current.value <= p.GrowthRatio*level || current.value-level < r.growthFloor || z < p.K {
		return nil
	}
	return &finding{
		Metric: r.growthMetric, Severity: "warning",
		RunID: current.runID, RunAt: current.at,
		Observed: current.value, Expected: level, Threshold: p.GrowthRatio * level,
		Samples: len(samples), Details: levelDetails(current.value, level, spread, z),
	}
}

// spread is the series' own noise, floored so that a series which always
// measures the same value does not put every later value infinitely far out.
func (r sizeRule) spread(xs []float64, level float64) float64 {
	return math.Max(math.Max(mad(xs, level), sizeRelativeMADFloor*level), r.madFloor)
}

// detectDuration watches restic's own time for a series, the figure that leaves
// out container stops, hooks and BombVault's own lock waits. A faster run is
// never reported, and a live stall is the stall guard's business.
func detectDuration(in itemInput, p sensParams) ([]finding, []absence, int) {
	metric, family := metricDurationSlower, familyDuration
	if in.Kind == seriesDump {
		metric, family = metricDumpDurationSlower, familyDumpDuration
	}
	rows := oldestFirst(eligibleRuns(in.Series))
	measured := measurements(rows, func(run store.SeriesRun) *int64 { return run.ResticMS })
	if len(measured) == 0 {
		return nil, nil, 0
	}
	current, prior := measured[len(measured)-1], measured[:len(measured)-1]
	from := max(selectionRebaseAt(in.Series), in.Expectations[family].SinceAt)
	samples := newestSamples(samplesFrom(prior, from), anomalyWindow)

	found, absent := verdict(in.Open, metric, current, slowerRun(current, samples, metric, p),
		func(expected float64) bool { return current.value <= durationClearedFrac*expected })
	return found, absent, min(len(samples), anomalyMinSamples)
}

func slowerRun(current measurement, samples []measurement, metric string, p sensParams) *finding {
	if len(samples) < anomalyMinSamples {
		return nil
	}
	xs := sampleValues(samples)
	level := median(xs)
	spread := math.Max(math.Max(mad(xs, level), durationRelativeMADFloor*level), durationMADFloorMS)
	z := modifiedZ(current.value, level, spread)
	if current.value < p.DurRatio*level || current.value-level < float64(p.DurFloorMS) || z < p.K {
		return nil
	}
	return &finding{
		Metric: metric, Severity: "warning",
		RunID: current.runID, RunAt: current.at,
		Observed: current.value, Expected: level, Threshold: p.DurRatio * level,
		Samples: len(samples), Details: levelDetails(current.value, level, spread, z),
	}
}

// detectReliability watches whether a series finishes at all: failures one
// after another, and a series that fails often enough to be unreliable without
// ever failing twice in a row.
func detectReliability(in itemInput, p sensParams) ([]finding, []absence) {
	streakMetric, flakyMetric := metricFailureStreak, metricFlaky
	if in.Kind == seriesDump {
		streakMetric, flakyMetric = metricDumpFailureStreak, metricDumpFlaky
	}
	runs := newestFinished(in.Series)
	if len(runs) == 0 {
		return nil, nil
	}

	streak, failed, unbroken := 0, 0, true
	for _, run := range runs {
		if run.Status != "failed" {
			unbroken = false
			continue
		}
		failed++
		if unbroken {
			streak++
		}
	}
	raise := func(metric, severity string, observed, threshold float64) finding {
		return finding{
			Metric: metric, Severity: severity,
			RunID: runs[0].ID, RunAt: runs[0].StartedAt,
			Observed: observed, Threshold: threshold, Samples: len(runs),
			Details: map[string]any{"streak": streak, "failed": failed, "total": len(runs)},
		}
	}

	var found []finding
	var absent []absence
	switch {
	case streak >= 2*p.StreakWarn:
		found = append(found, raise(streakMetric, "critical", float64(streak), float64(p.StreakWarn)))
	case streak >= p.StreakWarn:
		found = append(found, raise(streakMetric, "warning", float64(streak), float64(p.StreakWarn)))
	default:
		absent = append(absent, absence{Metric: streakMetric, Observed: float64(streak)})
	}
	if len(runs) >= flakyMinRuns && failed >= p.FlakyK && streak < p.StreakWarn {
		found = append(found, raise(flakyMetric, "warning", float64(failed), float64(p.FlakyK)))
	} else {
		absent = append(absent, absence{Metric: flakyMetric, Observed: float64(failed)})
	}
	return found, absent
}

// drillMetrics are the two restore checks a finding can be raised from: the
// subset check that reads real pack data back, and the test restore from an
// off-site copy.
var drillMetrics = map[string]string{"subset": metricDrillSubset, "dr": metricDrillDR}

// drillSkipped marks a check that never ran, recorded so the dashboard says why
// instead of freezing the previous result.
const drillSkipped = "skipped:"

// detectIntegrity watches one series of restore checks: the newest one failed
// while an earlier one proved the backup restorable. A series that never passed
// is not a regression, and the failing check announces itself.
func detectIntegrity(key store.DrillKey, checks []store.RestoreDrill) ([]finding, []absence) {
	metric, known := drillMetrics[key.Kind]
	if !known {
		return nil, nil
	}
	ran := make([]store.RestoreDrill, 0, len(checks))
	for _, check := range checks {
		if !strings.HasPrefix(check.Detail, drillSkipped) {
			ran = append(ran, check)
		}
	}
	if len(ran) == 0 {
		return nil, nil
	}
	newest := ran[0]
	passedBefore := slices.ContainsFunc(ran[1:], func(check store.RestoreDrill) bool { return check.OK })
	if newest.OK || !passedBefore {
		return nil, []absence{{Metric: metric}}
	}
	return []finding{{
		Metric: metric, Severity: "critical",
		RunAt: newest.At, Samples: len(ran),
		Details: map[string]any{"source": key.Source, "kind": key.Kind},
	}}, nil
}

// verdict turns one metric's rules into what the lifecycle needs: the finding
// while the condition holds, an absence once it is gone. An open row carries
// the level its episode started from, and only a return to that level ends it,
// so a baseline that slowly absorbs the new value cannot clear an alarm on its
// own.
func verdict(open map[string]store.Anomaly, metric string, current measurement,
	raised *finding, cleared func(expected float64) bool) ([]finding, []absence) {
	row, isOpen := open[metric]
	switch {
	case isOpen && cleared(row.Expected):
		return nil, []absence{{Metric: metric, Observed: current.value}}
	case raised != nil:
		return []finding{*raised}, nil
	case isOpen:
		return []finding{{
			Metric: metric, Severity: row.Severity,
			RunID: current.runID, RunAt: current.at,
			Observed: current.value, Expected: row.Expected, Threshold: row.Threshold,
			Samples: row.Samples,
		}}, nil
	default:
		return nil, []absence{{Metric: metric, Observed: current.value}}
	}
}

// selectionRebaseAt is when the series last started to cover different data by
// configuration. The rules that compare against a level begin again there,
// because the runs before it measured something else.
func selectionRebaseAt(series []store.SeriesRun) int64 {
	rows := oldestFirst(eligibleRuns(series))
	for i := len(rows) - 1; i > 0; i-- {
		if selectionChangedAt(rows, i) {
			return rows[i].StartedAt
		}
	}
	return 0
}

// eligibleRuns keeps the runs a detector may learn from: a success that left a
// snapshot or a measurement behind. A success that recorded a decision instead
// of a backup, such as a container that was gone, carries neither and would
// otherwise read as a source that vanished.
func eligibleRuns(series []store.SeriesRun) []store.SeriesRun {
	out := make([]store.SeriesRun, 0, len(series))
	for _, run := range series {
		if run.Status == "success" && (run.SnapshotID != "" || run.SourceBytes != nil) {
			out = append(out, run)
		}
	}
	return out
}

// newestFinished keeps the runs the item is answerable for, newest first: a
// success or a failure of its own, never one somebody cancelled, one that was
// skipped, or one a restart cut short.
func newestFinished(series []store.SeriesRun) []store.SeriesRun {
	out := make([]store.SeriesRun, 0, reliabilityRuns)
	for _, run := range series {
		ownFailure := run.Status == "failed" && run.Error != store.ReasonInterrupted
		if run.Status != "success" && !ownFailure {
			continue
		}
		out = append(out, run)
		if len(out) == reliabilityRuns {
			break
		}
	}
	return out
}

// measurement is what one run of a series measured for one metric.
type measurement struct {
	runID string
	at    int64
	value float64
}

// measurements reads a metric off the runs that carry it, keeping the order it
// is given. A run whose column is NULL never measured it and is no sample.
func measurements(rows []store.SeriesRun, of func(store.SeriesRun) *int64) []measurement {
	out := make([]measurement, 0, len(rows))
	for _, run := range rows {
		if value := of(run); value != nil {
			out = append(out, measurement{runID: run.ID, at: run.StartedAt, value: float64(*value)})
		}
	}
	return out
}

func samplesFrom(samples []measurement, at int64) []measurement {
	for i, sample := range samples {
		if sample.at >= at {
			return samples[i:]
		}
	}
	return nil
}

func newestSamples(samples []measurement, n int) []measurement {
	if len(samples) <= n {
		return samples
	}
	return samples[len(samples)-n:]
}

func sampleValues(samples []measurement) []float64 {
	xs := make([]float64, len(samples))
	for i, sample := range samples {
		xs[i] = sample.value
	}
	return xs
}

func levelDetails(value, level, spread, z float64) map[string]any {
	return finiteDetails(map[string]any{
		"median": level,
		"mad":    spread,
		"z":      z,
		"ratio":  value / level,
	})
}

// finiteDetails drops what JSON cannot carry, so writing a finding can never
// fail over one figure that came out infinite.
func finiteDetails(details map[string]any) map[string]any {
	for key, value := range details {
		if f, ok := value.(float64); ok && (math.IsNaN(f) || math.IsInf(f, 0)) {
			delete(details, key)
		}
	}
	return details
}

// median is the middle of xs, or the mean of the two middle values. An empty
// input has no middle and gives zero.
func median(xs []float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	sorted := slices.Clone(xs)
	slices.Sort(sorted)
	mid := len(sorted) / 2
	if len(sorted)%2 == 1 {
		return sorted[mid]
	}
	return (sorted[mid-1] + sorted[mid]) / 2
}

// mad is the median absolute deviation from m, the spread that a single outlier
// cannot inflate.
func mad(xs []float64, m float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	deviations := make([]float64, len(xs))
	for i, x := range xs {
		deviations[i] = math.Abs(x - m)
	}
	return median(deviations)
}

// quantile is the q-th quantile of xs, q in [0,1], interpolated between the two
// neighbouring values.
func quantile(xs []float64, q float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	sorted := slices.Clone(xs)
	slices.Sort(sorted)
	pos := min(max(q, 0), 1) * float64(len(sorted)-1)
	lo := int(math.Floor(pos))
	hi := int(math.Ceil(pos))
	return sorted[lo] + (pos-float64(lo))*(sorted[hi]-sorted[lo])
}

// modifiedZ is how far x sits from the median in MAD units, on the scale that
// makes it comparable with a standard score. madFloored is the spread after its
// floor, so a series of identical values cannot put every value infinitely far
// out.
func modifiedZ(x, m, madFloored float64) float64 {
	if madFloored <= 0 {
		return 0
	}
	return 0.6745 * (x - m) / madFloored
}

// learningInfo is how many samples each family of rules could learn from, so
// the page can say what the baseline still lacks instead of staying silent.
type learningInfo struct {
	NewData, Source, Duration, Needed int
	// NoData marks a series that is measured and backs up nothing, which no
	// amount of further runs will turn into a baseline.
	NoData bool
}

// typicalValues are the medians behind an item's "usual" figures, each nil
// while its rule is still learning.
type typicalValues struct{ SourceBytes, NewDataBytes, ResticMS *int64 }

// itemResult is one series' whole verdict of one pass.
type itemResult struct {
	Findings []finding
	Absent   []absence
	Learning learningInfo
	Typical  typicalValues
}

// evaluateItem runs every rule a series is watched by and reports what each of
// them could learn from.
func evaluateItem(in itemInput) itemResult {
	p := paramsFor(in.Sens)
	newData, newDataSamples := detectNewData(in, p)
	source, sourceGone, sourceSamples := detectSource(in, p)
	duration, durationGone, durationSamples := detectDuration(in, p)
	reliability, reliabilityGone := detectReliability(in, p)

	res := itemResult{
		Learning: learningInfo{
			NewData: newDataSamples, Source: sourceSamples, Duration: durationSamples,
			Needed: anomalyMinSamples,
		},
	}
	res.Findings = append(append(append(append(res.Findings, newData...), source...), duration...), reliability...)
	res.Absent = append(append(append(res.Absent, sourceGone...), durationGone...), reliabilityGone...)

	eligible := oldestFirst(eligibleRuns(in.Series))
	sizes := measurements(eligible, func(run store.SeriesRun) *int64 { return run.SourceBytes })
	res.Learning.NoData = len(sizes) > 0 && slices.Max(sampleValues(sizes)) == 0
	res.Typical = typicalValues{
		SourceBytes: typicalOf(sizes, sourceSamples),
		ResticMS: typicalOf(measurements(eligible,
			func(run store.SeriesRun) *int64 { return run.ResticMS }), durationSamples),
		NewDataBytes: typicalOf(newDataMeasurements(in.NewData), newDataSamples),
	}
	return res
}

// typicalOf is the median of a rule's samples once it has enough of them. A
// rule that is still learning has no usual value to show.
func typicalOf(samples []measurement, learned int) *int64 {
	if learned < anomalyMinSamples || len(samples) == 0 {
		return nil
	}
	value := int64(median(sampleValues(newestSamples(samples, anomalyWindow))))
	return &value
}

// newDataMeasurements reads the data each backup of the window added. It is a
// plain column rather than a nullable one, so a run that measured nothing and
// one that added nothing look alike; the new-data rules tell them apart by the
// source figures, and the median here only needs the amounts.
func newDataMeasurements(window []store.SeriesRun) []measurement {
	rows := oldestFirst(window)
	out := make([]measurement, 0, len(rows))
	for _, run := range rows {
		out = append(out, measurement{runID: run.ID, at: run.StartedAt, value: float64(run.Bytes)})
	}
	return out
}
