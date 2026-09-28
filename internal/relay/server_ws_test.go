package relay

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
)

// wsTimeout bounds every step of the end-to-end tests, so a protocol mistake
// fails the run instead of hanging it.
const wsTimeout = 10 * time.Second

// dialInstance opens a real WebSocket to the relay and completes the hello
// handshake with a sealed identity, returning the connection an instance
// would then keep open.
func dialInstance(t *testing.T, url, key, id string) *websocket.Conn {
	t.Helper()
	sealed, err := SealIdentity(testFrameKey, id, Identity{Name: id})
	if err != nil {
		t.Fatalf("seal identity for %s: %v", id, err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), wsTimeout)
	defer cancel()
	c, _, err := websocket.Dial(ctx, url, nil)
	if err != nil {
		t.Fatalf("dial %s as %s: %v", url, id, err)
	}
	t.Cleanup(func() { _ = c.CloseNow() })
	writeFrame(t, c, TypeHello, Hello{Key: key, Announce: Announce{InstanceID: id, Sealed: sealed}})
	return c
}

// rawDial opens a WebSocket without completing the hello handshake, for a
// test that needs to send an unusual or malformed one itself.
func rawDial(t *testing.T, url string) *websocket.Conn {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), wsTimeout)
	defer cancel()
	c, _, err := websocket.Dial(ctx, url, nil)
	if err != nil {
		t.Fatalf("dial %s: %v", url, err)
	}
	t.Cleanup(func() { _ = c.CloseNow() })
	return c
}

func writeFrame(t *testing.T, c *websocket.Conn, typ string, data any) {
	t.Helper()
	frame, err := Encode(typ, data)
	if err != nil {
		t.Fatalf("encode %s: %v", typ, err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), wsTimeout)
	defer cancel()
	if err := c.Write(ctx, websocket.MessageText, frame); err != nil {
		t.Fatalf("write %s: %v", typ, err)
	}
}

func readFrame(t *testing.T, c *websocket.Conn, want string) Envelope {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), wsTimeout)
	defer cancel()
	_, frame, err := c.Read(ctx)
	if err != nil {
		t.Fatalf("waiting for a %s frame: %v", want, err)
	}
	env, err := Decode(frame)
	if err != nil {
		t.Fatalf("waiting for a %s frame: %s is not an envelope (%v)", want, frame, err)
	}
	if env.Type != want {
		t.Fatalf("got a %q frame, want %q", env.Type, want)
	}
	return env
}

// TestEndToEndOverRealWebSockets runs two real coder/websocket clients
// against the relay's http.Handler. The registry tests drive Join and Route
// directly and cannot catch a frame no real client can parse or a handshake
// the library rejects.
func TestEndToEndOverRealWebSockets(t *testing.T) {
	srv := httptest.NewServer(NewServer())
	t.Cleanup(srv.Close)
	url := "ws" + strings.TrimPrefix(srv.URL, "http") + connectPath

	alpha := dialInstance(t, url, "shared-relay-test-key-0123456789ab", "alpha")
	bravo := dialInstance(t, url, "shared-relay-test-key-0123456789ab", "bravo")

	// bravo joined last, so it hears about alpha; alpha hears bravo arrive.
	var sib Announce
	if err := readFrame(t, bravo, TypeAnnounce).Into(&sib); err != nil {
		t.Fatalf("sibling announce: %v", err)
	}
	if sib.InstanceID != "alpha" {
		t.Errorf("bravo was introduced to %+v, want alpha", sib)
	}
	if id, err := OpenIdentity(testFrameKey, sib.InstanceID, sib.Sealed); err != nil || id.Name != "alpha" {
		t.Errorf("alpha's identity opened as %+v (%v), want its own name", id, err)
	}
	var arrival Announce
	if err := readFrame(t, alpha, TypeAnnounce).Into(&arrival); err != nil {
		t.Fatalf("arrival announce: %v", err)
	}
	if arrival.InstanceID != "bravo" {
		t.Errorf("alpha was told about %+v, want bravo", arrival)
	}

	// Unlike server_test.go, both payloads are really sealed, so the round
	// trip through base64 in the JSON envelope is covered.
	call := ProxyCall{
		Method: "POST", Path: "/api/links",
		Body: []byte(`{"url":"https://example.invalid/file.bin"}`),
	}
	writeFrame(t, alpha, TypeProxyRequest, ProxyRequest{
		RequestID: "r1", Target: "bravo",
		Sealed: sealFor(t, "r1", "bravo", call),
	})
	var req ProxyRequest
	if err := readFrame(t, bravo, TypeProxyRequest).Into(&req); err != nil {
		t.Fatalf("proxy-request: %v", err)
	}
	if req.RequestID != "r1" || req.Target != "bravo" {
		t.Fatalf("bravo received %+v, want alpha's request unchanged", req)
	}
	got := openFrom(t, "r1", "bravo", req.Sealed)
	if got.Method != "POST" || got.Path != "/api/links" {
		t.Fatalf("bravo opened %+v, want alpha's request unchanged", got)
	}
	if string(got.Body) != `{"url":"https://example.invalid/file.bin"}` {
		t.Errorf("body arrived as %s, want it byte for byte", got.Body)
	}

	writeFrame(t, bravo, TypeProxyResponse, ProxyResponse{
		RequestID: req.RequestID,
		Sealed:    sealResultFor(t, req.RequestID, ProxyResult{Status: 201, Body: []byte(`{"added":1}`)}),
	})
	var resp ProxyResponse
	if err := readFrame(t, alpha, TypeProxyResponse).Into(&resp); err != nil {
		t.Fatalf("proxy-response: %v", err)
	}
	if resp.RequestID != "r1" {
		t.Fatalf("alpha received %+v, want bravo's own answer", resp)
	}
	res := openResultFrom(t, "r1", resp.Sealed)
	if res.Status != 201 || string(res.Body) != `{"added":1}` {
		t.Errorf("alpha opened %+v, want bravo's own answer", res)
	}

	// The Instances page's live status depends on a real socket closing, not
	// only on Leave.
	_ = bravo.CloseNow()
	var gone Presence
	if err := readFrame(t, alpha, TypePresence).Into(&gone); err != nil {
		t.Fatalf("presence: %v", err)
	}
	if gone.InstanceID != "bravo" || gone.Online {
		t.Errorf("got %+v, want bravo offline", gone)
	}
}

func TestConnectionWithoutAKeyIsRejected(t *testing.T) {
	relaySrv := NewServer()
	srv := httptest.NewServer(relaySrv)
	t.Cleanup(srv.Close)
	url := "ws" + strings.TrimPrefix(srv.URL, "http") + connectPath

	ctx, cancel := context.WithTimeout(context.Background(), wsTimeout)
	defer cancel()
	c, _, err := websocket.Dial(ctx, url, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { _ = c.CloseNow() })

	writeFrame(t, c, TypeHello, Hello{Announce: Announce{InstanceID: "alpha"}})
	if _, _, err := c.Read(ctx); err == nil {
		t.Fatal("a hello without a relay key was accepted")
	}
	if relaySrv.Len() != 0 {
		t.Errorf("%d connections registered, want the keyless one rejected", relaySrv.Len())
	}
}

func TestConnectionWithATooShortKeyIsRejected(t *testing.T) {
	relaySrv := NewServer()
	srv := httptest.NewServer(relaySrv)
	t.Cleanup(srv.Close)
	url := "ws" + strings.TrimPrefix(srv.URL, "http") + connectPath

	ctx, cancel := context.WithTimeout(context.Background(), wsTimeout)
	defer cancel()
	c, _, err := websocket.Dial(ctx, url, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { _ = c.CloseNow() })

	writeFrame(t, c, TypeHello, Hello{Key: "too-short", Announce: Announce{InstanceID: "alpha"}})
	if _, _, err := c.Read(ctx); err == nil {
		t.Fatal("a hello with a key shorter than minKeyLength was accepted")
	}
	if relaySrv.Len() != 0 {
		t.Errorf("%d connections registered, want the short-keyed one rejected", relaySrv.Len())
	}
}

func TestAnOversizedFrameBeforeTheHelloIsRefused(t *testing.T) {
	relaySrv := NewServer()
	srv := httptest.NewServer(relaySrv)
	t.Cleanup(srv.Close)
	c := rawDial(t, "ws"+strings.TrimPrefix(srv.URL, "http")+connectPath)

	ctx, cancel := context.WithTimeout(context.Background(), wsTimeout)
	defer cancel()
	frame, _ := Encode(TypeHello, Hello{Key: strings.Repeat("k", 64<<10), Announce: Announce{InstanceID: "alpha"}})
	_ = c.Write(ctx, websocket.MessageText, frame)
	if _, _, err := c.Read(ctx); websocket.CloseStatus(err) != websocket.StatusMessageTooBig {
		t.Fatalf("a 64 KiB first frame got %v, want the connection closed as too big", err)
	}
	if relaySrv.Len() != 0 {
		t.Errorf("%d connections registered, want none", relaySrv.Len())
	}
}

func TestFramesAfterTheHelloMayBeLarge(t *testing.T) {
	srv := httptest.NewServer(NewServer())
	t.Cleanup(srv.Close)
	url := "ws" + strings.TrimPrefix(srv.URL, "http") + connectPath
	alpha := dialInstance(t, url, "shared-relay-test-key-0123456789ab", "alpha")
	bravo := dialInstance(t, url, "shared-relay-test-key-0123456789ab", "bravo")
	bravo.SetReadLimit(readLimit)
	readFrame(t, alpha, TypeAnnounce)
	readFrame(t, bravo, TypeAnnounce)

	big := ProxyCall{Method: "POST", Path: "/x", Body: []byte(strings.Repeat("x", 1<<20))}
	writeFrame(t, alpha, TypeProxyRequest, ProxyRequest{RequestID: "r1", Target: "bravo", Sealed: sealFor(t, "r1", "bravo", big)})
	var req ProxyRequest
	if err := readFrame(t, bravo, TypeProxyRequest).Into(&req); err != nil || req.RequestID != "r1" {
		t.Fatalf("bravo got %+v, %v; want the 1 MiB call", req, err)
	}
}
