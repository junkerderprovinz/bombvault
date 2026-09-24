package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/backup"
	"github.com/junkerderprovinz/bombvault/internal/model"
	"github.com/junkerderprovinz/bombvault/internal/paths"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
	"github.com/junkerderprovinz/bombvault/internal/template"
)

// containerRestorePlan carries everything prepareRestore validated and resolved
// so the long-running execution can run detached from the request that asked
// for it (StartRestore) while the sync Restore path keeps identical behaviour.
type containerRestorePlan struct {
	repo         string
	mode         restic.Mode
	targetID     string
	snapshotID   string
	recreateOnly bool
	appdataPaths []string            // restored per-path back to origin, in snapshot-path form (nil = recreate-only)
	restoreDirs  []backup.RestoreDir // cross-pool remap: Subtree->Target; empty = in-place via appdataPaths
	// skippedPaths carries stored paths that had no mapping in the chosen
	// snapshot. They are skipped one by one (scrubbed log plus a note in the
	// orchestrator's run record), never a global abort.
	skippedPaths []string
	inspect      model.Inspect
	templateXML  string
}

// Restore runs a full container restore. The recreate profile is taken from the
// persisted definition (stored at backup time) so restore works even after the
// container has been deleted. For old targets without a stored definition the
// live inspect is used as a fallback; if that also fails a clear error is
// returned prompting the user to run one backup first.
func (s *Service) Restore(ctx context.Context, name, snapshotID string, confirm bool, source string, leaveStopped bool) error {
	plan, err := s.prepareRestore(ctx, name, snapshotID, confirm, source)
	if err != nil {
		return err
	}
	return s.executeRestore(ctx, name, plan, leaveStopped)
}

// repoRef identifies an already-resolved restic repository together with the
// mode (encryption + backend credentials) needed to open it. The
// settings-driven paths build one via repoFor/ModeFor; a caller restoring from
// a repo that is not in Settings (e.g. another BombVault instance's repo) can
// build its own ref and reuse the same preparation logic.
type repoRef struct {
	repo string
	mode restic.Mode
}

// prepareRestore resolves the settings-configured containers repo (local or
// off-site) and delegates to prepareRestoreIn. The request guards run
// first, before the settings and repo resolution, so an unconfirmed or
// malformed request fails with its own error (the sentinel, the name error,
// the snapshot-id error) rather than a resolution error; prepareRestoreIn
// re-validates them for non-settings callers.
func (s *Service) prepareRestore(ctx context.Context, name, snapshotID string, confirm bool, source string) (containerRestorePlan, error) {
	// Guard confirmation before touching the store/docker so an unconfirmed
	// restore surfaces the sentinel (and never errors on a missing target first).
	if !confirm {
		return containerRestorePlan{}, backup.ErrNotConfirmed
	}
	if !validResourceName(name) {
		return containerRestorePlan{}, errors.New("invalid container name")
	}
	if snapshotID != "latest" && snapshotID != "" && !backup.ValidSnapshotID(snapshotID) {
		return containerRestorePlan{}, backup.ErrInvalidSnapshotID
	}
	settings, err := s.store.GetSettings()
	if err != nil {
		return containerRestorePlan{}, fmt.Errorf("read settings: %w", err)
	}
	repo, err := s.containerRepoForName(settings, name, source)
	if err != nil {
		return containerRestorePlan{}, err
	}
	return s.prepareRestoreIn(ctx, repoRef{repo: repo, mode: s.repoModeFor(settings, "containers", source, repo)}, name, snapshotID, confirm)
}

// prepareRestoreIn performs all of a container restore's validation and
// resolution synchronously against an explicit repository (confirmation,
// name and snapshot-id guards, snapshot ownership, path containment and the
// recreate-recipe lookup), so a bad request fails immediately with a clear
// error before anything long-running or destructive starts. The returned
// plan is everything executeRestore needs.
func (s *Service) prepareRestoreIn(ctx context.Context, ref repoRef, name, snapshotID string, confirm bool) (containerRestorePlan, error) {
	if !confirm {
		return containerRestorePlan{}, backup.ErrNotConfirmed
	}
	// Re-validate the name at the service layer (defense-in-depth): the HTTP route
	// guards it via nameParam, but RestoreStack enumerates names from the store, so
	// the name-as-template-filename sink must be guarded here too, in case a
	// stored/imported name ever bypassed the boundary.
	if !validResourceName(name) {
		return containerRestorePlan{}, errors.New("invalid container name")
	}
	// An explicit snapshot id must be well-formed hex. The orchestrator re-checks
	// this, but guarding here makes a bad id fail synchronously (fail-fast for the
	// async StartRestore path). "latest"/"" resolve below.
	explicitID := snapshotID != "latest" && snapshotID != ""
	if explicitID && !backup.ValidSnapshotID(snapshotID) {
		return containerRestorePlan{}, backup.ErrInvalidSnapshotID
	}

	tg, err := s.store.GetTargetByContainer(name)
	if err != nil {
		log.Printf("api: restore: unknown target %q: %v", name, err) //nolint:gosec // G706: name is %q-quoted; no raw user bytes reach the log formatter
		return containerRestorePlan{}, errors.New("container has not been backed up yet")
	}
	// Same-instance restore: no destBase (in-place) and no overwrite prompt,
	// since the cross-pool remap is foreign-only. ref is this instance's own
	// repo, so the local alias history applies.
	return s.prepareRestoreForTarget(ctx, ref, name, snapshotID, tg, s.containerIdentity(name), "", false)
}

// prepareRestoreForTarget builds a container restore plan for an already
// resolved target tg against an explicit repo ref, without reading or writing
// the store. prepareRestoreIn passes the stored target; the foreign restore
// passes a target built from the decrypted foreign definition, so snapshot
// ownership and appdata containment are validated before that foreign recipe
// is persisted locally (prepareForeignRestore adopts it only once this returns
// a plan, never on a validation failure, which would clobber a same-named
// local target). The caller runs the confirm / name / explicit-snapshot-id-shape
// guards first.
//
// id is the identity the ownership check accepts. The caller decides it
// because it depends on where ref points: prepareRestoreIn passes the local
// containerIdentity, prepareForeignRestore only the literal container:<name>
// tag, since this instance's rename history says nothing about a foreign
// repository.
func (s *Service) prepareRestoreForTarget(ctx context.Context, ref repoRef, name, snapshotID string, tg store.Target, id entryIdentity, destBase string, overwrite bool) (containerRestorePlan, error) {
	explicitID := snapshotID != "latest" && snapshotID != ""

	// "latest" (or empty) resolves to the container's newest snapshot, as the
	// bulk "restore selected" action needs; restic returns snapshots oldest
	// first. A definition-only backup (stateless container with no restic
	// snapshot) is recreated from the stored definition instead. An explicit id
	// must be one id owns, the same access check the file and to-path restores
	// make, listed against the caller's repo ref rather than the settings repo.
	recreateOnly := false
	snaps, snapErr := s.snapshotsOwnedBy(ctx, ref.repo, ref.mode, id)
	if snapErr != nil {
		return containerRestorePlan{}, snapErr
	}
	if explicitID {
		if !snapshotBelongs(snaps, snapshotID) {
			return containerRestorePlan{}, notInListing{snapshotID, "container"}
		}
	} else {
		switch {
		case len(snaps) > 0:
			snapshotID = snaps[len(snaps)-1].ID
		case tg.Definition != "":
			recreateOnly = true
		default:
			return containerRestorePlan{}, errors.New("no backups found for this container")
		}
	}

	// Re-validate the stored appdata paths stay within the host mount root before
	// restoring (defense-in-depth in case the DB was tampered with). Skipped for a
	// recreate-only restore, which has no paths.
	var appdataForRestore []string
	var restoreDirs []backup.RestoreDir
	var bindRemap map[string]string
	var planSkipped []string
	if recreateOnly {
		appdataForRestore = nil
	} else {
		if len(tg.AppdataPaths) == 0 {
			return containerRestorePlan{}, errors.New("no backup paths recorded for this container: run a backup once, then restore")
		}
		for _, p := range tg.AppdataPaths {
			if !paths.Within(s.cfg.HostMountRoot, p) {
				log.Printf("api: restore: appdata path %q escapes mount root", p) //nolint:gosec // G706: %q-quoted
				return containerRestorePlan{}, errors.New("a stored backup path is outside the host mount, so refusing to restore")
			}
		}
		// Map the stored selection onto this snapshot's recorded Paths
		// (longest-prefix) before anything destructive. The stored list is the
		// selection as of the latest backup, and the chosen snapshot may be an
		// older one whose Paths have a different shape. Replaying the stored list
		// verbatim would fail the restore mid-loop at the adapter, after the caller
		// had already stopped and removed the container. All failure resolution
		// happens here, in the synchronous prepare phase: the orchestrator only
		// receives a list it has checked.
		chosen := chosenSnapshot(snaps, snapshotID)
		mapped, skipped, narrowed := mapRestorePaths(tg.AppdataPaths, chosen.Paths)
		// A pass-2 result is a stored path for which only an ancestor was
		// recorded. That proves it lies under a backed-up root, not that it is in
		// the snapshot: the branch may have been carved out by an --exclude when
		// the backup ran, or the folder may not have existed yet. Either way the
		// selector would miss, and the miss would land after the container was
		// stopped and removed, which is what this mapping exists to prevent. So
		// they are checked against the snapshot's real tree here, before anything
		// destructive, and a path not in it becomes an ordinary per-path skip,
		// never a widening back to the ancestor, which could lose data.
		if len(narrowed) > 0 {
			present, lsErr := s.pathsPresentInSnapshot(ctx, ref.repo, snapshotID, ref.mode, narrowed)
			if lsErr != nil {
				return containerRestorePlan{}, fmt.Errorf("read snapshot contents: %w", lsErr)
			}
			kept := mapped[:0]
			for _, q := range mapped {
				if present[q] {
					kept = append(kept, q)
					continue
				}
				skipped = append(skipped, q)
			}
			mapped = kept
		}
		if len(tg.AppdataPaths) > 0 && len(mapped) == 0 {
			return containerRestorePlan{}, errors.New("nothing to restore for this item from this snapshot")
		}
		if len(skipped) > 0 {
			// Per-path skip: an orphan stored path is never fatal. The reason is
			// scrubbed (paths become [path] first) and the skips reach the run record
			// via RestoreDeps.SkippedPaths below.
			log.Printf("api: restore: %d stored path(s) absent from snapshot %s, skipping: %s", len(skipped), snapshotID, scrubSecrets(strings.Join(skipped, ", "))) //nolint:gosec // G706: paths scrubbed to [path] before the formatter sees them
		}
		// The same containment check as the stored list above, over the mapped
		// selectors. Pass 1 takes them from the snapshot's recorded Paths (repo
		// metadata), a different and lower-trust source than the DB row the loop
		// above validated. Within cleans both sides, so a raw uncleaned metadata
		// string (or a crafted one with "..") is judged on its cleaned form. Pass 2
		// maps a stored path to itself and cannot produce a selector above the
		// stored list, so this guards pass 1 and crafted metadata.
		for _, q := range mapped {
			if !paths.Within(s.cfg.HostMountRoot, q) {
				log.Printf("api: restore: mapped path %q escapes mount root", q) //nolint:gosec // G706: %q-quoted
				return containerRestorePlan{}, errors.New("a mapped restore path is outside the host mount, so refusing to restore")
			}
		}
		appdataForRestore = mapped
		planSkipped = skipped
		// Cross-instance, cross-pool remap (destBase set: foreign restore,
		// #123/#125). A foreign recipe carries the source host's absolute appdata
		// paths; if this host lacks that pool, the in-place write would land in an
		// unmounted dir under the host mount (the array or RAM rootfs), writing
		// appdata to the wrong place or bricking the host (#122). Instead restore
		// every appdata path's contents into its container-relative place under
		// destBase on this host (containerAppdataRemap; a multi-bind container
		// keeps the folder its binds share), guard those destination dirs, and
		// point the recreated binds and template there. A standard container whose
		// appdata already lives under destBase remaps to the same path. destBase ==
		// "" is the same-instance in-place restore.
		if destBase != "" {
			if !paths.Within(s.cfg.HostMountRoot, destBase) {
				return containerRestorePlan{}, errors.New("restore destination is outside the host mount")
			}
			restoreDirs, bindRemap = s.containerAppdataRemap(destBase, appdataForRestore)
			destDirs := make([]string, len(restoreDirs))
			for i, d := range restoreDirs {
				destDirs[i] = d.Target
			}
			// Host-brick guard on the destination dirs (not the source), before
			// executeRestore's destructive Stop and Remove: prove each target is on a
			// real mounted pool so a cross-pool restore can never fill the RAM rootfs.
			if err := s.guardContainerRestoreDestination(ctx, ref, snapshotID, destDirs); err != nil {
				return containerRestorePlan{}, err
			}
			// Overwrite guard: a target that is not this container's own source path
			// and already holds data is likely a different container's appdata, so
			// refuse unless the caller explicitly confirmed the overwrite.
			if !overwrite {
				for _, d := range restoreDirs {
					if d.Target != d.Subtree && s.dirNonEmptyFn()(d.Target) {
						return containerRestorePlan{}, destinationRefusal("restore destination %q already contains data and may belong to a different container; confirm overwrite to proceed", s.toHostPath(d.Target))
					}
				}
			}
		}
	}

	// Resolve recreate recipe: prefer the stored definition (works for deleted
	// containers), fall back to live inspect (for old targets without a stored
	// definition), fail with a clear message if both are unavailable.
	var in model.Inspect
	var xml string
	if tg.Definition != "" {
		var def containerDefinition
		if jsonErr := json.Unmarshal([]byte(tg.Definition), &def); jsonErr != nil {
			return containerRestorePlan{}, fmt.Errorf("restore: unmarshal stored definition: %w", jsonErr)
		}
		in = def.Inspect
		xml = def.TemplateXML
	} else {
		// Fallback: a target with no stored definition; try live inspect.
		liveIn, liveErr := s.docker.Inspect(ctx, name)
		if liveErr != nil {
			return containerRestorePlan{}, errors.New("no stored definition for this container: run a backup once after upgrading, then restore is possible even after deletion")
		}
		in = liveIn
		xml, _, _ = template.Read(s.cfg.FlashTemplatesDir, name)
	}

	// Cross-pool remap: point the recreated container's binds and flashed
	// template at the appdata's new location. Only appdata binds are rewritten
	// (exact host-path match); docker.sock, /etc/localtime, /dev/dri and every
	// non-appdata bind stay verbatim, since they carry no backed-up data (#125).
	if len(bindRemap) > 0 {
		in.HostConfig.Binds = rewriteBinds(in.HostConfig.Binds, bindRemap)
		in.Mounts = rewriteMountSources(in.Mounts, bindRemap)
		xml = template.RewriteHostPaths(xml, bindRemap)
	}

	return containerRestorePlan{
		repo:         ref.repo,
		mode:         ref.mode,
		targetID:     tg.ID,
		snapshotID:   snapshotID,
		recreateOnly: recreateOnly,
		appdataPaths: appdataForRestore,
		restoreDirs:  restoreDirs,
		skippedPaths: planSkipped,
		inspect:      in,
		templateXML:  xml,
	}, nil
}

// executeRestore drives the long-running (destructive) part of a container
// restore described by an already-validated plan, publishing "container:<name>"
// progress. The orchestrator records the run (kindRestore) itself.
func (s *Service) executeRestore(ctx context.Context, name string, plan containerRestorePlan, leaveStopped bool) error {
	// Hold the domain repo lock for the whole restic and docker phase,
	// including the destination pre-create below. The scheduler calls
	// Backup/BackupVM directly and bypasses the batchActive single-flight
	// guard, so the domain lock is the one layer scheduled jobs respect;
	// without it a detached multi-hour restore could overlap a scheduled backup
	// of the same domain in both directions.
	unlock := s.lockDomainFor("containers", "restore")
	defer unlock()
	// Pre-create every remapped destination readable (0o755), as the file-set
	// and to-path restores do (see EnsureDirReadable). restic's restorer
	// creates a fresh subtree target at 0o700 and never revisits that root's
	// own metadata (only its restored contents get the snapshot's ownership and
	// mode), which leaves the cross-pool remap's new destination dirs
	// root:root/0700, unreadable to whatever UID the container runs as.
	// Pre-created, restic's MkdirAll finds the dir present and leaves the mode
	// alone. In-place restores (empty RestoreDirs, or a Target that already
	// existed) are a cheap no-op: EnsureDirReadable only heals mode, never
	// ownership; the numeric owner:group is restored after a successful restore
	// by healRestoreDirOwnership (#125).
	for _, rd := range plan.restoreDirs {
		if err := paths.EnsureDirReadable(rd.Target); err != nil {
			return fmt.Errorf("restore: prepare destination %q: %w", s.toHostPath(rd.Target), err)
		}
	}
	rkey := "container:" + name
	rctx, startedAt := s.progBegin(ctx, rkey, "restore")
	rerr := backup.RestoreContainer(rctx, backup.RestoreDeps{
		Confirmed:         true, // prepareRestore rejected unconfirmed requests
		RecreateOnly:      plan.recreateOnly,
		ContainerRef:      name,
		ContainerName:     name,
		RepoPath:          plan.repo,
		SnapshotID:        plan.snapshotID,
		AppdataPaths:      plan.appdataPaths, // restored per-path back to origin, snapshot-path form (nil = recreate-only)
		RestoreDirs:       plan.restoreDirs,  // cross-pool remap (foreign restore); empty = in-place
		SkippedPaths:      plan.skippedPaths, // unmapped stored paths → run-record note, never an abort
		TemplateXML:       plan.templateXML,
		FlashTemplatesDir: s.cfg.FlashTemplatesDir,
		Inspect:           plan.inspect,
		LeaveStopped:      leaveStopped,
		TargetID:          plan.targetID,
		Docker:            s.docker,
		Restic:            &resticAdapter{engine: s.engine, mode: plan.mode},
		Templates:         templatesAdapter{},
		Runs:              runsAdapter{st: s.store, ctx: ctx},
	})
	if rerr == nil {
		s.healRestoreDirOwnership(rctx, plan.repo, plan.snapshotID, plan.mode, plan.restoreDirs)
	}
	s.progEnd(rkey, "restore", rerr == nil, startedAt)
	return rerr
}

// healRestoreDirOwnership restores, best-effort, each remapped restore
// directory's own root to the owner and mode it had in the snapshot.
// restic's restorer creates a fresh subtree target itself but never
// re-applies that root's own metadata (only its restored contents get the
// snapshot's ownership and mode; the EnsureDirReadable call above fixes
// readability only). Read-then-heal, never fatal: the restore already
// succeeded when this runs, so a failure here (repo busy, a stale snapshot
// layout, an unexpected filesystem) must not turn a completed restore into
// a reported failure; it is logged and the next dir is tried.
func (s *Service) healRestoreDirOwnership(ctx context.Context, repo, snapshotID string, mode restic.Mode, dirs []backup.RestoreDir) {
	for _, rd := range dirs {
		entries, err := s.engine.LsPath(ctx, repo, snapshotID, rd.Subtree, mode)
		if err != nil {
			log.Printf("api: restore: reading original owner for %q: %v", s.toHostPath(rd.Target), err)
			continue
		}
		var found *restic.FileEntry
		for i := range entries {
			if entries[i].Path == rd.Subtree {
				found = &entries[i]
				break
			}
		}
		if found == nil {
			log.Printf("api: restore: no snapshot entry for %q, leaving owner as restored", s.toHostPath(rd.Target))
			continue
		}
		// Owner before mode: on some systems chown() clears setuid/setgid as a
		// security measure, so applying it first means a later chmod restores
		// the snapshot's real bits rather than having them silently stripped.
		if err := os.Lchown(rd.Target, found.Uid, found.Gid); err != nil {
			log.Printf("api: restore: restoring owner on %q: %v", s.toHostPath(rd.Target), err)
			continue
		}
		if err := os.Chmod(rd.Target, os.FileMode(found.Mode).Perm()); err != nil {
			log.Printf("api: restore: restoring mode on %q: %v", s.toHostPath(rd.Target), err)
		}
	}
}

// restoreTimeout is the hard cap on every detached restore goroutine
// (StartRestore, StartRestoreVM, StartRestoreFiles, StartRestoreToPath,
// StartRestoreStack). Aborting a restore mid-flight is destructive, because
// the container has already been removed and the appdata is partly
// written, so unlike the configurable backup cap (backupHardCap, default
// 48h) this one is generous: it exists only so a truly wedged restic can't
// hold the single-flight guard and the domain lock forever, never to bound
// a legitimate huge restore.
const restoreTimeout = 48 * time.Hour

// registerCancel records the CancelFunc of a running restore under its progress
// key so POST /api/restore/cancel can stop it. Called on launch; paired with a
// deferred unregisterCancel.
func (s *Service) registerCancel(key string, cancel context.CancelFunc) {
	s.cancelMu.Lock()
	if s.runCancels == nil {
		s.runCancels = map[string]context.CancelFunc{}
	}
	s.runCancels[key] = cancel
	s.cancelMu.Unlock()
}

// unregisterCancel drops a restore's cancel entry once it has finished, so a
// later cancel of the same key is a harmless no-op.
func (s *Service) unregisterCancel(key string) {
	s.cancelMu.Lock()
	delete(s.runCancels, key)
	s.cancelMu.Unlock()
}

// CancelRun cancels a running restore by its progress key and reports whether one
// was registered. Cancelling an unknown/already-finished key returns false and is
// a no-op (idempotent), so the endpoint can be called safely at any time.
func (s *Service) CancelRun(key string) bool {
	s.cancelMu.Lock()
	cancel, ok := s.runCancels[key]
	s.cancelMu.Unlock()
	if ok {
		cancel()
	}
	return ok
}

// StartRestore launches an in-place container restore in a background
// goroutine and returns immediately, mirroring StartBackup. Long restores
// need this: the work runs on the server, detached from the request, so a
// multi-hour restore can't be killed by the browser or a proxy dropping the
// idle HTTP connection, which would cancel the request context and abort
// restic mid-restore. All validation runs synchronously first, so a bad
// request still fails immediately with a clear error and no goroutine is
// started.
//
// It shares batchActive with the backup starters so a restore can never run
// concurrently with a backup or another restore (they contend on repo locks and
// container stop/start). Returns (false, nil) when one is already running.
func (s *Service) StartRestore(ctx context.Context, name, snapshotID, source string, leaveStopped bool) (bool, error) {
	if !s.batchActive.CompareAndSwap(false, true) {
		return false, nil
	}
	plan, err := s.prepareRestore(ctx, name, snapshotID, true, source)
	if err != nil {
		s.batchActive.Store(false)
		return false, err
	}
	// Detach so the run is independent of the request that started it (canceled
	// the moment the handler returns), capped by restoreTimeout (see its comment
	// for why the restore cap is far more generous than the backup one).
	bctx := context.WithoutCancel(ctx)
	key := "container:" + name // the exact progBegin key executeRestore publishes under
	go func() {
		defer s.recoverOperation("restore: "+name, nil, func(msg string) {
			s.failStuckRun(plan.targetID, msg)
		})
		defer s.batchActive.Store(false)
		tctx, tcancel := context.WithTimeout(bctx, restoreTimeout)
		defer tcancel()
		rctx, cancel := context.WithCancel(tctx)
		defer cancel()
		s.registerCancel(key, cancel)
		defer s.unregisterCancel(key)
		if rerr := s.executeRestore(rctx, name, plan, leaveStopped); rerr != nil {
			log.Printf("api: restore: %q failed: %v", name, rerr) //nolint:gosec // G706: name is %q-quoted
		}
	}()
	return true, nil
}

// ListSnapshotFiles lists the files in a container snapshot, for file-level
// restore. snapshotID must be valid hex.
func (s *Service) ListSnapshotFiles(ctx context.Context, name, snapshotID, source string) ([]restic.FileEntry, error) {
	if !backup.ValidSnapshotID(snapshotID) {
		return nil, backup.ErrInvalidSnapshotID
	}
	// Scope to the named container: the snapshot must be one of its snapshots,
	// so one container's file tree can't be listed through another's route.
	snaps, err := s.Snapshots(ctx, name, source)
	if err != nil {
		return nil, err
	}
	found := false
	for _, sn := range snaps {
		if sn.ID == snapshotID || strings.HasPrefix(sn.ID, snapshotID) {
			found = true
			break
		}
	}
	if !found {
		return nil, notInListing{snapshotID, "container"}
	}
	settings, err := s.store.GetSettings()
	if err != nil {
		return nil, fmt.Errorf("read settings: %w", err)
	}
	repo, err := s.containerRepoForName(settings, name, source)
	if err != nil {
		return nil, err
	}
	return s.lsSelfHeal(ctx, repo, snapshotID, s.repoModeFor(settings, "containers", source, repo))
}

// RestoreContainerFiles restores one or more files or dirs from a container
// snapshot. With targetSubPath empty the selected paths are written back to
// their original locations (in place, restic target "/"); with a non-empty
// targetSubPath the selection is extracted into an alternate folder under
// the host mount (non-destructive, same containment as
// RestoreContainerToPath). It returns the resolved absolute target folder
// for the alternate-folder case, or "" for an in-place restore.
//
// Security: confirm-gated; the snapshot id passes the strict hex guard
// (backup.ValidSnapshotID) and must belong to the named container
// (tag-scoped via Snapshots, like RestoreContainerToPath) so one
// container's data can't be extracted through another's route; every
// selected path is path.Cleaned and must sit within the host mount
// (paths.Within), so a restore can never read or write outside the backup
// mount; and the alternate target is resolved with paths.Resolve and
// created (EnsureDir) only after containment passes.
func (s *Service) RestoreContainerFiles(ctx context.Context, name, source, snapshotID string, filePaths []string, targetSubPath string, confirm bool) (string, error) {
	plan, err := s.prepareRestoreFiles(ctx, name, source, snapshotID, filePaths, targetSubPath, confirm)
	if err != nil {
		return "", err
	}
	if err := s.runRestoreFiles(ctx, plan); err != nil {
		return "", err
	}
	return plan.resolved, nil
}

// filesRestorePlan carries everything prepareRestoreFiles validated and
// resolved so the restic loop can run detached from the request that asked for
// it (StartRestoreFiles) while the sync path keeps identical behaviour.
type filesRestorePlan struct {
	repo       string
	mode       restic.Mode
	snapshotID string
	paths      []string // cleaned selection, containment-validated for in-place
	target     string   // restic --target: "/" = in place, else the resolved folder
	resolved   string   // the resolved alternate folder ("" = in-place)
}

// prepareRestoreFiles performs all of a file-level restore's validation and
// resolution synchronously (see the security notes on
// RestoreContainerFiles), so a bad request fails immediately with a clear
// error, and creates the alternate target folder once containment passes.
func (s *Service) prepareRestoreFiles(ctx context.Context, name, source, snapshotID string, filePaths []string, targetSubPath string, confirm bool) (filesRestorePlan, error) {
	if !confirm {
		return filesRestorePlan{}, backup.ErrNotConfirmed
	}
	if !validResourceName(name) {
		return filesRestorePlan{}, errors.New("invalid container name")
	}
	if source != "local" && !isOffsiteSource(source) {
		return filesRestorePlan{}, errors.New("invalid source (must be local or offsite)")
	}
	if !backup.ValidSnapshotID(snapshotID) {
		return filesRestorePlan{}, backup.ErrInvalidSnapshotID
	}
	if len(filePaths) == 0 {
		return filesRestorePlan{}, errors.New("no files selected")
	}

	// Clean each selected path once, so the validated path is the path that runs.
	cleaned := make([]string, 0, len(filePaths))
	for _, p := range filePaths {
		cleaned = append(cleaned, path.Clean(p))
	}

	// Scope to the named container: the snapshot must be one of its snapshots
	// (the same access check as RestoreContainerToPath).
	snaps, err := s.Snapshots(ctx, name, source)
	if err != nil {
		return filesRestorePlan{}, err
	}
	if !snapshotBelongs(snaps, snapshotID) {
		return filesRestorePlan{}, notInListing{snapshotID, "container"}
	}

	// Resolve the destination. Empty targetSubPath → in-place (restic target "/",
	// which writes each included path back to its absolute location). Otherwise
	// resolve the alternate folder under the host mount (shared containment helper)
	// and create it only after containment passes.
	target := "/"
	resolved := ""
	if sub := strings.TrimSpace(targetSubPath); sub != "" {
		t, err := paths.Resolve(s.cfg.HostMountRoot, sub)
		if err != nil {
			return filesRestorePlan{}, errors.New("invalid target folder: must be a relative subpath under the host mount")
		}
		if err := paths.EnsureDir(t); err != nil {
			return filesRestorePlan{}, fmt.Errorf("create target folder: %w", err)
		}
		target = t
		resolved = t
	} else {
		// In place writes each path back to its absolute location, so every path
		// must sit within the host mount (defense-in-depth). Validate all up front
		// so one bad entry fails the whole batch before anything is written. For an
		// alternate folder this is unnecessary: restic writes under --target, which
		// paths.Resolve already contained above.
		for _, c := range cleaned {
			if !paths.Within(s.cfg.HostMountRoot, c) {
				return filesRestorePlan{}, errors.New("restore file: path is outside the backup mount")
			}
		}
	}

	settings, err := s.store.GetSettings()
	if err != nil {
		return filesRestorePlan{}, fmt.Errorf("read settings: %w", err)
	}
	repo, err := s.containerRepoForName(settings, name, source)
	if err != nil {
		return filesRestorePlan{}, err
	}
	return filesRestorePlan{
		repo:       repo,
		mode:       s.repoModeFor(settings, "containers", source, repo),
		snapshotID: snapshotID,
		paths:      cleaned,
		target:     target,
		resolved:   resolved,
	}, nil
}

// runRestoreFiles restores each selected path of an already-validated plan.
// It is not atomic, since restic writes per path, so if one fails mid-batch
// the error says how many already went through and which path stopped it,
// instead of a bare failure that hides the paths already restored.
func (s *Service) runRestoreFiles(ctx context.Context, plan filesRestorePlan) error {
	// Hold the domain repo lock for the restic work: scheduled backups bypass
	// batchActive and the domain lock is the layer they respect (see
	// executeRestore).
	unlock := s.lockDomainFor("containers", "restore")
	defer unlock()
	for i, c := range plan.paths {
		// escapeGlobLiteral, because --include is a glob like --exclude and c is a
		// path the user ticked in the file picker, not a pattern anyone wrote. A
		// raw "/Inception (2010) [1080p]" restores zero files and restic still
		// exits 0, so the run would be recorded a success with an empty target
		// folder.
		if err := s.engine.RestoreInclude(ctx, plan.repo, plan.snapshotID, escapeGlobLiteral(c), plan.target, plan.mode); err != nil {
			if len(plan.paths) > 1 {
				return fmt.Errorf("restored %d of %d files, then failed on %q: %w", i, len(plan.paths), c, err)
			}
			return err
		}
	}
	return nil
}

// StartRestoreFiles launches a file-level restore in a background goroutine
// and returns immediately (see StartRestore for why). All validation runs
// synchronously (a bad request fails right away, no goroutine); the
// resolved alternate target folder ("" for in-place) is returned in the ack
// so the UI can show it. The detached run publishes "container:<name>"
// progress (phase "restore") and records a run (kind "restore"), so the
// outcome, including the real restic error text, lands in the run history.
//
// Shares batchActive with backups and the other restores; returns
// ("", false, nil) when one is already running.
func (s *Service) StartRestoreFiles(ctx context.Context, name, source, snapshotID string, filePaths []string, targetSubPath string, confirm bool) (string, bool, error) {
	if !s.batchActive.CompareAndSwap(false, true) {
		return "", false, nil
	}
	plan, err := s.prepareRestoreFiles(ctx, name, source, snapshotID, filePaths, targetSubPath, confirm)
	if err != nil {
		s.batchActive.Store(false)
		return "", false, err
	}
	bctx := context.WithoutCancel(ctx)
	rkey := "container:" + name // the exact progBegin key this restore publishes under
	go func() {
		// runID is declared before recoverOperation is deferred, so the onPanic
		// closure sees whatever value it holds at panic time. A panic before the
		// assignment means beginRestoreRun never ran and there is no run to close.
		var runID string
		defer s.recoverOperation("restore files: "+name, nil, func(msg string) {
			s.finishRestoreRun(runID, "", errors.New(msg))
		})
		defer s.batchActive.Store(false)
		tctx, tcancel := context.WithTimeout(bctx, restoreTimeout)
		defer tcancel()
		rctx, cancel := context.WithCancel(tctx)
		defer cancel()
		s.registerCancel(rkey, cancel)
		defer s.unregisterCancel(rkey)
		runID = s.beginRestoreRun(name)
		pctx, startedAt := s.progBegin(rctx, rkey, "restore")
		rerr := s.runRestoreFiles(pctx, plan)
		s.progEnd(rkey, "restore", rerr == nil, startedAt)
		s.finishRestoreRun(runID, plan.snapshotID, rerr)
		if rerr != nil {
			log.Printf("api: restore files: %q failed: %v", name, rerr) //nolint:gosec // G706: name is %q-quoted
		}
	}()
	return plan.resolved, true, nil
}

// beginRestoreRun records, best-effort, the start of a service-layer
// restore run (kind "restore") against the container's target row, so the
// outcome shows up in the run history like the orchestrated in-place
// restore does. It returns "" when recording is impossible (no target row,
// store error); bookkeeping must never block the restore itself.
func (s *Service) beginRestoreRun(name string) string {
	tg, err := s.store.GetTargetByContainer(name)
	if err != nil {
		log.Printf("api: restore: no target row for %q, so the outcome won't appear in the run history: %v", name, err) //nolint:gosec // G706: name is %q-quoted
		return ""
	}
	return s.beginRestoreRunForTarget(tg.ID)
}

// beginRestoreRunForTarget is beginRestoreRun for an already resolved
// runs.target_id (a container target's ID, or a file set's stable id,
// since the files domain records its restores against file_sets.id
// directly), so every per-item domain shares one restore-bookkeeping path.
// It returns "" when recording fails; bookkeeping must never block the
// restore itself.
func (s *Service) beginRestoreRunForTarget(targetID string) string {
	// A restore run is never part of a "Backup Everything" pass's grouped
	// children (see runGroupKey), so context.Background() loses nothing.
	runID, err := runsAdapter{st: s.store, ctx: context.Background()}.Start(targetID, "restore")
	if err != nil {
		log.Printf("api: restore: record run start for target %q failed: %v", targetID, err) //nolint:gosec // G706: targetID is %q-quoted
		return ""
	}
	return runID
}

// finishRestoreRun closes a run opened by beginRestoreRun with the terminal
// status + the (truncated) error text. A "" runID (recording was skipped) is a
// no-op; a finish failure is logged, never surfaced (best-effort bookkeeping).
func (s *Service) finishRestoreRun(runID, snapshotID string, rerr error) {
	if runID == "" {
		return
	}
	// As in beginRestoreRunForTarget, a restore run is never part of a grouped
	// pass, so context.Background() loses nothing in any branch below.
	var err error
	switch {
	case rerr == nil:
		err = runsAdapter{st: s.store, ctx: context.Background()}.Finish(runID, "success", snapshotID, 0, "")
	case errors.Is(rerr, context.Canceled):
		// A user cancel is an intentional, recorded outcome, not a failure: record
		// it as "cancelled" and fire no failure alert (the terminal progEnd already
		// fired to clear the bar).
		err = runsAdapter{st: s.store, ctx: context.Background()}.Finish(runID, "cancelled", "", 0, "cancelled by user")
	default:
		err = runsAdapter{st: s.store, ctx: context.Background()}.Finish(runID, "failed", "", 0, truncateRunErr(rerr))
	}
	if err != nil {
		log.Printf("api: restore: record run finish failed: %v", err)
	}
}

// finishRestoreRunWarn closes a restore run as success but records warn in
// the run's error column, for when restic extracted all data yet could not
// set ownership or metadata on the target (see
// restic.ErrRestoreMetadataOnly). The run counts as a success everywhere
// (health, retention gates); the message tells the operator the original
// ownership could not be reproduced on the share.
func (s *Service) finishRestoreRunWarn(runID, snapshotID, warn string) {
	if runID == "" {
		return
	}
	// As in beginRestoreRunForTarget, a restore run is never part of a grouped
	// pass, so context.Background() loses nothing.
	err := runsAdapter{st: s.store, ctx: context.Background()}.Finish(runID, "success", snapshotID, 0, warn)
	if err != nil {
		log.Printf("api: restore: record run finish (warning) failed: %v", err)
	}
}

// RestoreContainerToPath extracts a whole container snapshot into an
// alternate folder under the host mount; it is non-destructive, and the
// live container is never touched. Unlike Restore, it stops, removes and
// recreates nothing: it is for inspecting, cloning or migrating a
// snapshot's data. It returns the resolved absolute target path
// (container-visible, under the host mount root); the handler scrubs it for
// the UI.
//
// Security: the snapshot id passes the strict hex guard
// (backup.ValidSnapshotID, as for the file and in-place restores), the
// snapshot must belong to the named container (tag-scoped via Snapshots,
// like ListSnapshotFiles), and the target is resolved with
// paths.Resolve(HostMountRoot, targetSubPath), the containment helper
// SetBackupPaths and handleBrowse use, which path.Cleans and rejects
// absolute and `..` escapes. The directory is created (MkdirAll) only
// after containment passes.
func (s *Service) RestoreContainerToPath(ctx context.Context, name, source, snapshotID, targetSubPath string) (string, error) {
	plan, err := s.prepareRestoreToPath(ctx, name, source, snapshotID, targetSubPath)
	if err != nil {
		return "", err
	}
	if err := s.runRestoreToPath(ctx, plan); err != nil {
		return "", err
	}
	return plan.target, nil
}

// toPathRestorePlan carries everything prepareRestoreToPath validated and
// resolved so the restic extraction can run detached from the request that
// asked for it (StartRestoreToPath) while the sync path keeps identical
// behaviour.
type toPathRestorePlan struct {
	repo       string
	mode       restic.Mode
	snapshotID string
	target     string // resolved absolute target folder (under the host mount)
}

// prepareRestoreToPath performs all of a to-folder restore's validation and
// resolution synchronously (see the security notes on
// RestoreContainerToPath), so a bad request fails immediately with a clear
// error, and creates the target folder once containment passes.
func (s *Service) prepareRestoreToPath(ctx context.Context, name, source, snapshotID, targetSubPath string) (toPathRestorePlan, error) {
	if !validResourceName(name) {
		return toPathRestorePlan{}, errors.New("invalid container name")
	}
	if source != "local" && !isOffsiteSource(source) {
		return toPathRestorePlan{}, errors.New("invalid source (must be local or offsite)")
	}
	if !backup.ValidSnapshotID(snapshotID) {
		return toPathRestorePlan{}, backup.ErrInvalidSnapshotID
	}

	// Resolve the target against the host mount root with the shared containment
	// helper: it path.Cleans the input and rejects an absolute path or any "../"
	// that would escape the mount. The result is guaranteed to sit under the mount.
	target, err := paths.Resolve(s.cfg.HostMountRoot, targetSubPath)
	if err != nil {
		// paths.Resolve returns ErrTraversal or ErrAbsoluteSub, neither of which
		// leaks a host path; keep the message generic, as handleBrowse does.
		return toPathRestorePlan{}, errors.New("invalid target folder: must be a relative subpath under the host mount")
	}

	// Scope to the named container: the snapshot must be one of its snapshots,
	// so one container's data can't be extracted through another's route (the
	// same access check as ListSnapshotFiles).
	snaps, err := s.Snapshots(ctx, name, source)
	if err != nil {
		return toPathRestorePlan{}, err
	}
	if !snapshotBelongs(snaps, snapshotID) {
		return toPathRestorePlan{}, notInListing{snapshotID, "container"}
	}

	settings, err := s.store.GetSettings()
	if err != nil {
		return toPathRestorePlan{}, fmt.Errorf("read settings: %w", err)
	}
	repo, err := s.containerRepoForName(settings, name, source)
	if err != nil {
		return toPathRestorePlan{}, err
	}

	// Create the target dir only after containment passed.
	if err := paths.EnsureDir(target); err != nil {
		return toPathRestorePlan{}, fmt.Errorf("create target folder: %w", err)
	}
	return toPathRestorePlan{
		repo:       repo,
		mode:       s.repoModeFor(settings, "containers", source, repo),
		snapshotID: snapshotID,
		target:     target,
	}, nil
}

// runRestoreToPath restores the whole snapshot tree of an already-validated
// plan into the target dir: restic restore --target <dir> --include /
// (everything). It reuses the restore-to-target engine method; "/"
// includes all paths in the snapshot.
func (s *Service) runRestoreToPath(ctx context.Context, plan toPathRestorePlan) error {
	// Hold the domain repo lock for the restic work: scheduled backups bypass
	// batchActive and the domain lock is the layer they respect (see
	// executeRestore).
	unlock := s.lockDomainFor("containers", "restore")
	defer unlock()
	return s.engine.RestoreInclude(ctx, plan.repo, plan.snapshotID, "/", plan.target, plan.mode)
}

// StartRestoreToPath launches a whole-snapshot extraction into an
// alternate folder in a background goroutine and returns immediately (see
// StartRestore for why; multi-hour extractions are the usual case, #24).
// All validation runs synchronously (a bad request fails right away, no
// goroutine); the resolved target folder is returned in the ack so the UI
// can show it. The detached run publishes "container:<name>" progress
// (phase "restore") and records a run (kind "restore"), so the outcome,
// including the real restic error text, lands in the run history.
//
// Shares batchActive with backups and the other restores; returns
// ("", false, nil) when one is already running.
func (s *Service) StartRestoreToPath(ctx context.Context, name, source, snapshotID, targetSubPath string) (string, bool, error) {
	if !s.batchActive.CompareAndSwap(false, true) {
		return "", false, nil
	}
	plan, err := s.prepareRestoreToPath(ctx, name, source, snapshotID, targetSubPath)
	if err != nil {
		s.batchActive.Store(false)
		return "", false, err
	}
	bctx := context.WithoutCancel(ctx)
	rkey := "container:" + name // the exact progBegin key this restore publishes under
	go func() {
		var runID string // see StartRestoreFiles's identical goroutine for why this is declared here
		defer s.recoverOperation("restore to path: "+name, nil, func(msg string) {
			s.finishRestoreRun(runID, "", errors.New(msg))
		})
		defer s.batchActive.Store(false)
		tctx, tcancel := context.WithTimeout(bctx, restoreTimeout)
		defer tcancel()
		rctx, cancel := context.WithCancel(tctx)
		defer cancel()
		s.registerCancel(rkey, cancel)
		defer s.unregisterCancel(rkey)
		runID = s.beginRestoreRun(name)
		pctx, startedAt := s.progBegin(rctx, rkey, "restore")
		rerr := s.runRestoreToPath(pctx, plan)
		s.progEnd(rkey, "restore", rerr == nil, startedAt)
		s.finishRestoreRun(runID, plan.snapshotID, rerr)
		if rerr != nil {
			log.Printf("api: restore to folder: %q failed: %v", name, rerr) //nolint:gosec // G706: name is %q-quoted
		}
	}()
	return plan.target, true, nil
}

// snapshotRestoreRoot returns the single tree node that covers every path
// the snapshot in snaps (matched by id, exact or unambiguous prefix, like
// snapshotBelongs) recorded: their deepest common ancestor. It is the
// subtree a to-folder restore extracts (<id>:<subtree>), read from the
// snapshot so it stays valid even when HostMountRoot changed since the
// backup (a recompute from the set's path would then miss). "" means there
// is no match, the snapshot recorded no path, or the paths share no
// ancestor below "/"; all three fall back to a whole-tree restore at the
// call site.
//
// A multi-root snapshot is the normal shape, since the file-set backup
// compiles one positional per selected root (fileSetPositionals ->
// FileSetBackupDeps.SourcePaths), so returning only the first recorded
// root would drop the others and report success with data missing.
// Restoring the common ancestor widens nothing: a snapshot tree contains
// only what was backed up, so the ancestor node holds exactly the recorded
// roots. With restic 0.17, a snapshot recording docs/keep-a and docs/keep-b
// restored as <id>:docs yields keep-a and keep-b and not the
// never-backed-up sibling; TestRestoreCommonAncestorOfRecordedRoots in
// internal/restic pins it.
func snapshotRestoreRoot(snaps []restic.Snapshot, id string) string {
	for _, sn := range snaps {
		if sn.ID == id || strings.HasPrefix(sn.ID, id) {
			return commonAncestor(sn.Paths)
		}
	}
	return ""
}

// commonAncestor returns the deepest path that is at or above every one of
// paths, or "" when there is none below the filesystem root (an empty
// list, or roots on different top-level branches). Segment-aligned, so /a
// never counts as an ancestor of /ab, the same rule as isStrictDescendant
// and internal/paths.Resolve.
// It trims the first path down rather than rebuilding one from segments: a
// recorded path is a selector that has to reach restic unchanged, and a
// rebuild would normalise whatever the original looked like (snapshots
// taken on Windows carry a drive letter and mixed separators, and a
// leading "/" would make every containment check miss).
func commonAncestor(paths []string) string {
	if len(paths) == 0 {
		return ""
	}
	if len(paths) == 1 {
		// A single root is handed back exactly as recorded, byte for byte.
		return paths[0]
	}
	covers := func(a string) bool {
		for _, p := range paths {
			if q := path.Clean(p); q != a && !isStrictDescendant(q, a) {
				return false
			}
		}
		return true
	}
	cand := path.Clean(paths[0])
	for {
		if covers(cand) {
			return cand
		}
		parent := path.Dir(cand)
		// Reaching the filesystem root means the roots live on different
		// branches: there is no single node to extract, so the caller falls back
		// to a whole-tree restore rather than emitting a "<id>:/" selector.
		if parent == cand || parent == "." || parent == "/" {
			return ""
		}
		cand = parent
	}
}

// guardContainerRestoreDestination is the container counterpart of
// guardVMRestoreDestination, run only for a cross-instance (foreign)
// container restore (#123, the #122 class for containers). A foreign
// recipe carries the source host's absolute appdata paths; if the
// destination host lacks that pool (the source used /mnt/zfs but this box
// has no zfs share), restic would write appdata into an unmounted dir
// under the host mount, the array or RAM rootfs, landing the data in the
// wrong place or bricking the host on a large restore. This proves every
// appdata target sits on a mounted pool or share before the caller
// reaches executeRestore's destructive Stop and Remove, turning that
// silent wrong write into a clear refusal. The standard /mnt/user/appdata
// case always passes (shfs is a live mount below the host bind), so a
// normal cross-Unraid restore is unaffected.
func (s *Service) guardContainerRestoreDestination(ctx context.Context, ref repoRef, snapshotID string, appdataPaths []string) error {
	for _, p := range appdataPaths {
		if !s.destinationMounted(p) {
			return destinationRefusal("appdata destination %q is not on a mounted pool or share on this system: the source backed it up from a pool this host does not have, so restoring would write it to the wrong place. Create or mount that share here, then retry", s.toHostPath(p))
		}
	}
	// Free-space preflight only when the whole restore lands in a single
	// appdata path: several paths may sit on different pools, and the
	// snapshot's total restore-size can't be attributed per pool, which would
	// falsely refuse a legitimate split restore. A probe error is "cannot
	// prove insufficient" and never blocks; only a proven shortfall aborts,
	// as in guardVMRestoreDestination.
	if len(appdataPaths) == 1 {
		if _, wantBytes, err := s.engine.StatsRestoreSize(ctx, ref.repo, snapshotID, ref.mode); err == nil && wantBytes > 0 {
			if free, ferr := s.diskFreeFn()(nearestExistingDir(appdataPaths[0])); ferr == nil && free < uint64(wantBytes) {
				return destinationRefusal("not enough free space to restore this container's appdata: it needs %d bytes but %q has only %d free. Free up space and retry", wantBytes, s.toHostPath(appdataPaths[0]), free)
			}
		}
	}
	return nil
}

// appdataRelPath splits a backed-up container path into the appdata root
// it lives in and the container-relative remainder below it, e.g.
// /host/user/user/appdata/SnapOtter/conf → ("/host/user/user/appdata",
// "SnapOtter/conf") and /host/user/zfs/appdata/nexterm →
// ("/host/user/zfs/appdata", "nexterm"). The split is on the first
// "appdata" segment, the share or pool root every recorded path is
// selected by (resolveAppdataPaths keeps a bind only when
// hasSegment(src,"appdata")); taking the first occurrence keeps a
// container that owns a nested folder called "appdata"
// (…/appdata/foo/appdata) anchored at the real root.
//
// rel is "" when the path has no "appdata" segment or is an appdata root
// (a container binding /mnt/user/appdata itself); callers then fall back
// to the plain basename.
func appdataRelPath(src string) (root, rel string) {
	segs := strings.Split(src, "/")
	for i, seg := range segs {
		if seg == "appdata" && i+1 < len(segs) {
			return strings.Join(segs[:i+1], "/"), strings.Join(segs[i+1:], "/")
		}
	}
	return "", ""
}

// containerAppdataRemap builds the per-appdata-path remap for a cross-pool
// (foreign) container restore: each backed-up appdata path's contents are
// restored into <destBase>/<container-relative path> on the destination
// pool, and the recreated container's binds and template are pointed
// there. destBase is a container path under HostMountRoot. It returns (1)
// the restic restore dirs (Subtree = the source appdata path, one of the
// snapshot's own backed-up paths; Target = the dest path), and (2) a
// bindRemap of host source path to host dest path for rewriting
// HostConfig.Binds and the flashed template.
//
// The destination keeps the path structure below the source's appdata
// root, not just the final segment. resolveAppdataPaths records every
// appdata bind as its own entry without merging binds that share a folder,
// so a container with two binds, /mnt/user/appdata/SnapOtter/conf and
// …/SnapOtter/data, arrives here as two paths sharing the "SnapOtter"
// ancestor. Mapped by basename alone they would land as "conf" and "data"
// directly in the destination appdata root, next to (and able to
// overwrite) other containers' folders; the relative path nests them under
// <destBase>/SnapOtter/…. For a single bind the relative path of
// /mnt/user/appdata/nexterm is "nexterm", so it lands in
// <destBase>/nexterm.
//
// RestoreSubtreeTo dumps the subtree's contents directly into Target (no
// path nesting, #62), so two source paths mapping to the same Target would
// silently merge data. Uniqueness is therefore enforced on the destination
// root folder: a residual name collision (the same container folder name
// from two different pools) gets a "-2"/"-3" suffix. The suffix is decided
// once per source root directory and reused by every path below it, so a
// multi-bind container is never split across <destBase>/SnapOtter and
// <destBase>/SnapOtter-2. Distinct source roots always get distinct
// destination roots, and two paths under the same source root differ in
// their remainder, so every Target stays unique.
func (s *Service) containerAppdataRemap(destBase string, appdataPaths []string) ([]backup.RestoreDir, map[string]string) {
	base := path.Clean(destBase)
	dirs := make([]backup.RestoreDir, 0, len(appdataPaths))
	remap := make(map[string]string, len(appdataPaths))
	destRootFor := map[string]string{} // source root dir -> its assigned dest root name
	takenRoot := map[string]bool{}     // dest root names already handed out
	for _, cp := range appdataPaths {
		src := path.Clean(cp)
		root, rel := appdataRelPath(src)
		if rel == "" {
			root, rel = path.Dir(src), path.Base(src)
		}
		rootName, sub, _ := strings.Cut(rel, "/")
		srcRoot := path.Join(root, rootName)
		destRoot, ok := destRootFor[srcRoot]
		if !ok {
			destRoot = rootName
			for n := 2; takenRoot[destRoot]; n++ {
				destRoot = rootName + "-" + strconv.Itoa(n)
			}
			takenRoot[destRoot] = true
			destRootFor[srcRoot] = destRoot
		}
		dest := base + "/" + destRoot
		if sub != "" {
			dest += "/" + sub
		}
		dirs = append(dirs, backup.RestoreDir{Subtree: src, Target: dest})
		remap[s.toHostPath(src)] = s.toHostPath(dest)
		// The container's bind points at the mount, not at the folder inside it
		// that the selection narrowed to, and rewriteBinds matches exactly. So map
		// the root level too, or a narrowed selection leaves the bind pointing at
		// the source host's path on this host: the restored data sits under the
		// destination and the recreated container starts against something else
		// entirely, with no warning (foreignBindWarnings stays quiet because
		// /mnt/user/appdata is mounted on every Unraid).
		//
		// Never overwrite a more specific entry: a selection that is the root
		// already wrote the identical value.
		if rootKey := s.toHostPath(srcRoot); remap[rootKey] == "" {
			remap[rootKey] = s.toHostPath(base + "/" + destRoot)
		}
	}
	return dirs, remap
}

// rewriteBinds points a recreated container's docker binds at the remapped
// appdata locations. Each bind is "HOSTPATH:CONTAINERPATH[:opts]"; only the
// host part is rewritten, and only on an exact match against a remap key,
// so appdata binds move to the destination while docker.sock,
// /etc/localtime, /dev/dri and every other non-appdata bind (which carry no
// backed-up data) stay byte for byte. The container path and option suffix
// (:ro,:z,…) are preserved.
func rewriteBinds(binds []string, remap map[string]string) []string {
	if len(binds) == 0 || len(remap) == 0 {
		return binds
	}
	out := make([]string, len(binds))
	for i, b := range binds {
		if host, rest, found := strings.Cut(b, ":"); found {
			// Canonicalize the bind host before the lookup: the remap keys are
			// path.Clean'd host paths, so a non-canonical bind (trailing/doubled
			// slash) must be cleaned the same way or it would be left pointing at the
			// source pool while the warning classifier (foreignBindWarnings, which
			// also cleans) silently treats it as remapped appdata.
			if nw, ok := remap[path.Clean(host)]; ok {
				out[i] = nw + ":" + rest
				continue
			}
		}
		out[i] = b
	}
	return out
}

// rewriteMountSources rewrites the Source of each captured Mount whose source is a
// remap key, for display fidelity in the recreated definition (the actual recreate
// binds come from HostConfig.Binds via rewriteBinds). Exact match; empty remap is a
// no-op.
func rewriteMountSources(mounts []model.Mount, remap map[string]string) []model.Mount {
	if len(mounts) == 0 || len(remap) == 0 {
		return mounts
	}
	out := make([]model.Mount, len(mounts))
	for i, m := range mounts {
		if nw, ok := remap[path.Clean(m.Source)]; ok {
			m.Source = nw
		}
		out[i] = m
	}
	return out
}

// dirNonEmpty reports whether p exists and contains at least one entry,
// the overwrite guard's "there is already data here" check.
func dirNonEmpty(p string) bool {
	entries, err := os.ReadDir(p)
	return err == nil && len(entries) > 0
}

// dirNonEmptyFn returns the overwrite guard's dir probe: the injected test seam
// when set, else the real filesystem check.
func (s *Service) dirNonEmptyFn() func(string) bool {
	if s.dirNonEmptyProbe != nil {
		return s.dirNonEmptyProbe
	}
	return dirNonEmpty
}

// nearestExistingDir walks up from p until it finds a directory that exists, so
// a free-space probe (statfs needs a real path) can run against the filesystem
// the restore will write into even before the leaf destination dir is created.
func nearestExistingDir(p string) string {
	for {
		if _, err := os.Stat(p); err == nil {
			return p
		}
		parent := path.Dir(p)
		if parent == p {
			return p
		}
		p = parent
	}
}
