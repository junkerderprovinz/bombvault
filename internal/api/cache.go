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

// SetResticCacheDir tells the service where restic's persistent cache lives on
// disk — the same path main.go exports to the engine as RESTIC_CACHE_DIR — so
// TrimResticCache can measure and evict per-repo cache subdirectories. An empty
// dir (the mkdir-failed fallback, where restic uses its default location)
// disables the size-based trim; restic's own `cache --cleanup` still runs.
func (s *Service) SetResticCacheDir(dir string) { s.resticCacheDir = dir }

// TrimResticCache bounds restic's persistent cache. The cache moved under
// /config (RESTIC_CACHE_DIR, v6.7.0) so it survives container restarts — which
// also means it now grows unbounded, one subdirectory per repository ever
// opened (local, off-site, foreign). Called at the end of each scheduled domain
// run (after the batched off-site replication — see main.go's after-bulk hook).
// Two passes, both best-effort (errors are logged, never propagated — a cache
// trim must never fail a backup):
//
//  1. `restic cache --cleanup` — restic's own janitor removes per-repo cache
//     directories not used for over 30 days (its --max-age default), e.g. the
//     cache of a repo location that was reconfigured away.
//  2. When Settings.ResticCacheMaxMB > 0, the first-level subdirectories (one
//     per repo) are measured and the least-recently-used ones (newest file
//     mtime inside each) are evicted until the total fits the limit. The most
//     recently used subdirectory is never evicted — it is the one most likely
//     to belong to a currently-running or just-finished operation, and evicting
//     the hottest cache would defeat the point of persisting it.
func (s *Service) TrimResticCache(ctx context.Context) {
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
		return // no size limit configured, or cache at restic's default (unmanaged) location
	}
	trimCacheDirLRU(s.resticCacheDir, int64(settings.ResticCacheMaxMB)*1024*1024)
}

// cacheSubdir is one first-level subdirectory of the restic cache base dir
// (one per repository), with its total size and the newest mtime found inside
// (the best available "last used" signal — restic touches pack/index files in
// the subdir it is using, while the subdir's own mtime only changes when
// entries are added/removed at its top level).
type cacheSubdir struct {
	path     string
	size     int64
	lastUsed time.Time
}

// trimCacheDirLRU evicts least-recently-used first-level subdirectories of dir
// until their total size fits limitBytes. The most recently used subdirectory
// is always kept (see TrimResticCache). Best-effort: every error is logged and
// skipped, never returned.
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
			continue // stray file (e.g. restic's CACHEDIR.TAG lives inside subdirs, not here)
		}
		sub := measureDir(filepath.Join(dir, e.Name()))
		subs = append(subs, sub)
		total += sub.size
	}
	if total <= limitBytes {
		return
	}
	// LRU first; the last element (most recently used) is never evicted.
	sort.Slice(subs, func(i, j int) bool { return subs[i].lastUsed.Before(subs[j].lastUsed) })
	for i := 0; i < len(subs)-1 && total > limitBytes; i++ {
		if err := os.RemoveAll(subs[i].path); err != nil {
			log.Printf("api: cache trim: evict %s: %v", filepath.Base(subs[i].path), err)
			continue
		}
		total -= subs[i].size
		log.Printf("api: cache trim: evicted repo cache %s (%d MB, last used %s) — cache was over the limit",
			filepath.Base(subs[i].path), subs[i].size/(1024*1024), subs[i].lastUsed.Format("2006-01-02"))
	}
	if total > limitBytes {
		log.Printf("api: cache trim: still %d MB over the limit — only the most recently used repo cache remains (never evicted)",
			(total-limitBytes)/(1024*1024)+1)
	}
}

// measureDir walks root once, accumulating the total file size and the newest
// file mtime (falling back to the root's own mtime when the walk yields none).
// Walk errors are skipped — a file disappearing mid-walk must not abort the trim.
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
