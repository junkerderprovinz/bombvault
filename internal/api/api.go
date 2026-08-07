package api

import (
	"net/http"
	"sync"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/dockercli"
	"github.com/junkerderprovinz/bombvault/internal/progress"
	"github.com/junkerderprovinz/bombvault/internal/schedule"
	"github.com/junkerderprovinz/bombvault/internal/spike"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// Handler bundles the JSON API dependencies and builds the router.
type Handler struct {
	cfg       config.Config
	store     *store.Repo
	docker    dockercli.Docker
	svc       *Service
	scheduler *schedule.Scheduler
	probes    []spike.Probe
	progress  *progress.Store // optional; nil = SSE progress endpoint streams nothing
	// containersLastRun / vmsLastRun drive the everyN due-gates in
	// ReloadWithDueChecks for their respective domains.
	containersLastRun schedule.LastRunFunc
	vmsLastRun        schedule.LastRunFunc
	flashLastRun      schedule.LastRunFunc
	configLastRun     schedule.LastRunFunc
	filesLastRun      schedule.LastRunFunc

	// Cached host-integration check, warmed once at startup so the dashboard
	// shows the result list instantly. Guarded by spikeMu; refreshed on POST.
	spikeMu     sync.RWMutex
	spikeChecks any
	spikeAllOK  bool
	spikeRan    bool

	// login brute-force throttle: timestamps of recent failed logins. A global
	// (not per-IP) window is enough for this single-operator LAN tool — it just
	// slows password guessing on the optional auth gate.
	loginMu    sync.Mutex
	loginFails []time.Time
}

// NewHandler constructs the API handler.
func NewHandler(
	cfg config.Config,
	st *store.Repo,
	d dockercli.Docker,
	svc *Service,
	scheduler *schedule.Scheduler,
	probes []spike.Probe,
) *Handler {
	return &Handler{
		cfg:               cfg,
		store:             st,
		docker:            d,
		svc:               svc,
		scheduler:         scheduler,
		probes:            probes,
		containersLastRun: schedule.LastRunFunc(st.LastSuccessfulContainerBackup),
		vmsLastRun:        schedule.LastRunFunc(st.LastSuccessfulVMBackup),
		flashLastRun:      schedule.LastRunFunc(st.LastSuccessfulFlashBackup),
		configLastRun:     schedule.LastRunFunc(st.LastSuccessfulConfigBackup),
		filesLastRun:      schedule.LastRunFunc(st.LastSuccessfulFilesBackup),
	}
}

// SetProgress wires the live-progress store the SSE endpoint streams from (the
// same store the service publishes backup/restore percentages to). Called from
// main; must be set before Router() so the route reflects it.
func (h *Handler) SetProgress(p *progress.Store) { h.progress = p }

// Router returns the API mux with Go 1.22 method+path patterns. All routes are
// under /api/.  The entire mux is wrapped with authGate so that when
// authentication is enabled every request (other than the public allow-listed
// paths) requires a valid session cookie.
func (h *Handler) Router() http.Handler {
	mux := http.NewServeMux()

	// Public / auth endpoints — also allow-listed inside authGate.
	mux.HandleFunc("GET /api/health", h.handleHealth)
	mux.HandleFunc("GET /api/auth", h.handleAuthStatus)
	mux.HandleFunc("POST /api/login", h.handleLogin)
	mux.HandleFunc("POST /api/logout", h.handleLogout)
	// NOT on the authGate allowlist: when auth is on, only an authenticated
	// session may revoke every other session (epoch rotation).
	mux.HandleFunc("POST /api/logout-all", h.handleLogoutAll)
	mux.HandleFunc("POST /api/auth/password", h.handleSetPassword)

	// Opt-in Prometheus scrape endpoint. NOT under /api so it never collides with
	// the JSON routes; it bypasses the session authGate (allow-listed there) and
	// is gated by its own settings (enabled flag + optional bearer token) inside
	// the handler instead.
	mux.HandleFunc("GET /metrics", h.handleMetrics)

	// Embeddable dashboard widget (mini activity log for iframes). The page +
	// its feed follow the /metrics pattern: allow-listed in authGate (an iframe
	// can't carry the session cookie) and self-gated on the stored widget token
	// inside the handlers — no token stored = 403, fail closed. The token
	// management endpoints stay session-protected like every other /api route.
	mux.HandleFunc("GET /widget", h.handleWidgetPage)
	mux.HandleFunc("GET /api/widget/data", h.handleWidgetData)
	mux.HandleFunc("POST /api/widget/token", h.handleWidgetTokenGenerate)
	mux.HandleFunc("DELETE /api/widget/token", h.handleWidgetTokenDisable)

	// Protected endpoints.
	mux.HandleFunc("GET /api/containers", h.handleListContainers)
	mux.HandleFunc("POST /api/containers/backup-all", h.handleBackupAll)
	mux.HandleFunc("POST /api/containers/schedule-include", h.handleScheduleIncludeAll)
	mux.HandleFunc("GET /api/containers/backup-order", h.handleGetBackupOrder)
	mux.HandleFunc("PUT /api/containers/backup-order", h.handleSetBackupOrder)
	mux.HandleFunc("POST /api/containers/{name}/backup", h.handleBackup)
	mux.HandleFunc("GET /api/containers/{name}/snapshots", h.handleSnapshots)
	mux.HandleFunc("POST /api/containers/{name}/restore", h.handleRestore)
	mux.HandleFunc("POST /api/restore/cancel", h.handleRestoreCancel)
	mux.HandleFunc("POST /api/stacks/{project}/restore", h.handleRestoreStack)
	mux.HandleFunc("GET /api/containers/{name}/mounts", h.handleContainerMounts)
	mux.HandleFunc("POST /api/containers/{name}/excludes/preview", h.handleExcludesPreview)
	mux.HandleFunc("GET /api/containers/{name}/excludes/suggest", h.handleExcludesSuggest)
	mux.HandleFunc("POST /api/containers/{name}/export", h.handleExportContainer)
	mux.HandleFunc("GET /api/containers/{name}/files", h.handleListFiles)
	mux.HandleFunc("POST /api/containers/{name}/restore-files", h.handleRestoreFiles)
	mux.HandleFunc("POST /api/containers/{name}/restore-to", h.handleRestoreContainerTo)
	mux.HandleFunc("GET /api/containers/{name}/diff", h.handleDiff)
	mux.HandleFunc("POST /api/containers/{name}/tag", h.handleTagSnapshot)
	mux.HandleFunc("DELETE /api/containers/{name}/backups", h.handleDeleteBackups)
	mux.HandleFunc("PATCH /api/containers/{name}", h.handlePatchContainer)
	mux.HandleFunc("GET /api/settings", h.handleGetSettings)
	mux.HandleFunc("PUT /api/settings", h.handlePutSettings)
	// Portable settings export / import (a JSON file to move a configuration
	// between instances; no live link). Session-protected like every other /api
	// route — the credentialed export is as sensitive as the recovery kit.
	mux.HandleFunc("GET /api/settings/export", h.handleExportSettings)
	mux.HandleFunc("POST /api/settings/import", h.handleImportSettings)
	mux.HandleFunc("GET /api/recovery-kit", h.handleRecoveryKit)
	mux.HandleFunc("POST /api/recovery-kit/ack", h.handleRecoveryKitAck)
	mux.HandleFunc("GET /api/rclone", h.handleRcloneInfo)
	mux.HandleFunc("POST /api/rclone", h.handleSetRclone)
	mux.HandleFunc("GET /api/cloud", h.handleGetCloud)
	mux.HandleFunc("POST /api/cloud", h.handleSetCloud)
	mux.HandleFunc("GET /api/notify", h.handleGetNotify)
	mux.HandleFunc("POST /api/notify", h.handleSetNotify)
	mux.HandleFunc("POST /api/notify/test", h.handleTestNotify)
	mux.HandleFunc("GET /api/release-notes", h.handleReleaseNotes)
	mux.HandleFunc("GET /api/schedule/next", h.handleScheduleNext)
	mux.HandleFunc("POST /api/check/{domain}", h.handleCheck)
	mux.HandleFunc("POST /api/verify/{domain}", h.handleRunDrill)
	mux.HandleFunc("GET /api/verify", h.handleDrills)
	mux.HandleFunc("POST /api/unlock/{domain}", h.handleUnlock)
	mux.HandleFunc("POST /api/prune/{domain}", h.handlePrune)
	mux.HandleFunc("DELETE /api/snapshots/{domain}/{id}", h.handleDeleteSnapshot)
	// Off-site target CRUD (multi-off-site). The literal "targets" segment is more
	// specific than "{domain}", so these never collide with the per-domain routes.
	mux.HandleFunc("GET /api/offsite/targets", h.handleListOffsiteTargets)
	mux.HandleFunc("POST /api/offsite/targets", h.handleCreateOffsiteTarget)
	mux.HandleFunc("PUT /api/offsite/targets/{id}", h.handleUpdateOffsiteTarget)
	mux.HandleFunc("DELETE /api/offsite/targets/{id}", h.handleDeleteOffsiteTarget)
	mux.HandleFunc("POST /api/offsite/{domain}", h.handleReplicateOffsite)
	mux.HandleFunc("POST /api/offsite/{domain}/test", h.handleTestOffsite)
	mux.HandleFunc("GET /api/offsite/{domain}/deploy-snippet", h.handleDeploySnippet)
	mux.HandleFunc("POST /api/offsite/{domain}/tamper-test", h.handleTamperTest)
	mux.HandleFunc("GET /api/spike", h.handleSpikeCached)
	mux.HandleFunc("POST /api/spike", h.handleSpikeFresh)
	mux.HandleFunc("POST /api/discover", h.handleDiscover)
	mux.HandleFunc("GET /api/runs", h.handleRuns)
	mux.HandleFunc("GET /api/status", h.handleStatus)
	mux.HandleFunc("GET /api/history", h.handleHistory)
	mux.HandleFunc("GET /api/stats", h.handleStats)
	mux.HandleFunc("GET /api/browse", h.handleBrowse)
	mux.HandleFunc("POST /api/browse/mkdir", h.handleMkdir)
	mux.HandleFunc("GET /api/progress", h.handleProgress)

	// VM endpoints.
	mux.HandleFunc("GET /api/vms", h.handleListVMs)
	mux.HandleFunc("POST /api/vms/discover", h.handleDiscoverVMs)
	mux.HandleFunc("POST /api/vms/schedule-include", h.handleVMScheduleIncludeAll)
	mux.HandleFunc("POST /api/vms/{name}/backup", h.handleBackupVM)
	mux.HandleFunc("GET /api/vms/{name}/snapshots", h.handleSnapshotsVM)
	mux.HandleFunc("POST /api/vms/{name}/restore", h.handleRestoreVM)
	mux.HandleFunc("POST /api/vms/{name}/export", h.handleExportVM)
	mux.HandleFunc("DELETE /api/vms/{name}/backups", h.handleDeleteBackupsVM)
	mux.HandleFunc("DELETE /api/vms/{name}", h.handleForgetVM)
	mux.HandleFunc("PATCH /api/vms/{name}", h.handlePatchVM)
	mux.HandleFunc("GET /api/vm/ssh", h.handleVMSSHInfo)
	mux.HandleFunc("POST /api/vm/ssh/test", h.handleVMSSHTest)

	// Companion Unraid dashboard-tile plugin (install/remove over host SSH).
	// Session-protected like every other /api route — these MODIFY the host, so
	// they must never join the authGate public allowlist.
	mux.HandleFunc("GET /api/dashboard-plugin", h.handleDashboardPluginStatus)
	mux.HandleFunc("POST /api/dashboard-plugin/install", h.handleDashboardPluginInstall)
	mux.HandleFunc("POST /api/dashboard-plugin/remove", h.handleDashboardPluginRemove)

	// Flash endpoints (singleton domain — the Unraid USB).
	mux.HandleFunc("POST /api/flash/backup", h.handleBackupFlash)
	mux.HandleFunc("GET /api/flash/snapshots", h.handleSnapshotsFlash)
	mux.HandleFunc("GET /api/flash/download", h.handleDownloadFlash)

	// Config endpoints (singleton domain — BombVault's own /config self-backup).
	mux.HandleFunc("POST /api/config/backup", h.handleBackupConfig)
	mux.HandleFunc("GET /api/config/snapshots", h.handleSnapshotsConfig)
	mux.HandleFunc("POST /api/config/restore", h.handleRestoreConfig)

	// Files endpoints (the files domain — named host folders backed up as file
	// sets, #62).
	mux.HandleFunc("GET /api/files", h.handleListFileSets)
	mux.HandleFunc("POST /api/files/sets", h.handleCreateFileSet)
	mux.HandleFunc("PATCH /api/files/sets/{id}", h.handlePatchFileSet)
	mux.HandleFunc("DELETE /api/files/sets/{id}", h.handleDeleteFileSet)
	mux.HandleFunc("DELETE /api/files/sets/{id}/backups", h.handleDeleteBackupsFileSet)
	mux.HandleFunc("POST /api/files/sets/{id}/backup", h.handleBackupFileSet)
	mux.HandleFunc("POST /api/files/backup-all", h.handleBackupFilesAll)
	mux.HandleFunc("GET /api/files/sets/{id}/snapshots", h.handleSnapshotsFileSet)
	mux.HandleFunc("GET /api/files/sets/{id}/files", h.handleListSnapshotFilesFileSet)
	mux.HandleFunc("POST /api/files/sets/{id}/restore", h.handleRestoreFileSet)
	mux.HandleFunc("POST /api/files/sets/{id}/restore-files", h.handleRestoreFileSetFiles)
	mux.HandleFunc("POST /api/files/discover", h.handleDiscoverFiles)

	// Foreign-repo read-only session endpoints (restore from ANOTHER BombVault
	// instance's repo, #61). Sessions are in-memory with a TTL — never persisted
	// to Settings. AuthGate-protected like every other /api route (the public
	// allowlist stays exactly as is).
	mux.HandleFunc("POST /api/foreign/open", h.handleForeignOpen)
	mux.HandleFunc("POST /api/foreign/close", h.handleForeignClose)
	mux.HandleFunc("POST /api/foreign/restore", h.handleForeignRestore)
	mux.HandleFunc("POST /api/foreign/files", h.handleForeignFiles)
	mux.HandleFunc("POST /api/foreign/container-warnings", h.handleForeignContainerWarnings)

	// Receiver dashboard (read-only): register + monitor immutable off-site repos
	// this box RECEIVES copies into. Gated behind the receiverEnabled settings flag
	// in the SPA; the endpoints stay session-protected like every other /api route.
	// The literal path segments are more specific than any {id}, so no collision.
	mux.HandleFunc("GET /api/receiver/repos", h.handleListReceiverRepos)
	mux.HandleFunc("POST /api/receiver/repos", h.handleCreateReceiverRepo)
	mux.HandleFunc("PUT /api/receiver/repos/{id}", h.handleUpdateReceiverRepo)
	mux.HandleFunc("DELETE /api/receiver/repos/{id}", h.handleDeleteReceiverRepo)
	mux.HandleFunc("GET /api/receiver/repos/{id}/inventory", h.handleReceiverInventory)
	mux.HandleFunc("POST /api/receiver/repos/{id}/check", h.handleReceiverCheck)

	return h.authGate(mux)
}
