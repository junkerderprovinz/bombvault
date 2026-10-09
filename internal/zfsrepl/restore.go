package zfsrepl

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/zfs"
)

// RestoreName is the dataset BringBack lands the root of dataset in when it
// starts at at: next to it as <dataset>-bombvault-restore-<unix nanoseconds>,
// the landing name every other ZFS restore in BombVault uses. A pool's top
// dataset has nothing next to it, so its copy lands inside the pool.
func RestoreName(dataset string, at time.Time) string {
	if !strings.Contains(dataset, "/") {
		return fmt.Sprintf("%s/bombvault-restore-%d", dataset, at.UnixNano())
	}
	return fmt.Sprintf("%s-bombvault-restore-%d", dataset, at.UnixNano())
}

// Restore is one replica snapshot of an entry's tree to bring back to the
// host it came from.
type Restore struct {
	// Replica is the root of the copy on the host that keeps it.
	Replica  string
	Snapshot string
	// Dataset is the entry's root on the other host. The tree lands next to
	// it, never in it.
	Dataset string
	Now     func() time.Time
	// Progress hears the bytes of the member being sent and its estimate.
	Progress func(done, total int64)
}

// Restored is what a bring back created: Root and below it every member in
// Members, parents first. Skipped are the members of the copy that do not
// hold the snapshot, with everything below them.
type Restored struct {
	Root    string
	Members []RestoredMember
	Skipped []string
}

// RestoredMember is one dataset or volume a bring back created.
type RestoredMember struct {
	Dataset   string
	Volume    bool
	Encrypted bool
}

// BringBack sends one replica snapshot of every member of the copy under
// r.Replica into a new tree under RestoreName on the other host. Encrypted
// members travel raw and arrive under their own keys. The root has to hold
// the snapshot; a member below that does not is left out. On a failure the
// result names what had landed by then.
func BringBack(ctx context.Context, from, to End, r Restore) (Restored, error) {
	tree, err := listTree(ctx, from, r.Replica)
	if err != nil {
		return Restored{}, err
	}
	out := Restored{Root: RestoreName(r.Dataset, r.Now())}
	var skipped []string
	for i, d := range tree {
		if excluded(d.Name, skipped) {
			continue
		}
		kept, err := points(ctx, from, d.Name)
		if err != nil {
			return out, err
		}
		want, ok := find(kept, r.Snapshot)
		if !ok || want.Bookmark {
			if i == 0 {
				return out, &Refusal{Code: "not-found", Detail: d.Name + "@" + r.Snapshot}
			}
			skipped = append(skipped, d.Name)
			out.Skipped = append(out.Skipped, d.Name)
			continue
		}
		m := RestoredMember{
			Dataset:   out.Root + strings.TrimPrefix(d.Name, r.Replica),
			Volume:    d.Type == "volume",
			Encrypted: d.Encryption != "off",
		}
		if err := bringBackOne(ctx, from, to, d.Name, m, want, r.Progress); err != nil {
			return out, err
		}
		out.Members = append(out.Members, m)
	}
	return out, nil
}

func bringBackOne(ctx context.Context, from, to End, replica string, m RestoredMember, want zfs.ReplicaPoint, progress func(done, total int64)) error {
	send := zfs.SendSpec{Member: replica, Snap: want.Name, Raw: m.Encrypted}
	sendArgs, err := zfs.SendArgs(send)
	if err != nil {
		return err
	}
	estArgs, err := zfs.EstimateArgs(send)
	if err != nil {
		return err
	}
	recvArgs, err := zfs.RestoreReceiveArgs(m.Dataset, m.Volume)
	if err != nil {
		return err
	}
	var report func(int64)
	if progress != nil {
		total := estimate(ctx, from, estArgs)
		report = func(done int64) { progress(done, total) }
	}
	if _, err := pipe(ctx, from, to, sendArgs, recvArgs, report); err != nil {
		return err
	}
	got, err := points(ctx, to, m.Dataset)
	if err != nil {
		return err
	}
	if p, ok := find(got, want.Name); !ok || p.GUID != want.GUID {
		return &Refusal{Code: "zfs-error", Detail: m.Dataset + "@" + want.Name + " is not there after the receive"}
	}
	return nil
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
