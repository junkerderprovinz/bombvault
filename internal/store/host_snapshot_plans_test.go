package store_test

import (
	"errors"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

func TestHostSnapshotPlansKeepWhatTheyTakeAndWhatTheyDid(t *testing.T) {
	r := newRepo(t)
	p, err := r.AddHostSnapshotPlan(store.HostSnapshotPlan{Kind: "zfs", Source: "tank/apps", Cadence: "daily 03:00", Keep: 7, SkipAsleep: true, Enabled: true, CreatedAt: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(p.ID) != 32 {
		t.Fatalf("id %q", p.ID)
	}
	p.Keep, p.Cadence, p.Source = 3, "weekly Sun 04:00", "tank/other"
	if err := r.UpdateHostSnapshotPlan(p); err != nil {
		t.Fatal(err)
	}
	if err := r.RecordHostSnapshotRun(p.ID, 99, "busy", "dataset is busy"); err != nil {
		t.Fatal(err)
	}
	got, err := r.GetHostSnapshotPlan(p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Keep != 3 || got.Cadence != "weekly Sun 04:00" || got.Source != "tank/apps" || !got.SkipAsleep || got.LastRunAt != 99 || got.LastCode != "busy" {
		t.Errorf("plan = %+v, want the edit applied, the source unchanged and the run recorded", got)
	}
	if err := r.DeleteHostSnapshotPlan(p.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := r.GetHostSnapshotPlan(p.ID); !errors.Is(err, store.ErrHostSnapshotPlanNotFound) {
		t.Errorf("after delete: %v", err)
	}
	list, err := r.ListHostSnapshotPlans()
	if err != nil || len(list) != 0 {
		t.Errorf("list = %v, %v", list, err)
	}
}
