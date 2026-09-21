package api

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

var errNestedLocation = errors.New("this location lies inside another repository or target, or contains one")

var errMirroredField = errors.New("a direct repository takes this value from its target; change it there")

// repoLocationsOverlap reports whether two locations are the same place or one
// lies inside the other, path element by path element.
func repoLocationsOverlap(a, b string) bool {
	storeA, pathA := locationParts(a)
	storeB, pathB := locationParts(b)
	if storeA != storeB {
		return false
	}
	if len(pathA) > len(pathB) {
		pathA, pathB = pathB, pathA
	}
	return slices.Equal(pathA, pathB[:len(pathA)])
}

// locationParts splits a location into the store it lives in (scheme with host,
// bucket or remote, empty for a local path) and the path elements below it.
// Credentials in a URL do not name a different place.
func locationParts(loc string) (string, []string) {
	loc = strings.TrimSpace(loc)
	if !restic.IsRemoteRepo(loc) {
		return "", pathElements(filepath.ToSlash(filepath.Clean(loc)))
	}
	scheme, rest, _ := strings.Cut(loc, ":")
	switch scheme {
	case "rest", "s3":
		host, path, _ := strings.Cut(strings.TrimPrefix(strings.TrimPrefix(rest, "https://"), "http://"), "/")
		return scheme + ":" + hostOnly(host), pathElements(path)
	case "sftp":
		if url, ok := strings.CutPrefix(rest, "//"); ok {
			host, path, _ := strings.Cut(url, "/")
			return scheme + ":" + hostOnly(host), pathElements(path)
		}
		host, path, _ := strings.Cut(rest, ":")
		return scheme + ":" + hostOnly(host), pathElements(path)
	}
	name, path, _ := strings.Cut(rest, ":")
	return scheme + ":" + name, pathElements(path)
}

func hostOnly(host string) string {
	if i := strings.LastIndexByte(host, '@'); i >= 0 {
		host = host[i+1:]
	}
	return strings.ToLower(host)
}

func pathElements(p string) []string {
	var out []string
	for _, e := range strings.Split(p, "/") {
		if e != "" && e != "." {
			out = append(out, e)
		}
	}
	return out
}

// locationSelf is what the location being checked already stands for, so
// locationClash does not hold a place against itself: rows by id, and the
// domain whose own repository or off-site field it is.
type locationSelf struct {
	ids   []string
	own   string
	field string
}

// locationClash refuses a location that is, holds or lies inside a domain
// repository, an off-site field, an off-site target or a named repository.
// Two rows over one place, or one inside the other, would give every question
// about that place two answers.
func (s *Service) locationClash(settings store.Settings, loc string, self locationSelf) error {
	for _, d := range offsiteConfigDomains {
		// Two domains may share one repository; one inside the other may not.
		if own, err := s.repoFor(settings, d, "local"); err == nil && d != self.own && repoLocationsOverlap(own, loc) &&
			(self.own == "" || !sameRepoLocation(own, loc)) {
			return fmt.Errorf("%w: the %s domain's own repository", errNestedLocation, d)
		}
		if off := offsiteRepoFromSettings(d, settings); off != "" && d != self.field {
			if offLoc, err := s.resolveRepo(off); err == nil && repoLocationsOverlap(offLoc, loc) {
				return fmt.Errorf("%w: the %s domain's off-site destination", errNestedLocation, d)
			}
		}
	}
	targets, err := s.store.ListOffsiteTargets()
	if err != nil {
		return fmt.Errorf("read the off-site targets to check this location: %w", err)
	}
	for _, t := range targets {
		if slices.Contains(self.ids, t.ID) {
			continue
		}
		if tLoc, rErr := s.resolveRepo(t.Repo); rErr == nil && repoLocationsOverlap(tLoc, loc) {
			return fmt.Errorf("%w: the off-site destination %q", errNestedLocation, t.Name)
		}
	}
	named, err := s.store.ListNamedRepos()
	if err != nil {
		return fmt.Errorf("read the repositories to check this location: %w", err)
	}
	for _, r := range named {
		if slices.Contains(self.ids, r.ID) {
			continue
		}
		if other, rErr := s.resolveRepo(r.Repo); rErr == nil && repoLocationsOverlap(other, loc) {
			return fmt.Errorf("%w: another repository", errNestedLocation)
		}
	}
	return nil
}

// directSuggestion is the location the create dialog starts from.
type directSuggestion struct {
	Location string `json:"location"`
	Note     string `json:"note"` // "" | "bucket-root" | "path-needed"
}

// directLocationFor derives where a target's direct repository goes: beside the
// target, never inside it. A bucket root has nothing beside it in its bucket,
// and a rest-server user root may forbid everything outside it, so those two
// get a note instead of a plain suggestion.
func directLocationFor(target store.OffsiteTarget) directSuggestion {
	loc := strings.TrimRight(strings.TrimSpace(target.Repo), "/:")
	place, elems := locationParts(loc)
	scheme, _, _ := strings.Cut(place, ":")
	switch {
	case scheme == "rest" && len(elems) < 2,
		(scheme == "sftp" || scheme == "rclone") && len(elems) == 0:
		return directSuggestion{Note: "path-needed"}
	case scheme == "s3" && len(elems) < 2,
		(scheme == "b2" || scheme == "gs" || scheme == "azure" || scheme == "swift") && len(elems) == 0:
		return directSuggestion{Location: loc + "-direct", Note: "bucket-root"}
	}
	return directSuggestion{Location: loc + "-direct"}
}

// directLocation checks a direct repository location the way the Repositories
// card checks any named one, plus the overlap rule, and returns it resolved.
func (s *Service) directLocation(settings store.Settings, location string) (string, error) {
	if msg := staticNamedRepoRefusals(location, s.cfg.HostMountRoot); msg != "" {
		return "", errors.New(msg)
	}
	loc, err := s.resolveRepo(strings.TrimSpace(location))
	if err != nil {
		return "", err
	}
	if err := s.locationClash(settings, loc, locationSelf{}); err != nil {
		return "", err
	}
	return loc, nil
}

// probeDirectLocation is the create dialog's connection test. It opens location
// with the target's credentials and writes nothing; a local path that does not
// exist yet, or is empty, counts as reachable and empty.
func (s *Service) probeDirectLocation(ctx context.Context, target store.OffsiteTarget, location string) (reachable, initialized bool, err error) {
	settings, err := s.store.GetSettings()
	if err != nil {
		return false, false, fmt.Errorf("read settings: %w", err)
	}
	loc, err := s.directLocation(settings, location)
	if err != nil {
		return false, false, err
	}
	if !restic.IsRemoteRepo(loc) {
		entries, rErr := os.ReadDir(loc)
		if errors.Is(rErr, fs.ErrNotExist) || (rErr == nil && len(entries) == 0) {
			return true, false, nil
		}
	}
	return s.probeOffsiteRepo(ctx, loc, s.offsiteModeForTarget(settings, target))
}

// offsiteTargetParam reads {id} as an off-site target and writes the refusal
// itself when there is none.
func (h *Handler) offsiteTargetParam(w http.ResponseWriter, r *http.Request) (store.OffsiteTarget, bool) {
	target, ok, err := h.store.GetOffsiteTarget(r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return store.OffsiteTarget{}, false
	}
	if !ok {
		placementFail(w, errUnknownOffsiteTarget, nil)
	}
	return target, ok
}

// handleGetDirectRepo serves GET /api/offsite/targets/{id}/direct: the target's
// direct repository, null while there is none, and where the dialog starts.
func (h *Handler) handleGetDirectRepo(w http.ResponseWriter, r *http.Request) {
	target, ok := h.offsiteTargetParam(w, r)
	if !ok {
		return
	}
	direct, found, err := h.store.CompanionFor(target.ID)
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	var repo any
	if found {
		repo = h.namedRepoViews([]store.OffsiteTarget{direct})[0]
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"repo": repo, "suggestion": directLocationFor(target)}))
}

// handleTestDirectLocation serves POST /api/offsite/targets/{id}/direct/test.
func (h *Handler) handleTestDirectLocation(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Location string `json:"location"`
	}
	if !decodeBody(w, r, &body) {
		return
	}
	target, ok := h.offsiteTargetParam(w, r)
	if !ok {
		return
	}
	reachable, initialized, err := h.svc.probeDirectLocation(r.Context(), target, body.Location)
	if err != nil {
		placementFail(w, err, nil)
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"reachable": reachable, "initialized": initialized}))
}

// opensWith is one bounded RepoOpens, so a dead backend cannot hold a request.
func (s *Service) opensWith(ctx context.Context, loc string, mode restic.Mode) bool {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	return s.engine.RepoOpens(ctx, loc, mode)
}

// createDirectRepo is "Create and use" in the create dialog. It sets up the
// restic repository with the target's credentials and only then writes the row,
// so a dialog that is cancelled leaves nothing in the bucket.
func (s *Service) createDirectRepo(ctx context.Context, targetID, name, location string) (store.OffsiteTarget, error) {
	target, ok, err := s.store.GetOffsiteTarget(targetID)
	if err != nil {
		return store.OffsiteTarget{}, err
	}
	if !ok || !validPlacementDomain(target.Domain) {
		return store.OffsiteTarget{}, errUnknownOffsiteTarget
	}
	_, taken, err := s.store.CompanionFor(targetID)
	if err != nil {
		return store.OffsiteTarget{}, err
	}
	if taken {
		return store.OffsiteTarget{}, store.ErrCompanionTaken
	}
	settings, err := s.store.GetSettings()
	if err != nil {
		return store.OffsiteTarget{}, fmt.Errorf("read settings: %w", err)
	}
	loc, err := s.directLocation(settings, location)
	if err != nil {
		return store.OffsiteTarget{}, err
	}
	if err := s.EnsureRepo(ctx, loc, s.offsiteModeForTarget(settings, target)); err != nil {
		return store.OffsiteTarget{}, err
	}
	if strings.TrimSpace(name) == "" {
		name = placementTargetName(target) + " direct"
	}
	return s.store.CreateCompanionRepo(targetID, name, location)
}

// connectDirectRepo is "Connect with <target>" after Discover found bv:direct
// snapshots in a plain named repository. The repository has to open with the
// target's credentials before it is linked.
func (s *Service) connectDirectRepo(ctx context.Context, repoID, targetID string) (store.OffsiteTarget, error) {
	repo, err := s.store.GetNamedRepo(repoID)
	if err != nil {
		return store.OffsiteTarget{}, errors.New("no such repository")
	}
	if repo.CompanionOf != "" && repo.CompanionOf != targetID {
		return store.OffsiteTarget{}, errors.New("this repository already belongs to another target")
	}
	target, ok, err := s.store.GetOffsiteTarget(targetID)
	if err != nil {
		return store.OffsiteTarget{}, err
	}
	if !ok {
		return store.OffsiteTarget{}, errUnknownOffsiteTarget
	}
	settings, err := s.store.GetSettings()
	if err != nil {
		return store.OffsiteTarget{}, fmt.Errorf("read settings: %w", err)
	}
	loc, err := s.resolveRepo(repo.Repo)
	if err != nil {
		return store.OffsiteTarget{}, err
	}
	if !s.opensWith(ctx, loc, s.offsiteModeForTarget(settings, target)) {
		return store.OffsiteTarget{}, errors.New("the repository does not open with the target's credentials, so it was not connected")
	}
	if err := s.store.ConnectCompanion(repoID, targetID); err != nil {
		return store.OffsiteTarget{}, err
	}
	return s.store.GetNamedRepo(repoID)
}

// handleCreateDirectRepo is POST /api/repos with companionOf. Everything but the
// name and the location comes from the target.
func (h *Handler) handleCreateDirectRepo(w http.ResponseWriter, r *http.Request, body namedRepoBody) {
	if body.CredsRef != nil || body.StorageClass != nil || body.LimitUpload != nil ||
		body.LimitDownload != nil || body.Immutable != nil || body.Enabled != nil {
		placementFail(w, errMirroredField, nil)
		return
	}
	var name, loc string
	if body.Name != nil {
		name = *body.Name
	}
	if body.Repo != nil {
		loc = *body.Repo
	}
	row, err := h.svc.createDirectRepo(r.Context(), strings.TrimSpace(*body.CompanionOf), name, loc)
	if err != nil {
		placementFail(w, err, nil)
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"repo": h.namedRepoViews([]store.OffsiteTarget{row})[0]}))
}

// handleConnectRepo serves POST /api/repos/{id}/connect.
func (h *Handler) handleConnectRepo(w http.ResponseWriter, r *http.Request) {
	var body struct {
		TargetID string `json:"targetId"`
	}
	if !decodeBody(w, r, &body) {
		return
	}
	row, err := h.svc.connectDirectRepo(r.Context(), strings.TrimSpace(r.PathValue("id")), strings.TrimSpace(body.TargetID))
	if err != nil {
		placementFail(w, err, nil)
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"repo": h.namedRepoViews([]store.OffsiteTarget{row})[0]}))
}
