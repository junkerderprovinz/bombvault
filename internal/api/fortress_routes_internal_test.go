package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

func fortressInstance(t *testing.T) *instance {
	t.Helper()
	in := newInstance(t, "attic", strings.Repeat("b2", 32))
	t.Cleanup(in.svc.CloseFortress)
	return in
}

func TestOutsideAFortressThePageOffersToJoinUnderTheServersName(t *testing.T) {
	in := fortressInstance(t)
	code, out := in.do(t, http.MethodGet, "/api/fortress", nil)
	if code != http.StatusOK || out["ok"] != true || out["joined"] != false || out["device"] != "attic" || out["defaultDevice"] != "attic" || out["passwordSet"] != true {
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

func TestTheFortressPhraseShowsOnlyWithThePassword(t *testing.T) {
	in := fortressInstance(t)
	words, _ := newPhrase(t)
	in.do(t, http.MethodPost, "/api/fortress/join", map[string]any{"phrase": words})
	if _, out := in.do(t, http.MethodPost, "/api/fortress/phrase/show", map[string]any{"password": "wrong"}); out["code"] != "passwordWrong" || out["phrase"] != nil {
		t.Fatalf("the wrong password got %v", out)
	}
	if _, out := in.do(t, http.MethodPost, "/api/fortress/phrase/show", map[string]any{"password": testPassword}); out["ok"] != true || out["phrase"] != words {
		t.Fatalf("the right password got %v", out)
	}
}

func TestWithoutALoginPasswordTheFortressPhraseShowsWithoutOne(t *testing.T) {
	in := fortressInstance(t)
	if _, err := in.st.MutateSettings(func(s *store.Settings) error { s.AuthPasswordHash = ""; return nil }); err != nil {
		t.Fatal(err)
	}
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
	_, out := in.do(t, http.MethodPost, "/api/fortress/phrase", nil)
	changed, _ := out["phrase"].(string)
	if out["ok"] != true || len(strings.Fields(changed)) != 12 || changed == words {
		t.Fatalf("change = %v", out)
	}
	if left, ok := out["left"].([]any); !ok || len(left) != 0 {
		t.Fatalf("left = %v", out["left"])
	}
	if _, shown := in.do(t, http.MethodPost, "/api/fortress/phrase/show", map[string]any{"password": testPassword}); shown["phrase"] != changed {
		t.Fatalf("the phrase shown after the change = %v", shown)
	}
}

func TestTheServerNameSavesAndAnEmptyOneGoesBackToTheFoundName(t *testing.T) {
	in := fortressInstance(t)
	if _, out := in.do(t, http.MethodPut, "/api/fortress/device", map[string]any{"device": "cellar"}); out["ok"] != true || out["device"] != "cellar" || out["defaultDevice"] != "attic" {
		t.Fatalf("rename = %v", out)
	}
	if _, out := in.do(t, http.MethodPut, "/api/fortress/device", map[string]any{"device": "two\nlines"}); out["ok"] != false || out["code"] != "deviceName" {
		t.Fatalf("a name on two lines = %v", out)
	}
	if _, out := in.do(t, http.MethodPut, "/api/fortress/device", map[string]any{"device": "  "}); out["device"] != "attic" {
		t.Fatalf("cleared = %v", out)
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
