package schedule

// ---------------------------------------------------------------------------
// #166 — "every N days" for the drills / tamper-test / digest schedules.
//
// These are BEHAVIOURAL tests, not symbol checks. Each one registers the real
// domain spec through ReloadWithDueChecks and then fires the registered cron
// entry the same way cron itself would (`sc.c.Entry(id).WrappedJob.Run()`, the
// idiom the catch-up and after-bulk tests already use). So what is exercised is
// the actual daily trigger → due-gate → job chain, with the interval driven by
// manipulating the STORED last-run timestamp rather than by waiting days.
//
// The four cases pinned for all three schedules:
//
//	inside the interval   → the trigger fires, the job does NOT run
//	older than the interval → the job DOES run
//	no record at all      → the job DOES run (see below)
//	last-run query fails  → the job does NOT run
//
// "No record at all" runs on purpose. A zero time with a nil error is a definite
// "has never run" (fresh install, or the schedule was just switched on), not an
// unknown — and deferring the first pass by a whole interval would leave a user
// who enabled drills with no verification for N days while the UI says drills
// are on, and would skip FOREVER if the record never appeared. The unknown is a
// query ERROR, and that skips. The five backup domains have always read a
// never-backed-up domain as due, so all eight schedules now agree.
// ---------------------------------------------------------------------------

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

// fakeJobRuns is an in-memory JobRunStore whose answer is fully controllable:
// a stored time, a deliberate "never ran" (zero + nil), or a query failure.
type fakeJobRuns struct {
	mu sync.Mutex
	// at holds the last-run time per job. A job absent from the map answers
	// "never ran" — zero time, NIL error — which is what a fresh install and a
	// just-migrated database both look like.
	at map[string]time.Time
	// queryErr, when set, makes every LastScheduleJobRun fail. This is the
	// "cannot tell" case, which must never be confused with "never ran".
	queryErr error
	// recordErr, when set, makes every RecordScheduleJobRun fail.
	recordErr error
	// recorded counts successful writes per job, so a test can prove the job
	// stamped its own last-run time (and that the digest did NOT stamp one after
	// a failed send).
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
	return f.at[job], nil // absent → zero time, nil error → "never ran"
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

// gateProbe is one schedule under test: the settings that register it with an
// everyN cadence, the job label its entry carries, and a counter wired into the
// real job function so "did the job run?" is answered by the work actually being
// invoked, not by inspecting the gate.
type gateProbe struct {
	name     string
	jobKey   string // store.ScheduleJob* key
	label    string // scheduledEntry.job label (jobDomainFromName)
	settings store.Settings
	// wire installs the job fn on sc, incrementing runs when the real work is
	// invoked. jobErr makes that work report a failure.
	wire func(sc *Scheduler, runs *int, jobErr error)
}

// interval used by every probe below: 7 days, so "1h ago" is comfortably inside
// it and "8 days ago" comfortably outside, with no clock-edge ambiguity.
const probeIntervalDays = 7

func gateProbes() []gateProbe {
	return []gateProbe{
		{
			name:   "drills",
			jobKey: store.ScheduleJobDrills,
			label:  "drill",
			settings: store.Settings{
				DrillsEnabled: true,
				// One enabled domain gives drillTasks a non-empty task list; an
				// empty pass deliberately records nothing (see the domain spec).
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

// fireEntry registers p's schedule and invokes its cron entry exactly once,
// through cron's own wrapped job chain — the same path a real daily trigger
// takes. It returns how many times the underlying work ran.
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

// TestEveryNGateInsideIntervalSkips — case 1. The daily trigger fires and the
// job does NOT run, because the stored last-run is 1h old against a 7-day
// interval. Driven by writing that timestamp straight into the job-run store.
func TestEveryNGateInsideIntervalSkips(t *testing.T) {
	for _, p := range gateProbes() {
		t.Run(p.name, func(t *testing.T) {
			jr := newFakeJobRuns()
			jr.set(p.jobKey, time.Now().Add(-time.Hour))
			if runs := fireEntry(t, p, jr, nil); runs != 0 {
				t.Fatalf("%s: last run 1h ago with a %d-day interval must SKIP, but the job ran %d time(s)", p.name, probeIntervalDays, runs)
			}
			// A skipped fire must not refresh the timestamp either — otherwise a
			// daily trigger would keep pushing the due date out and the pass would
			// never come due at all.
			if got := jr.recordCount(p.jobKey); got != 0 {
				t.Fatalf("%s: a skipped fire must not record a run, got %d record(s)", p.name, got)
			}
		})
	}
}

// TestEveryNGateOlderThanIntervalRuns — case 2. Same wiring, but the stored
// last-run is 8 days old against the 7-day interval, so the job DOES run and
// stamps a fresh timestamp.
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

// TestEveryNGateNoRecordRuns — case 3, the chosen "never ran" behaviour. The
// store answers zero-time WITHOUT an error (no row: fresh install, or an upgrade
// that has only just created the table), and the first trigger after enabling
// runs the pass rather than deferring it by a whole interval.
func TestEveryNGateNoRecordRuns(t *testing.T) {
	for _, p := range gateProbes() {
		t.Run(p.name, func(t *testing.T) {
			jr := newFakeJobRuns() // empty map → "never ran"
			if runs := fireEntry(t, p, jr, nil); runs == 0 {
				t.Fatalf("%s: with no last-run record the first fire after enabling must RUN, but the job never ran", p.name)
			}
			// And it must leave a record behind, so the SECOND day is gated.
			if got := jr.recordCount(p.jobKey); got != 1 {
				t.Fatalf("%s: the first run must record a last-run time, got %d record(s)", p.name, got)
			}
			if jr.storedAt(p.jobKey).IsZero() {
				t.Fatalf("%s: the recorded last-run is still zero", p.name)
			}
		})
	}
}

// TestEveryNGateQueryFailureSkips — case 4, the safety property. The last-run
// query ERRORS, which is "cannot tell", and the job must not run. This is the
// case that must never be collapsed into case 3.
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

// TestEveryNGateWithoutJobRunStoreSkips is the same safety property one level
// up: forgetting SetJobRunStore entirely must not degrade into firing the job
// daily (the exact failure the pre-#166 code had, and the reason the option was
// refused at the API). jobLastRun reports the unwired store as an error, so the
// gate skips.
func TestEveryNGateWithoutJobRunStoreSkips(t *testing.T) {
	for _, p := range gateProbes() {
		t.Run(p.name, func(t *testing.T) {
			if runs := fireEntry(t, p, nil, nil); runs != 0 {
				t.Fatalf("%s: with no job-run store an everyN cadence must SKIP, but the job ran %d time(s)", p.name, runs)
			}
		})
	}
}

// TestEveryNPartialFailureStillCountsAsRun pins requirement 3's answer for the
// two expensive multi-task passes: every task was ATTEMPTED, so the pass counts
// as a run even though each task reported an error. Gating on success instead
// would re-run the whole pass — DR restore included — every single night for as
// long as one repo stayed broken.
func TestEveryNPartialFailureStillCountsAsRun(t *testing.T) {
	for _, p := range gateProbes() {
		if p.jobKey == store.ScheduleJobDigest {
			continue // the digest's opposite choice is pinned below
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

// TestEveryNDigestFailureDoesNotCountAsRun pins the digest's opposite choice: a
// send that FAILED records nothing, so tomorrow's trigger retries instead of
// losing the digest for the whole interval. It is one cheap idempotent message,
// so a retry costs almost nothing — unlike a DR restore.
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

// TestEveryNDrillsEmptyTaskListRecordsNothing covers the one empty-pass case:
// drills enabled with no domain enabled attempts nothing, so it must not stamp a
// last-run time — otherwise enabling a domain the next day would be gated behind
// a whole interval by a pass that did no work.
func TestEveryNDrillsEmptyTaskListRecordsNothing(t *testing.T) {
	p := gateProbes()[0]
	if p.jobKey != store.ScheduleJobDrills {
		t.Fatalf("probe order changed; expected drills first, got %q", p.jobKey)
	}
	p.settings.ContainersEnabled = false // → drillTasks returns nothing
	jr := newFakeJobRuns()
	if runs := fireEntry(t, p, jr, nil); runs != 0 {
		t.Fatalf("drills: no enabled domain means no task may run, got %d", runs)
	}
	if got := jr.recordCount(store.ScheduleJobDrills); got != 0 {
		t.Fatalf("drills: an empty pass must not record a run, got %d", got)
	}
}

// TestEveryNRecordFailureLeavesJobDue proves a failed WRITE is the safe
// direction: the work ran, the stamp did not land, so the next trigger runs the
// pass again rather than the failure silently pushing the schedule out.
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
	// Still due on the next fire, because nothing was stamped.
	if runs := fireEntry(t, p, jr, nil); runs == 0 {
		t.Fatal("drills: after a failed record the next trigger must run the pass again")
	}
}

// TestEveryNUnenforceableCadenceIsNotRegistered pins the fail-safe added
// alongside this feature: a domain with NO last-run query at all (the five
// off-site replication schedules) used to have its everyN cadence silently
// downgraded to the bare daily trigger — firing the job every day, N times too
// often. Such an entry is now refused registration outright.
func TestEveryNUnenforceableCadenceIsNotRegistered(t *testing.T) {
	noTargets := func() ([]store.Target, error) { return nil, nil }
	sc := New(func(string) error { return nil }, noTargets)
	replicated := 0
	sc.SetOffsiteJob(func(string) error { replicated++; return nil })

	settings := store.Settings{ContainersOffsiteSchedule: "everyN 5 02:00"}
	if err := sc.ReloadWithDueChecks(settings, nil, nil, nil, nil, nil, nil); err != nil {
		t.Fatalf("ReloadWithDueChecks: %v", err)
	}
	for _, e := range sc.entries {
		if e.job == "offsite" {
			sc.c.Entry(e.id).WrappedJob.Run()
			t.Fatalf("an unenforceable everyN off-site cadence must not be registered (it would replicate daily); entry for domain %q fired %d replication(s)", e.domain, replicated)
		}
	}

	// The same schedule on a plain daily cadence is registered and fires — the
	// refusal is specific to an unenforceable everyN, not to off-site as such.
	if err := sc.ReloadWithDueChecks(store.Settings{ContainersOffsiteSchedule: "daily 02:00"}, nil, nil, nil, nil, nil, nil); err != nil {
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
