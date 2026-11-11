package api

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/junkerderprovinz/parleyport/kit/fortress"
	kitgroup "github.com/junkerderprovinz/parleyport/kit/group"
	"github.com/junkerderprovinz/parleyport/kit/lan"
	kitmember "github.com/junkerderprovinz/parleyport/kit/member"
	"github.com/junkerderprovinz/parleyport/kit/phrase"
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

// phoneOnAGroup starts a group on a single instance that reaches its own
// relay and pairs a phone with it. It returns the group's words.
func phoneOnAGroup(t *testing.T) (*instance, *phone, string) {
	t.Helper()
	a := newInstance(t, "cellar", strings.Repeat("a1", 32))
	t.Cleanup(a.svc.CloseFortress)
	rs := httptest.NewServer(relay.New())
	t.Cleanup(rs.Close)
	a.do(t, http.MethodPut, "/api/group/relay", map[string]any{"mode": "own", "url": rs.URL})
	_, created := a.do(t, http.MethodPost, "/api/group/phrase", nil)
	old, _ := created["phrase"].(string)
	return a, joinPhone(t, rs.URL, old, a), old
}

// movedPhone pairs a phone with a single instance and moves the instance
// into a fortress. It returns the fortress secret.
func movedPhone(t *testing.T) (*instance, *phone, []byte) {
	t.Helper()
	a, p, _ := phoneOnAGroup(t)
	if _, err := fortress.MovedKeys(context.Background(), p.client, p.target(a)); err == nil {
		t.Fatal("the keys are handed out before the group moved")
	}
	words, sec := newPhrase(t)
	if _, out := a.do(t, http.MethodPost, "/api/fortress/join", map[string]any{"phrase": words}); out["ok"] != true {
		t.Fatalf("join = %v", out)
	}
	if got := oldPhrase(t, a); !slices.Equal(got, []string{"pixel"}) {
		t.Fatalf("still on the old phrase: %v, want the phone", got)
	}
	return a, p, sec
}

// follow takes p onto the pairing in the fortress with the keys a hands
// out, and waits until a answers it there.
func (p *phone) follow(a *instance) {
	p.t.Helper()
	got, err := fortress.MovedKeys(context.Background(), p.client, p.target(a))
	if err != nil {
		p.t.Fatalf("moved keys: %v", err)
	}
	p.reconnect(got.Group, got.Frame)
	waitLong(p.t, "the phone to reach the instance with the new keys", func() bool {
		status, _ := p.get(a, group.ProbePath)
		return status == http.StatusOK
	})
}

func TestAPhoneTakesTheKeysOfThePairingInTheFortressOverTheOldGroup(t *testing.T) {
	a, p, sec := movedPhone(t)
	got, err := fortress.MovedKeys(context.Background(), p.client, p.target(a))
	if err != nil {
		t.Fatalf("moved keys: %v", err)
	}
	want := group.Domain.Keys(sec)
	if got.Group != want.Group || !bytes.Equal(got.Frame, want.Frame) || !bytes.Equal(got.PeerAuth, want.PeerAuth) {
		t.Fatal("the phone did not get the keys of the pairing under the fortress phrase")
	}

	p.follow(a)
	waitLong(t, "the old group to end once the phone moved", func() bool {
		g, err := a.st.GetGroupState()
		return err == nil && len(g.SecretEnc) == 0
	})
}

func TestAPhoneOfAnEarlierReleaseStillFindsTheKeysWhereItAsks(t *testing.T) {
	a, p, sec := movedPhone(t)
	status, raw := p.get(a, "/api/group/peer/moved")
	var keys struct {
		OK    bool   `json:"ok"`
		Group string `json:"group"`
		Frame []byte `json:"frame"`
	}
	if status != http.StatusOK || json.Unmarshal(raw, &keys) != nil || !keys.OK {
		t.Fatalf("moved = %d %s", status, raw)
	}
	if keys.Group != group.Domain.GroupKey(sec) || !bytes.Equal(keys.Frame, group.Domain.FrameKey(sec)) {
		t.Fatalf("the phone got %s, not the keys of the pairing under the fortress phrase", raw)
	}
	if bytes.Contains(raw, sec) {
		t.Fatal("the answer carries the fortress secret")
	}
}

func TestAMovedPhoneIsNotWaitedForAgainAfterARestart(t *testing.T) {
	a, p, old := phoneOnAGroup(t)
	away := joinPhone(t, p.relayURL, old, a)
	_ = away.client.Close()
	words, _ := newPhrase(t)
	if _, out := a.do(t, http.MethodPost, "/api/fortress/join", map[string]any{"phrase": words}); out["ok"] != true {
		t.Fatalf("join = %v", out)
	}
	p.follow(a)
	waitLong(t, "the moved phone to drop off the old phrase", func() bool {
		return len(oldPhrase(t, a)) == 1
	})
	_ = p.client.Close()

	a.svc.reopenFortress()
	if got := oldPhrase(t, a); len(got) != 1 {
		t.Fatalf("still on the old phrase after a restart: %v, want only the phone that was away", got)
	}
}

func TestAMemberStillOnTheOldPhraseReachesTheInstanceDirectly(t *testing.T) {
	a := newInstance(t, "cellar", strings.Repeat("a1", 32))
	t.Cleanup(a.svc.CloseFortress)
	srv := httptest.NewServer(a.router)
	t.Cleanup(srv.Close)
	_, created := a.do(t, http.MethodPost, "/api/group/phrase", nil)
	old, err := phrase.Decode(created["phrase"].(string))
	if err != nil {
		t.Fatal(err)
	}

	seed := make([]byte, 32)
	_, _ = rand.Read(seed)
	ident, err := kitmember.FromSeed(kitmember.KindInstance, seed)
	if err != nil {
		t.Fatal(err)
	}
	nearby := lan.New(lan.Config{})
	b, err := kitgroup.Open(kitgroup.Config{
		Domain: group.Domain, Secret: old, Identity: ident, Self: kitgroup.Self{Name: "attic", Deployment: "container"},
		Relay: kitgroup.RelayOff, LAN: nearby, Store: &heldState{}, Kinds: group.Kinds, JoinByPhrase: true,
	}, http.NotFoundHandler())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = b.Close() })
	aID := kitmember.ID(kitmember.KindInstance, a.memberKey(t))
	probe := func() bool {
		nearby.Observe(announcedUnder(old, lan.Peer{ID: aID, Name: "cellar", URL: srv.URL, Deployment: "container"}))
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		status, _, err := b.Call(ctx, aID, http.MethodGet, group.ProbePath, nil)
		return err == nil && status == http.StatusOK
	}
	waitLong(t, "the instance to list the other one as waiting", func() bool {
		probe()
		return waitsOn(a, ident.ID())
	})
	if code, out := a.do(t, http.MethodPost, "/api/group/members/"+ident.ID()+"/approve", nil); code != http.StatusOK || out["ok"] != true {
		t.Fatalf("approve: %d %v", code, out)
	}
	waitLong(t, "the other instance to be answered directly", probe)

	words, _ := newPhrase(t)
	if _, out := a.do(t, http.MethodPost, "/api/fortress/join", map[string]any{"phrase": words}); out["ok"] != true {
		t.Fatalf("join = %v", out)
	}
	if got := oldPhrase(t, a); !slices.Equal(got, []string{ident.ID()}) {
		t.Fatalf("still on the old phrase: %v, want the other instance", got)
	}
	if !probe() {
		t.Fatal("a direct call on the old phrase is refused once the group moved")
	}
}

func TestThePairingStaysOnTheNetworkOnceTheOldGroupEnds(t *testing.T) {
	a := newInstance(t, "cellar", strings.Repeat("a1", 32))
	t.Cleanup(a.svc.CloseFortress)
	a.do(t, http.MethodPost, "/api/group/phrase", nil)
	words, sec := newPhrase(t)
	if _, out := a.do(t, http.MethodPost, "/api/fortress/join", map[string]any{"phrase": words}); out["ok"] != true {
		t.Fatalf("join = %v", out)
	}
	waitLong(t, "the old group to end with nobody left on it", func() bool {
		g, err := a.st.GetGroupState()
		return err == nil && len(g.SecretEnc) == 0 && a.svc.pairing().Following() && a.svc.pairing().Former() == nil
	})

	nearby := a.svc.pairing().LAN()
	self := kitmember.ID(kitmember.KindInstance, a.memberKey(t))
	nearby.Observe(lan.Peer{ID: self, Name: "cellar", URL: "http://192.168.1.2:3000", Deployment: "container"})
	for _, p := range nearby.Strangers() {
		if p.ID == self {
			t.Fatal("the pairing does not announce this instance on the network")
		}
	}
	other := kitmember.ID(kitmember.KindInstance, bytes.Repeat([]byte{7}, 32))
	nearby.Observe(announcedUnder(sec, lan.Peer{ID: other, Name: "attic", URL: "http://192.168.1.3:3000", Deployment: "container"}))
	if m := nearby.Members(); len(m) != 1 || m[0].ID != other {
		t.Fatalf("the pairing does not take a member's announce on the network: %+v", m)
	}
}

// announcedUnder tags p the way a member of the group on sec announces
// itself on the network.
func announcedUnder(sec []byte, p lan.Peer) lan.Peer {
	p.Sent = time.Now().Unix()
	mac := hmac.New(sha256.New, group.Domain.Keys(sec).PeerAuth)
	mac.Write([]byte("announce\x00" + p.ID + "\x00" + p.URL + "\x00" + p.Name + "\x00" + p.Deployment + "\x00" + p.Address + "\x00" + strconv.FormatInt(p.Sent, 10)))
	p.Tag = hex.EncodeToString(mac.Sum(nil))
	return p
}

// heldState keeps a kit group's state in memory.
type heldState struct {
	mu  sync.Mutex
	raw []byte
}

func (h *heldState) Load() ([]byte, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	return bytes.Clone(h.raw), nil
}

func (h *heldState) Save(raw []byte) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.raw = bytes.Clone(raw)
	return nil
}

// target is in's id on the relay.
func (p *phone) target(in *instance) string {
	return kitmember.ID(kitmember.KindInstance, in.memberKey(p.t))
}

// get sends in a GET in the phone's own name over the relay and returns the
// status and body, 0 while in does not answer.
func (p *phone) get(in *instance, path string) (int, []byte) {
	p.t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, status, err := p.client.Proxy(ctx, p.target(in), http.MethodGet, path, nil)
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

func TestAPhraseChangeNamesAnInstanceOfThePairingTheFortressHasNotMet(t *testing.T) {
	a, b := pairedForAMove(t)
	b.svc.pairing().Close()
	words, _ := newPhrase(t)
	if _, out := a.do(t, http.MethodPost, "/api/fortress/join", map[string]any{"phrase": words}); out["ok"] != true {
		t.Fatalf("join = %v", out)
	}
	_, out := a.do(t, http.MethodPost, "/api/fortress/phrase", map[string]any{"password": testPassword})
	if out["ok"] != true {
		t.Fatalf("change = %v", out)
	}
	if left, _ := out["left"].([]any); len(left) != 1 || left[0] != "attic" {
		t.Fatalf("left on the old phrase = %v, want the instance still on its way in", out["left"])
	}
}
