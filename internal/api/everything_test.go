package api_test

// The pass is exercised over containers, flash and config, the domains that
// are cheap to fake. vms and files are switched on too but have nothing to back
// up, as for an operator who has not set up VMs or file sets.

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
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// everythingOrderLog is the trail of domain backups and hook runs. The pass
// runs on the calling goroutine, so the entries are in execution order.
type everythingOrderLog struct {
	entries []string
}

func (o *everythingOrderLog) add(s string) { o.entries = append(o.entries, s) }

// orderedEngine logs the domain of every Backup call, judged by the repo path.
type orderedEngine struct {
	*fakeResticEngine
	log *everythingOrderLog
}

func (e *orderedEngine) Backup(ctx context.Context, repo string, paths, tags []string, mode restic.Mode, excludes ...string) (restic.Summary, error) {
	switch {
	case strings.Contains(repo, "containers"):
		e.log.add("containers")
	case strings.Contains(repo, "flash"):
		e.log.add("flash")
	case strings.Contains(repo, "config"):
		e.log.add("config")
	default:
		e.log.add("backup:" + repo)
	}
	return e.fakeResticEngine.Backup(ctx, repo, paths, tags, mode, excludes...)
}

// everythingFakeHostShell records every command it runs and, with a log set,
// adds a "hook:<cmd>" entry to it.
type everythingFakeHostShell struct {
	log   *everythingOrderLog
	calls []string
}

var _ api.HostShell = (*everythingFakeHostShell)(nil)

func (f *everythingFakeHostShell) Run(_ context.Context, cmd string) error {
	f.calls = append(f.calls, cmd)
	if f.log != nil {
		f.log.add("hook:" + cmd)
	}
	return nil
}

// everythingTestService builds a Service with all five domains switched on:
// one container target whose SelectedPaths folder exists (so the containers
// step reaches Restic.Backup and can be blocked through eng), a /boot for
// flash, and a ConfigPath. There are no VM targets or file sets.
func everythingTestService(t *testing.T, eng api.ResticEngine) (svc *api.Service, st *store.Repo, docker *fakeServiceDocker, tg store.Target) {
	t.Helper()
	dir := t.TempDir()
	root := filepath.ToSlash(dir)
	flashDir := root + "/boot"
	if err := os.MkdirAll(flashDir, 0o750); err != nil {
		t.Fatal(err)
	}
	dataDir := filepath.Join(dir, "data", "primary")
	if err := os.MkdirAll(dataDir, 0o750); err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{
		AppKey:            strings.Repeat("a", 64),
		DataDir:           dir,
		HostMountRoot:     root,
		FlashDir:          flashDir,
		FlashTemplatesDir: filepath.Join(dir, "flashtpl"),
	}
	st = newMemStore(t)
	s := mustSettings(t, st)
	s.EncryptionEnabled = false
	s.ContainersPath = "backups/containers"
	s.FlashPath = "backups/flash"
	s.ConfigPath = "backups/config"
	// The store defaults leave vms, flash, config and files off, and the pass
	// skips switched-off domains.
	s.ContainersEnabled = true
	s.VMsEnabled = true
	s.FlashEnabled = true
	s.FilesEnabled = true
	s.ConfigEnabled = true
	if err := st.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}

	// Seed the containers repo so EnsureRepo passes cleanly.
	repo := filepath.Join(dir, "backups", "containers")
	if err := os.MkdirAll(repo, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "config"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	docker = &fakeServiceDocker{}
	svc = api.NewService(cfg, st, docker, fakeVirsh{}, eng)

	var err error
	tg, err = st.UpsertTarget(store.Target{
		ContainerName:     "primary",
		IncludeInSchedule: true,
		SelectedPaths:     []string{dataDir},
	})
	if err != nil {
		t.Fatalf("seed target: %v", err)
	}
	return svc, st, docker, tg
}

// waitForEverythingDone blocks until StartBackupEverything's goroutine has
// released the guard.
func waitForEverythingDone(t *testing.T, svc *api.Service) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if !svc.EverythingInProgress() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("timed out waiting for the Backup Everything pass to finish")
}

// The order is containers, vms, flash, files, config. vms and files have
// nothing to back up here, so only the other three show in the log.
func TestBackupEverythingOrder(t *testing.T) {
	log := &everythingOrderLog{}
	oe := &orderedEngine{fakeResticEngine: &fakeResticEngine{}, log: log}
	svc, _, _, _ := everythingTestService(t, oe)

	if _, err := svc.BackupEverything(context.Background()); err != nil {
		t.Fatalf("BackupEverything: %v", err)
	}

	want := []string{"containers", "flash", "config"}
	if len(log.entries) != len(want) {
		t.Fatalf("order log = %v, want %v", log.entries, want)
	}
	for i, w := range want {
		if log.entries[i] != w {
			t.Fatalf("order log[%d] = %q, want %q (full log: %v)", i, log.entries[i], w, log.entries)
		}
	}
}

// The pre-hook fires before any domain and the post-hook after all of them,
// checked in the same log as the domains.
func TestBackupEverythingHooksFireExactlyOnce(t *testing.T) {
	log := &everythingOrderLog{}
	oe := &orderedEngine{fakeResticEngine: &fakeResticEngine{}, log: log}
	svc, st, _, _ := everythingTestService(t, oe)

	shell := &everythingFakeHostShell{log: log}
	svc.SetHostShell(shell)

	s := mustSettings(t, st)
	s.EverythingPreHook = "echo pre"
	s.EverythingPostHook = "echo post"
	if err := st.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}

	if _, err := svc.BackupEverything(context.Background()); err != nil {
		t.Fatalf("BackupEverything: %v", err)
	}

	if len(shell.calls) != 2 || shell.calls[0] != "echo pre" || shell.calls[1] != "echo post" {
		t.Fatalf("hostShell calls = %v, want exactly [echo pre, echo post]", shell.calls)
	}

	want := []string{"hook:echo pre", "containers", "flash", "config", "hook:echo post"}
	if len(log.entries) != len(want) {
		t.Fatalf("combined order log = %v, want %v", log.entries, want)
	}
	for i, w := range want {
		if log.entries[i] != w {
			t.Fatalf("combined order log[%d] = %q, want %q (full log: %v)", i, log.entries[i], w, log.entries)
		}
	}
}

// Every Docker inspect fails, so containers fails as a whole. Flash and config
// still run, the post-hook fires once, and the breakdown names the failure.
func TestBackupEverythingSurvivesOneDomainFailing(t *testing.T) {
	log := &everythingOrderLog{}
	oe := &orderedEngine{fakeResticEngine: &fakeResticEngine{}, log: log}
	svc, st, docker, _ := everythingTestService(t, oe)

	shell := &everythingFakeHostShell{log: log}
	svc.SetHostShell(shell)

	s := mustSettings(t, st)
	s.EverythingPostHook = "echo post"
	if err := st.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}

	docker.inspectErr = errors.New("simulated docker inspect failure")

	sum, err := svc.BackupEverything(context.Background())
	if err != nil {
		t.Fatalf("BackupEverything: %v", err)
	}
	if sum.Status != "failed" {
		t.Fatalf("Status = %q, want %q (domains: %+v)", sum.Status, "failed", sum.Domains)
	}
	if !strings.Contains(sum.Error, "containers") || !strings.Contains(sum.Error, "simulated docker inspect failure") {
		t.Fatalf("breakdown %q does not name the failing domain/reason", sum.Error)
	}

	// The other domains still ran, in order.
	want := []string{"flash", "config", "hook:echo post"}
	if len(log.entries) != len(want) {
		t.Fatalf("order log = %v, want %v (flash/config must still run, post-hook must still fire)", log.entries, want)
	}
	for i, w := range want {
		if log.entries[i] != w {
			t.Fatalf("order log[%d] = %q, want %q (full log: %v)", i, log.entries[i], w, log.entries)
		}
	}
	if len(shell.calls) != 1 || shell.calls[0] != "echo post" {
		t.Fatalf("post-hook calls = %v, want exactly one [echo post]", shell.calls)
	}
}

func TestBackupEverythingAllCleanPass(t *testing.T) {
	svc, _, _, _ := everythingTestService(t, &fakeResticEngine{})

	sum, err := svc.BackupEverything(context.Background())
	if err != nil {
		t.Fatalf("BackupEverything: %v", err)
	}
	if sum.Status != "success" {
		t.Fatalf("Status = %q, want %q (domains: %+v)", sum.Status, "success", sum.Domains)
	}
	if sum.Error != "" {
		t.Fatalf("Error = %q, want empty on a clean pass", sum.Error)
	}
}

// The containers child run carries the parent run's id as GroupID.
func TestBackupEverythingGroupStamping(t *testing.T) {
	svc, st, _, tg := everythingTestService(t, &fakeResticEngine{})

	sum, err := svc.BackupEverything(context.Background())
	if err != nil {
		t.Fatalf("BackupEverything: %v", err)
	}
	if sum.RunID == "" {
		t.Fatal("expected a non-empty parent run id")
	}

	run, err := st.LastRunForTarget(tg.ID)
	if err != nil {
		t.Fatal(err)
	}
	if run == nil {
		t.Fatal("expected a recorded container run")
	}
	if run.GroupID != sum.RunID {
		t.Fatalf("child run GroupID = %q, want the parent run id %q", run.GroupID, sum.RunID)
	}
}

// The fake engine's block channel holds the first pass inside the containers
// backup, so it is still in flight for the second call.
func TestStartBackupEverythingRefusesConcurrent(t *testing.T) {
	eng := &fakeResticEngine{block: make(chan struct{})}
	svc, _, _, _ := everythingTestService(t, eng)

	started1, err1 := svc.StartBackupEverything(context.Background())
	if err1 != nil || !started1 {
		t.Fatalf("first StartBackupEverything should start: started=%v err=%v", started1, err1)
	}

	started2, err2 := svc.StartBackupEverything(context.Background())
	if err2 != nil || started2 {
		t.Fatalf("second StartBackupEverything while one is in flight should be refused: started=%v err=%v", started2, err2)
	}

	close(eng.block)
	waitForEverythingDone(t, svc)
}

// The ZFS step sits between folders and the self-backup, so an item that holds
// a container's data is snapshotted before the configuration that describes it.
func TestBackupEverythingRunsZFSAfterFiles(t *testing.T) {
	svc, st, _, _ := everythingTestService(t, &fakeResticEngine{})
	s := mustSettings(t, st)
	s.ZFSEnabled = true
	s.ZFSPath = "backups/zfs"
	if err := st.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateZFSDataset(store.ZFSDataset{Dataset: "cache/appdata", Enabled: true}); err != nil {
		t.Fatalf("seed zfs item: %v", err)
	}

	sum, err := svc.BackupEverything(context.Background())
	if err != nil {
		t.Fatalf("BackupEverything: %v", err)
	}
	var order []string
	for _, d := range sum.Domains {
		order = append(order, d.Domain)
	}
	want := []string{"containers", "vms", "flash", "files", "zfs", "config"}
	if len(order) != len(want) {
		t.Fatalf("domain order = %v, want %v", order, want)
	}
	for i, w := range want {
		if order[i] != w {
			t.Fatalf("domain order[%d] = %q, want %q (full order: %v)", i, order[i], w, order)
		}
	}
	for _, d := range sum.Domains {
		if d.Domain == "zfs" && d.Attempted != 1 {
			t.Fatalf("zfs step attempted %d items, want 1", d.Attempted)
		}
	}
}
