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
	// read-only). quiesce uses the qemu guest agent for app-consistency.
	SnapshotCreateDiskOnly(ctx context.Context, name, snapName string, quiesce bool) error
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
	// DiskDevice is the first disk's target dev (e.g. "vda", "hdc") — the
	// blockcommit target for live backup. Empty disables live commit.
	DiskDevice string
	// NVRAMPath is the container-visible NVRAM path (empty for BIOS VMs).
	NVRAMPath string
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

	VM     VM
	Restic Restic
	Runs   Runs
}

// VMRestoreDeps bundles everything RestoreVM needs.
type VMRestoreDeps struct {
	// Confirmed MUST be true — guard against an accidental destructive restore.
	Confirmed bool
	// Name is the libvirt domain name.
	Name string
	// SnapshotID is the restic snapshot to restore (validated hex).
	SnapshotID string
	// DiskPaths are the absolute container-visible paths to restore.
	DiskPaths []string
	// NVRAMPath is the absolute container-visible NVRAM path (may be empty).
	NVRAMPath string
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

	VM     VM
	Restic Restic
	Runs   Runs
}

// ---------------------------------------------------------------------------
// BackupVMGraceful
// ---------------------------------------------------------------------------

// BackupVMGraceful orchestrates a graceful VM backup:
//
//	recordRunStart
//	→ IsActive (capture wasRunning)
//	→ Shutdown → poll State until "shut off" (timeout → Destroy)
//	→ restic Backup (diskPaths + nvram, tags ["vm:<name>", "p2"])
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

	wasRunning, err := d.VM.IsActive(ctx, d.Name)
	if err != nil {
		_ = d.Runs.Finish(runID, statusFailed, "", 0, truncateErr(err))
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

		// Build path list: disks + nvram (if present).
		paths := append([]string(nil), d.DiskPaths...)
		if d.NVRAMPath != "" {
			paths = append(paths, d.NVRAMPath)
		}

		tags := []string{"vm:" + d.Name, "p2"}
		summary, backupErr = d.Restic.Backup(ctx, d.RepoPath, paths, tags)
		if backupErr != nil {
			backupErr = fmt.Errorf("vm backup: restic: %w", backupErr)
		}
	}()

	if backupErr != nil {
		_ = d.Runs.Finish(runID, statusFailed, "", 0, truncateErr(backupErr))
		return Summary{}, backupErr
	}

	if err := d.Runs.Finish(runID, statusSuccess, summary.SnapshotID, summary.Bytes, ""); err != nil {
		return summary, fmt.Errorf("vm backup: record run finish: %w", err)
	}
	return summary, nil
}

// BackupVMLive backs up a RUNNING VM without shutting it down:
//
//	snapshot-create-as --disk-only --atomic (VM writes to a fresh overlay)
//	→ restic backs up the now-static base disk(s)
//	→ blockcommit --active --pivot (merge overlay back, pivot the live VM)
//
// SAFETY: on ANY failure the VM is left RUNNING and usable — it is never
// destroyed or undefined. A blockcommit failure surfaces a clear, actionable
// error (the VM keeps running on its overlay; no data is lost) and records the
// run failed. Requires DiskDevice (the blockcommit target); falls back to a
// clear error when it is empty.
func BackupVMLive(ctx context.Context, d VMBackupDeps) (Summary, error) {
	runID, err := d.Runs.Start(d.TargetID, kindBackup)
	if err != nil {
		return Summary{}, fmt.Errorf("vm live backup: record run start: %w", err)
	}
	if d.DiskDevice == "" {
		e := fmt.Errorf("vm live backup: no disk device to commit — cannot safely snapshot; use graceful method")
		_ = d.Runs.Finish(runID, statusFailed, "", 0, truncateErr(e))
		return Summary{}, e
	}

	const snap = "bombvault-tmp"
	quiesce := d.VM.GuestAgentPing(ctx, d.Name)

	// Create the overlay. Nothing destructive yet — on failure the VM is untouched.
	if err := d.VM.SnapshotCreateDiskOnly(ctx, d.Name, snap, quiesce); err != nil {
		e := fmt.Errorf("vm live backup: snapshot: %w", err)
		_ = d.Runs.Finish(runID, statusFailed, "", 0, truncateErr(e))
		return Summary{}, e
	}

	// Back up the now-static base disk(s).
	paths := append([]string(nil), d.DiskPaths...)
	if d.NVRAMPath != "" {
		paths = append(paths, d.NVRAMPath)
	}
	tags := []string{"vm:" + d.Name, "p2", "live"}
	summary, backupErr := d.Restic.Backup(ctx, d.RepoPath, paths, tags)

	// ALWAYS attempt to commit the overlay back, even if the backup failed, so the
	// VM does not keep diverging on the overlay.
	if commitErr := d.VM.BlockCommitActivePivot(ctx, d.Name, d.DiskDevice); commitErr != nil {
		e := fmt.Errorf("vm live backup: blockcommit failed — the VM is STILL RUNNING on its snapshot overlay (no data lost); resolve the overlay before the next backup: %w", commitErr)
		_ = d.Runs.Finish(runID, statusFailed, "", 0, truncateErr(e))
		return Summary{}, e
	}
	if backupErr != nil {
		e := fmt.Errorf("vm live backup: restic: %w", backupErr)
		_ = d.Runs.Finish(runID, statusFailed, "", 0, truncateErr(e))
		return Summary{}, e
	}
	if err := d.Runs.Finish(runID, statusSuccess, summary.SnapshotID, summary.Bytes, ""); err != nil {
		return summary, fmt.Errorf("vm live backup: record run finish: %w", err)
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
//	→ restic RestorePaths (diskPaths + nvram, per-path back to origin)
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
		_ = d.Runs.Finish(runID, statusFailed, "", 0, truncateErr(restoreErr))
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
	if len(allPaths) == 0 {
		return fmt.Errorf("vm restore: no paths to restore (unsafe)")
	}
	for _, p := range allPaths {
		if !strings.HasPrefix(p, "/") || strings.Contains(p, "..") {
			return fmt.Errorf("vm restore: unsafe path %q (unsafe)", p)
		}
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

	// VM disk images and NVRAM are FILES; restic's <id>:<subpath> subtree form
	// needs a DIRECTORY (a file path fails with "not a directory"). Restore each
	// file's PARENT directory instead (deduplicated): restic restores only the
	// snapshot's files in that dir and never deletes existing siblings.
	restoreDirs := parentDirs(allPaths)
	if len(restoreDirs) == 0 {
		return fmt.Errorf("vm restore: no restorable directories derived from paths")
	}
	if err := d.Restic.RestorePaths(ctx, d.RepoPath, d.SnapshotID, restoreDirs); err != nil {
		return fmt.Errorf("vm restore: restic restore: %w", err)
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
