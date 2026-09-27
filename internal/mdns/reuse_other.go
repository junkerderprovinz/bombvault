//go:build !linux

package mdns

import "syscall"

// reuseAddr leaves the socket as it is. BombVault runs on Linux, where
// reuse_linux.go shares the port with the host's own responder.
func reuseAddr(_, _ string, _ syscall.RawConn) error { return nil }
