package api

import (
	"context"
	"fmt"
	"math"
	"slices"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/junkerderprovinz/bombvault/internal/dockercli"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/schedule"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// mcpStartItem is one item a start tool acted on. Stops and the last duration
// are what an assistant tells the user before the apps go down; the Backup
// Everything pass lists domains rather than items and carries neither.
type mcpStartItem struct {
	ID                  string    `json:"id"`
	Name                string    `json:"name"`
	Stops               *mcpStops `json:"stops,omitempty"`
	LastDurationSeconds int64     `json:"lastDurationSeconds,omitempty"`
}

// mcpSkipped is one item a start left out, with the reason an assistant can
// pass on: no_folder or retention_guard.
type mcpSkipped struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Reason string `json:"reason"`
}

const (
	mcpReadOnlyKeyMessage = "this key may only read; allow backups for it under Settings > System > MCP server"
	mcpItemFollowUp       = "Call get_activity for progress and list_runs with this domain and item for the result."
	mcpEverythingFollowUp = "Call get_activity for progress and list_runs with domain everything for the result."
)

type startBackupInput struct {
	Domain string `json:"domain"`
	Item   string `json:"item"`
}

func (h *Handler) toolStartBackup(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	const tool = "start_backup"
	caller, ok := mcpCallerFrom(ctx)
	if !ok {
		return mcpNoCaller(), nil
	}
	var in startBackupInput
	if err := decodeMCPArgs(req.Params.Arguments, &in); err != nil {
		h.logMCPCall(ctx, tool, "invalid_argument")
		return mcpToolError("invalid_argument", err.Error(), nil), nil
	}
	settings, refusal := h.mcpStartAllowed(ctx, tool, caller, in.Domain)
	if refusal != nil {
		return refusal, nil
	}
	ctx, cancel := h.mcpToolContext(ctx, mcpReadTimeout)
	defer cancel()

	item, bad := h.resolveMCPItem(in.Domain, in.Item)
	if bad != nil {
		h.logMCPCall(ctx, tool, "refused")
		return bad, nil
	}
	if in.Domain == "containers" && item.Name == h.svc.SelfContainerName(ctx) {
		h.logMCPCall(ctx, tool, "invalid_argument")
		return mcpToolError("invalid_argument", item.Name+" is the container BombVault runs in, and it cannot back itself up", nil), nil
	}

	now := h.mcp.now()
	release, refusal := h.mcpStartPreflight(ctx, tool, caller.KeyID, item.Name, []string{item.ID}, now)
	if refusal != nil {
		return refusal, nil
	}
	hold, err := h.mcpRetentionHold(settings, item, now)
	if err != nil {
		release()
		h.logMCPCall(ctx, tool, "failed")
		return mcpServiceError(err), nil
	}
	if hold != nil {
		release()
		h.logMCPCall(ctx, tool, "retention_guard")
		return mcpToolError("retention_guard", hold.message, hold.detail), nil
	}

	rows, err := h.mcpStartRows(ctx, in.Domain, []mcpItem{item})
	if err != nil {
		release()
		h.logMCPCall(ctx, tool, "failed")
		return mcpServiceError(err), nil
	}

	sctx := WithRunOrigin(ctx, RunOrigin{Via: "mcp", KeyID: caller.KeyID})
	started, err := h.startMCPItem(sctx, item)
	return h.mcpStartOutcome(ctx, tool, release, started, err, "a backup is already running", map[string]any{
		"started":  true,
		"domain":   in.Domain,
		"items":    rows,
		"skipped":  []mcpSkipped{},
		"followUp": mcpItemFollowUp,
	}), nil
}

// startMCPItem calls the service function behind the item's domain, the same
// one the web interface reaches.
func (h *Handler) startMCPItem(ctx context.Context, item mcpItem) (bool, error) {
	switch item.Domain {
	case "containers":
		return h.svc.StartBackup(ctx, item.Name)
	case "vms":
		return h.svc.StartBackupVM(ctx, item.Name)
	case "files":
		return h.svc.StartBackupFileSet(ctx, item.ID)
	case "flash":
		return h.svc.StartBackupFlash(ctx)
	default:
		return h.svc.StartBackupConfig(ctx)
	}
}

type startDomainBackupInput struct {
	Domain string `json:"domain"`
}

func (h *Handler) toolStartDomainBackup(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	const tool = "start_domain_backup"
	caller, ok := mcpCallerFrom(ctx)
	if !ok {
		return mcpNoCaller(), nil
	}
	var in startDomainBackupInput
	if err := decodeMCPArgs(req.Params.Arguments, &in); err != nil {
		h.logMCPCall(ctx, tool, "invalid_argument")
		return mcpToolError("invalid_argument", err.Error(), nil), nil
	}
	settings, refusal := h.mcpStartAllowed(ctx, tool, caller, in.Domain)
	if refusal != nil {
		return refusal, nil
	}
	ctx, cancel := h.mcpToolContext(ctx, mcpReadTimeout)
	defer cancel()

	items, skipped, err := h.domainStartSelection(ctx, settings, in.Domain)
	if err != nil {
		h.logMCPCall(ctx, tool, "failed")
		return mcpServiceError(err), nil
	}
	if len(items) == 0 {
		h.logMCPCall(ctx, tool, "nothing_to_back_up")
		return mcpToolError("nothing_to_back_up", "no item in "+in.Domain+" is included in backups", nil), nil
	}

	now := h.mcp.now()
	targets := append([]string{domainRunTargetID(in.Domain)}, mcpItemIDs(items)...)
	release, refusal := h.mcpStartPreflight(ctx, tool, caller.KeyID, in.Domain, targets, now)
	if refusal != nil {
		return refusal, nil
	}

	kept := make([]mcpItem, 0, len(items))
	var lastHold *mcpHeldBack
	for _, item := range items {
		hold, hErr := h.mcpRetentionHold(settings, item, now)
		if hErr != nil {
			release()
			h.logMCPCall(ctx, tool, "failed")
			return mcpServiceError(hErr), nil
		}
		if hold != nil {
			lastHold = hold
			skipped = append(skipped, mcpSkipped{ID: item.ID, Name: item.Name, Reason: "retention_guard"})
			continue
		}
		kept = append(kept, item)
	}
	if len(kept) == 0 {
		release()
		h.logMCPCall(ctx, tool, "retention_guard")
		return mcpToolError("retention_guard", lastHold.message, lastHold.detail), nil
	}

	rows, err := h.mcpStartRows(ctx, in.Domain, kept)
	if err != nil {
		release()
		h.logMCPCall(ctx, tool, "failed")
		return mcpServiceError(err), nil
	}

	sctx := WithRunOrigin(ctx, RunOrigin{Via: "mcp", KeyID: caller.KeyID})
	started, err := h.startMCPDomain(sctx, in.Domain, kept)
	return h.mcpStartOutcome(ctx, tool, release, started, err, "a backup is already running", map[string]any{
		"started":  true,
		"domain":   in.Domain,
		"items":    rows,
		"skipped":  skipped,
		"followUp": mcpItemFollowUp,
	}), nil
}

// startMCPDomain runs the selection as one batch, which is what gives the
// domain a single prune and a single off-site copy after it.
func (h *Handler) startMCPDomain(ctx context.Context, domain string, items []mcpItem) (bool, error) {
	switch domain {
	case "containers":
		return h.svc.StartBackupAll(ctx, mcpItemNames(items))
	case "vms":
		return h.svc.StartBackupVMsAll(ctx, mcpItemNames(items))
	case "files":
		return h.svc.StartBackupFilesAll(ctx, mcpItemIDs(items))
	case "flash":
		return h.svc.StartBackupFlash(ctx)
	default:
		return h.svc.StartBackupConfig(ctx)
	}
}

func (h *Handler) toolStartBackupEverything(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	const tool = "start_backup_everything"
	caller, ok := mcpCallerFrom(ctx)
	if !ok {
		return mcpNoCaller(), nil
	}
	var in struct{}
	if err := decodeMCPArgs(req.Params.Arguments, &in); err != nil {
		h.logMCPCall(ctx, tool, "invalid_argument")
		return mcpToolError("invalid_argument", err.Error(), nil), nil
	}
	settings, refusal := h.mcpStartAllowed(ctx, tool, caller, "")
	if refusal != nil {
		return refusal, nil
	}
	ctx, cancel := h.mcpToolContext(ctx, mcpReadTimeout)
	defer cancel()

	domains := mcpEnabledDomains(settings)
	if len(domains) == 0 {
		h.logMCPCall(ctx, tool, "nothing_to_back_up")
		return mcpToolError("nothing_to_back_up", "every backup domain is switched off in Settings", nil), nil
	}

	now := h.mcp.now()
	release, refusal := h.mcpStartPreflight(ctx, tool, caller.KeyID, "everything", []string{store.EverythingTargetID}, now)
	if refusal != nil {
		return refusal, nil
	}

	rows := make([]mcpStartItem, 0, len(domains))
	for _, domain := range domains {
		rows = append(rows, mcpStartItem{ID: domain, Name: domain})
	}
	sctx := WithRunOrigin(ctx, RunOrigin{Via: "mcp", KeyID: caller.KeyID})
	started, err := h.svc.StartBackupEverything(sctx)
	return h.mcpStartOutcome(ctx, tool, release, started, err, "a Backup Everything pass is already running", map[string]any{
		"started":  true,
		"domain":   "everything",
		"items":    rows,
		"skipped":  []mcpSkipped{},
		"followUp": mcpEverythingFollowUp,
	}), nil
}

// mcpEnabledDomains lists the domains a Backup Everything pass walks, in the
// order the pass takes them.
func mcpEnabledDomains(s store.Settings) []string {
	var out []string
	for _, d := range []struct {
		name string
		on   bool
	}{
		{"containers", s.ContainersEnabled},
		{"vms", s.VMsEnabled},
		{"flash", s.FlashEnabled},
		{"files", s.FilesEnabled},
		{"config", s.ConfigEnabled},
	} {
		if d.on {
			out = append(out, d.name)
		}
	}
	return out
}

// mcpStartAllowed is what every start tool settles before it looks at an item:
// whether the key may start anything and whether the domain is switched on at
// all. An empty domain skips that last check, which is what the Backup
// Everything pass needs.
func (h *Handler) mcpStartAllowed(ctx context.Context, tool string, caller mcpCaller, domain string) (store.Settings, *mcp.CallToolResult) {
	if !caller.CanStartBackups {
		h.logMCPCall(ctx, tool, "not_permitted")
		return store.Settings{}, mcpToolError("not_permitted", mcpReadOnlyKeyMessage, nil)
	}
	if domain != "" && !slices.Contains(mcpDomains, domain) {
		h.logMCPCall(ctx, tool, "invalid_argument")
		return store.Settings{}, mcpToolError("invalid_argument", "domain must be one of "+strings.Join(mcpDomains, ", "), nil)
	}
	settings, err := h.store.GetSettings()
	if err != nil {
		h.logMCPCall(ctx, tool, "unavailable")
		return store.Settings{}, mcpToolError("unavailable", "settings could not be read", nil)
	}
	if domain != "" && !mcpDomainEnabled(settings, domain) {
		h.logMCPCall(ctx, tool, "domain_off")
		return settings, mcpToolError("domain_off", "the "+domain+" domain is switched off in Settings", nil)
	}
	return settings, nil
}

// mcpStartPreflight holds the cooldown and the hourly budget against what a
// call is about to start. The budget slot is taken here and given back by
// release unless a backup began, so two calls arriving together cannot spend
// the same slot. what names the target in the refusal.
func (h *Handler) mcpStartPreflight(ctx context.Context, tool, keyID, what string, targetIDs []string, now time.Time) (func(), *mcp.CallToolResult) {
	last, err := h.store.LatestMCPStartAt(targetIDs, now.Add(-mcpStartCooldown).Unix())
	if err != nil {
		h.logMCPCall(ctx, tool, "failed")
		return nil, mcpServiceError(err)
	}
	if last > 0 {
		since := now.Sub(time.Unix(last, 0))
		wait := mcpStartCooldown - since
		h.logMCPCall(ctx, tool, "cooldown")
		return nil, mcpToolError("cooldown", fmt.Sprintf(
			"a backup of %s was started through MCP %d minutes ago; wait %d minutes or start it in the web interface",
			what, int(since.Minutes()), int(math.Ceil(wait.Minutes()))),
			map[string]any{"retryAfterSeconds": secondsUntil(wait)})
	}

	release, ok, retry := h.mcp.starts.reserve(keyID, now)
	if !ok {
		h.logMCPCall(ctx, tool, "rate_limited")
		return nil, mcpToolError("rate_limited", fmt.Sprintf("this key started %d backups in the last hour", mcpStartsPerHour),
			map[string]any{"retryAfterSeconds": secondsUntil(retry)})
	}
	return release, nil
}

// mcpStartOutcome maps what a Start function answered onto the tool result and
// gives the reserved budget slot back unless something started.
func (h *Handler) mcpStartOutcome(ctx context.Context, tool string, release func(), started bool, err error, busyMsg string, out map[string]any) *mcp.CallToolResult {
	if started {
		h.logMCPCall(ctx, tool, "ok")
		return mcpOK(out)
	}
	release()
	h.logMCPCall(ctx, tool, "busy")
	if err != nil {
		return mcpToolError("busy", mcpScrubText(scrubError(err)), nil)
	}
	return mcpToolError("busy", busyMsg, nil)
}

// mcpHeldBack is a start the retention guard refuses, with the numbers behind
// it.
type mcpHeldBack struct {
	message string
	detail  map[string]any
}

// mcpRetentionHold reports why one more MCP-started backup of the item would be
// one too many, and nil while it would not. Under a count-only policy the
// window has to keep at least one restore point the schedule or the operator
// made, and no item takes more than mcpItemStartsPerDay MCP backups a day
// whatever the policy.
func (h *Handler) mcpRetentionHold(s store.Settings, item mcpItem, now time.Time) (*mcpHeldBack, error) {
	dayAgo := now.Add(-24 * time.Hour).Unix()
	today, oldest, err := h.store.MCPBackupsSince(item.ID, dayAgo)
	if err != nil {
		return nil, err
	}
	if today >= mcpItemStartsPerDay {
		free := time.Unix(oldest, 0).Add(24 * time.Hour).Sub(now)
		return &mcpHeldBack{
			message: fmt.Sprintf("%s already got %d backups through MCP in the last 24 hours", item.Name, today),
			detail:  map[string]any{"mcpStartsToday": today, "retryAfterSeconds": secondsUntil(free)},
		}, nil
	}

	keepLast := countOnlyKeepLast(h.svc.retentionPolicy(s))
	if offsite := countOnlyKeepLast(h.svc.offsiteRetentionPolicy(s)); offsite > 0 && (keepLast == 0 || offsite < keepLast) {
		keepLast = offsite
	}
	if keepLast == 0 {
		return nil, nil
	}
	if keepLast == 1 {
		return &mcpHeldBack{
			message: fmt.Sprintf("retention keeps a single restore point of %s, and an MCP backup would make it the only one there is; "+
				"back it up in the web interface", item.Name),
			detail: h.retentionRetryDetail(keepLast, 0, item.Domain, now),
		}, nil
	}
	for _, kind := range mcpRetentionSeries(item.Domain) {
		total, viaMCP, oErr := h.store.NewestBackupOrigins(item.ID, kind, keepLast-1)
		if oErr != nil {
			return nil, oErr
		}
		if total == keepLast-1 && viaMCP == total {
			return &mcpHeldBack{
				message: fmt.Sprintf("retention keeps the newest %d restore points of %s and the last %d came through MCP; "+
					"another one would leave only MCP-made restore points. "+
					"The next scheduled backup makes room again, or start it in the web interface", keepLast, item.Name, viaMCP),
				detail: h.retentionRetryDetail(keepLast, viaMCP, item.Domain, now),
			}, nil
		}
	}
	return nil, nil
}

// mcpRetentionSeries are the run kinds whose restore points share one count-only
// window. A container's database dumps rotate with its backups and are their own
// series in the repository.
func mcpRetentionSeries(domain string) []string {
	if domain == "containers" {
		return []string{"backup", "dbdump"}
	}
	return []string{"backup"}
}

// retentionRetryDetail is the numbers a held-back start reports. The next
// scheduled backup of the domain is what makes room again; without one there is
// nothing to wait for and the field stays out.
func (h *Handler) retentionRetryDetail(keepLast, viaMCP int, domain string, now time.Time) map[string]any {
	detail := map[string]any{"keepLast": keepLast, "mcpInWindow": viaMCP}
	if h.scheduler == nil {
		return detail
	}
	for _, next := range h.scheduler.NextRuns() {
		if next.Job == "backup" && next.Domain == domain {
			detail["retryAfterSeconds"] = secondsUntil(next.Next.Sub(now))
			break
		}
	}
	return detail
}

// countOnlyKeepLast is how many restore points a policy keeps when it keeps by
// count alone, and 0 for one with a daily, weekly or monthly rule underneath,
// which holds the older days whatever a new snapshot rotates out.
func countOnlyKeepLast(p restic.RetentionPolicy) int {
	if p.KeepDaily > 0 || p.KeepWeekly > 0 || p.KeepMonthly > 0 {
		return 0
	}
	return p.KeepLast
}

// secondsUntil rounds a wait up to whole seconds, never below one, which is what
// an assistant can act on.
func secondsUntil(d time.Duration) int {
	s := int(math.Ceil(d.Seconds()))
	if s < 1 {
		s = 1
	}
	return s
}

// domainStartSelection is every item the operator protects in this domain and
// has not paused, in the order a batch runs them, plus the ones it leaves out
// with the reason. Items with a cadence of their own are in: a person asking to
// back up a domain now means those too, which is where this parts company with
// the scheduler's domain pass.
func (h *Handler) domainStartSelection(ctx context.Context, s store.Settings, domain string) ([]mcpItem, []mcpSkipped, error) {
	skipped := []mcpSkipped{}
	var items []mcpItem
	switch domain {
	case "containers":
		targets, err := h.store.ListTargetsScheduleOrder()
		if err != nil {
			return nil, nil, err
		}
		self := h.svc.SelfContainerName(ctx)
		for _, t := range targets {
			if !t.IncludeInSchedule || t.ContainerName == self {
				continue
			}
			if schedule.PausedByOverride(t.ScheduleCadence, s.PerItemSchedules) {
				continue
			}
			items = append(items, mcpItem{Domain: domain, ID: t.ID, Name: t.ContainerName})
		}
	case "vms":
		vms, err := h.store.ListVMTargets()
		if err != nil {
			return nil, nil, err
		}
		store.SortVMTargetsForRun(vms)
		for _, vm := range vms {
			if !vm.IncludeInSchedule || schedule.PausedByOverride(vm.ScheduleCadence, s.PerItemSchedules) {
				continue
			}
			items = append(items, mcpItem{Domain: domain, ID: vm.ID, Name: vm.Name})
		}
	case "files":
		sets, err := h.store.ListFileSets()
		if err != nil {
			return nil, nil, err
		}
		for _, set := range sets {
			if !set.Enabled || schedule.PausedByOverride(set.ScheduleCadence, s.PerItemSchedules) {
				continue
			}
			if set.Path == "" {
				skipped = append(skipped, mcpSkipped{ID: set.ID, Name: set.Name, Reason: "no_folder"})
				continue
			}
			items = append(items, mcpItem{Domain: domain, ID: set.ID, Name: set.Name})
		}
	default:
		items = append(items, mcpItem{Domain: domain, ID: domainRunTargetID(domain), Name: domain})
	}
	return items, skipped, nil
}

// mcpStartRows is what the result says about the items a start covers.
func (h *Handler) mcpStartRows(ctx context.Context, domain string, items []mcpItem) ([]mcpStartItem, error) {
	stamps, err := h.store.LatestBackupsByTarget()
	if err != nil {
		return nil, err
	}
	stops := h.mcpStopsFor(ctx, domain, items)
	rows := make([]mcpStartItem, 0, len(items))
	for _, item := range items {
		row := mcpStartItem{ID: item.ID, Name: item.Name, LastDurationSeconds: stamps[item.ID].LastDurationSeconds}
		if s, ok := stops[item.ID]; ok {
			row.Stops = &s
		}
		rows = append(rows, row)
	}
	return rows, nil
}

// mcpStopsFor is what a backup of each item takes down. Docker is asked once for
// the whole selection, so a domain start costs one call however many containers
// it covers.
func (h *Handler) mcpStopsFor(ctx context.Context, domain string, items []mcpItem) map[string]mcpStops {
	out := make(map[string]mcpStops, len(items))
	switch domain {
	case "containers":
		infos, listErr := h.docker.List(ctx)
		live := make(map[string]dockercli.ContainerInfo, len(infos))
		for _, c := range infos {
			live[c.Name] = c
		}
		targets, tErr := h.store.ListTargets()
		byName := make(map[string]store.Target, len(targets))
		for _, t := range targets {
			byName[t.ContainerName] = t
		}
		known := listErr == nil && tErr == nil
		for _, item := range items {
			stops := mcpStops{Containers: []string{}, Known: known}
			if known {
				stops.Self = isRunning(live[item.Name])
				for _, name := range byName[item.Name].StopContainers {
					if isRunning(live[name]) {
						stops.Containers = append(stops.Containers, name)
					}
				}
			}
			out[item.ID] = stops
		}
	case "vms":
		vms, err := h.store.ListVMTargets()
		graceful := make(map[string]bool, len(vms))
		for _, vm := range vms {
			graceful[vm.ID] = vm.Method != "live"
		}
		for _, item := range items {
			out[item.ID] = mcpStops{Self: graceful[item.ID], Containers: []string{}, Known: err == nil}
		}
	default:
		for _, item := range items {
			out[item.ID] = mcpStops{Containers: []string{}, Known: true}
		}
	}
	return out
}

func mcpItemIDs(items []mcpItem) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		out = append(out, item.ID)
	}
	return out
}

func mcpItemNames(items []mcpItem) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		out = append(out, item.Name)
	}
	return out
}
