// On TrueNAS SCALE 25.10 libvirt keeps <source dev='/dev/zvol/<pool>/<dataset>'/>
// as written rather than resolving it to /dev/zdN, so ZvolDatasetFromDevPath
// sees the path it parses. zfs snapshot works on a zvol that a running VM is
// using, a clone of that snapshot shows up as a device to read, and the raw
// image survives restic backup --stdin and restic dump byte for byte.
//
// TrueNAS needs LIBVIRT_URI to name its own socket,
// /run/truenas_libvirt/libvirt-sock; see docs/vm-backup-ssh-setup.md.

package virshcli

import (
	"strconv"
	"strings"
	"time"
)

// zvolDevPrefix starts the device node of a zvol: /dev/zvol/<pool>/<dataset>.
const zvolDevPrefix = "/dev/zvol/"

// hasUnsafeZFSNameChars reports whether s contains "..", whitespace or a
// quote, none of which belong in a zfs argv.
func hasUnsafeZFSNameChars(s string) bool {
	return strings.Contains(s, "..") || strings.ContainsAny(s, " \t\n\r'\"")
}

// ZvolDatasetFromDevPath returns the "<pool>/<dataset>" behind a zvol device
// path of the form /dev/zvol/<pool>/<dataset>. Anything else (a file, another
// block device, a bare pool, or a name hasUnsafeZFSNameChars rejects) gives
// ok == false, and such a disk cannot go through the zvol backup; the caller
// must not guess a dataset for it.
func ZvolDatasetFromDevPath(devPath string) (string, bool) {
	if !strings.HasPrefix(devPath, zvolDevPrefix) {
		return "", false
	}
	rest := strings.TrimPrefix(devPath, zvolDevPrefix)
	rest = strings.Trim(rest, "/")
	if rest == "" {
		return "", false
	}
	if !strings.Contains(rest, "/") {
		return "", false // pool only, no dataset
	}
	if hasUnsafeZFSNameChars(rest) {
		return "", false
	}
	return rest, true
}

// RebaseZvolDatasetPool replaces the pool of dataset with destPool and keeps
// the rest, so "tank/vms/win10/disk0" onto "flashpool" becomes
// "flashpool/vms/win10/disk0". A cross-instance restore needs it because the
// source pool rarely exists on the destination.
//
// destPool must be one pool name: not empty, no "/", nothing
// hasUnsafeZFSNameChars rejects, and no leading '-', '@', '#' or '%'. On
// ok == false the caller must refuse the restore rather than fall back to the
// unrebased dataset, which would land it in the wrong pool.
func RebaseZvolDatasetPool(dataset, destPool string) (string, bool) {
	destPool = strings.TrimSpace(destPool)
	if destPool == "" {
		return "", false
	}
	if strings.Contains(destPool, "/") || hasUnsafeZFSNameChars(destPool) {
		return "", false
	}
	if strings.IndexByte("-@#%", destPool[0]) >= 0 {
		// No pool name starts with these. A leading '-' would also reach zfs
		// receive as an option, so rejecting it here gives a clearer error.
		return "", false
	}
	_, rest, ok := strings.Cut(dataset, "/")
	if !ok || rest == "" {
		return "", false // no dataset below the pool
	}
	return destPool + "/" + rest, true
}

// ZFSSnapshotArgs returns the argv for `zfs snapshot <dataset>@<snapName>`,
// taken before a zvol is streamed into restic. Like virsh, the command runs on
// the host over SSH, so the container needs no zfs tools.
func ZFSSnapshotArgs(dataset, snapName string) []string {
	return []string{"zfs", "snapshot", dataset + "@" + snapName}
}

// ZFSDestroyArgs returns the argv for `zfs destroy <name>`, for the snapshot
// and the clone a backup takes only to read the zvol.
func ZFSDestroyArgs(name string) []string {
	return []string{"zfs", "destroy", name}
}

// ZFSCloneArgs returns the argv that clones snapshot into clone as a read-only
// device. volmode=dev keeps the host from scanning the guest's partitions on
// it, which could otherwise activate the guest's own LVM volumes.
func ZFSCloneArgs(snapshot, clone string) []string {
	return []string{"zfs", "clone", "-o", "volmode=dev", "-o", "readonly=on", snapshot, clone}
}

// ZFSListVolumesArgs returns the argv that lists every zvol of pool with its
// size in bytes, one "name<TAB>volsize" line each. Listing the pool rather
// than the zvol tells a missing zvol from a failed command.
func ZFSListVolumesArgs(pool string) []string {
	return []string{"zfs", "list", "-H", "-p", "-t", "volume", "-o", "name,volsize", "-r", pool}
}

// ZFSCreateVolumeArgs returns the argv that creates a zvol of size bytes,
// with any missing parent datasets.
func ZFSCreateVolumeArgs(dataset string, size int64) []string {
	return []string{"zfs", "create", "-p", "-V", strconv.FormatInt(size, 10), dataset}
}

// ZFSSetVolSizeArgs returns the argv that resizes a zvol to size bytes.
func ZFSSetVolSizeArgs(dataset string, size int64) []string {
	return []string{"zfs", "set", "volsize=" + strconv.FormatInt(size, 10), dataset}
}

// ZvolDevPath is the device node of a zvol.
func ZvolDevPath(dataset string) string {
	return zvolDevPrefix + dataset
}

// UdevSettleArgs returns the argv that waits for udev to finish with new
// devices. A clone destroyed while udev still probes its device fails with
// "dataset is busy".
func UdevSettleArgs() []string {
	return []string{"udevadm", "settle"}
}

// BlockDeviceExistsArgs returns the argv that succeeds once dev is a block
// device. udev creates the /dev/zvol link a moment after zfs returns.
func BlockDeviceExistsArgs(dev string) []string {
	return []string{"test", "-b", dev}
}

// ReadDeviceArgs returns the argv that writes the whole of dev to stdout.
func ReadDeviceArgs(dev string) []string {
	return []string{"dd", "if=" + dev, "bs=1M", "status=none"}
}

// WriteDeviceArgs returns the argv that writes stdin onto dev and flushes it.
// fullblock keeps dd writing whole blocks from a pipe that delivers less.
func WriteDeviceArgs(dev string) []string {
	return []string{"dd", "of=" + dev, "bs=1M", "iflag=fullblock", "conv=fsync", "status=none"}
}

// zvolSnapshotPrefix marks snapshots BombVault takes, so one left behind by an
// interrupted backup is not mistaken for a user's.
const zvolSnapshotPrefix = "bombvault-"

// ZvolSnapshotName returns the snapshot name for a backup taken at now. ZFS
// snapshot names may not contain ':', so the time is written as plain digits.
func ZvolSnapshotName(now time.Time) string {
	return zvolSnapshotPrefix + now.UTC().Format("20060102150405")
}
