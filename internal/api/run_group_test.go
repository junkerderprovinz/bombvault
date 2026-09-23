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

// runGroupContainerBackupService sets up a stateless container backup, the
// cheapest real path through runsAdapter.Start.
func runGroupContainerBackupService(t *testing.T) (*api.Service, *store.Repo, store.Target) {
	t.Helper()
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
	// Seed the repo so EnsureRepo passes. Without appdata the backup is
	// definition-only but still records a run.
	repo := filepath.Join(dir, "backups", "containers")
	if err := os.MkdirAll(repo, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "config"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	d := &fakeServiceDocker{}
	svc := api.NewService(cfg, st, d, fakeVirsh{}, &fakeResticEngine{})

	tg, err := st.UpsertTarget(store.Target{ContainerName: "stateless", IncludeInSchedule: true})
	if err != nil {
		t.Fatalf("seed target: %v", err)
	}
	return svc, st, tg
}

// A backup run without WithRunGroup records no group id.
func TestRunsAdapterGroupStampNoOpByDefault(t *testing.T) {
	svc, st, tg := runGroupContainerBackupService(t)

	if _, err := svc.Backup(context.Background(), "stateless"); err != nil {
		t.Fatalf("Backup: %v", err)
	}

	run, err := st.LastRunForTarget(tg.ID)
	if err != nil {
		t.Fatal(err)
	}
	if run == nil {
		t.Fatal("expected a recorded run")
	}
	if run.GroupID != "" {
		t.Fatalf("GroupID = %q, want \"\" (a plain context.Background() must never stamp a group)", run.GroupID)
	}
}

// A backup run under WithRunGroup records that group id.
func TestRunsAdapterGroupStampWithRunGroup(t *testing.T) {
	svc, st, tg := runGroupContainerBackupService(t)

	ctx := api.WithRunGroup(context.Background(), "grp-abc123")
	if _, err := svc.Backup(ctx, "stateless"); err != nil {
		t.Fatalf("Backup: %v", err)
	}

	run, err := st.LastRunForTarget(tg.ID)
	if err != nil {
		t.Fatal(err)
	}
	if run == nil {
		t.Fatal("expected a recorded run")
	}
	if run.GroupID != "grp-abc123" {
		t.Fatalf("GroupID = %q, want %q", run.GroupID, "grp-abc123")
	}
}

// BackupVM starts its run before building startedRunsAdapter, so it stamps the
// group itself and needs its own test. vmZvolTestService is only used as a
// file-only VM harness.
func TestBackupVMGroupStamp(t *testing.T) {
	t.Run("no group by default", func(t *testing.T) {
		svc, _, st, _ := vmZvolTestService(t, fileOnlyVMDomainXML, &zvolTPMSSH{})
		if _, err := svc.BackupVM(context.Background(), "plainvm"); err != nil {
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
		if run == nil {
			t.Fatal("expected a recorded run")
		}
		if run.GroupID != "" {
			t.Fatalf("GroupID = %q, want \"\" (a plain context.Background() must never stamp a group)", run.GroupID)
		}
	})

	t.Run("stamped when WithRunGroup is set", func(t *testing.T) {
		svc, _, st, _ := vmZvolTestService(t, fileOnlyVMDomainXML, &zvolTPMSSH{})
		ctx := api.WithRunGroup(context.Background(), "grp-vm456")
		if _, err := svc.BackupVM(ctx, "plainvm"); err != nil {
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
		if run == nil {
			t.Fatal("expected a recorded run")
		}
		if run.GroupID != "grp-vm456" {
			t.Fatalf("GroupID = %q, want %q", run.GroupID, "grp-vm456")
		}
	})
}
