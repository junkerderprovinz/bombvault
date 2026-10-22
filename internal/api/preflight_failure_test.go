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
	"github.com/junkerderprovinz/bombvault/internal/notify"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// preflightService has flash, config and folders configured, with every
// Healthchecks ping going to hc. Nothing has a repository yet, so eng decides
// whether one can be created.
func preflightService(t *testing.T, eng *fakeResticEngine, hc *hcRecorder) (*api.Service, *store.Repo, string) {
	t.Helper()
	dir := t.TempDir()
	root := filepath.ToSlash(dir)
	flashDir := root + "/boot"
	if err := os.MkdirAll(flashDir, 0o750); err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{
		AppKey:            strings.Repeat("a", 64),
		DataDir:           dir,
		HostMountRoot:     root,
		FlashDir:          flashDir,
		FlashTemplatesDir: filepath.Join(dir, "flashtpl"),
	}
	st := newMemStore(t)
	s := mustSettings(t, st)
	s.EncryptionEnabled = false
	s.FlashPath = "backups/flash"
	s.ConfigPath = "backups/config"
	s.FilesPath = "backups/files"
	if err := st.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}
	svc := api.NewService(cfg, st, &fakeServiceDocker{}, fakeVirsh{}, eng)
	if err := svc.SetNotifyConfig(notify.Config{On: "failure", HealthchecksURL: hc.srv.URL + "/hc"}); err != nil {
		t.Fatal(err)
	}
	return svc, st, flashDir
}

func failedRuns(t *testing.T, st *store.Repo, targetID string) int {
	t.Helper()
	runs, err := st.ListRuns(100)
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, r := range runs {
		if r.TargetID == targetID && r.Status == "failed" {
			n++
		}
	}
	return n
}

// A flash or config backup whose repository cannot be opened or created
// fails before its run starts, and still shows as a failed run and a fail
// ping.
func TestSingletonRepoFailureRecordsRunAndPingsFail(t *testing.T) {
	cases := []struct {
		name     string
		targetID string
		run      func(*api.Service) error
	}{
		{"flash", store.FlashTargetID, func(s *api.Service) error { _, err := s.BackupFlash(context.Background()); return err }},
		{"config", store.ConfigTargetID, func(s *api.Service) error { _, err := s.BackupConfig(context.Background()); return err }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			hc := newHCRecorder(t)
			svc, st, _ := preflightService(t, &fakeResticEngine{initErr: errors.New("no space left on device")}, hc)

			if err := tc.run(svc); err == nil {
				t.Fatal("expected the backup to fail on the repository")
			}
			if n := failedRuns(t, st, tc.targetID); n != 1 {
				t.Fatalf("failed runs = %d, want 1", n)
			}
			if !hc.pinged("/hc/fail") {
				t.Fatalf("no fail ping, pings = %v", hc.seen())
			}
		})
	}
}

// An unmounted flash is recorded like any other failure before the run.
func TestFlashNotMountedRecordsFailedRun(t *testing.T) {
	hc := newHCRecorder(t)
	svc, st, flashDir := preflightService(t, &fakeResticEngine{}, hc)
	if err := os.Remove(flashDir); err != nil {
		t.Fatal(err)
	}

	if _, err := svc.BackupFlash(context.Background()); err == nil || !strings.Contains(err.Error(), "not mounted") {
		t.Fatalf("BackupFlash err = %v, want the not-mounted refusal", err)
	}
	if n := failedRuns(t, st, store.FlashTargetID); n != 1 {
		t.Fatalf("failed runs = %d, want 1", n)
	}
}

// A folder set whose repository cannot be created records one failed run;
// the missing-source guard, which recorded its own, records no second one.
func TestFileSetPreflightFailureRecordsOneRun(t *testing.T) {
	t.Run("repository", func(t *testing.T) {
		hc := newHCRecorder(t)
		svc, st, flashDir := preflightService(t, &fakeResticEngine{initErr: errors.New("no space left on device")}, hc)
		set, err := st.CreateFileSet(store.FileSet{Name: "docs", Path: "boot", Enabled: true})
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(flashDir, "a.txt"), []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}

		if _, err := svc.BackupFileSet(context.Background(), set.ID); err == nil {
			t.Fatal("expected the backup to fail on the repository")
		}
		if n := failedRuns(t, st, set.ID); n != 1 {
			t.Fatalf("failed runs = %d, want 1", n)
		}
		if !hc.pinged("/hc/fail") {
			t.Fatalf("no fail ping, pings = %v", hc.seen())
		}
	})
	t.Run("missing source", func(t *testing.T) {
		hc := newHCRecorder(t)
		svc, st, _ := preflightService(t, &fakeResticEngine{}, hc)
		set, err := st.CreateFileSet(store.FileSet{Name: "gone", Path: "data/vanished", Enabled: true})
		if err != nil {
			t.Fatal(err)
		}

		if _, err := svc.BackupFileSet(context.Background(), set.ID); err == nil {
			t.Fatal("expected the missing source to fail")
		}
		if n := failedRuns(t, st, set.ID); n != 1 {
			t.Fatalf("failed runs = %d, want 1", n)
		}
	})
}
