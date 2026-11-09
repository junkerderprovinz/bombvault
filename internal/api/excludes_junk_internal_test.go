package api

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"
)

var junkLines = []string{".DS_Store", "Thumbs.db", "desktop.ini", "@eaDir", ".Recycle.Bin", ".Trash-*", "*.tmp"}

func TestTheFolderSetPresetSuggestsSystemJunkTickedThroughout(t *testing.T) {
	f := newPlacementFixture(t)
	res := f.do(http.MethodGet, "/api/files/sets/preset", nil)
	junk, ok := res["junk"].(map[string]any)
	if !ok || junk["app"] != "systemJunk" {
		t.Fatalf("junk = %v, want the system junk group", res["junk"])
	}
	var lines []string
	for _, e := range junk["entries"].([]any) {
		entry := e.(map[string]any)
		if entry["optional"] == true {
			t.Errorf("%v starts unticked, want every line ticked", entry["line"])
		}
		if !presetKinds[entry["kind"].(string)] {
			t.Errorf("%v: kind %v has no text in the interface", entry["line"], entry["kind"])
		}
		lines = append(lines, entry["line"].(string))
	}
	if !slices.Equal(lines, junkLines) {
		t.Fatalf("lines = %v, want %v", lines, junkLines)
	}
}

func TestANewFolderSetGetsOnlyTheExcludesItIsCreatedWith(t *testing.T) {
	f := newPlacementFixture(t)
	if err := os.MkdirAll(filepath.Join(filepath.FromSlash(f.root), "docs"), 0o750); err != nil {
		t.Fatal(err)
	}
	res := f.do(http.MethodPost, "/api/files/sets", map[string]any{"name": "Docs", "path": "docs"})
	if res["ok"] != true {
		t.Fatalf("create = %v", res)
	}
	sets, err := f.st.ListFileSets()
	if err != nil || len(sets) != 1 {
		t.Fatalf("sets = %+v, %v", sets, err)
	}
	if len(sets[0].Excludes) != 0 {
		t.Fatalf("excludes = %v, want none: the server never adds the junk group by itself", sets[0].Excludes)
	}
}

// The lines are bare names, which restic matches at any depth, so a file or
// folder of that name goes wherever it lies and everything else stays.
func TestSystemJunkLeavesOutJunkAtAnyDepth(t *testing.T) {
	f := newResticFixture(t)
	f.write("share", map[string]string{
		"keep.txt":                          "k",
		".DS_Store":                         "j",
		"photos/2024/.DS_Store":             "j",
		"photos/2024/Thumbs.db":             "j",
		"photos/2024/desktop.ini":           "j",
		"photos/2024/beach.jpg":             "k",
		"photos/@eaDir/beach.jpg/SYNOPHOTO": "j",
		".Recycle.Bin/old/report.pdf":       "j",
		"work/.Trash-1000/files/draft.txt":  "j",
		"work/build/output.tmp":             "j",
		"work/build/output.txt":             "k",
		"work/notes.tmp.txt":                "k",
	})
	ctx := context.Background()
	repo, mode := f.repoOf("files")
	if err := f.svc.engine.Init(ctx, repo, mode); err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(filepath.FromSlash(f.root), "src", "share")
	sum, err := f.svc.engine.BackupDir(ctx, repo, src, []string{"junk"}, mode, systemJunkLines()...)
	if err != nil {
		t.Fatal(err)
	}
	entries, err := f.svc.engine.Ls(ctx, repo, sum.SnapshotID, mode)
	if err != nil {
		t.Fatal(err)
	}
	var files []string
	for _, e := range entries {
		if e.Type == "file" {
			files = append(files, strings.TrimPrefix(e.Path, "/"))
		}
	}
	sort.Strings(files)
	want := []string{"keep.txt", "photos/2024/beach.jpg", "work/build/output.txt", "work/notes.tmp.txt"}
	if !slices.Equal(files, want) {
		t.Fatalf("backed up %v, want %v", files, want)
	}
}

func systemJunkLines() []string {
	var lines []string
	for _, e := range systemJunk.entries {
		lines = append(lines, e.line)
	}
	return lines
}
