package restic

import (
	"strings"
	"testing"
)

func memLimitEnv(env []string) (string, bool) {
	got, found := "", false
	for _, e := range env {
		if strings.HasPrefix(e, "GOMEMLIMIT=") {
			got, found = e, true
		}
	}
	return got, found
}

func TestAuthEnvCarriesTheMemoryLimit(t *testing.T) {
	t.Cleanup(func() { SetMemLimitMB(0) })
	t.Setenv("GOMEMLIMIT", "")
	r := Restic{Bin: "restic"}

	SetMemLimitMB(0)
	if got, _ := memLimitEnv(r.authEnv(Mode{})); got != "GOMEMLIMIT=" {
		t.Fatalf("no limit must export nothing of its own, got %q", got)
	}

	SetMemLimitMB(512)
	if got, _ := memLimitEnv(r.authEnv(Mode{})); got != "GOMEMLIMIT=512MiB" {
		t.Fatalf("the limit must reach the child in MiB, got %q", got)
	}
}

func TestMemoryLimitBeatsTheAmbientEnvironment(t *testing.T) {
	t.Cleanup(func() { SetMemLimitMB(0) })
	t.Setenv("GOMEMLIMIT", "64MiB")
	r := Restic{Bin: "restic"}

	SetMemLimitMB(0)
	if got, _ := memLimitEnv(r.authEnv(Mode{})); got != "GOMEMLIMIT=64MiB" {
		t.Fatalf("without a setting the container's own limit stays, got %q", got)
	}

	SetMemLimitMB(256)
	if got, _ := memLimitEnv(r.authEnv(Mode{})); got != "GOMEMLIMIT=256MiB" {
		t.Fatalf("the setting must override the container's limit, got %q", got)
	}
}

func TestNegativeMemoryLimitSetsNone(t *testing.T) {
	t.Cleanup(func() { SetMemLimitMB(0) })
	SetMemLimitMB(-5)
	if memLimitMB.Load() != 0 {
		t.Fatalf("a negative limit must clamp to 0, got %d", memLimitMB.Load())
	}
}
