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
	// suggestPartialSeats bounds how many of those seats a LOWER BOUND may take
	// out of size order. A truncated walk's genuinely-unmeasured set is the stop
	// point's ancestor chain, which is at most suggestMaxDepth deep and belongs at
	// the table no matter how small its measured fraction looks. Unreadable
	// subtrees are unbounded, and seating those unconditionally is how an exactly
	// measured 55 GiB folder gets evicted by twenty "at least 0 B" rows — the
	// mirror image of the bug this file exists to fix. Past the reservation a
	// lower bound competes on size like everything else.
	suggestPartialSeats = suggestMaxDepth
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

// suggestLive* say WHY a result came from the live walk. One string cannot
// serve all three: the panel used to announce "this container has no backup
// yet" on every live result, including the folder scan the user asked for after
// an index read failed — a container that demonstrably HAS a backup, which is
// why the index was read at all. The UI picks its sentence from this.
const (
	// suggestLiveNoSnapshot — no readable local snapshot for this container at
	// all: never backed up, off-site-only repo, or the repo itself is missing.
	suggestLiveNoSnapshot = "no-snapshot"
	// suggestLiveRequested — the caller asked for ?source=live.
	suggestLiveRequested = "requested"
	// suggestLiveNotInSnapshot — a snapshot exists, but it holds none of the
	// folders currently selected, so reading it would answer about the wrong ones.
	suggestLiveNotInSnapshot = "not-in-snapshot"
)

// errSuggestIndexRead is returned when the snapshot pass could not be completed
// — the listing errored, or it outran suggestSnapshotTimeout. The aggregate is
// DISCARDED in that case: a partially consumed `ls` stream would reproduce the
// exact defect this whole change exists to remove (#175), so there is no code
// path that emits a partial snapshot list.
var errSuggestIndexRead = errors.New("could not finish reading the backup index")

// errSuggestNodeOrder is the loud failure for a listing that hands a file to the
// collector before the folder holding it. The aggregate credits a file's bytes
// only to ancestors it has already seen, which is sound for restic's depth-first
// emission order and silently catastrophic without it: every directory would
// read 0 bytes, fall under the size threshold, and the panel would report
// "nothing left to exclude" for a 55 GB tree while flagging nothing. There is no
// test fixture that can drift into this — snapEntries builds its input with
// WalkDir — so the invariant is checked at runtime instead of assumed, and a
// violation becomes an index-read failure with a working live-scan fallback.
var errSuggestNodeOrder = errors.New("the backup index listed a file before the folder holding it")

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
//
// The three list fields below exist because "the list is short" and "the list is
// short for a reason nobody was told" are different products. Each names a
// specific way this pass came up empty of information rather than empty of
// findings:
//
//	UnexaminedRoots — backup folders the walk never opened, because the budget
//	  was already spent on an earlier one. StoppedAt names ONE place; with
//	  several roots it would otherwise be the only thing said about a run that
//	  never looked at three of them.
//	UnreadableRoots — backup folders the walk could not read at all. Without
//	  this they are indistinguishable from empty folders: nothing below them
//	  produces a row, so no per-row flag can speak for them.
//	PathsUnavailable — the container's folders are configured but none is
//	  reachable right now (an unmounted array or share) AND there was no
//	  snapshot to answer from instead.
type SuggestResult struct {
	Suggestions      []ExcludeSuggestion
	Truncated        bool
	Source           string
	LiveReason       string
	SnapshotTime     string
	StoppedAt        string
	UnexaminedRoots  []string
	UnreadableRoots  []string
	PathsUnavailable bool
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

// suggestFlight is one in-flight snapshot aggregate. Two requests for the same
// key (a refresh, a second tab, two viewers) share ONE `restic ls`: the second
// waits on done and reads the same result. Without it the panel's auto-scan
// multiplies restic processes inside a memory-capped container, which is the
// one place this feature is cheap to make expensive.
type suggestFlight struct {
	done     chan struct{}
	cands    []suggestCandidate
	snapTime string
	err      error
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
		// Wildcards, which the plain comparison above cannot see. The basename
		// branch has used path.Match all along; this branch compared literally, so
		// a path pattern with a wildcard in it — `/config/*/Cache`, the shape the
		// assistant's own suggestions take — matched nothing at all. The already
		// excluded folders were then suggested again AND their bytes counted
		// against the parent, both of which this function exists to prevent.
		//
		// path.Match does not cross "/" with "*", so `/config/*/Cache` matches one
		// level deep exactly as restic reads it. Nothing is needed for what lies
		// BELOW a match: a matched directory is added to the pruned set, and
		// underPruned rejects its whole subtree for both feeders.
		if ok, err := path.Match(p, full); err == nil && ok {
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
	// outOfOrder latches when a file arrived before a within-bound ancestor of
	// its own — i.e. the feeder broke the parents-before-children contract `add`
	// attributes sizes with. See errSuggestNodeOrder.
	outOfOrder bool
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
	// A FILE an exclude pattern covers is skipped by restic, so it must not be
	// counted here either — this function's own doc says "already excluded —
	// restic skips it, so do we", and until now only the directory branch above
	// honoured it. Without this, a stored line like `*.log` still let every log
	// file's bytes flow into its parents' totals, so an appdata folder that is
	// 90% logs was proposed as a fat exclusion candidate at its FULL on-disk
	// size, i.e. the assistant recommended excluding a folder on the strength of
	// bytes the user had already excluded.
	//
	// Only the live walk reaches this: the snapshot feeder replays what restic
	// itself stored, which never contains an excluded file. Returning false is
	// free for a file — the walk ignores the result for non-directories (there
	// is no subtree to prune) and only directories use it for fs.SkipDir.
	if matchesExcludePatterns(full, base, c.patterns) {
		return false
	}
	// Attribute the file's size to every ancestor directory within the depth
	// bound: each '/' in rel marks one ancestor prefix. An ancestor DEEPER than
	// the bound was never collected and is expected to be absent; one within the
	// bound that is absent means the feeder sent this file before its folder, and
	// those bytes are being lost silently — latch it (errSuggestNodeOrder).
	for i := 0; i < len(rel); i++ {
		if rel[i] != '/' {
			continue
		}
		anc := rel[:i]
		a, ok := c.byRel[anc]
		if !ok {
			if strings.Count(anc, "/")+1 <= c.opts.maxDepth {
				c.outOfOrder = true
			}
			continue
		}
		a.size += size
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

// capSuggestions applies the suggestMaxResults cap without letting the cut fall
// on a lower bound merely BECAUSE it is a lower bound, and without letting lower
// bounds shove out folders that are demonstrably bigger.
//
// The list mixes exact sizes with minimums, so a plain cands[:max] drops a
// partially measured 55 GB folder in favour of a fully measured 300 MiB one —
// the same silent-drop bug as the size gate, one step later. Seating EVERY
// incomplete candidate first is the same bug pointed the other way: unreadable
// subtrees are unbounded, so twenty "at least 0 B" rows evict the one exactly
// measured 55 GiB folder the user came for.
//
// So: suggestPartialSeats of the budget are reserved for the biggest lower
// bounds (cands arrives sorted, so a forward pass takes them biggest first) —
// that covers a truncated walk's ancestor chain, which is the bounded population
// the reservation exists for. Everything after that competes on size, either
// kind. Relative order is preserved.
func capSuggestions(cands []suggestCandidate, max int) []suggestCandidate {
	if max <= 0 || len(cands) <= max {
		return cands
	}
	keep := make([]bool, len(cands))
	budget := max
	seats := suggestPartialSeats
	if seats > max {
		seats = max
	}
	for i, c := range cands {
		if seats == 0 || budget == 0 {
			break
		}
		if !c.complete {
			keep[i], seats, budget = true, seats-1, budget-1
		}
	}
	for i := range cands {
		if budget == 0 {
			break
		}
		if !keep[i] {
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
	// rootUnreadable is set when the walk could not read the ROOT itself. Nothing
	// below it produced a row, so no per-row flag can speak for it and an empty
	// candidate list is otherwise indistinguishable from an empty folder — with
	// several roots, one unreadable root would read as a finished scan of all of
	// them. markIncomplete cannot cover this: at the root there is nothing
	// collected yet to mark.
	rootUnreadable bool
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
			if full == col.rootSlash {
				// The ROOT is the thing that could not be read. There are no ancestors
				// to mark and no rows to flag, so the fact has to travel out on its own
				// or the caller reports a confident empty list for the whole folder.
				out.rootUnreadable = true
			}
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
		if col.outOfOrder {
			// Sizes were dropped on the floor as the stream was read. Every directory
			// would read short (most of them 0), fall under the threshold and vanish,
			// and nothing in the result would say so. Fail loudly instead.
			return nil, fmt.Errorf("%w (root %s)", errSuggestNodeOrder, col.rootSlash)
		}
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
// aggregate depends on: which repo, which snapshot, which ROOTS were aggregated,
// and the RESOLVED excludes the collector pruned with.
//
// The roots are not optional. They are a live input to the aggregate — deselect
// a folder without touching the excludes and the candidate set must change — and
// leaving them out meant a cache hit kept offering exclude lines for folders the
// user had just stopped backing up, on every rescan until the next backup wrote
// a new snapshot ID. Both lists are sorted into the hash so merely REORDERING
// the same selection still hits.
func suggestExcludesKey(repo, snapshotID string, roots, resolved []string) string {
	h := sha256.New()
	for _, set := range [][]string{roots, resolved} {
		sorted := append([]string(nil), set...)
		sort.Strings(sorted)
		for _, p := range sorted {
			_, _ = h.Write([]byte(p))
			_, _ = h.Write([]byte{0})
		}
		_, _ = h.Write([]byte{1}) // separator: the two lists never blur together
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
func (s *Service) suggestSnapshotAggregate(ctx context.Context, name string, roots, resolved []string, o suggestOpts) (cands []suggestCandidate, snapTime string, ok bool, why string, err error) {
	// The budget starts HERE, not at the `ls`. suggestSnapshotTimeout's own
	// comment justifies its value against the 60s proxy timeout in front of a
	// typical Unraid install — but it used to wrap only the listing, while
	// newestSnapshotFor ahead of it (GetSettings, repoFor, and a snapshots
	// listing that itself retries after unlocking a stale lock) ran on the naked
	// request context with no bound at all. On a cold-starting array that prelude
	// alone can outlast the proxy, so the promise the constant makes was not one
	// the code kept.
	ctx, cancelBudget := context.WithTimeout(ctx, suggestSnapshotTimeout)
	defer cancelBudget()

	snap, repo, mode, found := s.newestSnapshotFor(ctx, name)
	if !found {
		return nil, "", false, suggestLiveNoSnapshot, nil
	}
	roots = snapshotRoots(snap.Paths, roots)
	if len(roots) == 0 {
		// The snapshot holds none of the currently selected folders — reading it
		// would produce nothing. Let the live walk answer instead.
		return nil, "", false, suggestLiveNotInSnapshot, nil
	}
	key := suggestExcludesKey(repo, snap.ID, roots, resolved)
	if hit, hitOK := s.suggestCacheGet(name, key); hitOK {
		return hit.cands, hit.snapshotTime, true, "", nil
	}

	// SINGLEFLIGHT. Panel-open auto-scans arrive in bursts (a refresh, a second
	// tab, two viewers), and each miss otherwise starts its own `restic ls`
	// against the same repo inside a memory-capped container.
	fl, leader := s.suggestFlightFor(key)
	if !leader {
		select {
		case <-fl.done:
		case <-ctx.Done():
			return nil, "", false, "", fmt.Errorf("%w: %v", errSuggestIndexRead, ctx.Err())
		}
		if fl.err != nil {
			return nil, "", false, "", fl.err
		}
		return fl.cands, fl.snapTime, true, "", nil
	}
	defer s.suggestFlightDone(key, fl)

	// DETACHED from the leader's request, the same way the encryption-detect
	// singleflight next door does it (encryption_detect.go, which has a test
	// pinning exactly this). The pass is SHARED and the leader is merely whoever
	// arrived first, so running the restic ls on ITS request context meant that
	// closing its browser tab killed the process — and every follower, with a
	// perfectly live request of its own, got handed errSuggestIndexRead for
	// something that had nothing to do with it. Followers still honour their own
	// ctx while waiting on fl.done above; the pass runs to completion under its
	// own budget.
	// WithoutCancel detaches from the LEADER's request (see below); the timeout is
	// what remains of the budget opened at the top of this function, so the two
	// halves share one bound instead of the listing getting a fresh full one.
	deadline, hasDeadline := ctx.Deadline()
	sctx, cancel := context.WithCancel(context.WithoutCancel(ctx))
	if hasDeadline {
		sctx, cancel = context.WithDeadline(context.WithoutCancel(ctx), deadline)
	}
	defer cancel()
	stream := func(onEntry func(restic.FileEntry)) error {
		return s.lsStreamSelfHeal(sctx, repo, snap.ID, mode, onEntry)
	}
	agg, aErr := aggregateSnapshotCandidates(stream, roots, resolved, o)
	if aErr != nil {
		// ALL-OR-NOTHING: whatever was aggregated so far is dropped on the floor.
		// Emitting it would be defect #175 with a different feeder.
		log.Printf("api: exclusion assistant: snapshot index read failed: %v", aErr)
		fl.err = fmt.Errorf("%w: %v", errSuggestIndexRead, aErr)
		return nil, "", false, "", fl.err
	}
	if sctx.Err() != nil {
		fl.err = fmt.Errorf("%w: %v", errSuggestIndexRead, sctx.Err())
		return nil, "", false, "", fl.err
	}
	s.suggestCachePut(name, suggestCacheEntry{key: key, snapshotTime: snap.Time, cands: agg})
	// Clears the errSuggestAborted seed — success is the ONLY thing that does, so
	// a leader that dies anywhere before this line leaves followers a failure
	// rather than an empty list.
	fl.cands, fl.snapTime, fl.err = agg, snap.Time, nil
	return agg, snap.Time, true, "", nil
}

// suggestFlightFor joins the in-flight aggregate for key, or registers this
// caller as the one that will produce it. leader=false means the returned flight
// belongs to someone else and must only be waited on.
// errSuggestAborted is what a follower is told when the leading pass died before
// writing a result. A flight is seeded with it so "no answer" can never be read
// as "the answer is: nothing" — the zero value of err is nil, and nil with a nil
// candidate list is indistinguishable from a genuine empty result. The follower
// would then be told there is nothing left to exclude on a large tree. Same
// treatment, and the same reasoning, as encryption_detect.go's
// errEncryptionDetectAborted.
var errSuggestAborted = errors.New("exclusion scan did not complete")

func (s *Service) suggestFlightFor(key string) (fl *suggestFlight, leader bool) {
	s.suggestMu.Lock()
	defer s.suggestMu.Unlock()
	if s.suggestFlights == nil {
		s.suggestFlights = map[string]*suggestFlight{}
	}
	if existing, ok := s.suggestFlights[key]; ok {
		return existing, false
	}
	// Seeded with the aborted error, not the zero value — see errSuggestAborted.
	fl = &suggestFlight{done: make(chan struct{}), err: errSuggestAborted}
	s.suggestFlights[key] = fl
	return fl, true
}

// suggestFlightDone publishes the leader's result to every waiter and retires
// the flight. Deferred, so a panic cannot leave followers blocked forever.
func (s *Service) suggestFlightDone(key string, fl *suggestFlight) {
	s.suggestMu.Lock()
	if s.suggestFlights[key] == fl {
		delete(s.suggestFlights, key)
	}
	s.suggestMu.Unlock()
	close(fl.done)
}

// lsStreamSelfHeal is lsSelfHeal for the streaming listing: on a stale-lock
// error it clears stale locks and retries ONCE. Without it this path would
// reintroduce #129 verbatim — an exclusive lock left by an interrupted write
// elsewhere in the repo blocks even a shared-lock `ls`, and nothing else clears
// it until the next scheduled backup happens to run its own unlockStale.
//
// THE RETRY IS NOT IDEMPOTENT and the guard below is what makes that safe. The
// buffered lsSelfHeal this was modelled on could retry freely because it
// returned a fresh slice each time; the streaming twin hands nodes straight to
// collectors that keep them, so replaying a stream that had already emitted
// would count every one of those nodes twice — a directory listed twice, at
// double its size, under a duplicate React key. restic takes the repo lock
// before it emits anything, so a lock error after the first node is not the
// stale-lock-at-open case this exists for: it is reported as-is, the
// all-or-nothing rule discards the partial aggregate, and the panel offers the
// live scan.
func (s *Service) lsStreamSelfHeal(ctx context.Context, repo, snapshotID string, mode restic.Mode, onEntry func(restic.FileEntry)) error {
	emitted := false
	fed := func(e restic.FileEntry) {
		emitted = true
		onEntry(e)
	}
	err := s.engine.LsStream(ctx, repo, snapshotID, mode, fed)
	if isLockErr(err) && !emitted {
		s.unlockStale(ctx, repo, mode)
		err = s.engine.LsStream(ctx, repo, snapshotID, mode, fed)
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
// index read, and the standing "what is on disk RIGHT NOW" question a snapshot
// cannot answer); anything else prefers the snapshot aggregate and falls back to
// the live walk when there is no snapshot to read. Read-only — nothing is
// persisted.
//
// The snapshot pass runs against the CONFIGURED folders, not the ones that
// currently stat: it reads the backup, so an unmounted array is no reason to
// withhold the answer it holds. Only the live walk needs a filesystem, and when
// it has none it says so instead of returning an empty list.
func (s *Service) SuggestExcludes(ctx context.Context, name, source string) (SuggestResult, error) {
	in, err := s.docker.Inspect(ctx, name)
	if err != nil {
		return SuggestResult{}, fmt.Errorf("inspect container: %w", err)
	}
	configured := s.configuredBackupPaths(name, in)
	if len(configured) == 0 {
		// Genuinely stateless: no selection, no appdata mount. Nothing to scan and
		// nothing to read — an empty list here is the true answer.
		return SuggestResult{Source: suggestSourceLive, LiveReason: suggestLiveNoSnapshot}, nil
	}
	roots := onlyExistingPaths(configured)
	var resolved []string
	if tg, gErr := s.store.GetTargetByContainer(name); gErr == nil {
		resolved = s.resolveExcludePatterns(tg.Excludes, in)
	}
	opts := suggestOpts{maxDepth: suggestMaxDepth, largeBytes: suggestLargeBytes}

	res := SuggestResult{Source: suggestSourceSnapshot}
	var cands []suggestCandidate
	if source == suggestSourceLive {
		res.LiveReason = suggestLiveRequested
	} else {
		snapCands, snapTime, ok, why, sErr := s.suggestSnapshotAggregate(ctx, name, configured, resolved, opts)
		if sErr != nil {
			return SuggestResult{Source: suggestSourceSnapshot}, sErr
		}
		if ok {
			cands, res.SnapshotTime = snapCands, snapTime
		} else {
			source, res.LiveReason = suggestSourceLive, why // only a walk can answer
		}
	}
	if source == suggestSourceLive {
		res.Source = suggestSourceLive
		if len(roots) == 0 {
			// Folders are configured but none is reachable, and there was no snapshot
			// to answer from. A walk has nothing to walk; reporting "nothing left to
			// exclude" would be the loudest lie this panel can tell.
			res.PathsUnavailable = true
			res.Suggestions = []ExcludeSuggestion{}
			return res, nil
		}
		sctx, cancel := context.WithTimeout(ctx, suggestScanTimeout)
		defer cancel()
		cands = append(cands, suggestLivePass(sctx, roots, resolved, opts, &res)...)
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
	res.UnexaminedRoots = s.excludeLinesFor(res.UnexaminedRoots, in)
	res.UnreadableRoots = s.excludeLinesFor(res.UnreadableRoots, in)
	return res, nil
}

// suggestLivePass walks every root under ONE shared budget and records, into
// res, everything about the run that no per-row flag can express.
//
// The shared budget is the reason this is not a plain loop over
// scanExcludeCandidates: once it expires, every remaining root would abort on
// its own first callback and contribute nothing, while StoppedAt still named the
// FIRST root's stop point. With four backup folders that reads as a finished
// scan of all four. A root never opened is named as such instead, and a root
// that aborted AT its own top is counted the same way — the walk got nowhere
// inside it either.
//
// Its own function so a test can drive the multi-root ordering with a context it
// controls (context.WithTimeout cannot be made to expire on demand).
func suggestLivePass(ctx context.Context, roots, resolved []string, o suggestOpts, res *SuggestResult) []suggestCandidate {
	var cands []suggestCandidate
	// NESTED roots are dropped before scanning. Each root gets its own collector,
	// so /appdata/plex and /appdata/plex/Media walked the same directories twice
	// and emitted the same candidate twice — with the same exclude line, and
	// therefore the same React key, while burning two of the twenty suggestion
	// slots. The snapshot feeder cannot hit this (one pass over one tree), which
	// is why it only shows up on the live path.
	roots = dropNestedRoots(roots)
	for _, root := range roots {
		if ctx.Err() != nil {
			res.Truncated = true
			res.UnexaminedRoots = append(res.UnexaminedRoots, root)
			continue
		}
		sc := scanExcludeCandidates(ctx, root, resolved, o)
		cands = append(cands, sc.cands...)
		if sc.rootUnreadable {
			res.UnreadableRoots = append(res.UnreadableRoots, root)
		}
		if sc.truncated {
			res.Truncated = true
			if sc.stoppedAt == root {
				res.UnexaminedRoots = append(res.UnexaminedRoots, root)
			} else if res.StoppedAt == "" {
				res.StoppedAt = sc.stoppedAt
			}
		}
	}
	return cands
}

// dropNestedRoots removes any root that already lies underneath another one,
// keeping the outermost. Sorting puts a parent ahead of its children, so a
// single pass over the kept set answers it.
//
// Comparison is on the slash-normalised path with a trailing separator, so
// "/appdata/plex-extra" is NOT treated as nested under "/appdata/plex".
func dropNestedRoots(roots []string) []string {
	if len(roots) < 2 {
		return roots
	}
	sorted := make([]string, len(roots))
	copy(sorted, roots)
	sort.Strings(sorted)
	kept := make([]string, 0, len(sorted))
	for _, r := range sorted {
		norm := strings.TrimSuffix(filepath.ToSlash(r), "/")
		nested := false
		for _, k := range kept {
			if norm == k || strings.HasPrefix(norm, k+"/") {
				nested = true
				break
			}
		}
		if !nested {
			kept = append(kept, norm)
		}
	}
	return kept
}

// excludeLinesFor maps a list of scanned paths into the editor's vocabulary, so
// a folder named in a banner reads the same way as the lines in the box above
// it. Returns nil for an empty list (the JSON stays absent rather than []).
func (s *Service) excludeLinesFor(paths []string, in model.Inspect) []string {
	if len(paths) == 0 {
		return nil
	}
	out := make([]string, 0, len(paths))
	for _, p := range paths {
		out = append(out, s.excludeLineFor(p, in))
	}
	return out
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
		"suggestions":      res.Suggestions,
		"truncated":        res.Truncated,
		"source":           res.Source,
		"liveReason":       res.LiveReason,
		"snapshotTime":     res.SnapshotTime,
		"stoppedAt":        res.StoppedAt,
		"unexaminedRoots":  res.UnexaminedRoots,
		"unreadableRoots":  res.UnreadableRoots,
		"pathsUnavailable": res.PathsUnavailable,
		"indexFailed":      false,
	}))
}
