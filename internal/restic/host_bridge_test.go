package restic_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/restic"
)

const ownHost = "bv-0123456789ab"

// hostSwitchRepo is a fresh repository and a source folder of three files.
func hostSwitchRepo(t *testing.T) (repo, src string) {
	t.Helper()
	if _, err := exec.LookPath("restic"); err != nil {
		t.Skip("no restic")
	}
	dir := resolved(t, t.TempDir())
	repo, src = filepath.Join(dir, "repo"), filepath.Join(dir, "src")
	if err := os.MkdirAll(src, 0o750); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"a.txt", "b.txt", "c.txt"} {
		if err := os.WriteFile(filepath.Join(src, name), []byte(name), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := (restic.Restic{Bin: "restic"}).Init(context.Background(), repo, restic.Mode{}); err != nil {
		t.Fatal(err)
	}
	return repo, src
}

// checkHostSwitch holds three runs of one item, before and after the switch to
// the server's own host, against what restic recorded, and then retention
// across both hosts.
func checkHostSwitch(t *testing.T, repo, tag string, backup func(restic.Mode) restic.Summary) {
	t.Helper()
	r, ctx := restic.Restic{Bin: "restic"}, context.Background()
	legacy := backup(restic.Mode{})
	first := backup(restic.Mode{Host: ownHost})
	if first.FilesNew != 0 || first.FilesUnmodified != 3 {
		t.Fatalf("the first run under the new host read %d new and %d unmodified files, want all 3 unmodified", first.FilesNew, first.FilesUnmodified)
	}
	if parent, err := r.SnapshotParent(ctx, repo, first.SnapshotID, restic.Mode{}); err != nil || parent != legacy.SnapshotID {
		t.Fatalf("the first run under the new host built on %q (%v), want the legacy snapshot %s", parent, err, legacy.SnapshotID)
	}
	second := backup(restic.Mode{Host: ownHost})
	if parent, err := r.SnapshotParent(ctx, repo, second.SnapshotID, restic.Mode{}); err != nil || parent != first.SnapshotID {
		t.Fatalf("the second run built on %q (%v), want the first run under its own host %s", parent, err, first.SnapshotID)
	}

	snaps, err := r.Snapshots(ctx, repo, restic.Mode{})
	if err != nil {
		t.Fatal(err)
	}
	hosts := map[string]int{}
	for _, sn := range snaps {
		hosts[sn.Hostname]++
	}
	if hosts[restic.LegacyHost] != 1 || hosts[ownHost] != 2 {
		t.Fatalf("hosts = %v, want one snapshot under %s and two under %s", hosts, restic.LegacyHost, ownHost)
	}
	if err := r.ForgetPolicy(ctx, repo, restic.RetentionPolicy{KeepLast: 1}, restic.Mode{}, []string{tag}, false); err != nil {
		t.Fatal(err)
	}
	snaps, err = r.Snapshots(ctx, repo, restic.Mode{})
	if err != nil {
		t.Fatal(err)
	}
	if len(snaps) != 1 || snaps[0].ID != second.SnapshotID {
		t.Fatalf("keep-last 1 left %v, want only the newest snapshot across both hosts", snaps)
	}
}

func TestADirectoryBackupKeepsItsParentAcrossTheSwitchToItsOwnHost(t *testing.T) {
	repo, src := hostSwitchRepo(t)
	checkHostSwitch(t, repo, "zfs:tank/data", func(m restic.Mode) restic.Summary {
		sum, err := restic.Restic{Bin: "restic"}.BackupDir(context.Background(), repo, src, []string{"zfs:tank/data"}, m)
		if err != nil {
			t.Fatal(err)
		}
		return sum
	})
}

func TestAPathBackupKeepsItsParentAcrossTheSwitchToItsOwnHost(t *testing.T) {
	repo, src := hostSwitchRepo(t)
	checkHostSwitch(t, repo, "container:plex", func(m restic.Mode) restic.Summary {
		sum, err := restic.Restic{Bin: "restic"}.Backup(context.Background(), repo, []string{src}, []string{"container:plex", "p1"}, m)
		if err != nil {
			t.Fatal(err)
		}
		return sum
	})
}
