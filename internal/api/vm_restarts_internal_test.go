package api

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// stateVirsh answers State from a script per VM, the last answer sticking, and
// records every Start.
type stateVirsh struct {
	scriptedVirsh
	states  map[string][]string
	errs    map[string]error
	started []string
}

func (v *stateVirsh) State(_ context.Context, name string) (string, error) {
	if err := v.errs[name]; err != nil {
		return "", err
	}
	seq := v.states[name]
	s := seq[0]
	if len(seq) > 1 {
		v.states[name] = seq[1:]
	}
	return s, nil
}

func (v *stateVirsh) Start(_ context.Context, name string) error {
	v.started = append(v.started, name)
	return nil
}

func TestRestartLeftVMsStartsOnlyTheOnesLeftOff(t *testing.T) {
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	st := store.New(db)
	names := []string{"off", "stopping", "running", "gone", "unreachable", "updating"}
	if err := st.MarkVMsStopped(names, 1); err != nil {
		t.Fatal(err)
	}
	v := &stateVirsh{
		states: map[string][]string{
			"off":      {"shut off"},
			"stopping": {"running", "shut off"},
			"running":  {"running"},
			"gone":     {""},
			"updating": {"in shutdown"},
		},
		errs: map[string]error{"unreachable": errors.New("ssh: connect: no route to host")},
	}
	dir := t.TempDir()
	svc := NewService(config.Config{AppKey: strings.Repeat("a", 64), DataDir: dir, HostMountRoot: dir}, st, nil, v, nil)

	svc.restartLeftVMs(context.Background(), names, 3, time.Millisecond)

	slices.Sort(v.started)
	if !slices.Equal(v.started, []string{"off", "stopping"}) {
		t.Fatalf("started %v, want off and stopping", v.started)
	}
	left, err := st.ListVMsStopped()
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(left, []string{"unreachable", "updating"}) {
		t.Fatalf("still recorded %v, want the VM whose state could not be read and the one still shutting down", left)
	}
}
