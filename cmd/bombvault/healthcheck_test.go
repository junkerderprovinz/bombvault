package main

import (
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"
)

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

	if code := healthcheckAt(u.Hostname(), u.Port(), "1"); code != 0 {
		t.Fatalf("a server answering /api/health 200 must exit 0, got %d", code)
	}
	if code := healthcheckAt(u.Hostname(), "1", "2"); code != 1 {
		t.Fatalf("no server listening must exit 1, got %d", code)
	}
}

func TestHealthcheckProbesTheBindAddress(t *testing.T) {
	ln, err := net.Listen("tcp", "[::1]:0")
	if err != nil {
		t.Skip("no IPv6 loopback:", err)
	}
	ts := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	ts.Listener = ln
	ts.Start()
	defer ts.Close()
	port := strconv.Itoa(ln.Addr().(*net.TCPAddr).Port)

	t.Setenv("BIND_HOST", "::1")
	t.Setenv("PORT", port)
	t.Setenv("HTTPS_PORT", "1")
	if code := healthcheck(); code != 0 {
		t.Fatalf("a WebUI bound to ::1 must pass the healthcheck, got %d", code)
	}
}

func TestHealthcheckHost(t *testing.T) {
	for bind, want := range map[string]string{
		"":          "127.0.0.1",
		"0.0.0.0":   "127.0.0.1",
		"::":        "127.0.0.1",
		"127.0.0.1": "127.0.0.1",
		"10.0.0.5":  "10.0.0.5",
		"localhost": "localhost",
	} {
		if got := healthcheckHost(bind); got != want {
			t.Errorf("healthcheckHost(%q) = %q, want %q", bind, got, want)
		}
	}
}
