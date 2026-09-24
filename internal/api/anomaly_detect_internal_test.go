package api

import (
	"encoding/json"
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

const (
	anomalyNow = int64(1_800_000_000)
	anomalyDay = int64(86400)
)

// steadyRuns is a series that adds the same amount every day, the history most
// of the new-data tests put one unusual run on top of.
func steadyRuns(n int, start, bytes int64) []store.SeriesRun {
	return daily(n, start, func(i int) store.SeriesRun {
		return mkRun(fmt.Sprintf("r%d", i), 0, "success", bytes, withParent(true))
	})
}

// after puts run at the head of a series the store returned newest first.
func after(run store.SeriesRun, history []store.SeriesRun) []store.SeriesRun {
	return append([]store.SeriesRun{run}, history...)
}

func newDataInput(rows []store.SeriesRun, evaluateFrom int64) itemInput {
	return itemInput{
		Kind:         seriesItem,
		NewData:      rows,
		Sens:         sensBalanced,
		EvaluateFrom: evaluateFrom,
	}
}

func runNewData(in itemInput) ([]finding, int) {
	return detectNewData(in, paramsFor(in.Sens))
}

func onlyFinding(t *testing.T, found []finding) finding {
	t.Helper()
	if len(found) != 1 {
		t.Fatalf("got %d findings, want 1: %+v", len(found), found)
	}
	return found[0]
}

func findingIDs(found []finding) []string {
	ids := make([]string, len(found))
	for i, f := range found {
		ids[i] = f.RunID
	}
	return ids
}

func TestStatsHelpersAreFinite(t *testing.T) {
	inputs := [][]float64{
		nil,
		{},
		{5},
		{2, 2, 2},
		{0, 0, 0, 0},
		{1, 2, 10},
		{-3, 0, 7, 100},
	}
	for _, xs := range inputs {
		m := median(xs)
		spread := mad(xs, m)
		z := modifiedZ(1, m, spread)
		values := map[string]float64{"median": m, "mad": spread, "z": z}
		for _, q := range []float64{0, 0.5, 0.9, 1} {
			values[fmt.Sprintf("q%v", q)] = quantile(xs, q)
		}
		for name, v := range values {
			if math.IsNaN(v) || math.IsInf(v, 0) {
				t.Fatalf("%s of %v = %v", name, xs, v)
			}
		}
		details := make(map[string]any, len(values))
		for name, v := range values {
			details[name] = v
		}
		if _, err := json.Marshal(finiteDetails(details)); err != nil {
			t.Fatalf("details of %v: %v", xs, err)
		}
	}

	if got := median([]float64{1, 2, 10}); got != 2 {
		t.Fatalf("median = %v, want 2", got)
	}
	if got := median([]float64{1, 2, 10, 20}); got != 6 {
		t.Fatalf("median of four = %v, want 6", got)
	}
	if got := mad([]float64{1, 2, 10}, 2); got != 1 {
		t.Fatalf("mad = %v, want 1", got)
	}
	if got := quantile([]float64{1, 2, 10}, 0); got != 1 {
		t.Fatalf("quantile(0) = %v, want 1", got)
	}
	if got := quantile([]float64{1, 2, 10}, 1); got != 10 {
		t.Fatalf("quantile(1) = %v, want 10", got)
	}
	if got := quantile([]float64{1, 2, 10}, 0.9); math.Abs(got-8.4) > 1e-9 {
		t.Fatalf("quantile(0.9) = %v, want 8.4", got)
	}
	if got := modifiedZ(10, 4, 2); math.Abs(got-0.6745*3) > 1e-9 {
		t.Fatalf("modifiedZ = %v", got)
	}

	inf := finiteDetails(map[string]any{"ratio": math.Inf(1), "median": 4.0})
	if _, ok := inf["ratio"]; ok {
		t.Fatal("an infinite value stayed in the details")
	}
	if inf["median"] != 4.0 {
		t.Fatalf("details = %v", inf)
	}
}

func TestNewDataSteadySeriesNotFlagged(t *testing.T) {
	rows := daily(40, anomalyNow-39*anomalyDay, func(i int) store.SeriesRun {
		bytes := int64(100<<20) + int64(i%3-1)*(10<<20)
		return mkRun(fmt.Sprintf("r%d", i), 0, "success", bytes, withParent(true))
	})

	found, learning := runNewData(newDataInput(rows, rows[1].StartedAt))
	if len(found) != 0 {
		t.Fatalf("a steady series raised %+v", found)
	}
	if learning != anomalyMinSamples {
		t.Fatalf("learning = %d, want %d", learning, anomalyMinSamples)
	}
}

func TestNewDataLearningBelowTenSamples(t *testing.T) {
	rows := daily(11, anomalyNow-10*anomalyDay, func(i int) store.SeriesRun {
		bytes := int64(100 << 20)
		if i == 10 {
			bytes = 50 << 30
		}
		return mkRun(fmt.Sprintf("r%d", i), 0, "success", bytes, withParent(true))
	})

	found, learning := runNewData(newDataInput(rows, rows[1].StartedAt))
	if len(found) != 0 {
		t.Fatalf("a learning series raised %+v", found)
	}
	// Ten runs precede the outlier, and the oldest of them is the series' first
	// backup, with nothing before it to compare against.
	if learning != 9 {
		t.Fatalf("learning = %d, want 9", learning)
	}
}

func TestNewDataOutlierFlaggedAgainstReference(t *testing.T) {
	history := steadyRuns(31, anomalyNow-31*anomalyDay, 200<<20)
	outlier := mkRun("spike", anomalyNow, "success", 20<<30, withParent(true))

	found, _ := runNewData(newDataInput(after(outlier, history), history[0].StartedAt))
	got := onlyFinding(t, found)
	if got.Metric != "new_data" || got.Severity != "warning" || got.RunID != "spike" {
		t.Fatalf("finding = %+v", got)
	}
	if got.Samples != 30 {
		t.Fatalf("samples = %d, want 30", got.Samples)
	}
	if got.Expected != float64(200<<20) || got.Details["refBytes"] != float64(200<<20) {
		t.Fatalf("reference = %v / %v", got.Expected, got.Details["refBytes"])
	}
	if got.Observed != float64(20<<30) || !got.Event {
		t.Fatalf("finding = %+v", got)
	}
}

func TestNewDataAbsoluteFloor(t *testing.T) {
	history := steadyRuns(31, anomalyNow-31*anomalyDay, 4096)

	small := mkRun("small", anomalyNow, "success", 500<<20, withParent(true))
	found, _ := runNewData(newDataInput(after(small, history), history[0].StartedAt))
	if len(found) != 0 {
		t.Fatalf("half a gigabyte crossed the one gigabyte floor: %+v", found)
	}

	big := mkRun("big", anomalyNow, "success", 3<<30, withParent(true))
	found, _ = runNewData(newDataInput(after(big, history), history[0].StartedAt))
	got := onlyFinding(t, found)
	if got.Metric != "new_data" || got.Severity != "warning" {
		t.Fatalf("finding = %+v", got)
	}
	if _, err := json.Marshal(got.Details); err != nil {
		t.Fatalf("details %v: %v", got.Details, err)
	}
	for key, value := range got.Details {
		if f, ok := value.(float64); ok && (math.IsNaN(f) || math.IsInf(f, 0)) {
			t.Fatalf("details[%q] = %v", key, f)
		}
	}
}

func TestNewDataHourlyWithNightlyDumpNotFlagged(t *testing.T) {
	const hours = 30 * 24
	rows := hourly(hours+1, anomalyNow-hours*3600, func(i int) store.SeriesRun {
		bytes := int64(5 << 20)
		if i%24 == 4 {
			bytes = 1 << 30
		}
		return mkRun(fmt.Sprintf("r%d", i), 0, "success", bytes, withParent(true))
	})
	from := rows[1].StartedAt

	nightly := append([]store.SeriesRun(nil), rows...)
	nightly[0].Bytes = 1100 << 20
	found, _ := runNewData(newDataInput(nightly, from))
	if len(found) != 0 {
		t.Fatalf("the nightly dump raised %+v", found)
	}

	// The reference is time-bounded for this series: over the newest 30 runs
	// alone the nightly mode is a single value, and a P90 over them sits at the
	// hourly level, so the same run would cross.
	recent := make([]float64, 0, 30)
	for _, run := range nightly[1:31] {
		recent = append(recent, float64(run.Bytes))
	}
	p90 := math.Max(quantile(recent, 0.9), float64(newDataRefFloor))
	if float64(nightly[0].Bytes) < paramsFor(sensBalanced).NewDataMult*p90 {
		t.Fatalf("a P90 reference of %v would not have flagged the run either", p90)
	}

	runaway := append([]store.SeriesRun(nil), rows...)
	runaway[0].Bytes = 25 << 30
	found, _ = runNewData(newDataInput(runaway, from))
	if got := onlyFinding(t, found); got.Metric != "new_data" {
		t.Fatalf("finding = %+v", got)
	}
}

func TestNewDataWeeklyFullAmongDailies(t *testing.T) {
	history := daily(35, anomalyNow-35*anomalyDay, func(i int) store.SeriesRun {
		bytes := int64(200 << 20)
		if i%7 == 0 {
			bytes = 5 << 30
		}
		return mkRun(fmt.Sprintf("r%d", i), 0, "success", bytes, withParent(true))
	})
	weekly := mkRun("weekly", anomalyNow, "success", 5<<30+200<<20, withParent(true))

	found, _ := runNewData(newDataInput(after(weekly, history), history[0].StartedAt))
	if len(found) != 0 {
		t.Fatalf("the weekly full backup raised %+v", found)
	}
}

func TestNewDataRateNormalisesLongGap(t *testing.T) {
	history := steadyRuns(31, anomalyNow-40*anomalyDay, 1<<30)
	gap := mkRun("gap", anomalyNow, "success", 8<<30, withParent(true))

	rows := after(gap, history)
	found, _ := runNewData(newDataInput(rows, history[0].StartedAt))
	if len(found) != 0 {
		t.Fatalf("ten days' worth of data in one backup raised %+v", found)
	}

	next := mkRun("next", anomalyNow+anomalyDay, "success", 8<<30, withParent(true))
	found, _ = runNewData(newDataInput(after(next, rows), gap.StartedAt))
	if got := onlyFinding(t, found); got.RunID != "next" {
		t.Fatalf("finding = %+v", got)
	}
}

func TestNewDataRewriteIsCritical(t *testing.T) {
	history := steadyRuns(12, anomalyNow-12*anomalyDay, 100<<20)
	rewrite := mkRun("rewrite", anomalyNow, "success", 30<<30, withParent(true), withSource(40<<30, 20000))

	found, _ := runNewData(newDataInput(after(rewrite, history), history[0].StartedAt))
	got := onlyFinding(t, found)
	if got.Metric != "new_data_rewrite" || got.Severity != "critical" {
		t.Fatalf("finding = %+v", got)
	}
	if got.Details["sourceBytes"] != float64(40<<30) {
		t.Fatalf("details = %v", got.Details)
	}
	if got.LastGoodRunID != history[0].ID {
		t.Fatalf("last good = %q, want %q", got.LastGoodRunID, history[0].ID)
	}
}

// Ransomware does not wait for a series to grow a history. A second backup
// that stored most of the source again is the rule's own case, whatever the
// spike rule still needs to know about the item.
func TestRewriteIsRaisedOnTheSecondBackup(t *testing.T) {
	first := mkRun("first", anomalyNow-anomalyDay, "success", 40<<30, withParent(false))
	second := mkRun("second", anomalyNow, "success", 30<<30, withParent(true), withSource(40<<30, 900))

	found, _ := runNewData(newDataInput(after(second, []store.SeriesRun{first}), first.StartedAt-1))
	got := onlyFinding(t, found)
	if got.Metric != "new_data_rewrite" || got.Severity != "critical" {
		t.Fatalf("finding = %+v", got)
	}
	if got.LastGoodRunID != "first" {
		t.Fatalf("last good = %q, want the backup before the rewrite", got.LastGoodRunID)
	}
	if got.Samples != 0 {
		t.Fatalf("samples = %d, want none: the rule judged one run against its parent", got.Samples)
	}
}

func TestRewriteBelowTheNewDataFloor(t *testing.T) {
	history := steadyRuns(3, anomalyNow-3*anomalyDay, 10<<20)

	rewrite := mkRun("rewrite", anomalyNow, "success", 500<<20, withParent(true), withSource(600<<20, 900))
	found, _ := runNewData(newDataInput(after(rewrite, history), history[0].StartedAt))
	if got := onlyFinding(t, found); got.Metric != "new_data_rewrite" || got.Severity != "critical" {
		t.Fatalf("finding = %+v", got)
	}

	small := mkRun("small", anomalyNow, "success", 40<<20, withParent(true), withSource(60<<20, 900))
	found, _ = runNewData(newDataInput(after(small, history), history[0].StartedAt))
	if len(found) != 0 {
		t.Fatalf("forty megabytes raised %+v", found)
	}
}

func TestRenamedFilesWithParentAreARewriteNotAFreshUpload(t *testing.T) {
	history := steadyRuns(3, anomalyNow-3*anomalyDay, 10<<20)
	renamed := func(opts ...runOpt) []store.SeriesRun {
		opts = append(opts, withSource(40<<30, 1000), withFilesNew(1000))
		return after(mkRun("renamed", anomalyNow, "success", 30<<30, opts...), history)
	}

	found, _ := runNewData(newDataInput(renamed(withParent(true)), history[0].StartedAt))
	got := onlyFinding(t, found)
	if got.Metric != "new_data_rewrite" || got.Severity != "critical" {
		t.Fatalf("finding = %+v", got)
	}
	if got.Details["allFilesNew"] != true {
		t.Fatalf("details = %v", got.Details)
	}

	found, _ = runNewData(newDataInput(renamed(withParent(false)), history[0].StartedAt))
	if got := onlyFinding(t, found); got.Metric != "new_data_full" || got.Severity != "info" {
		t.Fatalf("finding = %+v", got)
	}

	found, _ = runNewData(newDataInput(renamed(), history[0].StartedAt))
	if got := onlyFinding(t, found); got.Metric != "new_data_rewrite" {
		t.Fatalf("an unknown parent silenced the rewrite rule: %+v", got)
	}
}

func TestNewDataFullUploadIsInfoAndExcluded(t *testing.T) {
	history := steadyRuns(31, anomalyNow-32*anomalyDay, 200<<20)
	fresh := mkRun("fresh", anomalyNow-anomalyDay, "success", 50<<30,
		withParent(false), withSource(50<<30, 1000), withFilesNew(1000))
	later := mkRun("later", anomalyNow, "success", 20<<30, withParent(true))

	rows := after(later, after(fresh, history))
	found, _ := runNewData(newDataInput(rows, history[0].StartedAt))
	if ids := findingIDs(found); len(found) != 2 || ids[0] != "fresh" || ids[1] != "later" {
		t.Fatalf("findings = %+v", found)
	}
	if found[0].Metric != "new_data_full" || found[0].Severity != "info" {
		t.Fatalf("fresh upload = %+v", found[0])
	}
	if found[1].Metric != "new_data" {
		t.Fatalf("later run = %+v", found[1])
	}
	// Thirty of the window's rows precede the later run; the fresh upload is
	// not one of its samples.
	if found[1].Samples != 29 || found[1].Expected != float64(200<<20) {
		t.Fatalf("later run judged against %d samples of %v", found[1].Samples, found[1].Expected)
	}
}

func TestSelectionChangeExemptsTheNextRunOnly(t *testing.T) {
	history := daily(31, anomalyNow-33*anomalyDay, func(i int) store.SeriesRun {
		opts := []runOpt{withParent(true)}
		if i >= 10 {
			opts = append(opts, withSelection("a"))
		}
		return mkRun(fmt.Sprintf("r%d", i), 0, "success", 200<<20, opts...)
	})
	changed := mkRun("changed", anomalyNow-2*anomalyDay, "success", 20<<30, withSelection("b"))
	next := mkRun("next", anomalyNow-anomalyDay, "success", 20<<30, withParent(true), withSelection("b"))

	rows := after(next, after(changed, history))
	found, _ := runNewData(newDataInput(rows, history[0].StartedAt))
	got := onlyFinding(t, found)
	if got.RunID != "next" || got.Metric != "new_data" {
		t.Fatalf("finding = %+v", got)
	}
	// The runs from before the change keep counting, including the one where a
	// missing fingerprint turned into a known one.
	if got.Samples != 29 {
		t.Fatalf("samples = %d, want 29", got.Samples)
	}
}

func TestNewDataCeilingSuppresses(t *testing.T) {
	history := steadyRuns(31, anomalyNow-31*anomalyDay, 200<<20)
	ceiling := map[string]store.AnomalyExpectation{
		"new_data": {Family: "new_data", Ceiling: 25 << 30},
	}
	withCeiling := func(run store.SeriesRun, exp map[string]store.AnomalyExpectation) []finding {
		in := newDataInput(after(run, history), history[0].StartedAt)
		in.Expectations = exp
		found, _ := runNewData(in)
		return found
	}

	below := mkRun("below", anomalyNow, "success", 20<<30, withParent(true))
	if found := withCeiling(below, ceiling); len(found) != 0 {
		t.Fatalf("a run under the ceiling raised %+v", found)
	}

	above := mkRun("above", anomalyNow, "success", 30<<30, withParent(true))
	got := onlyFinding(t, withCeiling(above, ceiling))
	if got.Metric != "new_data" || got.Samples != 30 {
		t.Fatalf("finding = %+v", got)
	}

	rewrite := mkRun("rewrite", anomalyNow, "success", 20<<30, withParent(true), withSource(40<<30, 1000))
	if found := withCeiling(rewrite, nil); onlyFinding(t, found).Metric != "new_data_rewrite" {
		t.Fatalf("findings = %+v", found)
	}
	if found := withCeiling(rewrite, ceiling); len(found) != 0 {
		t.Fatalf("the ceiling did not cover the rewrite rule: %+v", found)
	}
}

func TestNewDataVMUsesMeasuredRowsOnly(t *testing.T) {
	old := daily(20, anomalyNow-21*anomalyDay, func(i int) store.SeriesRun {
		return mkRun(fmt.Sprintf("r%d", i), 0, "success", 100<<20, withParent(true))
	})
	first := mkRun("m0", anomalyNow-anomalyDay, "success", 20<<30, withParent(true), withSource(200<<30, 4000))
	second := mkRun("m1", anomalyNow, "success", 20<<30, withParent(true), withSource(200<<30, 4000))

	in := newDataInput(after(second, after(first, old)), first.StartedAt)
	in.Domain = "vm"
	found, learning := runNewData(in)
	if len(found) != 0 {
		t.Fatalf("the first measured runs raised %+v", found)
	}
	if learning != 1 {
		t.Fatalf("learning = %d, want 1", learning)
	}
}

func TestNewDataEvaluatesEveryRunSinceLastPass(t *testing.T) {
	history := steadyRuns(31, anomalyNow-33*anomalyDay, 200<<20)
	first := mkRun("o1", anomalyNow-2*anomalyDay, "success", 20<<30, withParent(true))
	second := mkRun("o2", anomalyNow-anomalyDay, "success", 150<<30, withParent(true))
	third := mkRun("o3", anomalyNow, "success", 800<<30, withParent(true))
	rows := after(third, after(second, after(first, history)))

	found, _ := runNewData(newDataInput(rows, first.StartedAt))
	if ids := findingIDs(found); len(ids) != 2 || ids[0] != "o2" || ids[1] != "o3" {
		t.Fatalf("findings = %v", findingIDs(found))
	}

	found, _ = runNewData(newDataInput(rows, history[0].StartedAt))
	if ids := findingIDs(found); len(ids) != 3 || ids[0] != "o1" {
		t.Fatalf("findings = %v", findingIDs(found))
	}
}

func TestSensitivityPresets(t *testing.T) {
	want := map[Sensitivity]sensParams{
		sensStrict: {
			K: 2.5, NewDataMult: 3, NewDataFloor: 256 << 20,
			CollapseFrac: 0.25, ShrinkRatio: 0.85, ShrinkFloor: 256 << 20, FilesFloor: 200,
			GrowthRatio: 1.5, GrowthFloor: 1 << 30,
			DurRatio: 2, DurFloorMS: 6 * 60 * 1000,
			StreakWarn: 2, FlakyK: 2, EtaWarnDays: 56, EtaCritDays: 14,
		},
		sensBalanced: {
			K: 3.5, NewDataMult: 5, NewDataFloor: 1 << 30,
			CollapseFrac: 0.10, ShrinkRatio: 0.70, ShrinkFloor: 1 << 30, FilesFloor: 1000,
			GrowthRatio: 2.0, GrowthFloor: 5 << 30,
			DurRatio: 3, DurFloorMS: 10 * 60 * 1000,
			StreakWarn: 3, FlakyK: 3, EtaWarnDays: 28, EtaCritDays: 7,
		},
		sensPermissive: {
			K: 5.0, NewDataMult: 10, NewDataFloor: 4 << 30,
			CollapseFrac: 0.05, ShrinkRatio: 0.50, ShrinkFloor: 5 << 30, FilesFloor: 10000,
			GrowthRatio: 4.0, GrowthFloor: 20 << 30,
			DurRatio: 5, DurFloorMS: 30 * 60 * 1000,
			StreakWarn: 5, FlakyK: 5, EtaWarnDays: 14, EtaCritDays: 3,
		},
	}
	for sens, params := range want {
		if got := paramsFor(sens); got != params {
			t.Fatalf("paramsFor(%q) = %+v, want %+v", sens, got, params)
		}
	}
	if got := paramsFor("bogus"); got != want[sensBalanced] {
		t.Fatalf("an unknown preset gives %+v", got)
	}

	cases := []struct {
		item, global string
		want         Sensitivity
	}{
		{"", "strict", sensStrict},
		{"permissive", "strict", sensPermissive},
		{"bogus", "", sensBalanced},
		{"", "", sensBalanced},
	}
	for _, c := range cases {
		if got := resolveSensitivity(c.item, c.global); got != c.want {
			t.Fatalf("resolveSensitivity(%q, %q) = %q, want %q", c.item, c.global, got, c.want)
		}
	}
}

// byteRuns is a daily series that measured its source, with a file count that
// stays put so that only the size rules can speak.
func byteRuns(n int, start int64, bytes func(i int) int64) []store.SeriesRun {
	return daily(n, start, func(i int) store.SeriesRun {
		return mkRun(fmt.Sprintf("r%d", i), 0, "success", 0, withSource(bytes(i), 1000))
	})
}

// fileRuns is the same series seen from the other side: a steady size and a
// moving file count.
func fileRuns(n int, start int64, files func(i int) int64) []store.SeriesRun {
	return daily(n, start, func(i int) store.SeriesRun {
		return mkRun(fmt.Sprintf("r%d", i), 0, "success", 0, withSource(10<<30, files(i)))
	})
}

func durationRuns(n int, start int64, ms func(i int) int64) []store.SeriesRun {
	return daily(n, start, func(i int) store.SeriesRun {
		return mkRun(fmt.Sprintf("r%d", i), 0, "success", 0, withSnapshot("snap"), withResticMS(ms(i)))
	})
}

// outcomeRuns builds a series from a list of outcomes, oldest first: s a
// success, f a failure, c a cancelled run, k a skipped one and i a run the
// startup sweep marked as interrupted.
func outcomeRuns(outcomes string) []store.SeriesRun {
	return daily(len(outcomes), anomalyNow-int64(len(outcomes))*anomalyDay, func(i int) store.SeriesRun {
		id := fmt.Sprintf("r%d", i)
		switch outcomes[i] {
		case 'f':
			return mkRun(id, 0, "failed", 0)
		case 'c':
			return mkRun(id, 0, "cancelled", 0)
		case 'k':
			return mkRun(id, 0, "skipped", 0)
		case 'i':
			return mkRun(id, 0, "failed", 0, withError(store.ReasonInterrupted))
		default:
			return mkRun(id, 0, "success", 0, withSnapshot("snap"))
		}
	})
}

func flatSize(v int64) func(int) int64 {
	return func(int) int64 { return v }
}

func sourceInput(rows []store.SeriesRun) itemInput {
	return itemInput{Kind: seriesItem, Series: rows, Sens: sensBalanced}
}

func runSource(in itemInput) ([]finding, []absence) {
	found, absent, _ := detectSource(in, paramsFor(in.Sens))
	return found, absent
}

func findingFor(t *testing.T, found []finding, metric string) finding {
	t.Helper()
	for _, f := range found {
		if f.Metric == metric {
			return f
		}
	}
	t.Fatalf("no %s finding in %+v", metric, found)
	return finding{}
}

func noFindingFor(t *testing.T, found []finding, metric string) {
	t.Helper()
	for _, f := range found {
		if f.Metric == metric {
			t.Fatalf("%s raised %+v", metric, f)
		}
	}
}

func absenceFor(t *testing.T, absent []absence, metric string) absence {
	t.Helper()
	for _, a := range absent {
		if a.Metric == metric {
			return a
		}
	}
	t.Fatalf("%s is neither present nor absent: %+v", metric, absent)
	return absence{}
}

func TestSourceCollapseFromSecondBackup(t *testing.T) {
	history := byteRuns(1, anomalyNow-anomalyDay, flatSize(40<<30))
	emptied := mkRun("emptied", anomalyNow, "success", 0, withSource(30<<20, 1000))

	found, absent, learning := detectSource(sourceInput(after(emptied, history)), paramsFor(sensBalanced))
	got := findingFor(t, found, metricSourceBytesShrink)
	if got.Severity != "critical" || got.RunID != "emptied" {
		t.Fatalf("finding = %+v", got)
	}
	if got.Details["collapse"] != true {
		t.Fatalf("details = %v", got.Details)
	}
	if got.Observed != float64(30<<20) || got.Expected != float64(40<<30) {
		t.Fatalf("finding = %+v", got)
	}
	if learning != 1 {
		t.Fatalf("learning = %d, want 1", learning)
	}
	absenceFor(t, absent, metricSourceFilesShrink)

	small := after(mkRun("small", anomalyNow, "success", 0, withSource(1<<20, 1000)),
		byteRuns(1, anomalyNow-anomalyDay, flatSize(40<<20)))
	found, absent = runSource(sourceInput(small))
	noFindingFor(t, found, metricSourceBytesShrink)
	if got := absenceFor(t, absent, metricSourceBytesShrink); got.Observed != float64(1<<20) {
		t.Fatalf("absence = %+v", got)
	}
}

func TestSourceShrinkNeedsSamplesAndZ(t *testing.T) {
	tight := byteRuns(30, anomalyNow-31*anomalyDay, func(i int) int64 {
		return 100<<30 + int64(i%3-1)*(1<<30)
	})

	shrunk := mkRun("shrunk", anomalyNow, "success", 0, withSource(65<<30, 1000))
	found, _ := runSource(sourceInput(after(shrunk, tight)))
	got := findingFor(t, found, metricSourceBytesShrink)
	if got.Severity != "warning" || got.Samples != 30 {
		t.Fatalf("finding = %+v", got)
	}
	if got.Details["collapse"] == true {
		t.Fatalf("a shrink was reported as a collapse: %v", got.Details)
	}

	strong := mkRun("strong", anomalyNow, "success", 0, withSource(45<<30, 1000))
	found, _ = runSource(sourceInput(after(strong, tight)))
	if got := findingFor(t, found, metricSourceBytesShrink); got.Severity != "critical" {
		t.Fatalf("finding = %+v", got)
	}

	noisy := byteRuns(30, anomalyNow-31*anomalyDay, func(i int) int64 {
		return 100<<30 + int64(i%3-1)*(40<<30)
	})
	found, absent := runSource(sourceInput(after(shrunk, noisy)))
	noFindingFor(t, found, metricSourceBytesShrink)
	absenceFor(t, absent, metricSourceBytesShrink)
}

func TestSourceGrowthWarningOnly(t *testing.T) {
	history := byteRuns(30, anomalyNow-31*anomalyDay, flatSize(10<<30))

	grown := mkRun("grown", anomalyNow, "success", 0, withSource(25<<30, 1000))
	found, _ := runSource(sourceInput(after(grown, history)))
	got := findingFor(t, found, metricSourceBytesGrowth)
	if got.Severity != "warning" || got.Observed != float64(25<<30) {
		t.Fatalf("finding = %+v", got)
	}

	huge := mkRun("huge", anomalyNow, "success", 0, withSource(400<<30, 1000))
	found, _ = runSource(sourceInput(after(huge, history)))
	if got := findingFor(t, found, metricSourceBytesGrowth); got.Severity != "warning" {
		t.Fatalf("growth turned critical: %+v", got)
	}

	steady := byteRuns(30, anomalyNow-31*anomalyDay, flatSize(1<<30))
	nudged := mkRun("nudged", anomalyNow, "success", 0, withSource(4<<30, 1000))
	found, absent := runSource(sourceInput(after(nudged, steady)))
	noFindingFor(t, found, metricSourceBytesGrowth)
	absenceFor(t, absent, metricSourceBytesGrowth)
}

func TestSourceConditionUsesFrozenExpected(t *testing.T) {
	settled := byteRuns(11, anomalyNow-11*anomalyDay, flatSize(50<<30))
	in := sourceInput(settled)
	in.Open = map[string]store.Anomaly{
		metricSourceBytesShrink: {
			Detector: detectorSource, Metric: metricSourceBytesShrink,
			Severity: "critical", Expected: float64(100 << 30), Samples: 30,
		},
	}

	found, absent := runSource(in)
	got := findingFor(t, found, metricSourceBytesShrink)
	if got.Severity != "critical" || got.Observed != float64(50<<30) {
		t.Fatalf("the settled baseline cleared the finding: %+v %+v", got, absent)
	}

	in.Series = after(mkRun("back", anomalyNow, "success", 0, withSource(95<<30, 1000)), settled)
	found, absent = runSource(in)
	noFindingFor(t, found, metricSourceBytesShrink)
	if got := absenceFor(t, absent, metricSourceBytesShrink); got.Observed != float64(95<<30) {
		t.Fatalf("absence = %+v", got)
	}
}

func TestSourceDrainOverThirtyDays(t *testing.T) {
	const steps = 21
	sizes := make([]int64, steps)
	sizes[0] = 40 << 30
	for i := 1; i < steps; i++ {
		sizes[i] = sizes[i-1] * 88 / 100
	}
	series := byteRuns(steps, anomalyNow-steps*anomalyDay, func(i int) int64 { return sizes[i] })

	drained := -1
	for i := range sizes {
		if sizes[i]*10 <= sizes[0] {
			drained = i
			break
		}
	}
	if drained < 1 {
		t.Fatalf("the series never falls to a tenth of where it started: %v", sizes)
	}

	// series is newest first, so cutting from the front leaves run i newest.
	upTo := func(i int) []store.SeriesRun { return series[steps-1-i:] }

	for i := 1; i < drained; i++ {
		found, _ := runSource(sourceInput(upTo(i)))
		noFindingFor(t, found, metricSourceBytesShrink)
	}

	found, _ := runSource(sourceInput(upTo(drained)))
	got := findingFor(t, found, metricSourceBytesShrink)
	if got.Severity != "critical" || got.Details["drain"] != true {
		t.Fatalf("finding = %+v", got)
	}
	if got.Details["collapse"] == true {
		t.Fatalf("a drain was reported as a collapse: %v", got.Details)
	}
}

func TestSourceRebaseRelearnsButKeepsCollapse(t *testing.T) {
	history := byteRuns(30, anomalyNow-40*anomalyDay, flatSize(100<<30))
	marked := mkRun("marked", anomalyNow-3*anomalyDay, "success", 0, withSource(50<<30, 1000))
	next := mkRun("next", anomalyNow-2*anomalyDay, "success", 0, withSource(60<<30, 1000))
	third := mkRun("third", anomalyNow-anomalyDay, "success", 0, withSource(45<<30, 1000))
	rows := after(third, after(next, after(marked, history)))

	if found, _ := runSource(sourceInput(rows)); len(found) == 0 {
		t.Fatal("the new level raised nothing even without an expectation")
	}

	in := sourceInput(rows)
	in.Expectations = map[string]store.AnomalyExpectation{
		familySourceBytesDown: {Family: familySourceBytesDown, SinceAt: marked.StartedAt},
	}
	found, _ := runSource(in)
	noFindingFor(t, found, metricSourceBytesShrink)

	in.Series = after(mkRun("gone", anomalyNow, "success", 0, withSource(10<<20, 1000)), rows)
	found, _ = runSource(in)
	got := findingFor(t, found, metricSourceBytesShrink)
	if got.Severity != "critical" || got.Details["collapse"] != true {
		t.Fatalf("a re-base switched the collapse rule off: %+v", got)
	}
}

func TestSelectionChangeRestartsCollapseReference(t *testing.T) {
	excluded := daily(6, anomalyNow-6*anomalyDay, func(i int) store.SeriesRun {
		fp, size := "before", int64(40<<30)
		if i == 5 {
			fp, size = "after", int64(2<<30)
		}
		return mkRun(fmt.Sprintf("r%d", i), 0, "success", 0, withSource(size, 1000), withSelection(fp))
	})

	found, _ := runSource(sourceInput(excluded))
	noFindingFor(t, found, metricSourceBytesShrink)

	gone := mkRun("gone", anomalyNow, "success", 0, withSource(5<<20, 1000), withSelection("after"))
	found, _ = runSource(sourceInput(after(gone, excluded)))
	if got := findingFor(t, found, metricSourceBytesShrink); got.Details["collapse"] != true {
		t.Fatalf("finding = %+v", got)
	}

	vanished := daily(6, anomalyNow-6*anomalyDay, func(i int) store.SeriesRun {
		size := int64(40 << 30)
		if i == 5 {
			size = 2 << 30
		}
		return mkRun(fmt.Sprintf("r%d", i), 0, "success", 0, withSource(size, 1000), withSelection("before"))
	})
	found, _ = runSource(sourceInput(vanished))
	got := findingFor(t, found, metricSourceBytesShrink)
	if got.Severity != "critical" || got.Details["collapse"] != true {
		t.Fatalf("a source that vanished under an unchanged selection raised %+v", got)
	}
}

func TestGrowthExpectationKeepsShrinkActive(t *testing.T) {
	history := byteRuns(30, anomalyNow-31*anomalyDay, flatSize(10<<30))
	grown := mkRun("grown", anomalyNow-3*anomalyDay, "success", 0, withSource(25<<30, 1000))
	rows := after(grown, history)

	if found, _ := runSource(sourceInput(rows)); len(found) == 0 {
		t.Fatal("the growth raised nothing even without an expectation")
	}

	in := sourceInput(rows)
	in.Expectations = map[string]store.AnomalyExpectation{
		familySourceBytesUp: {Family: familySourceBytesUp, SinceAt: grown.StartedAt},
	}
	found, _ := runSource(in)
	noFindingFor(t, found, metricSourceBytesGrowth)

	in.Series = after(mkRun("drop", anomalyNow-2*anomalyDay, "success", 0, withSource(6<<30, 1000)), rows)
	found, _ = runSource(in)
	if got := findingFor(t, found, metricSourceBytesShrink); got.Severity != "warning" {
		t.Fatalf("finding = %+v", got)
	}

	in.Series = after(mkRun("half", anomalyNow-2*anomalyDay, "success", 0, withSource(4<<30, 1000)), rows)
	found, _ = runSource(in)
	if got := findingFor(t, found, metricSourceBytesShrink); got.Severity != "critical" {
		t.Fatalf("finding = %+v", got)
	}
}

func TestFilesCollapseAndShrink(t *testing.T) {
	history := fileRuns(1, anomalyNow-anomalyDay, flatSize(3000))
	emptied := mkRun("emptied", anomalyNow, "success", 0, withSource(10<<30, 12))
	found, _ := runSource(sourceInput(after(emptied, history)))
	got := findingFor(t, found, metricSourceFilesShrink)
	if got.Severity != "critical" || got.Details["collapse"] != true {
		t.Fatalf("finding = %+v", got)
	}

	many := fileRuns(30, anomalyNow-31*anomalyDay, flatSize(50000))
	fewer := mkRun("fewer", anomalyNow, "success", 0, withSource(10<<30, 30000))
	found, _ = runSource(sourceInput(after(fewer, many)))
	if got := findingFor(t, found, metricSourceFilesShrink); got.Severity != "warning" {
		t.Fatalf("finding = %+v", got)
	}

	half := mkRun("half", anomalyNow, "success", 0, withSource(10<<30, 20000))
	found, _ = runSource(sourceInput(after(half, many)))
	if got := findingFor(t, found, metricSourceFilesShrink); got.Severity != "critical" {
		t.Fatalf("finding = %+v", got)
	}

	// The ratio passes both times, and ninety-five files are still too few to
	// mean anything.
	tiny := after(mkRun("tiny", anomalyNow, "success", 0, withSource(10<<30, 5)),
		fileRuns(1, anomalyNow-anomalyDay, flatSize(100)))
	found, absent := runSource(sourceInput(tiny))
	noFindingFor(t, found, metricSourceFilesShrink)
	absenceFor(t, absent, metricSourceFilesShrink)

	// Growth in the file count is not a finding at all.
	more := mkRun("more", anomalyNow, "success", 0, withSource(10<<30, 500000))
	found, absent = runSource(sourceInput(after(more, many)))
	for _, f := range found {
		if f.Metric == "source_files_growth" {
			t.Fatalf("a growing file count raised %+v", f)
		}
	}
	for _, a := range absent {
		if a.Metric == "source_files_growth" {
			t.Fatalf("a file growth metric was evaluated: %+v", a)
		}
	}
}

func TestDumpSeriesUsesDumpFloorsAndNoFileRule(t *testing.T) {
	dumps := func(rows []store.SeriesRun) itemInput {
		in := sourceInput(rows)
		in.Kind = seriesDump
		return in
	}

	history := byteRuns(1, anomalyNow-anomalyDay, flatSize(30<<20))
	shrunk := mkRun("shrunk", anomalyNow, "success", 0, withSource(200<<10, 1))
	in := dumps(after(shrunk, history))
	found, absent := runSource(in)
	got := findingFor(t, found, metricDumpBytesShrink)
	if got.Severity != "critical" || got.Details["collapse"] != true {
		t.Fatalf("finding = %+v", got)
	}
	noFindingFor(t, found, metricSourceBytesShrink)
	noFindingFor(t, found, metricSourceFilesShrink)
	for _, a := range absent {
		if a.Metric == metricSourceFilesShrink || a.Metric == metricSourceBytesShrink {
			t.Fatalf("a dump series evaluated %q", a.Metric)
		}
	}

	small := byteRuns(1, anomalyNow-anomalyDay, flatSize(3<<20))
	found, absent = runSource(dumps(after(mkRun("crumb", anomalyNow, "success", 0, withSource(10<<10, 1)), small)))
	noFindingFor(t, found, metricDumpBytesShrink)
	absenceFor(t, absent, metricDumpBytesShrink)

	if found, _ := detectNewData(in, paramsFor(sensBalanced)); len(found) != 0 {
		t.Fatalf("a dump series raised %+v", found)
	}
}

func TestDurationSlowerOnly(t *testing.T) {
	steady := durationRuns(30, anomalyNow-31*anomalyDay, func(i int) int64 {
		return 60_000 + int64(i%3-1)*3_000
	})

	slow := mkRun("slow", anomalyNow, "success", 0, withSnapshot("snap"), withResticMS(20*60*1000))
	found, _, _ := detectDuration(sourceInput(after(slow, steady)), paramsFor(sensBalanced))
	got := findingFor(t, found, metricDurationSlower)
	if got.Severity != "warning" || got.Samples != 30 {
		t.Fatalf("finding = %+v", got)
	}

	fast := mkRun("fast", anomalyNow, "success", 0, withSnapshot("snap"), withResticMS(1_000))
	found, absent, _ := detectDuration(sourceInput(after(fast, steady)), paramsFor(sensBalanced))
	noFindingFor(t, found, metricDurationSlower)
	absenceFor(t, absent, metricDurationSlower)

	quick := durationRuns(30, anomalyNow-31*anomalyDay, flatSize(2_000))
	found, _, _ = detectDuration(sourceInput(after(
		mkRun("blip", anomalyNow, "success", 0, withSnapshot("snap"), withResticMS(40_000)), quick)),
		paramsFor(sensBalanced))
	noFindingFor(t, found, metricDurationSlower)

	lockWait := mkRun("lock", anomalyNow, "success", 0, withSnapshot("snap"), withResticMS(5*60*1000+30_000))
	in := sourceInput(after(lockWait, steady))
	in.Sens = sensStrict
	found, _, _ = detectDuration(in, paramsFor(in.Sens))
	noFindingFor(t, found, metricDurationSlower)
}

func TestDurationIgnoresNullAndWallClock(t *testing.T) {
	steady := durationRuns(30, anomalyNow-31*anomalyDay, flatSize(60_000))
	unmeasured := daily(5, anomalyNow-5*anomalyDay, func(i int) store.SeriesRun {
		return mkRun(fmt.Sprintf("u%d", i), 0, "success", 0, withSnapshot("snap"))
	})
	history := append(unmeasured, steady...)

	slow := mkRun("slow", anomalyNow, "success", 0, withSnapshot("snap"), withResticMS(20*60*1000))
	found, _, _ := detectDuration(sourceInput(after(slow, history)), paramsFor(sensBalanced))
	if got := findingFor(t, found, metricDurationSlower); got.Samples != 30 {
		t.Fatalf("samples = %d, want the thirty measured runs", got.Samples)
	}

	waited := mkRun("waited", anomalyNow, "success", 0, withSnapshot("snap"), withResticMS(60_000))
	waited.FinishedAt = waited.StartedAt + 3*3600
	found, absent, _ := detectDuration(sourceInput(after(waited, history)), paramsFor(sensBalanced))
	noFindingFor(t, found, metricDurationSlower)
	absenceFor(t, absent, metricDurationSlower)
}

func TestReliabilityStreak(t *testing.T) {
	warn, _ := detectReliability(sourceInput(outcomeRuns("sssssssfff")), paramsFor(sensBalanced))
	got := findingFor(t, warn, metricFailureStreak)
	if got.Severity != "warning" || got.Observed != 3 {
		t.Fatalf("finding = %+v", got)
	}
	if got.Details["streak"] != 3 || got.Details["failed"] != 3 || got.Details["total"] != 10 {
		t.Fatalf("details = %v", got.Details)
	}

	crit, _ := detectReliability(sourceInput(outcomeRuns("ssssffffff")), paramsFor(sensBalanced))
	if got := findingFor(t, crit, metricFailureStreak); got.Severity != "critical" {
		t.Fatalf("finding = %+v", got)
	}

	found, absent := detectReliability(sourceInput(outcomeRuns("ssssfffffs")), paramsFor(sensBalanced))
	noFindingFor(t, found, metricFailureStreak)
	absenceFor(t, absent, metricFailureStreak)

	found, _ = detectReliability(sourceInput(outcomeRuns("ssssfcfkfi")), paramsFor(sensBalanced))
	if got := findingFor(t, found, metricFailureStreak); got.Severity != "warning" || got.Observed != 3 {
		t.Fatalf("a cancelled, skipped or interrupted run broke the streak: %+v", got)
	}
}

func TestReliabilityFlaky(t *testing.T) {
	found, _ := detectReliability(sourceInput(outcomeRuns("sssfsfssfs")), paramsFor(sensBalanced))
	got := findingFor(t, found, metricFlaky)
	if got.Severity != "warning" || got.Observed != 3 {
		t.Fatalf("finding = %+v", got)
	}
	noFindingFor(t, found, metricFailureStreak)

	found, absent := detectReliability(sourceInput(outcomeRuns("ffsf")), paramsFor(sensBalanced))
	noFindingFor(t, found, metricFlaky)
	absenceFor(t, absent, metricFlaky)

	found, absent = detectReliability(sourceInput(outcomeRuns("ssssssffff")), paramsFor(sensBalanced))
	findingFor(t, found, metricFailureStreak)
	noFindingFor(t, found, metricFlaky)
	absenceFor(t, absent, metricFlaky)
}

func TestDumpSeriesHasItsOwnReliabilityMetrics(t *testing.T) {
	in := sourceInput(outcomeRuns("sssssssfff"))
	in.Kind = seriesDump
	found, _ := detectReliability(in, paramsFor(sensBalanced))
	if got := findingFor(t, found, metricDumpFailureStreak); got.Severity != "warning" {
		t.Fatalf("finding = %+v", got)
	}
	noFindingFor(t, found, metricFailureStreak)
}

// drills builds one series of restore checks from a compact outcome string,
// newest first: p a pass, f a failure, s a check that never ran.
func drills(key store.DrillKey, outcomes string) []store.RestoreDrill {
	out := make([]store.RestoreDrill, 0, len(outcomes))
	for i, outcome := range outcomes {
		d := store.RestoreDrill{
			Domain: key.Domain, Source: key.Source, Kind: key.Kind,
			OffsiteTargetID: key.TargetID,
			At:              anomalyNow - int64(i)*anomalyDay,
			OK:              outcome == 'p',
		}
		if outcome == 's' {
			d.Detail = "skipped: repository busy longer than 5m0s"
		}
		out = append(out, d)
	}
	return out
}

func TestIntegrityRegression(t *testing.T) {
	local := store.DrillKey{Domain: "containers", Source: "local", Kind: "subset"}

	found, absent := detectIntegrity(local, drills(local, "fp"))
	got := findingFor(t, found, metricDrillSubset)
	if got.Severity != "critical" || got.Samples != 2 || got.RunAt != anomalyNow {
		t.Fatalf("finding = %+v", got)
	}
	if got.Details["source"] != "local" || got.Details["kind"] != "subset" {
		t.Fatalf("details = %+v", got.Details)
	}
	if len(absent) != 0 {
		t.Fatalf("absent = %+v", absent)
	}

	found, _ = detectIntegrity(local, drills(local, "ffp"))
	if got := findingFor(t, found, metricDrillSubset); got.Samples != 3 {
		t.Fatalf("a second failure after the same pass ended the finding: %+v", got)
	}

	found, _ = detectIntegrity(local, drills(local, "sfp"))
	findingFor(t, found, metricDrillSubset)

	found, absent = detectIntegrity(local, drills(local, "spf"))
	noFindingFor(t, found, metricDrillSubset)
	absenceFor(t, absent, metricDrillSubset)

	found, absent = detectIntegrity(local, drills(local, "pf"))
	noFindingFor(t, found, metricDrillSubset)
	absenceFor(t, absent, metricDrillSubset)

	found, absent = detectIntegrity(local, drills(local, "ff"))
	noFindingFor(t, found, metricDrillSubset)
	absenceFor(t, absent, metricDrillSubset)

	found, absent = detectIntegrity(local, drills(local, "ss"))
	if len(found) != 0 || len(absent) != 0 {
		t.Fatalf("a series of checks that never ran was judged: %+v %+v", found, absent)
	}
}

func TestIntegrityKeepsEveryDrillSeriesApart(t *testing.T) {
	named := store.DrillKey{Domain: "files", Source: "offsite", TargetID: "backblaze", Kind: "subset"}
	found, _ := detectIntegrity(named, drills(named, "fp"))
	if got := findingFor(t, found, metricDrillSubset); got.Details["source"] != "offsite" {
		t.Fatalf("details = %+v", got.Details)
	}

	restore := store.DrillKey{Domain: "files", Source: "offsite", Kind: "dr"}
	found, _ = detectIntegrity(restore, drills(restore, "fp"))
	if got := findingFor(t, found, metricDrillDR); got.Details["kind"] != "dr" {
		t.Fatalf("details = %+v", got.Details)
	}

	unknown := store.DrillKey{Domain: "files", Source: "local", Kind: "smoke"}
	found, absent := detectIntegrity(unknown, drills(unknown, "fp"))
	if len(found) != 0 || len(absent) != 0 {
		t.Fatalf("a check outside the catalogue was judged: %+v %+v", found, absent)
	}
}

// volumeSamples builds n readings a day apart, the oldest at start, oldest
// first as ListVolumeSamples returns them. free(i) is the free space of
// reading i.
func volumeSamples(n int, start int64, total *int64, free func(i int) int64) []store.VolumeSample {
	out := make([]store.VolumeSample, n)
	for i := range n {
		out[i] = store.VolumeSample{
			Volume: "dev:801", At: start + int64(i)*anomalyDay,
			FreeBytes: free(i), TotalBytes: total, Domains: []string{"containers"},
		}
	}
	return out
}

func totalOf(b int64) *int64 { return &b }

func capacityInput(samples []store.VolumeSample) volumeInput {
	return volumeInput{
		Volume: "dev:801", Domains: []string{"containers"},
		Samples: samples, Sens: sensBalanced, Now: anomalyNow,
	}
}

func TestCapacityEtaFromGrowthAndFreeSlope(t *testing.T) {
	const perDay = 10 * gib
	falling := func(free int64) []store.VolumeSample {
		return volumeSamples(20, anomalyNow-20*anomalyDay, totalOf(4<<40), func(i int) int64 {
			return free + int64(19-i)*perDay
		})
	}

	found, _ := detectCapacity(capacityInput(falling(200 * gib)))
	if got := findingFor(t, found, metricCapacityETA); got.Severity != "warning" {
		t.Fatalf("twenty days of room = %+v, want a warning below the 28 day mark", got)
	}

	found, _ = detectCapacity(capacityInput(falling(50 * gib)))
	if got := findingFor(t, found, metricCapacityETA); got.Severity != "critical" {
		t.Fatalf("five days of room = %+v, want critical below the 7 day mark", got)
	}

	rising := volumeSamples(20, anomalyNow-20*anomalyDay, totalOf(4<<40), func(i int) int64 {
		return 200*gib + int64(i)*perDay
	})
	found, absent := detectCapacity(capacityInput(rising))
	noFindingFor(t, found, metricCapacityETA)
	absenceFor(t, absent, metricCapacityETA)

	t.Run("too few readings leave only the growth", func(t *testing.T) {
		short := volumeSamples(4, anomalyNow-4*anomalyDay, nil, func(i int) int64 {
			return 200*gib + int64(3-i)*perDay
		})
		in := capacityInput(short)
		found, absent := detectCapacity(in)
		noFindingFor(t, found, metricCapacityETA)
		absenceFor(t, absent, metricCapacityETA)

		in.Growth = map[string]int64{"containers": 70 * gib} // ten gibibytes a day
		grown, _ := detectCapacity(in)
		if got := findingFor(t, grown, metricCapacityETA).Severity; got != "warning" {
			t.Fatalf("the growth alone gave %q, want a warning at 20 days of room", got)
		}
	})

	t.Run("the nearer of the two wins", func(t *testing.T) {
		in := capacityInput(falling(200 * gib))
		in.Growth = map[string]int64{"containers": 7 * gib} // 200 days, far beyond the slope's 20
		both, _ := detectCapacity(in)
		if got := findingFor(t, both, metricCapacityETA).Observed; math.Abs(got-20) > 0.5 {
			t.Fatalf("eta = %v days, want the slope's 20", got)
		}
	})

	t.Run("the growth reads like the storage forecast", func(t *testing.T) {
		stats := []store.RepoStat{
			{At: anomalyNow - 28*anomalyDay, RawSize: 100 * gib},
			{At: anomalyNow, RawSize: 128 * gib},
		}
		week, ok := growthBytesPerWeek(stats, time.Unix(anomalyNow, 0))
		if !ok {
			t.Fatal("the forecast found no growth to compare against")
		}
		in := capacityInput(volumeSamples(2, anomalyNow-anomalyDay, totalOf(4<<40), func(int) int64 { return 200 * gib }))
		in.Growth = map[string]int64{"containers": week}
		eta, known := capacityETA(in)
		if !known {
			t.Fatal("no projection from the forecast's own growth")
		}
		if want := float64(200*gib) / (float64(week) / 7); math.Abs(eta-want) > 0.001 {
			t.Fatalf("eta = %v days, want %v", eta, want)
		}
	})
}

func TestCapacityWindowResetAndLowFree(t *testing.T) {
	small, large := totalOf(1<<40), totalOf(13<<39) // a thirty per cent bigger disk

	t.Run("a resized volume forgets the readings before it", func(t *testing.T) {
		var samples []store.VolumeSample
		samples = append(samples, volumeSamples(10, anomalyNow-20*anomalyDay, small, func(i int) int64 {
			return 500*gib - int64(i)*10*gib
		})...)
		samples = append(samples, volumeSamples(10, anomalyNow-10*anomalyDay, large, func(int) int64 {
			return 50 * gib
		})...)
		found, absent := detectCapacity(capacityInput(samples))
		noFindingFor(t, found, metricCapacityETA)
		absenceFor(t, absent, metricCapacityETA)
	})

	t.Run("a nearly full volume is critical", func(t *testing.T) {
		samples := volumeSamples(6, anomalyNow-6*anomalyDay, small, func(int) int64 { return (1 << 40) / 25 })
		found, _ := detectCapacity(capacityInput(samples))
		if got := findingFor(t, found, metricCapacityLow); got.Severity != "critical" {
			t.Fatalf("four per cent free = %+v, want critical", got)
		}
	})

	t.Run("a warning holds until the volume is properly free again", func(t *testing.T) {
		open := map[string]store.Anomaly{metricCapacityLow: {Metric: metricCapacityLow, Severity: "warning"}}

		at11 := volumeSamples(6, anomalyNow-6*anomalyDay, small, func(int) int64 { return (1 << 40) * 11 / 100 })
		in := capacityInput(at11)
		in.Open = open
		found, _ := detectCapacity(in)
		if got := findingFor(t, found, metricCapacityLow); got.Severity != "warning" {
			t.Fatalf("eleven per cent free cleared the open warning: %+v", got)
		}

		at13 := volumeSamples(6, anomalyNow-6*anomalyDay, small, func(int) int64 { return (1 << 40) * 13 / 100 })
		in = capacityInput(at13)
		in.Open = open
		found, absent := detectCapacity(in)
		noFindingFor(t, found, metricCapacityLow)
		absenceFor(t, absent, metricCapacityLow)
	})

	t.Run("a backend without a size is only projected", func(t *testing.T) {
		samples := volumeSamples(20, anomalyNow-20*anomalyDay, nil, func(i int) int64 {
			return 50*gib + int64(19-i)*10*gib
		})
		found, absent := detectCapacity(capacityInput(samples))
		findingFor(t, found, metricCapacityETA)
		noFindingFor(t, found, metricCapacityLow)
		for _, a := range absent {
			if a.Metric == metricCapacityLow {
				t.Fatal("a volume of unknown size reported its free share as fine")
			}
		}
	})
}
