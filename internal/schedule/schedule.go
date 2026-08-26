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
	"sync"
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

// ---------------------------------------------------------------------------
// Per-item schedule overrides (#121)
// ---------------------------------------------------------------------------

// itemSchedule is how one INCLUDED item participates when per-item schedules are
// ON, derived from its optional per-item override string.
type itemSchedule struct {
	// ownEntry is true when the item has a concrete, valid cadence override and
	// should therefore get its OWN cron entry firing on Spec. When false the item
	// is handled by the domain (inDomainRun) unless it is explicitly off.
	ownEntry bool
	// inDomainRun is true when the item is backed up as part of the domain-cadence
	// run (an empty or invalid override falls back to the domain default exactly as
	// today). Mutually exclusive with ownEntry.
	inDomainRun bool
	// Spec is the 5-field cron expression for the item's own entry (valid only when
	// ownEntry is true).
	Spec string
}

// classifyItemOverride decides how an item participates under per-item schedules
// from its override string. It is the single seam both the per-item entry
// registration and the domain-run filter consult, so their decisions can never
// diverge — and it is a pure function so the due-selection logic is unit-testable
// without cron or a store.
//
//   - ""  (empty)            → follow the domain default (inDomainRun). This is the
//     "no override, unchanged" case: exactly as today.
//   - invalid (ParseCadence  → follow the domain default. A garbage override never
//     errors)                  silently drops an item from all scheduling.
//   - "everyN N HH:MM"        → follow the domain default. Per-item entries have no
//     per-item last-run gate, so an everyN override cannot enforce its interval;
//     it degrades to the domain schedule rather than firing every day. The API
//     rejects everyN overrides at save time, so this only guards a legacy value.
//   - "off"                   → the item is NOT scheduled at all (no entry, and
//     excluded from the domain run). A deliberate per-item pause.
//   - any concrete cadence    → the item gets its OWN entry on that cadence.
func classifyItemOverride(override string) itemSchedule {
	if strings.TrimSpace(override) == "" {
		return itemSchedule{inDomainRun: true}
	}
	cad, err := ParseCadence(override)
	if err != nil {
		return itemSchedule{inDomainRun: true} // invalid → domain default
	}
	if cad.IntervalDays > 0 {
		return itemSchedule{inDomainRun: true} // everyN unsupported per-item → domain default
	}
	if !cad.Enabled {
		return itemSchedule{} // "off" → not scheduled at all
	}
	return itemSchedule{ownEntry: true, Spec: cad.Spec}
}

// DomainRunTargets filters container targets to those the DOMAIN-cadence job
// should back up. When perItem is false it returns targets UNCHANGED (byte-for-
// byte as before — overrides are ignored). When true it drops targets that have
// their own per-item entry (a concrete override) or that are explicitly "off",
// leaving the items that follow the domain default. IncludeInSchedule is still
// checked by RunContainersJob, so this only removes overridden/off items.
func DomainRunTargets(targets []store.Target, perItem bool) []store.Target {
	if !perItem {
		return targets
	}
	out := make([]store.Target, 0, len(targets))
	for _, t := range targets {
		if classifyItemOverride(t.ScheduleCadence).inDomainRun {
			out = append(out, t)
		}
	}
	return out
}

// DomainRunVMTargets is the VM counterpart of DomainRunTargets.
func DomainRunVMTargets(vms []store.VMTarget, perItem bool) []store.VMTarget {
	if !perItem {
		return vms
	}
	out := make([]store.VMTarget, 0, len(vms))
	for _, v := range vms {
		if classifyItemOverride(v.ScheduleCadence).inDomainRun {
			out = append(out, v)
		}
	}
	return out
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

// LastFire returns the most recent fire time of this cadence's cron spec at or
// before now — the "Prev" robfig/cron does not provide. It is the basis of the
// anacron-style catch-up: a domain whose last successful backup predates its
// last scheduled fire MISSED that run (the box was off).
//
// robfig only exposes Next(), so the last fire is found by walking Next() from
// a reference in the past: a doubling search window locates SOME fire at or
// before now, then the walk steps forward to the LAST one. Both loops are
// bounded — the window at most doubles once past one period, so the final walk
// crosses at most ~2× period worth of fires (≤ ~61 steps even for a
// every-minute spec at the initial 1-hour window).
//
// The bool is false when the cadence is disabled, unparseable, or has no fire
// within the two-year lookback (a spec that fires less than every two years has
// no meaningful catch-up semantics).
func (c Cadence) LastFire(now time.Time) (time.Time, bool) {
	if !c.Enabled || c.Spec == "" {
		return time.Time{}, false
	}
	sched, err := cron.ParseStandard(c.Spec)
	if err != nil {
		return time.Time{}, false
	}
	const maxLookback = 2 * 366 * 24 * time.Hour
	for window := time.Hour; window <= maxLookback; window *= 2 {
		first := sched.Next(now.Add(-window))
		if first.IsZero() || first.After(now) {
			continue // no fire inside this window yet — widen it
		}
		last := first
		for {
			next := sched.Next(last)
			if next.IsZero() || next.After(now) {
				return last, true
			}
			last = next
		}
	}
	return time.Time{}, false
}

// WatchdogCadence is the fixed daily cadence of the overdue-backup watchdog.
// It is deliberately not user-configurable: the check is cheap and only its
// once-a-day rhythm matters — 09:00 is late enough that any overnight backup
// window has had its chance to complete before the currency verdict is taken.
const WatchdogCadence = "daily 09:00"

// ReceiverCadence is the fixed daily cadence of the receiver watch (dead-mans-
// switch sweep + due integrity checks for received off-site repos). Like the
// watchdog it is not user-configurable at the app level: the per-repo integrity
// check cadence is configured on each received repo, and the daily tick only
// decides which repos are due and evaluates each repo's dead-mans-switch. 09:15 is
// just after the watchdog so the two currency passes do not fire in the same
// minute.
const ReceiverCadence = "daily 09:15"

// FleetCadence is the fixed daily cadence of the fleet peer sweep (polling
// every enabled peer's protection status). Not user-configurable, like the
// watchdog/receiver: a Fleet page reflects the cached result of the last sweep
// plus whatever the manual poll-now button fetched, not a live poll on every
// page load. 09:30 is just after the receiver watch so the three daily
// currency passes do not fire in the same minute.
const FleetCadence = "daily 09:30"

// catchUpGrace is the slack applied when deciding whether a scheduled fire was
// missed: a success within this margin BEFORE the fire still counts as covering
// it (a manual run moments before the trigger, or clock jitter, must not cause
// a duplicate catch-up backup right after boot).
const catchUpGrace = 10 * time.Minute

// missedRun decides whether a domain MISSED its most recent scheduled run:
// the cadence is enabled, the last fire lies more than catchUpGrace after the
// last successful backup, and (for everyN cadences) the interval-days due-gate
// would actually let a run proceed. It returns the computed last fire time for
// logging alongside the verdict.
//
// A domain that has NEVER succeeded (lastSuccess zero) is deliberately not
// treated as missed: the last computed fire may predate the schedule's very
// creation (we do not record when a cadence was configured), and surprising a
// fresh setup with a full backup on every restart until the first scheduled
// success would be worse than waiting for the next regular fire.
func missedRun(cad Cadence, lastSuccess, now time.Time) (lastFire time.Time, missed bool) {
	if !cad.Enabled || lastSuccess.IsZero() {
		return time.Time{}, false
	}
	lastFire, ok := cad.LastFire(now)
	if !ok {
		return time.Time{}, false
	}
	// everyN: the daily trigger fires every day but the due-gate only runs the
	// job once the interval elapsed — mirror it here so a not-yet-due domain is
	// never flagged missed (the invoked job re-checks the gate anyway).
	if cad.IntervalDays > 0 && now.Sub(lastSuccess) < time.Duration(cad.IntervalDays)*24*time.Hour {
		return lastFire, false
	}
	return lastFire, lastSuccess.Add(catchUpGrace).Before(lastFire)
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
	backupVM       BackupFunc                // nil until SetVMJob wires VM backup
	listVMsFn      ListVMTargetsFunc         // nil until SetVMJob wires VM backup
	backupFiles    BackupFunc                // nil until SetFilesJob wires file-set backup
	listFileSetsFn ListFileSetsFunc          // nil until SetFilesJob wires file-set backup
	backupFlash    func() error              // nil until SetFlashJob wires flash backup
	configJob      func() error              // nil until SetConfigJob wires config self-backup
	replicateOffFn func(domain string) error // nil until SetOffsiteJob wires off-site replication
	// replicateAfterBulkFn runs ONE batched off-site replication after a scheduled
	// multi-item backup loop (containers/VMs/files) when the domain replicates on a
	// blank (coupled) schedule — the per-item inline replication is suppressed in
	// that case, so the whole domain is copied once at the end instead of 44× (#95).
	// nil until SetOffsiteAfterBulkJob wires it; then it is a no-op for domains with
	// no off-site repo or a separate off-site schedule (the callee gates that).
	replicateAfterBulkFn func(domain string)
	// pruneAfterBulkFn runs ONE local prune after a scheduled multi-item backup
	// loop (containers/VMs/files): each item's post-backup retention runs forget
	// WITHOUT --prune under the bulk flag, so the expensive space-reclaim happens
	// once per run instead of once per item. Invoked BEFORE replicateAfterBulkFn
	// (retention first = fewer snapshots to copy off-site). nil until
	// SetPruneAfterBulkJob wires it; then it is a no-op for domains without a
	// retention policy (the callee gates that).
	pruneAfterBulkFn func(domain string)
	drillFn          func(domain, source, kind string) error // nil until SetDrillJob wires restore-verification drills
	tamperFn         func(domain string) error               // nil until SetTamperJob wires off-site tamper tests
	digestFn         func() error                            // nil until SetDigestJob wires the weekly digest notification
	watchdogFn       func() error                            // nil until SetWatchdogJob wires the overdue-backup watchdog
	receiverFn       func() error                            // nil until SetReceiverJob wires the receiver watch (dead-mans-switch + integrity checks)
	fleetFn          func() error                            // nil until SetFleetJob wires the fleet peer sweep
	everythingFn     func() error                            // nil until SetEverythingJob wires the "Backup Everything" pass
	// hcRunStart / hcRunFinish aggregate the Healthchecks ping across a scheduled
	// multi-item domain run (containers/VMs): one /start before the first item and
	// one success/fail after the last, instead of once per item (#49). nil until
	// SetHealthchecksAggregator wires them; then per-item pings are suppressed by the
	// injected backup closures (see cmd/bombvault/main.go).
	hcRunStart  func(domain string)
	hcRunFinish func(domain string, attempted, failed int, failures []ItemFailure)
	// jobRuns is the durable "when did this scheduled job last run" record for
	// the three jobs that have no natural last-run signal of their own — the
	// drill pass, the tamper sweep and the digest (#166). nil until
	// SetJobRunStore wires it; an everyN cadence on any of the three then fails
	// the due-gate CLOSED (the job skips and says so) rather than degrading into
	// a daily fire. See jobLastRun.
	jobRuns JobRunStore

	// mu guards entries and catchUps: ReloadWithDueChecks (settings POST
	// goroutine) mutates them while NextRuns (the /api/schedule/next GET handler
	// goroutine) and CatchUpMissed (the startup goroutine) read them
	// concurrently. It guards ONLY the slice access — never held while
	// calling into cron.Cron (AddFunc/Remove/Entry), which has its own
	// internal locking, so the two locks never nest and cannot deadlock.
	mu      sync.Mutex
	entries []scheduledEntry
	// catchUps is the anacron seam: one entry per registered BACKUP domain
	// (containers/vms/flash/config/files) with a last-run query, so
	// CatchUpMissed can compare each domain's last scheduled fire against its
	// last success and re-run what the box slept through. Rebuilt on every
	// ReloadWithDueChecks alongside entries.
	catchUps []catchUpEntry
}

// catchUpEntry pairs a backup domain's parsed cadence and last-run query with
// its registered cron entry, so a missed run can be triggered through the SAME
// wrapped job chain (SkipIfStillRunning + Recover) a real cron fire would use.
type catchUpEntry struct {
	domain  string
	cadence Cadence
	lastRun LastRunFunc
	id      cron.EntryID
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
// (job=drill/tamper, domain="" — each iterates multiple domains per fire), and
// "digest" (job=digest, domain="" — one app-wide summary per fire).
func jobDomainFromName(name string) (job, domain string) {
	switch name {
	case "drills":
		return "drill", ""
	case "tamper":
		return "tamper", ""
	case "digest":
		return "digest", ""
	case "watchdog":
		return "watchdog", "" // one app-wide overdue check per fire
	case "receiver":
		return "receiver", "" // one app-wide received-repo watch per fire
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
	// Take a locked snapshot of the entry list, then call into cron (s.c.Entry)
	// OUTSIDE the lock — s.mu only ever guards the entries slice itself.
	s.mu.Lock()
	entries := make([]scheduledEntry, len(s.entries))
	copy(entries, s.entries)
	s.mu.Unlock()

	out := make([]NextRun, 0, len(entries))
	for _, e := range entries {
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
		// SkipIfStillRunning: if a domain's previous nightly run is still going when
		// its next trigger fires, skip the new one instead of starting a second
		// concurrent run over the same repo (#95 — a run that overran its window used
		// to spawn an overlapping run that re-processed the head and starved the tail
		// further). Each job gets its own guard (the wrapper is applied per entry).
		// Recover then wraps every job so a panic in one backup is logged and
		// contained instead of crashing the whole process (which would silently stop
		// ALL schedules and take the web UI down).
		c:      cron.New(cron.WithChain(cron.SkipIfStillRunning(cron.DefaultLogger), cron.Recover(cron.DefaultLogger))),
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

// SetOffsiteAfterBulkJob wires the batched post-loop off-site replication used by
// scheduled multi-item domains (containers/VMs/files). After the whole backup loop
// finishes, the job calls replicateFn(domain) ONCE — replacing the per-item inline
// replication those runs suppress — so a high-latency off-site backend is opened and
// its index reloaded a single time per domain instead of once per item (#95). The
// callee no-ops when the domain has no off-site repo or uses its own off-site
// schedule. Until this is called the batched pass is skipped. Call before Reload.
func (s *Scheduler) SetOffsiteAfterBulkJob(replicateFn func(domain string)) {
	s.replicateAfterBulkFn = replicateFn
}

// SetPruneAfterBulkJob wires the batched post-loop local prune used by scheduled
// multi-item domains (containers/VMs/files). After the whole backup loop finishes,
// the job calls pruneFn(domain) ONCE — replacing the per-item inline prune those
// runs defer (each item's forget runs without --prune under the bulk flag) — so a
// 44-container night pays one local prune instead of 44. It runs BEFORE the
// batched off-site replication: retention first means fewer snapshots to copy.
// The callee no-ops when the domain has no retention policy configured. Until
// this is called the batched prune is skipped. Call before Reload.
func (s *Scheduler) SetPruneAfterBulkJob(pruneFn func(domain string)) {
	s.pruneAfterBulkFn = pruneFn
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

// SetDigestJob wires the weekly digest notification so the digest schedule
// actually runs. digestFn composes and sends ONE app-wide summary message when
// the digest schedule fires. Until this is called the digest schedule is a
// no-op (logged). Call before Reload.
func (s *Scheduler) SetDigestJob(digestFn func() error) {
	s.digestFn = digestFn
}

// JobRunStore is the durable "when did this scheduled job last run" record for
// the drill, tamper and digest schedules (#166) — the DI seam that keeps this
// package store-free, exactly like LastRunFunc does for the backup domains.
//
// LastScheduleJobRun MUST distinguish its two zero-ish answers: a zero time with
// a NIL error means "this job has never run" (a definite fact), while an error
// means "cannot tell". The due-gate treats those oppositely — never-ran lets the
// first fire through, cannot-tell skips — so an implementation that flattens a
// query failure into a zero time would silently convert an unknown into a run.
type JobRunStore interface {
	LastScheduleJobRun(job string) (time.Time, error)
	RecordScheduleJobRun(job string, at time.Time) error
}

// SetJobRunStore wires the last-run record that lets the drill, tamper and
// digest schedules honour an "every N days" cadence (#166). Until this is
// called those three still run fine on off/daily/weekly/cron cadences, but an
// everyN cadence on them SKIPS every fire (loudly logged) rather than firing
// daily — see jobLastRun. Call before Reload.
func (s *Scheduler) SetJobRunStore(jobRuns JobRunStore) {
	s.jobRuns = jobRuns
}

// jobLastRun is the everyN due-gate query for one self-recording job.
//
// It deliberately NEVER returns nil. A nil LastRunFunc is what makes Reload skip
// wrapping the job in the due-check, which for an everyN cadence means the daily
// trigger fires the real job EVERY day — a nightly DR restore for the drill
// schedule. So an unwired store is reported as an ERROR here instead, which the
// due-gate turns into a skip: the failure mode of forgetting SetJobRunStore is a
// job that does not run and says why, never a job that runs 14x too often.
func (s *Scheduler) jobLastRun(job string) LastRunFunc {
	return func() (time.Time, error) {
		store := s.jobRuns
		if store == nil {
			return time.Time{}, fmt.Errorf("no job-run store wired (SetJobRunStore) for %q", job)
		}
		return store.LastScheduleJobRun(job)
	}
}

// recordJobRun stamps a self-recording job's last-run time. Call it only after
// the pass has actually done its work — what counts as "done" is decided per job
// at the call site, and each of the three call sites documents its own choice.
//
// A failure to record is logged, not propagated: the work already happened, and
// the only consequence is that the next daily trigger sees a stale (or absent)
// timestamp and runs the pass again. That is the safe direction for a
// verification job — repeat the check rather than silently drop it.
func (s *Scheduler) recordJobRun(job string) {
	store := s.jobRuns
	if store == nil {
		return
	}
	if err := store.RecordScheduleJobRun(job, time.Now()); err != nil {
		log.Printf("schedule: %s: recording the last-run time failed (the next trigger will run it again): %v", job, err)
	}
}

// SetWatchdogJob wires the daily overdue-backup watchdog so its fixed schedule
// (WatchdogCadence) actually runs. watchdogFn checks every enabled domain's
// backup currency and notifies once per overdue episode. Until this is called
// the watchdog schedule is a no-op (logged). Call before Reload.
func (s *Scheduler) SetWatchdogJob(watchdogFn func() error) {
	s.watchdogFn = watchdogFn
}

// SetReceiverJob wires the daily receiver watch so its fixed schedule
// (ReceiverCadence) actually runs. receiverFn evaluates every enabled received
// repo's dead-mans-switch and runs each repo's due integrity check, notifying once
// per stale episode / once per integrity-failure transition. Until this is called
// the receiver schedule is a no-op (logged). Call before Reload.
func (s *Scheduler) SetReceiverJob(receiverFn func() error) {
	s.receiverFn = receiverFn
}

// SetFleetJob wires the daily fleet peer sweep so its fixed schedule
// (FleetCadence) actually runs. fleetFn polls every enabled fleet peer's
// protection status and records the result (read-only, no notifications — a
// peer's own instance already alerts on its own overdue backups). Until this
// is called the fleet schedule is a no-op (logged). Call before Reload.
func (s *Scheduler) SetFleetJob(fleetFn func() error) {
	s.fleetFn = fleetFn
}

// SetEverythingJob wires the "Backup Everything" pass so its own schedule
// actually runs. Everything is a singleton from the scheduler's point of view
// (like flash/config): everythingFn already loops over all five domains
// internally (internal/api/everything.go's BackupEverything), so the job
// takes no arguments. Until this is called the everything domain is a no-op
// (logged). Call before Reload.
func (s *Scheduler) SetEverythingJob(everythingFn func() error) {
	s.everythingFn = everythingFn
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
// the job is a no-op when now − lastRun < IntervalDays. A nil lastRunFn leaves a
// plain cron cadence (daily/weekly/cron) completely unaffected, but it makes an
// everyN cadence unenforceable — such a domain is NOT registered at all, since
// firing it daily would be N times too often (see the loop below).
func (s *Scheduler) Reload(settings store.Settings) error {
	return s.ReloadWithDueChecks(settings, nil, nil, nil, nil, nil, nil)
}

// ReloadWithDueChecks is the full-fidelity Reload that accepts per-domain
// last-run queries so the everyN due-gate is enforced. Pass nil for any BACKUP
// domain that does not need the gate — that domain then simply cannot use an
// everyN cadence (see Reload's doc and the registration loop).
//
// The drill, tamper and digest schedules are NOT parameters here: their last-run
// record is a fixed store binding rather than a per-reload input, so it is wired
// once via SetJobRunStore alongside SetDrillJob / SetTamperJob / SetDigestJob.
func (s *Scheduler) ReloadWithDueChecks(
	settings store.Settings,
	containersLastRun, vmsLastRun, flashLastRun, configLastRun, filesLastRun, everythingLastRun LastRunFunc,
) error {
	// Snapshot + clear the existing entries under the lock, then remove them
	// from cron OUTSIDE the lock — never call into cron while holding s.mu.
	// catchUps is rebuilt alongside entries: a stale catch-up entry would point
	// at a removed cron EntryID (CatchUpMissed additionally nil-guards that).
	s.mu.Lock()
	oldEntries := make([]scheduledEntry, len(s.entries))
	copy(oldEntries, s.entries)
	s.entries = s.entries[:0]
	s.catchUps = s.catchUps[:0]
	s.mu.Unlock()

	for _, e := range oldEntries {
		s.c.Remove(e.id)
	}

	// Per-item schedule overrides (#121). When OFF (the default) every item follows
	// its domain schedule and the domain jobs run the FULL included list exactly as
	// before; the override column is ignored. When ON, an item with a concrete
	// override runs on its own per-item entry (registered after the domain loop) and
	// is filtered out of the domain run, so it is never backed up twice.
	perItem := settings.PerItemSchedules

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
				targets = DomainRunTargets(targets, perItem) // drop items on their own per-item cadence (#121)
				s.runAggregatedHC("containers", func() (int, int, []ItemFailure) {
					return RunContainersJob(targets, s.backup)
				})
				// Retention first: ONE local prune for the whole loop (each item's
				// forget ran without --prune under the bulk flag), then the batched
				// off-site copy — fewer snapshots left to replicate.
				if s.pruneAfterBulkFn != nil {
					s.pruneAfterBulkFn("containers")
				}
				// #95: one batched off-site replication after the whole loop (no-op
				// unless containers replicate on a blank/coupled schedule with an
				// off-site repo configured — the per-item inline copy was suppressed).
				if s.replicateAfterBulkFn != nil {
					s.replicateAfterBulkFn("containers")
				}
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
				store.SortVMTargetsForRun(vms)         // #119: explicit VM backup order first, name-order tiebreak
				vms = DomainRunVMTargets(vms, perItem) // drop VMs on their own per-item cadence (#121)
				s.runAggregatedHC("vms", func() (int, int, []ItemFailure) {
					return RunVMsJob(vms, s.backupVM)
				})
				if s.pruneAfterBulkFn != nil {
					s.pruneAfterBulkFn("vms") // one batched local prune first (retention before replication)
				}
				if s.replicateAfterBulkFn != nil {
					s.replicateAfterBulkFn("vms") // #95: one batched off-site copy after the loop
				}
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
				if s.pruneAfterBulkFn != nil {
					s.pruneAfterBulkFn("files") // one batched local prune first (retention before replication)
				}
				if s.replicateAfterBulkFn != nil {
					s.replicateAfterBulkFn("files") // #95: one batched off-site copy after the loop
				}
			},
			lastRun: filesLastRun,
		},
		{
			cadence: settings.EverythingSchedule,
			name:    "everything",
			fn: func() {
				if s.everythingFn == nil {
					log.Print("schedule: everything job skipped — Backup Everything not wired (SetEverythingJob)")
					return
				}
				if err := s.everythingFn(); err != nil {
					log.Printf("schedule: everything job: backup failed: %v", err)
				}
			},
			lastRun: everythingLastRun,
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
				if len(tasks) == 0 {
					// Drills are on but no domain is enabled, so nothing was
					// attempted. Recording a "run" here would let an empty pass
					// suppress the first REAL one for a whole everyN interval
					// after the user enables a domain.
					return
				}
				for _, tk := range tasks {
					if err := s.drillFn(tk.domain, tk.source, tk.kind); err != nil {
						log.Printf("schedule: drills job: %s/%s(%s): %v", tk.domain, tk.source, tk.kind, err)
					}
				}
				// The pass counts as a run once every task has been ATTEMPTED,
				// whatever each one concluded (#166). A drill's cost is paid on
				// attempt — `restic check --read-data-subset` reads back real pack
				// data, a "dr" task restores a whole off-site snapshot into a
				// sandbox — and that cost is identical whether the verdict comes
				// back good or bad. Gating on success instead would re-run the
				// full pass, DR restore included, every single night for as long
				// as one repo stayed broken: maximum expense at exactly the moment
				// the system is already unhealthy. A failed drill is not lost
				// either way — drillFn records an ok=false row per task, which is
				// what the dashboard badge and the notifications read.
				s.recordJobRun(store.ScheduleJobDrills)
			},
			lastRun: s.jobLastRun(store.ScheduleJobDrills),
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
				// Same rule as the drill pass: a sweep counts as a run once every
				// immutable domain has been PROBED, pass or fail (#166). Each probe
				// is a real round-trip to the off-site backend, so gating on
				// success would hammer an unreachable destination nightly while
				// the user's cadence asked for every N days. The verdict itself is
				// preserved per domain by tamperFn (a tamper_tests row), which is
				// what the ransomware scorecard reads. tamperDomains is non-empty
				// by construction — this spec is only registered when at least one
				// domain is flagged immutable — so there is no empty-pass case to
				// exclude here.
				s.recordJobRun(store.ScheduleJobTamper)
			},
			lastRun: s.jobLastRun(store.ScheduleJobTamper),
		})
	}

	// Weekly digest notification: ONE app-wide summary per fire, on its own
	// cadence. Inert unless explicitly enabled (mirrors the drills gate).
	if settings.DigestEnabled {
		domains = append(domains, domainSpec{
			cadence: settings.DigestSchedule,
			name:    "digest",
			fn: func() {
				if s.digestFn == nil {
					log.Print("schedule: digest job skipped — digest not wired (SetDigestJob)")
					return
				}
				if err := s.digestFn(); err != nil {
					// The digest is the one of the three that records ONLY on
					// success (#166). It is a single cheap, idempotent message
					// with no expensive side effects, so retrying on tomorrow's
					// trigger costs almost nothing — whereas recording a failed
					// send would drop that digest entirely and leave the user
					// silent until the next interval. digestFn returns nil when
					// notifications are switched off (it is a deliberate no-op
					// then, not a failure), so a "never" configuration still
					// records and stays gated instead of retrying daily.
					log.Printf("schedule: digest job: %v", err)
					return
				}
				s.recordJobRun(store.ScheduleJobDigest)
			},
			lastRun: s.jobLastRun(store.ScheduleJobDigest),
		})
	}

	// Overdue-backup watchdog: ONE lightweight app-wide currency check per fire
	// on the fixed WatchdogCadence (no per-user cadence — the check is cheap and
	// its exact hour does not matter, only that it runs daily after the usual
	// overnight backup window). Gated on WatchdogEnabled (default on).
	if settings.WatchdogEnabled {
		domains = append(domains, domainSpec{
			cadence: WatchdogCadence,
			name:    "watchdog",
			fn: func() {
				if s.watchdogFn == nil {
					log.Print("schedule: watchdog job skipped — watchdog not wired (SetWatchdogJob)")
					return
				}
				if err := s.watchdogFn(); err != nil {
					log.Printf("schedule: watchdog job: %v", err)
				}
			},
		})
	}

	// Receiver watch: ONE app-wide pass per fire on the fixed ReceiverCadence,
	// evaluating every enabled received repo's dead-mans-switch and running each
	// repo's due integrity check. Gated on ReceiverEnabled (default off), exactly
	// like the domain toggles the receiver dashboard hangs off.
	if settings.ReceiverEnabled {
		domains = append(domains, domainSpec{
			cadence: ReceiverCadence,
			name:    "receiver",
			fn: func() {
				if s.receiverFn == nil {
					log.Print("schedule: receiver job skipped — receiver not wired (SetReceiverJob)")
					return
				}
				if err := s.receiverFn(); err != nil {
					log.Printf("schedule: receiver job: %v", err)
				}
			},
		})
	}

	// Fleet peer sweep: ONE app-wide pass per fire on the fixed FleetCadence,
	// polling every enabled fleet peer's protection status. Gated on
	// FleetEnabled (default off), exactly like ReceiverEnabled.
	if settings.FleetEnabled {
		domains = append(domains, domainSpec{
			cadence: FleetCadence,
			name:    "fleet",
			fn: func() {
				if s.fleetFn == nil {
					log.Print("schedule: fleet job skipped — fleet not wired (SetFleetJob)")
					return
				}
				if err := s.fleetFn(); err != nil {
					log.Printf("schedule: fleet job: %v", err)
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

		// An everyN cadence is a DAILY cron trigger plus a due-gate, so a domain
		// that cannot answer "when did this last run?" cannot enforce the interval
		// at all — the trigger would just fire the real job every single day, N
		// times too often, silently. Refuse to register it rather than run it
		// wrong: an unenforceable everyN is a permanent misconfiguration, and the
		// honest outcome is a schedule that does not run and says so loudly.
		//
		// No supported configuration reaches this: the five backup domains supply
		// a last-run query, the drill/tamper/digest schedules supply one via
		// jobLastRun, and handlers.go still rejects everyN at save time for the
		// off-site schedules — the only remaining specs without one. It catches a
		// legacy cadence stored before that rejection existed, an imported
		// settings file carrying one, and any future domain wired up without its
		// gate.
		if cad.IntervalDays > 0 && d.lastRun == nil {
			log.Printf("schedule: %s NOT registered — an 'everyN' cadence needs a last-run query to enforce its interval and this schedule has none; it would fire daily. Use daily/weekly/cron instead.", d.name)
			continue
		}

		// For everyN cadences wrap the job with the due-check so the daily
		// trigger does nothing when the interval has not elapsed yet.
		//
		// The gate has three outcomes, and the difference between the last two is
		// the whole safety property:
		//
		//   - query FAILED       → skip. "Cannot tell" is never treated as due; a
		//                          broken database must not authorise a run.
		//   - last run recently  → skip, the interval has not elapsed.
		//   - zero time, no error → RUN. This is not an unknown, it is a definite
		//                          "has never run": a fresh install, or a schedule
		//                          just switched on. Deferring the first pass by a
		//                          whole interval would mean a user who enables
		//                          drills every 14 days gets no verification at all
		//                          for 14 days while the UI says drills are on —
		//                          and if the record never appeared (a wiped table,
		//                          a bug) the job would skip FOREVER, silently. The
		//                          five backup domains already read a
		//                          never-backed-up domain as due for exactly this
		//                          reason, so all eight schedules now agree: the
		//                          first fire after enabling runs, then the
		//                          interval applies.
		if cad.IntervalDays > 0 {
			innerFn := jobFn
			intervalDays := cad.IntervalDays
			lastRunFn := d.lastRun
			jobFn = func() {
				last, err := lastRunFn()
				if err != nil {
					log.Printf("schedule: %s everyN due-check: last-run query failed, skipping this fire: %v", domainName, err)
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
		s.mu.Lock()
		s.entries = append(s.entries, scheduledEntry{id: id, job: job, domain: domain})
		// Backup domains with a last-run query join the anacron catch-up set —
		// only they can tell whether the last scheduled fire was actually covered
		// by a success. job=="backup" is exactly the five per-domain backup specs
		// (offsite/drills/tamper/digest/watchdog map to their own job labels).
		if job == "backup" && d.lastRun != nil {
			s.catchUps = append(s.catchUps, catchUpEntry{domain: d.name, cadence: cad, lastRun: d.lastRun, id: id})
		}
		s.mu.Unlock()
	}

	// #121: register one cron entry per INCLUDED container/VM that carries a concrete
	// per-item cadence override (only when the feature is on). Each fires on its OWN
	// cadence and backs up just that item through the SAME batched machinery a domain
	// run uses (aggregated Healthchecks + after-bulk prune/off-site), so off-site
	// replication and retention still happen. Items without an override stay in the
	// domain run above. A no-op when the feature is off.
	if perItem {
		if err := s.registerPerItemEntries(); err != nil {
			return err
		}
	}

	return nil
}

// registerPerItemEntries adds a dedicated cron entry for every INCLUDED item whose
// per-item override is a concrete cadence (#121). It is called only when the
// feature is on and after the domain entries are registered. The item list is read
// once here (not re-listed at fire time like the domain jobs), so a settings save —
// which reloads the scheduler — is what picks up a newly added or newly overridden
// item. A per-item entry re-checks the item still exists and is still included at
// fire time, so a container removed or excluded between reloads is skipped cleanly.
func (s *Scheduler) registerPerItemEntries() error {
	if s.listFn != nil {
		targets, err := s.listFn()
		if err != nil {
			log.Printf("schedule: per-item containers: list targets: %v", err)
		} else {
			for _, t := range targets {
				if !t.IncludeInSchedule {
					continue
				}
				sched := classifyItemOverride(t.ScheduleCadence)
				if !sched.ownEntry {
					continue
				}
				name := t.ContainerName
				if err := s.addPerItemEntry(sched.Spec, "containers", func() {
					s.runContainerItem(name)
				}); err != nil {
					return fmt.Errorf("schedule: per-item container %q: %w", name, err)
				}
			}
		}
	}
	if s.backupVM != nil && s.listVMsFn != nil {
		vms, err := s.listVMsFn()
		if err != nil {
			log.Printf("schedule: per-item vms: list VM targets: %v", err)
		} else {
			for _, v := range vms {
				if !v.IncludeInSchedule {
					continue
				}
				sched := classifyItemOverride(v.ScheduleCadence)
				if !sched.ownEntry {
					continue
				}
				name := v.Name
				if err := s.addPerItemEntry(sched.Spec, "vms", func() {
					s.runVMItem(name)
				}); err != nil {
					return fmt.Errorf("schedule: per-item vm %q: %w", name, err)
				}
			}
		}
	}
	return nil
}

// addPerItemEntry registers one per-item cron entry (#121) and records it under the
// domain's "backup" label so it surfaces in NextRuns like any other backup fire.
// It is deliberately NOT added to the anacron catch-up set: per-item entries have
// no per-item last-run query, so a missed override run is simply picked up on the
// next fire (the domain catch-up still covers the domain-default items).
func (s *Scheduler) addPerItemEntry(spec, domain string, jobFn func()) error {
	id, err := s.c.AddFunc(spec, func() {
		log.Printf("schedule: running per-item %s job", domain)
		jobFn()
	})
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.entries = append(s.entries, scheduledEntry{id: id, job: "backup", domain: domain})
	s.mu.Unlock()
	return nil
}

// runContainerItem backs up a single container on its per-item cadence (#121)
// through the same batched machinery a scheduled containers run uses. It re-lists
// at fire time so a container removed or excluded since the last reload is skipped.
func (s *Scheduler) runContainerItem(name string) {
	targets, err := s.listFn()
	if err != nil {
		log.Printf("schedule: per-item containers job: list targets: %v", err)
		return
	}
	var one *store.Target
	for i := range targets {
		if targets[i].ContainerName == name {
			one = &targets[i]
			break
		}
	}
	if one == nil || !one.IncludeInSchedule {
		return // removed or excluded since the last reload
	}
	s.runAggregatedHC("containers", func() (int, int, []ItemFailure) {
		return RunContainersJob([]store.Target{*one}, s.backup)
	})
	if s.pruneAfterBulkFn != nil {
		s.pruneAfterBulkFn("containers")
	}
	if s.replicateAfterBulkFn != nil {
		s.replicateAfterBulkFn("containers")
	}
}

// runVMItem is the VM counterpart of runContainerItem (#121).
func (s *Scheduler) runVMItem(name string) {
	vms, err := s.listVMsFn()
	if err != nil {
		log.Printf("schedule: per-item vms job: list VM targets: %v", err)
		return
	}
	var one *store.VMTarget
	for i := range vms {
		if vms[i].Name == name {
			one = &vms[i]
			break
		}
	}
	if one == nil || !one.IncludeInSchedule {
		return // removed or excluded since the last reload
	}
	s.runAggregatedHC("vms", func() (int, int, []ItemFailure) {
		return RunVMsJob([]store.VMTarget{*one}, s.backupVM)
	})
	if s.pruneAfterBulkFn != nil {
		s.pruneAfterBulkFn("vms")
	}
	if s.replicateAfterBulkFn != nil {
		s.replicateAfterBulkFn("vms")
	}
}

// CatchUpMissed runs, once, every enabled backup domain that MISSED its most
// recent scheduled fire while the app was down (anacron-style): a home server
// that is off overnight simply never sees its "daily 03:00" trigger, so the
// backup silently ages until the RPO indicator goes red. Call it once shortly
// after startup (main.go delays it a couple of minutes so the array/Docker are
// up); it compares each domain's last scheduled fire (Cadence.LastFire) with
// its last successful backup and, when the fire was missed (missedRun), invokes
// the SAME wrapped cron job a real fire would run — including its
// SkipIfStillRunning guard, so a concurrent scheduled run is never doubled, and
// the run records/notifies exactly like a normal scheduled run (no new run
// kinds). Domains run sequentially on the caller's goroutine: at boot the box
// is busy enough without five concurrent repo jobs.
//
// It returns the domains it caught up (for tests/logging). Deliberately NOT
// re-run after a settings reload: editing a schedule must not surprise the user
// with an immediate backup.
func (s *Scheduler) CatchUpMissed(now time.Time) []string {
	s.mu.Lock()
	pending := make([]catchUpEntry, len(s.catchUps))
	copy(pending, s.catchUps)
	s.mu.Unlock()

	var ran []string
	for _, e := range pending {
		last, err := e.lastRun()
		if err != nil {
			log.Printf("schedule: catch-up %s: last-run query failed: %v", e.domain, err)
			continue
		}
		lastFire, missed := missedRun(e.cadence, last, now)
		if !missed {
			continue
		}
		entry := s.c.Entry(e.id)
		if entry.WrappedJob == nil {
			continue // entry vanished under a concurrent Reload — its fresh set will judge again
		}
		log.Printf("schedule: catching up missed %s backup (last fire %s, last success %s)",
			e.domain, lastFire.Format(time.RFC3339), last.Format(time.RFC3339))
		entry.WrappedJob.Run()
		ran = append(ran, e.domain)
	}
	return ran
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
// drill for containers, VMs, flash and files when their off-site repo is
// configured (a file-set snapshot is as cheap to sandbox-restore as a flash one).
// config is intentionally excluded from DR drills — a sandbox restore of
// BombVault's own settings DB is meaningless (its real recovery path is the
// in-place staged restart); it still gets the local subset integrity check like
// every other domain.
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
		if settings.VMsEnabled && settings.VMsOffsite != "" {
			out = append(out, drillTask{domain: "vms", source: "offsite", kind: "dr"})
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
