package api

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/junkerderprovinz/bombvault/internal/restic"
)

// snapshotListCache holds, per local repository, the last listing restic gave
// together with the file names its snapshots directory had when it was asked.
type snapshotListCache struct {
	mu      sync.Mutex
	entries map[string]cachedSnapshotList
}

type cachedSnapshotList struct {
	access string
	files  string
	snaps  []restic.Snapshot
}

func (c *snapshotListCache) get(repo, access, files string) ([]restic.Snapshot, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.entries[repo]
	if !ok || e.access != access || e.files != files {
		return nil, false
	}
	return e.snaps, true
}

func (c *snapshotListCache) put(repo, access, files string, snaps []restic.Snapshot) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.entries == nil {
		c.entries = map[string]cachedSnapshotList{}
	}
	c.entries[repo] = cachedSnapshotList{access: access, files: files, snaps: snaps}
}

// cachedSnapshots is listSnapshots for the list pages. Every restic call spends
// about half a second deriving the repository key, and the pages ask on every
// load. restic names a snapshot file after its content and never rewrites one,
// so while the snapshots directory holds the same names the last listing is
// still true. The names are read before restic runs, so a backup that lands
// during the listing makes the next call ask again.
func (s *Service) cachedSnapshots(ctx context.Context, repo string, mode restic.Mode) ([]restic.Snapshot, error) {
	files, err := snapshotFileNames(repo)
	if err != nil {
		return s.listSnapshots(ctx, repo, mode)
	}
	access := repoAccessKey(mode)
	if snaps, ok := s.snapLists.get(repo, access, files); ok {
		return snaps, nil
	}
	snaps, err := s.listSnapshots(ctx, repo, mode)
	if err == nil {
		s.snapLists.put(repo, access, files, snaps)
	}
	return snaps, err
}

func snapshotFileNames(repo string) (string, error) {
	if restic.IsRemoteRepo(repo) {
		return "", os.ErrNotExist
	}
	entries, err := os.ReadDir(filepath.Join(repo, "snapshots"))
	if err != nil {
		return "", err
	}
	names := make([]string, len(entries))
	for i, e := range entries {
		names[i] = e.Name()
	}
	return strings.Join(names, "\n"), nil
}

// repoAccessKey tells the credentials a listing was made with apart without
// keeping them, so a changed password asks restic again rather than serving a
// listing the new one might not open.
func repoAccessKey(mode restic.Mode) string {
	h := sha256.New()
	if mode.Encrypted {
		h.Write([]byte{1})
		h.Write([]byte(mode.Password))
	}
	for _, e := range mode.Env {
		h.Write([]byte{0})
		h.Write([]byte(e))
	}
	return hex.EncodeToString(h.Sum(nil))
}
