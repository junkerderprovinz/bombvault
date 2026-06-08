// Package schedule provides a per-domain in-process scheduler backed by
// github.com/robfig/cron/v3. Each domain (containers / VMs / flash) has its
// own cadence parsed from the settings row.
package schedule

import (
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"

	"github.com/robfig/cron/v3"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

// BackupFunc is the function called for each container that is due for backup.
// It is injected so the scheduler is unit-testable.
type BackupFunc func(containerName string) error

// ListTargetsFunc returns the current list of targets.
type ListTargetsFunc func() ([]store.Target, error)

// ParseCadence converts a user-facing cadence string into a 5-field cron
// expression. Recognised forms:
//
//   - "off"              → spec="", enabled=false
//   - "daily HH:MM"      → "MM HH * * *",  enabled=true
//   - "weekly DOW HH:MM" → "MM HH * * N",  enabled=true  (DOW = Sun–Sat)
//   - raw 5-field cron   → passed through,  enabled=true
//
// Any other input returns an error.
func ParseCadence(s string) (spec string, enabled bool, err error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", false, errors.New("schedule: empty cadence string")
	}

	parts := strings.Fields(s)

	switch parts[0] {
	case "off":
		if len(parts) != 1 {
			return "", false, fmt.Errorf("schedule: unexpected tokens after 'off'")
		}
		return "", false, nil

	case "daily":
		if len(parts) != 2 {
			return "", false, fmt.Errorf("schedule: 'daily' requires exactly one HH:MM argument")
		}
		h, m, parseErr := parseHHMM(parts[1])
		if parseErr != nil {
			return "", false, fmt.Errorf("schedule: invalid time %q: %w", parts[1], parseErr)
		}
		return fmt.Sprintf("%d %d * * *", m, h), true, nil

	case "weekly":
		if len(parts) != 3 {
			return "", false, fmt.Errorf("schedule: 'weekly' requires DOW and HH:MM arguments")
		}
		dow, dowErr := parseDOW(parts[1])
		if dowErr != nil {
			return "", false, fmt.Errorf("schedule: invalid day-of-week %q: %w", parts[1], dowErr)
		}
		h, m, parseErr := parseHHMM(parts[2])
		if parseErr != nil {
			return "", false, fmt.Errorf("schedule: invalid time %q: %w", parts[2], parseErr)
		}
		return fmt.Sprintf("%d %d * * %d", m, h, dow), true, nil

	default:
		// Accept a raw 5-field cron expression.
		if len(parts) != 5 {
			return "", false, fmt.Errorf("schedule: unrecognised cadence %q (expected 'off', 'daily HH:MM', 'weekly DOW HH:MM', or a 5-field cron)", s)
		}
		// Validate it parses correctly.
		parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
		if _, parseErr := parser.Parse(s); parseErr != nil {
			return "", false, fmt.Errorf("schedule: invalid cron expression %q: %w", s, parseErr)
		}
		return s, true, nil
	}
}

// parseHHMM splits "HH:MM" into (hour, minute) integers and validates ranges.
func parseHHMM(s string) (h, m int, err error) {
	parts := strings.SplitN(s, ":", 2)
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("expected HH:MM, got %q", s)
	}
	h, err = strconv.Atoi(parts[0])
	if err != nil || h < 0 || h > 23 {
		return 0, 0, fmt.Errorf("invalid hour in %q", s)
	}
	m, err = strconv.Atoi(parts[1])
	if err != nil || m < 0 || m > 59 {
		return 0, 0, fmt.Errorf("invalid minute in %q", s)
	}
	return h, m, nil
}

// dowMap maps 3-letter day abbreviations to cron DOW numbers (Sun=0 … Sat=6).
var dowMap = map[string]int{
	"Sun": 0, "Mon": 1, "Tue": 2, "Wed": 3,
	"Thu": 4, "Fri": 5, "Sat": 6,
}

func parseDOW(s string) (int, error) {
	n, ok := dowMap[s]
	if !ok {
		return 0, fmt.Errorf("unknown day %q (expected Sun Mon Tue Wed Thu Fri Sat)", s)
	}
	return n, nil
}

// ---------------------------------------------------------------------------
// Scheduler
// ---------------------------------------------------------------------------

// Scheduler manages per-domain cron entries using robfig/cron/v3.
type Scheduler struct {
	c        *cron.Cron
	backup   BackupFunc
	listFn   ListTargetsFunc
	entryIDs []cron.EntryID
}

// New creates a Scheduler. backupFn is called for each due container;
// listFn retrieves the current target list when the job fires.
func New(backupFn BackupFunc, listFn ListTargetsFunc) *Scheduler {
	return &Scheduler{
		c:      cron.New(),
		backup: backupFn,
		listFn: listFn,
	}
}

// Start starts the underlying cron runner. Call once at app startup.
func (s *Scheduler) Start() {
	s.c.Start()
}

// Stop halts the scheduler and waits for running jobs to finish.
func (s *Scheduler) Stop() {
	s.c.Stop()
}

// Reload re-reads the schedule settings and re-registers all domain entries.
// It removes any previously registered entries first, so it is safe to call
// repeatedly (e.g. after a settings change).
func (s *Scheduler) Reload(settings store.Settings) error {
	// Remove all existing entries.
	for _, id := range s.entryIDs {
		s.c.Remove(id)
	}
	s.entryIDs = s.entryIDs[:0]

	// Register enabled domains.
	domains := []struct {
		cadence string
		name    string
		fn      func()
	}{
		{
			cadence: settings.ContainersSchedule,
			name:    "containers",
			fn: func() {
				targets, err := s.listFn()
				if err != nil {
					log.Printf("schedule: containers job: list targets: %v", err)
					return
				}
				RunContainersJob(targets, s.backup)
			},
		},
		// VMs and Flash cadences are stored but their jobs are no-ops in Phase 1.
		{
			cadence: settings.VMsSchedule,
			name:    "vms",
			fn:      func() { log.Print("schedule: vms job: not yet implemented in Phase 1") },
		},
		{
			cadence: settings.FlashSchedule,
			name:    "flash",
			fn:      func() { log.Print("schedule: flash job: not yet implemented in Phase 1") },
		},
	}

	for _, d := range domains {
		spec, enabled, err := ParseCadence(d.cadence)
		if err != nil {
			return fmt.Errorf("schedule: domain %s: %w", d.name, err)
		}
		if !enabled {
			continue
		}
		domainName := d.name
		jobFn := d.fn
		id, err := s.c.AddFunc(spec, func() {
			log.Printf("schedule: running %s job", domainName)
			jobFn()
		})
		if err != nil {
			return fmt.Errorf("schedule: domain %s: add cron entry: %w", d.name, err)
		}
		s.entryIDs = append(s.entryIDs, id)
	}

	return nil
}

// RunContainersJob backs up each target that has IncludeInSchedule=true,
// calling backupFn sequentially. Errors from individual containers are logged
// but do not abort the remaining containers.
//
// This function is exported so tests can invoke the job synchronously without
// waiting for real wall-clock time.
func RunContainersJob(targets []store.Target, backupFn BackupFunc) {
	for _, t := range targets {
		if !t.IncludeInSchedule {
			continue
		}
		if err := backupFn(t.ContainerName); err != nil {
			log.Printf("schedule: containers job: backup %q failed: %v", t.ContainerName, err)
		}
	}
}
