package api

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/keydeposit"
	"github.com/junkerderprovinz/bombvault/internal/nodelink"
	"github.com/junkerderprovinz/bombvault/internal/restickey"
	"github.com/junkerderprovinz/bombvault/internal/secret"
	"github.com/junkerderprovinz/bombvault/internal/store"
	"github.com/junkerderprovinz/bombvault/internal/trust"
)

// selfSignedPEM is a certificate such as a reverse proxy in front of the
// receiver might serve.
func selfSignedPEM(t *testing.T) string {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "backup.example.org"}, NotBefore: time.Now(), NotAfter: time.Now().Add(time.Hour)}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
}

// nodeUnderTest is a node with its routes and home, and the id home calls
// it by, without a relay between them.
type nodeUnderTest struct {
	home, node *instance
	id         string
	routes     http.Handler
}

func newNodeUnderTest(t *testing.T) *nodeUnderTest {
	t.Helper()
	home := homeWithRelay(t)
	compose := addNode(t, home, "vps")
	id, _ := nodelink.NodeID(compose["APP_KEY"])
	node := newInstance(t, "vps", compose["APP_KEY"])
	node.svc.cfg.Mode = config.ModeNode
	h, err := nodelink.DecodeHome(compose["APP_KEY"], compose["NODE_HOME"])
	if err != nil {
		t.Fatal(err)
	}
	routes, err := node.svc.homeRoutes(h, "vps")
	if err != nil {
		t.Fatal(err)
	}
	return &nodeUnderTest{home: home, node: node, id: id, routes: routes}
}

func (n *nodeUnderTest) call(t *testing.T, action, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	n.routes.ServeHTTP(w, httptest.NewRequest(http.MethodPost, path, bytes.NewReader(signedBodyForNode(t, n.home, action, n.id, "home", body))))
	return w
}

func (n *nodeUnderTest) adopt(t *testing.T) {
	t.Helper()
	if w := n.call(t, keydeposit.ProofNodeAdopt, nodeAdoptPath, nil); w.Code != http.StatusOK {
		t.Fatalf("adopt = %d %s", w.Code, w.Body)
	}
}

func TestANodeTakesItsPlacementOnlyWhileAdopted(t *testing.T) {
	n := newNodeUnderTest(t)
	placements := []nodePlacement{{Domain: "containers", Repo: "rest:https://backup.example.org/vps/containers"}}
	if w := n.call(t, keydeposit.ProofNodePlacement, nodePlacementPath, placements); w.Code != http.StatusConflict || !bytes.Contains(w.Body.Bytes(), []byte(codeNodeNotAdopted)) {
		t.Errorf("a placement before the adoption = %d %s", w.Code, w.Body)
	}
	n.adopt(t)
	if w := n.call(t, keydeposit.ProofNodePlacement, nodePlacementPath, placements); w.Code != http.StatusOK {
		t.Errorf("a placement once adopted = %d %s", w.Code, w.Body)
	}
	if w := n.call(t, keydeposit.ProofNodeRelease, nodeReleasePath, nil); w.Code != http.StatusOK {
		t.Fatalf("release = %d %s", w.Code, w.Body)
	}
	if w := n.call(t, keydeposit.ProofNodePlacement, nodePlacementPath, placements); w.Code != http.StatusConflict || !bytes.Contains(w.Body.Bytes(), []byte(codeNodeReleased)) {
		t.Errorf("a placement after the release = %d %s", w.Code, w.Body)
	}
}

func TestANodeKeepsItsPlacementSealedAndSaysWhetherItGetsThere(t *testing.T) {
	n := newNodeUnderTest(t)
	n.adopt(t)
	n.node.engine.want = n.node.svc.backupPassword()
	cert := selfSignedPEM(t)
	placements := []nodePlacement{
		{Domain: "containers", Repo: "rest:https://backup.example.org/vps/containers", Creds: CloudCreds{RESTUser: "vps", RESTPassword: "rest-secret"}, CAPEM: cert},
		{Domain: "files", Repo: "rest:https://backup.example.org/vps/files", Creds: CloudCreds{RESTUser: "vps", RESTPassword: "rest-secret"}},
	}
	w := n.call(t, keydeposit.ProofNodePlacement, nodePlacementPath, placements)
	var ans nodePlacementAnswer
	if err := json.Unmarshal(w.Body.Bytes(), &ans); err != nil || w.Code != http.StatusOK || len(ans.Reach) != 2 || !ans.Reach[0].Reached || !ans.Reach[1].Reached {
		t.Fatalf("placement = %d %s", w.Code, w.Body)
	}
	kept, err := n.node.st.NodePlacements()
	if err != nil || len(kept) != 2 || kept[0].Repo != placements[0].Repo || kept[0].CAPEM != cert {
		t.Fatalf("kept = %+v %v", kept, err)
	}
	if bytes.Contains(kept[0].CredsEnc, []byte("rest-secret")) {
		t.Error("the sign-in is kept in the clear")
	}
	if got, err := os.ReadFile(filepath.Join(trust.Dir(n.node.svc.cfg.DataDir), "node-containers.pem")); err != nil || string(got) != cert {
		t.Errorf("certificate on disk = %q %v", got, err)
	}

	n.node.engine.want = "another password"
	w = n.call(t, keydeposit.ProofNodePlacement, nodePlacementPath, placements[1:])
	if err := json.Unmarshal(w.Body.Bytes(), &ans); err != nil || len(ans.Reach) != 1 || ans.Reach[0].Reached || ans.Reach[0].Detail == "" {
		t.Errorf("a repository the node does not open = %s", w.Body)
	}
}

func TestANodeBacksUpAPlacedSectionWithTheSignInHomeSentAlone(t *testing.T) {
	n := newNodeUnderTest(t)
	n.adopt(t)
	svc := n.node.svc
	n.node.engine.want = svc.backupPassword()
	placed := nodePlacement{Domain: "containers", Repo: "rest:https://backup.example.org/vps/containers", Creds: CloudCreds{RESTUser: "vps", RESTPassword: "rest-secret"}}
	if w := n.call(t, keydeposit.ProofNodePlacement, nodePlacementPath, []nodePlacement{placed}); w.Code != http.StatusOK {
		t.Fatalf("placement = %d %s", w.Code, w.Body)
	}
	settings, err := n.node.st.MutateSettings(func(s *store.Settings) error {
		s.ContainersPath, s.FilesPath = "backups/containers", "backups/files"
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	repo, err := svc.containerRepoForName(settings, "plex", "local")
	if err != nil || repo != placed.Repo {
		t.Fatalf("the container backs up to %q %v, want %q", repo, err, placed.Repo)
	}
	mode := svc.repoModeFor(settings, "containers", "local", repo)
	if !mode.NoAmbientCreds || !slices.Contains(mode.Env, "RESTIC_REST_PASSWORD=rest-secret") || mode.Password != restickey.Derive(n.node.appKey) {
		t.Errorf("the placed repository opens with %+v", mode)
	}
	files, err := svc.repoFor(settings, "files", "local")
	if want, _ := svc.resolveRepo("backups/files"); err != nil || files != want {
		t.Errorf("the section home placed nowhere backs up to %q %v, want its own folder %q", files, err, want)
	}
	if mode := svc.repoModeFor(settings, "files", "local", files); mode.NoAmbientCreds {
		t.Errorf("the section home placed nowhere opens with %+v", mode)
	}
}

func TestANodeRefusesAPlacementItCannotUse(t *testing.T) {
	n := newNodeUnderTest(t)
	n.adopt(t)
	for name, ps := range map[string][]nodePlacement{
		"another section":   {{Domain: "vms", Repo: "rest:https://backup.example.org/vps/vms"}},
		"a folder here":     {{Domain: "files", Repo: "/mnt/user/backups"}},
		"an rclone option":  {{Domain: "files", Repo: "rclone:-o=x:f"}},
		"a section twice":   {{Domain: "files", Repo: "rest:https://a.example.org/f"}, {Domain: "files", Repo: "rest:https://b.example.org/f"}},
		"no certificate":    {{Domain: "files", Repo: "rest:https://a.example.org/f", CAPEM: "not a certificate"}},
		"a key, not a cert": {{Domain: "files", Repo: "rest:https://a.example.org/f", CAPEM: "-----BEGIN PRIVATE KEY-----\nAAAA\n-----END PRIVATE KEY-----\n"}},
	} {
		if w := n.call(t, keydeposit.ProofNodePlacement, nodePlacementPath, ps); w.Code != http.StatusBadRequest {
			t.Errorf("%s = %d %s", name, w.Code, w.Body)
		}
	}
	if kept, _ := n.node.st.NodePlacements(); len(kept) != 0 {
		t.Errorf("kept %+v from refused placements", kept)
	}
}

func TestANodeBacksUpUnderItsOwnAPPKeyWhateverKeyItWasGiven(t *testing.T) {
	appKey := strings.Repeat("e5", 32)
	node := newInstance(t, "vps", appKey)
	node.svc.cfg.Mode = config.ModeNode
	other := strings.Repeat("f6", 32)
	enc, err := secret.Encrypt(appKey, []byte(other))
	if err != nil {
		t.Fatal(err)
	}
	if err := node.st.SetAdoptedKey(enc, "another server", true); err != nil {
		t.Fatal(err)
	}
	node.svc.forgetBackupKey()
	if got, want := node.svc.backupPassword(), restickey.Derive(appKey); got != want {
		t.Errorf("a node backs up with %s, want the password of its own APP_KEY %s", got, want)
	}
}
