package api_test

import (
	"context"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/api"
	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/dockercli"
	"github.com/junkerderprovinz/bombvault/internal/model"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

func backedUpInspect() model.Inspect {
	return model.Inspect{
		ID:    "old",
		Name:  "/app",
		Image: "sha256:aaa",
		Config: model.Config{
			Image: "alpine:3.21",
			Env:   []string{"APP_SECRET=hunter2", "TZ=Europe/Vienna"},
		},
		HostConfig: model.HostConfig{
			Binds:        []string{"/mnt/user/appdata/app:/config"},
			PortBindings: map[string][]model.PortBinding{"80/tcp": {{HostPort: "8080"}}},
		},
	}
}

func TestABackupRecordsTheShapeTheChangeNoticeComparesWith(t *testing.T) {
	dir := t.TempDir()
	cfg := config.Config{AppKey: strings.Repeat("a", 64), DataDir: dir, HostMountRoot: filepath.ToSlash(dir)}
	st := newMemStore(t)
	s := mustSettings(t, st)
	s.EncryptionEnabled = false
	s.ContainersPath = "backups/containers"
	if err := st.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}
	d := &fakeServiceDocker{inspect: backedUpInspect()}
	svc := api.NewService(cfg, st, d, fakeVirsh{}, &fakeResticEngine{})
	if _, err := svc.Backup(context.Background(), "app"); err != nil {
		t.Fatal(err)
	}
	tg, err := st.GetTargetByContainer("app")
	if err != nil {
		t.Fatal(err)
	}
	shapes, err := st.BackedUpShapes()
	if err != nil {
		t.Fatal(err)
	}
	shape := shapes[tg.ID]
	if !strings.Contains(shape, `"ID":"old"`) || !strings.Contains(shape, "APP_SECRET=") {
		t.Fatalf("shape = %s", shape)
	}
	if strings.Contains(shape, "hunter2") {
		t.Fatalf("the shape holds a variable's value: %s", shape)
	}
}

func TestAFailedBackupKeepsTheShapeOfTheLastGoodOne(t *testing.T) {
	dir := t.TempDir()
	cfg := config.Config{AppKey: strings.Repeat("a", 64), DataDir: dir, HostMountRoot: filepath.ToSlash(dir)}
	st := newMemStore(t)
	s := mustSettings(t, st)
	s.EncryptionEnabled = false
	s.ContainersPath = "backups/containers"
	if err := st.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}
	// Without a source root the conventional <root>/appdata/<name> is backed
	// up, so restic runs and can fail.
	if err := os.MkdirAll(filepath.Join(dir, "appdata", "app"), 0o750); err != nil {
		t.Fatal(err)
	}
	d := &fakeServiceDocker{inspect: backedUpInspect()}
	eng := &fakeResticEngine{}
	svc := api.NewService(cfg, st, d, fakeVirsh{}, eng)
	if _, err := svc.Backup(context.Background(), "app"); err != nil {
		t.Fatal(err)
	}
	d.inspect.ID = "new"
	eng.backupErr = errors.New("repository is gone")
	if _, err := svc.Backup(context.Background(), "app"); err == nil {
		t.Fatal("want the second backup to fail")
	}
	tg, _ := st.GetTargetByContainer("app")
	shapes, _ := st.BackedUpShapes()
	if !strings.Contains(shapes[tg.ID], `"ID":"old"`) {
		t.Fatalf("shape = %s", shapes[tg.ID])
	}
}

func TestListContainersNamesWhatChangedSinceTheBackup(t *testing.T) {
	now := backedUpInspect()
	now.ID = "new"
	now.Image = "sha256:bbb"
	now.Config.Image = "alpine:3.22"
	now.Config.Env = []string{"APP_SECRET=swordfish", "TZ=Europe/Vienna", "APP_EXTRA=1"}
	now.HostConfig.PortBindings = map[string][]model.PortBinding{"80/tcp": {{HostPort: "8081"}}}
	d := &fakeServiceDocker{
		listOut: []dockercli.ContainerInfo{{ID: "new", Name: "app", State: "running"}},
		inspect: now,
	}
	h, st := newTestRouter(t, d, &fakeResticEngine{})
	tg := backedUpTarget(t, st, "app")
	if err := st.SetBackedUpShape(tg.ID, api.BackedUpShapeJSON(backedUpInspect())); err != nil {
		t.Fatal(err)
	}

	w, m := doJSON(t, h, http.MethodGet, "/api/containers", "")
	if w.Code != http.StatusOK || m["ok"] != true {
		t.Fatalf("list failed: %d %v", w.Code, m)
	}
	row := containerRow(t, m["containers"].([]any), "app")
	changes, _ := row["changedSinceBackup"].([]any)
	got := map[string]bool{}
	for _, c := range changes {
		cm := c.(map[string]any)
		key := cm["field"].(string) + ":" + cm["change"].(string)
		if n, ok := cm["name"].(string); ok {
			key += ":" + n
		}
		got[key] = true
		for _, v := range []any{cm["backup"], cm["now"]} {
			if s, _ := v.(string); strings.Contains(s, "hunter2") || strings.Contains(s, "swordfish") {
				t.Fatalf("a variable's value leaked: %v", cm)
			}
		}
	}
	for _, want := range []string{"image:changed", "port:added", "port:removed", "env:changed:APP_SECRET", "env:removed:APP_EXTRA"} {
		if !got[want] {
			t.Errorf("missing %s in %v", want, got)
		}
	}
	if got["env:changed:TZ"] {
		t.Errorf("an unchanged variable is listed: %v", got)
	}
}

func TestListContainersSaysNothingWhenTheContainerWasNotRecreated(t *testing.T) {
	d := &fakeServiceDocker{listOut: []dockercli.ContainerInfo{{ID: "old", Name: "app", State: "running"}}}
	h, st := newTestRouter(t, d, &fakeResticEngine{})
	tg := backedUpTarget(t, st, "app")
	if err := st.SetBackedUpShape(tg.ID, api.BackedUpShapeJSON(backedUpInspect())); err != nil {
		t.Fatal(err)
	}
	w, m := doJSON(t, h, http.MethodGet, "/api/containers", "")
	if w.Code != http.StatusOK {
		t.Fatalf("list failed: %d", w.Code)
	}
	row := containerRow(t, m["containers"].([]any), "app")
	if c, ok := row["changedSinceBackup"]; ok && len(c.([]any)) > 0 {
		t.Fatalf("changes = %v", c)
	}
	for _, call := range d.calls {
		if call == "inspect:app" {
			t.Fatal("an unchanged container was inspected")
		}
	}
}

func TestListContainersSeesANewImageUnderTheSameTag(t *testing.T) {
	now := backedUpInspect()
	now.ID = "new"
	now.Image = "sha256:bbb"
	d := &fakeServiceDocker{listOut: []dockercli.ContainerInfo{{ID: "new", Name: "app"}}, inspect: now}
	h, st := newTestRouter(t, d, &fakeResticEngine{})
	tg := backedUpTarget(t, st, "app")
	if err := st.SetBackedUpShape(tg.ID, api.BackedUpShapeJSON(backedUpInspect())); err != nil {
		t.Fatal(err)
	}
	_, m := doJSON(t, h, http.MethodGet, "/api/containers", "")
	row := containerRow(t, m["containers"].([]any), "app")
	changes, _ := row["changedSinceBackup"].([]any)
	if len(changes) != 1 || changes[0].(map[string]any)["change"] != "updated" {
		t.Fatalf("changes = %v", changes)
	}
}

func TestListContainersFallsBackToTheStoredDefinition(t *testing.T) {
	now := backedUpInspect()
	now.ID = "new"
	now.Config.Env = append(now.Config.Env, "APP_EXTRA=1")
	d := &fakeServiceDocker{listOut: []dockercli.ContainerInfo{{ID: "new", Name: "app"}}, inspect: now}
	h, st := newTestRouter(t, d, &fakeResticEngine{})
	def, err := marshalDefinition(backedUpInspect(), "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.UpsertTarget(store.Target{ContainerName: "app", Definition: string(def)}); err != nil {
		t.Fatal(err)
	}
	backedUpTarget(t, st, "app")
	_, m := doJSON(t, h, http.MethodGet, "/api/containers", "")
	row := containerRow(t, m["containers"].([]any), "app")
	changes, _ := row["changedSinceBackup"].([]any)
	if len(changes) != 1 || changes[0].(map[string]any)["name"] != "APP_EXTRA" {
		t.Fatalf("changes = %v", changes)
	}
}

// backedUpTarget gives name a target row with one successful backup.
func backedUpTarget(t *testing.T, st *store.Repo, name string) store.Target {
	t.Helper()
	tg, err := st.GetTargetByContainer(name)
	if err != nil {
		if tg, err = st.UpsertTarget(store.Target{ContainerName: name}); err != nil {
			t.Fatal(err)
		}
	}
	id, err := st.StartRun(tg.ID, "backup")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.FinishRun(id, "success", "abcdef12", 1, ""); err != nil {
		t.Fatal(err)
	}
	return tg
}

func TestMCPListItemsCarriesTheChangesWithoutVolumePaths(t *testing.T) {
	now := backedUpInspect()
	now.ID = "new"
	now.HostConfig.Binds = []string{"/mnt/user/appdata/app2:/config"}
	c := runningContainer("app")
	c.ID = "new"
	docker := &fakeServiceDocker{listOut: []dockercli.ContainerInfo{c}, inspect: now}
	h, st, _, key := newMCPToolRouter(t, docker, &fakeResticEngine{})
	tg := seedTarget(t, st, "app")
	seedRun(t, st, tg.ID, "backup", "success", store.RunMeta{})
	if err := st.SetBackedUpShape(tg.ID, api.BackedUpShapeJSON(backedUpInspect())); err != nil {
		t.Fatal(err)
	}
	settings := mustSettings(t, st)
	settings.ContainersEnabled = true
	if err := st.UpdateSettings(settings); err != nil {
		t.Fatal(err)
	}

	domains := mcpItemDomains(t, mcpCallTool(t, h, key, "list_items", `{"domain":"containers"}`))
	app := mcpItemsByName(t, domains["containers"])["app"]
	changes, _ := app["changedSinceBackup"].([]any)
	if len(changes) != 2 {
		t.Fatalf("changes = %v", changes)
	}
	for _, raw := range changes {
		ch := raw.(map[string]any)
		if ch["field"] != "volume" || ch["backup"] != nil || ch["now"] != nil {
			t.Fatalf("change = %v", ch)
		}
	}
}
