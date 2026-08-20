// Package virshcli — zvol-aware VM disk backup support (v8.0.0 TrueNAS
// platform expansion, Task 10).
//
// ⚠ UNVERIFIED AGAINST REAL HARDWARE. Everything in this file is REASONED
// from ZFS's and TrueNAS's PUBLIC documentation (zfs(8), the
// /dev/zvol/<pool>/<dataset> device-node convention libvirt's own <source
// dev="..."> XML attribute exposes for a block-device-backed disk, and `zfs
// send`/`zfs receive` as the ZFS-native way to move a stable point-in-time
// byte stream off/onto a dataset). No TrueNAS Scale test instance was
// available anywhere in this project's development environment to exercise
// it end-to-end against a real zvol. Only the pure functions below (argv/name
// builders, no process execution) are unit-tested here; the orchestration
// that actually runs these commands over SSH lives in
// internal/backup/vm_orchestrator.go and carries the same caveat. Treat this
// whole mechanism as a documented, unit-tested STARTING POINT — not a
// confirmed-working backup path — until it has been exercised against a real
// TrueNAS Scale box with a real zvol-backed VM (see
// docs/vm-backup-ssh-setup.md's TrueNAS section for the operator-facing
// version of this same warning).
package virshcli

import (
	"fmt"
	"strings"
	"time"
)

// zvolDevPrefix is the libvirt/TrueNAS-documented convention for a zvol's
// device node: /dev/zvol/<pool>/<dataset...>. A block-device disk's
// <source dev="..."> is expected to look like this when the backing store is
// a ZFS zvol (as opposed to some other raw block device — an iSCSI LUN, a
// passed-through physical disk — which this parser deliberately does NOT
// attempt to recognize; see zvolDatasetFromDevPath's doc comment).
const zvolDevPrefix = "/dev/zvol/"

// ZvolDatasetFromDevPath extracts the "<pool>/<dataset>" ZFS dataset path
// from a block-device disk's source dev path, following ZFS/TrueNAS's
// documented /dev/zvol/<pool>/<dataset> device-node convention.
//
// It NEVER guesses. Any devPath that does not match this convention exactly
// — a file-backed path, an unrelated block device (/dev/sda1, an iSCSI LUN,
// a passed-through disk), a bare pool with no dataset segment, an empty
// string, or a path carrying characters that would be unsafe to splice into a
// later shell-quoted `zfs` argv (whitespace, quotes, ".." traversal) — returns
// ok=false rather than an invented dataset name. A caller MUST treat
// ok=false as "this disk cannot be handled by the zvol backup path", never
// as a signal to fall back to a best-effort guess.
//
// Exported (v8.0.0 VM service-layer integration, Task 2) so
// internal/api/service.go can resolve DomainInfo.BlockDisks entries into
// backup.VMBlockDisk.Dataset without duplicating this parser — previously
// unexported and called nowhere outside this package's own test.
func ZvolDatasetFromDevPath(devPath string) (string, bool) {
	if !strings.HasPrefix(devPath, zvolDevPrefix) {
		return "", false
	}
	rest := strings.TrimPrefix(devPath, zvolDevPrefix)
	rest = strings.Trim(rest, "/")
	if rest == "" {
		return "", false // bare "/dev/zvol" or "/dev/zvol/" — no pool at all
	}
	if !strings.Contains(rest, "/") {
		return "", false // a bare pool name with no dataset segment is not a valid zvol path
	}
	if strings.Contains(rest, "..") {
		return "", false // defensive: never let a traversal-looking segment through
	}
	if strings.ContainsAny(rest, " \t\n\r'\"") {
		return "", false // defensive: never splice whitespace/quotes into a `zfs` argv
	}
	return rest, true
}

// RebaseZvolDatasetPool replaces dataset's leading POOL component with
// destPool, leaving every dataset segment after it unchanged — e.g.
// "tank/vms/win10/disk0" rebased onto "flashpool" becomes
// "flashpool/vms/win10/disk0".
//
// This exists for a CROSS-INSTANCE zvol restore: destBase (internal/api/
// service.go's prepareRestoreVMForTarget) remaps a FILE-backed disk onto a
// chosen destination FILESYSTEM PATH, but that path carries no ZFS pool
// information at all — a zvol-backed disk's `zfs receive` target is a
// DATASET, and the source dataset's pool (the first path segment
// ZvolDatasetFromDevPath returns) almost certainly does not exist on the
// destination box under that name. Without this rebase, RestoreZvolDisk
// would derive its receive target purely from the SOURCE pool's name and
// fail deep inside `zfs receive` on the destination — see
// docs/vm-backup-ssh-setup.md's TrueNAS section for the operator-facing
// explanation of why an explicit destination pool is required.
//
// dataset MUST already be in the "<pool>/<rest...>" shape
// ZvolDatasetFromDevPath returns (the only caller in this codebase). destPool
// is validated with the SAME defensive rules ZvolDatasetFromDevPath applies
// to a dataset segment: never empty, never containing a path separator
// (a pool name is one segment, not a nested dataset path — a caller wanting
// to also pin a destination PARENT dataset gets that for free from the
// source's own remaining segments), ".." traversal, or shell-unsafe
// whitespace/quote characters. ok=false on any violation — a caller MUST
// treat that as "cannot safely rebase this restore" and refuse, NEVER fall
// back to the unrebased dataset (which would silently reintroduce the exact
// wrong-pool bug this function exists to close).
func RebaseZvolDatasetPool(dataset, destPool string) (string, bool) {
	destPool = strings.TrimSpace(destPool)
	if destPool == "" {
		return "", false
	}
	if strings.ContainsAny(destPool, " \t\n\r'\"/") {
		return "", false // a pool name is a single segment — never a nested path or unsafe shell character
	}
	if strings.Contains(destPool, "..") {
		return "", false // defensive: never let a traversal-looking segment through
	}
	_, rest, ok := strings.Cut(dataset, "/")
	if !ok || rest == "" {
		return "", false // dataset must already carry at least one segment past its own pool
	}
	return destPool + "/" + rest, true
}

// ZFSSnapshotArgs returns the argv for `zfs snapshot <dataset>@<snapName>` —
// the ZFS-native, point-in-time consistency step taken before streaming a
// zvol's content into a restic backup. Pure/unit-testable: calling this
// touches no real ZFS system. Executed over the EXISTING SSH transport
// (internal/sshconn.Conn — see its StreamCommand/RunWithStdin doc comments),
// the same mechanism virshcli already uses to shell `virsh`/NVRAM commands to
// the host rather than requiring a local zfs toolchain in the container.
func ZFSSnapshotArgs(dataset, snapName string) []string {
	return []string{"zfs", "snapshot", dataset + "@" + snapName}
}

// ZFSSnapshotDestroyArgs returns the argv for `zfs destroy <dataset>@<snapName>`
// — cleanup for the snapshot ZFSSnapshotArgs created. The snapshot is a live
// consistency point for the backup, not the backup artifact itself, so a
// caller MUST destroy it unconditionally (success or failure of everything in
// between) via a `defer` — see BackupZvolDisk in
// internal/backup/vm_orchestrator.go.
func ZFSSnapshotDestroyArgs(dataset, snapName string) []string {
	return []string{"zfs", "destroy", dataset + "@" + snapName}
}

// ZFSSendArgs returns the argv for `zfs send <dataset>@<snapName>`, which
// streams the snapshot's content to stdout as a ZFS send stream — the
// ZFS-native, documented way to get a stable point-in-time byte stream off a
// dataset/zvol without mounting or converting it. The caller pipes this
// stdout, over SSH, straight into a restic backup's stdin (see
// internal/restic.Restic.BackupStdin) rather than staging it to a local file.
func ZFSSendArgs(dataset, snapName string) []string {
	return []string{"zfs", "send", dataset + "@" + snapName}
}

// ZFSReceiveArgs returns the argv for `zfs receive <targetDataset>` — the
// restore-side counterpart of ZFSSendArgs.
//
// targetDataset MUST be a fresh, never-before-existing dataset name (see
// RestoreZvolTargetDataset) — NEVER the original source dataset. Receiving
// into an EXISTING dataset can destroy live data; the restore orchestrator
// (internal/backup/vm_orchestrator.go's RestoreZvolDisk) is structured so the
// original dataset name can never reach this function.
func ZFSReceiveArgs(targetDataset string) []string {
	return []string{"zfs", "receive", targetDataset}
}

// zvolRestoreSuffix marks a dataset RestoreZvolTargetDataset created — never
// an operator's own naming, so it is unambiguous which datasets are BombVault
// restore landing zones awaiting the operator's manual rename/promote step.
const zvolRestoreSuffix = "-bombvault-restore-"

// RestoreZvolTargetDataset returns a freshly-named target dataset for a zvol
// restore's `zfs receive`, derived from the source dataset and a timestamp:
// "<dataset>-bombvault-restore-<unix-nanoseconds>".
//
// SAFETY PROPERTY (structural, not just documented): the returned name is
// NEVER equal to dataset — it always carries a non-empty suffix — so a caller
// that always builds ZFSReceiveArgs from THIS function's output can never
// receive into the live source dataset. Restoring the received data back over
// the original zvol (renaming/promoting this fresh dataset over dataset) is a
// deliberate, DOCUMENTED MANUAL follow-up step for the operator — never
// automated here (see docs/vm-backup-ssh-setup.md's TrueNAS section).
//
// Nanosecond resolution keeps repeated restores of the same source dataset
// (e.g. a retried or re-run restore) landing on distinct target datasets
// rather than colliding on one fixed name.
func RestoreZvolTargetDataset(dataset string, now time.Time) string {
	return fmt.Sprintf("%s%s%d", dataset, zvolRestoreSuffix, now.UnixNano())
}

// zvolSnapshotPrefix marks a snapshot BackupZvolDisk created, mirroring how
// LiveSnapshotName (vm_orchestrator.go) marks BombVault's own live-backup
// overlay — unambiguous so a leftover snapshot from an interrupted zvol
// backup is recognizable as BombVault's, never a user's manual ZFS snapshot.
const zvolSnapshotPrefix = "bombvault-"

// ZvolSnapshotName returns the ZFS snapshot name BackupZvolDisk uses for a
// backup taken at instant now: "bombvault-<RFC3339-ish, colon-free>". Pure
// function of its input (same now → same name), so a caller (and a test) can
// reason about it deterministically. ZFS snapshot names may not contain ':',
// so the timestamp is rendered as compact digits, not RFC3339 verbatim.
func ZvolSnapshotName(now time.Time) string {
	return zvolSnapshotPrefix + now.UTC().Format("20060102150405")
}
