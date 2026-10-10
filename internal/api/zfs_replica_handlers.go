package api

import (
	"errors"
	"log"
	"net/http"

	"github.com/junkerderprovinz/bombvault/internal/store"
	"github.com/junkerderprovinz/bombvault/internal/zfs"
	"github.com/junkerderprovinz/bombvault/internal/zfsrepl"
)

// registerZFSReplicaRoutes serves the ZFS servers a replica goes to and each
// item's replica. Reads answer bare, changes in the ZFS envelope.
func (h *Handler) registerZFSReplicaRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/zfs/replica/key", h.handleZFSReplicaKey)
	mux.HandleFunc("GET /api/zfs/replica/local-pools", h.handleZFSLocalPools)
	mux.HandleFunc("GET /api/zfs/replica/servers", h.handleListZFSReplicaServers)
	mux.HandleFunc("POST /api/zfs/replica/servers", h.handleCreateZFSReplicaServer)
	mux.HandleFunc("POST /api/zfs/replica/servers/test", h.handleTestZFSReplicaConnection)
	mux.HandleFunc("PATCH /api/zfs/replica/servers/{id}", h.handlePatchZFSReplicaServer)
	mux.HandleFunc("DELETE /api/zfs/replica/servers/{id}", h.handleDeleteZFSReplicaServer)
	mux.HandleFunc("POST /api/zfs/replica/servers/{id}/test", h.handleTestZFSReplicaServer)
	mux.HandleFunc("POST /api/zfs/replica/servers/{id}/forget-host-key", h.handleForgetZFSReplicaHostKey)
	mux.HandleFunc("GET /api/zfs/datasets/{id}/replica", h.handleGetZFSReplica)
	mux.HandleFunc("PATCH /api/zfs/datasets/{id}/replica", h.handlePatchZFSReplica)
	mux.HandleFunc("POST /api/zfs/datasets/{id}/replica/run", h.handleRunZFSReplica)
	mux.HandleFunc("POST /api/zfs/datasets/{id}/replica/restore", h.handleRestoreZFSReplica)
}

// handleZFSReplicaKey serves the public key a ZFS server has to accept.
// GET /api/zfs/replica/key
func (h *Handler) handleZFSReplicaKey(w http.ResponseWriter, _ *http.Request) {
	key, err := h.svc.ZFSReplicaPublicKey()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"publicKey": key})
}

// handleZFSLocalPools lists the pools of this instance's host. A host that
// does not answer lists none, which the pool picker shows as such.
// GET /api/zfs/replica/local-pools
func (h *Handler) handleZFSLocalPools(w http.ResponseWriter, r *http.Request) {
	pools, err := h.svc.ZFSLocalPools(r.Context())
	if err != nil {
		log.Printf("api: zfs replica: listing the local pools failed: %v", err)
	}
	if pools == nil {
		pools = []zfs.Pool{}
	}
	writeJSON(w, http.StatusOK, pools)
}

// GET /api/zfs/replica/servers
func (h *Handler) handleListZFSReplicaServers(w http.ResponseWriter, r *http.Request) {
	servers, err := h.svc.ListZFSReplicaServers(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, servers)
}

type zfsReplicaServerBody struct {
	Name string `json:"name"`
	Host string `json:"host"`
	User string `json:"user"`
	Port int    `json:"port"`
	Pool string `json:"pool"`
	Root string `json:"root"`
}

// POST /api/zfs/replica/servers
func (h *Handler) handleCreateZFSReplicaServer(w http.ResponseWriter, r *http.Request) {
	var body zfsReplicaServerBody
	if !decodeBody(w, r, &body) {
		return
	}
	v, err := h.svc.CreateZFSReplicaServer(store.ZFSReplicaServer{
		Name: body.Name, Host: body.Host, User: body.User, Port: body.Port, Pool: body.Pool, Root: body.Root,
	})
	if err != nil {
		zfsReplicaFail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(v.fields()))
}

// PATCH /api/zfs/replica/servers/{id}
func (h *Handler) handlePatchZFSReplicaServer(w http.ResponseWriter, r *http.Request) {
	id, ok := h.zfsIDParam(w, r)
	if !ok {
		return
	}
	var body ZFSReplicaServerPatch
	if !decodeBody(w, r, &body) {
		return
	}
	v, err := h.svc.PatchZFSReplicaServer(id, body)
	if err != nil {
		zfsReplicaFail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(v.fields()))
}

// handleDeleteZFSReplicaServer removes a server, refused with in-use while
// items replicate there unless detach=1 switches their replica off.
// DELETE /api/zfs/replica/servers/{id}?detach=1
func (h *Handler) handleDeleteZFSReplicaServer(w http.ResponseWriter, r *http.Request) {
	id, ok := h.zfsIDParam(w, r)
	if !ok {
		return
	}
	if err := h.svc.DeleteZFSReplicaServer(r.Context(), id, r.URL.Query().Get("detach") == "1"); err != nil {
		zfsReplicaFail(w, err)
		return
	}
	h.reloadAfterZFSReplicaChange()
	writeJSON(w, http.StatusOK, okEnvelope(nil))
}

// POST /api/zfs/replica/servers/test
func (h *Handler) handleTestZFSReplicaConnection(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Host string `json:"host"`
		User string `json:"user"`
		Port int    `json:"port"`
	}
	if !decodeBody(w, r, &body) {
		return
	}
	pools, err := h.svc.TestZFSReplicaConnection(r.Context(), body.Host, body.User, body.Port)
	writeZFSReplicaTest(w, pools, err)
}

// POST /api/zfs/replica/servers/{id}/test
func (h *Handler) handleTestZFSReplicaServer(w http.ResponseWriter, r *http.Request) {
	id, ok := h.zfsIDParam(w, r)
	if !ok {
		return
	}
	pools, err := h.svc.TestZFSReplicaServer(r.Context(), id)
	writeZFSReplicaTest(w, pools, err)
}

// POST /api/zfs/replica/servers/{id}/forget-host-key
func (h *Handler) handleForgetZFSReplicaHostKey(w http.ResponseWriter, r *http.Request) {
	id, ok := h.zfsIDParam(w, r)
	if !ok {
		return
	}
	if err := h.svc.ForgetZFSReplicaHostKey(id); err != nil {
		zfsReplicaFail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(nil))
}

func writeZFSReplicaTest(w http.ResponseWriter, pools []zfs.Pool, err error) {
	if err != nil {
		body := codedFailEnvelope(err, zfsReplicaCode(err))
		body["pools"] = []zfs.Pool{}
		writeJSON(w, http.StatusOK, body)
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"code": "ok", "pools": pools}))
}

// GET /api/zfs/datasets/{id}/replica
func (h *Handler) handleGetZFSReplica(w http.ResponseWriter, r *http.Request) {
	id, ok := h.zfsIDParam(w, r)
	if !ok {
		return
	}
	v, err := h.svc.ZFSReplicaView(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, v)
}

// PATCH /api/zfs/datasets/{id}/replica
func (h *Handler) handlePatchZFSReplica(w http.ResponseWriter, r *http.Request) {
	id, ok := h.zfsIDParam(w, r)
	if !ok {
		return
	}
	var body ZFSReplicaPatch
	if !decodeBody(w, r, &body) {
		return
	}
	if err := h.svc.PatchZFSReplica(r.Context(), id, body); err != nil {
		zfsReplicaFail(w, err)
		return
	}
	// The target, the switch to after each backup and the cadence all decide
	// whether the item has a cron entry of its own.
	if body.Target != nil || body.AfterBackup != nil || body.Cadence != nil {
		h.reloadAfterZFSReplicaChange()
	}
	writeJSON(w, http.StatusOK, okEnvelope(nil))
}

// handleRunZFSReplica starts a run and answers at once; it reports on the
// zfs-replica:<id> progress key. POST /api/zfs/datasets/{id}/replica/run
func (h *Handler) handleRunZFSReplica(w http.ResponseWriter, r *http.Request) {
	id, ok := h.zfsIDParam(w, r)
	if !ok {
		return
	}
	runID, err := h.svc.StartZFSReplica(r.Context(), id)
	if err != nil {
		zfsReplicaFail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"runId": runID}))
}

// handleRestoreZFSReplica brings a replica snapshot back into a new dataset
// next to the item's root, reporting on zfs-replica-restore:<id>.
// POST /api/zfs/datasets/{id}/replica/restore
func (h *Handler) handleRestoreZFSReplica(w http.ResponseWriter, r *http.Request) {
	id, ok := h.zfsIDParam(w, r)
	if !ok {
		return
	}
	var body struct {
		Snapshot string `json:"snapshot"`
	}
	if !decodeBody(w, r, &body) {
		return
	}
	ack, err := h.svc.StartZFSReplicaRestore(r.Context(), id, body.Snapshot)
	if err != nil {
		zfsReplicaFail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{
		"runId": ack.RunID, "dataset": ack.Dataset, "keyNeeded": ack.KeyNeeded,
	}))
}

// reloadAfterZFSReplicaChange gives the scheduler the replica cadences as
// they are now. The change itself is stored, so a reload that fails is only
// logged and the next settings save retries it.
func (h *Handler) reloadAfterZFSReplicaChange() {
	if err := h.reloadScheduler(); err != nil {
		log.Printf("api: zfs replica: reloading the scheduler failed: %v", err)
	}
}

// zfsReplicaFail answers a failure with its reason code when it has one: a
// refusal of the domain, of the engine or of a host. Anything else, such as a
// field that does not validate, answers with its text alone.
func zfsReplicaFail(w http.ResponseWriter, err error) {
	var rf *zfsrepl.Refusal
	var ce *zfs.CmdError
	var ne *zfs.NameError
	if _, ok := zfsRefusalCode(err); ok || errors.As(err, &rf) || errors.As(err, &ce) || errors.As(err, &ne) {
		writeJSON(w, http.StatusOK, codedFailEnvelope(err, zfsReplicaCode(err)))
		return
	}
	writeJSON(w, http.StatusOK, failEnvelope(err))
}
