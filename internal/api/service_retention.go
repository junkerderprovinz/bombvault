package api

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/notify"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// retentionPolicy maps the stored settings to a restic keep-policy.
func (s *Service) retentionPolicy(settings store.Settings) restic.RetentionPolicy {
	return restic.RetentionPolicy{
		KeepLast:    settings.RetentionKeepLast,
		KeepDaily:   settings.RetentionKeepDaily,
		KeepWeekly:  settings.RetentionKeepWeekly,
		KeepMonthly: settings.RetentionKeepMonthly,
	}
}

// offsiteRetentionPolicy is the separate keep-policy for the off-site repo,
// so it can be kept longer (archive) than the local copy. All-zero (the
// default) means no off-site pruning: the off-site repo keeps everything
// until the user sets this policy.
func (s *Service) offsiteRetentionPolicy(settings store.Settings) restic.RetentionPolicy {
	return restic.RetentionPolicy{
		KeepLast:    settings.OffsiteRetentionKeepLast,
		KeepDaily:   settings.OffsiteRetentionKeepDaily,
		KeepWeekly:  settings.OffsiteRetentionKeepWeekly,
		KeepMonthly: settings.OffsiteRetentionKeepMonthly,
	}
}

// targetOffsiteRetentionPolicy is the per-destination off-site keep-policy,
// the plural successor to offsiteRetentionPolicy, which reads the single
// global columns. All-zero means keep everything, as the global default does.
// A backfilled N=1 target carries the global policy.
func targetOffsiteRetentionPolicy(t store.OffsiteTarget) restic.RetentionPolicy {
	return restic.RetentionPolicy{
		KeepLast:    t.RetentionKeepLast,
		KeepDaily:   t.RetentionKeepDaily,
		KeepWeekly:  t.RetentionKeepWeekly,
		KeepMonthly: t.RetentionKeepMonthly,
	}
}

// retentionPolicyForSource returns the keep-policy for a repo source: the
// off-site policy for any off-site source, the local policy otherwise. The
// off-site policy here is the settings-level one.
func (s *Service) retentionPolicyForSource(settings store.Settings, source string) restic.RetentionPolicy {
	if isOffsiteSource(source) {
		return s.offsiteRetentionPolicy(settings)
	}
	return s.retentionPolicy(settings)
}

// applyRetention prunes the just-backed-up item to the configured keep-policy.
// id is the item's identity: container:<name>, vm:<name>, fileset:<name> or
// the fixed flash/config tag, plus a renamed container's or VM's aliases. The
// tags retentionTagsFor allows are forgotten as one group, so neither a path
// change nor a rename leaves older snapshots in a group that never ages out.
// Best-effort: a failure never fails the backup that just succeeded, but it is
// notified rather than only logged, because silently skipped retention lets
// the repo grow unseen for weeks.
//
// During a bulk run (the #95 bulk-suppress flag on ctx: scheduled multi-item
// loops and the manual "back up all" batches) the expensive --prune is
// deferred: each item's forget runs without prune, and PruneAfterBulk reclaims
// the space once after the whole loop instead of once per item, since the
// prune pass is the costly part and a 44-container night would otherwise pay
// for it 44 times over. Single and manual backups (and flash/config, which
// never set the flag) keep the immediate inline prune.
//
// domain identifies which domain repo belongs to, so repo can be checked for
// a remote primary flagged append-only in its safety settings (#152,
// primaryIsImmutable). Retention is then skipped entirely, as
// copyToOffsiteTarget skips its pass for an immutable off-site destination:
// an immutable repository has no separate off-site copy behind it, so this
// box's own credentials must not be able to prune its only backup.
//
// "Immutable" is asked of the repository, not of the domain. A named
// repository (#204) carries its own flag and may be a plain folder on a
// share, so this applies to a local path as readily as to a cloud bucket.
// Anything with no append-only flag anywhere is unaffected.
func (s *Service) applyRetention(ctx context.Context, repo string, settings store.Settings, mode restic.Mode, id entryIdentity, domain string) {
	p := s.retentionPolicyForRef(settings, s.refFor(settings, domain, repo))
	if !p.Any() {
		return
	}
	if s.primaryIsImmutable(domain, repo) {
		// Name the repository, so the operator can tell which one was spared while
		// the rest of the domain pruned normally.
		log.Printf("api: %s: retention skipped: %s is flagged append-only", domain, shortRepoName(repo)) //nolint:gosec // G706: domain is a fixed literal and the name is shortened
		return
	}
	// restic forget without a tag would select the whole repository.
	if id.tag == "" {
		log.Printf("api: %s: retention skipped: the item has no identity tag", domain) //nolint:gosec // G706: domain is a fixed literal
		return
	}
	prune := !bulkReplicateSuppressed(ctx) // bulk run: one batched prune after the loop
	tags, ok := s.retentionTagsFor(ctx, repo, mode, id)
	if !ok {
		return // paused, and said so: nothing is forgotten
	}
	if err := s.forgetWithLockHeal(ctx, repo, p, mode, tags, prune); err != nil {
		log.Printf("api: retention prune failed (backup is safe): %v", err)
		// The current name, the one an operator recognises after a rename.
		s.notifyRetentionFailed(ctx, id.tag, truncateRunErr(err))
	}
}

// retentionTagsFor returns the tags restic forget may take for id, and false
// when the pass must not run at all. forget selects by tag and has no time
// bound, so:
//   - an alias tag joins only while every snapshot under it predates the
//     link. One that does not, or whose listing fails, is left out and its
//     pre-link snapshots are kept.
//   - an entry whose current name is another entry's alias pauses while that
//     entry's pre-link snapshots sit under the name, or when the listing
//     fails, because its forget would age them under its own policy.
//
// Either way the cost is storage, never another entry's snapshots.
func (s *Service) retentionTagsFor(ctx context.Context, repo string, mode restic.Mode, id entryIdentity) ([]string, bool) {
	if len(id.aliases) == 0 && id.ceded == nil {
		return id.listTags(), true
	}
	listed, err := s.snapshotsForTags(ctx, repo, mode, id.listTags())
	if err != nil {
		if id.ceded != nil {
			log.Printf("api: retention of %s paused: listing it failed, and it may hold another entry's snapshots: %v", id.tag, err) //nolint:gosec // G706: tags are validated names
			return nil, false
		}
		for _, a := range id.aliases {
			log.Printf("api: retention: listing %s failed, so %s is left out of its retention and its snapshots are kept: %v", id.tag, a.tag, err) //nolint:gosec // G706: tags are validated names
		}
		return []string{id.tag}, true
	}
	tags, withheld, paused := id.retentionTags(listed)
	if paused {
		owner := id.ceded.owner
		if owner == "" {
			owner = "another entry"
		}
		log.Printf("api: retention of %s paused: it still holds snapshots of %s from before that entry was renamed away from the name, and forget cannot tell them apart; they are kept", id.tag, owner) //nolint:gosec // G706: tags are validated names
		return nil, false
	}
	for _, a := range withheld {
		log.Printf("api: retention: %s has snapshots from after it was linked to %s (the name is in use again), so it is left out of that retention and its older snapshots are kept", a.tag, id.tag) //nolint:gosec // G706: tags are validated names
	}
	return tags, true
}

// forgetWithLockHeal runs a ForgetPolicy pass after clearing a genuinely
// stale lock with plain `restic unlock` (removeAll=false), which applies
// restic's own staleness rules: a dead PID on this host, or any lock past
// restic's ~30-minute age threshold. forget needs an exclusive lock, so even
// a stale non-exclusive lock, which lets backups keep succeeding, would block
// every retention pass, and one orphan would fail a whole night's retention
// across all items. A live lock is not force-removed (see the body); it has
// the same bounded prior-incarnation hostname gap noted on CheckDomain.
func (s *Service) forgetWithLockHeal(ctx context.Context, repo string, p restic.RetentionPolicy, mode restic.Mode, tags []string, prune bool) error {
	// A live or concurrent lock is not force-removed: reads run --no-lock,
	// writes are serialized under the domain lock, and forget passes
	// --retry-lock to wait out a transient cross-process lock. Force-removing a
	// live lock cannot fix a live holder and endangers a running operation.
	s.unlockStale(ctx, repo, mode)
	return s.engine.ForgetPolicy(ctx, repo, p, mode, tags, prune)
}

// identityTags returns the distinct identity tags in snaps: container:, vm:,
// fileset: and stack: names and the fixed flash and config tags. Marker tags
// such as p1, p2 and live are not identities.
func identityTags(snaps []restic.Snapshot) []string {
	seen := map[string]bool{}
	var out []string
	for _, sn := range snaps {
		for _, t := range sn.Tags {
			isIdentity := t == "flash" || t == "config" ||
				strings.HasPrefix(t, "container:") || strings.HasPrefix(t, "vm:") ||
				strings.HasPrefix(t, "fileset:") || strings.HasPrefix(t, "stack:")
			if isIdentity && !seen[t] {
				seen[t] = true
				out = append(out, t)
			}
		}
	}
	return out
}

// applyRetentionPerIdentity applies policy per identity, one tag-scoped forget
// per item, then prunes once. Used where no single item is in scope (manual
// prune, off-site retention).
//
// A failed listing forgets nothing: the repo-wide paths-grouped pass ignores
// identities and aliases, so it would age a renamed entry's pre-link snapshots
// together with a later machine's under the same name. That pass runs only for
// a repository with no identity tag at all, one written before identity tags
// existed, so its retention does not silently stop.
func (s *Service) applyRetentionPerIdentity(ctx context.Context, repo string, p restic.RetentionPolicy, mode restic.Mode) error {
	if !p.Any() {
		return nil
	}
	snaps, err := s.engine.Snapshots(ctx, repo, mode)
	if err != nil {
		return fmt.Errorf("list snapshots for retention: %w", err)
	}
	tags := identityTags(snaps)
	if len(tags) == 0 {
		return s.forgetWithLockHeal(ctx, repo, p, mode, nil, true)
	}
	return s.applyRetentionToTags(ctx, repo, p, mode, tags, snaps)
}

// applyRetentionToTags forgets per identity and prunes once, for a caller that
// already knows what the repository holds. snaps is what the tags were read
// from, which is what decides how an alias's old name folds.
func (s *Service) applyRetentionToTags(ctx context.Context, repo string, p restic.RetentionPolicy, mode restic.Mode, tags []string, snaps []restic.Snapshot) error {
	groups, _ := s.foldAliasedIdentityTags(tags, snaps) // skipped tags are logged there and kept
	var errs []error
	for _, group := range groups {
		if fErr := s.forgetWithLockHeal(ctx, repo, p, mode, group, false); fErr != nil {
			errs = append(errs, fmt.Errorf("%s: %w", strings.Join(group, ","), fErr))
		}
	}
	if pErr := s.engine.Prune(ctx, repo, mode); pErr != nil {
		errs = append(errs, pErr)
	}
	return errors.Join(errs...)
}

// foldAliasedIdentityTags groups the identity tags that belong to one entry,
// so applyRetentionPerIdentity forgets them together. A rename leaves the old
// tag on the snapshots already written; forgotten on its own, that tag never
// receives another snapshot and its "keep last N" set never shrinks.
//
// An old name can be taken up again by a different machine, and restic forget
// selects by tag, not by time. So for a tag that is an alias's old name, the
// alias's link time decides (aliasClaim):
//   - the tag holds snapshots the alias claims next to ones it does not: two
//     machines' history under one tag. It gets no group and is returned in
//     skipped, because its own pass would age the owner's pre-link snapshots
//     out. An alias lookup that fails skips the tag the same way.
//   - every snapshot under it is the alias's: it folds into the owner's
//     group, unless a row holds the name and may back up under it at any
//     time. Then it stays a group of its own, keyed apart from that row's.
//   - none is: the later machine's own group.
//
// A domain whose rows cannot be read does not fold at all, since an empty
// liveNames would switch off the reused-name check. A missed fold costs one
// extra pass; a wrong one merges two entries' retention for good.
func (s *Service) foldAliasedIdentityTags(tags []string, snaps []restic.Snapshot) (groups [][]string, skipped []string) {
	domains := s.aliasFoldDomains()
	byCanon := map[string][]string{}
	var order []string
	for _, tag := range tags {
		canon, skip := s.foldTag(tag, snaps, domains)
		if skip {
			skipped = append(skipped, tag)
			continue
		}
		if _, seen := byCanon[canon]; !seen {
			order = append(order, canon)
		}
		byCanon[canon] = append(byCanon[canon], tag)
	}
	groups = make([][]string, 0, len(order))
	for _, canon := range order {
		groups = append(groups, byCanon[canon])
	}
	return groups, skipped
}

// aliasFoldDomain is one domain's rows as foldAliasedIdentityTags needs them.
type aliasFoldDomain struct {
	domain, prefix string
	idToName       map[string]string
	liveNames      map[string]bool
	readable       bool
}

func (s *Service) aliasFoldDomains() []aliasFoldDomain {
	c := aliasFoldDomain{domain: "container", prefix: "container:", idToName: map[string]string{}, liveNames: map[string]bool{}}
	if targets, err := s.store.ListTargets(); err != nil {
		log.Printf("api: retention: listing targets for alias fold: %v; leaving every container tag as its own identity", err)
	} else {
		c.readable = true
		for _, t := range targets {
			c.idToName[t.ID] = t.ContainerName
			c.liveNames[t.ContainerName] = true
		}
	}
	v := aliasFoldDomain{domain: "vm", prefix: "vm:", idToName: map[string]string{}, liveNames: map[string]bool{}}
	if vms, err := s.store.ListVMTargets(); err != nil {
		log.Printf("api: retention: listing VMs for alias fold: %v; leaving every VM tag as its own identity", err)
	} else {
		v.readable = true
		for _, t := range vms {
			v.idToName[t.ID] = t.Name
			v.liveNames[t.Name] = true
		}
	}
	return []aliasFoldDomain{c, v}
}

// foldTag is foldAliasedIdentityTags's decision for one tag: the group it
// joins, or skip.
func (s *Service) foldTag(tag string, snaps []restic.Snapshot, domains []aliasFoldDomain) (canon string, skip bool) {
	for _, d := range domains {
		name, ok := strings.CutPrefix(tag, d.prefix)
		if !ok {
			continue
		}
		a, err := s.store.AliasByOldName(d.domain, name)
		if errors.Is(err, sql.ErrNoRows) {
			return tag, false
		}
		if err != nil {
			log.Printf("api: retention: %s left out: could not read whether it is a renamed entry's old name: %v", tag, err) //nolint:gosec // G706: tags are validated names
			return tag, true
		}
		claim := newAliasClaim(d.prefix, a)
		if claim.mixed(snaps) {
			log.Printf("api: retention: %s left out: it holds a renamed entry's snapshots from before the rename next to a later machine's, and a forget by that tag cannot keep them apart; they are kept", tag) //nolint:gosec // G706: tags are validated names
			return tag, true
		}
		if !claim.claimsEvery(snaps) {
			return tag, false
		}
		if cur := d.idToName[a.TargetID]; d.readable && !d.liveNames[name] && cur != "" {
			return d.prefix + cur, false
		}
		// Only the alias owner's snapshots, under a name a row may hold as its
		// current one. That row folds its own aliases under this tag, so this
		// group needs a key none of them can share.
		return "former:" + tag, false
	}
	return tag, false
}

// notifyRetentionFailed sends a best-effort alert when the post-backup
// retention prune fails. Mirrors notifyReplicationFailed's policy gate; a no-op
// when notifications are off.
func (s *Service) notifyRetentionFailed(ctx context.Context, tag, detail string) {
	c, err := s.NotifyConfig()
	if err != nil || c.On == "" || c.On == "never" {
		return
	}
	subject := "Retention prune FAILED for " + tag
	msg := fmt.Sprintf("Applying the retention policy for %s failed, so old snapshots are not being pruned (the new backup itself is safe): %s", tag, detail)
	notify.Send(ctx, c, tag, notify.Event{Title: "BombVault", Message: subject + ": " + msg, OK: false})
	if s.unraidGate(c.Unraid) {
		if e := s.sendUnraidNotify(ctx, "BombVault: "+subject, msg, "warning"); e != nil {
			log.Printf("notify: unraid: %v", e)
		}
	}
}

// PruneDomain reclaims repository space freed by forgotten snapshots
// (restic prune), bounded by a generous timeout since pruning a large repo
// is slow. Once the domain lock is held it publishes a "maintenance"
// progress pair (begin and terminal, indeterminate, since restic prune and
// forget stream no percentage) and records a "prune" run, so a manual or
// scheduled prune shows up on the dashboard activity log and run history
// instead of running invisibly.
func (s *Service) PruneDomain(ctx context.Context, domain, source string) error {
	return s.pruneDomain(ctx, domain, source, true)
}

// PruneAfterBulk runs one local prune for a domain after a bulk backup
// loop, replacing the per-item inline prune the bulk run deferred: under
// the #95 bulk flag applyRetention runs each item's forget without
// --prune, so the expensive space reclaim happens here once per run. It
// reuses the PruneDomain core, so the batched prune takes the domain lock
// itself (the bulk loop has released all locks by now), publishes
// maintenance progress and records a kind="prune" run, visible in Run
// History and the Activity Log like a manual prune. Local repo only:
// off-site retention stays inside copyToOffsite, and an immutable off-site
// repo is never pruned from this box. Skipped silently when no repository
// of the domain ages by a keep-policy (its own local repository, a named
// one, or a direct repository under its target's mirrored rules): nothing
// was forgotten, so there is nothing to reclaim. Best-effort: failures are
// logged, never propagated.
func (s *Service) PruneAfterBulk(ctx context.Context, domain string) {
	settings, err := s.store.GetSettings()
	if err != nil {
		log.Printf("api: prune %s: batched prune: read settings: %v", domain, err) //nolint:gosec // G706: domain is a fixed literal
		return
	}
	if !s.domainHasRetention(settings, domain) {
		return // no repository of this domain ages by a policy, so nothing was forgotten
	}
	// applyPolicy=false: the per-item tag-scoped forgets already ran inline during
	// the loop (without --prune), so this pass is a plain space-reclaim.
	if err := s.pruneDomain(ctx, domain, "local", false); err != nil {
		log.Printf("api: prune %s: batched prune failed: %v", domain, err) //nolint:gosec // G706: domain is a fixed literal
	}
}

// pruneDomain is the shared core of PruneDomain and PruneAfterBulk.
// applyPolicy selects the manual-prune semantics (a configured retention
// policy is applied: per-identity forget --keep-* then one prune, "apply
// retention now") versus a plain space reclaim (`restic prune` only, the
// batched post-bulk pass, whose per-item forgets already ran inline
// without --prune).
func (s *Service) pruneDomain(ctx context.Context, domain, source string, applyPolicy bool) (err error) {
	// Every repository this domain's items write to (#204). Prune is what
	// turns a forgotten snapshot back into free space, so pruning only the
	// domain repository would never reclaim the space retention freed on a
	// named one, and nothing would say so.
	settings, repos, skipped, err := s.domainReposForOp(domain, source)
	if err != nil {
		return err
	}
	// An immutable repo is never pruned from this box (append-only is the
	// point), and that is decided per repository:
	//   - off-site: the flag of the target the source names.
	//   - #152: the same refusal when the "local" source is a remote primary
	//     flagged append-only in its saved safety settings. There is no
	//     separate off-site copy in that shape, so refusing is the only thing
	//     standing between an on-box credential and deleting the only backup.
	//     A named repository carries its own flag, so one append-only archive
	//     among an item's repositories is skipped instead of blocking the
	//     whole domain.
	prunable := make([]domainRepoRef, 0, len(repos))
	// …and why each one was left out, so the refusal below can name the card the
	// toggle lives on. With a mixed set the first reason is the one reported: a
	// sentence per repository would be worse than one that names a place to go,
	// and the operator who clears that one comes straight back here for the next.
	refusal := error(nil)
	for _, r := range repos {
		if isOffsiteSource(source) {
			immutable, iErr := s.offsiteSourceImmutable(settings, domain, source)
			if iErr != nil {
				return iErr
			}
			if immutable {
				if refusal == nil {
					refusal = errAppendOnlyOffsiteTarget
				}
				continue
			}
		} else if f := s.refAppendOnly(domain, r); f != appendOnlyNone {
			if refusal == nil {
				refusal = appendOnlyRefusal(f)
			}
			continue
		}
		prunable = append(prunable, r)
	}
	if len(prunable) == 0 {
		if refusal == nil {
			refusal = errOffsiteAppendOnly
		}
		// The skip list travels with this refusal too. Without it an operator
		// whose repositories are all append-only and one of which was switched off
		// or unresolvable would hear only "append-only", never that a repository
		// was not considered at all.
		if sErr := skippedError("this prune", skipped); sErr != nil {
			return errors.Join(refusal, sErr)
		}
		return refusal
	}
	repos, missing, err := s.reposThatExist(prunable, "no backups to prune yet")
	if err != nil {
		return err
	}
	skipped = append(skipped, missing...)
	unlock, ok := s.tryLockDomainFor(domain, "prune")
	if !ok {
		return errDomainBusy
	}
	defer unlock()
	// Per repository, like the other three (see CheckDomain).
	ctx, cancel := context.WithTimeout(ctx, time.Duration(len(repos))*30*time.Minute)
	defer cancel()

	pkey := "prune:" + domain
	_, startedAt := s.progBegin(ctx, pkey, "maintenance")
	defer func() { s.progEnd(pkey, "maintenance", err == nil, startedAt) }()
	runID, rErr := s.store.StartRun(domainRunTargetID(domain), "prune")
	if rErr != nil {
		log.Printf("api: prune %s: could not start run record (continuing): %v", domain, rErr) //nolint:gosec // G706: domain is a fixed literal
		runID = ""
	}
	defer func() {
		if runID == "" {
			return
		}
		status := "success"
		if err != nil {
			status = "failed"
		}
		if fErr := s.store.FinishRun(runID, status, "", 0, truncateRunErr(err)); fErr != nil {
			log.Printf("api: prune %s: could not finish run record: %v", domain, fErr) //nolint:gosec // G706: domain is a fixed literal
		}
	}()

	// Clear any stale lock left by a previously interrupted run so it can't
	// block this prune: a manual prune (and forget --prune) takes restic's
	// exclusive lock, and an interrupted backup or prune leaves one behind.
	// BombVault is the sole writer, so an existing lock is always stale. Every
	// other repo-mutating path (backups, DeleteSnapshot) does the same.
	//
	// When a retention policy is configured, Prune applies it (forget
	// --keep-* --prune): it collapses snapshots per the policy and reclaims
	// space, the "apply retention now" users expect from a manual prune.
	// Without a policy it stays a plain space reclaim; forget with no
	// keep-flags would delete every snapshot, so that path is guarded by
	// p.Any(). The policy is per source: pruning the off-site repo uses the
	// off-site policy, not the local one, so an archive off-site isn't trimmed
	// to the local rules. The batched post-bulk pass skips this
	// (applyPolicy=false): its per-item forgets already ran inline, and
	// re-running them would cost 44 more exclusive-lock round-trips for
	// nothing.
	for _, r := range repos {
		rMode := s.repoModeFor(settings, domain, source, r.Loc)
		s.unlockStale(ctx, r.Loc, rMode)
		policy := restic.RetentionPolicy{}
		switch {
		case !applyPolicy:
		case isOffsiteSource(source):
			policy = s.retentionPolicyForSource(settings, source)
		default:
			policy = s.retentionPolicyForRef(settings, r)
		}
		if policy.Any() {
			// Per identity: a tag-scoped, ungrouped forget per item and one prune,
			// which also drains frozen path-groups (#91).
			if err = s.applyRetentionPerIdentity(ctx, r.Loc, policy, rMode); err != nil {
				err = fmt.Errorf("pruning %s: %w", s.refName(r), err)
				return err
			}
			continue
		}
		if err = s.engine.Prune(ctx, r.Loc, rMode); err != nil {
			err = fmt.Errorf("pruning %s: %w", s.refName(r), err)
			return err
		}
	}
	err = skippedError("this prune", skipped)
	return err
}
