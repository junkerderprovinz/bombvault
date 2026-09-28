package api

import (
	"context"
	"errors"
	"os"
	"slices"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/backup"
	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/store"
	"github.com/junkerderprovinz/bombvault/internal/virshcli"
)

// checkpointVirsh is a VM with libvirt's backup API. bitmaps are in the disk
// image, names are libvirt's record of the checkpoints.
type checkpointVirsh struct {
	scriptedVirsh
	running bool
	names   []string
	bitmaps map[string]bool
	job     string
	calls   []string
}

var _ virshcli.BlockBackups = (*checkpointVirsh)(nil)

func (v *checkpointVirsh) CheckpointNames(context.Context, string) ([]string, error) {
	return slices.Clone(v.names), nil
}

func (v *checkpointVirsh) CheckpointDelete(_ context.Context, _, name string) error {
	v.calls = append(v.calls, "delete "+name)
	if !v.running {
		return errors.New("cannot delete checkpoint for inactive domain")
	}
	if !v.bitmaps[name] {
		return errors.New("bitmap not found in backing chain")
	}
	delete(v.bitmaps, name)
	v.names = slices.DeleteFunc(v.names, func(n string) bool { return n == name })
	return nil
}

func (v *checkpointVirsh) CheckpointForget(_ context.Context, _, name string) error {
	v.calls = append(v.calls, "forget "+name)
	v.names = slices.DeleteFunc(v.names, func(n string) bool { return n == name })
	return nil
}

func (v *checkpointVirsh) CheckpointXML(_ context.Context, _, name string) (string, error) {
	return "<domaincheckpoint><name>" + name + "</name></domaincheckpoint>", nil
}

func (v *checkpointVirsh) CheckpointRedefine(_ context.Context, _, def string, validate bool) error {
	name := backup.CheckpointName(def)
	v.calls = append(v.calls, "redefine "+name)
	if validate && (!v.running || !v.bitmaps[name]) {
		return errors.New("checkpoint inconsistent")
	}
	v.names = append(v.names, name)
	return nil
}

func (v *checkpointVirsh) BackupBegin(context.Context, string, string, string) error { return nil }

func (v *checkpointVirsh) BackupJobXML(context.Context, string) (string, error) { return v.job, nil }

func (v *checkpointVirsh) AbortJob(context.Context, string) error {
	v.calls = append(v.calls, "abort")
	v.job = ""
	return nil
}

func (v *checkpointVirsh) FSFreeze(context.Context, string) error { return nil }
func (v *checkpointVirsh) FSThaw(context.Context, string) error   { return nil }

func checkpointService(t *testing.T, v *checkpointVirsh) (*Service, blockCheckpointFile) {
	t.Helper()
	st := newTestStore(t)
	tg, err := st.UpsertVMTarget(store.VMTarget{Name: "win", Method: "graceful"})
	if err != nil {
		t.Fatal(err)
	}
	if err := st.SetVMBlockBackupEnabled(tg.ID, true); err != nil {
		t.Fatal(err)
	}
	s := &Service{virsh: v, store: st, cfg: config.Config{DataDir: t.TempDir()}}
	kept := s.blockCheckpointFile(tg.ID)
	if err := kept.Save("<domaincheckpoint><name>bombvault-1</name></domaincheckpoint>"); err != nil {
		t.Fatal(err)
	}
	return s, kept
}

func TestTurningBlockBackupsOffDeletesTheKeptBitmap(t *testing.T) {
	v := &checkpointVirsh{running: true, bitmaps: map[string]bool{"bombvault-1": true, "nightly": true}, names: []string{"nightly"}}
	s, kept := checkpointService(t, v)
	if err := s.SetVMBlockBackup(t.Context(), "win", false); err != nil {
		t.Fatal(err)
	}
	if v.bitmaps["bombvault-1"] || !v.bitmaps["nightly"] || !slices.Equal(v.names, []string{"nightly"}) {
		t.Fatalf("bitmaps %v, checkpoints %v, want only the foreign one left", v.bitmaps, v.names)
	}
	if _, err := os.Stat(string(kept)); !os.IsNotExist(err) {
		t.Fatalf("kept definition still there: %v", err)
	}
}

func TestTurningBlockBackupsOffOnAStoppedVMLeavesNoCheckpointOnRecord(t *testing.T) {
	v := &checkpointVirsh{bitmaps: map[string]bool{"bombvault-1": true}}
	s, _ := checkpointService(t, v)
	if err := s.SetVMBlockBackup(t.Context(), "win", false); err != nil {
		t.Fatal(err)
	}
	if len(v.names) != 0 {
		t.Fatalf("checkpoints on record = %v, want none so the VM can be undefined", v.names)
	}
}
