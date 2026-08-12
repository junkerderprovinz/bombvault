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
// Cancel SIGTERMs the whole group (-pid) first. WaitDelay bounds how long Wait
// gives it to exit on its own before Go force-SIGKILLs the group — same escape
// hatch as before, just no longer the FIRST signal sent.
//
// SIGTERM, not SIGKILL, matters: restic treats SIGTERM/SIGINT as a clean-abort
// request — it stops starting new uploads and exits WITHOUT writing the final
// snapshot object, so an interrupted backup simply produces no snapshot rather
// than a broken one. SIGKILL skips that handler entirely. Root-caused live
// 2026-08-12 against a real damaged containers repo (dozens of snapshots with
// trees referencing blobs "not found in repository"/"not found in index"):
// BombVault's own container got restarted (a routine image update) while a
// catch-up backup batch was mid-upload; the in-flight restic process was
// SIGKILLed via this exact code path, and a later automatic prune correctly
// swept up the resulting orphaned pack data — but the killed snapshot's tree
// metadata had already been written and still referenced it. `restic repair
// snapshots --forget` recovered the repo (67 clean snapshots survived across
// 42 containers); this is the fix for the corruption happening again.
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
