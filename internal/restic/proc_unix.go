//go:build !windows

package restic

import (
	"errors"
	"os"
	"os/exec"
	"syscall"
	"time"
)

// resticWaitDelay bounds how long cmd.Wait blocks after ctx cancel before Go
// force-kills the process and returns, so a wedged restic can't hang the caller.
const resticWaitDelay = 10 * time.Second

// configureProcGroup makes a restic exec.Cmd killable as a whole process group.
// restic spawns an rclone child for cloud backends; without a process group, ctx
// cancel/timeout kills only the direct restic child and the rclone grandchild
// (and restic's lock refresh) can linger. Setpgid puts restic in its own group;
// Cancel SIGKILLs the whole group (-pid). WaitDelay bounds the post-cancel Wait.
//
// Honest limit: a process stuck in uninterruptible I/O on a truly dead mount
// (NFS/SMB) cannot be reaped even by SIGKILL — that needs separate mount-health
// detection. This handles the common hangs and the rclone grandchild.
func configureProcGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return os.ErrProcessDone
		}
		if err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL); err != nil {
			if errors.Is(err, syscall.ESRCH) {
				return os.ErrProcessDone
			}
			return err
		}
		return os.ErrProcessDone
	}
	cmd.WaitDelay = resticWaitDelay
}
