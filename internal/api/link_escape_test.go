package api_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/api"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// linkLeadingOut puts a link at rel under the rig's mount that leads to a folder
// outside it, and returns that folder.
func linkLeadingOut(t *testing.T, rig firstBackupRig, rel string) string {
	t.Helper()
	outside := t.TempDir()
	link := filepath.Join(rig.root, rel)
	if err := os.MkdirAll(filepath.Dir(link), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("no symlinks here: %v", err)
	}
	return outside
}

func TestAFileSetBackupDoesNotReadThroughALinkAboveItsPath(t *testing.T) {
	rig := newFirstBackupRig(t)
	outside := linkLeadingOut(t, rig, "shared/parent")
	if err := os.WriteFile(filepath.Join(outside, "secret.txt"), []byte("invented"), 0o600); err != nil {
		t.Fatal(err)
	}
	set, err := rig.st.CreateFileSet(store.FileSet{Name: "docs", Path: "shared/parent/inner", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(outside, "inner"), 0o750); err != nil {
		t.Fatal(err)
	}

	_, err = rig.svc.BackupFileSet(context.Background(), set.ID)
	if err == nil || !strings.Contains(err.Error(), "link above") {
		t.Fatalf("BackupFileSet = %v, want the link refused", err)
	}
	if len(rig.eng.backedUp) != 0 || rig.eng.lastDir != "" {
		t.Fatalf("restic was asked to back up %v %q", rig.eng.backedUp, rig.eng.lastDir)
	}
	runs, err := rig.st.ListRuns(5)
	if err != nil {
		t.Fatal(err)
	}
	if len(runs) != 1 || !strings.Contains(runs[0].Error, "link above") {
		t.Fatalf("runs = %+v, want one failed run that says why", runs)
	}
}

func TestAFileSetBackupDoesNotReadThroughALinkAboveASelectedFolder(t *testing.T) {
	rig := newFirstBackupRig(t)
	linkLeadingOut(t, rig, "docs/parent")
	set, err := rig.st.CreateFileSet(store.FileSet{Name: "docs", Path: "docs", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := rig.st.SetFileSetSelectedPaths(set.ID, []string{rig.root + "/docs/parent/inner"}); err != nil {
		t.Fatal(err)
	}

	if _, err := rig.svc.BackupFileSet(context.Background(), set.ID); err == nil || !strings.Contains(err.Error(), "link above") {
		t.Fatalf("BackupFileSet = %v, want the link refused", err)
	}
}

func TestAnInPlaceFileSetRestoreDoesNotWriteThroughALink(t *testing.T) {
	rig := newFirstBackupRig(t)
	linkLeadingOut(t, rig, "shared/parent")
	set, err := rig.st.CreateFileSet(store.FileSet{Name: "docs", Path: "shared/parent/inner", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	rig.eng.snapsByRepo[rig.root+"/backups/files"] = []restic.Snapshot{{
		ID: strings.Repeat("a", 64), Tags: []string{"fileset:docs"},
		Paths: []string{rig.root + "/shared/parent/inner"},
	}}

	_, started, err := rig.svc.StartRestoreFileSet(context.Background(), set.ID, strings.Repeat("a", 64), "local", "", true, api.RestoreOptions{})

	if started || err == nil || !strings.Contains(err.Error(), "link above") {
		t.Fatalf("started=%v err=%v, want the link refused", started, err)
	}
	if len(rig.eng.restored) != 0 {
		t.Fatalf("restic was asked to restore %v", rig.eng.restored)
	}
}

func TestAContainerBackupDoesNotReadThroughALinkAboveAStoredPath(t *testing.T) {
	rig := newFirstBackupRig(t)
	outside := linkLeadingOut(t, rig, "appdata/parent")
	if err := os.MkdirAll(filepath.Join(outside, "config"), 0o750); err != nil {
		t.Fatal(err)
	}
	if _, err := rig.svc.Backup(context.Background(), "nginx"); err != nil {
		t.Fatalf("first backup: %v", err)
	}
	stored := rig.root + "/appdata/parent/config"
	if err := rig.st.SetBackupPaths("nginx", []string{stored}); err != nil {
		t.Fatal(err)
	}
	before := len(rig.eng.backedUp)

	_, err := rig.svc.Backup(context.Background(), "nginx")

	if err == nil || !strings.Contains(err.Error(), "link above") {
		t.Fatalf("Backup = %v, want the link refused", err)
	}
	if len(rig.eng.backedUp) != before {
		t.Fatalf("restic was asked to back up %v", rig.eng.backedUp[before:])
	}
}
