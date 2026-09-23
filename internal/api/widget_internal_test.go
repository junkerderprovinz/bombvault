package api

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestWidgetTokenOK checks that an empty stored token never matches, that the
// feed takes the token only from the header and that the page also takes it
// from the query.
func TestWidgetTokenOK(t *testing.T) {
	req := func(query, header string) *http.Request {
		r := httptest.NewRequest(http.MethodGet, "/api/widget/data"+query, nil)
		if header != "" {
			r.Header.Set("X-Widget-Token", header)
		}
		return r
	}

	// With no stored token nothing matches, not even an empty one.
	if widgetTokenOK(req("", ""), "") {
		t.Fatal("empty stored + empty presented must fail (fail closed)")
	}
	if widgetTokenOK(req("?token=", ""), "") {
		t.Fatal("empty stored + empty query token must fail")
	}
	if widgetTokenOK(req("?token=x", ""), "") {
		t.Fatal("empty stored + any token must fail")
	}

	const tok = "0123456789abcdef0123456789abcdef"
	if widgetTokenOK(req("", ""), tok) {
		t.Fatal("missing token must fail")
	}
	if widgetTokenOK(req("?token=wrong", ""), tok) {
		t.Fatal("wrong query token must fail")
	}
	// The feed is polled, so a token in its query would land in the proxy's
	// access log on every refresh. The page keeps the query form because an
	// iframe cannot set a header on the document request.
	if widgetTokenOK(req("?token="+tok, ""), tok) {
		t.Fatal("the feed must refuse a right token in the query")
	}
	if !widgetPageTokenOK(req("?token="+tok, ""), tok) {
		t.Fatal("the page must still accept a right token in the query")
	}
	if widgetPageTokenOK(req("?token=wrong", ""), tok) {
		t.Fatal("the page must refuse a wrong query token")
	}
	if !widgetPageTokenOK(req("", tok), tok) {
		t.Fatal("the page must accept the header too")
	}
	if widgetPageTokenOK(req("?token="+tok, ""), "") {
		t.Fatal("an empty stored token must fail on the page as well")
	}
	if !widgetTokenOK(req("", tok), tok) {
		t.Fatal("right header token must pass")
	}
	if widgetTokenOK(req("?token="+tok, "wrong"), tok) {
		t.Fatal("a wrong header must not be rescued by a right query token")
	}
	// No prefix matching. Checked on the page, the only gate that reads the
	// query.
	if widgetPageTokenOK(req("?token="+tok[:16], ""), tok) || widgetPageTokenOK(req("?token="+tok+"ff", ""), tok) {
		t.Fatal("prefix/extended tokens must fail")
	}
}

// TestSecurityHeadersWidgetFraming checks that /widget is the only path that
// can be framed; the SPA and every /api route, the widget feed included, keep
// X-Frame-Options: DENY and frame-ancestors 'none'.
func TestSecurityHeadersWidgetFraming(t *testing.T) {
	h := securityHeaders(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	get := func(path string) http.Header {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		return w.Header()
	}

	wh := get("/widget")
	if got := wh.Get("X-Frame-Options"); got != "" {
		t.Fatalf("/widget must not send X-Frame-Options, got %q", got)
	}
	if csp := wh.Get("Content-Security-Policy"); !strings.Contains(csp, "frame-ancestors *") {
		t.Fatalf("/widget CSP must allow frame-ancestors *, got %q", csp)
	}
	if wh.Get("X-Content-Type-Options") != "nosniff" {
		t.Fatal("/widget must keep the nosniff header")
	}

	for _, path := range []string{"/", "/api/status", "/api/widget/data", "/metrics"} {
		hh := get(path)
		if got := hh.Get("X-Frame-Options"); got != "DENY" {
			t.Fatalf("%s must keep X-Frame-Options: DENY, got %q", path, got)
		}
		if csp := hh.Get("Content-Security-Policy"); !strings.Contains(csp, "frame-ancestors 'none'") {
			t.Fatalf("%s CSP must keep frame-ancestors 'none', got %q", path, csp)
		}
	}
}

// TestThemeBootScriptCSPHashMatches checks the script-src hash in
// securityHeaders against the inline theme-boot script in web/index.html,
// which sets data-theme before first paint. Without 'unsafe-inline' the script
// runs only because its hash is allowed, and the vite dev server sends no CSP,
// so a stale hash would show only in production.
func TestThemeBootScriptCSPHashMatches(t *testing.T) {
	html, err := os.ReadFile(filepath.Join("..", "..", "web", "index.html"))
	if err != nil {
		t.Fatalf("reading web/index.html: %v", err)
	}

	// The theme-boot script is the only <script> tag without attributes.
	const openTag = "<script>"
	start := bytes.Index(html, []byte(openTag))
	if start == -1 {
		t.Fatal("web/index.html: no bare <script> tag found; did the theme-boot script move or gain an attribute?")
	}
	contentStart := start + len(openTag)
	end := bytes.Index(html[contentStart:], []byte("</script>"))
	if end == -1 {
		t.Fatal("web/index.html: found an opening <script> tag with no matching </script>")
	}
	content := html[contentStart : contentStart+end]

	sum := sha256.Sum256(content)
	wantSource := "'sha256-" + base64.StdEncoding.EncodeToString(sum[:]) + "'"

	h := securityHeaders(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))
	csp := w.Header().Get("Content-Security-Policy")

	if !strings.Contains(csp, wantSource) {
		t.Fatalf("CSP script-src does not contain the theme-boot script's current hash.\n"+
			"web/index.html's inline script hashes to: %s\n"+
			"CSP header script-src was: %s\n"+
			"The script changed (even whitespace changes the hash). Recompute it and update "+
			"the script-src hash source in server.go's securityHeaders, or the theme-boot "+
			"script will be silently blocked by CSP in production.", wantSource, csp)
	}
}

// TestWidgetOffsiteColourMatchesToken checks that widget.html's off-site colour
// matches --status-offsite-text in web/src/index.css. The widget is served from
// the binary and cannot read the CSS custom properties, so it copies the hex.
func TestWidgetOffsiteColourMatchesToken(t *testing.T) {
	css, err := os.ReadFile(filepath.Join("..", "..", "web", "src", "index.css"))
	if err != nil {
		t.Fatalf("reading web/src/index.css: %v", err)
	}

	// The widget always renders the dark palette, which is the first
	// declaration: the bare :root block is dark and the light override comes
	// later.
	want := firstDeclValue(t, string(css), "--status-offsite-text")
	got := firstDeclValue(t, string(widgetPage), ".offsite { color")

	if !strings.EqualFold(want, got) {
		t.Fatalf("off-site colour drifted between the two surfaces:\n"+
			"  web/src/index.css --status-offsite-text (dark) = %s\n"+
			"  internal/api/widget.html .offsite            = %s\n"+
			"These must stay identical: the widget cannot read the CSS token, "+
			"so it hard-copies the hex. Change both together.", want, got)
	}
}

// firstDeclValue returns the hex value of the first `<prefix>: #rrggbb`
// declaration in src, failing the test when there is none.
func firstDeclValue(t *testing.T, src, prefix string) string {
	t.Helper()
	i := strings.Index(src, prefix+":")
	if i == -1 {
		t.Fatalf("no %q declaration found", prefix)
	}
	rest := src[i+len(prefix)+1:]
	j := strings.Index(rest, "#")
	if j == -1 || j > 40 { // guard against skipping ahead into an unrelated rule
		t.Fatalf("no hex value follows the %q declaration", prefix)
	}
	hex := rest[j:]
	if len(hex) < 7 {
		t.Fatalf("truncated hex value after %q", prefix)
	}
	return hex[:7]
}
