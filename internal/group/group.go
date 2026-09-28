// Package group connects the BombVault instances that share one pairing
// phrase. Members on the same network talk directly; members elsewhere talk
// through a relay. Either way every call and answer is sealed under a key
// derived from the phrase, so neither a relay nor anything on the network in
// between can read or forge them.
package group

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/discovery"
	"github.com/junkerderprovinz/bombvault/internal/relay"
)

// peerAuthDomain separates the key that signs direct calls and discovery
// announces from the relay and frame keys.
const peerAuthDomain = "bombvault/peer-auth/v1"

// PeerAuthKey returns the HMAC key that signs direct calls and discovery
// announces between members.
func PeerAuthKey(secret []byte) []byte {
	h := sha256.New()
	h.Write([]byte(peerAuthDomain))
	h.Write(secret)
	return h.Sum(nil)
}

// Mode is which relay an instance uses for members it cannot reach directly.
type Mode string

// The relay modes.
const (
	ModeProject Mode = "project"
	ModeOwn     Mode = "own"
	ModeOff     Mode = "off"
)

// ValidMode reports whether m is one of the relay modes.
func ValidMode(m Mode) bool {
	return m == ModeProject || m == ModeOwn || m == ModeOff
}

// Config is what the Manager runs on. A nil Secret means this instance is in
// no group.
type Config struct {
	Secret     []byte
	InstanceID string
	Name       string
	Version    string
	// DirectURL is where this instance answers direct calls on the local
	// network, or empty when it cannot tell.
	DirectURL string
	Mode      Mode
	// RelayURL is the own relay's address, used in ModeOwn only.
	RelayURL string
}

// Member is another instance in the group that is reachable now.
type Member struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Version string `json:"version"`
	// Direct is true when the member announced itself on this network.
	Direct bool `json:"direct"`
	// Relay is true when the member is connected to the same relay.
	Relay bool `json:"relay"`
}

// ErrNotMember is returned for a call to an instance that is not reachable
// in this group.
var ErrNotMember = errors.New("group: no such member is reachable")

// directPath is the route a member answers direct calls on.
const directPath = "/api/group/call"

const (
	headerPeer      = "X-Bombvault-Peer"
	headerTarget    = "X-Bombvault-Target"
	headerTime      = "X-Bombvault-Time"
	headerSignature = "X-Bombvault-Signature"
	// MaxCallBytes caps a direct call as it arrives, before anything in it
	// is checked. A mesh offer, the largest call a member makes, stays far
	// below it.
	MaxCallBytes = 1 << 20
	// maxAnswerBytes caps a direct answer, which can carry a whole scorecard.
	maxAnswerBytes = 8 << 20
	// maxDirectCalls is how many direct calls are served at once.
	maxDirectCalls = 16
)

type keys struct {
	relayKey string
	frameKey []byte
	peerAuth []byte
}

func deriveKeys(secret []byte) *keys {
	return &keys{
		relayKey: relay.DeriveKey(secret),
		frameKey: relay.DeriveFrameKey(secret),
		peerAuth: PeerAuthKey(secret),
	}
}

// Manager keeps this instance's side of the group running: the discovery
// announce, the relay connection and the direct transport.
type Manager struct {
	serve  relay.Handler
	replay *relay.ReplayGuard
	disc   *discovery.Service
	hc     *http.Client
	// slots holds one token per direct call being served.
	slots chan struct{}

	mu     sync.Mutex
	cfg    Config
	keys   *keys
	client *relay.Client
}

// NewManager returns a Manager that answers members' calls with serve. It is
// in no group until Apply is called with a secret.
func NewManager(serve relay.Handler) *Manager {
	return &Manager{
		serve:  serve,
		replay: relay.NewReplayGuard(),
		disc:   discovery.New(),
		slots:  make(chan struct{}, maxDirectCalls),
		hc: &http.Client{
			Timeout: relay.CallTimeout,
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
			// Instances serve self-signed certificates, so there is nothing
			// to verify against. Calls and answers are sealed under the frame
			// key and signed with the peer-auth key, so the channel does not
			// rely on TLS for either secrecy or authenticity.
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec // G402: see the comment above
			},
		},
	}
}

// Start begins listening for members on the local network.
func (m *Manager) Start() { m.disc.Start() }

// Close stops the relay connection and the discovery service.
func (m *Manager) Close() {
	m.mu.Lock()
	c := m.client
	m.client = nil
	m.mu.Unlock()
	if c != nil {
		_ = c.Close()
	}
	_ = m.disc.Close()
}

// Apply switches the Manager to cfg: it rebuilds the relay connection and the
// discovery announce, and closes whatever the old configuration had open.
func (m *Manager) Apply(cfg Config) {
	var k *keys
	if len(cfg.Secret) > 0 {
		k = deriveKeys(cfg.Secret)
	}

	var next *relay.Client
	if k != nil {
		if url := relayURLFor(cfg); url != "" {
			c, err := relay.NewClient(relay.ClientOptions{
				URL:        url,
				Key:        k.relayKey,
				FrameKey:   k.frameKey,
				InstanceID: cfg.InstanceID,
				Identity:   relay.Identity{Name: cfg.Name, Version: cfg.Version},
				Serve:      m.serve,
				Replay:     m.replay,
			})
			if err != nil {
				log.Printf("group: relay not started: %v", err)
			} else {
				next = c
			}
		}
	}

	m.mu.Lock()
	prev := m.client
	m.cfg, m.keys, m.client = cfg, k, next
	m.mu.Unlock()
	if prev != nil {
		_ = prev.Close()
	}
	if next != nil {
		next.Start()
	}

	if k == nil {
		m.disc.SetAccept(nil)
		m.disc.SetSelf(discovery.Peer{})
		return
	}
	self := discovery.Peer{ID: cfg.InstanceID, Name: cfg.Name, Version: cfg.Version, URL: cfg.DirectURL}
	self.Tag = announceTag(k.peerAuth, self)
	m.disc.SetSelf(self)
	m.disc.SetAccept(func(p discovery.Peer) bool {
		return hmac.Equal([]byte(p.Tag), []byte(announceTag(k.peerAuth, p)))
	})
}

// relayURLFor is the address the relay client dials, or empty for none.
func relayURLFor(cfg Config) string {
	switch cfg.Mode {
	case ModeOff:
		return ""
	case ModeOwn:
		return strings.TrimSpace(cfg.RelayURL)
	default:
		return relay.DefaultURL
	}
}

// announceTag signs everything a discovery announce claims, so a device on
// the network cannot point members at an address of its choosing.
func announceTag(peerAuth []byte, p discovery.Peer) string {
	mac := hmac.New(sha256.New, peerAuth)
	mac.Write([]byte("announce\x00" + p.ID + "\x00" + p.URL + "\x00" + p.Name + "\x00" + p.Version))
	return hex.EncodeToString(mac.Sum(nil))
}

// Active reports whether this instance is in a group.
func (m *Manager) Active() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.keys != nil
}

// RelayConnected reports whether the relay socket is up.
func (m *Manager) RelayConnected() bool {
	m.mu.Lock()
	c := m.client
	m.mu.Unlock()
	return c != nil && c.Connected()
}

// AdmitsRelayKey reports whether key is this group's relay key, which is the
// only key a relay served by this instance lets in.
func (m *Manager) AdmitsRelayKey(key string) bool {
	m.mu.Lock()
	k := m.keys
	m.mu.Unlock()
	return k != nil && hmac.Equal([]byte(key), []byte(k.relayKey))
}

// Members lists the members reachable now, sorted by name.
func (m *Manager) Members() []Member {
	m.mu.Lock()
	c, active := m.client, m.keys != nil
	m.mu.Unlock()
	if !active {
		return []Member{}
	}
	byID := map[string]*Member{}
	for _, p := range m.disc.Peers() {
		byID[p.ID] = &Member{ID: p.ID, Name: p.Name, Version: p.Version, Direct: true}
	}
	if c != nil {
		for _, s := range c.Siblings() {
			if mem, ok := byID[s.InstanceID]; ok {
				mem.Relay = true
				continue
			}
			byID[s.InstanceID] = &Member{ID: s.InstanceID, Name: s.Name, Version: s.Version, Relay: true}
		}
	}
	out := make([]Member, 0, len(byID))
	for _, mem := range byID {
		out = append(out, *mem)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Name != out[j].Name {
			return out[i].Name < out[j].Name
		}
		return out[i].ID < out[j].ID
	})
	return out
}

// Call asks one member and returns its answer. A member on this network is
// asked directly; if that gets no answer and the member is on the relay too,
// the relay carries the call instead. An error means no answer came.
func (m *Manager) Call(ctx context.Context, memberID, method, path string, body []byte) (int, []byte, error) {
	m.mu.Lock()
	k, c, self := m.keys, m.client, m.cfg.InstanceID
	m.mu.Unlock()
	if k == nil {
		return 0, nil, ErrNotMember
	}
	call := relay.ProxyCall{Method: method, Path: path, Body: body}

	var directErr error
	for _, p := range m.disc.Peers() {
		if p.ID != memberID {
			continue
		}
		res, err := m.callDirect(ctx, k, self, p, call)
		if err == nil {
			return res.Status, res.Body, nil
		}
		directErr = err
	}
	if c != nil {
		for _, s := range c.Siblings() {
			if s.InstanceID == memberID {
				res, err := c.Call(ctx, memberID, call)
				if err != nil {
					return 0, nil, err
				}
				return res.Status, res.Body, nil
			}
		}
	}
	if directErr != nil {
		return 0, nil, directErr
	}
	return 0, nil, ErrNotMember
}

func (m *Manager) callDirect(ctx context.Context, k *keys, self string, p discovery.Peer, call relay.ProxyCall) (relay.ProxyResult, error) {
	id, err := relay.NewRequestID()
	if err != nil {
		return relay.ProxyResult{}, err
	}
	sealed, err := relay.SealCall(k.frameKey, id, p.ID, call)
	if err != nil {
		return relay.ProxyResult{}, err
	}
	payload, err := json.Marshal(relay.ProxyRequest{RequestID: id, Target: p.ID, Sealed: sealed})
	if err != nil {
		return relay.ProxyResult{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, relay.CallTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(p.URL, "/")+directPath, bytes.NewReader(payload))
	if err != nil {
		return relay.ProxyResult{}, err
	}
	now := strconv.FormatInt(time.Now().Unix(), 10)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(headerPeer, self)
	req.Header.Set(headerTarget, p.ID)
	req.Header.Set(headerTime, now)
	req.Header.Set(headerSignature, directSignature(k.peerAuth, self, now, payload))

	resp, err := m.hc.Do(req)
	if err != nil {
		return relay.ProxyResult{}, fmt.Errorf("group: %s is not reachable directly: %w", p.Name, err)
	}
	defer resp.Body.Close() //nolint:errcheck // nothing to do about a failed close of a read body
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxAnswerBytes))
	if err != nil {
		return relay.ProxyResult{}, err
	}
	if resp.StatusCode != http.StatusOK {
		return relay.ProxyResult{}, fmt.Errorf("group: %s refused the direct call with HTTP %d", p.Name, resp.StatusCode)
	}
	var out relay.ProxyResponse
	if err := json.Unmarshal(raw, &out); err != nil || out.RequestID != id {
		return relay.ProxyResult{}, fmt.Errorf("group: %s answered unreadably", p.Name)
	}
	return relay.OpenResult(k.frameKey, id, out.Sealed)
}

// directSignature binds a direct call's sender, time and exact body to the
// peer-auth key.
func directSignature(peerAuth []byte, sender, unix string, body []byte) string {
	sum := sha256.Sum256(body)
	mac := hmac.New(sha256.New, peerAuth)
	mac.Write([]byte("direct\x00" + sender + "\x00" + unix + "\x00" + hex.EncodeToString(sum[:])))
	return hex.EncodeToString(mac.Sum(nil))
}

// ServeDirect answers a member's direct call. Outside a group it answers like
// an instance without the feature. A call that is unsigned, stale, replayed,
// addressed to another instance or does not open is refused without saying
// which, since the reason would only help a guesser.
func (m *Manager) ServeDirect(w http.ResponseWriter, r *http.Request) {
	m.mu.Lock()
	k, self := m.keys, m.cfg.InstanceID
	m.mu.Unlock()
	if k == nil {
		http.NotFound(w, r)
		return
	}
	// The route is public, so whatever a stranger can send without the key
	// is checked before the body is read.
	sender, target, unix := r.Header.Get(headerPeer), r.Header.Get(headerTarget), r.Header.Get(headerTime)
	sent, err := strconv.ParseInt(unix, 10, 64)
	if err != nil || sender == "" || target != self || r.ContentLength > MaxCallBytes {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	if d := time.Since(time.Unix(sent, 0)); d > relay.ClockSkew || d < -relay.ClockSkew {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	select {
	case m.slots <- struct{}{}:
		defer func() { <-m.slots }()
	default:
		http.Error(w, "busy", http.StatusServiceUnavailable)
		return
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, MaxCallBytes))
	if err != nil {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	want := directSignature(k.peerAuth, sender, unix, body)
	if !hmac.Equal([]byte(r.Header.Get(headerSignature)), []byte(want)) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	var req relay.ProxyRequest
	if json.Unmarshal(body, &req) != nil || req.RequestID == "" || req.Target != target {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	call, err := relay.OpenCall(k.frameKey, req.RequestID, self, req.Sealed)
	if err != nil || !m.replay.Admit(call) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	status, out := m.serve(r.Context(), call)
	sealed, err := relay.SealResult(k.frameKey, req.RequestID, relay.ProxyResult{Status: status, Body: out})
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(relay.ProxyResponse{RequestID: req.RequestID, Sealed: sealed})
}
