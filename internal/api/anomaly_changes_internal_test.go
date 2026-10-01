package api

import (
	"fmt"
	"testing"
)

func TestIndexRebuildIsTracedToTheIndexFolder(t *testing.T) {
	changes := map[string]fileChange{
		"data/media.json": {kind: fileModified, size: 4 << 10},
	}
	for i := range 40 {
		changes[fmt.Sprintf("data/index/old-%d.store", i)] = fileChange{kind: fileRemoved, size: 85 << 20}
	}
	for i := range 5 {
		changes[fmt.Sprintf("data/index/new-%d.store", i)] = fileChange{kind: fileAdded, size: 92 << 20}
	}

	got := summarizeChanges(changes)
	if got.Focus != "data/index" || !got.Regenerable {
		t.Fatalf("focus %q, regenerable %v; want data/index and true", got.Focus, got.Regenerable)
	}
	if got.Total.RemovedFiles != 40 || got.Total.AddedFiles != 5 || got.Total.ChangedFiles != 1 {
		t.Fatalf("total = %+v", got.Total)
	}
	if len(got.Folders) != 1 || got.Folders[0].Path != "data/index" || got.Folders[0].RemovedBytes != 40*(85<<20) {
		t.Fatalf("folders = %+v", got.Folders)
	}
}

func TestSpreadOutChangesHaveNoFocus(t *testing.T) {
	changes := map[string]fileChange{}
	for i, dir := range []string{"photos", "documents", "music", "video", "mail", "notes", "backups", "games"} {
		changes[dir+"/a"] = fileChange{kind: fileRemoved, size: int64(10+i) << 20}
	}
	got := summarizeChanges(changes)
	if got.Focus != "" || got.Regenerable {
		t.Fatalf("focus %q, regenerable %v; want neither", got.Focus, got.Regenerable)
	}
	if len(got.Folders) != changeFolderRows || got.Other == nil || got.Other.RemovedFiles != 2 {
		t.Fatalf("folders = %+v, other = %+v", got.Folders, got.Other)
	}
	if got.Folders[0].Path != "games" {
		t.Fatalf("busiest first, got %s", got.Folders[0].Path)
	}
}

func TestDataLeavingAnOrdinaryFolderIsNotCalledRegenerable(t *testing.T) {
	changes := map[string]fileChange{}
	for i := range 20 {
		changes[fmt.Sprintf("share/scans/%d.pdf", i)] = fileChange{kind: fileRemoved, size: 2 << 20}
	}
	got := summarizeChanges(changes)
	if got.Focus != "share/scans" || got.Regenerable {
		t.Fatalf("focus %q, regenerable %v; want share/scans and false", got.Focus, got.Regenerable)
	}
}

func TestEmptyFilesAreWeighedByCount(t *testing.T) {
	changes := map[string]fileChange{
		"a/cache/x": {kind: fileRemoved}, "a/cache/y": {kind: fileRemoved},
		"a/cache/z": {kind: fileRemoved}, "a/cache/w": {kind: fileRemoved},
		"a/cache/v": {kind: fileRemoved}, "a/cache/u": {kind: fileRemoved},
		"a/cache/t": {kind: fileRemoved}, "a/cache/s": {kind: fileRemoved},
		"a/cache/r": {kind: fileRemoved}, "a/cache/q": {kind: fileRemoved},
	}
	got := summarizeChanges(changes)
	if got.Focus != "a/cache" || !got.Regenerable {
		t.Fatalf("focus %q, regenerable %v", got.Focus, got.Regenerable)
	}
}

func TestFilesBesideTheFoldersCountAsTheRest(t *testing.T) {
	got := summarizeChanges(map[string]fileChange{
		"data/index/a":    {kind: fileAdded, size: 10 << 20},
		"data/media.json": {kind: fileModified, size: 4 << 10},
	})
	if got.Other == nil || got.Other.ChangedFiles != 1 || got.Other.AddedFiles != 0 {
		t.Fatalf("other = %+v", got.Other)
	}
}
