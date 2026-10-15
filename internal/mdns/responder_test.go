package mdns

import (
	"net"
	"net/netip"
	"testing"

	"golang.org/x/net/ipv4"
)

// RFC 6762 section 11: a query from outside the link it came in on gets no
// answer, so the responder cannot be used to reach past the local network.
func TestOnlyASenderOnTheLinkGetsAnAnswer(t *testing.T) {
	nets := []netip.Prefix{netip.MustParsePrefix("192.168.1.0/24"), netip.MustParsePrefix("10.20.0.0/16")}
	cases := []struct {
		src  string
		want bool
	}{
		{"192.168.1.77", true},
		{"10.20.3.4", true},
		{"127.0.0.1", true},
		{"192.168.2.77", false},
		{"8.8.8.8", false},
	}
	for _, c := range cases {
		src := &net.UDPAddr{IP: net.ParseIP(c.src), Port: 40000}
		if got := onLink(src, nets); got != c.want {
			t.Errorf("onLink(%s) = %v, want %v", c.src, got, c.want)
		}
	}
	if onLink(nil, nets) {
		t.Error("a sender without an address got an answer")
	}
}

// Docker's and libvirt's bridges lead into containers and VMs on this host,
// not to the network a browser looks on. Unraid's br0 is the LAN itself.
func TestDockerAndLibvirtBridgesAreLeftOut(t *testing.T) {
	for name, want := range map[string]bool{
		"eth0": true, "br0": true, "bond0": true, "br0.20": true, "wlan0": true,
		"docker0": false, "br-4f2a9c1d0e3b": false, "virbr0": false, "vethd1c2a3b": false,
	} {
		if got := announcedOn(name); got != want {
			t.Errorf("announcedOn(%q) = %v, want %v", name, got, want)
		}
	}
}

func TestTheAddressesAHostAnnouncesAreIPv4AndNeitherLoopbackNorLinkLocal(t *testing.T) {
	for _, a := range Addresses() {
		ip, err := netip.ParseAddr(a)
		if err != nil || !ip.Is4() || ip.IsLoopback() || ip.IsLinkLocalUnicast() {
			t.Errorf("an address the host would show: %q", a)
		}
	}
}

func TestAProbingResponderGivesWayOnlyToAProbeThatWins(t *testing.T) {
	svc := Service{Instance: "BombVault", Host: "bombvault", Type: "_https._tcp", Subtype: "_bombvault", Port: 3443}
	r := &Responder{svc: svc, ifaces: []iface{
		{ifi: net.Interface{Index: 1, Name: "eth0"}, addrs: []netip.Addr{netip.MustParseAddr("192.168.1.20")}},
		{ifi: net.Interface{Index: 2, Name: "eth1"}, addrs: []netip.Addr{netip.MustParseAddr("10.0.0.20")}},
	}}
	probeFrom := func(addr string) []byte {
		t.Helper()
		z, err := r.zone([]netip.Addr{netip.MustParseAddr(addr)})
		if err != nil {
			t.Fatal(err)
		}
		msg, err := z.probe()
		if err != nil {
			t.Fatal(err)
		}
		return msg
	}
	on := func(index int) *ipv4.ControlMessage { return &ipv4.ControlMessage{IfIndex: index} }

	if !r.lostClaim(probeFrom("192.168.1.30"), on(1)) {
		t.Fatal("a probe for the same names from a later address did not win")
	}
	if r.lostClaim(probeFrom("192.168.1.10"), on(1)) {
		t.Fatal("a probe for the same names from an earlier address won")
	}
	for _, own := range []string{"192.168.1.20", "10.0.0.20"} {
		for _, index := range []int{1, 2, 0} {
			if r.lostClaim(probeFrom(own), on(index)) {
				t.Fatalf("the responder's own probe from %s, heard on interface %d, took its names", own, index)
			}
		}
	}
}
