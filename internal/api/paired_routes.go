package api

import (
	"context"
	"errors"
	"net/http"

	"github.com/junkerderprovinz/bombvault/internal/paths"
)

// pairedRoutes are the routes the session of a paired phone reaches, by the
// pattern they are registered under. The phone holds the group's words and
// nothing else, so it gets what the app shows: the overview, the runs, the
// backups to look at, start and restore, the anomalies and the members. A
// route that is not here answers own-login-needed to such a session, so a new
// one stays out until somebody puts it here.
var pairedRoutes = map[string]bool{
	// Signing out, and what the shell of every page reads. The settings answer
	// a paired session with every location and hook scrubbed.
	"POST /api/logout":       true,
	"GET /api/settings":      true,
	"GET /api/display-prefs": true,
	"PUT /api/display-prefs": true,
	"GET /api/release-notes": true,
	"GET /api/versions":      true,

	// The overview.
	"GET /api/status":           true,
	"GET /api/coverage":         true,
	"GET /api/stats":            true,
	"GET /api/breakdown":        true,
	"GET /api/history":          true,
	"GET /api/schedule/next":    true,
	"GET /api/schedule/waiting": true,
	"GET /api/spike":            true,
	"GET /api/checks/items":     true,
	"GET /api/verify":           true,

	// The runs, their progress and the activity log.
	"GET /api/runs":                     true,
	"POST /api/runs/ack":                true,
	"GET /api/progress":                 true,
	"GET /api/zfs/runs/{runId}/members": true,

	// The backups and their entries, to look at. Lists that carry a location
	// or a hook answer a paired session with those scrubbed.
	"GET /api/items":                              true,
	"GET /api/items/{kind}/{key}":                 true,
	"GET /api/items/{domain}/{name}/timeline":     true,
	"GET /api/items/{domain}/{name}/diff":         true,
	"GET /api/containers":                         true,
	"GET /api/containers/backup-order":            true,
	"GET /api/containers/{name}/snapshots":        true,
	"GET /api/containers/{name}/diff":             true,
	"GET /api/containers/{name}/files":            true,
	"GET /api/containers/{name}/dbdumps":          true,
	"GET /api/vms":                                true,
	"GET /api/vms/backup-order":                   true,
	"GET /api/vms/{name}/snapshots":               true,
	"GET /api/files":                              true,
	"GET /api/files/sets/{id}/snapshots":          true,
	"GET /api/files/sets/{id}/files":              true,
	"GET /api/zfs":                                true,
	"GET /api/zfs/datasets/{id}/files":            true,
	"GET /api/zfs/datasets/{id}/restore-points":   true,
	"GET /api/zfs/datasets/{id}/safety-snapshots": true,
	"GET /api/zfs/connection":                     true,
	"GET /api/zfs/host":                           true,
	"GET /api/containers/{name}/mounts":           true,
	"GET /api/retention/preview/{domain}":         true,
	"GET /api/flash/snapshots":                    true,
	"GET /api/flash/plugins":                      true,
	"GET /api/offsite/targets":                    true,
	"GET /api/offsite/destinations":               true,
	"GET /api/offsite/providers":                  true,
	"GET /api/repos":                              true,
	"GET /api/storage/locations":                  true,
	"GET /api/storage/locations/{id}":             true,

	// Starting a backup or a copy, checking one, and stopping a run.
	"POST /api/backup-everything":             true,
	"POST /api/containers/backup-all":         true,
	"POST /api/containers/{name}/backup":      true,
	"POST /api/vms/backup-all":                true,
	"POST /api/vms/{name}/backup":             true,
	"POST /api/files/backup-all":              true,
	"POST /api/files/sets/{id}/backup":        true,
	"POST /api/zfs/backup-all":                true,
	"POST /api/zfs/datasets/{id}/backup":      true,
	"POST /api/flash/backup":                  true,
	"POST /api/config/backup":                 true,
	"POST /api/offsite/{domain}":              true,
	"POST /api/zfs/datasets/{id}/replica/run": true,
	"POST /api/pull/sources/{id}/run":         true,
	"POST /api/check/{domain}":                true,
	"POST /api/verify/{domain}":               true,
	"POST /api/backup/cancel":                 true,

	// Restoring and following a restore. The handlers back up what a restore
	// in place replaces, or refuse it to a paired session, and restore into a
	// folder only while it holds nothing.
	"POST /api/restore/check":                     true,
	"POST /api/restore/cancel":                    true,
	"POST /api/containers/{name}/restore":         true,
	"POST /api/containers/{name}/restore-files":   true,
	"POST /api/containers/{name}/restore-to":      true,
	"POST /api/vms/{name}/restore":                true,
	"POST /api/files/sets/{id}/restore":           true,
	"POST /api/files/sets/{id}/restore-files":     true,
	"POST /api/zfs/datasets/{id}/restore":         true,
	"POST /api/zfs/datasets/{id}/replica/restore": true,
	"GET /api/stacks/{project}/dir":               true,
	"GET /api/browse":                             true,

	// The anomalies, to read. Acknowledging one ends the hold on the backups
	// from before it.
	"GET /api/anomalies":                         true,
	"GET /api/anomalies/summary":                 true,
	"GET /api/anomalies/items":                   true,
	"GET /api/anomalies/items/{targetId}/series": true,
	"GET /api/anomalies/{id}":                    true,
	"GET /api/anomalies/{id}/changes":            true,

	// The Instances page, to look at.
	"GET /api/group":                    true,
	"GET /api/group/roles":              true,
	"GET /api/group/members/{id}/repos": true,
	"GET /api/fleet/peers":              true,
	"POST /api/fleet/peers/{id}/poll":   true,
	"GET /api/offsite/group-receivers":  true,
	"GET /api/zfs/receive/requests":     true,
}

type pairedKey struct{}

// withPaired marks a request as one made with the session of a paired phone.
func withPaired(r *http.Request) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), pairedKey{}, true))
}

// isPaired reports whether a request was made with the session of a paired
// phone, which an answer that carries a location or a hook scrubs them for.
func isPaired(r *http.Request) bool {
	return pairedContext(r.Context())
}

// pairedContext is isPaired for the context a handler hands to the service.
func pairedContext(ctx context.Context) bool {
	paired, _ := ctx.Value(pairedKey{}).(bool)
	return paired
}

// errPairedFolderInUse refuses a paired phone a restore into a folder that
// already holds something, which it could not undo.
var errPairedFolderInUse = errors.New("a paired phone restores only into a new or empty folder")

// makeRestoreFolder creates the folder a restore to a folder writes into with
// ensure. A paired phone gets only a folder that holds nothing yet, and one
// that is already there keeps its mode.
func makeRestoreFolder(ctx context.Context, target string, ensure func(string) error) error {
	if !pairedContext(ctx) {
		return ensure(target)
	}
	err := paths.EnsureEmptyDir(target)
	if errors.Is(err, paths.ErrNotEmpty) {
		return errPairedFolderInUse
	}
	return err
}

// refusePaired answers a paired phone's request for a route it does not
// reach.
func refusePaired(w http.ResponseWriter) {
	writeJSON(w, http.StatusForbidden, map[string]any{
		"ok":    false,
		"error": "sign in at this instance's web interface to do this; the session of a paired phone does not reach it",
		"code":  codeOwnLoginNeeded,
	})
}
