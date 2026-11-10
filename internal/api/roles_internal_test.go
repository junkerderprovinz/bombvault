package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

// askForRole has asker ask the other instance for a role the way a person on
// asker would, and returns the request as asker stores it.
func askForRole(t *testing.T, asker, asked *instance, role string, sections ...string) store.RoleRequest {
	t.Helper()
	m, ok := asker.svc.reachableMember(asked.id(t))
	if !ok {
		t.Fatal("the asked instance is not in reach")
	}
	sections, where, msg := checkRoleAsk(role, sections, "")
	if msg != "" {
		t.Fatal(msg)
	}
	req, err := asker.svc.askRole(context.Background(), m, role, sections, where)
	if err != nil {
		t.Fatalf("ask for %s: %v", role, err)
	}
	return req
}

// roleWith is the request in holds for role with the other instance in one
// direction, and whether it holds one.
func roleWith(t *testing.T, in *instance, direction string, other *instance, role string) (store.RoleRequest, bool) {
	t.Helper()
	req, ok, err := in.st.FindRoleRequest(direction, other.id(t), role)
	if err != nil {
		t.Fatal(err)
	}
	return req, ok
}

// allowRole has asker ask granter for a role and a person on granter allow it.
func allowRole(t *testing.T, asker, granter *instance, role string, sections ...string) store.RoleRequest {
	t.Helper()
	askForRole(t, asker, granter, role, sections...)
	req, ok := roleWith(t, granter, store.RoleRequestIn, asker, role)
	if !ok {
		t.Fatalf("the %s request did not arrive", role)
	}
	return answerRole(t, granter, req.ID, store.RoleAllowed)
}

func answerRole(t *testing.T, in *instance, id, state string) store.RoleRequest {
	t.Helper()
	req, told, err := in.svc.decideRole(context.Background(), id, store.RoleDecision{State: state})
	if err != nil || !told {
		t.Fatalf("answer %s: told %v, %v", state, told, err)
	}
	return req
}

// runReceiver gives in a receiver whose container runs, the way setting one
// up leaves it.
func runReceiver(t *testing.T, in *instance) {
	t.Helper()
	in.svc.cfg.HostMountRoot = t.TempDir()
	rs := store.ReceiverServer{ContainerName: "rest-server", Folder: "restic", Port: 8001, User: "outsider", Host: "192.168.1.20"}
	if err := in.st.SaveReceiverServer(rs); err != nil {
		t.Fatal(err)
	}
	in.svc.docker = &receiverDocker{live: map[string]bool{"rest-server": true}}
}

// peerCallAs sends in one peer call that the group channel says came from
// sender, and decodes the answer.
func peerCallAs(t *testing.T, in *instance, sender, path string, body any) map[string]any {
	t.Helper()
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	status, answer := in.svc.callPeer(context.Background(), peerCall{Method: http.MethodPost, Path: path, Body: raw, Sender: sender})
	var out map[string]any
	if err := json.Unmarshal(answer, &out); err != nil {
		t.Fatalf("%s answered %d unreadably: %s", path, status, answer)
	}
	return out
}

func roleRequestCount(t *testing.T, in *instance) int {
	t.Helper()
	all, err := in.st.ListRoleRequests()
	if err != nil {
		t.Fatal(err)
	}
	return len(all)
}

func TestARoleRequestWaitsOnBothSidesUntilAPersonAllowsIt(t *testing.T) {
	for _, role := range []string{store.RoleReceiver, store.RoleFetcher} {
		t.Run(role, func(t *testing.T) {
			tower := newInstance(t, "tower", strings.Repeat("a1", 32))
			attic := newInstance(t, "attic", strings.Repeat("b2", 32))
			pairThroughRelay(t, tower, attic)
			runReceiver(t, attic)

			sent := askForRole(t, tower, attic, role, "flash", "containers")
			if sent.State != store.RoleAsked || sent.Direction != store.RoleRequestOut || sent.MemberName != "attic" {
				t.Fatalf("the request as the asking instance keeps it = %+v", sent)
			}
			got, ok := roleWith(t, attic, store.RoleRequestIn, tower, role)
			if !ok || got.State != store.RoleAsked || got.MemberName != "tower" || !slices.Equal(got.Sections, []string{"containers", "flash"}) {
				t.Fatalf("the request as the asked instance keeps it = %+v, %v", got, ok)
			}

			allowed := answerRole(t, attic, got.ID, store.RoleAllowed)
			if allowed.State != store.RoleAllowed || allowed.DecidedBy != store.RoleDecidedHere {
				t.Fatalf("allowed request = %+v", allowed)
			}
			mirror, _ := roleWith(t, tower, store.RoleRequestOut, attic, role)
			if mirror.State != store.RoleAllowed || mirror.DecidedAt == 0 || mirror.DecidedBy != store.RoleDecidedMember {
				t.Fatalf("the asking instance after the allow = %+v", mirror)
			}
		})
	}
}

func TestARefusalAndARevokeReachTheAskingInstance(t *testing.T) {
	tower := newInstance(t, "tower", strings.Repeat("a1", 32))
	attic := newInstance(t, "attic", strings.Repeat("b2", 32))
	pairThroughRelay(t, tower, attic)
	runReceiver(t, attic)

	askForRole(t, tower, attic, store.RoleFetcher, "containers")
	fetch, _ := roleWith(t, attic, store.RoleRequestIn, tower, store.RoleFetcher)
	answerRole(t, attic, fetch.ID, store.RoleRefused)
	if mirror, _ := roleWith(t, tower, store.RoleRequestOut, attic, store.RoleFetcher); mirror.State != store.RoleRefused {
		t.Fatalf("the asking instance after the refusal = %+v", mirror)
	}
	if again := askForRole(t, tower, attic, store.RoleFetcher, "containers"); again.State != store.RoleRefused {
		t.Fatalf("asking again after a refusal = %+v, want it refused", again)
	}

	askForRole(t, tower, attic, store.RoleReceiver, "containers")
	recv, _ := roleWith(t, attic, store.RoleRequestIn, tower, store.RoleReceiver)
	answerRole(t, attic, recv.ID, store.RoleAllowed)
	answerRole(t, attic, recv.ID, store.RoleRevoked)
	if mirror, _ := roleWith(t, tower, store.RoleRequestOut, attic, store.RoleReceiver); mirror.State != store.RoleRevoked {
		t.Fatalf("the asking instance after the revoke = %+v", mirror)
	}
	if again := askForRole(t, tower, attic, store.RoleReceiver, "containers"); again.State != store.RoleAsked {
		t.Fatalf("a person asking anew after a revoke = %+v, want it waiting", again)
	}
}

func TestAnsweringTwiceChangesNothing(t *testing.T) {
	tower := newInstance(t, "tower", strings.Repeat("a1", 32))
	attic := newInstance(t, "attic", strings.Repeat("b2", 32))
	pairThroughRelay(t, tower, attic)

	askForRole(t, tower, attic, store.RoleFetcher, "containers")
	again := askForRole(t, tower, attic, store.RoleFetcher, "containers")
	req, _ := roleWith(t, attic, store.RoleRequestIn, tower, store.RoleFetcher)
	if again.State != store.RoleAsked || roleRequestCount(t, attic) != 1 || roleRequestCount(t, tower) != 1 {
		t.Fatalf("asking twice left %d and %d requests", roleRequestCount(t, tower), roleRequestCount(t, attic))
	}

	first := answerRole(t, attic, req.ID, store.RoleAllowed)
	second := answerRole(t, attic, req.ID, store.RoleAllowed)
	if second.State != first.State || second.DecidedAt != first.DecidedAt || second.DecidedBy != first.DecidedBy {
		t.Fatalf("a second allow = %+v, want %+v", second, first)
	}
	if _, _, err := attic.svc.decideRole(context.Background(), req.ID, store.RoleDecision{State: store.RoleRefused}); !errors.Is(err, store.ErrRoleRequestMove) {
		t.Fatalf("refusing an allowed request = %v, want ErrRoleRequestMove", err)
	}
}

func TestWithdrawingARoleRequestEndsItOnBothSides(t *testing.T) {
	tower := newInstance(t, "tower", strings.Repeat("a1", 32))
	attic := newInstance(t, "attic", strings.Repeat("b2", 32))
	pairThroughRelay(t, tower, attic)

	askForRole(t, tower, attic, store.RoleFetcher, "containers")
	told, err := tower.svc.withdrawRole(context.Background(), attic.id(t), store.RoleFetcher)
	if err != nil || !told {
		t.Fatalf("withdraw: told %v, %v", told, err)
	}
	if roleRequestCount(t, tower) != 0 || roleRequestCount(t, attic) != 0 {
		t.Fatal("a withdrawn request nobody answered is still stored")
	}
	if told, err := tower.svc.withdrawRole(context.Background(), attic.id(t), store.RoleFetcher); err != nil || !told {
		t.Fatalf("a second withdraw: told %v, %v", told, err)
	}

	askForRole(t, tower, attic, store.RoleFetcher, "containers")
	req, _ := roleWith(t, attic, store.RoleRequestIn, tower, store.RoleFetcher)
	answerRole(t, attic, req.ID, store.RoleAllowed)
	if _, err := tower.svc.withdrawRole(context.Background(), attic.id(t), store.RoleFetcher); err != nil {
		t.Fatal(err)
	}
	if _, ok := roleWith(t, tower, store.RoleRequestOut, attic, store.RoleFetcher); ok {
		t.Fatal("the asking instance still holds the request it took back")
	}
	ended, _ := roleWith(t, attic, store.RoleRequestIn, tower, store.RoleFetcher)
	if ended.State != store.RoleRevoked || ended.DecidedBy != store.RoleDecidedMember {
		t.Fatalf("the asked instance after the withdrawal = %+v, want it revoked by the member", ended)
	}
}

func TestAWithdrawalTheMemberMissedReachesItOnceItIsBack(t *testing.T) {
	tower := newInstance(t, "tower", strings.Repeat("a1", 32))
	attic := newInstance(t, "attic", strings.Repeat("b2", 32))
	pairThroughRelay(t, tower, attic)
	askForRole(t, tower, attic, store.RoleFetcher, "containers")
	req, _ := roleWith(t, attic, store.RoleRequestIn, tower, store.RoleFetcher)
	answerRole(t, attic, req.ID, store.RoleAllowed)

	attic.svc.StopGroup()
	waitFor(t, "tower to lose attic", func() bool { _, ok := tower.svc.reachableMember(attic.id(t)); return !ok })
	if told, err := tower.svc.withdrawRole(context.Background(), attic.id(t), store.RoleFetcher); err != nil || told {
		t.Fatalf("withdraw while attic is away: told %v, %v", told, err)
	}

	attic.svc.applyGroup()
	waitFor(t, "tower to reach attic again", func() bool { _, ok := tower.svc.reachableMember(attic.id(t)); return ok })
	waitFor(t, "attic to reach tower again", func() bool { _, ok := attic.svc.reachableMember(tower.id(t)); return ok })
	tower.svc.retellRoleWithdrawals(context.Background())

	if ended, _ := roleWith(t, attic, store.RoleRequestIn, tower, store.RoleFetcher); ended.State != store.RoleRevoked {
		t.Fatalf("attic after it came back = %+v, want the request withdrawn", ended)
	}
	if pending, err := tower.st.ListRoleWithdrawals(); err != nil || len(pending) != 0 {
		t.Fatalf("withdrawals still to tell = %+v, %v", pending, err)
	}
}

func TestAskingAgainReplacesAWithdrawalTheMemberMissed(t *testing.T) {
	tower := newInstance(t, "tower", strings.Repeat("a1", 32))
	attic := newInstance(t, "attic", strings.Repeat("b2", 32))
	pairThroughRelay(t, tower, attic)
	if err := tower.st.NoteRoleWithdrawal(attic.id(t), store.RoleFetcher); err != nil {
		t.Fatal(err)
	}
	askForRole(t, tower, attic, store.RoleFetcher, "containers")
	tower.svc.retellRoleWithdrawals(context.Background())
	if req, ok := roleWith(t, attic, store.RoleRequestIn, tower, store.RoleFetcher); !ok || req.State != store.RoleAsked {
		t.Fatalf("attic = %+v, want the new request still waiting", req)
	}
}

func TestAnAnswerTheAskingInstanceMissedArrivesWhenItAsksAgain(t *testing.T) {
	tower := newInstance(t, "tower", strings.Repeat("a1", 32))
	attic := newInstance(t, "attic", strings.Repeat("b2", 32))
	pairThroughRelay(t, tower, attic)

	askForRole(t, tower, attic, store.RoleFetcher, "containers")
	req, _ := roleWith(t, attic, store.RoleRequestIn, tower, store.RoleFetcher)
	if _, err := attic.st.DecideRoleRequest(req.ID, store.RoleDecision{State: store.RoleAllowed}); err != nil {
		t.Fatal(err)
	}
	tower.svc.refreshSentRoles(context.Background())
	if mirror, _ := roleWith(t, tower, store.RoleRequestOut, attic, store.RoleFetcher); mirror.State != store.RoleAllowed {
		t.Fatalf("after asking again = %+v, want the allow", mirror)
	}

	if _, err := attic.st.DecideRoleRequest(req.ID, store.RoleDecision{State: store.RoleRevoked}); err != nil {
		t.Fatal(err)
	}
	tower.svc.refreshSentRoles(context.Background())
	tower.svc.refreshSentRoles(context.Background())
	if mirror, _ := roleWith(t, tower, store.RoleRequestOut, attic, store.RoleFetcher); mirror.State != store.RoleRevoked {
		t.Fatalf("after asking again = %+v, want the revoke", mirror)
	}
	if still, _ := roleWith(t, attic, store.RoleRequestIn, tower, store.RoleFetcher); still.State != store.RoleRevoked {
		t.Fatalf("asking again as the instance reopened a revoked request: %+v", still)
	}
}

func TestOnlyTheAskedMemberAnswersARoleRequest(t *testing.T) {
	tower := newInstance(t, "tower", strings.Repeat("a1", 32))
	attic := newInstance(t, "attic", strings.Repeat("b2", 32))
	pairThroughRelay(t, tower, attic)
	const barn = "0123456789abcdef0123456789abcdef"

	askForRole(t, tower, attic, store.RoleFetcher, "containers")
	allowed := map[string]any{"role": store.RoleFetcher, "state": store.RoleAllowed}

	if out := peerCallAs(t, tower, barn, "/api/group/peer/roles/answer", allowed); out["ok"] != false {
		t.Fatalf("another member's unsigned answer = %v, want it refused", out)
	}
	if mirror, _ := roleWith(t, tower, store.RoleRequestOut, attic, store.RoleFetcher); mirror.State != store.RoleAsked {
		t.Fatalf("another member answered a request that was not put to it: %+v", mirror)
	}

	if out := peerCallAs(t, attic, tower.id(t), "/api/group/peer/roles/answer", allowed); out["ok"] != false {
		t.Fatalf("the asking member's own unsigned answer = %v, want it refused", out)
	}
	if req, _ := roleWith(t, attic, store.RoleRequestIn, tower, store.RoleFetcher); req.State != store.RoleAsked {
		t.Fatalf("the asking member allowed its own request: %+v", req)
	}

	if out := peerCallAs(t, attic, barn, "/api/group/peer/roles/withdraw", map[string]any{"role": store.RoleFetcher}); out["ok"] != false {
		t.Fatalf("withdraw = %v", out)
	}
	if _, ok := roleWith(t, attic, store.RoleRequestIn, tower, store.RoleFetcher); !ok {
		t.Fatal("another member withdrew a request that was not its own")
	}
	sent, _ := roleWith(t, tower, store.RoleRequestOut, attic, store.RoleFetcher)
	if _, _, err := tower.svc.decideRole(context.Background(), sent.ID, store.RoleDecision{State: store.RoleAllowed}); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("a person on the asking instance allowing its own request = %v, want sql.ErrNoRows", err)
	}
}

func TestARoleCallIsBoundToTheSenderTheChannelNames(t *testing.T) {
	attic := newInstance(t, "attic", strings.Repeat("b2", 32))
	if code, _ := attic.do(t, http.MethodPost, "/api/group/phrase", nil); code != http.StatusOK {
		t.Fatalf("create phrase: %d", code)
	}
	const tower = "0123456789abcdef0123456789abcdef"
	ask := map[string]any{"role": store.RoleFetcher, "sections": []string{"containers"}, "name": "tower", "instanceId": "ffffffffffffffffffffffffffffffff"}

	for _, sender := range []string{"", attic.id(t), "not an id"} {
		for _, path := range []string{"/api/group/peer/roles/ask", "/api/group/peer/roles/answer", "/api/group/peer/roles/withdraw"} {
			if out := peerCallAs(t, attic, sender, path, ask); out["ok"] != false {
				t.Errorf("%s from sender %q = %v, want it refused", path, sender, out)
			}
		}
	}
	if roleRequestCount(t, attic) != 0 {
		t.Fatal("a call without a sender left a request behind")
	}

	seed := pinnedRoleSeed(t, attic, tower)
	signed := signedRoleAsk(t, attic, seed, tower, peerRoleAsk{Name: "tower", Role: store.RoleFetcher, Sections: []string{"containers"}}, time.Now().UnixMilli())
	if out := peerCallAs(t, attic, tower, "/api/group/peer/roles/ask", signed); out["ok"] != true || out["state"] != store.RoleAsked {
		t.Fatalf("ask = %v", out)
	}
	all, _ := attic.st.ListRoleRequests()
	if len(all) != 1 || all[0].MemberID != tower {
		t.Fatalf("requests = %+v, want one under the sender the channel named", all)
	}

	counter := time.Now().UnixMilli()
	for name, bad := range map[string]peerRoleAsk{
		"an unknown role":    {Role: "admin", Sections: []string{"containers"}},
		"no section":         {Role: store.RoleFetcher, Sections: []string{}},
		"an unknown section": {Role: store.RoleFetcher, Sections: []string{"settings"}},
		"an unknown store":   {Role: store.RoleReceiver, Sections: []string{"containers"}, Store: "tape"},
	} {
		counter++
		if out := peerCallAs(t, attic, tower, "/api/group/peer/roles/ask", signedRoleAsk(t, attic, seed, tower, bad, counter)); out["ok"] != false {
			t.Errorf("an ask with %s = %v, want it refused", name, out)
		}
	}
	if out := peerCallAs(t, attic, tower, "/api/group/peer/roles/answer", map[string]any{"role": store.RoleFetcher, "state": store.RoleAsked}); out["ok"] != false {
		t.Errorf("an answer that is no answer = %v, want it refused", out)
	}
}
