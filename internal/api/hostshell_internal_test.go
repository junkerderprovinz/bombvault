package api

import (
	"context"
	"errors"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/config"
)

// fakeHostShell is a minimal configurable HostShell fake for the "Backup
// Everything" global hook seam's wiring test (Task 2): it records every
// command it was asked to run, in order, and can be told to fail. It
// deliberately never shells out to a real /bin/sh — the CI runner and this
// dev environment may not both be POSIX, and the point of this test is the
// interface/wiring, not os/exec itself.
type fakeHostShell struct {
	calls []string // every cmd Run was asked to run, in order
	err   error    // returned by every Run call
}

var _ HostShell = (*fakeHostShell)(nil)

func (f *fakeHostShell) Run(_ context.Context, cmd string) error {
	f.calls = append(f.calls, cmd)
	return f.err
}

// TestNewServiceDefaultsHostShell proves NewService wires a non-nil real
// execHostShell adapter by default, so any production Service — even one
// main.go never calls SetHostShell on — has a working HostShell.
func TestNewServiceDefaultsHostShell(t *testing.T) {
	svc := NewService(config.Config{}, nil, nil, nil, nil)
	if svc.hostShell == nil {
		t.Fatal("NewService must default hostShell to a non-nil adapter")
	}
	if _, ok := svc.hostShell.(execHostShell); !ok {
		t.Fatalf("NewService's default hostShell = %#v, want execHostShell", svc.hostShell)
	}
}

// TestSetHostShellOverridesDefault proves SetHostShell replaces the default
// adapter with an injected fake, mirroring SetHostSSH/SetProgress's own
// override contract — the seam later "Backup Everything" tests use to assert
// hook call count/ordering/best-effort failure handling without ever
// invoking a real shell.
func TestSetHostShellOverridesDefault(t *testing.T) {
	svc := NewService(config.Config{}, nil, nil, nil, nil)

	fake := &fakeHostShell{err: errors.New("boom")}
	svc.SetHostShell(fake)

	got, ok := svc.hostShell.(*fakeHostShell)
	if !ok || got != fake {
		t.Fatalf("SetHostShell did not override hostShell: got %#v", svc.hostShell)
	}

	// The fake records what it was asked to run and can be told to fail —
	// exactly the fake HostShell contract this task's test requires.
	err := svc.hostShell.Run(context.Background(), "curl -fsS https://example.invalid/ping")
	if err == nil {
		t.Fatal("fake HostShell.Run must return the configured error")
	}
	if len(fake.calls) != 1 || fake.calls[0] != "curl -fsS https://example.invalid/ping" {
		t.Fatalf("fake HostShell did not record the run command: %#v", fake.calls)
	}
}
