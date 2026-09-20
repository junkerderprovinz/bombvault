package api

import (
	"context"
	"database/sql"
	"errors"

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
