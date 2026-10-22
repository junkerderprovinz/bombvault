package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/relay"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// pairAndClose pairs a with b and shuts b down, the way a desktop or a
// server that is switched off leaves the group's relay.
func pairAndClose(t *testing.T) (a, b *instance) {
	t.Helper()
	a = newInstance(t, "cellar", strings.Repeat("a1", 32))
	b = newInstance(t, "attic", strings.Repeat("b2", 32))
	pairThroughRelay(t, a, b)
	if _, out := a.do(t, http.MethodGet, "/api/group", nil); len(out["away"].([]any)) != 0 {
		t.Fatalf("away while b is there = %v, want none", out["away"])
	}
	b.svc.StopGroup()
	deadline := time.Now().Add(10 * time.Second)
	for len(a.svc.pairing().Members()) > 0 {
		if time.Now().After(deadline) {
			t.Fatalf("a still sees %+v after b was shut down", a.svc.pairing().Members())
		}
		time.Sleep(20 * time.Millisecond)
	}
	return a, b
}

func awayIn(t *testing.T, out map[string]any) []map[string]any {
	t.Helper()
	list, ok := out["away"].([]any)
	if !ok {
		t.Fatalf("no away list in %v", out)
	}
	var away []map[string]any
	for _, m := range list {
		away = append(away, m.(map[string]any))
	}
	return away
}

func TestAnInstanceThatWasShutDownStaysInTheGroupAsAway(t *testing.T) {
	a, b := pairAndClose(t)
	_, out := a.do(t, http.MethodGet, "/api/group", nil)
	away := awayIn(t, out)
	if len(away) != 1 || away[0]["id"] != b.id(t) || away[0]["name"] != "attic" {
		t.Fatalf("away = %v, want attic", away)
	}
	if seen, _ := away[0]["lastSeen"].(float64); seen <= 0 || int64(seen) > time.Now().Unix() {
		t.Fatalf("attic last seen at %v, want a time up to now", away[0]["lastSeen"])
	}
}

func TestEnteringAnotherGroupsPhraseLeavesTheFirstGroupOnBothSides(t *testing.T) {
	a := newInstance(t, "cellar", strings.Repeat("a1", 32))
	b := newInstance(t, "attic", strings.Repeat("b2", 32))
	c := newInstance(t, "barn", strings.Repeat("c3", 32))
	pairThroughRelay(t, a, b)
	_, created := c.do(t, http.MethodPost, "/api/group/phrase", nil)
	phrase, _ := created["phrase"].(string)

	a.do(t, http.MethodGet, "/api/group", nil)
	b.do(t, http.MethodGet, "/api/group", nil)
	if !hasFleetRow(t, a, b) || !hasFleetRow(t, b, a) {
		t.Fatal("setup: the two have no rows for each other")
	}
	if code, out := b.do(t, http.MethodPost, "/api/group/join", map[string]any{"phrase": phrase}); code != http.StatusOK || out["ok"] != true {
		t.Fatalf("join: %d %v", code, out)
	}
	if _, out := b.do(t, http.MethodGet, "/api/group", nil); len(awayIn(t, out)) != 0 {
		t.Fatalf("away on attic after it joined another group = %v, want none", out["away"])
	}
	if hasFleetRow(t, a, b) {
		t.Fatal("cellar kept its row for attic, so attic stays away there")
	}
	if _, out := a.do(t, http.MethodGet, "/api/group", nil); len(awayIn(t, out)) != 0 {
		t.Fatalf("away on cellar after attic left = %v, want none", out["away"])
	}
}

func TestEnteringTheSamePhraseAgainKeepsTheMembers(t *testing.T) {
	a := newInstance(t, "cellar", strings.Repeat("a1", 32))
	b := newInstance(t, "attic", strings.Repeat("b2", 32))
	pairThroughRelay(t, a, b)
	code, shown := a.do(t, http.MethodPost, "/api/group/phrase/show", map[string]any{"password": testPassword})
	phrase, _ := shown["phrase"].(string)
	if code != http.StatusOK || phrase == "" {
		t.Fatalf("show: %d %v", code, shown)
	}
	b.do(t, http.MethodGet, "/api/group", nil)
	if !hasFleetRow(t, b, a) {
		t.Fatal("setup: attic has no row for cellar")
	}
	if code, out := b.do(t, http.MethodPost, "/api/group/join", map[string]any{"phrase": phrase}); code != http.StatusOK || out["ok"] != true {
		t.Fatalf("join again: %d %v", code, out)
	}
	if !hasFleetRow(t, b, a) {
		t.Fatal("entering its own group's phrase again dropped attic's row for cellar")
	}
}

func hasFleetRow(t *testing.T, in, of *instance) bool {
	t.Helper()
	rows, err := in.st.ListFleetPeers()
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range rows {
		if p.MemberID == of.id(t) {
			return true
		}
	}
	return false
}

func TestAnAwayInstanceIsStillAwayAfterARestart(t *testing.T) {
	a, b := pairAndClose(t)
	a.svc.StopGroup()
	restarted := NewService(a.svc.cfg, a.st, nil, nil, a.engine)
	t.Cleanup(restarted.StopGroup)
	restarted.applyGroup()
	members, away, settling, err := restarted.groupRoster(time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if len(members) != 0 || len(away) != 1 || away[0].ID != b.id(t) {
		t.Fatalf("after a restart members = %+v, away = %+v; want attic away", members, away)
	}
	if !settling {
		t.Fatal("right after a restart the group should still be settling")
	}
}

func TestRemovingAnAwayInstanceTakesItOffForGood(t *testing.T) {
	a, b := pairAndClose(t)
	if code, out := a.do(t, http.MethodDelete, "/api/group/members/"+b.id(t), nil); code != http.StatusOK || out["ok"] != true {
		t.Fatalf("remove: %d %v", code, out)
	}
	if _, out := a.do(t, http.MethodGet, "/api/group", nil); len(awayIn(t, out)) != 0 {
		t.Fatalf("away after remove = %v, want none", out["away"])
	}
	restarted := NewService(a.svc.cfg, a.st, nil, nil, a.engine)
	t.Cleanup(restarted.StopGroup)
	restarted.applyGroup()
	if _, away, _, err := restarted.groupRoster(time.Now()); err != nil || len(away) != 0 {
		t.Fatalf("away after remove and restart = %+v, %v; want none", away, err)
	}
	if _, found, err := a.st.MemberKey(b.id(t)); err != nil || found {
		t.Fatalf("the removed member's key is still pinned: found=%v err=%v", found, err)
	}
}

func TestRemovingAnAwayInstanceEndsItsRoleRequests(t *testing.T) {
	a := newInstance(t, "cellar", strings.Repeat("a1", 32))
	b := newInstance(t, "attic", strings.Repeat("b2", 32))
	pairThroughRelay(t, a, b)
	askForRole(t, a, b, store.RoleFetcher, "containers")
	req, _ := roleWith(t, b, store.RoleRequestIn, a, store.RoleFetcher)
	answerRole(t, b, req.ID, store.RoleAllowed)
	b.svc.StopGroup()
	waitFor(t, "cellar to lose attic", func() bool { _, ok := a.svc.reachableMember(b.id(t)); return !ok })

	if code, out := a.do(t, http.MethodDelete, "/api/group/members/"+b.id(t), nil); code != http.StatusOK || out["ok"] != true {
		t.Fatalf("remove: %d %v", code, out)
	}
	members, _, err := a.svc.rolesOverview()
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range members {
		if m.MemberID == b.id(t) {
			t.Fatalf("the roles still name the removed instance: %+v", m)
		}
	}
	if pending, err := a.st.ListRoleWithdrawals(); err != nil || len(pending) != 1 || pending[0].MemberID != b.id(t) {
		t.Fatalf("withdrawals to tell attic = %+v, %v; want the Fetcher request", pending, err)
	}
}

func TestAnInstanceThatComesBackIsListedOnce(t *testing.T) {
	a := newInstance(t, "cellar", strings.Repeat("a1", 32))
	b := newInstance(t, "attic", strings.Repeat("b2", 32))
	pairThroughRelay(t, a, b)
	const barn = "0123456789abcdef0123456789abcdef"
	if _, err := a.st.CreateFleetPeer(store.FleetPeer{MemberID: barn, Name: "barn", Enabled: true, LastSeen: 1}); err != nil {
		t.Fatal(err)
	}
	if _, away, _, err := a.svc.groupRoster(time.Now()); err != nil || len(away) != 1 || away[0].ID != barn {
		t.Fatalf("away before barn is back = %+v, %v; want barn", away, err)
	}

	a.svc.pairing().ConfirmAddress(barn, "https://192.0.2.7:3443", "barn", "")
	members, away, _, err := a.svc.groupRoster(time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if len(away) != 0 {
		t.Fatalf("away once barn is back = %+v, want none", away)
	}
	listed := 0
	for _, m := range members {
		if m.ID == barn {
			listed++
		}
	}
	rows, err := a.st.ListFleetPeers()
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range rows {
		if p.MemberID == barn {
			listed++
			if p.LastSeen <= 1 {
				t.Fatalf("barn's row = %+v, want it seen again", p)
			}
		}
	}
	if listed != 2 {
		t.Fatalf("barn is listed %d times over members and rows, want once in each", listed)
	}
}

func TestOnlyAMemberThatWasInReachIsRemembered(t *testing.T) {
	a := newInstance(t, "cellar", strings.Repeat("a1", 32))
	// A row from before pairing names no member, and outside a group there
	// is nobody to be away.
	if _, err := a.st.CreateFleetPeer(store.FleetPeer{Name: "barn", URL: "https://barn.lan", Enabled: true}); err != nil {
		t.Fatal(err)
	}
	if _, out := a.do(t, http.MethodGet, "/api/group", nil); len(awayIn(t, out)) != 0 {
		t.Fatalf("away outside a group = %v, want none", out["away"])
	}
	b := newInstance(t, "attic", strings.Repeat("b2", 32))
	pairThroughRelay(t, a, b)
	if _, out := a.do(t, http.MethodGet, "/api/group", nil); len(awayIn(t, out)) != 0 {
		t.Fatalf("away with everyone in reach = %v, want none", out["away"])
	}
}

func TestAMemberAsksForTheGroupAndHearsWhoIsAway(t *testing.T) {
	a, b := pairAndClose(t)
	status, body := a.svc.servePeer(context.Background(), relay.ProxyCall{Method: http.MethodGet, Path: "/api/group/peer/members", Sender: strings.Repeat("c3", 16)})
	if status != http.StatusOK {
		t.Fatalf("members call: %d %s", status, body)
	}
	var ans peerMembersAnswer
	if err := json.Unmarshal(body, &ans); err != nil {
		t.Fatal(err)
	}
	if !ans.OK || len(ans.Members) != 0 || len(ans.Away) != 1 || ans.Away[0].ID != b.id(t) || ans.Away[0].Name != "attic" {
		t.Fatalf("a member hears %+v, want attic away", ans)
	}
}
