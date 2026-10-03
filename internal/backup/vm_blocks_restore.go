package backup

import (
	"archive/tar"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path"
	"path/filepath"
	"time"
)

func writeManifest(p string, m BlocksManifest, mtime time.Time) error {
	b, err := json.Marshal(m)
	if err != nil {
		return fmt.Errorf("vm backup: manifest: %w", err)
	}
	if err := os.WriteFile(p, b, 0o600); err != nil {
		return fmt.Errorf("vm backup: manifest: %w", err)
	}
	if err := os.Chtimes(p, mtime, mtime); err != nil {
		return fmt.Errorf("vm backup: manifest: %w", err)
	}
	return nil
}

// ParseBlocksManifest reads a manifest a changed-block backup wrote.
func ParseBlocksManifest(b []byte) (BlocksManifest, error) {
	var m BlocksManifest
	if err := json.Unmarshal(b, &m); err != nil {
		return BlocksManifest{}, fmt.Errorf("vm blocks manifest: %w", err)
	}
	if m.Version != 1 {
		return BlocksManifest{}, fmt.Errorf("vm blocks manifest: unknown version %d", m.Version)
	}
	for _, d := range m.Disks {
		if d.Dev == "" || d.Size < 0 || d.Segment <= 0 {
			return BlocksManifest{}, errors.New("vm blocks manifest: a disk entry is incomplete")
		}
	}
	return m, nil
}

// BlockDumper streams a directory of a snapshot as a tar archive.
type BlockDumper interface {
	DumpDir(ctx context.Context, repo, snapshotID, dir string, w io.Writer) error
}

// ImageConverter turns the raw image at rawPath into target in format.
type ImageConverter func(ctx context.Context, rawPath, target, format string) error

// VMRestoreImage is one disk of a segment-layout snapshot and the file it is
// restored to.
type VMRestoreImage struct {
	Disk   BlocksDisk
	Target string
}

// RestoreBlockImage puts one disk of a segment-layout snapshot back together
// at img.Target: the segments are written at their offsets with zeros left
// as holes, and a disk that was not raw is converted back to its format. The
// target is only replaced once the new image is complete.
func RestoreBlockImage(ctx context.Context, dump BlockDumper, convert ImageConverter, repo, snapshotID string, img VMRestoreImage) error {
	disk := img.Disk
	if err := os.MkdirAll(filepath.Dir(img.Target), 0o755); err != nil { //nolint:gosec // G301: VM folders are readable, like the ones libvirt creates
		return fmt.Errorf("restore disk %s: %w", disk.Dev, err)
	}
	raw := img.Target + ".bombvault-restore"
	part := img.Target + ".bombvault-part"
	defer func() {
		_ = os.Remove(raw)
		_ = os.Remove(part)
	}()
	if err := assembleImage(ctx, dump, repo, snapshotID, disk, raw); err != nil {
		return fmt.Errorf("restore disk %s: %w", disk.Dev, err)
	}
	built := raw
	if disk.Format != "" && disk.Format != "raw" {
		if convert == nil {
			return fmt.Errorf("restore disk %s: no converter for %s", disk.Dev, disk.Format)
		}
		if err := convert(ctx, raw, part, disk.Format); err != nil {
			return fmt.Errorf("restore disk %s: convert to %s: %w", disk.Dev, disk.Format, err)
		}
		built = part
	}
	if disk.Mode != 0 {
		if err := os.Chmod(built, os.FileMode(disk.Mode).Perm()); err != nil {
			log.Printf("vm restore: disk %s: set mode: %v", disk.Dev, err)
		}
	}
	if err := os.Chown(built, disk.UID, disk.GID); err != nil {
		log.Printf("vm restore: disk %s: set owner: %v", disk.Dev, err)
	}
	if err := os.Rename(built, img.Target); err != nil {
		return fmt.Errorf("restore disk %s: %w", disk.Dev, err)
	}
	return nil
}

// assembleImage writes the raw image of disk into out.
func assembleImage(ctx context.Context, dump BlockDumper, repo, snapshotID string, disk BlocksDisk, out string) error {
	f, err := os.OpenFile(out, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600) //nolint:gosec // G304: the restore's own temporary file
	if err != nil {
		return err
	}
	defer f.Close() //nolint:errcheck // closed explicitly below on success

	pr, pw := io.Pipe()
	dumped := make(chan error, 1)
	go func() {
		err := dump.DumpDir(ctx, repo, snapshotID, "/"+disk.Dev, pw)
		_ = pw.CloseWithError(err)
		dumped <- err
	}()
	werr := writeSegments(ctx, tar.NewReader(pr), f, disk)
	_ = pr.CloseWithError(werr)
	if dErr := <-dumped; dErr != nil && werr == nil {
		werr = fmt.Errorf("restic dump: %w", dErr)
	}
	if werr != nil {
		return werr
	}
	if err := f.Truncate(disk.Size); err != nil {
		return err
	}
	if err := f.Sync(); err != nil {
		return err
	}
	return f.Close()
}

// CheckBlocksLayout says whether the segment files listed for disk, by name
// and size, are the whole disk the way a restore reads it back.
func CheckBlocksLayout(disk BlocksDisk, segments map[string]int64) error {
	count := (disk.Size + disk.Segment - 1) / disk.Segment
	for name, size := range segments {
		idx, ok := segmentIndex(name)
		if !ok || idx >= count {
			return fmt.Errorf("disk %s: unexpected segment %q", disk.Dev, name)
		}
		if want := min(disk.Segment, disk.Size-idx*disk.Segment); size != want {
			return fmt.Errorf("disk %s: segment %q holds %d bytes, want %d", disk.Dev, name, size, want)
		}
	}
	if int64(len(segments)) != count {
		return fmt.Errorf("disk %s: the snapshot holds %d of %d segments", disk.Dev, len(segments), count)
	}
	return nil
}

func writeSegments(ctx context.Context, tr *tar.Reader, f *os.File, disk BlocksDisk) error {
	count := (disk.Size + disk.Segment - 1) / disk.Segment
	seen := make(map[int64]bool, count)
	buf := make([]byte, blockIOSize)
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return fmt.Errorf("read segments: %w", err)
		}
		if hdr.Typeflag != tar.TypeReg {
			continue
		}
		idx, ok := segmentIndex(path.Base(hdr.Name))
		if !ok || idx >= count || seen[idx] {
			return fmt.Errorf("unexpected segment %q", hdr.Name)
		}
		want := min(disk.Segment, disk.Size-idx*disk.Segment)
		if hdr.Size != want {
			return fmt.Errorf("segment %q holds %d bytes, want %d", hdr.Name, hdr.Size, want)
		}
		seen[idx] = true
		for pos := int64(0); pos < want; {
			if err := ctx.Err(); err != nil {
				return err
			}
			n, err := io.ReadFull(tr, buf[:min(int64(len(buf)), want-pos)])
			if err != nil {
				return fmt.Errorf("read segment %q: %w", hdr.Name, err)
			}
			if err := writeSparse(f, buf[:n], idx*disk.Segment+pos); err != nil {
				return err
			}
			pos += int64(n)
		}
	}
	if int64(len(seen)) != count {
		return fmt.Errorf("the snapshot holds %d of %d segments", len(seen), count)
	}
	return nil
}
