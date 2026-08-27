package api

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/notify"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// The overdue-backup watchdog: a daily scheduled check (schedule.WatchdogCadence,
// gated on Settings.WatchdogEnabled) that actively NOTIFIES when a domain's
// backups are overdue — a state that is otherwise only visible on the dashboard,
// which nobody looks at precisely when backups have quietly stopped. "Overdue"
// is decided by rpoStatus, the EXACT predicate the dashboard's protection status
// uses (age > 2× the cadence period), so the push and the dashboard can never
// disagree. Each overdue episode notifies ONCE (store.WatchdogState remembers
// the last-success timestamp the verdict was based on); a new success ends the
// episode and re-arms the watchdog.

// watchdogDecision is the pure per-domain verdict RunWatchdog acts on, so the
// once-per-episode dedupe is unit-testable without a store, clock or notify
// transport:
//
//   - not overdue (by rpoStatus — the dashboard's own predicate): never notify;
//     clear any recorded episode (the domain recovered, re-arming the watchdog).
//     "never" and "warn" are deliberately not push-worthy — the watchdog alerts
//     REGRESSIONS (backups that stopped), not setups that have not started.
//   - overdue with a recorded episode based on the SAME last success: the one
//     notification for this episode already went out — stay quiet.
//   - overdue otherwise (no state, or the last success changed since): notify.
func watchdogDecision(now, lastSuccess, periodSeconds int64, enabled bool, state store.WatchdogState, haveState bool) (notifyNeeded, clearState bool) {
	if rpoStatus(now, lastSuccess, periodSeconds, enabled && periodSeconds > 0) != "overdue" {
		return false, haveState
	}
	if haveState && state.LastSuccessAt == lastSuccess {
		return false, false
	}
	return true, false
}

// watchdogPeriod renders an RPO period compactly ("1d", "12h", "45m") for the
// overdue message's "expected every …" clause.
func watchdogPeriod(seconds int64) string {
	switch {
	case seconds%86400 == 0:
		return fmt.Sprintf("%dd", seconds/86400)
	case seconds%3600 == 0:
		return fmt.Sprintf("%dh", seconds/3600)
	default:
		return fmt.Sprintf("%dm", seconds/60)
	}
}

// RunWatchdog performs the daily overdue-backup check across all domains. It is
// the scheduler's watchdog job (SetWatchdogJob in cmd/bombvault/main.go).
func (s *Service) RunWatchdog(ctx context.Context) error {
	return s.runWatchdogAt(ctx, time.Now().Unix())
}

// runWatchdogAt is the testable core of RunWatchdog with an injectable "now".
// A muted notify policy (On empty/"never") skips silently — with no way to
// deliver, recording episodes would only suppress the alert the user asked for
// by enabling notifications later.
func (s *Service) runWatchdogAt(ctx context.Context, now int64) error {
	c, err := s.NotifyConfig()
	if err != nil {
		return fmt.Errorf("read notify config: %w", err)
	}
	if c.On == "" || c.On == "never" {
		return nil
	}
	settings, err := s.store.GetSettings()
	if err != nil {
		return fmt.Errorf("read settings: %w", err)
	}

	// Same domain table DomainStatus drives the dashboard with, so the watchdog
	// judges exactly what the protection chips show.
	domains := []struct {
		name     string
		enabled  bool
		schedule string
		lastFn   func() (time.Time, error)
	}{
		{"containers", settings.ContainersEnabled, settings.ContainersSchedule, s.store.LastSuccessfulContainerBackup},
		{"vms", settings.VMsEnabled, settings.VMsSchedule, s.store.LastSuccessfulVMBackup},
		{"flash", settings.FlashEnabled, settings.FlashSchedule, s.store.LastSuccessfulFlashBackup},
		{"config", settings.ConfigEnabled, settings.ConfigSchedule, s.store.LastSuccessfulConfigBackup},
		{"files", settings.FilesEnabled, settings.FilesSchedule, s.store.LastSuccessfulFilesBackup},
	}

	for _, d := range domains {
		last, lErr := d.lastFn()
		if lErr != nil {
			log.Printf("api: watchdog: %s last-success query failed (skipping): %v", d.name, lErr) //nolint:gosec // G706: domain is a fixed literal
			continue
		}
		var lastUnix int64
		if !last.IsZero() {
			lastUnix = last.Unix()
		}
		// Same coverage rule DomainStatus uses, and it bites harder here: a domain
		// backed up only by the "Backup Everything" pass has no cadence of its
		// own, so reading that cadence alone gave period 0 — and period 0 means
		// "no expectation", which switches this watchdog OFF for that domain. A
		// user who runs the whole server through the pass, the configuration
		// #177's reporter is in, had no dead-man's switch on any domain at all,
		// and nothing said so.
		period, _ := domainCoverage(d.schedule, settings.EverythingSchedule)

		state, haveState, sErr := s.store.GetWatchdogState(d.name)
		if sErr != nil {
			log.Printf("api: watchdog: %s state read failed (skipping): %v", d.name, sErr) //nolint:gosec // G706: domain is a fixed literal
			continue
		}
		notifyNeeded, clearState := watchdogDecision(now, lastUnix, period, d.enabled, state, haveState)
		if clearState {
			if dErr := s.store.DeleteWatchdogState(d.name); dErr != nil {
				log.Printf("api: watchdog: %s state clear failed: %v", d.name, dErr) //nolint:gosec // G706: domain is a fixed literal
			}
		}
		if !notifyNeeded {
			continue
		}
		s.notifyBackupOverdue(ctx, c, d.name, lastUnix, period, now)
		// Record the episode AFTER the send: a failed record means at worst one
		// duplicate alert on the next fire — better than a silently lost episode.
		if uErr := s.store.UpsertWatchdogState(store.WatchdogState{Domain: d.name, NotifiedAt: now, LastSuccessAt: lastUnix}); uErr != nil {
			log.Printf("api: watchdog: %s state record failed: %v", d.name, uErr) //nolint:gosec // G706: domain is a fixed literal
		}
	}
	return nil
}

// notifyBackupOverdue sends the overdue alert through the established notify
// fan-out (message channels + the Unraid mirror, like notifyReplicationFailed).
// The Healthchecks ping is suppressed: this is a human alert about MISSING
// runs, not a run lifecycle event — a /fail ping here would corrupt the
// domain check's start/success pairing.
func (s *Service) notifyBackupOverdue(ctx context.Context, c notify.Config, domain string, lastSuccess, period, now int64) {
	msg := fmt.Sprintf("Backups for %s are overdue: last success %s, expected every %s.",
		domain, digestAge(now, lastSuccess), watchdogPeriod(period))
	notify.Send(notify.WithHealthchecksSuppressed(ctx), c, domain,
		notify.Event{Title: "BombVault", Message: msg, OK: false})
	if s.unraidGate(c.Unraid) {
		if e := s.sendUnraidNotify(ctx, "BombVault: backups overdue for "+domain, msg, "warning"); e != nil {
			log.Printf("notify: unraid: %v", e)
		}
	}
}
