package api

import (
	"errors"
	"net/http"

	"github.com/junkerderprovinz/bombvault/internal/hostsnap"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

func (h *Handler) registerHostSnapshotRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/host-snapshots", h.handleListHostSnapshots)
	mux.HandleFunc("POST /api/host-snapshots/create", h.handleCreateHostSnapshot)
	mux.HandleFunc("POST /api/host-snapshots/rename", h.handleRenameHostSnapshot)
	mux.HandleFunc("POST /api/host-snapshots/hold", h.handleHoldHostSnapshot)
	mux.HandleFunc("POST /api/host-snapshots/estimate", h.handleEstimateHostSnapshots)
	mux.HandleFunc("POST /api/host-snapshots/delete", h.handleDeleteHostSnapshots)
	mux.HandleFunc("POST /api/host-snapshots/where", h.handleWhereHostSnapshot)
	mux.HandleFunc("POST /api/host-snapshots/free", h.handleFreeHostSnapshot)
	mux.HandleFunc("POST /api/host-snapshots/plans", h.handleAddHostSnapshotPlan)
	mux.HandleFunc("PUT /api/host-snapshots/plans/{id}", h.handleUpdateHostSnapshotPlan)
	mux.HandleFunc("DELETE /api/host-snapshots/plans/{id}", h.handleDeleteHostSnapshotPlan)
	mux.HandleFunc("POST /api/host-snapshots/plans/{id}/run", h.handleRunHostSnapshotPlan)
}

// hostSnapFail answers a refusal with its reason code, so the page builds the
// sentence.
func hostSnapFail(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, store.ErrHostSnapshotPlanNotFound):
		writeJSON(w, http.StatusNotFound, codedFailEnvelope(err, "not-found"))
	case errors.Is(err, errPlanCadence), errors.Is(err, errPlanKeep), errors.Is(err, errPlanSource):
		writeJSON(w, http.StatusOK, codedFailEnvelope(err, "invalid-plan"))
	default:
		writeJSON(w, http.StatusOK, codedFailEnvelope(err, hostsnap.CodeOf(err)))
	}
}

// handleListHostSnapshots lists the server's own snapshots. Pools whose disks
// sleep are only read with wake=1. GET /api/host-snapshots?wake=1
func (h *Handler) handleListHostSnapshots(w http.ResponseWriter, r *http.Request) {
	view, err := h.svc.HostSnapshots(r.Context(), r.URL.Query().Get("wake") == "1")
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{
		"kinds": view.Kinds, "pools": view.Pools, "sources": view.Sources, "snapshots": view.Snapshots,
		"hiddenSystem": view.HiddenSystem, "plans": view.Plans,
	}))
}

// POST /api/host-snapshots/create
func (h *Handler) handleCreateHostSnapshot(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Kind      hostsnap.Kind `json:"kind"`
		FS        string        `json:"fs"`
		Source    string        `json:"source"`
		Name      string        `json:"name"`
		Recursive bool          `json:"recursive"`
	}
	if !decodeBody(w, r, &body) || !h.requireSignIn(w, r, "taking a snapshot") {
		return
	}
	src := hostsnap.Source{Kind: body.Kind, FS: body.FS, Name: body.Source}
	if err := h.svc.CreateHostSnapshot(r.Context(), src, body.Name, body.Recursive); err != nil {
		hostSnapFail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(nil))
}

// POST /api/host-snapshots/rename
func (h *Handler) handleRenameHostSnapshot(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Ref  hostsnap.Ref `json:"ref"`
		Name string       `json:"name"`
	}
	if !decodeBody(w, r, &body) || !h.requireSignIn(w, r, "renaming a snapshot") {
		return
	}
	if err := h.svc.RenameHostSnapshot(r.Context(), body.Ref, body.Name); err != nil {
		hostSnapFail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(nil))
}

// POST /api/host-snapshots/hold
func (h *Handler) handleHoldHostSnapshot(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Ref  hostsnap.Ref `json:"ref"`
		Hold bool         `json:"hold"`
	}
	if !decodeBody(w, r, &body) || !h.requireSignIn(w, r, "holding a snapshot") {
		return
	}
	if err := h.svc.HoldHostSnapshot(r.Context(), body.Ref, body.Hold); err != nil {
		hostSnapFail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(nil))
}

func decodeRefs(w http.ResponseWriter, r *http.Request) ([]hostsnap.Ref, bool) {
	var body struct {
		Refs []hostsnap.Ref `json:"refs"`
	}
	if !decodeBody(w, r, &body) {
		return nil, false
	}
	if len(body.Refs) == 0 || len(body.Refs) > hostSnapMaxRefs {
		writeJSON(w, http.StatusOK, codedFailEnvelope(errors.New("between 1 and 500 snapshots at a time"), "invalid-selection"))
		return nil, false
	}
	return body.Refs, true
}

// POST /api/host-snapshots/estimate
func (h *Handler) handleEstimateHostSnapshots(w http.ResponseWriter, r *http.Request) {
	refs, ok := decodeRefs(w, r)
	if !ok {
		return
	}
	n, err := h.svc.EstimateHostSnapshots(r.Context(), refs)
	if err != nil {
		hostSnapFail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"zfsBytes": n}))
}

// POST /api/host-snapshots/delete
func (h *Handler) handleDeleteHostSnapshots(w http.ResponseWriter, r *http.Request) {
	refs, ok := decodeRefs(w, r)
	if !ok || !h.requireSignIn(w, r, "deleting snapshots") {
		return
	}
	results, err := h.svc.DeleteHostSnapshots(r.Context(), refs)
	if err != nil {
		hostSnapFail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"results": results}))
}

// POST /api/host-snapshots/where
func (h *Handler) handleWhereHostSnapshot(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Ref hostsnap.Ref `json:"ref"`
	}
	if !decodeBody(w, r, &body) {
		return
	}
	mounts, err := h.svc.WhereHostSnapshot(r.Context(), body.Ref)
	if err != nil {
		hostSnapFail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"mounts": mounts}))
}

// POST /api/host-snapshots/free
func (h *Handler) handleFreeHostSnapshot(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Ref hostsnap.Ref `json:"ref"`
	}
	if !decodeBody(w, r, &body) || !h.requireSignIn(w, r, "freeing and deleting a snapshot") {
		return
	}
	mounts, err := h.svc.FreeHostSnapshot(r.Context(), body.Ref)
	if mounts == nil {
		mounts = []hostsnap.Mount{}
	}
	if err != nil {
		env := codedFailEnvelope(err, hostsnap.CodeOf(err))
		env["mounts"] = mounts
		writeJSON(w, http.StatusOK, env)
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"mounts": mounts}))
}

func (h *Handler) planIDParam(w http.ResponseWriter, r *http.Request) (string, bool) {
	id := r.PathValue("id")
	if !validResourceName(id) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "invalid plan id"})
		return "", false
	}
	return id, true
}

// POST /api/host-snapshots/plans
func (h *Handler) handleAddHostSnapshotPlan(w http.ResponseWriter, r *http.Request) {
	var in HostSnapshotPlanInput
	if !decodeBody(w, r, &in) || !h.requireSignIn(w, r, "adding a snapshot plan") {
		return
	}
	p, err := h.svc.AddHostSnapshotPlan(in)
	if err != nil {
		hostSnapFail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"plan": p}))
}

// PUT /api/host-snapshots/plans/{id}
func (h *Handler) handleUpdateHostSnapshotPlan(w http.ResponseWriter, r *http.Request) {
	id, ok := h.planIDParam(w, r)
	if !ok {
		return
	}
	var in HostSnapshotPlanInput
	if !decodeBody(w, r, &in) || !h.requireSignIn(w, r, "changing a snapshot plan") {
		return
	}
	p, err := h.svc.UpdateHostSnapshotPlan(id, in)
	if err != nil {
		hostSnapFail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"plan": p}))
}

// DELETE /api/host-snapshots/plans/{id}
func (h *Handler) handleDeleteHostSnapshotPlan(w http.ResponseWriter, r *http.Request) {
	id, ok := h.planIDParam(w, r)
	if !ok || !h.requireSignIn(w, r, "removing a snapshot plan") {
		return
	}
	if err := h.svc.DeleteHostSnapshotPlan(id); err != nil {
		hostSnapFail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(nil))
}

// POST /api/host-snapshots/plans/{id}/run
func (h *Handler) handleRunHostSnapshotPlan(w http.ResponseWriter, r *http.Request) {
	id, ok := h.planIDParam(w, r)
	if !ok || !h.requireSignIn(w, r, "running a snapshot plan") {
		return
	}
	res, err := h.svc.RunHostSnapshotPlan(r.Context(), id)
	if err != nil {
		hostSnapFail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"result": res}))
}
