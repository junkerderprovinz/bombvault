package zfsrepl

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/zfs"
)

// restoreSuffix marks a dataset a bring back created, the same landing name
// every other ZFS restore in BombVault uses.
const restoreSuffix = "-bombvault-restore-"

// RestoreName is the dataset BringBack lands a snapshot of dataset in when it
// starts at at.
func RestoreName(dataset string, at time.Time) string {
	return fmt.Sprintf("%s%s%d", dataset, restoreSuffix, at.UnixNano())
}

// Restore is one replica snapshot to bring back to the host it came from.
type Restore struct {
	// Replica is the dataset on the host that keeps the replica.
	Replica  string
	Snapshot string
	// Dataset is the original on the other host. The snapshot lands next to
	// it, never in it.
	Dataset  string
	Now      func() time.Time
	Progress func(done, total int64)
}

// BringBack sends a replica snapshot from the host that keeps it into a new
// dataset <Dataset>-bombvault-restore-<unix nanoseconds> on the other host and
// returns that name. An encrypted replica travels raw, so it arrives under its
// own key.
func BringBack(ctx context.Context, from, to End, r Restore) (string, error) {
	st, err := state(ctx, from, r.Replica)
	if err != nil {
		return "", err
	}
	kept, err := points(ctx, from, r.Replica)
	if err != nil {
		return "", err
	}
	want, ok := find(kept, r.Snapshot)
	if !ok || want.Bookmark {
		return "", &Refusal{Code: "not-found", Detail: r.Replica + "@" + r.Snapshot}
	}
	dest := RestoreName(r.Dataset, r.Now())
	send := zfs.SendSpec{Member: r.Replica, Snap: r.Snapshot, Raw: st.Encrypted()}
	sendArgs, err := zfs.SendArgs(send)
	if err != nil {
		return "", err
	}
	estArgs, err := zfs.EstimateArgs(send)
	if err != nil {
		return "", err
	}
	recvArgs, err := zfs.RestoreReceiveArgs(dest, st.Type == "volume")
	if err != nil {
		return "", err
	}
	var report func(int64)
	if r.Progress != nil {
		total := estimate(ctx, from, estArgs)
		report = func(done int64) { r.Progress(done, total) }
	}
	if _, err := pipe(ctx, from, to, sendArgs, recvArgs, report); err != nil {
		return "", err
	}
	got, err := points(ctx, to, dest)
	if err != nil {
		return "", err
	}
	if p, ok := find(got, r.Snapshot); !ok || p.GUID != want.GUID {
		return "", &Refusal{Code: "zfs-error", Detail: dest + "@" + r.Snapshot + " is not there after the receive"}
	}
	return dest, nil
}

// ReleaseHolds lifts the replica hold from every replica snapshot in the tree
// under root. A held snapshot blocks zfs destroy -r of the tree, so this runs
// before an entry or one of its datasets is removed. It tries every snapshot
// and reports what failed.
func ReleaseHolds(ctx context.Context, source End, root string) error {
	tree, err := listTree(ctx, source, root)
	if err != nil {
		return err
	}
	var errs []error
	for _, d := range tree {
		pts, err := points(ctx, source, d.Name)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		for _, p := range snapshots(pts) {
			if !zfs.IsReplicaSnapshot(p.Name) {
				continue
			}
			if err := release(ctx, source, d.Name, p.Name); err != nil {
				errs = append(errs, err)
			}
		}
	}
	return errors.Join(errs...)
}
