//go:build flows && unix

package flows

import (
	"os"
	"os/exec"
	"syscall"
)

// ownGroup starts the server in a process group of its own, so the restic it
// runs can be stopped with it.
func ownGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// endGroup kills what the server left running, such as a prune it started, so
// nothing writes into the install's folders while the test removes them.
func endGroup(p *os.Process) {
	_ = syscall.Kill(-p.Pid, syscall.SIGKILL)
}
