package api

import (
	"net/http"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/secret"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// TestProposeMeshOfferCreatesAWaitingReceiver checks that sending an offer
// sets up the sender's own receiver right away, sealed with the rest-server
// login the deploy snippet hands out, rather than leaving the admin to add it
// by hand once the container exists.
func TestProposeMeshOfferCreatesAWaitingReceiver(t *testing.T) {
	a := newInstance(t, "cellar", strings.Repeat("a5", 32))
	b := newInstance(t, "attic", strings.Repeat("b6", 32))
	pairThroughRelay(t, a, b)

	peer, err := a.st.CreateFleetPeer(store.FleetPeer{MemberID: b.id(t), Name: "attic", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}

	code, out := a.do(t, http.MethodPost, "/api/fleet/peers/"+peer.ID+"/mesh-offer", map[string]any{
		"domain": "containers", "baseUrl": "http://192.168.1.50:8000",
	})
	if code != http.StatusOK || out["ok"] != true {
		t.Fatalf("propose: %d %v", code, out)
	}
	snippet, _ := out["snippet"].(map[string]any)
	user, _ := snippet["user"].(string)
	password, _ := snippet["password"].(string)
	repo, _ := snippet["repo"].(string)
	if user == "" || password == "" || repo == "" {
		t.Fatalf("snippet missing fields: %v", snippet)
	}

	repos, err := a.st.ListReceivedRepos()
	if err != nil || len(repos) != 1 {
		t.Fatalf("received repos on the sender = %+v, %v", repos, err)
	}
	rr := repos[0]
	if rr.MemberID != b.id(t) {
		t.Fatalf("receiver row paired with the wrong member: %+v", rr)
	}
	if !rr.Waiting {
		t.Fatal("a receiver created before the rest-server is deployed must wait, not claim to be reachable")
	}
	if rr.Repo != repo {
		t.Fatalf("receiver repo = %q, want the same location sent to the peer %q", rr.Repo, repo)
	}
	if rr.RESTUser != user {
		t.Fatalf("receiver rest_user = %q, want %q", rr.RESTUser, user)
	}
	decPass, err := secret.Decrypt(a.appKey, rr.RESTPasswordEnc)
	if err != nil || string(decPass) != password {
		t.Fatalf("stored rest-server password = %q err=%v, want %q", decPass, err, password)
	}
	decRestic, err := secret.Decrypt(a.appKey, rr.ResticPasswordEnc)
	if err != nil || len(decRestic) == 0 {
		t.Fatalf("receiver row must hold the peer's restic password: %v", err)
	}

	// The peer still only has a pending offer to accept; propose alone must
	// not act on B's behalf.
	offers, err := b.st.ListMeshOffers()
	if err != nil || len(offers) != 1 || offers[0].Status != "pending" {
		t.Fatalf("peer offers = %+v, %v", offers, err)
	}
}

// TestProposeMeshOfferNeverLeaksTheSealedCredentialsBack checks that the
// snippet the propose call answers with carries the plaintext deploy password
// (the admin still has to paste it into the rest-server once), but the
// receiver list that same instance later serves never repeats the sealed
// restic or rest-server password it just stored.
func TestProposeMeshOfferNeverLeaksTheSealedCredentialsBack(t *testing.T) {
	a := newInstance(t, "cellar", strings.Repeat("a7", 32))
	b := newInstance(t, "attic", strings.Repeat("b8", 32))
	pairThroughRelay(t, a, b)
	peer, err := a.st.CreateFleetPeer(store.FleetPeer{MemberID: b.id(t), Name: "attic", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	if code, out := a.do(t, http.MethodPost, "/api/fleet/peers/"+peer.ID+"/mesh-offer", map[string]any{
		"domain": "containers", "baseUrl": "http://192.168.1.50:8000",
	}); code != http.StatusOK || out["ok"] != true {
		t.Fatalf("propose: %d %v", code, out)
	}

	code, out := a.do(t, http.MethodGet, "/api/receiver/repos", nil)
	if code != http.StatusOK || out["ok"] != true {
		t.Fatalf("list: %d %v", code, out)
	}
	repos, _ := out["repos"].([]any)
	if len(repos) != 1 {
		t.Fatalf("want 1 receiver row, got %d", len(repos))
	}
	row, _ := repos[0].(map[string]any)
	for _, field := range []string{"resticPassword", "resticPasswordEnc", "restPassword", "restPasswordEnc", "appKey"} {
		if _, leaked := row[field]; leaked {
			t.Fatalf("the receiver list carries %s", field)
		}
	}
}
