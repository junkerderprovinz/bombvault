package api

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/keydeposit"
	"github.com/junkerderprovinz/bombvault/internal/relay"
	"github.com/junkerderprovinz/bombvault/internal/restickey"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// groupOf pairs the first instance with every other through one in-process
// relay and returns the phrase, so that another instance can join later.
func groupOf(t *testing.T, first *instance, others ...*instance) (phrase, relayURL string) {
	t.Helper()
	rs := httptest.NewServer(relay.NewServer())
	t.Cleanup(rs.Close)
	useRelay(t, rs.URL, first)
	code, out := first.do(t, http.MethodPost, "/api/group/phrase", nil)
	phrase, _ = out["phrase"].(string)
	if code != http.StatusOK || len(strings.Fields(phrase)) != 12 {
		t.Fatalf("create phrase: %d %v", code, out)
	}
	for _, in := range others {
		joinGroup(t, in, rs.URL, phrase)
	}
	seeEachOther(t, append([]*instance{first}, others...)...)
	return phrase, rs.URL
}

func useRelay(t *testing.T, url string, in *instance) {
	t.Helper()
	if code, out := in.do(t, http.MethodPut, "/api/group/relay", map[string]any{"mode": "own", "url": url}); code != http.StatusOK || out["ok"] != true {
		t.Fatalf("set relay: %d %v", code, out)
	}
}

func joinGroup(t *testing.T, in *instance, relayURL, phrase string) {
	t.Helper()
	useRelay(t, relayURL, in)
	if code, out := in.do(t, http.MethodPost, "/api/group/join", map[string]any{"phrase": phrase}); code != http.StatusOK || out["active"] != true {
		t.Fatalf("join: %d %v", code, out)
	}
}

// seeEachOther waits until every instance has every other one in reach.
func seeEachOther(t *testing.T, all ...*instance) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		ready := true
		for _, in := range all {
			ready = ready && len(in.svc.pairing().Members()) == len(all)-1
		}
		if ready {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("the instances never saw each other")
}

// deposit has owner give keeper its deposit the way a person would, and
// returns the answer.
func deposit(t *testing.T, owner, keeper *instance) map[string]any {
	t.Helper()
	_, out := owner.do(t, http.MethodPut, "/api/group/members/"+keeper.id(t)+"/roles/keeper", nil)
	return out
}

func kept(t *testing.T, keeper, owner *instance) (store.KeptDeposit, bool) {
	t.Helper()
	d, found, err := keeper.st.KeptDeposit(owner.id(t))
	if err != nil {
		t.Fatal(err)
	}
	return d, found
}

// keyRunsOf lists kind and status of the key events an instance wrote.
func keyRunsOf(t *testing.T, in *instance) []string {
	t.Helper()
	rows, err := in.db.Query(`SELECT kind, status FROM runs WHERE target_id = ? ORDER BY rowid`, store.KeyTargetID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close() //nolint:errcheck // test query
	var out []string
	for rows.Next() {
		var kind, status string
		if err := rows.Scan(&kind, &status); err != nil {
			t.Fatal(err)
		}
		out = append(out, kind+" "+status)
	}
	return out
}

var codeShape = regexp.MustCompile(`^([0-9A-HJKMNP-TV-Z]{5}-){3}[0-9A-HJKMNP-TV-Z]{5}$`)

func TestAKeeperHoldsADepositItCannotRead(t *testing.T) {
	tower := newInstance(t, "tower", strings.Repeat("a1", 32))
	attic := newInstance(t, "attic", strings.Repeat("b2", 32))
	groupOf(t, tower, attic)

	out := deposit(t, tower, attic)
	code, _ := out["recoveryCode"].(string)
	keeper, _ := out["keeper"].(map[string]any)
	if out["ok"] != true || !codeShape.MatchString(code) || keeper["state"] != store.DepositHeld {
		t.Fatalf("deposit: %v", out)
	}

	held, found := kept(t, attic, tower)
	if !found || held.State != store.DepositHeld || held.OwnerName != "tower" || held.Version != keydeposit.Version || len(held.Salt) == 0 {
		t.Fatalf("the keeper holds %+v, found %v", held, found)
	}
	raw, err := hex.DecodeString(tower.appKey)
	if err != nil {
		t.Fatal(err)
	}
	for name, secret := range map[string][]byte{
		"the key": []byte(tower.appKey), "the key's bytes": raw,
		"the restic password": []byte(restickey.Derive(tower.appKey)), "the kit's heading": []byte("BombVault"),
	} {
		if bytes.Contains(held.Deposit, secret) || bytes.Contains(held.Salt, secret) {
			t.Fatalf("what the keeper holds carries %s in the clear", name)
		}
	}
	other, err := keydeposit.NewCode()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := keydeposit.Open(held.Deposit, other, held.Salt); !errors.Is(err, keydeposit.ErrWrongCode) {
		t.Fatalf("the deposit opens with a code that is not its own: %v", err)
	}
	plain, err := keydeposit.Open(held.Deposit, code, held.Salt)
	if err != nil {
		t.Fatalf("the deposit does not open with its code: %v", err)
	}
	content, err := keydeposit.Unpack(plain)
	if err != nil {
		t.Fatalf("what the code opens: %v", err)
	}
	if kit, err := readRecoveryKit(content.Kit); err != nil || kit.appKey != tower.appKey {
		t.Fatalf("the deposit is not the owner's kit: %v", err)
	}

	// The owner keeps the public half and the salt, not the code.
	identity, found, err := tower.st.DepositIdentity()
	if err != nil || !found || len(identity.RecipientEnc) == 0 {
		t.Fatalf("the owner's identity: %+v, %v, %v", identity, found, err)
	}
	if again := deposit(t, tower, attic); again["ok"] != true || again["recoveryCode"] != nil {
		t.Fatalf("a second deposit made a second code: %v", again)
	}

	_, roles := tower.do(t, http.MethodGet, "/api/group/roles", nil)
	member := roles["members"].([]any)[0].(map[string]any)
	if got := member["byMember"].(map[string]any)["keeper"].(map[string]any); got["state"] != store.DepositHeld || member["canKeep"] != true {
		t.Fatalf("the owner's roles show %v, canKeep %v", got, member["canKeep"])
	}
	_, roles = attic.do(t, http.MethodGet, "/api/group/roles", nil)
	member = roles["members"].([]any)[0].(map[string]any)
	if got := member["forMember"].(map[string]any)["keeper"].(map[string]any); got["state"] != store.DepositHeld {
		t.Fatalf("the keeper's roles show %v", got)
	}
	if self := roles["self"].(map[string]any)["keeper"].([]any); len(self) != 1 {
		t.Fatalf("the keeper lists itself as keeper for %v", self)
	}
	if got := keyRunsOf(t, tower); len(got) != 2 || got[0] != "keydeposit success" {
		t.Fatalf("the owner's activity log has %v", got)
	}
	if got := keyRunsOf(t, attic); len(got) != 2 || got[0] != "keykeep success" {
		t.Fatalf("the keeper's activity log has %v", got)
	}
}

func TestADepositIsRenewedWithoutTheCode(t *testing.T) {
	tower := newInstance(t, "tower", strings.Repeat("a1", 32))
	attic := newInstance(t, "attic", strings.Repeat("b2", 32))
	groupOf(t, tower, attic)
	code, _ := deposit(t, tower, attic)["recoveryCode"].(string)
	first, _ := kept(t, attic, tower)

	tower.svc.RenewDeposits(context.Background())
	if same, _ := kept(t, attic, tower); !bytes.Equal(same.Deposit, first.Deposit) {
		t.Fatal("a renewal sent a deposit although nothing changed")
	}

	if _, err := tower.st.MutateSettings(func(s *store.Settings) error {
		s.VMsOffsite = "rest:https://box:8000/vms"
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	tower.svc.RenewDeposits(context.Background())

	renewed, _ := kept(t, attic, tower)
	if bytes.Equal(renewed.Deposit, first.Deposit) {
		t.Fatal("the keeper still holds the deposit from before the change")
	}
	plain, err := keydeposit.Open(renewed.Deposit, code, renewed.Salt)
	if err != nil {
		t.Fatalf("the renewed deposit does not open with the first code: %v", err)
	}
	if !strings.Contains(string(plain), "rest:https://box:8000/vms") {
		t.Fatal("the renewed deposit does not name the new location")
	}
	if got := keyRunsOf(t, tower); got[len(got)-1] != "keyrenew success" {
		t.Fatalf("the owner's activity log ends with %v", got)
	}
}

func TestADepositTheKeeperDeletedComesBackOnlyWhenAPersonAsks(t *testing.T) {
	tower := newInstance(t, "tower", strings.Repeat("a1", 32))
	attic := newInstance(t, "attic", strings.Repeat("b2", 32))
	groupOf(t, tower, attic)
	deposit(t, tower, attic)

	if _, out := attic.do(t, http.MethodDelete, "/api/group/deposits/"+tower.id(t), nil); out["ok"] != true {
		t.Fatalf("the keeper deletes the deposit: %v", out)
	}
	if held, _ := kept(t, attic, tower); held.State != store.DepositDeclined || len(held.Deposit) != 0 || len(held.Salt) != 0 {
		t.Fatalf("after the delete the keeper holds %+v", held)
	}

	tower.svc.RenewDeposits(context.Background())
	sent, err := tower.st.SentDeposits()
	if err != nil || len(sent) != 1 || sent[0].State != store.DepositDeclined {
		t.Fatalf("the owner sees %+v, %v; want its deposit declined", sent, err)
	}
	if held, _ := kept(t, attic, tower); held.State != store.DepositDeclined {
		t.Fatal("a renewal put a deposit back that the keeper had deleted")
	}

	if out := deposit(t, tower, attic); out["ok"] != true || out["recoveryCode"] != nil {
		t.Fatalf("asking again: %v", out)
	}
	if held, _ := kept(t, attic, tower); held.State != store.DepositHeld || len(held.Deposit) == 0 {
		t.Fatalf("after a person asked again the keeper holds %+v", held)
	}

	if _, out := tower.do(t, http.MethodDelete, "/api/group/members/"+attic.id(t)+"/roles/keeper", nil); out["ok"] != true || out["told"] != true {
		t.Fatalf("the owner takes the deposit back: %v", out)
	}
	if _, found := kept(t, attic, tower); found {
		t.Fatal("the keeper still holds a deposit the owner took back")
	}
	if _, found, _ := tower.st.DepositIdentity(); found {
		t.Fatal("the owner keeps a recovery code's identity with no keeper left")
	}
}

func TestAnInstanceThatDoesNotNameTheCapabilityIsSentNoDeposit(t *testing.T) {
	tower := newInstance(t, "tower", strings.Repeat("a1", 32))
	attic := newInstance(t, "attic", strings.Repeat("b2", 32))
	groupOf(t, tower, attic)
	was := peerCapabilities
	peerCapabilities = nil
	t.Cleanup(func() { peerCapabilities = was })

	out := deposit(t, tower, attic)
	if out["ok"] != false || out["code"] != "keeper-too-old" {
		t.Fatalf("a deposit for an instance without the capability: %v", out)
	}
	if _, found := kept(t, attic, tower); found {
		t.Fatal("the instance was sent a deposit")
	}
	if _, found, _ := tower.st.DepositIdentity(); found {
		t.Fatal("a refused deposit made a recovery code")
	}
}

// emergencyGroup has in ask keeper for owner's deposit with a code. ask is
// the person pressing the button; without it the call only looks.
func emergencyGroup(t *testing.T, in, keeper *instance, ownerID, code string, ask bool) map[string]any {
	t.Helper()
	_, out := in.do(t, http.MethodPost, "/api/emergency/group", map[string]any{"memberId": keeper.id(t), "ownerId": ownerID, "code": code, "ask": ask})
	return out
}

func releasesOn(t *testing.T, keeper *instance) []releaseView {
	t.Helper()
	_, out := keeper.do(t, http.MethodGet, "/api/group/deposits", nil)
	b, err := json.Marshal(out["deposits"])
	if err != nil {
		t.Fatal(err)
	}
	var view depositsView
	if err := json.Unmarshal(b, &view); err != nil {
		t.Fatal(err)
	}
	return view.Releases
}

func decideRelease(t *testing.T, keeper *instance, id, decision string) map[string]any {
	t.Helper()
	_, out := keeper.do(t, http.MethodPost, "/api/group/deposits/releases/"+id, map[string]any{"decision": decision})
	return out
}

// goneTower is a group of tower and attic in which attic keeps tower's
// deposit and tower has since gone, and a fresh install that joined in its
// place.
func goneTower(t *testing.T) (attic, fresh *instance, towerID, towerKey, code string) {
	t.Helper()
	tower := newInstance(t, "tower", strings.Repeat("a1", 32))
	attic = newInstance(t, "attic", strings.Repeat("b2", 32))
	phrase, relayURL := groupOf(t, tower, attic)
	code, _ = deposit(t, tower, attic)["recoveryCode"].(string)
	towerID = tower.id(t)
	tower.svc.StopGroup()

	fresh = newInstance(t, "fresh", strings.Repeat("c3", 32))
	joinGroup(t, fresh, relayURL, phrase)
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if _, ok := fresh.svc.reachableMember(attic.id(t)); ok {
			if _, ok := attic.svc.reachableMember(fresh.id(t)); ok {
				return attic, fresh, towerID, tower.appKey, code
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("the fresh install never reached the keeper")
	return nil, nil, "", "", ""
}

func TestANewInstallGetsTheDepositOnlyAfterAPersonOnTheKeeperAllowsIt(t *testing.T) {
	var logged bytes.Buffer
	log.SetOutput(&logged)
	defer log.SetOutput(os.Stderr)
	attic, fresh, towerID, towerKey, code := goneTower(t)

	_, offered := fresh.do(t, http.MethodGet, "/api/emergency/deposits", nil)
	list, _ := offered["deposits"].([]any)
	if len(list) != 1 || list[0].(map[string]any)["ownerName"] != "tower" || list[0].(map[string]any)["memberId"] != attic.id(t) {
		t.Fatalf("the deposits the group offers: %v", offered)
	}

	if out := emergencyGroup(t, fresh, attic, towerID, code, false); out["release"] != releaseUnasked {
		t.Fatalf("looking before anybody asked: %v", out)
	}
	out := emergencyGroup(t, fresh, attic, towerID, code, true)
	if out["ok"] != true || out["release"] != store.ReleaseAsked || out["emergency"] != nil {
		t.Fatalf("the ask: %v", out)
	}
	emergencyGroup(t, fresh, attic, towerID, code, false)
	asks := releasesOn(t, attic)
	if len(asks) != 1 || asks[0].State != store.ReleaseAsked || asks[0].RequesterName != "fresh" || asks[0].OwnerName != "tower" ||
		asks[0].Requests != 1 || asks[0].RequestsLastHour != 1 {
		t.Fatalf("what the keeper sees of the request: %+v", asks)
	}
	if _, view := fresh.do(t, http.MethodGet, "/api/emergency", nil); view["emergency"].(map[string]any)["session"] != nil {
		t.Fatal("an emergency started before the keeper allowed anything")
	}
	_, roles := attic.do(t, http.MethodGet, "/api/group/roles", nil)
	waiting := 0
	for _, raw := range roles["members"].([]any) {
		if m := raw.(map[string]any); m["memberId"] == fresh.id(t) {
			waiting = int(m["asks"].(float64))
		}
	}
	if waiting != 1 {
		t.Fatalf("the keeper's card of the asking instance counts %d requests, want 1", waiting)
	}

	if res := decideRelease(t, attic, asks[0].ID, "allow"); res["ok"] != true || res["state"] != store.ReleaseAllowed {
		t.Fatalf("allow: %v", res)
	}
	wrong, err := keydeposit.NewCode()
	if err != nil {
		t.Fatal(err)
	}
	if out := emergencyGroup(t, fresh, attic, towerID, wrong, false); out["code"] != "deposit-code" || out["release"] != store.ReleaseAllowed {
		t.Fatalf("a wrong code: %v", out)
	}
	if got := releasesOn(t, attic); got[0].State != store.ReleaseDone {
		t.Fatalf("after the deposit went out the request reads %q", got[0].State)
	}
	out = emergencyGroup(t, fresh, attic, towerID, code, false)
	if out["ok"] != true || out["release"] != store.ReleaseAllowed {
		t.Fatalf("the right code: %v", out)
	}
	session := out["emergency"].(map[string]any)["session"].(map[string]any)
	if session["source"] != store.KeyFromGroup {
		t.Fatalf("the emergency that started: %v", session)
	}
	fresh.svc.emergencyMu.Lock()
	got, collected := fresh.svc.emergency.backupKey, fresh.svc.collected
	fresh.svc.emergencyMu.Unlock()
	if got != towerKey || collected != nil {
		t.Fatalf("the emergency holds the old install's key: %v, deposit still in memory: %v", got == towerKey, collected != nil)
	}
	if _, found, _ := fresh.st.AdoptedKey(); found {
		t.Fatal("opening the deposit stored the key before anybody adopted it")
	}

	if got := strings.Join(keyRunsOf(t, attic), ","); !strings.HasSuffix(got, "keyask success,keyallow success,keyrelease success") {
		t.Fatalf("the keeper's activity log has %v", got)
	}
	if got := strings.Join(keyRunsOf(t, fresh), ","); got != "keyread failed,keyread success" {
		t.Fatalf("the new install's activity log has %v", got)
	}
	for _, secret := range []string{code, strings.ReplaceAll(code, "-", ""), wrong, towerKey} {
		if strings.Contains(logged.String(), secret) {
			t.Fatal("a code or the key reached a log")
		}
	}
}

func TestADepositGoesToNobodyWithoutMembershipOrAnAllow(t *testing.T) {
	attic, fresh, towerID, _, code := goneTower(t)
	ask := peerReleaseAsk{OwnerID: towerID, Name: "fresh", Renew: true}

	unnamed := peerCallAs(t, attic, "", "/api/group/peer/deposit/release", ask)
	if unnamed["ok"] != false || unnamed["deposit"] != nil {
		t.Fatalf("a call that names no member: %v", unnamed)
	}
	if stranger := peerCallAs(t, attic, strings.Repeat("9", 32), "/api/group/peer/deposit/release", ask); stranger["ok"] != false || len(releasesOn(t, attic)) != 0 {
		t.Fatalf("a call from an id that is no member: %v", stranger)
	}
	waiting := peerCallAs(t, attic, fresh.id(t), "/api/group/peer/deposit/release", ask)
	if waiting["state"] != store.ReleaseAsked || waiting["deposit"] != nil || waiting["salt"] != nil {
		t.Fatalf("a request nobody allowed: %v", waiting)
	}
	if none := peerCallAs(t, attic, fresh.id(t), "/api/group/peer/deposit/release", peerReleaseAsk{OwnerID: strings.Repeat("0", 32), Renew: true}); none["state"] != releaseNone {
		t.Fatalf("a request for a deposit nobody made: %v", none)
	}

	first := releasesOn(t, attic)[0]
	if res := decideRelease(t, attic, first.ID, "refuse"); res["state"] != store.ReleaseDeclined {
		t.Fatalf("refuse: %v", res)
	}
	if out := emergencyGroup(t, fresh, attic, towerID, code, false); out["ok"] != false || out["code"] != "release-declined" {
		t.Fatalf("looking after the refusal: %v", out)
	}
	if res := decideRelease(t, attic, first.ID, "allow"); res["code"] != "release-decided" {
		t.Fatalf("allowing a request that was refused: %v", res)
	}
	if out := emergencyGroup(t, fresh, attic, towerID, "nonsense", true); out["code"] != "deposit-code-shape" {
		t.Fatalf("a code of the wrong shape: %v", out)
	}

	// Three new requests an hour: the refused one was the first.
	for round := 2; round <= 3; round++ {
		if out := emergencyGroup(t, fresh, attic, towerID, code, true); out["release"] != store.ReleaseAsked {
			t.Fatalf("request %d: %v", round, out)
		}
		decideRelease(t, attic, releasesOn(t, attic)[0].ID, "refuse")
	}
	if out := emergencyGroup(t, fresh, attic, towerID, code, true); out["code"] != "release-limit" {
		t.Fatalf("the fourth request of the hour: %v", out)
	}
	asks := releasesOn(t, attic)
	if len(asks) != 3 || asks[0].Requests != 3 {
		t.Fatalf("the keeper recorded %d requests: %+v", len(asks), asks)
	}
	if got := keyRunsOf(t, attic); got[len(got)-1] != "keyask failed" {
		t.Fatalf("the keeper's activity log ends with %v", got)
	}
}
