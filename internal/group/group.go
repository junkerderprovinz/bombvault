// Package group connects the BombVault instances that share one pairing
// phrase, and the phones paired with them, through the parleyport kit.
//
// The kit addresses every member by an id made from its member key. BombVault
// keeps the ids its members had before, in its tables and in what they sign,
// so this package translates: a member's key is pinned under its BombVault id
// in the Directory, and a key pinned nowhere yet is asked for its id once.
package group

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"net/netip"
	"strings"
	"sync"
	"time"

	kitgroup "github.com/junkerderprovinz/parleyport/kit/group"
	"github.com/junkerderprovinz/parleyport/kit/keys"
	"github.com/junkerderprovinz/parleyport/kit/lan"
	"github.com/junkerderprovinz/parleyport/kit/member"
	"github.com/junkerderprovinz/parleyport/kit/relay"
)

// Domain is what every key of a BombVault group is derived under.
const Domain keys.Domain = "bombvault"

// Multicast is where BombVault instances on one network announce themselves.
var Multicast = netip.MustParseAddrPort("239.255.77.51:8751")

// Ports are where BombVault answers direct calls unless told otherwise, HTTPS
// and plain HTTP.
var Ports = []int{3443, 3000}

// DirectPath is the route a member answers direct calls on.
const DirectPath = kitgroup.DirectPath

// MaxCallBytes caps what one call to a member carries.
const MaxCallBytes = kitgroup.MaxCallBytes

// maxAnswerBytes caps a direct answer, which can carry a whole scorecard.
const maxAnswerBytes = 8 << 20

// Mode is which relay an instance uses for members it cannot reach directly.
type Mode string

// The relay modes.
const (
	ModeProject Mode = "project"
	ModeOwn     Mode = "own"
	ModeOff     Mode = "off"
)

// ValidMode reports whether m is one of the relay modes.
func ValidMode(m Mode) bool {
	return m == ModeProject || m == ModeOwn || m == ModeOff
}

// KindPhone is what a phone of the group goes by in BombVault.
const KindPhone = "android"

// PhoneID is the id a phone that joined on the kit goes by in BombVault: the
// start of its key's hash, short enough for every place an id goes, and one
// the phone works out the same way.
func PhoneID(key []byte) string {
	sum := sha256.Sum256(key)
	return hex.EncodeToString(sum[:16])
}

// ProbePath is where a member says which id it goes by in BombVault.
const ProbePath = "/api/group/peer/hello"

// Hello is what a member answers at ProbePath.
type Hello struct {
	OK         bool   `json:"ok"`
	InstanceID string `json:"instanceId"`
	Name       string `json:"name"`
	Version    string `json:"version"`
	// DirectURL is where the instance's pages are, for a phone at home.
	DirectURL string `json:"directUrl,omitempty"`
}

// Directory keeps each member's key under the id BombVault knows it by.
type Directory interface {
	MemberByKey(key []byte) (id string, found bool, err error)
	MemberKey(id string) (key []byte, found bool, err error)
	PinMemberKey(id string, key []byte) error
}

// Config is what the Manager runs on. A nil Secret means this instance is in
// no group.
type Config struct {
	Secret []byte
	// Seed is the seed of this instance's member key, the key it signs its
	// calls with.
	Seed []byte
	// InstanceID is the id this instance goes by in BombVault.
	InstanceID string
	Name       string
	// DirectURL is where this instance answers, as a browser reached it.
	DirectURL string
	Mode      Mode
	// RelayURL is the own relay's address, used in ModeOwn only.
	RelayURL string
	Store    kitgroup.Store
	// Former is what this instance knew of its group before the kit kept it,
	// taken in once, the first time the group opens with an empty Store.
	Former func() (kitgroup.Roster, error)
	// OnRemoved runs when another instance took this one out of the group.
	OnRemoved func()
}

// Member is another member of the group that is reachable now, under the id
// BombVault knows it by.
type Member struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Version string `json:"version"`
	// Direct is true when the member is reachable on this network.
	Direct bool `json:"direct"`
	// Relay is true when the member is connected to the same relay.
	Relay bool `json:"relay"`
	// Address is where the member's pages are, when it said.
	Address string `json:"address"`
	// Kind is KindPhone for a phone, empty for an instance.
	Kind string `json:"kind,omitempty"`
}

// Waiting is a member that asked to join and waits for a person here, or one
// this instance waits for there.
type Waiting struct {
	// ID is the member's id in the group, the one Approve and Remove take.
	ID   string `json:"id"`
	Name string `json:"name"`
	Kind string `json:"kind,omitempty"`
	// Fingerprint is how its key reads to compare by eye.
	Fingerprint string `json:"fingerprint,omitempty"`
	Online      bool   `json:"online"`
	// There means this instance waits for its approval on that member.
	There bool `json:"there,omitempty"`
}

// Sender is the member a call into this instance came from.
type Sender struct {
	ID   string
	Kind string
	// Key is the member key the call is signed with.
	Key []byte
}

type senderKey struct{}

// WithSender returns ctx carrying s as the sender of a call.
func WithSender(ctx context.Context, s Sender) context.Context {
	return context.WithValue(ctx, senderKey{}, s)
}

// SenderOf returns the member a call came from.
func SenderOf(ctx context.Context) (Sender, bool) {
	s, ok := ctx.Value(senderKey{}).(Sender)
	return s, ok
}

// ErrNotMember is returned for a call to an instance that is not reachable
// in this group.
var ErrNotMember = errors.New("group: no such member is reachable")

// learnEvery is how often a member that did not say its id is asked again,
// such as one where this instance still waits for approval.
const learnEvery = 5 * time.Second

// Manager keeps this instance's side of the group running.
type Manager struct {
	serve http.Handler
	dir   Directory
	lan   *lan.Service

	applyMu sync.Mutex
	mu      sync.Mutex
	cfg     Config
	g       *kitgroup.Group
	asked   map[string]time.Time
	// retrying is set while learnAll is due again.
	retrying bool
}

// NewManager returns a Manager that answers members' calls with serve, which
// finds each caller through SenderOf. It is in no group until Apply is called
// with a secret.
func NewManager(serve http.Handler, dir Directory) *Manager {
	return &Manager{
		serve: serve,
		dir:   dir,
		lan:   lan.New(lan.Config{Group: Multicast, Ports: Ports}),
		asked: map[string]time.Time{},
	}
}

// Start begins listening for members on the local network.
func (m *Manager) Start() { m.lan.Start() }

// Close leaves the group and the network.
func (m *Manager) Close() {
	m.mu.Lock()
	g := m.g
	m.g = nil
	m.mu.Unlock()
	if g != nil {
		_ = g.Close()
	}
	_ = m.lan.Close()
}

// Apply switches the Manager to cfg. A change the relay and the keys do not
// see, a new name or address, keeps the group open.
func (m *Manager) Apply(cfg Config) {
	m.applyMu.Lock()
	defer m.applyMu.Unlock()
	self := kitgroup.Self{Name: cfg.Name, Deployment: "container", Address: cfg.DirectURL, DirectURL: cfg.DirectURL}
	m.mu.Lock()
	prev, old := m.cfg, m.g
	m.cfg = cfg
	if old != nil && sameGroup(prev, cfg) {
		m.mu.Unlock()
		old.SetSelf(self)
		return
	}
	m.g = nil
	m.mu.Unlock()
	// Close waits for the calls the group serves, which take m.mu.
	if old != nil {
		_ = old.Close()
	}
	if len(cfg.Secret) == 0 {
		return
	}
	if g := m.open(cfg, self); g != nil {
		m.mu.Lock()
		m.g = g
		m.mu.Unlock()
	}
}

func (m *Manager) open(cfg Config, self kitgroup.Self) *kitgroup.Group {
	id, err := member.FromSeed(member.KindInstance, cfg.Seed)
	if err != nil {
		log.Printf("group: the member key does not open: %v", err)
		return nil
	}
	stored, err := cfg.Store.Load()
	if err != nil {
		log.Printf("group: %v", err)
		return nil
	}
	g, err := kitgroup.Open(kitgroup.Config{
		Domain:         Domain,
		Secret:         cfg.Secret,
		Identity:       id,
		Self:           self,
		Relay:          kitgroup.RelayMode(cfg.Mode),
		RelayURL:       strings.TrimSpace(cfg.RelayURL),
		LAN:            m.lan,
		Store:          cfg.Store,
		Kinds:          []member.Kind{member.KindMobile},
		Ports:          Ports,
		MaxAnswerBytes: maxAnswerBytes,
		OnRemoved:      cfg.OnRemoved,
		OnChange:       func() { go m.learnAll() },
	}, http.HandlerFunc(m.serveMember))
	if err != nil {
		log.Printf("group: %v", err)
		return nil
	}
	if len(stored) == 0 && cfg.Former != nil {
		former, err := cfg.Former()
		if err == nil {
			err = g.Import(former)
		}
		if err != nil {
			log.Printf("group: take over the members known before: %v", err)
		}
	}
	return g
}

// sameGroup reports whether a and b open the same group the same way.
func sameGroup(a, b Config) bool {
	return hmac.Equal(a.Secret, b.Secret) && hmac.Equal(a.Seed, b.Seed) && a.Mode == b.Mode &&
		strings.TrimSpace(a.RelayURL) == strings.TrimSpace(b.RelayURL) && a.InstanceID == b.InstanceID
}

func (m *Manager) group() (*kitgroup.Group, Config) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.g, m.cfg
}

// Active reports whether this instance is in a group.
func (m *Manager) Active() bool {
	g, _ := m.group()
	return g != nil
}

// RelayConnected reports whether the relay socket is up.
func (m *Manager) RelayConnected() bool {
	g, _ := m.group()
	return g != nil && g.RelayConnected()
}

// AdmitsRelayKey reports whether key is this group's relay key, the only key a
// relay served by this instance lets in.
func (m *Manager) AdmitsRelayKey(key string) bool {
	g, cfg := m.group()
	return g != nil && hmac.Equal([]byte(key), []byte(Domain.GroupKey(cfg.Secret)))
}

// Members lists the approved members reachable now, sorted by name.
func (m *Manager) Members() []Member {
	g, _ := m.group()
	if g == nil {
		return []Member{}
	}
	out := []Member{}
	for _, km := range g.Members() {
		if km.State != kitgroup.StateApproved || !km.Online() {
			continue
		}
		id, ok := m.known(km.ID, km.Kind, km.Key)
		if !ok {
			continue
		}
		out = append(out, Member{ID: id, Name: km.Name, Direct: km.Direct, Relay: km.Relay, Address: km.Address, Kind: kindOf(km.Kind)})
	}
	return out
}

// Waiting lists the members that wait for approval here, and the ones this
// instance waits for there.
func (m *Manager) Waiting() []Waiting {
	g, _ := m.group()
	out := []Waiting{}
	if g == nil {
		return out
	}
	for _, km := range g.Members() {
		w := Waiting{ID: km.ID, Name: km.Name, Kind: kindOf(km.Kind), Online: km.Online()}
		if len(km.Key) > 0 {
			w.Fingerprint = member.Fingerprint(km.Key)
		}
		switch {
		case km.State == kitgroup.StatePending:
			out = append(out, w)
		case km.State == kitgroup.StateApproved && km.Awaiting:
			w.There = true
			out = append(out, w)
		}
	}
	return out
}

// Approve lets in the member that waits under id.
func (m *Manager) Approve(id string) error {
	g, _ := m.group()
	if g == nil {
		return ErrNotMember
	}
	if err := g.Approve(id); err != nil {
		return err
	}
	go m.learnAll()
	return nil
}

// Remove takes a member out of the group on every instance in reach, under
// its id here or its id in the group. Its key stays shut out.
func (m *Manager) Remove(id string) error {
	g, _ := m.group()
	if g == nil {
		return ErrNotMember
	}
	for _, km := range g.Members() {
		if km.ID == id {
			return g.Remove(id)
		}
	}
	key, found, err := m.dir.MemberKey(id)
	if err != nil || !found {
		return errors.Join(err, ErrNotMember)
	}
	for _, kind := range []member.Kind{member.KindInstance, member.KindMobile} {
		if kid := member.ID(kind, key); kid != "" && m.listed(g, kid) {
			return g.Remove(kid)
		}
	}
	return ErrNotMember
}

func (m *Manager) listed(g *kitgroup.Group, id string) bool {
	for _, km := range g.Members() {
		if km.ID == id {
			return true
		}
	}
	return false
}

// Call asks member id, under the id BombVault knows it by, and returns its
// answer. An error means no answer came from that member.
func (m *Manager) Call(ctx context.Context, id, method, path string, body []byte) (int, []byte, error) {
	g, _ := m.group()
	if g == nil {
		return 0, nil, ErrNotMember
	}
	key, found, err := m.dir.MemberKey(id)
	if err != nil {
		return 0, nil, err
	}
	target := id
	if found {
		target = member.ID(member.KindInstance, key)
	}
	if !relay.ValidInstanceID(target) {
		return 0, nil, ErrNotMember
	}
	status, out, err := g.Call(ctx, target, method, path, body)
	if errors.Is(err, kitgroup.ErrUnreachable) || errors.Is(err, kitgroup.ErrNoSuchMember) {
		return 0, nil, ErrNotMember
	}
	return status, out, err
}

// ServeDirect answers a member's direct call, or like an instance without
// the feature outside a group.
func (m *Manager) ServeDirect(w http.ResponseWriter, r *http.Request) {
	g, _ := m.group()
	if g == nil {
		http.NotFound(w, r)
		return
	}
	g.DirectHandler().ServeHTTP(w, r)
}

// serveMember hands a member's call to the app with the member's BombVault
// id. A member nobody here knows yet is asked for it first, and only told
// this instance's own at ProbePath until it says.
func (m *Manager) serveMember(w http.ResponseWriter, r *http.Request) {
	c, ok := kitgroup.CallerOf(r.Context())
	if !ok {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	id, known := m.known(c.ID, c.Kind, c.Key)
	if !known && r.URL.Path == ProbePath {
		// Saying who this instance is cannot wait for the caller to say the
		// same, which is what the caller asks this for.
		m.serve.ServeHTTP(w, r)
		return
	}
	if !known {
		id, known = m.learn(r.Context(), c.ID, c.Key)
	}
	if !known {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write(relay.RefusalBody("memberUnknown", "this instance could not learn which instance is calling; it asks again shortly"))
		return
	}
	m.serve.ServeHTTP(w, r.WithContext(WithSender(r.Context(), Sender{ID: id, Kind: kindOf(c.Kind), Key: c.Key})))
}

// known returns the BombVault id of the member with key: the one key is
// pinned under, else for a phone its PhoneID, which key is pinned under from
// then on.
func (m *Manager) known(kitID string, kind member.Kind, key []byte) (string, bool) {
	if len(key) == 0 {
		return "", false
	}
	id, found, err := m.dir.MemberByKey(key)
	if err != nil {
		log.Printf("group: %v", err)
		return "", false
	}
	if found {
		return id, true
	}
	if kind == member.KindMobile {
		id := PhoneID(key)
		if err := m.dir.PinMemberKey(id, key); err != nil {
			log.Printf("group: pin the key of phone %s: %v", kitID, err)
			return "", false
		}
		return id, true
	}
	return "", false
}

// learnAll asks every approved instance whose key is pinned nowhere which id
// it goes by, and again later while one has not said.
func (m *Manager) learnAll() {
	g, _ := m.group()
	if g == nil {
		return
	}
	open := false
	for _, km := range g.Members() {
		if km.Kind != member.KindInstance || km.State != kitgroup.StateApproved || !km.Online() {
			continue
		}
		if _, ok := m.known(km.ID, km.Kind, km.Key); ok {
			continue
		}
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		if _, ok := m.learn(ctx, km.ID, km.Key); !ok {
			open = true
		}
		cancel()
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if open && !m.retrying && m.g == g {
		m.retrying = true
		time.AfterFunc(learnEvery, func() {
			m.mu.Lock()
			m.retrying = false
			m.mu.Unlock()
			m.learnAll()
		})
	}
}

// learn asks instance kitID, whose answers key signs, which id it goes by in
// BombVault, and pins key under it. An id pinned for another key, or this
// instance's own, is not taken.
func (m *Manager) learn(ctx context.Context, kitID string, key []byte) (string, bool) {
	g, cfg := m.group()
	if g == nil || len(key) == 0 || member.ID(member.KindInstance, key) != kitID {
		return "", false
	}
	m.mu.Lock()
	if at, ok := m.asked[kitID]; ok && time.Since(at) < learnEvery {
		m.mu.Unlock()
		return "", false
	}
	m.asked[kitID] = time.Now()
	m.mu.Unlock()

	status, body, err := g.Call(ctx, kitID, http.MethodGet, ProbePath, nil)
	if err != nil || status != http.StatusOK {
		return "", false
	}
	var h Hello
	if json.Unmarshal(body, &h) != nil || !h.OK || h.InstanceID == "" || len(h.InstanceID) > 64 || h.InstanceID == cfg.InstanceID {
		return "", false
	}
	if err := m.dir.PinMemberKey(h.InstanceID, key); err != nil {
		log.Printf("group: instance %s names the id %s, which another key is pinned for: %v", kitID, h.InstanceID, err)
		return "", false
	}
	m.mu.Lock()
	delete(m.asked, kitID)
	m.mu.Unlock()
	return h.InstanceID, true
}

func kindOf(k member.Kind) string {
	if k == member.KindMobile {
		return KindPhone
	}
	return ""
}
