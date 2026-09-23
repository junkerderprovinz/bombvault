package schedule

import (
	"errors"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

// TestScheduledRunAggregatesHealthchecksPings checks that a scheduled domain run
// sends one Healthchecks start and one finish, and that the finish carries the
// attempted and failed counts plus each failed item's name and reason for the
// summary notification.
func TestScheduledRunAggregatesHealthchecksPings(t *testing.T) {
	sc := New(func(string) error { return nil }, func() ([]store.Target, error) { return nil, nil })

	var starts []string
	type finish struct {
		domain            string
		attempted, failed int
		failures          []ItemFailure
	}
	var finishes []finish
	sc.SetHealthchecksAggregator(
		func(domain string) { starts = append(starts, domain) },
		func(domain string, attempted, failed int, failures []ItemFailure) {
			finishes = append(finishes, finish{domain, attempted, failed, failures})
		},
	)

	targets := []store.Target{
		{ContainerName: "a", IncludeInSchedule: true},
		{ContainerName: "b", IncludeInSchedule: true},
		{ContainerName: "c", IncludeInSchedule: true},
	}

	sc.runAggregatedHC("containers", func() (int, int, []ItemFailure) {
		return RunContainersJob(targets, func(string) error { return nil })
	})
	if len(starts) != 1 || starts[0] != "containers" {
		t.Fatalf("expected exactly one aggregate start for the run, got %v", starts)
	}
	if len(finishes) != 1 || finishes[0].attempted != 3 || finishes[0].failed != 0 || len(finishes[0].failures) != 0 {
		t.Fatalf("expected one success finish (attempted 3, failed 0, no failures), got %+v", finishes)
	}

	sc.runAggregatedHC("containers", func() (int, int, []ItemFailure) {
		return RunContainersJob(targets, func(name string) error {
			if name == "b" {
				return errors.New("boom")
			}
			return nil
		})
	})
	if len(starts) != 2 {
		t.Fatalf("expected a second aggregate start, got %v", starts)
	}
	if len(finishes) != 2 || finishes[1].attempted != 3 || finishes[1].failed != 1 {
		t.Fatalf("expected one fail finish (attempted 3, failed 1), got %+v", finishes)
	}
	if fs := finishes[1].failures; len(fs) != 1 || fs[0].Name != "b" || fs[0].Reason != "boom" {
		t.Fatalf("expected the finish to carry the failed container b: boom, got %+v", finishes[1].failures)
	}
}

func TestScheduledRunNoAggregatorStillRunsEveryItem(t *testing.T) {
	sc := New(nil, nil)
	var called int
	sc.runAggregatedHC("containers", func() (int, int, []ItemFailure) {
		return RunContainersJob(
			[]store.Target{
				{ContainerName: "a", IncludeInSchedule: true},
				{ContainerName: "b", IncludeInSchedule: true},
			},
			func(string) error { called++; return nil },
		)
	})
	if called != 2 {
		t.Fatalf("item loop must still run without an aggregator, called=%d", called)
	}
}

// TestConfigJobScheduledAndExcludedFromDrills checks that the config backup
// registers on its own cadence and gets a local subset drill but no dr drill:
// BombVault's own settings are recovered through a staged restart, so a
// sandbox restore of them proves nothing. VMs in the same settings do get a dr
// task, so the exclusion is specific to config.
func TestConfigJobScheduledAndExcludedFromDrills(t *testing.T) {
	noopBackup := func(string) error { return nil }
	noTargets := func() ([]store.Target, error) { return nil, nil }

	// With every other domain off, only the config backup registers.
	sc := New(noopBackup, noTargets)
	sc.SetConfigJob(func() error { return nil })

	s := store.Settings{ConfigEnabled: true, ConfigSchedule: "daily 03:30"}
	if err := sc.ReloadWithDueChecks(s, nil, nil, nil, nil, nil, nil); err != nil {
		t.Fatalf("ReloadWithDueChecks: %v", err)
	}
	if got := len(sc.entries); got != 1 {
		t.Fatalf("expected exactly 1 registered job (config backup), got %d", got)
	}

	// Turning the cadence off removes the entry, so it was the config job.
	s.ConfigSchedule = "off"
	if err := sc.ReloadWithDueChecks(s, nil, nil, nil, nil, nil, nil); err != nil {
		t.Fatalf("ReloadWithDueChecks (config off): %v", err)
	}
	if got := len(sc.entries); got != 0 {
		t.Fatalf("expected 0 registered jobs when config schedule is off, got %d", got)
	}

	drillCfg := store.Settings{
		ConfigEnabled:        true,
		ConfigSchedule:       "daily 03:30",
		ConfigOffsite:        "rclone:remote:bombvault-config",
		VMsEnabled:           true,
		VMsOffsite:           "rclone:remote:bombvault-vms",
		DrillsEnabled:        true,
		DrillsSchedule:       "weekly Sun 05:00",
		OffsiteDrillsEnabled: true,
	}
	var haveVMDr bool
	for _, tk := range drillTasks(drillCfg) {
		if tk.kind == "dr" && tk.domain == "config" {
			t.Fatal("config must be excluded from DR drills")
		}
		if tk.kind == "dr" && tk.domain == "vms" {
			haveVMDr = true
		}
	}
	if !haveVMDr {
		t.Fatal("expected a {vms, offsite, dr} task under this settings shape; config's exclusion should not depend on vms also being excluded")
	}

	var haveConfigSubset bool
	for _, d := range enabledDrillDomains(drillCfg) {
		if d == "config" {
			haveConfigSubset = true
		}
	}
	if !haveConfigSubset {
		t.Fatal("expected config in the local subset drill domains")
	}
}

// TestEverythingJobScheduledAndSurvivesReload checks that Backup Everything is
// off by default, that a cadence registers one job=backup domain=everything
// entry, that reloading the same cadence keeps exactly one, and that turning
// it off removes it.
func TestEverythingJobScheduledAndSurvivesReload(t *testing.T) {
	noopBackup := func(string) error { return nil }
	noTargets := func() ([]store.Target, error) { return nil, nil }

	sc := New(noopBackup, noTargets)
	sc.SetEverythingJob(func() error { return nil })

	off := store.Settings{}
	if err := sc.ReloadWithDueChecks(off, nil, nil, nil, nil, nil, nil); err != nil {
		t.Fatalf("ReloadWithDueChecks (off): %v", err)
	}
	if got := len(sc.entries); got != 0 {
		t.Fatalf("expected 0 registered jobs with EverythingSchedule off, got %d", got)
	}

	on := store.Settings{EverythingSchedule: "daily 03:00"}
	if err := sc.ReloadWithDueChecks(on, nil, nil, nil, nil, nil, nil); err != nil {
		t.Fatalf("ReloadWithDueChecks (on): %v", err)
	}
	if got := len(sc.entries); got != 1 {
		t.Fatalf("expected exactly 1 registered job (everything), got %d", got)
	}
	if sc.entries[0].job != "backup" || sc.entries[0].domain != "everything" {
		t.Fatalf("expected job=backup domain=everything, got job=%q domain=%q", sc.entries[0].job, sc.entries[0].domain)
	}

	if err := sc.ReloadWithDueChecks(on, nil, nil, nil, nil, nil, nil); err != nil {
		t.Fatalf("ReloadWithDueChecks (on again): %v", err)
	}
	if got := len(sc.entries); got != 1 {
		t.Fatalf("expected the entry to survive a same-cadence reload as exactly 1, got %d", got)
	}

	if err := sc.ReloadWithDueChecks(off, nil, nil, nil, nil, nil, nil); err != nil {
		t.Fatalf("ReloadWithDueChecks (off again): %v", err)
	}
	if got := len(sc.entries); got != 0 {
		t.Fatalf("expected 0 registered jobs when EverythingSchedule is off again, got %d", got)
	}
}

// TestFilesJobScheduledWithOffsiteEntry checks that FilesSchedule registers the
// files backup, FilesOffsiteSchedule adds a separate replication entry, and
// turning both off removes them.
func TestFilesJobScheduledWithOffsiteEntry(t *testing.T) {
	noopBackup := func(string) error { return nil }
	noTargets := func() ([]store.Target, error) { return nil, nil }

	sc := New(noopBackup, noTargets)
	sc.SetFilesJob(
		func(string) error { return nil },
		func() ([]store.FileSet, error) { return nil, nil },
	)

	s := store.Settings{FilesEnabled: true, FilesSchedule: "daily 03:00"}
	if err := sc.ReloadWithDueChecks(s, nil, nil, nil, nil, nil, nil); err != nil {
		t.Fatalf("ReloadWithDueChecks: %v", err)
	}
	if got := len(sc.entries); got != 1 {
		t.Fatalf("expected exactly 1 registered job (files backup), got %d", got)
	}

	s.FilesOffsiteSchedule = "daily 04:00"
	if err := sc.ReloadWithDueChecks(s, nil, nil, nil, nil, nil, nil); err != nil {
		t.Fatalf("ReloadWithDueChecks (files offsite): %v", err)
	}
	if got := len(sc.entries); got != 2 {
		t.Fatalf("expected 2 registered jobs (files backup + files-offsite), got %d", got)
	}

	s.FilesSchedule = "off"
	s.FilesOffsiteSchedule = ""
	if err := sc.ReloadWithDueChecks(s, nil, nil, nil, nil, nil, nil); err != nil {
		t.Fatalf("ReloadWithDueChecks (files off): %v", err)
	}
	if got := len(sc.entries); got != 0 {
		t.Fatalf("expected 0 registered jobs when files schedules are off, got %d", got)
	}
}

// TestFilesDrillTasksLocalSubsetAndOffsiteDR checks that an enabled files domain
// gets the local subset check, plus a dr drill once an off-site repo is set and
// off-site drills are on.
func TestFilesDrillTasksLocalSubsetAndOffsiteDR(t *testing.T) {
	has := func(tasks []drillTask, want drillTask) bool {
		for _, tk := range tasks {
			if tk == want {
				return true
			}
		}
		return false
	}

	base := store.Settings{
		FilesEnabled:   true,
		FilesSchedule:  "daily 03:00",
		DrillsEnabled:  true,
		DrillsSchedule: "weekly Sun 05:00",
	}

	tasks := drillTasks(base)
	if !has(tasks, drillTask{domain: "files", source: "local", kind: "subset"}) {
		t.Fatalf("expected {files, local, subset} drill task, got %v", tasks)
	}
	for _, tk := range tasks {
		if tk.domain == "files" && tk.kind == "dr" {
			t.Fatalf("files must not get a DR task without an off-site repo: %v", tasks)
		}
	}

	withOff := base
	withOff.FilesOffsite = "rclone:remote:bombvault-files"
	withOff.OffsiteDrillsEnabled = true
	tasks = drillTasks(withOff)
	if !has(tasks, drillTask{domain: "files", source: "local", kind: "subset"}) {
		t.Fatalf("expected {files, local, subset} drill task, got %v", tasks)
	}
	if !has(tasks, drillTask{domain: "files", source: "offsite", kind: "dr"}) {
		t.Fatalf("expected {files, offsite, dr} drill task with FilesOffsite set, got %v", tasks)
	}
}

// TestImmutableOffsiteDomainsIncludesConfig checks that the scheduled tamper test
// covers config when its off-site repo is flagged immutable; otherwise an
// append-only config repo would never be verified.
func TestImmutableOffsiteDomainsIncludesConfig(t *testing.T) {
	has := func(list []string, want string) bool {
		for _, d := range list {
			if d == want {
				return true
			}
		}
		return false
	}

	if got := immutableOffsiteDomains(store.Settings{}); has(got, "config") {
		t.Fatalf("config must not be a tamper-test domain when ConfigOffsiteImmutable is unset: %v", got)
	}

	got := immutableOffsiteDomains(store.Settings{
		ContainersOffsiteImmutable: true,
		FlashOffsiteImmutable:      true,
		ConfigOffsiteImmutable:     true,
	})
	if !has(got, "config") {
		t.Fatalf("config must be a tamper-test domain when ConfigOffsiteImmutable is set: %v", got)
	}
	if !has(got, "containers") || !has(got, "flash") {
		t.Fatalf("flagged domains missing: %v", got)
	}
}

// TestImmutableOffsiteDomainsIncludesFiles checks the same for files.
func TestImmutableOffsiteDomainsIncludesFiles(t *testing.T) {
	has := func(list []string, want string) bool {
		for _, d := range list {
			if d == want {
				return true
			}
		}
		return false
	}

	if got := immutableOffsiteDomains(store.Settings{}); has(got, "files") {
		t.Fatalf("files must not be a tamper-test domain when FilesOffsiteImmutable is unset: %v", got)
	}

	got := immutableOffsiteDomains(store.Settings{
		FlashOffsiteImmutable: true,
		FilesOffsiteImmutable: true,
	})
	if !has(got, "files") {
		t.Fatalf("files must be a tamper-test domain when FilesOffsiteImmutable is set: %v", got)
	}
	if !has(got, "flash") {
		t.Fatalf("flagged domains missing: %v", got)
	}
}
