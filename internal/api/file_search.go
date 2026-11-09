package api

import (
	"cmp"
	"context"
	"errors"
	"log"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/sizetree"
)

// maxFoundFiles is how many files one search lists. A name that matches more
// is too broad to pick a file from.
const maxFoundFiles = 200

// fileSearchItem is one entry a search reads, by its timeline domain and key.
type fileSearchItem struct {
	domain string
	key    string
}

// fileSearchDomains maps the kinds of entry whose files can be searched to
// their timeline domain. A VM is saved as whole disks, and the self-backup has
// a way back of its own.
var fileSearchDomains = map[string]string{
	"container": "containers",
	"files":     "files",
	zfsDomain:   zfsDomain,
	"flash":     "flash",
}

// foundVersion is one backup that holds a found file. At is the index of the
// searched place it was read at.
type foundVersion struct {
	At       int    `json:"at"`
	Row      string `json:"row"`
	Time     string `json:"time"`
	Place    string `json:"place"`
	Snapshot string `json:"snapshot"`
	Size     int64  `json:"size"`
	Modified string `json:"modified"`
}

// foundFile is a file or folder of one entry and every backup holding it,
// newest first. Dataset names the ZFS dataset whose backup holds the path, and
// root is the folder the backup was taken of.
type foundFile struct {
	Domain   string         `json:"domain"`
	Key      string         `json:"key"`
	Dataset  string         `json:"dataset,omitempty"`
	Path     string         `json:"path"`
	Type     string         `json:"type"`
	Versions []foundVersion `json:"versions"`
	root     string
}

// searchedPlace is one repository and how the search went there. State is
// "searched", "covered" when every backup it holds was read at a place
// before it, "notAsked" for a remote one the caller did not ask for, or
// "failed". Domain names the section whose folder an empty Label stands for.
type searchedPlace struct {
	Label    string `json:"label"`
	DirectOf string `json:"directOf,omitempty"`
	Domain   string `json:"domain"`
	Remote   bool   `json:"remote"`
	State    string `json:"state"`
	Backups  int    `json:"backups"`
	Error    string `json:"error,omitempty"`
}

type fileSearch struct {
	Files  []foundFile     `json:"files"`
	Places []searchedPlace `json:"places"`
	More   bool            `json:"more"`
}

// searchSnap is what a snapshot handed to restic find stands for.
type searchSnap struct {
	item    fileSearchItem
	row     string
	time    string
	place   string
	dataset string
	root    string
}

// searchRepo is one repository the search reads and the snapshots it reads
// there.
type searchRepo struct {
	place  searchedPlace
	repo   string
	mode   restic.Mode
	listed []restic.Snapshot
	ids    []string
	snaps  map[string]searchSnap
}

func (r *searchRepo) add(id string, sn searchSnap) {
	r.ids = append(r.ids, id)
	r.snaps[id] = sn
	r.place.State, r.place.Backups = "searched", len(r.ids)
}

// SearchFiles looks for files and folders whose name matches text in every
// backup of the items. Each backup is read once, at the first place holding
// it, places on the premises before remote ones. A remote place is read only
// when offsite is set, and then only for the backups no other place holds,
// since a remote search can take long and its provider may charge for reads.
func (s *Service) SearchFiles(ctx context.Context, items []fileSearchItem, text string, offsite bool) (fileSearch, error) {
	pattern, err := restic.FileNamePattern(text)
	if err != nil {
		return fileSearch{}, err
	}
	repos, err := s.searchRepos(ctx, items, offsite)
	if err != nil {
		return fileSearch{}, err
	}
	found := foundFiles{index: map[string]int{}}
	for at, r := range repos {
		if r.place.State != "searched" {
			continue
		}
		err := s.engine.Find(ctx, r.repo, r.ids, pattern, r.mode, func(id string, files []restic.FoundFile) {
			found.add(at, r.snaps[id], id, files)
		})
		if ctx.Err() != nil {
			return fileSearch{}, context.Cause(ctx)
		}
		if err != nil {
			r.place.State, r.place.Error = "failed", scrubError(err)
		}
	}
	res := fileSearch{Files: found.sorted(), Places: make([]searchedPlace, 0, len(repos)), More: found.more}
	for _, r := range repos {
		res.Places = append(res.Places, r.place)
	}
	return res, nil
}

// searchRepos works out which repository each backup of the items is read in.
func (s *Service) searchRepos(ctx context.Context, items []fileSearchItem, offsite bool) ([]*searchRepo, error) {
	var repos []*searchRepo
	byKey := map[string]*searchRepo{}
	for _, item := range items {
		it, refs, err := s.timelineRefs(item.domain, item.key)
		if err != nil {
			log.Printf("api: file search: %s %q left out: %v", item.domain, item.key, err) //nolint:gosec // G706: the domain is a fixed literal and the key is quoted
			continue
		}
		slices.SortStableFunc(refs, func(a, b placeRef) int { return compareBool(a.Remote, b.Remote) })
		read := map[string]bool{}
		for _, p := range refs {
			key := searchRepoKey(item.domain, p)
			r, ok := byKey[key]
			if !ok {
				r = s.openSearchRepo(ctx, item.domain, p, offsite)
				byKey[key] = r
				repos = append(repos, r)
			}
			if r.place.State == "notAsked" || r.place.State == "failed" {
				continue
			}
			own, err := s.ownedAmong(it, r.listed)
			if err != nil {
				return nil, err
			}
			byID := make(map[string]restic.Snapshot, len(own))
			for _, sn := range own {
				byID[sn.ID] = sn
			}
			for _, row := range rowsAt(it, p.Place, own) {
				if read[row.Key] {
					continue
				}
				read[row.Key] = true
				ids := row.Places[0].SnapshotIDs
				// Only a ZFS run holds more than one tree; two ids of any
				// other row are the same backup copied twice.
				if it.zfsRoot == "" {
					ids = ids[:1]
				}
				for _, id := range ids {
					// A ZFS backup holds its dataset's files from the top.
					sn := searchSnap{item: item, row: row.Key, time: row.Time, place: p.Place, root: "/"}
					if it.zfsRoot != "" {
						sn.dataset, _ = zfsSnapshotDataset(byID[id], it.zfsRoot)
					} else {
						sn.root = sizetree.CommonRoot(byID[id].Paths)
					}
					r.add(id, sn)
				}
			}
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, context.Cause(ctx)
	}
	slices.SortStableFunc(repos, func(a, b *searchRepo) int { return compareBool(a.place.Remote, b.place.Remote) })
	return repos, nil
}

// openSearchRepo starts the search's view of one repository and lists it,
// unless it is remote and the caller did not ask for remote places.
func (s *Service) openSearchRepo(ctx context.Context, domain string, p placeRef, offsite bool) *searchRepo {
	r := &searchRepo{
		place: searchedPlace{Label: p.Label, DirectOf: p.DirectOf, Domain: domain, Remote: p.Remote, State: "covered"},
		repo:  p.repo,
		mode:  p.mode,
		snaps: map[string]searchSnap{},
	}
	switch {
	case p.openErr != nil:
		r.place.State, r.place.Error = "failed", scrubError(p.openErr)
	case p.Remote && !offsite:
		r.place.State = "notAsked"
	default:
		listed, err := s.listRepo(ctx, p.repo, p.mode)
		if err != nil {
			r.place.State, r.place.Error = "failed", scrubError(err)
		}
		r.listed = listed
	}
	return r
}

// searchRepoKey tells repositories apart by location and the access that
// opens them. A place whose location does not resolve stands alone.
func searchRepoKey(domain string, p placeRef) string {
	if p.openErr != nil {
		return strings.Join([]string{"unresolved", domain, p.Place, p.Label}, "\x00")
	}
	return p.repo + "\x00" + repoAccessKey(p.mode)
}

func compareBool(a, b bool) int {
	switch {
	case a == b:
		return 0
	case a:
		return 1
	default:
		return -1
	}
}

// foundFiles collects the matches of a search by entry, dataset and path.
type foundFiles struct {
	files []foundFile
	index map[string]int
	more  bool
}

func (f *foundFiles) add(at int, sn searchSnap, snapshotID string, files []restic.FoundFile) {
	for _, file := range files {
		key := strings.Join([]string{sn.item.domain, sn.item.key, sn.dataset, file.Path}, "\x00")
		i, ok := f.index[key]
		if !ok {
			if len(f.files) == maxFoundFiles {
				f.more = true
				continue
			}
			i = len(f.files)
			f.index[key] = i
			f.files = append(f.files, foundFile{Domain: sn.item.domain, Key: sn.item.key, Dataset: sn.dataset, Path: file.Path, Type: file.Type, root: sn.root})
		}
		f.files[i].Versions = append(f.files[i].Versions, foundVersion{
			At:       at,
			Row:      sn.row,
			Time:     sn.time,
			Place:    sn.place,
			Snapshot: snapshotID,
			Size:     file.Size,
			Modified: file.ModTime.UTC().Format(time.RFC3339),
		})
	}
}

// sorted returns the files by entry and path, each with its newest backup
// first.
func (f *foundFiles) sorted() []foundFile {
	out := slices.Clone(f.files)
	if out == nil {
		out = []foundFile{}
	}
	for _, file := range out {
		slices.SortStableFunc(file.Versions, func(a, b foundVersion) int { return cmp.Compare(unixOf(b.Time), unixOf(a.Time)) })
	}
	slices.SortFunc(out, func(a, b foundFile) int {
		return cmp.Or(cmp.Compare(a.Domain, b.Domain), cmp.Compare(a.Key, b.Key), cmp.Compare(a.Dataset, b.Dataset), cmp.Compare(a.Path, b.Path))
	})
	return out
}

// fileSearchEntries are the entries the Restore page lists whose files can be
// searched, and the name of each.
func (h *Handler) fileSearchEntries(ctx context.Context) ([]fileSearchItem, map[fileSearchItem]string, error) {
	settings, err := h.store.GetSettings()
	if err != nil {
		return nil, nil, err
	}
	listed, _, err := h.backupItems(ctx, settings)
	if err != nil {
		return nil, nil, err
	}
	var items []fileSearchItem
	names := map[fileSearchItem]string{}
	for _, item := range listed {
		if domain, ok := fileSearchDomains[item.Kind]; ok {
			it := fileSearchItem{domain: domain, key: item.Key}
			items = append(items, it)
			names[it] = item.Name
		}
	}
	return items, names, nil
}

// handleFileSearch serves GET /api/search/files?q=<name>&offsite=1: the files
// whose name matches q in the backups of every entry the Restore page lists.
func (h *Handler) handleFileSearch(w http.ResponseWriter, r *http.Request) {
	items, _, err := h.fileSearchEntries(r.Context())
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	q := r.URL.Query()
	res, err := h.svc.SearchFiles(r.Context(), items, q.Get("q"), q.Get("offsite") == "1")
	if errors.Is(err, restic.ErrFileNamePattern) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": err.Error(), "code": "bad-file-name"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"files": res.Files, "places": res.Places, "more": res.More}))
}

// States of a file search running in the background.
const (
	fileSearchRunning = "running"
	fileSearchReady   = "ready"
	fileSearchFailed  = "failed"
)

// fileSearchTimeout bounds one background search, every place together.
const fileSearchTimeout = 10 * time.Minute

// errFileSearchBusy refuses a search while another one runs.
var errFileSearchBusy = errors.New("another file search is still running; try again when it has finished")

// fileSearches is the one search MCP runs at a time. A search over every
// backup can take minutes, longer than a tool call may wait, so the first call
// starts it and a later one with the same question collects the result.
type fileSearches struct {
	mu      sync.Mutex
	key     string
	running bool
	done    *fileSearchView
}

// fileSearchView is how a background search stands. Error says why a failed
// one failed.
type fileSearchView struct {
	State  string
	Error  string
	Result fileSearch
}

// SearchFilesInBackground is SearchFiles without the wait. The first call
// starts the search and answers running. Once it is done, the next call with
// the same question gets the result, and the call after that searches again.
func (s *Service) SearchFilesInBackground(items []fileSearchItem, text string, offsite bool) (fileSearchView, error) {
	pattern, err := restic.FileNamePattern(text)
	if err != nil {
		return fileSearchView{}, err
	}
	key := fileSearchKey(items, pattern, offsite)
	j := &s.fileSearches
	j.mu.Lock()
	defer j.mu.Unlock()
	switch {
	case j.running && j.key != key:
		return fileSearchView{}, errFileSearchBusy
	case j.running:
		return fileSearchView{State: fileSearchRunning}, nil
	case j.done != nil && j.key == key:
		view := *j.done
		j.done = nil
		return view, nil
	}
	j.key, j.running, j.done = key, true, nil
	go s.runFileSearch(items, text, offsite)
	return fileSearchView{State: fileSearchRunning}, nil
}

func (s *Service) runFileSearch(items []fileSearchItem, text string, offsite bool) {
	ctx, cancel := context.WithTimeout(s.StopContext(), fileSearchTimeout)
	defer cancel()
	res, err := s.SearchFiles(ctx, items, text, offsite)
	view := fileSearchView{State: fileSearchReady, Result: res}
	if err != nil {
		log.Printf("api: file search: %v", err)
		view = fileSearchView{State: fileSearchFailed, Error: scrubError(err)}
	}
	s.fileSearches.mu.Lock()
	s.fileSearches.running, s.fileSearches.done = false, &view
	s.fileSearches.mu.Unlock()
}

// fileSearchKey tells two questions apart: the pattern, whether remote places
// are read, and the items in any order.
func fileSearchKey(items []fileSearchItem, pattern string, offsite bool) string {
	keys := make([]string, 0, len(items))
	for _, it := range items {
		keys = append(keys, it.domain+"/"+it.key)
	}
	slices.Sort(keys)
	return strings.Join(append([]string{pattern, strconv.FormatBool(offsite)}, keys...), "\x00")
}

// fileSearchMCPDomains are the domains search_files reads. A VM is saved as
// whole disks, and the self-backup has a way back of its own.
var fileSearchMCPDomains = []string{"containers", "files", zfsDomain, "flash"}

type searchFilesInput struct {
	Name    string `json:"name"`
	Domain  string `json:"domain"`
	Item    string `json:"item"`
	Offsite bool   `json:"offsite"`
}

// mcpFoundFile is a match as an assistant reads it. Path is inside the backup
// of the item, so no host path leaves.
type mcpFoundFile struct {
	Domain   string            `json:"domain"`
	Item     string            `json:"item"`
	Dataset  string            `json:"dataset,omitempty"`
	Path     string            `json:"path"`
	Type     string            `json:"type"`
	Versions []mcpFoundVersion `json:"versions"`
}

type mcpFoundVersion struct {
	Time       string `json:"time"`
	Modified   string `json:"modified"`
	Size       int64  `json:"size"`
	Snapshot   string `json:"snapshot"`
	Repository string `json:"repository"`
}

type mcpSearchedRepo struct {
	Name    string `json:"name"`
	Domain  string `json:"domain"`
	Remote  bool   `json:"remote"`
	State   string `json:"state"`
	Backups int    `json:"backups"`
	Error   string `json:"error,omitempty"`
}

func (h *Handler) toolSearchFiles(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if _, ok := mcpCallerFrom(ctx); !ok {
		return mcpNoCaller(), nil
	}
	refuse := func(msg string) (*mcp.CallToolResult, error) {
		h.logMCPCall(ctx, "search_files", "invalid_argument")
		return mcpToolError("invalid_argument", msg, nil), nil
	}
	var in searchFilesInput
	if err := decodeMCPArgs(req.Params.Arguments, &in); err != nil {
		return refuse(err.Error())
	}
	switch {
	case in.Domain != "" && !slices.Contains(fileSearchMCPDomains, in.Domain):
		return refuse("files can be searched in the domains " + strings.Join(fileSearchMCPDomains, ", "))
	case in.Item != "" && in.Domain == "":
		return refuse("item needs the domain it belongs to")
	}
	ctx, cancel := h.mcpToolContext(ctx, mcpReadTimeout)
	defer cancel()

	var items []fileSearchItem
	names := map[fileSearchItem]string{}
	if in.Item != "" {
		item, bad := h.resolveMCPItem(in.Domain, in.Item)
		if bad != nil {
			h.logMCPCall(ctx, "search_files", mcpErrorCodeOf(bad))
			return bad, nil
		}
		key := item.Name
		if item.Domain == "files" || item.Domain == zfsDomain {
			key = item.ID
		}
		one := fileSearchItem{domain: item.Domain, key: key}
		items, names[one] = []fileSearchItem{one}, item.Name
	} else {
		all, allNames, err := h.fileSearchEntries(ctx)
		if err != nil {
			return h.mcpFailure(ctx, "search_files", err), nil
		}
		for _, it := range all {
			if in.Domain == "" || it.domain == in.Domain {
				items = append(items, it)
			}
		}
		names = allNames
	}

	view, err := h.svc.SearchFilesInBackground(items, in.Name, in.Offsite)
	switch {
	case errors.Is(err, restic.ErrFileNamePattern):
		return refuse(err.Error())
	case errors.Is(err, errFileSearchBusy):
		h.logMCPCall(ctx, "search_files", "busy")
		return mcpToolError("busy", err.Error(), nil), nil
	case err != nil:
		return h.mcpFailure(ctx, "search_files", err), nil
	}
	h.logMCPCall(ctx, "search_files", "ok")
	return mcpOK(mcpFileSearchOf(view, names)), nil
}

// mcpFileSearchOf is a background search as an assistant reads it. A match
// above the folder a backup was taken of is a parent folder that happens to
// carry the name, and is left out.
func mcpFileSearchOf(view fileSearchView, names map[fileSearchItem]string) map[string]any {
	out := map[string]any{"state": view.State}
	switch view.State {
	case fileSearchRunning:
		return out
	case fileSearchFailed:
		out["error"] = mcpScrubText(view.Error)
		return out
	}
	repos := make([]mcpSearchedRepo, 0, len(view.Result.Places))
	for _, p := range view.Result.Places {
		name := "folder " + p.Domain
		if p.Label != "" {
			name = mcpScrubText(p.Label)
		}
		repos = append(repos, mcpSearchedRepo{
			Name: name, Domain: p.Domain, Remote: p.Remote, State: p.State, Backups: p.Backups, Error: mcpScrubText(p.Error),
		})
	}
	files := make([]mcpFoundFile, 0, len(view.Result.Files))
	for _, f := range view.Result.Files {
		rel, ok := pathBelow(f.root, f.Path)
		if !ok {
			continue
		}
		versions := make([]mcpFoundVersion, 0, len(f.Versions))
		for _, v := range f.Versions {
			versions = append(versions, mcpFoundVersion{
				Time: v.Time, Modified: v.Modified, Size: v.Size, Snapshot: v.Snapshot, Repository: repos[v.At].Name,
			})
		}
		files = append(files, mcpFoundFile{
			Domain: f.Domain, Item: names[fileSearchItem{f.Domain, f.Key}], Dataset: f.Dataset,
			Path: rel, Type: f.Type, Versions: versions,
		})
	}
	out["files"], out["truncated"], out["repositories"] = files, view.Result.More, repos
	return out
}

// pathBelow is p relative to root, and false for root itself and anything
// outside it.
func pathBelow(root, p string) (string, bool) {
	rel, ok := strings.CutPrefix(p, strings.TrimSuffix(root, "/")+"/")
	return rel, ok && rel != ""
}
