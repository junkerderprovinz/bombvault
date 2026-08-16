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

// fleetTestService builds a Service backed by a fresh in-memory store, mirroring
// drDrillService's shape but with no restic-engine/docker/virsh behavior needed
// (RunFleetPolls never touches any of them).
func fleetTestService(t *testing.T) (*api.Service, *store.Repo, string) {
	t.Helper()
	dir := t.TempDir()
	appKey := strings.Repeat("a", 64)
	cfg := config.Config{AppKey: appKey, DataDir: dir, HostMountRoot: filepath.ToSlash(dir)}
	st := newMemStore(t)
	return api.NewService(cfg, st, &fakeServiceDocker{}, fakeVirsh{}, &fakeResticEngine{}), st, appKey
}

// TestRunFleetPollsHappyPath pins the real peer-poll flow end to end: a fake
// peer server answers GET /api/fleet/status with a valid token, RunFleetPolls
// records success plus the peer's reported instance name/version/domains.
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

// TestRunFleetPollsWrongTokenFails pins that a peer refusing the presented
// token (403, as the real fleetGate would answer) is recorded as a failure
// with a scrubbed detail, and the LAST-GOOD cached scorecard (if any) is left
// untouched rather than being wiped by a failed poll.
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

// TestRunFleetPollsSkipsDisabled pins that a disabled peer is never contacted
// and its poll state is left completely untouched.
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

// TestRunFleetPollsAcceptsSelfSignedCert pins that a peer served over HTTPS
// with an untrusted (self-signed) certificate is still polled successfully —
// discovered live: every BombVault instance's own default cert is self-signed
// and scoped to loopback names, never valid for the real address a peer would
// actually be reached at, so strict TLS verification here would make the
// feature fail against a completely standard BombVault install.
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
