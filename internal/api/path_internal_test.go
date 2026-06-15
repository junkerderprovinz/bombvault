package api

import (
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/config"
)

// svcWithMount builds a Service with only the Host Data mount config set —
// enough to exercise toContainerPath in isolation. NVRAM is no longer a mount
// (it travels over SSH), so there is a single host→container mapping.
func svcWithMount() *Service {
	return &Service{cfg: config.Config{
		HostSourceRoot: "/mnt",
		HostMountRoot:  "/host/user",
	}}
}

func TestToContainerPath(t *testing.T) {
	s := svcWithMount()
	cases := []struct {
		name string
		host string
		want string
		ok   bool
	}{
		{"appdata under /mnt", "/mnt/user/appdata/foo", "/host/user/user/appdata/foo", true},
		{"vm disk under /mnt", "/mnt/cache/vms/win11/vdisk1.img", "/host/user/cache/vms/win11/vdisk1.img", true},
		{"source root exactly", "/mnt", "/host/user", true},
		{"nvram under /etc/libvirt is NOT reachable via the mount", "/etc/libvirt/qemu/nvram/x_VARS.fd", "", false},
		{"unreachable etc path", "/etc/passwd", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := s.toContainerPath(tc.host)
			if ok != tc.ok || got != tc.want {
				t.Fatalf("toContainerPath(%q) = (%q,%v), want (%q,%v)", tc.host, got, ok, tc.want, tc.ok)
			}
		})
	}
}
