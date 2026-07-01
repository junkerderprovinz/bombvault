package api_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/api"
	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/model"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// composeInspectState builds a stored inspect for a compose stack member, carrying
// the project/service/depends_on labels the restore enumeration + ordering read,
// with an explicit run-state-at-backup (restore only starts members that were
// running when backed up).
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

// seedStackTargetState seeds a container target with a stored compose definition +
// a backup path under the mount root, so RestoreStack enumerates it and Restore can
// reach the fake docker. mountRoot is a Linux-absolute host mount (paths.Within
// uses forward-slash semantics), so appdata paths are built with forward slashes.
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

// stackTestService builds a service + store with a real (empty) local containers
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

// TestRestoreStack exercises the full stack-restore path: all members restore
// STOPPED (leaveStopped), and with startAfter they start in dependency order.
func TestRestoreStack(t *testing.T) {
	dir := t.TempDir()
	// HostMountRoot is a Linux-absolute host mount: paths.Within validates the
	// stored appdata paths with forward-slash semantics (see the sibling
	// TestRestoreUsesStoredDefinitionWhenContainerDeleted). The repo/templates live
	// on the real Windows temp dir; the two are independent.
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

	// media stack: a depends_on b, b depends_on c, c none → start order c,b,a.
	seedStackTarget(t, st, mountRoot, "svc-a", "media", "a", "b")
	seedStackTarget(t, st, mountRoot, "svc-b", "media", "b", "c")
	seedStackTarget(t, st, mountRoot, "svc-c", "media", "c", "")
	// A target in a DIFFERENT project must never be touched.
	seedStackTarget(t, st, mountRoot, "other-1", "otherstack", "web", "")

	d := &fakeServiceDocker{liveName: ""} // absent → fresh restore path
	// Every stack member's snapshot resolves from this list (tag-scoped per name).
	eng := &fakeResticEngine{snaps: []restic.Snapshot{
		{ID: "aaaa1111", Tags: []string{"container:svc-a"}},
		{ID: "bbbb2222", Tags: []string{"container:svc-b"}},
		{ID: "cccc3333", Tags: []string{"container:svc-c"}},
		{ID: "dddd4444", Tags: []string{"container:other-1"}},
	}}
	svc := api.NewService(cfg, st, d, fakeVirsh{}, eng)

	res, err := svc.RestoreStack(context.Background(), "media", "local", true, true)
	if err != nil {
		t.Fatalf("RestoreStack: %v", err)
	}

	// Three members, in stable (alphabetical) enumeration order: svc-a, svc-b, svc-c.
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

	// Every member must have been recreated LEFT STOPPED — CreateAndStart's start
	// flag false for each (leaveStopped overrides the running-at-backup state).
	if d.createdStart {
		t.Fatalf("last CreateAndStart start=true; every stack member must be recreated stopped")
	}

	// The Start calls must have happened in dependency order c, b, a. Extract the
	// start:<name> calls from the recorded call log and check the sequence.
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

	// The container in the OTHER project must NOT have been restored or started.
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

// TestRestoreStackNotConfirmed pins the confirm gate.
func TestRestoreStackNotConfirmed(t *testing.T) {
	st := newMemStore(t)
	cfg := config.Config{AppKey: strings.Repeat("a", 64), DataDir: t.TempDir(), HostMountRoot: t.TempDir()}
	svc := api.NewService(cfg, st, &fakeServiceDocker{}, fakeVirsh{}, &fakeResticEngine{})
	if _, err := svc.RestoreStack(context.Background(), "media", "local", false, false); err == nil {
		t.Fatal("RestoreStack must reject an unconfirmed request")
	}
}

// TestRestoreStackEmpty errors when no backed-up container belongs to the project.
func TestRestoreStackEmpty(t *testing.T) {
	st := newMemStore(t)
	cfg := config.Config{AppKey: strings.Repeat("a", 64), DataDir: t.TempDir(), HostMountRoot: t.TempDir()}
	svc := api.NewService(cfg, st, &fakeServiceDocker{}, fakeVirsh{}, &fakeResticEngine{})
	_, err := svc.RestoreStack(context.Background(), "nope", "local", false, true)
	if err == nil || !strings.Contains(err.Error(), "no backed-up containers") {
		t.Fatalf("expected 'no backed-up containers' error, got %v", err)
	}
}

// TestRestoreStackRespectsRunState: startAfter starts only members that were
// running when backed up; a member stopped at backup is restored but not started.
func TestRestoreStackRespectsRunState(t *testing.T) {
	d := &fakeServiceDocker{liveName: ""}
	eng := &fakeResticEngine{snaps: []restic.Snapshot{
		{ID: "aaaa1111", Tags: []string{"container:web"}},
		{ID: "bbbb2222", Tags: []string{"container:worker"}},
	}}
	svc, st, mountRoot := stackTestService(t, eng, d)
	seedStackTargetState(t, st, mountRoot, "web", "app", "web", "", true)        // running
	seedStackTargetState(t, st, mountRoot, "worker", "app", "worker", "", false) // stopped

	res, err := svc.RestoreStack(context.Background(), "app", "local", true, true)
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

// TestRestoreStackBlocksDependentOnFailedDependency: when a dependency fails to
// restore, its dependent is held back (not started) with a clear reason — exactly
// the race the stack restore exists to avoid.
func TestRestoreStackBlocksDependentOnFailedDependency(t *testing.T) {
	// db's recreate fails deterministically, so its dependent app must be held back.
	d := &fakeServiceDocker{liveName: "", createErrName: "db"}
	eng := &fakeResticEngine{snaps: []restic.Snapshot{
		{ID: "aaaa1111", Tags: []string{"container:app"}},
	}}
	svc, st, mountRoot := stackTestService(t, eng, d)
	seedStackTarget(t, st, mountRoot, "app", "shop", "app", "db") // app depends_on db
	// db: definition-only (no snapshot) → recreate-only path → its CreateAndStart
	// fails via createErrName, so db does not come up and app is blocked.
	dbDef, err := marshalDefinition(composeInspect("db", "shop", "db", ""), "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.UpsertTarget(store.Target{ContainerName: "db", Definition: string(dbDef)}); err != nil {
		t.Fatal(err)
	}

	res, err := svc.RestoreStack(context.Background(), "shop", "local", true, true)
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
