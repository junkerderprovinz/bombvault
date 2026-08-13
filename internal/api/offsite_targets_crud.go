package api

import (
	"net/http"
	"strings"

	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// ---------------------------------------------------------------------------
// CRUD API for additional off-site targets
// ---------------------------------------------------------------------------

// offsiteTargetView is the JSON wire shape of an off-site DESTINATION. store.
// OffsiteTarget carries no json tags (it is a storage type), so this view owns
// the field names the SPA sees. No field is a secret: creds_ref selects a
// credential set (it is not a credential), and storage_class is a class name.
type offsiteTargetView struct {
	ID                   string `json:"id"`
	Domain               string `json:"domain"`
	Name                 string `json:"name"`
	Repo                 string `json:"repo"`
	CredsRef             string `json:"credsRef"`
	StorageClass         string `json:"storageClass"`
	Immutable            bool   `json:"immutable"`
	Schedule             string `json:"schedule"`
	RetentionKeepLast    int    `json:"retentionKeepLast"`
	RetentionKeepDaily   int    `json:"retentionKeepDaily"`
	RetentionKeepWeekly  int    `json:"retentionKeepWeekly"`
	RetentionKeepMonthly int    `json:"retentionKeepMonthly"`
	LimitUpload          int    `json:"limitUpload"`
	LimitDownload        int    `json:"limitDownload"`
	GrowthBudgetGB       int    `json:"growthBudgetGb"`
	Enabled              bool   `json:"enabled"`
	CreatedAt            int64  `json:"createdAt"`
	SortOrder            int    `json:"sortOrder"`
}

func offsiteTargetToView(t store.OffsiteTarget) offsiteTargetView {
	return offsiteTargetView{
		ID:                   t.ID,
		Domain:               t.Domain,
		Name:                 t.Name,
		Repo:                 t.Repo,
		CredsRef:             t.CredsRef,
		StorageClass:         t.StorageClass,
		Immutable:            t.Immutable,
		Schedule:             t.Schedule,
		RetentionKeepLast:    t.RetentionKeepLast,
		RetentionKeepDaily:   t.RetentionKeepDaily,
		RetentionKeepWeekly:  t.RetentionKeepWeekly,
		RetentionKeepMonthly: t.RetentionKeepMonthly,
		LimitUpload:          t.LimitUpload,
		LimitDownload:        t.LimitDownload,
		GrowthBudgetGB:       t.GrowthBudgetGB,
		Enabled:              t.Enabled,
		CreatedAt:            t.CreatedAt,
		SortOrder:            t.SortOrder,
	}
}

func offsiteTargetsToViews(ts []store.OffsiteTarget) []offsiteTargetView {
	out := make([]offsiteTargetView, 0, len(ts))
	for _, t := range ts {
		out = append(out, offsiteTargetToView(t))
	}
	return out
}

// toStoreTarget maps the view to the storage type. Numeric fields are floored at
// zero (a negative retention/limit/budget is meaningless), the storage class is
// normalized (trim+upper), and the id/created_at are NOT taken from the body —
// the handlers own those (create mints them, update preserves the existing row's).
func (v offsiteTargetView) toStoreTarget() store.OffsiteTarget {
	return store.OffsiteTarget{
		Domain:               v.Domain,
		Name:                 v.Name,
		Repo:                 strings.TrimSpace(v.Repo),
		CredsRef:             v.CredsRef,
		StorageClass:         strings.ToUpper(strings.TrimSpace(v.StorageClass)),
		Immutable:            v.Immutable,
		Schedule:             v.Schedule,
		RetentionKeepLast:    max(0, v.RetentionKeepLast),
		RetentionKeepDaily:   max(0, v.RetentionKeepDaily),
		RetentionKeepWeekly:  max(0, v.RetentionKeepWeekly),
		RetentionKeepMonthly: max(0, v.RetentionKeepMonthly),
		LimitUpload:          max(0, v.LimitUpload),
		LimitDownload:        max(0, v.LimitDownload),
		GrowthBudgetGB:       max(0, v.GrowthBudgetGB),
		Enabled:              v.Enabled,
		SortOrder:            v.SortOrder,
	}
}

// validateOffsiteTargetInput enforces the create/update contract: a valid domain,
// a non-empty repo, and — when set — a restore-readable storage class. Returns a
// user-facing error string, or "" when valid. Assumes t was built via
// toStoreTarget (repo trimmed, class trim+uppercased).
func validateOffsiteTargetInput(t store.OffsiteTarget) string {
	if !validOffsiteDomain(t.Domain) {
		return "invalid domain — must be one of containers, vms, flash, config, files"
	}
	if t.Repo == "" {
		return "repo must not be empty"
	}
	if t.StorageClass != "" && !restic.StorageClassAllowed(t.StorageClass) {
		return "unsupported storage class " + t.StorageClass + " (allowed: " + strings.Join(restic.AllowedStorageClasses, ", ") + ")"
	}
	return ""
}

// handleListOffsiteTargets lists off-site targets. GET /api/offsite/targets
// (all, in stable per-domain order) or GET /api/offsite/targets?domain=<d> (one
// domain). An unknown ?domain is rejected; an empty result is a valid [] list.
func (h *Handler) handleListOffsiteTargets(w http.ResponseWriter, r *http.Request) {
	domain := strings.TrimSpace(r.URL.Query().Get("domain"))
	var (
		targets []store.OffsiteTarget
		err     error
	)
	if domain != "" {
		if !validOffsiteDomain(domain) {
			writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "unknown domain"})
			return
		}
		targets, err = h.store.OffsiteTargetsForDomain(domain)
	} else {
		targets, err = h.store.ListOffsiteTargets()
	}
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"targets": offsiteTargetsToViews(targets)}))
}

// handleCreateOffsiteTarget creates an additional off-site target.
// POST /api/offsite/targets — body is an offsiteTargetView (id/createdAt ignored).
func (h *Handler) handleCreateOffsiteTarget(w http.ResponseWriter, r *http.Request) {
	var v offsiteTargetView
	if !decodeBody(w, r, &v) {
		return
	}
	t := v.toStoreTarget()
	if msg := validateOffsiteTargetInput(t); msg != "" {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": msg})
		return
	}
	t.ID = "" // force a fresh id even if the client sent one
	stored, err := h.store.UpsertOffsiteTarget(t)
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"target": offsiteTargetToView(stored)}))
}

// handleUpdateOffsiteTarget updates an existing off-site target in place.
// PUT /api/offsite/targets/{id}. The id comes from the path; created_at is
// preserved from the stored row so an update never re-stamps it.
func (h *Handler) handleUpdateOffsiteTarget(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	existing, ok, err := h.store.GetOffsiteTarget(id)
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]any{"ok": false, "error": "no such off-site target"})
		return
	}
	var v offsiteTargetView
	if !decodeBody(w, r, &v) {
		return
	}
	t := v.toStoreTarget()
	if msg := validateOffsiteTargetInput(t); msg != "" {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": msg})
		return
	}
	t.ID = existing.ID
	t.CreatedAt = existing.CreatedAt // preserve; UpsertOffsiteTarget would otherwise keep it via ON CONFLICT, but be explicit
	stored, err := h.store.UpsertOffsiteTarget(t)
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"target": offsiteTargetToView(stored)}))
}

// handleDeleteOffsiteTarget removes an off-site target by id (a no-op, still ok,
// when it does not exist). DELETE /api/offsite/targets/{id}.
func (h *Handler) handleDeleteOffsiteTarget(w http.ResponseWriter, r *http.Request) {
	if err := h.store.DeleteOffsiteTarget(r.PathValue("id")); err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(nil))
}

// handleTestOffsiteTarget probes ONE off-site target (reachable / initialised)
// without modifying it. Same response shape as handleTestOffsite, which only
// ever probes a domain's PRIMARY target — so an additional target could sit
// broken behind that button's green verdict (issue #138).
// POST /api/offsite/targets/{id}/test
func (h *Handler) handleTestOffsiteTarget(w http.ResponseWriter, r *http.Request) {
	reachable, initialized, err := h.svc.TestOffsiteTarget(r.Context(), r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{
		"reachable":   reachable,
		"initialized": initialized,
	}))
}
