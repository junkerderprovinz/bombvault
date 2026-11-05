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
	states   map[string][]string
	errs     map[string]error
	startErr error
	started  []string
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
	if v.startErr != nil {
		return v.startErr
	}
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

// lateShutdownRig is a VM on record as down, with one finished failed run.
func lateShutdownRig(t *testing.T, states ...string) (*Service, *store.Repo, *stateVirsh, string) {
	t.Helper()
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	st := store.New(db)
	if err := st.MarkVMsStopped([]string{"lab"}, 1); err != nil {
		t.Fatal(err)
	}
	tg, err := st.UpsertVMTarget(store.VMTarget{Name: "lab", Method: "graceful"})
	if err != nil {
		t.Fatal(err)
	}
	runID, err := st.StartRun(tg.ID, "backup")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.FinishRun(runID, "failed", "", 0, "did not shut down in time"); err != nil {
		t.Fatal(err)
	}
	v := &stateVirsh{states: map[string][]string{"lab": states}}
	dir := t.TempDir()
	return NewService(config.Config{AppKey: strings.Repeat("a", 64), DataDir: dir, HostMountRoot: dir}, st, nil, v, nil), st, v, runID
}

func recordedDown(t *testing.T, st *store.Repo) []string {
	t.Helper()
	left, err := st.ListVMsStopped()
	if err != nil {
		t.Fatal(err)
	}
	return left
}

func TestLateShutdownStartsTheVMOnceItIsOff(t *testing.T) {
	svc, st, v, runID := lateShutdownRig(t, "running", "running", "shut off")
	svc.awaitLateShutdown(t.Context(), "lab", runID, time.Millisecond)

	if !slices.Equal(v.started, []string{"lab"}) || len(recordedDown(t, st)) != 0 {
		t.Fatalf("started %v, recorded %v", v.started, recordedDown(t, st))
	}
}

func TestLateShutdownThatCannotStartTheVMSaysSoOnTheRun(t *testing.T) {
	svc, st, v, runID := lateShutdownRig(t, "shut off")
	v.startErr = errors.New("device busy")
	svc.awaitLateShutdown(t.Context(), "lab", runID, time.Millisecond)

	run, err := st.GetRun(runID)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(run.Error, "did not shut down in time") || !strings.Contains(run.Error, "restart vm: device busy") {
		t.Fatalf("run error = %q", run.Error)
	}
	if !slices.Equal(recordedDown(t, st), []string{"lab"}) {
		t.Fatalf("a VM left off has to stay recorded: %v", recordedDown(t, st))
	}
}

func TestLateShutdownOfAGuestThatKeepsRunningForgetsItWithoutStartingIt(t *testing.T) {
	svc, st, v, runID := lateShutdownRig(t, "running")
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Millisecond)
	defer cancel()
	svc.awaitLateShutdown(ctx, "lab", runID, time.Millisecond)

	if len(v.started) != 0 || len(recordedDown(t, st)) != 0 {
		t.Fatalf("started %v, recorded %v", v.started, recordedDown(t, st))
	}
}

func TestLateShutdownLeavesAVMAloneOnceItsRecordIsGone(t *testing.T) {
	svc, st, v, runID := lateShutdownRig(t, "shut off")
	if err := st.ClearVMsStopped([]string{"lab"}); err != nil {
		t.Fatal(err)
	}
	svc.awaitLateShutdown(t.Context(), "lab", runID, time.Millisecond)

	if len(v.started) != 0 {
		t.Fatalf("started %v", v.started)
	}
}

func TestLateShutdownWaitsForABackupThatHoldsTheVMs(t *testing.T) {
	svc, _, v, runID := lateShutdownRig(t, "shut off")
	unlock := svc.lockDomainFor("vms", "backup")
	done := make(chan struct{})
	go func() {
		svc.awaitLateShutdown(context.Background(), "lab", runID, time.Millisecond)
		close(done)
	}()
	select {
	case <-done:
		t.Fatal("it did not wait for the backup")
	case <-time.After(50 * time.Millisecond):
	}
	unlock()
	<-done
	if !slices.Equal(v.started, []string{"lab"}) {
		t.Fatalf("started %v", v.started)
	}
}
