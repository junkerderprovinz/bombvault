package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/backup"
	"github.com/junkerderprovinz/bombvault/internal/paths"
	"github.com/junkerderprovinz/bombvault/internal/platform"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
	"github.com/junkerderprovinz/bombvault/internal/virshcli"
)

// errZvolRebaseFailed is the scrubber-bypass sentinel for a rebase failure in
// prepareRestoreVMForTarget's cross-instance zvol rebase loop, matched by
// handlers.go's scrubError. The dataset and pool names are the message, and a
// ZFS dataset name "<pool>/<rest>" contains "/", which handlers.go's absPathRe
// would mistake for a filesystem path and mangle (`dataset "tank/vm-disk1"`
// becomes `dataset "tank[path]"`). Same bypass as errRestoreDestination and
// errUnraidPlatformMismatch.
var errZvolRebaseFailed = errors.New("zvol dataset rebase failed")

// zvolRebaseErr carries a zvol-rebase failure's ready-to-show message (see
// errZvolRebaseFailed) while still satisfying
// errors.Is(err, errZvolRebaseFailed) for the scrubber bypass.
type zvolRebaseErr struct{ msg string }

func (e *zvolRebaseErr) Error() string { return e.msg }

func (e *zvolRebaseErr) Is(target error) bool { return target == errZvolRebaseFailed }

// StartBackupVM launches a single VM backup in a background goroutine and
// returns immediately, mirroring StartBackup for the VM domain. Progress is
// published under "vm:<name>". Shares batchActive (no overlap with any other
// backup); returns (false, nil) if one is already running, or (false, err) if the
// vms domain is already busy with another op.
func (s *Service) StartBackupVM(ctx context.Context, name string) (bool, error) {
	if !s.batchActive.CompareAndSwap(false, true) {
		return false, nil
	}
	if op, busy := s.domainBusy("vms"); busy {
		s.batchActive.Store(false)
		return false, fmt.Errorf("%s is running on vms", op)
	}
	bctx := context.WithoutCancel(ctx)
	go func() {
		defer s.recoverOperation("backup vm: "+name, nil, func(msg string) {
			if tg, tErr := s.store.GetVMTargetByName(name); tErr == nil {
				s.failStuckRun(tg.ID, msg)
			}
		})
		defer s.batchActive.Store(false)
		if _, err := s.BackupVM(bctx, name); err != nil {
			log.Printf("api: backup vm: %q failed: %v", name, err) //nolint:gosec // G706: name is %q-quoted
		}
	}()
	return true, nil
}

// vmRunTag returns the "vmrun:<runID>" correlation tag
// (VMBackupDeps.RunTag in internal/backup/vm_orchestrator.go) carried by
// the snapshot in snaps matching id (exact or unambiguous prefix, like
// snapshotBelongs), or "" when there is no match or the matching snapshot
// carries no such tag.
//
// "" is a permanent restore fallback: BackupVM sets RunTag only when the
// VM has zvol disks, since a file-only VM's single snapshot is already
// identified by its plain "vm:<name>" tag (see BackupVM's RunTag comment).
// So "" covers both runs older than the tag and every file-only VM's
// backup, which is most VMs. Callers treat "" as "resolve via id alone".
func vmRunTag(snaps []restic.Snapshot, id string) string {
	for _, sn := range snaps {
		if sn.ID != id && !strings.HasPrefix(sn.ID, id) {
			continue
		}
		for _, t := range sn.Tags {
			if strings.HasPrefix(t, "vmrun:") {
				return t
			}
		}
		return ""
	}
	return ""
}

// vmrunGroupSnapshot returns the snapshot in group (a vmRunTag-keyed
// snapshotsForTag listing) carrying tag exactly, a zvol disk's own
// "vm:<name>:zvol:<dev>" identity tag (see VMBlockDisk.Dev in
// internal/backup/vm_orchestrator.go), or false when no member does.
func vmrunGroupSnapshot(group []restic.Snapshot, tag string) (restic.Snapshot, bool) {
	for _, sn := range group {
		for _, t := range sn.Tags {
			if t == tag {
				return sn, true
			}
		}
	}
	return restic.Snapshot{}, false
}

// vmDefinition is the recreate recipe persisted at VM backup time so restore
// works even after the VM has been deleted or BombVault's /config is lost
// (full DR). It carries container-visible paths so the restore orchestrator
// can pass them directly to restic.
type vmDefinition struct {
	DomainXML string   `json:"domain_xml"`
	DiskPaths []string `json:"disk_paths"` // container-visible absolute paths (under the Host Data mount)
	// NVRAM travels in the definition (read and written over SSH), not via a
	// libvirt mount. NVRAMHostPath is the host path from the domain XML;
	// NVRAMBytes is the captured var store (base64 in JSON). Empty for BIOS VMs
	// or when SSH capture failed; EnsureNVRAMTemplate then regenerates it on
	// restore.
	NVRAMHostPath string `json:"nvram_host_path"`
	NVRAMBytes    []byte `json:"nvram_bytes,omitempty"`
	// TPMBytes is the captured vTPM state, read and written over SSH like
	// NVRAMBytes (see BackupVM's TPM read and prepareRestoreVMForTarget's
	// PreDefine write-back). Unlike NVRAM, the TPM's host path is not stored:
	// it is re-derived from DomainXML (virshcli.ParseDomain's TPMPath) at
	// backup and restore time, since it is a stable property of the domain
	// definition that restore never remaps (destBase's cross-instance remap
	// only rewrites file-disk sources and NVRAM; see
	// virshcli.RewriteDiskSources/RewriteNVRAM). Empty for a VM without vTPM
	// (DomainInfo.TPMPath == "") or when SSH capture failed; a read failure is
	// non-fatal, like NVRAM's.
	TPMBytes     []byte `json:"tpm_bytes,omitempty"`
	Method       string `json:"method"`
	WasAutostart bool   `json:"was_autostart"`
	// WasRunning is the VM's run state at backup time. A pointer, so a backup
	// without the field reads as nil (unknown) and restore boots the VM. A
	// non-nil value is honoured so restore mirrors the captured state, as for
	// containers.
	WasRunning *bool `json:"was_running,omitempty"`
	// Aliases are the links the mirror records, as in containerDefinition.
	Aliases []definitionAlias `json:"aliases,omitempty"`
}

// VMView is the per-VM row returned by ListVMs.
type VMView struct {
	Name string `json:"name"`
	// LibvirtName is the raw libvirt domain name (vm.Name, never
	// vm.FriendlyName) on every platform. It equals Name everywhere except
	// TrueNAS, where Name is the presentation-only friendly name (see the
	// isTrueNAS block in ListVMs). It is the only field the frontend may send
	// back on a VM action call (backup, restore, snapshots, forget, method,
	// include, scheduleCadence, backup-order, DR-drill-target): every such
	// route (see vmNameParam in handlers.go) hands the path segment straight
	// to virsh without resolution. Name is display-only; sent as an identifier
	// on TrueNAS it would target a domain name virsh has never heard of.
	LibvirtName       string `json:"libvirtName"`
	State             string `json:"state"`
	Method            string `json:"method"`
	IncludeInSchedule bool   `json:"includeInSchedule"`
	LastBackup        *int64 `json:"lastBackup"`
	LastBackupStarted *int64 `json:"lastBackupStarted"`
	// ScheduleCadence is the VM's optional per-item schedule override (#121); ""
	// means it follows the VMs domain schedule. Only takes effect when the
	// perItemSchedules setting is on.
	ScheduleCadence string        `json:"scheduleCadence"`
	Placement       placementView `json:"placement"`
	// RenameFrom and RenameReason suggest a rename: they are set on a live VM
	// with no backups of its own whose libvirt UUID matches a not-installed
	// entry's. They match containerView's fields so the frontend treats both
	// alike.
	RenameFrom   string `json:"renameFrom"`
	RenameReason string `json:"renameReason"`
	// AliasConflicts are the former names of this entry that are live domains
	// again, alphabetically, as in containerView.
	AliasConflicts []string `json:"aliasConflicts"`
	// Aliases are the libvirt names this entry had before, oldest link first.
	Aliases []string `json:"aliases"`
}

// vmUUID returns tg's libvirt UUID. An empty column is filled from the saved
// domain XML and stored, because the migration that added the column cannot
// parse XML. A definition that yields no UUID returns "" and writes nothing.
func (s *Service) vmUUID(tg store.VMTarget) string {
	if tg.UUID != "" {
		return tg.UUID
	}
	uuid := definitionUUID(tg.Definition)
	if uuid == "" {
		return ""
	}
	if err := s.store.SetVMUUID(tg.Name, uuid); err != nil {
		log.Printf("api: vmUUID: backfill %q: %v", tg.Name, err) //nolint:gosec // G706: %q-quoted
	}
	return uuid
}

// definitionUUID is the libvirt UUID in the domain XML of a stored VM
// definition, "" when it names none or does not parse.
func definitionUUID(definition string) string {
	var def vmDefinition
	if err := json.Unmarshal([]byte(definition), &def); err != nil {
		return ""
	}
	info, err := virshcli.ParseDomain(def.DomainXML)
	if err != nil {
		return ""
	}
	return info.UUID
}

// ListVMs returns all known VMs (from virsh) merged with the DB targets.
// VMs with no virsh entry but with backup history appear as state="not-installed".
func (s *Service) ListVMs(ctx context.Context) ([]VMView, error) {
	// Only reach libvirt over SSH when the VMs domain is enabled. The dashboard
	// calls this on every GUI load, and for users who don't back up VMs at all
	// an unconditional virsh-over-SSH connect would spam the container log
	// with "could not resolve hostname / connection reset" errors. Stored VM
	// targets are still listed (as orphans); only the live enumeration is
	// skipped.
	settings, err := s.store.GetSettings()
	if err != nil {
		return nil, fmt.Errorf("read settings: %w", err)
	}
	var infos []virshcli.VMInfo
	if settings.VMsEnabled {
		infos, err = s.virsh.List(ctx)
		if err != nil {
			return nil, fmt.Errorf("list vms: virsh: %w", err)
		}
	}
	targets, _ := s.store.ListVMTargets()
	byName := make(map[string]store.VMTarget, len(targets))
	for _, t := range targets {
		byName[t.Name] = t
	}

	// displayName is the raw libvirt name on every platform except TrueNAS,
	// where it is the presentation-only virshcli.VMInfo.FriendlyName (the bare
	// UUID libvirt uses for domain names on TrueNAS 26, resolved to something
	// readable). Gated explicitly on Kind() because of the gotcha documented
	// on FriendlyName (internal/virshcli/types.go): the classifier that
	// populates it is shape-based, not platform-gated, so a non-TrueNAS host
	// must never trust it even when it differs from Name. vm.Name, never
	// FriendlyName, is still used below for the byName lookup and stays the
	// identifier everywhere else.
	isTrueNAS := s.platformFn().Kind() == platform.KindTrueNAS

	live := make(map[string]bool, len(infos))
	for _, vm := range infos {
		live[vm.Name] = true
	}

	// A failed alias read only drops the conflict warnings and aliases: a
	// missing warning does less harm than a VM list that does not load.
	var formerNames, aliasConflicts aliasIndex
	if aliases, aErr := s.store.ListAliases("vm"); aErr != nil {
		log.Printf("api: list vms: alias conflict check: %v", aErr)
	} else {
		formerNames = newAliasIndex(aliases)
		aliasConflicts = liveFormerNames(aliases, live)
	}

	var orphanTargets []store.VMTarget
	for _, t := range targets {
		if !live[t.Name] {
			orphanTargets = append(orphanTargets, t)
		}
	}

	// One listing dates every row and tells the rename pass whether a live VM
	// has backups under its own name; a failed read keeps that pass from
	// guessing, as on the container list. With no VM and no entry there is
	// nothing to date.
	var snapTimes map[string]int64
	snapTimesFailed := false
	if len(infos) > 0 || len(targets) > 0 {
		if m, sErr := s.LatestVMBackupTimes(ctx); sErr != nil {
			log.Printf("api: list vms: latest backup times: %v", sErr)
			snapTimesFailed = true
		} else {
			snapTimes = m
		}
	}

	views := make([]VMView, 0, len(infos)+len(targets))
	viewIndex := make(map[string]int, len(infos)) // live rows only
	hasOwnBackup := make(map[string]bool, len(infos))
	needsRenameSuggestion := false
	for _, vm := range infos {
		displayName := vm.Name
		if isTrueNAS {
			displayName = vm.FriendlyName
		}
		v := VMView{Name: displayName, LibvirtName: vm.Name, State: vm.State, Method: "graceful", AliasConflicts: []string{}, Aliases: []string{}}
		var run *store.Run
		if t, ok := byName[vm.Name]; ok {
			v.AliasConflicts = aliasConflicts.of(t.ID)
			v.Aliases = formerNames.of(t.ID)
			v.Method = t.Method
			v.IncludeInSchedule = t.IncludeInSchedule
			v.ScheduleCadence = t.ScheduleCadence
			run, _ = s.store.LastSuccessfulBackup(t.ID)
		}
		v.LastBackup, v.LastBackupStarted = lastBackupDate(vm.Name, run, snapTimes, snapTimesFailed)
		own := v.LastBackup != nil
		hasOwnBackup[vm.Name] = own
		if !own {
			needsRenameSuggestion = true
		}
		viewIndex[vm.Name] = len(views)
		views = append(views, v)
	}

	// The match runs over every live domain to keep it one-to-one, so a VM
	// with backups of its own can still come back matched and is skipped here.
	for liveName, cand := range s.suggestVMRenames(ctx, infos, orphanTargets, needsRenameSuggestion && !snapTimesFailed) {
		if hasOwnBackup[liveName] {
			continue
		}
		if idx, ok := viewIndex[liveName]; ok {
			views[idx].RenameFrom = cand.OldName
			views[idx].RenameReason = cand.Reason
		}
	}

	// Orphans: targets whose VM is not defined on the host.
	for _, t := range orphanTargets {
		v := VMView{Name: t.Name, LibvirtName: t.Name, State: "not-installed", Method: t.Method, IncludeInSchedule: t.IncludeInSchedule, ScheduleCadence: t.ScheduleCadence, AliasConflicts: aliasConflicts.of(t.ID), Aliases: formerNames.of(t.ID)}
		run, _ := s.store.LastSuccessfulBackup(t.ID)
		v.LastBackup, v.LastBackupStarted = lastBackupDate(t.Name, run, snapTimes, snapTimesFailed)
		views = append(views, v)
	}
	return views, nil
}

// leftoverOverlayDevices returns the target devices of any writable disk
// whose source is a leftover BombVault live-snapshot overlay (a
// "*.bombvault-tmp" file) from an interrupted live backup. Such an overlay
// blocks the next snapshot ("…already exists…") and, left in place, would
// make a backup capture only the overlay and not its base disk. Matching
// on BombVault's own snapshot name is unambiguous: never a cdrom or a
// user's manual snapshot.
func leftoverOverlayDevices(d virshcli.DomainInfo) []string {
	// libvirt names a snapshot-create-as overlay "<base>.<snapname>", so a
	// BombVault leftover is exactly a "*.bombvault-tmp" file. Match the suffix,
	// not a bare substring, so a disk whose path merely contains the name is
	// not hit.
	suffix := "." + backup.LiveSnapshotName
	var devs []string
	for _, disk := range d.Disks {
		if strings.HasSuffix(disk.Source, suffix) {
			devs = append(devs, disk.Dev)
		}
	}
	return devs
}

// recoverLeftoverOverlay commits a leftover BombVault snapshot overlay back
// into its base before a backup, so the VM is on a clean disk chain (live
// snapshots work again and the backup captures the real base, not just the
// overlay). It only ever commits a disk whose source is BombVault's own
// "*.bombvault-tmp". It returns the refreshed domain XML and parsed info; a
// domain without a leftover is returned unchanged. The VM must be running
// to active-commit; a shut-off VM with a leftover is an error the user must
// resolve, since it is never started silently.
func (s *Service) recoverLeftoverOverlay(ctx context.Context, name, xmlStr string, domain virshcli.DomainInfo) (string, virshcli.DomainInfo, error) {
	devs := leftoverOverlayDevices(domain)
	if len(devs) == 0 {
		return xmlStr, domain, nil
	}
	// Must be running to active-commit. The check error is not swallowed: a
	// flaky host must not be misread as "shut off", which would send a
	// confusing message and could mask a real fault.
	running, aerr := s.virsh.IsActive(ctx, name)
	if aerr != nil {
		return xmlStr, domain, fmt.Errorf("backup vm: check running state for overlay recovery: %w", aerr)
	}
	if !running {
		return xmlStr, domain, fmt.Errorf("backup vm: %q is shut off but left on a BombVault snapshot overlay from an interrupted live backup; start it briefly so the overlay can be merged, then retry", name)
	}
	log.Printf("api: BackupVM: %q is on a leftover BombVault snapshot overlay (%v); committing it back before backup", name, devs) //nolint:gosec // G706: %q-quoted name
	for _, dev := range devs {
		if cErr := s.virsh.BlockCommitActivePivot(ctx, name, dev); cErr != nil {
			return xmlStr, domain, fmt.Errorf("backup vm: recover leftover snapshot overlay (%s): %w", dev, cErr)
		}
	}
	// Re-read the now-clean domain so the backup reads the real base disk, not the overlay.
	fresh, err := s.virsh.DumpXML(ctx, name)
	if err != nil {
		return xmlStr, domain, fmt.Errorf("backup vm: re-dumpxml after overlay recovery: %w", err)
	}
	freshDomain, err := virshcli.ParseDomain(fresh)
	if err != nil {
		return xmlStr, domain, fmt.Errorf("backup vm: parse domain after overlay recovery: %w", err)
	}
	// Verify the commit actually cleared the overlay; if libvirt reported success
	// but the chain is still dirty, fail with a precise message rather than letting
	// the next snapshot fail with an opaque "already exists".
	if still := leftoverOverlayDevices(freshDomain); len(still) > 0 {
		return xmlStr, domain, fmt.Errorf("backup vm: overlay recovery did not clear the snapshot overlay on %v for %q; resolve it manually", still, name)
	}
	return fresh, freshDomain, nil
}

// removeStrayOverlays deletes leftover BombVault live-snapshot overlay
// files ("*.bombvault-tmp") next to the VM's base disks. blockcommit
// --active --pivot merges an overlay back into its base and switches the VM
// onto the base, but does not delete the orphaned overlay file, so without
// this every successful live backup would leave one behind and the next
// snapshot would fail with "external snapshot file ... already exists".
// The caller must make sure the VM is on its base disks first (after
// recovery or commit), so these files are never in use. Best-effort:
// failures are logged, never fatal.
func (s *Service) removeStrayOverlays(diskPaths []string) {
	suffix := "." + backup.LiveSnapshotName
	seen := map[string]bool{}
	for _, dp := range diskPaths {
		dir := filepath.Dir(dp)
		if dir == "" || dir == "." || seen[dir] {
			continue
		}
		seen[dir] = true
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), suffix) {
				continue
			}
			p := filepath.Join(dir, e.Name())
			if rmErr := os.Remove(p); rmErr != nil { //nolint:gosec // G304: dir derived from a translated VM disk path; name has our fixed suffix
				log.Printf("api: BackupVM: could not remove stray overlay %q: %v", e.Name(), rmErr) //nolint:gosec // G706: %q-quoted
			} else {
				log.Printf("api: BackupVM: removed stray live-snapshot overlay %q", e.Name()) //nolint:gosec // G706: %q-quoted
			}
		}
	}
}

// failVMBackup makes a pre-orchestrator VM backup failure visible: it
// records a failed run against the VM's existing target (so it shows in the
// dashboard run history) and fires a notification. It covers failures
// before the orchestrator starts its own run (overlay recovery, the
// running-state check), so a destructive or aborted attempt is never
// silent, least of all for scheduled backups, where nobody sees the HTTP
// error. Best-effort: bookkeeping errors are ignored, since the real cause
// is already returned to the caller.
func (s *Service) failVMBackup(ctx context.Context, name string, cause error) {
	if tg, err := s.store.GetVMTargetByName(name); err == nil {
		if runID, sErr := s.store.StartRun(tg.ID, "backup"); sErr == nil {
			msg := cause.Error()
			if len(msg) > 500 {
				msg = msg[:500]
			}
			_ = s.store.FinishRun(runID, "failed", "", 0, msg)
		}
	}
	s.notifyBackup(ctx, "VM", name, false, backup.Summary{}, cause)
}

// vmDiskContainerPaths is where restic reads the file disks of domain through
// the Host Data mount, the paths a VM definition stores. A domain with no file
// disk, or with one outside the mount, is an error: a snapshot of it would
// restore nothing.
func (s *Service) vmDiskContainerPaths(name string, domain virshcli.DomainInfo) ([]string, error) {
	if len(domain.DiskPaths) == 0 {
		return nil, fmt.Errorf("no disk paths found in domain XML for %q", name)
	}
	diskPaths := make([]string, 0, len(domain.DiskPaths))
	for _, hp := range domain.DiskPaths {
		cp, ok := s.toContainerPath(hp)
		if !ok {
			return nil, fmt.Errorf("disk %q is not under the host mount and can't be reached for backup. The VM disk must live under your Host Data mount (/mnt)", hp)
		}
		diskPaths = append(diskPaths, cp)
	}
	return diskPaths, nil
}

// BackupVM orchestrates a full VM backup: resolve repo and mode, ensure the
// repo, dump the XML, parse the domain, translate paths, upsert the VM
// target and run the orchestrator.
func (s *Service) BackupVM(ctx context.Context, name string) (_ backup.Summary, retErr error) {
	// Survive the client that triggered it disconnecting (see Backup): detach from
	// the request's cancellation with a generous hard cap.
	ctx, cancel := backupHoldCtx(ctx)
	defer cancel()
	s.registerBackupCancel("vm:"+name, cancel) // reachable by shutdown
	defer s.unregisterBackupCancel("vm:" + name)
	defer s.lockDomain("vms")() // serialise per repo; blocks maintenance ops meanwhile

	// Everything down to the orchestrator returns before any run is recorded,
	// so a failure there would leave the card that started this backup waiting
	// for one. Record it as Backup does; a VM that is not defined on the host
	// is a skip, not a failure.
	var targetID string
	if tg, tErr := s.store.GetVMTargetByName(name); tErr == nil {
		targetID = tg.ID
	}
	orchestrated := false
	defer func() {
		if retErr != nil && !orchestrated && !errors.Is(retErr, backup.ErrVMNotInstalled) {
			s.recordPreflightFailure("BackupVM", name, targetID, retErr)
		}
	}()
	settings, err := s.store.GetSettings()
	if err != nil {
		return backup.Summary{}, fmt.Errorf("read settings: %w", err)
	}
	item := store.ItemRef{Domain: "vms", Key: name}
	step, err := s.prepareHome(ctx, settings, item)
	if err != nil {
		return backup.Summary{}, err
	}

	// Pin the host key before any virsh-over-SSH call (libvirt's qemu+ssh won't
	// self-populate known_hosts). Best-effort: a failure here surfaces again on
	// the virsh call below with full context.
	if s.ssh != nil {
		if err := s.ssh.EnsureKnownHost(ctx); err != nil {
			return backup.Summary{}, fmt.Errorf("backup vm: ssh: %w", err)
		}
	}

	// Capture the domain XML and parse disk/NVRAM paths.
	xmlStr, err := s.virsh.DumpXML(ctx, name)
	if err != nil {
		// The host no longer defines this domain (deleted, or an undefined
		// template). A scheduled target can outlive the VM, so skip it with an
		// info log and a sentinel instead of failing: the scheduler treats
		// ErrVMNotInstalled as a skip, so the nightly job neither errors nor
		// spams. Returns before any run is recorded or failure notification is
		// sent.
		if virshcli.IsNotFound(err) {
			log.Printf("api: BackupVM: skipping %q: not defined on the host (not installed; backups only)", name) //nolint:gosec // G706: name is %q-quoted
			return backup.Summary{}, backup.ErrVMNotInstalled
		}
		return backup.Summary{}, fmt.Errorf("backup vm: dumpxml: %w", err)
	}
	domain, err := virshcli.ParseDomain(xmlStr)
	if err != nil {
		return backup.Summary{}, fmt.Errorf("backup vm: parse domain: %w", err)
	}

	// If the VM is still on a leftover BombVault snapshot overlay from an
	// interrupted live backup, commit it back first so live snapshots work
	// again and the backup reads the real base disk, not just the overlay.
	// No-op otherwise.
	xmlStr, domain, err = s.recoverLeftoverOverlay(ctx, name, xmlStr, domain)
	if err != nil {
		s.failVMBackup(ctx, name, err) // attempted or needed a destructive commit; don't fail silently
		return backup.Summary{}, err
	}

	diskPaths, err := s.vmDiskContainerPaths(name, domain)
	if err != nil {
		return backup.Summary{}, fmt.Errorf("backup vm: %w", err)
	}

	// The VM is now guaranteed on its base disks (recoverLeftoverOverlay committed
	// any overlay). Delete stray "*.bombvault-tmp" overlay files left behind by a
	// previous live backup, otherwise the next snapshot-create fails "already
	// exists". This recovers a VM already stuck in that state.
	s.removeStrayOverlays(diskPaths)

	// NVRAM (the UEFI var store) lives under /etc/libvirt on the host. Read it
	// over SSH and keep it in the definition (no mount, no restic staging). On
	// restore it is written back over SSH; if it is missing,
	// EnsureNVRAMTemplate regenerates it from the OVMF master. A read failure
	// is non-fatal.
	var nvramBytes []byte
	if domain.NVRAMPath != "" && s.ssh != nil {
		if b, rerr := s.ssh.ReadFile(ctx, domain.NVRAMPath); rerr == nil {
			nvramBytes = b
		} else {
			log.Printf("api: BackupVM: WARN NVRAM read over SSH failed for %q (%v); the disks are backed up, but on restore the UEFI variables (boot entries) will be regenerated from the firmware template, not restored", name, rerr) //nolint:gosec // G706: name is %q-quoted
		}
	}

	// vTPM state is captured like NVRAM above: read over SSH from
	// domain.TPMPath (parsed by ParseDomain) and kept in the definition.
	// Skipped when TPMPath is "" (no vTPM on this domain, or an unrecognized
	// <tpm> backend shape; see virshcli.DomainInfo.TPMPath). A read failure is
	// non-fatal.
	var tpmBytes []byte
	if domain.TPMPath != "" && s.ssh != nil {
		if b, rerr := s.ssh.ReadFile(ctx, domain.TPMPath); rerr == nil {
			tpmBytes = b
		} else {
			log.Printf("api: BackupVM: WARN TPM state read over SSH failed for %q (%v); the disks are backed up, but on restore the vTPM will start fresh/empty, not restore its captured state", name, rerr) //nolint:gosec // G706: name is %q-quoted
		}
	}

	// Block-device (zvol) disks go through a separate backup mechanism
	// (BackupZvolDisk, wired via deps.BlockDisks, ZFSHost and ZvolRestic
	// below), since restic cannot back up a raw block device by path. Each
	// disk's ZFS dataset is resolved from its /dev/zvol/<pool>/<dataset> source
	// path. As with a disk outside the host mount above, a block device this
	// mechanism cannot reach fails the whole backup rather than silently
	// producing an incomplete one.
	var vmBlockDisks []backup.VMBlockDisk
	for _, bd := range domain.BlockDisks {
		dataset, ok := virshcli.ZvolDatasetFromDevPath(bd.Source)
		if !ok {
			return backup.Summary{}, fmt.Errorf("backup vm: block-device disk %q (%s) is not a recognizable ZFS zvol and can't be reached for backup", bd.Dev, bd.Source)
		}
		vmBlockDisks = append(vmBlockDisks, backup.VMBlockDisk{Dataset: dataset, Dev: bd.Dev})
	}
	if len(vmBlockDisks) > 0 && s.ssh == nil {
		return backup.Summary{}, fmt.Errorf("backup vm: %q has block-device (zvol) disks but no SSH host connection is configured, and zvol backup requires SSH", name)
	}

	// Default autostart to true (safe: most Unraid-managed VMs have autostart
	// on).
	// TODO: read the real flag from virsh dominfo.
	wasAutostart := true

	// Get method from existing target (default graceful).
	method := "graceful"
	if existing, tErr := s.store.GetVMTargetByName(name); tErr == nil {
		method = existing.Method
	}

	// Store the persistent (inactive) definition for restore so a live-snapshot
	// restore does not re-pin transient/hot-plugged devices (e.g. a guest USB
	// manager's serial stick) that the guest re-adds itself on boot. Fall back to
	// the live XML if --inactive is unavailable.
	defXML := xmlStr
	if inactive, ierr := s.virsh.DumpXMLInactive(ctx, name); ierr == nil && strings.TrimSpace(inactive) != "" {
		defXML = inactive
	}
	// Capture the run-state so restore can mirror it (like containers). Best-effort:
	// a probe failure just leaves it unrecorded (nil) and restore falls back to
	// booting. The VM is still in its original state here (the backup stops/snapshots
	// it later, in the orchestrator).
	var wasRunning *bool
	if running, aerr := s.virsh.IsActive(ctx, name); aerr == nil {
		wasRunning = &running
	}
	def := vmDefinition{
		DomainXML:     defXML,
		DiskPaths:     diskPaths,
		NVRAMHostPath: domain.NVRAMPath,
		NVRAMBytes:    nvramBytes,
		TPMBytes:      tpmBytes,
		Method:        method,
		WasAutostart:  wasAutostart,
		WasRunning:    wasRunning,
	}
	defBytes, _ := json.Marshal(def)

	if step, err = s.recordHome(ctx, settings, item, step); err != nil {
		return backup.Summary{}, err
	}
	repo, mode := step.repo, step.mode

	tg, err := s.store.UpsertVMTarget(store.VMTarget{
		Name: name, Method: method, Definition: string(defBytes), UUID: domain.UUID,
	})
	if err != nil {
		return backup.Summary{}, fmt.Errorf("upsert vm target: %w", err)
	}

	// Every writable disk gets an overlay in a live snapshot, so every one must be
	// committed back afterwards (not just the first).
	var commitDevs []string
	for _, disk := range domain.Disks {
		commitDevs = append(commitDevs, disk.Dev)
	}

	// As in Backup, an alias read failure costs this one backup its formerly:
	// tags and the mirror update, not the backup itself.
	aliases, aliasErr := s.store.TargetAliasesWithDefinitions("vm", tg.ID)
	if aliasErr != nil {
		log.Printf("api: backup vm: aliases of %q: %v", name, aliasErr) //nolint:gosec // G706: %q-quoted
	}

	deps := backup.VMBackupDeps{
		Name:             name,
		FormerNames:      aliasOldNames(aliases),
		DiskPaths:        diskPaths,
		DiskDevice:       domain.DiskDevice,
		CommitDevs:       commitDevs,
		SkipSnapshotDevs: domain.SkipSnapshotDevs,
		RepoPath:         repo,
		TargetID:         tg.ID,
		DataDir:          s.cfg.DataDir,
		VM:               s.virsh,
		Restic:           &resticAdapter{engine: s.engine, mode: mode, extraTags: s.directTags(settings, "vms", repo)},
		BlockDisks:       vmBlockDisks,
		ZFSHost:          sshZFSHost{ssh: s.ssh},
		ZvolRestic:       &resticZvolAdapter{engine: s.engine, mode: mode, extraTags: s.directTags(settings, "vms", repo)},
	}
	live := false
	if method == "live" {
		// A live snapshot only works on a running VM (blockcommit --active --pivot
		// needs an active domain). A shut-off VM is backed up gracefully, which for
		// an already-off VM just backs up the disks and leaves it off. The check
		// error is not swallowed: a flaky host must never be misread as "not
		// running" and silently downgrade a live VM to a shutdown backup.
		running, aerr := s.virsh.IsActive(ctx, name)
		if aerr != nil {
			e := fmt.Errorf("backup vm: check running state: %w", aerr)
			s.failVMBackup(ctx, name, e)
			return backup.Summary{}, e
		}
		if running {
			live = true
		} else {
			log.Printf("api: BackupVM: %q is not running; using graceful backup instead of live", name) //nolint:gosec // G706: %q-quoted
		}
	}

	// Start the run here rather than in the orchestrator's own Runs.Start, so
	// the run id is known before the orchestrator runs: RunTag must be set on
	// deps before they are passed in (see startedRunsAdapter). Wrapped in
	// startedRunsAdapter, the orchestrator's Runs.Start is a read of this same
	// id, not a second run row.
	runID, err := s.store.StartRun(tg.ID, "backup")
	if err != nil {
		return backup.Summary{}, fmt.Errorf("backup vm: record run start: %w", err)
	}
	if gid := runGroupFromContext(ctx); gid != "" {
		// The "Backup Everything" group stamp runsAdapter.Start does, inline here
		// because the run id is produced once, right here, rather than in a later
		// Start() call. Best-effort: a stamp failure must never fail a backup that
		// already started.
		if serr := s.store.SetRunGroup(runID, gid); serr != nil {
			log.Printf("api: BackupVM: run %s: stamp group %s failed: %v", runID, gid, serr) //nolint:gosec // G706: runID/gid are internal ids, not user input
		}
	}
	deps.Runs = startedRunsAdapter{st: s.store, runID: runID, svc: s, cancelKey: "vm:" + name}
	// RunTag correlates every snapshot one backup invocation produces, and is
	// only set when this backup produces more than one restic snapshot. A
	// file-only VM's single snapshot is already identified by its "vm:<name>"
	// tag, and a "vmrun:" tag on it would be permanent noise on every VM
	// snapshot in the repo.
	if len(vmBlockDisks) > 0 {
		deps.RunTag = "vmrun:" + runID
	}

	vkey := "vm:" + name
	// Healthchecks /start ping: deferred to here, past every pre-flight early-return
	// (incl. the ErrVMNotInstalled skip), so the paired done/fail notifyBackup below
	// always follows (no dangling /start).
	s.notifyBackupStart(ctx, "VM")
	// The orchestrator records its own run from here on, so the finisher above
	// stands down.
	orchestrated = true
	bctx, startedAt := s.progBegin(ctx, vkey, "backup")
	var sum backup.Summary
	if live {
		sum, err = backup.BackupVMLive(bctx, deps)
	} else {
		sum, err = backup.BackupVMGraceful(bctx, deps)
	}
	s.progEnd(vkey, "backup", err == nil, startedAt)
	s.notifyBackup(ctx, "VM", name, err == nil, sum, err)
	if err != nil {
		return backup.Summary{}, err
	}
	// A successful live backup commits its overlay back into the base and
	// pivots the VM onto it, but leaves the orphaned overlay file behind;
	// delete it so the next snapshot doesn't fail "already exists". No-op
	// after graceful.
	if live {
		s.removeStrayOverlays(diskPaths)
	}
	// Mirror the definition (encrypted) onto the backup storage so a freshly
	// installed BombVault can rebuild this VM via DiscoverVMs after a database
	// loss, and so a VM deleted from the host stays restorable. Best-effort. A
	// failed alias read leaves it as it was, as in Backup.
	if aliasErr != nil {
		log.Printf("api: backup vm: WARN the stored definition of %q stays as it was, since its aliases could not be read", name) //nolint:gosec // G706: name is %q-quoted
	} else if wErr := s.writeVMDefToStorage(settings, name, repo, defBytes, aliases); wErr != nil {
		log.Printf("api: backup vm: WARN could not persist definition for %q to storage: %v", name, wErr) //nolint:gosec // G706: name is %q-quoted
	}
	// Retention runs once for the VM's identity, aliases included, and once per
	// zvol disk tag, so each disk's history ages as its own group. zvol tags
	// have no aliases: a VM with block disks is never offered a takeover.
	s.applyRetention(ctx, repo, settings, mode, s.vmIdentity(name), "vms")
	for _, bd := range vmBlockDisks {
		if bd.Dev == "" {
			continue // without a target dev it has no tag of its own and ages with "vm:<name>"
		}
		s.applyRetention(ctx, repo, settings, mode, tagIdentity("vm:"+name+":zvol:"+bd.Dev), "vms")
	}
	makeRepoReadable(repo, s.cfg.DataDir) // keep the local repo copyable off-box by a non-root user
	s.replicateOffsite(ctx, "vms", settings, repo, "vm:"+name)
	s.collectStatsAfterItem(ctx, "vms")
	s.checkPrimaryRemoteBudget(ctx, "vms", repo, settings)
	return sum, nil
}

// RestoreVM orchestrates a VM restore from a stored definition.
func (s *Service) RestoreVM(ctx context.Context, name, snapshotID string, confirm bool, source string, leaveStopped bool) error {
	plan, err := s.prepareRestoreVM(ctx, name, snapshotID, confirm, source)
	if err != nil {
		return err
	}
	return s.executeRestoreVM(ctx, name, plan, leaveStopped)
}

// vmRestorePlan carries everything prepareRestoreVM validated and resolved so
// the long-running execution can run detached from the request that asked for
// it (StartRestoreVM) while the sync RestoreVM path keeps identical behaviour.
type vmRestorePlan struct {
	repo         string
	mode         restic.Mode
	targetID     string
	snapshotID   string
	diskPaths    []string
	domainXML    string
	wasAutostart bool
	// restoreDirs restores each snapshot subtree into a destination dir, for a
	// cross-instance restore or a snapshot from before a rename moved the disks.
	// Empty means each disk goes back to its own path.
	restoreDirs []backup.VMRestoreDir
	// wasRunning is the captured run state (nil = a backup without it, so the
	// VM boots after restore).
	wasRunning *bool
	preDefine  func(context.Context) error
	// blockDisks are the domain's block-device (zvol) disks to restore, each
	// entry's SourceDataset resolved from the domain XML and its SnapshotID
	// and StdinPath from the "vmrun:" group (see prepareRestoreVMForTarget).
	// Empty for a VM with only file-backed disks.
	blockDisks []backup.VMRestoreBlockDisk
}

// prepareRestoreVM resolves the settings-configured vms repo (local or
// off-site) and delegates to prepareRestoreVMIn, which performs all of a
// VM restore's validation and resolution synchronously (confirmation,
// snapshot-id guard and ownership, definition lookup, disk-path
// containment and the SSH host-key pin), so a bad request fails
// immediately with a clear error, before anything long-running starts.
// Request guards run here first so a bad request fails with its own error
// before any resolution; prepareRestoreVMIn re-validates them for
// non-settings callers such as the foreign-repo session.
func (s *Service) prepareRestoreVM(ctx context.Context, name, snapshotID string, confirm bool, source string) (vmRestorePlan, error) {
	if !confirm {
		return vmRestorePlan{}, backup.ErrNotConfirmed
	}
	if snapshotID != "latest" && snapshotID != "" && !backup.ValidSnapshotID(snapshotID) {
		return vmRestorePlan{}, backup.ErrInvalidSnapshotID
	}
	settings, err := s.store.GetSettings()
	if err != nil {
		return vmRestorePlan{}, fmt.Errorf("read settings: %w", err)
	}
	repo, err := s.vmRepoForName(settings, name, source)
	if err != nil {
		return vmRestorePlan{}, err
	}
	return s.prepareRestoreVMIn(ctx, repoRef{repo: repo, mode: s.repoModeFor(settings, "vms", source, repo)}, name, snapshotID, confirm)
}

// prepareRestoreVMIn performs all of a VM restore's validation and resolution
// synchronously against an explicit repository, mirroring prepareRestoreIn.
func (s *Service) prepareRestoreVMIn(ctx context.Context, ref repoRef, name, snapshotID string, confirm bool) (vmRestorePlan, error) {
	if !confirm {
		return vmRestorePlan{}, backup.ErrNotConfirmed
	}
	// An explicit snapshot id must be well-formed hex. The orchestrator re-checks
	// this, but guarding here makes a bad id fail synchronously (fail-fast for the
	// async StartRestoreVM path). "latest"/"" resolve below.
	explicitID := snapshotID != "latest" && snapshotID != ""
	if explicitID && !backup.ValidSnapshotID(snapshotID) {
		return vmRestorePlan{}, backup.ErrInvalidSnapshotID
	}

	tg, err := s.store.GetVMTargetByName(name)
	if err != nil {
		return vmRestorePlan{}, errors.New("vm has not been backed up yet")
	}
	// Same-instance restore: no destination base and no destination zvol pool,
	// so each disk returns to the definition's path, from the old folder when a
	// rename moved it, and the domain XML is used verbatim. ref is this
	// instance's own repo, so the local alias history applies.
	return s.prepareRestoreVMForTarget(ctx, ref, name, snapshotID, tg, s.vmIdentity(name), "", "")
}

// prepareRestoreVMForTarget builds a VM restore plan for an already resolved VM
// target tg against an explicit repo ref, without reading or writing the store:
// the VM counterpart of prepareRestoreForTarget. The foreign restore passes a
// target built from the decrypted foreign definition so its disk-path
// containment is validated before that recipe is persisted locally
// (prepareForeignRestore adopts it only once this returns a plan, never on a
// validation failure). The caller runs the confirm / explicit-snapshot-id-shape
// guards first.
//
// id is the identity the ownership check accepts, decided by the caller as in
// prepareRestoreForTarget: the local vmIdentity for a same-instance restore,
// only the literal vm:<name> tag for a foreign one, so a local rename cannot
// widen what a foreign restore accepts.
//
// destBase, when non-empty, remaps every file-backed disk (and the NVRAM) to
// <destBase>/<name>/<basename>: the restic restore target, the domain XML
// <disk><source file> / <nvram> paths, and the SSH NVRAM write all point at the
// destination folder instead of the source server's paths. A remap also gates
// the restore behind guardVMRestoreDestination, so it can never write a
// multi-GB disk onto an unmounted path (the RAM rootfs) and brick the host
// (#122). An empty destBase restores each disk to the definition's path, from
// wherever the snapshot holds it.
//
// destZvolPool is the separate remap a block-device (zvol) disk needs: destBase
// is a filesystem path under the host mount and carries no ZFS pool
// information at all, so it cannot rebase a zvol's `zfs receive` target the
// way it rebases a file-backed disk's path. When destBase is non-empty (a
// cross-instance restore) and the domain has recognizable zvol disks,
// destZvolPool must be supplied: every such disk's dataset is rebased onto it
// via virshcli.RebaseZvolDatasetPool, so `zfs receive` lands on the
// destination box's pool rather than the source box's. An empty destZvolPool
// on a cross-instance restore with zvol disks refuses here, before any
// restic/SSH work starts (a destination pool cannot be guessed from destBase),
// rather than silently attempting `zfs receive` against the source pool's name
// and failing deep inside that call once the restore is already underway.
// Ignored (never validated) for a same-instance restore or a VM with no zvol
// disks; a same-instance zvol restore derives its target from SourceDataset.
func (s *Service) prepareRestoreVMForTarget(ctx context.Context, ref repoRef, name, snapshotID string, tg store.VMTarget, id entryIdentity, destBase, destZvolPool string) (vmRestorePlan, error) {
	explicitID := snapshotID != "latest" && snapshotID != ""

	// "latest" (or empty) resolves to the VM's newest snapshot. An explicit id
	// must be one id owns, listed against the caller's repo ref, the same
	// access check the container restores make.
	snaps, snapErr := s.snapshotsOwnedBy(ctx, ref.repo, ref.mode, id)
	if snapErr != nil {
		return vmRestorePlan{}, snapErr
	}
	var snap *restic.Snapshot
	if explicitID {
		if snap = chosenSnapshot(snaps, snapshotID); snap == nil {
			return vmRestorePlan{}, notInListing{snapshotID, "vm"}
		}
	} else {
		if len(snaps) == 0 {
			return vmRestorePlan{}, errors.New("no backups found for this vm")
		}
		snap = &snaps[len(snaps)-1]
		snapshotID = snap.ID
	}

	// Resolve this run's "vmrun:<runID>" group: every snapshot one backup
	// produced, the main file-backed one plus one per zvol disk (see
	// VMBackupDeps.RunTag). The tag is read off the snapshot resolved above,
	// so one more listing over the same repo ref finds the group.
	//
	// vmrunGroup stays nil when that snapshot carries no "vmrun:" tag (see
	// vmRunTag), and vmRestoreBlockDisks then leaves every entry's
	// SnapshotID/StdinPath at their zero values.
	var vmrunGroup []restic.Snapshot
	if runTag := vmRunTag(snaps, snapshotID); runTag != "" {
		group, gErr := s.snapshotsForTag(ctx, ref.repo, ref.mode, runTag)
		if gErr != nil {
			return vmRestorePlan{}, gErr
		}
		vmrunGroup = group
	}

	if tg.Definition == "" {
		return vmRestorePlan{}, errors.New("no stored definition for this vm: run a backup once first")
	}
	var def vmDefinition
	if err := json.Unmarshal([]byte(tg.Definition), &def); err != nil {
		return vmRestorePlan{}, fmt.Errorf("restore vm: unmarshal definition: %w", err)
	}

	// Disks must live within the Host Data mount, which is how restic reaches
	// them. The backup refuses a VM with a disk outside it and refuses it whole,
	// so the restore refuses too: leaving the disk out and defining the VM from
	// an XML that still lists it hands back a machine with a missing or stale
	// disk and says nothing.
	for _, p := range def.DiskPaths {
		if !paths.Within(s.cfg.HostMountRoot, p) {
			return vmRestorePlan{}, destinationRefusal("disk %s in this backup is not under your Host Data mount (%s) and cannot be restored. Move it under that mount, or restore this VM to another instance with a destination folder", p, s.cfg.HostSourceRoot)
		}
	}
	diskPaths := def.DiskPaths
	if len(diskPaths) == 0 {
		return vmRestorePlan{}, errors.New("no restorable disk paths found in this backup")
	}
	sources, err := snapshotDiskSources(diskPaths, snap.Paths)
	if err != nil {
		return vmRestorePlan{}, err
	}

	domainXML := def.DomainXML
	nvramHostPath := def.NVRAMHostPath
	var restoreDirs []backup.VMRestoreDir

	// A snapshot from before a rename holds the disks in the old folder. Any
	// folder list replaces the in-place restore, so it names every disk's
	// folder, an unmoved one onto itself. A snapshot folder restored into two
	// folders would copy all of its disks into both, over any file of the same
	// name there.
	if destBase == "" && !slices.Equal(sources, diskPaths) {
		targets := make(map[string]string, len(diskPaths))
		for i, cp := range diskPaths {
			src, dst := path.Dir(sources[i]), path.Dir(cp)
			switch t, seen := targets[src]; {
			case !seen:
				targets[src] = dst
				restoreDirs = append(restoreDirs, backup.VMRestoreDir{Subtree: src, Target: dst})
			case t != dst:
				return vmRestorePlan{}, destinationRefusal("the snapshot folder %s holds disks that now sit in %s and in %s, and restoring it into both would copy all of its disks into each; move those disks into one folder or pick another snapshot", s.toHostPath(src), s.toHostPath(t), s.toHostPath(dst))
			}
		}
	}

	// Remap for a cross-instance restore: place every disk under
	// <destBase>/<name>/ on the destination pool, rewrite the domain XML disk
	// and nvram sources to match, and guard the destination so the restore
	// can never fill an unmounted path (the RAM rootfs) and brick the host
	// (#122). An empty destBase is the same-instance restore and skips all of
	// this (disks return to their own paths, XML verbatim).
	if destBase != "" {
		destDir := path.Join(path.Clean(destBase), name) // container path
		destHostDir := s.toHostPath(destDir)             // host path for the domain XML
		diskRemap := make(map[string]string, len(diskPaths))
		seenDir := map[string]bool{}
		remapped := make([]string, 0, len(diskPaths))
		// Every disk lands in one folder here, so two disks that share a file
		// name would land on each other. The same-instance path refuses the
		// mirror image of this a few lines up.
		takenBy := make(map[string]string, len(diskPaths))
		for i, cp := range diskPaths {
			base := path.Base(cp)
			if other, taken := takenBy[base]; taken {
				return vmRestorePlan{}, destinationRefusal("this VM has two disks called %s, %s and %s, and both would be restored into %s. Rename one of them in the VM, or restore to a destination that keeps their folders apart", base, s.toHostPath(other), s.toHostPath(cp), destHostDir)
			}
			takenBy[base] = cp
			remapped = append(remapped, destDir+"/"+base)
			diskRemap[s.toHostPath(cp)] = destHostDir + "/" + base
			if src := path.Dir(sources[i]); !seenDir[src] {
				seenDir[src] = true
				restoreDirs = append(restoreDirs, backup.VMRestoreDir{Subtree: src, Target: destDir})
			}
		}
		diskPaths = remapped
		domainXML = virshcli.RewriteDiskSources(domainXML, diskRemap)
		if nvramHostPath != "" {
			newNVRAM := destHostDir + "/" + path.Base(nvramHostPath)
			domainXML = virshcli.RewriteNVRAM(domainXML, newNVRAM)
			nvramHostPath = newNVRAM
		}
		// Host-brick guard: prove the destination is on a real mounted pool with
		// room before any restic write. On failure nothing is written.
		if err := s.guardVMRestoreDestination(ctx, ref, snapshotID, destDir); err != nil {
			return vmRestorePlan{}, err
		}
	}

	// Make UEFI domains bootable even if the captured NVRAM is absent: add a
	// template= to <nvram> so libvirt regenerates it from the OVMF master. When
	// NVRAM bytes were captured, PreDefine writes them back over SSH first, so
	// libvirt uses the real var store (boot entries preserved).
	domainXML = virshcli.EnsureNVRAMTemplate(domainXML)

	// Re-derive the TPM path and block-device (zvol) disk list from the
	// (possibly remapped) domain XML, since vmDefinition does not store them
	// (see its TPMBytes field), as BackupVM derives them at backup time via
	// the same virshcli.ParseDomain call. The destBase remap above does not
	// affect either: RewriteDiskSources only rewrites <source file=...>
	// (file-backed disks); a <tpm> element and a <source dev=...>
	// (block-device disk) are left untouched. A zvol disk's dataset, as
	// opposed to the XML's device path (which keeps the source box's value
	// for the operator's reference), is rebased separately below, in the
	// cross-instance zvol rebase after the SSH guard.
	var tpmPath string
	var vmRestoreBlockDisks []backup.VMRestoreBlockDisk
	if parsed, perr := virshcli.ParseDomain(domainXML); perr == nil {
		tpmPath = parsed.TPMPath
		for _, bd := range parsed.BlockDisks {
			dataset, ok := virshcli.ZvolDatasetFromDevPath(bd.Source)
			if !ok {
				log.Printf("api: RestoreVM: WARN block-device disk %q (%s) is not a recognizable ZFS zvol for %q, so it will not be restored", bd.Dev, bd.Source, name) //nolint:gosec // G706: name is %q-quoted
				continue
			}
			rbd := backup.VMRestoreBlockDisk{SourceDataset: dataset}
			// Resolve this disk's own restic snapshot from the vmrun: group above.
			// Its identity tag is "vm:"+name+":zvol:"+bd.Dev (see VMBlockDisk.Dev in
			// internal/backup/vm_orchestrator.go): find the group member carrying
			// exactly that tag and take its snapshot id plus the one path it
			// recorded, which is the StdinPath BackupZvolDisk gave restic
			// (BackupStdin backs up one synthetic file per invocation, see
			// ZvolStdinPath, so Paths has exactly one entry when set at all).
			//
			// Left at zero value, the permanent fallback (see vmRunTag), when there
			// is no group or no member carries this disk's tag (an empty bd.Dev
			// never comes from the real BackupVM caller): RestoreZvolDisk then fails
			// loudly on the empty snapshot id rather than silently skipping the disk.
			if bd.Dev != "" {
				if gs, ok := vmrunGroupSnapshot(vmrunGroup, "vm:"+name+":zvol:"+bd.Dev); ok && len(gs.Paths) > 0 {
					rbd.SnapshotID = gs.ID
					rbd.StdinPath = gs.Paths[0]
				}
			}
			vmRestoreBlockDisks = append(vmRestoreBlockDisks, rbd)
		}
	} else {
		log.Printf("api: RestoreVM: WARN could not re-parse domain xml for %q (%v), so TPM state and any zvol disks will not be restored", name, perr) //nolint:gosec // G706: name is %q-quoted
	}

	// Mirrors BackupVM's "zvol backup requires SSH" guard: RestoreZvolDisk
	// calls ZFSHost.StreamReceive, which sshZFSHost forwards to s.ssh, and a
	// nil HostSSH there is a nil interface method call, an unrecovered panic
	// deep inside the async restore goroutine (StartRestoreVM) that would
	// crash the whole process rather than fail this one restore. Caught here,
	// before a plan is returned.
	if len(vmRestoreBlockDisks) > 0 && s.ssh == nil {
		return vmRestorePlan{}, fmt.Errorf("restore vm: %q has block-device (zvol) disks but no SSH host connection is configured, and zvol restore requires SSH", name)
	}

	// Cross-instance zvol rebase: destBase remaps a file-backed disk onto the
	// destination's filesystem path, but that path carries no ZFS pool
	// information, and RestoreZvolDisk would otherwise derive its `zfs
	// receive` target from SourceDataset (the source box's pool, re-derived
	// unchanged from the domain XML above), which almost certainly does not
	// exist on the destination box, and fail deep inside `zfs receive` once
	// the restore is underway. Refuse here, before any restic or SSH work,
	// when destZvolPool was not supplied; when it was, rebase every zvol
	// disk's dataset onto it now so the plan's blockDisks carry the
	// destination target.
	//
	// destZvolPool is trimmed before the emptiness check so a whitespace-only
	// value (a stray space in a direct API call) gets this clear refusal
	// rather than RebaseZvolDatasetPool's less actionable error below, which
	// would read it as empty and fail the "no segment past its own pool"
	// check instead of naming the real problem.
	if destBase != "" && len(vmRestoreBlockDisks) > 0 {
		if strings.TrimSpace(destZvolPool) == "" {
			// The web UI has no field for zvolPool (Recovery page), so an operator
			// hitting this through the UI cannot act on it in the app, and the
			// message names the only path there is: a direct API call. Keep it in
			// sync with docs/vm-backup-ssh-setup.md's TrueNAS section, which carries
			// the same guidance for someone reading ahead of time.
			return vmRestorePlan{}, fmt.Errorf("restore vm: %q has %d TrueNAS zvol-backed disk(s) and this is a cross-instance restore, but no destination ZFS pool was specified, and the destination pool cannot be inferred from the chosen destination folder. There is no web UI field for this yet: call POST /api/foreign/restore directly with its zvolPool parameter set to the destination pool name (see docs/vm-backup-ssh-setup.md's TrueNAS section), or restore this VM on the instance it was backed up from", name, len(vmRestoreBlockDisks))
		}
		for i := range vmRestoreBlockDisks {
			rebased, ok := virshcli.RebaseZvolDatasetPool(vmRestoreBlockDisks[i].SourceDataset, destZvolPool)
			if !ok {
				return vmRestorePlan{}, &zvolRebaseErr{msg: fmt.Sprintf("restore vm: %q: cannot rebase zvol dataset %q onto destination pool %q", name, vmRestoreBlockDisks[i].SourceDataset, destZvolPool)}
			}
			vmRestoreBlockDisks[i].RestoreBaseDataset = rebased
		}
	}

	// preDefine writes the captured NVRAM and TPM state back to the host over
	// SSH after the old domain is undefined (which removes its nvram) and
	// before `virsh define`, so the restored VM boots with its original UEFI
	// variables and vTPM state. It writes to the (possibly remapped)
	// destination nvram path and the re-derived TPM path. No-op for either
	// when there is nothing to write or SSH is unavailable.
	var preDefine func(context.Context) error
	if s.ssh != nil && ((len(def.NVRAMBytes) > 0 && nvramHostPath != "") || (len(def.TPMBytes) > 0 && tpmPath != "")) {
		writeNVRAMPath := nvramHostPath
		writeTPMPath := tpmPath
		preDefine = func(ctx context.Context) error {
			if len(def.NVRAMBytes) > 0 && writeNVRAMPath != "" {
				if err := s.ssh.WriteFile(ctx, writeNVRAMPath, def.NVRAMBytes); err != nil {
					log.Printf("api: RestoreVM: WARN NVRAM write over SSH failed for %q (%v); the VM is restored and will boot, but libvirt regenerates the UEFI variables from the firmware template, so boot entries may need to be re-added", name, err) //nolint:gosec // G706: name is %q-quoted
				}
			}
			if len(def.TPMBytes) > 0 && writeTPMPath != "" {
				if err := s.ssh.WriteFile(ctx, writeTPMPath, def.TPMBytes); err != nil {
					log.Printf("api: RestoreVM: WARN TPM state write over SSH failed for %q (%v); the VM is restored and will boot, but the vTPM starts fresh/empty instead of restoring its captured state", name, err) //nolint:gosec // G706: name is %q-quoted
				}
			}
			return nil // never block the restore on NVRAM/TPM; the firmware-template fallback keeps the VM bootable
		}
	}

	// Pin the host key before the orchestrator's virsh-over-SSH calls.
	if s.ssh != nil {
		if err := s.ssh.EnsureKnownHost(ctx); err != nil {
			return vmRestorePlan{}, fmt.Errorf("restore vm: ssh: %w", err)
		}
	}

	return vmRestorePlan{
		repo:         ref.repo,
		mode:         ref.mode,
		targetID:     tg.ID,
		snapshotID:   snapshotID,
		diskPaths:    diskPaths,
		domainXML:    domainXML,
		wasAutostart: def.WasAutostart,
		restoreDirs:  restoreDirs,
		wasRunning:   def.WasRunning,
		preDefine:    preDefine,
		blockDisks:   vmRestoreBlockDisks,
	}, nil
}

// snapshotDiskSources returns the path the snapshot holds each disk under: the
// disk's own path, or else the one file of the same name, which is where a
// snapshot from before a rename that moved the disk folder keeps it. A disk the
// snapshot lacks keeps its own path when another disk of its folder matched
// there, because the snapshot then predates the disk rather than a move.
func snapshotDiskSources(disks, snapshotPaths []string) ([]string, error) {
	byName := make(map[string][]string, len(snapshotPaths))
	for _, p := range snapshotPaths {
		byName[path.Base(p)] = append(byName[path.Base(p)], p)
	}
	diskNames := make(map[string]int, len(disks))
	inPlace := make(map[string]bool, len(disks))
	for _, d := range disks {
		diskNames[path.Base(d)]++
		if slices.Contains(snapshotPaths, d) {
			inPlace[path.Dir(d)] = true
		}
	}
	sources := make([]string, len(disks))
	for i, d := range disks {
		name := path.Base(d)
		held := byName[name]
		switch {
		case slices.Contains(held, d), len(held) == 0 && inPlace[path.Dir(d)]:
			sources[i] = d
		case len(held) == 0:
			return nil, fmt.Errorf("this snapshot has no disk named %s; pick a snapshot that includes it", name)
		case len(held) > 1 || diskNames[name] > 1:
			return nil, fmt.Errorf("the disk name %s is not unique, so this snapshot's copy of it cannot be matched; pick another snapshot or give the disks distinct names", name)
		default:
			sources[i] = held[0]
		}
	}
	return sources, nil
}

// guardVMRestoreDestination is the host-brick guard for a remapped
// (cross-instance) VM restore (#122): it proves the destination directory
// is safe to write a multi-GB disk image into before restic writes
// anything. Two checks:
//
//  1. destDir must sit on a real mounted pool or share, a mount point that
//     is a proper descendant of HostMountRoot (destinationMounted, shared
//     with #120). Otherwise a source path like /mnt/zfs/domains maps to an
//     unmounted dir on the destination, /mnt lives on the RAM rootfs, and
//     restic writes the image into tmpfs; OOM then kills emhttpd and nginx
//     and the host is bricked.
//  2. There must be enough free space for the restore. The size comes from
//     the source snapshot's restore-size (a read of the source repo); the
//     free-space probe runs on the nearest existing ancestor of destDir. A
//     probe error counts as "cannot prove insufficient" and does not block
//     (the mount check is the primary defence); only a proven shortfall
//     aborts.
func (s *Service) guardVMRestoreDestination(ctx context.Context, ref repoRef, snapshotID, destDir string) error {
	if !s.destinationMounted(destDir) {
		return destinationRefusal("restore destination %q is not on a mounted pool or share; a VM disk restored there would be written into the host's RAM and crash it. Choose a destination folder on real storage and retry", s.toHostPath(destDir))
	}
	_, wantBytes, err := s.engine.StatsRestoreSize(ctx, ref.repo, snapshotID, ref.mode)
	if err != nil {
		return fmt.Errorf("restore preflight: measure restore size: %w", err)
	}
	if wantBytes > 0 {
		if free, ferr := s.diskFreeFn()(nearestExistingDir(destDir)); ferr == nil && free < uint64(wantBytes) {
			return destinationRefusal("not enough free space to restore this VM: it needs %d bytes but the destination %q has only %d free. Free up space or choose another destination", wantBytes, s.toHostPath(destDir), free)
		}
	}
	return nil
}

// executeRestoreVM drives the long-running (destructive) part of a VM restore
// described by an already-validated plan, publishing "vm:<name>" progress. The
// orchestrator records the run (kindRestore) itself.
func (s *Service) executeRestoreVM(ctx context.Context, name string, plan vmRestorePlan, leaveStopped bool) error {
	// Hold the domain repo lock for the whole restic and libvirt phase,
	// including the destination pre-create below: the scheduler calls BackupVM
	// directly, bypassing batchActive, and the domain lock is the layer
	// scheduled jobs respect (see executeRestore).
	unlock := s.lockDomainFor("vms", "restore")
	defer unlock()
	// restic leaves a subtree target it creates root:root/0700, as in a container
	// restore (#125), whether the target is another pool or the folder a rename
	// moved the disks to. Pre-create each one readable; healRestoreDirOwnership
	// restores owner and mode after a successful restore.
	for _, rd := range plan.restoreDirs {
		if err := paths.EnsureDirReadable(rd.Target); err != nil {
			return fmt.Errorf("restore: prepare destination %q: %w", s.toHostPath(rd.Target), err)
		}
	}
	rkey := "vm:" + name
	rctx, startedAt := s.progBegin(ctx, rkey, "restore")
	rerr := backup.RestoreVM(rctx, backup.VMRestoreDeps{
		Confirmed:    true, // prepareRestoreVM rejected unconfirmed requests
		Name:         name,
		SnapshotID:   plan.snapshotID,
		DiskPaths:    plan.diskPaths,
		RestoreDirs:  plan.restoreDirs,
		DomainXML:    plan.domainXML,
		WasAutostart: plan.wasAutostart,
		// Boot after restore iff the VM was running when backed up (nil = a
		// backup without recorded state, so boot) and the restore didn't ask to
		// leave it stopped.
		StartAfter: (plan.wasRunning == nil || *plan.wasRunning) && !leaveStopped,
		PreDefine:  plan.preDefine,
		RepoPath:   plan.repo,
		TargetID:   plan.targetID,
		DataDir:    s.cfg.DataDir,
		VM:         s.virsh,
		Restic:     &resticAdapter{engine: s.engine, mode: plan.mode},
		Runs:       runsAdapter{st: s.store, ctx: ctx},
		BlockDisks: plan.blockDisks,
		ZFSHost:    sshZFSHost{ssh: s.ssh},
		ZvolRestic: &resticZvolAdapter{engine: s.engine, mode: plan.mode},
	})
	if rerr == nil {
		s.healRestoreDirOwnership(rctx, plan.repo, plan.snapshotID, plan.mode, plan.restoreDirs)
	}
	s.progEnd(rkey, "restore", rerr == nil, startedAt)
	return rerr
}

// StartRestoreVM launches a VM restore in a background goroutine and
// returns immediately, mirroring StartRestore for the VM domain (a VM disk
// restore can run for hours, far past any browser or proxy idle timeout).
// All validation runs synchronously (a bad request fails right away, no
// goroutine); progress is published under "vm:<name>" and the orchestrator
// records the run.
//
// Shares batchActive with backups and the other restores; returns (false, nil)
// when one is already running.
func (s *Service) StartRestoreVM(ctx context.Context, name, snapshotID, source string, leaveStopped bool) (bool, error) {
	if !s.batchActive.CompareAndSwap(false, true) {
		return false, nil
	}
	plan, err := s.prepareRestoreVM(ctx, name, snapshotID, true, source)
	if err != nil {
		s.batchActive.Store(false)
		return false, err
	}
	bctx := context.WithoutCancel(ctx)
	key := "vm:" + name // the exact progBegin key executeRestoreVM publishes under
	go func() {
		defer s.recoverOperation("restore vm: "+name, nil, func(msg string) {
			s.failStuckRun(plan.targetID, msg)
		})
		defer s.batchActive.Store(false)
		tctx, tcancel := context.WithTimeout(bctx, restoreTimeout)
		defer tcancel()
		rctx, cancel := context.WithCancel(tctx)
		defer cancel()
		s.registerCancel(key, cancel)
		defer s.unregisterCancel(key)
		if rerr := s.executeRestoreVM(rctx, name, plan, leaveStopped); rerr != nil {
			log.Printf("api: restore vm: %q failed: %v", name, rerr) //nolint:gosec // G706: name is %q-quoted
		}
	}()
	return true, nil
}

// VMSSHInfo returns the libvirt SSH host and BombVault's public key for the user
// to authorize on the Unraid host (Settings → VM Backup). Errors when SSH is not
// wired (no key yet).
func (s *Service) VMSSHInfo() (host, publicKey string, err error) {
	if s.ssh == nil {
		return "", "", errors.New("vm backup over SSH is not configured")
	}
	pub, err := s.ssh.PublicKey()
	if err != nil {
		return "", "", err
	}
	return s.cfg.LibvirtHost, pub, nil
}

// VMSSHTest checks that libvirt is reachable over SSH (used by the Settings
// "Test connection" button). Bounded by a timeout so an unreachable host
// (e.g. a macvlan container with no route) fails fast instead of hanging.
func (s *Service) VMSSHTest(ctx context.Context) error {
	if s.ssh == nil {
		return errors.New("vm backup over SSH is not configured")
	}
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	if err := s.ssh.EnsureKnownHost(ctx); err != nil {
		return err // SSH, auth or reachability problem; clearer than libvirt's error
	}
	if err := s.ssh.Test(ctx); err != nil {
		// EnsureKnownHost passed, so SSH auth and reachability are fine and only
		// libvirt is missing. Say so, so a notifications-only user (who needs the
		// SSH connection but not libvirt) isn't misled into thinking their SSH is
		// broken (#53).
		return fmt.Errorf("%w. The SSH connection itself is working, and libvirt is only needed for VM backups, not for Unraid notifications", err)
	}
	return nil
}

// LibvirtReachable reports whether libvirt is reachable over SSH, for the
// host-integration check's best-effort libvirt probe. Bounded by a timeout
// so a hung SSH attempt can't stall the check.
func (s *Service) LibvirtReachable() error {
	if s.ssh == nil {
		return errors.New("vm backup over SSH is not configured")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()
	if err := s.ssh.EnsureKnownHost(ctx); err != nil {
		return err
	}
	return s.ssh.Test(ctx)
}

// SnapshotsVM lists the restic snapshots a VM's identity owns: those under its
// "vm:<name>" tag plus each alias's from before that alias was linked.
func (s *Service) SnapshotsVM(ctx context.Context, name, source string) ([]restic.Snapshot, error) {
	return s.vmSnapshotsOf(ctx, name, source, s.vmIdentity(name))
}

// vmSnapshotsOf is containerSnapshotsOf for VMs.
func (s *Service) vmSnapshotsOf(ctx context.Context, name, source string, id entryIdentity) ([]restic.Snapshot, error) {
	settings, err := s.store.GetSettings()
	if err != nil {
		return nil, fmt.Errorf("read settings: %w", err)
	}
	repo, err := s.vmRepoForName(settings, name, source)
	if err != nil {
		return nil, err
	}
	return s.snapshotsOwnedBy(ctx, repo, s.repoModeFor(settings, "vms", source, repo), id)
}
