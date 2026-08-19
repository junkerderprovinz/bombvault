package api

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"

	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// ---------------------------------------------------------------------------
// Remote primary repositories (issue #152)
//
// A domain's primary Backup Path (Settings.<Domain>Path) already accepts a raw
// restic remote URL (s3:/rest:/sftp:/b2:/rclone:...) and resolveRepo passes it
// to restic verbatim, exactly like a local path — nothing about backup PATH
// RESOLUTION changes here. What was missing is a place to configure the SAFETY
// settings an off-site DESTINATION already gets (bandwidth limits, append-
// only/tamper-test protection, a growth-budget alarm) for when a domain's
// PRIMARY happens to be remote.
//
// Rather than a parallel schema, this reuses store.OffsiteTarget itself: a
// "primary" role row (store.RolePrimary), at most one per domain, holds ONLY
// those safety fields. It is never a replication destination — it is excluded
// from every off-site query (store.OffsiteTargetsForDomain, ListOffsiteTargets,
// GetOffsiteTarget, DeleteOffsiteTarget all filter to role="offsite") — so the
// existing multi-target replication loop, the off-site CRUD UI and the
// primaryOffsiteTarget/offsiteTargetsFor helpers never see it. Reusing the
// struct means every existing per-target mechanism already works on a
// "primary" row for free: limitFlags (via Mode.Limits, primaryLimitsFor),
// offsiteModeForTarget's CredsRef/StorageClass resolution, and
// runTamperTestForTarget's REST-delete probe (with its verdict recorded under
// the row's own id, exactly like any other target).
//
// The row's Repo field is a best-effort snapshot of Settings.<Domain>Path at
// the moment the safety settings were last saved — it is NEVER authoritative
// for backup path resolution (Settings.<Domain>Path always is, read directly
// by containersRepoPath/vmsRepoPath/etc, unchanged). It exists only to give
// the connection-test and tamper-test probes, and the tamper probe's `rest:`
// prefix check, a repo string; TestPrimaryRepo and RunPrimaryTamperTest below
// always refresh it from the LIVE settings before probing, so a stale row
// (the path changed after the safety settings were last saved) can never test
// the wrong location.
// ---------------------------------------------------------------------------

// domainPathRaw returns the UNRESOLVED backup path configured for domain — the
// raw Settings.<Domain>Path value, which may be a plain relative local subpath
// or a restic remote URL. Unlike containersRepoPath/vmsRepoPath/flashRepoPath/
// configRepoPath/filesRepoPath it does not resolve or validate the value, so a
// caller can test it with restic.IsRemoteRepo BEFORE deciding whether
// resolveRepo even applies (a local subpath and a remote URL take different
// paths through it).
func domainPathRaw(domain string, settings store.Settings) string {
	switch domain {
	case "containers":
		return settings.ContainersPath
	case "vms":
		return settings.VMsPath
	case "flash":
		return settings.FlashPath
	case "config":
		return settings.ConfigPath
	case "files":
		return settings.FilesPath
	}
	return ""
}

// primaryRemoteTarget returns the domain's remote-primary safety-config row
// (store.RolePrimary), if one has been saved. ok=false (never an error to the
// caller — a store failure is logged and treated the same as "unconfigured")
// is the common case: a local primary, or a remote primary nobody has opened
// the safety dialog for yet. Every call site below treats that as "apply no
// bandwidth limit, no immutable-skip, no growth-budget check" — so an install
// with a hand-typed remote path and no saved safety row keeps behaving exactly
// as it does today (issue #152's starting point: "already works, just with
// none of the safety net off-site destinations get").
func (s *Service) primaryRemoteTarget(domain string) (store.OffsiteTarget, bool) {
	if s.store == nil {
		return store.OffsiteTarget{}, false
	}
	t, ok, err := s.store.PrimaryRemoteTarget(domain)
	if err != nil {
		log.Printf("api: primary-remote %s: read failed (treating as unconfigured): %v", domain, err) //nolint:gosec // G706: domain is a fixed literal
		return store.OffsiteTarget{}, false
	}
	return t, ok
}

// primaryLimitsFor returns the restic.Limits to fold into Mode.Limits for a
// domain's PRIMARY backup: the saved safety row's caps when repo is remote AND
// a row exists AND it is enabled, else a zero (unlimited) Limits — so a local
// primary, or a remote primary with no saved safety settings, builds a Mode
// byte-identical to one built before this field existed (BackupArgs then
// emits no --limit-* flags at all, see limitFlags).
func (s *Service) primaryLimitsFor(domain, repo string) restic.Limits {
	if !restic.IsRemoteRepo(repo) {
		return restic.Limits{}
	}
	t, ok := s.primaryRemoteTarget(domain)
	if !ok || !t.Enabled {
		return restic.Limits{}
	}
	return restic.Limits{UploadKBps: t.LimitUpload, DownloadKBps: t.LimitDownload}
}

// primaryIsImmutable reports whether repo is a domain's remote primary AND its
// saved safety settings flag it append-only — the primary-repo counterpart of
// offsiteImmutableFor, used by applyRetention to skip the local retention
// prune exactly like copyToOffsiteTarget skips it for an immutable off-site
// destination (the credentials on this box must not be able to delete the
// only copy of the backup either).
func (s *Service) primaryIsImmutable(domain, repo string) bool {
	if !restic.IsRemoteRepo(repo) {
		return false
	}
	t, ok := s.primaryRemoteTarget(domain)
	return ok && t.Enabled && t.Immutable
}

// PrimaryRemoteConfig returns the domain's saved remote-primary safety
// settings (ok=false, no error, when none have been saved yet — see
// primaryRemoteTarget) for the Settings UI to pre-fill the inline "Remote"
// safety dialog.
func (s *Service) PrimaryRemoteConfig(domain string) (store.OffsiteTarget, bool, error) {
	if !validOffsiteDomain(domain) {
		return store.OffsiteTarget{}, false, fmt.Errorf("unknown domain %q", domain)
	}
	return s.store.PrimaryRemoteTarget(domain)
}

// SetPrimaryRemoteConfig saves a domain's remote-primary safety settings
// (bandwidth limits, append-only, growth budget). It refuses when the domain's
// CURRENT backup path is not actually a remote repo (safety settings for a
// local primary are meaningless and would silently do nothing — better to say
// so than let the UI show a saved-but-inert config) and stamps Repo from the
// live Settings.<Domain>Path so the row can never drift from what is being
// probed. cfg.Enabled is forced true: this endpoint is the ONLY writer of a
// "primary" row and always means "the safety settings below are active" — a
// disabled-but-present row is not a state the UI exposes.
func (s *Service) SetPrimaryRemoteConfig(domain string, cfg store.OffsiteTarget) (store.OffsiteTarget, error) {
	if !validOffsiteDomain(domain) {
		return store.OffsiteTarget{}, fmt.Errorf("unknown domain %q", domain)
	}
	settings, err := s.store.GetSettings()
	if err != nil {
		return store.OffsiteTarget{}, fmt.Errorf("read settings: %w", err)
	}
	loc := domainPathRaw(domain, settings)
	if loc == "" || !restic.IsRemoteRepo(loc) {
		return store.OffsiteTarget{}, errors.New("this domain's backup path is not a remote repository — set a remote URL (s3:/rest:/sftp:/b2:/rclone:...) on the path field first")
	}
	cfg.Repo = loc
	cfg.Enabled = true
	cfg.LimitUpload = max(0, cfg.LimitUpload)
	cfg.LimitDownload = max(0, cfg.LimitDownload)
	cfg.GrowthBudgetGB = max(0, cfg.GrowthBudgetGB)
	return s.store.UpsertPrimaryRemoteTarget(domain, cfg)
}

// ClearPrimaryRemoteConfig removes a domain's saved remote-primary safety
// settings (a no-op, no error, when none exist) — used when the operator
// switches the domain's path back to a local folder, or wants to reset the
// safety config without necessarily changing the path.
func (s *Service) ClearPrimaryRemoteConfig(domain string) error {
	if !validOffsiteDomain(domain) {
		return fmt.Errorf("unknown domain %q", domain)
	}
	return s.store.DeletePrimaryRemoteTarget(domain)
}

// TestPrimaryRepo probes a domain's CURRENT backup path the same
// reachable/initialised way TestOffsite probes an off-site destination — see
// probeOffsiteRepo for the full reachable/initialized/err contract. It always
// resolves the LIVE Settings.<Domain>Path (never a possibly-stale saved safety
// row's Repo) and applies that row's CredsRef/StorageClass when one exists
// (offsiteModeForTarget, reused verbatim — it already works on any
// store.OffsiteTarget regardless of role).
func (s *Service) TestPrimaryRepo(ctx context.Context, domain string) (reachable, initialized bool, err error) {
	if !validOffsiteDomain(domain) {
		return false, false, fmt.Errorf("unknown domain %q", domain)
	}
	settings, err := s.store.GetSettings()
	if err != nil {
		return false, false, fmt.Errorf("read settings: %w", err)
	}
	loc := domainPathRaw(domain, settings)
	if loc == "" {
		return false, false, errors.New("no backup path configured for this domain")
	}
	if !restic.IsRemoteRepo(loc) {
		return false, false, errors.New("this domain's backup path is local, not remote")
	}
	repo, err := s.resolveRepo(loc)
	if err != nil {
		return false, false, err
	}
	mode := s.ModeFor(settings)
	if t, ok := s.primaryRemoteTarget(domain); ok {
		t.Repo = loc // always probe the LIVE path, never a possibly-stale saved one
		mode = s.offsiteModeForTarget(settings, t)
	}
	return s.probeOffsiteRepo(ctx, repo, mode)
}

// RunPrimaryTamperTest runs the SAME active append-only probe RunTamperTest
// runs for an off-site destination (runTamperTestForTarget: two side-effect-
// free DELETEs against provably non-existent objects on the far-side
// rest-server), against a domain's remote PRIMARY instead. It requires a saved
// safety row (created by the first SetPrimaryRemoteConfig call, which the
// Settings UI makes whenever append-only is switched on — mirroring
// OffsiteWizard's toggleImmutable-then-tamper-test flow) so the verdict has a
// real target id to record against; an un-configured remote primary is asked
// to save the safety settings first rather than silently probing under a
// synthetic empty id that could collide with the domain's off-site verdict.
func (s *Service) RunPrimaryTamperTest(ctx context.Context, domain string) (verdict TamperVerdict, err error) {
	if !validOffsiteDomain(domain) {
		return TamperVerdict{}, fmt.Errorf("unknown domain %q", domain)
	}
	// Reuse the SAME per-domain tamper lock the off-site test uses: the two
	// probe DIFFERENT targets (different ids), so there is no correctness
	// requirement to share it, but serialising avoids two concurrent tamper
	// tests for the same domain racing their run-row/progress bookkeeping.
	defer s.lockTamper(domain)()
	tkey := "tamper:primary:" + domain
	s.progBegin(ctx, tkey, "maintenance")
	defer func() { s.progEnd(tkey, "maintenance", err == nil) }()

	settings, err := s.store.GetSettings()
	if err != nil {
		return TamperVerdict{}, fmt.Errorf("read settings: %w", err)
	}
	loc := domainPathRaw(domain, settings)
	if loc == "" || !restic.IsRemoteRepo(loc) {
		return TamperVerdict{}, errors.New("this domain's backup path is not a remote repository")
	}
	target, ok := s.primaryRemoteTarget(domain)
	if !ok {
		return TamperVerdict{}, errors.New("save the remote-primary safety settings (with append-only on) before running a tamper test")
	}
	target.Repo = loc // always probe the LIVE path, never a possibly-stale saved one

	runID, rErr := s.store.StartRun(domainRunTargetID(domain), "tamper")
	if rErr != nil {
		log.Printf("api: primary tamper %s: could not start run record (continuing): %v", domain, rErr) //nolint:gosec // G706: domain is a fixed literal
		runID = ""
	}
	defer func() {
		if runID == "" {
			return
		}
		status := "success"
		detail := verdict.Detail
		switch {
		case err != nil:
			status = statusSkipped
			detail = truncateRunErr(err)
		case !verdict.Testable:
			status = statusSkipped
		case !verdict.Protected:
			status = "failed"
		}
		const maxDetail = 500
		if len(detail) > maxDetail {
			detail = detail[:maxDetail]
		}
		if fErr := s.store.FinishRun(runID, status, "", 0, detail); fErr != nil {
			log.Printf("api: primary tamper %s: could not finish run record: %v", domain, fErr) //nolint:gosec // G706: domain is a fixed literal
		}
	}()

	creds, _ := s.decodeCloud(settings)
	return s.runTamperTestForTarget(ctx, domain, target, creds)
}

// handlePrimaryRemoteDomain validates the {domain} path value shared by every
// primary-remote handler below, writing the response and returning ok=false on
// an invalid one so the caller can just `if !ok { return }`.
func handlePrimaryRemoteDomain(w http.ResponseWriter, r *http.Request) (string, bool) {
	domain := r.PathValue("domain")
	if !validOffsiteDomain(domain) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "unknown domain"})
		return "", false
	}
	return domain, true
}

// primaryRemoteView is the JSON wire shape of a domain's remote-primary safety
// settings. Deliberately narrower than offsiteTargetView (no id/name/schedule/
// retention/creds/storage-class — those are either meaningless for a primary
// row today or already exposed via other cards): only the fields the inline
// "Remote" dialog actually edits.
type primaryRemoteView struct {
	Configured     bool   `json:"configured"`
	Repo           string `json:"repo"`
	Immutable      bool   `json:"immutable"`
	LimitUpload    int    `json:"limitUpload"`
	LimitDownload  int    `json:"limitDownload"`
	GrowthBudgetGB int    `json:"growthBudgetGb"`
}

func primaryRemoteToView(t store.OffsiteTarget, configured bool) primaryRemoteView {
	return primaryRemoteView{
		Configured:     configured,
		Repo:           t.Repo,
		Immutable:      t.Immutable,
		LimitUpload:    t.LimitUpload,
		LimitDownload:  t.LimitDownload,
		GrowthBudgetGB: t.GrowthBudgetGB,
	}
}

// handleGetPrimaryRemote returns a domain's saved remote-primary safety
// settings. GET /api/settings/primary-remote/{domain}
func (h *Handler) handleGetPrimaryRemote(w http.ResponseWriter, r *http.Request) {
	domain, ok := handlePrimaryRemoteDomain(w, r)
	if !ok {
		return
	}
	t, configured, err := h.svc.PrimaryRemoteConfig(domain)
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"config": primaryRemoteToView(t, configured)}))
}

// handleSetPrimaryRemote saves a domain's remote-primary safety settings.
// PUT /api/settings/primary-remote/{domain}  body: {immutable,limitUpload,limitDownload,growthBudgetGb}
func (h *Handler) handleSetPrimaryRemote(w http.ResponseWriter, r *http.Request) {
	domain, ok := handlePrimaryRemoteDomain(w, r)
	if !ok {
		return
	}
	var v primaryRemoteView
	if !decodeBody(w, r, &v) {
		return
	}
	stored, err := h.svc.SetPrimaryRemoteConfig(domain, store.OffsiteTarget{
		Immutable:      v.Immutable,
		LimitUpload:    v.LimitUpload,
		LimitDownload:  v.LimitDownload,
		GrowthBudgetGB: v.GrowthBudgetGB,
	})
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"config": primaryRemoteToView(stored, true)}))
}

// handleDeletePrimaryRemote clears a domain's saved remote-primary safety
// settings. DELETE /api/settings/primary-remote/{domain}
func (h *Handler) handleDeletePrimaryRemote(w http.ResponseWriter, r *http.Request) {
	domain, ok := handlePrimaryRemoteDomain(w, r)
	if !ok {
		return
	}
	if err := h.svc.ClearPrimaryRemoteConfig(domain); err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(nil))
}

// handleTestPrimaryRemote probes a domain's current (live) backup path.
// POST /api/settings/primary-remote/{domain}/test
func (h *Handler) handleTestPrimaryRemote(w http.ResponseWriter, r *http.Request) {
	domain, ok := handlePrimaryRemoteDomain(w, r)
	if !ok {
		return
	}
	reachable, initialized, err := h.svc.TestPrimaryRepo(r.Context(), domain)
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{
		"reachable":   reachable,
		"initialized": initialized,
	}))
}

// handlePrimaryRemoteTamperTest runs an active append-only probe against a
// domain's remote primary. POST /api/settings/primary-remote/{domain}/tamper-test
func (h *Handler) handlePrimaryRemoteTamperTest(w http.ResponseWriter, r *http.Request) {
	domain, ok := handlePrimaryRemoteDomain(w, r)
	if !ok {
		return
	}
	verdict, err := h.svc.RunPrimaryTamperTest(r.Context(), domain)
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{
		"testable":  verdict.Testable,
		"protected": verdict.Protected,
		"detail":    verdict.Detail,
	}))
}
