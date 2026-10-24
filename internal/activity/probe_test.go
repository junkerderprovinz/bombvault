package activity

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// execRecorder is the way through the container, answering with out and code.
type execRecorder struct {
	out   string
	code  int
	err   error
	calls [][]string
}

func (e *execRecorder) exec(_ context.Context, argv []string) (string, int, error) {
	e.calls = append(e.calls, argv)
	return e.out, e.code, e.err
}

// serverAt starts a fake media server and returns its address as host, with
// the preset's port mapped onto the test server by a client that dials it.
func serverAt(t *testing.T, h http.HandlerFunc) (*http.Client, string) {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	target := strings.TrimPrefix(srv.URL, "http://")
	client := &http.Client{Transport: &http.Transport{
		DialContext: func(ctx context.Context, network, _ string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, network, target)
		},
	}, Timeout: 5 * time.Second}
	return client, "172.17.0.5"
}

func TestAReachableServerIsAskedDirectly(t *testing.T) {
	var gotPath, gotToken string
	client, addr := serverAt(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotToken = r.URL.Path, r.Header.Get("X-Plex-Token")
		_, _ = w.Write([]byte(`<MediaContainer size="0"></MediaContainer>`))
	})
	ex := &execRecorder{}
	body, via, err := Probe{Preset: Plex, Token: "tok", Addr: addr, Client: client, Exec: ex.exec}.Fetch(context.Background())
	if err != nil || via != ViaDirect {
		t.Fatalf("via = %q, err = %v", via, err)
	}
	if gotPath != "/status/sessions" || gotToken != "tok" || !strings.Contains(string(body), "MediaContainer") {
		t.Fatalf("path %q, token %q, body %q", gotPath, gotToken, body)
	}
	if len(ex.calls) != 0 {
		t.Fatalf("the container was asked too: %v", ex.calls)
	}
}

func TestJellyfinAndEmbyGetTheirOwnHeader(t *testing.T) {
	for preset, want := range map[Preset][2]string{
		Jellyfin: {"Authorization", `MediaBrowser Token="key"`},
		Emby:     {"X-Emby-Token", "key"},
	} {
		var got string
		client, addr := serverAt(t, func(w http.ResponseWriter, r *http.Request) {
			got = r.Header.Get(want[0])
			_, _ = w.Write([]byte(`[]`))
		})
		if _, _, err := (Probe{Preset: preset, Token: "key", Addr: addr, Client: client}).Fetch(context.Background()); err != nil {
			t.Fatal(err)
		}
		if got != want[1] {
			t.Errorf("%s sent %s %q, want %q", preset, want[0], got, want[1])
		}
	}
}

func TestAServerBombVaultCannotReachIsAskedThroughTheContainer(t *testing.T) {
	ex := &execRecorder{out: `<MediaContainer size="0"></MediaContainer>`}
	failing := &http.Client{Transport: &http.Transport{
		DialContext: func(context.Context, string, string) (net.Conn, error) {
			return nil, errors.New("no route to host")
		},
	}}
	body, via, err := Probe{Preset: Plex, Token: "tok", Addr: "192.168.1.50", Client: failing, Exec: ex.exec}.Fetch(context.Background())
	if err != nil || via != ViaExec || !strings.Contains(string(body), "MediaContainer") {
		t.Fatalf("via = %q, err = %v, body %q", via, err, body)
	}
	if len(ex.calls) != 1 {
		t.Fatalf("exec calls = %v", ex.calls)
	}
	argv := ex.calls[0]
	if argv[0] != "sh" || argv[4] != "http://127.0.0.1:32400/status/sessions" || argv[5] != "X-Plex-Token" || argv[6] != "tok" {
		t.Fatalf("argv = %q", argv)
	}
}

func TestAContainerWithoutAnAddressIsAskedThroughItself(t *testing.T) {
	ex := &execRecorder{out: `[]`}
	_, via, err := Probe{Preset: Emby, Token: "k", Exec: ex.exec}.Fetch(context.Background())
	if err != nil || via != ViaExec || len(ex.calls) != 1 {
		t.Fatalf("via = %q, err = %v, calls %d", via, err, len(ex.calls))
	}
	if !strings.Contains(ex.calls[0][4], ":8096/Sessions") {
		t.Fatalf("url = %q", ex.calls[0][4])
	}
}

func TestAContainerWithoutCurlOrWgetSaysSo(t *testing.T) {
	ex := &execRecorder{code: 127}
	_, _, err := Probe{Preset: Plex, Exec: ex.exec}.Fetch(context.Background())
	if err == nil || !strings.Contains(err.Error(), "neither curl nor wget") {
		t.Fatalf("err = %v", err)
	}
}

func TestBothWaysFailingNamesBoth(t *testing.T) {
	client, addr := serverAt(t, func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "nope", http.StatusUnauthorized)
	})
	ex := &execRecorder{code: 22}
	_, _, err := Probe{Preset: Jellyfin, Token: "bad", Addr: addr, Client: client, Exec: ex.exec}.Fetch(context.Background())
	if err == nil || !strings.Contains(err.Error(), "401") || !strings.Contains(err.Error(), "exit code 22") {
		t.Fatalf("err = %v", err)
	}
}
