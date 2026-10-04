package api

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/backup"
	"github.com/junkerderprovinz/bombvault/internal/nbd"
	"github.com/junkerderprovinz/bombvault/internal/notify"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
	"github.com/junkerderprovinz/bombvault/internal/virshcli"
)

// vmBlockEngine is what a changed-block backup needs from restic beyond
// ResticEngine. *restic.Restic has it; an engine without it keeps every VM on
// the classic backup.
type vmBlockEngine interface {
	BackupDirFrom(ctx context.Context, repo, dir, parent string, tags []string, m restic.Mode) (restic.Summary, error)
	LsTimes(ctx context.Context, repo, snapshotID, dirPath string, m restic.Mode) ([]restic.NodeTime, error)
}

// unixForwarder reaches a Unix socket on the host from the container.
// sshconn.Conn has it.
type unixForwarder interface {
	ForwardUnix(ctx context.Context, local, remote string) (func(), error)
}

var _ vmBlockEngine = (*restic.Restic)(nil)

// Reasons a VM with changed-block backups on is still backed up the classic
// way. The orchestrator adds BlocksReasonJobFailed.
const (
	blocksReasonVMOff       = "vm_off"
	blocksReasonDiskFormat  = "disk_format"
	blocksReasonZvol        = "zvol"
	blocksReasonNoSSH       = "no_ssh"
	blocksReasonUnsupported = "unsupported"
	blocksReasonNoSpace     = "no_space"
)

// minBlocksBudget is the least staging room a changed-block backup starts
// with; below it the VM is backed up the classic way.
const minBlocksBudget = 256 << 20

const remoteNBDSocketPrefix = "/tmp/bombvault-nbd-"

// vmBlockPlan is everything a changed-block backup of one VM needs, or the
// reason it cannot run.
type vmBlockPlan struct {
	reason string
	disks  []backup.BlocksDisk
	host   *vmBlockHost
	budget int64
}

// planBlockBackup decides whether the running VM can be read through the
// libvirt backup API and prepares that path.
func (s *Service) planBlockBackup(ctx context.Context, name string, domain virshcli.DomainInfo, diskPaths []string, zvols int) vmBlockPlan {
	bb, okVirsh := s.virsh.(virshcli.BlockBackups)
	_, okEngine := s.engine.(vmBlockEngine)
	fwd, okFwd := s.ssh.(unixForwarder)
	switch {
	case !okVirsh || !okEngine:
		return vmBlockPlan{reason: blocksReasonUnsupported}
	case zvols > 0:
		return vmBlockPlan{reason: blocksReasonZvol}
	case s.ssh == nil || !okFwd:
		return vmBlockPlan{reason: blocksReasonNoSSH}
	case len(diskPaths) != len(domain.Disks):
		return vmBlockPlan{reason: blocksReasonUnsupported}
	}
	for _, d := range domain.Disks {
		if d.Format != "qcow2" {
			return vmBlockPlan{reason: blocksReasonDiskFormat}
		}
	}
	// A paused guest still has its qemu process, which is what the backup
	// job talks to.
	if state, err := s.virsh.State(ctx, name); err != nil || (state != "running" && state != "paused") {
		return vmBlockPlan{reason: blocksReasonVMOff}
	}
	budget := int64(backup.DefaultBlocksBudget)
	if free, err := s.diskFreeFn()(s.cfg.DataDir); err == nil {
		budget = min(budget, int64(free/4)) //nolint:gosec // G115: free space fits
	}
	if budget < minBlocksBudget {
		return vmBlockPlan{reason: blocksReasonNoSpace}
	}

	disks := make([]backup.BlocksDisk, 0, len(domain.Disks))
	hostDisks := make(map[string]string, len(domain.Disks))
	for i, p := range diskPaths {
		d := domain.Disks[i]
		bd := backup.BlocksDisk{Dev: d.Dev, Path: p, Format: d.Format, Mode: 0o644}
		if fi, err := os.Stat(p); err == nil {
			bd.Mode = uint32(fi.Mode().Perm())
			if uid, gid, ok := dirOwner(fi); ok {
				bd.UID, bd.GID = uid, gid
			}
		}
		disks = append(disks, bd)
		hostDisks[d.Dev] = d.Source
	}
	sum := sha256.Sum256([]byte(name))
	short := hex.EncodeToString(sum[:6])
	return vmBlockPlan{
		disks:  disks,
		budget: budget,
		host: &vmBlockHost{
			BlockBackups: bb,
			ping:         s.virsh.GuestAgentPing,
			fwd:          fwd,
			hostDisks:    hostDisks,
			skip:         domain.SkipSnapshotDevs,
			localSock:    filepath.Join(s.cfg.DataDir, "vm-blocks", "nbd-"+short+".sock"),
			remoteSock:   remoteNBDSocketPrefix + short + ".sock",
		},
	}
}

// backupVMBlocks runs the changed-block backup. A *backup.BlocksUnavailableError
// means nothing was written and no run recorded.
func (s *Service) backupVMBlocks(ctx context.Context, name string, tg store.VMTarget, plan vmBlockPlan, repo string, mode restic.Mode, extraTags, formerNames []string, runs backup.Runs) (backup.VMBlocksResult, error) {
	var latest *backup.BlocksLatest
	snaps, err := s.snapshotsOwnedBy(ctx, repo, mode, s.vmIdentity(name))
	if err != nil {
		return backup.VMBlocksResult{}, fmt.Errorf("backup vm: list snapshots: %w", err)
	}
	if len(snaps) > 0 {
		last := snaps[len(snaps)-1]
		latest = &backup.BlocksLatest{ID: last.ID, Blocks: slices.Contains(last.Tags, backup.BlocksTag)}
	}
	if err := os.MkdirAll(filepath.Dir(plan.host.localSock), 0o700); err != nil {
		return backup.VMBlocksResult{}, &backup.BlocksUnavailableError{Reason: backup.BlocksReasonJobFailed, Err: err}
	}
	return backup.BackupVMBlocks(ctx, backup.VMBlocksDeps{
		Name:        name,
		FormerNames: formerNames,
		TargetID:    tg.ID,
		Disks:       plan.disks,
		RepoPath:    repo,
		StageDir:    filepath.Join(s.cfg.DataDir, "vm-blocks", tg.ID),
		Budget:      plan.budget,
		Latest:      latest,
		Host:        plan.host,
		Checkpoints: s.blockCheckpointFile(tg.ID),
		Restic:      &vmBlockRestic{engine: s.engine, mode: mode, extraTags: extraTags, svc: s},
		Runs:        runs,
		ThawFailed:  func(err error) { s.notifyGuestStillFrozen(context.WithoutCancel(ctx), name, err) },
	})
}

// notifyGuestStillFrozen reports a VM whose filesystems may still be frozen
// after a changed-block backup, which stops every write inside it until
// someone thaws them.
func (s *Service) notifyGuestStillFrozen(ctx context.Context, name string, err error) {
	c, cErr := s.NotifyConfig()
	if cErr != nil || !c.Active() {
		return
	}
	msg := fmt.Sprintf("BombVault froze the filesystems of VM %s for its backup and could not thaw them again (%v). Nothing inside the VM can write until they are thawed: run \"virsh domfsthaw %s\" on the host or restart the VM.", name, scrubError(err), name)
	notify.Send(ctx, c, "vms", notify.Event{Title: "BombVault", Message: msg, OK: false})
	if s.unraidGate(c.Unraid) {
		if e := s.sendUnraidNotify(ctx, "BombVault: VM "+name+" may still be frozen", msg, "alert"); e != nil {
			log.Printf("notify: unraid: %v", e)
		}
	}
}

// recordBlockRun stores how a VM with changed-block backups on was read.
func (s *Service) recordBlockRun(targetID, mode, reason string) {
	if err := s.store.RecordVMBlockBackupRun(targetID, mode, reason, time.Now().Unix()); err != nil {
		log.Printf("api: backup vm: record read mode: %v", err)
	}
}

// SetVMBlockBackup turns changed-block backups on or off for a VM. Turning
// them off removes BombVault's checkpoints from the VM.
func (s *Service) SetVMBlockBackup(ctx context.Context, name string, on bool) error {
	tg, err := s.store.GetVMTargetByName(name)
	if err != nil {
		if tg, err = s.store.UpsertVMTarget(store.VMTarget{Name: name, Method: "graceful"}); err != nil {
			return fmt.Errorf("ensure vm target: %w", err)
		}
	}
	if err := s.store.SetVMBlockBackupEnabled(tg.ID, on); err != nil {
		return err
	}
	// A backup of the VM that is running now drops the chain itself when it
	// finds the switch off.
	if !on && !s.vmBackupRunning(name) {
		s.dropBlockCheckpoints(ctx, name)
	}
	return nil
}

// dropBlockCheckpointsIfOn removes the checkpoints of a VM whose
// changed-block backups are on, for when it leaves the backup.
func (s *Service) dropBlockCheckpointsIfOn(ctx context.Context, name string) {
	tg, err := s.store.GetVMTargetByName(name)
	if err != nil {
		return
	}
	if b, err := s.store.GetVMBlockBackup(tg.ID); err == nil && b.Enabled && !s.vmBackupRunning(name) {
		s.dropBlockCheckpoints(ctx, name)
	}
}

// dropBlockCheckpoints ends BombVault's changed-block chain on the VM: a job
// an interrupted run left, the kept checkpoint with its bitmap and every
// checkpoint of BombVault's still on record. Best-effort: a VM that is gone took them with it. libvirt
// deletes a bitmap only while the VM runs, so on a VM that is off the kept
// one stays in the image, unused.
func (s *Service) dropBlockCheckpoints(ctx context.Context, name string) {
	bb, ok := s.virsh.(virshcli.BlockBackups)
	if !ok {
		return
	}
	var kept blockCheckpointFile
	if tg, err := s.store.GetVMTargetByName(name); err == nil {
		kept = s.blockCheckpointFile(tg.ID)
		defer kept.clear()
	}
	if _, err := bb.CheckpointNames(ctx, name); err != nil {
		if !virshcli.IsNotFound(err) {
			log.Printf("api: vm %q: list checkpoints: %v", name, err) //nolint:gosec // G706: %q-quoted
		}
		return
	}
	if err := endLeftoverBlockJob(ctx, bb, name); err != nil {
		log.Printf("api: vm %q: %v", name, err) //nolint:gosec // G706: %q-quoted
	}
	if kept != "" {
		if def, err := kept.Load(); err == nil && def != "" {
			if err := bb.CheckpointRedefine(ctx, name, def, false); err != nil {
				log.Printf("api: vm %q: restore checkpoint to delete it: %v", name, err) //nolint:gosec // G706: %q-quoted
			}
		}
	}
	backup.SettleBlockCheckpoints(ctx, bb, kept, name, "")
}

// vmBackupRunning reports whether a backup of the VM is under way.
func (s *Service) vmBackupRunning(name string) bool {
	s.cancelMu.Lock()
	defer s.cancelMu.Unlock()
	_, ok := s.backupRuns["vm:"+name]
	return ok
}

// SweepBlockBackupLeftovers ends what changed-block backups left on their VMs
// when BombVault stopped in the middle of one: the backup job, which holds the
// disks, and the checkpoints on record, which keep the VM from being
// undefined. The kept checkpoint's bitmap stays for the next run.
func (s *Service) SweepBlockBackupLeftovers(ctx context.Context) {
	bb, ok := s.virsh.(virshcli.BlockBackups)
	if !ok {
		return
	}
	rows, err := s.store.ListVMBlockBackups()
	if err != nil {
		log.Printf("api: changed-block leftovers: %v", err)
		return
	}
	defer s.lockDomainFor("vms", "startup-sweep")()
	for id, row := range rows {
		if !row.Enabled {
			continue
		}
		tg, err := s.store.GetVMTargetByID(id)
		if err != nil {
			continue
		}
		if _, err := bb.CheckpointNames(ctx, tg.Name); err != nil {
			continue
		}
		if err := endLeftoverBlockJob(ctx, bb, tg.Name); err != nil {
			log.Printf("api: vm %q: %v", tg.Name, err) //nolint:gosec // G706: %q-quoted
		}
		kept := s.blockCheckpointFile(id)
		def, err := kept.Load()
		if err != nil {
			log.Printf("api: vm %q: read the kept checkpoint: %v", tg.Name, err) //nolint:gosec // G706: %q-quoted
		}
		backup.SettleBlockCheckpoints(ctx, bb, kept, tg.Name, backup.CheckpointName(def))
	}
}

// blockCheckpointFile keeps the definition of a VM's checkpoint between
// changed-block backups, one file per VM target.
type blockCheckpointFile string

var _ backup.CheckpointStore = blockCheckpointFile("")

func (s *Service) blockCheckpointFile(targetID string) blockCheckpointFile {
	return blockCheckpointFile(filepath.Join(s.cfg.DataDir, "vm-checkpoints", targetID+".xml"))
}

func (f blockCheckpointFile) Load() (string, error) {
	b, err := os.ReadFile(string(f))
	if errors.Is(err, fs.ErrNotExist) {
		return "", nil
	}
	return string(b), err
}

func (f blockCheckpointFile) Save(def string) error {
	if err := os.MkdirAll(filepath.Dir(string(f)), 0o700); err != nil {
		return err
	}
	tmp := string(f) + ".tmp"
	if err := os.WriteFile(tmp, []byte(def), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, string(f))
}

func (f blockCheckpointFile) clear() {
	if err := os.Remove(string(f)); err != nil && !errors.Is(err, fs.ErrNotExist) {
		log.Printf("api: remove kept checkpoint: %v", err)
	}
}

// vmBlockHost is backup.BlockHost over virsh and the SSH socket forward.
type vmBlockHost struct {
	virshcli.BlockBackups
	ping       func(ctx context.Context, name string) bool
	fwd        unixForwarder
	hostDisks  map[string]string
	skip       []string
	localSock  string
	remoteSock string
}

var _ backup.BlockHost = (*vmBlockHost)(nil)

func (h *vmBlockHost) GuestAgentPing(ctx context.Context, domain string) bool {
	return h.ping(ctx, domain)
}

func (h *vmBlockHost) EndLeftoverJob(ctx context.Context, domain string) error {
	return endLeftoverBlockJob(ctx, h.BlockBackups, domain)
}

// endLeftoverBlockJob ends a backup job an interrupted run left running. Only
// one that exports on BombVault's own socket is touched.
func endLeftoverBlockJob(ctx context.Context, bb virshcli.BlockBackups, domain string) error {
	job, err := bb.BackupJobXML(ctx, domain)
	if err != nil || job == "" || !strings.HasPrefix(virshcli.BackupJobSocket(job), remoteNBDSocketPrefix) {
		return nil
	}
	log.Printf("api: vm %q: ending a backup job left by an interrupted run", domain) //nolint:gosec // G706: %q-quoted
	if err := bb.AbortJob(ctx, domain); err != nil {
		return fmt.Errorf("end leftover backup job: %w", err)
	}
	return nil
}

// StartJob only starts the job, since the guest may be frozen while it runs.
// The socket forward follows with the first Open.
func (h *vmBlockHost) StartJob(ctx context.Context, domain, base, checkpoint string, devs []string) (backup.BlockJob, error) {
	disks := make([]virshcli.BackupDisk, 0, len(devs)+len(h.skip))
	for _, dev := range devs {
		disks = append(disks, virshcli.BackupDisk{Dev: dev, Scratch: h.hostDisks[dev] + ".bombvault-scratch"})
	}
	for _, dev := range h.skip {
		disks = append(disks, virshcli.BackupDisk{Dev: dev, Skip: true})
	}
	err := h.BackupBegin(ctx, domain, virshcli.PullBackupXML(h.remoteSock, base, disks), virshcli.CheckpointXML(checkpoint, disks))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", backup.ErrBlockJobRefused, err)
	}
	return &vmBlockJob{host: h, domain: domain, incremental: base != ""}, nil
}

type vmBlockJob struct {
	host        *vmBlockHost
	domain      string
	incremental bool
	stopForward func()
}

func (j *vmBlockJob) Open(ctx context.Context, dev string) (backup.BlockReader, error) {
	bitmap := "qemu:dirty-bitmap:backup-" + dev
	contexts := []string{nbd.AllocationContext}
	if j.incremental {
		contexts = append(contexts, bitmap)
	}
	if j.stopForward == nil {
		stop, err := j.host.fwd.ForwardUnix(ctx, j.host.localSock, j.host.remoteSock)
		if err != nil {
			return nil, err
		}
		j.stopForward = stop
	}
	c, err := nbd.Dial(ctx, "unix", j.host.localSock, dev, contexts)
	if err != nil {
		return nil, err
	}
	if j.incremental && !c.HasContext(bitmap) {
		_ = c.Close()
		return nil, fmt.Errorf("the export of %s has no dirty bitmap", dev)
	}
	return &nbdDisk{c: c, bitmap: bitmap}, nil
}

func (j *vmBlockJob) Stop(ctx context.Context) error {
	err := j.host.AbortJob(ctx, j.domain)
	if j.stopForward != nil {
		j.stopForward()
	}
	return err
}

// nbdDisk is backup.BlockReader over an NBD export.
type nbdDisk struct {
	c      *nbd.Client
	bitmap string
}

func (d *nbdDisk) Size() int64                             { return d.c.Size() }
func (d *nbdDisk) ReadAt(p []byte, off int64) (int, error) { return d.c.ReadAt(p, off) }
func (d *nbdDisk) Close() error                            { return d.c.Close() }

func (d *nbdDisk) Allocated() ([]backup.Range, error) {
	if !d.c.HasContext(nbd.AllocationContext) {
		return []backup.Range{{Off: 0, Len: d.c.Size()}}, nil
	}
	return d.ranges(nbd.AllocationContext, func(flags uint32) bool { return flags&nbd.StateZero == 0 })
}

func (d *nbdDisk) Changed() ([]backup.Range, error) {
	return d.ranges(d.bitmap, func(flags uint32) bool { return flags&nbd.StateDirty != 0 })
}

func (d *nbdDisk) ranges(context string, keep func(uint32) bool) ([]backup.Range, error) {
	exts, err := d.c.BlockStatus(context, 0, d.c.Size())
	if err != nil {
		return nil, err
	}
	var out []backup.Range
	for _, e := range exts {
		if !keep(e.Flags) {
			continue
		}
		if n := len(out); n > 0 && out[n-1].Off+out[n-1].Len == e.Offset {
			out[n-1].Len += e.Length
			continue
		}
		out = append(out, backup.Range{Off: e.Offset, Len: e.Length})
	}
	return out, nil
}

// vmBlockRestic is backup.BlockRestic and backup.BlockDumper over the
// restic engine.
type vmBlockRestic struct {
	engine ResticEngine
	mode   restic.Mode
	// extraTags go on every snapshot of the run, after the run's own.
	extraTags []string
	// svc clears stale locks when one stops a write. A remote home is not
	// unlocked before the run, so without it a lock left by a killed run
	// would fail every later one.
	svc *Service
}

// healed runs op, once more after clearing stale locks when svc is set and a
// lock was in the way.
func (r *vmBlockRestic) healed(ctx context.Context, repo string, op func() error) error {
	if r.svc == nil {
		return op()
	}
	return r.svc.retryAfterUnlock(ctx, repo, r.mode, op)
}

var (
	_ backup.BlockRestic = (*vmBlockRestic)(nil)
	_ backup.BlockDumper = (*vmBlockRestic)(nil)
)

func (r *vmBlockRestic) Manifest(ctx context.Context, repo, snapshotID string) (backup.BlocksManifest, error) {
	var buf bytes.Buffer
	if err := r.engine.DumpRaw(ctx, repo, snapshotID, "/"+backup.BlocksManifestName, &buf, r.mode); err != nil {
		return backup.BlocksManifest{}, err
	}
	return backup.ParseBlocksManifest(buf.Bytes())
}

func (r *vmBlockRestic) Nodes(ctx context.Context, repo, snapshotID, dir string) ([]backup.BlockNode, error) {
	nodes, err := r.engine.(vmBlockEngine).LsTimes(ctx, repo, snapshotID, dir, r.mode)
	if err != nil {
		return nil, err
	}
	var out []backup.BlockNode
	for _, n := range nodes {
		if n.Type == "file" && path.Dir(n.Path) == dir {
			out = append(out, backup.BlockNode{Name: path.Base(n.Path), Size: n.Size, Mtime: n.Mtime})
		}
	}
	return out, nil
}

func (r *vmBlockRestic) BackupDir(ctx context.Context, repo, dir, parent string, tags []string) (backup.BlockBackup, error) {
	var sum restic.Summary
	err := r.healed(ctx, repo, func() (err error) {
		sum, err = r.engine.(vmBlockEngine).BackupDirFrom(ctx, repo, dir, parent, withTags(tags, r.extraTags), r.mode)
		return err
	})
	if err != nil {
		return backup.BlockBackup{}, err
	}
	return backup.BlockBackup{
		Summary:    backupSummaryFrom(sum),
		New:        sum.FilesNew,
		Changed:    sum.FilesChanged,
		Unmodified: sum.FilesUnmodified,
	}, nil
}

func (r *vmBlockRestic) SnapshotIDsTagged(ctx context.Context, repo, tag string) ([]string, error) {
	snaps, err := r.engine.Snapshots(ctx, repo, r.mode)
	if err != nil {
		return nil, err
	}
	var ids []string
	for _, sn := range snaps {
		if slices.Contains(sn.Tags, tag) {
			ids = append(ids, sn.ID)
		}
	}
	return ids, nil
}

func (r *vmBlockRestic) Forget(ctx context.Context, repo string, ids []string) error {
	return r.healed(ctx, repo, func() error { return r.engine.Forget(ctx, repo, ids, false, r.mode) })
}

func (r *vmBlockRestic) DumpDir(ctx context.Context, repo, snapshotID, dir string, w io.Writer) error {
	return r.engine.DumpRaw(ctx, repo, snapshotID, dir, w, r.mode)
}

var imageFormatRe = regexp.MustCompile(`^[a-z0-9]+$`)

// qemuImgConvert writes the raw image at raw to target in format with the
// qemu-img the image ships.
func qemuImgConvert(ctx context.Context, raw, target, format string) error {
	if !imageFormatRe.MatchString(format) {
		return fmt.Errorf("unknown disk format %q", format)
	}
	out, err := exec.CommandContext(ctx, "qemu-img", "convert", "-q", "-f", "raw", "-O", format, raw, target).CombinedOutput() //nolint:gosec // G204: format checked above, paths built by the restore
	if err != nil {
		return fmt.Errorf("qemu-img: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

// blockRestoreImages maps the disks of a changed-block snapshot onto the
// disk files of the definition: by path, else by file name when a rename
// moved the folder. It returns the index into defDisks per manifest disk.
func blockRestoreImages(m backup.BlocksManifest, defDisks []string) ([]int, error) {
	idx := make([]int, len(m.Disks))
	for i, d := range m.Disks {
		j := slices.Index(defDisks, d.Path)
		if j < 0 {
			for k, p := range defDisks {
				if path.Base(p) != path.Base(d.Path) {
					continue
				}
				if j >= 0 {
					return nil, fmt.Errorf("the disk name %s is not unique, so this snapshot's copy of it cannot be matched", path.Base(d.Path))
				}
				j = k
			}
		}
		if j < 0 {
			return nil, fmt.Errorf("this VM has no disk %s any more; pick a later snapshot", path.Base(d.Path))
		}
		idx[i] = j
	}
	return idx, nil
}
