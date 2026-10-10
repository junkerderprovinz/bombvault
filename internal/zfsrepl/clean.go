package zfsrepl

import (
	"context"
	"errors"

	"github.com/junkerderprovinz/bombvault/internal/zfs"
)

// Clean removes what replicating left on the source tree under root but the
// bookmarks a later run can continue the copy kept on the target from: the
// newest bookmark of each dataset, and one for every replica snapshot taken
// after it. Datasets the item excludes are cleaned too, since it may have
// excluded less earlier, but none of their snapshots was sent, so they keep
// only the newest bookmark they already have. It tries every point and
// reports what failed.
func Clean(ctx context.Context, source End, root string, excludes []string) error {
	tree, err := listTree(ctx, source, root)
	if err != nil {
		return err
	}
	var errs []error
	for _, d := range tree {
		if RestoreLanding(d.Name, root) {
			continue
		}
		if err := cleanDataset(ctx, source, d.Name, !excluded(d.Name, excludes)); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func cleanDataset(ctx context.Context, source End, dataset string, sent bool) error {
	pts, err := points(ctx, source, dataset)
	if err != nil {
		return err
	}
	var replica []zfs.ReplicaPoint
	var mark zfs.ReplicaPoint
	marked := false
	for _, p := range pts {
		if !zfs.IsReplicaSnapshot(p.Name) {
			continue
		}
		replica = append(replica, p)
		if p.Bookmark && (!marked || p.CreateTxg > mark.CreateTxg) {
			mark, marked = p, true
		}
	}
	keep := map[string]bool{}
	if marked {
		keep[mark.Name] = true
	}
	// A run bookmarks a snapshot only once the target holds it, so the newest
	// bookmark is a base the target shares. A snapshot after it may have
	// arrived there or never left, and as a bookmark it costs nothing.
	if sent {
		for _, p := range replica {
			if p.Bookmark || (marked && p.CreateTxg <= mark.CreateTxg) {
				continue
			}
			if err := bookmarkOn(ctx, source, dataset, p.Name); err != nil {
				return err
			}
			keep[p.Name] = true
		}
	}
	var errs []error
	for _, p := range replica {
		if p.Bookmark {
			if keep[p.Name] {
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

func bookmarkOn(ctx context.Context, end End, dataset, snap string) error {
	args, err := zfs.BookmarkArgs(dataset, snap)
	if err != nil {
		return err
	}
	_, err = end.Run(ctx, args)
	return ignore(err, zfs.IsExists)
}
