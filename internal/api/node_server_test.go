package api_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/api"
	"github.com/junkerderprovinz/bombvault/internal/config"
)

func TestANodeAnswersItsHealthAndNothingElse(t *testing.T) {
	h := api.NewNodeServer(config.Config{}, api.NodeRouter(nil)).Handler()
	for _, c := range []struct {
		method, path string
		want         int
	}{
		{http.MethodGet, "/api/health", http.StatusOK},
		{http.MethodGet, "/", http.StatusNotFound},
		{http.MethodGet, "/index.html", http.StatusNotFound},
		{http.MethodGet, "/settings", http.StatusNotFound},
		{http.MethodGet, "/api/status", http.StatusNotFound},
		{http.MethodGet, "/api/containers", http.StatusNotFound},
		{http.MethodGet, "/metrics", http.StatusNotFound},
		{http.MethodPost, "/mcp", http.StatusNotFound},
		{http.MethodPost, "/api/group/call", http.StatusNotFound},
	} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(c.method, c.path, nil))
		if w.Code != c.want {
			t.Errorf("%s %s = %d, want %d", c.method, c.path, w.Code, c.want)
		}
		if c.want == http.StatusOK && !strings.Contains(w.Body.String(), `"ok":true`) {
			t.Errorf("%s %s answered %s", c.method, c.path, w.Body)
		}
	}
}

func TestANodeHandsDirectCallsToItsLink(t *testing.T) {
	reached := false
	direct := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		reached = true
		w.WriteHeader(http.StatusOK)
	})
	h := api.NewNodeServer(config.Config{}, api.NodeRouter(direct)).Handler()
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/group/call", nil))
	if !reached || w.Code != http.StatusOK {
		t.Errorf("a direct call got %d, reached the link: %t", w.Code, reached)
	}
}
