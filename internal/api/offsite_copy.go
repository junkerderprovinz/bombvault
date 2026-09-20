package api

import (
	"context"
	"errors"
	"fmt"
	"log"
	"maps"
	"slices"
	"strings"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/progress"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// errTargetsUncertain stops a pass of a domain with copy rules while only the
// settings field's target could be read: a rule names a target by its id, and
// that target has none.
var errTargetsUncertain = errors.New("the off-site targets could not be read by id, so nothing is copied while copy rules exist")

// failPass records a failed run at every target a pass would have visited and
// returns cause, so the history shows the replication that did not happen.
func (s *Service) failPass(domain string, targets []store.OffsiteTarget, cause error) error {
	reason := truncateRunErr(cause)
	if errors.Is(cause, errPlacementUnreadable) {
		reason = store.ReasonCopyRulesUnreadable
	}
	now := time.Now().Unix()
	for _, t := range targets {
		id, err := s.store.RecordOffsiteRunForTarget(domain, t.ID, now)
		if err == nil {
			err = s.store.FinishOffsiteRun(id, false, reason)
		}
		if err != nil {
			log.Printf("api: offsite %s: could not record the failed run: %v", domain, err) //nolint:gosec // G706: domain is a fixed literal
		}
	}
	return cause
}

// copyChunkSize is how many snapshot ids one restic copy carries. They go on the
// command line, and a long history once crossed its limit.
const copyChunkSize = 500

// replicationPass is what one copyToOffsite call reads once and shares across
// its targets.
type replicationPass struct {
	p      placementRead
	owners ownerContext
	listed map[string]store.TargetObservation // by target id; a target missing here was never listed
	items  []placedItem                       // read while the domain has rules
	copies []store.ItemCopies                 // read while the domain has rules
}

// newPass reads what applying the rules needs. Without it no rule can be
// applied, so any failed read is an unreadable placement.
func (s *Service) newPass(settings store.Settings, p placementRead) (replicationPass, error) {
	r := replicationPass{p: p}
	unreadable := func(err error) (replicationPass, error) {
		return replicationPass{}, fmt.Errorf("%w: %v", errPlacementUnreadable, err)
	}
	var err error
	if r.owners, err = s.ownerContextFor(p.Domain); err != nil {
		return unreadable(err)
	}
	if !validPlacementDomain(p.Domain) {
		return r, nil
	}
	if r.listed, err = s.store.TargetObservationsForDomain(p.Domain); err != nil {
		return unreadable(err)
	}
	if !p.State.HasRules() {
		return r, nil
	}
	if r.items, err = s.placedItems(settings, p.Domain); err != nil {
		return unreadable(err)
	}
	if r.copies, err = s.store.ItemCopiesForDomain(p.Domain); err != nil {
		return unreadable(err)
	}
	return r, nil
}

// targetVisit is what a pass does at one target.
type targetVisit struct {
	p         placementRead
	owners    ownerContext
	targetID  string
	filtered  bool // a name is left out here, so every id restic gets is chosen by the rules
	observe   bool // the target has a row, so what it holds is recorded
	agingOnly bool // no item is copied here; the visit lists and ages what the target holds
	aged      bool // it was aged under this state of the rules already
	wasAged   bool // an aging mark exists, cleared when items are copied here again
}

// visit decides what the pass does at one target, and whether it opens it at
// all: a target nothing is copied to that held nothing of the domain at its
// last listing stays closed.
func (r replicationPass) visit(t store.OffsiteTarget) (targetVisit, bool) {
	v := targetVisit{p: r.p, owners: r.owners, targetID: t.ID, filtered: r.p.leavesOutAny(t.ID), observe: t.ID != ""}
	if !r.p.State.HasRules() || t.ID == "" {
		return v, true
	}
	o, listed := r.listed[t.ID]
	v.agingOnly = !r.itemsCopiedTo(t.ID)
	v.wasAged = listed && o.AgedAt > 0
	v.aged = v.wasAged && o.RulesRev == r.p.rulesRev(t)
	return v, r.p.anyCopiesTo(t.ID) || !listed || r.holds(t.ID)
}

// itemsCopiedTo reports whether an item row is copied to the target.
func (r replicationPass) itemsCopiedTo(targetID string) bool {
	return slices.ContainsFunc(r.items, func(it placedItem) bool {
		return it.Kind.copySource() && slices.ContainsFunc(r.p.effectiveTargets(it.Identity),
			func(t store.OffsiteTarget) bool { return t.ID == targetID })
	})
}

// holds reports whether the target's last listing counted anything of the domain.
func (r replicationPass) holds(targetID string) bool {
	return slices.ContainsFunc(r.copies, func(c store.ItemCopies) bool {
		return c.TargetID == targetID && c.SnapshotCount > 0
	})
}

// leavesOutAny reports whether the default or a rule keeps something from the target.
func (p placementRead) leavesOutAny(targetID string) bool {
	if skipsTarget(p.defaultSkip(), targetID) {
		return true
	}
	for _, r := range p.State.Rules {
		if skipsTarget(r.Skip, targetID) {
			return true
		}
	}
	return false
}

// sends keeps the snapshots the target gets: all but those every possible owner
// leaves out.
func (v targetVisit) sends(snaps []restic.Snapshot) []restic.Snapshot {
	owners := v.owners.owners(snaps)
	out := []restic.Snapshot{}
	for _, sn := range snaps {
		if v.p.copiesTo(v.targetID, owners[sn.ID].Possible) {
			out = append(out, sn)
		}
	}
	return out
}

// copyOutcome is what copying every source left at one target.
type copyOutcome struct {
	errs          []error
	copied        int // sources copied without an error
	accounted     int // sources that answered for the domain: copied, or already held there
	destIsASource bool
	landed        []restic.Snapshot // source snapshots this pass put at the target
}

// sourceCopy is one source planned for one target.
type sourceCopy struct {
	src     domainRepoRef
	whole   bool              // hand restic no ids; the unfiltered domain path, where restic decides
	send    []restic.Snapshot // what goes; with whole, what restic is expected to take
	answers bool              // the source holds snapshots of the domain
}

// copySources copies every source to one target; dst is the target's listing
// before the copy. With rules and no listing nothing is copied, because handing
// restic every id would send what the rules leave out.
func (s *Service) copySources(ctx context.Context, domain, dest string, mode restic.Mode, target store.OffsiteTarget, v targetVisit,
	sources []domainRepoRef, dst []restic.Snapshot, dstErr error, startedAt int64, lastCopy *offsiteLastCopy) copyOutcome {
	var out copyOutcome
	if v.filtered && dstErr != nil {
		out.errs = append(out.errs, fmt.Errorf("listing %s: %w", placementTargetName(target), dstErr))
		return out
	}
	held := slices.Clone(dst)
	var plan []sourceCopy
	total := 0
	for _, src := range sources {
		// A named repository created on the location this domain replicates to: the
		// copy would do nothing, and the keep-policy would age the only copy.
		if sameRepoLocation(src.Loc, dest) {
			log.Printf("api: offsite %s: %s is this destination itself; not copying a repository onto itself, and not aging it either", domain, shortRepoName(src.Loc)) //nolint:gosec // G706: domain is a fixed literal, the name is shortened
			out.errs = append(out.errs, fmt.Errorf("%s is this off-site destination itself, so it has no second copy", shortRepoName(src.Loc)))
			out.destIsASource = true
			continue
		}
		c, err := s.planSource(ctx, domain, mode, v, src, held, dstErr == nil)
		if err != nil {
			log.Printf("api: offsite %s: could not read %s, skipping it this pass: %v", domain, shortRepoName(src.Loc), scrubError(err)) //nolint:gosec // G706: domain is a fixed literal, the name is shortened and the error scrubbed here
			out.errs = append(out.errs, fmt.Errorf("reading %s: %w", shortRepoName(src.Loc), err))
			continue
		}
		// A whole source hands restic nil ids and lets its own dedup decide what
		// actually lands, so c.send here is only an estimate. Feeding an estimate
		// into held would let it silently suppress a later source's real, narrowed
		// send whenever the two happen to share an identity.
		if !c.whole {
			held = append(held, c.send...)
		}
		total += len(c.send)
		plan = append(plan, c)
	}
	if dstErr != nil {
		total = 0 // unknown; the progress shows no "of N"
	}
	copyCtx := s.progBeginCopySink(ctx, domain, startedAt, total, lastCopy)
	lim := targetOffsiteLimits(target)
	done := 0
	for _, c := range plan {
		if !c.whole && len(c.send) == 0 {
			if c.answers {
				out.accounted++
			}
			continue
		}
		var err error
		var landed []restic.Snapshot
		if c.whole {
			if err = s.engine.Copy(withIndexOffset(copyCtx, done), dest, c.src.Loc, nil, lim, mode); err == nil {
				landed = c.send
			}
		} else {
			landed, err = s.copyInChunks(copyCtx, dest, c.src.Loc, c.send, lim, mode, done)
		}
		out.landed = append(out.landed, landed...)
		done += len(landed)
		if err != nil {
			log.Printf("api: offsite %s: copying %s failed (continuing with the other sources): %v", domain, shortRepoName(c.src.Loc), scrubError(err)) //nolint:gosec // G706: domain is a fixed literal, the name is shortened and the error scrubbed here
			out.errs = append(out.errs, fmt.Errorf("copying %s: %w", shortRepoName(c.src.Loc), err))
			continue
		}
		out.copied++
		out.accounted++
	}
	return out
}

// planSource lists one source and decides what it sends to the target. held is
// what the target holds plus what earlier sources of this pass send it.
func (s *Service) planSource(ctx context.Context, domain string, mode restic.Mode, v targetVisit, src domainRepoRef, held []restic.Snapshot, heldKnown bool) (sourceCopy, error) {
	c := sourceCopy{src: src, whole: !v.filtered && !src.CountOnly && (src.Own || domainTagPrefix(domain) == "")}
	snaps, err := s.listSnapshots(ctx, src.Loc, mode)
	if err != nil {
		if c.whole {
			// restic reads the source itself; the listing only fed the estimate.
			log.Printf("api: offsite %s: could not estimate pending snapshot count (continuing without it): %v", domain, err) //nolint:gosec // G706: domain is a fixed literal
			return c, nil
		}
		return c, err
	}
	if !src.Own {
		snaps = ofDomain(snaps, domain)
	}
	c.answers = len(snaps) > 0
	switch {
	case src.CountOnly:
	case v.filtered:
		c.send = pendingSnapshots(v.sends(snaps), held)
	case heldKnown:
		c.send = pendingSnapshots(snaps, held)
	default:
		c.send = snaps // the target could not be read: every id, and restic skips what it holds
	}
	return c, nil
}

// copyInChunks hands restic the snapshots in blocks of copyChunkSize and returns
// those that landed before the first failure.
func (s *Service) copyInChunks(ctx context.Context, dest, src string, send []restic.Snapshot, lim restic.Limits, mode restic.Mode, done int) ([]restic.Snapshot, error) {
	var landed []restic.Snapshot
	for chunk := range slices.Chunk(send, copyChunkSize) {
		if err := s.engine.Copy(withIndexOffset(ctx, done+len(landed)), dest, src, snapshotIDs(chunk), lim, mode); err != nil {
			return landed, err
		}
		landed = append(landed, chunk...)
	}
	return landed, nil
}

// pendingSnapshots are the snapshots of src that held has no copy of, by the
// rule restic.PendingCopyIDs keeps.
func pendingSnapshots(src, held []restic.Snapshot) []restic.Snapshot {
	pending := map[string]bool{}
	for _, id := range restic.PendingCopyIDs(src, held) {
		pending[id] = true
	}
	out := make([]restic.Snapshot, 0, len(pending))
	for _, sn := range src {
		if pending[sn.ID] {
			out = append(out, sn)
		}
	}
	return out
}

// ofDomain keeps the snapshots carrying the domain's tag prefix; a named
// repository may hold other domains' snapshots too.
func ofDomain(snaps []restic.Snapshot, domain string) []restic.Snapshot {
	prefix := domainTagPrefix(domain)
	var out []restic.Snapshot
	for _, sn := range snaps {
		if slices.ContainsFunc(sn.Tags, func(tag string) bool { return strings.HasPrefix(tag, prefix) }) {
			out = append(out, sn)
		}
	}
	return out
}

func snapshotIDs(snaps []restic.Snapshot) []string {
	out := make([]string, 0, len(snaps))
	for _, sn := range snaps {
		out = append(out, sn.ID)
	}
	return out
}

// withIndexOffset counts restic's snapshot index on from what earlier copy calls
// to the same target handed over, so "snapshot k of N" does not start again.
func withIndexOffset(ctx context.Context, offset int) context.Context {
	sink := progress.CopySinkFrom(ctx)
	if sink == nil || offset == 0 {
		return ctx
	}
	return progress.WithCopySink(ctx, func(cp progress.CopyProgress) {
		cp.SnapshotIndex += offset
		sink(cp)
	})
}

// recordListing writes what the target holds for the domain: its listing and
// what this pass copied there since.
func (s *Service) recordListing(domain string, target store.OffsiteTarget, owners ownerContext, held, landed []restic.Snapshot) {
	rows := owners.itemCopies(append(slices.Clone(held), landed...))
	if err := s.store.RecordTargetListing(domain, target.ID, time.Now().Unix(), rows); err != nil {
		log.Printf("api: offsite %s: could not record what %s holds: %v", domain, placementTargetName(target), err) //nolint:gosec // G706: domain is a fixed literal, the name is the row's own
	}
}

// itemCopies counts snapshots per owning item. A snapshot whose owner the run
// sibling does not settle counts for nobody.
func (c ownerContext) itemCopies(snaps []restic.Snapshot) []store.ItemCopies {
	owners := c.owners(snaps)
	byIdentity := map[string]*store.ItemCopies{}
	for _, sn := range snaps {
		identity := owners[sn.ID].Owner
		if identity == "" {
			continue
		}
		row, ok := byIdentity[identity]
		if !ok {
			row = &store.ItemCopies{Identity: identity}
			byIdentity[identity] = row
		}
		row.SnapshotCount++
		if t := parseSnapshotTime(sn.Time); !t.IsZero() && t.Unix() > row.LatestSnapshotAt {
			row.LatestSnapshotAt = t.Unix()
		}
	}
	out := make([]store.ItemCopies, 0, len(byIdentity))
	for _, identity := range slices.Sorted(maps.Keys(byIdentity)) {
		out = append(out, *byIdentity[identity])
	}
	return out
}

// ageTarget runs the target's keep-policy over the names it holds and records
// what it holds afterwards. held is its listing before the copy and landed what
// the copy added; without a listing the policy lists by itself.
func (s *Service) ageTarget(ctx context.Context, domain, dest string, mode restic.Mode, target store.OffsiteTarget, v targetVisit, held []restic.Snapshot, heldErr error, landed []restic.Snapshot) bool {
	op := targetOffsiteRetentionPolicy(target)
	if !op.Any() {
		return false
	}
	var tags []string
	if heldErr == nil {
		tags = identityTags(append(slices.Clone(held), landed...))
	}
	var err error
	if len(tags) == 0 {
		err = s.applyRetentionPerIdentity(ctx, dest, op, mode)
	} else {
		err = s.applyRetentionToTags(ctx, dest, op, mode, tags)
	}
	if err != nil {
		log.Printf("api: offsite %s: retention prune failed (replica is safe): %v", domain, err) //nolint:gosec // G706: domain is a fixed literal
	}
	if v.observe {
		after, lErr := s.listSnapshots(ctx, dest, mode)
		if lErr != nil {
			log.Printf("api: offsite %s: could not list %s after its keep-policy: %v", domain, placementTargetName(target), scrubError(lErr)) //nolint:gosec // G706: domain is a fixed literal, the name is the row's own and the error scrubbed here
		} else {
			s.recordListing(domain, target, v.owners, after, nil)
		}
	}
	return err == nil
}

// noteAged keeps the aging mark: set after a pass that only aged the target,
// cleared once something lands there again, predicted or not, since an
// untracked snapshot can slip through a pass the rules still call
// aging-only. A new state of the rules needs no clearing, because its
// fingerprint differs.
func (s *Service) noteAged(domain string, target store.OffsiteTarget, v targetVisit, agingOnly, settled, landed bool) {
	var err error
	switch {
	case agingOnly && settled:
		err = s.store.MarkTargetAged(domain, target.ID, v.p.rulesRev(target), time.Now().Unix())
	case (landed || !v.agingOnly) && v.wasAged:
		err = s.store.ResetTargetAged(domain, target.ID)
	}
	if err != nil {
		log.Printf("api: offsite %s: could not note the aging of %s: %v", domain, placementTargetName(target), err) //nolint:gosec // G706: domain is a fixed literal, the name is the row's own
	}
}

// nothingCopiedNote says a domain's pass had no source to copy from, which
// follows from where its items live and is no failure.
func nothingCopiedNote(domain string) repoSkip {
	return repoSkip{Name: domain, Reason: "nothing in it is copied off site", Note: true}
}

// placeSources applies the rules to the pass's sources: one nothing is copied
// from stays as a counted source while a target may still hold its items, and
// drops out once none does. An unreachable one becomes a note, since it cannot
// leave the pass incomplete, while the keep-policy still stops for it.
func (r replicationPass) placeSources(domain string, sources []domainRepoRef, skipped []repoSkip) ([]domainRepoRef, []repoSkip) {
	if !r.p.State.HasRules() {
		return sources, skipped
	}
	copying := r.copyingRepos()
	out := make([]domainRepoRef, 0, len(sources))
	for _, src := range sources {
		switch {
		case !src.Own && src.Named.ID == "":
			// Neither the domain path nor a known named repository: refFor's
			// answer for a location it does not recognise, which is the
			// post-backup hook's own source. It just received the write the
			// hook exists to copy, so the rules never get a say in reading it.
			out = append(out, src)
		case copying[repoKey(src)]:
			out = append(out, src)
		case r.stillHeld(src):
			src.CountOnly = true
			out = append(out, src)
		default:
			log.Printf("api: offsite %s: nothing of %s is copied and no target holds any of it; not reading it", domain, shortRepoName(src.Loc)) //nolint:gosec // G706: domain is a fixed literal, the name is shortened
		}
	}
	skipped = slices.Clone(skipped)
	for i, sk := range skipped {
		if sk.Ref.Loc != "" && !copying[repoKey(sk.Ref)] {
			skipped[i].Note = true
		}
	}
	if len(out) == 0 && len(sources) > 0 {
		skipped = append(skipped, nothingCopiedNote(domain))
	}
	return out, skipped
}

// repoKey names a source the way item rows name their home: "" for the domain
// path, the named repository's id otherwise.
func repoKey(ref domainRepoRef) string {
	if ref.Own {
		return ""
	}
	return ref.Named.ID
}

// copyingRepos are the sources, by repoKey, that at least one item is copied
// from. The containers domain path also copies while the default does, because
// project folders live there and follow it.
func (r replicationPass) copyingRepos() map[string]bool {
	out := map[string]bool{}
	for _, it := range r.items {
		if it.Kind.copySource() && len(r.p.effectiveTargets(it.Identity)) > 0 {
			out[it.RepoID] = true
		}
	}
	if r.p.Domain == "containers" && len(r.p.targetsFor(r.p.defaultSkip())) > 0 {
		out[""] = true
	}
	return out
}

// pauseOnFirstListing looks at what a domain that never replicated finds when a
// target is listed for it the first time. The domain's history already at the
// target, or in a source and older than this database, means the database was
// rebuilt and the rules that kept items away are gone, so the domain pauses
// until its default is confirmed. Once that default is confirmed, this check
// stays out of it for good: the confirmation is what ends the pause, and a
// target still unlisted after it must not start a new one.
func (s *Service) pauseOnFirstListing(ctx context.Context, settings store.Settings, r replicationPass, targets []store.OffsiteTarget, sources []domainRepoRef) (bool, error) {
	if !validPlacementDomain(r.p.Domain) {
		return false, nil
	}
	if r.p.State.HasDefault && !r.p.State.Default.Paused() {
		return false, nil
	}
	var first []store.OffsiteTarget
	for _, t := range targets {
		if _, listed := r.listed[t.ID]; t.ID != "" && !listed {
			first = append(first, t)
		}
	}
	if len(first) == 0 {
		return false, nil
	}
	if never, err := s.neverReplicated(r.p.Domain); err != nil || !never {
		return false, err
	}
	for _, t := range first {
		held, err := s.listTarget(ctx, settings, t)
		if err != nil {
			continue // the pass lists it again and reports the failure there
		}
		s.recordListing(r.p.Domain, t, r.owners, held, nil)
		if r.owners.ownsAny(held) {
			return true, s.pausePlacement(ctx, r.p.Domain, "found-history")
		}
	}
	return s.pauseOnOlderSources(ctx, settings, r, sources)
}

// neverReplicated reports whether the domain has no successful off-site run in
// this database.
func (s *Service) neverReplicated(domain string) (bool, error) {
	_, offsite, err := s.store.DomainHasHistory(domain)
	if err != nil {
		return false, fmt.Errorf("%w: %v", errPlacementUnreadable, err)
	}
	return !offsite, nil
}

// pauseOnOlderSources pauses the domain when a source holds a snapshot of it
// older than this database.
func (s *Service) pauseOnOlderSources(ctx context.Context, settings store.Settings, r replicationPass, sources []domainRepoRef) (bool, error) {
	born, err := s.store.DatabaseBornAt()
	if err != nil {
		return false, fmt.Errorf("%w: %v", errPlacementUnreadable, err)
	}
	for _, src := range sources {
		snaps, err := s.listSnapshots(ctx, src.Loc, s.primaryModeFor(settings, r.p.Domain, src.Loc))
		if err != nil {
			continue // an unreadable source is no finding; the copy reports it
		}
		owners := r.owners.owners(snaps)
		for _, sn := range snaps {
			at := parseSnapshotTime(sn.Time)
			if len(owners[sn.ID].Possible) > 0 && !at.IsZero() && at.Before(born) {
				return true, s.pausePlacement(ctx, r.p.Domain, "found-history")
			}
		}
	}
	return false, nil
}

// ownsAny reports whether one of the snapshots belongs to the domain.
func (c ownerContext) ownsAny(snaps []restic.Snapshot) bool {
	owners := c.owners(snaps)
	return slices.ContainsFunc(snaps, func(sn restic.Snapshot) bool { return len(owners[sn.ID].Possible) > 0 })
}

// listTarget lists one target with its own credentials. A local target not
// created yet holds nothing.
func (s *Service) listTarget(ctx context.Context, settings store.Settings, t store.OffsiteTarget) ([]restic.Snapshot, error) {
	dest, err := s.resolveRepo(t.Repo)
	if err != nil {
		return nil, err
	}
	if localRepoMissing(dest) {
		return nil, nil
	}
	return s.listSnapshots(ctx, dest, s.offsiteModeForTarget(settings, t))
}

// stillHeld reports whether an enabled target may still hold items of the
// source: one never listed for the domain, or one whose listing counted copies
// of them.
func (r replicationPass) stillHeld(src domainRepoRef) bool {
	enabled := map[string]bool{}
	for _, t := range r.p.enabledTargets() {
		if _, listed := r.listed[t.ID]; !listed {
			return true
		}
		enabled[t.ID] = true
	}
	names := map[string]bool{}
	for _, it := range r.items {
		if it.RepoID == repoKey(src) {
			names[it.Identity] = true
		}
	}
	for _, c := range r.copies {
		ofSource := names[c.Identity] || (src.Own && strings.HasPrefix(c.Identity, "stack:"))
		if enabled[c.TargetID] && c.SnapshotCount > 0 && ofSource {
			return true
		}
	}
	return false
}
