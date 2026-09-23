package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/api"
	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/secret"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// fleetTestService returns a Service on an in-memory store, along with the
// store and the app key.
func fleetTestService(t *testing.T) (*api.Service, *store.Repo, string) {
	t.Helper()
	dir := t.TempDir()
	appKey := strings.Repeat("a", 64)
	cfg := config.Config{AppKey: appKey, DataDir: dir, HostMountRoot: filepath.ToSlash(dir)}
	st := newMemStore(t)
	return api.NewService(cfg, st, &fakeServiceDocker{}, fakeVirsh{}, &fakeResticEngine{}), st, appKey
}

func TestRunFleetPollsHappyPath(t *testing.T) {
	var gotToken string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/fleet/status" {
			http.NotFound(w, r)
			return
		}
		gotToken = r.Header.Get("X-Fleet-Token")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ok":           true,
			"instanceName": "tower",
			"version":      "8.0.0",
			"domains": []map[string]any{
				{"domain": "containers", "protection": "green"},
			},
		})
	}))
	defer srv.Close()

	svc, st, appKey := fleetTestService(t)
	enc, err := secret.Encrypt(appKey, []byte("peer-secret-token"))
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	peer, err := st.CreateFleetPeer(store.FleetPeer{Name: "tower", URL: srv.URL, TokenEnc: enc, Enabled: true})
	if err != nil {
		t.Fatalf("CreateFleetPeer: %v", err)
	}

	if err := svc.RunFleetPolls(context.Background()); err != nil {
		t.Fatalf("RunFleetPolls: %v", err)
	}
	if gotToken != "peer-secret-token" {
		t.Fatalf("peer received token %q, want the decrypted stored token", gotToken)
	}

	got, ok, err := st.GetFleetPeer(peer.ID)
	if err != nil || !ok {
		t.Fatalf("GetFleetPeer: ok=%v err=%v", ok, err)
	}
	if !got.LastPollOK.Valid || !got.LastPollOK.Bool {
		t.Fatalf("want LastPollOK=true, got %+v", got.LastPollOK)
	}
	if got.LastPollInstanceName != "tower" || got.LastPollVersion != "8.0.0" {
		t.Fatalf("want instanceName/version cached from the peer, got %+v", got)
	}
	if !strings.Contains(got.LastPollDomainsJSON, `"protection":"green"`) {
		t.Fatalf("want the peer's domains cached verbatim, got %q", got.LastPollDomainsJSON)
	}
	if got.LastPollAt == 0 {
		t.Fatal("want LastPollAt stamped")
	}
}

// A refused token is recorded as a failure, and the last good scorecard stays
// cached.
func TestRunFleetPollsWrongTokenFails(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "forbidden", http.StatusForbidden)
	}))
	defer srv.Close()

	svc, st, appKey := fleetTestService(t)
	enc, err := secret.Encrypt(appKey, []byte("wrong-token"))
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	peer, err := st.CreateFleetPeer(store.FleetPeer{
		Name: "tower", URL: srv.URL, TokenEnc: enc, Enabled: true,
		LastPollDomainsJSON: `[{"domain":"containers","protection":"green"}]`,
	})
	if err != nil {
		t.Fatalf("CreateFleetPeer: %v", err)
	}

	if err := svc.RunFleetPolls(context.Background()); err != nil {
		t.Fatalf("RunFleetPolls: %v", err)
	}

	got, ok, err := st.GetFleetPeer(peer.ID)
	if err != nil || !ok {
		t.Fatalf("GetFleetPeer: ok=%v err=%v", ok, err)
	}
	if !got.LastPollOK.Valid || got.LastPollOK.Bool {
		t.Fatalf("want LastPollOK=false, got %+v", got.LastPollOK)
	}
	if got.LastPollError == "" {
		t.Fatal("want a non-empty LastPollError")
	}
	if got.LastPollDomainsJSON != `[{"domain":"containers","protection":"green"}]` {
		t.Fatalf("a failed poll must keep the last-good cached scorecard, got %q", got.LastPollDomainsJSON)
	}
}

func TestRunFleetPollsSkipsDisabled(t *testing.T) {
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
	}))
	defer srv.Close()

	svc, st, appKey := fleetTestService(t)
	enc, err := secret.Encrypt(appKey, []byte("tok"))
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	peer, err := st.CreateFleetPeer(store.FleetPeer{Name: "tower", URL: srv.URL, TokenEnc: enc, Enabled: false})
	if err != nil {
		t.Fatalf("CreateFleetPeer: %v", err)
	}

	if err := svc.RunFleetPolls(context.Background()); err != nil {
		t.Fatalf("RunFleetPolls: %v", err)
	}
	if called {
		t.Fatal("a disabled peer must never be polled")
	}
	got, ok, err := st.GetFleetPeer(peer.ID)
	if err != nil || !ok {
		t.Fatalf("GetFleetPeer: ok=%v err=%v", ok, err)
	}
	if got.LastPollAt != 0 || got.LastPollOK.Valid {
		t.Fatalf("a disabled peer's poll state must stay untouched, got %+v", got)
	}
}

// Every instance serves a self-signed certificate, so a peer behind one must
// still be polled.
func TestRunFleetPollsAcceptsSelfSignedCert(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "instanceName": "tower", "version": "8.0.0", "domains": []map[string]any{}})
	}))
	defer srv.Close()

	svc, st, appKey := fleetTestService(t)
	enc, err := secret.Encrypt(appKey, []byte("tok"))
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	peer, err := st.CreateFleetPeer(store.FleetPeer{Name: "tower", URL: srv.URL, TokenEnc: enc, Enabled: true})
	if err != nil {
		t.Fatalf("CreateFleetPeer: %v", err)
	}

	if err := svc.RunFleetPolls(context.Background()); err != nil {
		t.Fatalf("RunFleetPolls: %v", err)
	}
	got, ok, err := st.GetFleetPeer(peer.ID)
	if err != nil || !ok {
		t.Fatalf("GetFleetPeer: ok=%v err=%v", ok, err)
	}
	if !got.LastPollOK.Valid || !got.LastPollOK.Bool {
		t.Fatalf("a self-signed peer cert must not fail the poll, got LastPollOK=%+v error=%q", got.LastPollOK, got.LastPollError)
	}
}
