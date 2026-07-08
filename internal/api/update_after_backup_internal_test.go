package api

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

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
}

func (f *updateFakeDocker) Pull(_ context.Context, ref string) error {
	f.calls = append(f.calls, "pull:"+ref)
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
