//go:build windows

package api

import "os/exec"

// configureHookProcGroup only bounds the wait after cancel. Process groups are
// POSIX, so Windows keeps the default cancel.
func configureHookProcGroup(cmd *exec.Cmd) {
	cmd.WaitDelay = hostShellWaitDelay
}
