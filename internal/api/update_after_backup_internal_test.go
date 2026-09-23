package api

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/dockercli"
	"github.com/junkerderprovinz/bombvault/internal/model"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// updateFakeDocker implements the Docker methods updateContainerAfterBackup
// uses and records the calls; the embedded interface stays nil.
type updateFakeDocker struct {
	dockercli.Docker
	imageID string
	calls   []string
	// pullAuths holds the registryAuth of every pull, "" for an anonymous one.
	pullAuths []string
}

func (f *updateFakeDocker) Pull(_ context.Context, ref string) error {
	f.calls = append(f.calls, "pull:"+ref)
	f.pullAuths = append(f.pullAuths, "")
	return nil
}

// PullWithAuth records the same "pull:" call as Pull, plus the auth string.
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

// A newer image recreates the container and records a successful "update" run.
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
	// Pruning is off by default, so the old image stays.
	if strings.Contains(calls, "imageRemove:") {
		t.Fatalf("prune is off by default, so the old image must stay; calls %v", f.calls)
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

// An unchanged image leaves the container alone and records no update run.
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

// With prune-after-update on, a successful update removes the old image.
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

// unraidReconcilePHPRun finds the `php -r <snippet> -- <ref>` run that calls
// reloadUpdateStatus and returns its ref.
func unraidReconcilePHPRun(runs [][]string) (string, bool) {
	for _, r := range runs {
		if len(r) >= 5 && r[0] == "php" && r[1] == "-r" &&
			strings.Contains(r[2], "reloadUpdateStatus") && r[3] == "--" {
			return r[4], true
		}
	}
	return "", false
}

// An applied update asks Unraid over SSH to recheck the image's update status,
// so the Docker tab drops its stale update banner. The setting is on by default.
func TestUpdateAfterBackup_ReconcilesUnraidStatusOnUpdate(t *testing.T) {
	svc, st := newUpdateTestSvc(t)
	ssh := &fakeHostSSH{}
	svc.ssh = ssh
	tg, err := st.UpsertTarget(store.Target{ContainerName: "plex"})
	if err != nil {
		t.Fatal(err)
	}
	svc.docker = &updateFakeDocker{imageID: "sha256:NEW"}

	in := model.Inspect{Name: "/plex", Image: "sha256:OLD", Config: model.Config{Image: "plex:latest"}}
	svc.updateContainerAfterBackup(context.Background(), "plex", in, tg.ID)

	ref, ok := unraidReconcilePHPRun(ssh.runs)
	if !ok {
		t.Fatalf("an applied update must reconcile Unraid's status; runs %v", ssh.runs)
	}
	if ref != "plex:latest" {
		t.Fatalf("reconcile must pass the image ref as a separate token: got %q", ref)
	}
}

// Without an update, Unraid's status file is left alone.
func TestUpdateAfterBackup_NoReconcileWhenUpToDate(t *testing.T) {
	svc, st := newUpdateTestSvc(t)
	ssh := &fakeHostSSH{}
	svc.ssh = ssh
	tg, err := st.UpsertTarget(store.Target{ContainerName: "plex"})
	if err != nil {
		t.Fatal(err)
	}
	svc.docker = &updateFakeDocker{imageID: "sha256:SAME"}

	in := model.Inspect{Name: "/plex", Image: "sha256:SAME", Config: model.Config{Image: "plex:latest"}}
	svc.updateContainerAfterBackup(context.Background(), "plex", in, tg.ID)

	if _, ok := unraidReconcilePHPRun(ssh.runs); ok {
		t.Fatalf("no update happened, so the Unraid status must not be reconciled; runs %v", ssh.runs)
	}
}

// With the toggle disabled, an applied update must skip the Unraid reconcile.
func TestUpdateAfterBackup_ReconcileSkippedWhenDisabled(t *testing.T) {
	svc, st := newUpdateTestSvc(t)
	ssh := &fakeHostSSH{}
	svc.ssh = ssh
	cfg, err := st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	cfg.ReconcileUnraidUpdateStatus = false
	if err := st.UpdateSettings(cfg); err != nil {
		t.Fatal(err)
	}
	tg, err := st.UpsertTarget(store.Target{ContainerName: "plex"})
	if err != nil {
		t.Fatal(err)
	}
	svc.docker = &updateFakeDocker{imageID: "sha256:NEW"}

	in := model.Inspect{Name: "/plex", Image: "sha256:OLD", Config: model.Config{Image: "plex:latest"}}
	svc.updateContainerAfterBackup(context.Background(), "plex", in, tg.ID)

	if _, ok := unraidReconcilePHPRun(ssh.runs); ok {
		t.Fatalf("reconcile is disabled, so it must not run; runs %v", ssh.runs)
	}
}

// The reconcile is best effort: when it fails, the update run is still a
// success.
func TestUpdateAfterBackup_ReconcileErrorDoesNotFailUpdate(t *testing.T) {
	svc, st := newUpdateTestSvc(t)
	svc.ssh = &fakeHostSSH{runErr: errors.New("ssh down")}
	tg, err := st.UpsertTarget(store.Target{ContainerName: "plex"})
	if err != nil {
		t.Fatal(err)
	}
	svc.docker = &updateFakeDocker{imageID: "sha256:NEW"}

	in := model.Inspect{Name: "/plex", Image: "sha256:OLD", Config: model.Config{Image: "plex:latest"}}
	svc.updateContainerAfterBackup(context.Background(), "plex", in, tg.ID)

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
	if updateRun == nil || updateRun.Status != "success" {
		t.Fatalf("a reconcile failure must not fail the update run; got %v", runs)
	}
}

// A pull from a registry with a stored credential carries the encoded
// RegistryAuth; a ref on another registry pulls anonymously.
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

	// Matching host: the pull carries the credential.
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

	// A bare Docker Hub ref pulls anonymously.
	f2 := &updateFakeDocker{imageID: "sha256:SAME"}
	svc.docker = f2
	in2 := model.Inspect{Name: "/plex", Image: "sha256:SAME", Config: model.Config{Image: "plex:latest"}}
	svc.updateContainerAfterBackup(context.Background(), "plex", in2, tg.ID)
	if len(f2.pullAuths) != 1 || f2.pullAuths[0] != "" {
		t.Fatalf("a Docker Hub ref must NOT receive the ghcr.io credential: %q", f2.pullAuths)
	}
}
