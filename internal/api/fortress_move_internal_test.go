package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	kitgroup "github.com/junkerderprovinz/parleyport/kit/group"
	kitmember "github.com/junkerderprovinz/parleyport/kit/member"
	"github.com/junkerderprovinz/parleyport/kit/relay"

	"github.com/junkerderprovinz/bombvault/internal/group"
)

// waitLong waits for cond as long as two instances take to find each other
// on a new phrase.
func waitLong(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func pairedForAMove(t *testing.T) (a, b *instance) {
	t.Helper()
	a = newInstance(t, "cellar", strings.Repeat("a1", 32))
	b = newInstance(t, "attic", strings.Repeat("b2", 32))
	t.Cleanup(a.svc.CloseFortress)
	t.Cleanup(b.svc.CloseFortress)
	pairThroughRelay(t, a, b)
	return a, b
}

// inPairing reports whether here's pairing in the fortress approves there
// and has reached it.
func inPairing(t *testing.T, here, there *instance) bool {
	t.Helper()
	f := here.svc.fortressNow()
	if f == nil || f.Pairing() == nil {
		return false
	}
	want := kitmember.ID(kitmember.KindInstance, there.memberKey(t))
	for _, m := range f.Pairing().Members() {
		if m.ID == want && m.State == kitgroup.StateApproved && m.Online() {
			return true
		}
	}
	return false
}

func oldPhrase(t *testing.T, in *instance) []string {
	t.Helper()
	_, out := in.do(t, http.MethodGet, "/api/group", nil)
	var names []string
	for _, n := range out["oldPhrase"].([]any) {
		names = append(names, n.(string))
	}
	return names
}

func TestJoiningTheFortressTakesThePairedInstanceAlong(t *testing.T) {
	a, b := pairedForAMove(t)
	words, _ := newPhrase(t)
	if _, out := a.do(t, http.MethodPost, "/api/fortress/join", map[string]any{"phrase": words}); out["ok"] != true {
		t.Fatalf("join = %v", out)
	}
	waitLong(t, "the paired instance to follow into the fortress", func() bool {
		got, _ := b.svc.fortressPhrase()
		return got == words
	})
	waitLong(t, "both to reach each other in the pairing of the fortress", func() bool {
		return inPairing(t, a, b) && inPairing(t, b, a)
	})
	if !isMember(a, b.id(t)) || !isMember(b, a.id(t)) {
		t.Fatalf("the members under their BombVault ids: a=%+v b=%+v", a.svc.pairing().Members(), b.svc.pairing().Members())
	}
	if w := a.svc.pairing().Waiting(); len(w) != 0 {
		t.Fatalf("somebody waits to be let in again: %+v", w)
	}
	if _, out := a.do(t, http.MethodGet, "/api/group", nil); out["fortress"] != true {
		t.Fatalf("the group does not say it runs on the fortress phrase: %v", out)
	}
}

func TestAnInstanceAwayDuringTheMoveFollowsOnceItIsBack(t *testing.T) {
	a, b := pairedForAMove(t)
	b.svc.pairing().Close()
	words, _ := newPhrase(t)
	if _, out := a.do(t, http.MethodPost, "/api/fortress/join", map[string]any{"phrase": words}); out["ok"] != true {
		t.Fatalf("join = %v", out)
	}
	if got := oldPhrase(t, a); !slices.Equal(got, []string{"attic"}) {
		t.Fatalf("still on the old phrase: %v, want attic", got)
	}

	b.svc.applyGroup()
	waitLong(t, "the instance that was away to follow", func() bool {
		got, _ := b.svc.fortressPhrase()
		return got == words
	})
	waitLong(t, "the old group to end once everybody moved", func() bool {
		g, err := a.st.GetGroupState()
		return err == nil && len(g.SecretEnc) == 0 && len(oldPhrase(t, a)) == 0
	})
	waitLong(t, "both to reach each other in the pairing of the fortress", func() bool {
		return inPairing(t, a, b) && inPairing(t, b, a)
	})
}

func TestAPhoneTakesTheKeysOfThePairingInTheFortressOverTheOldGroup(t *testing.T) {
	a := newInstance(t, "cellar", strings.Repeat("a1", 32))
	t.Cleanup(a.svc.CloseFortress)
	rs := httptest.NewServer(relay.New())
	t.Cleanup(rs.Close)
	a.do(t, http.MethodPut, "/api/group/relay", map[string]any{"mode": "own", "url": rs.URL})
	_, created := a.do(t, http.MethodPost, "/api/group/phrase", nil)
	old, _ := created["phrase"].(string)
	p := joinPhone(t, rs.URL, old, a)

	if status, _ := p.get(a, movedPath); status == http.StatusOK {
		t.Fatal("the route answers before the group moved")
	}
	words, sec := newPhrase(t)
	if _, out := a.do(t, http.MethodPost, "/api/fortress/join", map[string]any{"phrase": words}); out["ok"] != true {
		t.Fatalf("join = %v", out)
	}
	if got := oldPhrase(t, a); !slices.Equal(got, []string{"pixel"}) {
		t.Fatalf("still on the old phrase: %v, want the phone", got)
	}
	if status, _ := a.svc.callPeer(context.Background(), peerCall{Method: http.MethodGet, Path: movedPath, Sender: "someone", Kind: ""}); status == http.StatusOK {
		t.Fatal("an instance got the keys, which it takes with the phrase instead")
	}

	status, raw := p.get(a, movedPath)
	var keys struct {
		Group string `json:"group"`
		Frame []byte `json:"frame"`
	}
	if status != http.StatusOK || json.Unmarshal(raw, &keys) != nil {
		t.Fatalf("moved = %d %s", status, raw)
	}
	if keys.Group != group.Domain.GroupKey(sec) || !bytes.Equal(keys.Frame, group.Domain.FrameKey(sec)) {
		t.Fatalf("the phone got %s, not the keys of the pairing under the fortress phrase", raw)
	}
	if bytes.Contains(raw, []byte(strings.Fields(words)[0]+" ")) || bytes.Contains(raw, sec) {
		t.Fatal("the answer carries the fortress phrase")
	}

	p.reconnect(keys.Group, keys.Frame)
	waitLong(t, "the phone to reach the instance with the new keys", func() bool {
		status, _ := p.get(a, group.ProbePath)
		return status == http.StatusOK
	})
	waitLong(t, "the old group to end once the phone moved", func() bool {
		g, err := a.st.GetGroupState()
		return err == nil && len(g.SecretEnc) == 0
	})
}

// get sends in a GET in the phone's own name over the relay and returns the
// status and body, 0 while in does not answer.
func (p *phone) get(in *instance, path string) (int, []byte) {
	p.t.Helper()
	to := kitmember.ID(kitmember.KindInstance, in.memberKey(p.t))
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, status, err := p.client.Proxy(ctx, to, http.MethodGet, path, nil)
	if err != nil {
		return 0, nil
	}
	return status, out
}

// reconnect takes the phone onto the relay under other keys, as the app does
// once it moved.
func (p *phone) reconnect(groupKey string, frame []byte) {
	p.t.Helper()
	old := p.client
	c, err := relay.NewClient(relay.ClientOptions{
		URL: p.relayURL, Key: groupKey, FrameKey: frame, Signing: group.Domain.Signing(),
		Self:     relay.Announce{InstanceID: p.ident.ID(), Name: "pixel", Deployment: string(kitmember.KindMobile), Client: true, Member: p.ident.ID()},
		Identity: &p.ident,
	})
	if err != nil {
		p.t.Fatal(err)
	}
	c.Start()
	p.t.Cleanup(func() { _ = c.Close() })
	_ = old.Close()
	p.client = c
}

func TestTheFortressPageNamesTheInstancesThePairingTakesAlong(t *testing.T) {
	a, _ := pairedForAMove(t)
	_, out := a.do(t, http.MethodGet, "/api/fortress", nil)
	names, _ := out["pairedWith"].([]any)
	if len(names) != 1 || names[0] != "attic" {
		t.Fatalf("pairedWith = %v, want attic", out["pairedWith"])
	}
}
