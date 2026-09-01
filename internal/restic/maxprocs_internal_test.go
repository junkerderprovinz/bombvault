package restic

import (
	"strings"
	"testing"
)

// The CPU cap has to reach the CHILD process, not just be stored ([558], issue
// #189). restic is a Go program: with no GOMAXPROCS it takes every core for
// compression and encryption, which is what pinned a 12-thread box at 99% on
// every core and 100 °C for the length of a backup. authEnv is the single place
// every restic child's environment is built, shared by run() and DumpZip, so
// asserting there covers every invocation.
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

	// 0 means "every core" — restic's own default — so nothing must be exported.
	// An installation that never touches the setting has to behave exactly as it
	// did before this existed.
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

	// Negative is meaningless and must not become a negative GOMAXPROCS, which
	// would make restic refuse to start rather than run slowly.
	SetMaxProcs(-4)
	if MaxProcs() != 0 {
		t.Fatalf("a negative cap must clamp to 0 (= every core), got %d", MaxProcs())
	}
	if got, ok := find(r.authEnv(Mode{})); ok {
		t.Fatalf("a clamped-to-zero cap must export nothing, got %q", got)
	}
}
