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

func TestExcludesARootCatchesALineAtOrAboveABackupRoot(t *testing.T) {
	roots := []string{"/host/user/user/appdata/plex", "/host/user/user/appdata/Plex [Media]"}
	cases := []struct {
		line string
		want bool
	}{
		{"/host", true},
		{"/host/user", true},
		{"/host/user/", true},
		{"/host/user/user/appdata", true},
		{"/host/user/user/appdata/plex", true},
		{"/host/user/user/appdata/plex/*", true},
		{"/host/user/user/appdata/plex/**", true},
		{"/host/*", true},
		{"/host/**/plex", true},
		{`/host/user/user/appdata/Plex \[Media\]`, true},
		{"appdata", true},
		{"user/appdata", true},
		{"pl*", true},
		{"/", true},
		{"*", true},
		{"/host/user/user/appdata/plex/Cache", false},
		{"/host/user/user/appdata/plex/*.log", false},
		{"/host/user/user/appdata/plexy", false},
		{"/host/user/user/appdata/other", false},
		{"/user/appdata", false},
		{"/host/**/Cache", false},
		{"Cache", false},
		{"*.log", false},
		{"", false},
	}
	for _, tc := range cases {
		if got := backup.ExcludesARoot(tc.line, roots); got != tc.want {
			t.Errorf("ExcludesARoot(%q) = %v, want %v", tc.line, got, tc.want)
		}
	}
	if backup.ExcludesARoot("/", nil) {
		t.Error("ExcludesARoot with no roots = true, want false")
	}
}

func TestFilesBackupIgnoresAnExcludeThatWouldLeaveTheSetsFolderOut(t *testing.T) {
	r := &fakeFilesRestic{}
	_, err := backup.BackupFileSetDir(context.Background(), backup.FileSetBackupDeps{
		SourceDir: "/host/user/data/docs",
		Repo:      "/repo/files",
		TargetID:  "set-1",
		SetName:   "docs",
		Excludes:  []string{"*.tmp", "/host", "/host/user", "/host/user/data/docs", "/host/user/data/docs/*", "docs", "/host/user/data/docs/old"},
		Restic:    r,
		Runs:      &fakeRuns{},
	})
	if err != nil {
		t.Fatalf("BackupFileSetDir: %v", err)
	}
	if got := strings.Join(r.excludes, ","); got != "*.tmp,/host/user/data/docs/old" {
		t.Fatalf("excludes = %q, want only *.tmp and the folder inside the set", got)
	}
}

func TestZFSBackupIgnoresAnExcludeThatWouldLeaveADatasetOut(t *testing.T) {
	f := newZFSFixture()
	f.deps.Excludes = []string{"*.tmp", "appdata", "/plex/*", "/plex/cache"}
	if _, err := f.run(t, t.Context()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	root, ok := f.restic.callFor(zfsTestRoot)
	if !ok {
		t.Fatalf("%s was not backed up: %v", zfsTestRoot, f.restic.calls)
	}
	if got := strings.Join(root.excludes, ","); got != "*.tmp" {
		t.Fatalf("%s excludes = %q, want only *.tmp", zfsTestRoot, got)
	}
	plex, ok := f.restic.callFor(zfsTestPlex)
	if !ok {
		t.Fatalf("%s was not backed up: %v", zfsTestPlex, f.restic.calls)
	}
	if len(plex.excludes) != 2 || plex.excludes[0] != "*.tmp" || !strings.HasSuffix(plex.excludes[1], "/cache") {
		t.Fatalf("%s excludes = %q, want *.tmp and its cache folder", zfsTestPlex, plex.excludes)
	}
}
