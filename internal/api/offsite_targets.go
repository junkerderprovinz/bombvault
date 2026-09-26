package api

import (
	"errors"
	"log"
	"slices"
	"strings"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

// offsiteConfigDomains lists the domains that can have an off-site target.
var offsiteConfigDomains = []string{"containers", "vms", "flash", "config", "files"}

func validOffsiteDomain(domain string) bool {
	for _, d := range offsiteConfigDomains {
		if d == domain {
			return true
		}
	}
	return false
}

// syncPrimaryOffsiteTarget writes a domain's off-site settings field into the
// target on sort_order 0. A target already on the field's location takes that
// slot, so the domain does not copy there twice, and the row it takes over
// from is switched off behind the others. Otherwise the row on the slot
// follows the field, or failing that a new row is made. Clearing the field
// switches that row off, so filling it again brings back the same target. A
// target at a place is not written, on the slot or on the field's location:
// it only moves onto the slot, and from then on its place writes the field.
func (s *Service) syncPrimaryOffsiteTarget(domain string, settings store.Settings) error {
	if s.store == nil {
		return nil
	}
	primary, ok, err := s.store.FieldOffsiteTarget(domain)
	if err != nil || (ok && primary.PlaceID != "") {
		return err
	}
	repo := offsiteRepoFromSettings(domain, settings)
	if repo == "" {
		if !ok || !primary.Enabled {
			return nil
		}
		primary.Enabled = false
		_, err := s.store.UpsertOffsiteTarget(primary)
		return err
	}
	if !ok || primary.Repo != repo {
		targets, err := s.store.OffsiteTargetsForDomain(domain)
		if err != nil {
			return err
		}
		onRepo := func(t store.OffsiteTarget) bool { return t.Repo == repo }
		unplaced := func(t store.OffsiteTarget) bool { return onRepo(t) && t.PlaceID == "" }
		i := slices.IndexFunc(targets, unplaced)
		if i < 0 {
			i = slices.IndexFunc(targets, onRepo)
		}
		if i >= 0 {
			if ok {
				// The field leaves this row's location, so it stops copying there.
				primary.Enabled = false
				if _, err := s.store.UpsertOffsiteTarget(primary); err != nil {
					return err
				}
				if err := s.store.MoveOffsiteTargetBehind(primary.ID); err != nil {
					return err
				}
			}
			if err := s.store.MakeFieldOffsiteTarget(targets[i].ID); err != nil {
				return err
			}
			if targets[i].PlaceID != "" {
				return nil
			}
			primary, ok = targets[i], true
		}
	}
	t := settingsOffsiteTarget(domain, settings, repo)
	// Unreadable cloud credentials leave the class empty, which
	// offsiteModeForTarget reads as the global class.
	if c, cErr := s.decodeCloud(settings); cErr == nil {
		t.StorageClass = c.S3StorageClass
	}
	if ok {
		t.ID = primary.ID
		t.CreatedAt = primary.CreatedAt
		t.CredsRef = primary.CredsRef
	}
	_, err = s.store.UpsertOffsiteTarget(t)
	return err
}

// MoveMeshTargetsOffPrimarySlot moves every target accepted from a mesh offer
// off sort order 0, where a settings save would take it for the row the
// domain's off-site field edits, and returns how many it moved. One on the
// location the field names is that row and stays, and so does one at a place,
// which a settings save leaves alone.
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
			if t.SortOrder != 0 || t.PlaceID != "" || !accepted[t.Repo] || t.Repo == offsiteRepoFromSettings(d, settings) {
				continue
			}
			if err := s.store.MoveOffsiteTargetBehind(t.ID); err != nil {
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
// "offsite" is the first enabled target; "offsite:<id>" is that row among all of
// the domain's targets, switched off or not, and nothing else.
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
	if targets := s.offsiteTargetsFor(domain); len(targets) > 0 {
		return targets[0], nil
	}
	// No target row: the one target the settings columns describe.
	loc := offsiteRepoFromSettings(domain, settings)
	if loc == "" {
		return store.OffsiteTarget{}, errNoOffsiteRepo
	}
	return settingsOffsiteTarget(domain, settings, loc), nil
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
