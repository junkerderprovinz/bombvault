package api

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/http"
	"slices"

	"github.com/junkerderprovinz/bombvault/internal/backup"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// ErrNoPlace refuses a backup whose section has every storage location
// switched off for it.
var ErrNoPlace = errors.New("every storage location of this section is switched off, so there is nowhere to back it up to")

// codeNoPlace is the code a refusal by ErrNoPlace carries, which the page
// turns into a sentence in its language.
const codeNoPlace = "no-place"

// The codes of a start refused because another run holds the domain, and
// because another batch holds the server.
const (
	codeDomainBusy = "domain-busy"
	codeBatchBusy  = "batch-busy"
)

// startRefusal is the body of the 409 for an action pressed by hand that did
// not start.
func startRefusal(err error) map[string]any {
	body := map[string]any{"ok": false, "error": err.Error()}
	if errors.Is(err, ErrNoPlace) {
		body["code"] = codeNoPlace
	}
	if errors.Is(err, ErrNoLocation) {
		body["code"] = codeNoLocation
	}
	if refusedAsBusy(err) {
		body["code"] = codeDomainBusy
	}
	return body
}

// batchBusyRefusal is the body of the 409 for a batch refused while another
// one runs.
func batchBusyRefusal() map[string]any {
	return map[string]any{"ok": false, "error": "a batch backup is already running", "code": codeBatchBusy}
}

// refusedAsBusy reports whether a start was refused because another
// operation holds its domain.
func refusedAsBusy(err error) bool {
	_, busy := errors.AsType[domainBusyError](err)
	return busy || errors.Is(err, errDomainBusy)
}

// localSwitchDomain reports whether a section's own folder can be switched
// off: the sections without placement, whose backups then go straight to
// their off-site targets.
func localSwitchDomain(domain string) bool {
	return slices.Contains(store.LocalSwitchDomains, domain)
}

func (s *Service) localOff(domain string) (bool, error) {
	if !localSwitchDomain(domain) {
		return false, nil
	}
	off, err := s.store.LocalOff()
	return off[domain], err
}

// sectionPlace is one repository a backup of a section without placement is
// written to. target is nil for the section's own folder.
type sectionPlace struct {
	repo   string
	mode   restic.Mode
	target *store.OffsiteTarget
}

func (pl sectionPlace) own() bool { return pl.target == nil }

// sectionPlaces is where a backup of a section without placement goes: the
// repository in its own folder, own, or while that folder is switched off the
// repository of every target of the section that is switched on, each opened
// with that target's credentials. A section with neither has no place.
func (s *Service) sectionPlaces(settings store.Settings, domain, own string) ([]sectionPlace, error) {
	off, err := s.localOff(domain)
	if err != nil {
		return nil, err
	}
	if !off {
		return []sectionPlace{{repo: own, mode: s.primaryModeFor(settings, domain, own)}}, nil
	}
	targets, err := s.store.OffsiteTargetsForDomain(domain)
	if err != nil {
		return nil, err
	}
	var places []sectionPlace
	for _, t := range targets {
		if !t.Enabled {
			continue
		}
		repo, err := s.resolveRepo(t.Repo)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", placementTargetName(t), err)
		}
		mode := s.offsiteModeForTarget(settings, t)
		mode.Limits = targetOffsiteLimits(t)
		places = append(places, sectionPlace{repo: repo, mode: mode, target: &t})
	}
	if len(places) == 0 {
		return nil, ErrNoPlace
	}
	return places, nil
}

// readyPlaces creates the repositories that do not exist yet. Only the own
// folder has its stale locks cleared up front: a remote one bills every call,
// so its locks are cleared when one stops the backup.
func (s *Service) readyPlaces(ctx context.Context, places []sectionPlace) error {
	for _, pl := range places {
		if err := s.EnsureRepo(ctx, pl.repo, pl.mode); err != nil {
			return err
		}
		if pl.own() {
			s.unlockStale(ctx, pl.repo, pl.mode)
		}
	}
	return nil
}

// placesAdapter writes each backup to every place of a section in turn. The
// first place's summary is the run's. A place that fails fails the run, and
// the places before it keep what they took.
type placesAdapter struct {
	places   []sectionPlace
	adapters []*resticAdapter
}

func (s *Service) placesAdapter(settings store.Settings, domain string, places []sectionPlace, selectionFP string) placesAdapter {
	a := placesAdapter{places: places}
	for _, pl := range places {
		engine := s.engine
		if !pl.own() {
			engine = unlockOnLockErr{ResticEngine: s.engine, s: s}
		}
		a.adapters = append(a.adapters, &resticAdapter{
			engine: engine, mode: pl.mode, selectionFP: selectionFP, extraTags: s.directTags(settings, domain, pl.repo),
		})
	}
	return a
}

func (a placesAdapter) Backup(ctx context.Context, repo string, paths, tags []string, excludes ...string) (backup.Summary, error) {
	sum, err := a.adapters[0].Backup(ctx, repo, paths, tags, excludes...)
	if err != nil {
		return sum, err
	}
	for i, pl := range a.places[1:] {
		if _, err := a.adapters[i+1].Backup(ctx, pl.repo, paths, tags, excludes...); err != nil {
			return sum, fmt.Errorf("%s: %w", placementTargetName(*pl.target), err)
		}
	}
	return sum, nil
}

func (a placesAdapter) BackupDir(ctx context.Context, repo, dir string, tags []string, excludes ...string) (backup.ZFSBackupSummary, error) {
	sum, err := a.adapters[0].BackupDir(ctx, repo, dir, tags, excludes...)
	if err != nil {
		return sum, err
	}
	for i, pl := range a.places[1:] {
		if _, err := a.adapters[i+1].BackupDir(ctx, pl.repo, dir, tags, excludes...); err != nil {
			return sum, fmt.Errorf("%s: %w", placementTargetName(*pl.target), err)
		}
	}
	return sum, nil
}

// keepAt is the keep-policy a place ages by and whether it refuses deletes:
// the section's own for its folder, the target's for a target.
func (s *Service) keepAt(settings store.Settings, domain string, pl sectionPlace) (restic.RetentionPolicy, bool) {
	if !pl.own() {
		return targetOffsiteRetentionPolicy(*pl.target), pl.target.Immutable
	}
	p := s.retentionPolicyForRef(settings, domain, s.refFor(settings, domain, pl.repo))
	return p, p.Any() && s.primaryIsImmutable(domain, pl.repo)
}

// skipOwnCopySource leaves the section's own folder out of a copy while that
// folder is switched off: the section's backups go to its targets directly,
// and what the folder already holds stays there. done is true when nothing
// is left to copy.
func (s *Service) skipOwnCopySource(domain string, sources []domainRepoRef) (rest []domainRepoRef, done bool) {
	off, err := s.localOff(domain)
	if err != nil {
		log.Printf("api: offsite %s: whether its own folder is switched off could not be read, so it is copied as before: %v", domain, err) //nolint:gosec // G706: domain is a fixed literal
		return sources, false
	}
	if !off {
		return sources, false
	}
	rest = slices.DeleteFunc(slices.Clone(sources), func(r domainRepoRef) bool { return r.Own })
	return rest, len(rest) == 0
}

// placeless names the sections in which an item that takes the section's
// place has none: a default that gives new items no place, or an own folder
// that is switched off while no target of the section is switched on.
type placeless map[string]bool

func (s *Service) placelessSections() (placeless, error) {
	out := placeless{}
	defaults, err := s.store.ListPlacementDefaults()
	if err != nil {
		return nil, err
	}
	for _, d := range defaults {
		if d.Home == store.HomeNowhere {
			out[d.Domain] = true
		}
	}
	off, err := s.store.LocalOff()
	if err != nil {
		return nil, err
	}
	for domain := range off {
		targets, err := s.store.OffsiteTargetsForDomain(domain)
		if err != nil {
			return nil, err
		}
		if !slices.ContainsFunc(targets, func(t store.OffsiteTarget) bool { return t.Enabled }) {
			out[domain] = true
		}
	}
	return out, nil
}

// takesSectionPlace reports whether the entry key of domain backs up where
// its section sends new backups. key is a container or VM name, the id of a
// folder set or ZFS entry, and unused for flash and config. An entry without
// a row yet is open.
func (s *Service) takesSectionPlace(domain, key string) (bool, error) {
	var choice store.RepoChoice
	switch domain {
	case "flash", "config":
		return true, nil
	case zfsDomain:
		d, err := s.store.GetZFSDataset(key)
		return err == nil && zfsTakesSectionPlace(d), err
	case "containers":
		tg, err := s.store.GetTargetByContainer(key)
		if errors.Is(err, sql.ErrNoRows) {
			return true, nil
		}
		if err != nil {
			return false, err
		}
		choice = tg.RepoChosen
	case "vms":
		vm, err := s.store.GetVMTargetByName(key)
		if errors.Is(err, sql.ErrNoRows) {
			return true, nil
		}
		if err != nil {
			return false, err
		}
		choice = vm.RepoChosen
	case "files":
		set, err := s.store.GetFileSet(key)
		if err != nil {
			return false, err
		}
		choice = set.RepoChosen
	}
	return choice == store.RepoOpen, nil
}

// refuseNoPlace answers a backup pressed by hand for an entry whose section
// has nowhere to put it with ErrNoPlace at once. The run itself starts in the
// background, where the refusal would only reach the log.
func (s *Service) refuseNoPlace(domain, key string) error {
	takes, err := s.takesSectionPlace(domain, key)
	if err != nil || !takes {
		return nil //nolint:nilerr // the backup itself reports an entry it cannot read
	}
	if settings, sErr := s.store.GetSettings(); sErr == nil && localSwitchDomain(domain) {
		if err := s.requireOwnLocation(settings, domain); err != nil {
			return err
		}
	}
	nowhere, err := s.placelessSections()
	if err != nil {
		return nil //nolint:nilerr // see above
	}
	if nowhere[domain] {
		return ErrNoPlace
	}
	return nil
}

// handleSetSectionLocal serves PUT /api/storage/sections/{domain}/local:
// switches the own folder of a section without placement on or off for new
// backups. Nothing is moved or deleted.
func (h *Handler) handleSetSectionLocal(w http.ResponseWriter, r *http.Request) {
	domain := r.PathValue("domain")
	if !localSwitchDomain(domain) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "unknown domain"})
		return
	}
	var body struct {
		On bool `json:"on"`
	}
	if !decodeBody(w, r, &body) {
		return
	}
	if err := h.store.SetLocal(domain, body.On); err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(nil))
}

// SetZFSReplicaOnly switches every place of the ZFS section off, so that its
// entries with a replica only send that, or brings back the places it turned
// off. Each place is remembered before it goes off, so a run that stops
// halfway still restores what it took.
func (s *Service) SetZFSReplicaOnly(on bool) error {
	if on {
		return s.zfsPlacesOff()
	}
	return s.zfsPlacesBack()
}

func (s *Service) zfsPlacesOff() error {
	off, err := s.localOff(zfsDomain)
	if err != nil {
		return err
	}
	if !off {
		if err := s.store.SetReplicaOnlyOff(zfsDomain, "", true); err != nil {
			return err
		}
		if err := s.store.SetLocal(zfsDomain, false); err != nil {
			return err
		}
	}
	targets, err := s.store.OffsiteTargetsForDomain(zfsDomain)
	if err != nil {
		return err
	}
	for _, t := range targets {
		if !t.Enabled {
			continue
		}
		if err := s.store.SetReplicaOnlyOff(zfsDomain, t.ID, true); err != nil {
			return err
		}
		if err := s.setTargetEnabled(t, false); err != nil {
			return err
		}
	}
	return nil
}

// zfsPlacesBack switches on the places zfsPlacesOff turned off. A section
// that has none afterwards, because they were switched off by hand or their
// targets are gone, gets its own folder back.
func (s *Service) zfsPlacesBack() error {
	ids, err := s.store.ReplicaOnlyOff(zfsDomain)
	if err != nil {
		return err
	}
	for _, id := range ids {
		if id == "" {
			err = s.store.SetLocal(zfsDomain, true)
		} else {
			err = s.enableTargetByID(id)
		}
		if err != nil {
			return err
		}
		if err := s.store.SetReplicaOnlyOff(zfsDomain, id, false); err != nil {
			return err
		}
	}
	nowhere, err := s.placelessSections()
	if err != nil {
		return err
	}
	if nowhere[zfsDomain] {
		return s.store.SetLocal(zfsDomain, true)
	}
	return nil
}

func (s *Service) enableTargetByID(id string) error {
	t, ok, err := s.store.GetOffsiteTarget(id)
	if err != nil || !ok || t.Enabled {
		return err
	}
	return s.setTargetEnabled(t, true)
}

// setTargetEnabled switches a target on or off as handleUpdateOffsiteTarget
// does, keeping the primary's switch apart from the settings that rewrite it.
func (s *Service) setTargetEnabled(t store.OffsiteTarget, on bool) error {
	t.Enabled = on
	stored, err := s.store.UpsertOffsiteTarget(t)
	if err != nil {
		return err
	}
	if stored.SortOrder == 0 {
		return s.store.SetPrimaryOff(stored.Domain, stored.Repo, !on)
	}
	return nil
}

// handleSetZFSReplicaOnly serves PUT /api/zfs/replica-only.
func (h *Handler) handleSetZFSReplicaOnly(w http.ResponseWriter, r *http.Request) {
	var body struct {
		On bool `json:"on"`
	}
	if !decodeBody(w, r, &body) {
		return
	}
	err := h.svc.SetZFSReplicaOnly(body.On)
	// A switch that stopped halfway has still changed targets.
	h.replanAfterTargetChange()
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(nil))
}
