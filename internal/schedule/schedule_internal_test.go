package schedule

import (
	"errors"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

// TestScheduledRunAggregatesHealthchecksPings verifies the scheduled per-domain run
// wraps its item loop in exactly ONE Healthchecks start + ONE finish, that the finish
// carries the run's attempted/failed counts (success when nothing failed, a failure
// count when any item did) — the core of #49 — and that the finish also carries the
// per-item failures (name + reason) so the summary notification can enumerate them (#64).
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

	// All three succeed → one aggregate start, one success finish, no failures listed.
	sc.runAggregatedHC("containers", func() (int, int, []ItemFailure) {
		return RunContainersJob(targets, func(string) error { return nil })
	})
	if len(starts) != 1 || starts[0] != "containers" {
		t.Fatalf("expected exactly one aggregate start for the run, got %v", starts)
	}
	if len(finishes) != 1 || finishes[0].attempted != 3 || finishes[0].failed != 0 || len(finishes[0].failures) != 0 {
		t.Fatalf("expected one success finish (attempted 3, failed 0, no failures), got %+v", finishes)
	}

	// One item fails → still exactly one finish, now reporting the failure count AND
	// the failing container's name and reason.
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

// TestScheduledRunNoAggregatorStillRunsEveryItem: without an aggregator wired the run
// still backs up every scheduled item (backwards-compatible) and simply sends no
// aggregate pings — the container-only callers and tests are unaffected.
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

// TestConfigJobScheduledAndExcludedFromDrills verifies the config self-backup
// domain is wired end to end: (a) a config backup job registers when it has a
// real cadence, and (b) config is excluded from DR (off-site sandbox) drills the
// same way VMs are — it still gets the local "subset" integrity check, but never
// a "dr" task (runDRDrill has no config arm, exactly as it refuses VMs).
//
// This is a white-box (package schedule) test because the observables are the
// unexported entry list and the drill-task builder; the black-box tests in
// schedule_test.go cannot reach them.
func TestConfigJobScheduledAndExcludedFromDrills(t *testing.T) {
	noopBackup := func(string) error { return nil }
	noTargets := func() ([]store.Target, error) { return nil, nil }

	// (a) A config job is registered when config has a real cadence. With every
	// other domain off (zero-value schedules parse to "off"), drills off, and no
	// immutable off-site, exactly one entry must register — the config backup job.
	sc := New(noopBackup, noTargets)
	sc.SetConfigJob(func() error { return nil })

	s := store.Settings{ConfigEnabled: true, ConfigSchedule: "daily 03:30"}
	if err := sc.ReloadWithDueChecks(s, nil, nil, nil, nil, nil); err != nil {
		t.Fatalf("ReloadWithDueChecks: %v", err)
	}
	if got := len(sc.entries); got != 1 {
		t.Fatalf("expected exactly 1 registered job (config backup), got %d", got)
	}

	// Turning the config cadence off must deregister it — proving the single
	// entry above was driven by ConfigSchedule (i.e. it is the config job).
	s.ConfigSchedule = "off"
	if err := sc.ReloadWithDueChecks(s, nil, nil, nil, nil, nil); err != nil {
		t.Fatalf("ReloadWithDueChecks (config off): %v", err)
	}
	if got := len(sc.entries); got != 0 {
		t.Fatalf("expected 0 registered jobs when config schedule is off, got %d", got)
	}

	// (b) config must NOT appear as a DR ("dr") drill task, even with an off-site
	// repo configured and drills enabled — a sandbox restore of the settings DB is
	// meaningless (same exclusion VMs get). It still gets the local subset check.
	drillCfg := store.Settings{
		ConfigEnabled:  true,
		ConfigSchedule: "daily 03:30",
		ConfigOffsite:  "rclone:remote:bombvault-config",
		VMsEnabled:     true,
		VMsOffsite:     "rclone:remote:bombvault-vms",
		DrillsEnabled:  true,
		DrillsSchedule: "weekly Sun 05:00",
	}
	for _, tk := range drillTasks(drillCfg) {
		if tk.kind == "dr" && tk.domain == "config" {
			t.Fatal("config must be excluded from DR drills (like VMs)")
		}
		if tk.kind == "dr" && tk.domain == "vms" {
			t.Fatal("vms must be excluded from DR drills — baseline for the config exclusion")
		}
	}

	// config (like VMs) still gets the local subset integrity check, so it must be
	// in the enabled-drill-domain list. runSubsetDrill supports the config domain.
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

// TestFilesJobScheduledWithOffsiteEntry verifies the files domain is wired into
// the scheduler like the other domains: (a) a files backup job registers when
// FilesSchedule has a real cadence, (b) FilesOffsiteSchedule registers a separate
// files-offsite replication entry, and (c) turning both off deregisters them.
//
// White-box (package schedule) because the observable is the unexported entry
// list — same rationale as the config-domain test above.
func TestFilesJobScheduledWithOffsiteEntry(t *testing.T) {
	noopBackup := func(string) error { return nil }
	noTargets := func() ([]store.Target, error) { return nil, nil }

	sc := New(noopBackup, noTargets)
	sc.SetFilesJob(
		func(string) error { return nil },
		func() ([]store.FileSet, error) { return nil, nil },
	)

	// (a) With every other domain off (zero-value schedules parse to "off"),
	// drills off, and no immutable off-site, exactly one entry must register —
	// the files backup job.
	s := store.Settings{FilesEnabled: true, FilesSchedule: "daily 03:00"}
	if err := sc.ReloadWithDueChecks(s, nil, nil, nil, nil, nil); err != nil {
		t.Fatalf("ReloadWithDueChecks: %v", err)
	}
	if got := len(sc.entries); got != 1 {
		t.Fatalf("expected exactly 1 registered job (files backup), got %d", got)
	}

	// (b) An off-site cadence adds the files-offsite replication entry.
	s.FilesOffsiteSchedule = "daily 04:00"
	if err := sc.ReloadWithDueChecks(s, nil, nil, nil, nil, nil); err != nil {
		t.Fatalf("ReloadWithDueChecks (files offsite): %v", err)
	}
	if got := len(sc.entries); got != 2 {
		t.Fatalf("expected 2 registered jobs (files backup + files-offsite), got %d", got)
	}

	// (c) Turning both cadences off must deregister them — proving the entries
	// above were driven by FilesSchedule/FilesOffsiteSchedule.
	s.FilesSchedule = "off"
	s.FilesOffsiteSchedule = ""
	if err := sc.ReloadWithDueChecks(s, nil, nil, nil, nil, nil); err != nil {
		t.Fatalf("ReloadWithDueChecks (files off): %v", err)
	}
	if got := len(sc.entries); got != 0 {
		t.Fatalf("expected 0 registered jobs when files schedules are off, got %d", got)
	}
}

// TestFilesDrillTasksLocalSubsetAndOffsiteDR verifies files participates in
// scheduled drills like flash: an enabled files domain always gets the local
// "subset" integrity check, and — unlike VMs/config — it is DR-capable, so an
// off-site repo (with off-site drills enabled) adds a real {files, offsite, dr}
// task.
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

	// No off-site repo: local subset check only, never a DR task.
	tasks := drillTasks(base)
	if !has(tasks, drillTask{domain: "files", source: "local", kind: "subset"}) {
		t.Fatalf("expected {files, local, subset} drill task, got %v", tasks)
	}
	for _, tk := range tasks {
		if tk.domain == "files" && tk.kind == "dr" {
			t.Fatalf("files must not get a DR task without an off-site repo: %v", tasks)
		}
	}

	// Off-site configured + off-site drills enabled: the DR task appears.
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

// TestImmutableOffsiteDomainsIncludesConfig verifies the scheduled off-site tamper
// test covers the config domain when its off-site repo is flagged immutable — the
// same way containers/vms/flash are. Without this, a config repo advertised as
// append-only would never be actively verified.
func TestImmutableOffsiteDomainsIncludesConfig(t *testing.T) {
	has := func(list []string, want string) bool {
		for _, d := range list {
			if d == want {
				return true
			}
		}
		return false
	}

	// Not flagged → config must be absent.
	if got := immutableOffsiteDomains(store.Settings{}); has(got, "config") {
		t.Fatalf("config must not be a tamper-test domain when ConfigOffsiteImmutable is unset: %v", got)
	}

	// Flagged → config must be present, alongside the other flagged domains.
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

// TestImmutableOffsiteDomainsIncludesFiles verifies the scheduled off-site tamper
// test covers the files domain when its off-site repo is flagged immutable — the
// same coverage the other domains get.
func TestImmutableOffsiteDomainsIncludesFiles(t *testing.T) {
	has := func(list []string, want string) bool {
		for _, d := range list {
			if d == want {
				return true
			}
		}
		return false
	}

	// Not flagged → files must be absent.
	if got := immutableOffsiteDomains(store.Settings{}); has(got, "files") {
		t.Fatalf("files must not be a tamper-test domain when FilesOffsiteImmutable is unset: %v", got)
	}

	// Flagged → files must be present, alongside the other flagged domains.
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
