package api

import (
	"net/http"
	"strconv"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

// handleCheck verifies the integrity of a domain's restic repo (restic check).
// POST /api/check/{domain}  domain ∈ {containers, vms, flash, files}
func (h *Handler) handleCheck(w http.ResponseWriter, r *http.Request) {
	domain := r.PathValue("domain")
	switch domain {
	case "containers", "vms", "flash", "files":
	default:
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "unknown domain"})
		return
	}
	if err := h.svc.CheckDomain(r.Context(), domain, sourceParam(r)); err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(nil))
}

// handleRunDrill runs a restore-verification drill for a domain and returns the
// recorded result. ?kind=subset (default) is the classic `restic check
// --read-data-subset` integrity check; ?kind=dr is a real off-site sandbox restore
// (containers, flash + files only). POST /api/verify/{domain}?source=&kind=
// domain ∈ {containers,vms,flash,files}
func (h *Handler) handleRunDrill(w http.ResponseWriter, r *http.Request) {
	domain := r.PathValue("domain")
	switch domain {
	case "containers", "vms", "flash", "files":
	default:
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "unknown domain"})
		return
	}
	// Manual: fail fast with immediate busy feedback (wait=false) so the UI can tell
	// the user a backup is running rather than blocking the request.
	drill, err := h.svc.RunRestoreDrill(r.Context(), domain, sourceParam(r), kindParam(r), false)
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"drill": drill}))
}

// handleDrills returns the recorded restore-verification drills for a domain +
// source (newest first), plus the latest one for the badge.
// GET /api/verify?domain=&source=&limit=
func (h *Handler) handleDrills(w http.ResponseWriter, r *http.Request) {
	domain := r.URL.Query().Get("domain")
	switch domain {
	case "containers", "vms", "flash", "files":
	default:
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "unknown domain"})
		return
	}
	source := sourceParam(r)

	limit := 90
	if q := r.URL.Query().Get("limit"); q != "" {
		if n, err := strconv.Atoi(q); err == nil {
			limit = n
		}
	}
	if limit < 1 {
		limit = 1
	}
	if limit > 365 {
		limit = 365
	}

	drills, err := h.svc.Drills(domain, source, limit)
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	if drills == nil {
		drills = []store.RestoreDrill{}
	}
	var latest any // null when there are no drills yet
	if len(drills) > 0 {
		latest = drills[0] // newest first
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"drills": drills, "latest": latest}))
}

// handleUnlock clears repository locks for a domain (restic unlock --remove-all),
// the manual recovery for a "repository is already locked" error left by a
// crashed/interrupted run. POST /api/unlock/{domain}
func (h *Handler) handleUnlock(w http.ResponseWriter, r *http.Request) {
	domain := r.PathValue("domain")
	switch domain {
	case "containers", "vms", "flash", "files":
	default:
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "unknown domain"})
		return
	}
	skipped, err := h.svc.UnlockDomain(r.Context(), domain, sourceParam(r))
	if err != nil {
		// The skip list goes along on both paths. A repository that only got the
		// stale-lock clear is a note, never the error, so the operator needs it
		// next to whatever did fail.
		body := failEnvelope(err)
		body["skipped"] = skipped
		writeJSON(w, http.StatusOK, body)
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"skipped": skipped}))
}

// handlePrune reclaims repository space freed by forgotten snapshots
// (restic prune). POST /api/prune/{domain}
func (h *Handler) handlePrune(w http.ResponseWriter, r *http.Request) {
	domain := r.PathValue("domain")
	switch domain {
	case "containers", "vms", "flash", "config", "files":
	default:
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "unknown domain"})
		return
	}
	if err := h.svc.PruneDomain(r.Context(), domain, sourceParam(r)); err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(nil))
}

// handleDeleteSnapshot forgets a single snapshot from a domain's repo.
// DELETE /api/snapshots/{domain}/{id}
func (h *Handler) handleDeleteSnapshot(w http.ResponseWriter, r *http.Request) {
	domain := r.PathValue("domain")
	switch domain {
	case "containers", "vms", "flash", "config", "files":
	default:
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "unknown domain"})
		return
	}
	if err := h.svc.DeleteSnapshot(r.Context(), domain, r.PathValue("id"), sourceParam(r)); err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(nil))
}

// handleReplicateOffsite starts an on-demand replication of a domain's local
// repo to its off-site repo (restic copy) and returns immediately, because a
// long first replication outlives a browser or proxy timeout. Config errors
// and a busy domain still report synchronously. POST /api/offsite/{domain}
func (h *Handler) handleReplicateOffsite(w http.ResponseWriter, r *http.Request) {
	domain := r.PathValue("domain")
	switch domain {
	case "containers", "vms", "flash", "config", "files":
	default:
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "unknown domain"})
		return
	}
	if err := h.svc.StartReplicateOffsite(domain); err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"started": true}))
}

// handleTestOffsite probes a domain's off-site repo (reachable / initialised)
// without modifying it, so the UI can verify the location before relying on it.
// Modelled on handleVMSSHTest. POST /api/offsite/{domain}/test
func (h *Handler) handleTestOffsite(w http.ResponseWriter, r *http.Request) {
	domain := r.PathValue("domain")
	switch domain {
	case "containers", "vms", "flash", "config", "files":
	default:
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "unknown domain"})
		return
	}
	reachable, initialized, err := h.svc.TestOffsite(r.Context(), domain)
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{
		"reachable":   reachable,
		"initialized": initialized,
	}))
}
