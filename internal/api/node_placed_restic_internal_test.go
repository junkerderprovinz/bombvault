package api

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/keydeposit"
	"github.com/junkerderprovinz/bombvault/internal/model"
	"github.com/junkerderprovinz/bombvault/internal/nodelink"
	"github.com/junkerderprovinz/bombvault/internal/platform"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// placedNode is a node on the real restic binary that home adopted and told
// to keep both its sections itself, with a container and a folder set to
// back up.
type placedNode struct {
	*restoreRig
	n       *nodeUnderTest
	set     store.FileSet
	compose map[string]string
}

func newPlacedNode(t *testing.T) *placedNode {
	t.Helper()
	r := newBackupRig(t)
	home := homeWithRelay(t)
	compose := addNode(t, home, "vps")
	id, err := nodelink.NodeID(compose["APP_KEY"])
	if err != nil {
		t.Fatal(err)
	}
	r.svc.cfg.AppKey, r.svc.cfg.Mode = compose["APP_KEY"], config.ModeNode
	p := &placedNode{restoreRig: r, n: &nodeUnderTest{home: home, id: id}, compose: compose}
	p.serve(t)
	p.n.adopt(t)

	// Home makes each repository through the node's rest-server before it
	// places the section there.
	for _, d := range nodeDomains {
		if err := r.svc.engine.Init(context.Background(), p.keptRepo(d), p.nodeMode()); err != nil {
			t.Fatal(err)
		}
	}
	p.place(t)

	r.write(t, map[string]string{
		"appdata/plex/library.db": "library at backup time",
		"docs/letter.txt":         "dear home",
	})
	in := model.Inspect{ID: "id-plex", Name: "/plex", Running: true, Config: model.Config{Image: plexImage}}
	in.HostConfig.Binds = []string{"/mnt/user/appdata/plex:/config"}
	in.Mounts = []model.Mount{{Type: "bind", Source: "/mnt/user/appdata/plex", Destination: "/config"}}
	r.docker.containers["plex"] = in
	r.docker.images[plexImage] = true
	if p.set, err = r.st.CreateFileSet(store.FileSet{Name: "Docs", Path: "docs", Enabled: true}); err != nil {
		t.Fatal(err)
	}
	return p
}

// serve answers home's calls with the node's service as it is.
func (p *placedNode) serve(t *testing.T) {
	t.Helper()
	p.svc.ServeNodeAPI(NewHandler(p.svc.cfg, p.st, p.docker, p.svc, nil, nil).Router())
	link, err := nodelink.DecodeHome(p.compose["APP_KEY"], p.compose["NODE_HOME"])
	if err != nil {
		t.Fatal(err)
	}
	if p.n.routes, err = p.svc.homeRoutes(link, "vps"); err != nil {
		t.Fatal(err)
	}
}

// place tells the node to keep both its sections itself.
func (p *placedNode) place(t *testing.T) {
	t.Helper()
	var placements []nodePlacement
	for _, d := range nodeDomains {
		placements = append(placements, nodePlacement{
			Domain: d, Repo: "rest:https://vps.invalid/" + nodeRestUser + "/" + d, Kept: true,
			Creds: CloudCreds{RESTUser: nodeRestUser, RESTPassword: "rest-secret"},
		})
	}
	w := p.n.call(t, keydeposit.ProofNodePlacement, nodePlacementPath, placements)
	var ans nodePlacementAnswer
	if err := json.Unmarshal(w.Body.Bytes(), &ans); err != nil || w.Code != http.StatusOK {
		t.Fatalf("placement = %d %s", w.Code, w.Body)
	}
	for _, reach := range ans.Reach {
		if !reach.Reached {
			t.Errorf("the node does not reach the %s it keeps itself: %s", reach.Domain, reach.Detail)
		}
	}
}

// rebuild has home keep the node's records as its watch does, and sets the
// node up anew on the same Host Data with an empty config folder. Home adopts
// it again, places it and gives its records back.
func (p *placedNode) rebuild(t *testing.T) {
	t.Helper()
	var ans nodeRecordsAnswer
	w := p.n.call(t, keydeposit.ProofNodeRecords, nodeRecordsPath, nodeRecordsCall{})
	if err := json.Unmarshal(w.Body.Bytes(), &ans); err != nil || w.Code != http.StatusOK || len(ans.Records) == 0 {
		t.Fatalf("records = %d %s", w.Code, w.Body)
	}
	if err := p.n.home.svc.holdNodeRecords(p.n.id, ans.Records); err != nil {
		t.Fatal(err)
	}

	cfg := p.svc.cfg
	cfg.DataDir = filepath.Join(p.root, "bombvault-anew")
	if err := os.Mkdir(cfg.DataDir, 0o700); err != nil {
		t.Fatal(err)
	}
	p.st = newTestStore(t)
	p.svc = NewService(cfg, p.st, p.docker, p.virsh, restic.Restic{Bin: "restic"})
	p.svc.SetPlatform(platform.Generic{})
	p.svc.SetProgress(p.progress)
	p.serve(t)
	p.n.adopt(t)
	p.place(t)

	held, err := p.n.home.svc.heldNodeRecords(p.n.id)
	if err != nil {
		t.Fatal(err)
	}
	if w := p.n.call(t, keydeposit.ProofNodeRecordsBack, nodeRecordsBackPath, nodeRecordsBack{Records: held}); w.Code != http.StatusOK {
		t.Fatalf("records back = %d %s", w.Code, w.Body)
	}
}

// keptRepo is the folder behind the node's rest-server that holds the
// repository of domain.
func (p *placedNode) keptRepo(domain string) string {
	return filepath.Join(p.root, nodeRestFolder, nodeRestUser, domain)
}

// nodeMode opens the node's repositories, which home makes encrypted with the
// password of the node's APP_KEY.
func (p *placedNode) nodeMode() restic.Mode {
	return restic.Mode{Encrypted: true, Password: p.svc.backupPassword()}
}

func (p *placedNode) backUp(t *testing.T) {
	t.Helper()
	if _, err := p.svc.Backup(context.Background(), "plex"); err != nil {
		t.Fatalf("container backup: %v", err)
	}
	if _, err := p.svc.BackupFileSet(context.Background(), p.set.ID); err != nil {
		t.Fatalf("folder set backup: %v", err)
	}
}

// tagsIn are the tags of every snapshot in the repository of domain the node
// keeps itself.
func (p *placedNode) tagsIn(t *testing.T, domain string) [][]string {
	t.Helper()
	snaps, err := p.svc.engine.Snapshots(context.Background(), p.keptRepo(domain), p.nodeMode())
	if err != nil {
		t.Fatal(err)
	}
	var out [][]string
	for _, sn := range snaps {
		out = append(out, sn.Tags)
	}
	return out
}

// listed is the snapshots home sees of an entry of the node, through the
// node's own route.
func (p *placedNode) listed(t *testing.T, path string) []restic.Snapshot {
	t.Helper()
	code, ans := p.n.api(t, nodeAPIRequest{Method: http.MethodGet, Path: path})
	body, err := ans.body()
	if code != http.StatusOK || err != nil {
		t.Fatalf("GET %s = %d %v", path, code, err)
	}
	var out struct {
		OK        bool              `json:"ok"`
		Error     string            `json:"error"`
		Snapshots []restic.Snapshot `json:"snapshots"`
	}
	if err := json.Unmarshal(body, &out); err != nil || !out.OK {
		t.Fatalf("GET %s = %s", path, body)
	}
	return out.Snapshots
}

// requireBackupsWherePlaced checks that each section's backups lie in the
// repository the node keeps itself, and none in the folder the node's own
// settings name.
func (p *placedNode) requireBackupsWherePlaced(t *testing.T, want int) {
	t.Helper()
	for domain, tag := range map[string]string{"containers": "container:plex", "files": "fileset:Docs"} {
		got := 0
		for _, tags := range p.tagsIn(t, domain) {
			if slices.Contains(tags, tag) {
				got++
			}
		}
		if got != want {
			t.Errorf("the %s repository the node keeps holds %d snapshots of %s, want %d", domain, got, tag, want)
		}
	}
	for _, own := range []string{"backups/containers", "backups/files"} {
		if !localRepoMissing(filepath.Join(p.root, filepath.FromSlash(own))) {
			t.Errorf("the node backed up into %s, which home did not place it at", own)
		}
	}
}

func TestANodeBacksUpListsAndPrunesWhereHomePlacedItsSections(t *testing.T) {
	p := newPlacedNode(t)
	p.backUp(t)
	p.requireBackupsWherePlaced(t, 1)
	if got := p.listed(t, "/api/containers/plex/snapshots"); len(got) != 1 {
		t.Errorf("home sees %d snapshots of the container, want 1", len(got))
	}
	if got := p.listed(t, "/api/files/sets/"+p.set.ID+"/snapshots"); len(got) != 1 {
		t.Errorf("home sees %d snapshots of the folder set, want 1", len(got))
	}

	p.write(t, map[string]string{"appdata/plex/library.db": "a newer library"})
	p.backUp(t)
	p.requireBackupsWherePlaced(t, 2)
	prune := nodePruneCall{Domain: "containers", Run: "first", Keep: restic.RetentionPolicy{KeepLast: 1}}
	deadline := time.Now().Add(time.Minute)
	for {
		var ans nodePruneAnswer
		w := p.n.call(t, keydeposit.ProofNodePrune, nodePrunePath, prune)
		if err := json.Unmarshal(w.Body.Bytes(), &ans); err != nil || !ans.OK || ans.State == nodePruneFailed {
			t.Fatalf("prune = %d %s", w.Code, w.Body)
		}
		if ans.State == nodePruneDone {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the prune never finished")
		}
		time.Sleep(50 * time.Millisecond)
	}
	if got := p.listed(t, "/api/containers/plex/snapshots"); len(got) != 1 {
		t.Errorf("after keeping the last one home sees %d snapshots of the container", len(got))
	}
}

func TestANodeRestoresFromWhereHomePlacedItsSections(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("restic's <id>:<path> selector cannot carry a Windows drive letter")
	}
	p := newPlacedNode(t)
	p.backUp(t)
	p.requireBackupsWherePlaced(t, 1)
	p.write(t, map[string]string{"appdata/plex/library.db": "changed since", "docs/letter.txt": "changed since"})

	container := p.listed(t, "/api/containers/plex/snapshots")
	folders := p.listed(t, "/api/files/sets/"+p.set.ID+"/snapshots")
	if len(container) != 1 || len(folders) != 1 {
		t.Fatalf("home sees %d snapshots of the container and %d of the folder set, want one each", len(container), len(folders))
	}
	if started, err := p.svc.StartRestore(context.Background(), "plex", container[0].ID, "local", RestoreOptions{}); err != nil || !started {
		t.Fatalf("container restore: started=%v err=%v", started, err)
	}
	p.wait(t)
	if _, started, err := p.svc.StartRestoreFileSet(context.Background(), p.set.ID, folders[0].ID, "local", "", true, RestoreOptions{}); err != nil || !started {
		t.Fatalf("folder set restore: started=%v err=%v", started, err)
	}
	p.wait(t)

	for file, want := range map[string]string{"appdata/plex/library.db": "library at backup time", "docs/letter.txt": "dear home"} {
		got, err := os.ReadFile(filepath.Join(p.root, filepath.FromSlash(file)))
		if err != nil || string(got) != want {
			t.Errorf("%s = %q %v after the restore, want %q", file, got, err, want)
		}
	}
}

func TestANodeSetUpAnewListsItsBackupsAndRecreatesARemovedContainerFromItsDefinition(t *testing.T) {
	p := newPlacedNode(t)
	p.backUp(t)
	p.rebuild(t)

	code, ans := p.n.api(t, nodeAPIRequest{Method: http.MethodGet, Path: "/api/files"})
	body, err := ans.body()
	var listed struct {
		FileSets []FileSetView `json:"fileSets"`
	}
	if code != http.StatusOK || err != nil || json.Unmarshal(body, &listed) != nil || len(listed.FileSets) != 1 || listed.FileSets[0].ID != p.set.ID {
		t.Fatalf("GET /api/files = %d %s %v, want the folder set back", code, body, err)
	}
	if got := p.listed(t, "/api/containers/plex/snapshots"); len(got) != 1 {
		t.Errorf("home sees %d snapshots of the container, want 1", len(got))
	}
	if got := p.listed(t, "/api/files/sets/"+p.set.ID+"/snapshots"); len(got) != 1 {
		t.Errorf("home sees %d snapshots of the folder set, want 1", len(got))
	}
	runs, err := p.st.ListRuns(10)
	if err != nil || len(runs) != 2 {
		t.Errorf("the node has %d runs %v, want both backups back", len(runs), err)
	}
	p.backUp(t)
	p.requireBackupsWherePlaced(t, 2)

	delete(p.docker.containers, "plex")
	if started, err := p.svc.StartRestore(context.Background(), "plex", definitionOnly, "local", RestoreOptions{}); err != nil || !started {
		t.Fatalf("recreate: started=%v err=%v", started, err)
	}
	p.wait(t)
	if in, back := p.docker.container("plex"); !back || in.Config.Image != plexImage {
		t.Errorf("the container came back as %+v %v, want it made again from its definition", in, back)
	}
}

func TestANodeSetUpAnewRecreatesARemovedContainerFromWhereHomePlacedIt(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("restic's <id>:<path> selector cannot carry a Windows drive letter")
	}
	p := newPlacedNode(t)
	p.backUp(t)
	delete(p.docker.containers, "plex")
	p.write(t, map[string]string{"appdata/plex/library.db": "changed since"})
	p.rebuild(t)

	container := p.listed(t, "/api/containers/plex/snapshots")
	if len(container) != 1 {
		t.Fatalf("home sees %d snapshots of the container, want 1", len(container))
	}
	if started, err := p.svc.StartRestore(context.Background(), "plex", container[0].ID, "local", RestoreOptions{}); err != nil || !started {
		t.Fatalf("container restore: started=%v err=%v", started, err)
	}
	p.wait(t)
	if in, back := p.docker.container("plex"); !back || in.Config.Image != plexImage {
		t.Errorf("the container came back as %+v %v, want it made again from its definition", in, back)
	}
	got, err := os.ReadFile(filepath.Join(p.root, "appdata", "plex", "library.db"))
	if err != nil || string(got) != "library at backup time" {
		t.Errorf("library.db = %q %v after the restore", got, err)
	}
}
