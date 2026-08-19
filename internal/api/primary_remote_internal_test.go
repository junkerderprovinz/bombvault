package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/notify"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// TestPrimaryLimitsFor pins primaryLimitsFor's four-way contract: only a
// REMOTE repo with an ENABLED saved safety row contributes non-zero Limits —
// every other combination (local, remote-but-unconfigured, remote-but-
// disabled) yields a zero Limits, so BackupArgs emits no --limit-* flags
// (byte-identical to a domain that never touched this feature).
func TestPrimaryLimitsFor(t *testing.T) {
	s, st := newSyncTestService(t)

	if got := s.primaryLimitsFor("containers", "/mnt/user/bombvault/containers"); got != (restic.Limits{}) {
		t.Fatalf("local repo: Limits = %+v, want zero", got)
	}
	if got := s.primaryLimitsFor("containers", "s3:bucket/containers"); got != (restic.Limits{}) {
		t.Fatalf("remote, unconfigured: Limits = %+v, want zero", got)
	}

	if _, err := st.UpsertPrimaryRemoteTarget("containers", store.OffsiteTarget{
		Repo: "s3:bucket/containers", LimitUpload: 1000, LimitDownload: 2000, Enabled: false,
	}); err != nil {
		t.Fatal(err)
	}
	if got := s.primaryLimitsFor("containers", "s3:bucket/containers"); got != (restic.Limits{}) {
		t.Fatalf("remote, saved but disabled: Limits = %+v, want zero", got)
	}

	if _, err := st.UpsertPrimaryRemoteTarget("containers", store.OffsiteTarget{
		Repo: "s3:bucket/containers", LimitUpload: 1000, LimitDownload: 2000, Enabled: true,
	}); err != nil {
		t.Fatal(err)
	}
	want := restic.Limits{UploadKBps: 1000, DownloadKBps: 2000}
	if got := s.primaryLimitsFor("containers", "s3:bucket/containers"); got != want {
		t.Fatalf("remote, saved and enabled: Limits = %+v, want %+v", got, want)
	}
}

// TestPrimaryIsImmutable mirrors TestPrimaryLimitsFor for the append-only gate.
func TestPrimaryIsImmutable(t *testing.T) {
	s, st := newSyncTestService(t)

	if s.primaryIsImmutable("vms", "/mnt/user/bombvault/vms") {
		t.Fatal("a local repo must never report immutable")
	}
	if s.primaryIsImmutable("vms", "rest:http://host:8000/vms") {
		t.Fatal("an unconfigured remote primary must not report immutable")
	}
	if _, err := st.UpsertPrimaryRemoteTarget("vms", store.OffsiteTarget{Repo: "rest:http://host:8000/vms", Immutable: false, Enabled: true}); err != nil {
		t.Fatal(err)
	}
	if s.primaryIsImmutable("vms", "rest:http://host:8000/vms") {
		t.Fatal("a saved-but-not-immutable row must not report immutable")
	}
	if _, err := st.UpsertPrimaryRemoteTarget("vms", store.OffsiteTarget{Repo: "rest:http://host:8000/vms", Immutable: true, Enabled: true}); err != nil {
		t.Fatal(err)
	}
	if !s.primaryIsImmutable("vms", "rest:http://host:8000/vms") {
		t.Fatal("an immutable saved row on a remote repo must report immutable")
	}
}

// forgetTrackingEngine embeds ResticEngine (nil) so every method BombVault
// does not explicitly stub here panics if actually called — a deliberate
// trip-wire, since the whole point of the tests below is that certain paths
// must NEVER reach the engine. ForgetPolicy and Unlock are the two the
// retention path can legitimately reach, so they are the only ones stubbed.
type forgetTrackingEngine struct {
	ResticEngine
	mu      sync.Mutex
	forgetN int
	unlockN int
}

func (e *forgetTrackingEngine) ForgetPolicy(context.Context, string, restic.RetentionPolicy, restic.Mode, string, bool) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.forgetN++
	return nil
}

func (e *forgetTrackingEngine) Unlock(context.Context, string, bool, restic.Mode) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.unlockN++
	return nil
}

func (e *forgetTrackingEngine) forgetCalls() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.forgetN
}

// TestApplyRetentionSkipsPruneForImmutableRemotePrimary is the issue-#152 core
// safety proof: a domain whose primary is remote AND flagged append-only in
// its saved safety settings must NEVER have its retention policy applied
// (skip BEFORE the engine is even touched — forgetCalls stays 0), mirroring
// copyToOffsiteTarget's "never prune an immutable off-site destination"
// behavior for the case where there IS no separate off-site destination.
func TestApplyRetentionSkipsPruneForImmutableRemotePrimary(t *testing.T) {
	s, st := newSyncTestService(t)
	eng := &forgetTrackingEngine{}
	s.engine = eng

	settings, err := st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	settings.RetentionKeepLast = 5 // a real policy — otherwise applyRetention's own p.Any() gate would explain a zero count
	if err := st.UpdateSettings(settings); err != nil {
		t.Fatal(err)
	}
	if _, err := st.UpsertPrimaryRemoteTarget("containers", store.OffsiteTarget{
		Repo: "s3:bucket/containers", Immutable: true, Enabled: true,
	}); err != nil {
		t.Fatal(err)
	}

	s.applyRetention(context.Background(), "s3:bucket/containers", settings, restic.Mode{}, "container:plex", "containers")
	if n := eng.forgetCalls(); n != 0 {
		t.Fatalf("an immutable remote primary must never be pruned, got %d ForgetPolicy call(s)", n)
	}
}

// TestApplyRetentionStillPrunesWhenNotImmutable is the control for the test
// above: the SAME remote repo, but the saved safety row is NOT flagged
// immutable, prunes exactly as it always has.
func TestApplyRetentionStillPrunesWhenNotImmutable(t *testing.T) {
	s, st := newSyncTestService(t)
	eng := &forgetTrackingEngine{}
	s.engine = eng

	settings, err := st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	settings.RetentionKeepLast = 5
	if err := st.UpdateSettings(settings); err != nil {
		t.Fatal(err)
	}
	if _, err := st.UpsertPrimaryRemoteTarget("containers", store.OffsiteTarget{
		Repo: "s3:bucket/containers", Immutable: false, Enabled: true,
	}); err != nil {
		t.Fatal(err)
	}

	s.applyRetention(context.Background(), "s3:bucket/containers", settings, restic.Mode{}, "container:plex", "containers")
	if n := eng.forgetCalls(); n != 1 {
		t.Fatalf("a non-immutable remote primary must prune normally, got %d ForgetPolicy call(s), want 1", n)
	}
}

// TestApplyRetentionStillPrunesLocalPrimary is a second control: a plain LOCAL
// repo (no saved safety row at all — the overwhelming majority of installs)
// is completely unaffected by primaryIsImmutable's remote check.
func TestApplyRetentionStillPrunesLocalPrimary(t *testing.T) {
	s, _ := newSyncTestService(t)
	eng := &forgetTrackingEngine{}
	s.engine = eng

	settings := store.Settings{RetentionKeepLast: 5}
	s.applyRetention(context.Background(), "/mnt/user/bombvault/containers", settings, restic.Mode{}, "container:plex", "containers")
	if n := eng.forgetCalls(); n != 1 {
		t.Fatalf("a local primary must prune normally, got %d ForgetPolicy call(s), want 1", n)
	}
}

// TestSetGetClearPrimaryRemoteConfig covers the service-level CRUD contract:
// SetPrimaryRemoteConfig refuses a local path, accepts + persists a remote
// one (stamping Repo from the LIVE settings, never trusting a caller-supplied
// value), PrimaryRemoteConfig round-trips it, and ClearPrimaryRemoteConfig
// removes it.
func TestSetGetClearPrimaryRemoteConfig(t *testing.T) {
	s, st := newSyncTestService(t)

	settings, err := st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	settings.ContainersPath = "user/bombvault/containers" // a plain LOCAL subpath
	if err := st.UpdateSettings(settings); err != nil {
		t.Fatal(err)
	}

	// A local path is refused — safety settings for a local primary would be inert.
	if _, err := s.SetPrimaryRemoteConfig("containers", store.OffsiteTarget{Immutable: true}); err == nil {
		t.Fatal("expected SetPrimaryRemoteConfig to refuse a local backup path")
	}
	if _, configured, err := s.PrimaryRemoteConfig("containers"); err != nil || configured {
		t.Fatalf("PrimaryRemoteConfig after a refused save = configured:%v err:%v, want false/nil", configured, err)
	}

	// Switch the path to a remote URL, then save safety settings.
	settings.ContainersPath = "s3:bucket/containers"
	if err := st.UpdateSettings(settings); err != nil {
		t.Fatal(err)
	}
	stored, err := s.SetPrimaryRemoteConfig("containers", store.OffsiteTarget{
		Immutable: true, LimitUpload: 500, LimitDownload: 250, GrowthBudgetGB: 50,
	})
	if err != nil {
		t.Fatalf("SetPrimaryRemoteConfig: %v", err)
	}
	if stored.Repo != "s3:bucket/containers" {
		t.Fatalf("stored.Repo = %q, want the live settings path", stored.Repo)
	}
	if !stored.Enabled {
		t.Fatal("SetPrimaryRemoteConfig must always store Enabled=true")
	}

	got, configured, err := s.PrimaryRemoteConfig("containers")
	if err != nil || !configured {
		t.Fatalf("PrimaryRemoteConfig: configured=%v err=%v", configured, err)
	}
	if !got.Immutable || got.LimitUpload != 500 || got.LimitDownload != 250 || got.GrowthBudgetGB != 50 {
		t.Fatalf("PrimaryRemoteConfig round-trip mismatch: %+v", got)
	}

	if err := s.ClearPrimaryRemoteConfig("containers"); err != nil {
		t.Fatalf("ClearPrimaryRemoteConfig: %v", err)
	}
	if _, configured, err := s.PrimaryRemoteConfig("containers"); err != nil || configured {
		t.Fatalf("PrimaryRemoteConfig after clear = configured:%v err:%v, want false/nil", configured, err)
	}
}

// TestSetPrimaryRemoteConfigRejectsUnknownDomain guards the domain whitelist.
func TestSetPrimaryRemoteConfigRejectsUnknownDomain(t *testing.T) {
	s, _ := newSyncTestService(t)
	if _, err := s.SetPrimaryRemoteConfig("nope", store.OffsiteTarget{}); err == nil {
		t.Fatal("expected an error for an unknown domain")
	}
}

// TestCheckPrimaryRemoteBudgetFiresOncePerCrossing mirrors
// TestCheckOffsiteBudgetFiresOncePerCrossing (budget_internal_test.go) for the
// remote-primary path: an over-budget "local" repo_stats sample alarms exactly
// once per false→true crossing. Uses checkGrowthBudget directly (the shared
// core) with a pre-seeded sample so the test does not need a working restic
// engine just to prove the latch.
func TestCheckPrimaryRemoteBudgetFiresOncePerCrossing(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	st := store.New(db)
	if err := st.AddRepoStat(store.RepoStat{
		Domain: "containers", Source: "local", At: 1700000000,
		RawSize: 2 * 1024 * 1024 * 1024, // 2 GiB > 1 GiB budget
	}); err != nil {
		t.Fatal(err)
	}

	ssh := &fakeHostSSH{}
	svc := &Service{
		cfg:               config.Config{AppKey: strings.Repeat("a", 64)},
		store:             st,
		ssh:               ssh,
		offsiteOverBudget: map[string]bool{},
	}
	if err := svc.SetNotifyConfig(notify.Config{On: "failure", Unraid: true}); err != nil {
		t.Fatal(err)
	}

	svc.checkGrowthBudget(context.Background(), "containers", "local", "primary:containers", 1, "primary")
	svc.checkGrowthBudget(context.Background(), "containers", "local", "primary:containers", 1, "primary") // still over → no second alarm

	if len(ssh.runs) != 1 {
		t.Fatalf("a budget breach must alarm exactly once per crossing, got %d", len(ssh.runs))
	}
	if joined := strings.Join(ssh.runs[0], " "); !strings.Contains(joined, "over budget") {
		t.Fatalf("the notification should announce the budget breach, got %v", ssh.runs[0])
	}
}

// TestCheckPrimaryRemoteBudgetLatchIndependentOfOffsite proves the two budget
// latches (a domain's off-site destination vs. its remote primary) never
// collide: crossing one must not silently "use up" the other's alarm.
func TestCheckPrimaryRemoteBudgetLatchIndependentOfOffsite(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	st := store.New(db)
	for _, source := range []string{"local", "offsite"} {
		if err := st.AddRepoStat(store.RepoStat{
			Domain: "containers", Source: source, At: 1700000000, RawSize: 2 * 1024 * 1024 * 1024,
		}); err != nil {
			t.Fatal(err)
		}
	}
	ssh := &fakeHostSSH{}
	svc := &Service{
		cfg:               config.Config{AppKey: strings.Repeat("a", 64)},
		store:             st,
		ssh:               ssh,
		offsiteOverBudget: map[string]bool{},
	}
	if err := svc.SetNotifyConfig(notify.Config{On: "failure", Unraid: true}); err != nil {
		t.Fatal(err)
	}

	// Cross the OFF-SITE budget first ...
	svc.checkGrowthBudget(context.Background(), "containers", "offsite", "containers", 1, "off-site")
	// ... then the PRIMARY budget: a SEPARATE crossing, must alarm too (2 total).
	svc.checkGrowthBudget(context.Background(), "containers", "local", "primary:containers", 1, "primary")

	if len(ssh.runs) != 2 {
		t.Fatalf("off-site and primary budget crossings must alarm independently, got %d alarm(s)", len(ssh.runs))
	}
}

// TestCheckPrimaryRemoteBudgetNoOpCases pins the silent-no-op shapes:
// checkPrimaryRemoteBudget must never sample or compare anything for a local
// repo, or for a remote repo with no saved config, or with a config whose
// budget is 0 (off).
func TestCheckPrimaryRemoteBudgetNoOpCases(t *testing.T) {
	s, st := newSyncTestService(t)
	settings, err := st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}

	// Local repo: never even reaches the remote-check.
	s.checkPrimaryRemoteBudget(context.Background(), "containers", "/mnt/user/bombvault/containers", settings)

	// Remote, unconfigured.
	s.checkPrimaryRemoteBudget(context.Background(), "containers", "s3:bucket/containers", settings)

	// Remote, configured, budget off (0, the zero value).
	if _, err := st.UpsertPrimaryRemoteTarget("containers", store.OffsiteTarget{Repo: "s3:bucket/containers", Enabled: true}); err != nil {
		t.Fatal(err)
	}
	s.checkPrimaryRemoteBudget(context.Background(), "containers", "s3:bucket/containers", settings)

	// None of the above must have latched anything (they never got far enough
	// to sample/compare — CollectStats against a nil/absent engine would panic
	// if reached, so a clean return here IS the proof).
	if len(s.offsiteOverBudget) != 0 {
		t.Fatalf("no-op cases must not touch the budget latch, got %+v", s.offsiteOverBudget)
	}
}

// ---------------------------------------------------------------------------
// HTTP handlers
// ---------------------------------------------------------------------------

// TestPrimaryRemoteHandlersEnvelope covers the GET/PUT/DELETE round trip's
// wire shape end-to-end through the HTTP handlers (not just the service
// methods above).
func TestPrimaryRemoteHandlersEnvelope(t *testing.T) {
	h, st := newCRUDHandler(t)

	settings, err := st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	settings.VMsPath = "rest:http://host:8000/vms"
	if err := st.UpdateSettings(settings); err != nil {
		t.Fatal(err)
	}

	// GET before any save: configured=false.
	req := httptest.NewRequest(http.MethodGet, "/api/settings/primary-remote/vms", nil)
	req.SetPathValue("domain", "vms")
	rec := httptest.NewRecorder()
	h.handleGetPrimaryRemote(rec, req)
	env := decodeEnvelope(t, rec)
	cfg, _ := env["config"].(map[string]any)
	if cfg["configured"] != false {
		t.Fatalf("GET before save: configured = %v, want false", cfg["configured"])
	}

	// PUT saves it.
	body, _ := json.Marshal(primaryRemoteView{Immutable: true, LimitUpload: 100, LimitDownload: 200, GrowthBudgetGB: 10})
	req = httptest.NewRequest(http.MethodPut, "/api/settings/primary-remote/vms", bytes.NewReader(body))
	req.SetPathValue("domain", "vms")
	rec = httptest.NewRecorder()
	h.handleSetPrimaryRemote(rec, req)
	env = decodeEnvelope(t, rec)
	if env["ok"] != true {
		t.Fatalf("PUT not ok: %v", env)
	}
	cfg, _ = env["config"].(map[string]any)
	if cfg["configured"] != true || cfg["immutable"] != true || cfg["limitUpload"].(float64) != 100 {
		t.Fatalf("PUT response mismatch: %v", cfg)
	}

	// GET now reflects the save.
	req = httptest.NewRequest(http.MethodGet, "/api/settings/primary-remote/vms", nil)
	req.SetPathValue("domain", "vms")
	rec = httptest.NewRecorder()
	h.handleGetPrimaryRemote(rec, req)
	env = decodeEnvelope(t, rec)
	cfg, _ = env["config"].(map[string]any)
	if cfg["configured"] != true || cfg["growthBudgetGb"].(float64) != 10 {
		t.Fatalf("GET after save mismatch: %v", cfg)
	}

	// DELETE clears it.
	req = httptest.NewRequest(http.MethodDelete, "/api/settings/primary-remote/vms", nil)
	req.SetPathValue("domain", "vms")
	rec = httptest.NewRecorder()
	h.handleDeletePrimaryRemote(rec, req)
	if env := decodeEnvelope(t, rec); env["ok"] != true {
		t.Fatalf("DELETE not ok: %v", env)
	}
	req = httptest.NewRequest(http.MethodGet, "/api/settings/primary-remote/vms", nil)
	req.SetPathValue("domain", "vms")
	rec = httptest.NewRecorder()
	h.handleGetPrimaryRemote(rec, req)
	env = decodeEnvelope(t, rec)
	cfg, _ = env["config"].(map[string]any)
	if cfg["configured"] != false {
		t.Fatalf("GET after delete: configured = %v, want false", cfg["configured"])
	}
}

// TestSetPrimaryRemoteHandlerRejectsLocalPath: the handler surfaces
// SetPrimaryRemoteConfig's "not remote" refusal as ok:false with a reason,
// not a 5xx or a silently-accepted no-op row.
func TestSetPrimaryRemoteHandlerRejectsLocalPath(t *testing.T) {
	h, st := newCRUDHandler(t)
	settings, err := st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	settings.FlashPath = "user/bombvault/flash" // local
	if err := st.UpdateSettings(settings); err != nil {
		t.Fatal(err)
	}

	body, _ := json.Marshal(primaryRemoteView{Immutable: true})
	req := httptest.NewRequest(http.MethodPut, "/api/settings/primary-remote/flash", bytes.NewReader(body))
	req.SetPathValue("domain", "flash")
	rec := httptest.NewRecorder()
	h.handleSetPrimaryRemote(rec, req)
	env := decodeEnvelope(t, rec)
	if env["ok"] != false || env["error"] == "" {
		t.Fatalf("expected a rejection with a reason, got %v", env)
	}
}

// TestPrimaryRemoteHandlersRejectUnknownDomain covers the {domain} path-value
// guard shared by every handler in this file.
func TestPrimaryRemoteHandlersRejectUnknownDomain(t *testing.T) {
	h, _ := newCRUDHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/api/settings/primary-remote/nope", nil)
	req.SetPathValue("domain", "nope")
	rec := httptest.NewRecorder()
	h.handleGetPrimaryRemote(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

// TestPrimaryRemoteRoutesRegister proves the new /api/settings/primary-remote/
// routes coexist with the rest of the router without a pattern collision
// (http.ServeMux panics at registration time on a conflict) — the same guard
// TestOffsiteTargetTestRouteRegisters applies to the off-site routes.
func TestPrimaryRemoteRoutesRegister(t *testing.T) {
	_ = (&Handler{}).Router() // panics on a conflicting pattern
}

// TestTestPrimaryRepoAndTamperTest covers the two probe endpoints end-to-end
// through the SERVICE methods (TestPrimaryRepo / RunPrimaryTamperTest), reusing
// probeStubEngine from offsite_target_probe_internal_test.go.
func TestTestPrimaryRepoAndTamperTest(t *testing.T) {
	const repo = "rest:http://good:8000/containers"
	svc, st, eng := newProbeSvc(t, map[string]bool{repo: true})

	settings, err := st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}

	// No path configured for the domain at all → a clear error, nothing probed.
	if _, _, err := svc.TestPrimaryRepo(context.Background(), "containers"); err == nil {
		t.Fatal("expected an error when no backup path is configured")
	}

	// A local path → rejected without probing.
	settings.ContainersPath = "user/bombvault/containers"
	if err := st.UpdateSettings(settings); err != nil {
		t.Fatal(err)
	}
	if _, _, err := svc.TestPrimaryRepo(context.Background(), "containers"); err == nil {
		t.Fatal("expected an error for a local backup path")
	}
	if n := eng.probeCount(); n != 0 {
		t.Fatalf("a local path must never reach the engine, got %d probe(s)", n)
	}

	// A remote, reachable path → reachable+initialized.
	settings.ContainersPath = repo
	if err := st.UpdateSettings(settings); err != nil {
		t.Fatal(err)
	}
	reachable, initialized, err := svc.TestPrimaryRepo(context.Background(), "containers")
	if err != nil || !reachable || !initialized {
		t.Fatalf("TestPrimaryRepo = (%v, %v, %v), want reachable+initialized", reachable, initialized, err)
	}

	// Tamper test on an unsaved config: a clear "save first" error, not a
	// crash or a silent probe under a synthetic id.
	if _, err := svc.RunPrimaryTamperTest(context.Background(), "containers"); err == nil {
		t.Fatal("expected RunPrimaryTamperTest to require a saved safety config first")
	}
}
