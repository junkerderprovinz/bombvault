package api

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/notify"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// The overdue-backup watchdog runs once a day and notifies when a domain's
// backups are overdue, which is otherwise visible only on the dashboard. It
// uses rpoStatus, the predicate behind the dashboard's protection status, so
// the two cannot disagree. Each overdue episode notifies once:
// store.WatchdogState keeps the last success the alert was based on, and a
// newer success starts a new episode.

// watchdogDecision returns what RunWatchdog does for one domain. "never" and
// "warn" are not reported, because the watchdog alerts on backups that
// stopped, not on setups that have not started.
func watchdogDecision(now, lastSuccess, periodSeconds int64, enabled bool, state store.WatchdogState, haveState bool) (notifyNeeded, clearState bool) {
	if rpoStatus(now, lastSuccess, periodSeconds, enabled && periodSeconds > 0) != "overdue" {
		return false, haveState
	}
	if haveState && state.LastSuccessAt == lastSuccess {
		return false, false
	}
	return true, false
}

// watchdogPeriod formats an RPO period as "1d", "12h" or "45m" for the overdue
// message.
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

// RunWatchdog runs the daily overdue-backup check across all domains.
func (s *Service) RunWatchdog(ctx context.Context) error {
	return s.runWatchdogAt(ctx, time.Now().Unix())
}

// runWatchdogAt is RunWatchdog at a given time. With notifications muted it
// does nothing, because recording episodes it cannot deliver would suppress
// the alert once the user turns notifications on.
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

	// The same domains DomainStatus shows on the dashboard.
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
		// A domain backed up only by the Backup Everything pass has no cadence of
		// its own, and period 0 would disarm the watchdog for it.
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
		// Recording after the send means a failed record costs one duplicate
		// alert on the next run instead of a lost one.
		if uErr := s.store.UpsertWatchdogState(store.WatchdogState{Domain: d.name, NotifiedAt: now, LastSuccessAt: lastUnix}); uErr != nil {
			log.Printf("api: watchdog: %s state record failed: %v", d.name, uErr) //nolint:gosec // G706: domain is a fixed literal
		}
	}
	return nil
}

// notifyBackupOverdue sends the overdue alert to the message channels and the
// Unraid mirror. The Healthchecks ping is suppressed: the alert is about runs
// that did not happen, and a /fail ping would break the check's start/success
// pairing.
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
