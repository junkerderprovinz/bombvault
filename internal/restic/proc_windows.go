//go:build windows

package restic

import (
	"os/exec"
	"time"
)

// resticWaitDelay bounds how long cmd.Wait blocks after ctx cancel before Go
// force-kills the process and returns.
const resticWaitDelay = 10 * time.Second

// configureProcGroup on Windows only bounds the post-cancel Wait; the process-group
// SIGKILL machinery is POSIX-specific (see proc_unix.go). exec.CommandContext's
// default cancel (os.Kill on the child) stays in effect.
func configureProcGroup(cmd *exec.Cmd) {
	cmd.WaitDelay = resticWaitDelay
}
