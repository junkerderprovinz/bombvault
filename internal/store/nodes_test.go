package store_test

import (
	"errors"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

func TestHomeKeepsItsNodesUntilTheyAreReleased(t *testing.T) {
	r := newRepo(t)
	a := store.Node{ID: "aa", Name: "vps", AppKeyEnc: []byte{1, 2}, RelayMode: "project", CreatedAt: 10}
	b := store.Node{ID: "bb", Name: "cloud", AppKeyEnc: []byte{3}, RelayMode: "own", RelayURL: "https://relay.example.org", CreatedAt: 20}
	for _, n := range []store.Node{b, a} {
		if err := r.AddNode(n); err != nil {
			t.Fatal(err)
		}
	}
	if err := r.AddNode(a); err == nil {
		t.Error("the same node was added twice")
	}
	if err := r.SetNodeAdopted("aa", 30); err != nil {
		t.Fatal(err)
	}
	got, err := r.GetNode("aa")
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "vps" || string(got.AppKeyEnc) != "\x01\x02" || got.RelayMode != "project" || got.AdoptedAt != 30 {
		t.Errorf("node = %+v", got)
	}
	list, err := r.ListNodes()
	if err != nil || len(list) != 2 || list[0].ID != "aa" || list[1].RelayURL != "https://relay.example.org" {
		t.Errorf("list = %+v, %v, want both, oldest first", list, err)
	}
	if err := r.SetNodeState("aa", store.NodeReleasing); err != nil {
		t.Fatal(err)
	}
	if got, _ := r.GetNode("aa"); got.State != store.NodeReleasing {
		t.Errorf("state = %q, want releasing", got.State)
	}
	if err := r.DeleteNode("aa"); err != nil {
		t.Fatal(err)
	}
	if _, err := r.GetNode("aa"); !errors.Is(err, store.ErrNodeNotFound) {
		t.Errorf("after delete: %v", err)
	}
	if err := r.SetNodeAdopted("aa", 40); !errors.Is(err, store.ErrNodeNotFound) {
		t.Errorf("adopting a node that is gone: %v", err)
	}
}

func TestANodeRemembersTheHomeThatAdoptedIt(t *testing.T) {
	r := newRepo(t)
	link, err := r.GetNodeLink()
	if err != nil {
		t.Fatal(err)
	}
	if link != (store.NodeLink{}) {
		t.Errorf("a fresh node has %+v", link)
	}
	if err := r.SetNodeLinkAdopted("home-1", 50); err != nil {
		t.Fatal(err)
	}
	if link, _ = r.GetNodeLink(); link.HomeID != "home-1" || link.AdoptedAt != 50 || link.ReleasedAt != 0 {
		t.Errorf("adopted: %+v", link)
	}
	if err := r.SetNodeLinkReleased(60); err != nil {
		t.Fatal(err)
	}
	if link, _ = r.GetNodeLink(); link.HomeID != "home-1" || link.ReleasedAt != 60 {
		t.Errorf("released: %+v", link)
	}
}
