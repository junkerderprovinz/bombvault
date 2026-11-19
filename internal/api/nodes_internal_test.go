package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/keydeposit"
	"github.com/junkerderprovinz/bombvault/internal/nodelink"
	"github.com/junkerderprovinz/bombvault/internal/store"
	"github.com/junkerderprovinz/parleyport/kit/relay"
)

// homeWithRelay is an instance whose nodes use an in-process relay.
func homeWithRelay(t *testing.T) *instance {
	t.Helper()
	rs := httptest.NewServer(relay.New())
	t.Cleanup(rs.Close)
	t.Cleanup(rs.CloseClientConnections)
	home := newInstance(t, "home", strings.Repeat("a1", 32))
	t.Cleanup(home.svc.CloseNodeLinks)
	if err := home.st.SetGroupRelay("own", rs.URL, false); err != nil {
		t.Fatal(err)
	}
	return home
}

// addNode adds a node on home through the API and returns the values its
// compose file takes.
func addNode(t *testing.T, home *instance, name string) map[string]string {
	t.Helper()
	code, out := home.do(t, http.MethodPost, "/api/nodes", map[string]any{"name": name})
	if code != http.StatusOK || out["ok"] != true {
		t.Fatalf("add node: %d %v", code, out)
	}
	raw, _ := json.Marshal(out["compose"])
	var compose map[string]string
	if err := json.Unmarshal(raw, &compose); err != nil {
		t.Fatal(err)
	}
	return compose
}

// startNode boots an instance as the node compose describes.
func startNode(t *testing.T, compose map[string]string) *instance {
	t.Helper()
	node := newInstance(t, compose["NODE_NAME"], compose["APP_KEY"])
	t.Cleanup(node.svc.CloseHomeLink)
	home, err := nodelink.DecodeHome(compose["APP_KEY"], compose["NODE_HOME"])
	if err != nil {
		t.Fatal(err)
	}
	if err := node.svc.OpenHomeLink(home, compose["NODE_NAME"]); err != nil {
		t.Fatal(err)
	}
	return node
}

func eventually(t *testing.T, what string, ok func() bool) {
	t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	for !ok() {
		if time.Now().After(deadline) {
			t.Fatalf("never: %s", what)
		}
		time.Sleep(25 * time.Millisecond)
	}
}

func TestHomeAddsANodeThatTakesItAsHomeAndLetsItGo(t *testing.T) {
	home := homeWithRelay(t)
	compose := addNode(t, home, "vps")
	if compose["MODE"] != "node" || len(compose["APP_KEY"]) != 64 || !strings.HasPrefix(compose["NODE_HOME"], "bvhome1.") || compose["NODE_NAME"] != "vps" {
		t.Fatalf("compose = %v", compose)
	}
	id, err := nodelink.NodeID(compose["APP_KEY"])
	if err != nil {
		t.Fatal(err)
	}
	node := startNode(t, compose)

	eventually(t, "home to record the adoption", func() bool {
		n, err := home.st.GetNode(id)
		return err == nil && n.AdoptedAt != 0
	})
	link, err := node.st.GetNodeLink()
	if err != nil || link.HomeID != home.id(t) || link.AdoptedAt == 0 {
		t.Errorf("the node's link = %+v, %v", link, err)
	}
	code, out := home.do(t, http.MethodGet, "/api/nodes", nil)
	listed, _ := json.Marshal(out)
	if code != http.StatusOK || !bytes.Contains(listed, []byte(id)) || !bytes.Contains(listed, []byte(`"adoptedAt"`)) {
		t.Errorf("list = %d %s", code, listed)
	}
	if bytes.Contains(listed, []byte(compose["APP_KEY"])) {
		t.Error("the list shows the node's APP_KEY")
	}

	code, out = home.do(t, http.MethodDelete, "/api/nodes/"+id, nil)
	if code != http.StatusOK || out["ok"] != true || out["told"] != true {
		t.Fatalf("release = %d %v", code, out)
	}
	if link, _ := node.st.GetNodeLink(); link.ReleasedAt == 0 {
		t.Errorf("the node did not hear it was let go: %+v", link)
	}
	if nodes, _ := home.st.ListNodes(); len(nodes) != 0 {
		t.Errorf("home still keeps %+v", nodes)
	}
}

func TestAddingANodeNeedsALoginPasswordAndAName(t *testing.T) {
	home := homeWithRelay(t)
	if code, out := home.do(t, http.MethodPost, "/api/nodes", map[string]any{"name": " "}); code != http.StatusBadRequest {
		t.Errorf("a node without a name = %d %v", code, out)
	}
	if code, out := home.do(t, http.MethodPost, "/api/nodes", map[string]any{"name": "a\nb"}); code != http.StatusBadRequest {
		t.Errorf("a name over two lines = %d %v", code, out)
	}
	if _, err := home.st.MutateSettings(func(s *store.Settings) error { s.AuthPasswordHash = ""; return nil }); err != nil {
		t.Fatal(err)
	}
	if code, out := home.do(t, http.MethodPost, "/api/nodes", map[string]any{"name": "vps"}); code != http.StatusForbidden || out["code"] != codeNoPassword {
		t.Errorf("without a login password = %d %v", code, out)
	}
}

// signedForNode is a call home signs for the node with id.
func signedForNode(t *testing.T, home *instance, action, id, name string) []byte {
	t.Helper()
	p, err := home.svc.signCall(action, id, callPayload(name))
	if err != nil {
		t.Fatal(err)
	}
	body, err := json.Marshal(nodeCall{peerProof: p, Name: name})
	if err != nil {
		t.Fatal(err)
	}
	return body
}

func post(h http.Handler, path string, body []byte) int {
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body)))
	return w.Code
}

func TestANodeTakesEachSignedCallOnceAndOnlyFromItsHome(t *testing.T) {
	home := homeWithRelay(t)
	compose := addNode(t, home, "vps")
	id, _ := nodelink.NodeID(compose["APP_KEY"])
	node := newInstance(t, "vps", compose["APP_KEY"])
	h, err := nodelink.DecodeHome(compose["APP_KEY"], compose["NODE_HOME"])
	if err != nil {
		t.Fatal(err)
	}
	routes, err := node.svc.homeRoutes(h, "vps")
	if err != nil {
		t.Fatal(err)
	}

	stranger := newInstance(t, "stranger", strings.Repeat("c3", 32))
	if code := post(routes, nodeAdoptPath, signedForNode(t, stranger, keydeposit.ProofNodeAdopt, id, "stranger")); code != http.StatusForbidden {
		t.Errorf("an adoption signed by another instance = %d", code)
	}
	if code := post(routes, nodeAdoptPath, signedForNode(t, home, keydeposit.ProofNodeAdopt, "another-node", "home")); code != http.StatusForbidden {
		t.Errorf("an adoption signed for another node = %d", code)
	}
	if code := post(routes, nodeAdoptPath, signedForNode(t, home, keydeposit.ProofNodeRelease, id, "")); code != http.StatusForbidden {
		t.Errorf("a release passed off as an adoption = %d", code)
	}
	adopt := signedForNode(t, home, keydeposit.ProofNodeAdopt, id, "home")
	if code := post(routes, nodeAdoptPath, adopt); code != http.StatusOK {
		t.Fatalf("home's adoption = %d", code)
	}
	if code := post(routes, nodeAdoptPath, adopt); code != http.StatusForbidden {
		t.Errorf("the same adoption sent again = %d", code)
	}
	if code := post(routes, nodeReleasePath, signedForNode(t, home, keydeposit.ProofNodeRelease, id, "")); code != http.StatusOK {
		t.Fatalf("home's release = %d", code)
	}
	if code := post(routes, nodeAdoptPath, signedForNode(t, home, keydeposit.ProofNodeAdopt, id, "home")); code != http.StatusConflict {
		t.Errorf("an adoption after the release = %d, want 409", code)
	}
	if code := post(routes, "/node/elsewhere", nil); code != http.StatusNotFound {
		t.Errorf("an unknown call = %d", code)
	}
}

func TestANodeRefusesAnotherHomeOnceAdopted(t *testing.T) {
	home := homeWithRelay(t)
	compose := addNode(t, home, "vps")
	node := newInstance(t, "vps", compose["APP_KEY"])
	h, err := nodelink.DecodeHome(compose["APP_KEY"], compose["NODE_HOME"])
	if err != nil {
		t.Fatal(err)
	}
	other := newInstance(t, "other", strings.Repeat("d4", 32))
	seed, err := other.svc.depositSeed()
	if err != nil {
		t.Fatal(err)
	}
	otherKey := keydeposit.OwnerKey(seed)

	// Before an adoption the home NODE_HOME names may change.
	if _, err := node.svc.homeRoutes(nodelink.Home{ID: h.ID, Key: otherKey, Relay: h.Relay, RelayURL: h.RelayURL}, "vps"); err != nil {
		t.Errorf("a new key for home before the adoption: %v", err)
	}
	if _, err := node.svc.homeRoutes(h, "vps"); err != nil {
		t.Fatalf("home's own key again: %v", err)
	}
	if err := node.st.SetNodeLinkAdopted(h.ID, 1); err != nil {
		t.Fatal(err)
	}
	if _, err := node.svc.homeRoutes(nodelink.Home{ID: "other", Key: otherKey, Relay: h.Relay, RelayURL: h.RelayURL}, "vps"); err == nil {
		t.Error("an adopted node took another home")
	}
	if _, err := node.svc.homeRoutes(nodelink.Home{ID: h.ID, Key: otherKey, Relay: h.Relay, RelayURL: h.RelayURL}, "vps"); err == nil {
		t.Error("an adopted node took another key for its home")
	}
}

func TestHomeAnswersItsNodesNothingButHello(t *testing.T) {
	home := homeWithRelay(t)
	routes := home.svc.answersForNodes()
	w := httptest.NewRecorder()
	routes.ServeHTTP(w, httptest.NewRequest(http.MethodPost, nodeHelloPath, nil))
	var hello nodeHello
	if err := json.Unmarshal(w.Body.Bytes(), &hello); err != nil || w.Code != http.StatusOK || hello.ID != home.id(t) || hello.Name != "home" {
		t.Errorf("hello = %d %s", w.Code, w.Body)
	}
	for _, path := range []string{nodeAdoptPath, nodeReleasePath, "/api/status"} {
		if code := post(routes, path, nil); code != http.StatusNotFound {
			t.Errorf("%s = %d, want 404", path, code)
		}
	}
}
