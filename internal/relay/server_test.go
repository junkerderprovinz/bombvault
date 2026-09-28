package relay

import (
	"context"
	"encoding/json"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coder/websocket"
)

// fakeConn is a connection whose Write can be held open on demand, to model
// an instance on a bad link without a real socket.
type fakeConn struct {
	writes chan []byte
	block  chan struct{} // if non-nil, Write waits for it to be closed
	closed atomic.Bool
}

func newFakeConn() *fakeConn {
	return &fakeConn{writes: make(chan []byte, 4*queueDepth)}
}

func (f *fakeConn) Write(ctx context.Context, _ websocket.MessageType, p []byte) error {
	if f.block != nil {
		select {
		case <-f.block:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	f.writes <- append([]byte(nil), p...)
	return nil
}

func (f *fakeConn) CloseNow() error {
	f.closed.Store(true)
	return nil
}

// next takes one frame off a fake connection or fails, so a missing
// broadcast shows up as a named failure instead of a hung test.
func next(t *testing.T, f *fakeConn, msg string) Envelope {
	t.Helper()
	select {
	case raw := <-f.writes:
		env, err := Decode(raw)
		if err != nil {
			t.Fatalf("%s: frame is not an envelope (%v): %s", msg, err, raw)
		}
		return env
	case <-time.After(5 * time.Second):
		t.Fatal(msg)
		return Envelope{}
	}
}

// nothing asserts that a connection stays quiet.
func nothing(t *testing.T, f *fakeConn, msg string) {
	t.Helper()
	select {
	case extra := <-f.writes:
		t.Fatalf("%s: %s", msg, extra)
	case <-time.After(100 * time.Millisecond):
	}
}

func announceOf(t *testing.T, env Envelope, msg string) Announce {
	t.Helper()
	if env.Type != TypeAnnounce {
		t.Fatalf("%s: got a %q frame, want an announce", msg, env.Type)
	}
	var a Announce
	if err := env.Into(&a); err != nil {
		t.Fatalf("%s: %v", msg, err)
	}
	return a
}

func join(t *testing.T, s *Server, key, id string) *fakeConn {
	t.Helper()
	c := newFakeConn()
	s.Join(key, c, Announce{InstanceID: id})
	t.Cleanup(func() { s.Leave(c) })
	return c
}

// send routes one frame as if the connection had written it up its socket.
func send(t *testing.T, s *Server, c Conn, typ string, data any) {
	t.Helper()
	frame, err := Encode(typ, data)
	if err != nil {
		t.Fatalf("encode %s: %v", typ, err)
	}
	s.Route(c, frame)
}

// TestJoinIntroducesBothWays: a late joiner learns about the instances that
// were already up, not only the ones that connect after it.
func TestJoinIntroducesBothWays(t *testing.T) {
	s := NewServer()
	a := join(t, s, "key-1", "alpha")
	b := join(t, s, "key-1", "bravo")

	if got := announceOf(t, next(t, b, "the joiner never heard about its sibling"), "sibling announce"); got.InstanceID != "alpha" {
		t.Errorf("joiner was told about %q, want alpha", got.InstanceID)
	}
	if got := announceOf(t, next(t, a, "the sibling never heard about the joiner"), "arrival announce"); got.InstanceID != "bravo" {
		t.Errorf("sibling was told about %q, want bravo", got.InstanceID)
	}
	nothing(t, b, "the joiner received its own arrival announce back")
}

// TestKeysNeverSeeEachOther pins the relay's only isolation boundary.
func TestKeysNeverSeeEachOther(t *testing.T) {
	s := NewServer()
	mine := join(t, s, "key-mine", "alpha")
	theirs := join(t, s, "key-theirs", "bravo")

	nothing(t, mine, "a connection on another key was announced")
	nothing(t, theirs, "a connection on another key was announced")

	send(t, s, mine, TypeProxyRequest, ProxyRequest{RequestID: "r1", Target: "bravo"})
	nothing(t, theirs, "a proxy-request crossed a key boundary")

	env := next(t, mine, "no answer for a target on another key")
	var resp ProxyResponse
	if env.Type != TypeProxyResponse || env.Into(&resp) != nil {
		t.Fatalf("got a %q frame, want a proxy-response", env.Type)
	}
	if resp.Error == "" {
		t.Error("a target on another key was answered without an error")
	}
}

// TestLeaveTellsSiblingsTheInstanceWentOffline: the Instances page takes live
// status from these frames, so a silent disconnect would leave a dead
// instance looking online.
func TestLeaveTellsSiblingsTheInstanceWentOffline(t *testing.T) {
	s := NewServer()
	a := join(t, s, "key-1", "alpha")
	b := join(t, s, "key-1", "bravo")
	next(t, a, "no arrival announce")
	next(t, b, "no sibling announce")

	s.Leave(b)

	env := next(t, a, "no presence frame after a sibling disconnected")
	var p Presence
	if env.Type != TypePresence || env.Into(&p) != nil {
		t.Fatalf("got a %q frame, want a presence", env.Type)
	}
	if p.InstanceID != "bravo" || p.Online {
		t.Errorf("got %+v, want bravo offline", p)
	}
	if s.Len() != 1 {
		t.Errorf("%d connections still registered, want 1", s.Len())
	}
}

// TestProxyRoundTripRoutesByRequestID covers forwarding in both directions
// and that the relay carries the sealed bytes unaltered. They are not real
// ciphertext; opening is the client's job and client_test.go covers it.
func TestProxyRoundTripRoutesByRequestID(t *testing.T) {
	s := NewServer()
	a := join(t, s, "key-1", "alpha")
	b := join(t, s, "key-1", "bravo")
	next(t, a, "no arrival announce")
	next(t, b, "no sibling announce")

	sealed := []byte("sealed-call-bytes-the-relay-cannot-read")
	send(t, s, a, TypeProxyRequest, ProxyRequest{
		RequestID: "r1", Target: "bravo", Sealed: sealed,
	})

	env := next(t, b, "the target never received the proxy-request")
	var req ProxyRequest
	if env.Type != TypeProxyRequest || env.Into(&req) != nil {
		t.Fatalf("got a %q frame, want a proxy-request", env.Type)
	}
	if req.RequestID != "r1" || req.Target != "bravo" || string(req.Sealed) != string(sealed) {
		t.Fatalf("target received %+v, want the request unchanged", req)
	}

	answer := []byte("sealed-result-bytes")
	send(t, s, b, TypeProxyResponse, ProxyResponse{RequestID: "r1", Sealed: answer})

	env = next(t, a, "the caller never received the proxy-response")
	var resp ProxyResponse
	if env.Type != TypeProxyResponse || env.Into(&resp) != nil {
		t.Fatalf("got a %q frame, want a proxy-response", env.Type)
	}
	if string(resp.Sealed) != string(answer) || resp.Error != "" {
		t.Errorf("caller received %+v, want the target's own answer", resp)
	}
	nothing(t, b, "the response was echoed back to the responder")
}

func TestUnroutableRequestAnswersImmediately(t *testing.T) {
	cases := []struct {
		name   string
		target string
	}{
		{"nobody is connected under that id", "charlie"},
		{"the caller addressed itself", "alpha"},
		{"the id is on another key", "bravo"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := NewServer()
			a := join(t, s, "key-1", "alpha")
			join(t, s, "key-2", "bravo")

			send(t, s, a, TypeProxyRequest, ProxyRequest{RequestID: "r1", Target: tc.target})

			env := next(t, a, "no answer for an unroutable request")
			var resp ProxyResponse
			if env.Type != TypeProxyResponse || env.Into(&resp) != nil {
				t.Fatalf("got a %q frame, want a proxy-response", env.Type)
			}
			// The relay holds no frame key, so a refusal carries only Error.
			if resp.RequestID != "r1" || resp.Error == "" || len(resp.Sealed) != 0 {
				t.Errorf("got %+v, want an unsealed refusal carrying an error", resp)
			}
		})
	}
}

// TestFullQueueDropsOnlyThatConnection: an instance that cannot drain is
// disconnected, and everyone else on the key keeps being routed.
func TestFullQueueDropsOnlyThatConnection(t *testing.T) {
	s := NewServer()
	stuck := newFakeConn()
	stuck.block = make(chan struct{})
	s.Join("key-1", stuck, Announce{InstanceID: "stuck"})
	healthy := join(t, s, "key-1", "healthy")
	next(t, healthy, "no sibling announce")

	// One frame sits in Write and queueDepth more fit in the queue. Since
	// each sender may only have maxPendingPerSender requests in flight, a
	// handful of senders together push past that.
	need := queueDepth + 5
	for i := 0; need > 0; i++ {
		sender := join(t, s, "key-1", fmt.Sprintf("sender-%d", i))
		batch := maxPendingPerSender
		if batch > need {
			batch = need
		}
		for j := 0; j < batch; j++ {
			send(t, s, sender, TypeProxyRequest, ProxyRequest{
				RequestID: fmt.Sprintf("r-%d-%d", i, j), Target: "stuck",
			})
		}
		need -= batch
	}

	if s.Len() == 0 {
		t.Fatal("dropping the stuck connection took the whole key down")
	}
	found := false
	s.mu.Lock()
	for _, cl := range s.keys["key-1"] {
		if cl.announce.InstanceID == "stuck" {
			found = true
		}
	}
	s.mu.Unlock()
	if found {
		t.Error("the over-full connection is still registered")
	}

	close(stuck.block)
	deadline := time.Now().Add(5 * time.Second)
	for !stuck.closed.Load() && time.Now().Before(deadline) {
		time.Sleep(2 * time.Millisecond)
	}
	if !stuck.closed.Load() {
		t.Fatal("the dropped connection was never closed")
	}

	late := join(t, s, "key-1", "late")
	if got := announceOf(t, next(t, late, "the relay stopped routing after a drop"), "sibling announce"); got.InstanceID == "" {
		t.Error("the late joiner was introduced to nobody")
	}
}

// TestReconnectReplacesTheDeadSocket: an instance whose socket died without
// a close frame reconnects while the relay still holds the dead socket, and
// requests must go to the new one.
func TestReconnectReplacesTheDeadSocket(t *testing.T) {
	s := NewServer()
	sib := join(t, s, "key-1", "watcher")
	first := join(t, s, "key-1", "alpha")
	next(t, sib, "no arrival announce")
	next(t, first, "no sibling announce")

	second := join(t, s, "key-1", "alpha")
	next(t, sib, "the reconnect was never announced")
	// No offline presence for a replaced socket: the instance is still here.
	env := next(t, second, "the reconnected socket was introduced to nobody")
	if got := announceOf(t, env, "sibling announce"); got.InstanceID != "watcher" {
		t.Errorf("reconnected socket was told about %q, want watcher", got.InstanceID)
	}
	if s.Len() != 2 {
		t.Errorf("%d connections registered, want the dead socket replaced rather than joined beside", s.Len())
	}

	send(t, s, sib, TypeProxyRequest, ProxyRequest{RequestID: "r1", Target: "alpha"})
	got := next(t, second, "the request was not routed to the live socket")
	if got.Type != TypeProxyRequest {
		t.Fatalf("live socket got a %q frame, want the proxy-request", got.Type)
	}
	nothing(t, first, "the request was routed to the replaced socket")
}

// TestGarbageFramesAreIgnored: a newer client may send frame types this
// build does not know, and that must not take its connection down.
func TestGarbageFramesAreIgnored(t *testing.T) {
	s := NewServer()
	a := join(t, s, "key-1", "alpha")
	b := join(t, s, "key-1", "bravo")
	next(t, a, "no arrival announce")
	next(t, b, "no sibling announce")

	for _, frame := range [][]byte{
		[]byte("not json at all"),
		[]byte(`{"type":"something-new","data":{"x":1}}`),
		frameOf(TypeProxyRequest, ProxyRequest{Target: "bravo"}),            // no request id
		frameOf(TypeProxyRequest, ProxyRequest{RequestID: "r1"}),            // no target
		frameOf(TypeProxyResponse, ProxyResponse{Sealed: []byte("x")}),      // no request id
		frameOf(TypeProxyResponse, ProxyResponse{RequestID: "unasked-for"}), // nobody is waiting
	} {
		s.Route(a, frame)
	}

	if s.Len() != 2 {
		t.Fatalf("%d connections registered, want both still up after garbage", s.Len())
	}
	nothing(t, a, "a garbage frame produced an answer")
	nothing(t, b, "a garbage frame was forwarded to a sibling")

	send(t, s, a, TypeProxyRequest, ProxyRequest{RequestID: "r2", Target: "bravo"})
	if got := next(t, b, "the connection stopped working after a garbage frame"); got.Type != TypeProxyRequest {
		t.Errorf("got a %q frame, want the proxy-request", got.Type)
	}
}

// TestResponseFromWrongConnectionIsIgnored: only the connection a request was
// routed to can answer it, not a sibling that was never asked or a
// connection on another key that learned the request ID.
func TestResponseFromWrongConnectionIsIgnored(t *testing.T) {
	s := NewServer()
	a := join(t, s, "key-1", "alpha")
	b := join(t, s, "key-1", "bravo")
	impostor := join(t, s, "key-1", "impostor")
	outsider := join(t, s, "key-2", "outsider")
	next(t, a, "no bravo arrival announce")
	next(t, a, "no impostor arrival announce")
	next(t, b, "no alpha sibling announce")
	next(t, b, "no impostor arrival announce")
	next(t, impostor, "no alpha sibling announce")
	next(t, impostor, "no bravo sibling announce")

	send(t, s, a, TypeProxyRequest, ProxyRequest{RequestID: "r1", Target: "bravo"})
	next(t, b, "the real target never received the request")

	send(t, s, impostor, TypeProxyResponse, ProxyResponse{RequestID: "r1", Sealed: []byte("forged")})
	send(t, s, outsider, TypeProxyResponse, ProxyResponse{RequestID: "r1", Sealed: []byte("forged")})
	nothing(t, a, "a forged proxy-response was delivered to the requester")

	send(t, s, b, TypeProxyResponse, ProxyResponse{RequestID: "r1", Sealed: []byte("real")})
	env := next(t, a, "the real answer was never delivered")
	var resp ProxyResponse
	if env.Type != TypeProxyResponse || env.Into(&resp) != nil {
		t.Fatalf("got a %q frame, want a proxy-response", env.Type)
	}
	if string(resp.Sealed) != "real" {
		t.Errorf("requester received %q, want the real target's own answer", resp.Sealed)
	}
}

// TestTooManyInFlightRequestsAreRefused: the cap refuses the sender before an
// excess frame reaches the target's queue, where overflow would evict the
// target instead of the flooder.
func TestTooManyInFlightRequestsAreRefused(t *testing.T) {
	s := NewServer()
	a := join(t, s, "key-1", "alpha")
	b := join(t, s, "key-1", "bravo")
	next(t, a, "no arrival announce")
	next(t, b, "no sibling announce")

	for i := 0; i < maxPendingPerSender; i++ {
		send(t, s, a, TypeProxyRequest, ProxyRequest{
			RequestID: fmt.Sprintf("r%d", i), Target: "bravo",
		})
		next(t, b, "a within-budget request was refused")
	}

	send(t, s, a, TypeProxyRequest, ProxyRequest{RequestID: "over-budget", Target: "bravo"})
	nothing(t, b, "an over-budget request reached the target anyway")
	env := next(t, a, "the sender was never told it was refused")
	var resp ProxyResponse
	if env.Type != TypeProxyResponse || env.Into(&resp) != nil {
		t.Fatalf("got a %q frame, want a proxy-response", env.Type)
	}
	if resp.RequestID != "over-budget" || resp.Error == "" || len(resp.Sealed) != 0 {
		t.Errorf("got %+v, want a 429 carrying an error", resp)
	}
}

func TestKeyGroupSizeIsCapped(t *testing.T) {
	s := NewServer()
	for i := 0; i < maxClientsPerKey; i++ {
		if !s.Join("key-1", newFakeConn(), Announce{InstanceID: fmt.Sprintf("i%d", i)}) {
			t.Fatalf("join %d was refused before the cap was reached", i)
		}
	}
	if s.Join("key-1", newFakeConn(), Announce{InstanceID: "one-too-many"}) {
		t.Error("a connection past maxClientsPerKey was admitted")
	}
	if s.Len() != maxClientsPerKey {
		t.Errorf("%d connections registered, want exactly maxClientsPerKey", s.Len())
	}
}

// TestTargetDisconnectFailsThePendingRequestFast: if the target leaves
// mid-request, the requester learns it at once instead of waiting out
// pendingTTL.
func TestTargetDisconnectFailsThePendingRequestFast(t *testing.T) {
	s := NewServer()
	a := join(t, s, "key-1", "alpha")
	b := join(t, s, "key-1", "bravo")
	next(t, a, "no arrival announce")
	next(t, b, "no sibling announce")

	send(t, s, a, TypeProxyRequest, ProxyRequest{RequestID: "r1", Target: "bravo"})
	next(t, b, "the target never received the request")

	s.Leave(b)

	env := next(t, a, "the pending request was never failed fast")
	var resp ProxyResponse
	if env.Type != TypeProxyResponse || env.Into(&resp) != nil {
		t.Fatalf("got a %q frame first, want the failed proxy-response ahead of the presence frame", env.Type)
	}
	if resp.RequestID != "r1" || resp.Error == "" || len(resp.Sealed) != 0 {
		t.Errorf("got %+v, want an unsealed refusal carrying an error", resp)
	}

	presenceEnv := next(t, a, "no presence frame after the disconnect")
	var p Presence
	if presenceEnv.Type != TypePresence || presenceEnv.Into(&p) != nil || p.InstanceID != "bravo" || p.Online {
		t.Errorf("got a %+v presence frame, want bravo offline", presenceEnv)
	}
}

// TestReconnectFailsThePendingRequestFast: a request routed to a socket that
// Join then replaced can never be answered, so the requester is told at once.
func TestReconnectFailsThePendingRequestFast(t *testing.T) {
	s := NewServer()
	a := join(t, s, "key-1", "alpha")
	join(t, s, "key-1", "bravo")
	next(t, a, "no arrival announce")

	send(t, s, a, TypeProxyRequest, ProxyRequest{RequestID: "r1", Target: "bravo"})

	join(t, s, "key-1", "bravo") // reconnect, replaces the connection r1 was routed to

	env := next(t, a, "the pending request was never failed fast on reconnect")
	var resp ProxyResponse
	if env.Type != TypeProxyResponse || env.Into(&resp) != nil {
		t.Fatalf("got a %q frame, want the failed proxy-response", env.Type)
	}
	if resp.RequestID != "r1" || resp.Error == "" || len(resp.Sealed) != 0 {
		t.Errorf("got %+v, want an unsealed refusal carrying an error", resp)
	}
}

// TestEnvelopeShapeIsStable pins the wire bytes any other implementation
// speaking this protocol is written against. A renamed field compiles fine
// and still round-trips through this package's own types.
func TestEnvelopeShapeIsStable(t *testing.T) {
	cases := []struct {
		name string
		typ  string
		data any
		want string
	}{
		{
			"hello",
			TypeHello,
			Hello{Key: "s3cret", Announce: Announce{InstanceID: "alpha", Sealed: []byte("hi")}},
			`{"type":"hello","data":{"key":"s3cret","announce":{"instanceId":"alpha","sealed":"aGk="}}}`,
		},
		{
			"announce",
			TypeAnnounce,
			Announce{InstanceID: "alpha", Sealed: []byte("hi")},
			`{"type":"announce","data":{"instanceId":"alpha","sealed":"aGk="}}`,
		},
		{
			"presence",
			TypePresence,
			Presence{InstanceID: "alpha"},
			`{"type":"presence","data":{"instanceId":"alpha","online":false}}`,
		},
		{
			"proxy-request",
			TypeProxyRequest,
			ProxyRequest{RequestID: "r1", Target: "bravo", Sealed: []byte("hi")},
			`{"type":"proxy-request","data":{"requestId":"r1","target":"bravo","sealed":"aGk="}}`,
		},
		{
			"proxy-response",
			TypeProxyResponse,
			ProxyResponse{RequestID: "r1", Sealed: []byte("hi")},
			`{"type":"proxy-response","data":{"requestId":"r1","sealed":"aGk="}}`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Encode(tc.typ, tc.data)
			if err != nil {
				t.Fatalf("encode: %v", err)
			}
			if string(got) != tc.want {
				t.Errorf("wire shape changed:\n got %s\nwant %s", got, tc.want)
			}
			env, err := Decode(got)
			if err != nil || env.Type != tc.typ {
				t.Fatalf("decode: %v, type %q", err, env.Type)
			}
			if !json.Valid(env.Data) {
				t.Error("payload is not valid JSON on its own")
			}
		})
	}
}

func TestRevokeClosesTheConnectionsOnAKeyNoLongerAdmitted(t *testing.T) {
	s := NewServer()
	var served atomic.Value
	served.Store("old-key-0123456789abcdef0123456789")
	s.Admit = func(key string) bool { return key == served.Load() }
	old := join(t, s, "old-key-0123456789abcdef0123456789", "alpha")
	s.Revoke()
	if old.closed.Load() || s.Len() != 1 {
		t.Fatal("Revoke closed a connection whose key is still admitted")
	}

	served.Store("new-key-0123456789abcdef0123456789")
	s.Revoke()
	if !old.closed.Load() || s.Len() != 0 {
		t.Fatalf("after the key changed the old connection is open=%v, registered=%d", !old.closed.Load(), s.Len())
	}
	if s.Join("old-key-0123456789abcdef0123456789", newFakeConn(), Announce{InstanceID: "bravo"}) {
		t.Fatal("a connection that passed the handshake on the old key joined after the change")
	}
}
