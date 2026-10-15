package backup

import (
	"context"
	"fmt"
	"io"
	"log"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"
)

// ZFSHost runs the zfs and device commands of a zvol backup and restore on
// the host. The service layer implements it over SSH, so the container needs
// no zfs tools of its own.
type ZFSHost interface {
	SnapshotCreate(ctx context.Context, dataset, snapName string) error
	// Snapshots lists the snapshots of dataset with the clones made of each.
	Snapshots(ctx context.Context, dataset string) ([]ZFSSnapshot, error)
	// Clone makes a read-only zvol clone of snapshot named clone.
	Clone(ctx context.Context, snapshot, clone string) error
	// Destroy destroys a zvol, a clone or a snapshot by its full name.
	Destroy(ctx context.Context, name string) error
	// VolSize reports a zvol's size in bytes, and false when there is no such
	// zvol.
	VolSize(ctx context.Context, dataset string) (int64, bool, error)
	CreateVolume(ctx context.Context, dataset string, size int64) error
	SetVolSize(ctx context.Context, dataset string, size int64) error
	// ReadVolume streams the raw contents of a zvol. The caller calls wait
	// once when it is done reading; it reports how the read ended.
	ReadVolume(ctx context.Context, dataset string) (io.ReadCloser, func() error, error)
	// WriteVolume writes rd onto a zvol from its first byte.
	WriteVolume(ctx context.Context, rd io.Reader, dataset string) error
}

// ZFSSnapshot is one snapshot of a dataset, by full name, with the full names
// of the datasets cloned from it.
type ZFSSnapshot struct {
	Name   string
	Clones []string
}

// ZvolRestic is the part of restic the zvol disks need: a backup read from a
// stream and a dump written to one, since a block device has no file for
// restic to read or restore.
type ZvolRestic interface {
	// BackupStdin backs up all of rd as one file recorded under path.
	BackupStdin(ctx context.Context, repo string, rd io.Reader, path string, tags []string) (Summary, error)
	// DumpTo writes the file at path in snapshotID to w.
	DumpTo(ctx context.Context, repo, snapshotID, path string, w io.Writer) error
}

// ZvolStdinPath is the path a zvol's image has in its snapshot. It stays the
// same from one backup to the next, so restic compares each with the last.
func ZvolStdinPath(dataset string) string {
	return "/vm-disks/" + dataset + ".img"
}

// BackupZvolDeps is what BackupZvolDisk needs to back up one zvol.
type BackupZvolDeps struct {
	Dataset  string
	SnapName string
	RepoPath string
	Tags     []string

	Host   ZFSHost
	Restic ZvolRestic
}

// BackupZvolDisk stores the raw image of one zvol, read from a clone of a
// fresh snapshot. It does not use zfs send: every record of a send stream
// carries the snapshot's guid and a running checksum, so restic would find
// nothing of the last backup to reuse and store the whole disk again. The
// clone and the snapshot are destroyed whether or not the backup worked.
func BackupZvolDisk(ctx context.Context, d BackupZvolDeps) (Summary, error) {
	snapshot := d.Dataset + "@" + d.SnapName
	readingZvols.Store(snapshot, struct{}{})
	defer readingZvols.Delete(snapshot)
	sweepZvolLeftovers(ctx, d.Host, d.Dataset)

	if err := d.Host.SnapshotCreate(ctx, d.Dataset, d.SnapName); err != nil {
		return Summary{}, fmt.Errorf("zvol backup: snapshot: %w", err)
	}
	// A cancelled backup still has to take its snapshot and clone away.
	cleanup := context.WithoutCancel(ctx)
	defer destroyOrLog(cleanup, d.Host, snapshot)

	clone := d.Dataset + "-" + d.SnapName
	if err := d.Host.Clone(ctx, snapshot, clone); err != nil {
		return Summary{}, fmt.Errorf("zvol backup: clone the snapshot: %w", err)
	}
	defer destroyOrLog(cleanup, d.Host, clone)

	stream, wait, err := d.Host.ReadVolume(ctx, clone)
	if err != nil {
		return Summary{}, fmt.Errorf("zvol backup: start reading: %w", err)
	}
	defer func() { _ = stream.Close() }()

	sum, backupErr := d.Restic.BackupStdin(ctx, d.RepoPath, stream, ZvolStdinPath(d.Dataset), d.Tags)
	// A read that died halfway closes the pipe like a finished one, and restic
	// would store the short image as a good backup.
	if waitErr := wait(); waitErr != nil {
		if backupErr != nil {
			return Summary{}, fmt.Errorf("zvol backup: restic: %w (the read failed too: %v)", backupErr, waitErr)
		}
		return Summary{}, fmt.Errorf("zvol backup: read: %w", waitErr)
	}
	if backupErr != nil {
		return Summary{}, fmt.Errorf("zvol backup: restic: %w", backupErr)
	}
	return sum, nil
}

// destroyAttempts and destroyRetryDelay space the tries at destroying a clone
// that udev may still hold for a moment after it was read.
var (
	destroyAttempts   = 3
	destroyRetryDelay = time.Second
)

// destroyOrLog destroys name and only logs a failure: the backup itself is
// already in the repository, and a leftover snapshot or clone loses no data.
// It reports whether name is gone.
func destroyOrLog(ctx context.Context, host ZFSHost, name string) bool {
	var err error
	for attempt := range destroyAttempts {
		if attempt > 0 {
			time.Sleep(destroyRetryDelay)
		}
		if err = host.Destroy(ctx, name); err == nil {
			return true
		}
	}
	log.Printf("zvol backup: could not destroy %s, remove it by hand with zfs destroy: %v", name, err)
	return false
}

// zvolSnapshotName names the snapshot a backup taken at now reads from. ZFS
// snapshot names may not contain ':', so the time is plain digits. The ZFS
// datasets take bombvault-<time> snapshots, recursive ones that can reach a
// zvol too, so the VM backup's carry a prefix of their own and neither sweep
// can take the other's.
func zvolSnapshotName(now time.Time) string {
	return "bombvault-vm-" + now.UTC().Format("20060102150405")
}

// zvolSnapshotRe matches the snapshot names zvolSnapshotName gives, and
// nothing else, such as a safety snapshot taken before a restore.
var zvolSnapshotRe = regexp.MustCompile(`^bombvault-vm-[0-9]{14}$`)

// readingZvols holds the full snapshot names of the zvol backups running in
// this process, which a sweep leaves alone.
var readingZvols sync.Map

// sweepZvolLeftovers destroys the snapshot and clone a backup of dataset left
// behind when the process stopped halfway through it. It takes only names
// BombVault gives, and keeps a snapshot something else was cloned from.
func sweepZvolLeftovers(ctx context.Context, host ZFSHost, dataset string) {
	snaps, err := host.Snapshots(ctx, dataset)
	if err != nil {
		log.Printf("zvol backup: could not look for snapshots an earlier backup left on %s; remove any %s@bombvault-vm-<time> and its clone by hand: %v", dataset, dataset, err)
		return
	}
	cleanup := context.WithoutCancel(ctx)
	for _, snap := range snaps {
		name, ok := strings.CutPrefix(snap.Name, dataset+"@")
		if !ok || !zvolSnapshotRe.MatchString(name) {
			continue
		}
		if _, running := readingZvols.Load(snap.Name); running {
			continue
		}
		ours := dataset + "-" + name
		if foreign := slices.DeleteFunc(slices.Clone(snap.Clones), func(c string) bool { return c == ours }); len(foreign) > 0 {
			log.Printf("zvol backup: %s is left from an earlier backup but kept, since %s was cloned from it", snap.Name, strings.Join(foreign, ", "))
			continue
		}
		if len(snap.Clones) > 0 {
			if !destroyOrLog(cleanup, host, ours) {
				continue
			}
			log.Printf("zvol backup: removed the clone %s left by an earlier backup that did not finish", ours)
		}
		if destroyOrLog(cleanup, host, snap.Name) {
			log.Printf("zvol backup: removed %s left by an earlier backup that did not finish", snap.Name)
		}
	}
}

// backupBlockDisksAndLog backs up every zvol of d. All of them share one
// snapshot name, taken once, so the disks of a run are read at the same
// moment as far as the host allows. A failed disk does not stop the others,
// and the first failure fails the run.
func backupBlockDisksAndLog(ctx context.Context, d VMBackupDeps, logPrefix string) (Summary, error) {
	if len(d.BlockDisks) == 0 {
		return Summary{}, nil
	}
	snapName := zvolSnapshotName(time.Now())
	var firstErr error
	total := Summary{Measured: true}
	for _, bd := range d.BlockDisks {
		identity := "vm:" + d.Name
		if bd.Dev != "" {
			identity = "vm:" + d.Name + ":zvol:" + bd.Dev
		}
		tags := withRunTag([]string{identity, "p2"}, d.RunTag)
		sum, err := BackupZvolDisk(ctx, BackupZvolDeps{
			Dataset:  bd.Dataset,
			SnapName: snapName,
			RepoPath: d.RepoPath,
			Tags:     tags,
			Host:     d.ZFSHost,
			Restic:   d.ZvolRestic,
		})
		if err != nil {
			log.Printf("%s: zvol disk %q: %v", logPrefix, bd.Dataset, err)
			if firstErr == nil {
				firstErr = fmt.Errorf("%s: zvol disk %q: %w", logPrefix, bd.Dataset, err)
			}
			continue
		}
		total = total.Plus(sum)
		log.Printf("%s: zvol disk %q: backed up as restic snapshot %s (%d bytes added), tags %v", logPrefix, bd.Dataset, sum.SnapshotID, sum.Bytes, tags)
	}
	return total, firstErr
}

// RestoreZvolDeps is what RestoreZvolDisk needs to restore one zvol.
type RestoreZvolDeps struct {
	// Dataset is the zvol written to: the one backed up, or its counterpart
	// on another pool for a restore onto another instance.
	Dataset string
	// Size is the zvol's size when it was backed up, which is the length of
	// its image.
	Size       int64
	RepoPath   string
	SnapshotID string
	StdinPath  string

	Host   ZFSHost
	Restic ZvolRestic
}

// RestoreZvolDisk writes a zvol's image back onto the zvol, as a file disk
// is restored onto its file, so the redefined VM boots the restored data. An
// existing zvol is first snapshotted, and the returned snapshot name is how
// the user rolls the restore back; a missing one is created. The size goes
// back to what it was at the backup.
func RestoreZvolDisk(ctx context.Context, d RestoreZvolDeps) (string, error) {
	size, exists, err := d.Host.VolSize(ctx, d.Dataset)
	if err != nil {
		return "", fmt.Errorf("zvol restore: read the size of %s: %w", d.Dataset, err)
	}
	var safety string
	if exists {
		snapName := "bombvault-before-restore-" + time.Now().UTC().Format("20060102150405")
		if err := d.Host.SnapshotCreate(ctx, d.Dataset, snapName); err != nil {
			return "", fmt.Errorf("zvol restore: snapshot %s before writing to it: %w", d.Dataset, err)
		}
		safety = d.Dataset + "@" + snapName
		if size != d.Size {
			if err := d.Host.SetVolSize(ctx, d.Dataset, d.Size); err != nil {
				return safety, fmt.Errorf("zvol restore: resize %s to %d bytes: %w", d.Dataset, d.Size, err)
			}
		}
	} else if err := d.Host.CreateVolume(ctx, d.Dataset, d.Size); err != nil {
		return "", fmt.Errorf("zvol restore: create %s: %w", d.Dataset, err)
	}

	// The pipe carries the dump straight onto the device, so a disk of any
	// size never sits in memory.
	pr, pw := io.Pipe()
	dumpDone := make(chan error, 1)
	go func() {
		dumpErr := d.Restic.DumpTo(ctx, d.RepoPath, d.SnapshotID, d.StdinPath, pw)
		_ = pw.CloseWithError(dumpErr)
		dumpDone <- dumpErr
	}()
	writeErr := d.Host.WriteVolume(ctx, pr, d.Dataset)
	// A write that gave up early leaves the dump blocked on the pipe.
	_ = pr.CloseWithError(writeErr)
	dumpErr := <-dumpDone

	if dumpErr != nil {
		return safety, fmt.Errorf("zvol restore: restic dump: %w", dumpErr)
	}
	if writeErr != nil {
		return safety, fmt.Errorf("zvol restore: write %s: %w", d.Dataset, writeErr)
	}
	return safety, nil
}

// restoreBlockDisksAndLog restores every zvol of d. A failed disk does not
// stop the others, and the first failure stops the restore before the VM is
// defined.
func restoreBlockDisksAndLog(ctx context.Context, d VMRestoreDeps) error {
	var firstErr error
	for _, bd := range d.BlockDisks {
		safety, err := RestoreZvolDisk(ctx, RestoreZvolDeps{
			Dataset:    bd.Dataset,
			Size:       bd.Size,
			RepoPath:   d.RepoPath,
			SnapshotID: bd.SnapshotID,
			StdinPath:  bd.StdinPath,
			Host:       d.ZFSHost,
			Restic:     d.ZvolRestic,
		})
		if safety != "" {
			log.Printf("vm restore: zvol disk %q: its contents from before the restore are kept in %s; zfs rollback %s undoes the restore", bd.Dataset, safety, safety)
		}
		if err != nil {
			log.Printf("vm restore: zvol disk %q: %v", bd.Dataset, err)
			if firstErr == nil {
				firstErr = fmt.Errorf("vm restore: zvol disk %q: %w", bd.Dataset, err)
			}
			continue
		}
		log.Printf("vm restore: zvol disk %q: restored from restic snapshot %s", bd.Dataset, bd.SnapshotID)
	}
	return firstErr
}
