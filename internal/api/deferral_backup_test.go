package api_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/api"
	"github.com/junkerderprovinz/bombvault/internal/backup"
	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/dockercli"
	"github.com/junkerderprovinz/bombvault/internal/model"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// plexBackup is a service with one running container, plex, whose pre-hook
// answers with the fake's hookErr.
func plexBackup(t *testing.T) (*api.Service, *store.Repo, *fakeServiceDocker) {
	t.Helper()
	dir := t.TempDir()
	root := filepath.ToSlash(dir)
	cfg := config.Config{
		AppKey: strings.Repeat("a", 64), DataDir: dir,
		HostMountRoot: root, HostSourceRoot: root, DataRootSegments: []string{"appdata"},
	}
	st := newMemStore(t)
	s := mustSettings(t, st)
	s.EncryptionEnabled = false
	s.ContainersPath = "backups/containers"
	if err := st.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}
	appdata := root + "/appdata/plex"
	if err := os.MkdirAll(appdata, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(appdata, "Preferences.xml"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	repo := filepath.Join(dir, "backups", "containers")
	if err := os.MkdirAll(repo, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "config"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	d := &fakeServiceDocker{inspect: model.Inspect{
		Name: "/plex", Image: "plexinc/pms-docker:latest", Running: true,
		Mounts: []model.Mount{{Type: "bind", Source: appdata, Destination: "/config"}},
	}}
	svc := api.NewService(cfg, st, d, fakeVirsh{}, &fakeResticEngine{})
	if _, err := st.UpsertTarget(store.Target{ContainerName: "plex", IncludeInSchedule: true}); err != nil {
		t.Fatal(err)
	}
	if err := svc.SetContainerHooks(context.Background(), "plex", "check-streams", ""); err != nil {
		t.Fatal(err)
	}
	return svc, st, d
}

func TestAPreHookThatExits75DefersTheBackupUntilItGoesThrough(t *testing.T) {
	svc, st, d := plexBackup(t)
	d.hookErr = &dockercli.ExitError{Code: 75, Reason: "2 streams"}

	_, err := svc.Backup(context.Background(), "plex")
	if reason, deferred := backup.IsDeferred(err); !deferred || !strings.Contains(reason, "hook exited 75: 2 streams") {
		t.Fatalf("err = %v, want a deferral", err)
	}
	for _, c := range d.calls {
		if strings.HasPrefix(c, "stop:") {
			t.Fatalf("a deferred backup stopped the container: %v", d.calls)
		}
	}
	tg, err := st.GetTargetByContainer("plex")
	if err != nil {
		t.Fatal(err)
	}
	run, err := st.LastRunForTarget(tg.ID)
	if err != nil || run == nil || run.Status != "deferred" || run.RetryAt == nil {
		t.Fatalf("run = %+v, %v; want a deferred run with its next try", run, err)
	}
	waiting := svc.DeferredBackups()
	if len(waiting) != 1 || waiting[0].Name != "plex" || waiting[0].NextTry != *run.RetryAt || waiting[0].Tries != 1 {
		t.Fatalf("waiting = %+v", waiting)
	}

	d.hookErr = nil
	if _, err := svc.Backup(context.Background(), "plex"); err != nil {
		t.Fatalf("the backup once the app was free: %v", err)
	}
	if left := svc.DeferredBackups(); len(left) != 0 {
		t.Fatalf("the wait outlived the backup: %+v", left)
	}
}

func TestAPreHookFailingWithAnotherCodeStaysAFailure(t *testing.T) {
	svc, _, d := plexBackup(t)
	d.hookErr = &dockercli.ExitError{Code: 1, Reason: "streams"}
	_, err := svc.Backup(context.Background(), "plex")
	if _, deferred := backup.IsDeferred(err); deferred || err == nil {
		t.Fatalf("err = %v, want a failure", err)
	}
	if left := svc.DeferredBackups(); len(left) != 0 {
		t.Fatalf("a failure started a wait: %+v", left)
	}
}
