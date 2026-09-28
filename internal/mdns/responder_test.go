package mdns

import (
	"net"
	"net/netip"
	"testing"
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
