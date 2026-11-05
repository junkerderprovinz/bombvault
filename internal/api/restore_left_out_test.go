package api_test

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/api"
	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/model"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// olderBackupOfTwoFolders is a container whose older backup holds app and
// app-media, while the last backup only stored app.
func olderBackupOfTwoFolders(t *testing.T) (svc *api.Service, st *store.Repo, eng *fakeResticEngine, media string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("paths.Within only reads Linux paths")
	}
	dir := t.TempDir()
	cfg := config.Config{AppKey: strings.Repeat("a", 64), DataDir: dir, HostMountRoot: dir, FlashTemplatesDir: filepath.Join(dir, "flash")}
	st = newMemStore(t)
	s := mustSettings(t, st)
	s.EncryptionEnabled = false
	s.ContainersPath = "rest:http://127.0.0.1:8000/containers"
	if err := st.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}
	app := dir + "/appdata/app"
	media = dir + "/appdata/app-media"
	if err := os.MkdirAll(app, 0o750); err != nil {
		t.Fatal(err)
	}
	def, err := marshalDefinition(model.Inspect{Name: "/app", Running: true, Config: model.Config{Image: "app:1"}}, "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.UpsertTarget(store.Target{ContainerName: "app", AppdataPaths: []string{app}, Definition: string(def)}); err != nil {
		t.Fatal(err)
	}
	eng = &fakeResticEngine{snaps: []restic.Snapshot{{ID: "aaaa1111", Tags: []string{"container:app"}, Paths: []string{app, media}}}}
	return api.NewService(cfg, st, &fakeServiceDocker{}, fakeVirsh{}, eng), st, eng, media
}

func restoredPaths(eng *fakeResticEngine) []string {
	out := make([]string, len(eng.restored))
	for i, r := range eng.restored {
		out[i] = r[strings.LastIndex(r, ":")+1:]
	}
	return out
}

func lastRestoreNote(t *testing.T, st *store.Repo, name string) string {
	t.Helper()
	tg, err := st.GetTargetByContainer(name)
	if err != nil {
		t.Fatal(err)
	}
	run, err := st.LastRestoreRun(tg.ID)
	if err != nil || run == nil {
		t.Fatalf("no restore run: %v, %v", run, err)
	}
	return run.Error
}

func TestAnOlderBackupBringsBackAFolderThatIsGone(t *testing.T) {
	svc, st, eng, media := olderBackupOfTwoFolders(t)

	if err := svc.Restore(context.Background(), "app", "aaaa1111", true, "local", false); err != nil {
		t.Fatal(err)
	}

	if !slices.Contains(restoredPaths(eng), media) {
		t.Errorf("restored %v, want %s among them", eng.restored, media)
	}
	if note := lastRestoreNote(t, st, "app"); note != "" {
		t.Errorf("run note = %q, want none", note)
	}
}

func TestAnOlderBackupLeavesAFolderThatHoldsFilesAndSaysSo(t *testing.T) {
	svc, st, eng, media := olderBackupOfTwoFolders(t)
	if err := os.MkdirAll(media, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(media+"/new.txt", []byte("newer than the backup"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := svc.Restore(context.Background(), "app", "aaaa1111", true, "local", false); err != nil {
		t.Fatal(err)
	}

	if slices.Contains(restoredPaths(eng), media) {
		t.Errorf("restored %v, want %s left alone", eng.restored, media)
	}
	if note := lastRestoreNote(t, st, "app"); !strings.HasPrefix(note, store.NoteRestoreLeftOut+": ") || !strings.Contains(note, "app-media") {
		t.Errorf("run note = %q, want the left-out folder named", note)
	}
}
