package api

import (
	"context"
	"errors"
	"fmt"

	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/restickey"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// nodeRepoID is the id of the received repository home watches a section of
// node id's backups in.
func nodeRepoID(id, domain string) string {
	return "node-" + id[:12] + "-" + domain
}

// pruneNodeRepos ages each watched repository of n by home's rules for its
// section and prunes it. Only the snapshots under the node's own host are
// aged, so whatever else lies there stays. The rest-server of a node that
// keeps its own backups takes no deletes, so those are left alone.
func (s *Service) pruneNodeRepos(ctx context.Context, n store.Node) error {
	if n.Placement == nodePlaceNode {
		return nil
	}
	appKey, err := s.nodeAppKey(n)
	if err != nil {
		return err
	}
	settings, err := s.store.GetSettings()
	if err != nil {
		return err
	}
	var errs []error
	for _, d := range nodeDomains {
		rr, found, err := s.store.GetReceivedRepo(nodeRepoID(n.ID, d))
		if err != nil {
			errs = append(errs, err)
			continue
		}
		if !found || !rr.Enabled {
			continue
		}
		p := keepPolicy(namedRepoKeep(settings, d))
		// Everything in the repository is the node's own, so a direct
		// snapshot of it ages with the rest.
		p.Direct = true
		if !p.Any() {
			continue
		}
		if err := s.pruneNodeRepo(ctx, rr, restickey.Host(appKey), p); err != nil {
			errs = append(errs, fmt.Errorf("the %s: %w", d, err))
		}
	}
	return errors.Join(errs...)
}

func (s *Service) pruneNodeRepo(ctx context.Context, rr store.ReceivedRepo, host string, p restic.RetentionPolicy) error {
	repo, mode, err := s.receiverOpen(ctx, rr)
	if err != nil {
		return err
	}
	mode.NoLock = false
	mode.Host, mode.Sharing = host, restic.SharingOwner
	snaps, err := s.engine.Snapshots(ctx, repo, mode)
	if err != nil {
		return err
	}
	for _, tag := range identityTags(snaps) {
		if err := s.forgetWithLockHeal(ctx, repo, p, mode, []string{tag}, false); err != nil {
			return err
		}
	}
	return s.retryAfterUnlock(ctx, repo, mode, func() error { return s.engine.Prune(ctx, repo, mode) })
}
