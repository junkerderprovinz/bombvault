package restic

import (
	"errors"
	"io/fs"
	"os"
	"strconv"
	"strings"
)

// ownerMayLive reports whether the process that took a lock under pid may
// still run. A PID nothing runs under, this process's own, or one a process
// started after this one holds cannot belong to a lock from before this
// process started.
func ownerMayLive(pid int) bool {
	if pid <= 0 {
		return true
	}
	// A lock under this process's own PID was left by whoever had the PID
	// before: this process only runs restic as children.
	if pid == os.Getpid() {
		return false
	}
	mine, err := procStartTicks(os.Getpid())
	if err != nil {
		return true
	}
	theirs, err := procStartTicks(pid)
	if errors.Is(err, fs.ErrNotExist) {
		return false
	}
	if err != nil {
		return true
	}
	return theirs <= mine
}

// procStartTicks reads when a process started, in clock ticks since boot
// (field 22 of /proc/<pid>/stat).
func procStartTicks(pid int) (uint64, error) {
	raw, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/stat")
	if err != nil {
		return 0, err
	}
	// The command name can hold spaces and parentheses; the fields after it
	// start behind its last ')'.
	s := string(raw)
	fields := strings.Fields(s[strings.LastIndexByte(s, ')')+1:])
	if len(fields) < 20 {
		return 0, errors.New("short /proc stat")
	}
	return strconv.ParseUint(fields[19], 10, 64)
}
