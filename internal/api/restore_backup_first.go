package api

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/backup"
	"github.com/junkerderprovinz/bombvault/internal/paths"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
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

// newestToRestore is the newest of snaps, which restic lists oldest first, for a
// restore that names no snapshot. One taken before a restore holds the state that
// restore set out to replace, so it only counts when there is no other.
func newestToRestore(snaps []restic.Snapshot) restic.Snapshot {
	for _, sn := range slices.Backward(snaps) {
		if !slices.ContainsFunc(sn.Tags, isBeforeRestoreTag) {
			return sn
		}
	}
	return snaps[len(snaps)-1]
}

func isBeforeRestoreTag(tag string) bool {
	return tag == beforeRestoreTag || strings.HasPrefix(tag, beforeRestoreTag+":")
}

// saveContainerState backs up a container that has state a backup could save,
// see containerNothingToSave. A container gone from the host cannot be backed up
// as one, while the folders the restore writes over can still hold its data.
func (s *Service) saveContainerState(ctx context.Context, name string, folders []string) error {
	why, err := s.containerNothingToSave(ctx, name)
	switch {
	case err != nil:
		return err
	case why == reasonGone:
		tg, err := s.store.GetTargetByContainer(name)
		if err != nil {
			return err
		}
		return s.saveFoldersOfGone(ctx, store.ItemRef{Domain: "containers", Key: name}, []string{"container:" + name, "p1"}, tg.ID, folders)
	case why != "":
		return nil
	}
	_, err = s.Backup(ctx, name)
	return err
}

// saveVMState is saveContainerState for a VM.
func (s *Service) saveVMState(ctx context.Context, name string, folders []string) error {
	state, err := s.virsh.State(ctx, name)
	if err != nil {
		return err
	}
	if state == "" {
		tg, err := s.store.GetVMTargetByName(name)
		if err != nil {
			return err
		}
		return s.saveFoldersOfGone(ctx, store.ItemRef{Domain: "vms", Key: name}, []string{"vm:" + name, "p2"}, tg.ID, folders)
	}
	_, err = s.BackupVM(ctx, name)
	return err
}

// foldersHoldingFiles keeps the paths a restore would overwrite something at.
func foldersHoldingFiles(folders []string) []string {
	var held []string
	for _, p := range folders {
		if !paths.EmptyOrMissing(p) {
			held = append(held, p)
		}
	}
	return held
}

// saveFoldersOfGone backs up the folders of an entry that is gone from the host
// under the entry's own tags, as the backup taken before its restore.
func (s *Service) saveFoldersOfGone(ctx context.Context, item store.ItemRef, identity []string, targetID string, folders []string) error {
	held := foldersHoldingFiles(folders)
	if len(held) == 0 {
		return nil
	}
	if err := s.refuseLinkOutOfMount(held); err != nil {
		return err
	}
	settings, err := s.store.GetSettings()
	if err != nil {
		return fmt.Errorf("read settings: %w", err)
	}
	step, err := s.prepareHome(ctx, settings, item)
	if err != nil {
		return err
	}
	if step, err = s.recordHome(ctx, settings, item, step); err != nil {
		return err
	}
	unlock := s.lockDomainFor(item.Domain, "backup")
	defer unlock()
	runs := runsAdapter{st: s.store, ctx: ctx}
	runID, err := runs.Start(targetID, "backup")
	if err != nil {
		return fmt.Errorf("record run start: %w", err)
	}
	tags := withTags(identity, withTags(s.directTags(settings, item.Domain, step.repo), beforeRestoreTags(ctx)))
	sum, err := step.engine.Backup(ctx, step.repo, held, tags, step.mode)
	if err != nil {
		if fErr := runs.Finish(runID, "failed", backup.Summary{}, truncateRunErr(err)); fErr != nil {
			log.Printf("api: backup before restore: record run finish: %v", fErr)
		}
		return err
	}
	if fErr := runs.Finish(runID, "success", backupSummaryFrom(sum), ""); fErr != nil {
		log.Printf("api: backup before restore: record run finish: %v", fErr)
	}
	return nil
}

// saveFileSetState is saveContainerState for a file set.
func (s *Service) saveFileSetState(ctx context.Context, id string) error {
	if why, err := s.fileSetNothingToSave(id); err != nil || why != "" {
		return err
	}
	_, err := s.BackupFileSet(ctx, id)
	return err
}

// containerNothingToSave says why a container has no state a backup could
// save: reasonGone when it is not on the host, reasonEmpty when the data its
// last backup took is gone from disk, which the backup refuses. It is empty
// when there is state to save.
func (s *Service) containerNothingToSave(ctx context.Context, name string) (string, error) {
	in, found, err := s.liveContainer(ctx, name)
	switch {
	case err != nil:
		return "", err
	case !found:
		return reasonGone, nil
	case s.emptyBackupRefusal(name, in, s.effectiveBackupPaths(name, in)) != nil:
		return reasonEmpty, nil
	}
	return "", nil
}

// fileSetNothingToSave is containerNothingToSave for a file set, which is gone
// with its folder and empty when the folder holds no file.
func (s *Service) fileSetNothingToSave(id string) (string, error) {
	set, err := s.store.GetFileSet(id)
	if err != nil {
		return "", errFileSetNotFound
	}
	if strings.TrimSpace(set.Path) == "" {
		return reasonGone, nil
	}
	src, err := paths.Resolve(s.cfg.HostMountRoot, set.Path)
	if err != nil {
		return reasonGone, nil
	}
	if _, err := os.Stat(src); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return reasonGone, nil
		}
		return "", err
	}
	if empty, err := fileSetSourceEmpty(src); err == nil && empty {
		return reasonEmpty, nil
	}
	return "", nil
}

// BackupFirstPlan says what becomes of the backup a restore in place is asked
// to take first. Status is "ok" when it will be taken, "skip" with the reason
// "gone" when the entry is not on the host or "empty" when it has no data a
// backup could take, and "fail" with Detail when it cannot be taken, which
// keeps the restore from starting. Seconds and Bytes are what the entry's
// last backup took and added, both zero for an entry that was never backed up
// from here.
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
	var why, targetID string
	var err error
	switch req.Kind {
	case checkContainer, checkContainerFiles:
		if self := s.selfContainerName(ctx); self != "" && self == req.Name {
			return &BackupFirstPlan{Status: lineFail, Detail: ErrSelfBackup.Error()}
		}
		why, err = s.containerNothingToSave(ctx, req.Name)
		if tg, tErr := s.store.GetTargetByContainer(req.Name); tErr == nil {
			targetID = tg.ID
		}
	case checkVM:
		var state string
		state, err = s.virsh.State(ctx, req.Name)
		if state == "" {
			why = reasonGone
		}
		if tg, tErr := s.store.GetVMTargetByName(req.Name); tErr == nil {
			targetID = tg.ID
		}
	case checkFileSet:
		why, err = s.fileSetNothingToSave(req.Name)
		targetID = req.Name
	default:
		return nil
	}
	if why == reasonGone && req.Kind != checkFileSet && len(foldersHoldingFiles(sc.written)) > 0 {
		why = ""
	}
	switch {
	case err != nil:
		return &BackupFirstPlan{Status: lineFail, Detail: scrubError(err)}
	case why != "":
		return &BackupFirstPlan{Status: lineSkip, Reason: why}
	}
	plan := &BackupFirstPlan{Status: lineOK}
	if last, lErr := s.store.LastSuccessfulBackup(targetID); lErr == nil && last != nil && last.FinishedAt != nil {
		plan.Seconds, plan.Bytes = *last.FinishedAt-last.StartedAt, last.Bytes
	}
	return plan
}
