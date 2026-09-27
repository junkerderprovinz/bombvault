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

// breakdownDomains are the domains whose items have a breakdown. ZFS already
// has its sizes per dataset; flash and the configuration are one small folder.
var breakdownDomains = []string{"containers", "vms", "files"}

// errBreakdownDomain refuses a domain without breakdowns.
var errBreakdownDomain = errors.New("a size breakdown exists for containers, VMs and folder sets")

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

// breakdownItem is an item with a breakdown: its row id, its name and where
// its backups are.
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

// newestSnapshots returns the item's newest backup and the one before it.
func (s *Service) newestSnapshots(ctx context.Context, it breakdownItem) (newest, prev *restic.Snapshot, err error) {
	var snaps []restic.Snapshot
	switch it.domain {
	case "containers":
		snaps, err = s.Snapshots(ctx, it.name, "local")
	case "vms":
		snaps, err = s.SnapshotsVM(ctx, it.name, "local")
	default:
		snaps, err = s.SnapshotsFileSet(ctx, it.id, "local")
	}
	if err != nil || len(snaps) == 0 {
		return nil, nil, err
	}
	slices.SortStableFunc(snaps, func(a, b restic.Snapshot) int {
		return parseSnapshotTime(b.Time).Compare(parseSnapshotTime(a.Time))
	})
	newest = &snaps[0]
	if len(snaps) > 1 {
		prev = &snaps[1]
	}
	return newest, prev, nil
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
	newest, prev, err := s.newestSnapshots(ctx, it)
	if err != nil || newest == nil {
		return "", err
	}
	key := breakdownKey(it.id, newest.ID)
	if _, ok, err := s.store.GetSizeBreakdown(it.id, newest.ID); err != nil || ok {
		return key, err
	}
	row := store.SizeBreakdown{TargetID: it.id, SnapshotID: newest.ID, Domain: it.domain, CreatedAt: time.Now().Unix()}
	var changed map[string]struct{}
	if prev != nil {
		row.ParentID = prev.ID
		changed = map[string]struct{}{}
		err := s.engine.DiffStream(ctx, it.repo, prev.ID, newest.ID, it.mode, func(c restic.DiffChange) {
			// A folder, a removed file or one whose metadata alone changed
			// adds no data.
			if strings.HasSuffix(c.Path, "/") || !strings.ContainsAny(c.Modifier, "+MT") {
				return
			}
			if len(changed) >= sizetree.MaxChanged {
				row.Partial = true
				return
			}
			changed[path.Clean(c.Path)] = struct{}{}
		})
		if err != nil {
			return key, fmt.Errorf("compare with the backup before: %w", err)
		}
	}
	b := sizetree.NewBuilder(sizetree.CommonRoot(newest.Paths), changed)
	err = s.engine.LsStreamNoLock(ctx, it.repo, newest.ID, it.mode, func(e restic.FileEntry) {
		b.Add(e.Path, e.Type, e.Size)
	})
	if err != nil {
		return key, fmt.Errorf("list the backup: %w", err)
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

// BreakdownView is one folder of an item's newest backup. Size is what its
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
	if !slices.Contains(breakdownDomains, domain) {
		return view, errBreakdownDomain
	}
	it, err := s.breakdownItem(domain, item)
	if err != nil {
		return view, err
	}
	if s.breakdownPending(it.id) {
		view.State = breakdownRunning
		return view, nil
	}
	newest, _, err := s.newestSnapshots(ctx, it)
	if err != nil {
		return view, err
	}
	if newest == nil {
		view.State = breakdownNone
		return view, nil
	}
	view.Snapshot, view.Time = shortID(newest.ID), newest.Time
	row, ok, err := s.store.GetSizeBreakdown(it.id, newest.ID)
	if err != nil {
		return view, err
	}
	if !ok {
		key := breakdownKey(it.id, newest.ID)
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
// item is the name of a container or VM and the id of a folder set; path is
// a folder of the breakdown, empty for its top.
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
