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

// TestValidResourceName guards the container/VM {name} validator: the Go 1.22
// router decodes "%2f"/"%2e%2e", so an unvalidated name could carry "../" into
// the template/XML file sinks. Valid Docker/libvirt names pass; anything with a
// separator, "..", a leading "-"/".", NUL or that is empty is rejected.
func TestValidResourceName(t *testing.T) {
	valid := []string{"plex", "Windows-11", "ubuntu_test", "a.b-c_1", "X"}
	for _, n := range valid {
		if !validResourceName(n) {
			t.Errorf("expected %q to be valid", n)
		}
	}
	invalid := []string{
		"", "..", "../../etc", "a/b", `a\b`, "a..b", "-rf", ".hidden",
		"a/../b", "with space", "nul\x00byte", "..%2f", // %2f stays literal here; the decoded "/" form is "a/b" above
	}
	for _, n := range invalid {
		if validResourceName(n) {
			t.Errorf("expected %q to be REJECTED", n)
		}
	}
}
