package backup_test

import (
	"context"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/backup"
)

var everythingLines = []string{"/", " // ", "/.", "/config/..", "*", "**", "*/", "/*", "/**", "**/*", "/**/*"}

func TestExcludesEverythingCatchesOnlyALineThatMatchesEveryPath(t *testing.T) {
	for _, line := range everythingLines {
		if !backup.ExcludesEverything(line) {
			t.Errorf("ExcludesEverything(%q) = false, want true", line)
		}
	}
	for _, line := range []string{"", ".", "a/..", "*.log", ".git", "/config", "/config/*", "/*/cache", "**/cache", "../*"} {
		if backup.ExcludesEverything(line) {
			t.Errorf("ExcludesEverything(%q) = true, want false", line)
		}
	}
}

func TestFilesBackupIgnoresAnExcludeThatWouldLeaveEverythingOut(t *testing.T) {
	r := &fakeFilesRestic{}
	_, err := backup.BackupFileSetDir(context.Background(), backup.FileSetBackupDeps{
		SourceDir: "/host/user/data/docs",
		Repo:      "/repo/files",
		TargetID:  "set-1",
		SetName:   "docs",
		Excludes:  append([]string{"*.tmp"}, everythingLines...),
		Restic:    r,
		Runs:      &fakeRuns{},
	})
	if err != nil {
		t.Fatalf("BackupFileSetDir: %v", err)
	}
	if got := strings.Join(r.excludes, ","); got != "*.tmp" {
		t.Fatalf("excludes = %q, want only *.tmp", got)
	}
}

func TestZFSBackupIgnoresAnExcludeThatWouldLeaveEverythingOut(t *testing.T) {
	f := newZFSFixture()
	f.deps.Excludes = append([]string{"*.tmp"}, everythingLines...)
	if _, err := f.run(t, t.Context()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, member := range []string{zfsTestRoot, zfsTestPlex} {
		call, ok := f.restic.callFor(member)
		if !ok {
			t.Fatalf("%s was not backed up: %v", member, f.restic.calls)
		}
		if got := strings.Join(call.excludes, ","); got != "*.tmp" {
			t.Fatalf("%s excludes = %q, want only *.tmp", member, got)
		}
	}
}
