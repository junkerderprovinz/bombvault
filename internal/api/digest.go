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

// digestWindow is how far back the digest looks, whatever its schedule, so
// moving the schedule never narrows or widens the window.
const digestWindow = 7 * 24 * time.Hour

// digestMaxFailures is how many failed runs the digest lists before collapsing
// the rest into "+N more".
const digestMaxFailures = 5

// digestKindOrder keeps the count lines in the same order every week.
var digestKindOrder = []string{"backup", "dbdump", "dbdumpsave", "dbimport", "restore", "update", "prune", "verify", "offsite", "drill", "drdrill", "tamper", "export"}

// digestKindCount is one kind's finished-run tally inside the digest window.
type digestKindCount struct {
	OK     int
	Failed int
}

// digestOffsiteLine is one domain's last successful replication (0 = never).
// Stale means older than twice the expected period, the factor the tamper
// scorecard uses.
type digestOffsiteLine struct {
	Domain string
	LastOK int64
	Stale  bool
}

// digestStats is everything composeDigest needs, collected up front so that
// composing is a pure function.
type digestStats struct {
	// Now is the reference time for relative ages, in unix seconds.
	Now int64
	// Kinds counts finished runs per kind; kinds without runs are absent.
	// TotalFailed is the sum of every kind's Failed.
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

// digestBackupScheduleFor returns a domain's local backup schedule, which sets
// the replication cadence when the domain has no off-site schedule of its own.
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

// runTargetNames maps run target ids to display names, as handleRuns does.
// Unknown ids stay unresolved.
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

// collectDigestStats gathers the finished runs of the last digestWindow and the
// replication age of each off-site domain. RunsSince is bounded by time, so the
// ListRuns row cap does not apply.
func (s *Service) collectDigestStats(now time.Time) (digestStats, error) {
	stats := digestStats{Now: now.Unix(), Kinds: map[string]digestKindCount{}}

	runs, err := s.store.RunsSince(now.Add(-digestWindow).Unix())
	if err != nil {
		return digestStats{}, fmt.Errorf("read runs: %w", err)
	}
	names := s.runTargetNames()
	for _, run := range runs {
		// A running run has no verdict yet, and a skipped one is neither success
		// nor failure.
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
					// Domain-scoped runs use the domain name as their target id.
					name = run.TargetID
				}
				reason := shareableRunError(run.Kind, run.Error)
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
			// Without its own off-site schedule a domain replicates after each
			// local backup, including the Backup Everything pass. A domain covered
			// only by that pass would otherwise have period 0 and never go stale.
			period := cadencePeriodSeconds(s.offsiteScheduleFor(domain, settings))
			if period == 0 {
				period, _ = domainCoverage(digestBackupScheduleFor(domain, settings), settings.EverythingSchedule)
			}
			if period > 0 && stats.Now-line.LastOK > 2*period {
				line.Stale = true
			}
		}
		stats.Offsite = append(stats.Offsite, line)
	}
	return stats, nil
}

// digestAge renders the age of at relative to now, both unix seconds, as
// "5m ago", "3h ago" or "2d ago".
func digestAge(now, at int64) string {
	d := now - at
	if d < 0 {
		d = 0 // clock skew
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

// composeDigest renders stats as the plaintext digest message.
func composeDigest(stats digestStats) string {
	var b strings.Builder
	b.WriteString("BombVault weekly digest, last 7 days\n")

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
		// Kinds missing from digestKindOrder still show up, after the known ones.
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
				fmt.Fprintf(&b, "- %s: stale, last successful copy %s\n", line.Domain, digestAge(stats.Now, line.LastOK))
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

// SendDigest sends the weekly digest to the notify channels and the Unraid
// mirror without recording a run. The Healthchecks ping is suppressed because
// a summary must not flip a domain check. A muted policy sends nothing, and
// On="failure" lets notify.Send drop an all-green digest.
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
	if s.unraidGate(c.Unraid) && (c.On == "always" || !ok) {
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
