package api

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"testing"
	"time"

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
	var bv string
	for _, m := range p.f.Members() {
		if m.App.ID == "bombvault" {
			bv = m.ID
		}
	}
	if err := svc.RemoveFortressMember(p.id); err != nil {
		t.Fatal(err)
	}
	want := []fortressRemoved{{App: "firesquire", Name: "FireSquire", Device: "tower"}}
	if got := removedNow(t, svc); !slices.Equal(got, want) {
		t.Fatalf("removed = %+v", got)
	}

	// The next start of BombVault still names it and still refuses it.
	svc.CloseFortress()
	next := NewService(svc.cfg, svc.store, nil, nil, nil)
	if err := next.OpenFortress(nil); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(next.CloseFortress)
	if got := removedNow(t, next); !slices.Equal(got, want) {
		t.Fatalf("removed after a restart = %+v", got)
	}
	time.Sleep(3 * time.Second)
	for _, m := range next.fortressNow().Members() {
		if m.ID == p.id {
			t.Fatal("the removed plugin is a member again after a restart")
		}
	}
	if _, err := p.f.Status(context.Background(), bv, "en"); err == nil {
		t.Fatal("BombVault answers the removed plugin after a restart")
	}

	if _, _, err := next.ChangeFortressPhrase(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := removedNow(t, next); len(got) != 0 {
		t.Fatalf("still named after the phrase changed: %+v", got)
	}
}

func removedNow(t *testing.T, svc *Service) []fortressRemoved {
	t.Helper()
	got, err := svc.fortressRemovedList()
	if err != nil {
		t.Fatal(err)
	}
	return got
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
	next := NewService(in.svc.cfg, in.svc.store, nil, nil, nil)
	if m, err := next.store.FortressMembership(); err != nil || !m.RemovedHere {
		t.Fatalf("the next start forgets that another member took BombVault out: %v", err)
	}
	if _, out := in.do(t, http.MethodPost, "/api/fortress/join", map[string]any{"phrase": words}); out["joined"] != true || out["removedHere"] != false {
		t.Fatalf("joining again = %v", out)
	}
}

func TestAPluginThatGaveOnlyItsAppIDGoesByTheAppsOwnSpelling(t *testing.T) {
	svc, url, sec := fortressOnRelay(t)
	p := joinPlugin(t, url, sec)
	p.open(t)
	h := &Handler{svc: svc, store: svc.store}
	waitLong(t, "BombVault to show the plugin", func() bool {
		v, err := h.fortressViewNow()
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range v.Members {
			if m.ID == p.id {
				return m.Name == "FireSquire"
			}
		}
		return false
	})
}
