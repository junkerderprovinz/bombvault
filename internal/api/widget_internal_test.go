package api

import (
	"net/http"
	"net/http/httptest"
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
