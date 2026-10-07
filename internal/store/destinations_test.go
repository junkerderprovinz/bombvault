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

func TestAdoptingThePrimaryKeepsItsRepositoryAndItsSlot(t *testing.T) {
	r := newRepo(t)
	s, err := r.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	s.ContainersOffsite = "s3:http://nas:9000/bv/dxp/container"
	if err := r.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}
	primary, err := r.UpsertOffsiteTarget(store.OffsiteTarget{Domain: "containers", Name: "Primary",
		Repo: "s3:http://nas:9000/bv/dxp/container", Enabled: true, RetentionKeepDaily: 9})
	if err != nil {
		t.Fatal(err)
	}
	d := saveDestination(t, r, store.OffsiteTarget{Name: "QNAP", Repo: "s3:http://nas:9000/bv/dxp", CredsRef: "c1", Provider: "s3", Immutable: true})

	got, err := r.AdoptIntoDestination(primary.ID, d.ID, func(s *store.Settings) { s.ContainersOffsiteImmutable = true })
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != primary.ID || got.Repo != primary.Repo || got.RetentionKeepDaily != 9 || got.SortOrder != 0 {
		t.Fatalf("adopted = %+v, want the same primary on its own repository with its retention", got)
	}
	if got.DestinationID != d.ID || got.Name != "QNAP" || got.CredsRef != "c1" || got.Provider != "s3" || !got.Immutable {
		t.Fatalf("adopted = %+v, want the destination's fields", got)
	}
	if s, _ := r.GetSettings(); s.ContainersOffsite != primary.Repo || !s.ContainersOffsiteImmutable {
		t.Fatalf("containers off-site field = %q, append-only %v, want the field kept and the flag settled", s.ContainersOffsite, s.ContainersOffsiteImmutable)
	}
	if _, err := r.AdoptIntoDestination(primary.ID, d.ID, nil); !errors.Is(err, store.ErrTargetFollowsDestination) {
		t.Fatalf("adopting again: %v, want ErrTargetFollowsDestination", err)
	}
}

func TestThePrimaryMovesIntoTheFolderOfADestination(t *testing.T) {
	r := newRepo(t)
	primary, err := r.UpsertOffsiteTarget(store.OffsiteTarget{Domain: "vms", Name: "Primary",
		Repo: "rest:http://old:8000/vms", Enabled: true, CredsRef: "own", RetentionKeepDaily: 9})
	if err != nil {
		t.Fatal(err)
	}
	d := saveDestination(t, r, store.OffsiteTarget{Name: "Mani", Repo: "s3:http://nas:9000/bv", CredsRef: "c1", Provider: "s3"})

	got, err := r.PrimaryFromDestination(d.ID, "vms", "s3:http://nas:9000/bv/vms", func(s *store.Settings) { s.VMsOffsite = "s3:http://nas:9000/bv/vms" })
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != primary.ID || got.SortOrder != 0 || !got.Enabled || got.Repo != "s3:http://nas:9000/bv/vms" || got.RetentionKeepDaily != 9 {
		t.Fatalf("primary = %+v, want the same row in the primary slot at the destination's folder", got)
	}
	if got.DestinationID != d.ID || got.Name != "Mani" || got.CredsRef != "c1" {
		t.Fatalf("primary = %+v, want the destination's fields", got)
	}
	if s, _ := r.GetSettings(); s.VMsOffsite != got.Repo {
		t.Fatalf("vms off-site field = %q, want %q", s.VMsOffsite, got.Repo)
	}
}

func TestADomainWithoutAPrimaryGetsOneFromADestination(t *testing.T) {
	r := newRepo(t)
	d := saveDestination(t, r, store.OffsiteTarget{Name: "B2", Repo: "rclone:b2:bv"})

	got, err := r.PrimaryFromDestination(d.ID, "flash", "rclone:b2:bv/flash", func(*store.Settings) {})
	if err != nil {
		t.Fatal(err)
	}
	field, ok, err := r.FieldOffsiteTarget("flash")
	if err != nil || !ok || field.ID != got.ID || field.DestinationID != d.ID || !field.Enabled {
		t.Fatalf("field target = %+v, %v, %v, want the new primary following B2", field, ok, err)
	}
}

func TestThePrimaryCanSwitchFromOneDestinationToAnother(t *testing.T) {
	r := newRepo(t)
	a := saveDestination(t, r, store.OffsiteTarget{Name: "A", Repo: "rclone:a:bv"})
	b := saveDestination(t, r, store.OffsiteTarget{Name: "B", Repo: "rclone:b:bv"})
	first, err := r.PrimaryFromDestination(a.ID, "zfs", "rclone:a:bv/zfs", func(*store.Settings) {})
	if err != nil {
		t.Fatal(err)
	}
	got, err := r.PrimaryFromDestination(b.ID, "zfs", "rclone:b:bv/zfs", func(*store.Settings) {})
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != first.ID || got.DestinationID != b.ID || got.Repo != "rclone:b:bv/zfs" {
		t.Fatalf("primary = %+v, want the same row following B", got)
	}
	if left, err := r.DestinationTargets(a.ID); err != nil || len(left) != 0 {
		t.Fatalf("targets of A = %+v, %v, want none", left, err)
	}
}

func TestThePrimaryCannotTakeADestinationTheDomainAlreadyCopiesTo(t *testing.T) {
	r := newRepo(t)
	d := saveDestination(t, r, store.OffsiteTarget{Name: "B2", Repo: "rclone:b2:bv"})
	if _, _, err := r.EnsureDestinationTarget(d.ID, "files", "rclone:b2:bv/files"); err != nil {
		t.Fatal(err)
	}
	if _, err := r.PrimaryFromDestination(d.ID, "files", "rclone:b2:bv/files", func(*store.Settings) {}); !errors.Is(err, store.ErrDomainHasDestinationTarget) {
		t.Fatalf("PrimaryFromDestination: %v, want ErrDomainHasDestinationTarget", err)
	}
}

func TestADomainTakesOneTargetPerDestination(t *testing.T) {
	r := newRepo(t)
	d := saveDestination(t, r, store.OffsiteTarget{Name: "B2", Repo: "rclone:b2:bv"})
	if _, _, err := r.EnsureDestinationTarget(d.ID, "vms", "rclone:b2:bv/vms"); err != nil {
		t.Fatal(err)
	}
	extra, err := r.CreateOffsiteTarget(store.OffsiteTarget{Domain: "vms", Name: "old", Repo: "rclone:b2:bv/old-vms", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.AdoptIntoDestination(extra.ID, d.ID, nil); !errors.Is(err, store.ErrDomainHasDestinationTarget) {
		t.Fatalf("adopting a second vms target: %v, want ErrDomainHasDestinationTarget", err)
	}
}
