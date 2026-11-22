package api

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/nodelink"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// placedHome is home with a node it adopted and placed one way, and what it
// takes to set that node up again.
type placedHome struct {
	home    *instance
	repos   *repoStore
	node    *instance
	id      string
	compose map[string]string
	// prepare readies a node instance for its placement before it reaches
	// home, the first one and the one set up anew.
	prepare func(t *testing.T, node *instance)
	// placedAt is the repository of the containers the node is told.
	placedAt string
}

// bootNode starts a node as its compose file describes, with prepare run on
// it first.
func (p *placedHome) bootNode(t *testing.T) *instance {
	t.Helper()
	node := newInstance(t, p.compose["NODE_NAME"], p.compose["APP_KEY"])
	node.svc.cfg.Mode = config.ModeNode
	node.svc.engine = p.repos
	// A node has no web interface and so no login password of its own.
	if _, err := node.st.MutateSettings(func(s *store.Settings) error { s.AuthPasswordHash = ""; return nil }); err != nil {
		t.Fatal(err)
	}
	node.svc.ServeNodeAPI(NewHandler(node.svc.cfg, node.st, nil, node.svc, nil, nil).Router())
	if p.prepare != nil {
		p.prepare(t, node)
	}
	t.Cleanup(node.svc.CloseHomeLink)
	h, err := nodelink.DecodeHome(p.compose["APP_KEY"], p.compose["NODE_HOME"])
	if err != nil {
		t.Fatal(err)
	}
	if err := node.svc.OpenHomeLink(h, p.compose["NODE_NAME"]); err != nil {
		t.Fatal(err)
	}
	return node
}

// adoptAndPlace adds the node, waits for its adoption and places it as req
// says.
func (p *placedHome) adoptAndPlace(t *testing.T, req map[string]any) {
	t.Helper()
	p.compose = addNode(t, p.home, "vps")
	p.id, _ = nodelink.NodeID(p.compose["APP_KEY"])
	p.node = p.bootNode(t)
	eventually(t, "the adoption", func() bool {
		n, err := p.home.st.GetNode(p.id)
		return err == nil && n.AdoptedAt != 0
	})
	if code, out := p.home.do(t, http.MethodPost, "/api/nodes/"+p.id+"/placement", req); code != http.StatusOK || out["ok"] != true {
		t.Fatalf("place = %d %v", code, out)
	}
}

// rebuild sets the node up anew on an empty config folder with the same
// APP_KEY and NODE_HOME, and lets home's watch look at it once it is back.
func (p *placedHome) rebuild(t *testing.T) *instance {
	t.Helper()
	rebuilt := p.setUpAnew(t)
	n, err := p.home.st.GetNode(p.id)
	if err != nil {
		t.Fatal(err)
	}
	if p.home.svc.nodes.poll(context.Background(), n, time.Now()) {
		t.Error("home's watch read the runs of a node that has not taken it as its home")
	}
	return rebuilt
}

// setUpAnew replaces the node with one on an empty config folder and waits
// until home sees it on the relay.
func (p *placedHome) setUpAnew(t *testing.T) *instance {
	t.Helper()
	link, _ := p.home.svc.nodeLinkOf(p.id)
	p.node.svc.CloseHomeLink()
	eventually(t, "home to see the node away", func() bool { return !link.Online() })
	p.node = p.bootNode(t)
	eventually(t, "home to see the node back", link.Online)
	return p.node
}

func receiverPlaced(t *testing.T) *placedHome {
	home, repos := receiverHome(t)
	if code, out := home.do(t, http.MethodPut, "/api/receiver/node-address", map[string]any{"url": "https://backup.example.org"}); code != http.StatusOK {
		t.Fatalf("address = %d %v", code, out)
	}
	for _, d := range nodeDomains {
		local, err := home.svc.resolveRepo("backups/vps/" + d)
		if err != nil {
			t.Fatal(err)
		}
		repos.alias["rest:https://backup.example.org/vps/"+d] = local
	}
	p := &placedHome{home: home, repos: repos, placedAt: "rest:https://backup.example.org/vps/containers"}
	p.adoptAndPlace(t, map[string]any{"kind": "receiver"})
	return p
}

func locationPlaced(t *testing.T) *placedHome {
	home := homeWithRelay(t)
	repos := newRepoStore()
	home.svc.engine = repos
	if err := home.svc.SetCloudCredSets([]CloudCredSet{
		{ID: "own", Name: "home's", CloudCreds: CloudCreds{S3KeyID: "HOME", S3Secret: "home-secret"}},
		{ID: "vps", Name: "the node's", CloudCreds: CloudCreds{S3KeyID: "NODE", S3Secret: "node-secret"}},
	}); err != nil {
		t.Fatal(err)
	}
	dest, err := home.st.SaveDestination(store.OffsiteTarget{Name: "Bucket", Repo: "s3:https://s3.example.org/bucket", CredsRef: "own"})
	if err != nil {
		t.Fatal(err)
	}
	p := &placedHome{home: home, repos: repos}
	p.adoptAndPlace(t, map[string]any{"kind": "location", "location": dest.ID, "signIn": "vps"})
	p.placedAt = "s3:https://s3.example.org/bucket/bombvault-node-" + p.id[:12] + "-containers"
	return p
}

func keptPlaced(t *testing.T) *placedHome {
	home := homeWithRelay(t)
	repos := newRepoStore()
	home.svc.engine = repos
	home.svc.nodes.fetch = func(store.Node, string) {}
	addr := "https://203.0.113.7:8443"
	p := &placedHome{home: home, repos: repos, placedAt: "rest:" + addr + "/" + nodeRestUser + "/containers"}
	// The server set up anew mounts the same Host Data, where home finds the
	// rest-server's certificate.
	hostData := t.TempDir()
	servesNodeRest(t, home, hostData)
	p.prepare = func(t *testing.T, node *instance) {
		node.svc.docker = &receiverDocker{imageID: "sha256:rest"}
		node.svc.cfg.HostMountRoot = hostData
		node.svc.cfg.HostSourceRoot = "/srv/data"
	}
	p.adoptAndPlace(t, map[string]any{"kind": "node", "address": addr})
	return p
}

func TestHomeAdoptsANodeSetUpAnewAndPlacesItAgain(t *testing.T) {
	for kind, place := range map[string]func(*testing.T) *placedHome{
		nodePlaceReceiver: receiverPlaced,
		nodePlaceLocation: locationPlaced,
		nodePlaceNode:     keptPlaced,
	} {
		t.Run(kind, func(t *testing.T) {
			p := place(t)
			before, err := p.home.st.GetNode(p.id)
			if err != nil {
				t.Fatal(err)
			}
			rebuilt := p.rebuild(t)

			eventually(t, "home to adopt the node again", func() bool {
				link, err := rebuilt.st.GetNodeLink()
				return err == nil && link.AdoptedAt != 0 && link.HomeID == p.home.id(t)
			})
			eventually(t, "the node to hear where its backups go", func() bool {
				kept, err := rebuilt.st.NodePlacements()
				return err == nil && len(kept) == 2 && kept[0].Repo == p.placedAt
			})
			eventually(t, "home to note that it heard it", func() bool {
				n, err := p.home.st.GetNode(p.id)
				return err == nil && !n.PlacementDue
			})
			after, _ := p.home.st.GetNode(p.id)
			if after.AdoptedAt != before.AdoptedAt || after.Placement != kind || after.PlacementTarget != before.PlacementTarget {
				t.Errorf("home keeps %+v, want the card of %+v carried on", after, before)
			}
			if !p.home.svc.nodes.poll(context.Background(), after, time.Now()) {
				t.Error("home's watch does not reach the node adopted again")
			}
			if kind != nodePlaceNode {
				return
			}
			if d := rebuilt.svc.docker.(*receiverDocker); len(d.created) != 1 {
				t.Errorf("the node set up anew started %d rest-servers, want its own", len(d.created))
			}
			if _, err := os.Stat(filepath.Join(rebuilt.svc.cfg.HostMountRoot, nodeRestFolder, ".tls", "cert.pem")); err != nil {
				t.Errorf("the node set up anew serves no certificate: %v", err)
			}
			sources, _ := p.home.st.ListPullSources()
			for _, ps := range sources {
				if !ps.Enabled || !strings.HasPrefix(ps.Repo, "rest:https://203.0.113.7:8443/") {
					t.Errorf("home fetches from %+v, want the node set up anew", ps)
				}
			}
		})
	}
}

func TestANodeSetUpAnewGetsItsContainersFolderSetsAndRunsBackFromHome(t *testing.T) {
	for kind, place := range map[string]func(*testing.T) *placedHome{
		nodePlaceReceiver: receiverPlaced,
		nodePlaceLocation: locationPlaced,
		nodePlaceNode:     keptPlaced,
	} {
		t.Run(kind, func(t *testing.T) {
			p := place(t)
			set, err := p.node.st.CreateFileSet(store.FileSet{Name: "Docs", Path: "docs", Excludes: []string{"*.tmp"}, Enabled: true})
			if err != nil {
				t.Fatal(err)
			}
			definition := `{"inspect":{"Name":"/plex","Config":{"Image":"plexinc/pms-docker:latest"}}}`
			tg, err := p.node.st.UpsertTarget(store.Target{ContainerName: "plex", AppdataPaths: []string{"/mnt/user/appdata/plex"}, Definition: definition})
			if err != nil {
				t.Fatal(err)
			}
			if err := p.node.st.SetInclude("plex", true); err != nil {
				t.Fatal(err)
			}
			run, err := p.node.st.StartRun(tg.ID, "backup")
			if err != nil {
				t.Fatal(err)
			}
			if err := p.node.st.FinishRun(run, "success", "snap-1", 10, ""); err != nil {
				t.Fatal(err)
			}
			n, _ := p.home.st.GetNode(p.id)
			if !p.home.svc.nodes.poll(context.Background(), n, time.Now()) {
				t.Fatal("home's watch does not reach the node")
			}

			rebuilt := p.rebuild(t)
			eventually(t, "home to give the node its folder sets back", func() bool {
				sets, err := rebuilt.st.ListFileSets()
				return err == nil && len(sets) == 1
			})
			eventually(t, "home to note that the node has what it had", func() bool {
				n, err := p.home.st.GetNode(p.id)
				return err == nil && !n.AdoptionDue
			})
			sets, _ := rebuilt.st.ListFileSets()
			if got := sets[0]; got.ID != set.ID || got.Name != "Docs" || got.Path != "docs" || !got.Enabled || len(got.Excludes) != 1 {
				t.Errorf("the folder set came back as %+v, want %+v", got, set)
			}
			back, err := rebuilt.st.GetTargetByContainer("plex")
			if err != nil || back.ID != tg.ID || back.Definition != definition || !back.IncludeInSchedule || len(back.AppdataPaths) != 1 {
				t.Errorf("the container came back as %+v %v", back, err)
			}
			runs, err := rebuilt.st.ListRuns(10)
			if err != nil || len(runs) != 1 || runs[0].ID != run || runs[0].SnapshotID != "snap-1" || runs[0].Status != "success" {
				t.Errorf("the runs came back as %+v %v", runs, err)
			}
		})
	}
}

func TestANodeSetUpAnewTwiceWhileHomeAdoptsItIsAdoptedAndPlacedAsTheLastOne(t *testing.T) {
	p := receiverPlaced(t)
	if _, err := p.node.st.CreateFileSet(store.FileSet{Name: "Docs", Path: "docs", Enabled: true}); err != nil {
		t.Fatal(err)
	}
	n, _ := p.home.st.GetNode(p.id)
	if !p.home.svc.nodes.poll(context.Background(), n, time.Now()) {
		t.Fatal("home's watch does not reach the node")
	}
	// Without its address for nodes the receiver holds the placement back,
	// so home is still adopting the first node set up anew when the second
	// one comes.
	rs, _, err := p.home.st.GetReceiverServer()
	if err != nil {
		t.Fatal(err)
	}
	if err := p.home.st.SetReceiverNodeAddress("", ""); err != nil {
		t.Fatal(err)
	}
	first := p.rebuild(t)
	eventually(t, "home to adopt the first node set up anew", func() bool {
		link, err := first.st.GetNodeLink()
		return err == nil && link.AdoptedAt != 0
	})
	second := p.setUpAnew(t)
	if err := p.home.st.SetReceiverNodeAddress(rs.NodeURL, rs.NodeCAPEM); err != nil {
		t.Fatal(err)
	}

	eventually(t, "home to adopt the second node set up anew", func() bool {
		link, err := second.st.GetNodeLink()
		return err == nil && link.AdoptedAt != 0
	})
	eventually(t, "the second one to hear where its backups go and get its folder sets back", func() bool {
		kept, _ := second.st.NodePlacements()
		sets, _ := second.st.ListFileSets()
		return len(kept) == 2 && kept[0].Repo == p.placedAt && len(sets) == 1
	})
	eventually(t, "home to note that the node has what it had", func() bool {
		n, err := p.home.st.GetNode(p.id)
		return err == nil && !n.AdoptionDue && !n.PlacementDue
	})
}
