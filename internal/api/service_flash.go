package api

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/ageseal"
	"github.com/junkerderprovinz/bombvault/internal/backup"
	"github.com/junkerderprovinz/bombvault/internal/paths"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// flashZipExportDir resolves the operator-configured output folder for the
// scheduled flash zip export. Unlike flashRepoPath, which may hand a remote
// backend like "s3:…" straight to restic, this is always a local folder, so
// it applies only the containment half of resolveRepo: paths.Resolve rejects
// absolute paths and traversal.
func (s *Service) flashZipExportDir(settings store.Settings) (string, error) {
	dir, err := paths.Resolve(s.cfg.HostMountRoot, settings.FlashZipExportPath)
	if err != nil {
		return "", fmt.Errorf("flash zip export: resolve path: %w", err)
	}
	return dir, nil
}

// StartBackupFlash launches the singleton flash backup in a background goroutine
// and returns immediately, mirroring StartBackup. Progress is published under
// "flash". Shares batchActive; returns (false, nil) if a backup is already
// running, or (false, err) if the flash domain is already busy with another op.
func (s *Service) StartBackupFlash(ctx context.Context) (bool, error) {
	if !s.batchActive.CompareAndSwap(false, true) {
		return false, nil
	}
	if op, busy := s.domainBusy("flash"); busy {
		s.batchActive.Store(false)
		return false, fmt.Errorf("%s is running on flash", op)
	}
	bctx := context.WithoutCancel(ctx)
	go func() {
		defer s.recoverOperation("backup flash: "+store.FlashTargetID, nil, func(msg string) {
			s.failStuckRun(store.FlashTargetID, msg)
		})
		defer s.batchActive.Store(false)
		if _, err := s.BackupFlash(bctx); err != nil {
			log.Printf("api: backup flash failed: %v", err)
		}
	}()
	return true, nil
}

// resticAdapter also satisfies the flash domain's backup surface.
var _ backup.FlashRestic = (*resticAdapter)(nil)

// BackupFlash backs up the whole Unraid USB flash (the mounted /boot) to the
// flash repo via restic. Fails with a clear message if the flash directory is
// not mounted (the /boot → /host/boot mount is required for this domain).
func (s *Service) BackupFlash(ctx context.Context) (backup.Summary, error) {
	// Survive the client that triggered it disconnecting (see Backup): detach from
	// the request's cancellation with a generous hard cap.
	ctx, cancel := backupHoldCtx(ctx)
	defer cancel()
	// Reachable by shutdown, like every other backup.
	s.registerBackupCancel("flash", cancel)
	defer s.unregisterBackupCancel("flash")
	defer s.lockDomain("flash")() // serialise per repo; blocks maintenance ops meanwhile
	settings, err := s.store.GetSettings()
	if err != nil {
		return backup.Summary{}, fmt.Errorf("read settings: %w", err)
	}
	if _, statErr := os.Stat(s.cfg.FlashDir); errors.Is(statErr, fs.ErrNotExist) {
		return backup.Summary{}, fmt.Errorf("flash backup: the Unraid flash is not mounted. Add the /boot → %s mount to the container template", s.cfg.FlashDir)
	}
	repo, err := s.flashRepoPath(settings)
	if err != nil {
		return backup.Summary{}, err
	}
	// issue #152 (bandwidth caps) and #182 (this domain's own credential set)
	mode := s.primaryModeFor(settings, "flash", repo)
	if err := s.EnsureRepo(ctx, repo, mode); err != nil {
		return backup.Summary{}, err
	}
	// Clear any stale lock left by a previously interrupted run so it can't block
	// this backup (BombVault is the sole writer; an active lock is never stale).
	s.unlockStale(ctx, repo, mode)
	// Healthchecks /start ping: deferred to here, past the /boot-mounted + EnsureRepo
	// guards, so the paired done/fail notifyBackup below always follows (no dangling /start).
	s.notifyBackupStart(ctx, "flash")
	fctx, startedAt := s.progBegin(ctx, "flash", "backup")
	sum, err := backup.BackupFlash(fctx, backup.FlashBackupDeps{
		SourceDir: s.cfg.FlashDir,
		Repo:      repo,
		TargetID:  store.FlashTargetID,
		Restic:    &resticAdapter{engine: s.engine, mode: mode},
		Runs:      runsAdapter{st: s.store, ctx: ctx, svc: s, cancelKey: "flash"},
	})
	s.progEnd("flash", "backup", err == nil, startedAt)
	s.notifyBackup(ctx, "flash", "", err == nil, sum, err)
	if err != nil {
		return backup.Summary{}, err
	}
	s.applyRetention(ctx, repo, settings, mode, tagIdentity("flash"), "flash")
	makeRepoReadable(repo, s.cfg.DataDir) // keep the local repo copyable off-box by a non-root user
	s.replicateOffsite(ctx, "flash", settings, repo, "")
	s.collectStatsAfterItem(ctx, "flash")
	s.checkPrimaryRemoteBudget(ctx, "flash", repo, settings)
	if err := s.exportFlashZip(ctx, settings, sum.SnapshotID, mode, repo); err != nil {
		log.Printf("flash zip export failed (backup is still valid): %v", err)
	}
	return sum, nil
}

// flashZipRe matches only the timestamped export filenames pruneFlashZips is
// allowed to delete (flash-<YYYYMMDD>-<HHMMSS>.zip, or the .age-sealed variant
// when export encryption is on). flash-latest.zip(.age) and any unrelated file the
// operator drops in the folder never match, so they survive.
var flashZipRe = regexp.MustCompile(`^flash-\d{8}-\d{6}\.zip(\.age)?$`)

// exportFlashZip writes the just-backed-up flash snapshot to the configured
// folder as a plain .zip, for off-server sync (Syncthing etc.). It is
// non-fatal: any failure is returned to the caller (BackupFlash logs it)
// and never fails the backup itself. The write is atomic (a temp file is
// renamed into place), so a sync tool never sees a half-written zip. Each
// attempt is recorded as a kind="export" run on the flash target (bytes =
// the written zip size) so it shows in the dashboard Activity Log and Run
// History; a disabled export records nothing.
func (s *Service) exportFlashZip(ctx context.Context, settings store.Settings, snapshotID string, mode restic.Mode, repo string) (err error) {
	if !settings.FlashZipExportEnabled || settings.FlashZipExportPath == "" {
		return nil
	}
	// Resolve age recipients up front: with export encryption on but no valid
	// recipient this fails before the temp zip is created, so no plaintext
	// artifact is produced when the user asked for encryption.
	recipients, _, err := s.exportRecipients(settings)
	if err != nil {
		return err
	}
	// Publish a live "maintenance" progress pair keyed "export:flash" (like
	// prune:, verify:, drill:) so a long zip export of a big flash shows on the
	// dashboard activity log while it writes, not only after (#109). The
	// terminal event is deferred so any failure path above clears the live
	// line. A disabled export publishes nothing, hence below the guard.
	_, startedAt := s.progBegin(ctx, "export:flash", "maintenance")
	defer func() { s.progEnd("export:flash", "maintenance", err == nil, startedAt) }()
	runID, rErr := s.store.StartRun(store.FlashTargetID, "export")
	if rErr != nil {
		log.Printf("api: flash zip export: could not start run record (continuing): %v", rErr)
		runID = ""
	}
	var zipBytes int64
	defer func() {
		if runID == "" {
			return
		}
		status := "success"
		if err != nil {
			status = "failed"
		}
		if fErr := s.store.FinishRun(runID, status, "", zipBytes, truncateRunErr(err)); fErr != nil {
			log.Printf("api: flash zip export: could not finish run record: %v", fErr)
		}
	}()
	dir, err := s.flashZipExportDir(settings)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil { //nolint:gosec // G301: operator-configured sync folder must be readable by the off-server sync tool
		return fmt.Errorf("flash zip export: mkdir: %w", err)
	}
	tmp := filepath.Join(dir, ".flash-export.tmp.zip")
	f, err := os.Create(tmp) //nolint:gosec // G304: dir is an operator-configured path under the host mount root
	if err != nil {
		return fmt.Errorf("flash zip export: create temp: %w", err)
	}
	dumpErr := s.engine.DumpZip(ctx, repo, snapshotID, s.cfg.FlashDir, f, mode)
	closeErr := f.Close()
	if dumpErr != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("flash zip export: dump: %w", dumpErr)
	}
	if closeErr != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("flash zip export: close temp: %w", closeErr)
	}
	name := "flash-latest.zip"
	if settings.FlashZipExportKeep > 0 {
		name = "flash-" + time.Now().UTC().Format("20060102-150405") + ".zip"
	}
	// Publish the temp zip: plain rename, or age-encrypted to <name>.zip.age when
	// export encryption is on (the plaintext temp is removed by sealOrRename).
	final, err := sealOrRename(tmp, filepath.Join(dir, name), recipients)
	if err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("flash zip export: finalize: %w", err)
	}
	// Cheap size sample for the run record (bytes column), best-effort only.
	if fi, statErr := os.Stat(final); statErr == nil {
		zipBytes = fi.Size()
	}
	// Prune in both modes: in latest mode (Keep==0) the user opted out of history,
	// so this deletes any stale flash-<ts>.zip left over from a previous history
	// run; in history mode (Keep>0) it trims to the newest N. flash-latest.zip never
	// matches flashZipRe, so it is never touched.
	s.pruneFlashZips(dir, settings.FlashZipExportKeep)
	return nil
}

// pruneFlashZips keeps the newest `keep` timestamped flash-*.zip files, deleting
// older ones. Best-effort; only files matching the exact flash-<ts>.zip pattern
// are ever touched (flash-latest.zip and unrelated files are left alone).
func (s *Service) pruneFlashZips(dir string, keep int) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		log.Printf("flash zip export: prune: read dir: %v", err)
		return
	}
	var zips []string
	for _, e := range entries {
		if !e.IsDir() && flashZipRe.MatchString(e.Name()) {
			zips = append(zips, e.Name())
		}
	}
	sort.Sort(sort.Reverse(sort.StringSlice(zips))) // timestamp names sort chronologically → newest first
	if keep >= len(zips) {
		return
	}
	for _, name := range zips[keep:] {
		if err := os.Remove(filepath.Join(dir, name)); err != nil {
			log.Printf("flash zip export: prune: remove %q: %v", name, err)
		}
	}
}

// FlashDownloadName is the suggested filename for a flash zip download.
func FlashDownloadName(id string) string { return "flash-" + id + ".zip" }

// resolveFlashSnapshot maps a user-supplied selector ("" / "latest", a
// full id, or a short prefix) to the single matching full snapshot id. It
// errors when the selector matches none or is an ambiguous prefix of more
// than one, so the caller rejects it before any download bytes or headers
// are committed, and restic always receives an unambiguous full id.
func resolveFlashSnapshot(snaps []restic.Snapshot, selector string) (string, error) {
	if len(snaps) == 0 {
		return "", errors.New("flash has not been backed up yet")
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

// DownloadFlashZip streams a flash snapshot to w as a zip (restic dump),
// the non-destructive flash restore: the live /boot is never touched and no
// filesystem metadata is restored (so it can't hit the per-file permission
// errors a to-disk restore gets on /mnt/user), and via dumpFlashZipCompat
// the file drops straight into the Unraid USB creator, which restic's own
// zip dump does not (#136).
//
// "latest"/"" resolves to the newest snapshot; an explicit id is validated
// against the repo. onResolved (optional) is called with the concrete id
// once it is known-good and before streaming begins, so the HTTP handler
// sets the download headers only on the happy path. A restore run is
// recorded for history.
func (s *Service) DownloadFlashZip(ctx context.Context, snapshotID, source string, onResolved func(id string), w io.Writer) error {
	settings, err := s.store.GetSettings()
	if err != nil {
		return fmt.Errorf("read settings: %w", err)
	}
	// Resolve age recipients up front: with export encryption on but no valid
	// recipient the download fails before onResolved fires (no headers) and
	// before any bytes are streamed, so a plaintext zip is never sent.
	recipients, encOn, err := s.exportRecipients(settings)
	if err != nil {
		return err
	}
	repo, err := s.repoFor(settings, "flash", source)
	if err != nil {
		return err
	}
	mode := s.repoModeFor(settings, "flash", source, repo)
	snaps, err := s.engine.Snapshots(ctx, repo, mode)
	if err != nil {
		return err
	}
	id, err := resolveFlashSnapshot(snaps, snapshotID)
	if err != nil {
		return err
	}
	if onResolved != nil {
		onResolved(id)
	}
	runID, err := s.store.StartRun(store.FlashTargetID, "restore")
	if err != nil {
		return fmt.Errorf("flash download: start run: %w", err)
	}
	// With encryption on, wrap the response writer so the streamed zip is
	// age-sealed on the fly. The age writer has to be closed to finalize the
	// stream.
	dst := w
	var ageW io.WriteCloser
	if encOn {
		ageW, err = ageseal.WrapWriter(w, recipients)
		if err != nil {
			_ = s.store.FinishRun(runID, "failed", "", 0, err.Error())
			return err
		}
		dst = ageW
	}
	if derr := s.dumpFlashZipCompat(ctx, repo, id, s.cfg.FlashDir, dst, mode); derr != nil {
		// A client disconnect or user cancel of the download is context.Canceled;
		// record it as "cancelled", not a failure.
		status, msg := "failed", derr.Error()
		if errors.Is(derr, context.Canceled) {
			status, msg = "cancelled", "cancelled by user"
		}
		_ = s.store.FinishRun(runID, status, "", 0, msg)
		return derr
	}
	if ageW != nil {
		if cerr := ageW.Close(); cerr != nil { // flush + finalize the age stream
			_ = s.store.FinishRun(runID, "failed", "", 0, cerr.Error())
			return cerr
		}
	}
	_ = s.store.FinishRun(runID, "success", id, 0, "")
	return nil
}

// ExportEncryptionOn reports whether the plain-export age encryption is enabled
// (best-effort; a settings read error reports false). The flash-download handler
// uses it to append the ".age" suffix to the Content-Disposition filename before
// streaming begins.
func (s *Service) ExportEncryptionOn() bool {
	settings, err := s.store.GetSettings()
	if err != nil {
		return false
	}
	return settings.ExportEncryptEnabled
}

// SnapshotsFlash lists restic snapshots in the flash repo (the repo is dedicated
// to flash, so all of its snapshots are flash backups).
func (s *Service) SnapshotsFlash(ctx context.Context, source string) ([]restic.Snapshot, error) {
	settings, err := s.store.GetSettings()
	if err != nil {
		return nil, fmt.Errorf("read settings: %w", err)
	}
	repo, err := s.repoFor(settings, "flash", source)
	if err != nil {
		return nil, err
	}
	mode := s.repoModeFor(settings, "flash", source, repo)
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
