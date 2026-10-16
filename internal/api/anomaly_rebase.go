package api

import (
	"log"
	"path"
	"slices"
	"strings"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/paths"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// rebaseAnomaliesFor records that BombVault itself just changed the folders at
// hostPaths, so the series that back them up start learning again at their
// next backup instead of reading the change as data loss or growth. also are
// series known to hold the folders beyond those found by path.
func (s *Service) rebaseAnomaliesFor(hostPaths []string, also ...store.AnomalyScopeKey) {
	scopes, err := s.scopesHolding(hostPaths)
	if err != nil {
		log.Printf("api: find the backups that hold %s: %v", strings.Join(hostPaths, ", "), err) //nolint:gosec // G706: paths this process built
	}
	scopes = append(scopes, also...)
	slices.SortFunc(scopes, func(a, b store.AnomalyScopeKey) int {
		return strings.Compare(a.Kind+":"+a.ID, b.Kind+":"+b.ID)
	})
	if err := s.store.RebaseAnomalyScopes(slices.Compact(scopes), time.Now().Unix()); err != nil {
		log.Printf("api: record the change of %s for the anomaly rules: %v", strings.Join(hostPaths, ", "), err) //nolint:gosec // G706: paths this process built
	}
}

// scopesHolding finds the series whose backups take the given host paths
// along: the ZFS dataset each path lies in, the datasets mounted inside it,
// and the folder sets that reach it.
func (s *Service) scopesHolding(hostPaths []string) ([]store.AnomalyScopeKey, error) {
	mounts, err := s.store.ZFSMountpoints()
	if err != nil {
		return nil, err
	}
	sets, err := s.store.ListFileSets()
	if err != nil {
		return nil, err
	}
	var out []store.AnomalyScopeKey
	for _, p := range hostPaths {
		holder, depth := "", -1
		for dataset, mountpoint := range mounts {
			switch {
			case pathWithin(p, mountpoint):
				if len(mountpoint) > depth {
					holder, depth = dataset, len(mountpoint)
				}
			case pathWithin(mountpoint, p):
				out = append(out, store.AnomalyScopeKey{Kind: anomalyScopeZFSDS, ID: dataset})
			}
		}
		if holder != "" {
			out = append(out, store.AnomalyScopeKey{Kind: anomalyScopeZFSDS, ID: holder})
		}
		for _, set := range sets {
			src, rErr := paths.Resolve(s.cfg.HostMountRoot, set.Path)
			if rErr != nil {
				continue
			}
			if root := s.toHostPath(src); pathWithin(p, root) || pathWithin(root, p) {
				out = append(out, store.AnomalyScopeKey{Kind: anomalyScopeItem, ID: set.ID})
			}
		}
	}
	return out, nil
}

// pathWithin reports whether p is root or lies below it.
func pathWithin(p, root string) bool {
	p, root = path.Clean(p), path.Clean(root)
	return p == root || root == "/" || strings.HasPrefix(p, root+"/")
}
