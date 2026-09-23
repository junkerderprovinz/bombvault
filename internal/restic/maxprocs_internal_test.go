package restic

import (
	"strings"
	"testing"
)

// TestAuthEnvCarriesTheCPUCap checks that the CPU cap reaches the restic
// child as GOMAXPROCS; without it restic uses every core for compression and
// encryption. authEnv builds the environment of every restic child, run() and DumpZip
// alike, so checking it covers all of them.
func TestAuthEnvCarriesTheCPUCap(t *testing.T) {
	find := func(env []string) (string, bool) {
		for _, e := range env {
			if strings.HasPrefix(e, "GOMAXPROCS=") {
				return e, true
			}
		}
		return "", false
	}

	t.Cleanup(func() { SetMaxProcs(0) })
	r := Restic{Bin: "restic"}

	// 0 means every core, restic's own default, so nothing is exported.
	SetMaxProcs(0)
	if got, ok := find(r.authEnv(Mode{})); ok {
		t.Fatalf("an uncapped engine must export no GOMAXPROCS, got %q", got)
	}

	SetMaxProcs(3)
	got, ok := find(r.authEnv(Mode{}))
	if !ok {
		t.Fatal("a capped engine must export GOMAXPROCS to the child")
	}
	if got != "GOMAXPROCS=3" {
		t.Fatalf("cap must reach the child verbatim, got %q", got)
	}

	// A negative GOMAXPROCS would stop restic from starting at all.
	SetMaxProcs(-4)
	if MaxProcs() != 0 {
		t.Fatalf("a negative cap must clamp to 0 (= every core), got %d", MaxProcs())
	}
	if got, ok := find(r.authEnv(Mode{})); ok {
		t.Fatalf("a clamped-to-zero cap must export nothing, got %q", got)
	}
}
