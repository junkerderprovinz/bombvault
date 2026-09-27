package main

import (
	"net"
	"strconv"
	"time"
)

// tcpProbe dials address until a connection is accepted or the given number
// of seconds has passed, and returns 0 on success. Its arguments are the
// address and the seconds.
func tcpProbe(args []string, every time.Duration) int {
	if len(args) != 2 {
		return 2
	}
	secs, err := strconv.Atoi(args[1])
	if err != nil || secs < 1 {
		return 2
	}
	deadline := time.Now().Add(time.Duration(secs) * time.Second)
	for {
		conn, err := net.DialTimeout("tcp", args[0], 3*time.Second) //nolint:gosec // G704: the command exists to dial the address it is given
		if err == nil {
			_ = conn.Close()
			return 0
		}
		if time.Now().Add(every).After(deadline) {
			return 1
		}
		time.Sleep(every)
	}
}
