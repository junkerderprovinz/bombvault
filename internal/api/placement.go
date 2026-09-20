package api

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"maps"
	"net/http"
	"slices"
	"strings"

	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

var (
	errPlacementUnreadable = errors.New("placement rules could not be read, so nothing is copied until they can")
	errInvalidPlacement    = errors.New("home and copies each take follow or one value")
	errCopiesNotAllowed    = errors.New("an item on a remote or direct repository takes no copies")
	errNotATarget          = errors.New("that is not an off-site target of this domain")
)

// placementCodes is searched in order and the first match wins, so an error that
// can arrive wrapped in a broader one keeps its row above that one's.
var placementCodes = []struct {
	err  error
	code string
}{
	{errPlacementUnreadable, "placement-unreadable"},
	{errInvalidPlacement, "invalid-placement"},
	{store.ErrRuleDomain, "invalid-placement"},
	{errCopiesNotAllowed, "copies-not-allowed"},
	{errNotATarget, "unknown-target"},
	{errUnknownOffsiteTarget, "unknown-target"},
	{store.ErrStackCopyRule, "stack-rule"},
	{store.ErrCopyRuleTaken, "copy-rule-taken"},
}

// placementCode returns the code the interface translates err by, "" for none.
func placementCode(err error) string {
	for _, c := range placementCodes {
		if errors.Is(err, c.err) {
			return c.code
		}
	}
	return ""
}

// placementFail writes a refusal: HTTP 200, ok false, the scrubbed error, its code
// when it has one, and the extra fields.
func placementFail(w http.ResponseWriter, err error, extra map[string]any) {
	env := map[string]any{"ok": false, "error": scrubError(err)}
	if code := placementCode(err); code != "" {
		env["code"] = code
	}
	maps.Copy(env, extra)
	writeJSON(w, http.StatusOK, env)
}

// validPlacementDomain reports whether domain takes a placement, the check an
// item route runs before it touches the store.
//
//nolint:unused // called by the item routes
func validPlacementDomain(domain string) bool {
	return slices.Contains(store.PlacementDomains, domain)
}

// itemParam reads {domain} and {name} of an /api/items route with the validator
// the domain's own routes use, and writes the 400 itself.
func (h *Handler) itemParam(w http.ResponseWriter, r *http.Request, domains ...string) (domain, key string, ok bool) {
	domain, key = r.PathValue("domain"), r.PathValue("name")
	if !slices.Contains(domains, domain) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "unknown domain"})
		return "", "", false
	}
	var valid bool
	switch domain {
	case "vms":
		valid = validVMName(key)
	case "flash", "config":
		valid = key == domain
	default:
		valid = validResourceName(key)
	}
	if !valid {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "invalid name"})
		return "", "", false
	}
	return domain, key, true
}

// placementRead is one read of a domain's placement, taken before a run's first
// restic call and used for every decision in that run.
type placementRead struct {
	Domain           string
	State            store.PlacementState
	Targets          []store.OffsiteTarget // every target row of the domain, switched off or not, in sort order
	TargetsUncertain bool                  // the target list could not be read, or only the synthetic settings target exists
}

// readPlacement reads a domain's rules, default and targets. On an error the read
// still carries the targets, so a pass can record a failed row for each.
func (s *Service) readPlacement(settings store.Settings, domain string) (placementRead, error) {
	p := placementRead{Domain: domain}
	rows, err := s.store.OffsiteTargetsForDomain(domain)
	switch {
	case err != nil:
		log.Printf("api: placement %s: the off-site targets could not be read: %v", domain, err) //nolint:gosec // G706: domain is a fixed literal
		p.Targets, p.TargetsUncertain = legacyTargets(domain, settings), true
	case len(rows) == 0:
		p.Targets = legacyTargets(domain, settings)
		p.TargetsUncertain = len(p.Targets) > 0
	default:
		p.Targets = rows
	}
	state, err := s.store.ReadPlacement(domain)
	if err != nil {
		return p, fmt.Errorf("%w: %v", errPlacementUnreadable, err)
	}
	p.State = state
	return p, nil
}

// legacyTargets is the target the domain's off-site field describes while no
// target row exists.
func legacyTargets(domain string, settings store.Settings) []store.OffsiteTarget {
	if loc := offsiteRepoFromSettings(domain, settings); loc != "" {
		return []store.OffsiteTarget{settingsOffsiteTarget(domain, settings, loc)}
	}
	return nil
}

func (p placementRead) enabledTargets() []store.OffsiteTarget {
	out := []store.OffsiteTarget{}
	for _, t := range p.Targets {
		if t.Enabled {
			out = append(out, t)
		}
	}
	return out
}

// defaultSkip is what an item without a rule of its own leaves out.
func (p placementRead) defaultSkip() []string {
	if p.State.HasDefault {
		return p.State.Default.Skip
	}
	return []string{}
}

// resolvedSkip is the item's own rule or the default's skip, which project
// folders always take. own says whether a rule of its own applied.
func (p placementRead) resolvedSkip(identity string) (skip []string, own bool) {
	if !strings.HasPrefix(identity, "stack:") {
		if r, ok := p.State.Rules[identity]; ok {
			return r.Skip, true
		}
	}
	return p.defaultSkip(), false
}

// effectiveTargets are the enabled targets the item's skip leaves in, in sort
// order. It asks neither about a pause nor about where the item lives.
func (p placementRead) effectiveTargets(identity string) []store.OffsiteTarget {
	skip, _ := p.resolvedSkip(identity)
	return p.targetsFor(skip)
}

// targetsFor is the enabled targets a skip list leaves in.
func (p placementRead) targetsFor(skip []string) []store.OffsiteTarget {
	out := []store.OffsiteTarget{}
	for _, t := range p.enabledTargets() {
		if !skipsTarget(skip, t.ID) {
			out = append(out, t)
		}
	}
	return out
}

// copiesTo is the copy filter: a snapshot stays away from a target only when
// every identity it may belong to leaves the target out. No owner means copy.
func (p placementRead) copiesTo(targetID string, owners []string) bool {
	if len(owners) == 0 {
		return true
	}
	for _, o := range owners {
		if skip, _ := p.resolvedSkip(o); !skipsTarget(skip, targetID) {
			return true
		}
	}
	return false
}

// anyCopiesTo reports whether anything could be copied to the target: a name
// without a rule of its own through the default, or one of the rules.
func (p placementRead) anyCopiesTo(targetID string) bool {
	if !skipsTarget(p.defaultSkip(), targetID) {
		return true
	}
	for _, r := range p.State.Rules {
		if !skipsTarget(r.Skip, targetID) {
			return true
		}
	}
	return false
}

// rulesRev fingerprints what decides how a target ages: the domain's rules, the
// default's skip and the target's keep-policy. A deleted rule changes it too.
func (p placementRead) rulesRev(target store.OffsiteTarget) string {
	h := sha256.New()
	for _, id := range slices.Sorted(maps.Keys(p.State.Rules)) {
		_, _ = fmt.Fprintf(h, "rule %s %q\n", id, p.State.Rules[id].Skip)
	}
	_, _ = fmt.Fprintf(h, "default %q\n", p.defaultSkip())
	_, _ = fmt.Fprintf(h, "keep %d %d %d %d\n", target.RetentionKeepLast, target.RetentionKeepDaily,
		target.RetentionKeepWeekly, target.RetentionKeepMonthly)
	return hex.EncodeToString(h.Sum(nil))
}

// skipsTarget reports whether a skip list leaves the target out.
func skipsTarget(skip []string, targetID string) bool {
	return slices.Contains(skip, store.SkipAll) || slices.Contains(skip, targetID)
}

// homeKind is what an item's home is, as far as copies and segments care.
type homeKind string

const (
	homeDomain       homeKind = "domain"        // local domain path
	homeDomainRemote homeKind = "domain-remote" // remote domain path, still a copy source
	homeLocal        homeKind = "local"         // local named repository
	homeRemote       homeKind = "remote"        // remote named repository with its own credentials
	homeMissing      homeKind = "missing"       // an id without a row
)

// copySource reports whether snapshots at a home of this kind are copied off
// site: restic copy has the target's credentials and no others.
func (k homeKind) copySource() bool {
	return k == homeDomain || k == homeDomainRemote || k == homeLocal
}

// homeKindOf is the kind of home a repo id names. named is ListNamedRepos by id,
// read once per request.
func (s *Service) homeKindOf(settings store.Settings, domain, repoID string, named map[string]store.OffsiteTarget) homeKind {
	if repoID == "" {
		if own, err := s.repoFor(settings, domain, "local"); err == nil && restic.IsRemoteRepo(own) {
			return homeDomainRemote
		}
		return homeDomain
	}
	row, ok := named[repoID]
	switch {
	case !ok:
		return homeMissing
	case restic.IsRemoteRepo(row.Repo):
		return homeRemote
	}
	return homeLocal
}

// itemIdentity is the snapshot tag that names an item: container:<name>,
// vm:<name>, fileset:<set name>. A file set is found by id.
func (s *Service) itemIdentity(item store.ItemRef) (string, error) {
	switch item.Domain {
	case "containers":
		return "container:" + item.Key, nil
	case "vms":
		return "vm:" + item.Key, nil
	case "files":
		fs, err := s.store.GetFileSet(item.Key)
		if errors.Is(err, sql.ErrNoRows) {
			return "", errFileSetNotFound
		}
		if err != nil {
			return "", err
		}
		return "fileset:" + fs.Name, nil
	}
	return "", fmt.Errorf("%q has no placement", item.Domain)
}
