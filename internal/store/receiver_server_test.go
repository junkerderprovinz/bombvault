package store_test

import (
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

func TestReceiverServerRoundTripsAndForgets(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	r := store.New(db)

	if _, ok, err := r.GetReceiverServer(); err != nil || ok {
		t.Fatalf("a fresh database has no receiver, got ok=%v err=%v", ok, err)
	}
	in := store.ReceiverServer{
		ContainerName: "rest-server", Folder: "user/restic", HostPath: "/mnt/user/restic",
		Port: 8001, User: "vault", PasswordEnc: []byte{1, 2, 3}, Host: "192.168.1.20",
	}
	if err := r.SaveReceiverServer(in); err != nil {
		t.Fatal(err)
	}
	got, ok, err := r.GetReceiverServer()
	if err != nil || !ok {
		t.Fatalf("GetReceiverServer: ok=%v err=%v", ok, err)
	}
	if got.CreatedAt == 0 {
		t.Error("a saved receiver gets a creation time")
	}
	got.CreatedAt = 0
	if got.ContainerName != in.ContainerName || got.Folder != in.Folder || got.HostPath != in.HostPath ||
		got.Port != in.Port || got.User != in.User || string(got.PasswordEnc) != string(in.PasswordEnc) || got.Host != in.Host {
		t.Fatalf("read back %+v, want %+v", got, in)
	}

	if err := r.RecordReceiverServerCheck("protected", ""); err != nil {
		t.Fatal(err)
	}
	got, _, _ = r.GetReceiverServer()
	if got.Check != "protected" || got.CheckedAt == 0 {
		t.Fatalf("check = %q at %d, want protected with a time", got.Check, got.CheckedAt)
	}

	if err := r.DeleteReceiverServer(); err != nil {
		t.Fatal(err)
	}
	if _, ok, _ := r.GetReceiverServer(); ok {
		t.Fatal("a forgotten receiver must be gone")
	}
}

func TestReceiverLoginsAreKeptPerMemberAndGoWithTheServer(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	r := store.New(db)
	if err := r.SaveReceiverServer(store.ReceiverServer{ContainerName: "rest-server", User: "vault"}); err != nil {
		t.Fatal(err)
	}
	if err := r.CreateReceiverLogin(store.ReceiverLogin{MemberID: "m1", MemberName: "attic", User: "attic"}); err != nil {
		t.Fatal(err)
	}
	if err := r.CreateReceiverLogin(store.ReceiverLogin{MemberID: "m2", MemberName: "barn", User: "attic"}); err == nil {
		t.Fatal("two members may not share a login")
	}
	if err := r.RenameReceiverLogin("m1", "attic loft"); err != nil {
		t.Fatal(err)
	}
	if l, ok, _ := r.GetReceiverLogin("m1"); !ok || l.User != "attic" || l.MemberName != "attic loft" || l.CreatedAt == 0 {
		t.Fatalf("login = %+v", l)
	}
	if err := r.DeleteReceiverServer(); err != nil {
		t.Fatal(err)
	}
	if logins, _ := r.ListReceiverLogins(); len(logins) != 0 {
		t.Fatalf("forgetting the server must forget its logins, %d left", len(logins))
	}
}
