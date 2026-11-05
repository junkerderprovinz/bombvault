package restic

import (
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/junkerderprovinz/bombvault/internal/redact"
)

// ErrLocationReadsAsOption refuses a location whose host or remote starts with
// a dash.
var ErrLocationReadsAsOption = errors.New("the host or rclone remote of a storage location can't start with a dash")

// CheckLocation refuses a remote location that restic would pass on in a place
// where it reads as a command-line option: the host of an sftp location goes to
// ssh and the remote of an rclone location to rclone, each as a plain argument.
// No real host or rclone remote starts with a dash, so this takes nothing away
// from a location that works.
func CheckLocation(loc string) error {
	loc = strings.TrimSpace(loc)
	var refused bool
	switch {
	case strings.HasPrefix(loc, "sftp://"):
		// A location url.Parse refuses never reaches ssh, since restic parses it
		// the same way.
		if u, err := url.Parse(loc); err == nil {
			refused = strings.HasPrefix(u.Hostname(), "-")
		}
	case strings.HasPrefix(loc, "sftp:"):
		refused = sftpShortHostReadsAsOption(strings.TrimPrefix(loc, "sftp:"))
	case strings.HasPrefix(loc, "rclone:"):
		refused = strings.HasPrefix(strings.TrimPrefix(loc, "rclone:"), "-")
	}
	if refused {
		return fmt.Errorf("%w: %s", ErrLocationReadsAsOption, redact.Credentials(loc))
	}
	return nil
}

// sftpShortHostReadsAsOption looks at user@host:path in both of the ways it can
// be split: at the first colon, and at the colon after a bracketed address,
// which may have a colon of the user name in front of it.
func sftpShortHostReadsAsOption(s string) bool {
	head, _, _ := strings.Cut(s, ":")
	host := head[strings.LastIndex(head, "@")+1:]
	if strings.HasPrefix(strings.TrimPrefix(host, "["), "-") {
		return true
	}
	at := strings.LastIndex(s, "@[")
	return at >= 0 && strings.HasPrefix(s[at+len("@["):], "-")
}

// checkRepoArgs is the last stop before a restic process starts. It catches what
// the routes that store a location never saw, such as an older row or a location
// read from a recovery kit.
func checkRepoArgs(args []string) error {
	for i, arg := range args[:max(len(args)-1, 0)] {
		if arg == "-r" || arg == "--from-repo" {
			if err := CheckLocation(args[i+1]); err != nil {
				return err
			}
		}
	}
	return nil
}
