package backup

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/progress"
)

// A changed-block VM backup stores each disk as a folder of fixed-size
// segment files holding what the guest sees, next to a manifest. Every
// snapshot holds every segment, so each one restores on its own. Unchanged
// segments are empty placeholders with the size and mtime of their node in
// the previous snapshot; restic takes those over without reading them.
const (
	// BlocksTag marks a VM snapshot in the segment layout.
	BlocksTag = "blocks"
	// BlocksManifestName is the manifest at the snapshot's root.
	BlocksManifestName = "bombvault-vm.json"
	// BlocksCheckpointPrefix starts the name of every libvirt checkpoint
	// BombVault creates, so it never touches anyone else's.
	BlocksCheckpointPrefix = "bombvault-"

	blocksPartialTagPrefix = "blocks-partial:"
	minBlockSegment        = 8 << 20
	maxBlockSegments       = 16384
	// DefaultBlocksBudget caps the segment data staged for one restic run.
	DefaultBlocksBudget = 4 << 30
)

// Modes and reasons a VM backup reports for how it read the disks.
const (
	BlocksModeChanged = "changed"
	BlocksModeFull    = "full"
	BlocksModeClassic = "classic"

	BlocksReasonFirst            = "first"
	BlocksReasonNewerBackup      = "newer_backup"
	BlocksReasonNoCheckpoint     = "no_checkpoint"
	BlocksReasonDisksChanged     = "disks_changed"
	BlocksReasonCheckpointBroken = "checkpoint_broken"
	BlocksReasonChainBroken      = "chain_broken"
)

// BlocksPartialTag tags the intermediate snapshots of one VM's run.
func BlocksPartialTag(name string) string { return blocksPartialTagPrefix + name }

// BlockSegmentSize is the segment size for a disk of size bytes: 8 MiB,
// doubled until the disk needs no more than 16384 segments.
func BlockSegmentSize(size int64) int64 {
	seg := int64(minBlockSegment)
	for size > seg*maxBlockSegments {
		seg *= 2
	}
	return seg
}

// BlocksManifest describes the disks of a segment-layout snapshot.
type BlocksManifest struct {
	Version    int          `json:"version"`
	Checkpoint string       `json:"checkpoint"`
	Disks      []BlocksDisk `json:"disks"`
}

// BlocksDisk is one disk of a segment-layout snapshot. Path is the disk
// file's container path at backup time; Mode, UID and GID are that file's, so
// a restore can give them back.
type BlocksDisk struct {
	Dev     string `json:"dev"`
	Path    string `json:"path"`
	Format  string `json:"format"`
	Size    int64  `json:"size"`
	Segment int64  `json:"segment"`
	Mode    uint32 `json:"mode"`
	UID     int    `json:"uid"`
	GID     int    `json:"gid"`
}

// Range is a byte range of a disk.
type Range struct {
	Off int64
	Len int64
}

// BlockReader reads one disk from a running backup job at the job's point in
// time.
type BlockReader interface {
	Size() int64
	ReadAt(p []byte, off int64) (int, error)
	// Allocated lists the ranges that hold data; the rest reads as zeros.
	Allocated() ([]Range, error)
	// Changed lists the ranges written since the job's incremental base.
	Changed() ([]Range, error)
	Close() error
}

// BlockJob is a running pull-mode backup job.
type BlockJob interface {
	Open(ctx context.Context, dev string) (BlockReader, error)
	// Stop ends the job. The checkpoint it created stays.
	Stop(ctx context.Context) error
}

// CheckpointHost is libvirt's checkpoint bookkeeping.
type CheckpointHost interface {
	CheckpointNames(ctx context.Context, domain string) ([]string, error)
	// CheckpointDelete removes a checkpoint with its bitmap.
	CheckpointDelete(ctx context.Context, domain, checkpoint string) error
	// CheckpointForget removes only libvirt's record and keeps the bitmap.
	CheckpointForget(ctx context.Context, domain, checkpoint string) error
	CheckpointXML(ctx context.Context, domain, checkpoint string) (string, error)
	CheckpointRedefine(ctx context.Context, domain, checkpointXML string, validate bool) error
}

// CheckpointStore keeps the definition of the one checkpoint a VM's next
// changed-block backup builds on. Between runs libvirt holds only its bitmap:
// libvirt refuses to undefine a shut-off domain that has checkpoints, which
// would break a restore and the host's own "remove VM".
type CheckpointStore interface {
	// Load returns the kept definition, "" when there is none.
	Load() (string, error)
	Save(checkpointXML string) error
}

// CheckpointName reads the name out of a checkpoint definition.
func CheckpointName(checkpointXML string) string {
	var doc struct {
		Name string `xml:"name"`
	}
	if xml.Unmarshal([]byte(checkpointXML), &doc) != nil {
		return ""
	}
	return strings.TrimSpace(doc.Name)
}

// BlockHost is the libvirt side of a changed-block backup.
type BlockHost interface {
	CheckpointHost
	GuestAgentPing(ctx context.Context, domain string) bool
	FSFreeze(ctx context.Context, domain string) error
	FSThaw(ctx context.Context, domain string) error
	// EndLeftoverJob ends a backup job an interrupted run of BombVault's left
	// running, which would hold the disks. Anyone else's job stays.
	EndLeftoverJob(ctx context.Context, domain string) error
	// StartJob starts a pull-mode job over devs that creates checkpoint and,
	// with a non-empty base, reports the changes since base.
	StartJob(ctx context.Context, domain, base, checkpoint string, devs []string) (BlockJob, error)
}

// BlockNode is a file node of a snapshot directory.
type BlockNode struct {
	Name  string
	Size  int64
	Mtime time.Time
}

// BlockBackup is one restic run of the staging folder with the file counts
// the placeholder check needs.
type BlockBackup struct {
	Summary    Summary
	New        int
	Changed    int
	Unmodified int
}

// BlockRestic is the restic side of a changed-block backup.
type BlockRestic interface {
	Manifest(ctx context.Context, repo, snapshotID string) (BlocksManifest, error)
	// Nodes lists the files directly in dir of a snapshot.
	Nodes(ctx context.Context, repo, snapshotID, dir string) ([]BlockNode, error)
	// BackupDir backs up dir as the snapshot root on top of parent.
	BackupDir(ctx context.Context, repo, dir, parent string, tags []string) (BlockBackup, error)
	SnapshotIDsTagged(ctx context.Context, repo, tag string) ([]string, error)
	Forget(ctx context.Context, repo string, ids []string) error
}

// BlocksLatest is the newest snapshot of the VM.
type BlocksLatest struct {
	ID     string
	Blocks bool
}

// VMBlocksDeps bundles what BackupVMBlocks needs.
type VMBlocksDeps struct {
	Name        string
	FormerNames []string
	TargetID    string
	// Disks are the writable disks with Dev, Path, Format and the file mode
	// set; Size and Segment are read from the export.
	Disks    []BlocksDisk
	RepoPath string
	// StageDir is a folder only this VM's backups use.
	StageDir string
	Budget   int64
	// Latest is the VM's newest snapshot, nil when it has none.
	Latest *BlocksLatest
	Host   BlockHost
	// Checkpoints keeps the definition of the checkpoint between runs.
	Checkpoints CheckpointStore
	Restic      BlockRestic
	Runs        Runs
	Now         func() time.Time
	// ThawFailed hears about a guest whose filesystems stayed frozen after
	// every thaw attempt.
	ThawFailed func(err error)
}

// VMBlocksResult says how the disks were read.
type VMBlocksResult struct {
	Summary    Summary
	Mode       string
	Reason     string
	Checkpoint string
}

// BlocksUnavailableError means the changed-block path could not start and
// nothing was written; the caller backs the VM up the classic way instead.
type BlocksUnavailableError struct {
	Reason string
	Err    error
}

func (e *BlocksUnavailableError) Error() string {
	return fmt.Sprintf("changed-block backup unavailable (%s): %v", e.Reason, e.Err)
}

func (e *BlocksUnavailableError) Unwrap() error { return e.Err }

// ErrBlockJobRefused marks a StartJob error where libvirt refused the job
// itself, which for an incremental job points at its checkpoint.
var ErrBlockJobRefused = errors.New("libvirt refused the backup job")

// BlocksReasonJobFailed is the classic fallback's reason when libvirt would
// not start a backup job at all.
const BlocksReasonJobFailed = "job_failed"

// BackupVMBlocks backs up a running VM through a libvirt pull-mode backup
// job, reading only the segments changed since the previous backup's
// checkpoint when that chain is intact and every allocated block otherwise.
func BackupVMBlocks(ctx context.Context, d VMBlocksDeps) (VMBlocksResult, error) {
	if d.Now == nil {
		d.Now = time.Now
	}
	if d.Budget <= 0 {
		d.Budget = DefaultBlocksBudget
	}
	devs := make([]string, len(d.Disks))
	for i, disk := range d.Disks {
		devs[i] = disk.Dev
	}

	forgetPartials(ctx, d)

	names, err := d.Host.CheckpointNames(ctx, d.Name)
	if err != nil {
		return VMBlocksResult{}, &BlocksUnavailableError{Reason: BlocksReasonJobFailed, Err: err}
	}
	kept := recallCheckpoint(ctx, d, &names)
	base, prev, reason := chainBase(ctx, d, names)

	checkpoint := BlocksCheckpointPrefix + d.Now().UTC().Format("20060102150405")
	for slices.Contains(names, checkpoint) {
		checkpoint += "x"
	}

	cleanCtx := context.WithoutCancel(ctx)
	if err := d.Host.EndLeftoverJob(ctx, d.Name); err != nil {
		SettleBlockCheckpoints(cleanCtx, d.Host, d.Checkpoints, d.Name, kept)
		return VMBlocksResult{}, &BlocksUnavailableError{Reason: BlocksReasonJobFailed, Err: err}
	}
	job, err := startFrozen(ctx, d, base, checkpoint, devs)
	if err != nil && base != "" && errors.Is(err, ErrBlockJobRefused) {
		log.Printf("vm blocks: %q: incremental start from %s refused (%v); reading every block", d.Name, base, err)
		base, prev, reason = "", nil, BlocksReasonCheckpointBroken
		job, err = startFrozen(ctx, d, "", checkpoint, devs)
	}
	if err != nil {
		SettleBlockCheckpoints(cleanCtx, d.Host, d.Checkpoints, d.Name, kept)
		return VMBlocksResult{}, &BlocksUnavailableError{Reason: BlocksReasonJobFailed, Err: err}
	}

	runID, err := d.Runs.Start(d.TargetID, kindBackup)
	if err != nil {
		stopJob(ctx, d, job)
		SettleBlockCheckpoints(cleanCtx, d.Host, d.Checkpoints, d.Name, kept)
		return VMBlocksResult{}, fmt.Errorf("vm backup: record run start: %w", err)
	}

	res, runErr := runBlocks(ctx, d, job, base, prev, reason, checkpoint)
	stopJob(ctx, d, job)
	if err := os.RemoveAll(d.StageDir); err != nil {
		log.Printf("vm blocks: %q: remove staging folder: %v", d.Name, err)
	}
	forgetPartials(cleanCtx, d)
	if runErr != nil {
		SettleBlockCheckpoints(cleanCtx, d.Host, d.Checkpoints, d.Name, kept)
		_ = d.Runs.Finish(runID, statusFailed, Summary{}, truncateErr(runErr))
		return VMBlocksResult{}, runErr
	}
	// Only the checkpoint the new snapshot was read at is needed from here.
	SettleBlockCheckpoints(cleanCtx, d.Host, d.Checkpoints, d.Name, checkpoint)
	if err := d.Runs.Finish(runID, statusSuccess, res.Summary, ""); err != nil {
		return res, fmt.Errorf("vm backup: record run finish: %w", err)
	}
	return res, nil
}

// recallCheckpoint gives libvirt back the kept checkpoint when it does not
// know it, adding it to names, and returns its name. A bitmap that is gone
// or broken makes libvirt refuse, and the run reads every block.
func recallCheckpoint(ctx context.Context, d VMBlocksDeps, names *[]string) string {
	def, err := d.Checkpoints.Load()
	if err != nil {
		log.Printf("vm blocks: %q: read the kept checkpoint: %v", d.Name, err)
		return ""
	}
	name := CheckpointName(def)
	if name == "" || slices.Contains(*names, name) {
		return name
	}
	if err := d.Host.CheckpointRedefine(ctx, d.Name, def, true); err != nil {
		log.Printf("vm blocks: %q: checkpoint %s cannot be restored (%v); reading every block", d.Name, name, err)
		return ""
	}
	*names = append(*names, name)
	return name
}

// SettleBlockCheckpoints leaves the domain without any of BombVault's
// checkpoints on record. Every one but keep goes with its bitmap; keep's
// definition goes to store and only its record is removed, so its bitmap
// keeps counting changes for the next run. An empty keep drops them all.
func SettleBlockCheckpoints(ctx context.Context, host CheckpointHost, store CheckpointStore, domain, keep string) {
	names, err := host.CheckpointNames(ctx, domain)
	if err != nil {
		log.Printf("vm blocks: %q: list checkpoints: %v", domain, err)
		return
	}
	for _, n := range names {
		if strings.HasPrefix(n, BlocksCheckpointPrefix) && n != keep {
			dropCheckpoint(ctx, host, domain, n)
		}
	}
	if keep == "" || !slices.Contains(names, keep) {
		return
	}
	def, err := host.CheckpointXML(ctx, domain, keep)
	if err == nil {
		err = store.Save(def)
	}
	if err != nil {
		log.Printf("vm blocks: %q: keep checkpoint %s: %v; the next run reads every block", domain, keep, err)
		dropCheckpoint(ctx, host, domain, keep)
		return
	}
	if err := host.CheckpointForget(ctx, domain, keep); err != nil {
		log.Printf("vm blocks: %q: forget checkpoint %s: %v", domain, keep, err)
	}
}

// chainBase decides whether this run can build on the newest snapshot. It
// returns the checkpoint to read changes from with that snapshot's manifest,
// or a reason for a full read.
func chainBase(ctx context.Context, d VMBlocksDeps, names []string) (string, *chainPrev, string) {
	switch {
	case d.Latest == nil:
		return "", nil, BlocksReasonFirst
	case !d.Latest.Blocks:
		return "", nil, BlocksReasonNewerBackup
	}
	m, err := d.Restic.Manifest(ctx, d.RepoPath, d.Latest.ID)
	if err != nil {
		log.Printf("vm blocks: %q: read manifest of %s: %v", d.Name, d.Latest.ID, err)
		return "", nil, BlocksReasonChainBroken
	}
	if m.Checkpoint == "" || !slices.Contains(names, m.Checkpoint) {
		return "", nil, BlocksReasonNoCheckpoint
	}
	if len(m.Disks) != len(d.Disks) {
		return "", nil, BlocksReasonDisksChanged
	}
	for i, disk := range d.Disks {
		if m.Disks[i].Dev != disk.Dev {
			return "", nil, BlocksReasonDisksChanged
		}
	}
	return m.Checkpoint, &chainPrev{snapshot: d.Latest.ID, manifest: m}, ""
}

type chainPrev struct {
	snapshot string
	manifest BlocksManifest
}

// startFrozen starts the job with the guest's filesystems frozen when its
// agent answers. The job fixes its point in time when it starts, so the
// guest is thawed right after and nothing else happens in between.
func startFrozen(ctx context.Context, d VMBlocksDeps, base, checkpoint string, devs []string) (BlockJob, error) {
	frozen := false
	if d.Host.GuestAgentPing(ctx, d.Name) {
		if err := d.Host.FSFreeze(ctx, d.Name); err != nil {
			log.Printf("vm blocks: %q: freeze failed (%v); the backup is crash-consistent", d.Name, err)
		} else {
			frozen = true
		}
	}
	job, err := d.Host.StartJob(ctx, d.Name, base, checkpoint, devs)
	if frozen {
		thaw(context.WithoutCancel(ctx), d)
	}
	return job, err
}

// thawAttempts and thawRetryDelay bound how long a failing thaw is retried:
// five attempts over about fifteen seconds, the wait doubling each time.
const thawAttempts = 5

var thawRetryDelay = time.Second

// thaw thaws the guest, retrying a failure, since a guest left frozen stops
// writing altogether. The backup goes on either way.
func thaw(ctx context.Context, d VMBlocksDeps) {
	wait := thawRetryDelay
	for attempt := 1; ; attempt++ {
		err := d.Host.FSThaw(ctx, d.Name)
		if err == nil {
			return
		}
		if attempt == thawAttempts {
			log.Printf("vm blocks: %q: thaw failed %d times, the guest may still be frozen: %v", d.Name, attempt, err)
			if d.ThawFailed != nil {
				d.ThawFailed(err)
			}
			return
		}
		log.Printf("vm blocks: %q: thaw failed (%v); trying again", d.Name, err)
		time.Sleep(wait)
		wait *= 2
	}
}

func stopJob(ctx context.Context, d VMBlocksDeps, job BlockJob) {
	if err := job.Stop(context.WithoutCancel(ctx)); err != nil {
		log.Printf("vm blocks: %q: stop backup job: %v", d.Name, err)
	}
}

// dropCheckpoint deletes a checkpoint with its bitmap. libvirt refuses that
// when the bitmap is already gone or the domain is off, and then at least
// its record goes.
func dropCheckpoint(ctx context.Context, host CheckpointHost, domain, checkpoint string) {
	err := host.CheckpointDelete(ctx, domain, checkpoint)
	if err == nil {
		return
	}
	if fErr := host.CheckpointForget(ctx, domain, checkpoint); fErr != nil {
		log.Printf("vm blocks: %q: delete checkpoint %s: %v; forget it: %v", domain, checkpoint, err, fErr)
	}
}

func forgetPartials(ctx context.Context, d VMBlocksDeps) {
	ids, err := d.Restic.SnapshotIDsTagged(ctx, d.RepoPath, BlocksPartialTag(d.Name))
	if err != nil {
		log.Printf("vm blocks: %q: list intermediate snapshots: %v", d.Name, err)
		return
	}
	if len(ids) == 0 {
		return
	}
	if err := d.Restic.Forget(ctx, d.RepoPath, ids); err != nil {
		log.Printf("vm blocks: %q: forget intermediate snapshots: %v", d.Name, err)
	}
}

// segRef is one segment of one disk.
type segRef struct {
	disk int
	idx  int64
}

// blockDisk is a disk open for reading during the run.
type blockDisk struct {
	meta     BlocksDisk
	reader   BlockReader
	alloc    []Range
	count    int64
	parent   map[int64]time.Time
	fileSize func(idx int64) int64
}

func runBlocks(ctx context.Context, d VMBlocksDeps, job BlockJob, base string, prev *chainPrev, reason, checkpoint string) (VMBlocksResult, error) {
	disks := make([]*blockDisk, 0, len(d.Disks))
	defer func() {
		for _, bd := range disks {
			_ = bd.reader.Close()
		}
	}()
	for _, meta := range d.Disks {
		r, err := job.Open(ctx, meta.Dev)
		if err != nil {
			return VMBlocksResult{}, fmt.Errorf("vm backup: open disk %s: %w", meta.Dev, err)
		}
		bd := &blockDisk{meta: meta, reader: r}
		disks = append(disks, bd)
		bd.meta.Size = r.Size()
		bd.meta.Segment = BlockSegmentSize(bd.meta.Size)
		bd.count = (bd.meta.Size + bd.meta.Segment - 1) / bd.meta.Segment
		size, seg := bd.meta.Size, bd.meta.Segment
		bd.fileSize = func(idx int64) int64 { return min(seg, size-idx*seg) }
		if bd.alloc, err = r.Allocated(); err != nil {
			return VMBlocksResult{}, fmt.Errorf("vm backup: allocation of disk %s: %w", meta.Dev, err)
		}
	}

	mode := BlocksModeFull
	if prev != nil {
		mode = BlocksModeChanged
		for i, bd := range disks {
			pd := prev.manifest.Disks[i]
			if pd.Size != bd.meta.Size || pd.Segment != bd.meta.Segment {
				mode, reason, prev = BlocksModeFull, BlocksReasonDisksChanged, nil
				break
			}
		}
	}
	if prev != nil {
		for _, bd := range disks {
			nodes, err := d.Restic.Nodes(ctx, d.RepoPath, prev.snapshot, "/"+bd.meta.Dev)
			if err != nil || !loadParent(bd, nodes) {
				log.Printf("vm blocks: %q: segments of %s in %s do not match (%v); reading every block", d.Name, bd.meta.Dev, prev.snapshot, err)
				mode, reason, prev = BlocksModeFull, BlocksReasonChainBroken, nil
				break
			}
		}
	}

	manifest := BlocksManifest{Version: 1, Checkpoint: checkpoint}
	for _, bd := range disks {
		manifest.Disks = append(manifest.Disks, bd.meta)
	}

	parentID := ""
	var floor int64
	var todo []segRef
	if prev != nil {
		parentID = prev.snapshot
		// The manifest is rewritten every run and must be read every run, so
		// its mtime has to move past the one it had.
		root, err := d.Restic.Nodes(ctx, d.RepoPath, prev.snapshot, "/")
		if err != nil {
			return VMBlocksResult{}, fmt.Errorf("vm backup: list %s: %w", prev.snapshot, err)
		}
		for _, n := range root {
			if n.Name == BlocksManifestName {
				floor = n.Mtime.Unix() + 1
			}
		}
		for i, bd := range disks {
			changed, err := bd.reader.Changed()
			if err != nil {
				return VMBlocksResult{}, fmt.Errorf("vm backup: changes of disk %s: %w", bd.meta.Dev, err)
			}
			todo = append(todo, segmentsTouched(i, bd.meta.Segment, bd.count, changed)...)
		}
	} else {
		for _, bd := range disks {
			bd.parent = map[int64]time.Time{}
		}
		for i, bd := range disks {
			for idx := range bd.count {
				todo = append(todo, segRef{disk: i, idx: idx})
			}
		}
	}

	sum, err := stageAndBackup(ctx, d, disks, manifest, parentID, floor, todo)
	if errors.Is(err, errPlaceholderRead) && prev != nil {
		log.Printf("vm blocks: %q: restic read a placeholder of %s; reading every block", d.Name, prev.snapshot)
		mode, reason = BlocksModeFull, BlocksReasonChainBroken
		todo = todo[:0]
		for i, bd := range disks {
			bd.parent = map[int64]time.Time{}
			for idx := range bd.count {
				todo = append(todo, segRef{disk: i, idx: idx})
			}
		}
		sum, err = stageAndBackup(ctx, d, disks, manifest, "", 0, todo)
	}
	if err != nil {
		return VMBlocksResult{}, err
	}
	return VMBlocksResult{Summary: sum, Mode: mode, Reason: reason, Checkpoint: checkpoint}, nil
}

// loadParent fills bd.parent from the previous snapshot's segment nodes and
// reports whether they are exactly the segments this disk has now.
func loadParent(bd *blockDisk, nodes []BlockNode) bool {
	bd.parent = make(map[int64]time.Time, bd.count)
	for _, n := range nodes {
		idx, ok := segmentIndex(n.Name)
		if !ok || idx >= bd.count || n.Size != bd.fileSize(idx) {
			return false
		}
		bd.parent[idx] = n.Mtime
	}
	return int64(len(bd.parent)) == bd.count
}

func segmentName(idx int64) string { return fmt.Sprintf("%08d", idx) }

func segmentIndex(name string) (int64, bool) {
	if len(name) != 8 {
		return 0, false
	}
	var idx int64
	for _, c := range name {
		if c < '0' || c > '9' {
			return 0, false
		}
		idx = idx*10 + int64(c-'0')
	}
	return idx, true
}

// segmentsTouched lists the segments of one disk that any of ranges touches,
// in order and without repeats.
func segmentsTouched(disk int, seg, count int64, ranges []Range) []segRef {
	var out []segRef
	last := int64(-1)
	for _, r := range ranges {
		if r.Len <= 0 {
			continue
		}
		first, end := r.Off/seg, (r.Off+r.Len-1)/seg
		for idx := max(first, last+1); idx <= end && idx < count; idx++ {
			out = append(out, segRef{disk: disk, idx: idx})
			last = idx
		}
	}
	return out
}

var errPlaceholderRead = errors.New("restic read a placeholder segment")

// stageAndBackup writes the segments in todo in batches that fit the budget
// and backs the staging folder up after each, every run on top of the last.
// Only the last run's snapshot carries the VM's tags. Segments written now get
// an mtime past every one in the parent and past floor; the manifest gets a
// later one per run, whole seconds apart, so no file system's timestamp
// precision can make two of them equal.
func stageAndBackup(ctx context.Context, d VMBlocksDeps, disks []*blockDisk, manifest BlocksManifest, parentID string, floor int64, todo []segRef) (Summary, error) {
	if err := os.RemoveAll(d.StageDir); err != nil {
		return Summary{}, fmt.Errorf("vm backup: clear staging folder: %w", err)
	}
	stamp := max(d.Now().Unix(), floor)
	placeholders := 0
	for _, bd := range disks {
		dir := filepath.Join(d.StageDir, bd.meta.Dev)
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return Summary{}, fmt.Errorf("vm backup: staging folder: %w", err)
		}
		for idx, mt := range bd.parent {
			stamp = max(stamp, mt.Unix()+1)
			if err := placeholder(filepath.Join(dir, segmentName(idx)), bd.fileSize(idx), mt); err != nil {
				return Summary{}, err
			}
			placeholders++
		}
	}
	mtime := time.Unix(stamp, 0)

	var total int64
	costs := make([]int64, len(todo))
	for i, s := range todo {
		costs[i] = allocatedIn(disks[s.disk].alloc, s.idx*disks[s.disk].meta.Segment, disks[s.disk].fileSize(s.idx))
		total += costs[i]
	}
	outer := progress.SinkFrom(ctx)
	var done int64
	report := func(pct float64, batch int64) {
		if outer != nil && total > 0 {
			outer((float64(done) + pct/100*float64(batch)) / float64(total) * 100)
		}
	}

	tags := withFormerNames([]string{"vm:" + d.Name, "p2", BlocksTag}, d.FormerNames)
	var bytesAdded int64
	var final Summary
	batchNo := 0
	for start := 0; start < len(todo) || batchNo == 0; batchNo++ {
		end, cost := start, int64(0)
		for end < len(todo) && (end == start || cost+costs[end] <= d.Budget) {
			cost += costs[end]
			end++
		}
		batch := todo[start:end]
		reused := placeholders
		for _, s := range batch {
			bd := disks[s.disk]
			if _, ok := bd.parent[s.idx]; ok {
				reused--
			}
			if err := writeSegment(ctx, bd, filepath.Join(d.StageDir, bd.meta.Dev, segmentName(s.idx)), s.idx, mtime); err != nil {
				return Summary{}, err
			}
		}
		if err := writeManifest(filepath.Join(d.StageDir, BlocksManifestName), manifest, time.Unix(stamp+int64(batchNo)+1, 0)); err != nil {
			return Summary{}, err
		}

		last := end == len(todo)
		runTags := tags
		if !last {
			runTags = []string{BlocksPartialTag(d.Name)}
		}
		bctx := progress.WithSink(ctx, func(p float64) { report(p, cost) })
		bk, err := d.Restic.BackupDir(bctx, d.RepoPath, d.StageDir, parentID, runTags)
		if err != nil {
			return Summary{}, fmt.Errorf("vm backup: restic: %w", err)
		}
		if bk.Unmodified != reused || bk.New+bk.Changed != len(batch)+1 {
			if fErr := d.Restic.Forget(context.WithoutCancel(ctx), d.RepoPath, []string{bk.Summary.SnapshotID}); fErr != nil {
				log.Printf("vm blocks: %q: forget snapshot %s: %v", d.Name, bk.Summary.SnapshotID, fErr)
			}
			log.Printf("vm blocks: %q: expected %d unchanged and %d written files, restic reported %d unchanged, %d new, %d changed", d.Name, reused, len(batch)+1, bk.Unmodified, bk.New, bk.Changed)
			return Summary{}, errPlaceholderRead
		}
		bytesAdded += bk.Summary.Bytes
		final = bk.Summary
		parentID = bk.Summary.SnapshotID
		done += cost

		// The written segments become placeholders for the next run on top of
		// this snapshot, which frees their data again.
		for _, s := range batch {
			bd := disks[s.disk]
			if _, ok := bd.parent[s.idx]; !ok {
				placeholders++
			}
			bd.parent[s.idx] = mtime
			if err := placeholder(filepath.Join(d.StageDir, bd.meta.Dev, segmentName(s.idx)), bd.fileSize(s.idx), mtime); err != nil {
				return Summary{}, err
			}
		}
		start = end
	}
	final.Bytes = bytesAdded
	return final, nil
}

// placeholder makes path an empty sparse file of size bytes with mtime.
func placeholder(path string, size int64, mtime time.Time) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600) //nolint:gosec // G304: a path inside BombVault's own staging folder
	if err != nil {
		return fmt.Errorf("vm backup: placeholder: %w", err)
	}
	err = f.Truncate(size)
	if cErr := f.Close(); err == nil {
		err = cErr
	}
	if err == nil {
		err = os.Chtimes(path, mtime, mtime)
	}
	if err != nil {
		return fmt.Errorf("vm backup: placeholder: %w", err)
	}
	return nil
}

// allocatedIn is how many bytes of [off, off+n) the ranges cover.
func allocatedIn(alloc []Range, off, n int64) int64 {
	var sum int64
	for _, r := range alloc {
		lo, hi := max(r.Off, off), min(r.Off+r.Len, off+n)
		if hi > lo {
			sum += hi - lo
		}
	}
	return sum
}

const blockIOSize = 4 << 20

// writeSegment copies segment idx of the disk into path, reading only its
// allocated ranges and leaving zeros as holes.
func writeSegment(ctx context.Context, bd *blockDisk, path string, idx int64, mtime time.Time) error {
	off, n := idx*bd.meta.Segment, bd.fileSize(idx)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600) //nolint:gosec // G304: a path inside BombVault's own staging folder
	if err != nil {
		return fmt.Errorf("vm backup: stage segment: %w", err)
	}
	werr := func() error {
		if err := f.Truncate(n); err != nil {
			return err
		}
		buf := make([]byte, blockIOSize)
		for _, r := range bd.alloc {
			lo, hi := max(r.Off, off), min(r.Off+r.Len, off+n)
			for pos := lo; pos < hi; {
				if err := ctx.Err(); err != nil {
					return err
				}
				chunk := buf[:min(int64(len(buf)), hi-pos)]
				if _, err := bd.reader.ReadAt(chunk, pos); err != nil {
					return fmt.Errorf("read disk %s at %d: %w", bd.meta.Dev, pos, err)
				}
				if err := writeSparse(f, chunk, pos-off); err != nil {
					return err
				}
				pos += int64(len(chunk))
			}
		}
		return nil
	}()
	if cErr := f.Close(); werr == nil {
		werr = cErr
	}
	if werr == nil {
		werr = os.Chtimes(path, mtime, mtime)
	}
	if werr != nil {
		return fmt.Errorf("vm backup: stage segment %s/%s: %w", bd.meta.Dev, segmentName(idx), werr)
	}
	return nil
}

const sparseBlock = 64 << 10

// writeSparse writes p at off, skipping blocks of zeros so they stay holes.
func writeSparse(f *os.File, p []byte, off int64) error {
	for i := 0; i < len(p); i += sparseBlock {
		b := p[i:min(i+sparseBlock, len(p))]
		if allZero(b) {
			continue
		}
		if _, err := f.WriteAt(b, off+int64(i)); err != nil {
			return err
		}
	}
	return nil
}

func allZero(b []byte) bool {
	for _, c := range b {
		if c != 0 {
			return false
		}
	}
	return true
}
