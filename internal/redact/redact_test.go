package redact

import "testing"

func TestCredentialsKeepsTheSchemeOfAShortSFTPLocation(t *testing.T) {
	for in, want := range map[string]string{
		"sftp:backup@box:/srv/repo":             "sftp:backup@box:/srv/repo",
		"sftp:backup:invented@box:/srv/repo":    "sftp:[redacted]@box:/srv/repo",
		"sftp://backup:invented@box:22//repo":   "sftp://[redacted]@box:22//repo",
		"rest:https://bob:invented@box:8000/r":  "rest:https://[redacted]@box:8000/r",
		"s3:https://key:invented@box/bucket":    "s3:https://[redacted]@box/bucket",
		"opening sftp:backup@box:/srv/r failed": "opening sftp:backup@box:/srv/r failed",
	} {
		if got := Credentials(in); got != want {
			t.Errorf("Credentials(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestRestorePutsTheCredentialBackIntoAnEditOfThePath(t *testing.T) {
	for _, c := range []struct{ name, stored, edited, want string }{
		{
			name:   "nothing changed",
			stored: "rest:https://bob:invented@box:8000/repo",
			edited: "rest:https://[redacted]@box:8000/repo",
			want:   "rest:https://bob:invented@box:8000/repo",
		},
		{
			name:   "another path on the same server",
			stored: "rest:https://bob:invented@box:8000/repo",
			edited: "rest:https://[redacted]@box:8000/moved",
			want:   "rest:https://bob:invented@box:8000/moved",
		},
		{
			name:   "another bucket with the same rclone connection string",
			stored: `rclone::s3,access_key_id=AKIA,secret_access_key="in,vented":bucket/old`,
			edited: "rclone::s3,access_key_id=[redacted],secret_access_key=[redacted]:bucket/new",
			want:   `rclone::s3,access_key_id=AKIA,secret_access_key="in,vented":bucket/new`,
		},
		{
			name:   "a location an import left masked stays masked",
			stored: "rest:https://[redacted]@box:8000/repo",
			edited: "rest:https://[redacted]@box:8000/repo",
			want:   "rest:https://[redacted]@box:8000/repo",
		},
	} {
		got, ok := Restore(c.edited, c.stored)
		if !ok || got != c.want {
			t.Errorf("%s: Restore = %q, %v, want %q", c.name, got, ok, c.want)
		}
	}
}

func TestRestoreRefusesToCarryACredentialSomewhereElse(t *testing.T) {
	for _, c := range []struct{ name, stored, edited string }{
		{"another host", "rest:https://bob:invented@box:8000/repo", "rest:https://[redacted]@other:8000/repo"},
		{"another port", "rest:https://bob:invented@box:8000/repo", "rest:https://[redacted]@box:9000/repo"},
		{"a longer port", "rest:https://bob:invented@box:8000/repo", "rest:https://[redacted]@box:80001/repo"},
		{"a longer host", "rest:https://bob:invented@box/repo", "rest:https://[redacted]@box.example.com/repo"},
		{"a longer host with no path stored", "rest:https://bob:invented@box", "rest:https://[redacted]@box.example.com/repo"},
		{"another parameter", "rclone::s3,secret_access_key=invented:bucket", "rclone::s3,secret_access_key=[redacted],endpoint=other:bucket"},
		{"another scheme", "rest:https://bob:invented@box:8000/repo", "rest:http://[redacted]@box:8000/repo"},
		{"another rclone backend", "rclone::s3,secret_access_key=invented:bucket", "rclone::b2,secret_access_key=[redacted]:bucket"},
		{"nothing stored to bring back", "", "rest:https://[redacted]@box:8000/repo"},
		{"nothing hidden in what is stored", "rest:https://box:8000/repo", "rest:https://[redacted]@box:8000/repo"},
	} {
		if got, ok := Restore(c.edited, c.stored); ok {
			t.Errorf("%s: Restore = %q, want a refusal", c.name, got)
		}
	}
}
