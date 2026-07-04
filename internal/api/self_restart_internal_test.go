package api

import (
	"context"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/dockercli"
)

// selfRestartFakeDocker embeds the Docker interface (left nil) and overrides only
// the two methods ScheduleSelfRestart exercises: Self (own-name resolution) and
// Restart (the recorded call). Any other method would panic on the nil embed, but
// none is reached on these paths — keeping the fake minimal and self-contained in
// package api (the api_test fakeServiceDocker isn't visible here).
type selfRestartFakeDocker struct {
	dockercli.Docker
	selfName  string
	restarted chan string
}

func (f *selfRestartFakeDocker) Self(context.Context) (string, error) { return f.selfName, nil }

func (f *selfRestartFakeDocker) Restart(_ context.Context, name string, _ time.Duration) error {
	if f.restarted != nil {
		f.restarted <- name
	}
	return nil
}

// TestScheduleSelfRestartReturnsFalseWithoutSelfName: when the own-container name
// can't be resolved, no restart is scheduled and the caller is told to restart
// manually (false).
func TestScheduleSelfRestartReturnsFalseWithoutSelfName(t *testing.T) {
	t.Setenv("BOMBVAULT_SELF_CONTAINER", "") // ignore any ambient override; force docker.Self resolution
	svc := &Service{docker: &selfRestartFakeDocker{selfName: ""}}
	if svc.ScheduleSelfRestart() {
		t.Fatal("expected false when self-name is unknown")
	}
}

// TestScheduleSelfRestartInvokesRestart: with a known self-name the restart is
// scheduled (true) and the docker Restart is invoked with that exact name. The
// delay is shrunk so the test observes the call promptly; the channel + timeout
// makes it non-flaky (no sleep-then-assert).
func TestScheduleSelfRestartInvokesRestart(t *testing.T) {
	t.Setenv("BOMBVAULT_SELF_CONTAINER", "") // force resolution via the fake's Self
	fake := &selfRestartFakeDocker{selfName: "BombVault", restarted: make(chan string, 1)}
	svc := &Service{docker: fake}

	orig := selfRestartDelay
	selfRestartDelay = 10 * time.Millisecond
	t.Cleanup(func() { selfRestartDelay = orig })

	if !svc.ScheduleSelfRestart() {
		t.Fatal("expected true when self-name is known")
	}
	select {
	case name := <-fake.restarted:
		if name != "BombVault" {
			t.Fatalf("restarted %q, want BombVault", name)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Restart was not called")
	}
}

// TestStartRestoreConfigRefusesWhenBusy: with the single-flight guard already held
// (a backup/restore in flight), StartRestoreConfig must decline WITHOUT staging or
// scheduling a self-restart — started=false, err=nil — so a config self-restart can
// never kill the container mid-write of another operation. It returns before
// touching the store/docker, so a zero-value Service with the guard pre-set is
// enough to exercise the guard.
func TestStartRestoreConfigRefusesWhenBusy(t *testing.T) {
	s := &Service{}
	s.batchActive.Store(true) // simulate another backup/restore already running

	started, auto, err := s.StartRestoreConfig(context.Background(), "latest", "local")
	if started {
		t.Fatal("expected started=false while another operation holds the guard")
	}
	if err != nil {
		t.Fatalf("expected nil error (busy is not an error), got %v", err)
	}
	if auto {
		t.Fatal("expected autoRestart=false when nothing was started")
	}
	if !s.batchActive.Load() {
		t.Fatal("the pre-existing guard must remain held (StartRestoreConfig must not clear it)")
	}
}
