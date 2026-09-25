package api

import (
	"net/http"
	"strings"

	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// offsiteTargetView is an off-site target as the SPA sees it; store.OffsiteTarget
// has no json tags. No field is secret: CredsRef only names a credential set.
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

// toStoreTarget floors the numeric fields at zero, trims the repo and
// upper-cases the storage class. ID and CreatedAt are left to the handlers.
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

// validateOffsiteTargetInput returns a user-facing error, or "" when t is
// valid. t must come from toStoreTarget.
func validateOffsiteTargetInput(t store.OffsiteTarget) string {
	if !validOffsiteDomain(t.Domain) {
		return "invalid domain: must be one of containers, vms, flash, config, files"
	}
	if t.Repo == "" {
		return "repo must not be empty"
	}
	if t.StorageClass != "" && !restic.StorageClassAllowed(t.StorageClass) {
		return "unsupported storage class " + t.StorageClass + " (allowed: " + strings.Join(restic.AllowedStorageClasses, ", ") + ")"
	}
	return ""
}

// rejectOffsiteTargetOnNamedRepo refuses an off-site target on a location that
// is already a named repository, the counterpart of validateNamedRepo. Items
// backed up there would write their only copy where replication keeps the
// second: the copy moves nothing, the run reports success, and off-site
// retention then ages the only copy. It returns a user-facing sentence, or ""
// when the location is free, and refuses when the store or the target's
// location cannot be read.
//
// It needs the store, so it stays out of validateOffsiteTargetInput, which the
// settings import runs on rows it has not resolved yet.
func (h *Handler) rejectOffsiteTargetOnNamedRepo(t store.OffsiteTarget) string {
	loc, err := h.svc.resolveRepo(t.Repo)
	if err != nil {
		return scrubError(err)
	}
	rows, lErr := h.store.ListNamedRepos()
	if lErr != nil {
		return "could not check this location against the named repositories: " + scrubError(lErr)
	}
	for _, r := range rows {
		other, oErr := h.svc.resolveRepo(r.Repo)
		if oErr != nil || !sameRepoLocation(other, loc) {
			continue
		}
		// The row's name is left out: it is free text, and scrubError turns a
		// name with a slash into "[path]".
		return "that location is already a named repository; backups written there would be their own off-site copy, and the off-site retention would then age the only copy"
	}
	return ""
}

// handleListOffsiteTargets lists all off-site targets, or those of one domain
// when ?domain is set.
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

// handleCreateOffsiteTarget adds an additional off-site target. An id or
// createdAt in the body is ignored, and without a sortOrder the target goes
// after the domain's existing ones.
func (h *Handler) handleCreateOffsiteTarget(w http.ResponseWriter, r *http.Request) {
	// The pointer shadows the view's sortOrder so a missing one can be told
	// apart from 0.
	var v struct {
		offsiteTargetView
		SortOrder *int `json:"sortOrder"`
	}
	if !decodeBody(w, r, &v) {
		return
	}
	t := v.toStoreTarget()
	if msg := validateOffsiteTargetInput(t); msg != "" {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": msg})
		return
	}
	// Sort order 0 is the primary. A row created there would be deleted or
	// rewritten by the next settings save.
	if v.SortOrder != nil && *v.SortOrder < 1 {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": "sortOrder must be 1 or higher: 0 is the primary target, and the off-site setting in Settings manages it"})
		return
	}
	if msg := h.rejectOffsiteTargetOnNamedRepo(t); msg != "" {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": msg})
		return
	}
	if v.SortOrder != nil {
		t.SortOrder = *v.SortOrder
	} else {
		next, err := h.svc.nextOffsiteSortOrder(t.Domain)
		if err != nil {
			writeJSON(w, http.StatusOK, failEnvelope(err))
			return
		}
		t.SortOrder = next
	}
	stored, err := h.store.UpsertOffsiteTarget(t)
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"target": offsiteTargetToView(stored)}))
}

// handleUpdateOffsiteTarget replaces the target named in the path, keeping its
// id and creation time.
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
	// Checked only when the request moves the target, as in
	// rejectSettingsPathOnNamedRepo, so a row the settings import left on a
	// colliding location can still be edited rather than only deleted.
	if !sameRepoLocation(strings.TrimSpace(t.Repo), strings.TrimSpace(existing.Repo)) {
		if msg := h.rejectOffsiteTargetOnNamedRepo(t); msg != "" {
			writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": msg})
			return
		}
	}
	t.ID = existing.ID
	t.CreatedAt = existing.CreatedAt
	stored, err := h.store.UpsertOffsiteTarget(t)
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"target": offsiteTargetToView(stored)}))
}

// handleDeleteOffsiteTarget removes an off-site target. An unknown id still
// answers ok.
func (h *Handler) handleDeleteOffsiteTarget(w http.ResponseWriter, r *http.Request) {
	if err := h.store.DeleteOffsiteTarget(r.PathValue("id")); err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(nil))
}

// handleTestOffsiteTarget reports whether one off-site target is reachable and
// initialised, in the same shape as handleTestOffsite, which only probes a
// domain's primary target.
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
