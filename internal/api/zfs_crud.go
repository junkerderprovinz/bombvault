package api

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/junkerderprovinz/bombvault/internal/backup"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/schedule"
	"github.com/junkerderprovinz/bombvault/internal/store"
	"github.com/junkerderprovinz/bombvault/internal/zfs"
)

const (
	// zfsMaxCreateItems bounds one add, which the dialog sends as a batch over
	// a tree the user ticked through.
	zfsMaxCreateItems = 200
	// zfsMaxExcludedChildren bounds the per-item exclusion list, which is
	// written into the row as one JSON value.
	zfsMaxExcludedChildren = 500
)

// ZFSCreateItem is one item the add dialog asks for.
type ZFSCreateItem struct {
	Dataset          string   `json:"dataset"`
	ExcludedChildren []string `json:"excludedChildren"`
	Excludes         []string `json:"excludes"`
	StopContainers   []string `json:"stopContainers"`
	Repo             string   `json:"repo"`
	Enabled          *bool    `json:"enabled"`
}

// ZFSCreateResult is what became of one requested item. A batch never fails as
// a whole, so a tree the user ticked through comes back item by item.
type ZFSCreateResult struct {
	Dataset string `json:"dataset"`
	ID      string `json:"id"`
	Code    string `json:"code"`
	Detail  string `json:"detail"`
}

// ZFSDatasetPatch carries the settings of one item. A nil field is not part of
// the patch; the root dataset is not in here because it is the item's identity.
type ZFSDatasetPatch struct {
	Enabled          *bool
	Excludes         *[]string
	ExcludedChildren *[]string
	ScheduleCadence  *string
	Repo             *string
	StopContainers   *[]string
	HookContainer    *string
	PreSnapshot      *string
	PostSnapshot     *string
}

// ZFSRunDetail is what one ZFS run did, beyond what the runs table holds.
type ZFSRunDetail struct {
	Members       []store.ZFSRunMember `json:"members"`
	WindowSeconds int64                `json:"windowSeconds"`
	HookDetail    string               `json:"hookDetail"`
}

// ZFSDeleteResult says what the delete left on the pool.
type ZFSDeleteResult struct {
	LeftoversRemaining int `json:"leftoversRemaining"`
	SafetyRemaining    int `json:"safetyRemaining"`
}

// zfsRefuse builds a refusal that carries its reason code to the page, which
// builds the sentence from the code rather than showing this text.
func zfsRefuse(code, detail string) error {
	return &backup.ZFSRefusal{Code: code, Detail: detail}
}

// zfsRefusalCode reads the reason code off a refusal of this domain.
func zfsRefusalCode(err error) (string, bool) {
	var ref *backup.ZFSRefusal
	if errors.As(err, &ref) && ref.Code != "" {
		return ref.Code, true
	}
	return "", false
}

// CreateZFSDatasets adds the items the dialog ticked through, one verdict per
// item. An item that is refused is not stored, and the rest of the batch still
// lands.
func (s *Service) CreateZFSDatasets(ctx context.Context, items []ZFSCreateItem) []ZFSCreateResult {
	if len(items) > zfsMaxCreateItems {
		log.Printf("api: zfs: refusing an add of %d items, the limit is %d", len(items), zfsMaxCreateItems)
		return nil
	}
	rows, err := s.store.ListZFSDatasets()
	if err != nil {
		log.Printf("api: zfs: could not read the existing items before adding: %v", err)
	}
	out := make([]ZFSCreateResult, 0, len(items))
	for _, item := range items {
		res := ZFSCreateResult{Dataset: item.Dataset, Code: "ok"}
		created, err := s.createZFSDataset(ctx, item, rows)
		if err != nil {
			code, ok := zfsRefusalCode(err)
			if !ok {
				code = "zfs-error"
			}
			res.Code, res.Detail = code, err.Error()
		} else {
			res.ID = created.ID
			rows = append(rows, created)
		}
		out = append(out, res)
	}
	return out
}

// createZFSDataset validates one requested item against the items that exist
// and stores it.
func (s *Service) createZFSDataset(ctx context.Context, item ZFSCreateItem, rows []store.ZFSDataset) (store.ZFSDataset, error) {
	if err := zfsValidateRootName(item.Dataset); err != nil {
		return store.ZFSDataset{}, err
	}
	for _, other := range rows {
		if other.Dataset == item.Dataset ||
			zfs.DescendantOf(item.Dataset, other.Dataset) ||
			zfs.DescendantOf(other.Dataset, item.Dataset) {
			return store.ZFSDataset{}, zfsRefuse("overlaps-item", other.Dataset)
		}
	}
	if err := zfsValidateExcludedChildren(item.Dataset, item.ExcludedChildren); err != nil {
		return store.ZFSDataset{}, err
	}
	if err := s.zfsValidateStopContainers(ctx, item.StopContainers); err != nil {
		return store.ZFSDataset{}, err
	}
	enabled := item.Enabled == nil || *item.Enabled
	return s.store.CreateZFSDataset(store.ZFSDataset{
		Dataset:          item.Dataset,
		Enabled:          enabled,
		Excludes:         item.Excludes,
		ExcludedChildren: item.ExcludedChildren,
		StopContainers:   item.StopContainers,
		Repo:             item.Repo,
	})
}

// PatchZFSDataset changes one item's settings. Everything is validated before
// anything is written, and switching an item on looks at its tree so the page
// shows what the next run will find.
func (s *Service) PatchZFSDataset(ctx context.Context, id string, p ZFSDatasetPatch) error {
	d, err := s.store.GetZFSDataset(id)
	if err != nil {
		return fmt.Errorf("zfs: load dataset: %w", err)
	}
	if p.ExcludedChildren != nil {
		if vErr := zfsValidateExcludedChildren(d.Dataset, *p.ExcludedChildren); vErr != nil {
			return vErr
		}
	}
	if p.Excludes != nil {
		if vErr := s.zfsValidateExcludes(d, *p.Excludes); vErr != nil {
			return vErr
		}
	}
	if p.StopContainers != nil {
		if vErr := s.zfsValidateStopContainers(ctx, *p.StopContainers); vErr != nil {
			return vErr
		}
	}
	if p.HookContainer != nil && *p.HookContainer != "" {
		if _, iErr := s.inspectNamed(ctx, *p.HookContainer); iErr != nil {
			return zfsRefuse("container-unknown", *p.HookContainer)
		}
	}
	if p.ScheduleCadence != nil {
		if vErr := zfsValidateCadence(*p.ScheduleCadence); vErr != nil {
			return vErr
		}
	}
	if p.Repo != nil && strings.TrimSpace(*p.Repo) != d.Repo {
		has, hErr := s.zfsHasBackups(ctx, d)
		if hErr != nil {
			return fmt.Errorf("the repository cannot be changed right now: %w", hErr)
		}
		if has {
			return errors.New("cannot change the repository of a dataset item that already has backups; " +
				"they stay in the repository they were written to and nothing moves them. Delete its backups first")
		}
	}

	if err := s.applyZFSPatch(d, p); err != nil {
		return err
	}
	if p.Enabled != nil && *p.Enabled {
		children := d.ExcludedChildren
		if p.ExcludedChildren != nil {
			children = *p.ExcludedChildren
		}
		s.recordZFSCheck(d, s.CheckZFSDataset(ctx, d.Dataset, children))
	}
	return nil
}

// applyZFSPatch writes the fields the patch carries, each through the setter
// that owns its column.
func (s *Service) applyZFSPatch(d store.ZFSDataset, p ZFSDatasetPatch) error {
	if p.Enabled != nil {
		if err := s.store.SetZFSDatasetEnabled(d.ID, *p.Enabled); err != nil {
			return err
		}
	}
	if p.ExcludedChildren != nil {
		if err := s.store.SetZFSExcludedChildren(d.ID, *p.ExcludedChildren); err != nil {
			return err
		}
	}
	if p.StopContainers != nil {
		if err := s.store.SetZFSDatasetStopContainers(d.ID, *p.StopContainers); err != nil {
			return err
		}
	}
	if p.ScheduleCadence != nil {
		if err := s.store.SetZFSDatasetScheduleCadence(d.ID, strings.TrimSpace(*p.ScheduleCadence)); err != nil {
			return err
		}
	}
	if p.Repo != nil {
		if err := s.store.SetZFSDatasetRepo(d.ID, strings.TrimSpace(*p.Repo)); err != nil {
			return err
		}
	}
	if p.HookContainer != nil || p.PreSnapshot != nil || p.PostSnapshot != nil {
		container, pre, post := d.HookContainer, d.PreSnapshot, d.PostSnapshot
		if p.HookContainer != nil {
			container = *p.HookContainer
		}
		if p.PreSnapshot != nil {
			pre = *p.PreSnapshot
		}
		if p.PostSnapshot != nil {
			post = *p.PostSnapshot
		}
		if err := s.store.SetZFSHooks(d.ID, container, pre, post); err != nil {
			return err
		}
	}
	if p.Excludes != nil {
		d.Excludes = *p.Excludes
		if err := s.store.UpdateZFSDataset(d); err != nil {
			return err
		}
	}
	return nil
}

// ZFSRunDetail returns what one run did to each dataset of the tree, how long
// its applications were held and what a failing post-snapshot command said.
func (s *Service) ZFSRunDetail(_ context.Context, runID string) (ZFSRunDetail, error) {
	out := ZFSRunDetail{Members: []store.ZFSRunMember{}, WindowSeconds: -1}
	if run, err := s.store.GetZFSRun(runID); err == nil {
		out.WindowSeconds = run.WindowSeconds
		out.HookDetail = zfsDetail(run.HookDetail)
	}
	members, err := s.store.ListZFSRunMembers(runID)
	if err != nil {
		return out, err
	}
	out.Members = members
	return out, nil
}

// DeleteZFSDataset removes an item and leaves the pool as clean as it can: the
// leftover stamps of its own runs go first, its safety snapshots only when the
// user asked, and the answer says what is still there.
func (s *Service) DeleteZFSDataset(ctx context.Context, id string, removeSafety bool) (ZFSDeleteResult, error) {
	unlock, ok := s.tryLockDomainFor(zfsDomain, "delete")
	if !ok {
		return ZFSDeleteResult{}, errDomainBusy
	}
	defer unlock()
	return s.deleteZFSDatasetLocked(ctx, id, removeSafety)
}

func (s *Service) deleteZFSDatasetLocked(ctx context.Context, id string, removeSafety bool) (ZFSDeleteResult, error) {
	d, err := s.store.GetZFSDataset(id)
	if err != nil {
		return ZFSDeleteResult{}, fmt.Errorf("zfs: load dataset: %w", err)
	}
	var res ZFSDeleteResult
	safety, sErr := s.store.ListZFSSafetySnapshots(id)
	if sErr != nil {
		log.Printf("api: zfs: could not read the safety snapshots of %s: %v", d.Dataset, sErr)
	}
	res.SafetyRemaining = len(safety)
	if s.zfs != nil {
		remaining, wErr := s.sweepZFSTree(ctx, d)
		if wErr != nil {
			log.Printf("api: zfs: sweeping %s before it was removed failed: %v", d.Dataset, wErr)
			remaining = d.LeftoverCount
		}
		res.LeftoversRemaining = remaining
		if removeSafety {
			res.SafetyRemaining = s.destroyZFSSafetySnapshots(ctx, d, safety)
		}
	} else {
		res.LeftoversRemaining = d.LeftoverCount
	}
	if err := s.store.DeleteZFSDataset(id); err != nil {
		return res, fmt.Errorf("zfs: delete dataset: %w", err)
	}
	return res, nil
}

// destroyZFSSafetySnapshots removes the snapshots an in-place restore kept and
// reports how many survived. A failure is not fatal: the row goes either way,
// and what is left is named so the user can remove it by hand.
func (s *Service) destroyZFSSafetySnapshots(ctx context.Context, d store.ZFSDataset, safety []store.ZFSSafetySnapshot) int {
	remaining := 0
	for _, snap := range safety {
		if err := s.zfs.DestroySafety(ctx, snap.Dataset, snap.Name); err != nil {
			remaining++
			log.Printf("api: zfs: %s@%s survived the delete of %s: %v", snap.Dataset, snap.Name, d.Dataset, err)
		}
	}
	return remaining
}

// DeleteBackupsZFSDataset forgets every snapshot the item's tree ever wrote,
// reclaims the space and then removes the item itself.
func (s *Service) DeleteBackupsZFSDataset(ctx context.Context, id string) error {
	d, err := s.store.GetZFSDataset(id)
	if err != nil {
		return fmt.Errorf("zfs: load dataset: %w", err)
	}
	settings, err := s.store.GetSettings()
	if err != nil {
		return fmt.Errorf("read settings: %w", err)
	}
	// The item's own repository, not the domain's: its snapshots live wherever
	// it backs up, and reading the domain repository would report an empty
	// history for an item that has one.
	repo, err := s.zfsDatasetRepoPath(settings, d)
	if err != nil {
		return err
	}
	if f := s.primaryAppendOnly(zfsDomain, repo); f != appendOnlyNone {
		return appendOnlyRefusal(f)
	}
	if err := s.requireExistingRepo(repo, "no backups to delete yet"); err != nil {
		return err
	}
	unlock, ok := s.tryLockDomainFor(zfsDomain, "delete")
	if !ok {
		return errDomainBusy
	}
	defer unlock()
	mode := s.repoModeFor(settings, zfsDomain, "local", repo)
	s.unlockStale(ctx, repo, mode)

	snaps, err := s.zfsItemSnapshots(ctx, d, "local")
	if err != nil {
		return err
	}
	ids := make([]string, 0, len(snaps))
	for _, snap := range snaps {
		ids = append(ids, snap.ID)
	}
	if len(ids) > 0 {
		if err := s.engine.Forget(ctx, repo, ids, true, mode); err != nil {
			return fmt.Errorf("forget snapshots: %w", err)
		}
	}
	_, err = s.deleteZFSDatasetLocked(ctx, id, false)
	return err
}

// DiscoverZFSDatasets rebuilds the item list from the zfs: tags in storage,
// after a fresh install or a lost database. Items never overlap, so the minimal
// roots among the names found are exactly the items that were backed up.
func (s *Service) DiscoverZFSDatasets(ctx context.Context, dryRun bool) (int, []repoSkip, error) {
	settings, err := s.store.GetSettings()
	if err != nil {
		return 0, nil, fmt.Errorf("read settings: %w", err)
	}
	names, _, skipped, readErr := s.discoverNamesAcrossRepos(ctx, settings, zfsDomain, "zfs:")

	// Dataset names carry slashes and spaces, so the boundary charset of the
	// other domains would drop every one of them.
	usable := make(map[string]string, len(names))
	for name, repoID := range names {
		if vErr := zfs.ValidateDatasetName(name); vErr != nil {
			log.Printf("api: discover zfs: skipping the unusable dataset name %q: %v", name, vErr) //nolint:gosec // G706: %q-quoted
			continue
		}
		usable[name] = repoID
	}

	discovered := 0
	for name, repoID := range usable {
		if zfsHasAncestorIn(name, usable) {
			continue
		}
		if dryRun {
			discovered++
			continue
		}
		// Never attributed from an incomplete pass, as the other Discover
		// passes do: a repository that could not be read may hold the newer
		// snapshots of this very name.
		if readErr != nil && repoID != "" {
			log.Printf("api: discover zfs: the domain's own repository could not be read, so %q keeps the default repository until a pass that can see every repository", name) //nolint:gosec // G706: %q-quoted
			repoID = ""
		}
		if existing, gErr := s.store.GetZFSDatasetByName(name); gErr == nil {
			// An empty repository column is no choice, only the domain default.
			// Left empty, the item would back up to the domain repository while
			// its history sits somewhere else.
			if repoID != "" && strings.TrimSpace(existing.Repo) == "" {
				if sErr := s.store.SetZFSDatasetRepo(existing.ID, repoID); sErr != nil {
					log.Printf("api: discover zfs: could not put %q back on the repository its snapshots are in: %v", name, sErr) //nolint:gosec // G706: %q-quoted
				}
			}
			discovered++
			continue
		}
		// Stored switched off: the stop list, the commands and the exclusions
		// are not in the tags and have to be reviewed before the next run.
		if _, cErr := s.store.CreateZFSDataset(store.ZFSDataset{Dataset: name, Enabled: false, Repo: repoID}); cErr != nil {
			log.Printf("api: discover zfs: could not create the item %q: %v", name, cErr) //nolint:gosec // G706: %q-quoted
			continue
		}
		discovered++
	}
	return discovered, skipped, readErr
}

// zfsHasAncestorIn reports whether one of the other found names is an ancestor
// of name, which makes name a member of that item rather than an item.
func zfsHasAncestorIn(name string, names map[string]string) bool {
	for other := range names {
		if zfs.DescendantOf(name, other) {
			return true
		}
	}
	return false
}

// zfsItemSnapshots lists everything an item's tree ever wrote: the root's own
// snapshots and those of its members, each under its own dataset name.
func (s *Service) zfsItemSnapshots(ctx context.Context, d store.ZFSDataset, source string) ([]restic.Snapshot, error) {
	settings, err := s.store.GetSettings()
	if err != nil {
		return nil, fmt.Errorf("read settings: %w", err)
	}
	repo, err := s.zfsDatasetRepoFor(settings, d, source)
	if err != nil {
		return nil, err
	}
	if localRepoMissing(repo) {
		if s.repoEstablished(repo) && !s.destinationMounted(repo) {
			return nil, ErrBackupPathNotMounted
		}
		return nil, nil
	}
	all, err := s.listSnapshots(ctx, repo, s.repoModeFor(settings, zfsDomain, source, repo))
	if err != nil {
		return nil, err
	}
	out := make([]restic.Snapshot, 0, len(all))
	for _, snap := range all {
		if zfsSnapshotInTree(snap, d.Dataset) {
			out = append(out, snap)
		}
	}
	return out, nil
}

// zfsSnapshotInTree reports whether a snapshot belongs to the tree under root.
func zfsSnapshotInTree(snap restic.Snapshot, root string) bool {
	for _, tag := range snap.Tags {
		dataset, ok := strings.CutPrefix(tag, "zfs:")
		if ok && (dataset == root || zfs.DescendantOf(dataset, root)) {
			return true
		}
	}
	return false
}

// zfsDatasetRepoFor resolves where one item's snapshots are for a given source.
// An off-site copy belongs to the domain's target, never to the item's own
// repository, so the source logic is the one every other domain uses.
func (s *Service) zfsDatasetRepoFor(settings store.Settings, d store.ZFSDataset, source string) (string, error) {
	if isOffsiteSource(source) {
		return s.repoFor(settings, zfsDomain, source)
	}
	return s.zfsDatasetRepoPath(settings, d)
}

// zfsHasBackups reports whether the item's tree already has snapshots, which
// pins it to the repository they were written to. A repository that cannot be
// read counts as having them: moving an item on a guess strands its history.
func (s *Service) zfsHasBackups(ctx context.Context, d store.ZFSDataset) (bool, error) {
	run, err := s.store.LastSuccessfulBackup(d.ID)
	if err != nil {
		return false, err
	}
	if run != nil {
		return true, nil
	}
	snaps, err := s.zfsItemSnapshots(ctx, d, "local")
	if err != nil {
		return true, err
	}
	return len(snaps) > 0, nil
}

// zfsValidateRootName checks a name a user chose as an item root.
func zfsValidateRootName(name string) error {
	err := zfs.ValidateDatasetName(name)
	if err == nil {
		return nil
	}
	var ne *zfs.NameError
	if errors.As(err, &ne) {
		return zfsRefuse(ne.Code, name)
	}
	return zfsRefuse("invalid-name", name)
}

// zfsValidateExcludedChildren checks that every excluded name is a dataset
// below the item's root. The root itself cannot be excluded: an item is its
// tree, and an item without its root is a different item.
func zfsValidateExcludedChildren(root string, names []string) error {
	if len(names) > zfsMaxExcludedChildren {
		return zfsRefuse("invalid-exclude", fmt.Sprintf("more than %d excluded datasets", zfsMaxExcludedChildren))
	}
	for _, name := range names {
		if !zfs.DescendantOf(name, root) {
			return zfsRefuse("invalid-exclude", name)
		}
		if err := zfs.ValidateDatasetName(name); err != nil {
			return zfsRefuse("invalid-exclude", name)
		}
	}
	return nil
}

// zfsValidateExcludes refuses a restic pattern that covers a whole dataset of
// the tree. That dataset would still be snapshotted and still count as backed
// up while nothing of it reached the repository; the excluded-children list is
// where a whole dataset is left out.
func (s *Service) zfsValidateExcludes(d store.ZFSDataset, patterns []string) error {
	members, err := s.store.ListZFSMembers(d.ID)
	if err != nil {
		return err
	}
	for _, pattern := range patterns {
		clean := strings.TrimSuffix(strings.TrimSpace(pattern), "/")
		for _, m := range members {
			if m.Dataset != d.Dataset && clean == zfsRelPath(d.Dataset, m.Dataset) {
				return zfsRefuse("invalid-exclude", m.Dataset)
			}
		}
	}
	return nil
}

// zfsValidateStopContainers checks that every container of the consistency stop
// list exists and is not BombVault itself, which would stop the run with it.
func (s *Service) zfsValidateStopContainers(ctx context.Context, names []string) error {
	if len(names) == 0 || s.docker == nil {
		return nil
	}
	self := s.selfContainerName(ctx)
	for _, name := range names {
		if name == self {
			return zfsRefuse("container-is-self", name)
		}
		if _, err := s.inspectNamed(ctx, name); err != nil {
			return zfsRefuse("container-unknown", name)
		}
	}
	return nil
}

// zfsValidateCadence checks a per-item schedule against the domain grammar.
// An interval cadence is refused for the same reason the other domains refuse
// it: it maps back to the domain default instead of giving the item its own
// entry, so it would be stored and do nothing.
func zfsValidateCadence(cadence string) error {
	cadence = strings.TrimSpace(cadence)
	if cadence == "" {
		return nil
	}
	cad, err := schedule.ParseCadence(cadence)
	if err != nil {
		return fmt.Errorf("invalid schedule: %w", err)
	}
	if cad.IntervalDays > 0 {
		return errors.New("per-item schedules do not support 'everyN': use 'off', 'daily HH:MM', 'weekly DOW HH:MM', or a cron expression")
	}
	return nil
}
