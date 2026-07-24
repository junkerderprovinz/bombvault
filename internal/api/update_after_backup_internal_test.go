package api

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/dockercli"
	"github.com/junkerderprovinz/bombvault/internal/model"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// updateFakeDocker embeds the Docker interface (left nil) and overrides only the
// methods updateContainerAfterBackup exercises, recording calls so the test can
// assert whether the recreate happened. Kept in package api since the api_test
// fakeServiceDocker isn't visible to internal tests.
type updateFakeDocker struct {
	dockercli.Docker
	imageID string
	calls   []string
	// pullAuths records the registryAuth string of every PullWithAuth call, so
	// the #106 tests can assert whether a credential reached the pull.
	pullAuths []string
}

func (f *updateFakeDocker) Pull(_ context.Context, ref string) error {
	f.calls = append(f.calls, "pull:"+ref)
	f.pullAuths = append(f.pullAuths, "")
	return nil
}

// PullWithAuth records the same "pull:" label as Pull (assertions cover both
// entry points) plus the auth string the service resolved.
func (f *updateFakeDocker) PullWithAuth(_ context.Context, ref, registryAuth string) error {
	f.calls = append(f.calls, "pull:"+ref)
	f.pullAuths = append(f.pullAuths, registryAuth)
	return nil
}

func (f *updateFakeDocker) ImageID(_ context.Context, ref string) (string, error) {
	f.calls = append(f.calls, "imageID:"+ref)
	return f.imageID, nil
}

func (f *updateFakeDocker) Stop(_ context.Context, name string, _ time.Duration) error {
	f.calls = append(f.calls, "stop:"+name)
	return nil
}

func (f *updateFakeDocker) Remove(_ context.Context, name string) error {
	f.calls = append(f.calls, "remove:"+name)
	return nil
}

func (f *updateFakeDocker) CreateAndStart(_ context.Context, in model.Inspect, _ bool) error {
	f.calls = append(f.calls, "createAndStart:"+in.Name)
	return nil
}

func (f *updateFakeDocker) ImageRemove(_ context.Context, id string) error {
	f.calls = append(f.calls, "imageRemove:"+id)
	return nil
}

func newUpdateTestSvc(t *testing.T) (*Service, *store.Repo) {
	t.Helper()
	db, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() }) // close before TempDir cleanup (Windows file lock)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	st := store.New(db)
	return &Service{store: st}, st
}

// A newer pulled image must trigger a stop/remove/recreate and record a
// successful "update" run (#52).
func TestUpdateAfterBackup_RecreatesOnNewerImage(t *testing.T) {
	svc, st := newUpdateTestSvc(t)
	tg, err := st.UpsertTarget(store.Target{ContainerName: "plex"})
	if err != nil {
		t.Fatal(err)
	}
	f := &updateFakeDocker{imageID: "sha256:NEW"}
	svc.docker = f

	in := model.Inspect{Name: "/plex", Image: "sha256:OLD", Config: model.Config{Image: "plex:latest"}}
	svc.updateContainerAfterBackup(context.Background(), "plex", in, tg.ID)

	calls := strings.Join(f.calls, ",")
	for _, want := range []string{"pull:plex:latest", "imageID:plex:latest", "remove:plex", "createAndStart:/plex"} {
		if !strings.Contains(calls, want) {
			t.Fatalf("a newer image must recreate the container: missing %q in calls %v", want, f.calls)
		}
	}
	// Prune is opt-in (default off): the superseded image must be kept.
	if strings.Contains(calls, "imageRemove:") {
		t.Fatalf("prune is off by default — the old image must NOT be removed; calls %v", f.calls)
	}
	runs, err := st.ListRuns(10)
	if err != nil {
		t.Fatal(err)
	}
	var updateRun *store.Run
	for i := range runs {
		if runs[i].Kind == "update" {
			updateRun = &runs[i]
		}
	}
	if updateRun == nil {
		t.Fatalf("a successful update must record an \"update\" run; got %v", runs)
	}
	if updateRun.Status != "success" {
		t.Fatalf("update run status = %q, want success", updateRun.Status)
	}
}

// An image that did not change must NOT recreate the container and must not
// clutter the run history with a no-op update (#52).
func TestUpdateAfterBackup_SkipsWhenUpToDate(t *testing.T) {
	svc, st := newUpdateTestSvc(t)
	tg, err := st.UpsertTarget(store.Target{ContainerName: "plex"})
	if err != nil {
		t.Fatal(err)
	}
	f := &updateFakeDocker{imageID: "sha256:SAME"} // equals the running image below
	svc.docker = f

	in := model.Inspect{Name: "/plex", Image: "sha256:SAME", Config: model.Config{Image: "plex:latest"}}
	svc.updateContainerAfterBackup(context.Background(), "plex", in, tg.ID)

	calls := strings.Join(f.calls, ",")
	if strings.Contains(calls, "remove:") || strings.Contains(calls, "createAndStart:") {
		t.Fatalf("an up-to-date image must NOT recreate the container; calls %v", f.calls)
	}
	runs, _ := st.ListRuns(10)
	for _, r := range runs {
		if r.Kind == "update" {
			t.Fatalf("an up-to-date image must not record an update run; got %v", runs)
		}
	}
}

// With prune-after-update opted in, a successful update removes the superseded
// (old) image (#56).
func TestUpdateAfterBackup_PrunesOldImageWhenEnabled(t *testing.T) {
	svc, st := newUpdateTestSvc(t)
	cfg, err := st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	cfg.PruneImageAfterUpdate = true
	if err := st.UpdateSettings(cfg); err != nil {
		t.Fatal(err)
	}
	tg, err := st.UpsertTarget(store.Target{ContainerName: "plex"})
	if err != nil {
		t.Fatal(err)
	}
	f := &updateFakeDocker{imageID: "sha256:NEW"}
	svc.docker = f

	in := model.Inspect{Name: "/plex", Image: "sha256:OLD", Config: model.Config{Image: "plex:latest"}}
	svc.updateContainerAfterBackup(context.Background(), "plex", in, tg.ID)

	if !strings.Contains(strings.Join(f.calls, ","), "imageRemove:sha256:OLD") {
		t.Fatalf("prune-after-update must remove the OLD image; calls %v", f.calls)
	}
}

// TestUpdateAfterBackup_RegistryAuthReachesPull (#106): with a credential
// stored for the image's registry host, the post-backup update pull carries the
// encoded RegistryAuth; a ref on a different registry still pulls anonymously.
func TestUpdateAfterBackup_RegistryAuthReachesPull(t *testing.T) {
	svc, st := newUpdateTestSvc(t)
	svc.cfg = config.Config{AppKey: strings.Repeat("a", 64)}

	// Store one credential for ghcr.io via the same encrypt path the settings
	// PUT uses.
	blob, err := svc.EncodeRegistryAuths([]RegistryAuth{
		{Host: "ghcr.io", Username: "sponsor", Token: "s3cret"},
	})
	if err != nil {
		t.Fatal(err)
	}
	settings, err := st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	settings.RegistryAuths = blob
	if err := st.UpdateSettings(settings); err != nil {
		t.Fatal(err)
	}
	tg, err := st.UpsertTarget(store.Target{ContainerName: "tcm"})
	if err != nil {
		t.Fatal(err)
	}

	// Matching host → the encoded credential must reach the pull.
	f := &updateFakeDocker{imageID: "sha256:SAME"}
	svc.docker = f
	in := model.Inspect{Name: "/tcm", Image: "sha256:SAME", Config: model.Config{Image: "ghcr.io/owner/tcm-ui:latest"}}
	svc.updateContainerAfterBackup(context.Background(), "tcm", in, tg.ID)
	if len(f.pullAuths) != 1 {
		t.Fatalf("expected exactly one pull, got calls %v", f.calls)
	}
	want, err := dockercli.EncodeRegistryAuth("sponsor", "s3cret", "ghcr.io")
	if err != nil {
		t.Fatal(err)
	}
	if f.pullAuths[0] != want {
		t.Fatalf("ghcr.io pull must carry the stored credential: got %q, want %q", f.pullAuths[0], want)
	}

	// Non-matching host (a bare Docker Hub ref) → anonymous pull ("").
	f2 := &updateFakeDocker{imageID: "sha256:SAME"}
	svc.docker = f2
	in2 := model.Inspect{Name: "/plex", Image: "sha256:SAME", Config: model.Config{Image: "plex:latest"}}
	svc.updateContainerAfterBackup(context.Background(), "plex", in2, tg.ID)
	if len(f2.pullAuths) != 1 || f2.pullAuths[0] != "" {
		t.Fatalf("a Docker Hub ref must NOT receive the ghcr.io credential: %q", f2.pullAuths)
	}
}
