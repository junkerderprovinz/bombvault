package api

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"

	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// backupPresence is the has-backups answer with "could not read" kept apart.
type backupPresence int

const (
	backupsNone backupPresence = iota
	backupsPresent
	backupsUnreadable
)

// itemBackups asks the item's current location: a successful run, or snapshots
// of its identity there. An open item's location is the domain path.
func (s *Service) itemBackups(ctx context.Context, item store.ItemRef) (backupPresence, error) {
	rowID, err := s.itemRowID(item)
	if err != nil {
		return backupsNone, err
	}
	if rowID != "" {
		run, err := s.store.LastSuccessfulBackup(rowID)
		if err != nil {
			return backupsNone, err
		}
		if run != nil {
			return backupsPresent, nil
		}
	}
	var snaps []restic.Snapshot
	switch item.Domain {
	case "containers":
		snaps, err = s.Snapshots(ctx, item.Key, "local")
	case "vms":
		snaps, err = s.SnapshotsVM(ctx, item.Key, "local")
	default:
		snaps, err = s.SnapshotsFileSet(ctx, item.Key, "local")
	}
	switch {
	case errors.Is(err, errFileSetNotFound):
		return backupsNone, err
	case err != nil:
		return backupsUnreadable, err
	case len(snaps) > 0:
		return backupsPresent, nil
	}
	return backupsNone, nil
}

// itemRowID is the id an item's runs are recorded under, "" for a container or
// VM that has no row yet.
func (s *Service) itemRowID(item store.ItemRef) (string, error) {
	var id string
	var err error
	switch item.Domain {
	case "containers":
		var tg store.Target
		tg, err = s.store.GetTargetByContainer(item.Key)
		id = tg.ID
	case "vms":
		var vm store.VMTarget
		vm, err = s.store.GetVMTargetByName(item.Key)
		id = vm.ID
	default:
		return item.Key, nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return id, err
}

// countsAsBackedUp folds an unreadable location into yes: moving an item whose
// history cannot be seen risks splitting it.
func countsAsBackedUp(p backupPresence, err error) (bool, error) {
	if p == backupsUnreadable {
		return true, nil
	}
	return p == backupsPresent, err
}

// settleHome is the step before an open item's first backup. It returns the repo
// id the backup writes to and a commit the entry point calls once EnsureRepo
// succeeded and the item exists; commit reports false when the row changed.
func (s *Service) settleHome(ctx context.Context, settings store.Settings, item store.ItemRef) (string, func() (bool, error), error) {
	read, err := s.store.ItemHome(item)
	if err != nil {
		return "", nil, err
	}
	if read.Choice != store.RepoOpen {
		return read.Repo, func() (bool, error) { return true, nil }, nil
	}
	presence, err := s.itemBackups(ctx, item)
	switch {
	case presence == backupsUnreadable:
		return "", nil, fmt.Errorf("the %s repository could not be read, so this item gets no location yet: %w", item.Domain, err)
	case err != nil:
		return "", nil, err
	}
	repoID := ""
	if presence == backupsNone {
		p, err := s.readPlacement(settings, item.Domain)
		if err != nil {
			return "", nil, err
		}
		repoID, _ = p.effectiveHome(read)
		if err := s.validateItemRepoID(item.Domain, repoID); err != nil {
			return "", nil, fmt.Errorf("the %s default points at a repository that cannot take this backup: %w", item.Domain, err)
		}
	}
	commit := func() (bool, error) {
		return s.store.WritePlacement(item, &store.HomeWrite{Repo: repoID, Choice: store.RepoChosen}, nil, &read)
	}
	return repoID, commit, nil
}

// homeStep is where an entry point backs up and the write that records it there.
type homeStep struct {
	repo   string
	mode   restic.Mode
	commit func() (bool, error)
}

// prepareHome settles the item's location and makes sure a repository exists
// there. The location is written by recordHome, once the item is known to exist.
func (s *Service) prepareHome(ctx context.Context, settings store.Settings, item store.ItemRef) (homeStep, error) {
	repoID, commit, err := s.settleHome(ctx, settings, item)
	if err != nil {
		return homeStep{}, err
	}
	repo, err := s.itemRepoPath(repoID, func() (string, error) { return s.repoFor(settings, item.Domain, "local") })
	if err != nil {
		return homeStep{}, err
	}
	mode := s.primaryModeFor(settings, item.Domain, repo)
	if err := s.EnsureRepo(ctx, repo, mode); err != nil {
		return homeStep{}, err
	}
	s.unlockStale(ctx, repo, mode)
	return homeStep{repo: repo, mode: mode, commit: commit}, nil
}

// homeSettleAttempts caps recordHome's retry loop. Each retry re-derives the
// item's location and probes its repository again, both under the domain
// lock, so a row under constant outside contention must not be able to hold
// that lock forever.
const homeSettleAttempts = 3

// errHomeSettleExhausted means the item's row kept changing out from under
// every attempt to record its settled location, so recordHome gave up rather
// than hold the domain lock without end.
var errHomeSettleExhausted = errors.New("its location kept changing while the first backup tried to settle it")

// recordHome writes the settled location, starting the step over from the
// row's current state when it changed since it was read. It gives up after a
// few turns rather than retry without end.
func (s *Service) recordHome(ctx context.Context, settings store.Settings, item store.ItemRef, step homeStep) (homeStep, error) {
	for attempt := 1; ; attempt++ {
		ok, err := step.commit()
		if err != nil || ok {
			return step, err
		}
		if attempt == homeSettleAttempts {
			return step, fmt.Errorf("%s %q: %w", item.Domain, item.Key, errHomeSettleExhausted)
		}
		if step, err = s.prepareHome(ctx, settings, item); err != nil {
			return step, err
		}
	}
}

// DiscoverResult is what one Discover pass found and did besides the rows it wrote.
type DiscoverResult struct {
	Found    int
	Skipped  []repoSkip
	Paused   bool     // this pass paused the domain's replication
	LeftOpen []string // names whose repo stayed unset because a backup held the domain
	Direct   []directFinding
}

// discoverWrite is the location a row Discover creates starts with. From a pass
// that could not read the domain path the evidence is incomplete, so the row
// stays on the domain path and a later full pass may still place it.
//
// A repository the item may not use leaves the row open. What the pass found
// is evidence of where snapshots lie, and settling a home no interactive route
// would accept gives the item a location every later edit refuses.
func (s *Service) discoverWrite(domain, name, found string, readErr error) store.HomeWrite {
	if readErr != nil {
		return store.HomeWrite{Choice: store.RepoChosenUnread}
	}
	if err := s.validateItemRepoID(domain, found); err != nil {
		log.Printf("api: discover %s: %q was found in a repository it may not use, so its location stays open: %v", domain, name, err) //nolint:gosec // G706: domain is a fixed literal and the name is %q-quoted
		return store.HomeWrite{Choice: store.RepoOpen}
	}
	return store.HomeWrite{Repo: found, Choice: store.RepoChosen}
}

// discoverHome gives an existing row the location Discover found, while the row
// is open or was left empty by an unreadable pass. It needs the domain lock and
// reports true when the row stays as it is because a backup held the domain.
func (s *Service) discoverHome(ctx context.Context, item store.ItemRef, found string, readErr error, locked bool) (bool, error) {
	read, err := s.store.ItemHome(item)
	if err != nil || read.Choice == store.RepoChosen {
		return false, err
	}
	if !locked {
		return true, nil
	}
	want := s.discoverWrite(item.Domain, item.Key, found, readErr)
	if readErr == nil {
		presence, err := s.itemBackups(ctx, item)
		switch {
		case presence == backupsUnreadable:
			want = store.HomeWrite{Choice: store.RepoChosenUnread}
		case err != nil:
			return false, err
		case presence == backupsPresent:
			want = store.HomeWrite{Choice: store.RepoChosen}
		}
	}
	_, err = s.store.WritePlacement(item, &want, nil, &read)
	return false, err
}

// discoverLockReason labels the domain lock a Discover pass holds while it
// writes, so a backup refused during the pass names Discover rather than the
// unrelated item-PATCH placement reason.
const discoverLockReason = "discover"

// discoverLock takes the domain lock for a pass that writes. Without it the pass
// still rebuilds rows but leaves open locations alone.
func (s *Service) discoverLock(domain string, dryRun bool) (func(), bool) {
	if dryRun {
		return func() {}, false
	}
	unlock, ok := s.tryLockDomainFor(domain, discoverLockReason)
	if !ok {
		return func() {}, false
	}
	return unlock, true
}

// pauseAfterDiscover pauses the domain's replication when Discover rebuilt rows
// in a database that never backed up or copied anything for it: the rules that
// kept items off site were lost with the old one. A domain the operator
// already confirmed stays out of this for good, since a restore can rebuild
// its rows again without writing a backup run.
func (s *Service) pauseAfterDiscover(ctx context.Context, domain string, res *DiscoverResult) error {
	d, _, err := s.store.PlacementDefaultFor(domain)
	if err != nil {
		return err
	}
	if d.ConfirmedManually {
		return nil
	}
	backedUp, copied, err := s.store.DomainHasHistory(domain)
	if err != nil || backedUp || copied {
		return err
	}
	if err := s.pausePlacement(ctx, domain, reasonDiscover); err != nil {
		return err
	}
	res.Paused = true
	return nil
}
