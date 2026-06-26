package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/api"
	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/model"
	"github.com/junkerderprovinz/bombvault/internal/progress"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/restickey"
	"github.com/junkerderprovinz/bombvault/internal/secret"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

func TestServiceEnsureRepoIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	cfg := config.Config{AppKey: strings.Repeat("a", 64), DataDir: dir, HostMountRoot: dir}
	st := newMemStore(t)

	eng := &fakeResticEngine{}
	svc := api.NewService(cfg, st, &fakeServiceDocker{}, fakeVirsh{}, eng)

	repo := filepath.Join(dir, "repo")
	mode := restic.Mode{Encrypted: false}

	// First EnsureRepo on an empty dir → Init runs.
	if err := svc.EnsureRepo(context.Background(), repo, mode); err != nil {
		t.Fatalf("ensure repo: %v", err)
	}
	if len(eng.inited) != 1 {
		t.Fatalf("expected 1 init, got %d", len(eng.inited))
	}
	// Simulate restic having created its config marker.
	if err := os.WriteFile(filepath.Join(repo, "config"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	// Second EnsureRepo: config marker present → Init skipped.
	if err := svc.EnsureRepo(context.Background(), repo, mode); err != nil {
		t.Fatalf("ensure repo 2: %v", err)
	}
	if len(eng.inited) != 1 {
		t.Fatalf("expected init skipped second time, got %d inits", len(eng.inited))
	}
}

func TestEnsureRepoReconcilesEncryptionMode(t *testing.T) {
	newSvc := func(t *testing.T, eng *fakeResticEngine) (*api.Service, string) {
		t.Helper()
		dir := t.TempDir()
		cfg := config.Config{AppKey: strings.Repeat("a", 64), DataDir: dir, HostMountRoot: dir}
		st := newMemStore(t)
		return api.NewService(cfg, st, &fakeServiceDocker{}, fakeVirsh{}, eng), filepath.Join(dir, "repo")
	}
	enc := restic.Mode{Encrypted: true, Password: "pw"}
	plain := restic.Mode{Encrypted: false}

	t.Run("existing unencrypted, setting now encrypted → mismatch error, no init", func(t *testing.T) {
		no := false
		eng := &fakeResticEngine{existingMode: &no}
		svc, repo := newSvc(t, eng)
		err := svc.EnsureRepo(context.Background(), repo, enc)
		if err == nil {
			t.Fatal("expected a mode-mismatch error, got nil")
		}
		if !strings.Contains(err.Error(), "Encryption") {
			t.Fatalf("error should name the Encryption setting: %v", err)
		}
		if len(eng.inited) != 0 {
			t.Fatalf("must not init on a mode mismatch, got %v", eng.inited)
		}
	})

	t.Run("existing encrypted, setting now unencrypted → mismatch error, no init", func(t *testing.T) {
		yes := true
		eng := &fakeResticEngine{existingMode: &yes}
		svc, repo := newSvc(t, eng)
		err := svc.EnsureRepo(context.Background(), repo, plain)
		if err == nil {
			t.Fatal("expected a mode-mismatch error, got nil")
		}
		if len(eng.inited) != 0 {
			t.Fatalf("must not init on a mode mismatch, got %v", eng.inited)
		}
	})

	// Regression guard: the v2.7.0 attempt broke the default unencrypted setup on
	// the 2nd+ run. A consistent repo must open cleanly and never re-init.
	t.Run("existing unencrypted, setting still unencrypted → ok, no init", func(t *testing.T) {
		no := false
		eng := &fakeResticEngine{existingMode: &no}
		svc, repo := newSvc(t, eng)
		if err := svc.EnsureRepo(context.Background(), repo, plain); err != nil {
			t.Fatalf("consistent unencrypted repo must open cleanly: %v", err)
		}
		if len(eng.inited) != 0 {
			t.Fatalf("must not re-init an existing repo, got %v", eng.inited)
		}
	})

	t.Run("existing encrypted, setting still encrypted → ok, no init", func(t *testing.T) {
		yes := true
		eng := &fakeResticEngine{existingMode: &yes}
		svc, repo := newSvc(t, eng)
		if err := svc.EnsureRepo(context.Background(), repo, enc); err != nil {
			t.Fatalf("consistent encrypted repo must open cleanly: %v", err)
		}
		if len(eng.inited) != 0 {
			t.Fatalf("must not re-init an existing repo, got %v", eng.inited)
		}
	})
}

func TestServiceModeEncryptionOn(t *testing.T) {
	cfg := config.Config{AppKey: strings.Repeat("a", 64), HostMountRoot: t.TempDir()}
	st := newMemStore(t)
	s := mustSettings(t, st)
	s.EncryptionEnabled = true
	if err := st.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}
	svc := api.NewService(cfg, st, &fakeServiceDocker{}, fakeVirsh{}, &fakeResticEngine{})

	mode := svc.ModeFor(s)
	if !mode.Encrypted {
		t.Fatal("expected encrypted mode when EncryptionEnabled")
	}
	if mode.Password != restickey.Derive(cfg.AppKey) {
		t.Fatal("password must be derived from APP_KEY")
	}
}

func TestServiceModeEncryptionOff(t *testing.T) {
	cfg := config.Config{AppKey: strings.Repeat("a", 64), HostMountRoot: t.TempDir()}
	st := newMemStore(t)
	s := mustSettings(t, st)
	s.EncryptionEnabled = false
	if err := st.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}
	svc := api.NewService(cfg, st, &fakeServiceDocker{}, fakeVirsh{}, &fakeResticEngine{})

	mode := svc.ModeFor(s)
	if mode.Encrypted {
		t.Fatal("expected non-encrypted mode when EncryptionEnabled is off")
	}
	if mode.Password != "" {
		t.Fatal("password must be empty when encryption off")
	}
}

func TestDownloadFlashZip(t *testing.T) {
	cfg := config.Config{AppKey: strings.Repeat("a", 64), HostMountRoot: t.TempDir(), FlashDir: "/host/boot"}
	st := newMemStore(t)
	s := mustSettings(t, st)
	s.FlashPath = "backups/flash"
	if err := st.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}
	eng := &fakeResticEngine{snaps: []restic.Snapshot{{ID: "aaaa1111bbbb2222"}, {ID: "cccc3333dddd4444"}}}
	svc := api.NewService(cfg, st, &fakeServiceDocker{}, fakeVirsh{}, eng)

	t.Run("latest resolves to newest and streams zip bytes", func(t *testing.T) {
		var buf bytes.Buffer
		var resolved string
		if err := svc.DownloadFlashZip(context.Background(), "latest", "", func(id string) { resolved = id }, &buf); err != nil {
			t.Fatal(err)
		}
		if resolved != "cccc3333dddd4444" {
			t.Fatalf("expected newest id resolved, got %q", resolved)
		}
		if buf.Len() == 0 {
			t.Fatal("expected zip bytes to be streamed")
		}
	})

	t.Run("unknown id is rejected before any bytes or headers", func(t *testing.T) {
		var buf bytes.Buffer
		called := false
		err := svc.DownloadFlashZip(context.Background(), "deadbeef", "", func(string) { called = true }, &buf)
		if err == nil {
			t.Fatal("expected an error for an unknown snapshot id")
		}
		if called {
			t.Fatal("onResolved must not fire for an unknown id (headers would be wrongly committed)")
		}
		if buf.Len() != 0 {
			t.Fatal("no bytes may be written on a validation failure")
		}
	})
}

func TestBackupFlashReplicatesOffsite(t *testing.T) {
	mk := func(offsite string) (*fakeResticEngine, error) {
		dir := t.TempDir()
		root := filepath.ToSlash(dir)
		flashDir := root + "/boot"
		if err := os.MkdirAll(flashDir, 0o750); err != nil {
			t.Fatal(err)
		}
		cfg := config.Config{AppKey: strings.Repeat("a", 64), DataDir: dir, HostMountRoot: root, FlashDir: flashDir}
		st := newMemStore(t)
		s := mustSettings(t, st)
		s.FlashPath = "backups/flash"
		s.FlashOffsite = offsite
		if err := st.UpdateSettings(s); err != nil {
			t.Fatal(err)
		}
		eng := &fakeResticEngine{}
		svc := api.NewService(cfg, st, &fakeServiceDocker{}, fakeVirsh{}, eng)
		_, err := svc.BackupFlash(context.Background())
		return eng, err
	}

	t.Run("copies to off-site when configured", func(t *testing.T) {
		eng, err := mk("backups/flash-offsite")
		if err != nil {
			t.Fatal(err)
		}
		if len(eng.copied) != 1 {
			t.Fatalf("expected exactly one off-site copy, got %v", eng.copied)
		}
	})

	t.Run("no copy when off-site is blank", func(t *testing.T) {
		eng, err := mk("")
		if err != nil {
			t.Fatal(err)
		}
		if len(eng.copied) != 0 {
			t.Fatalf("expected no off-site copy, got %v", eng.copied)
		}
	})
}

// TestSnapshotsFlashRemoteOffsiteLists pins the fix for the off-site view being
// wrongly empty: a REMOTE off-site repo must be listed directly (no local
// config-file stat, which always fails for rest:/s3:/… and returned nil before).
func TestSnapshotsFlashRemoteOffsiteLists(t *testing.T) {
	cfg := config.Config{AppKey: strings.Repeat("a", 64), HostMountRoot: t.TempDir(), FlashDir: "/host/boot"}
	st := newMemStore(t)
	s := mustSettings(t, st)
	s.FlashPath = "backups/flash"
	s.FlashOffsite = "rest:http://nas:8000/flash" // remote off-site repo
	if err := st.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}
	eng := &fakeResticEngine{snaps: []restic.Snapshot{{ID: "aaaa1111bbbb2222"}}}
	svc := api.NewService(cfg, st, &fakeServiceDocker{}, fakeVirsh{}, eng)

	got, err := svc.SnapshotsFlash(context.Background(), "offsite")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("a remote off-site repo must be listed (not short-circuited to empty), got %d", len(got))
	}
}

// TestContainerMountsNoPhantomAppdata pins the fix for stateless containers
// showing a non-existent /mnt/user/appdata/<name> as a selected folder.
func TestContainerMountsNoPhantomAppdata(t *testing.T) {
	dir := t.TempDir()
	root := filepath.ToSlash(dir)
	cfg := config.Config{AppKey: strings.Repeat("a", 64), DataDir: dir, HostMountRoot: root, HostSourceRoot: "/mnt"}
	st := newMemStore(t)
	mustSettings(t, st)
	// A stateless container: no appdata bind mount, and no conventional appdata
	// folder exists on disk.
	d := &fakeServiceDocker{inspect: model.Inspect{Name: "/stateless", Image: "x:latest"}}
	svc := api.NewService(cfg, st, d, fakeVirsh{}, &fakeResticEngine{})

	mounts, custom, err := svc.ContainerMounts(context.Background(), "stateless")
	if err != nil {
		t.Fatal(err)
	}
	if len(custom) != 0 {
		t.Fatalf("a stateless container must not show a phantom appdata folder, got custom=%v", custom)
	}
	for _, m := range mounts {
		if m.Selected {
			t.Fatalf("no mount should be auto-selected for a stateless container, got %+v", m)
		}
	}
}

func TestOffsiteScheduleDecouplesFromBackup(t *testing.T) {
	dir := t.TempDir()
	root := filepath.ToSlash(dir)
	flashDir := root + "/boot"
	if err := os.MkdirAll(flashDir, 0o750); err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{AppKey: strings.Repeat("a", 64), DataDir: dir, HostMountRoot: root, FlashDir: flashDir}
	st := newMemStore(t)
	s := mustSettings(t, st)
	s.FlashPath = "backups/flash"
	s.FlashOffsite = "backups/flash-offsite"
	s.FlashOffsiteSchedule = "weekly Sun 03:00" // separate schedule → not after every backup
	if err := st.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}
	eng := &fakeResticEngine{}
	svc := api.NewService(cfg, st, &fakeServiceDocker{}, fakeVirsh{}, eng)

	if _, err := svc.BackupFlash(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(eng.copied) != 0 {
		t.Fatalf("with a separate off-site schedule, backup must NOT replicate, got %v", eng.copied)
	}

	// The scheduled/on-demand path replicates explicitly.
	if err := svc.ReplicateOffsite(context.Background(), "flash"); err != nil {
		t.Fatal(err)
	}
	if len(eng.copied) != 1 {
		t.Fatalf("ReplicateOffsite must copy once, got %v", eng.copied)
	}
}

func TestServiceBackupResolvesAppdataFromMounts(t *testing.T) {
	dir := t.TempDir()
	// HostMountRoot must be writable so EnsureRepo can create the repo dir, and
	// slash-separated so it matches the service's slash-based path logic on every
	// OS (Go's file ops accept forward slashes on Windows too). A literal
	// "/host/..." would hit a permission-denied mkdir on CI. Mount sources below
	// are placed under it so appdata resolution matches.
	root := filepath.ToSlash(dir)
	cfg := config.Config{AppKey: strings.Repeat("a", 64), DataDir: dir, HostMountRoot: root}
	st := newMemStore(t)
	s := mustSettings(t, st)
	s.EncryptionEnabled = false
	s.ContainersPath = "backups/containers"
	if err := st.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}

	// A container whose mount source is under <root>/appdata/plex.
	appdata := root + "/appdata/plex"
	if err := os.MkdirAll(appdata, 0o750); err != nil { // must exist (backup filters missing paths)
		t.Fatal(err)
	}
	d := &fakeServiceDocker{inspect: model.Inspect{
		Name:    "/plex",
		Image:   "plex:latest",
		Running: true,
		Mounts: []model.Mount{
			{Type: "bind", Source: appdata, Destination: "/config"},
			{Type: "bind", Source: "/etc/localtime", Destination: "/etc/localtime"}, // outside root → excluded
		},
	}}
	eng := &fakeResticEngine{}
	svc := api.NewService(cfg, st, d, fakeVirsh{}, eng)

	sum, err := svc.Backup(context.Background(), "plex")
	if err != nil {
		t.Fatalf("backup: %v", err)
	}
	if sum.SnapshotID != "deadbeef12345678" {
		t.Fatalf("snapshot id = %q", sum.SnapshotID)
	}
	if len(eng.backedUp) != 1 {
		t.Fatalf("expected 1 backup, got %d", len(eng.backedUp))
	}
	if !contains(eng.lastPaths, appdata) {
		t.Fatalf("appdata path not backed up: %v", eng.lastPaths)
	}
	for _, p := range eng.lastPaths {
		if p == "/etc/localtime" {
			t.Fatalf("out-of-root mount must be excluded: %v", eng.lastPaths)
		}
	}
	tg, err := st.GetTargetByContainer("plex")
	if err != nil {
		t.Fatalf("target not created: %v", err)
	}
	if tg.ContainerName != "plex" {
		t.Fatalf("target name = %q", tg.ContainerName)
	}
	// BytesAdded float64 → int64 bytes in the recorded run.
	runs, _ := st.ListRuns(10)
	if len(runs) == 0 || runs[0].Bytes != 2048 {
		t.Fatalf("expected recorded bytes 2048, got runs=%v", runs)
	}
	// Container must be restarted (orchestrator always-start contract).
	if !d.started {
		t.Fatal("container must be restarted after backup")
	}
}

// TestServiceBackupNoAppdataDefinitionOnly pins the forum fix: a stateless
// container with no existing source paths is backed up "definition-only" (its
// recreate recipe is captured) instead of failing with restic's "all source
// directories do not exist". restic is never called.
func TestServiceBackupNoAppdataDefinitionOnly(t *testing.T) {
	dir := t.TempDir()
	root := filepath.ToSlash(dir)
	cfg := config.Config{AppKey: strings.Repeat("a", 64), DataDir: dir, HostMountRoot: root}
	st := newMemStore(t)
	s := mustSettings(t, st)
	s.EncryptionEnabled = false
	s.ContainersPath = "backups/containers"
	if err := st.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}

	// No mounts, and the conventional appdata dir is NOT created → nothing exists.
	d := &fakeServiceDocker{inspect: model.Inspect{Name: "/bentopdf", Image: "bentopdf:latest", Running: true}}
	eng := &fakeResticEngine{}
	svc := api.NewService(cfg, st, d, fakeVirsh{}, eng)

	sum, err := svc.Backup(context.Background(), "bentopdf")
	if err != nil {
		t.Fatalf("backup should succeed (definition-only), got: %v", err)
	}
	if len(eng.backedUp) != 0 {
		t.Fatalf("restic must NOT run when there are no source paths, got %d calls", len(eng.backedUp))
	}
	if sum.SnapshotID != "" {
		t.Fatalf("definition-only backup should have no snapshot, got %q", sum.SnapshotID)
	}
	tg, err := st.GetTargetByContainer("bentopdf")
	if err != nil || tg.Definition == "" {
		t.Fatalf("definition should be captured for recreate-on-restore (tg=%+v err=%v)", tg, err)
	}
	if runs, _ := st.ListRuns(10); len(runs) == 0 || runs[0].Status != "success" {
		t.Fatalf("expected a recorded success run, got %v", runs)
	}
}

// backupTestService builds a service whose container Inspect resolves to an
// existing appdata path (so restic actually runs), with a progress store wired
// up. Used by the self-protection + batch tests.
func backupTestService(t *testing.T) (*api.Service, *fakeServiceDocker, *fakeResticEngine, *progress.Store) {
	t.Helper()
	dir := t.TempDir()
	root := filepath.ToSlash(dir)
	cfg := config.Config{AppKey: strings.Repeat("a", 64), DataDir: dir, HostMountRoot: root}
	st := newMemStore(t)
	s := mustSettings(t, st)
	s.EncryptionEnabled = false
	s.ContainersPath = "backups/containers"
	if err := st.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}
	// With HostSourceRoot unset, mount translation is identity-less, so path
	// resolution falls back to the conventional <root>/appdata/<name> dir — which
	// must exist for restic to run. Create one per container the batch tests use.
	for _, n := range []string{"plex", "radarr"} {
		if err := os.MkdirAll(root+"/appdata/"+n, 0o750); err != nil {
			t.Fatal(err)
		}
	}
	d := &fakeServiceDocker{inspect: model.Inspect{
		Name: "/app", Image: "app:latest", Running: true,
	}}
	eng := &fakeResticEngine{}
	prog := progress.NewStore()
	svc := api.NewService(cfg, st, d, fakeVirsh{}, eng)
	svc.SetProgress(prog)
	return svc, d, eng, prog
}

// waitBatchDone drains the progress store until the terminal "batch:containers"
// event (Active=false), or fails after a timeout. The channel receive of that
// event happens-after every Backup the batch goroutine ran, so callers may read
// the fakes race-free once it returns.
func waitBatchDone(t *testing.T, ch <-chan progress.Event) {
	t.Helper()
	deadline := time.After(5 * time.Second)
	for {
		select {
		case ev := <-ch:
			if ev.Key == "batch:containers" && !ev.Active {
				return
			}
		case <-deadline:
			t.Fatal("timed out waiting for batch to finish")
		}
	}
}

// TestBackupRefusesSelf pins the forum fix: BombVault must never back up its own
// container (stopping it mid-backup is suicide). With the self-container known,
// Backup returns ErrSelfBackup and never touches Docker's lifecycle.
func TestBackupRefusesSelf(t *testing.T) {
	t.Setenv("BOMBVAULT_SELF_CONTAINER", "BombVault")
	svc, d, eng, _ := backupTestService(t)

	_, err := svc.Backup(context.Background(), "BombVault")
	if !errors.Is(err, api.ErrSelfBackup) {
		t.Fatalf("want ErrSelfBackup, got %v", err)
	}
	if len(eng.backedUp) != 0 {
		t.Fatalf("self-backup must not run restic, got %d", len(eng.backedUp))
	}
	for _, c := range d.calls {
		if strings.HasPrefix(c, "stop:") {
			t.Fatalf("self-backup must never stop a container, calls=%v", d.calls)
		}
	}
}

// TestStartBackupAllSkipsSelfRunsOthers verifies the server-side batch backs up
// every selected container EXCEPT BombVault itself, independent of the request.
func TestStartBackupAllSkipsSelfRunsOthers(t *testing.T) {
	t.Setenv("BOMBVAULT_SELF_CONTAINER", "BombVault")
	svc, _, eng, store := backupTestService(t)
	ch, cancel := store.Subscribe()
	defer cancel()

	if !svc.StartBackupAll(context.Background(), []string{"BombVault", "plex", "radarr"}) {
		t.Fatal("StartBackupAll should start")
	}
	waitBatchDone(t, ch)

	if len(eng.backedUp) != 2 {
		t.Fatalf("want 2 backups (self skipped), got %d", len(eng.backedUp))
	}
}

// TestStartBackupAllRejectsConcurrent pins the single-batch (409) guard: while a
// batch is in flight, a second StartBackupAll returns false.
func TestStartBackupAllRejectsConcurrent(t *testing.T) {
	t.Setenv("BOMBVAULT_SELF_CONTAINER", "BombVault")
	svc, _, eng, store := backupTestService(t)
	eng.block = make(chan struct{}) // hold the first batch inside restic Backup
	ch, cancel := store.Subscribe()
	defer cancel()

	if !svc.StartBackupAll(context.Background(), []string{"plex"}) {
		t.Fatal("first batch should start")
	}
	// The flag is set synchronously by StartBackupAll, so the second call sees a
	// run in flight regardless of goroutine scheduling.
	if svc.StartBackupAll(context.Background(), []string{"radarr"}) {
		t.Fatal("second concurrent batch must be rejected")
	}
	close(eng.block) // let the first batch finish, then wait so cleanup is safe
	waitBatchDone(t, ch)
}

// TestServiceBackupRefusesEmptyWhenPriorDataVanishes pins the silent-no-op fix: a
// container that PREVIOUSLY backed up data but now resolves to no paths (its
// appdata share went missing) must be refused, not recorded as a successful empty
// backup that overwrites the stored path list. A first backup is NOT affected.
func TestServiceBackupRefusesEmptyWhenPriorDataVanishes(t *testing.T) {
	dir := t.TempDir()
	root := filepath.ToSlash(dir)
	cfg := config.Config{AppKey: strings.Repeat("a", 64), DataDir: dir, HostMountRoot: root}
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
	d := &fakeServiceDocker{inspect: model.Inspect{
		Name: "/plex", Image: "plex:latest", Running: true,
		Mounts: []model.Mount{{Type: "bind", Source: appdata, Destination: "/config"}},
	}}
	eng := &fakeResticEngine{}
	svc := api.NewService(cfg, st, d, fakeVirsh{}, eng)

	// First backup captures data and records the path (so the target "expects data").
	if _, err := svc.Backup(context.Background(), "plex"); err != nil {
		t.Fatalf("first backup should succeed: %v", err)
	}
	if len(eng.backedUp) != 1 {
		t.Fatalf("first backup should run restic once, got %d", len(eng.backedUp))
	}

	// The appdata share goes missing → the next backup resolves to no paths.
	if err := os.RemoveAll(appdata); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Backup(context.Background(), "plex"); err == nil || !strings.Contains(err.Error(), "not reachable") {
		t.Fatalf("expected refusal once prior data vanished, got %v", err)
	}
	if len(eng.backedUp) != 1 {
		t.Fatalf("restic must NOT run for the empty re-backup, got %d total", len(eng.backedUp))
	}
}

// TestServiceBackupFirstTimeEmptyIsDefinitionOnly pins the false-positive guard:
// the FIRST backup of a container with no resolvable paths yet (new container,
// appdata not created) is a definition-only success, never refused.
func TestServiceBackupFirstTimeEmptyIsDefinitionOnly(t *testing.T) {
	dir := t.TempDir()
	root := filepath.ToSlash(dir)
	cfg := config.Config{AppKey: strings.Repeat("a", 64), DataDir: dir, HostMountRoot: root}
	st := newMemStore(t)
	s := mustSettings(t, st)
	s.EncryptionEnabled = false
	s.ContainersPath = "backups/containers"
	if err := st.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}
	// Appdata mount present, but the source dir does not exist yet (brand-new app).
	d := &fakeServiceDocker{inspect: model.Inspect{
		Name: "/newapp", Image: "newapp:latest", Running: true,
		Mounts: []model.Mount{{Type: "bind", Source: root + "/appdata/newapp", Destination: "/config"}},
	}}
	eng := &fakeResticEngine{}
	svc := api.NewService(cfg, st, d, fakeVirsh{}, eng)

	sum, err := svc.Backup(context.Background(), "newapp")
	if err != nil {
		t.Fatalf("first backup of a new container must not be refused: %v", err)
	}
	if sum.SnapshotID != "" || len(eng.backedUp) != 0 {
		t.Fatalf("expected a definition-only backup (no restic), got sum=%+v calls=%d", sum, len(eng.backedUp))
	}
}

// TestServiceContainerMountsAndSelection covers the backup-folder selector:
// listing a container's bind mounts (appdata default selected, others not, an
// out-of-mount bind marked unreachable), storing an explicit selection, and that
// a subsequent backup honours it. Host paths equal container paths here because
// HostSourceRoot == HostMountRoot (identity translation).
func TestServiceContainerMountsAndSelection(t *testing.T) {
	dir := t.TempDir()
	root := filepath.ToSlash(dir)
	cfg := config.Config{
		AppKey: strings.Repeat("a", 64), DataDir: dir,
		HostMountRoot: root, HostSourceRoot: "/mnt", // host /mnt mounted at <root> (mirrors box-gate)
	}
	st := newMemStore(t)
	s := mustSettings(t, st)
	s.EncryptionEnabled = false
	s.ContainersPath = "backups/containers"
	if err := st.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}

	// HOST paths (what docker reports + what the UI shows) and their translated
	// container paths under <root>.
	appdataHost, mediaHost := "/mnt/user/appdata/plex", "/mnt/user/media"
	mediaCP := root + "/user/media"
	// Both selected dirs must exist (backup filters out missing source paths).
	for _, p := range []string{root + "/user/appdata/plex", mediaCP} {
		if err := os.MkdirAll(p, 0o750); err != nil {
			t.Fatal(err)
		}
	}
	d := &fakeServiceDocker{inspect: model.Inspect{
		Name: "/plex", Image: "plex:latest", Running: true,
		Mounts: []model.Mount{
			{Type: "bind", Source: appdataHost, Destination: "/config"},
			{Type: "bind", Source: mediaHost, Destination: "/media"},
			{Type: "bind", Source: "/etc/localtime", Destination: "/etc/localtime"}, // outside /mnt
		},
	}}
	eng := &fakeResticEngine{}
	svc := api.NewService(cfg, st, d, fakeVirsh{}, eng)
	ctx := context.Background()

	// Default selection: appdata selected, media not, localtime unreachable.
	mounts, custom, err := svc.ContainerMounts(ctx, "plex")
	if err != nil {
		t.Fatalf("ContainerMounts: %v", err)
	}
	if len(mounts) != 3 || len(custom) != 0 {
		t.Fatalf("mounts=%d custom=%d", len(mounts), len(custom))
	}
	byDest := map[string]api.MountInfo{}
	for _, m := range mounts {
		byDest[m.Dest] = m
	}
	if !byDest["/config"].Selected || !byDest["/config"].IsAppdata || !byDest["/config"].Reachable {
		t.Fatalf("appdata mount: %+v", byDest["/config"])
	}
	if byDest["/media"].Selected || byDest["/media"].IsAppdata || !byDest["/media"].Reachable {
		t.Fatalf("media mount: %+v", byDest["/media"])
	}
	if byDest["/etc/localtime"].Reachable {
		t.Fatalf("out-of-mount bind should be unreachable: %+v", byDest["/etc/localtime"])
	}

	// Storing an explicit selection (host paths) flips media to selected.
	if err := svc.SetBackupPaths(ctx, "plex", []string{appdataHost, mediaHost}); err != nil {
		t.Fatalf("SetBackupPaths: %v", err)
	}
	mounts, _, _ = svc.ContainerMounts(ctx, "plex")
	for _, m := range mounts {
		if m.Dest == "/media" && !m.Selected {
			t.Fatal("media should be selected after SetBackupPaths")
		}
	}

	// An unreachable path is rejected.
	if err := svc.SetBackupPaths(ctx, "plex", []string{"/etc/localtime"}); err == nil {
		t.Fatal("SetBackupPaths must reject a path outside the host mount")
	}

	// A backup now uses the explicit selection (includes media).
	if _, err := svc.Backup(ctx, "plex"); err != nil {
		t.Fatalf("backup: %v", err)
	}
	if !contains(eng.lastPaths, mediaCP) {
		t.Fatalf("selected media not backed up: %v", eng.lastPaths)
	}
}

// TestServiceBackupTranslatesHostAppdataPath pins the box-gate fix: the broad
// mount is host /mnt → container /host/user, so host /mnt/user/appdata/<x> is
// reachable at /host/user/USER/appdata/<x> (note the extra "user" segment). Docker
// reports the bind source as the HOST path; BombVault translates it via
// HOST_SOURCE_ROOT (=/mnt) → HOST_MOUNT_ROOT and backs up the real, correctly
// cased dir — not a guess. Non-appdata binds (media) are excluded.
func TestServiceBackupTranslatesHostAppdataPath(t *testing.T) {
	dir := t.TempDir()
	root := filepath.ToSlash(dir)
	cfg := config.Config{
		AppKey:         strings.Repeat("a", 64),
		DataDir:        dir,
		HostMountRoot:  root,   // container side; the whole host /mnt is mounted here
		HostSourceRoot: "/mnt", // the full /mnt is mounted (covers /mnt/user + cache pools)
	}
	st := newMemStore(t)
	s := mustSettings(t, st)
	s.EncryptionEnabled = false
	s.ContainersPath = "backups/containers"
	if err := st.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}
	// The translated appdata dirs must exist (backup filters out missing paths).
	for _, p := range []string{root + "/user/appdata/pingvin_share_x/data", root + "/user/appdata/pingvin_share_x/images"} {
		if err := os.MkdirAll(p, 0o750); err != nil {
			t.Fatal(err)
		}
	}

	// Exactly the box-gate container: appdata binds under /mnt/user/appdata (real
	// lowercase dir though the name is mixed-case) + a media bind that must NOT be
	// backed up. Translation must yield <root>/user/appdata/... (the extra "user").
	d := &fakeServiceDocker{inspect: model.Inspect{
		Name:  "/Pingvin-Share-X",
		Image: "smp46/pingvin-share-x:latest",
		Mounts: []model.Mount{
			{Type: "bind", Source: "/mnt/user/appdata/pingvin_share_x/data", Destination: "/opt/app/backend/data"},
			{Type: "bind", Source: "/mnt/user/appdata/pingvin_share_x/images", Destination: "/opt/app/frontend/public/img"},
			{Type: "bind", Source: "/mnt/user/Media", Destination: "/media"}, // not appdata → excluded
			{Type: "bind", Source: "/etc/localtime", Destination: "/etc/localtime"},
		},
	}}
	eng := &fakeResticEngine{}
	svc := api.NewService(cfg, st, d, fakeVirsh{}, eng)

	if _, err := svc.Backup(context.Background(), "Pingvin-Share-X"); err != nil {
		t.Fatalf("backup: %v", err)
	}
	for _, want := range []string{
		root + "/user/appdata/pingvin_share_x/data",
		root + "/user/appdata/pingvin_share_x/images",
	} {
		if !contains(eng.lastPaths, want) {
			t.Fatalf("expected translated container path %q, got %v", want, eng.lastPaths)
		}
	}
	for _, p := range eng.lastPaths {
		if strings.Contains(p, "Media") || p == "/etc/localtime" {
			t.Fatalf("non-appdata mount must be excluded, got %v", eng.lastPaths)
		}
	}
}

// TestServiceSetIncludeFindOrCreate verifies that SetInclude creates the target
// row when it does not exist yet, rather than returning an error.
func TestServiceSetIncludeFindOrCreate(t *testing.T) {
	dir := t.TempDir()
	root := filepath.ToSlash(dir)
	cfg := config.Config{AppKey: strings.Repeat("a", 64), DataDir: dir, HostMountRoot: root, HostSourceRoot: "/mnt"}
	st := newMemStore(t)
	s := mustSettings(t, st)
	s.ContainersPath = "backups/containers"
	if err := st.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}

	// Realistic appdata bind mount: host /mnt/appdata/radarr → container
	// <root>/appdata/radarr (the mount branch captures it from inspect).
	appdata := root + "/appdata/radarr"
	if err := os.MkdirAll(appdata, 0o750); err != nil {
		t.Fatal(err)
	}
	d := &fakeServiceDocker{inspect: model.Inspect{
		Name:  "/radarr",
		Image: "radarr:latest",
		Mounts: []model.Mount{
			{Type: "bind", Source: "/mnt/appdata/radarr", Destination: "/config"},
		},
	}}
	svc := api.NewService(cfg, st, d, fakeVirsh{}, &fakeResticEngine{})

	// No target exists — SetInclude must find-or-create it.
	if err := svc.SetInclude(context.Background(), "radarr", true); err != nil {
		t.Fatalf("SetInclude (find-or-create): %v", err)
	}
	tg, err := st.GetTargetByContainer("radarr")
	if err != nil {
		t.Fatalf("target must have been created: %v", err)
	}
	if !tg.IncludeInSchedule {
		t.Fatal("include flag must be true after SetInclude")
	}
	if !contains(tg.AppdataPaths, appdata) {
		t.Fatalf("expected appdata path from inspect, got %v", tg.AppdataPaths)
	}

	// Calling again (target already exists) must be idempotent.
	if err := svc.SetInclude(context.Background(), "radarr", false); err != nil {
		t.Fatalf("SetInclude (already exists): %v", err)
	}
	tg2, err := st.GetTargetByContainer("radarr")
	if err != nil {
		t.Fatal(err)
	}
	if tg2.IncludeInSchedule {
		t.Fatal("include flag must be false after second SetInclude")
	}
}

// TestServiceSetIncludeInspectFailFallback verifies that SetInclude still
// succeeds when docker inspect fails (a fallback path is used).
func TestServiceSetIncludeInspectFailFallback(t *testing.T) {
	dir := t.TempDir()
	cfg := config.Config{AppKey: strings.Repeat("a", 64), DataDir: dir, HostMountRoot: "/host/user"}
	st := newMemStore(t)

	d := &fakeServiceDocker{inspectErr: errors.New("no such container")}
	svc := api.NewService(cfg, st, d, fakeVirsh{}, &fakeResticEngine{})

	if err := svc.SetInclude(context.Background(), "unknown", true); err != nil {
		t.Fatalf("SetInclude must not fail when inspect errors: %v", err)
	}
	tg, err := st.GetTargetByContainer("unknown")
	if err != nil {
		t.Fatalf("target must have been created via fallback: %v", err)
	}
	if !tg.IncludeInSchedule {
		t.Fatal("include flag must be true")
	}
}

func TestServiceSnapshotsFilteredByContainer(t *testing.T) {
	dir := t.TempDir()
	// HostMountRoot is the test temp dir so the resolved repo lives under it and
	// the initialised-repo marker can be created.
	cfg := config.Config{AppKey: strings.Repeat("a", 64), DataDir: dir, HostMountRoot: dir}
	st := newMemStore(t)
	s := mustSettings(t, st)
	s.ContainersPath = "backups/containers"
	if err := st.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}
	// Mark the repo as initialised so Snapshots calls the engine (a never-backed-up
	// repo returns an empty list, exercised elsewhere).
	repo := filepath.Join(dir, "backups", "containers")
	if err := os.MkdirAll(repo, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "config"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	// The single repo holds snapshots for multiple containers; the per-container
	// endpoint must only return the ones tagged container:<name>.
	eng := &fakeResticEngine{snaps: []restic.Snapshot{
		{ID: "aaaa1111", Tags: []string{"container:plex", "p1"}},
		{ID: "bbbb2222", Tags: []string{"container:sonarr", "p1"}},
		{ID: "cccc3333", Tags: []string{"container:plex", "p1"}},
		{ID: "dddd4444", Tags: nil}, // untagged → excluded
	}}
	svc := api.NewService(cfg, st, &fakeServiceDocker{}, fakeVirsh{}, eng)

	got, err := svc.Snapshots(context.Background(), "plex", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 plex snapshots, got %d: %+v", len(got), got)
	}
	for _, s := range got {
		if !contains(s.Tags, "container:plex") {
			t.Fatalf("returned a non-plex snapshot: %+v", s)
		}
	}
}

// TestListSnapshotFilesScopedToContainer pins the access-control fix: the
// file-listing endpoint only lists files of a snapshot that belongs to the named
// container, so one container's tree can't be browsed through another's route.
func TestListSnapshotFilesScopedToContainer(t *testing.T) {
	dir := t.TempDir()
	cfg := config.Config{AppKey: strings.Repeat("a", 64), DataDir: dir, HostMountRoot: dir}
	st := newMemStore(t)
	s := mustSettings(t, st)
	s.ContainersPath = "backups/containers"
	if err := st.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}
	repo := filepath.ToSlash(dir) + "/backups/containers"
	if err := os.MkdirAll(repo, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "config"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	eng := &fakeResticEngine{
		snaps: []restic.Snapshot{
			{ID: "aaaa1111", Tags: []string{"container:plex"}},
			{ID: "bbbb2222", Tags: []string{"container:sonarr"}},
		},
		lsEntries: []restic.FileEntry{{}},
	}
	svc := api.NewService(cfg, st, &fakeServiceDocker{}, fakeVirsh{}, eng)

	// plex's own snapshot lists.
	if files, err := svc.ListSnapshotFiles(context.Background(), "plex", "aaaa1111", ""); err != nil || len(files) != 1 {
		t.Fatalf("own snapshot must list files: files=%v err=%v", files, err)
	}
	// sonarr's snapshot must NOT be listable via plex's route.
	if _, err := svc.ListSnapshotFiles(context.Background(), "plex", "bbbb2222", ""); err == nil || !strings.Contains(err.Error(), "does not belong") {
		t.Fatalf("foreign snapshot must be refused, got %v", err)
	}
}

// TestDeleteBackupsForgetsSnapshotsAndTarget verifies that deleting a container's
// backups forgets only that container's snapshots (tag-filtered) and removes its
// target from the store — the path used to clean up no-longer-installed containers.
func TestDeleteBackupsForgetsSnapshotsAndTarget(t *testing.T) {
	dir := t.TempDir()
	cfg := config.Config{AppKey: strings.Repeat("a", 64), DataDir: dir, HostMountRoot: dir}
	st := newMemStore(t)
	s := mustSettings(t, st)
	s.ContainersPath = "backups/containers"
	if err := st.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}
	// Mark the repo initialised so Snapshots reaches the engine.
	repo := filepath.Join(dir, "backups", "containers")
	if err := os.MkdirAll(repo, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "config"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := st.UpsertTarget(store.Target{ContainerName: "plex", AppdataPaths: []string{"/x"}}); err != nil {
		t.Fatal(err)
	}

	eng := &fakeResticEngine{snaps: []restic.Snapshot{
		{ID: "aaaa1111", Tags: []string{"container:plex", "p1"}},
		{ID: "bbbb2222", Tags: []string{"container:sonarr", "p1"}}, // other container — must be left alone
		{ID: "cccc3333", Tags: []string{"container:plex", "p1"}},
	}}
	svc := api.NewService(cfg, st, &fakeServiceDocker{}, fakeVirsh{}, eng)

	if err := svc.DeleteBackups(context.Background(), "plex"); err != nil {
		t.Fatalf("DeleteBackups: %v", err)
	}

	// Only plex's snapshots are forgotten.
	if len(eng.forgotten) != 2 || !contains(eng.forgotten, "aaaa1111") || !contains(eng.forgotten, "cccc3333") {
		t.Fatalf("expected aaaa1111+cccc3333 forgotten, got %v", eng.forgotten)
	}
	if contains(eng.forgotten, "bbbb2222") {
		t.Fatalf("forgot another container's snapshot: %v", eng.forgotten)
	}
	// Target is gone.
	if _, err := st.GetTargetByContainer("plex"); err == nil {
		t.Fatal("expected target to be deleted")
	}
}

// TestRestoreUsesStoredDefinitionWhenContainerDeleted verifies the core
// disaster-recovery fix: if the container no longer exists on the host,
// Restore falls back to the definition persisted at backup time and
// successfully recreates the container from it.
func TestRestoreUsesStoredDefinitionWhenContainerDeleted(t *testing.T) {
	dir := t.TempDir()
	// Container paths are Linux-absolute under the host mount root; the restore
	// uses fakes (no real FS access to these paths), so a fixed Linux root is fine.
	cfg := config.Config{
		AppKey:        strings.Repeat("a", 64),
		DataDir:       dir,
		HostMountRoot: "/host/user",
		// FlashTemplatesDir must be writable — use a temp subdir.
		FlashTemplatesDir: filepath.Join(dir, "flash"),
	}
	st := newMemStore(t)
	s := mustSettings(t, st)
	s.EncryptionEnabled = false
	s.ContainersPath = "backups/containers"
	if err := st.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}

	// Seed a target with a stored definition containing the recreate recipe.
	storedInspect := model.Inspect{
		Name:  "/Pingvin-Share-X",
		Image: "sha256:abc123",
		Config: model.Config{
			Image: "smp46/pingvin-share-x:latest",
		},
	}
	defBytes, err := marshalDefinition(storedInspect, "<xml/>")
	if err != nil {
		t.Fatalf("marshal definition: %v", err)
	}
	tg, err := st.UpsertTarget(store.Target{
		ContainerName: "Pingvin-Share-X",
		AppdataPaths:  []string{"/host/user/user/appdata/pingvin_share_x"},
		Definition:    string(defBytes),
	})
	if err != nil {
		t.Fatalf("seed target: %v", err)
	}

	// Seed a dummy run so Start/Finish have a valid target_id reference.
	_ = tg

	// Docker fake: Inspect returns an error (container deleted); InspectName
	// returns ("", nil) meaning "container absent — fresh restore is fine".
	d := &fakeServiceDocker{
		inspectErr: errors.New("No such container: Pingvin-Share-X"),
		liveName:   "", // absent
	}
	// The snapshot must exist for the restore preflight (VerifySnapshot) to pass.
	eng := &fakeResticEngine{snaps: []restic.Snapshot{{ID: "deadbeef"}}}
	svc := api.NewService(cfg, st, d, fakeVirsh{}, eng)

	// Use a valid 8-hex snapshot id to pass the orchestrator's regex guard.
	restoreErr := svc.Restore(context.Background(), "Pingvin-Share-X", "deadbeef", true, "")
	if restoreErr != nil {
		t.Fatalf("restore must succeed with stored definition: %v", restoreErr)
	}

	// CreateAndStart must have been called.
	if d.createdIn.Config.Image == "" {
		t.Fatal("CreateAndStart was not called")
	}
	// The image must come from the STORED definition, not the live (failed) inspect.
	if d.createdIn.Config.Image != "smp46/pingvin-share-x:latest" {
		t.Fatalf("recreated with wrong image %q; want smp46/pingvin-share-x:latest", d.createdIn.Config.Image)
	}
	// The live Inspect must NOT have been called (container is deleted).
	for _, c := range d.calls {
		if c == "inspect:Pingvin-Share-X" {
			t.Fatal("live Inspect must not be called when stored definition is available")
		}
	}
	// Restic restore must have been called with the correct snapshot id.
	if len(eng.restored) == 0 {
		t.Fatal("restic restore was not called")
	}
	if !strings.Contains(eng.restored[0], "deadbeef") {
		t.Fatalf("restic restore called with wrong snapshot id: %v", eng.restored)
	}
}

// TestDiscoverRebuildsTargetsFromStorage verifies full disaster recovery: with
// an empty store (fresh install), Discover reads the encrypted definitions from
// the backup storage + the repo's container tags and rebuilds the targets.
func TestDiscoverRebuildsTargetsFromStorage(t *testing.T) {
	dir := t.TempDir()
	cfg := config.Config{AppKey: strings.Repeat("a", 64), DataDir: dir, HostMountRoot: dir}
	st := newMemStore(t)
	s := mustSettings(t, st)
	s.ContainersPath = "backups/containers"
	if err := st.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}
	// Repo exists (config marker) so Discover reaches the engine.
	repo := filepath.Join(dir, "backups", "containers")
	if err := os.MkdirAll(repo, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "config"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	// Encrypted definition mirrored to the defs dir (sibling of the repo).
	defsDir := filepath.Join(dir, "backups", "bombvault-defs")
	if err := os.MkdirAll(defsDir, 0o700); err != nil {
		t.Fatal(err)
	}
	defJSON, err := marshalDefinition(
		model.Inspect{Name: "/plex", Config: model.Config{Image: "plex:latest"}},
		"<xml/>", "/host/user/appdata/plex",
	)
	if err != nil {
		t.Fatal(err)
	}
	enc, err := secret.Encrypt(cfg.AppKey, defJSON)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(defsDir, "plex.def"), enc, 0o600); err != nil {
		t.Fatal(err)
	}

	// The repo reports a data snapshot tagged container:plex (+ one with no def).
	eng := &fakeResticEngine{snaps: []restic.Snapshot{
		{ID: "aaaa1111", Tags: []string{"container:plex", "p1"}},
		{ID: "bbbb2222", Tags: []string{"container:ghost", "p1"}}, // no def file → skipped
	}}
	svc := api.NewService(cfg, st, &fakeServiceDocker{}, fakeVirsh{}, eng)

	n, err := svc.Discover(context.Background())
	if err != nil {
		t.Fatalf("discover: %v", err)
	}
	if n != 1 {
		t.Fatalf("discovered = %d, want 1 (ghost has no def, skipped)", n)
	}
	tg, err := st.GetTargetByContainer("plex")
	if err != nil {
		t.Fatalf("plex target not rebuilt: %v", err)
	}
	if len(tg.AppdataPaths) != 1 || tg.AppdataPaths[0] != "/host/user/appdata/plex" {
		t.Fatalf("rebuilt appdata = %v", tg.AppdataPaths)
	}
	if tg.Definition == "" {
		t.Fatal("rebuilt target has no definition")
	}
}

// marshalDefinition is a test helper that encodes a containerDefinition JSON
// blob without importing the unexported type from package api.
// The struct layout mirrors api.containerDefinition exactly.
func marshalDefinition(inspect model.Inspect, templateXML string, appdata ...string) ([]byte, error) {
	type def struct {
		Inspect      model.Inspect `json:"inspect"`
		TemplateXML  string        `json:"template_xml"`
		AppdataPaths []string      `json:"appdata_paths"`
	}
	return json.Marshal(def{Inspect: inspect, TemplateXML: templateXML, AppdataPaths: appdata})
}

// ---------------------------------------------------------------------------
// fakes
// ---------------------------------------------------------------------------

type fakeResticEngine struct {
	inited          []string
	backedUp        []string
	lastPaths       []string
	restored        []string
	forgotten       []string
	prunedRepos     []string
	checked         []string
	copied          []string
	copyErr         error
	snaps           []restic.Snapshot
	lsEntries       []restic.FileEntry
	unlockedRepos   []string
	unlockRemoveAll []bool
	manualPruned    []string
	snapshotsCalls  int
	snapshotsErr    error
	initErr         error
	backupErr       error
	dumpErr         error
	forgetPolicyErr error
	checkErr        error
	unlockErr       error
	// block, when non-nil, makes Backup wait on it — lets a test hold a batch
	// run "in flight" to exercise the single-batch (409) guard deterministically.
	block chan struct{}
	// existingMode, when non-nil, simulates an already-created repo of that
	// encryption mode: RepoOpens then returns true only for a probe whose mode
	// matches. When nil, RepoOpens mirrors a local repo and "opens" once restic's
	// `config` marker exists on disk (mode-agnostic).
	existingMode *bool
}

func (f *fakeResticEngine) Init(_ context.Context, repo string, _ restic.Mode) error {
	f.inited = append(f.inited, repo)
	return f.initErr
}

func (f *fakeResticEngine) RepoOpens(_ context.Context, repo string, m restic.Mode) bool {
	// Simulated existing repo of a pinned mode: opens only when the probe mode
	// matches (lets a test exercise the encryption-mode-mismatch path).
	if f.existingMode != nil {
		return m.Encrypted == *f.existingMode
	}
	// Otherwise mirror a real local repo: it "opens" once restic's config marker
	// exists on disk, regardless of mode. Keeps the idempotency test meaningful.
	_, err := os.Stat(filepath.Join(repo, "config"))
	return err == nil
}

func (f *fakeResticEngine) Backup(_ context.Context, repo string, paths, _ []string, _ restic.Mode) (restic.Summary, error) {
	if f.block != nil {
		<-f.block
	}
	f.backedUp = append(f.backedUp, repo)
	f.lastPaths = paths
	if f.backupErr != nil {
		return restic.Summary{}, f.backupErr
	}
	return restic.Summary{SnapshotID: "deadbeef12345678", BytesAdded: 2048}, nil
}

func (f *fakeResticEngine) RestorePath(_ context.Context, repo, snapshotID, path string, _ restic.Mode) error {
	f.restored = append(f.restored, repo+":"+snapshotID+":"+path)
	return nil
}

func (f *fakeResticEngine) DumpZip(_ context.Context, repo, snapshotID, subfolder string, w io.Writer, _ restic.Mode) error {
	f.restored = append(f.restored, repo+":"+snapshotID+":"+subfolder)
	if f.dumpErr != nil {
		return f.dumpErr
	}
	_, _ = w.Write([]byte("PK\x03\x04zip")) // minimal zip-magic stand-in
	return nil
}

func (f *fakeResticEngine) Snapshots(_ context.Context, _ string, _ restic.Mode) ([]restic.Snapshot, error) {
	f.snapshotsCalls++
	if f.snapshotsErr != nil {
		e := f.snapshotsErr
		f.snapshotsErr = nil // fail once, then succeed (exercises the stale-unlock retry)
		return nil, e
	}
	return f.snaps, nil
}

func (f *fakeResticEngine) Forget(_ context.Context, _ string, snapshotIDs []string, _ bool, _ restic.Mode) error {
	f.forgotten = append(f.forgotten, snapshotIDs...)
	return nil
}

func (f *fakeResticEngine) ForgetPolicy(_ context.Context, repo string, p restic.RetentionPolicy, _ restic.Mode) error {
	if p.Any() {
		f.prunedRepos = append(f.prunedRepos, repo)
	}
	return f.forgetPolicyErr
}

func (f *fakeResticEngine) Ls(_ context.Context, _, _ string, _ restic.Mode) ([]restic.FileEntry, error) {
	return f.lsEntries, nil
}

func (f *fakeResticEngine) RestoreInclude(_ context.Context, repo, snapshotID, includePath, target string, _ restic.Mode) error {
	f.restored = append(f.restored, repo+":"+snapshotID+":"+includePath+"->"+target)
	return nil
}

func (f *fakeResticEngine) Check(_ context.Context, repo string, _ restic.Mode) error {
	f.checked = append(f.checked, repo)
	return f.checkErr
}

func (f *fakeResticEngine) Unlock(_ context.Context, repo string, removeAll bool, _ restic.Mode) error {
	f.unlockedRepos = append(f.unlockedRepos, repo)
	f.unlockRemoveAll = append(f.unlockRemoveAll, removeAll)
	return f.unlockErr
}

func (f *fakeResticEngine) Prune(_ context.Context, repo string, _ restic.Mode) error {
	f.manualPruned = append(f.manualPruned, repo)
	return nil
}

func (f *fakeResticEngine) Copy(_ context.Context, destRepo, srcRepo string, _ []string, _ restic.Mode) error {
	f.copied = append(f.copied, srcRepo+"->"+destRepo)
	return f.copyErr
}

// initRepoSvc builds a service whose containers repo is marked initialised, so
// repo-management methods reach the engine instead of the "not created yet" guard.
func initRepoSvc(t *testing.T, eng *fakeResticEngine) *api.Service {
	t.Helper()
	dir := t.TempDir()
	cfg := config.Config{AppKey: strings.Repeat("a", 64), DataDir: dir, HostMountRoot: dir}
	st := newMemStore(t)
	s := mustSettings(t, st)
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
	return api.NewService(cfg, st, &fakeServiceDocker{}, fakeVirsh{}, eng)
}

func TestUnlockDomainRemovesAllLocks(t *testing.T) {
	eng := &fakeResticEngine{}
	svc := initRepoSvc(t, eng)
	if err := svc.UnlockDomain(context.Background(), "containers", ""); err != nil {
		t.Fatalf("UnlockDomain: %v", err)
	}
	if len(eng.unlockedRepos) != 1 || len(eng.unlockRemoveAll) != 1 || !eng.unlockRemoveAll[0] {
		t.Fatalf("expected one unlock with removeAll=true, got repos=%v removeAll=%v", eng.unlockedRepos, eng.unlockRemoveAll)
	}
}

func TestUnlockDomainNoRepoYet(t *testing.T) {
	eng := &fakeResticEngine{}
	dir := t.TempDir()
	cfg := config.Config{AppKey: strings.Repeat("a", 64), DataDir: dir, HostMountRoot: dir}
	st := newMemStore(t)
	s := mustSettings(t, st)
	s.ContainersPath = "backups/containers" // never initialised (no config marker)
	if err := st.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}
	svc := api.NewService(cfg, st, &fakeServiceDocker{}, fakeVirsh{}, eng)
	if err := svc.UnlockDomain(context.Background(), "containers", ""); err == nil {
		t.Fatal("expected a friendly error when the repo does not exist yet")
	}
	if len(eng.unlockedRepos) != 0 {
		t.Fatalf("must not call unlock on a non-existent repo: %v", eng.unlockedRepos)
	}
}

// TestPruneDomainCallsPrune: with NO retention policy set, Prune is a plain
// space-reclaim (restic prune) and must NOT forget anything.
func TestPruneDomainCallsPrune(t *testing.T) {
	eng := &fakeResticEngine{}
	svc := initRepoSvc(t, eng)
	if err := svc.PruneDomain(context.Background(), "containers", ""); err != nil {
		t.Fatalf("PruneDomain: %v", err)
	}
	if len(eng.manualPruned) != 1 {
		t.Fatalf("expected one prune, got %v", eng.manualPruned)
	}
	if len(eng.prunedRepos) != 0 {
		t.Fatalf("without a policy, Prune must not apply retention, got %v", eng.prunedRepos)
	}
}

// TestPruneDomainAppliesRetentionWhenSet: with a retention policy configured,
// Prune APPLIES it (forget --keep-* --prune) so it collapses snapshots per the
// policy, not just a plain space-reclaim.
func TestPruneDomainAppliesRetentionWhenSet(t *testing.T) {
	dir := t.TempDir()
	cfg := config.Config{AppKey: strings.Repeat("a", 64), DataDir: dir, HostMountRoot: dir}
	st := newMemStore(t)
	s := mustSettings(t, st)
	s.ContainersPath = "backups/containers"
	s.RetentionKeepDaily = 14 // a policy is set
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
	eng := &fakeResticEngine{}
	svc := api.NewService(cfg, st, &fakeServiceDocker{}, fakeVirsh{}, eng)

	if err := svc.PruneDomain(context.Background(), "containers", ""); err != nil {
		t.Fatalf("PruneDomain: %v", err)
	}
	if len(eng.prunedRepos) != 1 {
		t.Fatalf("Prune with a policy must apply retention (ForgetPolicy), got prunedRepos=%v", eng.prunedRepos)
	}
	if len(eng.manualPruned) != 0 {
		t.Fatalf("Prune with a policy must NOT do a plain prune, got %v", eng.manualPruned)
	}
}

// TestDiscoverVMsRebuildsTargetFromStorage pins VM disaster recovery: after a
// DB loss (no VM target), DiscoverVMs reads the snapshot tags + the mirrored
// encrypted definition and re-creates the target so the deleted VM is restorable.
func TestDiscoverVMsRebuildsTargetFromStorage(t *testing.T) {
	dir := t.TempDir()
	cfg := config.Config{AppKey: strings.Repeat("a", 64), DataDir: dir, HostMountRoot: dir}
	st := newMemStore(t)
	s := mustSettings(t, st)
	s.EncryptionEnabled = false
	s.VMsPath = "backups/vms"
	if err := st.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}
	// Mark the vms repo initialised.
	repo := filepath.Join(dir, "backups", "vms")
	if err := os.MkdirAll(repo, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "config"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	// Mirror an encrypted definition for a VM with no DB target.
	defsDir := filepath.Join(dir, "backups", "bombvault-vm-defs")
	if err := os.MkdirAll(defsDir, 0o750); err != nil {
		t.Fatal(err)
	}
	enc, err := secret.Encrypt(cfg.AppKey, []byte(`{"Method":"live","DomainXML":"<domain/>"}`))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(defsDir, "Tailscale.def"), enc, 0o600); err != nil {
		t.Fatal(err)
	}

	eng := &fakeResticEngine{snaps: []restic.Snapshot{{ID: "aaaa1111", Tags: []string{"vm:Tailscale", "p2"}}}}
	svc := api.NewService(cfg, st, &fakeServiceDocker{}, fakeVirsh{}, eng)

	n, err := svc.DiscoverVMs(context.Background())
	if err != nil {
		t.Fatalf("DiscoverVMs: %v", err)
	}
	if n != 1 {
		t.Fatalf("expected 1 VM discovered, got %d", n)
	}
	tg, err := st.GetVMTargetByName("Tailscale")
	if err != nil {
		t.Fatalf("target not recreated: %v", err)
	}
	if tg.Method != "live" {
		t.Fatalf("method = %q, want live", tg.Method)
	}
}

func TestDeleteSnapshotForgetsByID(t *testing.T) {
	eng := &fakeResticEngine{}
	svc := initRepoSvc(t, eng)
	if err := svc.DeleteSnapshot(context.Background(), "containers", "deadbeef12345678", ""); err != nil {
		t.Fatalf("DeleteSnapshot: %v", err)
	}
	if len(eng.forgotten) != 1 || eng.forgotten[0] != "deadbeef12345678" {
		t.Fatalf("expected forget of the one id, got %v", eng.forgotten)
	}
}

func TestDeleteSnapshotRejectsBadID(t *testing.T) {
	eng := &fakeResticEngine{}
	svc := initRepoSvc(t, eng)
	if err := svc.DeleteSnapshot(context.Background(), "containers", "not-hex!", ""); err == nil {
		t.Fatal("expected an invalid-snapshot-id error")
	}
	if len(eng.forgotten) != 0 {
		t.Fatalf("must not forget on an invalid id: %v", eng.forgotten)
	}
}

// TestSnapshotsSelfHealsStaleLock: a stale-lock error on listing is recovered by
// a stale-unlock + retry, so "Failed to load backups" heals itself.
func TestSnapshotsSelfHealsStaleLock(t *testing.T) {
	eng := &fakeResticEngine{
		snapshotsErr: errors.New("unable to create lock in backend: repository is already locked by PID 877"),
		snaps:        []restic.Snapshot{{ID: "aaaa1111", Tags: []string{"container:plex", "p1"}}},
	}
	svc := initRepoSvc(t, eng)
	got, err := svc.Snapshots(context.Background(), "plex", "")
	if err != nil {
		t.Fatalf("Snapshots should self-heal a stale lock, got %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 snapshot after retry, got %d", len(got))
	}
	if len(eng.unlockedRepos) != 1 || eng.unlockRemoveAll[0] {
		t.Fatalf("expected one STALE unlock (removeAll=false), got repos=%v removeAll=%v", eng.unlockedRepos, eng.unlockRemoveAll)
	}
	if eng.snapshotsCalls != 2 {
		t.Fatalf("expected snapshots to be retried once (2 calls), got %d", eng.snapshotsCalls)
	}
}

func mustSettings(t *testing.T, st *store.Repo) store.Settings {
	t.Helper()
	s, err := st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func contains(ss []string, want string) bool {
	for _, s := range ss {
		if s == want {
			return true
		}
	}
	return false
}
