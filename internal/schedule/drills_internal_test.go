package schedule

import (
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

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
