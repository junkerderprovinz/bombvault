package api

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
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
	PreHook           string   `json:"preHook"`
	PostHook          string   `json:"postHook"`
	StopContainers    []string `json:"stopContainers"`
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
			Self:      self != "" && c.Name == self,
		}
		if t, ok := byName[c.Name]; ok {
			v.IncludeInSchedule = t.IncludeInSchedule
			v.PreHook = t.PreHook
			v.PostHook = t.PostHook
			v.StopContainers = t.StopContainers
			if run, _ := h.store.LastSuccessfulBackup(t.ID); run != nil {
				v.LastBackup = run.FinishedAt
			}
		}
		views = append(views, v)
	}

	// Orphans: targets with backups whose container is no longer installed. The
	// image comes from the stored recreate definition (so the row is recognisable
	// even though the container is gone).
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
			}
		}
		if run, _ := h.store.LastSuccessfulBackup(t.ID); run != nil {
			v.LastBackup = run.FinishedAt
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
	n, err := h.svc.Discover(r.Context())
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"discovered": n}))
}

// handleDiscoverVMs rebuilds the VM target list from backup storage, so a VM
// deleted from the host (or lost with the database) becomes restorable again.
func (h *Handler) handleDiscoverVMs(w http.ResponseWriter, r *http.Request) {
	n, err := h.svc.DiscoverVMs(r.Context())
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
	if !h.svc.StartBackup(r.Context(), name) {
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
	if !h.svc.StartBackupAll(r.Context(), body.Names) {
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

func (h *Handler) handleRestore(w http.ResponseWriter, r *http.Request) {
	name, ok := h.nameParam(w, r)
	if !ok {
		return
	}
	var body struct {
		SnapshotID string `json:"snapshotId"`
		Confirm    bool   `json:"confirm"`
	}
	if !decodeBody(w, r, &body) {
		return
	}
	if err := h.svc.Restore(r.Context(), name, body.SnapshotID, body.Confirm, sourceParam(r)); err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(nil))
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

// handleRestoreFile restores a single file/dir from a container snapshot back to
// its original location. POST /api/containers/{name}/restore-file
func (h *Handler) handleRestoreFile(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.nameParam(w, r); !ok {
		return
	}
	var body struct {
		SnapshotID string `json:"snapshotId"`
		Path       string `json:"path"`
		Confirm    bool   `json:"confirm"`
	}
	if !decodeBody(w, r, &body) {
		return
	}
	if err := h.svc.RestoreContainerFile(r.Context(), body.SnapshotID, body.Path, body.Confirm, sourceParam(r)); err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(nil))
}

// handleRestoreContainerTo extracts a whole container snapshot into an ALTERNATE
// folder under the host mount (non-destructive — the live container is never
// touched). POST /api/containers/{name}/restore-to
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
	target, err := h.svc.RestoreContainerToPath(r.Context(), name, sourceParam(r), body.SnapshotID, body.TargetPath)
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"target": target}))
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
	ContainersPath            string `json:"containersPath"`
	VMsPath                   string `json:"vmsPath"`
	FlashPath                 string `json:"flashPath"`
	RestoreFolder             string `json:"restoreFolder"`
	ContainersOffsite         string `json:"containersOffsite"`
	VMsOffsite                string `json:"vmsOffsite"`
	FlashOffsite              string `json:"flashOffsite"`
	ContainersOffsiteSchedule string `json:"containersOffsiteSchedule"`
	VMsOffsiteSchedule        string `json:"vmsOffsiteSchedule"`
	FlashOffsiteSchedule      string `json:"flashOffsiteSchedule"`
	ContainersSchedule        string `json:"containersSchedule"`
	VMsSchedule               string `json:"vmsSchedule"`
	FlashSchedule             string `json:"flashSchedule"`
	DefaultLanguage           string `json:"defaultLanguage"`
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
	// RecoveryKitAck dismisses the dashboard nag once the user has downloaded +
	// safely stored the encryption-key recovery kit.
	RecoveryKitAck bool `json:"recoveryKitAck"`
}

func toView(s store.Settings) settingsView {
	return settingsView{
		EncryptionEnabled:           s.EncryptionEnabled,
		ContainersEnabled:           s.ContainersEnabled,
		VMsEnabled:                  s.VMsEnabled,
		FlashEnabled:                s.FlashEnabled,
		ContainersPath:              s.ContainersPath,
		VMsPath:                     s.VMsPath,
		FlashPath:                   s.FlashPath,
		RestoreFolder:               s.RestoreFolder,
		ContainersOffsite:           s.ContainersOffsite,
		VMsOffsite:                  s.VMsOffsite,
		FlashOffsite:                s.FlashOffsite,
		ContainersOffsiteSchedule:   s.ContainersOffsiteSchedule,
		VMsOffsiteSchedule:          s.VMsOffsiteSchedule,
		FlashOffsiteSchedule:        s.FlashOffsiteSchedule,
		ContainersSchedule:          s.ContainersSchedule,
		VMsSchedule:                 s.VMsSchedule,
		FlashSchedule:               s.FlashSchedule,
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
		RecoveryKitAck:              s.RecoveryKitAck,
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

	// Validate each domain repo location: a remote backend (rclone:…/s3:…) is
	// accepted verbatim; a local path must stay under the mount root.
	// Local domain repos, plus any configured off-site repos (off-site may be
	// blank = none). A remote backend (rclone:/s3:/rest:…) is accepted verbatim;
	// a local path must stay under the mount root.
	for _, sub := range []string{
		v.ContainersPath, v.VMsPath, v.FlashPath, v.RestoreFolder,
		v.ContainersOffsite, v.VMsOffsite, v.FlashOffsite,
	} {
		if sub == "" || restic.IsRemoteRepo(sub) {
			continue
		}
		if _, err := paths.Resolve(h.cfg.HostMountRoot, sub); err != nil {
			log.Printf("api: settings: rejected path %q: %v", sub, err)
			writeJSON(w, http.StatusOK, map[string]any{
				"ok": false, "error": "invalid backup path: must be a relative subpath under the mount root, or an rclone:/s3: remote",
			})
			return
		}
	}

	// Validate each cadence parses (backup schedules + off-site + drills schedules).
	for _, cad := range []string{
		v.ContainersSchedule, v.VMsSchedule, v.FlashSchedule,
		v.ContainersOffsiteSchedule, v.VMsOffsiteSchedule, v.FlashOffsiteSchedule,
		v.DrillsSchedule,
	} {
		if _, err := schedule.ParseCadence(cad); err != nil {
			writeJSON(w, http.StatusOK, map[string]any{
				"ok": false, "error": scrubError(err),
			})
			return
		}
	}
	// Off-site and drills schedules can't use "everyN": those jobs have no
	// per-domain last-run gate, so an everyN cadence would silently fire daily.
	// Restrict them to off / daily / weekly / cron, which all fire on an exact schedule.
	for _, cad := range []string{v.ContainersOffsiteSchedule, v.VMsOffsiteSchedule, v.FlashOffsiteSchedule, v.DrillsSchedule} {
		if c, _ := schedule.ParseCadence(cad); c.IntervalDays > 0 {
			writeJSON(w, http.StatusOK, map[string]any{
				"ok": false, "error": "this schedule does not support 'everyN' — use 'daily HH:MM', 'weekly DOW HH:MM', or a cron expression",
			})
			return
		}
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
		ContainersPath:              v.ContainersPath,
		VMsPath:                     v.VMsPath,
		FlashPath:                   v.FlashPath,
		RestoreFolder:               v.RestoreFolder,
		ContainersOffsite:           v.ContainersOffsite,
		VMsOffsite:                  v.VMsOffsite,
		FlashOffsite:                v.FlashOffsite,
		ContainersOffsiteSchedule:   v.ContainersOffsiteSchedule,
		VMsOffsiteSchedule:          v.VMsOffsiteSchedule,
		FlashOffsiteSchedule:        v.FlashOffsiteSchedule,
		ContainersSchedule:          v.ContainersSchedule,
		VMsSchedule:                 v.VMsSchedule,
		FlashSchedule:               v.FlashSchedule,
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
		RecoveryKitAck:              v.RecoveryKitAck,
		AuthPasswordHash:            existing.AuthPasswordHash,
		RcloneConf:                  existing.RcloneConf,
		NotifyConf:                  existing.NotifyConf,
		CloudConf:                   existing.CloudConf,
	}
	if err := h.store.UpdateSettings(s); err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	if err := h.scheduler.ReloadWithDueChecks(s, h.containersLastRun, h.vmsLastRun, h.flashLastRun); err != nil {
		// Settings persisted but the scheduler could not re-register — report it.
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": scrubError(err)})
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
// POST /api/check/{domain}  domain ∈ {containers, vms, flash}
func (h *Handler) handleCheck(w http.ResponseWriter, r *http.Request) {
	domain := r.PathValue("domain")
	switch domain {
	case "containers", "vms", "flash":
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

// handleRunDrill runs a restore-verification drill for a domain (restic check
// --read-data-subset), proving the backup is actually restorable, and returns the
// recorded result. POST /api/verify/{domain}?source=  domain ∈ {containers,vms,flash}
func (h *Handler) handleRunDrill(w http.ResponseWriter, r *http.Request) {
	domain := r.PathValue("domain")
	switch domain {
	case "containers", "vms", "flash":
	default:
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "unknown domain"})
		return
	}
	drill, err := h.svc.RunRestoreDrill(r.Context(), domain, sourceParam(r))
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
	case "containers", "vms", "flash":
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
	case "containers", "vms", "flash":
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
	case "containers", "vms", "flash":
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
	case "containers", "vms", "flash":
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
	Domain string `json:"domain"` // "container" | "vm" | "flash" | ""
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
	name := map[string]string{store.FlashTargetID: "Unraid flash"}
	domain := map[string]string{store.FlashTargetID: "flash"}
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
	if !h.svc.StartBackupVM(r.Context(), name) {
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

func (h *Handler) handleRestoreVM(w http.ResponseWriter, r *http.Request) {
	name, ok := h.vmNameParam(w, r)
	if !ok {
		return
	}
	var body struct {
		SnapshotID string `json:"snapshotId"`
		Confirm    bool   `json:"confirm"`
	}
	if !decodeBody(w, r, &body) {
		return
	}
	if err := h.svc.RestoreVM(r.Context(), name, body.SnapshotID, body.Confirm, sourceParam(r)); err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(nil))
}

// handleBackupFlash starts the Unraid USB flash backup (singleton domain) ON
// THE SERVER and returns immediately (see handleBackup). The SPA watches the
// "flash" progress key over SSE.
func (h *Handler) handleBackupFlash(w http.ResponseWriter, r *http.Request) {
	if !h.svc.StartBackupFlash(r.Context()) {
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
