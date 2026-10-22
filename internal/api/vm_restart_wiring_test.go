package api_test

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/api"
	"github.com/junkerderprovinz/bombvault/internal/config"
)

// unstartableVirsh is a running VM that shuts down cleanly and then refuses to
// start, as when BombVault stops between the two.
type unstartableVirsh struct{ zvolTPMVirsh }

func (unstartableVirsh) IsActive(context.Context, string) (bool, error) { return true, nil }
func (unstartableVirsh) State(context.Context, string) (string, error)  { return "shut off", nil }
func (unstartableVirsh) Start(context.Context, string) error            { return errors.New("connection reset") }

// A VM a graceful backup shut down stays on record until it runs again, so
// the next boot can start it.
func TestGracefulVMBackupLeavesARecordForBootWhenTheVMStaysOff(t *testing.T) {
	dir := t.TempDir()
	root := t.TempDir()
	cfg := config.Config{AppKey: strings.Repeat("a", 64), DataDir: dir, HostMountRoot: root, HostSourceRoot: "/mnt"}
	st := newMemStore(t)
	settings := mustSettings(t, st)
	settings.VMsPath = "backups/vms"
	if err := st.UpdateSettings(settings); err != nil {
		t.Fatal(err)
	}
	svc := api.NewService(cfg, st, &fakeServiceDocker{}, unstartableVirsh{zvolTPMVirsh{domainXML: fileOnlyVMDomainXML}}, &fakeResticEngine{})
	svc.SetHostSSH(&zvolTPMSSH{})

	if _, err := svc.BackupVM(context.Background(), "plainvm"); err == nil {
		t.Fatal("expected the failed restart to fail the backup")
	}
	left, err := st.ListVMsStopped()
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(left, []string{"plainvm"}) {
		t.Fatalf("recorded %v, want plainvm", left)
	}
}
