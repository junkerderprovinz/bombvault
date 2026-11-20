package api

import (
	"context"

	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/restickey"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// nodeRepoID is the id of the received repository home watches a section of
// node id's backups in.
func nodeRepoID(id, domain string) string {
	return "node-" + id[:12] + "-" + domain
}

// pruneNodeSection ages the watched repository of section domain of n and
// prunes it. It ages by the keep rule of the storage location it lies at, or
// by home's own rule for the section, and only the snapshots under the node's
// own host, so whatever else lies there stays. The rest-server of a node that
// keeps its own backups takes no deletes, so those are left alone.
func (s *Service) pruneNodeSection(ctx context.Context, n store.Node, domain string) error {
	if n.Placement == nodePlaceNode {
		return nil
	}
	rr, found, err := s.store.GetReceivedRepo(nodeRepoID(n.ID, domain))
	if err != nil || !found || !rr.Enabled {
		return err
	}
	settings, err := s.store.GetSettings()
	if err != nil {
		return err
	}
	p := keepPolicy(namedRepoKeep(settings, domain))
	if n.Placement == nodePlaceLocation {
		if d, found, err := s.store.GetDestination(n.PlacementTarget); err == nil && found && keepPolicy(targetKeep(d)).Any() {
			p = keepPolicy(targetKeep(d))
		}
	}
	// Everything in the repository is the node's own, so a direct snapshot of
	// it ages with the rest.
	p.Direct = true
	if !p.Any() {
		return nil
	}
	appKey, err := s.nodeAppKey(n)
	if err != nil {
		return err
	}
	return s.pruneNodeRepo(ctx, rr, restickey.Host(appKey), p)
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
