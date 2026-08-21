package api

// BackupEverything is the "Backup Everything" pass: a 6th, independent
// pseudo-domain that sequentially runs containers → vms → flash → files →
// config, group-stamping every child run it produces (WithRunGroup) under one
// parent run row (target_id = store.EverythingTargetID), and fires the
// operator-configured global pre/post hooks around the whole pass — the
// dead-man's-switch use case the feature exists for. See the design spec
// (docs/superpowers/specs/2026-08-20-backup-everything-design.md, decisions 3,
// 5 and 7) and the implementation plan's Task 4
// (docs/superpowers/plans/2026-08-20-backup-everything.md) for the full
// rationale — this file implements that plan.

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/junkerderprovinz/bombvault/internal/backup"
	"github.com/junkerderprovinz/bombvault/internal/notify"
	"github.com/junkerderprovinz/bombvault/internal/schedule"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// everythingBreakdownMaxLen bounds the structured per-domain breakdown string
// FinishRun stores as the parent run's error text, matching truncateRunErr's
// existing 500-char convention for run error messages elsewhere in this file.
const everythingBreakdownMaxLen = 500

// EverythingSummary is the outcome of one "Backup Everything" pass, returned
// by BackupEverything for callers that want more than the plain error
// StartBackupEverything's background goroutine settles for. RunID is the
// parent run's id (every child run this pass produced carries GroupID ==
// RunID); Status/Error mirror what was written to that parent run via
// FinishRun; Domains carries the per-domain detail the breakdown string was
// built from, for callers (tests, a future UI) that want structured data
// instead of re-parsing Error.
type EverythingSummary struct {
	RunID   string
	Status  string // "success" | "failed" — mirrors the parent run's own status
	Error   string // the structured breakdown; empty on a clean pass
	Domains []EverythingDomainResult
}

// EverythingDomainResult is one domain step's outcome within a "Backup
// Everything" pass. Attempted/Failed/Failures mirror
// schedule.RunContainersJob/RunVMsJob/RunFilesJob's own return shape for the
// three multi-item domains (containers/vms/files); flash/config (singletons)
// are represented the same way with Attempted always 1, so one formatter
// (formatEverythingDomain) covers every domain uniformly. Attempted == 0 with
// Failed == 0 means nothing was eligible this pass (e.g. no file sets
// defined) — a benign no-op, not a failure (design spec, decision 3).
// Attempted == 0 with Failed == 1 means the domain faulted before any item
// could even be attempted (e.g. ListTargetsScheduleOrder itself errored) —
// Failures[0] carries that one synthetic entry.
type EverythingDomainResult struct {
	Domain    string
	Attempted int
	Failed    int
	Failures  []schedule.ItemFailure
}

// BackupEverything runs one "Backup Everything" pass and returns once every
// domain step has been attempted. It essentially never returns a non-nil
// error once the parent run has started recording: every REAL failure (a
// domain erroring, an individual item failing) is captured as that domain's
// own EverythingDomainResult and folded into the parent run's structured
// breakdown, never propagated up — mirroring RunContainersJob/RunVMsJob/
// RunFilesJob's own contract of returning counts, not an error, for exactly
// this reason (a scheduled job logs per-item failures, it does not abort
// over them). A non-nil error here means the pass could not even be recorded
// as attempted (reading Settings or starting the parent run itself failed) —
// there is no partial outcome to report in that case.
func (s *Service) BackupEverything(ctx context.Context) (EverythingSummary, error) {
	settings, err := s.store.GetSettings()
	if err != nil {
		return EverythingSummary{}, fmt.Errorf("backup everything: read settings: %w", err)
	}

	// Best-effort, before anything else: a failing/hanging pre-hook must never
	// block the pass it is meant to gate (design spec, decision 6). HostShell.Run
	// already applies its own bounded timeout, so ctx is passed through as-is —
	// StartBackupEverything is the layer responsible for detaching it from
	// request cancellation (context.WithoutCancel), same as StartBackupAll.
	if settings.EverythingPreHook != "" {
		if err := s.hostShell.Run(ctx, settings.EverythingPreHook); err != nil {
			log.Printf("api: backup everything: pre-hook failed (best-effort, pass continues): %v", err)
		}
	}

	// The parent run row: every child run the five domain steps below produce
	// gets group_id = runID (via WithRunGroup + runsAdapter/startedRunsAdapter,
	// Task 3). If we can't even record that a pass started, no pass was really
	// attempted — return immediately, and deliberately WITHOUT firing the
	// post-hook (design spec, decision 7 / this task's own instructions): a
	// dead-man's-switch ping for a pass that never ran would be a false "done".
	runID, err := s.store.StartRun(store.EverythingTargetID, "backup")
	if err != nil {
		return EverythingSummary{}, fmt.Errorf("backup everything: start run: %w", err)
	}

	// Each domain step is independently wrapped: a step's own error (even one
	// that faults before any item is attempted, e.g. ListTargetsScheduleOrder
	// itself erroring) is captured into that step's own EverythingDomainResult
	// and never aborts the remaining steps (design spec, decision 5's explicit
	// "survive one domain failing" requirement).
	results := []EverythingDomainResult{
		s.everythingRunContainers(ctx, runID, settings),
		s.everythingRunVMs(ctx, runID, settings),
		s.everythingRunFlash(ctx, runID),
		s.everythingRunFiles(ctx, runID),
		s.everythingRunConfig(ctx, runID),
	}

	// Unconditional, exactly once, after every domain step has been attempted —
	// success or failure of any/all of them — the actual dead-man's-switch
	// requirement (design spec, decision 6). Best-effort: a failure here must
	// never change the pass's own recorded status.
	if settings.EverythingPostHook != "" {
		if err := s.hostShell.Run(ctx, settings.EverythingPostHook); err != nil {
			log.Printf("api: backup everything: post-hook failed (best-effort): %v", err)
		}
	}

	status := "success"
	for _, r := range results {
		if r.Failed > 0 {
			status = "failed"
			break
		}
	}
	var errMsg string
	if status == "failed" {
		errMsg = everythingBreakdown(results)
	}
	if err := s.store.FinishRun(runID, status, "", 0, errMsg); err != nil {
		// Best-effort like every other post-hoc run bookkeeping call in this
		// package: the pass itself already ran to completion, so a store error
		// recording its OWN outcome is only logged, never turned into a
		// returned error (there is nothing left for a caller to retry).
		log.Printf("api: backup everything: finish run %s: %v", runID, err)
	}

	return EverythingSummary{RunID: runID, Status: status, Error: errMsg, Domains: results}, nil
}

// StartBackupEverything launches a "Backup Everything" pass in a background
// goroutine and returns immediately, mirroring StartBackupAll's exact shape
// (design spec, decision 7): an atomic.Bool single-flight guard answers a
// concurrent second call with (false, nil) rather than overlapping, and
// context.WithoutCancel detaches the pass from the HTTP request that started
// it — each domain step already applies its own hold/hard-cap (backupHoldCtx
// inside s.Backup/s.BackupVM/etc.), so the pass itself needs no deadline of
// its own.
//
// Deliberately NO per-domain busy pre-flight check (unlike StartBackupAll's
// domainBusy("containers") check for its one domain): each domain step's own
// existing lock (s.lockDomain) already governs contention with any OTHER
// concurrent operation on that domain exactly as it does for every other
// caller today. A domain that is busy when its turn in the pass comes up
// simply surfaces as that domain's own failure in the pass's breakdown —
// consistent with "survive one domain failing" (design spec, decision 7).
func (s *Service) StartBackupEverything(ctx context.Context) (bool, error) {
	if !s.everythingActive.CompareAndSwap(false, true) {
		return false, nil
	}
	bctx := context.WithoutCancel(ctx)
	go func() {
		// Deferred FIRST (so it runs LAST), exactly like every other detached
		// backup/restore goroutine in this package — see recoverOperation's own
		// doc comment. A panic anywhere inside the pass would otherwise reach
		// the top of this goroutine unrecovered and take the whole process down,
		// including the four domain steps that had nothing to do with it.
		// failStuckRun closes out the PARENT run row (EverythingTargetID), which
		// BackupEverything opened with store.StartRun and would otherwise leave
		// "running" forever: the pass's own FinishRun is what the panic skipped.
		// Any CHILD domain run that was in flight is closed out by that domain's
		// own orchestrator path exactly as it is for any other caller.
		defer s.recoverOperation("backup everything: "+store.EverythingTargetID, nil, func(msg string) {
			s.failStuckRun(store.EverythingTargetID, msg)
		})
		defer s.everythingActive.Store(false)
		if _, err := s.BackupEverything(bctx); err != nil {
			log.Printf("api: backup everything: pass failed to start: %v", err)
		}
	}()
	return true, nil
}

// EverythingInProgress reports whether a "Backup Everything" pass is
// currently running, mirroring BackupInProgress's own test-support role for
// batchActive: tests poll this to wait out StartBackupEverything's detached
// background goroutine before asserting on state it touches.
func (s *Service) EverythingInProgress() bool { return s.everythingActive.Load() }

// everythingRunCtx builds the context the three MULTI-ITEM domain steps
// (containers/vms/files) hand to each ITEM's own backup call: group-stamped
// (WithRunGroup) so every child run traces back to runID, plus the exact same
// suppression stack main.go's scheduled multi-item closures apply to each
// item — inline off-site replication and per-item Healthchecks/message
// notifications are suppressed in favour of the ONE aggregate ping/batched
// replication this domain step performs itself after its loop (see
// ScheduledHealthchecksStart/Result, ScheduledNotifyResult below). The
// PruneAfterBulk/ReplicateOffsiteAfterBulk calls below deliberately do NOT
// use this suppressed ctx — mirroring main.go's own pruneAfterBulkFn/
// replicateAfterBulkFn closures, which take no per-item ctx at all and
// construct a fresh, unsuppressed context.Background() — so a failure
// surfaced by the batched off-site replication itself still pings
// Healthchecks/sends its failure notification exactly as a real scheduled
// domain run's batched replication does; using the suppressed ctx here would
// silently swallow that notification instead. The two SINGLETON domains
// (flash/config) do not use everythingRunCtx at all — they call
// WithRunGroup(ctx, runID) directly, mirroring SetFlashJob/SetConfigJob's
// closures, which apply none of this suppression either (design spec,
// decision 5).
func everythingRunCtx(ctx context.Context, runID string) context.Context {
	return WithRunGroup(
		WithBulkReplicateSuppressed(notify.WithMessagesSuppressed(notify.WithHealthchecksSuppressed(ctx))),
		runID,
	)
}

// everythingRunContainers backs up every IncludeInSchedule=true container,
// reproducing schedule.RunContainersJob's exact skip/continue-on-error
// semantics inline (targets not on the domain schedule this run — dropped by
// DomainRunTargets — are skipped; ErrContainerNotInstalled is a skip, not a
// failure, matching main.go's own scheduled containers closure; any other
// per-item error is logged and the loop continues) so this step's per-item
// Attempted/Failed/Failures can feed both the Healthchecks/notify aggregation
// below and this pass's own structured breakdown.
func (s *Service) everythingRunContainers(ctx context.Context, runID string, settings store.Settings) EverythingDomainResult {
	const domain = "containers"
	targets, err := s.store.ListTargetsScheduleOrder()
	if err != nil {
		log.Printf("api: backup everything: containers: list targets: %v", err)
		return everythingDomainFault(domain, err)
	}
	targets = schedule.DomainRunTargets(targets, settings.PerItemSchedules) // drop items on their own per-item cadence (#121)

	runCtx := everythingRunCtx(ctx, runID)

	s.ScheduledHealthchecksStart(ctx, domain)
	var attempted, failed int
	var failures []schedule.ItemFailure
	for _, t := range targets {
		if !t.IncludeInSchedule {
			continue
		}
		attempted++
		if _, err := s.Backup(runCtx, t.ContainerName); err != nil {
			if errors.Is(err, backup.ErrContainerNotInstalled) {
				continue // removed container: a skip (already recorded), not a job failure (#57)
			}
			failed++
			failures = append(failures, schedule.ItemFailure{Name: t.ContainerName, Reason: err.Error()})
			log.Printf("api: backup everything: containers: backup %q failed: %v", t.ContainerName, err) //nolint:gosec // G706: name is %q-quoted
		}
	}
	s.ScheduledHealthchecksResult(ctx, domain, attempted, failed)
	s.ScheduledNotifyResult(ctx, domain, attempted, failed, failures)
	// Retention before replication, same order every scheduled multi-item
	// domain uses (fewer snapshots left to copy). Plain ctx, not runCtx — see
	// everythingRunCtx's doc comment: main.go's real pruneAfterBulkFn/
	// replicateAfterBulkFn closures always run under a fresh, unsuppressed
	// context.Background(), so a batched-replication failure still notifies.
	s.PruneAfterBulk(ctx, domain)
	s.ReplicateOffsiteAfterBulk(ctx, domain)

	return EverythingDomainResult{Domain: domain, Attempted: attempted, Failed: failed, Failures: failures}
}

// everythingRunVMs mirrors everythingRunContainers for the VM domain,
// reproducing schedule.RunVMsJob's exact semantics inline (including
// ErrVMNotInstalled being a skip, not a failure, matching main.go's own
// scheduled VMs closure).
func (s *Service) everythingRunVMs(ctx context.Context, runID string, settings store.Settings) EverythingDomainResult {
	const domain = "vms"
	vms, err := s.store.ListVMTargets()
	if err != nil {
		log.Printf("api: backup everything: vms: list vm targets: %v", err)
		return everythingDomainFault(domain, err)
	}
	store.SortVMTargetsForRun(vms)                                    // #119: explicit VM backup order first, name-order tiebreak
	vms = schedule.DomainRunVMTargets(vms, settings.PerItemSchedules) // drop VMs on their own per-item cadence (#121)

	runCtx := everythingRunCtx(ctx, runID)

	s.ScheduledHealthchecksStart(ctx, domain)
	var attempted, failed int
	var failures []schedule.ItemFailure
	for _, v := range vms {
		if !v.IncludeInSchedule {
			continue
		}
		attempted++
		if _, err := s.BackupVM(runCtx, v.Name); err != nil {
			if errors.Is(err, backup.ErrVMNotInstalled) {
				continue // VM no longer on the host: a skip (already logged), not a job failure
			}
			failed++
			failures = append(failures, schedule.ItemFailure{Name: v.Name, Reason: err.Error()})
			log.Printf("api: backup everything: vms: backup %q failed: %v", v.Name, err) //nolint:gosec // G706: name is %q-quoted
		}
	}
	s.ScheduledHealthchecksResult(ctx, domain, attempted, failed)
	s.ScheduledNotifyResult(ctx, domain, attempted, failed, failures)
	// Plain ctx, not runCtx — see everythingRunCtx's doc comment.
	s.PruneAfterBulk(ctx, domain)
	s.ReplicateOffsiteAfterBulk(ctx, domain)

	return EverythingDomainResult{Domain: domain, Attempted: attempted, Failed: failed, Failures: failures}
}

// everythingRunFiles mirrors everythingRunContainers for the files domain,
// reproducing schedule.RunFilesJob's exact semantics inline (Enabled sets
// only; any other per-item error is logged and the loop continues — files has
// no "not installed" sentinel, matching main.go's own scheduled files
// closure). Unlike containers/vms, the real scheduled files job applies no
// per-item-schedule filtering (schedule.go has no DomainRun-style filter for
// file sets), so neither does this step.
func (s *Service) everythingRunFiles(ctx context.Context, runID string) EverythingDomainResult {
	const domain = "files"
	sets, err := s.store.ListFileSets()
	if err != nil {
		log.Printf("api: backup everything: files: list file sets: %v", err)
		return everythingDomainFault(domain, err)
	}

	runCtx := everythingRunCtx(ctx, runID)

	s.ScheduledHealthchecksStart(ctx, domain)
	var attempted, failed int
	var failures []schedule.ItemFailure
	for _, fs := range sets {
		if !fs.Enabled {
			continue
		}
		attempted++
		if _, err := s.BackupFileSet(runCtx, fs.ID); err != nil {
			failed++
			failures = append(failures, schedule.ItemFailure{Name: fs.Name, Reason: err.Error()})
			log.Printf("api: backup everything: files: backup %q failed: %v", fs.Name, err) //nolint:gosec // G706: name is %q-quoted
		}
	}
	s.ScheduledHealthchecksResult(ctx, domain, attempted, failed)
	s.ScheduledNotifyResult(ctx, domain, attempted, failed, failures)
	// Plain ctx, not runCtx — see everythingRunCtx's doc comment.
	s.PruneAfterBulk(ctx, domain)
	s.ReplicateOffsiteAfterBulk(ctx, domain)

	return EverythingDomainResult{Domain: domain, Attempted: attempted, Failed: failed, Failures: failures}
}

// everythingRunFlash runs the singleton flash backup, mirroring
// SetFlashJob's scheduled closure exactly: no bulk-replicate/message/
// Healthchecks suppression (there is nothing to aggregate — it is called
// exactly once), just WithRunGroup so the one run it produces still traces
// back to this pass.
func (s *Service) everythingRunFlash(ctx context.Context, runID string) EverythingDomainResult {
	const domain = "flash"
	if _, err := s.BackupFlash(WithRunGroup(ctx, runID)); err != nil {
		log.Printf("api: backup everything: flash: backup failed: %v", err)
		return everythingSingletonFault(domain, err)
	}
	return EverythingDomainResult{Domain: domain, Attempted: 1}
}

// everythingRunConfig runs the singleton config (self) backup, mirroring
// SetConfigJob's scheduled closure exactly — see everythingRunFlash.
func (s *Service) everythingRunConfig(ctx context.Context, runID string) EverythingDomainResult {
	const domain = "config"
	if _, err := s.BackupConfig(WithRunGroup(ctx, runID)); err != nil {
		log.Printf("api: backup everything: config: backup failed: %v", err)
		return everythingSingletonFault(domain, err)
	}
	return EverythingDomainResult{Domain: domain, Attempted: 1}
}

// everythingDomainFault builds the EverythingDomainResult for a domain that
// faulted BEFORE any item could even be attempted (e.g. ListTargetsScheduleOrder/
// ListVMTargets/ListFileSets itself erroring) — Attempted stays 0, Failed is 1,
// and the one synthetic ItemFailure carries the reason for the breakdown.
func everythingDomainFault(domain string, err error) EverythingDomainResult {
	return EverythingDomainResult{
		Domain:   domain,
		Failed:   1,
		Failures: []schedule.ItemFailure{{Name: domain, Reason: err.Error()}},
	}
}

// everythingSingletonFault builds the EverythingDomainResult for a failed
// singleton domain (flash/config): Attempted is 1 (it was always attempted —
// singletons have no "eligible items" concept), Failed is 1.
func everythingSingletonFault(domain string, err error) EverythingDomainResult {
	return EverythingDomainResult{
		Domain:    domain,
		Attempted: 1,
		Failed:    1,
		Failures:  []schedule.ItemFailure{{Name: domain, Reason: err.Error()}},
	}
}

// everythingBreakdown joins every domain's formatEverythingDomain line into
// the parent run's structured error text (design spec, decision 3), bounded
// to everythingBreakdownMaxLen like every other run error message in this
// package (see truncateRunErr).
func everythingBreakdown(results []EverythingDomainResult) string {
	parts := make([]string, 0, len(results))
	for _, r := range results {
		parts = append(parts, formatEverythingDomain(r))
	}
	s := strings.Join(parts, "; ")
	if len(s) > everythingBreakdownMaxLen {
		s = s[:everythingBreakdownMaxLen]
	}
	return s
}

// formatEverythingDomain renders one domain's line of the breakdown. It
// treats every domain uniformly (multi-item and singleton alike, since
// EverythingDomainResult represents both the same way):
//
//   - Attempted == 0, Failed == 0: "<domain>: ok" — nothing was eligible this
//     pass (e.g. no file sets defined), a benign no-op, not a failure.
//   - Attempted == 0, Failed == 1: "<domain>: failed (<reason>)" — the domain
//     faulted before any item was attempted.
//   - Failed == 0, Attempted > 0: "<domain>: N/N ok".
//   - Failed > 0: "<domain>: ok/attempted ok (<item>: <reason>, …)".
func formatEverythingDomain(r EverythingDomainResult) string {
	if r.Attempted == 0 {
		if r.Failed == 0 {
			return r.Domain + ": ok"
		}
		detail := "unknown error"
		if len(r.Failures) > 0 {
			detail = r.Failures[0].Reason
		}
		return fmt.Sprintf("%s: failed (%s)", r.Domain, detail)
	}
	ok := r.Attempted - r.Failed
	if r.Failed == 0 {
		return fmt.Sprintf("%s: %d/%d ok", r.Domain, ok, r.Attempted)
	}
	details := make([]string, 0, len(r.Failures))
	for _, f := range r.Failures {
		details = append(details, fmt.Sprintf("%s: %s", f.Name, f.Reason))
	}
	return fmt.Sprintf("%s: %d/%d ok (%s)", r.Domain, ok, r.Attempted, strings.Join(details, ", "))
}
