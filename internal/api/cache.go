package api

import (
	"context"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// SetResticCacheDir sets the directory exported to restic as RESTIC_CACHE_DIR.
// An empty dir means restic uses its default location, and only restic's own
// `cache --cleanup` runs.
func (s *Service) SetResticCacheDir(dir string) { s.resticCacheDir = dir }

// TrimResticCache bounds restic's persistent cache, which lives under /config
// and grows by one subdirectory for every repository ever opened. It runs after
// each scheduled domain run, in two passes whose errors are only logged, since
// a cache trim must not fail a backup:
//
//  1. `restic cache --cleanup` removes per-repo caches unused for 30 days.
//  2. When Settings.ResticCacheMaxMB > 0, the least recently used per-repo
//     subdirectories are evicted until the total fits. The most recently used
//     one is always kept, as it most likely belongs to a running operation.
func (s *Service) TrimResticCache(ctx context.Context) {
	// A second caller is turned away rather than queued. The after-bulk hook
	// fires for every per-item cron entry, so several trims can start in the
	// same minute, and each would measure the whole cache and evict on its own.
	// A queued trim would act on a measurement the previous one made stale.
	if !s.cacheTrimming.CompareAndSwap(false, true) {
		return
	}
	defer s.cacheTrimming.Store(false)
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	if err := s.engine.CacheCleanup(ctx); err != nil {
		log.Printf("api: restic cache cleanup failed (continuing): %v", err)
	}
	settings, err := s.store.GetSettings()
	if err != nil {
		log.Printf("api: cache trim: read settings: %v", err)
		return
	}
	if settings.ResticCacheMaxMB <= 0 || s.resticCacheDir == "" {
		return
	}
	trimCacheDirLRU(s.resticCacheDir, int64(settings.ResticCacheMaxMB)*1024*1024)
}

// cacheSubdir is the cache of one repository. lastUsed is the newest mtime
// inside it: restic touches the files it uses, while the directory's own mtime
// only changes when entries are added or removed at its top level.
type cacheSubdir struct {
	path     string
	size     int64
	lastUsed time.Time
}

// trimCacheDirLRU evicts the least recently used subdirectories of dir until
// their total size fits limitBytes, always keeping the most recently used one.
// Errors are logged and skipped.
func trimCacheDirLRU(dir string, limitBytes int64) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		log.Printf("api: cache trim: read cache dir: %v", err)
		return
	}
	var subs []cacheSubdir
	var total int64
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		sub := measureDir(filepath.Join(dir, e.Name()))
		subs = append(subs, sub)
		total += sub.size
	}
	if total <= limitBytes {
		return
	}
	// Oldest first; the loop stops before the last, most recently used entry.
	sort.Slice(subs, func(i, j int) bool { return subs[i].lastUsed.Before(subs[j].lastUsed) })
	for i := 0; i < len(subs)-1 && total > limitBytes; i++ {
		if err := os.RemoveAll(subs[i].path); err != nil {
			log.Printf("api: cache trim: evict %s: %v", filepath.Base(subs[i].path), err)
			continue
		}
		total -= subs[i].size
		log.Printf("api: cache trim: evicted repo cache %s (%d MB, last used %s) because the cache was over the limit",
			filepath.Base(subs[i].path), subs[i].size/(1024*1024), subs[i].lastUsed.Format("2006-01-02"))
	}
	if total > limitBytes {
		log.Printf("api: cache trim: still %d MB over the limit; only the most recently used repo cache remains, and it is never evicted",
			(total-limitBytes)/(1024*1024)+1)
	}
}

// measureDir returns the total file size under root and the newest mtime,
// starting from root's own. Walk errors are skipped, since files can disappear
// mid-walk.
func measureDir(root string) cacheSubdir {
	sub := cacheSubdir{path: root}
	if info, err := os.Stat(root); err == nil {
		sub.lastUsed = info.ModTime()
	}
	_ = filepath.WalkDir(root, func(_ string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil //nolint:nilerr // best-effort: skip unreadable entries, keep walking
		}
		info, iErr := d.Info()
		if iErr != nil {
			return nil //nolint:nilerr // best-effort: entry vanished mid-walk
		}
		sub.size += info.Size()
		if info.ModTime().After(sub.lastUsed) {
			sub.lastUsed = info.ModTime()
		}
		return nil
	})
	return sub
}
