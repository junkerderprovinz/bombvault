package store_test

import (
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

func TestTheSharedRepositoryServerIsKeptWithItsLastCheck(t *testing.T) {
	r := sharedRepoStore(t)
	if _, ok, err := r.GetSharedRepoServer(); err != nil || ok {
		t.Fatalf("a fresh database holds a shared repository server: ok %v, err %v", ok, err)
	}
	in := store.SharedRepoServer{
		ContainerName: "rest-server-shared", Folder: "user/shared", HostPath: "/mnt/user/shared",
		Port: 8001, User: "nas", PasswordEnc: []byte{1, 2, 3}, Repo: "user/shared/repo",
	}
	if err := r.SaveSharedRepoServer(in); err != nil {
		t.Fatal(err)
	}
	if err := r.RecordSharedRepoServerCheck("protected", "delete refused"); err != nil {
		t.Fatal(err)
	}
	got, ok, err := r.GetSharedRepoServer()
	if err != nil || !ok {
		t.Fatalf("GetSharedRepoServer: ok %v, err %v", ok, err)
	}
	if got.Port != 8001 || got.Repo != in.Repo || string(got.PasswordEnc) != string(in.PasswordEnc) || got.CreatedAt == 0 ||
		got.Check != "protected" || got.CheckDetail != "delete refused" || got.CheckedAt == 0 {
		t.Fatalf("got %+v", got)
	}
}

func TestAJoinIsKeptPerDirectionAndServer(t *testing.T) {
	r := sharedRepoStore(t)
	in := store.SharedJoin{
		Direction: store.SharedJoinIn, MemberID: "tower-id", MemberName: "Tower", Host: "bv-0123456789ab",
		State: store.SharedJoinAsked,
	}
	if err := r.SaveSharedJoin(in); err != nil {
		t.Fatal(err)
	}
	out := store.SharedJoin{Direction: store.SharedJoinOut, MemberID: "tower-id", MemberName: "Tower", State: store.SharedJoinAsked}
	if err := r.SaveSharedJoin(out); err != nil {
		t.Fatal(err)
	}
	in.State, in.User, in.LoginEnc, in.OnceEnc, in.OnceKey = store.SharedJoinAllowed, "tower", []byte{4}, []byte{5}, "abcd"
	if err := r.SaveSharedJoin(in); err != nil {
		t.Fatal(err)
	}

	got, ok, err := r.GetSharedJoin(store.SharedJoinIn, "tower-id")
	if err != nil || !ok {
		t.Fatalf("GetSharedJoin: ok %v, err %v", ok, err)
	}
	if got.State != store.SharedJoinAllowed || got.User != "tower" || got.OnceKey != "abcd" || string(got.OnceEnc) != "\x05" || got.CreatedAt == 0 {
		t.Fatalf("got %+v", got)
	}
	list, err := r.ListSharedJoins(store.SharedJoinOut)
	if err != nil || len(list) != 1 || list[0].State != store.SharedJoinAsked {
		t.Fatalf("outgoing joins = %+v, %v", list, err)
	}

	if err := r.DeleteSharedJoin(store.SharedJoinIn, "tower-id"); err != nil {
		t.Fatal(err)
	}
	if _, ok, _ := r.GetSharedJoin(store.SharedJoinIn, "tower-id"); ok {
		t.Fatal("the join is still there")
	}
	if _, ok, _ := r.GetSharedJoin(store.SharedJoinOut, "tower-id"); !ok {
		t.Fatal("deleting one direction took the other")
	}
}
