package api

import (
	"log"
	"strings"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

// offsiteConfigDomains lists the domains that can have an off-site target.
var offsiteConfigDomains = []string{"containers", "vms", "flash", "config", "files", "zfs"}

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
// When the domain's off-site repo is cleared, the primary is deleted so
// offsiteRepoFor cannot return a stale repo. Other additional targets sort after
// it and are never touched here.
//
// The storage class comes from the cloud credentials. If they cannot be
// decoded it stays empty, which offsiteModeForTarget treats as the global
// class.
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
	if repo == "" {
		if primary != nil {
			return s.store.DeleteOffsiteTarget(primary.ID)
		}
		return nil
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
	if c, cErr := s.decodeCloud(settings); cErr == nil {
		t.StorageClass = c.S3StorageClass
	}
	if primary != nil {
		t.ID = primary.ID
		t.CreatedAt = primary.CreatedAt
		t.CredsRef = primary.CredsRef
	}
	_, err = s.store.UpsertOffsiteTarget(t)
	return err
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

// MoveMeshTargetsOffPrimarySlot moves every target accepted from a mesh offer
// off sort order 0, which a settings save treats as the primary, and returns how
// many it moved. One on the repo the domain's off-site setting names is that
// primary and stays.
func (s *Service) MoveMeshTargetsOffPrimarySlot() (int, error) {
	offers, err := s.store.ListMeshOffers()
	if err != nil {
		return 0, err
	}
	accepted := make(map[string]bool, len(offers))
	for _, o := range offers {
		if o.Status == "accepted" {
			accepted[o.Repo] = true
		}
	}
	settings, err := s.store.GetSettings()
	if err != nil {
		return 0, err
	}
	moved := 0
	for _, d := range offsiteConfigDomains {
		targets, err := s.store.OffsiteTargetsForDomain(d)
		if err != nil {
			return moved, err
		}
		for _, t := range targets {
			if t.SortOrder != 0 || !accepted[t.Repo] || t.Repo == offsiteRepoFromSettings(d, settings) {
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

// offsiteSourcePrefix starts a source that names one off-site target,
// "offsite:<id>". The bare source "offsite" means the domain's primary target.
const offsiteSourcePrefix = "offsite:"

// isOffsiteSource reports whether a browse, restore, delete or prune source
// addresses an off-site repo in either form. Call sites use it instead of
// comparing against "offsite" so the id form works everywhere.
func isOffsiteSource(source string) bool {
	return source == "offsite" || strings.HasPrefix(source, offsiteSourcePrefix)
}

// offsiteTargetIDFromSource returns the id of an "offsite:<id>" source as is,
// or "" for any other source.
func offsiteTargetIDFromSource(source string) string {
	if id, ok := strings.CutPrefix(source, offsiteSourcePrefix); ok {
		return id
	}
	return ""
}

// validOffsiteTargetID reports whether id looks like an id from store.newID
// (lowercase hex), so a malformed query value never ends up in a source. A
// well-formed id that matches no target is harmless: offsiteTargetForSource
// falls back to the primary.
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

// offsiteTargetForSource resolves the off-site target a source addresses in a
// domain:
//   - "offsite" is the primary, the first enabled target
//   - "offsite:<id>" is the enabled target with that id; an unknown id falls
//     back to the primary so a stale id cannot strand a restore
//   - without any target rows, the target is built from the Settings columns
//
// ok is false for a non-offsite source or a domain with no off-site repo.
func (s *Service) offsiteTargetForSource(settings store.Settings, domain, source string) (store.OffsiteTarget, bool) {
	if !isOffsiteSource(source) {
		return store.OffsiteTarget{}, false
	}
	targets := s.offsiteTargetsFor(domain)
	if id := offsiteTargetIDFromSource(source); id != "" {
		for _, t := range targets {
			if t.ID == id {
				return t, true
			}
		}
	}
	if len(targets) > 0 {
		return targets[0], true
	}
	loc := offsiteRepoFromSettings(domain, settings)
	if loc == "" {
		return store.OffsiteTarget{}, false
	}
	return settingsOffsiteTarget(domain, settings, loc), true
}

// offsiteSourceImmutable reports whether the target a source addresses is
// append-only. When no target resolves it falls back to the domain's Settings
// flag.
func (s *Service) offsiteSourceImmutable(settings store.Settings, domain, source string) bool {
	if target, ok := s.offsiteTargetForSource(settings, domain, source); ok {
		return target.Immutable
	}
	return offsiteImmutableFor(domain, settings)
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
