package api

import (
	"context"
	"errors"
	"fmt"
	"log"
	"slices"
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
	// zfsMaxStopContainers bounds the consistency stop, whose worst case is one
	// stop grace per dependency level.
	zfsMaxStopContainers = 32
	// zfsMaxCommandBytes bounds a pre- or post-snapshot command.
	zfsMaxCommandBytes = 4 << 10
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
	candidate := store.ZFSDataset{Dataset: item.Dataset, ExcludedChildren: item.ExcludedChildren}
	for _, other := range rows {
		if zfsItemsOverlap(candidate, other) {
			return store.ZFSDataset{}, zfsRefuse("overlaps-item", other.Dataset)
		}
	}
	if err := zfsValidateExcludedChildren(item.Dataset, item.ExcludedChildren); err != nil {
		return store.ZFSDataset{}, err
	}
	if err := s.zfsValidateStopContainers(ctx, item.StopContainers); err != nil {
		return store.ZFSDataset{}, err
	}
	if err := s.validateItemRepoID(zfsDomain, item.Repo); err != nil {
		return store.ZFSDataset{}, err
	}
	if err := s.zfsCreateRefusal(ctx, item.Dataset); err != nil {
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

// zfsCreateRefusal lists the tree the way the add dialog's check does, so a
// client other than the dialog cannot store an item every run of which fails
// in its preflight. A host that cannot answer right now proves nothing, and the
// first run's preflight still asks it.
func (s *Service) zfsCreateRefusal(ctx context.Context, root string) error {
	if s.zfs == nil {
		return nil
	}
	lctx, cancel := context.WithTimeout(ctx, zfsListTimeout)
	defer cancel()
	tree, err := s.zfs.Tree(lctx, root)
	if err != nil {
		if zfs.IsNotFound(err) {
			return zfsRefuse(zfsErrCode(err), root)
		}
		return nil
	}
	if ref := zfsTreeRefusal(root, tree); ref != nil {
		return ref
	}
	return nil
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
		if vErr := s.zfsValidateUnexclude(d, *p.ExcludedChildren); vErr != nil {
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
		if *p.HookContainer == s.selfContainerName(ctx) {
			return zfsRefuse("container-is-self", *p.HookContainer)
		}
		if _, iErr := s.inspectNamed(ctx, *p.HookContainer); iErr != nil {
			return zfsRefuse("container-unknown", *p.HookContainer)
		}
	}
	for _, cmd := range []*string{p.PreSnapshot, p.PostSnapshot} {
		if cmd != nil && len(*cmd) > zfsMaxCommandBytes {
			return fmt.Errorf("a snapshot command may be at most %d bytes", zfsMaxCommandBytes)
		}
	}
	// A command with no container to run in would be stored and then skipped
	// on every run, so the snapshot the user thought was consistent is not.
	if container, pre, post := zfsPatchedHooks(d, p); container == "" && (strings.TrimSpace(pre) != "" || strings.TrimSpace(post) != "") {
		return zfsRefuse("hook-container-missing", "")
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
		if vErr := s.validateItemRepoID(zfsDomain, *p.Repo); vErr != nil {
			return vErr
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
		// Carried on the row as well: the excludes write below goes through
		// UpdateZFSDataset, which writes the enabled column too and would put
		// the old value back.
		d.Enabled = *p.Enabled
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
		container, pre, post := zfsPatchedHooks(d, p)
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

// zfsPatchedHooks is the hook container and the two commands an item has once
// the patch is applied.
func zfsPatchedHooks(d store.ZFSDataset, p ZFSDatasetPatch) (container, pre, post string) {
	container, pre, post = d.HookContainer, d.PreSnapshot, d.PostSnapshot
	if p.HookContainer != nil {
		container = *p.HookContainer
	}
	if p.PreSnapshot != nil {
		pre = *p.PreSnapshot
	}
	if p.PostSnapshot != nil {
		post = *p.PostSnapshot
	}
	return container, pre, post
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
	if members != nil {
		out.Members = members
	}
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
	// A replica switched off while the host was out of reach kept its
	// snapshots, so the clean does not ask for a target.
	unlock, ok := s.lockZFSReplica(id)
	if !ok {
		return ZFSDeleteResult{}, errZFSReplicaBusy
	}
	defer unlock()
	s.cleanZFSReplicaSource(ctx, d)
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
// reclaims the space and then removes the item itself, answering like
// DeleteZFSDataset what is still on the pool.
func (s *Service) DeleteBackupsZFSDataset(ctx context.Context, id string, removeSafety bool) (ZFSDeleteResult, error) {
	d, err := s.store.GetZFSDataset(id)
	if err != nil {
		return ZFSDeleteResult{}, fmt.Errorf("zfs: load dataset: %w", err)
	}
	settings, err := s.store.GetSettings()
	if err != nil {
		return ZFSDeleteResult{}, fmt.Errorf("read settings: %w", err)
	}
	homes, err := s.zfsBackupHomes(settings, d)
	if err != nil {
		return ZFSDeleteResult{}, err
	}
	unlock, ok := s.tryLockDomainFor(zfsDomain, "delete")
	if !ok {
		return ZFSDeleteResult{}, errDomainBusy
	}
	defer unlock()

	// Every place is listed before anything is forgotten, so a place that
	// cannot be read leaves all of them, and the entry, as they were.
	forget := make([][]string, len(homes))
	found := false
	for i, h := range homes {
		if h.own() {
			s.unlockStale(ctx, h.repo, h.mode)
		}
		snaps, err := s.zfsItemSnapshots(ctx, d, h.source())
		if err != nil {
			if !h.own() && isRepoDefinitelyAbsent(err) {
				continue
			}
			return ZFSDeleteResult{}, err
		}
		for _, snap := range snaps {
			forget[i] = append(forget[i], snap.ID)
		}
		found = true
	}
	if !found {
		return ZFSDeleteResult{}, errors.New("no backups to delete yet")
	}
	for i, h := range homes {
		if len(forget[i]) == 0 {
			continue
		}
		op := func() error { return s.engine.Forget(ctx, h.repo, forget[i], true, h.mode) }
		if err := s.retryAfterUnlock(ctx, h.repo, h.mode, op); err != nil {
			return ZFSDeleteResult{}, fmt.Errorf("forget snapshots: %w", err)
		}
	}
	return s.deleteZFSDatasetLocked(ctx, id, removeSafety)
}

// zfsBackupHomes is every place an item's backups were written to: its own
// repository, or the section's folder and, while that folder is switched off,
// the targets the backups went to straight. A folder that never held a
// repository is left out, and an append-only place refuses the whole delete.
func (s *Service) zfsBackupHomes(settings store.Settings, d store.ZFSDataset) ([]sectionPlace, error) {
	repo, err := s.zfsDatasetRepoPath(settings, d)
	if err != nil {
		return nil, err
	}
	own := sectionPlace{repo: repo, mode: s.repoModeFor(settings, zfsDomain, "local", repo)}
	var homes []sectionPlace
	if s.requireExistingRepo(repo, "") == nil {
		if f := s.primaryAppendOnly(zfsDomain, repo); f != appendOnlyNone {
			return nil, appendOnlyRefusal(f)
		}
		homes = append(homes, own)
	}
	if d.Repo != "" {
		if len(homes) == 0 {
			return nil, errors.New("no backups to delete yet")
		}
		return homes, nil
	}
	places, err := s.sectionPlacesSwitchedOn(settings, zfsDomain, repo)
	if err != nil && !errors.Is(err, ErrNoPlace) {
		return nil, err
	}
	for _, pl := range places {
		if pl.own() {
			continue
		}
		if err := s.refuseAppendOnlyTarget(settings, zfsDomain, pl.source()); err != nil {
			return nil, err
		}
		homes = append(homes, pl)
	}
	if len(homes) == 0 {
		return nil, errors.New("no backups to delete yet")
	}
	return homes, nil
}

// DiscoverZFSDatasets rebuilds the item list from the zfs: tags in storage,
// after a fresh install or a lost database. Items never overlap, so the minimal
// roots among the names found are exactly the items that were backed up.
func (s *Service) DiscoverZFSDatasets(ctx context.Context, dryRun bool) (int, []repoSkip, error) {
	settings, err := s.store.GetSettings()
	if err != nil {
		return 0, nil, fmt.Errorf("read settings: %w", err)
	}
	names, _, skipped, _, readErr := s.discoverNamesAcrossRepos(ctx, settings, zfsDomain, "zfs:")

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

	found := make([]string, 0, len(usable))
	for name := range usable {
		found = append(found, name)
	}
	rows, err := s.store.ListZFSDatasets()
	if err != nil {
		return 0, skipped, fmt.Errorf("discover zfs: list the items: %w", err)
	}

	discovered := 0
	for name, repoID := range usable {
		if zfsMinimalRoot(name, found) != name {
			continue
		}
		if zfsCoveredByItem(name, rows) {
			discovered++
			continue
		}
		// An item below this name is where its history went when the old root
		// was split into child items; a row over it would refuse its every run.
		if other, ok := zfsItemBelow(name, rows); ok {
			log.Printf("api: discover zfs: not rebuilding %q, the item %q lies inside it", name, other.Dataset) //nolint:gosec // G706: %q-quoted
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

// zfsCoveredByItem reports whether an existing item's tree holds name below its
// root, so the name's history already belongs to that item.
func zfsCoveredByItem(name string, rows []store.ZFSDataset) bool {
	for _, d := range rows {
		if zfs.DescendantOf(name, d.Dataset) {
			return true
		}
	}
	return false
}

// zfsItemBelow finds the first existing item whose root lies below name.
func zfsItemBelow(name string, rows []store.ZFSDataset) (store.ZFSDataset, bool) {
	for _, d := range rows {
		if zfs.DescendantOf(d.Dataset, name) {
			return d, true
		}
	}
	return store.ZFSDataset{}, false
}

// zfsMinimalRoot names the item a dataset belongs to: the shortest of the found
// names that is an ancestor of it, or the dataset itself when none of them is.
func zfsMinimalRoot(dataset string, names []string) string {
	root := dataset
	for _, other := range names {
		if zfs.DescendantOf(dataset, other) && len(other) < len(root) {
			root = other
		}
	}
	return root
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
	owns, err := s.zfsOwnsDataset(d)
	if err != nil {
		return nil, err
	}
	all, err := s.listSnapshots(ctx, repo, s.repoModeFor(settings, zfsDomain, source, repo))
	if err != nil {
		return nil, err
	}
	out := make([]restic.Snapshot, 0, len(all))
	for _, snap := range all {
		if zfsSnapshotInTree(snap, d.Dataset, owns) {
			out = append(out, snap)
		}
	}
	return out, nil
}

// zfsSnapshotInTree reports whether a snapshot holds a dataset of the tree
// under root that owns gives to the item.
func zfsSnapshotInTree(snap restic.Snapshot, root string, owns func(dataset string) bool) bool {
	for _, tag := range snap.Tags {
		dataset, ok := strings.CutPrefix(tag, "zfs:")
		if ok && (dataset == root || zfs.DescendantOf(dataset, root)) && owns(dataset) {
			return true
		}
	}
	return false
}

// zfsOwnsDataset returns whether a dataset under d's root holds d's backups.
// An item can sit inside the tree of another that leaves it out, and both may
// write to the same repository. A dataset belongs to the item whose newest run
// recorded it, as ZFSDatasetOwners says; one that no existing item's run
// recorded belongs to the item whose root lies deepest above it.
func (s *Service) zfsOwnsDataset(d store.ZFSDataset) (func(dataset string) bool, error) {
	items, err := s.store.ListZFSDatasets()
	if err != nil {
		return nil, err
	}
	owners, err := s.store.ZFSDatasetOwners()
	if err != nil {
		return nil, err
	}
	exists := make(map[string]bool, len(items))
	var nested []string
	for _, it := range items {
		exists[it.ID] = true
		if it.ID != d.ID && zfs.DescendantOf(it.Dataset, d.Dataset) {
			nested = append(nested, it.Dataset)
		}
	}
	return func(dataset string) bool {
		if owner, ok := owners[dataset]; ok && exists[owner] {
			return owner == d.ID
		}
		return !slices.ContainsFunc(nested, func(root string) bool {
			return dataset == root || zfs.DescendantOf(dataset, root)
		})
	}, nil
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

// zfsExcludesDataset reports whether the item leaves name out of its tree.
func zfsExcludesDataset(d store.ZFSDataset, name string) bool {
	for _, ex := range d.ExcludedChildren {
		if name == ex || zfs.DescendantOf(name, ex) {
			return true
		}
	}
	return false
}

// zfsItemsOverlap reports whether two items would snapshot the same dataset.
// A tree below another item's root is its own item when that item leaves it
// out.
func zfsItemsOverlap(a, b store.ZFSDataset) bool {
	switch {
	case a.Dataset == b.Dataset:
		return true
	case zfs.DescendantOf(b.Dataset, a.Dataset):
		return !zfsExcludesDataset(a, b.Dataset)
	case zfs.DescendantOf(a.Dataset, b.Dataset):
		return !zfsExcludesDataset(b, a.Dataset)
	}
	return false
}

// zfsValidateUnexclude refuses a new exclusion list that takes a dataset back
// while another item has it as its root.
func (s *Service) zfsValidateUnexclude(d store.ZFSDataset, names []string) error {
	rows, err := s.store.ListZFSDatasets()
	if err != nil {
		return err
	}
	updated := d
	updated.ExcludedChildren = names
	for _, other := range rows {
		if other.ID != d.ID && !zfsItemsOverlap(d, other) && zfsItemsOverlap(updated, other) {
			return zfsRefuse("overlaps-item", other.Dataset)
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
	if len(names) > zfsMaxStopContainers {
		return fmt.Errorf("at most %d containers can be stopped for one item", zfsMaxStopContainers)
	}
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
