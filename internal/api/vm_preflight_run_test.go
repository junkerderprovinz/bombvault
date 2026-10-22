package api_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/api"
	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// overlayVirsh serves a shut-off domain still on a BombVault snapshot overlay,
// which BackupVM refuses before the orchestrator starts.
type overlayVirsh struct{ fakeVirsh }

func (overlayVirsh) DumpXML(context.Context, string) (string, error) {
	return `<domain><devices>
  <disk type='file' device='disk'>
    <source file='/mnt/user/domains/HA/vdisk1.bombvault-tmp'/>
    <target dev='hdc'/>
  </disk>
</devices></domain>`, nil
}

// A VM backup refused on a leftover overlay shows as one failed run.
func TestVMOverlayRefusalRecordsOneFailedRun(t *testing.T) {
	dir := t.TempDir()
	cfg := config.Config{AppKey: strings.Repeat("a", 64), DataDir: dir, HostMountRoot: dir}
	st := newMemStore(t)
	s := mustSettings(t, st)
	s.EncryptionEnabled = false
	s.VMsEnabled = true
	s.VMsPath = "backups/vms"
	if err := st.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}
	repo := filepath.Join(dir, "backups", "vms")
	if err := os.MkdirAll(repo, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "config"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	tg, err := st.UpsertVMTarget(store.VMTarget{Name: "HA", Method: "graceful"})
	if err != nil {
		t.Fatal(err)
	}
	svc := api.NewService(cfg, st, &fakeServiceDocker{}, overlayVirsh{}, &fakeResticEngine{})

	if _, err := svc.BackupVM(context.Background(), "HA"); err == nil || !strings.Contains(err.Error(), "overlay") {
		t.Fatalf("BackupVM err = %v, want the overlay refusal", err)
	}

	runs, err := st.ListRuns(50)
	if err != nil {
		t.Fatal(err)
	}
	failed := 0
	for _, r := range runs {
		if r.TargetID == tg.ID && r.Status == "failed" {
			failed++
		}
	}
	if failed != 1 {
		t.Fatalf("failed runs for the VM = %d, want 1", failed)
	}
}
