package store_test

import (
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

func TestAFormerMemberIsKeptUntilItsBackupsAreForgottenOrItComesBack(t *testing.T) {
	r := sharedRepoStore(t)
	sr, err := r.PutSharedRepo(store.SharedRepo{Repo: "/mnt/user/shared", Owner: "nas-id"})
	if err != nil {
		t.Fatal(err)
	}
	if err := r.NoteSharedFormer(sr.ID, store.SharedFormer{Host: "bv-0123456789ab", Name: "Tower", LeftAt: 100}); err != nil {
		t.Fatal(err)
	}
	if err := r.NoteSharedFormer(sr.ID, store.SharedFormer{Host: "bv-0123456789ab", Name: "Tower", LeftAt: 200, ForgetError: "locked"}); err != nil {
		t.Fatal(err)
	}
	got, err := r.ListSharedFormer(sr.ID)
	if err != nil || len(got) != 1 || got[0].LeftAt != 200 || got[0].ForgetError != "locked" {
		t.Fatalf("former %+v, %v; want Tower once as noted last", got, err)
	}
	if err := r.DropSharedFormer(sr.ID, "bv-0123456789ab"); err != nil {
		t.Fatal(err)
	}
	if got, err := r.ListSharedFormer(sr.ID); err != nil || len(got) != 0 {
		t.Fatalf("former %+v, %v; want none", got, err)
	}
}

func TestTheFormerMembersGoWithTheirSharedRepository(t *testing.T) {
	r := sharedRepoStore(t)
	sr, err := r.PutSharedRepo(store.SharedRepo{Repo: "/mnt/user/shared", Owner: "nas-id"})
	if err != nil {
		t.Fatal(err)
	}
	if err := r.NoteSharedFormer(sr.ID, store.SharedFormer{Host: "bv-0123456789ab", Name: "Tower", LeftAt: 100}); err != nil {
		t.Fatal(err)
	}
	if err := r.DeleteSharedRepo(sr.ID); err != nil {
		t.Fatal(err)
	}
	if got, err := r.ListSharedFormer(sr.ID); err != nil || len(got) != 0 {
		t.Fatalf("former %+v, %v; want none", got, err)
	}
}
