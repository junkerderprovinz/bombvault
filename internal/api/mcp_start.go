package api

import (
	"context"
	"errors"
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

// mcpStartItem is one item a start or a cancel acted on. Stops and the last
// duration are what an assistant tells the user before the apps go down; the
// Backup Everything pass lists domains rather than items and carries neither.
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
	mcpNotRunningMessage  = "this run is not running any more"
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
		h.logMCPCall(ctx, tool, mcpErrorCodeOf(bad))
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
	started, err := h.mcpStartOutsideEverything(func() (bool, error) { return h.startMCPItem(sctx, item) })
	return h.mcpStartOutcome(ctx, tool, release, started, err, "a backup is already running", map[string]any{
		"started":  true,
		"domain":   in.Domain,
		"items":    rows,
		"skipped":  []mcpSkipped{},
		"followUp": mcpItemFollowUp,
	}), nil
}

// startMCPItem calls the service function behind the item's domain, the same
// one the web interface reaches. A domain without a branch here starts nothing,
// so one added to mcpDomains alone cannot back up something else instead.
func (h *Handler) startMCPItem(ctx context.Context, item mcpItem) (bool, error) {
	switch item.Domain {
	case "containers":
		return h.svc.StartBackup(ctx, item.Name)
	case "vms":
		return h.svc.StartBackupVM(ctx, item.Name)
	case "files":
		return h.svc.StartBackupFileSet(ctx, item.ID)
	case zfsDomain:
		return h.svc.StartBackupZFSDataset(ctx, item.ID)
	case "flash":
		return h.svc.StartBackupFlash(ctx)
	case "config":
		return h.svc.StartBackupConfig(ctx)
	}
	return false, errMCPNoStart(item.Domain)
}

// errMCPNoStart is what a start switch answers for a domain it has no branch
// for.
func errMCPNoStart(domain string) error {
	return fmt.Errorf("BombVault cannot start a backup of the %s domain through MCP", domain)
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

	items, skipped, err := h.domainStartSelection(ctx, settings, in.Domain, false)
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
	started, err := h.mcpStartOutsideEverything(func() (bool, error) { return h.startMCPDomain(sctx, in.Domain, kept) })
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
	case zfsDomain:
		return h.svc.StartBackupZFSAll(ctx, mcpItemIDs(items))
	case "flash":
		return h.svc.StartBackupFlash(ctx)
	case "config":
		return h.svc.StartBackupConfig(ctx)
	}
	return false, errMCPNoStart(domain)
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
	settings, refusal := h.mcpMayStart(ctx, tool, caller)
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

	guard, err := h.everythingRetentionHold(ctx, settings, domains, now)
	if err != nil {
		release()
		h.logMCPCall(ctx, tool, "failed")
		return mcpServiceError(err), nil
	}
	if guard.kept == 0 && guard.last != nil {
		release()
		h.logMCPCall(ctx, tool, "retention_guard")
		return mcpToolError("retention_guard", guard.last.message, guard.last.detail), nil
	}

	rows := make([]mcpStartItem, 0, len(guard.domains))
	for _, domain := range guard.domains {
		rows = append(rows, mcpStartItem{ID: domain, Name: domain})
	}
	sctx := WithEverythingSkips(WithRunOrigin(ctx, RunOrigin{Via: "mcp", KeyID: caller.KeyID}), guard.skip)
	started, err := h.mcpStartEverythingAlone(sctx)
	return h.mcpStartOutcome(ctx, tool, release, started, err, "a Backup Everything pass is already running", map[string]any{
		"started":  true,
		"domain":   "everything",
		"items":    rows,
		"skipped":  guard.skipped,
		"followUp": mcpEverythingFollowUp,
	}), nil
}

// mcpEverythingHold is what the retention guard leaves of a Backup Everything
// pass: the items it has to leave out, the rows the result names them under,
// how many items still run and the last refusal, which is what the tool answers
// when none do.
type mcpEverythingHold struct {
	skip    []string
	skipped []mcpSkipped
	kept    int
	last    *mcpHeldBack
	// domains are the ones with an item the pass still runs, in its order.
	domains []string
}

// everythingRetentionHold applies the retention guard to every item the pass
// would touch. Without it the widest start tool would be the one way around a
// guard the narrower two enforce.
func (h *Handler) everythingRetentionHold(ctx context.Context, s store.Settings, domains []string, now time.Time) (mcpEverythingHold, error) {
	out := mcpEverythingHold{skipped: []mcpSkipped{}}
	for _, domain := range domains {
		items, _, err := h.domainStartSelection(ctx, s, domain, true)
		if err != nil {
			return out, err
		}
		for _, item := range items {
			hold, hErr := h.mcpRetentionHold(s, item, now)
			if hErr != nil {
				return out, hErr
			}
			if hold == nil {
				out.kept++
				if !slices.Contains(out.domains, domain) {
					out.domains = append(out.domains, domain)
				}
				continue
			}
			out.last = hold
			out.skip = append(out.skip, item.ID)
			out.skipped = append(out.skipped, mcpSkipped{ID: item.ID, Name: item.Name, Reason: "retention_guard"})
		}
	}
	return out, nil
}

type cancelBackupInput struct {
	RunID string `json:"runId"`
}

func (h *Handler) toolCancelBackup(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	const tool = "cancel_backup"
	caller, ok := mcpCallerFrom(ctx)
	if !ok {
		return mcpNoCaller(), nil
	}
	var in cancelBackupInput
	if err := decodeMCPArgs(req.Params.Arguments, &in); err != nil {
		h.logMCPCall(ctx, tool, "invalid_argument")
		return mcpToolError("invalid_argument", err.Error(), nil), nil
	}
	if !caller.CanStartBackups {
		h.logMCPCall(ctx, tool, "not_permitted")
		return mcpToolError("not_permitted", mcpReadOnlyKeyMessage, nil), nil
	}
	if !validRunID(in.RunID) {
		h.logMCPCall(ctx, tool, "invalid_argument")
		return mcpToolError("invalid_argument", "runId must be a run id as list_runs and get_activity report it", nil), nil
	}

	run, err := h.store.GetRun(in.RunID)
	if errors.Is(err, store.ErrRunNotFound) {
		h.logMCPCall(ctx, tool, "not_found")
		return mcpToolError("not_found", "no run with this id", nil), nil
	}
	if err != nil {
		h.logMCPCall(ctx, tool, "failed")
		return mcpServiceError(err), nil
	}
	if run.Kind != "backup" || run.Status != "running" {
		h.logMCPRunCall(ctx, tool, "not_running", run.ID)
		return mcpToolError("not_running", mcpNotRunningMessage, nil), nil
	}
	if run.StartedVia != "mcp" || run.StartedViaKey != caller.KeyID {
		h.logMCPRunCall(ctx, tool, "not_permitted", run.ID)
		return mcpToolError("not_permitted", "this run was not started by this key; cancel it in the web interface", nil), nil
	}
	if run.TargetID == store.EverythingTargetID {
		h.logMCPRunCall(ctx, tool, "not_permitted", run.ID)
		return mcpToolError("not_permitted", "a Backup Everything pass cannot be cancelled as a whole; cancel the item backup running inside it", nil), nil
	}
	key, item, ok := h.mcpCancelKey(run)
	if !ok {
		h.logMCPRunCall(ctx, tool, "not_found", run.ID)
		return mcpToolError("not_found", "the item this run belongs to is not set up in BombVault any more", nil), nil
	}
	out := map[string]any{
		"cancelled": true,
		"runId":     run.ID,
		"domain":    item.Domain,
		"item":      mcpStartItem{ID: item.ID, Name: item.Name},
	}
	if !h.svc.CancelBackupRun(key, run.ID) {
		if h.svc.BackupCommitted(key, run.ID) {
			out["cancelled"] = false
			out["warning"] = "this backup already wrote its restore point and is starting its containers again, so a cancel cannot take it back"
			h.logMCPRunCall(ctx, tool, "ok", run.ID)
			return mcpOK(out), nil
		}
		h.logMCPRunCall(ctx, tool, "not_running", run.ID)
		return mcpToolError("not_running", mcpNotRunningMessage, nil), nil
	}
	// restic writes the snapshot last, so a backup that was nearly done can
	// finish before the cancellation reaches it.
	if after, aErr := h.store.GetRun(run.ID); aErr == nil && after.Status != "running" && after.Status != "cancelled" {
		out["cancelled"] = false
		out["warning"] = "this backup finished before the cancellation reached it"
	}
	h.logMCPRunCall(ctx, tool, "ok", run.ID)
	return mcpOK(out), nil
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
		{zfsDomain, s.ZFSEnabled},
		{"config", s.ConfigEnabled},
	} {
		if d.on {
			out = append(out, d.name)
		}
	}
	return out
}

// mcpMayStart is what every start tool settles before it looks at an item:
// whether the key may start anything. It hands back the settings the start
// works from.
func (h *Handler) mcpMayStart(ctx context.Context, tool string, caller mcpCaller) (store.Settings, *mcp.CallToolResult) {
	if !caller.CanStartBackups {
		h.logMCPCall(ctx, tool, "not_permitted")
		return store.Settings{}, mcpToolError("not_permitted", mcpReadOnlyKeyMessage, nil)
	}
	settings, err := h.store.GetSettings()
	if err != nil {
		h.logMCPCall(ctx, tool, "unavailable")
		return store.Settings{}, mcpToolError("unavailable", "settings could not be read", nil)
	}
	return settings, nil
}

// mcpStartAllowed is mcpMayStart for a start within one domain, which has to
// be one BombVault knows and switched on. A read-only key hears that it may not
// start rather than what is wrong with its arguments.
func (h *Handler) mcpStartAllowed(ctx context.Context, tool string, caller mcpCaller, domain string) (store.Settings, *mcp.CallToolResult) {
	if caller.CanStartBackups && !slices.Contains(mcpDomains, domain) {
		h.logMCPCall(ctx, tool, "invalid_argument")
		return store.Settings{}, mcpToolError("invalid_argument", "domain must be one of "+strings.Join(mcpDomains, ", "), nil)
	}
	settings, refusal := h.mcpMayStart(ctx, tool, caller)
	if refusal != nil {
		return settings, refusal
	}
	if !mcpDomainEnabled(settings, domain) {
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

// The retention guard counts finished restore points only, so a Backup
// Everything pass must not run beside a narrower MCP start: the pass would back
// up again what the other one is still backing up, and neither guard would have
// counted it.
var (
	errMCPEverythingRunning = errors.New("a Backup Everything pass is running; start this once it has finished")
	errMCPBackupRunning     = errors.New("a backup is already running; start Backup Everything once it has finished")
)

// mcpStartOutsideEverything runs start unless a Backup Everything pass is in
// flight.
func (h *Handler) mcpStartOutsideEverything(start func() (bool, error)) (bool, error) {
	h.mcp.startMu.Lock()
	defer h.mcp.startMu.Unlock()
	if h.svc.EverythingInProgress() {
		return false, errMCPEverythingRunning
	}
	return start()
}

// mcpStartEverythingAlone starts a Backup Everything pass unless another backup
// is in flight.
func (h *Handler) mcpStartEverythingAlone(ctx context.Context) (bool, error) {
	h.mcp.startMu.Lock()
	defer h.mcp.startMu.Unlock()
	if h.svc.BackupInProgress() {
		return false, errMCPBackupRunning
	}
	return h.svc.StartBackupEverything(ctx)
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

	keepLast, err := h.mcpKeepLast(s, item.Domain)
	if err != nil {
		return nil, err
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
	if item.Domain == zfsDomain {
		datasets, zErr := h.store.ZFSDatasetsWithMCPWindow(item.ID, keepLast-1)
		if zErr != nil {
			return nil, zErr
		}
		if len(datasets) > 0 {
			return &mcpHeldBack{
				message: fmt.Sprintf("retention keeps the newest %d restore points of each dataset of %s, and the last %d of %s came through MCP; "+
					"another one would leave only MCP-made restore points. "+
					"The next scheduled backup makes room again, or start it in the web interface", keepLast, item.Name, keepLast-1, datasets[0]),
				detail: h.retentionRetryDetail(keepLast, keepLast-1, item.Domain, now),
			}, nil
		}
	}
	return nil, nil
}

// mcpKeepLast is the smallest count-only window a domain's restore points live
// under: the local policy's and that of every off-site destination the domain
// replicates to, read from the destinations themselves because each one prunes
// by its own policy. An append-only destination prunes on the far side, out of
// reach of anything BombVault starts.
func (h *Handler) mcpKeepLast(s store.Settings, domain string) (int, error) {
	targets, err := h.svc.enabledOffsiteTargets(domain)
	if err != nil {
		return 0, err
	}
	keepLast := countOnlyKeepLast(h.svc.retentionPolicy(s))
	for _, t := range orSettingsOffsiteTarget(targets, domain, s) {
		if t.Immutable {
			continue
		}
		if n := countOnlyKeepLast(targetOffsiteRetentionPolicy(t)); n > 0 && (keepLast == 0 || n < keepLast) {
			keepLast = n
		}
	}
	return keepLast, nil
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
// the scheduler's domain pass. passOnly narrows it to what that pass runs, which
// is all a Backup Everything pass backs up.
func (h *Handler) domainStartSelection(ctx context.Context, s store.Settings, domain string, passOnly bool) ([]mcpItem, []mcpSkipped, error) {
	perItem := passOnly && s.PerItemSchedules
	skipped := []mcpSkipped{}
	var items []mcpItem
	switch domain {
	case "containers":
		targets, err := h.store.ListTargetsScheduleOrder()
		if err != nil {
			return nil, nil, err
		}
		self := h.svc.SelfContainerName(ctx)
		for _, t := range schedule.DomainRunTargets(targets, perItem) {
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
		for _, vm := range schedule.DomainRunVMTargets(vms, perItem) {
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
		for _, set := range schedule.DomainRunFileSets(sets, perItem) {
			if !set.Enabled || schedule.PausedByOverride(set.ScheduleCadence, s.PerItemSchedules) {
				continue
			}
			if set.Path == "" {
				skipped = append(skipped, mcpSkipped{ID: set.ID, Name: set.Name, Reason: "no_folder"})
				continue
			}
			items = append(items, mcpItem{Domain: domain, ID: set.ID, Name: set.Name})
		}
	case zfsDomain:
		datasets, err := h.store.ListZFSDatasets()
		if err != nil {
			return nil, nil, err
		}
		for _, d := range schedule.DomainRunZFSDatasets(datasets, perItem) {
			if !d.Enabled || schedule.PausedByOverride(d.ScheduleCadence, s.PerItemSchedules) {
				continue
			}
			items = append(items, mcpItem{Domain: domain, ID: d.ID, Name: d.Dataset})
		}
	case "flash", "config":
		items = append(items, mcpItem{Domain: domain, ID: domainRunTargetID(domain), Name: domain})
	default:
		return nil, nil, errMCPNoStart(domain)
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
	case zfsDomain:
		datasets, dErr := h.store.ListZFSDatasets()
		byID := make(map[string]store.ZFSDataset, len(datasets))
		for _, d := range datasets {
			byID[d.ID] = d
		}
		docker := h.mcpDockerOnce(ctx)
		for _, item := range items {
			stops := mcpStops{Containers: []string{}, Known: dErr == nil}
			if names := byID[item.ID].StopContainers; len(names) > 0 {
				state := docker()
				stops.Known = stops.Known && state.answered
				for _, name := range names {
					if isRunning(state.live[name]) {
						stops.Containers = append(stops.Containers, name)
					}
				}
			}
			out[item.ID] = stops
		}
	default:
		for _, item := range items {
			out[item.ID] = mcpStops{Containers: []string{}, Known: true}
		}
	}
	return out
}

// mcpCancelKey is the progress key the service registered a run's backup under,
// with the item behind it. It has to read exactly as the key in Backup,
// BackupVM, BackupFileSet, BackupZFSDataset, BackupFlash and BackupConfig, or a
// cancel reaches nothing.
func (h *Handler) mcpCancelKey(run store.Run) (string, mcpItem, bool) {
	switch run.TargetID {
	case store.FlashTargetID:
		return "flash", mcpItem{Domain: "flash", ID: run.TargetID, Name: "flash"}, true
	case store.ConfigTargetID:
		return "config", mcpItem{Domain: "config", ID: run.TargetID, Name: "config"}, true
	}
	if t, err := h.store.GetTargetByID(run.TargetID); err == nil {
		return "container:" + t.ContainerName, mcpItem{Domain: "containers", ID: t.ID, Name: t.ContainerName}, true
	}
	if vm, err := h.store.GetVMTargetByID(run.TargetID); err == nil {
		return "vm:" + vm.Name, mcpItem{Domain: "vms", ID: vm.ID, Name: vm.Name}, true
	}
	if set, err := h.store.GetFileSet(run.TargetID); err == nil {
		return "files:" + set.Name, mcpItem{Domain: "files", ID: set.ID, Name: set.Name}, true
	}
	if d, err := h.store.GetZFSDataset(run.TargetID); err == nil {
		return zfsDomain + ":" + d.Dataset, mcpItem{Domain: zfsDomain, ID: d.ID, Name: d.Dataset}, true
	}
	return "", mcpItem{}, false
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
