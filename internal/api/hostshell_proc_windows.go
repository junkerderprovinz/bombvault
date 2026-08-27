//go:build windows

package api

import "os/exec"

// configureHookProcGroup on Windows only bounds the post-cancel Wait; the
// process-group signalling is POSIX-specific (see hostshell_proc_unix.go), and
// exec.CommandContext's default cancel stays in effect. Same split as
// internal/restic's proc_unix.go / proc_windows.go pair.
func configureHookProcGroup(cmd *exec.Cmd) {
	cmd.WaitDelay = hostShellWaitDelay
}
