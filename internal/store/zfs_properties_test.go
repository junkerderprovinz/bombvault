package store_test

import (
	"reflect"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

func TestZFSPropertiesAreFoundByTheirSnapshot(t *testing.T) {
	_, r := zfsStore(t)
	d := aZFSDataset(t, r, "cache/appdata")
	other := aZFSDataset(t, r, "cache/other")
	for _, item := range []store.ZFSDataset{d, other} {
		runID, err := r.StartRun(item.ID, "backup")
		if err != nil {
			t.Fatal(err)
		}
		if err := r.RecordZFSRun(runID, item.ID, "bombvault-20260927120000", -1, ""); err != nil {
			t.Fatal(err)
		}
		for _, m := range []store.ZFSRunMember{
			{RunID: runID, Dataset: item.Dataset, Outcome: "backed-up", ResticSnapshot: item.Dataset + "-snap"},
			{RunID: runID, Dataset: item.Dataset + "/old", Outcome: "backed-up", ResticSnapshot: item.Dataset + "-old"},
		} {
			if err := r.AddZFSRunMember(m); err != nil {
				t.Fatal(err)
			}
		}
		if err := r.SetZFSRunMemberProperties(runID, item.Dataset, map[string]string{"compression": "zstd", "item": item.Dataset}); err != nil {
			t.Fatal(err)
		}
	}

	got, err := r.ZFSPropertiesOfItem(d.ID)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]map[string]string{"cache/appdata-snap": {"compression": "zstd", "item": "cache/appdata"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want only the item's own member that has properties", got)
	}
}
