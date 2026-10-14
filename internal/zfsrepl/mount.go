package zfsrepl

import (
	"context"
	"log"

	"github.com/junkerderprovinz/bombvault/internal/zfs"
)

// What mounting a member on the target came to. A member that was not asked
// to mount, a volume for one, has none of these.
const (
	MountDone   = "mounted"
	MountLocked = "locked"
	MountNoPath = "no-mountpoint"
	MountFailed = "failed"
)

// mount makes a received filesystem readable on the target and lets it mount
// by itself from now on. A full stream lands unmounted and an increment into a
// mounted copy leaves it mounted, so every run reads the state and does only
// what is missing. A failure does not fail the member, whose data arrived.
func (r *run) mount(ctx context.Context, target string) string {
	args, err := zfs.MountStateArgs(target)
	if err != nil {
		return MountFailed
	}
	out, err := r.dst.Run(ctx, args)
	var st zfs.MountState
	if err == nil {
		st, err = zfs.ParseMountState(out)
	}
	if err != nil {
		log.Printf("zfs replica: reading the mount state of %s failed: %v", target, err)
		return MountFailed
	}
	switch {
	case st.Locked():
		return MountLocked
	case !st.HasMountpoint():
		return MountNoPath
	}
	var steps []func(string) ([]string, error)
	if !st.ReadOnly {
		steps = append(steps, zfs.ReadOnlyArgs)
	}
	if !st.Mounted {
		steps = append(steps, zfs.MountArgs)
	}
	if st.CanMount != "on" {
		steps = append(steps, zfs.CanMountArgs)
	}
	for _, build := range steps {
		args, err := build(target)
		if err == nil {
			_, err = r.dst.Run(ctx, args)
		}
		if err != nil {
			log.Printf("zfs replica: mounting %s failed: %v", target, err)
			return MountFailed
		}
	}
	return MountDone
}
