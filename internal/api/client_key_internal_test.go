package api

import (
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
)

func mustCIDR(t *testing.T, s string) net.IPNet {
	t.Helper()
	_, n, err := net.ParseCIDR(s)
	if err != nil {
		t.Fatalf("ParseCIDR(%q): %v", s, err)
	}
	return *n
}

// Believing a forwarded-for header by default would let any caller pick their
// own throttle bucket.
func TestLoginKeyIgnoresForwardedHeaderByDefault(t *testing.T) {
	h, _, _ := newAuthGateHandler(t)
	r := httptest.NewRequest(http.MethodPost, "/api/login", nil)
	r.RemoteAddr = "203.0.113.9:5555"
	r.Header.Set("X-Forwarded-For", "10.9.9.9")
	if got := h.loginClientKey(r); got != "203.0.113.9" {
		t.Fatalf("key = %q, want the real peer 203.0.113.9", got)
	}
}

func TestLoginKeyUsesForwardedClientBehindATrustedProxy(t *testing.T) {
	h, _, _ := newAuthGateHandler(t)
	h.cfg.TrustedProxies = []net.IPNet{mustCIDR(t, "192.168.20.0/24")}
	r := httptest.NewRequest(http.MethodPost, "/api/login", nil)
	r.RemoteAddr = "192.168.20.11:44321"
	r.Header.Set("X-Forwarded-For", "203.0.113.7")
	if got := h.loginClientKey(r); got != "203.0.113.7" {
		t.Fatalf("key = %q, want the client the proxy named", got)
	}
}

// Somebody reaching the port directly must not inherit the proxy's trust by
// sending the same header.
func TestForwardedHeaderIsIgnoredFromAnUntrustedPeer(t *testing.T) {
	h, _, _ := newAuthGateHandler(t)
	h.cfg.TrustedProxies = []net.IPNet{mustCIDR(t, "192.168.20.0/24")}
	r := httptest.NewRequest(http.MethodPost, "/api/login", nil)
	r.RemoteAddr = "203.0.113.9:5555"
	r.Header.Set("X-Forwarded-For", "10.0.0.1")
	if got := h.loginClientKey(r); got != "203.0.113.9" {
		t.Fatalf("key = %q, want the untrusted peer's own address", got)
	}
}

// An attacker controls the left end of the chain, so reading it from the left
// would hand them a fresh bucket per request.
func TestForwardedChainIsReadFromTheRight(t *testing.T) {
	proxies := []net.IPNet{mustCIDR(t, "192.168.20.0/24"), mustCIDR(t, "10.0.0.0/8")}
	cases := []struct{ header, want string }{
		{"203.0.113.7", "203.0.113.7"},
		{"1.2.3.4, 203.0.113.7", "203.0.113.7"},
		{"203.0.113.7, 10.0.0.5", "203.0.113.7"},
		{"1.2.3.4, 203.0.113.7, 10.0.0.5, 192.168.20.11", "203.0.113.7"},
		// Every entry trusted: nothing left to name a client.
		{"10.0.0.5, 192.168.20.11", ""},
		{"", ""},
		// One bad entry refuses the whole header, so an attacker cannot steer
		// which entry is picked by inserting one.
		{"1.2.3.4, nonsense, 10.0.0.5", ""},
	}
	for _, c := range cases {
		if got := forwardedClient(c.header, proxies); got != c.want {
			t.Fatalf("forwardedClient(%q) = %q, want %q", c.header, got, c.want)
		}
	}
}

// Two clients behind one proxy must not share a bucket, or an attacker's
