package api

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	kitgroup "github.com/junkerderprovinz/parleyport/kit/group"
	"github.com/junkerderprovinz/parleyport/kit/relay"
)

// closedFortress opens three apps in a fortress on a relay of the test's
// own, has the first take the third out, and returns the relay's address and
// the phrase, which takes no new member from then on.
func closedFortress(t *testing.T) (string, string) {
	t.Helper()
	rs := httptest.NewServer(relay.New())
	t.Cleanup(rs.Close)
	words, sec := newPhrase(t)
	a := joinApp(t, rs.URL, sec, "firesquire", "tower")
	b := joinApp(t, rs.URL, sec, "shiplog", "attic")
	c := joinApp(t, rs.URL, sec, "towertorch", "cellar")
	for _, p := range []*plugin{a, b, c} {
		p.open(t)
	}
	for _, p := range []*plugin{a, b, c} {
		waitLong(t, "every app to see the other two", func() bool { return len(p.f.Members()) == 2 })
	}
	for _, pair := range [][2]*plugin{{a, b}, {b, a}} {
		waitLong(t, "the first two to know each other's app", func() bool {
			for _, m := range pair[0].f.Members() {
				if m.ID == pair[1].id && m.App.ID != "" {
					return true
				}
			}
			return false
		})
	}
	if err := a.f.Remove(c.id); err != nil {
		t.Fatal(err)
	}
	waitLong(t, "the second app to take the third out", func() bool {
		for _, m := range b.f.Members() {
			if m.ID == c.id {
				return false
			}
		}
		return true
	})
	c.close()
	return rs.URL, words
}

func joinClosed(t *testing.T, in *instance, words string) {
	t.Helper()
	_, out := in.do(t, http.MethodPost, "/api/fortress/join", map[string]any{"phrase": words})
	if out["ok"] != false || out["code"] != kitgroup.ClosedUntilNewPhrase {
		t.Fatalf("joining after a removal = %v", out)
	}
	if _, ok := out["error"]; ok {
		t.Errorf("the refusal carries an error of its own, which would hide the page's words: %v", out)
	}
}

func TestAJoinAfterARemovalIsTurnedAwayAndLeavesNothingBehind(t *testing.T) {
	url, words := closedFortress(t)
	in := fortressInstance(t)
	in.do(t, http.MethodPut, "/api/group/relay", map[string]any{"mode": "own", "url": url})
	joinClosed(t, in, words)
	if in.svc.fortressNow() != nil {
		t.Fatal("the refused join left a fortress open")
	}
	if m, _ := in.st.FortressMembership(); m.Joined() {
		t.Fatal("the refused join kept the membership")
	}
	if _, view := in.do(t, http.MethodGet, "/api/fortress", nil); view["joined"] != false {
		t.Fatalf("view after the refused join = %v", view)
	}
}

func TestARefusedJoinLeavesTheFortressOnTheGroupsPhraseAsItWas(t *testing.T) {
	url, words := closedFortress(t)
	in := newInstance(t, "cellar", strings.Repeat("a1", 32))
	t.Cleanup(in.svc.CloseFortress)
	in.do(t, http.MethodPut, "/api/group/relay", map[string]any{"mode": "own", "url": url})
	own := createGroupPhrase(t, in)
	before, err := in.st.FortressMembership()
	if err != nil {
		t.Fatal(err)
	}

	joinClosed(t, in, words)
	if in.svc.fortressNow() == nil || !in.svc.onGroupsPhrase() {
		t.Fatal("the fortress on the group's phrase did not come back")
	}
	if got, _ := in.svc.fortressPhrase(); got != own {
		t.Fatalf("the fortress phrase is %q, want the group's", got)
	}
	after, _ := in.st.FortressMembership()
	if !bytes.Equal(after.SeedEnc, before.SeedEnc) {
		t.Fatal("the fortress on the group's phrase came back under another key")
	}
	if !in.svc.pairing().Active() || in.svc.pairing().Following() {
		t.Fatal("the group left its own phrase for the fortress that turned BombVault away")
	}
}

func TestEnteringTheWordsOfAClosedFortressKeepsNothingOfThem(t *testing.T) {
	url, words := closedFortress(t)
	in := fortressInstance(t)
	in.do(t, http.MethodPut, "/api/group/relay", map[string]any{"mode": "own", "url": url})
	_, out := in.do(t, http.MethodPost, "/api/group/join", map[string]any{"phrase": words})
	if out["ok"] != false || out["code"] != kitgroup.ClosedUntilNewPhrase {
		t.Fatalf("entering the words after a removal = %v", out)
	}
	if _, ok := out["error"]; ok {
		t.Errorf("the refusal carries an error of its own: %v", out)
	}
	if in.svc.pairing().Active() || groupSecretOf(t, in) != nil {
		t.Fatal("the group kept the words of the fortress that turned BombVault away")
	}
	if m, _ := in.st.FortressMembership(); m.Joined() || in.svc.fortressNow() != nil {
		t.Fatal("the fortress that turned BombVault away stayed")
	}
}
