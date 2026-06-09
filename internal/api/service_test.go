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
	"github.com/junkerderprovinz/bombvault/internal/model"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/restickey"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

func TestServiceEnsureRepoIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	cfg := config.Config{AppKey: strings.Repeat("a", 64), DataDir: dir, HostMountRoot: dir}
	st := newMemStore(t)

	eng := &fakeResticEngine{}
	svc := api.NewService(cfg, st, &fakeServiceDocker{}, eng)

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

func TestServiceModeEncryptionOn(t *testing.T) {
	cfg := config.Config{AppKey: strings.Repeat("a", 64), HostMountRoot: t.TempDir()}
	st := newMemStore(t)
	s := mustSettings(t, st)
	s.EncryptionEnabled = true
	if err := st.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}
	svc := api.NewService(cfg, st, &fakeServiceDocker{}, &fakeResticEngine{})

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
	svc := api.NewService(cfg, st, &fakeServiceDocker{}, &fakeResticEngine{})

	mode := svc.ModeFor(s)
	if mode.Encrypted {
		t.Fatal("expected non-encrypted mode when EncryptionEnabled is off")
	}
	if mode.Password != "" {
		t.Fatal("password must be empty when encryption off")
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
	d := &fakeServiceDocker{inspect: model.Inspect{
		Name:  "/plex",
		Image: "plex:latest",
		Mounts: []model.Mount{
			{Type: "bind", Source: appdata, Destination: "/config"},
			{Type: "bind", Source: "/etc/localtime", Destination: "/etc/localtime"}, // outside root → excluded
		},
	}}
	eng := &fakeResticEngine{}
	svc := api.NewService(cfg, st, d, eng)

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

// TestServiceSetIncludeFindOrCreate verifies that SetInclude creates the target
// row when it does not exist yet, rather than returning an error.
func TestServiceSetIncludeFindOrCreate(t *testing.T) {
	dir := t.TempDir()
	cfg := config.Config{AppKey: strings.Repeat("a", 64), DataDir: dir, HostMountRoot: "/host/user"}
	st := newMemStore(t)
	s := mustSettings(t, st)
	s.ContainersPath = "backups/containers"
	if err := st.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}

	d := &fakeServiceDocker{inspect: model.Inspect{
		Name:  "/radarr",
		Image: "radarr:latest",
		Mounts: []model.Mount{
			{Type: "bind", Source: "/host/user/appdata/radarr", Destination: "/config"},
		},
	}}
	svc := api.NewService(cfg, st, d, &fakeResticEngine{})

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
	if !contains(tg.AppdataPaths, "/host/user/appdata/radarr") {
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
	svc := api.NewService(cfg, st, d, &fakeResticEngine{})

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
	svc := api.NewService(cfg, st, &fakeServiceDocker{}, eng)

	got, err := svc.Snapshots(context.Background(), "plex")
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

// ---------------------------------------------------------------------------
// fakes
// ---------------------------------------------------------------------------

type fakeResticEngine struct {
	inited    []string
	backedUp  []string
	lastPaths []string
	restored  []string
	snaps     []restic.Snapshot
	initErr   error
	backupErr error
}

func (f *fakeResticEngine) Init(_ context.Context, repo string, _ restic.Mode) error {
	f.inited = append(f.inited, repo)
	return f.initErr
}

func (f *fakeResticEngine) Backup(_ context.Context, repo string, paths, _ []string, _ restic.Mode) (restic.Summary, error) {
	f.backedUp = append(f.backedUp, repo)
	f.lastPaths = paths
	if f.backupErr != nil {
		return restic.Summary{}, f.backupErr
	}
	return restic.Summary{SnapshotID: "deadbeef12345678", BytesAdded: 2048}, nil
}

func (f *fakeResticEngine) Restore(_ context.Context, repo, snapshotID, target string, _ restic.Mode) error {
	f.restored = append(f.restored, repo+":"+snapshotID+":"+target)
	return nil
}

func (f *fakeResticEngine) Snapshots(_ context.Context, _ string, _ restic.Mode) ([]restic.Snapshot, error) {
	return f.snaps, nil
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
