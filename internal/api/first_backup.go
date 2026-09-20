package api

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

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
		if err := s.validateItemRepoID(repoID); err != nil {
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

// recordHome writes the settled location. When the row changed after it was
// read, the step starts over from the new row.
func (s *Service) recordHome(ctx context.Context, settings store.Settings, item store.ItemRef, step homeStep) (homeStep, error) {
	for {
		ok, err := step.commit()
		if err != nil || ok {
			return step, err
		}
		if step, err = s.prepareHome(ctx, settings, item); err != nil {
			return step, err
		}
	}
}
