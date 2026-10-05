package api

import (
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/platform"
)

func TestRestoreTargetMountedTrustsTheHostDataBindOnAPlainDockerHost(t *testing.T) {
	writeMountFixture(t, "/", "/host/user")
	s := &Service{cfg: config.Config{HostMountRoot: "/host/user"}}
	s.SetPlatform(platform.Generic{})

	if !s.restoreTargetMounted("/host/user/bombvault/restore/web") {
		t.Fatal("a folder on the Host Data bind of a plain Docker host must count as storage")
	}
	if s.restoreTargetMounted("/host/user") {
		t.Fatal("the mount root itself is not a restore folder")
	}
}

func TestRestoreTargetMountedRefusesAnUnmountedHostDataRoot(t *testing.T) {
	writeMountFixture(t, "/")
	s := &Service{cfg: config.Config{HostMountRoot: "/host/user"}}
	s.SetPlatform(platform.Generic{})

	if s.restoreTargetMounted("/host/user/bombvault/restore/web") {
		t.Fatal("without the Host Data bind the folder is in the container's own filesystem")
	}
}

func TestRestoreTargetMountedKeepsUnraidToMountedShares(t *testing.T) {
	writeMountFixture(t, "/", "/host/user", "/host/user/cache")
	s := &Service{cfg: config.Config{HostMountRoot: "/host/user"}}
	s.SetPlatform(platform.Unraid{})

	if s.restoreTargetMounted("/host/user/zfs/appdata/web") {
		t.Fatal("on Unraid a folder outside a mounted share lives in RAM")
	}
	if !s.restoreTargetMounted("/host/user/cache/appdata/web") {
		t.Fatal("a folder on a mounted pool must count")
	}
}
