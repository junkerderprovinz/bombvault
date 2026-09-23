package api

import (
	"fmt"
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
	// CompanionOf is set only on a NamedRepos row: the off-site target this
	// repository is the direct repository of, "" for a plain one. Read-only,
	// since toStoreTarget never maps it back: an import can read a companion
	// link the file already describes, never create or change one.
	CompanionOf string `json:"companionOf,omitempty"`
	// OffPremises is set only on a named repository (RoleRepo): a destination
	// carries no meaning for it, so the export leaves the field off entirely
	// rather than send a value that means nothing there.
	OffPremises *bool `json:"offPremises,omitempty"`
}

func offsiteTargetToView(t store.OffsiteTarget) offsiteTargetView {
	v := offsiteTargetView{
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
		CompanionOf:          t.CompanionOf,
	}
	if t.Role == store.RoleRepo {
		v.OffPremises = &t.OffPremises
	}
	return v
}

func offsiteTargetsToViews(ts []store.OffsiteTarget) []offsiteTargetView {
	out := make([]offsiteTargetView, 0, len(ts))
	for _, t := range ts {
		out = append(out, offsiteTargetToView(t))
	}
	return out
}

// offsiteTargetBody is a target as the Off-site tab sends it. The answer to the
// new-target question rides here rather than on offsiteTargetView, which the
// settings file uses too.
type offsiteTargetBody struct {
	offsiteTargetView
	AlsoExclude *newTargetExclusion `json:"alsoExclude"`
}

// toStoreTarget floors the numeric fields at zero, trims the repo and
// upper-cases the storage class. ID and CreatedAt are left to the handlers.
func (v offsiteTargetView) toStoreTarget() store.OffsiteTarget {
	t := store.OffsiteTarget{
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
	if v.OffPremises != nil {
		t.OffPremises = *v.OffPremises
	}
	return t
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
		if oErr != nil || !repoLocationsOverlap(other, loc) {
			continue
		}
		// The row's name is left out: it is free text, and scrubError turns a
		// name with a slash into "[path]".
		return "that location is a named repository, or lies inside or around one; backups written there would be their own off-site copy, and the off-site retention would then age the only copy"
	}
	return ""
}

// nestedTargetLocation refuses a target location that holds or lies inside a
// domain repository, an off-site field, another target or a named repository,
// and one that is a named repository's place; id is "" for a new target.
func (h *Handler) nestedTargetLocation(id string, t store.OffsiteTarget) error {
	settings, err := h.store.GetSettings()
	if err != nil {
		return fmt.Errorf("read settings to check this location: %w", err)
	}
	loc, err := h.svc.resolveRepo(t.Repo)
	if err != nil {
		return err
	}
	self := locationSelf{target: true}
	if id != "" {
		self.ids = []string{id}
		field, ok, err := h.store.FieldOffsiteTarget(t.Domain)
		if err != nil {
			return err
		}
		if ok && field.ID == id {
			self.field = t.Domain
		}
	}
	return h.svc.locationClash(settings, loc, self)
}

// removeHalfMadeTarget takes back a target whose exclusions could not be
// written and returns the error to answer with. Nothing on screen shows that
// target, so the next move is to press the button again, and
// CreateOffsiteTarget mints a fresh id every time.
func (h *Handler) removeHalfMadeTarget(id string, cause error) error {
	if err := h.store.DeleteOffsiteTarget(id); err != nil {
		return fmt.Errorf("%w; the target could not be taken back either: %v", cause, err)
	}
	return cause
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

// handleCreateOffsiteTarget creates an additional off-site target behind the
// domain's last one. POST /api/offsite/targets; the body's id, createdAt and
// sortOrder are ignored.
func (h *Handler) handleCreateOffsiteTarget(w http.ResponseWriter, r *http.Request) {
	var v offsiteTargetBody
	if !decodeBody(w, r, &v) {
		return
	}
	t := v.toStoreTarget()
	if msg := validateOffsiteTargetInput(t); msg != "" {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": msg})
		return
	}
	if msg := h.rejectOffsiteTargetOnNamedRepo(t); msg != "" {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": msg})
		return
	}
	if err := h.nestedTargetLocation("", t); err != nil {
		placementFail(w, err, nil)
		return
	}
	if v.AlsoExclude != nil {
		if err := checkExclusion(t.Domain, *v.AlsoExclude); err != nil {
			placementFail(w, err, nil)
			return
		}
	}
	stored, err := h.store.CreateOffsiteTarget(t)
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	if v.AlsoExclude != nil {
		if err := h.svc.excludeFromTarget(stored.Domain, stored.ID, *v.AlsoExclude); err != nil {
			placementFail(w, h.removeHalfMadeTarget(stored.ID, err), nil)
			return
		}
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"target": offsiteTargetToView(stored)}))
}

// handleUpdateOffsiteTarget updates an existing off-site target in place.
// PUT /api/offsite/targets/{id}. The id comes from the path; created_at and
// sort_order stay as stored, and a request that tries to move the target to
// another domain is refused (see the domain check below).
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
	var v offsiteTargetBody
	if !decodeBody(w, r, &v) {
		return
	}
	t := v.toStoreTarget()
	if msg := validateOffsiteTargetInput(t); msg != "" {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": msg})
		return
	}
	// PUT keeps the stored sort_order, so a domain change would carry the row's
	// current slot into a domain where that slot may already be taken.
	if t.Domain != existing.Domain {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": "cannot move an off-site target to another domain"})
		return
	}
	// Checked only when the request moves the target, as in
	// rejectSettingsPathOnNamedRepo, so a row the settings import left on a
	// colliding location can still be edited rather than only deleted.
	moved := !sameRepoLocation(strings.TrimSpace(t.Repo), strings.TrimSpace(existing.Repo))
	if moved {
		if msg := h.rejectOffsiteTargetOnNamedRepo(t); msg != "" {
			writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": msg})
			return
		}
		if err := h.nestedTargetLocation(existing.ID, t); err != nil {
			placementFail(w, err, nil)
			return
		}
	}
	// The answer is kept whether or not this save moves the target: the client
	// asks as soon as the field changed, and two spellings of one location are
	// no reason to drop what the user chose to leave out.
	if v.AlsoExclude != nil {
		if err := checkExclusion(t.Domain, *v.AlsoExclude); err != nil {
			placementFail(w, err, nil)
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
	if v.AlsoExclude != nil {
		if err := h.svc.excludeFromTarget(stored.Domain, stored.ID, *v.AlsoExclude); err != nil {
			// The row keeps its id, so there is nothing to take back here; the
			// answer says which half of the save went through.
			placementFail(w, fmt.Errorf("%w: %w", errExclusionUnsaved, err), nil)
			return
		}
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{
		"target":   offsiteTargetToView(stored),
		"warnings": h.svc.targetSaveWarnings(r.Context(), existing, stored),
	}))
}

// handleDeleteOffsiteTarget removes an off-site target and its direct
// repository (a no-op, still ok, when the target does not exist), refused
// while an item or a default still uses that direct repository.
// DELETE /api/offsite/targets/{id}.
func (h *Handler) handleDeleteOffsiteTarget(w http.ResponseWriter, r *http.Request) {
	use, err := h.store.DeleteOffsiteTargetIfUnused(r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	if use.InUse() {
		placementFail(w, errTargetInUse, map[string]any{"use": targetUseView(use)})
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
