package nodelink

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/group"
	kitgroup "github.com/junkerderprovinz/parleyport/kit/group"
	"github.com/junkerderprovinz/parleyport/kit/member"
	"github.com/junkerderprovinz/parleyport/kit/relay"
)

var homeSeed = bytes.Repeat([]byte{7}, 32)

func homeKey(t *testing.T) []byte {
	t.Helper()
	id, err := member.FromSeed(member.KindInstance, homeSeed)
	if err != nil {
		t.Fatal(err)
	}
	return id.Public()
}

func startRelay(t *testing.T) (string, *relay.Server) {
	t.Helper()
	rs := relay.New()
	srv := httptest.NewServer(rs)
	t.Cleanup(srv.Close)
	t.Cleanup(srv.CloseClientConnections)
	return srv.URL, rs
}

// echo answers every call with the end's name and what the call carried.
func echo(name string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.NewEncoder(w).Encode(name + " " + r.Method + " " + r.URL.Path + " " + string(body))
	})
}

// pair opens both ends of the link for the node with appKey.
func pair(t *testing.T, relayURL, appKey string) (home, node *Link) {
	t.Helper()
	var err error
	node, err = Open(Config{NodeKey: appKey, Home: homeKey(t), Name: "node", Relay: group.ModeOwn, RelayURL: relayURL})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = node.Close() })
	node.Serve(echo("node"))
	home, err = Open(Config{NodeKey: appKey, Seed: homeSeed, Name: "home", Relay: group.ModeOwn, RelayURL: relayURL})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = home.Close() })
	home.Serve(echo("home"))
	return home, node
}

// answered calls l at path until the other end is on the relay.
func answered(t *testing.T, l *Link, path string) (int, string) {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for {
		status, body, err := l.Call(context.Background(), path, []byte("ping"))
		if err == nil {
			return status, strings.TrimSpace(string(body))
		}
		if time.Now().After(deadline) {
			t.Fatalf("no answer at %s: %v", path, err)
		}
		time.Sleep(25 * time.Millisecond)
	}
}

func TestHomeAndNodeCallEachOtherOverTheRelay(t *testing.T) {
	url, _ := startRelay(t)
	home, node := pair(t, url, strings.Repeat("a", 64))

	if status, body := answered(t, home, "/hello"); status != http.StatusOK || body != `"node POST /hello ping"` {
		t.Errorf("home calls the node = %d %q", status, body)
	}
	if status, body := answered(t, node, "/back"); status != http.StatusOK || body != `"home POST /back ping"` {
		t.Errorf("the node calls home = %d %q", status, body)
	}
}

func TestALinkServesNothingBeforeItHasAHandler(t *testing.T) {
	url, _ := startRelay(t)
	appKey := strings.Repeat("b", 64)
	node, err := Open(Config{NodeKey: appKey, Home: homeKey(t), Relay: group.ModeOwn, RelayURL: url})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = node.Close() })
	home, err := Open(Config{NodeKey: appKey, Seed: homeSeed, Relay: group.ModeOwn, RelayURL: url})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = home.Close() })

	if status, _ := answered(t, home, "/hello"); status != http.StatusServiceUnavailable {
		t.Errorf("a node without a handler answers %d, want 503", status)
	}
}

func TestALinkRefusesAThirdKeyThatHoldsItsSecret(t *testing.T) {
	url, _ := startRelay(t)
	appKey := strings.Repeat("c", 64)
	home, node := pair(t, url, appKey)
	answered(t, home, "/hello")

	secret, err := linkSecret(appKey)
	if err != nil {
		t.Fatal(err)
	}
	stranger, err := member.FromSeed(member.KindInstance, bytes.Repeat([]byte{9}, 32))
	if err != nil {
		t.Fatal(err)
	}
	g, err := kitgroup.Open(kitgroup.Config{
		Domain: group.Domain, Secret: secret, Identity: stranger, JoinByPhrase: true,
		Relay: kitgroup.RelayOwn, RelayURL: url, Store: &memStore{},
	}, echo("stranger"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = g.Close() })

	for name, target := range map[string]string{"node": node.peer, "home": home.peer} {
		var status int
		var body []byte
		deadline := time.Now().Add(5 * time.Second)
		for status == 0 && time.Now().Before(deadline) {
			status, body, _ = g.Call(context.Background(), target, http.MethodPost, "/hello", nil)
			time.Sleep(25 * time.Millisecond)
		}
		if status != http.StatusForbidden {
			t.Errorf("a stranger calls the %s's end = %d %s, want 403", name, status, body)
		}
	}
}

func TestClosingOneLinkLeavesTheOthersAndTheMainGroupUp(t *testing.T) {
	url, rs := startRelay(t)
	m := group.NewManager(http.NotFoundHandler(), &memDir{})
	t.Cleanup(m.Close)
	m.Apply(group.Config{Secret: []byte("0123456789abcdef"), Seed: homeSeed, InstanceID: "home", Mode: group.ModeOwn, RelayURL: url, Store: &memStore{}})

	var homes []*Link
	for _, k := range []string{"1", "2", "3"} {
		home, _ := pair(t, url, strings.Repeat(k, 64))
		answered(t, home, "/hello")
		homes = append(homes, home)
	}
	waitFor(t, "the main group on the relay", m.RelayConnected)
	if n := rs.Len(); n != 7 {
		t.Errorf("the relay holds %d connections, want the main group and both ends of three links", n)
	}

	if err := homes[1].Close(); err != nil {
		t.Fatal(err)
	}
	if _, _, err := homes[1].Call(context.Background(), "/hello", nil); err == nil {
		t.Error("a closed link still calls")
	}
	for _, i := range []int{0, 2} {
		if status, _, err := homes[i].Call(context.Background(), "/hello", nil); err != nil || status != http.StatusOK {
			t.Errorf("link %d after another closed = %d %v", i, status, err)
		}
	}
	if !m.RelayConnected() {
		t.Error("the main group left the relay as a link closed")
	}
}

func TestOpenRefusesAConfigWithoutOneEnd(t *testing.T) {
	appKey := strings.Repeat("d", 64)
	for name, cfg := range map[string]Config{
		"neither end":   {NodeKey: appKey},
		"both ends":     {NodeKey: appKey, Seed: homeSeed, Home: homeKey(t)},
		"a bad APP_KEY": {NodeKey: "zz", Seed: homeSeed},
	} {
		if l, err := Open(cfg); err == nil {
			_ = l.Close()
			t.Errorf("%s: opened", name)
		}
	}
}

type memDir struct{}

func (memDir) MemberByKey([]byte) (string, bool, error) { return "", false, nil }
func (memDir) MemberKey(string) ([]byte, bool, error)   { return nil, false, nil }
func (memDir) PinMemberKey(string, []byte) error        { return nil }

func waitFor(t *testing.T, what string, ok func() bool) {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for !ok() {
		if time.Now().After(deadline) {
			t.Fatalf("never: %s", what)
		}
		time.Sleep(25 * time.Millisecond)
	}
}

func TestALinkSaysWhetherTheOtherEndIsThere(t *testing.T) {
	url, _ := startRelay(t)
	appKey := strings.Repeat("9", 64)
	home, err := Open(Config{NodeKey: appKey, Seed: homeSeed, Relay: group.ModeOwn, RelayURL: url})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = home.Close() })
	if home.Online() {
		t.Error("home sees a node that never came")
	}
	node, err := Open(Config{NodeKey: appKey, Home: homeKey(t), Relay: group.ModeOwn, RelayURL: url})
	if err != nil {
		t.Fatal(err)
	}
	waitFor(t, "home to see the node", home.Online)
	if err := node.Close(); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "home to see the node gone", func() bool { return !home.Online() })
}
