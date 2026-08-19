package api_test

// Tests for Task 3 of the "Backup Everything" plan (docs/superpowers/plans/
// 2026-08-20-backup-everything.md): the runGroupKey/WithRunGroup context-flag
// idiom threaded through runsAdapter/startedRunsAdapter so a child run
// produced during a "Backup Everything" pass carries group_id = the parent
// run's id (store.SetRunGroup, Task 1). The critical regression guard is that
// this is a PURE NO-OP for every existing caller — nothing today ever calls
// WithRunGroup, so runGroupFromContext returns "" everywhere and
// store.SetRunGroup is never even invoked. Both the ungrouped (today's
// behaviour) and grouped case are asserted explicitly below, not just
// inferred from inspection.

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

// runGroupContainerBackupService builds the same minimal "stateless container
// backup succeeds" harness as TestBackupSuccessRecordsSingleRun
// (backup_failure_recorded_test.go) — the cheapest real path through
// runsAdapter.Start (s.Backup → backup.BackupContainer → deps.Runs.Start).
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
	// Seed the repo so EnsureRepo passes; a stateless container (no appdata)
	// makes it a definition-only backup that still records a run.
	repo := filepath.Join(dir, "backups", "containers")
	if err := os.MkdirAll(repo, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "config"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	d := &fakeServiceDocker{} // Inspect succeeds → stateless container
	svc := api.NewService(cfg, st, d, fakeVirsh{}, &fakeResticEngine{})

	tg, err := st.UpsertTarget(store.Target{ContainerName: "stateless", IncludeInSchedule: true})
	if err != nil {
		t.Fatalf("seed target: %v", err)
	}
	return svc, st, tg
}

// TestRunsAdapterGroupStampNoOpByDefault pins the critical regression guard
// for Task 3: a container backup driven through a plain context.Background()
// (exactly what every caller in the codebase passes today) must produce a Run
// row with GroupID == "" — byte-identical to before runGroupKey/WithRunGroup
// existed. Nothing today ever calls WithRunGroup, so this must hold for the
// REAL production call path, not just runGroupFromContext in isolation.
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

// TestRunsAdapterGroupStampWithRunGroup is the positive case: a container
// backup driven through a WithRunGroup-wrapped context must produce a Run row
// carrying that group id — proving runsAdapter.Start's SetRunGroup stamp
// actually reaches the real backup call path, not just its own isolated unit
// test.
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

// TestBackupVMGroupStamp covers the OTHER half of Task 3 — BackupVM's inline
// group-stamp. startedRunsAdapter pre-obtains its run id via a raw
// s.store.StartRun call BEFORE the adapter is built (see startedRunsAdapter's
// doc comment), so the group stamp for the VM path happens inline right after
// that raw call, not inside a later Start(). Reuses vmZvolTestService
// (vm_zvol_tpm_wiring_test.go), the existing file-only-VM harness — the
// zvol/TPM plumbing it exercises is irrelevant here, only the run-group
// wiring is under test. Both the no-op (plain context.Background()) and the
// grouped case are covered, mirroring the runsAdapter guard above.
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
