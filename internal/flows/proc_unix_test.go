//go:build flows && unix

package flows

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

// ownGroup starts the server in a process group of its own, so endGroup
// reaches what it started there.
func ownGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// endGroup kills what is left of the server's group, so nothing writes into
// the install's folders while the test removes them.
func endGroup(p *os.Process) {
	_ = syscall.Kill(-p.Pid, syscall.SIGKILL)
}

// endStragglers kills the processes still working in root outside the
// server's group, such as a restic in a group of its own, and returns their
// command lines. The server has to end those itself before it exits. It reads
// /proc, so where there is none it finds nothing.
func endStragglers(root string, server *os.Process) []string {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil
	}
	var found []string
	for _, e := range entries {
		pid, err := strconv.Atoi(e.Name())
		if err != nil {
			continue
		}
		cwd, err := os.Readlink(filepath.Join("/proc", e.Name(), "cwd"))
		if err != nil || (cwd != root && !strings.HasPrefix(cwd, root+string(filepath.Separator))) {
			continue
		}
		pgid, err := syscall.Getpgid(pid)
		if err != nil || pgid == server.Pid {
			continue
		}
		cmdline, _ := os.ReadFile(filepath.Join("/proc", e.Name(), "cmdline"))
		found = append(found, strings.TrimSpace(strings.ReplaceAll(string(cmdline), "\x00", " ")))
		_ = syscall.Kill(-pgid, syscall.SIGKILL)
	}
	return found
}
