package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/nodelink"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/restickey"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// repoStore is storage several instances share in a test: each repository
// with the passwords of its keys. A repository a node reaches at an address
// of its own is listed under the place home reaches it at, as alias says.
type repoStore struct {
	ResticEngine
	mu    sync.Mutex
	keys  map[string][]string
	alias map[string]string
	env   map[string][]string
}

func newRepoStore() *repoStore {
	return &repoStore{keys: map[string][]string{}, alias: map[string]string{}, env: map[string][]string{}}
}

func (r *repoStore) at(repo string) string {
	if a, ok := r.alias[repo]; ok {
		return a
	}
	return repo
}

func (r *repoStore) Init(_ context.Context, repo string, m restic.Mode) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	repo = r.at(repo)
	if _, ok := r.keys[repo]; ok {
		return errors.New("Fatal: config file already exists")
	}
	r.keys[repo] = []string{m.Password}
	r.env[repo] = m.Env
	return nil
}

func (r *repoStore) RepoOpensErr(_ context.Context, repo string, m restic.Mode) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !slices.Contains(r.keys[r.at(repo)], m.Password) {
		return errors.New("Fatal: wrong password or no key found")
	}
	return nil
}

func (r *repoStore) AddKey(ctx context.Context, repo string, m restic.Mode, password, _, _ string) error {
	if err := r.RepoOpensErr(ctx, repo, m); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.keys[r.at(repo)] = append(r.keys[r.at(repo)], password)
	return nil
}

func (r *repoStore) Keys(context.Context, string, restic.Mode) ([]restic.Key, error) {
	return nil, nil
}

func (r *repoStore) RemoveKey(context.Context, string, restic.Mode, string) error { return nil }

func (r *repoStore) SnapshotsOfEveryHost(context.Context, string, restic.Mode) ([]restic.Snapshot, error) {
	return nil, nil
}

func (r *repoStore) opens(repo, password string) bool {
	return r.RepoOpensErr(context.Background(), repo, restic.Mode{Password: password}) == nil
}

// adoptedNode is a node home made and that took home on over the relay.
func adoptedNode(t *testing.T, home *instance, repos *repoStore) (node *instance, id, appKey string) {
	t.Helper()
	compose := addNode(t, home, "vps")
	id, _ = nodelink.NodeID(compose["APP_KEY"])
	node = startNode(t, compose)
	node.svc.engine = repos
	eventually(t, "the adoption", func() bool {
		n, err := home.st.GetNode(id)
		return err == nil && n.AdoptedAt != 0
	})
	return node, id, compose["APP_KEY"]
}

func TestHomeSendsANodeToItsReceiverWithALoginOfItsOwn(t *testing.T) {
	home := homeWithRelay(t)
	repos := newRepoStore()
	home.svc.engine = repos
	home.svc.cfg.HostMountRoot = t.TempDir()
	if err := os.MkdirAll(filepath.Join(home.svc.cfg.HostMountRoot, "backups"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := home.st.SaveReceiverServer(store.ReceiverServer{ContainerName: "bombvault-rest-server", Folder: "backups", Port: 8000, User: "home"}); err != nil {
		t.Fatal(err)
	}
	node, id, appKey := adoptedNode(t, home, repos)

	if _, out := home.do(t, http.MethodPost, "/api/nodes/"+id+"/placement", map[string]any{"kind": "receiver"}); out["ok"] != false {
		t.Errorf("a placement before the address for nodes = %v", out)
	}
	if code, out := home.do(t, http.MethodPut, "/api/receiver/node-address", map[string]any{"url": "http://203.0.113.7:8000"}); code != http.StatusBadRequest {
		t.Errorf("plain http to a public address = %d %v", code, out)
	}
	cert := selfSignedPEM(t)
	if code, out := home.do(t, http.MethodPut, "/api/receiver/node-address", map[string]any{"url": "https://backup.example.org/", "caPem": cert}); code != http.StatusOK || out["ok"] != true {
		t.Fatalf("set the address for nodes = %d %v", code, out)
	}

	user := "vps"
	local := map[string]string{}
	for _, d := range nodeDomains {
		resolved, err := home.svc.resolveRepo("backups/" + user + "/" + d)
		if err != nil {
			t.Fatal(err)
		}
		local[d] = resolved
		repos.alias["rest:https://backup.example.org/"+user+"/"+d] = resolved
	}
	code, out := home.do(t, http.MethodPost, "/api/nodes/"+id+"/placement", map[string]any{"kind": "receiver"})
	if code != http.StatusOK || out["ok"] != true {
		t.Fatalf("place on the receiver = %d %v", code, out)
	}
	raw, _ := json.Marshal(out["reach"])
	var reach []nodeReach
	if err := json.Unmarshal(raw, &reach); err != nil || len(reach) != 2 || !reach[0].Reached || !reach[1].Reached {
		t.Errorf("reach = %s", raw)
	}

	l, ok, err := home.st.GetReceiverLogin(id)
	if err != nil || !ok || !l.ForNode || l.User != user {
		t.Fatalf("the node's login = %+v %v %v", l, ok, err)
	}
	htpasswd, _ := os.ReadFile(filepath.Join(home.svc.cfg.HostMountRoot, "backups", htpasswdName))
	if !strings.HasPrefix(string(htpasswd), user+":") && !strings.Contains(string(htpasswd), "\n"+user+":") {
		t.Errorf("htpasswd = %q, want the node's login in it", htpasswd)
	}
	nodePassword := restickey.Derive(appKey)
	for d, at := range local {
		if !repos.opens(at, nodePassword) || !repos.opens(at, home.svc.backupPassword()) {
			t.Errorf("%s opens with %v, want the node's password and home's", d, repos.keys[at])
		}
	}
	kept, err := node.st.NodePlacements()
	if err != nil || len(kept) != 2 || kept[0].Repo != "rest:https://backup.example.org/vps/containers" || kept[0].CAPEM == "" {
		t.Errorf("the node keeps %+v %v", kept, err)
	}
	watched, err := home.st.ListReceivedRepos()
	if err != nil || len(watched) != 2 || watched[0].NodeID != id || watched[0].Repo != "backups/vps/containers" || !watched[0].Enabled {
		t.Errorf("home watches %+v %v", watched, err)
	}

	if _, _, err := home.svc.receiverOpen(context.Background(), watched[0]); err != nil {
		t.Errorf("the receiver watch cannot open the node's repository: %v", err)
	}
	if v := receivedRepoToView(watched[0]); v.NodeID != id || v.NeedsPairing {
		t.Errorf("the watch shows %+v", v)
	}

	if code, out := home.do(t, http.MethodPost, "/api/nodes/"+id+"/placement", map[string]any{"kind": "receiver"}); code != http.StatusOK || out["ok"] != true {
		t.Errorf("placing again = %d %v", code, out)
	}
	if watched, _ := home.st.ListReceivedRepos(); len(watched) != 2 {
		t.Errorf("placing again watches %d repositories, want the same two", len(watched))
	}

	if code, out := home.do(t, http.MethodDelete, "/api/nodes/"+id, nil); code != http.StatusOK || out["told"] != true {
		t.Fatalf("release = %d %v", code, out)
	}
	if _, ok, _ := home.st.GetReceiverLogin(id); ok {
		t.Error("the node let go keeps its login on the receiver")
	}
	htpasswd, _ = os.ReadFile(filepath.Join(home.svc.cfg.HostMountRoot, "backups", htpasswdName))
	if strings.Contains("\n"+string(htpasswd), "\n"+user+":") {
		t.Errorf("htpasswd = %q, want the node's login gone", htpasswd)
	}
	watched, _ = home.st.ListReceivedRepos()
	if len(watched) != 2 || watched[0].Enabled || watched[1].Enabled {
		t.Errorf("after the release home watches %+v, want the repositories kept and switched off", watched)
	}
}

func TestHomeSendsANodeToAStorageLocationWithASignInMadeForIt(t *testing.T) {
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
	plain, err := home.st.SaveDestination(store.OffsiteTarget{Name: "LAN", Repo: "rest:http://203.0.113.9:8000/home", CredsRef: "own"})
	if err != nil {
		t.Fatal(err)
	}
	node, id, appKey := adoptedNode(t, home, repos)

	for name, req := range map[string]map[string]any{
		"home's own sign-in":  {"kind": "location", "location": dest.ID, "signIn": "own"},
		"no sign-in":          {"kind": "location", "location": dest.ID},
		"plain http, public":  {"kind": "location", "location": plain.ID, "signIn": "vps"},
		"an unknown location": {"kind": "location", "location": "nowhere", "signIn": "vps"},
		"an unknown kind":     {"kind": "somewhere"},
	} {
		if _, out := home.do(t, http.MethodPost, "/api/nodes/"+id+"/placement", req); out["ok"] != false {
			t.Errorf("%s = %v", name, out)
		}
	}

	code, out := home.do(t, http.MethodPost, "/api/nodes/"+id+"/placement", map[string]any{"kind": "location", "location": dest.ID, "signIn": "vps"})
	if code != http.StatusOK || out["ok"] != true || out["warning"] == "" {
		t.Fatalf("place at the location = %d %v", code, out)
	}
	repo := "s3:https://s3.example.org/bucket/bombvault-nodes/" + id[:12] + "/containers"
	if !repos.opens(repo, restickey.Derive(appKey)) || !repos.opens(repo, home.svc.backupPassword()) {
		t.Errorf("%s opens with %v", repo, repos.keys[repo])
	}
	if env := strings.Join(repos.env[repo], " "); !strings.Contains(env, "AWS_ACCESS_KEY_ID=HOME") {
		t.Errorf("home made the repository with %q, want its own sign-in", env)
	}
	kept, err := node.st.NodePlacements()
	if err != nil || len(kept) != 2 || kept[0].Repo != repo {
		t.Fatalf("the node keeps %+v %v", kept, err)
	}
	creds, err := node.svc.placementCreds(kept[0])
	if err != nil || creds.S3KeyID != "NODE" {
		t.Errorf("the node got the sign-in %+v %v, want the one made for it", creds, err)
	}
	watched, _ := home.st.ListReceivedRepos()
	if len(watched) != 2 || watched[0].CredsRef != "own" || watched[0].Repo != repo {
		t.Errorf("home watches %+v", watched)
	}
}

func TestANodeThatIsNotAdoptedGetsNoPlacement(t *testing.T) {
	home := homeWithRelay(t)
	compose := addNode(t, home, "vps")
	id, _ := nodelink.NodeID(compose["APP_KEY"])
	if _, out := home.do(t, http.MethodPost, "/api/nodes/"+id+"/placement", map[string]any{"kind": "receiver"}); out["ok"] != false {
		t.Errorf("a placement for a node that never came = %v", out)
	}
}

func TestTheRecoveryKitNamesEachNodeWithItsKeyAndRepositories(t *testing.T) {
	home := homeWithRelay(t)
	repos := newRepoStore()
	home.svc.engine = repos
	if err := home.svc.SetCloudCredSets([]CloudCredSet{
		{ID: "own", Name: "home's", CloudCreds: CloudCreds{S3KeyID: "HOME"}},
		{ID: "vps", Name: "the node's", CloudCreds: CloudCreds{S3KeyID: "NODE"}},
	}); err != nil {
		t.Fatal(err)
	}
	dest, err := home.st.SaveDestination(store.OffsiteTarget{Name: "Bucket", Repo: "s3:https://s3.example.org/bucket", CredsRef: "own"})
	if err != nil {
		t.Fatal(err)
	}
	_, id, appKey := adoptedNode(t, home, repos)
	if code, out := home.do(t, http.MethodPost, "/api/nodes/"+id+"/placement", map[string]any{"kind": "location", "location": dest.ID, "signIn": "vps"}); code != http.StatusOK || out["ok"] != true {
		t.Fatalf("place = %d %v", code, out)
	}

	kit, err := home.svc.RecoveryKit()
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"## Nodes", "vps", appKey, restickey.Derive(appKey), "s3:https://s3.example.org/bucket/bombvault-nodes/" + id[:12] + "/containers"} {
		if !strings.Contains(kit, want) {
			t.Errorf("the kit does not name %q", want)
		}
	}
}
