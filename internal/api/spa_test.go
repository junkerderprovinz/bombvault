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
