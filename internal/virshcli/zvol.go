// On TrueNAS SCALE 25.10 libvirt keeps <source dev='/dev/zvol/<pool>/<dataset>'/>
// as written rather than resolving it to /dev/zdN, so ZvolDatasetFromDevPath
// sees the path it parses. zfs snapshot works on a zvol that a running VM is
// using, and a zfs send stream survives restic backup --stdin and restic dump
// byte for byte.
//
// TrueNAS needs LIBVIRT_URI to name its own socket,
// /run/truenas_libvirt/libvirt-sock; see docs/vm-backup-ssh-setup.md.

package virshcli

import (
	"fmt"
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

// ZFSSnapshotDestroyArgs returns the argv for `zfs destroy <dataset>@<snapName>`.
// The snapshot only exists for the backup, so the caller destroys it in a
// defer whether or not the backup succeeded.
func ZFSSnapshotDestroyArgs(dataset, snapName string) []string {
	return []string{"zfs", "destroy", dataset + "@" + snapName}
}

// ZFSSendArgs returns the argv for `zfs send <dataset>@<snapName>`. The caller
// pipes its stdout over SSH straight into restic's stdin.
func ZFSSendArgs(dataset, snapName string) []string {
	return []string{"zfs", "send", dataset + "@" + snapName}
}

// ZFSReceiveArgs returns the argv for `zfs receive <targetDataset>`. Receiving
// into an existing dataset can destroy live data, so targetDataset must be a
// new name from RestoreZvolTargetDataset, never the source dataset.
func ZFSReceiveArgs(targetDataset string) []string {
	return []string{"zfs", "receive", targetDataset}
}

// zvolRestoreSuffix marks a dataset created by a restore, so the operator can
// tell it from their own before renaming it into place.
const zvolRestoreSuffix = "-bombvault-restore-"

// RestoreZvolTargetDataset returns a new dataset name for a zvol restore,
// "<dataset>-bombvault-restore-<unix-nanoseconds>". It never equals dataset,
// so a receive into it cannot overwrite the live source; moving the data over
// the original zvol is left to the operator. Nanoseconds keep repeated
// restores of the same dataset apart.
func RestoreZvolTargetDataset(dataset string, now time.Time) string {
	return fmt.Sprintf("%s%s%d", dataset, zvolRestoreSuffix, now.UnixNano())
}

// zvolSnapshotPrefix marks snapshots BombVault takes, so one left behind by an
// interrupted backup is not mistaken for a user's.
const zvolSnapshotPrefix = "bombvault-"

// ZvolSnapshotName returns the snapshot name for a backup taken at now. ZFS
// snapshot names may not contain ':', so the time is written as plain digits.
func ZvolSnapshotName(now time.Time) string {
	return zvolSnapshotPrefix + now.UTC().Format("20060102150405")
}
