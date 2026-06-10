package api

import (
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/config"
)

// svcWithMounts builds a Service with only the mount-translation config set —
// enough to exercise toContainerPath / withinAnyMount in isolation.
func svcWithMounts() *Service {
	return &Service{cfg: config.Config{
		HostSourceRoot:  "/mnt",
		HostMountRoot:   "/host/user",
		NVRAMSourceRoot: "/etc/libvirt/qemu/nvram",
		NVRAMMountRoot:  "/host/nvram",
	}}
}

func TestToContainerPathMultiMount(t *testing.T) {
	s := svcWithMounts()
	cases := []struct {
		name string
		host string
		want string
		ok   bool
	}{
		{"appdata under /mnt", "/mnt/user/appdata/foo", "/host/user/user/appdata/foo", true},
		{"vm disk under /mnt", "/mnt/cache/domains/win11/vdisk1.img", "/host/user/cache/domains/win11/vdisk1.img", true},
		{"source root exactly", "/mnt", "/host/user", true},
		{"uefi nvram", "/etc/libvirt/qemu/nvram/abc_VARS-pure-efi.fd", "/host/nvram/abc_VARS-pure-efi.fd", true},
		{"nvram root exactly", "/etc/libvirt/qemu/nvram", "/host/nvram", true},
		{"unreachable etc path", "/etc/passwd", "", false},
		{"unreachable libvirt sibling", "/etc/libvirt/qemu.conf", "", false},
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

func TestWithinAnyMount(t *testing.T) {
	s := svcWithMounts()
	cases := []struct {
		p    string
		want bool
	}{
		{"/host/user/user/appdata/foo", true},
		{"/host/nvram/abc_VARS.fd", true},
		{"/etc/libvirt/qemu/nvram/abc_VARS.fd", false}, // host path, not container path
		{"/host/other/thing", false},
		{"/tmp/escape", false},
	}
	for _, tc := range cases {
		if got := s.withinAnyMount(tc.p); got != tc.want {
			t.Errorf("withinAnyMount(%q) = %v, want %v", tc.p, got, tc.want)
		}
	}
}

// NVRAM translation must degrade gracefully when the NVRAM mount is not
// configured (empty roots): the path is simply unreachable, not a crash.
func TestToContainerPathNVRAMMountDisabled(t *testing.T) {
	s := &Service{cfg: config.Config{HostSourceRoot: "/mnt", HostMountRoot: "/host/user"}}
	if got, ok := s.toContainerPath("/etc/libvirt/qemu/nvram/x_VARS.fd"); ok {
		t.Fatalf("expected nvram unreachable without mount, got %q", got)
	}
}
