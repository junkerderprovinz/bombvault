package api

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"log"
	"slices"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/junkerderprovinz/bombvault/internal/dockercli"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/schedule"
	"github.com/junkerderprovinz/bombvault/internal/store"
	"github.com/junkerderprovinz/bombvault/internal/zfs"
)

// How many repository size samples one call may ask for, and how many it gets
// without asking.
const (
	mcpStatsLimitMax     = 90
	mcpStatsLimitDefault = 30
)

// How many runs one call may ask for, and how many it gets without asking.
const (
	mcpRunsLimitMax     = 100
	mcpRunsLimitDefault = 20
)

// mcpItemsPerDomain caps one domain's item list, so a single answer stays a
// size a client can read.
const mcpItemsPerDomain = 1000

// How many restore points and database dumps one call may ask for, and how many
// it gets without asking.
const (
	mcpPointsLimitMax     = 200
	mcpPointsLimitDefault = 50
	mcpDumpLimitDefault   = 20
)

func (h *Handler) toolGetHealth(ctx context.Context, _ *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	caller, ok := mcpCallerFrom(ctx)
	if !ok {
		return mcpNoCaller(), nil
	}
	settings, err := h.store.GetSettings()
	if err != nil {
		h.logMCPCall(ctx, "get_health", "unavailable")
		return mcpToolError("unavailable", "settings could not be read", nil), nil
	}
	h.logMCPCall(ctx, "get_health", "ok")
	return mcpOK(map[string]any{
		"ok":                true,
		"version":           Version,
		"instanceName":      settings.InstanceName,
		"serverTime":        time.Now().Unix(),
		"backupRunning":     h.svc.BackupInProgress(),
		"everythingRunning": h.svc.EverythingInProgress(),
		"key": map[string]any{
			"label":              caller.Label,
			"canStartBackups":    caller.CanStartBackups,
			"startsLeftThisHour": h.mcp.starts.remaining(caller.KeyID, h.mcp.now()),
		},
	}), nil
}

func (h *Handler) toolGetStatus(ctx context.Context, _ *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if _, ok := mcpCallerFrom(ctx); !ok {
		return mcpNoCaller(), nil
	}
	settings, err := h.store.GetSettings()
	if err != nil {
		h.logMCPCall(ctx, "get_status", "unavailable")
		return mcpToolError("unavailable", "settings could not be read", nil), nil
	}
	domains, err := h.svc.domainStatusFrom(settings)
	if err != nil {
		return h.mcpFailure(ctx, "get_status", err), nil
	}
	for i := range domains {
		domains[i].VerifiedDetail = mcpScrubText(domains[i].VerifiedDetail)
		domains[i].DrillDetail = mcpScrubText(domains[i].DrillDetail)
	}

	next := []schedule.NextRun{}
	if h.scheduler != nil {
		next = append(next, h.scheduler.NextRuns()...)
	}

	h.logMCPCall(ctx, "get_status", "ok")
	return mcpOK(map[string]any{
		"domains":           domains,
		"nextRuns":          next,
		"backupRunning":     h.svc.BackupInProgress(),
		"everythingRunning": h.svc.EverythingInProgress(),
	}), nil
}

func (h *Handler) toolGetCoverage(ctx context.Context, _ *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if _, ok := mcpCallerFrom(ctx); !ok {
		return mcpNoCaller(), nil
	}
	ctx, cancel := h.mcpToolContext(ctx, mcpReadTimeout)
	defer cancel()

	report, err := h.svc.Coverage(ctx)
	if err != nil {
		return h.mcpFailure(ctx, "get_coverage", err), nil
	}
	h.logMCPCall(ctx, "get_coverage", "ok")
	return mcpOK(report), nil
}

func (h *Handler) toolGetActivity(ctx context.Context, _ *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if _, ok := mcpCallerFrom(ctx); !ok {
		return mcpNoCaller(), nil
	}
	snap := h.svc.ActivitySnapshot()
	// The progress store hands its entries out in map order, and an assistant
	// comparing two readings should not see them shuffle.
	slices.SortFunc(snap.Items, func(a, b ActivityItem) int {
		return cmp.Or(cmp.Compare(a.Domain, b.Domain), cmp.Compare(a.Item, b.Item))
	})

	running := make([]map[string]any, 0, len(snap.Items))
	for _, item := range snap.Items {
		row := map[string]any{
			"domain":    item.Domain,
			"item":      item.Item,
			"phase":     item.Phase,
			"percent":   item.Percent,
			"startedAt": item.StartedAt,
		}
		if id := h.runningRunID(item); id != "" {
			row["runId"] = id
		}
		running = append(running, row)
	}

	h.logMCPCall(ctx, "get_activity", "ok")
	return mcpOK(map[string]any{
		"running":           running,
		"domainsBusy":       snap.DomainsBusy,
		"backupRunning":     snap.BackupRunning,
		"everythingRunning": snap.EverythingRunning,
	}), nil
}

// runningRunID is the id of the run an activity entry belongs to, so an
// assistant can name the run it is reporting on. It is "" whenever the entry
// names nothing with a run row of its own, such as a batch.
func (h *Handler) runningRunID(item ActivityItem) string {
	targetID := ""
	switch item.Domain {
	case "containers":
		if tg, err := h.store.GetTargetByContainer(item.Item); err == nil {
			targetID = tg.ID
		}
	case "vms":
		if vm, err := h.store.GetVMTargetByName(item.Item); err == nil {
			targetID = vm.ID
		}
	case "files":
		if set, err := h.store.GetFileSetByName(item.Item); err == nil {
			targetID = set.ID
		}
	case zfsDomain:
		if d, err := h.store.GetZFSDatasetByName(item.Item); err == nil {
			targetID = d.ID
		}
	case "flash":
		targetID = store.FlashTargetID
	case "config":
		targetID = store.ConfigTargetID
	}
	if targetID == "" {
		return ""
	}
	run, err := h.store.LastRunForTarget(targetID)
	if err != nil || run == nil || run.Status != "running" {
		return ""
	}
	return run.ID
}

type storageStatsInput struct {
	Domain string `json:"domain"`
	Limit  *int   `json:"limit"`
}

func (h *Handler) toolGetStorageStats(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if _, ok := mcpCallerFrom(ctx); !ok {
		return mcpNoCaller(), nil
	}
	var in storageStatsInput
	if err := decodeMCPArgs(req.Params.Arguments, &in); err != nil {
		h.logMCPCall(ctx, "get_storage_stats", "invalid_argument")
		return mcpToolError("invalid_argument", err.Error(), nil), nil
	}
	if !slices.Contains(mcpDomains, in.Domain) {
		h.logMCPCall(ctx, "get_storage_stats", "invalid_argument")
		return mcpToolError("invalid_argument", "domain must be one of "+strings.Join(mcpDomains, ", "), nil), nil
	}
	limit := mcpStatsLimitDefault
	if in.Limit != nil {
		limit = *in.Limit
		if limit < 1 || limit > mcpStatsLimitMax {
			h.logMCPCall(ctx, "get_storage_stats", "invalid_argument")
			return mcpToolError("invalid_argument", fmt.Sprintf("limit must be between 1 and %d", mcpStatsLimitMax), nil), nil
		}
	}

	stats, err := h.svc.RepoStats(in.Domain, "local", limit)
	if err != nil {
		return h.mcpFailure(ctx, "get_storage_stats", err), nil
	}

	var growth any
	if rate, ok := growthBytesPerWeek(stats, time.Now()); ok {
		growth = rate
	}
	samples := make([]map[string]any, 0, len(stats))
	for i := len(stats) - 1; i >= 0; i-- {
		samples = append(samples, map[string]any{
			"at":          stats[i].At,
			"rawSize":     stats[i].RawSize,
			"restoreSize": stats[i].RestoreSize,
			"snapshots":   stats[i].Snapshots,
		})
	}

	h.logMCPCall(ctx, "get_storage_stats", "ok")
	return mcpOK(map[string]any{
		"domain":             in.Domain,
		"samples":            samples,
		"growthBytesPerWeek": growth,
	}), nil
}

// mcpStops is what a backup of one item takes down, read from stored settings
// and the live Docker state. Known is false when Docker could not be asked, so
// an assistant does not read silence as "nothing is stopped".
type mcpStops struct {
	Self       bool     `json:"self"`
	Containers []string `json:"containers"`
	Known      bool     `json:"known"`
}

// mcpLastDump is the newest database dump attempt of a container.
type mcpLastDump struct {
	At     int64  `json:"at"`
	Status string `json:"status"`
}

// mcpDatabase is the dump picture of a container that holds a database.
type mcpDatabase struct {
	Engine      string       `json:"engine"`
	EngineKnown bool         `json:"engineKnown"`
	DumpOff     bool         `json:"dumpOff"`
	LastDump    *mcpLastDump `json:"lastDump,omitempty"`
}

// mcpItemView is one protected thing as list_items reports it. Installed is a
// pointer because "not installed" and "nobody could ask Docker" are different
// answers, and only containers have either. LastCheckCode is a pointer for the
// same reason: a ZFS item that was never checked reports an empty code, and no
// other item has a check at all.
type mcpItemView struct {
	ID                  string       `json:"id"`
	Name                string       `json:"name"`
	Installed           *bool        `json:"installed,omitempty"`
	Included            bool         `json:"included"`
	Paused              bool         `json:"paused"`
	Schedule            string       `json:"schedule"`
	LastCheckCode       *string      `json:"lastCheckCode,omitempty"`
	Stops               mcpStops     `json:"stops"`
	LastDurationSeconds int64        `json:"lastDurationSeconds"`
	LastSuccessAt       int64        `json:"lastSuccessAt"`
	LastRunAt           int64        `json:"lastRunAt"`
	LastRunStatus       string       `json:"lastRunStatus"`
	Database            *mcpDatabase `json:"database,omitempty"`
}

func (v *mcpItemView) stamp(s store.BackupStamp) {
	v.LastSuccessAt = s.LastSuccessAt
	v.LastDurationSeconds = s.LastDurationSeconds
	v.LastRunAt = s.LastRunAt
	v.LastRunStatus = s.LastRunStatus
}

type listItemsInput struct {
	Domain string `json:"domain"`
}

func (h *Handler) toolListItems(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if _, ok := mcpCallerFrom(ctx); !ok {
		return mcpNoCaller(), nil
	}
	var in listItemsInput
	if err := decodeMCPArgs(req.Params.Arguments, &in); err != nil {
		h.logMCPCall(ctx, "list_items", "invalid_argument")
		return mcpToolError("invalid_argument", err.Error(), nil), nil
	}
	if in.Domain != "" && !slices.Contains(mcpDomains, in.Domain) {
		h.logMCPCall(ctx, "list_items", "invalid_argument")
		return mcpToolError("invalid_argument", "domain must be one of "+strings.Join(mcpDomains, ", "), nil), nil
	}
	ctx, cancel := h.mcpToolContext(ctx, mcpReadTimeout)
	defer cancel()

	settings, err := h.store.GetSettings()
	if err != nil {
		h.logMCPCall(ctx, "list_items", "unavailable")
		return mcpToolError("unavailable", "settings could not be read", nil), nil
	}
	items, known, err := h.mcpItems(ctx, settings, in.Domain)
	if err != nil {
		return h.mcpFailure(ctx, "list_items", err), nil
	}

	domains := make([]map[string]any, 0, len(mcpDomains))
	for _, domain := range mcpDomains {
		if in.Domain != "" && domain != in.Domain {
			continue
		}
		rows := items[domain]
		truncated := len(rows) > mcpItemsPerDomain
		if truncated {
			rows = rows[:mcpItemsPerDomain]
		}
		row := map[string]any{
			"domain":    domain,
			"enabled":   domainEnabled(settings, domain),
			"truncated": truncated,
			"items":     rows,
		}
		if domain == "containers" {
			row["installedKnown"] = known[domain]
		}
		domains = append(domains, row)
	}

	h.logMCPCall(ctx, "list_items", "ok")
	return mcpOK(map[string]any{"domains": domains}), nil
}

// mcpItems collects the protected things of every domain the caller asked for
// ("" means all) and reports per domain whether the live state behind them
// could be read. A switched-off domain is skipped: nothing backs its items up,
// so nothing in it is a protected thing.
func (h *Handler) mcpItems(ctx context.Context, settings store.Settings, domain string) (map[string][]mcpItemView, map[string]bool, error) {
	items := map[string][]mcpItemView{}
	for _, d := range mcpDomains {
		if domain == "" || d == domain {
			items[d] = []mcpItemView{}
		}
	}
	// Without container rows, nothing in the listing rests on Docker.
	known := map[string]bool{"containers": true}

	stamps, err := h.store.LatestBackupsByTarget()
	if err != nil {
		return nil, nil, err
	}
	live := h.mcpDockerOnce(ctx)

	if _, want := items["containers"]; want && settings.ContainersEnabled {
		rows, dockerAnswered, cErr := h.mcpContainerItems(ctx, settings, stamps, live)
		if cErr != nil {
			return nil, nil, cErr
		}
		items["containers"], known["containers"] = rows, dockerAnswered
	}
	if _, want := items["vms"]; want && settings.VMsEnabled {
		vms, vErr := h.store.ListVMTargets()
		if vErr != nil {
			return nil, nil, vErr
		}
		rows := make([]mcpItemView, 0, len(vms))
		for _, vm := range vms {
			view := mcpItemView{
				ID:       vm.ID,
				Name:     vm.Name,
				Included: vm.IncludeInSchedule,
				Paused:   schedule.PausedByOverride(vm.ScheduleCadence, settings.PerItemSchedules),
				Schedule: schedule.EffectiveVMSchedule(vm, settings).Kind,
				Stops:    mcpStops{Self: vm.Method != "live", Containers: []string{}, Known: true},
			}
			view.stamp(stamps[vm.ID])
			rows = append(rows, view)
		}
		items["vms"] = rows
	}
	if _, want := items["files"]; want && settings.FilesEnabled {
		sets, fErr := h.store.ListFileSets()
		if fErr != nil {
			return nil, nil, fErr
		}
		rows := make([]mcpItemView, 0, len(sets))
		for _, set := range sets {
			view := mcpItemView{
				ID:       set.ID,
				Name:     set.Name,
				Included: set.Enabled,
				Paused:   schedule.PausedByOverride(set.ScheduleCadence, settings.PerItemSchedules),
				Schedule: schedule.EffectiveFileSetSchedule(set, settings).Kind,
				Stops:    mcpStops{Containers: []string{}, Known: true},
			}
			view.stamp(stamps[set.ID])
			rows = append(rows, view)
		}
		items["files"] = rows
	}
	if _, want := items[zfsDomain]; want && settings.ZFSEnabled {
		rows, zErr := h.mcpZFSItems(settings, stamps, live)
		if zErr != nil {
			return nil, nil, zErr
		}
		items[zfsDomain] = rows
	}
	if _, want := items["flash"]; want && settings.FlashEnabled {
		items["flash"] = []mcpItemView{mcpSingletonItem("flash", settings.FlashSchedule, settings.EverythingSchedule, stamps)}
	}
	if _, want := items["config"]; want && settings.ConfigEnabled {
		items["config"] = []mcpItemView{mcpSingletonItem("config", settings.ConfigSchedule, settings.EverythingSchedule, stamps)}
	}
	return items, known, nil
}

// mcpContainerItems lists the container targets with their live state. A Docker
// failure leaves the rows in place without an installed flag: the stored items
// are still what an operator asks about, and dropping them because the socket
// was busy would read as "BombVault protects nothing".
func (h *Handler) mcpContainerItems(ctx context.Context, settings store.Settings, stamps map[string]store.BackupStamp, docker func() mcpDockerState) ([]mcpItemView, bool, error) {
	targets, err := h.store.ListTargets()
	if err != nil {
		return nil, false, err
	}
	dumps, err := h.store.LastRunsOfKind("dbdump")
	if err != nil {
		return nil, false, err
	}

	state := docker()
	live, dockerAnswered := state.live, state.answered
	byName := make(map[string]store.Target, len(targets))
	for _, t := range targets {
		byName[t.ContainerName] = t
	}
	dbRows := h.svc.dbDumpRows(ctx, state.infos, byName)
	self := h.svc.SelfContainerName(ctx)

	rows := make([]mcpItemView, 0, len(targets))
	for _, t := range targets {
		if self != "" && t.ContainerName == self {
			continue
		}
		view := mcpItemView{
			ID:       t.ID,
			Name:     t.ContainerName,
			Included: t.IncludeInSchedule,
			Paused:   schedule.PausedByOverride(t.ScheduleCadence, settings.PerItemSchedules),
			Schedule: schedule.EffectiveContainerSchedule(t, settings).Kind,
			Stops:    mcpStops{Containers: []string{}, Known: dockerAnswered},
		}
		if dockerAnswered {
			c, installed := live[t.ContainerName]
			view.Installed = &installed
			view.Stops.Self = isRunning(c)
			for _, name := range t.StopContainers {
				if isRunning(live[name]) {
					view.Stops.Containers = append(view.Stops.Containers, name)
				}
			}
		}
		view.stamp(stamps[t.ID])
		if db := mcpDatabaseOf(t, dbRows[t.ContainerName], dockerAnswered); db != nil {
			if dump, ok := dumps[t.ID]; ok {
				at := dump.StartedAt
				if dump.FinishedAt != nil {
					at = *dump.FinishedAt
				}
				db.LastDump = &mcpLastDump{At: at, Status: dump.Status}
			}
			view.Database = db
		}
		rows = append(rows, view)
	}
	return rows, dockerAnswered, nil
}

func isRunning(c dockercli.ContainerInfo) bool {
	return strings.EqualFold(c.State, "running")
}

// mcpDockerState is the container list one call works from, and whether Docker
// gave it.
type mcpDockerState struct {
	infos    []dockercli.ContainerInfo
	live     map[string]dockercli.ContainerInfo
	answered bool
}

// mcpDockerOnce asks Docker for its containers the first time a domain needs
// them and hands every later domain the same answer, so one listing costs one
// Docker call however many domains read what is running.
func (h *Handler) mcpDockerOnce(ctx context.Context) func() mcpDockerState {
	var state *mcpDockerState
	return func() mcpDockerState {
		if state != nil {
			return *state
		}
		infos, err := h.docker.List(ctx)
		if err != nil {
			log.Printf("api: mcp: list_items: the container list is unavailable: %v", err)
		}
		live := make(map[string]dockercli.ContainerInfo, len(infos))
		for _, c := range infos {
			live[c.Name] = c
		}
		state = &mcpDockerState{infos: infos, live: live, answered: err == nil}
		return *state
	}
}

// mcpZFSItems lists the ZFS items off their stored rows. The host is never
// asked: it may be switched off, and the rows already hold everything the
// listing says. Docker is, but only when an item stops containers.
func (h *Handler) mcpZFSItems(settings store.Settings, stamps map[string]store.BackupStamp, docker func() mcpDockerState) ([]mcpItemView, error) {
	datasets, err := h.store.ListZFSDatasets()
	if err != nil {
		return nil, err
	}
	rows := make([]mcpItemView, 0, len(datasets))
	for _, d := range datasets {
		code := d.LastCheckCode
		view := mcpItemView{
			ID:            d.ID,
			Name:          d.Dataset,
			Included:      d.Enabled,
			Paused:        schedule.PausedByOverride(d.ScheduleCadence, settings.PerItemSchedules),
			Schedule:      schedule.EffectiveZFSDatasetSchedule(d, settings).Kind,
			LastCheckCode: &code,
			Stops:         mcpStops{Containers: []string{}, Known: true},
		}
		if len(d.StopContainers) > 0 {
			state := docker()
			view.Stops.Known = state.answered
			for _, name := range d.StopContainers {
				if isRunning(state.live[name]) {
					view.Stops.Containers = append(view.Stops.Containers, name)
				}
			}
		}
		view.stamp(stamps[d.ID])
		rows = append(rows, view)
	}
	return rows, nil
}

// mcpDatabaseOf is the database block of a container, or nil for one nothing
// says holds a database. The operator's stored engine outranks the recognition,
// and with Docker unreachable the row's own dump switch is the only evidence
// left that there is a database at all.
func mcpDatabaseOf(t store.Target, row dbDumpRow, dockerAnswered bool) *mcpDatabase {
	switch {
	case t.DBDumpEngine != "":
		return &mcpDatabase{Engine: t.DBDumpEngine, EngineKnown: true, DumpOff: t.DBDumpOff}
	case row.Tier != "":
		return &mcpDatabase{Engine: row.Engine, EngineKnown: true, DumpOff: t.DBDumpOff}
	case !dockerAnswered && t.DBDumpOff:
		return &mcpDatabase{DumpOff: true}
	}
	return nil
}

// mcpSingletonItem is the one item of the flash or config domain, neither of
// which has rows of its own.
func mcpSingletonItem(domain, own, everything string, stamps map[string]store.BackupStamp) mcpItemView {
	view := mcpItemView{
		ID:       domainRunTargetID(domain),
		Name:     domain,
		Included: true,
		Schedule: domainScheduleKind(own, everything),
		Stops:    mcpStops{Containers: []string{}, Known: true},
	}
	view.stamp(stamps[view.ID])
	return view
}

// domainScheduleKind answers for a whole domain what EffectiveSchedule answers
// for an item, off the same cadences domainStatusFrom reads.
func domainScheduleKind(own, everything string) string {
	switch {
	case cadencePeriodSeconds(own) > 0:
		return schedule.EffectiveDomain
	case cadencePeriodSeconds(everything) > 0:
		return schedule.EffectiveEverything
	default:
		return schedule.EffectiveNone
	}
}

// mcpItem is one resolved thing a tool was asked about.
type mcpItem struct{ Domain, ID, Name string }

// resolveMCPItem turns a domain and the name or id an assistant passed into the
// stored item, refusing before the request reaches restic. The second return
// value is the tool error to answer with, nil when the item resolved.
func (h *Handler) resolveMCPItem(domain, item string) (mcpItem, *mcp.CallToolResult) {
	switch domain {
	case "containers":
		if !validResourceName(item) {
			return mcpItem{}, mcpToolError("invalid_argument", "item must be the name of a container", nil)
		}
		t, err := h.store.GetTargetByContainer(item)
		if err != nil {
			return mcpItem{}, mcpToolError("not_found", "BombVault does not protect a container called "+item+"; it has to be added in the web interface first", nil)
		}
		return mcpItem{Domain: domain, ID: t.ID, Name: t.ContainerName}, nil
	case "vms":
		if !validVMName(item) {
			return mcpItem{}, mcpToolError("invalid_argument", "item must be the name of a VM", nil)
		}
		vm, err := h.store.GetVMTargetByName(item)
		if err != nil {
			return mcpItem{}, mcpToolError("not_found", "BombVault does not protect a VM called "+item+"; it has to be added in the web interface first", nil)
		}
		return mcpItem{Domain: domain, ID: vm.ID, Name: vm.Name}, nil
	case "files":
		return h.resolveMCPFileSet(item)
	case zfsDomain:
		return h.resolveMCPDataset(item)
	case "flash", "config":
		if item != "" && item != domain {
			return mcpItem{}, mcpToolError("invalid_argument", "the "+domain+" domain holds a single item, called "+domain, nil)
		}
		return mcpItem{Domain: domain, ID: domainRunTargetID(domain), Name: domain}, nil
	}
	return mcpItem{}, mcpToolError("invalid_argument", "domain must be one of "+strings.Join(mcpDomains, ", "), nil)
}

// resolveMCPFileSet finds a folder set by its id, its exact name, or a name
// that differs only in case. Two sets whose names differ only in case are legal,
// so that last step can be ambiguous and says so instead of picking one.
func (h *Handler) resolveMCPFileSet(item string) (mcpItem, *mcp.CallToolResult) {
	sets, err := h.store.ListFileSets()
	if err != nil {
		return mcpItem{}, mcpServiceError(err)
	}
	notFound := mcpToolError("not_found", "BombVault has no folder set called "+item, nil)
	if runIDRe.MatchString(item) {
		for _, set := range sets {
			if set.ID == item {
				return mcpItem{Domain: "files", ID: set.ID, Name: set.Name}, nil
			}
		}
		return mcpItem{}, notFound
	}

	var matches []store.FileSet
	for _, set := range sets {
		if set.Name == item {
			matches = []store.FileSet{set}
			break
		}
		if strings.EqualFold(set.Name, item) {
			matches = append(matches, set)
		}
	}
	switch len(matches) {
	case 0:
		return mcpItem{}, notFound
	case 1:
		return mcpItem{Domain: "files", ID: matches[0].ID, Name: matches[0].Name}, nil
	}
	candidates := make([]map[string]any, 0, len(matches))
	for _, set := range matches {
		candidates = append(candidates, map[string]any{"id": set.ID, "name": set.Name})
	}
	return mcpItem{}, mcpToolError("ambiguous", "several folder sets are called "+item+"; pass the id of the one you mean",
		map[string]any{"candidates": candidates})
}

// resolveMCPDataset finds a ZFS item by its id or by the dataset at its root. A
// dataset on the host that nobody added is not an item, and MCP adds none.
func (h *Handler) resolveMCPDataset(item string) (mcpItem, *mcp.CallToolResult) {
	var d store.ZFSDataset
	var err error
	if runIDRe.MatchString(item) {
		d, err = h.store.GetZFSDataset(item)
	} else {
		if zfs.ValidateDatasetName(item) != nil {
			return mcpItem{}, mcpToolError("invalid_argument", "item must be the id or the name of a ZFS dataset, such as cache/appdata", nil)
		}
		d, err = h.store.GetZFSDatasetByName(item)
	}
	if err != nil {
		return mcpItem{}, mcpToolError("not_found", "BombVault does not protect a ZFS dataset called "+item+"; it has to be added in the web interface first", nil)
	}
	return mcpItem{Domain: zfsDomain, ID: d.ID, Name: d.Dataset}, nil
}

// mcpRunStatuses are the states a run row can be in.
var mcpRunStatuses = []string{"running", "success", "failed", "cancelled", "skipped"}

// mcpRunDomains is the domain vocabulary of a run: the item domains plus the
// Backup Everything pass, which owns runs but no items.
var mcpRunDomains = append(append([]string{}, mcpDomains...), "everything")

type listRunsInput struct {
	Limit  *int   `json:"limit"`
	Domain string `json:"domain"`
	Item   string `json:"item"`
	Status string `json:"status"`
	Kind   string `json:"kind"`
	Since  int64  `json:"since"`
}

func (h *Handler) toolListRuns(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if _, ok := mcpCallerFrom(ctx); !ok {
		return mcpNoCaller(), nil
	}
	var in listRunsInput
	if err := decodeMCPArgs(req.Params.Arguments, &in); err != nil {
		h.logMCPCall(ctx, "list_runs", "invalid_argument")
		return mcpToolError("invalid_argument", err.Error(), nil), nil
	}
	limit := mcpRunsLimitDefault
	if in.Limit != nil {
		limit = *in.Limit
	}
	refuse := func(msg string) (*mcp.CallToolResult, error) {
		h.logMCPCall(ctx, "list_runs", "invalid_argument")
		return mcpToolError("invalid_argument", msg, nil), nil
	}
	switch {
	case limit < 1 || limit > mcpRunsLimitMax:
		return refuse(fmt.Sprintf("limit must be between 1 and %d", mcpRunsLimitMax))
	case in.Domain != "" && !slices.Contains(mcpRunDomains, in.Domain):
		return refuse("domain must be one of " + strings.Join(mcpRunDomains, ", "))
	case in.Item != "" && in.Domain == "":
		return refuse("item needs the domain it belongs to")
	case in.Status != "" && !slices.Contains(mcpRunStatuses, in.Status):
		return refuse("status must be one of " + strings.Join(mcpRunStatuses, ", "))
	case in.Kind != "" && !slices.Contains(digestKindOrder, in.Kind):
		return refuse("kind must be one of " + strings.Join(digestKindOrder, ", "))
	case in.Since < 0:
		return refuse("since must be a unix time in seconds")
	}

	// One row over the limit, so the answer can say whether there is more
	// without a second query.
	filter := store.RunFilter{Limit: limit + 1, Since: in.Since}
	if in.Status != "" {
		filter.Statuses = []string{in.Status}
	}
	if in.Kind != "" {
		filter.Kinds = []string{in.Kind}
	}
	switch {
	case in.Item != "":
		item, bad := h.resolveMCPItem(in.Domain, in.Item)
		if bad != nil {
			h.logMCPCall(ctx, "list_runs", mcpErrorCodeOf(bad))
			return bad, nil
		}
		filter.TargetIDs = []string{item.ID}
	case in.Domain != "":
		ids, err := h.mcpDomainTargetIDs(in.Domain)
		if err != nil {
			return h.mcpFailure(ctx, "list_runs", err), nil
		}
		filter.TargetIDs = ids
	}

	runs, err := h.store.ListRunsFiltered(filter)
	if err != nil {
		return h.mcpFailure(ctx, "list_runs", err), nil
	}
	truncated := len(runs) > limit
	if truncated {
		runs = runs[:limit]
	}

	views := h.runViews(runs)
	rows := make([]map[string]any, 0, len(views))
	for _, view := range views {
		rows = append(rows, mcpRunRow(view))
	}

	h.logMCPCall(ctx, "list_runs", "ok")
	return mcpOK(map[string]any{"runs": rows, "truncated": truncated}), nil
}

// mcpDomainTargetIDs is every target id a domain's runs can be stored under:
// its current items plus the literal id that carries the domain's own prune,
// verify, off-site, tamper and drill rows.
func (h *Handler) mcpDomainTargetIDs(domain string) ([]string, error) {
	ids := []string{domainRunTargetID(domain)}
	switch domain {
	case "containers":
		targets, err := h.store.ListTargets()
		if err != nil {
			return nil, err
		}
		for _, t := range targets {
			ids = append(ids, t.ID)
		}
	case "vms":
		vms, err := h.store.ListVMTargets()
		if err != nil {
			return nil, err
		}
		for _, vm := range vms {
			ids = append(ids, vm.ID)
		}
	case "files":
		sets, err := h.store.ListFileSets()
		if err != nil {
			return nil, err
		}
		for _, set := range sets {
			ids = append(ids, set.ID)
		}
	case zfsDomain:
		datasets, err := h.store.ListZFSDatasets()
		if err != nil {
			return nil, err
		}
		for _, d := range datasets {
			ids = append(ids, d.ID)
		}
	}
	return ids, nil
}

// mcpRestorePoint is one restic snapshot as a tool reports it. The paths and
// the host name restic also stores describe this machine and stay here.
type mcpRestorePoint struct {
	ID      string `json:"id"`
	ShortID string `json:"shortId"`
	Time    string `json:"time"`
}

// mcpDatabaseDump is one database dump of a container. Damaged marks a dump a
// failed run left behind, which the web interface offers delete for and nothing
// else.
type mcpDatabaseDump struct {
	ID               string   `json:"id"`
	ShortID          string   `json:"shortId"`
	Time             string   `json:"time"`
	Engine           string   `json:"engine"`
	Version          string   `json:"version"`
	Databases        []string `json:"databases"`
	Bytes            int64    `json:"bytes"`
	Damaged          bool     `json:"damaged"`
	PairedSnapshotID string   `json:"pairedSnapshotId,omitempty"`
}

type listRestorePointsInput struct {
	Domain    string `json:"domain"`
	Item      string `json:"item"`
	Limit     *int   `json:"limit"`
	DumpLimit *int   `json:"dumpLimit"`
}

func (h *Handler) toolListRestorePoints(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if _, ok := mcpCallerFrom(ctx); !ok {
		return mcpNoCaller(), nil
	}
	var in listRestorePointsInput
	if err := decodeMCPArgs(req.Params.Arguments, &in); err != nil {
		h.logMCPCall(ctx, "list_restore_points", "invalid_argument")
		return mcpToolError("invalid_argument", err.Error(), nil), nil
	}
	limit := mcpPointsLimitDefault
	if in.Limit != nil {
		limit = *in.Limit
	}
	dumpLimit := mcpDumpLimitDefault
	if in.DumpLimit != nil {
		dumpLimit = *in.DumpLimit
	}
	refuse := func(msg string) (*mcp.CallToolResult, error) {
		h.logMCPCall(ctx, "list_restore_points", "invalid_argument")
		return mcpToolError("invalid_argument", msg, nil), nil
	}
	switch {
	case !slices.Contains(mcpDomains, in.Domain):
		return refuse("domain must be one of " + strings.Join(mcpDomains, ", "))
	case limit < 1 || limit > mcpPointsLimitMax:
		return refuse(fmt.Sprintf("limit must be between 1 and %d", mcpPointsLimitMax))
	case dumpLimit < 1 || dumpLimit > mcpPointsLimitMax:
		return refuse(fmt.Sprintf("dumpLimit must be between 1 and %d", mcpPointsLimitMax))
	}

	item, bad := h.resolveMCPItem(in.Domain, in.Item)
	if bad != nil {
		h.logMCPCall(ctx, "list_restore_points", mcpErrorCodeOf(bad))
		return bad, nil
	}
	settings, err := h.store.GetSettings()
	if err != nil {
		h.logMCPCall(ctx, "list_restore_points", "unavailable")
		return mcpToolError("unavailable", "settings could not be read", nil), nil
	}

	release, free := h.mcp.acquireList()
	if !free {
		h.logMCPCall(ctx, "list_restore_points", "busy")
		return mcpToolError("busy", "another restore point listing is still running; try again in a moment", nil), nil
	}
	defer release()

	ctx, cancel := h.mcpToolContext(ctx, mcpResticTimeout)
	defer cancel()

	var points any
	var total int
	if item.Domain == zfsDomain {
		zfsPoints, zErr := h.svc.ListZFSRestorePoints(ctx, item.ID, "local")
		if zErr != nil {
			return h.mcpListingFailed(ctx, zErr), nil
		}
		total = len(zfsPoints)
		points = mcpZFSRestorePointsOf(zfsPoints[:min(total, limit)])
	} else {
		snaps, sErr := h.mcpSnapshotsOf(ctx, item)
		if sErr != nil {
			return h.mcpListingFailed(ctx, sErr), nil
		}
		all := mcpRestorePointsOf(snaps)
		total = len(all)
		points = all[:min(total, limit)]
	}

	repoItem := item.Name
	if item.Domain == "files" || item.Domain == zfsDomain {
		repoItem = item.ID
	}
	out := map[string]any{
		"domain":        item.Domain,
		"item":          map[string]any{"id": item.ID, "name": item.Name},
		"repository":    "primary",
		"remote":        h.svc.primaryRepoIsRemote(settings, item.Domain, repoItem),
		"total":         total,
		"truncated":     total > limit,
		"restorePoints": points,
	}
	if item.Domain == "containers" {
		views, dErr := h.svc.DBDumps(ctx, item.Name, "local")
		if dErr != nil {
			return h.mcpListingFailed(ctx, dErr), nil
		}
		dumps := mcpDatabaseDumpsOf(views)
		out["databaseDumpsTotal"] = len(dumps)
		out["databaseDumpsTruncated"] = len(dumps) > dumpLimit
		if len(dumps) > dumpLimit {
			dumps = dumps[:dumpLimit]
		}
		out["databaseDumps"] = dumps
	}

	h.logMCPCall(ctx, "list_restore_points", "ok")
	return mcpOK(out), nil
}

// mcpSnapshotsOf lists an item's restore points in its primary repository. A
// ZFS item is not listed here: its snapshots only mean something grouped.
func (h *Handler) mcpSnapshotsOf(ctx context.Context, item mcpItem) ([]restic.Snapshot, error) {
	switch item.Domain {
	case "containers":
		return h.svc.Snapshots(ctx, item.Name, "local")
	case "vms":
		return h.svc.SnapshotsVM(ctx, item.Name, "local")
	case "files":
		return h.svc.SnapshotsFileSet(ctx, item.ID, "local")
	case "flash":
		return h.svc.SnapshotsFlash(ctx, "local")
	case "config":
		return h.svc.SnapshotsConfig(ctx, "local")
	default:
		return nil, fmt.Errorf("unknown domain %q", item.Domain)
	}
}

// mcpListingFailed answers a listing that did not come back. A deadline the
// tool set itself reads as a timeout whatever restic reported on its way out.
// A cancellation does not: the client hung up, the repository is not slow, and
// the underlying error is the one worth reporting.
func (h *Handler) mcpListingFailed(ctx context.Context, err error) *mcp.CallToolResult {
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		h.logMCPCall(ctx, "list_restore_points", "timeout")
		return mcpToolError("timeout", "BombVault did not finish reading the repository in time", nil)
	}
	return h.mcpFailure(ctx, "list_restore_points", err)
}

// mcpRestorePointsOf slims restic's snapshots down to what an assistant may
// see, newest first.
func mcpRestorePointsOf(snaps []restic.Snapshot) []mcpRestorePoint {
	out := make([]mcpRestorePoint, 0, len(snaps))
	for _, sn := range snaps {
		out = append(out, mcpRestorePoint{ID: sn.ID, ShortID: shortID(sn.ID), Time: sn.Time})
	}
	slices.SortStableFunc(out, func(a, b mcpRestorePoint) int {
		return parseSnapshotTime(b.Time).Compare(parseSnapshotTime(a.Time))
	})
	return out
}

// mcpZFSRestorePoint is one moment of a ZFS item: the snapshots one backup took
// of the datasets below it. A member's place under the item's mount is a path on
// this host and stays here.
type mcpZFSRestorePoint struct {
	Stamp   string         `json:"stamp"`
	Time    string         `json:"time"`
	Members []mcpZFSMember `json:"members"`
}

// mcpZFSMember is one dataset of a ZFS restore point. A member the backup did
// not read has no snapshot, and its outcome says why.
type mcpZFSMember struct {
	Dataset string `json:"dataset"`
	ID      string `json:"id"`
	ShortID string `json:"shortId"`
	Outcome string `json:"outcome"`
}

// mcpZFSRestorePointsOf keeps a ZFS item's restore points grouped the way the
// service hands them over, newest first. Flattened, the same moment would stand
// in the list once for every dataset below the item.
func mcpZFSRestorePointsOf(points []ZFSRestorePoint) []mcpZFSRestorePoint {
	out := make([]mcpZFSRestorePoint, 0, len(points))
	for _, p := range points {
		at := ""
		if p.Time > 0 {
			at = time.Unix(p.Time, 0).UTC().Format(time.RFC3339)
		}
		members := make([]mcpZFSMember, 0, len(p.Members))
		for _, m := range p.Members {
			members = append(members, mcpZFSMember{
				Dataset: m.Dataset,
				ID:      m.SnapshotID,
				ShortID: shortID(m.SnapshotID),
				Outcome: m.Outcome,
			})
		}
		out = append(out, mcpZFSRestorePoint{Stamp: p.Stamp, Time: at, Members: members})
	}
	return out
}

// mcpDatabaseDumpsOf is mcpRestorePointsOf for the dump series of a container,
// which DBDumps already hands over newest first.
func mcpDatabaseDumpsOf(views []DBDumpView) []mcpDatabaseDump {
	out := make([]mcpDatabaseDump, 0, len(views))
	for _, v := range views {
		out = append(out, mcpDatabaseDump{
			ID:               v.ID,
			ShortID:          shortID(v.ID),
			Time:             v.Time,
			Engine:           v.Engine,
			Version:          v.Version,
			Databases:        v.Databases,
			Bytes:            v.Bytes,
			Damaged:          v.Damaged,
			PairedSnapshotID: v.PairedSnapshotID,
		})
	}
	return out
}

// mcpRunRow is one run in the shape a tool answers with: no snapshot id, no
// host paths and no database tool output in the error, and a domain an
// assistant can pass back in. The web
// interface reads a domain operation by its target id, so a row that carries no
// item domain takes that literal id as its domain here.
func mcpRunRow(v runView) map[string]any {
	domain := mcpDomainOut(v.Domain)
	if domain == "" && slices.Contains(mcpRunDomains, v.TargetID) {
		domain = v.TargetID
	}
	var finished int64
	if v.FinishedAt != nil {
		finished = *v.FinishedAt
	}
	return map[string]any{
		"id":              v.ID,
		"domain":          domain,
		"itemId":          v.TargetID,
		"itemName":        v.Target,
		"kind":            v.Kind,
		"status":          v.Status,
		"startedAt":       v.StartedAt,
		"finishedAt":      finished,
		"bytes":           v.Bytes,
		"error":           mcpScrubText(shareableRunError(v.Kind, v.Error)),
		"acknowledged":    v.Acknowledged,
		"groupId":         v.GroupID,
		"startedVia":      v.StartedVia,
		"startedViaLabel": v.StartedViaLabel,
	}
}
