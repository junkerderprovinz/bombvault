package schedule

import (
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

// hasDrillTask reports whether the task list holds an exact task.
func hasDrillTask(tasks []drillTask, want drillTask) bool {
	for _, tk := range tasks {
		if tk == want {
			return true
		}
	}
	return false
}

// TestDrillTasksIncludeZFSOffsiteDR checks that an enabled ZFS domain gets the
// local subset check, plus a DR drill once it has an off-site target and
// off-site drills are on.
func TestDrillTasksIncludeZFSOffsiteDR(t *testing.T) {
	base := store.Settings{
		ZFSEnabled:           true,
		ZFSSchedule:          "daily 03:00",
		DrillsEnabled:        true,
		OffsiteDrillsEnabled: true,
	}

	tasks := drillScheduler(targetRows{}, nil, nil).drillTasks(base)
	if !hasDrillTask(tasks, drillTask{domain: "zfs", source: "local", kind: "subset"}) {
		t.Fatalf("expected a local subset drill for zfs, got %v", tasks)
	}
	for _, tk := range tasks {
		if tk.domain == "zfs" && tk.kind == "dr" {
			t.Fatalf("zfs must not get a DR task without an off-site target: %v", tasks)
		}
	}

	sc := drillScheduler(targetRows{"zfs": {offsiteTarget("zfs", "z1", true)}}, nil, nil)
	tasks = sc.drillTasks(base)
	if !hasDrillTask(tasks, drillTask{domain: "zfs", source: "offsite:z1", kind: "dr", targetID: "z1"}) {
		t.Fatalf("expected an off-site DR drill for zfs, got %v", tasks)
	}

	optedOut := base
	optedOut.OffsiteDrillsEnabled = false
	for _, tk := range sc.drillTasks(optedOut) {
		if tk.domain == "zfs" && tk.kind == "dr" {
			t.Fatalf("no off-site DR task expected when off-site drills are off, got %v", tk)
		}
	}
}

// TestEnabledDrillDomainsIncludeZFS checks that the subset drill follows the
// ZFS switch: a disabled domain has no current backups to drill.
func TestEnabledDrillDomainsIncludeZFS(t *testing.T) {
	for _, d := range enabledDrillDomains(store.Settings{}) {
		if d == "zfs" {
			t.Fatalf("a switched-off ZFS domain must not be drilled: %v", d)
		}
	}
	got := enabledDrillDomains(store.Settings{FilesEnabled: true, ZFSEnabled: true})
	if len(got) != 2 || got[0] != "files" || got[1] != "zfs" {
		t.Fatalf("zfs must be drilled after files, got %v", got)
	}
}

// TestImmutableOffsiteDomainsIncludeZFS checks that the scheduled tamper test
// covers the ZFS domain once its off-site repo is flagged immutable.
func TestImmutableOffsiteDomainsIncludeZFS(t *testing.T) {
	for _, d := range immutableOffsiteDomains(store.Settings{}) {
		if d == "zfs" {
			t.Fatalf("zfs must not be a tamper-test domain when the flag is unset: %v", d)
		}
	}
	got := immutableOffsiteDomains(store.Settings{FilesOffsiteImmutable: true, ZFSOffsiteImmutable: true})
	if len(got) != 2 || got[0] != "files" || got[1] != "zfs" {
		t.Fatalf("zfs must follow files in the tamper-test domains, got %v", got)
	}
}

// drillSettings switches on every domain and both kinds of drill, the way a
// box that drills everything is set up.
func drillSettings() store.Settings {
	return store.Settings{
		ContainersEnabled:    true,
		VMsEnabled:           true,
		FlashEnabled:         true,
		ConfigEnabled:        true,
		FilesEnabled:         true,
		DrillsEnabled:        true,
		OffsiteDrillsEnabled: true,
		DrillsSchedule:       "daily 03:00",
	}
}

func offsiteTarget(domain, id string, enabled bool) store.OffsiteTarget {
	return store.OffsiteTarget{ID: id, Domain: domain, Name: id, Repo: "s3:" + id, Enabled: enabled}
}

// targetRows answers the drills job's target listing from a fixed table.
type targetRows map[string][]store.OffsiteTarget

func (r targetRows) list(domain string) ([]store.OffsiteTarget, error) {
	return r[domain], nil
}

func drillScheduler(rows targetRows, jobRuns JobRunStore, drillFn func(domain, source, kind string) error) *Scheduler {
	sc := New(func(string) error { return nil }, func() ([]store.Target, error) { return nil, nil })
	if jobRuns != nil {
		sc.SetJobRunStore(jobRuns)
	}
	sc.SetDrillJob(drillFn, rows.list)
	return sc
}

// drillPass registers the drills entry from settings, fires it once the way
// cron does, and returns the sources of the DR drills it ran.
func drillPass(t *testing.T, sc *Scheduler, settings store.Settings, drilled *[]string) []string {
	t.Helper()
	if err := sc.Reload(settings); err != nil {
		t.Fatalf("Reload: %v", err)
	}
	before := len(*drilled)
	for _, e := range sc.entries {
		if e.job == "drill" {
			sc.c.Entry(e.id).WrappedJob.Run()
		}
	}
	return (*drilled)[before:]
}

// age moves every stored time back by d. Turns are stamped to the second, and
// passes a test runs back to back would otherwise share one.
func (f *fakeJobRuns) age(d time.Duration) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for job, at := range f.at {
		f.at[job] = at.Add(-d)
	}
}

func recordDR(drilled *[]string, err error) func(domain, source, kind string) error {
	return func(_, source, kind string) error {
		if kind == "dr" {
			*drilled = append(*drilled, source)
		}
		return err
	}
}

func TestDrillTargetsAreTheDomainsSwitchedOnTargets(t *testing.T) {
	b2 := offsiteTarget("containers", "b2", true)
	nas := offsiteTarget("containers", "nas", false)
	hetzner := offsiteTarget("containers", "hetzner", true)
	rows := []store.OffsiteTarget{b2, nas, hetzner}

	got := DrillTargets(drillSettings(), "containers", rows)
	if len(got) != 2 || got[0].ID != "b2" || got[1].ID != "hetzner" {
		t.Fatalf("drill targets %+v, want b2 and hetzner in their order", got)
	}

	for name, change := range map[string]func(*store.Settings){
		"drills off":         func(s *store.Settings) { s.DrillsEnabled = false },
		"off-site drill off": func(s *store.Settings) { s.OffsiteDrillsEnabled = false },
		"domain off":         func(s *store.Settings) { s.ContainersEnabled = false },
	} {
		s := drillSettings()
		change(&s)
		if got := DrillTargets(s, "containers", rows); len(got) != 0 {
			t.Errorf("%s: drill targets %+v, want none", name, got)
		}
	}
	if got := DrillTargets(drillSettings(), "config", []store.OffsiteTarget{offsiteTarget("config", "b2", true)}); len(got) != 0 {
		t.Errorf("config drill targets %+v, want none", got)
	}
}

func TestDrillTasks(t *testing.T) {
	rows := targetRows{
		"containers": {offsiteTarget("containers", "c1", true)},
		"vms":        {offsiteTarget("vms", "v1", true)},
		"flash":      {offsiteTarget("flash", "f1", true)},
		"files":      {offsiteTarget("files", "d1", true)},
		"config":     {offsiteTarget("config", "k1", true)},
	}
	sc := drillScheduler(rows, nil, nil)

	t.Run("a local subset check per enabled domain", func(t *testing.T) {
		var subset []string
		for _, tk := range sc.drillTasks(drillSettings()) {
			if tk.kind == "subset" && tk.source == "local" {
				subset = append(subset, tk.domain)
			}
		}
		if !slices.Equal(subset, []string{"containers", "vms", "flash", "config", "files"}) {
			t.Fatalf("subset drills for %v, want every enabled domain", subset)
		}
	})

	t.Run("one DR drill per domain with a target, though its off-site field is empty", func(t *testing.T) {
		var dr []string
		for _, tk := range sc.drillTasks(drillSettings()) {
			if tk.kind == "dr" {
				dr = append(dr, tk.domain+" "+tk.source)
			}
		}
		want := []string{"containers offsite:c1", "vms offsite:v1", "flash offsite:f1", "files offsite:d1"}
		if !slices.Equal(dr, want) {
			t.Fatalf("DR drills %v, want %v", dr, want)
		}
	})

	t.Run("no DR drill without a switched-on target", func(t *testing.T) {
		sc := drillScheduler(targetRows{"flash": {offsiteTarget("flash", "f1", false)}}, nil, nil)
		for _, tk := range sc.drillTasks(drillSettings()) {
			if tk.kind == "dr" {
				t.Fatalf("unexpected DR drill %+v", tk)
			}
		}
	})

	t.Run("a switched-off domain gets no drill at all", func(t *testing.T) {
		s := drillSettings()
		s.ContainersEnabled = false
		for _, tk := range sc.drillTasks(s) {
			if tk.domain == "containers" {
				t.Fatalf("a switched-off domain must yield no drill task, got %+v", tk)
			}
		}
	})

	t.Run("the off-site drill switch leaves the local checks", func(t *testing.T) {
		s := drillSettings()
		s.OffsiteDrillsEnabled = false
		tasks := sc.drillTasks(s)
		for _, tk := range tasks {
			if tk.kind == "dr" {
				t.Fatalf("no DR drill expected with off-site drills off, got %+v", tk)
			}
		}
		if len(tasks) != 5 {
			t.Fatalf("tasks %+v, want the five local subset checks", tasks)
		}
	})
}

func TestTheDrillsJobTakesADomainsTargetsInTurn(t *testing.T) {
	rows := targetRows{"flash": {
		offsiteTarget("flash", "b2", true),
		offsiteTarget("flash", "nas", false),
		offsiteTarget("flash", "hetzner", true),
	}}
	jr := newFakeJobRuns()
	var drilled []string
	sc := drillScheduler(rows, jr, recordDR(&drilled, nil))

	for i, want := range []string{"offsite:b2", "offsite:hetzner", "offsite:b2"} {
		got := drillPass(t, sc, drillSettings(), &drilled)
		if !slices.Equal(got, []string{want}) {
			t.Fatalf("pass %d drilled %v, want only %s", i+1, got, want)
		}
		jr.age(time.Minute)
	}

	rows["flash"] = append(rows["flash"], offsiteTarget("flash", "wasabi", true))
	if got := drillPass(t, sc, drillSettings(), &drilled); !slices.Equal(got, []string{"offsite:wasabi"}) {
		t.Fatalf("a new target drilled %v, want wasabi before the others come round again", got)
	}
}

func TestTheTurnOfTheDrillTargetsSurvivesARestart(t *testing.T) {
	rows := targetRows{"containers": {offsiteTarget("containers", "b2", true), offsiteTarget("containers", "hetzner", true)}}
	jr := newFakeJobRuns()
	jr.set(store.ScheduleJobDrillTarget("b2"), time.Now().Add(-24*time.Hour))
	var drilled []string

	sc := drillScheduler(rows, jr, recordDR(&drilled, nil))
	if got := drillPass(t, sc, drillSettings(), &drilled); !slices.Equal(got, []string{"offsite:hetzner"}) {
		t.Fatalf("drilled %v after b2 took the last turn, want hetzner", got)
	}
}

func TestAFailedDrillStillPassesTheTurn(t *testing.T) {
	rows := targetRows{"files": {offsiteTarget("files", "b2", true), offsiteTarget("files", "hetzner", true)}}
	jr := newFakeJobRuns()
	var drilled []string
	sc := drillScheduler(rows, jr, recordDR(&drilled, errors.New("repository unreachable")))

	drillPass(t, sc, drillSettings(), &drilled)
	if got := drillPass(t, sc, drillSettings(), &drilled); !slices.Equal(got, []string{"offsite:hetzner"}) {
		t.Fatalf("drilled %v after b2 failed, want hetzner", got)
	}
}

func TestADomainWithOneTargetDrillsItEveryPass(t *testing.T) {
	rows := targetRows{"containers": {offsiteTarget("containers", "b2", true)}}
	var drilled []string
	sc := drillScheduler(rows, newFakeJobRuns(), recordDR(&drilled, nil))
	for range 2 {
		if got := drillPass(t, sc, drillSettings(), &drilled); !slices.Equal(got, []string{"offsite:b2"}) {
			t.Fatalf("drilled %v, want b2", got)
		}
	}
}
