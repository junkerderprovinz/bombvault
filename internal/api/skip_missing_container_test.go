package api_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/api"
	"github.com/junkerderprovinz/bombvault/internal/backup"
	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// TestBackupSkipsRemovedContainer: a backup of a container that no longer exists on
// the host returns ErrContainerNotInstalled (a skip, not a failure), never drives
// the orchestrator (no stop/start/create side effects), and records exactly one
// "skipped" run per attempt so the dashboard reflects it and agrees with the green
// aggregate Healthchecks ping instead of showing nothing (#57).
func TestBackupSkipsRemovedContainer(t *testing.T) {
	dir := t.TempDir()
	// HostMountRoot == the temp dir so resolveRepo("backups/containers") lands under
	// it (a writable, platform-neutral path); EnsureRepo then opens the seeded repo
	// instead of trying to mkdir an unwritable host path like /host on Linux CI.
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
	// Seed a real (empty) local repo on disk so EnsureRepo sees an established repo.
	repo := filepath.Join(dir, "backups", "containers")
	if err := os.MkdirAll(repo, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "config"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	d := &fakeServiceDocker{
		inspectErr: errors.New("Error response from daemon: No such container: Nexterm"),
	}
	svc := api.NewService(cfg, st, d, fakeVirsh{}, &fakeResticEngine{})

	tg, err := st.UpsertTarget(store.Target{ContainerName: "Nexterm", IncludeInSchedule: true})
	if err != nil {
		t.Fatalf("seed target: %v", err)
	}

	_, err = svc.Backup(context.Background(), "Nexterm")
	if !errors.Is(err, backup.ErrContainerNotInstalled) {
		t.Fatalf("Backup err = %v, want ErrContainerNotInstalled", err)
	}

	// The orchestrator must never run for a removed container.
	for _, c := range d.calls {
		if strings.HasPrefix(c, "stop:") || strings.HasPrefix(c, "start:") || strings.HasPrefix(c, "createAndStart:") {
			t.Fatalf("unexpected orchestrator side effect after skip: %q (calls=%v)", c, d.calls)
		}
	}

	// Exactly one "skipped" run, attributed to the target, carrying a reason.
	runs, err := st.ListRuns(10)
	if err != nil {
		t.Fatalf("ListRuns: %v", err)
	}
	if len(runs) != 1 {
		t.Fatalf("runs = %d, want 1 (%+v)", len(runs), runs)
	}
	if r := runs[0]; r.Status != "skipped" || r.TargetID != tg.ID || !strings.Contains(r.Error, "no longer exists") {
		t.Fatalf("run = %+v, want status=skipped target=%s error~='no longer exists'", r, tg.ID)
	}

	// A second attempt on the still-missing target records another skip: the run row
	// is an honest per-attempt audit trail (only the notification is debounced).
	if _, err := svc.Backup(context.Background(), "Nexterm"); !errors.Is(err, backup.ErrContainerNotInstalled) {
		t.Fatalf("second Backup err = %v, want ErrContainerNotInstalled", err)
	}
	if runs, _ = st.ListRuns(10); len(runs) != 2 {
		t.Fatalf("after second skip, runs = %d, want 2", len(runs))
	}
}
