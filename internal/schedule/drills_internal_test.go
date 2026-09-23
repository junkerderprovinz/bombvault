package schedule

import (
	"testing"

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
// local subset check, plus a DR drill once an off-site repo is set and off-site
// drills are on.
func TestDrillTasksIncludeZFSOffsiteDR(t *testing.T) {
	base := store.Settings{
		ZFSEnabled:           true,
		ZFSSchedule:          "daily 03:00",
		OffsiteDrillsEnabled: true,
	}

	tasks := drillTasks(base)
	if !hasDrillTask(tasks, drillTask{domain: "zfs", source: "local", kind: "subset"}) {
		t.Fatalf("expected a local subset drill for zfs, got %v", tasks)
	}
	for _, tk := range tasks {
		if tk.domain == "zfs" && tk.kind == "dr" {
			t.Fatalf("zfs must not get a DR task without an off-site repo: %v", tasks)
		}
	}

	withOff := base
	withOff.ZFSOffsite = "rclone:remote:bombvault-zfs"
	tasks = drillTasks(withOff)
	if !hasDrillTask(tasks, drillTask{domain: "zfs", source: "offsite", kind: "dr"}) {
		t.Fatalf("expected an off-site DR drill for zfs, got %v", tasks)
	}

	optedOut := withOff
	optedOut.OffsiteDrillsEnabled = false
	for _, tk := range drillTasks(optedOut) {
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

// TestDrillTasks checks the scheduled drills: a local "subset" integrity check
// per enabled domain, plus an off-site "dr" drill for containers, VMs and flash
// when off-site is configured.
func TestDrillTasks(t *testing.T) {
	base := store.Settings{
		ContainersEnabled:    true,
		VMsEnabled:           true,
		FlashEnabled:         true,
		OffsiteDrillsEnabled: true, // the default
	}

	t.Run("subset per enabled domain, no dr without off-site", func(t *testing.T) {
		got := drillTasks(base)
		subset := map[string]bool{}
		for _, tk := range got {
			if tk.kind == "dr" {
				t.Fatalf("no dr task expected without an off-site repo, got %+v", tk)
			}
			if tk.kind == "subset" && tk.source == "local" {
				subset[tk.domain] = true
			}
		}
		for _, d := range []string{"containers", "vms", "flash"} {
			if !subset[d] {
				t.Fatalf("expected a local subset drill for %q, got %+v", d, got)
			}
		}
	})

	t.Run("off-site dr for containers + vms + flash", func(t *testing.T) {
		s := base
		s.ContainersOffsite = "rest:http://192.168.20.9:8000/containers"
		s.FlashOffsite = "rest:http://192.168.20.9:8000/flash"
		s.VMsOffsite = "rest:http://192.168.20.9:8000/vms"
		dr := map[string]bool{}
		for _, tk := range drillTasks(s) {
			if tk.kind != "dr" {
				continue
			}
			if tk.source != "offsite" {
				t.Fatalf("a dr task must be off-site, got %+v", tk)
			}
			dr[tk.domain] = true
		}
		if !dr["containers"] || !dr["flash"] || !dr["vms"] {
			t.Fatalf("containers + flash + vms must each get an off-site dr drill, got %+v", dr)
		}
	})

	t.Run("a disabled domain gets neither subset nor dr", func(t *testing.T) {
		s := base
		s.ContainersEnabled = false
		s.ContainersOffsite = "rest:http://192.168.20.9:8000/containers" // off-site set but domain off
		for _, tk := range drillTasks(s) {
			if tk.domain == "containers" {
				t.Fatalf("a disabled domain must yield no drill task, got %+v", tk)
			}
		}
	})

	t.Run("OffsiteDrillsEnabled false omits dr tasks but keeps local subset", func(t *testing.T) {
		s := base
		s.OffsiteDrillsEnabled = false
		s.ContainersOffsite = "rest:http://192.168.20.9:8000/containers"
		s.FlashOffsite = "rest:http://192.168.20.9:8000/flash"
		subset := map[string]bool{}
		for _, tk := range drillTasks(s) {
			if tk.kind == "dr" {
				t.Fatalf("no off-site dr task expected when OffsiteDrillsEnabled is false, got %+v", tk)
			}
			if tk.kind == "subset" && tk.source == "local" {
				subset[tk.domain] = true
			}
		}
		for _, d := range []string{"containers", "vms", "flash"} {
			if !subset[d] {
				t.Fatalf("local subset drill for %q must remain when off-site DR is opted out", d)
			}
		}
	})
}
