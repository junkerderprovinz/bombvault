package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
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

	cmd := exec.CommandContext(ctx, "rclone", "about", "--json", remote) //nolint:gosec // G204: the remote comes from a stored repository location
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

// rcloneRemoteOf turns a restic repository location into the remote an about
// call takes: "rclone:box:bv" is rclone's own "box:bv".
func rcloneRemoteOf(loc string) string {
	return strings.TrimPrefix(strings.TrimSpace(loc), "rclone:")
}
