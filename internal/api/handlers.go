package api

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/backup"
	"github.com/junkerderprovinz/bombvault/internal/notify"
	"github.com/junkerderprovinz/bombvault/internal/paths"
	"github.com/junkerderprovinz/bombvault/internal/releasenotes"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/schedule"
	"github.com/junkerderprovinz/bombvault/internal/secret"
	"github.com/junkerderprovinz/bombvault/internal/spike"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// ---------------------------------------------------------------------------
// JSON helpers + error scrubbing
// ---------------------------------------------------------------------------

// writeJSON encodes v as JSON with the given status code.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("api: encode response: %v", err)
	}
}

// okEnvelope returns a success envelope merged with extra fields.
func okEnvelope(extra map[string]any) map[string]any {
	m := map[string]any{"ok": true}
	for k, v := range extra {
		m[k] = v
	}
	return m
}

// failEnvelope returns a graceful failure envelope. The error is scrubbed so no
// repo path or secret leaks to the client (defense-in-depth; the restic/docker
// adapters already scrub their own errors).
func failEnvelope(err error) map[string]any {
	return map[string]any{"ok": false, "error": scrubError(err)}
}

// absPathRe matches absolute unix paths so they can be stripped from any error
// message that slips through to the API surface.
var absPathRe = regexp.MustCompile(`(/[^\s:"']+)+`)

// scrubError maps known sentinels to clear messages and strips absolute paths
// from anything else.
func scrubError(err error) string {
	switch {
	case err == nil:
		return ""
	case errors.Is(err, backup.ErrNotConfirmed):
		return "restore not confirmed — set confirm:true to proceed"
	case errors.Is(err, backup.ErrInvalidSnapshotID):
		return "invalid snapshot id (must be 8–64 lowercase hex)"
	case errors.Is(err, backup.ErrRestoreConflict):
		// Already user-safe (IP / host-port / container names, no host paths) and
		// must bypass the path scrubber, which would mangle "8080/tcp" → "8080[path]".
		return err.Error()
	}
	msg := err.Error()
	// Map restic's password/key mismatch to an actionable hint: the repo was
	// created with a different APP_KEY or a different encryption setting.
	if strings.Contains(msg, "wrong password or no key found") {
		return "backup repository can't be opened — the APP_KEY differs from when this repo was first created (or encryption was toggled). Use the original APP_KEY, or point Settings at a fresh, empty backup path."
	}
	msg = absPathRe.ReplaceAllString(msg, "[path]")
	return strings.TrimSpace(msg)
}

// decodeBody decodes a JSON request body into v. Returns false (and writes a
// graceful failure) on malformed JSON.
func decodeBody(w http.ResponseWriter, r *http.Request, v any) bool {
	if r.Body == nil {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": "missing request body"})
		return false
	}
	// Cap the request body so a giant payload (e.g. an enormous hook or rclone
	// config) can't exhaust memory.
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1 MiB
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": "invalid request body"})
		return false
	}
	return true
}

// ---------------------------------------------------------------------------
// handlers
// ---------------------------------------------------------------------------

func (h *Handler) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "version": Version})
}

// handleMetrics serves the opt-in Prometheus /metrics endpoint (GET /metrics).
// It bypasses the session authGate (Prometheus can't carry the cookie) and is
// gated by its own settings instead:
//   - metrics disabled            → 404 (not served at all)
//   - a metrics token is set      → require Authorization: Bearer <token>
//     (constant-time compare), else 401
//   - no token                    → open (LAN trust model, like /api/health)
//
// Only non-sensitive operational metrics are exposed (no repo paths, secrets, or
// hostnames). The response is Prometheus text exposition format.
func (h *Handler) handleMetrics(w http.ResponseWriter, r *http.Request) {
	enabled, token, err := h.svc.MetricsAccess()
	if err != nil {
		// Fail closed: a store error must not silently expose or 200 the endpoint.
		log.Printf("api: metrics: settings read failed: %v", err)
		http.Error(w, "metrics unavailable", http.StatusServiceUnavailable)
		return
	}
	if !enabled {
		http.NotFound(w, r) // opt-in: not served when disabled
		return
	}
	if token != "" {
		const prefix = "Bearer "
		got := r.Header.Get("Authorization")
		ok := strings.HasPrefix(got, prefix) &&
			subtle.ConstantTimeCompare([]byte(strings.TrimPrefix(got, prefix)), []byte(token)) == 1
		if !ok {
			w.Header().Set("WWW-Authenticate", `Bearer realm="metrics"`)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
	}

	body, err := h.svc.Metrics()
	if err != nil {
		log.Printf("api: metrics: build failed: %v", err)
		http.Error(w, "metrics error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", metricsContentType)
	w.WriteHeader(http.StatusOK)
	if _, wErr := w.Write([]byte(body)); wErr != nil {
		log.Printf("api: metrics: write failed: %v", wErr)
	}
}

// containerView is the per-container row returned by GET /api/containers.
// Installed is false for "orphan" rows: containers that are no longer installed
// on the host but still have backups (so the user can restore or delete them).
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
	// Stack is the compose project (com.docker.compose.project label) this
	// container belongs to, "" if none. Drives the "restore whole stack" panel.
	Stack string `json:"stack"`
	// Self marks BombVault's own container: the UI hides its backup action and
	// excludes it from "select all" so a batch can never stop the app itself.
	Self bool `json:"self"`
}

func (h *Handler) handleListContainers(w http.ResponseWriter, r *http.Request) {
	infos, err := h.docker.List(r.Context())
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}

	// Index targets by name for include flag + last backup.
	targets, _ := h.store.ListTargets()
	byName := make(map[string]store.Target, len(targets))
	for _, t := range targets {
		byName[t.ContainerName] = t
	}

	self := h.svc.SelfContainerName(r.Context())

	live := make(map[string]bool, len(infos))
	views := make([]containerView, 0, len(infos)+len(targets))
	for _, c := range infos {
		live[c.Name] = true
		v := containerView{
			Name:      c.Name,
			Image:     c.Image,
			State:     c.State,
			Status:    c.Status,
			IP:        c.IP,
			Installed: true,
			Stack:     c.Stack,
			Self:      self != "" && c.Name == self,
		}
		if t, ok := byName[c.Name]; ok {
			v.IncludeInSchedule = t.IncludeInSchedule
			v.PreHook = t.PreHook
			v.PostHook = t.PostHook
			v.StopContainers = t.StopContainers
			v.Excludes = t.Excludes
			v.UpdateAfterBackup = t.UpdateAfterBackup
			if run, _ := h.store.LastSuccessfulBackup(t.ID); run != nil {
				v.LastBackup = run.FinishedAt
				v.LastBackupStarted = &run.StartedAt
			}
		}
		views = append(views, v)
	}

	// Orphans: targets with backups whose container is no longer installed. The
	// image comes from the stored recreate definition (so the row is recognisable
	// even though the container is gone).
	//
	// A Discover-rebuilt orphan has a fresh target id with NO run record, so its
	// run-based "last backup" is nil and would read "Never" despite having
	// snapshots (#44). Fall back to the newest snapshot's time — listed once, and
	// only when an orphan actually exists.
	var snapTimes map[string]int64
	for _, t := range targets {
		if !live[t.ContainerName] {
			if m, sErr := h.svc.LatestContainerBackupTimes(r.Context()); sErr != nil {
				log.Printf("api: list containers: latest backup times: %v", sErr)
			} else {
				snapTimes = m
			}
			break
		}
	}
	for _, t := range targets {
		if live[t.ContainerName] {
			continue
		}
		v := containerView{
			Name:              t.ContainerName,
			State:             "not-installed",
			Installed:         false,
			IncludeInSchedule: t.IncludeInSchedule,
		}
		if t.Definition != "" {
			var def containerDefinition
			if json.Unmarshal([]byte(t.Definition), &def) == nil {
				v.Image = def.Inspect.Config.Image
				v.Stack = def.Inspect.Config.Labels["com.docker.compose.project"]
			}
		}
		if run, _ := h.store.LastSuccessfulBackup(t.ID); run != nil {
			v.LastBackup = run.FinishedAt
			v.LastBackupStarted = &run.StartedAt
		} else if ts, ok := snapTimes[t.ContainerName]; ok && ts > 0 {
			// No run record (Discover-rebuilt target) but snapshots exist → show the
			// newest snapshot's time instead of "Never" (#44).
			tsCopy := ts
			v.LastBackup = &tsCopy
		}
		views = append(views, v)
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "containers": views})
}

// resourceNameRe matches a safe Docker container / libvirt VM name: it starts
// with an alphanumeric and contains only [A-Za-z0-9._-]. This forbids path
// separators, a leading "-" (argv option-injection) and an empty name; the
// extra ".." check forbids parent-dir traversal even within the charset. The
// Go 1.22 router decodes "%2f"/"%2e%2e" into the path value, so an unvalidated
// {name} could otherwise carry "../" into the template/XML file sinks (CWE-22).
var resourceNameRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)

func validResourceName(name string) bool {
	return resourceNameRe.MatchString(name) && !strings.Contains(name, "..")
}

// nameParam extracts and validates the {name} path value, writing a 400 and
// returning ok=false when it is unsafe. Every name-keyed handler calls this at
// the boundary so no traversal/option-injection name reaches the service layer.
func (h *Handler) nameParam(w http.ResponseWriter, r *http.Request) (string, bool) {
	name := r.PathValue("name")
	if !validResourceName(name) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "invalid name"})
		return "", false
	}
	return name, true
}

// validVMName accepts libvirt domain names, which (unlike Docker container
// names) routinely contain spaces — e.g. "Windows 11", "Home Assistant". The VM
// name never becomes a filesystem path or template filename (it only flows into
// argv-separated virsh args, restic tags after "--", and SQLite params), so the
// strict resourceNameRe is wrong here. We still block what could be dangerous:
// empty, over-long, path separators / "..", a leading "-" (option injection),
// and control characters.
func validVMName(name string) bool {
	if name == "" || len(name) > 128 {
		return false
	}
	if strings.HasPrefix(name, "-") || strings.Contains(name, "..") || strings.ContainsAny(name, "/\\") {
		return false
	}
	for _, r := range name {
		if r < 0x20 || r == 0x7f {
			return false
		}
	}
	return true
}

// vmNameParam is nameParam for VM routes — it uses the libvirt-aware validator
// so VMs with spaces in their names are not rejected with a 400.
func (h *Handler) vmNameParam(w http.ResponseWriter, r *http.Request) (string, bool) {
	name := r.PathValue("name")
	if !validVMName(name) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "invalid VM name"})
		return "", false
	}
	return name, true
}

// handleDeleteBackups removes ALL backups of a container and forgets it from the
// store. Used for containers that are no longer installed.
func (h *Handler) handleDeleteBackups(w http.ResponseWriter, r *http.Request) {
	name, ok := h.nameParam(w, r)
	if !ok {
		return
	}
	if err := h.svc.DeleteBackups(r.Context(), name); err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(nil))
}

// handleDeleteBackupsVM removes ALL backups of a VM from the selected source
// (local or off-site) in one go and prunes the freed space. The one-shot
// counterpart to deleting each snapshot individually per source.
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

// handleForgetVM clears a VM's stale "Not installed" entry (its target row),
// without touching any repo — for a no-longer-defined VM that has no backups
// (DeleteBackupsVM handles VMs that still have snapshots). DELETE /api/vms/{name}
func (h *Handler) handleForgetVM(w http.ResponseWriter, r *http.Request) {
	name, ok := h.vmNameParam(w, r)
	if !ok {
		return
	}
	if err := h.svc.ForgetVMTarget(name); err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(nil))
}

// handleDiscover rebuilds the target list from the backup storage (disaster
// recovery after a fresh install / loss of /config).
func (h *Handler) handleDiscover(w http.ResponseWriter, r *http.Request) {
	// ?probe=true = a read-only readability check (Recovery tab): open + decrypt
	// to prove the repo/APP_KEY, but write no targets — so a readiness check never
	// resurrects orphan entries. The default (no probe) is the real rebuild (#44).
	probe := r.URL.Query().Get("probe") == "true"
	n, err := h.svc.Discover(r.Context(), probe)
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"discovered": n}))
}

// handleDiscoverVMs rebuilds the VM target list from backup storage, so a VM
// deleted from the host (or lost with the database) becomes restorable again.
func (h *Handler) handleDiscoverVMs(w http.ResponseWriter, r *http.Request) {
	probe := r.URL.Query().Get("probe") == "true" // read-only readiness check, see handleDiscover (#44)
	n, err := h.svc.DiscoverVMs(r.Context(), probe)
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"discovered": n}))
}

// handleBackup starts a single container backup ON THE SERVER and returns
// immediately. The work runs in the background (independent of this request) so
// a long backup — or backing up the reverse-proxy container the UI runs through,
// which severs this connection — can't make the SPA report a phantom failure for
// a backup the server actually completes. The SPA watches the "container:<name>"
// progress key over SSE and reads the recorded run for the outcome.
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

// handleBackupAll starts a SERVER-SIDE batch backup of the selected containers.
// The work runs in the background (independent of this request) so closing the
// browser — even stopping the container the UI runs in — can't interrupt it; the
// SPA watches progress over SSE ("batch:containers" + per-container keys).
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
	if len(body.Names) > 1000 { // far beyond any real container count — reject abuse
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

// sourceParam returns the requested repo source from the ?source= query:
// "offsite" selects the off-site replica, anything else (incl. absent) is the
// local repo. Used by the snapshot-browser, restore and maintenance endpoints.
func sourceParam(r *http.Request) string {
	if r.URL.Query().Get("source") == "offsite" {
		return "offsite"
	}
	return "local"
}

// kindParam extracts the drill kind from the query: "dr" selects a real off-site
// sandbox-restore drill; anything else (incl. absent) is the classic "subset"
// integrity check. Used by POST /api/verify/{domain}.
func kindParam(r *http.Request) string {
	if r.URL.Query().Get("kind") == "dr" {
		return "dr"
	}
	return "subset"
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

// handleRestore starts an in-place container restore ON THE SERVER and returns
// immediately (see handleBackup — restores got the same treatment in issue #24:
// a multi-hour restore held this request open until the browser/proxy dropped
// it, which canceled the context and killed restic mid-restore). Validation
// still runs synchronously, so a bad request fails right away; the SPA watches
// the "container:<name>" progress key over SSE and reads the recorded run for
// the outcome.
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
	// Confirmation is guarded here so an unconfirmed request fails synchronously
	// with the familiar sentinel (the sync service core re-checks it for the
	// stack-restore path — defense-in-depth).
	if !body.Confirm {
		writeJSON(w, http.StatusOK, failEnvelope(backup.ErrNotConfirmed))
		return
	}
	started, err := h.svc.StartRestore(r.Context(), name, body.SnapshotID, sourceParam(r), body.LeaveStopped)
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
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

// handleRestoreStack restores every backed-up member of a compose stack STOPPED,
// then (optionally) starts them in dependency order. POST /api/stacks/{project}/restore
// The {project} is a compose project name, which is laxer than a container name
// (validResourceName would wrongly reject some), so it gets its own minimal check
// that still blocks path traversal / separators reaching the store enumeration.
//
// ASYNC (see handleRestore): validation + member enumeration run synchronously
// (a bad request — including an empty stack — still fails right away); the
// per-member restore + start loops run detached. Per-member outcomes land in
// the run history (each member's restore records a kind "restore" run).
func (h *Handler) handleRestoreStack(w http.ResponseWriter, r *http.Request) {
	project := r.PathValue("project")
	if project == "" || strings.Contains(project, "/") || strings.Contains(project, "..") {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "invalid stack name"})
		return
	}
	var body struct {
		StartAfter bool `json:"startAfter"`
		Confirm    bool `json:"confirm"`
	}
	if !decodeBody(w, r, &body) {
		return
	}
	started, err := h.svc.StartRestoreStack(r.Context(), project, sourceParam(r), body.StartAfter, body.Confirm)
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	if !started {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": "a backup or restore is already running"})
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"started": true}))
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
		writeJSON(w, http.StatusOK, failEnvelope(err))
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
// ASYNC (see handleRestore): validation + target resolution run synchronously
// (the resolved target is returned in the ack); the restic work runs detached,
// publishing "container:<name>" progress and recording a run for the outcome.
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
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	if !started {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": "a backup or restore is already running"})
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"started": true, "target": target}))
}

// handleRestoreContainerTo extracts a whole container snapshot into an ALTERNATE
// folder under the host mount (non-destructive — the live container is never
// touched). POST /api/containers/{name}/restore-to
//
// ASYNC (see handleRestore — this is THE flow of issue #24: a 700GB extraction
// held the request open for hours until the connection dropped and killed
// restic). Validation + target resolution run synchronously (the resolved
// target is returned in the ack); the restic work runs detached, publishing
// "container:<name>" progress and recording a run for the outcome.
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
		writeJSON(w, http.StatusOK, failEnvelope(err))
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
	// Pointers so a hooks-only PATCH doesn't reset the schedule flag (and vice
	// versa) — only the fields actually sent are applied.
	var body struct {
		IncludeInSchedule *bool     `json:"includeInSchedule"`
		PreHook           *string   `json:"preHook"`
		PostHook          *string   `json:"postHook"`
		BackupPaths       *[]string `json:"backupPaths"`
		StopContainers    *[]string `json:"stopContainers"`
		Excludes          *[]string `json:"excludes"`
		UpdateAfterBackup *bool     `json:"updateAfterBackup"`
	}
	if !decodeBody(w, r, &body) {
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
		if err := h.svc.SetBackupPaths(r.Context(), name, *body.BackupPaths); err != nil {
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
	if body.UpdateAfterBackup != nil {
		if err := h.svc.SetUpdateAfterBackup(r.Context(), name, *body.UpdateAfterBackup); err != nil {
			writeJSON(w, http.StatusOK, failEnvelope(err))
			return
		}
	}
	writeJSON(w, http.StatusOK, okEnvelope(nil))
}

// handleScheduleIncludeAll sets the include_in_schedule flag for EVERY installed
// container in one call — the one-click "include all in schedule" / "exclude all"
// action. POST /api/containers/schedule-include  body {include: bool}
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

// handleContainerMounts lists a container's bind mounts (annotated with the
// current selection) for the backup-folder selector.
// GET /api/containers/{name}/mounts
func (h *Handler) handleContainerMounts(w http.ResponseWriter, r *http.Request) {
	name, ok := h.nameParam(w, r)
	if !ok {
		return
	}
	mounts, custom, err := h.svc.ContainerMounts(r.Context(), name)
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	if mounts == nil {
		mounts = []MountInfo{}
	}
	if custom == nil {
		custom = []string{}
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"mounts": mounts, "custom": custom}))
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

// strOr returns *p or "" when p is nil.
func strOr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// settingsView is the JSON shape for GET/PUT /api/settings.
type settingsView struct {
	EncryptionEnabled         bool   `json:"encryptionEnabled"`
	ContainersEnabled         bool   `json:"containersEnabled"`
	VMsEnabled                bool   `json:"vmsEnabled"`
	FlashEnabled              bool   `json:"flashEnabled"`
	ConfigEnabled             bool   `json:"configEnabled"`
	FilesEnabled              bool   `json:"filesEnabled"`
	ContainersPath            string `json:"containersPath"`
	VMsPath                   string `json:"vmsPath"`
	FlashPath                 string `json:"flashPath"`
	ConfigPath                string `json:"configPath"`
	FilesPath                 string `json:"filesPath"`
	RestoreFolder             string `json:"restoreFolder"`
	ContainersOffsite         string `json:"containersOffsite"`
	VMsOffsite                string `json:"vmsOffsite"`
	FlashOffsite              string `json:"flashOffsite"`
	ConfigOffsite             string `json:"configOffsite"`
	FilesOffsite              string `json:"filesOffsite"`
	ContainersOffsiteSchedule string `json:"containersOffsiteSchedule"`
	VMsOffsiteSchedule        string `json:"vmsOffsiteSchedule"`
	FlashOffsiteSchedule      string `json:"flashOffsiteSchedule"`
	ConfigOffsiteSchedule     string `json:"configOffsiteSchedule"`
	FilesOffsiteSchedule      string `json:"filesOffsiteSchedule"`
	ContainersSchedule        string `json:"containersSchedule"`
	VMsSchedule               string `json:"vmsSchedule"`
	FlashSchedule             string `json:"flashSchedule"`
	ConfigSchedule            string `json:"configSchedule"`
	FilesSchedule             string `json:"filesSchedule"`
	// Scheduled flash ZIP export: enable, destination folder (relative subpath
	// under the mount root), and how many timestamped zips to keep (0 = a single
	// overwriting flash-latest.zip).
	FlashZipExportEnabled bool   `json:"flashZipExportEnabled"`
	FlashZipExportPath    string `json:"flashZipExportPath"`
	FlashZipExportKeep    int    `json:"flashZipExportKeep"`
	DefaultLanguage       string `json:"defaultLanguage"`
	// Retention keep-policy (0 = that dimension off; all 0 = retention off).
	RetentionKeepLast    int `json:"retentionKeepLast"`
	RetentionKeepDaily   int `json:"retentionKeepDaily"`
	RetentionKeepWeekly  int `json:"retentionKeepWeekly"`
	RetentionKeepMonthly int `json:"retentionKeepMonthly"`
	// Separate off-site retention keep-policy (all 0 = off-site keeps everything).
	OffsiteRetentionKeepLast    int `json:"offsiteRetentionKeepLast"`
	OffsiteRetentionKeepDaily   int `json:"offsiteRetentionKeepDaily"`
	OffsiteRetentionKeepWeekly  int `json:"offsiteRetentionKeepWeekly"`
	OffsiteRetentionKeepMonthly int `json:"offsiteRetentionKeepMonthly"`
	// Off-site transfer bandwidth caps (KiB/s; 0 = unlimited).
	OffsiteLimitUpload   int `json:"offsiteLimitUpload"`
	OffsiteLimitDownload int `json:"offsiteLimitDownload"`
	// Opt-in Prometheus /metrics endpoint + its optional bearer scrape token.
	MetricsEnabled bool   `json:"metricsEnabled"`
	MetricsToken   string `json:"metricsToken"`
	// Scheduled restore-verification drills (restic check --read-data-subset).
	DrillsEnabled   bool   `json:"drillsEnabled"`
	DrillsSchedule  string `json:"drillsSchedule"`
	DrillsSubsetPct int    `json:"drillsSubsetPct"`
	// OffsiteDrillsEnabled gates ONLY the scheduled off-site DR drill (#37); the
	// local subset check + the manual DR button are unaffected. Default on.
	OffsiteDrillsEnabled bool `json:"offsiteDrillsEnabled"`
	// RecoveryKitAck dismisses the dashboard nag once the user has downloaded +
	// safely stored the encryption-key recovery kit.
	RecoveryKitAck bool `json:"recoveryKitAck"`
	// Per-domain "off-site repo is append-only (immutable)" flags: BombVault then
	// skips its own off-site prune and refuses off-site deletes.
	ContainersOffsiteImmutable bool `json:"containersOffsiteImmutable"`
	VMsOffsiteImmutable        bool `json:"vmsOffsiteImmutable"`
	FlashOffsiteImmutable      bool `json:"flashOffsiteImmutable"`
	ConfigOffsiteImmutable     bool `json:"configOffsiteImmutable"`
	FilesOffsiteImmutable      bool `json:"filesOffsiteImmutable"`
	// Off-site growth budget in GB (0 = alarm off) + tamper-test cadence +
	// DR-drill target container ('' = auto).
	OffsiteGrowthBudgetGB int    `json:"offsiteGrowthBudgetGB"`
	TamperTestSchedule    string `json:"tamperTestSchedule"`
	DRDrillTarget         string `json:"drDrillTarget"`
	PruneImageAfterUpdate bool   `json:"pruneImageAfterUpdate"`
}

func toView(s store.Settings) settingsView {
	return settingsView{
		EncryptionEnabled:           s.EncryptionEnabled,
		ContainersEnabled:           s.ContainersEnabled,
		VMsEnabled:                  s.VMsEnabled,
		FlashEnabled:                s.FlashEnabled,
		ConfigEnabled:               s.ConfigEnabled,
		FilesEnabled:                s.FilesEnabled,
		ContainersPath:              s.ContainersPath,
		VMsPath:                     s.VMsPath,
		FlashPath:                   s.FlashPath,
		ConfigPath:                  s.ConfigPath,
		FilesPath:                   s.FilesPath,
		RestoreFolder:               s.RestoreFolder,
		ContainersOffsite:           s.ContainersOffsite,
		VMsOffsite:                  s.VMsOffsite,
		FlashOffsite:                s.FlashOffsite,
		ConfigOffsite:               s.ConfigOffsite,
		FilesOffsite:                s.FilesOffsite,
		ContainersOffsiteSchedule:   s.ContainersOffsiteSchedule,
		VMsOffsiteSchedule:          s.VMsOffsiteSchedule,
		FlashOffsiteSchedule:        s.FlashOffsiteSchedule,
		ConfigOffsiteSchedule:       s.ConfigOffsiteSchedule,
		FilesOffsiteSchedule:        s.FilesOffsiteSchedule,
		ContainersSchedule:          s.ContainersSchedule,
		VMsSchedule:                 s.VMsSchedule,
		FlashSchedule:               s.FlashSchedule,
		ConfigSchedule:              s.ConfigSchedule,
		FilesSchedule:               s.FilesSchedule,
		FlashZipExportEnabled:       s.FlashZipExportEnabled,
		FlashZipExportPath:          s.FlashZipExportPath,
		FlashZipExportKeep:          s.FlashZipExportKeep,
		DefaultLanguage:             s.DefaultLanguage,
		RetentionKeepLast:           s.RetentionKeepLast,
		RetentionKeepDaily:          s.RetentionKeepDaily,
		RetentionKeepWeekly:         s.RetentionKeepWeekly,
		RetentionKeepMonthly:        s.RetentionKeepMonthly,
		OffsiteRetentionKeepLast:    s.OffsiteRetentionKeepLast,
		OffsiteRetentionKeepDaily:   s.OffsiteRetentionKeepDaily,
		OffsiteRetentionKeepWeekly:  s.OffsiteRetentionKeepWeekly,
		OffsiteRetentionKeepMonthly: s.OffsiteRetentionKeepMonthly,
		OffsiteLimitUpload:          s.OffsiteLimitUpload,
		OffsiteLimitDownload:        s.OffsiteLimitDownload,
		MetricsEnabled:              s.MetricsEnabled,
		MetricsToken:                s.MetricsToken,
		DrillsEnabled:               s.DrillsEnabled,
		DrillsSchedule:              s.DrillsSchedule,
		DrillsSubsetPct:             s.DrillsSubsetPct,
		OffsiteDrillsEnabled:        s.OffsiteDrillsEnabled,
		RecoveryKitAck:              s.RecoveryKitAck,
		ContainersOffsiteImmutable:  s.ContainersOffsiteImmutable,
		VMsOffsiteImmutable:         s.VMsOffsiteImmutable,
		FlashOffsiteImmutable:       s.FlashOffsiteImmutable,
		ConfigOffsiteImmutable:      s.ConfigOffsiteImmutable,
		FilesOffsiteImmutable:       s.FilesOffsiteImmutable,
		OffsiteGrowthBudgetGB:       s.OffsiteGrowthBudgetGB,
		TamperTestSchedule:          s.TamperTestSchedule,
		DRDrillTarget:               s.DRDrillTarget,
		PruneImageAfterUpdate:       s.PruneImageAfterUpdate,
	}
}

func (h *Handler) handleGetSettings(w http.ResponseWriter, _ *http.Request) {
	s, err := h.store.GetSettings()
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	// Nest under "settings" so the GET response is shape-symmetric with the PUT
	// body: a client can GET, edit, and PUT back the same settings object without
	// the envelope's "ok" leaking into the strict PUT decoder.
	// hostMountRoot is a sibling (not inside settings) so the strict PUT decoder
	// never sees it and cannot reject it as an unknown field.
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":            true,
		"settings":      toView(s),
		"hostMountRoot": h.cfg.HostMountRoot,
	})
}

func (h *Handler) handlePutSettings(w http.ResponseWriter, r *http.Request) {
	var v settingsView
	if !decodeBody(w, r, &v) {
		return
	}

	// RestoreFolder is ALWAYS a local filesystem path (restores land on the local
	// mount root). A remote-looking value (e.g. "s3:foo") would slip past the
	// containment check below, which skips remotes with `continue`, so reject it
	// up front — it can never legitimately be a remote backend.
	if v.RestoreFolder != "" && restic.IsRemoteRepo(v.RestoreFolder) {
		writeJSON(w, http.StatusOK, map[string]any{
			"ok": false, "error": "restore folder must be a local path under the mount root",
		})
		return
	}

	// Validate each domain repo location: a remote backend (rclone:…/s3:…) is
	// accepted verbatim; a local path must stay under the mount root.
	// Local domain repos, plus any configured off-site repos (off-site may be
	// blank = none). A remote backend (rclone:/s3:/rest:…) is accepted verbatim;
	// a local path must stay under the mount root.
	for _, sub := range []string{
		v.ContainersPath, v.VMsPath, v.FlashPath, v.ConfigPath, v.FilesPath, v.RestoreFolder,
		v.ContainersOffsite, v.VMsOffsite, v.FlashOffsite, v.ConfigOffsite, v.FilesOffsite,
	} {
		if sub == "" || restic.IsRemoteRepo(sub) {
			continue
		}
		// A "word:" prefix that isn't a recognized remote is almost always a
		// mistyped off-site path (e.g. "BackBlaze:bucket" instead of
		// "rclone:BackBlaze:bucket"); reject it with guidance rather than
		// silently treating it as a local folder named after the string.
		if restic.LooksLikeUnprefixedRemote(sub) {
			writeJSON(w, http.StatusOK, map[string]any{
				"ok": false, "error": fmt.Sprintf("%q looks like a remote backend but is missing its prefix — off-site backends need one of rclone:/s3:/rest:/sftp:/b2:, for example rclone:%s", sub, sub),
			})
			return
		}
		if _, err := paths.Resolve(h.cfg.HostMountRoot, sub); err != nil {
			log.Printf("api: settings: rejected path %q: %v", sub, err)
			writeJSON(w, http.StatusOK, map[string]any{
				"ok": false, "error": "invalid backup path: must be a relative subpath under the mount root, or an rclone:/s3: remote",
			})
			return
		}
	}

	// Validate each cadence parses (backup schedules + off-site + drills +
	// tamper-test schedules).
	for _, cad := range []string{
		v.ContainersSchedule, v.VMsSchedule, v.FlashSchedule, v.ConfigSchedule, v.FilesSchedule,
		v.ContainersOffsiteSchedule, v.VMsOffsiteSchedule, v.FlashOffsiteSchedule, v.ConfigOffsiteSchedule, v.FilesOffsiteSchedule,
		v.DrillsSchedule, v.TamperTestSchedule,
	} {
		if _, err := schedule.ParseCadence(cad); err != nil {
			writeJSON(w, http.StatusOK, map[string]any{
				"ok": false, "error": scrubError(err),
			})
			return
		}
	}
	// Off-site, drills and tamper-test schedules can't use "everyN": those jobs
	// have no per-domain last-run gate, so an everyN cadence would silently fire
	// daily. Restrict them to off / daily / weekly / cron, which all fire on an
	// exact schedule.
	for _, cad := range []string{v.ContainersOffsiteSchedule, v.VMsOffsiteSchedule, v.FlashOffsiteSchedule, v.ConfigOffsiteSchedule, v.FilesOffsiteSchedule, v.DrillsSchedule, v.TamperTestSchedule} {
		if c, _ := schedule.ParseCadence(cad); c.IntervalDays > 0 {
			writeJSON(w, http.StatusOK, map[string]any{
				"ok": false, "error": "this schedule does not support 'everyN' — use 'daily HH:MM', 'weekly DOW HH:MM', or a cron expression",
			})
			return
		}
	}

	// A DR-drill target, when set, is a container name fed by the UI dropdown.
	// Validate it with the same rule that guards name-keyed handler paths, so a
	// garbage/injection-shaped value is rejected at save time rather than stored
	// (parity with the other name validations above).
	if dt := strings.TrimSpace(v.DRDrillTarget); dt != "" && !validResourceName(dt) {
		writeJSON(w, http.StatusOK, map[string]any{
			"ok": false, "error": "invalid DR-drill target",
		})
		return
	}

	// Preserve fields that are NOT part of the settings form — they are managed
	// by their own endpoints/flows (auth password) or are encrypted secrets
	// (rclone config). Without this, every settings save would wipe them.
	existing, err := h.store.GetSettings()
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}

	// Enabling the VMs domain requires a working SSH connection to the host —
	// otherwise the tab would appear but nothing could be backed up. Verify only
	// on the OFF→ON transition so unrelated saves aren't blocked by a transient
	// host outage.
	if v.VMsEnabled && !existing.VMsEnabled {
		if tErr := h.svc.VMSSHTest(r.Context()); tErr != nil {
			writeJSON(w, http.StatusOK, map[string]any{
				"ok":    false,
				"error": "Can't enable VM backup yet: " + scrubError(tErr) + ". Set up the SSH key under “VM Backup over SSH” and click Test connection first.",
			})
			return
		}
	}

	s := store.Settings{
		EncryptionEnabled:           v.EncryptionEnabled,
		ContainersEnabled:           v.ContainersEnabled,
		VMsEnabled:                  v.VMsEnabled,
		FlashEnabled:                v.FlashEnabled,
		ConfigEnabled:               v.ConfigEnabled,
		FilesEnabled:                v.FilesEnabled,
		ContainersPath:              v.ContainersPath,
		VMsPath:                     v.VMsPath,
		FlashPath:                   v.FlashPath,
		ConfigPath:                  v.ConfigPath,
		FilesPath:                   v.FilesPath,
		RestoreFolder:               v.RestoreFolder,
		ContainersOffsite:           v.ContainersOffsite,
		VMsOffsite:                  v.VMsOffsite,
		FlashOffsite:                v.FlashOffsite,
		ConfigOffsite:               v.ConfigOffsite,
		FilesOffsite:                v.FilesOffsite,
		ContainersOffsiteSchedule:   v.ContainersOffsiteSchedule,
		VMsOffsiteSchedule:          v.VMsOffsiteSchedule,
		FlashOffsiteSchedule:        v.FlashOffsiteSchedule,
		ConfigOffsiteSchedule:       v.ConfigOffsiteSchedule,
		FilesOffsiteSchedule:        v.FilesOffsiteSchedule,
		ContainersSchedule:          v.ContainersSchedule,
		VMsSchedule:                 v.VMsSchedule,
		FlashSchedule:               v.FlashSchedule,
		ConfigSchedule:              v.ConfigSchedule,
		FilesSchedule:               v.FilesSchedule,
		FlashZipExportEnabled:       v.FlashZipExportEnabled,
		FlashZipExportPath:          v.FlashZipExportPath,
		FlashZipExportKeep:          max(0, v.FlashZipExportKeep),
		DefaultLanguage:             v.DefaultLanguage,
		RetentionKeepLast:           max(0, v.RetentionKeepLast),
		RetentionKeepDaily:          max(0, v.RetentionKeepDaily),
		RetentionKeepWeekly:         max(0, v.RetentionKeepWeekly),
		RetentionKeepMonthly:        max(0, v.RetentionKeepMonthly),
		OffsiteRetentionKeepLast:    max(0, v.OffsiteRetentionKeepLast),
		OffsiteRetentionKeepDaily:   max(0, v.OffsiteRetentionKeepDaily),
		OffsiteRetentionKeepWeekly:  max(0, v.OffsiteRetentionKeepWeekly),
		OffsiteRetentionKeepMonthly: max(0, v.OffsiteRetentionKeepMonthly),
		OffsiteLimitUpload:          max(0, v.OffsiteLimitUpload),
		OffsiteLimitDownload:        max(0, v.OffsiteLimitDownload),
		MetricsEnabled:              v.MetricsEnabled,
		MetricsToken:                strings.TrimSpace(v.MetricsToken),
		DrillsEnabled:               v.DrillsEnabled,
		DrillsSchedule:              v.DrillsSchedule,
		DrillsSubsetPct:             max(1, min(100, v.DrillsSubsetPct)),
		OffsiteDrillsEnabled:        v.OffsiteDrillsEnabled,
		RecoveryKitAck:              v.RecoveryKitAck,
		ContainersOffsiteImmutable:  v.ContainersOffsiteImmutable,
		VMsOffsiteImmutable:         v.VMsOffsiteImmutable,
		FlashOffsiteImmutable:       v.FlashOffsiteImmutable,
		ConfigOffsiteImmutable:      v.ConfigOffsiteImmutable,
		FilesOffsiteImmutable:       v.FilesOffsiteImmutable,
		OffsiteGrowthBudgetGB:       max(0, v.OffsiteGrowthBudgetGB),
		TamperTestSchedule:          v.TamperTestSchedule,
		DRDrillTarget:               strings.TrimSpace(v.DRDrillTarget),
		PruneImageAfterUpdate:       v.PruneImageAfterUpdate,
		AuthPasswordHash:            existing.AuthPasswordHash,
		RcloneConf:                  existing.RcloneConf,
		NotifyConf:                  existing.NotifyConf,
		CloudConf:                   existing.CloudConf,
	}
	if err := h.store.UpdateSettings(s); err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	if err := h.scheduler.ReloadWithDueChecks(s, h.containersLastRun, h.vmsLastRun, h.flashLastRun, h.configLastRun, h.filesLastRun); err != nil {
		// Settings persisted but the scheduler could not re-register — report it.
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": scrubError(err)})
		return
	}
	// Immutable off-site + an off-site retention policy both set: warn, don't
	// fail. BombVault never prunes an append-only repo, so the policy is inert
	// until enforced far-side. The "warnings" array is a backward-compatible
	// extension of the ok envelope (absent when there is nothing to warn about).
	var warnings []string
	if (s.ContainersOffsiteImmutable || s.VMsOffsiteImmutable || s.FlashOffsiteImmutable || s.ConfigOffsiteImmutable || s.FilesOffsiteImmutable) &&
		(s.OffsiteRetentionKeepLast > 0 || s.OffsiteRetentionKeepDaily > 0 ||
			s.OffsiteRetentionKeepWeekly > 0 || s.OffsiteRetentionKeepMonthly > 0) {
		warnings = append(warnings, "The off-site repo is append-only (immutable), so BombVault will not apply the off-site retention policy — enforce retention far-side (e.g. a rest-server prune cron) or use a maintenance window.")
	}
	if len(warnings) > 0 {
		writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"warnings": warnings}))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(nil))
}

// handleRecoveryKit streams the encryption-key recovery kit as a download.
// GET /api/recovery-kit — BEHIND authGate (NOT public allow-listed): the kit
// contains the master APP_KEY + the derived restic password, so only the
// session-authenticated owner may fetch it. The body is the owner's own recovery
// document and carries the real repo locations (no path scrubbing here), and it
// is never logged.
func (h *Handler) handleRecoveryKit(w http.ResponseWriter, _ *http.Request) {
	kit, err := h.svc.RecoveryKit()
	if err != nil {
		// A build failure (settings read) is reported as JSON before any body is
		// streamed; the secret body is never logged.
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="bombvault-recovery-kit.md"`)
	w.WriteHeader(http.StatusOK)
	if _, wErr := w.Write([]byte(kit)); wErr != nil {
		// Log only the failure, never the body (it contains the master key).
		log.Printf("api: recovery-kit: write failed: %v", wErr)
	}
}

// handleRecoveryKitAck records that the user has stored the recovery kit, which
// dismisses the dashboard nag. It reads the current settings and flips ONLY that
// flag, so acknowledging never overwrites unrelated settings changes made
// elsewhere (a full-settings round-trip from the dashboard could clobber them).
// POST /api/recovery-kit/ack
func (h *Handler) handleRecoveryKitAck(w http.ResponseWriter, _ *http.Request) {
	s, err := h.store.GetSettings()
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	if !s.RecoveryKitAck {
		s.RecoveryKitAck = true
		if err := h.store.UpdateSettings(s); err != nil {
			writeJSON(w, http.StatusOK, failEnvelope(err))
			return
		}
	}
	writeJSON(w, http.StatusOK, okEnvelope(nil))
}

// handleCheck verifies the integrity of a domain's restic repo (restic check).
// POST /api/check/{domain}  domain ∈ {containers, vms, flash, files}
func (h *Handler) handleCheck(w http.ResponseWriter, r *http.Request) {
	domain := r.PathValue("domain")
	switch domain {
	case "containers", "vms", "flash", "files":
	default:
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "unknown domain"})
		return
	}
	if err := h.svc.CheckDomain(r.Context(), domain, sourceParam(r)); err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(nil))
}

// handleRunDrill runs a restore-verification drill for a domain and returns the
// recorded result. ?kind=subset (default) is the classic `restic check
// --read-data-subset` integrity check; ?kind=dr is a real off-site sandbox restore
// (containers, flash + files only). POST /api/verify/{domain}?source=&kind=
// domain ∈ {containers,vms,flash,files}
func (h *Handler) handleRunDrill(w http.ResponseWriter, r *http.Request) {
	domain := r.PathValue("domain")
	switch domain {
	case "containers", "vms", "flash", "files":
	default:
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "unknown domain"})
		return
	}
	// Manual: fail fast with immediate busy feedback (wait=false) so the UI can tell
	// the user a backup is running rather than blocking the request.
	drill, err := h.svc.RunRestoreDrill(r.Context(), domain, sourceParam(r), kindParam(r), false)
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"drill": drill}))
}

// handleDrills returns the recorded restore-verification drills for a domain +
// source (newest first), plus the latest one for the badge.
// GET /api/verify?domain=&source=&limit=
func (h *Handler) handleDrills(w http.ResponseWriter, r *http.Request) {
	domain := r.URL.Query().Get("domain")
	switch domain {
	case "containers", "vms", "flash", "files":
	default:
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "unknown domain"})
		return
	}
	source := sourceParam(r)

	limit := 90
	if q := r.URL.Query().Get("limit"); q != "" {
		if n, err := strconv.Atoi(q); err == nil {
			limit = n
		}
	}
	if limit < 1 {
		limit = 1
	}
	if limit > 365 {
		limit = 365
	}

	drills, err := h.svc.Drills(domain, source, limit)
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	if drills == nil {
		drills = []store.RestoreDrill{}
	}
	var latest any // null when there are no drills yet
	if len(drills) > 0 {
		latest = drills[0] // newest first
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"drills": drills, "latest": latest}))
}

// handleUnlock clears repository locks for a domain (restic unlock --remove-all),
// the manual recovery for a "repository is already locked" error left by a
// crashed/interrupted run. POST /api/unlock/{domain}
func (h *Handler) handleUnlock(w http.ResponseWriter, r *http.Request) {
	domain := r.PathValue("domain")
	switch domain {
	case "containers", "vms", "flash", "files":
	default:
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "unknown domain"})
		return
	}
	if err := h.svc.UnlockDomain(r.Context(), domain, sourceParam(r)); err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(nil))
}

// handlePrune reclaims repository space freed by forgotten snapshots
// (restic prune). POST /api/prune/{domain}
func (h *Handler) handlePrune(w http.ResponseWriter, r *http.Request) {
	domain := r.PathValue("domain")
	switch domain {
	case "containers", "vms", "flash", "config", "files":
	default:
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "unknown domain"})
		return
	}
	if err := h.svc.PruneDomain(r.Context(), domain, sourceParam(r)); err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(nil))
}

// handleDeleteSnapshot forgets a single snapshot from a domain's repo.
// DELETE /api/snapshots/{domain}/{id}
func (h *Handler) handleDeleteSnapshot(w http.ResponseWriter, r *http.Request) {
	domain := r.PathValue("domain")
	switch domain {
	case "containers", "vms", "flash", "config", "files":
	default:
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "unknown domain"})
		return
	}
	if err := h.svc.DeleteSnapshot(r.Context(), domain, r.PathValue("id"), sourceParam(r)); err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(nil))
}

// handleReplicateOffsite replicates a domain's local repo to its off-site repo on
// demand (restic copy). POST /api/offsite/{domain}
func (h *Handler) handleReplicateOffsite(w http.ResponseWriter, r *http.Request) {
	domain := r.PathValue("domain")
	switch domain {
	case "containers", "vms", "flash", "files":
	default:
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "unknown domain"})
		return
	}
	if err := h.svc.ReplicateOffsite(r.Context(), domain); err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(nil))
}

// handleTestOffsite probes a domain's off-site repo (reachable / initialised)
// without modifying it, so the UI can verify the location before relying on it.
// Modelled on handleVMSSHTest. POST /api/offsite/{domain}/test
func (h *Handler) handleTestOffsite(w http.ResponseWriter, r *http.Request) {
	domain := r.PathValue("domain")
	switch domain {
	case "containers", "vms", "flash", "files":
	default:
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "unknown domain"})
		return
	}
	reachable, initialized, err := h.svc.TestOffsite(r.Context(), domain)
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{
		"reachable":   reachable,
		"initialized": initialized,
	}))
}

// handleDeploySnippet returns a one-time rest-server deployment recipe for a
// domain's append-only off-site repo (docker run + compose + generated htpasswd
// credentials). Nothing is persisted server-side — the plaintext password is
// shown once. GET /api/offsite/{domain}/deploy-snippet
func (h *Handler) handleDeploySnippet(w http.ResponseWriter, r *http.Request) {
	domain := r.PathValue("domain")
	switch domain {
	case "containers", "vms", "flash", "files":
	default:
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "unknown domain"})
		return
	}
	snip, err := buildDeploySnippet(domain)
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"snippet": snip}))
}

// handleTamperTest runs an active off-site tamper test for a domain: it probes the
// far-side rest-server's delete path with side-effect-free DELETEs to verify the
// append-only protection is actually enforced (not just configured).
// POST /api/offsite/{domain}/tamper-test
func (h *Handler) handleTamperTest(w http.ResponseWriter, r *http.Request) {
	domain := r.PathValue("domain")
	switch domain {
	case "containers", "vms", "flash", "files":
	default:
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "unknown domain"})
		return
	}
	verdict, err := h.svc.RunTamperTest(r.Context(), domain)
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{
		"testable":  verdict.Testable,
		"protected": verdict.Protected,
		"detail":    verdict.Detail,
	}))
}

// handleRcloneInfo returns the configured rclone remote names (never secrets).
// GET /api/rclone
func (h *Handler) handleRcloneInfo(w http.ResponseWriter, _ *http.Request) {
	remotes, err := h.svc.RcloneRemotes()
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	if remotes == nil {
		remotes = []string{}
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"remotes": remotes}))
}

// handleSetRclone stores the rclone config (encrypted) and writes the on-disk
// file. An empty conf clears it. POST /api/rclone  body {conf}
func (h *Handler) handleSetRclone(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Conf string `json:"conf"`
	}
	if !decodeBody(w, r, &body) {
		return
	}
	if err := h.svc.SetRcloneConf(body.Conf); err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(nil))
}

// handleGetNotify returns the notification config WITHOUT the stored credentials:
// the SMTP password and Matrix access token are blanked and reported via "is-set"
// flags, so the UI can show what's configured and edit it without shipping the
// secrets to the browser (mirrors the cloud-credentials endpoint). GET /api/notify
func (h *Handler) handleGetNotify(w http.ResponseWriter, _ *http.Request) {
	c, err := h.svc.NotifyConfig()
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	smtpPasswordSet := c.SMTPPassword != ""
	matrixTokenSet := c.MatrixToken != ""
	c.SMTPPassword = ""
	c.MatrixToken = ""
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{
		"notify":          c,
		"smtpPasswordSet": smtpPasswordSet,
		"matrixTokenSet":  matrixTokenSet,
	}))
}

// fillNotifySecrets fills blank credential fields from the stored config. Because
// handleGetNotify never ships the SMTP password / Matrix token to the browser, an
// unchanged form re-submits them blank — blank therefore means "keep the stored one".
func (h *Handler) fillNotifySecrets(c notify.Config) notify.Config {
	if c.SMTPPassword != "" && c.MatrixToken != "" {
		return c
	}
	cur, err := h.svc.NotifyConfig()
	if err != nil {
		return c
	}
	if c.SMTPPassword == "" {
		c.SMTPPassword = cur.SMTPPassword
	}
	if c.MatrixToken == "" {
		c.MatrixToken = cur.MatrixToken
	}
	return c
}

// handleSetNotify stores the notification config (encrypted). A blank SMTP password
// or Matrix token keeps the previously stored one. POST /api/notify
func (h *Handler) handleSetNotify(w http.ResponseWriter, r *http.Request) {
	var c notify.Config
	if !decodeBody(w, r, &c) {
		return
	}
	if err := h.svc.SetNotifyConfig(h.fillNotifySecrets(c)); err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(nil))
}

// handleGetCloud returns the cloud-backend credentials WITHOUT the secrets: the
// non-secret fields plus "is-set" flags so the UI can show what's configured and
// edit it without exposing the stored keys. GET /api/cloud
func (h *Handler) handleGetCloud(w http.ResponseWriter, _ *http.Request) {
	c, err := h.svc.CloudConfig()
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{
		"s3KeyId":         c.S3KeyID,
		"s3Region":        c.S3Region,
		"restUser":        c.RESTUser,
		"s3SecretSet":     c.S3Secret != "",
		"restPasswordSet": c.RESTPassword != "",
	}))
}

// handleSetCloud stores the cloud-backend credentials (encrypted). A blank secret
// field keeps the previously stored one. POST /api/cloud
func (h *Handler) handleSetCloud(w http.ResponseWriter, r *http.Request) {
	var c CloudCreds
	if !decodeBody(w, r, &c) {
		return
	}
	if err := h.svc.SetCloudCreds(c); err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(nil))
}

// handleTestNotify sends a test notification using the POSTed config (so the
// user can test the form before saving). POST /api/notify/test
func (h *Handler) handleTestNotify(w http.ResponseWriter, r *http.Request) {
	var c notify.Config
	if !decodeBody(w, r, &c) {
		return
	}
	if err := h.svc.TestNotify(r.Context(), h.fillNotifySecrets(c)); err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(nil))
}

// handleReleaseNotes serves the running version's own embedded release notes so
// the "What's new" dialog (#48) works without a runtime call to api.github.com —
// which the app's own CSP (connect-src 'self') blocks, so the dialog always
// failed (#54). Same-origin, so the CSP allows it. GET /api/release-notes?version=vX.Y.Z
// (version defaults to the running build). Returns {ok, version, body, htmlUrl};
// ok=false when there are no bundled notes so the dialog shows its GitHub link.
func (h *Handler) handleReleaseNotes(w http.ResponseWriter, r *http.Request) {
	version := r.URL.Query().Get("version")
	if version == "" {
		version = Version
	}
	tag := releasenotes.Tag(version)
	htmlURL := "https://github.com/junkerderprovinz/bombvault/releases"
	if tag != "" {
		htmlURL += "/tag/" + tag
	}
	body, ok := releasenotes.Notes(version)
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":      ok,
		"version": tag,
		"body":    body,
		"htmlUrl": htmlURL,
	})
}

// runSpikeAndCache executes the host-integration probes and stores the result
// so the dashboard can render it instantly. The probes are read-only.
func (h *Handler) runSpikeAndCache() (any, bool) {
	deps := spike.Deps{
		Docker:        h.docker,
		ContainerPath: h.svc.ContainerPath(),
		LibvirtTest:   h.svc.LibvirtReachable,
	}
	checks, allOK := spike.Run(deps, h.probes)
	h.spikeMu.Lock()
	h.spikeChecks, h.spikeAllOK, h.spikeRan = checks, allOK, true
	h.spikeMu.Unlock()
	return checks, allOK
}

// WarmSpike runs the host-integration check once at startup so the cached result
// is ready the instant the dashboard loads — no manual click required.
func (h *Handler) WarmSpike() { _, _ = h.runSpikeAndCache() }

// handleSpikeFresh (POST /api/spike) always re-runs the probes — the dashboard's
// "Host Integration Check" button — and refreshes the cache.
func (h *Handler) handleSpikeFresh(w http.ResponseWriter, _ *http.Request) {
	checks, allOK := h.runSpikeAndCache()
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":     true,
		"allOk":  allOK,
		"checks": checks,
	})
}

// handleSpikeCached (GET /api/spike) returns the cached result for an instant
// view, running the probes once if they have never run (cold start).
func (h *Handler) handleSpikeCached(w http.ResponseWriter, _ *http.Request) {
	h.spikeMu.RLock()
	ran, checks, allOK := h.spikeRan, h.spikeChecks, h.spikeAllOK
	h.spikeMu.RUnlock()
	if !ran {
		checks, allOK = h.runSpikeAndCache()
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":     true,
		"allOk":  allOK,
		"checks": checks,
	})
}

// runView enriches a stored Run with the human target name + domain so the
// dashboard's run history can show WHICH container/VM/flash each run was for —
// and, on a failure, the error — instead of an opaque snapshot id.
type runView struct {
	store.Run
	Target string `json:"target"`
	Domain string `json:"domain"` // "container" | "vm" | "flash" | "config" | "files" | ""
}

func (h *Handler) handleRuns(w http.ResponseWriter, _ *http.Request) {
	// Return a generous window so the dashboard's day-filter can show several
	// days of history, not just the latest handful.
	runs, err := h.store.ListRuns(500)
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	// Resolve target_id → (name, domain). Best-effort: an unknown id (e.g. a
	// deleted target) just leaves the name blank.
	name := map[string]string{store.FlashTargetID: "Unraid flash", store.ConfigTargetID: "App configuration"}
	domain := map[string]string{store.FlashTargetID: "flash", store.ConfigTargetID: "config"}
	if cts, lErr := h.store.ListTargets(); lErr == nil {
		for _, t := range cts {
			name[t.ID] = t.ContainerName
			domain[t.ID] = "container"
		}
	}
	if vts, lErr := h.store.ListVMTargets(); lErr == nil {
		for _, t := range vts {
			name[t.ID] = t.Name
			domain[t.ID] = "vm"
		}
	}
	if fss, lErr := h.store.ListFileSets(); lErr == nil {
		for _, fs := range fss {
			name[fs.ID] = fs.Name
			domain[fs.ID] = "files"
		}
	}
	views := make([]runView, 0, len(runs))
	for _, r := range runs {
		views = append(views, runView{Run: r, Target: name[r.TargetID], Domain: domain[r.TargetID]})
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "runs": views})
}

// handleStatus returns the per-domain RPO (protection) status for the dashboard's
// "are my backups current?" indicator. GET /api/status
func (h *Handler) handleStatus(w http.ResponseWriter, _ *http.Request) {
	domains, err := h.svc.DomainStatus()
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	if domains == nil {
		domains = []DomainStatusEntry{}
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"domains": domains}))
}

// handleHistory returns per-day backup outcomes for the dashboard's
// backup-health heatmap. GET /api/history?days=90 — days defaults to 90 and is
// clamped to 1..366.
func (h *Handler) handleHistory(w http.ResponseWriter, r *http.Request) {
	days := 90
	if q := r.URL.Query().Get("days"); q != "" {
		if n, err := strconv.Atoi(q); err == nil {
			days = n
		}
	}
	if days < 1 {
		days = 1
	}
	if days > 366 {
		days = 366
	}
	hist, err := h.svc.BackupHistory(days)
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	if hist == nil {
		hist = []HistoryDay{}
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"days": hist}))
}

// handleStats returns a domain's recorded repository-size samples for the
// size/dedup trend. GET /api/stats?domain=&source=&limit= — domain ∈ {containers,
// vms, flash}; source ∈ {local, offsite} (default local); limit defaults to 90,
// clamped to 1..365. The response carries the ascending sample list plus the
// latest sample (or null when there is none) for the headline figure.
func (h *Handler) handleStats(w http.ResponseWriter, r *http.Request) {
	domain := r.URL.Query().Get("domain")
	switch domain {
	case "containers", "vms", "flash":
	default:
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "unknown domain"})
		return
	}
	source := sourceParam(r)

	limit := 90
	if q := r.URL.Query().Get("limit"); q != "" {
		if n, err := strconv.Atoi(q); err == nil {
			limit = n
		}
	}
	if limit < 1 {
		limit = 1
	}
	if limit > 365 {
		limit = 365
	}

	stats, err := h.svc.RepoStats(domain, source, limit)
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	if stats == nil {
		stats = []store.RepoStat{}
	}
	var latest any // null when there are no samples yet
	if len(stats) > 0 {
		latest = stats[len(stats)-1]
	} else {
		// No sample yet (a repo that predates this feature, or no backup since
		// upgrading): kick off a detached, throttled collection so the Storage card
		// fills in on the next load instead of staying on "no data".
		h.svc.CollectStatsAsync(domain, source)
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"stats": stats, "latest": latest}))
}

// browseDirEntry is a single subdirectory entry in the browse response.
type browseDirEntry struct {
	Name string `json:"name"`
	Path string `json:"path"` // relative to HostMountRoot (e.g. "appdata/plex")
}

// handleBrowse serves GET /api/browse?path=<subpath>.
// It lists the immediate subdirectories of <HostMountRoot>/<subpath>,
// excluding hidden entries (dot-prefixed names), sorted alphabetically.
//
// The response is always HTTP 200; errors use {ok:false,error} so the UI can
// display a graceful message. A missing or empty `path` query parameter lists
// the mount root itself.
// ---------------------------------------------------------------------------
// Authentication
// ---------------------------------------------------------------------------

const (
	sessionCookieName = "bv_session"
	sessionTTL        = 7 * 24 * time.Hour // 7 days
)

// authEnabled reads the stored password hash and reports whether authentication
// is enabled.  On a store error it logs and treats auth as OFF (safe default for
// a trusted-LAN tool — a transient DB error should not lock everyone out).
func (h *Handler) authEnabled() (hash string, on bool) {
	s, err := h.store.GetSettings()
	if err != nil {
		log.Printf("api: authEnabled: GetSettings: %v", err)
		return "", false
	}
	return s.AuthPasswordHash, s.AuthPasswordHash != ""
}

// newSessionCookie constructs the bv_session cookie with the correct attributes.
// Secure is set to true when the server is in HTTPS mode (cfg.HTTPOnly == false)
// and false for plain HTTP — which is intentional for local/LAN HTTP-only
// deployments.
func (h *Handler) newSessionCookie(value string, maxAge int) *http.Cookie {
	return &http.Cookie{ //nolint:gosec // G124: Secure is conditionally false only in HTTP-only (cfg.HTTPOnly) mode; intentional for LAN deployments
		Name:     sessionCookieName,
		Value:    value,
		Path:     "/",
		MaxAge:   maxAge,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   !h.cfg.HTTPOnly,
	}
}

// handleAuthStatus handles GET /api/auth.
// Returns {ok, enabled, authed} so the SPA can decide whether to show the
// login screen.
func (h *Handler) handleAuthStatus(w http.ResponseWriter, r *http.Request) {
	hash, on := h.authEnabled()
	authed := false
	if on {
		if c, err := r.Cookie(sessionCookieName); err == nil {
			authed = secret.ValidSessionToken(h.cfg.AppKey, hash, c.Value)
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":      true,
		"enabled": on,
		"authed":  authed,
	})
}

// handleLogin handles POST /api/login.
// Body: {password string}
// login brute-force throttle: lock out after loginMaxFails failures within
// loginWindow, so the optional password gate can't be guessed at full speed.
const (
	loginMaxFails = 5
	loginWindow   = time.Minute
)

// loginThrottled prunes the failed-attempt window and reports whether logins are
// currently locked out.
func (h *Handler) loginThrottled() bool {
	h.loginMu.Lock()
	defer h.loginMu.Unlock()
	cutoff := time.Now().Add(-loginWindow)
	kept := h.loginFails[:0]
	for _, ts := range h.loginFails {
		if ts.After(cutoff) {
			kept = append(kept, ts)
		}
	}
	h.loginFails = kept
	return len(h.loginFails) >= loginMaxFails
}

func (h *Handler) recordLoginFail() {
	h.loginMu.Lock()
	h.loginFails = append(h.loginFails, time.Now())
	h.loginMu.Unlock()
}

func (h *Handler) recordLoginSuccess() {
	h.loginMu.Lock()
	h.loginFails = nil
	h.loginMu.Unlock()
}

func (h *Handler) handleLogin(w http.ResponseWriter, r *http.Request) {
	hash, on := h.authEnabled()
	if !on {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": "authentication is not enabled"})
		return
	}
	if h.loginThrottled() {
		writeJSON(w, http.StatusTooManyRequests, map[string]any{"ok": false, "error": "too many failed attempts — wait a minute and try again"})
		return
	}

	var body struct {
		Password string `json:"password"`
	}
	if !decodeBody(w, r, &body) {
		return
	}

	if !secret.VerifyPassword(h.cfg.AppKey, body.Password, hash) {
		h.recordLoginFail()
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": "invalid password"})
		return
	}
	h.recordLoginSuccess()

	tok := secret.NewSessionToken(h.cfg.AppKey, hash, sessionTTL)
	http.SetCookie(w, h.newSessionCookie(tok, int(sessionTTL.Seconds())))
	writeJSON(w, http.StatusOK, okEnvelope(nil))
}

// handleLogout handles POST /api/logout.
// Clears the session cookie unconditionally.
func (h *Handler) handleLogout(w http.ResponseWriter, _ *http.Request) {
	http.SetCookie(w, h.newSessionCookie("", -1))
	writeJSON(w, http.StatusOK, okEnvelope(nil))
}

// handleSetPassword handles POST /api/auth/password.
// Body: {password string} — empty string disables auth.
func (h *Handler) handleSetPassword(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Password string `json:"password"`
	}
	if !decodeBody(w, r, &body) {
		return
	}

	s, err := h.store.GetSettings()
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}

	if body.Password == "" {
		s.AuthPasswordHash = ""
	} else {
		s.AuthPasswordHash = secret.HashPassword(h.cfg.AppKey, body.Password)
	}

	if err := h.store.UpdateSettings(s); err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}

	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{
		"enabled": s.AuthPasswordHash != "",
	}))
}

// authGate is a middleware that enforces authentication when auth is enabled.
// When auth is OFF it is a no-op passthrough, preserving today's behaviour.
// The following paths are always permitted (so the SPA and health-check work):
//   - GET  /api/auth
//   - POST /api/login
//   - GET  /api/health
//   - GET  /metrics  (Prometheus can't carry the session cookie; the endpoint
//     gates itself via its own enabled flag + optional bearer token)
func (h *Handler) authGate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Read auth state directly so we can fail CLOSED on a store error: a
		// transient DB failure must never silently drop the auth gate and expose
		// the API. Public liveness/auth endpoints stay reachable so the SPA can
		// still render and recover.
		s, err := h.store.GetSettings()
		if err != nil {
			log.Printf("api: authGate: GetSettings: %v", err)
			switch r.URL.Path {
			case "/api/auth", "/api/login", "/api/health", "/metrics":
				next.ServeHTTP(w, r)
			default:
				writeJSON(w, http.StatusServiceUnavailable, map[string]any{
					"ok":    false,
					"error": "authentication unavailable",
				})
			}
			return
		}
		hash := s.AuthPasswordHash
		on := hash != ""
		if !on {
			next.ServeHTTP(w, r)
			return
		}

		// Always allow the public auth + health endpoints, plus the self-gating
		// /metrics scrape endpoint (Prometheus can't carry the session cookie).
		switch r.URL.Path {
		case "/api/auth", "/api/login", "/api/health", "/metrics":
			next.ServeHTTP(w, r)
			return
		}

		// All other /api/* routes require a valid session cookie.
		c, err := r.Cookie(sessionCookieName)
		if err != nil || !secret.ValidSessionToken(h.cfg.AppKey, hash, c.Value) {
			writeJSON(w, http.StatusUnauthorized, map[string]any{
				"ok":    false,
				"error": "authentication required",
			})
			return
		}

		next.ServeHTTP(w, r)
	})
}

// ---------------------------------------------------------------------------
// VM handlers
// ---------------------------------------------------------------------------

func (h *Handler) handleListVMs(w http.ResponseWriter, r *http.Request) {
	views, err := h.svc.ListVMs(r.Context())
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	if views == nil {
		views = []VMView{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "vms": views})
}

// handleBackupVM starts a single VM backup ON THE SERVER and returns
// immediately (see handleBackup). The SPA watches "vm:<name>" over SSE.
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

// handleRestoreVM starts a VM restore ON THE SERVER and returns immediately
// (see handleRestore). The SPA watches "vm:<name>" over SSE and reads the
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
	// Confirmation is guarded here so an unconfirmed request fails synchronously
	// with the familiar sentinel (the sync service core re-checks it).
	if !body.Confirm {
		writeJSON(w, http.StatusOK, failEnvelope(backup.ErrNotConfirmed))
		return
	}
	started, err := h.svc.StartRestoreVM(r.Context(), name, body.SnapshotID, sourceParam(r), body.LeaveStopped)
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	if !started {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": "a backup or restore is already running"})
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"started": true}))
}

// handleBackupFlash starts the Unraid USB flash backup (singleton domain) ON
// THE SERVER and returns immediately (see handleBackup). The SPA watches the
// "flash" progress key over SSE.
func (h *Handler) handleBackupFlash(w http.ResponseWriter, r *http.Request) {
	started, err := h.svc.StartBackupFlash(r.Context())
	if err != nil { // the flash domain is busy with another op → 409 with the reason
		writeJSON(w, http.StatusConflict, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	if !started {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": "a backup is already running"})
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"started": true}))
}

// handleSnapshotsFlash lists flash snapshots.
func (h *Handler) handleSnapshotsFlash(w http.ResponseWriter, r *http.Request) {
	snaps, err := h.svc.SnapshotsFlash(r.Context(), sourceParam(r))
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	if snaps == nil {
		snaps = []restic.Snapshot{}
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"snapshots": snaps}))
}

// handleBackupConfig starts the singleton config self-backup — BombVault's own
// /config (settings DB + rclone.conf + ssh keypair) — ON THE SERVER and returns
// immediately, mirroring handleBackupFlash. The SPA watches the "config" progress
// key over SSE.
func (h *Handler) handleBackupConfig(w http.ResponseWriter, r *http.Request) {
	started, err := h.svc.StartBackupConfig(r.Context())
	if err != nil { // the config domain is busy with another op → 409 with the reason
		writeJSON(w, http.StatusConflict, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	if !started {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": "a backup is already running"})
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"started": true}))
}

// handleSnapshotsConfig lists config snapshots (BombVault's own /config backups).
func (h *Handler) handleSnapshotsConfig(w http.ResponseWriter, r *http.Request) {
	snaps, err := h.svc.SnapshotsConfig(r.Context(), sourceParam(r))
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	if snaps == nil {
		snaps = []restic.Snapshot{}
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"snapshots": snaps}))
}

// handleRestoreConfig STAGES a restore of BombVault's own /config and then triggers
// a self-restart so the boot-time staging→live swap applies it (see RestoreConfig /
// selfrestore.ApplyPending — the live SQLite DB can't be swapped while this process
// holds it open). It reports whether an auto-restart was scheduled; when it wasn't
// (Docker unreachable), autoRestart:false tells the SPA to ask the user to restart
// the container manually. Restore errors are mapped exactly like the other restore
// handlers — a scrubbed fail envelope (e.g. an APP_KEY / encryption mismatch surfaces
// as a plain message, not a raw restic error).
func (h *Handler) handleRestoreConfig(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Source   string `json:"source"`
		Snapshot string `json:"snapshot"`
	}
	if !decodeBody(w, r, &body) {
		return
	}
	started, auto, err := h.svc.StartRestoreConfig(r.Context(), body.Snapshot, body.Source)
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	if !started {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": "a backup or restore is already running"})
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"staged": true, "autoRestart": auto}))
}

// headerOnFirstWrite defers the download headers (and so the 200 status) until
// the first byte is actually streamed. That way a restic failure BEFORE any
// output (bad id, repo locked, no backups) is reported as a clean JSON error
// instead of a truncated 200 zip; only a genuine mid-stream failure can leave a
// partial body.
type headerOnFirstWrite struct {
	w      http.ResponseWriter
	header func()
	wrote  bool
}

func (h *headerOnFirstWrite) Write(p []byte) (int, error) {
	if !h.wrote {
		h.wrote = true
		h.header()
	}
	return h.w.Write(p)
}

// handleDownloadFlash streams a flash snapshot to the browser as a zip download
// (restic dump). GET so it can be a plain link; non-destructive — the live /boot
// is never touched. ?snapshot=<id> selects the snapshot ("" / "latest" = newest).
func (h *Handler) handleDownloadFlash(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("snapshot")
	var resolved string
	lw := &headerOnFirstWrite{w: w, header: func() {
		w.Header().Set("Content-Type", "application/zip")
		w.Header().Set("Content-Disposition", `attachment; filename="`+FlashDownloadName(resolved)+`"`)
	}}
	err := h.svc.DownloadFlashZip(r.Context(), id, sourceParam(r), func(rid string) { resolved = rid }, lw)
	// No bytes streamed yet → headers not sent, so report the failure as JSON
	// (bad/ambiguous id, no backups, repo locked). A mid-stream failure (after
	// bytes flowed) can only truncate the body; the failed run is recorded.
	if err != nil && !lw.wrote {
		writeJSON(w, http.StatusOK, failEnvelope(err))
	}
}

func (h *Handler) handlePatchVM(w http.ResponseWriter, r *http.Request) {
	name, ok := h.vmNameParam(w, r)
	if !ok {
		return
	}
	var body struct {
		Method            *string `json:"method"`
		IncludeInSchedule *bool   `json:"includeInSchedule"`
	}
	if !decodeBody(w, r, &body) {
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
	writeJSON(w, http.StatusOK, okEnvelope(nil))
}

// handleVMScheduleIncludeAll sets the include_in_schedule flag for EVERY known VM
// in one call — the VM counterpart to handleScheduleIncludeAll.
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

func (h *Handler) handleBrowse(w http.ResponseWriter, r *http.Request) {
	subpath := r.URL.Query().Get("path")

	// Resolve the absolute path to read.
	// An empty subpath lists the mount root itself — paths.Resolve requires a
	// non-empty child (strict containment), so we use the root directly.
	var abs string
	if subpath == "" {
		abs = h.cfg.HostMountRoot
	} else {
		var err error
		abs, err = paths.Resolve(h.cfg.HostMountRoot, subpath)
		if err != nil {
			// paths.Resolve returns ErrTraversal or ErrAbsoluteSub — neither
			// leaks host paths; report a generic message for defense-in-depth.
			writeJSON(w, http.StatusOK, map[string]any{
				"ok":    false,
				"error": "invalid path: must be a relative subpath under the mount root",
			})
			return
		}
	}

	entries, err := os.ReadDir(abs)
	if err != nil {
		log.Printf("api: browse: ReadDir %q: %v", abs, err) //nolint:gosec // G706: abs is always either cfg.HostMountRoot or a Resolve-validated child path; no raw user bytes reach the log formatter
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":    false,
			"error": "could not read directory",
		})
		return
	}

	dirs := make([]browseDirEntry, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasPrefix(name, ".") {
			continue // skip hidden entries
		}
		// Build the relative path from HostMountRoot to this entry.
		var rel string
		if subpath == "" {
			rel = name
		} else {
			rel = subpath + "/" + name
		}
		dirs = append(dirs, browseDirEntry{Name: name, Path: rel})
	}

	// os.ReadDir returns entries in directory order (usually alphabetical on
	// most filesystems), but sort explicitly to guarantee it.
	sort.Slice(dirs, func(i, j int) bool { return dirs[i].Name < dirs[j].Name })

	writeJSON(w, http.StatusOK, map[string]any{
		"ok":   true,
		"root": h.cfg.HostMountRoot,
		"path": subpath,
		"dirs": dirs,
	})
}

// ---------------------------------------------------------------------------
// Files handlers (the files domain — named host folders backed up as file sets)
// ---------------------------------------------------------------------------

// fileSetIDParam extracts and validates the {id} path value. Set ids are
// store-generated 32-hex strings, so the strict container-name charset fits;
// validating at the boundary blocks traversal / option-injection ids from ever
// reaching the service layer (same discipline as nameParam).
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
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "fileSets": views})
}

// handleCreateFileSet creates a file set. POST /api/files/sets
// body {name, path, excludes, enabled} — path is required here (only
// DiscoverFileSets may store a path-less set) and, like the name, is fully
// validated before the row is written.
func (h *Handler) handleCreateFileSet(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name     string   `json:"name"`
		Path     string   `json:"path"`
		Excludes []string `json:"excludes"`
		Enabled  *bool    `json:"enabled"`
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
	if fs.Path == "" {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": "path is required"})
		return
	}
	if err := h.svc.validateFileSet(fs); err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	created, err := h.store.CreateFileSet(fs)
	if err != nil {
		// A duplicate name violates the UNIQUE constraint — report it clearly.
		if strings.Contains(err.Error(), "UNIQUE") {
			writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": "a file set with this name already exists"})
			return
		}
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"id": created.ID}))
}

// handlePatchFileSet partially updates a file set. PATCH /api/files/sets/{id}
// body {name?, path?, excludes?, enabled?} — pointers so an enabled-only PATCH
// doesn't reset the other fields; the MERGED set is re-validated so a patch
// can never sneak an invalid name/path past the create-time checks.
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
	}
	if !decodeBody(w, r, &body) {
		return
	}
	fs, err := h.store.GetFileSet(id)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": "file set not found"})
		return
	}
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
	if err := h.svc.validateFileSet(fs); err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	if err := h.store.UpdateFileSet(fs); err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": "a file set with this name already exists"})
			return
		}
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(nil))
}

// handleDeleteFileSet removes a file set (row + run history) WITHOUT touching
// any repo — its existing snapshots stay in the repo and can be resurfaced via
// DiscoverFileSets. Deleting the backups too is handleDeleteBackupsFileSet
// (the ForgetVM/DeleteBackupsVM split). DELETE /api/files/sets/{id}
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

// handleDeleteBackupsFileSet removes ALL backups of a file set (every
// fileset:<Name>-tagged snapshot, pruned) and forgets the set from the store.
// DELETE /api/files/sets/{id}/backups
func (h *Handler) handleDeleteBackupsFileSet(w http.ResponseWriter, r *http.Request) {
	id, ok := h.fileSetIDParam(w, r)
	if !ok {
		return
	}
	if err := h.svc.DeleteBackupsFileSet(r.Context(), id); err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(nil))
}

// handleBackupFileSet starts a single file-set backup ON THE SERVER and
// returns immediately (see handleBackup). The SPA watches "files:<name>" over
// SSE. POST /api/files/sets/{id}/backup
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

// handleBackupFilesAll starts a SERVER-SIDE batch backup of the selected file
// sets (see handleBackupAll — same detached-batch semantics; the SPA watches
// "batch:files" + per-set keys). POST /api/files/backup-all  body {ids: [...]}
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
	if len(body.IDs) > 1000 { // far beyond any real set count — reject abuse
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

// handleRestoreFileSet starts a file-set restore ON THE SERVER and returns
// immediately (see handleRestore). An empty targetPath restores IN PLACE over
// the set's source folder (confirm-gated, never silent); a non-empty
// targetPath extracts the snapshot into that folder under the host mount
// (non-destructive). Validation + target resolution run synchronously (the
// resolved target is returned in the ack); the restic work runs detached,
// publishing "files:<name>" progress and recording a run for the outcome.
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
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	if !started {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": "a backup or restore is already running"})
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"started": true, "target": target}))
}

// handleDiscoverFiles rebuilds the file-set list from backup storage (from the
// fileset: snapshot tags alone — the files domain mirrors no definitions), so
// sets lost with the database become restorable again. POST /api/files/discover
func (h *Handler) handleDiscoverFiles(w http.ResponseWriter, r *http.Request) {
	probe := r.URL.Query().Get("probe") == "true" // read-only readiness check, see handleDiscover (#44)
	n, err := h.svc.DiscoverFileSets(r.Context(), probe)
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"discovered": n}))
}
