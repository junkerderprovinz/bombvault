package api

import (
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/junkerderprovinz/parleyport/kit/relay"
)

func relayState(t *testing.T, in *instance) map[string]any {
	t.Helper()
	_, out := in.do(t, http.MethodGet, "/api/group", nil)
	state, _ := out["relay"].(map[string]any)
	return state
}

// stoppableRelay is a relay of the test's own that stop takes down along with
// the sockets it handed to the relay, which httptest leaves open.
func stoppableRelay(t *testing.T) (url string, stop func()) {
	t.Helper()
	var mu sync.Mutex
	var sockets []net.Conn
	rs := httptest.NewUnstartedServer(relay.New())
	rs.Config.ConnState = func(c net.Conn, s http.ConnState) {
		if s == http.StateHijacked {
			mu.Lock()
			sockets = append(sockets, c)
			mu.Unlock()
		}
	}
	rs.Start()
	var once sync.Once
	stop = func() {
		once.Do(func() {
			rs.Close()
			mu.Lock()
			defer mu.Unlock()
			for _, c := range sockets {
				_ = c.Close()
			}
		})
	}
	t.Cleanup(stop)
	return rs.URL, stop
}

func TestInAFortressTheRelayStateFollowsTheFortress(t *testing.T) {
	url, stop := stoppableRelay(t)
	in := newInstance(t, "cellar", strings.Repeat("a1", 32))
	if err := in.st.SetGroupRelay("own", url, false); err != nil {
		t.Fatal(err)
	}
	words, _ := newPhrase(t)
	if err := in.svc.JoinFortress(words); err != nil {
		t.Fatal(err)
	}
	waitLong(t, "the fortress to reach its relay", func() bool { return in.svc.fortressNow().RelayConnected() })
	if state := relayState(t, in); state["connected"] != true || state["fortress"] != true {
		t.Fatalf("relay = %v, want the fortress's connected relay", state)
	}

	stop()
	waitLong(t, "the fortress to lose its relay", func() bool { return !in.svc.fortressNow().RelayConnected() })
	if state := relayState(t, in); state["connected"] != false {
		t.Fatalf("relay = %v, want it down with the fortress's", state)
	}
}

func TestOutsideAFortressTheRelayStateFollowsThePairing(t *testing.T) {
	in := newInstance(t, "cellar", strings.Repeat("a1", 32))
	if state := relayState(t, in); state["connected"] != false || state["fortress"] != false {
		t.Fatalf("relay = %v, want no fortress and nothing connected", state)
	}
}
