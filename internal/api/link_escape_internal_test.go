package api

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/restic"
)

func TestARestoreStopsAtALinkPlantedAfterItsPlan(t *testing.T) {
	dir := filepath.ToSlash(t.TempDir())
	outside := t.TempDir()
	if err := os.MkdirAll(dir+"/appdata", 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, dir+"/appdata/parent"); err != nil {
		t.Skipf("no symlinks here: %v", err)
	}
	eng := &stackEngine{}
	s := &Service{engine: eng}
	s.cfg.HostMountRoot = dir
	adapter := &resticAdapter{engine: eng, refuseWrite: s.refuseLinkOutOfMount}
	ctx := context.Background()

	if err := adapter.RestorePaths(ctx, "repo", "abcd", []string{dir + "/appdata/parent/config"}); !errors.Is(err, errLinkOutOfMount) {
		t.Errorf("RestorePaths = %v, want the link refused", err)
	}
	if err := adapter.RestoreSubtreeTo(ctx, "repo", "abcd", "/x", dir+"/appdata/parent/config"); !errors.Is(err, errLinkOutOfMount) {
		t.Errorf("RestoreSubtreeTo = %v, want the link refused", err)
	}
	if eng.restored != 0 {
		t.Errorf("restic restored %d times", eng.restored)
	}
}

// stackSvcWithRepo is stackSvc with its containers repository on disk.
func stackSvcWithRepo(t *testing.T, eng *stackEngine) *Service {
	t.Helper()
	s := stackSvc(t, eng)
	repo := filepath.Join(filepath.FromSlash(s.cfg.HostMountRoot), "backups", "containers")
	if err := os.MkdirAll(repo, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "config"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	return s
}

func TestStackRestoreRefusesARecordedPathOutsideTheMount(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("paths.Within only reads Linux paths")
	}
	eng := &stackEngine{snaps: []restic.Snapshot{{
		ID: strings.Repeat("a", 64), Time: "2026-11-05T00:00:00Z", Tags: []string{stackSnapshotTag("immich")}, Paths: []string{"/host/boot/config"},
	}}}

	ok, err := stackSvcWithRepo(t, eng).RestoreStackDir(context.Background(), "immich", "")

	if ok || err == nil || !strings.Contains(err.Error(), "outside the host mount") {
		t.Fatalf("RestoreStackDir = %v, %v, want a refusal", ok, err)
	}
	if eng.restored != 0 {
		t.Errorf("restic restored %d times", eng.restored)
	}
}

func TestStackRestoreStillRestoresAPathInsideTheMount(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("paths.Within only reads Linux paths")
	}
	eng := &stackEngine{}
	s := stackSvcWithRepo(t, eng)
	eng.snaps = []restic.Snapshot{{
		ID: strings.Repeat("a", 64), Time: "2026-11-05T00:00:00Z", Tags: []string{stackSnapshotTag("immich")}, Paths: []string{s.cfg.HostMountRoot + "/appdata/immich"},
	}}

	ok, err := s.RestoreStackDir(context.Background(), "immich", "")

	if !ok || err != nil || eng.restored != 1 {
		t.Fatalf("RestoreStackDir = %v, %v, restored %d", ok, err, eng.restored)
	}
}
