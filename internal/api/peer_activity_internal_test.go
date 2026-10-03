package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/progress"
	"github.com/junkerderprovinz/bombvault/internal/relay"
	"github.com/junkerderprovinz/bombvault/internal/secret"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

func TestAMemberReadsWhatRunsHereOverTheGroup(t *testing.T) {
	h, _, _, _ := newMCPGateHandler(t)
	h.svc.peerActivity = h.activity
	h.progress = progress.NewStore()
	h.progress.Publish(progress.Event{Key: "container:nextcloud", Phase: "backup", Percent: 40, Active: true})

	status, body := h.svc.servePeer(context.Background(), relay.ProxyCall{Method: http.MethodGet, Path: "/api/group/peer/activity"})
	if status != http.StatusOK {
		t.Fatalf("status %d: %s", status, body)
	}
	for _, want := range []string{`"runs":[]`, `"key":"container:nextcloud"`, `"next":[]`} {
		if !strings.Contains(string(body), want) {
			t.Fatalf("answer %s lacks %s", body, want)
		}
	}
}

func TestAMemberReadsTheLookStoredHere(t *testing.T) {
	h, _, _, _ := newMCPGateHandler(t)
	if _, err := h.store.MutateSettings(func(s *store.Settings) error {
		s.DisplayPrefs = `{"bv-accent":"#ff7eb6","bv-theme":"light"}`
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	status, body := h.svc.servePeer(context.Background(), relay.ProxyCall{Method: http.MethodGet, Path: "/api/group/peer/display-prefs"})
	if status != http.StatusOK {
		t.Fatalf("status %d: %s", status, body)
	}
	for _, want := range []string{`"bv-accent":"#ff7eb6"`, `"stored":true`} {
		if !strings.Contains(string(body), want) {
			t.Fatalf("answer %s lacks %s", body, want)
		}
	}
}

func TestAPhoneOnTheInstancesPageSignsInWithoutThePassword(t *testing.T) {
	h, _, _, _ := newMCPGateHandler(t)
	h.svc.peerSession = h.phoneSession
	hash, err := secret.HashPassword(h.cfg.AppKey, "correct horse")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := h.store.MutateSettings(func(s *store.Settings) error {
		s.AuthPasswordHash = hash
		s.SessionEpoch = "e1"
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := h.store.CreateFleetPeer(store.FleetPeer{MemberID: "phone", Name: "Phone", Kind: "android", Enabled: true}); err != nil {
		t.Fatal(err)
	}

	ask := func(id string) (int, peerSessionResponse) {
		status, body := h.svc.servePeer(context.Background(), relay.ProxyCall{
			Method: http.MethodPost,
			Path:   "/api/group/peer/session",
			Body:   []byte(`{"instanceId":"` + id + `"}`),
		})
		var resp peerSessionResponse
		_ = json.Unmarshal(body, &resp)
		return status, resp
	}

	status, resp := ask("phone")
	if status != http.StatusOK || !resp.Needed || resp.Name != h.sessionCookieNameFor() {
		t.Fatalf("status %d, answer %+v", status, resp)
	}
	if !secret.ValidSessionToken(h.cfg.AppKey, hash, "e1", resp.Value) {
		t.Fatal("the session the phone got does not sign in")
	}

	if status, _ := ask("another-instance"); status != http.StatusForbidden {
		t.Fatalf("a member that is not a listed phone got status %d", status)
	}
}

func TestAPhoneNeedsNoSessionWhereThereIsNoPassword(t *testing.T) {
	h, _, _, _ := newMCPGateHandler(t)
	h.svc.peerSession = h.phoneSession
	if _, err := h.store.CreateFleetPeer(store.FleetPeer{MemberID: "phone", Name: "Phone", Kind: "android", Enabled: true}); err != nil {
		t.Fatal(err)
	}
	status, body := h.svc.servePeer(context.Background(), relay.ProxyCall{Method: http.MethodPost, Path: "/api/group/peer/session", Body: []byte(`{"instanceId":"phone"}`)})
	if status != http.StatusOK || !strings.Contains(string(body), `"needed":false`) {
		t.Fatalf("status %d: %s", status, body)
	}
}
