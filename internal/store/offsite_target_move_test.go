package store_test

import (
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

func TestAMovedTargetForgetsWhatItsOldPlaceSaid(t *testing.T) {
	r := newRepo(t)
	tgt, err := r.UpsertOffsiteTarget(store.OffsiteTarget{Domain: "containers", Name: "Box", Repo: "rest:http://box:8000/c", Enabled: true, SortOrder: 1})
	if err != nil {
		t.Fatal(err)
	}
	if err := r.RecordTamperTestForTarget("containers", tgt.ID, true, ""); err != nil {
		t.Fatal(err)
	}
	run, err := r.RecordOffsiteRunForTarget("containers", tgt.ID, 100)
	if err != nil {
		t.Fatal(err)
	}
	if err := r.FinishOffsiteRun(run, true, ""); err != nil {
		t.Fatal(err)
	}
	store.SeedListing(t, r, "containers", tgt.ID, 100, store.CopiesOf("container:nginx", 3, 90))

	tgt.Repo = "s3:https://s3.example.com/fresh/c"
	if _, err := r.MoveOffsiteTarget(tgt); err != nil {
		t.Fatal(err)
	}

	if _, found, _ := r.LatestTamperTestForTarget("containers", tgt.ID); found {
		t.Error("the old place's tamper verdict stayed with the target")
	}
	if _, found, _ := r.LatestSuccessfulOffsiteRunForTarget("containers", tgt.ID); found {
		t.Error("a copy to the old place still counts as the last copy")
	}
	if _, found, _ := r.LatestSuccessfulOffsiteRun("containers"); found {
		t.Error("a copy to the old place still counts for the domain")
	}
	if copies, _ := r.ItemCopiesForDomain("containers"); len(copies) != 0 {
		t.Errorf("copies = %+v, listed at the old place", copies)
	}
	if _, found, _ := r.TargetObservationFor("containers", tgt.ID); found {
		t.Error("the old place's listing stayed")
	}
	if _, copied, err := r.DomainHasHistory("containers"); err != nil || !copied {
		t.Errorf("copied = %v, %v: the domain has to stay replicated, or a first-listing pause starts", copied, err)
	}
}

func TestAnEditThatKeepsThePlaceKeepsItsHistory(t *testing.T) {
	r := newRepo(t)
	tgt, err := r.UpsertOffsiteTarget(store.OffsiteTarget{Domain: "containers", Name: "Box", Repo: "rest:http://box:8000/c", Enabled: true, SortOrder: 1})
	if err != nil {
		t.Fatal(err)
	}
	if err := r.RecordTamperTestForTarget("containers", tgt.ID, true, ""); err != nil {
		t.Fatal(err)
	}
	tgt.Name = "Box upstairs"
	if _, err := r.UpsertOffsiteTarget(tgt); err != nil {
		t.Fatal(err)
	}
	if _, found, _ := r.LatestTamperTestForTarget("containers", tgt.ID); !found {
		t.Error("renaming a target dropped its tamper verdict")
	}
}
