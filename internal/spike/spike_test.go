package spike_test

import (
	"errors"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/spike"
)

// ---------------------------------------------------------------------------
// Helper probes for injection
// ---------------------------------------------------------------------------

func alwaysOK(_ spike.Deps) (string, error)   { return "ok", nil }
func alwaysFail(_ spike.Deps) (string, error) { return "", errors.New("probe failed") }

// ---------------------------------------------------------------------------
// Run tests
// ---------------------------------------------------------------------------

func TestRunAllPassReturnsAllOK(t *testing.T) {
	probes := []spike.Probe{
		{Name: "docker", Fn: alwaysOK},
		{Name: "restic", Fn: alwaysOK},
		{Name: "qemu-img", Fn: alwaysOK},
	}
	checks, allOK := spike.Run(spike.Deps{}, probes)
	if !allOK {
		t.Fatal("expected AllOK=true when all probes pass")
	}
	if len(checks) != 3 {
		t.Fatalf("expected 3 checks, got %d", len(checks))
	}
	for _, c := range checks {
		if !c.OK {
			t.Fatalf("check %q should be OK", c.Name)
		}
	}
}

func TestRunOneFailYieldsNotAllOK(t *testing.T) {
	probes := []spike.Probe{
		{Name: "docker", Fn: alwaysOK},
		{Name: "restic", Fn: alwaysFail},
		{Name: "rclone", Fn: alwaysOK},
	}
	checks, allOK := spike.Run(spike.Deps{}, probes)
	if allOK {
		t.Fatal("expected AllOK=false when a probe fails")
	}
	if len(checks) != 3 {
		t.Fatalf("expected 3 checks, got %d", len(checks))
	}

	var failCheck spike.Check
	for _, c := range checks {
		if c.Name == "restic" {
			failCheck = c
		}
	}
	if failCheck.OK {
		t.Fatal("restic check must be !OK")
	}
	if failCheck.Detail == "" {
		t.Fatal("failing check must carry a Detail message")
	}
}

func TestRunFailingProbeDoesNotPanic(t *testing.T) {
	// A probe that panics must be recovered gracefully.
	panicProbe := func(_ spike.Deps) (string, error) {
		panic("unexpected panic in probe")
	}
	probes := []spike.Probe{
		{Name: "panic-probe", Fn: panicProbe},
	}

	// Must not panic the test process.
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Run let a probe panic escape: %v", r)
		}
	}()

	checks, allOK := spike.Run(spike.Deps{}, probes)
	if allOK {
		t.Fatal("a panicking probe must yield AllOK=false")
	}
	if len(checks) != 1 || checks[0].OK {
		t.Fatalf("expected one failed check, got %v", checks)
	}
}

func TestRunEmptyProbesReturnsAllOK(t *testing.T) {
	_, allOK := spike.Run(spike.Deps{}, nil)
	if !allOK {
		t.Fatal("no probes → AllOK must be true")
	}
}

// TestRunBestEffortFailDoesNotGateAllOK verifies that a failing best-effort
// probe (e.g. libvirt / qemu-img / rclone) is recorded as OK=false but does
// not lower AllOK when all gating probes pass.
func TestRunBestEffortFailDoesNotGateAllOK(t *testing.T) {
	probes := []spike.Probe{
		{Name: "docker", Fn: alwaysOK},                       // gating
		{Name: "restic", Fn: alwaysOK},                       // gating
		{Name: "libvirt", Fn: alwaysFail, BestEffort: true},  // optional
		{Name: "qemu-img", Fn: alwaysFail, BestEffort: true}, // optional
		{Name: "path-writable", Fn: alwaysOK},                // gating
	}
	checks, allOK := spike.Run(spike.Deps{}, probes)
	if !allOK {
		t.Fatal("expected AllOK=true: failing best-effort probes must not gate health")
	}
	if len(checks) != 5 {
		t.Fatalf("expected 5 checks, got %d", len(checks))
	}
	// Verify the best-effort failures are still reported as !OK.
	for _, c := range checks {
		if (c.Name == "libvirt" || c.Name == "qemu-img") && c.OK {
			t.Fatalf("best-effort probe %q must be reported as OK=false", c.Name)
		}
		if !c.BestEffort && !c.OK {
			t.Fatalf("gating probe %q must be OK=true in this scenario", c.Name)
		}
	}
}

// TestRunGatingFailStillLowersAllOK confirms that a non-best-effort probe
// failure does lower AllOK even when best-effort probes also pass.
func TestRunGatingFailStillLowersAllOK(t *testing.T) {
	probes := []spike.Probe{
		{Name: "docker", Fn: alwaysFail},                  // gating — fails
		{Name: "libvirt", Fn: alwaysOK, BestEffort: true}, // optional — passes
	}
	_, allOK := spike.Run(spike.Deps{}, probes)
	if allOK {
		t.Fatal("expected AllOK=false: a failing gating probe must lower AllOK")
	}
}

func TestRunNameAndDetailPopulated(t *testing.T) {
	probes := []spike.Probe{
		{Name: "my-probe", Fn: func(_ spike.Deps) (string, error) { return "detail-text", nil }},
	}
	checks, _ := spike.Run(spike.Deps{}, probes)
	if checks[0].Name != "my-probe" {
		t.Fatalf("expected name 'my-probe', got %q", checks[0].Name)
	}
	if checks[0].Detail != "detail-text" {
		t.Fatalf("expected detail 'detail-text', got %q", checks[0].Detail)
	}
}

// ---------------------------------------------------------------------------
// DefaultProbes smoke test — just ensure they construct without panic.
// ---------------------------------------------------------------------------

func TestDefaultProbesConstruct(t *testing.T) {
	probes := spike.DefaultProbes()
	if len(probes) == 0 {
		t.Fatal("DefaultProbes must return at least one probe")
	}
	for _, p := range probes {
		if p.Name == "" {
			t.Fatal("every probe must have a Name")
		}
		if p.Fn == nil {
			t.Fatalf("probe %q has nil Fn", p.Name)
		}
	}
}
