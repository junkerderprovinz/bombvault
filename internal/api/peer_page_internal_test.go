package api

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/keydeposit"
	"github.com/junkerderprovinz/bombvault/internal/progress"
	"github.com/junkerderprovinz/bombvault/internal/relay"
	"github.com/junkerderprovinz/bombvault/internal/seedphrase"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// phone is a paired phone in these tests: the group's words, a signing key
// of its own and the session an instance gave it.
type phone struct {
	t       *testing.T
	id      string
	seed    []byte
	client  *relay.Client
	session string
}

// joinPhone connects a phone with the words and its own key, which it names
// to an instance that asks, the way the app does.
func joinPhone(t *testing.T, relayURL, phrase, id string, seenBy ...*instance) *phone {
	t.Helper()
	sec, err := seedphrase.Decode(phrase)
	if err != nil {
		t.Fatal(err)
	}
	seed, err := keydeposit.NewOwnerSeed()
	if err != nil {
		t.Fatal(err)
	}
	serve := func(_ context.Context, call relay.ProxyCall) (int, []byte) {
		if call.Method != http.MethodGet || call.Path != "/api/group/peer/key" {
			return http.StatusNotFound, nil
		}
		proof := keydeposit.Prove(seed, keydeposit.Claim{Action: keydeposit.ProofKey, OwnerID: id, KeeperID: call.Sender})
		b, _ := json.Marshal(peerKeyAnswer{OK: true, Key: keydeposit.OwnerKey(seed), Proof: proof})
		return http.StatusOK, b
	}
	c, err := relay.NewClient(relay.ClientOptions{
		URL: relayURL, Key: relay.DeriveKey(sec), FrameKey: relay.DeriveFrameKey(sec), InstanceID: id,
		Identity: relay.Identity{Name: "pixel", Version: Version, Kind: "android"}, Serve: serve,
	})
	if err != nil {
		t.Fatal(err)
	}
	c.Start()
	t.Cleanup(func() { _ = c.Close() })
	deadline := time.Now().Add(10 * time.Second)
	for _, in := range seenBy {
		for !isMember(in, id) {
			if time.Now().After(deadline) {
				t.Fatal("an instance never saw the phone")
			}
			time.Sleep(20 * time.Millisecond)
		}
	}
	return &phone{t: t, id: id, seed: seed, client: c}
}

// ask sends target one peer call in the name of sender and returns its status
// and body.
func (p *phone) ask(target, sender, path string, body []byte) (int, []byte) {
	p.t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for {
		res, err := p.client.Call(context.Background(), target, relay.ProxyCall{Method: http.MethodPost, Path: path, Body: body, Sender: sender})
		if err == nil {
			return res.Status, res.Body
		}
		if time.Now().After(deadline) {
			p.t.Fatalf("%s: %v", path, err)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// signed is a proof in the phone's name over payload, for in.
func (p *phone) signed(seed []byte, in *instance, action string, payload []byte) peerProof {
	return p.signedAt(seed, in, action, payload, time.Now().UnixMilli())
}

func (p *phone) signedAt(seed []byte, in *instance, action string, payload []byte, at int64) peerProof {
	claim := keydeposit.Claim{Action: action, OwnerID: p.id, KeeperID: in.id(p.t), Counter: at, Deposit: payload}
	return peerProof{Key: keydeposit.OwnerKey(seed), Counter: at, Proof: keydeposit.Prove(seed, claim)}
}

// signIn asks in for a session for the phone and keeps it.
func (p *phone) signIn(in *instance) {
	p.t.Helper()
	body, _ := json.Marshal(struct {
		InstanceID string `json:"instanceId"`
		peerProof
	}{p.id, p.signed(p.seed, in, keydeposit.ProofSession, callPayload(p.id))})
	status, raw := p.ask(in.id(p.t), p.id, "/api/group/peer/session", body)
	var out peerSessionResponse
	_ = json.Unmarshal(raw, &out)
	if status != http.StatusOK || out.Value == "" {
		p.t.Fatalf("no session: %d %s", status, raw)
	}
	p.session = out.Value
}

// page sends in one request of a page as the phone signs it, after edit has
// had its say on the call, and returns the call's status with the answer.
func (p *phone) page(in *instance, ask peerPageAsk, edit func(*peerPageCall)) (int, peerPageAnswer) {
	p.t.Helper()
	return p.pageFrom(in, p.id, ask, edit)
}

// pageFrom is page with the call naming sender as where it came from.
func (p *phone) pageFrom(in *instance, sender string, ask peerPageAsk, edit func(*peerPageCall)) (int, peerPageAnswer) {
	p.t.Helper()
	raw, err := json.Marshal(ask)
	if err != nil {
		p.t.Fatal(err)
	}
	call := peerPageCall{Ask: raw, Session: p.session, peerProof: p.signed(p.seed, in, keydeposit.ProofPage, raw)}
	if edit != nil {
		edit(&call)
	}
	body, _ := json.Marshal(call)
	status, out := p.ask(in.id(p.t), sender, "/api/group/peer/page", body)
	var answer peerPageAnswer
	_ = json.Unmarshal(out, &answer)
	if status == http.StatusOK && answer.Proof != nil {
		key := keydeposit.OwnerKey(depositSeedOf(p.t, in))
		claim := keydeposit.Claim{Action: keydeposit.ProofPageAnswer, OwnerID: in.id(p.t), KeeperID: p.id, Counter: call.Counter}
		whole := answer
		whole.Body = p.wholeBody(in, answer)
		claim.Deposit = pageAnswerDigest(raw, whole)
		if !keydeposit.Proven(key, answer.Proof, claim) {
			p.t.Fatalf("%s %s: the answer's signature does not hold", ask.Method, ask.Path)
		}
		answer.Body = whole.Body
	}
	return status, answer
}

// wholeBody fetches every part of a first answer.
func (p *phone) wholeBody(in *instance, first peerPageAnswer) []byte {
	p.t.Helper()
	whole := append([]byte(nil), first.Body...)
	for len(whole) < first.Size {
		status, next := p.page(in, peerPageAsk{Part: first.Part, Offset: len(whole)}, nil)
		if status != http.StatusOK || len(next.Body) == 0 {
			p.t.Fatalf("the part at %d answered %d", len(whole), status)
		}
		whole = append(whole, next.Body...)
	}
	return whole
}

// open is page for a request that has to go through, with the body unpacked.
func (p *phone) open(in *instance, method, path string, body any) (peerPageAnswer, []byte) {
	p.t.Helper()
	ask := peerPageAsk{Method: method, Path: path, Header: map[string]string{"Content-Type": "application/json"}}
	if body != nil {
		ask.Body, _ = json.Marshal(body)
	}
	status, answer := p.page(in, ask, nil)
	if status != http.StatusOK || !answer.OK {
		p.t.Fatalf("%s %s: the call answered %d %+v", method, path, status, answer)
	}
	unpacked := answer.Body
	if answer.Header["Content-Encoding"] == "gzip" {
		zr, err := gzip.NewReader(bytes.NewReader(unpacked))
		if err != nil {
			p.t.Fatal(err)
		}
		if unpacked, err = io.ReadAll(zr); err != nil {
			p.t.Fatal(err)
		}
	}
	return answer, unpacked
}

func depositSeedOf(t *testing.T, in *instance) []byte {
	t.Helper()
	seed, err := in.svc.depositSeed()
	if err != nil {
		t.Fatal(err)
	}
	return seed
}

// pagesOverRelay puts attic in a group with tower, serves attic's interface
// from spa and connects a phone that attic gave a session.
func pagesOverRelay(t *testing.T, spa fstest.MapFS) (tower, attic *instance, ph *phone, relayURL, phrase string) {
	t.Helper()
	tower = newInstance(t, "tower", strings.Repeat("a1", 32))
	attic = newInstance(t, "attic", strings.Repeat("b2", 32))
	phrase, relayURL = groupOf(t, tower, attic)
	attic.svc.ServePhonePages(NewServer(config.Config{HTTPOnly: true}, spa, attic.router).Handler())
	ph = joinPhone(t, relayURL, phrase, wordsHolderID, attic)
	ph.signIn(attic)
	return tower, attic, ph, relayURL, phrase
}

func decoded(t *testing.T, body []byte) map[string]any {
	t.Helper()
	out := map[string]any{}
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatalf("not JSON: %.200s", body)
	}
	return out
}

var shell = fstest.MapFS{"index.html": {Data: []byte("<!doctype html><title>attic</title>")}}

func TestAPageThroughTheRelayReachesWhatTheSessionOfAPhoneDoes(t *testing.T) {
	_, attic, ph, _, _ := pagesOverRelay(t, shell)

	answer, body := ph.open(attic, http.MethodGet, "/instances", nil)
	if answer.Status != http.StatusOK || !strings.Contains(string(body), "<title>attic</title>") {
		t.Fatalf("the shell: %d %s", answer.Status, body)
	}
	if !strings.Contains(answer.Header["Content-Security-Policy"], "default-src 'self'") {
		t.Fatalf("the shell came without its policy: %v", answer.Header)
	}
	if answer, body := ph.open(attic, http.MethodGet, "/api/auth", nil); answer.Status != http.StatusOK || decoded(t, body)["authed"] != true {
		t.Fatalf("the page is not signed in: %d %s", answer.Status, body)
	}
	if answer, body := ph.open(attic, http.MethodGet, "/api/status", nil); answer.Status != http.StatusOK || decoded(t, body)["ok"] != true {
		t.Fatalf("the status: %d %s", answer.Status, body)
	}

	refused := []struct {
		method, path string
		body         any
	}{
		{http.MethodGet, "/api/recovery-kit", nil},
		{http.MethodGet, "/api/settings/export?includeCredentials=1", nil},
		{http.MethodPost, "/api/login", map[string]any{"password": testPassword}},
		{http.MethodPost, "/api/logout", map[string]any{}},
		{http.MethodPost, "/api/auth/reset", map[string]any{}},
		{http.MethodPost, "/api/group/call", map[string]any{}},
		{http.MethodPut, "/api/settings", map[string]any{}},
		{http.MethodGet, "/metrics", nil},
		{http.MethodGet, "/api//recovery-kit", nil},
		{http.MethodGet, "/api/x/../recovery-kit", nil},
	}
	check := func(when string) {
		t.Helper()
		for _, r := range refused {
			answer, body := ph.open(attic, r.method, r.path, r.body)
			if answer.Status < 300 {
				t.Errorf("%s: %s %s answered %d %s", when, r.method, r.path, answer.Status, body)
			}
			if answer.Status == http.StatusForbidden && decoded(t, body)["code"] != codeOwnLoginNeeded {
				t.Errorf("%s: %s %s refused without the code: %s", when, r.method, r.path, body)
			}
		}
	}
	check("with a password")

	if _, err := attic.st.MutateSettings(func(s *store.Settings) error {
		s.AuthPasswordHash = ""
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	check("without a password")
	if answer, _ := ph.open(attic, http.MethodGet, "/api/status", nil); answer.Status != http.StatusOK {
		t.Fatalf("the status without a password: %d", answer.Status)
	}
}

func TestOddPathsAndMethodsDoNotGoThroughTheRelay(t *testing.T) {
	_, attic, ph, _, _ := pagesOverRelay(t, shell)
	for _, ask := range []peerPageAsk{
		{Method: http.MethodGet, Path: "//evil.example/api/status"},
		{Method: http.MethodGet, Path: "http://evil.example/api/status"},
		{Method: http.MethodGet, Path: "api/status"},
		{Method: http.MethodOptions, Path: "/api/status"},
		{Method: http.MethodConnect, Path: "/api/status"},
		{Method: "TRACE", Path: "/"},
	} {
		if status, answer := ph.page(attic, ask, nil); status != http.StatusBadRequest {
			t.Errorf("%s %s: %d %+v", ask.Method, ask.Path, status, answer)
		}
	}
	// A change sent without JSON is what a form of another page would send.
	form := peerPageAsk{Method: http.MethodPost, Path: "/api/backup/cancel", Header: map[string]string{"Content-Type": "text/plain"}}
	if status, answer := ph.page(attic, form, nil); status != http.StatusOK || answer.Status != http.StatusUnsupportedMediaType {
		t.Fatalf("a change without JSON: %d %d %s", status, answer.Status, answer.Body)
	}
}

func TestACookieOrHostInThePageDoesNotSignIn(t *testing.T) {
	_, attic, ph, _, _ := pagesOverRelay(t, shell)
	full := attic.session
	ask := peerPageAsk{Method: http.MethodGet, Path: "/api/recovery-kit", Header: map[string]string{
		"Cookie": sessionCookieNameHTTP + "=" + full, "Host": "attic.example", "Authorization": "Bearer x",
	}}
	if _, answer := ph.page(attic, ask, nil); answer.Status != http.StatusForbidden {
		t.Fatalf("a cookie in the page signed in: %d %s", answer.Status, answer.Body)
	}
	if status, _ := ph.page(attic, peerPageAsk{Method: http.MethodGet, Path: "/api/status"}, func(c *peerPageCall) { c.Session = full }); status != http.StatusUnauthorized {
		t.Fatalf("a person's session named in the call: %d", status)
	}
}

func TestOnlyThePhoneItselfOpensPagesThroughTheRelay(t *testing.T) {
	tower, attic, ph, _, _ := pagesOverRelay(t, shell)
	ask := peerPageAsk{Method: http.MethodGet, Path: "/api/status"}
	forger, err := keydeposit.NewOwnerSeed()
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(ask)

	cases := map[string]func(*peerPageCall){
		"unsigned":                func(c *peerPageCall) { c.peerProof = peerProof{} },
		"signed with another key": func(c *peerPageCall) { c.peerProof = ph.signed(forger, attic, keydeposit.ProofPage, raw) },
		"signed for another ask":  func(c *peerPageCall) { c.Ask, _ = json.Marshal(peerPageAsk{Method: http.MethodGet, Path: "/api/runs"}) },
		"signed long ago": func(c *peerPageCall) {
			c.peerProof = ph.signedAt(ph.seed, attic, keydeposit.ProofPage, raw, time.Now().Add(-time.Hour).UnixMilli())
		},
	}
	for name, edit := range cases {
		if status, _ := ph.page(attic, ask, edit); status != http.StatusForbidden {
			t.Errorf("%s: %d", name, status)
		}
	}
	if status, _ := ph.pageFrom(attic, tower.id(t), ask, nil); status != http.StatusForbidden {
		t.Errorf("sent by an instance in the phone's place: %d", status)
	}

	// The words let tower name the phone, and its key is not the phone's.
	towerSeed := depositSeedOf(t, tower)
	signedByTower := func(c *peerPageCall) { c.peerProof = ph.signed(towerSeed, attic, keydeposit.ProofPage, c.Ask) }
	if status, _ := ph.page(attic, ask, signedByTower); status != http.StatusForbidden {
		t.Fatalf("tower in the phone's name: %d", status)
	}

	var once peerPageCall
	if status, _ := ph.page(attic, ask, func(c *peerPageCall) { once = *c }); status != http.StatusOK {
		t.Fatalf("the phone's own call: %d", status)
	}
	if status, _ := ph.page(attic, ask, func(c *peerPageCall) { *c = once }); status != http.StatusForbidden {
		t.Fatalf("the same call again: %d", status)
	}

	forgedSession, _ := json.Marshal(struct {
		InstanceID string `json:"instanceId"`
		peerProof
	}{ph.id, ph.signed(forger, attic, keydeposit.ProofSession, callPayload(ph.id))})
	if status, raw := ph.ask(attic.id(t), ph.id, "/api/group/peer/session", forgedSession); status != http.StatusForbidden {
		t.Fatalf("a session for another key: %d %s", status, raw)
	}

	rows, err := attic.st.ListFleetPeers()
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range rows {
		if row.MemberID == ph.id {
			row.Enabled = false
			if err := attic.st.UpdateFleetPeer(row); err != nil {
				t.Fatal(err)
			}
		}
	}
	if status, _ := ph.page(attic, ask, nil); status != http.StatusForbidden {
		t.Fatalf("a phone switched off on the Instances page: %d", status)
	}
}

func TestAPasswordResetEndsThePagesOfEveryPhone(t *testing.T) {
	_, attic, ph, _, _ := pagesOverRelay(t, shell)
	ask := peerPageAsk{Method: http.MethodGet, Path: "/api/status"}
	if _, answer := ph.page(attic, ask, nil); answer.Status != http.StatusOK {
		t.Fatalf("before: %d", answer.Status)
	}

	if code, out := attic.do(t, http.MethodPost, "/api/logout-all", nil); code != http.StatusOK {
		t.Fatalf("sign out everywhere: %d %v", code, out)
	}
	if status, _ := ph.page(attic, ask, nil); status != http.StatusUnauthorized {
		t.Fatalf("after signing out everywhere: %d", status)
	}
	ph.signIn(attic)

	out := &bytes.Buffer{}
	attic.h.resetOut = out
	finished, _ := finishReset(t, attic.h, askForResetCode(t, attic.h, out), resetNewPass)
	if finished["ok"] != true {
		t.Fatalf("the reset: %v", finished)
	}
	if status, _ := ph.page(attic, ask, nil); status != http.StatusForbidden {
		t.Fatalf("after a reset: %d", status)
	}
	if err := attic.st.SetPhonePairing(true); err != nil {
		t.Fatal(err)
	}
	if status, _ := ph.page(attic, ask, nil); status != http.StatusUnauthorized {
		t.Fatalf("the old session after pairing is on again: %d", status)
	}
}

func TestALargeAnswerComesThroughTheRelayWhole(t *testing.T) {
	big := make([]byte, 3*pagePart+1234)
	if _, err := rand.Read(big); err != nil {
		t.Fatal(err)
	}
	spa := fstest.MapFS{"index.html": shell["index.html"], "assets/big.js": {Data: big}}
	_, attic, ph, _, _ := pagesOverRelay(t, spa)

	answer, body := ph.open(attic, http.MethodGet, "/assets/big.js", nil)
	if answer.Part == "" {
		t.Fatal("the answer came in one call")
	}
	if answer.Status != http.StatusOK || !bytes.Equal(body, big) {
		t.Fatalf("the bundle came back %d with %d of %d bytes", answer.Status, len(body), len(big))
	}
	if !strings.Contains(answer.Header["Cache-Control"], "immutable") {
		t.Fatalf("the bundle lost its caching: %v", answer.Header)
	}
	if status, _ := ph.page(attic, peerPageAsk{Part: answer.Part, Offset: pagePart}, nil); status != http.StatusNotFound {
		t.Fatalf("a fetched answer was kept: %d", status)
	}
}

func TestADownloadDoesNotGoThroughTheRelay(t *testing.T) {
	_, attic, ph, _, _ := pagesOverRelay(t, shell)
	// No route a phone reaches hands out a file, so one is put behind the
	// same server, compression and security headers included.
	download := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/zip")
		w.Header().Set("Content-Disposition", `attachment; filename="flash.zip"`)
		_, _ = w.Write(bytes.Repeat([]byte("PK the archive "), 1000))
	})
	attic.svc.ServePhonePages(NewServer(config.Config{HTTPOnly: true}, shell, download).Handler())
	answer, body := ph.open(attic, http.MethodGet, "/api/flash/download", nil)
	if answer.Status != http.StatusForbidden || bytes.Contains(body, []byte("PK")) || decoded(t, body)["code"] != codeRelayRefused {
		t.Fatalf("a download: %d %s", answer.Status, body)
	}
}

func TestTheProgressStreamEndsWhenItGoesThroughTheRelay(t *testing.T) {
	_, attic, ph, _, _ := pagesOverRelay(t, shell)
	live := progress.NewStore()
	attic.h.SetProgress(live)
	live.Publish(progress.Event{Key: "container:nginx", Phase: "backup", Percent: 40, Active: true})

	answer, body := ph.open(attic, http.MethodGet, "/api/progress", nil)
	if answer.Status != http.StatusOK || !strings.HasPrefix(answer.Header["Content-Type"], "text/event-stream") || !strings.Contains(string(body), `"container:nginx"`) {
		t.Fatalf("the progress: %d %v %s", answer.Status, answer.Header, body)
	}
}

func TestPartsGoOnlyToTheSenderThatAskedAndExpire(t *testing.T) {
	var parts pageParts
	body := bytes.Repeat([]byte("x"), 2*pagePart)
	token, ok := parts.keep("phone", body)
	if !ok {
		t.Fatal("nothing kept")
	}
	if _, _, ok := parts.take(token, "someone else", pagePart); ok {
		t.Fatal("another member took the part")
	}
	part, size, ok := parts.take(token, "phone", pagePart)
	if !ok || size != len(body) || len(part) != pagePart {
		t.Fatalf("the last part: %v %d %d", ok, size, len(part))
	}
	if parts.held != 0 || len(parts.by) != 0 {
		t.Fatalf("%d bytes held after the last part", parts.held)
	}

	token, _ = parts.keep("phone", bytes.Repeat([]byte("y"), 4*pagePart))
	for _, offset := range []int{3 * pagePart, pagePart, 3 * pagePart, 2 * pagePart} {
		if _, _, ok := parts.take(token, "phone", offset); !ok {
			t.Fatalf("the part at %d, fetched out of order, was gone", offset)
		}
	}
	if parts.held != 0 {
		t.Fatalf("%d bytes held after every part", parts.held)
	}

	token, _ = parts.keep("phone", body)
	parts.entries[token].expires = time.Now().Add(-time.Second)
	if _, _, ok := parts.take(token, "phone", 0); ok || parts.held != 0 {
		t.Fatal("an expired answer was handed out")
	}
}

func TestOnePhoneCannotHoldEveryWaitingPart(t *testing.T) {
	var parts pageParts
	body := bytes.Repeat([]byte("z"), 8<<20)
	for range pagePartsPerSender / len(body) {
		if _, ok := parts.keep("greedy", body); !ok {
			t.Fatal("refused below the phone's share")
		}
	}
	if _, ok := parts.keep("greedy", body); ok {
		t.Fatal("one phone kept beyond its share")
	}
	if _, ok := parts.keep("other", body); !ok {
		t.Fatal("another phone was refused while one held its share")
	}
	for i := range pagePartsBudget / len(body) {
		parts.keep("p"+string(rune('a'+i)), body)
	}
	if _, ok := parts.keep("last", body); ok {
		t.Fatal("kept beyond the budget")
	}
}

func TestAnOversizedAnswerStaysOffTheRelay(t *testing.T) {
	big := &pageRecorder{header: http.Header{}}
	chunk := make([]byte, 1<<20)
	for range pageLimit/len(chunk) + 1 {
		_, _ = big.Write(chunk)
	}
	if a := big.answer(); a.Status != http.StatusRequestEntityTooLarge || a.Size > 1024 {
		t.Fatalf("an oversized answer: %d with %d bytes", a.Status, a.Size)
	}
}

func TestTheAnswerDigestMatchesTheApp(t *testing.T) {
	// The same inputs in the app's PageWireTest give the same digest.
	a := peerPageAnswer{Status: 200, Header: map[string]string{"Content-Type": "text/javascript", "Cache-Control": "immutable"}, Body: []byte("bundle")}
	got := pageAnswerDigest([]byte(`{"method":"GET","path":"/"}`), a)
	const want = "34199ce6a09cbfe79396ca3e5bb294b36084ad7ba5fc127e94509ecb63ec1733"
	if hex.EncodeToString(got) != want {
		t.Fatalf("digest %x", got)
	}
}

func TestAPhoneSignatureMatchesTheApp(t *testing.T) {
	// The app's PageWireTest signs the same claim with the same seed.
	seed := bytes.Repeat([]byte{7}, 32)
	claim := keydeposit.Claim{Action: keydeposit.ProofPage, OwnerID: "phone1", KeeperID: "attic1", Counter: 1790000000000, Deposit: []byte(`{"method":"GET","path":"/"}`)}
	const wantKey = "ea4a6c63e29c520abef5507b132ec5f9954776aebebe7b92421eea691446d22c"
	const wantProof = "f5eae457fb1c03dda4c99e15e842e34596c13e560a61420645fb7127698661e056d6dca0d11f1b8a1c0e383c2110d4a811ad69d02603ac0c1a56c7bc1fcf1b05"
	if got := hex.EncodeToString(keydeposit.OwnerKey(seed)); got != wantKey {
		t.Errorf("key %s", got)
	}
	if got := hex.EncodeToString(keydeposit.Prove(seed, claim)); got != wantProof {
		t.Errorf("proof %s", got)
	}
}
