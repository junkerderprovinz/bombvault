package store_test

import (
	"database/sql"
	"errors"
	"slices"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

func aRoleAsk(role string, sections ...string) store.RoleRequest {
	return store.RoleRequest{MemberID: "peer-tower", MemberName: "tower", Role: role, Sections: sections, Store: store.RoleStoreRest}
}

func decide(t *testing.T, r *store.Repo, id, state string) store.RoleRequest {
	t.Helper()
	req, err := r.DecideRoleRequest(id, store.RoleDecision{State: state})
	if err != nil {
		t.Fatalf("decide %s: %v", state, err)
	}
	return req
}

func TestAFirstRoleRequestWaitsForAnAnswer(t *testing.T) {
	_, r := zfsStore(t)
	req, err := r.AskRoleRequest(aRoleAsk(store.RoleReceiver, "containers", "flash"), true)
	if err != nil {
		t.Fatal(err)
	}
	if req.ID == "" || req.Direction != store.RoleRequestIn || req.State != store.RoleAsked || req.AskedAt == 0 ||
		req.DecidedAt != 0 || req.DecidedBy != "" {
		t.Fatalf("first request = %+v", req)
	}
	got, ok, err := r.GetRoleRequest(req.ID)
	if err != nil || !ok || !slices.Equal(got.Sections, []string{"containers", "flash"}) || got.Store != store.RoleStoreRest {
		t.Fatalf("stored request = %+v, %v, %v", got, ok, err)
	}
	if _, ok, _ := r.FindRoleRequest(store.RoleRequestIn, "peer-tower", store.RoleFetcher); ok {
		t.Fatal("a Receiver request must not count for the Fetcher role")
	}
}

func TestARoleRequestRecordsWhoDecidedAndWhen(t *testing.T) {
	_, r := zfsStore(t)
	req, _ := r.AskRoleRequest(aRoleAsk(store.RoleFetcher, "containers"), true)
	if _, err := r.DecideRoleRequest(req.ID, store.RoleDecision{State: store.RoleRevoked}); !errors.Is(err, store.ErrRoleRequestMove) {
		t.Fatalf("revoking a waiting request = %v, want ErrRoleRequestMove", err)
	}
	allowed := decide(t, r, req.ID, store.RoleAllowed)
	if allowed.State != store.RoleAllowed || allowed.DecidedAt == 0 || allowed.DecidedBy != store.RoleDecidedHere {
		t.Fatalf("allowed request = %+v", allowed)
	}
	revoked := decide(t, r, req.ID, store.RoleRevoked)
	if revoked.State != store.RoleRevoked {
		t.Fatalf("revoked request = %+v", revoked)
	}
	if _, err := r.DecideRoleRequest(req.ID, store.RoleDecision{State: store.RoleAllowed}); !errors.Is(err, store.ErrRoleRequestMove) {
		t.Fatalf("allowing a revoked request = %v, want ErrRoleRequestMove", err)
	}
	if _, err := r.DecideRoleRequest("0123456789abcdef0123456789abcdef", store.RoleDecision{State: store.RoleAllowed}); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("deciding a missing request = %v, want sql.ErrNoRows", err)
	}
}

func TestGivingARoleRequestTheAnswerItHasChangesNothing(t *testing.T) {
	db, r := zfsStore(t)
	req, _ := r.AskRoleRequest(aRoleAsk(store.RoleReceiver, "containers"), true)
	first := decide(t, r, req.ID, store.RoleAllowed)
	if _, err := db.Exec(`UPDATE role_requests SET decided_at = 1700000000 WHERE id = ?`, req.ID); err != nil {
		t.Fatal(err)
	}
	first.DecidedAt = 1700000000
	again := decide(t, r, req.ID, store.RoleAllowed)
	if again.State != first.State || again.DecidedAt != first.DecidedAt || again.DecidedBy != first.DecidedBy {
		t.Fatalf("a second allow = %+v, want %+v untouched", again, first)
	}
}

func TestAnAllowNamesTheSectionsThePersonWasShown(t *testing.T) {
	_, r := zfsStore(t)
	req, _ := r.AskRoleRequest(aRoleAsk(store.RoleReceiver, "containers"), true)
	if _, err := r.AskRoleRequest(aRoleAsk(store.RoleReceiver, "containers", "vms"), true); err != nil {
		t.Fatal(err)
	}
	_, err := r.DecideRoleRequest(req.ID, store.RoleDecision{State: store.RoleAllowed, Sections: []string{"containers"}})
	if !errors.Is(err, store.ErrRoleRequestChanged) {
		t.Fatalf("allowing a request that grew meanwhile = %v, want ErrRoleRequestChanged", err)
	}
	if _, err := r.DecideRoleRequest(req.ID, store.RoleDecision{State: store.RoleAllowed, Sections: []string{"vms", "containers"}}); err != nil {
		t.Fatalf("allowing what the request asks for: %v", err)
	}
}

func TestAskingAgainKeepsTheAnswerARoleRequestHas(t *testing.T) {
	_, r := zfsStore(t)
	req, _ := r.AskRoleRequest(aRoleAsk(store.RoleReceiver, "containers", "flash"), true)
	decide(t, r, req.ID, store.RoleAllowed)

	wider, err := r.AskRoleRequest(aRoleAsk(store.RoleReceiver, "containers", "flash", "vms"), true)
	if err != nil || wider.ID != req.ID || wider.State != store.RoleAllowed || len(wider.Sections) != 3 {
		t.Fatalf("an allowed request asked again = %+v, %v; want it allowed with the new sections", wider, err)
	}

	decide(t, r, req.ID, store.RoleRevoked)
	if again, _ := r.AskRoleRequest(aRoleAsk(store.RoleReceiver, "containers"), false); again.State != store.RoleRevoked {
		t.Fatalf("a revoked request asked again without a person = %+v, want it revoked", again)
	}
	renewed, _ := r.AskRoleRequest(aRoleAsk(store.RoleReceiver, "containers"), true)
	if renewed.State != store.RoleAsked || renewed.DecidedAt != 0 || renewed.DecidedBy != "" {
		t.Fatalf("a revoked request a person asked anew = %+v, want it waiting", renewed)
	}
}

func TestARefusedRoleRequestOpensOnlyForMoreSections(t *testing.T) {
	_, r := zfsStore(t)
	req, _ := r.AskRoleRequest(aRoleAsk(store.RoleFetcher, "containers", "flash"), true)
	decide(t, r, req.ID, store.RoleRefused)

	if same, _ := r.AskRoleRequest(aRoleAsk(store.RoleFetcher, "containers"), true); same.State != store.RoleRefused {
		t.Fatalf("a refused request asked again for less = %+v, want it refused", same)
	}
	more, _ := r.AskRoleRequest(aRoleAsk(store.RoleFetcher, "containers", "vms"), true)
	if more.State != store.RoleAsked {
		t.Fatalf("a refused request asked again for more = %+v, want it waiting", more)
	}
	decide(t, r, req.ID, store.RoleRefused)
	if allowed := decide(t, r, req.ID, store.RoleAllowed); allowed.State != store.RoleAllowed {
		t.Fatalf("a refusal turned into an allow = %+v", allowed)
	}
}

func TestWithdrawingEndsARoleRequestAccordingToItsState(t *testing.T) {
	_, r := zfsStore(t)

	waiting, _ := r.AskRoleRequest(aRoleAsk(store.RoleReceiver, "containers"), true)
	if prev, found, err := r.WithdrawRoleRequest("peer-tower", store.RoleReceiver); err != nil || !found || prev.State != store.RoleAsked {
		t.Fatalf("withdraw of a waiting request = %+v, %v, %v", prev, found, err)
	}
	if _, ok, _ := r.GetRoleRequest(waiting.ID); ok {
		t.Fatal("a withdrawn request nobody answered must be forgotten")
	}
	if _, found, err := r.WithdrawRoleRequest("peer-tower", store.RoleReceiver); err != nil || found {
		t.Fatalf("a second withdraw = %v, %v; want nothing found and no error", found, err)
	}

	allowed, _ := r.AskRoleRequest(aRoleAsk(store.RoleReceiver, "containers"), true)
	decide(t, r, allowed.ID, store.RoleAllowed)
	if prev, _, _ := r.WithdrawRoleRequest("peer-tower", store.RoleReceiver); prev.State != store.RoleAllowed {
		t.Fatalf("withdraw reported %+v, want the request as it stood", prev)
	}
	ended, _, _ := r.GetRoleRequest(allowed.ID)
	if ended.State != store.RoleRevoked || ended.DecidedBy != store.RoleDecidedMember {
		t.Fatalf("a withdrawn allowed request = %+v, want it revoked by the member", ended)
	}

	refused, _ := r.AskRoleRequest(aRoleAsk(store.RoleFetcher, "containers"), true)
	decide(t, r, refused.ID, store.RoleRefused)
	if _, _, err := r.WithdrawRoleRequest("peer-tower", store.RoleFetcher); err != nil {
		t.Fatal(err)
	}
	if again, _ := r.AskRoleRequest(aRoleAsk(store.RoleFetcher, "containers"), true); again.State != store.RoleRefused {
		t.Fatalf("withdrawing and asking again undid a refusal: %+v", again)
	}
}

func TestASentRoleRequestMirrorsTheMembersAnswer(t *testing.T) {
	_, r := zfsStore(t)
	sent := aRoleAsk(store.RoleFetcher, "containers")
	sent.State = store.RoleAsked
	first, err := r.SaveSentRoleRequest(sent)
	if err != nil || first.Direction != store.RoleRequestOut || first.State != store.RoleAsked || first.DecidedAt != 0 {
		t.Fatalf("sent request = %+v, %v", first, err)
	}
	if _, ok, _ := r.FindRoleRequest(store.RoleRequestIn, "peer-tower", store.RoleFetcher); ok {
		t.Fatal("a request this instance sent must not read as one it received")
	}

	found, err := r.SetSentRoleRequestState("peer-tower", store.RoleFetcher, store.RoleAllowed)
	if err != nil || !found {
		t.Fatalf("recording the answer = %v, %v", found, err)
	}
	got, _, _ := r.FindRoleRequest(store.RoleRequestOut, "peer-tower", store.RoleFetcher)
	if got.ID != first.ID || got.State != store.RoleAllowed || got.DecidedAt == 0 || got.DecidedBy != store.RoleDecidedMember {
		t.Fatalf("answered request = %+v", got)
	}
	if found, _ := r.SetSentRoleRequestState("peer-barn", store.RoleFetcher, store.RoleAllowed); found {
		t.Fatal("an answer to a request this instance never sent was recorded")
	}

	sent.Sections, sent.State = []string{"containers", "vms"}, store.RoleAllowed
	again, _ := r.SaveSentRoleRequest(sent)
	if again.ID != first.ID || again.DecidedAt != got.DecidedAt || len(again.Sections) != 2 {
		t.Fatalf("a request sent again = %+v, want the same row with its answer kept", again)
	}

	if err := r.DeleteRoleRequest(store.RoleRequestOut, "peer-tower", store.RoleFetcher); err != nil {
		t.Fatal(err)
	}
	if _, ok, _ := r.FindRoleRequest(store.RoleRequestOut, "peer-tower", store.RoleFetcher); ok {
		t.Fatal("a withdrawn request is still stored")
	}
}

func TestAGrantWithoutAnAnswerNeverOverridesAPerson(t *testing.T) {
	_, r := zfsStore(t)
	grant := aRoleAsk(store.RoleReceiver, "containers")
	grant.Direction = store.RoleRequestIn
	if err := r.GrantRoleRequest(grant); err != nil {
		t.Fatal(err)
	}
	got, ok, _ := r.FindRoleRequest(store.RoleRequestIn, "peer-tower", store.RoleReceiver)
	if !ok || got.State != store.RoleAllowed || got.DecidedBy != store.RoleDecidedAutomatic {
		t.Fatalf("granted request = %+v, %v", got, ok)
	}
	decide(t, r, got.ID, store.RoleRevoked)
	if err := r.GrantRoleRequest(grant); err != nil {
		t.Fatal(err)
	}
	if still, _, _ := r.GetRoleRequest(got.ID); still.State != store.RoleRevoked || still.DecidedBy != store.RoleDecidedHere {
		t.Fatalf("a grant reopened what a person revoked: %+v", still)
	}
}

func TestRoleRequestsListInBothDirections(t *testing.T) {
	_, r := zfsStore(t)
	if _, err := r.AskRoleRequest(aRoleAsk(store.RoleReceiver, "containers"), true); err != nil {
		t.Fatal(err)
	}
	sent := aRoleAsk(store.RoleReceiver, "flash")
	sent.State = store.RoleAsked
	if _, err := r.SaveSentRoleRequest(sent); err != nil {
		t.Fatal(err)
	}
	all, err := r.ListRoleRequests()
	if err != nil || len(all) != 2 {
		t.Fatalf("requests = %+v, %v; want one in each direction for the same member and role", all, err)
	}
}
