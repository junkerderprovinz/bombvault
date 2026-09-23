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

// TestBackupRecordsFailedRunOnPreflightFault: a backup that fails before
// backup.BackupContainer runs still records a failed run with the reason. A
// fault that hits the whole domain mid-batch (lost mount, full disk, broken
// repository) fails every remaining container here, and each of those has to
// show up in the run history.
func TestBackupRecordsFailedRunOnPreflightFault(t *testing.T) {
	cases := []struct {
		name string
		// seedRepo creates the repository so EnsureRepo passes.
		seedRepo bool
		// initErr fails restic init inside EnsureRepo. inspectErr is a daemon
		// error other than NotFound, as when the Docker socket disappears.
		initErr    error
		inspectErr error
		wantReason string // substring the recorded failed run's error must carry
	}{
		{
			name:       "EnsureRepo fault (disk full / repo error)",
			seedRepo:   false,
			initErr:    errors.New("no space left on device"),
			wantReason: "no space left on device",
		},
		{
			name:       "inspect fault (docker socket gone)",
			seedRepo:   true,
			inspectErr: errors.New("Cannot connect to the Docker daemon at unix:///var/run/docker.sock"),
			wantReason: "Cannot connect to the Docker daemon",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
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
			if tc.seedRepo {
				repo := filepath.Join(dir, "backups", "containers")
				if err := os.MkdirAll(repo, 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(repo, "config"), []byte("x"), 0o600); err != nil {
					t.Fatal(err)
				}
			}

			d := &fakeServiceDocker{inspectErr: tc.inspectErr}
			eng := &fakeResticEngine{initErr: tc.initErr}
			svc := api.NewService(cfg, st, d, fakeVirsh{}, eng)

			tg, err := st.UpsertTarget(store.Target{ContainerName: "Nexterm", IncludeInSchedule: true})
			if err != nil {
				t.Fatalf("seed target: %v", err)
			}

			if _, err := svc.Backup(context.Background(), "Nexterm"); err == nil {
				t.Fatal("expected Backup to fail on the injected fault")
			}

			// A pre-flight fault must not stop, start or recreate anything.
			for _, c := range d.calls {
				if strings.HasPrefix(c, "stop:") || strings.HasPrefix(c, "start:") || strings.HasPrefix(c, "createAndStart:") {
					t.Fatalf("unexpected orchestrator side effect after a pre-flight fault: %q (calls=%v)", c, d.calls)
				}
			}

			runs, err := st.ListRuns(10)
			if err != nil {
				t.Fatalf("ListRuns: %v", err)
			}
			if len(runs) != 1 {
				t.Fatalf("runs = %d, want exactly 1 failed run (%+v)", len(runs), runs)
			}
			run, err := st.LastRunForTarget(tg.ID)
			if err != nil {
				t.Fatal(err)
			}
			if run == nil || run.Status != "failed" {
				t.Fatalf("run = %+v, want status=failed against target %s", run, tg.ID)
			}
			if run.TargetID != tg.ID {
				t.Fatalf("run target = %q, want %q", run.TargetID, tg.ID)
			}
			if !strings.Contains(run.Error, tc.wantReason) {
				t.Fatalf("recorded reason = %q, want it to contain %q", run.Error, tc.wantReason)
			}
		})
	}
}

// TestBackupSuccessRecordsSingleRun: the deferred pre-flight finisher must not
// add a failed run next to the orchestrator's successful one.
func TestBackupSuccessRecordsSingleRun(t *testing.T) {
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
	// A stateless container makes this a definition-only backup, which still
	// records a run.
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

	if _, err := svc.Backup(context.Background(), "stateless"); err != nil {
		t.Fatalf("Backup of a stateless container should succeed: %v", err)
	}

	runs, err := st.ListRuns(10)
	if err != nil {
		t.Fatalf("ListRuns: %v", err)
	}
	if len(runs) != 1 {
		t.Fatalf("a successful backup must record exactly ONE run (no double-record), got %d: %+v", len(runs), runs)
	}
	run, err := st.LastRunForTarget(tg.ID)
	if err != nil {
		t.Fatal(err)
	}
	if run == nil || run.Status != "success" {
		t.Fatalf("expected a single successful run, got %+v", run)
	}
}
