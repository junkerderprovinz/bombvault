package api

import (
	"reflect"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/backup"
	"github.com/junkerderprovinz/bombvault/internal/config"
)

// plainDockerHost maps /srv/appdata on the host to /host/user, the way the
// generic compose file is set up.
func plainDockerHost() *Service {
	return &Service{cfg: config.Config{HostMountRoot: "/host/user", HostSourceRoot: "/srv/appdata"}}
}

func TestRemapSourceBindsFindsAnUnraidBindOnAPlainDockerHost(t *testing.T) {
	s := plainDockerHost()
	binds := []string{"/mnt/user/appdata/web:/config", "/var/run/docker.sock:/var/run/docker.sock"}
	dirs := []backup.RestoreDir{{Subtree: "/host/user/user/appdata/web", Target: "/host/user/restore/web"}}
	remap := map[string]string{"/srv/appdata/user/appdata/web": "/srv/appdata/restore/web"}

	got := rewriteBinds(binds, s.remapSourceBinds(binds, dirs, remap))
	want := []string{"/srv/appdata/restore/web:/config", "/var/run/docker.sock:/var/run/docker.sock"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("binds = %v, want %v", got, want)
	}
}

func TestRemapSourceBindsFollowsABindOnTheContainerFolder(t *testing.T) {
	s := plainDockerHost()
	binds := []string{"/mnt/cache/appdata/web:/data"}
	dirs := []backup.RestoreDir{{Subtree: "/host/user/cache/appdata/web/conf", Target: "/host/user/restore/web/conf"}}

	got := rewriteBinds(binds, s.remapSourceBinds(binds, dirs, map[string]string{"x": "y"}))
	if got[0] != "/srv/appdata/restore/web:/data" {
		t.Fatalf("bind = %q, want the container folder under the restore folder", got[0])
	}
}

func TestRemapSourceBindsLeavesAnAmbiguousBindAlone(t *testing.T) {
	s := plainDockerHost()
	binds := []string{"/mnt/user/appdata/web:/a", "/mnt/cache/appdata/web:/b"}
	dirs := []backup.RestoreDir{{Subtree: "/host/user/user/appdata/web", Target: "/host/user/restore/web"}}

	got := rewriteBinds(binds, s.remapSourceBinds(binds, dirs, map[string]string{"x": "y"}))
	if !reflect.DeepEqual(got, binds) {
		t.Fatalf("binds = %v, want them untouched", got)
	}
}

func TestRemapSourceBindsKeepsAnExactMatch(t *testing.T) {
	s := &Service{cfg: config.Config{HostMountRoot: "/host/user", HostSourceRoot: "/mnt"}}
	binds := []string{"/mnt/user/appdata/web:/config"}
	dirs := []backup.RestoreDir{{Subtree: "/host/user/user/appdata/web", Target: "/host/user/zfs/appdata/web"}}
	remap := map[string]string{"/mnt/user/appdata/web": "/mnt/zfs/appdata/web"}

	got := s.remapSourceBinds(binds, dirs, remap)
	if !reflect.DeepEqual(got, remap) {
		t.Fatalf("remap = %v, want it unchanged", got)
	}
}
