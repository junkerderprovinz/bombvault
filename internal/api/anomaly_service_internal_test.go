package api

import (
	"context"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

// The service API is what HTTP and, later, the MCP tools sit on: it turns the
// rows the engine wrote into sentences the frontend can build, and it is the
// only way a user closes an episode or changes an item's settings.

func (f *engineFixture) collapsedContainer(t *testing.T, name string) (string, store.Anomaly) {
	t.Helper()
	id := f.container(t, name)
	f.steadySeries(t, id, "backup", 12, 40*gib)
	f.run(t, id, "backup", f.now-60, 20*mib)
	f.pass(t)
	return id, onlyRow(t, f.openRows(t))
}

func TestListAnomaliesCarriesTheNameAndWhatCanBeExpected(t *testing.T) {
	f := newEngineFixture(t)
	id, row := f.collapsedContainer(t, "nextcloud")

	page, err := f.svc.ListAnomalies(context.Background(), store.AnomalyFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Anomalies) != 1 {
		t.Fatalf("want one view, got %d", len(page.Anomalies))
	}
	view := page.Anomalies[0]
	if view.ID != row.ID || view.Name != "nextcloud" || view.TargetID != id {
		t.Fatalf("view does not name the item: %+v", view)
	}
	if !view.Expectable {
		t.Fatal("a source shrink can be marked as expected")
	}
	if !view.RetentionHeld {
		t.Fatal("an open critical collapse holds retention")
	}
	if view.Details["collapse"] != true {
		t.Fatalf("details lost the collapse flag: %v", view.Details)
	}

	one, found, err := f.svc.GetAnomaly(context.Background(), row.ID)
	if err != nil || !found || one.ID != row.ID {
		t.Fatalf("GetAnomaly = %+v %v %v", one, found, err)
	}
	if _, found, err = f.svc.GetAnomaly(context.Background(), "no-such-row"); err != nil || found {
		t.Fatalf("an unknown id = %v %v", found, err)
	}
}

func TestAcknowledgeCountsWhatItCouldNotClose(t *testing.T) {
	f := newEngineFixture(t)
	_, row := f.collapsedContainer(t, "nextcloud")

	changed, skipped, released, err := f.svc.AcknowledgeAnomalies(context.Background(),
		[]string{row.ID, row.ID, "no-such-row"}, "planned clean-up")
	if err != nil {
		t.Fatal(err)
	}
	if changed != 1 || skipped != 1 || released != 1 {
		t.Fatalf("changed = %d skipped = %d released = %d, want 1, 1 and 1", changed, skipped, released)
	}
	after, _, err := f.st.GetAnomaly(row.ID)
	if err != nil {
		t.Fatal(err)
	}
	if after.State != "acknowledged" || after.AckNote != "planned clean-up" {
		t.Fatalf("row = %s %q", after.State, after.AckNote)
	}

	changed, skipped, released, err = f.svc.AcknowledgeAnomalies(context.Background(), []string{row.ID}, "")
	if err != nil || changed != 0 || skipped != 1 || released != 0 {
		t.Fatalf("a settled row = %d %d %d %v", changed, skipped, released, err)
	}
}

func TestMarkExpectedRecordsWhatTheSeriesMayDoNow(t *testing.T) {
	f := newEngineFixture(t)
	id, row := f.collapsedContainer(t, "nextcloud")

	changed, skipped, _, err := f.svc.MarkAnomaliesExpected(context.Background(), []string{row.ID}, "moved to a share")
	if err != nil || changed != 1 || skipped != 0 {
		t.Fatalf("mark expected = %d %d %v", changed, skipped, err)
	}
	exps, err := f.st.ListAnomalyExpectationsForTarget(id)
	if err != nil {
		t.Fatal(err)
	}
	if len(exps) != 1 {
		t.Fatalf("want one expectation, got %+v", exps)
	}
	if exps[0].Family != familySourceBytesDown || exps[0].SinceAt != row.LastRunAt {
		t.Fatalf("expectation = %+v, want the shrink direction from the flagged run", exps[0])
	}

	items, err := f.svc.AnomalyItems(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	item := itemByID(t, items, id)
	if len(item.Expectations) != 1 || item.Expectations[0].Family != familySourceBytesDown {
		t.Fatalf("the item does not list its expectation: %+v", item.Expectations)
	}

	if err := f.svc.ForgetAnomalyExpectation(context.Background(), id, anomalyScopeItem, "", familySourceBytesDown); err != nil {
		t.Fatal(err)
	}
	items, err = f.svc.AnomalyItems(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if left := itemByID(t, items, id).Expectations; len(left) != 0 {
		t.Fatalf("the expectation was not forgotten: %+v", left)
	}
}

// Reliability findings are about runs that failed, which announce themselves,
// so there is nothing to declare expected about them.
func TestMarkExpectedSkipsAMetricNobodyCanExpect(t *testing.T) {
	f := newEngineFixture(t)
	id := f.container(t, "nextcloud")
	for i := 3; i >= 1; i-- {
		runID, err := f.st.StartRun(id, "backup")
		if err != nil {
			t.Fatal(err)
		}
		if err := f.st.FinishRun(runID, "failed", "", 0, "restic exited"); err != nil {
			t.Fatal(err)
		}
		f.backdate(t, runID, f.now-int64(i)*86400)
	}
	f.pass(t)
	row := onlyRow(t, f.openRows(t))
	if row.Metric != metricFailureStreak {
		t.Fatalf("want a failure streak, got %s", row.Metric)
	}

	changed, skipped, _, err := f.svc.MarkAnomaliesExpected(context.Background(), []string{row.ID}, "")
	if err != nil || changed != 0 || skipped != 1 {
		t.Fatalf("mark expected = %d %d %v", changed, skipped, err)
	}
	if after, _, gErr := f.st.GetAnomaly(row.ID); gErr != nil || after.State != "open" {
		t.Fatalf("the row must stay open, got %v %v", after.State, gErr)
	}
}

func TestAnomalyItemsCoversScheduledItemsAndRecentHistory(t *testing.T) {
	f := newEngineFixture(t)
	scheduled := f.container(t, "nextcloud")
	f.steadySeries(t, scheduled, "backup", 12, 40*gib)

	excluded, err := f.st.UpsertTarget(store.Target{ContainerName: "plex", IncludeInSchedule: false})
	if err != nil {
		t.Fatal(err)
	}
	f.steadySeries(t, excluded.ID, "backup", 3, 10*gib)
	quiet, err := f.st.UpsertTarget(store.Target{ContainerName: "sonarr", IncludeInSchedule: false})
	if err != nil {
		t.Fatal(err)
	}

	if f.e.summary().Ready {
		t.Fatal("nothing is ready before the first full pass")
	}
	f.e.MarkAllDirty()
	f.pass(t)
	if !f.e.summary().Ready {
		t.Fatal("a full pass has computed the learning figures")
	}

	items, err := f.svc.AnomalyItems(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	listed := map[string]AnomalyItem{}
	for _, item := range items {
		listed[item.TargetID] = item
	}
	if _, ok := listed[quiet.ID]; ok {
		t.Fatal("an item with no schedule and no history is not listed")
	}
	if item, ok := listed[excluded.ID]; !ok || item.Scheduled {
		t.Fatalf("an excluded item with history is listed as not scheduled: %+v", item)
	}
	if listed[excluded.ID].Learning.Samples != 0 {
		t.Fatalf("an unscheduled item has no learning state: %+v", listed[excluded.ID].Learning)
	}
	item := listed[scheduled]
	if !item.Scheduled || item.Domain != "container" || item.Name != "nextcloud" {
		t.Fatalf("scheduled item = %+v", item)
	}
	if item.Learning.Samples != item.Learning.Needed {
		t.Fatalf("twelve runs are enough to have learned: %+v", item.Learning)
	}
	if item.Typical.SourceBytes == nil || *item.Typical.SourceBytes != 40*gib {
		t.Fatalf("typical source = %v, want 40 GiB", item.Typical.SourceBytes)
	}
	if item.Effective != string(sensBalanced) {
		t.Fatalf("effective sensitivity = %q", item.Effective)
	}
	if sum := f.e.summary(); sum.LearningItems != 0 {
		t.Fatalf("learningItems = %d, want 0", sum.LearningItems)
	}
}

func TestAnomalyItemsCountsAContainerStillLearning(t *testing.T) {
	f := newEngineFixture(t)
	id := f.container(t, "nextcloud")
	f.steadySeries(t, id, "backup", 4, 40*gib)

	f.e.MarkAllDirty()
	f.pass(t)

	item := itemByID(t, mustItems(t, f), id)
	if item.Learning.Samples == item.Learning.Needed {
		t.Fatalf("four runs are not a baseline: %+v", item.Learning)
	}
	if item.Typical.SourceBytes != nil {
		t.Fatal("no typical value while the item is still learning")
	}
	if sum := f.e.summary(); sum.LearningItems != 1 {
		t.Fatalf("learningItems = %d, want 1", sum.LearningItems)
	}
}

func TestAnomalyItemsCarriesTheDumpSeriesOfItsOwn(t *testing.T) {
	f := newEngineFixture(t)
	id := f.container(t, "nextcloud")
	for i := 12; i >= 1; i-- {
		f.run(t, id, "backup", f.now-int64(i)*86400, 40*gib)
		f.run(t, id, "dbdump", f.now-int64(i)*86400+600, 300*mib)
	}

	f.e.MarkAllDirty()
	f.pass(t)

	item := itemByID(t, mustItems(t, f), id)
	if item.Dump == nil {
		t.Fatal("a container with dumps carries its dump series")
	}
	if item.Dump.Typical.SourceBytes == nil || *item.Dump.Typical.SourceBytes != 300*mib {
		t.Fatalf("typical dump size = %v", item.Dump.Typical.SourceBytes)
	}
	if len(item.Datasets) != 0 {
		t.Fatalf("no dataset series without ZFS items: %+v", item.Datasets)
	}
}

func TestSetItemPrefsRefusesAnythingButAnItemAndAKnownSetting(t *testing.T) {
	f := newEngineFixture(t)
	id := f.container(t, "nextcloud")
	f.steadySeries(t, id, "backup", 12, 40*gib)
	f.e.MarkAllDirty()
	f.pass(t)

	ctx := context.Background()
	if err := f.svc.SetItemAnomalyPrefs(ctx, "containers", prefsPatch("strict", "")); err == nil {
		t.Fatal("a domain literal is not an item")
	}
	if err := f.svc.SetItemAnomalyPrefs(ctx, id, prefsPatch("paranoid", "")); err == nil {
		t.Fatal("an unknown preset must be refused")
	}
	if err := f.svc.SetItemAnomalyPrefs(ctx, id, prefsPatch("", "loud")); err == nil {
		t.Fatal("an unknown notification minimum must be refused")
	}

	if err := f.svc.SetItemAnomalyPrefs(ctx, id, prefsPatch("strict", "warning")); err != nil {
		t.Fatal(err)
	}
	item := itemByID(t, mustItems(t, f), id)
	if item.Sensitivity != "strict" || item.Effective != "strict" {
		t.Fatalf("sensitivity = %q/%q", item.Sensitivity, item.Effective)
	}
	if item.NotifyMin != "warning" || item.EffectiveNotifyMin != "warning" {
		t.Fatalf("notify minimum = %q/%q", item.NotifyMin, item.EffectiveNotifyMin)
	}
}

func mustItems(t *testing.T, f *engineFixture) []AnomalyItem {
	t.Helper()
	items, err := f.svc.AnomalyItems(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return items
}

func itemByID(t *testing.T, items []AnomalyItem, targetID string) AnomalyItem {
	t.Helper()
	for _, item := range items {
		if item.TargetID == targetID {
			return item
		}
	}
	t.Fatalf("item %s is not listed", targetID)
	return AnomalyItem{}
}
