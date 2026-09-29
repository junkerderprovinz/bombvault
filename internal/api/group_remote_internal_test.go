package api

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/relay"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// concreteCall turns a registered pattern such as "POST /api/containers/{name}/backup"
// into a method and a path a real request could carry, filling every {..}
// segment with a placeholder value. Route parameters are never inspected by
// the allowlist match itself, only by the handler behind it, so a fixed
// filler exercises every pattern the same way.
func concreteCall(pattern string) (method, path string) {
	method, path, _ = strings.Cut(pattern, " ")
	var b strings.Builder
	for _, seg := range strings.Split(path, "/") {
		if strings.HasPrefix(seg, "{") && strings.HasSuffix(seg, "}") {
			seg = "x"
		}
		b.WriteByte('/')
		b.WriteString(seg)
	}
	return method, strings.TrimPrefix(b.String(), "/")
}

// Every route remoteViewRoutes lists must be on the allowlist, and the
// allowlist must accept nothing else it did not just register. This walks
// the real table rather than a copy, so a route added there is covered here
// without anyone having to remember to update a parallel list.
func TestRemoteViewAllowlistMatchesItsOwnTable(t *testing.T) {
	in := newInstance(t, "attic", strings.Repeat("b2", 32))
	for _, rt := range remoteViewRoutes() {
		method, path := concreteCall(rt.pattern)
		if !in.h.remoteViewAllowed(method, path) {
			t.Errorf("remoteViewAllowed(%q, %q) = false, want true (registered as %q)", method, path, rt.pattern)
		}
	}
}

// The routes decision 2 says stay local, always: every restore and delete,
// settings and credentials, auth, group secrets and diagnostics. None of
// these may be reachable through remote view regardless of the switch.
func TestRemoteViewAllowlistRefusesDestructiveAndLocalOnlyRoutes(t *testing.T) {
	in := newInstance(t, "attic", strings.Repeat("b2", 32))
	refused := []struct{ method, path string }{
		{http.MethodPost, "/api/containers/mydb/restore"},
		{http.MethodPost, "/api/containers/mydb/restore-files"},
		{http.MethodPost, "/api/containers/mydb/restore-to"},
		{http.MethodPost, "/api/stacks/proj/restore"},
		{http.MethodPost, "/api/vms/win/restore"},
		{http.MethodPost, "/api/zfs/datasets/x/restore"},
		{http.MethodPost, "/api/files/sets/x/restore"},
		{http.MethodPost, "/api/config/restore"},
		{http.MethodPost, "/api/foreign/restore"},
		{http.MethodPost, "/api/containers/mydb/dbdumps/x/import"},
		{http.MethodDelete, "/api/containers/mydb/backups"},
		{http.MethodDelete, "/api/containers/mydb"},
		{http.MethodDelete, "/api/snapshots/containers/x"},
		{http.MethodDelete, "/api/vms/win/backups"},
		{http.MethodDelete, "/api/files/sets/x/backups"},
		{http.MethodDelete, "/api/zfs/datasets/x/backups"},
		{http.MethodDelete, "/api/zfs/datasets/x/safety-snapshots"},
		{http.MethodDelete, "/api/zfs/datasets/x"},
		{http.MethodPost, "/api/prune/containers"},
		{http.MethodPost, "/api/unlock/containers"},
		{http.MethodPost, "/api/containers/mydb/takeover"},
		{http.MethodPost, "/api/vms/win/takeover"},
		{http.MethodGet, "/api/settings"},
		{http.MethodPut, "/api/settings"},
		{http.MethodGet, "/api/settings/export"},
		{http.MethodPost, "/api/settings/import"},
		{http.MethodGet, "/api/cloud"},
		{http.MethodPost, "/api/cloud"},
		{http.MethodGet, "/api/cloud/creds-sets"},
		{http.MethodPost, "/api/rclone"},
		{http.MethodGet, "/api/notify"},
		{http.MethodPost, "/api/notify"},
		{http.MethodPut, "/api/settings/primary-remote/containers"},
		{http.MethodPost, "/api/repos"},
		{http.MethodPost, "/api/offsite/targets"},
		{http.MethodPut, "/api/offsite/targets/x"},
		{http.MethodDelete, "/api/offsite/targets/x"},
		{http.MethodPost, "/api/auth/password"},
		{http.MethodPost, "/api/auth/totp/setup"},
		{http.MethodGet, "/api/auth/passkeys"},
		{http.MethodPost, "/api/widget/token"},
		{http.MethodGet, "/api/group"},
		{http.MethodPost, "/api/group/phrase"},
		{http.MethodPost, "/api/group/join"},
		{http.MethodDelete, "/api/group"},
		{http.MethodPut, "/api/group/relay"},
		{http.MethodGet, "/api/diagnostics"},
		{http.MethodGet, "/api/recovery-kit"},
		{http.MethodGet, "/api/browse"},
		{http.MethodPost, "/api/browse/mkdir"},
		{http.MethodPost, "/api/dashboard-plugin/install"},
	}
	for _, c := range refused {
		if in.h.remoteViewAllowed(c.method, c.path) {
			t.Errorf("remoteViewAllowed(%q, %q) = true, want false", c.method, c.path)
		}
	}
}

// path.Clean guards handleInstanceForward's caller side; the same check
// inside remoteViewAllowed is what actually protects the instance being
// called, since a call can also be sealed and sent without ever going
// through that handler.
func TestRemoteViewAllowlistRejectsPathTraversal(t *testing.T) {
	in := newInstance(t, "attic", strings.Repeat("b2", 32))
	for _, path := range []string{
		"/api/containers/../settings",
		"/api/containers/x/../../settings",
		"/api/containers/%2e%2e/settings",
		"/api/containers//..//settings",
	} {
		if in.h.remoteViewAllowed(http.MethodGet, path) {
			t.Errorf("remoteViewAllowed(GET, %q) = true, want false", path)
		}
	}
}

// With the switch off, servePeer refuses every remote-view route while the
// pairing routes that predate the feature keep working.
func TestRemoteViewSwitchOffRefusesEveryRoute(t *testing.T) {
	in := newInstance(t, "attic", strings.Repeat("b2", 32))
	if _, err := in.st.MutateSettings(func(s *store.Settings) error {
		s.RemoteViewEnabled = false
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	status, body := in.svc.servePeer(context.Background(), relay.ProxyCall{Method: http.MethodGet, Path: "/api/status", Sender: "member-x"})
	if status != http.StatusForbidden {
		t.Fatalf("GET /api/status with remote view off = %d %s, want 403", status, body)
	}
	status, body = in.svc.servePeer(context.Background(), relay.ProxyCall{Method: http.MethodPost, Path: "/api/backup-everything", Sender: "member-x"})
	if status != http.StatusForbidden {
		t.Fatalf("POST /api/backup-everything with remote view off = %d %s, want 403", status, body)
	}
	if status, _ := in.svc.servePeer(context.Background(), relay.ProxyCall{Method: http.MethodGet, Path: "/api/group/peer/status"}); status != http.StatusOK {
		t.Fatalf("the legacy peer status route must still answer with remote view off, got %d", status)
	}
}

// The switch defaults on, matching the owner's decision that this feature is
// opt-out rather than opt-in like Fleet, the receiver and pull.
func TestRemoteViewSwitchDefaultsOn(t *testing.T) {
	in := newInstance(t, "attic", strings.Repeat("b2", 32))
	settings, err := in.st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	if !settings.RemoteViewEnabled {
		t.Fatal("RemoteViewEnabled must default to true on a fresh database")
	}
}

// End to end: a asks b, over the outbound forward route a real browser call
// hits, for an allowlisted read and an allowlisted trigger, and b refuses a
// route that is not on the list. The pair talks through an in-process relay,
// never the project one (see pairThroughRelay).
func TestRemoteViewForwardsAllowedCallsAndRefusesOthers(t *testing.T) {
	a := newInstance(t, "cellar", strings.Repeat("a1", 32))
	b := newInstance(t, "attic", strings.Repeat("b2", 32))
	pairThroughRelay(t, a, b)

	// A read: b's own run history, fetched by a through the forward route.
	if code, out := a.do(t, http.MethodGet, "/api/instances/"+b.id(t)+"/runs", nil); code != http.StatusOK || out["ok"] != true {
		t.Fatalf("GET runs through remote view: %d %v", code, out)
	}

	// A trigger: Backup Everything on b, with nothing enabled so it completes
	// at once, must record its parent run with b naming a as the caller.
	code, out := a.do(t, http.MethodPost, "/api/instances/"+b.id(t)+"/backup-everything", nil)
	if code != http.StatusOK || out["ok"] != true {
		t.Fatalf("POST backup-everything through remote view: %d %v", code, out)
	}
	deadline := time.Now().Add(5 * time.Second)
	var run store.Run
	for time.Now().Before(deadline) {
		runs, lErr := b.st.ListRuns(10)
		if lErr != nil {
			t.Fatal(lErr)
		}
		for _, r := range runs {
			if r.TargetID == store.EverythingTargetID {
				run = r
			}
		}
		if run.ID != "" {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if run.ID == "" {
		t.Fatal("b never recorded a Backup Everything run started by a")
	}
	if run.StartedVia != "remote" || run.StartedViaKey != a.id(t) {
		t.Fatalf("run started via remote view = %+v, want StartedVia=remote and StartedViaKey=%s", run, a.id(t))
	}
	views := b.h.runViews([]store.Run{run})
	if len(views) != 1 || views[0].StartedViaLabel != "cellar" {
		t.Fatalf("runViews did not name the calling member: %+v", views)
	}

	// A route that stays local, always: refused with 403, not proxied at all.
	code, _ = a.do(t, http.MethodPut, "/api/instances/"+b.id(t)+"/settings", map[string]any{})
	if code != http.StatusForbidden {
		t.Fatalf("PUT settings through remote view = %d, want 403", code)
	}

	// With b's switch off, an otherwise allowed call is refused too.
	if _, err := b.st.MutateSettings(func(s *store.Settings) error {
		s.RemoteViewEnabled = false
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	code, out = a.do(t, http.MethodGet, "/api/instances/"+b.id(t)+"/runs", nil)
	if code != http.StatusForbidden || out["ok"] != false {
		t.Fatalf("GET runs with b's remote view off = %d %v, want a 403 ok:false envelope", code, out)
	}
}

// A body over the 1 MB cap is refused before it reaches the outbound call, so
// an oversized request never even asks a member.
func TestRemoteViewForwardCapsTheRequestBody(t *testing.T) {
	a := newInstance(t, "cellar", strings.Repeat("a1", 32))
	b := newInstance(t, "attic", strings.Repeat("b2", 32))
	pairThroughRelay(t, a, b)

	big := strings.Repeat("x", (1<<20)+1)
	code, _ := a.do(t, http.MethodPost, "/api/instances/"+b.id(t)+"/backup-everything", map[string]any{"padding": big})
	if code != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversized remote-view body = %d, want %d", code, http.StatusRequestEntityTooLarge)
	}
}
