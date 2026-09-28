package api

import (
	"context"
	"net/http"
	"strings"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

// pullSourceView is a pull source as the SPA sees it. The stored restic
// password never goes back out.
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
	MemberID        string `json:"memberId"`
	NeedsPairing    bool   `json:"needsPairing"`
}

// pullSourceInput is the create/update request body. MemberID names the source
// instance in the pairing group; on update an empty MemberID keeps the stored
// password.
// Enabled is a pointer so its absence is distinguishable from an explicit false.
type pullSourceInput struct {
	Name          string `json:"name"`
	Repo          string `json:"repo"`
	MemberID      string `json:"memberId"`
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
		MemberID:        ps.MemberID,
		NeedsPairing:    ps.NeedsPairing(),
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
var pullDomains = map[string]bool{"containers": true, "vms": true, "files": true, "flash": true, "config": true, "zfs": true}

// buildPullSource validates in and folds it onto existing (the zero value on
// create), returning the row to persist or a user-facing message.
func (h *Handler) buildPullSource(ctx context.Context, in pullSourceInput, existing store.PullSource, isCreate bool) (store.PullSource, string) {
	repo := strings.TrimSpace(in.Repo)
	if repo == "" {
		return store.PullSource{}, "the repository location must not be empty"
	}
	if isRcloneLocation(repo) {
		return store.PullSource{}, "an rclone: location cannot be a pull source, because rclone would reach it with THIS instance's remotes. Use the repository's own address (rest:, s3:, sftp: or b2:) and its own credentials"
	}
	domain := strings.TrimSpace(in.Domain)
	if !pullDomains[domain] {
		return store.PullSource{}, "choose which kind of backup this source holds (containers, vms, files, zfs, flash or config)"
	}

	ps := existing
	ps.Name = strings.TrimSpace(in.Name)
	ps.Repo = repo
	ps.Domain = domain
	ps.CredsRef = strings.TrimSpace(in.CredsRef)
	// Left blank, reuse whatever credential set already reaches this host
	// instead of making the admin pick or retype a login BombVault already
	// holds, typically the one a mesh "Offer storage" setup created.
	if ps.CredsRef == "" {
		ps.CredsRef = h.svc.pullCredsRefForHost(repoHost(repo))
	}

	// Naming a member pairs the source with it: the member's restic password
	// is fetched over the group now. An edit that names none keeps the
	// password already stored.
	switch memberID := strings.TrimSpace(in.MemberID); {
	case memberID == "" && isCreate:
		return store.PullSource{}, "choose the instance whose backups this box pulls"
	case memberID != "":
		enc, err := h.svc.pairedPassword(ctx, memberID)
		if err != nil {
			return store.PullSource{}, scrubError(err)
		}
		ps.MemberID, ps.ResticPasswordEnc = memberID, enc
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
// location is rejected on the form instead of failing every night.
func (h *Handler) handleCreatePullSource(w http.ResponseWriter, r *http.Request) {
	var in pullSourceInput
	if !decodeBody(w, r, &in) {
		return
	}
	ps, msg := h.buildPullSource(r.Context(), in, store.PullSource{}, true)
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
	ps, msg := h.buildPullSource(r.Context(), in, existing, false)
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
