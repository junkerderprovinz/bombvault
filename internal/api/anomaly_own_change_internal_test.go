package api

import (
	"fmt"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

func TestFilesBombVaultRemovedItselfAreNoDataLoss(t *testing.T) {
	// Twelve backups of a folder that holds a kept data folder of 1275 files
	// next to 725 of its own, then the backup after the kept folder went.
	history := fileRuns(12, anomalyNow-13*anomalyDay, flatSize(2000))
	removedAt := anomalyNow - anomalyDay/2
	next := mkRun("next", anomalyNow, "success", 0, withSource(10<<30, 725))
	rows := after(next, history)

	found, _ := runSource(sourceInput(rows))
	if got := findingFor(t, found, metricSourceFilesShrink); got.Severity != "critical" {
		t.Fatalf("without the record of the removal the shrink raised %+v", got)
	}

	in := sourceInput(rows)
	in.ChangedAt = removedAt
	found, _ = runSource(in)
	noFindingFor(t, found, metricSourceFilesShrink)

	// The backup after that is watched again, against the level the removal left.
	emptied := mkRun("emptied", anomalyNow+anomalyDay, "success", 0, withSource(10<<30, 20))
	in.Series = after(emptied, rows)
	found, _ = runSource(in)
	if got := findingFor(t, found, metricSourceFilesShrink); got.RunID != "emptied" || got.Expected != 725 {
		t.Fatalf("finding = %+v, want the emptied run held against the 725 files after the removal", got)
	}
}

func TestDataBombVaultAddedItselfIsNoGrowth(t *testing.T) {
	history := daily(12, anomalyNow-13*anomalyDay, func(i int) store.SeriesRun {
		return mkRun(fmt.Sprintf("r%d", i), 0, "success", 200<<20, withParent(true), withSnapshot("snap"), withSource(10<<30, 1000))
	})
	keptAt := anomalyNow - anomalyDay/2
	next := mkRun("next", anomalyNow, "success", 20<<30, withParent(true), withSnapshot("snap"), withSource(30<<30, 3000))
	rows := after(next, history)

	plain := itemInput{Kind: seriesItem, Series: rows, NewData: rows, Sens: sensBalanced, EvaluateFrom: history[0].StartedAt}
	found := evaluateItem(plain).Findings
	findingFor(t, found, metricSourceBytesGrowth)
	findingFor(t, found, metricNewData)

	changed := plain
	changed.ChangedAt = keptAt
	if found := evaluateItem(changed).Findings; len(found) != 0 {
		t.Fatalf("the backup after the kept folder appeared raised %+v", found)
	}
}

func TestABackupRunningWhileBombVaultChangedTheSourceIsNotJudged(t *testing.T) {
	history := fileRuns(12, anomalyNow-13*anomalyDay, flatSize(2000))
	running := mkRun("running", anomalyNow-anomalyDay/2, "success", 0, withSource(10<<30, 725))
	running.FinishedAt = anomalyNow - anomalyDay/4

	in := sourceInput(after(running, history))
	in.ChangedAt = anomalyNow - anomalyDay/3
	found, _ := runSource(in)
	noFindingFor(t, found, metricSourceFilesShrink)
}
