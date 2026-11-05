package api_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/api"
	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/platform"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/virshcli"
)

// blockVirsh is a running VM on a host with the backup API.
type blockVirsh struct {
	fakeVirsh
	xml string
}

func (v *blockVirsh) DumpXML(context.Context, string) (string, error)         { return v.xml, nil }
func (v *blockVirsh) DumpXMLInactive(context.Context, string) (string, error) { return v.xml, nil }
func (v *blockVirsh) IsActive(context.Context, string) (bool, error)          { return true, nil }
func (v *blockVirsh) State(context.Context, string) (string, error)           { return "running", nil }
func (v *blockVirsh) CheckpointNames(context.Context, string) ([]string, error) {
	return nil, nil
}
func (v *blockVirsh) CheckpointDelete(context.Context, string, string) error { return nil }
func (v *blockVirsh) CheckpointForget(context.Context, string, string) error { return nil }
func (v *blockVirsh) CheckpointXML(context.Context, string, string) (string, error) {
	return "", nil
}
func (v *blockVirsh) CheckpointRedefine(context.Context, string, string, bool) error { return nil }
func (v *blockVirsh) BackupBegin(context.Context, string, string, string) error      { return nil }
func (v *blockVirsh) BackupJobXML(context.Context, string) (string, error)           { return "", nil }
func (v *blockVirsh) AbortJob(context.Context, string) error                         { return nil }
func (v *blockVirsh) FSFreeze(context.Context, string) error                         { return nil }
func (v *blockVirsh) FSThaw(context.Context, string) error                           { return nil }

var _ virshcli.BlockBackups = (*blockVirsh)(nil)

type forwardSSH struct{ zvolTPMSSH }

func (*forwardSSH) ForwardUnix(context.Context, string, string) (func(), error) {
	return func() {}, nil
}

// blockEngine fails its snapshot listing, as a repository that does not answer.
type blockEngine struct{ *fakeResticEngine }

func (blockEngine) BackupDirFrom(context.Context, string, string, string, []string, restic.Mode) (restic.Summary, error) {
	return restic.Summary{}, nil
}
func (blockEngine) LsTimes(context.Context, string, string, string, restic.Mode) ([]restic.NodeTime, error) {
	return nil, nil
}
func (blockEngine) Snapshots(context.Context, string, restic.Mode) ([]restic.Snapshot, error) {
	return nil, errors.New("Fatal: unable to open repository: connection refused")
}

func TestBlockBackupWhoseSnapshotListingFailsClosesItsRun(t *testing.T) {
	st := newMemStore(t)
	settings := mustSettings(t, st)
	settings.VMsPath = "backups/vms"
	if err := st.UpdateSettings(settings); err != nil {
		t.Fatal(err)
	}
	root := filepath.ToSlash(t.TempDir())
	if err := os.MkdirAll(filepath.FromSlash(root+"/backups/vms"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.FromSlash(root+"/backups/vms/config"), []byte("cfg"), 0o600); err != nil {
		t.Fatal(err)
	}
	writeQCOW2(t, root, "user/vm/lab/vdisk1.qcow2", "")
	cfg := config.Config{AppKey: strings.Repeat("a", 64), DataDir: t.TempDir(), HostMountRoot: root, HostSourceRoot: "/mnt"}
	v := &blockVirsh{xml: qcow2DomainXML("/mnt/user/vm/lab/vdisk1.qcow2")}
	svc := api.NewService(cfg, st, &fakeServiceDocker{}, v, blockEngine{&fakeResticEngine{}})
	svc.SetPlatform(platform.Generic{})
	svc.SetHostSSH(&forwardSSH{})
	if err := svc.SetVMBlockBackup(context.Background(), "lab", true); err != nil {
		t.Fatal(err)
	}

	if _, err := svc.BackupVM(context.Background(), "lab"); err == nil {
		t.Fatal("want the failure of the listing")
	}
	tg, err := st.GetVMTargetByName("lab")
	if err != nil {
		t.Fatal(err)
	}
	run, err := st.LastRunForTarget(tg.ID)
	if err != nil || run == nil {
		t.Fatalf("run = %v, err = %v", run, err)
	}
	if run.Status != "failed" || run.FinishedAt == nil || !strings.Contains(run.Error, "connection refused") {
		t.Fatalf("run = %+v", run)
	}
}
