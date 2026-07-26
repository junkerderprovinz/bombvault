package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

// setWidgetToken stores a widget token directly (what POST /api/widget/token
// does), so gate tests can control the stored value precisely.
func setWidgetToken(t *testing.T, st *store.Repo, token string) {
	t.Helper()
	s, err := st.GetSettings()
	if err != nil {
		t.Fatalf("get settings: %v", err)
	}
	s.WidgetToken = token
	if err := st.UpdateSettings(s); err != nil {
		t.Fatalf("update settings: %v", err)
	}
}

// widgetGet performs a GET with an optional X-Widget-Token header.
func widgetGet(t *testing.T, h http.Handler, path, headerToken string) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(http.MethodGet, path, nil)
	if headerToken != "" {
		r.Header.Set("X-Widget-Token", headerToken)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

// TestWidgetTokenGate pins the fail-closed token gate on BOTH widget endpoints:
// no stored token (feature off) → 403 even with an empty presented token; a
// missing or wrong token → 403; the right token (query param or header) → 200.
func TestWidgetTokenGate(t *testing.T) {
	h, st := newTestRouter(t, &fakeServiceDocker{}, &fakeResticEngine{})

	// Feature OFF (empty stored token): everything is 403, including the
	// empty-presented-token probe (""=="" must NOT pass).
	for _, path := range []string{"/widget", "/api/widget/data", "/widget?token=", "/api/widget/data?token="} {
		if w := widgetGet(t, h, path, ""); w.Code != http.StatusForbidden {
			t.Fatalf("empty stored token: GET %s = %d, want 403", path, w.Code)
		}
	}

	const tok = "0123456789abcdef0123456789abcdef"
	setWidgetToken(t, st, tok)

	// No token / wrong token → 403.
	for _, path := range []string{"/widget", "/api/widget/data"} {
		if w := widgetGet(t, h, path, ""); w.Code != http.StatusForbidden {
			t.Fatalf("no token: GET %s = %d, want 403", path, w.Code)
		}
		if w := widgetGet(t, h, path+"?token=wrong", ""); w.Code != http.StatusForbidden {
			t.Fatalf("wrong token: GET %s = %d, want 403", path, w.Code)
		}
	}

	// Right token via query param → 200; the page is the self-contained HTML.
	w := widgetGet(t, h, "/widget?token="+tok, "")
	if w.Code != http.StatusOK {
		t.Fatalf("right token (query): GET /widget = %d, want 200", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Fatalf("widget page Content-Type = %q, want text/html", ct)
	}
	if !strings.Contains(w.Body.String(), "BombVault") || !strings.Contains(w.Body.String(), "/api/widget/data") {
		t.Fatalf("widget page body does not look like the widget HTML")
	}

	// Right token via the X-Widget-Token header → 200 on the feed too.
	if w := widgetGet(t, h, "/api/widget/data", tok); w.Code != http.StatusOK {
		t.Fatalf("right token (header): GET /api/widget/data = %d, want 200", w.Code)
	}
}

// widgetDataBody fetches /api/widget/data with the token and decodes the body.
func widgetDataBody(t *testing.T, h http.Handler, tok string) map[string]any {
	t.Helper()
	w := widgetGet(t, h, "/api/widget/data?token="+tok, "")
	if w.Code != http.StatusOK {
		t.Fatalf("GET /api/widget/data = %d, want 200 (body %q)", w.Code, w.Body.String())
	}
	var m map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &m); err != nil {
		t.Fatalf("decode widget data: %v", err)
	}
	if m["ok"] != true {
		t.Fatalf("widget data not ok: %v", m)
	}
	return m
}

// TestWidgetDataShape pins the slim feed: version present, runs carry the
// resolved target/domain (items AND domain-scoped ops), error text is
// truncated, and the schedule-next list is always an array.
func TestWidgetDataShape(t *testing.T) {
	h, st := newTestRouter(t, &fakeServiceDocker{}, &fakeResticEngine{})
	const tok = "0123456789abcdef0123456789abcdef"
	setWidgetToken(t, st, tok)

	tg, err := st.UpsertTarget(store.Target{ContainerName: "plex", AppdataPaths: []string{"/x"}})
	if err != nil {
		t.Fatal(err)
	}
	// A successful item backup, a failed one with an oversized error, and a
	// domain-scoped off-site run (recorded on the domain literal target id).
	okRun, err := st.StartRun(tg.ID, "backup")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.FinishRun(okRun, "success", "deadbeef12345678", 2048, ""); err != nil {
		t.Fatal(err)
	}
	longErr := strings.Repeat("x", 300)
	failRun, err := st.StartRun(tg.ID, "backup")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.FinishRun(failRun, "failed", "", 0, longErr); err != nil {
		t.Fatal(err)
	}
	offRun, err := st.StartRun("containers", "offsite")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.FinishRun(offRun, "success", "", 0, ""); err != nil {
		t.Fatal(err)
	}

	m := widgetDataBody(t, h, tok)
	if v, _ := m["version"].(string); v == "" {
		t.Fatalf("widget data must carry the app version, got %v", m["version"])
	}
	if _, isArr := m["next"].([]any); !isArr {
		t.Fatalf("widget data next must be an array, got %T", m["next"])
	}
	runs, _ := m["runs"].([]any)
	if len(runs) != 3 {
		t.Fatalf("want 3 runs, got %d", len(runs))
	}

	byID := map[string]map[string]any{}
	for _, r := range runs {
		row := r.(map[string]any)
		byID[row["id"].(string)] = row
		// Slim contract: none of the heavyweight run fields leak into the feed.
		for _, forbidden := range []string{"snapshotId", "targetId"} {
			if _, has := row[forbidden]; has {
				t.Fatalf("widget run must not carry %q: %v", forbidden, row)
			}
		}
	}
	okRow := byID[okRun]
	if okRow["target"] != "plex" || okRow["domain"] != "container" {
		t.Fatalf("item run target/domain wrong: %v", okRow)
	}
	if okRow["kind"] != "backup" || okRow["status"] != "success" || okRow["bytes"] != float64(2048) {
		t.Fatalf("item run fields wrong: %v", okRow)
	}
	if okRow["finishedAt"] == nil || okRow["startedAt"] == nil {
		t.Fatalf("item run timestamps missing: %v", okRow)
	}
	failRow := byID[failRun]
	gotErr, _ := failRow["error"].(string)
	if !strings.HasSuffix(gotErr, "…") || len(gotErr) >= len(longErr) {
		t.Fatalf("error must be truncated with an ellipsis: %d chars, %q…", len(gotErr), gotErr[:20])
	}
	offRow := byID[offRun]
	if offRow["domain"] != "containers" || offRow["kind"] != "offsite" {
		t.Fatalf("domain-op run must carry its domain literal: %v", offRow)
	}
}

// TestWidgetDataLimit pins the ~40-run cap: the feed is a glanceable tile, not
// the 500-run dashboard history.
func TestWidgetDataLimit(t *testing.T) {
	h, st := newTestRouter(t, &fakeServiceDocker{}, &fakeResticEngine{})
	const tok = "0123456789abcdef0123456789abcdef"
	setWidgetToken(t, st, tok)

	tg, err := st.UpsertTarget(store.Target{ContainerName: "plex", AppdataPaths: []string{"/x"}})
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 50; i++ {
		id, sErr := st.StartRun(tg.ID, "backup")
		if sErr != nil {
			t.Fatal(sErr)
		}
		if fErr := st.FinishRun(id, "success", "deadbeef12345678", 1, ""); fErr != nil {
			t.Fatal(fErr)
		}
	}
	m := widgetDataBody(t, h, tok)
	runs, _ := m["runs"].([]any)
	if len(runs) != 40 {
		t.Fatalf("widget feed must cap at 40 runs, got %d", len(runs))
	}
}

var hex32Re = regexp.MustCompile(`^[0-9a-f]{32}$`)

// TestWidgetTokenGenerateAndDisable pins the management endpoints: POST
// generates+stores+returns a 32-hex token once, a second POST rotates it
// (revoking the old one), and DELETE clears it so the widget fails closed
// again.
func TestWidgetTokenGenerateAndDisable(t *testing.T) {
	h, st := newTestRouter(t, &fakeServiceDocker{}, &fakeResticEngine{})

	w, m := doJSON(t, h, http.MethodPost, "/api/widget/token", "")
	if w.Code != http.StatusOK || m["ok"] != true {
		t.Fatalf("generate: code=%d body=%v", w.Code, m)
	}
	tok, _ := m["token"].(string)
	if !hex32Re.MatchString(tok) {
		t.Fatalf("generated token must be 32 lowercase hex, got %q", tok)
	}
	s, err := st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	if s.WidgetToken != tok {
		t.Fatalf("generated token not stored: settings=%q returned=%q", s.WidgetToken, tok)
	}
	if w := widgetGet(t, h, "/api/widget/data?token="+tok, ""); w.Code != http.StatusOK {
		t.Fatalf("fresh token must authorize the feed, got %d", w.Code)
	}

	// Regenerate: a NEW token replaces (revokes) the old one.
	_, m2 := doJSON(t, h, http.MethodPost, "/api/widget/token", "")
	tok2, _ := m2["token"].(string)
	if !hex32Re.MatchString(tok2) || tok2 == tok {
		t.Fatalf("regenerate must return a fresh 32-hex token, got %q (old %q)", tok2, tok)
	}
	if w := widgetGet(t, h, "/api/widget/data?token="+tok, ""); w.Code != http.StatusForbidden {
		t.Fatalf("old token must be revoked after regenerate, got %d", w.Code)
	}
	if w := widgetGet(t, h, "/api/widget/data?token="+tok2, ""); w.Code != http.StatusOK {
		t.Fatalf("new token must authorize the feed, got %d", w.Code)
	}

	// Disable: clears the token → fail closed again.
	w3, m3 := doJSON(t, h, http.MethodDelete, "/api/widget/token", "")
	if w3.Code != http.StatusOK || m3["ok"] != true {
		t.Fatalf("disable: code=%d body=%v", w3.Code, m3)
	}
	if w := widgetGet(t, h, "/api/widget/data?token="+tok2, ""); w.Code != http.StatusForbidden {
		t.Fatalf("disabled widget must 403 even with the last token, got %d", w.Code)
	}
}

// TestWidgetTokenSettingsRoundTrip pins the settingsView secret contract
// (EXACTLY like metricsToken): GET returns widgetToken blank with
// widgetTokenSet=true, and PUTting that round-tripped body back KEEPS the
// stored token instead of wiping it.
func TestWidgetTokenSettingsRoundTrip(t *testing.T) {
	h, _ := newTestRouter(t, &fakeServiceDocker{}, &fakeResticEngine{})

	_, gen := doJSON(t, h, http.MethodPost, "/api/widget/token", "")
	tok, _ := gen["token"].(string)
	if tok == "" {
		t.Fatal("generate returned no token")
	}

	w, m := doJSON(t, h, http.MethodGet, "/api/settings", "")
	if w.Code != http.StatusOK || m["ok"] != true {
		t.Fatalf("get settings: code=%d body=%v", w.Code, m)
	}
	view, _ := m["settings"].(map[string]any)
	if view["widgetToken"] != "" || view["widgetTokenSet"] != true {
		t.Fatalf("GET must blank the token and report it set: token=%v set=%v",
			view["widgetToken"], view["widgetTokenSet"])
	}

	// Round-trip the GET body through PUT — the blank widgetToken must KEEP the
	// stored one.
	body, err := json.Marshal(view)
	if err != nil {
		t.Fatal(err)
	}
	wPut, mPut := doJSON(t, h, http.MethodPut, "/api/settings", string(body))
	if wPut.Code != http.StatusOK || mPut["ok"] != true {
		t.Fatalf("put settings: code=%d body=%v", wPut.Code, mPut)
	}
	if w := widgetGet(t, h, "/api/widget/data?token="+tok, ""); w.Code != http.StatusOK {
		t.Fatalf("settings round-trip must keep the widget token, got %d", w.Code)
	}
}
