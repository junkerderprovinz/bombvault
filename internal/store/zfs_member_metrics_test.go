package store_test

import (
	"database/sql"
	"reflect"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

// memberRun writes one finished run of a ZFS item straight into the table, so a
// test can choose its start and its selection fingerprint.
func memberRun(t *testing.T, db *sql.DB, id, itemID, status string, startedAt int64, fp string) {
	t.Helper()
	var selection any
	if fp != "" {
		selection = fp
	}
	_, err := db.Exec(`
		INSERT INTO runs (id, target_id, kind, status, started_at, finished_at, bytes, selection_fp)
		VALUES (?, ?, 'backup', ?, ?, ?, 0, ?)`,
		id, itemID, status, startedAt, startedAt+60, selection)
	if err != nil {
		t.Fatalf("insert run %s: %v", id, err)
	}
}

func measuredMember(runID, dataset string, sourceBytes int64) store.ZFSRunMember {
	parent := true
	files := int64(40)
	return store.ZFSRunMember{
		RunID: runID, Dataset: dataset, Outcome: "backed-up", ResticSnapshot: "snap-" + runID,
		BytesAdded: 512, FilesNew: 2, DurationMS: 1500,
		SourceBytes: &sourceBytes, SourceFiles: &files, HasParent: &parent,
	}
}

func addMember(t *testing.T, r *store.Repo, m store.ZFSRunMember) {
	t.Helper()
	if err := r.AddZFSRunMember(m); err != nil {
		t.Fatalf("AddZFSRunMember %s/%s: %v", m.RunID, m.Dataset, err)
	}
}

func TestZFSRunMemberCarriesItsSourceMetrics(t *testing.T) {
	db, r := zfsStore(t)
	d := aZFSDataset(t, r, "tank/media")
	memberRun(t, db, "run", d.ID, "success", 100, "fp")

	addMember(t, r, measuredMember("run", "tank/media", 4096))
	addMember(t, r, store.ZFSRunMember{RunID: "run", Dataset: "tank/media/locked", Outcome: "key-not-loaded"})

	got, err := r.ListZFSRunMembers("run")
	if err != nil {
		t.Fatalf("ListZFSRunMembers: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("members = %+v", got)
	}
	read := got[0]
	if read.SourceBytes == nil || *read.SourceBytes != 4096 || read.SourceFiles == nil || *read.SourceFiles != 40 {
		t.Fatalf("the measured member came back as %+v", read)
	}
	if read.HasParent == nil || !*read.HasParent {
		t.Fatalf("the parent flag came back as %v", read.HasParent)
	}
	skipped := got[1]
	if skipped.SourceBytes != nil || skipped.SourceFiles != nil || skipped.HasParent != nil {
		t.Fatalf("a skipped member came back measured: %+v", skipped)
	}
}

func TestDatasetSeriesJoinsEachMemberToItsRun(t *testing.T) {
	db, r := zfsStore(t)
	old := aZFSDataset(t, r, "tank/a")
	renamed := aZFSDataset(t, r, "tank")

	memberRun(t, db, "first", old.ID, "success", 100, "fp-1")
	memberRun(t, db, "second", old.ID, "failed", 200, "fp-1")
	memberRun(t, db, "running", old.ID, "running", 250, "fp-1")
	memberRun(t, db, "under-new-root", renamed.ID, "success", 300, "fp-2")
	memberRun(t, db, "future", renamed.ID, "success", 900, "fp-2")

	addMember(t, r, measuredMember("first", "tank/a/child", 8<<30))
	addMember(t, r, store.ZFSRunMember{RunID: "second", Dataset: "tank/a/child", Outcome: "key-not-loaded"})
	addMember(t, r, measuredMember("running", "tank/a/child", 1))
	addMember(t, r, measuredMember("under-new-root", "tank/a/child", 7<<30))
	addMember(t, r, measuredMember("future", "tank/a/child", 1))
	addMember(t, r, measuredMember("first", "tank/a", 1<<30))

	series, err := r.DatasetSeries("tank/a/child", 500, 90)
	if err != nil {
		t.Fatalf("DatasetSeries: %v", err)
	}
	var ids []string
	for _, run := range series {
		ids = append(ids, run.ID)
	}
	if want := []string{"under-new-root", "second", "first"}; !reflect.DeepEqual(ids, want) {
		t.Fatalf("DatasetSeries = %v, want %v", ids, want)
	}

	newest := series[0]
	if newest.Status != "success" || newest.StartedAt != 300 || newest.Outcome != "backed-up" {
		t.Fatalf("the run columns did not travel: %+v", newest)
	}
	if newest.SnapshotID != "snap-under-new-root" || newest.Bytes != 512 {
		t.Fatalf("the member's snapshot and new data did not travel: %+v", newest)
	}
	if newest.SourceBytes == nil || *newest.SourceBytes != 7<<30 || newest.SourceFiles == nil || *newest.SourceFiles != 40 {
		t.Fatalf("the member's source figures did not travel: %+v", newest)
	}
	if newest.ResticMS == nil || *newest.ResticMS != 1500 || newest.HasParent == nil || *newest.HasParent != 1 {
		t.Fatalf("the member's duration or parent flag did not travel: %+v", newest)
	}
	if newest.SelectionFP == nil || *newest.SelectionFP != "fp-2" {
		t.Fatalf("the run's selection fingerprint did not travel: %v", newest.SelectionFP)
	}

	skipped := series[1]
	if skipped.Status != "failed" || skipped.Outcome != "key-not-loaded" {
		t.Fatalf("the skipped member = %+v", skipped)
	}
	if skipped.SourceBytes != nil || skipped.FilesNew != nil {
		t.Fatalf("a member restic never read came back measured: %+v", skipped)
	}

	limited, err := r.DatasetSeries("tank/a/child", 500, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(limited) != 1 || limited[0].ID != "under-new-root" {
		t.Fatalf("limit ignored: %+v", limited)
	}
}

func TestZFSDatasetOwnersFollowTheNewestRun(t *testing.T) {
	db, r := zfsStore(t)
	old := aZFSDataset(t, r, "tank/a")
	renamed := aZFSDataset(t, r, "tank")

	memberRun(t, db, "old-run", old.ID, "success", 100, "")
	memberRun(t, db, "new-run", renamed.ID, "success", 200, "")
	addMember(t, r, measuredMember("old-run", "tank/a", 1))
	addMember(t, r, measuredMember("old-run", "tank/a/child", 1))
	addMember(t, r, measuredMember("new-run", "tank/a/child", 1))
	addMember(t, r, measuredMember("new-run", "tank", 1))

	owners, err := r.ZFSDatasetOwners()
	if err != nil {
		t.Fatalf("ZFSDatasetOwners: %v", err)
	}
	want := map[string]string{"tank/a": old.ID, "tank/a/child": renamed.ID, "tank": renamed.ID}
	if !reflect.DeepEqual(owners, want) {
		t.Fatalf("ZFSDatasetOwners = %v, want %v", owners, want)
	}
}

func TestUnmeasuredSnapshotRunsListsTheDatasetsOfAZFSRun(t *testing.T) {
	db, r := zfsStore(t)
	d := aZFSDataset(t, r, "tank")
	memberRun(t, db, "zfs-run", d.ID, "success", 100, "")
	if _, err := db.Exec(`UPDATE runs SET snapshot_id = 'root-snap' WHERE id = 'zfs-run'`); err != nil {
		t.Fatal(err)
	}
	addMember(t, r, store.ZFSRunMember{RunID: "zfs-run", Dataset: "tank", Outcome: "backed-up", ResticSnapshot: "root-snap"})
	addMember(t, r, store.ZFSRunMember{RunID: "zfs-run", Dataset: "tank/media", Outcome: "backed-up", ResticSnapshot: "media-snap"})
	addMember(t, r, measuredMember("zfs-run", "tank/done", 1))
	addMember(t, r, store.ZFSRunMember{RunID: "zfs-run", Dataset: "tank/locked", Outcome: "key-not-loaded"})

	runs, err := r.UnmeasuredSnapshotRuns()
	if err != nil {
		t.Fatalf("UnmeasuredSnapshotRuns: %v", err)
	}
	want := []store.UnmeasuredRun{
		{ID: "zfs-run", TargetID: d.ID, Kind: "backup", SnapshotID: "root-snap", StartedAt: 100, Dataset: "tank"},
		{ID: "zfs-run", TargetID: d.ID, Kind: "backup", SnapshotID: "media-snap", StartedAt: 100, Dataset: "tank/media"},
	}
	if !reflect.DeepEqual(runs, want) {
		t.Fatalf("UnmeasuredSnapshotRuns = %+v, want only the unmeasured datasets and not the item's own run: %+v", runs, want)
	}
}

func TestSetZFSMemberMetricsNeverOverwritesLiveMeasurement(t *testing.T) {
	db, r := zfsStore(t)
	d := aZFSDataset(t, r, "tank")
	memberRun(t, db, "run", d.ID, "success", 100, "")
	addMember(t, r, measuredMember("run", "tank", 4096))
	addMember(t, r, store.ZFSRunMember{RunID: "run", Dataset: "tank/media", Outcome: "backed-up", ResticSnapshot: "s2"})

	parent := false
	n, err := r.SetZFSMemberMetrics(map[store.ZFSMemberRef]store.RunMetrics{
		{RunID: "run", Dataset: "tank"}:       {SourceBytes: 1, SourceFiles: 1},
		{RunID: "run", Dataset: "tank/media"}: {SourceBytes: 2048, SourceFiles: 5, HasParent: &parent},
		{RunID: "run", Dataset: "tank/gone"}:  {SourceBytes: 3},
	})
	if err != nil {
		t.Fatalf("SetZFSMemberMetrics: %v", err)
	}
	if n != 1 {
		t.Fatalf("SetZFSMemberMetrics set %d rows, want 1", n)
	}

	members, err := r.ListZFSRunMembers("run")
	if err != nil {
		t.Fatal(err)
	}
	byName := map[string]store.ZFSRunMember{}
	for _, m := range members {
		byName[m.Dataset] = m
	}
	if live := byName["tank"]; live.SourceBytes == nil || *live.SourceBytes != 4096 {
		t.Fatalf("the live measurement was overwritten: %+v", live)
	}
	filled := byName["tank/media"]
	if filled.SourceBytes == nil || *filled.SourceBytes != 2048 || filled.SourceFiles == nil || *filled.SourceFiles != 5 {
		t.Fatalf("the unmeasured member was not filled: %+v", filled)
	}
	if filled.HasParent == nil || *filled.HasParent {
		t.Fatalf("the parent flag = %v, want a recorded false", filled.HasParent)
	}
}
