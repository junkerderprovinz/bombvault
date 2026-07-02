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
	"github.com/junkerderprovinz/bombvault/internal/backup"
	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/dockercli"
	"github.com/junkerderprovinz/bombvault/internal/model"
	"github.com/junkerderprovinz/bombvault/internal/progress"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/restickey"
	"github.com/junkerderprovinz/bombvault/internal/secret"
	"github.com/junkerderprovinz/bombvault/internal/store"
	"github.com/junkerderprovinz/bombvault/internal/virshcli"
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

// TestReplicateOffsiteAppliesOffsiteRetention pins that a replication applies the
// SEPARATE off-site retention policy to the off-site repo after copying — but only
// when that policy is set (so an off-site repo defaults to keep-everything).
func TestReplicateOffsiteAppliesOffsiteRetention(t *testing.T) {
	dir := t.TempDir()
	root := filepath.ToSlash(dir)
	cfg := config.Config{AppKey: strings.Repeat("a", 64), DataDir: dir, HostMountRoot: root}
	st := newMemStore(t)
	s := mustSettings(t, st)
	s.FlashPath = "backups/flash"
	s.FlashOffsite = "backups/flash-offsite"

	// First: NO off-site policy → copy only, no off-site prune.
	if err := st.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}
	eng := &fakeResticEngine{}
	svc := api.NewService(cfg, st, &fakeServiceDocker{}, fakeVirsh{}, eng)
	if err := svc.ReplicateOffsite(context.Background(), "flash"); err != nil {
		t.Fatal(err)
	}
	if len(eng.copied) != 1 || len(eng.prunedRepos) != 0 {
		t.Fatalf("no off-site policy → copy only, got copied=%v prunedRepos=%v", eng.copied, eng.prunedRepos)
	}

	// Now set an off-site policy → replication also prunes the off-site repo.
	s.OffsiteRetentionKeepDaily = 14
	if err := st.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}
	eng2 := &fakeResticEngine{}
	svc2 := api.NewService(cfg, st, &fakeServiceDocker{}, fakeVirsh{}, eng2)
	if err := svc2.ReplicateOffsite(context.Background(), "flash"); err != nil {
		t.Fatal(err)
	}
	if len(eng2.copied) != 1 || len(eng2.prunedRepos) != 1 {
		t.Fatalf("off-site policy set → copy + off-site retention, got copied=%v prunedRepos=%v", eng2.copied, eng2.prunedRepos)
	}
}

// TestDomainStatus drives DomainStatus through a seeded store: a disabled domain
// is "off", an enabled+scheduled domain with no successful backup is "never", and
// one with a fresh successful backup is "ok". The time-boundary cases
// (warn/overdue) are covered exhaustively by the pure rpoStatus helper test.
func TestDomainStatus(t *testing.T) {
	cfg := config.Config{AppKey: strings.Repeat("a", 64), HostMountRoot: t.TempDir()}
	st := newMemStore(t)
	s := mustSettings(t, st)
	// containers: enabled + scheduled, with a fresh successful backup → ok.
	s.ContainersEnabled = true
	s.ContainersSchedule = "daily 02:30"
	// vms: enabled + scheduled, but no successful backup yet → never.
	s.VMsEnabled = true
	s.VMsSchedule = "weekly Mon 03:00"
	// flash: disabled → off (regardless of schedule).
	s.FlashEnabled = false
	s.FlashSchedule = "daily 04:00"
	if err := st.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}

	// Seed a successful container backup so the containers domain reads "ok".
	tg, err := st.UpsertTarget(store.Target{ContainerName: "plex", AppdataPaths: []string{"/x"}})
	if err != nil {
		t.Fatal(err)
	}
	runID, err := st.StartRun(tg.ID, "backup")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.FinishRun(runID, "success", "deadbeef12345678", 2048, ""); err != nil {
		t.Fatal(err)
	}

	svc := api.NewService(cfg, st, &fakeServiceDocker{}, fakeVirsh{}, &fakeResticEngine{})
	entries, err := svc.DomainStatus()
	if err != nil {
		t.Fatalf("DomainStatus: %v", err)
	}
	byDomain := map[string]api.DomainStatusEntry{}
	for _, e := range entries {
		byDomain[e.Domain] = e
	}

	cont := byDomain["containers"]
	if cont.Status != "ok" {
		t.Fatalf("containers status = %q, want ok (entry=%+v)", cont.Status, cont)
	}
	if cont.PeriodSeconds != 86400 {
		t.Fatalf("containers period = %d, want 86400", cont.PeriodSeconds)
	}
	if cont.LastSuccess == 0 {
		t.Fatal("containers lastSuccess should be set after a successful backup")
	}

	vms := byDomain["vms"]
	if vms.Status != "never" {
		t.Fatalf("vms status = %q, want never (entry=%+v)", vms.Status, vms)
	}
	if vms.PeriodSeconds != 604800 {
		t.Fatalf("vms period = %d, want 604800", vms.PeriodSeconds)
	}

	flash := byDomain["flash"]
	if flash.Status != "off" {
		t.Fatalf("flash status = %q, want off (disabled domain)", flash.Status)
	}
	if flash.Enabled {
		t.Fatal("flash should report enabled=false")
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

// TestStartBackupSingleFlight pins the single-backup async guard: StartBackup
// fires the work in the background and returns true; while it is in flight a
// second StartBackup — and a StartBackupAll sharing the same guard — must be
// rejected (returns false), so a single backup and a batch can never overlap.
func TestStartBackupSingleFlight(t *testing.T) {
	t.Setenv("BOMBVAULT_SELF_CONTAINER", "BombVault")
	svc, _, eng, store := backupTestService(t)
	eng.block = make(chan struct{}) // hold the first backup inside restic Backup
	ch, cancel := store.Subscribe()
	defer cancel()

	if !svc.StartBackup(context.Background(), "plex") {
		t.Fatal("first backup should start")
	}
	// The guard is set synchronously by StartBackup, so the second call sees a run
	// in flight regardless of goroutine scheduling.
	if svc.StartBackup(context.Background(), "radarr") {
		t.Fatal("second concurrent backup must be rejected")
	}
	if svc.StartBackupAll(context.Background(), []string{"radarr"}) {
		t.Fatal("a batch must be rejected while a single backup is in flight")
	}
	close(eng.block) // let the backup finish

	// Drain until the per-container terminal event so cleanup is race-free.
	deadline := time.After(5 * time.Second)
	for {
		select {
		case ev := <-ch:
			if ev.Key == "container:plex" && !ev.Active {
				if len(eng.backedUp) != 1 {
					t.Fatalf("backup should run restic once, got %d", len(eng.backedUp))
				}
				return
			}
		case <-deadline:
			t.Fatal("timed out waiting for single backup to finish")
		}
	}
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

// TestServiceSetIncludeAll verifies the one-click action toggles the
// include_in_schedule flag for EVERY installed container, find-or-creating a
// target row for any container that has not been backed up yet.
func TestServiceSetIncludeAll(t *testing.T) {
	dir := t.TempDir()
	cfg := config.Config{AppKey: strings.Repeat("a", 64), DataDir: dir, HostMountRoot: "/host/user"}
	st := newMemStore(t)

	// Two installed containers, neither with a target row yet. Inspect fails so
	// the fallback path is exercised (the point is the loop, not appdata resolution).
	d := &fakeServiceDocker{
		listOut: []dockercli.ContainerInfo{
			{Name: "plex"},
			{Name: "sonarr"},
		},
		inspectErr: errors.New("no such container"),
	}
	svc := api.NewService(cfg, st, d, fakeVirsh{}, &fakeResticEngine{})

	if err := svc.SetIncludeAll(context.Background(), true); err != nil {
		t.Fatalf("SetIncludeAll(true): %v", err)
	}
	for _, name := range []string{"plex", "sonarr"} {
		tg, err := st.GetTargetByContainer(name)
		if err != nil {
			t.Fatalf("target %q must have been created: %v", name, err)
		}
		if !tg.IncludeInSchedule {
			t.Fatalf("include flag must be true for %q", name)
		}
	}

	// Excluding all flips every flag back.
	if err := svc.SetIncludeAll(context.Background(), false); err != nil {
		t.Fatalf("SetIncludeAll(false): %v", err)
	}
	for _, name := range []string{"plex", "sonarr"} {
		tg, err := st.GetTargetByContainer(name)
		if err != nil {
			t.Fatal(err)
		}
		if tg.IncludeInSchedule {
			t.Fatalf("include flag must be false for %q", name)
		}
	}
}

// TestServiceSetVMIncludeAll verifies the VM one-click action toggles the flag
// for every live VM (find-or-creating its target) AND every already-known VM
// target (orphans with backups but no live domain).
func TestServiceSetVMIncludeAll(t *testing.T) {
	dir := t.TempDir()
	cfg := config.Config{AppKey: strings.Repeat("a", 64), DataDir: dir, HostMountRoot: "/host/user"}
	st := newMemStore(t)

	// Pre-seed an orphan VM target (no live domain) so we prove orphans are toggled.
	if _, err := st.UpsertVMTarget(store.VMTarget{Name: "old-vm", Method: "graceful"}); err != nil {
		t.Fatal(err)
	}

	// virsh reports two live VMs; "old-vm" is NOT among them.
	v := listVMsVirsh{vms: []virshcli.VMInfo{
		{Name: "win11", State: "running"},
		{Name: "ubuntu", State: "shut off"},
	}}
	svc := api.NewService(cfg, st, &fakeServiceDocker{}, v, &fakeResticEngine{})

	if err := svc.SetVMIncludeAll(context.Background(), true); err != nil {
		t.Fatalf("SetVMIncludeAll(true): %v", err)
	}
	for _, name := range []string{"win11", "ubuntu", "old-vm"} {
		tg, err := st.GetVMTargetByName(name)
		if err != nil {
			t.Fatalf("vm target %q must exist: %v", name, err)
		}
		if !tg.IncludeInSchedule {
			t.Fatalf("include flag must be true for vm %q", name)
		}
	}

	if err := svc.SetVMIncludeAll(context.Background(), false); err != nil {
		t.Fatalf("SetVMIncludeAll(false): %v", err)
	}
	for _, name := range []string{"win11", "ubuntu", "old-vm"} {
		tg, err := st.GetVMTargetByName(name)
		if err != nil {
			t.Fatal(err)
		}
		if tg.IncludeInSchedule {
			t.Fatalf("include flag must be false for vm %q", name)
		}
	}
}

// listVMsVirsh is a fakeVirsh whose List returns a configured set of VMs, for
// the SetVMIncludeAll test (the base fakeVirsh always returns an empty list).
type listVMsVirsh struct {
	fakeVirsh
	vms []virshcli.VMInfo
}

func (v listVMsVirsh) List(_ context.Context) ([]virshcli.VMInfo, error) { return v.vms, nil }

// countingVirsh records List calls (and can make List fail), so a test can prove
// ListVMs does or does not reach libvirt.
type countingVirsh struct {
	fakeVirsh
	listCalls int
	listErr   error
}

func (c *countingVirsh) List(context.Context) ([]virshcli.VMInfo, error) {
	c.listCalls++
	return nil, c.listErr
}

// TestListVMsSkipsVirshWhenDomainDisabled pins the BJZwart fix: with the VMs
// domain disabled, ListVMs must NOT reach libvirt over SSH (which spammed the
// container log on every dashboard load); with it enabled, it must.
func TestListVMsSkipsVirshWhenDomainDisabled(t *testing.T) {
	st := newMemStore(t)
	cfg := config.Config{AppKey: strings.Repeat("a", 64), DataDir: t.TempDir(), HostMountRoot: t.TempDir()}
	v := &countingVirsh{listErr: errors.New("ssh: could not resolve hostname")}
	svc := api.NewService(cfg, st, &fakeServiceDocker{}, v, &fakeResticEngine{})

	// Disabled (default): no virsh call, no error.
	s := mustSettings(t, st)
	s.VMsEnabled = false
	if err := st.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ListVMs(context.Background()); err != nil {
		t.Fatalf("ListVMs with VMs disabled must not error: %v", err)
	}
	if v.listCalls != 0 {
		t.Fatalf("virsh.List must NOT be called when the VMs domain is disabled, got %d calls", v.listCalls)
	}

	// Enabled: virsh IS consulted (and its error surfaces).
	s.VMsEnabled = true
	if err := st.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ListVMs(context.Background()); err == nil {
		t.Fatal("ListVMs with VMs enabled must surface the virsh error")
	}
	if v.listCalls != 1 {
		t.Fatalf("virsh.List must be called once when enabled, got %d", v.listCalls)
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

// TestRestoreContainerToPath covers the alternate-folder (non-destructive)
// restore: it rejects a bad snapshot id BEFORE touching restic, rejects a target
// that escapes the host mount (the shared paths.Resolve containment guard), and
// on the happy path restores the WHOLE snapshot tree into the resolved target dir
// via the engine's restore-to-target method (RestoreInclude with include "/").
func TestRestoreContainerToPath(t *testing.T) {
	dir := t.TempDir()
	root := filepath.ToSlash(dir)
	cfg := config.Config{AppKey: strings.Repeat("a", 64), DataDir: dir, HostMountRoot: root}
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
	eng := &fakeResticEngine{snaps: []restic.Snapshot{
		{ID: "aaaa1111", Tags: []string{"container:plex"}},
		{ID: "bbbb2222", Tags: []string{"container:sonarr"}}, // foreign — must be refused
	}}
	svc := api.NewService(cfg, st, &fakeServiceDocker{}, fakeVirsh{}, eng)
	ctx := context.Background()

	// Bad snapshot id is rejected before any restic call.
	if _, err := svc.RestoreContainerToPath(ctx, "plex", "local", "not-hex!", "user/restore/plex"); !errors.Is(err, backup.ErrInvalidSnapshotID) {
		t.Fatalf("expected ErrInvalidSnapshotID, got %v", err)
	}
	if len(eng.restored) != 0 {
		t.Fatalf("must not restore on a bad snapshot id, got %v", eng.restored)
	}

	// A target escaping the host mount (../) is refused by the containment guard.
	if _, err := svc.RestoreContainerToPath(ctx, "plex", "local", "aaaa1111", "../escape"); err == nil {
		t.Fatal("expected a containment error for a path escaping the host mount")
	}
	if len(eng.restored) != 0 {
		t.Fatalf("must not restore when the target escapes the mount, got %v", eng.restored)
	}

	// A foreign snapshot (sonarr's) must not be extractable through plex's route.
	if _, err := svc.RestoreContainerToPath(ctx, "plex", "local", "bbbb2222", "user/restore/plex"); err == nil || !strings.Contains(err.Error(), "does not belong") {
		t.Fatalf("foreign snapshot must be refused, got %v", err)
	}

	// Happy path: the whole snapshot is restored into the resolved target dir.
	target, err := svc.RestoreContainerToPath(ctx, "plex", "local", "aaaa1111", "user/restore/plex")
	if err != nil {
		t.Fatalf("RestoreContainerToPath: %v", err)
	}
	wantTarget := root + "/user/restore/plex"
	if target != wantTarget {
		t.Fatalf("resolved target = %q, want %q", target, wantTarget)
	}
	if _, statErr := os.Stat(wantTarget); statErr != nil {
		t.Fatalf("target dir must be created after containment passes: %v", statErr)
	}
	// The fake records repo:snapshot:include->target; include "/" = whole snapshot.
	if len(eng.restored) != 1 || !strings.Contains(eng.restored[0], "aaaa1111:/->"+wantTarget) {
		t.Fatalf("expected a whole-snapshot restore-to-target, got %v", eng.restored)
	}
}

// TestRestoreContainerFiles covers the multi-select file restore: it is
// confirm-gated, rejects a bad snapshot id and an empty selection before touching
// restic, refuses a foreign snapshot, refuses an in-place path that escapes the
// host mount and a target folder that escapes it, and on the happy path extracts
// the selection into a resolved alternate folder (created only after containment
// passes) via the engine.
func TestRestoreContainerFiles(t *testing.T) {
	dir := t.TempDir()
	root := filepath.ToSlash(dir)
	cfg := config.Config{AppKey: strings.Repeat("a", 64), DataDir: dir, HostMountRoot: root}
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
	eng := &fakeResticEngine{snaps: []restic.Snapshot{
		{ID: "aaaa1111", Tags: []string{"container:plex"}},
		{ID: "bbbb2222", Tags: []string{"container:sonarr"}}, // foreign — must be refused
	}}
	svc := api.NewService(cfg, st, &fakeServiceDocker{}, fakeVirsh{}, eng)
	ctx := context.Background()
	fileA := root + "/appdata/plex/a.conf"

	// Not confirmed → refused before any restic call.
	if _, err := svc.RestoreContainerFiles(ctx, "plex", "local", "aaaa1111", []string{fileA}, "user/restore/plex", false); !errors.Is(err, backup.ErrNotConfirmed) {
		t.Fatalf("expected ErrNotConfirmed, got %v", err)
	}
	// Bad snapshot id.
	if _, err := svc.RestoreContainerFiles(ctx, "plex", "local", "not-hex!", []string{fileA}, "user/restore/plex", true); !errors.Is(err, backup.ErrInvalidSnapshotID) {
		t.Fatalf("expected ErrInvalidSnapshotID, got %v", err)
	}
	// Empty selection.
	if _, err := svc.RestoreContainerFiles(ctx, "plex", "local", "aaaa1111", nil, "user/restore/plex", true); err == nil {
		t.Fatal("expected an error for an empty selection")
	}
	// A foreign snapshot (sonarr's) must not be restorable through plex's route.
	if _, err := svc.RestoreContainerFiles(ctx, "plex", "local", "bbbb2222", []string{fileA}, "user/restore/plex", true); err == nil || !strings.Contains(err.Error(), "does not belong") {
		t.Fatalf("foreign snapshot must be refused, got %v", err)
	}
	// An in-place path escaping the host mount is refused (empty target = in place).
	if _, err := svc.RestoreContainerFiles(ctx, "plex", "local", "aaaa1111", []string{"/etc/passwd"}, "", true); err == nil {
		t.Fatal("expected a containment error for an in-place path outside the mount")
	}
	// A target folder escaping the host mount (../) is refused by the guard.
	if _, err := svc.RestoreContainerFiles(ctx, "plex", "local", "aaaa1111", []string{fileA}, "../escape", true); err == nil {
		t.Fatal("expected a containment error for a target escaping the mount")
	}
	if len(eng.restored) != 0 {
		t.Fatalf("no restore must have run on a rejected request, got %v", eng.restored)
	}

	// Happy path — into an alternate folder: the resolved dir is created and BOTH
	// selected paths are extracted into it, in order (multi-file batch).
	fileB := root + "/appdata/plex/sub/b.dat"
	target, err := svc.RestoreContainerFiles(ctx, "plex", "local", "aaaa1111", []string{fileA, fileB}, "user/restore/plex", true)
	if err != nil {
		t.Fatalf("to-folder restore: %v", err)
	}
	wantTarget := root + "/user/restore/plex"
	if target != wantTarget {
		t.Fatalf("resolved target = %q, want %q", target, wantTarget)
	}
	if _, statErr := os.Stat(wantTarget); statErr != nil {
		t.Fatalf("target dir must be created after containment passes: %v", statErr)
	}
	if len(eng.restored) != 2 ||
		!strings.Contains(eng.restored[0], "aaaa1111:"+fileA+"->"+wantTarget) ||
		!strings.Contains(eng.restored[1], "aaaa1111:"+fileB+"->"+wantTarget) {
		t.Fatalf("expected both files restored into the folder, got %v", eng.restored)
	}

	// Mid-batch failure: the second of three paths fails, so the error names the
	// progress (how many went through) and the remaining path is not attempted.
	eng.restored = nil
	eng.restoreErrPath = fileB
	if _, err := svc.RestoreContainerFiles(ctx, "plex", "local", "aaaa1111", []string{fileA, fileB, fileA}, "user/restore/plex", true); err == nil || !strings.Contains(err.Error(), "restored 1 of 3 files") {
		t.Fatalf("expected a progress-annotated mid-batch error, got %v", err)
	}
	if len(eng.restored) != 1 {
		t.Fatalf("must stop at the failing path (only the first should have run), got %v", eng.restored)
	}
}

// restoreTestService builds a service with an initialised containers repo, a
// plex-tagged snapshot ("aaaa1111") in the fake engine, and a real temp host
// mount root — the shared setup for the async-restore (Start*) tests. It
// returns the service, store, engine and the resolved mount root.
func restoreTestService(t *testing.T, eng *fakeResticEngine) (*api.Service, *store.Repo, string) {
	t.Helper()
	dir := t.TempDir()
	root := filepath.ToSlash(dir)
	cfg := config.Config{AppKey: strings.Repeat("a", 64), DataDir: dir, HostMountRoot: root}
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
	eng.snaps = []restic.Snapshot{{ID: "aaaa1111", Tags: []string{"container:plex"}}}
	svc := api.NewService(cfg, st, &fakeServiceDocker{}, fakeVirsh{}, eng)
	return svc, st, root
}

// TestStartRestoreSingleFlight pins the shared single-flight guard for the
// async restore starters: while one restore is in flight, every other restore
// starter — and a backup, which shares the same guard — must be rejected busy
// (started=false, no error), so a restore can never overlap a backup or another
// restore (they contend on repo locks and container stop/start).
func TestStartRestoreSingleFlight(t *testing.T) {
	eng := &fakeResticEngine{blockRestore: make(chan struct{})}
	svc, _, root := restoreTestService(t, eng)
	ctx := context.Background()
	fileA := root + "/appdata/plex/a.conf"

	// First restore starts (and blocks inside the engine); the resolved target is
	// returned in the ack.
	target, started, err := svc.StartRestoreToPath(ctx, "plex", "local", "aaaa1111", "user/restore/plex")
	if err != nil || !started {
		t.Fatalf("first restore should start: started=%v err=%v", started, err)
	}
	if want := root + "/user/restore/plex"; target != want {
		t.Fatalf("ack target = %q, want %q", target, want)
	}

	// The guard is set synchronously by the starters, so every second call sees a
	// run in flight regardless of goroutine scheduling.
	if _, started, err := svc.StartRestoreToPath(ctx, "plex", "local", "aaaa1111", "user/restore/plex2"); err != nil || started {
		t.Fatalf("second to-folder restore must be rejected busy: started=%v err=%v", started, err)
	}
	if _, started, err := svc.StartRestoreFiles(ctx, "plex", "local", "aaaa1111", []string{fileA}, "user/restore/plex", true); err != nil || started {
		t.Fatalf("a files restore must be rejected busy: started=%v err=%v", started, err)
	}
	if started, err := svc.StartRestore(ctx, "plex", "aaaa1111", "local", false); err != nil || started {
		t.Fatalf("an in-place restore must be rejected busy: started=%v err=%v", started, err)
	}
	if started, err := svc.StartRestoreVM(ctx, "win11", "aaaa1111", "local", false); err != nil || started {
		t.Fatalf("a VM restore must be rejected busy: started=%v err=%v", started, err)
	}
	if svc.StartBackup(ctx, "plex") {
		t.Fatal("a backup must be rejected while a restore is in flight (shared guard)")
	}

	close(eng.blockRestore) // let the restore finish
	waitForBackupDone(t, svc)
	if len(eng.restored) != 1 {
		t.Fatalf("exactly the first restore should have run, got %v", eng.restored)
	}
}

// TestStartRestoreValidationFailsFast pins the sync/async split: validation
// runs SYNCHRONOUSLY in every Start* restore wrapper, so a bad request fails
// immediately with a clear error, no goroutine is started, and the shared
// guard is released right away.
func TestStartRestoreValidationFailsFast(t *testing.T) {
	eng := &fakeResticEngine{}
	svc, _, root := restoreTestService(t, eng)
	ctx := context.Background()
	fileA := root + "/appdata/plex/a.conf"

	// Bad snapshot id → synchronous ErrInvalidSnapshotID from every wrapper.
	if _, started, err := svc.StartRestoreToPath(ctx, "plex", "local", "not-hex!", "user/restore/plex"); !errors.Is(err, backup.ErrInvalidSnapshotID) || started {
		t.Fatalf("to-folder: want ErrInvalidSnapshotID + not started, got started=%v err=%v", started, err)
	}
	if _, started, err := svc.StartRestoreFiles(ctx, "plex", "local", "not-hex!", []string{fileA}, "user/restore/plex", true); !errors.Is(err, backup.ErrInvalidSnapshotID) || started {
		t.Fatalf("files: want ErrInvalidSnapshotID + not started, got started=%v err=%v", started, err)
	}
	if started, err := svc.StartRestore(ctx, "plex", "not-hex!", "local", false); !errors.Is(err, backup.ErrInvalidSnapshotID) || started {
		t.Fatalf("in-place: want ErrInvalidSnapshotID + not started, got started=%v err=%v", started, err)
	}
	if started, err := svc.StartRestoreVM(ctx, "win11", "not-hex!", "local", false); !errors.Is(err, backup.ErrInvalidSnapshotID) || started {
		t.Fatalf("vm: want ErrInvalidSnapshotID + not started, got started=%v err=%v", started, err)
	}

	// A foreign snapshot is refused synchronously too (ownership check).
	if _, started, err := svc.StartRestoreToPath(ctx, "plex", "local", "bbbb2222", "user/restore/plex"); err == nil || !strings.Contains(err.Error(), "does not belong") || started {
		t.Fatalf("foreign snapshot must be refused synchronously, got started=%v err=%v", started, err)
	}

	// The guard must have been released by every failed validation (no goroutine
	// holds it), so a valid start is not wrongly answered "busy"...
	if svc.BackupInProgress() {
		t.Fatal("failed validation must release the single-flight guard")
	}
	// ...and no restore ever reached the engine.
	if len(eng.restored) != 0 {
		t.Fatalf("no restore must have run for rejected requests, got %v", eng.restored)
	}
}

// TestStartRestoreFilesRecordsRun pins the run-history bookkeeping of the async
// file-level restore: the detached run records a kind "restore" run against the
// container's target row — failed WITH the real restic error text, success WITH
// the snapshot id — so the outcome survives the browser going away.
func TestStartRestoreFilesRecordsRun(t *testing.T) {
	eng := &fakeResticEngine{}
	svc, st, root := restoreTestService(t, eng)
	ctx := context.Background()
	fileA := root + "/appdata/plex/a.conf"

	// The run is recorded against the container's target row.
	tg, err := st.UpsertTarget(store.Target{ContainerName: "plex", AppdataPaths: []string{fileA}})
	if err != nil {
		t.Fatal(err)
	}

	// Failure: the engine rejects the path — the run must be failed and carry the
	// real restic error text.
	eng.restoreErrPath = fileA
	if _, started, err := svc.StartRestoreFiles(ctx, "plex", "local", "aaaa1111", []string{fileA}, "user/restore/plex", true); err != nil || !started {
		t.Fatalf("restore should start: started=%v err=%v", started, err)
	}
	waitForBackupDone(t, svc)
	runs, err := st.ListRuns(10)
	if err != nil {
		t.Fatal(err)
	}
	if len(runs) != 1 {
		t.Fatalf("want 1 recorded run, got %d", len(runs))
	}
	failed := runs[0]
	if failed.TargetID != tg.ID || failed.Kind != "restore" || failed.Status != "failed" {
		t.Fatalf("want a failed kind=restore run for the target, got %+v", failed)
	}
	if !strings.Contains(failed.Error, "restore boom") {
		t.Fatalf("the run must carry the real restic error text, got %q", failed.Error)
	}

	// Success: the run must be success and carry the snapshot id.
	eng.restoreErrPath = ""
	if _, started, err := svc.StartRestoreFiles(ctx, "plex", "local", "aaaa1111", []string{fileA}, "user/restore/plex", true); err != nil || !started {
		t.Fatalf("second restore should start: started=%v err=%v", started, err)
	}
	waitForBackupDone(t, svc)
	runs, err = st.ListRuns(10)
	if err != nil {
		t.Fatal(err)
	}
	if len(runs) != 2 {
		t.Fatalf("want 2 recorded runs, got %d", len(runs))
	}
	// Both runs started within the same second, so the list order can tie —
	// find the success run by content instead of position.
	found := false
	for _, run := range runs {
		if run.Status != "success" {
			continue
		}
		found = true
		if run.Kind != "restore" || run.SnapshotID != "aaaa1111" || run.Error != "" {
			t.Fatalf("want a success kind=restore run with the snapshot id, got %+v", run)
		}
	}
	if !found {
		t.Fatalf("want a success run recorded, got %+v", runs)
	}
}

// TestRestoreHoldsDomainLockAgainstScheduledBackup pins the domain-lock layer
// of the restore execute paths: the scheduler calls s.Backup DIRECTLY and
// bypasses the batchActive single-flight guard by design, so the domain repo
// lock is the only layer a scheduled backup respects. A direct Backup of the
// same domain must therefore BLOCK until a running detached restore releases
// the lock — serialization, never overlap (in either direction).
func TestRestoreHoldsDomainLockAgainstScheduledBackup(t *testing.T) {
	eng := &fakeResticEngine{
		blockRestore:   make(chan struct{}),
		restoreEntered: make(chan struct{}, 1),
	}
	svc, _, _ := restoreTestService(t, eng)
	ctx := context.Background()

	// Start a detached restore and wait until it is INSIDE the engine — at that
	// point the restore execute path already holds the "containers" domain lock.
	if _, started, err := svc.StartRestoreToPath(ctx, "plex", "local", "aaaa1111", "user/restore/plex"); err != nil || !started {
		t.Fatalf("restore should start: started=%v err=%v", started, err)
	}
	select {
	case <-eng.restoreEntered:
	case <-time.After(5 * time.Second):
		t.Fatal("restore never reached the engine")
	}

	// A scheduled-style DIRECT backup (no batchActive involved) must block on
	// the domain lock instead of overlapping the in-flight restore.
	backupDone := make(chan error, 1)
	go func() {
		_, err := svc.Backup(ctx, "plex")
		backupDone <- err
	}()
	select {
	case err := <-backupDone:
		t.Fatalf("backup completed while the restore held the domain lock (err=%v)", err)
	case <-time.After(200 * time.Millisecond):
		// Still blocked — serialized behind the restore, exactly as intended.
	}

	close(eng.blockRestore) // restore finishes → releases the domain lock
	select {
	case err := <-backupDone:
		if err != nil {
			t.Fatalf("backup after the restore released the lock: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("backup never ran after the restore released the domain lock")
	}
	waitForBackupDone(t, svc)
	if len(eng.restored) != 1 {
		t.Fatalf("exactly the restore should have hit the engine's restore path, got %v", eng.restored)
	}
}

// diffTagTestService builds a service with an initialised containers repo and the
// given snapshots, so DiffSnapshots/TagSnapshot reach the fake engine.
func diffTagTestService(t *testing.T, eng *fakeResticEngine) *api.Service {
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

// TestDiffSnapshots pins the snapshot-diff access control + happy path: a bad
// snapshot id is rejected before any restic call, a foreign snapshot (another
// container's) is refused, and a valid pair diffs through the engine and returns
// the summary the engine reports.
func TestDiffSnapshots(t *testing.T) {
	eng := &fakeResticEngine{
		snaps: []restic.Snapshot{
			{ID: "aaaa1111", Tags: []string{"container:plex"}},
			{ID: "cccc3333", Tags: []string{"container:plex"}},
			{ID: "bbbb2222", Tags: []string{"container:sonarr"}}, // foreign — must be refused
		},
		diffResult: restic.DiffResult{AddedFiles: 3, RemovedFiles: 1, ChangedFiles: 2, AddedBytes: 4096, RemovedBytes: 512},
	}
	svc := diffTagTestService(t, eng)
	ctx := context.Background()

	// Bad snapshot id is rejected before any restic call.
	if _, err := svc.DiffSnapshots(ctx, "plex", "local", "not-hex!", "cccc3333"); !errors.Is(err, backup.ErrInvalidSnapshotID) {
		t.Fatalf("expected ErrInvalidSnapshotID, got %v", err)
	}
	if len(eng.diffPairs) != 0 {
		t.Fatalf("must not diff on a bad snapshot id, got %v", eng.diffPairs)
	}

	// A foreign snapshot (sonarr's) must not be diffable through plex's route.
	if _, err := svc.DiffSnapshots(ctx, "plex", "local", "aaaa1111", "bbbb2222"); err == nil || !strings.Contains(err.Error(), "does not belong") {
		t.Fatalf("foreign snapshot must be refused, got %v", err)
	}
	if len(eng.diffPairs) != 0 {
		t.Fatalf("must not diff a foreign snapshot, got %v", eng.diffPairs)
	}

	// Happy path: both snapshots belong to plex → diff runs and the summary is returned.
	got, err := svc.DiffSnapshots(ctx, "plex", "local", "aaaa1111", "cccc3333")
	if err != nil {
		t.Fatalf("DiffSnapshots: %v", err)
	}
	if got != eng.diffResult {
		t.Fatalf("diff result = %+v, want %+v", got, eng.diffResult)
	}
	if len(eng.diffPairs) != 1 || eng.diffPairs[0] != "aaaa1111->cccc3333" {
		t.Fatalf("expected a single diff aaaa1111->cccc3333, got %v", eng.diffPairs)
	}
}

// TestTagSnapshot pins the tag-add access control + sanitisation: a bad snapshot
// id is rejected, a tag with a comma is refused (restic tags are
// comma-separated), tags are trimmed and empties dropped, and a valid call tags
// the snapshot through the engine.
func TestTagSnapshot(t *testing.T) {
	eng := &fakeResticEngine{snaps: []restic.Snapshot{
		{ID: "aaaa1111", Tags: []string{"container:plex"}},
	}}
	svc := diffTagTestService(t, eng)
	ctx := context.Background()

	// Bad snapshot id is rejected before any restic call.
	if err := svc.TagSnapshot(ctx, "plex", "local", "not-hex!", []string{"keep"}); !errors.Is(err, backup.ErrInvalidSnapshotID) {
		t.Fatalf("expected ErrInvalidSnapshotID, got %v", err)
	}
	if len(eng.taggedSnaps) != 0 {
		t.Fatalf("must not tag on a bad snapshot id, got %v", eng.taggedSnaps)
	}

	// A tag containing a comma is refused (would split into two restic tags).
	if err := svc.TagSnapshot(ctx, "plex", "local", "aaaa1111", []string{"a,b"}); err == nil || !strings.Contains(err.Error(), "comma") {
		t.Fatalf("comma tag must be refused, got %v", err)
	}
	if len(eng.taggedSnaps) != 0 {
		t.Fatalf("must not tag with an invalid tag, got %v", eng.taggedSnaps)
	}

	// Happy path: tags are trimmed, empties dropped, the snapshot is tagged.
	if err := svc.TagSnapshot(ctx, "plex", "local", "aaaa1111", []string{"  keep  ", "", "milestone"}); err != nil {
		t.Fatalf("TagSnapshot: %v", err)
	}
	if len(eng.taggedSnaps) != 1 || eng.taggedSnaps[0] != "aaaa1111:keep,milestone" {
		t.Fatalf("expected aaaa1111 tagged keep,milestone, got %v", eng.taggedSnaps)
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

// TestDeleteBackupsVMForgetsOnlyThatVMAndPrunes pins the one-shot VM bulk delete:
// it forgets ONLY the target VM's tagged snapshots (not other VMs'), prunes the
// freed space (Forget prune=true), and drops the store target on the LOCAL source.
func TestDeleteBackupsVMForgetsOnlyThatVMAndPrunes(t *testing.T) {
	dir := t.TempDir()
	cfg := config.Config{AppKey: strings.Repeat("a", 64), DataDir: dir, HostMountRoot: dir}
	st := newMemStore(t)
	s := mustSettings(t, st)
	s.VMsPath = "backups/vms"
	if err := st.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}
	repo := filepath.Join(dir, "backups", "vms")
	if err := os.MkdirAll(repo, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "config"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := st.UpsertVMTarget(store.VMTarget{Name: "win11"}); err != nil {
		t.Fatal(err)
	}

	eng := &fakeResticEngine{snaps: []restic.Snapshot{
		{ID: "aaaa1111", Tags: []string{"vm:win11", "p2"}},
		{ID: "bbbb2222", Tags: []string{"vm:ubuntu", "p2"}}, // other VM — must be left alone
		{ID: "cccc3333", Tags: []string{"vm:win11", "p2"}},
	}}
	svc := api.NewService(cfg, st, &fakeServiceDocker{}, fakeVirsh{}, eng)

	if err := svc.DeleteBackupsVM(context.Background(), "win11", ""); err != nil {
		t.Fatalf("DeleteBackupsVM: %v", err)
	}
	if len(eng.forgotten) != 2 || !contains(eng.forgotten, "aaaa1111") || !contains(eng.forgotten, "cccc3333") {
		t.Fatalf("expected win11's aaaa1111+cccc3333 forgotten, got %v", eng.forgotten)
	}
	if contains(eng.forgotten, "bbbb2222") {
		t.Fatalf("forgot another VM's snapshot: %v", eng.forgotten)
	}
	if !eng.forgetPruned {
		t.Fatal("bulk delete must forget with prune=true (reclaim space)")
	}
	if _, err := st.GetVMTargetByName("win11"); err == nil {
		t.Fatal("local bulk delete must drop the VM target")
	}
}

// TestForgetVMTargetRemovesEntry pins the orphan-cleanup fix: a no-longer-installed
// VM with no backups can be cleared from the list (its target row) without needing
// a repo — answering "how do I delete this entry".
func TestForgetVMTargetRemovesEntry(t *testing.T) {
	cfg := config.Config{AppKey: strings.Repeat("a", 64), DataDir: t.TempDir()}
	st := newMemStore(t)
	if _, err := st.UpsertVMTarget(store.VMTarget{Name: "DietPi_template"}); err != nil {
		t.Fatal(err)
	}
	svc := api.NewService(cfg, st, &fakeServiceDocker{}, fakeVirsh{}, &fakeResticEngine{})
	if err := svc.ForgetVMTarget("DietPi_template"); err != nil {
		t.Fatalf("ForgetVMTarget: %v", err)
	}
	if _, err := st.GetVMTargetByName("DietPi_template"); err == nil {
		t.Fatal("VM target should be gone after ForgetVMTarget")
	}
}

// TestDeleteBackupsVMOffsiteKeepsTarget: purging only the OFF-SITE replica must
// not delete the store target — the VM stays restorable from the local copy.
func TestDeleteBackupsVMOffsiteKeepsTarget(t *testing.T) {
	dir := t.TempDir()
	cfg := config.Config{AppKey: strings.Repeat("a", 64), DataDir: dir, HostMountRoot: dir}
	st := newMemStore(t)
	s := mustSettings(t, st)
	s.VMsPath = "backups/vms"
	s.VMsOffsite = "rest:http://offsite/vms" // a remote repo (assumed to exist)
	if err := st.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}
	if _, err := st.UpsertVMTarget(store.VMTarget{Name: "win11"}); err != nil {
		t.Fatal(err)
	}
	eng := &fakeResticEngine{snaps: []restic.Snapshot{
		{ID: "aaaa1111", Tags: []string{"vm:win11", "p2"}},
	}}
	svc := api.NewService(cfg, st, &fakeServiceDocker{}, fakeVirsh{}, eng)

	if err := svc.DeleteBackupsVM(context.Background(), "win11", "offsite"); err != nil {
		t.Fatalf("DeleteBackupsVM offsite: %v", err)
	}
	if !contains(eng.forgotten, "aaaa1111") {
		t.Fatalf("off-site snapshot must be forgotten, got %v", eng.forgotten)
	}
	if _, err := st.GetVMTargetByName("win11"); err != nil {
		t.Fatalf("off-site purge must KEEP the VM target (still restorable from local): %v", err)
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
	// A remote-style repo location: Restore now verifies the explicit snapshot id
	// belongs to the container BEFORE anything runs, and that listing reaches the
	// fake engine directly for a remote repo (a local one would need an on-disk
	// marker, which can't live under the fixed Linux root above).
	s.ContainersPath = "rest:http://127.0.0.1/containers"
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
	// The snapshot must exist for the restore preflight (VerifySnapshot) to pass,
	// and carry the ownership tag every backup writes, since Restore now verifies
	// an explicit snapshot id belongs to the container BEFORE anything runs.
	eng := &fakeResticEngine{snaps: []restic.Snapshot{{ID: "deadbeef", Tags: []string{"container:Pingvin-Share-X"}}}}
	svc := api.NewService(cfg, st, d, fakeVirsh{}, eng)

	// Use a valid 8-hex snapshot id to pass the orchestrator's regex guard.
	restoreErr := svc.Restore(context.Background(), "Pingvin-Share-X", "deadbeef", true, "", false)
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
	restoreErrPath  string // when set, RestoreInclude fails on this include path
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
	checkDataRepos  []string // repo of each CheckData (drill) call
	checkDataPct    []int    // subset percent of each CheckData call
	checkDataErr    error    // returned by CheckData (drill outcome)
	unlockErr       error
	statsCalls      []string // --mode value of each Stats call
	statsErr        error
	diffResult      restic.DiffResult // returned by Diff
	diffPairs       []string          // "snap1->snap2" of each Diff call
	taggedSnaps     []string          // "snapID:tag,tag" of each TagAdd call
	forgetPruned    bool              // prune flag of the last Forget call
	// block, when non-nil, makes Backup wait on it — lets a test hold a batch
	// run "in flight" to exercise the single-batch (409) guard deterministically.
	block chan struct{}
	// blockRestore, when non-nil, makes RestoreInclude AND RestorePath wait on
	// it — lets a test hold an async restore "in flight" to exercise the shared
	// single-flight guard and the domain lock deterministically.
	blockRestore chan struct{}
	// restoreEntered, when non-nil, receives one (non-blocking) signal the
	// moment a blocked restore call is INSIDE the engine — i.e. the restore
	// execute path has already acquired the domain repo lock — so a test can
	// order its next step deterministically instead of sleeping.
	restoreEntered chan struct{}
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

// blockIfArmed signals restoreEntered (non-blocking) and waits on blockRestore
// when it is armed — shared by the restore entry points of the fake.
func (f *fakeResticEngine) blockIfArmed() {
	if f.blockRestore == nil {
		return
	}
	if f.restoreEntered != nil {
		select {
		case f.restoreEntered <- struct{}{}:
		default:
		}
	}
	<-f.blockRestore
}

func (f *fakeResticEngine) RestorePath(_ context.Context, repo, snapshotID, path string, _ restic.Mode) error {
	f.blockIfArmed()
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

func (f *fakeResticEngine) Forget(_ context.Context, _ string, snapshotIDs []string, prune bool, _ restic.Mode) error {
	f.forgotten = append(f.forgotten, snapshotIDs...)
	f.forgetPruned = prune
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
	f.blockIfArmed()
	if f.restoreErrPath != "" && includePath == f.restoreErrPath {
		return errors.New("restore boom")
	}
	f.restored = append(f.restored, repo+":"+snapshotID+":"+includePath+"->"+target)
	return nil
}

func (f *fakeResticEngine) Check(_ context.Context, repo string, _ restic.Mode) error {
	f.checked = append(f.checked, repo)
	return f.checkErr
}

func (f *fakeResticEngine) CheckData(_ context.Context, repo string, subsetPercent int, _ restic.Mode) error {
	f.checkDataRepos = append(f.checkDataRepos, repo)
	f.checkDataPct = append(f.checkDataPct, subsetPercent)
	return f.checkDataErr
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

func (f *fakeResticEngine) Copy(_ context.Context, destRepo, srcRepo string, _ []string, _ restic.Limits, _ restic.Mode) error {
	f.copied = append(f.copied, srcRepo+"->"+destRepo)
	return f.copyErr
}

func (f *fakeResticEngine) Stats(_ context.Context, _, mode string, _ restic.Mode) (restic.StatsResult, error) {
	f.statsCalls = append(f.statsCalls, mode)
	if f.statsErr != nil {
		return restic.StatsResult{}, f.statsErr
	}
	if mode == "raw-data" {
		return restic.StatsResult{TotalSize: 1000, BlobCount: 10}, nil
	}
	return restic.StatsResult{TotalSize: 5000, FileCount: 50}, nil
}

func (f *fakeResticEngine) Diff(_ context.Context, _, snap1, snap2 string, _ restic.Mode) (restic.DiffResult, error) {
	f.diffPairs = append(f.diffPairs, snap1+"->"+snap2)
	return f.diffResult, nil
}

func (f *fakeResticEngine) TagAdd(_ context.Context, _, snapID string, tags []string, _ restic.Mode) error {
	f.taggedSnaps = append(f.taggedSnaps, snapID+":"+strings.Join(tags, ","))
	return nil
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

// TestRunRestoreDrillRecordsResult pins the happy path: CheckData passes, so the
// drill is recorded ok=true (with the configured subset percent) and returned.
func TestRunRestoreDrillRecordsResult(t *testing.T) {
	eng := &fakeResticEngine{snaps: []restic.Snapshot{{ID: "aaaa1111bbbb2222"}}}
	svc := initRepoSvc(t, eng)

	drill, err := svc.RunRestoreDrill(context.Background(), "containers", "local")
	if err != nil {
		t.Fatalf("RunRestoreDrill: %v", err)
	}
	if !drill.OK || drill.Domain != "containers" || drill.Source != "local" {
		t.Fatalf("expected an ok drill for containers/local, got %+v", drill)
	}
	if len(eng.checkDataRepos) != 1 {
		t.Fatalf("expected exactly one CheckData call, got %v", eng.checkDataRepos)
	}
	if len(eng.checkDataPct) != 1 || eng.checkDataPct[0] != 5 {
		t.Fatalf("expected the default 5%% subset, got %v", eng.checkDataPct)
	}
	// The result is persisted and readable via LatestDrill.
	latest, found, err := svc.LatestDrill("containers", "local")
	if err != nil || !found {
		t.Fatalf("latest drill not recorded: found=%v err=%v", found, err)
	}
	if !latest.OK || latest.At == 0 {
		t.Fatalf("recorded drill = %+v, want ok=true with a timestamp", latest)
	}
}

// TestRunRestoreDrillNoBackups pins that a repo with no snapshots yields a clear
// "no backups" error and records NOTHING (no misleading failure), and that
// CheckData is never run.
func TestRunRestoreDrillNoBackups(t *testing.T) {
	eng := &fakeResticEngine{} // snaps nil → empty repo
	svc := initRepoSvc(t, eng)

	_, err := svc.RunRestoreDrill(context.Background(), "containers", "local")
	if err == nil {
		t.Fatal("expected an error when there are no snapshots to verify")
	}
	if len(eng.checkDataRepos) != 0 {
		t.Fatalf("CheckData must not run with no snapshots, got %v", eng.checkDataRepos)
	}
	if _, found, fErr := svc.LatestDrill("containers", "local"); fErr != nil {
		t.Fatalf("LatestDrill: %v", fErr)
	} else if found {
		t.Fatal("no drill must be recorded when there are no backups to verify")
	}
}

// TestRunRestoreDrillFailureRecorded pins that a CheckData failure is recorded as
// a drill with ok=false (so the badge shows "not restorable") AND surfaced as an
// error to the caller.
func TestRunRestoreDrillFailureRecorded(t *testing.T) {
	eng := &fakeResticEngine{
		snaps:        []restic.Snapshot{{ID: "aaaa1111bbbb2222"}},
		checkDataErr: errors.New("data corruption in pack /repo/data/ab/cd"),
	}
	svc := initRepoSvc(t, eng)

	drill, err := svc.RunRestoreDrill(context.Background(), "containers", "local")
	if err == nil {
		t.Fatal("expected the drill to surface the CheckData failure")
	}
	if drill.OK {
		t.Fatalf("a failed drill must be ok=false, got %+v", drill)
	}
	latest, found, lErr := svc.LatestDrill("containers", "local")
	if lErr != nil || !found {
		t.Fatalf("a failed drill must still be recorded: found=%v err=%v", found, lErr)
	}
	if latest.OK {
		t.Fatalf("recorded drill must be ok=false, got %+v", latest)
	}
	if latest.Detail == "" {
		t.Fatal("a failed drill should record a (scrubbed) detail")
	}
	// Defense-in-depth: the recorded detail must not leak the absolute repo path.
	if strings.Contains(latest.Detail, "/repo/data") {
		t.Fatalf("drill detail must be path-scrubbed, got %q", latest.Detail)
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

// TestPruneDomainClearsStaleLockFirst pins that a manual prune clears a stale
// restic lock BEFORE pruning. Without this, a lock left by a previously
// interrupted backup/prune makes every manual Prune fail with "repository is
// already locked" — the reported "prune is broken". The unlock must be a
// stale-only unlock (removeAll=false), exactly as backups and DeleteSnapshot do.
func TestPruneDomainClearsStaleLockFirst(t *testing.T) {
	eng := &fakeResticEngine{}
	svc := initRepoSvc(t, eng)
	if err := svc.PruneDomain(context.Background(), "containers", ""); err != nil {
		t.Fatalf("PruneDomain: %v", err)
	}
	if len(eng.unlockedRepos) != 1 {
		t.Fatalf("prune must clear a stale lock exactly once before pruning, got unlocks=%v", eng.unlockedRepos)
	}
	if eng.unlockRemoveAll[0] {
		t.Fatalf("stale-unlock must be removeAll=false (only stale locks), got removeAll=%v", eng.unlockRemoveAll)
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

// TestPruneDomainPerSourceRetention pins the per-source retention fix: pruning the
// OFF-SITE repo uses the off-site policy, and pruning the LOCAL repo uses the local
// policy. Here local retention is OFF and off-site is SET, so off-site prune
// applies retention (ForgetPolicy) while local prune is a plain space-reclaim.
func TestPruneDomainPerSourceRetention(t *testing.T) {
	dir := t.TempDir()
	cfg := config.Config{AppKey: strings.Repeat("a", 64), DataDir: dir, HostMountRoot: dir}
	st := newMemStore(t)
	s := mustSettings(t, st)
	s.ContainersPath = "backups/containers"
	s.ContainersOffsite = "backups/containers-offsite"
	// Local policy OFF, off-site policy SET (archive: keep 30 daily).
	s.RetentionKeepLast, s.RetentionKeepDaily, s.RetentionKeepWeekly, s.RetentionKeepMonthly = 0, 0, 0, 0
	s.OffsiteRetentionKeepDaily = 30
	if err := st.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}
	for _, r := range []string{"backups/containers", "backups/containers-offsite"} {
		if err := os.MkdirAll(filepath.Join(dir, r), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, r, "config"), []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	eng := &fakeResticEngine{}
	svc := api.NewService(cfg, st, &fakeServiceDocker{}, fakeVirsh{}, eng)

	// Off-site prune → off-site policy is set → applies retention (ForgetPolicy).
	if err := svc.PruneDomain(context.Background(), "containers", "offsite"); err != nil {
		t.Fatalf("PruneDomain offsite: %v", err)
	}
	if len(eng.prunedRepos) != 1 || len(eng.manualPruned) != 0 {
		t.Fatalf("off-site prune must apply the off-site policy, got prunedRepos=%v manualPruned=%v", eng.prunedRepos, eng.manualPruned)
	}

	// Local prune → local policy is OFF → plain space-reclaim, NOT the off-site policy.
	if err := svc.PruneDomain(context.Background(), "containers", ""); err != nil {
		t.Fatalf("PruneDomain local: %v", err)
	}
	if len(eng.prunedRepos) != 1 {
		t.Fatalf("local prune must NOT apply a policy when local retention is off, got prunedRepos=%v", eng.prunedRepos)
	}
	if len(eng.manualPruned) != 1 {
		t.Fatalf("local prune with no policy must plain-prune, got manualPruned=%v", eng.manualPruned)
	}
}

// notInstalledVirsh is a fakeVirsh whose DumpXML reports the libvirt "failed to
// get domain" error — i.e. the host no longer defines the VM.
type notInstalledVirsh struct{ fakeVirsh }

func (notInstalledVirsh) DumpXML(_ context.Context, _ string) (string, error) {
	return "", errors.New("virshcli: dumpxml: error: failed to get domain 'DietPi_template'")
}

// TestBackupVMSkipsWhenDomainNotInstalled pins that a scheduled VM whose domain
// was deleted/undefined on the host is SKIPPED (backup.ErrVMNotInstalled), not
// failed — so the nightly vms job stops erroring on a leftover schedule entry.
func TestBackupVMSkipsWhenDomainNotInstalled(t *testing.T) {
	dir := t.TempDir()
	cfg := config.Config{AppKey: strings.Repeat("a", 64), DataDir: dir, HostMountRoot: dir}
	st := newMemStore(t)
	s := mustSettings(t, st)
	s.VMsPath = "backups/vms"
	if err := st.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}
	eng := &fakeResticEngine{}
	svc := api.NewService(cfg, st, &fakeServiceDocker{}, notInstalledVirsh{}, eng)

	_, err := svc.BackupVM(context.Background(), "DietPi_template")
	if !errors.Is(err, backup.ErrVMNotInstalled) {
		t.Fatalf("expected backup.ErrVMNotInstalled for a removed domain, got %v", err)
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

// TestCollectStatsNoRepoIsNoop pins that CollectStats records nothing and returns
// nil when the local repo has not been created yet — so the post-backup hook can
// never turn a good backup into a failure on a fresh setup.
func TestCollectStatsNoRepoIsNoop(t *testing.T) {
	dir := t.TempDir()
	cfg := config.Config{AppKey: strings.Repeat("a", 64), DataDir: dir, HostMountRoot: dir}
	st := newMemStore(t)
	s := mustSettings(t, st)
	s.ContainersPath = "backups/containers" // never initialised (no config marker)
	if err := st.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}
	eng := &fakeResticEngine{}
	svc := api.NewService(cfg, st, &fakeServiceDocker{}, fakeVirsh{}, eng)

	if err := svc.CollectStats(context.Background(), "containers", "local"); err != nil {
		t.Fatalf("CollectStats on a missing repo must be a no-op, got %v", err)
	}
	if len(eng.statsCalls) != 0 {
		t.Fatalf("Stats must not be called for a missing repo, got %v", eng.statsCalls)
	}
	if got, err := svc.RepoStats("containers", "local", 0); err != nil || len(got) != 0 {
		t.Fatalf("no sample should be recorded, got %v err=%v", got, err)
	}
}

// TestCollectStatsEmptyRepoIsNoop pins that an initialised but empty
// (zero-snapshot) repo records nothing — Stats is never run over an empty repo.
func TestCollectStatsEmptyRepoIsNoop(t *testing.T) {
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
	eng := &fakeResticEngine{} // no snapshots
	svc := api.NewService(cfg, st, &fakeServiceDocker{}, fakeVirsh{}, eng)

	if err := svc.CollectStats(context.Background(), "containers", "local"); err != nil {
		t.Fatalf("CollectStats on an empty repo must be a no-op, got %v", err)
	}
	if len(eng.statsCalls) != 0 {
		t.Fatalf("Stats must not run on a zero-snapshot repo, got %v", eng.statsCalls)
	}
}

// TestCollectStatsRecordsSample pins the happy path: a repo with snapshots is
// sampled with both restic modes and one row is recorded.
func TestCollectStatsRecordsSample(t *testing.T) {
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
	eng := &fakeResticEngine{snaps: []restic.Snapshot{{ID: "aaaa1111"}, {ID: "bbbb2222"}}}
	svc := api.NewService(cfg, st, &fakeServiceDocker{}, fakeVirsh{}, eng)

	if err := svc.CollectStats(context.Background(), "containers", "local"); err != nil {
		t.Fatalf("CollectStats: %v", err)
	}
	if !contains(eng.statsCalls, "raw-data") || !contains(eng.statsCalls, "restore-size") {
		t.Fatalf("both restic stats modes must be sampled, got %v", eng.statsCalls)
	}
	got, err := svc.RepoStats("containers", "local", 0)
	if err != nil {
		t.Fatalf("RepoStats: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected one recorded sample, got %d", len(got))
	}
	if got[0].RawSize != 1000 || got[0].RestoreSize != 5000 || got[0].Snapshots != 2 {
		t.Fatalf("sample = %+v, want rawSize=1000 restoreSize=5000 snapshots=2", got[0])
	}
}

func TestRecoveryKit(t *testing.T) {
	t.Run("encryption on: contains key, password line and restore steps", func(t *testing.T) {
		dir := t.TempDir()
		appKey := strings.Repeat("a", 64)
		cfg := config.Config{AppKey: appKey, DataDir: dir, HostMountRoot: dir}
		st := newMemStore(t)
		s := mustSettings(t, st)
		s.EncryptionEnabled = true
		if err := st.UpdateSettings(s); err != nil {
			t.Fatal(err)
		}
		svc := api.NewService(cfg, st, &fakeServiceDocker{}, fakeVirsh{}, &fakeResticEngine{})

		kit, err := svc.RecoveryKit()
		if err != nil {
			t.Fatalf("RecoveryKit: %v", err)
		}
		if !strings.Contains(kit, appKey) {
			t.Error("kit must contain the APP_KEY when encryption is on")
		}
		// The derived restic repo password must appear, using the SAME derivation the
		// engine uses (restickey.Derive) — not a reinvented one.
		if !strings.Contains(kit, restickey.Derive(appKey)) {
			t.Error("kit must contain the APP_KEY-derived restic password")
		}
		if !strings.Contains(kit, "RESTIC_PASSWORD") {
			t.Error("kit must show the RESTIC_PASSWORD export line")
		}
		if !strings.Contains(kit, "restic restore") {
			t.Error("kit must contain the manual `restic restore` step")
		}
		if !strings.Contains(kit, "ENABLED") {
			t.Error("kit must state encryption is ENABLED")
		}
	})

	t.Run("encryption off: contains the no-password note, no key", func(t *testing.T) {
		dir := t.TempDir()
		appKey := strings.Repeat("b", 64)
		cfg := config.Config{AppKey: appKey, DataDir: dir, HostMountRoot: dir}
		st := newMemStore(t)
		// Default settings have encryption off (the migration flips the default, but
		// the test store starts from the schema default which is on); set it off.
		s := mustSettings(t, st)
		s.EncryptionEnabled = false
		if err := st.UpdateSettings(s); err != nil {
			t.Fatal(err)
		}
		svc := api.NewService(cfg, st, &fakeServiceDocker{}, fakeVirsh{}, &fakeResticEngine{})

		kit, err := svc.RecoveryKit()
		if err != nil {
			t.Fatalf("RecoveryKit: %v", err)
		}
		if !strings.Contains(kit, "insecure-no-password") {
			t.Error("kit must explain the no-password (--insecure-no-password) mode when encryption is off")
		}
		if strings.Contains(kit, appKey) {
			t.Error("kit must NOT expose the APP_KEY when encryption is off (no key in play)")
		}
		if !strings.Contains(kit, "restic restore") {
			t.Error("kit must still contain the manual `restic restore` step")
		}
	})
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
