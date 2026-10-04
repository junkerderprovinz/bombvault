package store_test

import (
	"errors"
	"slices"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

func saveDestination(t *testing.T, r *store.Repo, d store.OffsiteTarget) store.OffsiteTarget {
	t.Helper()
	saved, err := r.SaveDestination(d)
	if err != nil {
		t.Fatal(err)
	}
	return saved
}

func TestADestinationIsInvisibleToReplication(t *testing.T) {
	r := newRepo(t)
	saveDestination(t, r, store.OffsiteTarget{Name: "B2", Repo: "s3:https://s3.example.com/bucket/bv", Provider: "b2"})

	all, err := r.ListOffsiteTargets()
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 0 {
		t.Fatalf("ListOffsiteTargets = %+v, want no replication targets", all)
	}
	list, err := r.ListDestinations()
	if err != nil || len(list) != 1 || list[0].Provider != "b2" || list[0].Domain != "" {
		t.Fatalf("ListDestinations = %+v, %v, want the one destination without a domain", list, err)
	}
}

func TestADomainTargetIsDerivedOnceAndNeverTakesTheFieldSlot(t *testing.T) {
	r := newRepo(t)
	d := saveDestination(t, r, store.OffsiteTarget{Name: "B2", Repo: "s3:https://s3.example.com/bucket/bv", CredsRef: "c1", Immutable: true})

	first, created, err := r.EnsureDestinationTarget(d.ID, "vms", "s3:https://s3.example.com/bucket/bv/vms")
	if err != nil || !created {
		t.Fatalf("first ensure: created=%v err=%v", created, err)
	}
	if first.SortOrder == 0 {
		t.Fatal("a derived target took sort order 0, the slot of the settings field")
	}
	if first.DestinationID != d.ID || first.CredsRef != "c1" || !first.Immutable || first.Name != "B2" {
		t.Fatalf("derived target = %+v, want the destination's fields", first)
	}
	again, created, err := r.EnsureDestinationTarget(d.ID, "vms", "s3:https://elsewhere/vms")
	if err != nil || created || again.ID != first.ID || again.Repo != first.Repo {
		t.Fatalf("second ensure = %+v created=%v err=%v, want the same target untouched", again, created, err)
	}
}

func TestANewDomainTargetStartsUnticked(t *testing.T) {
	r := newRepo(t)
	if _, err := r.PutPlacementDefault("containers", "", []string{"old"}); err != nil {
		t.Fatal(err)
	}
	store.SeedCopyRule(t, r, "containers", "container:nginx")
	store.SeedCopyRule(t, r, "containers", "container:plex", store.SkipAll)
	d := saveDestination(t, r, store.OffsiteTarget{Name: "B2", Repo: "rclone:b2:bv"})

	tgt, _, err := r.EnsureDestinationTarget(d.ID, "containers", "rclone:b2:bv/containers")
	if err != nil {
		t.Fatal(err)
	}
	def, _, err := r.PlacementDefaultFor("containers")
	if err != nil || len(def.Skip) != 2 || !slices.Contains(def.Skip, "old") || !slices.Contains(def.Skip, tgt.ID) {
		t.Fatalf("default skip = %v, %v, want [old %s]", def.Skip, err, tgt.ID)
	}
	if skip, _ := store.RuleSkip(t, r, "containers", "container:nginx"); !slices.Equal(skip, []string{tgt.ID}) {
		t.Fatalf("nginx skip = %v, want the new target", skip)
	}
	if skip, _ := store.RuleSkip(t, r, "containers", "container:plex"); !slices.Equal(skip, []string{store.SkipAll}) {
		t.Fatalf("plex skip = %v, want it to keep skipping everything", skip)
	}
}

func TestSavingADestinationCarriesItsFieldsIntoItsTargets(t *testing.T) {
	r := newRepo(t)
	d := saveDestination(t, r, store.OffsiteTarget{Name: "B2", Repo: "rclone:b2:bv"})
	tgt, _, err := r.EnsureDestinationTarget(d.ID, "flash", "rclone:b2:bv/flash")
	if err != nil {
		t.Fatal(err)
	}
	d.Name, d.CredsRef, d.Immutable = "Backblaze", "c2", true
	saveDestination(t, r, d)

	got, _, err := r.GetOffsiteTarget(tgt.ID)
	if err != nil || got.Name != "Backblaze" || got.CredsRef != "c2" || !got.Immutable || got.Repo != tgt.Repo {
		t.Fatalf("derived target = %+v, %v, want the new name, credentials and flag at the old location", got, err)
	}
}

func TestADestinationInUseKeepsItsLocation(t *testing.T) {
	r := newRepo(t)
	d := saveDestination(t, r, store.OffsiteTarget{Name: "B2", Repo: "rclone:b2:bv"})
	if _, _, err := r.EnsureDestinationTarget(d.ID, "flash", "rclone:b2:bv/flash"); err != nil {
		t.Fatal(err)
	}

	d.Repo = "rclone:b2:moved"
	if _, err := r.SaveDestination(d); !errors.Is(err, store.ErrDestinationInUse) {
		t.Fatalf("moving a destination in use: err = %v, want ErrDestinationInUse", err)
	}
	if err := r.DeleteDestinationIfUnused(d.ID); !errors.Is(err, store.ErrDestinationInUse) {
		t.Fatalf("deleting a destination in use: err = %v, want ErrDestinationInUse", err)
	}
}

func TestAnUnusedDestinationCanMoveAndGo(t *testing.T) {
	r := newRepo(t)
	d := saveDestination(t, r, store.OffsiteTarget{Name: "B2", Repo: "rclone:b2:bv"})
	d.Repo = "rclone:b2:moved"
	if got := saveDestination(t, r, d); got.Repo != "rclone:b2:moved" {
		t.Fatalf("repo = %q, want the new location", got.Repo)
	}
	if err := r.DeleteDestinationIfUnused(d.ID); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := r.GetDestination(d.ID); ok || err != nil {
		t.Fatalf("after delete: found=%v err=%v", ok, err)
	}
}
