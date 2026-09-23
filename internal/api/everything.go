package api

// "Backup Everything" runs containers, vms, flash, files, zfs and config in
// turn under one parent run row (target_id = store.EverythingTargetID), stamps
// every child run with that row's group (WithRunGroup), and fires the global
// pre and post hooks around the whole pass so a dead-man's switch can watch it.

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

// everythingBreakdownMaxLen bounds the per-domain breakdown stored as the
// parent run's error text, the same 500 characters truncateRunErr allows.
const everythingBreakdownMaxLen = 500

// EverythingSummary is the outcome of one "Backup Everything" pass. RunID is
// the parent run, which every child run of the pass carries as GroupID. Status
// and Error are what was written to the parent run; Domains holds the results
// the breakdown was built from.
type EverythingSummary struct {
	RunID   string
	Status  string // "success" or "failed"
	Error   string // the breakdown; empty on a clean pass
	Domains []EverythingDomainResult
}

// EverythingDomainResult is one domain's outcome within a pass, in the shape
// schedule.RunContainersJob and its siblings return. Flash and config always
// have Attempted 1. Attempted 0 with Failed 0 means nothing was eligible;
// Attempted 0 with Failed 1 means the domain failed before any item was tried,
// and Failures[0] carries the reason.
type EverythingDomainResult struct {
	Domain    string
	Attempted int
	Failed    int
	Failures  []schedule.ItemFailure
}

// everythingStep is one domain of a pass with the operator's switch for it,
// so the enabled check lives in one loop instead of five call sites.
type everythingStep struct {
	domain  string
	enabled bool
	run     func() EverythingDomainResult
}

// ErrEverythingInFlight is returned by BackupEverything while another pass is
// running. The HTTP handler answers the same case with 409.
var ErrEverythingInFlight = errors.New("a Backup Everything pass is already running")

// BackupEverything runs one "Backup Everything" pass and returns once every
// domain has been attempted. Domain and item failures go into the parent run's
// breakdown, not into the returned error, as with RunContainersJob and the
// other scheduled jobs. An error means the pass could not be recorded at all:
// reading Settings or starting the parent run failed, or a pass is already in
// flight.
//
// The single-flight guard is taken here and not only in StartBackupEverything
// because the scheduler calls this function directly. Otherwise a manual
// "Run now" during a nightly pass would start a second one, backing up every
// domain twice and firing the post-hook twice. cron's SkipIfStillRunning only
// keeps a scheduled pass from overlapping itself.
func (s *Service) BackupEverything(ctx context.Context) (_ EverythingSummary, retErr error) {
	if !s.everythingActive.CompareAndSwap(false, true) {
		return EverythingSummary{}, ErrEverythingInFlight
	}
	defer s.everythingActive.Store(false)
	// cron.Recover keeps the process alive after a panic in a domain step, but
	// nothing would close the parent run row, and the Activity Log would show
	// the pass as running until a restart.
	defer s.recoverOperation("backup everything: "+store.EverythingTargetID, &retErr, func(msg string) {
		s.failStuckRun(store.EverythingTargetID, msg)
	})
	return s.backupEverythingHoldingGuard(ctx)
}

// backupEverythingHoldingGuard is the pass itself. The caller holds
// everythingActive and releases it afterwards. StartBackupEverything takes it
// synchronously so the handler can answer 409 without waiting on a goroutine.
func (s *Service) backupEverythingHoldingGuard(ctx context.Context) (EverythingSummary, error) {
	settings, err := s.store.GetSettings()
	if err != nil {
		return EverythingSummary{}, fmt.Errorf("backup everything: read settings: %w", err)
	}

	// A failing or hanging pre-hook must not block the pass it gates.
	// HostShell.Run has its own timeout, so ctx is passed through as is.
	if settings.EverythingPreHook != "" {
		if err := s.hostShell.Run(ctx, settings.EverythingPreHook); err != nil {
			log.Printf("api: backup everything: pre-hook failed (best-effort, pass continues): %v", err)
		}
	}

	// Every child run of the domain steps below gets group_id = runID. If the
	// parent run cannot be recorded, return without the post-hook: a
	// dead-man's-switch ping for a pass that never ran would be a false "done".
	runID, err := s.store.StartRun(store.EverythingTargetID, "backup")
	if err != nil {
		return EverythingSummary{}, fmt.Errorf("backup everything: start run: %w", err)
	}

	// A failing step, even one that fails before any item, is recorded in its
	// own result and does not stop the others.
	//
	// A domain switched off in Settings is left out, as in rpoStatus, the
	// overdue watchdog and drillTasks, and the UI hides its tab. Otherwise a
	// host without an Unraid flash would fail the parent run on every pass, and
	// FlashEnabled=false on Unraid would still create a flash repository.
	steps := []everythingStep{
		{"containers", settings.ContainersEnabled, func() EverythingDomainResult { return s.everythingRunContainers(ctx, runID, settings) }},
		{"vms", settings.VMsEnabled, func() EverythingDomainResult { return s.everythingRunVMs(ctx, runID, settings) }},
		{"flash", settings.FlashEnabled, func() EverythingDomainResult { return s.everythingRunFlash(ctx, runID) }},
		{"files", settings.FilesEnabled, func() EverythingDomainResult { return s.everythingRunFiles(ctx, runID, settings) }},
		{zfsDomain, settings.ZFSEnabled, func() EverythingDomainResult { return s.everythingRunZFS(ctx, runID, settings) }},
		{"config", settings.ConfigEnabled, func() EverythingDomainResult { return s.everythingRunConfig(ctx, runID) }},
	}
	results := make([]EverythingDomainResult, 0, len(steps))
	for _, step := range steps {
		if !step.enabled {
			log.Printf("api: backup everything: %s skipped, the domain is switched off in Settings", step.domain)
			continue
		}
		results = append(results, step.run())
	}
	if len(results) == 0 {
		// Not an error, but a "Backup Everything" schedule with every domain off
		// looks like protection on the dashboard, so it gets logged.
		log.Print("api: backup everything: no domain is switched on, so the pass backed up nothing")
	}

	// The post-hook runs exactly once after every step, whatever the outcome;
	// the dead-man's switch relies on that. Its failure does not change the
	// recorded status.
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
		// The pass has run, so there is nothing for a caller to retry.
		log.Printf("api: backup everything: finish run %s: %v", runID, err)
	}

	return EverythingSummary{RunID: runID, Status: status, Error: errMsg, Domains: results}, nil
}

// StartBackupEverything starts a "Backup Everything" pass in the background and
// returns at once, like StartBackupAll. A call while a pass is running returns
// (false, nil). The pass is detached from the request's cancellation and needs
// no deadline of its own, since every domain step applies its own hold and
// hard cap.
//
// Unlike StartBackupAll there is no busy check up front: each step takes its
// domain lock like any other caller and waits while the domain is busy.
func (s *Service) StartBackupEverything(ctx context.Context) (bool, error) {
	if !s.everythingActive.CompareAndSwap(false, true) {
		return false, nil
	}
	bctx := context.WithoutCancel(ctx)
	go func() {
		// A panic in the pass would otherwise take the process down and leave
		// the parent run row "running", because the pass's FinishRun was
		// skipped. Child runs are closed by their own domain code.
		defer s.recoverOperation("backup everything: "+store.EverythingTargetID, nil, func(msg string) {
			s.failStuckRun(store.EverythingTargetID, msg)
		})
		defer s.everythingActive.Store(false)
		// The guard is already held, so BackupEverything would refuse.
		if _, err := s.backupEverythingHoldingGuard(bctx); err != nil {
			log.Printf("api: backup everything: pass failed to start: %v", err)
		}
	}()
	return true, nil
}

// EverythingInProgress reports whether a "Backup Everything" pass is running.
// Tests poll it to wait for StartBackupEverything's goroutine.
func (s *Service) EverythingInProgress() bool { return s.everythingActive.Load() }

// everythingRunCtx is the context the multi-item steps pass to each item's
// backup: stamped with runID, and with inline off-site replication and
// per-item Healthchecks and message notifications suppressed, as in main.go's
// scheduled closures. The step then sends one aggregate ping and runs one
// batched replication itself.
//
// PruneAfterBulk and ReplicateOffsiteAfterBulk get the plain ctx, as main.go's
// pruneAfterBulkFn and replicateAfterBulkFn do, so a failed batched
// replication still notifies. Flash and config only need WithRunGroup, like
// SetFlashJob and SetConfigJob.
func everythingRunCtx(ctx context.Context, runID string) context.Context {
	return WithRunGroup(
		WithBulkReplicateSuppressed(notify.WithMessagesSuppressed(notify.WithHealthchecksSuppressed(ctx))),
		runID,
	)
}

// everythingRunContainers backs up every container with IncludeInSchedule, with
// the semantics of schedule.RunContainersJob: items on their own per-item
// schedule are left out, ErrContainerNotInstalled is a skip, and any other
// error is recorded while the loop goes on.
func (s *Service) everythingRunContainers(ctx context.Context, runID string, settings store.Settings) EverythingDomainResult {
	const domain = "containers"
	targets, err := s.store.ListTargetsScheduleOrder()
	if err != nil {
		log.Printf("api: backup everything: containers: list targets: %v", err)
		return everythingDomainFault(domain, err)
	}
	targets = schedule.DomainRunTargets(targets, settings.PerItemSchedules)
	if !schedule.DomainRunHasWork(targets) {
		return everythingDomainIdle(domain)
	}

	// The items and the summary below share one tally, so a dump that failed in
	// summary mode is named in the one message the round sends.
	ctx = withDBDumpTally(ctx)
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
				continue // a removed container is a skip and already recorded
			}
			failed++
			failures = append(failures, schedule.ItemFailure{Name: t.ContainerName, Reason: truncateRunErr(err)})
			log.Printf("api: backup everything: containers: backup %q failed: %v", t.ContainerName, err) //nolint:gosec // G706: name is %q-quoted
		}
	}
	s.ScheduledHealthchecksResult(ctx, domain, attempted, failed)
	s.ScheduledNotifyResult(ctx, domain, attempted, failed, failures)
	// Pruning first leaves fewer snapshots to copy. Plain ctx, see
	// everythingRunCtx.
	s.PruneAfterBulk(ctx, domain)
	s.ReplicateOffsiteAfterBulk(ctx, domain)

	return EverythingDomainResult{Domain: domain, Attempted: attempted, Failed: failed, Failures: failures}
}

// everythingRunVMs is everythingRunContainers for VMs, with the semantics of
// schedule.RunVMsJob (ErrVMNotInstalled is a skip).
func (s *Service) everythingRunVMs(ctx context.Context, runID string, settings store.Settings) EverythingDomainResult {
	const domain = "vms"
	vms, err := s.store.ListVMTargets()
	if err != nil {
		log.Printf("api: backup everything: vms: list vm targets: %v", err)
		return everythingDomainFault(domain, err)
	}
	store.SortVMTargetsForRun(vms)
	vms = schedule.DomainRunVMTargets(vms, settings.PerItemSchedules)
	if !schedule.DomainRunHasVMWork(vms) {
		return everythingDomainIdle(domain)
	}

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
				continue // a VM gone from the host is a skip and already logged
			}
			failed++
			failures = append(failures, schedule.ItemFailure{Name: v.Name, Reason: truncateRunErr(err)})
			log.Printf("api: backup everything: vms: backup %q failed: %v", v.Name, err) //nolint:gosec // G706: name is %q-quoted
		}
	}
	s.ScheduledHealthchecksResult(ctx, domain, attempted, failed)
	s.ScheduledNotifyResult(ctx, domain, attempted, failed, failures)
	s.PruneAfterBulk(ctx, domain)
	s.ReplicateOffsiteAfterBulk(ctx, domain)

	return EverythingDomainResult{Domain: domain, Attempted: attempted, Failed: failed, Failures: failures}
}

// everythingRunFiles is everythingRunContainers for file sets, with the
// semantics of schedule.RunFilesJob: only enabled sets run, and a set with a
// schedule of its own is left to it.
func (s *Service) everythingRunFiles(ctx context.Context, runID string, settings store.Settings) EverythingDomainResult {
	const domain = "files"
	sets, err := s.store.ListFileSets()
	if err != nil {
		log.Printf("api: backup everything: files: list file sets: %v", err)
		return everythingDomainFault(domain, err)
	}
	sets = schedule.DomainRunFileSets(sets, settings.PerItemSchedules)
	if !schedule.DomainRunHasFileWork(sets) {
		return everythingDomainIdle(domain)
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
			failures = append(failures, schedule.ItemFailure{Name: fs.Name, Reason: truncateRunErr(err)})
			log.Printf("api: backup everything: files: backup %q failed: %v", fs.Name, err) //nolint:gosec // G706: name is %q-quoted
		}
	}
	s.ScheduledHealthchecksResult(ctx, domain, attempted, failed)
	s.ScheduledNotifyResult(ctx, domain, attempted, failed, failures)
	s.PruneAfterBulk(ctx, domain)
	s.ReplicateOffsiteAfterBulk(ctx, domain)

	return EverythingDomainResult{Domain: domain, Attempted: attempted, Failed: failed, Failures: failures}
}

// everythingRunZFS runs the eligible ZFS items like everythingRunFiles does for
// file sets.
func (s *Service) everythingRunZFS(ctx context.Context, runID string, settings store.Settings) EverythingDomainResult {
	items, err := s.store.ListZFSDatasets()
	if err != nil {
		log.Printf("api: backup everything: zfs: list items: %v", err)
		return everythingDomainFault(zfsDomain, err)
	}
	items = schedule.DomainRunZFSDatasets(items, settings.PerItemSchedules)
	if !schedule.DomainRunHasZFSWork(items) {
		return everythingDomainIdle(zfsDomain)
	}

	runCtx := everythingRunCtx(ctx, runID)

	s.ScheduledHealthchecksStart(ctx, zfsDomain)
	var attempted, failed int
	var failures []schedule.ItemFailure
	for _, d := range items {
		if !d.Enabled {
			continue
		}
		attempted++
		if _, err := s.BackupZFSDataset(runCtx, d.ID); err != nil {
			failed++
			failures = append(failures, schedule.ItemFailure{Name: d.Dataset, Reason: truncateRunErr(err)})
			log.Printf("api: backup everything: zfs: backup %q failed: %v", d.Dataset, err) //nolint:gosec // G706: name is %q-quoted
		}
	}
	s.ScheduledHealthchecksResult(ctx, zfsDomain, attempted, failed)
	s.ScheduledNotifyResult(ctx, zfsDomain, attempted, failed, failures)
	s.PruneAfterBulk(ctx, zfsDomain)
	s.ReplicateOffsiteAfterBulk(ctx, zfsDomain)

	return EverythingDomainResult{Domain: zfsDomain, Attempted: attempted, Failed: failed, Failures: failures}
}

// everythingRunFlash runs the flash backup like SetFlashJob's closure. A single
// run has nothing to aggregate, so only WithRunGroup is applied.
func (s *Service) everythingRunFlash(ctx context.Context, runID string) EverythingDomainResult {
	const domain = "flash"
	if _, err := s.BackupFlash(WithRunGroup(ctx, runID)); err != nil {
		log.Printf("api: backup everything: flash: backup failed: %v", err)
		return everythingSingletonFault(domain, err)
	}
	return EverythingDomainResult{Domain: domain, Attempted: 1}
}

// everythingRunConfig runs BombVault's own config backup like SetConfigJob's
// closure.
func (s *Service) everythingRunConfig(ctx context.Context, runID string) EverythingDomainResult {
	const domain = "config"
	if _, err := s.BackupConfig(WithRunGroup(ctx, runID)); err != nil {
		log.Printf("api: backup everything: config: backup failed: %v", err)
		return everythingSingletonFault(domain, err)
	}
	return EverythingDomainResult{Domain: domain, Attempted: 1}
}

// everythingDomainIdle is the result for a multi-item domain with nothing to
// back up. Returning it early skips the Healthchecks ping pair, the prune and
// the off-site copy, as the scheduler's DomainRunHasWork gate does. Otherwise
// a box without VMs would ping "0 of 0 items succeeded" to the dead-man's
// switch and turn a red check green.
func everythingDomainIdle(domain string) EverythingDomainResult {
	return EverythingDomainResult{Domain: domain}
}

// everythingDomainFault is the result for a domain that failed before any item
// was attempted, for example because its target list could not be read.
func everythingDomainFault(domain string, err error) EverythingDomainResult {
	return EverythingDomainResult{
		Domain:   domain,
		Failed:   1,
		Failures: []schedule.ItemFailure{{Name: domain, Reason: truncateRunErr(err)}},
	}
}

// everythingSingletonFault is the result for a failed flash or config backup.
func everythingSingletonFault(domain string, err error) EverythingDomainResult {
	return EverythingDomainResult{
		Domain:    domain,
		Attempted: 1,
		Failed:    1,
		Failures:  []schedule.ItemFailure{{Name: domain, Reason: truncateRunErr(err)}},
	}
}

// everythingBreakdown joins the domain lines into the parent run's error text,
// cut at everythingBreakdownMaxLen.
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

// formatEverythingDomain renders one domain's line of the breakdown:
//
//   - nothing attempted or failed: "<domain>: ok"
//   - failed before any item: "<domain>: failed (<reason>)"
//   - every item ok: "<domain>: N/N ok"
//   - otherwise: "<domain>: ok/attempted ok (<item>: <reason>, …)"
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
