package schedule

import (
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

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
	if err := sc.ReloadWithDueChecks(s, nil, nil, nil, nil); err != nil {
		t.Fatalf("ReloadWithDueChecks: %v", err)
	}
	if got := len(sc.entryIDs); got != 1 {
		t.Fatalf("expected exactly 1 registered job (config backup), got %d", got)
	}

	// Turning the config cadence off must deregister it — proving the single
	// entry above was driven by ConfigSchedule (i.e. it is the config job).
	s.ConfigSchedule = "off"
	if err := sc.ReloadWithDueChecks(s, nil, nil, nil, nil); err != nil {
		t.Fatalf("ReloadWithDueChecks (config off): %v", err)
	}
	if got := len(sc.entryIDs); got != 0 {
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
