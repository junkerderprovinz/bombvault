package api

import (
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

func TestAnImportKeepsWhatWasObservedAtAnUnchangedTarget(t *testing.T) {
	f := newPlacementFixture(t)
	b2 := f.target("files", "B2", "b2:bucket:files")
	hz := f.target("files", "Hetzner", "sftp:u@box:/files")
	f.listing("files", b2.ID, 500, copiesRow("fileset:Photos", 20, 400))
	f.listing("files", hz.ID, 600, copiesRow("fileset:Photos", 5, 400))
	if err := f.st.MarkTargetAged("files", b2.ID, "rev-1", 700); err != nil {
		t.Fatal(err)
	}
	moved := offsiteTargetToView(hz)
	moved.Repo = "sftp:u@other:/files"

	if err := f.h.replaceOffsiteTargets([]offsiteTargetView{offsiteTargetToView(b2), moved}, settingsView{}); err != nil {
		t.Fatal(err)
	}

	o, listed, err := f.st.TargetObservationFor("files", b2.ID)
	if err != nil || !listed || o.ListedAt != 500 || o.RulesRev != "rev-1" || o.AgedAt != 700 {
		t.Fatalf("B2 after the import = %+v listed=%v err=%v, want its listing and aging kept", o, listed, err)
	}
	rows, err := f.st.ItemCopiesFor("files", "fileset:Photos")
	if err != nil || len(rows) != 1 || rows[0].TargetID != b2.ID || rows[0].SnapshotCount != 20 {
		t.Fatalf("copies after the import = %+v, %v, want B2's 20 only", rows, err)
	}
	if _, listed, err := f.st.TargetObservationFor("files", hz.ID); err != nil || listed {
		t.Fatalf("Hetzner moved and kept its observation (listed=%v err=%v)", listed, err)
	}
}

func TestRestoreObservationsSkipsAFailureAndKeepsGoing(t *testing.T) {
	f := newPlacementFixture(t)
	b2 := f.target("files", "B2", "b2:bucket:files")

	kept := []keptObservation{
		{obs: store.TargetObservation{Domain: "files", TargetID: "", ListedAt: 500}},
		{
			obs:    store.TargetObservation{Domain: "files", TargetID: b2.ID, ListedAt: 600},
			copies: []store.ItemCopies{copiesRow("fileset:Photos", 3, 400)},
		},
	}
	f.svc.restoreObservations(kept)

	if _, listed, err := f.st.TargetObservationFor("files", b2.ID); err != nil || !listed {
		t.Fatalf("the observation after a failed one = listed=%v err=%v, want it restored anyway", listed, err)
	}
}
