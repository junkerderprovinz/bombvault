package api

// A failure that does not say the object is missing must read as unreachable,
// or BombVault may init an empty repository beside real backups. Every
// backend's real "does not exist" wording must still read as absent, or each
// first-time setup would ask the user for a mode they cannot know.

import (
	"errors"
	"testing"
)

// These messages carry restic's loose "unable to open config file" but name no
// absence: the backend was not reached or answered something else.
func TestUnenumeratedBackendFailureIsUnreachable(t *testing.T) {
	eng := &modeStubEngine{encrypted: map[string]bool{}}
	s, _, _ := newDetectSvc(t, eng)

	cases := map[string]string{
		// A reverse proxy answering for a rest-server container that is down.
		"proxy 502": `Fatal: unable to open config file: unexpected HTTP response (502): 502 Bad Gateway
Is there a repository at the following location?
rest:https://backup.example.com/repo`,
		"proxy 503":     "Fatal: unable to open config file: unexpected HTTP response (503): 503 Service Unavailable",
		"server 500":    "Fatal: unable to open config file: unexpected HTTP response (500): 500 Internal Server Error",
		"proxy 404":     "Fatal: unable to open config file: unexpected HTTP response (404): 404 Not Found",
		"gateway html":  "Fatal: unable to open config file: unexpected HTTP response (502): <html><head><title>502 Bad Gateway</title></head></html>",
		"rclone remote": `Fatal: unable to open config file: rclone stdio connection: Failed to create file system for "b2:bucket/repo": didn't find section in config file`,
		"rclone exit":   "Fatal: unable to open config file: rclone: exit status 1",
		"s3 clock skew": "Fatal: unable to open config file: Stat: RequestTimeTooSkewed: The difference between the request time and the current time is too large.",
		"bare fatal":    "Fatal: unable to open config file",
	}
	for name, msg := range cases {
		t.Run(name, func(t *testing.T) {
			err := errors.New(msg)
			if isRepoDefinitelyAbsent(err) {
				t.Fatalf("%q must not be read as a definitely-absent repository", msg)
			}
			state, detail := s.classifyClosedRepo("rest:https://backup.example.com/repo", err)
			if state != RepoUnreachable {
				t.Fatalf("state = %q, want unreachable", state)
			}
			if detail == "" {
				t.Fatal("expected the failure to be reported to the user")
			}
		})
	}
}

func TestBackendSaysObjectMissingIsAbsent(t *testing.T) {
	eng := &modeStubEngine{encrypted: map[string]bool{}}
	s, _, _ := newDetectSvc(t, eng)

	cases := map[string]string{
		// Verbatim from a real rest-server, see listsnapshots_internal_test.go.
		"rest":   "Fatal: unable to open config file: <config/> does not exist\nIs there a repository at the following location?\nrest:http://box:8000/flash",
		"restic": "Fatal: repository does not exist: unable to open config file",
		"s3":     "Fatal: unable to open config file: Stat: The specified key does not exist.",
		"minio":  "Fatal: unable to open config file: Stat: NoSuchKey: the specified key does not exist",
		"b2":     "Fatal: unable to open config file: b2_download_file_by_name: 404: File with such name does not exist",
		"azure":  "Fatal: unable to open config file: BlobNotFound",
		"gcs":    "Fatal: unable to open config file: storage: object doesn't exist",
		"swift":  "Fatal: unable to open config file: Object Not Found",
		"sftp":   "Fatal: unable to open config file: stat /srv/backup/repo/config: file does not exist",
	}
	for name, msg := range cases {
		t.Run(name, func(t *testing.T) {
			err := errors.New(msg)
			if !isRepoDefinitelyAbsent(err) {
				t.Fatalf("%q states the object is missing and must still count as absent", msg)
			}
			state, _ := s.classifyClosedRepo("rest:http://offsite:8000/repo", err)
			if state != RepoAbsent {
				t.Fatalf("state = %q, want absent", state)
			}
		})
	}
}

// A named transport failure outweighs absence wording: "no such host" is a
// dead name, not an empty repository.
func TestTransportVetoBeatsAbsenceWording(t *testing.T) {
	cases := []string{
		"Fatal: unable to open config file: dial tcp: lookup backup.example: no such host",
		"Fatal: unable to open config file: open /mnt/remote/config: permission denied: no such file or directory",
		`Fatal: unable to open config file: Head "https://x/config": context deadline exceeded: object not found`,
	}
	for _, msg := range cases {
		if isRepoDefinitelyAbsent(errors.New(msg)) {
			t.Fatalf("%q names a transport failure and must never read as absent", msg)
		}
	}
}

// One off-site repo behind a broken proxy leaves the verdict at unknown, which
// is never applied, rather than absent.
func TestDetectFoldsBrokenRemoteToUnknown(t *testing.T) {
	repos := []RepoEncryption{
		{Domain: "containers", Source: "offsite", State: RepoUnreachable, Err: "502 Bad Gateway"},
	}
	if v := foldEncryption(repos); v != VerdictUnknown {
		t.Fatalf("verdict = %q, want unknown", v)
	}
	if _, definite := VerdictUnknown.encryptionEnabled(); definite {
		t.Fatal("an unknown verdict must never be applied to the stored setting")
	}
}
