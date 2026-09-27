// Package appdatabackup reads the backups the Unraid plugin "Appdata Backup"
// (commifreak/unraid-appdata.backup) writes: one ab_YYYYMMDD_HHMMSS folder per
// run, holding a tar of each container's volumes and the Unraid templates.
package appdatabackup

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/klauspost/compress/zstd"
)

// folderLayout is the plugin's date("Ymd_His") in Go's notation, in the
// server's local time.
const folderLayout = "20060102_150405"

var (
	folderRe  = regexp.MustCompile(`^ab_[0-9]{8}_[0-9]{6}$`)
	archiveRe = regexp.MustCompile(`^(.+)\.tar(\.gz|\.zst)?$`)
)

// Archive is one container's tar in a backup folder.
type Archive struct {
	Container string
	File      string
	Size      int64
}

// Backup is one run of the plugin.
type Backup struct {
	Folder    string
	Path      string
	Time      time.Time
	Archives  []Archive
	Templates []string
}

// Scan finds the backup folders in dir, oldest first. dir may also be one
// backup folder itself. Nothing is written.
func Scan(dir string, loc *time.Location) ([]Backup, error) {
	if b, ok, err := readFolder(dir, loc); ok || err != nil {
		if err != nil {
			return nil, err
		}
		return []Backup{b}, nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var out []Backup
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		b, ok, err := readFolder(filepath.Join(dir, e.Name()), loc)
		if err != nil {
			return nil, err
		}
		if ok {
			out = append(out, b)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Time.Before(out[j].Time) })
	return out, nil
}

func readFolder(dir string, loc *time.Location) (Backup, bool, error) {
	name := filepath.Base(dir)
	if !folderRe.MatchString(name) {
		return Backup{}, false, nil
	}
	at, err := time.ParseInLocation(folderLayout, strings.TrimPrefix(name, "ab_"), loc)
	if err != nil {
		return Backup{}, false, nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return Backup{}, false, err
	}
	b := Backup{Folder: name, Path: dir, Time: at}
	for _, e := range entries {
		if !e.Type().IsRegular() {
			continue
		}
		fn := e.Name()
		if strings.HasPrefix(fn, "my-") && strings.HasSuffix(fn, ".xml") {
			b.Templates = append(b.Templates, fn)
			continue
		}
		m := archiveRe.FindStringSubmatch(fn)
		if m == nil || m[1] == "extra_files" {
			continue
		}
		info, err := e.Info()
		if err != nil {
			return Backup{}, false, err
		}
		b.Archives = append(b.Archives, Archive{Container: m[1], File: fn, Size: info.Size()})
	}
	sort.Slice(b.Archives, func(i, j int) bool { return b.Archives[i].Container < b.Archives[j].Container })
	return b, true, nil
}

// Template returns the saved Unraid template of a container, if the backup
// holds one.
func (b Backup) Template(container string) (string, bool) {
	want := "my-" + container + ".xml"
	for _, t := range b.Templates {
		if t == want {
			raw, err := os.ReadFile(filepath.Join(b.Path, t)) //nolint:gosec // G304: a file of the folder the user picked, found by Scan
			return string(raw), err == nil
		}
	}
	return "", false
}

// Stats counts what an extraction wrote and what it left out.
type Stats struct {
	Files   int64
	Bytes   int64
	Skipped int64
}

// ErrOutsideSource marks an archive that holds nothing below the mapped root.
var ErrOutsideSource = errors.New("the archive holds no path BombVault can map")

// Extract unpacks one archive below dest. The plugin stores absolute host
// paths (tar -P); mapPath turns each into the path the entry gets below dest
// and says whether it is wanted at all. Devices and FIFOs are left out, and
// progress is called with the compressed bytes read so far.
func Extract(ctx context.Context, archive string, dest string, mapPath func(string) (string, bool), progress func(int64)) (Stats, error) {
	f, err := os.Open(archive) //nolint:gosec // G304: an archive Scan found in the folder the user picked
	if err != nil {
		return Stats{}, err
	}
	defer func() { _ = f.Close() }()
	counted := &countingReader{r: f, progress: progress}
	var r io.Reader = counted
	switch {
	case strings.HasSuffix(archive, ".gz"):
		gz, err := gzip.NewReader(counted)
		if err != nil {
			return Stats{}, fmt.Errorf("gzip: %w", err)
		}
		defer func() { _ = gz.Close() }()
		r = gz
	case strings.HasSuffix(archive, ".zst"):
		zr, err := zstd.NewReader(counted)
		if err != nil {
			return Stats{}, fmt.Errorf("zstd: %w", err)
		}
		defer zr.Close()
		r = zr
	}
	return extractTar(ctx, tar.NewReader(r), dest, mapPath)
}

type dirMeta struct {
	path string
	hdr  *tar.Header
}

func extractTar(ctx context.Context, tr *tar.Reader, dest string, mapPath func(string) (string, bool)) (Stats, error) {
	var st Stats
	var dirs []dirMeta
	for {
		if err := ctx.Err(); err != nil {
			return st, err
		}
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return st, fmt.Errorf("read archive: %w", err)
		}
		target, ok := entryTarget(hdr.Name, dest, mapPath)
		if !ok {
			st.Skipped++
			continue
		}
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil { //nolint:gosec // G301: the mode from the archive is applied at the end
				return st, err
			}
			dirs = append(dirs, dirMeta{target, hdr})
		case tar.TypeReg:
			n, err := writeFile(target, tr, hdr)
			if err != nil {
				return st, err
			}
			st.Files++
			st.Bytes += n
		case tar.TypeSymlink:
			if err := makeParent(target); err != nil {
				return st, err
			}
			_ = os.Remove(target)
			if err := os.Symlink(hdr.Linkname, target); err != nil {
				return st, err
			}
			st.Files++
		case tar.TypeLink:
			src, ok := entryTarget(hdr.Linkname, dest, mapPath)
			if !ok {
				st.Skipped++
				continue
			}
			if err := makeParent(target); err != nil {
				return st, err
			}
			_ = os.Remove(target)
			if err := os.Link(src, target); err != nil {
				return st, err
			}
			st.Files++
		default:
			st.Skipped++
		}
	}
	// Directories get their mode and time last: a read-only one would refuse
	// its files, and every file written into one moves its time.
	for i := len(dirs) - 1; i >= 0; i-- {
		applyMeta(dirs[i].path, dirs[i].hdr)
	}
	if st.Files == 0 && st.Skipped > 0 {
		return st, ErrOutsideSource
	}
	return st, nil
}

// entryTarget maps an archive name to its place below dest. Names are cleaned
// as absolute paths first, so "..", a missing leading slash and doubled
// slashes cannot reach outside dest.
func entryTarget(name, dest string, mapPath func(string) (string, bool)) (string, bool) {
	clean := path.Clean("/" + strings.TrimPrefix(name, "./"))
	mapped, ok := mapPath(clean)
	if !ok {
		return "", false
	}
	mapped = path.Clean("/" + strings.TrimPrefix(mapped, filepath.VolumeName(mapped)))
	if mapped == "/" {
		return "", false
	}
	return filepath.Join(dest, filepath.FromSlash(mapped)), true
}

func makeParent(p string) error {
	return os.MkdirAll(filepath.Dir(p), 0o755) //nolint:gosec // G301: a directory the archive did not list gets the usual mode
}

func writeFile(target string, r io.Reader, hdr *tar.Header) (int64, error) {
	if err := makeParent(target); err != nil {
		return 0, err
	}
	_ = os.Remove(target)
	out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600) //nolint:gosec // G304: target was mapped below the staging folder
	if err != nil {
		return 0, err
	}
	n, err := io.Copy(out, r) //nolint:gosec // G110: the archive is the user's own backup, extracted to disk as it was
	if cerr := out.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return n, err
	}
	applyMeta(target, hdr)
	return n, nil
}

// applyMeta puts back mode, owner and time as far as the platform lets it.
// Owner changes need root and fail elsewhere, which only costs the owner.
func applyMeta(target string, hdr *tar.Header) {
	_ = os.Chmod(target, os.FileMode(hdr.Mode).Perm()) //nolint:gosec // G115: tar modes fit in 32 bits
	_ = os.Lchown(target, hdr.Uid, hdr.Gid)
	_ = os.Chtimes(target, hdr.ModTime, hdr.ModTime)
}

type countingReader struct {
	r        io.Reader
	n        int64
	progress func(int64)
}

func (c *countingReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.n += int64(n)
	if c.progress != nil && n > 0 {
		c.progress(c.n)
	}
	return n, err
}
