package api

import (
	"errors"
	"log"
	"net/http"
	"strings"

	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
	"github.com/junkerderprovinz/bombvault/internal/zfs"
)

// zfsMaxBackupAllIDs bounds one batch press, like the container and folder
// batches.
const zfsMaxBackupAllIDs = 1000

// zfsIDParam extracts and validates the {id} path value. Item ids are
// store-generated 32-hex strings, so the strict resource charset fits, and
// validating here keeps a traversal id out of the service layer.
func (h *Handler) zfsIDParam(w http.ResponseWriter, r *http.Request) (string, bool) {
	id := r.PathValue("id")
	if !validResourceName(id) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "invalid ZFS item id"})
		return "", false
	}
	return id, true
}

// zfsFail answers a refusal that carries a reason code with that code, so the
// page builds the sentence instead of showing the backend's text.
func zfsFail(w http.ResponseWriter, err error) {
	if code, ok := zfsRefusalCode(err); ok {
		writeJSON(w, http.StatusOK, codedFailEnvelope(err, code))
		return
	}
	writeJSON(w, http.StatusOK, failEnvelope(err))
}

// handleListZFSDatasets lists the items with what the last run and the last
// look at the tree found. It never reaches the host. GET /api/zfs
func (h *Handler) handleListZFSDatasets(w http.ResponseWriter, r *http.Request) {
	views, err := h.svc.ListZFSDatasetViews(r.Context())
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	if views == nil {
		views = []ZFSDatasetView{}
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"datasets": views}))
}

// handleZFSConnection reports what stands between the domain and a working
// backup. GET /api/zfs/connection
func (h *Handler) handleZFSConnection(w http.ResponseWriter, r *http.Request) {
	res := h.svc.ZFSConnectionTest(r.Context())
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{
		"code": res.Code, "target": res.Target, "uriTarget": res.URITarget,
		"version": res.Version, "detail": res.Detail, "zfsBinary": res.ZFSBinary,
		"propagation": res.Propagation, "unpropagated": res.Unpropagated,
	}))
}

// handleZFSHostDatasets lists the pools as the host has them, with what each
// dataset may become. The name limit travels along because a name-too-long
// blocker's sentence states it. With cached=true the last listing that worked
// answers, which is enough for the page's counts.
// GET /api/zfs/host?cached=true|false
func (h *Handler) handleZFSHostDatasets(w http.ResponseWriter, r *http.Request) {
	res := h.svc.DiscoverZFSHost(r.Context(), r.URL.Query().Get("cached") != "true")
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{
		"available": res.Available, "code": res.Code, "target": res.Target,
		"datasets": res.Datasets, "hiddenLegacy": res.HiddenLegacy,
		"unusedZvols": res.UnusedZvols, "notInItem": res.NotInItem, "truncated": res.Truncated,
		"listedAt": res.ListedAt, "maxNameLength": zfs.MaxDatasetNameLen,
	}))
}

// handleZFSCheck looks at a tree the add dialog is offering and reports what
// each of its datasets would be. POST /api/zfs/check
func (h *Handler) handleZFSCheck(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Dataset          string   `json:"dataset"`
		ExcludedChildren []string `json:"excludedChildren"`
	}
	if !decodeBody(w, r, &body) {
		return
	}
	if err := zfs.ValidateDatasetName(body.Dataset); err != nil {
		writeJSON(w, http.StatusOK, codedFailEnvelope(err, "invalid-name"))
		return
	}
	check := h.svc.CheckZFSDataset(r.Context(), body.Dataset, body.ExcludedChildren)
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"check": check}))
}

// handleCreateZFSDatasets adds the items the dialog ticked through. The batch
// is answered item by item, so one refused root does not lose the rest.
// POST /api/zfs/datasets
func (h *Handler) handleCreateZFSDatasets(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Items []ZFSCreateItem `json:"items"`
	}
	if !decodeBody(w, r, &body) {
		return
	}
	if len(body.Items) == 0 {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": "no datasets selected"})
		return
	}
	if len(body.Items) > zfsMaxCreateItems {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": "too many datasets in one request"})
		return
	}
	results := h.svc.CreateZFSDatasets(r.Context(), body.Items)
	if results == nil {
		results = []ZFSCreateResult{}
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"results": results}))
}

// handlePatchZFSDataset changes one item's settings. Pointers everywhere, so a
// form that does not know a field cannot clear it by leaving it out.
// PATCH /api/zfs/datasets/{id}
func (h *Handler) handlePatchZFSDataset(w http.ResponseWriter, r *http.Request) {
	id, ok := h.zfsIDParam(w, r)
	if !ok {
		return
	}
	var body struct {
		Enabled          *bool     `json:"enabled"`
		Excludes         *[]string `json:"excludes"`
		ExcludedChildren *[]string `json:"excludedChildren"`
		ScheduleCadence  *string   `json:"scheduleCadence"`
		Repo             *string   `json:"repo"`
		StopContainers   *[]string `json:"stopContainers"`
		HookContainer    *string   `json:"hookContainer"`
		PreSnapshot      *string   `json:"preSnapshot"`
		PostSnapshot     *string   `json:"postSnapshot"`
	}
	if !decodeBody(w, r, &body) {
		return
	}
	err := h.svc.PatchZFSDataset(r.Context(), id, ZFSDatasetPatch{
		Enabled:          body.Enabled,
		Excludes:         body.Excludes,
		ExcludedChildren: body.ExcludedChildren,
		ScheduleCadence:  body.ScheduleCadence,
		Repo:             body.Repo,
		StopContainers:   body.StopContainers,
		HookContainer:    body.HookContainer,
		PreSnapshot:      body.PreSnapshot,
		PostSnapshot:     body.PostSnapshot,
	})
	if err != nil {
		zfsFail(w, err)
		return
	}
	// A cadence change is structural: without a reload the item keeps the cron
	// entry it had, or never gets one.
	if body.ScheduleCadence != nil {
		if rErr := h.reloadScheduler(); rErr != nil {
			writeJSON(w, http.StatusOK, failEnvelope(rErr))
			return
		}
	}
	writeJSON(w, http.StatusOK, okEnvelope(nil))
}

// handleDeleteZFSDataset removes an item and says what is still on the pool.
// DELETE /api/zfs/datasets/{id}?safety=true|false
func (h *Handler) handleDeleteZFSDataset(w http.ResponseWriter, r *http.Request) {
	id, ok := h.zfsIDParam(w, r)
	if !ok {
		return
	}
	res, err := h.svc.DeleteZFSDataset(r.Context(), id, r.URL.Query().Get("safety") == "true")
	if err != nil {
		if errors.Is(err, errDomainBusy) {
			writeJSON(w, http.StatusConflict, failEnvelope(err))
			return
		}
		zfsFail(w, err)
		return
	}
	if rErr := h.reloadScheduler(); rErr != nil {
		log.Printf("api: zfs: reloading the scheduler after a delete failed: %v", rErr)
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{
		"leftoversRemaining": res.LeftoversRemaining, "safetyRemaining": res.SafetyRemaining,
	}))
}

// handleDeleteBackupsZFSDataset forgets every member's snapshots and then the
// item, and says what is still on the pool.
// DELETE /api/zfs/datasets/{id}/backups?safety=true|false
func (h *Handler) handleDeleteBackupsZFSDataset(w http.ResponseWriter, r *http.Request) {
	id, ok := h.zfsIDParam(w, r)
	if !ok {
		return
	}
	res, err := h.svc.DeleteBackupsZFSDataset(r.Context(), id, r.URL.Query().Get("safety") == "true")
	if err != nil {
		if errors.Is(err, errDomainBusy) {
			writeJSON(w, http.StatusConflict, failEnvelope(err))
			return
		}
		zfsFail(w, err)
		return
	}
	if rErr := h.reloadScheduler(); rErr != nil {
		log.Printf("api: zfs: reloading the scheduler after a delete failed: %v", rErr)
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{
		"leftoversRemaining": res.LeftoversRemaining, "safetyRemaining": res.SafetyRemaining,
	}))
}

// handleListZFSSafetySnapshots lists what in-place restores kept, reconciled
// against the pool. GET /api/zfs/datasets/{id}/safety-snapshots
func (h *Handler) handleListZFSSafetySnapshots(w http.ResponseWriter, r *http.Request) {
	id, ok := h.zfsIDParam(w, r)
	if !ok {
		return
	}
	snaps, err := h.svc.ListZFSSafetySnapshots(r.Context(), id)
	if err != nil {
		zfsFail(w, err)
		return
	}
	if snaps == nil {
		snaps = []store.ZFSSafetySnapshot{}
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"snapshots": snaps}))
}

// handleDeleteZFSSafetySnapshot destroys one snapshot a restore kept.
// DELETE /api/zfs/datasets/{id}/safety-snapshots?dataset=&name=
func (h *Handler) handleDeleteZFSSafetySnapshot(w http.ResponseWriter, r *http.Request) {
	id, ok := h.zfsIDParam(w, r)
	if !ok {
		return
	}
	q := r.URL.Query()
	if err := h.svc.DeleteZFSSafetySnapshot(r.Context(), id, q.Get("dataset"), q.Get("name")); err != nil {
		if errors.Is(err, errDomainBusy) {
			writeJSON(w, http.StatusConflict, failEnvelope(err))
			return
		}
		zfsFail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(nil))
}

// handleZFSRunMembers serves one run's detail in a single request: what it did
// to each dataset, how long the applications were held and what a failing
// post-snapshot command said. GET /api/zfs/runs/{runId}/members
func (h *Handler) handleZFSRunMembers(w http.ResponseWriter, r *http.Request) {
	runID := r.PathValue("runId")
	if !validResourceName(runID) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "invalid run id"})
		return
	}
	detail, err := h.svc.ZFSRunDetail(r.Context(), runID)
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{
		"members": detail.Members, "windowSeconds": detail.WindowSeconds, "hookDetail": detail.HookDetail,
	}))
}

// handlePreviewZFSExcludes reports what each exclude line would leave out.
// POST /api/zfs/excludes/preview
func (h *Handler) handlePreviewZFSExcludes(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ID       string   `json:"id"`
		Excludes []string `json:"excludes"`
	}
	if !decodeBody(w, r, &body) {
		return
	}
	if !validResourceName(body.ID) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "invalid ZFS item id"})
		return
	}
	rows, err := h.svc.PreviewZFSExcludes(r.Context(), body.ID, body.Excludes)
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	if rows == nil {
		rows = []ZFSExcludePreviewRow{}
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"rows": rows}))
}

// handleBackupZFSDataset starts one item's backup in the background.
// POST /api/zfs/datasets/{id}/backup
func (h *Handler) handleBackupZFSDataset(w http.ResponseWriter, r *http.Request) {
	id, ok := h.zfsIDParam(w, r)
	if !ok {
		return
	}
	started, err := h.svc.StartBackupZFSDataset(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusConflict, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	if !started {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": "a backup is already running"})
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"started": true}))
}

// handleBackupZFSAll starts a detached batch over the chosen items, watched
// through the "batch:zfs" progress key. POST /api/zfs/backup-all
func (h *Handler) handleBackupZFSAll(w http.ResponseWriter, r *http.Request) {
	var body struct {
		IDs []string `json:"ids"`
	}
	if !decodeBody(w, r, &body) {
		return
	}
	if len(body.IDs) == 0 {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": "no ZFS items selected"})
		return
	}
	if len(body.IDs) > zfsMaxBackupAllIDs {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": "too many ZFS items in one request"})
		return
	}
	for _, id := range body.IDs {
		if !validResourceName(id) {
			writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "invalid ZFS item id"})
			return
		}
	}
	started, err := h.svc.StartBackupZFSAll(r.Context(), body.IDs)
	if err != nil {
		writeJSON(w, http.StatusConflict, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	if !started {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": "a backup is already running"})
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"started": len(body.IDs)}))
}

// handleProbeZFSDataset takes a real snapshot of the tree, checks the container
// can read it and removes it again. POST /api/zfs/datasets/{id}/probe
func (h *Handler) handleProbeZFSDataset(w http.ResponseWriter, r *http.Request) {
	id, ok := h.zfsIDParam(w, r)
	if !ok {
		return
	}
	if op, busy := h.svc.domainBusy(zfsDomain); busy {
		writeJSON(w, http.StatusConflict, map[string]any{"ok": false, "error": op + " is running on zfs"})
		return
	}
	check := h.svc.ProbeZFSSnapshotAccess(r.Context(), id)
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"check": check}))
}

// handleSweepZFSDataset removes the snapshot stamps earlier runs left on the
// tree. POST /api/zfs/datasets/{id}/sweep
func (h *Handler) handleSweepZFSDataset(w http.ResponseWriter, r *http.Request) {
	id, ok := h.zfsIDParam(w, r)
	if !ok {
		return
	}
	if op, busy := h.svc.domainBusy(zfsDomain); busy {
		writeJSON(w, http.StatusConflict, map[string]any{"ok": false, "error": op + " is running on zfs"})
		return
	}
	remaining, err := h.svc.SweepZFSLeftovers(r.Context(), id)
	if err != nil {
		zfsFail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"remaining": remaining}))
}

// handleZFSRestorePoints lists the run instants an item can be restored from,
// newest first. GET /api/zfs/datasets/{id}/restore-points?source=
func (h *Handler) handleZFSRestorePoints(w http.ResponseWriter, r *http.Request) {
	id, ok := h.zfsIDParam(w, r)
	if !ok {
		return
	}
	points, err := h.svc.ListZFSRestorePoints(r.Context(), id, sourceParam(r))
	if err != nil {
		zfsFail(w, err)
		return
	}
	if points == nil {
		points = []ZFSRestorePoint{}
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"points": points}))
}

// handleListSnapshotFilesZFS lists one member snapshot's files for the
// selective restore. GET /api/zfs/datasets/{id}/files?snapshot=&source=
func (h *Handler) handleListSnapshotFilesZFS(w http.ResponseWriter, r *http.Request) {
	id, ok := h.zfsIDParam(w, r)
	if !ok {
		return
	}
	files, err := h.svc.ListSnapshotFilesZFS(r.Context(), id, r.URL.Query().Get("snapshot"), sourceParam(r))
	if err != nil {
		zfsFail(w, err)
		return
	}
	if files == nil {
		files = []restic.FileEntry{}
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"files": files}))
}

// handleRestoreZFS starts a restore and answers with where it writes and which
// safety snapshot it took. POST /api/zfs/datasets/{id}/restore?source=
func (h *Handler) handleRestoreZFS(w http.ResponseWriter, r *http.Request) {
	id, ok := h.zfsIDParam(w, r)
	if !ok {
		return
	}
	var req ZFSRestoreRequest
	if !decodeBody(w, r, &req) {
		return
	}
	ack, started, err := h.svc.StartRestoreZFS(r.Context(), id, sourceParam(r), req)
	if err != nil {
		zfsFail(w, err)
		return
	}
	if !started {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": "a backup or restore is already running"})
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{
		"started": true, "target": ack.Target, "safetySnapshot": ack.SafetySnapshot,
	}))
}

// handleDiscoverZFS rebuilds the item list from the zfs: tags in the
// repositories, for a server that lost its database.
// POST /api/zfs/discover?probe=true|false
func (h *Handler) handleDiscoverZFS(w http.ResponseWriter, r *http.Request) {
	probe := r.URL.Query().Get("probe") == "true"
	n, skipped, err := h.svc.DiscoverZFSDatasets(r.Context(), probe)
	if err != nil {
		body := failEnvelope(err)
		body["discovered"] = n
		body["skipped"] = skipNames(skipped)
		body["skippedNeedsAction"] = len(actionableSkips(skipped)) > 0
		writeJSON(w, http.StatusOK, body)
		return
	}
	if !probe {
		if rErr := h.reloadScheduler(); rErr != nil {
			log.Printf("api: zfs: reloading the scheduler after the discover failed: %v", rErr)
		}
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{
		"discovered":         n,
		"repo":               h.svc.DiscoverSource(zfsDomain),
		"skipped":            skipNames(skipped),
		"skippedNeedsAction": len(actionableSkips(skipped)) > 0,
		"rootsToCheck":       h.zfsRootsToCheck(),
	}))
}

// zfsRootsToCheck counts the rebuilt items whose root is really a child of a
// dataset that was never backed up, so the Discover result can say that the
// roots want a look. Two items under one parent that is not an item itself is
// what that looks like from the tags alone.
func (h *Handler) zfsRootsToCheck() int {
	items, err := h.store.ListZFSDatasets()
	if err != nil {
		return 0
	}
	isItem := make(map[string]bool, len(items))
	children := map[string]int{}
	for _, d := range items {
		isItem[d.Dataset] = true
	}
	for _, d := range items {
		if parent := zfsParentOf(d.Dataset); parent != "" && !isItem[parent] {
			children[parent]++
		}
	}
	n := 0
	for _, d := range items {
		if children[zfsParentOf(d.Dataset)] > 1 {
			n++
		}
	}
	return n
}

// zfsParentOf returns the dataset one level up, empty for a pool root.
func zfsParentOf(dataset string) string {
	i := strings.LastIndex(dataset, "/")
	if i <= 0 {
		return ""
	}
	return dataset[:i]
}
