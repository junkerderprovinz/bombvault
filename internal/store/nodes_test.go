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
	if err := r.SetNodePlaced("aa", "receiver", "https://backup.example.org"); err != nil {
		t.Fatal(err)
	}
	if err := r.SetNodePlacementDue("aa"); err != nil {
		t.Fatal(err)
	}
	if got, _ := r.GetNode("aa"); got.Placement != "receiver" || got.PlacementTarget != "https://backup.example.org" || !got.PlacementDue {
		t.Errorf("placed = %+v, want on the receiver with a change to hear", got)
	}
	if err := r.SetNodePlaced("aa", "location", "dest-1"); err != nil {
		t.Fatal(err)
	}
	if got, _ := r.GetNode("aa"); got.Placement != "location" || got.PlacementTarget != "dest-1" || got.PlacementDue {
		t.Errorf("placed again = %+v", got)
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

func TestANodeKeepsWhereHomeSendsEachSectionsBackups(t *testing.T) {
	r := newRepo(t)
	for _, p := range []store.NodePlacement{
		{Domain: "files", Repo: "rest:https://home.example.org/vps/files", CredsEnc: []byte{1}, SetAt: 10},
		{Domain: "containers", Repo: "rest:https://home.example.org/vps/containers", CredsEnc: []byte{2}, CAPEM: "pem", SetAt: 10},
		{Domain: "files", Repo: "s3:https://s3.example.org/bucket/vps/files", CredsEnc: []byte{3}, SetAt: 20},
	} {
		if err := r.SetNodePlacement(p); err != nil {
			t.Fatal(err)
		}
	}
	got, err := r.NodePlacements()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Domain != "containers" || got[0].CAPEM != "pem" || got[1].Repo != "s3:https://s3.example.org/bucket/vps/files" || got[1].SetAt != 20 {
		t.Errorf("placements = %+v, want one per section, the later one kept", got)
	}
}

func TestHomeKeepsWhatItsReceiverHandsToNodes(t *testing.T) {
	r := newRepo(t)
	if err := r.SaveReceiverServer(store.ReceiverServer{ContainerName: "rest", Folder: "backups", Port: 8000, User: "home"}); err != nil {
		t.Fatal(err)
	}
	if err := r.SetReceiverNodeAddress("https://backup.example.org", "pem"); err != nil {
		t.Fatal(err)
	}
	rs, _, err := r.GetReceiverServer()
	if err != nil {
		t.Fatal(err)
	}
	if rs.NodeURL != "https://backup.example.org" || rs.NodeCAPEM != "pem" {
		t.Errorf("receiver = %+v", rs)
	}
	rs.Check = "protected"
	if err := r.SaveReceiverServer(rs); err != nil {
		t.Fatal(err)
	}
	if again, _, _ := r.GetReceiverServer(); again.NodeURL != rs.NodeURL {
		t.Errorf("a save lost the address for nodes: %+v", again)
	}

	if err := r.CreateReceiverLogin(store.ReceiverLogin{MemberID: "node-1", MemberName: "vps", User: "vps", ForNode: true}); err != nil {
		t.Fatal(err)
	}
	if l, ok, err := r.GetReceiverLogin("node-1"); err != nil || !ok || !l.ForNode {
		t.Errorf("login = %+v %v %v", l, ok, err)
	}

	rr, err := r.CreateReceivedRepo(store.ReceivedRepo{Name: "vps containers", Repo: "backups/vps/containers", NodeID: "node-1", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	got, _, err := r.GetReceivedRepo(rr.ID)
	if err != nil || got.NodeID != "node-1" || got.NeedsPairing() {
		t.Errorf("received repo = %+v %v, want it from the node and paired", got, err)
	}
}
