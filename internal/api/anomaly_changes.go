package api

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"path"
	"slices"
	"strings"
	"sync"

	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/sizetree"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// A finding says that a backup differs from the ones before it, and the first
// question is always where. The comparison answers it from the repository: what
// restic diff reports between the last good backup and the first affected one,
// weighed with the file sizes both listings carry.

// changeMetrics are the findings a comparison can explain, the ones about how
// much a backup held or stored.
var changeMetrics = []string{
	metricSourceBytesShrink, metricSourceFilesShrink, metricSourceBytesGrowth,
	metricNewData, metricNewDataRewrite,
}

// changeDomains maps an item's anomaly domain to the one its backups are
// listed under.
var changeDomains = map[string]string{
	anomalyDomainContainer: "containers",
	anomalyDomainVM:        "vms",
	"files":                "files",
}

const (
	// changeFolderRows is how many folders a comparison names before it sums
	// up the rest.
	changeFolderRows = 6
	// changeFocusShare is how much of a folder's churn one subfolder has to
	// carry to count as where it happened.
	changeFocusShare = 0.9
	// changeResultsKept bounds the comparisons held in memory.
	changeResultsKept = 64
)

// regenerableNames are folder names that apps fill with data they can rebuild:
// search indexes, caches, thumbnails and transcodes. Churn in one of them is
// rarely a loss.
var regenerableNames = []string{
	"index", "indexes", "indices", "cache", "caches", ".cache", "__pycache__",
	"thumbnails", "thumbnail", "thumbs", ".thumbnails", "transcode", "transcodes",
	"tmp", "temp", ".tmp",
}

// ChangeFolder is what changed under one folder between two backups. Changed
// files are in both and differ; their bytes are the new size.
type ChangeFolder struct {
	Path         string `json:"path"`
	RemovedBytes int64  `json:"removedBytes"`
	RemovedFiles int64  `json:"removedFiles"`
	AddedBytes   int64  `json:"addedBytes"`
	AddedFiles   int64  `json:"addedFiles"`
	ChangedBytes int64  `json:"changedBytes"`
	ChangedFiles int64  `json:"changedFiles"`
}

func (f *ChangeFolder) churn() int64 {
	return f.RemovedBytes + f.AddedBytes + f.ChangedBytes
}

func (f *ChangeFolder) files() int64 {
	return f.RemovedFiles + f.AddedFiles + f.ChangedFiles
}

func (f *ChangeFolder) add(c fileChange) {
	switch c.kind {
	case fileRemoved:
		f.RemovedBytes += c.size
		f.RemovedFiles++
	case fileAdded:
		f.AddedBytes += c.size
		f.AddedFiles++
	default:
		f.ChangedBytes += c.size
		f.ChangedFiles++
	}
}

// ChangeSummary is the comparison of two backups of one item.
type ChangeSummary struct {
	Total ChangeFolder `json:"total"`
	// Folders are the busiest folders at the first level where the changes
	// part ways, Other the rest of that level summed up.
	Folders []ChangeFolder `json:"folders"`
	Other   *ChangeFolder  `json:"other,omitempty"`
	// Focus is the deepest folder that holds nearly all of the churn, empty
	// when it is spread out.
	Focus string `json:"focus,omitempty"`
	// Regenerable says Focus looks like data the app rebuilds by itself.
	Regenerable bool `json:"regenerable,omitempty"`
	// Partial says the diff named more files than a comparison follows.
	Partial bool `json:"partial,omitempty"`
}

// ChangesView answers for one finding's comparison.
type ChangesView struct {
	State   string         `json:"state"`
	Error   string         `json:"error,omitempty"`
	FromAt  int64          `json:"fromAt,omitempty"`
	ToAt    int64          `json:"toAt,omitempty"`
	Root    string         `json:"root,omitempty"`
	Summary *ChangeSummary `json:"summary,omitempty"`
}

type changeKind int

const (
	fileRemoved changeKind = iota
	fileAdded
	fileModified
)

type fileChange struct {
	kind changeKind
	size int64
}

// summarizeChanges groups the changed files, keyed by their path below root,
// into folders.
func summarizeChanges(changes map[string]fileChange) ChangeSummary {
	type node struct {
		sum      ChangeFolder
		children map[string]*node
	}
	top := &node{children: map[string]*node{}}
	for rel, c := range changes {
		top.sum.add(c)
		n := top
		parts := strings.Split(rel, "/")
		for i, part := range parts[:len(parts)-1] {
			child := n.children[part]
			if child == nil {
				child = &node{sum: ChangeFolder{Path: strings.Join(parts[:i+1], "/")}, children: map[string]*node{}}
				n.children[part] = child
			}
			child.sum.add(c)
			n = child
		}
	}

	weight := func(f *ChangeFolder) int64 { return f.churn() }
	if top.sum.churn() == 0 {
		weight = func(f *ChangeFolder) int64 { return f.files() }
	}
	busiest := func(n *node) *node {
		var best *node
		for _, child := range n.children {
			if best == nil || weight(&child.sum) > weight(&best.sum) ||
				(weight(&child.sum) == weight(&best.sum) && child.sum.Path < best.sum.Path) {
				best = child
			}
		}
		return best
	}

	out := ChangeSummary{Total: top.sum}

	level := top
	for len(level.children) == 1 {
		only := busiest(level)
		if weight(&only.sum) != weight(&level.sum) {
			break
		}
		level = only
	}
	rows := make([]ChangeFolder, 0, len(level.children))
	for _, child := range level.children {
		rows = append(rows, child.sum)
	}
	slices.SortFunc(rows, func(a, b ChangeFolder) int {
		return cmp.Or(cmp.Compare(weight(&b), weight(&a)), cmp.Compare(a.Path, b.Path))
	})
	rows = rows[:min(len(rows), changeFolderRows)]
	// The rest is what the level holds beyond the rows shown, which takes in
	// the files that sit in it directly.
	other := level.sum
	other.Path = ""
	for _, r := range rows {
		other.RemovedBytes -= r.RemovedBytes
		other.RemovedFiles -= r.RemovedFiles
		other.AddedBytes -= r.AddedBytes
		other.AddedFiles -= r.AddedFiles
		other.ChangedBytes -= r.ChangedBytes
		other.ChangedFiles -= r.ChangedFiles
	}
	if other.files() > 0 {
		out.Other = &other
	}
	out.Folders = rows

	focus := top
	for {
		next := busiest(focus)
		if next == nil || float64(weight(&next.sum)) < changeFocusShare*float64(weight(&focus.sum)) {
			break
		}
		focus = next
	}
	if focus != top {
		out.Focus = focus.sum.Path
		out.Regenerable = slices.ContainsFunc(strings.Split(out.Focus, "/"), func(part string) bool {
			return slices.Contains(regenerableNames, strings.ToLower(part))
		})
	}
	return out
}

// changeComparisons runs one comparison at a time and keeps the results, which
// never change for a pair of snapshots.
type changeComparisons struct {
	mu      sync.Mutex
	run     sync.Mutex
	results map[string]ChangesView
}

// changePair is the two backups a finding is explained by.
type changePair struct {
	item     breakdownItem
	from, to store.Run
}

func (p changePair) key() string { return p.item.id + "/" + p.from.SnapshotID + "/" + p.to.SnapshotID }

var errNoComparison = errors.New("this finding has no backup before it to compare with")

// AnomalyChanges answers what changed between the last good backup and the
// first affected one of a finding. Without a result it starts the comparison
// and says so; retry asks again after one failed.
func (s *Service) AnomalyChanges(id string, retry bool) (ChangesView, error) {
	pair, err := s.changePairOf(id)
	if err != nil {
		return ChangesView{}, err
	}
	key := pair.key()
	c := &s.changes
	c.mu.Lock()
	view, known := c.results[key]
	if known && view.State == breakdownFailed && retry {
		known = false
	}
	if !known {
		if c.results == nil || len(c.results) >= changeResultsKept {
			c.results = map[string]ChangesView{}
		}
		view = ChangesView{State: breakdownRunning, FromAt: pair.from.StartedAt, ToAt: pair.to.StartedAt}
		c.results[key] = view
		go s.compareBackups(pair)
	}
	c.mu.Unlock()
	return view, nil
}

// changePairOf finds the two backups behind a finding: the first affected one,
// and the last good one or, for a finding without one, the backup before it.
func (s *Service) changePairOf(id string) (changePair, error) {
	row, found, err := s.store.GetAnomaly(id)
	if err != nil {
		return changePair{}, err
	}
	if !found {
		return changePair{}, errors.New("no such entry")
	}
	domain, ok := changeDomains[row.Domain]
	if !ok || row.ScopeKind != anomalyScopeItem || !slices.Contains(changeMetrics, row.Metric) {
		return changePair{}, errors.New("a comparison exists for the size findings of containers, VMs and folder sets")
	}
	item, err := s.breakdownItemByID(domain, row.TargetID)
	if err != nil {
		return changePair{}, err
	}
	to, err := s.store.GetRun(row.RunID)
	if err != nil {
		return changePair{}, err
	}
	if to.SnapshotID == "" {
		return changePair{}, errNoComparison
	}
	var from store.Run
	if row.LastGoodRunID != "" {
		if from, err = s.store.GetRun(row.LastGoodRunID); err != nil {
			return changePair{}, err
		}
	} else {
		series, err := s.store.ItemSeries(row.TargetID, "backup", to.StartedAt-1, anomalyMinSamples)
		if err != nil {
			return changePair{}, err
		}
		for _, run := range series {
			if run.Status == "success" && run.SnapshotID != "" {
				from = store.Run{ID: run.ID, SnapshotID: run.SnapshotID, StartedAt: run.StartedAt}
				break
			}
		}
	}
	if from.SnapshotID == "" {
		return changePair{}, errNoComparison
	}
	return changePair{item: item, from: from, to: to}, nil
}

func (s *Service) compareBackups(pair changePair) {
	c := &s.changes
	c.run.Lock()
	defer c.run.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), breakdownTimeout)
	defer cancel()
	view := ChangesView{FromAt: pair.from.StartedAt, ToAt: pair.to.StartedAt}
	root, summary, err := s.diffBackups(ctx, pair)
	if err != nil {
		log.Printf("api: compare the backups of %s: %v", pair.item.id, err)
		view.State, view.Error = breakdownFailed, scrubError(err)
	} else {
		view.State, view.Root, view.Summary = breakdownReady, s.toHostPath(root), &summary
	}
	c.mu.Lock()
	c.results[pair.key()] = view
	c.mu.Unlock()
}

// diffBackups reads which files differ and then their sizes, from the older
// listing for what went away and from the newer one for what is there now.
func (s *Service) diffBackups(ctx context.Context, pair changePair) (string, ChangeSummary, error) {
	it := pair.item
	kinds := map[string]changeKind{}
	partial := false
	err := s.engine.DiffStream(ctx, it.repo, pair.from.SnapshotID, pair.to.SnapshotID, it.mode, func(c restic.DiffChange) {
		if strings.HasSuffix(c.Path, "/") {
			return
		}
		var kind changeKind
		switch {
		case c.Modifier == "-":
			kind = fileRemoved
		case c.Modifier == "+":
			kind = fileAdded
		case strings.ContainsAny(c.Modifier, "MT"):
			kind = fileModified
		default:
			return
		}
		if len(kinds) >= sizetree.MaxChanged {
			partial = true
			return
		}
		kinds[path.Clean(c.Path)] = kind
	})
	if err != nil {
		return "", ChangeSummary{}, fmt.Errorf("compare the two backups: %w", err)
	}

	snaps, err := s.itemSnapshots(ctx, it)
	if err != nil {
		return "", ChangeSummary{}, fmt.Errorf("list the backups: %w", err)
	}
	root := "/"
	for _, snap := range snaps {
		if snap.ID == pair.to.SnapshotID || strings.HasPrefix(snap.ID, pair.to.SnapshotID) {
			root = sizetree.CommonRoot(snap.Paths)
			break
		}
	}

	changes := make(map[string]fileChange, len(kinds))
	read := func(snapshot string, want func(changeKind) bool) error {
		return s.engine.LsStreamNoLock(ctx, it.repo, snapshot, it.mode, func(e restic.FileEntry) {
			p := path.Clean(e.Path)
			kind, ok := kinds[p]
			if !ok || !want(kind) || e.Type == "dir" {
				return
			}
			changes[p] = fileChange{kind: kind, size: e.Size}
		})
	}
	if err := read(pair.from.SnapshotID, func(k changeKind) bool { return k == fileRemoved }); err != nil {
		return "", ChangeSummary{}, fmt.Errorf("list the last good backup: %w", err)
	}
	if err := read(pair.to.SnapshotID, func(k changeKind) bool { return k != fileRemoved }); err != nil {
		return "", ChangeSummary{}, fmt.Errorf("list the affected backup: %w", err)
	}

	rel := make(map[string]fileChange, len(changes))
	for p, c := range changes {
		rel[strings.TrimPrefix(strings.TrimPrefix(p, root), "/")] = c
	}
	summary := summarizeChanges(rel)
	summary.Partial = partial
	return root, summary, nil
}

// handleAnomalyChanges answers GET /api/anomalies/{id}/changes?retry=1.
func (h *Handler) handleAnomalyChanges(w http.ResponseWriter, r *http.Request) {
	view, err := h.svc.AnomalyChanges(r.PathValue("id"), r.URL.Query().Get("retry") == "1")
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"changes": view}))
}
