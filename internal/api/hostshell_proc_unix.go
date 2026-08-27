//go:build !windows

package api

import (
	"errors"
	"os"
	"os/exec"
	"syscall"
)

// configureHookProcGroup makes a hook command killable as a whole process
// GROUP, the same treatment internal/restic/proc_unix.go gives restic and for
// the same reason.
//
// A hook is a shell command the operator wrote, so it is routinely a pipeline,
// an `&&` chain or something with a background child. exec.CommandContext's
// default cancel signals only the direct `sh`, which leaves those children
// orphaned inside BombVault's container when the command overruns
// hostShellTimeout — and hostShellWaitDelay then merely stops US waiting for
// them, it does not reap them. Setpgid plus a group-directed signal does.
//
// SIGTERM first, not SIGKILL: a hook is somebody's script and may well have its
// own cleanup. WaitDelay stays as the escalation to a forced kill, so the pass
// still cannot be wedged.
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
