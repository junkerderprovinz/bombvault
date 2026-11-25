package api

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/dockercli"
	"github.com/junkerderprovinz/bombvault/internal/model"
	"github.com/junkerderprovinz/bombvault/internal/zfs"
)

type peekDocker struct {
	dockercli.Docker
	holds map[string]bool
	asked []string
}

func (d *peekDocker) HoldsFiles(_ context.Context, id, path string) (bool, error) {
	d.asked = append(d.asked, id+":"+path)
	return d.holds[id+":"+path], nil
}

func lateMountService(t *testing.T, d *peekDocker) (*Service, string) {
	t.Helper()
	root := t.TempDir()
	return &Service{docker: d, cfg: config.Config{HostMountRoot: root, HostSourceRoot: root}}, root
}

func appWith(root string) model.Inspect {
	return model.Inspect{ID: "cid", Mounts: []model.Mount{{Type: "bind", Source: filepath.Join(root, "appdata", "late"), Destination: "/data"}}}
}

func TestAFolderEmptyHereButFullForTheAppWasMountedAfterStart(t *testing.T) {
	d := &peekDocker{holds: map[string]bool{"cid:/data": true}}
	s, root := lateMountService(t, d)
	dir := filepath.Join(root, "appdata", "late")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	got := s.mountedAfterStart(context.Background(), appWith(root), []string{dir})
	if !slices.Equal(got, []string{dir}) {
		t.Fatalf("mountedAfterStart = %v, want %v", got, []string{dir})
	}
}

func TestAFolderTheAppSeesEmptyTooIsNotReported(t *testing.T) {
	d := &peekDocker{holds: map[string]bool{}}
	s, root := lateMountService(t, d)
	dir := filepath.Join(root, "appdata", "late")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	if got := s.mountedAfterStart(context.Background(), appWith(root), []string{dir}); len(got) != 0 {
		t.Fatalf("mountedAfterStart = %v, want none", got)
	}
}

func TestAFolderWithFilesHereIsNotCheckedWithDocker(t *testing.T) {
	d := &peekDocker{holds: map[string]bool{"cid:/data": true}}
	s, root := lateMountService(t, d)
	dir := filepath.Join(root, "appdata", "late")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := s.mountedAfterStart(context.Background(), appWith(root), []string{dir}); len(got) != 0 || len(d.asked) != 0 {
		t.Fatalf("mountedAfterStart = %v, asked %v, want none and no question", got, d.asked)
	}
}

func TestAFolderBelowABindIsLookedUpBelowItsTarget(t *testing.T) {
	d := &peekDocker{holds: map[string]bool{"cid:/data/db": true}}
	s, root := lateMountService(t, d)
	dir := filepath.Join(root, "appdata", "late", "db")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	got := s.mountedAfterStart(context.Background(), appWith(root), []string{dir})
	if !slices.Equal(got, []string{dir}) || !slices.Equal(d.asked, []string{"cid:/data/db"}) {
		t.Fatalf("mountedAfterStart = %v, asked %v, want %v and cid:/data/db", got, d.asked, []string{dir})
	}
}

func withMountRecords(t *testing.T, recs ...zfs.MountRecord) {
	t.Helper()
	previous := zfsMountRecords
	zfsMountRecords = func() []zfs.MountRecord { return recs }
	t.Cleanup(func() { zfsMountRecords = previous })
}

func TestAHostDataBindWithoutPropagationGetsAStartupNote(t *testing.T) {
	withMountRecords(t, zfs.MountRecord{MountPoint: "/mnt/tank", FSType: "zfs"})
	s := &Service{cfg: config.Config{HostMountRoot: "/mnt/tank"}}
	if note := s.LateMountNote(); !strings.Contains(note, "/mnt/tank") || !strings.Contains(note, "restart") {
		t.Fatalf("LateMountNote = %q, want a note naming /mnt/tank and the restart", note)
	}
}

func TestAHostDataBindWithPropagationGetsNoStartupNote(t *testing.T) {
	withMountRecords(t, zfs.MountRecord{MountPoint: "/mnt/tank", FSType: "zfs", Optional: []string{"master:42"}})
	s := &Service{cfg: config.Config{HostMountRoot: "/mnt/tank"}}
	if note := s.LateMountNote(); note != "" {
		t.Fatalf("LateMountNote = %q, want none", note)
	}
}

func TestNoHostDataMountGetsNoStartupNote(t *testing.T) {
	withMountRecords(t, zfs.MountRecord{MountPoint: "/", FSType: "overlay"})
	s := &Service{cfg: config.Config{HostMountRoot: "/mnt/tank"}}
	if note := s.LateMountNote(); note != "" {
		t.Fatalf("LateMountNote = %q, want none outside a container", note)
	}
}
