package api

import (
	"cmp"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"maps"
	"mime"
	"net"
	"net/http"
	"os"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/junkerderprovinz/bombvault/internal/backup"
	"github.com/junkerderprovinz/bombvault/internal/notify"
	"github.com/junkerderprovinz/bombvault/internal/paths"
	"github.com/junkerderprovinz/bombvault/internal/releasenotes"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/schedule"
	"github.com/junkerderprovinz/bombvault/internal/secret"
	"github.com/junkerderprovinz/bombvault/internal/spike"
	"github.com/junkerderprovinz/bombvault/internal/store"
	"runtime"
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

// sameSiteOnly refuses a request a browser fired from another website, judged by
// the Sec-Fetch-Site header, which the browser sets and a page cannot forge.
// Only "cross-site" is refused: what a browser counts as one site for a bare LAN
// IP is too uncertain to bet an install on, and a missing header means a
// non-browser client such as curl or a peer's mesh POST.
//
// It runs over every unsafe method, not only in decodeBody, because several
// state-changing routes take no body, such as POST /api/backup-everything.
func sameSiteOnly(w http.ResponseWriter, r *http.Request) bool {
	if r.Header.Get("Sec-Fetch-Site") != "cross-site" {
		return true
	}
	writeJSON(w, http.StatusForbidden, map[string]any{
		"ok":    false,
		"error": "refused a cross-site request",
	})
	return false
}

// csrfGate applies sameSiteOnly to every method that can change state. Safe
// methods (GET/HEAD/OPTIONS) pass through untouched: they are reachable
// cross-site by design, the browser's same-origin policy keeps the response
// unreadable, and gating them would break the widget iframe and a peer's status
// poll, both of which are GETs on purpose.
func csrfGate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
		default:
			if !sameSiteOnly(w, r) {
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

// crossOriginGuard refuses a body-carrying request that a browser fired from
// another website. With no login password set, authGate lets every request
// through on the assumption that an attacker has to be on the LAN, but a
// cross-site HTML form with enctype="text/plain" can post a body the JSON
// decoder accepts from the operator's own browser, with no preflight and no
// cookie. That would be enough to repoint notifications or set the Backup
// Everything hooks, which run as `sh -c` next to the Docker socket.
//
// Requiring a JSON Content-Type does the work: a cross-origin form cannot send
// it, and fetch() with it needs a CORS preflight this server never answers.
// sameSiteOnly adds the Sec-Fetch-Site check. Neither needs a token, which
// matters because trusted-LAN mode has no session to hang one on.
func crossOriginGuard(w http.ResponseWriter, r *http.Request) bool {
	if !sameSiteOnly(w, r) {
		return false
	}
	ct := r.Header.Get("Content-Type")
	if mediaType, _, err := mime.ParseMediaType(ct); err != nil || mediaType != "application/json" {
		writeJSON(w, http.StatusUnsupportedMediaType, map[string]any{
			"ok":    false,
			"error": "this endpoint takes application/json; set the Content-Type header",
		})
		return false
	}
	return true
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

// handleTakeOverVM moves a VM entry onto the libvirt name its VM was renamed
// to. POST /api/vms/{name}/takeover with body {"from":"<old libvirt name>"};
// both are libvirt names, never TrueNAS display names.
func (h *Handler) handleTakeOverVM(w http.ResponseWriter, r *http.Request) {
	name, ok := h.vmNameParam(w, r)
	if !ok {
		return
	}
	var body struct {
		From string `json:"from"`
	}
	if !decodeBody(w, r, &body) {
		return
	}
	if !validVMName(body.From) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "invalid VM name"})
		return
	}
	if err := h.svc.TakeOverVM(r.Context(), body.From, name); err != nil {
		takeoverFail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(nil))
}

// handleUnlinkVMAlias undoes a VM takeover. DELETE /api/vms/{name}/alias/{old};
// {old} alone finds the entry, since a former name belongs to one entry.
func (h *Handler) handleUnlinkVMAlias(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.vmNameParam(w, r); !ok {
		return
	}
	old := r.PathValue("old")
	if !validVMName(old) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "invalid VM name"})
		return
	}
	if err := h.svc.UnlinkVMAlias(r.Context(), old); err != nil {
		takeoverFail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(nil))
}

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

// handleBackupEverything starts a "Backup Everything" pass, which runs the
// containers, vms, flash, files and config domains in sequence (everything.go),
// on the server and returns immediately. Like handleBackupAll it answers 409
// with a reason when the pass fails to start or one is already running.
func (h *Handler) handleBackupEverything(w http.ResponseWriter, r *http.Request) {
	started, err := h.svc.StartBackupEverything(r.Context())
	if err != nil { // mirrors handleBackupAll: any failure to even start is reported the same way
		writeJSON(w, http.StatusConflict, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	if !started {
		writeJSON(w, http.StatusConflict, map[string]any{"ok": false, "error": "a Backup Everything pass is already running"})
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"started": true}))
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

// stackParam reads {project}, a compose project name, which is laxer than a
// container name but must not carry a path.
func stackParam(w http.ResponseWriter, r *http.Request) (string, bool) {
	project := r.PathValue("project")
	if project == "" || strings.Contains(project, "/") || strings.Contains(project, "..") {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "invalid stack name"})
		return "", false
	}
	return project, true
}

// handleRestoreStack restores every backed-up member of a compose stack stopped,
// then optionally starts them in dependency order.
// POST /api/stacks/{project}/restore
//
// Like handleRestore it runs detached after validation and member enumeration,
// so a bad request or an empty stack still fails right away. Each member's
// restore records its own "restore" run.
func (h *Handler) handleRestoreStack(w http.ResponseWriter, r *http.Request) {
	project, ok := stackParam(w, r)
	if !ok {
		return
	}
	var body struct {
		StartAfter     bool   `json:"startAfter"`
		Confirm        bool   `json:"confirm"`
		StackDirSource string `json:"stackDirSource"`
	}
	if !decodeBody(w, r, &body) {
		return
	}
	dirSource := ""
	if body.StackDirSource != "" {
		dirSource = normalizeSource(body.StackDirSource)
	}
	started, err := h.svc.StartRestoreStack(r.Context(), project, sourceParam(r), dirSource, body.StartAfter, body.Confirm)
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
	// BackupCores caps the CPU threads each restic child uses (GOMAXPROCS);
	// 0 means every core, restic's own default.
	BackupCores int `json:"backupCores"`
	// The opt-in Prometheus /metrics endpoint and its optional bearer token.
	// Like every secret here, GET returns the token blank with MetricsTokenSet
	// reporting whether one is stored, and a blank token on PUT keeps the
	// stored one. MetricsTokenSet sits on the struct because the strict PUT
	// decoder has to accept a round-tripped GET body.
	MetricsEnabled  bool   `json:"metricsEnabled"`
	MetricsToken    string `json:"metricsToken"`
	MetricsTokenSet bool   `json:"metricsTokenSet"`
	// WidgetToken authorizes the embeddable dashboard widget, with the same
	// secret contract as MetricsToken. POST and DELETE /api/widget/token set
	// and clear it; it is part of the PUT round-trip so a full settings save
	// cannot wipe it.
	WidgetToken    string `json:"widgetToken"`
	WidgetTokenSet bool   `json:"widgetTokenSet"`
	// Scheduled restore-verification drills (restic check --read-data-subset).
	DrillsEnabled   bool   `json:"drillsEnabled"`
	DrillsSchedule  string `json:"drillsSchedule"`
	DrillsSubsetPct int    `json:"drillsSubsetPct"`
	// OffsiteDrillsEnabled gates the scheduled off-site DR drill alone; the local
	// subset check and the manual DR button are unaffected. Default on.
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
	// tar.gz, xml and zip exports). Recipients are public keys (age1... or SSH),
	// so they round-trip in the clear. With encryption on and no valid recipient
	// every export fails rather than writing plaintext.
	ExportEncryptEnabled bool   `json:"exportEncryptEnabled"`
	ExportAgeRecipients  string `json:"exportAgeRecipients"`
	// ReceiverEnabled gates the read-only receiver dashboard, for a box that
	// receives immutable off-site copies and monitors them. Off by default.
	ReceiverEnabled bool `json:"receiverEnabled"`
	// RestartHealthWait makes the restart of the "stop other containers during
	// backup" set wait for each stopped dependency to become healthy (or running
	// plus a short grace without a healthcheck) before starting what depends on
	// it. The depends_on order always applies. Default on.
	// RestartHealthTimeoutSec caps that wait per container (default 120).
	RestartHealthWait       bool `json:"restartHealthWait"`
	RestartHealthTimeoutSec int  `json:"restartHealthTimeoutSec"`
	// ReconcileUnraidUpdateStatus asks Unraid to refresh its own cached "update
	// available" status after the post-backup update step recreates a container,
	// so the Docker tab's stale banner clears. It runs over the host SSH link and
	// a failure is not fatal. Default on.
	ReconcileUnraidUpdateStatus bool `json:"reconcileUnraidUpdateStatus"`
	// PerItemSchedules lets an included container or VM with a non-empty
	// scheduleCadence run on its own cadence. Off by default, which keeps the
	// domain schedule in charge of every item.
	PerItemSchedules bool `json:"perItemSchedules"`
	// RegistryAuths are the private registry credentials for the post-backup
	// update pull. Each token follows the MetricsToken contract; a host missing
	// from the list is deleted, and nil (an old client) keeps the stored list.
	RegistryAuths []registryAuthView `json:"registryAuths"`
	// FleetEnabled gates the read-only Fleet view of peer instances this box
	// polls for their protection status. Off by default.
	FleetEnabled bool `json:"fleetEnabled"`
	// PullEnabled gates fetching another instance's backups into this box's own
	// repository. Off by default, and unlike the two flags above it writes data.
	PullEnabled bool `json:"pullEnabled"`
	// InstanceName is this instance's display name, reported to polling fleet
	// peers so their Fleet page can label this box. Not a secret.
	InstanceName string `json:"instanceName"`
	// FleetToken lets other instances' Fleet views poll GET /api/fleet/status,
	// with the same secret contract as WidgetToken. POST and DELETE
	// /api/fleet/token set and clear it.
	FleetToken    string `json:"fleetToken"`
	FleetTokenSet bool   `json:"fleetTokenSet"`
	// EverythingSchedule is the cadence of the "Backup Everything" pass over all
	// domains; 'off', the default, leaves it inert.
	EverythingSchedule string `json:"everythingSchedule"`
	// The hooks run in BombVault's own container before and after the whole
	// pass. They are never echoed, because a useful hook often carries a secret
	// in its URL (a healthchecks.io ping is a UUID) and without a login password
	// anyone on the LAN can read this. The ...Set flags report presence, a blank
	// field on PUT keeps the stored command, and the ...Clear flags remove one,
	// since blank alone could never get rid of it.
	EverythingPreHook       string `json:"everythingPreHook"`
	EverythingPostHook      string `json:"everythingPostHook"`
	EverythingPreHookSet    bool   `json:"everythingPreHookSet"`
	EverythingPostHookSet   bool   `json:"everythingPostHookSet"`
	EverythingPreHookClear  bool   `json:"everythingPreHookClear"`
	EverythingPostHookClear bool   `json:"everythingPostHookClear"`
}

// registryAuthView is one container registry credential in the settings view.
// Token is write-only; TokenSet sits on the struct because the strict PUT
// decoder has to accept a round-tripped GET body.
type registryAuthView struct {
	Host     string `json:"host"`
	Username string `json:"username"`
	Token    string `json:"token"`
	TokenSet bool   `json:"tokenSet"`
}

func toView(s store.Settings) settingsView {
	return settingsView{
		EncryptionEnabled: s.EncryptionEnabled,
		ContainersEnabled: s.ContainersEnabled,
		VMsEnabled:        s.VMsEnabled,
		FlashEnabled:      s.FlashEnabled,
		ConfigEnabled:     s.ConfigEnabled,
		FilesEnabled:      s.FilesEnabled,
		ContainersPath:    s.ContainersPath,
		VMsPath:           s.VMsPath,
		FlashPath:         s.FlashPath,
		ConfigPath:        s.ConfigPath,
		FilesPath:         s.FilesPath,
		RestoreFolder:     s.RestoreFolder,
		// Verbatim: the credentialed export, gated on a login password, needs a
		// complete copy. The other exits scrub on their own, through
		// redactExportLocations and scrubGetSettingsSecrets.
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
		BackupCores:                 s.BackupCores,
		MetricsEnabled:              s.MetricsEnabled,
		MetricsToken:                "", // secret, never echoed; MetricsTokenSet reports presence
		MetricsTokenSet:             s.MetricsToken != "",
		WidgetToken:                 "", // secret, never echoed; WidgetTokenSet reports presence
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
		PullEnabled:                 s.PullEnabled,
		InstanceName:                s.InstanceName,
		FleetToken:                  "", // secret, never echoed; FleetTokenSet reports presence
		FleetTokenSet:               s.FleetToken != "",
		EverythingSchedule:          s.EverythingSchedule,
		EverythingPreHook:           s.EverythingPreHook,
		EverythingPostHook:          s.EverythingPostHook,
		EverythingPreHookSet:        s.EverythingPreHook != "",
		EverythingPostHookSet:       s.EverythingPostHook != "",
	}
}

// clampHealthTimeoutSec keeps the restart health-wait timeout between 5 seconds
// and an hour, so a typo cannot let one stuck dependency block a restart for
// days. A non-positive value falls back to the 120s default.
func clampHealthTimeoutSec(sec int) int {
	if sec <= 0 {
		return 120
	}
	return min(3600, max(5, sec))
}

// scrubGetSettingsSecrets applies GET /api/settings' own policy to the shared
// view, which without a login password any host on the LAN can read.
//
// Off-site locations are scrubbed rather than blanked: a location is not a
// secret, and the wizard's backend inference and cron snippet need it, but it
// can carry one (rest:https://user:pass@host/repo). A location that comes back
// on PUT with the marker keeps the stored one. Hooks are blanked, because a
// hook's whole value is often the secret; the ...Set flags report presence.
func scrubGetSettingsSecrets(v settingsView) settingsView {
	v.ContainersOffsite = scrubRepoLocation(v.ContainersOffsite)
	v.VMsOffsite = scrubRepoLocation(v.VMsOffsite)
	v.FlashOffsite = scrubRepoLocation(v.FlashOffsite)
	v.ConfigOffsite = scrubRepoLocation(v.ConfigOffsite)
	v.FilesOffsite = scrubRepoLocation(v.FilesOffsite)
	v.EverythingPreHook = ""
	v.EverythingPostHook = ""
	return v
}

func (h *Handler) handleGetSettings(w http.ResponseWriter, _ *http.Request) {
	s, err := h.store.GetSettings()
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	view := scrubGetSettingsSecrets(toView(s))
	// Registry credentials are stored encrypted, which toView cannot decode.
	// Tokens are never echoed; TokenSet reports presence.
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
	// Nested under "settings" so a client can GET, edit and PUT back the same
	// object; hostMountRoot and platform sit beside it so the strict PUT
	// decoder never sees them. platform is the detected or overridden
	// platform.Kind ("unraid", "generic", "truenas") and cannot be changed here.
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":            true,
		"settings":      view,
		"hostMountRoot": h.cfg.HostMountRoot,
		"platform":      string(h.svc.platformFn().Kind()),
	})
}

// rejectEveryNSchedules returns the user-facing error for a view whose off-site
// replication cadence uses "everyN", or "" when it is acceptable. everyN is a
// daily trigger plus a "has the interval elapsed?" gate, and a replication job
// has no last run to answer that with, so it would fire daily.
//
// Every settings write path calls it, handlePutSettings and validateExport
// alike, because the UI always PUTs the full object and one bad imported field
// would break every later save. The domain, drill, tamper-test, digest and
// Everything schedules record their last run, so everyN works for them. The
// scheduler also refuses to register an everyN it cannot enforce; this is the
// friendlier check at save time.
func rejectEveryNSchedules(v settingsView) string {
	for _, cad := range []string{
		v.ContainersOffsiteSchedule, v.VMsOffsiteSchedule, v.FlashOffsiteSchedule, v.ConfigOffsiteSchedule, v.FilesOffsiteSchedule,
	} {
		if c, _ := schedule.ParseCadence(cad); c.IntervalDays > 0 {
			return "this schedule does not support 'everyN': use 'daily HH:MM', 'weekly DOW HH:MM', or a cron expression"
		}
	}
	return ""
}

// rejectSettingsPathOnNamedRepo refuses a settings save that moves a domain's
// own repository or an off-site destination onto a location a named repository
// already occupies. It returns a user-facing sentence, or "".
//
// Only the fields this save changes, compared with cur, are checked. The SPA
// sends the whole settings object from every card, so checking everything would
// refuse every save over an existing collision the operator is not touching. A
// failed store read refuses the save, since it can simply be repeated.
func (h *Handler) rejectSettingsPathOnNamedRepo(v settingsView, cur store.Settings) string {
	rows, err := h.store.ListNamedRepos()
	if err != nil {
		return "could not check this path against the repositories you set up; try again"
	}
	if len(rows) == 0 {
		return ""
	}
	for _, f := range []struct{ label, loc, was string }{
		{"Containers", v.ContainersPath, cur.ContainersPath},
		{"VMs", v.VMsPath, cur.VMsPath},
		{"Flash", v.FlashPath, cur.FlashPath},
		{"Config", v.ConfigPath, cur.ConfigPath},
		{"Folders", v.FilesPath, cur.FilesPath},
		{"Containers off-site", v.ContainersOffsite, cur.ContainersOffsite},
		{"VMs off-site", v.VMsOffsite, cur.VMsOffsite},
		{"Flash off-site", v.FlashOffsite, cur.FlashOffsite},
		{"Config off-site", v.ConfigOffsite, cur.ConfigOffsite},
		{"Folders off-site", v.FilesOffsite, cur.FilesOffsite},
	} {
		if strings.TrimSpace(f.loc) == "" {
			continue
		}
		if sameRepoLocation(strings.TrimSpace(f.loc), strings.TrimSpace(f.was)) {
			continue // unchanged by this save
		}
		loc, rErr := h.svc.resolveRepo(f.loc)
		if rErr != nil {
			continue // rejectInvalidSettingsPaths already refused what cannot resolve
		}
		for _, r := range rows {
			other, oErr := h.svc.resolveRepo(r.Repo)
			if oErr != nil || !repoLocationsOverlap(other, loc) {
				continue
			}
			return fmt.Sprintf("the %s path is, holds or lies inside the repository %q you set up under Repositories; pick a different folder, or remove that repository first", f.label, r.Name)
		}
	}
	return ""
}

// rejectNestedSettingsPath refuses a save that moves a domain path or an
// off-site field into or around another repository or target. Like
// rejectSettingsPathOnNamedRepo it checks only the fields this save changes.
func (h *Handler) rejectNestedSettingsPath(v settingsView, cur store.Settings) string {
	next := cur
	next.ContainersPath, next.VMsPath, next.FlashPath, next.ConfigPath, next.FilesPath =
		v.ContainersPath, v.VMsPath, v.FlashPath, v.ConfigPath, v.FilesPath
	next.ContainersOffsite, next.VMsOffsite, next.FlashOffsite, next.ConfigOffsite, next.FilesOffsite =
		v.ContainersOffsite, v.VMsOffsite, v.FlashOffsite, v.ConfigOffsite, v.FilesOffsite
	for _, f := range []struct {
		label, domain, loc, was string
		field                   bool
	}{
		{"Containers", "containers", v.ContainersPath, cur.ContainersPath, false},
		{"VMs", "vms", v.VMsPath, cur.VMsPath, false},
		{"Flash", "flash", v.FlashPath, cur.FlashPath, false},
		{"Config", "config", v.ConfigPath, cur.ConfigPath, false},
		{"Folders", "files", v.FilesPath, cur.FilesPath, false},
		{"Containers off-site", "containers", v.ContainersOffsite, cur.ContainersOffsite, true},
		{"VMs off-site", "vms", v.VMsOffsite, cur.VMsOffsite, true},
		{"Flash off-site", "flash", v.FlashOffsite, cur.FlashOffsite, true},
		{"Config off-site", "config", v.ConfigOffsite, cur.ConfigOffsite, true},
		{"Folders off-site", "files", v.FilesOffsite, cur.FilesOffsite, true},
	} {
		if strings.TrimSpace(f.loc) == "" || sameRepoLocation(strings.TrimSpace(f.loc), strings.TrimSpace(f.was)) {
			continue
		}
		loc, err := h.svc.resolveRepo(f.loc)
		if err != nil {
			continue // rejectInvalidSettingsPaths already refused what cannot resolve
		}
		self := locationSelf{own: f.domain}
		if f.field {
			self = locationSelf{field: f.domain}
			row, ok, err := h.store.FieldOffsiteTarget(f.domain)
			if err != nil {
				return "could not check this path against the off-site targets; try again"
			}
			if ok {
				self.ids = []string{row.ID}
			}
		}
		if err := h.svc.locationClash(next, loc, self); err != nil {
			return fmt.Sprintf("the %s path: %s", f.label, scrubError(err))
		}
	}
	return ""
}

// rejectInvalidSettingsPaths validates every repo location a settings row
// carries: the restore folder is always local, a remote backend (rclone:/s3:/
// rest:/sftp:/b2:) is accepted verbatim, an unprefixed remote-looking value is
// refused with guidance, and a local path must resolve under the mount root.
// Returns a user-facing message, or "" when the whole set is acceptable. Both
// settings write paths share it, for the reason rejectEveryNSchedules gives.
func rejectInvalidSettingsPaths(v settingsView, mountRoot string) string {
	// Restores land on the local mount root. A remote-looking value such as
	// "s3:foo" would slip past the containment check below, which skips remotes.
	if v.RestoreFolder != "" && restic.IsRemoteRepo(v.RestoreFolder) {
		return "restore folder must be a local path under the mount root"
	}

	// A blank off-site field means none.
	for _, sub := range []string{
		v.ContainersPath, v.VMsPath, v.FlashPath, v.ConfigPath, v.FilesPath, v.RestoreFolder,
		v.ContainersOffsite, v.VMsOffsite, v.FlashOffsite, v.ConfigOffsite, v.FilesOffsite,
	} {
		if sub == "" || restic.IsRemoteRepo(sub) {
			continue
		}
		// A "word:" prefix that is not a known remote is almost always a
		// mistyped off-site path ("BackBlaze:bucket" for
		// "rclone:BackBlaze:bucket"), not a local folder of that name.
		if restic.LooksLikeUnprefixedRemote(sub) {
			return fmt.Sprintf("%q looks like a remote backend but is missing its prefix; off-site backends need one of rclone:/s3:/rest:/sftp:/b2:, for example rclone:%s", sub, sub)
		}
		if _, err := paths.Resolve(mountRoot, sub); err != nil {
			log.Printf("api: settings: rejected path %q: %v", sub, err)
			return "invalid backup path: must be a relative subpath under the mount root, or an rclone:/s3: remote"
		}
	}
	return ""
}

// rejectInvalidSettingsNames validates the DR-drill targets, which are
// container and VM names from the UI dropdown, with the same rules as the
// name-keyed routes. Both settings write paths share it.
func rejectInvalidSettingsNames(v settingsView) string {
	if dt := strings.TrimSpace(v.DRDrillTarget); dt != "" && !validResourceName(dt) {
		return "invalid DR-drill target"
	}
	// VM names may contain spaces ("Windows 11").
	if dt := strings.TrimSpace(v.DRDrillTargetVM); dt != "" && !validVMName(dt) {
		return "invalid DR-drill target"
	}
	return ""
}

func (h *Handler) handlePutSettings(w http.ResponseWriter, r *http.Request) {
	var v settingsView
	if !decodeBody(w, r, &v) {
		return
	}

	// The same guard the import path applies, so a value one path refuses
	// cannot arrive through the other.
	if msg := rejectInvalidSettingsPaths(v, h.cfg.HostMountRoot); msg != "" {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": msg})
		return
	}
	// validateNamedRepo keeps a named repository off a domain's own path; this
	// is the other direction, a domain path moved onto a named repository,
	// which would leave a row answering for the domain with its own empty
	// credentials and an append-only flag the domain never set.
	cur, curErr := h.store.GetSettings()
	if curErr != nil {
		writeJSON(w, http.StatusOK, failEnvelope(curErr))
		return
	}
	if msg := h.rejectSettingsPathOnNamedRepo(v, cur); msg != "" {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": msg})
		return
	}
	if msg := h.rejectNestedSettingsPath(v, cur); msg != "" {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": msg})
		return
	}

	for _, cad := range []string{
		v.ContainersSchedule, v.VMsSchedule, v.FlashSchedule, v.ConfigSchedule, v.FilesSchedule,
		v.ContainersOffsiteSchedule, v.VMsOffsiteSchedule, v.FlashOffsiteSchedule, v.ConfigOffsiteSchedule, v.FilesOffsiteSchedule,
		v.DrillsSchedule, v.TamperTestSchedule, v.DigestSchedule, v.EverythingSchedule,
	} {
		if _, err := schedule.ParseCadence(cad); err != nil {
			writeJSON(w, http.StatusOK, map[string]any{
				"ok": false, "error": scrubError(err),
			})
			return
		}
	}
	if msg := rejectEveryNSchedules(v); msg != "" {
		writeJSON(w, http.StatusOK, map[string]any{
			"ok": false, "error": msg,
		})
		return
	}

	if msg := rejectInvalidSettingsNames(v); msg != "" {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": msg})
		return
	}

	// This snapshot only spots the VMs domain being switched on. Nothing written
	// back may come from it: the SSH test below can burn its whole timeout, so
	// by the time of the write it may be minutes old and would revert another
	// save. What is kept is read inside the transaction.
	existing, err := h.store.GetSettings()
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}

	// Enabling the VMs domain needs a working SSH connection to the host, or the
	// tab would appear with nothing able to back up. It is checked only when the
	// domain is switched on, so a brief host outage does not block other saves.
	if v.VMsEnabled && !existing.VMsEnabled {
		if tErr := h.svc.VMSSHTest(r.Context()); tErr != nil {
			writeJSON(w, http.StatusOK, map[string]any{
				"ok":    false,
				"error": "Can't enable VM backup yet: " + scrubError(tErr) + ". Set up the SSH key under “VM Backup over SSH” and click Test connection first.",
			})
			return
		}
	}

	// mergeRegistryAuths refuses malformed input with a message the client has
	// to get verbatim, since a registry host can contain "/" and failEnvelope
	// would turn it into "[path]". It is carried out of the callback for that.
	var registryInputErr error

	// The form's own fields are assigned one by one onto the current row inside
	// the transaction, never as a whole struct literal, so a column this form
	// does not own keeps its stored value, and the auth hash and session epoch
	// come from the row as it is now rather than from the stale snapshot.
	before := h.svc.fieldTargets()
	s, err := h.store.MutateSettings(func(cur *store.Settings) error {
		cur.EncryptionEnabled = v.EncryptionEnabled
		cur.ContainersEnabled = v.ContainersEnabled
		cur.VMsEnabled = v.VMsEnabled
		cur.FlashEnabled = v.FlashEnabled
		cur.ConfigEnabled = v.ConfigEnabled
		cur.FilesEnabled = v.FilesEnabled
		cur.ContainersPath = v.ContainersPath
		cur.VMsPath = v.VMsPath
		cur.FlashPath = v.FlashPath
		cur.ConfigPath = v.ConfigPath
		cur.FilesPath = v.FilesPath
		cur.RestoreFolder = v.RestoreFolder
		// GET hands these locations out with any credential replaced by the
		// redaction marker, so a location that still carries it keeps the stored
		// one, as the settings import does. Otherwise the next unrelated save
		// would destroy the stored password.
		keepLocation := func(incoming, stored string) string {
			if locationRedacted(incoming) {
				return stored
			}
			return incoming
		}
		cur.ContainersOffsite = keepLocation(v.ContainersOffsite, cur.ContainersOffsite)
		cur.VMsOffsite = keepLocation(v.VMsOffsite, cur.VMsOffsite)
		cur.FlashOffsite = keepLocation(v.FlashOffsite, cur.FlashOffsite)
		cur.ConfigOffsite = keepLocation(v.ConfigOffsite, cur.ConfigOffsite)
		cur.FilesOffsite = keepLocation(v.FilesOffsite, cur.FilesOffsite)
		cur.ContainersOffsiteSchedule = v.ContainersOffsiteSchedule
		cur.VMsOffsiteSchedule = v.VMsOffsiteSchedule
		cur.FlashOffsiteSchedule = v.FlashOffsiteSchedule
		cur.ConfigOffsiteSchedule = v.ConfigOffsiteSchedule
		cur.FilesOffsiteSchedule = v.FilesOffsiteSchedule
		cur.ContainersSchedule = v.ContainersSchedule
		cur.VMsSchedule = v.VMsSchedule
		cur.FlashSchedule = v.FlashSchedule
		cur.ConfigSchedule = v.ConfigSchedule
		cur.FilesSchedule = v.FilesSchedule
		cur.FlashZipExportEnabled = v.FlashZipExportEnabled
		cur.FlashZipExportPath = v.FlashZipExportPath
		cur.FlashZipExportKeep = max(0, v.FlashZipExportKeep)
		cur.DefaultLanguage = v.DefaultLanguage
		cur.RetentionKeepLast = max(0, v.RetentionKeepLast)
		cur.RetentionKeepDaily = max(0, v.RetentionKeepDaily)
		cur.RetentionKeepWeekly = max(0, v.RetentionKeepWeekly)
		cur.RetentionKeepMonthly = max(0, v.RetentionKeepMonthly)
		cur.OffsiteRetentionKeepLast = max(0, v.OffsiteRetentionKeepLast)
		cur.OffsiteRetentionKeepDaily = max(0, v.OffsiteRetentionKeepDaily)
		cur.OffsiteRetentionKeepWeekly = max(0, v.OffsiteRetentionKeepWeekly)
		cur.OffsiteRetentionKeepMonthly = max(0, v.OffsiteRetentionKeepMonthly)
		cur.OffsiteLimitUpload = max(0, v.OffsiteLimitUpload)
		cur.OffsiteLimitDownload = max(0, v.OffsiteLimitDownload)
		// A number above the machine's thread count caps nothing. 0 means every
		// core.
		cur.BackupCores = min(max(0, v.BackupCores), runtime.NumCPU())
		cur.MetricsEnabled = v.MetricsEnabled
		cur.DrillsEnabled = v.DrillsEnabled
		cur.DrillsSchedule = v.DrillsSchedule
		cur.DrillsSubsetPct = max(1, min(100, v.DrillsSubsetPct))
		cur.OffsiteDrillsEnabled = v.OffsiteDrillsEnabled
		cur.RecoveryKitAck = v.RecoveryKitAck
		cur.ContainersOffsiteImmutable = v.ContainersOffsiteImmutable
		cur.VMsOffsiteImmutable = v.VMsOffsiteImmutable
		cur.FlashOffsiteImmutable = v.FlashOffsiteImmutable
		cur.ConfigOffsiteImmutable = v.ConfigOffsiteImmutable
		cur.FilesOffsiteImmutable = v.FilesOffsiteImmutable
		cur.OffsiteGrowthBudgetGB = max(0, v.OffsiteGrowthBudgetGB)
		cur.TamperTestSchedule = v.TamperTestSchedule
		cur.DRDrillTarget = strings.TrimSpace(v.DRDrillTarget)
		cur.DRDrillTargetVM = strings.TrimSpace(v.DRDrillTargetVM)
		cur.PruneImageAfterUpdate = v.PruneImageAfterUpdate
		cur.ResticCacheMaxMB = max(0, v.ResticCacheMaxMB)
		cur.DigestEnabled = v.DigestEnabled
		cur.DigestSchedule = v.DigestSchedule
		cur.CatchUpMissed = v.CatchUpMissed
		cur.WatchdogEnabled = v.WatchdogEnabled
		cur.ReconcileUnraidUpdateStatus = v.ReconcileUnraidUpdateStatus
		cur.ExportEncryptEnabled = v.ExportEncryptEnabled
		cur.ExportAgeRecipients = strings.TrimSpace(v.ExportAgeRecipients)
		cur.ReceiverEnabled = v.ReceiverEnabled
		cur.RestartHealthWait = v.RestartHealthWait
		cur.RestartHealthTimeoutSec = clampHealthTimeoutSec(v.RestartHealthTimeoutSec)
		cur.PerItemSchedules = v.PerItemSchedules
		cur.FleetEnabled = v.FleetEnabled
		cur.PullEnabled = v.PullEnabled
		cur.InstanceName = strings.TrimSpace(v.InstanceName)
		cur.EverythingSchedule = v.EverythingSchedule
		// Blank keeps the stored command, like the tokens below: GET never
		// echoes a hook, so every card submits blanks, and removing one needs
		// the ...Clear flag.
		switch {
		case v.EverythingPreHookClear:
			cur.EverythingPreHook = ""
		case strings.TrimSpace(v.EverythingPreHook) != "":
			cur.EverythingPreHook = strings.TrimSpace(v.EverythingPreHook)
		}
		switch {
		case v.EverythingPostHookClear:
			cur.EverythingPostHook = ""
		case strings.TrimSpace(v.EverythingPostHook) != "":
			cur.EverythingPostHook = strings.TrimSpace(v.EverythingPostHook)
		}

		// Blank keeps the stored token. It is read inside the transaction, so a
		// token minted by POST /api/{widget,fleet}/token while the form was open
		// is kept, not reverted.
		if t := strings.TrimSpace(v.MetricsToken); t != "" {
			cur.MetricsToken = t
		}
		if t := strings.TrimSpace(v.WidgetToken); t != "" {
			cur.WidgetToken = t
		}
		if t := strings.TrimSpace(v.FleetToken); t != "" {
			cur.FleetToken = t
		}
		// nil (an old client) keeps the stored registry list. A present list
		// replaces it, a blank token taking the stored one for its host, read
		// here for the same reason as the tokens above. Decoding and encoding
		// touch no store, so both are safe inside the transaction.
		if v.RegistryAuths != nil {
			stored, dErr := h.svc.decodeRegistryAuths(*cur)
			if dErr != nil {
				return dErr
			}
			merged, mErr := mergeRegistryAuths(v.RegistryAuths, stored)
			if mErr != nil {
				registryInputErr = mErr
				return mErr
			}
			blob, eErr := h.svc.EncodeRegistryAuths(merged)
			if eErr != nil {
				return eErr
			}
			cur.RegistryAuths = blob
		}
		return nil
	})
	if registryInputErr != nil {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": registryInputErr.Error()})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	// The replication path reads each domain's primary offsite_targets row, so
	// the saved off-site config is mirrored there; settings stay the source for
	// the fallback path.
	h.svc.syncAllPrimaryOffsiteTargets(s)
	// The CPU cap reaches restic through the environment of the next child it
	// starts, so it applies without a restart; a running backup keeps its value.
	restic.SetMaxProcs(s.BackupCores)
	if err := h.scheduler.ReloadWithDueChecks(s, h.containersLastRun, h.vmsLastRun, h.flashLastRun, h.configLastRun, h.filesLastRun, h.everythingLastRun); err != nil {
		// The settings are saved, but the scheduler could not re-register.
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": scrubError(err)})
		return
	}
	// An immutable off-site repo with an off-site retention policy gets a note,
	// not a failure: BombVault never prunes an append-only repo, so the policy
	// does nothing until the far side enforces it.
	notes := []string{}
	if (s.ContainersOffsiteImmutable || s.VMsOffsiteImmutable || s.FlashOffsiteImmutable || s.ConfigOffsiteImmutable || s.FilesOffsiteImmutable) &&
		(s.OffsiteRetentionKeepLast > 0 || s.OffsiteRetentionKeepDaily > 0 ||
			s.OffsiteRetentionKeepWeekly > 0 || s.OffsiteRetentionKeepMonthly > 0) {
		notes = append(notes, "The off-site repo is append-only (immutable), so BombVault will not apply the off-site retention policy; enforce retention far-side (e.g. a rest-server prune cron) or use a maintenance window.")
	}
	warnings := []saveWarning{}
	after := h.svc.fieldTargets()
	for _, d := range offsiteConfigDomains {
		b, hadRow := before[d]
		a, hasRow := after[d]
		if hadRow && hasRow {
			warnings = append(warnings, h.svc.targetSaveWarnings(r.Context(), b, a)...)
		}
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"warnings": warnings, "notes": notes}))
}

// handleDetectEncryption probes the configured repositories and reports which
// encryption mode they are in, applying a definite result to
// Settings.EncryptionEnabled so a restore on a fresh instance need not assert
// it. POST /api/encryption/detect
//
// "encrypted" and "plain" are applied; "conflict", "absent", "unknown" and
// "unconfigured" leave the setting alone and show as undecided. A failed probe
// is never reported as "plain". The service has already scrubbed each repo's
// Err.
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
// GET /api/recovery-kit
//
// Besides authGate it requires a login password to be set: the kit holds the
// APP_KEY, the derived restic password and the off-site credentials, and
// decrypts every repo for good, including the append-only off-site archives
// meant to survive a host compromise. The body carries the real repo
// locations, unscrubbed, and is never logged.
func (h *Handler) handleRecoveryKit(w http.ResponseWriter, _ *http.Request) {
	if !h.requireAuthForSecrets(w, "downloading the recovery kit") {
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
// dismisses the dashboard nag. It changes that one flag through
// MutateSettings, so a settings change made elsewhere meanwhile survives.
// POST /api/recovery-kit/ack
func (h *Handler) handleRecoveryKitAck(w http.ResponseWriter, _ *http.Request) {
	if _, err := h.store.MutateSettings(func(s *store.Settings) error {
		s.RecoveryKitAck = true
		return nil
	}); err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
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
	skipped, err := h.svc.UnlockDomain(r.Context(), domain, sourceParam(r))
	if err != nil {
		// The skip list goes along on both paths. A repository that only got the
		// stale-lock clear is a note, never the error, so the operator needs it
		// next to whatever did fail.
		body := failEnvelope(err)
		body["skipped"] = skipped
		writeJSON(w, http.StatusOK, body)
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"skipped": skipped}))
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

// handleReplicateOffsite starts an on-demand replication of a domain's local
// repo to its off-site repo (restic copy) and returns immediately, because a
// long first replication outlives a browser or proxy timeout. Config errors
// and a busy domain still report synchronously. POST /api/offsite/{domain}
func (h *Handler) handleReplicateOffsite(w http.ResponseWriter, r *http.Request) {
	domain := r.PathValue("domain")
	switch domain {
	case "containers", "vms", "flash", "config", "files":
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
	case "containers", "vms", "flash", "config", "files":
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
// credentials). Nothing is stored on the server, so the plaintext password is
// shown once. GET /api/offsite/{domain}/deploy-snippet
func (h *Handler) handleDeploySnippet(w http.ResponseWriter, r *http.Request) {
	domain := r.PathValue("domain")
	switch domain {
	case "containers", "vms", "flash", "config", "files":
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
	case "containers", "vms", "flash", "config", "files":
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

// handleGetNotify returns the notification config without the stored
// credentials: the SMTP password and Matrix access token are blanked and
// reported through "is-set" flags, like the cloud credentials. GET /api/notify
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

// decodeNotifyBody decodes a POSTed notify.Config, refusing unknown fields like
// every other body. notify.Config has its own UnmarshalJSON, which
// DisallowUnknownFields cannot see into, so the raw body goes to
// notify.Config.DecodeStrict instead.
func decodeNotifyBody(w http.ResponseWriter, r *http.Request, c *notify.Config) bool {
	var raw json.RawMessage
	if !decodeBody(w, r, &raw) {
		return false
	}
	if err := c.DecodeStrict(raw); err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": "invalid request body"})
		return false
	}
	return true
}

// fillNotifySecrets fills blank credential fields from the stored config. An
// unchanged form submits them blank, since handleGetNotify never sends them.
func (h *Handler) fillNotifySecrets(c notify.Config) (notify.Config, error) {
	if c.SMTPPassword != "" && c.MatrixToken != "" {
		return c, nil
	}
	cur, err := h.svc.NotifyConfig()
	if err != nil {
		return c, nil
	}
	// A stored secret is refilled only for the destination it was stored for.
	// Otherwise POST /api/notify/test with a homeserver of the caller's choice
	// and a blank token would send the stored Matrix token there. A changed
	// destination is refused rather than blanked, which would silently stop
	// notifications while the setup reported success.
	if c.MatrixToken == "" && cur.MatrixToken != "" {
		if !sameMatrixTarget(c, cur) {
			return c, errors.New("enter the Matrix access token again: the stored one belongs to the previous homeserver")
		}
		c.MatrixToken = cur.MatrixToken
	}
	if c.SMTPPassword == "" && cur.SMTPPassword != "" {
		if !sameSMTPTarget(c, cur) {
			return c, errors.New("enter the SMTP password again: the stored one belongs to the previous server")
		}
		c.SMTPPassword = cur.SMTPPassword
	}
	return c, nil
}

// sameMatrixTarget reports whether a request names the same Matrix destination
// the stored token was saved for. The token is sent to the homeserver and
// grants access to the room, so a changed room is a changed destination too.
func sameMatrixTarget(req, cur notify.Config) bool {
	return req.MatrixHomeserver == cur.MatrixHomeserver && req.MatrixRoom == cur.MatrixRoom
}

// sameSMTPTarget is the SMTP counterpart: host, port and username together
// identify the account the stored password belongs to.
func sameSMTPTarget(req, cur notify.Config) bool {
	return req.SMTPHost == cur.SMTPHost && req.SMTPPort == cur.SMTPPort && req.SMTPUsername == cur.SMTPUsername
}

// handleSetNotify stores the notification config (encrypted). A blank SMTP password
// or Matrix token keeps the stored one. POST /api/notify
func (h *Handler) handleSetNotify(w http.ResponseWriter, r *http.Request) {
	var c notify.Config
	if !decodeNotifyBody(w, r, &c) {
		return
	}
	filled, err := h.fillNotifySecrets(c)
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	if err := h.svc.SetNotifyConfig(filled); err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(nil))
}

// handleGetCloud returns the cloud backend credentials without the secrets:
// the other fields plus "is-set" flags. GET /api/cloud
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
// field keeps the stored one. POST /api/cloud
func (h *Handler) handleSetCloud(w http.ResponseWriter, r *http.Request) {
	var c CloudCreds
	if !decodeBody(w, r, &c) {
		return
	}
	before, err := h.store.GetSettings()
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	if err := h.svc.SetCloudCreds(c); err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	// The S3 storage class is part of the cloud creds, and replication reads it
	// from each domain's primary off-site target. A failed read skips the sync.
	if settings, sErr := h.store.GetSettings(); sErr == nil {
		h.svc.syncAllPrimaryOffsiteTargets(settings)
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{
		"warnings": h.svc.directCredsWarnings(r.Context(), before, func(direct, target store.OffsiteTarget) bool {
			return direct.CredsRef == "" || target.CredsRef == ""
		}),
	}))
}

// handleGetCloudCredSets returns the additional named credential sets without
// secrets, with the same is-set flags as handleGetCloud.
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
// credential sets. A blank secret field on a set whose id matches a stored set
// keeps the stored secret, as in handleSetCloud. POST /api/cloud/creds-sets
func (h *Handler) handleSetCloudCredSets(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Sets []CloudCredSet `json:"sets"`
	}
	if !decodeBody(w, r, &body) {
		return
	}
	before, err := h.store.GetSettings()
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	if err := h.svc.SetCloudCredSets(body.Sets); err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{
		"warnings": h.svc.directCredsWarnings(r.Context(), before, func(direct, target store.OffsiteTarget) bool {
			return direct.CredsRef != "" || target.CredsRef != ""
		}),
	}))
}

// handleTestNotify sends a test notification using the POSTed config (so the
// user can test the form before saving). POST /api/notify/test
func (h *Handler) handleTestNotify(w http.ResponseWriter, r *http.Request) {
	var c notify.Config
	if !decodeNotifyBody(w, r, &c) {
		return
	}
	filled, err := h.fillNotifySecrets(c)
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	if err := h.svc.TestNotify(r.Context(), filled); err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(nil))
}

// handleReleaseNotes serves the embedded release notes for the "What's new"
// dialog, since the app's CSP (connect-src 'self') blocks api.github.com.
// GET /api/release-notes?version=vX.Y.Z, defaulting to the running build.
// ok is false when no notes are bundled, so the dialog shows its GitHub link.
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
// is ready when the dashboard loads.
func (h *Handler) WarmSpike() { _, _ = h.runSpikeAndCache() }

// handleSpikeFresh re-runs the probes for the dashboard's "Host Integration
// Check" button and refreshes the cache. POST /api/spike
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

// runView adds the target's name and domain to a stored Run, so the run history
// shows which container, VM or flash backup a run was for.
type runView struct {
	store.Run
	Target string `json:"target"`
	Domain string `json:"domain"` // "container" | "vm" | "flash" | "config" | "files" | "everything" | ""
}

// runTargetMaps resolves target_id → (human name, domain) across every domain,
// for enriching stored runs (handleRuns + the widget feed). Best-effort: an
// unknown id (e.g. a deleted target) simply stays absent, so lookups yield "".
func (h *Handler) runTargetMaps() (name, domain map[string]string) {
	name = map[string]string{
		store.FlashTargetID:      "Unraid flash",
		store.ConfigTargetID:     "App configuration",
		store.EverythingTargetID: "Backup Everything",
	}
	domain = map[string]string{
		store.FlashTargetID:      "flash",
		store.ConfigTargetID:     "config",
		store.EverythingTargetID: "everything",
	}
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
// can dismiss them from the failure count. POST /api/runs/ack with body
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
	settings, err := h.store.GetSettings()
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	domains, err := h.svc.domainStatusFrom(settings)
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	if domains == nil {
		domains = []DomainStatusEntry{}
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"domains": domains}))
}

// handleScheduleNext returns the next fire time of every registered schedule
// entry, soonest first, for the activity log's "up next" line.
// GET /api/schedule/next. Tests build a Handler without a scheduler, which
// yields an empty list.
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
// backup-health heatmap. GET /api/history?days=90; days defaults to 90 and is
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
// size and dedup trend. GET /api/stats?domain=&source=&limit=, where domain is
// containers, vms, flash or files, source is local (default) or offsite, and
// limit defaults to 90, clamped to 1..365. The answer carries the samples in
// ascending order, the latest one (or null) for the headline figure, and a
// "forecast" of growth, free space and time to full (see StorageForecast), or
// null when nothing could be determined.
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
		// Without a sample, a detached and throttled collection fills the Storage
		// card in on the next load instead of leaving it on "no data".
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

// maxBrowseEntries caps one browse listing, because an appdata tree can hold
// tens of thousands of entries and the SPA would parse megabytes of JSON to draw
// one level. The first entries in sorted order are kept, and the "truncated"
// flag tells the tree there are more.
const maxBrowseEntries = 500

const (
	sessionCookieName = "bv_session"
	sessionTTL        = 7 * 24 * time.Hour // 7 days
)

// authEnabled reads the stored password hash and session epoch and reports
// whether authentication is enabled. A store error counts as off, so a passing
// database error does not lock everyone out of a trusted-LAN tool.
func (h *Handler) authEnabled() (hash, epoch string, on bool) {
	s, err := h.store.GetSettings()
	if err != nil {
		log.Printf("api: authEnabled: GetSettings: %v", err)
		return "", "", false
	}
	return s.AuthPasswordHash, s.SessionEpoch, s.AuthPasswordHash != ""
}

// requireAuthForSecrets reports whether a handler that hands out stored secrets
// in the clear may proceed, answering 403 itself when it may not. Without a
// login password authGate lets everything through, which is acceptable for
// current data but not for keys that decrypt every repository, including the
// append-only off-site archives, so these require a password. A store error
// refuses too. action names what is refused, such as "downloading the
// recovery kit".
func (h *Handler) requireAuthForSecrets(w http.ResponseWriter, action string) bool {
	if _, _, on := h.authEnabled(); on {
		return true
	}
	writeJSON(w, http.StatusForbidden, map[string]any{
		"ok":    false,
		"error": "set a login password before " + action,
	})
	return false
}

// newSessionCookie builds the bv_session cookie. Secure is off only in HTTP-only
// mode, for LAN installs without TLS.
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

// handleAuthStatus handles GET /api/auth, telling the SPA whether to show the
// login screen and what the settings page should say about the account. The
// route is public, so second-factor details such as the recovery codes left
// go only to a caller who is signed in.
func (h *Handler) handleAuthStatus(w http.ResponseWriter, r *http.Request) {
	hash, epoch, on := h.authEnabled()
	authed := false
	if on {
		if c, err := r.Cookie(sessionCookieName); err == nil {
			authed = secret.ValidSessionToken(h.cfg.AppKey, hash, epoch, c.Value)
		}
	}
	out := map[string]any{
		"ok":      true,
		"enabled": on,
		"authed":  authed,
		// Told to everyone, because the login screen has to know whether to ask
		// for a code, and it asks before anybody is signed in. It reveals only
		// that this instance is harder to get into.
		"totp": false,
		// The rule the password field enforces, so the frontend can say the
		// number rather than hard-code a second copy of it.
		"minPasswordLen": secret.MinPasswordLen,
	}
	if on {
		if s, err := h.store.GetSettings(); err == nil {
			out["totp"] = s.TOTPEnabled
			if authed || !on {
				out["recoveryCodesLeft"] = len(decodeRecoveryCodes(s.TOTPRecovery))
				out["passwordNeedsUpgrade"] = secret.NeedsRehash(hash)
			}
		}
	}
	writeJSON(w, http.StatusOK, out)
}

// A client is locked out after loginMaxFails failed logins within loginWindow.
// The throttle is checked before the password is hashed, so a locked-out
// client cannot learn from a correct guess either; it gets a 429 even with the
// right password.
const (
	loginMaxFails = 5
	loginWindow   = time.Minute
)

// passwordHashSlots bounds how many Argon2id verifications run at once. Each
// costs about 19 MiB, and the login throttle is per client, so an attacker
// rotating source addresses could otherwise push a memory-capped container into
// the OOM killer. Two slots cap hashing at about 38 MiB; further logins wait.
var passwordHashSlots = make(chan struct{}, 2)

// verifyPassword is secret.VerifyPassword behind that cap.
func verifyPassword(appKey, password, storedHash string) bool {
	passwordHashSlots <- struct{}{}
	defer func() { <-passwordHashSlots }()
	return secret.VerifyPassword(appKey, password, storedHash)
}

// loginClientKey returns the throttle key for r: the TCP peer's IP without its
// port. It never reads X-Forwarded-For, which a caller could set to pick a fresh
// bucket per request; behind a reverse proxy all clients share the proxy's
// bucket unless TRUSTED_PROXY is set (see (*Handler).loginClientKey).
func loginClientKey(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		// The raw value, so callers without a port (tests set RemoteAddr
		// directly) do not all share one "" bucket.
		return r.RemoteAddr
	}
	return host
}

// loginClientKey is loginClientKey plus the TRUSTED_PROXY step: when the peer is
// a configured trusted proxy, the throttle keys on the client that proxy names
// rather than on the proxy itself.
func (h *Handler) loginClientKey(r *http.Request) string {
	peer := loginClientKey(r)
	if len(h.cfg.TrustedProxies) == 0 {
		return peer
	}
	ip := net.ParseIP(peer)
	if ip == nil || !trusted(h.cfg.TrustedProxies, ip) {
		// Someone other than the proxy connects directly, so their header is
		// ignored, or naming a trusted proxy would let anyone spoof the key.
		return peer
	}
	if fwd := forwardedClient(r.Header.Get("X-Forwarded-For"), h.cfg.TrustedProxies); fwd != "" {
		return fwd
	}
	return peer
}

// forwardedClient returns the client address from an X-Forwarded-For chain,
// reading right to left and stopping at the first entry that is not a trusted
// proxy. Each hop appends to the header, so the left end is whatever the caller
// sent and only the right end was written by hops we trust.
func forwardedClient(header string, proxies []net.IPNet) string {
	parts := strings.Split(header, ",")
	for i := len(parts) - 1; i >= 0; i-- {
		entry := strings.TrimSpace(parts[i])
		if entry == "" {
			continue
		}
		// A chain entry may carry a port (rare, but legal for IPv6 forms).
		if host, _, err := net.SplitHostPort(entry); err == nil {
			entry = host
		}
		ip := net.ParseIP(strings.Trim(entry, "[]"))
		if ip == nil {
			// Refuse the whole header, so an unparseable entry cannot steer
			// which entry is used.
			return ""
		}
		if trusted(proxies, ip) {
			continue
		}
		return ip.String()
	}
	return ""
}

func trusted(proxies []net.IPNet, ip net.IP) bool {
	for _, n := range proxies {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}

// loginSweepEvery is how many throttle checks pass between sweeps of the whole
// failure map. loginThrottled prunes only the key it is asked about, so keys
// that fail once and never return, such as an attacker rotating through an
// IPv6 /64, would otherwise stay forever.
const loginSweepEvery = 256

// loginMaxTracked caps the keys loginFails holds, for a burst of one-off keys
// that arrives faster than the periodic sweep.
const loginMaxTracked = 10_000

// loginThrottled prunes key's failure window and reports whether logins from it
// are locked out. It also sweeps the whole map now and then and caps its size,
// so keys queried only once cannot pile up.
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

// sweepLoginFailsLocked prunes every key's window once every loginSweepEvery
// calls, or at once when the map has grown past loginMaxTracked, and evicts
// when a full prune still leaves it over the cap. The caller holds loginMu.
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

// evictLeastRecentlyTouchedLocked deletes entries from h.loginFails, the one
// with the oldest latest failure first, until it is back under loginMaxTracked.
// The caller holds loginMu.
//
// A throttled key is never evicted. Its timestamps stop advancing once it is
// locked out, because handleLogin checks the throttle before recording a
// failure, so by age alone a flood of fresh one-off keys would evict it and
// lift its lockout. Throttled keys may sit over the cap, but only callers that
// reach loginMaxFails count, which is a small population.
func (h *Handler) evictLeastRecentlyTouchedLocked() {
	type keyAge struct {
		key  string
		last time.Time
	}
	ages := make([]keyAge, 0, len(h.loginFails))
	for k, fails := range h.loginFails {
		if len(fails) >= loginMaxFails {
			continue // throttled, see above
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

// handleLogin handles POST /api/login.
func (h *Handler) handleLogin(w http.ResponseWriter, r *http.Request) {
	hash, epoch, on := h.authEnabled()
	if !on {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": "authentication is not enabled"})
		return
	}
	key := h.loginClientKey(r)
	if h.loginThrottled(key) {
		writeJSON(w, http.StatusTooManyRequests, map[string]any{"ok": false, "error": "too many failed attempts; wait a minute and try again"})
		return
	}

	var body struct {
		Password string `json:"password"`
		// Code is the authenticator app's six digits, or a recovery code, and
		// is only read when the second factor is switched on.
		Code string `json:"code"`
	}
	if !decodeBody(w, r, &body) {
		return
	}

	if !verifyPassword(h.cfg.AppKey, body.Password, hash) {
		h.recordLoginFail(key)
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": "invalid password"})
		return
	}

	// With a second factor on, nothing is granted until it passes too, not even
	// the throttle reset, so someone with the password still gets five tries a
	// minute at the code.
	s, err := h.store.GetSettings()
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	if s.TOTPEnabled {
		if strings.TrimSpace(body.Code) == "" {
			// Not a failure: the client now knows to show the code field.
			writeJSON(w, http.StatusOK, map[string]any{
				"ok":       false,
				"needCode": true,
				"error":    "enter the code from your authenticator app",
			})
			return
		}
		if !h.secondFactorOK(&s, body.Code) {
			h.recordLoginFail(key)
			writeJSON(w, http.StatusOK, map[string]any{
				"ok":       false,
				"needCode": true,
				"error":    "that code is not valid",
			})
			return
		}
	}
	h.recordLoginSuccess(key)

	// A legacy password hash can only be upgraded while the verified plaintext
	// is at hand. It happens before the token is minted, because the session
	// HMAC signs the stored hash and the rehash would invalidate a token issued
	// against the old one.
	if secret.NeedsRehash(hash) {
		if fresh, hErr := secret.HashPassword(h.cfg.AppKey, body.Password); hErr != nil {
			log.Printf("api: login: rehash: %v", hErr)
		} else if _, mErr := h.store.MutateSettings(func(st *store.Settings) error {
			// Re-check inside the mutation: a parallel password change between
			// the verify and here must not be overwritten with the old one.
			if st.AuthPasswordHash == hash {
				st.AuthPasswordHash = fresh
			}
			return nil
		}); mErr != nil {
			// Not fatal: the hash stays in the old format until the next login.
			log.Printf("api: login: storing upgraded password hash: %v", mErr)
		} else {
			hash = fresh
		}
	}

	tok := secret.NewSessionToken(h.cfg.AppKey, hash, epoch, sessionTTL)
	http.SetCookie(w, h.newSessionCookie(tok, int(sessionTTL.Seconds())))
	writeJSON(w, http.StatusOK, okEnvelope(nil))
}

// secondFactorOK checks code against the time-based secret and, failing that,
// against the single-use recovery codes. A matching recovery code is removed
// from the stored list before this returns true.
func (h *Handler) secondFactorOK(s *store.Settings, code string) bool {
	if sec, err := h.decryptTOTPSecret(s.TOTPSecret); err == nil && sec != "" {
		if secret.ValidTOTP(sec, code, time.Now()) {
			return true
		}
	}
	stored := decodeRecoveryCodes(s.TOTPRecovery)
	idx := secret.MatchRecoveryCode(h.cfg.AppKey, code, stored)
	if idx < 0 {
		return false
	}
	remaining := append(append([]string{}, stored[:idx]...), stored[idx+1:]...)
	if _, err := h.store.MutateSettings(func(st *store.Settings) error {
		st.TOTPRecovery = encodeRecoveryCodes(remaining)
		return nil
	}); err != nil {
		// The code was correct, but it could not be burned. Refuse the login:
		// a recovery code that survives its own use is a permanent password.
		log.Printf("api: login: spending recovery code: %v", err)
		return false
	}
	log.Printf("api: login: a recovery code was used, %d left", len(remaining))
	return true
}

func decodeRecoveryCodes(raw string) []string {
	if raw == "" {
		return nil
	}
	var out []string
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		log.Printf("api: recovery codes: unreadable, treating as none: %v", err)
		return nil
	}
	return out
}

func encodeRecoveryCodes(codes []string) string {
	if len(codes) == 0 {
		// Empty string, not "[]": the column's zero value and "none left" are
		// the same state, and only one of them should exist in the database.
		return ""
	}
	b, err := json.Marshal(codes)
	if err != nil {
		return ""
	}
	return string(b)
}

// decryptTOTPSecret opens the stored (hex-encoded, APP_KEY-encrypted) secret.
func (h *Handler) decryptTOTPSecret(stored string) (string, error) {
	if stored == "" {
		return "", nil
	}
	raw, err := hex.DecodeString(stored)
	if err != nil {
		return "", fmt.Errorf("totp secret: %w", err)
	}
	plain, err := secret.Decrypt(h.cfg.AppKey, raw)
	if err != nil {
		return "", fmt.Errorf("totp secret: %w", err)
	}
	return string(plain), nil
}

func (h *Handler) encryptTOTPSecret(plain string) (string, error) {
	sealed, err := secret.Encrypt(h.cfg.AppKey, []byte(plain))
	if err != nil {
		return "", fmt.Errorf("totp secret: %w", err)
	}
	return hex.EncodeToString(sealed), nil
}

// handleLogout handles POST /api/logout by clearing the session cookie. The
// stateless token stays valid until it expires, so a copied cookie would still
// work; handleLogoutAll is the revocation.
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

// handleLogoutAll handles POST /api/logout-all ("log out everywhere"). It
// rotates the session epoch that every token's HMAC is bound to, so every
// outstanding cookie becomes invalid at once, and clears the caller's own.
func (h *Handler) handleLogoutAll(w http.ResponseWriter, _ *http.Request) {
	epoch, err := newSessionEpoch()
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	if _, err := h.store.MutateSettings(func(s *store.Settings) error {
		s.SessionEpoch = epoch
		return nil
	}); err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	http.SetCookie(w, h.newSessionCookie("", -1))
	writeJSON(w, http.StatusOK, okEnvelope(nil))
}

// handleSetPassword handles POST /api/auth/password with body {password}; an
// empty password turns the login off.
func (h *Handler) handleSetPassword(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Password string `json:"password"`
	}
	if !decodeBody(w, r, &body) {
		return
	}

	hash := ""
	if body.Password != "" {
		// The minimum applies only when a password is set, so an older, shorter
		// password still verifies and its owner is asked to change it. Runes,
		// not bytes, so a passphrase in a multi-byte script is not favoured.
		if utf8.RuneCountInString(body.Password) < secret.MinPasswordLen {
			writeJSON(w, http.StatusOK, map[string]any{
				"ok":     false,
				"error":  fmt.Sprintf("the password must be at least %d characters", secret.MinPasswordLen),
				"minLen": secret.MinPasswordLen,
			})
			return
		}
		var err error
		if hash, err = secret.HashPassword(h.cfg.AppKey, body.Password); err != nil {
			writeJSON(w, http.StatusOK, failEnvelope(err))
			return
		}
	}
	epoch := ""
	if _, err := h.store.MutateSettings(func(s *store.Settings) error {
		s.AuthPasswordHash = hash
		if hash == "" {
			// Switching the login off takes the second factor with it, so a
			// login switched on again later does not ask for a code from an app
			// the operator may have deleted long ago.
			s.TOTPEnabled = false
			s.TOTPSecret = ""
			s.TOTPRecovery = ""
		}
		// Setting a password ends every other session, because a password that
		// may have leaked is worth nothing while sessions minted under it stay
		// alive. The caller keeps working: the cookie below is signed with the
		// new epoch.
		next, err := newSessionEpoch()
		if err != nil {
			return err
		}
		s.SessionEpoch = next
		epoch = next
		return nil
	}); err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}

	// Setting a password signs the operator in, or every later request would
	// get a 401 until the page is reloaded and the password typed again. It
	// grants nothing new: without a login this route was already open to the
	// caller, and with one authGate guards it. The token signs the new hash,
	// so it is minted after the write; clearing the password clears the cookie.
	if hash == "" {
		http.SetCookie(w, h.newSessionCookie("", -1))
	} else {
		tok := secret.NewSessionToken(h.cfg.AppKey, hash, epoch, sessionTTL)
		http.SetCookie(w, h.newSessionCookie(tok, int(sessionTTL.Seconds())))
	}

	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{
		"enabled": hash != "",
		// The caller is signed in as of this answer, so the Security card can
		// show the sign-out controls without a round trip or a reload.
		"authed": hash != "",
	}))
}

// handleTOTPSetup handles POST /api/auth/totp/setup: it mints a fresh secret,
// stores it encrypted but not yet armed, and returns the otpauth URI for the QR
// code. Storing it lets the confirm step check against the server's copy, and
// TOTPEnabled stays false until a working code arrives, so an enrolment
// abandoned halfway leaves the login as it was.
func (h *Handler) handleTOTPSetup(w http.ResponseWriter, r *http.Request) {
	if !h.requireAuthForSecrets(w, "setting up two-factor authentication") {
		return
	}
	s, err := h.store.GetSettings()
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	if s.AuthPasswordHash == "" {
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":    false,
			"error": "set a login password first: a second factor with no first one protects nothing",
		})
		return
	}
	if s.TOTPEnabled {
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":    false,
			"error": "two-factor authentication is already on; turn it off before setting it up again",
		})
		return
	}

	plain, err := secret.NewTOTPSecret()
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	sealed, err := h.encryptTOTPSecret(plain)
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	if _, err := h.store.MutateSettings(func(st *store.Settings) error {
		st.TOTPSecret = sealed
		st.TOTPEnabled = false
		return nil
	}); err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}

	name := s.InstanceName
	if name == "" {
		name = "BombVault"
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{
		"secret": plain,
		"uri":    secret.TOTPURI("BombVault", name, plain),
	}))
}

// handleTOTPConfirm handles POST /api/auth/totp/confirm: it arms the second
// factor once the operator types a code the stored secret produces, and returns
// the recovery codes. They are stored hashed, so this answer is the only time
// they are shown.
func (h *Handler) handleTOTPConfirm(w http.ResponseWriter, r *http.Request) {
	if !h.requireAuthForSecrets(w, "setting up two-factor authentication") {
		return
	}
	var body struct {
		Code string `json:"code"`
	}
	if !decodeBody(w, r, &body) {
		return
	}
	s, err := h.store.GetSettings()
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	plain, err := h.decryptTOTPSecret(s.TOTPSecret)
	if err != nil || plain == "" {
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":    false,
			"error": "no pending setup; start again",
		})
		return
	}
	if !secret.ValidTOTP(plain, body.Code, time.Now()) {
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":    false,
			"error": "that code is not valid; check the clock on your phone and try the next one",
		})
		return
	}

	codes, hashed, err := secret.NewRecoveryCodes(h.cfg.AppKey)
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	if _, err := h.store.MutateSettings(func(st *store.Settings) error {
		st.TOTPEnabled = true
		st.TOTPRecovery = encodeRecoveryCodes(hashed)
		return nil
	}); err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{
		"recoveryCodes": codes,
	}))
}

// handleTOTPDisable handles POST /api/auth/totp/disable. It requires a current
// code or a recovery code, so an unattended session cannot remove the factor.
func (h *Handler) handleTOTPDisable(w http.ResponseWriter, r *http.Request) {
	if !h.requireAuthForSecrets(w, "changing two-factor authentication") {
		return
	}
	var body struct {
		Code string `json:"code"`
	}
	if !decodeBody(w, r, &body) {
		return
	}
	s, err := h.store.GetSettings()
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	if !s.TOTPEnabled {
		writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"enabled": false}))
		return
	}
	if !h.secondFactorOK(&s, body.Code) {
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":    false,
			"error": "that code is not valid",
		})
		return
	}
	if _, err := h.store.MutateSettings(func(st *store.Settings) error {
		st.TOTPEnabled = false
		st.TOTPSecret = ""
		st.TOTPRecovery = ""
		return nil
	}); err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"enabled": false}))
}

// authGate requires a session cookie once a login password is set, and passes
// everything through while none is. These paths are always open:
//   - GET /api/auth, POST /api/login and GET /api/health, so the SPA can load
//     and sign in.
//   - GET /metrics, GET /widget, GET /api/widget/data and GET
//     /api/fleet/status. Prometheus, an embedding iframe and a polling peer
//     cannot carry the cookie, so each gates itself on its own token and
//     refuses with 403 when none is set. Managing those tokens needs a session.
//   - POST /api/fleet/mesh-offer, the one write on this list, behind the same
//     fleet token. It only stores a pending offer for a person to review.
//   - The passkey status and the two passkey login halves.
func (h *Handler) authGate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// A store error must not drop the gate, so it fails closed, keeping
		// only the public endpoints reachable so the SPA can still recover.
		s, err := h.store.GetSettings()
		if err != nil {
			log.Printf("api: authGate: GetSettings: %v", err)
			switch r.URL.Path {
			case "/api/auth", "/api/login", "/api/health", "/metrics", "/widget", "/api/widget/data", "/api/fleet/status", "/api/fleet/mesh-offer",
				"/api/auth/passkeys", "/api/auth/passkey/login/begin", "/api/auth/passkey/login/finish":
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

		// The public paths from the doc comment above.
		switch r.URL.Path {
		case "/api/auth", "/api/login", "/api/health", "/metrics", "/widget", "/api/widget/data", "/api/fleet/status", "/api/fleet/mesh-offer",
			// The passkey login halves are how someone signs in. The status
			// tells an unauthenticated caller only counts and whether this
			// address can carry a passkey; the key list needs a session.
			"/api/auth/passkeys", "/api/auth/passkey/login/begin", "/api/auth/passkey/login/finish":
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

// handleBackupFlash starts the Unraid USB flash backup on the server and
// returns immediately, like handleBackup. The SPA follows the "flash" progress
// key over SSE.
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

// handleBackupConfig starts the self-backup of BombVault's own /config
// (settings database, rclone.conf, SSH key pair) on the server and returns
// immediately, like handleBackupFlash. The SPA follows the "config" progress
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

// handleRestoreConfig stages a restore of BombVault's own /config and triggers
// a restart, because the live SQLite database cannot be swapped while this
// process holds it open; selfrestore.ApplyPending swaps it in at boot. When
// Docker is unreachable, autoRestart:false tells the SPA to ask for a manual
// restart. Errors go through restoreFail like the other restores.
func (h *Handler) handleRestoreConfig(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Source   string `json:"source"`
		Snapshot string `json:"snapshot"`
	}
	if !decodeBody(w, r, &body) {
		return
	}
	// The source comes in the body here, not as ?source=.
	source := normalizeSource(body.Source)
	started, auto, err := h.svc.StartRestoreConfig(r.Context(), body.Snapshot, source)
	if err != nil {
		restoreFail(w, source, err)
		return
	}
	if !started {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": "a backup or restore is already running"})
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"staged": true, "autoRestart": auto}))
}

// headerOnFirstWrite defers the download headers (and so the 200 status) until
// the first byte is streamed, so a restic failure before any output (bad id,
// repo locked, no backups) is reported as a JSON error instead of a truncated
// 200 zip.
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
// (restic dump), as a GET so it can be a plain link. ?snapshot=<id> selects the
// snapshot; "" or "latest" is the newest.
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
	// With nothing streamed the headers are unsent and the failure can go out
	// as JSON. A failure mid-stream can only truncate the body; its run is
	// recorded.
	if err != nil && !lw.wrote {
		restoreFail(w, sourceParam(r), err)
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

// classifyReadDirError maps a browse read failure to the listing's `status`, so
// the tree can tell a vanished node ("missing") from an unreadable one
// ("restricted") without the path leaving the server. Anything else, an os.Root
// escape included, is plain "error", so an attempted escape looks like any
// other failure.
func classifyReadDirError(err error) string {
	switch {
	case errors.Is(err, fs.ErrPermission):
		return "restricted"
	case errors.Is(err, fs.ErrNotExist):
		return "missing"
	default:
		return "error"
	}
}

// handleBrowse serves GET /api/browse?path=<subpath>[&hidden=1], listing the
// subdirectories of <HostMountRoot>/<subpath> sorted by name and capped at
// maxBrowseEntries. Dot-prefixed entries appear only with hidden=1. An empty
// path lists the mount root.
//
// paths.Resolve rejects a traversal lexically, and os.Root then enforces the
// same boundary at open time, so a symlink inside the mount root cannot list a
// location outside it. Every outcome is an HTTP 200 with a "status"; an empty
// list with status "ok" means the folder is empty.
func (h *Handler) handleBrowse(w http.ResponseWriter, r *http.Request) {
	subpath := r.URL.Query().Get("path")
	// Only the literal "1" opts in, so an empty or mistyped value cannot.
	includeHidden := r.URL.Query().Get("hidden") == "1"

	// paths.Resolve needs a non-empty child, and only its verdict is used;
	// Root.Open below does the resolving.
	if subpath != "" {
		if _, err := paths.Resolve(h.cfg.HostMountRoot, subpath); err != nil {
			// No "status" field here: the FolderBrowser relies on this exact
			// response.
			writeJSON(w, http.StatusOK, map[string]any{
				"ok":    false,
				"error": "invalid path: must be a relative subpath under the mount root",
			})
			return
		}
	}

	// paths.Resolve cannot see a symlink inside the mount root that points
	// outside it; os.Root checks every component at open time.
	root, err := os.OpenRoot(h.cfg.HostMountRoot)
	if err != nil {
		log.Printf("api: browse: OpenRoot: %v", err)
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":     false,
			"error":  "could not read directory",
			"status": classifyReadDirError(err),
		})
		return
	}
	defer root.Close() //nolint:errcheck // read-only browse descriptor: close error is not actionable

	// "." names the root itself.
	rel := subpath
	if rel == "" {
		rel = "."
	}
	f, err := root.Open(rel)
	if err != nil {
		log.Printf("api: browse: open %q: %v", rel, err) //nolint:gosec // G706: rel comes from the query but passed paths.Resolve, and %q escapes control characters
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":     false,
			"error":  "could not read directory",
			"status": classifyReadDirError(err),
		})
		return
	}
	defer f.Close() //nolint:errcheck // read-only browse descriptor: close error is not actionable

	entries, err := f.ReadDir(-1)
	if err != nil {
		log.Printf("api: browse: ReadDir %q: %v", rel, err) //nolint:gosec // G706: rel comes from the query but passed paths.Resolve, and %q escapes control characters
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":     false,
			"error":  "could not read directory",
			"status": classifyReadDirError(err),
		})
		return
	}

	dirs := make([]browseDirEntry, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		if !includeHidden && strings.HasPrefix(name, ".") {
			continue // skip hidden entries unless hidden=1 opted in
		}
		var entryPath string
		if subpath == "" {
			entryPath = name
		} else {
			entryPath = subpath + "/" + name
		}
		dirs = append(dirs, browseDirEntry{Name: name, Path: entryPath})
	}

	// Sorted after filtering, so a capped listing is a stable prefix of the
	// full one. Hidden entries go last: a dot sorts before letters, and they
	// would otherwise push ordinary folders off the end of a capped page.
	sort.Slice(dirs, func(i, j int) bool {
		hi, hj := strings.HasPrefix(dirs[i].Name, "."), strings.HasPrefix(dirs[j].Name, ".")
		if hi != hj {
			return hj
		}
		return dirs[i].Name < dirs[j].Name
	})

	truncated := false
	if len(dirs) > maxBrowseEntries {
		dirs = dirs[:maxBrowseEntries]
		truncated = true
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"ok":        true,
		"root":      h.cfg.HostMountRoot,
		"path":      subpath,
		"dirs":      dirs,
		"status":    "ok",
		"truncated": truncated,
	})
}

// mkdirRequest is the JSON body for POST /api/browse/mkdir.
type mkdirRequest struct {
	Path string `json:"path"` // parent subpath under HostMountRoot ("" = the root)
	Name string `json:"name"` // new folder name, a single plain path component
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
	// One plain path component: no nested path, no traversal, and no leading
	// dot, which the browser would hide again.
	if name == "" || name == "." || name == ".." ||
		strings.ContainsAny(name, "/\\\x00") || strings.HasPrefix(name, ".") {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": "invalid folder name"})
		return
	}
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
	// Mkdir rather than MkdirAll, so an existing folder is reported instead of
	// reused; the Chmod undoes a strict umask.
	if err := os.Mkdir(abs, 0o755); err != nil { //nolint:gosec // G301: a backup destination on a user-visible share must be operator-readable
		if os.IsExist(err) {
			writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": "a folder with that name already exists"})
			return
		}
		log.Printf("api: mkdir %q: %v", abs, err) //nolint:gosec // G706: abs is a Resolve-validated child path; no raw user bytes reach the log formatter
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": "could not create folder"})
		return
	}
	_ = os.Chmod(abs, 0o755) //nolint:gosec // G302: must be readable by the non-root share user
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "path": sub, "name": name})
}

// fileSetIDParam extracts and validates the {id} path value like nameParam.
// Set ids are 32 hex characters, which the container name charset covers.
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
	sets, _ := h.store.ListFileSets()
	byID := make(map[string]store.FileSet, len(sets))
	for _, fs := range sets {
		byID[fs.ID] = fs
	}
	items := make([]placementItem, 0, len(views))
	for _, v := range views {
		fs := byID[v.ID]
		it := placementItem{
			Key: v.ID, Identity: "fileset:" + v.Name,
			Home: store.HomeState{Exists: true, Repo: fs.Repo, Choice: fs.RepoChosen},
		}
		if run, _ := h.store.LastSuccessfulBackup(v.ID); run != nil {
			it.LastSuccess = run.StartedAt
		}
		items = append(items, it)
	}
	placements := h.svc.listPlacements(r.Context(), "files", items)
	for i := range views {
		views[i].Placement = placements[views[i].ID]
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "fileSets": views})
}

// handleCreateFileSet creates a file set. POST /api/files/sets
// body {name, path, excludes, enabled, repo, copies}. Without repo the set is
// open and takes the Folders default at its first backup; with it, the empty
// string for the domain repository included, it stays where it is put.
func (h *Handler) handleCreateFileSet(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name     string        `json:"name"`
		Path     string        `json:"path"`
		Excludes []string      `json:"excludes"`
		Enabled  *bool         `json:"enabled"`
		Repo     *string       `json:"repo"`
		Copies   *copiesChoice `json:"copies"`
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
	if body.Repo != nil {
		fs.Repo = strings.TrimSpace(*body.Repo)
		fs.RepoChosen = store.RepoChosen
	}
	if fs.Path == "" {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": "path is required"})
		return
	}
	// A new set's path counts as changed, so it has to exist.
	if err := h.svc.validateFileSet(fs, true); err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	// A live set already owns this name: say so plainly rather than running
	// the leftover-snapshot check below, which would answer as if the name
	// were free and its history orphaned.
	if _, err := h.store.GetFileSetByName(fs.Name); err == nil {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": "a file set with this name already exists"})
		return
	}
	// A name whose fileset:<name> snapshots are still in the repo (for
	// example after "Remove set", which keeps them) must not be silently
	// adopted by an unrelated new set.
	if err := h.svc.fileSetNameAdoptable(r.Context(), fs.Name, fs.Repo, fs.Path); err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	created, err := h.svc.createFileSet(fs, body.Copies)
	if err != nil {
		// A duplicate name violates the UNIQUE constraint; report it clearly.
		if strings.Contains(err.Error(), "UNIQUE") {
			writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": "a file set with this name already exists"})
			return
		}
		placementFail(w, err, nil)
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"id": created.ID}))
}

// handlePatchFileSet partially updates a file set. PATCH /api/files/sets/{id}
// body {name?, path?, excludes?, enabled?, selectedPaths?}. Pointers keep an
// enabled-only PATCH from resetting the other fields, and the merged set is
// validated again so a patch cannot slip past the create-time checks.
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
		// Kept apart from the set, like its store setter, so a form that does
		// not know about the cadence cannot clear one by leaving it out.
		ScheduleCadence *string `json:"scheduleCadence"`
		// SelectedPaths is the set's tree selection. nil leaves it untouched, so
		// a plain name or path form cannot clear it; [] is refused, not stored.
		SelectedPaths *[]string `json:"selectedPaths"`
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
	item := store.ItemRef{Domain: "files", Key: id}
	if err := h.svc.checkPlacementChange(r.Context(), item, change); err != nil {
		placementFail(w, err, nil)
		return
	}
	fs, err := h.store.GetFileSet(id)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": "file set not found"})
		return
	}
	oldName := fs.Name
	// The stored selection was validated against the old path, which the merge
	// below overwrites.
	oldPath := fs.Path
	wasEnabled := fs.Enabled
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
	// A path change moves the anchor every stored entry was validated against,
	// so it clears the selection in the same save (fileSetPositionals also
	// re-anchors at compile time). The clear wins over entries sent in the same
	// request, since keeping both would rewrite their meaning under the new
	// root. Roots are compared resolved, so re-sending the same path is no
	// change; an unresolvable one counts as changed.
	pathChanged := false
	if body.Path != nil {
		oldResolved, oldErr := paths.Resolve(h.cfg.HostMountRoot, oldPath)
		newResolved, newErr := paths.Resolve(h.cfg.HostMountRoot, fs.Path)
		pathChanged = oldErr != nil || newErr != nil || newResolved != oldResolved
	}
	isEnabling := fs.Enabled && !wasEnabled
	if err := h.svc.validateFileSet(fs, pathChanged || isEnabling); err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	// A rename moves where the set's next backup writes, so it needs the files
	// domain lock: BackupFileSet holds it for its whole run, and a rename
	// landing mid-backup would leave that run's snapshot behind the old name
	// while the set already points at the new one. A home change in the same
	// request brings the same lock along in writeItemPlacement, so it is left
	// to take it there.
	nameChanging := fs.Name != oldName
	if nameChanging && change.Home == nil {
		unlock, ok := h.svc.tryLockDomainFor("files", "rename")
		if !ok {
			op, busy := h.svc.domainBusy("files")
			if !busy {
				op = "another operation"
			}
			writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": fmt.Sprintf("%s is running on files; try the change again once it finishes", op)})
			return
		}
		defer unlock()
	}
	// A set with backups cannot be renamed: its snapshots are tagged
	// fileset:<oldName> and never re-tagged, so a rename would strand them where
	// DeleteBackupsFileSet cannot find them.
	if nameChanging {
		presence, bErr := h.svc.itemBackups(r.Context(), store.ItemRef{Domain: "files", Key: id})
		if bErr != nil && presence != backupsUnreadable {
			writeJSON(w, http.StatusOK, failEnvelope(bErr))
			return
		}
		if presence != backupsNone {
			msg := "cannot rename a file set that already has backups; create a new set instead"
			if presence == backupsUnreadable {
				msg = fmt.Sprintf("cannot rename: %s: %v", errFileSetRepoUnreachable, bErr)
			}
			writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": msg})
			return
		}
		// The new name's fileset:<name> snapshots can still be in the
		// repository, left behind by a set that was removed without them.
		if err := h.svc.fileSetNameAdoptable(r.Context(), fs.Name, fs.Repo, fs.Path); err != nil {
			writeJSON(w, http.StatusOK, failEnvelope(err))
			return
		}
	}
	// On a path change the row and the cleared selection are one statement
	// (UpdateFileSetClearingSelection), so a failure cannot leave the new path
	// live with the old selection still stored.
	if fs.Name != oldName {
		if err := h.svc.moveFileSetRule(oldName, fs.Name); err != nil {
			placementFail(w, err, nil)
			return
		}
	}
	var upErr error
	if pathChanged {
		upErr = h.store.UpdateFileSetClearingSelection(fs)
	} else {
		upErr = h.store.UpdateFileSet(fs)
	}
	if upErr != nil {
		if fs.Name != oldName {
			if err := h.svc.moveFileSetRule(fs.Name, oldName); err != nil {
				log.Printf("api: file set %q: its copy rule stays under the new name after the rename failed: %v", oldName, err) //nolint:gosec // G706: the name is %q-quoted
			}
		}
		if strings.Contains(upErr.Error(), "UNIQUE") {
			writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": "a file set with this name already exists"})
			return
		}
		writeJSON(w, http.StatusOK, failEnvelope(upErr))
		return
	}
	switch {
	case pathChanged:
		// UpdateFileSetClearingSelection already cleared the selection.
	case body.SelectedPaths != nil:
		if err := h.svc.SetFileSetSelectedPaths(r.Context(), id, *body.SelectedPaths); err != nil {
			if errors.Is(err, errFileSetEmptySelection) {
				// The code lets the tree tell "empty" from other failures and
				// keep its local state, since nothing was stored.
				writeJSON(w, http.StatusOK, codedFailEnvelope(err, "empty-selection"))
				return
			}
			writeJSON(w, http.StatusOK, failEnvelope(err))
			return
		}
	}
	if body.ScheduleCadence != nil {
		if err := h.svc.SetFileSetScheduleCadence(r.Context(), id, *body.ScheduleCadence); err != nil {
			writeJSON(w, http.StatusOK, failEnvelope(err))
			return
		}
		// As for VMs: the set's own cron entry comes or goes, and a file-set
		// PATCH does not otherwise reload the scheduler.
		if err := h.reloadScheduler(); err != nil {
			writeJSON(w, http.StatusOK, failEnvelope(err))
			return
		}
	}
	// Written last, after every other field landed: WritePlacement reads the
	// set's name fresh inside its own transaction, so it keys the copy rule by
	// the new name on its own when this same request also renamed the set.
	placed, ok := h.applyPlacement(w, r, item, change)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{
		"dropped":   placed.Dropped,
		"placement": h.svc.placementViewOf(r.Context(), store.ItemRef{Domain: "files", Key: id}),
	}))
}

// handleDeleteFileSet removes a file set and its run history without touching
// any repo, so DiscoverFileSets can bring its snapshots back;
// handleDeleteBackupsFileSet deletes the backups. DELETE /api/files/sets/{id}
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

// handleDeleteBackupsFileSet removes every backup of a file set from the
// selected source. DELETE /api/files/sets/{id}/backups?source=
func (h *Handler) handleDeleteBackupsFileSet(w http.ResponseWriter, r *http.Request) {
	id, ok := h.fileSetIDParam(w, r)
	if !ok {
		return
	}
	if err := h.svc.DeleteBackupsFileSet(r.Context(), id, sourceParam(r)); err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(nil))
}

// handleBackupFileSet starts a single file-set backup on the server and returns
// immediately, like handleBackup. The SPA follows "files:<name>" over SSE.
// POST /api/files/sets/{id}/backup
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

// handleBackupFilesAll starts a server-side batch backup of the selected file
// sets, like handleBackupAll; the SPA follows "batch:files" and the per-set
// keys. POST /api/files/backup-all  body {ids: [...]}
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
	if len(body.IDs) > 1000 { // far beyond any real set count
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

// handleRestoreFileSet starts a file-set restore on the server and returns
// immediately, like handleRestore. An empty targetPath restores in place over
// the set's source folder and needs confirm; otherwise the snapshot is
// extracted into that folder under the host mount. The resolved target comes
// back in the answer, and the SPA follows "files:<name>".
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
		restoreFail(w, sourceParam(r), err)
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
		restoreFail(w, sourceParam(r), err)
		return
	}
	if files == nil {
		files = []restic.FileEntry{}
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"files": files}))
}

// handleRestoreFileSetFiles starts a selective file-set restore of the given
// paths, otherwise like handleRestoreFileSet: an empty targetPath restores them
// in place and needs confirm.
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
		restoreFail(w, sourceParam(r), err)
		return
	}
	if !started {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": "a backup or restore is already running"})
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"started": true, "target": target}))
}

// handleDiscoverFiles rebuilds the file-set list from the fileset: snapshot
// tags alone, since the files domain stores no definitions in the repository,
// so sets lost with the database become restorable again.
// POST /api/files/discover
func (h *Handler) handleDiscoverFiles(w http.ResponseWriter, r *http.Request) {
	probe := r.URL.Query().Get("probe") == "true" // read-only readiness check, see handleDiscover
	res, err := h.svc.DiscoverFileSets(r.Context(), probe)
	if err != nil {
		// The partial result goes along, as in handleDiscover.
		body := failEnvelope(err)
		maps.Copy(body, discoverFields(res))
		writeJSON(w, http.StatusOK, body)
		return
	}
	fields := discoverFields(res)
	fields["repo"] = h.svc.DiscoverSource("files")
	writeJSON(w, http.StatusOK, okEnvelope(fields))
}

// handleForeignOpen opens another BombVault instance's repository read-only
// with that instance's APP_KEY and returns an in-memory session id and the
// snapshot inventory. The session and the key live only in memory with a TTL,
// the repo is never initialised, and the key is never logged or echoed.
// POST /api/foreign/open  body {location, key}
func (h *Handler) handleForeignOpen(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Location string `json:"location"`
		Key      string `json:"key"`
		// Creds are the foreign repository's own backend credentials, needed for
		// a remote location and used for this session only, never stored.
		Creds *CloudCreds `json:"creds"`
	}
	if !decodeBody(w, r, &body) {
		return
	}
	session, inv, err := h.svc.OpenForeign(r.Context(), body.Location, body.Key, body.Creds)
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

// handleForeignRestore restores one container, VM or file set from an open
// foreign-repo session on the server and returns immediately, like
// handleRestore. A bad request (unknown or expired session, no confirm, a file
// set without target) fails with a 4xx before anything starts, and a busy
// guard answers 409. The session key stays on the server.
// POST /api/foreign/restore  body {session, domain, item, snapshot, confirm, target, paths, zvolPool}
//
// A non-empty paths restores just those entries of a file set into target.
// zvolPool, for VMs only, names the destination ZFS pool a TrueNAS zvol disk
// needs on a restore to another instance (see StartForeignRestore). Only a
// direct API call sets it; without it such a restore fails early with a clear
// message.
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
	if err != nil { // validation failed, nothing was started
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
// session, so the recovery UI can offer a picker before a selective restore.
// The answer has the shape of the local list-files endpoint.
// POST /api/foreign/files  body {session, domain, item, snapshot}
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
// before a cross-pool restore. Appdata is remapped automatically; these binds
// are the operator's to fix in the template.
// POST /api/foreign/container-warnings  body {session, item}
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
