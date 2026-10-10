package api

import (
	"net/http"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

// memberRoles reads the roles in keeps with the other instance from the
// route a card loads, together with the list of who in serves.
func memberRoles(t *testing.T, in, other *instance) (member, self map[string]any) {
	t.Helper()
	code, out := in.do(t, http.MethodGet, "/api/group/roles", nil)
	if code != http.StatusOK || out["ok"] != true {
		t.Fatalf("roles: %d %v", code, out)
	}
	self, _ = out["self"].(map[string]any)
	members, _ := out["members"].([]any)
	for _, m := range members {
		if entry, _ := m.(map[string]any); entry["memberId"] == other.id(t) {
			return entry, self
		}
	}
	t.Fatalf("roles list no member %s: %v", other.id(t), out)
	return nil, nil
}

// roleIn picks one role out of a member's entry: side is byMember or
// forMember, and nil means nobody asked for the role.
func roleIn(entry map[string]any, side, role string) map[string]any {
	s, _ := entry[side].(map[string]any)
	r, _ := s[role].(map[string]any)
	return r
}

func TestARoleGoesFromRequestToRevokeThroughTheSessionRoutes(t *testing.T) {
	for _, role := range []string{store.RoleReceiver, store.RoleFetcher} {
		t.Run(role, func(t *testing.T) {
			tower := newInstance(t, "tower", strings.Repeat("a1", 32))
			attic := newInstance(t, "attic", strings.Repeat("b2", 32))
			pairThroughRelay(t, tower, attic)
			runReceiver(t, attic)
			ask := "/api/group/members/" + attic.id(t) + "/roles/" + role

			if entry, self := memberRoles(t, tower, attic); roleIn(entry, "byMember", role) != nil || entry["asks"] != float64(0) ||
				entry["reachable"] != true || entry["answers"] != true || len(self[role].([]any)) != 0 {
				t.Fatalf("before anyone asked: %v, self %v", entry, self)
			}

			code, out := tower.do(t, http.MethodPut, ask, map[string]any{"sections": []string{"flash", "containers"}})
			sent, _ := out["request"].(map[string]any)
			if code != http.StatusOK || out["ok"] != true || sent["state"] != store.RoleAsked {
				t.Fatalf("ask: %d %v", code, out)
			}
			entry, _ := memberRoles(t, tower, attic)
			if got := roleIn(entry, "byMember", role); got["state"] != store.RoleAsked || roleIn(entry, "forMember", role) != nil {
				t.Fatalf("the asking instance's card = %v", entry)
			}
			entry, self := memberRoles(t, attic, tower)
			waiting := roleIn(entry, "forMember", role)
			if waiting["state"] != store.RoleAsked || entry["asks"] != float64(1) || entry["name"] != "tower" ||
				len(waiting["sections"].([]any)) != 2 || len(self[role].([]any)) != 0 {
				t.Fatalf("the asked instance's card = %v, self %v", entry, self)
			}

			decide := "/api/group/roles/requests/" + waiting["id"].(string)
			code, out = attic.do(t, http.MethodPost, decide, map[string]any{"decision": "allow", "sections": []string{"containers", "flash"}})
			if code != http.StatusOK || out["ok"] != true || out["told"] != true {
				t.Fatalf("allow: %d %v", code, out)
			}
			entry, self = memberRoles(t, attic, tower)
			serves, _ := self[role].([]any)
			if got := roleIn(entry, "forMember", role); got["state"] != store.RoleAllowed || got["decidedBy"] != store.RoleDecidedHere ||
				got["decidedAt"] == "" || entry["asks"] != float64(0) || len(serves) != 1 || serves[0].(map[string]any)["name"] != "tower" {
				t.Fatalf("the asked instance after the allow = %v, self %v", entry, self)
			}
			entry, _ = memberRoles(t, tower, attic)
			if got := roleIn(entry, "byMember", role); got["state"] != store.RoleAllowed || got["decidedBy"] != store.RoleDecidedMember {
				t.Fatalf("the asking instance after the allow = %v", entry)
			}

			code, out = tower.do(t, http.MethodPut, ask, map[string]any{"sections": []string{"containers"}})
			if changed, _ := out["request"].(map[string]any); code != http.StatusOK || changed["state"] != store.RoleAllowed || len(changed["sections"].([]any)) != 1 {
				t.Fatalf("changing the sections of an allowed role: %d %v", code, out)
			}

			if code, out := attic.do(t, http.MethodPost, decide, map[string]any{"decision": "revoke"}); code != http.StatusOK || out["ok"] != true {
				t.Fatalf("revoke: %d %v", code, out)
			}
			entry, _ = memberRoles(t, tower, attic)
			if got := roleIn(entry, "byMember", role); got["state"] != store.RoleRevoked {
				t.Fatalf("the asking instance after the revoke = %v", entry)
			}
			if _, self := memberRoles(t, attic, tower); len(self[role].([]any)) != 0 {
				t.Fatalf("after the revoke this instance still serves %v", self[role])
			}

			code, out = tower.do(t, http.MethodDelete, ask, nil)
			if code != http.StatusOK || out["ok"] != true || out["told"] != true {
				t.Fatalf("withdraw: %d %v", code, out)
			}
			if entry, _ := memberRoles(t, tower, attic); roleIn(entry, "byMember", role) != nil {
				t.Fatalf("the asking instance after it took the request back = %v", entry)
			}
		})
	}
}

func TestARefusalThroughTheSessionRoutesReachesTheAskingCard(t *testing.T) {
	tower := newInstance(t, "tower", strings.Repeat("a1", 32))
	attic := newInstance(t, "attic", strings.Repeat("b2", 32))
	pairThroughRelay(t, tower, attic)
	runReceiver(t, attic)
	for _, role := range []string{store.RoleReceiver, store.RoleFetcher} {
		ask := "/api/group/members/" + attic.id(t) + "/roles/" + role
		if _, out := tower.do(t, http.MethodPut, ask, map[string]any{"sections": []string{"containers"}}); out["ok"] != true {
			t.Fatalf("ask for %s: %v", role, out)
		}
		entry, _ := memberRoles(t, attic, tower)
		id := roleIn(entry, "forMember", role)["id"].(string)
		if _, out := attic.do(t, http.MethodPost, "/api/group/roles/requests/"+id, map[string]any{"decision": "refuse"}); out["ok"] != true {
			t.Fatalf("refuse %s: %v", role, out)
		}
		entry, _ = memberRoles(t, tower, attic)
		if got := roleIn(entry, "byMember", role); got["state"] != store.RoleRefused {
			t.Fatalf("the asking instance after the %s refusal = %v", role, entry)
		}
	}
	if entry, _ := memberRoles(t, attic, tower); entry["asks"] != float64(0) {
		t.Fatalf("refused requests still count as waiting: %v", entry)
	}
	if out := wizardLogin(t, tower, attic); out["ok"] != false {
		t.Fatalf("a login after the Receiver request was refused: %v", out)
	}
}

func TestOnlyTheAskedInstanceAnswersThroughTheSessionRoutes(t *testing.T) {
	tower := newInstance(t, "tower", strings.Repeat("a1", 32))
	attic := newInstance(t, "attic", strings.Repeat("b2", 32))
	pairThroughRelay(t, tower, attic)
	sections := []string{"containers"}
	sent := askForRole(t, tower, attic, store.RoleReceiver, "containers")
	got, _ := roleWith(t, attic, store.RoleRequestIn, tower, store.RoleReceiver)
	allow := map[string]any{"decision": "allow", "sections": sections}

	if code, out := tower.do(t, http.MethodPost, "/api/group/roles/requests/"+sent.ID, allow); code != http.StatusNotFound || out["ok"] != false {
		t.Fatalf("the asking instance allowing its own request: %d %v", code, out)
	}
	if code, _ := attic.do(t, http.MethodPost, "/api/group/roles/requests/"+strings.Repeat("0", 32), allow); code != http.StatusNotFound {
		t.Fatalf("an unknown request answered %d, want 404", code)
	}
	if code, _ := attic.do(t, http.MethodPost, "/api/group/roles/requests/not-an-id", allow); code != http.StatusBadRequest {
		t.Fatalf("a malformed id answered %d, want 400", code)
	}

	decide := "/api/group/roles/requests/" + got.ID
	if _, out := attic.do(t, http.MethodPost, decide, allow); out["ok"] != false || out["code"] != "no-receiver" {
		t.Fatalf("allowing a Receiver without a receiver: %v", out)
	}
	runReceiver(t, attic)
	if _, out := attic.do(t, http.MethodPost, decide, map[string]any{"decision": "allow"}); out["ok"] != false {
		t.Fatalf("an allow that names no sections: %v", out)
	}
	if _, out := attic.do(t, http.MethodPost, decide, map[string]any{"decision": "allow", "sections": []string{"vms"}}); out["code"] != "request-changed" {
		t.Fatalf("an allow for other sections than asked: %v", out)
	}
	if _, out := attic.do(t, http.MethodPost, decide, map[string]any{"decision": "grant"}); out["ok"] != false {
		t.Fatalf("an unknown decision: %v", out)
	}
	if _, out := attic.do(t, http.MethodPost, decide, map[string]any{"decision": "revoke"}); out["ok"] != false {
		t.Fatalf("revoking a request nobody allowed: %v", out)
	}
	if still, _ := roleWith(t, attic, store.RoleRequestIn, tower, store.RoleReceiver); still.State != store.RoleAsked {
		t.Fatalf("a refused answer changed the request: %+v", still)
	}
	for i := range 2 {
		if _, out := attic.do(t, http.MethodPost, decide, allow); out["ok"] != true {
			t.Fatalf("allow number %d: %v", i+1, out)
		}
	}
}

func TestAskingThroughTheSessionRouteIsChecked(t *testing.T) {
	tower := newInstance(t, "tower", strings.Repeat("a1", 32))
	attic := newInstance(t, "attic", strings.Repeat("b2", 32))
	pairThroughRelay(t, tower, attic)
	member := "/api/group/members/" + attic.id(t) + "/roles/"
	sections := map[string]any{"sections": []string{"containers"}}

	for name, call := range map[string]struct {
		path string
		body map[string]any
	}{
		"an unknown role":              {member + "admin", sections},
		"no sections":                  {member + store.RoleFetcher, map[string]any{"sections": []string{}}},
		"an unknown section":           {member + store.RoleFetcher, map[string]any{"sections": []string{"settings"}}},
		"an unknown store":             {member + store.RoleReceiver, map[string]any{"sections": []string{"containers"}, "store": "tape"}},
		"an instance out of the group": {"/api/group/members/" + strings.Repeat("0", 32) + "/roles/" + store.RoleFetcher, sections},
		"this instance itself":         {"/api/group/members/" + tower.id(t) + "/roles/" + store.RoleFetcher, sections},
	} {
		if _, out := tower.do(t, http.MethodPut, call.path, call.body); out["ok"] != false {
			t.Errorf("asking with %s = %v, want it refused", name, out)
		}
	}
	if _, out := tower.do(t, http.MethodDelete, member+"admin", nil); out["ok"] != false {
		t.Errorf("withdrawing an unknown role = %v, want it refused", out)
	}
	if roleRequestCount(t, tower) != 0 || roleRequestCount(t, attic) != 0 {
		t.Fatal("a refused ask left a request behind")
	}
}

func TestTheRoleRoutesNeedASession(t *testing.T) {
	tower := newInstance(t, "tower", strings.Repeat("a1", 32))
	tower.session = ""
	for _, call := range []struct{ method, path string }{
		{http.MethodGet, "/api/group/roles"},
		{http.MethodPut, "/api/group/members/" + strings.Repeat("0", 32) + "/roles/fetcher"},
		{http.MethodDelete, "/api/group/members/" + strings.Repeat("0", 32) + "/roles/fetcher"},
		{http.MethodPost, "/api/group/roles/requests/" + strings.Repeat("0", 32)},
	} {
		if code, _ := tower.do(t, call.method, call.path, map[string]any{}); code != http.StatusUnauthorized {
			t.Errorf("%s %s without a session answered %d, want 401", call.method, call.path, code)
		}
	}
}

func TestTheZFSServerRoleReadsFromTheReplicaRequests(t *testing.T) {
	tower := newInstance(t, "tower", strings.Repeat("a1", 32))
	attic := newInstance(t, "attic", strings.Repeat("b2", 32))
	pairThroughRelay(t, tower, attic)

	appdata, err := tower.st.CreateZFSDataset(store.ZFSDataset{Dataset: "cache/appdata", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	domains, err := tower.st.CreateZFSDataset(store.ZFSDataset{Dataset: "cache/domains", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range []store.ZFSDataset{appdata, domains} {
		if err := tower.st.SetZFSReplicaTarget(item.ID, store.ZFSReplicaTargetPeer, attic.id(t)); err != nil {
			t.Fatal(err)
		}
	}
	if err := tower.st.SetZFSReplicaPeer(appdata.ID, store.ZFSReplicaPeer{State: store.ZFSReceiveAllowed}); err != nil {
		t.Fatal(err)
	}
	entry, _ := memberRoles(t, tower, attic)
	zfsRole := roleIn(entry, "byMember", "zfs")
	if zfsRole["state"] != store.ZFSReceiveAsked || len(zfsRole["items"].([]any)) != 2 {
		t.Fatalf("with one item allowed and one unanswered = %v, want the role waiting", zfsRole)
	}
	if err := tower.st.SetZFSReplicaPeer(domains.ID, store.ZFSReplicaPeer{State: store.ZFSReceiveRefused}); err != nil {
		t.Fatal(err)
	}
	entry, _ = memberRoles(t, tower, attic)
	if zfsRole := roleIn(entry, "byMember", "zfs"); zfsRole["state"] != store.ZFSReceiveAllowed {
		t.Fatalf("with one item allowed and one refused = %v, want the role active", zfsRole)
	}

	slot, err := attic.st.AskZFSReceive(store.ZFSReceiveSlot{
		PeerID: tower.id(t), PeerName: "tower", ItemID: appdata.ID, Dataset: "cache/appdata",
		SourceServer: "tower", Members: []string{"cache/appdata"},
	}, false)
	if err != nil {
		t.Fatal(err)
	}
	entry, self := memberRoles(t, attic, tower)
	if got := roleIn(entry, "forMember", "zfs"); got["state"] != store.ZFSReceiveAsked || entry["asks"] != float64(1) || len(self["zfs"].([]any)) != 0 {
		t.Fatalf("a replica request waiting here = %v, self %v", entry, self)
	}
	if _, err := attic.st.DecideZFSReceive(slot.ID, store.ZFSReceiveDecision{State: store.ZFSReceiveAllowed, Pool: "tank", Root: "tank/replica"}); err != nil {
		t.Fatal(err)
	}
	entry, self = memberRoles(t, attic, tower)
	if got := roleIn(entry, "forMember", "zfs"); got["state"] != store.ZFSReceiveAllowed || entry["asks"] != float64(0) || len(self["zfs"].([]any)) != 1 {
		t.Fatalf("an allowed replica request = %v, self %v", entry, self)
	}
}

func TestTheRolesListKeepsAMemberThatIsOutOfReach(t *testing.T) {
	attic := newInstance(t, "attic", strings.Repeat("b2", 32))
	const gone = "0123456789abcdef0123456789abcdef"
	if _, err := attic.st.AskRoleRequest(store.RoleRequest{MemberID: gone, MemberName: "tower", Role: store.RoleFetcher, Sections: []string{"containers"}}, true); err != nil {
		t.Fatal(err)
	}
	_, out := attic.do(t, http.MethodGet, "/api/group/roles", nil)
	members, _ := out["members"].([]any)
	if len(members) != 1 {
		t.Fatalf("members = %v, want the one a request names", out)
	}
	entry := members[0].(map[string]any)
	if entry["memberId"] != gone || entry["name"] != "tower" || entry["reachable"] != false || entry["asks"] != float64(1) {
		t.Fatalf("the member out of reach = %v", entry)
	}
}
