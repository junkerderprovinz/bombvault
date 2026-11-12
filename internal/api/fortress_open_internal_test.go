package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/junkerderprovinz/parleyport/kit/fortress"
	"github.com/junkerderprovinz/parleyport/kit/relay"

	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/secret"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

var openedShell = fstest.MapFS{
	"index.html": {Data: []byte(`<!doctype html><html><head><base href="/" /></head><body>shell</body></html>`)},
}

// servePages serves in's whole interface from openedShell, the way main does.
func servePages(in *instance) http.Handler {
	h := NewServer(config.Config{HTTPOnly: true}, openedShell, in.router, in.h.FortressTabs()).Handler()
	in.svc.ServePhonePages(h)
	return h
}

// openedRequest is a request as the kit hands it to the pages it opens: from
// far away, without a host, a cookie or anything the browser said about
// where it came from.
func openedRequest(method, target string) *http.Request {
	r := httptest.NewRequest(method, target, strings.NewReader("{}"))
	r.RemoteAddr, r.Host = "192.0.2.1:0", ""
	r.Header = http.Header{"Content-Type": {"application/json"}}
	return r
}

func throughFortress(in *instance, r *http.Request) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	in.svc.serveOpened(w, r)
	return w
}

func codeOf(t *testing.T, w *httptest.ResponseRecorder) string {
	t.Helper()
	var out struct {
		Code string `json:"code"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	return out.Code
}

func TestBombVaultOffersItsPagesAndOpensTheOtherApps(t *testing.T) {
	in := fortressInstance(t)
	cfg := in.svc.fortressConfig(fortress.Config{}, nil)
	if cfg.UI == nil || cfg.UI.Handler == nil || !slices.Contains(cfg.Uses, fortress.OpenAction) {
		t.Fatalf("ui %+v, uses %v", cfg.UI, cfg.Uses)
	}
	if !cfg.UI.SignIn() {
		t.Fatal("with a login password the pages do not open")
	}
	if _, err := in.st.MutateSettings(func(s *store.Settings) error { s.AuthPasswordHash = ""; return nil }); err != nil {
		t.Fatal(err)
	}
	if cfg.UI.SignIn() {
		t.Fatal("without a login password the pages open to every holder of the phrase")
	}
}

func TestAnOpenedPageFindsTheWaysInAndTheKeysClosed(t *testing.T) {
	in := fortressInstance(t)
	servePages(in)
	for _, c := range []struct{ method, path string }{
		{http.MethodPost, "/api/auth/password"},
		{http.MethodPost, "/api/auth/reset"},
		{http.MethodPost, "/api/auth/reset/finish"},
		{http.MethodPost, "/api/auth/totp/setup"},
		{http.MethodPost, "/api/auth/passkey/register/begin"},
		{http.MethodDelete, "/api/auth/passkeys/abc"},
		{http.MethodGet, "/api/tokens"},
		{http.MethodPost, "/api/tokens/abc/rotate"},
		{http.MethodGet, "/api/mcp/keys"},
		{http.MethodPost, "/mcp"},
		{http.MethodGet, "/oauth/authorize"},
		{http.MethodPost, "/oauth/token"},
		{http.MethodGet, "/.well-known/oauth-authorization-server"},
		{http.MethodGet, "/api/v1/status"},
		{http.MethodGet, "/metrics"},
		{http.MethodGet, "/widget"},
		{http.MethodGet, "/api/widget/data"},
		{http.MethodPost, "/api/fortress/open/abc"},
		{http.MethodGet, "/fortress/open/abc/"},
		{http.MethodGet, "/api//tokens"},
		{http.MethodGet, "/api/health/../tokens"},
	} {
		w := throughFortress(in, openedRequest(c.method, c.path))
		if w.Code != http.StatusForbidden || codeOf(t, w) != codeOpenedRefused {
			t.Errorf("%s %s through the fortress = %d %s", c.method, c.path, w.Code, w.Body.String())
		}
	}
	for _, p := range []string{"/api/health", "/api/auth", "/api/auth/passkeys", "/settings/pairing"} {
		if w := throughFortress(in, openedRequest(http.MethodGet, p)); w.Code != http.StatusOK {
			t.Errorf("GET %s through the fortress = %d %s", p, w.Code, w.Body.String())
		}
	}
}

func TestAnOpenedPageSavesNoFile(t *testing.T) {
	in := fortressInstance(t)
	in.svc.ServePhonePages(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Disposition", `attachment; filename="kit.zip"`)
		_, _ = w.Write([]byte("the recovery kit"))
	}))
	w := throughFortress(in, openedRequest(http.MethodGet, "/api/recovery-kit"))
	if w.Code != http.StatusForbidden || codeOf(t, w) != codeOpenedRefused || strings.Contains(w.Body.String(), "recovery kit") || w.Header().Get("Content-Disposition") != "" {
		t.Fatalf("a download through the fortress = %d %v %s", w.Code, w.Header(), w.Body.String())
	}
}

func TestAnOpenedPageMayBeFramedOnlyWhereItWasOpened(t *testing.T) {
	in := fortressInstance(t)
	pages := servePages(in)
	w := throughFortress(in, openedRequest(http.MethodGet, "/"))
	if csp := w.Header().Get("Content-Security-Policy"); !strings.Contains(csp, "frame-ancestors 'self'") || w.Header().Get("X-Frame-Options") != "" {
		t.Fatalf("an opened page: %v", w.Header())
	}
	direct := httptest.NewRecorder()
	pages.ServeHTTP(direct, httptest.NewRequest(http.MethodGet, "/", nil))
	if csp := direct.Header().Get("Content-Security-Policy"); !strings.Contains(csp, "frame-ancestors 'none'") || direct.Header().Get("X-Frame-Options") != "DENY" {
		t.Fatalf("the page at its own address: %v", direct.Header())
	}
}

func TestAnOpenedPageTakesItsBaseFromThePrefix(t *testing.T) {
	in := fortressInstance(t)
	servePages(in)
	r := openedRequest(http.MethodGet, "/")
	r.Header.Set("X-Forwarded-Prefix", "/fortress/open/0123abcd/")
	if w := throughFortress(in, r); !strings.Contains(w.Body.String(), `<base href="/fortress/open/0123abcd/" />`) {
		t.Fatalf("the shell under the prefix: %s", w.Body.String())
	}
}

func TestOnlyASessionSignedInWithThePasswordOpensAnotherApp(t *testing.T) {
	in := fortressInstance(t)
	if _, out := in.do(t, http.MethodPost, "/api/fortress/open/abc", nil); out["code"] != "noFortress" {
		t.Fatalf("outside a fortress = %v", out)
	}
	hash := mustSettings(t, in).AuthPasswordHash
	in.as(secret.NewPairedSessionToken(in.appKey, hash, "", wordsHolderID, time.Hour), func() {
		if code, out := in.do(t, http.MethodPost, "/api/fortress/open/abc", nil); code < 400 || out["path"] != nil {
			t.Fatalf("a phone's session = %d %v", code, out)
		}
	})
	if _, err := in.st.MutateSettings(func(s *store.Settings) error { s.AuthPasswordHash = ""; return nil }); err != nil {
		t.Fatal(err)
	}
	in.as("", func() {
		if code, out := in.do(t, http.MethodPost, "/api/fortress/open/abc", nil); code != http.StatusForbidden || out["code"] != codeNoPassword {
			t.Fatalf("without a login password = %d %v", code, out)
		}
	})
}

func mustSettings(t *testing.T, in *instance) store.Settings {
	t.Helper()
	s, err := in.st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestAPhonesPageReachesNoTab(t *testing.T) {
	in, _ := joinedInstance(t)
	pages := servePages(in)
	r := httptest.NewRequest(http.MethodGet, "/fortress/open/0123/", nil)
	w := httptest.NewRecorder()
	pages.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), relayedKey{}, true)))
	if w.Code != http.StatusNotFound {
		t.Fatalf("a phone's page at a tab = %d %s", w.Code, w.Body.String())
	}
}

// joinedInstance is an instance in a fortress of its own, with the phrase.
func joinedInstance(t *testing.T) (*instance, string) {
	t.Helper()
	in := fortressInstance(t)
	words, _ := newPhrase(t)
	if _, out := in.do(t, http.MethodPost, "/api/fortress/join", map[string]any{"phrase": words}); out["ok"] != true {
		t.Fatalf("join = %v", out)
	}
	return in, words
}

func TestASandboxedPageReachesItsTabWhileTheAPIRefusesItsWrites(t *testing.T) {
	in, _ := joinedInstance(t)
	pages := servePages(in)
	sandboxed := func(r *http.Request) *http.Request {
		r.Header.Set("Origin", "null")
		r.Header.Set("Sec-Fetch-Site", "cross-site")
		r.Header.Set("Content-Type", "application/json")
		r.AddCookie(&http.Cookie{Name: sessionCookieNameHTTP, Value: in.session}) //nolint:gosec // G124: a cookie on a test request, never set by a server
		return r
	}
	tab := httptest.NewRecorder()
	pages.ServeHTTP(tab, sandboxed(httptest.NewRequest(http.MethodPost, "/fortress/open/0123/api/settings", strings.NewReader("{}"))))
	if tab.Code != http.StatusGone || !strings.Contains(tab.Body.String(), `"closed"`) {
		t.Fatalf("a tab that is not open = %d %s", tab.Code, tab.Body.String())
	}
	write := httptest.NewRecorder()
	pages.ServeHTTP(write, sandboxed(httptest.NewRequest(http.MethodPut, "/api/fortress/permissions", strings.NewReader(`{"from":"bombvault","to":"knightloader","action":"ui.open","on":false}`))))
	if write.Code != http.StatusForbidden || !strings.Contains(write.Body.String(), "cross-site") {
		t.Fatalf("a cross-site write = %d %s", write.Code, write.Body.String())
	}
}

// inOneFortress puts a and b into one fortress over an in-process relay and
// waits until a can open b.
func inOneFortress(t *testing.T, a, b *instance) string {
	t.Helper()
	rs := httptest.NewServer(relay.New())
	t.Cleanup(rs.Close)
	for _, in := range []*instance{a, b} {
		if _, out := in.do(t, http.MethodPut, "/api/group/relay", map[string]any{"mode": "own", "url": rs.URL}); out["ok"] != true {
			t.Fatalf("set relay: %v", out)
		}
	}
	words, _ := newPhrase(t)
	for _, in := range []*instance{a, b} {
		in.svc.house.mu.Lock()
		in.svc.house.src = fortressSource{in.h}
		in.svc.house.mu.Unlock()
		if _, out := in.do(t, http.MethodPost, "/api/fortress/join", map[string]any{"phrase": words}); out["ok"] != true {
			t.Fatalf("join = %v", out)
		}
	}
	var id string
	waitLong(t, "a to see that b opens", func() bool {
		for _, m := range fortressMembers(t, a) {
			if m.Opens && m.Online && m.Device == "attic" {
				id = m.ID
				return true
			}
		}
		return false
	})
	return id
}

func fortressMembers(t *testing.T, in *instance) []fortressMemberView {
	t.Helper()
	v, err := in.h.fortressViewNow()
	if err != nil {
		t.Fatal(err)
	}
	return v.Members
}

func TestAMemberOpensAnotherThroughTheFortress(t *testing.T) {
	a := newInstance(t, "cellar", strings.Repeat("a1", 32))
	b := newInstance(t, "attic", strings.Repeat("b2", 32))
	pagesA := servePages(a)
	servePages(b)
	id := inOneFortress(t, a, b)

	_, out := a.do(t, http.MethodPost, "/api/fortress/open/"+id, nil)
	tab, _ := out["path"].(string)
	if out["ok"] != true || !strings.HasPrefix(tab, fortress.OpenPath) {
		t.Fatalf("open = %v", out)
	}
	get := func(path string, signedIn bool) *httptest.ResponseRecorder {
		r := httptest.NewRequest(http.MethodGet, path, nil)
		if signedIn {
			r.AddCookie(&http.Cookie{Name: sessionCookieNameHTTP, Value: a.session}) //nolint:gosec // G124: a cookie on a test request, never set by a server
		}
		w := httptest.NewRecorder()
		pagesA.ServeHTTP(w, r)
		return w
	}
	if w := get(tab, false); w.Code != http.StatusSeeOther {
		t.Fatalf("the tab without the sign-in = %d", w.Code)
	}
	if w := get(tab, true); w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "<iframe") {
		t.Fatalf("the tab = %d %s", w.Code, w.Body.String())
	}
	w := get(tab+"/", false)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `<base href="`+tab+`/" />`) {
		t.Fatalf("the opened shell = %d %s", w.Code, w.Body.String())
	}
	if csp := strings.Join(w.Header().Values("Content-Security-Policy"), " | "); !strings.Contains(csp, "sandbox") || strings.Contains(csp, "frame-ancestors 'none'") {
		t.Fatalf("the opened shell's policies: %s", csp)
	}
	if w := get(tab+"/api/tokens", false); w.Code != http.StatusForbidden {
		t.Fatalf("the tokens of the opened app = %d %s", w.Code, w.Body.String())
	}

	_, st := a.do(t, http.MethodGet, "/api/fortress/members/"+id+"/status?lang=de", nil)
	wire, _ := json.Marshal(st)
	if st["ok"] != true || st["report"] == nil || !strings.Contains(string(wire), "über die Festung geöffnet") {
		t.Fatalf("the status of the opened app = %s", wire)
	}

	runs, err := b.st.ListRuns(10)
	if err != nil {
		t.Fatal(err)
	}
	i := slices.IndexFunc(runs, func(r store.Run) bool { return r.TargetID == store.FortressTargetID && r.Kind == runKindOpened })
	if i < 0 || !strings.HasPrefix(runs[i].Subject, "BombVault · cellar · ") || runs[i].Status != "success" {
		t.Fatalf("the activity log of the opened app: %+v", runs)
	}
}
