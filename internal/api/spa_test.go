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
	// A client-side route with no matching file gets index.html.
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

// /metrics and /widget live outside /api but belong to the API router. A scraper
// or iframe that got index.html would break without any error.
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
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/does-not-exist", nil))
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for unknown api route, got %d body=%q", w.Code, w.Body.String())
	}
}

// The shell is never cached and hashed assets are cached for a year. Both ways
// to the shell are checked: a client-side route goes through serveIndex,
// "/index.html" through the file server.
func TestSPACacheHeaders(t *testing.T) {
	h := api.NewSPAHandler(testSPAFS(), http.NewServeMux())

	for _, path := range []string{"/dashboard", "/index.html", "/"} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		if got := w.Header().Get("Cache-Control"); got != "no-store, must-revalidate" {
			t.Errorf("%s: Cache-Control = %q, want the shell to be uncacheable", path, got)
		}
	}

	// A new build gives a hashed asset a new name, so a long cache is safe.
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/assets/app.js", nil))
	if got := w.Header().Get("Cache-Control"); got != "public, max-age=31536000, immutable" {
		t.Errorf("asset Cache-Control = %q, want a long immutable cache", got)
	}

	// Other files keep whatever the file server sends.
	w = httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/favicon.ico", nil))
	if got := w.Header().Get("Cache-Control"); got != "" {
		t.Errorf("favicon Cache-Control = %q, want it left alone", got)
	}
}

// A stale index.html asking for a chunk that no longer exists gets a 404, not
// markup it would try to parse as JavaScript.
func TestAssetsMissDoesNotFallBack(t *testing.T) {
	h := api.NewSPAHandler(testSPAFS(), http.NewServeMux())

	for _, path := range []string{"/assets/gone.js", "/assets/index-abc123.js.map", "/assets/style.css"} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		if w.Code != http.StatusNotFound {
			t.Errorf("%s: status = %d, want 404 rather than the SPA index", path, w.Code)
		}
	}

	// Existing assets and top-level client routes are unaffected.
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/assets/app.js", nil))
	if w.Code != http.StatusOK {
		t.Errorf("existing asset: status = %d, want 200", w.Code)
	}
	w = httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/dashboard", nil))
	if w.Code != http.StatusOK {
		t.Errorf("client route: status = %d, want the SPA index", w.Code)
	}
}
