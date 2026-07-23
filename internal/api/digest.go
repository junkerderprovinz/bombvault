package api

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/notify"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// digestWindow is how far back the weekly digest looks. The digest fires on its
// own cadence (DigestSchedule, weekly by default), and always summarises the
// trailing 7 days regardless of when it fires, so a rescheduled digest never
// silently narrows or widens its window.
const digestWindow = 7 * 24 * time.Hour

// digestMaxFailures caps how many failed runs the digest enumerates
// individually before collapsing the rest into a "+N more" tail — same idea as
// the scheduled summary's maxListedFailures, tighter because the digest already
// carries the per-kind counts.
const digestMaxFailures = 5

// digestKindOrder fixes the print order of the per-kind count lines so the
// digest reads stably week over week (map iteration order is random).
var digestKindOrder = []string{"backup", "restore", "update", "prune", "verify", "offsite", "drill", "tamper", "export"}

// digestKindCount is one kind's finished-run tally inside the digest window.
type digestKindCount struct {
	OK     int
	Failed int
}

// digestOffsiteLine is one domain's off-site currency verdict: when the last
// SUCCESSFUL replication landed (0 = never) and whether that is stale relative
// to the domain's replication cadence (older than 2× the expected period —
// the same staleness factor the tamper scorecard uses).
type digestOffsiteLine struct {
	Domain string
	LastOK int64
	Stale  bool
}

// digestStats is everything composeDigest needs, collected up front so the
// compose step is a pure, unit-testable function of plain data.
type digestStats struct {
	// Now pins "now" (unix seconds) so relative ages in the composed text are
	// deterministic for a given stats value.
	Now int64
	// Kinds tallies finished runs per runs.kind; kinds with no activity are
	// absent. TotalFailed is the sum of every kind's Failed.
	Kinds       map[string]digestKindCount
	TotalFailed int
	// BackupBytes sums the bytes added by successful backup runs in the window.
	BackupBytes int64
	// Offsite carries one currency line per domain with an off-site repo.
	Offsite []digestOffsiteLine
	// Failures are up to digestMaxFailures pre-rendered "kind name: reason"
	// lines (newest first); MoreFailures counts the collapsed remainder.
	Failures     []string
	MoreFailures int
}

// digestBackupScheduleFor returns a domain's LOCAL backup cadence — the
// replication expectation when the off-site schedule is blank (coupled:
// replicate after every local backup).
func digestBackupScheduleFor(domain string, settings store.Settings) string {
	switch domain {
	case "containers":
		return settings.ContainersSchedule
	case "vms":
		return settings.VMsSchedule
	case "flash":
		return settings.FlashSchedule
	case "config":
		return settings.ConfigSchedule
	case "files":
		return settings.FilesSchedule
	}
	return ""
}

// runTargetNames resolves runs.target_id → human name, mirroring handleRuns'
// map (container targets, VM targets, file sets, plus the reserved flash/config
// ids). Best-effort: an unknown id just stays unresolved.
func (s *Service) runTargetNames() map[string]string {
	names := map[string]string{store.FlashTargetID: "Unraid flash", store.ConfigTargetID: "App configuration"}
	if cts, err := s.store.ListTargets(); err == nil {
		for _, t := range cts {
			names[t.ID] = t.ContainerName
		}
	}
	if vts, err := s.store.ListVMTargets(); err == nil {
		for _, t := range vts {
			names[t.ID] = t.Name
		}
	}
	if fss, err := s.store.ListFileSets(); err == nil {
		for _, fs := range fss {
			names[fs.ID] = fs.Name
		}
	}
	return names
}

// collectDigestStats gathers the digest's inputs: the last digestWindow of
// finished runs (RunsSince is time-bounded, so the ListRuns 500-row cap does
// not apply here) plus each off-site domain's replication currency.
func (s *Service) collectDigestStats(now time.Time) (digestStats, error) {
	stats := digestStats{Now: now.Unix(), Kinds: map[string]digestKindCount{}}

	runs, err := s.store.RunsSince(now.Add(-digestWindow).Unix())
	if err != nil {
		return digestStats{}, fmt.Errorf("read runs: %w", err)
	}
	names := s.runTargetNames()
	for _, run := range runs {
		// Only finished outcomes count; a still-running run has no verdict yet and
		// a skipped one is intentionally neither success nor failure (#57).
		switch run.Status {
		case "success":
			c := stats.Kinds[run.Kind]
			c.OK++
			stats.Kinds[run.Kind] = c
			if run.Kind == "backup" {
				stats.BackupBytes += run.Bytes
			}
		case "failed":
			c := stats.Kinds[run.Kind]
			c.Failed++
			stats.Kinds[run.Kind] = c
			stats.TotalFailed++
			if len(stats.Failures) < digestMaxFailures {
				name := names[run.TargetID]
				if name == "" {
					// Domain-scoped runs (prune/verify/offsite/drill/tamper) carry the
					// domain literal as their target id — already readable as-is.
					name = run.TargetID
				}
				reason := run.Error
				const maxReason = 160
				if len(reason) > maxReason {
					reason = reason[:maxReason]
				}
				stats.Failures = append(stats.Failures, fmt.Sprintf("%s %s: %s", run.Kind, name, reason))
			} else {
				stats.MoreFailures++
			}
		}
	}

	// Off-site currency per domain with a configured off-site repo: the age of
	// the last SUCCESSFUL replication versus its cadence (the off-site schedule
	// when set, else the coupled local backup schedule). Stale = older than 2×
	// the expected period, mirroring the scorecard's staleness factor.
	settings, err := s.store.GetSettings()
	if err != nil {
		return digestStats{}, fmt.Errorf("read settings: %w", err)
	}
	for _, domain := range []string{"containers", "vms", "flash", "config", "files"} {
		if s.offsiteRepoFor(domain, settings) == "" {
			continue
		}
		line := digestOffsiteLine{Domain: domain}
		if run, found, oErr := s.store.LatestSuccessfulOffsiteRun(domain); oErr != nil {
			log.Printf("api: digest: latest off-site run for %s: %v", domain, oErr) //nolint:gosec // G706: domain is a fixed literal
		} else if found {
			line.LastOK = run.FinishedAt
			schedule := s.offsiteScheduleFor(domain, settings)
			if schedule == "" {
				schedule = digestBackupScheduleFor(domain, settings)
			}
			if period := cadencePeriodSeconds(schedule); period > 0 && stats.Now-line.LastOK > 2*period {
				line.Stale = true
			}
		}
		stats.Offsite = append(stats.Offsite, line)
	}
	return stats, nil
}

// digestAge renders a unix timestamp as a compact "3h ago" / "2d ago" age
// relative to now (unix seconds), for the digest's off-site currency lines.
func digestAge(now, at int64) string {
	d := now - at
	if d < 0 {
		d = 0 // clock skew — never render a negative age
	}
	switch {
	case d < 3600:
		return fmt.Sprintf("%dm ago", d/60)
	case d < 86400:
		return fmt.Sprintf("%dh ago", d/3600)
	default:
		return fmt.Sprintf("%dd ago", d/86400)
	}
}

// composeDigest renders the collected stats as the compact plaintext digest
// message. Pure — same stats, same text — so it is unit-testable without a
// store, clock or notify transport.
func composeDigest(stats digestStats) string {
	var b strings.Builder
	b.WriteString("BombVault weekly digest — last 7 days\n")

	if len(stats.Kinds) == 0 {
		b.WriteString("No finished runs in this window.\n")
	} else {
		b.WriteString("Runs:\n")
		for _, kind := range digestKindOrder {
			c, ok := stats.Kinds[kind]
			if !ok {
				continue
			}
			fmt.Fprintf(&b, "- %s: %d ok, %d failed\n", kind, c.OK, c.Failed)
		}
		// A future/unknown kind still shows up rather than silently vanishing.
		for kind, c := range stats.Kinds {
			known := false
			for _, k := range digestKindOrder {
				if k == kind {
					known = true
					break
				}
			}
			if !known {
				fmt.Fprintf(&b, "- %s: %d ok, %d failed\n", kind, c.OK, c.Failed)
			}
		}
		if stats.BackupBytes > 0 {
			fmt.Fprintf(&b, "New backup data: %s\n", humanBytes(stats.BackupBytes))
		}
	}

	if len(stats.Offsite) > 0 {
		b.WriteString("Off-site currency:\n")
		for _, line := range stats.Offsite {
			switch {
			case line.LastOK == 0:
				fmt.Fprintf(&b, "- %s: no successful copy yet\n", line.Domain)
			case line.Stale:
				fmt.Fprintf(&b, "- %s: STALE — last successful copy %s\n", line.Domain, digestAge(stats.Now, line.LastOK))
			default:
				fmt.Fprintf(&b, "- %s: current (last copy %s)\n", line.Domain, digestAge(stats.Now, line.LastOK))
			}
		}
	}

	if len(stats.Failures) > 0 {
		b.WriteString("Failures:\n")
		for _, f := range stats.Failures {
			b.WriteString("- " + f + "\n")
		}
		if stats.MoreFailures > 0 {
			fmt.Fprintf(&b, "(+%d more)\n", stats.MoreFailures)
		}
	} else {
		b.WriteString("No failures. All good.\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

// SendDigest composes and sends the weekly digest through the existing notify
// fan-out (message channels + the Unraid mirror, like notifyReplicationFailed).
// It records nothing in runs — the digest reports history, it is not history.
// The Healthchecks ping is suppressed: the digest is a human summary, never a
// monitor lifecycle event, so it must not flip a domain check. A muted policy
// (On empty/"never") skips silently; On="failure" lets notify.Send drop an
// all-green digest per the house policy gate.
func (s *Service) SendDigest(ctx context.Context) error {
	c, err := s.NotifyConfig()
	if err != nil {
		return fmt.Errorf("read notify config: %w", err)
	}
	if c.On == "" || c.On == "never" {
		return nil
	}
	stats, err := s.collectDigestStats(time.Now())
	if err != nil {
		return err
	}
	ok := stats.TotalFailed == 0
	msg := composeDigest(stats)
	notify.Send(notify.WithHealthchecksSuppressed(ctx), c, "digest",
		notify.Event{Title: "BombVault", Message: msg, OK: ok})
	if c.Unraid && s.ssh != nil && (c.On == "always" || !ok) {
		level := "normal"
		if !ok {
			level = "warning"
		}
		if e := s.sendUnraidNotify(ctx, "BombVault: weekly digest", msg, level); e != nil {
			log.Printf("notify: unraid: %v", e)
		}
	}
	return nil
}
