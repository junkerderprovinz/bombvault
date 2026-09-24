package api

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path"
	"path/filepath"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/notify"
	"github.com/junkerderprovinz/bombvault/internal/paths"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// CheckDomain verifies the integrity of a domain's restic repo (restic check).
// domain is "containers" | "vms" | "flash" | "files". Returns a friendly error
// when the repo has not been created yet. Bounded by a timeout so a huge repo
// can't hang the request forever.
func (s *Service) CheckDomain(ctx context.Context, domain, source string) (err error) {
	// Every repository this domain's items write to (#204), not just its own.
	// Verifying the domain repository while an item's data sits in a named one
	// would give a green tick about a repository the data is not in.
	settings, repos, skipped, err := s.domainReposForOp(domain, source)
	if err != nil {
		return err
	}
	repos, missing, err := s.reposThatExist(repos, "no backups to verify yet")
	if err != nil {
		return err
	}
	skipped = append(skipped, missing...)
	// Hold the in-process domain lock for the whole verify so no other
	// BombVault op (backup, prune, replicate) runs against this repo during
	// the check. If one already holds it, report a clean "busy" instead of
	// colliding on restic's repo lock. This rules out BombVault itself as the
	// source of a lock the check hits, but not a live restic process (a manual
	// or external invocation); that is a real, live lock, not an orphan, and
	// it is left alone below (waited out by --retry-lock in the engine, never
	// force-removed).
	unlock, ok := s.tryLockDomainFor(domain, "verify")
	if !ok {
		return errDomainBusy
	}
	defer unlock()
	// The ceiling is per repository, multiplied up so the whole pass fits; a
	// single ceiling would cut a two-repository domain off half way through
	// the second one with a timeout naming neither. The multiplication is
	// bounded by the number of repositories the domain has, which is the
	// number of rows an operator created.
	ctx, cancel := context.WithTimeout(ctx, time.Duration(len(repos))*15*time.Minute)
	defer cancel()
	// Publish a "maintenance" progress pair (begin and terminal, indeterminate,
	// since restic check streams no percentage) and record a "verify" run, so
	// a manual or scheduled verify shows up on the dashboard activity log and
	// run history instead of running invisibly.
	vkey := "verify:" + domain
	_, startedAt := s.progBegin(ctx, vkey, "maintenance")
	defer func() { s.progEnd(vkey, "maintenance", err == nil, startedAt) }()
	runID, rErr := s.store.StartRun(domainRunTargetID(domain), "verify")
	if rErr != nil {
		log.Printf("api: verify %s: could not start run record (continuing): %v", domain, rErr) //nolint:gosec // G706: domain is a fixed literal
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
			log.Printf("api: verify %s: could not finish run record: %v", domain, fErr) //nolint:gosec // G706: domain is a fixed literal
		}
	}()

	// Clear a genuinely stale orphan before `restic check` takes its lock:
	// unlockStale runs plain `restic unlock`, which removes only locks restic
	// itself deems stale (a dead PID on this host, or any lock past restic's
	// ~30-minute age threshold). A live or concurrent lock is never
	// force-removed: the domain lock is held for the whole verify, so no other
	// BombVault op can collide, and `restic check` passes --retry-lock to wait
	// out a transient cross-process lock. A known, bounded gap: an orphan from
	// a prior container incarnation carries that container's random hostname,
	// so restic can't PID-probe it and calls it stale only at ~30 minutes old;
	// until then check fails "already locked" (it heals itself, or a manual
	// Unlock clears it). A stable container hostname would close this.
	//
	// Each repository in turn, under the one domain lock. The first failure is
	// the answer: a domain whose data is spread over a domain repository and
	// named ones is only verified when every one of them is. The mode is built
	// per repository, because a named repository carries its own credentials,
	// storage class and bandwidth caps.
	for _, r := range repos {
		rMode := s.repoModeFor(settings, domain, source, r.Loc)
		s.unlockStale(ctx, r.Loc, rMode)
		if err = s.engine.Check(ctx, r.Loc, rMode); err != nil {
			err = fmt.Errorf("verifying %s: %w", s.refName(r), err)
			return err
		}
	}
	// Everything that was reachable verified clean. That is not a success
	// while part of the domain was never opened, so the run records what was
	// missed instead of a green tick over the remainder.
	err = skippedError("this verify", skipped)
	return err
}

// drillSubsetPct clamps the configured drill subset percentage into restic's
// valid 1..100 range, defaulting an unset/zero value to 5.
func drillSubsetPct(pct int) int {
	if pct <= 0 {
		return 5
	}
	if pct > 100 {
		return 100
	}
	return pct
}

// RunRestoreDrill runs a restore-verification drill of the requested kind and
// records the result so the UI can show a "last verified restorable" badge.
// kind "subset" (or "") is the classic in-place integrity check; kind "dr" is a
// real off-site sandbox restore (see runDRDrill). domain is
// {containers,vms,flash,config,files}; source is {local,offsite} (ignored for
// kind "dr", which is always off-site).
// wait selects the lock discipline for the underlying drill: a scheduled
// drill passes wait=true so it blocks for the per-domain lock and always
// records a result (a nightly backup or replication firing at the same
// time must not make it vanish and leave the dashboard at "never"); a
// manual drill passes wait=false for immediate errDomainBusy feedback and
// records nothing (#30).
func (s *Service) RunRestoreDrill(ctx context.Context, domain, source, kind string, wait bool) (store.RestoreDrill, error) {
	switch kind {
	case "", "subset":
		return s.runSubsetDrill(ctx, domain, source, wait)
	case "dr":
		return s.runDRDrill(ctx, domain, source, wait)
	default:
		return store.RestoreDrill{}, fmt.Errorf("unknown drill kind %q", kind)
	}
}

// runSubsetDrill proves a domain's backup is restorable by running `restic
// check --read-data-subset` (it reads back and re-verifies a random subset
// of the real pack data, not just metadata, with no scratch disk needed)
// and records the result. domain is {containers,vms,flash,config,files};
// source is {local,offsite}.
//
// It takes the per-domain busy guard like Prune and Unlock: if a backup is
// running it returns errDomainBusy and records nothing.
//
// A repository that was never created returns a clear "no backups to
// verify" error and records nothing: there is nothing to be red about. A
// repository that was there and is gone records a failed row, or the
// restorability badge would keep last week's green tick over a share that
// stopped mounting.
//
// Passing and failing drills are both recorded. The notification is
// confined to the scheduled pass (wait), at all four sites: a manual press
// puts the answer on the screen of whoever pressed it, and
// notifyDrillFailure has no throttle, so repeated presses while somebody
// diagnoses an unmounted share would send a mail each time.
func (s *Service) runSubsetDrill(ctx context.Context, domain, source string, wait bool) (drill store.RestoreDrill, err error) {
	switch domain {
	case "containers", "vms", "flash", "config", "files":
	default:
		return store.RestoreDrill{}, fmt.Errorf("unknown domain %q", domain)
	}
	switch {
	case source == "local", isOffsiteSource(source):
	default:
		return store.RestoreDrill{}, fmt.Errorf("unknown source %q", source)
	}

	// Every repository this domain's items write to (#204). A drill reads real
	// pack data back; reading it out of the domain repository while an item's
	// data sits in a named one would verify the wrong bytes and hand back a
	// green badge for it.
	settings, repos, skipped, err := s.domainReposForOp(domain, source)
	if err != nil {
		return store.RestoreDrill{}, err
	}
	repos, missing, err := s.reposThatExist(repos, "no backups to verify yet")
	if err != nil {
		// The all-missing case gets the same treatment as the read-nothing case
		// below, for the same reason: returning bare records no row, and the
		// restorability badge keeps last week's green tick while the share it
		// verified stopped mounting.
		//
		// "Never created yet" is not that case: reposThatExist reports it with an
		// empty skip list, and a domain that has never been backed up has nothing
		// to be red about.
		if len(missing) > 0 {
			rec := store.RestoreDrill{
				Domain: domain, Source: source, Kind: "subset",
				At: time.Now().Unix(), OK: false, Detail: truncateRunErr(err),
			}
			if aErr := s.store.AddRestoreDrill(rec); aErr != nil {
				log.Printf("api: drill: record unreachable-domain result for %q: %v", domain, aErr) //nolint:gosec // G706: domain is %q-quoted
			}
			s.recordDomainRun(domain, "drill", false, rec.Detail)
			// Notify only on the scheduled pass, like every other notification in
			// this function (see the doc comment).
			if wait {
				s.notifyDrillFailure(ctx, domain, source, rec.Detail)
			}
			return rec, err
		}
		return store.RestoreDrill{}, err
	}
	skipped = append(skipped, missing...)

	// Serialise with backups (and other maintenance) so a drill never reads a
	// repo a backup is writing. A scheduled drill (wait) blocks for the domain
	// so it always records a result even when a nightly backup or replication
	// fires at the same time; a manual drill fails fast with immediate busy
	// feedback without recording (#30).
	var unlock func()
	if wait {
		// Bounded wait: poll the lock up to drillLockWait, then log and skip, so a
		// wedged lock-holder can't block a scheduled drill forever or pile up a
		// goroutine each night.
		u, ok := s.waitLockDomainFor(domain, "verify")
		if !ok {
			log.Printf("api: drill: %q busy longer than %v, skipping this scheduled run", domain, drillLockWait) //nolint:gosec // G706: domain is %q-quoted and validated to a fixed allow-list above
			// Record the skip as a dated failed row, so the dashboard shows why the
			// check did not run rather than freezing the previous red with no reason
			// (#30).
			skip := store.RestoreDrill{
				Domain: domain,
				Source: source,
				Kind:   "subset",
				At:     time.Now().Unix(),
				OK:     false,
				Detail: "skipped: repository busy longer than " + drillLockWait.String() + " (a backup or off-site copy held it)",
			}
			if aErr := s.store.AddRestoreDrill(skip); aErr != nil {
				log.Printf("api: drill: record busy-skip for %q: %v", domain, aErr) //nolint:gosec // G706: domain is %q-quoted and validated above
			}
			s.recordDomainRun(domain, "drill", false, skip.Detail)
			s.notifyDrillFailure(ctx, domain, source, skip.Detail)
			return skip, errDomainBusy
		}
		unlock = u
	} else {
		u, ok := s.tryLockDomainFor(domain, "verify")
		if !ok {
			return store.RestoreDrill{}, errDomainBusy
		}
		unlock = u
	}
	defer unlock()

	// Publish a live "maintenance" progress pair keyed "drill:<domain>" (like
	// prune: and verify:) so a running restore-verification drill shows on the
	// dashboard activity log while it reads back pack data, not only after it
	// finished (#109). The terminal event is deferred so an error or panic can
	// never leave a stuck live line.
	dkey := "drill:" + domain
	_, startedAt := s.progBegin(ctx, dkey, "maintenance")
	defer func() { s.progEnd(dkey, "maintenance", err == nil, startedAt) }()

	// Reading back a subset of real pack data can be slow on a large repo, so
	// the whole pass over the domain's repositories is bounded.
	//
	// ctx is rebound rather than given a second name, as in CheckDomain,
	// UnlockDomain and pruneDomain, so the snapshot listing and the stale-lock
	// clear inside the loop are under the ceiling too. A scheduled drill
	// enters on context.Background() holding the domain lock, so without the
	// ceiling one unreachable remote repository could park the listing forever
	// and leave every other operation on the domain answering "busy".
	//
	// outer is kept because the failure notification must not travel on the
	// drill's own deadline: the failure a drill most needs to announce is the
	// one where it ran out of time.
	outer := ctx
	ctx, cancel := context.WithTimeout(ctx, time.Duration(len(repos))*2*time.Hour)
	defer cancel()
	pct := drillSubsetPct(settings.DrillsSubsetPct)
	var checkErr error
	drilled := 0
	for _, r := range repos {
		rMode := s.repoModeFor(settings, domain, source, r.Loc)
		// An initialised but empty repo (no snapshots) has nothing to verify.
		// Treat it like a missing repo: skipped here, and a clear error below when
		// none of them had anything, with no misleading failure recorded either
		// way.
		snaps, sErr := s.listSnapshots(ctx, r.Loc, rMode)
		if sErr != nil {
			// One unreadable repository is a skip, not the end of the pass: returning
			// would record nothing and leave the badge frozen on its previous verdict
			// for every other repository of the domain too. restic's own sentence goes
			// into the reason, since "could not be listed" alone does not say whether
			// the share is gone or the password wrong.
			skipped = append(skipped, repoSkip{Name: s.refName(r), Reason: scrubError(sErr), Unreachable: true})
			continue
		}
		if len(snaps) == 0 {
			continue
		}
		drilled++
		// Clear any stale lock a previously interrupted off-site op (replication
		// copy or integrity check) left behind before `restic check
		// --read-data-subset` takes its lock, so a drill can't fail "repository is
		// already locked"; BombVault is the sole writer, so an existing lock is
		// always stale (as in CheckDomain, #29).
		s.unlockStale(ctx, r.Loc, rMode)
		if checkErr = s.engine.CheckData(ctx, r.Loc, pct, rMode); checkErr != nil {
			checkErr = fmt.Errorf("reading back %s: %w", s.refName(r), checkErr)
			break
		}
	}
	if drilled == 0 {
		// Nothing could be read back. If that is because something was
		// unreachable rather than empty, it is a failed drill and has to be
		// recorded as one: returning without a row would leave the restorability
		// badge frozen on its previous verdict, and a domain nobody can read any
		// more would keep last week's green tick. The busy-skip branch above
		// records its reason for the same purpose.
		if sErr := skippedError("this restorability check", skipped); sErr != nil {
			rec := store.RestoreDrill{
				Domain: domain, Source: source, Kind: "subset",
				At: time.Now().Unix(), OK: false, Detail: truncateRunErr(sErr),
			}
			if aErr := s.store.AddRestoreDrill(rec); aErr != nil {
				log.Printf("api: drill: record unreadable-domain result for %q: %v", domain, aErr) //nolint:gosec // G706: domain is %q-quoted
			}
			s.recordDomainRun(domain, "drill", false, rec.Detail)
			// Scheduled pass only, like the other notifications in this function. It
			// matters most here: the unreadable-share branch is exactly the state
			// somebody presses the button at repeatedly while diagnosing it.
			if wait {
				s.notifyDrillFailure(outer, domain, source, rec.Detail)
			}
			return rec, sErr
		}
		return store.RestoreDrill{}, errors.New("no backups to verify yet")
	}
	// A pass that read back everything it could reach, but could not reach all of
	// the domain, is not a passing drill: the badge would be green about data
	// nobody opened.
	if checkErr == nil {
		checkErr = skippedError("this restorability check", skipped)
	}

	drill = store.RestoreDrill{
		Domain: domain,
		Source: source,
		At:     time.Now().Unix(),
		OK:     checkErr == nil,
		Kind:   "subset",
	}
	if checkErr != nil {
		drill.Detail = scrubError(checkErr)
		if len(drill.Detail) > 200 {
			drill.Detail = drill.Detail[:200]
		}
	}
	if recErr := s.store.AddRestoreDrill(drill); recErr != nil {
		// Recording is what a drill is for; surface a record failure.
		return store.RestoreDrill{}, fmt.Errorf("record drill: %w", recErr)
	}
	// Mirror the drill outcome into the shared runs table so it shows in the
	// dashboard Activity Log/Run History (the restore_drills row above stays the
	// badge/scorecard source of truth).
	s.recordDomainRun(domain, "drill", drill.OK, drill.Detail)
	// A failed restorability check is important, so notify on failure
	// (best-effort), on the scheduled pass only like the other three in this
	// function. A manual press puts the same answer on the screen of whoever
	// pressed it.
	if checkErr != nil && wait {
		s.notifyDrillFailure(outer, domain, source, drill.Detail)
	}
	return drill, checkErr
}

// drillMarkerName is the sentinel file written into a DR-drill sandbox at
// creation. Cleanup deletes a sandbox only when this marker is present in
// that exact directory, a safety interlock so a drill can never
// os.RemoveAll a path that is not a marked drill sandbox of its own.
const drillMarkerName = ".bombvault-drill"

// drillByteToleranceFloor is the only slack allowed between restic's
// reported restore-size and the on-disk restore of a DR drill: a tight
// few-KB absolute floor for filesystem metadata rounding, not a percentage
// of the total. `restic restore` is content-addressed, so a completed
// restore reproduces the exact logical bytes; a percentage band (e.g. 5%
// of the whole snapshot) would wave through a large file restored
// truncated by less than that fraction, with the file count unchanged. The
// count must match exactly and the bytes to within this floor.
const drillByteToleranceFloor = 4096

// drillSnapshotTimeout bounds the DR-drill's off-site snapshot listing so a
// black-holed off-site (a `restic snapshots` that hangs on a dead network) can't
// hold the domain lock indefinitely and starve a concurrent scheduled backup. The
// restore itself is bounded separately (restoreTimeout), matching a real restore.
const drillSnapshotTimeout = 15 * time.Minute

// errNothingToDrill signals that the newest off-site snapshot has no
// restorable file data (0 files, 0 bytes, e.g. a definition-only or
// stateless container). A DR drill then records nothing: a green would be
// a false "verified restorable" and a red a false failure.
var errNothingToDrill = errors.New("no restorable file data in the newest off-site snapshot: nothing to drill")

// runDRDrill performs a real off-site disaster-recovery drill for a domain: it
// restores the newest off-site snapshot of the drill target into a marker-guarded
// sandbox under the restore folder, verifies the restored file count + bytes
// against restic's own accounting, deletes the sandbox (marker-guarded), and
// records a restore_drills(kind='dr', source='offsite') row. It takes the domain
// repo lock exactly like a real restore, so a scheduled backup can never fire
// mid-drill and vice-versa; busy → errDomainBusy, recording nothing. A failure
// records kind='dr' ok=false and fires the drill-failure notification.
func (s *Service) runDRDrill(ctx context.Context, domain, source string, wait bool) (drill store.RestoreDrill, err error) {
	switch domain {
	case "containers", "vms", "flash", "files":
	default:
		return store.RestoreDrill{}, fmt.Errorf("unknown domain %q", domain)
	}
	// A DR drill only ever restores from an off-site repo. A non-offsite
	// source (the scheduler's legacy call, or "local") normalises to the bare
	// "offsite" primary; an "offsite:<id>" source drills, and records under,
	// that specific destination.
	if !isOffsiteSource(source) {
		source = "offsite"
	}

	settings, err := s.store.GetSettings()
	if err != nil {
		return store.RestoreDrill{}, fmt.Errorf("read settings: %w", err)
	}
	target, err := s.offsiteTargetForSource(settings, domain, source)
	if err != nil {
		return store.RestoreDrill{}, err
	}
	repo, err := s.resolveRepo(target.Repo)
	if err != nil {
		return store.RestoreDrill{}, err
	}

	// Serialise with backups and restores on the domain repo: a scheduled
	// backup must never fire mid-drill and vice versa. A scheduled drill
	// (wait) blocks for the domain so it always records a result even when a
	// nightly op fires at the same time; a manual drill fails fast with
	// immediate busy feedback without recording (#30).
	var unlock func()
	if wait {
		// Bounded wait: poll the lock up to drillLockWait, then log and skip, so a
		// wedged lock-holder can't block a scheduled drill forever or pile up a
		// goroutine each night.
		u, ok := s.waitLockDomainFor(domain, "verify")
		if !ok {
			log.Printf("api: drill: %q busy longer than %v, skipping this scheduled run", domain, drillLockWait) //nolint:gosec // G706: domain is %q-quoted and validated to a fixed allow-list above
			// Record the skip as a dated failed row, so the dashboard shows why the
			// off-site DR check did not run rather than freezing the red with no
			// reason (#30).
			skip := store.RestoreDrill{
				Domain: domain,
				Source: source,
				Kind:   "dr",
				At:     time.Now().Unix(),
				OK:     false,
				Detail: "skipped: repository busy longer than " + drillLockWait.String() + " (a backup or off-site copy held it)",
			}
			if aErr := s.store.AddRestoreDrill(skip); aErr != nil {
				log.Printf("api: drill: record busy-skip for %q: %v", domain, aErr) //nolint:gosec // G706: domain is %q-quoted and validated above
			}
			s.recordDomainRun(domain, "drdrill", false, skip.Detail)
			s.notifyDrillFailure(ctx, domain, source, skip.Detail)
			return skip, errDomainBusy
		}
		unlock = u
	} else {
		u, ok := s.tryLockDomainFor(domain, "verify")
		if !ok {
			return store.RestoreDrill{}, errDomainBusy
		}
		unlock = u
	}
	defer unlock()

	// Publish a live "maintenance" progress pair keyed "drdrill:<domain>"
	// (like prune: and verify:) so a running DR drill shows on the dashboard
	// activity log while it sandbox-restores, not only after it finished
	// (#109). The "drdrill" key and kind are distinct from the local subset
	// drill's "drill", so the log tells the off-site DR restore check apart
	// from the local read-back check. The terminal event is deferred so an
	// error or panic can never leave a stuck live line.
	dkey := "drdrill:" + domain
	_, startedAt := s.progBegin(ctx, dkey, "maintenance")
	defer func() { s.progEnd(dkey, "maintenance", err == nil, startedAt) }()

	// Detach from the request/scheduler ctx for the whole drill: a real DR restore
	// can take hours over a slow off-site link, and a browser tab close (request
	// ctx) or a context.Background scheduler parent must not abort it. The snapshot
	// listing is then bounded (drillSnapshotTimeout) so a black-holed off-site can't
	// hold the domain lock forever; the restore is bounded by restoreTimeout inside
	// sandboxRestoreVerify.
	drillCtx := context.WithoutCancel(ctx)
	// The destination is described by the target row already in hand above.
	// offsiteModeForTarget is the builder copyToOffsiteTarget uses to write
	// this same repository; opened with the shared mode, a destination with
	// its own credentials could be written and never read back, and the drill
	// could not pass.
	mode := s.offsiteModeForTarget(settings, target)
	listCtx, listCancel := context.WithTimeout(drillCtx, drillSnapshotTimeout)
	snapID, err := s.pickDRSnapshot(listCtx, domain, settings, repo, mode)
	listCancel()
	if err != nil {
		return store.RestoreDrill{}, err
	}

	// Clear any stale lock a previously interrupted off-site op left behind
	// before the sandbox restore takes its lock, so a drill can't fail
	// "repository is already locked"; BombVault is the sole writer, so an
	// existing lock is always stale (as in CheckDomain, #29). drillCtx
	// (detached) matches the restore below.
	s.unlockStale(drillCtx, repo, mode)
	// Restore into the sandbox, verify, and clean up (marker-guarded). The
	// outcome is recorded either way; a failure also notifies. An empty
	// (0-file, 0-byte) snapshot records nothing, neither a false green nor a
	// false red.
	drillErr := s.sandboxRestoreVerify(drillCtx, domain, settings, repo, snapID, mode)
	if errors.Is(drillErr, errNothingToDrill) {
		return store.RestoreDrill{}, drillErr
	}
	drill = store.RestoreDrill{
		Domain: domain,
		Source: source,
		At:     time.Now().Unix(),
		OK:     drillErr == nil,
		Kind:   "dr",
	}
	if drillErr != nil {
		drill.Detail = scrubError(drillErr)
		if len(drill.Detail) > 200 {
			drill.Detail = drill.Detail[:200]
		}
	}
	if recErr := s.store.AddRestoreDrill(drill); recErr != nil {
		return store.RestoreDrill{}, fmt.Errorf("record drill: %w", recErr)
	}
	// Mirror the drill outcome into the shared runs table so it shows in the
	// dashboard Activity Log and Run History (the restore_drills row above
	// stays the badge and scorecard source of truth). Kind "drdrill", distinct
	// from the local subset drill's "drill", so the log names the off-site DR
	// restore check.
	s.recordDomainRun(domain, "drdrill", drill.OK, drill.Detail)
	if drillErr != nil {
		s.notifyDrillFailure(ctx, domain, source, drill.Detail)
	}
	return drill, drillErr
}

// pickDRSnapshot resolves the newest off-site snapshot to drill for a
// domain. containers: the DRDrillTarget container (or, when unset, the
// most recently backed-up container), scoped to its container:<name> tag.
// vms: the DRDrillTargetVM VM (or, when unset, the most recently
// backed-up VM), scoped to its vm:<name> tag, the same pattern. flash: the
// newest snapshot outright (flash is a single whole-USB image, no per-item
// scoping). files follows flash, the newest snapshot in the files repo
// outright, since a file-set restore is sandbox-cheap and any set proves
// the repo restorable. An empty repo or a target with no off-site snapshot
// yields a clear error.
func (s *Service) pickDRSnapshot(ctx context.Context, domain string, settings store.Settings, repo string, mode restic.Mode) (string, error) {
	all, err := s.listSnapshots(ctx, repo, mode)
	if err != nil {
		return "", err
	}
	if len(all) == 0 {
		return "", errors.New("no off-site backups to drill yet")
	}
	switch domain {
	case "flash", "files":
		return newestSnapshot(all).ID, nil
	case "containers", "vms":
		var (
			target string
			tagPfx string
		)
		if domain == "containers" {
			target, tagPfx = settings.DRDrillTarget, "container:"
			if target == "" {
				target, err = s.newestBackedUpContainer()
			}
		} else {
			target, tagPfx = settings.DRDrillTargetVM, "vm:"
			if target == "" {
				target, err = s.newestBackedUpVM()
			}
		}
		if err != nil {
			return "", err
		}
		tag := tagPfx + target
		var scoped []restic.Snapshot
		for _, snap := range all {
			for _, t := range snap.Tags {
				if t == tag {
					scoped = append(scoped, snap)
					break
				}
			}
		}
		if len(scoped) == 0 {
			return "", fmt.Errorf("no off-site snapshot for drill target %q", target)
		}
		return newestSnapshot(scoped).ID, nil
	default:
		return "", fmt.Errorf("unknown domain %q", domain)
	}
}

// newestSnapshot returns the snapshot with the latest Time (RFC3339 sorts
// chronologically as a string). snaps must be non-empty.
func newestSnapshot(snaps []restic.Snapshot) restic.Snapshot {
	best := snaps[0]
	for _, sn := range snaps[1:] {
		if sn.Time > best.Time {
			best = sn
		}
	}
	return best
}

// newestBackedUpContainer returns the container name with the most recent
// successful backup run, the default DR-drill target when none is pinned.
func (s *Service) newestBackedUpContainer() (string, error) {
	targets, err := s.store.ListTargets()
	if err != nil {
		return "", fmt.Errorf("list targets: %w", err)
	}
	best := ""
	var bestAt int64
	for _, t := range targets {
		run, rErr := s.store.LastSuccessfulBackup(t.ID)
		if rErr != nil {
			return "", rErr
		}
		if run == nil || run.FinishedAt == nil {
			continue
		}
		if *run.FinishedAt >= bestAt {
			bestAt = *run.FinishedAt
			best = t.ContainerName
		}
	}
	if best == "" {
		return "", errors.New("no backed-up container to drill")
	}
	return best, nil
}

// newestBackedUpVM returns the VM name with the most recent successful
// backup run, the default DR-drill target when none is pinned. It mirrors
// newestBackedUpContainer (VM runs share the same runs table and column,
// see migration 5 runs_relax_fk).
func (s *Service) newestBackedUpVM() (string, error) {
	targets, err := s.store.ListVMTargets()
	if err != nil {
		return "", fmt.Errorf("list VM targets: %w", err)
	}
	best := ""
	var bestAt int64
	for _, t := range targets {
		run, rErr := s.store.LastSuccessfulBackup(t.ID)
		if rErr != nil {
			return "", rErr
		}
		if run == nil || run.FinishedAt == nil {
			continue
		}
		if *run.FinishedAt >= bestAt {
			bestAt = *run.FinishedAt
			best = t.Name
		}
	}
	if best == "" {
		return "", errors.New("no backed-up VM to drill")
	}
	return best, nil
}

// sandboxRestoreVerify restores the whole snapshot tree into a fresh
// marker-guarded sandbox under the restore folder, verifies the restored
// files and bytes against restic's own accounting, and always attempts
// marker-guarded cleanup. A mismatch or restore/verify error is returned;
// the sandbox is still removed. It reuses the RestoreInclude machinery and
// paths.Resolve containment of a real restore to a folder.
func (s *Service) sandboxRestoreVerify(ctx context.Context, domain string, settings store.Settings, repo, snapID string, mode restic.Mode) error {
	sub := path.Join(settings.RestoreFolder, fmt.Sprintf("bombvault-drill-%s-%d", domain, time.Now().UnixNano()))
	sandbox, err := paths.Resolve(s.cfg.HostMountRoot, sub)
	if err != nil {
		return errors.New("invalid restore folder: must be a relative subpath under the host mount")
	}
	// Create the parent (restore folder), then the sandbox leaf with os.Mkdir,
	// which fails if it already exists: a positive assertion that this is a
	// fresh directory before it becomes a marker-guarded RemoveAll target
	// (MkdirAll would silently adopt a pre-existing directory).
	if err := paths.EnsureDir(filepath.Dir(sandbox)); err != nil {
		return fmt.Errorf("create drill sandbox parent: %w", err)
	}
	if err := os.Mkdir(sandbox, 0o700); err != nil { //nolint:gosec // G703: sandbox is resolved strictly under the host mount root by paths.Resolve
		return fmt.Errorf("create drill sandbox: %w", err)
	}
	// Marker first, before any restore, so the cleanup interlock can always
	// confirm this is a sandbox of its own, even if the restore fails midway.
	// If the marker write itself fails the (still empty) dir would leak, so it
	// is removed explicitly on that path before the cleanup defer is
	// registered.
	markerPath := filepath.Join(sandbox, drillMarkerName)
	if err := os.WriteFile(markerPath, []byte("bombvault dr drill\n"), 0o600); err != nil { //nolint:gosec // G306: marker is a non-secret sentinel; 0600 is already restrictive
		if rmErr := os.Remove(sandbox); rmErr != nil { //nolint:gosec // G703: sandbox is resolved strictly under the host mount root by paths.Resolve (rejects absolute/traversal); it was just created empty by os.Mkdir above
			log.Printf("api: dr-drill: could not remove sandbox after marker-write failure: %v", rmErr)
		}
		return fmt.Errorf("write drill marker: %w", err)
	}
	defer func() {
		if cErr := cleanupDrillSandbox(sandbox); cErr != nil {
			log.Printf("api: dr-drill: cleanup: %v", cErr)
		}
	}()

	// Bound the restore at restoreTimeout, like a real restore: reading a
	// whole snapshot back over a slow off-site link can take many hours, far
	// more than a short drill-only deadline. ctx is already detached from the
	// request by the caller (runDRDrill), so a browser tab close can't abort a
	// legitimate drill.
	dctx, cancel := context.WithTimeout(ctx, restoreTimeout)
	defer cancel()

	// A VM disk image (or any large snapshot) can be hundreds of GB; fail fast
	// with a clear error if the sandbox filesystem doesn't have room, rather
	// than letting the restore run the host out of disk midway. The same
	// preflight idiom as guardVMRestoreDestination and
	// guardContainerRestoreDestination: a stats or probe error (e.g. the
	// unsupported-platform stub in diskfree_other.go) is "cannot prove
	// insufficient" and never blocks the drill; only a proven shortfall
	// aborts.
	if _, wantBytes, statErr := s.engine.StatsRestoreSize(dctx, repo, snapID, mode); statErr == nil && wantBytes > 0 {
		if free, fErr := s.diskFreeFn()(sandbox); fErr == nil && free < uint64(wantBytes) {
			return fmt.Errorf("not enough free space to sandbox-restore: it needs %d bytes but %q has only %d free. Free up space and retry", wantBytes, settings.RestoreFolder, free)
		}
	}

	if err := s.engine.RestoreInclude(dctx, repo, snapID, "/", sandbox, mode); err != nil {
		return fmt.Errorf("restore into sandbox: %w", err)
	}

	// Verify: restic's own file count (ls) + restore-size bytes+files (stats) vs an
	// on-disk walk of the sandbox (the marker is excluded from the walk).
	lsEntries, err := s.engine.Ls(dctx, repo, snapID, mode)
	if err != nil {
		return fmt.Errorf("list snapshot: %w", err)
	}
	lsFiles := 0
	for _, e := range lsEntries {
		if e.Type == "file" {
			lsFiles++
		}
	}
	statsFiles, wantBytes, err := s.engine.StatsRestoreSize(dctx, repo, snapID, mode)
	if err != nil {
		return fmt.Errorf("snapshot stats: %w", err)
	}
	// A snapshot with no restorable file data exercises no real restore, and
	// recording it green would be a false "verified restorable". Signal
	// "nothing to drill" so the caller records neither green nor red.
	if lsFiles == 0 && wantBytes == 0 {
		return errNothingToDrill
	}
	gotFiles, gotBytes, err := walkDrillSandbox(sandbox)
	if err != nil {
		return fmt.Errorf("walk sandbox: %w", err)
	}
	if !drillVerifyOK(lsFiles, gotFiles, wantBytes, gotBytes) {
		return fmt.Errorf("verification mismatch: restic ls %d files, stats %d files / %d bytes; restored sandbox %d files / %d bytes", lsFiles, statsFiles, wantBytes, gotFiles, gotBytes)
	}
	return nil
}

// walkDrillSandbox counts the regular files and their total bytes under a drill
// sandbox, excluding the .bombvault-drill marker at its root. Only regular files
// count toward restore-size (dirs/symlinks/devices are ignored, matching restic).
func walkDrillSandbox(sandbox string) (files int, bytes int64, err error) {
	marker := filepath.Join(sandbox, drillMarkerName)
	err = filepath.WalkDir(sandbox, func(p string, d fs.DirEntry, walkErr error) error { //nolint:gosec // G703: sandbox is resolved strictly under the host mount root by paths.Resolve (rejects absolute/traversal); read-only walk
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() || p == marker {
			return nil
		}
		info, iErr := d.Info()
		if iErr != nil {
			return iErr
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		files++
		bytes += info.Size()
		return nil
	})
	return files, bytes, err
}

// drillVerifyOK reports whether a restored sandbox matches restic's own
// accounting. The completeness proof is the on-disk restore versus restic:
//
//   - the on-disk file count == restic ls file count (a completed restore
//     materialises every file node restic recorded);
//   - the on-disk bytes == restic's restore-size bytes to within
//     drillByteToleranceFloor (a few KB for fs metadata), not a
//     percentage, since a content-addressed restore reproduces the exact
//     logical bytes.
//
// restic's `stats --mode restore-size` file count is not compared against
// `ls`: the two counters legitimately differ on real snapshots (hardlinks,
// and how restore-size tallies files versus how ls enumerates nodes), so
// requiring statsFiles == lsFiles would flag perfectly restorable backups,
// such as an Unraid flash restoring the exact file count and bytes (#30).
// A truncated restore is already caught by gotFiles != lsFiles and the
// byte check, which both measure the restored sandbox; statsFiles measures
// neither and could only add false negatives.
func drillVerifyOK(lsFiles, gotFiles int, statsBytes, gotBytes int64) bool {
	if gotFiles != lsFiles {
		return false
	}
	diff := statsBytes - gotBytes
	if diff < 0 {
		diff = -diff
	}
	return diff <= drillByteToleranceFloor
}

// cleanupDrillSandbox removes a DR-drill sandbox, but only after
// confirming the .bombvault-drill marker written at creation is present in
// that exact directory. This is a safety-critical interlock: os.RemoveAll
// is destructive, so a drill must never delete a path that is not a marked
// sandbox (e.g. a mis-resolved or operator-configured folder). A missing
// marker removes nothing and returns an error.
func cleanupDrillSandbox(sandbox string) error {
	if _, err := os.Stat(filepath.Join(sandbox, drillMarkerName)); err != nil { //nolint:gosec // G703: sandbox is resolved strictly under the host mount root by paths.Resolve; this stat is the marker interlock itself
		return fmt.Errorf("drill sandbox %q lacks the %s marker; refusing to delete", filepath.Base(sandbox), drillMarkerName)
	}
	return os.RemoveAll(sandbox) //nolint:gosec // G703: sandbox is under the host mount root (paths.Resolve) and guarded above by the .bombvault-drill marker, so it never removes a non-drill path
}

// LatestDrill returns the most recent restore-verification drill for a domain +
// source (a thin passthrough to the store). found is false when none ran yet.
func (s *Service) LatestDrill(domain, source string) (store.RestoreDrill, bool, error) {
	return s.store.LatestRestoreDrill(domain, source)
}

// Drills returns the recorded restore-verification drills for a domain + source
// (newest first), a thin passthrough to the store.
func (s *Service) Drills(domain, source string, limit int) ([]store.RestoreDrill, error) {
	return s.store.ListRestoreDrills(domain, source, limit)
}

// notifyDrillFailure sends a best-effort notification when a restore-verification
// drill fails (the backup is not provably restorable). Mirrors notifyBackup's
// policy + Unraid fan-out; a no-op when notifications are off.
func (s *Service) notifyDrillFailure(ctx context.Context, domain, source, detail string) {
	c, err := s.NotifyConfig()
	if err != nil || c.On == "" || c.On == "never" {
		return
	}
	target := "Unraid flash"
	if domain != "flash" {
		target = domain
	}
	msg := fmt.Sprintf("Restore verification of %s (%s) FAILED, so the backup may not be restorable: %s", target, source, detail)
	notify.Send(ctx, c, domain, notify.Event{Title: "BombVault", Message: msg, OK: false})
	if s.unraidGate(c.Unraid) {
		if e := s.sendUnraidNotify(ctx, "BombVault: restore verification FAILED", msg, "warning"); e != nil {
			log.Printf("notify: unraid: %v", e)
		}
	}
}

// recordDomainRun persists an already completed domain-scoped check (restore
// drill / tamper test) as a runs row on the reserved domain target id, so it
// shows up in the dashboard Activity Log/Run History like prune/verify runs do.
// The operation has finished by the time this is called, so the row is opened
// and closed back-to-back (its own history table carries the full timing).
// detail is bounded to the same cap as truncateRunErr. Best-effort: a store
// error is logged and never fails the check that already ran.
func (s *Service) recordDomainRun(domain, kind string, ok bool, detail string) {
	runID, err := s.store.StartRun(domainRunTargetID(domain), kind)
	if err != nil {
		log.Printf("api: %s %s: could not start run record (continuing): %v", kind, domain, err) //nolint:gosec // G706: kind and domain are fixed literals
		return
	}
	status := "success"
	if !ok {
		status = "failed"
	}
	const maxDetail = 500 // mirror truncateRunErr's cap for the runs.error column
	if len(detail) > maxDetail {
		detail = detail[:maxDetail]
	}
	if err := s.store.FinishRun(runID, status, "", 0, detail); err != nil {
		log.Printf("api: %s %s: could not finish run record: %v", kind, domain, err) //nolint:gosec // G706: kind and domain are fixed literals
	}
}
