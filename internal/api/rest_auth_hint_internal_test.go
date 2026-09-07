package api

import (
	"errors"
	"strings"
	"testing"
)

// A 401 from a rest-server has two causes and the code knew both of them, in a
// comment above isRepoUninitialized, where only a maintainer would ever read it.
// Issue #194 is what that costs: a user spent hours on "I continue to get a 401
// error", could not tell which of the two he had hit, and left. These checks
// pin the hint to the case it belongs to and, just as importantly, keep it away
// from the cases it would mislead.
func TestRestAuthHint(t *testing.T) {
	cases := []struct {
		name string
		msg  string
		want bool
	}{
		{
			name: "the real one from the issue",
			msg:  "Fatal: unable to open repository at rest:http://192.168.1.50:8000/backups: server response unexpected: 401 Unauthorized",
			want: true,
		},
		{
			name: "the wording restic uses for a config read",
			msg:  "Fatal: unable to open config file: unexpected HTTP response (401): 401 Unauthorized for rest:http://box:8000/user/repo",
			want: true,
		},
		{
			name: "spelled out rather than numeric",
			msg:  "unable to open repository at rest:http://box:8000/repo: Unauthorized",
			want: true,
		},
		{
			// An S3 403 is a different problem with different causes (a wrong
			// access key, a bucket policy, a clock skew), and the rest-server
			// advice would send that user looking for an htpasswd file that
			// does not exist.
			name: "an S3 refusal keeps its own wording",
			msg:  "Fatal: unable to open repository at s3:s3.amazonaws.com/bucket: Access Denied 403",
			want: false,
		},
		{
			// The digits can appear in a port, a byte count or a path.
			name: "the digits 401 elsewhere are not an auth failure",
			msg:  "Fatal: unable to open repository at rest:http://box:8401/repo: connection refused",
			want: false,
		},
		{
			name: "a missing repository is not an auth failure",
			msg:  "Fatal: repository does not exist: unable to open config file at rest:http://box:8000/repo",
			want: false,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := restAuthHint(c.msg)
			if (got != "") != c.want {
				t.Fatalf("restAuthHint(%q) = %q, want hint=%v", c.msg, got, c.want)
			}
			if !c.want {
				return
			}
			// The hint has to name both causes, or it sends the reader down
			// one path and leaves the other one to be found by accident.
			for _, must := range []string{"--private-repos", "first path segment", "credential set", "connection test"} {
				if !strings.Contains(got, must) {
					t.Errorf("hint does not mention %q: %s", must, got)
				}
			}
		})
	}
}

// scrubError is the single funnel every user-facing error goes through, so the
// hint has to survive it rather than only exist next to it.
func TestScrubErrorCarriesTheRestAuthHint(t *testing.T) {
	err := errors.New("Fatal: unable to open repository at rest:http://box:8000/repo: server response unexpected: 401 Unauthorized")
	got := scrubError(err)
	if !strings.Contains(got, "--private-repos") {
		t.Fatalf("scrubError dropped the hint: %s", got)
	}
	// The older mapping must keep winning where it applies: a key mismatch is
	// not an auth refusal, and its own message is the more useful one.
	keyErr := errors.New("Fatal: wrong password or no key found for rest:http://box:8000/repo")
	if got := scrubError(keyErr); !strings.Contains(got, "APP_KEY") {
		t.Fatalf("the APP_KEY mapping lost its precedence: %s", got)
	}
}
