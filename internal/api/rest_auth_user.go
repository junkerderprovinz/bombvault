package api

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
)

// A rest-server started with --private-repos hands each htpasswd user only the
// tree under its own name, so the repository URL's first path segment IS the
// user. Get that one word wrong and the server answers 401, the same 401 a wrong
// password gives, and nothing in the answer says which of the two it was.
//
// Issue #194 is what that costs. Its reporter had every piece right and still
// could not get a backup out: the credential set authenticated as
// "bombvault_containers" while the URL began "bombvault-containers". One
// character, an underscore where the generated recipe writes a hyphen, three
// screenshots deep and invisible to him, to me, and to restic, which can only
// repeat what the server said.
//
// BombVault holds both values at the moment it asks. Comparing them is not a
// guess about the cause, it is the cause, so the error says it outright instead
// of listing what usually goes wrong.
var errRestPathUser = errors.New("rest repository path does not name the credential user")

// restPathUserErr carries the ready-to-show comparison and satisfies
// errors.Is(err, errRestPathUser). Same shape as restoreDestErr and the restic
// package's sentinels: the text is the message, the type is the classification.
type restPathUserErr struct{ msg string }

func (e *restPathUserErr) Error() string { return e.msg }

func (e *restPathUserErr) Is(target error) bool { return target == errRestPathUser }

// restEnvUser returns the RESTIC_REST_USERNAME value a restic Mode's env carries,
// or "" when it carries none (an install that never filled in REST credentials,
// or an S3/sftp destination). A later assignment wins, matching how the child
// process would read it.
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

// restRepoUserSegment returns the first path segment of a rest: repository URL,
// but only when a second segment follows it.
//
// That condition is the difference between "the user typed the wrong name" and
// "the user left the name out", and only the first is safe to state as a fact.
// A URL like rest:http://box:8000/containers is a perfectly ordinary repository
// path on a server running WITHOUT --private-repos, where the 401 then means a
// wrong password. Those fall through to restAuthHint, whose first cause covers
// exactly them.
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

// restPathUserMismatch turns a refused rest repository into the one sentence that
// ends the search, or returns nil when it cannot say that much. nil means the
// caller keeps the error it had, so an ordinary wrong password still reads as one.
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
