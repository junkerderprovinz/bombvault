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

// TestWidgetTokenOK pins the token-compare helper's semantics: an empty STORED
// token always fails (feature off = fail closed — even ""=="" must not pass),
// the query param and the X-Widget-Token header both work, the header wins
// when both are present, and any mismatch fails. The compare itself is
// crypto/subtle.ConstantTimeCompare (see widgetTokenOK) so a byte-by-byte
// timing probe can't recover the token.
func TestWidgetTokenOK(t *testing.T) {
	req := func(query, header string) *http.Request {
		r := httptest.NewRequest(http.MethodGet, "/api/widget/data"+query, nil)
		if header != "" {
			r.Header.Set("X-Widget-Token", header)
		}
		return r
	}

	// Feature off: empty stored token never authorizes anything.
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
	if !widgetTokenOK(req("?token="+tok, ""), tok) {
		t.Fatal("right query token must pass")
	}
	if !widgetTokenOK(req("", tok), tok) {
		t.Fatal("right header token must pass")
	}
	// The header wins over the query param when both are present.
	if widgetTokenOK(req("?token="+tok, "wrong"), tok) {
		t.Fatal("a wrong header must not be rescued by a right query token")
	}
	// A truncated/extended token must fail (no prefix matching).
	if widgetTokenOK(req("?token="+tok[:16], ""), tok) || widgetTokenOK(req("?token="+tok+"ff", ""), tok) {
		t.Fatal("prefix/extended tokens must fail")
	}
}

// TestSecurityHeadersWidgetFraming pins the ONE framing exception: /widget is
// served without X-Frame-Options and with `frame-ancestors *` (it exists to be
// iframed by other dashboards), while every other path — the SPA and all /api
// routes including the widget's own feed — keeps X-Frame-Options: DENY and
// `frame-ancestors 'none'`.
func TestSecurityHeadersWidgetFraming(t *testing.T) {
	h := securityHeaders(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	get := func(path string) http.Header {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		return w.Header()
	}

	// /widget: frame-able cross-origin.
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

	// Everything else keeps the strict posture — including the widget FEED.
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

// TestThemeBootScriptCSPHashMatches pins the CSP script-src hash in
// securityHeaders to the ACTUAL current content of web/index.html's inline
// theme-boot script (GlimStone form-engine #1). That script exists so
// data-theme is stamped on <html> before first paint — without it, a
// "system" theme user gets a flash of the wrong theme while the module
// bundle loads. `script-src` has no 'unsafe-inline' (deliberately — that
// would allow ANY inline script, not just this one), so the script only
// runs at all because its hash is explicitly allow-listed. vite dev/preview
// send no CSP header, so a mismatch here is invisible in local dev and only
// breaks in the real, CSP-enforcing production server — this test is what
// catches it instead. If you touch the script (even its whitespace),
// recompute the sha256 and update the const in server.go, or this test
// fails on purpose.
func TestThemeBootScriptCSPHashMatches(t *testing.T) {
	html, err := os.ReadFile(filepath.Join("..", "..", "web", "index.html"))
	if err != nil {
		t.Fatalf("reading web/index.html: %v", err)
	}

	// The theme-boot script is the one bare <script> tag (no type= or src=
	// attribute) in the document — everything else is either the CSP-exempt
	// bundled module script or a <link>.
	const openTag = "<script>"
	start := bytes.Index(html, []byte(openTag))
	if start == -1 {
		t.Fatal("web/index.html: no bare <script> tag found — did the theme-boot script move or gain an attribute?")
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
			"The script changed (even whitespace changes the hash) — recompute it and update "+
			"the script-src hash source in server.go's securityHeaders, or the theme-boot "+
			"script will be silently blocked by CSP in production.", wantSource, csp)
	}
}

// TestWidgetOffsiteColourMatchesToken pins the pairing that issue #164 broke.
//
// widget.html is a standalone, dark-only document served straight from the Go
// binary: it cannot read web/src/index.css's custom properties, so it hard-copies
// their hexes. Nothing enforced that copy, so when GlimStone Phase 2 Task 7
// re-pointed the dashboard's off-site log lines at the accent, this file was left
// frozen at the old blue and the two surfaces rendered the SAME "Off-site
// replication done — Containers" line in two different colours.
//
// The resolution of #164 was to restore blue on BOTH surfaces as a narrow
// off-site IDENTITY colour (--status-offsite-text, see index.css's
// --color-statusOffsite comment for why that is not the removed fifth state hue
// coming back). This test is the guard so the next person to touch either side
// finds out immediately instead of via a screenshot months later.
func TestWidgetOffsiteColourMatchesToken(t *testing.T) {
	css, err := os.ReadFile(filepath.Join("..", "..", "web", "src", "index.css"))
	if err != nil {
		t.Fatalf("reading web/src/index.css: %v", err)
	}

	// The DARK value is the one the widget mirrors — it always renders the dark
	// palette regardless of the embedding dashboard's theme. It is the FIRST
	// declaration in the file: the bare :root block is dark, and the light
	// theme's override comes later.
	want := firstDeclValue(t, string(css), "--status-offsite-text")
	got := firstDeclValue(t, string(widgetPage), ".offsite { color")

	if !strings.EqualFold(want, got) {
		t.Fatalf("off-site colour drifted between the two surfaces:\n"+
			"  web/src/index.css --status-offsite-text (dark) = %s\n"+
			"  internal/api/widget.html .offsite            = %s\n"+
			"These must stay byte-identical — the widget cannot read the CSS token, "+
			"so it hard-copies the hex. Change both together (issue #164).", want, got)
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
