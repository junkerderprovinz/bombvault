// Package backup — VM orchestrators for graceful-shutdown backup and restore.
// This file mirrors orchestrator.go's patterns: DI interfaces, ALWAYS-restart
// guard via defer, confirmation + path validation guards.
package backup

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

// parentDirs returns the deduplicated parent directories of the given absolute
// file paths (slash semantics — these are container Linux paths). Used to
// restore VM disk/NVRAM FILES via restic's directory-subtree restore. The root
// "/" is never returned (defensive: never restore the whole filesystem).
func parentDirs(paths []string) []string {
	seen := map[string]bool{}
	var dirs []string
	for _, p := range paths {
		d := path.Dir(p)
		if d == "" || d == "/" || d == "." || seen[d] {
			continue
		}
		seen[d] = true
		dirs = append(dirs, d)
	}
	return dirs
}

// ---------------------------------------------------------------------------
// VM DI interface (the seam — no concrete virshcli imported here)
// ---------------------------------------------------------------------------

// VM is the subset of virsh host control the VM orchestrators need.
// Any adapter satisfying virshcli.Virsh automatically satisfies VM since
// virshcli.Virsh is a superset of this interface.
type VM interface {
	State(ctx context.Context, name string) (string, error)
	IsActive(ctx context.Context, name string) (bool, error)
	DumpXML(ctx context.Context, name string) (string, error)
	Shutdown(ctx context.Context, name string) error
	Destroy(ctx context.Context, name string) error
	Start(ctx context.Context, name string) error
	Define(ctx context.Context, xmlPath string) error
	Undefine(ctx context.Context, name string) error
	Autostart(ctx context.Context, name string, on bool) error
	// SnapshotCreateDiskOnly creates an external, atomic, disk-only snapshot
	// (the VM keeps running and writes to a fresh overlay; the base goes
	// read-only). quiesce uses the qemu guest agent for app-consistency. skipDevs
	// lists target devices to exclude (cdrom / read-only) so they are not
	// snapshotted (which fails for non-block-device files).
	SnapshotCreateDiskOnly(ctx context.Context, name, snapName string, quiesce bool, skipDevs []string) error
	// BlockCommitActivePivot commits the active overlay back into its base and
	// pivots the running VM onto the base (blockcommit --active --pivot --wait).
	BlockCommitActivePivot(ctx context.Context, name, device string) error
	// GuestAgentPing reports whether the qemu guest agent answers inside the VM.
	GuestAgentPing(ctx context.Context, name string) bool
}

// ---------------------------------------------------------------------------
// VMBackupDeps / VMRestoreDeps
// ---------------------------------------------------------------------------

const (
	defaultVMShutdownPollInterval = 5 * time.Second
	defaultVMShutdownMaxPolls     = 18 // 18 × 5s = 90s timeout
)

// VMBackupDeps bundles everything BackupVMGraceful needs.
type VMBackupDeps struct {
	// Name is the libvirt domain name (used for tags + run recording).
	Name string
	// DiskPaths are the container-visible absolute paths to the disk images.
	DiskPaths []string
	// DiskDevice is the first disk's target dev (e.g. "vda", "hdc"). Used as the
	// blockcommit target for live backup when CommitDevs is empty (back-compat).
	DiskDevice string
	// CommitDevs are ALL writable disk devices the live snapshot creates an overlay
	// for, each of which must be committed back afterwards. A multi-disk VM needs
	// every overlay committed — committing only the first leaves the others
	// diverging on an uncommitted overlay. Falls back to [DiskDevice] when empty.
	CommitDevs []string
	// SkipSnapshotDevs are target devices excluded from the live snapshot
	// (cdrom / read-only disks). Passed through to SnapshotCreateDiskOnly.
	SkipSnapshotDevs []string
	// NVRAMPath is the container-visible NVRAM path (empty for BIOS VMs).
	NVRAMPath string
	// TPMPath is the container-visible vTPM device/state path (empty for a
	// VM with no vTPM — see virshcli.DomainInfo.TPMPath's doc comment for
	// how that's determined). Populated ADDITIVELY alongside NVRAMPath and
	// given the EXACT SAME treatment in runVMGraceful/runVMLive below: when
	// non-empty it is appended to the restic backup path list right after
	// NVRAMPath, so it rides along with the disks/NVRAM in the same restic
	// snapshot — no separate best-effort/non-fatal handling exists AT THIS
	// LAYER, because NVRAMPath doesn't get any either; that degrade (a
	// failed capture logs a warning and continues rather than failing the
	// whole backup) is entirely a property of internal/api/service.go's
	// SEPARATE SSH-based NVRAM read/write mechanism (bytes captured over SSH
	// and stored inline in the VM's definition, never touching this
	// path-list field) — see docs/vm-backup-ssh-setup.md's TrueNAS section
	// for the precise, honest statement of which mechanism is real. ⚠ LIKE
	// NVRAMPath, THIS FIELD IS NOT POPULATED BY internal/api/service.go's
	// ACTUAL BackupVM CALLER TODAY (confirmed by reading the real code, not
	// assumed) — service.go's BackupVM builds VMBackupDeps without ever
	// setting NVRAMPath, so both fields are exercised only by this package's
	// own direct unit tests until a caller wires them up. TPM introduces NO
	// gap beyond that PRE-EXISTING one; it does not make anything less
	// wired than NVRAM already is.
	TPMPath string
	// RepoPath is the local restic repository path for the vms domain.
	RepoPath string
	// TargetID is the run-recording target id.
	TargetID string
	// DataDir is used to write temp files (e.g. the vm-define xml dir).
	DataDir string
	// ShutdownTimeout is the maximum number of poll cycles to wait for
	// "shut off" state before calling Destroy. 0 = use default (18 × 5s = 90s).
	// Set to 1 in tests for instant timeout.
	ShutdownTimeout int

	// BlockDisks are the domain's block-device-backed (zvol) disks — additive
	// to DiskPaths above (see virshcli.DomainInfo.BlockDisks's doc comment) —
	// each backed up via BackupZvolDisk (snapshot → zfs send → restic --stdin
	// → destroy-snapshot, see backupBlockDisksAndLog below) INSTEAD of the
	// file-copy d.Restic.Backup call above, since restic cannot back up a raw
	// block device by path the way it backs up a file. The caller resolves
	// each entry's Dataset from the domain's block-device disk source path
	// (this package never imports virshcli — see the DI-seam comment at the
	// top of this file). Empty for a VM with only file-backed disks (every VM
	// in production today) — in that case this field, ZFSHost and ZvolRestic
	// below are never touched and backup behavior is BYTE-IDENTICAL to before
	// this field existed.
	//
	// ⚠ KNOWN GAP — see this file's "Zvol-aware VM disk backup/restore"
	// section header comment below: a mixed file+block VM backup produces
	// MULTIPLE restic snapshots (one for the file disks, recorded as
	// summary.SnapshotID below; one PER block disk, from its own
	// BackupZvolDisk call), because restic's --stdin mode cannot be combined
	// with a normal path-list backup in one invocation. This package's
	// run-recording call (Runs.Finish takes exactly one snapshotID per run)
	// only records the file-portion's snapshot id; each block disk's
	// snapshot id is logged (see backupBlockDisksAndLog) but NOT otherwise
	// persisted anywhere a later restore can find it. A caller wiring this up
	// for a REAL, restorable backup must solve that (e.g. a disk-unique tag +
	// its own snapshot lookup) — this package does not.
	BlockDisks []VMBlockDisk
	// ZFSHost / ZvolRestic are BackupZvolDisk's own dependencies (see their
	// doc comments below) — required only when BlockDisks is non-empty.
	ZFSHost    ZFSHost
	ZvolRestic ZvolRestic

	VM     VM
	Restic Restic
	Runs   Runs
}

// VMBlockDisk names ONE block-device-backed VM disk (a zvol) for
// VMBackupDeps.BlockDisks to back up via BackupZvolDisk.
type VMBlockDisk struct {
	// Dataset is the zvol's ZFS dataset ("<pool>/<dataset...>"), already
	// resolved by the caller from the domain's block-device disk source path
	// (e.g. via virshcli's zvol dataset parsing — this package never imports
	// virshcli, see the DI-seam comment at the top of this file).
	Dataset string
}

// VMRestoreDir pairs a snapshot subtree (Subtree, a dir the backup recorded)
// with the destination dir (Target) its contents are restored into. Used to
// place a cross-instance VM restore's disks on a chosen pool rather than the
// source server's original paths.
type VMRestoreDir struct {
	Subtree string
	Target  string
}

// VMRestoreDeps bundles everything RestoreVM needs.
type VMRestoreDeps struct {
	// Confirmed MUST be true — guard against an accidental destructive restore.
	Confirmed bool
	// Name is the libvirt domain name.
	Name string
	// SnapshotID is the restic snapshot to restore (validated hex).
	SnapshotID string
	// DiskPaths are the absolute container-visible paths the restored disks END UP
	// at (the destination). For a same-instance restore these ARE the snapshot's
	// own paths; for a cross-instance restore they are the remapped destination
	// paths. Used for the safety path check and (in the same-instance case) as the
	// restic restore subtrees.
	DiskPaths []string
	// RestoreDirs, when non-empty, drives a REMAPPED restore: each entry restores a
	// snapshot subtree (Subtree, the source dir) INTO a destination dir (Target),
	// instead of the default restore-each-path-back-to-its-own-location behaviour.
	// A cross-instance VM restore sets this so the disks land on the chosen pool.
	RestoreDirs []VMRestoreDir
	// NVRAMPath is the absolute container-visible NVRAM path (may be empty).
	NVRAMPath string
	// TPMPath is the absolute container-visible vTPM device/state path (may
	// be empty). Mirrors NVRAMPath's exact treatment in runVMRestore below —
	// included in the same path-safety validation and the same restic
	// restore-directory list as DiskPaths/NVRAMPath — and mirrors NVRAMPath's
	// own real-world wiring status too: see VMBackupDeps.TPMPath's doc
	// comment above for the full, precise statement of what is and is not
	// wired today. The SEPARATE, actually-live NVRAM write-back mechanism
	// (internal/api/service.go's SSH WriteFile, best-effort/non-fatal) is
	// invoked through PreDefine below, NOT through this field — PreDefine is
	// already a generic caller-supplied hook, so it needs no change here to
	// carry TPM state too once a caller (service.go, out of this task's
	// file scope) builds a closure that does so.
	TPMPath string
	// DomainXML is the captured libvirt domain XML, written to a temp file and
	// passed to virsh define so the VM reappears in the VM Manager.
	DomainXML string
	// WasAutostart is the autostart flag captured at backup time; re-applied
	// after define so the VM has the same boot-on-host-start behaviour.
	WasAutostart bool
	// StartAfter, when true, boots the VM after define (mirrors a running VM).
	StartAfter bool
	// PreDefine, when set, runs after restic restore and AFTER the old domain is
	// undefined, but BEFORE `virsh define` — used to write the captured NVRAM
	// back to the host over SSH so the VM defines with its real var store. It
	// must be best-effort (never fatal): a nil error always continues.
	PreDefine func(ctx context.Context) error
	// RepoPath is the local restic repository path for the vms domain.
	RepoPath string
	// TargetID is the run-recording target id.
	TargetID string
	// DataDir is used to write temp files (the domain XML before virsh define).
	DataDir string

	// BlockDisks are the domain's block-device-backed (zvol) disks to restore
	// via RestoreZvolDisk instead of the file-based restic restore above —
	// each restored into a FRESH dataset (see RestoreZvolDisk's doc comment;
	// the live original dataset is NEVER touched — renaming/promoting the
	// fresh one over it is a documented manual operator step, see
	// docs/vm-backup-ssh-setup.md's TrueNAS section). The caller resolves,
	// for EACH entry, the restic snapshot id that actually holds THAT disk's
	// backup — NOT necessarily SnapshotID above (see VMBackupDeps.BlockDisks's
	// doc comment for why a mixed file+block VM backup produces multiple
	// distinct restic snapshots). Empty for a VM with only file-backed disks
	// — in that case restore behavior is BYTE-IDENTICAL to before this field
	// existed.
	BlockDisks []VMRestoreBlockDisk
	// ZFSHost / ZvolRestic are RestoreZvolDisk's own dependencies — required
	// only when BlockDisks is non-empty.
	ZFSHost    ZFSHost
	ZvolRestic ZvolRestic

	VM     VM
	Restic Restic
	Runs   Runs
}

// VMRestoreBlockDisk names ONE block-device-backed VM disk for
// VMRestoreDeps.BlockDisks to restore via RestoreZvolDisk.
type VMRestoreBlockDisk struct {
	// SourceDataset is the ORIGINAL zvol dataset this disk was backed up from
	// (see RestoreZvolDeps.SourceDataset — RestoreZvolDisk never issues a
	// `zfs receive` against this value directly, see its doc comment).
	SourceDataset string
	// SnapshotID is the restic snapshot holding THIS disk's backup — resolved
	// by the caller (see VMRestoreDeps.BlockDisks's doc comment above).
	SnapshotID string
	// StdinPath is the exact path string the original BackupZvolDisk call
	// used (see ZvolStdinPath) — required to retrieve the right synthetic
	// file out of the snapshot.
	StdinPath string
}

// ---------------------------------------------------------------------------
// BackupVMGraceful
// ---------------------------------------------------------------------------

// LiveSnapshotName is the fixed name BombVault gives the temporary external
// overlay it creates for a live backup. It is exported so the service layer can
// recognise a leftover overlay (a disk whose source file contains this name,
// left by a previously interrupted live backup) and commit it back before the
// next backup.
const LiveSnapshotName = "bombvault-tmp"

// finishVMRun records the single run outcome shared by the graceful and live
// paths: failed on error, success otherwise.
func finishVMRun(d VMBackupDeps, runID string, summary Summary, backupErr error) (Summary, error) {
	if backupErr != nil {
		_ = d.Runs.Finish(runID, statusFailed, "", 0, truncateErr(backupErr))
		return Summary{}, backupErr
	}
	if err := d.Runs.Finish(runID, statusSuccess, summary.SnapshotID, summary.Bytes, ""); err != nil {
		return summary, fmt.Errorf("vm backup: record run finish: %w", err)
	}
	return summary, nil
}

// BackupVMGraceful orchestrates a graceful VM backup:
//
//	recordRunStart
//	→ IsActive (capture wasRunning)
//	→ Shutdown → poll State until "shut off" (timeout → Destroy)
//	→ restic Backup (diskPaths + nvram + tpm, tags ["vm:<name>", "p2"])
//	→ FINALLY Start (only if wasRunning — mirrors BackupContainer's always-start)
//	→ recordRunFinish(success|failed)
//	→ re-throw on failure
//
// The VM is GUARANTEED to be restarted if it was running before the backup,
// even if any intermediate step fails.
func BackupVMGraceful(ctx context.Context, d VMBackupDeps) (Summary, error) {
	runID, err := d.Runs.Start(d.TargetID, kindBackup)
	if err != nil {
		return Summary{}, fmt.Errorf("vm backup: record run start: %w", err)
	}
	summary, backupErr := runVMGraceful(ctx, d)
	return finishVMRun(d, runID, summary, backupErr)
}

// runVMGraceful performs the graceful shutdown→restic→restart sequence WITHOUT
// recording a run (the caller owns the run). The VM is guaranteed to be
// restarted if it was running before, even on any error.
func runVMGraceful(ctx context.Context, d VMBackupDeps) (Summary, error) {
	wasRunning, err := d.VM.IsActive(ctx, d.Name)
	if err != nil {
		return Summary{}, fmt.Errorf("vm backup: check active: %w", err)
	}

	var backupErr error
	var summary Summary

	func() {
		// ALWAYS restart the VM if it was running before — even on any error below.
		defer func() {
			if !wasRunning {
				return
			}
			if startErr := d.VM.Start(ctx, d.Name); startErr != nil && backupErr == nil {
				backupErr = fmt.Errorf("vm backup: restart vm: %w", startErr)
			}
		}()

		// Graceful shutdown + poll until "shut off".
		if wasRunning {
			if backupErr = d.VM.Shutdown(ctx, d.Name); backupErr != nil {
				backupErr = fmt.Errorf("vm backup: shutdown: %w", backupErr)
				return
			}
			if backupErr = waitShutOff(ctx, d.VM, d.Name, d.ShutdownTimeout); backupErr != nil {
				return
			}
		}

		// Build path list: disks + nvram + tpm (if present).
		paths := append([]string(nil), d.DiskPaths...)
		if d.NVRAMPath != "" {
			paths = append(paths, d.NVRAMPath)
		}
		if d.TPMPath != "" {
			paths = append(paths, d.TPMPath)
		}

		tags := []string{"vm:" + d.Name, "p2"}
		summary, backupErr = d.Restic.Backup(ctx, d.RepoPath, paths, tags)
		if backupErr != nil {
			backupErr = fmt.Errorf("vm backup: restic: %w", backupErr)
			return
		}

		// Block-device-backed (zvol) disks, if any, go through the SEPARATE
		// zvol mechanism (see backupBlockDisksAndLog) — file-backed disks
		// above are completely untouched by this. A no-op when d.BlockDisks
		// is empty (every VM in production today).
		if zErr := backupBlockDisksAndLog(ctx, d, "vm backup"); zErr != nil {
			backupErr = zErr
		}
	}()

	return summary, backupErr
}

// BackupVMLive backs up a RUNNING VM without shutting it down:
//
//	snapshot-create-as --disk-only --atomic (VM writes to a fresh overlay)
//	→ restic backs up the now-static base disk(s)
//	→ blockcommit --active --pivot (merge overlay back, pivot the live VM)
//
// RELIABILITY: there is NO fallback to a graceful (shutdown) backup — a VM the
// user chose to back up live is never silently shut down (that is what the
// explicit "graceful" method is for). The only self-heal here is the fsfreeze
// case: a quiesced snapshot that fails with a freeze error (guest agent present
// but its fsfreeze hook broken/blocking) is retried ONCE crash-consistent
// (without --quiesce). Any other snapshot failure — no writable disk to
// snapshot, or snapshot-create-as refusing a device — fails with a clear error
// while the VM is untouched and still RUNNING (the snapshot is --atomic, so a
// failed attempt creates nothing). Recovery from a leftover overlay of a
// previously interrupted live run lives in the SERVICE layer, which commits a
// leftover BombVault overlay back BEFORE this runs.
//
// SAFETY: on a failure AFTER the snapshot exists (restic or blockcommit) the VM
// is left RUNNING and usable — never destroyed or undefined. A blockcommit
// failure surfaces a clear, actionable error (the VM keeps running on its
// overlay; no data is lost) and we do NOT fall back (a graceful shutdown with a
// live overlay would be unsafe).
func BackupVMLive(ctx context.Context, d VMBackupDeps) (Summary, error) {
	runID, err := d.Runs.Start(d.TargetID, kindBackup)
	if err != nil {
		return Summary{}, fmt.Errorf("vm live backup: record run start: %w", err)
	}
	summary, backupErr := runVMLive(ctx, d)
	return finishVMRun(d, runID, summary, backupErr)
}

// runVMLive performs the live snapshot→restic→blockcommit sequence WITHOUT
// recording a run. It NEVER shuts the VM down: on any failure the VM is left
// running and a clear error is returned (a VM the user chose to back up live must
// not be silently shut down — that is what the explicit "graceful" method is
// for). Reliability for the common "leftover overlay" failure comes from the
// service layer committing a leftover BombVault overlay back BEFORE this runs.
// Requires d.DiskDevice (the blockcommit target).
func runVMLive(ctx context.Context, d VMBackupDeps) (Summary, error) {
	commitDevs := d.CommitDevs
	if len(commitDevs) == 0 && d.DiskDevice != "" {
		commitDevs = []string{d.DiskDevice}
	}
	if len(commitDevs) == 0 {
		return Summary{}, fmt.Errorf("vm live backup: no writable disk to snapshot/commit — use the graceful method for this VM")
	}
	quiesce := d.VM.GuestAgentPing(ctx, d.Name)

	// Create the overlay(s) (writable disks only; cdrom/read-only excluded). The
	// snapshot is --atomic, so on failure nothing was created and the VM is
	// untouched and still running.
	if snapErr := d.VM.SnapshotCreateDiskOnly(ctx, d.Name, LiveSnapshotName, quiesce, d.SkipSnapshotDevs); snapErr != nil {
		// A guest with the agent present but a broken/blocking fsfreeze hook (e.g.
		// Home Assistant during startup) fails a quiesced snapshot. Retry once
		// crash-consistent (no --quiesce) instead of failing the whole backup; a
		// non-freeze error (or an already-unquiesced attempt) still fails clearly.
		if quiesce && isFreezeErr(snapErr) {
			log.Printf("schedule/backup: vm %q quiesced snapshot failed (%v); retrying crash-consistent without --quiesce", d.Name, snapErr)
			if snapErr2 := d.VM.SnapshotCreateDiskOnly(ctx, d.Name, LiveSnapshotName, false, d.SkipSnapshotDevs); snapErr2 != nil {
				return Summary{}, fmt.Errorf("vm live backup: snapshot (after fsfreeze fallback): %w", snapErr2)
			}
		} else {
			return Summary{}, fmt.Errorf("vm live backup: snapshot: %w", snapErr)
		}
	}

	// Back up the now-static base disk(s).
	paths := append([]string(nil), d.DiskPaths...)
	if d.NVRAMPath != "" {
		paths = append(paths, d.NVRAMPath)
	}
	if d.TPMPath != "" {
		paths = append(paths, d.TPMPath)
	}
	tags := []string{"vm:" + d.Name, "p2", "live"}
	summary, backupErr := d.Restic.Backup(ctx, d.RepoPath, paths, tags)

	// ALWAYS commit EVERY overlay back, even if the backup failed, so no disk keeps
	// diverging on an uncommitted overlay. Attempt all devices; report the first
	// failure (the VM keeps running on its overlay either way — no data lost).
	var commitErr error
	for _, dev := range commitDevs {
		if cErr := d.VM.BlockCommitActivePivot(ctx, d.Name, dev); cErr != nil && commitErr == nil {
			commitErr = cErr
		}
	}
	if commitErr != nil {
		return Summary{}, fmt.Errorf("vm live backup: blockcommit failed — the VM is STILL RUNNING on its snapshot overlay (no data lost); resolve the overlay before the next backup: %w", commitErr)
	}
	if backupErr != nil {
		return Summary{}, fmt.Errorf("vm live backup: restic: %w", backupErr)
	}

	// Block-device-backed (zvol) disks, if any, are entirely separate from
	// the qemu external-snapshot/blockcommit dance above (ParseDomain already
	// excludes them via SkipSnapshotDevs — they can never be a
	// SnapshotCreateDiskOnly/BlockCommitActivePivot target) — see
	// backupBlockDisksAndLog. A no-op when d.BlockDisks is empty (every VM in
	// production today).
	if zErr := backupBlockDisksAndLog(ctx, d, "vm live backup"); zErr != nil {
		return Summary{}, zErr
	}
	return summary, nil
}

// waitShutOff polls the VM state until it reaches "shut off". On timeout it
// calls Destroy (force off) and returns nil (the VM is now off either way).
// If maxPolls is 0, uses defaultVMShutdownMaxPolls.
func waitShutOff(ctx context.Context, vm VM, name string, maxPolls int) error {
	if maxPolls <= 0 {
		maxPolls = defaultVMShutdownMaxPolls
	}
	for i := 0; i < maxPolls; i++ {
		state, err := vm.State(ctx, name)
		if err != nil {
			return fmt.Errorf("vm backup: poll state: %w", err)
		}
		if state == "shut off" {
			return nil
		}
		// Sleep between polls, but not on the last one (avoid unnecessary delay
		// before the timeout/destroy path).
		if i < maxPolls-1 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(defaultVMShutdownPollInterval):
			}
		}
	}
	// Timeout reached: force the VM off.
	log.Printf("vm backup: graceful shutdown timed out for %q; forcing destroy", name)
	if err := vm.Destroy(ctx, name); err != nil {
		return fmt.Errorf("vm backup: force destroy after timeout: %w", err)
	}
	return nil
}

// ---------------------------------------------------------------------------
// RestoreVM
// ---------------------------------------------------------------------------

// RestoreVM orchestrates a VM restore:
//
//	guard Confirmed + validate snapshotID (hex) + validate paths
//	→ recordRunStart
//	→ if VM exists: Destroy (if running) + Undefine
//	→ restic RestorePaths (diskPaths + nvram + tpm, per-path back to origin)
//	→ write DomainXML to DataDir/vm-define/<name>.xml → Define
//	→ Autostart(wasAutostart) → Start (if StartAfter)
//	→ recordRunFinish(success|failed)
//
// Returns an error WITHOUT recording a run when not confirmed or the snapshot
// id is invalid (nothing destructive has happened yet).
func RestoreVM(ctx context.Context, d VMRestoreDeps) error {
	if !d.Confirmed {
		return ErrNotConfirmed
	}
	if !snapshotIDRe.MatchString(d.SnapshotID) {
		return ErrInvalidSnapshotID
	}

	runID, err := d.Runs.Start(d.TargetID, kindRestore)
	if err != nil {
		return fmt.Errorf("vm restore: record run start: %w", err)
	}

	restoreErr := runVMRestore(ctx, d)
	if restoreErr != nil {
		_ = d.Runs.Finish(runID, restoreOutcome(restoreErr), "", 0, truncateErr(restoreErr))
		return restoreErr
	}
	if err := d.Runs.Finish(runID, statusSuccess, d.SnapshotID, 0, ""); err != nil {
		return fmt.Errorf("vm restore: record run finish: %w", err)
	}
	return nil
}

func runVMRestore(ctx context.Context, d VMRestoreDeps) error {
	// Validate: every path must be absolute and traversal-free (SEC parity with
	// container restore — same pattern as runRestore in orchestrator.go).
	allPaths := append([]string(nil), d.DiskPaths...)
	if d.NVRAMPath != "" {
		allPaths = append(allPaths, d.NVRAMPath)
	}
	if d.TPMPath != "" {
		allPaths = append(allPaths, d.TPMPath)
	}
	if len(allPaths) == 0 {
		return fmt.Errorf("vm restore: no paths to restore (unsafe)")
	}
	for _, p := range allPaths {
		if !strings.HasPrefix(p, "/") || strings.Contains(p, "..") {
			return fmt.Errorf("vm restore: unsafe path %q (unsafe)", p)
		}
	}

	// PRE-FLIGHT: confirm the snapshot is restorable (exists + repo readable)
	// BEFORE destroying/undefining the live VM, so a missing snapshot or an
	// unreadable repo can never leave the VM gone with nothing restored.
	if err := d.Restic.VerifySnapshot(ctx, d.RepoPath, d.SnapshotID); err != nil {
		return fmt.Errorf("vm restore: snapshot preflight: %w", err)
	}

	// If the VM currently exists, destroy (if running) then undefine it.
	state, err := d.VM.State(ctx, d.Name)
	if err != nil {
		return fmt.Errorf("vm restore: check state: %w", err)
	}
	if state != "" {
		// VM exists on the host.
		if state == "running" {
			if err := d.VM.Destroy(ctx, d.Name); err != nil {
				return fmt.Errorf("vm restore: destroy running vm: %w", err)
			}
		}
		if err := d.VM.Undefine(ctx, d.Name); err != nil {
			return fmt.Errorf("vm restore: undefine: %w", err)
		}
	}

	// VM disk images, NVRAM, and TPM state are all FILES; restic's
	// <id>:<subpath> subtree form needs a DIRECTORY (a file path fails with
	// "not a directory").
	if len(d.RestoreDirs) > 0 {
		// REMAPPED restore (cross-instance): each source subtree is restored INTO a
		// chosen destination dir, so the disks land on the destination host's pool
		// rather than the source server's original paths.
		for _, rd := range d.RestoreDirs {
			if err := d.Restic.RestoreSubtreeTo(ctx, d.RepoPath, d.SnapshotID, rd.Subtree, rd.Target); err != nil {
				return fmt.Errorf("vm restore: restic restore: %w", err)
			}
		}
	} else {
		// Same-instance restore: restore each file's PARENT directory back to its own
		// location (deduplicated). restic restores only the snapshot's files in that
		// dir and never deletes existing siblings.
		restoreDirs := parentDirs(allPaths)
		if len(restoreDirs) == 0 {
			return fmt.Errorf("vm restore: no restorable directories derived from paths")
		}
		if err := d.Restic.RestorePaths(ctx, d.RepoPath, d.SnapshotID, restoreDirs); err != nil {
			return fmt.Errorf("vm restore: restic restore: %w", err)
		}
	}

	// Block-device-backed (zvol) disks, if any, go through the SEPARATE
	// RestoreZvolDisk mechanism (see restoreBlockDisksAndLog) — restored into
	// a FRESH dataset, never overwriting the live original. Abort BEFORE
	// defining the domain below if any failed (a VM with an incomplete/
	// missing disk restore must not be defined/started). A no-op when
	// d.BlockDisks is empty (every VM in production today).
	if err := restoreBlockDisksAndLog(ctx, d); err != nil {
		return err
	}

	// Write the captured NVRAM back to the host (over SSH) now that the old
	// domain is undefined (its nvram removed) and before define, so libvirt picks
	// up the real var store. Best-effort — never blocks the restore.
	if d.PreDefine != nil {
		if err := d.PreDefine(ctx); err != nil {
			return fmt.Errorf("vm restore: pre-define: %w", err)
		}
	}

	// Write domain XML to a temp file then define it with virsh.
	xmlDir := filepath.Join(d.DataDir, "vm-define")
	if err := os.MkdirAll(xmlDir, 0o700); err != nil {
		return fmt.Errorf("vm restore: create vm-define dir: %w", err)
	}
	xmlPath := filepath.Join(xmlDir, d.Name+".xml")
	if err := os.WriteFile(xmlPath, []byte(d.DomainXML), 0o600); err != nil { //nolint:gosec // G306: 0600 is intentional (domain XML may contain sensitive paths)
		return fmt.Errorf("vm restore: write domain xml: %w", err)
	}
	if err := d.VM.Define(ctx, xmlPath); err != nil {
		return fmt.Errorf("vm restore: define: %w", err)
	}

	// Restore the autostart flag captured at backup time.
	if err := d.VM.Autostart(ctx, d.Name, d.WasAutostart); err != nil {
		return fmt.Errorf("vm restore: autostart: %w", err)
	}

	// Optionally boot the VM (e.g. it was running before).
	if d.StartAfter {
		if err := d.VM.Start(ctx, d.Name); err != nil {
			return fmt.Errorf("vm restore: start: %w", err)
		}
	}
	return nil
}

// isFreezeErr reports whether a snapshot error is a guest-agent freeze failure
// (the fsfreeze hook blocked or failed), so a quiesced snapshot can be retried
// crash-consistent (without --quiesce) rather than failing the whole backup.
func isFreezeErr(err error) bool {
	if err == nil {
		return false
	}
	m := strings.ToLower(err.Error())
	return strings.Contains(m, "fsfreeze") ||
		strings.Contains(m, "freeze") ||
		strings.Contains(m, "guest agent") ||
		strings.Contains(m, "guest-agent") ||
		strings.Contains(m, "quiesce")
}

// ---------------------------------------------------------------------------
// Zvol-aware VM disk backup/restore (v8.0.0 TrueNAS platform expansion,
// Task 10)
//
// ⚠ UNVERIFIED AGAINST REAL HARDWARE. Everything below is REASONED from ZFS's
// public documentation — `zfs snapshot`/`zfs send`/`zfs receive` as the
// ZFS-native way to move a stable point-in-time dataset byte stream off/onto
// a zvol — and exercised only with fakes (vm_zvol_test.go and
// vm_zvol_wiring_test.go); no TrueNAS Scale test instance was available
// anywhere in this project's development environment to run it against a
// real zvol over a real SSH connection. See internal/virshcli/zvol.go's
// package doc comment for the full caveat, which applies equally here.
//
// WIRED IN (Task 10 Step 5): BackupZvolDisk/RestoreZvolDisk below ARE called
// from BackupVMGraceful/BackupVMLive/RestoreVM above, via the
// VMBackupDeps.BlockDisks / VMRestoreDeps.BlockDisks fields and the
// backupBlockDisksAndLog/restoreBlockDisksAndLog helpers — when a domain's
// disk is block-device-backed, it goes through the
// snapshot→stream→restic-backup→destroy-snapshot path here INSTEAD of the
// file-copy path, exactly as planned. File-backed (Unraid) VM disk
// backup/restore is completely UNTOUCHED: BlockDisks defaults to nil (no
// caller populates it today), so this is a dormant, zero-behavior-change
// no-op for every VM in production — proven by the regression tests in
// vm_zvol_wiring_test.go that pin file-only backup/restore as byte-identical
// to before this wiring existed.
//
// ⚠ NOT YET DONE — a real, code-verified gap, not a hand-wave: nothing calls
// this from a live backup/restore today. internal/api/service.go (the actual
// caller BackupVM/RestoreVM's HTTP handlers reach) never reads
// virshcli.DomainInfo.BlockDisks and never populates VMBackupDeps.BlockDisks/
// VMRestoreDeps.BlockDisks/ZFSHost/ZvolRestic — that requires a real
// SSH-backed ZFSHost/ZvolRestic adapter (none exists yet) and resolving each
// BlockDisks entry's Dataset (virshcli.zvolDatasetFromDevPath, already
// unit-tested but currently called nowhere outside its own test). More
// fundamentally: restic's `--stdin` backup mode cannot be combined with a
// normal path-list `restic backup` in one invocation (see
// internal/restic/restic.go's BackupStdinArgs/BackupArgs), so a VM with BOTH
// file-backed and block-device disks necessarily produces MULTIPLE distinct
// restic snapshots per backup — one for the file disks, one PER block disk.
// This package's run-recording model (the Runs interface's
// Finish(runID, status, snapshotID string, ...) — exactly one snapshot id per
// run) and the service layer's restore resolution (prepareRestoreVMForTarget
// lists snapshots by the single tag "vm:<name>" and picks the newest as
// "latest") both assume ONE snapshot represents a VM's backup; tagging a
// block disk's separate snapshot with that same "vm:<name>" tag would make
// "latest" resolve ambiguously between the file snapshot and a block disk's
// snapshot. Solving this (a disk-unique tag + its own lookup, or persisting
// per-disk snapshot ids in the VM target definition, or something else) is a
// real design decision for whoever does the service-layer integration — it
// is OUT OF SCOPE here and intentionally NOT decided by this file; see Task
// 10 in docs/superpowers/plans/2026-08-16-bombvault-platform-expansion.md
// and docs/vm-backup-ssh-setup.md's TrueNAS section for the operator-facing
// version of this same status.
// ---------------------------------------------------------------------------

// ZFSHost is the host-control surface the zvol backup/restore orchestrators
// need — the SSH-transported ZFS snapshot/send/receive steps. Semantic (not
// raw argv), mirroring how VM above abstracts virsh commands rather than
// exposing raw CLI argv, and interface-shaped so this file is unit-testable
// with fakes, without a real SSH connection or ZFS system. The concrete
// adapter (internal/sshconn.Conn's Run/StreamCommand/RunWithStdin methods +
// internal/virshcli's ZFSSnapshotArgs/ZFSSnapshotDestroyArgs/ZFSSendArgs/
// ZFSReceiveArgs argv builders) is wired at the service layer — this package
// never imports either concrete package (see the package doc comment at the
// top of orchestrator.go: "imports ONLY the interfaces defined here... never
// the concrete dockercli/restic packages").
type ZFSHost interface {
	// SnapshotCreate runs `zfs snapshot <dataset>@<snapName>` on the host.
	SnapshotCreate(ctx context.Context, dataset, snapName string) error
	// SnapshotDestroy runs `zfs destroy <dataset>@<snapName>` on the host —
	// cleanup for the snapshot SnapshotCreate took. The snapshot is a live
	// consistency point for the backup, not the backup artifact itself.
	SnapshotDestroy(ctx context.Context, dataset, snapName string) error
	// StreamSend starts `zfs send <dataset>@<snapName>` on the host and
	// returns its stdout as a stream, plus a wait function the caller MUST
	// call exactly once after it is done reading (success or failure) to
	// reap the process and surface any failure (a short/truncated stream can
	// otherwise look like a clean backup — see BackupZvolDisk).
	StreamSend(ctx context.Context, dataset, snapName string) (io.ReadCloser, func() error, error)
	// StreamReceive runs `zfs receive <targetDataset>` on the host with its
	// stdin fed from rd, streamed (never buffered in memory — a zvol can be
	// many gigabytes).
	StreamReceive(ctx context.Context, rd io.Reader, targetDataset string) error
}

// ZvolRestic is the restic surface the zvol orchestrators need: streaming a
// backup FROM an io.Reader with no local file (internal/restic.Restic.
// BackupStdin) and streaming a restore snapshot TO an io.Writer
// (internal/restic.Restic.DumpRaw). Kept separate from this package's main
// Restic interface above (which every OTHER orchestrator in this file also
// implements) since file-backed disk backup/restore never needs stdin
// streaming — adding it to the shared interface would widen every existing
// fake/adapter for a capability only the zvol path uses.
type ZvolRestic interface {
	// BackupStdin backs up the ENTIRE content of rd as a single synthetic
	// file recorded under path, tagged with tags, and returns the parsed
	// summary.
	BackupStdin(ctx context.Context, repo string, rd io.Reader, path string, tags []string) (Summary, error)
	// DumpTo streams the synthetic file at path, from the given snapshot,
	// into w — raw bytes, byte-identical to what BackupStdin was given.
	DumpTo(ctx context.Context, repo, snapshotID, path string, w io.Writer) error
}

// ZvolStdinPath is the fixed, deterministic convention BackupZvolDisk uses as
// restic's stdin-filename (and RestoreZvolDeps.StdinPath must match) for one
// disk's backup, derived from the dataset + snapshot name so it never needs
// separate bookkeeping beyond what the caller already has to track anyway
// (which snapshot backed up which disk). Exported so a future caller (the
// service-layer VM target definition, once it persists dataset/snapName
// metadata alongside a zvol backup) can recompute the same path for a
// restore.
func ZvolStdinPath(dataset, snapName string) string {
	return "/vm-disks/" + dataset + "@" + snapName
}

// BackupZvolDeps bundles everything BackupZvolDisk needs for ONE
// block-device-backed VM disk. A VM with multiple such disks calls
// BackupZvolDisk once per disk — restic's --stdin backs up exactly one
// synthetic file per invocation (internal/restic.BackupStdinArgs), so there
// is no multi-disk variant of this function the way VMBackupDeps.DiskPaths
// batches multiple file-backed disks into one restic call.
type BackupZvolDeps struct {
	// Name is the libvirt domain name (used to build the restic tag).
	Name string
	// Dataset is the zvol's ZFS dataset, "<pool>/<dataset...>" (from
	// virshcli.zvolDatasetFromDevPath, applied to the disk's block-device
	// source path).
	Dataset string
	// SnapName is the ZFS snapshot name to create (e.g. from
	// virshcli.ZvolSnapshotName) — caller-chosen so it is loggable/known
	// before this function runs.
	SnapName string
	// RepoPath is the local restic repository path for the vms domain.
	RepoPath string
	// Tags are the restic tags for this disk's backup (e.g. ["vm:<name>"]).
	Tags []string

	Host   ZFSHost
	Restic ZvolRestic
}

// BackupZvolDisk backs up ONE block-device-backed VM disk:
//
//	zfs snapshot <dataset>@<snapName>
//	→ zfs send <dataset>@<snapName>, streamed over SSH straight into
//	  restic backup --stdin (no local staging file, no local ZFS/zvol access
//	  needed inside the container — mirrors how virshcli already shells
//	  virsh commands to the host over SSH instead of requiring libvirt
//	  locally)
//	→ ALWAYS zfs destroy <dataset>@<snapName> (deferred — a live consistency
//	  point for the send, not the backup artifact; cleaned up on EVERY path,
//	  success or failure — mirroring BackupVMLive's "always commit every
//	  overlay back" guarantee above)
//
// A snapshot-destroy failure is logged, not returned: the data already
// safely reached the restic repo (or didn't, in which case that IS the
// returned error) either way; a leftover snapshot is host-hygiene, not data
// loss — the operator can `zfs destroy` it manually.
//
// ⚠ UNVERIFIED AGAINST REAL HARDWARE — see this section's header comment.
func BackupZvolDisk(ctx context.Context, d BackupZvolDeps) (Summary, error) {
	if err := d.Host.SnapshotCreate(ctx, d.Dataset, d.SnapName); err != nil {
		return Summary{}, fmt.Errorf("zvol backup: snapshot create: %w", err)
	}
	// ALWAYS destroy the snapshot afterward, success or failure.
	defer func() {
		if destroyErr := d.Host.SnapshotDestroy(ctx, d.Dataset, d.SnapName); destroyErr != nil {
			log.Printf("backup: zvol %q: WARN snapshot destroy failed (%v) — a leftover bombvault ZFS snapshot may need manual cleanup", d.Dataset, destroyErr)
		}
	}()

	stream, wait, err := d.Host.StreamSend(ctx, d.Dataset, d.SnapName)
	if err != nil {
		return Summary{}, fmt.Errorf("zvol backup: start zfs send: %w", err)
	}
	defer func() { _ = stream.Close() }()

	path := ZvolStdinPath(d.Dataset, d.SnapName)
	sum, backupErr := d.Restic.BackupStdin(ctx, d.RepoPath, stream, path, d.Tags)

	// Reap the SSH/zfs-send process and surface ITS failure too — restic can
	// otherwise report a clean-looking (short) backup if the remote send died
	// mid-stream and the pipe simply closed; the wait error catches that.
	if waitErr := wait(); waitErr != nil {
		if backupErr != nil {
			return Summary{}, fmt.Errorf("zvol backup: restic: %w (zfs send also failed: %v)", backupErr, waitErr)
		}
		return Summary{}, fmt.Errorf("zvol backup: zfs send: %w", waitErr)
	}
	if backupErr != nil {
		return Summary{}, fmt.Errorf("zvol backup: restic: %w", backupErr)
	}
	return sum, nil
}

// zvolBackupSnapshotPrefix mirrors internal/virshcli.zvolSnapshotPrefix's
// IDENTICAL contract — independently defined (not shared code) for the same
// reason zvolRestoreTargetDataset below independently mirrors
// internal/virshcli.RestoreZvolTargetDataset: this package deliberately never
// imports the concrete virshcli adapter package (see this section's header
// comment above).
const zvolBackupSnapshotPrefix = "bombvault-"

// zvolBackupSnapshotName returns the ZFS snapshot name
// backupBlockDisksAndLog uses for a block-device disk's BackupZvolDisk call,
// taken at instant now — mirrors internal/virshcli.ZvolSnapshotName's
// IDENTICAL contract (see zvolBackupSnapshotPrefix's doc comment for why this
// package independently defines it). Pure function of its input (same now →
// same name), so a test can reason about it deterministically.
func zvolBackupSnapshotName(now time.Time) string {
	return zvolBackupSnapshotPrefix + now.UTC().Format("20060102150405")
}

// backupBlockDisksAndLog runs BackupZvolDisk for every entry in
// d.BlockDisks, shared by runVMGraceful and runVMLive (the file-backed
// d.Restic.Backup call in each stays completely untouched either way — this
// is purely additive and a no-op when d.BlockDisks is empty). All disk
// datasets share one snapshot name (taken once, at the start of this call) —
// distinct ZFS datasets never collide on the same snapshot name.
//
// Every entry is attempted even after an earlier one fails (mirrors this
// file's "commit every overlay" pattern in runVMLive above: a later disk's
// data should still reach the backup repo even if an earlier disk's did
// not) — the FIRST failure is returned to the caller, which fails the whole
// VM backup run. A successful disk's outcome (dataset, restic snapshot id,
// bytes) is only LOGGED: nothing yet persists it anywhere a later restore
// can find it — see VMBackupDeps.BlockDisks's doc comment for that known,
// intentionally-unsolved gap.
//
// logPrefix lets each caller's log/error lines carry ITS OWN prefix ("vm
// backup" for runVMGraceful, "vm live backup" for runVMLive) rather than a
// hardcoded one, matching every other error this file returns from either
// method — an operator triaging a live-backup failure should never see a
// "vm backup:"-prefixed line and wonder whether the graceful path ran.
func backupBlockDisksAndLog(ctx context.Context, d VMBackupDeps, logPrefix string) error {
	if len(d.BlockDisks) == 0 {
		return nil
	}
	snapName := zvolBackupSnapshotName(time.Now())
	tags := []string{"vm:" + d.Name, "p2"}
	var firstErr error
	for _, bd := range d.BlockDisks {
		sum, err := BackupZvolDisk(ctx, BackupZvolDeps{
			Name:     d.Name,
			Dataset:  bd.Dataset,
			SnapName: snapName,
			RepoPath: d.RepoPath,
			Tags:     tags,
			Host:     d.ZFSHost,
			Restic:   d.ZvolRestic,
		})
		if err != nil {
			log.Printf("%s: zvol disk %q: FAILED (%v)", logPrefix, bd.Dataset, err)
			if firstErr == nil {
				firstErr = fmt.Errorf("%s: zvol disk %q: %w", logPrefix, bd.Dataset, err)
			}
			continue
		}
		log.Printf("%s: zvol disk %q: backed up as restic snapshot %s (%d bytes) — NOT YET tracked by run-recording, see VMBackupDeps.BlockDisks's doc comment", logPrefix, bd.Dataset, sum.SnapshotID, sum.Bytes)
	}
	return firstErr
}

// zvolRestoreSuffix marks a dataset zvolRestoreTargetDataset created — never
// an operator's own naming, so it is unambiguous which datasets are BombVault
// restore landing zones awaiting the operator's manual rename/promote step.
const zvolRestoreSuffix = "-bombvault-restore-"

// zvolRestoreTargetDataset returns a freshly-named target dataset for a zvol
// restore's `zfs receive`, derived from the source dataset and the current
// instant: "<dataset>-bombvault-restore-<unix-nanoseconds>".
//
// SAFETY PROPERTY (structural, not just documented): the returned name is
// NEVER equal to dataset — it always carries a non-empty suffix — and
// RestoreZvolDisk is structured so the SOURCE dataset can never reach
// ZFSHost.StreamReceive directly, only this function's output does (see
// RestoreZvolDisk below; TestRestoreZvolDiskNeverTargetsSourceDataset proves
// this by inspecting what the fake host actually received).
//
// This mirrors internal/virshcli.RestoreZvolTargetDataset's IDENTICAL
// contract; the two are independently defined (not shared code) because this
// package deliberately never imports the concrete virshcli adapter package
// (see this section's header comment) — each is independently unit-tested in
// its own package, and a future caller building RestoreZvolDeps can use
// either (virshcli's version if it needs the name before calling this
// orchestrator, e.g. to show the operator where the restore will land).
func zvolRestoreTargetDataset(dataset string, now time.Time) string {
	return fmt.Sprintf("%s%s%d", dataset, zvolRestoreSuffix, now.UnixNano())
}

// RestoreZvolDeps bundles everything RestoreZvolDisk needs to restore ONE
// block-device-backed VM disk's backup.
type RestoreZvolDeps struct {
	// SourceDataset is the ORIGINAL zvol dataset the disk was backed up from.
	// RestoreZvolDisk NEVER issues a `zfs receive` against this value
	// directly — see zvolRestoreTargetDataset's safety property.
	SourceDataset string
	// RepoPath is the local restic repository path for the vms domain.
	RepoPath string
	// SnapshotID is the restic snapshot to restore from.
	SnapshotID string
	// StdinPath is the exact path string the original BackupZvolDisk call
	// used (see ZvolStdinPath) — required to retrieve the right synthetic
	// file out of the snapshot.
	StdinPath string

	Host   ZFSHost
	Restic ZvolRestic
}

// RestoreZvolDisk restores ONE block-device-backed VM disk's backup into a
// FRESH, never-before-existing dataset — NEVER the original source dataset:
//
//	restic dump <snapshotID> <stdinPath>, streamed straight into
//	→ zfs receive <freshly-named target dataset>
//
// This is a deliberately NEW, SEPARATE path from RestoreVM above (which
// restores FILE-backed disks back to their own original location) — reusing
// that path here would risk exactly what this function is built to prevent:
// `zfs receive` into an EXISTING dataset can destroy live data, so every
// restore lands on a fresh dataset via zvolRestoreTargetDataset, structurally
// (not just by convention — see that function's doc comment).
//
// Returns the fresh target dataset's name. Renaming/promoting it over the
// live original zvol is a deliberate, DOCUMENTED MANUAL follow-up step for
// the operator (see docs/vm-backup-ssh-setup.md's TrueNAS section) — this
// function never automates that step.
//
// ⚠ UNVERIFIED AGAINST REAL HARDWARE — see this section's header comment.
func RestoreZvolDisk(ctx context.Context, d RestoreZvolDeps) (string, error) {
	target := zvolRestoreTargetDataset(d.SourceDataset, time.Now())

	// Bridge Restic.DumpTo (writes to an io.Writer) into Host.StreamReceive
	// (reads from an io.Reader) via an in-process pipe — never buffers the
	// whole disk image in memory.
	pr, pw := io.Pipe()
	dumpDone := make(chan error, 1)
	go func() {
		dumpErr := d.Restic.DumpTo(ctx, d.RepoPath, d.SnapshotID, d.StdinPath, pw)
		_ = pw.CloseWithError(dumpErr) // CloseWithError(nil) behaves like Close() (clean EOF)
		dumpDone <- dumpErr
	}()

	recvErr := d.Host.StreamReceive(ctx, pr, target)
	// Unblock a pending/future Write on pw if StreamReceive returned WITHOUT
	// fully draining pr (e.g. an early SSH failure before any data was
	// accepted) — otherwise the goroutine above could block forever on
	// pw.Write and dumpDone would never receive.
	_ = pr.CloseWithError(recvErr)
	dumpErr := <-dumpDone

	if dumpErr != nil {
		return "", fmt.Errorf("zvol restore: restic dump: %w", dumpErr)
	}
	if recvErr != nil {
		return "", fmt.Errorf("zvol restore: zfs receive: %w", recvErr)
	}
	return target, nil
}

// restoreBlockDisksAndLog runs RestoreZvolDisk for every entry in
// d.BlockDisks, called from runVMRestore above (the file-based restic
// restore above stays completely untouched either way — this is purely
// additive and a no-op when d.BlockDisks is empty). Each disk restores into
// its OWN fresh dataset (RestoreZvolDisk never touches the live original —
// see its doc comment).
//
// Every entry is attempted even after an earlier one fails (same "attempt
// everything" reasoning as backupBlockDisksAndLog above) — the FIRST failure
// is returned, and the caller (runVMRestore) aborts BEFORE defining the
// domain when this returns non-nil (defining/starting a VM whose disk
// restore is incomplete is unsafe). A successful restore is only LOGGED with
// its fresh target dataset name — renaming/promoting it over the live
// original is a documented MANUAL operator follow-up (see RestoreZvolDisk's
// doc comment and docs/vm-backup-ssh-setup.md's TrueNAS section), never
// automated here.
func restoreBlockDisksAndLog(ctx context.Context, d VMRestoreDeps) error {
	var firstErr error
	for _, bd := range d.BlockDisks {
		target, err := RestoreZvolDisk(ctx, RestoreZvolDeps{
			SourceDataset: bd.SourceDataset,
			RepoPath:      d.RepoPath,
			SnapshotID:    bd.SnapshotID,
			StdinPath:     bd.StdinPath,
			Host:          d.ZFSHost,
			Restic:        d.ZvolRestic,
		})
		if err != nil {
			log.Printf("vm restore: zvol disk %q: FAILED (%v)", bd.SourceDataset, err)
			if firstErr == nil {
				firstErr = fmt.Errorf("vm restore: zvol disk %q: %w", bd.SourceDataset, err)
			}
			continue
		}
		log.Printf("vm restore: zvol disk %q: restored into fresh dataset %q — rename/promote it over the original manually (see docs/vm-backup-ssh-setup.md)", bd.SourceDataset, target)
	}
	return firstErr
}
