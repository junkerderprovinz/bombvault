package api

import (
	"math"
	"slices"

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
	detectorNewData      = "new_data"
	metricNewData        = "new_data"
	metricNewDataRewrite = "new_data_rewrite"
	metricNewDataFull    = "new_data_full"
)

// anomalyDomainVM is the domain of a VM item, whose runs only carry the data
// its disks added once the run was measured.
const anomalyDomainVM = "vm"

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
	// NewData is the new-data window, newest first, with enough history around
	// it for every evaluated run to find its own samples.
	NewData []store.SeriesRun
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
	Detector, Metric, Severity    string
	RunID, LastGoodRunID          string
	RunAt                         int64
	Observed, Expected, Threshold float64
	Samples                       int
	Details                       map[string]any
	Event                         bool
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
	ceiling := in.Expectations[metricNewData].Ceiling

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
		Detector: detectorNewData, Metric: metricNewData, Severity: "warning",
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
		Detector: detectorNewData, Metric: metricNewDataRewrite, Severity: "critical",
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
		Detector: detectorNewData, Metric: metricNewDataFull, Severity: "info",
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
