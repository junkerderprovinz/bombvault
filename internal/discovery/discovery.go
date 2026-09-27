// Package discovery lets group members on one network find each other's
// address with nothing configured, so they can talk directly instead of over
// the relay.
//
// Every instance sends a small JSON announce to a fixed UDP multicast group.
// Anything on the network can send to that group, so the caller supplies an
// Accept check: a group member proves itself with a tag only members can
// compute, and every other announce is ignored.
package discovery

import (
	"encoding/json"
	"net"
	"sort"
	"sync"
	"time"

	"golang.org/x/net/ipv4"
)

// group is in the administratively scoped 239.255/16 range (RFC 2365), which
// routers do not forward beyond the local network.
const (
	group = "239.255.77.51"
	port  = 8751
)

const (
	announceEvery = 5 * time.Second
	// peerTTL keeps a peer listed through a couple of dropped datagrams.
	peerTTL   = 3*announceEvery + 2*time.Second
	readLimit = 8 << 10
	// fieldLimit and maxPeers bound what a sender that is not a member can
	// make this process hold before Accept has had its say.
	fieldLimit = 256
	maxPeers   = 256
)

// Peer is one instance's announce.
type Peer struct {
	ID      string `json:"id"`
	Name    string `json:"name,omitempty"`
	Version string `json:"version,omitempty"`
	// URL is where the announcer answers direct calls.
	URL string `json:"url"`
	// Tag proves group membership; see the package comment.
	Tag string `json:"tag,omitempty"`
	// LastSeen is set by the receiver, never sent.
	LastSeen time.Time `json:"-"`
}

// Service announces this instance and tracks the others. A host without
// multicast simply sees no peers; nothing here reports an error after Start.
type Service struct {
	mu     sync.Mutex
	self   Peer
	accept func(Peer) bool
	peers  map[string]Peer

	conn    *ipv4.PacketConn
	started bool
	closing bool

	quit chan struct{}
	done chan struct{}
	once sync.Once
}

// New builds a Service that announces nothing and accepts nobody until
// SetSelf and SetAccept are called.
func New() *Service {
	return &Service{
		peers: map[string]Peer{},
		quit:  make(chan struct{}),
		done:  make(chan struct{}),
	}
}

// SetSelf replaces what this instance announces from the next tick on. An
// empty ID or URL stops the announcing.
func (s *Service) SetSelf(self Peer) {
	s.mu.Lock()
	s.self = self
	s.mu.Unlock()
}

// SetAccept replaces the membership check and forgets every peer the old one
// admitted, so leaving a group empties the list at once. accept runs under
// the Service's lock and must not call back into it.
func (s *Service) SetAccept(accept func(Peer) bool) {
	s.mu.Lock()
	s.accept = accept
	s.peers = map[string]Peer{}
	s.mu.Unlock()
}

func (s *Service) currentSelf() Peer {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.self
}

// Start joins the multicast group and begins announcing. It never blocks.
func (s *Service) Start() {
	s.mu.Lock()
	if s.closing {
		s.mu.Unlock()
		return
	}
	s.started = true
	s.mu.Unlock()

	addr := &net.UDPAddr{IP: net.ParseIP(group), Port: port}
	c, err := net.ListenMulticastUDP("udp4", nil, addr)
	if err != nil {
		close(s.done)
		return
	}
	// The standard library joins on one interface only and turns loopback
	// off, which on a multi-homed host delivered nothing at all.
	p := ipv4.NewPacketConn(c)
	_ = p.SetMulticastLoopback(true)
	joined := 0
	for _, ifc := range multicastInterfaces() {
		if p.JoinGroup(&ifc, addr) == nil {
			joined++
		}
	}
	if joined == 0 {
		_ = p.Close()
		close(s.done)
		return
	}
	s.mu.Lock()
	if s.closing {
		s.mu.Unlock()
		_ = p.Close()
		close(s.done)
		return
	}
	s.conn = p
	s.mu.Unlock()
	go s.readLoop(p)
	go s.announceLoop(p, addr)
}

// Close stops announcing and listening. It is safe to call more than once
// and on a Service that was never started.
func (s *Service) Close() error {
	s.mu.Lock()
	conn, started := s.conn, s.started
	first := !s.closing
	s.closing = true
	s.mu.Unlock()

	s.once.Do(func() {
		close(s.quit)
		if conn != nil {
			_ = conn.Close()
		}
	})
	if started {
		<-s.done
		return nil
	}
	if first {
		close(s.done)
	}
	return nil
}

func (s *Service) listening() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.conn != nil
}

// Peers is every accepted instance seen recently, this one excluded, sorted
// by name.
func (s *Service) Peers() []Peer {
	s.mu.Lock()
	s.pruneLocked(time.Now().Add(-peerTTL))
	out := make([]Peer, 0, len(s.peers))
	for _, p := range s.peers {
		out = append(out, p)
	}
	s.mu.Unlock()
	sort.Slice(out, func(i, j int) bool {
		if out[i].Name != out[j].Name {
			return out[i].Name < out[j].Name
		}
		return out[i].ID < out[j].ID
	})
	return out
}

func (s *Service) pruneLocked(cutoff time.Time) {
	for id, p := range s.peers {
		if p.LastSeen.Before(cutoff) {
			delete(s.peers, id)
		}
	}
}

func (s *Service) announceLoop(conn *ipv4.PacketConn, addr *net.UDPAddr) {
	defer close(s.done)
	send := func() {
		self := s.currentSelf()
		if self.ID == "" || self.URL == "" {
			return
		}
		payload, err := json.Marshal(self)
		if err != nil {
			return
		}
		// Out of every interface, since on a NAS with a Docker bridge the
		// routing table can pick the wrong one.
		for _, ifc := range multicastInterfaces() {
			if conn.SetMulticastInterface(&ifc) == nil {
				_, _ = conn.WriteTo(payload, nil, addr)
			}
		}
	}
	send()
	t := time.NewTicker(announceEvery)
	defer t.Stop()
	for {
		select {
		case <-s.quit:
			return
		case <-t.C:
			send()
		}
	}
}

func (s *Service) readLoop(conn *ipv4.PacketConn) {
	buf := make([]byte, readLimit)
	for {
		n, _, _, err := conn.ReadFrom(buf)
		if err != nil {
			return
		}
		var p Peer
		if json.Unmarshal(buf[:n], &p) != nil {
			continue
		}
		s.Observe(p)
	}
}

// Observe files one announce if it passes the membership check, as if it
// had arrived over the network.
func (s *Service) Observe(p Peer) {
	if p.ID == "" || p.URL == "" || len(p.ID) > fieldLimit || len(p.URL) > fieldLimit ||
		len(p.Name) > fieldLimit || len(p.Version) > fieldLimit || len(p.Tag) > fieldLimit {
		return
	}
	p.LastSeen = time.Now()

	s.mu.Lock()
	defer s.mu.Unlock()
	if p.ID == s.self.ID || s.accept == nil || !s.accept(p) {
		return
	}
	if _, known := s.peers[p.ID]; !known && len(s.peers) >= maxPeers {
		s.pruneLocked(time.Now().Add(-peerTTL))
		if len(s.peers) >= maxPeers {
			return
		}
	}
	s.peers[p.ID] = p
}

func multicastInterfaces() []net.Interface {
	ifs, err := net.Interfaces()
	if err != nil {
		return nil
	}
	out := ifs[:0]
	for _, ifc := range ifs {
		if ifc.Flags&net.FlagUp != 0 && ifc.Flags&net.FlagMulticast != 0 {
			out = append(out, ifc)
		}
	}
	return out
}

// LocalIPv4 is the address to announce: the first IPv4 on a local interface
// that is neither loopback nor link-local, since a 169.254 address is what a
// NIC gives itself when DHCP fails and nothing else can reach it.
func LocalIPv4() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return ""
	}
	for _, a := range addrs {
		ipnet, ok := a.(*net.IPNet)
		if !ok || ipnet.IP.IsLoopback() || ipnet.IP.IsLinkLocalUnicast() {
			continue
		}
		if ip4 := ipnet.IP.To4(); ip4 != nil {
			return ip4.String()
		}
	}
	return ""
}
