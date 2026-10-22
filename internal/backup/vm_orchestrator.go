// Package backup — VM orchestrators for graceful-shutdown backup and restore.
// This file mirrors orchestrator.go's patterns: DI interfaces, ALWAYS-restart
// guard via defer, confirmation + path validation guards.
package backup

import (
	"context"
	"fmt"
	"log"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/progress"
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

// withRunTag returns tags with runTag appended in a fresh slice, so the
// caller's backing array is never shared. An empty runTag returns tags as is.
func withRunTag(tags []string, runTag string) []string {
	if runTag == "" {
		return tags
	}
	out := make([]string, len(tags), len(tags)+1)
	copy(out, tags)
	return append(out, runTag)
}

// withFormerNames gives each former name a tag of its own, as BackupContainer
// does, so a reader matches a snapshot on any one of them.
func withFormerNames(tags, formerNames []string) []string {
	for _, n := range formerNames {
		tags = append(tags, "formerly:"+n)
	}
	return tags
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
	// FormerNames are the entry's former names, each written as a
	// "formerly:<name>" tag on the file-backed snapshot so the link survives in
	// the repository. The zvol snapshots go without: a VM with zvol disks
	// cannot be taken over.
	FormerNames []string
	// RunTag, when non-empty, is a per-run correlation tag (e.g.
	// "vmrun:<runID>") added to every restic backup this struct drives: the
	// file-backed disks and each BlockDisks entry. restic's --stdin mode splits
	// a mixed file and zvol backup into several snapshots, and the tag ties
	// them back to one run.
	RunTag string
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
	// Step is called with progress.StepStop, StepRead and StepStart as a
	// graceful backup of a running VM reaches each of them.
	Step func(step string)

	// BlockDisks are the domain's zvol disks. restic cannot read a block
	// device by path, so each goes in as a raw image of its own, a snapshot
	// tagged vm:<name>:zvol:<dev> beside the run's snapshot under vm:<name>.
	BlockDisks []VMBlockDisk
	// ZFSHost and ZvolRestic are needed only when there are BlockDisks.
	ZFSHost    ZFSHost
	ZvolRestic ZvolRestic
	// DomainXML is the domain definition. A VM whose disks are all zvols has
	// no file snapshot, so its run stores this under the VM's own tags
	// instead: the snapshot every listing, restore and retention of the VM
	// finds the run by.
	DomainXML string
	// Restarts remembers a VM the backup shuts down, so a process that dies
	// before starting it again does it at its next start. Nil remembers
	// nothing.
	Restarts RestartMarker

	VM     VM
	Restic Restic
	Runs   Runs
}

// VMBlockDisk is one zvol disk of a VM to back up.
type VMBlockDisk struct {
	// Dataset is the zvol, "<pool>/<path>".
	Dataset string
	// Dev is the disk's target device, such as "vdb". It gives the disk's
	// snapshots their own tag, vm:<name>:zvol:<dev>, so each disk ages under
	// retention as a series of its own. Without it the disk shares vm:<name>.
	Dev string
}

// VMRestoreDir pairs a snapshot subtree (Subtree, a dir the backup recorded)
// with the destination dir (Target) its contents are restored into: a chosen
// pool for a cross-instance restore, or the folder a rename moved the disks to.
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
	// RunTag mirrors VMBackupDeps.RunTag's field for API symmetry (a caller
	// building both structs for one VM can set it in one place conceptually)
	// and so a future restore-side use has an obvious place to live. UNLIKE
	// the backup side, it drives NO restic call in this file today — verified
	// by reading the real signatures, not assumed: every restic method the
	// restore path calls takes a snapshot id, never tags
	// (Restic.VerifySnapshot/RestorePaths/RestoreSubtreeTo in
	// orchestrator.go, ZvolRestic.DumpTo below) — restic tags are written
	// only at backup time, and a restore reads an already-tagged snapshot by
	// id, so there is nothing here for a tag to attach to. Set or left empty,
	// restore behavior — including a multi-disk BlockDisks restore via
	// restoreBlockDisksAndLog — is IDENTICAL either way.
	RunTag string
	// SnapshotID is the restic snapshot to restore (validated hex).
	SnapshotID string
	// DiskPaths are the absolute container-visible paths the restored disks end
	// up at, checked for safety either way. Without RestoreDirs they are the
	// snapshot's own paths and their folders are restored in place.
	DiskPaths []string
	// RestoreDirs, when non-empty, restores each snapshot subtree into its
	// Target instead. A cross-instance restore uses it to land the disks on the
	// chosen pool, and a snapshot from before a rename to reach the moved folder.
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

	// BlockDisks are the zvol disks, each written back from its own snapshot.
	BlockDisks []VMRestoreBlockDisk
	// ZFSHost and ZvolRestic are needed only when there are BlockDisks.
	ZFSHost    ZFSHost
	ZvolRestic ZvolRestic

	// Images, when set, are the disks of a changed-block snapshot, rebuilt
	// from their segments in place of the restic restore of the disk folders.
	// DiskPaths then lists their targets for the path checks.
	Images      []VMRestoreImage
	ImageDumper BlockDumper
	Convert     ImageConverter

	VM     VM
	Restic Restic
	Runs   Runs
}

// VMRestoreBlockDisk is one zvol disk of a VM to restore.
type VMRestoreBlockDisk struct {
	// Dataset is the zvol written to: the one backed up, or its counterpart on
	// another pool for a restore onto another instance.
	Dataset string
	// Size is the zvol's size in bytes when it was backed up.
	Size int64
	// SnapshotID and StdinPath name the disk's image in the repository.
	SnapshotID string
	StdinPath  string
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
		_ = d.Runs.Finish(runID, statusFailed, Summary{}, truncateErr(backupErr))
		return Summary{}, backupErr
	}
	if err := d.Runs.Finish(runID, statusSuccess, summary, ""); err != nil {
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

	// Not ctx: a cancelled or timed-out backup ends ctx, and a start on a done
	// context is refused at once, which left the VM shut off.
	restartCtx := context.WithoutCancel(ctx)
	var shuttingDown bool

	func() {
		// ALWAYS restart the VM if it was running before — even on any error below.
		defer func() {
			if !wasRunning {
				return
			}
			atStep(d.Step, progress.StepStart)
			// The guest goes on shutting down after a cancel. Starting it while
			// it is still up does nothing, and it would then stay off.
			if shuttingDown {
				if err := waitShutOff(restartCtx, d.VM, d.Name, d.ShutdownTimeout); err != nil {
					log.Printf("vm backup: wait for %q to shut off before starting it again: %v", d.Name, err)
				}
			}
			startErr := d.VM.Start(restartCtx, d.Name)
			if startErr == nil {
				clearVMDown(d)
			} else if backupErr == nil {
				backupErr = fmt.Errorf("vm backup: restart vm: %w", startErr)
			}
		}()

		// Graceful shutdown + poll until "shut off".
		if wasRunning {
			atStep(d.Step, progress.StepStop)
			markVMDown(d)
			if backupErr = d.VM.Shutdown(restartCtx, d.Name); backupErr != nil {
				// The guest never got the request, so it is still up.
				clearVMDown(d)
				backupErr = fmt.Errorf("vm backup: shutdown: %w", backupErr)
				return
			}
			shuttingDown = true
			if backupErr = waitShutOff(ctx, d.VM, d.Name, d.ShutdownTimeout); backupErr != nil {
				return
			}
			shuttingDown = false
		}

		// Build path list: disks + nvram + tpm (if present).
		paths := append([]string(nil), d.DiskPaths...)
		if d.NVRAMPath != "" {
			paths = append(paths, d.NVRAMPath)
		}
		if d.TPMPath != "" {
			paths = append(paths, d.TPMPath)
		}

		tags := withRunTag(withFormerNames([]string{"vm:" + d.Name, "p2"}, d.FormerNames), d.RunTag)
		if wasRunning {
			atStep(d.Step, progress.StepRead)
		}
		// A VM whose disks are all zvols has nothing for restic to read here.
		if len(paths) > 0 {
			summary, backupErr = d.Restic.Backup(ctx, d.RepoPath, paths, tags)
			if backupErr != nil {
				backupErr = fmt.Errorf("vm backup: restic: %w", backupErr)
				return
			}
		}

		// restic cannot read a block device by path, so each zvol goes in as
		// a raw image of its own.
		disks, zErr := backupBlockDisksAndLog(ctx, d, "vm backup")
		if zErr != nil {
			backupErr = zErr
			return
		}
		// Written after the disks, so a run whose disks failed leaves nothing
		// behind that looks like a backup of the VM.
		if len(paths) == 0 {
			if summary, backupErr = backupDomainXML(ctx, d, tags); backupErr != nil {
				backupErr = fmt.Errorf("vm backup: %w", backupErr)
				return
			}
		}
		if len(d.BlockDisks) > 0 {
			summary = summary.Plus(disks)
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
	// ZFS snapshots the zvols atomically, so a VM with only zvol disks needs
	// no qemu overlay.
	if len(commitDevs) == 0 && len(d.DiskPaths) == 0 && len(d.BlockDisks) > 0 {
		disks, zErr := backupBlockDisksAndLog(ctx, d, "vm live backup")
		if zErr != nil {
			return Summary{}, zErr
		}
		tags := withRunTag(withFormerNames([]string{"vm:" + d.Name, "p2", "live"}, d.FormerNames), d.RunTag)
		summary, err := backupDomainXML(ctx, d, tags)
		if err != nil {
			return Summary{}, fmt.Errorf("vm live backup: %w", err)
		}
		return summary.Plus(disks), nil
	}
	if len(commitDevs) == 0 {
		return Summary{}, fmt.Errorf("vm live backup: no writable disk to snapshot/commit. Use the graceful method for this VM")
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
	tags := withRunTag(withFormerNames([]string{"vm:" + d.Name, "p2", "live"}, d.FormerNames), d.RunTag)
	summary, backupErr := d.Restic.Backup(ctx, d.RepoPath, paths, tags)

	// ALWAYS commit EVERY overlay back, even if the backup failed, so no disk keeps
	// diverging on an uncommitted overlay. Attempt all devices; report the first
	// failure (the VM keeps running on its overlay either way — no data lost).
	// Not ctx: a cancelled or timed-out backup ends ctx, a commit on a done
	// context is refused at once, and the VM would stay on its overlay.
	var commitErr error
	commitCtx := context.WithoutCancel(ctx)
	for _, dev := range commitDevs {
		if cErr := d.VM.BlockCommitActivePivot(commitCtx, d.Name, dev); cErr != nil && commitErr == nil {
			commitErr = cErr
		}
	}
	if commitErr != nil {
		return Summary{}, fmt.Errorf("vm live backup: blockcommit failed. The VM is STILL RUNNING on its snapshot overlay (no data lost); resolve the overlay before the next backup: %w", commitErr)
	}
	if backupErr != nil {
		return Summary{}, fmt.Errorf("vm live backup: restic: %w", backupErr)
	}

	// zvol disks get no overlay (ParseDomain lists them in SkipSnapshotDevs);
	// a ZFS snapshot holds them still while they are read instead.
	disks, zErr := backupBlockDisksAndLog(ctx, d, "vm live backup")
	if zErr != nil {
		return Summary{}, zErr
	}
	if len(d.BlockDisks) > 0 {
		summary = summary.Plus(disks)
	}
	return summary, nil
}

// VMDomainStdinPath is where the run snapshot of a VM without file disks keeps
// the domain XML.
func VMDomainStdinPath(name string) string {
	return "/vm-domains/" + name + ".xml"
}

// backupDomainXML writes the run snapshot of a VM without file disks. The
// zvol snapshots carry tags of their own, so without it nothing under
// vm:<name> would show that the run happened.
func backupDomainXML(ctx context.Context, d VMBackupDeps, tags []string) (Summary, error) {
	sum, err := d.ZvolRestic.BackupStdin(ctx, d.RepoPath, strings.NewReader(d.DomainXML), VMDomainStdinPath(d.Name), tags)
	if err != nil {
		return Summary{}, fmt.Errorf("domain definition: %w", err)
	}
	return sum, nil
}

// markVMDown records the VM about to shut down. A failed record costs only the
// restart after a crash, not the backup.
func markVMDown(d VMBackupDeps) {
	if d.Restarts == nil {
		return
	}
	if err := d.Restarts.Mark([]string{d.Name}); err != nil {
		log.Printf("vm backup: recording that %q is about to shut down failed: %v", d.Name, err)
	}
}

// clearVMDown drops the record once the VM runs again.
func clearVMDown(d VMBackupDeps) {
	if d.Restarts == nil {
		return
	}
	if err := d.Restarts.Clear([]string{d.Name}); err != nil {
		log.Printf("vm backup: clearing the shutdown record of %q failed: %v", d.Name, err)
	}
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
		_ = d.Runs.Finish(runID, restoreOutcome(restoreErr), Summary{}, truncateErr(restoreErr))
		return restoreErr
	}
	if err := d.Runs.Finish(runID, statusSuccess, Summary{SnapshotID: d.SnapshotID}, ""); err != nil {
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
	if len(allPaths) == 0 && len(d.BlockDisks) == 0 {
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
	for _, bd := range d.BlockDisks {
		if bd.SnapshotID == "" || bd.StdinPath == "" || bd.Size <= 0 {
			return fmt.Errorf("vm restore: this backup holds no image of the zvol disk %s", bd.Dataset)
		}
		if err := d.Restic.VerifySnapshot(ctx, d.RepoPath, bd.SnapshotID); err != nil {
			return fmt.Errorf("vm restore: snapshot preflight of the zvol disk %s: %w", bd.Dataset, err)
		}
	}

	// If the VM currently exists, destroy (if running) then undefine it.
	state, err := d.VM.State(ctx, d.Name)
	if err != nil {
		return fmt.Errorf("vm restore: check state: %w", err)
	}
	if state != "" {
		// VM exists on the host.
		if err := releaseCheckpointsForRestore(ctx, d.VM, d.Name); err != nil {
			return err
		}
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
	if len(d.Images) > 0 {
		for _, img := range d.Images {
			if err := RestoreBlockImage(ctx, d.ImageDumper, d.Convert, d.RepoPath, d.SnapshotID, img); err != nil {
				return fmt.Errorf("vm restore: %w", err)
			}
		}
	} else if len(d.RestoreDirs) > 0 {
		// Each snapshot subtree goes into its Target: a chosen pool for a
		// cross-instance restore, the moved folder for a snapshot from before a rename.
		for _, rd := range d.RestoreDirs {
			if err := d.Restic.RestoreSubtreeTo(ctx, d.RepoPath, d.SnapshotID, rd.Subtree, rd.Target); err != nil {
				return fmt.Errorf("vm restore: restic restore: %w", err)
			}
		}
	} else if len(allPaths) > 0 {
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

	// A VM missing a disk it boots from is not defined.
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

// checkpointRecords is the part of CheckpointHost a restore needs. A VM
// without it has no checkpoints to worry about.
type checkpointRecords interface {
	CheckpointNames(ctx context.Context, domain string) ([]string, error)
	CheckpointForget(ctx context.Context, domain, checkpoint string) error
}

// releaseCheckpointsForRestore clears the checkpoint records that would make
// libvirt refuse to undefine the VM once it is off. BombVault's own go, since
// the restore replaces their disks anyway; anyone else's stop the restore
// before the VM is touched.
func releaseCheckpointsForRestore(ctx context.Context, vm VM, name string) error {
	cr, ok := vm.(checkpointRecords)
	if !ok {
		return nil
	}
	names, err := cr.CheckpointNames(ctx, name)
	if err != nil {
		return fmt.Errorf("vm restore: list checkpoints: %w", err)
	}
	var foreign []string
	for _, n := range names {
		if !strings.HasPrefix(n, BlocksCheckpointPrefix) {
			foreign = append(foreign, n)
		}
	}
	if len(foreign) > 0 {
		return fmt.Errorf("vm restore: the VM has checkpoints BombVault did not create (%s); libvirt cannot remove a VM that has checkpoints, so delete them first", strings.Join(foreign, ", "))
	}
	for _, n := range names {
		if err := cr.CheckpointForget(ctx, name, n); err != nil {
			return fmt.Errorf("vm restore: remove checkpoint %s: %w", n, err)
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
