package api

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log"
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

var errForeignDomain = errors.New("that direct repository belongs to a target of another domain")

var errTargetInUse = errors.New("this target's direct repository is still in use")

// targetUseView is the JSON shape of the store.TargetUse a refused off-site
// target delete carries back, so the SPA can point at what is still using the
// direct repository.
func targetUseView(u store.TargetUse) map[string]any {
	domains := u.DefaultDomains
	if domains == nil {
		domains = []string{}
	}
	return map[string]any{"directRepoId": u.CompanionID, "items": u.Items, "defaultDomains": domains}
}

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
// locationClash does not hold a place against itself: rows by id, the domain
// whose own repository or off-site field it is, and whether it is a target.
type locationSelf struct {
	ids    []string
	own    string
	field  string
	target bool
}

// shared reports whether this is a place other rows may name as well: a
// domain's own repository, a domain's off-site destination or an off-site
// target. A named repository owns its place alone, so that every question
// asked about that place has one answer.
func (s locationSelf) shared() bool { return s.own != "" || s.field != "" || s.target }

// clashCandidate resolves one place locationClash holds a location against.
// A place that does not resolve is left out of the check and said out loud: a
// clash nobody finds is a second row over one place.
func (s *Service) clashCandidate(kind, name, location string) (string, bool) {
	loc, err := s.resolveRepo(location)
	if err != nil {
		log.Printf("api: location check: the %s %q does not resolve, so this location was not held against it: %v", kind, name, err) //nolint:gosec // G706: the kind is fixed text and the name is %q-quoted
		return "", false
	}
	return loc, true
}

// locationClash refuses a location that holds or lies inside a domain
// repository, an off-site field, an off-site target or a named repository, and
// a second row over a named repository's place.
func (s *Service) locationClash(settings store.Settings, loc string, self locationSelf) error {
	// Two domains on one destination is a setup people run today, so only
	// nesting is refused between places that may be shared.
	clashes := func(other string) bool {
		return repoLocationsOverlap(other, loc) && (!self.shared() || !sameRepoLocation(other, loc))
	}
	for _, d := range offsiteConfigDomains {
		own, oErr := s.repoFor(settings, d, "local")
		switch {
		case oErr != nil:
			log.Printf("api: location check: the %s domain's own repository does not resolve, so this location was not held against it: %v", d, oErr) //nolint:gosec // G706: the domain is a fixed literal
		case d != self.own && clashes(own):
			return fmt.Errorf("%w: the %s domain's own repository", errNestedLocation, d)
		}
		if off := offsiteRepoFromSettings(d, settings); off != "" && d != self.field {
			if offLoc, ok := s.clashCandidate("off-site destination of the domain", d, off); ok && clashes(offLoc) {
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
		if tLoc, ok := s.clashCandidate("off-site destination", t.Name, t.Repo); ok && clashes(tLoc) {
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
		if other, ok := s.clashCandidate("repository", r.Name, r.Repo); ok && repoLocationsOverlap(other, loc) {
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

// offsiteTargetParam reads {id} as an off-site target of a domain that can
// have a direct repository, and writes the refusal itself when there is none.
// The domain is checked here rather than only on the create, so the dialog
// cannot suggest and probe a location for a target that can never take one.
func (h *Handler) offsiteTargetParam(w http.ResponseWriter, r *http.Request) (store.OffsiteTarget, bool) {
	target, ok, err := h.store.GetOffsiteTarget(r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return store.OffsiteTarget{}, false
	}
	if !ok || !validPlacementDomain(target.Domain) {
		placementFail(w, errUnknownOffsiteTarget, nil)
		return store.OffsiteTarget{}, false
	}
	return target, true
}

// offsiteTargetRef names a target for a refusal that has to point at it. A
// name that cannot be read falls back to the id, the way the card does it.
func (h *Handler) offsiteTargetRef(id string) directTargetRef {
	ref := directTargetRef{ID: id, Name: id}
	if target, ok, err := h.store.GetOffsiteTarget(id); err == nil && ok {
		ref.Name = placementTargetName(target)
	}
	return ref
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

// repoInUseErr is a repo-in-use refusal carrying what still points at the
// repository, so the answer can name it the way the delete route does.
type repoInUseErr struct{ use store.NamedRepoUse }

func (e *repoInUseErr) Error() string { return errRepoInUse.Error() }

func (e *repoInUseErr) Is(target error) bool { return target == errRepoInUse }

// namedRepoUse counts what points at a named repository: the same two
// questions the guarded writes ask inside their own transaction.
func (s *Service) namedRepoUse(repoID string) (store.NamedRepoUse, error) {
	items, err := s.store.ItemsUsingNamedRepo(repoID)
	if err != nil {
		return store.NamedRepoUse{}, err
	}
	defaults, err := s.store.ListPlacementDefaults()
	if err != nil {
		return store.NamedRepoUse{}, err
	}
	use := store.NamedRepoUse{Items: items, DefaultDomains: []string{}}
	for _, d := range defaults {
		if d.Home == repoID {
			use.DefaultDomains = append(use.DefaultDomains, d.Domain)
		}
	}
	slices.Sort(use.DefaultDomains)
	return use, nil
}

// connectDirectRepo is "Connect with <target>" after Discover found bv:direct
// snapshots in a plain named repository. The repository has to open with the
// target's credentials before it is linked.
//
// A repository an item or a default still uses is refused: linking it would
// age its whole history under the target's rules and stop its off-site copies,
// without anybody having chosen either.
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
	if !ok || !validPlacementDomain(target.Domain) {
		return store.OffsiteTarget{}, errUnknownOffsiteTarget
	}
	use, err := s.namedRepoUse(repoID)
	if err != nil {
		return store.OffsiteTarget{}, err
	}
	if use.InUse() {
		return store.OffsiteTarget{}, &repoInUseErr{use: use}
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

// importedLink is the link a new row from a settings file gets: its target,
// when that target exists here and has no direct repository yet, otherwise
// none, with lost true when the file named a target that could not be used.
func (h *Handler) importedLink(companionOf string) (id string, lost bool, err error) {
	companionOf = strings.TrimSpace(companionOf)
	if companionOf == "" {
		return "", false, nil
	}
	_, isTarget, err := h.store.GetOffsiteTarget(companionOf)
	if err != nil {
		return "", false, err
	}
	_, taken, err := h.store.CompanionFor(companionOf)
	if err != nil {
		return "", false, err
	}
	if !isTarget || taken {
		return "", true, nil
	}
	return companionOf, false, nil
}

// handleCreateDirectRepo is POST /api/repos with companionOf. Everything but the
// name and the location comes from the target.
func (h *Handler) handleCreateDirectRepo(w http.ResponseWriter, r *http.Request, body namedRepoBody) {
	if fields := lockedFieldsSet(body); len(fields) > 0 {
		placementFail(w, errMirroredField, map[string]any{"fields": fields})
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

// lockedFieldsSet names the mirrored fields a direct repository's create body
// sets; only its name and location are the caller's to choose, everything
// else has to be set on the target instead.
func lockedFieldsSet(b namedRepoBody) []string {
	var fields []string
	add := func(name string, set bool) {
		if set {
			fields = append(fields, name)
		}
	}
	add("credsRef", b.CredsRef != nil)
	add("storageClass", b.StorageClass != nil)
	add("limitUpload", b.LimitUpload != nil)
	add("limitDownload", b.LimitDownload != nil)
	add("immutable", b.Immutable != nil)
	add("enabled", b.Enabled != nil)
	add("offPremises", b.OffPremises != nil)
	return fields
}

// mirroredFieldsChanged names the fields an edit to a direct repository sends
// that differ from the stored row: everything but its name and on/off switch,
// which come from the target rather than the edit.
func mirroredFieldsChanged(b namedRepoBody, row store.OffsiteTarget) []string {
	differs := func(p *string, stored string) bool { return p != nil && strings.TrimSpace(*p) != stored }
	var fields []string
	add := func(name string, changed bool) {
		if changed {
			fields = append(fields, name)
		}
	}
	add("repo", differs(b.Repo, row.Repo))
	add("credsRef", differs(b.CredsRef, row.CredsRef))
	add("storageClass", b.StorageClass != nil && !strings.EqualFold(strings.TrimSpace(*b.StorageClass), row.StorageClass))
	add("limitUpload", b.LimitUpload != nil && *b.LimitUpload != row.LimitUpload)
	add("limitDownload", b.LimitDownload != nil && *b.LimitDownload != row.LimitDownload)
	add("immutable", b.Immutable != nil && *b.Immutable != row.Immutable)
	add("offPremises", b.OffPremises != nil && *b.OffPremises != row.OffPremises)
	return fields
}

// directTags is DirectTag for a backup into a direct repository, so every
// retention that is not the row's own keeps the snapshot.
func (s *Service) directTags(settings store.Settings, domain, repo string) []string {
	if s.refFor(settings, domain, repo).Named.CompanionOf != "" {
		return []string{restic.DirectTag}
	}
	return nil
}

// withTags appends extra in a fresh slice; the orchestrators reuse theirs.
func withTags(tags, extra []string) []string {
	if len(extra) == 0 {
		return tags
	}
	return append(slices.Clone(tags), extra...)
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
	var inUse *repoInUseErr
	if errors.As(err, &inUse) {
		placementFail(w, err, map[string]any{"items": inUse.use.Items, "defaultDomains": inUse.use.DefaultDomains})
		return
	}
	if err != nil {
		placementFail(w, err, nil)
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"repo": h.namedRepoViews([]store.OffsiteTarget{row})[0]}))
}

// saveWarning is a warning the answer of a save carries about a direct repository.
type saveWarning struct {
	Code       string `json:"code"` // "direct-retention-lowered" | "direct-append-only-off" | "direct-creds-kept"
	TargetID   string `json:"targetId"`
	TargetName string `json:"targetName"`
	Items      int    `json:"items"`
}

// retentionLowered reports whether after keeps fewer snapshots than before. All
// zero keeps everything, and the dimensions add up, so any one that shrinks
// keeps less.
func retentionLowered(before, after restic.RetentionPolicy) bool {
	if !after.Any() {
		return false
	}
	if !before.Any() {
		return true
	}
	return after.KeepLast < before.KeepLast || after.KeepDaily < before.KeepDaily ||
		after.KeepWeekly < before.KeepWeekly || after.KeepMonthly < before.KeepMonthly
}

// directSaveWarnings compares a target before and after a save and reports what
// the change means for items whose only copy is in its direct repository.
func (s *Service) directSaveWarnings(before, after store.OffsiteTarget) ([]saveWarning, error) {
	out := []saveWarning{}
	lowered := retentionLowered(targetOffsiteRetentionPolicy(before), targetOffsiteRetentionPolicy(after))
	appendOnlyOff := before.Immutable && !after.Immutable
	if !lowered && !appendOnlyOff {
		return out, nil
	}
	direct, ok, err := s.store.CompanionFor(after.ID)
	if err != nil || !ok {
		return out, err
	}
	n, err := s.store.ItemsUsingNamedRepo(direct.ID)
	if err != nil || n == 0 {
		return out, err
	}
	w := saveWarning{TargetID: after.ID, TargetName: placementTargetName(after), Items: n}
	if lowered {
		w.Code = "direct-retention-lowered"
		out = append(out, w)
	}
	if appendOnlyOff {
		w.Code = "direct-append-only-off"
		out = append(out, w)
	}
	return out, nil
}

// mirrorDirectCreds opens a target's direct repository with the target's
// current credentials and mirrors them when it opens. When it does not, the row
// keeps what it has and the answer is a direct-creds-kept warning.
func (s *Service) mirrorDirectCreds(ctx context.Context, targetID string) (*saveWarning, error) {
	target, ok, err := s.store.GetOffsiteTarget(targetID)
	if err != nil || !ok {
		return nil, err
	}
	direct, ok, err := s.store.CompanionFor(targetID)
	if err != nil || !ok {
		return nil, err
	}
	settings, err := s.store.GetSettings()
	if err != nil {
		return nil, fmt.Errorf("read settings: %w", err)
	}
	loc, err := s.resolveRepo(direct.Repo)
	if err != nil {
		return nil, err
	}
	if !s.opensWith(ctx, loc, s.offsiteModeForTarget(settings, target)) {
		n, err := s.store.ItemsUsingNamedRepo(direct.ID)
		if err != nil || n == 0 {
			return nil, err
		}
		return &saveWarning{Code: "direct-creds-kept", TargetID: target.ID, TargetName: placementTargetName(target), Items: n}, nil
	}
	_, err = s.store.MirrorCompanionCreds(targetID)
	return nil, err
}

// targetSaveWarnings is what a saved target means for its direct repository.
// The save stands either way, so a failed read is logged, not returned.
func (s *Service) targetSaveWarnings(ctx context.Context, before, after store.OffsiteTarget) []saveWarning {
	out, err := s.directSaveWarnings(before, after)
	if err != nil {
		log.Printf("api: target %s: could not check its direct repository after a save: %v", after.ID, err) //nolint:gosec // G706: the id is store-generated
		out = []saveWarning{}
	}
	direct, ok, err := s.store.CompanionFor(after.ID)
	if err != nil || !ok || direct.CredsRef == after.CredsRef {
		return out
	}
	w, err := s.mirrorDirectCreds(ctx, after.ID)
	if err != nil {
		log.Printf("api: target %s: could not probe its direct repository with the new credentials: %v", after.ID, err) //nolint:gosec // G706: the id is store-generated
		return out
	}
	if w != nil {
		out = append(out, *w)
	}
	return out
}

// directCredsWarnings runs mirrorDirectCreds for the direct repositories pick
// selects after a credential save.
func (s *Service) directCredsWarnings(ctx context.Context, pick func(direct, target store.OffsiteTarget) bool) []saveWarning {
	out := []saveWarning{}
	repos, err := s.store.ListNamedRepos()
	if err != nil {
		log.Printf("api: could not read the direct repositories after a credential save: %v", err)
		return out
	}
	for _, d := range repos {
		if d.CompanionOf == "" {
			continue
		}
		target, ok, err := s.store.GetOffsiteTarget(d.CompanionOf)
		if err != nil {
			log.Printf("api: repository %s: could not read its target: %v", d.ID, err) //nolint:gosec // G706: the id is store-generated
			continue
		}
		if !ok || !pick(d, target) {
			continue
		}
		w, err := s.mirrorDirectCreds(ctx, target.ID)
		if err != nil {
			log.Printf("api: repository %s: could not probe it with the new credentials: %v", d.ID, err) //nolint:gosec // G706: the id is store-generated
			continue
		}
		if w != nil {
			out = append(out, *w)
		}
	}
	return out
}

type directFinding struct {
	RepoID     string
	Name       string
	Candidates []store.OffsiteTarget // the domain's targets without a direct repository, <location>-direct first
}

// holdsDirect reports whether a listing has a snapshot of this domain that was
// written straight into a direct repository.
func holdsDirect(snaps []restic.Snapshot, tagPrefix string) bool {
	for _, sn := range snaps {
		if !slices.Contains(sn.Tags, restic.DirectTag) {
			continue
		}
		for _, tag := range sn.Tags {
			if strings.HasPrefix(tag, tagPrefix) {
				return true
			}
		}
	}
	return false
}

// directFindings pairs each plain repository holding bv:direct snapshots of a
// domain with the domain's targets that could take it back, the one whose
// derived location it matches first.
func (s *Service) directFindings(domain string, rows []store.OffsiteTarget) ([]directFinding, error) {
	out := []directFinding{}
	if len(rows) == 0 {
		return out, nil
	}
	targets, err := s.store.OffsiteTargetsForDomain(domain)
	if err != nil {
		return out, err
	}
	var free []store.OffsiteTarget
	for _, t := range targets {
		_, taken, err := s.store.CompanionFor(t.ID)
		if err != nil {
			return out, err
		}
		if !taken {
			free = append(free, t)
		}
	}
	for _, r := range rows {
		var match, rest []store.OffsiteTarget
		for _, t := range free {
			if sameRepoLocation(directLocationFor(t).Location, r.Repo) {
				match = append(match, t)
			} else {
				rest = append(rest, t)
			}
		}
		out = append(out, directFinding{RepoID: r.ID, Name: r.Name, Candidates: append(match, rest...)})
	}
	return out, nil
}

type directTargetRef struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type directRepoView struct {
	RepoID  string            `json:"repoId"`
	Name    string            `json:"name"`
	Targets []directTargetRef `json:"targets"`
}

func directRepoViews(findings []directFinding) []directRepoView {
	out := make([]directRepoView, 0, len(findings))
	for _, f := range findings {
		v := directRepoView{RepoID: f.RepoID, Name: f.Name, Targets: make([]directTargetRef, 0, len(f.Candidates))}
		for _, t := range f.Candidates {
			v.Targets = append(v.Targets, directTargetRef{ID: t.ID, Name: placementTargetName(t)})
		}
		out = append(out, v)
	}
	return out
}

// fieldTargets reads the row each domain's off-site settings field edits.
func (s *Service) fieldTargets() map[string]store.OffsiteTarget {
	out := map[string]store.OffsiteTarget{}
	for _, d := range offsiteConfigDomains {
		t, ok, err := s.store.FieldOffsiteTarget(d)
		if err != nil {
			log.Printf("api: settings: could not read the %s field target: %v", d, err) //nolint:gosec // G706: domain is a fixed literal
			continue
		}
		if ok {
			out[d] = t
		}
	}
	return out
}
