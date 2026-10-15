package schedule

import (
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
