package api

import (
	"fmt"
	"sort"
	"strings"
)

// metricsContentType is the Prometheus text exposition format media type.
const metricsContentType = "text/plain; version=0.0.4; charset=utf-8"

// MetricsAccess reports whether the opt-in /metrics endpoint is enabled and, if
// so, the optional bearer token that scrapes must present (empty = open). It
// reads settings directly so the handler can gate the endpoint without exposing
// the whole settings struct. A store error yields (false, "", err) so the
// handler can fail closed.
func (s *Service) MetricsAccess() (enabled bool, token string, err error) {
	settings, err := s.store.GetSettings()
	if err != nil {
		return false, "", err
	}
	return settings.MetricsEnabled, settings.MetricsToken, nil
}

// escapeLabelValue escapes a Prometheus label value per the exposition format:
// backslash, double-quote, and newline. The label values BombVault emits are
// fixed enums (domain/source/status) plus the build version, none of which can
// legitimately contain these, but the helper keeps the output well-formed for
// any value regardless.
func escapeLabelValue(v string) string {
	r := strings.NewReplacer(
		`\`, `\\`,
		`"`, `\"`,
		"\n", `\n`,
	)
	return r.Replace(v)
}

// Metrics builds the Prometheus text exposition for the operational metrics:
// build info, per-domain last-success timestamp + enabled flag, latest local
// repo size + snapshot count, and per-domain backup run counts. It exposes only
// non-sensitive values (no repo paths, secrets, or hostnames). Errors reading
// the store are returned so the caller can answer 500; a missing repo stat for a
// domain is simply omitted (not an error).
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

	// build info — Version is the ldflags-injected build version.
	b.WriteString("# HELP bombvault_build_info BombVault build information.\n")
	b.WriteString("# TYPE bombvault_build_info gauge\n")
	fmt.Fprintf(&b, "bombvault_build_info{version=\"%s\"} 1\n", escapeLabelValue(Version))

	// last successful backup timestamp per domain (0 = none yet).
	b.WriteString("# HELP bombvault_backup_last_success_timestamp_seconds Unix time of the last successful backup per domain (0 if none).\n")
	b.WriteString("# TYPE bombvault_backup_last_success_timestamp_seconds gauge\n")
	for _, d := range statuses {
		fmt.Fprintf(&b, "bombvault_backup_last_success_timestamp_seconds{domain=\"%s\"} %d\n",
			escapeLabelValue(d.Domain), d.LastSuccess)
	}

	// domain enabled flag.
	b.WriteString("# HELP bombvault_domain_enabled Whether a backup domain is enabled (1) or disabled (0).\n")
	b.WriteString("# TYPE bombvault_domain_enabled gauge\n")
	for _, d := range statuses {
		fmt.Fprintf(&b, "bombvault_domain_enabled{domain=\"%s\"} %d\n",
			escapeLabelValue(d.Domain), boolMetric(d.Enabled))
	}

	// latest local repo size (bytes) + snapshot count per domain. Skipped when no
	// sample has been recorded yet for that domain.
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

	// ransomware-protection gauges per domain (from DomainStatus). Emitted for
	// every domain so a scraper always sees a series: off-site append-only flag,
	// the last tamper-test outcome (1 = append-only proven, 0 = not/unknown), and
	// the last off-site replication timestamp (0 = never replicated).
	b.WriteString("# HELP bombvault_offsite_immutable Whether a domain's off-site repo is flagged append-only (1) or not (0).\n")
	b.WriteString("# TYPE bombvault_offsite_immutable gauge\n")
	for _, d := range statuses {
		fmt.Fprintf(&b, "bombvault_offsite_immutable{domain=\"%s\"} %d\n",
			escapeLabelValue(d.Domain), boolMetric(d.OffsiteImmutable))
	}
	b.WriteString("# HELP bombvault_tamper_test_ok Whether the last off-site tamper test proved append-only protection (1) or not (0).\n")
	b.WriteString("# TYPE bombvault_tamper_test_ok gauge\n")
	for _, d := range statuses {
		fmt.Fprintf(&b, "bombvault_tamper_test_ok{domain=\"%s\"} %d\n",
			escapeLabelValue(d.Domain), boolMetric(d.LastTamperOK))
	}
	b.WriteString("# HELP bombvault_offsite_last_replication_timestamp_seconds Unix time of the last off-site replication per domain (0 if none).\n")
	b.WriteString("# TYPE bombvault_offsite_last_replication_timestamp_seconds gauge\n")
	for _, d := range statuses {
		fmt.Fprintf(&b, "bombvault_offsite_last_replication_timestamp_seconds{domain=\"%s\"} %d\n",
			escapeLabelValue(d.Domain), d.LastReplicationAt)
	}

	// backup run counts per domain + status. Emitted for every domain/status pair
	// so a scraper always sees a series (0 when none), in a stable order.
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

	return b.String(), nil
}

// boolMetric maps a bool to the Prometheus 1/0 convention.
func boolMetric(v bool) int {
	if v {
		return 1
	}
	return 0
}
