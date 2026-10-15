package main

import (
	"bytes"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
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

	if code := healthcheckAt(ts.URL + "/api/health"); code != 0 {
		t.Fatalf("a server answering /api/health 200 must exit 0, got %d", code)
	}
	if code := healthcheckAt(ts.URL + "/elsewhere"); code != 1 {
		t.Fatalf("a non-200 answer must exit 1, got %d", code)
	}
	if code := healthcheckAt("http://127.0.0.1:1/api/health"); code != 1 {
		t.Fatalf("no server listening must exit 1, got %d", code)
	}
}

// healthServer serves /api/health with 200 over TLS or plain HTTP and records
// what net/http would log about the connections, such as a handshake error.
func healthServer(t *testing.T, ln net.Listener, useTLS bool) (port string, errorLog *bytes.Buffer) {
	t.Helper()
	ts := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	if ln != nil {
		ts.Listener = ln
	}
	errorLog = &bytes.Buffer{}
	ts.Config.ErrorLog = log.New(errorLog, "", 0)
	if useTLS {
		ts.StartTLS()
	} else {
		ts.Start()
	}
	t.Cleanup(ts.Close)
	u, err := url.Parse(ts.URL)
	if err != nil {
		t.Fatal(err)
	}
	return u.Port(), errorLog
}

func TestHealthcheckProbesTheBindAddress(t *testing.T) {
	ln, err := net.Listen("tcp", "[::1]:0")
	if err != nil {
		t.Skip("no IPv6 loopback:", err)
	}
	port, _ := healthServer(t, ln, true)

	t.Setenv("BIND_HOST", "::1")
	t.Setenv("HTTP_ONLY", "")
	t.Setenv("HTTPS_PORT", port)
	if code := healthcheck(); code != 0 {
		t.Fatalf("a WebUI bound to ::1 must pass the healthcheck, got %d", code)
	}
}

func TestHealthcheckSpeaksTheProtocolOfTheListener(t *testing.T) {
	for _, tc := range []struct {
		name     string
		httpOnly string
		useTLS   bool
	}{
		{"https", "", true},
		{"http only", "true", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			port, errorLog := healthServer(t, nil, tc.useTLS)
			t.Setenv("BIND_HOST", "")
			t.Setenv("HTTP_ONLY", tc.httpOnly)
			// The TrueNAS app sets both ports to the one it publishes.
			t.Setenv("PORT", port)
			t.Setenv("HTTPS_PORT", port)

			if code := healthcheck(); code != 0 {
				t.Fatalf("healthcheck = %d, want 0", code)
			}
			if errorLog.Len() > 0 {
				t.Fatalf("the probe made the server log %q", errorLog.String())
			}
		})
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
