package store_test

import (
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

// testedAndAged leaves a tamper verdict and an aging mark on the target.
func testedAndAged(t *testing.T, r *store.Repo, target store.OffsiteTarget) {
	t.Helper()
	if err := r.RecordTamperTestForTarget(target.Domain, target.ID, true, ""); err != nil {
		t.Fatal(err)
	}
	if err := r.RecordTargetListing(target.Domain, target.ID, 100, nil); err != nil {
		t.Fatal(err)
	}
	if err := r.MarkTargetAged(target.Domain, target.ID, "rev", 200); err != nil {
		t.Fatal(err)
	}
}

// remembers reports whether the target still has a tamper verdict and an
// aging mark.
func remembers(t *testing.T, r *store.Repo, target store.OffsiteTarget) (verdict, aged bool) {
	t.Helper()
	_, verdict, err := r.LatestTamperTestForTarget(target.Domain, target.ID)
	if err != nil {
		t.Fatal(err)
	}
	o, _, err := r.TargetObservationFor(target.Domain, target.ID)
	if err != nil {
		t.Fatal(err)
	}
	return verdict, o.AgedAt != 0
}

func TestATargetThatChangesPlaceForgetsItsTamperVerdictAndAgingMark(t *testing.T) {
	r := newRepo(t)
	target, err := r.UpsertOffsiteTarget(store.OffsiteTarget{Domain: "vms", Name: "A", Repo: "rest:https://a:8000/vms", Immutable: true, Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	testedAndAged(t, r, target)

	target.Name = "A renamed"
	if _, err := r.UpsertOffsiteTarget(target); err != nil {
		t.Fatal(err)
	}
	if verdict, aged := remembers(t, r, target); !verdict || !aged {
		t.Fatalf("after a save in place verdict=%v aged=%v, want both kept", verdict, aged)
	}

	target.Repo = "rest:https://b:8000/vms"
	if _, err := r.UpsertOffsiteTarget(target); err != nil {
		t.Fatal(err)
	}
	if verdict, aged := remembers(t, r, target); verdict || aged {
		t.Fatalf("after the move verdict=%v aged=%v, want both gone", verdict, aged)
	}
}

func TestAPrimaryTakenOverByALocationForgetsItsTamperVerdict(t *testing.T) {
	r := newRepo(t)
	primary, err := r.UpsertOffsiteTarget(store.OffsiteTarget{Domain: "vms", Name: "A", Repo: "rest:https://a:8000/vms", Immutable: true, Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	testedAndAged(t, r, primary)
	d, err := r.SaveDestination(store.OffsiteTarget{Name: "Box", Repo: "rest:https://box:8000/bv", Provider: "box", Immutable: true})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := r.PrimaryFromDestination(d.ID, "vms", d.Repo+"/vms", func(*store.Settings) {}); err != nil {
		t.Fatal(err)
	}

	if verdict, aged := remembers(t, r, primary); verdict || aged {
		t.Fatalf("after the takeover verdict=%v aged=%v, want both gone", verdict, aged)
	}
}
