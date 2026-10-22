package api_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/api"
	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/model"
	"github.com/junkerderprovinz/bombvault/internal/restic"
)

// appdataService backs up a container "vw" whose appdata sits at
// <root>/disks/ssd/appdata/vw.
func appdataService(t *testing.T, eng *fakeResticEngine) (*api.Service, string, string) {
	t.Helper()
	dir := t.TempDir()
	root := filepath.ToSlash(dir)
	cfg := config.Config{
		AppKey: strings.Repeat("a", 64), DataDir: dir,
		HostMountRoot: root, HostSourceRoot: root, DataRootSegments: []string{"appdata"},
	}
	st := newMemStore(t)
	s := mustSettings(t, st)
	s.EncryptionEnabled = false
	s.ContainersPath = "backups/containers"
	if err := st.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}
	appdata := root + "/disks/ssd/appdata/vw"
	if err := os.MkdirAll(appdata, 0o750); err != nil {
		t.Fatal(err)
	}
	repo := filepath.Join(dir, "backups", "containers")
	if err := os.MkdirAll(repo, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "config"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	d := &fakeServiceDocker{inspect: model.Inspect{
		Name: "/vw", Image: "vaultwarden/server:latest", Running: true,
		Mounts: []model.Mount{{Type: "bind", Source: appdata, Destination: "/data"}},
	}}
	return api.NewService(cfg, st, d, fakeVirsh{}, eng), root, appdata
}

// mountTable writes a mountinfo fixture listing the given mount points.
func mountTable(t *testing.T, points ...string) string {
	t.Helper()
	var b strings.Builder
	for i, p := range points {
		b.WriteString(strings.Join([]string{"1" + string(rune('0'+i)), "1", "0:1", "/", p, "rw", "-", "ext4", "/dev/x", "rw"}, " "))
		b.WriteString("\n")
	}
	f := filepath.Join(t.TempDir(), "mountinfo")
	if err := os.WriteFile(f, []byte(b.String()), 0o600); err != nil {
		t.Fatal(err)
	}
	return f
}

func TestBackupRefusesAFolderWhoseDiskDidNotMount(t *testing.T) {
	eng := &fakeResticEngine{}
	svc, root, appdata := appdataService(t, eng)
	if err := os.WriteFile(filepath.Join(appdata, "db.sqlite3"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	restore := api.SetMountinfoPath(mountTable(t, "/", root, root+"/disks/ssd"))
	defer restore()
	if _, err := svc.Backup(context.Background(), "vw"); err != nil {
		t.Fatalf("backup with the disk mounted: %v", err)
	}

	// After a reboot the disk is missing and Docker made the folder again.
	api.SetMountinfoPath(mountTable(t, "/", root))
	_, err := svc.Backup(context.Background(), "vw")
	if err == nil || !strings.Contains(err.Error(), "is that disk mounted") {
		t.Fatalf("backup without the disk = %v, want a refusal", err)
	}
	if len(eng.backedUp) != 1 {
		t.Fatalf("restic ran %d times, want only the backup with the disk mounted", len(eng.backedUp))
	}
}

// Unassigned Devices puts a small tmpfs at /mnt/disks, and a parent folder can
// be a mount of its own, so a path that is still on some mount after the
// reboot is not necessarily on its disk.
func TestBackupRefusesAFolderOffTheMountItWasOn(t *testing.T) {
	eng := &fakeResticEngine{}
	svc, root, appdata := appdataService(t, eng)
	if err := os.WriteFile(filepath.Join(appdata, "db.sqlite3"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	restore := api.SetMountinfoPath(mountTable(t, "/", root, root+"/disks", root+"/disks/ssd"))
	defer restore()
	if _, err := svc.Backup(context.Background(), "vw"); err != nil {
		t.Fatalf("backup with the disk mounted: %v", err)
	}

	api.SetMountinfoPath(mountTable(t, "/", root, root+"/disks"))
	if _, err := svc.Backup(context.Background(), "vw"); err == nil || !strings.Contains(err.Error(), "is that disk mounted") {
		t.Fatalf("backup with only the folder above the disk mounted = %v, want a refusal", err)
	}

	placeholder := filepath.Join(t.TempDir(), "mountinfo")
	table := "1 1 0:1 / / rw - ext4 /dev/x rw\n2 1 0:2 / " + root + " rw - shfs shfs rw\n3 2 0:3 / " + root + "/disks rw - tmpfs tmpfs rw,size=1024k\n"
	if err := os.WriteFile(placeholder, []byte(table), 0o600); err != nil {
		t.Fatal(err)
	}
	api.SetMountinfoPath(placeholder)
	if _, err := svc.Backup(context.Background(), "vw"); err == nil || !strings.Contains(err.Error(), "is that disk mounted") {
		t.Fatalf("backup over the placeholder tmpfs = %v, want a refusal", err)
	}
	if len(eng.backedUp) != 1 {
		t.Fatalf("restic ran %d times, want only the backup with the disk mounted", len(eng.backedUp))
	}
}

func TestBackupRefusesAnEmptiedFolderWithDataInEarlierBackups(t *testing.T) {
	eng := &fakeResticEngine{snaps: []restic.Snapshot{{
		ID: "aaaa1111", Tags: []string{"container:vw"}, Summary: &restic.SnapshotSummary{TotalBytesProcessed: 4096},
	}}}
	svc, _, _ := appdataService(t, eng)

	_, err := svc.Backup(context.Background(), "vw")
	if err == nil || !strings.Contains(err.Error(), "hold no files") {
		t.Fatalf("backup of an emptied folder = %v, want a refusal", err)
	}
	if len(eng.backedUp) != 0 {
		t.Fatalf("restic ran %d times for an emptied folder", len(eng.backedUp))
	}
}

func TestBackupKeepsBackingUpAFolderThatWasAlwaysEmpty(t *testing.T) {
	eng := &fakeResticEngine{snaps: []restic.Snapshot{{
		ID: "aaaa1111", Tags: []string{"container:vw"}, Summary: &restic.SnapshotSummary{},
	}}}
	svc, _, _ := appdataService(t, eng)

	if _, err := svc.Backup(context.Background(), "vw"); err != nil {
		t.Fatalf("backup of a folder that never held data: %v", err)
	}
}
