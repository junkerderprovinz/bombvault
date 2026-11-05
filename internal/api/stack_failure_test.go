package api_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/api"
	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/model"
	"github.com/junkerderprovinz/bombvault/internal/notify"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// failStackEngine fails the backup of a stack's project folder.
type failStackEngine struct {
	fakeResticEngine
}

func (e *failStackEngine) Backup(ctx context.Context, repo string, paths, tags []string, mode restic.Mode, excludes ...string) (restic.Summary, error) {
	if slices.ContainsFunc(tags, func(t string) bool { return strings.HasPrefix(t, "stack:") }) {
		return restic.Summary{}, errors.New("Fatal: unable to save snapshot")
	}
	return e.fakeResticEngine.Backup(ctx, repo, paths, tags, mode, excludes...)
}

func stackFixture(t *testing.T, workingDir string, eng api.ResticEngine) (*api.Service, *store.Repo) {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "stacks", "site"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "stacks", "site", "compose.yml"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{AppKey: strings.Repeat("a", 64), DataDir: dir, HostMountRoot: filepath.ToSlash(dir), HostSourceRoot: "/mnt"}
	st := newMemStore(t)
	s := mustSettings(t, st)
	s.EncryptionEnabled, s.ContainersEnabled = false, true
	s.ContainersPath = "backups/containers"
	if err := st.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}
	if _, err := st.UpsertTarget(store.Target{ContainerName: "web", IncludeInSchedule: true}); err != nil {
		t.Fatal(err)
	}
	d := &fakeServiceDocker{inspect: model.Inspect{
		Name:    "/web",
		Image:   "web:latest",
		Running: true,
		Config: model.Config{Labels: map[string]string{
			"com.docker.compose.project":             "site",
			"com.docker.compose.project.working_dir": workingDir,
		}},
	}}
	return api.NewService(cfg, st, d, fakeVirsh{}, eng), st
}

func TestAFailedStackBackupIsAFailedRunAndAMessage(t *testing.T) {
	var mu sync.Mutex
	var messages []string
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		mu.Lock()
		defer mu.Unlock()
		messages = append(messages, string(b))
	}))
	defer srv.Close()

	svc, st := stackFixture(t, "/mnt/stacks/site", &failStackEngine{})
	if err := svc.SetNotifyConfig(notify.Config{On: "failure", WebhookEnabled: true, WebhookURL: srv.URL}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Backup(context.Background(), "web"); err != nil {
		t.Fatalf("the member's own backup: %v", err)
	}
	tg, err := st.GetTargetByContainer("web")
	if err != nil {
		t.Fatal(err)
	}
	run, err := st.LastRunForTarget(tg.ID)
	if err != nil || run == nil || run.Status != "failed" || !strings.Contains(run.Error, "stack site") {
		t.Fatalf("newest run = %+v, %v, want a failed one naming the stack", run, err)
	}
	mu.Lock()
	defer mu.Unlock()
	if !slices.ContainsFunc(messages, func(m string) bool { return strings.Contains(m, "stack") && strings.Contains(m, "FAILED") }) {
		t.Fatalf("messages = %v, want one about the stack", messages)
	}
}

func TestAProjectFolderFollowsTheSectionsDefault(t *testing.T) {
	eng := &fakeResticEngine{}
	svc, st := stackFixture(t, "/mnt/stacks/site", eng)
	named, err := st.UpsertOffsiteTarget(store.OffsiteTarget{Role: store.RoleRepo, Name: "Elsewhere", Repo: "elsewhere/repo", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.PutPlacementDefault("containers", named.ID, []string{}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Backup(context.Background(), "web"); err != nil {
		t.Fatalf("Backup: %v", err)
	}
	if len(eng.backedUp) == 0 {
		t.Fatal("nothing was backed up")
	}
	for _, repo := range eng.backedUp {
		if !strings.Contains(filepath.ToSlash(repo), "elsewhere/repo") {
			t.Fatalf("backups went to %v, want every one at the default", eng.backedUp)
		}
	}
}

func TestAProjectOutsideHostDataIsNamedOnTheRun(t *testing.T) {
	svc, st := stackFixture(t, "/opt/stacks/app", &fakeResticEngine{})
	if _, err := svc.Backup(context.Background(), "web"); err != nil {
		t.Fatalf("Backup: %v", err)
	}
	tg, err := st.GetTargetByContainer("web")
	if err != nil {
		t.Fatal(err)
	}
	run, err := st.LastRunForTarget(tg.ID)
	if err != nil || run == nil || run.Status != "success" {
		t.Fatalf("run = %+v, %v", run, err)
	}
	if !strings.Contains(run.Error, store.NoteDataOutsideHost) || !strings.Contains(run.Error, "/opt/stacks/app") {
		t.Fatalf("note = %q, want it to name the project folder", run.Error)
	}
}

func TestAFolderMissingInsideHostDataIsNotCalledOutside(t *testing.T) {
	st := newMemStore(t)
	if _, err := st.UpsertTarget(store.Target{ContainerName: "app", IncludeInSchedule: true}); err != nil {
		t.Fatal(err)
	}
	d := &fakeServiceDocker{inspect: model.Inspect{
		Name: "/app", Image: "app:latest", Running: true,
		Mounts: []model.Mount{{Type: "bind", Source: "/mnt/user/appdata/k-media", Destination: "/media"}},
	}}
	dir := t.TempDir()
	cfg := config.Config{AppKey: strings.Repeat("a", 64), DataDir: dir, HostMountRoot: filepath.ToSlash(dir), HostSourceRoot: "/mnt"}
	svc := api.NewService(cfg, st, d, fakeVirsh{}, &fakeResticEngine{})
	if _, err := svc.Backup(context.Background(), "app"); err != nil {
		t.Fatalf("Backup: %v", err)
	}
	tg, err := st.GetTargetByContainer("app")
	if err != nil {
		t.Fatal(err)
	}
	run, err := st.LastRunForTarget(tg.ID)
	if err != nil || run == nil {
		t.Fatalf("run = %+v, %v", run, err)
	}
	if strings.Contains(run.Error, store.NoteDataOutsideHost) || !strings.Contains(run.Error, store.NoteDataMissing+": /mnt/user/appdata/k-media") {
		t.Fatalf("note = %q", run.Error)
	}
}
