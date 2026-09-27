package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

// rcloneAboutTimeout bounds the one API call the probe makes. A remote that is
// slow to answer must not hold up the backup that triggered the sample.
const rcloneAboutTimeout = 30 * time.Second

// errAboutUnsupported is a backend that cannot say how much room it has. It is
// a sentinel rather than a silent zero, because a repository nobody can measure
// has to be named as unmeasured instead of looking healthy.
var errAboutUnsupported = errors.New("this backend does not report its free space")

// errNotAnRcloneRemote is a stored location that does not name a remote. It is
// never rendered with the location itself, which can carry a bucket and a user.
var errNotAnRcloneRemote = errors.New("this repository location does not name an rclone remote")

// aboutResult is a remote's room. Total is nil on a backend that reports only
// what is left.
type aboutResult struct {
	Free  int64
	Total *int64
}

// rcloneAbout asks an rclone remote how much room it has, using this
// instance's own rclone configuration rather than whatever the process
// environment happens to carry.
func rcloneAbout(ctx context.Context, configPath, remote string) (aboutResult, error) {
	ctx, cancel := context.WithTimeout(ctx, rcloneAboutTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "rclone", rcloneAboutArgs(remote)...) //nolint:gosec // G204: the remote comes from a stored repository location
	cmd.Env = append(os.Environ(), "RCLONE_CONFIG="+configPath)
	out, err := cmd.Output()
	if err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) && strings.Contains(strings.ToLower(string(exit.Stderr)), "about") {
			return aboutResult{}, errAboutUnsupported
		}
		return aboutResult{}, fmt.Errorf("rclone about: %w", err)
	}
	return parseRcloneAbout(out)
}

func parseRcloneAbout(out []byte) (aboutResult, error) {
	var answer struct {
		Total *int64 `json:"total"`
		Free  *int64 `json:"free"`
	}
	if err := json.Unmarshal(out, &answer); err != nil {
		return aboutResult{}, fmt.Errorf("rclone about: parse JSON: %w", err)
	}
	if answer.Free == nil {
		return aboutResult{}, errAboutUnsupported
	}
	return aboutResult{Free: *answer.Free, Total: answer.Total}, nil
}

// rcloneAboutArgs is the argv for asking one remote how much room it has. The
// remote is a positional behind the end-of-flags marker, so a stored location
// that begins with a dash cannot become an option.
func rcloneAboutArgs(remote string) []string {
	return []string{"about", "--json", "--", remote}
}

// rcloneRemoteName is what rclone takes before the colon: a configured remote
// starts with a letter, a digit or an underscore.
var rcloneRemoteName = regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9_. -]*:`)

// rcloneRemoteOf turns a restic repository location into the remote an about
// call takes: "rclone:box:bv" is rclone's own "box:bv". A location that names
// no remote is refused rather than handed to rclone.
func rcloneRemoteOf(loc string) (string, error) {
	remote := strings.TrimPrefix(strings.TrimSpace(loc), "rclone:")
	if !rcloneRemoteName.MatchString(remote) {
		return "", errNotAnRcloneRemote
	}
	return remote, nil
}
