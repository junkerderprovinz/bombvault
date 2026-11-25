package api

import (
	"encoding/json"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/keydeposit"
)

// nodeRouter stands in for a node's own routes: it answers with the origin
// the run history would record, and with the request it got.
func nodeRouter() http.Handler {
	mux := http.NewServeMux()
	answer := func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"ok": true, "via": runOriginFromContext(r.Context()).Via,
			"path": r.URL.Path, "query": r.URL.RawQuery,
		})
	}
	mux.HandleFunc("GET /api/status", answer)
	mux.HandleFunc("GET /api/runs", answer)
	mux.HandleFunc("GET /api/progress", answer)
	mux.HandleFunc("GET /api/browse", answer)
	mux.HandleFunc("POST /api/browse/mkdir", answer)
	mux.HandleFunc("POST /api/containers/{name}/backup", answer)
	mux.HandleFunc("GET /api/settings", answer)
	mux.HandleFunc("DELETE /api/containers/{name}", answer)
	mux.HandleFunc("GET /api/containers/{name}/snapshots", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("not json"))
	})
	return mux
}

func (n *nodeUnderTest) api(t *testing.T, req nodeAPIRequest) (int, nodeAPIAnswer) {
	t.Helper()
	w := n.call(t, keydeposit.ProofNodeAPI, nodeAPIPath, req)
	var ans nodeAPIAnswer
	_ = json.Unmarshal(w.Body.Bytes(), &ans)
	return w.Code, ans
}

func TestANodePassesOnlyItsListedRoutesToItsOwnRouter(t *testing.T) {
	n := newNodeUnderTest(t)
	n.node.svc.ServeNodeAPI(nodeRouter())
	status := nodeAPIRequest{Method: http.MethodGet, Path: "/api/status"}
	if code, _ := n.api(t, status); code != http.StatusConflict {
		t.Errorf("a request before the adoption = %d", code)
	}
	n.adopt(t)

	code, ans := n.api(t, nodeAPIRequest{Method: http.MethodGet, Path: "/api/runs", Query: "limit=5"})
	body, err := ans.body()
	if code != http.StatusOK || ans.Status != http.StatusOK || err != nil {
		t.Fatalf("GET /api/runs = %d %+v %v", code, ans, err)
	}
	var got map[string]any
	if err := json.Unmarshal(body, &got); err != nil || got["path"] != "/api/runs" || got["query"] != "limit=5" {
		t.Errorf("the node's router got %s", body)
	}
	_, ans = n.api(t, nodeAPIRequest{Method: http.MethodPost, Path: "/api/containers/plex/backup", Body: json.RawMessage(`{}`)})
	body, _ = ans.body()
	if !strings.Contains(string(body), `"via":"home"`) {
		t.Errorf("a backup home started is recorded as %s", body)
	}

	for name, req := range map[string]nodeAPIRequest{
		"settings":            {Method: http.MethodGet, Path: "/api/settings"},
		"a delete":            {Method: http.MethodDelete, Path: "/api/containers/plex"},
		"a climb":             {Method: http.MethodGet, Path: "/api/containers/../settings"},
		"another method":      {Method: http.MethodPut, Path: "/api/status"},
		"a path without /":    {Method: http.MethodGet, Path: "api/status"},
		"a full address":      {Method: http.MethodGet, Path: "//elsewhere/api/status"},
		"a query in a path":   {Method: http.MethodGet, Path: "/api/settings?x=/api/status"},
		"the progress stream": {Method: http.MethodGet, Path: "/api/progress"},
		"a fixed name":        {Method: http.MethodGet, Path: "/api/containers/backup-order"},
	} {
		if code, _ := n.api(t, req); code != http.StatusForbidden {
			t.Errorf("%s = %d, want 403", name, code)
		}
	}
	if code, _ := n.api(t, nodeAPIRequest{Method: http.MethodGet, Path: "/api/containers/plex/snapshots"}); code != http.StatusBadGateway {
		t.Errorf("an answer that is no JSON = %d, want 502", code)
	}

	if w := n.call(t, keydeposit.ProofNodeRelease, nodeReleasePath, nil); w.Code != http.StatusOK {
		t.Fatal(w.Body)
	}
	if code, _ := n.api(t, status); code != http.StatusConflict {
		t.Errorf("a request after the release = %d", code)
	}
}

func TestEveryNodeRouteIsARouteThatNeitherDeletesNorReachesSecrets(t *testing.T) {
	routes := registeredRoutes(t)
	for _, r := range nodeRouteShadows {
		if !slices.Contains(routes, r) {
			t.Errorf("nodeRouteShadows lists %s, which is no route", r)
		}
	}
	for _, r := range nodeRoutes {
		if !slices.Contains(routes, r) {
			t.Errorf("nodeRoutes lists %s, which is no route", r)
		}
		method, path, _ := strings.Cut(r, " ")
		if method == http.MethodDelete || method == http.MethodPut {
			t.Errorf("nodeRoutes lists %s", r)
		}
		for _, refused := range []string{"/api/settings", "/api/destinations", "/api/storage", "/api/sign-ins", "/api/key", "/api/backup-key", "/api/group", "/api/fortress", "/api/tokens", "/api/nodes", "/api/receiver", "/api/auth", "/mcp", "/api/v1"} {
			if strings.HasPrefix(path, refused) {
				t.Errorf("nodeRoutes lists %s", r)
			}
		}
	}
}

func TestANodeLetsHomeBrowseItsFoldersOnlyWhileAdopted(t *testing.T) {
	n := newNodeUnderTest(t)
	n.node.svc.ServeNodeAPI(nodeRouter())
	browse := nodeAPIRequest{Method: http.MethodGet, Path: "/api/browse", Query: "path=appdata"}
	if code, _ := n.api(t, browse); code != http.StatusConflict {
		t.Errorf("browsing before the adoption = %d", code)
	}
	n.adopt(t)
	stranger := newInstance(t, "stranger", strings.Repeat("c3", 32))
	if code := post(n.routes, nodeAPIPath, signedBodyForNode(t, stranger, keydeposit.ProofNodeAPI, n.id, "stranger", browse)); code != http.StatusForbidden {
		t.Errorf("browsing signed by another instance = %d", code)
	}
	code, ans := n.api(t, browse)
	body, _ := ans.body()
	if code != http.StatusOK || !strings.Contains(string(body), `"query":"path=appdata"`) {
		t.Errorf("browsing once adopted = %d %s", code, body)
	}
	if code, _ := n.api(t, nodeAPIRequest{Method: http.MethodPost, Path: "/api/browse/mkdir", Body: json.RawMessage(`{"path":"","name":"x"}`)}); code != http.StatusForbidden {
		t.Errorf("making a folder = %d, want 403", code)
	}
	if w := n.call(t, keydeposit.ProofNodeRelease, nodeReleasePath, nil); w.Code != http.StatusOK {
		t.Fatal(w.Body)
	}
	if code, _ := n.api(t, browse); code != http.StatusConflict {
		t.Errorf("browsing after the release = %d", code)
	}
}

func TestHomeBrowsesANodesFoldersButMakesNone(t *testing.T) {
	home := homeWithRelay(t)
	node, id, _ := adoptedNode(t, home, newRepoStore())
	node.svc.ServeNodeAPI(nodeRouter())

	code, out := home.do(t, http.MethodGet, "/api/nodes/"+id+"/api/browse?path=appdata", nil)
	if code != http.StatusOK || out["path"] != "/api/browse" || out["query"] != "path=appdata" {
		t.Errorf("browsing through home = %d %v", code, out)
	}
	if code, out := home.do(t, http.MethodPost, "/api/nodes/"+id+"/api/browse/mkdir", map[string]string{"path": "", "name": "x"}); code != http.StatusForbidden {
		t.Errorf("making a folder through home = %d %v", code, out)
	}
}

func TestHomeReachesANodesRoutesBehindItsOwnSignIn(t *testing.T) {
	home := homeWithRelay(t)
	repos := newRepoStore()
	node, id, _ := adoptedNode(t, home, repos)
	node.svc.ServeNodeAPI(nodeRouter())

	code, out := home.do(t, http.MethodGet, "/api/nodes/"+id+"/api/runs?limit=3", nil)
	if code != http.StatusOK || out["path"] != "/api/runs" || out["query"] != "limit=3" || out["via"] != "home" {
		t.Errorf("GET through home = %d %v", code, out)
	}
	if code, out := home.do(t, http.MethodGet, "/api/nodes/"+id+"/api/settings", nil); code != http.StatusForbidden {
		t.Errorf("settings through home = %d %v", code, out)
	}
	if code, out := home.do(t, http.MethodGet, "/api/nodes/unknown/api/status", nil); code != http.StatusNotFound {
		t.Errorf("an unknown node = %d %v", code, out)
	}
	code, out = home.do(t, http.MethodGet, "/api/nodes/routes", nil)
	listed, _ := json.Marshal(out["routes"])
	want, _ := json.Marshal(nodeRoutes)
	if code != http.StatusOK || string(listed) != string(want) {
		t.Errorf("routes = %d %s, want %s", code, listed, want)
	}

	hash, epoch, _ := home.h.phoneSigning()
	home.as(listedPhoneSession(t, home.st, home.appKey, hash, epoch), func() {
		if code, out := home.do(t, http.MethodGet, "/api/nodes/"+id+"/api/status", nil); code != http.StatusForbidden {
			t.Errorf("a paired phone = %d %v", code, out)
		}
	})
}
