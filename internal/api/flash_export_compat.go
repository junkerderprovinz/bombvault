package api

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/junkerderprovinz/bombvault/internal/restic"
)

// fsckRecPattern matches dosfsck/fsck.vfat recovery dumps (FSCK0000.REC, ...),
// written to the filesystem root whenever a check finds orphaned clusters.
// Never legitimate boot content.
var fsckRecPattern = regexp.MustCompile(`^FSCK\d+\.REC$`)

// flashZipJunkEntry reports whether a zip entry has no place in a flash
// export, for one of two distinct reasons. Foreign-tool litter that never
// belonged on the flash drive in the first place: a stray git checkout at
// the flash root (.git/, .gitattributes — someone's accidental repo), Windows'
// System Volume Information (left behind whenever the FAT32 flash drive is
// plugged into a Windows PC), fsck recovery dumps, and 0-byte .bmp marker
// files from that same Windows-mount cause. A real syslinux background image
// (also .bmp) is never 0 bytes, so only empty ones are dropped — this must
// never blanket-exclude the extension itself (bombvault#136). And Unraid's
// own previous/ folder: the prior OS version's boot files, kept automatically
// as an upgrade rollback safety net. It is not part of what a flash export
// promises to restore (OS, license, array config, shares, network, plugins —
// all under config/ or the top-level bz* files), Unraid recreates it fresh on
// the next real upgrade regardless, and on real-world boot sticks it can be a
// third or more of the whole export's size.
func flashZipJunkEntry(name string, uncompressedSize uint64) bool {
	name = strings.TrimPrefix(name, "/")
	base := path.Base(strings.TrimSuffix(name, "/"))
	switch {
	case name == ".git" || strings.HasPrefix(name, ".git/"):
		return true
	case name == ".gitattributes":
		return true
	case name == "System Volume Information" || strings.HasPrefix(name, "System Volume Information/"):
		return true
	case fsckRecPattern.MatchString(base):
		return true
	case uncompressedSize == 0 && strings.EqualFold(path.Ext(base), ".bmp"):
		return true
	case name == "previous" || strings.HasPrefix(name, "previous/"):
		return true
	}
	return false
}

// recompressFlashZip reads a zip archive as restic's `dump -a zip` writes it
// (every entry stored uncompressed — restic's blobs are already compressed at
// the repository level, so it deliberately skips a redundant deflate pass)
// and rewrites it into dst with every entry deflated, dropping known-junk
// entries along the way (flashZipJunkEntry). The official Unraid USB
// Creator's unzip implementation fails on restic's fully-stored output
// (verified against a real device: it errors part-way through extraction on
// an all-Store archive but succeeds on an equivalent all-Deflate one —
// bombvault#136); re-compressing here is the only fix available to us, since
// restic's `dump` subcommand has no flag to choose its compression method.
func recompressFlashZip(src io.ReaderAt, size int64, dst io.Writer) error {
	zr, err := zip.NewReader(src, size)
	if err != nil {
		return fmt.Errorf("open dumped zip: %w", err)
	}
	zw := zip.NewWriter(dst)
	for _, f := range zr.File {
		if flashZipJunkEntry(f.Name, f.UncompressedSize64) {
			continue
		}
		if err := copyZipEntry(zw, f); err != nil {
			return fmt.Errorf("recompress %s: %w", f.Name, err)
		}
	}
	return zw.Close()
}

// copyZipEntry writes one entry of a source zip into zw, deflated. It builds
// a FRESH FileHeader (name, mtime, mode only) rather than reusing the source
// FileHeader wholesale: the source's raw Extra field may carry a Zip64
// extension sized for ITS original layout, and blindly copying those bytes
// forward while zip.Writer independently decides whether ITS OWN copy needs
// Zip64 (based on the new, recompressed size) risks two conflicting Zip64
// records in one entry. Leaving Extra unset lets zip.Writer own that decision
// cleanly for the entry it is actually writing.
func copyZipEntry(zw *zip.Writer, f *zip.File) error {
	hdr := &zip.FileHeader{
		Name:     f.Name,
		Modified: f.Modified,
		Comment:  f.Comment,
		Method:   zip.Deflate,
	}
	if f.FileInfo().IsDir() || f.UncompressedSize64 == 0 {
		hdr.Method = zip.Store // matches normal zip-tool convention; nothing to gain compressing empty/dir entries
	}
	hdr.SetMode(f.Mode())
	w, err := zw.CreateHeader(hdr)
	if err != nil {
		return err
	}
	r, err := f.Open()
	if err != nil {
		return err
	}
	defer func() { _ = r.Close() }()
	// Bound the copy to the entry's own declared size: restic's dump is
	// trusted input in normal operation, but capping this defends against a
	// corrupt zip whose compressed stream produces more bytes than its
	// header claims, and gives a clear error if it produces fewer
	// (gosec G110 decompression-bomb guard).
	if _, err := io.CopyN(w, r, int64(f.UncompressedSize64)); err != nil { //nolint:gosec // G115: a real zip entry size (flash backups are GBs at most), nowhere near MaxInt64
		return fmt.Errorf("copy %d declared bytes: %w", f.UncompressedSize64, err)
	}
	return nil
}

// dumpFlashZipCompat runs engine.DumpZip and rewrites its output through
// recompressFlashZip before it reaches dst (see that function's doc for why).
// archive/zip needs random access (io.ReaderAt) to read a zip's central
// directory, which a live streaming destination (the HTTP response, or an
// age-encryption pipe) can never provide — so the raw dump is staged through
// a scratch file under DataDir first, matching this codebase's established
// "write to a fixed, dot-prefixed temp path" convention (see exportFlashZip).
// The scratch file is always removed afterward; unlike exportFlashZip's own
// temp file, it is never the deliverable, only a means to get random access.
func (s *Service) dumpFlashZipCompat(ctx context.Context, repo, snapshotID, subfolder string, dst io.Writer, mode restic.Mode) error {
	tmp := filepath.Join(s.cfg.DataDir, ".flash-download.tmp.zip")
	if err := os.MkdirAll(s.cfg.DataDir, 0o700); err != nil { //nolint:gosec // G301: app-private data dir, not shared
		return fmt.Errorf("flash download: create data dir: %w", err)
	}
	f, err := os.Create(tmp) //nolint:gosec // G304: fixed path under the app's own DataDir, not user input
	if err != nil {
		return fmt.Errorf("flash download: create scratch file: %w", err)
	}
	defer func() {
		_ = f.Close()
		_ = os.Remove(tmp) //nolint:gosec // G104: best-effort scratch cleanup, never the deliverable
	}()

	if err := s.engine.DumpZip(ctx, repo, snapshotID, subfolder, f, mode); err != nil {
		return err
	}
	fi, err := f.Stat()
	if err != nil {
		return fmt.Errorf("flash download: stat scratch file: %w", err)
	}
	if err := recompressFlashZip(f, fi.Size(), dst); err != nil {
		return fmt.Errorf("flash download: recompress: %w", err)
	}
	return nil
}
