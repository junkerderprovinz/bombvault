package api

import (
	"cmp"
	"encoding/json"
	"errors"
	"log"
	"maps"
	"net/http"
	"slices"

	"github.com/junkerderprovinz/bombvault/internal/backup"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// containerView is the per-container row returned by GET /api/containers.
// Installed is false for "orphan" rows: containers gone from the host that still
// have backups, so the user can restore or delete them.
type containerView struct {
	Name              string   `json:"name"`
	Image             string   `json:"image"`
	State             string   `json:"state"`
	Status            string   `json:"status"`
	IP                string   `json:"ip"`
	Installed         bool     `json:"installed"`
	IncludeInSchedule bool     `json:"includeInSchedule"`
	LastBackup        *int64   `json:"lastBackup"`
	LastBackupStarted *int64   `json:"lastBackupStarted"`
	PreHook           string   `json:"preHook"`
	PostHook          string   `json:"postHook"`
	StopContainers    []string `json:"stopContainers"`
	Excludes          []string `json:"excludes"`
	UpdateAfterBackup bool     `json:"updateAfterBackup"`
	// BackupOrder is the container's manual backup position: a positive value
	// runs earlier, 0 means unordered (overdue first).
	BackupOrder int `json:"backupOrder"`
	// ScheduleCadence is the container's own schedule; "" follows the containers
	// domain schedule. It takes effect only with the perItemSchedules setting on.
	ScheduleCadence string `json:"scheduleCadence"`
	// LastUpdateCheck and LastUpdateResult record when the post-backup update
	// check last completed (unix seconds, 0 = never) and its outcome ('' |
	// 'up-to-date' | 'updated' | 'failed'), so "checked, up to date" shows
	// without a run row per night.
	LastUpdateCheck  int64  `json:"lastUpdateCheck"`
	LastUpdateResult string `json:"lastUpdateResult"`
	// Stack is the compose project (com.docker.compose.project label) this
	// container belongs to, "" if none. Drives the "restore whole stack" panel.
	Stack string `json:"stack"`
	// Self marks BombVault's own container: the UI hides its backup action and
	// excludes it from "select all" so a batch can never stop the app itself.
	Self      bool          `json:"self"`
	Placement placementView `json:"placement"`
	// RenameFrom and RenameReason name the not-installed entry a live container
	// without backups of its own looks renamed from (see matchRenames); the UI
	// words the reason. Both are empty when nothing matched or the backup times
	// could not be read, since a wrong suggestion is worse than none.
	RenameFrom   string `json:"renameFrom"`
	RenameReason string `json:"renameReason"`
	// AliasConflicts are the former names of this entry that are live
	// containers again, alphabetically. The UI offers no unlink onto any of
	// them and asks for that container to be renamed instead. The entry's
	// history is unaffected, because an alias claims only the old name's
	// snapshots from before its link.
	AliasConflicts []string `json:"aliasConflicts"`
	// Aliases are the names this entry had before, oldest link first.
	Aliases []string `json:"aliases"`
}

func (h *Handler) handleListContainers(w http.ResponseWriter, r *http.Request) {
	infos, err := h.docker.List(r.Context())
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}

	targets, _ := h.store.ListTargets()
	byName := make(map[string]store.Target, len(targets))
	for _, t := range targets {
		byName[t.ContainerName] = t
	}

	self := h.svc.SelfContainerName(r.Context())

	live := make(map[string]bool, len(infos))
	for _, c := range infos {
		live[c.Name] = true
	}

	// Alias conflicts land on the row of the entry the live former name
	// belongs to, not on the live container's own row.
	var formerNames, aliasConflicts aliasIndex
	if aliases, aErr := h.store.ListAliases("container"); aErr != nil {
		log.Printf("api: list containers: alias conflict check: %v", aErr)
	} else {
		formerNames = newAliasIndex(aliases)
		aliasConflicts = liveFormerNames(aliases, live)
	}

	var orphanTargets []store.Target
	for _, t := range targets {
		if !live[t.ContainerName] {
			orphanTargets = append(orphanTargets, t)
		}
	}

	// One listing dates every row and tells the rename pass whether a live
	// container has backups under its own name; snapTimesFailed keeps that pass
	// from guessing off a partial read.
	var snapTimes map[string]int64
	snapTimesFailed := false
	if m, sErr := h.svc.LatestContainerBackupTimes(r.Context()); sErr != nil {
		log.Printf("api: list containers: latest backup times: %v", sErr)
		snapTimesFailed = true
	} else {
		snapTimes = m
	}

	views := make([]containerView, 0, len(infos)+len(targets))
	viewIndex := make(map[string]int, len(infos)) // live rows only, for the rename-suggestion backfill below
	hasOwnBackup := make(map[string]bool, len(infos))
	needsRenameSuggestion := false
	for _, c := range infos {
		v := containerView{
			Name:           c.Name,
			Image:          c.Image,
			State:          c.State,
			Status:         c.Status,
			IP:             c.IP,
			Installed:      true,
			Stack:          c.Stack,
			Self:           self != "" && c.Name == self,
			AliasConflicts: []string{},
			Aliases:        []string{},
		}
		var run *store.Run
		if t, ok := byName[c.Name]; ok {
			v.AliasConflicts = aliasConflicts.of(t.ID)
			v.Aliases = formerNames.of(t.ID)
			v.IncludeInSchedule = t.IncludeInSchedule
			v.PreHook = t.PreHook
			v.PostHook = t.PostHook
			v.StopContainers = t.StopContainers
			v.Excludes = t.Excludes
			v.UpdateAfterBackup = t.UpdateAfterBackup
			v.LastUpdateCheck = t.LastUpdateCheck
			v.LastUpdateResult = t.LastUpdateResult
			v.BackupOrder = t.BackupOrder
			v.ScheduleCadence = t.ScheduleCadence
			run, _ = h.store.LastSuccessfulBackup(t.ID)
		}
		v.LastBackup, v.LastBackupStarted = lastBackupDate(c.Name, run, snapTimes, snapTimesFailed)
		own := v.LastBackup != nil
		hasOwnBackup[c.Name] = own
		if !own {
			needsRenameSuggestion = true
		}
		viewIndex[c.Name] = len(views)
		views = append(views, v)
	}

	// Rename suggestions need an orphan to match and a live container without
	// backups to offer it to. When the backup times could not be read the pass
	// is skipped, since a wrong suggestion is worse than none.
	if len(orphanTargets) > 0 && needsRenameSuggestion && !snapTimesFailed {
		for liveName, cand := range h.svc.suggestRenames(infos, orphanTargets) {
			if hasOwnBackup[liveName] {
				continue // the gate above is a global "worth trying" switch, not a per-row filter
			}
			if idx, ok := viewIndex[liveName]; ok {
				views[idx].RenameFrom = cand.OldName
				views[idx].RenameReason = cand.Reason
			}
		}
	}

	// Orphans: targets with backups whose container is not installed. The
	// image comes from the stored recreate definition (so the row is recognisable
	// even though the container is gone).
	for _, t := range orphanTargets {
		v := containerView{
			Name:              t.ContainerName,
			State:             "not-installed",
			Installed:         false,
			IncludeInSchedule: t.IncludeInSchedule,
			ScheduleCadence:   t.ScheduleCadence,
			AliasConflicts:    aliasConflicts.of(t.ID),
			Aliases:           formerNames.of(t.ID),
		}
		if t.Definition != "" {
			var def containerDefinition
			if json.Unmarshal([]byte(t.Definition), &def) == nil {
				v.Image = def.Inspect.Config.Image
				v.Stack = def.Inspect.Config.Labels["com.docker.compose.project"]
			}
		}
		run, _ := h.store.LastSuccessfulBackup(t.ID)
		v.LastBackup, v.LastBackupStarted = lastBackupDate(t.ContainerName, run, snapTimes, snapTimesFailed)
		views = append(views, v)
	}
	items := make([]placementItem, 0, len(views))
	for _, v := range views {
		it := placementItem{Key: v.Name, Identity: "container:" + v.Name, Stack: v.Stack}
		if t, ok := byName[v.Name]; ok {
			it.Home = store.HomeState{Exists: true, Repo: t.Repo, Choice: t.RepoChosen}
			if run, _ := h.store.LastSuccessfulBackup(t.ID); run != nil {
				it.LastSuccess = run.StartedAt
			}
		}
		items = append(items, it)
	}
	placements := h.svc.listPlacements(r.Context(), "containers", items)
	for i := range views {
		views[i].Placement = placements[views[i].Name]
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "containers": views})
}

// lastBackupDate is the newest backup name owns, so a card's date agrees with
// the list of backups under it. The run stands in only while the repository
// could not be listed, because an unreachable repository must not read as
// "never backed up". The start time comes from the run that wrote that backup
// and from no other, since the dashboard measures a duration from the pair.
func lastBackupDate(name string, run *store.Run, times map[string]int64, unreadable bool) (finished, started *int64) {
	if unreadable {
		if run == nil {
			return nil, nil
		}
		return run.FinishedAt, &run.StartedAt
	}
	ts, ok := times[name]
	if !ok || ts <= 0 {
		return nil, nil
	}
	if run != nil && run.FinishedAt != nil && run.StartedAt <= ts && ts <= *run.FinishedAt {
		return &ts, &run.StartedAt
	}
	return &ts, nil
}

// aliasIndex holds former names by the ID of the entry they belong to.
type aliasIndex map[string][]string

// newAliasIndex indexes every entry's former names, oldest link first.
func newAliasIndex(aliases []store.Alias) aliasIndex {
	byLink := slices.Clone(aliases)
	slices.SortStableFunc(byLink, func(a, b store.Alias) int { return cmp.Compare(a.LinkedAt, b.LinkedAt) })
	idx := make(aliasIndex, len(byLink))
	for _, a := range byLink {
		idx[a.TargetID] = append(idx[a.TargetID], a.OldName)
	}
	return idx
}

// liveFormerNames indexes the former names that are live machines again, each
// a conflict on its entry's row. aliases come from ListAliases, which orders
// them by name, so each entry's list is alphabetical.
func liveFormerNames(aliases []store.Alias, live map[string]bool) aliasIndex {
	idx := aliasIndex{}
	for _, a := range aliases {
		if live[a.OldName] {
			idx[a.TargetID] = append(idx[a.TargetID], a.OldName)
		}
	}
	return idx
}

// of returns an empty list rather than nil, so the row encodes it as [].
func (idx aliasIndex) of(targetID string) []string {
	if names, ok := idx[targetID]; ok {
		return names
	}
	return []string{}
}

// handleDeleteBackups removes every backup of a container from the selected
// source. DELETE /api/containers/{name}/backups?source=
func (h *Handler) handleDeleteBackups(w http.ResponseWriter, r *http.Request) {
	name, ok := h.nameParam(w, r)
	if !ok {
		return
	}
	if err := h.svc.DeleteBackups(r.Context(), name, sourceParam(r)); err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(nil))
}

// handleForgetContainer clears a container's stale "Not installed" entry (its
// target row) without touching any repo, the twin of handleForgetVM.
// DELETE /api/containers/{name}
func (h *Handler) handleForgetContainer(w http.ResponseWriter, r *http.Request) {
	name, ok := h.nameParam(w, r)
	if !ok {
		return
	}
	if err := h.svc.ForgetTarget(r.Context(), name); err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(nil))
}

// takeoverFail answers a refused takeover or unlink. A copy rule already on the
// name gets the code the placement routes send for it, so the interface can say
// it in the user's language.
func takeoverFail(w http.ResponseWriter, err error) {
	if errors.Is(err, store.ErrCopyRuleTaken) {
		placementFail(w, err, nil)
		return
	}
	writeJSON(w, http.StatusOK, failEnvelope(err))
}

// handleTakeOverContainer moves a not-installed entry onto the name its
// container was renamed to, keeping its history and settings with the old name
// as an alias. POST /api/containers/{name}/takeover with body
// {"from":"<old name>"}; "from" is checked like {name}, because
// RenameTargetWithAlias validates nothing itself.
func (h *Handler) handleTakeOverContainer(w http.ResponseWriter, r *http.Request) {
	name, ok := h.nameParam(w, r)
	if !ok {
		return
	}
	var body struct {
		From string `json:"from"`
	}
	if !decodeBody(w, r, &body) {
		return
	}
	if !validResourceName(body.From) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "invalid name"})
		return
	}
	if err := h.svc.TakeOverContainer(r.Context(), body.From, name); err != nil {
		takeoverFail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(nil))
}

// handleUnlinkContainerAlias undoes a takeover, moving the entry back to its
// old name. DELETE /api/containers/{name}/alias/{old}; {old} alone finds the
// entry, since a former name belongs to one entry.
func (h *Handler) handleUnlinkContainerAlias(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.nameParam(w, r); !ok {
		return
	}
	old := r.PathValue("old")
	if !validResourceName(old) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "invalid name"})
		return
	}
	if err := h.svc.UnlinkContainerAlias(r.Context(), old); err != nil {
		takeoverFail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(nil))
}

// discoverFields is what every Discover answer carries, success or not.
// skippedNeedsAction flags only skips that need action: a repository somebody
// switched off is named, but it is not a fault.
func discoverFields(res DiscoverResult) map[string]any {
	leftOpen := res.LeftOpen
	if leftOpen == nil {
		leftOpen = []string{}
	}
	return map[string]any{
		"discovered":         res.Found,
		"skipped":            skipNames(res.Skipped),
		"skippedNeedsAction": len(actionableSkips(res.Skipped)) > 0,
		"paused":             res.Paused,
		"leftOpen":           leftOpen,
		"directRepos":        directRepoViews(res.Direct),
	}
}

// handleDiscover rebuilds the target list from the backup storage (disaster
// recovery after a fresh install / loss of /config).
func (h *Handler) handleDiscover(w http.ResponseWriter, r *http.Request) {
	// probe=true is the Recovery tab's read-only check: it opens and decrypts
	// to prove the repo and APP_KEY but writes no targets, so a readiness check
	// never brings orphan entries back.
	probe := r.URL.Query().Get("probe") == "true"
	res, err := h.svc.Discover(r.Context(), probe)
	if err != nil {
		// The partial result goes along: named repositories are searched before
		// the domain's own, so when that one fails, everything found so far is
		// real, and this is the screen opened after losing the configuration.
		body := failEnvelope(err)
		maps.Copy(body, discoverFields(res))
		writeJSON(w, http.StatusOK, body)
		return
	}
	// `repo` names the folder this pass read, since the wizard asks for an
	// off-site repository a step earlier and then reads the primary path.
	// `skipped` names the repositories it could not read, so "0 found" after a
	// /config loss tells empty repositories apart from unreachable ones.
	fields := discoverFields(res)
	fields["repo"] = h.svc.DiscoverSource("containers")
	writeJSON(w, http.StatusOK, okEnvelope(fields))
}

// handleBackup starts a single container backup on the server and returns
// immediately, so a long backup, or a backup of the reverse proxy the UI runs
// through, cannot make the SPA report a failure for a backup that completes.
// The SPA follows the "container:<name>" progress key over SSE and reads the
// recorded run for the outcome.
func (h *Handler) handleBackup(w http.ResponseWriter, r *http.Request) {
	name, ok := h.nameParam(w, r)
	if !ok {
		return
	}
	started, err := h.svc.StartBackup(r.Context(), name)
	if err != nil { // the target domain is busy with another op → 409 with the reason
		writeJSON(w, http.StatusConflict, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	if !started {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": "a backup is already running"})
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"started": true}))
}

// handleBackupAll starts a server-side batch backup of the selected containers.
// It runs apart from this request, so closing the browser or stopping the
// container the UI runs in cannot interrupt it; the SPA follows progress over
// SSE ("batch:containers" and the per-container keys).
func (h *Handler) handleBackupAll(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Names []string `json:"names"`
	}
	if !decodeBody(w, r, &body) { // caps the body at 1 MiB
		return
	}
	if len(body.Names) == 0 {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": "no containers selected"})
		return
	}
	if len(body.Names) > 1000 { // far beyond any real container count
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": "too many containers"})
		return
	}
	// Validate every name at the boundary (same guard as the per-container route)
	// so no traversal/option-injection name reaches the service layer.
	for _, n := range body.Names {
		if !validResourceName(n) {
			writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": "invalid name"})
			return
		}
	}
	started, err := h.svc.StartBackupAll(r.Context(), body.Names)
	if err != nil { // the containers domain is busy with another op → 409 with the reason
		writeJSON(w, http.StatusConflict, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	if !started {
		writeJSON(w, http.StatusConflict, map[string]any{"ok": false, "error": "a batch backup is already running"})
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"started": len(body.Names)}))
}

func (h *Handler) handleSnapshots(w http.ResponseWriter, r *http.Request) {
	name, ok := h.nameParam(w, r)
	if !ok {
		return
	}
	snaps, err := h.svc.Snapshots(r.Context(), name, sourceParam(r))
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	if snaps == nil {
		snaps = []restic.Snapshot{}
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"snapshots": snaps}))
}

// handleRestore starts an in-place container restore on the server and returns
// immediately, because a restore that held the request open for hours died
// with it when the browser or proxy dropped the connection. Validation still
// runs first, so a bad request fails right away; the SPA follows the
// "container:<name>" progress key over SSE and reads the recorded run.
func (h *Handler) handleRestore(w http.ResponseWriter, r *http.Request) {
	name, ok := h.nameParam(w, r)
	if !ok {
		return
	}
	var body struct {
		SnapshotID   string `json:"snapshotId"`
		Confirm      bool   `json:"confirm"`
		LeaveStopped bool   `json:"leaveStopped"`
	}
	if !decodeBody(w, r, &body) {
		return
	}
	// Checked here so an unconfirmed request fails synchronously with the usual
	// sentinel; the service checks again for the stack restore path.
	if !body.Confirm {
		writeJSON(w, http.StatusOK, failEnvelope(backup.ErrNotConfirmed))
		return
	}
	started, err := h.svc.StartRestore(r.Context(), name, body.SnapshotID, sourceParam(r), body.LeaveStopped)
	if err != nil {
		restoreFail(w, sourceParam(r), err)
		return
	}
	if !started {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": "a backup or restore is already running"})
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"started": true}))
}

// handleRestoreCancel cancels an in-flight restore by its progress key
// (POST /api/restore/cancel {key}). Cancelling an unknown/already-finished key is
// an idempotent success (cancelled:false). A cancelled restore records a
// "cancelled" run (distinct from "failed") and fires no failure alert.
func (h *Handler) handleRestoreCancel(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Key string `json:"key"`
	}
	if !decodeBody(w, r, &body) {
		return
	}
	cancelled := h.svc.CancelRun(body.Key)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "cancelled": cancelled})
}

// handleBackupCancel cancels an in-flight backup by its progress key
// (POST /api/backup/cancel {key}). It is a separate route from
// handleRestoreCancel because a cancelled restore leaves a container gone and
// its appdata partial, and a wrong key prefix must not reach that. A key that
// is not running is an idempotent success (cancelled:false), so a tab still
// showing the button for a finished backup gets no error.
func (h *Handler) handleBackupCancel(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Key string `json:"key"`
	}
	if !decodeBody(w, r, &body) {
		return
	}
	cancelled := h.svc.CancelBackupRun(body.Key)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "cancelled": cancelled})
}

// handleListFiles lists the files in a container snapshot for file-level restore.
// GET /api/containers/{name}/files?snapshot=<id>
func (h *Handler) handleListFiles(w http.ResponseWriter, r *http.Request) {
	name, ok := h.nameParam(w, r)
	if !ok {
		return
	}
	snapshot := r.URL.Query().Get("snapshot")
	files, err := h.svc.ListSnapshotFiles(r.Context(), name, snapshot, sourceParam(r))
	if err != nil {
		restoreFail(w, sourceParam(r), err)
		return
	}
	if files == nil {
		files = []restic.FileEntry{}
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"files": files}))
}

// handleRestoreFiles restores one or more files/dirs from a container snapshot,
// either back to their original locations (targetPath empty) or into an alternate
// folder under the host mount. POST /api/containers/{name}/restore-files
//
// Like handleRestore it runs detached; validation and target resolution run
// first, and the resolved target comes back in the answer.
func (h *Handler) handleRestoreFiles(w http.ResponseWriter, r *http.Request) {
	name, ok := h.nameParam(w, r)
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
	target, started, err := h.svc.StartRestoreFiles(r.Context(), name, sourceParam(r), body.SnapshotID, body.Paths, body.TargetPath, body.Confirm)
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

// handleRestoreContainerTo extracts a whole container snapshot into another
// folder under the host mount, leaving the live container untouched.
// POST /api/containers/{name}/restore-to
//
// Like handleRestore it runs detached; validation and target resolution run
// first, and the resolved target comes back in the answer.
func (h *Handler) handleRestoreContainerTo(w http.ResponseWriter, r *http.Request) {
	name, ok := h.nameParam(w, r)
	if !ok {
		return
	}
	var body struct {
		SnapshotID string `json:"snapshotId"`
		TargetPath string `json:"targetPath"`
	}
	if !decodeBody(w, r, &body) {
		return
	}
	target, started, err := h.svc.StartRestoreToPath(r.Context(), name, sourceParam(r), body.SnapshotID, body.TargetPath)
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

// handleDiff compares two of a container's snapshots and returns the summary of
// what changed between them. GET /api/containers/{name}/diff?from=&to=&source=
func (h *Handler) handleDiff(w http.ResponseWriter, r *http.Request) {
	name, ok := h.nameParam(w, r)
	if !ok {
		return
	}
	from := r.URL.Query().Get("from")
	to := r.URL.Query().Get("to")
	d, err := h.svc.DiffSnapshots(r.Context(), name, sourceParam(r), from, to)
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{
		"diff": map[string]any{
			"addedFiles":   d.AddedFiles,
			"removedFiles": d.RemovedFiles,
			"changedFiles": d.ChangedFiles,
			"addedBytes":   d.AddedBytes,
			"removedBytes": d.RemovedBytes,
		},
	}))
}

// handleTagSnapshot adds tags to one of a container's snapshots (restic tag).
// POST /api/containers/{name}/tag  body {snapshotId, tags:[...]}
func (h *Handler) handleTagSnapshot(w http.ResponseWriter, r *http.Request) {
	name, ok := h.nameParam(w, r)
	if !ok {
		return
	}
	var body struct {
		SnapshotID string   `json:"snapshotId"`
		Tags       []string `json:"tags"`
	}
	if !decodeBody(w, r, &body) {
		return
	}
	if err := h.svc.TagSnapshot(r.Context(), name, sourceParam(r), body.SnapshotID, body.Tags); err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(nil))
}

func (h *Handler) handlePatchContainer(w http.ResponseWriter, r *http.Request) {
	name, ok := h.nameParam(w, r)
	if !ok {
		return
	}
	// Pointers, so a hooks-only PATCH does not reset the schedule flag and only
	// the fields actually sent are applied.
	var body struct {
		IncludeInSchedule *bool     `json:"includeInSchedule"`
		PreHook           *string   `json:"preHook"`
		PostHook          *string   `json:"postHook"`
		BackupPaths       *[]string `json:"backupPaths"`
		// SelectionSource says where a backupPaths save came from. Only "tree"
		// means anything: it refuses deselecting everything over a non-empty
		// selection. Any other value counts as absent, so it can never fail a
		// save. It has to be declared because decodeBody disallows unknown fields.
		SelectionSource *string   `json:"selectionSource"`
		StopContainers  *[]string `json:"stopContainers"`
		Excludes        *[]string `json:"excludes"`
		// ExcludeCaches maps a mount root's host path to its CACHEDIR.TAG toggle.
		// nil means untouched and an empty object clears every toggle, which a
		// plain map already tells apart. Service.SetExcludeCaches validates the
		// keys, and only the union of the values reaches restic argv.
		ExcludeCaches     map[string]bool `json:"excludeCaches"`
		UpdateAfterBackup *bool           `json:"updateAfterBackup"`
		ScheduleCadence   *string         `json:"scheduleCadence"`
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
	item := store.ItemRef{Domain: "containers", Key: name}
	if err := h.svc.checkPlacementChange(r.Context(), item, change); err != nil {
		placementFail(w, err, nil)
		return
	}
	if body.IncludeInSchedule != nil {
		if err := h.svc.SetInclude(r.Context(), name, *body.IncludeInSchedule); err != nil {
			writeJSON(w, http.StatusOK, failEnvelope(err))
			return
		}
	}
	if body.PreHook != nil || body.PostHook != nil {
		pre, post := strOr(body.PreHook), strOr(body.PostHook)
		if err := h.svc.SetContainerHooks(r.Context(), name, pre, post); err != nil {
			writeJSON(w, http.StatusOK, failEnvelope(err))
			return
		}
	}
	if body.BackupPaths != nil {
		if err := h.svc.SetBackupPaths(r.Context(), name, *body.BackupPaths, strOr(body.SelectionSource)); err != nil {
			// The empty-selection refusal gets a code so the UI can offer
			// guidance ("nothing would be backed up") instead of a bare failure.
			if errors.Is(err, errEmptySelection) {
				writeJSON(w, http.StatusOK, codedFailEnvelope(err, "empty-selection"))
				return
			}
			writeJSON(w, http.StatusOK, failEnvelope(err))
			return
		}
	}
	if body.StopContainers != nil {
		if err := h.svc.SetStopContainers(r.Context(), name, *body.StopContainers); err != nil {
			writeJSON(w, http.StatusOK, failEnvelope(err))
			return
		}
	}
	if body.Excludes != nil {
		if err := h.svc.SetExcludes(r.Context(), name, *body.Excludes); err != nil {
			writeJSON(w, http.StatusOK, failEnvelope(err))
			return
		}
	}
	if body.ExcludeCaches != nil {
		if err := h.svc.SetExcludeCaches(r.Context(), name, body.ExcludeCaches); err != nil {
			writeJSON(w, http.StatusOK, failEnvelope(err))
			return
		}
	}
	if body.UpdateAfterBackup != nil {
		if err := h.svc.SetUpdateAfterBackup(r.Context(), name, *body.UpdateAfterBackup); err != nil {
			writeJSON(w, http.StatusOK, failEnvelope(err))
			return
		}
	}
	if body.ScheduleCadence != nil {
		if err := h.svc.SetScheduleCadence(r.Context(), name, *body.ScheduleCadence); err != nil {
			writeJSON(w, http.StatusOK, failEnvelope(err))
			return
		}
		// A per-item cadence adds or removes its own cron entry, which takes
		// effect only on a scheduler reload, and a container PATCH does not
		// otherwise reload.
		if err := h.reloadScheduler(); err != nil {
			writeJSON(w, http.StatusOK, failEnvelope(err))
			return
		}
	}
	placed, ok := h.applyPlacement(w, r, item, change)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{
		"dropped":   placed.Dropped,
		"placement": h.svc.placementViewOf(r.Context(), store.ItemRef{Domain: "containers", Key: name}),
	}))
}

// handleScheduleIncludeAll sets the include_in_schedule flag for every installed
// container in one call, for the "include all" and "exclude all" actions.
// Excluding also reaches containers that are not installed.
// POST /api/containers/schedule-include  body {include: bool}
func (h *Handler) handleScheduleIncludeAll(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Include bool `json:"include"`
	}
	if !decodeBody(w, r, &body) {
		return
	}
	if err := h.svc.SetIncludeAll(r.Context(), body.Include); err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(nil))
}

// handleGetBackupOrder returns the manual backup ordering: the containers with
// an explicit order, ascending.
// GET /api/containers/backup-order  →  {order: [{container, order}, ...]}
func (h *Handler) handleGetBackupOrder(w http.ResponseWriter, r *http.Request) {
	orders, err := h.svc.BackupOrders(r.Context())
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	if orders == nil {
		orders = []store.ContainerOrder{}
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"order": orders}))
}

// handleSetBackupOrder replaces the manual backup ordering from a list of
// container names, the first running earliest. A container left out goes back
// to the most-overdue-first tiebreak, so an empty list clears every order.
// PUT /api/containers/backup-order  body {order: ["nameA", "nameB", ...]}
func (h *Handler) handleSetBackupOrder(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Order []string `json:"order"`
	}
	if !decodeBody(w, r, &body) { // caps the body at 1 MiB
		return
	}
	if len(body.Order) > 1000 { // far beyond any real container count
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": "too many containers"})
		return
	}
	// Validate every name at the boundary (same guard as the batch-backup route)
	// so no traversal/option-injection name reaches the service layer.
	for _, n := range body.Order {
		if !validResourceName(n) {
			writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": "invalid name"})
			return
		}
	}
	if err := h.svc.SetBackupOrders(r.Context(), body.Order); err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(nil))
}

// handleContainerMounts lists a container's bind mounts (annotated with the
// current selection) for the backup-folder selector. Stored exclusions come
// back in their own top-level excluded array (host form), not mixed into custom
// as stale paths.
// GET /api/containers/{name}/mounts
func (h *Handler) handleContainerMounts(w http.ResponseWriter, r *http.Request) {
	name, ok := h.nameParam(w, r)
	if !ok {
		return
	}
	mounts, custom, excluded, excludeCaches, err := h.svc.ContainerMounts(r.Context(), name)
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	if mounts == nil {
		mounts = []MountInfo{}
	}
	if custom == nil {
		custom = []CustomPath{}
	}
	if excluded == nil {
		excluded = []string{}
	}
	if excludeCaches == nil {
		// The SPA expects an object under excludeCaches, never null.
		excludeCaches = map[string]bool{}
	}
	// hostMountRoot/hostSourceRoot let the folder picker translate a browsed path
	// (relative to the host mount) back to the host path SetBackupPaths expects.
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{
		"mounts":         mounts,
		"custom":         custom,
		"excluded":       excluded,
		"excludeCaches":  excludeCaches,
		"hostMountRoot":  h.cfg.HostMountRoot,
		"hostSourceRoot": h.cfg.HostSourceRoot,
	}))
}

// handleExcludesPreview resolves a candidate list of exclude patterns against a
// container's live mounts and reports, per line, the restic --exclude pattern
// that will actually be used plus whether it would match anything in this
// container's backup (so the UI can warn on a line that excludes nothing).
// POST /api/containers/{name}/excludes/preview  body {patterns:[...]}
func (h *Handler) handleExcludesPreview(w http.ResponseWriter, r *http.Request) {
	name, ok := h.nameParam(w, r)
	if !ok {
		return
	}
	var body struct {
		Patterns []string `json:"patterns"`
	}
	if !decodeBody(w, r, &body) {
		return
	}
	preview, err := h.svc.PreviewExcludes(r.Context(), name, body.Patterns)
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	if preview == nil {
		preview = []ExcludePreview{}
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"preview": preview}))
}
