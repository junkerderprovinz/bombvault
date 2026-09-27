package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/relay"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/restickey"
	"github.com/junkerderprovinz/bombvault/internal/secret"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// passwordEngine opens a repository only with one restic password and
// records every password it was asked to try. Nothing else of the engine is
// used on the paths these tests take.
type passwordEngine struct {
	ResticEngine
	want string

	mu    sync.Mutex
	tried []string
}

func (e *passwordEngine) RepoOpens(_ context.Context, _ string, m restic.Mode) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.tried = append(e.tried, m.Password)
	return m.Encrypted && m.Password == e.want
}

// instance is one BombVault in a pairing test: its own store, APP_KEY and
// router.
type instance struct {
	appKey string
	st     *store.Repo
	svc    *Service
	h      *Handler
	router http.Handler
	engine *passwordEngine
}

func newInstance(t *testing.T, name, appKey string) *instance {
	t.Helper()
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	st := store.New(db)
	if _, err := st.MutateSettings(func(s *store.Settings) error {
		s.InstanceName = name
		s.FleetEnabled = true
		s.ContainersOffsite = ""
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{AppKey: appKey, DataDir: t.TempDir(), HostMountRoot: "/host/user", HTTPOnly: true}
	eng := &passwordEngine{}
	svc := NewService(cfg, st, nil, nil, eng)
	t.Cleanup(svc.StopGroup)
	h := NewHandler(cfg, st, nil, svc, nil, nil)
	return &instance{appKey: appKey, st: st, svc: svc, h: h, router: h.Router(), engine: eng}
}

// do sends one JSON request through the instance's full router, gates
// included, and decodes the answer.
func (in *instance) do(t *testing.T, method, path string, body any) (int, map[string]any) {
	t.Helper()
	var rd *bytes.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		rd = bytes.NewReader(b)
	} else {
		rd = bytes.NewReader(nil)
	}
	r := httptest.NewRequest(method, path, rd)
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	in.router.ServeHTTP(w, r)
	var out map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	return w.Code, out
}

func (in *instance) id(t *testing.T) string {
	t.Helper()
	g, err := in.st.GetGroupState()
	if err != nil {
		t.Fatal(err)
	}
	return g.InstanceID
}

// pairThroughRelay points both instances at an in-process relay, starts a
// group on a and joins b with the phrase a shows, the way a person would.
func pairThroughRelay(t *testing.T, a, b *instance) {
	t.Helper()
	rs := httptest.NewServer(relay.NewServer())
	t.Cleanup(rs.Close)
	for _, in := range []*instance{a, b} {
		if code, out := in.do(t, http.MethodPut, "/api/group/relay", map[string]any{"mode": "own", "url": rs.URL}); code != http.StatusOK || out["ok"] != true {
			t.Fatalf("set relay: %d %v", code, out)
		}
	}
	code, out := a.do(t, http.MethodPost, "/api/group/phrase", nil)
	phrase, _ := out["phrase"].(string)
	if code != http.StatusOK || len(strings.Fields(phrase)) != 12 {
		t.Fatalf("create phrase: %d %v", code, out)
	}
	if code, out := b.do(t, http.MethodPost, "/api/group/join", map[string]any{"phrase": phrase}); code != http.StatusOK || out["active"] != true {
		t.Fatalf("join: %d %v", code, out)
	}
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if len(a.svc.pairing().Members()) == 1 && len(b.svc.pairing().Members()) == 1 {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("the two instances never saw each other: a=%+v b=%+v", a.svc.pairing().Members(), b.svc.pairing().Members())
}

func TestTwoInstancesPairedByPhraseSeeEachOtherThroughTheRelay(t *testing.T) {
	a := newInstance(t, "cellar", strings.Repeat("a1", 32))
	b := newInstance(t, "attic", strings.Repeat("b2", 32))
	pairThroughRelay(t, a, b)

	_, out := a.do(t, http.MethodGet, "/api/group", nil)
	members, _ := out["members"].([]any)
	if len(members) != 1 {
		t.Fatalf("members = %v, want one", out["members"])
	}
	m, _ := members[0].(map[string]any)
	if m["id"] != b.id(t) || m["name"] != "attic" || m["relay"] != true || m["direct"] != false {
		t.Fatalf("member = %v, want attic through the relay", m)
	}
	relayState, _ := out["relay"].(map[string]any)
	if relayState["connected"] != true || relayState["mode"] != "own" {
		t.Fatalf("relay = %v", relayState)
	}
}

func TestFleetStatusTravelsOverTheRelay(t *testing.T) {
	a := newInstance(t, "cellar", strings.Repeat("a1", 32))
	b := newInstance(t, "attic", strings.Repeat("b2", 32))
	pairThroughRelay(t, a, b)

	if err := a.svc.RunFleetPolls(context.Background()); err != nil {
		t.Fatal(err)
	}
	peers, err := a.st.ListFleetPeers()
	if err != nil || len(peers) != 1 {
		t.Fatalf("fleet peers = %+v, %v", peers, err)
	}
	p := peers[0]
	if p.MemberID != b.id(t) || !p.LastPollOK.Valid || !p.LastPollOK.Bool || p.LastPollInstanceName != "attic" {
		t.Fatalf("fleet peer after the poll = %+v, want attic's scorecard", p)
	}
}

func TestReceiverPairingDeliversTheResticPasswordAndNeverTheAppKey(t *testing.T) {
	receiver := newInstance(t, "cellar", strings.Repeat("a1", 32))
	sender := newInstance(t, "attic", strings.Repeat("b2", 32))
	pairThroughRelay(t, receiver, sender)
	want := restickey.Derive(sender.appKey)
	receiver.engine.want = want

	code, out := receiver.do(t, http.MethodPost, "/api/receiver/repos", map[string]any{
		"name": "from attic", "repo": "rest:http://192.168.1.9:8000/attic", "memberId": sender.id(t),
	})
	if code != http.StatusOK || out["ok"] != true {
		t.Fatalf("create receiver: %d %v", code, out)
	}
	repos, err := receiver.st.ListReceivedRepos()
	if err != nil || len(repos) != 1 {
		t.Fatalf("received repos = %+v, %v", repos, err)
	}
	rr := repos[0]
	if rr.MemberID != sender.id(t) || rr.NeedsPairing() {
		t.Fatalf("receiver row = %+v, want it paired with the sender", rr)
	}
	got, err := secret.Decrypt(receiver.appKey, rr.ResticPasswordEnc)
	if err != nil || string(got) != want {
		t.Fatalf("stored password = %q, %v; want the sender's restic password", got, err)
	}
	for _, pw := range receiver.engine.tried {
		if strings.Contains(pw, sender.appKey) {
			t.Fatal("the receiver opened the repository with the sender's APP_KEY")
		}
	}
}

func TestPullPairingDeliversTheResticPassword(t *testing.T) {
	puller := newInstance(t, "cellar", strings.Repeat("a1", 32))
	source := newInstance(t, "attic", strings.Repeat("b2", 32))
	pairThroughRelay(t, puller, source)
	puller.engine.want = restickey.Derive(source.appKey)

	code, out := puller.do(t, http.MethodPost, "/api/pull/sources", map[string]any{
		"name": "attic", "repo": "rest:http://192.168.1.9:8000/attic", "domain": "containers", "memberId": source.id(t),
	})
	if code != http.StatusOK || out["ok"] != true {
		t.Fatalf("create pull source: %d %v", code, out)
	}
	sources, err := puller.st.ListPullSources()
	if err != nil || len(sources) != 1 || sources[0].MemberID != source.id(t) {
		t.Fatalf("pull sources = %+v, %v", sources, err)
	}
}

// The pairing answer is the only place a member hands out anything of its
// keys. It carries the restic password and must never carry the APP_KEY, nor
// a credential inside a repository URL.
func TestPairingAnswerOnTheWireCarriesNoAppKey(t *testing.T) {
	in := newInstance(t, "attic", strings.Repeat("b2", 32))
	if _, err := in.st.UpsertOffsiteTarget(store.OffsiteTarget{
		Domain: "containers", Name: "box", Repo: "rest:https://user:hunter2@box.example:8000/attic", Enabled: true,
	}); err != nil {
		t.Fatal(err)
	}
	status, body := in.svc.servePeer(context.Background(), relay.ProxyCall{Method: http.MethodGet, Path: "/api/group/peer/pairing"})
	if status != http.StatusOK {
		t.Fatalf("pairing answered %d: %s", status, body)
	}
	wire := string(body)
	if strings.Contains(wire, in.appKey) {
		t.Fatalf("the pairing answer carries the APP_KEY: %s", wire)
	}
	if strings.Contains(wire, "hunter2") {
		t.Fatalf("the pairing answer carries a password from a repository URL: %s", wire)
	}
	if !strings.Contains(wire, restickey.Derive(in.appKey)) {
		t.Fatalf("the pairing answer lacks the restic password: %s", wire)
	}
	if !strings.Contains(wire, "box.example:8000/attic") {
		t.Fatalf("the pairing answer lacks the off-site location: %s", wire)
	}
}

// A member holds the phrase and so reaches the peer routes, and nothing else:
// not the settings, not a secret, not the phrase.
func TestAMemberCannotReachSettingsOrSecrets(t *testing.T) {
	in := newInstance(t, "attic", strings.Repeat("b2", 32))
	for _, call := range []relay.ProxyCall{
		{Method: http.MethodGet, Path: "/api/settings"},
		{Method: http.MethodPut, Path: "/api/settings", Body: []byte(`{}`)},
		{Method: http.MethodGet, Path: "/api/settings/export?credentials=true"},
		{Method: http.MethodGet, Path: "/api/recovery-kit"},
		{Method: http.MethodGet, Path: "/api/cloud"},
		{Method: http.MethodGet, Path: "/api/notify"},
		{Method: http.MethodGet, Path: "/api/diagnostics"},
		{Method: http.MethodGet, Path: "/api/group"},
		{Method: http.MethodPost, Path: "/api/group/phrase/show", Body: []byte(`{"password":""}`)},
		{Method: http.MethodPut, Path: "/api/group/relay", Body: []byte(`{"serve":true}`)},
		{Method: http.MethodPost, Path: "/api/group/join", Body: []byte(`{"phrase":""}`)},
		{Method: http.MethodPost, Path: "/api/containers/backup-all"},
		{Method: http.MethodGet, Path: "/api/group/peer/../../settings"},
		{Method: http.MethodPost, Path: "/api/group/peer/check/../../../settings"},
		{Method: http.MethodGet, Path: "/api/group/peer/status/extra"},
	} {
		status, body := in.svc.servePeer(context.Background(), call)
		if status < 300 || status == http.StatusInternalServerError || bytes.Contains(body, []byte(`"ok":true`)) {
			t.Errorf("%s %s answered a member with %d: %s", call.Method, call.Path, status, body)
		}
	}
	for _, call := range []relay.ProxyCall{
		{Method: http.MethodGet, Path: "/api/group/peer/status"},
		{Method: http.MethodGet, Path: "/api/group/peer/pairing"},
	} {
		if status, body := in.svc.servePeer(context.Background(), call); status != http.StatusOK {
			t.Errorf("%s %s is a peer route but answered %d: %s", call.Method, call.Path, status, body)
		}
	}
}

func TestJoiningWithAMistypedWordNamesTheWordAndItsPlace(t *testing.T) {
	in := newInstance(t, "attic", strings.Repeat("b2", 32))
	phrase := "abandon abandon abandon abandon abandon abandon recieve abandon abandon abandon abandon about"
	code, out := in.do(t, http.MethodPost, "/api/group/join", map[string]any{"phrase": phrase})
	if code != http.StatusOK || out["ok"] != false || out["reason"] != "unknown_word" || out["word"] != "recieve" || out["position"] != float64(7) {
		t.Fatalf("join with a typo = %d %v, want unknown_word recieve at 7", code, out)
	}
	code, out = in.do(t, http.MethodPost, "/api/group/join", map[string]any{"phrase": strings.Replace(phrase, "recieve", "zoo", 1)})
	if code != http.StatusOK || out["reason"] != "checksum" {
		t.Fatalf("join with a swapped word = %d %v, want checksum", code, out)
	}
	if in.svc.pairing().Active() {
		t.Fatal("a phrase that did not decode still paired the instance")
	}
}

func TestShowingThePhraseAgainNeedsThePasswordWhenOneIsSet(t *testing.T) {
	in := newInstance(t, "attic", strings.Repeat("b2", 32))
	_, created := in.do(t, http.MethodPost, "/api/group/phrase", nil)
	phrase, _ := created["phrase"].(string)

	if _, out := in.do(t, http.MethodPost, "/api/group/phrase/show", map[string]any{}); out["phrase"] != phrase {
		t.Fatalf("without a login password the phrase shows again: %v", out)
	}

	hash, err := secret.HashPassword(in.appKey, "hunter2")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := in.st.MutateSettings(func(s *store.Settings) error { s.AuthPasswordHash = hash; return nil }); err != nil {
		t.Fatal(err)
	}
	token := secret.NewSessionToken(in.appKey, hash, "", time.Hour)
	show := func(password string) (int, map[string]any) {
		b, _ := json.Marshal(map[string]any{"password": password})
		r := httptest.NewRequest(http.MethodPost, "/api/group/phrase/show", bytes.NewReader(b))
		r.Header.Set("Content-Type", "application/json")
		r.AddCookie(&http.Cookie{Name: sessionCookieName, Value: token}) //nolint:gosec // G124: a cookie on a test request, never set by a server
		w := httptest.NewRecorder()
		in.router.ServeHTTP(w, r)
		var out map[string]any
		_ = json.Unmarshal(w.Body.Bytes(), &out)
		return w.Code, out
	}
	if code, out := show("wrong"); code != http.StatusOK || out["code"] != "passwordWrong" || out["phrase"] != nil {
		t.Fatalf("a session with the wrong password got %d %v", code, out)
	}
	if code, out := show("hunter2"); code != http.StatusOK || out["phrase"] != phrase {
		t.Fatalf("the right password got %d %v", code, out)
	}
}

func TestCreatingASecondGroupIsRefusedUntilTheFirstIsLeft(t *testing.T) {
	in := newInstance(t, "attic", strings.Repeat("b2", 32))
	if code, _ := in.do(t, http.MethodPost, "/api/group/phrase", nil); code != http.StatusOK {
		t.Fatalf("first create: %d", code)
	}
	if code, out := in.do(t, http.MethodPost, "/api/group/phrase", nil); code != http.StatusOK || out["code"] != "groupExists" {
		t.Fatalf("second create: %d %v, want the groupExists refusal", code, out)
	}
	if code, out := in.do(t, http.MethodDelete, "/api/group", nil); code != http.StatusOK || out["active"] != false {
		t.Fatalf("leave: %d %v", code, out)
	}
	if code, _ := in.do(t, http.MethodPost, "/api/group/phrase", nil); code != http.StatusOK {
		t.Fatalf("create after leaving: %d", code)
	}
}

func TestTheRelaySocketExistsOnlyWhileServing(t *testing.T) {
	in := newInstance(t, "attic", strings.Repeat("b2", 32))
	in.do(t, http.MethodPost, "/api/group/phrase", nil)
	if code, _ := in.do(t, http.MethodGet, relayConnectPath, nil); code != http.StatusNotFound {
		t.Fatalf("relay socket with the switch off answered %d, want 404", code)
	}
	in.do(t, http.MethodPut, "/api/group/relay", map[string]any{"serve": true})
	if code, _ := in.do(t, http.MethodGet, relayConnectPath, nil); code == http.StatusNotFound {
		t.Fatal("relay socket with the switch on answered 404")
	}
	g, err := in.st.GetGroupState()
	if err != nil {
		t.Fatal(err)
	}
	sec, err := in.svc.groupSecret(g)
	if err != nil {
		t.Fatal(err)
	}
	if !in.svc.relayServer().Admit(relay.DeriveKey(sec)) || in.svc.relayServer().Admit(relay.DeriveKey([]byte("fedcba9876543210"))) {
		t.Fatal("the served relay does not admit exactly this group's key")
	}
}

func TestRelayAddressIsNormalised(t *testing.T) {
	for in, want := range map[string]string{
		"relay.example.org":                    "https://relay.example.org",
		"  wss://relay.example.org/  ":         "wss://relay.example.org",
		"https://bv.example.org/relay/connect": "https://bv.example.org/relay/connect",
		"":                                     "",
	} {
		got, msg := normalizeRelayURL(in)
		if msg != "" || got != want {
			t.Errorf("normalizeRelayURL(%q) = %q %q, want %q", in, got, msg, want)
		}
	}
	for _, bad := range []string{"ftp://relay.example.org", "https://user:pw@relay.example.org"} {
		if _, msg := normalizeRelayURL(bad); msg == "" {
			t.Errorf("normalizeRelayURL(%q) accepted it", bad)
		}
	}
}

func TestStoredAppKeysAreConvertedToResticPasswords(t *testing.T) {
	in := newInstance(t, "cellar", strings.Repeat("a1", 32))
	theirs := strings.Repeat("c3", 32)
	enc, err := secret.Encrypt(in.appKey, []byte(theirs))
	if err != nil {
		t.Fatal(err)
	}
	rr, err := in.st.CreateReceivedRepo(store.ReceivedRepo{Name: "old", Repo: "/host/user/old", LegacyAppKeyEnc: enc})
	if err != nil {
		t.Fatal(err)
	}
	ps, err := in.st.CreatePullSource(store.PullSource{Name: "old", Repo: "rest:http://x/old", Domain: "containers", LegacyAppKeyEnc: enc})
	if err != nil {
		t.Fatal(err)
	}
	if err := in.svc.ConvertLegacyAppKeys(); err != nil {
		t.Fatal(err)
	}
	gotRR, _, _ := in.st.GetReceivedRepo(rr.ID)
	gotPS, _, _ := in.st.GetPullSource(ps.ID)
	for name, row := range map[string]struct{ legacy, pw []byte }{
		"received repo": {gotRR.LegacyAppKeyEnc, gotRR.ResticPasswordEnc},
		"pull source":   {gotPS.LegacyAppKeyEnc, gotPS.ResticPasswordEnc},
	} {
		if len(row.legacy) != 0 {
			t.Errorf("%s still stores the other instance's APP_KEY", name)
		}
		pw, err := secret.Decrypt(in.appKey, row.pw)
		if err != nil || string(pw) != restickey.Derive(theirs) {
			t.Errorf("%s password = %q, %v; want the one its APP_KEY derives", name, pw, err)
		}
	}
	if !gotRR.NeedsPairing() || !gotPS.NeedsPairing() {
		t.Fatal("converted rows must still ask to be paired")
	}
}

func TestMeshOfferReachesTheMemberOverTheGroup(t *testing.T) {
	a := newInstance(t, "cellar", strings.Repeat("a1", 32))
	b := newInstance(t, "attic", strings.Repeat("b2", 32))
	pairThroughRelay(t, a, b)
	peers, err := a.svc.syncFleetPeers()
	if err != nil || len(peers) != 1 {
		t.Fatalf("fleet rows = %+v, %v", peers, err)
	}
	code, out := a.do(t, http.MethodPost, "/api/fleet/peers/"+peers[0].ID+"/mesh-offer", map[string]any{
		"domain": "containers", "baseUrl": "http://192.168.1.9:8000",
	})
	if code != http.StatusOK || out["ok"] != true {
		t.Fatalf("propose: %d %v", code, out)
	}
	offers, err := b.st.ListMeshOffers()
	if err != nil || len(offers) != 1 {
		t.Fatalf("offers on the member = %+v, %v", offers, err)
	}
	if offers[0].From != "cellar" || !strings.HasPrefix(offers[0].Repo, "rest:http://192.168.1.9:8000/bombvault-containers/containers") {
		t.Fatalf("offer = %+v", offers[0])
	}
}

func TestAnOldFleetRowIsTakenOverByTheMemberWithItsName(t *testing.T) {
	a := newInstance(t, "cellar", strings.Repeat("a1", 32))
	b := newInstance(t, "attic", strings.Repeat("b2", 32))
	old, err := a.st.CreateFleetPeer(store.FleetPeer{Name: "attic box", URL: "https://192.168.1.9:3443", Enabled: true, LastPollInstanceName: "attic"})
	if err != nil {
		t.Fatal(err)
	}
	pairThroughRelay(t, a, b)
	peers, err := a.svc.syncFleetPeers()
	if err != nil || len(peers) != 1 {
		t.Fatalf("fleet rows = %+v, %v; want the old row reused", peers, err)
	}
	if peers[0].ID != old.ID || peers[0].MemberID != b.id(t) || peers[0].NeedsPairing() {
		t.Fatalf("fleet row = %+v, want the old row paired with attic", peers[0])
	}
}
