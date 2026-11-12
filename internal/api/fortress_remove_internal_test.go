package api

import (
	"context"
	"errors"
	"net/http"
	"testing"

	kitmember "github.com/junkerderprovinz/parleyport/kit/member"

	"github.com/junkerderprovinz/bombvault/internal/secret"
)

func TestRemovingAMemberKeepsItsNameUntilThePhraseChanges(t *testing.T) {
	svc, url, sec := fortressOnRelay(t)
	p := joinPlugin(t, url, sec)
	p.open(t)
	waitLong(t, "BombVault to know the plugin's app", func() bool {
		for _, m := range svc.fortressNow().Members() {
			if m.ID == p.id && m.App.ID == "firesquire" {
				return true
			}
		}
		return false
	})
	if err := svc.RemoveFortressMember(p.id); err != nil {
		t.Fatal(err)
	}
	svc.house.mu.Lock()
	removed := append([]fortressRemoved{}, svc.house.removed...)
	svc.house.mu.Unlock()
	if len(removed) != 1 || removed[0] != (fortressRemoved{App: "firesquire", Name: "firesquire", Device: "tower"}) {
		t.Fatalf("removed = %+v", removed)
	}
	for _, m := range svc.fortressNow().Members() {
		if m.ID == p.id {
			t.Fatal("the removed plugin is still a member")
		}
	}

	if _, _, err := svc.ChangeFortressPhrase(context.Background()); err != nil {
		t.Fatal(err)
	}
	svc.house.mu.Lock()
	left := len(svc.house.removed)
	svc.house.mu.Unlock()
	if left != 0 {
		t.Fatalf("%d removed members still named after the phrase changed", left)
	}
}

func TestBombVaultCannotRemoveItselfOrAStranger(t *testing.T) {
	svc, _ := joinedSvc(t)
	m, err := svc.store.FortressMembership()
	if err != nil {
		t.Fatal(err)
	}
	seed, err := secret.Decrypt(svc.cfg.AppKey, m.SeedEnc)
	if err != nil {
		t.Fatal(err)
	}
	own, err := kitmember.FromSeed(kitmember.KindInstance, seed)
	if err != nil {
		t.Fatal(err)
	}
	self := own.ID()
	for _, id := range []string{self, "nobody"} {
		if err := svc.RemoveFortressMember(id); !errors.Is(err, errNoMember) {
			t.Errorf("removing %q = %v, want errNoMember", id, err)
		}
	}
}

func TestTheRemoveRouteNamesAMissingMemberAndAMissingFortress(t *testing.T) {
	in := fortressInstance(t)
	if _, out := in.do(t, http.MethodDelete, "/api/fortress/members/abc", nil); out["ok"] != false || out["code"] != "noFortress" {
		t.Fatalf("removing outside a fortress = %v", out)
	}
	words, _ := newPhrase(t)
	in.do(t, http.MethodPost, "/api/fortress/join", map[string]any{"phrase": words})
	if _, out := in.do(t, http.MethodDelete, "/api/fortress/members/abc", nil); out["ok"] != false || out["code"] != "noMember" {
		t.Fatalf("removing a stranger = %v", out)
	}
	if _, view := in.do(t, http.MethodGet, "/api/fortress", nil); view["removedHere"] != false || len(view["removed"].([]any)) != 0 {
		t.Fatalf("view = %v", view)
	}
}

func TestAnInstanceTakenOutSaysSoUntilItJoinsAgain(t *testing.T) {
	in := fortressInstance(t)
	words, _ := newPhrase(t)
	in.do(t, http.MethodPost, "/api/fortress/join", map[string]any{"phrase": words})
	in.svc.house.mu.Lock()
	gen := in.svc.house.gen
	in.svc.house.mu.Unlock()
	in.svc.removedFrom(gen)
	waitLong(t, "the view to say another member took BombVault out", func() bool {
		_, view := in.do(t, http.MethodGet, "/api/fortress", nil)
		return view["joined"] == false && view["removedHere"] == true
	})
	if _, out := in.do(t, http.MethodPost, "/api/fortress/join", map[string]any{"phrase": words}); out["joined"] != true || out["removedHere"] != false {
		t.Fatalf("joining again = %v", out)
	}
}
