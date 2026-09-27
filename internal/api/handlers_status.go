package api

import (
	"errors"
	"log"
	"net/http"
	"slices"
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
		ZFSTest:       h.svc.ZFSSpikeStatus,
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
	// StartedViaLabel is the operator's name for the MCP key behind the run and
	// StartedViaRevoked says whether that key is revoked. Both stay empty for a
	// run the web interface or the scheduler started.
	StartedViaLabel   string `json:"startedViaLabel"`
	StartedViaRevoked bool   `json:"startedViaRevoked"`
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
	if ds, lErr := h.store.ListZFSDatasets(); lErr == nil {
		for _, d := range ds {
			name[d.ID] = d.Dataset
			domain[d.ID] = "zfs"
		}
	}
	return name, domain
}

func (h *Handler) handleRuns(w http.ResponseWriter, r *http.Request) {
	// Return a generous window so the dashboard's day-filter can show several
	// days of history, not just the latest handful.
	runs, err := h.store.ListRuns(500)
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	// ?run names the run a link opened the log on, which can be older than the
	// window.
	if id := r.URL.Query().Get("run"); id != "" && !slices.ContainsFunc(runs, func(run store.Run) bool { return run.ID == id }) {
		run, err := h.store.GetRun(id)
		switch {
		case err == nil:
			runs = append(runs, run)
		case !errors.Is(err, store.ErrRunNotFound):
			writeJSON(w, http.StatusOK, failEnvelope(err))
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "runs": h.runViews(runs)})
}

// runViews enriches stored runs with their target's name and domain and names
// the MCP key behind the ones an assistant started.
func (h *Handler) runViews(runs []store.Run) []runView {
	name, domain := h.runTargetMaps()
	keys := h.mcpKeysBehind(runs)
	views := make([]runView, 0, len(runs))
	for _, r := range runs {
		v := runView{Run: r, Target: name[r.TargetID], Domain: domain[r.TargetID]}
		if key, ok := keys[r.StartedViaKey]; ok {
			v.StartedViaLabel = key.Label
			v.StartedViaRevoked = key.RevokedAt > 0
		}
		views = append(views, v)
	}
	return views
}

// mcpKeysBehind indexes the MCP keys the runs name, and reads none at all when
// no run came from MCP. A revoked key is included: the audit trail has to keep
// naming the client that started a run after the key it used is gone.
func (h *Handler) mcpKeysBehind(runs []store.Run) map[string]store.MCPKey {
	if !slices.ContainsFunc(runs, func(r store.Run) bool { return r.StartedViaKey != "" }) {
		return nil
	}
	keys, err := h.store.ListMCPKeys()
	if err != nil {
		log.Printf("api: runs: reading the MCP key names failed: %v", err)
		return nil
	}
	out := make(map[string]store.MCPKey, len(keys))
	for _, key := range keys {
		out[key.ID] = key
	}
	return out
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
// containers, vms, flash, files or zfs, source is local (default) or offsite,
// and limit defaults to 90, clamped to 1..365. The answer carries the samples in
// ascending order, the latest one (or null) for the headline figure, and a
// "forecast" of growth, free space and time to full (see StorageForecast), or
// null when nothing could be determined.
func (h *Handler) handleStats(w http.ResponseWriter, r *http.Request) {
	domain := r.URL.Query().Get("domain")
	switch domain {
	case "containers", "vms", "flash", "files", "zfs":
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
		last := stats[len(stats)-1]
		if n, ok := h.svc.currentSnapshotCount(domain, source); ok {
			last.Snapshots = int64(n)
		}
		latest = last
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
