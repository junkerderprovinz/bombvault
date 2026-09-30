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
	"slices"
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

func TestMembersCarryTheAddressTheyAnswerOn(t *testing.T) {
	a, _ := member(t, "id-a", "Cellar", testSecret, echo)
	_, pb := member(t, "id-b", "Attic", testSecret, echo)
	a.disc.Observe(pb)
	a.ConfirmAddress("id-c", "https://192.168.1.10:3443", "Garage", "9.9.9")

	got := map[string]string{}
	for _, m := range a.Members() {
		got[m.ID] = m.Address
	}
	want := map[string]string{"id-b": pb.URL, "id-c": "https://192.168.1.10:3443"}
	if len(got) != len(want) || got["id-b"] != want["id-b"] || got["id-c"] != want["id-c"] {
		t.Fatalf("member addresses = %v, want %v", got, want)
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

// helloServe answers only ProbePath, the way a real instance's peer/hello
// route does, so a probe test does not need the api package's handler.
func helloServe(id, name, version string) relay.Handler {
	return func(_ context.Context, call relay.ProxyCall) (int, []byte) {
		if call.Path != ProbePath {
			return http.StatusNotFound, nil
		}
		b, _ := json.Marshal(Hello{OK: true, InstanceID: id, Name: name, Version: version})
		return http.StatusOK, b
	}
}

func TestProbeConfirmsAGroupMemberAtAnUnknownAddress(t *testing.T) {
	a := NewManager(echo)
	t.Cleanup(a.Close)
	a.Apply(Config{Secret: testSecret, InstanceID: "id-a", Mode: ModeOff})

	b := NewManager(helloServe("id-b", "Attic", "9.9.9"))
	mux := http.NewServeMux()
	mux.HandleFunc("POST "+directPath, b.ServeDirect)
	srv := httptest.NewTLSServer(mux)
	t.Cleanup(srv.Close)
	t.Cleanup(b.Close)
	b.Apply(Config{Secret: testSecret, InstanceID: "id-b", Mode: ModeOff})

	h, err := a.Probe(context.Background(), srv.URL)
	if err != nil || h.InstanceID != "id-b" || h.Name != "Attic" {
		t.Fatalf("Probe = %+v, %v", h, err)
	}
	got := a.Members()
	if len(got) != 1 || got[0].ID != "id-b" || !got[0].Direct || got[0].Name != "Attic" {
		t.Fatalf("members after a successful probe = %+v, want b listed and direct", got)
	}
}

// A member found only by its address, with no relay and no multicast, would
// drop out once its confirmation ages past addrTTL; the refresh asks it again
// before that, with nobody opening a page.
func TestRefreshKeepsAMemberFoundByAddressInTheGroup(t *testing.T) {
	a := NewManager(echo)
	t.Cleanup(a.Close)
	a.Apply(Config{Secret: testSecret, InstanceID: "id-a", Mode: ModeOff})

	b := NewManager(helloServe("id-b", "Attic", "9.9.9"))
	mux := http.NewServeMux()
	mux.HandleFunc("POST "+directPath, b.ServeDirect)
	srv := httptest.NewTLSServer(mux)
	t.Cleanup(srv.Close)
	t.Cleanup(b.Close)
	b.Apply(Config{Secret: testSecret, InstanceID: "id-b", Mode: ModeOff})

	if _, err := a.Probe(context.Background(), srv.URL); err != nil {
		t.Fatal(err)
	}
	age := func() {
		a.addrMu.Lock()
		a.addrs["id-b"].confirmed = time.Now().Add(-addrTTL - time.Second)
		a.addrMu.Unlock()
	}

	age()
	if got := a.Members(); len(got) != 0 {
		t.Fatalf("members with an aged address = %+v, want nobody before a refresh", got)
	}
	a.refresh(context.Background())
	if got := a.Members(); len(got) != 1 || got[0].ID != "id-b" || !got[0].Direct {
		t.Fatalf("members after a refresh = %+v, want b listed and direct again", got)
	}
	if refreshEvery >= addrTTL {
		t.Fatalf("refreshEvery %v must come round before addrTTL %v runs out", refreshEvery, addrTTL)
	}
}

// The member a probe reaches learns the prober from the same exchange, so the
// two know each other without waiting for the other side's own sweep.
func TestAProbedMemberProbesBackAndKnowsTheProber(t *testing.T) {
	serveOn := func(m *Manager) *httptest.Server {
		mux := http.NewServeMux()
		mux.HandleFunc("POST "+directPath, m.ServeDirect)
		srv := httptest.NewTLSServer(mux)
		t.Cleanup(srv.Close)
		return srv
	}
	a := NewManager(helloServe("id-a", "Cellar", "9.9.9"))
	t.Cleanup(a.Close)
	srvA := serveOn(a)
	a.Apply(Config{Secret: testSecret, InstanceID: "id-a", Name: "Cellar", Mode: ModeOff, DirectURL: srvA.URL})

	b := NewManager(helloServe("id-b", "Attic", "9.9.9"))
	t.Cleanup(b.Close)
	srvB := serveOn(b)
	b.Apply(Config{Secret: testSecret, InstanceID: "id-b", Name: "Attic", Mode: ModeOff, DirectURL: srvB.URL})

	if _, err := a.Probe(context.Background(), srvB.URL); err != nil {
		t.Fatalf("Probe: %v", err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for !b.confirmedDirect("id-a") {
		if time.Now().After(deadline) {
			t.Fatalf("B's members after A probed it = %+v, want A confirmed direct", b.Members())
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// A probe against a host running BombVault for a different group must find
// nothing: the signature is checked with this group's key, not the
// stranger's.
func TestProbeOfANonMemberFindsNothing(t *testing.T) {
	a := NewManager(echo)
	t.Cleanup(a.Close)
	a.Apply(Config{Secret: testSecret, InstanceID: "id-a", Mode: ModeOff})

	stranger := NewManager(helloServe("id-x", "Stranger", "1.0.0"))
	mux := http.NewServeMux()
	mux.HandleFunc("POST "+directPath, stranger.ServeDirect)
	srv := httptest.NewTLSServer(mux)
	t.Cleanup(srv.Close)
	t.Cleanup(stranger.Close)
	stranger.Apply(Config{Secret: []byte("fedcba9876543210"), InstanceID: "id-x", Mode: ModeOff})

	if _, err := a.Probe(context.Background(), srv.URL); err == nil {
		t.Fatal("a probe of an instance in another group was not refused")
	}
	if got := a.Members(); len(got) != 0 {
		t.Fatalf("members after a failed probe = %+v, want none", got)
	}
}

// A call sealed for ProbeTarget must reach the hello route and nothing else,
// so a blind probe cannot be turned into a call against a route meant for a
// known member.
func TestOnlyTheHelloRouteAcceptsAProbeTargetedCall(t *testing.T) {
	b := NewManager(echo)
	t.Cleanup(b.Close)
	b.Apply(Config{Secret: testSecret, InstanceID: "id-b", Mode: ModeOff})
	good := PeerAuthKey(testSecret)

	other := sealedCall(t, ProbeTarget, relay.ProxyCall{Method: http.MethodGet, Path: "/api/group/peer/status"})
	w := httptest.NewRecorder()
	r := directRequest("", good, "id-a", time.Now(), other)
	b.ServeDirect(w, r)
	if w.Code != http.StatusForbidden {
		t.Fatalf("a probe-targeted call to a non-hello path got HTTP %d, want 403", w.Code)
	}

	hello := sealedCall(t, ProbeTarget, relay.ProxyCall{Method: http.MethodGet, Path: ProbePath})
	w2 := httptest.NewRecorder()
	r2 := directRequest("", good, "id-a", time.Now(), hello)
	b.ServeDirect(w2, r2)
	if w2.Code != http.StatusOK {
		t.Fatalf("a probe-targeted call to the hello path got HTTP %d, want 200", w2.Code)
	}
}

func TestSubnetCandidatesOnlyForAPrivateIPv4OwnAddress(t *testing.T) {
	for _, own := range []string{"", "not a url", "https://8.8.8.8:3443", "https://bombvault.example.org:3443", "https://203.0.113.5:3443"} {
		if got := subnetCandidates(own); got != nil {
			t.Errorf("subnetCandidates(%q) = %d candidates, want none for a non-private address", own, len(got))
		}
	}

	got := subnetCandidates("https://192.168.1.5:3443")
	if len(got) != 253 {
		t.Fatalf("subnetCandidates on the default port = %d candidates, want 253 (every other host of the /24, one port)", len(got))
	}
	for _, addr := range got {
		if strings.Contains(addr, "192.168.1.5:") {
			t.Fatalf("subnetCandidates included this instance's own address: %v", got)
		}
		if !strings.HasPrefix(addr, "https://192.168.1.") {
			t.Fatalf("subnetCandidates left its own /24: %q", addr)
		}
	}

	got2 := subnetCandidates("http://10.0.5.9:8080")
	if len(got2) != 253*2+1 {
		t.Fatalf("subnetCandidates on a non-default port = %d candidates, want 253*2+1 (own port and 3443, plus 3443 on this machine)", len(got2))
	}
	if !slices.Contains(got2, "http://10.0.5.9:3443") || slices.Contains(got2, "http://10.0.5.9:8080") {
		t.Fatalf("subnetCandidates must try a second instance on this machine's other port and never itself: %v", got2)
	}
	sawOwnPort, saw3443 := false, false
	for _, addr := range got2 {
		if strings.HasSuffix(addr, ":8080") {
			sawOwnPort = true
		}
		if strings.HasSuffix(addr, ":3443") {
			saw3443 = true
		}
		if !strings.HasPrefix(addr, "http://10.0.5.") {
			t.Fatalf("subnetCandidates left its own /24: %q", addr)
		}
	}
	if !sawOwnPort || !saw3443 {
		t.Fatalf("subnetCandidates did not probe both its own port and 3443: %v", got2[:4])
	}
}

func TestSweepFindsAMemberAmongUnreachableDecoys(t *testing.T) {
	a := NewManager(echo)
	t.Cleanup(a.Close)
	a.Apply(Config{Secret: testSecret, InstanceID: "id-a", Mode: ModeOff})

	b := NewManager(helloServe("id-b", "Attic", "9.9.9"))
	mux := http.NewServeMux()
	mux.HandleFunc("POST "+directPath, b.ServeDirect)
	srv := httptest.NewTLSServer(mux)
	t.Cleanup(srv.Close)
	t.Cleanup(b.Close)
	b.Apply(Config{Secret: testSecret, InstanceID: "id-b", Mode: ModeOff})

	// The unreachable addresses stand in for the rest of a /24 a real sweep
	// would probe; sweepWith takes the candidate list as a parameter exactly
	// so a test can substitute a short one with a real member mixed in,
	// rather than dialling an actual subnet.
	candidates := []string{"https://127.0.0.1:1", "https://127.0.0.1:2", srv.URL, "https://127.0.0.1:4"}
	a.sweepWith(context.Background(), candidates)

	got := a.Members()
	if len(got) != 1 || got[0].ID != "id-b" || !got[0].Direct {
		t.Fatalf("members after a sweep of mixed candidates = %+v, want only b, found directly", got)
	}
}

// The sweep must never run more than sweepConcurrency probes at once, so a
// misconfigured or hostile /24 cannot turn it into a flood.
func TestSweepConcurrencyIsBounded(t *testing.T) {
	var cur, peak atomic.Int32
	release := make(chan struct{})
	mux := http.NewServeMux()
	mux.HandleFunc("POST "+directPath, func(w http.ResponseWriter, _ *http.Request) {
		n := cur.Add(1)
		for {
			p := peak.Load()
			if n <= p || peak.CompareAndSwap(p, n) {
				break
			}
		}
		<-release
		cur.Add(-1)
		w.WriteHeader(http.StatusForbidden)
	})
	srv := httptest.NewTLSServer(mux)
	t.Cleanup(srv.Close)

	a := NewManager(echo)
	t.Cleanup(a.Close)
	a.Apply(Config{Secret: testSecret, InstanceID: "id-a", Mode: ModeOff})

	candidates := make([]string, sweepConcurrency*3)
	for i := range candidates {
		candidates[i] = srv.URL
	}
	done := make(chan struct{})
	go func() {
		a.sweepWith(context.Background(), candidates)
		close(done)
	}()
	deadline := time.Now().Add(5 * time.Second)
	for cur.Load() < sweepConcurrency && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if cur.Load() != sweepConcurrency {
		t.Fatalf("only %d probes were in flight, want exactly %d before any is released", cur.Load(), sweepConcurrency)
	}
	close(release)
	<-done
	if got := peak.Load(); got > sweepConcurrency {
		t.Fatalf("peak concurrent probes = %d, want at most %d", got, sweepConcurrency)
	}
}

func TestNeedsSweepOutsideAGroupIsFalse(t *testing.T) {
	m := NewManager(echo)
	t.Cleanup(m.Close)
	if m.NeedsSweep() {
		t.Fatal("an instance in no group must never need a sweep")
	}
}

func TestNeedsSweepIsTrueUntilSomethingIsFoundDirectly(t *testing.T) {
	m := NewManager(echo)
	t.Cleanup(m.Close)
	m.Apply(Config{Secret: testSecret, InstanceID: "id-a", Mode: ModeOff})
	if !m.NeedsSweep() {
		t.Fatal("a group with nobody found by any route yet must need a sweep")
	}
	m.ConfirmAddress("id-b", "https://192.168.1.9:3443", "Attic", "9.9.9")
	if m.NeedsSweep() {
		t.Fatal("a confirmed direct address should stop the sweep from being needed")
	}
}

// A relay sibling this instance has no direct address for still needs a
// sweep, even though another member is already reachable directly: the
// point is to upgrade every member to a direct route, not just the first.
func TestNeedsSweepIsTrueForARelaySiblingWithNoDirectAddress(t *testing.T) {
	rs := httptest.NewServer(relay.NewServer())
	t.Cleanup(rs.Close)
	a := NewManager(echo)
	b := NewManager(echo)
	c := NewManager(echo)
	t.Cleanup(a.Close)
	t.Cleanup(b.Close)
	t.Cleanup(c.Close)
	a.Apply(Config{Secret: testSecret, InstanceID: "id-a", Mode: ModeOwn, RelayURL: rs.URL})
	b.Apply(Config{Secret: testSecret, InstanceID: "id-b", Name: "Attic", Mode: ModeOwn, RelayURL: rs.URL})
	c.Apply(Config{Secret: testSecret, InstanceID: "id-c", Name: "Garage", Mode: ModeOwn, RelayURL: rs.URL})

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) && len(a.Members()) < 2 {
		time.Sleep(20 * time.Millisecond)
	}
	if len(a.Members()) != 2 {
		t.Fatalf("a never saw both siblings over the relay: %+v", a.Members())
	}
	// b is confirmed directly, c is relay-only: a sweep is still needed for c.
	a.ConfirmAddress("id-b", "https://192.168.1.9:3443", "Attic", "9.9.9")
	if !a.NeedsSweep() {
		t.Fatal("a relay sibling with no known direct address should still need a sweep")
	}
	a.ConfirmAddress("id-c", "https://192.168.1.10:3443", "Garage", "9.9.9")
	if a.NeedsSweep() {
		t.Fatal("once every member has a confirmed direct address, no sweep should be needed")
	}
}

func TestMaybeSweepIsThrottled(t *testing.T) {
	m := NewManager(echo)
	t.Cleanup(m.Close)
	m.Apply(Config{Secret: testSecret, InstanceID: "id-a", DirectURL: "https://198.51.100.5:3443", Mode: ModeOff})

	m.sweepMu.Lock()
	m.lastSweep = time.Now()
	m.sweepMu.Unlock()

	// A sweep is due (nobody found yet) but the last one was just now, so
	// MaybeSweep must not touch the timestamp again this soon.
	m.MaybeSweep()
	m.sweepMu.Lock()
	last := m.lastSweep
	m.sweepMu.Unlock()
	if time.Since(last) > time.Second {
		t.Fatal("MaybeSweep started a new sweep before sweepMinInterval had passed")
	}
}

// Joining or creating a group must be able to sweep at once: with no prior
// sweep on record, MaybeSweep does not wait out sweepMinInterval.
func TestMaybeSweepRunsImmediatelyWithNoPriorSweep(t *testing.T) {
	m := NewManager(echo)
	t.Cleanup(m.Close)
	m.Apply(Config{Secret: testSecret, InstanceID: "id-a", DirectURL: "https://198.51.100.5:3443", Mode: ModeOff})

	m.MaybeSweep()
	m.sweepMu.Lock()
	started := !m.lastSweep.IsZero()
	m.sweepMu.Unlock()
	if !started {
		t.Fatal("MaybeSweep did not start a sweep for a group that just joined")
	}
}

// Leaving and joining a different group must not let a sweep due under the
// old one skip the new one's first sweep, and must forget the old
// addresses: neither means anything once the secret has changed.
func TestApplyResetsAddressesAndTheSweepClockOnANewGroup(t *testing.T) {
	m := NewManager(echo)
	t.Cleanup(m.Close)
	m.Apply(Config{Secret: testSecret, InstanceID: "id-a", Mode: ModeOff})
	m.ConfirmAddress("id-b", "https://192.168.1.9:3443", "Attic", "9.9.9")
	m.sweepMu.Lock()
	m.lastSweep = time.Now()
	m.sweepMu.Unlock()

	m.Apply(Config{Secret: []byte("fedcba9876543210"), InstanceID: "id-a", Mode: ModeOff})
	if got := m.Members(); len(got) != 0 {
		t.Fatalf("members after joining a different group = %+v, want the old ones forgotten", got)
	}
	m.sweepMu.Lock()
	zero := m.lastSweep.IsZero()
	m.sweepMu.Unlock()
	if !zero {
		t.Fatal("the sweep clock was not reset on a new group")
	}
}

func TestCallTriesAStoredAddressBeforeFallingBackToTheRelay(t *testing.T) {
	rs := httptest.NewServer(relay.NewServer())
	t.Cleanup(rs.Close)

	a := NewManager(echo)
	t.Cleanup(a.Close)
	a.Apply(Config{Secret: testSecret, InstanceID: "id-a", Mode: ModeOwn, RelayURL: rs.URL})

	b := NewManager(echo)
	mux := http.NewServeMux()
	mux.HandleFunc("POST "+directPath, b.ServeDirect)
	direct := httptest.NewTLSServer(mux)
	t.Cleanup(direct.Close)
	t.Cleanup(b.Close)
	// b never joins the relay a uses, so a's only possible route to it is
	// the address noted below.
	b.Apply(Config{Secret: testSecret, InstanceID: "id-b", Mode: ModeOff})

	a.NoteAddress("id-b", direct.URL)
	status, body, err := a.Call(context.Background(), "id-b", http.MethodGet, "/api/group/peer/status", nil)
	if err != nil || status != http.StatusOK || string(body) != "GET /api/group/peer/status " {
		t.Fatalf("Call via a noted address = %d %q %v", status, body, err)
	}
}

func TestCallFallsBackToTheRelayWhenTheStoredAddressDoesNotAnswer(t *testing.T) {
	rs := httptest.NewServer(relay.NewServer())
	t.Cleanup(rs.Close)
	a := NewManager(echo)
	b := NewManager(echo)
	t.Cleanup(a.Close)
	t.Cleanup(b.Close)
	a.Apply(Config{Secret: testSecret, InstanceID: "id-a", Mode: ModeOwn, RelayURL: rs.URL})
	b.Apply(Config{Secret: testSecret, InstanceID: "id-b", Name: "Attic", Mode: ModeOwn, RelayURL: rs.URL})

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) && len(a.Members()) == 0 {
		time.Sleep(20 * time.Millisecond)
	}
	a.NoteAddress("id-b", "https://127.0.0.1:1")
	status, _, err := a.Call(context.Background(), "id-b", http.MethodGet, "/api/group/peer/status", nil)
	if err != nil || status != http.StatusOK {
		t.Fatalf("Call did not fall back to the relay once the stored address failed: %d %v", status, err)
	}
}

// A flood of bad signatures from one source must not keep being checked
// forever: it earns that address a block, which holds even against a
// correctly signed call, and leaves an unrelated address alone.
func TestServeDirectBlocksAnAddressAfterRepeatedBadSignatures(t *testing.T) {
	b := NewManager(echo)
	t.Cleanup(b.Close)
	b.Apply(Config{Secret: testSecret, InstanceID: "id-b", Mode: ModeOff})
	bad := PeerAuthKey([]byte("fedcba9876543210"))
	good := PeerAuthKey(testSecret)

	for i := 0; i < 15; i++ {
		r := directRequest("", bad, "id-a", time.Now(), sealedFor(t, "id-b"))
		r.RemoteAddr = "203.0.113.9:1"
		b.ServeDirect(httptest.NewRecorder(), r)
	}

	r := directRequest("", good, "id-a", time.Now(), sealedFor(t, "id-b"))
	r.RemoteAddr = "203.0.113.9:2"
	w := httptest.NewRecorder()
	b.ServeDirect(w, r)
	if w.Code != http.StatusForbidden {
		t.Fatalf("a correctly signed call from a blocked address got HTTP %d, want 403", w.Code)
	}

	r2 := directRequest("", good, "id-a", time.Now(), sealedFor(t, "id-b"))
	r2.RemoteAddr = "198.51.100.5:1"
	w2 := httptest.NewRecorder()
	b.ServeDirect(w2, r2)
	if w2.Code != http.StatusOK {
		t.Fatalf("a correctly signed call from an unrelated address got HTTP %d, want 200", w2.Code)
	}
}
