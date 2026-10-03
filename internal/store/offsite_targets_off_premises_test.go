package store_test

import (
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

func TestOffPremisesStaysOnPlainNamedRepositoriesOnly(t *testing.T) {
	r := newRepo(t)

	nas := store.SeedNamedRepo(t, r, "NAS Keller", "nas/bv")
	nas.OffPremises = true
	if _, err := r.UpsertOffsiteTarget(nas); err != nil {
		t.Fatal(err)
	}
	got, err := r.GetNamedRepo(nas.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !got.OffPremises {
		t.Fatal("a named repository keeps the mark it was given")
	}
	got.OffPremises = false
	if _, err := r.UpsertOffsiteTarget(got); err != nil {
		t.Fatal(err)
	}
	if again, err := r.GetNamedRepo(nas.ID); err != nil || again.OffPremises {
		t.Fatalf("switched back on the premises: %+v, %v", again, err)
	}

	target := store.SeedOffsiteTarget(t, r, "containers", "b2:bucket:containers")
	target.OffPremises = true
	if _, err := r.UpsertOffsiteTarget(target); err != nil {
		t.Fatal(err)
	}
	if stored, _, err := r.GetOffsiteTarget(target.ID); err != nil || stored.OffPremises {
		t.Fatalf("an off-site target carries no mark: %+v, %v", stored, err)
	}

	direct := store.SeedCompanion(t, r, target)
	direct.OffPremises = true
	if _, err := r.UpsertOffsiteTarget(direct); err != nil {
		t.Fatal(err)
	}
	if stored, err := r.GetNamedRepo(direct.ID); err != nil || stored.OffPremises {
		t.Fatalf("a direct repository counts with its target, not on its own: %+v, %v", stored, err)
	}
}

func TestConnectingAsDirectRepositoryDropsTheOffPremisesMark(t *testing.T) {
	r := newRepo(t)
	target := store.SeedOffsiteTarget(t, r, "containers", "b2:bucket:containers")
	old := store.SeedNamedRepo(t, r, "B2 old", "b2:bucket:containers-direct")
	old.OffPremises = true
	if _, err := r.UpsertOffsiteTarget(old); err != nil {
		t.Fatal(err)
	}
	if err := r.ConnectCompanion(old.ID, target.ID); err != nil {
		t.Fatal(err)
	}
	got, err := r.GetNamedRepo(old.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.OffPremises {
		t.Fatal("a repository that became a direct repository still carries its own mark")
	}
}

func TestDeletingATargetOnImportLeavesARemoteDirectRepositoryOffThePremises(t *testing.T) {
	r := newRepo(t)
	locations := []string{"/mnt/remotes/nas/offsite", "S3:https://s3.example.com/upper"}
	for _, scheme := range restic.RemoteSchemes() {
		locations = append(locations, scheme+":host/bv")
	}
	for _, loc := range locations {
		target := store.SeedOffsiteTarget(t, r, "containers", loc)
		direct := store.SeedCompanion(t, r, target)
		if err := r.DeleteOffsiteTarget(target.ID); err != nil {
			t.Fatal(err)
		}
		got, err := r.GetNamedRepo(direct.ID)
		if err != nil {
			t.Fatal(err)
		}
		if got.OffPremises != restic.IsRemoteRepo(got.Repo) {
			t.Errorf("%s: off premises = %v, want %v", got.Repo, got.OffPremises, restic.IsRemoteRepo(got.Repo))
		}
	}
}
