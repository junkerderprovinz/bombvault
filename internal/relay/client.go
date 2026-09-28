package relay

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"
)

// connectPath is appended to an address that does not name the socket, so
// "https://relay.example.com" works as well as the full connect URL.
const connectPath = "/relay/connect"

const (
	// CallTimeout bounds one call to a sibling, over the relay or directly.
	CallTimeout = 15 * time.Second
	// dialTimeout bounds one connection attempt.
	dialTimeout = 15 * time.Second
	minBackoff  = time.Second
	maxBackoff  = time.Minute
	// stableSession is how long a connection must last before the backoff
	// resets, so a relay that accepts and then hangs up is not redialled
	// every second.
	stableSession = 30 * time.Second
	// pingInterval keeps the socket alive through proxies that close an idle
	// upstream after a minute or so.
	pingInterval = 30 * time.Second
	pingTimeout  = 10 * time.Second
)

// Handler answers one call a sibling made to this instance. It gets the
// opened call, never a routing field the relay was free to write.
type Handler func(ctx context.Context, call ProxyCall) (status int, body []byte)

// ClientOptions configures a Client. Every field but Replay and OnChange is
// required.
type ClientOptions struct {
	// URL is the relay's address, http(s) or ws(s), with or without the
	// connect path.
	URL string
	// Key is the relay key from DeriveKey.
	Key string
	// FrameKey is the 32-byte key from DeriveFrameKey.
	FrameKey []byte
	// InstanceID addresses this instance on the relay.
	InstanceID string
	// Identity is what siblings see about this instance.
	Identity Identity
	// Serve answers calls siblings make to this instance.
	Serve Handler
	// Replay admits the calls Serve gets. nil gives the client a guard of its
	// own; an instance that also takes direct calls passes the guard those
	// go through.
	Replay *ReplayGuard
	// OnChange fires when a sibling arrives or leaves and when the connection
	// comes up or goes down. It runs on the client's goroutine and must not
	// block.
	OnChange func()
}

// Client is one instance's connection to a relay. Every method works whether
// or not the relay is reachable: a disconnected Client reports no siblings
// and fails calls at once.
type Client struct {
	url        string
	key        string
	frameKey   []byte
	instanceID string
	identity   Identity
	serve      Handler
	replay     *ReplayGuard
	onChange   func()

	minBackoff time.Duration
	maxBackoff time.Duration

	mu       sync.Mutex
	conn     *websocket.Conn
	siblings map[string]Sibling
	pending  map[string]chan ProxyResponse
	started  bool

	startOnce sync.Once
	closeOnce sync.Once
	stop      chan struct{}
	done      chan struct{}
}

// NewClient validates the configuration and returns a Client that is not
// connected yet. Each check here is a misconfiguration that would otherwise
// show up as a connection that never works.
func NewClient(opts ClientOptions) (*Client, error) {
	connect, err := ConnectURL(opts.URL)
	if err != nil {
		return nil, err
	}
	if len(opts.Key) < minKeyLength {
		return nil, errors.New("relay: the relay key is shorter than the relay accepts")
	}
	if opts.InstanceID == "" {
		return nil, errors.New("relay: no instance id to announce")
	}
	if len(opts.FrameKey) != 32 {
		return nil, fmt.Errorf("relay: frame key is %d bytes, want 32", len(opts.FrameKey))
	}
	if opts.Serve == nil {
		return nil, errors.New("relay: no handler for calls from siblings")
	}
	replay := opts.Replay
	if replay == nil {
		replay = NewReplayGuard()
	}
	return &Client{
		url:        connect,
		key:        opts.Key,
		frameKey:   opts.FrameKey,
		instanceID: opts.InstanceID,
		identity:   opts.Identity,
		serve:      opts.Serve,
		replay:     replay,
		onChange:   opts.OnChange,
		minBackoff: minBackoff,
		maxBackoff: maxBackoff,
		siblings:   map[string]Sibling{},
		pending:    map[string]chan ProxyResponse{},
		stop:       make(chan struct{}),
		done:       make(chan struct{}),
	}, nil
}

// Start connects in the background and keeps reconnecting until Close. A
// second call, or a call after Close, does nothing.
func (c *Client) Start() {
	c.startOnce.Do(func() {
		c.mu.Lock()
		defer c.mu.Unlock()
		select {
		case <-c.stop:
			return
		default:
		}
		c.started = true
		go c.run()
	})
}

// Close ends the connection and the reconnect loop and waits for the loop,
// so the caller can tear down whatever Serve talks to.
func (c *Client) Close() error {
	c.closeOnce.Do(func() { close(c.stop) })
	c.mu.Lock()
	conn, started := c.conn, c.started
	c.mu.Unlock()
	if conn != nil {
		_ = conn.CloseNow()
	}
	if started {
		<-c.done
	}
	return nil
}

// Connected reports whether the relay connection is up, which an empty
// sibling list alone cannot tell.
func (c *Client) Connected() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.conn != nil
}

// Siblings returns the members visible through the relay, sorted by instance
// id.
func (c *Client) Siblings() []Sibling {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]Sibling, 0, len(c.siblings))
	for _, s := range c.siblings {
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].InstanceID < out[j].InstanceID })
	return out
}

// Call asks one sibling through the relay and waits for its answer. An error
// means no answer came from the sibling; an answer is returned with its
// status however bad, so a caller can tell "the other instance said no" from
// "the other instance is gone".
func (c *Client) Call(ctx context.Context, target string, call ProxyCall) (ProxyResult, error) {
	id, err := NewRequestID()
	if err != nil {
		return ProxyResult{}, err
	}

	c.mu.Lock()
	conn := c.conn
	answer := make(chan ProxyResponse, 1)
	if conn != nil {
		c.pending[id] = answer
	}
	c.mu.Unlock()
	if conn == nil {
		return ProxyResult{}, errors.New("relay: not connected")
	}
	defer func() {
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
	}()

	ctx, cancel := context.WithTimeout(ctx, CallTimeout)
	defer cancel()
	sealed, err := SealCall(c.frameKey, id, target, call)
	if err != nil {
		return ProxyResult{}, err
	}
	if err := writeFrameTo(ctx, conn, frameOf(TypeProxyRequest, ProxyRequest{RequestID: id, Target: target, Sealed: sealed})); err != nil {
		return ProxyResult{}, fmt.Errorf("relay: %w", err)
	}

	select {
	case <-ctx.Done():
		return ProxyResult{}, fmt.Errorf("relay: %s did not answer: %w", target, ctx.Err())
	case resp := <-answer:
		// Error is the one field a hostile relay can write, so an unsealed
		// response is never taken as a result. That limits a relay to denial
		// of service.
		if resp.Error != "" {
			return ProxyResult{}, errors.New("relay: " + resp.Error)
		}
		result, err := OpenResult(c.frameKey, id, resp.Sealed)
		if err != nil {
			return ProxyResult{}, fmt.Errorf("relay: %s answered unreadably: %w", target, err)
		}
		return result, nil
	}
}

func (c *Client) run() {
	defer close(c.done)
	wait := c.minBackoff
	for {
		select {
		case <-c.stop:
			return
		default:
		}
		began := time.Now()
		c.session()
		if time.Since(began) >= stableSession {
			wait = c.minBackoff
		}
		if !c.sleep(wait) {
			return
		}
		wait = min(wait*2, c.maxBackoff)
	}
}

func (c *Client) sleep(d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-c.stop:
		return false
	case <-t.C:
		return true
	}
}

// session runs one connection from dial to death.
func (c *Client) session() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		select {
		case <-c.stop:
			cancel()
		case <-ctx.Done():
		}
	}()

	dialCtx, dialCancel := context.WithTimeout(ctx, dialTimeout)
	conn, _, err := websocket.Dial(dialCtx, c.url, nil)
	dialCancel()
	if err != nil {
		return
	}
	conn.SetReadLimit(readLimit)
	defer func() { _ = conn.CloseNow() }()

	sealed, err := SealIdentity(c.frameKey, c.instanceID, c.identity)
	if err != nil {
		log.Printf("relay: this instance's announce could not be sealed, not connecting: %v", err)
		return
	}
	// The hello goes out before the connection is published, so nothing is
	// sent over a socket that has not introduced itself.
	hello := Hello{Key: c.key, Announce: Announce{InstanceID: c.instanceID, Sealed: sealed}}
	if err := writeFrameTo(ctx, conn, frameOf(TypeHello, hello)); err != nil {
		return
	}

	c.connected(conn)
	defer c.disconnected()
	go c.keepalive(ctx, conn)

	for {
		_, frame, err := conn.Read(ctx)
		if err != nil {
			return
		}
		c.handle(ctx, conn, frame)
	}
}

func (c *Client) keepalive(ctx context.Context, conn *websocket.Conn) {
	t := time.NewTicker(pingInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			pingCtx, cancel := context.WithTimeout(ctx, pingTimeout)
			err := conn.Ping(pingCtx)
			cancel()
			if err != nil {
				_ = conn.CloseNow()
				return
			}
		}
	}
}

func (c *Client) handle(ctx context.Context, conn *websocket.Conn, frame []byte) {
	env, err := Decode(frame)
	if err != nil {
		return
	}
	switch env.Type {
	case TypeAnnounce:
		var a Announce
		if env.Into(&a) != nil || a.InstanceID == "" {
			return
		}
		// An announce that does not open was not sealed by a group member,
		// so it is not shown as one.
		id, err := OpenIdentity(c.frameKey, a.InstanceID, a.Sealed)
		if err != nil {
			return
		}
		c.mu.Lock()
		c.siblings[a.InstanceID] = Sibling{InstanceID: a.InstanceID, Identity: id}
		c.mu.Unlock()
		c.changed()
	case TypePresence:
		var p Presence
		if env.Into(&p) != nil || p.InstanceID == "" || p.Online {
			return
		}
		c.mu.Lock()
		delete(c.siblings, p.InstanceID)
		c.mu.Unlock()
		c.changed()
	case TypeProxyResponse:
		var resp ProxyResponse
		if env.Into(&resp) != nil || resp.RequestID == "" {
			return
		}
		c.deliver(resp)
	case TypeProxyRequest:
		var req ProxyRequest
		if env.Into(&req) != nil || req.RequestID == "" {
			return
		}
		// Answering runs a real handler, and the read loop has to stay free
		// for the next frame.
		go c.answer(ctx, conn, req)
	}
}

// answer runs one inbound call and sends the result back. A call that does
// not open gets no reply, since a reply would only confirm the key; nor does
// one that is old or ran before, which is a relay sending a captured frame
// again.
func (c *Client) answer(ctx context.Context, conn *websocket.Conn, req ProxyRequest) {
	call, err := OpenCall(c.frameKey, req.RequestID, c.instanceID, req.Sealed)
	if err != nil || !c.replay.Admit(call) {
		return
	}
	serveCtx, cancel := context.WithTimeout(ctx, CallTimeout)
	status, body := c.serve(serveCtx, call)
	cancel()
	sealed, err := SealResult(c.frameKey, req.RequestID, ProxyResult{Status: status, Body: body})
	if err != nil {
		return
	}
	_ = writeFrameTo(ctx, conn, frameOf(TypeProxyResponse, ProxyResponse{RequestID: req.RequestID, Sealed: sealed}))
}

func (c *Client) deliver(resp ProxyResponse) {
	c.mu.Lock()
	answer := c.pending[resp.RequestID]
	delete(c.pending, resp.RequestID)
	c.mu.Unlock()
	if answer != nil {
		answer <- resp
	}
}

func (c *Client) connected(conn *websocket.Conn) {
	c.mu.Lock()
	c.conn = conn
	c.mu.Unlock()
	c.changed()
}

// disconnected clears the siblings, which exist only while the relay says
// so, and fails every call in flight instead of letting it time out.
func (c *Client) disconnected() {
	c.mu.Lock()
	c.conn = nil
	c.siblings = map[string]Sibling{}
	waiting := make([]chan ProxyResponse, 0, len(c.pending))
	for id, answer := range c.pending {
		waiting = append(waiting, answer)
		delete(c.pending, id)
	}
	c.mu.Unlock()
	for _, answer := range waiting {
		answer <- ProxyResponse{Error: "the relay connection dropped before the answer arrived"}
	}
	c.changed()
}

func (c *Client) changed() {
	if c.onChange != nil {
		c.onChange()
	}
}

func writeFrameTo(ctx context.Context, conn *websocket.Conn, frame []byte) error {
	ctx, cancel := context.WithTimeout(ctx, writeTimeout)
	defer cancel()
	return conn.Write(ctx, websocket.MessageText, frame)
}

// NewRequestID returns a random token a response is matched back by. A
// counter would restart on reconnect and hand a late answer to whoever
// inherited the number.
func NewRequestID() (string, error) {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return hex.EncodeToString(raw), nil
}

// ConnectURL turns a configured relay address into the WebSocket URL to dial.
// A path ending in /connect names the socket and stays; any other path is
// where a proxy serves an instance, whose relay sits below it.
func ConnectURL(raw string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Host == "" {
		return "", fmt.Errorf("relay: %q is not a relay address", raw)
	}
	switch u.Scheme {
	case "http":
		u.Scheme = "ws"
	case "https":
		u.Scheme = "wss"
	case "ws", "wss":
	default:
		return "", fmt.Errorf("relay: %q must be an http(s) or ws(s) address", raw)
	}
	p := strings.TrimRight(u.Path, "/")
	if !strings.HasSuffix(p, "/connect") {
		p += connectPath
	}
	u.Path, u.RawPath = p, ""
	return u.String(), nil
}
