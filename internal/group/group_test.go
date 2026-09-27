package group

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/discovery"
	"github.com/junkerderprovinz/bombvault/internal/relay"
)

var testSecret = []byte("0123456789abcdef")

func TestPeerAuthKeyIsItsOwnDomain(t *testing.T) {
	pa := PeerAuthKey(testSecret)
	if hex.EncodeToString(pa) == relay.DeriveKey(testSecret) || bytes.Equal(pa, relay.DeriveFrameKey(testSecret)) {
		t.Fatal("the peer-auth key equals another key derived from the same secret")
	}
	kl := sha256.Sum256(append([]byte("knightloader/peer-auth/v1"), testSecret...))
	if bytes.Equal(pa, kl[:]) {
		t.Fatal("the peer-auth key matches the one KnightLoader would derive")
	}
	want := sha256.Sum256(append([]byte("bombvault/peer-auth/v1"), testSecret...))
	if !bytes.Equal(pa, want[:]) {
		t.Fatalf("PeerAuthKey = %x, want %x; a changed derivation breaks every existing group", pa, want)
	}
}

// echo answers every call with its own method and path, so a test can see
// which call arrived.
func echo(_ context.Context, call relay.ProxyCall) (int, []byte) {
	return http.StatusOK, []byte(call.Method + " " + call.Path + " " + string(call.Body))
}

// member runs one Manager behind a TLS test server and returns it with the
// announce it would send.
func member(t *testing.T, id, name string, secret []byte, serve relay.Handler) (*Manager, discovery.Peer) {
	t.Helper()
	m := NewManager(serve)
	mux := http.NewServeMux()
	mux.HandleFunc("POST "+directPath, m.ServeDirect)
	srv := httptest.NewTLSServer(mux)
	t.Cleanup(srv.Close)
	t.Cleanup(m.Close)
	m.Apply(Config{Secret: secret, InstanceID: id, Name: name, Version: "9.9.9", DirectURL: srv.URL, Mode: ModeOff})
	p := discovery.Peer{ID: id, Name: name, Version: "9.9.9", URL: srv.URL}
	if secret != nil {
		p.Tag = announceTag(PeerAuthKey(secret), p)
	}
	return m, p
}

func TestMembersOnOneNetworkCallEachOtherDirectly(t *testing.T) {
	a, pa := member(t, "id-a", "Cellar", testSecret, echo)
	b, pb := member(t, "id-b", "Attic", testSecret, echo)
	a.disc.Observe(pb)
	b.disc.Observe(pa)

	got := a.Members()
	if len(got) != 1 || got[0].ID != "id-b" || !got[0].Direct || got[0].Relay || got[0].Version != "9.9.9" {
		t.Fatalf("A's members = %+v, want B reachable directly", got)
	}
	status, body, err := a.Call(context.Background(), "id-b", http.MethodGet, "/api/group/peer/status", nil)
	if err != nil || status != http.StatusOK || string(body) != "GET /api/group/peer/status " {
		t.Fatalf("Call = %d %q %v, want B's echo", status, body, err)
	}
}

func TestAnnounceFromAnotherGroupIsNotAMember(t *testing.T) {
	a, _ := member(t, "id-a", "Cellar", testSecret, echo)
	_, stranger := member(t, "id-x", "Stranger", []byte("fedcba9876543210"), echo)
	a.disc.Observe(stranger)
	if got := a.Members(); len(got) != 0 {
		t.Fatalf("members = %+v, want nobody from another group", got)
	}
}

func TestAnnounceWithARewrittenAddressIsIgnored(t *testing.T) {
	a, _ := member(t, "id-a", "Cellar", testSecret, echo)
	_, pb := member(t, "id-b", "Attic", testSecret, echo)
	pb.URL = "https://192.0.2.66:3443"
	a.disc.Observe(pb)
	if got := a.Members(); len(got) != 0 {
		t.Fatalf("members = %+v, want an announce with a forged address ignored", got)
	}
}

// signedPost sends a direct call the way callDirect does, with the parts a
// test wants to spoil.
func signedPost(t *testing.T, url string, peerAuth []byte, sender string, sent time.Time, req relay.ProxyRequest) int {
	t.Helper()
	payload, _ := json.Marshal(req)
	unix := strconv.FormatInt(sent.Unix(), 10)
	r, _ := http.NewRequest(http.MethodPost, url+directPath, bytes.NewReader(payload))
	r.Header.Set(headerPeer, sender)
	r.Header.Set(headerTime, unix)
	r.Header.Set(headerSignature, directSignature(peerAuth, sender, unix, payload))
	resp, err := NewManager(echo).hc.Do(r)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	return resp.StatusCode
}

func sealedFor(t *testing.T, target string) relay.ProxyRequest {
	t.Helper()
	id, _ := relay.NewRequestID()
	sealed, err := relay.SealCall(relay.DeriveFrameKey(testSecret), id, target, relay.ProxyCall{Method: "GET", Path: "/api/group/peer/status"})
	if err != nil {
		t.Fatal(err)
	}
	return relay.ProxyRequest{RequestID: id, Target: target, Sealed: sealed}
}

func TestDirectCallIsRefusedUnlessSignedFreshAndNew(t *testing.T) {
	_, pb := member(t, "id-b", "Attic", testSecret, echo)
	good := PeerAuthKey(testSecret)

	if code := signedPost(t, pb.URL, good, "id-a", time.Now(), sealedFor(t, "id-b")); code != http.StatusOK {
		t.Fatalf("a correct call got HTTP %d, so the refusals below prove nothing", code)
	}
	if code := signedPost(t, pb.URL, PeerAuthKey([]byte("fedcba9876543210")), "id-a", time.Now(), sealedFor(t, "id-b")); code != http.StatusForbidden {
		t.Errorf("a call signed with another group's key got HTTP %d, want 403", code)
	}
	if code := signedPost(t, pb.URL, good, "id-a", time.Now().Add(-10*time.Minute), sealedFor(t, "id-b")); code != http.StatusForbidden {
		t.Errorf("a stale call got HTTP %d, want 403", code)
	}
	if code := signedPost(t, pb.URL, good, "id-a", time.Now(), sealedFor(t, "id-c")); code != http.StatusForbidden {
		t.Errorf("a call for another instance got HTTP %d, want 403", code)
	}
	replayed := sealedFor(t, "id-b")
	if code := signedPost(t, pb.URL, good, "id-a", time.Now(), replayed); code != http.StatusOK {
		t.Fatalf("first sending got HTTP %d", code)
	}
	if code := signedPost(t, pb.URL, good, "id-a", time.Now(), replayed); code != http.StatusForbidden {
		t.Errorf("a replayed call got HTTP %d, want 403", code)
	}
}

func TestOutsideAGroupTheDirectRouteDoesNotExist(t *testing.T) {
	_, pb := member(t, "id-b", "Attic", nil, echo)
	if code := signedPost(t, pb.URL, PeerAuthKey(testSecret), "id-a", time.Now(), sealedFor(t, "id-b")); code != http.StatusNotFound {
		t.Fatalf("got HTTP %d, want 404 from an instance in no group", code)
	}
}

func TestMembersOnDifferentNetworksCallThroughTheRelay(t *testing.T) {
	srv := relay.NewServer()
	rs := httptest.NewServer(srv)
	t.Cleanup(rs.Close)

	a := NewManager(echo)
	b := NewManager(echo)
	t.Cleanup(a.Close)
	t.Cleanup(b.Close)
	a.Apply(Config{Secret: testSecret, InstanceID: "id-a", Name: "Cellar", Mode: ModeOwn, RelayURL: rs.URL})
	b.Apply(Config{Secret: testSecret, InstanceID: "id-b", Name: "Attic", Version: "9.9.9", Mode: ModeOwn, RelayURL: rs.URL})

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) && len(a.Members()) == 0 {
		time.Sleep(20 * time.Millisecond)
	}
	got := a.Members()
	if len(got) != 1 || got[0].ID != "id-b" || !got[0].Relay || got[0].Direct || got[0].Name != "Attic" {
		t.Fatalf("A's members = %+v, want B through the relay", got)
	}
	status, body, err := a.Call(context.Background(), "id-b", http.MethodPost, "/api/group/peer/check", []byte("x"))
	if err != nil || status != http.StatusOK || string(body) != "POST /api/group/peer/check x" {
		t.Fatalf("Call = %d %q %v", status, body, err)
	}
	if !a.AdmitsRelayKey(relay.DeriveKey(testSecret)) || a.AdmitsRelayKey(relay.DeriveKey([]byte("fedcba9876543210"))) {
		t.Fatal("AdmitsRelayKey does not admit exactly this group's key")
	}
}

func TestLeavingTheGroupForgetsEveryMember(t *testing.T) {
	a, _ := member(t, "id-a", "Cellar", testSecret, echo)
	_, pb := member(t, "id-b", "Attic", testSecret, echo)
	a.disc.Observe(pb)
	a.Apply(Config{InstanceID: "id-a"})
	if a.Active() || len(a.Members()) != 0 {
		t.Fatal("an instance that left still reports a group")
	}
	if _, _, err := a.Call(context.Background(), "id-b", "GET", "/api/group/peer/status", nil); err == nil {
		t.Fatal("an instance that left could still call a member")
	}
}
