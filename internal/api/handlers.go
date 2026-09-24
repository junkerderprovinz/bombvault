package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"strings"

	"github.com/junkerderprovinz/bombvault/internal/backup"
)

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

// codedFailEnvelope is failEnvelope plus a machine-readable code, so a client
// can branch on the kind of failure without parsing the error text. The tree
// view uses "empty-selection" to show guidance instead of a bare failure.
func codedFailEnvelope(err error, code string) map[string]any {
	return map[string]any{"ok": false, "error": scrubError(err), "code": code}
}

// absPathRe matches absolute unix paths so they can be stripped from any error
// message that slips through to the API surface.
var absPathRe = regexp.MustCompile(`(/[^\s:"']+)+`)

// credentialRe matches a "user:password@" userinfo segment such as the one in
// "rest:https://backupuser:Tr0ub4dor&3@host:8000/repo", which absPathRe misses
// because it stops at the first ":". The username may be all digits. Requiring
// the "@" and keeping "/" out of the password keeps it off plain "host:port"
// text. internal/restic and internal/virshcli run the same scrub, so an error
// that never went through restic.lastReason is still cleaned here.
//
// It also matches benign shapes such as "nginx:1.25@sha256:abc123", because a
// tighter pattern would risk missing a real credential. Outside a repository
// location, a password with an unencoded "/" is only partly caught: the path
// pass runs first and consumes the "//...@" span, so the part of the password
// before the "/" survives.
var credentialRe = regexp.MustCompile(`[\w.+%-]+:[^\s/@"']+@`)

// repoLocationRe matches a remote restic repository location inside a
// sentence. Such a location skips the path scrubber, because a failure message
// about a repository has to say which one: its host, port and bucket are the
// operator's own storage layout, and repoUserinfoRe removes the credential.
// internal/restic/restic.go keeps the same pattern.
var repoLocationRe = regexp.MustCompile(`\b(?:rclone|sftp|rest|s3|b2|azure|gs|swift):[^\s"']+`)

// repoUserinfoRe matches the "user:password@" of a remote repository location.
// Unlike credentialRe its password body may contain "/", which is safe because
// the match is bounded by the scheme on the left and the "@" on the right and
// never crosses whitespace or quotes. Without it, a password inside a location
// that skips the path scrubber would survive whole.
var repoUserinfoRe = regexp.MustCompile(`\b(rclone|sftp|rest|s3|b2|azure|gs|swift):((?:[A-Za-z0-9+.-]+:)?//)?[^@\s"']*@`)

// scrubSecrets redacts the userinfo of every remote repository location in s
// and scrubs the text around those locations with scrubOutsideARepoLocation.
func scrubSecrets(s string) string {
	var b strings.Builder
	last := 0
	for _, m := range repoLocationRe.FindAllStringIndex(s, -1) {
		b.WriteString(scrubOutsideARepoLocation(s[last:m[0]]))
		b.WriteString(repoUserinfoRe.ReplaceAllString(s[m[0]:m[1]], "${1}:${2}[redacted]@"))
		last = m[1]
	}
	b.WriteString(scrubOutsideARepoLocation(s[last:]))
	return b.String()
}

// scrubOutsideARepoLocation strips paths and then userinfo credentials from the
// parts of a message that are not a remote repository location. Paths go first:
// the other way round, the leftover "scheme://[redacted]@host" is path-shaped
// and absPathRe eats the hostname an operator needs to tell targets apart.
func scrubOutsideARepoLocation(s string) string {
	s = absPathRe.ReplaceAllString(s, "[path]")
	return credentialRe.ReplaceAllString(s, "[redacted]@")
}

// errRestoreDestination tags a restore destination refusal whose message only
// helps with the path in it, such as "already holds data", "not on a mounted
// pool" or "no room". The path is the operator's own chosen location, never a
// repo path or a secret, so these bypass the path scrubber.
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

// scrubBypassMessage returns err's message unscrubbed, and true, when err is a
// sentinel whose path-shaped content (a restore folder, the relative repo
// location to type instead, /boot vs /host/boot, a ZFS dataset, a host:port
// conflict list) is what the message is for. scrubError and truncateRunErr
// both ask it first, so they cannot disagree on what is shown verbatim.
func scrubBypassMessage(err error) (string, bool) {
	switch {
	case errors.Is(err, backup.ErrRestoreConflict):
		// IPs, host ports and container names only, and the path scrubber would
		// turn "8080/tcp" into "8080[path]".
		return err.Error(), true
	case errors.Is(err, errRestoreDestination):
		return err.Error(), true
	case errors.Is(err, errRepoPathGuidance):
		// The rejected location and the relative form to use instead are the
		// message.
		return err.Error(), true
	case errors.Is(err, errUnraidPlatformMismatch):
		// /boot and /host/boot are what the operator has to act on.
		return err.Error(), true
	case errors.Is(err, errZvolRebaseFailed):
		// The ZFS dataset and pool names are the message and contain "/".
		return err.Error(), true
	case errors.Is(err, errRestPathUser):
		// No path here, just two htpasswd user names. It bypasses so the exact
		// difference reaches the operator instead of restAuthHint's generic list.
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
		return "restore not confirmed: set confirm:true to proceed"
	case errors.Is(err, backup.ErrInvalidSnapshotID):
		return "invalid snapshot id (must be 8 to 64 lowercase hex)"
	}
	if msg, ok := scrubBypassMessage(err); ok {
		return msg
	}
	msg := err.Error()
	// Map restic's password/key mismatch to an actionable hint: the repo was
	// created with a different APP_KEY or a different encryption setting.
	if strings.Contains(msg, "wrong password or no key found") {
		return "backup repository can't be opened: the APP_KEY differs from when this repo was first created (or encryption was toggled). Use the original APP_KEY, or point Settings at a fresh, empty backup path."
	}
	if hint := restAuthHint(msg); hint != "" {
		return hint
	}
	msg = scrubSecrets(msg)
	return strings.TrimSpace(msg)
}

// restStatus401 matches 401 as a status code rather than as three digits inside
// a longer number, such as the port in `rest:http://box:8401/repo`.
var restStatus401 = regexp.MustCompile(`(^|[^0-9])401([^0-9]|$)`)

// isAuthRefusal reports whether a lowercased restic message is a rejection
// rather than any other failure. Shared with restPathUserMismatch so the two
// cannot drift on what counts as a 401.
func isAuthRefusal(low string) bool {
	return restStatus401.MatchString(low) || strings.Contains(low, "unauthorized")
}

// isRestBackendMessage reports whether a lowercased restic message came from the
// REST backend. "rest:" alone is not enough, because for a refused rest-server
// the stderr line runError keeps is
//
//	restic cat failed: Fatal: unable to open config file: unexpected HTTP response (401): 401 Unauthorized
//
// and the URL follows on a line lastReason skips as boilerplate. Only restic's
// REST backend words a failure as "unexpected HTTP response".
func isRestBackendMessage(low string) bool {
	return strings.Contains(low, "rest:") || strings.Contains(low, "unexpected http response")
}

// restAuthHint turns a rest-server "401 Unauthorized" into its two usual causes,
// or returns "" for any other message. It is limited to rest repositories, so an
// S3 403 keeps its own wording.
func restAuthHint(msg string) string {
	low := strings.ToLower(msg)
	if !isAuthRefusal(low) || !isRestBackendMessage(low) {
		return ""
	}
	return "the rest-server rejected these credentials (401). Two things cause almost every one of these. " +
		"First, the repository URL's first path segment has to be the htpasswd user itself when the server runs " +
		"with --private-repos: with user \"tower\", the URL is rest:http://host:8000/tower/<repo>, not " +
		"rest:http://host:8000/<repo>. Second, this repository may be using the shared REST credentials rather " +
		"than the credential set you filled in, which happens when its own row does not name that set. " +
		"Check both and run the connection test again. The two-box walkthrough is in docs/offsite-recovery.md."
}

// decodeBody decodes a JSON request body into v. Returns false (and writes a
// graceful failure) on malformed JSON, on a body that is not declared as JSON,
// or on a request a browser fired from another site (see crossOriginGuard).
func decodeBody(w http.ResponseWriter, r *http.Request, v any) bool {
	if !crossOriginGuard(w, r) {
		return false
	}
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

// decodeOptionalBody is decodeBody for a route whose fields are all optional:
// an absent body leaves v at its zero value instead of failing, so a caller
// does not have to send an empty JSON object just to take the defaults.
func decodeOptionalBody(w http.ResponseWriter, r *http.Request, v any) bool {
	if !crossOriginGuard(w, r) {
		return false
	}
	if r.Body == nil {
		return true
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1 MiB
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		if errors.Is(err, io.EOF) {
			return true
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": "invalid request body"})
		return false
	}
	return true
}

func (h *Handler) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "version": Version})
}

// resourceNameRe matches a safe Docker container or libvirt VM name: it starts
// with an alphanumeric and contains only [A-Za-z0-9._-]. That rules out path
// separators, a leading "-" (argv option injection) and an empty name, and
// validResourceName adds a ".." check. The router decodes "%2f" and "%2e%2e"
// into the path value, so an unchecked {name} could carry "../" into the
// template and XML file sinks.
var resourceNameRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)

func validResourceName(name string) bool {
	return resourceNameRe.MatchString(name) && !strings.Contains(name, "..")
}

// runIDRe matches an opaque run id: exactly 32 lowercase hex chars (newID is 16
// random bytes hex-encoded). The acknowledge route checks its ids against this,
// not against validResourceName, whose Docker and VM name shape is different.
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

// validVMName accepts libvirt domain names, which unlike Docker names often
// contain spaces ("Windows 11"). A VM name only reaches argv-separated virsh
// args, restic tags after "--" and SQLite parameters, never a file path, so it
// rejects just the dangerous shapes: empty, over-long, path separators or "..",
// a leading "-" and control characters.
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

// vmNameParam is nameParam for VM routes, using validVMName so a VM name with
// spaces is not refused with a 400.
func (h *Handler) vmNameParam(w http.ResponseWriter, r *http.Request) (string, bool) {
	name := r.PathValue("name")
	if !validVMName(name) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "invalid VM name"})
		return "", false
	}
	return name, true
}

// sourceParam returns the repo source a ?source= query asks for, through
// normalizeSource. Used by the snapshot browser, restore and maintenance routes.
func sourceParam(r *http.Request) string {
	return normalizeSource(r.URL.Query().Get("source"))
}

// normalizeSource maps a raw ?source= value onto a source the service
// understands. "offsite" and a well-formed "offsite:<id>" stay as they are; a
// malformed id becomes "offsite:" with no id, which offsiteTargetForSource
// refuses; anything else is the local repo.
func normalizeSource(raw string) string {
	if id, ok := strings.CutPrefix(raw, offsiteSourcePrefix); ok {
		if validOffsiteTargetID(id) {
			return offsiteSourcePrefix + id
		}
		return offsiteSourcePrefix
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

// reloadScheduler re-reads the settings and re-registers every schedule entry,
// including the per-item entries, after a change outside the settings form
// such as a per-item cadence PATCH.
func (h *Handler) reloadScheduler() error {
	s, err := h.store.GetSettings()
	if err != nil {
		return err
	}
	return h.scheduler.ReloadWithDueChecks(s, h.containersLastRun, h.vmsLastRun, h.flashLastRun, h.configLastRun, h.filesLastRun, h.everythingLastRun)
}
