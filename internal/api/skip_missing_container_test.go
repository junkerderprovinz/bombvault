package api_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/backup"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// TestBackupSkipsRemovedContainer: a backup of a container that no longer exists on
// the host returns ErrContainerNotInstalled (a skip, not a failure), never drives
// the orchestrator (no stop/start/create side effects), and records exactly one
// "skipped" run per attempt so the dashboard reflects it and agrees with the green
// aggregate Healthchecks ping instead of showing nothing (#57).
func TestBackupSkipsRemovedContainer(t *testing.T) {
	d := &fakeServiceDocker{
		inspectErr: errors.New("Error response from daemon: No such container: Nexterm"),
	}
	svc, st, _ := stackTestService(t, &fakeResticEngine{}, d)

	tg, err := st.UpsertTarget(store.Target{ContainerName: "Nexterm", IncludeInSchedule: true})
	if err != nil {
		t.Fatalf("seed target: %v", err)
	}

	_, err = svc.Backup(context.Background(), "Nexterm")
	if !errors.Is(err, backup.ErrContainerNotInstalled) {
		t.Fatalf("Backup err = %v, want ErrContainerNotInstalled", err)
	}

	// The orchestrator must never run for a removed container.
	for _, c := range d.calls {
		if strings.HasPrefix(c, "stop:") || strings.HasPrefix(c, "start:") || strings.HasPrefix(c, "createAndStart:") {
			t.Fatalf("unexpected orchestrator side effect after skip: %q (calls=%v)", c, d.calls)
		}
	}

	// Exactly one "skipped" run, attributed to the target, carrying a reason.
	runs, err := st.ListRuns(10)
	if err != nil {
		t.Fatalf("ListRuns: %v", err)
	}
	if len(runs) != 1 {
		t.Fatalf("runs = %d, want 1 (%+v)", len(runs), runs)
	}
	if r := runs[0]; r.Status != "skipped" || r.TargetID != tg.ID || !strings.Contains(r.Error, "no longer exists") {
		t.Fatalf("run = %+v, want status=skipped target=%s error~='no longer exists'", r, tg.ID)
	}

	// A second attempt on the still-missing target records another skip: the run row
	// is an honest per-attempt audit trail (only the notification is debounced).
	if _, err := svc.Backup(context.Background(), "Nexterm"); !errors.Is(err, backup.ErrContainerNotInstalled) {
		t.Fatalf("second Backup err = %v, want ErrContainerNotInstalled", err)
	}
	if runs, _ = st.ListRuns(10); len(runs) != 2 {
		t.Fatalf("after second skip, runs = %d, want 2", len(runs))
	}
}
