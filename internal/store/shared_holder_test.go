package store_test

import (
	"slices"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

func TestTheServersStillToHearOfANewHolderAreKeptUntilTheyHeard(t *testing.T) {
	r := sharedRepoStore(t)
	sr, err := r.PutSharedRepo(store.SharedRepo{Repo: "shared/shared", Owner: "cube-id"})
	if err != nil {
		t.Fatal(err)
	}
	if err := r.QueueSharedHolderNotices(sr.ID, []string{"tower-id", "nas-id", "tower-id"}); err != nil {
		t.Fatal(err)
	}
	got, err := r.SharedHolderNotices()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || !slices.Contains(got, store.SharedHolderNotice{RepoID: sr.ID, Instance: "nas-id"}) {
		t.Fatalf("notices %+v", got)
	}
	if err := r.DropSharedHolderNotice(sr.ID, "nas-id"); err != nil {
		t.Fatal(err)
	}
	if got, err := r.SharedHolderNotices(); err != nil || len(got) != 1 || got[0].Instance != "tower-id" {
		t.Fatalf("after one heard: %+v, %v", got, err)
	}
	if err := r.DeleteSharedRepo(sr.ID); err != nil {
		t.Fatal(err)
	}
	if got, err := r.SharedHolderNotices(); err != nil || len(got) != 0 {
		t.Fatalf("the notices outlive their repository: %+v, %v", got, err)
	}
}

func TestAServerRemembersWhoTookItsSharedRepositoryOver(t *testing.T) {
	r := sharedRepoStore(t)
	sr, err := r.PutSharedRepo(store.SharedRepo{Repo: "shared/shared", Owner: "cube-id"})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok, err := r.SharedTakenOver(sr.ID); err != nil || ok {
		t.Fatalf("taken over before anybody did: %v, %v", ok, err)
	}
	if err := r.NoteSharedTakenOver(store.SharedTakenOver{RepoID: sr.ID, By: "cube-id", Name: "Cube", At: 1700}); err != nil {
		t.Fatal(err)
	}
	got, ok, err := r.SharedTakenOver(sr.ID)
	if err != nil || !ok || got.By != "cube-id" || got.Name != "Cube" || got.At != 1700 {
		t.Fatalf("taken over %+v, %v, %v", got, ok, err)
	}
	if err := r.ClearSharedTakenOver(sr.ID); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := r.SharedTakenOver(sr.ID); err != nil || ok {
		t.Fatalf("still taken over after it was taken back: %v, %v", ok, err)
	}
}
