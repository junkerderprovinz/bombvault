package restic_test

import (
	"reflect"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/restic"
)

func TestInitArgs(t *testing.T) {
	t.Run("encrypted", func(t *testing.T) {
		got := restic.InitArgs("/repo", restic.Mode{Encrypted: true, Password: "pw"})
		want := []string{"-r", "/repo", "init"}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("got %v want %v", got, want)
		}
	})
	t.Run("unencrypted", func(t *testing.T) {
		got := restic.InitArgs("/repo", restic.Mode{Encrypted: false})
		want := []string{"-r", "/repo", "init", "--insecure-no-password"}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("got %v want %v", got, want)
		}
	})
}

func TestForgetPolicyArgs(t *testing.T) {
	t.Run("emits only set dimensions + prune", func(t *testing.T) {
		got := restic.ForgetPolicyArgs("/repo",
			restic.RetentionPolicy{KeepLast: 5, KeepMonthly: 6}, restic.Mode{Encrypted: true})
		want := []string{"-r", "/repo", "forget", "--keep-last", "5", "--keep-monthly", "6", "--prune"}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("got %v want %v", got, want)
		}
	})
	t.Run("unencrypted adds insecure flag, full policy", func(t *testing.T) {
		got := restic.ForgetPolicyArgs("/repo",
			restic.RetentionPolicy{KeepLast: 3, KeepDaily: 7, KeepWeekly: 4, KeepMonthly: 12},
			restic.Mode{Encrypted: false})
		want := []string{"-r", "/repo", "forget", "--insecure-no-password",
			"--keep-last", "3", "--keep-daily", "7", "--keep-weekly", "4", "--keep-monthly", "12", "--prune"}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("got %v want %v", got, want)
		}
	})
}

func TestCheckArgs(t *testing.T) {
	if got, want := restic.CheckArgs("/repo", restic.Mode{Encrypted: true}), []string{"-r", "/repo", "check"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
	if got, want := restic.CheckArgs("/repo", restic.Mode{Encrypted: false}), []string{"-r", "/repo", "check", "--insecure-no-password"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestIsRemoteRepo(t *testing.T) {
	remote := []string{"rclone:b2:my-bucket/path", "s3:s3.amazonaws.com/bucket", "sftp:user@host:/srv", "b2:bucket", "rest:https://host/"}
	for _, r := range remote {
		if !restic.IsRemoteRepo(r) {
			t.Errorf("expected %q to be remote", r)
		}
	}
	local := []string{"user/bombvault/container", "/host/user/x", "backups/flash", ""}
	for _, l := range local {
		if restic.IsRemoteRepo(l) {
			t.Errorf("expected %q to be local", l)
		}
	}
}

func TestRetentionPolicyAny(t *testing.T) {
	if (restic.RetentionPolicy{}).Any() {
		t.Fatal("empty policy must be inert")
	}
	if !(restic.RetentionPolicy{KeepWeekly: 1}).Any() {
		t.Fatal("a set dimension must make the policy active")
	}
}

func TestBackupArgs(t *testing.T) {
	got := restic.BackupArgs("/repo", []string{"-weird", "/p"}, []string{"container:plex"}, restic.Mode{Encrypted: true})
	want := []string{"-r", "/repo", "backup", "--json", "--tag", "container:plex", "--", "-weird", "/p"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestBackupArgsUnencrypted(t *testing.T) {
	got := restic.BackupArgs("/repo", []string{"/src"}, []string{"t"}, restic.Mode{Encrypted: false})
	want := []string{"-r", "/repo", "backup", "--insecure-no-password", "--json", "--tag", "t", "--", "/src"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestRestoreArgsEncrypted(t *testing.T) {
	got := restic.RestoreArgs("/repo", "abc123", "/target", restic.Mode{Encrypted: true})
	want := []string{"-r", "/repo", "restore", "--target", "/target", "--", "abc123"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestRestoreArgsNoPassword(t *testing.T) {
	got := restic.RestoreArgs("/repo", "abc123", "/", restic.Mode{Encrypted: false})
	want := []string{"-r", "/repo", "restore", "--insecure-no-password", "--target", "/", "--", "abc123"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestRestorePathArgs(t *testing.T) {
	t.Run("encrypted", func(t *testing.T) {
		got := restic.RestorePathArgs("/repo", "abc123", "/host/user/user/appdata/plex", restic.Mode{Encrypted: true})
		want := []string{"-r", "/repo", "restore", "--target", "/host/user/user/appdata/plex", "--", "abc123:/host/user/user/appdata/plex"}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("got %v want %v", got, want)
		}
	})
	t.Run("unencrypted", func(t *testing.T) {
		got := restic.RestorePathArgs("/repo", "abc123", "/p", restic.Mode{Encrypted: false})
		want := []string{"-r", "/repo", "restore", "--insecure-no-password", "--target", "/p", "--", "abc123:/p"}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("got %v want %v", got, want)
		}
	})
}

func TestSnapshotsArgs(t *testing.T) {
	t.Run("encrypted", func(t *testing.T) {
		got := restic.SnapshotsArgs("/repo", restic.Mode{Encrypted: true})
		want := []string{"-r", "/repo", "snapshots", "--json"}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("got %v want %v", got, want)
		}
	})
	t.Run("unencrypted", func(t *testing.T) {
		got := restic.SnapshotsArgs("/repo", restic.Mode{Encrypted: false})
		want := []string{"-r", "/repo", "snapshots", "--insecure-no-password", "--json"}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("got %v want %v", got, want)
		}
	})
}
