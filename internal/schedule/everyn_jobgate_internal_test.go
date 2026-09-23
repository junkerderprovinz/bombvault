package schedule

// These tests cover the everyN cadence of the drills, tamper-test and digest
// schedules. Each registers the real domain spec through ReloadWithDueChecks
// and fires the entry the way cron does, so trigger, due gate and job run
// together, and the interval is set through the stored last-run time.
//
// A zero time with a nil error means the job has never run, and it runs:
// waiting a whole interval would leave drills that the UI shows as on
// unverified for N days. Only a query error, where the store cannot tell,
// skips. The backup domains treat a domain that was never backed up the same
// way.

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

// fakeJobRuns is an in-memory JobRunStore that answers with a stored time,
// "never ran" (zero time, nil error) or a query failure.
type fakeJobRuns struct {
	mu sync.Mutex
	// at holds the last-run time per job. A job missing from it has never run,
	// as on a fresh install.
	at map[string]time.Time
	// queryErr, when set, makes every LastScheduleJobRun fail.
	queryErr error
	// recordErr, when set, makes every RecordScheduleJobRun fail.
	recordErr error
	// recorded counts successful writes per job.
	recorded map[string]int
}

func newFakeJobRuns() *fakeJobRuns {
	return &fakeJobRuns{at: map[string]time.Time{}, recorded: map[string]int{}}
}

func (f *fakeJobRuns) LastScheduleJobRun(job string) (time.Time, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.queryErr != nil {
		return time.Time{}, f.queryErr
	}
	return f.at[job], nil
}

func (f *fakeJobRuns) RecordScheduleJobRun(job string, at time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.recordErr != nil {
		return f.recordErr
	}
	f.at[job] = at
	f.recorded[job]++
	return nil
}

func (f *fakeJobRuns) set(job string, at time.Time) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.at[job] = at
}

func (f *fakeJobRuns) recordCount(job string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.recorded[job]
}

func (f *fakeJobRuns) storedAt(job string) time.Time {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.at[job]
}

// gateProbe is one schedule under test: settings that register it with an
// everyN cadence, the label its entry carries, and a wire func that counts
// calls to the real job function.
type gateProbe struct {
	name     string
	jobKey   string // store.ScheduleJob* key
	label    string // scheduledEntry.job label (jobDomainFromName)
	settings store.Settings
	// wire installs the job fn on sc, incrementing runs when the real work is
	// invoked. jobErr makes that work report a failure.
	wire func(sc *Scheduler, runs *int, jobErr error)
}

// probeIntervalDays puts "1h ago" well inside the interval and "8 days ago"
// well outside it.
const probeIntervalDays = 7

func gateProbes() []gateProbe {
	return []gateProbe{
		{
			name:   "drills",
			jobKey: store.ScheduleJobDrills,
			label:  "drill",
			settings: store.Settings{
				DrillsEnabled: true,
				// drillTasks needs an enabled domain; an empty pass records
				// nothing.
				ContainersEnabled: true,
				DrillsSchedule:    "everyN 7 03:00",
			},
			wire: func(sc *Scheduler, runs *int, jobErr error) {
				sc.SetDrillJob(func(_, _, _ string) error { *runs++; return jobErr })
			},
		},
		{
			name:   "tamper",
			jobKey: store.ScheduleJobTamper,
			label:  "tamper",
			settings: store.Settings{
				ContainersOffsiteImmutable: true,
				TamperTestSchedule:         "everyN 7 04:00",
			},
			wire: func(sc *Scheduler, runs *int, jobErr error) {
				sc.SetTamperJob(func(string) error { *runs++; return jobErr })
			},
		},
		{
			name:   "digest",
			jobKey: store.ScheduleJobDigest,
			label:  "digest",
			settings: store.Settings{
				DigestEnabled:  true,
				DigestSchedule: "everyN 7 05:00",
			},
			wire: func(sc *Scheduler, runs *int, jobErr error) {
				sc.SetDigestJob(func() error { *runs++; return jobErr })
			},
		},
	}
}

// fireEntry registers p's schedule, runs its cron entry once through cron's
// wrapped job chain, and returns how often the job ran.
func fireEntry(t *testing.T, p gateProbe, jobRuns JobRunStore, jobErr error) int {
	t.Helper()
	noTargets := func() ([]store.Target, error) { return nil, nil }
	sc := New(func(string) error { return nil }, noTargets)
	if jobRuns != nil {
		sc.SetJobRunStore(jobRuns)
	}
	runs := 0
	p.wire(sc, &runs, jobErr)

	if err := sc.ReloadWithDueChecks(p.settings, nil, nil, nil, nil, nil, nil); err != nil {
		t.Fatalf("%s: ReloadWithDueChecks: %v", p.name, err)
	}
	fired := 0
	for _, e := range sc.entries {
		if e.job != p.label {
			continue
		}
		fired++
		sc.c.Entry(e.id).WrappedJob.Run()
	}
	if fired != 1 {
		t.Fatalf("%s: expected exactly 1 registered %q entry to fire, got %d", p.name, p.label, fired)
	}
	return runs
}

// TestEveryNGateInsideIntervalSkips fires the trigger one hour after the last
// run of a 7-day interval; the job must not run.
func TestEveryNGateInsideIntervalSkips(t *testing.T) {
	for _, p := range gateProbes() {
		t.Run(p.name, func(t *testing.T) {
			jr := newFakeJobRuns()
			jr.set(p.jobKey, time.Now().Add(-time.Hour))
			if runs := fireEntry(t, p, jr, nil); runs != 0 {
				t.Fatalf("%s: last run 1h ago with a %d-day interval must SKIP, but the job ran %d time(s)", p.name, probeIntervalDays, runs)
			}
			// A skipped fire must not refresh the timestamp, or the daily
			// trigger would keep pushing the due date out.
			if got := jr.recordCount(p.jobKey); got != 0 {
				t.Fatalf("%s: a skipped fire must not record a run, got %d record(s)", p.name, got)
			}
		})
	}
}

// TestEveryNGateOlderThanIntervalRuns checks that a job last run 8 days ago on
// a 7-day interval runs and records a new time.
func TestEveryNGateOlderThanIntervalRuns(t *testing.T) {
	for _, p := range gateProbes() {
		t.Run(p.name, func(t *testing.T) {
			jr := newFakeJobRuns()
			old := time.Now().Add(-8 * 24 * time.Hour)
			jr.set(p.jobKey, old)
			if runs := fireEntry(t, p, jr, nil); runs == 0 {
				t.Fatalf("%s: last run 8 days ago with a %d-day interval must RUN, but the job never ran", p.name, probeIntervalDays)
			}
			if got := jr.recordCount(p.jobKey); got != 1 {
				t.Fatalf("%s: a completed pass must record exactly 1 run, got %d", p.name, got)
			}
			if !jr.storedAt(p.jobKey).After(old) {
				t.Fatalf("%s: the recorded last-run must move forward, still %v", p.name, jr.storedAt(p.jobKey))
			}
		})
	}
}

// TestEveryNGateNoRecordRuns checks that a job without a last-run record runs
// on the first trigger instead of waiting a whole interval.
func TestEveryNGateNoRecordRuns(t *testing.T) {
	for _, p := range gateProbes() {
		t.Run(p.name, func(t *testing.T) {
			jr := newFakeJobRuns()
			if runs := fireEntry(t, p, jr, nil); runs == 0 {
				t.Fatalf("%s: with no last-run record the first fire after enabling must RUN, but the job never ran", p.name)
			}
			// The record it leaves gates the next day.
			if got := jr.recordCount(p.jobKey); got != 1 {
				t.Fatalf("%s: the first run must record a last-run time, got %d record(s)", p.name, got)
			}
			if jr.storedAt(p.jobKey).IsZero() {
				t.Fatalf("%s: the recorded last-run is still zero", p.name)
			}
		})
	}
}

// TestEveryNGateQueryFailureSkips checks that a failing last-run query skips
// the job. Unlike a missing record, an error means the store cannot tell.
func TestEveryNGateQueryFailureSkips(t *testing.T) {
	for _, p := range gateProbes() {
		t.Run(p.name, func(t *testing.T) {
			jr := newFakeJobRuns()
			jr.queryErr = errors.New("database is locked")
			if runs := fireEntry(t, p, jr, nil); runs != 0 {
				t.Fatalf("%s: a failing last-run query must SKIP, but the job ran %d time(s)", p.name, runs)
			}
		})
	}
}

// TestEveryNGateWithoutJobRunStoreSkips checks that a scheduler without
// SetJobRunStore skips instead of firing the job daily: jobLastRun reports the
// missing store as an error.
func TestEveryNGateWithoutJobRunStoreSkips(t *testing.T) {
	for _, p := range gateProbes() {
		t.Run(p.name, func(t *testing.T) {
			if runs := fireEntry(t, p, nil, nil); runs != 0 {
				t.Fatalf("%s: with no job-run store an everyN cadence must SKIP, but the job ran %d time(s)", p.name, runs)
			}
		})
	}
}

// TestEveryNPartialFailureStillCountsAsRun checks that a drills or tamper pass
// counts as a run once every task was attempted, even if they all failed.
// Gating on success would repeat the whole pass, DR restore included, every
// night while one repo stays broken.
func TestEveryNPartialFailureStillCountsAsRun(t *testing.T) {
	for _, p := range gateProbes() {
		if p.jobKey == store.ScheduleJobDigest {
			continue // the digest retries instead; see below
		}
		t.Run(p.name, func(t *testing.T) {
			jr := newFakeJobRuns()
			jr.set(p.jobKey, time.Now().Add(-8*24*time.Hour))
			if runs := fireEntry(t, p, jr, errors.New("repo unreachable")); runs == 0 {
				t.Fatalf("%s: the pass must still run", p.name)
			}
			if got := jr.recordCount(p.jobKey); got != 1 {
				t.Fatalf("%s: an attempted-but-failing pass must still record a run, got %d", p.name, got)
			}
		})
	}
}

// TestEveryNDigestFailureDoesNotCountAsRun checks that a failed digest send
// records nothing, so the next day's trigger retries it instead of losing the
// digest for the whole interval. A retry is one cheap message.
func TestEveryNDigestFailureDoesNotCountAsRun(t *testing.T) {
	var p gateProbe
	for _, c := range gateProbes() {
		if c.jobKey == store.ScheduleJobDigest {
			p = c
		}
	}
	jr := newFakeJobRuns()
	jr.set(p.jobKey, time.Now().Add(-8*24*time.Hour))
	if runs := fireEntry(t, p, jr, errors.New("smtp down")); runs != 1 {
		t.Fatalf("digest: the send must be attempted once, got %d", runs)
	}
	if got := jr.recordCount(p.jobKey); got != 0 {
		t.Fatalf("digest: a FAILED send must not record a run (tomorrow retries), got %d", got)
	}
}

// TestEveryNDrillsEmptyTaskListRecordsNothing checks that a drill pass with no
// enabled domain records no run, so a domain enabled the next day is not held
// back a whole interval by a pass that did nothing.
func TestEveryNDrillsEmptyTaskListRecordsNothing(t *testing.T) {
	p := gateProbes()[0]
	if p.jobKey != store.ScheduleJobDrills {
		t.Fatalf("probe order changed; expected drills first, got %q", p.jobKey)
	}
	p.settings.ContainersEnabled = false // drillTasks returns nothing
	jr := newFakeJobRuns()
	if runs := fireEntry(t, p, jr, nil); runs != 0 {
		t.Fatalf("drills: no enabled domain means no task may run, got %d", runs)
	}
	if got := jr.recordCount(store.ScheduleJobDrills); got != 0 {
		t.Fatalf("drills: an empty pass must not record a run, got %d", got)
	}
}

// TestEveryNRecordFailureLeavesJobDue checks that when recording the run fails,
// the next trigger runs the pass again.
func TestEveryNRecordFailureLeavesJobDue(t *testing.T) {
	p := gateProbes()[0] // drills
	jr := newFakeJobRuns()
	jr.recordErr = errors.New("disk full")
	if runs := fireEntry(t, p, jr, nil); runs == 0 {
		t.Fatal("drills: the pass must run")
	}
	if !jr.storedAt(store.ScheduleJobDrills).IsZero() {
		t.Fatal("drills: a failed record must not leave a timestamp behind")
	}
	if runs := fireEntry(t, p, jr, nil); runs == 0 {
		t.Fatal("drills: after a failed record the next trigger must run the pass again")
	}
}

// TestEveryNUnenforceableCadenceIsNotRegistered checks that an everyN cadence on
// a schedule without a last-run query, such as off-site replication, is not
// registered, because it would fire every day.
func TestEveryNUnenforceableCadenceIsNotRegistered(t *testing.T) {
	noTargets := func() ([]store.Target, error) { return nil, nil }
	sc := New(func(string) error { return nil }, noTargets)
	replicated := 0
	sc.SetOffsiteJob(func(string) error { replicated++; return nil })

	settings := store.Settings{ContainersEnabled: true, ContainersOffsiteSchedule: "everyN 5 02:00"}
	if err := sc.ReloadWithDueChecks(settings, nil, nil, nil, nil, nil, nil); err != nil {
		t.Fatalf("ReloadWithDueChecks: %v", err)
	}
	for _, e := range sc.entries {
		if e.job == "offsite" {
			sc.c.Entry(e.id).WrappedJob.Run()
			t.Fatalf("an unenforceable everyN off-site cadence must not be registered (it would replicate daily); entry for domain %q fired %d replication(s)", e.domain, replicated)
		}
	}

	// The same schedule with a daily cadence registers and fires.
	if err := sc.ReloadWithDueChecks(store.Settings{ContainersEnabled: true, ContainersOffsiteSchedule: "daily 02:00"}, nil, nil, nil, nil, nil, nil); err != nil {
		t.Fatalf("ReloadWithDueChecks(daily): %v", err)
	}
	found := false
	for _, e := range sc.entries {
		if e.job == "offsite" {
			found = true
			sc.c.Entry(e.id).WrappedJob.Run()
		}
	}
	if !found || replicated != 1 {
		t.Fatalf("a daily off-site cadence must register and fire; found=%v replicated=%d", found, replicated)
	}
}
