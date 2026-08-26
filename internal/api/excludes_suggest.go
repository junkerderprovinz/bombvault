package api

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/model"
	"github.com/junkerderprovinz/bombvault/internal/restic"
)

// ---------------------------------------------------------------------------
// Exclusion assistant — scans a container's backed-up folders for well-known
// junk directories (caches, temp, logs, transcodes, …) and unusually large
// directories, so the UI can offer one-click restic excludes that shrink the
// backup. Read-only: nothing here persists anything — the user picks, and the
// existing SetExcludes/PATCH path stores the chosen lines.
//
// Two feeders, ONE collector (issue #175)
// ---------------------------------------
// The sizes shown must never be a fraction presented as a total. There is
// exactly one candidate collector (suggestCollector); it is fed either by
//
//	snapshot — the newest local `container:<name>` snapshot, aggregated per
//	  directory from the node sizes restic already recorded. Exact and
//	  all-or-nothing: a pass that misses its budget is DISCARDED, never emitted
//	  partially. This is the primary source, and it is also the better answer to
//	  the question the assistant is asked ("what is bloating my backup" — restic's
//	  recorded size IS what got backed up).
//
//	live — filepath.WalkDir, the only thing that can serve a container with no
//	  backup yet. A walk that runs out of time yields EXACT sizes for every
//	  subtree it finished and explicitly labelled lower bounds for the rest
//	  (see scanExcludeCandidates' lexical-DFS derivation).
//
// A single request never runs both: nginx's default proxy_read_timeout is 60s
// and sits between the browser and the app in the usual Unraid reverse-proxy
// setup, so 40s + 30s chained on one blocking GET would 504. When the snapshot
// pass fails, the response says so and the UI offers a live scan as a SECOND
// request (?source=live).
// ---------------------------------------------------------------------------

const (
	// suggestMaxDepth bounds how deep BELOW a backup root a directory may sit to
	// become a suggestion. Deeper junk still counts into its ancestors' sizes, so
	// a cache buried at depth 9 surfaces through its (suggested) parent.
	suggestMaxDepth = 4
	// suggestLargeBytes is the size at which ANY directory is suggested
	// regardless of name — big data is worth a look even when it isn't junk.
	suggestLargeBytes = 200 << 20 // 200 MiB
	// suggestMaxResults caps the response: the biggest wins, the tail is noise.
	suggestMaxResults = 20
	// suggestScanTimeout bounds the whole LIVE walk; on timeout the partial
	// result is returned with truncated=true, every candidate whose subtree did
	// not finish flagged as a lower bound rather than a total.
	suggestScanTimeout = 30 * time.Second
	// suggestSnapshotTimeout bounds the SNAPSHOT aggregate. Deliberately under
	// the 60s nginx proxy_read_timeout that sits in front of a typical Unraid
	// install: at the measured ~50k nodes/s this covers roughly 2M nodes, well
	// past a large Plex appdata tree, without ever 504-ing the browser.
	suggestSnapshotTimeout = 40 * time.Second
)

// suggestSourceSnapshot / suggestSourceLive name where a scan's sizes came
// from. The UI states this next to the list, because the two carry different
// promises: the snapshot's sizes are exact but AS OF the last backup, the live
// walk's are as of now but can be truncated.
const (
	suggestSourceSnapshot = "snapshot"
	suggestSourceLive     = "live"
)

// errSuggestIndexRead is returned when the snapshot pass could not be completed
// — the listing errored, or it outran suggestSnapshotTimeout. The aggregate is
// DISCARDED in that case: a partially consumed `ls` stream would reproduce the
// exact defect this whole change exists to remove (#175), so there is no code
// path that emits a partial snapshot list.
var errSuggestIndexRead = errors.New("could not finish reading the backup index")

// knownJunkDirNames are directory basenames (lowercased for the lookup) that
// are safe-to-skip regenerable data by convention: browser/app caches, temp
// dirs, logs, Python/Node build artifacts, and media-server transcode/thumbnail
// stores. Matched case-insensitively so "Cache", "cache" and "GPUCache" all hit.
var knownJunkDirNames = map[string]bool{
	"cache":         true,
	".cache":        true,
	"tmp":           true,
	"temp":          true,
	"logs":          true,
	"log":           true,
	"crash reports": true,
	"gpucache":      true,
	"code cache":    true,
	"shadercache":   true,
	"node_modules":  true,
	"__pycache__":   true,
	"transcodes":    true,
	"transcode":     true,
	"thumbnails":    true,
}

// ExcludeSuggestion is one exclusion-assistant candidate: a directory in this
// container's backup worth excluding. Path is relative to the backed-up folder
// (display), Line is the ready-to-store exclude line in the editor's vocabulary
// (the path as seen inside the target container when a mount covers it, else
// the scanned path verbatim), and Reason says why it surfaced.
//
// Complete says whether SizeBytes is the directory's real total or a LOWER
// BOUND: false means the scan stopped inside this subtree, so the UI must show
// the number as a minimum. It is always true on the snapshot source. Nothing
// downstream may present a Complete=false size as a total (#175: the reporter
// was shown "5.7 GB" for a folder that backs up 55 GB).
type ExcludeSuggestion struct {
	Path      string `json:"path"`
	Line      string `json:"line"`
	SizeBytes int64  `json:"sizeBytes"`
	Reason    string `json:"reason"` // "known-cache" | "large"
	Complete  bool   `json:"complete"`
}

// SuggestResult is one exclusion-assistant pass. Source names the feeder;
// SnapshotTime is the RFC3339 time of the snapshot the sizes came from (empty
// on the live source) and is REQUIRED next to the list — an as-of-last-backup
// size shown without its date would just be a different misleading number.
// StoppedAt names the folder the live walk stopped inside, so the truncation
// banner can say WHERE the list ends (a folder never visited has no row to
// carry a flag).
type SuggestResult struct {
	Suggestions  []ExcludeSuggestion
	Truncated    bool
	Source       string
	SnapshotTime string
	StoppedAt    string
}

// suggestCacheEntry is one container's cached snapshot aggregate. Key pins the
// (repo, snapshot id, resolved excludes) it was computed for; a newer snapshot
// or an edited exclude list misses and recomputes. Candidates (not finished
// suggestions) are cached, so a mount change still re-derives every Line.
type suggestCacheEntry struct {
	key          string
	snapshotTime string
	cands        []suggestCandidate
}

// suggestOpts are the scan bounds, injectable so tests can shrink them.
type suggestOpts struct {
	maxDepth   int
	largeBytes int64
}

// suggestCandidate is a directory the collector considered: full is the absolute
// slash-normalized path, rel its path below the walk root, size the recursive
// byte total, known whether its basename is a well-known junk name, complete
// whether size is that total or only a lower bound (see ExcludeSuggestion).
type suggestCandidate struct {
	full     string
	rel      string
	size     int64
	known    bool
	complete bool
}

// matchesExcludePatterns reports whether a directory is already covered by one
// of the container's RESOLVED exclude patterns (resolveExcludePatterns output):
// a pattern without a slash matches the basename at any depth (restic's bare-
// name rule, glob-aware via path.Match), an anchored pattern matches the
// directory itself or any parent prefix. Anything already excluded must be
// neither suggested nor counted — restic won't back it up.
func matchesExcludePatterns(full, base string, patterns []string) bool {
	for _, pat := range patterns {
		if pat == "" {
			continue
		}
		if !strings.Contains(pat, "/") {
			if ok, err := path.Match(pat, base); err == nil && ok {
				return true
			}
			continue
		}
		p := strings.TrimSuffix(pat, "/")
		if full == p || strings.HasPrefix(full, p+"/") {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// The collector — source-agnostic. Both feeders push (rel, isDir, size) nodes
// in here and NOTHING downstream of it knows which one ran.
// ---------------------------------------------------------------------------

type suggestCollector struct {
	rootSlash string
	patterns  []string
	opts      suggestOpts
	byRel     map[string]*suggestCandidate
	order     []string        // rel keys in feed order (parents before children)
	pruned    map[string]bool // rel of directories an exclude pattern covers
}

func newSuggestCollector(root string, patterns []string, o suggestOpts) *suggestCollector {
	return &suggestCollector{
		rootSlash: strings.TrimSuffix(filepath.ToSlash(root), "/"),
		patterns:  patterns,
		opts:      o,
		byRel:     map[string]*suggestCandidate{},
		pruned:    map[string]bool{},
	}
}

// relUnder returns p's path below root, and whether p is under root at all.
// The root itself yields ("", true) — it is never a suggestion.
func relUnder(rootSlash, p string) (string, bool) {
	if p == rootSlash {
		return "", true
	}
	if !strings.HasPrefix(p, rootSlash+"/") {
		return "", false
	}
	return p[len(rootSlash)+1:], true
}

// add feeds one node. rel is the path below the root, isDir marks a directory,
// size is a FILE's byte size (ignored for directories). It reports whether the
// node was accepted; false means an exclude pattern covers it and a walking
// caller should skip the whole subtree. Callers that cannot prune (the snapshot
// stream, which just replays nodes) need do nothing: the pruned set below
// rejects everything underneath an excluded directory on its own, so both
// feeders treat excludes exactly as restic will.
func (c *suggestCollector) add(rel string, isDir bool, size int64) bool {
	if rel == "" {
		return true // the root itself is never a suggestion
	}
	// Only consulted once something has actually been pruned, so the live walk —
	// which prunes for itself via fs.SkipDir — pays nothing for this.
	if len(c.pruned) > 0 && c.underPruned(rel) {
		return false
	}
	full := c.rootSlash + "/" + rel
	base := path.Base(rel)
	if isDir {
		if matchesExcludePatterns(full, base, c.patterns) {
			c.pruned[rel] = true
			return false // already excluded — restic skips it, so do we
		}
		if strings.Count(rel, "/")+1 <= c.opts.maxDepth {
			c.byRel[rel] = &suggestCandidate{
				full:     full,
				rel:      rel,
				known:    knownJunkDirNames[strings.ToLower(base)],
				complete: true, // until a feeder says otherwise
			}
			c.order = append(c.order, rel)
		}
		return true
	}
	// Attribute the file's size to every ancestor directory within the depth
	// bound: each '/' in rel marks one ancestor prefix.
	for i := 0; i < len(rel); i++ {
		if rel[i] == '/' {
			if a, ok := c.byRel[rel[:i]]; ok {
				a.size += size
			}
		}
	}
	return true
}

// underPruned reports whether rel sits below a directory an exclude pattern
// already covered.
func (c *suggestCollector) underPruned(rel string) bool {
	for i := 0; i < len(rel); i++ {
		if rel[i] == '/' && c.pruned[rel[:i]] {
			return true
		}
	}
	return c.pruned[rel]
}

// markIncomplete flags every candidate whose recursive size is only a lower
// bound. fullPaths are absolute slash paths the feeder could not account for
// (the point a walk stopped at, an unreadable subtree); the candidate at that
// path and every ANCESTOR of it lost bytes, so all of them are flagged.
func (c *suggestCollector) markIncomplete(fullPaths map[string]bool) {
	for p := range fullPaths {
		rel, ok := relUnder(c.rootSlash, p)
		if !ok {
			continue
		}
		if rel == "" {
			// Stopped at the root itself: nothing below it can be trusted.
			for _, cd := range c.byRel {
				cd.complete = false
			}
			continue
		}
		if cd, found := c.byRel[rel]; found {
			cd.complete = false
		}
		for i := 0; i < len(rel); i++ {
			if rel[i] == '/' {
				if cd, found := c.byRel[rel[:i]]; found {
					cd.complete = false
				}
			}
		}
	}
}

// results qualifies the collected directories and returns them biggest first.
//
// Junk by NAME regardless of size, or big regardless of name. A candidate under
// a known-junk candidate is suppressed — excluding the parent already covers it,
// so listing the child too is pure noise. (Children of a merely-LARGE parent
// stay: the targeted child is often the better exclude.)
func (c *suggestCollector) results() []suggestCandidate {
	var cands []suggestCandidate
	for _, rel := range c.order {
		cd := c.byRel[rel]
		// #175: a candidate may only be dropped for being small when its size is
		// KNOWN to be small. Dropping on a lower bound is how a truncated scan hid
		// the biggest offender the user came for — the 55 GB folder read 5.7 GB,
		// fell under the threshold, and vanished from the list entirely.
		if cd.complete && !cd.known && cd.size < c.opts.largeBytes {
			continue
		}
		suppressed := false
		for i := 0; i < len(rel); i++ {
			if rel[i] == '/' {
				if a, ok := c.byRel[rel[:i]]; ok && a.known {
					suppressed = true
					break
				}
			}
		}
		if !suppressed {
			cands = append(cands, *cd)
		}
	}
	sortSuggestCandidates(cands)
	return cands
}

// sortSuggestCandidates orders candidates biggest first, ties by path.
func sortSuggestCandidates(cands []suggestCandidate) {
	sort.Slice(cands, func(i, j int) bool {
		if cands[i].size != cands[j].size {
			return cands[i].size > cands[j].size
		}
		return cands[i].rel < cands[j].rel
	})
}

// capSuggestions applies the suggestMaxResults cap WITHOUT letting the cut fall
// on a lower bound. The list mixes exact sizes with minimums, so a plain
// cands[:max] can drop a partially-measured 55 GB folder in favour of a fully
// measured 300 MiB one — the same silent-drop bug as the size gate, one step
// later. Incomplete candidates are seated first (there are at most a walk's
// ancestor chain of them per root, plus any unreadable subtrees), then the
// budget goes to the biggest complete ones. Relative order is preserved.
func capSuggestions(cands []suggestCandidate, max int) []suggestCandidate {
	if max <= 0 || len(cands) <= max {
		return cands
	}
	keep := make([]bool, len(cands))
	budget := max
	for i, c := range cands {
		if budget == 0 {
			break
		}
		if !c.complete {
			keep[i], budget = true, budget-1
		}
	}
	for i, c := range cands {
		if budget == 0 {
			break
		}
		if c.complete {
			keep[i], budget = true, budget-1
		}
	}
	out := make([]suggestCandidate, 0, max)
	for i, c := range cands {
		if keep[i] {
			out = append(out, c)
		}
	}
	return out
}

// ---------------------------------------------------------------------------
// Feeder A — the live walk
// ---------------------------------------------------------------------------

// suggestScan is one root's live-walk result. stoppedAt is the absolute slash
// path the walk stopped at ("" when it finished), so the caller can name the
// place the list ends.
type suggestScan struct {
	cands     []suggestCandidate
	truncated bool
	stoppedAt string
}

// suggestWalkDir is filepath.WalkDir, behind a variable so a test can deliver
// the callback errors a genuinely unreadable subtree produces. There is no
// portable way to MAKE a directory unreadable (chmod 0 does not stop a
// directory listing on Windows), and the error branch below is load-bearing —
// it is what stops an ancestor of a skipped subtree from reporting its short
// total as final. Production always uses filepath.WalkDir.
var suggestWalkDir = filepath.WalkDir

// scanExcludeCandidates walks one backup root and returns every directory at
// depth <= maxDepth with its recursive size (files at ANY depth are attributed
// to all their ancestors within the bound, so a shallow candidate's size is
// exact even when its junk sits deep). Directories matching an already-stored
// exclude pattern are pruned entirely — not suggested, not counted, exactly as
// restic will treat them.
//
// Completeness is DERIVED, not tracked. filepath.WalkDir is a documented
// LEXICAL DEPTH-FIRST walk: when it stops at path p, every directory it entered
// has had its whole subtree walked EXCEPT the ancestor chain of p. So the set of
// candidates carrying a mere lower bound is exactly that chain, plus the
// ancestors of anything skipped by an error (an unreadable subtree leaves its
// ancestors short by however much it held — reporting those as finished is the
// same lie in a quieter form, and messy Unraid appdata trees are precisely where
// it happens). Everything else is byte-exact.
//
// IF THIS WALK IS EVER PARALLELISED, this derivation silently starts marking
// partial directories as complete and #175 returns. TestWalkDirLexicalDFSInvariant
// is the guard: it must break loudly before that can ship.
func scanExcludeCandidates(ctx context.Context, root string, patterns []string, o suggestOpts) suggestScan {
	col := newSuggestCollector(root, patterns, o)
	out := suggestScan{}
	// Paths whose bytes are unaccounted for: the stop point, and every entry an
	// error made unreadable. markIncomplete turns each into its ancestor chain.
	incomplete := map[string]bool{}

	_ = suggestWalkDir(root, func(p string, d fs.DirEntry, err error) error {
		full := filepath.ToSlash(p)
		if ctx.Err() != nil {
			out.truncated = true
			// The lexical-DFS marker. p is where the walk stops, so p's ancestors are
			// the only directories left holding a partial total. A file stop point
			// names its parent instead: "the scan stopped inside <folder>" is what the
			// UI says, and a file is not a folder.
			stop := full
			if err == nil && d != nil && !d.IsDir() {
				stop = path.Dir(full)
			}
			incomplete[stop] = true
			if out.stoppedAt == "" {
				out.stoppedAt = stop
			}
			return fs.SkipAll
		}
		if err != nil {
			// Unreadable subtree — skip it, keep scanning the rest, but never let its
			// ancestors report the short total they now carry as final.
			incomplete[full] = true
			return nil
		}
		rel, under := relUnder(col.rootSlash, full)
		if !under {
			return nil // defensive: not under the root
		}
		if d.IsDir() {
			if !col.add(rel, true, 0) {
				return fs.SkipDir
			}
			return nil
		}
		info, iErr := d.Info()
		if iErr != nil {
			incomplete[full] = true
			return nil
		}
		col.add(rel, false, info.Size())
		return nil
	})

	col.markIncomplete(incomplete)
	out.cands = col.results()
	return out
}

// ---------------------------------------------------------------------------
// Feeder B — the snapshot aggregate
// ---------------------------------------------------------------------------

// aggregateSnapshotCandidates replays a snapshot's nodes through one collector
// per root and returns the merged candidates. Sizes come from what restic
// RECORDED, so they are exact and need no completeness derivation — the pass is
// all-or-nothing (see suggestSnapshotAggregate).
//
// Roots are the caller's snapshot∩selection intersection; a node under none of
// them is ignored, which is what keeps a snapshot path the user has since
// deselected from producing phantom candidates.
func aggregateSnapshotCandidates(entries func(func(restic.FileEntry)) error, roots, patterns []string, o suggestOpts) ([]suggestCandidate, error) {
	cols := make([]*suggestCollector, 0, len(roots))
	for _, r := range roots {
		cols = append(cols, newSuggestCollector(r, patterns, o))
	}
	err := entries(func(e restic.FileEntry) {
		full := path.Clean(filepath.ToSlash(e.Path))
		isDir := e.Type == "dir"
		for _, col := range cols {
			rel, under := relUnder(col.rootSlash, full)
			if !under {
				continue
			}
			size := int64(0)
			if !isDir {
				size = e.Size
			}
			col.add(rel, isDir, size)
			break
		}
	})
	if err != nil {
		return nil, err
	}
	var cands []suggestCandidate
	for _, col := range cols {
		cands = append(cands, col.results()...)
	}
	sortSuggestCandidates(cands)
	return cands, nil
}

// excludeLineFor maps a scanned directory (container-visible under the host
// mount, e.g. /host/user/user/appdata/plex/Cache) back to the raw exclude line
// the editor vocabulary uses: the path as seen INSIDE the target container when
// one of its mounts covers it (e.g. /config/Cache — the exact inverse of
// resolveExcludeLine's translation), else the scanned path verbatim (a
// passthrough line resolves to itself, so it still excludes correctly).
func (s *Service) excludeLineFor(full string, in model.Inspect) string {
	mountRoot := path.Clean(s.cfg.HostMountRoot)
	srcRoot := path.Clean(s.cfg.HostSourceRoot)
	host := ""
	switch {
	case full == mountRoot:
		host = srcRoot
	case strings.HasPrefix(full, mountRoot+"/"):
		host = srcRoot + "/" + strings.TrimPrefix(full, mountRoot+"/")
	}
	if host != "" {
		var bestSrc, bestDest string
		for _, m := range in.Mounts {
			src := path.Clean(m.Source)
			if src == "" || src == "/" || src == "." || m.Destination == "" {
				continue
			}
			if (host == src || strings.HasPrefix(host, src+"/")) && len(src) > len(bestSrc) {
				bestSrc, bestDest = src, path.Clean(m.Destination)
			}
		}
		if bestSrc != "" {
			return path.Clean(bestDest + strings.TrimPrefix(host, bestSrc))
		}
	}
	return full
}

// suggestExcludesKey builds the snapshot-cache key from everything a cached
// aggregate depends on: which repo, which snapshot, and the RESOLVED excludes
// the collector pruned with. Editing the exclude list changes what is pruned, so
// it must recompute — the hash keeps the key short and order-sensitive.
func suggestExcludesKey(repo, snapshotID string, resolved []string) string {
	h := sha256.New()
	for _, p := range resolved {
		_, _ = h.Write([]byte(p))
		_, _ = h.Write([]byte{0})
	}
	return repo + "|" + snapshotID + "|" + hex.EncodeToString(h.Sum(nil)[:8])
}

// newestSnapshotFor returns the newest LOCAL snapshot of one container, and
// whether there is one at all. Local only, on purpose: pulling a remote repo's
// index and tree packs to fill a UI panel would drag ~51 MB over B2 for a large
// tree. An off-site-only repo (or an unmounted share) reports "none" and the
// caller falls back to the live walk.
func (s *Service) newestSnapshotFor(ctx context.Context, name string) (restic.Snapshot, string, restic.Mode, bool) {
	settings, err := s.store.GetSettings()
	if err != nil {
		return restic.Snapshot{}, "", restic.Mode{}, false
	}
	repo, err := s.repoFor(settings, "containers", "local")
	if err != nil || localRepoMissing(repo) {
		return restic.Snapshot{}, "", restic.Mode{}, false
	}
	mode := s.ModeFor(settings)
	snaps, err := s.snapshotsForTag(ctx, repo, mode, "container:"+name)
	if err != nil || len(snaps) == 0 {
		return restic.Snapshot{}, "", restic.Mode{}, false
	}
	newest := snaps[0]
	for _, sn := range snaps[1:] {
		if sn.Time > newest.Time { // RFC3339 sorts lexically
			newest = sn
		}
	}
	return newest, repo, mode, true
}

// snapshotRoots intersects a snapshot's own recorded paths with the container's
// CURRENT effective backup paths. Derived, never assumed: it is what makes the
// snapshot path shape safe by construction (restic stores the exact absolute
// strings the backup was given — service.go's backup path), and it is also what
// covers the drift case where the folder selection narrowed since the last run.
// Same pattern RestoreSubtreeIncludeArgs' callers already rely on.
func snapshotRoots(snapPaths, effective []string) []string {
	inSnap := make(map[string]bool, len(snapPaths))
	for _, p := range snapPaths {
		inSnap[path.Clean(filepath.ToSlash(p))] = true
	}
	var roots []string
	for _, p := range effective {
		if c := path.Clean(filepath.ToSlash(p)); inSnap[c] {
			roots = append(roots, c)
		}
	}
	return roots
}

// suggestSnapshotAggregate produces the candidate list from the newest local
// snapshot, or reports why it could not. ok=false with a nil error means "no
// snapshot to read" (a container that has never been backed up, an off-site-only
// repo, an unmounted share) and the caller falls back to the live walk; a
// non-nil error is errSuggestIndexRead and is NOT a fallback — the UI offers the
// live scan as an explicit second request instead of chaining 70s onto one GET.
func (s *Service) suggestSnapshotAggregate(ctx context.Context, name string, roots, resolved []string, o suggestOpts) (cands []suggestCandidate, snapTime string, ok bool, err error) {
	snap, repo, mode, found := s.newestSnapshotFor(ctx, name)
	if !found {
		return nil, "", false, nil
	}
	roots = snapshotRoots(snap.Paths, roots)
	if len(roots) == 0 {
		// The snapshot holds none of the currently selected folders — reading it
		// would produce nothing. Let the live walk answer instead.
		return nil, "", false, nil
	}
	key := suggestExcludesKey(repo, snap.ID, resolved)
	if hit, hitOK := s.suggestCacheGet(name, key); hitOK {
		return hit.cands, hit.snapshotTime, true, nil
	}

	sctx, cancel := context.WithTimeout(ctx, suggestSnapshotTimeout)
	defer cancel()
	stream := func(onEntry func(restic.FileEntry)) error {
		return s.lsStreamSelfHeal(sctx, repo, snap.ID, mode, onEntry)
	}
	cands, aErr := aggregateSnapshotCandidates(stream, roots, resolved, o)
	if aErr != nil {
		// ALL-OR-NOTHING: whatever was aggregated so far is dropped on the floor.
		// Emitting it would be defect #175 with a different feeder.
		log.Printf("api: exclusion assistant: snapshot index read failed: %v", aErr)
		return nil, "", false, fmt.Errorf("%w: %v", errSuggestIndexRead, aErr)
	}
	if sctx.Err() != nil {
		return nil, "", false, fmt.Errorf("%w: %v", errSuggestIndexRead, sctx.Err())
	}
	s.suggestCachePut(name, suggestCacheEntry{key: key, snapshotTime: snap.Time, cands: cands})
	return cands, snap.Time, true, nil
}

// lsStreamSelfHeal is lsSelfHeal for the streaming listing: on a stale-lock
// error it clears stale locks and retries ONCE. Without it this path would
// reintroduce #129 verbatim — an exclusive lock left by an interrupted write
// elsewhere in the repo blocks even a shared-lock `ls`, and nothing else clears
// it until the next scheduled backup happens to run its own unlockStale.
func (s *Service) lsStreamSelfHeal(ctx context.Context, repo, snapshotID string, mode restic.Mode, onEntry func(restic.FileEntry)) error {
	err := s.engine.LsStream(ctx, repo, snapshotID, mode, onEntry)
	if isLockErr(err) {
		s.unlockStale(ctx, repo, mode)
		err = s.engine.LsStream(ctx, repo, snapshotID, mode, onEntry)
	}
	return err
}

func (s *Service) suggestCacheGet(name, key string) (suggestCacheEntry, bool) {
	s.suggestMu.Lock()
	defer s.suggestMu.Unlock()
	e, ok := s.suggestCache[name]
	if !ok || e.key != key {
		return suggestCacheEntry{}, false
	}
	return e, true
}

func (s *Service) suggestCachePut(name string, e suggestCacheEntry) {
	s.suggestMu.Lock()
	defer s.suggestMu.Unlock()
	if s.suggestCache == nil {
		s.suggestCache = map[string]suggestCacheEntry{}
	}
	s.suggestCache[name] = e // one entry per container: bounded, no eviction needed
}

// SuggestExcludes returns exclusion candidates for a container: well-known junk
// directories by name and any directory over the size threshold, biggest first,
// capped. Directories the stored excludes already cover are skipped.
//
// source "live" forces the live walk (the UI's explicit retry after a failed
// index read); anything else prefers the snapshot aggregate and falls back to
// the live walk when the container has no local snapshot to read. Read-only —
// nothing is persisted.
func (s *Service) SuggestExcludes(ctx context.Context, name, source string) (SuggestResult, error) {
	in, err := s.docker.Inspect(ctx, name)
	if err != nil {
		return SuggestResult{}, fmt.Errorf("inspect container: %w", err)
	}
	roots := s.effectiveBackupPaths(name, in)
	if len(roots) == 0 {
		return SuggestResult{Source: suggestSourceLive}, nil // stateless container — nothing to scan
	}
	var resolved []string
	if tg, gErr := s.store.GetTargetByContainer(name); gErr == nil {
		resolved = s.resolveExcludePatterns(tg.Excludes, in)
	}
	opts := suggestOpts{maxDepth: suggestMaxDepth, largeBytes: suggestLargeBytes}

	res := SuggestResult{Source: suggestSourceSnapshot}
	var cands []suggestCandidate
	if source != suggestSourceLive {
		snapCands, snapTime, ok, sErr := s.suggestSnapshotAggregate(ctx, name, roots, resolved, opts)
		if sErr != nil {
			return SuggestResult{Source: suggestSourceSnapshot}, sErr
		}
		if ok {
			cands, res.SnapshotTime = snapCands, snapTime
		} else {
			source = suggestSourceLive // never backed up — only a walk can answer
		}
	}
	if source == suggestSourceLive {
		res.Source = suggestSourceLive
		sctx, cancel := context.WithTimeout(ctx, suggestScanTimeout)
		defer cancel()
		for _, root := range roots {
			sc := scanExcludeCandidates(sctx, root, resolved, opts)
			cands = append(cands, sc.cands...)
			res.Truncated = res.Truncated || sc.truncated
			if res.StoppedAt == "" {
				res.StoppedAt = sc.stoppedAt
			}
		}
	}

	// Roots are collected in order, each pre-sorted; merge-sort the union so the
	// cap keeps the globally biggest — without ever cutting a lower bound.
	sortSuggestCandidates(cands)
	cands = capSuggestions(cands, suggestMaxResults)

	res.Suggestions = make([]ExcludeSuggestion, 0, len(cands))
	for _, c := range cands {
		reason := "large"
		if c.known {
			reason = "known-cache"
		}
		res.Suggestions = append(res.Suggestions, ExcludeSuggestion{
			Path:      c.rel,
			Line:      s.excludeLineFor(c.full, in),
			SizeBytes: c.size,
			Reason:    reason,
			Complete:  c.complete,
		})
	}
	if res.StoppedAt != "" {
		res.StoppedAt = s.excludeLineFor(res.StoppedAt, in)
	}
	return res, nil
}

// handleExcludesSuggest runs the exclusion assistant's scan for one container
// and returns exclude candidates (well-known junk dirs by name + any directory
// over the size threshold), biggest first. `?source=live` forces the live walk;
// the default prefers the snapshot's recorded sizes. Stateless — the UI adds
// picked lines through the normal excludes PATCH.
// GET /api/containers/{name}/excludes/suggest
func (h *Handler) handleExcludesSuggest(w http.ResponseWriter, r *http.Request) {
	name, ok := h.nameParam(w, r)
	if !ok {
		return
	}
	source := ""
	if r.URL.Query().Get("source") == suggestSourceLive {
		source = suggestSourceLive
	}
	res, err := h.svc.SuggestExcludes(r.Context(), name, source)
	if err != nil {
		// A failed index read is not a failed REQUEST: the panel stays up, states
		// that it could not finish reading the backup, and offers a live scan.
		if errors.Is(err, errSuggestIndexRead) {
			writeJSON(w, http.StatusOK, okEnvelope(map[string]any{
				"suggestions": []ExcludeSuggestion{},
				"truncated":   false,
				"source":      suggestSourceSnapshot,
				"indexFailed": true,
			}))
			return
		}
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	if res.Suggestions == nil {
		res.Suggestions = []ExcludeSuggestion{}
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{
		"suggestions":  res.Suggestions,
		"truncated":    res.Truncated,
		"source":       res.Source,
		"snapshotTime": res.SnapshotTime,
		"stoppedAt":    res.StoppedAt,
		"indexFailed":  false,
	}))
}
