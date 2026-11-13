package api

import (
	"bytes"
	"net"
	"net/http/httptest"
	"net/netip"
	"testing"
	"time"

	"github.com/junkerderprovinz/parleyport/kit/fortress"
	"github.com/junkerderprovinz/parleyport/kit/lan"
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
