package api

import (
	"maps"
	"net/http"

	"github.com/junkerderprovinz/bombvault/internal/backup"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// handleDeleteBackupsVM removes every backup of a VM from the selected source
// (local or off-site) in one go and prunes the freed space.
// DELETE /api/vms/{name}/backups?source=
func (h *Handler) handleDeleteBackupsVM(w http.ResponseWriter, r *http.Request) {
	name, ok := h.vmNameParam(w, r)
	if !ok {
		return
	}
	if err := h.svc.DeleteBackupsVM(r.Context(), name, sourceParam(r)); err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(nil))
}

// handleForgetVM clears the stale "Not installed" entry of a VM that is gone and
// has no backups, without touching any repo (DeleteBackupsVM handles one that
// still has snapshots). DELETE /api/vms/{name}
func (h *Handler) handleForgetVM(w http.ResponseWriter, r *http.Request) {
	name, ok := h.vmNameParam(w, r)
	if !ok {
		return
	}
	if err := h.svc.ForgetVMTarget(r.Context(), name); err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(nil))
}

// handleDiscoverVMs rebuilds the VM target list from backup storage, so a VM
// deleted from the host (or lost with the database) becomes restorable again.
func (h *Handler) handleDiscoverVMs(w http.ResponseWriter, r *http.Request) {
	probe := r.URL.Query().Get("probe") == "true" // read-only readiness check, see handleDiscover
	res, err := h.svc.DiscoverVMs(r.Context(), probe)
	if err != nil {
		// The partial result goes along, as in handleDiscover.
		body := failEnvelope(err)
		maps.Copy(body, discoverFields(res))
		writeJSON(w, http.StatusOK, body)
		return
	}
	fields := discoverFields(res)
	fields["repo"] = h.svc.DiscoverSource("vms")
	writeJSON(w, http.StatusOK, okEnvelope(fields))
}

// handleGetVmBackupOrder returns the explicit VM backup ordering.
// GET /api/vms/backup-order
func (h *Handler) handleGetVmBackupOrder(w http.ResponseWriter, r *http.Request) {
	orders, err := h.svc.VMBackupOrders(r.Context())
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	if orders == nil {
		orders = []store.VMOrder{}
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"order": orders}))
}

// handleSetVmBackupOrder replaces the VM backup ordering from a list of VM
// names, the first running earliest in a scheduled VM run.
// A VM omitted from the list returns to the name-order tiebreak; an empty list
// clears all explicit orders.
// PUT /api/vms/backup-order  body {order: ["vmA", "vmB", ...]}
func (h *Handler) handleSetVmBackupOrder(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Order []string `json:"order"`
	}
	if !decodeBody(w, r, &body) { // caps the body at 1 MiB
		return
	}
	if len(body.Order) > 1000 { // far beyond any real VM count
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": "too many vms"})
		return
	}
	for _, n := range body.Order {
		if !validVMName(n) { // VM names may contain spaces ("Windows 11")
			writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": "invalid VM name"})
			return
		}
	}
	if err := h.svc.SetVMBackupOrders(r.Context(), body.Order); err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(nil))
}

func (h *Handler) handleListVMs(w http.ResponseWriter, r *http.Request) {
	views, err := h.svc.ListVMs(r.Context())
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	if views == nil {
		views = []VMView{}
	}
	targets, _ := h.store.ListVMTargets()
	byName := make(map[string]store.VMTarget, len(targets))
	for _, t := range targets {
		byName[t.Name] = t
	}
	items := make([]placementItem, 0, len(views))
	for _, v := range views {
		it := placementItem{Key: v.LibvirtName, Identity: "vm:" + v.LibvirtName}
		if t, ok := byName[v.LibvirtName]; ok {
			it.Home = store.HomeState{Exists: true, Repo: t.Repo, Choice: t.RepoChosen}
			if run, _ := h.store.LastSuccessfulBackup(t.ID); run != nil {
				it.LastSuccess = run.StartedAt
			}
		}
		items = append(items, it)
	}
	placements := h.svc.listPlacements(r.Context(), "vms", items)
	for i := range views {
		views[i].Placement = placements[views[i].LibvirtName]
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "vms": views})
}

// handleBackupVM starts a single VM backup on the server and returns
// immediately, like handleBackup. The SPA follows "vm:<name>" over SSE.
func (h *Handler) handleBackupVM(w http.ResponseWriter, r *http.Request) {
	name, ok := h.vmNameParam(w, r)
	if !ok {
		return
	}
	started, err := h.svc.StartBackupVM(r.Context(), name)
	if err != nil { // the vms domain is busy with another op → 409 with the reason
		writeJSON(w, http.StatusConflict, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	if !started {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": "a backup is already running"})
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"started": true}))
}

func (h *Handler) handleSnapshotsVM(w http.ResponseWriter, r *http.Request) {
	name, ok := h.vmNameParam(w, r)
	if !ok {
		return
	}
	snaps, err := h.svc.SnapshotsVM(r.Context(), name, sourceParam(r))
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	if snaps == nil {
		snaps = []restic.Snapshot{}
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"snapshots": snaps}))
}

// handleRestoreVM starts a VM restore on the server and returns immediately,
// like handleRestore. The SPA follows "vm:<name>" over SSE and reads the
// recorded run for the outcome.
func (h *Handler) handleRestoreVM(w http.ResponseWriter, r *http.Request) {
	name, ok := h.vmNameParam(w, r)
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
	// sentinel; the service checks again.
	if !body.Confirm {
		writeJSON(w, http.StatusOK, failEnvelope(backup.ErrNotConfirmed))
		return
	}
	started, err := h.svc.StartRestoreVM(r.Context(), name, body.SnapshotID, sourceParam(r), body.LeaveStopped)
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

func (h *Handler) handlePatchVM(w http.ResponseWriter, r *http.Request) {
	name, ok := h.vmNameParam(w, r)
	if !ok {
		return
	}
	var body struct {
		Method            *string `json:"method"`
		IncludeInSchedule *bool   `json:"includeInSchedule"`
		ScheduleCadence   *string `json:"scheduleCadence"`
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
	item := store.ItemRef{Domain: "vms", Key: name}
	if err := h.svc.checkPlacementChange(r.Context(), item, change); err != nil {
		placementFail(w, err, nil)
		return
	}
	if body.Method != nil {
		if err := h.svc.SetVMMethod(r.Context(), name, *body.Method); err != nil {
			writeJSON(w, http.StatusOK, failEnvelope(err))
			return
		}
	}
	if body.IncludeInSchedule != nil {
		if err := h.svc.SetVMInclude(r.Context(), name, *body.IncludeInSchedule); err != nil {
			writeJSON(w, http.StatusOK, failEnvelope(err))
			return
		}
	}
	if body.ScheduleCadence != nil {
		if err := h.svc.SetVMScheduleCadence(r.Context(), name, *body.ScheduleCadence); err != nil {
			writeJSON(w, http.StatusOK, failEnvelope(err))
			return
		}
		// A per-item cadence adds or removes the VM's own cron entry, and a VM
		// PATCH does not otherwise reload the scheduler.
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
		"placement": h.svc.placementViewOf(r.Context(), store.ItemRef{Domain: "vms", Key: name}),
	}))
}

// handleVMScheduleIncludeAll sets the include_in_schedule flag for every VM on
// the host in one call, like handleScheduleIncludeAll. Excluding also reaches
// VMs that are not defined any more.
// POST /api/vms/schedule-include  body {include: bool}
func (h *Handler) handleVMScheduleIncludeAll(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Include bool `json:"include"`
	}
	if !decodeBody(w, r, &body) {
		return
	}
	if err := h.svc.SetVMIncludeAll(r.Context(), body.Include); err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(nil))
}

func (h *Handler) handleVMSSHInfo(w http.ResponseWriter, r *http.Request) {
	host, pub, err := h.svc.VMSSHInfo()
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"host": host, "publicKey": pub}))
}

func (h *Handler) handleVMSSHTest(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.VMSSHTest(r.Context()); err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(nil))
}
