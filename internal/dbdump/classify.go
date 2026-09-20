package dbdump

import "strings"

// Observation is everything one dump attempt left behind.
type Observation struct {
	Cancelled, TimedOut, WriteFailed, DockerErr, NotRunning bool
	Exit                                                    int
	StderrTail                                              string
	Bytes                                                   int64
	MarkerSeen                                              bool
}

// Classify names the reason a dump attempt failed; the empty string means it
// succeeded. The first match wins, so a user's cancel is never booked as a
// time limit and a tool error never hides a container that went away.
func Classify(o Observation) string {
	switch {
	case o.Cancelled:
		return ReasonCancelled
	case o.TimedOut, o.Exit == 124, o.Exit == 143:
		return ReasonTimeout
	case o.NotRunning:
		return ReasonNotRunning
	case o.DockerErr:
		return ReasonDocker
	case o.WriteFailed:
		return ReasonWrite
	}
	switch o.Exit {
	case ExitNoClient:
		return ReasonNoClient
	case ExitSecret:
		return ReasonSecret
	case ExitNoCredentials:
		return ReasonNoCredentials
	}
	if o.Exit != 0 {
		if reason := reasonFromStderr(o.StderrTail); reason != "" {
			return reason
		}
		return ReasonTool
	}
	if o.Bytes == 0 {
		return ReasonEmpty
	}
	if !o.MarkerSeen {
		return ReasonIncomplete
	}
	return ""
}

// reasonFromStderr reads the tool's own message. needs-upgrade comes first
// because MariaDB reports the wrong system table count behind a line that
// looks like an authentication failure.
func reasonFromStderr(tail string) string {
	s := strings.ToLower(tail)
	switch {
	case strings.Contains(s, "(1558)"), strings.Contains(s, "error 1558"),
		strings.Contains(s, "column count of mysql.") && strings.Contains(s, "upgrade"):
		return ReasonNeedsUpgrade
	case strings.Contains(s, "access denied for user"),
		strings.Contains(s, "password authentication failed"),
		strings.Contains(s, "peer authentication failed"),
		strings.Contains(s, "no password supplied"),
		strings.Contains(s, `role "`) && strings.Contains(s, "does not exist"),
		strings.Contains(s, "error 1045"), strings.Contains(s, "error: 1045"),
		strings.Contains(s, "error 1698"):
		return ReasonAuth
	case strings.Contains(s, "insufficient privileges"),
		strings.Contains(s, "you need (at least one of)"),
		strings.Contains(s, "access denied; you need"):
		return ReasonPrivileges
	case strings.Contains(s, "can't connect to local"),
		strings.Contains(s, "could not connect to server"),
		strings.Contains(s, "connection to server on socket"),
		strings.Contains(s, "no such file or directory") && strings.Contains(s, "socket"):
		return ReasonUnreachable
	}
	return ""
}
