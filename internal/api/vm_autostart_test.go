package api_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/api"
	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/virshcli"
)

// autostartVirsh reads the autostart flag the way virsh dominfo shows it.
type autostartVirsh struct {
	zvolTPMVirsh
	on bool
}

func (v autostartVirsh) AutostartEnabled(context.Context, string) (bool, error) { return v.on, nil }

// The definition records the autostart flag the VM has, so a VM kept off at
// host boot, such as the second of two sharing one GPU, stays off after a
// restore.
func TestBackupVMRecordsTheVMsOwnAutostartSetting(t *testing.T) {
	stored := func(t *testing.T, v virshcli.Virsh) map[string]any {
		t.Helper()
		st := newMemStore(t)
		settings := mustSettings(t, st)
		settings.VMsPath = "backups/vms"
		if err := st.UpdateSettings(settings); err != nil {
			t.Fatal(err)
		}
		cfg := config.Config{AppKey: strings.Repeat("a", 64), DataDir: t.TempDir(), HostMountRoot: t.TempDir(), HostSourceRoot: "/mnt"}
		svc := api.NewService(cfg, st, &fakeServiceDocker{}, v, &fakeResticEngine{})
		if _, err := svc.BackupVM(context.Background(), "plainvm"); err != nil {
			t.Fatalf("BackupVM: %v", err)
		}
		tg, err := st.GetVMTargetByName("plainvm")
		if err != nil {
			t.Fatal(err)
		}
		var def map[string]any
		if err := json.Unmarshal([]byte(tg.Definition), &def); err != nil {
			t.Fatal(err)
		}
		return def
	}

	def := stored(t, autostartVirsh{zvolTPMVirsh: zvolTPMVirsh{domainXML: fileOnlyVMDomainXML}, on: false})
	if def["autostart"] != false || def["was_autostart"] != false {
		t.Fatalf("autostart = %v, was_autostart = %v, want both off", def["autostart"], def["was_autostart"])
	}

	def = stored(t, zvolTPMVirsh{domainXML: fileOnlyVMDomainXML})
	if _, ok := def["autostart"]; ok || def["was_autostart"] != true {
		t.Fatalf("a flag that cannot be read is not recorded: %v", def)
	}
}
