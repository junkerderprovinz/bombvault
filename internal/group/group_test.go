package group

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
	kitgroup "github.com/junkerderprovinz/parleyport/kit/group"
	"github.com/junkerderprovinz/parleyport/kit/member"
	"github.com/junkerderprovinz/parleyport/kit/relay"
)

var secret = []byte("0123456789abcdef")

// memDir is a Directory in memory.
type memDir struct {
	mu   sync.Mutex
	keys map[string][]byte
}

func (d *memDir) MemberByKey(key []byte) (string, bool, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	for id, k := range d.keys {
		if bytes.Equal(k, key) {
			return id, true, nil
		}
	}
	return "", false, nil
}

func (d *memDir) MemberKey(id string) ([]byte, bool, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	k, ok := d.keys[id]
	return k, ok, nil
}

func (d *memDir) PinMemberKey(id string, key []byte) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if k, ok := d.keys[id]; ok && !bytes.Equal(k, key) {
		return errors.New("another key")
	}
	d.keys[id] = key
	return nil
}

type memStore struct {
	mu  sync.Mutex
	raw []byte
}

func (s *memStore) Load() ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return bytes.Clone(s.raw), nil
}

func (s *memStore) Save(b []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.raw = bytes.Clone(b)
	return nil
}

// node is one BombVault in a test: its id, member key, directory and group.
type node struct {
	id      string
	seed    []byte
	dir     *memDir
	store   *memStore
	m       *Manager
	removed chan struct{}
}

func newNode(t *testing.T, id string, seedByte byte) *node {
	t.Helper()
	n := &node{id: id, seed: bytes.Repeat([]byte{seedByte}, 32), dir: &memDir{keys: map[string][]byte{}}, store: &memStore{}, removed: make(chan struct{}, 1)}
	mux := http.NewServeMux()
	mux.HandleFunc("GET "+ProbePath, func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(Hello{OK: true, InstanceID: n.id, Name: n.id})
	})
	mux.HandleFunc("GET /who", func(w http.ResponseWriter, r *http.Request) {
		s, _ := SenderOf(r.Context())
		_ = json.NewEncoder(w).Encode(s)
	})
	n.m = NewManager(mux, n.dir)
	t.Cleanup(n.m.Close)
	return n
}

func (n *node) key() []byte {
	id, _ := member.FromSeed(member.KindInstance, n.seed)
	return id.Public()
}

func (n *node) apply(relayURL string, former func() (kitgroup.Roster, error)) {
	n.m.Apply(Config{
		Secret: secret, Seed: n.seed, InstanceID: n.id, Name: n.id, Mode: ModeOwn, RelayURL: relayURL,
		Store: n.store, Former: former, OnRemoved: func() { n.removed <- struct{}{} },
	})
}

func startRelay(t *testing.T) string {
	t.Helper()
	srv := httptest.NewServer(relay.New())
	t.Cleanup(srv.Close)
	return srv.URL
}

func eventually(t *testing.T, what string, ok func() bool) {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		if ok() {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("never: %s", what)
}

func listed(m *Manager, id string) bool {
	for _, mem := range m.Members() {
		if mem.ID == id {
			return true
		}
	}
	return false
}

func whoCalls(t *testing.T, from *node, to string) Sender {
	t.Helper()
	status, body, err := from.m.Call(context.Background(), to, http.MethodGet, "/who", nil)
	if err != nil || status != http.StatusOK {
		t.Fatalf("call to %s: %d %s %v", to, status, body, err)
	}
	var s Sender
	if err := json.Unmarshal(body, &s); err != nil {
		t.Fatal(err)
	}
	return s
}

// formerOf is the roster an instance that pinned these keys before takes in.
func formerOf(nodes ...*node) func() (kitgroup.Roster, error) {
	return func() (kitgroup.Roster, error) {
		var r kitgroup.Roster
		for _, n := range nodes {
			k := n.key()
			r.Members = append(r.Members, kitgroup.Member{ID: member.ID(member.KindInstance, k), Kind: member.KindInstance, Key: k, State: kitgroup.StateApproved, Name: n.id})
		}
		return r, nil
	}
}

func TestMembersKnownBeforeKeepTheirIdsAndNeedNoApproval(t *testing.T) {
	url := startRelay(t)
	a := newNode(t, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", 1)
	b := newNode(t, "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", 2)
	a.dir.keys[b.id] = b.key()
	b.dir.keys[a.id] = a.key()
	a.apply(url, formerOf(b))
	b.apply(url, formerOf(a))

	eventually(t, "both list each other under their ids", func() bool { return listed(a.m, b.id) && listed(b.m, a.id) })
	if s := whoCalls(t, a, b.id); s.ID != a.id || s.Kind != "" {
		t.Fatalf("b saw the call come from %+v, want a's own id", s)
	}
	if w := a.m.Waiting(); len(w) != 0 {
		t.Fatalf("waiting = %+v, want nobody", w)
	}
}

func TestANewInstanceWaitsAndIsKnownByItsIdOnceApproved(t *testing.T) {
	url := startRelay(t)
	a := newNode(t, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", 1)
	b := newNode(t, "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", 2)
	a.apply(url, nil)
	b.apply(url, nil)

	kitB := member.ID(member.KindInstance, b.key())
	kitA := member.ID(member.KindInstance, a.key())
	waits := func(n *node, id string) bool {
		for _, w := range n.m.Waiting() {
			if w.ID == id && !w.There && w.Fingerprint != "" {
				return true
			}
		}
		return false
	}
	eventually(t, "each waits on the other", func() bool { return waits(a, kitB) && waits(b, kitA) })
	if listed(a.m, kitB) || listed(a.m, b.id) {
		t.Fatal("a lists b before anybody approved it")
	}
	if err := a.m.Approve(kitB); err != nil {
		t.Fatal(err)
	}
	if err := b.m.Approve(kitA); err != nil {
		t.Fatal(err)
	}
	eventually(t, "both list each other under their ids", func() bool { return listed(a.m, b.id) && listed(b.m, a.id) })
	if k, ok, _ := a.dir.MemberKey(b.id); !ok || !bytes.Equal(k, b.key()) {
		t.Fatal("a did not pin b's key under b's id")
	}
	if s := whoCalls(t, b, a.id); s.ID != b.id {
		t.Fatalf("a saw the call come from %+v", s)
	}
}

func TestAnIdPinnedForAnotherKeyIsNotTaken(t *testing.T) {
	url := startRelay(t)
	a := newNode(t, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", 1)
	b := newNode(t, "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", 2)
	a.dir.keys[b.id] = bytes.Repeat([]byte{9}, 32)
	a.apply(url, nil)
	b.apply(url, nil)
	kitB := member.ID(member.KindInstance, b.key())
	kitA := member.ID(member.KindInstance, a.key())
	eventually(t, "b waits on a", func() bool { return len(a.m.Waiting()) > 0 && len(b.m.Waiting()) > 0 })
	if err := a.m.Approve(kitB); err != nil {
		t.Fatal(err)
	}
	if err := b.m.Approve(kitA); err != nil {
		t.Fatal(err)
	}
	time.Sleep(time.Second)
	if listed(a.m, b.id) || listed(a.m, kitB) {
		t.Fatalf("a lists b under an id pinned for another key: %+v", a.m.Members())
	}
}

func TestAPhoneKeepsTheIdItsKeyWasPinnedUnder(t *testing.T) {
	url := startRelay(t)
	a := newNode(t, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", 1)
	phone, _ := member.FromSeed(member.KindMobile, bytes.Repeat([]byte{5}, 32))
	const old = "cccccccccccccccccccccccccccccccc"
	a.dir.keys[old] = phone.Public()
	a.apply(url, func() (kitgroup.Roster, error) {
		return kitgroup.Roster{Members: []kitgroup.Member{{ID: phone.ID(), Kind: member.KindMobile, Key: phone.Public(), State: kitgroup.StateApproved}}}, nil
	})

	c := phoneOn(t, url, phone)
	eventually(t, "a lists the phone under its old id", func() bool {
		for _, m := range a.m.Members() {
			if m.ID == old && m.Kind == KindPhone {
				return true
			}
		}
		return false
	})
	eventually(t, "the phone sees a", func() bool { return len(c.Siblings()) > 0 })
	out, status, err := c.Proxy(context.Background(), member.ID(member.KindInstance, a.key()), http.MethodGet, "/who", nil)
	var s Sender
	if err != nil || status != http.StatusOK || json.Unmarshal(out, &s) != nil || s.ID != old || s.Kind != KindPhone {
		t.Fatalf("phone call: %d %s %v, sender %+v", status, out, err, s)
	}
}

func TestANewPhoneWaitsAndGoesByTheIdItsKeyMakes(t *testing.T) {
	url := startRelay(t)
	a := newNode(t, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", 1)
	a.apply(url, nil)
	phone, _ := member.NewIdentity(member.KindMobile)
	c := phoneOn(t, url, phone)
	target := member.ID(member.KindInstance, a.key())
	eventually(t, "the phone sees a", func() bool { return len(c.Siblings()) > 0 })
	_, status, _ := c.Proxy(context.Background(), target, http.MethodGet, "/who", nil)
	if status != http.StatusForbidden {
		t.Fatalf("a new phone got %d, want it to wait", status)
	}
	if err := a.m.Approve(phone.ID()); err != nil {
		t.Fatal(err)
	}
	out, status, err := c.Proxy(context.Background(), target, http.MethodGet, "/who", nil)
	var s Sender
	if err != nil || status != http.StatusOK || json.Unmarshal(out, &s) != nil || s.ID != PhoneID(phone.Public()) || len(s.ID) != 32 {
		t.Fatalf("approved phone: %d %s %v %+v", status, out, err, s)
	}
}

func TestARemovedInstanceLeavesTheGroup(t *testing.T) {
	url := startRelay(t)
	a := newNode(t, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", 1)
	b := newNode(t, "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", 2)
	a.dir.keys[b.id] = b.key()
	b.dir.keys[a.id] = a.key()
	a.apply(url, formerOf(b))
	b.apply(url, formerOf(a))
	eventually(t, "both list each other", func() bool { return listed(a.m, b.id) && listed(b.m, a.id) })

	if err := a.m.Remove(b.id); err != nil {
		t.Fatal(err)
	}
	select {
	case <-b.removed:
	case <-time.After(15 * time.Second):
		t.Fatal("b never heard it was taken out")
	}
	if _, _, err := a.m.Call(context.Background(), b.id, http.MethodGet, "/who", nil); err == nil {
		t.Fatal("a still reaches b after taking it out")
	}
}

func TestOutsideAGroupNothingIsListedOrReached(t *testing.T) {
	a := newNode(t, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", 1)
	if a.m.Active() || len(a.m.Members()) != 0 || len(a.m.Waiting()) != 0 {
		t.Fatal("a fresh manager looks paired")
	}
	if _, _, err := a.m.Call(context.Background(), "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", http.MethodGet, "/who", nil); !errors.Is(err, ErrNotMember) {
		t.Fatalf("call = %v", err)
	}
	w := httptest.NewRecorder()
	a.m.ServeDirect(w, httptest.NewRequest(http.MethodPost, DirectPath, strings.NewReader("{}")))
	if w.Code != http.StatusNotFound {
		t.Fatalf("direct call outside a group = %d", w.Code)
	}
}

// phoneOn connects a phone with identity id to the relay at url the way the
// Android app does.
func phoneOn(t *testing.T, url string, id member.Identity) *relay.Client {
	t.Helper()
	c, err := relay.NewClient(relay.ClientOptions{
		URL: url, Key: Domain.GroupKey(secret), FrameKey: Domain.FrameKey(secret), Signing: Domain.Signing(),
		Self:     relay.Announce{InstanceID: id.ID(), Name: "phone", Deployment: string(member.KindMobile), Client: true, Member: id.ID()},
		Identity: &id,
	})
	if err != nil {
		t.Fatal(err)
	}
	c.Start()
	t.Cleanup(func() { _ = c.Close() })
	return c
}

func TestAMemberThatSaidItLeftIsNotListedForAMoment(t *testing.T) {
	url := startRelay(t)
	a := newNode(t, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", 1)
	b := newNode(t, "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", 2)
	a.dir.keys[b.id] = b.key()
	b.dir.keys[a.id] = a.key()
	a.apply(url, formerOf(b))
	b.apply(url, formerOf(a))
	eventually(t, "a lists b", func() bool { return listed(a.m, b.id) })

	a.m.Forget(b.id)
	if listed(a.m, b.id) {
		t.Fatal("a still lists b right after b said it left")
	}
}

func TestApplyingTheSameSettingsAgainKeepsTheRelayConnection(t *testing.T) {
	url := startRelay(t)
	a := newNode(t, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", 1)
	b := newNode(t, "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", 2)
	a.dir.keys[b.id] = b.key()
	b.dir.keys[a.id] = a.key()
	a.apply(url, formerOf(b))
	b.apply(url, formerOf(a))
	eventually(t, "a lists b", func() bool { return listed(a.m, b.id) })

	for range 5 {
		b.apply(url, formerOf(a))
		if !listed(a.m, b.id) {
			t.Fatal("b dropped off a's list when its settings were applied again unchanged")
		}
	}
}

func TestTwoNewInstancesKnowEachOtherSoonAfterBothApproved(t *testing.T) {
	url := startRelay(t)
	a := newNode(t, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", 1)
	b := newNode(t, "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", 2)
	a.apply(url, nil)
	b.apply(url, nil)
	kitA, kitB := member.ID(member.KindInstance, a.key()), member.ID(member.KindInstance, b.key())
	eventually(t, "each waits on the other", func() bool { return len(a.m.Waiting()) > 0 && len(b.m.Waiting()) > 0 })
	if err := a.m.Approve(kitB); err != nil {
		t.Fatal(err)
	}
	// Long enough for a's asking to have slowed down.
	time.Sleep(8 * time.Second)
	if err := b.m.Approve(kitA); err != nil {
		t.Fatal(err)
	}
	approved := time.Now()
	eventually(t, "both list each other", func() bool {
		// A page that lists the group makes b call a.
		_, _, _ = b.m.Call(context.Background(), a.id, http.MethodGet, "/who", nil)
		return listed(a.m, b.id) && listed(b.m, a.id)
	})
	if took := time.Since(approved); took > 3*time.Second {
		t.Fatalf("the two knew each other %v after the second approval", took)
	}
}

func TestADeviceOnAnOlderVersionIsNamed(t *testing.T) {
	url := startRelay(t)
	a := newNode(t, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", 1)
	a.apply(url, nil)
	eventually(t, "a on the relay", a.m.RelayConnected)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	conn, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(url, "http")+"/relay/connect", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.CloseNow() }()
	if _, _, err := conn.Read(ctx); err != nil {
		t.Fatal(err)
	}
	const old = "cccccccccccccccccccccccccccccccc"
	sealed, err := relay.SealIdentity(Domain.FrameKey(secret), old, relay.Identity{Name: "attic"})
	if err != nil {
		t.Fatal(err)
	}
	hello, err := relay.Encode(relay.TypeHello, relay.Hello{Key: Domain.GroupKey(secret), Announce: relay.Announce{InstanceID: old, Sealed: sealed}})
	if err != nil {
		t.Fatal(err)
	}
	if err := conn.Write(ctx, websocket.MessageText, hello); err != nil {
		t.Fatal(err)
	}
	eventually(t, "a names the device on the older version", func() bool {
		o := a.m.Older()
		return len(o) == 1 && o[0] == "attic"
	})
}
