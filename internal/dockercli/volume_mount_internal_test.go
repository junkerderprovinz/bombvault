package dockercli

import (
	"testing"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
)

// TestNeedsVolumeMountpoint pins the predicate fillVolumeMountSources uses to
// decide which Mounts entries still need a VolumeInspect round-trip: only a
// volume-type mount the daemon reported with an empty Source (the rare case —
// it normally already carries the volume's real host storage location) AND a
// resolvable Name. A bind mount, a volume that already has its Source filled
// in, and a nameless mount must all be left alone (no extra API call, and
// nothing to look up by for the nameless case).
func TestNeedsVolumeMountpoint(t *testing.T) {
	cases := []struct {
		name string
		m    container.MountPoint
		want bool
	}{
		{
			name: "volume with no source and a name needs resolving",
			m:    container.MountPoint{Type: mount.TypeVolume, Name: "myapp_data", Source: ""},
			want: true,
		},
		{
			name: "volume already carrying a source is left alone",
			m:    container.MountPoint{Type: mount.TypeVolume, Name: "myapp_data", Source: "/var/lib/docker/volumes/myapp_data/_data"},
			want: false,
		},
		{
			name: "volume with no name has nothing to look up by",
			m:    container.MountPoint{Type: mount.TypeVolume, Name: "", Source: ""},
			want: false,
		},
		{
			name: "bind mount is never a volume-mountpoint candidate",
			m:    container.MountPoint{Type: mount.TypeBind, Name: "", Source: "/mnt/user/appdata/myapp"},
			want: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := needsVolumeMountpoint(tc.m); got != tc.want {
				t.Fatalf("needsVolumeMountpoint(%+v) = %v, want %v", tc.m, got, tc.want)
			}
		})
	}
}
