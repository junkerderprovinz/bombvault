package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/relay"
	"github.com/junkerderprovinz/bombvault/internal/restickey"
	"github.com/junkerderprovinz/bombvault/internal/secret"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// announceAs makes in tell the group it runs the given version and waits
// until the listed instances have heard and in sees them again.
func announceAs(t *testing.T, in *instance, version string, heardBy ...*instance) {
	t.Helper()
	running := Version
	Version = version
	in.svc.applyGroup()
	Version = running
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		heard := len(in.svc.pairing().Members()) >= len(heardBy)
		for _, other := range heardBy {
			m, ok := other.svc.reachableMember(in.id(t))
			heard = heard && ok && m.Version == version
		}
		if heard {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("the group never heard that %s runs %s", in.id(t), version)
}

// joinThroughRelay brings one more instance into the group host started,
// over the relay host uses, and waits until every listed member sees it.
func joinThroughRelay(t *testing.T, in, host *instance, members ...*instance) {
	t.Helper()
	g, err := host.st.GetGroupState()
	if err != nil {
		t.Fatal(err)
	}
	if code, out := in.do(t, http.MethodPut, "/api/group/relay", map[string]any{"mode": "own", "url": g.RelayURL}); code != http.StatusOK || out["ok"] != true {
		t.Fatalf("set relay: %d %v", code, out)
	}
	_, shown := host.do(t, http.MethodPost, "/api/group/phrase/show", map[string]any{"password": testPassword})
	if code, out := in.do(t, http.MethodPost, "/api/group/join", map[string]any{"phrase": shown["phrase"]}); code != http.StatusOK || out["active"] != true {
		t.Fatalf("join: %d %v", code, out)
	}
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		seen := len(in.svc.pairing().Members()) == len(members)
		for _, m := range members {
			_, ok := m.svc.reachableMember(in.id(t))
			seen = seen && ok
		}
		if seen {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("the new instance never met the group")
}

// oldWayLogin asks in for a receiver login the way an instance from before
// role requests does. sender is who the group channel names, empty for a
// call such an instance makes through a relay.
func oldWayLogin(t *testing.T, in *instance, sender, claimedID, name string) map[string]any {
	t.Helper()
	return peerCallAs(t, in, sender, "/api/group/peer/receiver", peerReceiverRequest{InstanceID: claimedID, Name: name, Login: true})
}

func oldWayPairing(t *testing.T, in *instance, sender string) peerPairing {
	t.Helper()
	_, raw := in.svc.servePeer(context.Background(), relay.ProxyCall{Method: http.MethodGet, Path: "/api/group/peer/pairing", Sender: sender})
	var p peerPairing
	if err := json.Unmarshal(raw, &p); err != nil {
		t.Fatalf("pairing answered unreadably: %s", raw)
	}
	return p
}

func receiverLoginOf(t *testing.T, in, member *instance) (store.ReceiverLogin, bool) {
	t.Helper()
	l, ok, err := in.st.GetReceiverLogin(member.id(t))
	if err != nil {
		t.Fatal(err)
	}
	return l, ok
}

func wizardLogin(t *testing.T, sender, receiver *instance) map[string]any {
	t.Helper()
	_, out := sender.do(t, http.MethodPost, "/api/offsite/group-receivers/"+receiver.id(t)+"/login", nil)
	return out
}

func TestAReceiverLoginLastsOnlyWhileTheRoleIsAllowed(t *testing.T) {
	tower := newInstance(t, "tower", strings.Repeat("a1", 32))
	attic := newInstance(t, "attic", strings.Repeat("b2", 32))
	pairThroughRelay(t, tower, attic)
	runReceiver(t, attic)

	askForRole(t, tower, attic, store.RoleReceiver, "containers")
	if out := wizardLogin(t, tower, attic); out["ok"] != false {
		t.Fatalf("a login while the request waits: %v", out)
	}
	req, _ := roleWith(t, attic, store.RoleRequestIn, tower, store.RoleReceiver)
	answerRole(t, attic, req.ID, store.RoleAllowed)
	out := wizardLogin(t, tower, attic)
	login, _ := out["login"].(map[string]any)
	if out["ok"] != true || login["user"] != "tower" || login["password"] == "" {
		t.Fatalf("a login after the allow = %v", out)
	}
	if _, ok := receiverLoginOf(t, attic, tower); !ok {
		t.Fatal("the receiver keeps no login for the allowed member")
	}

	answerRole(t, attic, req.ID, store.RoleRevoked)
	if _, ok := receiverLoginOf(t, attic, tower); ok {
		t.Fatal("a revoke left the member's login on the receiver")
	}
	if out := wizardLogin(t, tower, attic); out["ok"] != false {
		t.Fatalf("a login after the revoke: %v", out)
	}
	if _, ok := receiverLoginOf(t, attic, tower); ok {
		t.Fatal("asking again after a revoke brought the login back")
	}
	if again, _ := roleWith(t, attic, store.RoleRequestIn, tower, store.RoleReceiver); again.State != store.RoleAsked {
		t.Fatalf("the request after the member asked anew = %+v, want it waiting for a person", again)
	}
}

func TestRevokingALoginOnTheReceiverRevokesTheRole(t *testing.T) {
	tower := newInstance(t, "tower", strings.Repeat("a1", 32))
	attic := newInstance(t, "attic", strings.Repeat("b2", 32))
	pairThroughRelay(t, tower, attic)
	runReceiver(t, attic)
	allowRole(t, tower, attic, store.RoleReceiver, "containers")
	if out := wizardLogin(t, tower, attic); out["ok"] != true {
		t.Fatalf("login: %v", out)
	}

	if code, out := attic.do(t, http.MethodDelete, "/api/receiver/server/logins/"+tower.id(t), nil); code != http.StatusOK || out["ok"] != true {
		t.Fatalf("revoke login: %d %v", code, out)
	}
	if req, _ := roleWith(t, attic, store.RoleRequestIn, tower, store.RoleReceiver); req.State != store.RoleRevoked {
		t.Fatalf("the role after its login was revoked = %+v", req)
	}
	if mirror, _ := roleWith(t, tower, store.RoleRequestOut, attic, store.RoleReceiver); mirror.State != store.RoleRevoked {
		t.Fatalf("the sender after its login was revoked = %+v", mirror)
	}
	tower.svc.refreshSentRoles(context.Background())
	if _, ok := receiverLoginOf(t, attic, tower); ok {
		t.Fatal("the instance asking again by itself brought the login back")
	}
}

func TestWithdrawingTheReceiverRoleGivesUpTheLogin(t *testing.T) {
	tower := newInstance(t, "tower", strings.Repeat("a1", 32))
	attic := newInstance(t, "attic", strings.Repeat("b2", 32))
	pairThroughRelay(t, tower, attic)
	runReceiver(t, attic)
	allowRole(t, tower, attic, store.RoleReceiver, "containers")
	if out := wizardLogin(t, tower, attic); out["ok"] != true {
		t.Fatalf("login: %v", out)
	}
	if told, err := tower.svc.withdrawRole(context.Background(), attic.id(t), store.RoleReceiver); err != nil || !told {
		t.Fatalf("withdraw: told %v, %v", told, err)
	}
	if _, ok := receiverLoginOf(t, attic, tower); ok {
		t.Fatal("the receiver kept the login of a member that withdrew")
	}
}

func TestTheResticPasswordIsReleasedOnlyWhileTheFetcherRoleIsAllowed(t *testing.T) {
	source := newInstance(t, "tower", strings.Repeat("a1", 32))
	fetcher := newInstance(t, "attic", strings.Repeat("b2", 32))
	pairThroughRelay(t, source, fetcher)
	fetcher.engine.want = restickey.Derive(source.appKey)
	for _, target := range []store.OffsiteTarget{
		{Domain: "containers", Name: "box", Repo: "rest:https://box.example:8000/containers", Enabled: true},
		{Domain: "vms", Name: "box", Repo: "rest:https://box.example:8000/vms", Enabled: true},
	} {
		if _, err := source.st.UpsertOffsiteTarget(target); err != nil {
			t.Fatal(err)
		}
	}
	pull := map[string]any{"name": "tower", "repo": "rest:https://box.example:8000/containers", "domain": "containers", "memberId": source.id(t)}
	repos := "/api/group/members/" + source.id(t) + "/repos"

	if _, out := fetcher.do(t, http.MethodGet, repos, nil); out["ok"] != false {
		t.Fatalf("repositories of an instance that asked for nothing: %v", out)
	}
	if _, out := fetcher.do(t, http.MethodPost, "/api/pull/sources", pull); out["ok"] != false {
		t.Fatalf("a pull source of an instance that asked for nothing: %v", out)
	}

	askForRole(t, source, fetcher, store.RoleFetcher, "containers")
	_, out := fetcher.do(t, http.MethodGet, repos, nil)
	listed, _ := out["repos"].([]any)
	if out["ok"] != true || len(listed) != 1 || listed[0].(map[string]any)["domain"] != "containers" {
		t.Fatalf("repositories while the request waits = %v, want the one section asked for", out)
	}
	if _, out := fetcher.do(t, http.MethodPost, "/api/pull/sources", pull); out["ok"] != false || out["error"] != errPasswordHeld.Error() {
		t.Fatalf("a pull source while the request waits = %v, want the password held back", out)
	}

	req, _ := roleWith(t, fetcher, store.RoleRequestIn, source, store.RoleFetcher)
	answerRole(t, fetcher, req.ID, store.RoleAllowed)
	_, out = fetcher.do(t, http.MethodPost, "/api/pull/sources", pull)
	if out["ok"] != true {
		t.Fatalf("a pull source after the allow: %v", out)
	}
	sources, _ := fetcher.st.ListPullSources()
	if len(sources) != 1 {
		t.Fatalf("pull sources = %+v", sources)
	}
	if err := fetcher.svc.fetcherRoleOpen(source.id(t), "containers"); err != nil {
		t.Fatalf("a pull of the allowed section: %v", err)
	}
	if err := fetcher.svc.fetcherRoleOpen(source.id(t), "vms"); err == nil {
		t.Fatal("a pull of a section the source did not offer must not run")
	}

	if _, err := source.svc.withdrawRole(context.Background(), fetcher.id(t), store.RoleFetcher); err != nil {
		t.Fatal(err)
	}
	edit := "/api/pull/sources/" + sources[0].ID
	if _, out := fetcher.do(t, http.MethodPut, edit, pull); out["ok"] != false {
		t.Fatalf("the password after the source withdrew: %v", out)
	}
	if _, err := fetcher.svc.PullFromSource(context.Background(), sources[0]); err == nil || !strings.Contains(err.Error(), store.RoleRevoked) {
		t.Fatalf("a pull after the source withdrew = %v, want it refused as revoked", err)
	}
}

func TestAFetcherThatRevokesNoLongerGetsThePassword(t *testing.T) {
	source := newInstance(t, "tower", strings.Repeat("a1", 32))
	fetcher := newInstance(t, "attic", strings.Repeat("b2", 32))
	pairThroughRelay(t, source, fetcher)
	granted := allowRole(t, source, fetcher, store.RoleFetcher, "containers")
	if p := oldWayPairing(t, source, fetcher.id(t)); !p.OK || p.ResticPassword != restickey.Derive(source.appKey) {
		t.Fatalf("pairing while allowed = %+v", p)
	}
	answerRole(t, fetcher, granted.ID, store.RoleRevoked)
	if p := oldWayPairing(t, source, fetcher.id(t)); p.OK || p.ResticPassword != "" {
		t.Fatalf("pairing after the fetcher revoked = %+v, want nothing", p)
	}
}

func TestTheReceiverRoleReleasesThePasswordToTheReceiver(t *testing.T) {
	sender := newInstance(t, "tower", strings.Repeat("a1", 32))
	receiver := newInstance(t, "attic", strings.Repeat("b2", 32))
	pairThroughRelay(t, sender, receiver)
	runReceiver(t, receiver)
	receiver.engine.want = restickey.Derive(sender.appKey)
	watch := map[string]any{"name": "from tower", "repo": "rest:http://192.168.1.9:8000/tower", "memberId": sender.id(t)}

	askForRole(t, sender, receiver, store.RoleReceiver, "containers")
	if _, out := receiver.do(t, http.MethodPost, "/api/receiver/repos", watch); out["ok"] != false {
		t.Fatalf("a watched repository while the request waits: %v", out)
	}
	req, _ := roleWith(t, receiver, store.RoleRequestIn, sender, store.RoleReceiver)
	answerRole(t, receiver, req.ID, store.RoleAllowed)
	if _, out := receiver.do(t, http.MethodPost, "/api/receiver/repos", watch); out["ok"] != true {
		t.Fatalf("a watched repository after the allow: %v", out)
	}
}

// An answer that did not arrive must not keep the allowed side from pairing:
// it tells the asking instance again before it asks for the password.
func TestAnAllowTheSourceMissedIsToldAgainBeforeThePasswordIsAsked(t *testing.T) {
	source := newInstance(t, "tower", strings.Repeat("a1", 32))
	fetcher := newInstance(t, "attic", strings.Repeat("b2", 32))
	pairThroughRelay(t, source, fetcher)
	fetcher.engine.want = restickey.Derive(source.appKey)

	askForRole(t, source, fetcher, store.RoleFetcher, "containers")
	req, _ := roleWith(t, fetcher, store.RoleRequestIn, source, store.RoleFetcher)
	if _, err := fetcher.st.DecideRoleRequest(req.ID, store.RoleDecision{State: store.RoleAllowed}); err != nil {
		t.Fatal(err)
	}
	_, out := fetcher.do(t, http.MethodPost, "/api/pull/sources", map[string]any{
		"name": "tower", "repo": "rest:https://box.example:8000/containers", "domain": "containers", "memberId": source.id(t),
	})
	if out["ok"] != true {
		t.Fatalf("a pull source after an allow the source had not heard of: %v", out)
	}
}

func TestLoginsAndPairingsFromBeforeTheUpgradeGoOnWithoutAnyoneAnswering(t *testing.T) {
	tower := newInstance(t, "tower", strings.Repeat("a1", 32))
	attic := newInstance(t, "attic", strings.Repeat("b2", 32))
	pairThroughRelay(t, tower, attic)
	runReceiver(t, attic)
	attic.engine.want = restickey.Derive(tower.appKey)

	sealed, err := secret.Encrypt(attic.appKey, []byte("kept across the upgrade"))
	if err != nil {
		t.Fatal(err)
	}
	if err := attic.st.CreateReceiverLogin(store.ReceiverLogin{MemberID: tower.id(t), MemberName: "tower", User: "tower", PasswordEnc: sealed}); err != nil {
		t.Fatal(err)
	}
	pulled, err := attic.st.CreatePullSource(store.PullSource{
		MemberID: tower.id(t), Name: "tower", Repo: "rest:https://box.example:8000/containers", Domain: "containers", Enabled: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := attic.db.Exec(`DELETE FROM schema_migrations WHERE name = 'role_requests_from_pairings'`); err != nil {
		t.Fatal(err)
	}
	if err := store.Migrate(attic.db); err != nil {
		t.Fatal(err)
	}

	for _, role := range []string{store.RoleReceiver, store.RoleFetcher} {
		req, ok := roleWith(t, attic, store.RoleRequestIn, tower, role)
		if !ok || req.State != store.RoleAllowed || req.DecidedBy != store.RoleDecidedUpgrade {
			t.Fatalf("the %s role after the upgrade = %+v, %v", role, req, ok)
		}
	}
	if err := attic.svc.fetcherRoleOpen(pulled.MemberID, pulled.Domain); err != nil {
		t.Fatalf("the pull that ran before the upgrade: %v", err)
	}

	// The sending side kept no record of its login, so it asks; the receiver
	// already allows it and nobody has to answer.
	out := oldWayLogin(t, attic, tower.id(t), tower.id(t), "tower")
	if held, _ := out["receiver"].(map[string]any); out["ok"] != true || held["password"] != "kept across the upgrade" {
		t.Fatalf("the login the member held before the upgrade: %v", out)
	}
	if mirror := askForRole(t, tower, attic, store.RoleReceiver, "containers"); mirror.State != store.RoleAllowed {
		t.Fatalf("the sender switching the role on after the upgrade = %+v, want it allowed at once", mirror)
	}
	if mirror := askForRole(t, tower, attic, store.RoleFetcher, "containers"); mirror.State != store.RoleAllowed {
		t.Fatalf("the source switching the role on after the upgrade = %+v, want it allowed at once", mirror)
	}
	if _, out := attic.do(t, http.MethodPut, "/api/pull/sources/"+pulled.ID, map[string]any{
		"name": "tower", "repo": pulled.Repo, "domain": "containers", "memberId": tower.id(t),
	}); out["ok"] != true {
		t.Fatalf("pairing the pull source again after the upgrade: %v", out)
	}
}

func TestAskingAMemberFromBeforeRoleRequestsNeedsNoAnswer(t *testing.T) {
	tower := newInstance(t, "tower", strings.Repeat("a1", 32))
	old := newInstance(t, "attic", strings.Repeat("b2", 32))
	pairThroughRelay(t, tower, old)
	announceAs(t, old, "v9.9.1", tower)
	m, _ := tower.svc.reachableMember(old.id(t))

	if _, err := tower.svc.askRole(context.Background(), m, store.RoleReceiver, []string{"containers"}, store.RoleStoreRest); !errors.Is(err, errNoMemberReceiver) {
		t.Fatalf("asking an old member without a receiver = %v, want %v", err, errNoMemberReceiver)
	}
	runReceiver(t, old)
	for _, role := range []string{store.RoleReceiver, store.RoleFetcher} {
		req, err := tower.svc.askRole(context.Background(), m, role, []string{"containers"}, "")
		if err != nil || req.State != store.RoleAllowed || req.DecidedBy != store.RoleDecidedAutomatic {
			t.Fatalf("asking an old member to be %s = %+v, %v; want it allowed without an answer", role, req, err)
		}
	}
	if roleRequestCount(t, old) != 0 {
		t.Fatal("an old member was sent a request it has no route for")
	}

	// The old member fetches the password the way it always did, and gets it
	// because a person here switched the role on.
	if p := oldWayPairing(t, tower, ""); !p.OK || p.ResticPassword != restickey.Derive(tower.appKey) {
		t.Fatalf("the old member's call through the relay = %+v", p)
	}
	if told, err := tower.svc.withdrawRole(context.Background(), old.id(t), store.RoleFetcher); err != nil || told {
		t.Fatalf("withdraw from an old member: told %v, %v; want nothing sent", told, err)
	}
	if _, err := tower.svc.withdrawRole(context.Background(), old.id(t), store.RoleReceiver); err != nil {
		t.Fatal(err)
	}
	if p := oldWayPairing(t, tower, ""); p.OK || p.ResticPassword != "" {
		t.Fatalf("the old member's call after the roles were switched off = %+v", p)
	}
}

func TestAMemberFromBeforeRoleRequestsWaitsForAPersonLikeAnyOther(t *testing.T) {
	attic := newInstance(t, "attic", strings.Repeat("b2", 32))
	old := newInstance(t, "tower", strings.Repeat("a1", 32))
	pairThroughRelay(t, attic, old)
	announceAs(t, old, "v9.9.1", attic)
	runReceiver(t, attic)

	if out := oldWayLogin(t, attic, old.id(t), old.id(t), "tower"); out["ok"] != false {
		t.Fatalf("an old member's first login request: %v", out)
	}
	req, ok := roleWith(t, attic, store.RoleRequestIn, old, store.RoleReceiver)
	if !ok || req.State != store.RoleAsked || req.MemberName != "tower" || len(req.Sections) != len(allRoleSections) {
		t.Fatalf("the request recorded for the old member = %+v, %v", req, ok)
	}
	if _, has := receiverLoginOf(t, attic, old); has {
		t.Fatal("an old member got a login before a person allowed it")
	}

	if _, _, err := attic.svc.decideRole(context.Background(), req.ID, store.RoleDecision{State: store.RoleAllowed}); err != nil {
		t.Fatal(err)
	}
	direct := oldWayLogin(t, attic, old.id(t), old.id(t), "tower")
	relayed := oldWayLogin(t, attic, "", old.id(t), "tower")
	for name, out := range map[string]map[string]any{"direct": direct, "relayed": relayed} {
		receiver, _ := out["receiver"].(map[string]any)
		if out["ok"] != true || receiver["user"] != "tower" || receiver["password"] == "" {
			t.Fatalf("the old member's %s login request after the allow = %v", name, out)
		}
	}

	if _, _, err := attic.svc.decideRole(context.Background(), req.ID, store.RoleDecision{State: store.RoleRevoked}); err != nil {
		t.Fatal(err)
	}
	if out := oldWayLogin(t, attic, "", old.id(t), "tower"); out["ok"] != false {
		t.Fatalf("an old member's login request after the revoke: %v", out)
	}
	if _, has := receiverLoginOf(t, attic, old); has {
		t.Fatal("an old member got its revoked login back by asking")
	}
}

func TestAnOldWayCallIsBoundToTheMemberTheGroupNames(t *testing.T) {
	attic := newInstance(t, "attic", strings.Repeat("b2", 32))
	tower := newInstance(t, "tower", strings.Repeat("a1", 32))
	barn := newInstance(t, "barn", strings.Repeat("c3", 32))
	pairThroughRelay(t, attic, tower)
	runReceiver(t, attic)
	allowRole(t, tower, attic, store.RoleReceiver, "containers")
	joinThroughRelay(t, barn, attic, attic, tower)

	if out := oldWayLogin(t, attic, barn.id(t), tower.id(t), "tower"); out["ok"] != false {
		t.Fatalf("a member asking for another member's login: %v", out)
	}
	if out := oldWayLogin(t, attic, barn.id(t), barn.id(t), "barn"); out["ok"] != false {
		t.Fatalf("a current member asking the old way without a request: %v", out)
	}
	if _, ok := roleWith(t, attic, store.RoleRequestIn, barn, store.RoleReceiver); ok {
		t.Fatal("a current member's old-way call was recorded as a request")
	}
	if out := oldWayLogin(t, attic, "", tower.id(t), "tower"); out["ok"] != false {
		t.Fatalf("a call that names no sender in a group without old members: %v", out)
	}

	if _, err := attic.st.SaveSentRoleRequest(store.RoleRequest{MemberID: tower.id(t), Role: store.RoleFetcher, Sections: []string{"containers"}, State: store.RoleAllowed}); err != nil {
		t.Fatal(err)
	}
	announceAs(t, tower, "v9.9.1", attic)
	if p := oldWayPairing(t, attic, ""); !p.OK || p.ResticPassword == "" {
		t.Fatalf("the one old member's call through the relay = %+v", p)
	}
	announceAs(t, barn, "v9.8.0", attic)
	if p := oldWayPairing(t, attic, ""); p.OK || p.ResticPassword != "" {
		t.Fatalf("a call through the relay that either of two old members could have sent = %+v, want it refused", p)
	}
	if out := oldWayLogin(t, attic, "", tower.id(t), "tower"); out["ok"] != false {
		t.Fatalf("a login request either of two old members could have sent: %v", out)
	}
	if p := oldWayPairing(t, attic, barn.id(t)); p.OK {
		t.Fatalf("an old member this instance asked for nothing = %+v", p)
	}
	if p := oldWayPairing(t, attic, tower.id(t)); !p.OK || p.ResticPassword == "" {
		t.Fatalf("the old member this instance asked, calling directly = %+v", p)
	}
}

func TestAVersionCountsAsOldOnlyWhenItSaysSo(t *testing.T) {
	for version, old := range map[string]bool{
		"v9.9.1": true, "v9.10.0": true, "v1.2.3": true,
		"v10.0.0": false, "v10.0.0-rc.1": false, "v11.4.0": false, "dev": false, "": false, "9.9.1": false, "v9.9.1+main.4c0c987": false,
	} {
		if got := predatesRoleRequests(version); got != old {
			t.Errorf("predatesRoleRequests(%q) = %v, want %v", version, got, old)
		}
	}
}
