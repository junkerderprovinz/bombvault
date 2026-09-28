package backup_test

import (
	"archive/tar"
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/backup"
)

const blockGrain = 64 << 10

// blockVM is a running VM with one disk whose writes qemu tracks in one
// bitmap per checkpoint. order is libvirt's record of the checkpoints, kept
// apart from the bitmaps the way checkpoint-delete --metadata separates them.
type blockVM struct {
	disk        []byte
	checkpoints map[string]map[int64]bool
	order       []string
	refuseIncr  bool
	refuseAll   bool
	jobs        int
	bytesRead   int64
}

func newBlockVM(size int) *blockVM {
	return &blockVM{disk: make([]byte, size), checkpoints: map[string]map[int64]bool{}}
}

func (v *blockVM) write(off int, p []byte) {
	copy(v.disk[off:], p)
	for _, bm := range v.checkpoints {
		for g := int64(off) / blockGrain; g <= int64(off+len(p)-1)/blockGrain; g++ {
			bm[g] = true
		}
	}
}

func (v *blockVM) CheckpointNames(context.Context, string) ([]string, error) {
	return slices.Clone(v.order), nil
}

// bitmaps lists the checkpoint bitmaps in the image, sorted.
func (v *blockVM) bitmaps() []string {
	var out []string
	for n := range v.checkpoints {
		out = append(out, n)
	}
	slices.Sort(out)
	return out
}

func (v *blockVM) CheckpointDelete(_ context.Context, _, name string) error {
	if !slices.Contains(v.order, name) {
		return errors.New("no domain checkpoint with matching name")
	}
	if _, ok := v.checkpoints[name]; !ok {
		return fmt.Errorf("bitmap %q not found in backing chain", name)
	}
	delete(v.checkpoints, name)
	return v.CheckpointForget(context.Background(), "", name)
}

func (v *blockVM) CheckpointForget(_ context.Context, _, name string) error {
	v.order = slices.DeleteFunc(v.order, func(n string) bool { return n == name })
	return nil
}

func (v *blockVM) CheckpointXML(_ context.Context, _, name string) (string, error) {
	if !slices.Contains(v.order, name) {
		return "", errors.New("no domain checkpoint with matching name")
	}
	return "<domaincheckpoint><name>" + name + "</name></domaincheckpoint>", nil
}

func (v *blockVM) CheckpointRedefine(_ context.Context, _, def string, validate bool) error {
	name := backup.CheckpointName(def)
	if slices.Contains(v.order, name) {
		return errors.New("checkpoint already exists")
	}
	if _, ok := v.checkpoints[name]; !ok && validate {
		return fmt.Errorf("checkpoint inconsistent: missing or broken bitmap %q", name)
	}
	v.order = append(v.order, name)
	return nil
}

func (v *blockVM) GuestAgentPing(context.Context, string) bool { return false }
func (v *blockVM) FSFreeze(context.Context, string) error      { return nil }
func (v *blockVM) FSThaw(context.Context, string) error        { return nil }

func (v *blockVM) StartJob(_ context.Context, _, base, checkpoint string, _ []string) (backup.BlockJob, error) {
	if v.refuseAll || (base != "" && v.refuseIncr) {
		return nil, fmt.Errorf("%w: bitmap is inconsistent", backup.ErrBlockJobRefused)
	}
	var changed []backup.Range
	if base != "" {
		bm, ok := v.checkpoints[base]
		if !ok || !slices.Contains(v.order, base) {
			return nil, errors.New("no such checkpoint")
		}
		var grains []int64
		for g := range bm {
			grains = append(grains, g)
		}
		slices.Sort(grains)
		for _, g := range grains {
			changed = append(changed, backup.Range{Off: g * blockGrain, Len: blockGrain})
		}
	}
	v.checkpoints[checkpoint] = map[int64]bool{}
	v.order = append(v.order, checkpoint)
	v.jobs++
	return &blockJob{vm: v, data: bytes.Clone(v.disk), changed: changed}, nil
}

type blockJob struct {
	vm      *blockVM
	data    []byte
	changed []backup.Range
}

func (j *blockJob) Open(context.Context, string) (backup.BlockReader, error) {
	return &blockReader{job: j}, nil
}
func (j *blockJob) Stop(context.Context) error { return nil }

type blockReader struct{ job *blockJob }

func (r *blockReader) Size() int64 { return int64(len(r.job.data)) }
func (r *blockReader) ReadAt(p []byte, off int64) (int, error) {
	r.job.vm.bytesRead += int64(len(p))
	return copy(p, r.job.data[off:]), nil
}
func (r *blockReader) Allocated() ([]backup.Range, error) {
	var out []backup.Range
	for off := 0; off < len(r.job.data); off += blockGrain {
		if !bytes.Equal(r.job.data[off:off+blockGrain], make([]byte, blockGrain)) {
			out = append(out, backup.Range{Off: int64(off), Len: blockGrain})
		}
	}
	return out, nil
}
func (r *blockReader) Changed() ([]backup.Range, error) { return r.job.changed, nil }
func (r *blockReader) Close() error                     { return nil }

// fakeBlockRepo keeps snapshots of a staging folder the way restic does: a
// file whose size and mtime match its node in the parent is taken over
// without being read.
type fakeBlockRepo struct {
	snaps            map[string]*blockSnap
	order            []string
	next             int
	ignoreParentOnce bool
	failBackup       bool
}

type blockSnap struct {
	tags  []string
	files map[string]blockFile
}

type blockFile struct {
	size  int64
	mtime time.Time
	data  []byte
}

func newFakeBlockRepo() *fakeBlockRepo { return &fakeBlockRepo{snaps: map[string]*blockSnap{}} }

func (f *fakeBlockRepo) BackupDir(_ context.Context, _, dir, parent string, tags []string) (backup.BlockBackup, error) {
	if f.failBackup {
		return backup.BlockBackup{}, errors.New("repository is full")
	}
	var p *blockSnap
	if parent != "" {
		p = f.snaps[parent]
		if p == nil {
			return backup.BlockBackup{}, errors.New("no parent")
		}
	}
	ignore := f.ignoreParentOnce
	f.ignoreParentOnce = false
	snap := &blockSnap{tags: slices.Clone(tags), files: map[string]blockFile{}}
	var out backup.BlockBackup
	err := filepath.WalkDir(dir, func(path string, e fs.DirEntry, err error) error {
		if err != nil || e.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(dir, path)
		rel = filepath.ToSlash(rel)
		fi, err := e.Info()
		if err != nil {
			return err
		}
		if p != nil && !ignore {
			if old, ok := p.files[rel]; ok && old.size == fi.Size() && old.mtime.Equal(fi.ModTime()) {
				snap.files[rel] = old
				out.Unmodified++
				return nil
			}
		}
		data, err := os.ReadFile(path) //nolint:gosec // a temp file of the test
		if err != nil {
			return err
		}
		snap.files[rel] = blockFile{size: fi.Size(), mtime: fi.ModTime(), data: data}
		out.Summary.Bytes += int64(len(data))
		if _, ok := p.filesOf()[rel]; ok {
			out.Changed++
		} else {
			out.New++
		}
		return nil
	})
	if err != nil {
		return backup.BlockBackup{}, err
	}
	f.next++
	id := fmt.Sprintf("%064x", f.next)
	f.snaps[id] = snap
	f.order = append(f.order, id)
	out.Summary.SnapshotID = id
	return out, nil
}

func (s *blockSnap) filesOf() map[string]blockFile {
	if s == nil {
		return nil
	}
	return s.files
}

func (f *fakeBlockRepo) Manifest(_ context.Context, _, id string) (backup.BlocksManifest, error) {
	s := f.snaps[id]
	if s == nil {
		return backup.BlocksManifest{}, errors.New("no snapshot")
	}
	return backup.ParseBlocksManifest(s.files[backup.BlocksManifestName].data)
}

func (f *fakeBlockRepo) Nodes(_ context.Context, _, id, dir string) ([]backup.BlockNode, error) {
	var out []backup.BlockNode
	prefix := strings.Trim(dir, "/")
	if prefix != "" {
		prefix += "/"
	}
	for rel, file := range f.snaps[id].files {
		if name, ok := strings.CutPrefix(rel, prefix); ok && !strings.Contains(name, "/") {
			out = append(out, backup.BlockNode{Name: name, Size: file.size, Mtime: file.mtime})
		}
	}
	return out, nil
}

func (f *fakeBlockRepo) SnapshotIDsTagged(_ context.Context, _, tag string) ([]string, error) {
	var ids []string
	for _, id := range f.order {
		if slices.Contains(f.snaps[id].tags, tag) {
			ids = append(ids, id)
		}
	}
	return ids, nil
}

func (f *fakeBlockRepo) Forget(_ context.Context, _ string, ids []string) error {
	for _, id := range ids {
		delete(f.snaps, id)
		f.order = slices.DeleteFunc(f.order, func(o string) bool { return o == id })
	}
	return nil
}

func (f *fakeBlockRepo) DumpDir(_ context.Context, _, id, dir string, w io.Writer) error {
	tw := tar.NewWriter(w)
	prefix := strings.TrimPrefix(dir, "/") + "/"
	var names []string
	for rel := range f.snaps[id].files {
		if strings.HasPrefix(rel, prefix) {
			names = append(names, rel)
		}
	}
	sort.Strings(names)
	for _, rel := range names {
		file := f.snaps[id].files[rel]
		if err := tw.WriteHeader(&tar.Header{Name: rel, Mode: 0o600, Size: file.size, Typeflag: tar.TypeReg}); err != nil {
			return err
		}
		data := file.data
		if data == nil {
			data = make([]byte, file.size)
		}
		if _, err := tw.Write(data); err != nil {
			return err
		}
	}
	return tw.Close()
}

// latest is the newest snapshot carrying the VM's own tag.
func (f *fakeBlockRepo) latest(name string) *backup.BlocksLatest {
	for i := len(f.order) - 1; i >= 0; i-- {
		s := f.snaps[f.order[i]]
		if slices.Contains(s.tags, "vm:"+name) {
			return &backup.BlocksLatest{ID: f.order[i], Blocks: slices.Contains(s.tags, backup.BlocksTag)}
		}
	}
	return nil
}

// keptCheckpoint is BombVault's own copy of the checkpoint definition.
type keptCheckpoint struct {
	def     string
	failing bool
}

func (k *keptCheckpoint) Load() (string, error) { return k.def, nil }

func (k *keptCheckpoint) Save(def string) error {
	if k.failing {
		return errors.New("disk full")
	}
	k.def = def
	return nil
}

type blockRig struct {
	t     *testing.T
	vm    *blockVM
	repo  *fakeBlockRepo
	runs  *fakeRuns
	kept  *keptCheckpoint
	stage string
	clock time.Time
}

func newBlockRig(t *testing.T, size int) *blockRig {
	return &blockRig{
		t:     t,
		vm:    newBlockVM(size),
		repo:  newFakeBlockRepo(),
		runs:  &fakeRuns{},
		kept:  &keptCheckpoint{},
		stage: filepath.Join(t.TempDir(), "stage"),
		clock: time.Unix(1_800_000_000, 0),
	}
}

func (r *blockRig) backup() (backup.VMBlocksResult, error) {
	r.clock = r.clock.Add(time.Hour)
	now := r.clock
	return backup.BackupVMBlocks(context.Background(), backup.VMBlocksDeps{
		Name:        "win",
		TargetID:    "t1",
		Disks:       []backup.BlocksDisk{{Dev: "vda", Path: "/host/user/domains/win/vdisk1.img", Format: "raw", Mode: 0o644}},
		RepoPath:    "/repo",
		StageDir:    r.stage,
		Budget:      8 << 20,
		Latest:      r.repo.latest("win"),
		Host:        r.vm,
		Checkpoints: r.kept,
		Restic:      r.repo,
		Runs:        r.runs,
		Now:         func() time.Time { return now },
	})
}

func (r *blockRig) mustBackup() backup.VMBlocksResult {
	r.t.Helper()
	res, err := r.backup()
	if err != nil {
		r.t.Fatalf("backup: %v", err)
	}
	return res
}

// restoredSum restores the disk of snapshot id and returns its checksum.
func (r *blockRig) restoredSum(id string) [32]byte {
	r.t.Helper()
	m, err := r.repo.Manifest(context.Background(), "/repo", id)
	if err != nil {
		r.t.Fatal(err)
	}
	target := filepath.Join(r.t.TempDir(), "vdisk1.img")
	if err := backup.RestoreBlockImage(context.Background(), r.repo, nil, "/repo", id, backup.VMRestoreImage{Disk: m.Disks[0], Target: target}); err != nil {
		r.t.Fatalf("restore: %v", err)
	}
	b, err := os.ReadFile(target) //nolint:gosec // a temp file of the test
	if err != nil {
		r.t.Fatal(err)
	}
	return sha256.Sum256(b)
}

func fill(n int, b byte) []byte { return bytes.Repeat([]byte{b}, n) }

func TestBlocksRestoreEqualsDiskAfterIncrementalRuns(t *testing.T) {
	r := newBlockRig(t, 40<<20)
	r.vm.write(1<<20, fill(3<<20, 0x11))
	r.vm.write(30<<20, fill(100, 0x22))

	first := r.mustBackup()
	if first.Mode != backup.BlocksModeFull || first.Reason != backup.BlocksReasonFirst {
		t.Fatalf("first run = %s/%s, want a full read", first.Mode, first.Reason)
	}
	sums := map[string][32]byte{first.Summary.SnapshotID: sha256.Sum256(r.vm.disk)}

	r.vm.write(17<<20, fill(4096, 0x33))
	r.vm.bytesRead = 0
	second := r.mustBackup()
	if second.Mode != backup.BlocksModeChanged {
		t.Fatalf("second run = %s/%s, want changed blocks", second.Mode, second.Reason)
	}
	if r.vm.bytesRead > 8<<20 {
		t.Fatalf("second run read %d bytes for one changed segment", r.vm.bytesRead)
	}
	sums[second.Summary.SnapshotID] = sha256.Sum256(r.vm.disk)

	r.vm.bytesRead = 0
	third := r.mustBackup()
	if third.Mode != backup.BlocksModeChanged || r.vm.bytesRead != 0 {
		t.Fatalf("an unchanged disk was read: mode %s, %d bytes", third.Mode, r.vm.bytesRead)
	}
	sums[third.Summary.SnapshotID] = sha256.Sum256(r.vm.disk)

	for id, want := range sums {
		if got := r.restoredSum(id); got != want {
			t.Fatalf("snapshot %s restores to other bytes than the disk had", id[:8])
		}
	}
	if !slices.Equal(r.vm.bitmaps(), []string{third.Checkpoint}) {
		t.Fatalf("bitmaps = %v, want only %s", r.vm.bitmaps(), third.Checkpoint)
	}
}

func TestBlocksLeaveNoCheckpointOnTheDomainBetweenRuns(t *testing.T) {
	r := newBlockRig(t, 16<<20)
	r.vm.write(0, fill(1<<20, 1))
	first := r.mustBackup()
	if len(r.vm.order) != 0 {
		t.Fatalf("checkpoints on the domain after a run = %v, want none so it can be undefined", r.vm.order)
	}
	if backup.CheckpointName(r.kept.def) != first.Checkpoint {
		t.Fatalf("kept definition = %q, want %s", r.kept.def, first.Checkpoint)
	}
	r.vm.write(9<<20, fill(4096, 2))
	r.vm.bytesRead = 0
	second := r.mustBackup()
	if second.Mode != backup.BlocksModeChanged || r.vm.bytesRead > 8<<20 {
		t.Fatalf("run on a restored checkpoint = %s/%s, %d bytes read", second.Mode, second.Reason, r.vm.bytesRead)
	}
	if len(r.vm.order) != 0 || !slices.Equal(r.vm.bitmaps(), []string{second.Checkpoint}) {
		t.Fatalf("checkpoints %v, bitmaps %v after the second run", r.vm.order, r.vm.bitmaps())
	}
	if r.restoredSum(second.Summary.SnapshotID) != sha256.Sum256(r.vm.disk) {
		t.Fatal("restore differs after a run on a restored checkpoint")
	}
}

func TestBlocksClearLeftoverCheckpointsOfAnInterruptedRun(t *testing.T) {
	r := newBlockRig(t, 16<<20)
	first := r.mustBackup()
	r.vm.checkpoints["bombvault-19990101000000"] = map[int64]bool{}
	r.vm.order = append(r.vm.order, "bombvault-19990101000000")
	r.vm.write(0, fill(4096, 5))
	res := r.mustBackup()
	if res.Mode != backup.BlocksModeChanged {
		t.Fatalf("run = %s/%s, want changed blocks from %s", res.Mode, res.Reason, first.Checkpoint)
	}
	if len(r.vm.order) != 0 || !slices.Equal(r.vm.bitmaps(), []string{res.Checkpoint}) {
		t.Fatalf("checkpoints %v, bitmaps %v, want only the new bitmap", r.vm.order, r.vm.bitmaps())
	}
}

func TestBlocksDropTheNewCheckpointWhenItsDefinitionCannotBeKept(t *testing.T) {
	r := newBlockRig(t, 8<<20)
	r.kept.failing = true
	r.mustBackup()
	if len(r.vm.order) != 0 || len(r.vm.bitmaps()) != 0 {
		t.Fatalf("checkpoints %v, bitmaps %v, want nothing left behind", r.vm.order, r.vm.bitmaps())
	}
	r.kept.failing = false
	if res := r.mustBackup(); res.Mode != backup.BlocksModeFull || res.Reason != backup.BlocksReasonNoCheckpoint {
		t.Fatalf("run = %s/%s, want a full read", res.Mode, res.Reason)
	}
}

func TestBlocksLeavesForeignCheckpointsAlone(t *testing.T) {
	r := newBlockRig(t, 8<<20)
	r.vm.checkpoints["nightly"] = map[int64]bool{}
	r.vm.order = append(r.vm.order, "nightly")
	r.mustBackup()
	r.mustBackup()
	if !slices.Equal(r.vm.order, []string{"nightly"}) || r.vm.checkpoints["nightly"] == nil {
		t.Fatalf("checkpoints = %v", r.vm.order)
	}
}

func TestBlocksBatchesStayWithinBudgetAndLeaveNoIntermediateSnapshot(t *testing.T) {
	r := newBlockRig(t, 40<<20)
	r.vm.write(0, fill(40<<20, 0x5a))
	res := r.mustBackup()
	if len(r.repo.order) != 1 || r.repo.order[0] != res.Summary.SnapshotID {
		t.Fatalf("snapshots left = %d, want only the final one", len(r.repo.order))
	}
	if got := r.restoredSum(res.Summary.SnapshotID); got != sha256.Sum256(r.vm.disk) {
		t.Fatal("a disk written in several batches restores to other bytes")
	}
	if r.repo.next < 5 {
		t.Fatalf("%d restic runs for 40 MiB with an 8 MiB budget", r.repo.next)
	}
	if res.Summary.Bytes < 40<<20 {
		t.Fatalf("bytes added = %d, want the sum over all batches", res.Summary.Bytes)
	}
}

func TestBlocksReadsEverythingWhenCheckpointIsGone(t *testing.T) {
	r := newBlockRig(t, 16<<20)
	r.vm.write(0, fill(1<<20, 1))
	first := r.mustBackup()
	delete(r.vm.checkpoints, first.Checkpoint)
	r.vm.write(9<<20, fill(1<<20, 2))
	res := r.mustBackup()
	if res.Mode != backup.BlocksModeFull || res.Reason != backup.BlocksReasonNoCheckpoint {
		t.Fatalf("run = %s/%s, want a full read for a missing checkpoint", res.Mode, res.Reason)
	}
	if r.restoredSum(res.Summary.SnapshotID) != sha256.Sum256(r.vm.disk) {
		t.Fatal("restore differs after a full read")
	}
}

func TestBlocksReadsEverythingWhenIncrementalStartIsRefused(t *testing.T) {
	r := newBlockRig(t, 16<<20)
	r.mustBackup()
	r.vm.refuseIncr = true
	r.vm.write(3<<20, fill(10, 9))
	res := r.mustBackup()
	if res.Mode != backup.BlocksModeFull || res.Reason != backup.BlocksReasonCheckpointBroken {
		t.Fatalf("run = %s/%s, want a full read for a broken checkpoint", res.Mode, res.Reason)
	}
	if r.restoredSum(res.Summary.SnapshotID) != sha256.Sum256(r.vm.disk) {
		t.Fatal("restore differs after a refused incremental start")
	}
}

func TestBlocksStartsNewChainAfterClassicBackup(t *testing.T) {
	r := newBlockRig(t, 8<<20)
	r.mustBackup()
	r.repo.next++
	id := fmt.Sprintf("%064x", r.repo.next)
	r.repo.snaps[id] = &blockSnap{tags: []string{"vm:win", "p2"}, files: map[string]blockFile{}}
	r.repo.order = append(r.repo.order, id)
	res := r.mustBackup()
	if res.Mode != backup.BlocksModeFull || res.Reason != backup.BlocksReasonNewerBackup {
		t.Fatalf("run = %s/%s, want a full read after a classic backup", res.Mode, res.Reason)
	}
}

func TestBlocksReadsEverythingWhenResticReadsAPlaceholder(t *testing.T) {
	r := newBlockRig(t, 24<<20)
	r.vm.write(0, fill(24<<20, 7))
	r.mustBackup()
	r.vm.write(20<<20, fill(10, 8))
	r.repo.ignoreParentOnce = true
	res := r.mustBackup()
	if res.Mode != backup.BlocksModeFull || res.Reason != backup.BlocksReasonChainBroken {
		t.Fatalf("run = %s/%s, want a full read after a placeholder was read", res.Mode, res.Reason)
	}
	if r.restoredSum(res.Summary.SnapshotID) != sha256.Sum256(r.vm.disk) {
		t.Fatal("restore differs after the placeholder check")
	}
	for _, id := range r.repo.order[:len(r.repo.order)-1] {
		if r.repo.order[len(r.repo.order)-1] == id {
			continue
		}
		if slices.Contains(r.repo.snaps[id].tags, backup.BlocksPartialTag("win")) {
			t.Fatal("an intermediate snapshot was left behind")
		}
	}
}

func TestBlocksFailureKeepsTheOldChain(t *testing.T) {
	r := newBlockRig(t, 16<<20)
	first := r.mustBackup()
	r.vm.write(0, fill(1<<20, 3))
	r.repo.failBackup = true
	if _, err := r.backup(); err == nil {
		t.Fatal("a failed restic run reported success")
	}
	if len(r.vm.order) != 0 || !slices.Equal(r.vm.bitmaps(), []string{first.Checkpoint}) {
		t.Fatalf("checkpoints %v, bitmaps %v after a failure, want only the bitmap of %s", r.vm.order, r.vm.bitmaps(), first.Checkpoint)
	}
	if got := r.runs.finishes; len(got) != 2 || got[1] != "failed" {
		t.Fatalf("run outcomes = %v", got)
	}
	r.repo.failBackup = false
	r.vm.write(12<<20, fill(1<<20, 4))
	res := r.mustBackup()
	if res.Mode != backup.BlocksModeChanged {
		t.Fatalf("run after a failure = %s/%s, want changed blocks", res.Mode, res.Reason)
	}
	if r.restoredSum(res.Summary.SnapshotID) != sha256.Sum256(r.vm.disk) {
		t.Fatal("changes from the failed run are missing after the next one")
	}
}

func TestBlocksUnavailableWhenNoJobStarts(t *testing.T) {
	r := newBlockRig(t, 8<<20)
	r.vm.refuseAll = true
	_, err := r.backup()
	var unavailable *backup.BlocksUnavailableError
	if !errors.As(err, &unavailable) || unavailable.Reason != backup.BlocksReasonJobFailed {
		t.Fatalf("err = %v, want the classic fallback", err)
	}
	if r.runs.started != 0 || len(r.vm.order) != 0 || len(r.vm.bitmaps()) != 0 {
		t.Fatalf("a run was recorded (%d) or a checkpoint kept (%v, %v)", r.runs.started, r.vm.order, r.vm.bitmaps())
	}
}

func TestBlockSegmentSizeKeepsSegmentCountBounded(t *testing.T) {
	for _, tc := range []struct{ size, want int64 }{
		{1 << 20, 8 << 20},
		{128 << 30, 8 << 20},
		{128<<30 + 1, 16 << 20},
		{1 << 40, 64 << 20},
	} {
		if got := backup.BlockSegmentSize(tc.size); got != tc.want {
			t.Errorf("BlockSegmentSize(%d) = %d, want %d", tc.size, got, tc.want)
		}
	}
}

func TestRestoreBlockImageConvertsNonRawDisks(t *testing.T) {
	r := newBlockRig(t, 8<<20)
	r.vm.write(100, []byte("guest data"))
	res := r.mustBackup()
	m, _ := r.repo.Manifest(context.Background(), "/repo", res.Summary.SnapshotID)
	disk := m.Disks[0]
	disk.Format = "qcow2"
	target := filepath.Join(t.TempDir(), "vdisk1.qcow2")
	var converted string
	convert := func(_ context.Context, raw, out, format string) error {
		converted = format
		b, err := os.ReadFile(raw) //nolint:gosec // a temp file of the test
		if err != nil {
			return err
		}
		return os.WriteFile(out, append([]byte("QFI\xfb"), b...), 0o600) //nolint:gosec // a temp file of the test
	}
	if err := backup.RestoreBlockImage(context.Background(), r.repo, convert, "/repo", res.Summary.SnapshotID, backup.VMRestoreImage{Disk: disk, Target: target}); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(target) //nolint:gosec // a temp file of the test
	if converted != "qcow2" || !bytes.HasPrefix(b, []byte("QFI\xfb")) || !bytes.Equal(b[4:], r.vm.disk) {
		t.Fatalf("converted %q, image of %d bytes", converted, len(b))
	}
	if entries, _ := os.ReadDir(filepath.Dir(target)); len(entries) != 1 {
		t.Fatalf("temporary files left next to the disk: %v", entries)
	}
}

func TestRestoreBlockImageRefusesMissingSegments(t *testing.T) {
	r := newBlockRig(t, 16<<20)
	res := r.mustBackup()
	snap := r.repo.snaps[res.Summary.SnapshotID]
	delete(snap.files, "vda/00000001")
	m, _ := r.repo.Manifest(context.Background(), "/repo", res.Summary.SnapshotID)
	target := filepath.Join(t.TempDir(), "vdisk1.img")
	if err := os.WriteFile(target, []byte("live disk"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := backup.RestoreBlockImage(context.Background(), r.repo, nil, "/repo", res.Summary.SnapshotID, backup.VMRestoreImage{Disk: m.Disks[0], Target: target}); err == nil {
		t.Fatal("an incomplete snapshot restored")
	}
	if b, _ := os.ReadFile(target); string(b) != "live disk" { //nolint:gosec // a temp file of the test
		t.Fatal("a failed restore replaced the disk")
	}
}

func TestRestoreVMRebuildsBlockImagesBeforeDefine(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the restore checks for absolute Linux paths")
	}
	r := newBlockRig(t, 16<<20)
	r.vm.write(5<<20, []byte("boot sector"))
	res := r.mustBackup()
	m, _ := r.repo.Manifest(context.Background(), "/repo", res.Summary.SnapshotID)
	target := filepath.Join(t.TempDir(), "vdisk1.img")

	vm := &fakeVM{stateVal: "shut off"}
	restic := &fakeRestic{}
	err := backup.RestoreVM(t.Context(), backup.VMRestoreDeps{
		Confirmed:   true,
		Name:        "win",
		SnapshotID:  res.Summary.SnapshotID,
		DiskPaths:   []string{filepath.ToSlash(target)},
		DomainXML:   "<domain/>",
		RepoPath:    "/repo",
		TargetID:    "t1",
		DataDir:     t.TempDir(),
		Images:      []backup.VMRestoreImage{{Disk: m.Disks[0], Target: target}},
		ImageDumper: r.repo,
		VM:          vm,
		Restic:      restic,
		Runs:        &fakeRuns{},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range restic.log {
		if strings.HasPrefix(e, "restore") {
			t.Fatalf("the disk folder was restored with restic as well: %v", restic.log)
		}
	}
	if !vmContains(vm.log, "define:") {
		t.Fatalf("the VM was not defined: %v", vm.log)
	}
	b, _ := os.ReadFile(target) //nolint:gosec // a temp file of the test
	if !bytes.Equal(b, r.vm.disk) {
		t.Fatal("the rebuilt disk differs from the backed-up one")
	}
}

// checkpointedVM is a VM whose domain has checkpoints on record, which make
// libvirt refuse to undefine it once it is off.
type checkpointedVM struct {
	*fakeVM
	names []string
}

func (v *checkpointedVM) CheckpointNames(context.Context, string) ([]string, error) {
	return slices.Clone(v.names), nil
}

func (v *checkpointedVM) CheckpointForget(_ context.Context, _, name string) error {
	v.log = append(v.log, "forget:"+name)
	v.names = slices.DeleteFunc(v.names, func(n string) bool { return n == name })
	return nil
}

func (v *checkpointedVM) Undefine(ctx context.Context, name string) error {
	if len(v.names) > 0 {
		return errors.New("cannot undefine domain with checkpoints")
	}
	return v.fakeVM.Undefine(ctx, name)
}

func TestRestoreVMClearsOwnCheckpointsBeforeTheVMGoesOff(t *testing.T) {
	vm := &checkpointedVM{fakeVM: &fakeVM{stateVal: "running"}, names: []string{"bombvault-20260101000000"}}
	deps := sampleVMRestoreDeps(t, vm.fakeVM, &fakeRestic{}, &fakeRuns{})
	deps.VM = vm
	if err := backup.RestoreVM(t.Context(), deps); err != nil {
		t.Fatal(err)
	}
	forget, destroy := slices.Index(vm.log, "forget:bombvault-20260101000000"), -1
	for i, e := range vm.log {
		if strings.HasPrefix(e, "destroy:") {
			destroy = i
		}
	}
	if forget < 0 || destroy < 0 || forget > destroy {
		t.Fatalf("calls = %v, want the checkpoint cleared before the destroy", vm.log)
	}
}

func TestRestoreVMLeavesAVMWithForeignCheckpointsRunning(t *testing.T) {
	vm := &checkpointedVM{fakeVM: &fakeVM{stateVal: "running"}, names: []string{"nightly"}}
	deps := sampleVMRestoreDeps(t, vm.fakeVM, &fakeRestic{}, &fakeRuns{})
	deps.VM = vm
	err := backup.RestoreVM(t.Context(), deps)
	if err == nil || !strings.Contains(err.Error(), "nightly") {
		t.Fatalf("err = %v, want a refusal naming the checkpoint", err)
	}
	for _, e := range vm.log {
		if strings.HasPrefix(e, "destroy:") || strings.HasPrefix(e, "forget:") {
			t.Fatalf("calls = %v, want the VM left alone", vm.log)
		}
	}
}
