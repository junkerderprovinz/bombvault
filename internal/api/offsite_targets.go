package api

import (
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

// syncPrimaryOffsiteTarget reconciles a domain's PRIMARY off-site target row with
// the current Settings columns (+ the decoded cloud storage class), so that
// editing off-site config through the legacy Settings setters keeps working now
// that the replication path reads the offsite_targets rows instead of Settings.
//
// The PRIMARY target is the domain's first row in per-domain order (the shape the
// stage-1 backfill produced: name "Primary", sort_order 0). When it exists, its
// identity (id/created_at/sort_order/creds_ref) is preserved and only the mutable
// config is rewritten from Settings — so an N=1 install stays a single, same-id
// target. When it does not exist yet (a post-backfill install configured only via
// Settings) a fresh one is created. When the domain's off-site repo has been
// CLEARED, the primary row is DELETED so offsiteRepoFor falls back to the (now
// empty) Settings column instead of a stale repo.
//
// The storage class is copied from the shared cloud creds (best-effort: a decode
// failure leaves it empty, which offsiteModeForTarget treats as "use the global
// class" — identical replication behavior). This finally populates the primary
// target's storage_class, which the pure-SQL backfill could not.
func (s *Service) syncPrimaryOffsiteTarget(domain string, settings store.Settings) error {
	if s.store == nil {
		return nil
	}
	targets, err := s.store.OffsiteTargetsForDomain(domain)
	if err != nil {
		return err
	}
	var primary *store.OffsiteTarget
	if len(targets) > 0 {
		primary = &targets[0] // first in (sort_order, created_at) order
	}

	repo := offsiteRepoFromSettings(domain, settings)
	if repo == "" {
		// Off-site cleared for this domain: drop the primary so offsiteRepoFor
		// resolves back to the empty Settings column rather than a stale target.
		if primary != nil {
			return s.store.DeleteOffsiteTarget(primary.ID)
		}
		return nil
	}

	t := settingsOffsiteTarget(domain, settings, repo)
	if c, cErr := s.decodeCloud(settings); cErr == nil {
		t.StorageClass = c.S3StorageClass
	}
	if primary != nil {
		// Preserve the existing row's identity + placement + creds selector; only
		// the Settings-derived config is refreshed.
		t.ID = primary.ID
		t.CreatedAt = primary.CreatedAt
		t.SortOrder = primary.SortOrder
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

// offsiteSourcePrefix is the "offsite:<id>" form's prefix. The bare source
// "offsite" (no id) addresses a domain's PRIMARY off-site target; the prefixed
// form addresses a SPECIFIC target by id. The prefixed form is dormant in
// stage 3 (the frontend still sends bare "offsite"); it merely becomes possible.
const offsiteSourcePrefix = "offsite:"

// isOffsiteSource reports whether a browse/restore/delete/prune source string
// addresses an off-site repo — either the bare "offsite" (primary target) or the
// "offsite:<id>" form (a specific target). This is the single predicate every
// call site uses instead of comparing the literal "offsite", so the id form
// flows through unchanged.
func isOffsiteSource(source string) bool {
	return source == "offsite" || strings.HasPrefix(source, offsiteSourcePrefix)
}

// offsiteTargetIDFromSource returns the target id carried by an "offsite:<id>"
// source, or "" for bare "offsite" and any non-offsite source. It never trims or
// otherwise rewrites the id, so it round-trips whatever the caller threaded in.
func offsiteTargetIDFromSource(source string) string {
	if id, ok := strings.CutPrefix(source, offsiteSourcePrefix); ok {
		return id
	}
	return ""
}

// validOffsiteTargetID reports whether id is a plausibly-formed off-site target
// id — a lowercase-hex token as minted by store.newID (16 random bytes → 32 hex
// chars). It is a cheap syntactic guard for the "offsite:<id>" query form so a
// malformed token is never carried into a source string. An id that PASSES this
// check but matches no target is still safe: offsiteTargetForSource falls back to
// the primary target, so a stale-but-well-formed id never breaks a restore.
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

// offsiteTargetForSource resolves the off-site DESTINATION a source string
// addresses within a domain, given the already-resolved settings (callers always
// hold them; taking them here avoids a redundant store read and keeps the
// pure-settings / nil-store paths working exactly like offsiteRepoFor):
//   - bare "offsite" → the PRIMARY (first enabled) target — the same repo as
//     today for a backfilled single-target install.
//   - "offsite:<id>" → the enabled target with that id; an UNKNOWN id falls back
//     to the primary (the safer choice — a stale id never strands a restore).
//   - when no target ROW exists (an un-backfilled install configured only through
//     the legacy Settings columns) it synthesizes the one target the backfill
//     would have produced, so N=1 resolves identically whether or not it was
//     backfilled.
//
// ok is false only for a non-offsite source or when the domain has no off-site
// repo configured at all (the caller then surfaces its "no such repo" error).
func (s *Service) offsiteTargetForSource(settings store.Settings, domain, source string) (store.OffsiteTarget, bool) {
	if !isOffsiteSource(source) {
		return store.OffsiteTarget{}, false
	}
	targets := s.offsiteTargetsFor(domain) // enabled, in stable per-domain order
	if id := offsiteTargetIDFromSource(source); id != "" {
		for _, t := range targets {
			if t.ID == id {
				return t, true
			}
		}
		// Unknown id → fall through to the primary/settings fallback below.
	}
	if len(targets) > 0 {
		return targets[0], true // primary = first enabled
	}
	// No rows: reproduce the backfill's single target from the legacy columns.
	loc := offsiteRepoFromSettings(domain, settings)
	if loc == "" {
		return store.OffsiteTarget{}, false
	}
	return settingsOffsiteTarget(domain, settings, loc), true
}

// offsiteSourceImmutable reports whether the off-site DESTINATION addressed by an
// (already offsite) source is flagged append-only/immutable. It reads the
// SPECIFIC target's flag; for bare "offsite" that is the primary target, whose
// flag equals offsiteImmutableFor for a backfilled install. When no target
// resolves (e.g. no off-site repo configured) it falls back to the legacy
// per-domain Settings flag so the delete/prune refusal stays byte-identical.
func (s *Service) offsiteSourceImmutable(settings store.Settings, domain, source string) bool {
	if target, ok := s.offsiteTargetForSource(settings, domain, source); ok {
		return target.Immutable
	}
	return offsiteImmutableFor(domain, settings)
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
