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
	"net"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

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
	// Address is where the member takes direct calls, as it announced it on
	// this network or passed it over the relay; empty when none is known.
	Address string `json:"address"`
	// Kind is what the member said it is over the relay, empty for an instance.
	Kind string `json:"kind,omitempty"`
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

// ProbeTarget stands in for a member id on a call whose sender does not yet
// know who, if anyone, will answer: the LAN sweep and a manually entered
// address both dial a host with nothing but the group secret to go on. It
// can never equal a real instance id, which the migration mints as random
// hex.
const ProbeTarget = "*"

// ProbePath is the only route a call addressed to ProbeTarget may reach.
// ServeDirect refuses every other path under that target, so a blind probe
// cannot be turned into a call against a route meant for a known member.
const ProbePath = "/api/group/peer/hello"

// Hello is what a member answers a probe with: enough to add it as a known
// member without running any other route first.
type Hello struct {
	OK         bool   `json:"ok"`
	InstanceID string `json:"instanceId"`
	Name       string `json:"name"`
	Version    string `json:"version"`
	// DirectURL is where the answering instance itself takes direct calls,
	// when it knows one; empty leaves the dialled address as the only one on
	// record.
	DirectURL string `json:"directUrl,omitempty"`
}

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

// sameKeys reports whether a and b belong to the same group, both nil (no
// group) counting as the same.
func sameKeys(a, b *keys) bool {
	if a == nil || b == nil {
		return a == b
	}
	return hmac.Equal(a.peerAuth, b.peerAuth)
}

// remoteAddr is one member's known direct address: where it was last seen
// answering, and whether that was a real answer (confirmed) or only a
// hint passed along by a relay call (noted). Members shows Direct only for
// a confirmed one, and Call only dials a stored address at all.
type remoteAddr struct {
	url       string
	name      string
	version   string
	confirmed time.Time // zero: never confirmed
	noted     time.Time
}

// addrTTL is how long a confirmed direct address keeps counting as Direct.
// It matches roughly how long a stale discovery peer lingers, so both routes
// to "reachable directly" age out on a similar clock once something stops
// answering.
const addrTTL = 5 * time.Minute

// Manager keeps this instance's side of the group running: the discovery
// announce, the relay connection and the direct transport.
type Manager struct {
	serve   relay.Handler
	replay  *relay.ReplayGuard
	disc    *discovery.Service
	hc      *http.Client
	limiter *relay.Limiter
	// slots holds one token per direct call being served.
	slots chan struct{}

	mu     sync.Mutex
	cfg    Config
	keys   *keys
	client *relay.Client

	addrMu sync.Mutex
	addrs  map[string]*remoteAddr

	sweepMu   sync.Mutex
	lastSweep time.Time

	stop     chan struct{}
	stopOnce sync.Once
}

// NewManager returns a Manager that answers members' calls with serve. It is
// in no group until Apply is called with a secret.
func NewManager(serve relay.Handler) *Manager {
	return &Manager{
		serve:   serve,
		replay:  relay.NewReplayGuard(),
		disc:    discovery.New(),
		limiter: relay.NewLimiter(),
		slots:   make(chan struct{}, maxDirectCalls),
		addrs:   map[string]*remoteAddr{},
		stop:    make(chan struct{}),
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

// Start begins listening for members on the local network and keeps the
// members found by address in reach.
func (m *Manager) Start() {
	m.disc.Start()
	go m.keepAlive()
}

// Close stops the relay connection, the discovery service and the refresh.
func (m *Manager) Close() {
	m.stopOnce.Do(func() { close(m.stop) })
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
	cfg.Name = clipName(cfg.Name)
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
	prev, prevKeys := m.client, m.keys
	m.cfg, m.keys, m.client = cfg, k, next
	m.mu.Unlock()
	if !sameKeys(prevKeys, k) {
		// A different group, or none, makes every address this instance
		// learned for the old one meaningless, and a sweep due under the old
		// group must not carry over and skip the new one's first sweep.
		m.addrMu.Lock()
		m.addrs = map[string]*remoteAddr{}
		m.addrMu.Unlock()
		m.sweepMu.Lock()
		m.lastSweep = time.Time{}
		m.sweepMu.Unlock()
	}
	if prev != nil {
		_ = prev.Close()
	}
	if next != nil {
		next.Start()
	}

	if k == nil {
		m.disc.SetAccept(nil)
		m.disc.SetSelf(discovery.Peer{}, nil)
		return
	}
	self := discovery.Peer{ID: cfg.InstanceID, Name: cfg.Name, Version: cfg.Version, URL: cfg.DirectURL}
	m.disc.SetSelf(self, func(p discovery.Peer) string { return announceTag(k.peerAuth, p) })
	m.disc.SetAccept(func(p discovery.Peer) bool {
		if d := time.Since(time.Unix(p.Sent, 0)); d > relay.ClockSkew || d < -relay.ClockSkew {
			return false
		}
		return hmac.Equal([]byte(p.Tag), []byte(announceTag(k.peerAuth, p)))
	})
}

// maxNameBytes keeps the name an instance goes by inside what a discovery
// announce and a relay hello may carry.
const maxNameBytes = 200

func clipName(name string) string {
	if len(name) <= maxNameBytes {
		return name
	}
	cut := maxNameBytes
	for cut > 0 && !utf8.RuneStart(name[cut]) {
		cut--
	}
	return name[:cut]
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

// announceTag signs everything a discovery announce claims, its time
// included, so a device on the network can neither point members at an
// address of its choosing nor keep a departed member listed by playing its
// announces back.
func announceTag(peerAuth []byte, p discovery.Peer) string {
	mac := hmac.New(sha256.New, peerAuth)
	mac.Write([]byte("announce\x00" + p.ID + "\x00" + p.URL + "\x00" + p.Name + "\x00" + p.Version + "\x00" + strconv.FormatInt(p.Sent, 10)))
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

// Members lists the members reachable now, sorted by name. A member shows as
// Direct when multicast placed it on this network or a direct call to it,
// found by the address exchange or the LAN sweep, has answered recently.
func (m *Manager) Members() []Member {
	m.mu.Lock()
	c, active := m.client, m.keys != nil
	m.mu.Unlock()
	if !active {
		return []Member{}
	}
	byID := map[string]*Member{}
	for _, p := range m.disc.Peers() {
		byID[p.ID] = &Member{ID: p.ID, Name: p.Name, Version: p.Version, Direct: true, Address: p.URL}
	}
	if c != nil {
		for _, s := range c.Siblings() {
			if mem, ok := byID[s.InstanceID]; ok {
				mem.Relay, mem.Kind = true, s.Kind
				continue
			}
			byID[s.InstanceID] = &Member{ID: s.InstanceID, Name: s.Name, Version: s.Version, Kind: s.Kind, Relay: true, Direct: m.confirmedDirect(s.InstanceID)}
		}
	}
	// A member with no relay and no multicast, only a sweep or a manually
	// probed address, would otherwise never appear: this is the only route
	// that tells the pairing card about it at all.
	for id, a := range m.confirmedAddresses() {
		if _, ok := byID[id]; ok {
			continue
		}
		byID[id] = &Member{ID: id, Name: a.name, Version: a.version, Direct: true, Address: a.url}
	}
	out := make([]Member, 0, len(byID))
	for id, mem := range byID {
		if mem.Address == "" {
			mem.Address, _ = m.knownAddress(id)
		}
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

// NoteAddress records url as a hint for where memberID might answer direct
// calls, without treating it as reachable yet: only a call that actually
// answers there, through ConfirmAddress, counts as Direct. It is what the
// address exchange over the relay feeds, since a member's say-so about its
// own address is not proof it works.
func (m *Manager) NoteAddress(memberID, url string) {
	memberID, url = strings.TrimSpace(memberID), strings.TrimSpace(url)
	if memberID == "" || url == "" || memberID == m.selfID() {
		return
	}
	m.addrMu.Lock()
	defer m.addrMu.Unlock()
	a := m.addrs[memberID]
	if a == nil {
		a = &remoteAddr{}
		m.addrs[memberID] = a
	}
	if a.url != url {
		a.url, a.confirmed = url, time.Time{}
	}
	a.noted = time.Now()
}

// ConfirmAddress records that memberID answered a direct call at url just
// now. name and version, when not empty, replace what is on record; an empty
// one leaves what is already known alone, so a probe's bare identification
// never blanks a name learned elsewhere.
func (m *Manager) ConfirmAddress(memberID, url, name, version string) {
	memberID, url = strings.TrimSpace(memberID), strings.TrimSpace(url)
	if memberID == "" || url == "" || memberID == m.selfID() {
		return
	}
	m.addrMu.Lock()
	defer m.addrMu.Unlock()
	a := m.addrs[memberID]
	if a == nil {
		a = &remoteAddr{}
		m.addrs[memberID] = a
	}
	a.url = url
	if name != "" {
		a.name = name
	}
	if version != "" {
		a.version = version
	}
	a.confirmed = time.Now()
	a.noted = a.confirmed
}

func (m *Manager) selfID() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.cfg.InstanceID
}

// knownAddress is any address on record for memberID, confirmed or only
// noted, for Call to try before giving up on a direct route.
func (m *Manager) knownAddress(memberID string) (string, bool) {
	m.addrMu.Lock()
	defer m.addrMu.Unlock()
	a := m.addrs[memberID]
	if a == nil || a.url == "" {
		return "", false
	}
	return a.url, true
}

func (m *Manager) confirmedDirect(memberID string) bool {
	m.addrMu.Lock()
	defer m.addrMu.Unlock()
	a := m.addrs[memberID]
	return a != nil && !a.confirmed.IsZero() && time.Since(a.confirmed) < addrTTL
}

// confirmedAddresses snapshots every member address that answered within
// addrTTL.
func (m *Manager) confirmedAddresses() map[string]remoteAddr {
	m.addrMu.Lock()
	defer m.addrMu.Unlock()
	out := map[string]remoteAddr{}
	now := time.Now()
	for id, a := range m.addrs {
		if !a.confirmed.IsZero() && now.Sub(a.confirmed) < addrTTL {
			out[id] = *a
		}
	}
	return out
}

// Call asks one member and returns its answer. A member found by multicast on
// this network is asked directly first; a member reachable only through an
// address the exchange or the sweep found is asked directly next; if neither
// answers and the member is on the relay too, the relay carries the call
// instead. An error means no answer came.
func (m *Manager) Call(ctx context.Context, memberID, method, path string, body []byte) (int, []byte, error) {
	m.mu.Lock()
	k, c, self := m.keys, m.client, m.cfg.InstanceID
	m.mu.Unlock()
	if k == nil {
		return 0, nil, ErrNotMember
	}
	call := relay.ProxyCall{Method: method, Path: path, Body: body}

	var directErr error
	tried := map[string]bool{}
	for _, p := range m.disc.Peers() {
		if p.ID != memberID {
			continue
		}
		tried[p.URL] = true
		res, err := m.callDirect(ctx, k, self, p, call)
		if err == nil {
			m.ConfirmAddress(memberID, p.URL, p.Name, p.Version)
			return res.Status, res.Body, nil
		}
		directErr = err
	}
	if addr, ok := m.knownAddress(memberID); ok && !tried[addr] {
		res, err := m.callDirect(ctx, k, self, discovery.Peer{ID: memberID, URL: addr}, call)
		if err == nil {
			m.ConfirmAddress(memberID, addr, "", "")
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

// Probe makes one blind, signed call to addr and reports whether a group
// member answered, without knowing beforehand who, if anyone, is there. It
// is what the LAN sweep and a manually entered address both use, and it
// records a successful answer the same way a normal direct call does.
func (m *Manager) Probe(ctx context.Context, addr string) (Hello, error) {
	m.mu.Lock()
	k, self, own := m.keys, m.cfg.InstanceID, m.cfg.DirectURL
	m.mu.Unlock()
	if k == nil {
		return Hello{}, ErrNotMember
	}
	intro, err := json.Marshal(probeIntro{DirectURL: own})
	if err != nil {
		return Hello{}, err
	}
	call := relay.ProxyCall{Method: http.MethodGet, Path: ProbePath, Body: intro}
	res, err := m.callDirect(ctx, k, self, discovery.Peer{ID: ProbeTarget, URL: addr, Name: addr}, call)
	if err != nil {
		return Hello{}, err
	}
	var h Hello
	if err := json.Unmarshal(res.Body, &h); err != nil || !h.OK || h.InstanceID == "" {
		return Hello{}, fmt.Errorf("group: %s answered a probe unreadably", addr)
	}
	confirmedAt := addr
	if strings.TrimSpace(h.DirectURL) != "" {
		confirmedAt = h.DirectURL
	}
	m.ConfirmAddress(h.InstanceID, confirmedAt, h.Name, h.Version)
	return h, nil
}

// probeIntro is what a probe tells the member it reaches: where the prober
// itself takes direct calls, so the pair knows each other after one probe
// instead of waiting for the other side's own sweep.
type probeIntro struct {
	DirectURL string `json:"directUrl,omitempty"`
}

// probeBack answers a probe's introduction by probing the prober at the
// address it named. Only a member not yet confirmed is probed back, so two
// instances stop after one round instead of probing each other forever.
func (m *Manager) probeBack(sender string, body []byte) {
	var in probeIntro
	if json.Unmarshal(body, &in) != nil {
		return
	}
	u, err := url.Parse(strings.TrimSpace(in.DirectURL))
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || m.confirmedDirect(sender) {
		return
	}
	m.NoteAddress(sender, u.String())
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*sweepTimeout)
		defer cancel()
		_, _ = m.Probe(ctx, u.String())
	}()
}

// The LAN sweep's bounds: sweepConcurrency limits how many probes run at
// once, sweepTimeout how long each may take, sweepBudget the whole sweep,
// and sweepMinInterval how often one may start at all.
const (
	sweepConcurrency = 32
	sweepTimeout     = 1500 * time.Millisecond
	sweepBudget      = 45 * time.Second
	sweepMinInterval = 3 * time.Minute
)

// refreshEvery is how often the Manager asks its known members again, well
// inside addrTTL. Without it a member reached only by address, with no relay
// and no multicast between the two, drops out of the group after addrTTL and
// comes back only once somebody opens a page that triggers a sweep.
const refreshEvery = addrTTL / 2

func (m *Manager) keepAlive() {
	t := time.NewTicker(refreshEvery)
	defer t.Stop()
	for {
		select {
		case <-m.stop:
			return
		case <-t.C:
			m.refresh(context.Background())
		}
	}
}

// refresh probes every member address on record, which confirms the ones that
// still answer, and then starts a sweep if one is due.
func (m *Manager) refresh(ctx context.Context) {
	m.mu.Lock()
	active := m.keys != nil
	m.mu.Unlock()
	if !active {
		return
	}
	for _, url := range m.knownURLs() {
		cctx, cancel := context.WithTimeout(ctx, sweepTimeout)
		_, _ = m.Probe(cctx, url)
		cancel()
	}
	m.MaybeSweep()
}

func (m *Manager) knownURLs() []string {
	m.addrMu.Lock()
	defer m.addrMu.Unlock()
	out := make([]string, 0, len(m.addrs))
	for _, a := range m.addrs {
		if a.url != "" {
			out = append(out, a.url)
		}
	}
	return out
}

// NeedsSweep reports whether the LAN sweep should run: this instance is in a
// group, and either nobody has been found by any route yet, or the relay
// knows of a sibling this instance has no direct address for.
func (m *Manager) NeedsSweep() bool {
	m.mu.Lock()
	active, c := m.keys != nil, m.client
	m.mu.Unlock()
	if !active {
		return false
	}
	disc := m.disc.Peers()
	confirmed := m.confirmedAddresses()
	if len(disc) == 0 && len(confirmed) == 0 {
		return true
	}
	if c == nil {
		return false
	}
	known := make(map[string]bool, len(disc)+len(confirmed))
	for _, p := range disc {
		known[p.ID] = true
	}
	for id := range confirmed {
		known[id] = true
	}
	for _, s := range c.Siblings() {
		if !known[s.InstanceID] {
			return true
		}
	}
	return false
}

// MaybeSweep starts a LAN sweep in the background when one is due, and
// returns at once either way. It is cheap enough to call on every group
// page load: NeedsSweep decides whether there is anything to look for, and
// the timestamp below throttles how often a sweep actually runs.
func (m *Manager) MaybeSweep() {
	if !m.NeedsSweep() {
		return
	}
	m.sweepMu.Lock()
	if !m.lastSweep.IsZero() && time.Since(m.lastSweep) < sweepMinInterval {
		m.sweepMu.Unlock()
		return
	}
	m.lastSweep = time.Now()
	m.sweepMu.Unlock()
	go m.sweepOnce(context.Background())
}

func (m *Manager) sweepOnce(ctx context.Context) {
	m.mu.Lock()
	own := m.cfg.DirectURL
	m.mu.Unlock()
	ctx, cancel := context.WithTimeout(ctx, sweepBudget)
	defer cancel()
	m.sweepWith(ctx, subnetCandidates(own))
}

// sweepWith probes every candidate address with bounded concurrency. It
// takes the candidate list as a parameter, rather than computing it itself,
// so a test can hand it a short list that includes a fake address instead of
// a real /24.
func (m *Manager) sweepWith(ctx context.Context, candidates []string) {
	if len(candidates) == 0 {
		return
	}
	sem := make(chan struct{}, sweepConcurrency)
	var wg sync.WaitGroup
	for _, addr := range candidates {
		wg.Add(1)
		sem <- struct{}{}
		go func(addr string) {
			defer wg.Done()
			defer func() { <-sem }()
			cctx, cancel := context.WithTimeout(ctx, sweepTimeout)
			defer cancel()
			_, _ = m.Probe(cctx, addr)
		}(addr)
	}
	wg.Wait()
}

// subnetCandidates lists own's port and port 3443 on every other host of
// own's /24, or nil when own is not a private IPv4 address. Scanning a
// public network, or one this instance only reaches through a reverse proxy,
// is exactly what the sweep must never do, so an address that does not
// clearly name this instance's own LAN interface yields no candidates at
// all rather than a guess.
func subnetCandidates(own string) []string {
	u, err := url.Parse(own)
	if err != nil || u.Hostname() == "" {
		return nil
	}
	ip := net.ParseIP(u.Hostname()).To4()
	if ip == nil || !discovery.PrivateIPv4(ip) {
		return nil
	}
	scheme := u.Scheme
	if scheme == "" {
		scheme = "https"
	}
	ports := []string{"3443"}
	if p := u.Port(); p != "" && p != "3443" {
		ports = append(ports, p)
	}
	base := ip.Mask(net.CIDRMask(24, 32))
	self := ip[3]
	ownPort := u.Port()
	if ownPort == "" {
		ownPort = "443"
		if scheme == "http" {
			ownPort = "80"
		}
	}
	out := make([]string, 0, 254*len(ports))
	for host := 1; host <= 254; host++ {
		addr := net.IPv4(base[0], base[1], base[2], byte(host)).String()
		for _, port := range ports {
			// A second instance on this same machine publishes another port.
			if byte(host) == self && port == ownPort {
				continue
			}
			out = append(out, scheme+"://"+addr+":"+port)
		}
	}
	return out
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
// which, since the reason would only help a guesser. Repeated bad signatures
// from one address are throttled the way the relay throttles a failed
// handshake, so a sweep from a non-member cannot be turned into a flood.
func (m *Manager) ServeDirect(w http.ResponseWriter, r *http.Request) {
	m.mu.Lock()
	k, self := m.keys, m.cfg.InstanceID
	m.mu.Unlock()
	if k == nil {
		http.NotFound(w, r)
		return
	}
	addr := relay.RequestAddr(r)
	if m.limiter.Blocked(addr) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	// The route is public, so whatever a stranger can send without the key
	// is checked before the body is read. target may also be ProbeTarget: a
	// blind probe cannot yet name the instance it is calling.
	sender, target, unix := r.Header.Get(headerPeer), r.Header.Get(headerTarget), r.Header.Get(headerTime)
	sent, err := strconv.ParseInt(unix, 10, 64)
	if err != nil || sender == "" || (target != self && target != ProbeTarget) || r.ContentLength > MaxCallBytes {
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
		m.limiter.Fail(addr)
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	m.limiter.Succeed(addr)
	var req relay.ProxyRequest
	if json.Unmarshal(body, &req) != nil || req.RequestID == "" || req.Target != target {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	call, err := relay.OpenCall(k.frameKey, req.RequestID, target, req.Sealed)
	if err != nil || !m.replay.Admit(call) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	if target == ProbeTarget {
		if call.Path != ProbePath {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		m.probeBack(sender, call.Body)
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
