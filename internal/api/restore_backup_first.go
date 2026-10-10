package api

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/paths"
)

// beforeRestoreTag marks the backup a restore took of the state it was about
// to replace. The short id of the restored snapshot follows it after a colon,
// unless the restore had no snapshot.
const beforeRestoreTag = "bv:before-restore"

type beforeRestoreKey struct{}

// withBeforeRestore marks ctx so that a backup run under it tags its
// snapshots as the state before the restore of snapshotID.
func withBeforeRestore(ctx context.Context, snapshotID string) context.Context {
	tag := beforeRestoreTag
	if snapshotID != "" {
		tag += ":" + shortID(snapshotID)
	}
	return context.WithValue(ctx, beforeRestoreKey{}, tag)
}

func isBeforeRestore(ctx context.Context) bool {
	return beforeRestoreTags(ctx) != nil
}

// beforeRestoreTags is the marker a backup under ctx puts on its snapshots,
// or nothing for an ordinary backup.
func beforeRestoreTags(ctx context.Context) []string {
	if mark, ok := ctx.Value(beforeRestoreKey{}).(string); ok {
		return []string{mark}
	}
	return nil
}

// backupBeforeRestore has save take an ordinary backup of an entry right
// before a restore replaces it. Cancelling the restore cancels that backup
// too. When it returns an error the restore must not start, and the error is
// already recorded as the restore's outcome.
//
// The copy to the off-site targets is left to the entry's next backup, so the
// restore does not wait for an upload.
func (s *Service) backupBeforeRestore(ctx context.Context, key, targetID, snapshotID string, save func(context.Context) error) error {
	stop := context.AfterFunc(ctx, func() { s.CancelBackupRun(key, "") })
	err := save(WithBulkReplicateSuppressed(withBeforeRestore(ctx, snapshotID)))
	stop()
	switch {
	case ctx.Err() != nil:
		err = ctx.Err()
	case IsBackupCancelled(err):
		err = context.Canceled
	case err != nil:
		err = fmt.Errorf("nothing was restored, because the backup of the current state failed: %w", err)
	default:
		return nil
	}
	s.finishRestoreRun(s.beginRestoreRunForTarget(targetID), "", err)
	s.progEnd(key, "restore", false, time.Now().Unix())
	return err
}

// saveContainerState backs up a container that is on the host. One that is
// gone has no state a backup could save.
func (s *Service) saveContainerState(ctx context.Context, name string) error {
	if _, found, err := s.liveContainer(ctx, name); err != nil || !found {
		return err
	}
	_, err := s.Backup(ctx, name)
	return err
}

// saveVMState is saveContainerState for a VM.
func (s *Service) saveVMState(ctx context.Context, name string) error {
	if state, err := s.virsh.State(ctx, name); err != nil || state == "" {
		return err
	}
	_, err := s.BackupVM(ctx, name)
	return err
}

// saveFileSetState is saveContainerState for a file set, which is gone when
// its folder is.
func (s *Service) saveFileSetState(ctx context.Context, id string) error {
	if found, err := s.fileSetOnHost(id); err != nil || !found {
		return err
	}
	_, err := s.BackupFileSet(ctx, id)
	return err
}

func (s *Service) fileSetOnHost(id string) (bool, error) {
	set, err := s.store.GetFileSet(id)
	if err != nil {
		return false, errFileSetNotFound
	}
	if strings.TrimSpace(set.Path) == "" {
		return false, nil
	}
	src, err := paths.Resolve(s.cfg.HostMountRoot, set.Path)
	if err != nil {
		return false, nil
	}
	if _, err := os.Stat(src); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// BackupFirstPlan says what becomes of the backup a restore in place is asked
// to take first. Status is "ok" when it will be taken, "skip" with the reason
// "gone" when the entry is not on the host and has no state to save, and
// "fail" with Detail when it cannot be taken, which keeps the restore from
// starting. Seconds and Bytes are what the entry's last backup took and
// added, both zero for an entry that was never backed up from here.
type BackupFirstPlan struct {
	Status  string `json:"status"`
	Reason  string `json:"reason,omitempty"`
	Detail  string `json:"detail,omitempty"`
	Seconds int64  `json:"seconds"`
	Bytes   int64  `json:"bytes"`
}

// backupFirstPlan answers a check that asks for a backup first. A restore
// that replaces nothing takes none and gets no answer.
func (s *Service) backupFirstPlan(ctx context.Context, req RestoreCheckRequest, sc restoreScope) *BackupFirstPlan {
	if !req.BackupFirst || !sc.inPlace {
		return nil
	}
	var found bool
	var err error
	var targetID string
	switch req.Kind {
	case checkContainer:
		if self := s.selfContainerName(ctx); self != "" && self == req.Name {
			return &BackupFirstPlan{Status: lineFail, Detail: ErrSelfBackup.Error()}
		}
		_, found, err = s.liveContainer(ctx, req.Name)
		if tg, tErr := s.store.GetTargetByContainer(req.Name); tErr == nil {
			targetID = tg.ID
		}
	case checkVM:
		var state string
		state, err = s.virsh.State(ctx, req.Name)
		found = state != ""
		if tg, tErr := s.store.GetVMTargetByName(req.Name); tErr == nil {
			targetID = tg.ID
		}
	case checkFileSet:
		found, err = s.fileSetOnHost(req.Name)
		targetID = req.Name
	default:
		return nil
	}
	switch {
	case err != nil:
		return &BackupFirstPlan{Status: lineFail, Detail: scrubError(err)}
	case !found:
		return &BackupFirstPlan{Status: lineSkip, Reason: reasonGone}
	}
	plan := &BackupFirstPlan{Status: lineOK}
	if last, lErr := s.store.LastSuccessfulBackup(targetID); lErr == nil && last != nil && last.FinishedAt != nil {
		plan.Seconds, plan.Bytes = *last.FinishedAt-last.StartedAt, last.Bytes
	}
	return plan
}
