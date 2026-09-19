package store_test

import (
	"errors"
	"slices"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

func TestAListingReplacesWhatTheTargetHeld(t *testing.T) {
	r := newRepo(t)
	store.SeedListing(t, r, "containers", "t1", 100,
		store.CopiesOf("container:nginx", 3, 90), store.CopiesOf("container:plex", 2, 80), store.CopiesOf("container:gone", 0, 0))
	store.SeedListing(t, r, "vms", "t1", 100, store.CopiesOf("vm:win11", 1, 70))
	if err := r.MarkTargetAged("containers", "t1", "rev-1", 110); err != nil {
		t.Fatal(err)
	}

	store.SeedListing(t, r, "containers", "t1", 200, store.CopiesOf("container:nginx", 4, 190))
	want := []store.ItemCopies{{Domain: "containers", Identity: "container:nginx", TargetID: "t1", SnapshotCount: 4, LatestSnapshotAt: 190, ObservedAt: 200}}
	if got, err := r.ItemCopiesForDomain("containers"); err != nil || !slices.Equal(got, want) {
		t.Fatalf("ItemCopiesForDomain = %+v, %v, want %+v", got, err, want)
	}
	obs, found, err := r.TargetObservationFor("containers", "t1")
	if err != nil || !found || obs.ListedAt != 200 || obs.RulesRev != "rev-1" || obs.AgedAt != 110 {
		t.Fatalf("observation = %+v found=%v err=%v, want the listing time moved and the aging kept", obs, found, err)
	}
	if vms, _ := r.ItemCopiesForDomain("vms"); len(vms) != 1 {
		t.Fatalf("another domain's rows = %v, want them untouched", vms)
	}

	store.SeedListing(t, r, "containers", "t1", 300)
	if got, _ := r.ItemCopiesForDomain("containers"); len(got) != 0 {
		t.Fatalf("after an empty listing: %v, want the target empty", got)
	}
	if obs, _, _ := r.TargetObservationFor("containers", "t1"); obs.ListedAt != 300 {
		t.Fatalf("listed_at = %d, want 300", obs.ListedAt)
	}
}

func TestNeverListedIsNotListedEmpty(t *testing.T) {
	r := newRepo(t)
	if _, found, err := r.TargetObservationFor("files", "t1"); found || err != nil {
		t.Fatalf("before any listing: found=%v err=%v", found, err)
	}
	store.SeedListing(t, r, "files", "t1", 100)
	if _, found, _ := r.TargetObservationFor("files", "t1"); !found {
		t.Fatal("an empty listing left no observation")
	}
	all, err := r.TargetObservationsForDomain("files")
	if err != nil || len(all) != 1 || all["t1"].ListedAt != 100 {
		t.Fatalf("TargetObservationsForDomain = %v, %v", all, err)
	}
	if none, err := r.TargetObservationsForDomain("vms"); err != nil || none == nil || len(none) != 0 {
		t.Fatalf("a domain never listed = %#v, %v, want an empty map", none, err)
	}
}

func TestAListingRefusesTheSynthesizedTargetAndARepeatedName(t *testing.T) {
	r := newRepo(t)
	if err := r.RecordTargetListing("containers", "", 1, nil); !errors.Is(err, store.ErrNoTargetID) {
		t.Fatalf("listing without a target id = %v, want ErrNoTargetID", err)
	}
	err := r.RecordTargetListing("containers", "t1", 1, []store.ItemCopies{
		store.CopiesOf("container:nginx", 1, 1), store.CopiesOf("container:nginx", 2, 2),
	})
	if err == nil {
		t.Fatal("a listing naming nginx twice was accepted")
	}
	if _, found, _ := r.TargetObservationFor("containers", "t1"); found {
		t.Fatal("a refused listing left an observation")
	}
}

func TestAdjustingCopiesKeepsTheListingTime(t *testing.T) {
	r := newRepo(t)
	store.SeedListing(t, r, "containers", "t1", 100, store.CopiesOf("container:nginx", 20, 90), store.CopiesOf("container:plex", 3, 80))
	if err := r.AdjustItemCopies("containers", "t1", 150, []store.ItemCopies{
		store.CopiesOf("container:nginx", 7, 90), store.CopiesOf("container:plex", 0, 0),
	}); err != nil {
		t.Fatal(err)
	}
	got, err := r.ItemCopiesFor("containers", "container:nginx")
	if err != nil || len(got) != 1 || got[0].SnapshotCount != 7 || got[0].ObservedAt != 150 {
		t.Fatalf("nginx = %+v, %v, want 7 seen at 150", got, err)
	}
	if plex, err := r.ItemCopiesFor("containers", "container:plex"); err != nil || plex == nil || len(plex) != 0 {
		t.Fatalf("plex = %#v, %v, want an empty list", plex, err)
	}
	if obs, _, _ := r.TargetObservationFor("containers", "t1"); obs.ListedAt != 100 {
		t.Fatalf("listed_at = %d, want it kept at 100", obs.ListedAt)
	}
}

func TestAgingNeedsAListingAndCanBeReset(t *testing.T) {
	r := newRepo(t)
	if err := r.MarkTargetAged("containers", "t1", "rev", 10); err == nil {
		t.Fatal("aging a target never listed was accepted")
	}
	store.SeedListing(t, r, "containers", "t1", 5)
	if err := r.MarkTargetAged("containers", "t1", "rev", 10); err != nil {
		t.Fatal(err)
	}
	if err := r.ResetTargetAged("containers", "t1"); err != nil {
		t.Fatal(err)
	}
	if obs, _, _ := r.TargetObservationFor("containers", "t1"); obs.RulesRev != "" || obs.AgedAt != 0 || obs.ListedAt != 5 {
		t.Fatalf("after the reset: %+v", obs)
	}
}

func TestDeletingATargetDropsWhatWasObservedThere(t *testing.T) {
	r := newRepo(t)
	b2 := store.SeedOffsiteTarget(t, r, "containers", "b2:bucket:containers")
	box := store.SeedOffsiteTarget(t, r, "containers", "sftp:u@box:/containers")
	store.SeedListing(t, r, "containers", b2.ID, 100, store.CopiesOf("container:nginx", 3, 90))
	store.SeedListing(t, r, "containers", box.ID, 100, store.CopiesOf("container:nginx", 1, 90))
	if err := r.DeleteOffsiteTarget(b2.ID); err != nil {
		t.Fatal(err)
	}
	if _, found, _ := r.TargetObservationFor("containers", b2.ID); found {
		t.Error("the deleted target kept its observation")
	}
	if got, _ := r.ItemCopiesForDomain("containers"); len(got) != 1 || got[0].TargetID != box.ID {
		t.Errorf("rows after the delete = %+v, want only the other target's", got)
	}
}
