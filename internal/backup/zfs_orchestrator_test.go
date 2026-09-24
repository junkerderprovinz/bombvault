package backup_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/backup"
	"github.com/junkerderprovinz/bombvault/internal/zfs"
)

const (
	zfsTestSnap   = "bombvault-20260917031500"
	zfsTestRoot   = "cache/appdata"
	zfsTestPlex   = "cache/appdata/plex"
	zfsTestDB     = "cache/appdata/db"
	zfsRootDir    = "/host/user/cache/appdata"
	zfsRootSnapD  = zfsRootDir + "/.zfs/snapshot/" + zfsTestSnap
	zfsPlexSnapD  = zfsRootDir + "/plex/.zfs/snapshot/" + zfsTestSnap
	zfsDBSnapDir  = zfsRootDir + "/db/.zfs/snapshot/" + zfsTestSnap
	zfsTestBudget = 8 * time.Second
)

var zfsTestNow = time.Date(2026, 9, 17, 3, 15, 0, 0, time.UTC)

// zfsLog is the single ordered call log the ZFS seams write into, so a test
// can assert what happened in which order across all of them.
type zfsLog struct{ entries []string }

func (l *zfsLog) add(format string, a ...any) {
	l.entries = append(l.entries, fmt.Sprintf(format, a...))
}

func (l *zfsLog) index(entry string) int {
	for i, e := range l.entries {
		if e == entry {
			return i
		}
	}
	return -1
}

func (l *zfsLog) has(entry string) bool { return l.index(entry) >= 0 }

func (l *zfsLog) count(entry string) int {
	n := 0
	for _, e := range l.entries {
		if e == entry {
			n++
		}
	}
	return n
}

type fakeZFSSnapshotter struct {
	log            *zfsLog
	snapshotErr    error
	primeErr       error
	destroyErrs    []error
	destroyCalls   int
	destroyCtxErrs []error
}

func (f *fakeZFSSnapshotter) SnapshotRecursive(_ context.Context, root, snap string) error {
	f.log.add("snapshot:%s@%s", root, snap)
	return f.snapshotErr
}

func (f *fakeZFSSnapshotter) DestroyRecursive(ctx context.Context, root, snap string) error {
	f.log.add("destroy:%s@%s", root, snap)
	f.destroyCtxErrs = append(f.destroyCtxErrs, ctx.Err())
	i := f.destroyCalls
	f.destroyCalls++
	if i < len(f.destroyErrs) {
		return f.destroyErrs[i]
	}
	if len(f.destroyErrs) > 0 {
		return f.destroyErrs[len(f.destroyErrs)-1]
	}
	return nil
}

func (f *fakeZFSSnapshotter) Prime(_ context.Context, hostMountpoint, snap string) error {
	f.log.add("prime:%s@%s", hostMountpoint, snap)
	return f.primeErr
}

type zfsBackupCall struct {
	repo     string
	dir      string
	tags     []string
	excludes []string
}

type fakeZFSRestic struct {
	log     *zfsLog
	calls   []zfsBackupCall
	errs    map[string]error
	onCall  func(dataset string)
	summary backup.ZFSBackupSummary
	// byDataset answers for one dataset instead of summary.
	byDataset map[string]backup.ZFSBackupSummary
}

func (f *fakeZFSRestic) BackupDir(_ context.Context, repo, dir string, tags []string, excludes ...string) (backup.ZFSBackupSummary, error) {
	f.log.add("backupDir:%s", dir)
	f.calls = append(f.calls, zfsBackupCall{repo: repo, dir: dir, tags: tags, excludes: excludes})
	dataset := strings.TrimPrefix(tags[0], "zfs:")
	if f.onCall != nil {
		f.onCall(dataset)
	}
	if err := f.errs[dataset]; err != nil {
		return backup.ZFSBackupSummary{}, err
	}
	sum := f.summary
	if own, ok := f.byDataset[dataset]; ok {
		sum = own
	}
	sum.SnapshotID = fmt.Sprintf("snap%d", len(f.calls))
	return sum, nil
}

func (f *fakeZFSRestic) callFor(dataset string) (zfsBackupCall, bool) {
	for _, c := range f.calls {
		if c.tags[0] == "zfs:"+dataset {
			return c, true
		}
	}
	return zfsBackupCall{}, false
}

type zfsRunRecord struct {
	runID      string
	snap       string
	window     time.Duration
	hookDetail string
}

type fakeZFSRecorder struct {
	log     *zfsLog
	runs    []zfsRunRecord
	members []backup.ZFSMemberResult
}

func (f *fakeZFSRecorder) RecordRun(runID, snap string, window time.Duration, hookDetail string) error {
	f.log.add("recordRun:%s:%s", runID, snap)
	f.runs = append(f.runs, zfsRunRecord{runID: runID, snap: snap, window: window, hookDetail: hookDetail})
	return nil
}

func (f *fakeZFSRecorder) AddMember(_ string, m backup.ZFSMemberResult) error {
	f.log.add("addMember:%s:%s", m.Dataset, m.Outcome)
	f.members = append(f.members, m)
	return nil
}

func (f *fakeZFSRecorder) member(t *testing.T, dataset string) backup.ZFSMemberResult {
	t.Helper()
	for _, m := range f.members {
		if m.Dataset == dataset {
			return m
		}
	}
	t.Fatalf("no member recorded for %q: %v", dataset, f.members)
	return backup.ZFSMemberResult{}
}

type fakeZFSConsistency struct {
	log         *zfsLog
	freezeErr   error
	window      time.Duration
	thaws       int
	thawCtxErrs []error
}

func (f *fakeZFSConsistency) Freeze(_ context.Context) (func(context.Context), func() time.Duration, error) {
	f.log.add("freeze")
	if f.freezeErr != nil {
		return nil, nil, f.freezeErr
	}
	thaw := func(ctx context.Context) {
		f.log.add("thaw")
		f.thaws++
		f.thawCtxErrs = append(f.thawCtxErrs, ctx.Err())
	}
	return thaw, func() time.Duration { return f.window }, nil
}

type fakeZFSHooks struct {
	log     *zfsLog
	preErr  error
	postErr error
}

func (f *fakeZFSHooks) Pre(context.Context) error {
	f.log.add("pre")
	return f.preErr
}

func (f *fakeZFSHooks) Post(context.Context) error {
	f.log.add("post")
	return f.postErr
}

// zfsRuns writes the run lifecycle into the shared log so ordering against the
// other seams can be asserted, and keeps recording into the package's fakeRuns.
type zfsRuns struct {
	*fakeRuns
	log *zfsLog
}

func (r *zfsRuns) Start(targetID, kind string) (string, error) {
	id, err := r.fakeRuns.Start(targetID, kind)
	r.log.add("runStart:%s:%s", targetID, kind)
	return id, err
}

func (r *zfsRuns) Finish(runID, status string, sum backup.Summary, errMsg string) error {
	r.log.add("runFinish:%s", status)
	return r.fakeRuns.Finish(runID, status, sum, errMsg)
}

// zfsClock is the injected clock: it stands still until a sleep moves it, so a
// retry budget can be asserted without waiting for it.
type zfsClock struct {
	now   time.Time
	slept []time.Duration
}

func (c *zfsClock) time() time.Time { return c.now }

func (c *zfsClock) sleep(_ context.Context, d time.Duration) {
	c.slept = append(c.slept, d)
	c.now = c.now.Add(d)
}

func (c *zfsClock) elapsed(start time.Time) time.Duration { return c.now.Sub(start) }

type zfsFixture struct {
	log         *zfsLog
	snapshotter *fakeZFSSnapshotter
	restic      *fakeZFSRestic
	recorder    *fakeZFSRecorder
	runs        *zfsRuns
	clock       *zfsClock
	visibleErrs map[string]error
	emptyDirs   map[string]bool
	deps        backup.ZFSBackupDeps
}

func zfsTestMembers() []backup.ZFSMemberPlan {
	return []backup.ZFSMemberPlan{
		{Dataset: zfsTestRoot, RelPath: "/", HostMountpoint: "/mnt/cache/appdata", ContainerMountpoint: zfsRootDir},
		{Dataset: zfsTestPlex, RelPath: "/plex", HostMountpoint: "/mnt/cache/appdata/plex", ContainerMountpoint: zfsRootDir + "/plex"},
		{Dataset: zfsTestDB, RelPath: "/db", HostMountpoint: "/mnt/cache/appdata/db", ContainerMountpoint: zfsRootDir + "/db"},
	}
}

func newZFSFixture() *zfsFixture {
	log := &zfsLog{}
	f := &zfsFixture{
		log:         log,
		snapshotter: &fakeZFSSnapshotter{log: log},
		restic:      &fakeZFSRestic{log: log, errs: map[string]error{}},
		recorder:    &fakeZFSRecorder{log: log},
		runs:        &zfsRuns{fakeRuns: &fakeRuns{}, log: log},
		clock:       &zfsClock{now: zfsTestNow},
		visibleErrs: map[string]error{},
		emptyDirs:   map[string]bool{},
	}
	f.deps = backup.ZFSBackupDeps{
		Root:     zfsTestRoot,
		Members:  zfsTestMembers(),
		Repo:     "/repo",
		TargetID: "item-1",
		Now:      func() time.Time { return zfsTestNow },
		Clock:    f.clock.time,
		ZFS:      f.snapshotter,
		Restic:   f.restic,
		Visible: func(_ context.Context, dataset, _, _ string) error {
			log.add("visible:%s", dataset)
			return f.visibleErrs[dataset]
		},
		DirEmpty: func(dir string) (bool, error) {
			log.add("dirEmpty:%s", dir)
			return f.emptyDirs[dir], nil
		},
		Runs:          f.runs,
		Recorder:      f.recorder,
		DestroyBudget: zfsTestBudget,
		ShuttingDown:  func() bool { return false },
		Sleep:         f.clock.sleep,
	}
	return f
}

func (f *zfsFixture) run(t *testing.T, ctx context.Context) (backup.Summary, error) {
	t.Helper()
	return backup.BackupZFSItem(ctx, f.deps)
}

func (f *zfsFixture) requireOrder(t *testing.T, earlier, later string) {
	t.Helper()
	a, b := f.log.index(earlier), f.log.index(later)
	if a < 0 || b < 0 {
		t.Fatalf("expected both %q and %q in the log: %v", earlier, later, f.log.entries)
	}
	if a >= b {
		t.Fatalf("expected %q before %q: %v", earlier, later, f.log.entries)
	}
}

func zfsBusyErr() error {
	return &zfs.CmdError{Code: "busy", Stderr: "cannot destroy snapshot: dataset is busy"}
}

func zfsNotFoundErr() error {
	return &zfs.CmdError{Code: "not-found", Stderr: "could not find any snapshots to destroy; check snapshot names."}
}

func TestBackupZFSItemCallOrder(t *testing.T) {
	f := newZFSFixture()
	if _, err := f.run(t, t.Context()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{
		"runStart:item-1:backup",
		"snapshot:cache/appdata@" + zfsTestSnap,
		"recordRun:run-1:" + zfsTestSnap,
		"prime:/mnt/cache/appdata@" + zfsTestSnap,
		"visible:" + zfsTestRoot,
		"dirEmpty:" + zfsRootSnapD,
		"backupDir:" + zfsRootSnapD,
		"addMember:" + zfsTestRoot + ":backed-up",
		"prime:/mnt/cache/appdata/plex@" + zfsTestSnap,
		"visible:" + zfsTestPlex,
		"dirEmpty:" + zfsPlexSnapD,
		"backupDir:" + zfsPlexSnapD,
		"addMember:" + zfsTestPlex + ":backed-up",
		"prime:/mnt/cache/appdata/db@" + zfsTestSnap,
		"visible:" + zfsTestDB,
		"dirEmpty:" + zfsDBSnapDir,
		"backupDir:" + zfsDBSnapDir,
		"addMember:" + zfsTestDB + ":backed-up",
		"runFinish:success",
		"destroy:cache/appdata@" + zfsTestSnap,
	}
	if strings.Join(f.log.entries, "\n") != strings.Join(want, "\n") {
		t.Fatalf("call order\ngot:\n%s\nwant:\n%s", strings.Join(f.log.entries, "\n"), strings.Join(want, "\n"))
	}
}

func TestBackupZFSItemPassesPerMemberDirTagAndExcludes(t *testing.T) {
	f := newZFSFixture()
	f.deps.Excludes = []string{"/plex/Library/Cache", "*.tmp"}
	if _, err := f.run(t, t.Context()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	root, ok := f.restic.callFor(zfsTestRoot)
	if !ok {
		t.Fatalf("the root was not backed up: %v", f.restic.calls)
	}
	if root.repo != "/repo" {
		t.Fatalf("root repository = %q, want /repo", root.repo)
	}
	if root.dir != zfsRootSnapD {
		t.Fatalf("root snapshot directory = %q, want %q", root.dir, zfsRootSnapD)
	}
	if len(root.tags) != 1 || root.tags[0] != "zfs:"+zfsTestRoot {
		t.Fatalf("root tags = %v, want exactly [zfs:%s]", root.tags, zfsTestRoot)
	}
	if want := []string{"*.tmp"}; strings.Join(root.excludes, ",") != strings.Join(want, ",") {
		t.Fatalf("root excludes = %v, want %v", root.excludes, want)
	}

	plex, ok := f.restic.callFor(zfsTestPlex)
	if !ok {
		t.Fatalf("plex was not backed up: %v", f.restic.calls)
	}
	if plex.dir != zfsPlexSnapD {
		t.Fatalf("plex snapshot directory = %q, want %q", plex.dir, zfsPlexSnapD)
	}
	if len(plex.tags) != 1 || plex.tags[0] != "zfs:"+zfsTestPlex {
		t.Fatalf("plex tags = %v, want exactly [zfs:%s]", plex.tags, zfsTestPlex)
	}
	want := []string{zfsPlexSnapD + "/Library/Cache", "*.tmp"}
	if strings.Join(plex.excludes, ",") != strings.Join(want, ",") {
		t.Fatalf("plex excludes = %v, want %v", plex.excludes, want)
	}
}

func TestBackupZFSItemSumsMemberBytesAndReportsTheRootSnapshot(t *testing.T) {
	f := newZFSFixture()
	f.restic.summary = backup.ZFSBackupSummary{BytesAdded: 1024, FilesNew: 3, FilesChanged: 1, FilesUnmodified: 7}
	summary, err := f.run(t, t.Context())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if summary.Bytes != 3072 {
		t.Fatalf("summary bytes = %d, want the three members summed to 3072", summary.Bytes)
	}
	if summary.SnapshotID != "snap1" {
		t.Fatalf("summary snapshot = %q, want the root member's snap1", summary.SnapshotID)
	}
	plex := f.recorder.member(t, zfsTestPlex)
	if plex.ResticSnapshot != "snap2" {
		t.Fatalf("plex restic snapshot = %q, want snap2", plex.ResticSnapshot)
	}
	if plex.BytesAdded != 1024 || plex.FilesNew != 3 || plex.FilesChanged != 1 || plex.FilesUnmodified != 7 {
		t.Fatalf("plex counters = %+v, want restic's own numbers", plex)
	}
}

// Anomaly detection watches every dataset of a tree on its own, so each member
// keeps what restic read of it. An empty snapshot directory is a dataset that
// holds nothing, not one nobody looked at.
func TestZFSRunMembersCarryTheSourceMetrics(t *testing.T) {
	f := newZFSFixture()
	parent := true
	f.restic.byDataset = map[string]backup.ZFSBackupSummary{
		zfsTestRoot: {BytesAdded: 100, FilesNew: 1, Measured: true, SourceBytes: 4000, SourceFiles: 40, ResticMS: 300, HasParent: &parent},
		zfsTestDB:   {BytesAdded: 20, FilesNew: 2, Measured: true, SourceBytes: 900, SourceFiles: 9, ResticMS: 200},
	}
	f.emptyDirs[zfsPlexSnapD] = true
	f.deps.Skipped = []backup.ZFSMemberResult{{Dataset: "cache/appdata/old", Outcome: "not-mounted"}}
	f.deps.SelectionFP = "fp-tree"

	summary, err := f.run(t, t.Context())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	root := f.recorder.member(t, zfsTestRoot)
	if !root.Measured || root.SourceBytes != 4000 || root.SourceFiles != 40 {
		t.Fatalf("root = %+v, want its own source figures", root)
	}
	if root.HasParent == nil || !*root.HasParent {
		t.Fatalf("root parent flag = %v, want what restic reported", root.HasParent)
	}
	if db := f.recorder.member(t, zfsTestDB); !db.Measured || db.SourceBytes != 900 || db.SourceFiles != 9 {
		t.Fatalf("db = %+v, want its own source figures", db)
	}
	plex := f.recorder.member(t, zfsTestPlex)
	if plex.Outcome != "empty" || !plex.Measured || plex.SourceBytes != 0 || plex.SourceFiles != 0 {
		t.Fatalf("empty member = %+v, want a measured zero", plex)
	}
	if old := f.recorder.member(t, "cache/appdata/old"); old.Measured || old.Outcome != "not-mounted" {
		t.Fatalf("skipped member = %+v, want its skip code and no measurement", old)
	}

	if !summary.Measured {
		t.Fatal("the run's summary is unmeasured although every member it read was measured")
	}
	if summary.SourceBytes != 4900 || summary.SourceFiles != 49 || summary.FilesNew != 3 || summary.ResticMS != 500 {
		t.Fatalf("summary = %+v, want the members summed", summary)
	}
	if summary.Bytes != 120 {
		t.Fatalf("summary bytes = %d, want 120", summary.Bytes)
	}
	if summary.SelectionFP != "fp-tree" {
		t.Fatalf("summary fingerprint = %q, want the item's", summary.SelectionFP)
	}
	finished := f.runs.finishCalls[len(f.runs.finishCalls)-1]
	if finished.sum.SourceBytes != 4900 || finished.sum.SelectionFP != "fp-tree" {
		t.Fatalf("the run finished with %+v, want the summed summary", finished.sum)
	}
}

func TestZFSRunWithOnlyEmptyMembersIsAMeasuredZero(t *testing.T) {
	f := newZFSFixture()
	for _, dir := range []string{zfsRootSnapD, zfsPlexSnapD, zfsDBSnapDir} {
		f.emptyDirs[dir] = true
	}
	summary, err := f.run(t, t.Context())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !summary.Measured || summary.SourceBytes != 0 || summary.SourceFiles != 0 {
		t.Fatalf("summary = %+v, want a measured zero", summary)
	}
}

func TestZFSRunWithAnUnmeasuredMemberRecordsNoTotals(t *testing.T) {
	f := newZFSFixture()
	f.restic.summary = backup.ZFSBackupSummary{BytesAdded: 10, Measured: true, SourceBytes: 100, SourceFiles: 1}
	f.restic.byDataset = map[string]backup.ZFSBackupSummary{zfsTestDB: {BytesAdded: 10}}
	summary, err := f.run(t, t.Context())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if summary.Measured {
		t.Fatalf("summary = %+v, want no totals when one member went unmeasured", summary)
	}
	if db := f.recorder.member(t, zfsTestDB); db.Measured {
		t.Fatalf("db = %+v, want it unmeasured", db)
	}
}

func TestBackupZFSItemReadsAMemberTheHostCouldNotPrime(t *testing.T) {
	f := newZFSFixture()
	f.snapshotter.primeErr = errors.New("ssh: connection closed")
	if _, err := f.run(t, t.Context()); err != nil {
		t.Fatalf("a failed prime must not fail the run: %v", err)
	}
	if !f.log.has("backupDir:" + zfsRootSnapD) {
		t.Fatalf("the member must still be read from the container: %v", f.log.entries)
	}
}

func TestBackupZFSItemFinishesRunBeforeDestroy(t *testing.T) {
	f := newZFSFixture()
	if _, err := f.run(t, t.Context()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	f.requireOrder(t, "runFinish:success", "destroy:cache/appdata@"+zfsTestSnap)
}

func TestBackupZFSItemMemberFailureContinuesAndFailsRun(t *testing.T) {
	f := newZFSFixture()
	f.restic.errs[zfsTestPlex] = errors.New("repository is locked")
	_, err := f.run(t, t.Context())
	if err == nil {
		t.Fatal("a failed member must fail the run")
	}
	if !f.log.has("backupDir:" + zfsDBSnapDir) {
		t.Fatalf("the run must continue with the next member: %v", f.log.entries)
	}
	if got := f.recorder.member(t, zfsTestPlex).Outcome; got != "backup-failed" {
		t.Fatalf("plex outcome = %q, want backup-failed", got)
	}
	finish := f.runs.finishOf(t, "run-1")
	if finish.status != "failed" {
		t.Fatalf("run status = %q, want failed", finish.status)
	}
	if !strings.Contains(finish.note, zfsTestPlex) || !strings.Contains(finish.note, "[backup-failed]") {
		t.Fatalf("the run error must name the failed member and its code, got %q", finish.note)
	}
	if n := f.log.count("destroy:cache/appdata@" + zfsTestSnap); n != 1 {
		t.Fatalf("destroy called %d times, want 1", n)
	}
}

func TestBackupZFSItemVisibilityFailureIsPerMember(t *testing.T) {
	f := newZFSFixture()
	f.visibleErrs[zfsTestRoot] = &backup.ZFSRefusal{Code: "snapshot-not-visible", Detail: zfsTestRoot}
	_, err := f.run(t, t.Context())
	if err == nil {
		t.Fatal("an invisible member must fail the run")
	}
	if got := f.recorder.member(t, zfsTestRoot).Outcome; got != "snapshot-not-visible" {
		t.Fatalf("root outcome = %q, want snapshot-not-visible", got)
	}
	if f.log.has("backupDir:" + zfsRootSnapD) {
		t.Fatalf("an invisible member must not be read: %v", f.log.entries)
	}
	for _, dataset := range []string{zfsTestPlex, zfsTestDB} {
		if got := f.recorder.member(t, dataset).Outcome; got != "backed-up" {
			t.Fatalf("%s outcome = %q, want backed-up", dataset, got)
		}
	}
	if n := f.log.count("destroy:cache/appdata@" + zfsTestSnap); n != 1 {
		t.Fatalf("destroy called %d times, want 1", n)
	}
}

func TestBackupZFSItemSnapshotFailureNeverDestroysOrBacksUp(t *testing.T) {
	f := newZFSFixture()
	f.snapshotter.snapshotErr = &zfs.CmdError{Code: "zfs-error", Stderr: "cannot create snapshot"}
	_, err := f.run(t, t.Context())
	if err == nil {
		t.Fatal("a failed snapshot must fail the run")
	}
	if f.snapshotter.destroyCalls != 0 {
		t.Fatalf("a snapshot that was never created must never be destroyed: %v", f.log.entries)
	}
	if len(f.restic.calls) != 0 {
		t.Fatalf("no member may be read without a snapshot: %v", f.restic.calls)
	}
	finish := f.runs.finishOf(t, "run-1")
	if finish.status != "failed" || !strings.Contains(finish.note, "snapshot-failed") {
		t.Fatalf("run finish = %+v, want failed with snapshot-failed", finish)
	}
}

func TestBackupZFSItemDestroyUsesUncancelledContext(t *testing.T) {
	f := newZFSFixture()
	ctx, cancel := context.WithCancel(t.Context())
	f.restic.onCall = func(dataset string) {
		if dataset == zfsTestRoot {
			cancel()
		}
	}
	_, _ = f.run(t, ctx)
	if len(f.snapshotter.destroyCtxErrs) != 1 {
		t.Fatalf("destroy attempts = %d, want 1", len(f.snapshotter.destroyCtxErrs))
	}
	if f.snapshotter.destroyCtxErrs[0] != nil {
		t.Fatalf("the destroy ran on a cancelled context: %v", f.snapshotter.destroyCtxErrs[0])
	}
}

func TestBackupZFSItemCancelMarksRemainingNotReached(t *testing.T) {
	f := newZFSFixture()
	ctx, cancel := context.WithCancel(t.Context())
	f.restic.onCall = func(dataset string) {
		if dataset == zfsTestRoot {
			cancel()
		}
	}
	f.restic.errs[zfsTestRoot] = context.Canceled
	if _, err := f.run(t, ctx); err == nil {
		t.Fatal("a cancelled run must fail")
	}
	for _, dataset := range []string{zfsTestPlex, zfsTestDB} {
		if got := f.recorder.member(t, dataset).Outcome; got != "not-reached" {
			t.Fatalf("%s outcome = %q, want not-reached", dataset, got)
		}
	}
	if len(f.restic.calls) != 1 {
		t.Fatalf("members after the cancel must not be read: %v", f.restic.calls)
	}
}

func TestBackupZFSItemEmptyMemberSucceedsWithoutRestic(t *testing.T) {
	f := newZFSFixture()
	f.emptyDirs[zfsPlexSnapD] = true
	if _, err := f.run(t, t.Context()); err != nil {
		t.Fatalf("an empty member must not fail the run: %v", err)
	}
	if f.log.has("backupDir:" + zfsPlexSnapD) {
		t.Fatalf("an empty member must not be handed to restic: %v", f.log.entries)
	}
	if got := f.recorder.member(t, zfsTestPlex).Outcome; got != "empty" {
		t.Fatalf("plex outcome = %q, want empty", got)
	}
	if f.snapshotter.destroyCalls != 1 {
		t.Fatalf("destroy calls = %d, want 1", f.snapshotter.destroyCalls)
	}
}

func TestBackupZFSItemThawsAfterSnapshotBeforeBackup(t *testing.T) {
	f := newZFSFixture()
	cons := &fakeZFSConsistency{log: f.log, window: 12 * time.Second}
	hooks := &fakeZFSHooks{log: f.log}
	f.deps.Consistency, f.deps.Hooks = cons, hooks
	if _, err := f.run(t, t.Context()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	f.requireOrder(t, "pre", "freeze")
	f.requireOrder(t, "freeze", "snapshot:cache/appdata@"+zfsTestSnap)
	f.requireOrder(t, "snapshot:cache/appdata@"+zfsTestSnap, "thaw")
	f.requireOrder(t, "thaw", "post")
	f.requireOrder(t, "post", "backupDir:"+zfsRootSnapD)
	if cons.thaws != 1 {
		t.Fatalf("thaw called %d times, want exactly 1", cons.thaws)
	}
	if cons.thawCtxErrs[0] != nil {
		t.Fatalf("thaw ran on a cancellable context: %v", cons.thawCtxErrs[0])
	}
	if f.recorder.runs[0].window != 12*time.Second {
		t.Fatalf("recorded window = %v, want 12s", f.recorder.runs[0].window)
	}
}

func TestBackupZFSItemThawsWhenSnapshotFails(t *testing.T) {
	f := newZFSFixture()
	cons := &fakeZFSConsistency{log: f.log}
	f.deps.Consistency = cons
	f.snapshotter.snapshotErr = errors.New("cannot create snapshot")
	if _, err := f.run(t, t.Context()); err == nil {
		t.Fatal("a failed snapshot must fail the run")
	}
	if cons.thaws != 1 {
		t.Fatalf("thaw called %d times, want exactly 1 after a failed snapshot", cons.thaws)
	}
}

func TestBackupZFSItemFreezeErrorTakesNoSnapshot(t *testing.T) {
	f := newZFSFixture()
	refusal := &backup.ZFSRefusal{Code: "containers-busy", Detail: "plex"}
	f.deps.Consistency = &fakeZFSConsistency{log: f.log, freezeErr: refusal}
	_, err := f.run(t, t.Context())
	if !errors.Is(err, backup.ErrZFSRefusal) {
		t.Fatalf("error = %v, want a ZFS refusal", err)
	}
	if f.log.has("snapshot:cache/appdata@" + zfsTestSnap) {
		t.Fatalf("a failed freeze must take no snapshot: %v", f.log.entries)
	}
	finish := f.runs.finishOf(t, "run-1")
	if finish.status != "failed" || !strings.Contains(finish.note, "containers-busy") {
		t.Fatalf("run finish = %+v, want failed with containers-busy", finish)
	}
}

func TestBackupZFSItemRunsPostHookWhenFreezeFails(t *testing.T) {
	f := newZFSFixture()
	f.deps.Consistency = &fakeZFSConsistency{log: f.log, freezeErr: &backup.ZFSRefusal{Code: "container-unknown", Detail: "db"}}
	f.deps.Hooks = &fakeZFSHooks{log: f.log}
	if _, err := f.run(t, t.Context()); err == nil {
		t.Fatal("a failed freeze must fail the run")
	}
	f.requireOrder(t, "pre", "freeze")
	f.requireOrder(t, "freeze", "post")
	if f.log.count("post") != 1 {
		t.Fatalf("post ran %d times, want once: %v", f.log.count("post"), f.log.entries)
	}
}

func TestBackupZFSItemPreHookFailureTakesNoSnapshot(t *testing.T) {
	f := newZFSFixture()
	cons := &fakeZFSConsistency{log: f.log}
	f.deps.Consistency = cons
	f.deps.Hooks = &fakeZFSHooks{log: f.log, preErr: errors.New("exit status 1")}
	_, err := f.run(t, t.Context())
	if !errors.Is(err, backup.ErrZFSRefusal) {
		t.Fatalf("error = %v, want a ZFS refusal", err)
	}
	if f.log.has("freeze") || f.log.has("snapshot:cache/appdata@"+zfsTestSnap) {
		t.Fatalf("a failed pre-snapshot command must stop the run: %v", f.log.entries)
	}
	finish := f.runs.finishOf(t, "run-1")
	if finish.status != "failed" || !strings.Contains(finish.note, "pre-snapshot-failed") {
		t.Fatalf("run finish = %+v, want failed with pre-snapshot-failed", finish)
	}
}

func TestBackupZFSItemPostHookFailureIsNotFatal(t *testing.T) {
	f := newZFSFixture()
	f.deps.Hooks = &fakeZFSHooks{log: f.log, postErr: errors.New("exit status 2")}
	if _, err := f.run(t, t.Context()); err != nil {
		t.Fatalf("a failed post-snapshot command must not fail the run: %v", err)
	}
	if f.recorder.runs[0].hookDetail == "" {
		t.Fatal("the failed post-snapshot command must be recorded on the run")
	}
	if got := f.runs.finishOf(t, "run-1").status; got != "success" {
		t.Fatalf("run status = %q, want success", got)
	}
}

func TestBackupZFSItemDestroyFailureKeepsRunStatus(t *testing.T) {
	f := newZFSFixture()
	f.snapshotter.destroyErrs = []error{errors.New("cannot destroy")}
	var failedRoot, failedSnap string
	calls := 0
	f.deps.OnDestroyFailed = func(root, snap string, _ error) {
		calls++
		failedRoot, failedSnap = root, snap
	}
	if _, err := f.run(t, t.Context()); err != nil {
		t.Fatalf("a failed destroy must not fail the backup: %v", err)
	}
	if got := f.runs.finishOf(t, "run-1").status; got != "success" {
		t.Fatalf("run status = %q, want success", got)
	}
	if calls != 1 || failedRoot != zfsTestRoot || failedSnap != zfsTestSnap {
		t.Fatalf("OnDestroyFailed calls = %d for %s@%s, want 1 for %s@%s", calls, failedRoot, failedSnap, zfsTestRoot, zfsTestSnap)
	}
}

func TestBackupZFSItemRecordsSkippedAndNewMembers(t *testing.T) {
	f := newZFSFixture()
	f.deps.Skipped = []backup.ZFSMemberResult{
		{Dataset: "cache/appdata/vmdisks", Outcome: "zvol"},
		{Dataset: "cache/appdata/old", Outcome: "not-mounted"},
	}
	members := zfsTestMembers()
	members[1].IsNew = true
	f.deps.Members = members
	if _, err := f.run(t, t.Context()); err != nil {
		t.Fatalf("skipped members must not fail the run: %v", err)
	}
	f.requireOrder(t, "addMember:cache/appdata/vmdisks:zvol", "backupDir:"+zfsRootSnapD)
	f.requireOrder(t, "addMember:cache/appdata/old:not-mounted", "backupDir:"+zfsRootSnapD)
	if !f.recorder.member(t, zfsTestPlex).IsNew {
		t.Fatal("a newly picked up child must be recorded as new")
	}
	if f.recorder.member(t, zfsTestRoot).IsNew {
		t.Fatal("a known member must not be recorded as new")
	}
}

func TestDestroyRecursiveWithRetryRetriesOnlyBusy(t *testing.T) {
	log := &zfsLog{}
	busy := &fakeZFSSnapshotter{log: log, destroyErrs: []error{zfsBusyErr(), zfsBusyErr(), nil}}
	clock := &zfsClock{now: zfsTestNow}
	if err := backup.DestroyRecursiveWithRetry(t.Context(), busy, zfsTestRoot, zfsTestSnap, zfsTestBudget, clock.time, clock.sleep, false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if busy.destroyCalls != 3 {
		t.Fatalf("destroy attempts = %d, want 3", busy.destroyCalls)
	}

	other := &fakeZFSSnapshotter{log: log, destroyErrs: []error{&zfs.CmdError{Code: "zfs-permission", Stderr: "permission denied"}}}
	clock = &zfsClock{now: zfsTestNow}
	if err := backup.DestroyRecursiveWithRetry(t.Context(), other, zfsTestRoot, zfsTestSnap, zfsTestBudget, clock.time, clock.sleep, false); err == nil {
		t.Fatal("a destroy that is not busy must not be retried and must return its error")
	}
	if other.destroyCalls != 1 {
		t.Fatalf("destroy attempts = %d, want 1", other.destroyCalls)
	}
}

func TestDestroyRecursiveWithRetryStaysWithinBudget(t *testing.T) {
	log := &zfsLog{}
	z := &fakeZFSSnapshotter{log: log, destroyErrs: []error{zfsBusyErr()}}
	clock := &zfsClock{now: zfsTestNow}
	start := clock.now
	if err := backup.DestroyRecursiveWithRetry(t.Context(), z, zfsTestRoot, zfsTestSnap, zfsTestBudget, clock.time, clock.sleep, false); err == nil {
		t.Fatal("a snapshot that stays busy must report the failure")
	}
	if got := clock.elapsed(start); got > zfsTestBudget {
		t.Fatalf("retries used %v of virtual time, want at most %v", got, zfsTestBudget)
	}
	if z.destroyCalls < 2 {
		t.Fatalf("destroy attempts = %d, want the budget to allow a retry", z.destroyCalls)
	}
}

func TestDestroyRecursiveOnceMakesOneAttempt(t *testing.T) {
	log := &zfsLog{}
	z := &fakeZFSSnapshotter{log: log, destroyErrs: []error{zfsBusyErr()}}
	clock := &zfsClock{now: zfsTestNow}
	if err := backup.DestroyRecursiveWithRetry(t.Context(), z, zfsTestRoot, zfsTestSnap, zfsTestBudget, clock.time, clock.sleep, true); err == nil {
		t.Fatal("the single attempt must report the failure")
	}
	if z.destroyCalls != 1 {
		t.Fatalf("destroy attempts = %d, want 1", z.destroyCalls)
	}
	if len(clock.slept) != 0 {
		t.Fatalf("nothing may wait during a shutdown, slept %v", clock.slept)
	}
}

func TestDestroyRecursiveNotFoundIsDone(t *testing.T) {
	log := &zfsLog{}
	z := &fakeZFSSnapshotter{log: log, destroyErrs: []error{zfsNotFoundErr()}}
	clock := &zfsClock{now: zfsTestNow}
	if err := backup.DestroyRecursiveWithRetry(t.Context(), z, zfsTestRoot, zfsTestSnap, zfsTestBudget, clock.time, clock.sleep, false); err != nil {
		t.Fatalf("a snapshot that is already gone must count as destroyed, got %v", err)
	}
	if z.destroyCalls != 1 {
		t.Fatalf("destroy attempts = %d, want 1", z.destroyCalls)
	}
}

func TestAnchorExcludes(t *testing.T) {
	dir := `/host/user/cache/Media [a]*?\z/.zfs/snapshot/` + zfsTestSnap
	got := backup.AnchorExcludes(dir, []string{"/Library/Cache", "*.tmp"})
	want := []string{`/host/user/cache/Media \[a]\*\?\\z/.zfs/snapshot/` + zfsTestSnap + "/Library/Cache", "*.tmp"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("AnchorExcludes\ngot:  %v\nwant: %v", got, want)
	}
}

func TestSplitExcludes(t *testing.T) {
	relPaths := []string{"/", "/plex", "/plexy"}
	excludes := []string{"/plex/a", "/a", "/plexy/a", "*.tmp", "/", "*.tmp", "Cache"}
	got := backup.SplitExcludes(excludes, relPaths)
	want := map[string][]string{
		"/":      {"/a", "*.tmp", "*.tmp", "Cache"},
		"/plex":  {"/a", "*.tmp", "*.tmp", "Cache"},
		"/plexy": {"/a", "*.tmp", "*.tmp", "Cache"},
	}
	if len(got) != len(want) {
		t.Fatalf("SplitExcludes returned %d members, want %d: %v", len(got), len(want), got)
	}
	for rel, wantPatterns := range want {
		if strings.Join(got[rel], "|") != strings.Join(wantPatterns, "|") {
			t.Fatalf("patterns for %q\ngot:  %v\nwant: %v", rel, got[rel], wantPatterns)
		}
	}
}
