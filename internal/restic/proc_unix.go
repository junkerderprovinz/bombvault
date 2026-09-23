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

// configureProcGroup puts restic in its own process group, so cancelling also
// stops the rclone child it starts for cloud backends. Cancel sends SIGTERM to
// the whole group; after WaitDelay Go falls back to SIGKILL, which reaches only
// restic itself.
//
// On SIGTERM restic stops uploading and exits without writing the snapshot, so
// an interrupted backup leaves no snapshot rather than a broken one. A SIGKILL
// at the wrong moment can leave a snapshot whose trees reference blobs that
// never reached the repository.
//
// A process stuck in uninterruptible I/O on a dead NFS or SMB mount survives
// even SIGKILL.
func configureProcGroup(cmd *exec.Cmd) {
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
	cmd.WaitDelay = resticWaitDelay
}
