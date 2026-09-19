package api

import (
	"errors"
	"log"
	"strings"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

// offsiteConfigDomains is the set of domains that can have an off-site
// destination — the same whitelist every off-site handler validates against.
var offsiteConfigDomains = []string{"containers", "vms", "flash", "config", "files"}

// validOffsiteDomain reports whether domain is one of the five off-site-capable
// domains (the CRUD/setter whitelist).
func validOffsiteDomain(domain string) bool {
	for _, d := range offsiteConfigDomains {
		if d == domain {
			return true
		}
	}
	return false
}

// syncPrimaryOffsiteTarget writes a domain's off-site settings field into the
// target on sort_order 0. Clearing the field switches that row off, so filling
// it again brings back the same target.
func (s *Service) syncPrimaryOffsiteTarget(domain string, settings store.Settings) error {
	if s.store == nil {
		return nil
	}
	primary, ok, err := s.store.FieldOffsiteTarget(domain)
	if err != nil {
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

// syncAllPrimaryOffsiteTargets reconciles every domain's primary off-site target
// with the given Settings. Best-effort per domain: a failure is logged and does
// not abort the others (Settings remain the source of truth for the fallback
// path, so a sync miss degrades to the legacy read, not data loss). Called after
// any write that changes off-site config: the settings save and the cloud-creds
// save (which changes the storage class).
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

// primaryOffsiteTarget returns the first ENABLED off-site target for a domain in
// the store's per-domain order (sort_order, then created_at), or ok=false when
// the domain has no enabled off-site target.
//
// Stage 1: this helper is dormant. Nothing in the live replication path calls it
// yet — offsiteRepoFor and copyToOffsite still read the single-repo Settings
// columns, so behavior is unchanged. Stage 2 rewires callers onto this. It is
// proven correct by a unit test in the meantime.
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
