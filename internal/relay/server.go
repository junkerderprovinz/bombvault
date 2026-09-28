// Package relay carries sealed calls between BombVault instances that cannot
// reach each other directly. Each instance dials out to the relay, which
// groups connections by the key in their hello frame and forwards frames it
// cannot open.
//
// The relay holds no accounts and no database; its state lives in memory and
// rebuilds as instances reconnect. Since any invented key gets in, every
// limit in this file caps what one connection can cost the relay or its
// siblings.
package relay

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"time"

	"github.com/coder/websocket"
)

const (
	// queueDepth is how many frames may wait for one connection's writer.
	queueDepth = 64
	// writeTimeout bounds one frame write.
	writeTimeout = 5 * time.Second
	// helloTimeout bounds how long a socket may stay open without saying who
	// it is.
	helloTimeout = 10 * time.Second
	// readLimit caps one inbound frame.
	readLimit = 8 << 20
	// pendingTTL bounds how long an unanswered request is remembered.
	pendingTTL = 2 * time.Minute
	// minKeyLength rules out the laziest keys. DeriveKey gives 64 characters.
	minKeyLength = 32
	// maxPendingPerSender stops one sender from filling its target's queue
	// until enqueue evicts the target.
	maxPendingPerSender = 32
	// maxClientsPerKey and maxTotalClients bound what a guessed or invented
	// key can make the relay hold.
	maxClientsPerKey = 64
	maxTotalClients  = 2048
)

// Conn is the part of *websocket.Conn the registry uses, so it can be tested
// without sockets. The dynamic type must be comparable; connections are map
// keys.
type Conn interface {
	Write(ctx context.Context, typ websocket.MessageType, p []byte) error
	CloseNow() error
}

type client struct {
	conn     Conn
	key      string
	announce Announce
	send     chan []byte
	// quit ends the writer. send is never closed, because frames are pushed
	// into it without the server lock.
	quit chan struct{}
	once sync.Once
}

func (cl *client) stop() { cl.once.Do(func() { close(cl.quit) }) }

// pending is one request in flight. routeResponse checks an answer's sender
// against target, so a connection that guesses a live request id cannot forge
// the answer.
type pending struct {
	from    Conn
	target  Conn
	expires time.Time
}

type pendingFailure struct {
	requestID string
	from      Conn
}

// Server is the relay: the key-grouped connection registry plus the WebSocket
// endpoint that fills it.
type Server struct {
	mu      sync.Mutex
	clients map[Conn]*client
	keys    map[string]map[Conn]*client
	pending map[string]map[string]pending // relay key, then request id

	// Admit decides whether a key may connect. nil admits every key. An
	// instance serving a relay sets it so only its own group gets in, rather
	// than becoming a rendezvous for anyone who finds the address. It is set
	// once before the server is mounted.
	Admit func(key string) bool

	// ClientAddr names the bucket a failed handshake counts against. nil
	// uses the peer's IP; an instance behind a trusted reverse proxy sets it,
	// so the proxy's address is not one bucket shared by every client.
	ClientAddr func(*http.Request) string

	limiter *limiter
}

// NewServer returns an empty relay that admits every key.
func NewServer() *Server {
	return &Server{
		clients: map[Conn]*client{},
		keys:    map[string]map[Conn]*client{},
		pending: map[string]map[string]pending{},
		limiter: newLimiter(),
	}
}

func (s *Server) admits(key string) bool {
	return s.Admit == nil || s.Admit(key)
}

// Len reports how many connections are registered across every key.
func (s *Server) Len() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.clients)
}

// Join registers a connection under its key and introduces it to the group
// both ways. A connection announcing an instance id already on the key
// replaces the old one, which is what a reconnect after a silently dead
// socket looks like; requests still waiting on the old one fail at once.
// Join reports false when Admit no longer admits the key, the limits are
// reached or the connection joined already, and the caller then closes it.
func (s *Server) Join(key string, c Conn, a Announce) bool {
	cl := &client{
		conn:     c,
		key:      key,
		announce: a,
		send:     make(chan []byte, queueDepth),
		quit:     make(chan struct{}),
	}

	s.mu.Lock()
	// Admit is asked again under the lock, so a key revoked while this
	// connection was shaking hands cannot slip in behind Revoke.
	if _, ok := s.clients[c]; ok || !s.admits(key) {
		s.mu.Unlock()
		return false
	}
	group := s.keys[key]
	if group == nil {
		group = map[Conn]*client{}
		s.keys[key] = group
	}
	var replaced *client
	siblings := make([]*client, 0, len(group))
	for _, other := range group {
		if other.announce.InstanceID == a.InstanceID {
			replaced = other
			continue
		}
		siblings = append(siblings, other)
	}
	if replaced == nil && (len(s.clients) >= maxTotalClients || len(group) >= maxClientsPerKey) {
		s.mu.Unlock()
		return false
	}
	var failures []pendingFailure
	if replaced != nil {
		_, failures = s.removeLocked(replaced.conn)
	}
	s.clients[c] = cl
	group[c] = cl
	s.mu.Unlock()

	if replaced != nil {
		replaced.stop()
	}
	s.failPending(failures, "the instance that would have answered this reconnected before it replied; retry")
	go s.writeLoop(cl)

	for _, sib := range siblings {
		s.enqueue(cl, frameOf(TypeAnnounce, sib.announce))
	}
	arrival := frameOf(TypeAnnounce, a)
	for _, sib := range siblings {
		s.enqueue(sib, arrival)
	}
	return true
}

// Leave unregisters a connection, tells its siblings it went offline and
// fails any request still waiting on it. It is safe to call twice and for a
// connection that never joined.
func (s *Server) Leave(c Conn) {
	s.mu.Lock()
	cl, failures := s.removeLocked(c)
	var siblings []*client
	if cl != nil {
		for _, sib := range s.keys[cl.key] {
			siblings = append(siblings, sib)
		}
	}
	s.mu.Unlock()
	if cl == nil {
		return
	}
	cl.stop()
	s.failPending(failures, "the instance that would have answered this disconnected before it replied")
	gone := frameOf(TypePresence, Presence{InstanceID: cl.announce.InstanceID})
	for _, sib := range siblings {
		s.enqueue(sib, gone)
	}
}

// Revoke closes every connection whose key Admit no longer admits. An
// instance serving a relay calls it when its group or its serve switch
// changes, since Admit is only asked on connect.
func (s *Server) Revoke() {
	s.mu.Lock()
	var gone []Conn
	for c, cl := range s.clients {
		if !s.admits(cl.key) {
			gone = append(gone, c)
		}
	}
	s.mu.Unlock()
	for _, c := range gone {
		s.Leave(c)
		_ = c.CloseNow()
	}
}

// Route handles one frame a client sent. Only the proxy frames mean anything
// inbound; anything else is ignored rather than closing the connection.
func (s *Server) Route(c Conn, frame []byte) {
	env, err := Decode(frame)
	if err != nil {
		return
	}
	switch env.Type {
	case TypeProxyRequest:
		var req ProxyRequest
		if env.Into(&req) != nil || req.RequestID == "" || req.Target == "" {
			return
		}
		s.routeRequest(c, req, frame)
	case TypeProxyResponse:
		var resp ProxyResponse
		if env.Into(&resp) != nil || resp.RequestID == "" {
			return
		}
		s.routeResponse(c, resp.RequestID, frame)
	}
}

// routeRequest forwards a call verbatim to the sibling it names. An unknown
// target, or a sender at maxPendingPerSender, gets an error response at once
// instead of waiting out its timeout.
func (s *Server) routeRequest(c Conn, req ProxyRequest, frame []byte) {
	s.mu.Lock()
	cl := s.clients[c]
	if cl == nil {
		s.mu.Unlock()
		return
	}
	var target *client
	for _, sib := range s.keys[cl.key] {
		if sib.conn != c && sib.announce.InstanceID == req.Target {
			target = sib
			break
		}
	}
	keyPending := s.pending[cl.key]
	if keyPending == nil {
		keyPending = map[string]pending{}
		s.pending[cl.key] = keyPending
	}
	sweepPendingLocked(keyPending)

	var refusal *ProxyResponse
	switch {
	case target == nil:
		refusal = &ProxyResponse{RequestID: req.RequestID, Error: "no instance " + req.Target + " is connected with this relay key"}
	case inFlightFromLocked(keyPending, c) >= maxPendingPerSender:
		refusal = &ProxyResponse{RequestID: req.RequestID, Error: "too many requests from this instance are still waiting on an answer"}
	default:
		keyPending[req.RequestID] = pending{from: c, target: target.conn, expires: time.Now().Add(pendingTTL)}
	}
	s.mu.Unlock()

	if refusal != nil {
		s.enqueue(cl, frameOf(TypeProxyResponse, *refusal))
		return
	}
	s.enqueue(target, frame)
}

// routeResponse sends an answer back to whoever asked, using the relay's own
// record of the request. A frame from any connection other than the one the
// request went to is dropped.
func (s *Server) routeResponse(c Conn, requestID string, frame []byte) {
	s.mu.Lock()
	cl := s.clients[c]
	if cl == nil {
		s.mu.Unlock()
		return
	}
	keyPending := s.pending[cl.key]
	p, ok := keyPending[requestID]
	if !ok || p.target != c {
		s.mu.Unlock()
		return
	}
	delete(keyPending, requestID)
	from := s.clients[p.from]
	s.mu.Unlock()
	if from == nil {
		return
	}
	s.enqueue(from, frame)
}

// removeLocked unregisters a connection and returns it with the requests its
// departure orphaned. The caller holds mu.
func (s *Server) removeLocked(c Conn) (*client, []pendingFailure) {
	cl := s.clients[c]
	if cl == nil {
		return nil, nil
	}
	delete(s.clients, c)
	group := s.keys[cl.key]
	delete(group, c)
	if len(group) == 0 {
		delete(s.keys, cl.key)
	}
	keyPending := s.pending[cl.key]
	var failures []pendingFailure
	for id, p := range keyPending {
		switch c {
		case p.from:
			delete(keyPending, id)
		case p.target:
			delete(keyPending, id)
			failures = append(failures, pendingFailure{requestID: id, from: p.from})
		}
	}
	if len(keyPending) == 0 {
		delete(s.pending, cl.key)
	}
	return cl, failures
}

// failPending answers each orphaned request with an error. It runs without
// the lock, since enqueue takes it again.
func (s *Server) failPending(failures []pendingFailure, reason string) {
	for _, f := range failures {
		s.mu.Lock()
		from := s.clients[f.from]
		s.mu.Unlock()
		if from == nil {
			continue
		}
		s.enqueue(from, frameOf(TypeProxyResponse, ProxyResponse{RequestID: f.requestID, Error: reason}))
	}
}

func sweepPendingLocked(keyPending map[string]pending) {
	now := time.Now()
	for id, p := range keyPending {
		if now.After(p.expires) {
			delete(keyPending, id)
		}
	}
}

func inFlightFromLocked(keyPending map[string]pending, from Conn) int {
	n := 0
	for _, p := range keyPending {
		if p.from == from {
			n++
		}
	}
	return n
}

// enqueue hands one frame to a connection, or drops the connection if its
// queue is full. A dropped client reconnects, whereas a relay stalled behind
// one bad link stops routing for the whole group.
func (s *Server) enqueue(cl *client, frame []byte) {
	select {
	case cl.send <- frame:
	default:
		s.Leave(cl.conn)
	}
}

// writeLoop is the only goroutine that writes to a connection, which keeps a
// slow link off the routing path.
func (s *Server) writeLoop(cl *client) {
	defer func() { _ = cl.conn.CloseNow() }()
	for {
		select {
		case <-cl.quit:
			return
		default:
		}
		select {
		case <-cl.quit:
			return
		case frame := <-cl.send:
			ctx, cancel := context.WithTimeout(context.Background(), writeTimeout)
			err := cl.conn.Write(ctx, websocket.MessageText, frame)
			cancel()
			if err != nil {
				s.Leave(cl.conn)
				return
			}
		}
	}
}

// frameOf marshals one outbound frame. Every payload is a protocol struct
// encoding/json cannot fail on.
func frameOf(typ string, data any) []byte {
	b, _ := Encode(typ, data)
	return b
}

// ServeHTTP is the relay endpoint: one long-lived WebSocket per instance.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	addr := s.clientAddrOf(r)
	if s.limiter.blocked(addr) {
		http.Error(w, "too many failed handshakes from this address", http.StatusTooManyRequests)
		return
	}

	// No origin check: there is no cookie to ride, the key in the first
	// frame is the only credential, and every client is cross-origin.
	c, err := websocket.Accept(w, r, &websocket.AcceptOptions{OriginPatterns: []string{"*"}})
	if err != nil {
		return
	}
	c.SetReadLimit(readLimit)

	hello, err := readHello(r.Context(), c)
	if err != nil {
		s.limiter.fail(addr)
		_ = c.Close(websocket.StatusPolicyViolation, "the first frame must be a hello with a relay key and an instance id")
		return
	}
	if !s.admits(hello.Key) {
		s.limiter.fail(addr)
		_ = c.Close(websocket.StatusPolicyViolation, "this relay does not serve that relay key")
		return
	}
	s.limiter.succeed(addr)
	if !s.Join(hello.Key, c, hello.Announce) {
		_ = c.Close(websocket.StatusPolicyViolation, "this relay cannot take the connection: the key is no longer served or too many instances use it")
		return
	}
	defer s.Leave(c)
	for {
		_, frame, err := c.Read(r.Context())
		if err != nil {
			return
		}
		s.Route(c, frame)
	}
}

func readHello(ctx context.Context, c *websocket.Conn) (Hello, error) {
	ctx, cancel := context.WithTimeout(ctx, helloTimeout)
	defer cancel()
	_, frame, err := c.Read(ctx)
	if err != nil {
		return Hello{}, err
	}
	env, err := Decode(frame)
	if err != nil {
		return Hello{}, err
	}
	if env.Type != TypeHello {
		return Hello{}, errors.New("first frame is not a hello")
	}
	var h Hello
	if err := env.Into(&h); err != nil {
		return Hello{}, err
	}
	if len(h.Key) < minKeyLength || h.Announce.InstanceID == "" {
		return Hello{}, errors.New("hello carries no relay key, a key shorter than the minimum, or no instance id")
	}
	return h, nil
}
