package discovery

import (
	"fmt"
	"net"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

// requireNetwork skips tests that join a real multicast group. On a
// workstation that opens a firewall prompt, so they run only when asked.
func requireNetwork(t *testing.T) {
	t.Helper()
	if os.Getenv("BOMBVAULT_NET_TESTS") == "" {
		t.Skip("joins a real multicast group; set BOMBVAULT_NET_TESTS=1 to run it")
	}
}

func acceptTag(tag string) func(Peer) bool {
	return func(p Peer) bool { return p.Tag == tag }
}

func TestTwoMembersFindEachOther(t *testing.T) {
	requireNetwork(t)
	a, b := New(), New()
	a.SetSelf(Peer{ID: "id-a", Name: "Cellar", URL: "https://192.168.1.10:3443", Tag: "member"}, nil)
	b.SetSelf(Peer{ID: "id-b", Name: "Attic", URL: "https://192.168.1.11:3443", Tag: "member"}, nil)
	a.SetAccept(acceptTag("member"))
	b.SetAccept(acceptTag("member"))
	a.Start()
	b.Start()
	defer func() { _ = a.Close() }()
	defer func() { _ = b.Close() }()
	if !a.listening() || !b.listening() {
		t.Skip("this host cannot join a multicast group")
	}

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if len(only(a.Peers(), "id-b")) > 0 && len(only(b.Peers(), "id-a")) > 0 {
			break
		}
		time.Sleep(150 * time.Millisecond)
	}
	if got := only(a.Peers(), "id-b"); len(got) != 1 || got[0].Name != "Attic" {
		t.Errorf("A sees %+v, want B with its announced name", a.Peers())
	}
	if got := only(b.Peers(), "id-a"); len(got) != 1 || got[0].URL != "https://192.168.1.10:3443" {
		t.Errorf("B sees %+v, want A with its announced address", b.Peers())
	}
}

func TestAnnounceWithoutTheGroupTagIsIgnored(t *testing.T) {
	s := New()
	s.SetSelf(Peer{ID: "id-self", URL: "https://192.168.1.2:3443"}, nil)
	s.SetAccept(acceptTag("member"))
	s.Observe(Peer{ID: "id-stranger", URL: "https://192.168.1.66:3443", Tag: "guess"})
	s.Observe(Peer{ID: "id-member", URL: "https://192.168.1.3:3443", Tag: "member"})
	got := s.Peers()
	if len(got) != 1 || got[0].ID != "id-member" {
		t.Fatalf("Peers = %+v, want only the member", got)
	}
}

func TestNoAcceptMeansNoPeers(t *testing.T) {
	s := New()
	s.Observe(Peer{ID: "id-any", URL: "https://192.168.1.3:3443", Tag: "member"})
	if got := s.Peers(); len(got) != 0 {
		t.Fatalf("Peers = %+v, want none while outside a group", got)
	}
}

func TestChangingTheGroupForgetsItsPeers(t *testing.T) {
	s := New()
	s.SetAccept(acceptTag("old"))
	s.Observe(Peer{ID: "id-old", URL: "https://192.168.1.3:3443", Tag: "old"})
	s.SetAccept(acceptTag("new"))
	if got := s.Peers(); len(got) != 0 {
		t.Fatalf("Peers = %+v after the group changed, want none", got)
	}
}

func TestAnInstanceIsNotItsOwnPeer(t *testing.T) {
	s := New()
	s.SetSelf(Peer{ID: "id-self", URL: "https://192.168.1.2:3443", Tag: "member"}, nil)
	s.SetAccept(acceptTag("member"))
	s.Observe(Peer{ID: "id-self", URL: "https://192.168.1.2:3443", Tag: "member"})
	if got := s.Peers(); len(got) != 0 {
		t.Fatalf("Peers = %+v, want the looped-back announce ignored", got)
	}
}

func TestAPeerExpires(t *testing.T) {
	s := New()
	s.peers["id-gone"] = Peer{ID: "id-gone", URL: "http://x", LastSeen: time.Now().Add(-peerTTL - time.Second)}
	s.peers["id-here"] = Peer{ID: "id-here", URL: "http://y", LastSeen: time.Now()}
	got := s.Peers()
	if len(got) != 1 || got[0].ID != "id-here" {
		t.Fatalf("got %+v, want only the peer still announcing", got)
	}
	if _, still := s.peers["id-gone"]; still {
		t.Error("the expired peer is still in the map")
	}
}

func TestAFloodCannotGrowTheMapWithoutBound(t *testing.T) {
	s := New()
	s.SetAccept(func(Peer) bool { return true })
	huge := strings.Repeat("A", 4000)
	for i := 0; i < 2000; i++ {
		s.Observe(Peer{ID: fmt.Sprintf("flood-%d", i), URL: "http://x"})
		s.Observe(Peer{ID: fmt.Sprintf("huge-%d", i), URL: huge})
	}
	s.mu.Lock()
	n := len(s.peers)
	s.mu.Unlock()
	if n > maxPeers {
		t.Errorf("map holds %d entries after a flood, want at most %d", n, maxPeers)
	}
	for _, p := range s.Peers() {
		if len(p.URL) > fieldLimit {
			t.Fatalf("an oversized announce was kept: %d bytes", len(p.URL))
		}
	}
}

func TestCloseWithoutStartReturns(t *testing.T) {
	s := New()
	done := make(chan struct{})
	go func() {
		_ = s.Close()
		_ = s.Close()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("Close on a Service that was never started did not return")
	}
}

func TestCloseRacingStartLeavesNothingRunning(t *testing.T) {
	requireNetwork(t)
	for i := 0; i < 40; i++ {
		s := New()
		var wg sync.WaitGroup
		wg.Add(2)
		go func() { defer wg.Done(); s.Start() }()
		go func() { defer wg.Done(); _ = s.Close() }()
		done := make(chan struct{})
		go func() { wg.Wait(); close(done) }()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Fatalf("iteration %d: Start and Close deadlocked", i)
		}
	}
}

func only(peers []Peer, ids ...string) []Peer {
	want := map[string]bool{}
	for _, id := range ids {
		want[id] = true
	}
	var out []Peer
	for _, p := range peers {
		if want[p.ID] {
			out = append(out, p)
		}
	}
	return out
}

func TestAnOlderAnnounceDoesNotReplaceANewerOne(t *testing.T) {
	s := New()
	s.SetSelf(Peer{ID: "id-self", URL: "https://192.168.1.2:3443"}, nil)
	s.SetAccept(acceptTag("member"))
	s.Observe(Peer{ID: "id-member", URL: "https://192.168.1.3:3443", Sent: 200, Tag: "member"})
	s.Observe(Peer{ID: "id-member", URL: "https://192.168.1.99:3443", Sent: 100, Tag: "member"})
	if got := s.Peers(); len(got) != 1 || got[0].URL != "https://192.168.1.3:3443" {
		t.Fatalf("peers = %+v, want the newer address kept", got)
	}
}

func TestPrivateIPv4(t *testing.T) {
	for _, c := range []struct {
		ip   string
		want bool
	}{
		{"10.0.0.1", true},
		{"10.255.255.255", true},
		{"172.16.0.1", true},
		{"172.31.255.255", true},
		{"192.168.0.1", true},
		{"192.168.255.255", true},
		{"172.15.0.1", false},
		{"172.32.0.1", false},
		{"192.167.0.1", false},
		{"192.169.0.1", false},
		{"1.1.1.1", false},
		{"127.0.0.1", false},
		{"169.254.1.1", false},
		{"::1", false},
	} {
		got := PrivateIPv4(net.ParseIP(c.ip))
		if got != c.want {
			t.Errorf("PrivateIPv4(%s) = %v, want %v", c.ip, got, c.want)
		}
	}
}
