package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/junkerderprovinz/parleyport/kit/fortress"
	"github.com/junkerderprovinz/parleyport/kit/group"
	kitmember "github.com/junkerderprovinz/parleyport/kit/member"
	"github.com/junkerderprovinz/parleyport/kit/relay"
)

func viewOf(t *testing.T, svc *Service) fortressView {
	t.Helper()
	v, err := (&Handler{store: svc.store, svc: svc}).fortressViewNow()
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func memberIn(v fortressView, id string) (fortressMemberView, bool) {
	for _, m := range v.Members {
		if m.ID == id {
			return m, true
		}
	}
	return fortressMemberView{}, false
}

// waitKnown waits until svc has heard which app member id runs.
func waitKnown(t *testing.T, svc *Service, id, app string) {
	t.Helper()
	waitLong(t, "BombVault to know the app of "+app, func() bool {
		f := svc.fortressNow()
		if f == nil {
			return false
		}
		for _, m := range f.Members() {
			if m.ID == id && m.App.ID == app {
				return true
			}
		}
		return false
	})
}

func online(f *fortress.Fortress, id string) bool {
	for _, m := range f.Members() {
		if m.ID == id && m.Online() {
			return true
		}
	}
	return false
}

func TestAMemberKnownOnlyFromTheBookIsListedByItsDevice(t *testing.T) {
	svc, url, sec := fortressOnRelay(t)
	p := joinPlugin(t, url, sec)
	p.open(t)
	waitKnown(t, svc, p.id, "firesquire")

	// The second plugin comes while BombVault is away and meets only the first.
	svc.CloseFortress()
	q := joinApp(t, url, sec, "shiplog", "attic")
	q.open(t)
	waitLong(t, "the two plugins to meet", func() bool { return online(p.f, q.id) && online(q.f, p.id) })
	q.close()
	if err := svc.OpenFortress(nil); err != nil {
		t.Fatal(err)
	}
	waitLong(t, "BombVault to hear of the second plugin", func() bool {
		_, ok := memberIn(viewOf(t, svc), q.id)
		return ok
	})
	got, _ := memberIn(viewOf(t, svc), q.id)
	if got.App != "" || got.Device != "attic" || got.Online || got.Opens {
		t.Fatalf("the plugin BombVault never met is listed as %+v", got)
	}
}

func TestAMemberAwayForAPhraseChangeIsMarkedUntilItFollows(t *testing.T) {
	svc, url, sec := fortressOnRelay(t)
	p := joinPlugin(t, url, sec)
	p.open(t)
	waitKnown(t, svc, p.id, "firesquire")
	if got, _ := memberIn(viewOf(t, svc), p.id); got.OldPhrase {
		t.Fatalf("a member in reach is marked as on the old phrase: %+v", got)
	}
	p.close()
	if _, _, err := svc.ChangeFortressPhrase(context.Background()); err != nil {
		t.Fatal(err)
	}
	restartFortress(t, svc)
	if got, ok := memberIn(viewOf(t, svc), p.id); !ok || !got.OldPhrase || got.Online {
		t.Fatalf("the plugin away for the change is listed as %+v", got)
	}

	p.open(t)
	waitLong(t, "the plugin to follow the new phrase", func() bool {
		followed, _ := fortress.Follow(context.Background(), p.dir, p.f)
		return followed
	})
	p.close()
	p.open(t)
	waitLong(t, "the mark to go once the plugin is back on the new phrase", func() bool {
		got, _ := memberIn(viewOf(t, svc), p.id)
		return got.Online && !got.OldPhrase
	})
}

func TestAPhraseChangeNamesEveryMemberLeftOnTheOldPhrase(t *testing.T) {
	svc, url, sec := fortressOnRelay(t)
	fire := joinApp(t, url, sec, "firesquire", "tower")
	ship := joinApp(t, url, sec, "shiplog", "tower")
	fire.open(t)
	ship.open(t)
	waitKnown(t, svc, fire.id, "firesquire")
	waitKnown(t, svc, ship.id, "shiplog")
	fire.close()
	ship.close()
	_, left, err := svc.ChangeFortressPhrase(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	want := []fortressLeft{{App: "firesquire", Name: "FireSquire", Device: "tower"}, {App: "shiplog", Name: "ShipLog", Device: "tower"}}
	if !slices.Equal(left, want) {
		t.Fatalf("left on the old phrase = %+v, want %+v", left, want)
	}
}

func TestAnInstanceLeftOnTheOldPhraseAsksForTheNewWordsAndTakesThem(t *testing.T) {
	rs := httptest.NewServer(relay.New())
	t.Cleanup(rs.Close)
	a := newInstance(t, "cellar", strings.Repeat("a1", 32))
	b := newInstance(t, "attic", strings.Repeat("b2", 32))
	t.Cleanup(a.svc.CloseFortress)
	t.Cleanup(b.svc.CloseFortress)
	words, sec := newPhrase(t)
	for _, in := range []*instance{a, b} {
		in.do(t, http.MethodPut, "/api/group/relay", map[string]any{"mode": "own", "url": rs.URL})
		if _, out := in.do(t, http.MethodPost, "/api/fortress/join", map[string]any{"phrase": words}); out["ok"] != true {
			t.Fatalf("join = %v", out)
		}
	}
	p := joinPlugin(t, rs.URL, sec)
	p.open(t)
	waitKnown(t, a.svc, p.id, "firesquire")
	waitKnown(t, b.svc, p.id, "firesquire")

	// The plugin follows a change made while a was away, and is all a
	// reaches afterwards.
	a.svc.CloseFortress()
	p.close()
	newWords, _, err := b.svc.ChangeFortressPhrase(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	restartFortress(t, b.svc)
	p.open(t)
	waitLong(t, "the plugin to follow the new phrase", func() bool {
		followed, _ := fortress.Follow(context.Background(), p.dir, p.f)
		return followed
	})
	p.close()
	b.svc.CloseFortress()
	p.open(t)
	if _, view := a.do(t, http.MethodGet, "/api/fortress", nil); view["phraseChanged"] != false {
		t.Fatalf("a closed fortress says the phrase changed: %v", view)
	}
	if err := a.svc.OpenFortress(nil); err != nil {
		t.Fatal(err)
	}
	waitLong(t, "the instance left behind to hear the phrase changed", func() bool {
		_, view := a.do(t, http.MethodGet, "/api/fortress", nil)
		return view["phraseChanged"] == true
	})

	if _, out := a.do(t, http.MethodPost, "/api/group/join", map[string]any{"phrase": newWords}); out["ok"] != true {
		t.Fatalf("entering the new words = %v", out)
	}
	if got, _ := a.svc.fortressPhrase(); got != newWords {
		t.Fatal("the instance did not take the new phrase")
	}
	waitLong(t, "the instance to reach the plugin on the new phrase", func() bool {
		f := a.svc.fortressNow()
		return f != nil && online(f, p.id)
	})
	if _, view := a.do(t, http.MethodGet, "/api/fortress", nil); view["phraseChanged"] != false {
		t.Fatalf("the view still asks for the new words: %v", view)
	}
}

// The fortress lists a member as soon as it hears from it, before it learns
// the member's name and app, while the pairing already knows it by name.
func TestAPairedInstanceTheFortressHasNotNamedYetIsLeftUnderItsName(t *testing.T) {
	const id = "instance-cellar"
	left := leftOnOldPhrase(nil,
		[]fortress.Member{{ID: id, Kind: kitmember.KindInstance, State: group.StateApproved}},
		[]group.Member{{ID: id, Kind: kitmember.KindInstance, Name: "cellar", State: group.StateApproved}},
		nil)
	want := []fortressLeft{{App: "bombvault", Name: "BombVault", Device: "cellar"}}
	if !slices.Equal(left, want) {
		t.Errorf("the change left %+v on the old phrase, want %+v", left, want)
	}
}
