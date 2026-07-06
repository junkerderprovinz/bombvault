package schedule

import (
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

// TestDrillTasks pins the scheduled-drill wiring: a local "subset" integrity check
// per enabled domain, plus a real off-site "dr" drill for containers + flash when
// off-site is configured — and NEVER a dr drill for VMs.
func TestDrillTasks(t *testing.T) {
	base := store.Settings{
		ContainersEnabled:    true,
		VMsEnabled:           true,
		FlashEnabled:         true,
		OffsiteDrillsEnabled: true, // default on: scheduled off-site DR drills run
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

	t.Run("off-site dr for containers + flash, never vms", func(t *testing.T) {
		s := base
		s.ContainersOffsite = "rest:http://192.168.20.9:8000/containers"
		s.FlashOffsite = "rest:http://192.168.20.9:8000/flash"
		s.VMsOffsite = "rest:http://192.168.20.9:8000/vms" // even with off-site, VMs get NO dr drill
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
		if !dr["containers"] || !dr["flash"] {
			t.Fatalf("containers + flash must each get an off-site dr drill, got %+v", dr)
		}
		if dr["vms"] {
			t.Fatal("VMs must NEVER get a dr drill")
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

	// #37: opting out of the scheduled off-site DR drill drops the {*,offsite,dr}
	// tasks but KEEPS the local {*,subset} integrity checks for every enabled domain.
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
