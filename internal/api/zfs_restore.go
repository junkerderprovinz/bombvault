package api

import (
	"context"
	"errors"
	"fmt"
	"log"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/backup"
	"github.com/junkerderprovinz/bombvault/internal/paths"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
	"github.com/junkerderprovinz/bombvault/internal/zfs"
)

const (
	// zfsRestorePointRuns is how many of an item's runs are read to learn what
	// each restore point did to each dataset. An older point is still offered,
	// with the member snapshots restic knows about.
	zfsRestorePointRuns = 200
	// outcomeBackedUp is what a member counts as when the run that wrote its
	// snapshot is no longer in the database.
	outcomeBackedUp = "backed-up"
)

// ZFSRestorePoint is one run instant of an item: the member snapshots that
// share one host snapshot stamp.
type ZFSRestorePoint struct {
	Stamp   string                `json:"stamp"`
	Time    int64                 `json:"time"`
	Members []ZFSRestorePointItem `json:"members"`
}

// ZFSRestorePointItem is one dataset of the tree as that run instant left it. A
// member with no snapshot id was skipped, empty or excluded at the time.
type ZFSRestorePointItem struct {
	Dataset    string `json:"dataset"`
	RelPath    string `json:"relPath"`
	SnapshotID string `json:"snapshotId"`
	Outcome    string `json:"outcome"`
}

// ZFSRestoreRequest is one restore the panel asks for.
type ZFSRestoreRequest struct {
	Stamp     string `json:"stamp"`
	Dataset   string `json:"dataset"`
	WholeTree bool   `json:"wholeTree"`
	// Paths are absolute inside the member's own tree; empty restores all of it.
	Paths []string `json:"paths"`
	// TargetPath is a folder below the host mount; empty restores in place.
	TargetPath       string `json:"targetPath"`
	Confirm          bool   `json:"confirm"`
	SafetySnapshot   bool   `json:"safetySnapshot"`
	SafetyOffConfirm bool   `json:"safetyOffConfirm"`
	StopContainers   bool   `json:"stopContainers"`
}

// ZFSRestoreAck is what the caller learns the moment a restore starts.
type ZFSRestoreAck struct {
	Target         string `json:"target"`
	SafetySnapshot string `json:"safetySnapshot"`
}

// zfsRestoreStep is one restic call of a restore: a member snapshot and where
// its contents go.
type zfsRestoreStep struct {
	snapshotID string
	target     string
}

// zfsRestorePlan is everything a validated restore needs, so the restic work
// runs detached from the request that asked for it.
type zfsRestorePlan struct {
	itemID  string
	root    string
	dataset string // the member an in-place restore writes into
	repo    string
	mode    restic.Mode
	steps   []zfsRestoreStep
	paths   []string
	inPlace bool
	// covered are the places inside an in-place target where a child dataset
	// is mounted, which the restore leaves alone.
	covered []string
	// consistency holds the item's containers down for the whole restore, and
	// is nil when nothing is to be stopped.
	consistency backup.ZFSConsistency
	// snapshotID is what the run record points at, the first member restored.
	snapshotID string
}

// ListZFSRestorePoints groups an item's member snapshots into the run instants
// they were taken in, newest first. Members the run did not write are listed
// with their outcome, so a restore point never hides what it left out.
func (s *Service) ListZFSRestorePoints(ctx context.Context, id, source string) ([]ZFSRestorePoint, error) {
	d, err := s.store.GetZFSDataset(id)
	if err != nil {
		return nil, fmt.Errorf("zfs restore: load dataset: %w", err)
	}
	snaps, err := s.zfsItemSnapshots(ctx, d, source)
	if err != nil {
		return nil, err
	}
	outcomes := s.zfsRunOutcomes(d.ID)
	points := map[string]map[string]ZFSRestorePointItem{}
	for _, snap := range snaps {
		dataset, ok := zfsSnapshotDataset(snap, d.Dataset)
		if !ok || len(snap.Paths) == 0 {
			continue
		}
		stamp, ok := zfs.StampFromPath(snap.Paths[0])
		if !ok {
			continue
		}
		if points[stamp] == nil {
			points[stamp] = map[string]ZFSRestorePointItem{}
		}
		outcome := outcomeBackedUp
		if recorded, ok := outcomes[stamp][dataset]; ok {
			outcome = recorded
		}
		points[stamp][dataset] = ZFSRestorePointItem{
			Dataset:    dataset,
			RelPath:    zfsRelPath(d.Dataset, dataset),
			SnapshotID: snap.ID,
			Outcome:    outcome,
		}
	}
	out := make([]ZFSRestorePoint, 0, len(points))
	for stamp, members := range points {
		for dataset, outcome := range outcomes[stamp] {
			if _, ok := members[dataset]; ok {
				continue
			}
			members[dataset] = ZFSRestorePointItem{
				Dataset: dataset,
				RelPath: zfsRelPath(d.Dataset, dataset),
				Outcome: outcome,
			}
		}
		out = append(out, ZFSRestorePoint{Stamp: stamp, Time: zfsStampUnix(stamp), Members: zfsSortMembers(members)})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Stamp > out[j].Stamp })
	return out, nil
}

// zfsSortMembers orders the members by name, which puts every dataset before
// its own children.
func zfsSortMembers(members map[string]ZFSRestorePointItem) []ZFSRestorePointItem {
	out := make([]ZFSRestorePointItem, 0, len(members))
	for _, m := range members {
		out = append(out, m)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Dataset < out[j].Dataset })
	return out
}

// zfsStampUnix is the instant a snapshot stamp names, or 0 for a stamp that
// carries no readable time.
func zfsStampUnix(stamp string) int64 {
	at, ok := zfs.StampTime(stamp)
	if !ok {
		return 0
	}
	return at.Unix()
}

// zfsRunOutcomes maps each recorded run instant of an item to what that run did
// to each dataset of the tree.
func (s *Service) zfsRunOutcomes(itemID string) map[string]map[string]string {
	runs, err := s.store.ListZFSRuns(itemID, zfsRestorePointRuns)
	if err != nil {
		log.Printf("api: zfs: reading the runs of %s failed: %v", itemID, err)
		return nil
	}
	out := make(map[string]map[string]string, len(runs))
	for _, run := range runs {
		members, mErr := s.store.ListZFSRunMembers(run.RunID)
		if mErr != nil {
			log.Printf("api: zfs: reading the members of run %s failed: %v", run.RunID, mErr)
			continue
		}
		byDataset := make(map[string]string, len(members))
		for _, m := range members {
			byDataset[m.Dataset] = m.Outcome
		}
		out[run.SnapshotName] = byDataset
	}
	return out
}

// zfsSnapshotDataset names the dataset a snapshot holds, when it is one of the
// tree under root. A name that would not pass validation was not written by
// this domain, so it is neither restored nor counted as a member.
func zfsSnapshotDataset(snap restic.Snapshot, root string) (string, bool) {
	for _, tag := range snap.Tags {
		dataset, ok := strings.CutPrefix(tag, "zfs:")
		if !ok || (dataset != root && !zfs.DescendantOf(dataset, root)) {
			continue
		}
		if zfs.ValidateMemberName(dataset) != nil {
			continue
		}
		return dataset, true
	}
	return "", false
}

// ListSnapshotFilesZFS lists the files of one member snapshot, for a restore of
// single files. The snapshot must be one of this item's, so one item's tree
// cannot be read through another's id.
func (s *Service) ListSnapshotFilesZFS(ctx context.Context, id, snapshotID, source string) ([]restic.FileEntry, error) {
	if !backup.ValidSnapshotID(snapshotID) {
		return nil, backup.ErrInvalidSnapshotID
	}
	d, err := s.store.GetZFSDataset(id)
	if err != nil {
		return nil, fmt.Errorf("zfs restore: load dataset: %w", err)
	}
	snaps, err := s.zfsItemSnapshots(ctx, d, source)
	if err != nil {
		return nil, err
	}
	if !snapshotBelongs(snaps, snapshotID) {
		return nil, fmt.Errorf("snapshot %s does not belong to this item", snapshotID)
	}
	settings, err := s.store.GetSettings()
	if err != nil {
		return nil, fmt.Errorf("read settings: %w", err)
	}
	repo, err := s.zfsDatasetRepoFor(settings, d, source)
	if err != nil {
		return nil, err
	}
	return s.lsSelfHeal(ctx, repo, snapshotID, s.repoModeFor(settings, zfsDomain, source, repo))
}

// StartRestoreZFS validates a restore, takes the safety snapshot its answer
// names and then runs the restic work detached. Everything that can refuse does
// so synchronously, so a bad request never leaves a half-started run behind.
//
// It shares batchActive with the backups and the other restores, and reports
// that nothing started when one of those is already running.
func (s *Service) StartRestoreZFS(ctx context.Context, id, source string, req ZFSRestoreRequest) (ZFSRestoreAck, bool, error) {
	if !s.batchActive.CompareAndSwap(false, true) {
		return ZFSRestoreAck{}, false, nil
	}
	plan, ack, err := s.prepareRestoreZFS(ctx, id, source, req)
	if err != nil {
		s.batchActive.Store(false)
		return ZFSRestoreAck{}, false, err
	}
	bctx := context.WithoutCancel(ctx)
	rkey := zfsDomain + ":" + plan.root
	go func() {
		var runID string
		defer s.recoverOperation("restore zfs dataset: "+plan.root, nil, func(msg string) {
			s.finishRestoreRun(runID, "", errors.New(msg))
		})
		defer s.batchActive.Store(false)
		tctx, tcancel := context.WithTimeout(bctx, restoreTimeout)
		defer tcancel()
		rctx, cancel := context.WithCancel(tctx)
		defer cancel()
		s.registerCancel(rkey, cancel)
		defer s.unregisterCancel(rkey)
		runID = s.beginRestoreRunForTarget(plan.itemID)
		pctx, startedAt := s.progBegin(rctx, rkey, "restore")
		rerr := s.runRestoreZFS(pctx, plan)
		if cErr := s.concludeFileSetRestore(runID, rkey, plan.snapshotID, rerr, startedAt); cErr != nil {
			log.Printf("api: restore zfs: %q failed: %v", plan.root, cErr) //nolint:gosec // G706: the root is %q-quoted
		}
	}()
	return ack, true, nil
}

// prepareRestoreZFS resolves and checks everything a restore needs, and takes
// the safety snapshot last, once nothing else can refuse.
func (s *Service) prepareRestoreZFS(ctx context.Context, id, source string, req ZFSRestoreRequest) (zfsRestorePlan, ZFSRestoreAck, error) {
	d, err := s.store.GetZFSDataset(id)
	if err != nil {
		return zfsRestorePlan{}, ZFSRestoreAck{}, fmt.Errorf("zfs restore: load dataset: %w", err)
	}
	if source != "local" && !isOffsiteSource(source) {
		return zfsRestorePlan{}, ZFSRestoreAck{}, errors.New("invalid source (must be local or offsite)")
	}
	if !zfs.IsBombVaultSnapshot(req.Stamp) {
		return zfsRestorePlan{}, ZFSRestoreAck{}, errors.New("the restore point is not a BombVault snapshot name")
	}
	points, err := s.ListZFSRestorePoints(ctx, id, source)
	if err != nil {
		return zfsRestorePlan{}, ZFSRestoreAck{}, err
	}
	point, ok := zfsFindPoint(points, req.Stamp)
	if !ok {
		return zfsRestorePlan{}, ZFSRestoreAck{}, fmt.Errorf("restore point %s does not belong to this item", req.Stamp)
	}
	settings, err := s.store.GetSettings()
	if err != nil {
		return zfsRestorePlan{}, ZFSRestoreAck{}, fmt.Errorf("read settings: %w", err)
	}
	repo, err := s.zfsDatasetRepoFor(settings, d, source)
	if err != nil {
		return zfsRestorePlan{}, ZFSRestoreAck{}, err
	}
	selected, err := zfsCleanRestorePaths(req.Paths)
	if err != nil {
		return zfsRestorePlan{}, ZFSRestoreAck{}, err
	}
	plan := zfsRestorePlan{
		itemID: d.ID,
		root:   d.Dataset,
		repo:   repo,
		mode:   s.repoModeFor(settings, zfsDomain, source, repo),
		paths:  selected,
	}
	if sub := strings.TrimSpace(req.TargetPath); sub != "" {
		ack, fErr := s.planZFSRestoreToFolder(ctx, &plan, point, req, sub)
		return plan, ack, fErr
	}
	ack, pErr := s.planZFSRestoreInPlace(ctx, &plan, d, settings, point, req)
	return plan, ack, pErr
}

// planZFSRestoreToFolder fills in a restore that writes beside the live data:
// one member into the folder itself, or the whole tree with each member in its
// own place below it.
func (s *Service) planZFSRestoreToFolder(ctx context.Context, plan *zfsRestorePlan, point ZFSRestorePoint, req ZFSRestoreRequest, sub string) (ZFSRestoreAck, error) {
	target, err := paths.Resolve(s.cfg.HostMountRoot, sub)
	if err != nil {
		return ZFSRestoreAck{}, errors.New("invalid target folder: must be a relative subpath under the host mount")
	}
	members, err := zfsRestoreMembers(point, req)
	if err != nil {
		return ZFSRestoreAck{}, err
	}
	for _, m := range members {
		dest := target
		if req.WholeTree {
			dest = zfsFolderTarget(target, m.RelPath)
		}
		plan.steps = append(plan.steps, zfsRestoreStep{snapshotID: m.SnapshotID, target: dest})
	}
	plan.dataset = req.Dataset
	plan.snapshotID = plan.steps[0].snapshotID
	if err := s.guardZFSRestoreFolder(ctx, *plan, target); err != nil {
		return ZFSRestoreAck{}, err
	}
	if err := paths.EnsureDirReadable(target); err != nil {
		return ZFSRestoreAck{}, fmt.Errorf("create target folder: %w", err)
	}
	return ZFSRestoreAck{Target: target}, nil
}

// planZFSRestoreInPlace fills in a restore that writes into the live dataset,
// which is why it is confirmed, checked against the live mount and preceded by
// the safety snapshot.
func (s *Service) planZFSRestoreInPlace(ctx context.Context, plan *zfsRestorePlan, d store.ZFSDataset, settings store.Settings, point ZFSRestorePoint, req ZFSRestoreRequest) (ZFSRestoreAck, error) {
	if req.WholeTree {
		return ZFSRestoreAck{}, errors.New("a whole tree can only be restored into a folder")
	}
	if !req.Confirm {
		return ZFSRestoreAck{}, backup.ErrNotConfirmed
	}
	if !req.SafetySnapshot && !req.SafetyOffConfirm {
		return ZFSRestoreAck{}, errors.New("restoring without a safety snapshot needs a second confirmation")
	}
	member, ok := zfsPointMember(point, req.Dataset)
	if !ok || member.SnapshotID == "" {
		return ZFSRestoreAck{}, fmt.Errorf("dataset %q was not backed up at this restore point", req.Dataset)
	}
	cpath, covered, code := s.zfsRestoreMount(ctx, d.Dataset, req.Dataset)
	if code != "" {
		return ZFSRestoreAck{}, zfsRefuse(code, req.Dataset)
	}
	if err := zfsRefuseCoveredPaths(plan.paths, covered); err != nil {
		return ZFSRestoreAck{}, err
	}
	plan.inPlace = true
	plan.covered = covered
	plan.dataset = req.Dataset
	plan.steps = []zfsRestoreStep{{snapshotID: member.SnapshotID, target: cpath}}
	plan.snapshotID = member.SnapshotID
	if req.StopContainers {
		plan.consistency = s.newZFSConsistency(d, settings, zfsLockWait(ctx))
	}
	ack := ZFSRestoreAck{Target: cpath}
	if req.SafetySnapshot {
		name, err := s.takeZFSSafetySnapshot(ctx, d, req.Dataset)
		if err != nil {
			return ZFSRestoreAck{}, err
		}
		ack.SafetySnapshot = req.Dataset + "@" + name
	}
	return ack, nil
}

// zfsRestoreMembers picks what a to-folder restore writes: every member the run
// instant holds, or the single dataset that was asked for.
func zfsRestoreMembers(point ZFSRestorePoint, req ZFSRestoreRequest) ([]ZFSRestorePointItem, error) {
	if !req.WholeTree {
		member, ok := zfsPointMember(point, req.Dataset)
		if !ok || member.SnapshotID == "" {
			return nil, fmt.Errorf("dataset %q was not backed up at this restore point", req.Dataset)
		}
		return []ZFSRestorePointItem{member}, nil
	}
	if len(req.Paths) > 0 {
		return nil, errors.New("a whole tree restores every dataset, not selected files")
	}
	var out []ZFSRestorePointItem
	for _, m := range point.Members {
		if m.SnapshotID != "" {
			out = append(out, m)
		}
	}
	if len(out) == 0 {
		return nil, errors.New("this restore point holds no dataset that was backed up")
	}
	return out, nil
}

func zfsFindPoint(points []ZFSRestorePoint, stamp string) (ZFSRestorePoint, bool) {
	for _, p := range points {
		if p.Stamp == stamp {
			return p, true
		}
	}
	return ZFSRestorePoint{}, false
}

func zfsPointMember(point ZFSRestorePoint, dataset string) (ZFSRestorePointItem, bool) {
	for _, m := range point.Members {
		if m.Dataset == dataset {
			return m, true
		}
	}
	return ZFSRestorePointItem{}, false
}

// zfsFolderTarget is where one member lands below the chosen folder: the root
// directly in it, a child in a subfolder named after its place in the tree.
func zfsFolderTarget(folder, relPath string) string {
	return path.Clean(folder + relPath)
}

// zfsCleanRestorePaths cleans the selection once, so the path that is checked
// is the path restic is given. A member snapshot's tree root is the dataset
// root, so every selected path is absolute inside that tree and a relative one
// is an attempt to reach out of it.
func zfsCleanRestorePaths(in []string) ([]string, error) {
	out := make([]string, 0, len(in))
	for _, p := range in {
		cleaned := path.Clean(p)
		if !strings.HasPrefix(cleaned, "/") {
			return nil, fmt.Errorf("selected path %q is outside the dataset", p)
		}
		out = append(out, cleaned)
	}
	if len(out) == 0 {
		return nil, nil
	}
	return out, nil
}

// zfsRestoreMount answers where the container may write a dataset's live data
// and where child datasets are mounted inside it, or the reason code why it
// may not. Writing into the mountpoint directory of an unmounted dataset lands
// in its parent, so nothing short of a writable mount of the dataset itself
// counts.
func (s *Service) zfsRestoreMount(ctx context.Context, root, dataset string) (string, []string, string) {
	if s.zfs == nil {
		return "", nil, "ssh-missing"
	}
	lctx, cancel := context.WithTimeout(ctx, zfsListTimeout)
	defer cancel()
	tree, err := s.zfs.Tree(lctx, root)
	if err != nil {
		return "", nil, zfsErrCode(err)
	}
	entry, ok := zfsTreeEntry(tree, dataset)
	if !ok {
		return "", nil, "not-found"
	}
	if code := zfs.MemberCode(entry, nil); code != "" {
		return "", nil, code
	}
	rec, code := s.resolveDatasetMount(zfsMountRecords(), entry, true)
	if code != "" {
		return "", nil, code
	}
	if !zfs.Writable(rec) {
		return "", nil, "read-only-mount"
	}
	return rec.MountPoint, zfsMountedBelow(tree, entry), ""
}

// zfsMountedBelow names the places inside a dataset where its descendants are
// mounted, relative to its mountpoint. The dataset's own snapshot holds there
// the directory the child's mount hides: restored in place, its files would
// land in the child and its mode and owner would replace the child's.
func zfsMountedBelow(tree []zfs.ListEntry, entry zfs.ListEntry) []string {
	var out []string
	for _, e := range tree {
		if e.Type != "filesystem" || !e.Mounted || !zfs.DescendantOf(e.Name, entry.Name) {
			continue
		}
		if rel, ok := strings.CutPrefix(e.Mountpoint, entry.Mountpoint+"/"); ok {
			out = append(out, "/"+rel)
		}
	}
	return out
}

// zfsRefuseCoveredPaths refuses a selected path at or below a mounted child,
// whose files belong to the child's own snapshot.
func zfsRefuseCoveredPaths(selected, covered []string) error {
	for _, p := range selected {
		for _, c := range covered {
			if p == c || strings.HasPrefix(p, c+"/") {
				return fmt.Errorf("%q lies on the mounted child dataset at %q; restore it from that dataset", p, c)
			}
		}
	}
	return nil
}

func zfsTreeEntry(tree []zfs.ListEntry, dataset string) (zfs.ListEntry, bool) {
	for _, e := range tree {
		if e.Name == dataset {
			return e, true
		}
	}
	return zfs.ListEntry{}, false
}

// takeZFSSafetySnapshot puts the dataset's current state out of reach of the
// restore about to overwrite it. It runs before the answer so its name can
// travel with it, and a failure means nothing is written at all.
func (s *Service) takeZFSSafetySnapshot(ctx context.Context, d store.ZFSDataset, dataset string) (string, error) {
	if !zfs.PreRestoreNameFits(dataset) {
		return "", zfsRefuse("safety-name-too-long", dataset)
	}
	name := zfs.PreRestoreSnapshotName(time.Now())
	sctx, cancel := context.WithTimeout(ctx, zfsSnapshotTimeout)
	defer cancel()
	if err := s.zfs.SnapshotSafety(sctx, dataset, name); err != nil {
		return "", zfsRefuse("safety-snapshot-failed", zfsDetail(err.Error()))
	}
	if err := s.store.UpsertZFSSafetySnapshot(store.ZFSSafetySnapshot{
		ItemID: d.ID, Dataset: dataset, Name: name, CreatedAt: time.Now().Unix(),
	}); err != nil {
		log.Printf("api: zfs: recording the safety snapshot %s@%s failed: %v", dataset, name, err)
	}
	return name, nil
}

// guardZFSRestoreFolder refuses a destination the host cannot take. A folder
// that is not on a mounted pool lives on the container's RAM rootfs, where a
// large restore brings the machine down; a shortfall that was actually measured
// stops the restore before it writes half of it.
func (s *Service) guardZFSRestoreFolder(ctx context.Context, plan zfsRestorePlan, target string) error {
	if !s.destinationMounted(target) {
		return zfsRefuse("destination-not-mounted", s.toHostPath(target))
	}
	if want, measured := s.zfsRestoreSize(ctx, plan); measured && want > 0 {
		if free, fErr := s.diskFreeFn()(nearestExistingDir(target)); fErr == nil && free < uint64(want) {
			return zfsRefuse("not-enough-space", fmt.Sprintf("%s needs %d bytes and has %d free", s.toHostPath(target), want, free))
		}
	}
	return nil
}

// zfsRestoreSize sums what the chosen snapshots would write and says whether
// the number can be trusted: a size the repository will not report is no proof
// of a shortfall and must not block a restore.
func (s *Service) zfsRestoreSize(ctx context.Context, plan zfsRestorePlan) (int64, bool) {
	var want int64
	for _, step := range plan.steps {
		_, bytes, err := s.engine.StatsRestoreSize(ctx, plan.repo, step.snapshotID, plan.mode)
		if err != nil {
			return 0, false
		}
		want += bytes
	}
	return want, true
}

// runRestoreZFS is the restic work of an already validated restore. It holds
// the domain lock, which is what a scheduled backup respects, and checks the
// live mount once more under it: a backup may have run since the answer went
// out and left the dataset unmounted.
func (s *Service) runRestoreZFS(ctx context.Context, plan zfsRestorePlan) error {
	defer s.lockDomainFor(zfsDomain, "restore")()
	if plan.inPlace {
		_, covered, code := s.zfsRestoreMount(ctx, plan.root, plan.dataset)
		if code != "" {
			return &backup.ZFSRefusal{Detail: zfsRefusalSentence("restore", plan.dataset, &backup.ZFSRefusal{Code: code})}
		}
		if err := zfsRefuseCoveredPaths(plan.paths, covered); err != nil {
			return err
		}
		plan.covered = covered
	}
	if plan.consistency != nil {
		thaw, _, err := plan.consistency.Freeze(ctx)
		if err != nil {
			return err
		}
		defer thaw(context.WithoutCancel(ctx))
	}
	for _, step := range plan.steps {
		if err := s.restoreZFSStep(ctx, plan, step); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) restoreZFSStep(ctx context.Context, plan zfsRestorePlan, step zfsRestoreStep) error {
	if len(plan.paths) == 0 {
		excludes := make([]string, len(plan.covered))
		for i, c := range plan.covered {
			excludes[i] = escapeGlobLiteral(c)
		}
		return s.engine.RestoreAll(ctx, plan.repo, step.snapshotID, step.target, plan.mode, excludes...)
	}
	for i, sel := range plan.paths {
		if err := s.restoreZFSFile(ctx, plan, step, sel); err != nil {
			if len(plan.paths) > 1 {
				return fmt.Errorf("restored %d of %d files, then failed on %q: %w", i, len(plan.paths), sel, err)
			}
			return err
		}
	}
	return nil
}

// restoreZFSFile puts one selected path back. In place it goes to its own place
// inside the dataset; into a folder it lands directly as <target>/<name>, with
// no tree in between, the way a folder set's file restore does it. A member
// snapshot's root is the dataset root, so a path at the top of the dataset
// already lands under its own name with a plain include; restic would refuse
// a file as the root of a subtree restore.
func (s *Service) restoreZFSFile(ctx context.Context, plan zfsRestorePlan, step zfsRestoreStep, sel string) error {
	parent, base := path.Dir(sel), path.Base(sel)
	if plan.inPlace || parent == "/" {
		return s.engine.RestoreInclude(ctx, plan.repo, step.snapshotID, escapeGlobLiteral(sel), step.target, plan.mode)
	}
	// The subtree root travels as a selector and stays raw; the include is a
	// glob and is escaped.
	return s.engine.RestoreSubtreeInclude(ctx, plan.repo, step.snapshotID, parent, escapeGlobLiteral("/"+base), step.target, plan.mode)
}

// ListZFSSafetySnapshots reads the item's tree on the host and returns the
// snapshots an in-place restore left behind, reconciling the store with what is
// really on the pool: one destroyed outside BombVault stops being offered.
func (s *Service) ListZFSSafetySnapshots(ctx context.Context, id string) ([]store.ZFSSafetySnapshot, error) {
	d, err := s.store.GetZFSDataset(id)
	if err != nil {
		return nil, fmt.Errorf("zfs safety snapshots: load dataset: %w", err)
	}
	if s.zfs == nil {
		return nil, errZFSHostMissing
	}
	lctx, cancel := context.WithTimeout(ctx, zfsListTimeout)
	defer cancel()
	snaps, err := s.zfs.Snapshots(lctx, d.Dataset)
	if err != nil {
		return nil, err
	}
	out := make([]store.ZFSSafetySnapshot, 0, len(snaps))
	for _, snap := range snaps {
		if !zfs.IsPreRestoreSnapshot(snap.Name) {
			continue
		}
		out = append(out, store.ZFSSafetySnapshot{
			ItemID: d.ID, Dataset: snap.Dataset, Name: snap.Name,
			CreatedAt: snap.Creation, UsedBytes: snap.Used,
		})
	}
	if err := s.store.ReplaceZFSSafetySnapshots(d.ID, out); err != nil {
		log.Printf("api: zfs: recording the safety snapshots of %s failed: %v", d.Dataset, err)
	}
	return out, nil
}

// DeleteZFSSafetySnapshot destroys one snapshot an in-place restore kept. Only
// a pre-restore name of this item's own tree is accepted, and the destroy is
// never recursive, so nothing else can be reached through this route.
func (s *Service) DeleteZFSSafetySnapshot(ctx context.Context, id, dataset, name string) error {
	d, err := s.store.GetZFSDataset(id)
	if err != nil {
		return fmt.Errorf("zfs safety snapshots: load dataset: %w", err)
	}
	if dataset != d.Dataset && !zfs.DescendantOf(dataset, d.Dataset) {
		return fmt.Errorf("dataset %q is not part of this item", dataset)
	}
	if !zfs.IsPreRestoreSnapshot(name) {
		return fmt.Errorf("snapshot %q is not a safety snapshot", name)
	}
	if s.zfs == nil {
		return errZFSHostMissing
	}
	// A restore that is still writing may be the one this snapshot undoes.
	unlock, ok := s.tryLockDomainFor(zfsDomain, "delete-safety")
	if !ok {
		return errDomainBusy
	}
	defer unlock()
	sctx, cancel := context.WithTimeout(ctx, zfsSnapshotTimeout)
	defer cancel()
	if err := s.zfs.DestroySafety(sctx, dataset, name); err != nil {
		return err
	}
	return s.store.DeleteZFSSafetySnapshot(dataset, name)
}
