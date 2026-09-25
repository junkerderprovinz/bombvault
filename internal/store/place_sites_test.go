package store_test

import (
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

func writeSitePlace(t *testing.T, r *store.Repo, p store.Place) store.Place {
	t.Helper()
	p.Enabled = true
	saved, err := r.WritePlace(store.PlaceWrite{Place: p})
	if err != nil {
		t.Fatalf("WritePlace %s: %v", p.Name, err)
	}
	return saved
}

func TestPlaceSitesAnswerByThePlaceAndNotByTheRow(t *testing.T) {
	r, db := placesRepo(t)
	away := writeSitePlace(t, r, store.Place{
		Name: "B2", Provider: "b2", Kind: "s3", Base: "s3:https://s3.example.com/bv",
		Folders: map[string]string{"containers": "containers"}, OffPremises: true,
	})
	here := writeSitePlace(t, r, store.Place{
		Name: "NAS Keller", Provider: "synology", Kind: "local", Base: "remotes/nas/bv",
		Folders: map[string]string{"vms": "vms"},
	})
	b2 := store.SeedOffsiteTarget(t, r, "containers", "s3:https://s3.example.com/bv/containers")
	// The NAS is the home of vms and holds its copies too, beside the domain path.
	nas := store.SeedOffsiteTarget(t, r, "vms", "remotes/nas/bv/vms-copies")
	loose := store.SeedOffsiteTarget(t, r, "files", "b2:bucket:files")

	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	must := func(err error) {
		t.Helper()
		if err != nil {
			_ = tx.Rollback()
			t.Fatal(err)
		}
	}
	must(store.AttachRowTx(tx, b2.ID, away.ID, "containers", ""))
	must(store.AttachRowTx(tx, nas.ID, here.ID, "vms", "-copies"))
	must(r.SetDomainPlaceTx(tx, "vms", here.ID))
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}

	if stored, _, err := r.GetOffsiteTarget(b2.ID); err != nil || stored.OffPremises {
		t.Fatalf("the target row carries a mark of its own: %+v, %v", stored, err)
	}
	sites, err := r.PlaceSites()
	if err != nil {
		t.Fatal(err)
	}
	if !sites.Row(b2.ID, false) {
		t.Error("a target at a place off the premises stands off the premises, whatever its row says")
	}
	if sites.Row(nas.ID, true) {
		t.Error("a target at a place in the house stands in the house")
	}
	if !sites.Row(loose.ID, true) || sites.Row(loose.ID, false) {
		t.Error("a row without a place takes the caller's reading")
	}
	if sites.Domain("vms", true) {
		t.Error("a domain path whose home place is in the house stands in the house")
	}
	if !sites.Domain("containers", true) || sites.Domain("containers", false) {
		t.Error("a domain path without a home place takes the caller's reading")
	}

	here.OffPremises = true
	if _, err := r.WritePlace(store.PlaceWrite{Place: here}); err != nil {
		t.Fatal(err)
	}
	if sites, err = r.PlaceSites(); err != nil || !sites.Row(nas.ID, false) || !sites.Domain("vms", false) {
		t.Fatalf("after the answer changed at the place: %+v, %v", sites, err)
	}
}
