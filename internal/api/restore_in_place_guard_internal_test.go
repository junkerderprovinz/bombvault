package api

import (
	"context"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/platform"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// poolDownFixture has Host Data bound on an Unraid host, with the pool "nvme"
// not mounted, and a folder set and a container whose data lies on it.
func poolDownFixture(t *testing.T) (f *placementFixture, setID, dir string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("paths.Within only reads Linux paths")
	}
	f = newPlacementFixture(t)
	writeMountFixture(t, "/", f.root)
	f.svc.SetPlatform(platform.Unraid{})
	t.Cleanup(func() { f.svc.SetPlatform(nil) })
	dir = f.root + "/nvme/media"
	set, err := f.st.CreateFileSet(store.FileSet{Name: "media", Path: "nvme/media", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	files, _ := f.svc.resolveRepo("backups/files")
	containers, _ := f.svc.resolveRepo("backups/containers")
	f.makeRepo(files)
	f.makeRepo(containers)
	f.eng.snaps[filepath.ToSlash(files)] = []restic.Snapshot{{ID: "aaaaaaaa11111111", Time: "2026-11-01T03:00:00Z", Tags: []string{"fileset:media"}, Paths: []string{dir}}}
	f.eng.snaps[filepath.ToSlash(containers)] = []restic.Snapshot{{ID: "bbbbbbbb22222222", Time: "2026-11-01T03:00:00Z", Tags: []string{"container:plex"}, Paths: []string{dir}}}
	if _, err := f.st.UpsertTarget(store.Target{ContainerName: "plex"}); err != nil {
		t.Fatal(err)
	}
	return f, set.ID, dir
}

func TestEveryRestoreInPlaceRefusesAPoolThatIsDown(t *testing.T) {
	f, setID, dir := poolDownFixture(t)
	ctx := context.Background()

	if _, err := f.svc.prepareRestoreFileSet(ctx, setID, "aaaaaaaa11111111", "local", "", true); !isDestinationRefusal(err) {
		t.Errorf("folder set whole: %v; want a destination refusal", err)
	}
	if _, err := f.svc.prepareRestoreFileSetFiles(ctx, setID, "local", "aaaaaaaa11111111", []string{dir + "/m1.txt"}, "", true); !isDestinationRefusal(err) {
		t.Errorf("folder set picks: %v; want a destination refusal", err)
	}
	if _, err := f.svc.prepareRestoreFiles(ctx, "plex", "local", "bbbbbbbb22222222", []string{dir + "/m1.txt"}, "", true); !isDestinationRefusal(err) {
		t.Errorf("container picks: %v; want a destination refusal", err)
	}
}

func TestARestoreInPlaceOnAMountedPoolStillGoesAhead(t *testing.T) {
	f, setID, dir := poolDownFixture(t)
	writeMountFixture(t, "/", f.root, f.root+"/nvme")
	ctx := context.Background()

	if _, err := f.svc.prepareRestoreFileSet(ctx, setID, "aaaaaaaa11111111", "local", "", true); err != nil {
		t.Errorf("folder set whole: %v", err)
	}
	if _, err := f.svc.prepareRestoreFileSetFiles(ctx, setID, "local", "aaaaaaaa11111111", []string{dir + "/m1.txt"}, "", true); err != nil {
		t.Errorf("folder set picks: %v", err)
	}
	if _, err := f.svc.prepareRestoreFiles(ctx, "plex", "local", "bbbbbbbb22222222", []string{dir + "/m1.txt"}, "", true); err != nil {
		t.Errorf("container picks: %v", err)
	}
}

func TestAStackFolderOnAPoolThatIsDownIsRefusedBeforeAWrite(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("paths.Within only reads Linux paths")
	}
	eng := &stackEngine{}
	s := stackSvcWithRepo(t, eng)
	writeMountFixture(t, "/", s.cfg.HostMountRoot)
	s.SetPlatform(platform.Unraid{})
	t.Cleanup(func() { s.SetPlatform(nil) })
	eng.snaps = []restic.Snapshot{{
		ID: "cccccccc33333333cccccccc33333333", Time: "2026-11-05T00:00:00Z", Tags: []string{stackSnapshotTag("immich")}, Paths: []string{s.cfg.HostMountRoot + "/nvme/stacks/immich"},
	}}

	ok, err := s.RestoreStackDir(context.Background(), "immich", "")

	if ok || !isDestinationRefusal(err) {
		t.Fatalf("RestoreStackDir = %v, %v, want a destination refusal", ok, err)
	}
	if eng.restored != 0 {
		t.Errorf("restic restored %d times", eng.restored)
	}
}
