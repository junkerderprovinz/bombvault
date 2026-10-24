package api_test

import (
	"context"
	"encoding/json"
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

func scheduled() context.Context { return api.WithScheduledBackup(context.Background()) }

func lastRun(t *testing.T, st *store.Repo) *store.Run {
	t.Helper()
	tg, err := st.GetTargetByContainer("plex")
	if err != nil {
		t.Fatal(err)
	}
	run, err := st.LastRunForTarget(tg.ID)
	if err != nil || run == nil {
		t.Fatalf("run = %+v, %v", run, err)
	}
	return run
}

func TestAScheduledBackupWhoseHookExits75WaitsUntilItGoesThrough(t *testing.T) {
	svc, st, d := plexBackup(t)
	d.hookErr = &dockercli.ExitError{Code: 75, Reason: "2 streams"}

	_, err := svc.Backup(scheduled(), "plex")
	if reason, deferred := backup.IsDeferred(err); !deferred || reason != store.ReasonDeferredHook+": 2 streams" {
		t.Fatalf("err = %v, want a deferral", err)
	}
	for _, c := range d.calls {
		if strings.HasPrefix(c, "stop:") {
			t.Fatalf("a deferred backup stopped the container: %v", d.calls)
		}
	}
	run := lastRun(t, st)
	if run.Status != "deferred" || run.RetryAt == nil {
		t.Fatalf("run = %+v; want a deferred run with its next try", run)
	}
	waiting := svc.DeferredBackups()
	if len(waiting) != 1 || waiting[0].Name != "plex" || waiting[0].NextTry != *run.RetryAt || waiting[0].Tries != 1 {
		t.Fatalf("waiting = %+v", waiting)
	}

	d.hookErr = nil
	if _, err := svc.Backup(scheduled(), "plex"); err != nil {
		t.Fatalf("the backup once the app was free: %v", err)
	}
	if left := svc.DeferredBackups(); len(left) != 0 {
		t.Fatalf("the wait outlived the backup: %+v", left)
	}
}

func TestAHookThatSaysNothingLeavesNoEmptyTail(t *testing.T) {
	svc, _, d := plexBackup(t)
	d.hookErr = &dockercli.ExitError{Code: 75}
	_, err := svc.Backup(scheduled(), "plex")
	if reason, _ := backup.IsDeferred(err); reason != store.ReasonDeferredHook {
		t.Fatalf("reason = %q", reason)
	}
}

func TestAManualBackupRunsThroughABusyApp(t *testing.T) {
	cases := map[string]func(*testing.T, *api.Service, *fakeServiceDocker){
		"a hook that exits 75": func(_ *testing.T, _ *api.Service, d *fakeServiceDocker) {
			d.hookErr = &dockercli.ExitError{Code: 75, Reason: "2 streams"}
		},
		"an activity check that cannot ask": func(t *testing.T, svc *api.Service, _ *fakeServiceDocker) {
			setDeferral(t, svc, `{"retryMin":30,"maxHours":12,"atDeadline":"give-up","preset":"jellyfin","failOpen":false,"netMin":5}`)
		},
		"a traffic limit not yet measured": func(t *testing.T, svc *api.Service, _ *fakeServiceDocker) {
			setDeferral(t, svc, `{"retryMin":30,"maxHours":12,"atDeadline":"give-up","failOpen":true,"netKiB":100,"netMin":5}`)
		},
	}
	for name, busy := range cases {
		t.Run(name, func(t *testing.T) {
			svc, st, d := plexBackup(t)
			busy(t, svc, d)
			if _, err := svc.Backup(scheduled(), "plex"); err == nil {
				t.Fatal("the scheduled backup went through a busy app")
			} else if _, deferred := backup.IsDeferred(err); !deferred {
				t.Fatalf("the scheduled backup failed instead of waiting: %v", err)
			}
			if _, err := svc.Backup(context.Background(), "plex"); err != nil {
				t.Fatalf("the manual backup: %v", err)
			}
			if run := lastRun(t, st); run.Status != "success" {
				t.Fatalf("run = %+v, want the manual backup through", run)
			}
			if left := svc.DeferredBackups(); len(left) != 0 {
				t.Fatalf("the manual backup left the wait of the scheduled one: %+v", left)
			}
		})
	}
}

func TestAPreHookFailingWithAnotherCodeStaysAFailure(t *testing.T) {
	for _, ctx := range []context.Context{context.Background(), scheduled()} {
		svc, _, d := plexBackup(t)
		d.hookErr = &dockercli.ExitError{Code: 1, Reason: "streams"}
		_, err := svc.Backup(ctx, "plex")
		if _, deferred := backup.IsDeferred(err); deferred || err == nil {
			t.Fatalf("err = %v, want a failure", err)
		}
		if left := svc.DeferredBackups(); len(left) != 0 {
			t.Fatalf("a failure started a wait: %+v", left)
		}
	}
}

func setDeferral(t *testing.T, svc *api.Service, body string) {
	t.Helper()
	var patch api.DeferralPatch
	if err := json.Unmarshal([]byte(body), &patch); err != nil {
		t.Fatal(err)
	}
	if err := svc.SetDeferPolicy(context.Background(), "plex", patch); err != nil {
		t.Fatal(err)
	}
}
