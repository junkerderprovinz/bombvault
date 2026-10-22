package schedule

import (
	"bytes"
	"log"
	"slices"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

// fireJob registers settings and runs every entry of job once.
func fireJob(t *testing.T, sc *Scheduler, settings store.Settings, job string) {
	t.Helper()
	if err := sc.Reload(settings); err != nil {
		t.Fatalf("Reload: %v", err)
	}
	fired := 0
	for _, e := range sc.entries {
		if e.job == job {
			fired++
			sc.c.Entry(e.id).WrappedJob.Run()
		}
	}
	if fired == 0 {
		t.Fatalf("no %q entry registered", job)
	}
}

func newOffsiteScheduler(inUse bool, jobRuns JobRunStore) *Scheduler {
	sc := New(func(string) error { return nil }, func() ([]store.Target, error) { return nil, nil })
	sc.SetJobRunStore(jobRuns)
	sc.SetOffsiteInUse(func(string) bool { return inUse })
	return sc
}

// The off-site setting stays filled while its target is switched off, so it
// alone does not say whether there is anywhere to drill.
var drillSettings = store.Settings{
	ContainersEnabled:    true,
	ContainersOffsite:    "rest:http://192.168.20.112:38000/bv/containers",
	DrillsEnabled:        true,
	OffsiteDrillsEnabled: true,
	DrillsSchedule:       "daily 03:00",
}

func TestTheScheduledDrillsLeaveOutADomainWhoseTargetsAreAllOff(t *testing.T) {
	for _, inUse := range []bool{true, false} {
		sc := newOffsiteScheduler(inUse, newFakeJobRuns())
		var ran []drillTask
		sc.SetDrillJob(func(domain, source, kind string) error {
			ran = append(ran, drillTask{domain: domain, source: source, kind: kind})
			return nil
		})

		fireJob(t, sc, drillSettings, "drill")

		dr := drillTask{domain: "containers", source: "offsite", kind: "dr"}
		if hasDrillTask(ran, dr) != inUse {
			t.Errorf("target in use %v: drills run %v", inUse, ran)
		}
		if !hasDrillTask(ran, drillTask{domain: "containers", source: "local", kind: "subset"}) {
			t.Errorf("target in use %v: the local drill must run either way, ran %v", inUse, ran)
		}
	}
}

// A group peer's or an added target leaves the off-site setting empty, and the
// DR drill still goes to it.
func TestTheScheduledDRDrillRunsForATargetTheOffsiteSettingDoesNotName(t *testing.T) {
	sc := newOffsiteScheduler(true, newFakeJobRuns())
	var ran []drillTask
	sc.SetDrillJob(func(domain, source, kind string) error {
		ran = append(ran, drillTask{domain: domain, source: source, kind: kind})
		return nil
	})
	settings := drillSettings
	settings.ContainersOffsite = ""

	fireJob(t, sc, settings, "drill")

	if !hasDrillTask(ran, drillTask{domain: "containers", source: "offsite", kind: "dr"}) {
		t.Fatalf("drills run %v, want the off-site DR drill", ran)
	}
}

func TestATamperSweepWithEveryTargetOffRunsNothingAndRecordsNoRun(t *testing.T) {
	jobRuns := newFakeJobRuns()
	sc := newOffsiteScheduler(false, jobRuns)
	tested := 0
	sc.SetTamperJob(func(string) error { tested++; return nil })

	fireJob(t, sc, store.Settings{ContainersOffsiteImmutable: true, TamperTestSchedule: "daily 04:00"}, "tamper")

	if tested != 0 {
		t.Fatalf("tamper test ran %d times with every target off", tested)
	}
	if n := jobRuns.recordCount(store.ScheduleJobTamper); n != 0 {
		t.Fatalf("recorded %d tamper runs for a sweep that tested nothing", n)
	}
}

func TestTheOffsiteScheduleSkipsADomainWhoseTargetsAreAllOff(t *testing.T) {
	for _, inUse := range []bool{true, false} {
		sc := newOffsiteScheduler(inUse, newFakeJobRuns())
		copies := 0
		sc.SetOffsiteJob(func(string) error { copies++; return nil })

		fireJob(t, sc, store.Settings{ContainersEnabled: true, ContainersOffsiteSchedule: "daily 02:00"}, "offsite")

		if (copies > 0) != inUse {
			t.Errorf("target in use %v: %d copies", inUse, copies)
		}
	}
}

// captureLog sends the standard logger into a buffer for the rest of the test.
func captureLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	prev, flags := log.Writer(), log.Flags()
	log.SetOutput(&buf)
	log.SetFlags(0)
	t.Cleanup(func() { log.SetOutput(prev); log.SetFlags(flags) })
	return &buf
}

func TestAnOffsiteFireWithEveryTargetOffIsLoggedAsSkippedWithItsReason(t *testing.T) {
	sc := newOffsiteScheduler(false, newFakeJobRuns())
	sc.SetOffsiteJob(func(string) error { return nil })
	out := captureLog(t)

	fireJob(t, sc, store.Settings{ContainersEnabled: true, ContainersOffsiteSchedule: "daily 02:00"}, "offsite")

	got := out.String()
	if strings.Contains(got, "running containers-offsite job") {
		t.Errorf("log = %q, want no running line for a fire that copies nothing", got)
	}
	if !strings.Contains(got, "containers-offsite job skipped: every off-site target of containers is switched off") {
		t.Errorf("log = %q, want the skip and its reason", got)
	}
}

func TestATamperFireWithEveryTargetOffIsLoggedAsSkippedWithItsReason(t *testing.T) {
	sc := newOffsiteScheduler(false, newFakeJobRuns())
	sc.SetTamperJob(func(string) error { return nil })
	out := captureLog(t)

	fireJob(t, sc, store.Settings{ContainersOffsiteImmutable: true, TamperTestSchedule: "daily 04:00"}, "tamper")

	got := out.String()
	if strings.Contains(got, "running tamper job") || !strings.Contains(got, "tamper job skipped: no switched-on off-site target has delete protection") {
		t.Errorf("log = %q, want the skip and its reason instead of a running line", got)
	}
}

func TestADrillLeftOutForItsTargetsIsLoggedWithTheReason(t *testing.T) {
	sc := newOffsiteScheduler(false, newFakeJobRuns())
	sc.SetDrillJob(func(string, string, string) error { return nil })
	out := captureLog(t)

	fireJob(t, sc, drillSettings, "drill")

	if got := out.String(); !strings.Contains(got, "drills job: containers/offsite(dr) skipped: every off-site target of containers is switched off") {
		t.Errorf("log = %q, want the left-out drill named with its reason", got)
	}
}

func TestTheTamperJobIsPlannedForASectionWhoseOnlyAppendOnlyCopyIsAnAdditionalTarget(t *testing.T) {
	sc := newOffsiteScheduler(true, newFakeJobRuns())
	sc.SetAppendOnlyDomains(func() []string { return []string{"vms"} })
	var tested []string
	sc.SetTamperJob(func(domain string) error { tested = append(tested, domain); return nil })

	// No off-site settings carry the flag, only the target does.
	fireJob(t, sc, store.Settings{VMsEnabled: true, TamperTestSchedule: "daily 04:00"}, "tamper")

	if !slices.Equal(tested, []string{"vms"}) {
		t.Fatalf("tamper test ran for %v, want vms", tested)
	}
}

func TestNoTamperJobIsPlannedWithoutAnAppendOnlyTarget(t *testing.T) {
	sc := newOffsiteScheduler(true, newFakeJobRuns())
	sc.SetAppendOnlyDomains(func() []string { return nil })
	if err := sc.Reload(store.Settings{VMsEnabled: true, TamperTestSchedule: "daily 04:00"}); err != nil {
		t.Fatal(err)
	}
	for _, e := range sc.entries {
		if e.job == "tamper" {
			t.Fatal("a tamper job was planned with nothing append-only to test")
		}
	}
}
