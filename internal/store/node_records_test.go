package store_test

import (
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

func TestANodeSetUpAnewGetsBackWhatItLacksOfItsRecords(t *testing.T) {
	old := newRepo(t)
	tg, err := old.UpsertTarget(store.Target{ContainerName: "plex", AppdataPaths: []string{"/mnt/user/appdata/plex"}, Definition: `{"inspect":{}}`})
	if err != nil {
		t.Fatal(err)
	}
	if err := old.SetExcludes("plex", []string{"cache/"}); err != nil {
		t.Fatal(err)
	}
	if err := old.SetStopContainers("plex", []string{"db"}); err != nil {
		t.Fatal(err)
	}
	docs, err := old.CreateFileSet(store.FileSet{Name: "Docs", Path: "docs", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := old.SetFileSetSelectedPaths(docs.ID, []string{"/docs/letters"}); err != nil {
		t.Fatal(err)
	}
	if _, err := old.CreateFileSet(store.FileSet{Name: "Photos", Path: "photos"}); err != nil {
		t.Fatal(err)
	}
	done, err := old.StartRun(tg.ID, "backup")
	if err != nil {
		t.Fatal(err)
	}
	if err := old.FinishRun(done, "success", "snap-1", 42, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := old.StartRun(tg.ID, "backup"); err != nil {
		t.Fatal(err)
	}
	rec, err := old.OwnRecords(10)
	if err != nil {
		t.Fatal(err)
	}
	if len(rec.Targets) != 1 || len(rec.FileSets) != 2 || len(rec.Runs) != 1 {
		t.Fatalf("records = %+v, want the container, both folder sets and the finished run", rec)
	}

	anew := newRepo(t)
	photos, err := anew.CreateFileSet(store.FileSet{Name: "Photos", Path: "pictures"})
	if err != nil {
		t.Fatal(err)
	}
	if err := anew.RestoreOwnRecords(rec); err != nil {
		t.Fatal(err)
	}
	if err := anew.RestoreOwnRecords(rec); err != nil {
		t.Errorf("the same records given back twice: %v", err)
	}
	back, err := anew.GetTargetByContainer("plex")
	if err != nil || back.ID != tg.ID || back.Definition != `{"inspect":{}}` || len(back.Excludes) != 1 || len(back.StopContainers) != 1 || len(back.AppdataPaths) != 1 {
		t.Errorf("the container came back as %+v %v", back, err)
	}
	sets, err := anew.ListFileSets()
	if err != nil || len(sets) != 2 {
		t.Fatalf("folder sets = %+v %v", sets, err)
	}
	for _, fs := range sets {
		switch fs.Name {
		case "Docs":
			if fs.ID != docs.ID || !fs.Enabled || len(fs.SelectedPaths) != 1 {
				t.Errorf("Docs came back as %+v", fs)
			}
		case "Photos":
			if fs.ID != photos.ID || fs.Path != "pictures" {
				t.Errorf("the node's own Photos became %+v, want it kept", fs)
			}
		}
	}
	runs, err := anew.ListRuns(10)
	if err != nil || len(runs) != 1 || runs[0].ID != done || runs[0].Bytes != 42 || runs[0].FinishedAt == nil {
		t.Errorf("runs = %+v %v, want the finished one", runs, err)
	}
}
