//go:build windows

package restic

import (
	"os/exec"
	"time"
)

// resticWaitDelay bounds how long cmd.Wait blocks after ctx cancel before Go
// force-kills the process and returns.
const resticWaitDelay = 10 * time.Second

// configureProcGroup only bounds the post-cancel Wait on Windows. Process groups
// are POSIX-only (see proc_unix.go), so exec.CommandContext's default cancel,
// which kills the child, stays in effect.
func configureProcGroup(cmd *exec.Cmd) {
	cmd.WaitDelay = resticWaitDelay
}
