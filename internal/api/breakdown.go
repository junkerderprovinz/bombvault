package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"path"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/sizetree"
	"github.com/junkerderprovinz/bombvault/internal/store"
	"github.com/junkerderprovinz/bombvault/internal/zfs"
)

// States of a size breakdown.
const (
	breakdownReady   = "ready"
	breakdownRunning = "running"
	breakdownFailed  = "failed"
	breakdownNone    = "none"
)

// breakdownTimeout bounds one breakdown, the diff and the listing together.
const breakdownTimeout = 10 * time.Minute

// breakdownDomains are the domains the get_size_breakdown tool answers for.
// The route takes flash, config and ZFS as well.
var breakdownDomains = []string{"containers", "vms", "files"}

// errBreakdownDomain refuses a domain without breakdowns.
var errBreakdownDomain = errors.New("no size breakdown exists for this domain")

// breakdowns is the one worker that computes breakdowns, one at a time, so
// several open panels never run several listings against the repository.
type breakdowns struct {
	mu      sync.Mutex
	queue   chan breakdownJob
	pending map[string]bool
	failed  map[string]string
	start   sync.Once
}

type breakdownJob struct{ domain, targetID string }

// breakdownItem is an item with a breakdown: the id its runs carry, its name
// and where its backups are. A ZFS entry's name is its root dataset.
type breakdownItem struct {
	domain, id, name string
	repo             string
	mode             restic.Mode
}

func (s *Service) breakdownItem(domain, item string) (breakdownItem, error) {
	settings, err := s.store.GetSettings()
	if err != nil {
		return breakdownItem{}, fmt.Errorf("read settings: %w", err)
	}
	it := breakdownItem{domain: domain}
	switch domain {
	case "containers":
		if !validResourceName(item) {
			return it, errors.New("invalid container name")
		}
		t, err := s.store.GetTargetByContainer(item)
		if err != nil {
			return it, fmt.Errorf("BombVault does not protect a container called %q", item)
		}
		it.id, it.name = t.ID, t.ContainerName
		it.repo, err = s.containerRepoForName(settings, item, "local")
		if err != nil {
			return it, err
		}
	case "vms":
		v, err := s.store.GetVMTargetByName(item)
		if err != nil {
			return it, fmt.Errorf("BombVault does not protect a VM called %q", item)
		}
		it.id, it.name = v.ID, v.Name
		it.repo, err = s.vmRepoForName(settings, item, "local")
		if err != nil {
			return it, err
		}
	case "files":
		set, err := s.store.GetFileSet(item)
		if err != nil {
			return it, errFileSetNotFound
		}
		it.id, it.name = set.ID, set.Name
		it.repo, err = s.fileSetRepoFor(settings, set, "local")
		if err != nil {
			return it, err
		}
	case "flash", "config":
		it.id, it.name = domainRunTargetID(domain), domain
		it.repo, err = s.repoFor(settings, domain, "local")
		if err != nil {
			return it, err
		}
	case zfsDomain:
		d, err := s.store.GetZFSDataset(item)
		if err != nil {
			return it, errors.New("BombVault has no ZFS entry with this id")
		}
		it.id, it.name = d.ID, d.Dataset
		it.repo, err = s.zfsDatasetRepoPath(settings, d)
		if err != nil {
			return it, err
		}
	default:
		return it, errBreakdownDomain
	}
	it.mode = s.repoModeFor(settings, domain, "local", it.repo)
	return it, nil
}

// breakdownItemByID finds an item by its row id, for the refresh after a
// backup.
func (s *Service) breakdownItemByID(domain, id string) (breakdownItem, error) {
	switch domain {
	case "containers":
		t, err := s.store.GetTargetByID(id)
		if err != nil {
			return breakdownItem{}, err
		}
		return s.breakdownItem(domain, t.ContainerName)
	case "vms":
		v, err := s.store.GetVMTargetByID(id)
		if err != nil {
			return breakdownItem{}, err
		}
		return s.breakdownItem(domain, v.Name)
	}
	return s.breakdownItem(domain, id)
}

func (s *Service) itemSnapshots(ctx context.Context, it breakdownItem) ([]restic.Snapshot, error) {
	switch it.domain {
	case "containers":
		return s.Snapshots(ctx, it.name, "local")
	case "vms":
		return s.SnapshotsVM(ctx, it.name, "local")
	case "flash":
		return s.SnapshotsFlash(ctx, "local")
	case "config":
		return s.SnapshotsConfig(ctx, "local")
	case zfsDomain:
		d, err := s.store.GetZFSDataset(it.id)
		if err != nil {
			return nil, err
		}
		return s.zfsItemSnapshots(ctx, d, "local")
	default:
		return s.SnapshotsFileSet(ctx, it.id, "local")
	}
}

// breakdownPart is one snapshot of the backup a breakdown reads, the snapshot
// it is compared with, and where its tree hangs below the breakdown's root.
type breakdownPart struct {
	snap   restic.Snapshot
	prev   *restic.Snapshot
	prefix string
}

// breakdownBackup is an item's newest backup. key is what its breakdown is
// stored under and label what the panel shows for it: a snapshot id and its
// short form, or the stamp of a ZFS run.
type breakdownBackup struct {
	key, label, time, root string
	parts                  []breakdownPart
}

// newestBackup returns the item's newest backup, compared with the one before
// it, or nil while there is none.
func (s *Service) newestBackup(ctx context.Context, it breakdownItem) (*breakdownBackup, error) {
	snaps, err := s.itemSnapshots(ctx, it)
	if err != nil || len(snaps) == 0 {
		return nil, err
	}
	slices.SortStableFunc(snaps, func(a, b restic.Snapshot) int {
		return parseSnapshotTime(b.Time).Compare(parseSnapshotTime(a.Time))
	})
	if it.domain == zfsDomain {
		return zfsNewestBackup(it.name, snaps), nil
	}
	part := breakdownPart{snap: snaps[0]}
	if len(snaps) > 1 {
		part.prev = &snaps[1]
	}
	return &breakdownBackup{
		key: snaps[0].ID, label: shortID(snaps[0].ID), time: snaps[0].Time,
		root: sizetree.CommonRoot(snaps[0].Paths), parts: []breakdownPart{part},
	}, nil
}

// zfsNewestBackup is the newest run of a ZFS entry out of its snapshots, newest
// first. Every dataset is a part of one tree, each below the root where its
// name puts it, and is compared with its own snapshot of an earlier run.
func zfsNewestBackup(root string, snaps []restic.Snapshot) *breakdownBackup {
	var b *breakdownBackup
	for _, snap := range snaps {
		stamp, stamped := zfsStampOf(snap)
		dataset, member := zfsSnapshotDataset(snap, root)
		if !stamped || !member {
			continue
		}
		if b == nil {
			at, _ := zfs.StampTime(stamp)
			b = &breakdownBackup{key: stamp, label: stamp, time: at.Format(time.RFC3339), root: "/"}
		}
		prefix := zfsRelPath(root, dataset)
		i := slices.IndexFunc(b.parts, func(p breakdownPart) bool { return p.prefix == prefix })
		switch {
		case stamp == b.key && i < 0:
			b.parts = append(b.parts, breakdownPart{snap: snap, prefix: prefix})
		case stamp != b.key && i >= 0 && b.parts[i].prev == nil:
			b.parts[i].prev = &snap
		}
	}
	return b
}

func breakdownKey(targetID, snapshotID string) string { return targetID + "/" + snapshotID }

// enqueueBreakdown asks the worker for the item's newest breakdown, once.
func (s *Service) enqueueBreakdown(domain, targetID string) {
	b := &s.breakdowns
	b.start.Do(func() {
		b.queue = make(chan breakdownJob, 256)
		go s.breakdownWorker()
	})
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.pending == nil {
		b.pending = map[string]bool{}
	}
	if b.pending[targetID] {
		return
	}
	select {
	case b.queue <- breakdownJob{domain: domain, targetID: targetID}:
		b.pending[targetID] = true
	default:
		log.Printf("api: size breakdown: the queue is full, %s waits for the next request", targetID)
	}
}

func (s *Service) breakdownPending(targetID string) bool {
	s.breakdowns.mu.Lock()
	defer s.breakdowns.mu.Unlock()
	return s.breakdowns.pending[targetID]
}

func (s *Service) breakdownWorker() {
	for job := range s.breakdowns.queue {
		key, err := s.computeBreakdown(job)
		s.breakdowns.mu.Lock()
		delete(s.breakdowns.pending, job.targetID)
		if err != nil && key != "" {
			if s.breakdowns.failed == nil {
				s.breakdowns.failed = map[string]string{}
			}
			s.breakdowns.failed[key] = scrubError(err)
		}
		s.breakdowns.mu.Unlock()
		if err != nil {
			log.Printf("api: size breakdown of %s: %v", job.targetID, err)
		}
	}
}

// computeBreakdown lists the item's newest backup and compares it with the
// one before. It returns the key the result belongs to.
func (s *Service) computeBreakdown(job breakdownJob) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), breakdownTimeout)
	defer cancel()
	it, err := s.breakdownItemByID(job.domain, job.targetID)
	if err != nil {
		return "", err
	}
	newest, err := s.newestBackup(ctx, it)
	if err != nil || newest == nil {
		return "", err
	}
	key := breakdownKey(it.id, newest.key)
	if _, ok, err := s.store.GetSizeBreakdown(it.id, newest.key); err != nil || ok {
		return key, err
	}
	row := store.SizeBreakdown{TargetID: it.id, SnapshotID: newest.key, Domain: it.domain, CreatedAt: time.Now().Unix()}
	var changed map[string]struct{}
	note := func(p string) {
		if len(changed) >= sizetree.MaxChanged {
			row.Partial = true
			return
		}
		changed[p] = struct{}{}
	}
	for _, part := range newest.parts {
		if part.prev == nil {
			continue
		}
		if changed == nil {
			row.ParentID = part.prev.ID
			changed = map[string]struct{}{}
		}
		_, err := s.engine.DiffStream(ctx, it.repo, part.prev.ID, part.snap.ID, it.mode, func(c restic.DiffChange) {
			// A folder, a removed file or one whose metadata alone changed
			// adds no data.
			if strings.HasSuffix(c.Path, "/") || !strings.ContainsAny(c.Modifier, "+MT") {
				return
			}
			note(path.Join(part.prefix, c.Path))
		})
		if err != nil {
			return key, fmt.Errorf("compare with the backup before: %w", err)
		}
	}
	b := sizetree.NewBuilder(newest.root, changed)
	for _, part := range newest.parts {
		// A dataset the earlier runs did not hold is new as a whole.
		fresh := changed != nil && part.prev == nil
		err = s.engine.LsStreamNoLock(ctx, it.repo, part.snap.ID, it.mode, func(e restic.FileEntry) {
			p := path.Join(part.prefix, e.Path)
			if fresh && e.Type != "dir" {
				note(p)
			}
			b.Add(p, e.Type, e.Size)
		})
		if err != nil {
			return key, fmt.Errorf("list the backup: %w", err)
		}
	}
	tree, err := json.Marshal(b.Tree())
	if err != nil {
		return key, err
	}
	row.Tree = string(tree)
	return key, s.store.PutSizeBreakdown(row)
}

// refreshBreakdownAfter keeps a breakdown someone asked for current: after
// each successful backup of such an item the new snapshot gets one too.
func (s *Service) refreshBreakdownAfter(runID string) {
	run, err := s.store.GetRun(runID)
	if err != nil || run.Kind != "backup" || run.Status != "success" {
		return
	}
	domain, ok, err := s.store.SizeBreakdownDomain(run.TargetID)
	if err != nil || !ok {
		return
	}
	s.enqueueBreakdown(domain, run.TargetID)
}

// BreakdownEntry is one row of a folder: a subfolder or a file.
type BreakdownEntry struct {
	sizetree.Entry
	// Open says the row is a folder the breakdown goes into.
	Open bool `json:"open,omitempty"`
}

// BreakdownView is one folder of an item's newest backup. A ZFS run shows as
// one tree with every dataset where its name puts it. Size is what its
// files take in the backup before deduplication and compression; Added is
// the part the latest backup brought in new or changed. First means there
// was no earlier backup, so all of it counts as added.
type BreakdownView struct {
	State    string           `json:"state"`
	Error    string           `json:"error,omitempty"`
	Snapshot string           `json:"snapshot,omitempty"`
	Time     string           `json:"time,omitempty"`
	First    bool             `json:"first,omitempty"`
	Partial  bool             `json:"partial,omitempty"`
	Root     string           `json:"root,omitempty"`
	Path     string           `json:"path"`
	Size     int64            `json:"size"`
	Files    int64            `json:"files"`
	Added    int64            `json:"added"`
	Children []BreakdownEntry `json:"children"`
	Other    *sizetree.Rest   `json:"other,omitempty"`
}

// SizeBreakdown answers for one folder of the item's newest backup. Without
// a breakdown for it yet, it starts one and says so; retry asks again after
// one failed.
func (s *Service) SizeBreakdown(ctx context.Context, domain, item, rel string, retry bool) (BreakdownView, error) {
	view := BreakdownView{Path: rel, Children: []BreakdownEntry{}}
	it, err := s.breakdownItem(domain, item)
	if err != nil {
		return view, err
	}
	if s.breakdownPending(it.id) {
		view.State = breakdownRunning
		return view, nil
	}
	newest, err := s.newestBackup(ctx, it)
	if err != nil {
		return view, err
	}
	if newest == nil {
		view.State = breakdownNone
		return view, nil
	}
	view.Snapshot, view.Time = newest.label, newest.time
	row, ok, err := s.store.GetSizeBreakdown(it.id, newest.key)
	if err != nil {
		return view, err
	}
	if !ok {
		key := breakdownKey(it.id, newest.key)
		s.breakdowns.mu.Lock()
		msg, failed := s.breakdowns.failed[key]
		if failed && retry {
			delete(s.breakdowns.failed, key)
		}
		s.breakdowns.mu.Unlock()
		if failed && !retry {
			view.State, view.Error = breakdownFailed, msg
			return view, nil
		}
		s.enqueueBreakdown(domain, it.id)
		view.State = breakdownRunning
		return view, nil
	}
	var tree sizetree.Tree
	if err := json.Unmarshal([]byte(row.Tree), &tree); err != nil {
		return view, fmt.Errorf("read the stored breakdown: %w", err)
	}
	node, ok := tree.Nodes[rel]
	if !ok {
		return view, fmt.Errorf("the breakdown does not go into %q", rel)
	}
	view.State, view.First, view.Partial = breakdownReady, row.ParentID == "", row.Partial
	view.Root = s.toHostPath(tree.Root)
	if domain == zfsDomain {
		view.Root = path.Join(it.name, tree.Root)
	}
	view.Size, view.Files, view.Added, view.Other = node.Size, node.Files, node.Added, node.Other
	for _, c := range node.Children {
		_, open := tree.Nodes[joinRel(rel, c.Name)]
		view.Children = append(view.Children, BreakdownEntry{Entry: c, Open: c.Dir && open})
	}
	return view, nil
}

func joinRel(a, b string) string {
	if a == "" {
		return b
	}
	return a + "/" + b
}

// handleSizeBreakdown answers GET /api/breakdown?domain=&item=&path=&retry=1.
// item is the name of a container or VM, the id of a folder set or ZFS entry,
// and nothing for flash and config; path is a folder of the breakdown, empty
// for its top.
func (h *Handler) handleSizeBreakdown(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	view, err := h.svc.SizeBreakdown(r.Context(), q.Get("domain"), q.Get("item"), q.Get("path"), q.Get("retry") == "1")
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"breakdown": view}))
}

type sizeBreakdownInput struct {
	Domain string `json:"domain"`
	Item   string `json:"item"`
	Path   string `json:"path"`
	Retry  bool   `json:"retry"`
}

func (h *Handler) toolGetSizeBreakdown(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if _, ok := mcpCallerFrom(ctx); !ok {
		return mcpNoCaller(), nil
	}
	var in sizeBreakdownInput
	if err := decodeMCPArgs(req.Params.Arguments, &in); err != nil {
		h.logMCPCall(ctx, "get_size_breakdown", "invalid_argument")
		return mcpToolError("invalid_argument", err.Error(), nil), nil
	}
	if !slices.Contains(breakdownDomains, in.Domain) {
		h.logMCPCall(ctx, "get_size_breakdown", "invalid_argument")
		return mcpToolError("invalid_argument", "a size breakdown exists for the domains "+strings.Join(breakdownDomains, ", "), nil), nil
	}
	item, bad := h.resolveMCPItem(in.Domain, in.Item)
	if bad != nil {
		h.logMCPCall(ctx, "get_size_breakdown", mcpErrorCodeOf(bad))
		return bad, nil
	}
	key := item.Name
	if item.Domain == "files" {
		key = item.ID
	}
	release, free := h.mcp.acquireList()
	if !free {
		h.logMCPCall(ctx, "get_size_breakdown", "busy")
		return mcpToolError("busy", "another repository listing is still running; try again in a moment", nil), nil
	}
	defer release()
	ctx, cancel := h.mcpToolContext(ctx, mcpResticTimeout)
	defer cancel()
	view, err := h.svc.SizeBreakdown(ctx, in.Domain, key, strings.Trim(in.Path, "/"), in.Retry)
	if err != nil {
		return h.mcpFailure(ctx, "get_size_breakdown", err), nil
	}
	view.Root = ""
	h.logMCPCall(ctx, "get_size_breakdown", "ok")
	return mcpOK(map[string]any{
		"domain":    item.Domain,
		"item":      map[string]any{"id": item.ID, "name": item.Name},
		"breakdown": view,
	}), nil
}
