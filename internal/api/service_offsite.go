package api

import (
	"context"
	"errors"
	"fmt"
	"log"
	"slices"
	"strings"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/notify"
	"github.com/junkerderprovinz/bombvault/internal/progress"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/schedule"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// targetOffsiteLimits is the per-destination bandwidth cap (KiB/s). All-zero
// means unlimited. A backfilled N=1 target carries the global caps.
func targetOffsiteLimits(t store.OffsiteTarget) restic.Limits {
	return restic.Limits{
		UploadKBps:   t.LimitUpload,
		DownloadKBps: t.LimitDownload,
	}
}

// offsiteTargetsFor returns a domain's enabled off-site destinations from the
// store, in stable per-domain order (sort_order, then created_at). It is the
// plural successor to the single-repo Settings.*Offsite* columns: a backfilled
// single-off-site install yields a one-element slice, an unconfigured domain an
// empty one. A missing store (used by pure-settings unit tests) or a query error
// falls back to empty so callers apply their Settings fallback.
func (s *Service) offsiteTargetsFor(domain string) []store.OffsiteTarget {
	out, err := s.enabledOffsiteTargets(domain)
	if err != nil {
		log.Printf("api: offsite %s: list targets failed (falling back to settings): %v", domain, err) //nolint:gosec // G706: domain is a fixed literal
		return nil
	}
	return out
}

// enabledOffsiteTargets is offsiteTargetsFor with the store error returned,
// for a check that has to refuse when it cannot see every copy.
func (s *Service) enabledOffsiteTargets(domain string) ([]store.OffsiteTarget, error) {
	if s.store == nil {
		return nil, nil
	}
	targets, err := s.store.OffsiteTargetsForDomain(domain)
	if err != nil {
		return nil, err
	}
	var out []store.OffsiteTarget
	for _, t := range targets {
		if t.Enabled {
			out = append(out, t)
		}
	}
	return out, nil
}

// offsiteReplicationTargets is the destination set copyToOffsite replicates
// a domain to: the domain's enabled off-site targets or, when no target row
// exists (an install configured only through the legacy Settings columns, or
// configured after the backfill ran), a single target synthesized from those
// columns, so N=1 behaves the same whether or not the row was backfilled.
func (s *Service) offsiteReplicationTargets(domain string, settings store.Settings) []store.OffsiteTarget {
	return orSettingsOffsiteTarget(s.offsiteTargetsFor(domain), domain, settings)
}

// orSettingsOffsiteTarget is targets, or when there are none, the one target
// the legacy Settings columns configure, if any.
func orSettingsOffsiteTarget(targets []store.OffsiteTarget, domain string, settings store.Settings) []store.OffsiteTarget {
	if len(targets) > 0 {
		return targets
	}
	loc := offsiteRepoFromSettings(domain, settings)
	if loc == "" {
		return nil
	}
	return []store.OffsiteTarget{settingsOffsiteTarget(domain, settings, loc)}
}

// settingsOffsiteTarget synthesizes the off-site target the backfill would
// have produced from the legacy Settings.*Offsite* columns, so the target
// loop keeps single-destination behavior for an install that was never
// backfilled. StorageClass stays "" because the global S3 class lives in
// CloudCreds and is supplied by ModeFor, and "" preserves it (see the
// per-target mode in copyToOffsiteTarget).
func settingsOffsiteTarget(domain string, settings store.Settings, loc string) store.OffsiteTarget {
	return store.OffsiteTarget{
		Domain:               domain,
		Name:                 "Primary",
		Repo:                 loc,
		StorageClass:         "",
		Immutable:            offsiteImmutableFor(domain, settings),
		Schedule:             offsiteScheduleFromSettings(domain, settings),
		RetentionKeepLast:    settings.OffsiteRetentionKeepLast,
		RetentionKeepDaily:   settings.OffsiteRetentionKeepDaily,
		RetentionKeepWeekly:  settings.OffsiteRetentionKeepWeekly,
		RetentionKeepMonthly: settings.OffsiteRetentionKeepMonthly,
		LimitUpload:          settings.OffsiteLimitUpload,
		LimitDownload:        settings.OffsiteLimitDownload,
		GrowthBudgetGB:       settings.OffsiteGrowthBudgetGB,
		Enabled:              true,
	}
}

// offsiteRepoFor returns the configured off-site repo location for a domain,
// or "" when none is set: the first enabled off-site target's, falling back
// to the legacy Settings column when no target row exists (for N=1 the
// backfilled target's Repo equals that column).
func (s *Service) offsiteRepoFor(domain string, settings store.Settings) string {
	if ts := s.offsiteTargetsFor(domain); len(ts) > 0 {
		return ts[0].Repo
	}
	return offsiteRepoFromSettings(domain, settings)
}

// offsiteRepoFromSettings reads the legacy single-repo off-site location straight
// off the Settings columns (the fallback source for offsiteRepoFor).
func offsiteRepoFromSettings(domain string, settings store.Settings) string {
	switch domain {
	case "containers":
		return settings.ContainersOffsite
	case "vms":
		return settings.VMsOffsite
	case "flash":
		return settings.FlashOffsite
	case "config":
		return settings.ConfigOffsite
	case "files":
		return settings.FilesOffsite
	}
	return ""
}

// offsiteScheduleFor returns the per-domain off-site replication schedule.
// Empty means "replicate after every local backup"; a non-empty cadence
// means the scheduler drives replication, decoupled from backups.
//
// The cadence comes from the per-domain Settings column and nothing else.
// Settings › Schedules edits that column and the scheduler registers each
// "<domain>-offsite" cron entry from it (see offsite() in
// internal/schedule/schedule.go). If another source said "decoupled" while
// the column is blank, the coupled after-backup copy would stand down with
// no cron entry to replace it, and the domain would silently stop
// replicating (#150). An off-site target row can carry a schedule value (the
// CRUD API accepts it and a settings import restores it), but per-target
// schedules are not a feature: every target replicates on its domain's
// off-site schedule and OffsiteTargetsSection exposes no such control, so
// the row value is ignored here.
func (s *Service) offsiteScheduleFor(domain string, settings store.Settings) string {
	return offsiteScheduleFromSettings(domain, settings)
}

// offsiteScheduleFromSettings reads the legacy per-domain off-site schedule
// straight off the Settings columns (the fallback source for offsiteScheduleFor).
func offsiteScheduleFromSettings(domain string, settings store.Settings) string {
	switch domain {
	case "containers":
		return settings.ContainersOffsiteSchedule
	case "vms":
		return settings.VMsOffsiteSchedule
	case "flash":
		return settings.FlashOffsiteSchedule
	case "config":
		return settings.ConfigOffsiteSchedule
	case "files":
		return settings.FilesOffsiteSchedule
	}
	return ""
}

// offsiteImmutableFor reports whether a domain's off-site repo is flagged
// append-only (immutable). The far side (e.g. rest-server --append-only)
// enforces it; the flag changes BombVault's own behaviour: replication skips
// the off-site retention prune, and off-site delete and prune are refused.
// Unlock stays allowed, because rest-server permits lock removal in
// append-only mode and clearing a stale lock is operationally required.
func offsiteImmutableFor(domain string, s store.Settings) bool {
	switch domain {
	case "containers":
		return s.ContainersOffsiteImmutable
	case "vms":
		return s.VMsOffsiteImmutable
	case "flash":
		return s.FlashOffsiteImmutable
	case "config":
		return s.ConfigOffsiteImmutable
	case "files":
		return s.FilesOffsiteImmutable
	}
	return false
}

// The refusals for a destructive operation against a repository flagged
// append-only: credentials on this box must not be able to delete history,
// so BombVault does not even try.
//
// There are three toggles on three different cards plus the case where the
// flag cannot be read, so each refusal names the way out for its own card.
// None says "far side": a named repository (#204) can be a plain folder on a
// share, with no far side and no maintenance window to wait for.
//
// None contains a slash: every error leaving the API goes through
// scrubError, whose absolute-path regex redacts any slash-led token.
var (
	// A named repository (#204). Its toggle is on the Repositories card.
	errOffsiteAppendOnly = errors.New("this repository is append-only, so nothing here may delete from it. Turn Append-only off for it under Settings, Repositories, delete what you meant to delete, and switch it back on")
	// An off-site destination. Its toggle is on the off-site destinations card.
	errAppendOnlyOffsiteTarget = errors.New("this off-site destination is append-only, so nothing here may delete from it. Turn Append-only off for it under Settings, Off-site, delete what you meant to delete, and switch it back on")
	// A domain's own remote primary. Its toggle is in the Remote safety dialog
	// beside that domain's backup path.
	errAppendOnlyPrimaryRemote = errors.New("this repository is append-only, so nothing here may delete from it. Turn Append-only off in the Remote safety settings beside this domain's backup path, delete what you meant to delete, and switch it back on")
	// Nobody's toggle: the store could not be read. primaryIsImmutable answers
	// yes then, and sending the operator to a card to switch off a flag that may
	// not exist anywhere wastes a diagnosis on a transient failure.
	errAppendOnlyUnknown = errors.New("whether this repository is append-only could not be read, so nothing here may delete from it. That is the safe answer rather than a flag anybody set. Try again in a moment, and check the BombVault log if it keeps happening")
)

// offsiteProgressHeartbeat is how often copyToOffsite re-publishes the
// indeterminate "replicating" progress event while a copy is in flight
// (#134). It is a var so tests can shrink it.
var offsiteProgressHeartbeat = 5 * time.Second

// copyToOffsite replicates a domain's local repo to its off-site destinations
// with `restic copy` (the local repo stays primary), one target after
// another. It creates each off-site repo on first use and copies everything
// not already there (restic skips dupes, so the first run seeds history and
// later runs ship just the new snapshot). It returns the scrubbed error so
// on-demand and scheduled callers can surface it, and never logs an off-site
// location, which can embed credentials. The caller holds the domain lock.
//
// item is the snapshot name a post-backup hook replicates for; the pass then
// visits only that item's targets. "" is the whole domain.
func (s *Service) copyToOffsite(ctx context.Context, domain string, settings store.Settings, item string, localRepos []domainRepoRef, skipped []repoSkip) (err error) {
	// One read of the rules, the default and the targets, before any restic call.
	p, perr := s.readPlacement(settings, domain)
	if perr == nil && p.State.Paused() {
		log.Printf("api: offsite %s: replication is paused until the placement default is confirmed", domain) //nolint:gosec // G706: domain is a fixed literal
		return nil
	}
	targets := p.enabledTargets()
	if perr == nil && item != "" {
		targets = p.effectiveTargets(item)
	}
	if len(targets) == 0 {
		if item != "" {
			return nil
		}
		return errNoOffsiteRepo
	}
	if perr == nil && p.TargetsUncertain && p.State.HasRules() {
		perr = errTargetsUncertain
	}
	var pass replicationPass
	if perr == nil {
		pass, perr = s.newPass(settings, p)
	}
	if perr == nil {
		localRepos, skipped = pass.placeSources(domain, localRepos, skipped)
	}
	if perr == nil && len(localRepos) == 0 {
		// nothingCoveredError, not skippedError: there are no sources at all, so
		// "covered only part of this domain" would be false.
		if sErr := nothingCoveredError(skipped); sErr != nil {
			return sErr
		}
		if len(skipped) > 0 {
			log.Printf("api: offsite %s: nothing to replicate: %s", domain, strings.Join(skipNames(skipped), ", ")) //nolint:gosec // G706: domain is a fixed literal and the names are the rows' own
			return nil
		}
		return errors.New("no repository to replicate")
	}
	if perr == nil {
		var paused bool
		var pending []string
		if paused, pending, perr = s.pauseOnFirstListing(ctx, settings, pass, targets, localRepos); paused && perr == nil {
			return nil
		}
		if perr == nil && len(pending) > 0 {
			targets = slices.DeleteFunc(slices.Clone(targets), func(t store.OffsiteTarget) bool {
				return slices.Contains(pending, t.ID)
			})
			if len(targets) == 0 {
				perr = errNoTargetVisited
			}
		}
	}
	// Additive kind="offsite" row in the shared runs table (StartRun/FinishRun
	// on the reserved domain target id, like prune/verify), so the replication
	// shows up in the dashboard Activity Log and Run History. It is one row per
	// domain per call. Best-effort.
	activityRunID, aErr := s.store.StartRun(domainRunTargetID(domain), "offsite")
	if aErr != nil {
		log.Printf("api: offsite %s: could not start activity run (continuing): %v", domain, aErr) //nolint:gosec // G706: domain is a fixed literal
		activityRunID = ""
	}
	// ok is set only when every target succeeded, so an unwinding panic (with
	// the named err still nil) can't stamp a phantom successful run; the
	// deferred finish then records a failure.
	var ok bool
	defer func() {
		if activityRunID == "" {
			return
		}
		status := "failed"
		if ok {
			status = "success"
		}
		if fErr := s.store.FinishRun(activityRunID, status, "", 0, truncateRunErr(err)); fErr != nil {
			log.Printf("api: offsite %s: could not finish activity run: %v", domain, fErr) //nolint:gosec // G706: domain is a fixed literal
		}
	}()
	if perr != nil {
		err = s.failPass(domain, targets, perr)
		return err
	}
	// Publish an active "off-site replication running" indicator for this
	// domain, so the UI shows which domain is replicating, with a live
	// percentage once one is available (#159; see restic.Copy and
	// progBeginCopySink). It stays per domain so the OffsiteIndicator and the
	// dashboard need no per-target handling; each target's live percentage comes
	// from copyToOffsiteTarget's progBeginCopySink call, keyed to this same
	// "offsite:"+domain event, so sequential targets (multiTarget) share one
	// indicator.
	//
	// startedAt is progBegin's single time.Now().Unix() capture, so the
	// heartbeat below publishes the exact same instant rather than a second
	// capture that could straddle a second boundary.
	_, startedAt := s.progBegin(ctx, "offsite:"+domain, "replicate")
	defer func() { s.progEnd("offsite:"+domain, "replicate", err == nil, startedAt) }()
	// Between a snapshot's live percentage updates, and during the tree walk
	// restic does before it copies packs (which prints no percentage at all),
	// lastSeen would only be touched at progBegin. The frontend's STALE_MS (15s,
	// web/src/lib/progress.ts) then hides the "running" dashboard line once a
	// quiet stretch passes that, which a real off-site copy easily does, and the
	// operation looks as if it vanished while it is still running (#134).
	// Re-publishing an active event periodically keeps lastSeen advancing.
	//
	// lastCopy carries the most recently published per-snapshot percentage (see
	// offsiteLastCopy), and a heartbeat tick republishes that, with
	// percent/snapshotIndex/snapshotTotal, rather than a blank Percent:0
	// placeholder that would overwrite the sink's last report; on a slow
	// transfer the heartbeat would otherwise win almost every render and hide
	// the live percentage on exactly the connections it is for. Until a real
	// update lands (before the first "copy started" line, or in a quiet stretch
	// with no per-snapshot signal) the heartbeat sends the bare "still alive"
	// event.
	//
	// Shutdown is a done-channel handshake, not a bare close: closing hbDone
	// only asks the goroutine to stop, and a `select` with both hbDone and the
	// ticker ready can still pick the ticker and publish one more tick. Waiting
	// on hbStopped blocks until the goroutine has returned (any in-flight
	// Publish included), so no tick can land after progEnd's terminal Publish
	// and resurrect a stale Active:true. The stop is deferred after progEnd's,
	// so it unwinds first.
	lastCopy := &offsiteLastCopy{}
	if s.progress != nil {
		hbDone := make(chan struct{})
		hbStopped := make(chan struct{})
		defer func() {
			close(hbDone)
			<-hbStopped
		}()
		go func() {
			defer close(hbStopped)
			t := time.NewTicker(offsiteProgressHeartbeat)
			defer t.Stop()
			for {
				select {
				case <-hbDone:
					return
				case <-t.C:
					e := progress.Event{Key: "offsite:" + domain, Phase: "replicate", Active: true, StartedAt: startedAt}
					if cp, total, ok := lastCopy.get(); ok {
						e.Percent, e.SnapshotIndex, e.SnapshotTotal = cp.Percent, cp.SnapshotIndex, total
					}
					s.progress.Publish(e)
				}
			}
		}()
	}

	// Replicate to each destination best-effort: one target's failure is
	// recorded and logged but does not abort the others. The joined error
	// reaches on-demand and scheduled callers so a failure still reports and
	// notifies. multiTarget is false for a single-destination domain, whose
	// budget stays on the domain path (source "offsite", the global
	// OffsiteGrowthBudgetGB, latch keyed by domain); only with 2+ destinations
	// does each carry its own growth budget, size sample and latch.
	//
	// It counts the domain's enabled targets, not the targets this pass visits
	// (which a hook may narrow to one): a narrowed pass on a multi-destination
	// domain still has to sample and budget-check under that target's own
	// source, not the domain's bare "offsite" one, which resolves to the first
	// enabled target regardless of which one this pass reached.
	multiTarget := len(p.enabledTargets()) > 1
	var errs []error
	for _, t := range targets {
		visit, open := pass.visit(t)
		if !open {
			log.Printf("api: offsite %s: %s held nothing of this domain and gets nothing; not opened", domain, placementTargetName(t)) //nolint:gosec // G706: domain is a fixed literal, the name is the row's own
			continue
		}
		// The skip list travels with the sources. It names repositories this domain
		// uses that no source in localRepos could speak for, and the retention
		// decision at the far end needs it as much as the error does: aging a
		// destination under the off-site keep-policy while a repository feeding it
		// was never opened prunes the copy of exactly the items whose other copy is
		// the unreachable one. Reporting it only at the end (skippedError, below)
		// reaches the run row after every target's retention has already run.
		if cerr := s.copyToOffsiteTarget(ctx, domain, settings, t, localRepos, skipped, multiTarget, startedAt, lastCopy, visit); cerr != nil {
			log.Printf("api: offsite %s: copy to a destination failed (continuing): %v", domain, cerr) //nolint:gosec // G706: domain is a fixed literal
			errs = append(errs, cerr)
		}
	}
	if sErr := skippedError("this replication", skipped); sErr != nil {
		errs = append(errs, sErr)
	}
	if len(errs) > 0 {
		// Joined before the deferred bookkeeping reads it, so the run row does not
		// say "success" while the same pass tells the operator it covered only part
		// of the domain.
		err = errors.Join(errs...)
		return err
	}
	ok = true
	return nil
}

// copyToOffsiteTarget replicates a domain's local repo to a single off-site
// destination and records that destination's own offsite_runs row (stamped
// with offsite_target_id). Destination repo, restic mode (S3 storage class),
// retention, bandwidth limits and append-only flag are all per target.
// Bookkeeping is best-effort; it returns the scrubbed copy error. startedAt
// is copyToOffsite's single progBegin capture, so this target's live
// copy-progress events (see progBeginCopySink) carry the same StartedAt as
// the domain-level begin, heartbeat and terminal events. lastCopy is
// copyToOffsite's shared heartbeat state (see offsiteLastCopy), updated with
// every real percentage this target reports; nil means no heartbeat is
// watching, which offsiteLastCopy's nil-safe methods handle.
//
// skipped lists the repositories this domain uses that never made it into
// localRepos. The caller folds it into the returned error once for the
// whole pass, but the retention decision below has to see it, because a
// source dropped before the loop leaves no copyErr behind and would
// otherwise look like a destination that is fully in sync.
//
// visit is what the pass decided for this target: whether the rules choose
// the ids, and whether what the target holds is recorded.
func (s *Service) copyToOffsiteTarget(ctx context.Context, domain string, settings store.Settings, target store.OffsiteTarget, localRepos []domainRepoRef, skipped []repoSkip, multiTarget bool, startedAt int64, lastCopy *offsiteLastCopy, visit targetVisit) (err error) {
	// Persist this destination's replication attempt to the off-site run
	// history (begin now, close via defer with the outcome and scrubbed error).
	// The row holds duration and outcome only: the live per-snapshot percentage
	// fed to progBeginCopySink (#159) is a real-time SSE signal, and a finished
	// run's duration says as much after the fact. Bookkeeping is best-effort: a
	// store error is logged, never fatal. offsite_target_id attributes the run
	// to this destination (empty for a settings-synthesized N=1 target).
	runStarted := time.Now().Unix()
	runID, recErr := s.store.RecordOffsiteRunForTarget(domain, target.ID, runStarted)
	if recErr != nil {
		log.Printf("api: offsite %s: could not record replication run (continuing): %v", domain, recErr) //nolint:gosec // G706: domain is a fixed literal
		runID = 0
	}
	var ok, agingOnly bool
	defer func() {
		if runID == 0 {
			return
		}
		// Marked before the row turns green, so no reader sees a success that
		// counts for the currency and is none.
		if agingOnly {
			if mErr := s.store.MarkOffsiteRunAgingOnly(domain, target.ID, runStarted); mErr != nil {
				log.Printf("api: offsite %s: could not mark the run as aging only: %v", domain, mErr) //nolint:gosec // G706: domain is a fixed literal
			}
		}
		if ferr := s.store.FinishOffsiteRun(runID, ok, truncateRunErr(err)); ferr != nil {
			log.Printf("api: offsite %s: could not finish replication run: %v", domain, ferr) //nolint:gosec // G706: domain is a fixed literal
		}
	}()
	dest, rerr := s.resolveRepo(target.Repo)
	if rerr != nil {
		return fmt.Errorf("resolve off-site repo: %w", rerr)
	}
	// Relax the destination repo tree the same way every local backup relaxes
	// the primary repo (makeRepoReadable). restic, run as root, writes a local
	// repo 0700/0400, so an off-site copy on a mounted share is root-only on the
	// far side too: the share's other clients see the repo folders but nothing
	// inside them. An NFS to SMB switch exposes it: an Unassigned Devices NFS
	// mount reads the share as uid 0 and passes the 0700 dirs, while a CIFS
	// mount authenticates as an ordinary SMB user the far side denies (#138).
	// Deferred, so a copy that fails half-way, or a retention prune that writes
	// fresh index or pack files after it, is covered too;
	// makeOffsiteRepoReadable skips remote destinations and a path that is not
	// there yet.
	defer makeOffsiteRepoReadable(dest, s.cfg.DataDir)
	// Per-target restic mode carrying this destination's S3 storage class (see
	// offsiteModeForTarget: the global class is preserved for a backfilled N=1
	// target whose class is "").
	mode := s.offsiteModeForTarget(settings, target)
	if err = s.EnsureRepo(ctx, dest, mode); err != nil {
		return fmt.Errorf("ensure off-site repo: %w", err)
	}
	// Clear any stale lock a previously interrupted off-site op (replication
	// copy or integrity check) left on the destination repo, so restic copy can
	// take its lock instead of failing with "repository is already locked".
	// BombVault is the sole writer, so an existing off-site lock is always stale
	// (#29).
	s.unlockStale(ctx, dest, mode)
	// One listing of the destination serves the copy, the record of what it holds
	// and the names its keep-policy ages.
	dstSnaps, dstErr := s.listSnapshots(ctx, dest, mode)
	if dstErr != nil {
		log.Printf("api: offsite %s: could not list the destination before copying: %v", domain, scrubError(dstErr)) //nolint:gosec // G706: domain is a fixed literal, the error scrubbed here
	}
	out := s.copySources(ctx, domain, dest, mode, target, visit, localRepos, dstSnaps, dstErr, startedAt, lastCopy)
	copied, accounted, destIsASource := out.copied, out.accounted, out.destIsASource
	// Carried past the maintenance below: whatever did arrive is aged, sampled and
	// measured against the budget, and the joined error still reaches the run row.
	copyErr := errors.Join(out.errs...)
	// Gated on copyErr too: a pass that errored out before the keep-policy ran
	// must not stamp the run aging-only, or its history claims a maintenance
	// pass that never happened.
	agingOnly = visit.agingOnly && copied == 0 && copyErr == nil
	if visit.observe && dstErr == nil && !out.uncertain {
		s.recordListing(domain, target, visit.owners, dstSnaps, out.landed)
	}
	// Nothing arrived at all: the maintenance below is about what did arrive,
	// so running a forget and a prune over the destination after a completely
	// failed pass would age a replica no fresh snapshot reached.
	if copied == 0 {
		if copyErr != nil {
			err = copyErr
			return err
		}
		// No error and nothing copied means nothing was pending anywhere. The
		// destination is still a real replica that has to be sampled, so fall
		// through; the retention below decides for itself, on whether the pass was
		// error-free and whether any source answered for this domain at all.
		// `copied` alone cannot tell those apart.
		log.Printf("api: offsite %s: nothing was pending, nothing copied", domain) //nolint:gosec // G706: domain is a fixed literal
	}
	// Apply the off-site retention policy (separate from local) after a
	// successful copy, only when one is set, so an off-site repo defaults to
	// keep-everything (archive). Best-effort: a prune failure must not fail the
	// replication that already succeeded. An immutable (append-only) off-site
	// repo is never pruned from here: the far side would refuse the delete
	// anyway, and it enforces retention itself.
	settled := false
	switch {
	case destIsASource:
		// The destination holds a repository this domain backs up to. Whatever the
		// other sources managed, a forget plus prune here deletes snapshots whose
		// only copy is the thing being pruned. Refused, and logged, because the run
		// is otherwise reported as a partial success.
		log.Printf("api: offsite %s: not applying retention: this destination is itself one of the sources, so its snapshots have no second copy", domain) //nolint:gosec // G706: domain is a fixed literal
	case copyErr != nil:
		// Any failed source stops the retention, not only a pass where every source
		// failed. The destination is then missing exactly what the failed source
		// was carrying, and aging it under the keep-policy deletes history against
		// a replica nobody refreshed.
		//
		// The test is "the pass completed without error", not "something moved".
		// Gating on movement would never apply the policy on an all-named domain
		// after an in-sync pass, the strongest evidence that the far side is
		// current; gating on "every source failed" would let one failure out of two
		// read as a partial success and age the destination.
		log.Printf("api: offsite %s: not applying retention: a source could not be copied this pass", domain) //nolint:gosec // G706: domain is a fixed literal
	case len(unreachableSkips(skipped)) > 0:
		// A source that never reached the loop. offsiteReplicationSources puts a
		// repository that was established and is now unreachable into the skip list
		// rather than localRepos, so it produces no copyErr and the case above
		// cannot see it, yet the destination is missing exactly what that source
		// was carrying.
		//
		// Unreachable, not "actionable": the question is whether the destination
		// might be the last copy of something, and only a repository this box could
		// not open raises it. A repository the operator switched off still holds
		// its data; refusing on it would stop the domain's off-site retention for
		// good on installs where the post-backup hook is the only replication.
		log.Printf("api: offsite %s: not applying retention: %s could not be reached this pass", domain, strings.Join(skipNames(unreachableSkips(skipped)), ", ")) //nolint:gosec // G706: domain is a fixed literal and the names are the rows' own
	case accounted == 0:
		// Nothing answered for this domain. Every source either holds none of it or
		// was dropped, so there is no evidence that what the destination holds
		// still exists anywhere else, and a tag-scoped forget plus prune would run
		// against what may be the last copy.
		log.Printf("api: offsite %s: not applying retention: no source could account for this domain's snapshots this pass", domain) //nolint:gosec // G706: domain is a fixed literal
	case target.Immutable:
		log.Printf("api: offsite %s: retention is enforced far-side (append-only)", domain) //nolint:gosec // G706: domain is a fixed literal
	case agingOnly && visit.aged:
		log.Printf("api: offsite %s: %s was aged under these rules already; listed only", domain, placementTargetName(target)) //nolint:gosec // G706: domain is a fixed literal, the name is the row's own
	default:
		settled = s.ageTarget(ctx, domain, dest, mode, target, visit, dstSnaps, dstErr, out.landed)
	}
	s.noteAged(domain, target, visit, agingOnly, settled, len(out.landed) > 0)
	// Sample the off-site repo size into the repo_stats series and evaluate the
	// growth budget. With a budget set, sample synchronously first so the check
	// sees this replication's fresh size, including on the very first
	// replication, which has no prior sample; for an immutable repo (no
	// far-side prune) the budget is the only growth backstop, so it must not lag
	// or miss the seed. Without a budget, sample in the background (throttled)
	// for the Storage card. The REST protocol can't see the far side's free
	// space, only BombVault's own growth, so the budget is a detection aid, not
	// a hard cap.
	//
	// With a single destination the budget stays on the domain path: the size
	// is sampled under source "offsite", the global OffsiteGrowthBudgetGB
	// drives it, and the latch is keyed by domain. With 2+ destinations each
	// carries its own budget: the size is sampled under the per-target source
	// ("offsite:<id>"), the threshold is the target's GrowthBudgetGB, and the
	// latch is keyed by (domain,targetID).
	if !multiTarget {
		if settings.OffsiteGrowthBudgetGB > 0 {
			if serr := s.CollectStats(ctx, domain, "offsite"); serr != nil {
				log.Printf("api: offsite %s: budget size sample failed (replica is safe): %v", domain, serr) //nolint:gosec // G706: domain is a fixed literal
			}
		} else {
			s.CollectStatsAsync(domain, "offsite")
		}
		s.checkOffsiteBudget(ctx, domain, settings)
		if copyErr != nil {
			err = copyErr
			return err
		}
		ok = true
		return nil
	}
	statSource := offsiteStatSource(target.ID)
	if target.GrowthBudgetGB > 0 {
		if serr := s.CollectStats(ctx, domain, statSource); serr != nil {
			log.Printf("api: offsite %s: budget size sample failed (replica is safe): %v", domain, serr) //nolint:gosec // G706: domain is a fixed literal
		}
	} else {
		s.CollectStatsAsync(domain, statSource)
	}
	s.checkOffsiteBudgetForTarget(ctx, domain, target)
	if copyErr != nil {
		err = copyErr
		return err
	}
	ok = true
	return nil
}

// offsiteStatSource maps an off-site destination id to the repo_stats source
// that samples its size: bare "offsite" for a settings-synthesized target
// (empty id), so an un-backfilled N=1 install keeps sampling under
// "offsite", or "offsite:<id>" for a real per-target destination.
func offsiteStatSource(targetID string) string {
	if targetID == "" {
		return "offsite"
	}
	return offsiteSourcePrefix + targetID
}

// offsiteBudgetLatchKey keys the over-budget latch per (domain,targetID) so each
// destination alarms independently. The NUL separator keeps it unambiguous.
func offsiteBudgetLatchKey(domain, targetID string) string {
	return domain + "\x00" + targetID
}

// checkOffsiteBudgetForTarget is checkOffsiteBudget for one off-site
// destination: it compares that destination's latest sampled size
// (repo_stats source "offsite:<id>") against the target's own
// GrowthBudgetGB and notifies once on each false→true crossing, latched per
// (domain,targetID). Only multi-destination domains use it; a
// single-destination domain stays on checkOffsiteBudget.
func (s *Service) checkOffsiteBudgetForTarget(ctx context.Context, domain string, target store.OffsiteTarget) {
	s.checkGrowthBudget(ctx, domain, offsiteStatSource(target.ID), offsiteBudgetLatchKey(domain, target.ID), target.GrowthBudgetGB, "off-site")
}

// checkOffsiteBudget compares the latest sampled off-site repo size for a domain
// against the configured growth budget (OffsiteGrowthBudgetGB, 0 = off) and fires
// a notification once on each false→true crossing. The latch (offsiteOverBudget)
// clears when growth drops back under budget so a later breach re-alarms. It reads
// the newest repo_stats row for domain+source="offsite"; if none exists yet (the
// async sample hasn't landed on the very first replication) it simply skips.
func (s *Service) checkOffsiteBudget(ctx context.Context, domain string, settings store.Settings) {
	s.checkGrowthBudget(ctx, domain, "offsite", domain, settings.OffsiteGrowthBudgetGB, "off-site")
}

// checkPrimaryRemoteBudget is checkOffsiteBudget's counterpart for a
// domain's remote primary (#152): when repo is remote and the domain has a
// saved primary-remote safety config (primaryRemoteTarget) with a growth
// budget, it measures the repo, which is the primary itself with no
// separate off-site copy, and compares against that config's own
// GrowthBudgetGB. The latch key is "primary:"-prefixed, so a domain that
// also has an off-site budget alarms for each independently. A local
// primary, or a remote one with no saved safety config, is a silent no-op
// like every other primary-remote safety feature when unconfigured.
func (s *Service) checkPrimaryRemoteBudget(ctx context.Context, domain, repo string, settings store.Settings) {
	if !restic.IsRemoteRepo(repo) {
		return
	}
	// …and it has to be the primary. Every call site passes the repository the
	// item it just backed up uses, which can be a named repository (#204),
	// while everything below is about the domain's own: the budget comes from
	// the primary-remote row, the alarm names the primary and the latch is
	// keyed "primary:"+domain. Charging a named repository's size to that
	// budget would alarm about a repository that is not the primary, and with
	// several items on different named repositories the latch would flap
	// between their sizes.
	own, oErr := s.repoFor(settings, domain, "local")
	if oErr != nil || !sameRepoLocation(own, repo) {
		return
	}
	t, ok := s.primaryRemoteTarget(domain)
	if !ok || !t.Enabled || t.GrowthBudgetGB <= 0 {
		return
	}
	// Measure synchronously, so the very first remote-primary backup after the
	// safety settings are saved is judged against a fresh size rather than a
	// stale or missing sample.
	//
	// One restic run, `stats --mode raw-data`, because the check reads exactly
	// one number. A full CollectStats sample also runs `snapshots` and the
	// expensive `stats --mode restore-size` (which walks every snapshot tree)
	// against the remote repo for every container of every round, and writes a
	// repo_stats row: N rows a night in a series the Storage card plots as one
	// point per day, closing the 20-hour throttle on the real sample (#189).
	// The primary repository is described by primaryModeFor: a named row at
	// that location carries its own credentials, storage class and caps, and a
	// size probe opened with the shared set answers about the wrong account.
	settingsMode := s.primaryModeFor(settings, domain, repo)
	raw, serr := s.engine.Stats(ctx, repo, "raw-data", settingsMode)
	if serr != nil {
		log.Printf("api: primary-remote %s: budget size measurement failed (backup is safe): %v", domain, serr) //nolint:gosec // G706: domain is a fixed literal
		return
	}
	// Compared directly rather than round-tripped via the store, which also
	// makes the first-backup case work: there is no "no sample yet".
	s.applyGrowthBudget(ctx, domain, "primary:"+domain, t.GrowthBudgetGB, "primary", raw.TotalSize)
}

// checkGrowthBudget is the shared compare-latch-notify core behind
// checkOffsiteBudget and checkOffsiteBudgetForTarget: it reads the newest
// repo_stats sample for domain+source, compares it against budgetGB (<=0 is
// off), and fires notifyOverBudget once per false→true crossing of the
// latchKey latch in offsiteOverBudget. The map is shared, so each caller
// passes its own collision-free key (offsiteBudgetLatchKey for a
// multi-target destination, the bare domain for the single off-site budget,
// a "primary:"-prefixed key for a remote primary) and a domain's off-site
// and primary budgets alarm independently. kind ("off-site" | "primary")
// only selects the wording; a missing sample (async collection has not
// landed, or the domain was never backed up) is skipped.
func (s *Service) checkGrowthBudget(ctx context.Context, domain, source, latchKey string, budgetGB int, kind string) {
	if budgetGB <= 0 {
		return // budget disabled
	}
	stat, found, err := s.store.LatestRepoStat(domain, source)
	if err != nil {
		log.Printf("api: %s %s: budget check could not read latest sample: %v", kind, domain, err) //nolint:gosec // G706: kind/domain are fixed-whitelist values
		return
	}
	if !found {
		return // no sample yet, nothing to compare
	}
	s.applyGrowthBudget(ctx, domain, latchKey, budgetGB, kind, stat.RawSize)
}

// applyGrowthBudget is checkGrowthBudget's compare-latch-notify half, for a
// caller that has just measured a size and should not write it to
// repo_stats and read it back (checkPrimaryRemoteBudget). The row is a
// user-visible series, not a scratch pad, and the budget check needs a
// number now rather than a sample.
func (s *Service) applyGrowthBudget(ctx context.Context, domain, latchKey string, budgetGB int, kind string, rawSize int64) {
	if budgetGB <= 0 {
		return // budget disabled
	}
	budgetBytes := int64(budgetGB) * 1024 * 1024 * 1024
	over := rawSize > budgetBytes

	// Latch the state under the mutex and detect the false→true crossing so the
	// alarm fires exactly once per breach (not on every backup/replication while
	// over).
	s.budgetMu.Lock()
	if s.offsiteOverBudget == nil {
		s.offsiteOverBudget = map[string]bool{}
	}
	prev := s.offsiteOverBudget[latchKey]
	s.offsiteOverBudget[latchKey] = over
	s.budgetMu.Unlock()

	if over && !prev {
		s.notifyOverBudget(ctx, domain, rawSize, budgetBytes, kind)
	}
}

// notifyOverBudget sends a best-effort alert when a domain's off-site repo
// or remote primary (kind: "off-site" | "primary") first crosses its growth
// budget. It mirrors notifyProtectionLost/notifyDrillFailure's policy gate
// and Unraid fan-out; a no-op when notifications are off.
func (s *Service) notifyOverBudget(ctx context.Context, domain string, size, budget int64, kind string) {
	c, err := s.NotifyConfig()
	if err != nil || c.On == "" || c.On == "never" {
		return
	}
	subject := strings.ToUpper(kind[:1]) + kind[1:] + " backup over budget for " + domain
	action := "Prune the far side or raise the budget."
	if kind == "primary" {
		action = "Adjust the retention policy, prune it, or raise the budget."
	}
	msg := fmt.Sprintf("The %s repository for %s has grown to %s, over the configured growth budget of %s. %s", kind, domain, humanBytes(size), humanBytes(budget), action)
	notify.Send(ctx, c, domain, notify.Event{Title: "BombVault", Message: subject + ": " + msg, OK: false})
	if s.unraidGate(c.Unraid) {
		if e := s.sendUnraidNotify(ctx, "BombVault: "+subject, msg, "warning"); e != nil {
			log.Printf("notify: unraid: %v", e)
		}
	}
}

// offsiteReplicatesOnOwnSchedule reports whether a domain's off-site copy is
// driven by its own cron entry (a real, enabled cadence) rather than coupled
// to the backup run. A blank schedule and the literal "off" both parse to a
// disabled cadence and mean "coupled": the backup path owns replication.
// ParseCadence makes this decision match the scheduler's registration (the
// off-site cron entry registers only for an enabled cadence, schedule.go),
// so a domain can never fall between the two gates and silently never
// replicate (#95). A cadence that fails to parse defaults to coupled, the
// safe direction.
func (s *Service) offsiteReplicatesOnOwnSchedule(domain string, settings store.Settings) bool {
	cad, err := schedule.ParseCadence(s.offsiteScheduleFor(domain, settings))
	return err == nil && cad.Enabled
}

// bulkReplicateSuppressKey marks a context whose post-backup inline off-site
// replication is deferred to a single batched pass after the whole scheduled
// domain loop (#95). A blank off-site schedule couples replication to each
// backup, and in a scheduled multi-item run that means a full off-site repo
// open and index reload per item (44 containers, 44 high-latency B2
// round-trips, turning seconds into hours). The scheduled multi-item
// closures (containers, VMs, files) set this flag so each item's inline
// replication is skipped and the scheduler runs one
// ReplicateOffsiteAfterBulk per domain instead. A manual "Back up now" does
// not set it, so a single-item backup still replicates immediately.
type bulkReplicateSuppressKey struct{}

// WithBulkReplicateSuppressed defers a context's inline off-site replication to a
// batched post-loop pass (see bulkReplicateSuppressKey). Set by the scheduled
// multi-item backup closures in main.go; read by replicateOffsite.
func WithBulkReplicateSuppressed(ctx context.Context) context.Context {
	return context.WithValue(ctx, bulkReplicateSuppressKey{}, true)
}

// bulkReplicateSuppressed reports whether inline off-site replication is deferred
// to a batched post-loop pass for this context.
func bulkReplicateSuppressed(ctx context.Context) bool {
	v, _ := ctx.Value(bulkReplicateSuppressKey{}).(bool)
	return v
}

// ReplicateOffsiteAfterBulk runs one off-site replication for a domain after
// a scheduled multi-item backup loop, replacing the per-item inline
// replications that run suppressed (#95). It is a no-op when the domain has
// no off-site repo, or when it replicates on its own off-site schedule (that
// path fires from its own cron entry). Like ScheduledReplicateOffsite it
// takes the domain lock, so it runs after the last item's backup releases
// it, and notifies on failure: a scheduled replication that failed silently
// would let the off-site copy rot unseen.
func (s *Service) ReplicateOffsiteAfterBulk(ctx context.Context, domain string) {
	settings, err := s.store.GetSettings()
	if err != nil {
		log.Printf("api: offsite %s: batched replicate: read settings: %v", domain, err) //nolint:gosec // G706: domain is a fixed literal
		return
	}
	if s.offsiteRepoFor(domain, settings) == "" {
		return // no off-site configured for this domain
	}
	if cadence := s.offsiteScheduleFor(domain, settings); s.offsiteReplicatesOnOwnSchedule(domain, settings) {
		// The domain's own "<domain>-offsite" cron entry replicates instead; the
		// scheduler registers it from this cadence. Log it, because a scheduled run
		// that backs up and prunes but never replicates otherwise looks like a
		// broken one in the activity log (#150).
		log.Printf("api: offsite %s: batched replicate skipped: this domain replicates on its own off-site schedule (%q)", domain, cadence) //nolint:gosec // G706: domain is a fixed literal, cadence is %q-quoted
		return
	}
	if err := s.ScheduledReplicateOffsite(ctx, domain); err != nil {
		log.Printf("api: offsite %s: batched replicate failed: %v", domain, err) //nolint:gosec // G706: domain is a fixed literal
	}
}

// replicateOffsite runs right after a successful local backup (the caller
// holds the domain lock). It replicates only when the domain has no
// separate off-site schedule: a blank schedule couples replication to each
// backup, a set one hands it to the scheduler. Best-effort: the local
// backup has already succeeded, so an off-site failure is logged, never
// propagated.
//
// localRepo is the repository the backup just wrote, which with named
// repositories (#204) need not be the domain's own, and copying exactly
// that one is the point: the hook exists to get the fresh snapshot off the
// box, and only that repository has it. The manual and scheduled paths
// cover the whole domain (offsiteReplicationSources), so the two agree.
//
// A remote named repository is skipped for the same reason it is skipped
// there: it is already off site, and one restic process cannot hold two
// clouds' credentials at once.
//
// item is the snapshot name of what the backup just wrote; the hook copies
// only to that item's targets. Flash and config pass "".
func (s *Service) replicateOffsite(ctx context.Context, domain string, settings store.Settings, localRepo, item string) {
	if bulkReplicateSuppressed(ctx) {
		return // scheduled multi-item run: replicated once after the whole loop (#95)
	}
	if s.offsiteRepoFor(domain, settings) == "" {
		return
	}
	if s.offsiteReplicatesOnOwnSchedule(domain, settings) {
		return // replicated on its own schedule, not after every backup
	}
	// The reference, not the bare string. This hook gets whatever repository
	// the item it just backed up uses, so it is the caller most likely to hold
	// a named repository, and the copy has to know that to narrow it to this
	// domain's snapshots. Deciding by position would never narrow here: a
	// one-element slice is always "index 0".
	ref := s.refFor(settings, domain, localRepo)
	if alreadyOffSite(ref) {
		log.Printf("api: offsite %s: this item's repository %s; not copied again", domain, offSiteReason(ref)) //nolint:gosec // G706: domain is a fixed literal
		return
	}
	// The skip list of the whole domain, even though this hook copies exactly
	// one repository. It is not a report (the hook only logs); the
	// destination's retention needs it.
	//
	// The retention at the far end is tag-scoped per identity over whatever the
	// destination holds, so a pass that opened one source ages the off-site
	// copies of every item in the domain. That is harmless while the other
	// sources are merely not copied this pass: they still exist. It is not
	// harmless when one of them is gone, because then the only copies of that
	// item's history are trimmed under the off-site keep-policy with nothing
	// left to restore them from. On an install with no separate off-site
	// schedule this hook is the replication, so it needs the same protection
	// as the whole-domain pass. Suppressing retention here outright would be
	// the other mistake: those installs would never age the destination at all.
	//
	// Narrowed to the unreachable ones, because the list also becomes the
	// pass's error, and a pass that copies one repository on purpose has no
	// business reporting that it did not cover the others. The whole list
	// would mark the domain's off-site run red after every backup on an install
	// with one switched-off repository.
	_, allSkips := s.offsiteReplicationSources(settings, domain)
	skipped := unreachableSkips(allSkips)
	if err := s.copyToOffsite(ctx, domain, settings, item, []domainRepoRef{ref}, skipped); err != nil {
		// domain is a fixed literal; the error is already path-scrubbed by restic.
		log.Printf("api: offsite %s: copy failed (local backup is safe): %v", domain, err)
		// The hook reports nothing else, so a replication that stopped over the
		// rules would otherwise go unnoticed.
		if errors.Is(err, errPlacementUnreadable) || errors.Is(err, errTargetsUncertain) {
			s.notifyReplicationFailed(ctx, domain, truncateRunErr(err))
		}
	}
}

// ReplicateOffsite replicates a domain's local repo to its off-site repo on
// demand, for the "Replicate now" button and the scheduled off-site job.
// Unlike the post-backup hook it surfaces the error (so the UI can report
// it) and takes the domain lock to serialise with backups.
//
// It copies every repository the domain's items write to (#204), not just
// the domain's own: an item pointed at a local named repository is part of
// this domain, and a "Replicate now" that quietly left it out would report
// success over a gap. See offsiteReplicationSources for which repositories
// qualify and why a remote named one does not.
func (s *Service) ReplicateOffsite(ctx context.Context, domain string) error {
	settings, err := s.store.GetSettings()
	if err != nil {
		return fmt.Errorf("read settings: %w", err)
	}
	if s.offsiteRepoFor(domain, settings) == "" {
		return errNoOffsiteRepo
	}
	defer s.lockDomainFor(domain, "replicate")()
	// The skip list goes in, so the run row this opens records it too rather
	// than being stamped a success the caller then contradicts.
	sources, skipped := s.offsiteReplicationSources(settings, domain)
	return s.copyToOffsite(ctx, domain, settings, "", sources, skipped)
}

// StartReplicateOffsite starts an on-demand off-site replication in the
// background and returns immediately (#93). A first replication of real
// data easily outlives browser and proxy timeouts, and a copy tied to the
// HTTP request context would be killed when the client gave up (504). The
// copy runs detached with its own generous ceiling; the off-site indicator
// shows it running, the run is recorded like any other, and a failure
// notifies like a scheduled replication. Configuration errors and a busy
// domain still surface synchronously so the UI can show them.
func (s *Service) StartReplicateOffsite(domain string) error {
	settings, _, err := s.domainRepoSource(domain, "local")
	if err != nil {
		return err
	}
	if s.offsiteRepoFor(domain, settings) == "" {
		return errNoOffsiteRepo
	}
	unlock, ok := s.tryLockDomainFor(domain, "replicate")
	if !ok {
		return errDomainBusy
	}
	go func() {
		defer s.recoverOperation("offsite replicate: "+domain, nil, func(msg string) {
			s.failStuckRun(domainRunTargetID(domain), msg)
		})
		defer unlock()
		// Detached from the HTTP request; the ceiling only guards against a copy
		// that hangs forever on a dead link.
		ctx, cancel := context.WithTimeout(context.Background(), 24*time.Hour)
		defer cancel()
		settings, err := s.store.GetSettings()
		if err != nil {
			log.Printf("api: offsite %s: replicate start: %v", domain, err) //nolint:gosec // G706: domain is a fixed literal
			return
		}
		sources, skipped := s.offsiteReplicationSources(settings, domain)
		err = s.copyToOffsite(ctx, domain, settings, "", sources, skipped)
		if err != nil {
			log.Printf("api: offsite %s: manual replication failed: %v", domain, err) //nolint:gosec // G706: domain is a fixed literal
			s.notifyReplicationFailed(ctx, domain, truncateRunErr(err))
		}
	}()
	return nil
}

// ScheduledReplicateOffsite runs a scheduled off-site replication and,
// unlike the interactive ReplicateOffsite (whose error the UI shows
// directly), notifies on failure. A scheduled replication that failed
// silently would let the off-site copy rot until the scorecard's currency
// check caught it. Best-effort notify; the error is still returned for the
// scheduler log.
func (s *Service) ScheduledReplicateOffsite(ctx context.Context, domain string) error {
	// Detached upper bound so a copy wedged on a dead link can't hold the
	// domain lock forever and block later backups (like the manual
	// StartReplicateOffsite ceiling). SkipIfStillRunning stops the next run
	// from piling on; this stops the current one from hanging indefinitely.
	ctx, cancel := context.WithTimeout(ctx, 24*time.Hour)
	defer cancel()
	err := s.ReplicateOffsite(ctx, domain)
	if err != nil {
		s.notifyReplicationFailed(ctx, domain, truncateRunErr(err))
	}
	return err
}

// notifyReplicationFailed sends a best-effort alert when a scheduled off-site
// replication fails (the off-site copy is not current). Mirrors
// notifyOverBudget/notifyProtectionLost's policy gate + Unraid fan-out; a no-op
// when notifications are off.
func (s *Service) notifyReplicationFailed(ctx context.Context, domain, detail string) {
	c, err := s.NotifyConfig()
	if err != nil || c.On == "" || c.On == "never" {
		return
	}
	subject := "Off-site replication FAILED for " + domain
	msg := fmt.Sprintf("The scheduled off-site replication for %s failed, so the off-site copy is not current: %s", domain, detail)
	notify.Send(ctx, c, domain, notify.Event{Title: "BombVault", Message: subject + ": " + msg, OK: false})
	if s.unraidGate(c.Unraid) {
		if e := s.sendUnraidNotify(ctx, "BombVault: "+subject, msg, "warning"); e != nil {
			log.Printf("notify: unraid: %v", e)
		}
	}
}

// TestOffsite probes a domain's off-site repo without modifying it, so the
// UI can tell the user whether the configured location is a reachable,
// initialised restic repository before relying on it. It uses the probe
// EnsureRepo uses to detect an existing repo, `restic cat config`
// (ResticEngine.RepoOpensErr), in both encryption modes, so a repo created
// under the opposite Encryption setting still counts as initialised
// (EnsureRepo reports that mismatch, not this).
//
// reachable reports whether the repo could be opened at all; initialized
// whether it is a real restic repository. A remote repo that is reachable
// but has never been replicated to fails `cat config` with restic's
// "repository does not exist", the signal isRepoUninitialized and
// listSnapshots treat as "reachable, just empty" (#117), so TestOffsite
// reports reachable=true, initialized=false, err=nil: the UI's
// "uninitialized" warning state, since BombVault creates the repo on the
// first replication (#130). Any other probe failure (a bad rclone remote
// type, wrong credentials, a backend refusing the connection) is a genuine
// reachability failure: neither reachable nor initialised, with err
// carrying the primary mode's probe failure, which the handler scrubs
// before it reaches the client. An unconfigured off-site repo for the
// domain is also an error.
func (s *Service) TestOffsite(ctx context.Context, domain string) (reachable, initialized bool, err error) {
	settings, err := s.store.GetSettings()
	if err != nil {
		return false, false, fmt.Errorf("read settings: %w", err)
	}
	loc := s.offsiteRepoFor(domain, settings)
	if loc == "" {
		return false, false, errNoOffsiteRepo
	}
	repo, err := s.resolveRepo(loc)
	if err != nil {
		return false, false, err
	}
	return s.probeOffsiteRepo(ctx, repo, s.ModeFor(settings))
}

// TestOffsiteTarget runs TestOffsite's probe against one off-site
// destination addressed by id, so every additional target can be verified
// on its own; TestOffsite only probes a domain's primary target, and an
// extra destination could otherwise stay broken behind a green "Test
// connection" (#138). The target's own S3 storage class is applied
// (offsiteModeForTarget) as replication does.
//
// An unknown id is an error rather than a fallback to the primary: a test
// that silently probed something else would hide exactly that failure.
func (s *Service) TestOffsiteTarget(ctx context.Context, id string) (reachable, initialized bool, err error) {
	settings, err := s.store.GetSettings()
	if err != nil {
		return false, false, fmt.Errorf("read settings: %w", err)
	}
	target, ok, err := s.store.GetOffsiteTarget(id)
	if err != nil {
		return false, false, fmt.Errorf("read off-site target: %w", err)
	}
	if !ok {
		return false, false, errors.New("no such off-site target")
	}
	if target.Repo == "" {
		return false, false, errors.New("this off-site target has no repository configured")
	}
	repo, err := s.resolveRepo(target.Repo)
	if err != nil {
		return false, false, err
	}
	return s.probeOffsiteRepo(ctx, repo, s.offsiteModeForTarget(settings, target))
}

// probeOffsiteRepo is the shared reachable/initialised probe behind TestOffsite
// and TestOffsiteTarget: it opens an already resolved repo read-only in both
// encryption modes and classifies the outcome. See TestOffsite for the full
// contract of the three results.
func (s *Service) probeOffsiteRepo(ctx context.Context, repo string, mode restic.Mode) (reachable, initialized bool, err error) {
	// Bound each probe so a dead backend fails fast instead of hanging the
	// request (cat config over an unreachable REST server can otherwise stall).
	// Per attempt, not shared: a cold sftp connection over a VPN (Tailscale
	// tunnel plus host-key pinning) can eat a shared budget on the first try
	// and leave the second probe no time, reporting a reachable repo as
	// unreachable (#93).
	probe := func(m restic.Mode) error {
		pctx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		return s.engine.RepoOpensErr(pctx, repo, m)
	}
	primaryErr := probe(mode)
	if primaryErr == nil {
		return true, true, nil
	}
	// Opens under the opposite encryption mode → the repo exists and is reachable,
	// just created under the other Encryption setting; still reachable + initialised
	// for this probe (EnsureRepo surfaces the mismatch on the next backup).
	if probe(s.oppositeMode(mode)) == nil {
		return true, true, nil
	}
	// A reachable remote destination that simply has no repository yet (no
	// replication has run) is not a failure, the same signal listSnapshots
	// treats as "empty, not fatal" (#117). Report it as the UI's uninitialized
	// state instead (#130).
	if restic.IsRemoteRepo(repo) && isRepoUninitialized(primaryErr) {
		return true, false, nil
	}
	// Neither mode opened and it is not the uninitialized case: surface the
	// primary mode's failure (the user's configured encryption setting) instead
	// of a silent false/false, so the UI can show the real reason.
	//
	// This is the one place that holds the repository URL and the credentials it
	// was tried with at the same time, so a 401 whose cause is the two disagreeing
	// gets named here rather than guessed at downstream (#194).
	if named := restPathUserMismatch(primaryErr, repo, mode.Env); named != nil {
		return false, false, named
	}
	return false, false, primaryErr
}

// offsiteReplicationSources returns the repositories a domain's off-site
// replication copies from: the domain's own, whether local or remote, plus
// every local named repository (#204) its items point at.
//
// A remote named repository is left out, for two reasons that agree:
//
//   - It is already off site. Pointing a VM straight at b2:bucket/cold is
//     what #204 asked for; copying that bucket into a second cloud is a
//     transfer bill nobody asked for.
//   - restic copy often cannot express it. One process carries one set of
//     backend credentials (restic.Mode.Env), used for the destination; only
//     the repository password has a --from- counterpart. A named repository
//     can carry its own credential set, possibly for a different account or
//     provider, and then one of the two would be authenticated wrongly.
//
// The domain's own remote repository stays in. Its credentials are not
// guaranteed to match either: primaryModeFor applies the primary-remote
// row's own CredsRef (#182) and offsiteModeForTarget the destination
// row's, and those are independent store fields. But it is the setup
// docs/offsite-recovery.md describes, an s3 primary replicated into a
// second bucket of the same account, and the only way such an install gets
// a second copy at all. When the two rows name different credential sets,
// the copy fails loudly with an authentication error every night and
// nothing is written or deleted. Excluding it would empty the source list
// for every domain with a remote primary and no local named repository
// (flash and config always, since they have no named repositories), and
// the nightly pass would report a failure without attempting anything.
//
// A local repository is the easy case: a share on the box is exactly as
// exposed as anything else on it, needs the off-site copy just as much,
// and needs no credentials to read.
//
// A source that does not exist is dropped rather than attempted. With
// every item on a named repository, the domain's own repository leading
// the list is never created (each backup only EnsureRepo's the item's own
// location), and `restic copy` has to open its source to enumerate
// snapshots, so leading with it would fail the whole pass before the named
// repositories were attempted.
func (s *Service) offsiteReplicationSources(settings store.Settings, domain string) ([]domainRepoRef, []repoSkip) {
	repos, skipped, err := s.domainReposInUse(settings, domain)
	if err != nil || len(repos) == 0 {
		own, oErr := s.repoFor(settings, domain, "local")
		if oErr != nil {
			// The domain's path does not resolve. Reporting that as a skip keeps
			// the actionable sentence ("invalid backup path: …") instead of the
			// caller's generic "no repository to replicate", which named nothing
			// anybody could act on.
			return nil, append(skipped, repoSkip{Name: "the " + domain + " repository", Reason: scrubError(oErr), Unreachable: true})
		}
		repos = []domainRepoRef{ownRef(own)}
	}
	// Whether a named repository was among the candidates at all, before any of
	// them is checked for presence: it decides what an all-missing pass means
	// below.
	namedCandidate := slices.ContainsFunc(repos, func(r domainRepoRef) bool { return !r.Own })
	out := make([]domainRepoRef, 0, len(repos))
	for _, r := range repos {
		// A remote named repository is left out; the domain's own is not, however
		// remote. The reason is the credentials, and they follow ownership: one
		// restic process carries one set of backend credentials and copy spends
		// them on the destination. A named repository can carry a credential set
		// of its own (CredsRef) and is offered to every domain, and leaving it out
		// costs nothing, since it is already off site. See the function comment
		// for why the domain's own remote primary stays in; the post-backup hook
		// shares alreadyOffSite, so both halves answer the same way.
		if alreadyOffSite(r) {
			// Logged, not skipped. A skip becomes the operation's error, a red run row
			// and a "replication FAILED" notification, and this exclusion is permanent
			// and correct: nothing the operator can do makes b2: stop being remote, so
			// reporting it as an incomplete pass would make a supported configuration
			// fail forever over a repository that was never meant to be copied.
			log.Printf("api: offsite %s: named repository %s %s; not copied again", domain, scrubRepoLocation(r.Loc), offSiteReason(r)) //nolint:gosec // G706: domain is a fixed literal, the location has any embedded credential redacted
			continue
		}
		out = append(out, r)
	}
	// A repository that was never created holds nothing and is dropped
	// silently. A repository that was created and is now gone is reported. In
	// an all-named install the domain's own repository does not exist at all
	// (each backup only creates the item's own location), so reporting it
	// would fail the nightly run forever; and an unmounted share holding real
	// backups dropped without a word would let the run report success while
	// those items had no off-site copy.
	//
	// markRepoEstablished/repoEstablished is the record of "this location was
	// a working repository once", the same distinction #55 needed for the
	// not-mounted guard.
	present := make([]domainRepoRef, 0, len(out))
	for _, r := range out {
		if !localRepoMissing(r.Loc) {
			present = append(present, r)
			continue
		}
		// Unknown is reported, not swallowed: a transient store error here would
		// otherwise stamp the run "off-site copy is current" for a repository that
		// got no copy at all. Same rule as reposThatExist.
		switch s.repoEstablishmentOf(r.Loc) {
		case repoWasEstablished:
			skipped = append(skipped, repoSkip{Name: s.refName(r), Reason: "it was there before and is not reachable now", Unreachable: true, Ref: r})
		case repoEstablishmentUnknown:
			skipped = append(skipped, repoSkip{Name: s.refName(r), Reason: "it is not reachable now, and whether it ever held backups could not be read", Unreachable: true, Ref: r})
		case repoNeverEstablished:
		}
	}
	if len(present) == 0 {
		if !namedCandidate {
			// A domain with only its own repository, never created, keeps it, so the
			// caller still gets restic's own error rather than a silent no-op.
			return out, skipped
		}
		// A named repository was among the candidates and none of them is
		// present: that is a domain whose items are not copied, not a failure.
		return nil, append(skipped, nothingCopiedNote(domain))
	}
	return present, skipped
}

// refAppendOnly reports whether this repository is flagged append-only,
// and under which toggle, asked of the reference.
//
// It must answer exactly as primaryAppendOnly does; the reference already
// carries the named row, which saves the store read. If the two differed,
// the toggle would be honoured by some of the six gates that read it and
// not by others, and those would repack a repository the screen promises
// is protected.
//
// The flag rather than a bool, because the refusal has to name the card
// the toggle actually lives on, and there are three different cards.
func (s *Service) refAppendOnly(domain string, r domainRepoRef) appendOnlyFlag {
	if !r.Own && r.Named.ID != "" {
		if r.Named.Enabled && r.Named.Immutable {
			return appendOnlyNamedRepo
		}
		return appendOnlyNone
	}
	return s.primaryAppendOnly(domain, r.Loc)
}

// repoSharedWithAnotherDomain reports whether this repository is also written to
// by a domain other than the given one. Only a named repository can be: a
// domain's own belongs to it alone.
func (s *Service) repoSharedWithAnotherDomain(settings store.Settings, domain string, r domainRepoRef) bool {
	if r.Own || r.Named.ID == "" {
		return false
	}
	for _, d := range []string{"containers", "vms", "files"} {
		if d == domain {
			continue
		}
		others, _, err := s.domainReposInUse(settings, d)
		if err != nil {
			// Unknown counts as shared: the question is whether it is safe to force a
			// live lock away, and "I could not find out" is not a yes.
			return true
		}
		for _, o := range others {
			if !o.Own && o.Named.ID == r.Named.ID {
				return true
			}
		}
	}
	return false
}

// alreadyOffSite reports whether a source is one the off-site copy leaves
// out: a direct repository, which lies at its target already, or a
// repository that is not the domain's own and is remote.
//
// The post-backup hook and the whole-domain pass share this one predicate
// so they cannot answer differently. A non-empty Named.ID is not required:
// refFor's fallback yields Own=false with no named row when
// namedRepoForLocation's store read fails, and such a source still has to
// be left out.
//
// The reason is the credentials, not the location, and credentials follow
// ownership: see offsiteReplicationSources for why a domain's own remote
// primary stays in.
func alreadyOffSite(r domainRepoRef) bool {
	return r.Named.CompanionOf != "" || (!r.Own && restic.IsRemoteRepo(r.Loc))
}

// offSiteReason says in a log line why alreadyOffSite left a source out.
func offSiteReason(r domainRepoRef) string {
	if r.Named.CompanionOf != "" {
		return "is the direct repository of an off-site target"
	}
	return "is remote and is already off site"
}
