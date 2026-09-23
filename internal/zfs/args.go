package zfs

import (
	"path"
	"strings"
)

// treeProps are the properties one item's tree is listed with. They are what
// MemberCode needs to decide whether a dataset can be read.
const treeProps = "name,type,mountpoint,mounted,canmount,encryption,keystatus,snapdir,referenced,usedbydataset"

// listProps add "used" for discovery, where the dialog shows what a pool holds
// including snapshots and children.
const listProps = "name,type,mountpoint,mounted,canmount,encryption,keystatus,snapdir,used,referenced,usedbydataset"

// zfsBinary is the argv[0] every builder emits. SSHHost replaces it when the
// host only has zfs under /usr/sbin.
const zfsBinary = "zfs"

// VersionArgs asks the host for its ZFS version, the cheapest proof that the
// SSH path works and that zfs is on the remote PATH.
func VersionArgs() []string { return []string{zfsBinary, "version"} }

// TreeArgs lists one item's tree, root first and in tree order.
func TreeArgs(root string) ([]string, error) {
	if err := validateNameChars(root); err != nil {
		return nil, err
	}
	return []string{zfsBinary, "list", "-H", "-p", "-r", "-t", "filesystem,volume", "-o", treeProps, root}, nil
}

// ListArgs lists every dataset and volume of every pool, for discovery.
func ListArgs() []string {
	return []string{zfsBinary, "list", "-H", "-p", "-t", "filesystem,volume", "-o", listProps}
}

// SnapshotsArgs lists the snapshots of one item's tree, for the leftover sweep.
func SnapshotsArgs(root string) ([]string, error) {
	if err := validateNameChars(root); err != nil {
		return nil, err
	}
	return []string{zfsBinary, "list", "-H", "-p", "-r", "-t", "snapshot", "-o", "name,creation,used", root}, nil
}

// SnapshotRecursiveArgs takes the one atomic snapshot a backup run reads. snap
// must be a name SnapshotName built, so nothing else can end up being swept or
// destroyed recursively later.
func SnapshotRecursiveArgs(root, snap string) ([]string, error) {
	if err := ValidateDatasetName(root); err != nil {
		return nil, err
	}
	if !IsBombVaultSnapshot(snap) {
		return nil, &NameError{Code: "invalid-name", Reason: "not a BombVault snapshot name: " + snap}
	}
	return []string{zfsBinary, "snapshot", "-r", root + "@" + snap}, nil
}

// DestroyRecursiveArgs removes a run's snapshot from the whole tree. This is
// the only builder that emits -r for destroy, and it only ever does so with a
// name IsBombVaultSnapshot accepts and always with an "@", so -r can never
// reach a dataset.
func DestroyRecursiveArgs(root, snap string) ([]string, error) {
	if err := ValidateDatasetName(root); err != nil {
		return nil, err
	}
	if !IsBombVaultSnapshot(snap) {
		return nil, &NameError{Code: "invalid-name", Reason: "not a BombVault snapshot name: " + snap}
	}
	return []string{zfsBinary, "destroy", "-r", root + "@" + snap}, nil
}

// SnapshotSafetyArgs takes the safety snapshot of the one dataset an in-place
// restore writes into. It is never recursive: the children keep their own data
// and the user deletes the snapshot when they are done with it.
func SnapshotSafetyArgs(dataset, snap string) ([]string, error) {
	if err := ValidateDatasetName(dataset); err != nil {
		return nil, err
	}
	if !IsPreRestoreSnapshot(snap) {
		return nil, &NameError{Code: "invalid-name", Reason: "not a pre-restore snapshot name: " + snap}
	}
	return []string{zfsBinary, "snapshot", dataset + "@" + snap}, nil
}

// DestroyPreRestoreArgs removes one safety snapshot on the user's request.
func DestroyPreRestoreArgs(dataset, snap string) ([]string, error) {
	if err := ValidateDatasetName(dataset); err != nil {
		return nil, err
	}
	if !IsPreRestoreSnapshot(snap) {
		return nil, &NameError{Code: "invalid-name", Reason: "not a pre-restore snapshot name: " + snap}
	}
	return []string{zfsBinary, "destroy", dataset + "@" + snap}, nil
}

// PrimeArgs reads the snapshot directory on the host. The kernel mounts a
// snapshot at the path the host sees, so triggering the automount from there
// leaves the container with nothing to do but wait for it to propagate.
func PrimeArgs(hostMountpoint, snap string) ([]string, error) {
	if !IsBombVaultSnapshot(snap) {
		return nil, &NameError{Code: "invalid-name", Reason: "not a BombVault snapshot name: " + snap}
	}
	if !strings.HasPrefix(hostMountpoint, "/") || hostMountpoint != path.Clean(hostMountpoint) {
		return nil, &NameError{Code: "invalid-name", Reason: "mountpoint is not an absolute clean path: " + hostMountpoint}
	}
	return []string{"stat", "-c", "%d", hostMountpoint + "/.zfs/snapshot/" + snap + "/."}, nil
}
