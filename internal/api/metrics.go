package api

import (
	"fmt"
	"log"
	"sort"
	"strings"
)

// metricsContentType is the Prometheus text exposition format media type.
const metricsContentType = "text/plain; version=0.0.4; charset=utf-8"

// MetricsAccess reports whether /metrics is enabled and the bearer token a
// scrape must present; an empty token leaves the endpoint open.
func (s *Service) MetricsAccess() (enabled bool, token string, err error) {
	settings, err := s.store.GetSettings()
	if err != nil {
		return false, "", err
	}
	return settings.MetricsEnabled, settings.MetricsToken, nil
}

// escapeLabelValue escapes backslash, double quote and newline in a Prometheus
// label value.
func escapeLabelValue(v string) string {
	r := strings.NewReplacer(
		`\`, `\\`,
		`"`, `\"`,
		"\n", `\n`,
	)
	return r.Replace(v)
}

// Metrics renders the Prometheus text exposition. It carries no repo paths,
// secrets or hostnames.
func (s *Service) Metrics() (string, error) {
	statuses, err := s.DomainStatus()
	if err != nil {
		return "", fmt.Errorf("metrics: domain status: %w", err)
	}
	runCounts, err := s.store.RunCounts()
	if err != nil {
		return "", fmt.Errorf("metrics: run counts: %w", err)
	}

	var b strings.Builder

	b.WriteString("# HELP bombvault_build_info BombVault build information.\n")
	b.WriteString("# TYPE bombvault_build_info gauge\n")
	fmt.Fprintf(&b, "bombvault_build_info{version=\"%s\"} 1\n", escapeLabelValue(Version))

	b.WriteString("# HELP bombvault_backup_last_success_timestamp_seconds Unix time of the last successful backup per domain (0 if none).\n")
	b.WriteString("# TYPE bombvault_backup_last_success_timestamp_seconds gauge\n")
	for _, d := range statuses {
		fmt.Fprintf(&b, "bombvault_backup_last_success_timestamp_seconds{domain=\"%s\"} %d\n",
			escapeLabelValue(d.Domain), d.LastSuccess)
	}

	b.WriteString("# HELP bombvault_domain_enabled Whether a backup domain is enabled (1) or disabled (0).\n")
	b.WriteString("# TYPE bombvault_domain_enabled gauge\n")
	for _, d := range statuses {
		fmt.Fprintf(&b, "bombvault_domain_enabled{domain=\"%s\"} %d\n",
			escapeLabelValue(d.Domain), boolMetric(d.Enabled))
	}

	// A domain without a repo stat sample yet gets no size or snapshot series.
	var sizeBody, snapBody strings.Builder
	for _, d := range statuses {
		latest, found, lErr := s.store.LatestRepoStat(d.Domain, "local")
		if lErr != nil {
			return "", fmt.Errorf("metrics: repo stat %s: %w", d.Domain, lErr)
		}
		if !found {
			continue
		}
		fmt.Fprintf(&sizeBody, "bombvault_repo_size_bytes{domain=\"%s\",source=\"local\"} %d\n",
			escapeLabelValue(d.Domain), latest.RawSize)
		fmt.Fprintf(&snapBody, "bombvault_repo_snapshots{domain=\"%s\",source=\"local\"} %d\n",
			escapeLabelValue(d.Domain), latest.Snapshots)
	}
	if sizeBody.Len() > 0 {
		b.WriteString("# HELP bombvault_repo_size_bytes Physical (deduplicated, compressed) repository size in bytes from the latest sample.\n")
		b.WriteString("# TYPE bombvault_repo_size_bytes gauge\n")
		b.WriteString(sizeBody.String())
	}
	if snapBody.Len() > 0 {
		b.WriteString("# HELP bombvault_repo_snapshots Snapshot count in the repository from the latest sample.\n")
		b.WriteString("# TYPE bombvault_repo_snapshots gauge\n")
		b.WriteString(snapBody.String())
	}

	// The protection gauges follow the scorecard: disabled domains get none, and
	// tamper_test_ok only appears for an immutable off-site, so a domain that
	// claims no append-only protection never shows a misleading 0.
	b.WriteString("# HELP bombvault_offsite_immutable Whether a domain's off-site repo is flagged append-only (1) or not (0).\n")
	b.WriteString("# TYPE bombvault_offsite_immutable gauge\n")
	for _, d := range statuses {
		if !d.Enabled {
			continue
		}
		fmt.Fprintf(&b, "bombvault_offsite_immutable{domain=\"%s\"} %d\n",
			escapeLabelValue(d.Domain), boolMetric(d.OffsiteImmutable))
	}
	b.WriteString("# HELP bombvault_tamper_test_ok Whether the last off-site tamper test proved append-only protection (1) or not (0).\n")
	b.WriteString("# TYPE bombvault_tamper_test_ok gauge\n")
	for _, d := range statuses {
		if !d.Enabled || !d.OffsiteImmutable {
			continue
		}
		fmt.Fprintf(&b, "bombvault_tamper_test_ok{domain=\"%s\"} %d\n",
			escapeLabelValue(d.Domain), boolMetric(d.LastTamperOK))
	}
	b.WriteString("# HELP bombvault_offsite_last_replication_timestamp_seconds Unix time of the last successful off-site replication per domain (0 if none).\n")
	b.WriteString("# TYPE bombvault_offsite_last_replication_timestamp_seconds gauge\n")
	for _, d := range statuses {
		if !d.Enabled {
			continue
		}
		fmt.Fprintf(&b, "bombvault_offsite_last_replication_timestamp_seconds{domain=\"%s\"} %d\n",
			escapeLabelValue(d.Domain), d.LastReplicationAt)
	}

	// Every domain and status pair gets a series, 0 when there were no runs, in a
	// stable order.
	b.WriteString("# HELP bombvault_runs_total Total number of finished backup runs per domain and status.\n")
	b.WriteString("# TYPE bombvault_runs_total counter\n")
	domains := make([]string, 0, len(statuses))
	for _, d := range statuses {
		domains = append(domains, d.Domain)
	}
	sort.Strings(domains)
	for _, domain := range domains {
		for _, status := range []string{"success", "failed"} {
			fmt.Fprintf(&b, "bombvault_runs_total{domain=\"%s\",status=\"%s\"} %d\n",
				escapeLabelValue(domain), status, runCounts[domain][status])
		}
	}

	if err := s.writeDBDumpMetrics(&b); err != nil {
		return "", err
	}
	return b.String(), nil
}

// writeDBDumpMetrics adds the database dump families. A dump never turns its
// container red, so alerting on a database that stopped being dumped needs
// series of its own.
func (s *Service) writeDBDumpMetrics(b *strings.Builder) error {
	dumpCounts, err := s.store.RunCountsOfKind("dbdump")
	if err != nil {
		return fmt.Errorf("metrics: database dump counts: %w", err)
	}
	b.WriteString("# HELP bombvault_dbdump_runs_total Total number of finished database dump runs per status.\n")
	b.WriteString("# TYPE bombvault_dbdump_runs_total counter\n")
	for _, status := range []string{"success", "failed"} {
		fmt.Fprintf(b, "bombvault_dbdump_runs_total{status=\"%s\"} %d\n", status, dumpCounts["containers"][status])
	}

	targets, err := s.store.ListTargets()
	if err != nil {
		return fmt.Errorf("metrics: targets: %w", err)
	}
	var body strings.Builder
	for _, t := range targets {
		at, lErr := s.store.LastSuccessOfKind(t.ID, "dbdump")
		if lErr != nil {
			return fmt.Errorf("metrics: last database dump of %s: %w", t.ContainerName, lErr)
		}
		if at == 0 {
			continue
		}
		fmt.Fprintf(&body, "bombvault_dbdump_last_success_timestamp_seconds{container=\"%s\"} %d\n",
			escapeLabelValue(t.ContainerName), at)
	}
	if body.Len() > 0 {
		b.WriteString("# HELP bombvault_dbdump_last_success_timestamp_seconds Unix time of the last successful database dump per container.\n")
		b.WriteString("# TYPE bombvault_dbdump_last_success_timestamp_seconds gauge\n")
		b.WriteString(body.String())
	}
	return nil
}

// mcpMetrics renders the MCP series, which let an operator alert on a looping
// or hostile client. The request counters are process-local and reset on a
// restart, like every other counter here.
func (h *Handler) mcpMetrics() string {
	h.mcp.countMu.Lock()
	requests := make(map[string]uint64, len(h.mcp.requests))
	for outcome, n := range h.mcp.requests {
		requests[outcome] = n
	}
	toolCalls := make(map[mcpToolOutcome]uint64, len(h.mcp.toolCalls))
	for call, n := range h.mcp.toolCalls {
		toolCalls[call] = n
	}
	h.mcp.countMu.Unlock()

	outcomes := make([]string, 0, len(requests))
	for outcome := range requests {
		outcomes = append(outcomes, outcome)
	}
	sort.Strings(outcomes)

	var b strings.Builder
	b.WriteString("# HELP bombvault_mcp_requests_total Requests to the MCP endpoint per gate outcome.\n")
	b.WriteString("# TYPE bombvault_mcp_requests_total counter\n")
	for _, outcome := range outcomes {
		fmt.Fprintf(&b, "bombvault_mcp_requests_total{outcome=\"%s\"} %d\n",
			escapeLabelValue(outcome), requests[outcome])
	}

	if len(toolCalls) > 0 {
		calls := make([]mcpToolOutcome, 0, len(toolCalls))
		for call := range toolCalls {
			calls = append(calls, call)
		}
		sort.Slice(calls, func(i, j int) bool {
			if calls[i].tool != calls[j].tool {
				return calls[i].tool < calls[j].tool
			}
			return calls[i].outcome < calls[j].outcome
		})
		b.WriteString("# HELP bombvault_mcp_tool_calls_total Tool calls per tool and outcome.\n")
		b.WriteString("# TYPE bombvault_mcp_tool_calls_total counter\n")
		for _, call := range calls {
			fmt.Fprintf(&b, "bombvault_mcp_tool_calls_total{tool=\"%s\",outcome=\"%s\"} %d\n",
				escapeLabelValue(call.tool), escapeLabelValue(call.outcome), toolCalls[call])
		}
	}

	keys, err := h.store.ActiveMCPKeys()
	if err != nil {
		log.Printf("api: mcp: could not count the active keys for /metrics: %v", err)
		return b.String()
	}
	b.WriteString("# HELP bombvault_mcp_active_keys Number of MCP keys that can currently authenticate.\n")
	b.WriteString("# TYPE bombvault_mcp_active_keys gauge\n")
	fmt.Fprintf(&b, "bombvault_mcp_active_keys %d\n", len(keys))
	return b.String()
}

func boolMetric(v bool) int {
	if v {
		return 1
	}
	return 0
}
