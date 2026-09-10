package api

import (
	"errors"
	"strings"
	"testing"
)

// The 401 that ended issue #194 was not a wrong password and not a missing
// --private-repos segment. It was one character: the credential set signed in as
// "bombvault_containers" while the URL began "bombvault-containers". Both values
// sat in BombVault at the moment it asked, and it said "401 Unauthorized"
// anyway. These checks pin the comparison to the case it can prove and keep it
// quiet everywhere it would be guessing.
func TestRestPathUserMismatch(t *testing.T) {
	unauthorized := errors.New("restic cat failed: Fatal: unable to open config file: unexpected HTTP response (401): 401 Unauthorized")
	env := func(user string) []string {
		return []string{"RESTIC_REST_PASSWORD=hunter2", "RESTIC_REST_USERNAME=" + user}
	}

	cases := []struct {
		name string
		err  error
		repo string
		env  []string
		want bool
	}{
		{
			name: "the underscore against the hyphen, from the issue",
			err:  unauthorized,
			repo: "rest:http://192.168.100.29:8000/bombvault-containers/containers",
			env:  env("bombvault_containers"),
			want: true,
		},
		{
			name: "the same word on both sides is not the problem",
			err:  unauthorized,
			repo: "rest:http://192.168.100.29:8000/bombvault-containers/containers",
			env:  env("bombvault-containers"),
			want: false,
		},
		{
			// A single segment is an ordinary repository path on a server
			// running without --private-repos, where the 401 means a wrong
			// password. Naming a "mismatch" there would send the reader to
			// rename something that is already right.
			name: "a URL with no user segment says nothing about a user",
			err:  unauthorized,
			repo: "rest:http://192.168.100.29:8000/containers",
			env:  env("bombvault-containers"),
			want: false,
		},
		{
			name: "no REST user configured, nothing to compare",
			err:  unauthorized,
			repo: "rest:http://192.168.100.29:8000/bombvault-containers/containers",
			env:  []string{"AWS_ACCESS_KEY_ID=AKIA"},
			want: false,
		},
		{
			name: "an S3 destination never reaches this",
			err:  unauthorized,
			repo: "s3:s3.amazonaws.com/bucket/containers",
			env:  env("bombvault-containers"),
			want: false,
		},
		{
			// Every other failure keeps its own words. A destination that is
			// switched off is not a naming mistake.
			name: "a failure that is not a refusal",
			err:  errors.New("restic cat failed: Fatal: unable to open config file: dial tcp 192.168.100.29:8000: connect: connection refused"),
			repo: "rest:http://192.168.100.29:8000/bombvault-containers/containers",
			env:  env("bombvault_containers"),
			want: false,
		},
		{
			name: "no error, no verdict",
			err:  nil,
			repo: "rest:http://192.168.100.29:8000/bombvault-containers/containers",
			env:  env("bombvault_containers"),
			want: false,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := restPathUserMismatch(c.err, c.repo, c.env)
			if (got != nil) != c.want {
				t.Fatalf("restPathUserMismatch = %v, want a message: %v", got, c.want)
			}
			if !c.want {
				return
			}
			// Both words have to be IN it. A message that says the two differ
			// without saying what they are leaves the reader exactly where the
			// bare 401 left him.
			for _, must := range []string{"bombvault_containers", "bombvault-containers", "--private-repos", "htpasswd"} {
				if !strings.Contains(got.Error(), must) {
					t.Errorf("message does not carry %q: %s", must, got)
				}
			}
			if !errors.Is(got, errRestPathUser) {
				t.Error("the message is not classified as errRestPathUser, so scrubError cannot let it through")
			}
		})
	}
}

// The comparison is worth nothing if the funnel every user-facing error passes
// through swaps it for the generic two-cause hint on the way out.
func TestScrubErrorKeepsTheNamedMismatch(t *testing.T) {
	err := restPathUserMismatch(
		errors.New("restic cat failed: Fatal: unable to open config file: unexpected HTTP response (401): 401 Unauthorized"),
		"rest:http://192.168.100.29:8000/bombvault-containers/containers",
		[]string{"RESTIC_REST_USERNAME=bombvault_containers"},
	)
	if err == nil {
		t.Fatal("no message to carry")
	}
	got := scrubError(err)
	if got != err.Error() {
		t.Fatalf("scrubError changed the message:\n got: %s\nwant: %s", got, err.Error())
	}
	if strings.Contains(got, "Two things cause almost every one of these") {
		t.Error("the generic hint replaced the message that names the actual difference")
	}
}

func TestRestRepoUserSegment(t *testing.T) {
	cases := map[string]string{
		"rest:http://box:8000/tower/containers":       "tower",
		"rest:https://box:8000/tower/containers/":     "tower",
		"rest:http://user:pw@box:8000/tower/flash":    "tower",
		"rest:http://box:8000/containers":             "",
		"rest:http://box:8000/":                       "",
		"rest:http://box:8000":                        "",
		"sftp:box:/mnt/user/backups/containers":       "",
		"/mnt/user/backups/containers":                "",
		"rest:http://box:8000/tower/deep/repo/nested": "tower",
	}
	for repo, want := range cases {
		if got := restRepoUserSegment(repo); got != want {
			t.Errorf("restRepoUserSegment(%q) = %q, want %q", repo, got, want)
		}
	}
}

func TestRestEnvUser(t *testing.T) {
	if got := restEnvUser([]string{"AWS_DEFAULT_REGION=eu-central-1"}); got != "" {
		t.Errorf("env without a REST user returned %q", got)
	}
	if got := restEnvUser([]string{"RESTIC_REST_USERNAME=tower", "RESTIC_REST_PASSWORD=x"}); got != "tower" {
		t.Errorf("got %q, want tower", got)
	}
	// applyTargetCreds replaces the whole env, but a duplicate would be read by
	// the child process as the last one wins, so read it the same way.
	if got := restEnvUser([]string{"RESTIC_REST_USERNAME=shared", "RESTIC_REST_USERNAME=tower"}); got != "tower" {
		t.Errorf("got %q, want the last assignment", got)
	}
}
