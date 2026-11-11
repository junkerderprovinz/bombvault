package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"slices"
	"testing"

	"github.com/junkerderprovinz/parleyport/kit/fortress"
	"github.com/junkerderprovinz/parleyport/kit/group"
	"github.com/junkerderprovinz/parleyport/kit/status"

	"github.com/junkerderprovinz/bombvault/internal/model"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// fakeImporter is a fortress with KnightLoader in knightLoaderContainer, which
// takes its backup back.
type fakeImporter struct {
	members  []fortress.Member
	reported map[string]string
	to       []string
	body     []byte
	n        int64
	fail     error
}

func newFakeImporter() *fakeImporter {
	return &fakeImporter{
		members: []fortress.Member{
			{ID: "t1", App: fortress.App{ID: "towertorch"}, Direct: true, Actions: []string{fortress.ImportAction}},
			{ID: "k1", App: fortress.App{ID: "knightloader", Name: "KnightLoader"}, Direct: true,
				Actions: []string{fortress.ExportAction, fortress.ImportAction}},
		},
		reported: map[string]string{"t1": "", "k1": knightLoaderContainer[:12]},
	}
}

func (f *fakeImporter) Members() []fortress.Member { return f.members }

func (f *fakeImporter) Status(_ context.Context, id, _ string) (status.Report, error) {
	return status.Report{Instance: status.Instance{Container: f.reported[id]}}, nil
}

func (f *fakeImporter) Import(_ context.Context, id string, backup io.Reader, n int64) error {
	f.to = append(f.to, id)
	if f.fail != nil {
		return f.fail
	}
	b, err := io.ReadAll(backup)
	f.body, f.n = b, n
	return err
}

// appContainerDocker answers for one running container.
type appContainerDocker struct {
	*placementDocker
	id      string
	running bool
}

func (d *appContainerDocker) Inspect(context.Context, string) (model.Inspect, error) {
	return model.Inspect{ID: d.id, Running: d.running}, nil
}

// appImportFixture is appExportFixture with the switch on, the container
// running and restic telling the size of the app backup.
func appImportFixture(t *testing.T) (*placementFixture, *appExportEngine, *fakeImporter) {
	t.Helper()
	f, eng := appExportFixture(t)
	f.svc.docker = &appContainerDocker{placementDocker: f.dock, id: knightLoaderContainer, running: true}
	f.eng.files = map[string][]restic.FileEntry{"e1e1e1e1": {
		{Path: "/appexport", Type: "dir"},
		{Path: "/appexport/knightloader/knightloader", Type: "file", Size: int64(len(zipBackup))},
	}}
	if _, err := f.st.MutateSettings(func(s *store.Settings) error { s.AppImportEnabled = true; return nil }); err != nil {
		t.Fatal(err)
	}
	return f, eng, newFakeImporter()
}

func importCode(err error) string {
	var refused *appImportRefusal
	if errors.As(err, &refused) {
		return refused.code
	}
	return ""
}

func TestTheOwnBackupGoesBackToTheAppInTheContainer(t *testing.T) {
	f, eng, im := appImportFixture(t)

	if err := f.svc.importAppExportTo(context.Background(), im, "knightloader", "local", "e1e1e1e1"); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(im.to, []string{"k1"}) || !bytes.Equal(im.body, zipBackup) || im.n != int64(len(zipBackup)) {
		t.Fatalf("handed %v %d bytes announced as %d, want KnightLoader its backup", im.to, len(im.body), im.n)
	}
	if want := []string{"e1e1e1e1 /appexport/knightloader/knightloader"}; !slices.Equal(eng.dumped, want) {
		t.Fatalf("streamed %v, want %v", eng.dumped, want)
	}
}

func TestNothingGoesBackWhileTheSwitchIsOff(t *testing.T) {
	f, _, im := appImportFixture(t)
	if _, err := f.st.MutateSettings(func(s *store.Settings) error { s.AppImportEnabled = false; return nil }); err != nil {
		t.Fatal(err)
	}
	err := f.svc.importAppExportTo(context.Background(), im, "knightloader", "local", "e1e1e1e1")
	if importCode(err) != appImportOff || len(im.to) != 0 {
		t.Fatalf("got %v and handed %v, want the switch to stop it", err, im.to)
	}
}

func TestABackupGoesBackOnlyToItsOwnAppOnThisNetwork(t *testing.T) {
	for _, tc := range []struct {
		name  string
		setup func(*placementFixture, *fakeImporter)
		want  string
	}{
		{"an app reached through the relay alone", func(_ *placementFixture, im *fakeImporter) { im.members[1].Direct = false }, appImportRelay},
		{"an app that takes no backups", func(_ *placementFixture, im *fakeImporter) { im.members[1].Actions = []string{fortress.ExportAction} }, appImportNoApp},
		{"another app in the container", func(_ *placementFixture, im *fakeImporter) { im.members[1].App.ID = "arrowloop" }, appImportOtherApp},
		{"a container that is stopped", func(f *placementFixture, _ *fakeImporter) { f.svc.docker.(*appContainerDocker).running = false }, appImportNotRunning},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f, eng, im := appImportFixture(t)
			tc.setup(f, im)
			err := f.svc.importAppExportTo(context.Background(), im, "knightloader", "local", "e1e1e1e1")
			if got := importCode(err); got != tc.want {
				t.Fatalf("refused with %q (%v), want %q", got, err, tc.want)
			}
			if len(im.to) != 0 || len(eng.dumped) != 0 {
				t.Fatalf("handed %v and streamed %v", im.to, eng.dumped)
			}
		})
	}
}

func TestTheAppsRefusalComesBackAsItsReason(t *testing.T) {
	for _, tc := range []struct {
		fail error
		want string
	}{
		{&fortress.Refusal{Status: 409, Code: "busy"}, appImportBusy},
		{&fortress.Refusal{Status: 409, Code: "version_too_new"}, appImportVersionTooNew},
		{&fortress.Refusal{Status: 409, Code: "wrong_app"}, appImportWrongApp},
		{&fortress.Refusal{Status: 409, Code: "key_missing"}, appImportKeyMissing},
		{&fortress.Refusal{Status: 400, Code: "stream_broken"}, appImportBroken},
		{&fortress.Refusal{Status: 409, Code: "something_else"}, appImportByApp},
		{fortress.ErrPermissionOff, appImportPermissionOff},
		{fortress.ErrNoSuchAction, appImportNoApp},
		{group.ErrNotDirect, appImportRelay},
		{group.ErrStreamBroken, appImportBroken},
	} {
		t.Run(tc.want, func(t *testing.T) {
			f, _, im := appImportFixture(t)
			im.fail = tc.fail
			err := f.svc.importAppExportTo(context.Background(), im, "knightloader", "local", "e1e1e1e1")
			if got := importCode(err); got != tc.want {
				t.Fatalf("refused with %q (%v), want %q", got, err, tc.want)
			}
		})
	}
}

func TestOnlyAnAppBackupOfThisContainerGoesBack(t *testing.T) {
	f, _, im := appImportFixture(t)
	err := f.svc.importAppExportTo(context.Background(), im, "knightloader", "local", "a1a1a1a1")
	if importCode(err) != appImportNotAnAppBackup || len(im.to) != 0 {
		t.Fatalf("got %v and handed %v, want the container's own backup refused", err, im.to)
	}
}

func TestGivingABackupBackNeedsAPersonSignedIn(t *testing.T) {
	f, _, _ := appImportFixture(t)
	code, out := f.call(http.MethodPost, "/api/containers/knightloader/appexports/e1e1e1e1/import", map[string]any{})
	if code != http.StatusForbidden || out["code"] != codeNoPassword {
		t.Fatalf("status %d with %v, want the refusal that asks for a password", code, out)
	}
	f.signIn()
	code, out = f.call(http.MethodPost, "/api/containers/knightloader/appexports/e1e1e1e1/import", map[string]any{})
	if code != http.StatusOK || out["ok"] != false || out["code"] != appImportNoFortress {
		t.Fatalf("status %d with %v, want the refusal of a server in no fortress", code, out)
	}
}

func TestBombVaultAsksForThePermissionToGiveBackupsBack(t *testing.T) {
	svc, _ := newFortressSvc(t)
	if cfg := svc.fortressConfig(fortress.Config{}, nil); !slices.Contains(cfg.Uses, fortress.ImportAction) {
		t.Fatalf("uses %v, want %s among them", cfg.Uses, fortress.ImportAction)
	}
}

func appImportOn(t *testing.T, f *placementFixture) bool {
	t.Helper()
	s, err := f.st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	return s.AppImportEnabled
}

func TestTheSwitchForGivingBackupsBackIsSavedAndKept(t *testing.T) {
	f := newPlacementFixture(t)
	settings, err := f.st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	v := toView(settings)
	if v.AppImportEnabled == nil || *v.AppImportEnabled {
		t.Fatalf("the settings say %v, want the switch there and off", v.AppImportEnabled)
	}

	on := true
	v.AppImportEnabled = &on
	if res := f.do("PUT", "/api/settings", v); res["ok"] != true {
		t.Fatalf("PUT: %v", res["error"])
	}
	if !appImportOn(t, f) {
		t.Fatal("the switch did not go on")
	}

	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var older map[string]any
	if err := json.Unmarshal(b, &older); err != nil {
		t.Fatal(err)
	}
	delete(older, "appImportEnabled")
	if res := f.do("PUT", "/api/settings", older); res["ok"] != true {
		t.Fatalf("PUT without the switch: %v", res["error"])
	}
	if !appImportOn(t, f) {
		t.Fatal("a body without the switch turned it off")
	}
}

func TestSettingsExportImportCarriesTheSwitchForGivingBackupsBack(t *testing.T) {
	src, srcStore := newPortableHandler(t, appKeyA)
	seedSource(t, src, srcStore)
	if _, err := srcStore.MutateSettings(func(s *store.Settings) error { s.AppImportEnabled = true; return nil }); err != nil {
		t.Fatal(err)
	}
	body, _ := doExport(t, src, "")

	dst, dstStore := newPortableHandler(t, appKeyB)
	if env := doImport(t, dst, body, "?apply=true"); env["ok"] != true {
		t.Fatalf("apply failed: %v", env)
	}
	if got, err := dstStore.GetSettings(); err != nil || !got.AppImportEnabled {
		t.Fatalf("switch = %v with %v, want it carried over", got.AppImportEnabled, err)
	}

	var raw map[string]any
	if err := json.Unmarshal(body, &raw); err != nil {
		t.Fatal(err)
	}
	delete(raw["settings"].(map[string]any), "appImportEnabled")
	older, err := json.Marshal(raw)
	if err != nil {
		t.Fatal(err)
	}
	if env := doImport(t, dst, older, "?apply=true"); env["ok"] != true {
		t.Fatalf("apply of a file without the switch failed: %v", env)
	}
	if got, err := dstStore.GetSettings(); err != nil || !got.AppImportEnabled {
		t.Fatalf("switch = %v with %v, want a file without it to keep it", got.AppImportEnabled, err)
	}
}
