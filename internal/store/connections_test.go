package store_test

import (
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

func TestADestinationsConnectionsReachItsTargetsAndTheirDirectRepositories(t *testing.T) {
	r := newRepo(t)
	d := saveDestination(t, r, store.OffsiteTarget{Name: "B2", Repo: "s3:https://s3.example.com/bucket/bv"})
	tgt, _, err := r.EnsureDestinationTarget(d.ID, "containers", "s3:https://s3.example.com/bucket/bv/containers")
	if err != nil {
		t.Fatal(err)
	}
	direct, err := r.CreateCompanionRepo(tgt.ID, "B2 direct", "s3:https://s3.example.com/bucket/bv/containers-direct")
	if err != nil {
		t.Fatal(err)
	}
	d.Connections = 12
	saveDestination(t, r, d)

	got, _, err := r.GetOffsiteTarget(tgt.ID)
	if err != nil || got.Connections != 12 {
		t.Fatalf("derived target = %+v, %v, want 12 connections", got, err)
	}
	repo, err := r.GetNamedRepo(direct.ID)
	if err != nil || repo.Connections != 12 {
		t.Fatalf("direct repository = %+v, %v, want 12 connections", repo, err)
	}
}

func TestADerivedTargetCannotHoldConnectionsApartFromItsDestination(t *testing.T) {
	r := newRepo(t)
	d := saveDestination(t, r, store.OffsiteTarget{Name: "B2", Repo: "rclone:b2:bv", Connections: 8})
	tgt, _, err := r.EnsureDestinationTarget(d.ID, "vms", "rclone:b2:bv/vms")
	if err != nil {
		t.Fatal(err)
	}
	if tgt.Connections != 8 {
		t.Fatalf("a new derived target has %d connections, want its destination's 8", tgt.Connections)
	}
	tgt.Connections = 0
	saved, err := r.UpsertOffsiteTarget(tgt)
	if err != nil || saved.Connections != 8 {
		t.Fatalf("saved = %+v, %v, want the destination's 8 connections kept", saved, err)
	}
}

func TestATargetOfItsOwnKeepsTheConnectionsItIsSavedWith(t *testing.T) {
	r := newRepo(t)
	created, err := r.CreateOffsiteTarget(store.OffsiteTarget{Domain: "files", Name: "NAS", Repo: "sftp:me@nas:/bv/files", Enabled: true, Connections: 3})
	if err != nil || created.Connections != 3 {
		t.Fatalf("created = %+v, %v, want 3 connections", created, err)
	}
	created.Connections = 0
	saved, err := r.UpsertOffsiteTarget(created)
	if err != nil || saved.Connections != 0 {
		t.Fatalf("saved = %+v, %v, want restic's default again", saved, err)
	}
}
