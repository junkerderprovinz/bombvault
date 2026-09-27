package relay

import (
	"context"
	"net"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"
)

// testBackoff is the reconnect cadence the tests run at, instead of the real
// one-second minimum.
const testBackoff = 20 * time.Millisecond

// testFrameKey is the frame key shared by every client and hand-rolled peer in
// these tests, as if they had entered the same connection phrase. It goes
// through the real DeriveFrameKey so the tests follow any change to it.
var testFrameKey = DeriveFrameKey([]byte("relay package tests"))

// sealFor, sealResultFor, openFrom and openResultFrom seal and open payloads
// for the hand-rolled peers, keeping the additional data in one place.
func sealFor(t *testing.T, requestID, target string, call ProxyCall) []byte {
	t.Helper()
	sealed, err := SealCall(testFrameKey, requestID, target, call)
	if err != nil {
		t.Fatalf("seal call: %v", err)
	}
	return sealed
}

func sealResultFor(t *testing.T, requestID string, res ProxyResult) []byte {
	t.Helper()
	sealed, err := SealResult(testFrameKey, requestID, res)
	if err != nil {
		t.Fatalf("seal result: %v", err)
	}
	return sealed
}

func openFrom(t *testing.T, requestID, target string, sealed []byte) ProxyCall {
	t.Helper()
	call, err := OpenCall(testFrameKey, requestID, target, sealed)
	if err != nil {
		t.Fatalf("open call: %v", err)
	}
	return call
}

func openResultFrom(t *testing.T, requestID string, sealed []byte) ProxyResult {
	t.Helper()
	res, err := OpenResult(testFrameKey, requestID, sealed)
	if err != nil {
		t.Fatalf("open result: %v", err)
	}
	return res
}

// tracking remembers every connection it accepted so they can all be killed
// at once. http.Server.Close cannot do it, because an upgraded WebSocket is
// hijacked out of the server's bookkeeping.
type tracking struct {
	net.Listener
	mu    sync.Mutex
	conns []net.Conn
}

func (l *tracking) Accept() (net.Conn, error) {
	c, err := l.Listener.Accept()
	if err == nil {
		l.mu.Lock()
		l.conns = append(l.conns, c)
		l.mu.Unlock()
	}
	return c, err
}

func (l *tracking) closeAll() {
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, c := range l.conns {
		_ = c.Close()
	}
	l.conns = nil
}

// relayOn serves a real relay on addr ("127.0.0.1:0" for a fresh port) and
// returns the address it landed on plus a stop that drops the listener and
// every live connection, as if the relay process died. Calling relayOn again
// on the same address then looks like a restarted relay, which an
// httptest.Server cannot do because it never gives its port back.
func relayOn(t *testing.T, addr string) (string, func()) {
	t.Helper()
	raw, err := net.Listen("tcp", addr)
	if err != nil {
		t.Fatalf("listen on %s: %v", addr, err)
	}
	l := &tracking{Listener: raw}
	srv := &http.Server{Handler: NewServer(), ReadHeaderTimeout: 5 * time.Second}
	go func() { _ = srv.Serve(l) }()
	var once sync.Once
	stop := func() {
		once.Do(func() {
			_ = srv.Close()
			l.closeAll()
		})
	}
	t.Cleanup(stop)
	return raw.Addr().String(), stop
}

// startClient builds a Client against addr with the fast test backoff and
// starts it. It passes the http:// form of the address so every test also
// exercises the scheme and path rewriting. A nil serve stands in for a test
// that never expects a sibling to call it.
func startClient(t *testing.T, addr, key, id string, serve Handler) *Client {
	t.Helper()
	if serve == nil {
		serve = func(context.Context, ProxyCall) (int, []byte) { return http.StatusNotImplemented, nil }
	}
	c, err := NewClient(ClientOptions{
		URL:        "http://" + addr,
		Key:        key,
		FrameKey:   testFrameKey,
		InstanceID: id,
		Identity:   Identity{Name: id},
		Serve:      serve,
	})
	if err != nil {
		t.Fatalf("new client %s: %v", id, err)
	}
	c.minBackoff, c.maxBackoff = testBackoff, 4*testBackoff
	c.Start()
	t.Cleanup(func() { _ = c.Close() })
	return c
}

// waitFor polls until cond holds, failing the test if it never does. The
// frames being waited on cross a real socket into a background goroutine the
// test has no handle on.
func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(wsTimeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

func TestClientAnnouncesItselfAndTracksSiblings(t *testing.T) {
	addr, _ := relayOn(t, "127.0.0.1:0")
	alpha := startClient(t, addr, "shared-relay-test-key-0123456789ab", "alpha", nil)
	waitFor(t, "alpha to connect", alpha.Connected)

	bravo := dialInstance(t, "ws://"+addr+connectPath, "shared-relay-test-key-0123456789ab", "bravo")

	// bravo is a raw socket, so it reads the wire form a relay operator sees:
	// the id in the clear, the identity only sealed.
	var seen Announce
	if err := readFrame(t, bravo, TypeAnnounce).Into(&seen); err != nil {
		t.Fatalf("announce: %v", err)
	}
	if seen.InstanceID != "alpha" {
		t.Errorf("bravo was introduced to %+v, want alpha's id in the clear", seen)
	}
	id, err := OpenIdentity(testFrameKey, seen.InstanceID, seen.Sealed)
	if err != nil {
		t.Fatalf("the sealed identity did not open: %v", err)
	}
	if id.Name != "alpha" {
		t.Errorf("sealed identity = %+v, want alpha's name", id)
	}

	waitFor(t, "alpha to see bravo", func() bool {
		sibs := alpha.Siblings()
		return len(sibs) == 1 && sibs[0].InstanceID == "bravo"
	})

	_ = bravo.CloseNow()
	waitFor(t, "alpha to see bravo go", func() bool { return len(alpha.Siblings()) == 0 })
}

func TestClientCallsASibling(t *testing.T) {
	addr, _ := relayOn(t, "127.0.0.1:0")
	alpha := startClient(t, addr, "shared-relay-test-key-0123456789ab", "alpha", nil)
	bravo := dialInstance(t, "ws://"+addr+connectPath, "shared-relay-test-key-0123456789ab", "bravo")
	readFrame(t, bravo, TypeAnnounce)
	waitFor(t, "alpha to see bravo", func() bool { return len(alpha.Siblings()) == 1 })

	type result struct {
		res ProxyResult
		err error
	}
	done := make(chan result, 1)
	go func() {
		res, err := alpha.Call(context.Background(), "bravo", ProxyCall{
			Method: http.MethodPost, Path: "/api/links", Body: []byte(`{"url":"x"}`),
		})
		done <- result{res, err}
	}()

	var req ProxyRequest
	if err := readFrame(t, bravo, TypeProxyRequest).Into(&req); err != nil {
		t.Fatalf("proxy-request: %v", err)
	}
	if req.Target != "bravo" {
		t.Fatalf("bravo received %+v, want alpha's call unchanged", req)
	}
	call := openFrom(t, req.RequestID, "bravo", req.Sealed)
	if call.Method != http.MethodPost || call.Path != "/api/links" {
		t.Fatalf("bravo opened %+v, want alpha's call unchanged", call)
	}
	if string(call.Body) != `{"url":"x"}` {
		t.Errorf("body arrived as %s, want it byte for byte", call.Body)
	}
	writeFrame(t, bravo, TypeProxyResponse, ProxyResponse{
		RequestID: req.RequestID,
		Sealed:    sealResultFor(t, req.RequestID, ProxyResult{Status: 201, Body: []byte(`{"added":1}`)}),
	})

	got := <-done
	if got.err != nil {
		t.Fatalf("call: %v", got.err)
	}
	if got.res.Status != 201 || string(got.res.Body) != `{"added":1}` {
		t.Errorf("got %+v, want bravo's own answer", got.res)
	}
}

func TestClientAnswersASiblingsCall(t *testing.T) {
	addr, _ := relayOn(t, "127.0.0.1:0")
	alpha := startClient(t, addr, "shared-relay-test-key-0123456789ab", "alpha", func(_ context.Context, call ProxyCall) (int, []byte) {
		return http.StatusOK, []byte(call.Method + " " + call.Path)
	})
	waitFor(t, "alpha to connect", alpha.Connected)
	bravo := dialInstance(t, "ws://"+addr+connectPath, "shared-relay-test-key-0123456789ab", "bravo")
	readFrame(t, bravo, TypeAnnounce)

	writeFrame(t, bravo, TypeProxyRequest, ProxyRequest{
		RequestID: "r1", Target: "alpha",
		Sealed: sealFor(t, "r1", "alpha", ProxyCall{Method: http.MethodGet, Path: "/api/tasks"}),
	})
	var resp ProxyResponse
	if err := readFrame(t, bravo, TypeProxyResponse).Into(&resp); err != nil {
		t.Fatalf("proxy-response: %v", err)
	}
	if resp.RequestID != "r1" {
		t.Fatalf("got %+v, want the answer to r1", resp)
	}
	res := openResultFrom(t, "r1", resp.Sealed)
	if res.Status != http.StatusOK || string(res.Body) != "GET /api/tasks" {
		t.Errorf("got %+v, want status %d body %q", res, http.StatusOK, "GET /api/tasks")
	}
}

// TestCallFailsFastRatherThanWaiting: a call that cannot reach its peer fails
// at once, not at the end of CallTimeout.
func TestCallFailsFastRatherThanWaiting(t *testing.T) {
	tests := []struct {
		name    string
		connect bool
		target  string
		wantErr string
	}{
		{
			name:    "the relay is not connected",
			connect: false,
			target:  "bravo",
			wantErr: "not connected",
		},
		{
			// The relay answers this one, and the client has to pass its
			// text on.
			name:    "nobody is connected as the target",
			connect: true,
			target:  "nobody",
			wantErr: "no instance nobody is connected",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			addr, stop := relayOn(t, "127.0.0.1:0")
			// Stopped before the client exists, so it cannot briefly connect.
			if !tc.connect {
				stop()
			}
			alpha := startClient(t, addr, "shared-relay-test-key-0123456789ab", "alpha", nil)
			if tc.connect {
				waitFor(t, "alpha to connect", alpha.Connected)
			}

			began := time.Now()
			_, err := alpha.Call(context.Background(), tc.target, ProxyCall{Method: http.MethodGet, Path: "/api/tasks"})
			if err == nil {
				t.Fatal("the call succeeded, want it to fail")
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("got %v, want it to mention %q", err, tc.wantErr)
			}
			if time.Since(began) >= CallTimeout {
				t.Errorf("the call took %v, want it to fail without waiting out the timeout", time.Since(began))
			}
		})
	}
}

// TestReconnectsAfterTheRelayDrops: an outage costs the instance its relay
// peers, and they come back on their own once the relay does.
func TestReconnectsAfterTheRelayDrops(t *testing.T) {
	addr, stop := relayOn(t, "127.0.0.1:0")
	alpha := startClient(t, addr, "shared-relay-test-key-0123456789ab", "alpha", nil)
	bravo := dialInstance(t, "ws://"+addr+connectPath, "shared-relay-test-key-0123456789ab", "bravo")
	waitFor(t, "alpha to see bravo", func() bool { return len(alpha.Siblings()) == 1 })

	stop()
	waitFor(t, "alpha to notice the outage", func() bool {
		return !alpha.Connected() && len(alpha.Siblings()) == 0
	})
	_ = bravo.CloseNow()

	// A fresh relay on the same address shares no state with the old one, so
	// everything the client sees now it re-established itself.
	relayOn(t, addr)
	waitFor(t, "alpha to reconnect", alpha.Connected)

	charlie := dialInstance(t, "ws://"+addr+connectPath, "shared-relay-test-key-0123456789ab", "charlie")
	var seen Announce
	if err := readFrame(t, charlie, TypeAnnounce).Into(&seen); err != nil {
		t.Fatalf("announce after reconnect: %v", err)
	}
	if seen.InstanceID != "alpha" {
		t.Errorf("charlie was introduced to %+v, want the reconnected alpha", seen)
	}
	waitFor(t, "alpha to see charlie", func() bool {
		sibs := alpha.Siblings()
		return len(sibs) == 1 && sibs[0].InstanceID == "charlie"
	})
}

func TestCallInFlightFailsWhenTheConnectionDies(t *testing.T) {
	addr, stop := relayOn(t, "127.0.0.1:0")
	alpha := startClient(t, addr, "shared-relay-test-key-0123456789ab", "alpha", nil)
	bravo := dialInstance(t, "ws://"+addr+connectPath, "shared-relay-test-key-0123456789ab", "bravo")
	readFrame(t, bravo, TypeAnnounce)
	waitFor(t, "alpha to see bravo", func() bool { return len(alpha.Siblings()) == 1 })

	failed := make(chan error, 1)
	go func() {
		_, err := alpha.Call(context.Background(), "bravo", ProxyCall{Method: http.MethodGet, Path: "/api/tasks"})
		failed <- err
	}()
	// bravo receives the call and never answers it.
	readFrame(t, bravo, TypeProxyRequest)
	stop()

	select {
	case err := <-failed:
		if err == nil {
			t.Fatal("the call succeeded, want it to fail with the connection")
		}
		// Two paths race and either is correct: alpha notices its own
		// connection died ("dropped"), or the relay notices bravo left first
		// and fails the call through failPending.
		if !strings.Contains(err.Error(), "dropped") && !strings.Contains(err.Error(), "disconnected before it replied") {
			t.Errorf("got %v, want it to say the connection dropped or that the target disconnected first", err)
		}
	case <-time.After(wsTimeout):
		t.Fatal("the call is still waiting; a dead connection must fail it at once")
	}
}

func TestConnectURL(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"https gets the path appended", "https://relay.example.com", "wss://relay.example.com/relay/connect"},
		{"a trailing slash is a bare host too", "https://relay.example.com/", "wss://relay.example.com/relay/connect"},
		{"http and a port", "http://192.168.20.30:8760", "ws://192.168.20.30:8760/relay/connect"},
		{"whitespace from a settings field", "  https://relay.example.com  ", "wss://relay.example.com/relay/connect"},
		{"a ws url is already one", "ws://127.0.0.1:8760/relay/connect", "ws://127.0.0.1:8760/relay/connect"},
		{"a mounted path is kept", "https://home.example.com/bv-relay/connect", "wss://home.example.com/bv-relay/connect"},
		{"an instance under a path has the relay below it", "https://example.com/bv", "wss://example.com/bv/relay/connect"},
		{"and with a trailing slash", "https://example.com/apps/bv/", "wss://example.com/apps/bv/relay/connect"},
		{"a full address under a path is kept", "https://example.com/bv/relay/connect", "wss://example.com/bv/relay/connect"},
		{"a slash after the socket goes", "https://example.com/bv/relay/connect/", "wss://example.com/bv/relay/connect"},
		{"no scheme", "relay.example.com", ""},
		{"a scheme nobody can dial", "ftp://relay.example.com", ""},
		{"empty", "", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ConnectURL(tc.in)
			if tc.want == "" {
				if err == nil {
					t.Fatalf("got %q, want %q rejected", got, tc.in)
				}
				return
			}
			if err != nil {
				t.Fatalf("ConnectURL(%q): %v", tc.in, err)
			}
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestNewClientRejectsMisconfiguration(t *testing.T) {
	valid := ClientOptions{
		URL:        "https://relay.example.com",
		Key:        "shared-relay-test-key-0123456789ab",
		FrameKey:   testFrameKey,
		InstanceID: "alpha",
		Serve:      func(context.Context, ProxyCall) (int, []byte) { return http.StatusNotImplemented, nil },
	}
	tests := []struct {
		name   string
		mangle func(o *ClientOptions)
	}{
		{"no address", func(o *ClientOptions) { o.URL = "" }},
		{"no key", func(o *ClientOptions) { o.Key = "   " }},
		{"no instance id", func(o *ClientOptions) { o.InstanceID = "" }},
		{"no frame key", func(o *ClientOptions) { o.FrameKey = nil }},
		{"a frame key of the wrong length", func(o *ClientOptions) { o.FrameKey = []byte("too short") }},
		{"no handler for a sibling's calls", func(o *ClientOptions) { o.Serve = nil }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			opts := valid
			tc.mangle(&opts)
			if _, err := NewClient(opts); err == nil {
				t.Fatal("accepted, want an error")
			}
		})
	}
	if _, err := NewClient(valid); err != nil {
		t.Fatalf("a valid configuration was rejected: %v", err)
	}
}
