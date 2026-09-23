package api

import (
	"net/http"
	"strings"

	"github.com/junkerderprovinz/bombvault/internal/secret"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// pullSourceView is a pull source as the SPA sees it. The stored APP_KEY never
// goes back out, so the view only says whether one is set.
type pullSourceView struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	Repo            string `json:"repo"`
	CredsRef        string `json:"credsRef"`
	Domain          string `json:"domain"`
	Cadence         string `json:"cadence"`
	LimitDownload   int    `json:"limitDownload"`
	LimitUpload     int    `json:"limitUpload"`
	LastPullAt      int64  `json:"lastPullAt"`
	LastPullOK      *bool  `json:"lastPullOk"` // null = never pulled
	LastPullError   string `json:"lastPullError"`
	SnapshotsPulled int    `json:"snapshotsPulled"`
	Enabled         bool   `json:"enabled"`
	CreatedAt       int64  `json:"createdAt"`
	SortOrder       int    `json:"sortOrder"`
	HasAppKey       bool   `json:"hasAppKey"`
}

// pullSourceInput is the create/update request body. AppKey is the source
// instance's 64-hex APP_KEY; on update an empty AppKey keeps the stored one.
// Enabled is a pointer so its absence is distinguishable from an explicit false.
type pullSourceInput struct {
	Name          string `json:"name"`
	Repo          string `json:"repo"`
	AppKey        string `json:"appKey"`
	CredsRef      string `json:"credsRef"`
	Domain        string `json:"domain"`
	Cadence       string `json:"cadence"`
	LimitDownload int    `json:"limitDownload"`
	LimitUpload   int    `json:"limitUpload"`
	Enabled       *bool  `json:"enabled"`
	SortOrder     *int   `json:"sortOrder"`
}

func pullSourceToView(ps store.PullSource) pullSourceView {
	v := pullSourceView{
		ID:              ps.ID,
		Name:            ps.Name,
		Repo:            ps.Repo,
		CredsRef:        ps.CredsRef,
		Domain:          ps.Domain,
		Cadence:         ps.Cadence,
		LimitDownload:   ps.LimitDownload,
		LimitUpload:     ps.LimitUpload,
		LastPullAt:      ps.LastPullAt,
		LastPullError:   ps.LastPullError,
		SnapshotsPulled: ps.SnapshotsPulled,
		Enabled:         ps.Enabled,
		CreatedAt:       ps.CreatedAt,
		SortOrder:       ps.SortOrder,
		HasAppKey:       len(ps.AppKeyEnc) > 0,
	}
	if ps.LastPullOK.Valid {
		ok := ps.LastPullOK.Bool
		v.LastPullOK = &ok
	}
	return v
}

// pullDomains is where a pulled snapshot may land. It is the same set the
// backup side knows, and a source must name exactly one: a sender replicates per
// domain into separate repositories, so a source repository holds one domain.
var pullDomains = map[string]bool{"containers": true, "vms": true, "files": true, "flash": true, "config": true}

// buildPullSource validates in and folds it onto existing (the zero value on
// create), returning the row to persist or a user-facing message.
func (h *Handler) buildPullSource(in pullSourceInput, existing store.PullSource, isCreate bool) (store.PullSource, string) {
	repo := strings.TrimSpace(in.Repo)
	if repo == "" {
		return store.PullSource{}, "the repository location must not be empty"
	}
	if isRcloneLocation(repo) {
		return store.PullSource{}, "an rclone: location cannot be a pull source, because rclone would reach it with THIS instance's remotes. Use the repository's own address (rest:, s3:, sftp: or b2:) and its own credentials"
	}
	domain := strings.TrimSpace(in.Domain)
	if !pullDomains[domain] {
		return store.PullSource{}, "choose which kind of backup this source holds (containers, vms, files, flash or config)"
	}

	ps := existing
	ps.Name = strings.TrimSpace(in.Name)
	ps.Repo = repo
	ps.Domain = domain
	ps.CredsRef = strings.TrimSpace(in.CredsRef)

	key := strings.TrimSpace(in.AppKey)
	switch {
	case key == "" && isCreate:
		return store.PullSource{}, "the source instance's APP_KEY is required (64 lowercase hex characters)"
	case key == "":
		// An edit without a key keeps the stored one, so a rename does not
		// clear it.
		ps.AppKeyEnc = existing.AppKeyEnc
	default:
		if !foreignKeyRe.MatchString(key) {
			return store.PullSource{}, "the APP_KEY must be exactly 64 lowercase hex characters"
		}
		enc, err := secret.Encrypt(h.cfg.AppKey, []byte(key))
		if err != nil {
			return store.PullSource{}, "could not encrypt the APP_KEY"
		}
		ps.AppKeyEnc = enc
	}

	cadence, msg := validateReceiverCadence(in.Cadence)
	if msg != "" {
		return store.PullSource{}, msg
	}
	ps.Cadence = cadence
	ps.LimitDownload = max(0, in.LimitDownload)
	ps.LimitUpload = max(0, in.LimitUpload)
	if in.Enabled != nil {
		ps.Enabled = *in.Enabled
	} else if isCreate {
		ps.Enabled = true
	}
	if in.SortOrder != nil {
		ps.SortOrder = *in.SortOrder
	}
	return ps, ""
}

// handleListPullSources returns every configured source. GET /api/pull/sources.
func (h *Handler) handleListPullSources(w http.ResponseWriter, _ *http.Request) {
	sources, err := h.store.ListPullSources()
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	out := make([]pullSourceView, 0, len(sources))
	for _, ps := range sources {
		out = append(out, pullSourceToView(ps))
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"sources": out}))
}

// handleCreatePullSource registers a source. POST /api/pull/sources.
// The source has to open read-only before the row is saved, so a mistyped
// location or key is rejected on the form instead of failing every night.
func (h *Handler) handleCreatePullSource(w http.ResponseWriter, r *http.Request) {
	var in pullSourceInput
	if !decodeBody(w, r, &in) {
		return
	}
	ps, msg := h.buildPullSource(in, store.PullSource{}, true)
	if msg != "" {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": msg})
		return
	}
	if err := h.svc.pullProbe(r.Context(), ps); err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	stored, err := h.store.CreatePullSource(ps)
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"source": pullSourceToView(stored)}))
}

// handleUpdatePullSource edits a source in place. PUT /api/pull/sources/{id}.
func (h *Handler) handleUpdatePullSource(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	existing, ok, err := h.store.GetPullSource(id)
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	if !ok {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": "no such pull source"})
		return
	}
	var in pullSourceInput
	if !decodeBody(w, r, &in) {
		return
	}
	ps, msg := h.buildPullSource(in, existing, false)
	if msg != "" {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": msg})
		return
	}
	if err := h.svc.pullProbe(r.Context(), ps); err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	if err := h.store.UpdatePullSource(ps); err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"source": pullSourceToView(ps)}))
}

// handleDeletePullSource removes a source. DELETE /api/pull/sources/{id}.
// It never touches either repository: the source is somebody else's, and what
// was already pulled belongs to this box and stays.
func (h *Handler) handleDeletePullSource(w http.ResponseWriter, r *http.Request) {
	if err := h.store.DeletePullSource(r.PathValue("id")); err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(nil))
}

// handleTestPullSource probes a source without pulling anything.
// POST /api/pull/sources/{id}/test.
func (h *Handler) handleTestPullSource(w http.ResponseWriter, r *http.Request) {
	ps, ok, err := h.store.GetPullSource(r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	if !ok {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": "no such pull source"})
		return
	}
	if err := h.svc.pullProbe(r.Context(), ps); err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(nil))
}

// handleRunPullSource pulls one source now. POST /api/pull/sources/{id}/run.
func (h *Handler) handleRunPullSource(w http.ResponseWriter, r *http.Request) {
	ps, ok, err := h.store.GetPullSource(r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	if !ok {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": "no such pull source"})
		return
	}
	n, err := h.svc.PullFromSource(r.Context(), ps)
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"snapshots": n}))
}
