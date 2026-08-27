package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// probeStubEngine implements only RepoOpensErr: a repo listed in opens probes
// clean, anything else fails. Every probed repo is recorded so a test can pin
// WHICH destination was actually contacted. All other ResticEngine methods come
// from the embedded nil interface and must not be called on these paths.
type probeStubEngine struct {
	ResticEngine
	opens map[string]bool

	mu     sync.Mutex
	probed []string
}

func (e *probeStubEngine) RepoOpensErr(_ context.Context, repo string, _ restic.Mode) error {
	e.mu.Lock()
	e.probed = append(e.probed, repo)
	e.mu.Unlock()
	if e.opens[repo] {
		return nil
	}
	return &probeFailure{repo: repo}
}

func (e *probeStubEngine) probeCount() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return len(e.probed)
}

type probeFailure struct{ repo string }

func (e *probeFailure) Error() string {
	return "Fatal: unable to open repository at " + e.repo + ": server response unexpected: 401 Unauthorized"
}

// newProbeSvc builds a Service over an in-memory store with a stub engine.
func newProbeSvc(t *testing.T, opens map[string]bool) (*Service, *store.Repo, *probeStubEngine) {
	t.Helper()
	s, st := newSyncTestService(t)
	eng := &probeStubEngine{opens: opens}
	s.cfg = config.Config{AppKey: strings.Repeat("a", 64), HostMountRoot: t.TempDir(), HostSourceRoot: "/mnt"}
	s.engine = eng
	return s, st, eng
}

// TestTestOffsiteTargetProbesThatTarget is the issue-#138 regression: a domain's
// SECOND off-site destination could be broken while "Test connection" (which
// only ever probes the PRIMARY) stayed green. Each target must now be probed on
// its own, at its own repo.
func TestTestOffsiteTargetProbesThatTarget(t *testing.T) {
	const primaryRepo = "rest:http://good:8000/containers"
	const secondRepo = "rest:http://broken:8000/containers"

	svc, st, eng := newProbeSvc(t, map[string]bool{primaryRepo: true})
	primary, err := st.UpsertOffsiteTarget(store.OffsiteTarget{
		Domain: "containers", Name: "Primary", Repo: primaryRepo, Enabled: true, SortOrder: 0,
	})
	if err != nil {
		t.Fatal(err)
	}
	second, err := st.UpsertOffsiteTarget(store.OffsiteTarget{
		Domain: "containers", Name: "NAS", Repo: secondRepo, Enabled: true, SortOrder: 1,
	})
	if err != nil {
		t.Fatal(err)
	}

	// The domain-level probe still reports the PRIMARY as healthy — that is the
	// misleading green the issue described, and it stays correct for what it says.
	reachable, initialized, err := svc.TestOffsite(context.Background(), "containers")
	if err != nil || !reachable || !initialized {
		t.Fatalf("primary probe = (%v, %v, %v), want reachable+initialized", reachable, initialized, err)
	}

	// Same target by id → same verdict.
	reachable, initialized, err = svc.TestOffsiteTarget(context.Background(), primary.ID)
	if err != nil || !reachable || !initialized {
		t.Fatalf("per-target probe of the primary = (%v, %v, %v), want reachable+initialized", reachable, initialized, err)
	}

	// The BROKEN second destination must now fail on its own, with the reason.
	reachable, initialized, err = svc.TestOffsiteTarget(context.Background(), second.ID)
	if reachable || initialized {
		t.Fatalf("a broken second destination must not report reachable/initialized, got (%v, %v)", reachable, initialized)
	}
	if err == nil {
		t.Fatal("a broken second destination must surface the probe failure")
	}
	if !strings.Contains(err.Error(), "broken") {
		t.Fatalf("the failure must name the destination that failed, got: %v", err)
	}

	// Every probe went to a repo that actually belongs to a target.
	for _, repo := range eng.probed {
		if repo != primaryRepo && repo != secondRepo {
			t.Fatalf("probed an unexpected repo %q", repo)
		}
	}
}

// TestTestOffsiteTargetUnknownID: an unknown id is an error, never a silent
// fallback to the primary — a "test" that probed something else is exactly the
// bug being fixed. Nothing is probed at all.
func TestTestOffsiteTargetUnknownID(t *testing.T) {
	svc, st, eng := newProbeSvc(t, map[string]bool{"rest:http://good:8000/containers": true})
	if _, err := st.UpsertOffsiteTarget(store.OffsiteTarget{
		Domain: "containers", Name: "Primary", Repo: "rest:http://good:8000/containers", Enabled: true,
	}); err != nil {
		t.Fatal(err)
	}

	reachable, initialized, err := svc.TestOffsiteTarget(context.Background(), "ffffffffffffffffffffffffffffffff")
	if err == nil || !strings.Contains(err.Error(), "no such off-site target") {
		t.Fatalf("expected a 'no such off-site target' error, got %v", err)
	}
	if reachable || initialized {
		t.Fatalf("an unknown target must report false/false, got (%v, %v)", reachable, initialized)
	}
	if n := eng.probeCount(); n != 0 {
		t.Fatalf("an unknown target must not probe anything, got %d probe(s)", n)
	}
}

// TestTestOffsiteTargetLocalPathGuidance: a target pointed at an ABSOLUTE host
// path answers with the relative-path guidance (issue #138's original symptom)
// instead of the raw paths sentinel, and never reaches the engine.
func TestTestOffsiteTargetLocalPathGuidance(t *testing.T) {
	svc, st, eng := newProbeSvc(t, nil)
	tgt, err := st.UpsertOffsiteTarget(store.OffsiteTarget{
		Domain: "containers", Name: "NAS", Repo: "/mnt/remotes/192.168.2.53_backup/bombvault", Enabled: true,
	})
	if err != nil {
		t.Fatal(err)
	}

	if _, _, err = svc.TestOffsiteTarget(context.Background(), tgt.ID); err == nil {
		t.Fatal("an absolute host path must be rejected")
	}
	if !strings.Contains(err.Error(), "remotes/192.168.2.53_backup/bombvault") {
		t.Fatalf("expected the relative-path guidance, got: %v", err)
	}
	if n := eng.probeCount(); n != 0 {
		t.Fatalf("an unresolvable location must not probe, got %d probe(s)", n)
	}
}

// TestOffsiteTargetTestRouteRegisters guards the pattern collision a new route
// under /api/offsite/ can cause: ServeMux PANICS at registration when two
// patterns conflict, and "POST /api/offsite/{domain}/test" sits right next to
// the new "POST /api/offsite/targets/{id}/test". Building the real router proves
// they coexist; the standalone mux proves the per-target pattern is the one that
// matches (and that a normal domain still reaches the domain handler).
func TestOffsiteTargetTestRouteRegisters(t *testing.T) {
	_ = (&Handler{}).Router() // panics on a conflicting pattern

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/offsite/targets/{id}/test", func(http.ResponseWriter, *http.Request) {})
	mux.HandleFunc("POST /api/offsite/{domain}/test", func(http.ResponseWriter, *http.Request) {})

	cases := []struct{ path, want string }{
		{"/api/offsite/targets/abc123/test", "/api/offsite/targets/{id}/test"},
		{"/api/offsite/containers/test", "/api/offsite/{domain}/test"},
	}
	for _, c := range cases {
		_, pattern := mux.Handler(jsonReq(http.MethodPost, c.path, nil))
		if !strings.HasSuffix(pattern, c.want) {
			t.Errorf("POST %s resolved to %q, want %q", c.path, pattern, c.want)
		}
	}
}

// TestHandleTestOffsiteTargetEnvelope pins the HTTP shape the SPA consumes:
// {ok,reachable,initialized} on success, {ok:false,error} on a failing probe.
func TestHandleTestOffsiteTargetEnvelope(t *testing.T) {
	const repo = "rest:http://good:8000/containers"
	svc, st, _ := newProbeSvc(t, map[string]bool{repo: true})
	h := &Handler{store: st, svc: svc}
	tgt, err := st.UpsertOffsiteTarget(store.OffsiteTarget{
		Domain: "containers", Name: "NAS", Repo: repo, Enabled: true,
	})
	if err != nil {
		t.Fatal(err)
	}

	req := jsonReq(http.MethodPost, "/api/offsite/targets/"+tgt.ID+"/test", nil)
	req.SetPathValue("id", tgt.ID)
	rec := httptest.NewRecorder()
	h.handleTestOffsiteTarget(rec, req)
	env := decodeEnvelope(t, rec)
	if env["ok"] != true || env["reachable"] != true || env["initialized"] != true {
		t.Fatalf("ok probe envelope = %v, want ok/reachable/initialized", env)
	}

	req = jsonReq(http.MethodPost, "/api/offsite/targets/nope/test", nil)
	req.SetPathValue("id", "nope")
	rec = httptest.NewRecorder()
	h.handleTestOffsiteTarget(rec, req)
	env = decodeEnvelope(t, rec)
	if env["ok"] != false || env["error"] == "" {
		t.Fatalf("unknown-target envelope = %v, want ok:false with a reason", env)
	}
}
