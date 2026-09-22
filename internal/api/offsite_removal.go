package api

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// itemAtTarget is one item's snapshots at one off-site target and what it takes
// to delete them there.
type itemAtTarget struct {
	Target store.OffsiteTarget
	Repo   string
	Mode   restic.Mode
	Snaps  []restic.Snapshot
}

// listRepo lists a repository the way snapshotsForTag does, without its tag
// filter: a local repository never created is empty, one on a share that is not
// mounted is an error.
func (s *Service) listRepo(ctx context.Context, repo string, mode restic.Mode) ([]restic.Snapshot, error) {
	if localRepoMissing(repo) {
		if s.repoEstablished(repo) && !s.destinationMounted(repo) {
			return nil, ErrBackupPathNotMounted
		}
		return nil, nil
	}
	return s.listSnapshots(ctx, repo, mode)
}

// listItemAtTarget lists what identity owns at the target the source names. A
// snapshot whose owner the listing cannot settle stays out.
func (s *Service) listItemAtTarget(ctx context.Context, settings store.Settings, domain, identity, source string) (itemAtTarget, error) {
	target, err := s.offsiteTargetForSource(settings, domain, source)
	if err != nil {
		return itemAtTarget{}, err
	}
	repo, err := s.resolveRepo(target.Repo)
	if err != nil {
		return itemAtTarget{}, err
	}
	at := itemAtTarget{Target: target, Repo: repo, Mode: s.offsiteModeForTarget(settings, target)}
	all, err := s.listRepo(ctx, repo, at.Mode)
	if err != nil {
		return itemAtTarget{}, err
	}
	oc, err := s.ownerContextFor(domain)
	if err != nil {
		return itemAtTarget{}, err
	}
	owners := oc.owners(all)
	for _, snap := range all {
		if owners[snap.ID].Owner == identity {
			at.Snaps = append(at.Snaps, snap)
		}
	}
	return at, nil
}

// refuseAppendOnlyTarget asks whether the target a source names is append-only,
// switched off or not.
func (s *Service) refuseAppendOnlyTarget(settings store.Settings, domain, source string) error {
	immutable, err := s.offsiteSourceImmutable(settings, domain, source)
	if err != nil {
		return err
	}
	if immutable {
		return errAppendOnlyOffsiteTarget
	}
	return nil
}

// forgetAtTarget deletes every snapshot identity owns at one off-site target and
// prunes there. Append-only is asked before the domain lock and again inside it,
// after the listing, so a flag switched on in between still refuses; check sees
// that listing and can refuse before anything is deleted.
func (s *Service) forgetAtTarget(ctx context.Context, domain, identity, source string, check func(store.Settings, itemAtTarget) error) (int, error) {
	settings, err := s.store.GetSettings()
	if err != nil {
		return 0, fmt.Errorf("read settings: %w", err)
	}
	if err := s.refuseAppendOnlyTarget(settings, domain, source); err != nil {
		return 0, err
	}
	unlock, ok := s.tryLockDomainFor(domain, "delete")
	if !ok {
		return 0, errDomainBusy
	}
	defer unlock()

	at, err := s.listItemAtTarget(ctx, settings, domain, identity, source)
	if err != nil {
		return 0, err
	}
	if settings, err = s.store.GetSettings(); err != nil {
		return 0, fmt.Errorf("read settings: %w", err)
	}
	if err := s.refuseAppendOnlyTarget(settings, domain, source); err != nil {
		return 0, err
	}
	if check != nil {
		if err := check(settings, at); err != nil {
			return 0, err
		}
	}
	if len(at.Snaps) > 0 {
		ids := make([]string, 0, len(at.Snaps))
		for _, snap := range at.Snaps {
			ids = append(ids, snap.ID)
		}
		s.unlockStale(ctx, at.Repo, at.Mode)
		if err := s.engine.Forget(ctx, at.Repo, ids, true, at.Mode); err != nil {
			return 0, fmt.Errorf("forget snapshots: %w", err)
		}
	}
	// The snapshots are gone by this point and the next listing of the target
	// writes the row again, so a failure here must not report a delete that
	// happened as failed.
	if at.Target.ID != "" {
		gone := []store.ItemCopies{{Domain: domain, Identity: identity, TargetID: at.Target.ID}}
		if err := s.store.AdjustItemCopies(domain, at.Target.ID, time.Now().Unix(), gone); err != nil {
			log.Printf("api: delete at target: observed copies of %q stay until the next listing: %v", identity, err) //nolint:gosec // G706: identity is %q-quoted
		}
	}
	return len(at.Snaps), nil
}
