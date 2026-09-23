package store_test

import (
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

func TestRestoreDrillsRoundTrip(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	r := store.New(db)

	if _, found, err := r.LatestRestoreDrill("containers", "local"); err != nil {
		t.Fatalf("LatestRestoreDrill (empty): %v", err)
	} else if found {
		t.Fatal("expected found=false on an empty store")
	}

	drills := []store.RestoreDrill{
		{Domain: "containers", Source: "local", At: 100, OK: true},
		{Domain: "containers", Source: "local", At: 200, OK: true},
		{Domain: "containers", Source: "local", At: 300, OK: false, Detail: "data corruption"},
	}
	for _, d := range drills {
		if err := r.AddRestoreDrill(d); err != nil {
			t.Fatalf("AddRestoreDrill: %v", err)
		}
	}

	latest, found, err := r.LatestRestoreDrill("containers", "local")
	if err != nil {
		t.Fatalf("LatestRestoreDrill: %v", err)
	}
	if !found {
		t.Fatal("expected found=true after adding drills")
	}
	if latest.At != 300 || latest.OK || latest.Detail != "data corruption" {
		t.Fatalf("latest = %+v, want at=300 ok=false detail='data corruption'", latest)
	}

	list, err := r.ListRestoreDrills("containers", "local", 0)
	if err != nil {
		t.Fatalf("ListRestoreDrills: %v", err)
	}
	if len(list) != 3 {
		t.Fatalf("expected 3 drills, got %d", len(list))
	}
	for i := 1; i < len(list); i++ {
		if list[i].At >= list[i-1].At {
			t.Fatalf("drills not descending by at: %+v", list)
		}
	}

	if _, found, err := r.LatestRestoreDrill("vms", "local"); err != nil {
		t.Fatalf("LatestRestoreDrill (other domain): %v", err)
	} else if found {
		t.Fatal("a different domain must not see containers drills")
	}
}

func TestListRestoreDrillsKindReadsOnlyThatKind(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	r := store.New(db)

	for i := range 2 {
		d := store.RestoreDrill{Domain: "containers", Source: "offsite", Kind: "dr", At: int64(100 + i), OK: true}
		if err := r.AddRestoreDrill(d); err != nil {
			t.Fatalf("AddRestoreDrill dr: %v", err)
		}
	}
	for i := range 25 {
		d := store.RestoreDrill{Domain: "containers", Source: "offsite", Kind: "subset", At: int64(1000 + i), OK: true}
		if err := r.AddRestoreDrill(d); err != nil {
			t.Fatalf("AddRestoreDrill subset: %v", err)
		}
	}
	named := store.RestoreDrill{
		Domain: "containers", Source: "offsite", Kind: "dr", At: 2000, OK: false,
		Detail: "restore came back short", OffsiteTargetID: "wasabi",
	}
	if err := r.AddRestoreDrill(named); err != nil {
		t.Fatalf("AddRestoreDrill named: %v", err)
	}

	drills, err := r.ListRestoreDrillsKind("containers", "offsite", "", "dr", 20)
	if err != nil {
		t.Fatalf("ListRestoreDrillsKind: %v", err)
	}
	if len(drills) != 2 {
		t.Fatalf("got %d dr drills, want the 2 that are not the named target's", len(drills))
	}
	for _, d := range drills {
		if d.Kind != "dr" || d.OffsiteTargetID != "" {
			t.Fatalf("read a foreign row: %+v", d)
		}
	}

	onTarget, err := r.ListRestoreDrillsKind("containers", "offsite", "wasabi", "dr", 20)
	if err != nil {
		t.Fatalf("ListRestoreDrillsKind named: %v", err)
	}
	if len(onTarget) != 1 || onTarget[0].Detail != "restore came back short" {
		t.Fatalf("the named target's drill is not keyed to it: %+v", onTarget)
	}

	keys, err := r.ListRestoreDrillKeys()
	if err != nil {
		t.Fatalf("ListRestoreDrillKeys: %v", err)
	}
	want := map[store.DrillKey]bool{
		{Domain: "containers", Source: "offsite", Kind: "dr"}:                     true,
		{Domain: "containers", Source: "offsite", Kind: "subset"}:                 true,
		{Domain: "containers", Source: "offsite", TargetID: "wasabi", Kind: "dr"}: true,
	}
	if len(keys) != len(want) {
		t.Fatalf("got %d keys, want %d: %+v", len(keys), len(want), keys)
	}
	for _, k := range keys {
		if !want[k] {
			t.Fatalf("unexpected key %+v", k)
		}
	}
}
