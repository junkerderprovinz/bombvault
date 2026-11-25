package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func fortressInstance(t *testing.T) *instance {
	t.Helper()
	in := newInstance(t, "attic", strings.Repeat("b2", 32))
	t.Cleanup(in.svc.CloseFortress)
	return in
}

func TestOutsideAFortressThePageOffersToJoin(t *testing.T) {
	in := fortressInstance(t)
	code, out := in.do(t, http.MethodGet, "/api/fortress", nil)
	if code != http.StatusOK || out["ok"] != true || out["joined"] != false || out["defaultDevice"] != "BombVault" || out["passwordSet"] != true {
		t.Fatalf("GET /api/fortress = %d %v", code, out)
	}
}

func TestJoiningNamesAMistypedWordAndItsPlace(t *testing.T) {
	in := fortressInstance(t)
	words := "abandon abandon abandon abandon abandon abandon recieve abandon abandon abandon abandon about"
	_, out := in.do(t, http.MethodPost, "/api/fortress/join", map[string]any{"phrase": words})
	if out["ok"] != false || out["reason"] != "unknown_word" || out["word"] != "recieve" || out["position"] != float64(7) {
		t.Fatalf("join with a typo = %v", out)
	}
}

func TestJoiningTakesThePhraseAndTheViewNeverGivesItBack(t *testing.T) {
	in := fortressInstance(t)
	words, _ := newPhrase(t)
	code, out := in.do(t, http.MethodPost, "/api/fortress/join", map[string]any{"phrase": words})
	if code != http.StatusOK || out["ok"] != true || out["joined"] != true || out["phrase"] != true {
		t.Fatalf("join = %d %v", code, out)
	}
	_, again := in.do(t, http.MethodPost, "/api/fortress/join", map[string]any{"phrase": words})
	if again["ok"] != false || again["code"] != "inFortress" {
		t.Fatalf("joining twice = %v", again)
	}
	_, view := in.do(t, http.MethodGet, "/api/fortress", nil)
	wire, _ := json.Marshal(view)
	if view["joined"] != true || bytes.Contains(wire, []byte(strings.Join(strings.Fields(words)[:2], " "))) {
		t.Fatalf("view = %s", wire)
	}
}

func TestTheFortressPhraseWantsAFreshSignIn(t *testing.T) {
	in := fortressInstance(t)
	words, _ := newPhrase(t)
	in.do(t, http.MethodPost, "/api/fortress/join", map[string]any{"phrase": words})
	if _, out := in.do(t, http.MethodPost, "/api/fortress/phrase/show", map[string]any{}); out["ok"] != true || out["phrase"] != words {
		t.Fatalf("a sign-in from moments ago got %v", out)
	}
	in.session = staleSession(t, in)
	if code, out := in.do(t, http.MethodPost, "/api/fortress/phrase/show", map[string]any{}); code != http.StatusForbidden || out["error"] != "signInAgain" || out["phrase"] != nil {
		t.Fatalf("a sign-in from days ago got %d %v", code, out)
	}
	if w := kitRequest(t, in.router, http.MethodPost, "me/confirm", in.session, map[string]any{"password": testPassword}); w.Code != http.StatusOK {
		t.Fatalf("the password again: %d %s", w.Code, w.Body.String())
	}
	if _, out := in.do(t, http.MethodPost, "/api/fortress/phrase/show", map[string]any{}); out["phrase"] != words {
		t.Fatalf("after the password again: %v", out)
	}
}

func TestWithoutALoginPasswordTheFortressPhraseShowsWithoutOne(t *testing.T) {
	in := fortressInstance(t)
	removePasswordOn(t, in.router, in.session, testPassword)
	in.session = ""
	words, _ := newPhrase(t)
	in.do(t, http.MethodPost, "/api/fortress/join", map[string]any{"phrase": words})
	if _, out := in.do(t, http.MethodPost, "/api/fortress/phrase/show", map[string]any{}); out["phrase"] != words {
		t.Fatalf("show without a login password = %v", out)
	}
	if _, view := in.do(t, http.MethodGet, "/api/fortress", nil); view["passwordSet"] != false {
		t.Fatalf("view = %v", view)
	}
}

func TestChangingTheFortressPhraseAnswersWithTheNewWords(t *testing.T) {
	in := fortressInstance(t)
	words, _ := newPhrase(t)
	in.do(t, http.MethodPost, "/api/fortress/join", map[string]any{"phrase": words})
	_, out := in.do(t, http.MethodPost, "/api/fortress/phrase", map[string]any{})
	changed, _ := out["phrase"].(string)
	if out["ok"] != true || len(strings.Fields(changed)) != 12 || changed == words {
		t.Fatalf("change = %v", out)
	}
	if left, ok := out["left"].([]any); !ok || len(left) != 0 {
		t.Fatalf("left = %v", out["left"])
	}
	if _, shown := in.do(t, http.MethodPost, "/api/fortress/phrase/show", map[string]any{}); shown["phrase"] != changed {
		t.Fatalf("the phrase shown after the change = %v", shown)
	}
}

func TestLeavingTheFortressFromThePage(t *testing.T) {
	in := fortressInstance(t)
	if _, out := in.do(t, http.MethodDelete, "/api/fortress", nil); out["ok"] != false || out["code"] != "noFortress" {
		t.Fatalf("leaving no fortress = %v", out)
	}
	words, _ := newPhrase(t)
	in.do(t, http.MethodPost, "/api/fortress/join", map[string]any{"phrase": words})
	if _, out := in.do(t, http.MethodDelete, "/api/fortress", nil); out["ok"] != true || out["joined"] != false {
		t.Fatalf("leave = %v", out)
	}
}

func TestAPermissionOfBombVaultSwitchesOffAndOn(t *testing.T) {
	in := fortressInstance(t)
	words, _ := newPhrase(t)
	in.do(t, http.MethodPost, "/api/fortress/join", map[string]any{"phrase": words})
	body := map[string]any{"from": "towertorch", "to": "bombvault", "action": "backup.request", "on": false}
	if _, out := in.do(t, http.MethodPut, "/api/fortress/permissions", body); out["ok"] != true {
		t.Fatalf("switch = %v", out)
	}
	body["action"] = "not an action"
	if _, out := in.do(t, http.MethodPut, "/api/fortress/permissions", body); out["ok"] != false {
		t.Fatalf("a switch that is no permission = %v", out)
	}
}

func TestTheFortressFollowsTheRelayOfTheGroup(t *testing.T) {
	in := fortressInstance(t)
	words, _ := newPhrase(t)
	in.do(t, http.MethodPost, "/api/fortress/join", map[string]any{"phrase": words})
	before := in.svc.fortressNow()
	if _, out := in.do(t, http.MethodPut, "/api/group/relay", map[string]any{"url": "relay.example.org"}); out["ok"] == false {
		t.Fatalf("relay = %v", out)
	}
	if now := in.svc.fortressNow(); now == nil || now == before {
		t.Fatal("the fortress did not open again on the group's relay")
	}
}

// A new phrase opens the fortress like the old one, so it reaches the browser
// only after a fresh sign-in, as showing the phrase does.
func TestChangingTheFortressPhraseWantsAFreshSignIn(t *testing.T) {
	in := fortressInstance(t)
	words, _ := newPhrase(t)
	in.do(t, http.MethodPost, "/api/fortress/join", map[string]any{"phrase": words})
	fresh := in.session
	in.session = staleSession(t, in)
	if code, out := in.do(t, http.MethodPost, "/api/fortress/phrase", map[string]any{}); code != http.StatusForbidden || out["phrase"] != nil {
		t.Fatalf("a change on a sign-in from days ago = %d %v", code, out)
	}
	in.session = fresh
	if _, shown := in.do(t, http.MethodPost, "/api/fortress/phrase/show", map[string]any{}); shown["phrase"] != words {
		t.Fatalf("a refused change still changed the phrase: %v", shown)
	}
}

func TestWithoutALoginPasswordThePhraseChangesWithoutOne(t *testing.T) {
	in := fortressInstance(t)
	removePasswordOn(t, in.router, in.session, testPassword)
	in.session = ""
	words, _ := newPhrase(t)
	in.do(t, http.MethodPost, "/api/fortress/join", map[string]any{"phrase": words})
	if _, out := in.do(t, http.MethodPost, "/api/fortress/phrase", map[string]any{}); out["ok"] != true || out["phrase"] == words {
		t.Fatalf("change without a login password = %v", out)
	}
}
