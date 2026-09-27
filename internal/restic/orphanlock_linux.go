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
// process started, unless that process is a child of this one.
func ownerMayLive(pid int) bool {
	if pid <= 0 {
		return true
	}
	// A lock under this process's own PID was left by whoever had the PID
	// before: this process only runs restic as children.
	if pid == os.Getpid() {
		return false
	}
	mine, err := procStat(os.Getpid())
	if err != nil {
		return true
	}
	theirs, err := procStat(pid)
	if errors.Is(err, fs.ErrNotExist) {
		return false
	}
	if err != nil {
		return true
	}
	// A child is a restic this process runs right now. Its lock time comes
	// from the wall clock, which can step back past processStart.
	if theirs.parent == os.Getpid() {
		return true
	}
	return theirs.startTicks <= mine.startTicks
}

// procStatus is what ownerMayLive reads from /proc/<pid>/stat.
type procStatus struct {
	parent     int
	startTicks uint64 // clock ticks since boot
}

func procStat(pid int) (procStatus, error) {
	raw, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/stat")
	if err != nil {
		return procStatus{}, err
	}
	return parseProcStat(string(raw))
}

// parseProcStat reads the parent (field 4) and the start time (field 22).
// The command name in field 2 can hold spaces and parentheses, so the fields
// after it start behind its last ')'.
func parseProcStat(s string) (procStatus, error) {
	end := strings.LastIndexByte(s, ')')
	if end < 0 {
		return procStatus{}, errors.New("no command name in /proc stat")
	}
	fields := strings.Fields(s[end+1:])
	if len(fields) < 20 {
		return procStatus{}, errors.New("short /proc stat")
	}
	parent, err := strconv.Atoi(fields[1])
	if err != nil {
		return procStatus{}, err
	}
	start, err := strconv.ParseUint(fields[19], 10, 64)
	if err != nil {
		return procStatus{}, err
	}
	return procStatus{parent: parent, startTicks: start}, nil
}
