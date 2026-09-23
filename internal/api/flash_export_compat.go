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

// fsckRecPattern matches the recovery dumps (FSCK0000.REC) that dosfsck writes
// to the filesystem root when it finds orphaned clusters.
var fsckRecPattern = regexp.MustCompile(`^FSCK\d+\.REC$`)

// flashZipJunkEntry reports whether a zip entry does not belong in a flash
// export. That covers litter from other tools (a stray .git checkout, the
// System Volume Information folder and empty .bmp files Windows leaves on the
// FAT32 stick, fsck recovery dumps) and Unraid's previous/ folder. Only empty
// .bmp files go, because a syslinux background image is a .bmp too. previous/
// holds the prior OS version for rollback; Unraid recreates it on the next
// upgrade, and it can take up a third of the export.
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

// recompressFlashZip rewrites the zip from restic's `dump -a zip`, which
// stores every entry uncompressed, into dst with every entry deflated, and
// drops junk entries on the way. The Unraid USB Creator fails part-way
// through extracting an all-stored archive (#136), and restic has no flag to
// choose the compression method.
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
// a new header instead of reusing f.FileHeader, because the source's Extra
// field may hold a Zip64 record that conflicts with the one zip.Writer adds
// for the new size.
func copyZipEntry(zw *zip.Writer, f *zip.File) error {
	hdr := &zip.FileHeader{
		Name:     f.Name,
		Modified: f.Modified,
		Comment:  f.Comment,
		Method:   zip.Deflate,
	}
	if f.FileInfo().IsDir() || f.UncompressedSize64 == 0 {
		hdr.Method = zip.Store
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
	// Copying exactly the declared size stops a corrupt entry that inflates
	// past its header (gosec G110) and reports one that falls short.
	if _, err := io.CopyN(w, r, int64(f.UncompressedSize64)); err != nil { //nolint:gosec // G115: a real zip entry size (flash backups are GBs at most), nowhere near MaxInt64
		return fmt.Errorf("copy %d declared bytes: %w", f.UncompressedSize64, err)
	}
	return nil
}

// dumpFlashZipCompat runs engine.DumpZip and passes its output through
// recompressFlashZip on the way to dst. Reading a zip needs random access,
// which the HTTP response or an age pipe cannot give, so the dump is staged
// in a scratch file under DataDir that is removed afterwards.
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
