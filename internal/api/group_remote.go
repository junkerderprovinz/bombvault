package api

// Remote view: from one paired instance, open another, see its data and
// start the harmless actions below. Two allowlists, one list: remoteViewRoutes
// registers the exact same handlers this instance's own browser reaches, and
// both the outbound forward route (handleInstanceForward, below) and the
// inbound dispatch a group member's call goes through (Service.serveRemoteView,
// in group.go) check a call against that same registration through
// (*Handler).remoteViewAllowed. Nothing else a member asks for is served.
//
// Destructive routes, settings, credentials and auth stay off this list on
// purpose; they are reached only by a session on the instance itself.

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log"
	"net/http"
	"net/url"
	"path"
	"slices"
	"strings"

	"github.com/junkerderprovinz/bombvault/internal/group"
	"github.com/junkerderprovinz/bombvault/internal/relay"
)

// remoteRoute is one call a group member's remote-view forward may reach,
// registered under the real pattern this instance's own Router() uses, so a
// call runs the exact route a browser would.
type remoteRoute struct {
	pattern string
	handle  func(h *Handler, w http.ResponseWriter, r *http.Request)
}

// remoteViewRoutes is the whole remote-view surface: reads for the pages
// remote view shows, and the harmless triggers it may start. Every restore,
// delete, prune, setting and credential is left off; those stay local.
//
// A function rather than a package-level slice: one of the triggers below
// (the fleet poll) calls back into the pairing machinery that reaches
// remoteMux, and a package-level var there would make the compiler see that
// as an initialization cycle even though nothing runs at init time.
func remoteViewRoutes() []remoteRoute {
	return []remoteRoute{
		// Dashboard and history.
		{"GET /api/status", (*Handler).handleStatus},
		{"GET /api/coverage", (*Handler).handleCoverage},
		{"GET /api/history", (*Handler).handleHistory},
		{"GET /api/runs", (*Handler).handleRuns},
		{"GET /api/stats", (*Handler).handleStats},
		{"GET /api/schedule/next", (*Handler).handleScheduleNext},
		{"GET /api/verify", (*Handler).handleDrills},
		// Anomalies.
		{"GET /api/anomalies", (*Handler).handleAnomalies},
		{"GET /api/anomalies/summary", (*Handler).handleAnomalySummary},
		{"GET /api/anomalies/items", (*Handler).handleAnomalyItems},
		{"GET /api/anomalies/{id}", (*Handler).handleAnomaly},
		// Domain lists and their snapshot metadata.
		{"GET /api/containers", (*Handler).handleListContainers},
		{"GET /api/containers/{name}/snapshots", (*Handler).handleSnapshots},
		{"GET /api/vms", (*Handler).handleListVMs},
		{"GET /api/vms/{name}/snapshots", (*Handler).handleSnapshotsVM},
		{"GET /api/files", (*Handler).handleListFileSets},
		{"GET /api/files/sets/{id}/snapshots", (*Handler).handleSnapshotsFileSet},
		{"GET /api/zfs", (*Handler).handleListZFSDatasets},
		{"GET /api/zfs/datasets/{id}/restore-points", (*Handler).handleZFSRestorePoints},
		{"GET /api/flash/snapshots", (*Handler).handleSnapshotsFlash},
		{"GET /api/config/snapshots", (*Handler).handleSnapshotsConfig},
		// Receiver and pull lists, and the off-site target list (already scrubbed
		// of credentials by the same view a session gets).
		{"GET /api/receiver/repos", (*Handler).handleListReceiverRepos},
		{"GET /api/receiver/repos/{id}/inventory", (*Handler).handleReceiverInventory},
		{"GET /api/pull/sources", (*Handler).handleListPullSources},
		{"GET /api/offsite/targets", (*Handler).handleListOffsiteTargets},

		// Triggers: backup of one item, a domain's "all", and everything.
		{"POST /api/containers/{name}/backup", (*Handler).handleBackup},
		{"POST /api/containers/backup-all", (*Handler).handleBackupAll},
		{"POST /api/vms/{name}/backup", (*Handler).handleBackupVM},
		{"POST /api/files/sets/{id}/backup", (*Handler).handleBackupFileSet},
		{"POST /api/files/backup-all", (*Handler).handleBackupFilesAll},
		{"POST /api/zfs/datasets/{id}/backup", (*Handler).handleBackupZFSDataset},
		{"POST /api/zfs/backup-all", (*Handler).handleBackupZFSAll},
		{"POST /api/flash/backup", (*Handler).handleBackupFlash},
		{"POST /api/config/backup", (*Handler).handleBackupConfig},
		{"POST /api/backup-everything", (*Handler).handleBackupEverything},
		{"POST /api/backup/cancel", (*Handler).handleBackupCancel},
		// Triggers: check, verify, replicate off-site, pull a run, receiver check,
		// fleet poll.
		{"POST /api/check/{domain}", (*Handler).handleCheck},
		{"POST /api/verify/{domain}", (*Handler).handleRunDrill},
		{"POST /api/offsite/{domain}", (*Handler).handleReplicateOffsite},
		{"POST /api/pull/sources/{id}/run", (*Handler).handleRunPullSource},
		{"POST /api/receiver/repos/{id}/check", (*Handler).handleReceiverCheck},
		{"POST /api/fleet/peers/{id}/poll", (*Handler).handleFleetPeerPoll},
	}
}

// remoteViewShadowed are literal routes that would otherwise fall through to
// a wildcard pattern remoteViewRoutes also registers: /api/offsite/targets
// and /api/offsite/rclone-remote are real routes, not a "targets" or
// "rclone-remote" off-site domain, and both create or change stored
// credentials. Registering them here, refused outright, makes this mux prefer
// them over the {domain} wildcard exactly as the real router does, instead of
// a stray call landing in handleReplicateOffsite with a domain that happens
// to read "targets".
var remoteViewShadowed = []string{
	"POST /api/offsite/targets",
	"POST /api/offsite/rclone-remote",
}

func refuseRemoteView(w http.ResponseWriter, _ *http.Request) {
	http.Error(w, "route not proxied", http.StatusForbidden)
}

// remoteMux is the http.ServeMux built from remoteViewRoutes, cached after
// the first call. Matching a request against it, without running it, is also
// how remoteViewAllowed checks the allowlist: one registration serves both
// jobs, so they cannot drift apart.
func (h *Handler) remoteMux() http.Handler {
	h.remoteMuxOnce.Do(func() {
		mux := http.NewServeMux()
		for _, rt := range remoteViewRoutes() {
			handle := rt.handle
			mux.HandleFunc(rt.pattern, func(w http.ResponseWriter, r *http.Request) { handle(h, w, r) })
		}
		for _, pattern := range remoteViewShadowed {
			mux.HandleFunc(pattern, refuseRemoteView)
		}
		h.remoteMuxHandler = mux
	})
	return h.remoteMuxHandler
}

// remoteViewAllowed reports whether method and target, an "/api/..." path
// with its query string if it has one, are on the remote-view allowlist. A
// nil h (a Service built without a Handler around it) allows nothing.
// path.Clean catches an encoded ".." segment that would otherwise walk the
// path outside what target names before the mux ever sees it.
func (h *Handler) remoteViewAllowed(method, target string) bool {
	if h == nil {
		return false
	}
	p := target
	if i := strings.IndexByte(p, '?'); i >= 0 {
		p = p[:i]
	}
	if path.Clean(p) != p {
		return false
	}
	sm, ok := h.remoteMux().(*http.ServeMux)
	if !ok {
		return false
	}
	_, pattern := sm.Handler(&http.Request{Method: method, URL: &url.URL{Path: p}})
	return pattern != "" && !slices.Contains(remoteViewShadowed, pattern)
}

// handleInstanceForward asks a paired member an allowlisted call and answers
// with whatever it answered. rest carries no leading /api/, matching the
// shape callMember and pairing().Call already use.
// ANY /api/instances/{id}/{rest...}
func (h *Handler) handleInstanceForward(w http.ResponseWriter, r *http.Request) {
	target := "/api/" + r.PathValue("rest")
	if r.URL.RawQuery != "" {
		target += "?" + r.URL.RawQuery
	}
	if !h.remoteViewAllowed(r.Method, target) {
		http.Error(w, "route not proxied", http.StatusForbidden)
		return
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, group.MaxCallBytes))
	if err != nil {
		http.Error(w, "request too large", http.StatusRequestEntityTooLarge)
		return
	}
	id := r.PathValue("id")
	status, out, err := h.svc.pairing().Call(r.Context(), id, r.Method, target, body)
	if err != nil {
		if !errors.Is(err, group.ErrNotMember) {
			log.Printf("group: remote view %s %q to member %q: %v", r.Method, target, id, err) //nolint:gosec // G706: target and id are %q-quoted
		}
		writeJSON(w, http.StatusOK, failEnvelope(remoteViewCallError(err)))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(out) //nolint:gosec // G705: out is the member's own JSON envelope, written back under the same content type, never interpreted as HTML
}

// remoteViewCallError turns a pairing().Call failure into the same wording
// callMember uses elsewhere, so a member unreachable through remote view
// reads the same as one unreachable for any other group call.
func remoteViewCallError(err error) error {
	if errors.Is(err, group.ErrNotMember) {
		return errMemberGone
	}
	return errMemberSilent
}

// serveRemoteView answers a group member's remote-view call: an allowlisted
// route of this instance's own handler, run only while the remote-view
// switch is on. matched is false when call is not on the allowlist at all,
// so servePeer falls through to its other routes instead of refusing a call
// that was never meant for this surface.
func (s *Service) serveRemoteView(ctx context.Context, call relay.ProxyCall) (status int, body []byte, matched bool) {
	h := s.remoteHandler
	if !h.remoteViewAllowed(call.Method, call.Path) {
		return 0, nil, false
	}
	settings, err := s.store.GetSettings()
	if err != nil {
		return http.StatusInternalServerError, nil, true
	}
	if !settings.RemoteViewEnabled {
		return http.StatusForbidden, []byte(`{"ok":false,"error":"remote view is off on this instance"}`), true
	}
	req, err := http.NewRequestWithContext(ctx, call.Method, call.Path, bytes.NewReader(call.Body))
	if err != nil {
		return http.StatusBadRequest, nil, true
	}
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(WithRunOrigin(req.Context(), RunOrigin{Via: "remote", KeyID: call.Sender}))
	rec := &peerRecorder{header: http.Header{}, status: http.StatusOK}
	// Mirrors servePeer's own recover: this runs on the relay client's read
	// goroutine or inside ServeDirect, neither of which has a net/http server
	// behind it to catch a panicking handler.
	defer func() {
		if r := recover(); r != nil {
			log.Printf("group: remote view handler panicked serving %s %s: %v", call.Method, call.Path, r)
			status, body, matched = http.StatusInternalServerError, nil, true
		}
	}()
	h.remoteMux().ServeHTTP(rec, req)
	return rec.status, rec.body.Bytes(), true
}
