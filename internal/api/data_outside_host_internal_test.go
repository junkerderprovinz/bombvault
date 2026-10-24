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
	got := s.dataOutsideHostMount(in, nil)
	want := []string{"/mnt/.ix-apps/app_mounts/nextcloud/data", "/mnt/tank/apps/missing"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("dataOutsideHostMount = %v, want %v", got, want)
	}
}

// A named volume under Docker's own data root lies outside a Host Data at
// /mnt, so its backup says the data was not backed up instead of passing for
// a definition-only one. A volume Host Data reaches is backed up as usual.
func TestDataOutsideHostMountNamesAVolumeItCannotReach(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "user", "volumes", "cache", "_data"), 0o750); err != nil {
		t.Fatal(err)
	}
	s := &Service{cfg: config.Config{
		HostSourceRoot:   "/mnt",
		HostMountRoot:    filepath.ToSlash(root),
		DataRootSegments: []string{"appdata"},
	}}
	in := model.Inspect{Mounts: []model.Mount{
		{Type: "volume", Source: "/var/lib/docker/volumes/stack_db_data/_data", Destination: "/var/lib/mysql"},
		{Type: "volume", Source: "/mnt/user/volumes/cache/_data", Destination: "/cache"},
		{Type: "volume", Destination: "/unresolved"},
	}}
	got := s.dataOutsideHostMount(in, nil)
	want := []string{"/var/lib/docker/volumes/stack_db_data/_data"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("dataOutsideHostMount = %v, want %v", got, want)
	}
}

const (
	anonVolume  = "/var/lib/docker/volumes/3f9c2a7e1b4d6058a9e7c3b1f2d4e6a8b0c2d4e6f8a0b2c4d6e8f0a2b4c6d8e0/_data"
	namedVolume = "/var/lib/docker/volumes/jellyfin_cache/_data"
	ixVolume    = "/mnt/.ix-apps/app_mounts/nextcloud/data"
)

// An exclude line written as a path inside the container turns the warning off
// for the mount it names or a folder above it, and for nothing else.
func TestDataOutsideHostMountLeavesOutAMountAnExcludeCovers(t *testing.T) {
	s := &Service{cfg: config.Config{
		HostSourceRoot:   "/mnt",
		HostMountRoot:    filepath.ToSlash(t.TempDir()),
		DataRootSegments: []string{"appdata"},
	}}
	in := model.Inspect{Mounts: []model.Mount{
		{Type: "volume", Source: anonVolume, Destination: "/var/lib/beszel-agent"},
		{Type: "volume", Source: namedVolume, Destination: "/cache"},
		{Type: "bind", Source: ixVolume, Destination: "/data"},
	}}
	all := []string{ixVolume, anonVolume, namedVolume}
	cases := []struct {
		name     string
		excludes []string
		want     []string
	}{
		{"no exclude", nil, all},
		{"the mount's own path", []string{"/var/lib/beszel-agent"}, []string{ixVolume, namedVolume}},
		{"a folder above the mount", []string{"/var/lib"}, []string{ixVolume, namedVolume}},
		{"a trailing slash and spaces", []string{"  /cache/ "}, []string{ixVolume, anonVolume}},
		{"a bind mount outside Host Data", []string{"/data"}, []string{anonVolume, namedVolume}},
		{"every mount", []string{"/var/lib/beszel-agent", "/cache", "/data"}, nil},
		{"a folder inside the mount", []string{"/cache/transcodes"}, all},
		{"a sibling that shares a prefix", []string{"/var/lib/beszel"}, all},
		{"an unrelated path", []string{"/config"}, all},
		{"the root", []string{"/"}, all},
		{"bare names", []string{"cache", "data", "beszel-agent"}, all},
		{"a relative path", []string{"var/lib/beszel-agent"}, all},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := s.dataOutsideHostMount(in, tc.excludes)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("dataOutsideHostMount(%q) = %v, want %v", tc.excludes, got, tc.want)
			}
		})
	}
}
