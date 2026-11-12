package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/junkerderprovinz/parleyport/kit/fortress"
	"github.com/junkerderprovinz/parleyport/kit/phrase"
	"github.com/junkerderprovinz/parleyport/kit/relay"
	"github.com/junkerderprovinz/parleyport/kit/status"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

func createGroupPhrase(t *testing.T, in *instance) string {
	t.Helper()
	code, out := in.do(t, http.MethodPost, "/api/group/phrase", nil)
	words, _ := out["phrase"].(string)
	if code != http.StatusOK || len(strings.Fields(words)) != 12 {
		t.Fatalf("create phrase: %d %v", code, out)
	}
	return words
}

func groupSecretOf(t *testing.T, in *instance) []byte {
	t.Helper()
	g, err := in.st.GetGroupState()
	if err != nil {
		t.Fatal(err)
	}
	sec, err := in.svc.groupSecret(g)
	if err != nil {
		t.Fatal(err)
	}
	return sec
}

func TestTheGroupsPhraseIsTheFortressPhraseToo(t *testing.T) {
	in := newInstance(t, "cellar", strings.Repeat("a1", 32))
	words := createGroupPhrase(t, in)
	f := in.svc.fortressNow()
	if f == nil {
		t.Fatal("a new phrase opened no fortress")
	}
	if got, err := in.svc.fortressPhrase(); err != nil || got != words {
		t.Fatalf("the fortress phrase is %q, %v; want the group's", got, err)
	}
	if f.Pairing() != nil {
		t.Fatal("on the group's own phrase the fortress opened a pairing beside the group")
	}
	if !in.svc.pairing().Active() || in.svc.pairing().Following() {
		t.Fatal("the group stopped running on its own phrase")
	}
}

func TestAnotherAppGivenTheGroupsWordsMeetsBombVault(t *testing.T) {
	rs := httptest.NewServer(relay.New())
	t.Cleanup(rs.Close)
	in := newInstance(t, "cellar", strings.Repeat("a1", 32))
	in.do(t, http.MethodPut, "/api/group/relay", map[string]any{"mode": "own", "url": rs.URL})
	words := createGroupPhrase(t, in)
	sec, err := phrase.Decode(words)
	if err != nil {
		t.Fatal(err)
	}
	p := joinPlugin(t, rs.URL, sec)
	p.open(t)
	waitLong(t, "the other app to meet BombVault", p.reaches)
}

func TestAGroupFromBeforeTheFortressTakesItsPhraseOnStart(t *testing.T) {
	in := newInstance(t, "cellar", strings.Repeat("a1", 32))
	words, sec := newPhrase(t)
	if err := in.h.storeGroupSecret(sec); err != nil {
		t.Fatal(err)
	}
	if in.svc.fortressNow() != nil {
		t.Fatal("storing the group's secret alone opened a fortress")
	}
	if err := in.svc.OpenFortress(nil); err != nil {
		t.Fatal(err)
	}
	if got, err := in.svc.fortressPhrase(); err != nil || got != words {
		t.Fatalf("after the start the fortress phrase is %q, %v; want the group's", got, err)
	}
}

func TestLeavingThePairingLeavesTheFortressAndTheOtherWayRound(t *testing.T) {
	in := newInstance(t, "cellar", strings.Repeat("a1", 32))
	createGroupPhrase(t, in)
	if code, out := in.do(t, http.MethodDelete, "/api/group", nil); code != http.StatusOK || out["ok"] != true {
		t.Fatalf("leave the group: %d %v", code, out)
	}
	if m, _ := in.st.FortressMembership(); m.Joined() || in.svc.fortressNow() != nil {
		t.Fatal("leaving the group kept its fortress")
	}

	createGroupPhrase(t, in)
	if code, out := in.do(t, http.MethodDelete, "/api/fortress", nil); code != http.StatusOK || out["ok"] != true {
		t.Fatalf("leave the fortress: %d %v", code, out)
	}
	if in.svc.pairing().Active() || groupSecretOf(t, in) != nil {
		t.Fatal("leaving the fortress kept the group on its phrase")
	}
}

func TestEnteringOtherWordsTakesBombVaultThere(t *testing.T) {
	in := newInstance(t, "cellar", strings.Repeat("a1", 32))
	createGroupPhrase(t, in)
	words, sec := newPhrase(t)
	if code, out := in.do(t, http.MethodPost, "/api/group/join", map[string]any{"phrase": words}); code != http.StatusOK || out["active"] != true {
		t.Fatalf("join: %d %v", code, out)
	}
	if got, err := in.svc.fortressPhrase(); err != nil || got != words {
		t.Fatalf("the fortress phrase is %q, %v; want the words entered", got, err)
	}
	if got := groupSecretOf(t, in); string(got) != string(sec) {
		t.Fatal("the group did not take the words entered")
	}
}

func TestEnteringAFortressPhraseOnTheGroupsOwnMovesTheGroupThere(t *testing.T) {
	in := newInstance(t, "cellar", strings.Repeat("a1", 32))
	createGroupPhrase(t, in)
	words, _ := newPhrase(t)
	if code, out := in.do(t, http.MethodPost, "/api/fortress/join", map[string]any{"phrase": words}); code != http.StatusOK || out["ok"] != true {
		t.Fatalf("join the fortress: %d %v", code, out)
	}
	if got, err := in.svc.fortressPhrase(); err != nil || got != words {
		t.Fatalf("the fortress phrase is %q, %v", got, err)
	}
	waitLong(t, "the group to run on the fortress phrase", in.svc.pairing().Following)
}

func TestTheInstanceGoesByItsNameOrElseTheUnraidServers(t *testing.T) {
	in := newInstance(t, "", strings.Repeat("a1", 32))
	in.svc.cfg.FlashDir = t.TempDir()
	if got := in.svc.pairName(); got != "BombVault" {
		t.Fatalf("without a name or a flash the instance is %q", got)
	}
	writeUnraidIdent(t, in.svc.cfg.FlashDir, "Tower")
	if got := in.svc.pairName(); got != "Tower" {
		t.Fatalf("on Unraid without a name the instance is %q", got)
	}

	createGroupPhrase(t, in)
	if _, err := in.st.MutateSettings(func(s *store.Settings) error {
		s.InstanceName = "cellar"
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	in.svc.renamed()
	deadline := time.Now().Add(5 * time.Second)
	for {
		in.svc.house.mu.Lock()
		device := in.svc.house.device
		in.svc.house.mu.Unlock()
		if device == "cellar" {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("after the rename the fortress knows the instance as %q", device)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// iconSource is the status of an app that shows its logo and nothing else.
type iconSource struct{ icon string }

func (s iconSource) Report(context.Context, string) (status.Report, error) {
	return status.Report{Contract: status.Contract, App: status.App{ID: "firesquire", Name: "FireSquire", Version: "3.1.2", Icon: s.icon}}, nil
}

func (iconSource) Log(context.Context, status.LogQuery) (status.LogPage, error) {
	return status.LogPage{}, nil
}

func TestTheFortressShowsTheLogoAnAppPublishes(t *testing.T) {
	rs := httptest.NewServer(relay.New())
	t.Cleanup(rs.Close)
	in := newInstance(t, "cellar", strings.Repeat("a1", 32))
	in.do(t, http.MethodPut, "/api/group/relay", map[string]any{"mode": "own", "url": rs.URL})
	words := createGroupPhrase(t, in)
	sec, err := phrase.Decode(words)
	if err != nil {
		t.Fatal(err)
	}
	icon, err := status.IconFromSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 8 8"><circle cx="4" cy="4" r="4"/></svg>`))
	if err != nil {
		t.Fatal(err)
	}
	p := joinPlugin(t, rs.URL, sec)
	cfg, err := fortress.Load(p.dir)
	if err != nil {
		t.Fatal(err)
	}
	cfg.App = fortress.App{ID: "firesquire", Name: "FireSquire", Version: "3.1.2"}
	cfg.Status = iconSource{icon: icon}
	if p.f, err = fortress.Open(cfg); err != nil {
		t.Fatal(err)
	}
	waitLong(t, "the other app's logo on the page", func() bool {
		_, out := in.do(t, http.MethodGet, "/api/fortress", nil)
		members, _ := out["members"].([]any)
		for _, m := range members {
			if mm, _ := m.(map[string]any); mm["app"] == "firesquire" && mm["logo"] == icon {
				return true
			}
		}
		return false
	})
}
