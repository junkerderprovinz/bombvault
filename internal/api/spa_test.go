package api_test

import (
	"io/fs"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"

	"github.com/junkerderprovinz/bombvault/internal/api"
)

func testSPAFS() fs.FS {
	return fstest.MapFS{
		"index.html":    {Data: []byte("<html>spa-root</html>")},
		"assets/app.js": {Data: []byte("console.log('app')")},
		"favicon.ico":   {Data: []byte("icon")},
	}
}

func TestSPAServesStaticAsset(t *testing.T) {
	apiMux := http.NewServeMux()
	apiMux.HandleFunc("GET /api/health", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"ok":true}`))
	})
	h := api.NewSPAHandler(testSPAFS(), apiMux)

	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/assets/app.js", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	if w.Body.String() != "console.log('app')" {
		t.Fatalf("asset body = %q", w.Body.String())
	}
}

func TestSPAFallsBackToIndexForClientRoute(t *testing.T) {
	apiMux := http.NewServeMux()
	h := api.NewSPAHandler(testSPAFS(), apiMux)

	w := httptest.NewRecorder()
	// A deep client-side route with no matching file → index.html.
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/settings/encryption", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	if w.Body.String() != "<html>spa-root</html>" {
		t.Fatalf("expected index fallback, got %q", w.Body.String())
	}
}

func TestSPADelegatesAPIRoutes(t *testing.T) {
	apiMux := http.NewServeMux()
	apiMux.HandleFunc("GET /api/health", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"ok":true}`))
	})
	h := api.NewSPAHandler(testSPAFS(), apiMux)

	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/health", nil))
	if w.Code != http.StatusOK || w.Body.String() != `{"ok":true}` {
		t.Fatalf("api delegation failed: code=%d body=%q", w.Code, w.Body.String())
	}
}

// The API-owned pages OUTSIDE /api (/metrics for Prometheus, /widget for the
// embeddable dashboard widget) must reach the API router — never the SPA index
// fallback (a scrape/iframe getting index.html would be a silent breakage).
func TestSPADelegatesAPIOwnedPages(t *testing.T) {
	apiMux := http.NewServeMux()
	apiMux.HandleFunc("GET /metrics", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("metrics-body"))
	})
	apiMux.HandleFunc("GET /widget", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("widget-body"))
	})
	h := api.NewSPAHandler(testSPAFS(), apiMux)

	for path, want := range map[string]string{"/metrics": "metrics-body", "/widget": "widget-body"} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		if w.Code != http.StatusOK || w.Body.String() != want {
			t.Fatalf("%s delegation failed: code=%d body=%q", path, w.Code, w.Body.String())
		}
	}
}

func TestSPAUnknownAPIRouteDoesNotFallBack(t *testing.T) {
	apiMux := http.NewServeMux()
	apiMux.HandleFunc("GET /api/health", func(w http.ResponseWriter, _ *http.Request) {})
	h := api.NewSPAHandler(testSPAFS(), apiMux)

	w := httptest.NewRecorder()
	// An unknown /api/ route must 404 (NOT serve index.html as if it were a route).
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/does-not-exist", nil))
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for unknown api route, got %d body=%q", w.Code, w.Body.String())
	}
}

// TestSPACacheHeaders pins the split the whole point of [333] rests on: the
// shell must never be cached, the hashed assets should be cached forever.
//
// Both PATHS to the shell are checked, because the fix's first cut set the
// header on only one of them and looked complete: a client-side route reaches
// serveIndex, while "/index.html" by name goes through the file server. A
// browser landing on the app hits the first, a reload of a bookmarked file URL
// the second, and half a fix here is indistinguishable from none - the symptom
// is a page that reports the previous deploy's version and no amount of
// deploying corrects it ([266], and the same confusion again while verifying a
// deploy on 2026-08-31).
func TestSPACacheHeaders(t *testing.T) {
	h := api.NewSPAHandler(testSPAFS(), http.NewServeMux())

	for _, path := range []string{"/dashboard", "/index.html", "/"} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		if got := w.Header().Get("Cache-Control"); got != "no-store, must-revalidate" {
			t.Errorf("%s: Cache-Control = %q, want the shell to be uncacheable", path, got)
		}
	}

	// A content-hashed asset is immutable by construction - a new build gives
	// it a new NAME - so caching it for a year costs nothing and is what makes
	// an uncacheable shell cheap.
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/assets/app.js", nil))
	if got := w.Header().Get("Cache-Control"); got != "public, max-age=31536000, immutable" {
		t.Errorf("asset Cache-Control = %q, want a long immutable cache", got)
	}

	// Everything else keeps whatever the file server decides. A favicon is
	// neither hashed nor the shell, and inventing a policy for it here would be
	// this test claiming more than the change actually made.
	w = httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/favicon.ico", nil))
	if got := w.Header().Get("Cache-Control"); got != "" {
		t.Errorf("favicon Cache-Control = %q, want it left alone", got)
	}
}
