package api

import (
	"errors"
	"fmt"
	"io/fs"
	"log"
	"maps"
	"net/http"
	"os"
	"sort"
	"strings"

	"github.com/junkerderprovinz/bombvault/internal/paths"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// browseDirEntry is a single subdirectory entry in the browse response.
type browseDirEntry struct {
	Name string `json:"name"`
	Path string `json:"path"` // relative to HostMountRoot (e.g. "appdata/plex")
}

// maxBrowseEntries caps one browse listing, because an appdata tree can hold
// tens of thousands of entries and the SPA would parse megabytes of JSON to draw
// one level. The first entries in sorted order are kept, and the "truncated"
// flag tells the tree there are more.
const maxBrowseEntries = 500

// classifyReadDirError maps a browse read failure to the listing's `status`, so
// the tree can tell a vanished node ("missing") from an unreadable one
// ("restricted") without the path leaving the server. Anything else, an os.Root
// escape included, is plain "error", so an attempted escape looks like any
// other failure.
func classifyReadDirError(err error) string {
	switch {
	case errors.Is(err, fs.ErrPermission):
		return "restricted"
	case errors.Is(err, fs.ErrNotExist):
		return "missing"
	default:
		return "error"
	}
}

// handleBrowse serves GET /api/browse?path=<subpath>[&hidden=1], listing the
// subdirectories of <HostMountRoot>/<subpath> sorted by name and capped at
// maxBrowseEntries. Dot-prefixed entries appear only with hidden=1. An empty
// path lists the mount root.
//
// paths.Resolve rejects a traversal lexically, and os.Root then enforces the
// same boundary at open time, so a symlink inside the mount root cannot list a
// location outside it. Every outcome is an HTTP 200 with a "status"; an empty
// list with status "ok" means the folder is empty.
func (h *Handler) handleBrowse(w http.ResponseWriter, r *http.Request) {
	subpath := r.URL.Query().Get("path")
	// Only the literal "1" opts in, so an empty or mistyped value cannot.
	includeHidden := r.URL.Query().Get("hidden") == "1"

	// paths.Resolve needs a non-empty child, and only its verdict is used;
	// Root.Open below does the resolving.
	if subpath != "" {
		if _, err := paths.Resolve(h.cfg.HostMountRoot, subpath); err != nil {
			// No "status" field here: the FolderBrowser relies on this exact
			// response.
			writeJSON(w, http.StatusOK, map[string]any{
				"ok":    false,
				"error": "invalid path: must be a relative subpath under the mount root",
			})
			return
		}
	}

	// paths.Resolve cannot see a symlink inside the mount root that points
	// outside it; os.Root checks every component at open time.
	root, err := os.OpenRoot(h.cfg.HostMountRoot)
	if err != nil {
		log.Printf("api: browse: OpenRoot: %v", err)
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":     false,
			"error":  "could not read directory",
			"status": classifyReadDirError(err),
		})
		return
	}
	defer root.Close() //nolint:errcheck // read-only browse descriptor: close error is not actionable

	// "." names the root itself.
	rel := subpath
	if rel == "" {
		rel = "."
	}
	f, err := root.Open(rel)
	if err != nil {
		log.Printf("api: browse: open %q: %v", rel, err) //nolint:gosec // G706: rel comes from the query but passed paths.Resolve, and %q escapes control characters
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":     false,
			"error":  "could not read directory",
			"status": classifyReadDirError(err),
		})
		return
	}
	defer f.Close() //nolint:errcheck // read-only browse descriptor: close error is not actionable

	entries, err := f.ReadDir(-1)
	if err != nil {
		log.Printf("api: browse: ReadDir %q: %v", rel, err) //nolint:gosec // G706: rel comes from the query but passed paths.Resolve, and %q escapes control characters
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":     false,
			"error":  "could not read directory",
			"status": classifyReadDirError(err),
		})
		return
	}

	dirs := make([]browseDirEntry, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		if !includeHidden && strings.HasPrefix(name, ".") {
			continue // skip hidden entries unless hidden=1 opted in
		}
		var entryPath string
		if subpath == "" {
			entryPath = name
		} else {
			entryPath = subpath + "/" + name
		}
		dirs = append(dirs, browseDirEntry{Name: name, Path: entryPath})
	}

	// Sorted after filtering, so a capped listing is a stable prefix of the
	// full one. Hidden entries go last: a dot sorts before letters, and they
	// would otherwise push ordinary folders off the end of a capped page.
	sort.Slice(dirs, func(i, j int) bool {
		hi, hj := strings.HasPrefix(dirs[i].Name, "."), strings.HasPrefix(dirs[j].Name, ".")
		if hi != hj {
			return hj
		}
		return dirs[i].Name < dirs[j].Name
	})

	truncated := false
	if len(dirs) > maxBrowseEntries {
		dirs = dirs[:maxBrowseEntries]
		truncated = true
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"ok":        true,
		"root":      h.cfg.HostMountRoot,
		"path":      subpath,
		"dirs":      dirs,
		"status":    "ok",
		"truncated": truncated,
	})
}

// mkdirRequest is the JSON body for POST /api/browse/mkdir.
type mkdirRequest struct {
	Path string `json:"path"` // parent subpath under HostMountRoot ("" = the root)
	Name string `json:"name"` // new folder name, a single plain path component
}

// handleMkdir serves POST /api/browse/mkdir: it creates a new folder <name>
// inside the browsed directory <path> (both under HostMountRoot) so the folder
// picker can make a fresh backup destination without leaving the app. The new
// folder is created operator-readable (0755, like every backup/restore target on
// a user-visible share).
func (h *Handler) handleMkdir(w http.ResponseWriter, r *http.Request) {
	var req mkdirRequest
	if !decodeBody(w, r, &req) {
		return
	}
	name := strings.TrimSpace(req.Name)
	// One plain path component: no nested path, no traversal, and no leading
	// dot, which the browser would hide again.
	if name == "" || name == "." || name == ".." ||
		strings.ContainsAny(name, "/\\\x00") || strings.HasPrefix(name, ".") {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": "invalid folder name"})
		return
	}
	sub := name
	if req.Path != "" {
		sub = req.Path + "/" + name
	}
	abs, err := paths.Resolve(h.cfg.HostMountRoot, sub)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":    false,
			"error": "invalid path: must be a relative subpath under the mount root",
		})
		return
	}
	// Mkdir rather than MkdirAll, so an existing folder is reported instead of
	// reused; the Chmod undoes a strict umask.
	if err := os.Mkdir(abs, 0o755); err != nil { //nolint:gosec // G301: a backup destination on a user-visible share must be operator-readable
		if os.IsExist(err) {
			writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": "a folder with that name already exists"})
			return
		}
		log.Printf("api: mkdir %q: %v", abs, err) //nolint:gosec // G706: abs is a Resolve-validated child path; no raw user bytes reach the log formatter
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": "could not create folder"})
		return
	}
	_ = os.Chmod(abs, 0o755) //nolint:gosec // G302: must be readable by the non-root share user
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "path": sub, "name": name})
}

// fileSetIDParam extracts and validates the {id} path value like nameParam.
// Set ids are 32 hex characters, which the container name charset covers.
func (h *Handler) fileSetIDParam(w http.ResponseWriter, r *http.Request) (string, bool) {
	id := r.PathValue("id")
	if !validResourceName(id) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "invalid file set id"})
		return "", false
	}
	return id, true
}

// handleListFileSets lists all configured file sets with last-backup time and
// source-path existence. GET /api/files
func (h *Handler) handleListFileSets(w http.ResponseWriter, r *http.Request) {
	views, err := h.svc.ListFileSetViews(r.Context())
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	if views == nil {
		views = []FileSetView{}
	}
	sets, _ := h.store.ListFileSets()
	byID := make(map[string]store.FileSet, len(sets))
	for _, fs := range sets {
		byID[fs.ID] = fs
	}
	items := make([]placementItem, 0, len(views))
	for _, v := range views {
		fs := byID[v.ID]
		it := placementItem{
			Key: v.ID, Identity: "fileset:" + v.Name,
			Home: store.HomeState{Exists: true, Repo: fs.Repo, Choice: fs.RepoChosen},
		}
		if run, _ := h.store.LastSuccessfulBackup(v.ID); run != nil {
			it.LastSuccess = run.StartedAt
		}
		items = append(items, it)
	}
	placements := h.svc.listPlacements(r.Context(), "files", items)
	for i := range views {
		views[i].Placement = placements[views[i].ID]
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "fileSets": views})
}

// handleCreateFileSet creates a file set. POST /api/files/sets
// body {name, path, excludes, enabled, repo, copies}. Without repo the set is
// open and takes the Folders default at its first backup; with it, the empty
// string for the domain repository included, it stays where it is put.
func (h *Handler) handleCreateFileSet(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name     string        `json:"name"`
		Path     string        `json:"path"`
		Excludes []string      `json:"excludes"`
		Enabled  *bool         `json:"enabled"`
		Repo     *string       `json:"repo"`
		Copies   *copiesChoice `json:"copies"`
	}
	if !decodeBody(w, r, &body) {
		return
	}
	enabled := true // a freshly created set participates by default
	if body.Enabled != nil {
		enabled = *body.Enabled
	}
	fs := store.FileSet{
		Name:     strings.TrimSpace(body.Name),
		Path:     strings.TrimSpace(body.Path),
		Excludes: body.Excludes,
		Enabled:  enabled,
	}
	if body.Repo != nil {
		fs.Repo = strings.TrimSpace(*body.Repo)
		fs.RepoChosen = store.RepoChosen
	}
	if fs.Path == "" {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": "path is required"})
		return
	}
	// A new set's path counts as changed, so it has to exist.
	if err := h.svc.validateFileSet(fs, true); err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	// A live set already owns this name: say so plainly rather than running
	// the leftover-snapshot check below, which would answer as if the name
	// were free and its history orphaned.
	if _, err := h.store.GetFileSetByName(fs.Name); err == nil {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": "a file set with this name already exists"})
		return
	}
	// A name whose fileset:<name> snapshots are still in the repo (for
	// example after "Remove set", which keeps them) must not be silently
	// adopted by an unrelated new set.
	if err := h.svc.fileSetNameAdoptable(r.Context(), fs.Name, fs.Repo, fs.Path); err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	created, err := h.svc.createFileSet(fs, body.Copies)
	if err != nil {
		// A duplicate name violates the UNIQUE constraint; report it clearly.
		if strings.Contains(err.Error(), "UNIQUE") {
			writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": "a file set with this name already exists"})
			return
		}
		placementFail(w, err, nil)
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"id": created.ID}))
}

// handlePatchFileSet partially updates a file set. PATCH /api/files/sets/{id}
// body {name?, path?, excludes?, enabled?, selectedPaths?}. Pointers keep an
// enabled-only PATCH from resetting the other fields, and the merged set is
// validated again so a patch cannot slip past the create-time checks.
func (h *Handler) handlePatchFileSet(w http.ResponseWriter, r *http.Request) {
	id, ok := h.fileSetIDParam(w, r)
	if !ok {
		return
	}
	var body struct {
		Name     *string   `json:"name"`
		Path     *string   `json:"path"`
		Excludes *[]string `json:"excludes"`
		Enabled  *bool     `json:"enabled"`
		// Kept apart from the set, like its store setter, so a form that does
		// not know about the cadence cannot clear one by leaving it out.
		ScheduleCadence *string `json:"scheduleCadence"`
		// SelectedPaths is the set's tree selection. nil leaves it untouched, so
		// a plain name or path form cannot clear it; [] is refused, not stored.
		SelectedPaths *[]string `json:"selectedPaths"`
		// Repo is the older spelling of home {repo}.
		Repo *string `json:"repo"`
		// Home is this item's own location: a named repository from Settings,
		// or follow to take the domain's default.
		Home *homeChoice `json:"home"`
		// Copies is the item's own copy rule: which off-site targets it goes to,
		// or follow to take the domain's default.
		Copies *copiesChoice `json:"copies"`
	}
	if !decodeBody(w, r, &body) {
		return
	}
	change, err := withLegacyRepo(placementChange{Home: body.Home, Copies: body.Copies}, body.Repo)
	if err != nil {
		placementFail(w, err, nil)
		return
	}
	item := store.ItemRef{Domain: "files", Key: id}
	if err := h.svc.checkPlacementChange(r.Context(), item, change); err != nil {
		placementFail(w, err, nil)
		return
	}
	fs, err := h.store.GetFileSet(id)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": "file set not found"})
		return
	}
	oldName := fs.Name
	// The stored selection was validated against the old path, which the merge
	// below overwrites.
	oldPath := fs.Path
	wasEnabled := fs.Enabled
	if body.Name != nil {
		fs.Name = strings.TrimSpace(*body.Name)
	}
	if body.Path != nil {
		fs.Path = strings.TrimSpace(*body.Path)
	}
	if body.Excludes != nil {
		fs.Excludes = *body.Excludes
	}
	if body.Enabled != nil {
		fs.Enabled = *body.Enabled
	}
	// A path change moves the anchor every stored entry was validated against,
	// so it clears the selection in the same save (fileSetPositionals also
	// re-anchors at compile time). The clear wins over entries sent in the same
	// request, since keeping both would rewrite their meaning under the new
	// root. Roots are compared resolved, so re-sending the same path is no
	// change; an unresolvable one counts as changed.
	pathChanged := false
	if body.Path != nil {
		oldResolved, oldErr := paths.Resolve(h.cfg.HostMountRoot, oldPath)
		newResolved, newErr := paths.Resolve(h.cfg.HostMountRoot, fs.Path)
		pathChanged = oldErr != nil || newErr != nil || newResolved != oldResolved
	}
	isEnabling := fs.Enabled && !wasEnabled
	if err := h.svc.validateFileSet(fs, pathChanged || isEnabling); err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	// A rename moves where the set's next backup writes, so it needs the files
	// domain lock: BackupFileSet holds it for its whole run, and a rename
	// landing mid-backup would leave that run's snapshot behind the old name
	// while the set already points at the new one. A home change in the same
	// request brings the same lock along in writeItemPlacement, so it is left
	// to take it there.
	nameChanging := fs.Name != oldName
	if nameChanging && change.Home == nil {
		unlock, ok := h.svc.tryLockDomainFor("files", "rename")
		if !ok {
			op, busy := h.svc.domainBusy("files")
			if !busy {
				op = "another operation"
			}
			writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": fmt.Sprintf("%s is running on files; try the change again once it finishes", op)})
			return
		}
		defer unlock()
	}
	// A set with backups cannot be renamed: its snapshots are tagged
	// fileset:<oldName> and never re-tagged, so a rename would strand them where
	// DeleteBackupsFileSet cannot find them.
	if nameChanging {
		presence, bErr := h.svc.itemBackups(r.Context(), store.ItemRef{Domain: "files", Key: id})
		if bErr != nil && presence != backupsUnreadable {
			writeJSON(w, http.StatusOK, failEnvelope(bErr))
			return
		}
		if presence != backupsNone {
			msg := "cannot rename a file set that already has backups; create a new set instead"
			if presence == backupsUnreadable {
				msg = fmt.Sprintf("cannot rename: %s: %v", errFileSetRepoUnreachable, bErr)
			}
			writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": msg})
			return
		}
		// The new name's fileset:<name> snapshots can still be in the
		// repository, left behind by a set that was removed without them.
		if err := h.svc.fileSetNameAdoptable(r.Context(), fs.Name, fs.Repo, fs.Path); err != nil {
			writeJSON(w, http.StatusOK, failEnvelope(err))
			return
		}
	}
	// On a path change the row and the cleared selection are one statement
	// (UpdateFileSetClearingSelection), so a failure cannot leave the new path
	// live with the old selection still stored.
	if fs.Name != oldName {
		if err := h.svc.moveFileSetRule(oldName, fs.Name); err != nil {
			placementFail(w, err, nil)
			return
		}
	}
	var upErr error
	if pathChanged {
		upErr = h.store.UpdateFileSetClearingSelection(fs)
	} else {
		upErr = h.store.UpdateFileSet(fs)
	}
	if upErr != nil {
		if fs.Name != oldName {
			if err := h.svc.moveFileSetRule(fs.Name, oldName); err != nil {
				log.Printf("api: file set %q: its copy rule stays under the new name after the rename failed: %v", oldName, err) //nolint:gosec // G706: the name is %q-quoted
			}
		}
		if strings.Contains(upErr.Error(), "UNIQUE") {
			writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": "a file set with this name already exists"})
			return
		}
		writeJSON(w, http.StatusOK, failEnvelope(upErr))
		return
	}
	switch {
	case pathChanged:
		// UpdateFileSetClearingSelection already cleared the selection.
	case body.SelectedPaths != nil:
		if err := h.svc.SetFileSetSelectedPaths(r.Context(), id, *body.SelectedPaths); err != nil {
			if errors.Is(err, errFileSetEmptySelection) {
				// The code lets the tree tell "empty" from other failures and
				// keep its local state, since nothing was stored.
				writeJSON(w, http.StatusOK, codedFailEnvelope(err, "empty-selection"))
				return
			}
			writeJSON(w, http.StatusOK, failEnvelope(err))
			return
		}
	}
	if body.ScheduleCadence != nil {
		if err := h.svc.SetFileSetScheduleCadence(r.Context(), id, *body.ScheduleCadence); err != nil {
			writeJSON(w, http.StatusOK, failEnvelope(err))
			return
		}
		// As for VMs: the set's own cron entry comes or goes, and a file-set
		// PATCH does not otherwise reload the scheduler.
		if err := h.reloadScheduler(); err != nil {
			writeJSON(w, http.StatusOK, failEnvelope(err))
			return
		}
	}
	// Written last, after every other field landed: WritePlacement reads the
	// set's name fresh inside its own transaction, so it keys the copy rule by
	// the new name on its own when this same request also renamed the set.
	placed, ok := h.applyPlacement(w, r, item, change)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{
		"dropped":   placed.Dropped,
		"placement": h.svc.placementViewOf(r.Context(), store.ItemRef{Domain: "files", Key: id}),
	}))
}

// handleDeleteFileSet removes a file set and its run history without touching
// any repo, so DiscoverFileSets can bring its snapshots back;
// handleDeleteBackupsFileSet deletes the backups. DELETE /api/files/sets/{id}
func (h *Handler) handleDeleteFileSet(w http.ResponseWriter, r *http.Request) {
	id, ok := h.fileSetIDParam(w, r)
	if !ok {
		return
	}
	if err := h.store.DeleteFileSet(id); err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(nil))
}

// handleDeleteBackupsFileSet removes every backup of a file set from the
// selected source. DELETE /api/files/sets/{id}/backups?source=
func (h *Handler) handleDeleteBackupsFileSet(w http.ResponseWriter, r *http.Request) {
	id, ok := h.fileSetIDParam(w, r)
	if !ok {
		return
	}
	if err := h.svc.DeleteBackupsFileSet(r.Context(), id, sourceParam(r)); err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(nil))
}

// handleBackupFileSet starts a single file-set backup on the server and returns
// immediately, like handleBackup. The SPA follows "files:<name>" over SSE.
// POST /api/files/sets/{id}/backup
func (h *Handler) handleBackupFileSet(w http.ResponseWriter, r *http.Request) {
	id, ok := h.fileSetIDParam(w, r)
	if !ok {
		return
	}
	started, err := h.svc.StartBackupFileSet(r.Context(), id)
	if err != nil { // the files domain is busy with another op → 409 with the reason
		writeJSON(w, http.StatusConflict, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	if !started {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": "a backup is already running"})
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"started": true}))
}

// handleBackupFilesAll starts a server-side batch backup of the selected file
// sets, like handleBackupAll; the SPA follows "batch:files" and the per-set
// keys. POST /api/files/backup-all  body {ids: [...]}
func (h *Handler) handleBackupFilesAll(w http.ResponseWriter, r *http.Request) {
	var body struct {
		IDs []string `json:"ids"`
	}
	if !decodeBody(w, r, &body) { // caps the body at 1 MiB
		return
	}
	if len(body.IDs) == 0 {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": "no file sets selected"})
		return
	}
	if len(body.IDs) > 1000 { // far beyond any real set count
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": "too many file sets"})
		return
	}
	// Validate every id at the boundary (same guard as the per-set route).
	for _, id := range body.IDs {
		if !validResourceName(id) {
			writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": "invalid file set id"})
			return
		}
	}
	started, err := h.svc.StartBackupFilesAll(r.Context(), body.IDs)
	if err != nil { // the files domain is busy with another op → 409 with the reason
		writeJSON(w, http.StatusConflict, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	if !started {
		writeJSON(w, http.StatusConflict, map[string]any{"ok": false, "error": "a batch backup is already running"})
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"started": len(body.IDs)}))
}

// handleSnapshotsFileSet lists one file set's snapshots (tag-filtered).
// GET /api/files/sets/{id}/snapshots?source=
func (h *Handler) handleSnapshotsFileSet(w http.ResponseWriter, r *http.Request) {
	id, ok := h.fileSetIDParam(w, r)
	if !ok {
		return
	}
	snaps, err := h.svc.SnapshotsFileSet(r.Context(), id, sourceParam(r))
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	if snaps == nil {
		snaps = []restic.Snapshot{}
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"snapshots": snaps}))
}

// handleRestoreFileSet starts a file-set restore on the server and returns
// immediately, like handleRestore. An empty targetPath restores in place over
// the set's source folder and needs confirm; otherwise the snapshot is
// extracted into that folder under the host mount. The resolved target comes
// back in the answer, and the SPA follows "files:<name>".
// POST /api/files/sets/{id}/restore  body {snapshotId, targetPath, confirm}
func (h *Handler) handleRestoreFileSet(w http.ResponseWriter, r *http.Request) {
	id, ok := h.fileSetIDParam(w, r)
	if !ok {
		return
	}
	var body struct {
		SnapshotID string `json:"snapshotId"`
		TargetPath string `json:"targetPath"`
		Confirm    bool   `json:"confirm"`
	}
	if !decodeBody(w, r, &body) {
		return
	}
	target, started, err := h.svc.StartRestoreFileSet(r.Context(), id, body.SnapshotID, sourceParam(r), body.TargetPath, body.Confirm)
	if err != nil {
		restoreFail(w, sourceParam(r), err)
		return
	}
	if !started {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": "a backup or restore is already running"})
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"started": true, "target": target}))
}

// handleListSnapshotFilesFileSet lists the files in a file-set snapshot for the
// selective restore. GET /api/files/sets/{id}/files?snapshot=<id>&source=
func (h *Handler) handleListSnapshotFilesFileSet(w http.ResponseWriter, r *http.Request) {
	id, ok := h.fileSetIDParam(w, r)
	if !ok {
		return
	}
	snapshot := r.URL.Query().Get("snapshot")
	files, err := h.svc.ListSnapshotFilesFileSet(r.Context(), id, snapshot, sourceParam(r))
	if err != nil {
		restoreFail(w, sourceParam(r), err)
		return
	}
	if files == nil {
		files = []restic.FileEntry{}
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"files": files}))
}

// handleRestoreFileSetFiles starts a selective file-set restore of the given
// paths, otherwise like handleRestoreFileSet: an empty targetPath restores them
// in place and needs confirm.
// POST /api/files/sets/{id}/restore-files  body {snapshotId, paths, targetPath, confirm}
func (h *Handler) handleRestoreFileSetFiles(w http.ResponseWriter, r *http.Request) {
	id, ok := h.fileSetIDParam(w, r)
	if !ok {
		return
	}
	var body struct {
		SnapshotID string   `json:"snapshotId"`
		Paths      []string `json:"paths"`
		TargetPath string   `json:"targetPath"`
		Confirm    bool     `json:"confirm"`
	}
	if !decodeBody(w, r, &body) {
		return
	}
	target, started, err := h.svc.StartRestoreFileSetFiles(r.Context(), id, sourceParam(r), body.SnapshotID, body.Paths, body.TargetPath, body.Confirm)
	if err != nil {
		restoreFail(w, sourceParam(r), err)
		return
	}
	if !started {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": "a backup or restore is already running"})
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"started": true, "target": target}))
}

// handleDiscoverFiles rebuilds the file-set list from the fileset: snapshot
// tags alone, since the files domain stores no definitions in the repository,
// so sets lost with the database become restorable again.
// POST /api/files/discover
func (h *Handler) handleDiscoverFiles(w http.ResponseWriter, r *http.Request) {
	probe := r.URL.Query().Get("probe") == "true" // read-only readiness check, see handleDiscover
	res, err := h.svc.DiscoverFileSets(r.Context(), probe)
	if err != nil {
		// The partial result goes along, as in handleDiscover.
		body := failEnvelope(err)
		maps.Copy(body, discoverFields(res))
		writeJSON(w, http.StatusOK, body)
		return
	}
	fields := discoverFields(res)
	fields["repo"] = h.svc.DiscoverSource("files")
	writeJSON(w, http.StatusOK, okEnvelope(fields))
}
