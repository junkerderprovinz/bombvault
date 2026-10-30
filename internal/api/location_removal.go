package api

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/http"
	"path"
	"slices"
	"strings"

	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

var (
	errHomeNeeded = errors.New("say which folder the sections move to")
	errHomeSame   = errors.New("the sections are in that folder already")
)

// homeID names a section's own repository among the live ones.
func homeID(domain string) string { return "home:" + domain }

// standaloneTarget is the off-site target with the id when it is a storage
// location by itself, one that follows no destination.
func (s *Service) standaloneTarget(id string) (store.OffsiteTarget, error) {
	t, ok, err := s.store.GetOffsiteTarget(id)
	if err != nil {
		return store.OffsiteTarget{}, err
	}
	if !ok || t.DestinationID != "" {
		return store.OffsiteTarget{}, errNoSuchLocation
	}
	return t, nil
}

func (s *Service) targetRemoval(id string) (store.OffsiteTarget, locationRemoval, error) {
	t, err := s.standaloneTarget(id)
	if err != nil {
		return store.OffsiteTarget{}, locationRemoval{}, err
	}
	section, err := s.removalSection(t)
	if err != nil {
		return store.OffsiteTarget{}, locationRemoval{}, err
	}
	return t, locationRemoval{Sections: []removalSection{section}}, nil
}

// removeTarget takes away an off-site target that follows no destination,
// after checking that it is the one expect shows. The data there stays.
func (s *Service) removeTarget(id string, expect locationRemoval) (store.Settings, error) {
	t, now, err := s.targetRemoval(id)
	if err != nil {
		return store.Settings{}, err
	}
	if !now.same(expect) {
		return store.Settings{}, &removalChangedErr{now: now}
	}
	if err := s.store.RemoveOffsiteTarget(id, clearOffsiteFields); err != nil {
		return store.Settings{}, err
	}
	s.dropDestinationSignIn(t)
	return s.store.GetSettings()
}

func (s *Service) planTargetErasure(ctx context.Context, id string) (erasurePlan, error) {
	settings, err := s.store.GetSettings()
	if err != nil {
		return erasurePlan{}, fmt.Errorf("read settings: %w", err)
	}
	t, removal, err := s.targetRemoval(id)
	if err != nil {
		return erasurePlan{}, err
	}
	plan, err := s.newErasurePlan(settings, removal, id, "")
	if err != nil {
		return erasurePlan{}, err
	}
	if err := plan.addTarget(ctx, s, settings, t); err != nil {
		return erasurePlan{}, err
	}
	plan.finish(ctx, s, settings)
	return plan, nil
}

// removableRepo is the named repository with the id when it is a storage
// location by itself. A direct repository is listed with its target.
func (s *Service) removableRepo(id string) (store.OffsiteTarget, error) {
	r, err := s.store.GetNamedRepo(id)
	if errors.Is(err, sql.ErrNoRows) || err == nil && r.CompanionOf != "" {
		return store.OffsiteTarget{}, errNoSuchLocation
	}
	return r, err
}

// repoRemoval lists, per section, what backs up to a named repository: the
// entries, which follow their section again once it is gone, and the
// section's default, which falls back to the section's own folder.
func (s *Service) repoRemoval(r store.OffsiteTarget) (locationRemoval, error) {
	items, err := s.store.NamedRepoItems(r.ID)
	if err != nil {
		return locationRemoval{}, err
	}
	use, err := s.namedRepoUse(r.ID)
	if err != nil {
		return locationRemoval{}, err
	}
	out := locationRemoval{Sections: []removalSection{}}
	for _, domain := range offsiteConfigDomains {
		def := slices.Contains(use.DefaultDomains, domain)
		if items[domain] == 0 && !def {
			continue
		}
		out.Sections = append(out.Sections, removalSection{
			Domain: domain, Use: removalRepo, Repo: scrubRepoLocation(r.Repo),
			Items: items[domain], Default: def, AppendOnly: r.Immutable,
		})
	}
	return out, nil
}

// removeNamedRepo takes a named repository away and sends what backed up to
// it back to its section, after checking that this is what expect shows. The
// data there stays.
func (s *Service) removeNamedRepo(id string, expect locationRemoval) (store.Settings, error) {
	r, err := s.removableRepo(id)
	if err != nil {
		return store.Settings{}, err
	}
	now, err := s.repoRemoval(r)
	if err != nil {
		return store.Settings{}, err
	}
	if !now.same(expect) {
		return store.Settings{}, &removalChangedErr{now: now}
	}
	if err := s.store.RemoveNamedRepo(id); err != nil {
		return store.Settings{}, err
	}
	s.dropDestinationSignIn(r)
	return s.store.GetSettings()
}

// planRepoErasure lists what each section keeps in a named repository. A
// section without backups there is left out, unless the repository could not
// be read and the section backs up to it.
func (s *Service) planRepoErasure(ctx context.Context, id string) (erasurePlan, error) {
	settings, err := s.store.GetSettings()
	if err != nil {
		return erasurePlan{}, fmt.Errorf("read settings: %w", err)
	}
	r, err := s.removableRepo(id)
	if err != nil {
		return erasurePlan{}, err
	}
	removal, err := s.repoRemoval(r)
	if err != nil {
		return erasurePlan{}, err
	}
	plan, err := s.newErasurePlan(settings, removal, id, "")
	if err != nil {
		return erasurePlan{}, err
	}
	kept := plan.sharedBy(s, r.Repo, r.ID)
	if kept == "" && r.Immutable {
		kept = erasureKeptAppendOnly
	}
	for _, domain := range offsiteConfigDomains {
		oc, err := plan.ownerContext(s, domain)
		if err != nil {
			return erasurePlan{}, err
		}
		at := len(plan.Sections)
		section := erasureSection{Domain: domain, RepoID: r.ID, Repo: scrubRepoLocation(r.Repo), Kept: kept, Use: removalRepo}
		plan.add(ctx, s, oc, section, erasureRepo{domain: domain}, r.Repo, func(loc string) restic.Mode {
			return s.primaryModeFor(settings, domain, loc)
		})
		backsUp := slices.ContainsFunc(removal.Sections, func(sec removalSection) bool { return sec.Domain == domain })
		if plan.Sections[at].Backups == 0 && (plan.Sections[at].Kept != erasureKeptUnreadable || !backsUp) {
			plan.Sections, plan.repos = plan.Sections[:at], plan.repos[:at]
		}
	}
	plan.finish(ctx, s, settings)
	return plan, nil
}

// sectionHome is a section whose own folder sits in a section folder location.
type sectionHome struct{ domain, raw string }

// sectionHomesAt lists the sections whose own folder sits in the location
// with the key.
func sectionHomesAt(settings store.Settings, key string) []sectionHome {
	var out []sectionHome
	for _, domain := range offsiteConfigDomains {
		raw := strings.TrimSpace(domainPathRaw(domain, settings))
		if raw != "" && repoLocationKey(scrubRepoLocation(pathRoot(raw))) == key {
			out = append(out, sectionHome{domain, raw})
		}
	}
	return out
}

// folderName is the last segment of a section's folder, the part it keeps
// when it moves.
func folderName(raw string) string {
	raw = strings.TrimRight(raw, "/")
	return strings.Trim(strings.TrimPrefix(raw, pathRoot(raw)), "/")
}

// underHome is the section folder named folder in home.
func underHome(home, folder string) string {
	home = strings.TrimRight(strings.TrimSpace(home), "/")
	if strings.HasSuffix(home, ":") {
		return home + folder
	}
	return home + "/" + folder
}

// sectionFolderRemoval lists the sections in a section folder and offers the
// folder they could move to: another section's on this host, or the one a
// fresh install uses.
func (s *Service) sectionFolderRemoval(settings store.Settings, key string) (locationRemoval, error) {
	homes := sectionHomesAt(settings, key)
	if len(homes) == 0 {
		return locationRemoval{}, errNoSuchLocation
	}
	out := locationRemoval{Sections: []removalSection{}}
	for _, h := range homes {
		section := removalSection{Domain: h.domain, Use: removalHome, Repo: scrubRepoLocation(h.raw), Folder: folderName(h.raw)}
		if restic.IsRemoteRepo(h.raw) {
			if row, ok := s.primaryRemoteTarget(h.domain); ok {
				section.AppendOnly = row.Immutable
			}
		}
		out.Sections = append(out.Sections, section)
	}
	for _, domain := range offsiteConfigDomains {
		raw := strings.TrimSpace(domainPathRaw(domain, settings))
		if raw != "" && !restic.IsRemoteRepo(raw) && repoLocationKey(scrubRepoLocation(pathRoot(raw))) != key {
			out.Suggest = pathRoot(raw)
			return out, nil
		}
	}
	if fresh := path.Dir(store.DefaultSectionPath("containers")); repoLocationKey(fresh) != key {
		out.Suggest = fresh
	}
	return out, nil
}

func (s *Service) planSectionFolderErasure(ctx context.Context, key string) (erasurePlan, error) {
	settings, err := s.store.GetSettings()
	if err != nil {
		return erasurePlan{}, fmt.Errorf("read settings: %w", err)
	}
	removal, err := s.sectionFolderRemoval(settings, key)
	if err != nil {
		return erasurePlan{}, err
	}
	plan, err := s.newErasurePlan(settings, removal, key, "")
	if err != nil {
		return erasurePlan{}, err
	}
	for _, h := range sectionHomesAt(settings, key) {
		oc, err := plan.ownerContext(s, h.domain)
		if err != nil {
			return erasurePlan{}, err
		}
		kept := plan.sharedBy(s, h.raw, homeID(h.domain))
		if row, ok := s.primaryRemoteTarget(h.domain); ok && restic.IsRemoteRepo(h.raw) && s.refusesDeletes(row) {
			kept = erasureKeptAppendOnly
		}
		section := erasureSection{Domain: h.domain, Repo: scrubRepoLocation(h.raw), Kept: kept, Use: removalHome}
		plan.add(ctx, s, oc, section, erasureRepo{domain: h.domain}, h.raw, func(loc string) restic.Mode {
			return s.primaryModeFor(settings, h.domain, loc)
		})
	}
	plan.finish(ctx, s, settings)
	return plan, nil
}

// sectionPath is the settings field that holds a section's own folder.
func sectionPath(settings *store.Settings, domain string) *string {
	switch domain {
	case "containers":
		return &settings.ContainersPath
	case "vms":
		return &settings.VMsPath
	case "flash":
		return &settings.FlashPath
	case "config":
		return &settings.ConfigPath
	case "files":
		return &settings.FilesPath
	case zfsDomain:
		return &settings.ZFSPath
	}
	return nil
}

// folderMove is where the sections of a section folder go: each one's folder
// as it is and as it will be, by section.
type folderMove map[string][2]string

// planFolderMove works out where the sections in the section folder with the
// key go under home and checks the new folders the way a settings save does.
func (h *Handler) planFolderMove(key string, expect locationRemoval, home string) (folderMove, error) {
	home = strings.TrimSpace(home)
	if home == "" {
		return nil, errHomeNeeded
	}
	cur, err := h.store.GetSettings()
	if err != nil {
		return nil, err
	}
	now, err := h.svc.sectionFolderRemoval(cur, key)
	if err != nil {
		return nil, err
	}
	if !now.same(expect) {
		return nil, &removalChangedErr{now: now}
	}
	move := folderMove{}
	next := cur
	for _, sh := range sectionHomesAt(cur, key) {
		to := underHome(home, folderName(sh.raw))
		if sameRepoLocation(to, sh.raw) {
			return nil, errHomeSame
		}
		move[sh.domain] = [2]string{sh.raw, to}
		*sectionPath(&next, sh.domain) = to
	}
	if err := h.settingsLocationRefusal(next, cur); err != nil {
		return nil, err
	}
	return move, nil
}

// moveFolders writes the new folders, unless a section's folder changed since
// the move was worked out.
func (h *Handler) moveFolders(move folderMove) (store.Settings, error) {
	return h.store.MutateSettings(func(cur *store.Settings) error {
		for domain, m := range move {
			field := sectionPath(cur, domain)
			if strings.TrimSpace(*field) != m[0] {
				return errPlacementStale
			}
			*field = m[1]
		}
		return nil
	})
}

// removeLocation takes the location with the id away and leaves its data
// where it is. home is where the sections of a section folder move to.
func (h *Handler) removeLocation(ctx context.Context, id string, expect locationRemoval, home string) (store.Settings, error) {
	return h.svc.removeWhileIdle(ctx, expect, func() (store.Settings, error) { return h.removeIdleLocation(id, expect, home) })
}

func (h *Handler) removeIdleLocation(id string, expect locationRemoval, home string) (store.Settings, error) {
	kind, oid, _ := strings.Cut(id, ":")
	switch kind {
	case locationDestination:
		return h.svc.RemoveDestination(oid, expect)
	case locationTarget:
		return h.svc.removeTarget(oid, expect)
	case locationRepo:
		return h.svc.removeNamedRepo(oid, expect)
	case locationPath:
		move, err := h.planFolderMove(oid, expect, home)
		if err != nil {
			return store.Settings{}, err
		}
		return h.moveFolders(move)
	}
	return store.Settings{}, errNoSuchLocation
}

func (s *Service) locationRemovalPreview(id string) (locationRemoval, error) {
	kind, oid, _ := strings.Cut(id, ":")
	switch kind {
	case locationDestination:
		_, removal, err := s.destinationRemovalPreview(oid)
		return removal, err
	case locationTarget:
		_, removal, err := s.targetRemoval(oid)
		return removal, err
	case locationRepo:
		r, err := s.removableRepo(oid)
		if err != nil {
			return locationRemoval{}, err
		}
		return s.repoRemoval(r)
	case locationPath:
		settings, err := s.store.GetSettings()
		if err != nil {
			return locationRemoval{}, err
		}
		return s.sectionFolderRemoval(settings, oid)
	}
	return locationRemoval{}, errNoSuchLocation
}

// erasureJob returns the deletion of the backups at the location with the
// id. A section folder's sections move to home once they are gone, and the
// move is checked before anything is deleted when expect is given.
func (h *Handler) erasureJob(id string, expect *locationRemoval, home string) (erasureJob, error) {
	kind, oid, _ := strings.Cut(id, ":")
	switch kind {
	case locationDestination:
		return h.svc.destinationErasureJob(oid), nil
	case locationTarget:
		return erasureJob{
			id:     oid,
			plan:   func(ctx context.Context) (erasurePlan, error) { return h.svc.planTargetErasure(ctx, oid) },
			remove: func(removal locationRemoval) (store.Settings, error) { return h.svc.removeTarget(oid, removal) },
		}, nil
	case locationRepo:
		return erasureJob{
			id:     oid,
			plan:   func(ctx context.Context) (erasurePlan, error) { return h.svc.planRepoErasure(ctx, oid) },
			remove: func(removal locationRemoval) (store.Settings, error) { return h.svc.removeNamedRepo(oid, removal) },
		}, nil
	case locationPath:
		if expect != nil {
			if _, err := h.planFolderMove(oid, *expect, home); err != nil {
				return erasureJob{}, err
			}
		}
		return erasureJob{
			id:   oid,
			plan: func(ctx context.Context) (erasurePlan, error) { return h.svc.planSectionFolderErasure(ctx, oid) },
			remove: func(removal locationRemoval) (store.Settings, error) {
				move, err := h.planFolderMove(oid, removal, home)
				if err != nil {
					return store.Settings{}, err
				}
				return h.moveFolders(move)
			},
		}, nil
	}
	return erasureJob{}, errNoSuchLocation
}

// removalFail writes a refusal of a removal or an erasure: 404 for a location
// that is not there, the removal or the erasure as it stands for one that
// changed.
func removalFail(w http.ResponseWriter, err error) {
	var changed *removalChangedErr
	var refused *erasureRefused
	switch {
	case errors.Is(err, errNoSuchLocation), errors.Is(err, store.ErrNotDestination):
		writeJSON(w, http.StatusNotFound, map[string]any{"ok": false, "error": err.Error()})
	case errors.As(err, &changed):
		placementFail(w, err, map[string]any{"removal": changed.now})
	case errors.As(err, &refused):
		placementFail(w, err, map[string]any{"erasure": refused.now})
	default:
		placementFail(w, err, nil)
	}
}

// afterRemoval brings the scheduler and the off-site plan in line with the
// settings a removal left: a removed primary target took the section's
// off-site field along, and a section folder moved.
func (h *Handler) afterRemoval(settings store.Settings) error {
	if err := h.scheduler.ReloadWithGates(settings, h.dueGates()); err != nil {
		return err
	}
	h.replanAfterTargetChange()
	return nil
}

// handleLocationRemovalPreview says what removing a storage location detaches
// or moves.
// GET /api/storage/locations/{id}/removal
func (h *Handler) handleLocationRemovalPreview(w http.ResponseWriter, r *http.Request) {
	removal, err := h.svc.locationRemovalPreview(r.PathValue("id"))
	if err != nil {
		removalFail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"removal": removal}))
}

// handleRemoveLocation removes a storage location and leaves the backups
// there. The body carries the preview that was shown and, for a section
// folder, the folder its sections move to.
// POST /api/storage/locations/{id}/removal
func (h *Handler) handleRemoveLocation(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Expect locationRemoval `json:"expect"`
		Home   string          `json:"home"`
	}
	if !decodeBody(w, r, &body) {
		return
	}
	settings, err := h.removeLocation(r.Context(), r.PathValue("id"), body.Expect, body.Home)
	if err != nil {
		removalFail(w, err)
		return
	}
	if err := h.afterRemoval(settings); err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": scrubError(err)})
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(nil))
}

// handleLocationErasurePreview says what deleting the backups at a storage
// location takes away.
// GET /api/storage/locations/{id}/erasure
func (h *Handler) handleLocationErasurePreview(w http.ResponseWriter, r *http.Request) {
	if !h.requireAuthForSecrets(w, r, "listing what deleting the backups at a storage location takes") {
		return
	}
	job, err := h.erasureJob(r.PathValue("id"), nil, "")
	if err != nil {
		removalFail(w, err)
		return
	}
	plan, err := job.plan(r.Context())
	if err != nil {
		removalFail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"erasure": plan.destinationErasure}))
}

// handleEraseLocation starts deleting the backups at a storage location and
// removing it afterwards. The body carries the removal that was shown, the
// identities of the entries that may lose their only backup and, for a
// section folder, the folder its sections move to. Without a login password
// anybody on the network could delete the backups, so it takes one.
// POST /api/storage/locations/{id}/erasure
func (h *Handler) handleEraseLocation(w http.ResponseWriter, r *http.Request) {
	if !h.requireAuthForSecrets(w, r, "deleting the backups at a storage location") {
		return
	}
	var body struct {
		Expect     locationRemoval `json:"expect"`
		LastCopies []string        `json:"lastCopies"`
		Home       string          `json:"home"`
	}
	if !decodeBody(w, r, &body) {
		return
	}
	job, err := h.erasureJob(r.PathValue("id"), &body.Expect, body.Home)
	if err != nil {
		removalFail(w, err)
		return
	}
	removed := func(settings store.Settings) {
		if err := h.afterRemoval(settings); err != nil {
			log.Printf("api: erase storage location: the scheduler did not reload: %v", err)
		}
	}
	erasure, err := h.svc.StartErasure(r.Context(), job, body.Expect, body.LastCopies, removed)
	if err != nil {
		removalFail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"started": true, "erasure": erasure}))
}
