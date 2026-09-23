package api

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"strings"
	"syscall"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/backup"
	"github.com/junkerderprovinz/bombvault/internal/zfs"
)

const (
	zfsDomain = "zfs"
	// zfsDestroyBudget is how long a run keeps retrying a busy snapshot before
	// it leaves the stamp to the sweeper.
	zfsDestroyBudget = 8 * time.Second
	// zfsListTimeout and zfsSnapshotTimeout keep a hung SSH session from
	// holding the domain lock for the length of a run.
	zfsListTimeout     = 30 * time.Second
	zfsSnapshotTimeout = 60 * time.Second
	zfsPrimeTimeout    = 10 * time.Second
	// zfsMaxLegacyFilesystems is where a tree stops looking like a user's data
	// and starts looking like Docker's ZFS storage, whose layers a recursive
	// snapshot would pin for the whole run.
	zfsMaxLegacyFilesystems = 20
)

// errZFSHostMissing is what the domain answers while no host is wired.
var errZFSHostMissing = errors.New("no ZFS host is configured")

// SetZFSHost wires the machine that owns the pools. Without it every entry
// point of the domain refuses with ssh-missing.
func (s *Service) SetZFSHost(h zfs.Host) { s.zfs = h }

// The three ways the domain looks at the container's own filesystem. They are
// vars so a test can force an ELOOP and let a mount appear between two polls.
var (
	zfsStat         = os.Stat
	zfsMountRecords = readZFSMountRecords
	zfsDirEmpty     = snapshotDirEmpty
)

// zfsSnapshotPoll is how often the mount table is read while a snapshot
// automount propagates into the container, and zfsSnapshotWait bounds that
// wait: the mount is already made on the host by then, so anything longer is a
// propagation problem rather than a slow kernel. Both are vars so a test does
// not have to wait them out.
var (
	zfsSnapshotPoll = 250 * time.Millisecond
	zfsSnapshotWait = 5 * time.Second
)

// snapshotDirEmpty reports whether a member's snapshot holds nothing. restic
// refuses an empty source, and a dataset may legitimately hold nothing at the
// snapshot instant.
func snapshotDirEmpty(dir string) (bool, error) {
	f, err := os.Open(dir) //nolint:gosec // G304: dir is built from a validated dataset name and this run's snapshot stamp
	if err != nil {
		return false, err
	}
	defer f.Close() //nolint:errcheck // read-only handle
	names, err := f.Readdirnames(1)
	if errors.Is(err, io.EOF) {
		return true, nil
	}
	if err != nil {
		return false, err
	}
	return len(names) == 0, nil
}

func readZFSMountRecords() []zfs.MountRecord {
	f, err := os.Open(mountinfoPath) //nolint:gosec // G304: mountinfoPath is a fixed package var (/proc/self/mountinfo), overridden only by tests
	if err != nil {
		return nil
	}
	defer f.Close() //nolint:errcheck // read-only handle
	return zfs.ParseMountinfo(f)
}

// resolveDatasetMount finds where the container sees a whole dataset, and
// returns a member code when it sees none. A restore writes, so it is confined
// to the Host Data mapping; a backup may also read an identity mapping such as
// a TrueNAS /mnt/pool:/mnt/pool.
func (s *Service) resolveDatasetMount(recs []zfs.MountRecord, e zfs.ListEntry, forRestore bool) (zfs.MountRecord, string) {
	preferred, _ := s.toContainerPath(e.Mountpoint)
	if rec, ok := zfs.FindDatasetMount(recs, e.Name, preferred, s.cfg.HostMountRoot, !forRestore); ok {
		return rec, ""
	}
	if preferred != "" && zfs.ShfsOnly(recs, preferred) {
		return zfs.MountRecord{}, "shfs-only"
	}
	return zfs.MountRecord{}, "not-visible"
}

// visibleSnapshot waits until the snapshot itself, not the empty control
// directory above it, is what restic would read at snapDir.
func (s *Service) visibleSnapshot(ctx context.Context, dataset, snap, snapDir string) error {
	cpath := strings.TrimSuffix(snapDir, "/.zfs/snapshot/"+snap)
	if _, err := zfsStat(snapDir + "/."); err != nil && !errors.Is(err, fs.ErrNotExist) {
		if errors.Is(err, syscall.ELOOP) {
			// The Host Data mapping does not pass new mounts on, so the kernel
			// walks the automount trigger instead of the snapshot.
			return &backup.ZFSRefusal{Code: "snapshot-loop", Detail: dataset}
		}
		return &backup.ZFSRefusal{Code: "snapshot-not-visible", Detail: dataset + ": " + err.Error()}
	}
	deadline := time.Now().Add(zfsSnapshotWait)
	for {
		if zfs.SnapshotMounted(zfsMountRecords(), dataset, snap, cpath) {
			return nil
		}
		if time.Now().After(deadline) {
			return &backup.ZFSRefusal{Code: "snapshot-not-visible", Detail: dataset}
		}
		select {
		case <-ctx.Done():
			return &backup.ZFSRefusal{Code: "snapshot-not-visible", Detail: dataset}
		case <-time.After(zfsSnapshotPoll):
		}
	}
}
