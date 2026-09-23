package api

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
)

// errRestPathUser marks a 401 where the credential user differs from the first
// path segment of the repository URL. A rest-server running --private-repos
// gives each htpasswd user only the tree under its own name and answers such a
// request with the same 401 as a wrong password. BombVault knows both values,
// so it can name the difference instead of guessing.
var errRestPathUser = errors.New("rest repository path does not name the credential user")

// restPathUserErr carries the message and matches errRestPathUser, like
// restoreDestErr.
type restPathUserErr struct{ msg string }

func (e *restPathUserErr) Error() string { return e.msg }

func (e *restPathUserErr) Is(target error) bool { return target == errRestPathUser }

// restEnvUser returns RESTIC_REST_USERNAME from env, or "" when it is unset.
// The last assignment wins, as it does for the child process.
func restEnvUser(env []string) string {
	const key = "RESTIC_REST_USERNAME="
	user := ""
	for _, kv := range env {
		if strings.HasPrefix(kv, key) {
			user = strings.TrimPrefix(kv, key)
		}
	}
	return user
}

// restRepoUserSegment returns the first path segment of a rest: repository URL
// if a second segment follows it. A single segment such as
// rest:http://box:8000/containers is an ordinary repository on a server without
// --private-repos, where a 401 means a wrong password; restAuthHint covers it.
func restRepoUserSegment(repo string) string {
	body := strings.TrimPrefix(repo, "rest:")
	if body == repo {
		return "" // not a rest repository
	}
	u, err := url.Parse(body)
	if err != nil {
		return ""
	}
	segs := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(segs) < 2 || segs[0] == "" {
		return ""
	}
	return segs[0]
}

// restPathUserMismatch returns an error naming both values when a refused rest
// repository's user and path segment differ. It returns nil when it cannot
// tell, and the caller keeps its original error.
func restPathUserMismatch(err error, repo string, env []string) error {
	if err == nil || !isAuthRefusal(strings.ToLower(err.Error())) {
		return nil
	}
	user, segment := restEnvUser(env), restRepoUserSegment(repo)
	if user == "" || segment == "" || user == segment {
		return nil
	}
	return &restPathUserErr{msg: fmt.Sprintf(
		"the rest-server rejected these credentials (401). BombVault signs in as %q, but the repository URL "+
			"starts with %q. A server running with --private-repos gives each user only the tree under its own "+
			"name, so those two have to be the same word. Change whichever one is wrong: the user in the "+
			"credential set, the first part of the repository URL, or the name in front of the colon in the "+
			"htpasswd line on the storage box. All three carry the same user.",
		user, segment)}
}
