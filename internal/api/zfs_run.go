package api

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/backup"
	"github.com/junkerderprovinz/bombvault/internal/notify"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
	"github.com/junkerderprovinz/bombvault/internal/zfs"
)

// zfsMember is one dataset of an item's tree as the preflight resolved it.
type zfsMember struct {
	Entry   zfs.ListEntry
	RelPath string
	Mount   zfs.MountRecord
	// Code is empty when the member can be read, otherwise the member code the
	// page turns into a sentence.
	Code  string
	IsNew bool
}

// zfsPreflight lists the item's tree, decides what can be read and refuses the
// whole item when the tree is unusable. previous is the member list of the last
// run, which is what makes a child picked up for the first time visible as new.
func (s *Service) zfsPreflight(ctx context.Context, d store.ZFSDataset, previous []store.ZFSMember) ([]zfsMember, *backup.ZFSRefusal) {
	if other, ok := s.zfsOverlappingItem(d); ok {
		return nil, &backup.ZFSRefusal{Code: "overlaps-item", Detail: other}
	}
	lctx, cancel := context.WithTimeout(ctx, zfsListTimeout)
	defer cancel()
	tree, err := s.zfs.Tree(lctx, d.Dataset)
	if err != nil {
		code := zfs.Classify("", err)
		if zfs.IsNotFound(err) {
			code = "not-found"
		}
		return nil, &backup.ZFSRefusal{Code: code, Detail: err.Error()}
	}
	if tree[0].Type != "filesystem" {
		return nil, &backup.ZFSRefusal{Code: "not-filesystem", Detail: d.Dataset}
	}
	legacy := 0
	for _, e := range tree {
		if !zfs.SnapshotNameFits(e.Name) {
			// The recursive snapshot is one command over the whole tree, so a
			// single name that does not fit fails every member with it.
			return nil, &backup.ZFSRefusal{Code: "name-too-long", Detail: e.Name}
		}
		if e.Type == "filesystem" && e.Mountpoint == "legacy" {
			legacy++
		}
	}
	if legacy > zfsMaxLegacyFilesystems {
		return nil, &backup.ZFSRefusal{Code: "docker-storage", Detail: d.Dataset}
	}

	seen := make(map[string]bool, len(previous))
	for _, p := range previous {
		seen[p.Dataset] = true
	}
	recs := zfsMountRecords()
	members := make([]zfsMember, 0, len(tree))
	readable := 0
	for _, e := range tree {
		m := zfsMember{
			Entry:   e,
			RelPath: zfsRelPath(d.Dataset, e.Name),
			Code:    zfs.MemberCode(e, d.ExcludedChildren),
			IsNew:   !seen[e.Name],
		}
		if m.Code == "" {
			m.Mount, m.Code = s.resolveDatasetMount(recs, e, false)
		}
		if m.Code == "" {
			readable++
		}
		members = append(members, m)
	}
	if readable == 0 {
		return nil, &backup.ZFSRefusal{Code: "nothing-readable", Detail: d.Dataset}
	}

	rows := make([]store.ZFSMember, 0, len(members))
	for _, m := range members {
		rows = append(rows, store.ZFSMember{
			ItemID:         d.ID,
			Dataset:        m.Entry.Name,
			HostMountpoint: m.Entry.Mountpoint,
			Outcome:        m.Code,
			UsedByDataset:  m.Entry.UsedByDataset,
		})
	}
	if err := s.store.ReplaceZFSMembers(d.ID, rows); err != nil {
		log.Printf("api: zfs: recording the tree of %s failed: %v", d.Dataset, err)
	}
	if _, err := s.store.SetZFSCheck(d.ID, "ok", "", tree[0].Mountpoint, time.Now().Unix()); err != nil {
		log.Printf("api: zfs: recording the check of %s failed: %v", d.Dataset, err)
	}
	return members, nil
}

// zfsOverlappingItem names the item whose tree shares datasets with d. Two
// items over one tree would destroy each other's snapshot stamps.
func (s *Service) zfsOverlappingItem(d store.ZFSDataset) (string, bool) {
	rows, err := s.store.ListZFSDatasets()
	if err != nil {
		log.Printf("api: zfs: could not check for overlapping items: %v", err)
		return "", false
	}
	for _, other := range rows {
		if other.ID == d.ID {
			continue
		}
		if zfs.DescendantOf(d.Dataset, other.Dataset) || zfs.DescendantOf(other.Dataset, d.Dataset) {
			return other.Dataset, true
		}
	}
	return "", false
}

// zfsRefusalSentence is the English run error a refusal leaves in the history.
// It ends in the reason code, from which the page builds its own sentence.
func zfsRefusalSentence(root string, ref *backup.ZFSRefusal) string {
	switch {
	case ref.Code == "":
		return "zfs backup: " + root + ": " + ref.Detail
	case ref.Detail == "":
		return "zfs backup: " + root + " [" + ref.Code + "]"
	default:
		return "zfs backup: " + root + ": " + ref.Detail + " [" + ref.Code + "]"
	}
}

// recordZFSRefusal turns a refusal that happened before the orchestrator into a
// failed run, so a scheduled item that cannot be read shows up in Run History
// instead of only in the log.
func (s *Service) recordZFSRefusal(ctx context.Context, d store.ZFSDataset, ref *backup.ZFSRefusal) error {
	err := &backup.ZFSRefusal{Detail: zfsRefusalSentence(d.Dataset, ref)}
	if _, sErr := s.store.SetZFSCheck(d.ID, ref.Code, ref.Detail, d.LastHostMountpoint, time.Now().Unix()); sErr != nil {
		log.Printf("api: zfs: recording the check of %s failed: %v", d.Dataset, sErr)
	}
	if runID, sErr := s.store.StartRun(d.ID, "backup"); sErr != nil {
		log.Printf("api: zfs: recording the refused run of %s failed: %v", d.Dataset, sErr)
	} else if fErr := s.store.FinishRun(runID, "failed", "", 0, truncateRunErr(err)); fErr != nil {
		log.Printf("api: zfs: finishing the refused run of %s failed: %v", d.Dataset, fErr)
	}
	s.notifyBackup(ctx, zfsDomain, d.Dataset, false, backup.Summary{}, err)
	return err
}

// notifyZFSUnsuppressed sends a message that has to reach the user although the
// run it belongs to is part of a scheduled summary.
func (s *Service) notifyZFSUnsuppressed(ev notify.Event) {
	c, err := s.NotifyConfig()
	if err != nil || c.On == "" || c.On == "never" {
		return
	}
	notify.Send(context.Background(), c, zfsDomain, ev)
}

// applyRetentionTags ages every member of a tree under its own identity tag and
// reclaims the space once at the end. One tag for the whole item would put all
// its datasets into a single keep-N group and age whole datasets out.
func (s *Service) applyRetentionTags(ctx context.Context, repo string, settings store.Settings, mode restic.Mode, tags []string, domain string) {
	p := s.retentionPolicy(settings)
	if !p.Any() || len(tags) == 0 {
		return
	}
	if s.primaryIsImmutable(domain, repo) {
		log.Printf("api: %s: retention skipped: %s is flagged append-only", domain, shortRepoName(repo))
		return
	}
	for _, tag := range tags {
		if err := s.forgetWithLockHeal(ctx, repo, p, mode, []string{tag}, false); err != nil {
			log.Printf("api: retention prune failed (backup is safe): %v", err)
			s.notifyRetentionFailed(ctx, tag, truncateRunErr(err))
		}
	}
	if bulkReplicateSuppressed(ctx) {
		return // one batched prune after the whole loop
	}
	if err := s.engine.Prune(ctx, repo, mode); err != nil {
		log.Printf("api: %s: reclaiming the space retention freed failed: %v", domain, err)
	}
}

// zfsRunHost gives the host calls of one run their own deadlines, so a hung SSH
// session cannot hold the domain lock for the length of the run. The destroy
// keeps the context it is given: the orchestrator bounds each of its attempts
// against the retry budget.
type zfsRunHost struct{ h zfs.Host }

func (z zfsRunHost) SnapshotRecursive(ctx context.Context, root, snap string) error {
	ctx, cancel := context.WithTimeout(ctx, zfsSnapshotTimeout)
	defer cancel()
	return z.h.SnapshotRecursive(ctx, root, snap)
}

func (z zfsRunHost) DestroyRecursive(ctx context.Context, root, snap string) error {
	return z.h.DestroyRecursive(ctx, root, snap)
}

func (z zfsRunHost) Prime(ctx context.Context, hostMountpoint, snap string) error {
	ctx, cancel := context.WithTimeout(ctx, zfsPrimeTimeout)
	defer cancel()
	return z.h.Prime(ctx, hostMountpoint, snap)
}

// zfsRunRecorder writes what the runs table has no column for.
type zfsRunRecorder struct {
	st     *store.Repo
	itemID string
	// stops is whether the item holds containers for the snapshot instant. An
	// item that stops nothing has no window to report.
	stops bool
}

func (r zfsRunRecorder) RecordRun(runID, snap string, window time.Duration, hookDetail string) error {
	seconds := int64(-1)
	if r.stops {
		seconds = int64(window / time.Second)
	}
	return r.st.RecordZFSRun(runID, r.itemID, snap, seconds, hookDetail)
}

func (r zfsRunRecorder) AddMember(runID string, m backup.ZFSMemberResult) error {
	return r.st.AddZFSRunMember(store.ZFSRunMember{
		RunID:           runID,
		Dataset:         m.Dataset,
		Outcome:         m.Outcome,
		ResticSnapshot:  m.ResticSnapshot,
		IsNew:           m.IsNew,
		BytesAdded:      m.BytesAdded,
		FilesNew:        m.FilesNew,
		FilesChanged:    m.FilesChanged,
		FilesUnmodified: m.FilesUnmodified,
		DurationMS:      m.DurationMS,
	})
}

// BackupZFSDataset backs up one item: its whole tree from a single recursive
// snapshot, one restic run per readable member, then the snapshot is destroyed.
func (s *Service) BackupZFSDataset(ctx context.Context, id string) (backup.Summary, error) {
	ctx, cancel := backupHoldCtx(ctx)
	defer cancel()
	defer s.lockDomain(zfsDomain)()

	settings, err := s.store.GetSettings()
	if err != nil {
		return backup.Summary{}, fmt.Errorf("read settings: %w", err)
	}
	d, err := s.store.GetZFSDataset(id)
	if err != nil {
		return backup.Summary{}, fmt.Errorf("zfs backup: load dataset: %w", err)
	}
	// The root dataset, not the row id: it is the key the progress stream
	// publishes under, and the only one a Cancel button ever has in hand.
	key := zfsDomain + ":" + d.Dataset
	s.registerBackupCancel(key, cancel)
	defer s.unregisterBackupCancel(key)

	if s.zfs == nil {
		return backup.Summary{}, s.recordZFSRefusal(ctx, d, &backup.ZFSRefusal{Code: "ssh-missing"})
	}
	previous, err := s.store.ListZFSMembers(d.ID)
	if err != nil {
		return backup.Summary{}, fmt.Errorf("zfs backup: load the tree of the previous run: %w", err)
	}
	members, refusal := s.zfsPreflight(ctx, d, previous)
	if refusal != nil {
		return backup.Summary{}, s.recordZFSRefusal(ctx, d, refusal)
	}
	if _, sErr := s.sweepZFSTree(ctx, d); sErr != nil {
		log.Printf("api: zfs: sweeping the leftovers of %s before the run failed: %v", d.Dataset, sErr)
	}

	repo, err := s.zfsDatasetRepoPath(settings, d)
	if err != nil {
		return backup.Summary{}, err
	}
	mode := s.primaryModeFor(settings, zfsDomain, repo)
	if err := s.EnsureRepo(ctx, repo, mode); err != nil {
		return backup.Summary{}, err
	}
	s.unlockStale(ctx, repo, mode)
	s.notifyBackupStart(ctx, zfsDomain)
	rctx, startedAt := s.progBegin(ctx, key, "backup")

	plans, skipped, tags := zfsSplitMembers(members)
	sum, runErr := backup.BackupZFSItem(rctx, backup.ZFSBackupDeps{
		Root:          d.Dataset,
		Members:       plans,
		Skipped:       skipped,
		Repo:          repo,
		TargetID:      d.ID,
		Excludes:      d.Excludes,
		Now:           time.Now,
		Clock:         time.Now,
		ZFS:           zfsRunHost{h: s.zfs},
		Restic:        &resticAdapter{engine: s.engine, mode: mode},
		Visible:       s.visibleSnapshot,
		DirEmpty:      zfsDirEmpty,
		Runs:          runsAdapter{st: s.store, ctx: ctx, svc: s, cancelKey: key},
		Recorder:      zfsRunRecorder{st: s.store, itemID: d.ID, stops: len(d.StopContainers) > 0},
		DestroyBudget: zfsDestroyBudget,
		ShuttingDown:  s.IsShuttingDown,
		Sleep:         sleepFor,
		OnDestroyFailed: func(root, snap string, destroyErr error) {
			s.recordZFSLeftover(d, root, snap, destroyErr)
		},
	})
	s.progEnd(key, "backup", runErr == nil, startedAt)
	s.notifyBackup(ctx, zfsDomain, d.Dataset, runErr == nil, sum, runErr)
	if changes := zfsMemberChanges(previous, members); len(changes) > 0 {
		s.notifyZFSUnsuppressed(notify.Event{
			Title:   "BombVault",
			Message: "The datasets below " + d.Dataset + " changed: " + strings.Join(changes, ", "),
			OK:      true,
		})
	}

	// Retention runs whether or not a member failed: the members that were read
	// wrote real snapshots, and their history ages under the same policy.
	s.applyRetentionTags(ctx, repo, settings, mode, tags, zfsDomain)
	makeRepoReadable(repo, s.cfg.DataDir)
	s.replicateOffsite(ctx, zfsDomain, settings, mode, repo)
	s.collectStatsAfterItem(ctx, zfsDomain)
	s.checkPrimaryRemoteBudget(ctx, zfsDomain, repo, settings)
	if runErr != nil {
		return backup.Summary{}, runErr
	}
	return sum, nil
}

// zfsSplitMembers separates what the run reads from what it only records, and
// collects the identity tags retention has to age one by one.
func zfsSplitMembers(members []zfsMember) ([]backup.ZFSMemberPlan, []backup.ZFSMemberResult, []string) {
	var plans []backup.ZFSMemberPlan
	var skipped []backup.ZFSMemberResult
	var tags []string
	for _, m := range members {
		if m.Code != "" {
			skipped = append(skipped, backup.ZFSMemberResult{
				Dataset: m.Entry.Name, Outcome: m.Code, IsNew: m.IsNew,
			})
			continue
		}
		plans = append(plans, backup.ZFSMemberPlan{
			Dataset:             m.Entry.Name,
			RelPath:             m.RelPath,
			HostMountpoint:      m.Entry.Mountpoint,
			ContainerMountpoint: m.Mount.MountPoint,
			IsNew:               m.IsNew,
		})
		tags = append(tags, "zfs:"+m.Entry.Name)
	}
	return plans, skipped, tags
}

// zfsMemberChanges names the datasets the tree gained or lost since the
// previous run, and the ones whose readability changed.
func zfsMemberChanges(previous []store.ZFSMember, members []zfsMember) []string {
	was := make(map[string]string, len(previous))
	for _, p := range previous {
		was[p.Dataset] = p.Outcome
	}
	present := make(map[string]bool, len(members))
	var out []string
	for _, m := range members {
		present[m.Entry.Name] = true
		old, known := was[m.Entry.Name]
		switch {
		case !known:
			out = append(out, m.Entry.Name+" is new")
		case m.Code == old: // the same skip every night is not news
		case m.Code != "":
			out = append(out, m.Entry.Name+" is skipped ["+m.Code+"]")
		default:
			out = append(out, m.Entry.Name+" can be read again")
		}
	}
	for _, p := range previous {
		if !present[p.Dataset] {
			out = append(out, p.Dataset+" is gone")
		}
	}
	return out
}

// recordZFSLeftover counts a snapshot the run could not destroy and says so
// once, on a context the scheduled summary cannot swallow.
func (s *Service) recordZFSLeftover(d store.ZFSDataset, root, snap string, err error) {
	log.Printf("api: zfs: %s@%s could not be removed, the next sweep retries it: %v", root, snap, err)
	if sErr := s.store.SetZFSLeftovers(d.ID, d.LeftoverCount+1, time.Now().Unix()); sErr != nil {
		log.Printf("api: zfs: recording the leftover snapshot of %s failed: %v", root, sErr)
	}
	s.notifyZFSUnsuppressed(notify.Event{
		Title:   "BombVault",
		Message: "The snapshot " + root + "@" + snap + " could not be removed. BombVault will retry before the next backup.",
		OK:      false,
	})
}

// StartBackupZFSDataset runs one item in the background and returns at once.
func (s *Service) StartBackupZFSDataset(ctx context.Context, id string) (bool, error) {
	if !s.batchActive.CompareAndSwap(false, true) {
		return false, nil
	}
	if op, busy := s.domainBusy(zfsDomain); busy {
		s.batchActive.Store(false)
		return false, fmt.Errorf("%s is running on zfs", op)
	}
	bctx := context.WithoutCancel(ctx)
	go func() {
		defer s.recoverOperation("backup zfs dataset: "+id, nil, func(msg string) {
			s.failStuckRun(id, msg)
		})
		defer s.batchActive.Store(false)
		if _, err := s.BackupZFSDataset(bctx, id); err != nil {
			log.Printf("api: backup zfs dataset: %q failed: %v", id, err)
		}
	}()
	return true, nil
}

// StartBackupZFSAll runs the given items one after another in one background
// batch, replicating and pruning once when the loop is done.
func (s *Service) StartBackupZFSAll(ctx context.Context, ids []string) (bool, error) {
	if !s.batchActive.CompareAndSwap(false, true) {
		return false, nil
	}
	if op, busy := s.domainBusy(zfsDomain); busy {
		s.batchActive.Store(false)
		return false, fmt.Errorf("%s is running on zfs", op)
	}
	bctx := WithBulkReplicateSuppressed(context.WithoutCancel(ctx))
	go func() {
		defer s.recoverOperation("backup-zfs-all", nil, nil)
		defer s.batchActive.Store(false)

		queue := make([]string, 0, len(ids))
		for _, id := range ids {
			if id != "" {
				queue = append(queue, id)
			}
		}
		total := len(queue)
		const key = "batch:zfs"
		s.publishBatch(key, 0, true)
		ok, fail := 0, 0
		for i, id := range queue {
			if err := s.backupZFSOneForBatch(bctx, id); err != nil {
				fail++
				log.Printf("api: backup-zfs-all: %q failed (continuing): %v", id, err)
			} else {
				ok++
			}
			s.publishBatch(key, float64(i+1)/float64(total)*100, true)
		}
		s.publishBatch(key, 100, false)
		s.PruneAfterBulk(bctx, zfsDomain)
		s.ReplicateOffsiteAfterBulk(bctx, zfsDomain)
		s.maybeCollectStats(bctx, zfsDomain)
		log.Printf("api: backup-zfs-all done: %d ok, %d failed (of %d requested %d)", ok, fail, total, len(ids))
	}()
	return true, nil
}

// backupZFSOneForBatch keeps a panic in one item from aborting the rest of the
// batch.
func (s *Service) backupZFSOneForBatch(ctx context.Context, id string) (err error) {
	defer s.recoverOperation("backup-zfs-all: "+id, &err, func(msg string) {
		s.failStuckRun(id, msg)
	})
	_, err = s.BackupZFSDataset(ctx, id)
	return err
}

// SweepZFSLeftovers removes BombVault's own snapshot stamps left on an item's
// tree and reports how many survived.
func (s *Service) SweepZFSLeftovers(ctx context.Context, id string) (int, error) {
	d, err := s.store.GetZFSDataset(id)
	if err != nil {
		return 0, fmt.Errorf("zfs sweep: load dataset: %w", err)
	}
	defer s.lockDomainFor(zfsDomain, "sweep")()
	return s.sweepZFSTree(ctx, d)
}

// SweepZFSLeftoversOnStartup sweeps every item's tree, even with the domain
// switched off and the item disabled: a leaked snapshot pins deleted blocks
// either way. The caller holds the domain lock.
func (s *Service) SweepZFSLeftoversOnStartup(ctx context.Context) {
	rows, err := s.store.ListZFSDatasets()
	if err != nil {
		log.Printf("api: zfs: the startup sweep could not list the items: %v", err)
		return
	}
	for _, d := range rows {
		if ctx.Err() != nil {
			return
		}
		if _, sErr := s.sweepZFSTree(ctx, d); sErr != nil {
			log.Printf("api: zfs: the startup sweep of %s failed: %v", d.Dataset, sErr)
		}
	}
}

// LockDomainForStartupSweep takes the domain lock synchronously, so a scheduled
// job waits for the sweep although it runs in the background.
func (s *Service) LockDomainForStartupSweep() func() {
	return s.lockDomainFor(zfsDomain, "startup-sweep")
}

// sweepZFSTree is the sweep itself, without the domain lock: a backup run
// already holds it.
func (s *Service) sweepZFSTree(ctx context.Context, d store.ZFSDataset) (int, error) {
	if s.zfs == nil {
		return 0, errZFSHostMissing
	}
	lctx, cancel := context.WithTimeout(ctx, zfsListTimeout)
	defer cancel()
	tree, err := s.zfs.Tree(lctx, d.Dataset)
	if err != nil {
		return 0, err
	}
	snaps, err := s.zfs.Snapshots(lctx, d.Dataset)
	if err != nil {
		return 0, err
	}
	remaining := 0
	for _, snap := range zfs.LeakedStamps(tree, snaps) {
		err := backup.DestroyRecursiveWithRetry(ctx, s.zfs, d.Dataset, snap,
			zfsDestroyBudget, time.Now, sleepFor, s.IsShuttingDown())
		if err != nil {
			remaining++
			log.Printf("api: zfs: %s@%s is still there after the sweep: %v", d.Dataset, snap, err)
			continue
		}
		log.Printf("api: zfs: removed the leftover snapshot %s@%s", d.Dataset, snap)
	}
	if err := s.store.SetZFSLeftovers(d.ID, remaining, time.Now().Unix()); err != nil {
		log.Printf("api: zfs: recording the sweep of %s failed: %v", d.Dataset, err)
	}
	return remaining, nil
}

// sleepFor waits, or gives up as soon as the context does.
func sleepFor(ctx context.Context, d time.Duration) {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
	case <-t.C:
	}
}
