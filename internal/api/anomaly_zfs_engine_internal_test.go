package api

import (
	"slices"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

// A ZFS tree is backed up as one run and many datasets, and a child emptied
// inside a large tree barely moves the tree's total. So every dataset is a
// series of its own, keyed by its name, and a finding about it holds only that
// dataset's backups.

// member is what one run did to one dataset. A source of -1 leaves the member
// unmeasured, the way a skip leaves it.
type member struct {
	dataset, outcome string
	source           int64
}

func backedUp(dataset string, source int64) member { return member{dataset, outcomeBackedUp, source} }

func (f *engineFixture) zfsItem(t *testing.T, root string) string {
	t.Helper()
	settings, err := f.st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	if !settings.ZFSEnabled {
		settings.ZFSEnabled = true
		if err := f.st.UpdateSettings(settings); err != nil {
			t.Fatal(err)
		}
	}
	d, err := f.st.CreateZFSDataset(store.ZFSDataset{Dataset: root, Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	return d.ID
}

// zfsRun records one finished run of a tree the way a backup does: every
// member first, then the run itself with the sum and the tree's fingerprint.
func (f *engineFixture) zfsRun(t *testing.T, itemID string, at int64, fp string, members ...member) string {
	t.Helper()
	id, err := f.st.StartRun(itemID, "backup")
	if err != nil {
		t.Fatal(err)
	}
	total := store.RunMetrics{ResticMS: 60_000}
	for _, m := range members {
		row := store.ZFSRunMember{RunID: id, Dataset: m.dataset, Outcome: m.outcome, DurationMS: 60_000}
		if m.source >= 0 && (m.outcome == outcomeBackedUp || m.outcome == zfsOutcomeEmpty) {
			source, files, parent := m.source, int64(seriesFiles), true
			row.SourceBytes, row.SourceFiles, row.HasParent = &source, &files, &parent
			if m.outcome == outcomeBackedUp {
				row.ResticSnapshot = "snap-" + m.dataset + "-" + id
			}
			total.SourceBytes += source
			total.SourceFiles += files
		}
		if err := f.st.AddZFSRunMember(row); err != nil {
			t.Fatal(err)
		}
	}
	if err := f.st.FinishRunMeasured(id, "success", "snap-"+id, 0, "", &total, fp); err != nil {
		t.Fatal(err)
	}
	f.backdate(t, id, at)
	return id
}

// steadyTree gives a tree n daily runs in which every dataset keeps its size,
// the newest a day before the fixture's clock.
func (f *engineFixture) steadyTree(t *testing.T, itemID string, n int, members ...member) {
	t.Helper()
	for i := n; i >= 1; i-- {
		f.zfsRun(t, itemID, f.now-int64(i)*86400, "tree-1", members...)
	}
}

func openFor(rows []store.Anomaly, scopeKind, scopeID string) []store.Anomaly {
	var out []store.Anomaly
	for _, row := range rows {
		if row.ScopeKind == scopeKind && row.ScopeID == scopeID && row.State == "open" {
			out = append(out, row)
		}
	}
	return out
}

func TestZFSDatasetsAreSeriesOfTheirOwn(t *testing.T) {
	f := newEngineFixture(t)
	item := f.zfsItem(t, "tank/a")
	f.steadyTree(t, item, 5, backedUp("tank/a", 1000*gib), backedUp("tank/a/child", 20*gib))

	f.zfsRun(t, item, f.now-3600, "tree-1", backedUp("tank/a", 1000*gib), backedUp("tank/a/child", 30*mib))
	f.pass(t)

	rows := f.openRows(t)
	child := onlyRow(t, openFor(rows, anomalyScopeZFSDS, "tank/a/child"))
	if child.Metric != metricSourceBytesShrink || child.Severity != "critical" {
		t.Fatalf("child finding = %+v, want a critical collapse", child)
	}
	if child.TargetID != item || child.Domain != zfsDomain {
		t.Fatalf("child finding belongs to %q in %q, want the item %q in zfs", child.TargetID, child.Domain, item)
	}
	if others := openFor(rows, anomalyScopeZFSDS, "tank/a"); len(others) != 0 {
		t.Fatalf("the root raised %+v", others)
	}
	if items := openFor(rows, anomalyScopeItem, item); len(items) != 0 {
		t.Fatalf("the tree raised %+v, which is its dataset's", items)
	}

	held, err := f.e.HeldIdentityTags()
	if err != nil {
		t.Fatal(err)
	}
	if !held.holds("zfs:tank/a/child") || held.holds("zfs:tank/a") {
		t.Fatalf("held = %v, want only the collapsed dataset", held.names())
	}

	// The tree is taken over by a new item under a new root. The dataset keeps
	// its name, so it keeps its history and its finding.
	renamed := f.zfsItem(t, "tank")
	f.zfsRun(t, renamed, f.now-60, "tree-2", backedUp("tank", 1*gib), backedUp("tank/a", 1000*gib), backedUp("tank/a/child", 30*mib))
	f.pass(t)
	still := onlyRow(t, openFor(f.openRows(t), anomalyScopeZFSDS, "tank/a/child"))
	if still.ID != child.ID || still.Occurrences < 2 {
		t.Fatalf("after the new root the finding is %+v, want the same row seen again", still)
	}
}

func TestZFSUnreadableDatasetHoldsLikeAnEmptiedOne(t *testing.T) {
	cases := []struct {
		name  string
		fp    string
		child member
		held  bool
	}{
		{"key not loaded", "tree-1", member{"tank/a/child", "key-not-loaded", -1}, true},
		{"gone from the tree", "tree-1", member{}, true},
		{"excluded by the user", "tree-2", member{"tank/a/child", "excluded", -1}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newEngineFixture(t)
			item := f.zfsItem(t, "tank/a")
			f.steadyTree(t, item, 5, backedUp("tank/a", 1000*gib), backedUp("tank/a/child", 20*gib))
			members := []member{backedUp("tank/a", 1000*gib)}
			if tc.child.dataset != "" {
				members = append(members, tc.child)
			}
			f.zfsRun(t, item, f.now-3600, tc.fp, members...)
			f.pass(t)

			raised := openFor(f.openRows(t), anomalyScopeZFSDS, "tank/a/child")
			if got := len(raised) == 1 && raised[0].Metric == metricSourceBytesShrink; got != tc.held {
				t.Fatalf("findings = %+v, want a collapse: %v", raised, tc.held)
			}
			held, err := f.e.HeldIdentityTags()
			if err != nil {
				t.Fatal(err)
			}
			if held.holds("zfs:tank/a/child") != tc.held {
				t.Fatalf("held = %v", held.names())
			}
		})
	}
}

func TestZFSRetentionHeldAsksTheDataset(t *testing.T) {
	f := newEngineFixture(t)
	item := f.zfsItem(t, "tank/a")
	f.steadyTree(t, item, 5, backedUp("tank/a", 1000*gib), backedUp("tank/a/child", 20*gib))
	f.zfsRun(t, item, f.now-3600, "tree-1", backedUp("tank/a", 1000*gib), backedUp("tank/a/child", 30*mib))

	held, why, err := f.e.RetentionHeld(t.Context(), anomalyScope{Kind: anomalyScopeZFSDS, ID: "tank/a/child"})
	if err != nil || !held || why == "" {
		t.Fatalf("the emptied dataset: held %v (%q), %v", held, why, err)
	}
	held, _, err = f.e.RetentionHeld(t.Context(), anomalyScope{Kind: anomalyScopeZFSDS, ID: "tank/a"})
	if err != nil || held {
		t.Fatalf("its sibling: held %v, %v", held, err)
	}
}

func TestZFSItemListsItsDatasets(t *testing.T) {
	f := newEngineFixture(t)
	item := f.zfsItem(t, "tank/a")
	f.steadyTree(t, item, 12, backedUp("tank/a", 100*gib), backedUp("tank/a/child", 20*gib))
	f.zfsRun(t, item, f.now-3600, "tree-1", backedUp("tank/a", 100*gib), backedUp("tank/a/child", 30*mib))
	f.e.MarkAllDirty()
	f.pass(t)

	views := f.e.itemViews()
	i := slices.IndexFunc(views, func(v AnomalyItem) bool { return v.TargetID == item })
	if i < 0 {
		t.Fatalf("the ZFS item is not listed: %+v", views)
	}
	view := views[i]
	if view.Domain != zfsDomain || view.Name != "tank/a" || !view.Scheduled {
		t.Fatalf("item = %+v", view)
	}
	if len(view.Datasets) != 2 || view.Datasets[0].Part != "tank/a" || view.Datasets[1].Part != "tank/a/child" {
		t.Fatalf("datasets = %+v, want both, by name", view.Datasets)
	}
	root, child := view.Datasets[0], view.Datasets[1]
	if root.Learning.Samples != anomalyMinSamples || root.Typical.SourceBytes == nil || *root.Typical.SourceBytes != 100*gib {
		t.Fatalf("root = %+v, want it learned at its usual size", root)
	}
	if child.Open.Critical != 1 || !child.RetentionHeld {
		t.Fatalf("child = %+v, want its collapse counted and holding", child)
	}
	if !view.RetentionHeld || view.Open.Critical != 0 {
		t.Fatalf("item = %+v, want the hold shown and the finding left to the dataset row", view)
	}
	if view.Learning.Samples != anomalyMinSamples {
		t.Fatalf("item learning = %+v, want what its datasets learned", view.Learning)
	}
}
