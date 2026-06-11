package virshcli

import (
	"fmt"
	"os"
	"path/filepath"
)

// LinkSocket makes the host libvirt sockets reachable at the path virsh expects.
//
// On Unraid /var/run/libvirt is recreated by the VM Manager every time VMs are
// enabled/disabled (and at boot). A container that bind-mounts /var/run/libvirt
// directly PINS that directory, so the host can no longer recreate it and
// libvirtd fails to start ("Address already in use" / VM Manager dead). To avoid
// that, the container instead mounts the run PARENT at runRoot (e.g. /host/run);
// the host is then free to manage runRoot/libvirt (a child of the bind mount can
// be removed/recreated). This symlinks linkPath (normally /var/run/libvirt) to
// runRoot/libvirt so `virsh -c qemu:///system` resolves the socket through the
// parent mount, surviving every toggle.
//
// It is a no-op (nil) when runRoot is empty or runRoot/libvirt is absent (VM
// backup not configured) — virsh then finds no socket, which the host-integration
// probe reports rather than failing the process. An already-correct symlink is
// left untouched.
func LinkSocket(runRoot, linkPath string) error {
	if runRoot == "" {
		return nil
	}
	target := filepath.Join(runRoot, "libvirt")
	if _, err := os.Stat(target); err != nil {
		return nil // host run dir not mounted — nothing to link
	}
	if fi, err := os.Lstat(linkPath); err == nil {
		if fi.Mode()&os.ModeSymlink != 0 {
			if dst, _ := os.Readlink(linkPath); dst == target {
				return nil // already correct
			}
		}
		if err := os.RemoveAll(linkPath); err != nil {
			return fmt.Errorf("virshcli: clear %s: %w", linkPath, err)
		}
	}
	if err := os.MkdirAll(filepath.Dir(linkPath), 0o750); err != nil {
		return fmt.Errorf("virshcli: mkdir %s: %w", filepath.Dir(linkPath), err)
	}
	if err := os.Symlink(target, linkPath); err != nil {
		return fmt.Errorf("virshcli: symlink %s -> %s: %w", linkPath, target, err)
	}
	return nil
}
