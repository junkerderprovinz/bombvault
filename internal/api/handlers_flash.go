package api

import (
	"net/http"

	"github.com/junkerderprovinz/bombvault/internal/restic"
)

// handleBackupFlash starts the Unraid USB flash backup on the server and
// returns immediately, like handleBackup. The SPA follows the "flash" progress
// key over SSE.
func (h *Handler) handleBackupFlash(w http.ResponseWriter, r *http.Request) {
	started, err := h.svc.StartBackupFlash(r.Context())
	if err != nil { // the flash domain is busy with another op → 409 with the reason
		writeJSON(w, http.StatusConflict, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	if !started {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": "a backup is already running"})
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"started": true}))
}

// handleSnapshotsFlash lists flash snapshots.
func (h *Handler) handleSnapshotsFlash(w http.ResponseWriter, r *http.Request) {
	snaps, err := h.svc.SnapshotsFlash(r.Context(), sourceParam(r))
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	if snaps == nil {
		snaps = []restic.Snapshot{}
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"snapshots": snaps}))
}

// headerOnFirstWrite defers the download headers (and so the 200 status) until
// the first byte is streamed, so a restic failure before any output (bad id,
// repo locked, no backups) is reported as a JSON error instead of a truncated
// 200 zip.
type headerOnFirstWrite struct {
	w      http.ResponseWriter
	header func()
	wrote  bool
}

func (h *headerOnFirstWrite) Write(p []byte) (int, error) {
	if !h.wrote {
		h.wrote = true
		h.header()
	}
	return h.w.Write(p)
}

// handleDownloadFlash streams a flash snapshot to the browser as a zip download
// (restic dump), as a GET so it can be a plain link. ?snapshot=<id> selects the
// snapshot; "" or "latest" is the newest.
func (h *Handler) handleDownloadFlash(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("snapshot")
	// When export encryption is on, DownloadFlashZip age-seals the stream, so the
	// attachment is <name>.zip.age and the browser must save it under that name.
	encrypted := h.svc.ExportEncryptionOn()
	var resolved string
	lw := &headerOnFirstWrite{w: w, header: func() {
		name := FlashDownloadName(resolved)
		if encrypted {
			w.Header().Set("Content-Type", "application/octet-stream")
			name += ".age"
		} else {
			w.Header().Set("Content-Type", "application/zip")
		}
		w.Header().Set("Content-Disposition", `attachment; filename="`+name+`"`)
	}}
	err := h.svc.DownloadFlashZip(r.Context(), id, sourceParam(r), func(rid string) { resolved = rid }, lw)
	// With nothing streamed the headers are unsent and the failure can go out
	// as JSON. A failure mid-stream can only truncate the body; its run is
	// recorded.
	if err != nil && !lw.wrote {
		restoreFail(w, sourceParam(r), err)
	}
}
