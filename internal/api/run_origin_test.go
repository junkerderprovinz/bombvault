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
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// mcpOrigin is the origin a tool call would put on its context.
var mcpOrigin = api.RunOrigin{Via: "mcp", KeyID: "k1"}

// wantOrigin fails unless the run carries the MCP origin.
func wantOrigin(t *testing.T, what string, run *store.Run) {
	t.Helper()
	if run == nil {
		t.Fatalf("%s: no run recorded", what)
	}
	if run.StartedVia != "mcp" || run.StartedViaKey != "k1" {
		t.Fatalf("%s: startedVia %q key %q, want mcp/k1", what, run.StartedVia, run.StartedViaKey)
	}
}

func TestContainerBackupCarriesOrigin(t *testing.T) {
	svc, st, tg := runGroupContainerBackupService(t)

	started, err := svc.StartBackup(api.WithRunOrigin(context.Background(), mcpOrigin), "stateless")
	if err != nil || !started {
		t.Fatalf("StartBackup: started=%v err=%v", started, err)
	}
	waitForBackupDone(t, svc)

	run, err := st.LastRunForTarget(tg.ID)
	if err != nil {
		t.Fatal(err)
	}
	wantOrigin(t, "container backup", run)
}

func TestContainerPreflightFailureRowCarriesOrigin(t *testing.T) {
	dir := t.TempDir()
	cfg := config.Config{
		AppKey:            strings.Repeat("a", 64),
		DataDir:           dir,
		HostMountRoot:     dir,
		FlashTemplatesDir: filepath.Join(dir, "flash"),
	}
	st := newMemStore(t)
	s := mustSettings(t, st)
	s.EncryptionEnabled = false
	s.ContainersPath = "backups/containers"
	if err := st.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}
	repo := filepath.Join(dir, "backups", "containers")
	if err := os.MkdirAll(repo, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "config"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	// A Docker daemon that is gone fails the backup before the orchestrator can
	// take over the run bookkeeping, so the row comes from recordPreflightFailure.
	d := &fakeServiceDocker{inspectErr: errors.New("Cannot connect to the Docker daemon")}
	svc := api.NewService(cfg, st, d, fakeVirsh{}, &fakeResticEngine{})
	tg, err := st.UpsertTarget(store.Target{ContainerName: "stateless", IncludeInSchedule: true})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := svc.Backup(api.WithRunOrigin(context.Background(), mcpOrigin), "stateless"); err == nil {
		t.Fatal("expected the backup to fail on the injected Docker fault")
	}

	run, err := st.LastRunForTarget(tg.ID)
	if err != nil {
		t.Fatal(err)
	}
	if run == nil || run.Status != "failed" {
		t.Fatalf("expected a failed pre-flight run, got %+v", run)
	}
	wantOrigin(t, "pre-flight failure row", run)
}

func TestBackupEverythingStampsParentAndChildren(t *testing.T) {
	svc, st, _, tg := everythingTestService(t, &fakeResticEngine{})

	if _, err := svc.BackupEverything(api.WithRunOrigin(context.Background(), mcpOrigin)); err != nil {
		t.Fatalf("BackupEverything: %v", err)
	}

	parent, err := st.LastRunForTarget(store.EverythingTargetID)
	if err != nil {
		t.Fatal(err)
	}
	wantOrigin(t, "the pass's parent run", parent)

	child, err := st.LastRunForTarget(tg.ID)
	if err != nil {
		t.Fatal(err)
	}
	wantOrigin(t, "the containers child run", child)
	if child.GroupID != parent.ID {
		t.Fatalf("child group %q, want the parent run id %q", child.GroupID, parent.ID)
	}
	for _, targetID := range []string{store.FlashTargetID, store.ConfigTargetID} {
		run, rErr := st.LastRunForTarget(targetID)
		if rErr != nil {
			t.Fatal(rErr)
		}
		wantOrigin(t, "the "+targetID+" child run", run)
	}
}

func TestFollowOnRunsInheritTheOrigin(t *testing.T) {
	// A batch's prune and off-site copy run after the loop, on the domain's own
	// target id, and belong to whoever asked for the batch.
	setup := func(t *testing.T) (*api.Service, *store.Repo) {
		t.Helper()
		svc, st, _ := runGroupContainerBackupService(t)
		s := mustSettings(t, st)
		s.RetentionKeepLast = 1
		s.ContainersOffsite = "rest:http://192.168.1.2:8000/containers"
		s.ContainersOffsiteSchedule = ""
		if err := st.UpdateSettings(s); err != nil {
			t.Fatal(err)
		}
		return svc, st
	}
	runOfKind := func(t *testing.T, st *store.Repo, kind string) *store.Run {
		t.Helper()
		runs, err := st.ListRuns(200)
		if err != nil {
			t.Fatal(err)
		}
		for i := range runs {
			if runs[i].Kind == kind && runs[i].TargetID == "containers" {
				return &runs[i]
			}
		}
		t.Fatalf("no %q run on the containers domain", kind)
		return nil
	}

	t.Run("started through MCP", func(t *testing.T) {
		svc, st := setup(t)
		started, err := svc.StartBackupAll(api.WithRunOrigin(context.Background(), mcpOrigin), []string{"stateless"})
		if err != nil || !started {
			t.Fatalf("StartBackupAll: started=%v err=%v", started, err)
		}
		waitForBackupDone(t, svc)

		wantOrigin(t, "the batch's prune run", runOfKind(t, st, "prune"))
		wantOrigin(t, "the batch's off-site run", runOfKind(t, st, "offsite"))
	})

	t.Run("started from the web interface", func(t *testing.T) {
		svc, st := setup(t)
		started, err := svc.StartBackupAll(context.Background(), []string{"stateless"})
		if err != nil || !started {
			t.Fatalf("StartBackupAll: started=%v err=%v", started, err)
		}
		waitForBackupDone(t, svc)

		for _, kind := range []string{"prune", "offsite"} {
			if run := runOfKind(t, st, kind); run.StartedVia != "" || run.StartedViaKey != "" {
				t.Fatalf("%s run stamped via %q key %q, want nothing", kind, run.StartedVia, run.StartedViaKey)
			}
		}
	})
}

func TestDatabaseDumpRunCarriesOrigin(t *testing.T) {
	f := newDumpBackupFixture(t, "postgres:16")

	if _, err := f.svc.Backup(api.WithRunOrigin(context.Background(), mcpOrigin), "pg"); err != nil {
		t.Fatalf("Backup: %v", err)
	}

	runs := f.runsOfKind(t, "dbdump")
	if len(runs) != 1 {
		t.Fatalf("%d dump runs, want one", len(runs))
	}
	wantOrigin(t, "the dump run", &runs[0])
}

func TestVMAndFileSetRunsCarryOrigin(t *testing.T) {
	t.Run("vm backup", func(t *testing.T) {
		svc, _, st, _ := vmZvolTestService(t, fileOnlyVMDomainXML, &zvolTPMSSH{})
		if _, err := svc.BackupVM(api.WithRunOrigin(context.Background(), mcpOrigin), "plainvm"); err != nil {
			t.Fatalf("BackupVM: %v", err)
		}
		tg, err := st.GetVMTargetByName("plainvm")
		if err != nil {
			t.Fatal(err)
		}
		run, err := st.LastRunForTarget(tg.ID)
		if err != nil {
			t.Fatal(err)
		}
		wantOrigin(t, "vm backup", run)
	})

	t.Run("file set backup", func(t *testing.T) {
		svc, st, set := fileSetOriginService(t, "data/docs", true)
		if _, err := svc.BackupFileSet(api.WithRunOrigin(context.Background(), mcpOrigin), set.ID); err != nil {
			t.Fatalf("BackupFileSet: %v", err)
		}
		run, err := st.LastRunForTarget(set.ID)
		if err != nil {
			t.Fatal(err)
		}
		wantOrigin(t, "file set backup", run)
	})

	t.Run("file set whose source path is gone", func(t *testing.T) {
		svc, st, set := fileSetOriginService(t, "data/vanished", false)
		if _, err := svc.BackupFileSet(api.WithRunOrigin(context.Background(), mcpOrigin), set.ID); err == nil {
			t.Fatal("expected an error for a missing source path")
		}
		run, err := st.LastRunForTarget(set.ID)
		if err != nil {
			t.Fatal(err)
		}
		if run == nil || run.Status != "failed" {
			t.Fatalf("expected a failed run for the missing source, got %+v", run)
		}
		wantOrigin(t, "missing source path row", run)
	})
}

// fileSetOriginService wires a files-domain service around one set at path,
// creating the source folder only when it should exist.
func fileSetOriginService(t *testing.T, path string, createSource bool) (*api.Service, *store.Repo, store.FileSet) {
	t.Helper()
	dir := t.TempDir()
	root := filepath.ToSlash(dir)
	if createSource {
		if err := os.MkdirAll(root+"/"+path, 0o750); err != nil {
			t.Fatal(err)
		}
	}
	cfg := config.Config{AppKey: strings.Repeat("a", 64), DataDir: dir, HostMountRoot: root}
	st := newMemStore(t)
	s := mustSettings(t, st)
	s.FilesPath = "backups/files"
	if err := st.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}
	set, err := st.CreateFileSet(store.FileSet{Name: "docs", Path: path, Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	svc := api.NewService(cfg, st, &fakeServiceDocker{}, fakeVirsh{}, &fakeResticEngine{})
	return svc, st, set
}
