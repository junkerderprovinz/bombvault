package api

import (
	"net/http"
	"sync"

	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/dockercli"
	"github.com/junkerderprovinz/bombvault/internal/schedule"
	"github.com/junkerderprovinz/bombvault/internal/spike"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// Handler bundles the JSON API dependencies and builds the router.
type Handler struct {
	cfg               config.Config
	store             *store.Repo
	docker            dockercli.Docker
	svc               *Service
	scheduler         *schedule.Scheduler
	probes            []spike.Probe
	// containersLastRun is used by the everyN due-gate in ReloadWithDueChecks.
	containersLastRun schedule.LastRunFunc

	// Cached host-integration check, warmed once at startup so the dashboard
	// shows the result list instantly. Guarded by spikeMu; refreshed on POST.
	spikeMu     sync.RWMutex
	spikeChecks any
	spikeAllOK  bool
	spikeRan    bool
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
	}
}

// Router returns the API mux with Go 1.22 method+path patterns. All routes are
// under /api/.
func (h *Handler) Router() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/health", h.handleHealth)
	mux.HandleFunc("GET /api/containers", h.handleListContainers)
	mux.HandleFunc("POST /api/containers/{name}/backup", h.handleBackup)
	mux.HandleFunc("GET /api/containers/{name}/snapshots", h.handleSnapshots)
	mux.HandleFunc("POST /api/containers/{name}/restore", h.handleRestore)
	mux.HandleFunc("DELETE /api/containers/{name}/backups", h.handleDeleteBackups)
	mux.HandleFunc("PATCH /api/containers/{name}", h.handlePatchContainer)
	mux.HandleFunc("GET /api/settings", h.handleGetSettings)
	mux.HandleFunc("PUT /api/settings", h.handlePutSettings)
	mux.HandleFunc("GET /api/spike", h.handleSpikeCached)
	mux.HandleFunc("POST /api/spike", h.handleSpikeFresh)
	mux.HandleFunc("GET /api/runs", h.handleRuns)
	mux.HandleFunc("GET /api/browse", h.handleBrowse)

	return mux
}
