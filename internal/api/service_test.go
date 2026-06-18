package api_test

import (
	"context"
	"encoding/json"
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
	eng := &fakeResticEngine{}
	svc := api.NewService(cfg, st, d, fakeVirsh{}, eng)

	// Use a valid 8-hex snapshot id to pass the orchestrator's regex guard.
	restoreErr := svc.Restore(context.Background(), "Pingvin-Share-X", "deadbeef", true)
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
	inited    []string
	backedUp  []string
	lastPaths []string
	restored    []string
	forgotten   []string
	prunedRepos []string
	checked     []string
	snaps       []restic.Snapshot
	lsEntries   []restic.FileEntry
	initErr     error
	backupErr   error
	forgetPolicyErr error
	checkErr        error
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

func (f *fakeResticEngine) RestorePath(_ context.Context, repo, snapshotID, path string, _ restic.Mode) error {
	f.restored = append(f.restored, repo+":"+snapshotID+":"+path)
	return nil
}

func (f *fakeResticEngine) Restore(_ context.Context, repo, snapshotID, target string, _ restic.Mode) error {
	f.restored = append(f.restored, repo+":"+snapshotID+"->"+target)
	return nil
}

func (f *fakeResticEngine) Snapshots(_ context.Context, _ string, _ restic.Mode) ([]restic.Snapshot, error) {
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
