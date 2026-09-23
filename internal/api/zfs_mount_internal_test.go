package api

import (
	"context"
	"errors"
	"io/fs"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/backup"
	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/zfs"
)

// mountinfo lines are taken from a Docker container on an Unraid host with the
// shipped Host Data mapping, with the pool names replaced.
const zfsTestMountinfo = `
30 1 0:28 / / rw,relatime - overlay overlay rw
40 30 0:29 /mnt /host/user rw,relatime master:5 - rootfs rootfs rw
41 40 0:31 / /host/user/cache rw,relatime master:6 - zfs cache rw,xattr,posixacl
42 41 0:32 / /host/user/cache/appdata rw,relatime master:7 - zfs cache/appdata rw,xattr,posixacl
43 40 0:33 / /host/user/user rw,relatime master:8 - fuse.shfs shfs rw
44 43 0:34 / /host/user/user/appdata rw,relatime master:9 - zfs cache/appdata rw,xattr,posixacl
`

func zfsTestRecords(t *testing.T, text string) []zfs.MountRecord {
	t.Helper()
	return zfs.ParseMountinfo(strings.NewReader(strings.TrimSpace(text)))
}

func TestResolveDatasetMountFromMountinfo(t *testing.T) {
	s := &Service{cfg: config.Config{HostMountRoot: "/host/user", HostSourceRoot: "/mnt"}}
	recs := zfsTestRecords(t, zfsTestMountinfo)

	cases := []struct {
		name      string
		entry     zfs.ListEntry
		wantPath  string
		wantCode  string
		forRestor bool
	}{
		{
			name:     "translated pool path wins over the exclusive share bind",
			entry:    zfs.ListEntry{Name: "cache/appdata", Mountpoint: "/mnt/cache/appdata"},
			wantPath: "/host/user/cache/appdata",
		},
		{
			name:     "a dataset reachable only through shfs",
			entry:    zfs.ListEntry{Name: "cache/media", Mountpoint: "/mnt/user/media"},
			wantCode: "shfs-only",
		},
		{
			name:     "a dataset with no record at all",
			entry:    zfs.ListEntry{Name: "cache/gone", Mountpoint: "/mnt/cache/gone"},
			wantCode: "not-visible",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec, code := s.resolveDatasetMount(recs, tc.entry, tc.forRestor)
			if code != tc.wantCode {
				t.Fatalf("code = %q, want %q", code, tc.wantCode)
			}
			if rec.MountPoint != tc.wantPath {
				t.Fatalf("mount point = %q, want %q", rec.MountPoint, tc.wantPath)
			}
		})
	}
}

func TestResolveDatasetMountIdentityRootOnlyForBackups(t *testing.T) {
	// A TrueNAS app maps /mnt/pool to /mnt/pool, so the dataset is visible but
	// not below the host mount root.
	const identity = `
30 1 0:28 / / rw,relatime - overlay overlay rw
50 30 0:40 / /mnt/pool/apps rw,relatime master:5 - zfs pool/apps rw,xattr
`
	s := &Service{cfg: config.Config{HostMountRoot: "/host/user", HostSourceRoot: "/mnt"}}
	recs := zfsTestRecords(t, identity)
	entry := zfs.ListEntry{Name: "pool/apps", Mountpoint: "/mnt/pool/apps"}

	rec, code := s.resolveDatasetMount(recs, entry, false)
	if code != "" || rec.MountPoint != "/mnt/pool/apps" {
		t.Fatalf("backup got (%q, %q), want the identity mapping", rec.MountPoint, code)
	}
	if _, code := s.resolveDatasetMount(recs, entry, true); code != "not-visible" {
		t.Fatalf("restore code = %q, want not-visible", code)
	}
}

func TestVisibleSnapshotMapsELOOP(t *testing.T) {
	s := &Service{}
	restoreStat := zfsStat
	zfsStat = func(name string) (fs.FileInfo, error) {
		return nil, &fs.PathError{Op: "stat", Path: name, Err: syscall.ELOOP}
	}
	t.Cleanup(func() { zfsStat = restoreStat })

	err := s.visibleSnapshot(context.Background(), "cache/appdata", "bombvault-20260101000000",
		"/host/user/appdata/.zfs/snapshot/bombvault-20260101000000")
	var refusal *backup.ZFSRefusal
	if !errors.As(err, &refusal) || refusal.Code != "snapshot-loop" {
		t.Fatalf("err = %v, want a snapshot-loop refusal", err)
	}
}

func TestVisibleSnapshotWaitsForPropagation(t *testing.T) {
	const snap = "bombvault-20260101000000"
	const cpath = "/host/user/appdata"
	mounted := zfsTestRecords(t, `
60 40 0:50 / /host/user/appdata/.zfs/snapshot/`+snap+` ro,relatime master:6 - zfs cache/appdata@`+snap+` ro,xattr
`)

	restoreStat, restoreRecords, restorePoll := zfsStat, zfsMountRecords, zfsSnapshotPoll
	t.Cleanup(func() { zfsStat, zfsMountRecords, zfsSnapshotPoll = restoreStat, restoreRecords, restorePoll })
	zfsStat = func(string) (fs.FileInfo, error) { return nil, nil }
	zfsSnapshotPoll = time.Millisecond
	reads := 0
	zfsMountRecords = func() []zfs.MountRecord {
		reads++
		if reads < 3 {
			return nil
		}
		return mounted
	}

	s := &Service{}
	if err := s.visibleSnapshot(context.Background(), "cache/appdata", snap, cpath+"/.zfs/snapshot/"+snap); err != nil {
		t.Fatalf("visibleSnapshot: %v", err)
	}
	if reads < 3 {
		t.Fatalf("mount table read %d times, want the poll to wait for the record", reads)
	}
}
