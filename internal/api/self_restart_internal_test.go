package api

import (
	"context"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/dockercli"
)

// selfRestartFakeDocker implements only the two methods ScheduleSelfRestart
// calls; any other method panics on the nil embedded interface.
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

// Without the own container name nothing is scheduled, and false tells the
// caller to restart manually.
func TestScheduleSelfRestartReturnsFalseWithoutSelfName(t *testing.T) {
	t.Setenv("BOMBVAULT_SELF_CONTAINER", "")
	svc := &Service{docker: &selfRestartFakeDocker{selfName: ""}}
	if svc.ScheduleSelfRestart() {
		t.Fatal("expected false when self-name is unknown")
	}
}

func TestScheduleSelfRestartInvokesRestart(t *testing.T) {
	t.Setenv("BOMBVAULT_SELF_CONTAINER", "") // resolve the name through the fake's Self
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

// While another backup or restore holds the guard, StartRestoreConfig declines
// without an error, so its self-restart cannot kill that operation mid-write. It
// returns before touching the store or docker, so a zero Service is enough.
func TestStartRestoreConfigRefusesWhenBusy(t *testing.T) {
	s := &Service{}
	s.batchActive.Store(true)

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
