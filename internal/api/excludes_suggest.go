package api

import (
	"context"
	"fmt"
	"io/fs"
	"net/http"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/model"
)

// ---------------------------------------------------------------------------
// Exclusion assistant — scans a container's backed-up folders for well-known
// junk directories (caches, temp, logs, transcodes, …) and unusually large
// directories, so the UI can offer one-click restic excludes that shrink the
// backup. Read-only: nothing here persists anything — the user picks, and the
// existing SetExcludes/PATCH path stores the chosen lines.
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
	// suggestScanTimeout bounds the whole walk; on timeout the partial result is
	// returned with truncated=true instead of hanging the request on a huge tree.
	suggestScanTimeout = 30 * time.Second
)

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
type ExcludeSuggestion struct {
	Path      string `json:"path"`
	Line      string `json:"line"`
	SizeBytes int64  `json:"sizeBytes"`
	Reason    string `json:"reason"` // "known-cache" | "large"
}

// suggestOpts are the scan bounds, injectable so tests can shrink them.
type suggestOpts struct {
	maxDepth   int
	largeBytes int64
}

// suggestCandidate is a directory the walker considered: full is the absolute
// slash-normalized path, rel its path below the walk root, size the recursive
// byte total, known whether its basename is a well-known junk name.
type suggestCandidate struct {
	full  string
	rel   string
	size  int64
	known bool
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

// scanExcludeCandidates walks one backup root and returns every directory at
// depth <= maxDepth with its recursive size (files at ANY depth are attributed
// to all their ancestors within the bound, so a shallow candidate's size is
// exact even when its junk sits deep). Directories matching an already-stored
// exclude pattern are pruned entirely — not suggested, not counted, exactly as
// restic will treat them. Unreadable entries are skipped, not fatal. A context
// deadline/cancel stops the walk early and reports truncated=true with the
// partial result.
func scanExcludeCandidates(ctx context.Context, root string, patterns []string, o suggestOpts) (cands []suggestCandidate, truncated bool) {
	rootSlash := strings.TrimSuffix(filepath.ToSlash(root), "/")
	byRel := map[string]*suggestCandidate{}
	var order []string // rel keys in walk order (parents before children)

	_ = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if ctx.Err() != nil {
			truncated = true
			return fs.SkipAll
		}
		if err != nil {
			return nil // unreadable subtree — skip it, keep scanning the rest
		}
		full := filepath.ToSlash(p)
		if full == rootSlash {
			return nil // the root itself is never a suggestion
		}
		rel := strings.TrimPrefix(full, rootSlash+"/")
		if rel == full {
			return nil // defensive: not under the root
		}
		base := path.Base(rel)
		if d.IsDir() {
			if matchesExcludePatterns(full, base, patterns) {
				return fs.SkipDir // already excluded — restic skips it, so do we
			}
			if strings.Count(rel, "/")+1 <= o.maxDepth {
				byRel[rel] = &suggestCandidate{full: full, rel: rel, known: knownJunkDirNames[strings.ToLower(base)]}
				order = append(order, rel)
			}
			return nil
		}
		info, iErr := d.Info()
		if iErr != nil {
			return nil
		}
		// Attribute the file's size to every ancestor directory within the depth
		// bound: each '/' in rel marks one ancestor prefix.
		for i := 0; i < len(rel); i++ {
			if rel[i] == '/' {
				if c, ok := byRel[rel[:i]]; ok {
					c.size += info.Size()
				}
			}
		}
		return nil
	})

	// Qualify: junk by NAME regardless of size, or big regardless of name. A
	// candidate under a known-junk candidate is suppressed — excluding the parent
	// already covers it, so listing the child too is pure noise. (Children of a
	// merely-LARGE parent stay: the targeted child is often the better exclude.)
	for _, rel := range order {
		c := byRel[rel]
		if !c.known && c.size < o.largeBytes {
			continue
		}
		suppressed := false
		for i := 0; i < len(rel); i++ {
			if rel[i] == '/' {
				if a, ok := byRel[rel[:i]]; ok && a.known {
					suppressed = true
					break
				}
			}
		}
		if !suppressed {
			cands = append(cands, *c)
		}
	}
	sort.Slice(cands, func(i, j int) bool {
		if cands[i].size != cands[j].size {
			return cands[i].size > cands[j].size
		}
		return cands[i].rel < cands[j].rel
	})
	return cands, truncated
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

// SuggestExcludes scans the container's backed-up folders server-side and
// returns exclusion candidates: well-known junk directories by name and any
// directory over the size threshold, biggest first, capped. Directories the
// stored excludes already cover are skipped. The scan is depth- and
// time-bounded; truncated=true means the time budget ran out and the list is
// the partial result. Read-only — nothing is persisted.
func (s *Service) SuggestExcludes(ctx context.Context, name string) (suggestions []ExcludeSuggestion, truncated bool, err error) {
	in, err := s.docker.Inspect(ctx, name)
	if err != nil {
		return nil, false, fmt.Errorf("inspect container: %w", err)
	}
	roots := s.effectiveBackupPaths(name, in)
	if len(roots) == 0 {
		return nil, false, nil // stateless container — nothing to scan
	}
	var resolved []string
	if tg, gErr := s.store.GetTargetByContainer(name); gErr == nil {
		resolved = s.resolveExcludePatterns(tg.Excludes, in)
	}
	sctx, cancel := context.WithTimeout(ctx, suggestScanTimeout)
	defer cancel()

	opts := suggestOpts{maxDepth: suggestMaxDepth, largeBytes: suggestLargeBytes}
	var cands []suggestCandidate
	for _, root := range roots {
		cs, tr := scanExcludeCandidates(sctx, root, resolved, opts)
		cands = append(cands, cs...)
		truncated = truncated || tr
	}
	// Roots are scanned in order, each pre-sorted; merge-sort the union so the
	// cap keeps the globally biggest.
	sort.Slice(cands, func(i, j int) bool {
		if cands[i].size != cands[j].size {
			return cands[i].size > cands[j].size
		}
		return cands[i].rel < cands[j].rel
	})
	if len(cands) > suggestMaxResults {
		cands = cands[:suggestMaxResults]
	}
	suggestions = make([]ExcludeSuggestion, 0, len(cands))
	for _, c := range cands {
		reason := "large"
		if c.known {
			reason = "known-cache"
		}
		suggestions = append(suggestions, ExcludeSuggestion{
			Path:      c.rel,
			Line:      s.excludeLineFor(c.full, in),
			SizeBytes: c.size,
			Reason:    reason,
		})
	}
	return suggestions, truncated, nil
}

// handleExcludesSuggest runs the exclusion assistant's scan for one container:
// it walks the container's backed-up folders server-side and returns exclude
// candidates (well-known junk dirs by name + any directory over the size
// threshold), biggest first. Stateless — the UI adds picked lines through the
// normal excludes PATCH. GET /api/containers/{name}/excludes/suggest
func (h *Handler) handleExcludesSuggest(w http.ResponseWriter, r *http.Request) {
	name, ok := h.nameParam(w, r)
	if !ok {
		return
	}
	suggestions, truncated, err := h.svc.SuggestExcludes(r.Context(), name)
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	if suggestions == nil {
		suggestions = []ExcludeSuggestion{}
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"suggestions": suggestions, "truncated": truncated}))
}
