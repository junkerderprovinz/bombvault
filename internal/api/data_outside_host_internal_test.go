package api

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/model"
)

func TestDataOutsideHostMount(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "apps", "plex"), 0o750); err != nil {
		t.Fatal(err)
	}
	s := &Service{cfg: config.Config{
		HostSourceRoot:   "/mnt/tank",
		HostMountRoot:    filepath.ToSlash(root),
		DataRootSegments: []string{"appdata"},
	}}
	in := model.Inspect{Mounts: []model.Mount{
		{Type: "bind", Source: "/mnt/tank/apps/plex", Destination: "/config"},
		{Type: "bind", Source: "/mnt/.ix-apps/app_mounts/nextcloud/data", Destination: "/data"},
		{Type: "bind", Source: "/mnt/tank/apps/missing", Destination: "/gone"},
		{Type: "bind", Source: "/etc/localtime", Destination: "/etc/localtime"},
		{Type: "bind", Source: "/var/run/docker.sock", Destination: "/var/run/docker.sock"},
	}}
	got := s.dataOutsideHostMount(in)
	want := []string{"/mnt/.ix-apps/app_mounts/nextcloud/data", "/mnt/tank/apps/missing"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("dataOutsideHostMount = %v, want %v", got, want)
	}
}
