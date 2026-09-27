package api

import (
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
		if i < 0 || sameOffsitePolicy(targets[i], settingsOffsiteTarget(d, settings, repo)) {
			continue
		}
		return fmt.Sprintf("the %s off-site repository is the additional target %q, which has settings of its own: remove that target or give it the off-site settings first", d, targets[i].Name), nil
	}
	return "", nil
}

// sameOffsitePolicy reports whether a and b replicate, age and limit the same way.
func sameOffsitePolicy(a, b store.OffsiteTarget) bool {
	return a.Immutable == b.Immutable && a.Schedule == b.Schedule &&
		a.RetentionKeepLast == b.RetentionKeepLast && a.RetentionKeepDaily == b.RetentionKeepDaily &&
		a.RetentionKeepWeekly == b.RetentionKeepWeekly && a.RetentionKeepMonthly == b.RetentionKeepMonthly &&
		a.RetentionKeepYearly == b.RetentionKeepYearly &&
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
// names, or the oldest row at 0 when none is; a domain without the setting has
// none.
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
			if keep < 0 && repo != "" && t.Repo == repo {
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
