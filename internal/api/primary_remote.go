package api

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// A domain's primary backup path may be a restic remote URL, which resolveRepo
// passes through like a local path. The safety settings an off-site destination
// has (bandwidth limits, append-only with its tamper test, a growth budget) are
// kept for a remote primary in a store.OffsiteTarget row with role "primary",
// at most one per domain. Every off-site query filters to role "offsite", so
// replication and the off-site UI never see it, while limitFlags,
// offsiteModeForTarget and runTamperTestForTarget work on it unchanged.
//
// The row's Repo is a snapshot of the path from the last save and plays no part
// in resolving the backup path. TestPrimaryRepo and RunPrimaryTamperTest
// replace it with the live path before probing.

// domainPathRaw returns the backup path configured for domain as typed: a
// relative local subpath or a restic remote URL. Unlike containersRepoPath and
// its siblings it does not resolve the value, so a caller can check
// restic.IsRemoteRepo first.
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
	case "zfs":
		return settings.ZFSPath
	}
	return ""
}

// primaryRemoteTarget returns the domain's remote-primary safety row, if one
// has been saved. A store error is logged and treated as no row, which means no
// bandwidth limit, no append-only skip and no growth budget.
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

// primaryLimitsFor returns the bandwidth caps for a domain's primary backup:
// those of the saved row when repo is remote and the row is enabled, otherwise
// none, in which case limitFlags adds no --limit-* flags.
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

// primaryModeFor builds the restic mode for a domain's primary backup: the
// shared mode plus the saved row's bandwidth caps and, when it names one, its
// credential set. TestPrimaryRepo applies the credential set as well, so
// leaving it out here would give a passing connection test and backups that
// run with the shared credentials.
func (s *Service) primaryModeFor(settings store.Settings, domain, repo string) restic.Mode {
	mode := s.ModeFor(settings)
	// A named repository answers for itself. The domain's row describes the
	// domain's own repository; applied here it would back up a container
	// pointed at b2:bucket/cold with the keys saved for the domain's s3: primary.
	if named, ok := s.namedRepoForLocation(repo); ok {
		mode.Limits = restic.Limits{UploadKBps: named.LimitUpload, DownloadKBps: named.LimitDownload}
		if restic.IsRemoteRepo(repo) {
			mode = s.applyTargetCreds(mode, settings, named)
			// As in offsiteModeForTarget, the row's own storage class beats the
			// credential set's default, which is why it is applied second.
			if named.StorageClass != "" {
				mode.StorageClass = named.StorageClass
			}
		}
		return mode
	}
	mode.Limits = s.primaryLimitsFor(domain, repo)
	if !restic.IsRemoteRepo(repo) {
		return mode
	}
	t, ok := s.primaryRemoteTarget(domain)
	if !ok {
		return mode
	}
	return s.applyTargetCreds(mode, settings, t)
}

// namedRepoForLocation finds the named repository a resolved location belongs
// to, so the mode and the append-only question are answered from its row. It
// matches on the location because resolution happened several layers up, and
// validateNamedRepo refuses two rows with the same location.
func (s *Service) namedRepoForLocation(repo string) (store.OffsiteTarget, bool) {
	if strings.TrimSpace(repo) == "" {
		return store.OffsiteTarget{}, false
	}
	rows, err := s.store.ListNamedRepos()
	if err != nil {
		// "Not a named repository" is a safe answer for callers that want a
		// mode. primaryAppendOnly reads the store itself so it can fail closed.
		log.Printf("api: could not read the named repositories while resolving a location: %v", err)
		return store.OffsiteTarget{}, false
	}
	for _, r := range rows {
		// sameRepoLocation, as the refusals use: with == a trailing slash would
		// give one bucket two sets of credentials.
		if loc, rErr := s.resolveRepo(r.Repo); rErr == nil && sameRepoLocation(loc, repo) {
			return r, true
		}
	}
	return store.OffsiteTarget{}, false
}

// primaryIsImmutable reports whether repo is flagged append-only, the primary
// counterpart of offsiteImmutableFor: the credentials on this box must not be
// able to delete the only copy. pruneDomain asks refAppendOnly instead.
//
// The named row is consulted before the remote check. A local primary has no
// safety row, but a named repository such as backups/cold on a NAS share can
// be append-only either way.
func (s *Service) primaryIsImmutable(domain, repo string) bool {
	return s.primaryAppendOnly(domain, repo) != appendOnlyNone
}

// appendOnlyFlag says which toggle protects a repository, so the refusal can
// send the operator to the card that toggle lives on.
type appendOnlyFlag int

const (
	appendOnlyNone appendOnlyFlag = iota
	// The flag is on the named repository's own row (Settings, Repositories).
	appendOnlyNamedRepo
	// The flag is on the domain's remote-primary safety row.
	appendOnlyPrimaryRemote
	// Nobody's flag: a store read failed and the safe answer was given.
	appendOnlyUnreadable
)

// appendOnlyRefusal turns the flag into the error to return. appendOnlyNone
// should never get here and receives the generic refusal rather than nil.
func appendOnlyRefusal(f appendOnlyFlag) error {
	switch f {
	case appendOnlyPrimaryRemote:
		return errAppendOnlyPrimaryRemote
	case appendOnlyUnreadable:
		return errAppendOnlyUnknown
	case appendOnlyNamedRepo, appendOnlyNone:
	}
	return errOffsiteAppendOnly
}

// primaryAppendOnly is primaryIsImmutable with the reason kept.
func (s *Service) primaryAppendOnly(domain, repo string) appendOnlyFlag {
	// A read failure counts as append-only. This gates forget and prune, and a
	// prune that waits a night is cheaper than history that does not come back.
	if _, err := s.store.ListNamedRepos(); err != nil {
		log.Printf("api: could not read the named repositories; treating %s as append-only until it can be read", shortRepoName(repo)) //nolint:gosec // G706: the name is shortened
		return appendOnlyUnreadable
	}
	// As in primaryModeFor, a named repository answers for itself; the domain's
	// flag says nothing about it.
	if named, ok := s.namedRepoForLocation(repo); ok {
		if named.Enabled && named.Immutable {
			return appendOnlyNamedRepo
		}
		return appendOnlyNone
	}
	if !restic.IsRemoteRepo(repo) {
		return appendOnlyNone
	}
	// Not through primaryRemoteTarget: it treats a read failure as unconfigured,
	// which here would mean unprotected in front of forget and prune.
	t, ok, err := s.store.PrimaryRemoteTarget(domain)
	if err != nil {
		log.Printf("api: could not read the remote-primary safety row for %s; treating %s as append-only until it can be read", domain, shortRepoName(repo)) //nolint:gosec // G706: domain is a fixed literal and the name is shortened
		return appendOnlyUnreadable
	}
	if ok && t.Enabled && t.Immutable {
		return appendOnlyPrimaryRemote
	}
	return appendOnlyNone
}

// PrimaryRemoteConfig returns the domain's saved remote-primary safety
// settings for the Remote dialog; ok is false when none have been saved.
func (s *Service) PrimaryRemoteConfig(domain string) (store.OffsiteTarget, bool, error) {
	if !validOffsiteDomain(domain) {
		return store.OffsiteTarget{}, false, fmt.Errorf("unknown domain %q", domain)
	}
	return s.store.PrimaryRemoteTarget(domain)
}

// SetPrimaryRemoteConfig saves a domain's remote-primary safety settings. It
// refuses when the current backup path is not remote, where the settings would
// do nothing, and takes Repo from the live path. Enabled is always set: this is
// the only writer of a primary row, and the UI has no disabled state for it.
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
		return store.OffsiteTarget{}, errors.New("this domain's backup path is not a remote repository. Set a remote URL (s3:/rest:/sftp:/b2:/rclone:...) on the path field first")
	}
	cfg.Repo = loc
	cfg.Enabled = true
	cfg.LimitUpload = max(0, cfg.LimitUpload)
	cfg.LimitDownload = max(0, cfg.LimitDownload)
	cfg.GrowthBudgetGB = max(0, cfg.GrowthBudgetGB)
	return s.store.UpsertPrimaryRemoteTarget(domain, cfg)
}

// ClearPrimaryRemoteConfig removes a domain's remote-primary safety settings.
// Clearing settings that do not exist is not an error.
func (s *Service) ClearPrimaryRemoteConfig(domain string) error {
	if !validOffsiteDomain(domain) {
		return fmt.Errorf("unknown domain %q", domain)
	}
	return s.store.DeletePrimaryRemoteTarget(domain)
}

// TestPrimaryRepo probes a domain's current backup path the way TestOffsite
// probes a destination (see probeOffsiteRepo). It uses the live path, not the
// saved row's Repo, and applies the row's CredsRef and StorageClass when a row
// exists.
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
		t.Repo = loc
		mode = s.offsiteModeForTarget(settings, t)
	}
	return s.probeOffsiteRepo(ctx, repo, mode)
}

// RunPrimaryTamperTest runs RunTamperTest's append-only probe (two harmless
// DELETEs of objects that cannot exist) against a domain's remote primary. It
// needs a saved safety row so the verdict has a target id of its own, rather
// than an empty one that could collide with the off-site verdict.
func (s *Service) RunPrimaryTamperTest(ctx context.Context, domain string) (verdict TamperVerdict, err error) {
	if !validOffsiteDomain(domain) {
		return TamperVerdict{}, fmt.Errorf("unknown domain %q", domain)
	}
	// The off-site test probes a different target, but sharing its lock keeps
	// two tamper tests of one domain from racing their run and progress rows.
	defer s.lockTamper(domain)()
	tkey := "tamper:primary:" + domain
	_, startedAt := s.progBegin(ctx, tkey, "maintenance")
	defer func() { s.progEnd(tkey, "maintenance", err == nil, startedAt) }()

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
	target.Repo = loc

	runID, rErr := s.startRun(ctx, domainRunTargetID(domain), "tamper")
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

// handlePrimaryRemoteDomain validates the {domain} path value of the
// primary-remote handlers and writes the error response when it is invalid.
func handlePrimaryRemoteDomain(w http.ResponseWriter, r *http.Request) (string, bool) {
	domain := r.PathValue("domain")
	if !validOffsiteDomain(domain) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "unknown domain"})
		return "", false
	}
	return domain, true
}

// primaryRemoteView holds the fields of a domain's remote-primary safety
// settings that the Remote dialog edits.
type primaryRemoteView struct {
	Configured     bool   `json:"configured"`
	Repo           string `json:"repo"`
	Immutable      bool   `json:"immutable"`
	LimitUpload    int    `json:"limitUpload"`
	LimitDownload  int    `json:"limitDownload"`
	GrowthBudgetGB int    `json:"growthBudgetGb"`
	CredsRef       string `json:"credsRef"`
}

func primaryRemoteToView(t store.OffsiteTarget, configured bool) primaryRemoteView {
	return primaryRemoteView{
		Configured:     configured,
		Repo:           t.Repo,
		Immutable:      t.Immutable,
		LimitUpload:    t.LimitUpload,
		LimitDownload:  t.LimitDownload,
		GrowthBudgetGB: t.GrowthBudgetGB,
		CredsRef:       t.CredsRef,
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
// PUT /api/settings/primary-remote/{domain}  body: {immutable,limitUpload,limitDownload,growthBudgetGb,credsRef}
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
		CredsRef:       v.CredsRef,
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
