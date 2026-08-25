package api

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
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

// credentialRe matches a "user:password@" URL-userinfo segment, e.g. the
// backupuser:Tr0ub4dor&3@ in "rest:https://backupuser:Tr0ub4dor&3@host:8000/repo"
// — a syntax the generated deploy recipe documents as valid for restic's
// rest:/s3: remote backends. absPathRe alone can't catch this: it stops at the
// first ":", so it never reaches past "user" into the password. The username
// class covers a fully-numeric username too (e.g. "123456:SuperSecret@host")
// — an earlier version of this regex required the FIRST character to be a
// letter, which let a numeric-only username through completely unscrubbed.
// Requiring the closing "@" and excluding "/" from the password body keeps
// this from matching ordinary "host:port" text (which has no "@") or a
// "name/tag" split by "/" from something after it (excluded from the password
// body) — only a real userinfo segment has a ":"-separated pair immediately
// followed by "@".
// See internal/restic/restic.go's identically-reasoned twin (and
// internal/virshcli/virshcli.go's, for libvirt URIs): this is one of several
// independent applications of the same defense-in-depth scrub, so an error
// that reaches this handler without having gone through restic.lastReason
// first still gets its credentials stripped here.
//
// Known cosmetic limitation, not a security issue: this also matches benign
// "word:word@word" shapes that merely look like userinfo, e.g. a Docker image
// digest reference "nginx:1.25@sha256:abc123" scrubs to
// "[redacted]@sha256:abc123", or an SFTP host reference like "user:1@host".
// A regex tight enough to exclude that class without risking a false
// NEGATIVE on a real credential (a numeric-looking password is legal; a
// scheme/"://" anchor doesn't survive scrubSecrets' ordering below, since
// absPathRe already strips it) wasn't obvious to construct safely, so this is
// accepted as a known tradeoff rather than forced.
//
// There is also a real, separate false NEGATIVE, pre-existing and independent
// of the false positive above: a password containing an unencoded "/" is only
// partially caught. credentialRe's password class excludes "/", and by the
// time it runs, scrubSecrets' path pass has already consumed the credential's
// leading "//...@" span as an ordinary path token (see scrubSecrets' doc
// comment for why paths must run first) — leaving no "@" left for
// credentialRe to anchor on, so its "[redacted]@" marker never fires at all.
// Depending on where the "/" falls inside the password, a fragment of the
// actual secret survives in the clear: e.g.
// "rest:https://user:wJalrXUtnFEMI/K7MDENG@host:8000/repo" scrubs to
// "rest:https:[path]:wJalrXUtnFEMI[path]:8000[path]" — the username and the
// back half of the password vanish as unlabeled path noise, but
// "wJalrXUtnFEMI" (the front half) is left sitting in the output in plain
// text. A password with no embedded "/" is unaffected.
var credentialRe = regexp.MustCompile(`[\w.+%-]+:[^\s/@"']+@`)

// scrubSecrets strips absolute-path-like tokens and then URL-embedded
// "user:pass@" credentials from s, in that order.
//
// This order is NOT a correctness requirement — both orders fully redact the
// password — but running credentialRe FIRST produces a worse result: once it
// replaces "user:pass@" with "[redacted]@", the leftover
// "scheme://[redacted]@host" is exactly the path-like shape absPathRe matches
// next, so absPathRe's pass eats the HOSTNAME right along with it (verified:
// "rest:https://user:pass@storage.example.com:8000/repo" scrubs to
// "rest:https:[path]:8000[path]" — storage.example.com is gone, along with
// any hope of telling which off-site target failed). Running absPathRe FIRST
// consumes the bare scheme separator "//" before credentialRe ever runs, so
// the only "word:...@" shape left for credentialRe to match stops at the
// real "@" — producing "[redacted]@storage.example.com:8000[path]" instead,
// which hides the password exactly as well while keeping the hostname an
// operator with multiple off-site targets needs to diagnose a failure.
func scrubSecrets(s string) string {
	s = absPathRe.ReplaceAllString(s, "[path]")
	return credentialRe.ReplaceAllString(s, "[redacted]@")
}

// errRestoreDestination tags a restore-DESTINATION refusal whose message is only
// actionable WITH the path in it: "this destination already holds data", "this
// destination is not on a mounted pool", "this destination has no room". The path
// is the operator's own chosen restore location on their own storage (or their own
// container's appdata layout, which the folder selector and the foreign bind
// warnings already show verbatim) — never a repo path, credential or secret — so
// these bypass the path scrubber. Without the bypass the UI rendered the literal
// placeholder, e.g. `restore destination "[path]" already contains data`, which
// tells the operator nothing about WHICH folder is in the way.
var errRestoreDestination = errors.New("restore destination refused")

// restoreDestErr carries a destination refusal's ready-to-show message and
// satisfies errors.Is(err, errRestoreDestination). Same shape as the restic
// package's metadataOnlyRestoreErr/backupUnreadableErr: the message text is
// untouched, only the classification is added.
type restoreDestErr struct{ msg string }

func (e *restoreDestErr) Error() string { return e.msg }

func (e *restoreDestErr) Is(target error) bool { return target == errRestoreDestination }

// destinationRefusal builds a restore-destination refusal (see
// errRestoreDestination) whose host path reaches the operator verbatim.
func destinationRefusal(format string, a ...any) error {
	return &restoreDestErr{msg: fmt.Sprintf(format, a...)}
}

// scrubBypassMessage reports whether err carries one of the sentinel types
// this codebase creates specifically because their Error() text is
// deliberately UNSAFE to run through scrubSecrets: the path-shaped content
// inside the message (a restore destination folder, the relative repo
// location an operator should type instead, /boot vs /host/boot, a ZFS
// dataset/pool name, a host:port conflict list) IS the actionable content the
// message exists to convey, not an internal filesystem/secret leak that
// needs hiding. When it matches, it returns err's message completely
// unscrubbed, and true.
//
// Both scrubError below and truncateRunErr (service.go — the other place
// error text is persisted, to runs.error) call this FIRST, before
// ever touching scrubSecrets, so the two can never independently drift on
// which shapes are safe to show verbatim. truncateRunErr didn't always do
// this: an earlier version scrubbed every error unconditionally, on the
// (false) theory that running the regexes over already-clean text is a
// harmless no-op. That broke exactly for these sentinels — scrubSecrets'
// path regex matches ANY slash-containing token, not just a filesystem path —
// mangling e.g. "host port 8080/tcp is already used by container ..." into
// "host port 8080[path] is already used ..." and eating a zvol rebase
// failure's ZFS dataset name the same way, even though scrubError itself had
// already solved precisely this problem for its own callers.
func scrubBypassMessage(err error) (string, bool) {
	switch {
	case errors.Is(err, backup.ErrRestoreConflict):
		// Already user-safe (IP / host-port / container names, no host paths) and
		// must bypass the path scrubber, which would mangle "8080/tcp" → "8080[path]".
		return err.Error(), true
	case errors.Is(err, errRestoreDestination):
		// The destination path IS the message (see errRestoreDestination).
		return err.Error(), true
	case errors.Is(err, errRepoPathGuidance):
		// Same deal: the rejected location AND the relative form to use instead
		// are the whole point of the message (see errRepoPathGuidance).
		return err.Error(), true
	case errors.Is(err, errUnraidPlatformMismatch):
		// Same deal again: /boot and /host/boot ARE the actionable content of
		// a platform-mismatch refusal (TestNotify's Unraid channel, the
		// dashboard-tile plugin) — see unraidPlatformMismatchError.
		return err.Error(), true
	case errors.Is(err, errZvolRebaseFailed):
		// Same deal again: the ZFS dataset/pool names ARE the message, and
		// necessarily contain "/" — see errZvolRebaseFailed.
		return err.Error(), true
	}
	return "", false
}

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
	}
	if msg, ok := scrubBypassMessage(err); ok {
		return msg
	}
	msg := err.Error()
	// Map restic's password/key mismatch to an actionable hint: the repo was
	// created with a different APP_KEY or a different encryption setting.
	if strings.Contains(msg, "wrong password or no key found") {
		return "backup repository can't be opened — the APP_KEY differs from when this repo was first created (or encryption was toggled). Use the original APP_KEY, or point Settings at a fresh, empty backup path."
	}
	msg = scrubSecrets(msg)
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
	// BackupOrder is the container's explicit manual backup position (#119): a
	// positive value runs earlier, 0 means unordered (overdue-first tiebreak).
	BackupOrder int `json:"backupOrder"`
	// ScheduleCadence is the container's optional per-item schedule override (#121);
	// "" means it follows the containers domain schedule. Only takes effect when the
	// perItemSchedules setting is on.
	ScheduleCadence string `json:"scheduleCadence"`
	// LastUpdateCheck / LastUpdateResult: when the post-backup update check last
	// completed (unix seconds, 0 = never) and its outcome ('' | 'up-to-date' |
	// 'updated' | 'failed') — so "checked, up to date" is visible without a
	// per-night run row.
	LastUpdateCheck  int64  `json:"lastUpdateCheck"`
	LastUpdateResult string `json:"lastUpdateResult"`
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
			v.LastUpdateCheck = t.LastUpdateCheck
			v.LastUpdateResult = t.LastUpdateResult
			v.BackupOrder = t.BackupOrder
			v.ScheduleCadence = t.ScheduleCadence
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
			ScheduleCadence:   t.ScheduleCadence,
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

// runIDRe matches an opaque run id: exactly 32 lowercase hex chars (newID is 16
// random bytes hex-encoded). The acknowledge route validates its ids against
// this — NOT validResourceName, whose Docker/VM name shape is a different thing.
var runIDRe = regexp.MustCompile(`^[0-9a-f]{32}$`)

func validRunID(id string) bool {
	return runIDRe.MatchString(id)
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
// The frontend still sends only bare "offsite"; the "offsite:<id>" form is
// accepted here (dormant) so a specific off-site target can be addressed later
// without a second parsing seam.
func sourceParam(r *http.Request) string {
	return normalizeSource(r.URL.Query().Get("source"))
}

// normalizeSource maps a raw ?source= value onto a repo source the service
// understands:
//   - "offsite" → the domain's primary off-site replica (unchanged).
//   - "offsite:<id>" → that specific off-site target, when <id> is plausibly
//     formed; a well-formed-but-unknown id later resolves to the primary in
//     offsiteTargetForSource, so a stale id never breaks a restore.
//   - a malformed "offsite:<junk>" collapses to bare "offsite" (safe primary),
//     never carrying the garbage token onward.
//   - anything else (incl. absent) → the local repo.
func normalizeSource(raw string) string {
	if id, ok := strings.CutPrefix(raw, "offsite:"); ok {
		if validOffsiteTargetID(id) {
			return "offsite:" + id
		}
		return "offsite"
	}
	if raw == "offsite" {
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
		ScheduleCadence   *string   `json:"scheduleCadence"`
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
	if body.ScheduleCadence != nil {
		if err := h.svc.SetScheduleCadence(r.Context(), name, *body.ScheduleCadence); err != nil {
			writeJSON(w, http.StatusOK, failEnvelope(err))
			return
		}
		// A per-item override registers/removes a dedicated cron entry (#121), which
		// only takes effect on a scheduler reload — settings changes reload, but a
		// container PATCH does not, so reload here.
		if err := h.reloadScheduler(); err != nil {
			writeJSON(w, http.StatusOK, failEnvelope(err))
			return
		}
	}
	writeJSON(w, http.StatusOK, okEnvelope(nil))
}

// reloadScheduler re-reads the settings and re-registers every schedule entry,
// including the per-item override entries (#121). Called after a change that alters
// the schedule structure outside the settings form (a per-item cadence PATCH).
func (h *Handler) reloadScheduler() error {
	s, err := h.store.GetSettings()
	if err != nil {
		return err
	}
	return h.scheduler.ReloadWithDueChecks(s, h.containersLastRun, h.vmsLastRun, h.flashLastRun, h.configLastRun, h.filesLastRun)
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

// handleGetBackupOrder returns the current manual backup ordering (#119): the
// containers with an explicit order, sorted by order ascending.
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

// handleSetBackupOrder replaces the manual backup ordering (#119) from an ordered
// list of container names: the first name runs earliest. Any container omitted
// from the list is returned to the most-overdue-first tiebreak. The ordering is
// authoritative, so an empty list clears all explicit orders.
// PUT /api/containers/backup-order  body {order: ["nameA", "nameB", ...]}
func (h *Handler) handleSetBackupOrder(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Order []string `json:"order"`
	}
	if !decodeBody(w, r, &body) { // caps the body at 1 MiB
		return
	}
	if len(body.Order) > 1000 { // far beyond any real container count — reject abuse
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

// handleGetVmBackupOrder returns the explicit VM backup ordering (#119, VMs).
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

// handleSetVmBackupOrder replaces the VM backup ordering (#119, VMs) from an
// ordered list of VM names: the first name runs earliest in a scheduled VM run.
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
	if len(body.Order) > 1000 { // far beyond any real VM count — reject abuse
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": "too many vms"})
		return
	}
	for _, n := range body.Order {
		if !validVMName(n) { // VM names may contain spaces ("Windows 11"); validResourceName wrongly rejected them (#127)
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
		custom = []CustomPath{}
	}
	// hostMountRoot/hostSourceRoot let the folder picker translate a browsed path
	// (relative to the host mount) back to the host path SetBackupPaths expects.
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{
		"mounts":         mounts,
		"custom":         custom,
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
	// The token is a secret and follows the house blank-and-report-is-set
	// contract (see handleGetNotify/handleGetCloud): GET always returns
	// MetricsToken blank + MetricsTokenSet reporting whether one is stored; on
	// PUT a blank MetricsToken means "keep the stored one". MetricsTokenSet is
	// on the struct (not a sibling) because the strict PUT decoder
	// (DisallowUnknownFields) must accept a round-tripped GET body.
	MetricsEnabled  bool   `json:"metricsEnabled"`
	MetricsToken    string `json:"metricsToken"`
	MetricsTokenSet bool   `json:"metricsTokenSet"`
	// Embeddable dashboard-widget token (GET /widget + GET /api/widget/data).
	// Same secret contract as MetricsToken: GET always returns WidgetToken blank
	// with WidgetTokenSet reporting presence; on PUT a blank WidgetToken keeps
	// the stored one. Generated/cleared via POST/DELETE /api/widget/token (the
	// Settings card), but the field participates in the PUT round-trip so a
	// full settings save can never silently wipe it.
	WidgetToken    string `json:"widgetToken"`
	WidgetTokenSet bool   `json:"widgetTokenSet"`
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
	// DR-drill target container/VM ('' = auto).
	OffsiteGrowthBudgetGB int    `json:"offsiteGrowthBudgetGB"`
	TamperTestSchedule    string `json:"tamperTestSchedule"`
	DRDrillTarget         string `json:"drDrillTarget"`
	DRDrillTargetVM       string `json:"drDrillTargetVm"`
	PruneImageAfterUpdate bool   `json:"pruneImageAfterUpdate"`
	// Size cap (MB) for restic's persistent cache under /config; LRU per-repo
	// eviction after scheduled runs. 0 = no limit (default 4096).
	ResticCacheMaxMB int `json:"resticCacheMaxMB"`
	// Weekly digest notification: one summary message per cadence fire through
	// the existing notify fan-out. Off by default.
	DigestEnabled  bool   `json:"digestEnabled"`
	DigestSchedule string `json:"digestSchedule"`
	// CatchUpMissed runs a scheduled backup the server slept through (it was off
	// across the scheduled fire) shortly after the next app start, anacron-style.
	// Default on.
	CatchUpMissed bool `json:"catchUpMissed"`
	// WatchdogEnabled turns on the daily overdue-backup watchdog: one push
	// notification per overdue episode through the notify channels. Default on.
	WatchdogEnabled bool `json:"watchdogEnabled"`
	// Optional age public-key encryption for the plain export paths (tool-free
	// tar.gz / xml / zip exports). Recipients are PUBLIC keys (age1... or SSH), so
	// they are not secret and round-trip in the clear. With encryption on and no
	// valid recipient every export fails loudly rather than writing plaintext.
	ExportEncryptEnabled bool   `json:"exportEncryptEnabled"`
	ExportAgeRecipients  string `json:"exportAgeRecipients"`
	// ReceiverEnabled gates the read-only receiver dashboard (a box that RECEIVES
	// immutable off-site copies and monitors the received repo). Default false
	// (opt-in), like the Files/VMs domain tabs; the sidebar gates its tab on it.
	ReceiverEnabled bool `json:"receiverEnabled"`
	// RestartHealthWait toggles the health-gated ordered restart of the "stop other
	// containers during backup" set (#119): when on, the restart waits for each
	// stopped dependency to become healthy (or Running plus a short grace when it
	// has no healthcheck) before starting the containers that depend on it. The
	// depends_on ordering is always applied; only this wait is toggled. Default on.
	// RestartHealthTimeoutSec caps that per-container wait (default 120).
	RestartHealthWait       bool `json:"restartHealthWait"`
	RestartHealthTimeoutSec int  `json:"restartHealthTimeoutSec"`
	// ReconcileUnraidUpdateStatus asks Unraid to refresh its OWN cached container
	// "update available" status after BombVault recreates a container in the
	// post-backup update step (#116), so the Docker tab's stale banner clears. The
	// recheck runs over the existing host SSH link; best-effort and non-fatal.
	// Default on.
	ReconcileUnraidUpdateStatus bool `json:"reconcileUnraidUpdateStatus"`
	// PerItemSchedules opts into per-container/VM schedule overrides (#121). Default
	// false: the per-domain schedule stays authoritative for every item and the UI
	// is unchanged. When on, an included item with a non-empty scheduleCadence runs
	// on its own cadence; an item with an empty override follows its domain schedule.
	PerItemSchedules bool `json:"perItemSchedules"`
	// Private container-registry credentials for the post-backup update pull
	// (#106). Per-entry the token follows the house blank-and-report-is-set
	// contract (see MetricsToken): GET returns every token blank with TokenSet
	// reporting presence; on PUT a blank token keeps the stored one for that
	// host, and a host missing from the list is deleted. nil (field absent, an
	// old client) keeps the stored list unchanged.
	RegistryAuths []registryAuthView `json:"registryAuths"`
	// FleetEnabled gates the read-only Fleet view (a list of peer BombVault
	// instances this box polls for their protection status). Default false
	// (opt-in), like ReceiverEnabled; the sidebar gates its tab on it.
	FleetEnabled bool `json:"fleetEnabled"`
	// InstanceName is this instance's own display name, reported to polling
	// fleet peers so a peer's Fleet page can label this box. Not a secret.
	InstanceName string `json:"instanceName"`
	// Peer status token (GET /api/fleet/status), authorizing OTHER instances'
	// Fleet views to poll THIS instance. Same secret contract as WidgetToken:
	// GET always returns FleetToken blank with FleetTokenSet reporting presence;
	// on PUT a blank FleetToken keeps the stored one. Generated/cleared via
	// POST/DELETE /api/fleet/token (the Settings card).
	FleetToken    string `json:"fleetToken"`
	FleetTokenSet bool   `json:"fleetTokenSet"`
}

// registryAuthView is one container-registry credential in the settings view
// (#106). Token is write-only; TokenSet lives on the struct (not a sibling)
// because the strict PUT decoder must accept a round-tripped GET body.
type registryAuthView struct {
	Host     string `json:"host"`
	Username string `json:"username"`
	Token    string `json:"token"`
	TokenSet bool   `json:"tokenSet"`
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
		MetricsToken:                "", // secret — never echoed; MetricsTokenSet reports presence
		MetricsTokenSet:             s.MetricsToken != "",
		WidgetToken:                 "", // secret — never echoed; WidgetTokenSet reports presence
		WidgetTokenSet:              s.WidgetToken != "",
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
		DRDrillTargetVM:             s.DRDrillTargetVM,
		PruneImageAfterUpdate:       s.PruneImageAfterUpdate,
		ResticCacheMaxMB:            s.ResticCacheMaxMB,
		DigestEnabled:               s.DigestEnabled,
		DigestSchedule:              s.DigestSchedule,
		CatchUpMissed:               s.CatchUpMissed,
		WatchdogEnabled:             s.WatchdogEnabled,
		ReconcileUnraidUpdateStatus: s.ReconcileUnraidUpdateStatus,
		ExportEncryptEnabled:        s.ExportEncryptEnabled,
		ExportAgeRecipients:         s.ExportAgeRecipients,
		ReceiverEnabled:             s.ReceiverEnabled,
		RestartHealthWait:           s.RestartHealthWait,
		RestartHealthTimeoutSec:     s.RestartHealthTimeoutSec,
		PerItemSchedules:            s.PerItemSchedules,
		FleetEnabled:                s.FleetEnabled,
		InstanceName:                s.InstanceName,
		FleetToken:                  "", // secret — never echoed; FleetTokenSet reports presence
		FleetTokenSet:               s.FleetToken != "",
	}
}

// clampHealthTimeoutSec keeps the per-container restart health-wait timeout in a
// sane range: a non-positive value falls back to the 120s default (so a client
// that omits or zeroes it never persists a nonsense 0), and it is capped at one
// hour so a typo cannot make a single stuck dependency block a restart for days.
func clampHealthTimeoutSec(sec int) int {
	if sec <= 0 {
		return 120
	}
	return min(3600, max(5, sec))
}

func (h *Handler) handleGetSettings(w http.ResponseWriter, _ *http.Request) {
	s, err := h.store.GetSettings()
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	view := toView(s)
	// Registry credentials (#106) live encrypted in the settings row, so toView
	// (a pure store.Settings mapping) can't decode them — fill the view here.
	// Tokens are secrets and never echoed; TokenSet reports presence.
	regs, err := h.svc.decodeRegistryAuths(s)
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	view.RegistryAuths = make([]registryAuthView, 0, len(regs))
	for _, a := range regs {
		view.RegistryAuths = append(view.RegistryAuths, registryAuthView{
			Host: a.Host, Username: a.Username, TokenSet: a.Token != "",
		})
	}
	// Nest under "settings" so the GET response is shape-symmetric with the PUT
	// body: a client can GET, edit, and PUT back the same settings object without
	// the envelope's "ok" leaking into the strict PUT decoder.
	// hostMountRoot/platform are siblings (not inside settings) so the strict PUT
	// decoder never sees them and cannot reject them as unknown fields.
	// platform is the detected/overridden platform.Kind ("unraid"/"generic"/
	// "truenas", see internal/platform) — read-only host-environment info, not a
	// setting the UI can change.
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":            true,
		"settings":      view,
		"hostMountRoot": h.cfg.HostMountRoot,
		"platform":      string(h.svc.platformFn().Kind()),
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
		v.DrillsSchedule, v.TamperTestSchedule, v.DigestSchedule,
	} {
		if _, err := schedule.ParseCadence(cad); err != nil {
			writeJSON(w, http.StatusOK, map[string]any{
				"ok": false, "error": scrubError(err),
			})
			return
		}
	}
	// The five OFF-SITE replication schedules still can't use "everyN". An everyN
	// cadence is a daily cron trigger plus a "has the interval elapsed?" gate, and
	// a replication job has no last-run fact to answer that with, so its interval
	// could not be enforced. Restrict them to off / daily / weekly / cron, which
	// all fire on an exact schedule. The scheduler now refuses to REGISTER an
	// unenforceable everyN as well, so a legacy or imported value cannot fire
	// daily either — this is the friendly save-time half of that same rule.
	//
	// The drills, tamper-test and digest schedules used to be in this list and no
	// longer are (#166): each records when its scheduled pass last ran
	// (store.RecordScheduleJobRun) and hands that back to the due-gate through
	// SetJobRunStore, so their interval is genuinely enforced now.
	for _, cad := range []string{v.ContainersOffsiteSchedule, v.VMsOffsiteSchedule, v.FlashOffsiteSchedule, v.ConfigOffsiteSchedule, v.FilesOffsiteSchedule} {
		if c, _ := schedule.ParseCadence(cad); c.IntervalDays > 0 {
			writeJSON(w, http.StatusOK, map[string]any{
				"ok": false, "error": "this schedule does not support 'everyN' — use 'daily HH:MM', 'weekly DOW HH:MM', or a cron expression",
			})
			return
		}
	}

	// A DR-drill target, when set, is a container/VM name fed by the UI dropdown.
	// Validate it with the same rule that guards name-keyed handler paths, so a
	// garbage/injection-shaped value is rejected at save time rather than stored
	// (parity with the other name validations above).
	if dt := strings.TrimSpace(v.DRDrillTarget); dt != "" && !validResourceName(dt) {
		writeJSON(w, http.StatusOK, map[string]any{
			"ok": false, "error": "invalid DR-drill target",
		})
		return
	}
	if dt := strings.TrimSpace(v.DRDrillTargetVM); dt != "" && !validVMName(dt) { // VM names may contain spaces ("Windows 11"); validResourceName wrongly rejected them (#127)
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

	// The metrics token is a secret the GET never echoes (toView blanks it), so
	// an unchanged form re-submits it blank — blank therefore means "keep the
	// stored token" (same contract as the notify/cloud secrets).
	metricsToken := strings.TrimSpace(v.MetricsToken)
	if metricsToken == "" {
		metricsToken = existing.MetricsToken
	}
	// Same contract for the widget token (normally managed by POST/DELETE
	// /api/widget/token; the round-trip here only has to never wipe it).
	widgetToken := strings.TrimSpace(v.WidgetToken)
	if widgetToken == "" {
		widgetToken = existing.WidgetToken
	}
	// Same contract for the fleet token (normally managed by POST/DELETE
	// /api/fleet/token; the round-trip here only has to never wipe it).
	fleetToken := strings.TrimSpace(v.FleetToken)
	if fleetToken == "" {
		fleetToken = existing.FleetToken
	}

	// Registry credentials (#106): nil = field absent (an old client) → keep the
	// stored encrypted blob unchanged; a present list REPLACES the stored one,
	// with blank tokens filled from storage per host (mergeRegistryAuths).
	registryAuths := existing.RegistryAuths
	if v.RegistryAuths != nil {
		stored, dErr := h.svc.decodeRegistryAuths(existing)
		if dErr != nil {
			writeJSON(w, http.StatusOK, failEnvelope(dErr))
			return
		}
		merged, mErr := mergeRegistryAuths(v.RegistryAuths, stored)
		if mErr != nil {
			writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": mErr.Error()})
			return
		}
		blob, eErr := h.svc.EncodeRegistryAuths(merged)
		if eErr != nil {
			writeJSON(w, http.StatusOK, failEnvelope(eErr))
			return
		}
		registryAuths = blob
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
		MetricsToken:                metricsToken,
		WidgetToken:                 widgetToken,
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
		DRDrillTargetVM:             strings.TrimSpace(v.DRDrillTargetVM),
		PruneImageAfterUpdate:       v.PruneImageAfterUpdate,
		ResticCacheMaxMB:            max(0, v.ResticCacheMaxMB),
		DigestEnabled:               v.DigestEnabled,
		DigestSchedule:              v.DigestSchedule,
		CatchUpMissed:               v.CatchUpMissed,
		WatchdogEnabled:             v.WatchdogEnabled,
		ReconcileUnraidUpdateStatus: v.ReconcileUnraidUpdateStatus,
		ExportEncryptEnabled:        v.ExportEncryptEnabled,
		ExportAgeRecipients:         strings.TrimSpace(v.ExportAgeRecipients),
		ReceiverEnabled:             v.ReceiverEnabled,
		RestartHealthWait:           v.RestartHealthWait,
		RestartHealthTimeoutSec:     clampHealthTimeoutSec(v.RestartHealthTimeoutSec),
		PerItemSchedules:            v.PerItemSchedules,
		FleetEnabled:                v.FleetEnabled,
		InstanceName:                strings.TrimSpace(v.InstanceName),
		FleetToken:                  fleetToken,
		AuthPasswordHash:            existing.AuthPasswordHash,
		SessionEpoch:                existing.SessionEpoch,
		RcloneConf:                  existing.RcloneConf,
		NotifyConf:                  existing.NotifyConf,
		CloudConf:                   existing.CloudConf,
		RegistryAuths:               registryAuths,
	}
	if err := h.store.UpdateSettings(s); err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	// Dual-write: mirror the just-saved off-site config into each domain's PRIMARY
	// offsite_targets row so the replication path (which now reads those rows) sees
	// the change. Settings stays authoritative for the fallback/rollback path.
	h.svc.syncAllPrimaryOffsiteTargets(s)
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

// handleDetectEncryption probes the configured repositories and reports which
// encryption mode they are actually in, applying a DEFINITE result to
// Settings.EncryptionEnabled so a user restoring on a fresh instance never has
// to assert it. POST /api/encryption/detect
//
// The verdict is the honest part of the contract and the UI branches on it:
// "encrypted"/"plain" are detected and applied; "conflict", "absent",
// "unknown" and "unconfigured" all leave the setting untouched and are shown as
// undecided. A probe failure is NEVER reported as "plain".
//
// Per-repo Err values are already scrubbed by the service (scrubError), so no
// repo path or backend credential reaches the client here.
func (h *Handler) handleDetectEncryption(w http.ResponseWriter, r *http.Request) {
	det, err := h.svc.DetectEncryption(r.Context())
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{
		"verdict":           string(det.Verdict),
		"applied":           det.Applied,
		"encryptionEnabled": det.EncryptionEnabled,
		"repos":             det.Repos,
	}))
}

// handleRecoveryKit streams the encryption-key recovery kit as a download.
// GET /api/recovery-kit — BEHIND authGate AND additionally requires auth to be
// ENABLED: the kit is the master secret (the APP_KEY + the derived restic
// password + the stored off-site backend credentials). The rest of the
// trusted-LAN API is intentionally open to CURRENT data when auth is off, but
// this export permanently decrypts EVERY repo — including the append-only
// off-site archives designed to survive host compromise — so it fails CLOSED
// when auth is disabled instead of handing the master key to any LAN client.
// The body is the owner's own recovery document and carries the real repo
// locations (no path scrubbing here), and it is never logged.
func (h *Handler) handleRecoveryKit(w http.ResponseWriter, _ *http.Request) {
	// Require auth to be enabled: when it is off, authGate is a pass-through and
	// this handler would otherwise hand the master key to any LAN client. A store
	// error also blocks (authEnabled reports auth OFF then) — fail closed.
	if _, _, on := h.authEnabled(); !on {
		writeJSON(w, http.StatusForbidden, map[string]any{
			"ok":    false,
			"error": "set a login password before downloading the recovery kit",
		})
		return
	}
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

// handleReplicateOffsite STARTS an on-demand replication of a domain's local
// repo to its off-site repo (restic copy) and returns immediately — the copy
// runs in the background (a long first replication used to die when the
// browser/proxy timed the synchronous request out and cancelled its context,
// #93). Config errors and a busy domain still report synchronously.
// POST /api/offsite/{domain}
func (h *Handler) handleReplicateOffsite(w http.ResponseWriter, r *http.Request) {
	domain := r.PathValue("domain")
	switch domain {
	case "containers", "vms", "flash", "files":
	default:
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "unknown domain"})
		return
	}
	if err := h.svc.StartReplicateOffsite(domain); err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"started": true}))
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
		"s3StorageClass":  c.S3StorageClass,
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
	// The S3 storage class rides the cloud creds, so a change here must re-flow
	// into each domain's primary off-site target (whose storage_class the
	// replication path reads). Best-effort; a store read failure just skips it.
	if settings, sErr := h.store.GetSettings(); sErr == nil {
		h.svc.syncAllPrimaryOffsiteTargets(settings)
	}
	writeJSON(w, http.StatusOK, okEnvelope(nil))
}

// handleGetCloudCredSets returns the additional named credential sets (#141
// stage 2) WITHOUT secrets, same is-set-flag contract as handleGetCloud.
// GET /api/cloud/creds-sets
func (h *Handler) handleGetCloudCredSets(w http.ResponseWriter, _ *http.Request) {
	sets, err := h.svc.CloudCredSets()
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	out := make([]map[string]any, len(sets))
	for i, c := range sets {
		out[i] = map[string]any{
			"id":              c.ID,
			"name":            c.Name,
			"s3KeyId":         c.S3KeyID,
			"s3Region":        c.S3Region,
			"restUser":        c.RESTUser,
			"s3StorageClass":  c.S3StorageClass,
			"s3SecretSet":     c.S3Secret != "",
			"restPasswordSet": c.RESTPassword != "",
		}
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"sets": out}))
}

// handleSetCloudCredSets replaces the whole list of additional named
// credential sets. A blank secret field on a set matched by id (against the
// previously stored set) keeps the previously stored one, same as
// handleSetCloud. POST /api/cloud/creds-sets
func (h *Handler) handleSetCloudCredSets(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Sets []CloudCredSet `json:"sets"`
	}
	if !decodeBody(w, r, &body) {
		return
	}
	if err := h.svc.SetCloudCredSets(body.Sets); err != nil {
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

// runTargetMaps resolves target_id → (human name, domain) across every domain,
// for enriching stored runs (handleRuns + the widget feed). Best-effort: an
// unknown id (e.g. a deleted target) simply stays absent, so lookups yield "".
func (h *Handler) runTargetMaps() (name, domain map[string]string) {
	name = map[string]string{store.FlashTargetID: "Unraid flash", store.ConfigTargetID: "App configuration"}
	domain = map[string]string{store.FlashTargetID: "flash", store.ConfigTargetID: "config"}
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
	return name, domain
}

func (h *Handler) handleRuns(w http.ResponseWriter, _ *http.Request) {
	// Return a generous window so the dashboard's day-filter can show several
	// days of history, not just the latest handful.
	runs, err := h.store.ListRuns(500)
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	name, domain := h.runTargetMaps()
	views := make([]runView, 0, len(runs))
	for _, r := range runs {
		views = append(views, runView{Run: r, Target: name[r.TargetID], Domain: domain[r.TargetID]})
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "runs": views})
}

// handleAckRuns marks failed runs as acknowledged so the dashboard's error panel
// can dismiss them from the failure count (#126). POST /api/runs/ack with body
// {"ids": []string (optional), "all": bool (optional)}: when `all` is set every
// unacknowledged failed run is acknowledged; otherwise the given run ids (capped
// at 5000, each a 32-hex opaque run id) are acknowledged. Responds {ok, count}.
func (h *Handler) handleAckRuns(w http.ResponseWriter, r *http.Request) {
	var body struct {
		IDs []string `json:"ids"`
		All bool     `json:"all"`
	}
	if !decodeBody(w, r, &body) {
		return
	}
	if body.All {
		n, err := h.store.AcknowledgeAllFailed()
		if err != nil {
			writeJSON(w, http.StatusOK, failEnvelope(err))
			return
		}
		writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"count": n}))
		return
	}
	if len(body.IDs) > 5000 {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "too many ids"})
		return
	}
	for _, id := range body.IDs {
		if !validRunID(id) {
			writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "invalid run id"})
			return
		}
	}
	n, err := h.store.AcknowledgeRuns(body.IDs)
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"count": n}))
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

// handleScheduleNext returns the next fire time for every currently registered
// schedule entry, soonest first — the dashboard activity log's "up next" line.
// GET /api/schedule/next. Like every other GET endpoint this wraps the payload
// in the {"ok":...} envelope (here under "runs"); the frontend's getScheduleNext
// unwraps it via the shared fetchJSON + envelope shape. A nil scheduler (should
// not happen outside tests that build a Handler without one) yields an empty
// array rather than panicking.
func (h *Handler) handleScheduleNext(w http.ResponseWriter, _ *http.Request) {
	var runs []schedule.NextRun
	if h.scheduler != nil {
		runs = h.scheduler.NextRuns()
	}
	if runs == nil {
		runs = []schedule.NextRun{}
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"runs": runs}))
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
// vms, flash, files}; source ∈ {local, offsite} (default local); limit defaults to
// 90, clamped to 1..365. The response carries the ascending sample list plus the
// latest sample (or null when there is none) for the headline figure. "files" is
// accepted because CollectStatsOnStartup / maybeCollectStats already sample the
// files repo, so the Storage card can show it (#61 Task 2).
//
// The response additionally carries "forecast" — growth rate + free space +
// time-to-full for the Storage card (see StorageForecast for the exact field
// contract) — null when nothing could be determined.
func (h *Handler) handleStats(w http.ResponseWriter, r *http.Request) {
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
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{
		"stats":    stats,
		"latest":   latest,
		"forecast": h.svc.StorageForecast(domain, source, stats),
	}))
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

// authEnabled reads the stored password hash + session epoch and reports whether
// authentication is enabled.  On a store error it logs and treats auth as OFF
// (safe default for a trusted-LAN tool — a transient DB error should not lock
// everyone out).
func (h *Handler) authEnabled() (hash, epoch string, on bool) {
	s, err := h.store.GetSettings()
	if err != nil {
		log.Printf("api: authEnabled: GetSettings: %v", err)
		return "", "", false
	}
	return s.AuthPasswordHash, s.SessionEpoch, s.AuthPasswordHash != ""
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
	hash, epoch, on := h.authEnabled()
	authed := false
	if on {
		if c, err := r.Cookie(sessionCookieName); err == nil {
			authed = secret.ValidSessionToken(h.cfg.AppKey, hash, epoch, c.Value)
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
//
// The throttle is checked BEFORE secret.VerifyPassword runs, deliberately: a
// throttled key is rejected without ever hashing the submitted password, not
// merely denied credit for a correct guess afterwards. If verification ran
// FIRST and the throttle only gated on failure, a throttled attacker could
// keep submitting guesses at full, unthrottled rate: a correct guess would
// still get verified before the throttle check ever saw it, letting the
// attacker infer correct/incorrect from response timing or content alone —
// making the throttle purely cosmetic for exactly the attacker it exists to
// stop. Checking the throttle first, unconditionally, is what actually caps
// *attempts* rather than just successes. The tradeoff is that a client which
// is currently throttled gets a 429 even on a correct password rather than
// having it be honored immediately; see loginClientKey's doc for why that
// tradeoff is now scoped to the offending client instead of every client.
const (
	loginMaxFails = 5
	loginWindow   = time.Minute
)

// loginClientKey returns the throttle key for r: the connecting peer's IP,
// with any port stripped.
//
// This deliberately reads net/http's RemoteAddr — the actual TCP peer — and
// NOT a client-supplied header such as X-Forwarded-For. This codebase has no
// trusted-proxy configuration (no allowlisted proxy CIDR, no "trust this hop"
// setting anywhere), so honoring a forwarded-for header here would let any
// caller pick their own throttle bucket at will, which defeats the point of
// keying by client in the first place. When BombVault does sit behind a
// reverse proxy (the documented remote-access setup, see
// docs/configuration.md), every request the proxy forwards shares the
// proxy's address here — clients behind it share one throttle bucket, same as
// they would for any other per-IP limiter with no trusted-proxy support. That
// is a coarser bucket than per-real-client, but it is still strictly better
// than the single global bucket this replaces, and it can't be spoofed by an
// unauthenticated caller.
func loginClientKey(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		// No port present (or an otherwise unparseable value, e.g. in tests
		// that set RemoteAddr directly) — fall back to the raw value rather
		// than collapsing every such caller onto one shared "" bucket.
		return r.RemoteAddr
	}
	return host
}

// loginSweepEvery bounds how often loginThrottled performs a full-map sweep
// (pruning every key's window, not just the one just queried), independent of
// the per-key prune below. Without this, a flood of one-off distinct keys
// that each fail exactly once — a botnet, or a single attacker rotating
// through an IPv6 /64 (effectively unlimited source addresses) — would each
// leave a permanent map entry: loginThrottled on its own only prunes the ONE
// key it was asked about, so a key that's never queried again never gets
// cleaned up. Sweeping the whole map every loginSweepEvery calls bounds that
// growth to at most loginSweepEvery stale one-off entries between sweeps,
// without paying the cost of a full sweep on every single request.
const loginSweepEvery = 256

// loginMaxTracked hard-caps the number of distinct keys loginFails holds,
// independent of the periodic sweep above. If a burst of one-off keys arrives
// faster than loginSweepEvery calls apart, the map could otherwise grow past
// that sweep's protection before it fires; this is the backstop that kicks in
// immediately once the map exceeds the cap, evicting the
// least-recently-touched entries down to the limit rather than letting an
// unauthenticated caller grow it without bound.
const loginMaxTracked = 10_000

// loginThrottled prunes key's failed-attempt window and reports whether
// logins from it are currently locked out. An empty window after pruning
// deletes key's own map entry. Separately (see loginSweepEvery/loginMaxTracked
// above), it also periodically sweeps EVERY key's window — not just key's —
// and hard-caps the map's total size, so a flood of one-off distinct keys
// that are each queried only once still can't accumulate unbounded memory.
func (h *Handler) loginThrottled(key string) bool {
	h.loginMu.Lock()
	defer h.loginMu.Unlock()
	h.sweepLoginFailsLocked()
	cutoff := time.Now().Add(-loginWindow)
	kept := pruneLoginFails(h.loginFails[key], cutoff)
	if len(kept) == 0 {
		delete(h.loginFails, key)
	} else {
		h.loginFails[key] = kept
	}
	return len(kept) >= loginMaxFails
}

// pruneLoginFails returns fails with every timestamp at or before cutoff
// dropped, reusing fails' backing array (no allocation on the common case).
func pruneLoginFails(fails []time.Time, cutoff time.Time) []time.Time {
	kept := fails[:0]
	for _, ts := range fails {
		if ts.After(cutoff) {
			kept = append(kept, ts)
		}
	}
	return kept
}

// sweepLoginFailsLocked prunes every key's window (not just the single key the
// caller is asking about) once every loginSweepEvery calls (see
// loginSweepCalls, api.go), or immediately if the map has already grown past
// loginMaxTracked — so a flood of one-off distinct keys that are each queried
// exactly once still gets cleaned up eventually, instead of leaving a
// permanent entry per key forever. If the map is still over loginMaxTracked
// after that full prune, hands off to evictLeastRecentlyTouchedLocked. Must
// be called with loginMu held.
func (h *Handler) sweepLoginFailsLocked() {
	h.loginSweepCalls++
	if h.loginSweepCalls < loginSweepEvery && len(h.loginFails) <= loginMaxTracked {
		return
	}
	h.loginSweepCalls = 0
	cutoff := time.Now().Add(-loginWindow)
	for k, fails := range h.loginFails {
		if kept := pruneLoginFails(fails, cutoff); len(kept) == 0 {
			delete(h.loginFails, k)
		} else {
			h.loginFails[k] = kept
		}
	}
	if len(h.loginFails) > loginMaxTracked {
		h.evictLeastRecentlyTouchedLocked()
	}
}

// evictLeastRecentlyTouchedLocked deletes entries from h.loginFails, oldest
// first, until it is back at or under loginMaxTracked. Only called by
// sweepLoginFailsLocked, and only once a full prune still leaves the map over
// that cap — e.g. loginMaxTracked distinct keys are ALL currently within
// their window, so none of them were empty for the prune to remove. Must be
// called with loginMu held.
//
// "Least recently touched" = the LATEST remaining failure timestamp
// (fails[len(fails)-1], since recordLoginFail appends in chronological
// order), so an attacker actively retrying stays tracked longer than one who
// fired once and went quiet.
//
// Eviction candidates EXCLUDE any key that is currently throttled (len(fails)
// >= loginMaxFails, the same condition loginThrottled itself uses to return
// true). This matters because a throttled key's own timestamps stop
// advancing the instant it starts being throttled: handleLogin checks
// loginThrottled BEFORE ever calling recordLoginFail, so a blocked attacker
// can't add new entries to its own window while waiting it out. Its "last
// touched" timestamp therefore goes stale immediately, even though the
// attacker is still very much active from a security standpoint — sorting on
// recency and evicting the oldest would evict a genuinely-throttled attacker
// BEFORE a flood of one-off keys that only just arrived, un-throttling them
// mid-lockout. (Reproduced end-to-end: attacker throttled after
// loginMaxFails failures, then a flood of one-off keys large enough to push
// the map over loginMaxTracked evicted the attacker's own entry — since
// their last fail predated the flood — and the attacker's very next request
// came back 200 instead of 429.) A bounded number of currently-throttled keys
// sitting over the nominal cap is an acceptable, self-limiting exception:
// it's bounded by how many callers actually reach loginMaxFails failures, a
// far smaller and self-capping population than an unlimited flood of one-off
// keys that only ever fail once — not the same unbounded-growth risk the cap
// exists to guard against.
func (h *Handler) evictLeastRecentlyTouchedLocked() {
	type keyAge struct {
		key  string
		last time.Time
	}
	ages := make([]keyAge, 0, len(h.loginFails))
	for k, fails := range h.loginFails {
		if len(fails) >= loginMaxFails {
			continue // currently throttled — never an eviction candidate
		}
		ages = append(ages, keyAge{k, fails[len(fails)-1]})
	}
	sort.Slice(ages, func(i, j int) bool { return ages[i].last.Before(ages[j].last) })
	for _, a := range ages {
		if len(h.loginFails) <= loginMaxTracked {
			break
		}
		delete(h.loginFails, a.key)
	}
}

func (h *Handler) recordLoginFail(key string) {
	h.loginMu.Lock()
	if h.loginFails == nil {
		h.loginFails = make(map[string][]time.Time)
	}
	h.loginFails[key] = append(h.loginFails[key], time.Now())
	h.loginMu.Unlock()
}

func (h *Handler) recordLoginSuccess(key string) {
	h.loginMu.Lock()
	delete(h.loginFails, key)
	h.loginMu.Unlock()
}

func (h *Handler) handleLogin(w http.ResponseWriter, r *http.Request) {
	hash, epoch, on := h.authEnabled()
	if !on {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": "authentication is not enabled"})
		return
	}
	key := loginClientKey(r)
	if h.loginThrottled(key) {
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
		h.recordLoginFail(key)
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": "invalid password"})
		return
	}
	h.recordLoginSuccess(key)

	tok := secret.NewSessionToken(h.cfg.AppKey, hash, epoch, sessionTTL)
	http.SetCookie(w, h.newSessionCookie(tok, int(sessionTTL.Seconds())))
	writeJSON(w, http.StatusOK, okEnvelope(nil))
}

// handleLogout handles POST /api/logout.
// Clears the session cookie unconditionally. This is CLIENT-SIDE cookie removal
// only: the stateless token itself stays valid until it expires, so a copied
// cookie would still work. Revocation is handleLogoutAll (POST /api/logout-all),
// which rotates the session epoch and thereby invalidates every outstanding
// cookie server-side.
func (h *Handler) handleLogout(w http.ResponseWriter, _ *http.Request) {
	http.SetCookie(w, h.newSessionCookie("", -1))
	writeJSON(w, http.StatusOK, okEnvelope(nil))
}

// newSessionEpoch returns a fresh random session epoch (16 bytes, hex).
func newSessionEpoch() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate session epoch: %w", err)
	}
	return hex.EncodeToString(buf), nil
}

// handleLogoutAll handles POST /api/logout-all — "log out everywhere".
// It rotates the stored session epoch to a fresh random value; because every
// session token's HMAC is bound to the epoch, ALL outstanding cookies (on every
// browser/device) become invalid at once. This is the revocation path for the
// otherwise stateless 7-day tokens. The caller's own cookie is cleared too, so
// the SPA lands on the login screen immediately.
func (h *Handler) handleLogoutAll(w http.ResponseWriter, _ *http.Request) {
	s, err := h.store.GetSettings()
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	epoch, err := newSessionEpoch()
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	s.SessionEpoch = epoch
	if err := h.store.UpdateSettings(s); err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
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
//   - GET  /widget and GET /api/widget/data  (the embeddable dashboard widget —
//     an iframe on another dashboard can't carry the session cookie either;
//     both endpoints gate themselves via the stored widget token instead,
//     failing closed with 403 when none is set. POST/DELETE /api/widget/token
//     stay session-protected — only a logged-in admin manages the token.)
//   - GET  /api/fleet/status  (another BombVault instance's Fleet view polling
//     this one — same reasoning as the widget, self-gated on the stored fleet
//     token instead, failing closed with 403 when none is set. POST/DELETE
//     /api/fleet/token and the /api/fleet/peers CRUD stay session-protected.)
//   - POST /api/fleet/mesh-offer  (a peer offering its own off-site storage —
//     same self-gated fleet token as the status poll above; the ONLY write
//     endpoint on this allowlist. It only ever stores a pending offer for a
//     human to review; accept/decline/propose stay session-protected.)
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
			case "/api/auth", "/api/login", "/api/health", "/metrics", "/widget", "/api/widget/data", "/api/fleet/status", "/api/fleet/mesh-offer":
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
		// /metrics scrape endpoint (Prometheus can't carry the session cookie),
		// the self-gating widget endpoints (an embedding iframe can't either),
		// the self-gating fleet status endpoint (a polling peer can't either),
		// and the self-gating mesh-offer inbox (same reasoning, and the same
		// fleet token — see the doc comment above).
		switch r.URL.Path {
		case "/api/auth", "/api/login", "/api/health", "/metrics", "/widget", "/api/widget/data", "/api/fleet/status", "/api/fleet/mesh-offer":
			next.ServeHTTP(w, r)
			return
		}

		// All other /api/* routes require a valid session cookie.
		c, err := r.Cookie(sessionCookieName)
		if err != nil || !secret.ValidSessionToken(h.cfg.AppKey, hash, s.SessionEpoch, c.Value) {
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
	// The source rides the BODY here (not ?source=), so normalize it explicitly —
	// same contract as sourceParam, incl. the "offsite:<id>" per-target form.
	started, auto, err := h.svc.StartRestoreConfig(r.Context(), body.Snapshot, normalizeSource(body.Source))
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
	// When export encryption is on, DownloadFlashZip age-seals the stream, so the
	// attachment is <name>.zip.age and the browser must save it under that name.
	encrypted := h.svc.ExportEncryptionOn()
	var resolved string
	lw := &headerOnFirstWrite{w: w, header: func() {
		name := FlashDownloadName(resolved)
		if encrypted {
			w.Header().Set("Content-Type", "application/octet-stream")
			name += ".age"
		} else {
			w.Header().Set("Content-Type", "application/zip")
		}
		w.Header().Set("Content-Disposition", `attachment; filename="`+name+`"`)
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
		ScheduleCadence   *string `json:"scheduleCadence"`
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
	if body.ScheduleCadence != nil {
		if err := h.svc.SetVMScheduleCadence(r.Context(), name, *body.ScheduleCadence); err != nil {
			writeJSON(w, http.StatusOK, failEnvelope(err))
			return
		}
		// A per-item override (#121) is structural: reload so the VM's own cron entry
		// is (de)registered (a VM PATCH does not otherwise reload the scheduler).
		if err := h.reloadScheduler(); err != nil {
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

// mkdirRequest is the JSON body for POST /api/browse/mkdir.
type mkdirRequest struct {
	Path string `json:"path"` // parent subpath under HostMountRoot ("" = the root)
	Name string `json:"name"` // new folder name — a single plain path component
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
	// The name must be one plain path component: reject empty, ".", "..", any
	// separator, NUL, or a leading dot. paths.Resolve below re-checks containment,
	// but this keeps "create ONE folder here" honest (no nested paths, no traversal,
	// and no hidden entry the browser would then hide again).
	if name == "" || name == "." || name == ".." ||
		strings.ContainsAny(name, "/\\\x00") || strings.HasPrefix(name, ".") {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": "invalid folder name"})
		return
	}
	// Build the new subpath under the mount root and validate containment exactly
	// the way handleBrowse does (rejecting traversal / absolute paths defensively).
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
	// os.Mkdir (not MkdirAll) so an existing folder is reported instead of silently
	// reused; then force 0755 in case a strict umask stripped the mode.
	if err := os.Mkdir(abs, 0o755); err != nil { //nolint:gosec // G301: a backup destination on a user-visible share must be operator-readable
		if os.IsExist(err) {
			writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": "a folder with that name already exists"})
			return
		}
		log.Printf("api: mkdir %q: %v", abs, err) //nolint:gosec // G706: abs is a Resolve-validated child path; no raw user bytes reach the log formatter
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": "could not create folder"})
		return
	}
	_ = os.Chmod(abs, 0o755) //nolint:gosec // G302: see above — must be readable by the non-root share user
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "path": sub, "name": name})
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
	oldName := fs.Name
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
	// Refuse a rename once the set has backups: its snapshots are tagged
	// fileset:<oldName> and are never re-tagged, so a rename would strand them
	// (DeleteBackupsFileSet then can't find them). Path/excludes/enabled edits stay
	// allowed — only the name is load-bearing for the snapshot tags.
	if fs.Name != oldName {
		hasBackups, bErr := h.svc.fileSetHasBackups(r.Context(), id)
		if bErr != nil {
			writeJSON(w, http.StatusOK, failEnvelope(bErr))
			return
		}
		if hasBackups {
			writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": "cannot rename a file set that already has backups; create a new set instead"})
			return
		}
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
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	if files == nil {
		files = []restic.FileEntry{}
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"files": files}))
}

// handleRestoreFileSetFiles starts a SELECTIVE file-set restore ON THE SERVER and
// returns immediately (see handleRestoreFileSet). An empty targetPath restores the
// selected paths IN PLACE (original locations, confirm-gated); a non-empty
// targetPath extracts the selection into that folder under the host mount
// (non-destructive). Validation + target resolution run synchronously (the
// resolved target is returned in the ack); the restic work runs detached,
// publishing "files:<name>" progress and recording a run for the outcome.
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

// handleForeignOpen opens ANOTHER BombVault instance's repository READ-ONLY
// with that instance's APP_KEY and returns an in-memory session id plus the
// full snapshot inventory (#61). Nothing is persisted — the session (and the
// foreign key) lives only in memory with a TTL, and the foreign repo is only
// probed (never initialised). The key is never logged or echoed; errors go
// through the scrubber (failEnvelope) like every other handler.
// POST /api/foreign/open  body {location, key}
func (h *Handler) handleForeignOpen(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Location string `json:"location"`
		Key      string `json:"key"`
	}
	if !decodeBody(w, r, &body) {
		return
	}
	session, inv, err := h.svc.OpenForeign(r.Context(), body.Location, body.Key)
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"session": session, "inventory": inv}))
}

// handleForeignClose drops a foreign-repo session immediately (the UI calls it
// on leave/unmount). Unknown or already-expired ids are a harmless no-op, so
// this always succeeds. POST /api/foreign/close  body {session}
func (h *Handler) handleForeignClose(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Session string `json:"session"`
	}
	if !decodeBody(w, r, &body) {
		return
	}
	h.svc.CloseForeign(body.Session)
	writeJSON(w, http.StatusOK, okEnvelope(nil))
}

// handleForeignRestore restores ONE item (container, VM or file set) from an
// open foreign-repo session ON THE SERVER and returns immediately (see
// handleRestore — the restic work runs detached; the SPA watches the item's
// "container:/vm:/files:<item>" progress key over SSE and reads the recorded
// run for the outcome). Validation runs synchronously: a bad request — an
// unknown/expired session, an unconfirmed restore, a missing file-set target —
// fails right away with a 4xx and nothing starts; the shared single-flight
// guard answers 409 like the other backup/restore starters. The session key
// stays server-side (never in this request), and errors are scrubbed.
// POST /api/foreign/restore  body {session, domain, item, snapshot, confirm, target, paths, zvolPool}
// A non-empty paths[] (files domain only) restores just those subfolders/files
// from the set into target; empty restores the whole set (issue #123). zvolPool
// is VMS-DOMAIN-ONLY and OPTIONAL: a destination ZFS pool name for a VM whose
// disks include a TrueNAS zvol (block-device) disk — required for such a VM's
// zvol disk(s) to actually restore on a cross-instance (target set) restore;
// see StartForeignRestore's doc comment. There is no UI for this field yet —
// it is reachable via a direct API call only; the request fails with a clear,
// actionable error instead of a deep zfs-receive failure when it's needed but
// missing.
func (h *Handler) handleForeignRestore(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Session   string   `json:"session"`
		Domain    string   `json:"domain"`
		Item      string   `json:"item"`
		Snapshot  string   `json:"snapshot"`
		Confirm   bool     `json:"confirm"`
		Target    string   `json:"target"`
		Paths     []string `json:"paths"`
		Overwrite bool     `json:"overwrite"`
		ZvolPool  string   `json:"zvolPool"`
	}
	if !decodeBody(w, r, &body) {
		return
	}
	started, err := h.svc.StartForeignRestore(r.Context(), body.Session, body.Domain, body.Item, body.Snapshot, body.Confirm, body.Target, body.Paths, body.Overwrite, body.ZvolPool)
	if err != nil { // synchronous validation failed — nothing was started
		writeJSON(w, http.StatusBadRequest, failEnvelope(err))
		return
	}
	if !started { // another backup/restore holds the single-flight guard
		writeJSON(w, http.StatusConflict, map[string]any{"ok": false, "error": "a backup or restore is already running"})
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"started": true}))
}

// handleForeignFiles lists the files of one file set's snapshot in an open foreign
// session, so the recovery UI can offer a subfolder/file picker before a selective
// restore (issue #123). Read-only; the session key stays server-side and errors
// are scrubbed. The response mirrors the local list-files endpoint so the SPA
// reuses the same shape. POST /api/foreign/files  body {session, domain, item, snapshot}
func (h *Handler) handleForeignFiles(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Session  string `json:"session"`
		Domain   string `json:"domain"`
		Item     string `json:"item"`
		Snapshot string `json:"snapshot"`
	}
	if !decodeBody(w, r, &body) {
		return
	}
	files, err := h.svc.ListForeignFiles(r.Context(), body.Session, body.Domain, body.Item, body.Snapshot)
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	if files == nil {
		files = []restic.FileEntry{}
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"files": files}))
}

// handleForeignContainerWarnings returns the non-appdata binds of a foreign
// container that point at a pool this host lacks, so the Recovery card can warn
// the operator BEFORE a cross-pool restore (appdata is remapped automatically;
// these binds are theirs to fix in the template). Read-only; key stays
// server-side. POST /api/foreign/container-warnings  body {session, item}
func (h *Handler) handleForeignContainerWarnings(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Session string `json:"session"`
		Item    string `json:"item"`
	}
	if !decodeBody(w, r, &body) {
		return
	}
	warnings, err := h.svc.ForeignContainerBindWarnings(r.Context(), body.Session, body.Item)
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	if warnings == nil {
		warnings = []ForeignBindWarning{}
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"warnings": warnings}))
}
