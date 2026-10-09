package zfsrepl

import (
	"context"
	"errors"

	"github.com/junkerderprovinz/bombvault/internal/zfs"
)

// Clean removes what replicating left on the source tree under root: it lifts
// the holds and destroys every replica snapshot and bookmark, on excluded
// datasets too, since the item may have excluded less when they were taken.
// The copy on the target stays, and a later run starts with a full stream. It
// tries every point and reports what failed.
func Clean(ctx context.Context, source End, root string) error {
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
		for _, p := range pts {
			if !zfs.IsReplicaSnapshot(p.Name) {
				continue
			}
			if p.Bookmark {
				if err := destroyBookmark(ctx, source, d.Name, p.Name); err != nil {
					errs = append(errs, err)
				}
				continue
			}
			if err := release(ctx, source, d.Name, p.Name); err != nil {
				errs = append(errs, err)
				continue
			}
			args, err := zfs.DestroyReplicaArgs(d.Name, p.Name)
			if err == nil {
				_, err = source.Run(ctx, args)
			}
			if err = ignore(err, zfs.IsNotFound); err != nil {
				errs = append(errs, err)
			}
		}
	}
	return errors.Join(errs...)
}
