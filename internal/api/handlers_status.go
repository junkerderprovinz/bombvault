package api

import (
	"net/http"
	"strconv"

	"github.com/junkerderprovinz/bombvault/internal/releasenotes"
	"github.com/junkerderprovinz/bombvault/internal/schedule"
	"github.com/junkerderprovinz/bombvault/internal/spike"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// handleReleaseNotes serves the embedded release notes for the "What's new"
// dialog, since the app's CSP (connect-src 'self') blocks api.github.com.
// GET /api/release-notes?version=vX.Y.Z, defaulting to the running build.
// ok is false when no notes are bundled, so the dialog shows its GitHub link.
func (h *Handler) handleReleaseNotes(w http.ResponseWriter, r *http.Request) {
	version := r.URL.Query().Get("version")
	if version == "" {
		version = Version
	}
	tag := releasenotes.Tag(version)
	htmlURL := "https://github.com/junkerderprovinz/bombvault/releases"
	if tag != "" {
		htmlURL += "/tag/" + tag
	}
	body, ok := releasenotes.Notes(version)
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":      ok,
		"version": tag,
		"body":    body,
		"htmlUrl": htmlURL,
	})
}

// runSpikeAndCache executes the host-integration probes and stores the result
// so the dashboard can render it instantly. The probes are read-only.
func (h *Handler) runSpikeAndCache() (any, bool) {
	deps := spike.Deps{
		Docker:        h.docker,
		ContainerPath: h.svc.ContainerPath(),
		LibvirtTest:   h.svc.LibvirtReachable,
	}
	checks, allOK := spike.Run(deps, h.probes)
	h.spikeMu.Lock()
	h.spikeChecks, h.spikeAllOK, h.spikeRan = checks, allOK, true
	h.spikeMu.Unlock()
	return checks, allOK
}

// WarmSpike runs the host-integration check once at startup so the cached result
// is ready when the dashboard loads.
func (h *Handler) WarmSpike() { _, _ = h.runSpikeAndCache() }

// handleSpikeFresh re-runs the probes for the dashboard's "Host Integration
// Check" button and refreshes the cache. POST /api/spike
func (h *Handler) handleSpikeFresh(w http.ResponseWriter, _ *http.Request) {
	checks, allOK := h.runSpikeAndCache()
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":     true,
		"allOk":  allOK,
		"checks": checks,
	})
}

// handleSpikeCached (GET /api/spike) returns the cached result for an instant
// view, running the probes once if they have never run (cold start).
func (h *Handler) handleSpikeCached(w http.ResponseWriter, _ *http.Request) {
	h.spikeMu.RLock()
	ran, checks, allOK := h.spikeRan, h.spikeChecks, h.spikeAllOK
	h.spikeMu.RUnlock()
	if !ran {
		checks, allOK = h.runSpikeAndCache()
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":     true,
		"allOk":  allOK,
		"checks": checks,
	})
}

// runView adds the target's name and domain to a stored Run, so the run history
// shows which container, VM or flash backup a run was for.
type runView struct {
	store.Run
	Target string `json:"target"`
	Domain string `json:"domain"` // "container" | "vm" | "flash" | "config" | "files" | "everything" | ""
}

// runTargetMaps resolves target_id → (human name, domain) across every domain,
// for enriching stored runs (handleRuns + the widget feed). Best-effort: an
// unknown id (e.g. a deleted target) simply stays absent, so lookups yield "".
func (h *Handler) runTargetMaps() (name, domain map[string]string) {
	name = map[string]string{
		store.FlashTargetID:      "Unraid flash",
		store.ConfigTargetID:     "App configuration",
		store.EverythingTargetID: "Backup Everything",
	}
	domain = map[string]string{
		store.FlashTargetID:      "flash",
		store.ConfigTargetID:     "config",
		store.EverythingTargetID: "everything",
	}
	if cts, lErr := h.store.ListTargets(); lErr == nil {
		for _, t := range cts {
			name[t.ID] = t.ContainerName
			domain[t.ID] = "container"
		}
	}
	if vts, lErr := h.store.ListVMTargets(); lErr == nil {
		for _, t := range vts {
			name[t.ID] = t.Name
			domain[t.ID] = "vm"
		}
	}
	if fss, lErr := h.store.ListFileSets(); lErr == nil {
		for _, fs := range fss {
			name[fs.ID] = fs.Name
			domain[fs.ID] = "files"
		}
	}
	return name, domain
}

func (h *Handler) handleRuns(w http.ResponseWriter, _ *http.Request) {
	// Return a generous window so the dashboard's day-filter can show several
	// days of history, not just the latest handful.
	runs, err := h.store.ListRuns(500)
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	name, domain := h.runTargetMaps()
	views := make([]runView, 0, len(runs))
	for _, r := range runs {
		views = append(views, runView{Run: r, Target: name[r.TargetID], Domain: domain[r.TargetID]})
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "runs": views})
}

// handleAckRuns marks failed runs as acknowledged so the dashboard's error panel
// can dismiss them from the failure count. POST /api/runs/ack with body
// {"ids": []string (optional), "all": bool (optional)}: when `all` is set every
// unacknowledged failed run is acknowledged; otherwise the given run ids (capped
// at 5000, each a 32-hex opaque run id) are acknowledged. Responds {ok, count}.
func (h *Handler) handleAckRuns(w http.ResponseWriter, r *http.Request) {
	var body struct {
		IDs []string `json:"ids"`
		All bool     `json:"all"`
	}
	if !decodeBody(w, r, &body) {
		return
	}
	if body.All {
		n, err := h.store.AcknowledgeAllFailed()
		if err != nil {
			writeJSON(w, http.StatusOK, failEnvelope(err))
			return
		}
		writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"count": n}))
		return
	}
	if len(body.IDs) > 5000 {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "too many ids"})
		return
	}
	for _, id := range body.IDs {
		if !validRunID(id) {
			writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "invalid run id"})
			return
		}
	}
	n, err := h.store.AcknowledgeRuns(body.IDs)
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"count": n}))
}

// handleStatus returns the per-domain RPO (protection) status for the dashboard's
// "are my backups current?" indicator. GET /api/status
func (h *Handler) handleStatus(w http.ResponseWriter, _ *http.Request) {
	settings, err := h.store.GetSettings()
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	domains, err := h.svc.domainStatusFrom(settings)
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	if domains == nil {
		domains = []DomainStatusEntry{}
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"domains": domains}))
}

// handleScheduleNext returns the next fire time of every registered schedule
// entry, soonest first, for the activity log's "up next" line.
// GET /api/schedule/next. Tests build a Handler without a scheduler, which
// yields an empty list.
func (h *Handler) handleScheduleNext(w http.ResponseWriter, _ *http.Request) {
	var runs []schedule.NextRun
	if h.scheduler != nil {
		runs = h.scheduler.NextRuns()
	}
	if runs == nil {
		runs = []schedule.NextRun{}
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"runs": runs}))
}

// handleHistory returns per-day backup outcomes for the dashboard's
// backup-health heatmap. GET /api/history?days=90; days defaults to 90 and is
// clamped to 1..366.
func (h *Handler) handleHistory(w http.ResponseWriter, r *http.Request) {
	days := 90
	if q := r.URL.Query().Get("days"); q != "" {
		if n, err := strconv.Atoi(q); err == nil {
			days = n
		}
	}
	if days < 1 {
		days = 1
	}
	if days > 366 {
		days = 366
	}
	hist, err := h.svc.BackupHistory(days)
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	if hist == nil {
		hist = []HistoryDay{}
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"days": hist}))
}

// handleStats returns a domain's recorded repository-size samples for the
// size and dedup trend. GET /api/stats?domain=&source=&limit=, where domain is
// containers, vms, flash or files, source is local (default) or offsite, and
// limit defaults to 90, clamped to 1..365. The answer carries the samples in
// ascending order, the latest one (or null) for the headline figure, and a
// "forecast" of growth, free space and time to full (see StorageForecast), or
// null when nothing could be determined.
func (h *Handler) handleStats(w http.ResponseWriter, r *http.Request) {
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

	stats, err := h.svc.RepoStats(domain, source, limit)
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	if stats == nil {
		stats = []store.RepoStat{}
	}
	var latest any // null when there are no samples yet
	if len(stats) > 0 {
		latest = stats[len(stats)-1]
	} else {
		// Without a sample, a detached and throttled collection fills the Storage
		// card in on the next load instead of leaving it on "no data".
		h.svc.CollectStatsAsync(domain, source)
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{
		"stats":    stats,
		"latest":   latest,
		"forecast": h.svc.StorageForecast(domain, source, stats),
	}))
}
