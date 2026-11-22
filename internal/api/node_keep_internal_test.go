package api

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/junkerderprovinz/bombvault/internal/keydeposit"
	"github.com/junkerderprovinz/bombvault/internal/model"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/restickey"
	"github.com/junkerderprovinz/bombvault/internal/trust"
)

// keepingNode is an adopted node whose Host Data lies in a temporary folder,
// with the Docker it starts its rest-server on.
func keepingNode(t *testing.T) (*nodeUnderTest, *receiverDocker) {
	t.Helper()
	n := newNodeUnderTest(t)
	n.adopt(t)
	d := &receiverDocker{imageID: "sha256:rest"}
	n.node.svc.docker = d
	n.node.svc.cfg.HostMountRoot = t.TempDir()
	n.node.svc.cfg.HostSourceRoot = "/srv/data"
	return n, d
}

func keepAnswer(t *testing.T, w interface{ Bytes() []byte }) nodeKeepAnswer {
	t.Helper()
	var ans nodeKeepAnswer
	if err := json.Unmarshal(w.Bytes(), &ans); err != nil {
		t.Fatalf("answer %s: %v", w.Bytes(), err)
	}
	return ans
}

func TestANodeKeepsItsBackupsOnlyWhileAdopted(t *testing.T) {
	n := newNodeUnderTest(t)
	n.node.svc.docker = &receiverDocker{imageID: "sha256:rest"}
	n.node.svc.cfg.HostMountRoot = t.TempDir()
	keep := nodeKeepCall{Address: "https://203.0.113.7:8000"}
	if w := n.call(t, keydeposit.ProofNodeKeep, nodeKeepPath, keep); w.Code != http.StatusConflict || !bytes.Contains(w.Body.Bytes(), []byte(codeNodeNotAdopted)) {
		t.Errorf("keeping before the adoption = %d %s", w.Code, w.Body)
	}
	n.adopt(t)
	if w := n.call(t, keydeposit.ProofNodePlacement, nodeKeepPath, keep); w.Code != http.StatusForbidden {
		t.Errorf("a placement passed off as keeping = %d %s", w.Code, w.Body)
	}
	if w := n.call(t, keydeposit.ProofNodeKeep, nodeKeepPath, keep); w.Code != http.StatusOK {
		t.Errorf("keeping once adopted = %d %s", w.Code, w.Body)
	}
	if w := n.call(t, keydeposit.ProofNodeRelease, nodeReleasePath, nil); w.Code != http.StatusOK {
		t.Fatalf("release = %d %s", w.Code, w.Body)
	}
	if w := n.call(t, keydeposit.ProofNodeKeep, nodeKeepPath, keep); w.Code != http.StatusConflict || !bytes.Contains(w.Body.Bytes(), []byte(codeNodeReleased)) {
		t.Errorf("keeping after the release = %d %s", w.Code, w.Body)
	}
}

func TestANodeStartsAnAppendOnlyRestServerWithACertificateForItsAddress(t *testing.T) {
	n, d := keepingNode(t)
	w := n.call(t, keydeposit.ProofNodeKeep, nodeKeepPath, nodeKeepCall{Address: "https://203.0.113.7:8443"})
	ans := keepAnswer(t, w.Body)
	if w.Code != http.StatusOK || !ans.OK || ans.User != nodeRestUser || ans.Password == "" || ans.CAPEM == "" {
		t.Fatalf("keep = %d %s", w.Code, w.Body)
	}
	if len(d.created) != 1 {
		t.Fatalf("created %d containers, want the rest-server", len(d.created))
	}
	spec := d.created[0]
	options := strings.Join(spec.Config.Env, " ")
	for _, want := range []string{"--append-only", "--private-repos", "--tls", "--tls-cert /data/.tls/cert.pem", "--tls-key /data/.tls/key.pem"} {
		if !strings.Contains(options, want) {
			t.Errorf("options %q lack %s", options, want)
		}
	}
	if spec.Name != nodeRestName || !slices.Equal(spec.HostConfig.Binds, []string{"/srv/data/" + nodeRestFolder + ":/data"}) ||
		spec.HostConfig.PortBindings["8000/tcp"][0].HostPort != "8443" {
		t.Errorf("spec = %s %v %v", spec.Name, spec.HostConfig.Binds, spec.HostConfig.PortBindings)
	}

	htpasswd, _ := os.ReadFile(filepath.Join(n.node.svc.cfg.HostMountRoot, nodeRestFolder, htpasswdName))
	user, hash, _ := strings.Cut(strings.TrimSpace(string(htpasswd)), ":")
	if user != nodeRestUser || bcrypt.CompareHashAndPassword([]byte(hash), []byte(ans.Password)) != nil {
		t.Errorf("htpasswd = %q, want the login handed to home", htpasswd)
	}
	served, _ := os.ReadFile(filepath.Join(n.node.svc.cfg.HostMountRoot, nodeRestFolder, ".tls", "cert.pem"))
	if string(served) != ans.CAPEM {
		t.Error("home was handed another certificate than the one the server serves")
	}
	if _, err := os.Stat(filepath.Join(n.node.svc.cfg.HostMountRoot, nodeRestFolder, ".tls", "key.pem")); err != nil {
		t.Errorf("the key: %v", err)
	}
	block, _ := pem.Decode([]byte(ans.CAPEM))
	leaf, err := x509.ParseCertificate(block.Bytes)
	if err != nil || leaf.VerifyHostname("203.0.113.7") != nil {
		t.Errorf("the certificate does not cover the address: %v", err)
	}
	pool := x509.NewCertPool()
	pool.AddCert(leaf)
	if _, err := leaf.Verify(x509.VerifyOptions{Roots: pool, DNSName: "203.0.113.7"}); err != nil {
		t.Errorf("a client that trusts the certificate does not accept it: %v", err)
	}

	d.live[nodeRestName] = true
	w = n.call(t, keydeposit.ProofNodeKeep, nodeKeepPath, nodeKeepCall{Address: "https://203.0.113.7:8443"})
	again := keepAnswer(t, w.Body)
	if !again.OK || again.CAPEM != ans.CAPEM || len(d.created) != 2 || len(d.removed) != 1 {
		t.Errorf("keeping again = %s, %d created, %v removed; want the same certificate and the server made anew", w.Body, len(d.created), d.removed)
	}
	w = n.call(t, keydeposit.ProofNodeKeep, nodeKeepPath, nodeKeepCall{Address: "https://vps.example.org:8443"})
	if moved := keepAnswer(t, w.Body); !moved.OK || moved.CAPEM == ans.CAPEM {
		t.Errorf("a new address = %s, want a certificate for it", w.Body)
	}
}

func TestANodeKeepsItsBackupsOverPlainHTTPOnlyAtAPrivateAddress(t *testing.T) {
	n, d := keepingNode(t)
	if w := n.call(t, keydeposit.ProofNodeKeep, nodeKeepPath, nodeKeepCall{Address: "http://203.0.113.7:8000"}); w.Code != http.StatusBadRequest {
		t.Errorf("plain http at a public address = %d %s", w.Code, w.Body)
	}
	w := n.call(t, keydeposit.ProofNodeKeep, nodeKeepPath, nodeKeepCall{Address: "http://100.101.102.103:8000"})
	ans := keepAnswer(t, w.Body)
	if !ans.OK || ans.CAPEM != "" || len(d.created) != 1 || strings.Contains(strings.Join(d.created[0].Config.Env, " "), "--tls") {
		t.Errorf("keep over a VPN = %s", w.Body)
	}
}

func TestANodeRefusesAPortAnotherContainerHolds(t *testing.T) {
	n, d := keepingNode(t)
	d.allocs = []model.Allocation{{Name: "caddy", HostPorts: []string{"8443/tcp"}}}
	w := n.call(t, keydeposit.ProofNodeKeep, nodeKeepPath, nodeKeepCall{Address: "https://203.0.113.7:8443"})
	if ans := keepAnswer(t, w.Body); ans.OK || !strings.Contains(ans.Error, "caddy") || len(d.created) != 0 {
		t.Errorf("keep on a taken port = %s", w.Body)
	}
}

func TestHomeFetchesTheBackupsANodeKeepsAndTrustsItsCertificate(t *testing.T) {
	home := homeWithRelay(t)
	repos := newRepoStore()
	home.svc.engine = repos
	node, id, appKey := adoptedNode(t, home, repos)
	d := &receiverDocker{imageID: "sha256:rest"}
	node.svc.docker = d
	node.svc.cfg.HostMountRoot = t.TempDir()
	node.svc.cfg.HostSourceRoot = "/srv/data"
	node.engine.want = node.svc.backupPassword()
	servesNodeRest(t, home, node.svc.cfg.HostMountRoot)

	for name, addr := range map[string]string{"plain http, public": "http://203.0.113.7:8000", "no address": "", "not an address": "vps:8000", "a folder": "https://203.0.113.7:8443/backups"} {
		if code, out := home.do(t, http.MethodPost, "/api/nodes/"+id+"/placement", map[string]any{"kind": "node", "address": addr}); code != http.StatusBadRequest || out["ok"] != false {
			t.Errorf("%s = %d %v", name, code, out)
		}
	}
	if len(d.created) != 0 {
		t.Fatal("the node started a rest-server for a refused address")
	}

	addr := "https://203.0.113.7:8443"
	code, out := home.do(t, http.MethodPost, "/api/nodes/"+id+"/placement", map[string]any{"kind": "node", "address": addr + "/"})
	if code != http.StatusOK || out["ok"] != true {
		t.Fatalf("keep on the node = %d %v", code, out)
	}
	served, err := os.ReadFile(filepath.Join(node.svc.cfg.HostMountRoot, nodeRestFolder, ".tls", "cert.pem"))
	if err != nil {
		t.Fatal(err)
	}
	if pinned, err := os.ReadFile(home.svc.nodeCertFile(id)); err != nil || !bytes.Equal(pinned, served) {
		t.Errorf("home trusts %q %v, want the certificate the node serves", pinned, err)
	}
	if !strings.HasPrefix(home.svc.nodeCertFile(id), trust.Dir(home.svc.cfg.DataDir)) {
		t.Errorf("the certificate lies at %s, outside the certs folder restic reads", home.svc.nodeCertFile(id))
	}

	nodePassword := restickey.Derive(appKey)
	for _, dom := range nodeDomains {
		repo := "rest:" + addr + "/" + nodeRestUser + "/" + dom
		if !repos.opens(repo, nodePassword) || !repos.opens(repo, home.svc.backupPassword()) {
			t.Errorf("%s opens with %v, want the node's password and home's", repo, repos.keys[repo])
		}
		if env := strings.Join(repos.env[repo], " "); !strings.Contains(env, "RESTIC_REST_USERNAME="+nodeRestUser) {
			t.Errorf("home made %s with %q, want the node's login", repo, env)
		}
	}
	kept, err := node.st.NodePlacements()
	if err != nil || len(kept) != 2 || kept[0].Repo != "rest:"+addr+"/bombvault/containers" || kept[0].CAPEM != string(served) {
		t.Fatalf("the node keeps %+v %v", kept, err)
	}
	creds, err := node.svc.placementCreds(kept[0])
	if err != nil || creds.RESTUser != nodeRestUser || creds.RESTPassword == "" {
		t.Errorf("the node signs in with %+v %v", creds, err)
	}

	settings, _ := home.st.GetSettings()
	signIn, err := home.svc.savedSignIn(settings, "node-"+id[:12])
	if err != nil || signIn != creds {
		t.Errorf("home signs in with %+v %v, want the node's login %+v", signIn, err, creds)
	}
	sources, _ := home.st.ListPullSources()
	if len(sources) != 2 {
		t.Fatalf("home fetches from %d sources, want one per section", len(sources))
	}
	for _, ps := range sources {
		password, _ := home.svc.openResticPassword(ps.ResticPasswordEnc)
		if ps.NodeID != id || ps.CredsRef != "node-"+id[:12] || !ps.Enabled || ps.NeedsPairing() || password != nodePassword ||
			ps.Repo != "rest:"+addr+"/bombvault/"+ps.Domain || ps.Cadence != nodePullCadence {
			t.Errorf("source %+v", ps)
		}
	}
	watched, _ := home.st.ListReceivedRepos()
	if len(watched) != 2 || watched[0].CredsRef != "node-"+id[:12] || !strings.HasPrefix(watched[0].Repo, "rest:"+addr) {
		t.Errorf("home watches %+v", watched)
	}
	if n, _ := home.st.GetNode(id); n.Placement != nodePlaceNode || n.PlacementTarget != addr {
		t.Errorf("node = %s %s, want it kept on the node at its address", n.Placement, n.PlacementTarget)
	}

	if code, out := home.do(t, http.MethodPost, "/api/nodes/"+id+"/placement", map[string]any{"kind": "node"}); code != http.StatusOK || out["ok"] != true {
		t.Fatalf("placing again at the same address = %d %v", code, out)
	}
	if sources, _ := home.st.ListPullSources(); len(sources) != 2 {
		t.Errorf("placing again fetches from %d sources, want the same two", len(sources))
	}

	if code, out := home.do(t, http.MethodDelete, "/api/nodes/"+id, nil); code != http.StatusOK || out["told"] != true {
		t.Fatalf("release = %d %v", code, out)
	}
	sources, _ = home.st.ListPullSources()
	if len(sources) != 2 || sources[0].Enabled || sources[1].Enabled {
		t.Errorf("after the release home fetches from %+v, want the sources kept and switched off", sources)
	}
}

// servesNodeRest has home reach the node's rest-server at a server on this
// machine that serves the certificate in hostData, as the one the node started
// there does.
func servesNodeRest(t *testing.T, home *instance, hostData string) {
	t.Helper()
	dir := filepath.Join(hostData, nodeRestFolder, nodeRestTLS)
	l, err := tls.Listen("tcp", "127.0.0.1:0", &tls.Config{
		MinVersion: tls.VersionTLS12,
		GetCertificate: func(*tls.ClientHelloInfo) (*tls.Certificate, error) {
			cert, err := tls.LoadX509KeyPair(filepath.Join(dir, "cert.pem"), filepath.Join(dir, "key.pem"))
			return &cert, err
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	srv := &http.Server{
		Handler:           http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusUnauthorized) }),
		ReadHeaderTimeout: time.Second,
		ErrorLog:          log.New(io.Discard, "", 0),
	}
	go srv.Serve(l) //nolint:errcheck // it ends when the test closes it
	t.Cleanup(func() { _ = srv.Close() })
	var d net.Dialer
	home.svc.nodeRestDial = func(ctx context.Context, network, _ string) (net.Conn, error) {
		return d.DialContext(ctx, network, l.Addr().String())
	}
}

// keepingHome is home with an adopted node that starts its rest-server on a
// Docker of its own.
func keepingHome(t *testing.T) (home *instance, node *instance, id string, repos *repoStore) {
	t.Helper()
	home = homeWithRelay(t)
	repos = newRepoStore()
	home.svc.engine = repos
	node, id, _ = adoptedNode(t, home, repos)
	node.svc.docker = &receiverDocker{imageID: "sha256:rest"}
	node.svc.cfg.HostMountRoot = t.TempDir()
	node.svc.cfg.HostSourceRoot = "/srv/data"
	node.engine.want = node.svc.backupPassword()
	return home, node, id, repos
}

func TestHomeGivesUpOnANodeRestServerThatTakesTheConnectionButNeverAnswers(t *testing.T) {
	reach := nodeRestReach
	nodeRestReach = 300 * time.Millisecond
	t.Cleanup(func() { nodeRestReach = reach })
	home, _, id, repos := keepingHome(t)
	silent, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = silent.Close() })
	held := make(chan net.Conn, 8)
	go func() {
		for {
			c, err := silent.Accept()
			if err != nil {
				return
			}
			held <- c
		}
	}()
	t.Cleanup(func() {
		for len(held) > 0 {
			_ = (<-held).Close()
		}
	})

	repos.mu.Lock()
	opened := repos.opened
	repos.mu.Unlock()
	addr := "https://" + silent.Addr().String()
	start := time.Now()
	code, out := home.do(t, http.MethodPost, "/api/nodes/"+id+"/placement", map[string]any{"kind": "node", "address": addr})
	if took := time.Since(start); took > 10*time.Second {
		t.Errorf("the placement took %s", took)
	}
	_, port, _ := net.SplitHostPort(silent.Addr().String())
	if code != http.StatusOK || out["ok"] != false || out["code"] != codeNodeRestUnreachable || out["address"] != addr || out["port"] != port {
		t.Fatalf("placement at a silent port = %d %v", code, out)
	}
	if msg, _ := out["error"].(string); !strings.Contains(msg, addr) || !strings.Contains(msg, "firewall") {
		t.Errorf("error %q does not say what to check", msg)
	}
	repos.mu.Lock()
	asked := repos.opened - opened
	repos.mu.Unlock()
	if asked != 0 {
		t.Errorf("home asked restic about %d repositories it cannot reach", asked)
	}
	if sources, _ := home.st.ListPullSources(); len(sources) != 0 {
		t.Errorf("home fetches from %+v", sources)
	}
	if n, _ := home.st.GetNode(id); n.Placement != "" {
		t.Errorf("the node is placed %q after a failed placement", n.Placement)
	}
}

func TestHomeRefusesARestServerWithAnotherCertificateThanTheNodeMade(t *testing.T) {
	home, _, id, _ := keepingHome(t)
	other := t.TempDir()
	certPEM, keyPEM, err := selfSignedFor("203.0.113.7")
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(other, nodeRestFolder, nodeRestTLS)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "cert.pem"), certPEM, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "key.pem"), keyPEM, 0o600); err != nil {
		t.Fatal(err)
	}
	servesNodeRest(t, home, other)

	start := time.Now()
	code, out := home.do(t, http.MethodPost, "/api/nodes/"+id+"/placement", map[string]any{"kind": "node", "address": "https://203.0.113.7:8443"})
	if took := time.Since(start); took > 5*time.Second {
		t.Errorf("home tried %s on a certificate that will not change", took)
	}
	if code != http.StatusOK || out["ok"] != false || out["code"] != codeNodeRestForeign || out["address"] != "https://203.0.113.7:8443" {
		t.Fatalf("placement behind another certificate = %d %v", code, out)
	}
}

// stuckRepos is storage whose every call waits until it is called off, as
// restic does while it retries a server that does not answer.
type stuckRepos struct{ *repoStore }

func (stuckRepos) RepoOpensErr(ctx context.Context, _ string, _ restic.Mode) error {
	<-ctx.Done()
	return ctx.Err()
}

func (stuckRepos) Init(ctx context.Context, _ string, _ restic.Mode) error {
	<-ctx.Done()
	return ctx.Err()
}

func TestHomeStopsWaitingForANodeRepositoryThatDoesNotAnswer(t *testing.T) {
	prepare := nodeRepoPrepare
	nodeRepoPrepare = 200 * time.Millisecond
	t.Cleanup(func() { nodeRepoPrepare = prepare })
	home, node, id, repos := keepingHome(t)
	servesNodeRest(t, home, node.svc.cfg.HostMountRoot)
	home.svc.engine = stuckRepos{repos}

	done := make(chan map[string]any, 1)
	go func() {
		_, out := home.do(t, http.MethodPost, "/api/nodes/"+id+"/placement", map[string]any{"kind": "node", "address": "https://203.0.113.7:8443"})
		done <- out
	}()
	select {
	case out := <-done:
		msg, _ := out["error"].(string)
		if out["ok"] != false || !strings.Contains(msg, "containers repository") || !strings.Contains(msg, "did not answer") {
			t.Errorf("placement with a repository that does not answer = %v", out)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("the placement still runs after 10 s")
	}
}
