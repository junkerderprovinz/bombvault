// Package schedule provides a per-domain in-process scheduler backed by
// github.com/robfig/cron/v3. Each domain (containers / VMs / flash) has its
// own cadence parsed from the settings row.
package schedule

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/robfig/cron/v3"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

// BackupFunc is the function called for each container that is due for backup.
// It is injected so the scheduler is unit-testable.
type BackupFunc func(containerName string) error

// ListTargetsFunc returns the current list of targets.
type ListTargetsFunc func() ([]store.Target, error)

// LastRunFunc returns the time of the last successful backup for a domain, or
// a zero time when there has been none. It is injected so the schedule package
// stays store-free (DI seam).
type LastRunFunc func() (time.Time, error)

// Cadence is the parsed result of a cadence string.
//
//   - Enabled=false: the domain is off (Spec is empty, IntervalDays is 0).
//   - Enabled=true, IntervalDays=0: a regular cron spec fires unconditionally.
//   - Enabled=true, IntervalDays>0: the spec is a daily trigger (fires once per
//     day at the given HH:MM) but the job must consult a due-check before
//     doing any real work — only proceed if now − last-successful-run ≥ IntervalDays.
type Cadence struct {
	Spec         string // 5-field cron expression; empty when Enabled=false
	Enabled      bool
	IntervalDays int // >0 for everyN cadences only
}

// ParseCadence converts a user-facing cadence string into a Cadence.
// Recognised forms:
//
//   - "off"                        → Cadence{Enabled:false}
//   - "daily HH:MM"                → daily cron spec, unconditional
//   - "weekly DOW[,DOW,...] HH:MM" → weekly on named days; DOW = Sun–Sat
//     (single or comma-separated set, case-insensitive)
//   - "everyN <N> HH:MM"           → daily cron spec + IntervalDays=N (N ≥ 1)
//   - raw 5-field cron              → passed through unconditionally
//
// Any other input returns an error.
func ParseCadence(s string) (Cadence, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		// Treat empty string as "off" — defensive against settings PUT with "".
		return Cadence{}, nil
	}

	parts := strings.Fields(s)

	switch parts[0] {
	case "off":
		if len(parts) != 1 {
			return Cadence{}, fmt.Errorf("schedule: unexpected tokens after 'off'")
		}
		return Cadence{}, nil

	case "daily":
		if len(parts) != 2 {
			return Cadence{}, fmt.Errorf("schedule: 'daily' requires exactly one HH:MM argument")
		}
		h, m, parseErr := parseHHMM(parts[1])
		if parseErr != nil {
			return Cadence{}, fmt.Errorf("schedule: invalid time %q: %w", parts[1], parseErr)
		}
		return Cadence{Spec: fmt.Sprintf("%d %d * * *", m, h), Enabled: true}, nil

	case "weekly":
		// Accepts "weekly DOW HH:MM" or "weekly DOW,DOW,... HH:MM".
		if len(parts) != 3 {
			return Cadence{}, fmt.Errorf("schedule: 'weekly' requires DOW (or DOW,DOW,...) and HH:MM arguments")
		}
		dowSpec, dowErr := parseDOWSet(parts[1])
		if dowErr != nil {
			return Cadence{}, fmt.Errorf("schedule: invalid day-of-week %q: %w", parts[1], dowErr)
		}
		h, m, parseErr := parseHHMM(parts[2])
		if parseErr != nil {
			return Cadence{}, fmt.Errorf("schedule: invalid time %q: %w", parts[2], parseErr)
		}
		return Cadence{Spec: fmt.Sprintf("%d %d * * %s", m, h, dowSpec), Enabled: true}, nil

	case "everyN":
		// "everyN <N> HH:MM" — every N days at HH:MM.
		if len(parts) != 3 {
			return Cadence{}, fmt.Errorf("schedule: 'everyN' requires an integer N and HH:MM arguments")
		}
		n, parseErr := strconv.Atoi(parts[1])
		if parseErr != nil || n < 1 {
			return Cadence{}, fmt.Errorf("schedule: 'everyN' N must be a positive integer, got %q", parts[1])
		}
		h, m, parseErr := parseHHMM(parts[2])
		if parseErr != nil {
			return Cadence{}, fmt.Errorf("schedule: invalid time %q: %w", parts[2], parseErr)
		}
		return Cadence{
			Spec:         fmt.Sprintf("%d %d * * *", m, h),
			Enabled:      true,
			IntervalDays: n,
		}, nil

	default:
		// Accept a raw 5-field cron expression.
		if len(parts) != 5 {
			return Cadence{}, fmt.Errorf("schedule: unrecognised cadence %q (expected 'off', 'daily HH:MM', 'weekly DOW[,DOW,...] HH:MM', 'everyN N HH:MM', or a 5-field cron)", s)
		}
		// Validate it parses correctly.
		parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
		if _, parseErr := parser.Parse(s); parseErr != nil {
			return Cadence{}, fmt.Errorf("schedule: invalid cron expression %q: %w", s, parseErr)
		}
		return Cadence{Spec: s, Enabled: true}, nil
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

// parseDOW parses a single day-of-week string (case-insensitive) and returns
// its cron number.
func parseDOW(s string) (int, error) {
	// Normalize to title-case so "mon", "MON", "Mon" all work.
	var normalized string
	if len(s) > 0 {
		normalized = strings.ToUpper(s[:1]) + strings.ToLower(s[1:])
	}
	n, ok := dowMap[normalized]
	if !ok {
		return 0, fmt.Errorf("unknown day %q (expected Sun Mon Tue Wed Thu Fri Sat)", s)
	}
	return n, nil
}

// parseDOWSet parses a comma-separated list of day-of-week strings and returns
// a cron-compatible DOW field string (e.g. "1,3,5" for Mon,Wed,Fri).
// A single day is returned as just its number string (e.g. "1") for
// backwards-compatibility with existing single-DOW weekly schedules.
func parseDOWSet(s string) (string, error) {
	tokens := strings.Split(s, ",")
	nums := make([]string, 0, len(tokens))
	seen := make(map[int]bool, len(tokens))
	for _, tok := range tokens {
		tok = strings.TrimSpace(tok)
		if tok == "" {
			return "", fmt.Errorf("empty day token in %q", s)
		}
		n, err := parseDOW(tok)
		if err != nil {
			return "", err
		}
		if seen[n] {
			return "", fmt.Errorf("duplicate day %q in %q", tok, s)
		}
		seen[n] = true
		nums = append(nums, strconv.Itoa(n))
	}
	if len(nums) == 0 {
		return "", fmt.Errorf("no days specified in %q", s)
	}
	return strings.Join(nums, ","), nil
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

// Stop halts the scheduler and blocks until all in-flight jobs finish.
// robfig/cron v3's Stop() returns a context that is cancelled when the last
// running job exits — we wait on it so main.go can shut down gracefully.
func (s *Scheduler) Stop() {
	ctx := s.c.Stop()
	<-ctx.Done()
}

// domainSpec bundles everything needed to register one scheduler domain entry.
type domainSpec struct {
	cadence string
	name    string
	fn      func()
	lastRun LastRunFunc // nil for domains without everyN support
}

// Reload re-reads the schedule settings and re-registers all domain entries.
// It removes any previously registered entries first, so it is safe to call
// repeatedly (e.g. after a settings change).
//
// For everyN domains the lastRunFn is consulted when the daily trigger fires;
// the job is a no-op when now − lastRun < IntervalDays. Pass nil for lastRunFn
// to disable the due-gate (used when a domain does not yet have a backing store
// query, e.g. VMs / flash in Phase 1).
func (s *Scheduler) Reload(settings store.Settings) error {
	return s.ReloadWithDueChecks(settings, nil, nil, nil)
}

// ReloadWithDueChecks is the full-fidelity Reload that accepts per-domain
// last-run queries so the everyN due-gate is enforced. Pass nil for any domain
// that does not need the gate (it is then equivalent to a plain daily trigger).
func (s *Scheduler) ReloadWithDueChecks(
	settings store.Settings,
	containersLastRun, vmsLastRun, flashLastRun LastRunFunc,
) error {
	// Remove all existing entries.
	for _, id := range s.entryIDs {
		s.c.Remove(id)
	}
	s.entryIDs = s.entryIDs[:0]

	// Register enabled domains.
	domains := []domainSpec{
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
			lastRun: containersLastRun,
		},
		// VMs and Flash cadences are stored but their jobs are no-ops in Phase 1.
		{
			cadence: settings.VMsSchedule,
			name:    "vms",
			fn:      func() { log.Print("schedule: vms job: not yet implemented in Phase 1") },
			lastRun: vmsLastRun,
		},
		{
			cadence: settings.FlashSchedule,
			name:    "flash",
			fn:      func() { log.Print("schedule: flash job: not yet implemented in Phase 1") },
			lastRun: flashLastRun,
		},
	}

	for _, d := range domains {
		cad, err := ParseCadence(d.cadence)
		if err != nil {
			return fmt.Errorf("schedule: domain %s: %w", d.name, err)
		}
		if !cad.Enabled {
			continue
		}

		domainName := d.name
		jobFn := d.fn

		// For everyN cadences wrap the job with the due-check so the daily
		// trigger does nothing when the interval has not elapsed yet.
		if cad.IntervalDays > 0 && d.lastRun != nil {
			innerFn := jobFn
			intervalDays := cad.IntervalDays
			lastRunFn := d.lastRun
			jobFn = func() {
				last, err := lastRunFn()
				if err != nil {
					log.Printf("schedule: %s everyN due-check: last-run query failed: %v", domainName, err)
					return
				}
				minAge := time.Duration(intervalDays) * 24 * time.Hour
				if !last.IsZero() && time.Since(last) < minAge {
					log.Printf("schedule: %s everyN skipped — last run %v ago, interval %d days", domainName, time.Since(last).Round(time.Second), intervalDays)
					return
				}
				innerFn()
			}
		}

		id, err := s.c.AddFunc(cad.Spec, func() {
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
