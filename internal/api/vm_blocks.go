package api

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
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
			virsh:      bb,
			ping:       s.virsh.GuestAgentPing,
			fwd:        fwd,
			hostDisks:  hostDisks,
			skip:       domain.SkipSnapshotDevs,
			localSock:  filepath.Join(s.cfg.DataDir, "vm-blocks", "nbd-"+short+".sock"),
			remoteSock: remoteNBDSocketPrefix + short + ".sock",
		},
	}
}

// backupVMBlocks runs the changed-block backup. A *backup.BlocksUnavailableError
// means nothing was written and no run recorded.
func (s *Service) backupVMBlocks(ctx context.Context, name string, tg store.VMTarget, plan vmBlockPlan, repo string, mode restic.Mode, formerNames []string, runs backup.Runs) (backup.VMBlocksResult, error) {
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
		Restic:      &vmBlockRestic{engine: s.engine, mode: mode},
		Runs:        runs,
	})
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
	if !on {
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
	if b, err := s.store.GetVMBlockBackup(tg.ID); err == nil && b.Enabled {
		s.dropBlockCheckpoints(ctx, name)
	}
}

// dropBlockCheckpoints deletes every checkpoint BombVault created on the VM.
// Best-effort: a VM that is gone took its checkpoints with it.
func (s *Service) dropBlockCheckpoints(ctx context.Context, name string) {
	bb, ok := s.virsh.(virshcli.BlockBackups)
	if !ok {
		return
	}
	names, err := bb.CheckpointNames(ctx, name)
	if err != nil {
		if !virshcli.IsNotFound(err) {
			log.Printf("api: vm %q: list checkpoints: %v", name, err) //nolint:gosec // G706: %q-quoted
		}
		return
	}
	for _, n := range names {
		if !strings.HasPrefix(n, backup.BlocksCheckpointPrefix) {
			continue
		}
		if err := bb.CheckpointDelete(ctx, name, n); err != nil {
			log.Printf("api: vm %q: delete checkpoint %s: %v", name, n, err) //nolint:gosec // G706: %q-quoted
		}
	}
}

// vmBlockHost is backup.BlockHost over virsh and the SSH socket forward.
type vmBlockHost struct {
	virsh      virshcli.BlockBackups
	ping       func(ctx context.Context, name string) bool
	fwd        unixForwarder
	hostDisks  map[string]string
	skip       []string
	localSock  string
	remoteSock string
}

var _ backup.BlockHost = (*vmBlockHost)(nil)

func (h *vmBlockHost) CheckpointNames(ctx context.Context, domain string) ([]string, error) {
	return h.virsh.CheckpointNames(ctx, domain)
}

func (h *vmBlockHost) CheckpointDelete(ctx context.Context, domain, checkpoint string) error {
	return h.virsh.CheckpointDelete(ctx, domain, checkpoint)
}

func (h *vmBlockHost) GuestAgentPing(ctx context.Context, domain string) bool {
	return h.ping(ctx, domain)
}

func (h *vmBlockHost) FSFreeze(ctx context.Context, domain string) error {
	return h.virsh.FSFreeze(ctx, domain)
}

func (h *vmBlockHost) FSThaw(ctx context.Context, domain string) error {
	return h.virsh.FSThaw(ctx, domain)
}

func (h *vmBlockHost) StartJob(ctx context.Context, domain, base, checkpoint string, devs []string) (backup.BlockJob, error) {
	// A job left running by an interrupted backup holds the disks; only one
	// that exports on BombVault's own socket is ended here.
	if job, err := h.virsh.BackupJobXML(ctx, domain); err == nil && job != "" {
		if strings.HasPrefix(virshcli.BackupJobSocket(job), remoteNBDSocketPrefix) {
			log.Printf("api: vm %q: ending a backup job left by an interrupted run", domain) //nolint:gosec // G706: %q-quoted
			if aErr := h.virsh.AbortJob(ctx, domain); aErr != nil {
				return nil, fmt.Errorf("end leftover backup job: %w", aErr)
			}
		}
	}
	disks := make([]virshcli.BackupDisk, 0, len(devs)+len(h.skip))
	for _, dev := range devs {
		disks = append(disks, virshcli.BackupDisk{Dev: dev, Scratch: h.hostDisks[dev] + ".bombvault-scratch"})
	}
	for _, dev := range h.skip {
		disks = append(disks, virshcli.BackupDisk{Dev: dev, Skip: true})
	}
	err := h.virsh.BackupBegin(ctx, domain, virshcli.PullBackupXML(h.remoteSock, base, disks), virshcli.CheckpointXML(checkpoint, disks))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", backup.ErrBlockJobRefused, err)
	}
	stop, err := h.fwd.ForwardUnix(ctx, h.localSock, h.remoteSock)
	if err != nil {
		cctx := context.WithoutCancel(ctx)
		_ = h.virsh.AbortJob(cctx, domain)
		_ = h.virsh.CheckpointDelete(cctx, domain, checkpoint)
		return nil, err
	}
	return &vmBlockJob{host: h, domain: domain, incremental: base != "", stopForward: stop}, nil
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
	err := j.host.virsh.AbortJob(ctx, j.domain)
	j.stopForward()
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
	sum, err := r.engine.(vmBlockEngine).BackupDirFrom(ctx, repo, dir, parent, tags, r.mode)
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
	return r.engine.Forget(ctx, repo, ids, false, r.mode)
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
