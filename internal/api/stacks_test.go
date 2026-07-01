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

// composeInspect builds a stored inspect for a compose stack member, carrying the
// project/service/depends_on labels the restore enumeration + ordering read.
func composeInspect(name, project, service, dependsOn string) model.Inspect {
	labels := map[string]string{
		"com.docker.compose.project": project,
		"com.docker.compose.service": service,
	}
	if dependsOn != "" {
		labels["com.docker.compose.depends_on"] = dependsOn
	}
	return model.Inspect{
		Name:    "/" + name,
		Running: true, // running-at-backup: proves leaveStopped overrides "start"
		Config: model.Config{
			Image:  "example/" + service + ":latest",
			Labels: labels,
		},
	}
}

// seedStackTarget seeds a container target with a stored compose definition + a
// backup path under the mount root, so RestoreStack enumerates it and Restore can
// reach the fake docker. mountRoot is a Linux-absolute host mount (paths.Within
// uses forward-slash semantics), so appdata paths are built with forward slashes.
func seedStackTarget(t *testing.T, st *store.Repo, mountRoot, name, project, service, dependsOn string) {
	t.Helper()
	def, err := marshalDefinition(composeInspect(name, project, service, dependsOn), "")
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
