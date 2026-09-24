package api_test

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/api"
	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/notify"
	"github.com/junkerderprovinz/bombvault/internal/secret"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

func TestMCPKeyCreateShowsKeyOnce(t *testing.T) {
	st := newMemStore(t)
	h, _ := newMCPKeyRouter(t, st, mcpAppKey)

	w, m := doMCPKey(t, h, http.MethodPost, "/api/mcp/keys", `{"label":"Claude Code laptop","canStartBackups":true}`)
	if w.Code != http.StatusOK || m["ok"] != true {
		t.Fatalf("create: status=%d body=%v", w.Code, m)
	}
	key, _ := m["key"].(string)
	if !strings.HasPrefix(key, secret.MCPKeyPrefix) || len(key) != 49 {
		t.Fatalf("key %q is not a 49-character bvmcp_ key", key)
	}
	if got := w.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("Cache-Control = %q, want no-store", got)
	}
	item, _ := m["item"].(map[string]any)
	id, _ := item["id"].(string)

	row, err := st.GetMCPKey(id)
	if err != nil {
		t.Fatalf("stored key: %v", err)
	}
	if row.Digest != secret.HashMCPKey(mcpAppKey, key) {
		t.Fatal("the stored digest is not the peppered HMAC of the key that was handed out")
	}
	if row.Check != secret.MCPKeyCheck(mcpAppKey, id) {
		t.Fatal("the stored check value is not derived from the id under this APP_KEY")
	}
	if row.Hint != key[len(key)-4:] {
		t.Fatalf("hint = %q, want the last four characters of the key", row.Hint)
	}

	w2, _ := doMCPKey(t, h, http.MethodGet, "/api/mcp/keys", "")
	body := w2.Body.String()
	if strings.Contains(body, key) || strings.Contains(body, `"key"`) {
		t.Fatalf("the listing hands out the key again: %s", body)
	}
	if !strings.Contains(body, row.Hint) {
		t.Fatalf("the listing does not carry the hint: %s", body)
	}
}

func TestMCPKeyCreateValidationCodes(t *testing.T) {
	st := newMemStore(t)
	h, _ := newMCPKeyRouter(t, st, mcpAppKey)

	for _, tc := range []struct {
		name string
		body string
	}{
		{"empty", `{"label":"   "}`},
		{"too long", fmt.Sprintf(`{"label":%q}`, strings.Repeat("x", 65))},
		{"control character", `{"label":"lap\u0001top"}`},
	} {
		_, m := doMCPKey(t, h, http.MethodPost, "/api/mcp/keys", tc.body)
		if m["code"] != "mcp-key-label-invalid" {
			t.Fatalf("%s label: code = %v, want mcp-key-label-invalid", tc.name, m["code"])
		}
	}

	createMCPKey(t, h, "Laptop", true)
	_, m := doMCPKey(t, h, http.MethodPost, "/api/mcp/keys", `{"label":"laptop"}`)
	if m["code"] != "mcp-key-label-taken" {
		t.Fatalf("duplicate label: code = %v, want mcp-key-label-taken", m["code"])
	}

	for i := 2; i <= store.MCPKeyLimit; i++ {
		createMCPKey(t, h, fmt.Sprintf("Client %d", i), true)
	}
	_, m = doMCPKey(t, h, http.MethodPost, "/api/mcp/keys", `{"label":"One too many"}`)
	if m["code"] != "mcp-key-limit" {
		t.Fatalf("key %d: code = %v, want mcp-key-limit", store.MCPKeyLimit+1, m["code"])
	}

	_, m = doMCPKey(t, h, http.MethodPost, "/api/mcp/keys", `{"label":"Unknown field","scopes":["all"]}`)
	if m["ok"] != false {
		t.Fatalf("an unknown field must be refused, got %v", m)
	}
}

func TestMCPKeyCreateHostRuleWithoutPassword(t *testing.T) {
	st := newMemStore(t)
	h, _ := newMCPKeyRouter(t, st, mcpAppKey)

	allowed := []string{"192.168.1.10:3443", "[fd00::1]:3443", "localhost", "tower", "tower.local", "nas.home.arpa", "bv.tail1234.ts.net"}
	for i, host := range allowed {
		w, m := doMCPKeyJSON(t, h, http.MethodPost, "/api/mcp/keys",
			fmt.Sprintf(`{"label":"Client %d"}`, i), host, "")
		if w.Code != http.StatusOK || m["ok"] != true {
			t.Fatalf("host %q must be able to create a key: status=%d body=%v", host, w.Code, m)
		}
		_, list := doMCPKeyJSON(t, h, http.MethodGet, "/api/mcp/keys", "", host, "")
		if list["hostAllowsKeys"] != true {
			t.Fatalf("host %q: hostAllowsKeys must be true", host)
		}
	}

	before, err := st.ActiveMCPKeys()
	if err != nil {
		t.Fatal(err)
	}
	for _, host := range []string{"bombvault.example.com", "rebind.attacker.net"} {
		_, m := doMCPKeyJSON(t, h, http.MethodPost, "/api/mcp/keys", `{"label":"From the web"}`, host, "")
		if m["code"] != "mcp-key-needs-password" {
			t.Fatalf("host %q: code = %v, want mcp-key-needs-password", host, m["code"])
		}
		after, err := st.ActiveMCPKeys()
		if err != nil {
			t.Fatal(err)
		}
		if len(after) != len(before) {
			t.Fatalf("host %q: a refused create still wrote a row (%d -> %d)", host, len(before), len(after))
		}

		_, rm := doMCPKeyJSON(t, h, http.MethodPost, "/api/mcp/keys/"+before[0].ID+"/rotate", "", host, "")
		if rm["code"] != "mcp-key-needs-password" {
			t.Fatalf("host %q: rotate code = %v, want mcp-key-needs-password", host, rm["code"])
		}

		// A page that rebound its DNS to this address could otherwise spend the
		// certificate's sixteen names, and nothing in the product takes one back.
		_, cm := doMCPKeyJSON(t, h, http.MethodPost, "/api/mcp/certificate/names", `{"host":"1.2.3.4"}`, host, "")
		if cm["code"] != "mcp-key-needs-password" {
			t.Fatalf("host %q: certificate name code = %v, want mcp-key-needs-password", host, cm["code"])
		}

		w, lm := doMCPKeyJSON(t, h, http.MethodGet, "/api/mcp/keys", "", host, "")
		if w.Code != http.StatusOK || lm["hostAllowsKeys"] != false {
			t.Fatalf("host %q: hostAllowsKeys = %v, want false", host, lm["hostAllowsKeys"])
		}
	}
}

func TestMCPKeyCreateHostRuleOffWithPassword(t *testing.T) {
	st := newMemStore(t)
	h, _ := newMCPKeyRouter(t, st, mcpAppKey)
	cookie := enableLogin(t, st, mcpAppKey)

	w, m := doMCPKeyJSON(t, h, http.MethodPost, "/api/mcp/keys", `{"label":"Behind a proxy"}`, "bombvault.example.com", cookie)
	if w.Code != http.StatusOK || m["ok"] != true {
		t.Fatalf("a public host name with a login password must create a key: status=%d body=%v", w.Code, m)
	}
	_, lm := doMCPKeyJSON(t, h, http.MethodGet, "/api/mcp/keys", "", "bombvault.example.com", cookie)
	if lm["hostAllowsKeys"] != true {
		t.Fatalf("hostAllowsKeys = %v, want true once a login password exists", lm["hostAllowsKeys"])
	}
}

func TestMCPKeyUnusableAfterAppKeyChange(t *testing.T) {
	st := newMemStore(t)
	before, _ := newMCPKeyRouter(t, st, mcpAppKey)
	oldKey, id := createMCPKey(t, before, "Laptop", true)

	after, _ := newMCPKeyRouter(t, st, mcpAppKeyAfter)
	rows := mcpKeyRows(t, mcpKeyList(t, after), "keys")
	if len(rows) != 1 || rows[0]["unusable"] != "app-key-changed" {
		t.Fatalf("a key from another APP_KEY must be reported unusable, got %v", rows)
	}

	cookie := enableLogin(t, st, mcpAppKeyAfter)
	bundle := getRaw(t, after, "/api/diagnostics", &http.Cookie{Name: "bv_session", Value: cookie}) //nolint:gosec // G124: request cookie; Secure and HttpOnly only apply to responses
	if bundle.Code != http.StatusOK {
		t.Fatalf("diagnostics: status = %d body = %s", bundle.Code, bundle.Body.String())
	}
	if manifest := zipMembers(t, bundle.Body.Bytes())["manifest.json"]; !strings.Contains(manifest, `"unusableKeys": 1`) {
		t.Fatalf("the bundle does not count the key an APP_KEY change broke: %s", manifest)
	}

	_, m := doMCPKeyJSON(t, after, http.MethodPost, "/api/mcp/keys/"+id+"/rotate", "", mcpTestHost, cookie)
	if m["ok"] != true {
		t.Fatalf("rotate under the new APP_KEY: %v", m)
	}
	newKey, _ := m["key"].(string)
	if newKey == oldKey {
		t.Fatal("rotate handed out the same key")
	}
	_, list := doMCPKeyJSON(t, after, http.MethodGet, "/api/mcp/keys", "", mcpTestHost, cookie)
	rows = mcpKeyRows(t, list, "keys")
	if rows[0]["unusable"] != "" {
		t.Fatalf("unusable = %v, want it cleared after a rotate", rows[0]["unusable"])
	}
	row, err := st.GetMCPKey(id)
	if err != nil {
		t.Fatal(err)
	}
	if row.Digest != secret.HashMCPKey(mcpAppKeyAfter, newKey) {
		t.Fatal("the rotated digest is not peppered with the current APP_KEY")
	}
}

func TestMCPKeyChangesNotify(t *testing.T) {
	var mu sync.Mutex
	var sent []string
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		var body struct {
			Title   string `json:"title"`
			Message string `json:"message"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		mu.Lock()
		sent = append(sent, body.Title+" "+body.Message)
		mu.Unlock()
	}))
	defer srv.Close()

	st := newMemStore(t)
	h, svc := newMCPKeyRouter(t, st, mcpAppKey)
	if err := svc.SetNotifyConfig(notify.Config{On: "always", WebhookEnabled: true, WebhookURL: srv.URL}); err != nil {
		t.Fatal(err)
	}

	// The notifications go out beside their responses, so a count is only
	// settled once the messages have arrived.
	waitForMessages := func(n int) []string {
		t.Helper()
		deadline := time.Now().Add(5 * time.Second)
		for {
			mu.Lock()
			got := append([]string(nil), sent...)
			mu.Unlock()
			if len(got) >= n || time.Now().After(deadline) {
				return got
			}
			time.Sleep(5 * time.Millisecond)
		}
	}

	_, id := createMCPKey(t, h, "Laptop", true)
	steps := []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{"rotate", http.MethodPost, "/api/mcp/keys/" + id + "/rotate", ""},
		{"permission", http.MethodPatch, "/api/mcp/keys/" + id, `{"canStartBackups":false}`},
		{"revoke", http.MethodPost, "/api/mcp/keys/" + id + "/revoke", ""},
	}
	for _, s := range steps {
		if _, m := doMCPKey(t, h, s.method, s.path, s.body); m["ok"] != true {
			t.Fatalf("%s: %v", s.name, m)
		}
	}

	got := waitForMessages(len(steps) + 1)
	if len(got) != len(steps)+1 {
		t.Fatalf("want one message per change, got %d: %v", len(got), got)
	}
	row, err := st.GetMCPKey(id)
	if err != nil {
		t.Fatal(err)
	}
	for _, msg := range got {
		for _, want := range []string{"Laptop", "192.0.2.1"} {
			if !strings.Contains(msg, want) {
				t.Fatalf("message %q does not name %q", msg, want)
			}
		}
	}
	revoked := ""
	for _, msg := range got {
		if strings.Contains(msg, "MCP key revoked") {
			revoked = msg
		}
	}
	if !strings.Contains(revoked, row.Hint) {
		t.Fatalf("the revoke message %q does not carry the hint %q", revoked, row.Hint)
	}

	if err := svc.SetNotifyConfig(notify.Config{On: "never", WebhookEnabled: true, WebhookURL: srv.URL}); err != nil {
		t.Fatal(err)
	}
	_, quiet := createMCPKey(t, h, "Desktop", true)

	// One more change that does notify, so the silent one is settled by a
	// message that arrived rather than by waiting for one that never does.
	if err := svc.SetNotifyConfig(notify.Config{On: "always", WebhookEnabled: true, WebhookURL: srv.URL}); err != nil {
		t.Fatal(err)
	}
	if _, m := doMCPKey(t, h, http.MethodPost, "/api/mcp/keys/"+quiet+"/revoke", ""); m["ok"] != true {
		t.Fatalf("revoke the second key: %v", m)
	}
	after := waitForMessages(len(got) + 1)
	if len(after) != len(got)+1 {
		t.Fatalf("notifications set to never still sent a message: %v", after[len(got):])
	}
}

// Behind a trusted reverse proxy every request comes from the proxy, so the
// notification has to name the client the proxy forwarded, as the key's
// lastUsedFrom does.
func TestMCPKeyChangeNotificationNamesTheClientBehindAProxy(t *testing.T) {
	messages := make(chan string, 4)
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		var body struct {
			Message string `json:"message"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		messages <- body.Message
	}))
	defer srv.Close()

	_, proxies, err := net.ParseCIDR("192.0.2.0/24")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if _, _, err := api.EnsureSelfSigned(dir); err != nil {
		t.Fatal(err)
	}
	h, svc := mcpRouterWith(t, newMemStore(t), config.Config{
		AppKey: mcpAppKey, DataDir: dir, HostMountRoot: dir, TrustedProxies: []net.IPNet{*proxies},
	})
	if err := svc.SetNotifyConfig(notify.Config{On: "always", WebhookEnabled: true, WebhookURL: srv.URL}); err != nil {
		t.Fatal(err)
	}

	r := httptest.NewRequest(http.MethodPost, "/api/mcp/keys", strings.NewReader(`{"label":"Laptop"}`))
	r.Header.Set("Content-Type", "application/json")
	r.Host = mcpTestHost
	r.Header.Set("Origin", "https://"+mcpTestHost)
	r.Header.Set("X-Forwarded-For", "198.51.100.7")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("create: status=%d body=%s", w.Code, w.Body)
	}

	select {
	case msg := <-messages:
		if !strings.Contains(msg, "198.51.100.7") {
			t.Fatalf("the notification reads %q, want it to name the forwarded client", msg)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("no notification was sent")
	}
}

func TestMCPKeyEndpointsAreSessionProtected(t *testing.T) {
	st := newMemStore(t)
	h, _ := newMCPKeyRouter(t, st, mcpAppKey)
	_, id := createMCPKey(t, h, "Laptop", true)
	cookie := enableLogin(t, st, mcpAppKey)

	routes := []struct {
		method string
		path   string
		body   string
	}{
		{http.MethodGet, "/api/mcp/keys", ""},
		{http.MethodPost, "/api/mcp/keys", `{"label":"New"}`},
		{http.MethodPatch, "/api/mcp/keys/" + id, `{"canStartBackups":false}`},
		{http.MethodPost, "/api/mcp/keys/" + id + "/rotate", ""},
		{http.MethodPost, "/api/mcp/keys/" + id + "/revoke", ""},
		{http.MethodDelete, "/api/mcp/keys/" + id, ""},
	}
	for _, r := range routes {
		w, _ := doMCPKey(t, h, r.method, r.path, r.body)
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("%s %s without a session: status = %d, want 401", r.method, r.path, w.Code)
		}
	}
	w, m := doMCPKeyJSON(t, h, http.MethodGet, "/api/mcp/keys", "", mcpTestHost, cookie)
	if w.Code != http.StatusOK || m["ok"] != true {
		t.Fatalf("with a session: status=%d body=%v", w.Code, m)
	}
}

func TestMCPKeyEndpointsRefuseCrossSite(t *testing.T) {
	st := newMemStore(t)
	h, _ := newMCPKeyRouter(t, st, mcpAppKey)
	_, id := createMCPKey(t, h, "Laptop", true)

	send := func(method, path, body, contentType, fetchSite string) int {
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.Host = mcpTestHost
		if contentType != "" {
			r.Header.Set("Content-Type", contentType)
		}
		if fetchSite != "" {
			r.Header.Set("Sec-Fetch-Site", fetchSite)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w.Code
	}

	for _, r := range []struct {
		method string
		path   string
		body   string
	}{
		{http.MethodPost, "/api/mcp/keys", `{"label":"From another site"}`},
		{http.MethodPatch, "/api/mcp/keys/" + id, `{"canStartBackups":false}`},
		{http.MethodDelete, "/api/mcp/keys/" + id, ""},
	} {
		if code := send(r.method, r.path, r.body, "application/json", "cross-site"); code != http.StatusForbidden {
			t.Fatalf("%s %s cross-site: status = %d, want 403", r.method, r.path, code)
		}
	}
	for _, r := range []struct {
		method string
		path   string
		body   string
	}{
		{http.MethodPost, "/api/mcp/keys", `{"label":"Plain text"}`},
		{http.MethodPatch, "/api/mcp/keys/" + id, `{"canStartBackups":false}`},
	} {
		if code := send(r.method, r.path, r.body, "text/plain", ""); code != http.StatusUnsupportedMediaType {
			t.Fatalf("%s %s as text/plain: status = %d, want 415", r.method, r.path, code)
		}
	}
}

func TestMCPCertificateEndpoints(t *testing.T) {
	st := newMemStore(t)
	h, _ := newMCPKeyRouter(t, st, mcpAppKey)

	w, _ := doMCPKey(t, h, http.MethodGet, "/api/mcp/certificate", "")
	if w.Code != http.StatusOK {
		t.Fatalf("download: status = %d, want 200", w.Code)
	}
	if got := w.Header().Get("Content-Type"); got != "application/x-pem-file" {
		t.Fatalf("Content-Type = %q", got)
	}
	if got := w.Header().Get("Content-Disposition"); got != `attachment; filename="bombvault-cert.pem"` {
		t.Fatalf("Content-Disposition = %q", got)
	}
	if !strings.HasPrefix(w.Body.String(), "-----BEGIN CERTIFICATE-----") {
		t.Fatalf("body is not a PEM certificate: %q", w.Body.String())
	}

	_, m := doMCPKey(t, h, http.MethodPost, "/api/mcp/certificate/names", `{"host":"192.168.1.10"}`)
	if m["ok"] != true {
		t.Fatalf("add name: %v", m)
	}
	cert, _ := m["certificate"].(map[string]any)
	names, _ := cert["names"].([]any)
	if len(names) != 4 || names[3] != "192.168.1.10" {
		t.Fatalf("names after adding one = %v", names)
	}

	if _, m = doMCPKey(t, h, http.MethodPost, "/api/mcp/certificate/names", `{"host":"bad host!"}`); m["code"] != "cert-name-invalid" {
		t.Fatalf("an unusable address: %v", m)
	}
	if _, m = doMCPKey(t, h, http.MethodPost, "/api/mcp/certificate/names", `{"host":"1.2.3.4","label":"x"}`); m["ok"] != false {
		t.Fatalf("an unknown field must be refused: %v", m)
	}

	r := httptest.NewRequest(http.MethodPost, "/api/mcp/certificate/names", strings.NewReader(`{"host":"1.2.3.4"}`))
	r.Host = mcpTestHost
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Sec-Fetch-Site", "cross-site")
	w = httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusForbidden {
		t.Fatalf("cross-site: status = %d, want 403", w.Code)
	}

	cookie := enableLogin(t, st, mcpAppKey)
	for _, route := range []struct {
		method string
		path   string
		body   string
	}{
		{http.MethodGet, "/api/mcp/certificate", ""},
		{http.MethodPost, "/api/mcp/certificate/names", `{"host":"192.168.1.11"}`},
	} {
		if w, _ = doMCPKey(t, h, route.method, route.path, route.body); w.Code != http.StatusUnauthorized {
			t.Fatalf("%s %s without a session: status = %d, want 401", route.method, route.path, w.Code)
		}
		if w, _ = doMCPKeyJSON(t, h, route.method, route.path, route.body, mcpTestHost, cookie); w.Code != http.StatusOK {
			t.Fatalf("%s %s with a session: status = %d, want 200", route.method, route.path, w.Code)
		}
	}
}

func TestMCPCertificateEndpointsAreGoneWithoutTLS(t *testing.T) {
	st := newMemStore(t)
	dir := t.TempDir()
	h, _ := mcpRouterWith(t, st, config.Config{AppKey: mcpAppKey, DataDir: dir, HostMountRoot: dir, HTTPOnly: true})

	for _, route := range []struct {
		method string
		path   string
		body   string
	}{
		{http.MethodGet, "/api/mcp/certificate", ""},
		{http.MethodPost, "/api/mcp/certificate/names", `{"host":"192.168.1.10"}`},
	} {
		if w, _ := doMCPKey(t, h, route.method, route.path, route.body); w.Code != http.StatusNotFound {
			t.Fatalf("%s %s on a plain HTTP install: status = %d, want 404", route.method, route.path, w.Code)
		}
	}
	if list := mcpKeyList(t, h); list["certificate"] != nil {
		t.Fatalf("certificate = %v, want null on a plain HTTP install", list["certificate"])
	}
}

func TestMCPKeyLifecycleEndpoints(t *testing.T) {
	st := newMemStore(t)
	h, _ := newMCPKeyRouter(t, st, mcpAppKey)
	oldKey, id := createMCPKey(t, h, "Laptop", true)

	_, m := doMCPKey(t, h, http.MethodPatch, "/api/mcp/keys/"+id, `{"label":"Desk","canStartBackups":false}`)
	item, _ := m["item"].(map[string]any)
	if m["ok"] != true || item["label"] != "Desk" || item["canStartBackups"] != false {
		t.Fatalf("patch: %v", m)
	}

	_, m = doMCPKey(t, h, http.MethodPost, "/api/mcp/keys/"+id+"/rotate", "")
	if m["ok"] != true || m["key"] == oldKey {
		t.Fatalf("rotate must hand out a new key: %v", m)
	}

	_, m = doMCPKey(t, h, http.MethodDelete, "/api/mcp/keys/"+id, "")
	if m["code"] != "mcp-key-active" {
		t.Fatalf("deleting an active key: code = %v, want mcp-key-active", m["code"])
	}

	if _, m = doMCPKey(t, h, http.MethodPost, "/api/mcp/keys/"+id+"/revoke", ""); m["ok"] != true {
		t.Fatalf("revoke: %v", m)
	}
	revoked := mcpKeyRows(t, mcpKeyList(t, h), "revoked")
	if len(revoked) != 1 || revoked[0]["revokedReason"] != "user" {
		t.Fatalf("revoked list = %v", revoked)
	}

	if _, m = doMCPKey(t, h, http.MethodDelete, "/api/mcp/keys/"+id, ""); m["ok"] != true {
		t.Fatalf("deleting a revoked, unreferenced key: %v", m)
	}

	_, used := createMCPKey(t, h, "Referenced", true)
	if _, err := st.StartRunWith("container:plex", "backup", store.RunMeta{StartedVia: "mcp", StartedViaKey: used}); err != nil {
		t.Fatal(err)
	}
	if _, m = doMCPKey(t, h, http.MethodPost, "/api/mcp/keys/"+used+"/revoke", ""); m["ok"] != true {
		t.Fatalf("revoke: %v", m)
	}
	if _, m = doMCPKey(t, h, http.MethodDelete, "/api/mcp/keys/"+used, ""); m["code"] != "mcp-key-in-use" {
		t.Fatalf("deleting a key a run names: code = %v, want mcp-key-in-use", m["code"])
	}

	w, m := doMCPKey(t, h, http.MethodDelete, "/api/mcp/keys/not-an-id", "")
	if w.Code != http.StatusBadRequest || m["ok"] != false {
		t.Fatalf("a malformed id: status=%d body=%v", w.Code, m)
	}
	unknown := strings.Repeat("a", 32)
	for _, r := range []struct {
		method string
		path   string
		body   string
	}{
		{http.MethodPatch, "/api/mcp/keys/" + unknown, `{"label":"Nobody"}`},
		{http.MethodPost, "/api/mcp/keys/" + unknown + "/rotate", ""},
		{http.MethodPost, "/api/mcp/keys/" + unknown + "/revoke", ""},
		{http.MethodDelete, "/api/mcp/keys/" + unknown, ""},
	} {
		if _, m := doMCPKey(t, h, r.method, r.path, r.body); m["code"] != "mcp-key-not-found" {
			t.Fatalf("%s %s: code = %v, want mcp-key-not-found", r.method, r.path, m["code"])
		}
	}
}

func TestMCPKeyListCarriesEndpointAndAuthState(t *testing.T) {
	st := newMemStore(t)
	h, _ := newMCPKeyRouter(t, st, mcpAppKey)
	_, id := createMCPKey(t, h, "Laptop", true)

	list := mcpKeyList(t, h)
	for field, want := range map[string]any{
		"endpointPath":     "/mcp",
		"limit":            float64(store.MCPKeyLimit),
		"startsPerHour":    float64(12),
		"cooldownMinutes":  float64(15),
		"itemStartsPerDay": float64(4),
		"authEnabled":      false,
	} {
		if list[field] != want {
			t.Fatalf("%s = %v, want %v", field, list[field], want)
		}
	}
	if rows := mcpKeyRows(t, list, "keys"); len(rows) != 1 || rows[0]["inUse"] != false {
		t.Fatalf("a key no run names must not be inUse: %v", rows)
	}
	cert, _ := list["certificate"].(map[string]any)
	if cert["selfIssued"] != true {
		t.Fatalf("certificate = %v, want BombVault's own on a fresh data dir", list["certificate"])
	}
	if names, _ := cert["names"].([]any); len(names) != 3 || names[0] != "localhost" || names[1] != "127.0.0.1" || names[2] != "::1" {
		t.Fatalf("certificate names = %v", cert["names"])
	}
	if fp, _ := cert["fingerprint"].(string); len(fp) != 64 {
		t.Fatalf("fingerprint = %q, want a sha256 in hex", fp)
	}

	if _, err := st.StartRunWith("container:plex", "backup", store.RunMeta{StartedVia: "mcp", StartedViaKey: id}); err != nil {
		t.Fatal(err)
	}
	_, revokedID := createMCPKey(t, h, "Old desktop", false)
	if _, m := doMCPKey(t, h, http.MethodPost, "/api/mcp/keys/"+revokedID+"/revoke", ""); m["ok"] != true {
		t.Fatalf("revoke: %v", m)
	}
	cookie := enableLogin(t, st, mcpAppKey)

	_, list = doMCPKeyJSON(t, h, http.MethodGet, "/api/mcp/keys", "", mcpTestHost, cookie)
	if list["authEnabled"] != true {
		t.Fatalf("authEnabled = %v, want true once a login password is set", list["authEnabled"])
	}
	active := mcpKeyRows(t, list, "keys")
	if len(active) != 1 || active[0]["id"] != id || active[0]["inUse"] != true {
		t.Fatalf("active keys = %v", active)
	}
	revoked := mcpKeyRows(t, list, "revoked")
	if len(revoked) != 1 || revoked[0]["id"] != revokedID {
		t.Fatalf("revoked keys = %v", revoked)
	}
}

// The key is handed out exactly once, so the response must not wait behind a
// notification endpoint that never answers: a cut connection would leave a key
// nobody has ever seen.
func TestMCPKeyReachesTheOperatorBeforeTheNotification(t *testing.T) {
	var once sync.Once
	reached := make(chan struct{})
	release := make(chan struct{})
	hook := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		once.Do(func() { close(reached) })
		<-release
	}))
	defer hook.Close()
	defer close(release)

	st := newMemStore(t)
	h, svc := newMCPKeyRouter(t, st, mcpAppKey)
	if err := svc.SetNotifyConfig(notify.Config{
		On: "always", WebhookEnabled: true, WebhookURL: hook.URL, WebhookFormat: "generic",
	}); err != nil {
		t.Fatal(err)
	}

	type answer struct {
		status int
		body   map[string]any
	}
	done := make(chan answer, 1)
	go func() {
		r := httptest.NewRequest(http.MethodPost, "/api/mcp/keys", strings.NewReader(`{"label":"Laptop"}`))
		r.Header.Set("Content-Type", "application/json")
		r.Host = mcpTestHost
		r.Header.Set("Origin", "https://"+mcpTestHost)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		var m map[string]any
		_ = json.Unmarshal(w.Body.Bytes(), &m)
		done <- answer{status: w.Code, body: m}
	}()

	select {
	case got := <-done:
		if got.status != http.StatusOK || got.body["ok"] != true {
			t.Fatalf("create: status=%d body=%v", got.status, got.body)
		}
		if key, _ := got.body["key"].(string); key == "" {
			t.Fatalf("the answer carries no key: %v", got.body)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("the key was still waiting on the notification endpoint")
	}

	select {
	case <-reached:
	case <-time.After(3 * time.Second):
		t.Fatal("no notification was sent at all")
	}
}
