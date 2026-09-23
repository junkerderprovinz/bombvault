package api

import (
	"encoding/json"
	"fmt"
	"math"
	"testing"

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
