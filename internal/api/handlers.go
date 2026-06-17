package api

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/backup"
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
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// containerView is the per-container row returned by GET /api/containers.
// Installed is false for "orphan" rows: containers that are no longer installed
// on the host but still have backups (so the user can restore or delete them).
type containerView struct {
	Name              string `json:"name"`
	Image             string `json:"image"`
	State             string `json:"state"`
	Status            string `json:"status"`
	IP                string `json:"ip"`
	Installed         bool   `json:"installed"`
	IncludeInSchedule bool   `json:"includeInSchedule"`
	LastBackup        *int64 `json:"lastBackup"`
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
		}
		if t, ok := byName[c.Name]; ok {
			v.IncludeInSchedule = t.IncludeInSchedule
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

func (h *Handler) handleBackup(w http.ResponseWriter, r *http.Request) {
	name, ok := h.nameParam(w, r)
	if !ok {
		return
	}
	sum, err := h.svc.Backup(r.Context(), name)
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{
		"snapshotId": sum.SnapshotID,
		"bytes":      sum.Bytes,
	}))
}

func (h *Handler) handleSnapshots(w http.ResponseWriter, r *http.Request) {
	name, ok := h.nameParam(w, r)
	if !ok {
		return
	}
	snaps, err := h.svc.Snapshots(r.Context(), name)
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
	if err := h.svc.Restore(r.Context(), name, body.SnapshotID, body.Confirm); err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(nil))
}

// handleListFiles lists the files in a container snapshot for file-level restore.
// GET /api/containers/{name}/files?snapshot=<id>
func (h *Handler) handleListFiles(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.nameParam(w, r); !ok {
		return
	}
	snapshot := r.URL.Query().Get("snapshot")
	files, err := h.svc.ListSnapshotFiles(r.Context(), snapshot)
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
	if err := h.svc.RestoreContainerFile(r.Context(), body.SnapshotID, body.Path, body.Confirm); err != nil {
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
	var body struct {
		IncludeInSchedule bool `json:"includeInSchedule"`
	}
	if !decodeBody(w, r, &body) {
		return
	}
	if err := h.svc.SetInclude(r.Context(), name, body.IncludeInSchedule); err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(nil))
}

// settingsView is the JSON shape for GET/PUT /api/settings.
type settingsView struct {
	EncryptionEnabled  bool   `json:"encryptionEnabled"`
	ContainersEnabled  bool   `json:"containersEnabled"`
	VMsEnabled         bool   `json:"vmsEnabled"`
	FlashEnabled       bool   `json:"flashEnabled"`
	ContainersPath     string `json:"containersPath"`
	VMsPath            string `json:"vmsPath"`
	FlashPath          string `json:"flashPath"`
	ContainersSchedule string `json:"containersSchedule"`
	VMsSchedule        string `json:"vmsSchedule"`
	FlashSchedule      string `json:"flashSchedule"`
	DefaultLanguage    string `json:"defaultLanguage"`
	// Retention keep-policy (0 = that dimension off; all 0 = retention off).
	RetentionKeepLast    int `json:"retentionKeepLast"`
	RetentionKeepDaily   int `json:"retentionKeepDaily"`
	RetentionKeepWeekly  int `json:"retentionKeepWeekly"`
	RetentionKeepMonthly int `json:"retentionKeepMonthly"`
}

func toView(s store.Settings) settingsView {
	return settingsView{
		EncryptionEnabled:  s.EncryptionEnabled,
		ContainersEnabled:  s.ContainersEnabled,
		VMsEnabled:         s.VMsEnabled,
		FlashEnabled:       s.FlashEnabled,
		ContainersPath:     s.ContainersPath,
		VMsPath:            s.VMsPath,
		FlashPath:          s.FlashPath,
		ContainersSchedule: s.ContainersSchedule,
		VMsSchedule:        s.VMsSchedule,
		FlashSchedule:      s.FlashSchedule,
		DefaultLanguage:    s.DefaultLanguage,
		RetentionKeepLast:    s.RetentionKeepLast,
		RetentionKeepDaily:   s.RetentionKeepDaily,
		RetentionKeepWeekly:  s.RetentionKeepWeekly,
		RetentionKeepMonthly: s.RetentionKeepMonthly,
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
	for _, sub := range []string{v.ContainersPath, v.VMsPath, v.FlashPath} {
		if restic.IsRemoteRepo(sub) {
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

	// Validate each cadence parses.
	for _, cad := range []string{v.ContainersSchedule, v.VMsSchedule, v.FlashSchedule} {
		if _, err := schedule.ParseCadence(cad); err != nil {
			writeJSON(w, http.StatusOK, map[string]any{
				"ok": false, "error": scrubError(err),
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

	s := store.Settings{
		EncryptionEnabled:  v.EncryptionEnabled,
		ContainersEnabled:  v.ContainersEnabled,
		VMsEnabled:         v.VMsEnabled,
		FlashEnabled:       v.FlashEnabled,
		ContainersPath:     v.ContainersPath,
		VMsPath:            v.VMsPath,
		FlashPath:          v.FlashPath,
		ContainersSchedule: v.ContainersSchedule,
		VMsSchedule:        v.VMsSchedule,
		FlashSchedule:      v.FlashSchedule,
		DefaultLanguage:    v.DefaultLanguage,
		RetentionKeepLast:    max(0, v.RetentionKeepLast),
		RetentionKeepDaily:   max(0, v.RetentionKeepDaily),
		RetentionKeepWeekly:  max(0, v.RetentionKeepWeekly),
		RetentionKeepMonthly: max(0, v.RetentionKeepMonthly),
		AuthPasswordHash:     existing.AuthPasswordHash,
		RcloneConf:           existing.RcloneConf,
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
	if err := h.svc.CheckDomain(r.Context(), domain); err != nil {
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

func (h *Handler) handleRuns(w http.ResponseWriter, _ *http.Request) {
	runs, err := h.store.ListRuns(100)
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	if runs == nil {
		runs = []store.Run{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "runs": runs})
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
func (h *Handler) handleLogin(w http.ResponseWriter, r *http.Request) {
	hash, on := h.authEnabled()
	if !on {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": "authentication is not enabled"})
		return
	}

	var body struct {
		Password string `json:"password"`
	}
	if !decodeBody(w, r, &body) {
		return
	}

	if !secret.VerifyPassword(h.cfg.AppKey, body.Password, hash) {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": "invalid password"})
		return
	}

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
			case "/api/auth", "/api/login", "/api/health":
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

		// Always allow the public auth + health endpoints.
		switch r.URL.Path {
		case "/api/auth", "/api/login", "/api/health":
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

func (h *Handler) handleBackupVM(w http.ResponseWriter, r *http.Request) {
	name, ok := h.nameParam(w, r)
	if !ok {
		return
	}
	sum, err := h.svc.BackupVM(r.Context(), name)
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{
		"snapshotId": sum.SnapshotID,
		"bytes":      sum.Bytes,
	}))
}

func (h *Handler) handleSnapshotsVM(w http.ResponseWriter, r *http.Request) {
	name, ok := h.nameParam(w, r)
	if !ok {
		return
	}
	snaps, err := h.svc.SnapshotsVM(r.Context(), name)
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
	if err := h.svc.RestoreVM(r.Context(), name, body.SnapshotID, body.Confirm); err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(nil))
}

// handleBackupFlash backs up the whole Unraid USB flash (singleton domain).
func (h *Handler) handleBackupFlash(w http.ResponseWriter, r *http.Request) {
	sum, err := h.svc.BackupFlash(r.Context())
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{
		"snapshotId": sum.SnapshotID,
		"bytes":      sum.Bytes,
	}))
}

// handleSnapshotsFlash lists flash snapshots.
func (h *Handler) handleSnapshotsFlash(w http.ResponseWriter, r *http.Request) {
	snaps, err := h.svc.SnapshotsFlash(r.Context())
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	if snaps == nil {
		snaps = []restic.Snapshot{}
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"snapshots": snaps}))
}

// handleRestoreFlash extracts a flash snapshot to the restore folder and returns
// its path (the live /boot is never overwritten).
func (h *Handler) handleRestoreFlash(w http.ResponseWriter, r *http.Request) {
	var body struct {
		SnapshotID string `json:"snapshotId"`
		Confirm    bool   `json:"confirm"`
	}
	if !decodeBody(w, r, &body) {
		return
	}
	target, err := h.svc.RestoreFlash(r.Context(), body.SnapshotID, body.Confirm)
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"target": target}))
}

func (h *Handler) handlePatchVM(w http.ResponseWriter, r *http.Request) {
	name, ok := h.nameParam(w, r)
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
