package api

import (
	"context"
	"errors"
	"net/http"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/restickey"
	"github.com/junkerderprovinz/bombvault/internal/secret"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

const onwardNodeKey = "b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2"

var onwardHost = restickey.Host(onwardNodeKey)

// onwardNode adds an adopted node called vps whose containers lie on the
// receiver at a folder of their own, and returns it with that folder.
func onwardNode(t *testing.T, f *placementFixture) (store.Node, string) {
	t.Helper()
	keyEnc, err := secret.Encrypt(f.svc.cfg.AppKey, []byte(onwardNodeKey))
	if err != nil {
		t.Fatal(err)
	}
	passwordEnc, err := secret.Encrypt(f.svc.cfg.AppKey, []byte(restickey.Derive(onwardNodeKey)))
	if err != nil {
		t.Fatal(err)
	}
	n := store.Node{ID: "aabbccddeeff00", Name: "vps", AppKeyEnc: keyEnc, RelayMode: "project", CreatedAt: 1, AdoptedAt: 1, Placement: nodePlaceReceiver}
	if err := f.st.AddNode(n); err != nil {
		t.Fatal(err)
	}
	repo := f.root + "/receiver/vps/containers"
	f.makeRepo(repo)
	rr := store.ReceivedRepo{ID: nodeRepoID(n.ID, "containers"), NodeID: n.ID, Name: "vps containers", Repo: repo, ResticPasswordEnc: passwordEnc, Enabled: true}
	if _, err := f.st.CreateReceivedRepo(rr); err != nil {
		t.Fatal(err)
	}
	return n, repo
}

// nodeSnap is a snapshot the node wrote.
func nodeSnap(id string, at int64, tags ...string) restic.Snapshot {
	s := snap(id, at, tags...)
	s.Hostname = onwardHost
	return s
}

// onwardScene is home's plex copied to B2, which keeps the last two, and a
// node with a plex of its own and a mail server, whose rule keeps the last
// one. B2 already holds the copy of the node's second plex from a pass
// before.
func onwardScene(t *testing.T) (*placementFixture, store.Node, string) {
	t.Helper()
	f := newPlacementFixture(t)
	keepLast(t, f, f.target("containers", "B2", "b2:bucket:containers"), 2)
	keepLastOne(t, f)
	f.container("plex", "")
	f.replicated("containers")
	f.hold(f.domainPath("containers"), snap("p1", 100, "container:plex"), snap("p2", 200, "container:plex"), snap("p3", 300, "container:plex"))
	n, repo := onwardNode(t, f)
	f.hold(repo, nodeSnap("n1", 110, "container:plex"), nodeSnap("n2", 210, "container:plex"), nodeSnap("n3", 310, "container:plex"), nodeSnap("m1", 150, "container:mail"))
	earlier := copied("c2", "n2", 210, "container:plex")
	earlier.Hostname = onwardHost
	f.hold("b2:bucket:containers", earlier)
	if err := f.svc.SetNodeCopyOnward(n.ID, true); err != nil {
		t.Fatal(err)
	}
	return f, n, repo
}

// whoHolds lists what a location holds as "<host or home> <original>", sorted.
func whoHolds(f *placementFixture, loc string) []string {
	f.eng.mu.Lock()
	defer f.eng.mu.Unlock()
	var out []string
	for _, sn := range f.eng.snaps[loc] {
		who := "home"
		if sn.Hostname == onwardHost {
			who = "node"
		}
		out = append(out, who+" "+restic.Identity(sn))
	}
	slices.Sort(out)
	return out
}

func TestANodeThatCopiesOnwardGoesToHomesTargetsAndAgesByItsOwnRule(t *testing.T) {
	f, _, repo := onwardScene(t)

	if err := f.svc.ReplicateOffsite(context.Background(), "containers"); err != nil {
		t.Fatal(err)
	}
	want := []string{"home p2", "home p3", "node m1", "node n3"}
	if got := whoHolds(f, "b2:bucket:containers"); !slices.Equal(got, want) {
		t.Fatalf("B2 holds %v, want home's two newest plex and the node's newest plex and its mail %v", got, want)
	}
	i := slices.IndexFunc(f.eng.copies, func(c copyCall) bool { return c.Src == repo })
	if i < 0 || !slices.Equal(f.eng.copies[i].IDs, []string{"n3", "m1"}) {
		t.Errorf("copies = %+v, want only what the node's rule keeps, n3 and m1", f.eng.copies)
	}
	var nodeForgets []forgetCall
	for _, c := range f.eng.forgets {
		if c.Host == onwardHost {
			nodeForgets = append(nodeForgets, c)
		}
	}
	if len(nodeForgets) != 1 || !slices.Equal(nodeForgets[0].Tags, []string{"container:plex"}) || nodeForgets[0].Policy.KeepLast != 1 {
		t.Errorf("forgets under the node's host = %+v, want its plex once under its rule", nodeForgets)
	}
}

func TestHomesKeepRuleAtATargetLeavesTheCopiesOfANodeApart(t *testing.T) {
	f, _, _ := onwardScene(t)
	f.hold("b2:bucket:containers",
		func() restic.Snapshot {
			s := copied("c9", "n9", 390, "container:plex")
			s.Hostname = onwardHost
			return s
		}(),
		copied("c1", "p1", 100, "container:plex"))

	if err := f.svc.ReplicateOffsite(context.Background(), "containers"); err != nil {
		t.Fatal(err)
	}
	got := whoHolds(f, "b2:bucket:containers")
	if !slices.Contains(got, "home p2") || !slices.Contains(got, "home p3") || slices.Contains(got, "home p1") {
		t.Errorf("B2 holds %v, want home's plex aged as two of its own, the node's newer one not counted", got)
	}
	copies, err := f.st.ItemCopiesForDomain("containers")
	if err != nil {
		t.Fatal(err)
	}
	i := slices.IndexFunc(copies, func(c store.ItemCopies) bool { return c.Identity == "container:plex" })
	if i < 0 || copies[i].SnapshotCount != 2 {
		t.Errorf("home's copies = %+v, want its own two of plex", copies)
	}
}

func TestANodeSwitchedOffIsNotCopiedOnwardAndItsCopiesStay(t *testing.T) {
	f, n, repo := onwardScene(t)
	if err := f.svc.SetNodeCopyOnward(n.ID, false); err != nil {
		t.Fatal(err)
	}

	if err := f.svc.ReplicateOffsite(context.Background(), "containers"); err != nil {
		t.Fatal(err)
	}
	if slices.ContainsFunc(f.eng.copies, func(c copyCall) bool { return c.Src == repo }) {
		t.Errorf("copies = %+v, want none from the node", f.eng.copies)
	}
	if got := whoHolds(f, "b2:bucket:containers"); !slices.Contains(got, "node n2") {
		t.Errorf("B2 holds %v, want the node's earlier copy left where it is", got)
	}
}

func TestAFailedCopyOfANodeAlertsOnceUntilACopyGoesThrough(t *testing.T) {
	f, n, repo := onwardScene(t)
	wh := newPathRecorder(t)
	if err := f.svc.SetNotifyConfig(oldNotify(map[string]any{"on": "failure", "webhookEnabled": true, "webhookUrl": wh.srv.URL, "webhookFormat": "generic"})); err != nil {
		t.Fatal(err)
	}
	f.eng.copyErr[repo] = errors.New("connection reset")

	for range 2 {
		if err := f.svc.ReplicateOffsite(context.Background(), "containers"); err != nil {
			t.Fatalf("home's own copy failed over the node's: %v", err)
		}
	}
	if got := len(wh.got()); got != 1 {
		t.Errorf("%d alerts, want one for the episode", got)
	}
	views, err := f.svc.ListNodes(context.Background())
	if err != nil || len(views) != 1 || views[0].Failures != 1 {
		t.Fatalf("views = %+v, %v, want the node's card to count the failed copy", views, err)
	}

	delete(f.eng.copyErr, repo)
	if err := f.svc.ReplicateOffsite(context.Background(), "containers"); err != nil {
		t.Fatal(err)
	}
	if open, _ := f.st.NodeEpisodes(n.ID); len(open) != 0 {
		t.Errorf("episodes = %v, want the failure over once a copy went through", open)
	}
}

func TestNothingOfANodeAgesAtAnAppendOnlyTarget(t *testing.T) {
	f, _, repo := onwardScene(t)
	f.appendOnly(f.target("containers", "Vault", "b2:vault:containers").ID)

	if err := f.svc.ReplicateOffsite(context.Background(), "containers"); err != nil {
		t.Fatal(err)
	}
	i := slices.IndexFunc(f.eng.copies, func(c copyCall) bool { return c.Src == repo && c.Dest == "b2:vault:containers" })
	if i < 0 || !slices.Equal(f.eng.copies[i].IDs, []string{"n1", "n2", "n3", "m1"}) {
		t.Errorf("copies = %+v, want everything at the append-only target, where nothing ages", f.eng.copies)
	}
	if slices.ContainsFunc(f.eng.forgets, func(c forgetCall) bool { return c.Repo == "b2:vault:containers" }) {
		t.Errorf("forgets = %+v, want none at an append-only target", f.eng.forgets)
	}
}

func TestANodeThatKeepsItsOwnBackupsIsCopiedOnwardFromWhatHomeFetched(t *testing.T) {
	f, n, repo := onwardScene(t)
	if _, err := f.db.Exec(`UPDATE nodes SET placement = ? WHERE id = ?`, nodePlaceNode, n.ID); err != nil {
		t.Fatal(err)
	}
	ps, err := f.st.CreatePullSource(store.PullSource{Name: "vps containers", Repo: "rest:https://203.0.113.7:8000/containers", Domain: "containers", Enabled: true, NodeID: n.ID})
	if err != nil {
		t.Fatal(err)
	}
	landing := filepath.ToSlash(pulledRepo(f.domainPath("containers"), ps.ID))
	f.makeRepo(landing)
	f.hold(landing, nodeSnap("n3", 310, "container:plex"))

	if err := f.svc.ReplicateOffsite(context.Background(), "containers"); err != nil {
		t.Fatal(err)
	}
	if slices.ContainsFunc(f.eng.copies, func(c copyCall) bool { return c.Src == repo || strings.HasPrefix(c.Src, "rest:") }) {
		t.Errorf("copies = %+v, want none from the node itself", f.eng.copies)
	}
	if !slices.ContainsFunc(f.eng.copies, func(c copyCall) bool { return c.Src == landing && slices.Equal(c.IDs, []string{"n3"}) }) {
		t.Errorf("copies = %+v, want n3 from what home fetched", f.eng.copies)
	}
}

func TestTheNodeCardSaysWhereTheCopiesGo(t *testing.T) {
	f, n, _ := onwardScene(t)
	// A section's own off-site field names its row after the section.
	f.target("files", "files", "rest:https://backup.example.org/files")
	wasabi, err := f.st.SaveDestination(store.OffsiteTarget{Name: "Wasabi", Repo: "s3:https://s3.example.org/bucket", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := f.st.EnsureDestinationTarget(wasabi.ID, "files", "s3:https://s3.example.org/bucket/files"); err != nil {
		t.Fatal(err)
	}
	f.signIn()

	f.do(http.MethodPut, "/api/nodes/"+n.ID+"/copy-onward", map[string]any{"on": true})
	views, err := f.svc.ListNodes(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"b2:bucket:containers", "rest:https://backup.example.org/files", "Wasabi"}
	if v := views[0]; !v.CopyOnward || !slices.Equal(v.CopiesTo, want) {
		t.Errorf("view = %+v, want the copies going to %v, as Storage names them", v, want)
	}
	f.do(http.MethodPut, "/api/nodes/"+n.ID+"/copy-onward", map[string]any{"on": false})
	if views, _ := f.svc.ListNodes(context.Background()); views[0].CopyOnward || len(views[0].CopiesTo) != 0 {
		t.Errorf("view = %+v, want nothing copied once it is off", views[0])
	}
}
