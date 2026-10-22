package api

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// realFolderGuard puts the real mount rule back for one test, with a mount
// table that has a share mounted at <root>/user and nothing else below root.
func realFolderGuard(t *testing.T, root string) {
	t.Helper()
	prev := folderOnStorage
	folderOnStorage = (*Service).restoreTargetMounted
	t.Cleanup(func() { folderOnStorage = prev })
	file := filepath.Join(t.TempDir(), "mountinfo")
	lines := []string{
		"30 1 0:28 / / rw,relatime - overlay overlay rw",
		"40 30 0:29 /mnt " + mountinfoEscape(root) + " rw,relatime master:5 - rootfs rootfs rw",
		"41 40 0:31 / " + mountinfoEscape(root+"/user") + " rw,relatime master:6 - fuse.shfs shfs rw",
	}
	if err := os.WriteFile(file, []byte(strings.Join(lines, "\n")+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	restore := SetMountinfoPath(file)
	t.Cleanup(restore)
}

// A restore folder on a pool this host does not have, such as one carried
// over in restored settings, lies on the RAM root of Unraid. Every restore
// into a folder and the drill sandbox refuse it before anything is created.
func TestRestoreFoldersOffStorageAreRefused(t *testing.T) {
	root := filepath.ToSlash(t.TempDir())
	realFolderGuard(t, root)
	st := newAliasLinkStore(t)
	set, err := st.CreateFileSet(store.FileSet{Name: "docs", Path: "user/docs"})
	if err != nil {
		t.Fatal(err)
	}
	s := &Service{store: st, cfg: config.Config{HostMountRoot: root}}
	ctx := context.Background()
	const gone = "nvme/restores"

	if _, err := s.prepareRestoreToPath(ctx, "plex", "local", "aaaa1111", gone); !isDestinationRefusal(err) {
		t.Errorf("container to folder: %v; want a destination refusal", err)
	}
	if _, err := s.prepareRestoreFileSet(ctx, set.ID, "aaaa1111", "local", gone, true); !isDestinationRefusal(err) {
		t.Errorf("folder set to folder: %v; want a destination refusal", err)
	}
	if _, _, err := s.newDrillSandbox(store.Settings{RestoreFolder: gone}, "drill"); !isDestinationRefusal(err) {
		t.Errorf("drill sandbox: %v; want a destination refusal", err)
	}
	if _, err := os.Stat(filepath.Join(root, "nvme")); !os.IsNotExist(err) {
		t.Fatalf("a refused restore created %s: %v", gone, err)
	}

	sandbox, cleanup, err := s.newDrillSandbox(store.Settings{RestoreFolder: "user/restore"}, "drill")
	if err != nil {
		t.Fatalf("a sandbox on the mounted share: %v", err)
	}
	cleanup()
	if !strings.HasPrefix(filepath.ToSlash(sandbox), root+"/user/restore/") {
		t.Fatalf("sandbox = %s", sandbox)
	}
}
