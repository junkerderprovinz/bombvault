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
