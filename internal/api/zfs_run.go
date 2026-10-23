package api

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"log"
	"slices"
	"strings"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/backup"
	"github.com/junkerderprovinz/bombvault/internal/notify"
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
		return nil, &backup.ZFSRefusal{Code: zfsErrCode(err), Detail: zfsDetail(err.Error())}
	}
	if ref := zfsTreeRefusal(d.Dataset, tree); ref != nil {
		return nil, ref
	}

	// Without a previous tree every dataset would be new, and a first run is no
	// news about the children.
	var seen map[string]bool
	if len(previous) > 0 {
		seen = make(map[string]bool, len(previous))
		for _, p := range previous {
			seen[p.Dataset] = true
		}
	}
	members := s.zfsResolveMembers(tree, d.Dataset, d.ExcludedChildren, seen)
	readable := 0
	for _, m := range members {
		if m.Code == "" {
			readable++
		}
	}
	if readable == 0 {
		return nil, &backup.ZFSRefusal{Code: "nothing-readable", Detail: d.Dataset}
	}

	s.recordZFSMembers(d, members)
	if _, err := s.store.SetZFSCheck(d.ID, "ok", "", tree[0].Mountpoint, time.Now().Unix()); err != nil {
		log.Printf("api: zfs: recording the check of %s failed: %v", d.Dataset, err)
	}
	return members, nil
}

// recordZFSMembers stores the tree as a preflight or a probe resolved it.
func (s *Service) recordZFSMembers(d store.ZFSDataset, members []zfsMember) {
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
}

// zfsTreeRefusal refuses a tree no item can be built on, before anything looks
// at the individual members.
func zfsTreeRefusal(root string, tree []zfs.ListEntry) *backup.ZFSRefusal {
	if tree[0].Type != "filesystem" {
		return &backup.ZFSRefusal{Code: "not-filesystem", Detail: root}
	}
	legacy := 0
	for _, e := range tree {
		if !zfs.SnapshotNameFits(e.Name) {
			// The recursive snapshot is one command over the whole tree, so a
			// single name that does not fit fails every member with it.
			return &backup.ZFSRefusal{Code: "name-too-long", Detail: e.Name}
		}
		if e.Type == "filesystem" && e.Mountpoint == "legacy" {
			legacy++
		}
	}
	if legacy > zfsMaxLegacyFilesystems {
		return &backup.ZFSRefusal{Code: "docker-storage", Detail: root}
	}
	return nil
}

// zfsResolveMembers turns a tree listing into the item's members, each with the
// code that says whether it can be read. seen holds the datasets a previous run
// knew, so a child picked up for the first time is marked as new.
func (s *Service) zfsResolveMembers(tree []zfs.ListEntry, root string, excluded []string, seen map[string]bool) []zfsMember {
	recs := zfsMountRecords()
	members := make([]zfsMember, 0, len(tree))
	for _, e := range tree {
		m := zfsMember{
			Entry:   e,
			RelPath: zfsRelPath(root, e.Name),
			Code:    zfs.MemberCode(e, excluded),
			IsNew:   seen != nil && !seen[e.Name],
		}
		if m.Code == "" {
			m.Mount, m.Code = s.resolveDatasetMount(recs, e, false)
		}
		members = append(members, m)
	}
	return members
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
		if zfsItemsOverlap(d, other) {
			return other.Dataset, true
		}
	}
	return "", false
}

// zfsRefusalSentence is the English run error a refusal leaves in the history.
// It ends in the reason code, from which the page builds its own sentence.
func zfsRefusalSentence(action, name string, ref *backup.ZFSRefusal) string {
	switch {
	case ref.Code == "":
		return "zfs " + action + ": " + name + ": " + ref.Detail
	case ref.Detail == "":
		return "zfs " + action + ": " + name + " [" + ref.Code + "]"
	default:
		return "zfs " + action + ": " + name + ": " + ref.Detail + " [" + ref.Code + "]"
	}
}

// recordZFSRefusal turns a refusal that happened before the orchestrator into a
// failed run, so a scheduled item that cannot be read shows up in Run History
// instead of only in the log.
func (s *Service) recordZFSRefusal(ctx context.Context, d store.ZFSDataset, ref *backup.ZFSRefusal) error {
	err := &backup.ZFSRefusal{Detail: zfsRefusalSentence("backup", d.Dataset, ref)}
	if _, sErr := s.store.SetZFSCheck(d.ID, ref.Code, ref.Detail, d.LastHostMountpoint, time.Now().Unix()); sErr != nil {
		log.Printf("api: zfs: recording the check of %s failed: %v", d.Dataset, sErr)
	}
	if runID, sErr := s.startRun(ctx, d.ID, "backup"); sErr != nil {
		log.Printf("api: zfs: recording the refused run of %s failed: %v", d.Dataset, sErr)
	} else if fErr := s.store.FinishRun(runID, "failed", "", 0, truncateRunErr(err)); fErr != nil {
		log.Printf("api: zfs: finishing the refused run of %s failed: %v", d.Dataset, fErr)
	}
	s.notifyBackup(ctx, zfsDomain, d.Dataset, "", false, backup.Summary{}, err)
	return err
}

// notifyZFSUnsuppressed sends a message that has to reach the user although the
// run it belongs to is part of a scheduled summary. notify.Send applies the
// policy itself. These messages are not a run's outcome, so they leave the
// Healthchecks check to the run.
func (s *Service) notifyZFSUnsuppressed(ev notify.Event) {
	c, err := s.NotifyConfig()
	if err != nil {
		return
	}
	notify.Send(notify.WithHealthchecksSuppressed(context.Background()), c, zfsDomain, ev)
}

// applyRetentionTags ages every member of a tree under its own identity tag and
// reclaims the space once at the end. One tag for the whole item would put all
// its datasets into a single keep-N group and age whole datasets out. A
// dataset whose data a finding says was lost keeps its old backups, the way a
// container's do, while its siblings prune.
func (s *Service) applyRetentionTags(ctx context.Context, settings store.Settings, pl sectionPlace, tags []string, domain string) {
	p, appendOnly := s.keepAt(settings, domain, pl)
	if !p.Any() || len(tags) == 0 {
		return
	}
	repo, mode := pl.repo, pl.mode
	if appendOnly {
		log.Printf("api: %s: retention skipped: %s is flagged append-only", domain, shortRepoName(repo))
		return
	}
	for _, tag := range tags {
		hold := anomalyScope{Kind: anomalyScopeZFSDS, ID: strings.TrimPrefix(tag, "zfs:")}
		held, why, hErr := s.anomalies.RetentionHeld(ctx, hold)
		if hErr != nil {
			log.Printf("api: %s: retention skipped for %s: the anomaly check failed: %v", domain, tag, hErr)
			s.notifyRetentionFailed(ctx, domain, tag, "the anomaly check could not run, so nothing was deleted: "+scrubError(hErr))
			continue
		}
		if held {
			log.Printf("api: %s: retention paused for %s: %s", domain, tag, why)
			continue
		}
		if err := s.forgetWithLockHeal(ctx, repo, p, mode, []string{tag}, false); err != nil {
			log.Printf("api: retention prune failed (backup is safe): %v", err)
			s.notifyRetentionFailed(ctx, domain, tag, truncateRunErr(err))
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
	// props are the tree's ZFS properties as read before the run, stored with
	// every member that was backed up.
	props map[string]zfs.Properties
}

func (r zfsRunRecorder) RecordRun(runID, snap string, window time.Duration, hookDetail string) error {
	seconds := int64(-1)
	if r.stops && window >= 0 {
		seconds = int64(window / time.Second)
	}
	return r.st.RecordZFSRun(runID, r.itemID, snap, seconds, hookDetail)
}

// AddMember records the member on the run and on the item's tree, which is what
// the coverage card and the row read. A dataset the run never got to keeps what
// the tree knew of it: a cancelled run says nothing about the dataset itself.
func (r zfsRunRecorder) AddMember(runID string, m backup.ZFSMemberResult) error {
	var backedUpAt int64
	if m.Outcome == outcomeBackedUp || m.Outcome == "empty" {
		backedUpAt = time.Now().Unix()
	}
	if m.Outcome != "not-reached" {
		if err := r.st.SetZFSMemberOutcome(r.itemID, m.Dataset, m.Outcome, backedUpAt); err != nil {
			log.Printf("api: zfs: recording the outcome of %s on its tree failed: %v", m.Dataset, err)
		}
	}
	row := store.ZFSRunMember{
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
		HasParent:       m.HasParent,
	}
	if m.Measured {
		row.SourceBytes, row.SourceFiles = &m.SourceBytes, &m.SourceFiles
	}
	if err := r.st.AddZFSRunMember(row); err != nil {
		return err
	}
	if p, ok := r.props[m.Dataset]; ok && m.ResticSnapshot != "" {
		if err := r.st.SetZFSRunMemberProperties(runID, m.Dataset, p); err != nil {
			log.Printf("api: zfs: recording the properties of %s failed: %v", m.Dataset, err)
		}
	}
	return nil
}

// zfsSelection is what a ZFS item is configured to back up. The datasets
// themselves come from the host on every run and are not part of it, so a
// child that disappears from the pool keeps the fingerprint and reads as the
// loss it is, while one the user excluded starts its series again.
func zfsSelection(d store.ZFSDataset) itemSelection {
	return itemSelection{Kind: zfsDomain, Root: d.Dataset, Children: d.ExcludedChildren, Excludes: d.Excludes}
}

// BackupZFSDataset runs an item's backup and, after it, its replica. The
// replica does not depend on the backup: it follows one that failed or found
// no place as well, and an item with no place of its own only sends it.
func (s *Service) BackupZFSDataset(ctx context.Context, id string) (backup.Summary, error) {
	d, err := s.store.GetZFSDataset(id)
	if err != nil {
		return backup.Summary{}, fmt.Errorf("zfs backup: load dataset: %w", err)
	}
	if s.zfsReplicaOnly(d) {
		return backup.Summary{}, s.replicateZFSItem(ctx, d)
	}
	sum, err := s.backupZFSToRepo(ctx, id)
	if IsBackupCancelled(err) || ctx.Err() != nil {
		return sum, err
	}
	if fresh, gErr := s.store.GetZFSDataset(id); gErr == nil {
		s.replicateAfterZFSBackup(ctx, fresh)
	}
	return sum, err
}

// backupZFSToRepo backs up one item: its whole tree from a single recursive
// snapshot, one restic run per readable member, then the snapshot is destroyed.
func (s *Service) backupZFSToRepo(ctx context.Context, id string) (_ backup.Summary, retErr error) {
	ctx, cancel := backupHoldCtx(ctx)
	defer cancel()
	defer s.lockDomain(zfsDomain)()
	defer s.sampleVolumesFor(ctx, zfsDomain)

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
	s.registerBackupCancel(ctx, key, cancel)
	defer s.endBackupCancel(key, &retErr)

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
	// A repository of the item's own is not the section's folder, so the
	// folder's switch leaves it alone.
	places := []sectionPlace{{repo: repo, mode: s.primaryModeFor(settings, zfsDomain, repo)}}
	if d.Repo == "" {
		if places, err = s.sectionPlaces(settings, zfsDomain, repo); err != nil {
			return backup.Summary{}, err
		}
	}
	if err := s.readyPlaces(ctx, places); err != nil {
		return backup.Summary{}, err
	}
	repo = places[0].repo
	s.notifyBackupStart(ctx, zfsDomain)
	rctx, startedAt := s.progBegin(ctx, key, "backup")

	plans, skipped, tags := zfsSplitMembers(members)
	props := s.zfsTreeProperties(ctx, d.Dataset)
	sum, runErr := backup.BackupZFSItem(rctx, backup.ZFSBackupDeps{
		Root:          d.Dataset,
		Members:       plans,
		Skipped:       skipped,
		Repo:          repo,
		TargetID:      d.ID,
		Excludes:      d.Excludes,
		SelectionFP:   selectionFingerprint(zfsSelection(d)),
		Now:           time.Now,
		ReserveName:   s.snapNames.reserve,
		Clock:         time.Now,
		ZFS:           zfsRunHost{h: s.zfs},
		Restic:        s.placesAdapter(settings, zfsDomain, places, ""),
		Visible:       s.visibleSnapshot,
		DirEmpty:      zfsDirEmpty,
		Consistency:   s.newZFSConsistency(d, settings, zfsLockWait(ctx)),
		Hooks:         s.newZFSHooks(d),
		Runs:          runsAdapter{st: s.store, ctx: ctx, svc: s, cancelKey: key},
		Recorder:      zfsRunRecorder{st: s.store, itemID: d.ID, stops: len(d.StopContainers) > 0, props: props},
		DestroyBudget: zfsDestroyBudget,
		ShuttingDown:  s.IsShuttingDown,
		Sleep:         sleepFor,
		OnDestroyFailed: func(root, snap string, destroyErr error) {
			s.recordZFSLeftover(d, root, snap, destroyErr)
		},
	})
	s.progEnd(key, "backup", runErr == nil, startedAt)
	s.notifyBackup(ctx, zfsDomain, d.Dataset, key, runErr == nil, sum, runErr)
	if changes := zfsMemberChanges(previous, members); len(changes) > 0 {
		s.notifyZFSUnsuppressed(notify.Event{
			Title:   "BombVault",
			Message: "The datasets below " + d.Dataset + " changed: " + strings.Join(changes, ", "),
			OK:      true,
		})
	}

	// Retention runs whether or not a member failed: the members that were read
	// wrote real snapshots, and their history ages under the same policy. A
	// cancelled or stalled run skips the aftercare, which would only fail on
	// its ended context.
	if ctx.Err() == nil {
		for _, pl := range places {
			s.applyRetentionTags(ctx, settings, pl, tags, zfsDomain)
		}
		if places[0].own() {
			makeRepoReadable(repo, s.cfg.DataDir)
			s.replicateOffsite(ctx, zfsDomain, settings, repo, "")
			s.checkPrimaryRemoteBudget(ctx, zfsDomain, repo, settings)
		}
		s.collectStatsAfterItem(ctx, zfsDomain)
	}
	if runErr != nil {
		return backup.Summary{}, runErr
	}
	s.queueFirstProbe(d.ID)
	return sum, nil
}

// zfsTreeProperties reads the tree's properties for the run to store. A
// failure costs only the properties, never the backup.
func (s *Service) zfsTreeProperties(ctx context.Context, root string) map[string]zfs.Properties {
	lctx, cancel := context.WithTimeout(ctx, zfsListTimeout)
	defer cancel()
	props, err := s.zfs.Properties(lctx, root)
	if err != nil {
		log.Printf("api: zfs: reading the properties of %s failed, the run stores none: %v", root, err)
		return nil
	}
	return props
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
// previous run, and the ones whose readability changed. The first run of an
// item has nothing to compare with.
func zfsMemberChanges(previous []store.ZFSMember, members []zfsMember) []string {
	if len(previous) == 0 {
		return nil
	}
	was := make(map[string]string, len(previous))
	for _, p := range previous {
		was[p.Dataset] = zfsSkipCode(p.Outcome)
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

// zfsSkipCode is the preflight's verdict inside a stored member outcome: what
// a run did to a member the preflight could read is no skip.
func zfsSkipCode(outcome string) string {
	switch outcome {
	case outcomeBackedUp, "empty", "backup-failed", "snapshot-not-visible", "snapshot-loop", "not-reached", "stalled":
		return ""
	}
	return outcome
}

// recordZFSLeftover counts a snapshot the run could not destroy and says so
// once, on a context the scheduled summary cannot swallow.
func (s *Service) recordZFSLeftover(d store.ZFSDataset, root, snap string, err error) {
	log.Printf("api: zfs: %s@%s could not be removed, the next sweep retries it: %v", root, snap, err)
	if sErr := s.store.AddZFSLeftover(d.ID, time.Now().Unix()); sErr != nil {
		log.Printf("api: zfs: recording the leftover snapshot of %s failed: %v", root, sErr)
	}
	s.notifyZFSUnsuppressed(notify.Event{
		Title:   "BombVault",
		Message: "The snapshot " + root + "@" + snap + " could not be removed. BombVault will retry before the next backup.",
		OK:      false,
	})
}

// StartBackupZFSDataset runs one item in the background and returns at once.
// An item that only replicates starts its replica instead, which holds no
// backup lock however long the send takes and answers a refusal here.
func (s *Service) StartBackupZFSDataset(ctx context.Context, id string) (bool, error) {
	if d, err := s.store.GetZFSDataset(id); err == nil && s.zfsReplicaOnly(d) {
		if _, err := s.StartZFSReplica(ctx, id); err != nil {
			return false, err
		}
		return true, nil
	}
	if err := s.refuseNoPlace(zfsDomain, id); err != nil {
		return false, err
	}
	if !s.batchActive.CompareAndSwap(false, true) {
		return false, s.batchRefusal()
	}
	if op, busy := s.domainBusy(zfsDomain); busy {
		s.batchActive.Store(false)
		return false, domainBusyError{op: op, domain: "zfs"}
	}
	bctx := context.WithoutCancel(ctx)
	go func() {
		defer s.recoverOperation("backup zfs dataset: "+id, nil, func(msg string) {
			s.failStuckRun(id, msg)
		})
		defer s.batchActive.Store(false)
		if _, err := s.BackupZFSDataset(bctx, id); err != nil {
			log.Printf("api: backup zfs dataset: %q %s: %v", id, backupEnding(err), err)
		}
	}()
	return true, nil
}

// StartBackupZFSAll runs the given items one after another in one background
// batch, replicating and pruning once when the loop is done.
func (s *Service) StartBackupZFSAll(ctx context.Context, ids []string) (bool, error) {
	queue, replicas := s.splitReplicaOnly(ids)
	if len(queue) == 0 {
		// With nothing else to run, a refusal of every replica is the answer.
		var refusal error
		replicas = slices.DeleteFunc(replicas, func(d store.ZFSDataset) bool {
			err := s.zfsReplicaRunRefusal(d)
			refusal = cmp.Or(refusal, err)
			return err != nil
		})
		if len(replicas) == 0 && refusal != nil {
			return false, refusal
		}
		s.replicateInTurn(ctx, replicas)
		return len(replicas) > 0, nil
	}
	if !s.batchActive.CompareAndSwap(false, true) {
		return false, s.batchRefusal()
	}
	if op, busy := s.domainBusy(zfsDomain); busy {
		s.batchActive.Store(false)
		return false, domainBusyError{op: op, domain: "zfs"}
	}
	s.replicateInTurn(ctx, replicas)
	bctx := WithBulkReplicateSuppressed(context.WithoutCancel(ctx))
	go func() {
		defer s.recoverOperation("backup-zfs-all", nil, nil)
		defer s.batchActive.Store(false)

		total := len(queue)
		const key = "batch:zfs"
		s.publishBatch(key, 0, true)
		ok, fail, skipped := 0, 0, 0
		for i, id := range queue {
			if err := s.backupZFSOneForBatch(bctx, id); errors.Is(err, ErrNoPlace) {
				skipped++
				log.Printf("api: backup-zfs-all: %q skipped: %v", id, err)
			} else if err != nil {
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
		log.Printf("api: backup-zfs-all done: %d ok, %d skipped, %d failed (of %d requested %d)", ok, skipped, fail, total, len(ids))
	}()
	return true, nil
}

// splitReplicaOnly sorts the items a bulk backup names into those it backs
// up and those that only replicate.
func (s *Service) splitReplicaOnly(ids []string) (backups []string, replicas []store.ZFSDataset) {
	for _, id := range ids {
		if id == "" {
			continue
		}
		if d, err := s.store.GetZFSDataset(id); err == nil && s.zfsReplicaOnly(d) {
			replicas = append(replicas, d)
			continue
		}
		backups = append(backups, id)
	}
	return backups, replicas
}

// replicateInTurn sends the replicas of items that only replicate one after
// the other in the background. A send can take hours, so it holds none of
// the backup locks, and each item takes its own replica lock.
func (s *Service) replicateInTurn(ctx context.Context, items []store.ZFSDataset) {
	if len(items) == 0 {
		return
	}
	bctx := context.WithoutCancel(ctx)
	s.replica.work.Add(1)
	go func() {
		defer s.replica.work.Done()
		defer s.recoverOperation("zfs replicas of a bulk backup", nil, nil)
		for _, d := range items {
			if refused, err := s.replicateZFSNow(bctx, d); refused {
				log.Printf("api: backup-zfs-all: the replica of %s was refused: %v", d.Dataset, err)
			} else if err != nil {
				log.Printf("api: backup-zfs-all: the replica of %s failed (continuing): %v", d.Dataset, err)
			}
		}
	}()
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
// switched off and the item disabled, and the datasets container backups
// snapshotted: a leaked snapshot pins deleted blocks either way. The caller
// holds the domain lock.
func (s *Service) SweepZFSLeftoversOnStartup(ctx context.Context) {
	defer s.sweepContainerSnapshots(ctx)
	rows, err := s.store.ListZFSDatasets()
	if err != nil {
		log.Printf("api: zfs: the startup sweep could not list the items: %v", err)
		return
	}
	if len(rows) == 0 {
		log.Print("api: zfs: the startup sweep found no dataset items, nothing to do")
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
	remaining, err := s.sweepStamps(ctx, d.Dataset)
	if err != nil {
		return 0, err
	}
	if err := s.store.SetZFSLeftovers(d.ID, remaining, time.Now().Unix()); err != nil {
		log.Printf("api: zfs: recording the sweep of %s failed: %v", d.Dataset, err)
	}
	return remaining, nil
}

// sweepStamps destroys the BombVault snapshots left on a tree and returns how
// many survived. A snapshot a run is still reading is no leftover: a dataset
// backup and a container backup can share a tree.
func (s *Service) sweepStamps(ctx context.Context, root string) (int, error) {
	if s.zfs == nil {
		return 0, errZFSHostMissing
	}
	lctx, cancel := context.WithTimeout(ctx, zfsListTimeout)
	defer cancel()
	tree, err := s.zfs.Tree(lctx, root)
	if err != nil {
		return 0, err
	}
	snaps, err := s.zfs.Snapshots(lctx, root)
	if err != nil {
		return 0, err
	}
	remaining := 0
	for _, snap := range zfs.LeakedStamps(tree, snaps) {
		if s.snapNames.busy(snap) {
			continue
		}
		err := backup.DestroyRecursiveWithRetry(ctx, s.zfs, root, snap,
			zfsDestroyBudget, time.Now, sleepFor, s.IsShuttingDown())
		if err != nil {
			remaining++
			log.Printf("api: zfs: %s@%s is still there after the sweep: %v", root, snap, err)
			continue
		}
		log.Printf("api: zfs: removed the leftover snapshot %s@%s", root, snap)
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
