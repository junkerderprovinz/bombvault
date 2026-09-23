package api

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/junkerderprovinz/bombvault/internal/schedule"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// How many repository size samples one call may ask for, and how many it gets
// without asking.
const (
	mcpStatsLimitMax     = 90
	mcpStatsLimitDefault = 30
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
			"startsLeftThisHour": mcpStartsPerHour,
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
		h.logMCPCall(ctx, "get_status", "failed")
		return mcpServiceError(err), nil
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
		h.logMCPCall(ctx, "get_coverage", "failed")
		return mcpServiceError(err), nil
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
		h.logMCPCall(ctx, "get_storage_stats", "failed")
		return mcpServiceError(err), nil
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
