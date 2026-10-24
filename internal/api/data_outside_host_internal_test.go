package api

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
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
	anonName    = "3f9c2a7e1b4d6058a9e7c3b1f2d4e6a8b0c2d4e6f8a0b2c4d6e8f0a2b4c6d8e0"
	anonVolume  = "/var/lib/docker/volumes/" + anonName + "/_data"
	namedVolume = "/var/lib/docker/volumes/jellyfin_cache/_data"
	ixVolume    = "/mnt/.ix-apps/app_mounts/nextcloud/data"
)

func TestDataOutsideHostMountKeepsQuietAboutAnAnonymousVolume(t *testing.T) {
	s := &Service{cfg: config.Config{
		HostSourceRoot:   "/mnt",
		HostMountRoot:    filepath.ToSlash(t.TempDir()),
		DataRootSegments: []string{"appdata"},
	}}
	cases := []struct {
		name  string
		mount model.Mount
		warns bool
	}{
		{"an anonymous volume", model.Mount{Type: "volume", Name: anonName, Source: anonVolume, Destination: "/cache"}, false},
		{"a named volume", model.Mount{Type: "volume", Name: "jellyfin_cache", Source: namedVolume, Destination: "/cache"}, true},
		{"a volume named with upper-case hex", model.Mount{Type: "volume", Name: strings.ToUpper(anonName), Source: anonVolume, Destination: "/cache"}, true},
		{"a volume named with 63 hex digits", model.Mount{Type: "volume", Name: anonName[1:], Source: anonVolume, Destination: "/cache"}, true},
		{"a bind outside Host Data", model.Mount{Type: "bind", Source: ixVolume, Destination: "/data"}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := s.dataOutsideHostMount(model.Inspect{Mounts: []model.Mount{tc.mount}}, nil)
			if warns := len(got) > 0; warns != tc.warns {
				t.Fatalf("dataOutsideHostMount = %v, want a warning: %v", got, tc.warns)
			}
		})
	}
}

// An exclude line written as the mount's path inside the container or on the
// host turns the warning off for the mount it names or a folder above it, and
// for nothing else.
func TestDataOutsideHostMountLeavesOutAMountAnExcludeCovers(t *testing.T) {
	s := &Service{cfg: config.Config{
		HostSourceRoot:   "/mnt",
		HostMountRoot:    filepath.ToSlash(t.TempDir()),
		DataRootSegments: []string{"appdata"},
	}}
	in := model.Inspect{Mounts: []model.Mount{
		{Type: "volume", Name: anonName, Source: anonVolume, Destination: "/var/lib/beszel-agent"},
		{Type: "volume", Name: "jellyfin_cache", Source: namedVolume, Destination: "/cache"},
		{Type: "bind", Source: ixVolume, Destination: "/data"},
	}}
	all := []string{ixVolume, namedVolume}
	cases := []struct {
		name     string
		excludes []string
		want     []string
	}{
		{"no exclude", nil, all},
		{"the mount's own path", []string{"/cache"}, []string{ixVolume}},
		{"a trailing slash and spaces", []string{"  /cache/ "}, []string{ixVolume}},
		{"a bind mount outside Host Data", []string{"/data"}, []string{namedVolume}},
		{"every mount", []string{"/cache", "/data"}, nil},
		{"the volume's host path", []string{namedVolume}, []string{ixVolume}},
		{"a folder above the volume's host path", []string{"/var/lib/docker/volumes"}, []string{ixVolume}},
		{"a host path with a trailing slash and spaces", []string{" " + namedVolume + "/ "}, []string{ixVolume}},
		{"the bind's host path", []string{ixVolume}, []string{namedVolume}},
		{"a folder above the bind's host path", []string{"/mnt/.ix-apps"}, []string{namedVolume}},
		{"every host path", []string{namedVolume, ixVolume}, nil},
		{"Host Data itself", []string{"/mnt"}, all},
		{"a folder inside the host path", []string{namedVolume + "/transcodes"}, all},
		{"a host path sibling that shares a prefix", []string{"/var/lib/docker/volumes/jellyfin"}, all},
		{"a folder inside the mount", []string{"/cache/transcodes"}, all},
		{"a sibling that shares a prefix", []string{"/cach"}, all},
		{"an unrelated path", []string{"/config"}, all},
		{"the root", []string{"/"}, all},
		{"bare names", []string{"cache", "data", "_data"}, all},
		{"a relative path", []string{"var/lib/docker/volumes"}, all},
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

// A host path stands for nothing restic could match once the mount behind it
// is out of reach, so the line never becomes an --exclude.
func TestResolveExcludePatternsLeavesAHostPathOutsideHostDataOut(t *testing.T) {
	s := &Service{cfg: config.Config{HostSourceRoot: "/mnt", HostMountRoot: filepath.ToSlash(t.TempDir())}}
	in := model.Inspect{Mounts: []model.Mount{
		{Type: "volume", Name: anonName, Source: anonVolume, Destination: "/cache"},
		{Type: "volume", Name: "jellyfin_cache", Source: namedVolume, Destination: "/transcode"},
	}}
	cases := []struct {
		line string
		want []string
	}{
		{anonVolume, nil},
		{namedVolume, nil},
		{"/var/lib/docker/volumes", nil},
		{"/var/lib/docker/other", []string{"/var/lib/docker/other"}},
		{"/", nil},
		{"/mnt", []string{"/mnt"}},
	}
	for _, tc := range cases {
		t.Run(tc.line, func(t *testing.T) {
			if got := s.resolveExcludePatterns([]string{tc.line}, in); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("resolveExcludePatterns(%q) = %q, want %q", tc.line, got, tc.want)
			}
		})
	}
}
