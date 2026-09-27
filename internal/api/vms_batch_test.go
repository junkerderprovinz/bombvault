package api_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/api"
	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/progress"
	"github.com/junkerderprovinz/bombvault/internal/store"
	"github.com/junkerderprovinz/bombvault/internal/virshcli"
)

// vmBatchService wires the vms domain with two VMs the operator backs up in the
// order alpha, bravo. The flash domain is wired as well so a test can hold the
// shared backup guard with another domain's backup.
func vmBatchService(t *testing.T, v virshcli.Virsh) (*api.Service, *fakeResticEngine, *store.Repo, <-chan progress.Event) {
	t.Helper()
	dir := t.TempDir()
	root := filepath.ToSlash(t.TempDir())
	flashDir := root + "/boot"
	if err := os.MkdirAll(flashDir, 0o750); err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{
		AppKey:         strings.Repeat("a", 64),
		DataDir:        dir,
		HostMountRoot:  root,
		HostSourceRoot: "/mnt",
		FlashDir:       flashDir,
	}
	st := newMemStore(t)
	settings := mustSettings(t, st)
	settings.EncryptionEnabled = false
	settings.VMsPath = "backups/vms"
	settings.FlashPath = "backups/flash"
	settings.RetentionKeepLast = 5
	if err := st.UpdateSettings(settings); err != nil {
		t.Fatal(err)
	}
	// Seeded repositories so EnsureRepo passes and the batch's prune finds
	// something to prune.
	for _, domain := range []string{"vms", "flash"} {
		repo := filepath.Join(root, "backups", domain)
		if err := os.MkdirAll(repo, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(repo, "config"), []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"alpha", "bravo"} {
		if _, err := st.UpsertVMTarget(store.VMTarget{Name: name, IncludeInSchedule: true}); err != nil {
			t.Fatal(err)
		}
	}
	if err := st.SetVMBackupOrders([]store.VMOrder{{VM: "alpha", Order: 1}, {VM: "bravo", Order: 2}}); err != nil {
		t.Fatal(err)
	}
	eng := &fakeResticEngine{}
	svc := api.NewService(cfg, st, &fakeServiceDocker{}, v, eng)
	svc.SetHostSSH(&zvolTPMSSH{})
	prog := progress.NewStore()
	svc.SetProgress(prog)
	ch, cancel := prog.Subscribe()
	t.Cleanup(cancel)
	return svc, eng, st, ch
}

// waitForVMBatch drains progress events until the terminal "batch:vms" event.
// That receive happens after every backup the batch ran, so the fakes can be
// read once it returns.
func waitForVMBatch(t *testing.T, ch <-chan progress.Event) {
	t.Helper()
	deadline := time.After(5 * time.Second)
	for {
		select {
		case ev := <-ch:
			if ev.Key == "batch:vms" && !ev.Active {
				return
			}
		case <-deadline:
			t.Fatal("timed out waiting for the vms batch to publish its terminal event")
		}
	}
}

func TestStartBackupVMsAllRunsInOperatorOrderAndPrunesOnce(t *testing.T) {
	svc, eng, _, ch := vmBatchService(t, zvolTPMVirsh{domainXML: fileOnlyVMDomainXML})

	started, err := svc.StartBackupVMsAll(context.Background(), []string{"bravo", "alpha"})
	if err != nil || !started {
		t.Fatalf("StartBackupVMsAll: started=%v err=%v", started, err)
	}
	waitForVMBatch(t, ch)
	waitForBackupDone(t, svc)

	if got := strings.Join(eng.forgetTags, " "); got != "vm:alpha vm:bravo" {
		t.Fatalf("backed up in the order %q, want the operator's order \"vm:alpha vm:bravo\"", got)
	}
	if len(eng.manualPruned) != 1 {
		t.Fatalf("%d prunes for the whole batch, want one: %v", len(eng.manualPruned), eng.manualPruned)
	}
}

func TestStartBackupVMsAllSkipsVMsNoLongerDefined(t *testing.T) {
	v := undefinedVMVirsh{zvolTPMVirsh: zvolTPMVirsh{domainXML: fileOnlyVMDomainXML}, undefined: "alpha"}
	svc, eng, st, ch := vmBatchService(t, v)

	started, err := svc.StartBackupVMsAll(context.Background(), []string{"alpha", "bravo"})
	if err != nil || !started {
		t.Fatalf("StartBackupVMsAll: started=%v err=%v", started, err)
	}
	waitForVMBatch(t, ch)
	waitForBackupDone(t, svc)

	alpha, err := st.GetVMTargetByName("alpha")
	if err != nil {
		t.Fatal(err)
	}
	run, err := st.LastRunForTarget(alpha.ID)
	if err != nil {
		t.Fatal(err)
	}
	if run != nil {
		t.Fatalf("a VM the host no longer defines must record no run, got %+v", run)
	}
	if len(eng.forgetTags) != 1 || eng.forgetTags[0] != "vm:bravo" {
		t.Fatalf("backups = %v, want bravo alone behind the skipped VM", eng.forgetTags)
	}
}

func TestStartBackupVMsAllRefusesWhileBatchActive(t *testing.T) {
	svc, eng, _, _ := vmBatchService(t, zvolTPMVirsh{domainXML: fileOnlyVMDomainXML})
	eng.block = make(chan struct{})
	eng.backupEntered = make(chan struct{}, 1)

	started, err := svc.StartBackupFlash(context.Background())
	if err != nil || !started {
		t.Fatalf("StartBackupFlash: started=%v err=%v", started, err)
	}
	select {
	case <-eng.backupEntered:
	case <-time.After(5 * time.Second):
		t.Fatal("the flash backup never reached the engine")
	}

	started, err = svc.StartBackupVMsAll(context.Background(), []string{"alpha"})
	if started || err != nil {
		t.Fatalf("a batch while another backup runs: started=%v err=%v, want false and no error", started, err)
	}

	close(eng.block)
	waitForBackupDone(t, svc)
}

func TestStartBackupVMsAllItemPanicContinues(t *testing.T) {
	svc, eng, st, ch := vmBatchService(t, zvolTPMVirsh{domainXML: fileOnlyVMDomainXML})
	eng.backupPanicTag = "vm:alpha"

	started, err := svc.StartBackupVMsAll(context.Background(), []string{"alpha", "bravo"})
	if err != nil || !started {
		t.Fatalf("StartBackupVMsAll: started=%v err=%v", started, err)
	}
	waitForVMBatch(t, ch)
	waitForBackupDone(t, svc)

	alpha, err := st.GetVMTargetByName("alpha")
	if err != nil {
		t.Fatal(err)
	}
	run := waitForRunTerminal(t, st, alpha.ID)
	if run.Status != "failed" || !strings.Contains(run.Error, "recovered panic") {
		t.Fatalf("the panicking VM's run = %+v, want a failed row carrying the panic", run)
	}
	bravo, err := st.GetVMTargetByName("bravo")
	if err != nil {
		t.Fatal(err)
	}
	next, err := st.LastRunForTarget(bravo.ID)
	if err != nil {
		t.Fatal(err)
	}
	if next == nil || next.Status != "success" {
		t.Fatalf("the VM queued behind the panicking one = %+v, want a successful run", next)
	}
}

// undefinedVMVirsh reports one VM as gone from the host, as libvirt does for a
// domain that was deleted while its backup entry stayed.
type undefinedVMVirsh struct {
	zvolTPMVirsh
	undefined string
}

func (v undefinedVMVirsh) DumpXML(ctx context.Context, name string) (string, error) {
	if name == v.undefined {
		return "", errors.New("virshcli: dumpxml: error: failed to get domain '" + name + "'")
	}
	return v.zvolTPMVirsh.DumpXML(ctx, name)
}
