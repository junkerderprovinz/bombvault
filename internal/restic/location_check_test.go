package restic

import (
	"errors"
	"path/filepath"
	"testing"
)

func TestCheckLocationRefusesAHostThatReadsAsAnOption(t *testing.T) {
	for _, loc := range []string{
		"sftp:-badhost:/srv/repo",
		"sftp:user@-badhost:/srv/repo",
		"sftp:first@second@-badhost:/srv/repo",
		"sftp:[-badhost]:/srv/repo",
		"sftp:user@[-badhost]:/srv/repo",
		"sftp://-badhost/srv/repo",
		"sftp://user@-badhost:2222//srv/repo",
		"  sftp:-badhost:/srv/repo",
		"rclone:-badremote:bucket",
	} {
		if err := CheckLocation(loc); !errors.Is(err, ErrLocationReadsAsOption) {
			t.Errorf("CheckLocation(%q) = %v, want a refusal", loc, err)
		}
	}
}

func TestCheckLocationPassesWhatRealLocationsLookLike(t *testing.T) {
	for _, loc := range []string{
		"",
		"user/backups/containers",
		"-folder/in/the/mount",
		"sftp:backup-box:/srv/repo",
		"sftp:user@backup-box:/srv/-repo",
		"sftp:-user@backup-box:/srv/repo",
		"sftp:user@[fe80::1]:/srv/repo",
		"sftp://user@backup-box:2222//srv/repo",
		"rclone:gdrive:-folder",
		"rclone::s3,provider=AWS:bucket",
		"rest:https://user:pass@-odd-but-http/repo",
		"s3:s3.amazonaws.com/-bucket",
		"b2:-bucket:path",
	} {
		if err := CheckLocation(loc); err != nil {
			t.Errorf("CheckLocation(%q) = %v, want nil", loc, err)
		}
	}
}

func TestNoResticProcessStartsForARefusedLocation(t *testing.T) {
	r := Restic{Bin: filepath.Join(t.TempDir(), "no-such-restic")}
	ctx := t.Context()
	bad := "sftp:-badhost:/srv/repo"

	checks := map[string]func() error{
		"snapshots": func() error { _, err := r.Snapshots(ctx, bad, Mode{}); return err },
		"init":      func() error { return r.Init(ctx, bad, Mode{}) },
		"copy source": func() error {
			return r.Copy(ctx, "user/backups/copy", bad, nil, Limits{}, Mode{})
		},
		"copy destination": func() error {
			return r.Copy(ctx, bad, "user/backups/own", nil, Limits{}, Mode{})
		},
		"ls": func() error {
			return r.LsStream(ctx, bad, "abcd", Mode{}, func(FileEntry) {})
		},
		"dump": func() error { return r.DumpRaw(ctx, bad, "abcd", "/x", nil, Mode{}) },
	}
	for name, run := range checks {
		if err := run(); !errors.Is(err, ErrLocationReadsAsOption) {
			t.Errorf("%s: got %v, want the location refused", name, err)
		}
	}
}
