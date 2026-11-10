package store_test

import (
	"bytes"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

func kitRepo(t *testing.T) *store.Repo {
	t.Helper()
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	return store.New(db)
}

func TestTheGroupStateOfTheKitIsKeptAsGiven(t *testing.T) {
	r := kitRepo(t)
	got, err := r.GroupKitState()
	if err != nil || len(got) != 0 {
		t.Fatalf("a new database keeps %q, %v; want nothing", got, err)
	}
	if err := r.SetGroupKitState([]byte("sealed state")); err != nil {
		t.Fatal(err)
	}
	if got, err := r.GroupKitState(); err != nil || string(got) != "sealed state" {
		t.Fatalf("kept %q, %v", got, err)
	}
	if err := r.SetGroupKitState(nil); err != nil {
		t.Fatal(err)
	}
	if got, err := r.GroupKitState(); err != nil || len(got) != 0 {
		t.Fatalf("after clearing it keeps %q, %v", got, err)
	}
}

func TestAMemberIsFoundByThePinnedKey(t *testing.T) {
	r := kitRepo(t)
	key := bytes.Repeat([]byte{7}, 32)
	if err := r.PinMemberKey("0123456789abcdef0123456789abcdef", key); err != nil {
		t.Fatal(err)
	}
	if id, found, err := r.MemberByKey(key); err != nil || !found || id != "0123456789abcdef0123456789abcdef" {
		t.Fatalf("MemberByKey = %q, %t, %v", id, found, err)
	}
	if _, found, err := r.MemberByKey(bytes.Repeat([]byte{8}, 32)); err != nil || found {
		t.Fatalf("another key found a member: %t, %v", found, err)
	}
	keys, err := r.MemberKeys()
	if err != nil || len(keys) != 1 || !bytes.Equal(keys["0123456789abcdef0123456789abcdef"], key) {
		t.Fatalf("MemberKeys = %v, %v", keys, err)
	}
}
