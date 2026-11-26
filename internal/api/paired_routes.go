package api

import (
	"context"
	"errors"
	"net/http"
	"path"
	"strings"

	"github.com/junkerderprovinz/bombvault/internal/paths"
)

// pairedRoutes are the routes the session of a paired phone reaches, by the
// pattern they are registered under. The phone holds the group's words and
// nothing else, so it gets what the app shows: the overview, the runs, the
// backups to look at, start and restore, the anomalies and the members. A
// route that is not here answers own-login-needed to such a session, so a new
// one stays out until somebody puts it here.
var pairedRoutes = map[string]bool{
	// Signing out, and what the shell of every page reads. The settings hide
	// every location's password from any caller and a hook from a paired session.
	// The update check only reads the release feed, and its answer opens the
	// notes of a newer release.
	"POST /api/logout":                true,
	"GET /api/settings":               true,
	"GET /api/display-prefs":          true,
	"PUT /api/display-prefs":          true,
	"GET /api/release-notes":          true,
	"GET /api/release-notes/versions": true,
	"GET /api/versions":               true,
	"POST /api/update-check":          true,

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

	// The backups and their entries, to look at. A list that carries a location
	// hides its password from any caller, and a hook from a paired session.
	"GET /api/items":                              true,
	"GET /api/items/{kind}/{key}":                 true,
	"GET /api/items/{domain}/{name}/timeline":     true,
	"GET /api/items/{domain}/{name}/diff":         true,
	"GET /api/search/files":                       true,
	"GET /api/containers":                         true,
	"GET /api/containers/{name}":                  true,
	"GET /api/containers/backup-order":            true,
	"GET /api/containers/{name}/snapshots":        true,
	"GET /api/containers/{name}/diff":             true,
	"GET /api/containers/{name}/files":            true,
	"GET /api/containers/{name}/dbdumps":          true,
	"GET /api/vms":                                true,
	"GET /api/vms/{name}":                         true,
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
	"GET /api/zfs/datasets/{id}/replica":          true,
	"GET /api/zfs/replica/servers":                true,
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
	"POST /api/backup/selection":              true,
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
	"POST /api/zfs/replica/run-all":           true,
	"POST /api/pull/sources/{id}/run":         true,
	"POST /api/check/{domain}":                true,
	"POST /api/verify/{domain}":               true,
	"POST /api/checks/probe/{id}":             true,
	"POST /api/checks/starttest/{id}":         true,
	"POST /api/backup/cancel":                 true,

	// What the backups and entry pages change of an entry: whether a schedule
	// reaches it, that it is not to be backed up, a person's own tags on one
	// backup, the snapshot stamps runs left on a ZFS tree, and the entries the
	// backup storage holds. None of them deletes a backup or shows a hook, and a
	// discover does not name the repository it read to a paired session.
	"PUT /api/items/{kind}/{key}/included": true,
	"PUT /api/items/{kind}/{key}/declined": true,
	"POST /api/items/{domain}/{name}/tag":  true,
	"POST /api/zfs/datasets/{id}/sweep":    true,
	"POST /api/discover":                   true,
	"POST /api/vms/discover":               true,
	"POST /api/files/discover":             true,
	"POST /api/zfs/discover":               true,

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

	// A plugin and a database dump go back only once the flash or the
	// container is backed up, and a dump is saved only into a new folder of
	// the restore folder. A download reaches the phone at the server's own
	// address, since the relay carries no files.
	"POST /api/flash/plugins/restore":                  true,
	"POST /api/containers/{name}/dbdumps/{id}/import":  true,
	"POST /api/containers/{name}/dbdumps/{id}/save":    true,
	"GET /api/containers/{name}/dbdumps/{id}/download": true,

	// The anomalies, to read and to settle, an item's expectations to forget
	// and its sensitivity. A finding a paired phone settles keeps its hold on
	// the old backups until a sign-in with the password ends it.
	"GET /api/anomalies":                                           true,
	"GET /api/anomalies/summary":                                   true,
	"GET /api/anomalies/items":                                     true,
	"GET /api/anomalies/items/{targetId}/series":                   true,
	"GET /api/anomalies/{id}":                                      true,
	"GET /api/anomalies/{id}/changes":                              true,
	"POST /api/anomalies/acknowledge":                              true,
	"POST /api/anomalies/expected":                                 true,
	"PUT /api/anomalies/items/{targetId}/sensitivity":              true,
	"DELETE /api/anomalies/items/{targetId}/expectations/{family}": true,

	// The Instances page, to look at.
	"GET /api/group":                    true,
	"GET /api/group/roles":              true,
	"GET /api/group/members/{id}/repos": true,
	"GET /api/fleet/peers":              true,
	"POST /api/fleet/peers/{id}/poll":   true,
	"GET /api/offsite/group-receivers":  true,
	"GET /api/shared-repos/upkeep":      true,
	"GET /api/zfs/receive/requests":     true,
	"GET /api/shared-repo/offers":       true,
}

type pairedKey struct{}

// withPaired marks a request as one made with the session of a paired phone.
func withPaired(r *http.Request) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), pairedKey{}, true))
}

// isPaired reports whether a request was made with the session of a paired
// phone, which an answer that carries a hook hides it from.
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

// refusePairedFolder answers a paired phone's restore or look into a folder
// outside the restore folder of section, and reports whether it did. An empty
// section stands for any section's folder. The phone's words are all it takes to
// reach this, and the folders it could otherwise pick include whatever another
// service hands out.
func (h *Handler) refusePairedFolder(w http.ResponseWriter, r *http.Request, section, sub string) bool {
	if !isPaired(r) {
		return false
	}
	allowed, err := h.svc.inRestoreFolder(section, sub)
	if err == nil && allowed {
		return false
	}
	writeJSON(w, http.StatusForbidden, map[string]any{
		"ok":    false,
		"error": "a paired phone restores only into the restore folder set under Settings; sign in at this instance's web interface to choose another",
		"code":  codeOwnLoginNeeded,
	})
	return true
}

// inRestoreFolder reports whether sub, relative to the host mount, is the restore
// folder of section or lies below it.
func (s *Service) inRestoreFolder(section, sub string) (bool, error) {
	settings, err := s.store.GetSettings()
	if err != nil {
		return false, err
	}
	sections := []string{section}
	if section == "" {
		sections = offsiteConfigDomains
	}
	sub = path.Clean(strings.TrimSpace(sub))
	for _, sec := range sections {
		folder := strings.TrimSpace(settings.RestoreFolderFor(sec))
		if folder == "" {
			continue
		}
		folder = path.Clean(folder)
		if sub == folder || strings.HasPrefix(sub, folder+"/") {
			return true, nil
		}
	}
	return false, nil
}

// targetHoldsFiles reports whether the restore folder sub names holds anything.
// A folder that does not resolve is the restore check's own to refuse.
func (s *Service) targetHoldsFiles(sub string) bool {
	sub = strings.TrimSpace(sub)
	if sub == "" {
		return false
	}
	target, err := paths.ResolveForWrite(s.cfg.HostMountRoot, sub)
	return err == nil && !paths.EmptyOrMissing(target)
}
