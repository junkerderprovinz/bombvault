package api_test

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/api"
	"github.com/junkerderprovinz/bombvault/internal/model"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// The fingerprint a run records is what lets the detectors tell a selection the
// user narrowed from a source that lost its data. A change the user made has to
// show up in it, and a save that changed nothing must not.
func TestExcludesChangeRecordsANewFingerprint(t *testing.T) {
	d := &fakeServiceDocker{inspect: model.Inspect{
		Name:   "/plex",
		Image:  "plex:latest",
		Mounts: []model.Mount{{Type: "bind", Source: "/host/appdata/plex", Destination: "/config"}},
	}}
	secs := 2.5
	eng := &fakeResticEngine{backupSummaries: []restic.Summary{{
		SnapshotID:          "deadbeef12345678",
		BytesAdded:          2048,
		FilesNew:            4,
		TotalBytesProcessed: 40 << 30,
		TotalFilesProcessed: 900,
		TotalDuration:       &secs,
		Elapsed:             2500 * time.Millisecond,
	}}}
	h, st, svc, dir := newTestRouterSvcDir(t, d, eng)

	first := backupAndReadFingerprint(t, h, st, svc, "plex")
	if run := newestRun(t, st, "plex"); run.SourceBytes == nil || *run.SourceBytes != 40<<30 || run.ResticMS == nil || *run.ResticMS != 2500 {
		t.Fatalf("the run recorded no source figures: %+v", run)
	}

	if w, _ := doJSON(t, h, http.MethodPatch, "/api/containers/plex", `{"excludes":["logs"]}`); w.Code != http.StatusOK {
		t.Fatalf("patch excludes: status %d", w.Code)
	}
	excluded := backupAndReadFingerprint(t, h, st, svc, "plex")
	if excluded == first {
		t.Fatalf("excluding a folder left the fingerprint at %q, so the drop in source size would read as data that vanished", first)
	}

	if w, _ := doJSON(t, h, http.MethodPatch, "/api/containers/plex", `{"excludes":["logs"]}`); w.Code != http.StatusOK {
		t.Fatalf("patch excludes again: status %d", w.Code)
	}
	if again := backupAndReadFingerprint(t, h, st, svc, "plex"); again != excluded {
		t.Fatalf("saving the same excludes moved the fingerprint from %q to %q, so every save would re-base the history", excluded, again)
	}

	sub := filepath.ToSlash(filepath.Join(dir, "appdata", "plex", "config"))
	if err := os.MkdirAll(sub, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := st.SetBackupPaths("plex", []string{sub}); err != nil {
		t.Fatal(err)
	}
	if narrowed := backupAndReadFingerprint(t, h, st, svc, "plex"); narrowed == excluded {
		t.Fatalf("narrowing the folder selection kept the fingerprint %q", narrowed)
	}
}

// TestAFolderSetRecordsItsOwnFingerprint: a set's paths are its selection, and
// the set keeps a fingerprint of its own rather than sharing the containers'.
func TestAFolderSetRecordsItsOwnFingerprint(t *testing.T) {
	h, st, svc, root := newTestRouterSvcDir(t, &fakeServiceDocker{}, &fakeResticEngine{})
	if err := os.MkdirAll(filepath.Join(root, "media"), 0o750); err != nil {
		t.Fatal(err)
	}
	set, err := st.CreateFileSet(store.FileSet{Name: "media", Path: "media"})
	if err != nil {
		t.Fatal(err)
	}

	before := backupSetAndReadFingerprint(t, h, st, svc, set.ID)
	if err := st.UpdateFileSet(store.FileSet{ID: set.ID, Name: "media", Path: "media", Excludes: []string{"*.part"}}); err != nil {
		t.Fatal(err)
	}
	if after := backupSetAndReadFingerprint(t, h, st, svc, set.ID); after == before {
		t.Fatalf("the set's excludes changed and its fingerprint stayed %q", before)
	}
}

func backupAndReadFingerprint(t *testing.T, h http.Handler, st *store.Repo, svc *api.Service, name string) string {
	t.Helper()
	if w, _ := doJSON(t, h, http.MethodPost, "/api/containers/"+name+"/backup", ""); w.Code != http.StatusOK {
		t.Fatalf("start backup: status %d", w.Code)
	}
	waitForBackupRun(t, st)
	waitForBackupDone(t, svc)
	return newestFingerprint(t, st, name)
}

func backupSetAndReadFingerprint(t *testing.T, h http.Handler, st *store.Repo, svc *api.Service, setID string) string {
	t.Helper()
	if w, _ := doJSON(t, h, http.MethodPost, "/api/files/sets/"+setID+"/backup", ""); w.Code != http.StatusOK {
		t.Fatalf("start set backup: status %d", w.Code)
	}
	waitForBackupDone(t, svc)
	runs, err := st.ItemSeries(setID, "backup", 1<<62, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(runs) == 0 || runs[0].SelectionFP == nil {
		t.Fatalf("the newest run of set %s recorded no fingerprint: %+v", setID, runs)
	}
	return *runs[0].SelectionFP
}

func newestFingerprint(t *testing.T, st *store.Repo, name string) string {
	t.Helper()
	run := newestRun(t, st, name)
	if run.SelectionFP == nil {
		t.Fatalf("the newest run of %s recorded no fingerprint: %+v", name, run)
	}
	return *run.SelectionFP
}

func newestRun(t *testing.T, st *store.Repo, name string) store.SeriesRun {
	t.Helper()
	tg, err := st.GetTargetByContainer(name)
	if err != nil {
		t.Fatal(err)
	}
	runs, err := st.ItemSeries(tg.ID, "backup", 1<<62, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(runs) == 0 {
		t.Fatalf("no run recorded for %s", name)
	}
	return runs[0]
}
