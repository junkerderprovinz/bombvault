package api

import (
	"net/http"

	"github.com/junkerderprovinz/bombvault/internal/restic"
)

// handleBackupConfig starts the self-backup of BombVault's own /config
// (settings database, rclone.conf, SSH key pair) on the server and returns
// immediately, like handleBackupFlash. The SPA follows the "config" progress
// key over SSE.
func (h *Handler) handleBackupConfig(w http.ResponseWriter, r *http.Request) {
	started, err := h.svc.StartBackupConfig(r.Context())
	if err != nil { // the config domain is busy with another op → 409 with the reason
		writeJSON(w, http.StatusConflict, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	if !started {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": "a backup is already running"})
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"started": true}))
}

// handleSnapshotsConfig lists config snapshots (BombVault's own /config backups).
func (h *Handler) handleSnapshotsConfig(w http.ResponseWriter, r *http.Request) {
	snaps, err := h.svc.SnapshotsConfig(r.Context(), sourceParam(r))
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	if snaps == nil {
		snaps = []restic.Snapshot{}
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"snapshots": snaps}))
}

// handleRestoreConfig stages a restore of BombVault's own /config and triggers
// a restart, because the live SQLite database cannot be swapped while this
// process holds it open; selfrestore.ApplyPending swaps it in at boot. When
// Docker is unreachable, autoRestart:false tells the SPA to ask for a manual
// restart. Errors go through restoreFail like the other restores.
func (h *Handler) handleRestoreConfig(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Source   string `json:"source"`
		Snapshot string `json:"snapshot"`
	}
	if !decodeBody(w, r, &body) {
		return
	}
	// The source comes in the body here, not as ?source=.
	source := normalizeSource(body.Source)
	started, auto, err := h.svc.StartRestoreConfig(r.Context(), body.Snapshot, source)
	if err != nil {
		restoreFail(w, source, err)
		return
	}
	if !started {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": "a backup or restore is already running"})
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"staged": true, "autoRestart": auto}))
}
