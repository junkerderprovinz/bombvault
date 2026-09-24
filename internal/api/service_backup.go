package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/backup"
	"github.com/junkerderprovinz/bombvault/internal/dockercli"
	"github.com/junkerderprovinz/bombvault/internal/model"
	"github.com/junkerderprovinz/bombvault/internal/notify"
	"github.com/junkerderprovinz/bombvault/internal/progress"
	"github.com/junkerderprovinz/bombvault/internal/store"
	"github.com/junkerderprovinz/bombvault/internal/template"
)

// runGroupKey marks a context whose backup call is one child step of a
// "Backup Everything" pass, a sequential run over every domain (containers,
// vms, flash, files, config) triggered as one unit, so that for example a
// dead-man's-switch ping fires only once everything is done. The value is
// the parent run's id; runsAdapter and startedRunsAdapter read it via
// runGroupFromContext and stamp it onto the child run they just started
// (store.SetRunGroup), so that run can be traced back to its pass.
type runGroupKey struct{}

// WithRunGroup marks ctx as belonging to the "Backup Everything" pass whose
// parent run id is groupID (see runGroupKey). Set by BackupEverything around
// each domain's own backup entry point (s.Backup/s.BackupVM/s.BackupFlash/
// s.BackupFileSet/s.BackupConfig); read by runsAdapter/startedRunsAdapter
// when they record that call's child run.
func WithRunGroup(ctx context.Context, groupID string) context.Context {
	return context.WithValue(ctx, runGroupKey{}, groupID)
}

// runGroupFromContext reports the parent run id this context's backup call
// belongs to, or "" when it is not part of a "Backup Everything" pass. A nil
// ctx (such as a zero-value runsAdapter built without one) counts as no
// group rather than panicking.
func runGroupFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	v, _ := ctx.Value(runGroupKey{}).(string)
	return v
}

// ErrSelfBackup is returned when a backup targets BombVault's own container.
// Backing it up stops the container mid-run (stop → backup → start), which kills
// the very process doing the backup and takes the app down. Its configuration is
// recovered separately via the encrypted definition mirror (Discover), so there
// is nothing to gain and a crash to lose.
var ErrSelfBackup = errors.New("BombVault won't back up its own container (it would stop itself mid-backup); its configuration is recovered via Discover")

// selfContainerName returns BombVault's own container name, cached after the
// first success. BOMBVAULT_SELF_CONTAINER overrides the lookup. An empty result
// is not cached, so a call made before Docker is reachable is retried later.
func (s *Service) selfContainerName(ctx context.Context) string {
	s.selfMu.Lock()
	defer s.selfMu.Unlock()
	if s.selfResolved {
		return s.selfName
	}
	if v := strings.TrimSpace(os.Getenv("BOMBVAULT_SELF_CONTAINER")); v != "" {
		s.selfName, s.selfResolved = v, true
		return s.selfName
	}
	name, err := s.docker.Self(ctx)
	if err != nil || name == "" {
		return "" // Docker not reachable or not in a container yet; retry next time
	}
	s.selfName, s.selfResolved = name, true
	return s.selfName
}

// SelfContainerName exposes the detected own-container name to the HTTP layer so
// the container list can flag it (the UI hides its backup action / excludes it
// from "select all").
func (s *Service) SelfContainerName(ctx context.Context) string {
	return s.selfContainerName(ctx)
}

// selfRestartDelay is the brief pause before BombVault restarts its own container,
// so the HTTP response for the triggering request flushes to the client first.
// A package var (not a const) so tests can shrink it.
var selfRestartDelay = 1500 * time.Millisecond

// ScheduleSelfRestart restarts BombVault's own container over the Docker socket
// shortly after returning, so a staged config restore is applied on the reboot
// (the daemon completes the stop+start even though this process dies mid-stop).
// It returns whether an auto-restart was scheduled: false when the self container
// can't be resolved (Docker unreachable / not in a container), in which case the
// caller instructs the user to restart the container manually.
func (s *Service) ScheduleSelfRestart() bool {
	name := s.selfContainerName(context.Background())
	if name == "" {
		return false
	}
	go func() {
		time.Sleep(selfRestartDelay)
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := s.docker.Restart(ctx, name, 10*time.Second); err != nil {
			log.Printf("api: self-restart of %q failed: %v (restart the container manually to apply)", name, err)
			s.batchActive.Store(false) // release the guard so operations can resume; user restarts manually
		}
	}()
	return true
}

// Backup runs a full container backup: resolve repo + mode, ensure the repo,
// inspect the container, find-or-create its target, and drive the orchestrator.
func (s *Service) Backup(ctx context.Context, name string) (_ backup.Summary, retErr error) {
	// A backup must survive the client that triggered it disconnecting, whether
	// by closing the browser tab or by stopping the very container the
	// BombVault UI runs in. Detach from the request's cancellation (keeping its
	// values) with a generous hard cap so a wedged run can't hold the domain
	// lock forever.
	ctx, cancel := backupHoldCtx(ctx)
	defer cancel()
	// Detached from the caller, but reachable by shutdown: a closing browser
	// tab must not stop a backup, while the process itself leaving must, or the
	// run is killed anyway without anyone recording that it happened.
	s.registerBackupCancel("container:"+name, cancel)
	defer s.unregisterBackupCancel("container:" + name)
	// Never back up BombVault's own container: stopping it mid-run kills this process.
	if self := s.selfContainerName(ctx); self != "" && name == self {
		return backup.Summary{}, ErrSelfBackup
	}
	defer s.lockDomain("containers")() // serialise per repo; blocks maintenance ops meanwhile

	// #64: a domain-wide fault (repo mount lost, disk full, restic repo error)
	// that begins mid-batch trips one of the pre-flight early returns below
	// (settings, repo path, EnsureRepo, inspect, empty-paths guard, upsert) for
	// every remaining container. None of those reach backup.BackupContainer,
	// the only other place a failed run is recorded, so they would leave
	// nothing in the dashboard heatmap or history and only a bare "N failed"
	// count in the notification. Resolve the target up front and, via the
	// named-return finisher below, record a failed run carrying the real
	// reason for any such early return. Once BackupContainer takes over run
	// bookkeeping, orchestrated=true keeps this from double-recording; a
	// NotFound target is excluded too, since it records its own "skipped" run
	// (recordAndNotifyContainerSkip).
	var targetID string
	if tg, tErr := s.store.GetTargetByContainer(name); tErr == nil {
		targetID = tg.ID
	}
	orchestrated := false
	defer func() {
		if retErr != nil && !orchestrated && !errors.Is(retErr, backup.ErrContainerNotInstalled) {
			s.recordPreflightFailure("Backup", name, targetID, retErr)
		}
	}()

	settings, err := s.store.GetSettings()
	if err != nil {
		return backup.Summary{}, fmt.Errorf("read settings: %w", err)
	}
	item := store.ItemRef{Domain: "containers", Key: name}
	step, err := s.prepareHome(ctx, settings, item)
	if err != nil {
		return backup.Summary{}, err
	}

	in, err := s.docker.Inspect(ctx, name)
	if err != nil {
		// The container is gone but is still a scheduled target. A scheduled target
		// can outlive its container, so skip it with a sentinel instead of failing:
		// the scheduler treats ErrContainerNotInstalled as a skip, so the nightly
		// job neither errors nor false-alarms Healthchecks (#57). Record a
		// "skipped" run (so the dashboard shows it and agrees with the green
		// aggregate ping) and warn the user once. Mirrors the VM path (BackupVM →
		// ErrVMNotInstalled). Returns before any real run is recorded.
		if dockercli.IsNotFound(err) {
			log.Printf("api: Backup: skipping %q: not present on host (not installed; backups only)", name) //nolint:gosec // G706: name is %q-quoted
			s.recordAndNotifyContainerSkip(ctx, name)
			return backup.Summary{}, backup.ErrContainerNotInstalled
		}
		return backup.Summary{}, fmt.Errorf("inspect container: %w", err)
	}
	// The paths actually backed up: the explicit folder selection if set, else
	// the automatic appdata detection, filtered to those that exist. A
	// stateless container ends up with an empty list and a definition-only
	// backup (its template and inspect are still captured so it can be
	// recreated on restore). Both halves of the selection come out of one
	// read: the includes become the positionals, and selection feeds the
	// --exclude tail further down (see effectiveBackupPathsWithSelection).
	effective, selection := s.effectiveBackupPathsWithSelection(name, in)

	// Guard against a silent no-op: if a previous backup captured data (or the
	// user selected folders) but every path now resolves away, e.g. because
	// the appdata share isn't mounted or HOST_SOURCE_ROOT is misconfigured,
	// refuse instead of recording an empty backup that looks successful and
	// overwrites the stored path list. A first backup of a new or stateless
	// container is unaffected.
	//
	// Gone, not merely absent (#181): an empty result also happens when the
	// user has deselected every folder on purpose, which leaves the container
	// correctly stateless and makes a definition-only backup the right
	// outcome. The guard measures what its message claims: whether the data a
	// previous backup captured has disappeared from disk.
	if s.emptyBackupIsUnreachable(name, effective) {
		err := fmt.Errorf("backup %q: its backup folders are not reachable right now (is the appdata share mounted?). Refusing an empty backup that would look successful", name)
		s.notifyBackup(ctx, "container", name, false, backup.Summary{}, err)
		return backup.Summary{}, err
	}

	if step, err = s.recordHome(ctx, settings, item, step); err != nil {
		return backup.Summary{}, err
	}
	repo, mode := step.repo, step.mode

	// Persist the recreate recipe (self-contained: inspect + template + backup
	// paths) so restore works even after the container has been deleted.
	xml, _, _ := template.Read(s.cfg.FlashTemplatesDir, name)
	defBytes, _ := json.Marshal(containerDefinition{Inspect: in, TemplateXML: xml, AppdataPaths: effective})
	defJSON := string(defBytes)

	tg, err := s.store.UpsertTarget(store.Target{ContainerName: name, AppdataPaths: effective, Definition: defJSON})
	if err != nil {
		return backup.Summary{}, fmt.Errorf("upsert target: %w", err)
	}

	// Compile the per-root CACHEDIR.TAG toggles into the item-level union the
	// argv flag needs. tg comes from UpsertTarget's authoritative re-read (the
	// ON CONFLICT clause never touches exclude_caches), so this is a fresh read
	// of the stored map on every backup: a toggle saved between two backups
	// affects exactly the later one. The union runs over the stored map whether
	// or not that root is currently included in the selection, because
	// restic's --exclude-caches applies to every positional source and per-root
	// gating would be a lie either way.
	mode.ExcludeCaches = anyRootExcludeCaches(tg.ExcludeCaches)

	// The formerly: tags take the bare former names, and the mirrored definition
	// records each with its link time. A failed read costs this one backup
	// those tags and the mirror update, not the backup itself.
	aliases, aliasErr := s.store.TargetAliases("container", tg.ID)
	if aliasErr != nil {
		log.Printf("api: backup: aliases of %q: %v", name, aliasErr) //nolint:gosec // G706: %q-quoted
	}

	// Give each dependency its own run-state so the backup never starts a
	// container the user had already stopped (#33): inspect each by name and
	// carry WasRunning (mirroring the target's in.Running). A dependency that
	// cannot be inspected (e.g. removed) is logged and left untouched.
	var deps []backup.StopContainer
	for _, dep := range tg.StopContainers {
		di, dErr := s.docker.Inspect(ctx, dep)
		if dErr != nil {
			log.Printf("api: backup: inspect dependency %q: %v (leaving as-is)", dep, dErr) //nolint:gosec // G706: dep is %q-quoted
			continue
		}
		// Carry the dependency's compose identity so the restart-after-backup phase
		// can bring the stopped set back up in depends_on order (dependencies first)
		// and, when enabled, wait for each to be healthy before its dependents (#119).
		deps = append(deps, backup.StopContainer{
			Name:       dep,
			WasRunning: di.Running,
			Service:    composeService(di.Config.Labels),
			DependsOn:  parseDependsOn(di.Config.Labels),
		})
	}

	// "Update after successful backup" recreates the container (stop, remove,
	// create) after the backup. Dependents already brought back by then would
	// break against the target while it is torn down (a dependent firing
	// against a down target gets connection refused, #119). Hand the update to
	// the orchestrator as its WhileDependentsStopped hook so it runs inside the
	// stop window and the dependents restart only after the recreate. Only for
	// a running, opted-in container; otherwise nil, and the dependents restart
	// right after the backup.
	var whileDepsStopped func()
	if tg.UpdateAfterBackup && in.Running {
		whileDepsStopped = func() { s.updateContainerAfterBackup(ctx, name, in, tg.ID) }
	}

	pkey := "container:" + name
	// Healthchecks /start ping: deferred to here, past every pre-flight early-return,
	// so the paired done/fail notifyBackup below always follows (no dangling /start).
	s.notifyBackupStart(ctx, "container")
	bctx, startedAt := s.progBegin(ctx, pkey, "backup")
	// BackupContainer owns run bookkeeping from here (it records its own failed
	// or success run), so the pre-flight failure finisher above stands down to
	// avoid a double record.
	orchestrated = true
	sum, err := backup.BackupContainer(bctx, backup.BackupDeps{
		ContainerRef:           name,
		FormerNames:            aliasOldNames(aliases),
		ContainerName:          name,
		RepoPath:               repo,
		AppdataPaths:           effective,
		StopTimeout:            30 * time.Second,
		TargetID:               tg.ID,
		SnapshotTemplatesDir:   filepath.Join(s.cfg.DataDir, "templates"),
		FlashTemplatesDir:      s.cfg.FlashTemplatesDir,
		WasRunning:             in.Running,
		PreHook:                tg.PreHook,
		PostHook:               tg.PostHook,
		StopContainers:         deps,
		HealthWait:             settings.RestartHealthWait,
		HealthTimeout:          time.Duration(settings.RestartHealthTimeoutSec) * time.Second,
		WhileDependentsStopped: whileDepsStopped,
		// User-owned exclude patterns stay first; the selection-derived tail
		// enforces the stored exclusion branches strictly below an included root,
		// so the snapshot content matches what the stored selection advertises.
		// It comes from the same read as the positionals above, never from
		// UpsertTarget's later re-read: those two reads could disagree, and a
		// --exclude from one selection biting a positional from another is a
		// snapshot nobody asked for. Patterns travel as typed builder arguments
		// into BackupArgs (excludes before --, positionals after), never through a
		// shell.
		Excludes:  append(s.resolveExcludePatterns(tg.Excludes, in), excludedBranches(selection)...),
		Docker:    s.docker,
		Restic:    &resticAdapter{engine: s.engine, mode: mode, extraTags: s.directTags(settings, "containers", repo)},
		Templates: templatesAdapter{},
		Runs:      runsAdapter{st: s.store, ctx: ctx, svc: s, cancelKey: "container:" + name},
	})
	s.progEnd(pkey, "backup", err == nil, startedAt)
	s.notifyBackup(ctx, "container", name, err == nil, sum, err)
	if err != nil {
		return backup.Summary{}, err
	}

	// Mirror the definition (encrypted) onto the backup storage so a freshly
	// installed BombVault can rebuild its state via Discover after losing
	// /config. Best-effort: a write failure must never fail a good backup.
	// Without its aliases the mirror would lose its link records, so it keeps
	// what the last write left.
	if aliasErr != nil {
		log.Printf("api: backup: WARN the stored definition of %q stays as it was, since its aliases could not be read", name) //nolint:gosec // G706: name is %q-quoted
	} else if wErr := s.writeDefToStorage(settings, name, repo, defBytes, aliases); wErr != nil {
		log.Printf("api: backup: WARN could not persist definition for %q to storage: %v", name, wErr) //nolint:gosec // G706: name is %q-quoted
	}
	// #52: the optional post-backup image update already ran, if enabled, as
	// the orchestrator's WhileDependentsStopped hook (above), inside the stop
	// window, so a recreate of the target completes before its dependents are
	// restarted (#119). The backup and fresh snapshot are its safety net; a
	// failure there is logged and recorded as a failed "update" run, but never
	// fails the backup.
	//
	// This container's compose stack, if it has one and this is a single
	// backup rather than one item of a round. The project directory does not
	// travel inside a member's own snapshot (see resolveAppdataPaths), so
	// backing up one member by hand would otherwise leave the shared folder
	// unprotected until the next full round. A round suppresses this and does
	// it once at the end, so the folder is walked once per project, not once
	// per service. It keys on the same flag as the batched off-site
	// replication, so there is one notion of "part of a round".
	if !bulkReplicateSuppressed(ctx) {
		// The stack's own retention forgets without pruning: the container's
		// applyRetention right below prunes once for both, the same repo either
		// way, instead of two separate prune passes for one manual backup.
		if err := s.BackupStacks(WithBulkReplicateSuppressed(ctx), []string{name}); err != nil {
			// Logged, never fatal: the container's own data is already safe.
			log.Printf("api: backup: %v", err)
		}
	}
	// A renamed container's "keep last N" counts across the rename, as long as
	// no other machine has used the old name since.
	s.applyRetention(ctx, repo, settings, mode, s.containerIdentity(name), "containers")
	makeRepoReadable(repo, s.cfg.DataDir) // keep the local repo copyable off-box by a non-root user
	s.replicateOffsite(ctx, "containers", settings, repo, "container:"+name)
	s.collectStatsAfterItem(ctx, "containers")
	s.checkPrimaryRemoteBudget(ctx, "containers", repo, settings)
	return sum, nil
}

// updateContainerAfterBackup implements the per-container "update after
// backup" opt-in (#52): pull the container's image and, only if a newer
// image arrived, recreate the container from its live inspect (which then
// resolves to the new image). It is recorded as its own "update" run so the
// recreate shows in Run History. Best-effort: a failure here never fails
// the backup, and the fresh snapshot lets the user roll back a bad update.
func (s *Service) updateContainerAfterBackup(ctx context.Context, name string, in model.Inspect, targetID string) {
	ref := in.Config.Image
	if ref == "" {
		ref = in.Image
	}
	// #106: images in a private or sponsor-gated registry need credentials:
	// resolve the ref's registry host against the stored registry credentials
	// and pass the match along ("" means no credential, an anonymous pull).
	if err := s.docker.PullWithAuth(ctx, ref, s.registryAuthFor(ref)); err != nil {
		// Reached, but couldn't even check for an update (registry rate limit,
		// auth, network). Record a failed "update" run so "why wasn't this
		// updated?" is answerable from Run History and the Activity Log instead of
		// only the server log (#95). The backup itself already succeeded.
		log.Printf("api: update-after-backup: pull %q failed (backup is safe): %v", name, err) //nolint:gosec // G706: name is %q-quoted
		s.recordUpdateFailure(name, targetID, fmt.Errorf("pull image: %w", err))
		return
	}
	newID, err := s.docker.ImageID(ctx, ref)
	if err != nil {
		log.Printf("api: update-after-backup: resolve image id for %q failed: %v", name, err) //nolint:gosec // G706: name is %q-quoted
		s.recordUpdateFailure(name, targetID, fmt.Errorf("resolve image id: %w", err))
		return
	}
	// Nothing newer arrived: no recreate, and no run record either (44
	// "nothing happened" rows a night would drown Run History). The check did
	// complete, so stamp it on the target and let the UI show "checked, up to
	// date" instead of something indistinguishable from "never reached".
	if newID == "" || newID == in.Image {
		s.setUpdateCheck(name, "up-to-date")
		return
	}
	// context.Background(), not ctx: an "update" run is a side effect of the
	// container backup, not one of the five domain steps a "Backup Everything"
	// pass groups (see runGroupKey), and recordUpdateFailure's failure path
	// leaves it ungrouped too, so grouping never depends on which path fires.
	// This applies to all three runsAdapter sites below; ctx stays in use for
	// the Docker calls.
	runID, rErr := runsAdapter{st: s.store, ctx: context.Background()}.Start(targetID, "update")
	if rErr != nil {
		log.Printf("api: update-after-backup: start run for %q: %v", name, rErr) //nolint:gosec // G706: name is %q-quoted
		return
	}
	if err := s.recreateForUpdate(ctx, name, in); err != nil {
		_ = runsAdapter{st: s.store, ctx: context.Background()}.Finish(runID, "failed", "", 0, truncateRunErr(err))
		s.setUpdateCheck(name, "failed")
		log.Printf("api: update-after-backup: recreate %q failed (backup is safe): %v", name, err) //nolint:gosec // G706: name is %q-quoted
		return
	}
	_ = runsAdapter{st: s.store, ctx: context.Background()}.Finish(runID, "success", "", 0, "")
	s.setUpdateCheck(name, "updated")

	// #116: BombVault just recreated the container, so its image tag moved to
	// the new digest, but Unraid did not perform the update and still shows a
	// stale "update available" banner on the Docker tab because its cached
	// status file was never refreshed. Ask Unraid to run its own update-status
	// recheck over the existing host SSH link so it rewrites that file itself.
	// Only fires when an update happened; opt-out via the toggle, best-effort
	// and non-fatal.
	if st, sErr := s.store.GetSettings(); sErr == nil && st.ReconcileUnraidUpdateStatus {
		s.reconcileUnraidUpdateStatus(ctx, ref)
	}

	// #56: optionally remove the superseded old image. Opt-in (default off),
	// because the old image is what makes a fresh-snapshot rollback cheap.
	// Best-effort with force=false, so the daemon refuses while another
	// container still references the image (a shared base image is never
	// deleted).
	if st, sErr := s.store.GetSettings(); sErr == nil && st.PruneImageAfterUpdate && in.Image != "" {
		if rErr := s.docker.ImageRemove(ctx, in.Image); rErr != nil {
			log.Printf("api: update-after-backup: prune old image %s for %q: %v (kept)", shortID(in.Image), name, rErr) //nolint:gosec // G706: name is %q-quoted
		}
	}

	// #56: notify per updated container (opt-in), so the user can verify it
	// still works. It fires per container (updates are rare) rather than in the
	// scheduled summary. Healthchecks is suppressed so an update can't flip the
	// domain monitor; the message ctx carries no message-suppress flag, so it
	// delivers in summary mode.
	if c, cErr := s.NotifyConfig(); cErr == nil && c.NotifyOnUpdate {
		msg := fmt.Sprintf("Updated container %q to a newer image. Please verify it still works.", name)
		notify.Send(notify.WithHealthchecksSuppressed(context.Background()), c, "containers",
			notify.Event{Title: "BombVault", Message: msg, OK: true})
		if s.unraidGate(c.Unraid) && c.On == "always" {
			if e := s.sendUnraidNotify(ctx, "BombVault: container updated", msg, "normal"); e != nil {
				log.Printf("notify: unraid: %v", e)
			}
		}
	}
}

// recreateForUpdate stops, removes and recreates+starts the container from
// its captured inspect, which after the preceding Pull resolves to the
// newer image. It mirrors the restore recreate path minus the appdata
// restore (the data is already current). Containers that share this one's
// network namespace (network_mode: container:<name>) may need a manual
// restart afterwards.
func (s *Service) recreateForUpdate(ctx context.Context, name string, in model.Inspect) error {
	if err := s.docker.Stop(ctx, name, 30*time.Second); err != nil {
		_ = err // absent/already-stopped is fine; a real problem surfaces at Remove
	}
	if err := s.docker.Remove(ctx, name); err != nil {
		return fmt.Errorf("remove container: %w", err)
	}
	if err := s.docker.CreateAndStart(ctx, in, true); err != nil {
		return fmt.Errorf("recreate container: %w", err)
	}
	return nil
}

// reconcileUnraidUpdateStatus asks the platform to refresh its own cached
// update status for a container BombVault just recreated (#116), so on
// Unraid the Docker tab's stale "update available" banner clears. The
// host-side step lives behind s.platformFn() (see
// platform.Platform.ReconcileContainerUpdateStatus: Unraid's PHP-over-SSH
// recheck, a no-op on platforms without an equivalent). This is the
// best-effort boundary: a nil SSH link, a non-Unraid platform or a command
// error is only logged and never affects the backup or update outcome.
func (s *Service) reconcileUnraidUpdateStatus(ctx context.Context, ref string) {
	if err := s.platformFn().ReconcileContainerUpdateStatus(ctx, s.ssh, ref); err != nil {
		log.Printf("api: update-after-backup: unraid update-status reconcile for %q failed (harmless): %v", ref, err) //nolint:gosec // G706: ref is %q-quoted
	}
}

// setUpdateCheck stamps the outcome of a completed post-backup update check on
// the container's target row (last_update_check/last_update_result), so the
// Containers page can show "checked, up to date / updated / check failed"
// without a per-night run row. Best-effort: a store error is only logged.
func (s *Service) setUpdateCheck(name, result string) {
	if err := s.store.SetUpdateCheck(name, time.Now().Unix(), result); err != nil {
		log.Printf("api: update-after-backup: %q could not record update check: %v", name, err) //nolint:gosec // G706: name is %q-quoted
	}
}

// recordUpdateFailure records a failed "update" run so a reached but failed
// post-backup update (a registry check that failed before it could tell
// whether a newer image exists) shows in Run History and the Activity Log
// rather than only the server log (#95). It also stamps the target's
// update-check outcome as 'failed'. Best-effort: the backup already
// succeeded, so a bookkeeping error here is only logged. An update that was
// available but failed to apply is recorded by the recreate path.
func (s *Service) recordUpdateFailure(name, targetID string, cause error) {
	s.setUpdateCheck(name, "failed")
	// This "update" run is never part of a "Backup Everything" pass's grouped
	// children (see runGroupKey), so context.Background() loses nothing.
	runID, rErr := runsAdapter{st: s.store, ctx: context.Background()}.Start(targetID, "update")
	if rErr != nil {
		log.Printf("api: update-after-backup: %q could not record update failure: %v (cause: %v)", name, rErr, cause) //nolint:gosec // G706: name is %q-quoted
		return
	}
	_ = runsAdapter{st: s.store, ctx: context.Background()}.Finish(runID, "failed", "", 0, truncateRunErr(cause))
}

// StartBackupAll launches a server-side batch backup of the named
// containers, running them sequentially in a background goroutine. Running
// on the server, it survives the browser that started it going away,
// including when the batch stops the very container the BombVault UI is
// open in. Self and blank names are skipped, and a single container
// failing is logged while the batch continues.
//
// It returns (false, nil) if a batch is already running (the caller answers
// 409), or (false, err) if the containers domain is already busy with
// another op (scheduled backup, restore, prune, ...), which the handler
// maps to a 409 instead of launching a goroutine that blocks silently on
// the lock. Progress is published under "batch:containers" for an overall
// indicator, while each container still publishes its own
// "container:<name>" bar.
//
// Unlike a scheduled domain run, which aggregates its Healthchecks pings
// into one per run (#49), this manual multi-select keeps the per-item ping:
// it backs up an arbitrary subset, not the whole domain, so pinging the
// domain check "success" here would reset the scheduled-cadence monitor and
// could mask an overdue scheduled backup.
func (s *Service) StartBackupAll(ctx context.Context, names []string) (bool, error) {
	if !s.batchActive.CompareAndSwap(false, true) {
		return false, nil
	}
	if op, busy := s.domainBusy("containers"); busy {
		s.batchActive.Store(false)
		return false, fmt.Errorf("%s is running on containers", op)
	}
	// Detach immediately so the run, and the self-detection it depends on, is
	// independent of the request that started it (canceled the moment the
	// handler returns). Each per-container Backup applies its own hard timeout,
	// so the batch needs no deadline of its own; WithoutCancel keeps request
	// values without a cancel func to leak. #95: each container's inline
	// off-site replication is suppressed and the whole batch is replicated once
	// after the loop (ReplicateOffsiteAfterBulk below), as on the scheduled
	// path.
	bctx := WithBulkReplicateSuppressed(context.WithoutCancel(ctx))
	go func() {
		// Batch-level safety net: a panic outside the per-item loop (ordering,
		// publishBatch, the post-loop prune and replicate) has no single container
		// to blame, so it is only contained. Each item gets its own, more precise
		// recovery (backupOneForBatch), so one bad container can't abort the rest
		// of the batch, just as a per-item error only counts against that item.
		defer s.recoverOperation("backup-all", nil, nil)
		defer s.batchActive.Store(false)

		self := s.selfContainerName(bctx)
		queue := make([]string, 0, len(names))
		for _, n := range names {
			if n != "" && n != self {
				queue = append(queue, n)
			}
		}
		// #119: order the batch by the user's explicit manual backup order first,
		// then most overdue first, as a scheduled run does. A store error here must
		// never abort the batch, so the selection order is the fallback.
		if ordered, err := s.store.OrderContainerNamesForRun(queue); err != nil {
			log.Printf("api: backup-all: order containers: %v (using selection order)", err)
		} else {
			queue = ordered
		}
		total := len(queue)
		const key = "batch:containers"
		s.publishBatch(key, 0, true)
		ok, fail, skipped := 0, 0, 0
		for i, n := range queue {
			if err := s.backupOneForBatch(bctx, n); err != nil {
				if errors.Is(err, backup.ErrContainerNotInstalled) {
					skipped++                                                                  // removed container: a skip (already recorded), not a batch failure (#57)
					log.Printf("api: backup-all: %q skipped: not installed (backups only)", n) //nolint:gosec // G706: n is %q-quoted
				} else {
					fail++
					log.Printf("api: backup-all: %q failed (continuing): %v", n, err) //nolint:gosec // G706: n is %q-quoted
				}
			} else {
				ok++
			}
			s.publishBatch(key, float64(i+1)/float64(total)*100, true)
		}
		s.publishBatch(key, 100, false)
		// Each compose stack's project directory, once for the whole batch rather
		// than once per member. Under bctx its retention forgets without --prune,
		// like every member's, so the one prune below reclaims the space for both.
		if err := s.BackupStacks(bctx, queue); err != nil {
			// Never fails the batch: the members are already safely backed up,
			// and a project folder that could not be read is its own problem to
			// report rather than a reason to call the whole round failed.
			log.Printf("api: backup-all: %v", err)
		}
		// Retention first: one local prune for the whole batch (each container's
		// forget ran without --prune under the bulk flag), before the batched
		// off-site replication, so fewer snapshots are left to copy.
		s.PruneAfterBulk(bctx, "containers")
		// #95: one batched off-site replication after the whole manual batch (no-op
		// unless containers replicate on a blank/coupled schedule with an off-site
		// repo). The per-container inline copy was suppressed via the bulk flag on bctx.
		s.ReplicateOffsiteAfterBulk(bctx, "containers")
		// One repo-size sample for the whole round, here rather than after every
		// container (see collectStatsAfterItem). Last, so it measures the repo the
		// round actually left behind: after the prune reclaimed space and after the
		// off-site copy, not somewhere in the middle of both.
		s.maybeCollectStats(bctx, "containers")
		log.Printf("api: backup-all done: %d ok, %d skipped, %d failed (of %d requested %d)", ok, skipped, fail, total, len(names))
	}()
	return true, nil
}

// backupOneForBatch backs up a single queued container on behalf of
// StartBackupAll, containing any panic to this item (recoverOperation) so
// one container's crash counts as that item failing, as a normal error
// does, instead of aborting every container queued behind it. On a
// recovered panic it also closes out the run StartRun'd inside
// backup.BackupContainer as failed (failStuckRun): that call is a plain
// sequential call, not deferred, so it would otherwise never be reached and
// the run would sit "running" forever.
func (s *Service) backupOneForBatch(ctx context.Context, name string) (err error) {
	// recoverOperation must be deferred directly, not wrapped in a
	// `defer func(){...}()` closure: recover() only works when called directly
	// by a deferred function, and &err is how it hands the panic back to this
	// function's named return.
	defer s.recoverOperation("backup-all: "+name, &err, func(msg string) {
		if tg, tErr := s.store.GetTargetByContainer(name); tErr == nil {
			s.failStuckRun(tg.ID, msg)
		}
	})
	_, err = s.Backup(ctx, name)
	return err
}

// StartBackup launches a single container backup in a background goroutine
// and returns immediately. Like StartBackupAll, it runs on the server, so
// it survives the browser that started it going away, including when
// backing up the reverse-proxy container BombVault's UI runs through
// severs the request connection mid-backup. The per-container
// "container:<name>" progress bar keeps reporting over SSE so the SPA can
// watch completion.
//
// It shares batchActive with StartBackupAll so a single backup and a batch can
// never overlap (the same repo lock would otherwise serialise them anyway).
// Returns (false, nil) if a backup/batch is already running (the caller answers
// busy), or (false, err) if the containers domain is already busy with another op.
func (s *Service) StartBackup(ctx context.Context, name string) (bool, error) {
	if !s.batchActive.CompareAndSwap(false, true) {
		return false, nil
	}
	if op, busy := s.domainBusy("containers"); busy {
		s.batchActive.Store(false)
		return false, fmt.Errorf("%s is running on containers", op)
	}
	// Detach so the run is independent of the request that started it (canceled
	// the moment the handler returns); Backup applies its own hard timeout.
	bctx := context.WithoutCancel(ctx)
	go func() {
		defer s.recoverOperation("backup: "+name, nil, func(msg string) {
			if tg, tErr := s.store.GetTargetByContainer(name); tErr == nil {
				s.failStuckRun(tg.ID, msg)
			}
		})
		defer s.batchActive.Store(false)
		if _, err := s.Backup(bctx, name); err != nil {
			if errors.Is(err, backup.ErrContainerNotInstalled) {
				log.Printf("api: backup: %q skipped: not installed (backups only)", name) //nolint:gosec // G706: name is %q-quoted
			} else {
				log.Printf("api: backup: %q failed: %v", name, err) //nolint:gosec // G706: name is %q-quoted
			}
		}
	}()
	return true, nil
}

// BackupInProgress reports whether a single backup, a batch, or a restore is
// running (they share the same single-flight guard), so callers and tests
// can observe when the detached goroutine has finished.
func (s *Service) BackupInProgress() bool { return s.batchActive.Load() }

// publishBatch emits an overall batch-progress event (no-op without a store).
func (s *Service) publishBatch(key string, percent float64, active bool) {
	if s.progress == nil {
		return
	}
	s.progress.Publish(progress.Event{Key: key, Phase: "backup", Percent: percent, Active: active})
}
