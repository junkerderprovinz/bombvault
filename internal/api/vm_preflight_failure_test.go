package api_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
	"github.com/junkerderprovinz/bombvault/internal/virshcli"
)

// failedRunFor returns the newest failed backup run of targetID.
func failedRunFor(t *testing.T, st *store.Repo, targetID string) (store.Run, bool) {
	t.Helper()
	runs, err := st.ListRuns(20)
	if err != nil {
		t.Fatalf("ListRuns: %v", err)
	}
	for _, r := range runs {
		if r.TargetID == targetID && r.Kind == "backup" && r.Status == "failed" {
			return r, true
		}
	}
	return store.Run{}, false
}

// A VM backup that gives up before the orchestrator takes over still records a
// failed run, so the card that started it stops waiting and the reason reaches
// the error panel.
func TestBackupVMRecordsAFailureFromBeforeTheRun(t *testing.T) {
	st := newMemStore(t)
	tg, err := st.UpsertVMTarget(store.VMTarget{Name: "win11", Method: "graceful"})
	if err != nil {
		t.Fatal(err)
	}
	v := &renameSuggestVirsh{
		vms:       []virshcli.VMInfo{{Name: "win11", State: "running"}},
		errByName: map[string]error{"win11": errors.New("disk \"/var/lib/libvirt/images/data.qcow2\" is not under the host mount")},
	}
	svc := vmTimesService(t, st, v, &fakeResticEngine{})

	if _, err := svc.BackupVM(context.Background(), "win11"); err == nil {
		t.Fatal("BackupVM: want an error from the pre-flight")
	}
	run, ok := failedRunFor(t, st, tg.ID)
	if !ok {
		t.Fatal("want a failed run for the VM, found none")
	}
	if !strings.Contains(run.Error, "not under the host mount") {
		t.Fatalf("run.Error = %q, want the reason the backup gave up", run.Error)
	}
}

// A VM the host no longer defines is a skip, not a failure, so it records
// nothing.
func TestBackupVMRecordsNothingForAVMTheHostNoLongerDefines(t *testing.T) {
	st := newMemStore(t)
	tg, err := st.UpsertVMTarget(store.VMTarget{Name: "win11", Method: "graceful"})
	if err != nil {
		t.Fatal(err)
	}
	v := &renameSuggestVirsh{
		vms:       []virshcli.VMInfo{{Name: "win11", State: "running"}},
		errByName: map[string]error{"win11": errors.New("failed to get domain 'win11'")},
	}
	svc := vmTimesService(t, st, v, &fakeResticEngine{})

	if _, err := svc.BackupVM(context.Background(), "win11"); err == nil {
		t.Fatal("BackupVM: want the not-installed sentinel")
	}
	if _, ok := failedRunFor(t, st, tg.ID); ok {
		t.Fatal("a VM that is not defined any more must not be recorded as failed")
	}
}
