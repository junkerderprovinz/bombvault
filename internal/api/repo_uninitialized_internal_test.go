package api

import (
	"errors"
	"testing"
)

// "Not initialised yet" and "the server refused you" are different answers, and
// restic's wrapper text does not distinguish them: it prefixes "unable to open
// config file" onto the transport failure as well.
//
// The three messages below are VERBATIM from a real rest-server started with
// --private-repos and an htpasswd file, captured before the fix. Reported as a
// remote primary repository stuck on "reachable, not initialized" with nothing
// to act on (issue #192).
func TestIsRepoUninitializedTellsRefusalFromEmptiness(t *testing.T) {
	for _, tc := range []struct {
		name string
		msg  string
		want bool
	}{
		{
			// The only genuine case: credentials accepted, no repository there.
			name: "right password, no repository yet",
			msg:  "Fatal: repository does not exist: unable to open config file: <config/> does not exist",
			want: true,
		},
		{
			// Wrong password. Was reported as "reachable, not initialized",
			// which reads as "your destination is fine, just empty".
			name: "wrong password",
			msg:  "Fatal: unable to open config file: unexpected HTTP response (401): 401 Unauthorized",
			want: false,
		},
		{
			// Right password, but the URL's first path segment is not the
			// htpasswd user, which --private-repos refuses. Same misleading
			// result, and one of the two commonest setup mistakes.
			name: "private-repos path belongs to another user",
			msg:  "Fatal: unable to open config file: unexpected HTTP response (401): 401 Unauthorized",
			want: false,
		},
		{
			// The same shape from an S3-style backend.
			name: "forbidden",
			msg:  "unable to open config file: AccessDenied: 403 Forbidden",
			want: false,
		},
		{
			// A local repository that was never initialised: no transport, no
			// status code, still genuinely empty.
			name: "local path with no repository",
			msg:  "Fatal: unable to open config file: stat /mnt/user/backups/config: no such file or directory",
			want: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := isRepoUninitialized(errors.New(tc.msg)); got != tc.want {
				t.Errorf("isRepoUninitialized(%q) = %v, want %v", tc.msg, got, tc.want)
			}
		})
	}

	if isRepoUninitialized(nil) {
		t.Error("no error is not an uninitialised repository")
	}
}
