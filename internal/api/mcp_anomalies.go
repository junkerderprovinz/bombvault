package api

import (
	"context"
	"fmt"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// How many findings one call may ask for, and how many it gets without asking.
const (
	mcpAnomaliesLimitMax     = 100
	mcpAnomaliesLimitDefault = 50
)

// mcpAnomalyStates are the listings list_anomalies offers. The page's finer
// states are all "closed" to an assistant: none of them needs anything done.
var mcpAnomalyStates = []string{"open", "closed", "all"}

// mcpAnomaly is one finding as the tools report it: ids and numbers, with the
// domain in the vocabulary every other tool takes.
type mcpAnomaly struct {
	ID            string           `json:"id"`
	Detector      string           `json:"detector"`
	Metric        string           `json:"metric"`
	Severity      string           `json:"severity"`
	State         string           `json:"state"`
	ScopeKind     string           `json:"scopeKind"`
	Domain        string           `json:"domain"`
	ItemID        string           `json:"itemId"`
	ItemName      string           `json:"itemName"`
	Part          string           `json:"part"`
	FirstSeenAt   int64            `json:"firstSeenAt"`
	LastSeenAt    int64            `json:"lastSeenAt"`
	Observed      float64          `json:"observed"`
	Expected      float64          `json:"expected"`
	Threshold     float64          `json:"threshold"`
	Samples       int              `json:"samples"`
	Occurrences   int              `json:"occurrences"`
	RecoveredAt   int64            `json:"recoveredAt"`
	RetentionHeld bool             `json:"retentionHeld"`
	LastGood      *RestorePointRef `json:"lastGood,omitempty"`
	Details       map[string]any   `json:"details"`
}

// mcpAnomalyDetail is what get_anomaly adds to a row: the note an operator left
// when settling it, and whether the web interface can mark it as expected.
type mcpAnomalyDetail struct {
	mcpAnomaly
	AckNote    string `json:"ackNote"`
	Expectable bool   `json:"expectable"`
}

func mcpAnomalyOf(v AnomalyView) mcpAnomaly {
	details := v.Details
	if details == nil {
		details = map[string]any{}
	}
	return mcpAnomaly{
		ID:            v.ID,
		Detector:      v.Detector,
		Metric:        v.Metric,
		Severity:      v.Severity,
		State:         v.State,
		ScopeKind:     v.ScopeKind,
		Domain:        mcpDomainOut(v.Domain),
		ItemID:        v.TargetID,
		ItemName:      v.Name,
		Part:          v.Part,
		FirstSeenAt:   v.FirstSeenAt,
		LastSeenAt:    v.LastSeenAt,
		Observed:      v.Observed,
		Expected:      v.Expected,
		Threshold:     v.Threshold,
		Samples:       v.Samples,
		Occurrences:   v.Occurrences,
		RecoveredAt:   v.RecoveredAt,
		RetentionHeld: v.RetentionHeld,
		LastGood:      v.LastGood,
		Details:       details,
	}
}

type listAnomaliesInput struct {
	State    string   `json:"state"`
	Severity []string `json:"severity"`
	Domain   string   `json:"domain"`
	Limit    *int     `json:"limit"`
}

func (h *Handler) toolListAnomalies(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	const tool = "list_anomalies"
	if _, ok := mcpCallerFrom(ctx); !ok {
		return mcpNoCaller(), nil
	}
	var in listAnomaliesInput
	if err := decodeMCPArgs(req.Params.Arguments, &in); err != nil {
		h.logMCPCall(ctx, tool, "invalid_argument")
		return mcpToolError("invalid_argument", err.Error(), nil), nil
	}
	limit := mcpAnomaliesLimitDefault
	if in.Limit != nil {
		limit = *in.Limit
	}
	refuse := func(msg string) (*mcp.CallToolResult, error) {
		h.logMCPCall(ctx, tool, "invalid_argument")
		return mcpToolError("invalid_argument", msg, nil), nil
	}
	switch {
	case limit < 1 || limit > mcpAnomaliesLimitMax:
		return refuse(fmt.Sprintf("limit must be between 1 and %d", mcpAnomaliesLimitMax))
	case in.State != "" && !slices.Contains(mcpAnomalyStates, in.State):
		return refuse("state must be one of " + strings.Join(mcpAnomalyStates, ", "))
	case in.Domain != "" && !slices.Contains(mcpDomains, in.Domain):
		return refuse("domain must be one of " + strings.Join(mcpDomains, ", "))
	}
	for _, severity := range in.Severity {
		if !slices.Contains(anomalySeverities, severity) {
			return refuse("severity must be one of " + strings.Join(anomalySeverities, ", "))
		}
	}

	// The query the Anomalies page sends, so a closed listing reaches back
	// exactly as far as the page does. One row over the limit tells whether
	// there is more.
	q := url.Values{"limit": {strconv.Itoa(limit + 1)}}
	if in.State != "" {
		q.Set("state", in.State)
	}
	if len(in.Severity) > 0 {
		q.Set("severity", strings.Join(in.Severity, ","))
	}
	if in.Domain != "" {
		q.Set("domain", strings.Join(mcpAnomalyDomainsIn(in.Domain), ","))
	}
	filter, err := anomalyFilterFrom(q, time.Now().Unix())
	if err != nil {
		return refuse(err.Error())
	}

	ctx, cancel := h.mcpToolContext(ctx, mcpReadTimeout)
	defer cancel()
	page, err := h.svc.ListAnomalies(ctx, filter)
	if err != nil {
		h.logMCPCall(ctx, tool, "failed")
		return mcpServiceError(err), nil
	}
	views := page.Anomalies
	truncated := len(views) > limit
	rows := make([]mcpAnomaly, 0, min(len(views), limit))
	for _, v := range views[:min(len(views), limit)] {
		rows = append(rows, mcpAnomalyOf(v))
	}
	summary := h.svc.AnomalySummary(ctx)

	h.logMCPCall(ctx, tool, "ok")
	return mcpOK(map[string]any{
		"summary": map[string]any{
			"enabled":       summary.Enabled,
			"ready":         summary.Ready,
			"open":          summary.Open,
			"learningItems": summary.LearningItems,
			"retentionHeld": summary.RetentionHeld,
		},
		"anomalies": rows,
		"truncated": truncated,
	}), nil
}

// mcpAnomalyDomainsIn is mcpDomainOut read backwards. A finding about one item
// stores the singular domain and one about a whole domain the plural, and an
// assistant asking about containers means both.
func mcpAnomalyDomainsIn(domain string) []string {
	switch domain {
	case "containers":
		return []string{"container", "containers"}
	case "vms":
		return []string{"vm", "vms"}
	}
	return []string{domain}
}

type getAnomalyInput struct {
	ID string `json:"id"`
}

func (h *Handler) toolGetAnomaly(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	const tool = "get_anomaly"
	if _, ok := mcpCallerFrom(ctx); !ok {
		return mcpNoCaller(), nil
	}
	var in getAnomalyInput
	if err := decodeMCPArgs(req.Params.Arguments, &in); err != nil {
		h.logMCPCall(ctx, tool, "invalid_argument")
		return mcpToolError("invalid_argument", err.Error(), nil), nil
	}
	if !runIDRe.MatchString(in.ID) {
		h.logMCPCall(ctx, tool, "invalid_argument")
		return mcpToolError("invalid_argument", "id must be the id of a finding as list_anomalies reports it", nil), nil
	}

	ctx, cancel := h.mcpToolContext(ctx, mcpReadTimeout)
	defer cancel()
	view, found, err := h.svc.GetAnomaly(ctx, in.ID)
	if err != nil {
		h.logMCPCall(ctx, tool, "failed")
		return mcpServiceError(err), nil
	}
	if !found {
		h.logMCPCall(ctx, tool, "not_found")
		return mcpToolError("not_found", "no finding with this id", nil), nil
	}

	h.logMCPCall(ctx, tool, "ok")
	return mcpOK(map[string]any{"anomaly": mcpAnomalyDetail{
		mcpAnomaly: mcpAnomalyOf(view),
		AckNote:    mcpScrubText(view.AckNote),
		Expectable: view.Expectable,
	}}), nil
}
