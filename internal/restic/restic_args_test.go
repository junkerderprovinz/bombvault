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

func TestCatConfigArgs(t *testing.T) {
	t.Run("encrypted", func(t *testing.T) {
		got := restic.CatConfigArgs("/repo", restic.Mode{Encrypted: true, Password: "pw"})
		want := []string{"-r", "/repo", "cat", "config"}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("got %v want %v", got, want)
		}
	})
	t.Run("unencrypted adds insecure flag", func(t *testing.T) {
		got := restic.CatConfigArgs("/repo", restic.Mode{Encrypted: false})
		want := []string{"-r", "/repo", "cat", "config", "--insecure-no-password"}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("got %v want %v", got, want)
		}
	})
}

func TestForgetPolicyArgs(t *testing.T) {
	t.Run("emits only set dimensions + prune", func(t *testing.T) {
		got := restic.ForgetPolicyArgs("/repo",
			restic.RetentionPolicy{KeepLast: 5, KeepMonthly: 6}, restic.Mode{Encrypted: true})
		want := []string{"-r", "/repo", "forget", "--group-by", "paths", "--keep-last", "5", "--keep-monthly", "6", "--prune"}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("got %v want %v", got, want)
		}
	})
	t.Run("unencrypted adds insecure flag, full policy", func(t *testing.T) {
		got := restic.ForgetPolicyArgs("/repo",
			restic.RetentionPolicy{KeepLast: 3, KeepDaily: 7, KeepWeekly: 4, KeepMonthly: 12},
			restic.Mode{Encrypted: false})
		want := []string{"-r", "/repo", "forget", "--insecure-no-password", "--group-by", "paths",
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

func TestCheckDataArgs(t *testing.T) {
	t.Run("encrypted builds read-data-subset", func(t *testing.T) {
		got := restic.CheckDataArgs("/repo", 5, restic.Mode{Encrypted: true})
		want := []string{"-r", "/repo", "check", "--read-data-subset=5%"}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("got %v want %v", got, want)
		}
	})
	t.Run("unencrypted adds insecure flag", func(t *testing.T) {
		got := restic.CheckDataArgs("/repo", 10, restic.Mode{Encrypted: false})
		want := []string{"-r", "/repo", "check", "--read-data-subset=10%", "--insecure-no-password"}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("got %v want %v", got, want)
		}
	})
	t.Run("percent clamps below 1 up to 1", func(t *testing.T) {
		got := restic.CheckDataArgs("/repo", 0, restic.Mode{Encrypted: true})
		want := []string{"-r", "/repo", "check", "--read-data-subset=1%"}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("got %v want %v", got, want)
		}
	})
	t.Run("percent clamps above 100 down to 100", func(t *testing.T) {
		got := restic.CheckDataArgs("/repo", 250, restic.Mode{Encrypted: true})
		want := []string{"-r", "/repo", "check", "--read-data-subset=100%"}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("got %v want %v", got, want)
		}
	})
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

func TestLooksLikeUnprefixedRemote(t *testing.T) {
	// A scheme-like "word:" prefix that is NOT a recognized remote — the common
	// mistake of omitting the rclone: prefix.
	unprefixed := []string{"BackBlaze:BombVault-test", "MyRemote:bucket", "gdrive:folder/sub"}
	for _, u := range unprefixed {
		if !restic.LooksLikeUnprefixedRemote(u) {
			t.Errorf("expected %q to look like an unprefixed remote", u)
		}
	}
	// Recognized remotes must NOT be flagged (they carry a valid prefix)...
	// ...nor must plain local paths (no scheme).
	ok := []string{
		"rclone:BackBlaze:BombVault-test", "s3:s3.amazonaws.com/bucket", "b2:bucket", "sftp:user@host:/srv",
		"user/bombvault/container", "/host/user/x", "backups/flash", "",
	}
	for _, o := range ok {
		if restic.LooksLikeUnprefixedRemote(o) {
			t.Errorf("expected %q NOT to be flagged as an unprefixed remote", o)
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
	want := []string{"-r", "/repo", "backup", "--json", "--host", "bombvault", "--tag", "container:plex", "--", "-weird", "/p"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestBackupArgsUnencrypted(t *testing.T) {
	got := restic.BackupArgs("/repo", []string{"/src"}, []string{"t"}, restic.Mode{Encrypted: false})
	want := []string{"-r", "/repo", "backup", "--insecure-no-password", "--json", "--host", "bombvault", "--tag", "t", "--", "/src"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestDumpZipArgsEncrypted(t *testing.T) {
	got := restic.DumpZipArgs("/repo", "abc123", "/host/boot", restic.Mode{Encrypted: true})
	want := []string{"-r", "/repo", "dump", "-a", "zip", "--", "abc123:/host/boot", "/"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestDumpZipArgsNoPassword(t *testing.T) {
	got := restic.DumpZipArgs("/repo", "abc123", "/host/boot", restic.Mode{Encrypted: false})
	want := []string{"-r", "/repo", "dump", "-a", "zip", "--insecure-no-password", "--", "abc123:/host/boot", "/"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestCopyArgsEncrypted(t *testing.T) {
	got := restic.CopyArgs("/dest", "/src", nil, restic.Limits{}, restic.Mode{Encrypted: true})
	want := []string{"-r", "/dest", "copy", "--from-repo", "/src"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestCopyArgsNoPasswordWithIDs(t *testing.T) {
	got := restic.CopyArgs("/dest", "/src", []string{"abc123"}, restic.Limits{}, restic.Mode{Encrypted: false})
	want := []string{"-r", "/dest", "copy", "--from-repo", "/src", "--insecure-no-password", "--from-insecure-no-password", "--", "abc123"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestCopyArgsLimits(t *testing.T) {
	t.Run("both limits set prepend global flags before the subcommand", func(t *testing.T) {
		got := restic.CopyArgs("/dest", "/src", nil, restic.Limits{UploadKBps: 1024, DownloadKBps: 512}, restic.Mode{Encrypted: true})
		want := []string{"-r", "/dest", "--limit-upload", "1024", "--limit-download", "512", "copy", "--from-repo", "/src"}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("got %v want %v", got, want)
		}
	})
	t.Run("upload only", func(t *testing.T) {
		got := restic.CopyArgs("/dest", "/src", nil, restic.Limits{UploadKBps: 2048}, restic.Mode{Encrypted: true})
		want := []string{"-r", "/dest", "--limit-upload", "2048", "copy", "--from-repo", "/src"}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("got %v want %v", got, want)
		}
	})
	t.Run("zero limits omit the flags", func(t *testing.T) {
		got := restic.CopyArgs("/dest", "/src", nil, restic.Limits{}, restic.Mode{Encrypted: true})
		want := []string{"-r", "/dest", "copy", "--from-repo", "/src"}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("got %v want %v", got, want)
		}
	})
}

func TestRestorePathArgs(t *testing.T) {
	t.Run("encrypted", func(t *testing.T) {
		got := restic.RestorePathArgs("/repo", "abc123", "/host/user/user/appdata/plex", restic.Mode{Encrypted: true})
		want := []string{"-r", "/repo", "restore", "--json", "--target", "/host/user/user/appdata/plex", "--", "abc123:/host/user/user/appdata/plex"}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("got %v want %v", got, want)
		}
	})
	t.Run("unencrypted", func(t *testing.T) {
		got := restic.RestorePathArgs("/repo", "abc123", "/p", restic.Mode{Encrypted: false})
		want := []string{"-r", "/repo", "restore", "--insecure-no-password", "--json", "--target", "/p", "--", "abc123:/p"}
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

func TestDiffArgs(t *testing.T) {
	t.Run("encrypted", func(t *testing.T) {
		got := restic.DiffArgs("/repo", "aaaa1111", "bbbb2222", restic.Mode{Encrypted: true})
		want := []string{"-r", "/repo", "diff", "--json", "--", "aaaa1111", "bbbb2222"}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("got %v want %v", got, want)
		}
	})
	t.Run("unencrypted adds insecure flag", func(t *testing.T) {
		got := restic.DiffArgs("/repo", "aaaa1111", "bbbb2222", restic.Mode{Encrypted: false})
		want := []string{"-r", "/repo", "diff", "--json", "--insecure-no-password", "--", "aaaa1111", "bbbb2222"}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("got %v want %v", got, want)
		}
	})
}

func TestTagAddArgs(t *testing.T) {
	t.Run("encrypted, one tag per --add, id after --", func(t *testing.T) {
		got := restic.TagAddArgs("/repo", "aaaa1111", []string{"keep", "milestone"}, restic.Mode{Encrypted: true})
		want := []string{"-r", "/repo", "tag", "--add", "keep", "--add", "milestone", "--", "aaaa1111"}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("got %v want %v", got, want)
		}
	})
	t.Run("unencrypted adds insecure flag", func(t *testing.T) {
		got := restic.TagAddArgs("/repo", "aaaa1111", []string{"keep"}, restic.Mode{Encrypted: false})
		want := []string{"-r", "/repo", "tag", "--insecure-no-password", "--add", "keep", "--", "aaaa1111"}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("got %v want %v", got, want)
		}
	})
}

func TestStatsArgs(t *testing.T) {
	t.Run("encrypted raw-data", func(t *testing.T) {
		got := restic.StatsArgs("/repo", "raw-data", restic.Mode{Encrypted: true})
		want := []string{"-r", "/repo", "stats", "--json", "--mode", "raw-data"}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("got %v want %v", got, want)
		}
	})
	t.Run("unencrypted restore-size adds insecure flag", func(t *testing.T) {
		got := restic.StatsArgs("/repo", "restore-size", restic.Mode{Encrypted: false})
		want := []string{"-r", "/repo", "stats", "--json", "--mode", "restore-size", "--insecure-no-password"}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("got %v want %v", got, want)
		}
	})
}

func TestStatsRestoreSizeArgs(t *testing.T) {
	t.Run("encrypted", func(t *testing.T) {
		got := restic.StatsRestoreSizeArgs("/repo", "deadbeef", restic.Mode{Encrypted: true})
		// Per-snapshot restore-size stats: `stats --mode restore-size --json <snap>`.
		// The snapshot id goes after -- (arg-injection guard, house pattern).
		want := []string{"-r", "/repo", "stats", "--mode", "restore-size", "--json", "--", "deadbeef"}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("got %v want %v", got, want)
		}
	})
	t.Run("unencrypted adds insecure flag", func(t *testing.T) {
		got := restic.StatsRestoreSizeArgs("/repo", "deadbeef", restic.Mode{Encrypted: false})
		want := []string{"-r", "/repo", "stats", "--mode", "restore-size", "--json", "--insecure-no-password", "--", "deadbeef"}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("got %v want %v", got, want)
		}
	})
}
