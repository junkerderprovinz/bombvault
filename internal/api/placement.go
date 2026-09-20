package api

import (
	"context"
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
	"time"

	"github.com/junkerderprovinz/bombvault/internal/notify"
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

// validPlacementDomain reports whether domain is one of the three that carry
// copy rules and named repositories (containers, vms, files); flash and config
// replicate a single repository and never reach this.
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

// retentionPolicyForRef is the keep-policy a repository ages by.
func (s *Service) retentionPolicyForRef(settings store.Settings, _ domainRepoRef) restic.RetentionPolicy {
	return s.retentionPolicy(settings)
}

// domainHasRetention reports whether any repository of the domain ages by a
// keep-policy, which is what makes the prune after a round worth it.
func (s *Service) domainHasRetention(settings store.Settings, domain string) bool {
	repos, _, err := s.domainReposInUse(settings, domain)
	if err != nil {
		return s.retentionPolicy(settings).Any()
	}
	return slices.ContainsFunc(repos, func(r domainRepoRef) bool { return s.retentionPolicyForRef(settings, r).Any() })
}

// placementTargetName is a target as the interface names it: its name, or its
// location without credentials when it has none.
func placementTargetName(t store.OffsiteTarget) string {
	if t.Name != "" {
		return t.Name
	}
	return scrubRepoLocation(t.Repo)
}

// placedItem is one item row as the copy path sees it.
type placedItem struct {
	ID       string // the row id runs are recorded under
	Identity string
	RepoID   string // named repository id, "" for the domain path
	Kind     homeKind
}

// placedItems lists a domain's item rows with the kind of their home.
func (s *Service) placedItems(settings store.Settings, domain string) ([]placedItem, error) {
	named, err := s.namedRepoIndex()
	if err != nil {
		return nil, err
	}
	var out []placedItem
	add := func(id, identity, repo string) {
		repo = strings.TrimSpace(repo)
		out = append(out, placedItem{ID: id, Identity: identity, RepoID: repo, Kind: s.homeKindOf(settings, domain, repo, named)})
	}
	switch domain {
	case "containers":
		rows, err := s.store.ListTargets()
		if err != nil {
			return nil, err
		}
		for _, t := range rows {
			add(t.ID, "container:"+t.ContainerName, t.Repo)
		}
	case "vms":
		rows, err := s.store.ListVMTargets()
		if err != nil {
			return nil, err
		}
		for _, v := range rows {
			add(v.ID, "vm:"+v.Name, v.Repo)
		}
	case "files":
		rows, err := s.store.ListFileSets()
		if err != nil {
			return nil, err
		}
		for _, fs := range rows {
			add(fs.ID, "fileset:"+fs.Name, fs.Repo)
		}
	}
	return out, nil
}

// namedRepoIndex is ListNamedRepos by id, read once per request.
func (s *Service) namedRepoIndex() (map[string]store.OffsiteTarget, error) {
	rows, err := s.store.ListNamedRepos()
	if err != nil {
		return nil, err
	}
	out := make(map[string]store.OffsiteTarget, len(rows))
	for _, r := range rows {
		out[r.ID] = r
	}
	return out, nil
}

// pausePlacement pauses the domain's replication until its default is confirmed,
// and notifies when this call is the one that started the pause.
func (s *Service) pausePlacement(ctx context.Context, domain, why string) error {
	s.placementMu.Lock()
	started, err := s.store.PausePlacement(domain)
	s.placementMu.Unlock()
	if err != nil {
		return err
	}
	if started {
		s.notifyPlacementPaused(ctx, domain, why)
	}
	return nil
}

// pauseReasons says, by the why of pausePlacement, what made a domain pause.
var pauseReasons = map[string]string{
	"found-history": "its first listing found backups this database never replicated, so the database may have been rebuilt without the rules that kept items from being copied",
	"older-source":  "one of its sources holds a snapshot older than this database, so the database may have been rebuilt without the rules that kept items from being copied",
	"discover":      "Discover rebuilt its items in a database that never backed them up or replicated them, so the rules that kept items from being copied are gone",
}

// notifyPlacementPaused says that a domain's replication waits for its default
// to be confirmed, and where. Same gate and fan-out as notifyReplicationFailed.
func (s *Service) notifyPlacementPaused(ctx context.Context, domain, why string) {
	c, err := s.NotifyConfig()
	if err != nil || c.On == "" || c.On == "never" {
		return
	}
	subject := "Off-site replication paused for " + domain
	msg := fmt.Sprintf("Nothing in %s is copied off site: %s. Confirm the placement default under Settings > Storage > Placement defaults to resume.", domain, pauseReasons[why])
	notify.Send(ctx, c, domain, notify.Event{Title: "BombVault", Message: subject + ": " + msg, OK: false})
	if s.unraidGate(c.Unraid) {
		if e := s.sendUnraidNotify(ctx, "BombVault: "+subject, msg, "warning"); e != nil {
			log.Printf("notify: unraid: %v", e)
		}
	}
}

// listTargetInBackground lists one target for one domain once, so a change that
// took copies away from a target never listed can name how many stay.
func (s *Service) listTargetInBackground(domain, targetID string) {
	key := domain + "\x00" + targetID
	s.listingMu.Lock()
	if s.listing == nil {
		s.listing = map[string]bool{}
	}
	if s.listing[key] {
		s.listingMu.Unlock()
		return
	}
	s.listing[key] = true
	s.listingMu.Unlock()
	go func() {
		defer func() {
			s.listingMu.Lock()
			delete(s.listing, key)
			s.listingMu.Unlock()
		}()
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()
		if err := s.listTargetOnce(ctx, domain, targetID); err != nil {
			log.Printf("api: offsite %s: listing a target in the background failed: %v", domain, scrubError(err)) //nolint:gosec // G706: domain is a fixed literal, the error scrubbed here
		}
	}()
}

// listTargetOnce lists the target and records what it holds. A domain that
// never replicated, and whose default is not already confirmed, looks at the
// listing for history first, as its passes do.
func (s *Service) listTargetOnce(ctx context.Context, domain, targetID string) error {
	settings, err := s.store.GetSettings()
	if err != nil {
		return err
	}
	p, err := s.readPlacement(settings, domain)
	if err != nil {
		return err
	}
	i := slices.IndexFunc(p.Targets, func(t store.OffsiteTarget) bool { return t.ID == targetID })
	if i < 0 {
		return errNotATarget
	}
	pass, err := s.newPass(settings, p)
	if err != nil {
		return err
	}
	held, err := s.listTarget(ctx, settings, p.Targets[i])
	if err != nil {
		return err
	}
	s.recordListing(domain, p.Targets[i], pass.owners, held, nil)
	if p.State.Confirmed() {
		return nil
	}
	if _, listed := pass.listed[targetID]; listed {
		return nil
	}
	never, err := s.neverReplicated(domain)
	if err != nil || !never {
		return err
	}
	if pass.owners.ownsAny(held) {
		return s.pausePlacement(ctx, domain, "found-history")
	}
	sources, _ := s.offsiteReplicationSources(settings, domain)
	_, err = s.pauseOnOlderSources(ctx, settings, pass, sources)
	return err
}
