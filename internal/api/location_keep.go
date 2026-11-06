package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"slices"
)

// repoPick narrows an operation over a domain's repositories to the ones it
// takes.
type repoPick func(domainRepoRef) bool

func (p repoPick) from(repos []domainRepoRef) []domainRepoRef {
	return slices.DeleteFunc(slices.Clone(repos), func(r domainRepoRef) bool { return !p(r) })
}

// keepScope is one section of a storage location as the retention pass reaches
// it: the source its backups are listed under and the repository of that
// source the location holds.
type keepScope struct {
	domain, use, source string
	pick                repoPick
}

// keepScopes are the sections of loc. A copy is its target's source, which
// names one repository already. A home is the domain's local source narrowed
// to its own folder or to the named repository it lies in.
func keepScopes(loc storageLocation) []keepScope {
	out := make([]keepScope, 0, len(loc.Sections))
	for _, sec := range loc.Sections {
		sc := keepScope{domain: sec.Domain, use: sec.Use, source: "local"}
		switch id := sec.RepoID; {
		case sec.Use == useCopy:
			sc.source = offsiteSourcePrefix + sec.TargetID
		case id != "":
			sc.pick = func(r domainRepoRef) bool { return r.Named.ID == id }
		default:
			sc.pick = func(r domainRepoRef) bool { return r.Own }
		}
		out = append(out, sc)
	}
	return out
}

// LocationKeepSection is one section's part of a location keep preview.
type LocationKeepSection struct {
	Domain  string           `json:"domain"`
	Use     string           `json:"use"`
	Preview RetentionPreview `json:"preview"`
	// Empty says the section has no backups at this location yet.
	Empty bool   `json:"empty,omitempty"`
	Error string `json:"error,omitempty"`
}

// LocationKeepPreview is what applying a storage location's keep rule now
// would remove, one part per section kept there.
type LocationKeepPreview struct {
	Sections []LocationKeepSection `json:"sections"`
	// Empty says no section has backups at this location yet.
	Empty bool `json:"empty,omitempty"`
}

// PreviewLocationKeep reports what ApplyLocationKeep would remove from each
// section of the location, without changing anything.
func (s *Service) PreviewLocationKeep(ctx context.Context, id string) (LocationKeepPreview, error) {
	loc, err := s.StorageLocation(id)
	if err != nil {
		return LocationKeepPreview{}, err
	}
	out := LocationKeepPreview{Sections: []LocationKeepSection{}, Empty: true}
	for _, sc := range keepScopes(loc) {
		part := LocationKeepSection{Domain: sc.domain, Use: sc.use}
		part.Preview, err = s.previewRetention(ctx, sc.domain, sc.source, sc.pick)
		switch {
		case err != nil && err.Error() == noBackupsYet:
			part.Empty = true
		case err != nil:
			part.Error = scrubError(err)
		}
		if part.Preview.Repos == nil {
			part.Preview.Repos = []RetentionPreviewRepo{}
		}
		for i := range part.Preview.Repos {
			if part.Preview.Repos[i].Items == nil {
				part.Preview.Repos[i].Items = []RetentionPreviewItem{}
			}
		}
		out.Empty = out.Empty && part.Empty
		out.Sections = append(out.Sections, part)
	}
	return out, nil
}

// LocationKeepResult is what applying a storage location's keep rule did.
type LocationKeepResult struct {
	// Paused names the entries whose old backups were kept because an unusual
	// backup is holding them.
	Paused []string `json:"paused,omitempty"`
	// Empty says no section had backups at this location yet.
	Empty bool `json:"empty,omitempty"`
}

// ApplyLocationKeep runs the retention pass a backup would run next on every
// section kept at the location, and only on its repositories there: each
// section ages by the rule that decides for it, which is the location's card
// unless the section has a rule of its own. Each section's pass takes its
// domain's lock and is recorded as a prune run. A section that fails leaves
// the others to go ahead, and its error comes back with theirs.
func (s *Service) ApplyLocationKeep(ctx context.Context, id string) (LocationKeepResult, error) {
	loc, err := s.StorageLocation(id)
	if err != nil {
		return LocationKeepResult{}, err
	}
	res := LocationKeepResult{Empty: true}
	var errs []error
	for _, sc := range keepScopes(loc) {
		paused, err := s.pruneDomain(ctx, sc.domain, sc.source, sc.pick, false)
		res.Paused = append(res.Paused, paused...)
		if err != nil && err.Error() == noBackupsToPrune {
			continue
		}
		res.Empty = false
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", sc.domain, err))
		}
	}
	return res, errors.Join(errs...)
}

// handleLocationKeepPreview says what applying a storage location's keep rule
// now would remove. It asks for the login the apply needs, so a refusal comes
// before the list rather than after it.
// GET /api/storage/locations/{id}/keep/preview
func (h *Handler) handleLocationKeepPreview(w http.ResponseWriter, r *http.Request) {
	if !h.requireAuthForSecrets(w, r, "pruning backups") {
		return
	}
	preview, err := h.svc.PreviewLocationKeep(r.Context(), r.PathValue("id"))
	switch {
	case errors.Is(err, errNoSuchLocation):
		writeJSON(w, http.StatusNotFound, map[string]any{"ok": false, "error": err.Error()})
	case err != nil:
		writeJSON(w, http.StatusOK, failEnvelope(err))
	default:
		writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"preview": preview}))
	}
}

// handleApplyLocationKeep applies a storage location's keep rule to every
// section kept there, without waiting for the next backup.
// POST /api/storage/locations/{id}/keep/apply
func (h *Handler) handleApplyLocationKeep(w http.ResponseWriter, r *http.Request) {
	if !h.requireAuthForSecrets(w, r, "pruning backups") {
		return
	}
	ctx, done := h.maintenanceContext(r)
	defer done()
	res, err := h.svc.ApplyLocationKeep(ctx, r.PathValue("id"))
	switch {
	case errors.Is(err, errNoSuchLocation):
		writeJSON(w, http.StatusNotFound, map[string]any{"ok": false, "error": err.Error()})
	case err != nil:
		body := failEnvelope(err)
		if len(res.Paused) > 0 {
			body["paused"] = res.Paused
		}
		writeJSON(w, http.StatusOK, body)
	default:
		writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"paused": res.Paused, "empty": res.Empty}))
	}
}
