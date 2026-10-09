package zfsrepl

import (
	"context"
	"errors"

	"github.com/junkerderprovinz/bombvault/internal/zfs"
)

// Clean removes what replicating left on the source tree under root but one
// bookmark per dataset, the newest, from which a later run can continue the
// copy kept on the target. Excluded datasets are cleaned too, since the item
// may have excluded less earlier. It tries every point and reports what failed.
func Clean(ctx context.Context, source End, root string) error {
	tree, err := listTree(ctx, source, root)
	if err != nil {
		return err
	}
	var errs []error
	for _, d := range tree {
		if err := cleanDataset(ctx, source, d.Name); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func cleanDataset(ctx context.Context, source End, dataset string) error {
	pts, err := points(ctx, source, dataset)
	if err != nil {
		return err
	}
	var replica []zfs.ReplicaPoint
	for _, p := range pts {
		if zfs.IsReplicaSnapshot(p.Name) {
			replica = append(replica, p)
		}
	}
	newest, ok := newestPoint(replica)
	if !ok {
		return nil
	}
	// The newest state may still be a snapshot alone, when the run that took
	// it stopped before its bookmark.
	if !newest.Bookmark {
		if err := bookmarkOn(ctx, source, dataset, newest.Name); err != nil {
			return err
		}
	}
	var errs []error
	for _, p := range replica {
		if p.Bookmark {
			if p.Name == newest.Name {
				continue
			}
			if err := destroyBookmark(ctx, source, dataset, p.Name); err != nil {
				errs = append(errs, err)
			}
			continue
		}
		if err := release(ctx, source, dataset, p.Name); err != nil {
			errs = append(errs, err)
			continue
		}
		args, err := zfs.DestroyReplicaArgs(dataset, p.Name)
		if err == nil {
			_, err = source.Run(ctx, args)
		}
		if err = ignore(err, zfs.IsNotFound); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// newestPoint is the replica point with the highest createtxg, the bookmark
// where a snapshot and its bookmark share one.
func newestPoint(pts []zfs.ReplicaPoint) (zfs.ReplicaPoint, bool) {
	var last zfs.ReplicaPoint
	found := false
	for _, p := range pts {
		if !found || p.CreateTxg > last.CreateTxg || (p.CreateTxg == last.CreateTxg && p.Bookmark) {
			last, found = p, true
		}
	}
	return last, found
}

func bookmarkOn(ctx context.Context, end End, dataset, snap string) error {
	args, err := zfs.BookmarkArgs(dataset, snap)
	if err != nil {
		return err
	}
	_, err = end.Run(ctx, args)
	return ignore(err, zfs.IsExists)
}
