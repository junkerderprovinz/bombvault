package api

import (
	"errors"
	"strings"
	"testing"
)

// The hint explains the two usual causes of a rest-server 401 and must stay off
// errors it would mislead.
func TestRestAuthHint(t *testing.T) {
	cases := []struct {
		name string
		msg  string
		want bool
	}{
		{
			name: "unexpected server response 401",
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
			// runError keeps the most informative stderr line, and restic prints
			// the URL on the next one, which lastReason skips as boilerplate. The
			// message users actually see therefore has no rest: URL in it.
			name: "the message BombVault itself shows, which carries no URL",
			msg:  "restic cat failed: Fatal: unable to open config file: unexpected HTTP response (401): 401 Unauthorized",
			want: true,
		},
		{
			// An S3 403 has other causes (access key, bucket policy, clock skew),
			// and rest-server advice would send the user looking for htpasswd.
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
			// The hint has to name both causes.
			for _, must := range []string{"--private-repos", "first path segment", "credential set", "connection test"} {
				if !strings.Contains(got, must) {
					t.Errorf("hint does not mention %q: %s", must, got)
				}
			}
		})
	}
}

// Every user-facing error goes through scrubError, so the hint has to survive it.
func TestScrubErrorCarriesTheRestAuthHint(t *testing.T) {
	err := errors.New("Fatal: unable to open repository at rest:http://box:8000/repo: server response unexpected: 401 Unauthorized")
	got := scrubError(err)
	if !strings.Contains(got, "--private-repos") {
		t.Fatalf("scrubError dropped the hint: %s", got)
	}
	// A key mismatch is not an auth refusal and keeps its own APP_KEY message.
	keyErr := errors.New("Fatal: wrong password or no key found for rest:http://box:8000/repo")
	if got := scrubError(keyErr); !strings.Contains(got, "APP_KEY") {
		t.Fatalf("the APP_KEY mapping lost its precedence: %s", got)
	}
}
