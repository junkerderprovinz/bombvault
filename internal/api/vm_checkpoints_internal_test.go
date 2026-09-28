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

type recordingForwarder struct{ calls *[]string }

func (f recordingForwarder) ForwardUnix(context.Context, string, string) (func(), error) {
	*f.calls = append(*f.calls, "forward")
	return func() {}, nil
}

func TestBlockJobForwardsTheSocketOnlyOnceADiskIsRead(t *testing.T) {
	v := &checkpointVirsh{running: true}
	h := &vmBlockHost{
		BlockBackups: v,
		fwd:          recordingForwarder{calls: &v.calls},
		hostDisks:    map[string]string{"vda": "/mnt/user/domains/win/vdisk1.qcow2"},
		localSock:    t.TempDir() + "/nbd.sock",
		remoteSock:   remoteNBDSocketPrefix + "x.sock",
	}
	job, err := h.StartJob(t.Context(), "win", "", "bombvault-2", []string{"vda"})
	if err != nil {
		t.Fatal(err)
	}
	if slices.Contains(v.calls, "forward") {
		t.Fatal("the socket was forwarded while the guest may still be frozen")
	}
	_, _ = job.Open(t.Context(), "vda")
	if !slices.Contains(v.calls, "forward") {
		t.Fatalf("calls = %v, want the forward when the disk is opened", v.calls)
	}
	if err := job.Stop(t.Context()); err != nil {
		t.Fatal(err)
	}
}

func TestEndLeftoverBlockJobEndsOnlyBombVaultsOwn(t *testing.T) {
	for _, tc := range []struct {
		socket string
		abort  bool
	}{
		{remoteNBDSocketPrefix + "abc.sock", true},
		{"/var/run/other-tool.sock", false},
	} {
		v := &checkpointVirsh{running: true, job: "<domainbackup mode='pull'><server transport='unix' socket='" + tc.socket + "'/></domainbackup>"}
		if err := endLeftoverBlockJob(t.Context(), v, "win"); err != nil {
			t.Fatal(err)
		}
		if got := slices.Contains(v.calls, "abort"); got != tc.abort {
			t.Fatalf("socket %s: aborted = %v, want %v", tc.socket, got, tc.abort)
		}
	}
}

const leftoverJob = "<domainbackup mode='pull'><server transport='unix' socket='" + remoteNBDSocketPrefix + "abc.sock'/></domainbackup>"

func TestStartupEndsWhatAnInterruptedBlockBackupLeft(t *testing.T) {
	v := &checkpointVirsh{
		running: true,
		job:     leftoverJob,
		names:   []string{"bombvault-1", "bombvault-2", "nightly"},
		bitmaps: map[string]bool{"bombvault-1": true, "bombvault-2": true, "nightly": true},
	}
	s, kept := checkpointService(t, v)
	s.SweepBlockBackupLeftovers(t.Context())
	if !slices.Contains(v.calls, "abort") {
		t.Fatalf("calls = %v, want the leftover job ended", v.calls)
	}
	if !slices.Equal(v.names, []string{"nightly"}) {
		t.Fatalf("checkpoints on record = %v, want only the foreign one", v.names)
	}
	if !v.bitmaps["bombvault-1"] || v.bitmaps["bombvault-2"] {
		t.Fatalf("bitmaps = %v, want the kept one and not the interrupted run's", v.bitmaps)
	}
	if def, _ := kept.Load(); backup.CheckpointName(def) != "bombvault-1" {
		t.Fatalf("kept definition = %q", def)
	}
}

func TestTurningBlockBackupsOffEndsALeftoverJob(t *testing.T) {
	v := &checkpointVirsh{running: true, job: leftoverJob, bitmaps: map[string]bool{"bombvault-1": true}}
	s, _ := checkpointService(t, v)
	if err := s.SetVMBlockBackup(t.Context(), "win", false); err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(v.calls, "abort") {
		t.Fatalf("calls = %v, want the leftover job ended", v.calls)
	}
}

func TestTurningBlockBackupsOffLeavesARunningBackupAlone(t *testing.T) {
	v := &checkpointVirsh{running: true, job: leftoverJob, names: []string{"bombvault-2"}, bitmaps: map[string]bool{"bombvault-1": true, "bombvault-2": true}}
	s, _ := checkpointService(t, v)
	s.bindBackupRun("vm:win", "run-1")
	if err := s.SetVMBlockBackup(t.Context(), "win", false); err != nil {
		t.Fatal(err)
	}
	if len(v.calls) != 0 {
		t.Fatalf("calls = %v, want the running backup's job and checkpoints left alone", v.calls)
	}
}
