package api

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"maps"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/progress"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// Why the backups of a section stay where they are when a destination is
// emptied.
const (
	// erasureKeptAppendOnly is a repository BombVault may only add to.
	erasureKeptAppendOnly = "append-only"
	// erasureKeptInUse is a repository that entries still back up to: a direct
	// repository, which outlives the destination as a repository of its own,
	// or one that shares its place with a repository entries back up to.
	erasureKeptInUse = "in-use"
	// erasureKeptShared shares its place with another section's copy or
	// another destination.
	erasureKeptShared = "shared"
	// erasureKeptUnreadable is a repository that could not be listed, so nobody
	// can say what deleting from it would take.
	erasureKeptUnreadable = "unreadable"
)

var (
	errLastCopiesUnnamed = errors.New("entries would lose their only backup and the request does not name them; check the list again")
	errNothingToErase    = errors.New("none of the backups at this destination can be deleted")
)

// forgetChunk is how many snapshots one restic forget is handed, which keeps
// its command line short however much a repository holds.
const forgetChunk = 200

// destinationErasure is what deleting the backups at a destination takes away,
// before the destination itself is removed.
type destinationErasure struct {
	// Removal is what removing the destination detaches. A request to delete
	// sends it back, so nothing is deleted for sections that changed since.
	Removal  locationRemoval  `json:"removal"`
	Sections []erasureSection `json:"sections"`
	// Backups and Entries count what loses a copy over the sections that are
	// emptied. BackupsOnlyThere are the backups among them that no other place
	// BombVault could read holds.
	Backups          int `json:"backups"`
	Entries          int `json:"entries"`
	BackupsOnlyThere int `json:"backupsOnlyThere"`
	// LastCopies are the entries left without any backup. A request has to
	// name each of them.
	LastCopies []erasureEntry `json:"lastCopies"`
	// Key is the progress key the running deletion reports under and is
	// cancelled by.
	Key string `json:"key"`
}

// erasureSection is one repository at the destination: the copy of a section,
// or the direct repository of its target when RepoID is set. One without a
// TargetID is what a section left behind when its copy moved elsewhere.
type erasureSection struct {
	Domain   string `json:"domain"`
	TargetID string `json:"targetId"`
	RepoID   string `json:"repoId,omitempty"`
	Repo     string `json:"repo"`
	Backups  int    `json:"backups"`
	Entries  int    `json:"entries"`
	// Kept is one of the erasureKept reasons, empty for a repository that is
	// emptied.
	Kept string `json:"kept,omitempty"`
	// Use is home for a section's own folder and repo for a named repository,
	// empty for a copy or a direct repository.
	Use string `json:"use,omitempty"`
}

// erasureEntry is an entry by the identity its backups carry, such as
// container:plex.
type erasureEntry struct {
	Domain   string `json:"domain"`
	Identity string `json:"identity"`
	Backups  int    `json:"backups"`
}

// erasureRepo is what emptying one section's repository needs.
type erasureRepo struct {
	domain string
	target store.OffsiteTarget
	// direct marks the target's direct repository, whose backups no listing of
	// the target counts.
	direct bool
	loc    string
	mode   restic.Mode
	snaps  []restic.Snapshot
	owners map[string]snapshotOwner
}

type erasurePlan struct {
	destinationErasure
	repos  []erasureRepo
	places places
	live   []liveRepo
	// destination is the destination being emptied, "" for any other kind of
	// location.
	destination string
	contexts    map[string]ownerContext
	// listed keeps each repository's listing, which the sections of a named
	// repository share.
	listed map[string]repoListing
}

type repoListing struct {
	snaps []restic.Snapshot
	err   error
}

func (s *Service) newErasurePlan(settings store.Settings, removal locationRemoval, objectID, destination string) (erasurePlan, error) {
	live, err := s.liveRepos(settings)
	if err != nil {
		return erasurePlan{}, err
	}
	return erasurePlan{
		destinationErasure: destinationErasure{
			Removal: removal, Sections: []erasureSection{}, LastCopies: []erasureEntry{}, Key: "erase:" + objectID,
		},
		places: s.placesNow(), live: live, destination: destination,
		contexts: map[string]ownerContext{}, listed: map[string]repoListing{},
	}, nil
}

// ownerContext reads the domain's owner rules once per plan.
func (p *erasurePlan) ownerContext(s *Service, domain string) (ownerContext, error) {
	if oc, ok := p.contexts[domain]; ok {
		return oc, nil
	}
	oc, err := s.ownerContextFor(domain)
	if err == nil {
		p.contexts[domain] = oc
	}
	return oc, err
}

func (p *erasurePlan) list(ctx context.Context, s *Service, loc string, mode restic.Mode) ([]restic.Snapshot, error) {
	if l, ok := p.listed[loc]; ok {
		return l.snaps, l.err
	}
	snaps, err := s.listRepo(ctx, loc, mode)
	p.listed[loc] = repoListing{snaps, err}
	return snaps, err
}

// sharedBy says why the repository at raw stays when it is, holds or lies
// inside a repository in use other than the rows named in self: in-use for one
// entries back up to, shared for another section's copy or another
// destination, empty when the place is its own.
func (p *erasurePlan) sharedBy(s *Service, raw string, self ...string) string {
	loc, err := s.resolveRepo(raw)
	if err != nil {
		return ""
	}
	homes := slices.DeleteFunc(slices.Clone(p.live), func(r liveRepo) bool { return r.kind != liveHome })
	switch {
	case overlapsLive(p.places, loc, homes, self...):
		return erasureKeptInUse
	case overlapsLive(p.places, loc, p.live, self...):
		return erasureKeptShared
	}
	return ""
}

// entryOf names the entry a backup belongs to: its settled owner, or the
// identity tag it carries where the domain settles none.
func entryOf(sn restic.Snapshot, owners map[string]snapshotOwner) string {
	if owner := owners[sn.ID].Owner; owner != "" {
		return owner
	}
	if tags := identityTags([]restic.Snapshot{sn}); len(tags) > 0 {
		return itemTag(tags[0])
	}
	return ""
}

// ofSection keeps the backups BombVault wrote for the domain. A repository can
// hold other domains' backups and ones written by hand, and those are nobody's
// to delete here.
func ofSection(snaps []restic.Snapshot, domain string) []restic.Snapshot {
	var out []restic.Snapshot
	for _, sn := range snaps {
		if len(domainIdentityTags(identityTags([]restic.Snapshot{sn}), domain)) > 0 {
			out = append(out, sn)
		}
	}
	return out
}

// heldElsewhere is what the places outside a destination hold of a domain.
type heldElsewhere struct{ entries, backups map[string]bool }

// heldOutside lists the domain's repositories and its targets at other
// destinations. A place that cannot be read, or that cannot be told apart
// from one the deletion empties, counts as holding nothing, which errs
// towards calling a copy the last one. emptied are the resolved locations the
// deletion empties.
func (s *Service) heldOutside(ctx context.Context, settings store.Settings, domain, destinationID string, oc ownerContext, emptied []string) heldElsewhere {
	held := heldElsewhere{entries: map[string]bool{}, backups: map[string]bool{}}
	places := s.placesNow()
	elsewhere := func(loc string) bool {
		at, named := places.of(loc)
		if !named {
			log.Printf("api: erase: %s cannot be told apart from the places being emptied and counts as empty", shortRepoName(loc)) //nolint:gosec // G706: the location is shortened
			return false
		}
		for _, gone := range emptied {
			other, ok := places.of(gone)
			if ok && at.same(other) || !ok && sameRepoLocation(loc, gone) {
				return false
			}
		}
		return true
	}
	add := func(snaps []restic.Snapshot) {
		snaps = ofSection(snaps, domain)
		owners := oc.owners(snaps)
		for _, sn := range snaps {
			held.entries[entryOf(sn, owners)] = true
			held.backups[restic.Identity(sn)] = true
		}
	}
	homes, _, err := s.domainReposInUse(settings, domain)
	if err != nil {
		log.Printf("api: erase: the %s repositories do not resolve and count as empty: %v", domain, err) //nolint:gosec // G706: domain is a fixed literal
	}
	for _, home := range homes {
		if !elsewhere(home.Loc) {
			continue
		}
		snaps, err := s.listRepo(ctx, home.Loc, s.primaryModeFor(settings, domain, home.Loc))
		if err != nil {
			log.Printf("api: erase: %s could not be listed and counts as empty: %v", shortRepoName(home.Loc), scrubError(err)) //nolint:gosec // G706: the location is shortened and the error scrubbed
			continue
		}
		add(snaps)
	}
	targets, err := s.store.OffsiteTargetsForDomain(domain)
	if err != nil {
		log.Printf("api: erase: the other %s targets could not be read and count as empty: %v", domain, err) //nolint:gosec // G706: domain is a fixed literal
	}
	for _, t := range targets {
		if destinationID != "" && t.DestinationID == destinationID {
			continue
		}
		if loc, err := s.resolveRepo(t.Repo); err != nil || !elsewhere(loc) {
			continue
		}
		snaps, err := s.listTarget(ctx, settings, t)
		if err != nil {
			log.Printf("api: erase: %s could not be listed and counts as empty: %v", placementTargetName(t), scrubError(err)) //nolint:gosec // G706: the name is the row's own and the error scrubbed
			continue
		}
		add(snaps)
	}
	return held
}

// planErasure lists every repository at the destination and works out what
// deleting its backups takes away. Nothing is deleted.
func (s *Service) planErasure(ctx context.Context, id string) (erasurePlan, error) {
	settings, err := s.store.GetSettings()
	if err != nil {
		return erasurePlan{}, fmt.Errorf("read settings: %w", err)
	}
	d, removal, err := s.destinationRemovalPreview(id)
	if err != nil {
		return erasurePlan{}, err
	}
	if removal.holdsOwnFolder() {
		return erasurePlan{}, errOwnFolderThere
	}
	targets, err := s.store.DestinationTargets(id)
	if err != nil {
		return erasurePlan{}, err
	}
	plan, err := s.newErasurePlan(settings, removal, id, id)
	if err != nil {
		return erasurePlan{}, err
	}
	for _, t := range targets {
		if err := plan.addTarget(ctx, s, settings, t); err != nil {
			return erasurePlan{}, err
		}
	}
	// A section that moved to another destination left its repository here,
	// where no target points at it. One that a repository in use shares is not
	// left behind.
	for _, domain := range offsiteConfigDomains {
		if slices.ContainsFunc(targets, func(t store.OffsiteTarget) bool { return t.Domain == domain }) ||
			plan.sharedBy(s, destinationLocation(d.Repo, domain), id) != "" {
			continue
		}
		left := store.OffsiteTarget{
			Domain: domain, Repo: destinationLocation(d.Repo, domain),
			CredsRef: d.CredsRef, StorageClass: d.StorageClass, Immutable: d.Immutable,
		}
		loc, err := s.resolveRepo(left.Repo)
		if err != nil || !s.engine.RepoOpens(ctx, loc, s.offsiteModeForTarget(settings, left)) {
			continue
		}
		oc, err := plan.ownerContext(s, domain)
		if err != nil {
			return erasurePlan{}, err
		}
		kept := ""
		if d.Immutable {
			kept = erasureKeptAppendOnly
		}
		at := len(plan.Sections)
		plan.addCopy(ctx, s, settings, oc, left, store.OffsiteTarget{}, kept)
		if plan.Sections[at].Backups == 0 {
			plan.Sections, plan.repos = plan.Sections[:at], plan.repos[:at]
		}
	}
	plan.finish(ctx, s, settings)
	return plan, nil
}

// addTarget appends a target's copy and, when it has one, its direct
// repository.
func (p *erasurePlan) addTarget(ctx context.Context, s *Service, settings store.Settings, t store.OffsiteTarget) error {
	oc, err := p.ownerContext(s, t.Domain)
	if err != nil {
		return err
	}
	direct, found, err := s.store.CompanionFor(t.ID)
	if err != nil {
		return err
	}
	// A direct repository in a folder inside its target is a repository of
	// its own there, which a delete in the target never reaches.
	inside := found && insideOwnTarget(t, direct.Repo)
	self := []string{t.ID, p.destination}
	if inside {
		self = append(self, direct.ID)
	}
	kept := p.sharedBy(s, t.Repo, self...)
	if s.refusesDeletes(t) {
		kept = erasureKeptAppendOnly
	}
	p.addCopy(ctx, s, settings, oc, t, store.OffsiteTarget{}, kept)

	if !found {
		return nil
	}
	use, err := s.namedRepoUse(direct.ID)
	if err != nil {
		return err
	}
	self = []string{direct.ID, p.destination}
	if inside {
		self = append(self, t.ID)
	}
	kept = p.sharedBy(s, direct.Repo, self...)
	switch {
	case use.InUse():
		kept = erasureKeptInUse
	case kept == "" && direct.Immutable:
		kept = erasureKeptAppendOnly
	}
	p.addCopy(ctx, s, settings, oc, t, direct, kept)
	return nil
}

// finish counts what the erasure takes and seals it with its digest.
func (p *erasurePlan) finish(ctx context.Context, s *Service, settings store.Settings) {
	p.count(ctx, s, settings)
	p.Removal.Erasure = p.digest()
}

// digest sums up what the erasure takes, down to the snapshot ids of each
// section, so a request can show it was made on the same.
func (p erasurePlan) digest() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%d\n", p.BackupsOnlyThere)
	for i, sec := range p.Sections {
		ids := snapshotIDs(p.repos[i].snaps)
		slices.Sort(ids)
		fmt.Fprintf(&b, "%s\x00%s\x00%s\x00%s\x00%s\x00%s\n", sec.Domain, sec.TargetID, sec.RepoID, sec.Repo, sec.Kept, strings.Join(ids, ","))
	}
	sum := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(sum[:])
}

// refusesDeletes reports whether a target is marked append-only or its last
// delete test was refused, by append-only or an object lock. A test that cannot
// be read counts as refused.
func (s *Service) refusesDeletes(t store.OffsiteTarget) bool {
	if t.Immutable {
		return true
	}
	test, found, err := s.store.LatestTamperTestForTarget(t.Domain, t.ID)
	return err != nil || found && test.Protected
}

// addCopy appends one repository of a target, its copy or its direct
// repository.
func (p *erasurePlan) addCopy(ctx context.Context, s *Service, settings store.Settings, oc ownerContext, t, direct store.OffsiteTarget, kept string) {
	raw := t.Repo
	if direct.ID != "" {
		raw = direct.Repo
	}
	section := erasureSection{Domain: t.Domain, TargetID: t.ID, RepoID: direct.ID, Repo: scrubRepoLocation(raw), Kept: kept}
	repo := erasureRepo{domain: t.Domain, target: t, direct: direct.ID != ""}
	p.add(ctx, s, oc, section, repo, raw, func(loc string) restic.Mode {
		if repo.direct {
			return s.primaryModeFor(settings, t.Domain, loc)
		}
		return s.offsiteModeForTarget(settings, t)
	})
}

// add lists the repository at raw and appends it as a section. One that
// cannot be listed is kept.
func (p *erasurePlan) add(ctx context.Context, s *Service, oc ownerContext, section erasureSection, repo erasureRepo, raw string, modeOf func(loc string) restic.Mode) {
	loc, err := s.resolveRepo(raw)
	if err == nil {
		repo.loc, repo.mode = loc, modeOf(loc)
		var all []restic.Snapshot
		if all, err = p.list(ctx, s, loc, repo.mode); err == nil {
			repo.snaps = ofSection(all, repo.domain)
			repo.owners = oc.owners(repo.snaps)
		}
	}
	if err != nil && section.Kept == "" {
		section.Kept = erasureKeptUnreadable
	}
	entries := map[string]bool{}
	for _, sn := range repo.snaps {
		entries[entryOf(sn, repo.owners)] = true
	}
	section.Backups, section.Entries = len(repo.snaps), len(entries)
	p.Sections = append(p.Sections, section)
	p.repos = append(p.repos, repo)
}

// count fills in the totals over the sections that are emptied and finds the
// entries nothing else holds a backup of.
func (p *erasurePlan) count(ctx context.Context, s *Service, settings store.Settings) {
	var emptied []string
	for i, section := range p.Sections {
		if section.Kept == "" {
			emptied = append(emptied, p.repos[i].loc)
		}
	}
	outside := map[string]heldElsewhere{}
	last := map[string]*erasureEntry{}
	entries := map[string]bool{}
	for i, section := range p.Sections {
		if section.Kept != "" {
			continue
		}
		repo := p.repos[i]
		held, known := outside[repo.domain]
		if !known {
			held = s.heldOutside(ctx, settings, repo.domain, p.destination, p.contexts[repo.domain], emptied)
			outside[repo.domain] = held
		}
		p.Backups += len(repo.snaps)
		for _, sn := range repo.snaps {
			entry := entryOf(sn, repo.owners)
			entries[entry] = true
			if !held.backups[restic.Identity(sn)] {
				p.BackupsOnlyThere++
			}
			if held.entries[entry] {
				continue
			}
			if last[entry] == nil {
				last[entry] = &erasureEntry{Domain: repo.domain, Identity: entry}
			}
			last[entry].Backups++
		}
	}
	p.Entries = len(entries)
	for _, identity := range slices.Sorted(maps.Keys(last)) {
		p.LastCopies = append(p.LastCopies, *last[identity])
	}
}

// erasureRefused is a refusal decided on a fresh listing. It carries what the
// deletion would take as it stands.
type erasureRefused struct {
	err error
	now destinationErasure
}

func (e *erasureRefused) Error() string { return e.err.Error() }
func (e *erasureRefused) Unwrap() error { return e.err }

// erasureJob is the deletion of the backups at one storage location: how its
// plan is made and how the location goes once they are gone.
type erasureJob struct {
	// id is the location's object id, which a running move names too.
	id     string
	plan   func(ctx context.Context) (erasurePlan, error)
	remove func(removal locationRemoval) (store.Settings, error)
}

func (s *Service) destinationErasureJob(id string) erasureJob {
	return erasureJob{
		id:     id,
		plan:   func(ctx context.Context) (erasurePlan, error) { return s.planErasure(ctx, id) },
		remove: func(removal locationRemoval) (store.Settings, error) { return s.RemoveDestination(id, removal) },
	}
}

// StartErasure deletes the backups at a storage location and then removes
// it, in the background. expect is the removal that was shown and named the
// entries the person agreed to leave without any backup. Every section of
// the location stays locked until the run ends, so no backup or copy lands
// there in between. removed runs once the location is gone.
func (s *Service) StartErasure(ctx context.Context, job erasureJob, expect locationRemoval, named []string, removed func(store.Settings)) (destinationErasure, error) {
	var unlocks []func()
	unlock := func() {
		for _, u := range unlocks {
			u()
		}
	}
	locked := map[string]bool{}
	lock := func(domain string) error {
		if locked[domain] {
			return nil
		}
		u, ok := s.tryLockDomainFor(domain, "delete")
		if !ok {
			return errDomainBusy
		}
		locked[domain] = true
		unlocks = append(unlocks, u)
		return nil
	}
	// Which sections a plan touches is known only once it is made, so their
	// domains are locked after it and the plan is made again under that lock.
	var plan erasurePlan
	var err error
	for {
		if plan, err = job.plan(ctx); err != nil {
			break
		}
		before := len(locked)
		for _, domain := range plan.domains() {
			if err = lock(domain); err != nil {
				break
			}
		}
		if err != nil || len(locked) == before {
			break
		}
	}
	if err == nil {
		err = plan.refusal(expect, named)
	}
	if err != nil {
		unlock()
		return destinationErasure{}, err
	}

	rctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 24*time.Hour)
	s.cancelMu.Lock()
	for _, places := range s.moving {
		if places[0] == job.id || places[1] == job.id {
			s.cancelMu.Unlock()
			cancel()
			unlock()
			return destinationErasure{}, errMoveRunning
		}
	}
	if s.runCancels == nil {
		s.runCancels = map[string]context.CancelFunc{}
	}
	s.runCancels[plan.Key] = cancel
	s.cancelMu.Unlock()

	go func() {
		defer s.recoverOperation("erase storage location", nil, nil)
		defer unlock()
		defer cancel()
		defer s.unregisterCancel(plan.Key)

		_, startedAt := s.progBegin(rctx, plan.Key, "maintenance")
		err := s.runErasure(rctx, plan, startedAt)
		if err == nil {
			var settings store.Settings
			if settings, err = job.remove(plan.Removal); err == nil {
				removed(settings)
			}
		}
		s.progEnd(plan.Key, "maintenance", err == nil, startedAt)
		if err != nil {
			log.Printf("api: erase storage location: %v", scrubError(err)) //nolint:gosec // G706: the error is scrubbed
		}
	}()
	return plan.destinationErasure, nil
}

// domains are the sections a plan deletes from or takes the location from.
func (p erasurePlan) domains() []string {
	var out []string
	for _, sec := range p.Sections {
		out = append(out, sec.Domain)
	}
	for _, sec := range p.Removal.Sections {
		out = append(out, sec.Domain)
	}
	return out
}

// refusal says why a planned deletion must not start, nil when it may.
func (p erasurePlan) refusal(expect locationRemoval, named []string) error {
	if !p.Removal.same(expect) {
		return &removalChangedErr{now: p.Removal}
	}
	if p.Removal.Erasure != expect.Erasure {
		return &erasureRefused{err: errPlacementStale, now: p.destinationErasure}
	}
	if !slices.ContainsFunc(p.Sections, func(sec erasureSection) bool { return sec.Kept == "" }) {
		return &erasureRefused{err: errNothingToErase, now: p.destinationErasure}
	}
	for _, entry := range p.LastCopies {
		if !slices.Contains(named, entry.Identity) {
			return &erasureRefused{err: errLastCopiesUnnamed, now: p.destinationErasure}
		}
	}
	return nil
}

// runErasure empties one repository after the other and stops at the first
// that fails, so a destination that still holds backups is not removed. Each
// is recorded as a prune of its section, and the one that fails names those
// already emptied.
func (s *Service) runErasure(ctx context.Context, plan erasurePlan, startedAt int64) error {
	var todo []erasureRepo
	for i, section := range plan.Sections {
		if section.Kept == "" {
			todo = append(todo, plan.repos[i])
		}
	}
	var emptied []string
	for done, repo := range todo {
		s.publishProgress(progress.Event{
			Key: plan.Key, Phase: "maintenance", Active: true, StartedAt: startedAt,
			Percent: 100 * float64(done) / float64(len(todo)), Done: int64(done), Total: int64(len(todo)), Unit: "repositories",
		})
		runID, rErr := s.startRun(ctx, domainRunTargetID(repo.domain), "prune")
		if rErr != nil {
			log.Printf("api: erase %s: the run is not recorded: %v", repo.domain, rErr) //nolint:gosec // G706: domain is a fixed literal
		}
		err := s.emptyRepo(ctx, repo)
		if err != nil && len(emptied) > 0 {
			err = fmt.Errorf("%w; already emptied: %s", err, strings.Join(emptied, ", "))
		}
		if runID != "" {
			status := "success"
			switch {
			case errors.Is(err, context.Canceled):
				status = "cancelled"
			case err != nil:
				status = "failed"
			}
			if fErr := s.store.FinishRun(runID, status, "", 0, truncateRunErr(err)); fErr != nil {
				log.Printf("api: erase %s: the run is not closed: %v", repo.domain, fErr) //nolint:gosec // G706: domain is a fixed literal
			}
		}
		if err != nil {
			return fmt.Errorf("%s at %s: %w", repo.domain, shortRepoName(repo.loc), err)
		}
		emptied = append(emptied, repo.domain+" at "+scrubRepoLocation(repo.loc))
	}
	return nil
}

// emptyRepo forgets the section's backups and prunes once. What the target
// was last seen holding goes with them, so a run that ends early leaves no
// copies listed that are gone.
func (s *Service) emptyRepo(ctx context.Context, repo erasureRepo) error {
	for ids := range slices.Chunk(snapshotIDs(repo.snaps), forgetChunk) {
		forget := func() error { return s.engine.Forget(ctx, repo.loc, ids, false, repo.mode) }
		if err := s.retryAfterUnlock(ctx, repo.loc, repo.mode, forget); err != nil {
			return fmt.Errorf("forget backups: %w", err)
		}
	}
	if len(repo.snaps) == 0 {
		return nil
	}
	if !repo.direct && repo.target.ID != "" && validPlacementDomain(repo.domain) {
		if err := s.store.RecordTargetListing(repo.domain, repo.target.ID, time.Now().Unix(), nil); err != nil {
			log.Printf("api: erase %s: the copies seen at %s stay listed: %v", repo.domain, placementTargetName(repo.target), err) //nolint:gosec // G706: domain is a fixed literal, the name is the row's own
		}
	}
	if err := s.engine.Prune(ctx, repo.loc, repo.mode); err != nil {
		return fmt.Errorf("prune: %w", err)
	}
	return nil
}

// handleDestinationErasurePreview says what deleting the backups at a
// destination would take away.
// GET /api/offsite/destinations/{id}/erasure
func (h *Handler) handleDestinationErasurePreview(w http.ResponseWriter, r *http.Request) {
	if !h.requireAuthForSecrets(w, r, "listing what deleting the backups at a destination takes") {
		return
	}
	plan, err := h.svc.planErasure(r.Context(), r.PathValue("id"))
	if errors.Is(err, store.ErrNotDestination) {
		writeJSON(w, http.StatusNotFound, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"erasure": plan.destinationErasure}))
}

// handleEraseDestination starts deleting the backups at a destination and
// removing it afterwards. The body carries the removal that was shown and the
// identities of the entries that may lose their only backup. A refusal on a
// fresh listing carries that listing. Without a login password anybody on the
// network could delete the backups, so it takes one.
// POST /api/offsite/destinations/{id}/erasure
func (h *Handler) handleEraseDestination(w http.ResponseWriter, r *http.Request) {
	if !h.requireAuthForSecrets(w, r, "deleting the backups at a destination") {
		return
	}
	var body struct {
		Expect     locationRemoval `json:"expect"`
		LastCopies []string        `json:"lastCopies"`
	}
	if !decodeBody(w, r, &body) {
		return
	}
	// A removed primary target took the section's off-site settings with it,
	// and the scheduler reads those.
	removed := func(settings store.Settings) {
		if err := h.scheduler.ReloadWithGates(settings, h.dueGates()); err != nil {
			log.Printf("api: erase destination: the scheduler did not reload: %v", err)
		}
	}
	erasure, err := h.svc.StartErasure(r.Context(), h.svc.destinationErasureJob(r.PathValue("id")), body.Expect, body.LastCopies, removed)
	var changed *removalChangedErr
	var refused *erasureRefused
	switch {
	case errors.Is(err, store.ErrNotDestination):
		writeJSON(w, http.StatusNotFound, map[string]any{"ok": false, "error": err.Error()})
	case errors.As(err, &changed):
		placementFail(w, err, map[string]any{"removal": changed.now})
	case errors.As(err, &refused):
		placementFail(w, err, map[string]any{"erasure": refused.now})
	case err != nil:
		placementFail(w, err, nil)
	default:
		writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"started": true, "erasure": erasure}))
	}
}
