package api

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/junkerderprovinz/bombvault/internal/backup"
	"github.com/junkerderprovinz/bombvault/internal/paths"
)

// foreignExtractTarget is the run target of a container's or VM's files
// restored out of a foreign repository into a folder. No target of this
// instance stands for them, so the run carries the item as its subject.
func foreignExtractTarget(domain string) string { return "foreign:" + domain }

// foreignExtractTag is the tag a foreign container's or VM's snapshots carry.
func foreignExtractTag(domain, item string) (string, error) {
	switch domain {
	case "containers":
		return "container:" + item, nil
	case "vms":
		return "vm:" + item, nil
	default:
		return "", errors.New("only a container or a VM is restored into a folder this way")
	}
}

// prepareForeignExtract resolves the restore of a foreign container's appdata
// or VM's disks into a folder, as plain files. It is the way to the data of an
// item whose definition the session cannot read, which cannot be recreated.
func (s *Service) prepareForeignExtract(ctx context.Context, sess foreignSession, domain, item, snapshotID, targetSubPath string) (fileSetRestorePlan, error) {
	tag, err := foreignExtractTag(domain, item)
	if err != nil {
		return fileSetRestorePlan{}, err
	}
	sub := strings.TrimSpace(targetSubPath)
	if sub == "" {
		return fileSetRestorePlan{}, errors.New("a target folder is required to restore files from a foreign repository")
	}
	target, err := paths.ResolveForWrite(s.cfg.HostMountRoot, sub)
	if err != nil {
		return fileSetRestorePlan{}, errors.New("invalid target folder: must be a relative subpath under the host mount")
	}
	if err := s.guardRestoreFolder(target); err != nil {
		return fileSetRestorePlan{}, err
	}
	explicitID := snapshotID != "latest" && snapshotID != ""
	if explicitID && !backup.ValidSnapshotID(snapshotID) {
		return fileSetRestorePlan{}, backup.ErrInvalidSnapshotID
	}
	snaps, err := s.snapshotsForTag(ctx, sess.repo, sess.mode, tag)
	if err != nil {
		return fileSetRestorePlan{}, err
	}
	switch {
	case explicitID && !snapshotBelongs(snaps, snapshotID):
		return fileSetRestorePlan{}, fmt.Errorf("snapshot %s does not belong to %s", snapshotID, item)
	case !explicitID && len(snaps) == 0:
		return fileSetRestorePlan{}, fmt.Errorf("no backups found for %s", item)
	case !explicitID:
		snapshotID = snaps[len(snaps)-1].ID
	}
	plan := fileSetRestorePlan{
		repo:       sess.repo,
		mode:       sess.mode,
		snapshotID: snapshotID,
		setID:      foreignExtractTarget(domain),
		setName:    item,
		target:     target,
		subtree:    snapshotRestoreRoot(snaps, snapshotID),
	}
	if err := s.markFileSubtree(ctx, &plan); err != nil {
		return fileSetRestorePlan{}, err
	}
	if !isPlanOnly(ctx) {
		if err := paths.EnsureDirReadable(target); err != nil {
			return fileSetRestorePlan{}, fmt.Errorf("create target folder: %w", err)
		}
	}
	return plan, nil
}

// StartForeignExtract restores a foreign container's or VM's snapshot into a
// folder in the background, the way StartForeignRestore restores a file set.
func (s *Service) StartForeignExtract(ctx context.Context, sessionID, domain, item, snapshotID string, confirm bool, targetSubPath string) (bool, error) {
	if !confirm {
		return false, backup.ErrNotConfirmed
	}
	if !foreignItemNameOK(domain, item) {
		return false, errors.New("invalid item name")
	}
	if !s.batchActive.CompareAndSwap(false, true) {
		return false, nil
	}
	sess, err := s.foreignSession(sessionID)
	if err != nil {
		s.batchActive.Store(false)
		return false, err
	}
	plan, err := s.prepareForeignExtract(ctx, sess, domain, item, snapshotID, targetSubPath)
	if err != nil {
		s.batchActive.Store(false)
		return false, err
	}
	key := "extract:" + domain + ":" + item
	var runID string
	run := func(rctx context.Context) error {
		id, err := s.startRun(withRunSubject(rctx, item), plan.setID, "restore")
		if err != nil {
			log.Printf("api: foreign extract: record the run of %q: %v", item, err) //nolint:gosec // G706: item is %q-quoted
		}
		runID = id
		pctx, startedAt := s.progBegin(rctx, key, "restore")
		return s.concludeFileSetRestore(runID, key, plan.snapshotID, s.extractForeign(pctx, plan), startedAt)
	}
	onPanic := func(msg string) { s.finishRestoreRun(runID, "", errors.New(msg)) }
	s.launchForeignRestore(ctx, domain, item, key, run, onPanic)
	return true, nil
}

// extractForeign writes a plan's snapshot into its folder. The repository
// belongs to another instance, so no domain of this one is locked for it.
func (s *Service) extractForeign(ctx context.Context, plan fileSetRestorePlan) error {
	if plan.subtree != "" {
		return s.engine.RestoreSubtreeTo(ctx, plan.repo, plan.snapshotID, plan.subtree, plan.subtreeTarget(), plan.mode)
	}
	return s.engine.RestoreInclude(ctx, plan.repo, plan.snapshotID, "/", plan.target, plan.mode)
}

// foreignExtractScope prepares the same restore for the restore check.
func (s *Service) foreignExtractScope(ctx context.Context, sess foreignSession, req RestoreCheckRequest) (restoreScope, error) {
	plan, err := s.prepareForeignExtract(ctx, sess, req.Domain, req.Name, req.SnapshotID, req.TargetPath)
	if err != nil {
		return restoreScope{}, err
	}
	return fileSetScope(plan), nil
}
