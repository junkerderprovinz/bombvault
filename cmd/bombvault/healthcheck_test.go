package main

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

// The Docker healthcheck (#60) must exit 0 when /api/health answers 200 and
// non-zero when the engine is not serving.
func TestHealthcheckAt(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/health" {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	u, err := url.Parse(ts.URL)
	if err != nil {
		t.Fatal(err)
	}

	if code := healthcheckAt(u.Port(), "1"); code != 0 {
		t.Fatalf("a server answering /api/health 200 must exit 0, got %d", code)
	}
	if code := healthcheckAt("1", "2"); code != 1 {
		t.Fatalf("no server listening must exit 1, got %d", code)
	}
}
