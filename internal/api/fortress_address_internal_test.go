package api

import (
	"bytes"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/junkerderprovinz/parleyport/kit/fortress"
	"github.com/junkerderprovinz/parleyport/kit/group"
	"github.com/junkerderprovinz/parleyport/kit/lan"
	kitmember "github.com/junkerderprovinz/parleyport/kit/member"
	"github.com/junkerderprovinz/parleyport/kit/relay"
)

// listenOnTestNetwork points the fortress's announces at a group of the
// test's own and returns a listener there, or skips the test on a host that
// cannot join one.
func listenOnTestNetwork(t *testing.T) *lan.Service {
	t.Helper()
	c, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := c.LocalAddr().(*net.UDPAddr).AddrPort().Port()
	_ = c.Close()
	addr := netip.AddrPortFrom(netip.MustParseAddr("239.255.77.251"), port)
	m, err := net.ListenMulticastUDP("udp4", nil, net.UDPAddrFromAddrPort(addr))
	if err != nil {
		t.Skipf("this host cannot join a multicast group: %v", err)
	}
	_ = m.Close()
	was := fortress.Multicast
	fortress.Multicast = addr
	t.Cleanup(func() { fortress.Multicast = was })
	l := lan.New(lan.Config{Group: addr})
	l.Start()
	t.Cleanup(func() { _ = l.Close() })
	return l
}

func announcedURL(l *lan.Service) string {
	for _, p := range l.Strangers() {
		return p.URL
	}
	return ""
}

func TestAnAddressLearnedWhileInAFortressGoesOutAtOnce(t *testing.T) {
	heard := listenOnTestNetwork(t)
	svc, _ := newFortressSvc(t)
	secret := bytes.Repeat([]byte{7}, 16)
	if err := fortress.Join(svc.fortressDir(), secret, fortress.Membership{App: "bombvault", Device: "tower", Relay: "off", LAN: true}); err != nil {
		t.Fatal(err)
	}
	if err := svc.OpenFortress(nil); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(svc.CloseFortress)

	r := httptest.NewRequest("GET", "/api/status", nil)
	r.Host = "192.168.77.9:13443"
	svc.learnDirectURL(r)

	deadline := time.Now().Add(15 * time.Second)
	for announcedURL(heard) != "https://192.168.77.9:13443" {
		if time.Now().After(deadline) {
			t.Fatalf("the fortress announces %q, want the address it learned", announcedURL(heard))
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func TestBombVaultThatLeavesIsGoneFromTheOtherMembers(t *testing.T) {
	listenOnTestNetwork(t)
	secret := bytes.Repeat([]byte{7}, 16)
	ident, err := kitmember.NewIdentity(kitmember.KindInstance)
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	l := lan.New(lan.Config{Group: fortress.Multicast})
	l.Start()
	t.Cleanup(func() { _ = l.Close() })
	other, err := fortress.Open(fortress.Config{
		App: fortress.App{ID: "shiplog", Name: "ShipLog", Version: "3.8.0"}, Secret: secret, Identity: ident,
		Self:  group.Self{Name: "tower", Deployment: "plugin", DirectURL: srv.URL},
		Relay: group.RelayOff, LAN: l, Store: &memFortressStore{},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = other.Close() })
	mux.Handle(group.DirectPath, other.DirectHandler())

	svc, _ := newFortressSvc(t)
	if err := fortress.Join(svc.fortressDir(), secret, fortress.Membership{App: "bombvault", Device: "tower", Relay: "off", LAN: true}); err != nil {
		t.Fatal(err)
	}
	if err := svc.OpenFortress(nil); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(svc.CloseFortress)
	// An announce lists a member before its key is pinned, and a leave goes
	// only to the members BombVault reaches and counts only where its key is
	// pinned, so being listed is not enough.
	met := func() bool {
		return slices.ContainsFunc(other.Members(), func(m fortress.Member) bool { return m.State == group.StateApproved }) &&
			slices.ContainsFunc(svc.fortressNow().Members(), func(m fortress.Member) bool { return m.State == group.StateApproved && m.Online() })
	}
	deadline := time.Now().Add(20 * time.Second)
	for !met() {
		if time.Now().After(deadline) {
			t.Fatal("BombVault and the other member never met")
		}
		time.Sleep(50 * time.Millisecond)
	}

	if err := svc.LeaveFortress(); err != nil {
		t.Fatal(err)
	}
	if got := other.Members(); len(got) != 0 {
		t.Errorf("the other member still lists %+v after BombVault left", got)
	}
}

type memFortressStore struct{ raw []byte }

func (m *memFortressStore) Load() ([]byte, error) { return m.raw, nil }
func (m *memFortressStore) Save(raw []byte) error { m.raw = raw; return nil }

// countingRelay is a relay of the test's own that counts the connections
// made to it.
func countingRelay(t *testing.T) (url string, dials func() int) {
	t.Helper()
	var n atomic.Int32
	rs := httptest.NewUnstartedServer(relay.New())
	rs.Config.ConnState = func(_ net.Conn, s http.ConnState) {
		if s == http.StateHijacked {
			n.Add(1)
		}
	}
	rs.Start()
	t.Cleanup(rs.Close)
	return rs.URL, func() int { return int(n.Load()) }
}

func learnHost(svc *Service, host string) {
	r := httptest.NewRequest("GET", "/api/status", nil)
	r.Host = host
	svc.learnDirectURL(r)
}

func TestTwoHostsInUseKeepTheFirstDirectAddressAndTheRelayLink(t *testing.T) {
	url, dials := countingRelay(t)
	in := newInstance(t, "cellar", strings.Repeat("a1", 32))
	if err := in.st.SetGroupRelay("own", url, false); err != nil {
		t.Fatal(err)
	}
	learnHost(in.svc, "bombvault.example.org")
	words, _ := newPhrase(t)
	if err := in.svc.JoinFortress(words); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(in.svc.CloseFortress)
	waitLong(t, "the fortress to reach its relay", func() bool { return in.svc.fortressNow().RelayConnected() })
	time.Sleep(500 * time.Millisecond)
	before := dials()

	for range 5 {
		learnHost(in.svc, "bombvault.example.org")
		learnHost(in.svc, "192.168.77.9:13080")
	}

	if got := in.svc.selfDirectURL(); got != "http://bombvault.example.org" {
		t.Errorf("direct address = %q, want the one in use first", got)
	}
	time.Sleep(3 * time.Second)
	if got := dials() - before; got != 0 {
		t.Errorf("the fortress dialled its relay %d more times, want it to keep the link", got)
	}
}

func TestAnotherHostTakesOverOnceTheStoredOneWentUnused(t *testing.T) {
	was := directSettle
	directSettle = 100 * time.Millisecond
	t.Cleanup(func() { directSettle = was })
	in := newInstance(t, "cellar", strings.Repeat("a1", 32))

	learnHost(in.svc, "bombvault.example.org")
	learnHost(in.svc, "192.168.77.9:13080")
	if got := in.svc.selfDirectURL(); got != "http://bombvault.example.org" {
		t.Fatalf("direct address = %q, want the one still in use", got)
	}
	time.Sleep(2 * directSettle)
	learnHost(in.svc, "192.168.77.9:13080")
	if got := in.svc.selfDirectURL(); got != "http://192.168.77.9:13080" {
		t.Errorf("direct address = %q, want the host in use now", got)
	}
}
