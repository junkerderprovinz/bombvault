package store_test

import (
	"errors"
	"fmt"
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
	if err := r.SetNodePlaced("aa", "receiver", "https://backup.example.org", ""); err != nil {
		t.Fatal(err)
	}
	if err := r.SetNodePlacementDue("aa"); err != nil {
		t.Fatal(err)
	}
	if got, _ := r.GetNode("aa"); got.Placement != "receiver" || got.PlacementTarget != "https://backup.example.org" || !got.PlacementDue {
		t.Errorf("placed = %+v, want on the receiver with a change to hear", got)
	}
	if err := r.SetNodePlaced("aa", "location", "dest-1", "vps"); err != nil {
		t.Fatal(err)
	}
	if got, _ := r.GetNode("aa"); got.Placement != "location" || got.PlacementTarget != "dest-1" || got.PlacementSignIn != "vps" || got.PlacementDue {
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

func TestHomeAdoptsANodeSetUpAnewOnceAndKeepsItsFirstAdoption(t *testing.T) {
	r := newRepo(t)
	for _, n := range []store.Node{
		{ID: "aa", Name: "vps", AppKeyEnc: []byte{1}, RelayMode: "project", CreatedAt: 10},
		{ID: "bb", Name: "cloud", AppKeyEnc: []byte{2}, RelayMode: "project", CreatedAt: 20},
		{ID: "cc", Name: "old", AppKeyEnc: []byte{3}, RelayMode: "project", CreatedAt: 30},
	} {
		if err := r.AddNode(n); err != nil {
			t.Fatal(err)
		}
	}
	if due, err := r.SetNodeAdoptionDue("aa"); err != nil || due {
		t.Errorf("a node never adopted = %v %v, want nothing due", due, err)
	}
	for _, id := range []string{"aa", "bb", "cc"} {
		if err := r.SetNodeAdopted(id, 40); err != nil {
			t.Fatal(err)
		}
	}
	if err := r.SetNodePlaced("aa", "location", "dest-1", "vps"); err != nil {
		t.Fatal(err)
	}
	if err := r.SetNodeState("cc", store.NodeReleasing); err != nil {
		t.Fatal(err)
	}
	if due, err := r.SetNodeAdoptionDue("cc"); err != nil || due {
		t.Errorf("a node being released = %v %v, want nothing due", due, err)
	}
	if due, err := r.SetNodeAdoptionDue("aa"); err != nil || !due {
		t.Fatalf("an adopted node set up anew = %v %v", due, err)
	}
	if due, _ := r.SetNodeAdoptionDue("aa"); due {
		t.Error("the same node set up anew was due twice")
	}
	if due, _ := r.SetNodeAdoptionDue("bb"); !due {
		t.Error("another node set up anew was not due")
	}
	if got, _ := r.GetNode("aa"); !got.AdoptionDue || !got.PlacementDue || got.PlacementSignIn != "vps" {
		t.Errorf("placed node = %+v, want its adoption and placement due", got)
	}
	if got, _ := r.GetNode("bb"); !got.AdoptionDue || got.PlacementDue {
		t.Errorf("unplaced node = %+v, want only its adoption due", got)
	}
	if err := r.SetNodePlaced("aa", "location", "dest-1", "vps"); err != nil {
		t.Fatal(err)
	}
	if due, _ := r.SetNodeAdoptionDue("aa"); due {
		t.Error("a node set up anew once more while due was due again")
	}
	if got, _ := r.GetNode("aa"); !got.AdoptionDue || !got.PlacementDue {
		t.Errorf("node set up anew once more = %+v, want its placement due again", got)
	}
	if err := r.SetNodeAdopted("aa", 50); err != nil {
		t.Fatal(err)
	}
	if err := r.SetNodeAdoptedAgain("aa"); err != nil {
		t.Fatal(err)
	}
	if got, _ := r.GetNode("aa"); got.AdoptedAt != 40 || got.AdoptionDue {
		t.Errorf("adopted again = %+v, want the first adoption kept and nothing due", got)
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

func TestHomeKeepsWhenItBacksUpEachSectionOfANode(t *testing.T) {
	r := newRepo(t)
	if err := r.AddNode(store.Node{ID: "aa", Name: "vps", AppKeyEnc: []byte{1}, RelayMode: "project", CreatedAt: 10}); err != nil {
		t.Fatal(err)
	}
	if err := r.SetNodeCadence("aa", "containers", "daily 03:00"); err != nil {
		t.Fatal(err)
	}
	if err := r.SetNodeCadence("aa", "files", "weekly sun 04:00"); err != nil {
		t.Fatal(err)
	}
	if err := r.SetNodeCadence("aa", "vms", "daily 03:00"); err == nil {
		t.Error("a node took a cadence for a section it does not back up")
	}
	if err := r.SetNodeStarted("aa", "files", 40); err != nil {
		t.Fatal(err)
	}
	if err := r.SetNodeSeen("aa", 50); err != nil {
		t.Fatal(err)
	}
	if err := r.SetNodePruned("aa", 60); err != nil {
		t.Fatal(err)
	}
	got, err := r.GetNode("aa")
	if err != nil {
		t.Fatal(err)
	}
	if got.Cadence("containers") != "daily 03:00" || got.Cadence("files") != "weekly sun 04:00" || got.StartedAt("containers") != 0 || got.StartedAt("files") != 40 {
		t.Errorf("schedule = %+v", got)
	}
	if got.SeenAt != 50 || got.PrunedWindow != 60 {
		t.Errorf("seen %d and pruned %d, want 50 and 60", got.SeenAt, got.PrunedWindow)
	}
	if err := r.SetNodeCadence("bb", "files", "off"); !errors.Is(err, store.ErrNodeNotFound) {
		t.Errorf("a cadence for a node that is not there: %v", err)
	}
}

func TestHomeKeepsWhatItLearnedOfANodesRunsAndForgetsItWithTheNode(t *testing.T) {
	r := newRepo(t)
	if err := r.AddNode(store.Node{ID: "aa", Name: "vps", AppKeyEnc: []byte{1}, RelayMode: "project", CreatedAt: 10}); err != nil {
		t.Fatal(err)
	}
	runs := []store.NodeRun{
		{ID: "r1", Domain: "containers", Target: "web", Status: "running", StartedAt: 100},
		{ID: "r2", Domain: "files", Target: "photos", Status: "failed", StartedAt: 110, FinishedAt: 120, Error: "disk full"},
	}
	if err := r.SaveNodeRuns("aa", runs); err != nil {
		t.Fatal(err)
	}
	runs[0].Status, runs[0].FinishedAt = "success", 130
	if err := r.SaveNodeRuns("aa", runs[:1]); err != nil {
		t.Fatal(err)
	}
	got, err := r.NodeRuns("aa")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].ID != "r2" || got[0].Error != "disk full" || got[1].Status != "success" || got[1].FinishedAt != 130 || got[1].NodeID != "aa" {
		t.Errorf("runs = %+v, want both, newest first, the first one finished", got)
	}

	if err := r.OpenNodeEpisode("aa", "away", 200); err != nil {
		t.Fatal(err)
	}
	if err := r.OpenNodeEpisode("aa", "failed:files:photos", 210); err != nil {
		t.Fatal(err)
	}
	if err := r.CloseNodeEpisode("aa", "away"); err != nil {
		t.Fatal(err)
	}
	open, err := r.NodeEpisodes("aa")
	if err != nil || len(open) != 1 || open["failed:files:photos"] != 210 {
		t.Errorf("episodes = %v %v, want the failure alone", open, err)
	}

	if err := r.DeleteNode("aa"); err != nil {
		t.Fatal(err)
	}
	if got, _ := r.NodeRuns("aa"); len(got) != 0 {
		t.Errorf("runs of a node that is gone: %+v", got)
	}
	if open, _ := r.NodeEpisodes("aa"); len(open) != 0 {
		t.Errorf("episodes of a node that is gone: %v", open)
	}
}

func TestHomeKeepsOnlyTheNewestRunsOfANode(t *testing.T) {
	r := newRepo(t)
	var runs []store.NodeRun
	for i := range store.NodeRunsKept + 5 {
		runs = append(runs, store.NodeRun{ID: fmt.Sprint(i), Domain: "containers", Target: "web", Status: "success", StartedAt: int64(i)})
	}
	if err := r.SaveNodeRuns("aa", runs); err != nil {
		t.Fatal(err)
	}
	got, err := r.NodeRuns("aa")
	if err != nil || len(got) != store.NodeRunsKept || got[len(got)-1].StartedAt != 5 {
		t.Errorf("kept %d runs down to %d, %v; want the newest %d", len(got), got[len(got)-1].StartedAt, err, store.NodeRunsKept)
	}
}

func TestHomeCountsWhatTheReceiverWatchRaisedAboutANodesRepositories(t *testing.T) {
	r := newRepo(t)
	own, err := r.CreateReceivedRepo(store.ReceivedRepo{Name: "vps files", Repo: "backups/vps/files", NodeID: "aa", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	member, err := r.CreateReceivedRepo(store.ReceivedRepo{Name: "tower", Repo: "backups/tower", MemberID: "m1", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	for _, st := range []store.ReceivedAlertState{
		{ReceivedRepoID: own.ID, Source: "bv-1\x00fileset:photos", NotifiedAt: 10, BasedOn: 5},
		{ReceivedRepoID: own.ID, Source: "\x00unread", NotifiedAt: 11, BasedOn: 11},
		{ReceivedRepoID: member.ID, Source: "\x00unread", NotifiedAt: 12, BasedOn: 12},
	} {
		if err := r.UpsertReceivedAlertState(st); err != nil {
			t.Fatal(err)
		}
	}
	if n, err := r.NodeWatchAlerts("aa"); err != nil || n != 2 {
		t.Errorf("alerts about the node = %d %v, want 2", n, err)
	}
	if n, err := r.NodeWatchAlerts("bb"); err != nil || n != 0 {
		t.Errorf("alerts about a node without repositories = %d %v", n, err)
	}
}

func TestHomeRemembersTheHostOfANodeItCopiesOnwardAfterTheNodeIsGone(t *testing.T) {
	r := newRepo(t)
	if err := r.AddNode(store.Node{ID: "aa", Name: "vps", AppKeyEnc: []byte{1}, RelayMode: "project", CreatedAt: 10}); err != nil {
		t.Fatal(err)
	}
	if got, _ := r.GetNode("aa"); got.CopyOnward {
		t.Fatal("a new node copies onward")
	}
	if err := r.SetNodeCopyOnward("aa", true, "bv-aaaaaaaaaaaa"); err != nil {
		t.Fatal(err)
	}
	if got, _ := r.GetNode("aa"); !got.CopyOnward {
		t.Error("the switch did not stay on")
	}
	if err := r.SetNodeCopyOnward("aa", false, "bv-aaaaaaaaaaaa"); err != nil {
		t.Fatal(err)
	}
	if got, _ := r.GetNode("aa"); got.CopyOnward {
		t.Error("the switch did not go off")
	}
	if err := r.DeleteNode("aa"); err != nil {
		t.Fatal(err)
	}
	if hosts, err := r.OnwardHosts(); err != nil || len(hosts) != 1 || hosts[0] != "bv-aaaaaaaaaaaa" {
		t.Errorf("hosts = %v, %v, want the node's, whose copies still lie off site", hosts, err)
	}
	if err := r.SetNodeCopyOnward("bb", true, "bv-bbbbbbbbbbbb"); !errors.Is(err, store.ErrNodeNotFound) {
		t.Errorf("a node that is not there: %v", err)
	}
	if hosts, _ := r.OnwardHosts(); len(hosts) != 1 {
		t.Errorf("hosts = %v, want none for a node that is not there", hosts)
	}
}
