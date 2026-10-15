package api

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"syscall"
)

// Variables so a test can stand in for a mount table and a disk.
var (
	dataDirIsMount = isMountPoint
	freeBytesOf    = diskFreeBytes
)

// isMountPoint reports whether dir is a mount of its own, such as a ZFS
// dataset under /mnt/<pool>. A mount point cannot be renamed, so the import
// has to empty it instead. An unreadable mount table counts as no mount.
func isMountPoint(dir string) bool {
	mounted, ok := mountPoints()
	return ok && mounted[path.Clean(filepath.ToSlash(dir))]
}

// checkDataDirAside refuses, before anything is stopped, a data folder that
// cannot be set aside. An ordinary folder is renamed and needs nothing. A mount
// point is copied to a sibling, which has to be writable and big enough.
func checkDataDirAside(dir string) error {
	if !dataDirIsMount(dir) {
		return nil
	}
	parent := filepath.Dir(dir)
	probe, err := os.MkdirTemp(parent, ".bombvault-probe-")
	if err != nil {
		return fmt.Errorf("the data folder %s is a mount of its own and its contents are kept next to it, but %s is not writable: %w", dir, parent, err)
	}
	_ = os.Remove(probe)

	var need uint64
	err = filepath.WalkDir(dir, func(_ string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if info, ierr := d.Info(); ierr == nil && info.Mode().IsRegular() {
			need += uint64(info.Size()) //nolint:gosec // G115: a regular file has a non-negative size
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("read the data folder %s: %w", dir, err)
	}
	if free, ferr := freeBytesOf(parent); ferr == nil && free < need {
		return fmt.Errorf("the data folder %s is a mount of its own and its %d MiB are kept next to it, but %s has only %d MiB free",
			dir, need>>20, parent, free>>20)
	}
	return nil
}

// setDataDirAside gets the data folder out of the way for the fresh database.
// An ordinary folder is renamed to kept and made again. A mount point stays
// where it is and only its contents move to kept, so the new server finds an
// empty folder with the owner and mode the mount already has.
func setDataDirAside(dir, kept string, info os.FileInfo, chown func(*os.File, int, int) error) error {
	if !dataDirIsMount(dir) {
		if err := os.Rename(dir, kept); err != nil {
			return fmt.Errorf("set the data folder aside: %w", err)
		}
		return recreateDataDir(dir, info, chown)
	}
	if err := os.Mkdir(kept, info.Mode().Perm()); err != nil {
		return fmt.Errorf("set the data folder aside: %w", err)
	}
	if err := moveContents(dir, kept); err != nil {
		return fmt.Errorf("set the data folder aside: %w", err)
	}
	return nil
}

// putDataDirBack reverses setDataDirAside after a failed import: whatever the
// fresh server wrote goes to failed, the kept contents return. It leaves the
// fresh data in failed even when the old data cannot be restored.
func putDataDirBack(dir, kept, failed string) (failedAt string, err error) {
	if !dataDirIsMount(dir) {
		if err := os.Rename(dir, failed); err != nil && !errors.Is(err, os.ErrNotExist) {
			return dir, err
		}
		if err := os.Rename(kept, dir); err != nil {
			return failed, err
		}
		return failed, nil
	}
	if err := os.Mkdir(failed, 0o700); err != nil {
		return dir, err
	}
	if err := moveContents(dir, failed); err != nil {
		return dir, err
	}
	if err := moveContents(kept, dir); err != nil {
		return failed, err
	}
	_ = os.Remove(kept)
	return failed, nil
}

// moveContents moves every entry of src into the existing folder dst. A rename
// is tried first; across filesystems the entry is copied and then removed, so a
// failure leaves it in src.
func moveContents(src, dst string) error {
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, e := range entries {
		from, to := filepath.Join(src, e.Name()), filepath.Join(dst, e.Name())
		err := os.Rename(from, to)
		if errors.Is(err, syscall.EXDEV) {
			if err = cloneTree(from, to); err == nil {
				err = os.RemoveAll(from)
			} else {
				_ = os.RemoveAll(to)
			}
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// cloneTree copies a file, a symlink or a directory tree with modes and owners.
func cloneTree(from, to string) error {
	info, err := os.Lstat(from)
	if err != nil {
		return err
	}
	switch {
	case info.Mode()&os.ModeSymlink != 0:
		target, err := os.Readlink(from)
		if err != nil {
			return err
		}
		if err := os.Symlink(target, to); err != nil {
			return err
		}
	case info.IsDir():
		if err := os.Mkdir(to, 0o700); err != nil {
			return err
		}
		entries, err := os.ReadDir(from)
		if err != nil {
			return err
		}
		for _, e := range entries {
			if err := cloneTree(filepath.Join(from, e.Name()), filepath.Join(to, e.Name())); err != nil {
				return err
			}
		}
		if err := os.Chmod(to, info.Mode().Perm()); err != nil {
			return err
		}
	case info.Mode().IsRegular():
		if err := cloneFile(from, to, info); err != nil {
			return err
		}
	default:
		return fmt.Errorf("%s is neither a file, a folder nor a link", from)
	}
	if uid, gid, ok := dirOwner(info); ok {
		return os.Lchown(to, uid, gid)
	}
	return nil
}

func cloneFile(from, to string, info os.FileInfo) (err error) {
	in, err := os.Open(from) //nolint:gosec // G304: a path below the container's data folder
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()
	out, err := os.OpenFile(to, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600) //nolint:gosec // G304: a path beside the data folder
	if err != nil {
		return err
	}
	defer func() {
		if cerr := out.Close(); err == nil {
			err = cerr
		}
	}()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Chmod(info.Mode().Perm())
}
