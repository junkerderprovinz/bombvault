package api

import (
	"fmt"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

// memberRows is one dataset's history as DatasetSeries returns it, newest
// first: outcome i belongs to run i of the item, a day apart, the oldest at
// start.
func memberRows(start int64, sourceBytes int64, outcomes ...string) []store.SeriesRun {
	return daily(len(outcomes), start, func(i int) store.SeriesRun {
		run := mkRun(fmt.Sprintf("run%d", i), 0, "success", 1<<20, withSelection("fp"))
		run.Outcome = outcomes[i]
		switch outcomes[i] {
		case "backed-up":
			run.SnapshotID = fmt.Sprintf("snap%d", i)
			withSource(sourceBytes, 1000)(&run)
			withParent(true)(&run)
			withResticMS(60_000)(&run)
		case "empty":
			withSource(0, 0)(&run)
		default:
			run.Bytes = 0
		}
		return run
	})
}

// itemRuns are the runs of the item that backs the dataset up, newest first,
// each under the fingerprint fps gives it.
func itemRuns(start int64, fps ...string) []store.SeriesRun {
	return daily(len(fps), start, func(i int) store.SeriesRun {
		return mkRun(fmt.Sprintf("run%d", i), 0, "success", 0, withSelection(fps[i]))
	})
}

func datasetInput(series []store.SeriesRun) itemInput {
	return itemInput{Kind: seriesDataset, Domain: zfsDomain, Series: series, NewData: series, Sens: sensBalanced}
}

func TestZFSMissingDatasetIsAMeasuredZero(t *testing.T) {
	start := anomalyNow - 3*anomalyDay
	members := memberRows(start, 40<<30, "backed-up", "backed-up", "backed-up")

	series := datasetRuns(members, itemRuns(start, "fp", "fp", "fp", "fp"))
	res := evaluateItem(datasetInput(series))
	got := findingFor(t, res.Findings, metricSourceBytesShrink)
	if got.Severity != "critical" || got.Details["collapse"] != true || got.Observed != 0 {
		t.Fatalf("a dataset that vanished from its tree = %+v, want a collapse to zero", got)
	}
	if got.RunID != "run3" || got.LastGoodRunID != "run2" {
		t.Fatalf("finding = %+v, want it on the run that missed the dataset", got)
	}

	series = datasetRuns(members, itemRuns(start, "fp", "fp", "fp", "fp-without-it"))
	res = evaluateItem(datasetInput(series))
	noFindingFor(t, res.Findings, metricSourceBytesShrink)
}

func TestZFSSkippedDatasetCollapsesLikeAMissingOne(t *testing.T) {
	start := anomalyNow - 4*anomalyDay
	for _, code := range []string{"key-not-loaded", "not-mounted", "canmount-off"} {
		members := memberRows(start, 40<<30, "backed-up", "backed-up", "backed-up", code)
		res := evaluateItem(datasetInput(datasetRuns(members, itemRuns(start, "fp", "fp", "fp", "fp"))))
		if got := findingFor(t, res.Findings, metricSourceBytesShrink); got.Severity != "critical" {
			t.Fatalf("%s after backups = %+v, want a critical collapse", code, got)
		}
	}

	excluded := memberRows(start, 40<<30, "backed-up", "backed-up", "backed-up", "excluded")
	res := evaluateItem(datasetInput(datasetRuns(excluded, itemRuns(start, "fp", "fp", "fp", "fp"))))
	noFindingFor(t, res.Findings, metricSourceBytesShrink)

	failed := memberRows(start, 40<<30, "backed-up", "backed-up", "backed-up", "backup-failed")
	res = evaluateItem(datasetInput(datasetRuns(failed, itemRuns(start, "fp", "fp", "fp", "fp"))))
	noFindingFor(t, res.Findings, metricSourceBytesShrink)
}

// A dataset that stays unreadable is one loss, not a new one every night: the
// first run without it measures the zero, and the ones after it keep that
// zero as the newest value instead of adding more.
func TestZFSDatasetSkippedAgainKeepsTheSameZero(t *testing.T) {
	start := anomalyNow - 5*anomalyDay
	members := memberRows(start, 40<<30, "backed-up", "backed-up", "backed-up", "not-mounted", "not-mounted")
	series := datasetRuns(members, itemRuns(start, "fp", "fp", "fp", "fp", "fp"))

	eligible := eligibleRuns(series)
	if len(eligible) != 4 {
		t.Fatalf("eligible = %+v, want the three backups and one zero", eligible)
	}
	got := findingFor(t, evaluateItem(datasetInput(series)).Findings, metricSourceBytesShrink)
	if got.RunID != "run3" {
		t.Fatalf("finding = %+v, want it on the first run that could not read the dataset", got)
	}
}

func TestZFSEmptyDatasetIsAMeasuredZero(t *testing.T) {
	start := anomalyNow - 3*anomalyDay
	members := memberRows(start, 40<<30, "backed-up", "backed-up", "empty")
	got := findingFor(t, evaluateItem(datasetInput(datasetRuns(members, nil))).Findings, metricSourceBytesShrink)
	if got.Severity != "critical" || got.Observed != 0 {
		t.Fatalf("an emptied dataset = %+v, want a collapse", got)
	}
}

func TestZFSTreeIsJudgedByItsDatasetsAndItsRunsByTheItem(t *testing.T) {
	collapsed := after(mkRun("emptied", anomalyNow, "success", 0, withSource(0, 0)),
		byteRuns(3, anomalyNow-3*anomalyDay, flatSize(40<<30)))
	tree := evaluateItem(itemInput{Kind: seriesTree, Domain: zfsDomain, Series: collapsed, NewData: collapsed, Sens: sensBalanced})
	for _, f := range tree.Findings {
		if f.Metric != metricFailureStreak && f.Metric != metricFlaky {
			t.Fatalf("the tree raised %s, which belongs to its datasets", f.Metric)
		}
	}

	failing := outcomeRuns("sssfff")
	tree = evaluateItem(itemInput{Kind: seriesTree, Domain: zfsDomain, Series: failing, Sens: sensBalanced})
	findingFor(t, tree.Findings, metricFailureStreak)

	dataset := evaluateItem(datasetInput(failing))
	noFindingFor(t, dataset.Findings, metricFailureStreak)
	for _, a := range dataset.Absent {
		if a.Metric == metricFailureStreak || a.Metric == metricFlaky {
			t.Fatalf("a dataset judged %s, which is its tree's", a.Metric)
		}
	}
}
