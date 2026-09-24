package api

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/junkerderprovinz/bombvault/internal/backup"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/selfrestore"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// configSnapshotDir is the staging directory for the config self-backup: a
// consistent, restic-ready copy of BombVault's own /config state, rebuilt
// before each config backup and removed afterwards. It lives under DataDir so
// it travels with the /config mount.
func (s *Service) configSnapshotDir() string { return filepath.Join(s.cfg.DataDir, ".snapshot") }

// stageConfigSnapshot builds a consistent, restic-ready copy of BombVault's own
// /config state in a staging dir: a VACUUM-INTO snapshot of the live DB plus the
// rclone.conf and ssh/ keypair (copied as-is; they are static files). The live
// DB is never handed to restic directly (WAL mode can tear a raw file copy).
// Returns the staging dir; the caller removes it after the backup.
func (s *Service) stageConfigSnapshot() (string, error) {
	dir := s.configSnapshotDir()
	if err := os.RemoveAll(dir); err != nil {
		return "", fmt.Errorf("config snapshot: clear staging: %w", err)
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("config snapshot: mkdir staging: %w", err)
	}
	// A partial staging holds sensitive plaintext (the settings DB, rclone.conf
	// creds, the ssh private key). Error paths below return before BackupConfig
	// has registered its `defer os.RemoveAll(stagingDir)`, so clean up here
	// unless the end is reached with ok=true.
	ok := false
	defer func() {
		if !ok {
			_ = os.RemoveAll(dir) // never leave a partial snapshot (DB + creds + ssh key) on disk
		}
	}()
	stagedDB := filepath.Join(dir, "bombvault.sqlite")
	if err := s.store.VacuumInto(stagedDB); err != nil {
		return "", err
	}
	// The SQLite driver creates the VACUUM'd DB at its default mode (~0o644); tighten
	// it to 0o600 so the staged settings DB is never group/other-readable, matching
	// the rclone.conf + ssh copies below. Defense-in-depth: the staging dir is already
	// 0o700, but the DB should not rely on the dir mode alone.
	if err := os.Chmod(stagedDB, 0o600); err != nil {
		return "", fmt.Errorf("config snapshot: chmod db: %w", err)
	}
	// rclone.conf + ssh/ are static on disk; copy verbatim if present.
	if src := filepath.Join(s.cfg.DataDir, "rclone.conf"); fileExists(src) {
		if err := copyFile(src, filepath.Join(dir, "rclone.conf"), 0o600); err != nil {
			return "", fmt.Errorf("config snapshot: copy rclone.conf: %w", err)
		}
	}
	if src := filepath.Join(s.cfg.DataDir, "ssh"); dirExists(src) {
		if err := copyTree(src, filepath.Join(dir, "ssh")); err != nil {
			return "", fmt.Errorf("config snapshot: copy ssh: %w", err)
		}
	}
	ok = true
	return dir, nil
}

// dirExists reports whether p exists and is a directory.
func dirExists(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.IsDir()
}

// copyFile copies src to dst with the given mode, truncating dst if it exists.
func copyFile(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src) //nolint:gosec // G304: src is an internal DataDir path (rclone.conf / ssh key), not user-supplied
	if err != nil {
		return err
	}
	// in is read-only; a close error is not actionable.
	defer func() { _ = in.Close() }()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode) //nolint:gosec // G304: dst is under our staging dir
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close() //nolint:errcheck,gosec // cleanup on error path; original error takes priority
		return err
	}
	return out.Close()
}

// copyTree recursively copies the directory src to dst, preserving file modes.
func copyTree(src, dst string) error {
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dst, 0o700); err != nil {
		return err
	}
	for _, e := range entries {
		sp := filepath.Join(src, e.Name())
		dp := filepath.Join(dst, e.Name())
		if e.IsDir() {
			if err := copyTree(sp, dp); err != nil {
				return err
			}
			continue
		}
		info, err := e.Info()
		if err != nil {
			return err
		}
		// Cap at 0o600: these are private keys/config; never widen perms on copy.
		mode := info.Mode().Perm()
		if mode > 0o600 {
			mode = 0o600
		}
		if err := copyFile(sp, dp, mode); err != nil {
			return err
		}
	}
	return nil
}

// StartBackupConfig launches the singleton config self-backup in a background
// goroutine and returns immediately, mirroring StartBackupFlash. Progress is
// published under "config". Shares batchActive; returns (false, nil) if a backup
// is already running, or (false, err) if the config domain is already busy with
// another op.
func (s *Service) StartBackupConfig(ctx context.Context) (bool, error) {
	if !s.batchActive.CompareAndSwap(false, true) {
		return false, nil
	}
	if op, busy := s.domainBusy("config"); busy {
		s.batchActive.Store(false)
		return false, fmt.Errorf("%s is running on config", op)
	}
	bctx := context.WithoutCancel(ctx)
	go func() {
		defer s.recoverOperation("backup config: "+store.ConfigTargetID, nil, func(msg string) {
			s.failStuckRun(store.ConfigTargetID, msg)
		})
		defer s.batchActive.Store(false)
		if _, err := s.BackupConfig(bctx); err != nil {
			log.Printf("api: backup config failed: %v", err)
		}
	}()
	return true, nil
}

// resticAdapter also satisfies the config domain's backup surface.
var _ backup.ConfigRestic = (*resticAdapter)(nil)

// BackupConfig backs up BombVault's own /config folder (the settings DB +
// rclone.conf + ssh/ keypair) to the config repo via restic. Unlike flash it
// never hands restic the live folder: it first stages a consistent, restic-ready
// snapshot (VACUUM-INTO of the WAL-mode DB + verbatim static files) and always
// removes that snapshot afterwards, so a rebuilt Unraid box can recover BombVault
// itself with no container stop.
func (s *Service) BackupConfig(ctx context.Context) (backup.Summary, error) {
	// Survive the client that triggered it disconnecting (see Backup): detach from
	// the request's cancellation with a generous hard cap.
	ctx, cancel := backupHoldCtx(ctx)
	defer cancel()
	s.registerBackupCancel("config", cancel) // reachable by shutdown
	defer s.unregisterBackupCancel("config")
	defer s.lockDomain("config")() // serialise per repo; blocks maintenance ops meanwhile
	settings, err := s.store.GetSettings()
	if err != nil {
		return backup.Summary{}, fmt.Errorf("read settings: %w", err)
	}
	// Build the consistent staging snapshot of /config; restic backs this up,
	// never the live WAL-mode DB. It is always removed afterwards.
	stagingDir, err := s.stageConfigSnapshot()
	if err != nil {
		return backup.Summary{}, err
	}
	defer func() { _ = os.RemoveAll(stagingDir) }()
	repo, err := s.configRepoPath(settings)
	if err != nil {
		return backup.Summary{}, err
	}
	// issue #152 (bandwidth caps) and #182 (this domain's own credential set)
	mode := s.primaryModeFor(settings, "config", repo)
	if err := s.EnsureRepo(ctx, repo, mode); err != nil {
		return backup.Summary{}, err
	}
	// Clear any stale lock left by a previously interrupted run so it can't block
	// this backup (BombVault is the sole writer; an active lock is never stale).
	s.unlockStale(ctx, repo, mode)
	// Healthchecks /start ping: deferred to here, past staging + EnsureRepo guards,
	// so the paired done/fail notifyBackup below always follows (no dangling /start).
	s.notifyBackupStart(ctx, "config")
	fctx, startedAt := s.progBegin(ctx, "config", "backup")
	sum, err := backup.BackupConfig(fctx, backup.ConfigBackupDeps{
		SourceDir: stagingDir,
		Repo:      repo,
		TargetID:  store.ConfigTargetID,
		Restic:    &resticAdapter{engine: s.engine, mode: mode},
		Runs:      runsAdapter{st: s.store, ctx: ctx, svc: s, cancelKey: "config"},
	})
	s.progEnd("config", "backup", err == nil, startedAt)
	s.notifyBackup(ctx, "config", "", err == nil, sum, err)
	if err != nil {
		return backup.Summary{}, err
	}
	s.applyRetention(ctx, repo, settings, mode, tagIdentity("config"), "config")
	s.replicateOffsite(ctx, "config", settings, repo, "")
	s.collectStatsAfterItem(ctx, "config")
	s.checkPrimaryRemoteBudget(ctx, "config", repo, settings)
	return sum, nil
}

// resolveConfigSnapshot maps a user-supplied selector ("" / "latest", a full id,
// or a short prefix) to the single matching full snapshot id in the config repo.
// It is resolveFlashSnapshot with a config-worded empty message: the config repo
// is dedicated to BombVault's own /config snapshots, so an empty repo means the
// app has never backed itself up yet.
func resolveConfigSnapshot(snaps []restic.Snapshot, selector string) (string, error) {
	if len(snaps) == 0 {
		return "", errors.New("BombVault's configuration has not been backed up yet")
	}
	if selector == "" || selector == "latest" {
		return snaps[len(snaps)-1].ID, nil
	}
	var match string
	for _, s := range snaps {
		if s.ID == selector {
			return s.ID, nil // exact id wins outright
		}
		if strings.HasPrefix(s.ID, selector) {
			if match != "" {
				return "", errors.New("ambiguous snapshot id")
			}
			match = s.ID
		}
	}
	if match == "" {
		return "", notInListing{id: selector}
	}
	return match, nil
}

// SnapshotsConfig lists restic snapshots in the config repo (the repo is
// dedicated to the config self-backup, so all of its snapshots are config
// backups). Mirrors SnapshotsFlash.
func (s *Service) SnapshotsConfig(ctx context.Context, source string) ([]restic.Snapshot, error) {
	settings, err := s.store.GetSettings()
	if err != nil {
		return nil, fmt.Errorf("read settings: %w", err)
	}
	repo, err := s.repoFor(settings, "config", source)
	if err != nil {
		return nil, err
	}
	mode := s.repoModeFor(settings, "config", source, repo)
	if localRepoMissing(repo) {
		// #55 vs #120: only surface "not mounted" when the backing store is truly
		// absent. If the destination is mounted, this is a fresh or phantom repo
		// on a healthy disk, so report an empty list (EnsureRepo re-establishes on
		// write).
		if s.repoEstablished(repo) && !s.destinationMounted(repo) {
			return nil, ErrBackupPathNotMounted // #55: backing store not mounted
		}
		return nil, nil // no backups yet
	}
	return s.listSnapshots(ctx, repo, mode)
}

// RestoreConfig stages a restore of BombVault's own /config: it cannot
// overwrite the live SQLite settings DB in place while this process holds
// it open (WAL), so it restic-restores the chosen snapshot into a staging
// root and writes a marker. The boot-time selfrestore.ApplyPending (called
// from main before store.Open on the next restart) performs the file-level
// staging→live swap. The restart is triggered separately (docker
// self-restart or manual), so this call only stages.
func (s *Service) RestoreConfig(ctx context.Context, snapshotID, source string) error {
	settings, err := s.store.GetSettings()
	if err != nil {
		return fmt.Errorf("read settings: %w", err)
	}
	repo, err := s.repoFor(settings, "config", source)
	if err != nil {
		return err
	}
	mode := s.repoModeFor(settings, "config", source, repo)
	snaps, err := s.engine.Snapshots(ctx, repo, mode)
	if err != nil {
		return err
	}
	id, err := resolveConfigSnapshot(snaps, snapshotID)
	if err != nil {
		return err
	}
	root := selfrestore.StagingRoot(s.cfg.DataDir)
	if err := os.RemoveAll(root); err != nil {
		return fmt.Errorf("config restore: clear staging: %w", err)
	}
	// Also clear any stale <root>.bad left by a failed restore on a prior
	// boot: it contains a plaintext rclone.conf and ssh private key, so a fresh
	// attempt should not let it linger. Best-effort; a leftover .bad must
	// never block a restore.
	_ = os.RemoveAll(root + ".bad")
	runID, err := s.store.StartRun(store.ConfigTargetID, "restore")
	if err != nil {
		return fmt.Errorf("config restore: start run: %w", err)
	}
	// Restore only the config snapshot's subtree (<DataDir>/.snapshot) into
	// the staging root; restic recreates that absolute path beneath --target,
	// landing it at selfrestore.RestoredSnapshotDir(DataDir), the exact path
	// the boot swap reads. The swap applies it on the next restart.
	if rerr := s.engine.RestoreInclude(ctx, repo, id, s.configSnapshotDir(), root, mode); rerr != nil {
		_ = s.store.FinishRun(runID, "failed", "", 0, rerr.Error())
		return rerr
	}
	if merr := selfrestore.WriteMarker(s.cfg.DataDir); merr != nil {
		_ = s.store.FinishRun(runID, "failed", "", 0, merr.Error())
		return merr
	}
	_ = s.store.FinishRun(runID, "success", id, 0, "")
	return nil
}

// StartRestoreConfig stages a restore of BombVault's own /config and, on
// success, triggers the self-restart that applies it on the next boot. It
// takes the shared single-flight guard (batchActive), so it never overlaps
// another backup or restore; a config self-restart would otherwise kill
// the container mid-write of an in-flight data restore. It returns
// (started, autoRestart, err): started=false with a nil err means another
// operation is already running; autoRestart=false means the caller must
// ask the user to restart the container manually. When an auto-restart is
// scheduled the guard stays held until the container goes down, so nothing
// new starts in the restart window; if the restart fails,
// ScheduleSelfRestart releases it.
//
// Unlike the other Start* restores, this one does not hand the restore to
// a background goroutine: the caller (the restore-own-config step in
// Recovery.tsx) needs the staged/autoRestart outcome synchronously to
// decide between polling for the self-restart and showing manual-restart
// instructions. Like RunRestoreDrill, RestoreConfig runs on this goroutine
// against a context detached from ctx (context.WithoutCancel) and capped
// by restoreTimeout, so a closed tab or a proxy idle timeout cannot kill a
// restore mid-write.
//
// A truncated restore does not corrupt the live config on the next boot:
// selfrestore.ApplyPending is marker-gated and runs validSQLite (PRAGMA
// quick_check) on the staged DB before touching the live one, moving a bad
// staged DB to <root>.bad. The remaining risk is narrower: RestoreConfig
// clears the staging dir but not a stale marker, so a successful restore
// (autoRestart=false, marker pending until a manual restart) followed by a
// failed one can leave the marker pointing at partial staging, and
// ApplyPending checks only the existence of rclone.conf and ssh/, not
// their content. On a client disconnect during a config restore, the
// restore still completes and self-restarts, but the SPA's fetch reports a
// failure; fixing that would mean reporting the outcome by polling or SSE
// instead of the HTTP response. A panic is recovered like every other
// manual op: RestoreConfig's run id is local to it, so the fallback is
// FailRunningRun keyed by store.ConfigTargetID.
func (s *Service) StartRestoreConfig(ctx context.Context, snapshotID, source string) (started bool, autoRestart bool, err error) {
	if !s.batchActive.CompareAndSwap(false, true) {
		return false, false, nil
	}
	if op, busy := s.domainBusy("config"); busy {
		s.batchActive.Store(false)
		return false, false, fmt.Errorf("%s is running on config", op)
	}
	// On a recovered panic the guard has to be released here as well: unlike
	// the success path below, nothing else left running would release it, and
	// every later backup or restore would refuse forever with "already
	// running".
	defer s.recoverOperation("restore config: "+store.ConfigTargetID, &err, func(msg string) {
		s.batchActive.Store(false)
		s.failStuckRun(store.ConfigTargetID, msg)
	})
	bctx := context.WithoutCancel(ctx)
	rctx, cancel := context.WithTimeout(bctx, restoreTimeout)
	defer cancel()
	if rerr := s.RestoreConfig(rctx, snapshotID, source); rerr != nil {
		s.batchActive.Store(false)
		return false, false, rerr
	}
	autoRestart = s.ScheduleSelfRestart()
	if !autoRestart {
		// No auto-restart scheduled (Docker self unreachable): let normal operations
		// resume. The staged restore applies on the next manual boot and does not
		// affect anything running now.
		s.batchActive.Store(false)
	}
	// autoRestart: keep the guard held; ScheduleSelfRestart's goroutine releases it
	// if the restart call fails.
	return true, autoRestart, nil
}
