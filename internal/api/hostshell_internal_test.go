package api

import (
	"context"
	"errors"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/config"
)

// fakeHostShell records the commands it is asked to run and returns err from
// every call.
type fakeHostShell struct {
	calls []string
	err   error
}

var _ HostShell = (*fakeHostShell)(nil)

func (f *fakeHostShell) Run(_ context.Context, cmd string) error {
	f.calls = append(f.calls, cmd)
	return f.err
}

func TestNewServiceDefaultsHostShell(t *testing.T) {
	svc := NewService(config.Config{}, nil, nil, nil, nil)
	if svc.hostShell == nil {
		t.Fatal("NewService must default hostShell to a non-nil adapter")
	}
	if _, ok := svc.hostShell.(execHostShell); !ok {
		t.Fatalf("NewService's default hostShell = %#v, want execHostShell", svc.hostShell)
	}
}

func TestSetHostShellOverridesDefault(t *testing.T) {
	svc := NewService(config.Config{}, nil, nil, nil, nil)

	fake := &fakeHostShell{err: errors.New("boom")}
	svc.SetHostShell(fake)

	got, ok := svc.hostShell.(*fakeHostShell)
	if !ok || got != fake {
		t.Fatalf("SetHostShell did not override hostShell: got %#v", svc.hostShell)
	}

	err := svc.hostShell.Run(context.Background(), "curl -fsS https://example.invalid/ping")
	if err == nil {
		t.Fatal("fake HostShell.Run must return the configured error")
	}
	if len(fake.calls) != 1 || fake.calls[0] != "curl -fsS https://example.invalid/ping" {
		t.Fatalf("fake HostShell did not record the run command: %#v", fake.calls)
	}
}
