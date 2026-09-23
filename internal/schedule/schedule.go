// Package schedule runs BombVault's scheduled jobs in-process on
// github.com/robfig/cron/v3. Each backup domain has its own cadence, parsed from
// the settings row.
package schedule

import (
	"fmt"
	"log"
	"math"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/robfig/cron/v3"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

// BackupFunc backs up one scheduled item. It is injected so the scheduler can be
// tested without the backup service.
type BackupFunc func(containerName string) error

// ListTargetsFunc returns the current list of targets.
type ListTargetsFunc func() ([]store.Target, error)

// ListVMTargetsFunc returns the current list of VM targets.
type ListVMTargetsFunc func() ([]store.VMTarget, error)

// ListFileSetsFunc returns the current list of file sets.
type ListFileSetsFunc func() ([]store.FileSet, error)

// LastRunFunc returns the time of the last successful backup for a domain, or
// a zero time when there has been none. It keeps this package independent of
// the store.
type LastRunFunc func() (time.Time, error)

// ItemFailure is one scheduled item (container, VM or file set) that failed
// during a run, with the backup error's message as the reason. The summary
// notification lists them.
type ItemFailure struct {
	Name   string
	Reason string
}

// Cadence is a parsed cadence string.
//
//   - Enabled=false: the domain is off (Spec is empty, IntervalDays is 0).
//   - Enabled=true, IntervalDays=0: Spec fires unconditionally.
//   - Enabled=true, IntervalDays>0: Spec is a daily trigger at HH:MM, and the job
//     runs only when the last run is at least IntervalDays calendar days back
//     (see EveryNDue).
type Cadence struct {
	Spec         string // 5-field cron expression; empty when Enabled=false
	Enabled      bool
	IntervalDays int // >0 for everyN cadences only
}

// ParseCadence converts a user-facing cadence string into a Cadence. The
// accepted forms are:
//
//   - "off"
//   - "daily HH:MM"
//   - "weekly DOW[,DOW,...] HH:MM", days Sun to Sat in any case
//   - "everyN N HH:MM", a daily trigger with IntervalDays=N, N >= 1
//   - a raw 5-field cron expression, passed through unchanged
func ParseCadence(s string) (Cadence, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		// A settings PUT may send an empty cadence; it means "off".
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
		if len(parts) != 5 {
			return Cadence{}, fmt.Errorf("schedule: unrecognised cadence %q (expected 'off', 'daily HH:MM', 'weekly DOW[,DOW,...] HH:MM', 'everyN N HH:MM', or a 5-field cron)", s)
		}
		parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
		if _, parseErr := parser.Parse(s); parseErr != nil {
			return Cadence{}, fmt.Errorf("schedule: invalid cron expression %q: %w", s, parseErr)
		}
		return Cadence{Spec: s, Enabled: true}, nil
	}
}

// itemSchedule is how one included item takes part in scheduling when per-item
// schedules are on, derived from its optional override string. With both flags
// false the item is not scheduled at all.
type itemSchedule struct {
	// ownEntry means the item has a valid cadence override and gets its own
	// cron entry firing on Spec.
	ownEntry bool
	// inDomainRun means the domain's run backs the item up. An empty or invalid
	// override falls back to the domain default. Exclusive with ownEntry.
	inDomainRun bool
	// Spec is the cron expression of the item's own entry, set with ownEntry.
	Spec string
}

// classifyItemOverride decides from an override string how an item takes part
// under per-item schedules. Entry registration and the domain-run filter both
// use it, so they cannot disagree about an item.
//
//   - empty: follow the domain default.
//   - invalid: follow the domain default, so a bad override never drops an item
//     from scheduling.
//   - "everyN N HH:MM": follow the domain default. A per-item entry has no
//     last-run gate, so it could not enforce the interval and would fire daily.
//     The API rejects everyN overrides on save; this covers older stored values.
//   - "off": not scheduled at all, neither on its own nor in the domain run.
//   - any other cadence: the item gets its own entry.
func classifyItemOverride(override string) itemSchedule {
	if strings.TrimSpace(override) == "" {
		return itemSchedule{inDomainRun: true}
	}
	cad, err := ParseCadence(override)
	if err != nil {
		return itemSchedule{inDomainRun: true}
	}
	if cad.IntervalDays > 0 {
		return itemSchedule{inDomainRun: true}
	}
	if !cad.Enabled {
		return itemSchedule{}
	}
	return itemSchedule{ownEntry: true, Spec: cad.Spec}
}

// DomainRunTargets filters container targets to those the domain job should
// back up. With perItem false it returns targets unchanged. With perItem true it
// drops targets that have their own per-item entry or are "off".
// RunContainersJob still checks IncludeInSchedule.
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

// DomainRunFileSets is the file-set counterpart of DomainRunTargets. RunFilesJob
// still checks Enabled.
func DomainRunFileSets(sets []store.FileSet, perItem bool) []store.FileSet {
	if !perItem {
		return sets
	}
	out := make([]store.FileSet, 0, len(sets))
	for _, fs := range sets {
		if classifyItemOverride(fs.ScheduleCadence).inDomainRun {
			out = append(out, fs)
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

// DomainRunHasWork reports whether a filtered container list still holds an item
// the domain job would back up. It is exported for the "Backup Everything" pass
// (internal/api/everything.go), which runs the same domain loops.
//
// The domain jobs skip their whole pass when it is false. The loop itself is
// free, but the prune, the off-site copy and the "0 of 0 items succeeded" ping
// after it are not. With per-item schedules on an empty list is common, and its
// everyN gate reads the zero time and reports due at every fire, because an
// empty pass records no run.
func DomainRunHasWork(targets []store.Target) bool {
	for _, t := range targets {
		if t.IncludeInSchedule {
			return true
		}
	}
	return false
}

// DomainRunHasVMWork is the VM counterpart of DomainRunHasWork.
func DomainRunHasVMWork(vms []store.VMTarget) bool {
	for _, v := range vms {
		if v.IncludeInSchedule {
			return true
		}
	}
	return false
}

// DomainRunHasFileWork is the file-set counterpart of DomainRunHasWork. It checks
// Enabled, as RunFilesJob does.
func DomainRunHasFileWork(sets []store.FileSet) bool {
	for _, fs := range sets {
		if fs.Enabled {
			return true
		}
	}
	return false
}

// DomainGateStore is what a multi-item domain's everyN due-gate reads: the
// settings (for the per-item switch), the domain's items, and the last
// successful backup among a given set of them.
type DomainGateStore interface {
	GetSettings() (store.Settings, error)
	ListTargets() ([]store.Target, error)
	ListVMTargets() ([]store.VMTarget, error)
	ListFileSets() ([]store.FileSet, error)
	LastSuccessfulBackupAmong(ids []string) (time.Time, error)
}

// ContainersDueGate returns the containers everyN due-gate query: the last
// successful backup among the items the domain run covers, filtered the way the
// domain job filters them. Counting any container's newest success instead would
// let one container on its own daily cadence keep the gate closed for all the
// others. A store error is returned rather than read as a zero time, because the
// gate skips a fire it cannot judge and a zero time would mean due.
func ContainersDueGate(st DomainGateStore) LastRunFunc {
	return func() (time.Time, error) {
		settings, err := st.GetSettings()
		if err != nil {
			return time.Time{}, fmt.Errorf("containers due-gate: read settings: %w", err)
		}
		targets, err := st.ListTargets()
		if err != nil {
			return time.Time{}, fmt.Errorf("containers due-gate: list targets: %w", err)
		}
		ids := make([]string, 0, len(targets))
		for _, t := range DomainRunTargets(targets, settings.PerItemSchedules) {
			if t.IncludeInSchedule {
				ids = append(ids, t.ID)
			}
		}
		return st.LastSuccessfulBackupAmong(ids)
	}
}

// VMsDueGate is the VM counterpart of ContainersDueGate.
func VMsDueGate(st DomainGateStore) LastRunFunc {
	return func() (time.Time, error) {
		settings, err := st.GetSettings()
		if err != nil {
			return time.Time{}, fmt.Errorf("vms due-gate: read settings: %w", err)
		}
		vms, err := st.ListVMTargets()
		if err != nil {
			return time.Time{}, fmt.Errorf("vms due-gate: list VM targets: %w", err)
		}
		ids := make([]string, 0, len(vms))
		for _, v := range DomainRunVMTargets(vms, settings.PerItemSchedules) {
			if v.IncludeInSchedule {
				ids = append(ids, v.ID)
			}
		}
		return st.LastSuccessfulBackupAmong(ids)
	}
}

// FilesDueGate is the file-set counterpart of ContainersDueGate. It counts only
// enabled sets that follow the domain cadence: the domain job does not back up a
// disabled set or one on its own cadence, so neither may answer for the others.
func FilesDueGate(st DomainGateStore) LastRunFunc {
	return func() (time.Time, error) {
		settings, err := st.GetSettings()
		if err != nil {
			return time.Time{}, fmt.Errorf("files due-gate: read settings: %w", err)
		}
		sets, err := st.ListFileSets()
		if err != nil {
			return time.Time{}, fmt.Errorf("files due-gate: list file sets: %w", err)
		}
		ids := make([]string, 0, len(sets))
		for _, fs := range DomainRunFileSets(sets, settings.PerItemSchedules) {
			if fs.Enabled {
				ids = append(ids, fs.ID)
			}
		}
		return st.LastSuccessfulBackupAmong(ids)
	}
}

// PeriodSeconds returns the expected interval between fires in seconds, the RPO
// window a backup should stay within: 0 when disabled, IntervalDays*86400 for
// everyN, and otherwise the largest gap between consecutive cron fires. It is
// the largest gap because a cadence on several weekdays has unequal gaps;
// "0 3 * * 0,6" fires a day apart and then six days apart. A Spec that fails to
// parse yields 0.
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
	// A fixed base keeps the result deterministic. UTC, because this is a length
	// rather than a wall-clock time, and a DST switch inside the scan would add
	// or drop an hour.
	base := time.Date(2000, time.January, 1, 0, 0, 0, 0, time.UTC)
	first := sched.Next(base)
	if first.IsZero() {
		return 0
	}
	var widest time.Duration
	cur := first
	for i := 0; i < periodScanMaxFires; i++ {
		next := sched.Next(cur)
		if next.IsZero() {
			break
		}
		if gap := next.Sub(cur); gap > widest {
			widest = gap
		}
		cur = next
		if cur.Sub(first) >= periodScanWindow {
			break
		}
	}
	if widest <= 0 {
		return 0
	}
	return int64(widest.Seconds())
}

// periodScanWindow and periodScanMaxFires bound PeriodSeconds' walk over a cron
// spec's fires. A year shows the whole cycle of a monthly or day-of-week cadence,
// February included. The fire cap stops a high-frequency raw cron from walking
// that year minute by minute, and a cadence that dense has repeated its cycle
// many times before it reaches the cap.
const (
	periodScanWindow   = 366 * 24 * time.Hour
	periodScanMaxFires = 2048
)

// LastFire returns the most recent fire of the cadence at or before now, the
// Prev that robfig/cron lacks, for the catch-up after downtime. It walks Next
// forward from a point in the past, doubling the lookback until a fire falls
// inside it. The bool is false when the cadence is disabled, unparseable, or has
// no fire within two years.
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
			continue
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

// WatchdogCadence is the fixed daily cadence of the overdue-backup watchdog. The
// check is cheap and only its daily rhythm matters; 09:00 is late enough for
// overnight backups to have finished.
const WatchdogCadence = "daily 09:00"

// ReceiverCadence is the fixed daily cadence of the receiver watch, which runs
// the dead-mans-switch and the due integrity checks for received off-site repos.
// Each repo sets its own check cadence; the daily tick only decides which are
// due. 09:15 keeps it out of the watchdog's minute.
const ReceiverCadence = "daily 09:15"

// PullCadence is the fixed daily cadence of the pull sweep. The tick decides
// which sources are due; each source carries its own cadence, as received repos
// do for their integrity checks.
const PullCadence = "daily 09:30"

// FleetCadence is the fixed daily cadence of the fleet sweep, which polls every
// enabled peer's protection status. The Fleet page shows the result of the last
// sweep plus any manual poll, not a live poll per page load.
const FleetCadence = "daily 09:30"

// catchUpGrace is how far before a fire a success may lie and still cover it, so
// a manual run moments before the trigger, or clock jitter, does not cause a
// duplicate catch-up backup right after boot.
const catchUpGrace = 10 * time.Minute

// EveryNDue reports whether an "every N days" cadence may run at now, given when
// it last ran. The registration gate and missedRun both use it.
//
// It counts local calendar days rather than N*24h. last is when the previous
// pass finished, later than its fire by the pass's runtime, so as a duration the
// Nth day would always come up short and every everyN schedule would run every
// N+1 days. The count starts from the fire behind last; see
// calendarDaysSinceFire.
//
// A zero last (never ran) is due. So is a last in the future, written while the
// clock was wrong: its negative day count would skip every fire, and the job
// that would write a new stamp is the one being skipped.
func EveryNDue(last, now time.Time, intervalDays int) bool {
	if intervalDays <= 0 || notAMeasurement(last, now) {
		return true
	}
	return calendarDaysSinceFire(last, now) >= intervalDays
}

// notAMeasurement reports whether last is no measurement of a previous pass at
// all: the zero time (never ran) or a stamp from the future (a wrong clock; see
// EveryNDue). Both due-gates read such a stamp as due.
func notAMeasurement(last, now time.Time) bool {
	return last.IsZero() || last.After(now)
}

// calendarDaysSinceFire counts local calendar days from the fire that produced
// last to now. A pass that fires at 23:30 and runs forty minutes stamps 00:10 the
// next day, and counting from the stamp would come out a day short.
//
// now is one of the cadence's daily fires, so the fire behind last is at now's
// clock time on last's day, or on the day before when last's clock time is
// earlier. A pass longer than a day is counted from the last fire it overran,
// which delays the next run rather than doubling it.
func calendarDaysSinceFire(last, now time.Time) int {
	days := calendarDaysBetween(last, now)
	l := last.In(now.Location())
	lastClock := l.Hour()*3600 + l.Minute()*60 + l.Second()
	// A cron fire lands on the minute, so the seconds `now` carries are the delay
	// between the trigger and this gate, not part of the schedule.
	fireClock := now.Hour()*3600 + now.Minute()*60
	if lastClock < fireClock {
		days++ // the pass fired the previous day and ran past midnight
	}
	return days
}

// PeriodDue applies EveryNDue's calendar-day rule to a cadence given as a period
// in seconds (Cadence.PeriodSeconds), for gates evaluated by a daily sweep rather
// than by the cadence's own cron entry. Measured in seconds, a check that took a
// few minutes last time would push a daily cadence to every 48h.
//
// It counts from the stamp rather than from a fire: now is the sweep's fire, and
// the gated cadence never fires on its own, so there is no fire to map the stamp
// onto.
//
// A period under a day is due at every evaluation, since the sweep runs only
// daily; this includes a period <= 0, and callers check their own cadence first.
// Otherwise whole days are compared, rounded down, which errs toward checking
// early.
func PeriodDue(last, now time.Time, periodSeconds int64) bool {
	if periodSeconds < 86400 || notAMeasurement(last, now) {
		return true
	}
	return calendarDaysBetween(last, now) >= int(periodSeconds/86400)
}

// calendarDaysBetween returns the number of local calendar days from `from` to
// `to` (same day = 0), negative when `to` precedes `from`. It subtracts the two
// local midnights and rounds, so a 23h or 25h DST day still counts as exactly
// one day.
func calendarDaysBetween(from, to time.Time) int {
	loc := to.Location()
	f := from.In(loc)
	fromMidnight := time.Date(f.Year(), f.Month(), f.Day(), 0, 0, 0, 0, loc)
	toMidnight := time.Date(to.Year(), to.Month(), to.Day(), 0, 0, 0, 0, loc)
	return int(math.Round(toMidnight.Sub(fromMidnight).Hours() / 24))
}

// missedRun reports whether a domain missed its most recent scheduled run: the
// cadence is enabled, the last fire lies more than catchUpGrace after the last
// success, and for everyN cadences the due-gate would have let that fire
// through. It also returns the last fire for logging.
//
// A domain that never succeeded is not treated as missed. Its last computed fire
// may predate the schedule itself (when a cadence was configured is not
// recorded), and a fresh setup would get a full backup on every restart until
// its first scheduled success.
func missedRun(cad Cadence, lastSuccess, now time.Time) (lastFire time.Time, missed bool) {
	if !cad.Enabled || lastSuccess.IsZero() {
		return time.Time{}, false
	}
	lastFire, ok := cad.LastFire(now)
	if !ok {
		return time.Time{}, false
	}
	// Ask the everyN gate about the fire, not about now. now is whenever this
	// sweep runs (mostly at boot) and is no fire of this cadence; asked at now,
	// a fire still hours away would read as due and the catch-up would run the
	// pass a day early on every boot.
	if !EveryNDue(lastSuccess, lastFire, cad.IntervalDays) {
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

// dowMap maps 3-letter day abbreviations to cron DOW numbers, Sun=0 to Sat=6.
var dowMap = map[string]int{
	"Sun": 0, "Mon": 1, "Tue": 2, "Wed": 3,
	"Thu": 4, "Fri": 5, "Sat": 6,
}

// parseDOW parses a single day-of-week string (case-insensitive) and returns
// its cron number.
func parseDOW(s string) (int, error) {
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

// parseDOWSet parses a comma-separated list of days and returns the cron DOW
// field, e.g. "1,3,5" for Mon,Wed,Fri.
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

// Scheduler manages the cron entries of all scheduled jobs. Each job is wired
// with its Set method before Reload; a job that has not been wired logs and does
// nothing when it fires.
type Scheduler struct {
	c              *cron.Cron
	backup         BackupFunc
	listFn         ListTargetsFunc
	backupVM       BackupFunc
	listVMsFn      ListVMTargetsFunc
	backupFiles    BackupFunc
	listFileSetsFn ListFileSetsFunc
	backupFlash    func() error
	configJob      func() error
	replicateOffFn func(domain string) error
	// replicateAfterBulkFn replicates a domain off-site once after a scheduled
	// multi-item run, for domains that replicate on the backup schedule. Those
	// runs suppress the per-item copies, so the domain is copied once instead of
	// once per item.
	replicateAfterBulkFn func(domain string)
	// pruneAfterBulkFn prunes a domain once after a scheduled multi-item run. In
	// a bulk run each item's retention runs forget without --prune, so space is
	// reclaimed once per run. It runs before replicateAfterBulkFn so there are
	// fewer snapshots to copy.
	pruneAfterBulkFn func(domain string)
	// stacksAfterBulkFn backs up each Docker Compose project directory once after
	// a scheduled container run instead of with every member service.
	stacksAfterBulkFn func(names []string)
	drillFn           func(domain, source, kind string) error
	tamperFn          func(domain string) error
	digestFn          func() error
	watchdogFn        func() error
	receiverFn        func() error
	pullFn            func() error
	fleetFn           func() error
	everythingFn      func() error
	// hcRunStart and hcRunFinish send one Healthchecks start and one result ping
	// per scheduled multi-item run instead of one per item.
	hcRunStart  func(domain string)
	hcRunFinish func(domain string, attempted, failed int, failures []ItemFailure)
	// jobRuns records when the drill, tamper and digest jobs last ran, since
	// they have no last-run signal of their own. Without it an everyN cadence on
	// those jobs skips every fire; see jobLastRun.
	jobRuns JobRunStore

	// mu guards entries and catchUps, which a reload writes while NextRuns and
	// CatchUpMissed read them. It is never held while calling into cron, which
	// has its own lock, so the two cannot deadlock.
	mu sync.Mutex
	// reloadMu serialises whole reloads. mu is released between removing the old
	// entries and adding the new ones, so two interleaved reloads would each
	// register a full set and leave every job in cron twice. It is taken before
	// mu, never while holding it.
	reloadMu sync.Mutex
	entries  []scheduledEntry
	// catchUps holds one entry per registered backup domain with a last-run
	// query, so CatchUpMissed can re-run what the box slept through. It is
	// rebuilt on every reload.
	catchUps []catchUpEntry
}

// catchUpEntry pairs a backup domain's cadence and last-run query with its cron
// entry, so a missed run goes through the same wrapped job (SkipIfStillRunning
// and Recover) a real fire would use.
type catchUpEntry struct {
	domain  string
	cadence Cadence
	lastRun LastRunFunc
	id      cron.EntryID
}

// scheduledEntry is a registered cron entry with the job and domain it belongs
// to, so NextRuns can say what each fire is for. For an everyN cadence it also
// carries the interval and last-run query, because cron fires that entry daily
// and only the gate decides which fire actually runs.
type scheduledEntry struct {
	id           cron.EntryID
	job          string
	domain       string
	intervalDays int         // >0 for everyN cadences only
	lastRun      LastRunFunc // set whenever intervalDays > 0
}

// NextRun is one upcoming scheduled fire for the activity log's "up next" line.
// Domain is empty for jobs that are not domain-specific.
type NextRun struct {
	Job    string    `json:"job"`
	Domain string    `json:"domain"`
	Next   time.Time `json:"next"`
}

// jobDomainFromName derives the job and domain label from a domainSpec name. A
// backup domain maps to ("backup", name), "<domain>-offsite" to ("offsite",
// domain), and the app-wide jobs (drills, tamper, digest, watchdog, receiver,
// pull, fleet) to their own job with an empty domain.
func jobDomainFromName(name string) (job, domain string) {
	switch name {
	case "drills":
		return "drill", ""
	case "tamper":
		return "tamper", ""
	case "digest":
		return "digest", ""
	case "watchdog":
		return "watchdog", ""
	case "receiver":
		return "receiver", ""
	case "pull":
		return "pull", ""
	case "fleet":
		return "fleet", ""
	}
	if d, ok := strings.CutSuffix(name, "-offsite"); ok {
		return "offsite", d
	}
	return "backup", name
}

// NextRuns returns the next time each registered entry will actually run, sorted
// soonest first, for the activity log's "up next" line. Entries without a
// computed Next (the cron runner has not started) are left out.
//
// An everyN entry fires daily and its gate skips most of those fires, so its
// cron Next is walked forward to the first fire EveryNDue would let through. If
// the last-run query fails, the raw cron fire is reported as the earliest the
// job could run.
func (s *Scheduler) NextRuns() []NextRun {
	if s.c == nil {
		return nil
	}
	// Copy the entries under the lock and call into cron outside it.
	s.mu.Lock()
	entries := make([]scheduledEntry, len(s.entries))
	copy(entries, s.entries)
	s.mu.Unlock()

	out := make([]NextRun, 0, len(entries))
	for _, e := range entries {
		entry := s.c.Entry(e.id)
		next := entry.Next
		if next.IsZero() {
			continue
		}
		out = append(out, NextRun{Job: e.job, Domain: e.domain, Next: nextDueFire(entry.Schedule, next, e)})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Next.Before(out[j].Next) })
	return out
}

// nextDueFire advances an everyN entry's next cron fire to the first one its
// due-gate would let through. Other entries, and entries whose last-run query
// fails, get next back unchanged. The trigger is daily and the gate counts whole
// days, so at most intervalDays fires can be skipped.
func nextDueFire(sched cron.Schedule, next time.Time, e scheduledEntry) time.Time {
	if e.intervalDays <= 0 || e.lastRun == nil || sched == nil {
		return next
	}
	last, err := e.lastRun()
	if err != nil {
		return next
	}
	for i := 0; i <= e.intervalDays; i++ {
		if EveryNDue(last, next, e.intervalDays) {
			return next
		}
		later := sched.Next(next)
		if later.IsZero() || !later.After(next) {
			return next
		}
		next = later
	}
	return next
}

// New creates a Scheduler. backupFn is called for each due container; listFn
// returns the current target list when the job fires.
func New(backupFn BackupFunc, listFn ListTargetsFunc) *Scheduler {
	return &Scheduler{
		// SkipIfStillRunning keeps a slow run from overlapping the next fire on
		// the same repo. Recover keeps a panic in one job from taking down the
		// process, and with it every schedule and the web UI.
		//
		// Cron runs in time.Local, so across DST a wall-clock cadence skips a day
		// in spring (02:30 does not exist) and fires twice in autumn. restic
		// dedupes the doubled run and the next day covers the skipped one; a
		// deployment that wants exact 24h intervals can leave TZ unset.
		// WithLocation would change nothing, since time.Local is already the
		// default, and a minimum-gap gate on the path that starts every backup
		// would risk more than the two odd days a year it avoids.
		c:      cron.New(cron.WithChain(cron.SkipIfStillRunning(cron.DefaultLogger), cron.Recover(cron.DefaultLogger))),
		backup: backupFn,
		listFn: listFn,
	}
}

// SetVMJob wires scheduled VM backups: backupVMFn is called for each due VM and
// listVMsFn returns the VM targets when the job fires. Call before Reload.
func (s *Scheduler) SetVMJob(backupVMFn BackupFunc, listVMsFn ListVMTargetsFunc) {
	s.backupVM = backupVMFn
	s.listVMsFn = listVMsFn
}

// SetFilesJob wires scheduled file-set backups: backupFilesFn is called with each
// due set's ID, which survives renames, and listFn returns the file sets when the
// job fires. Call before Reload.
func (s *Scheduler) SetFilesJob(backupFilesFn BackupFunc, listFn ListFileSetsFunc) {
	s.backupFiles = backupFilesFn
	s.listFileSetsFn = listFn
}

// SetFlashJob wires the scheduled flash backup. Flash is a singleton (the Unraid
// USB), so the job takes no arguments. Call before Reload.
func (s *Scheduler) SetFlashJob(backupFlashFn func() error) {
	s.backupFlash = backupFlashFn
}

// SetConfigJob wires the scheduled self-backup of BombVault's own settings.
// Call before Reload.
func (s *Scheduler) SetConfigJob(backupConfigFn func() error) {
	s.configJob = backupConfigFn
}

// SetOffsiteJob wires the per-domain off-site schedules. replicateFn is called
// with the domain when one of them fires. Call before Reload.
func (s *Scheduler) SetOffsiteJob(replicateFn func(domain string) error) {
	s.replicateOffFn = replicateFn
}

// SetOffsiteAfterBulkJob wires the off-site replication that runs once after a
// scheduled multi-item run (containers, VMs, files), in place of the per-item
// copies those runs suppress. A slow off-site backend is then opened and its
// index loaded once per domain instead of once per item. replicateFn does
// nothing for a domain without an off-site repo or with its own off-site
// schedule. Call before Reload.
func (s *Scheduler) SetOffsiteAfterBulkJob(replicateFn func(domain string)) {
	s.replicateAfterBulkFn = replicateFn
}

// SetPruneAfterBulkJob wires the local prune that runs once after a scheduled
// multi-item run, in place of the per-item prunes those runs defer, so a night
// of 44 containers pays for one prune instead of 44. It runs before the off-site
// replication so there are fewer snapshots to copy. pruneFn does nothing for a
// domain without a retention policy. Call before Reload.
func (s *Scheduler) SetPruneAfterBulkJob(pruneFn func(domain string)) {
	s.pruneAfterBulkFn = pruneFn
}

// SetStacksAfterBulkJob wires the once-per-run Compose stack backup. fn receives
// the container names the run attempted and visits each project once, however
// many of its services took part.
func (s *Scheduler) SetStacksAfterBulkJob(fn func(names []string)) {
	s.stacksAfterBulkFn = fn
}

// SetDrillJob wires the scheduled restore-verification drills. drillFn is called
// with (domain, source, kind) for each task from drillTasks. Call before Reload.
func (s *Scheduler) SetDrillJob(drillFn func(domain, source, kind string) error) {
	s.drillFn = drillFn
}

// SetTamperJob wires the scheduled off-site tamper tests. tamperFn is called for
// each domain whose off-site repo is flagged immutable. Call before Reload.
func (s *Scheduler) SetTamperJob(tamperFn func(domain string) error) {
	s.tamperFn = tamperFn
}

// SetDigestJob wires the weekly digest. digestFn sends one app-wide summary per
// fire. Call before Reload.
func (s *Scheduler) SetDigestJob(digestFn func() error) {
	s.digestFn = digestFn
}

// JobRunStore records when the drill, tamper and digest schedules last ran.
//
// LastScheduleJobRun has to keep its two empty answers apart. A zero time with a
// nil error means the job never ran, and the gate lets the first fire through;
// an error means the answer is unknown, and the gate skips. Flattening a query
// error into a zero time would turn an unknown into a run.
type JobRunStore interface {
	LastScheduleJobRun(job string) (time.Time, error)
	RecordScheduleJobRun(job string, at time.Time) error
}

// SetJobRunStore wires the last-run record the drill, tamper and digest schedules
// need for an "every N days" cadence. Without it they still run on other
// cadences, but an everyN cadence skips every fire; see jobLastRun. Call before
// Reload.
func (s *Scheduler) SetJobRunStore(jobRuns JobRunStore) {
	s.jobRuns = jobRuns
}

// jobLastRun is the everyN due-gate query for a job that records its own runs.
// Without a store it returns an error, so the gate skips each fire and logs why
// rather than running the job ungated.
func (s *Scheduler) jobLastRun(job string) LastRunFunc {
	return func() (time.Time, error) {
		store := s.jobRuns
		if store == nil {
			return time.Time{}, fmt.Errorf("no job-run store wired (SetJobRunStore) for %q", job)
		}
		return store.LastScheduleJobRun(job)
	}
}

// recordJobRun stamps a self-recording job's last-run time once its pass has done
// its work. A failed write is only logged; the worst case is that the next
// trigger runs the pass again.
//
// The gate cannot see a pass that is still running, since the stamp is written
// at the end. That is safe while recordJobRun ends a single cron entry, which
// SkipIfStillRunning serialises, and nothing else calls it. A second caller
// needs an in-flight guard first, or overlapping passes would each run a DR
// restore.
func (s *Scheduler) recordJobRun(job string) {
	store := s.jobRuns
	if store == nil {
		return
	}
	if err := store.RecordScheduleJobRun(job, time.Now()); err != nil {
		log.Printf("schedule: %s: recording the last-run time failed (the next trigger will run it again): %v", job, err)
	}
}

// SetWatchdogJob wires the daily overdue-backup watchdog (WatchdogCadence).
// watchdogFn checks every enabled domain and notifies once per overdue episode.
// Call before Reload.
func (s *Scheduler) SetWatchdogJob(watchdogFn func() error) {
	s.watchdogFn = watchdogFn
}

// SetReceiverJob wires the daily receiver watch (ReceiverCadence). receiverFn
// evaluates each enabled received repo's dead-mans-switch and runs its integrity
// check when due. Call before Reload.
func (s *Scheduler) SetReceiverJob(receiverFn func() error) {
	s.receiverFn = receiverFn
}

// SetPullJob wires the daily pull sweep (PullCadence). pullFn fetches every
// enabled pull source whose own cadence is due. Call before Reload.
func (s *Scheduler) SetPullJob(pullFn func() error) {
	s.pullFn = pullFn
}

// SetFleetJob wires the daily fleet sweep (FleetCadence). fleetFn polls every
// enabled peer's protection status and records it without notifying, since each
// peer alerts on its own backups. Call before Reload.
func (s *Scheduler) SetFleetJob(fleetFn func() error) {
	s.fleetFn = fleetFn
}

// SetEverythingJob wires the "Backup Everything" schedule. everythingFn already
// covers all five domains (BackupEverything in internal/api/everything.go), so
// the job takes no arguments. Call before Reload.
func (s *Scheduler) SetEverythingJob(everythingFn func() error) {
	s.everythingFn = everythingFn
}

// SetHealthchecksAggregator makes scheduled multi-item runs (containers, VMs)
// report to Healthchecks once per domain: startFn before the first item, and
// finishFn after the last with the attempted and failed counts (success when
// failed == 0) and the per-item failures for the summary notification. The
// per-item pings are suppressed by the backup closures passed to New and
// SetVMJob (see cmd/bombvault/main.go). Nil funcs leave each item pinging on its
// own. Call before Reload.
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

// Stop halts the scheduler and waits until all running jobs have finished.
func (s *Scheduler) Stop() {
	ctx := s.c.Stop()
	<-ctx.Done()
}

// domainSpec is one entry to register. off is the domain's own on/off switch,
// stated negatively so that the zero value means "registered" for the specs that
// are appended only when already enabled.
type domainSpec struct {
	cadence string
	name    string
	off     bool
	fn      func()
	lastRun LastRunFunc // nil for domains without everyN support
}

// Reload registers all entries from settings, replacing earlier ones, so it can
// be called after every settings change. It passes no last-run queries, so a
// backup domain with an everyN cadence is not registered; ReloadWithDueChecks
// handles those.
func (s *Scheduler) Reload(settings store.Settings) error {
	return s.ReloadWithDueChecks(settings, nil, nil, nil, nil, nil, nil)
}

// ReloadWithDueChecks is Reload with a last-run query per backup domain, which
// the everyN due-gate needs. A nil query means that domain cannot use an everyN
// cadence. The drill, tamper and digest schedules get theirs from
// SetJobRunStore instead.
func (s *Scheduler) ReloadWithDueChecks(
	settings store.Settings,
	containersLastRun, vmsLastRun, flashLastRun, configLastRun, filesLastRun, everythingLastRun LastRunFunc,
) error {
	s.reloadMu.Lock()
	defer s.reloadMu.Unlock()

	// Clear the entries under mu and remove them from cron after releasing it.
	// catchUps goes too, since its entries point at the removed IDs.
	s.mu.Lock()
	oldEntries := make([]scheduledEntry, len(s.entries))
	copy(oldEntries, s.entries)
	s.entries = s.entries[:0]
	s.catchUps = s.catchUps[:0]
	s.mu.Unlock()

	for _, e := range oldEntries {
		s.c.Remove(e.id)
	}

	// With per-item schedules on, an item with its own cadence gets its own entry
	// (registered below) and is left out of the domain run, so it is never backed
	// up twice. With them off, overrides are ignored.
	perItem := settings.PerItemSchedules

	domains := []domainSpec{
		{
			cadence: settings.ContainersSchedule,
			name:    "containers",
			off:     !settings.ContainersEnabled,
			fn: func() {
				targets, err := s.listFn()
				if err != nil {
					log.Printf("schedule: containers job: list targets: %v", err)
					return
				}
				targets = DomainRunTargets(targets, perItem)
				if !DomainRunHasWork(targets) {
					return
				}
				s.runAggregatedHC("containers", func() (int, int, []ItemFailure) {
					return RunContainersJob(targets, s.backup)
				})
				// Compose project directories, once per run and before the prune
				// so they fall into the same retention pass.
				if s.stacksAfterBulkFn != nil {
					names := make([]string, 0, len(targets))
					for _, t := range targets {
						if t.IncludeInSchedule {
							names = append(names, t.ContainerName)
						}
					}
					s.stacksAfterBulkFn(names)
				}
				// Prune before replicating so there are fewer snapshots to copy.
				if s.pruneAfterBulkFn != nil {
					s.pruneAfterBulkFn("containers")
				}
				if s.replicateAfterBulkFn != nil {
					s.replicateAfterBulkFn("containers")
				}
			},
			lastRun: containersLastRun,
		},
		{
			cadence: settings.VMsSchedule,
			name:    "vms",
			off:     !settings.VMsEnabled,
			fn: func() {
				if s.backupVM == nil || s.listVMsFn == nil {
					log.Print("schedule: vms job skipped, VM backup not wired (SetVMJob)")
					return
				}
				vms, err := s.listVMsFn()
				if err != nil {
					log.Printf("schedule: vms job: list VM targets: %v", err)
					return
				}
				store.SortVMTargetsForRun(vms)
				vms = DomainRunVMTargets(vms, perItem)
				if !DomainRunHasVMWork(vms) {
					return
				}
				s.runAggregatedHC("vms", func() (int, int, []ItemFailure) {
					return RunVMsJob(vms, s.backupVM)
				})
				if s.pruneAfterBulkFn != nil {
					s.pruneAfterBulkFn("vms")
				}
				if s.replicateAfterBulkFn != nil {
					s.replicateAfterBulkFn("vms")
				}
			},
			lastRun: vmsLastRun,
		},
		{
			cadence: settings.FlashSchedule,
			name:    "flash",
			off:     !settings.FlashEnabled,
			fn: func() {
				if s.backupFlash == nil {
					log.Print("schedule: flash job skipped, flash backup not wired (SetFlashJob)")
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
			off:     !settings.ConfigEnabled,
			fn: func() {
				if s.configJob == nil {
					log.Print("schedule: config job skipped, config backup not wired (SetConfigJob)")
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
			off:     !settings.FilesEnabled,
			fn: func() {
				if s.backupFiles == nil || s.listFileSetsFn == nil {
					log.Print("schedule: files job skipped, file-set backup not wired (SetFilesJob)")
					return
				}
				sets, err := s.listFileSetsFn()
				if err != nil {
					log.Printf("schedule: files job: list file sets: %v", err)
					return
				}
				sets = DomainRunFileSets(sets, settings.PerItemSchedules)
				if !DomainRunHasFileWork(sets) {
					return
				}
				s.runAggregatedHC("files", func() (int, int, []ItemFailure) {
					return RunFilesJob(sets, s.backupFiles)
				})
				if s.pruneAfterBulkFn != nil {
					s.pruneAfterBulkFn("files")
				}
				if s.replicateAfterBulkFn != nil {
					s.replicateAfterBulkFn("files")
				}
			},
			lastRun: filesLastRun,
		},
		{
			cadence: settings.EverythingSchedule,
			name:    "everything",
			fn: func() {
				if s.everythingFn == nil {
					log.Print("schedule: everything job skipped, Backup Everything not wired (SetEverythingJob)")
					return
				}
				if err := s.everythingFn(); err != nil {
					log.Printf("schedule: everything job: backup failed: %v", err)
				}
			},
			lastRun: everythingLastRun,
		},
	}

	// Off-site replication on its own per-domain schedule. A blank cadence means
	// "replicate after every local backup" and is handled in the backup path. A
	// switched-off domain makes no new snapshots, so its off-site schedule is off
	// as well.
	offsite := func(domain, cadence string, enabled bool) domainSpec {
		return domainSpec{
			cadence: cadence,
			name:    domain + "-offsite",
			off:     !enabled,
			fn: func() {
				if s.replicateOffFn == nil {
					log.Printf("schedule: %s-offsite job skipped, off-site not wired (SetOffsiteJob)", domain)
					return
				}
				if err := s.replicateOffFn(domain); err != nil {
					log.Printf("schedule: %s-offsite job: %v", domain, err)
				}
			},
		}
	}
	domains = append(domains,
		offsite("containers", settings.ContainersOffsiteSchedule, settings.ContainersEnabled),
		offsite("vms", settings.VMsOffsiteSchedule, settings.VMsEnabled),
		offsite("flash", settings.FlashOffsiteSchedule, settings.FlashEnabled),
		offsite("config", settings.ConfigOffsiteSchedule, settings.ConfigEnabled),
		offsite("files", settings.FilesOffsiteSchedule, settings.FilesEnabled),
	)

	if settings.DrillsEnabled {
		tasks := drillTasks(settings)
		domains = append(domains, domainSpec{
			cadence: settings.DrillsSchedule,
			name:    "drills",
			fn: func() {
				if s.drillFn == nil {
					log.Print("schedule: drills job skipped, drills not wired (SetDrillJob)")
					return
				}
				if len(tasks) == 0 {
					// Nothing was attempted. Recording a run here would hold off
					// the first real pass for a whole everyN interval once a
					// domain is switched on.
					return
				}
				for _, tk := range tasks {
					if err := s.drillFn(tk.domain, tk.source, tk.kind); err != nil {
						log.Printf("schedule: drills job: %s/%s(%s): %v", tk.domain, tk.source, tk.kind, err)
					}
				}
				// The pass counts as a run once every task was attempted, whatever
				// the verdicts. A drill costs the same whether it passes or fails
				// (a "dr" task restores a whole off-site snapshot), so gating on
				// success would repeat the full pass every night while one repo
				// stays broken. Failures are not lost: drillFn records an ok=false
				// row per task.
				s.recordJobRun(store.ScheduleJobDrills)
			},
			lastRun: s.jobLastRun(store.ScheduleJobDrills),
		})
	}

	if tamperDomains := immutableOffsiteDomains(settings); len(tamperDomains) > 0 {
		domains = append(domains, domainSpec{
			cadence: settings.TamperTestSchedule,
			name:    "tamper",
			fn: func() {
				if s.tamperFn == nil {
					log.Print("schedule: tamper job skipped, tamper test not wired (SetTamperJob)")
					return
				}
				for _, dom := range tamperDomains {
					if err := s.tamperFn(dom); err != nil {
						log.Printf("schedule: tamper job: %s: %v", dom, err)
					}
				}
				// As with drills, a sweep counts as a run whatever the verdicts,
				// or an unreachable backend would be probed every night. tamperFn
				// records each verdict, and tamperDomains is never empty here.
				s.recordJobRun(store.ScheduleJobTamper)
			},
			lastRun: s.jobLastRun(store.ScheduleJobTamper),
		})
	}

	if settings.DigestEnabled {
		domains = append(domains, domainSpec{
			cadence: settings.DigestSchedule,
			name:    "digest",
			fn: func() {
				if s.digestFn == nil {
					log.Print("schedule: digest job skipped, digest not wired (SetDigestJob)")
					return
				}
				if err := s.digestFn(); err != nil {
					// Unlike drills and tamper tests, the digest records only on
					// success. It is one cheap message, so retrying tomorrow costs
					// nothing, while recording a failed send would drop that digest.
					// digestFn returns nil when notifications are off, so that
					// configuration still records and stays gated.
					log.Printf("schedule: digest job: %v", err)
					return
				}
				s.recordJobRun(store.ScheduleJobDigest)
			},
			lastRun: s.jobLastRun(store.ScheduleJobDigest),
		})
	}

	if settings.WatchdogEnabled {
		domains = append(domains, domainSpec{
			cadence: WatchdogCadence,
			name:    "watchdog",
			fn: func() {
				if s.watchdogFn == nil {
					log.Print("schedule: watchdog job skipped, watchdog not wired (SetWatchdogJob)")
					return
				}
				if err := s.watchdogFn(); err != nil {
					log.Printf("schedule: watchdog job: %v", err)
				}
			},
		})
	}

	if settings.ReceiverEnabled {
		domains = append(domains, domainSpec{
			cadence: ReceiverCadence,
			name:    "receiver",
			fn: func() {
				if s.receiverFn == nil {
					log.Print("schedule: receiver job skipped, receiver not wired (SetReceiverJob)")
					return
				}
				if err := s.receiverFn(); err != nil {
					log.Printf("schedule: receiver job: %v", err)
				}
			},
		})
	}

	if settings.PullEnabled {
		domains = append(domains, domainSpec{
			cadence: PullCadence,
			name:    "pull",
			fn: func() {
				if s.pullFn == nil {
					log.Print("schedule: pull job skipped - pull not wired (SetPullJob)")
					return
				}
				if err := s.pullFn(); err != nil {
					log.Printf("schedule: pull job: %v", err)
				}
			},
		})
	}

	if settings.FleetEnabled {
		domains = append(domains, domainSpec{
			cadence: FleetCadence,
			name:    "fleet",
			fn: func() {
				if s.fleetFn == nil {
					log.Print("schedule: fleet job skipped, fleet not wired (SetFleetJob)")
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

		// Checked after the cadence, so only a real schedule on a switched-off
		// domain is logged. A schedule the user believes is running, and is not,
		// is the mistake that costs backups.
		if d.off {
			log.Printf("schedule: %s not registered: the domain is switched off in Settings, so its schedule stays inert until the domain is switched back on", d.name)
			continue
		}

		domainName := d.name
		jobFn := d.fn

		// An everyN cadence is a daily trigger plus a due-gate, so without a
		// last-run query the job would run every day. The API rejects everyN for
		// the off-site schedules, the only specs without a query; this catches
		// older stored values, imported settings and a domain wired up without
		// its gate.
		if cad.IntervalDays > 0 && d.lastRun == nil {
			log.Printf("schedule: %s not registered: an 'everyN' cadence needs a last-run query to enforce its interval and this schedule has none, so it would fire daily. Use daily/weekly/cron instead.", d.name)
			continue
		}

		// A failed last-run query skips the fire, because a broken database must
		// not authorise a run. A zero time with no error means the job never ran,
		// and that fire runs: only a run writes the record, so deferring it would
		// skip forever.
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
				now := time.Now()
				if !EveryNDue(last, now, intervalDays) {
					log.Printf("schedule: %s everyN skipped, last run %v ago (%d calendar day(s)), interval %d days",
						domainName, now.Sub(last).Round(time.Second), calendarDaysBetween(last, now), intervalDays)
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
		// The interval and last-run query let NextRuns report the fire this entry
		// will actually run on. The guard above keeps them set together.
		s.entries = append(s.entries, scheduledEntry{
			id: id, job: job, domain: domain,
			intervalDays: cad.IntervalDays, lastRun: d.lastRun,
		})
		// Backup entries with a last-run query join the catch-up, since only they
		// can tell whether their last fire was covered by a success.
		if job == "backup" && d.lastRun != nil {
			s.catchUps = append(s.catchUps, catchUpEntry{domain: d.name, cadence: cad, lastRun: d.lastRun, id: id})
		}
		s.mu.Unlock()
	}

	if perItem {
		if err := s.registerPerItemEntries(); err != nil {
			return err
		}
	}

	return nil
}

// registerPerItemEntries adds a cron entry for every included item whose override
// is a concrete cadence. Each entry backs its item up through the same path as a
// domain run (aggregated Healthchecks, then prune and off-site). The item lists
// are read once here, so a new or changed override takes effect at the next
// reload, which every settings save triggers. Each entry re-checks its item when
// it fires and skips one that was removed or excluded since.
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
	if s.backupFiles != nil && s.listFileSetsFn != nil {
		sets, err := s.listFileSetsFn()
		if err != nil {
			log.Printf("schedule: per-item files: list file sets: %v", err)
		} else {
			for _, fs := range sets {
				if !fs.Enabled {
					continue
				}
				sched := classifyItemOverride(fs.ScheduleCadence)
				if !sched.ownEntry {
					continue
				}
				id := fs.ID
				if err := s.addPerItemEntry(sched.Spec, "files", func() {
					s.runFileSetItem(id)
				}); err != nil {
					return fmt.Errorf("schedule: per-item file set %q: %w", fs.Name, err)
				}
			}
		}
	}
	return nil
}

// addPerItemEntry registers one per-item cron entry under the domain's "backup"
// label so it shows in NextRuns. It does not join the catch-up set: there is no
// per-item last-run query, so a missed run waits for the next fire.
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

// runContainerItem backs up one container on its own cadence through the same
// path as a domain run. It re-reads the targets so that a container removed or
// excluded since the last reload is skipped.
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
		return
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

// runVMItem is the VM counterpart of runContainerItem.
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
		return
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

// runFileSetItem is the file-set counterpart of runContainerItem. It looks the
// set up by ID rather than name, because a set can be renamed between reloads
// without losing its run history.
func (s *Scheduler) runFileSetItem(id string) {
	sets, err := s.listFileSetsFn()
	if err != nil {
		log.Printf("schedule: per-item files job: list file sets: %v", err)
		return
	}
	var one *store.FileSet
	for i := range sets {
		if sets[i].ID == id {
			one = &sets[i]
			break
		}
	}
	if one == nil || !one.Enabled {
		return
	}
	s.runAggregatedHC("files", func() (int, int, []ItemFailure) {
		return RunFilesJob([]store.FileSet{*one}, s.backupFiles)
	})
	if s.pruneAfterBulkFn != nil {
		s.pruneAfterBulkFn("files")
	}
	if s.replicateAfterBulkFn != nil {
		s.replicateAfterBulkFn("files")
	}
}

// CatchUpMissed runs, once, every backup domain that missed its most recent fire
// while the app was down, as anacron does for a server that is off overnight. A
// missed domain (see missedRun) runs through the same wrapped cron job a real
// fire uses, so SkipIfStillRunning still applies and the run records and
// notifies as usual. Domains run one after another on the caller's goroutine.
//
// Call it once shortly after startup, not after a reload, so that editing a
// schedule does not start a backup. It returns the domains it ran.
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
			continue // removed by a concurrent reload
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

// drillTasks returns the scheduled drill tasks: a local "subset" integrity check
// for every enabled domain, plus an off-site "dr" drill for containers, VMs,
// flash and files when their off-site repo is configured. Config gets no DR
// drill, because a sandbox restore of the settings DB proves nothing; its real
// recovery path is the staged restart.
func drillTasks(settings store.Settings) []drillTask {
	var out []drillTask
	for _, d := range enabledDrillDomains(settings) {
		out = append(out, drillTask{domain: d, source: "local", kind: "subset"})
	}
	// An off-site DR drill downloads a whole snapshot, which costs egress on
	// metered clouds, so these have their own switch.
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

// enabledDrillDomains returns each domain switched on in Settings. A disabled
// domain has no current backups to drill.
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
// immutable, which the tamper test verifies. BombVault never claimed an
// unflagged repo was protected, so there is nothing to test there.
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

// runAggregatedHC runs a scheduled item loop between one Healthchecks start ping
// and one result ping, when SetHealthchecksAggregator has wired them. run returns
// the attempted and failed counts and the failures for the summary notification.
func (s *Scheduler) runAggregatedHC(domain string, run func() (attempted, failed int, failures []ItemFailure)) {
	if s.hcRunStart != nil {
		s.hcRunStart(domain)
	}
	attempted, failed, failures := run()
	if s.hcRunFinish != nil {
		s.hcRunFinish(domain, attempted, failed, failures)
	}
}

// RunContainersJob backs up each target with IncludeInSchedule set, one after
// another. A failing container is logged and does not stop the rest. It returns
// the attempted and failed counts and the failures, which feed the aggregated
// Healthchecks ping and the summary notification. It is exported so tests can
// run the job synchronously.
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

// RunVMsJob is RunContainersJob for VM targets.
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

// RunFilesJob is RunContainersJob for enabled file sets. backupFn receives the
// set's ID, which survives renames, while failures are reported under the set's
// name.
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
