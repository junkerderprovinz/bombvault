package group

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/coder/websocket"
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
	p := discovery.Peer{ID: id, Name: name, Version: "9.9.9", URL: srv.URL, Sent: time.Now().Unix()}
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

func TestAnAnnounceOlderThanTheSkewIsIgnored(t *testing.T) {
	a, _ := member(t, "id-a", "Cellar", testSecret, echo)
	_, pb := member(t, "id-b", "Attic", testSecret, echo)
	pb.Sent = time.Now().Add(-10 * time.Minute).Unix()
	pb.Tag = announceTag(PeerAuthKey(testSecret), pb)
	a.disc.Observe(pb)
	if got := a.Members(); len(got) != 0 {
		t.Fatalf("members = %+v, want an announce from ten minutes ago ignored", got)
	}

	_, fresh := member(t, "id-c", "Garage", testSecret, echo)
	fresh.Sent++
	a.disc.Observe(fresh)
	if got := a.Members(); len(got) != 0 {
		t.Fatalf("members = %+v, want an announce whose time was changed after signing ignored", got)
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
	resp, err := NewManager(echo).hc.Do(directRequest(url, peerAuth, sender, sent, req))
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	return resp.StatusCode
}

func directRequest(url string, peerAuth []byte, sender string, sent time.Time, req relay.ProxyRequest) *http.Request {
	payload, _ := json.Marshal(req)
	unix := strconv.FormatInt(sent.Unix(), 10)
	r, _ := http.NewRequest(http.MethodPost, url+directPath, bytes.NewReader(payload))
	r.Header.Set(headerPeer, sender)
	r.Header.Set(headerTarget, req.Target)
	r.Header.Set(headerTime, unix)
	r.Header.Set(headerSignature, directSignature(peerAuth, sender, unix, payload))
	return r
}

func sealedFor(t *testing.T, target string) relay.ProxyRequest {
	t.Helper()
	return sealedCall(t, target, relay.ProxyCall{Method: "GET", Path: "/api/group/peer/status"})
}

func sealedCall(t *testing.T, target string, call relay.ProxyCall) relay.ProxyRequest {
	t.Helper()
	id, _ := relay.NewRequestID()
	sealed, err := relay.SealCall(relay.DeriveFrameKey(testSecret), id, target, call)
	if err != nil {
		t.Fatal(err)
	}
	return relay.ProxyRequest{RequestID: id, Target: target, Sealed: sealed}
}

// countingBody reports how much of a request body the server read.
type countingBody struct {
	r    io.Reader
	read atomic.Int64
}

func (c *countingBody) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.read.Add(int64(n))
	return n, err
}

func (c *countingBody) Close() error { return nil }

func TestAStaleOrMisaddressedDirectCallIsRefusedBeforeItsBodyIsRead(t *testing.T) {
	b := NewManager(echo)
	t.Cleanup(b.Close)
	b.Apply(Config{Secret: testSecret, InstanceID: "id-b", Mode: ModeOff})
	good := PeerAuthKey(testSecret)
	for name, r := range map[string]*http.Request{
		"stale":        directRequest("", good, "id-a", time.Now().Add(-10*time.Minute), sealedFor(t, "id-b")),
		"misaddressed": directRequest("", good, "id-a", time.Now(), sealedFor(t, "id-c")),
	} {
		body := &countingBody{r: io.MultiReader(r.Body, bytes.NewReader(make([]byte, 4<<20)))}
		r.Body, r.ContentLength = body, -1
		w := httptest.NewRecorder()
		b.ServeDirect(w, r)
		if w.Code != http.StatusForbidden || body.read.Load() != 0 {
			t.Errorf("a %s call got HTTP %d after %d bytes were read, want 403 before any", name, w.Code, body.read.Load())
		}
	}
}

func TestADirectCallOverTheSizeLimitIsRefused(t *testing.T) {
	_, pb := member(t, "id-b", "Attic", testSecret, echo)
	big := sealedCall(t, "id-b", relay.ProxyCall{Method: http.MethodPost, Path: "/api/group/peer/mesh-offer", Body: bytes.Repeat([]byte("x"), MaxCallBytes)})
	if code := signedPost(t, pb.URL, PeerAuthKey(testSecret), "id-a", time.Now(), big); code != http.StatusForbidden {
		t.Fatalf("a signed call over the limit got HTTP %d, want 403", code)
	}
}

func TestDirectCallsBeyondTheCapAreTurnedAway(t *testing.T) {
	release := make(chan struct{})
	var serving atomic.Int32
	b := NewManager(func(context.Context, relay.ProxyCall) (int, []byte) {
		serving.Add(1)
		<-release
		return http.StatusOK, nil
	})
	t.Cleanup(b.Close)
	b.Apply(Config{Secret: testSecret, InstanceID: "id-b", Mode: ModeOff})
	good := PeerAuthKey(testSecret)
	var wg sync.WaitGroup
	defer func() {
		close(release)
		wg.Wait()
	}()
	for range maxDirectCalls {
		wg.Go(func() {
			b.ServeDirect(httptest.NewRecorder(), directRequest("", good, "id-a", time.Now(), sealedFor(t, "id-b")))
		})
	}
	deadline := time.Now().Add(10 * time.Second)
	for serving.Load() < maxDirectCalls && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}

	extra := make(chan int, 1)
	wg.Go(func() {
		w := httptest.NewRecorder()
		b.ServeDirect(w, directRequest("", good, "id-a", time.Now(), sealedFor(t, "id-b")))
		extra <- w.Code
	})
	select {
	case code := <-extra:
		if code != http.StatusServiceUnavailable {
			t.Fatalf("the call past the cap got HTTP %d, want 503", code)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("the call past the cap waited for a slot instead of being turned away")
	}
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

// A relay operator holds every frame it carried. One it sends on to the
// member's direct address instead must meet the same record of ids.
func TestACallRunOverTheRelayIsRefusedWhenSentAgainDirectly(t *testing.T) {
	rs := httptest.NewServer(relay.NewServer())
	t.Cleanup(rs.Close)
	b := NewManager(echo)
	mux := http.NewServeMux()
	mux.HandleFunc("POST "+directPath, b.ServeDirect)
	direct := httptest.NewTLSServer(mux)
	t.Cleanup(direct.Close)
	t.Cleanup(b.Close)
	b.Apply(Config{Secret: testSecret, InstanceID: "id-b", Name: "Attic", DirectURL: direct.URL, Mode: ModeOwn, RelayURL: rs.URL})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	ws, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(rs.URL, "http")+"/relay/connect", nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ws.CloseNow() })
	frameKey := relay.DeriveFrameKey(testSecret)
	ident, _ := relay.SealIdentity(frameKey, "id-a", relay.Identity{Name: "Cellar"})
	write := func(typ string, data any) {
		frame, _ := relay.Encode(typ, data)
		if err := ws.Write(ctx, websocket.MessageText, frame); err != nil {
			t.Fatal(err)
		}
	}
	read := func(typ string) relay.Envelope {
		for {
			_, frame, err := ws.Read(ctx)
			if err != nil {
				t.Fatalf("waiting for %s: %v", typ, err)
			}
			if env, _ := relay.Decode(frame); env.Type == typ {
				return env
			}
		}
	}
	write(relay.TypeHello, relay.Hello{Key: relay.DeriveKey(testSecret), Announce: relay.Announce{InstanceID: "id-a", Sealed: ident}})
	read(relay.TypeAnnounce)

	req := sealedFor(t, "id-b")
	write(relay.TypeProxyRequest, req)
	var resp relay.ProxyResponse
	if err := read(relay.TypeProxyResponse).Into(&resp); err != nil || resp.RequestID != req.RequestID || len(resp.Sealed) == 0 {
		t.Fatalf("the call over the relay got %+v, %v", resp, err)
	}
	if code := signedPost(t, direct.URL, PeerAuthKey(testSecret), "id-a", time.Now(), req); code != http.StatusForbidden {
		t.Fatalf("the same call sent directly got HTTP %d, want 403", code)
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

func TestAnInstanceWithAVeryLongNameStillReachesTheRelay(t *testing.T) {
	rs := httptest.NewServer(relay.NewServer())
	t.Cleanup(rs.Close)
	a, b := NewManager(echo), NewManager(echo)
	t.Cleanup(a.Close)
	t.Cleanup(b.Close)
	a.Apply(Config{Secret: testSecret, InstanceID: "id-a", Name: strings.Repeat("é", 5000), Mode: ModeOwn, RelayURL: rs.URL})
	b.Apply(Config{Secret: testSecret, InstanceID: "id-b", Name: "Attic", Mode: ModeOwn, RelayURL: rs.URL})

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) && len(b.Members()) == 0 {
		time.Sleep(20 * time.Millisecond)
	}
	got := b.Members()
	if len(got) != 1 || len(got[0].Name) == 0 || len(got[0].Name) > maxNameBytes || !utf8.ValidString(got[0].Name) {
		t.Fatalf("B's members = %+v, want A under its name clipped to whole characters", got)
	}
}
