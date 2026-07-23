package api

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// writeCacheSubdir creates a fake per-repo cache subdir of the given size whose
// newest file mtime is `used` — the LRU signal trimCacheDirLRU sorts on.
func writeCacheSubdir(t *testing.T, base, name string, size int, used time.Time) string {
	t.Helper()
	dir := filepath.Join(base, name)
	if err := os.MkdirAll(filepath.Join(dir, "data"), 0o750); err != nil {
		t.Fatal(err)
	}
	f := filepath.Join(dir, "data", "pack")
	if err := os.WriteFile(f, make([]byte, size), 0o600); err != nil {
		t.Fatal(err)
	}
	// Pin every path's mtime (file AND dirs) so the walk's newest-mtime is `used`
	// regardless of filesystem timestamp granularity.
	for _, p := range []string{f, filepath.Join(dir, "data"), dir} {
		if err := os.Chtimes(p, used, used); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// TestTrimCacheDirLRUEvictsOldestFirst pins the eviction policy: subdirs go
// least-recently-used first, only until the total fits the limit, and the most
// recently used subdir survives even when it alone exceeds the limit (it most
// likely belongs to a currently-running or just-finished op).
func TestTrimCacheDirLRUEvictsOldestFirst(t *testing.T) {
	base := t.TempDir()
	now := time.Now()
	oldest := writeCacheSubdir(t, base, "repo-oldest", 4096, now.Add(-72*time.Hour))
	middle := writeCacheSubdir(t, base, "repo-middle", 4096, now.Add(-48*time.Hour))
	newest := writeCacheSubdir(t, base, "repo-newest", 4096, now.Add(-1*time.Hour))

	// Limit fits two subdirs: only the oldest must be evicted.
	trimCacheDirLRU(base, 9000)

	if _, err := os.Stat(oldest); !os.IsNotExist(err) {
		t.Fatalf("oldest subdir should have been evicted, stat err = %v", err)
	}
	for _, keep := range []string{middle, newest} {
		if _, err := os.Stat(keep); err != nil {
			t.Fatalf("%s should have survived: %v", filepath.Base(keep), err)
		}
	}

	// Limit smaller than any single subdir: everything except the most recently
	// used one is evicted — the hottest cache is never deleted.
	trimCacheDirLRU(base, 1024)
	if _, err := os.Stat(middle); !os.IsNotExist(err) {
		t.Fatalf("middle subdir should have been evicted, stat err = %v", err)
	}
	if _, err := os.Stat(newest); err != nil {
		t.Fatalf("most recently used subdir must never be evicted: %v", err)
	}
}

// TestTrimCacheDirLRUUnderLimitIsNoOp ensures a cache within its budget is left
// completely untouched.
func TestTrimCacheDirLRUUnderLimitIsNoOp(t *testing.T) {
	base := t.TempDir()
	a := writeCacheSubdir(t, base, "repo-a", 1024, time.Now().Add(-24*time.Hour))
	b := writeCacheSubdir(t, base, "repo-b", 1024, time.Now())

	trimCacheDirLRU(base, 1<<20)

	for _, keep := range []string{a, b} {
		if _, err := os.Stat(keep); err != nil {
			t.Fatalf("%s should have survived: %v", filepath.Base(keep), err)
		}
	}
}
