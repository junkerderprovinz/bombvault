//go:build linux

package api

import (
	"fmt"
	"os/exec"
	"syscall"
)

// enterView runs true inside root, which is what restic does there later:
// chroot into another container's root needs a capability and a security
// profile that allow it.
func enterView(root string) error {
	cmd := exec.Command("/bin/true")
	cmd.SysProcAttr = &syscall.SysProcAttr{Chroot: root}
	cmd.Dir = "/"
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("entering the snapshot view: %w: %s", err, out)
	}
	return nil
}
