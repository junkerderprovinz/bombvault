package backup

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/zfs"
)

// ErrZFSRefusal marks the errors a ZFS backup finishes a run with. Their text is
// built from validated dataset names, reason codes, host mountpoints and
// container names, so the path scrubber passes it through and a name like
// cache/appdata still reads as itself in the run history.
var ErrZFSRefusal = errors.New("zfs backup refused")

// ZFSRefusal is why a ZFS backup refused to run, or how its members failed.
type ZFSRefusal struct {
	// Code is a reason code from internal/zfs. It is empty when Detail already
	// carries one code per member.
	Code   string
	Detail string
}

func (e *ZFSRefusal) Error() string {
	if e.Code == "" {
		return e.Detail
	}
	if e.Detail == "" {
		return e.Code
	}
	return e.Code + ": " + e.Detail
}

func (e *ZFSRefusal) Is(target error) bool { return target == ErrZFSRefusal }

// ZFSSnapshotter is the host side of a run: one atomic snapshot over the whole
// tree, the automount prime per member, and the destroy that ends the run.
type ZFSSnapshotter interface {
	SnapshotRecursive(ctx context.Context, root, snap string) error
	DestroyRecursive(ctx context.Context, root, snap string) error
	Prime(ctx context.Context, hostMountpoint, snap string) error
}

// ZFSBackupSummary is what restic reports for one member. It carries the file
// counters the run records per member, which the domain's baselines read.
type ZFSBackupSummary struct {
	SnapshotID      string
	BytesAdded      int64
	FilesNew        int64
	FilesChanged    int64
	FilesUnmodified int64
}

// ZFSRestic backs up the contents of a snapshot directory as the snapshot's own
// tree root, so a member's tree root stays the dataset root from run to run.
type ZFSRestic interface {
	BackupDir(ctx context.Context, repo, dir string, tags []string, excludes ...string) (ZFSBackupSummary, error)
}

// ZFSConsistency stops the configured containers for the snapshot instant.
// Freeze returns thaw, which is called exactly once on a context that cannot be
// cancelled; Freeze itself restarts whatever it stopped before returning an
// error. window reports how long the apps were down.
type ZFSConsistency interface {
	Freeze(ctx context.Context) (thaw func(context.Context), window func() time.Duration, err error)
}

// ZFSHooks runs the item's pre- and post-snapshot commands.
type ZFSHooks interface {
	Pre(ctx context.Context) error
	Post(ctx context.Context) error
}

// ZFSMemberPlan is one readable member of the tree, resolved by the preflight.
type ZFSMemberPlan struct {
	Dataset string
	// RelPath is the member's place in the item's logical tree: "/" for the
	// root, "/plex" for cache/appdata/plex.
	RelPath             string
	HostMountpoint      string
	ContainerMountpoint string
	IsNew               bool
}

// ZFSMemberResult is what a run records per member.
type ZFSMemberResult struct {
	Dataset         string
	Outcome         string
	ResticSnapshot  string
	IsNew           bool
	BytesAdded      int64
	FilesNew        int64
	FilesChanged    int64
	FilesUnmodified int64
	DurationMS      int64
}

// ZFSRunRecorder stores the run's snapshot, its consistency window and the
// outcome of every member.
type ZFSRunRecorder interface {
	RecordRun(runID, snap string, window time.Duration, hookDetail string) error
	AddMember(runID string, m ZFSMemberResult) error
}

// ZFSBackupDeps bundles everything BackupZFSItem needs.
type ZFSBackupDeps struct {
	Root string
	// Members are the readable members in tree order; a member the preflight
	// skipped is in Skipped instead.
	Members []ZFSMemberPlan
	Skipped []ZFSMemberResult
	Repo    string
	// TargetID is the item's row id, so renaming the root keeps its history.
	TargetID string
	// Excludes are the item's patterns, relative to its logical tree.
	Excludes []string
	// Now names the snapshot; Clock measures durations and the destroy budget.
	Now    func() time.Time
	Clock  func() time.Time
	ZFS    ZFSSnapshotter
	Restic ZFSRestic
	// Visible waits until the member's snapshot mount has reached the container
	// and returns a *ZFSRefusal when it never does.
	Visible     func(ctx context.Context, dataset, snap, snapDir string) error
	DirEmpty    func(dir string) (bool, error)
	Consistency ZFSConsistency
	Hooks       ZFSHooks
	Runs        Runs
	Recorder    ZFSRunRecorder
	// DestroyBudget is how long the destroy may keep retrying a busy snapshot.
	DestroyBudget time.Duration
	// ShuttingDown makes the destroy a single attempt, because whatever Docker's
	// stop timeout leaves is not enough to promise more.
	ShuttingDown    func() bool
	Sleep           func(context.Context, time.Duration)
	OnDestroyFailed func(root, snap string, err error)
}

const (
	zfsTagPrefix = "zfs:"

	outcomeBackedUp        = "backed-up"
	outcomeEmpty           = "empty"
	codeNotReached         = "not-reached"
	codeBackupFailed       = "backup-failed"
	codeSnapshotNotVisible = "snapshot-not-visible"
)

// BackupZFSItem backs up one dataset tree from a single recursive snapshot: the
// apps are down only for the snapshot instant, every member is read from its own
// .zfs/snapshot directory under its own identity tag, and the run is finished
// before the snapshot is destroyed so a shutdown during the cleanup cannot leave
// the row to the reaper.
func BackupZFSItem(ctx context.Context, d ZFSBackupDeps) (Summary, error) {
	runID, err := d.Runs.Start(d.TargetID, kindBackup)
	if err != nil {
		return Summary{}, fmt.Errorf("zfs backup: start run: %w", err)
	}

	if d.Hooks != nil {
		if hookErr := d.Hooks.Pre(ctx); hookErr != nil {
			return Summary{}, d.failRun(runID, &ZFSRefusal{Code: "pre-snapshot-failed", Detail: truncateErr(hookErr)})
		}
	}

	var thaw func(context.Context)
	var windowOf func() time.Duration
	if d.Consistency != nil {
		frozen, measured, freezeErr := d.Consistency.Freeze(ctx)
		if freezeErr != nil {
			// The pre command has already put the app into its backup state, and
			// only the post command takes it out again.
			d.post(ctx)
			return Summary{}, d.failRun(runID, freezeErr)
		}
		thaw, windowOf = frozen, measured
	}

	snap := zfs.SnapshotName(d.Now())
	snapErr := d.ZFS.SnapshotRecursive(ctx, d.Root, snap)

	// The apps come back on a context nothing can cancel, so a user cancel or a
	// shutdown between the snapshot and here cannot leave them down.
	detached := context.WithoutCancel(ctx)
	var window time.Duration
	if thaw != nil {
		thaw(detached)
		window = windowOf()
	}
	hookDetail := d.post(detached)
	if recErr := d.Recorder.RecordRun(runID, snap, window, hookDetail); recErr != nil {
		log.Printf("zfs backup: record run for %s: %v", d.Root, recErr)
	}
	if snapErr != nil {
		return Summary{}, d.failRun(runID, &ZFSRefusal{Code: "snapshot-failed", Detail: d.Root + ": " + zfsHostDetail(snapErr)})
	}

	for _, m := range d.Skipped {
		d.addMember(runID, m)
	}

	summary, failures := d.backupMembers(ctx, runID, snap)

	status := statusSuccess
	var runErr error
	if len(failures) > 0 {
		status = statusFailed
		runErr = &ZFSRefusal{Detail: strings.Join(failures, ", ")}
	}
	if finErr := d.Runs.Finish(runID, status, summary, truncateErr(runErr)); finErr != nil {
		log.Printf("zfs backup: finish run for %s: %v", d.Root, finErr)
	}

	d.destroy(ctx, snap)
	return summary, runErr
}

// post runs the post-snapshot command on a context nothing cancels and returns
// what a failing command said, for the run's detail.
func (d ZFSBackupDeps) post(ctx context.Context) string {
	if d.Hooks == nil {
		return ""
	}
	if err := d.Hooks.Post(context.WithoutCancel(ctx)); err != nil {
		log.Printf("zfs backup: post-snapshot command for %s failed: %v", d.Root, err)
		return truncateErr(err)
	}
	return ""
}

// failRun records a refusal that ends the run before any member was read.
func (d ZFSBackupDeps) failRun(runID string, err error) error {
	if finErr := d.Runs.Finish(runID, statusFailed, Summary{}, truncateErr(err)); finErr != nil {
		log.Printf("zfs backup: finish run for %s: %v", d.Root, finErr)
	}
	return err
}

func (d ZFSBackupDeps) addMember(runID string, m ZFSMemberResult) {
	if err := d.Recorder.AddMember(runID, m); err != nil {
		log.Printf("zfs backup: record member %s: %v", m.Dataset, err)
	}
}

// backupMembers reads every member of the frozen tree and returns the item's
// summary together with the members that failed, named for the run's error.
func (d ZFSBackupDeps) backupMembers(ctx context.Context, runID, snap string) (Summary, []string) {
	relPaths := make([]string, len(d.Members))
	for i, m := range d.Members {
		relPaths[i] = m.RelPath
	}
	split := SplitExcludes(d.Excludes, relPaths)

	var summary Summary
	var failures []string
	for i, m := range d.Members {
		if ctx.Err() != nil {
			for _, rest := range d.Members[i:] {
				res := ZFSMemberResult{Dataset: rest.Dataset, Outcome: codeNotReached, IsNew: rest.IsNew}
				d.addMember(runID, res)
				failures = append(failures, memberFailure(res))
			}
			break
		}
		start := d.Clock()
		res := d.backupMember(ctx, m, snap, split[m.RelPath])
		res.DurationMS = d.Clock().Sub(start).Milliseconds()
		d.addMember(runID, res)

		switch res.Outcome {
		case outcomeBackedUp:
			summary.Bytes += res.BytesAdded
			// The root's restic snapshot identifies the restore point; a tree
			// whose root could not be read falls back to its first member.
			if summary.SnapshotID == "" || m.RelPath == "/" {
				summary.SnapshotID = res.ResticSnapshot
			}
		case outcomeEmpty:
		default:
			failures = append(failures, memberFailure(res))
		}
	}
	return summary, failures
}

func (d ZFSBackupDeps) backupMember(ctx context.Context, m ZFSMemberPlan, snap string, excludes []string) ZFSMemberResult {
	res := ZFSMemberResult{Dataset: m.Dataset, IsNew: m.IsNew}
	snapDir := m.ContainerMountpoint + "/.zfs/snapshot/" + snap

	// The prime runs in the host's mount namespace, where the kernel resolves the
	// automount against the real mountpoint; the container then only has to
	// receive the mount.
	if err := d.ZFS.Prime(ctx, m.HostMountpoint, snap); err != nil {
		log.Printf("zfs backup: priming %s on the host failed, reading it from the container instead: %v", m.Dataset, err)
	}
	if err := d.Visible(ctx, m.Dataset, snap, snapDir); err != nil {
		res.Outcome = refusalCode(err, codeSnapshotNotVisible)
		return res
	}
	empty, err := d.DirEmpty(snapDir)
	if err != nil {
		log.Printf("zfs backup: reading the snapshot of %s failed: %v", m.Dataset, err)
		res.Outcome = codeSnapshotNotVisible
		return res
	}
	if empty {
		// restic fails on an empty source, and a dataset may legitimately hold
		// nothing at this instant.
		res.Outcome = outcomeEmpty
		return res
	}

	// Exactly one tag. restic groups retention by it and finds the previous
	// snapshot of the same dataset through it, so a second tag would reset every
	// member's parent and read the whole tree in full every night.
	tags := []string{zfsTagPrefix + m.Dataset}
	sum, err := d.Restic.BackupDir(ctx, d.Repo, snapDir, tags, AnchorExcludes(snapDir, excludes)...)
	if err != nil {
		log.Printf("zfs backup: reading %s failed: %v", m.Dataset, err)
		res.Outcome = codeBackupFailed
		return res
	}
	res.Outcome = outcomeBackedUp
	res.ResticSnapshot = sum.SnapshotID
	res.BytesAdded = sum.BytesAdded
	res.FilesNew = sum.FilesNew
	res.FilesChanged = sum.FilesChanged
	res.FilesUnmodified = sum.FilesUnmodified
	return res
}

func (d ZFSBackupDeps) destroy(ctx context.Context, snap string) {
	// The snapshot has to go although the run was cancelled or the server is
	// stopping, or it pins deleted blocks until a sweep finds it.
	err := DestroyRecursiveWithRetry(context.WithoutCancel(ctx), d.ZFS, d.Root, snap,
		d.DestroyBudget, d.Clock, d.Sleep, d.ShuttingDown())
	if err == nil {
		return
	}
	log.Printf("zfs backup: removing %s@%s failed: %v", d.Root, snap, err)
	if d.OnDestroyFailed != nil {
		d.OnDestroyFailed(d.Root, snap, err)
	}
}

func memberFailure(m ZFSMemberResult) string { return m.Dataset + " [" + m.Outcome + "]" }

// zfsHostDetail makes what the host wrote fit for a refusal, which passes the
// run row's scrubber untouched: the host's paths and control characters go.
func zfsHostDetail(err error) string {
	return scrubRunErr(strings.Map(func(r rune) rune {
		if r == '\n' || r == '\t' || r >= 0x20 {
			return r
		}
		return -1
	}, err.Error()))
}

// refusalCode returns the reason code a refusal carries, or fallback when the
// seam answered with an ordinary error.
func refusalCode(err error, fallback string) string {
	var refusal *ZFSRefusal
	if errors.As(err, &refusal) && refusal.Code != "" {
		return refusal.Code
	}
	return fallback
}

// excludeMetaEscaper protects the glob characters a mountpoint may contain, so a
// directory such as "/mnt/cache/Media [old]" still anchors as itself.
var excludeMetaEscaper = strings.NewReplacer(`\`, `\\`, `*`, `\*`, `?`, `\?`, `[`, `\[`)

// destroyBackoff is what a busy snapshot is given between attempts.
var destroyBackoff = []time.Duration{
	250 * time.Millisecond,
	500 * time.Millisecond,
	time.Second,
	2 * time.Second,
	4 * time.Second,
}

// destroyAttemptCap bounds one attempt so a hung connection cannot eat the whole
// budget.
const destroyAttemptCap = 3 * time.Second

// SplitExcludes hands every member the patterns that apply to it: an anchored
// pattern goes to the member whose logical path is its longest prefix, re-rooted
// there, an unanchored one goes to every member unchanged.
func SplitExcludes(excludes []string, relPaths []string) map[string][]string {
	split := make(map[string][]string, len(relPaths))
	for _, rel := range relPaths {
		split[rel] = nil
	}
	for _, pattern := range excludes {
		p := strings.TrimSuffix(pattern, "/")
		if p == "" {
			log.Printf("zfs backup: exclude %q would exclude the whole tree, ignoring it", pattern)
			continue
		}
		if !strings.HasPrefix(p, "/") {
			for _, rel := range relPaths {
				split[rel] = append(split[rel], p)
			}
			continue
		}
		owner := longestMemberPrefix(p, relPaths)
		split[owner] = append(split[owner], strings.TrimPrefix(p, strings.TrimSuffix(owner, "/")))
	}
	return split
}

// longestMemberPrefix finds the member an anchored pattern belongs to. The match
// needs the separator, so /plexy never lands under /plex.
func longestMemberPrefix(pattern string, relPaths []string) string {
	owner := "/"
	for _, rel := range relPaths {
		if rel != "/" && strings.HasPrefix(pattern, rel+"/") && len(rel) > len(owner) {
			owner = rel
		}
	}
	return owner
}

// AnchorExcludes re-roots a member's anchored patterns at the snapshot directory
// restic reads, because restic matches an anchored pattern against the absolute
// path and that path carries the run's snapshot name.
func AnchorExcludes(snapDir string, excludes []string) []string {
	if len(excludes) == 0 {
		return nil
	}
	prefix := excludeMetaEscaper.Replace(snapDir)
	anchored := make([]string, 0, len(excludes))
	for _, p := range excludes {
		if strings.HasPrefix(p, "/") {
			anchored = append(anchored, prefix+p)
			continue
		}
		anchored = append(anchored, p)
	}
	return anchored
}

// DestroyRecursiveWithRetry removes the run's snapshot from the whole tree. A
// busy snapshot is retried while the budget allows another attempt; a snapshot
// that is already gone counts as destroyed. With once set, one attempt is made
// and nothing waits, because a shutdown has no room for more.
func DestroyRecursiveWithRetry(ctx context.Context, z ZFSSnapshotter, root, snap string, budget time.Duration, clock func() time.Time, sleep func(context.Context, time.Duration), once bool) error {
	start := clock()
	for attempt := 0; ; attempt++ {
		attemptCtx, cancel := context.WithTimeout(ctx, destroyAttemptTimeout(budget, clock().Sub(start)))
		err := z.DestroyRecursive(attemptCtx, root, snap)
		cancel()
		switch {
		case err == nil, zfs.IsNotFound(err):
			return nil
		case once, !zfs.IsBusy(err), attempt >= len(destroyBackoff):
			return err
		}
		wait := destroyBackoff[attempt]
		if clock().Sub(start)+wait >= budget {
			return err
		}
		sleep(ctx, wait)
	}
}

func destroyAttemptTimeout(budget, elapsed time.Duration) time.Duration {
	remaining := budget - elapsed
	if remaining <= 0 || remaining > destroyAttemptCap {
		return destroyAttemptCap
	}
	return remaining
}
