package api

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"regexp"
	"strings"

	"github.com/junkerderprovinz/bombvault/internal/backup"
	"github.com/junkerderprovinz/bombvault/internal/paths"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/schedule"
	"github.com/junkerderprovinz/bombvault/internal/spike"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// ---------------------------------------------------------------------------
// JSON helpers + error scrubbing
// ---------------------------------------------------------------------------

// writeJSON encodes v as JSON with the given status code.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("api: encode response: %v", err)
	}
}

// okEnvelope returns a success envelope merged with extra fields.
func okEnvelope(extra map[string]any) map[string]any {
	m := map[string]any{"ok": true}
	for k, v := range extra {
		m[k] = v
	}
	return m
}

// failEnvelope returns a graceful failure envelope. The error is scrubbed so no
// repo path or secret leaks to the client (defense-in-depth; the restic/docker
// adapters already scrub their own errors).
func failEnvelope(err error) map[string]any {
	return map[string]any{"ok": false, "error": scrubError(err)}
}

// absPathRe matches absolute unix paths so they can be stripped from any error
// message that slips through to the API surface.
var absPathRe = regexp.MustCompile(`(/[^\s:"']+)+`)

// scrubError maps known sentinels to clear messages and strips absolute paths
// from anything else.
func scrubError(err error) string {
	switch {
	case err == nil:
		return ""
	case errors.Is(err, backup.ErrNotConfirmed):
		return "restore not confirmed — set confirm:true to proceed"
	case errors.Is(err, backup.ErrInvalidSnapshotID):
		return "invalid snapshot id (must be 8–64 lowercase hex)"
	}
	msg := err.Error()
	msg = absPathRe.ReplaceAllString(msg, "[path]")
	return strings.TrimSpace(msg)
}

// decodeBody decodes a JSON request body into v. Returns false (and writes a
// graceful failure) on malformed JSON.
func decodeBody(w http.ResponseWriter, r *http.Request, v any) bool {
	if r.Body == nil {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": "missing request body"})
		return false
	}
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": "invalid request body"})
		return false
	}
	return true
}

// ---------------------------------------------------------------------------
// handlers
// ---------------------------------------------------------------------------

func (h *Handler) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// containerView is the per-container row returned by GET /api/containers.
type containerView struct {
	Name              string `json:"name"`
	Image             string `json:"image"`
	State             string `json:"state"`
	Status            string `json:"status"`
	IncludeInSchedule bool   `json:"includeInSchedule"`
	LastBackup        *int64 `json:"lastBackup"`
}

func (h *Handler) handleListContainers(w http.ResponseWriter, r *http.Request) {
	infos, err := h.docker.List(r.Context())
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}

	// Index targets by name for include flag + last backup.
	targets, _ := h.store.ListTargets()
	byName := make(map[string]store.Target, len(targets))
	for _, t := range targets {
		byName[t.ContainerName] = t
	}

	views := make([]containerView, 0, len(infos))
	for _, c := range infos {
		v := containerView{
			Name:   c.Name,
			Image:  c.Image,
			State:  c.State,
			Status: c.Status,
		}
		if t, ok := byName[c.Name]; ok {
			v.IncludeInSchedule = t.IncludeInSchedule
			if run, _ := h.store.LastSuccessfulBackup(t.ID); run != nil {
				v.LastBackup = run.FinishedAt
			}
		}
		views = append(views, v)
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "containers": views})
}

func (h *Handler) handleBackup(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	sum, err := h.svc.Backup(r.Context(), name)
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{
		"snapshotId": sum.SnapshotID,
		"bytes":      sum.Bytes,
	}))
}

func (h *Handler) handleSnapshots(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	snaps, err := h.svc.Snapshots(r.Context(), name)
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	if snaps == nil {
		snaps = []restic.Snapshot{}
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"snapshots": snaps}))
}

func (h *Handler) handleRestore(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	var body struct {
		SnapshotID string `json:"snapshotId"`
		Confirm    bool   `json:"confirm"`
	}
	if !decodeBody(w, r, &body) {
		return
	}
	if err := h.svc.Restore(r.Context(), name, body.SnapshotID, body.Confirm); err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(nil))
}

func (h *Handler) handlePatchContainer(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	var body struct {
		IncludeInSchedule bool `json:"includeInSchedule"`
	}
	if !decodeBody(w, r, &body) {
		return
	}
	if err := h.store.SetInclude(name, body.IncludeInSchedule); err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(nil))
}

// settingsView is the JSON shape for GET/PUT /api/settings.
type settingsView struct {
	EncryptionEnabled  bool   `json:"encryptionEnabled"`
	ContainersEnabled  bool   `json:"containersEnabled"`
	VMsEnabled         bool   `json:"vmsEnabled"`
	FlashEnabled       bool   `json:"flashEnabled"`
	ContainersPath     string `json:"containersPath"`
	VMsPath            string `json:"vmsPath"`
	FlashPath          string `json:"flashPath"`
	ContainersSchedule string `json:"containersSchedule"`
	VMsSchedule        string `json:"vmsSchedule"`
	FlashSchedule      string `json:"flashSchedule"`
	DefaultLanguage    string `json:"defaultLanguage"`
}

func toView(s store.Settings) settingsView {
	return settingsView{
		EncryptionEnabled:  s.EncryptionEnabled,
		ContainersEnabled:  s.ContainersEnabled,
		VMsEnabled:         s.VMsEnabled,
		FlashEnabled:       s.FlashEnabled,
		ContainersPath:     s.ContainersPath,
		VMsPath:            s.VMsPath,
		FlashPath:          s.FlashPath,
		ContainersSchedule: s.ContainersSchedule,
		VMsSchedule:        s.VMsSchedule,
		FlashSchedule:      s.FlashSchedule,
		DefaultLanguage:    s.DefaultLanguage,
	}
}

func (h *Handler) handleGetSettings(w http.ResponseWriter, _ *http.Request) {
	s, err := h.store.GetSettings()
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	// Nest under "settings" so the GET response is shape-symmetric with the PUT
	// body: a client can GET, edit, and PUT back the same settings object without
	// the envelope's "ok" leaking into the strict PUT decoder.
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "settings": toView(s)})
}

func (h *Handler) handlePutSettings(w http.ResponseWriter, r *http.Request) {
	var v settingsView
	if !decodeBody(w, r, &v) {
		return
	}

	// Validate each domain subpath stays under the mount root.
	for _, sub := range []string{v.ContainersPath, v.VMsPath, v.FlashPath} {
		if _, err := paths.Resolve(h.cfg.HostMountRoot, sub); err != nil {
			writeJSON(w, http.StatusOK, map[string]any{
				"ok": false, "error": "invalid backup path (must stay under the mount root): " + sub,
			})
			return
		}
	}

	// Validate each cadence parses.
	for _, cad := range []string{v.ContainersSchedule, v.VMsSchedule, v.FlashSchedule} {
		if _, _, err := schedule.ParseCadence(cad); err != nil {
			writeJSON(w, http.StatusOK, map[string]any{
				"ok": false, "error": scrubError(err),
			})
			return
		}
	}

	s := store.Settings{
		EncryptionEnabled:  v.EncryptionEnabled,
		ContainersEnabled:  v.ContainersEnabled,
		VMsEnabled:         v.VMsEnabled,
		FlashEnabled:       v.FlashEnabled,
		ContainersPath:     v.ContainersPath,
		VMsPath:            v.VMsPath,
		FlashPath:          v.FlashPath,
		ContainersSchedule: v.ContainersSchedule,
		VMsSchedule:        v.VMsSchedule,
		FlashSchedule:      v.FlashSchedule,
		DefaultLanguage:    v.DefaultLanguage,
	}
	if err := h.store.UpdateSettings(s); err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	if err := h.scheduler.Reload(s); err != nil {
		// Settings persisted but the scheduler could not re-register — report it.
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": scrubError(err)})
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(nil))
}

func (h *Handler) handleSpike(w http.ResponseWriter, _ *http.Request) {
	deps := spike.Deps{
		Docker:        h.docker,
		ContainerPath: h.svc.ContainerPath(),
	}
	checks, allOK := spike.Run(deps, h.probes)
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":     true,
		"allOk":  allOK,
		"checks": checks,
	})
}

func (h *Handler) handleRuns(w http.ResponseWriter, _ *http.Request) {
	runs, err := h.store.ListRuns(100)
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	if runs == nil {
		runs = []store.Run{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "runs": runs})
}
