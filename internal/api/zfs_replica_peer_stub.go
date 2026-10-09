package api

import (
	"context"

	"github.com/junkerderprovinz/bombvault/internal/store"
	"github.com/junkerderprovinz/bombvault/internal/zfsrepl"
)

// zfsReplicaPeerEnd is the receiving end of a paired instance for one item.
// Its targets are <server folder>/<pool>/<path>, which the receiving side
// places under the root it chose.
func (s *Service) zfsReplicaPeerEnd(_ context.Context, item store.ZFSDataset) (zfsrepl.End, error) {
	return nil, zfsRefuse("peer-not-ready", item.Replica.TargetID)
}

// zfsReplicaPeerRequest runs after a PATCH that set or kept a peer target or
// changed its keep, with the item as stored now. It asks the paired instance
// again where the answer allows that, and passes a new keep on while the
// request is still asked.
func (s *Service) zfsReplicaPeerRequest(_ context.Context, _ store.ZFSDataset) error {
	return nil
}

// zfsReplicaPeerState is what the paired instance answered for the item.
func (s *Service) zfsReplicaPeerState(_ store.ZFSDataset) string {
	return ""
}
