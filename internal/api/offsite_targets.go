package api

import (
	"errors"
	"fmt"
	"log"
	"slices"
	"strings"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

// offsiteConfigDomains lists the domains that can have an off-site target.
var offsiteConfigDomains = []string{"containers", "vms", "flash", "config", "files", "zfs"}

var invalidOffsiteDomain = "invalid domain: must be one of " + strings.Join(offsiteConfigDomains, ", ")

func validOffsiteDomain(domain string) bool {
	for _, d := range offsiteConfigDomains {
		if d == domain {
			return true
		}
	}
	return false
}

// syncPrimaryOffsiteTarget brings a domain's primary off-site target, the row
// with sort order 0, in line with the off-site columns in settings.
// Replication reads the target rows, while the Settings setters write those
// columns.
//
// An existing primary keeps its id, creation time and credential set. Without
// one, an additional target already on the repo becomes the primary, so the
// domain does not replicate there twice, and failing that a new row is created.
// When the domain's off-site repo is cleared, the primary is switched off
// rather than deleted: its copy rules, observations and direct repository name
// it by id, and filling the field again brings back the same target. Other
// additional targets sort after it and are never touched here. A primary
// switched off by hand stays off while the field names the place it had.
//
// The storage class comes from the cloud credentials. If they cannot be
// decoded it stays empty, which offsiteModeForTarget treats as the global
// class. A primary that follows a destination takes name, storage class,
// append-only flag and provider from it instead, and stops following it once
// the field names another location. A cleared field leaves it switched off
// on its destination, which still lists it and takes it along on removal. The
// keep-policy is set on the storage location's page, so an existing primary
// keeps its own; the off-site settings only seed a new one.
func (s *Service) syncPrimaryOffsiteTarget(domain string, settings store.Settings) error {
	if s.store == nil {
		return nil
	}
	targets, err := s.store.OffsiteTargetsForDomain(domain)
	if err != nil {
		return err
	}
	var primary *store.OffsiteTarget
	for i := range targets {
		if targets[i].SortOrder == 0 {
			primary = &targets[i]
			break
		}
	}

	repo := offsiteRepoFromSettings(domain, settings)
	if primary != nil && primary.DestinationID != "" && repo != "" && primary.Repo != repo {
		if err := s.store.DetachFromDestination(primary.ID); err != nil {
			return err
		}
		primary.DestinationID = ""
	}
	if repo == "" {
		if err := s.store.SetPrimaryOff(domain, "", false); err != nil {
			return err
		}
		if primary == nil || !primary.Enabled {
			return nil
		}
		primary.Enabled = false
		_, err := s.store.UpsertOffsiteTarget(*primary)
		return err
	}
	if primary == nil {
		for i := range targets {
			if targets[i].Repo == repo {
				primary = &targets[i]
				break
			}
		}
	}

	t := settingsOffsiteTarget(domain, settings, repo)
	offAt, err := s.store.PrimaryOffAt(domain)
	if err != nil {
		return err
	}
	t.Enabled = offAt != repo
	// Unreadable cloud credentials leave the class empty, which
	// offsiteModeForTarget reads as the global class.
	if c, cErr := s.decodeCloud(settings); cErr == nil {
		t.StorageClass = c.S3StorageClass
	}
	if primary != nil {
		t.ID = primary.ID
		t.CreatedAt = primary.CreatedAt
		t.CredsRef = primary.CredsRef
		t.Own = primary.Own
		t.TakeFrom(*primary, store.OwnRetention)
		if primary.DestinationID != "" {
			t.StorageClass, t.Immutable, t.Provider = primary.StorageClass, primary.Immutable, primary.Provider
		}
	}
	if primary != nil && primary.Repo == repo && primary.Name != "" {
		t.Name = primary.Name
	} else if t.Name, err = s.freeTargetName(locationName(repo), t.ID); err != nil {
		return err
	}
	if primary != nil && !sameRepoLocation(primary.Repo, repo) {
		_, err = s.store.MoveOffsiteTarget(t)
	} else {
		_, err = s.store.UpsertOffsiteTarget(t)
	}
	return err
}

// locationName is what a target is called before anyone names it: the last
// folder of its location, or the server of an address that has none.
func locationName(repo string) string {
	loc := strings.TrimRight(scrubRepoLocation(repo), "/:")
	host := 0
	if i := strings.Index(loc, "://"); i >= 0 {
		host = i + len("://")
	}
	if i := strings.LastIndexByte(loc[host:], '/'); i >= 0 {
		return loc[host+i+1:]
	}
	if host > 0 {
		server := loc[host:]
		return server[strings.LastIndexByte(server, '@')+1:]
	}
	return loc[strings.LastIndexByte(loc, ':')+1:]
}

// freeTargetName is name, numbered from 2 on while another target or
// destination goes by it, since the storage pages list them all side by side.
// The target with id self does not count against its own name.
func (s *Service) freeTargetName(name, self string) (string, error) {
	targets, err := s.store.ListOffsiteTargets()
	if err != nil {
		return "", err
	}
	destinations, err := s.store.ListDestinations()
	if err != nil {
		return "", err
	}
	taken := map[string]bool{}
	for _, t := range append(targets, destinations...) {
		if t.ID != self {
			taken[strings.ToLower(t.Name)] = true
		}
	}
	free := name
	for n := 2; taken[strings.ToLower(free)]; n++ {
		free = fmt.Sprintf("%s %d", name, n)
	}
	return free, nil
}

// linkedPrimaries returns, by domain, each primary off-site target that
// follows a destination.
func (s *Service) linkedPrimaries() (map[string]store.OffsiteTarget, error) {
	out := map[string]store.OffsiteTarget{}
	for _, d := range offsiteConfigDomains {
		t, ok, err := s.store.FieldOffsiteTarget(d)
		if err != nil {
			return nil, err
		}
		if ok && t.DestinationID != "" {
			out[d] = t
		}
	}
	return out, nil
}

// pinLinkedPrimaries gives the off-site settings of each domain whose field
// still names a primary that follows a destination the append-only flag of
// that destination. The flag decides on pruning, so a stale form must not
// switch it off behind the destination's back.
func pinLinkedPrimaries(settings *store.Settings, linked map[string]store.OffsiteTarget) {
	for d, t := range linked {
		if offsiteRepoFromSettings(d, *settings) == t.Repo {
			setOffsiteImmutableInSettings(settings, d, t.Immutable)
		}
	}
}

// settleLinkedPrimaries pins the stored off-site settings to the destinations
// their primaries follow, after a destination's append-only flag changed.
func (s *Service) settleLinkedPrimaries() (store.Settings, error) {
	linked, err := s.linkedPrimaries()
	if err != nil {
		return store.Settings{}, err
	}
	return s.store.MutateSettings(func(st *store.Settings) error {
		pinLinkedPrimaries(st, linked)
		return nil
	})
}

// rejectAdoptionOverOwnSettings refuses off-site settings under which
// syncPrimaryOffsiteTarget would take over an additional target as a domain's
// primary and replace settings the user gave that target, such as its
// append-only flag. It returns a user-facing sentence, or "".
func (s *Service) rejectAdoptionOverOwnSettings(settings store.Settings) (string, error) {
	for _, d := range offsiteConfigDomains {
		repo := offsiteRepoFromSettings(d, settings)
		if repo == "" {
			continue
		}
		targets, err := s.store.OffsiteTargetsForDomain(d)
		if err != nil {
			return "", err
		}
		if slices.ContainsFunc(targets, func(t store.OffsiteTarget) bool { return t.SortOrder == 0 }) {
			continue
		}
		i := slices.IndexFunc(targets, func(t store.OffsiteTarget) bool { return t.Repo == repo })
		if i < 0 {
			continue
		}
		if targets[i].DestinationID != "" {
			return fmt.Sprintf("the %s off-site repository is the target %q, which follows a destination: leave the field empty", d, targets[i].Name), nil
		}
		if sameOffsitePolicy(targets[i], settingsOffsiteTarget(d, settings, repo)) {
			continue
		}
		return fmt.Sprintf("the %s off-site repository is the additional target %q, which has settings of its own: remove that target or give it the off-site settings first", d, targets[i].Name), nil
	}
	return "", nil
}

// sameOffsitePolicy reports whether a and b replicate and limit the same way.
// The keep-policy does not count: a target taken over as the primary keeps its
// own.
func sameOffsitePolicy(a, b store.OffsiteTarget) bool {
	return a.Immutable == b.Immutable && a.Schedule == b.Schedule &&
		storedCompression(a.Compression) == storedCompression(b.Compression) &&
		a.LimitUpload == b.LimitUpload && a.LimitDownload == b.LimitDownload &&
		a.GrowthBudgetGB == b.GrowthBudgetGB
}

// nextOffsiteSortOrder returns a sort order after every target of domain, so a
// new or moved target stays off the primary's 0.
func (s *Service) nextOffsiteSortOrder(domain string) (int, error) {
	targets, err := s.store.OffsiteTargetsForDomain(domain)
	if err != nil {
		return 0, err
	}
	next := 1
	for _, t := range targets {
		next = max(next, t.SortOrder+1)
	}
	return next, nil
}

// MoveTargetsOffPrimarySlot runs moveTargetsOffPrimarySlot against the stored
// settings at startup, before a settings save can take a stray row for the
// primary.
func (s *Service) MoveTargetsOffPrimarySlot() (int, error) {
	settings, err := s.store.GetSettings()
	if err != nil {
		return 0, err
	}
	return s.moveTargetsOffPrimarySlot(settings)
}

// moveTargetsOffPrimarySlot moves every target but the primary off sort order
// 0, which a settings save treats as the primary, and returns how many it
// moved. The primary is the row on the repo the domain's off-site setting
// names, or the oldest row at 0 when none is. A domain without the setting
// keeps the oldest switched-off row at 0, the one a cleared field leaves behind
// for the next fill.
func (s *Service) moveTargetsOffPrimarySlot(settings store.Settings) (int, error) {
	moved := 0
	for _, d := range offsiteConfigDomains {
		targets, err := s.store.OffsiteTargetsForDomain(d)
		if err != nil {
			return moved, err
		}
		repo := offsiteRepoFromSettings(d, settings)
		var slot []store.OffsiteTarget
		keep := -1
		for _, t := range targets {
			if t.SortOrder != 0 {
				continue
			}
			if keep < 0 && (repo != "" && t.Repo == repo || repo == "" && !t.Enabled) {
				keep = len(slot)
			}
			slot = append(slot, t)
		}
		if keep < 0 && repo != "" && len(slot) > 0 {
			keep = 0
		}
		for i, t := range slot {
			if i == keep {
				continue
			}
			if t.SortOrder, err = s.nextOffsiteSortOrder(d); err != nil {
				return moved, err
			}
			if _, err := s.store.UpsertOffsiteTarget(t); err != nil {
				return moved, err
			}
			moved++
		}
	}
	return moved, nil
}

// syncAllPrimaryOffsiteTargets runs syncPrimaryOffsiteTarget for every domain
// after the settings or the cloud credentials are saved. A failure is logged
// and does not stop the other domains.
func (s *Service) syncAllPrimaryOffsiteTargets(settings store.Settings) {
	for _, d := range offsiteConfigDomains {
		if err := s.syncPrimaryOffsiteTarget(d, settings); err != nil {
			log.Printf("api: sync primary offsite target %s failed: %v", d, err) //nolint:gosec // G706: domain is a fixed literal
		}
	}
}

// offsiteSourcePrefix starts a source that names one off-site target by id. The
// bare source "offsite" names the domain's first enabled target.
const offsiteSourcePrefix = "offsite:"

var (
	errNoOffsiteRepo        = errors.New("no off-site repo configured for this domain")
	errOffsiteTargetsOff    = errors.New("every off-site target of this domain is switched off")
	errUnknownOffsiteTarget = errors.New("no such off-site target in this domain")
)

// isOffsiteSource reports whether a browse, restore, delete or prune source
// addresses an off-site repo, in either form.
func isOffsiteSource(source string) bool {
	return source == "offsite" || strings.HasPrefix(source, offsiteSourcePrefix)
}

// offsiteTargetIDFromSource returns the id an "offsite:<id>" source carries, or
// "" for any other source. Callers that branch on the id resolve the source
// with offsiteTargetForSource first.
func offsiteTargetIDFromSource(source string) string {
	if id, ok := strings.CutPrefix(source, offsiteSourcePrefix); ok {
		return id
	}
	return ""
}

// validOffsiteTargetID reports whether id has the shape store.newID mints: up
// to 64 lowercase hex characters. It keeps a malformed token out of a source
// string; whether the id names a target is offsiteTargetForSource's question.
func validOffsiteTargetID(id string) bool {
	if len(id) == 0 || len(id) > 64 {
		return false
	}
	for _, c := range id {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}

// offsiteTargetForSource resolves the destination a source addresses. Bare
// "offsite" is the first enabled target, or the one the off-site setting
// describes while the domain has no target row; "offsite:<id>" is that row
// among all of the domain's targets, switched off or not, and nothing else.
func (s *Service) offsiteTargetForSource(settings store.Settings, domain, source string) (store.OffsiteTarget, error) {
	if !isOffsiteSource(source) {
		return store.OffsiteTarget{}, errNoOffsiteRepo
	}
	if source != "offsite" {
		t, ok, err := s.store.GetOffsiteTarget(offsiteTargetIDFromSource(source))
		if err != nil {
			return store.OffsiteTarget{}, err
		}
		if !ok || t.Domain != domain {
			return store.OffsiteTarget{}, errUnknownOffsiteTarget
		}
		return t, nil
	}
	if targets := s.offsiteReplicationTargets(domain, settings); len(targets) > 0 {
		return targets[0], nil
	}
	return store.OffsiteTarget{}, s.noOffsiteRepoErr(domain)
}

// offsiteSourceImmutable reports whether the destination a source addresses is
// flagged append-only. A source that resolves to no target is an error, so a
// delete path refuses instead of treating it as writable.
func (s *Service) offsiteSourceImmutable(settings store.Settings, domain, source string) (bool, error) {
	t, err := s.offsiteTargetForSource(settings, domain, source)
	if err != nil {
		return false, err
	}
	return t.Immutable, nil
}

// primaryOffsiteTarget returns the domain's first enabled off-site target in
// store order.
func (s *Service) primaryOffsiteTarget(domain string) (store.OffsiteTarget, bool) {
	targets, err := s.store.OffsiteTargetsForDomain(domain)
	if err != nil {
		return store.OffsiteTarget{}, false
	}
	for _, t := range targets {
		if t.Enabled {
			return t, true
		}
	}
	return store.OffsiteTarget{}, false
}
