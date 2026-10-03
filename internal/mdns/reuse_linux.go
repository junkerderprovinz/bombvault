package mdns

import (
	"syscall"

	"golang.org/x/sys/unix"
)

// reuseAddr lets the responder share port 5353 with a host's own responder,
// such as Avahi on Unraid when BombVault runs in the host network.
func reuseAddr(_, _ string, c syscall.RawConn) error {
	var sockErr error
	err := c.Control(func(fd uintptr) {
		if sockErr = unix.SetsockoptInt(int(fd), unix.SOL_SOCKET, unix.SO_REUSEADDR, 1); sockErr != nil {
			return
		}
		sockErr = unix.SetsockoptInt(int(fd), unix.SOL_SOCKET, unix.SO_REUSEPORT, 1)
	})
	if err != nil {
		return err
	}
	return sockErr
}
