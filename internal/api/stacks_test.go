package api_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/api"
	"github.com/junkerderprovinz/bombvault/internal/backup"
	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/model"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// composeInspectState builds a stored inspect for a compose stack member with
// its compose labels and its run state at backup time.
func composeInspectState(name, project, service, dependsOn string, running bool) model.Inspect {
	labels := map[string]string{
		"com.docker.compose.project": project,
		"com.docker.compose.service": service,
	}
	if dependsOn != "" {
		labels["com.docker.compose.depends_on"] = dependsOn
	}
	return model.Inspect{
		Name:    "/" + name,
		Running: running,
		Config: model.Config{
			Image:  "example/" + service + ":latest",
			Labels: labels,
		},
	}
}

// composeInspect is composeInspectState for a member that was running at backup.
func composeInspect(name, project, service, dependsOn string) model.Inspect {
	return composeInspectState(name, project, service, dependsOn, true)
}

// seedStackTargetState stores a target with a compose definition and an appdata
// path under mountRoot. paths.Within works with forward slashes, so the path is
// built with them.
func seedStackTargetState(t *testing.T, st *store.Repo, mountRoot, name, project, service, dependsOn string, running bool) {
	t.Helper()
	def, err := marshalDefinition(composeInspectState(name, project, service, dependsOn, running), "")
	if err != nil {
		t.Fatalf("marshal definition for %s: %v", name, err)
	}
	if _, err := st.UpsertTarget(store.Target{
		ContainerName: name,
		AppdataPaths:  []string{mountRoot + "/appdata/" + name},
		Definition:    string(def),
	}); err != nil {
		t.Fatalf("seed target %s: %v", name, err)
	}
}

// seedStackTarget seeds a member that was running at backup.
func seedStackTarget(t *testing.T, st *store.Repo, mountRoot, name, project, service, dependsOn string) {
	t.Helper()
	seedStackTargetState(t, st, mountRoot, name, project, service, dependsOn, true)
}

// stackTestService builds a service and store with an empty local containers
// repo on disk, so RestoreStack's "latest" resolves the fake engine's snapshots.
func stackTestService(t *testing.T, eng *fakeResticEngine, d *fakeServiceDocker) (*api.Service, *store.Repo, string) {
	t.Helper()
	dir := t.TempDir()
	const mountRoot = "/host/user"
	cfg := config.Config{
		AppKey:            strings.Repeat("a", 64),
		DataDir:           dir,
		HostMountRoot:     mountRoot,
		FlashTemplatesDir: filepath.Join(dir, "flash"),
	}
	st := newMemStore(t)
	s := mustSettings(t, st)
	s.EncryptionEnabled = false
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
	return api.NewService(cfg, st, d, fakeVirsh{}, eng), st, mountRoot
}

// Members are restored stopped and, with startAfter, started in dependency order.
func TestRestoreStack(t *testing.T) {
	dir := t.TempDir()
	// paths.Within checks appdata paths with forward slashes, so the mount root is
	// a Linux path while the repo lives in the temp dir.
	const mountRoot = "/host/user"
	cfg := config.Config{
		AppKey:            strings.Repeat("a", 64),
		DataDir:           dir,
		HostMountRoot:     mountRoot,
		FlashTemplatesDir: filepath.Join(dir, "flash"),
	}
	st := newMemStore(t)
	s := mustSettings(t, st)
	s.EncryptionEnabled = false
	s.ContainersPath = "backups/containers"
	if err := st.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}
	// The local repo must exist on disk so Snapshots("latest") resolves the fake
	// engine's snapshot instead of reporting "no repo yet".
	repo := filepath.Join(dir, "backups", "containers")
	if err := os.MkdirAll(repo, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "config"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	// a depends on b and b on c, so the start order is c, b, a.
	seedStackTarget(t, st, mountRoot, "svc-a", "media", "a", "b")
	seedStackTarget(t, st, mountRoot, "svc-b", "media", "b", "c")
	seedStackTarget(t, st, mountRoot, "svc-c", "media", "c", "")
	// A target in another project stays untouched.
	seedStackTarget(t, st, mountRoot, "other-1", "otherstack", "web", "")

	d := &fakeServiceDocker{liveName: ""} // no live container: fresh restore
	eng := &fakeResticEngine{snaps: []restic.Snapshot{
		{ID: "aaaa1111", Tags: []string{"container:svc-a"}},
		{ID: "bbbb2222", Tags: []string{"container:svc-b"}},
		{ID: "cccc3333", Tags: []string{"container:svc-c"}},
		{ID: "dddd4444", Tags: []string{"container:other-1"}},
	}}
	svc := api.NewService(cfg, st, d, fakeVirsh{}, eng)

	res, err := svc.RestoreStack(context.Background(), "media", "local", "", true, true)
	if err != nil {
		t.Fatalf("RestoreStack: %v", err)
	}

	// Members come back in alphabetical order.
	if len(res.Members) != 3 {
		t.Fatalf("members = %d, want 3 (%+v)", len(res.Members), res.Members)
	}
	wantNames := []string{"svc-a", "svc-b", "svc-c"}
	for i, m := range res.Members {
		if m.Name != wantNames[i] {
			t.Fatalf("member[%d].Name = %q, want %q", i, m.Name, wantNames[i])
		}
		if !m.Restored {
			t.Fatalf("member %q not restored: %q", m.Name, m.Error)
		}
		if !m.Started {
			t.Fatalf("member %q not started (startAfter=true): %q", m.Name, m.Error)
		}
	}

	// leaveStopped overrides the run state at backup, so CreateAndStart never
	// starts a member.
	if d.createdStart {
		t.Fatalf("last CreateAndStart start=true; every stack member must be recreated stopped")
	}

	var starts []string
	for _, c := range d.calls {
		if name, ok := strings.CutPrefix(c, "start:"); ok {
			starts = append(starts, name)
		}
	}
	wantOrder := []string{"svc-c", "svc-b", "svc-a"}
	if len(starts) != len(wantOrder) {
		t.Fatalf("start calls = %v, want %v", starts, wantOrder)
	}
	for i := range wantOrder {
		if starts[i] != wantOrder[i] {
			t.Fatalf("start order = %v, want %v (dependency order)", starts, wantOrder)
		}
	}

	for _, c := range d.calls {
		if strings.Contains(c, "other-1") {
			t.Fatalf("other-project container was touched: %v", d.calls)
		}
	}
	for _, m := range res.Members {
		if m.Name == "other-1" {
			t.Fatal("other-project container leaked into the media stack result")
		}
	}
}

// A cancelled member stops the stack loop there: the result holds only that
// member, the remaining members get no run and nothing is started.
func TestRestoreStackCancelledMemberAbortsLoop(t *testing.T) {
	d := &fakeServiceDocker{liveName: ""} // no live container: fresh restore
	// restoreErr hits the first member the loop reaches. Enumeration is
	// alphabetical, so svc-a is cancelled and svc-b and svc-c are never reached.
	eng := &fakeResticEngine{
		restoreErr: context.Canceled,
		snaps: []restic.Snapshot{
			// The restore maps the stored selection against the snapshot's Paths.
			{ID: "aaaa1111", Tags: []string{"container:svc-a"}, Paths: []string{"/host/user/appdata/svc-a"}},
			{ID: "bbbb2222", Tags: []string{"container:svc-b"}, Paths: []string{"/host/user/appdata/svc-b"}},
			{ID: "cccc3333", Tags: []string{"container:svc-c"}, Paths: []string{"/host/user/appdata/svc-c"}},
		},
	}
	// A remote repo skips the local-existence check, so the restore reaches the
	// engine instead of taking the recreate-only path. The mount root stays a
	// Linux path for paths.Within.
	dir := t.TempDir()
	const mountRoot = "/host/user"
	cfg := config.Config{
		AppKey:            strings.Repeat("a", 64),
		DataDir:           dir,
		HostMountRoot:     mountRoot,
		FlashTemplatesDir: filepath.Join(dir, "flash"),
	}
	st := newMemStore(t)
	s := mustSettings(t, st)
	s.EncryptionEnabled = false
	s.ContainersPath = "rest:http://127.0.0.1:8000/containers"
	if err := st.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}
	svc := api.NewService(cfg, st, d, fakeVirsh{}, eng)
	seedStackTarget(t, st, mountRoot, "svc-a", "media", "a", "b")
	seedStackTarget(t, st, mountRoot, "svc-b", "media", "b", "c")
	seedStackTarget(t, st, mountRoot, "svc-c", "media", "c", "")

	res, err := svc.RestoreStack(context.Background(), "media", "local", "", true, true)
	if err != nil {
		t.Fatalf("RestoreStack: %v", err)
	}

	if len(res.Members) != 1 {
		t.Fatalf("a cancelled member must abort the loop: members = %d, want 1 (%+v)", len(res.Members), res.Members)
	}
	if res.Members[0].Name != "svc-a" {
		t.Fatalf("the aborted member should be the first enumerated (svc-a), got %q", res.Members[0].Name)
	}
	if res.Members[0].Restored {
		t.Fatalf("a cancelled member must not be marked restored: %+v", res.Members[0])
	}

	// The only restore run is svc-a's, recorded as cancelled rather than failed.
	runs, err := st.ListRuns(10)
	if err != nil {
		t.Fatal(err)
	}
	var restoreRuns []store.Run
	for _, run := range runs {
		if run.Kind == "restore" {
			restoreRuns = append(restoreRuns, run)
		}
	}
	if len(restoreRuns) != 1 {
		t.Fatalf("only the aborted member may record a run: got %d restore runs (%+v)", len(restoreRuns), restoreRuns)
	}
	if restoreRuns[0].Status != "cancelled" {
		t.Fatalf("the cancelled member must record status %q, got %q", "cancelled", restoreRuns[0].Status)
	}

	for _, c := range d.calls {
		if strings.HasPrefix(c, "start:") {
			t.Fatalf("a cancelled stack restore must start no containers, got %v", d.calls)
		}
	}
}

func TestRestoreStackNotConfirmed(t *testing.T) {
	st := newMemStore(t)
	cfg := config.Config{AppKey: strings.Repeat("a", 64), DataDir: t.TempDir(), HostMountRoot: t.TempDir()}
	svc := api.NewService(cfg, st, &fakeServiceDocker{}, fakeVirsh{}, &fakeResticEngine{})
	if _, err := svc.RestoreStack(context.Background(), "media", "local", "", false, false); err == nil {
		t.Fatal("RestoreStack must reject an unconfirmed request")
	}
}

// A project with no backed-up container is an error, not an empty result.
func TestRestoreStackEmpty(t *testing.T) {
	st := newMemStore(t)
	cfg := config.Config{AppKey: strings.Repeat("a", 64), DataDir: t.TempDir(), HostMountRoot: t.TempDir()}
	svc := api.NewService(cfg, st, &fakeServiceDocker{}, fakeVirsh{}, &fakeResticEngine{})
	_, err := svc.RestoreStack(context.Background(), "nope", "local", "", false, true)
	if err == nil || !strings.Contains(err.Error(), "no backed-up containers") {
		t.Fatalf("expected 'no backed-up containers' error, got %v", err)
	}
}

// With startAfter, only members that were running at backup time are started.
func TestRestoreStackRespectsRunState(t *testing.T) {
	d := &fakeServiceDocker{liveName: ""}
	eng := &fakeResticEngine{snaps: []restic.Snapshot{
		{ID: "aaaa1111", Tags: []string{"container:web"}},
		{ID: "bbbb2222", Tags: []string{"container:worker"}},
	}}
	svc, st, mountRoot := stackTestService(t, eng, d)
	seedStackTargetState(t, st, mountRoot, "web", "app", "web", "", true)        // running
	seedStackTargetState(t, st, mountRoot, "worker", "app", "worker", "", false) // stopped

	res, err := svc.RestoreStack(context.Background(), "app", "local", "", true, true)
	if err != nil {
		t.Fatalf("RestoreStack: %v", err)
	}
	byName := map[string]api.StackMemberResult{}
	for _, m := range res.Members {
		byName[m.Name] = m
	}
	if !byName["web"].Restored || !byName["web"].Started {
		t.Fatalf("running-at-backup member should be restored + started: %+v", byName["web"])
	}
	if !byName["worker"].Restored || byName["worker"].Started {
		t.Fatalf("stopped-at-backup member should be restored but NOT started: %+v", byName["worker"])
	}
	for _, c := range d.calls {
		if c == "start:worker" {
			t.Fatalf("a member stopped at backup must not be started: %v", d.calls)
		}
	}
}

// StartRestoreStack shares batchActive with every other backup and restore
// starter, so while a stack restore runs they all report busy. Each member still
// records its own restore run.
//
// The restore has to reach restic to be held in flight. A local repo under the
// "/host/user" placeholder resolves as missing and every member would take the
// recreate-only path, so the repo is remote.
func TestStartRestoreStackSingleFlight(t *testing.T) {
	d := &fakeServiceDocker{liveName: ""} // no live container: fresh restore
	eng := &fakeResticEngine{
		blockRestore:   make(chan struct{}),
		restoreEntered: make(chan struct{}, 1),
		snaps: []restic.Snapshot{
			{ID: "aaaa1111", Tags: []string{"container:web"}, Paths: []string{"/host/user/appdata/web"}},
			{ID: "bbbb2222", Tags: []string{"container:worker"}, Paths: []string{"/host/user/appdata/worker"}},
		},
	}
	dir := t.TempDir()
	const mountRoot = "/host/user"
	cfg := config.Config{
		AppKey:            strings.Repeat("a", 64),
		DataDir:           dir,
		HostMountRoot:     mountRoot,
		FlashTemplatesDir: filepath.Join(dir, "flash"),
	}
	st := newMemStore(t)
	s := mustSettings(t, st)
	s.EncryptionEnabled = false
	s.ContainersPath = "rest:http://127.0.0.1:8000/containers"
	if err := st.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}
	svc := api.NewService(cfg, st, d, fakeVirsh{}, eng)
	seedStackTarget(t, st, mountRoot, "web", "app", "web", "")
	seedStackTarget(t, st, mountRoot, "worker", "app", "worker", "")
	ctx := context.Background()

	started, err := svc.StartRestoreStack(ctx, "app", "local", "", true, true)
	if err != nil || !started {
		t.Fatalf("stack restore should start: started=%v err=%v", started, err)
	}
	// Wait until the first member's restore is inside the engine.
	select {
	case <-eng.restoreEntered:
	case <-time.After(5 * time.Second):
		runs, _ := st.ListRuns(10)
		t.Fatalf("stack restore never reached the engine; docker calls=%v runs=%+v restored=%v", d.calls, runs, eng.restored)
	}

	if started, err := svc.StartRestoreStack(ctx, "app", "local", "", true, true); err != nil || started {
		t.Fatalf("second stack restore must be rejected busy: started=%v err=%v", started, err)
	}
	if started, err := svc.StartRestore(ctx, "web", "aaaa1111", "local", false); err != nil || started {
		t.Fatalf("an in-place restore must be rejected busy: started=%v err=%v", started, err)
	}
	if started, _ := svc.StartBackup(ctx, "web"); started {
		t.Fatal("a backup must be rejected while a stack restore is in flight (shared guard)")
	}

	close(eng.blockRestore) // let the member restores finish
	waitForBackupDone(t, svc)

	runs, err := st.ListRuns(10)
	if err != nil {
		t.Fatal(err)
	}
	restoreRuns := 0
	for _, run := range runs {
		if run.Kind == "restore" && run.Status == "success" {
			restoreRuns++
		}
	}
	if restoreRuns != 2 {
		t.Fatalf("want one successful restore run per stack member (2), got %d (%+v)", restoreRuns, runs)
	}
}

// Validation runs before the goroutine starts, so a bad request fails at once
// and releases the shared guard.
func TestStartRestoreStackValidationFailsFast(t *testing.T) {
	d := &fakeServiceDocker{}
	eng := &fakeResticEngine{}
	svc, st, mountRoot := stackTestService(t, eng, d)
	seedStackTarget(t, st, mountRoot, "web", "app", "web", "")
	ctx := context.Background()

	if started, err := svc.StartRestoreStack(ctx, "app", "local", "", true, false); !errors.Is(err, backup.ErrNotConfirmed) || started {
		t.Fatalf("unconfirmed: want ErrNotConfirmed + not started, got started=%v err=%v", started, err)
	}
	if started, err := svc.StartRestoreStack(ctx, "app", "nope", "", true, true); err == nil || started {
		t.Fatalf("a bad source must fail synchronously, got started=%v err=%v", started, err)
	}
	if started, err := svc.StartRestoreStack(ctx, "ghost", "local", "", true, true); err == nil || !strings.Contains(err.Error(), "no backed-up containers") || started {
		t.Fatalf("an empty stack must fail synchronously, got started=%v err=%v", started, err)
	}

	if svc.BackupInProgress() {
		t.Fatal("failed validation must release the single-flight guard")
	}
	if len(eng.restored) != 0 {
		t.Fatalf("no restore must have run for rejected requests, got %v", eng.restored)
	}
	if len(d.calls) != 0 {
		t.Fatalf("docker must not have been touched, got %v", d.calls)
	}
}

// A member whose dependency failed to restore is not started, and its error
// says why.
func TestRestoreStackBlocksDependentOnFailedDependency(t *testing.T) {
	d := &fakeServiceDocker{liveName: "", createErrName: "db"}
	eng := &fakeResticEngine{snaps: []restic.Snapshot{
		{ID: "aaaa1111", Tags: []string{"container:app"}},
	}}
	svc, st, mountRoot := stackTestService(t, eng, d)
	seedStackTarget(t, st, mountRoot, "app", "shop", "app", "db") // app depends_on db
	// db has a definition but no snapshot, so it takes the recreate-only path,
	// where createErrName makes CreateAndStart fail.
	dbDef, err := marshalDefinition(composeInspect("db", "shop", "db", ""), "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.UpsertTarget(store.Target{ContainerName: "db", Definition: string(dbDef)}); err != nil {
		t.Fatal(err)
	}

	res, err := svc.RestoreStack(context.Background(), "shop", "local", "", true, true)
	if err != nil {
		t.Fatalf("RestoreStack: %v", err)
	}
	byName := map[string]api.StackMemberResult{}
	for _, m := range res.Members {
		byName[m.Name] = m
	}
	if byName["db"].Restored {
		t.Fatalf("db restore should have failed (no appdata paths): %+v", byName["db"])
	}
	if !byName["app"].Restored {
		t.Fatalf("app should have restored: %+v", byName["app"])
	}
	if byName["app"].Started {
		t.Fatal("app must NOT start while its dependency db is down")
	}
	if !strings.Contains(byName["app"].Error, "dependency") {
		t.Fatalf("app error should explain the held-back dependency, got %q", byName["app"].Error)
	}
	for _, c := range d.calls {
		if c == "start:app" {
			t.Fatalf("app was started despite its dependency failing: %v", d.calls)
		}
	}
}

// TestRestoreStackTakesTheProjectFolderFromTheSourceItIsGiven: the members come
// from B2, and so does the project folder unless stackDirSource names another
// place.
func TestRestoreStackTakesTheProjectFolderFromTheSourceItIsGiven(t *testing.T) {
	dir := t.TempDir()
	cfg := config.Config{AppKey: strings.Repeat("a", 64), DataDir: dir, HostMountRoot: dir, FlashTemplatesDir: filepath.Join(dir, "flash")}
	st := newMemStore(t)
	s := mustSettings(t, st)
	s.EncryptionEnabled = false
	s.ContainersPath = "backups/containers"
	if err := st.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}
	// resolveRepo joins HostMountRoot and the configured path with a plain
	// slash rather than filepath.Join, so the repo string it hands back keeps
	// HostMountRoot's own separators; local must match that, not an
	// OS-cleaned join, to compare equal to what RestorePath receives.
	local := dir + "/backups/containers"
	if err := os.MkdirAll(local, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(local, "config"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	b2, err := st.CreateOffsiteTarget(store.OffsiteTarget{Domain: "containers", Name: "B2", Repo: "s3:host/b2", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	seedStackTarget(t, st, "/host/user", "svc-a", "media", "a", "")
	eng := &fakeResticEngine{snaps: []restic.Snapshot{
		{ID: "aaaa1111", Time: "2026-09-18T03:00:00Z", Tags: []string{"container:svc-a"}},
		{ID: "eeee5555", Time: "2026-09-18T03:00:00Z", Tags: []string{"stack:media"}, Paths: []string{"/appdata/media"}},
	}}
	svc := api.NewService(cfg, st, &fakeServiceDocker{}, fakeVirsh{}, eng)

	for _, c := range []struct{ dirSource, want string }{
		{"", "s3:host/b2:eeee5555:/appdata/media"},
		{"local", local + ":eeee5555:/appdata/media"},
	} {
		eng.restored = nil
		if _, err := svc.RestoreStack(context.Background(), "media", "offsite:"+b2.ID, c.dirSource, false, true); err != nil {
			t.Fatalf("stackDirSource %q: %v", c.dirSource, err)
		}
		found := false
		for _, r := range eng.restored {
			found = found || r == c.want
		}
		if !found {
			t.Errorf("stackDirSource %q: restores = %v, want %s", c.dirSource, eng.restored, c.want)
		}
	}
}
