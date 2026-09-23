//go:build !windows

package api

import (
	"errors"
	"os"
	"os/exec"
	"syscall"
)

// configureHookProcGroup starts the hook in its own process group and sends
// SIGTERM to the whole group on cancel. The default cancel signals only sh,
// which would leave the children of a pipeline or background job orphaned in
// the container. SIGTERM lets the script clean up; WaitDelay still bounds the
// wait.
func configureHookProcGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return os.ErrProcessDone
		}
		if err := syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM); err != nil {
			if errors.Is(err, syscall.ESRCH) {
				return os.ErrProcessDone
			}
			return err
		}
		return os.ErrProcessDone
	}
	cmd.WaitDelay = hostShellWaitDelay
}
