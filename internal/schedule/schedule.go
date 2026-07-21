// Package schedule provides a per-domain in-process scheduler backed by
// github.com/robfig/cron/v3. Each domain (containers / VMs / flash) has its
// own cadence parsed from the settings row.
package schedule

import (
	"fmt"
	"log"
	"sort"
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

// ListVMTargetsFunc returns the current list of VM targets.
type ListVMTargetsFunc func() ([]store.VMTarget, error)

// ListFileSetsFunc returns the current list of file sets.
type ListFileSetsFunc func() ([]store.FileSet, error)

// LastRunFunc returns the time of the last successful backup for a domain, or
// a zero time when there has been none. It is injected so the schedule package
// stays store-free (DI seam).
type LastRunFunc func() (time.Time, error)

// ItemFailure names one scheduled item (container / VM / file set) that failed
// during a per-domain run, with a short reason (the backupFn error's message).
// A scheduled run continues past a failing item, so the aggregated outcome
// carries these so the scheduled-summary notification can enumerate WHICH items
// failed and WHY instead of only a count — the core of #64, where a domain-wide
// fault made 35 of 45 containers fail invisibly.
type ItemFailure struct {
	Name   string
	Reason string
}

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

// PeriodSeconds returns the expected interval between fires for this cadence, in
// seconds — the RPO (recovery-point objective) window a backup is expected to
// stay within. It is the basis of the per-domain protection status: a backup
// older than the period is overdue.
//
//   - off / disabled (Enabled=false)   → 0 (no RPO expectation)
//   - everyN (IntervalDays>0)           → IntervalDays * 86400
//   - daily / weekly / raw cron (Spec)  → the gap between the next two fires of
//     the parsed cron schedule (covers "daily" = 86400 and "weekly" = 604800
//     too, so there is one code path and no special-casing)
//
// A Spec that fails to parse (should never happen for a Cadence built by
// ParseCadence, which validates) yields 0.
func (c Cadence) PeriodSeconds() int64 {
	if !c.Enabled {
		return 0
	}
	if c.IntervalDays > 0 {
		return int64(c.IntervalDays) * 86400
	}
	if c.Spec == "" {
		return 0
	}
	sched, err := cron.ParseStandard(c.Spec)
	if err != nil {
		return 0
	}
	// Take two consecutive fires from a fixed reference and use their gap. A fixed
	// base keeps the result deterministic regardless of when this is called.
	base := time.Date(2000, time.January, 1, 0, 0, 0, 0, time.UTC)
	first := sched.Next(base)
	second := sched.Next(first)
	d := second.Sub(first)
	if d <= 0 {
		return 0
	}
	return int64(d.Seconds())
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
	c              *cron.Cron
	backup         BackupFunc
	listFn         ListTargetsFunc
	backupVM       BackupFunc                              // nil until SetVMJob wires VM backup
	listVMsFn      ListVMTargetsFunc                       // nil until SetVMJob wires VM backup
	backupFiles    BackupFunc                              // nil until SetFilesJob wires file-set backup
	listFileSetsFn ListFileSetsFunc                        // nil until SetFilesJob wires file-set backup
	backupFlash    func() error                            // nil until SetFlashJob wires flash backup
	configJob      func() error                            // nil until SetConfigJob wires config self-backup
	replicateOffFn func(domain string) error               // nil until SetOffsiteJob wires off-site replication
	drillFn        func(domain, source, kind string) error // nil until SetDrillJob wires restore-verification drills
	tamperFn       func(domain string) error               // nil until SetTamperJob wires off-site tamper tests
	// hcRunStart / hcRunFinish aggregate the Healthchecks ping across a scheduled
	// multi-item domain run (containers/VMs): one /start before the first item and
	// one success/fail after the last, instead of once per item (#49). nil until
	// SetHealthchecksAggregator wires them; then per-item pings are suppressed by the
	// injected backup closures (see cmd/bombvault/main.go).
	hcRunStart  func(domain string)
	hcRunFinish func(domain string, attempted, failed int, failures []ItemFailure)
	entries     []scheduledEntry
}

// scheduledEntry pairs a registered cron.EntryID with the job+domain label
// derived from the domainSpec that registered it, so NextRuns() can report
// WHAT each upcoming fire time belongs to (not just when).
type scheduledEntry struct {
	id     cron.EntryID
	job    string
	domain string
}

// NextRun is one upcoming scheduled fire time for the dashboard activity log's
// "what's next" line. Domain is "" for schedules that are not domain-specific
// (drills and tamper tests each iterate their own set of domains internally).
type NextRun struct {
	Job    string    `json:"job"`
	Domain string    `json:"domain"`
	Next   time.Time `json:"next"`
}

// jobDomainFromName derives the (job, domain) label from a domainSpec.name, so
// the label logic lives in one place next to the names it interprets. Names in
// use: "containers"|"vms"|"flash"|"config"|"files" (job=backup, domain=name),
// "<domain>-offsite" (job=offsite, domain=<domain>), "drills" and "tamper"
// (job=drill/tamper, domain="" — each iterates multiple domains per fire).
func jobDomainFromName(name string) (job, domain string) {
	switch name {
	case "drills":
		return "drill", ""
	case "tamper":
		return "tamper", ""
	}
	if d, ok := strings.CutSuffix(name, "-offsite"); ok {
		return "offsite", d
	}
	return "backup", name
}

// NextRuns returns the next fire time for every currently registered schedule
// entry that has one (a registered-but-not-yet-computed entry — the cron
// runner has not been started — has a zero Next and is omitted), sorted
// soonest-first. It is the data source for the dashboard activity log's "up
// next" line.
func (s *Scheduler) NextRuns() []NextRun {
	if s.c == nil {
		return nil
	}
	out := make([]NextRun, 0, len(s.entries))
	for _, e := range s.entries {
		next := s.c.Entry(e.id).Next
		if next.IsZero() {
			continue
		}
		out = append(out, NextRun{Job: e.job, Domain: e.domain, Next: next})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Next.Before(out[j].Next) })
	return out
}

// New creates a Scheduler. backupFn is called for each due container;
// listFn retrieves the current target list when the job fires.
func New(backupFn BackupFunc, listFn ListTargetsFunc) *Scheduler {
	return &Scheduler{
		// Recover wraps every job so a panic in one backup is logged and contained
		// instead of crashing the whole process (which would silently stop ALL
		// schedules and take the web UI down).
		c:      cron.New(cron.WithChain(cron.Recover(cron.DefaultLogger))),
		backup: backupFn,
		listFn: listFn,
	}
}

// SetVMJob wires the VMs domain so scheduled VM backups actually run. backupVMFn
// is called for each due VM; listVMsFn retrieves the current VM target list when
// the job fires. Until this is called the VMs domain is a no-op (logged), so the
// containers-only callers and tests keep working unchanged. Call before Reload.
func (s *Scheduler) SetVMJob(backupVMFn BackupFunc, listVMsFn ListVMTargetsFunc) {
	s.backupVM = backupVMFn
	s.listVMsFn = listVMsFn
}

// SetFilesJob wires the files domain so scheduled file-set backups actually run.
// backupFilesFn is called with each due file set's stable ID (not its name — the
// ID survives renames, keeping run attribution intact); listFn retrieves the
// current file-set list when the job fires. Until this is called the files
// domain is a no-op (logged). Call before Reload.
func (s *Scheduler) SetFilesJob(backupFilesFn BackupFunc, listFn ListFileSetsFunc) {
	s.backupFiles = backupFilesFn
	s.listFileSetsFn = listFn
}

// SetFlashJob wires the flash domain so a scheduled flash backup actually runs.
// Flash is a singleton (the Unraid USB), so the job takes no arguments. Until
// this is called the flash domain is a no-op (logged). Call before Reload.
func (s *Scheduler) SetFlashJob(backupFlashFn func() error) {
	s.backupFlash = backupFlashFn
}

// SetConfigJob wires the config domain so a scheduled self-backup of BombVault's
// own settings actually runs. Config is a singleton (BombVault's own state), so
// the job takes no arguments. Until this is called the config domain is a no-op
// (logged). Call before Reload.
func (s *Scheduler) SetConfigJob(backupConfigFn func() error) {
	s.configJob = backupConfigFn
}

// SetOffsiteJob wires off-site replication so the per-domain off-site schedules
// actually run. replicateFn is called with the domain ("containers"|"vms"|"flash")
// when an off-site schedule fires. Until this is called the off-site schedules are
// a no-op (logged). Call before Reload.
func (s *Scheduler) SetOffsiteJob(replicateFn func(domain string) error) {
	s.replicateOffFn = replicateFn
}

// SetDrillJob wires scheduled restore-verification drills so the single drill
// schedule actually runs. drillFn is called with (domain, source, kind) for each
// scheduled drill task when the drill schedule fires — a local "subset" integrity
// check per enabled domain, plus a real off-site "dr" drill for containers, flash
// and files when off-site is configured (see drillTasks). Until this is called the
// drill schedule is a no-op (logged). Call before Reload.
func (s *Scheduler) SetDrillJob(drillFn func(domain, source, kind string) error) {
	s.drillFn = drillFn
}

// SetTamperJob wires scheduled off-site tamper tests so the single tamper schedule
// actually runs. tamperFn is called with each domain whose off-site repo is flagged
// immutable when the tamper schedule fires. Until this is called the tamper
// schedule is a no-op (logged). Call before Reload.
func (s *Scheduler) SetTamperJob(tamperFn func(domain string) error) {
	s.tamperFn = tamperFn
}

// SetHealthchecksAggregator wires per-domain Healthchecks aggregation for SCHEDULED
// multi-item runs (containers + VMs). A scheduled run then pings the domain's check
// /start ONCE via startFn before the first item and success/fail ONCE via finishFn
// after the last — instead of once per item — so the check reflects the whole domain
// job rather than each container/VM (#49). finishFn receives the run's attempted and
// failed counts (success when failed == 0) plus the per-item failures so the summary
// notification can name which items failed and why (#64). The per-item Healthchecks ping is
// suppressed separately: the backup closures injected into New/SetVMJob run each item
// with a suppress-flagged context (see cmd/bombvault/main.go). Passing nil funcs
// leaves scheduled runs un-aggregated (each item pings as before). Call before Reload.
func (s *Scheduler) SetHealthchecksAggregator(
	startFn func(domain string),
	finishFn func(domain string, attempted, failed int, failures []ItemFailure),
) {
	s.hcRunStart = startFn
	s.hcRunFinish = finishFn
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
	return s.ReloadWithDueChecks(settings, nil, nil, nil, nil, nil)
}

// ReloadWithDueChecks is the full-fidelity Reload that accepts per-domain
// last-run queries so the everyN due-gate is enforced. Pass nil for any domain
// that does not need the gate (it is then equivalent to a plain daily trigger).
func (s *Scheduler) ReloadWithDueChecks(
	settings store.Settings,
	containersLastRun, vmsLastRun, flashLastRun, configLastRun, filesLastRun LastRunFunc,
) error {
	// Remove all existing entries.
	for _, e := range s.entries {
		s.c.Remove(e.id)
	}
	s.entries = s.entries[:0]

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
				s.runAggregatedHC("containers", func() (int, int, []ItemFailure) {
					return RunContainersJob(targets, s.backup)
				})
			},
			lastRun: containersLastRun,
		},
		{
			cadence: settings.VMsSchedule,
			name:    "vms",
			fn: func() {
				if s.backupVM == nil || s.listVMsFn == nil {
					log.Print("schedule: vms job skipped — VM backup not wired (SetVMJob)")
					return
				}
				vms, err := s.listVMsFn()
				if err != nil {
					log.Printf("schedule: vms job: list VM targets: %v", err)
					return
				}
				s.runAggregatedHC("vms", func() (int, int, []ItemFailure) {
					return RunVMsJob(vms, s.backupVM)
				})
			},
			lastRun: vmsLastRun,
		},
		{
			cadence: settings.FlashSchedule,
			name:    "flash",
			fn: func() {
				if s.backupFlash == nil {
					log.Print("schedule: flash job skipped — flash backup not wired (SetFlashJob)")
					return
				}
				if err := s.backupFlash(); err != nil {
					log.Printf("schedule: flash job: backup failed: %v", err)
				}
			},
			lastRun: flashLastRun,
		},
		{
			cadence: settings.ConfigSchedule,
			name:    "config",
			fn: func() {
				if s.configJob == nil {
					log.Print("schedule: config job skipped — config backup not wired (SetConfigJob)")
					return
				}
				if err := s.configJob(); err != nil {
					log.Printf("schedule: config job: backup failed: %v", err)
				}
			},
			lastRun: configLastRun,
		},
		{
			cadence: settings.FilesSchedule,
			name:    "files",
			fn: func() {
				if s.backupFiles == nil || s.listFileSetsFn == nil {
					log.Print("schedule: files job skipped — file-set backup not wired (SetFilesJob)")
					return
				}
				sets, err := s.listFileSetsFn()
				if err != nil {
					log.Printf("schedule: files job: list file sets: %v", err)
					return
				}
				s.runAggregatedHC("files", func() (int, int, []ItemFailure) {
					return RunFilesJob(sets, s.backupFiles)
				})
			},
			lastRun: filesLastRun,
		},
	}

	// Off-site replication on its own per-domain schedule (decoupled from the
	// backup schedules above). A blank cadence means "replicate after every local
	// backup" and is handled in the backup path, not here.
	offsite := func(domain, cadence string) domainSpec {
		return domainSpec{
			cadence: cadence,
			name:    domain + "-offsite",
			fn: func() {
				if s.replicateOffFn == nil {
					log.Printf("schedule: %s-offsite job skipped — off-site not wired (SetOffsiteJob)", domain)
					return
				}
				if err := s.replicateOffFn(domain); err != nil {
					log.Printf("schedule: %s-offsite job: %v", domain, err)
				}
			},
		}
	}
	domains = append(domains,
		offsite("containers", settings.ContainersOffsiteSchedule),
		offsite("vms", settings.VMsOffsiteSchedule),
		offsite("flash", settings.FlashOffsiteSchedule),
		offsite("config", settings.ConfigOffsiteSchedule),
		offsite("files", settings.FilesOffsiteSchedule),
	)

	// Restore-verification drills run on a single schedule across a set of
	// (domain, source, kind) tasks: a local "subset" integrity check per enabled
	// domain plus a real off-site "dr" drill for containers, flash and files when
	// off-site is configured (see drillTasks). A drill error just records ok=false (see drillFn);
	// it never aborts the others. The schedule is inert unless explicitly enabled.
	if settings.DrillsEnabled {
		tasks := drillTasks(settings)
		domains = append(domains, domainSpec{
			cadence: settings.DrillsSchedule,
			name:    "drills",
			fn: func() {
				if s.drillFn == nil {
					log.Print("schedule: drills job skipped — drills not wired (SetDrillJob)")
					return
				}
				for _, tk := range tasks {
					if err := s.drillFn(tk.domain, tk.source, tk.kind); err != nil {
						log.Printf("schedule: drills job: %s/%s(%s): %v", tk.domain, tk.source, tk.kind, err)
					}
				}
			},
		})
	}

	// Off-site tamper tests run on their own schedule across every domain whose
	// off-site repo is flagged immutable (append-only). Inert unless at least one
	// domain is flagged AND the schedule is enabled — the far side is what enforces
	// immutability, so there is nothing to verify for a non-immutable repo.
	if tamperDomains := immutableOffsiteDomains(settings); len(tamperDomains) > 0 {
		domains = append(domains, domainSpec{
			cadence: settings.TamperTestSchedule,
			name:    "tamper",
			fn: func() {
				if s.tamperFn == nil {
					log.Print("schedule: tamper job skipped — tamper test not wired (SetTamperJob)")
					return
				}
				for _, dom := range tamperDomains {
					if err := s.tamperFn(dom); err != nil {
						log.Printf("schedule: tamper job: %s: %v", dom, err)
					}
				}
			},
		})
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
		job, domain := jobDomainFromName(d.name)
		s.entries = append(s.entries, scheduledEntry{id: id, job: job, domain: domain})
	}

	return nil
}

// drillTask is one scheduled restore-verification drill: a (domain, source, kind)
// tuple the drills job iterates when it fires.
type drillTask struct {
	domain string
	source string
	kind   string
}

// drillTasks returns the scheduled drill tasks for the current settings: a local
// "subset" integrity check for every enabled domain, plus a real off-site "dr"
// drill for containers, flash and files when their off-site repo is configured
// (a file-set snapshot is as cheap to sandbox-restore as a flash one). VMs and
// config are intentionally excluded from DR drills — VM disk images are too large
// to sandbox-restore, and a sandbox restore of BombVault's own settings DB is
// meaningless (its real recovery path is the in-place staged restart). Both still
// get the local subset integrity check.
func drillTasks(settings store.Settings) []drillTask {
	var out []drillTask
	for _, d := range enabledDrillDomains(settings) {
		out = append(out, drillTask{domain: d, source: "local", kind: "subset"})
	}
	// The scheduled off-site DR drills are gated behind OffsiteDrillsEnabled: they
	// re-download the whole off-site snapshot each run (egress cost on metered
	// clouds), so the user can opt out of them while keeping the free local subset
	// integrity check above and running the off-site DR check manually (#37).
	if settings.OffsiteDrillsEnabled {
		if settings.ContainersEnabled && settings.ContainersOffsite != "" {
			out = append(out, drillTask{domain: "containers", source: "offsite", kind: "dr"})
		}
		if settings.FlashEnabled && settings.FlashOffsite != "" {
			out = append(out, drillTask{domain: "flash", source: "offsite", kind: "dr"})
		}
		if settings.FilesEnabled && settings.FilesOffsite != "" {
			out = append(out, drillTask{domain: "files", source: "offsite", kind: "dr"})
		}
	}
	return out
}

// enabledDrillDomains returns the domains a scheduled restore-verification drill
// should run against: each domain switched on in Settings. A disabled domain has
// no (current) backups worth drilling, so it is skipped.
func enabledDrillDomains(settings store.Settings) []string {
	var out []string
	if settings.ContainersEnabled {
		out = append(out, "containers")
	}
	if settings.VMsEnabled {
		out = append(out, "vms")
	}
	if settings.FlashEnabled {
		out = append(out, "flash")
	}
	if settings.ConfigEnabled {
		out = append(out, "config")
	}
	if settings.FilesEnabled {
		out = append(out, "files")
	}
	return out
}

// immutableOffsiteDomains returns the domains whose off-site repo is flagged
// immutable (append-only) — the domains a scheduled tamper test should verify. A
// domain without the flag has nothing to prove (BombVault never claimed it was
// protected), so it is skipped.
func immutableOffsiteDomains(settings store.Settings) []string {
	var out []string
	if settings.ContainersOffsiteImmutable {
		out = append(out, "containers")
	}
	if settings.VMsOffsiteImmutable {
		out = append(out, "vms")
	}
	if settings.FlashOffsiteImmutable {
		out = append(out, "flash")
	}
	if settings.ConfigOffsiteImmutable {
		out = append(out, "config")
	}
	if settings.FilesOffsiteImmutable {
		out = append(out, "files")
	}
	return out
}

// runAggregatedHC runs a scheduled per-domain item loop bracketed by a single
// Healthchecks /start (before the first item) and success/fail (after the last) ping
// when the aggregator is wired (SetHealthchecksAggregator). run performs the loop and
// returns (attempted, failed, failures). When the aggregator is not wired it just runs
// the loop — no pings — so container-only callers and the schedule package's tests are
// unchanged. The failures list is threaded to hcRunFinish so the summary notification
// can name which items failed and why (#64).
func (s *Scheduler) runAggregatedHC(domain string, run func() (attempted, failed int, failures []ItemFailure)) {
	if s.hcRunStart != nil {
		s.hcRunStart(domain)
	}
	attempted, failed, failures := run()
	if s.hcRunFinish != nil {
		s.hcRunFinish(domain, attempted, failed, failures)
	}
}

// RunContainersJob backs up each target that has IncludeInSchedule=true,
// calling backupFn sequentially. Errors from individual containers are logged
// but do not abort the remaining containers. It returns how many targets were
// attempted (IncludeInSchedule=true), how many of those failed, and the per-item
// failures (name + reason) — so a scheduled run can aggregate the outcome into a
// single Healthchecks ping (see runAggregatedHC) and name the failed containers
// in the summary notification (#64).
//
// This function is exported so tests can invoke the job synchronously without
// waiting for real wall-clock time.
func RunContainersJob(targets []store.Target, backupFn BackupFunc) (attempted, failed int, failures []ItemFailure) {
	for _, t := range targets {
		if !t.IncludeInSchedule {
			continue
		}
		attempted++
		if err := backupFn(t.ContainerName); err != nil {
			failed++
			failures = append(failures, ItemFailure{Name: t.ContainerName, Reason: err.Error()})
			log.Printf("schedule: containers job: backup %q failed: %v", t.ContainerName, err)
		}
	}
	return attempted, failed, failures
}

// RunVMsJob backs up each VM target that has IncludeInSchedule=true, calling
// backupFn sequentially. As with RunContainersJob, an individual VM failure is
// logged but does not abort the remaining VMs, and it returns the attempted/failed
// counts plus the per-item failures for Healthchecks and summary aggregation.
// Exported so tests can invoke the job synchronously without waiting for real
// wall-clock time.
func RunVMsJob(vms []store.VMTarget, backupFn BackupFunc) (attempted, failed int, failures []ItemFailure) {
	for _, v := range vms {
		if !v.IncludeInSchedule {
			continue
		}
		attempted++
		if err := backupFn(v.Name); err != nil {
			failed++
			failures = append(failures, ItemFailure{Name: v.Name, Reason: err.Error()})
			log.Printf("schedule: vms job: backup %q failed: %v", v.Name, err)
		}
	}
	return attempted, failed, failures
}

// RunFilesJob backs up each file set that is Enabled, calling backupFn
// sequentially with the set's stable ID (not its name — run attribution keys on
// file_sets.id, which survives renames). As with RunVMsJob, an individual set
// failure is logged but does not abort the remaining sets, and it returns the
// attempted/failed counts plus the per-item failures for Healthchecks and summary
// aggregation. The failure is named by the set's human Name (not its ID) so the
// summary reads naturally. Exported so tests can invoke the job synchronously
// without waiting for real wall-clock time.
func RunFilesJob(sets []store.FileSet, backupFn BackupFunc) (attempted, failed int, failures []ItemFailure) {
	for _, fs := range sets {
		if !fs.Enabled {
			continue
		}
		attempted++
		if err := backupFn(fs.ID); err != nil {
			failed++
			failures = append(failures, ItemFailure{Name: fs.Name, Reason: err.Error()})
			log.Printf("schedule: files job: backup %q failed: %v", fs.Name, err)
		}
	}
	return attempted, failed, failures
}
