package api

import (
	"cmp"
	"context"
	"errors"
	"log"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/restic"
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
// newest first. Dataset names the ZFS dataset whose backup holds the path.
type foundFile struct {
	Domain   string         `json:"domain"`
	Key      string         `json:"key"`
	Dataset  string         `json:"dataset,omitempty"`
	Path     string         `json:"path"`
	Type     string         `json:"type"`
	Versions []foundVersion `json:"versions"`
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
					sn := searchSnap{item: item, row: row.Key, time: row.Time, place: p.Place}
					if it.zfsRoot != "" {
						sn.dataset, _ = zfsSnapshotDataset(byID[id], it.zfsRoot)
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
			f.files = append(f.files, foundFile{Domain: sn.item.domain, Key: sn.item.key, Dataset: sn.dataset, Path: file.Path, Type: file.Type})
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

// handleFileSearch serves GET /api/search/files?q=<name>&offsite=1: the files
// whose name matches q in the backups of every entry the Restore page lists.
func (h *Handler) handleFileSearch(w http.ResponseWriter, r *http.Request) {
	settings, err := h.store.GetSettings()
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	listed, _, err := h.backupItems(r.Context(), settings)
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	var items []fileSearchItem
	for _, item := range listed {
		if domain, ok := fileSearchDomains[item.Kind]; ok {
			items = append(items, fileSearchItem{domain: domain, key: item.Key})
		}
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
