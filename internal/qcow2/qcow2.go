// Package qcow2 reads and rewrites the backing file a qcow2 image names in its
// header. A disk with external snapshots is a chain of such images, and only
// the whole chain holds the guest's data.
package qcow2

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
)

const magic = 0x514649fb

const (
	extEnd           = 0x00000000
	extBackingFormat = 0xe2792aca
)

// incompatDataFile is the incompatible feature bit of an image that keeps the
// guest's data in an external file.
const incompatDataFile = 1 << 2

// maxNameLen is the longest backing file name qemu accepts.
const maxNameLen = 1023

// ErrNotQCOW2 is returned for a file that does not start with the qcow2 magic.
var ErrNotQCOW2 = errors.New("not a qcow2 image")

// Header is the part of a qcow2 header that ties an image to its backing file.
type Header struct {
	// BackingFile is the name as stored: absolute, relative to the image's
	// folder, or empty for an image without one.
	BackingFile string
	// BackingFormat is the format the image records for its backing file,
	// empty when it records none.
	BackingFormat string

	nameOffset  int64
	clusterSize int64
}

// ReadHeader reads the header of the qcow2 image at file.
func ReadHeader(file string) (Header, error) {
	f, err := os.Open(file) //nolint:gosec // G304: callers pass VM disk paths below the host mount
	if err != nil {
		return Header{}, err
	}
	defer f.Close() //nolint:errcheck // read only
	return readHeader(f)
}

func readHeader(r io.ReaderAt) (Header, error) {
	var b [104]byte
	n, err := r.ReadAt(b[:], 0)
	if err != nil && !errors.Is(err, io.EOF) {
		return Header{}, err
	}
	if n < 4 || binary.BigEndian.Uint32(b[0:]) != magic {
		return Header{}, ErrNotQCOW2
	}
	if n < 72 {
		return Header{}, errors.New("the qcow2 header is cut short")
	}
	version := binary.BigEndian.Uint32(b[4:])
	nameOffset := int64(binary.BigEndian.Uint64(b[8:])) //nolint:gosec // G115: checked against the cluster size below
	nameLen := int64(binary.BigEndian.Uint32(b[16:]))
	clusterBits := binary.BigEndian.Uint32(b[20:])
	if version != 2 && version != 3 {
		return Header{}, fmt.Errorf("qcow2 version %d is not supported", version)
	}
	if clusterBits < 9 || clusterBits > 21 {
		return Header{}, fmt.Errorf("the qcow2 header gives an invalid cluster size (2^%d bytes)", clusterBits)
	}
	h := Header{nameOffset: nameOffset, clusterSize: int64(1) << clusterBits}
	extStart := int64(72)
	if version == 3 {
		if n < 104 {
			return Header{}, errors.New("the qcow2 header is cut short")
		}
		if binary.BigEndian.Uint64(b[72:])&incompatDataFile != 0 {
			return Header{}, errors.New("the image keeps its data in an external data file")
		}
		extStart = int64(binary.BigEndian.Uint32(b[100:]))
		if extStart < 104 || extStart > h.clusterSize {
			return Header{}, errors.New("the qcow2 header gives an invalid header length")
		}
	}
	if nameOffset != 0 && (nameLen == 0 || nameLen > maxNameLen || nameOffset < extStart || nameOffset+nameLen > h.clusterSize) {
		return Header{}, errors.New("the backing file name lies outside the qcow2 header")
	}

	// As in qemu, the extensions end at the backing file name when there is
	// one: a version 2 image may have the name right after the header and no
	// end marker.
	end := h.clusterSize
	if nameOffset != 0 {
		end = nameOffset
	}
	for off := extStart; off < end; {
		var ext [8]byte
		if _, err := r.ReadAt(ext[:], off); err != nil {
			return Header{}, fmt.Errorf("read the qcow2 header extensions: %w", err)
		}
		typ := binary.BigEndian.Uint32(ext[0:])
		size := int64(binary.BigEndian.Uint32(ext[4:]))
		off += 8
		if off > end || size > end-off {
			return Header{}, errors.New("a qcow2 header extension runs past the header")
		}
		if typ == extEnd {
			break
		}
		if typ == extBackingFormat {
			v := make([]byte, size)
			if _, err := r.ReadAt(v, off); err != nil {
				return Header{}, fmt.Errorf("read the backing format: %w", err)
			}
			h.BackingFormat = string(v)
		}
		off += (size + 7) &^ 7
	}

	if nameOffset != 0 {
		v := make([]byte, nameLen)
		if _, err := r.ReadAt(v, nameOffset); err != nil {
			return Header{}, fmt.Errorf("read the backing file name: %w", err)
		}
		h.BackingFile = string(v)
	}
	return h, nil
}

// SetBackingFile makes the image at file name backing as its backing file in
// place of the one it names. The backing format stays as it is, so backing
// has to be a copy of the file named before. The name goes where the old one
// was: the format keeps the rest of the first cluster free for it.
func SetBackingFile(file, backing string) error {
	f, err := os.OpenFile(file, os.O_RDWR, 0) //nolint:gosec // G304: callers pass restored VM disk paths
	if err != nil {
		return err
	}
	h, err := readHeader(f)
	if err == nil {
		err = writeBackingName(f, h, backing)
	}
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	return err
}

func writeBackingName(f *os.File, h Header, backing string) error {
	switch {
	case h.BackingFile == "":
		return errors.New("the image has no backing file to replace")
	case backing == "" || len(backing) > maxNameLen || h.nameOffset+int64(len(backing)) > h.clusterSize:
		return fmt.Errorf("the backing file name %q does not fit into the qcow2 header", backing)
	}
	name := make([]byte, max(len(backing), len(h.BackingFile)))
	copy(name, backing)
	if _, err := f.WriteAt(name, h.nameOffset); err != nil {
		return err
	}
	var size [4]byte
	binary.BigEndian.PutUint32(size[:], uint32(len(backing))) //nolint:gosec // G115: at most maxNameLen
	_, err := f.WriteAt(size[:], 16)
	return err
}

// Link says that Image reads through to File, whose format is Format. Both
// are paths this process opens the files by.
type Link struct {
	Image  string
	File   string
	Format string
}

// Chain follows the backing chain of the qcow2 image that the VM host knows
// as host and returns one link per step, down to the file that has no backing
// file of its own. local maps a host path to the path this process opens it
// by and reports false for one it cannot reach. A chain that cannot be
// followed to its end is an error, since a copy of part of it does not
// restore the disk.
func Chain(host string, local func(string) (string, bool)) ([]Link, error) {
	host = path.Clean(host)
	img, ok := local(host)
	if !ok {
		return nil, fmt.Errorf("%s lies outside the Host Data mount", host)
	}
	seen := map[string]bool{host: true}
	var links []Link
	for {
		h, err := ReadHeader(img)
		if err != nil {
			return nil, fmt.Errorf("read the header of %s: %w", host, err)
		}
		if h.BackingFile == "" {
			return links, nil
		}
		if hasProtocol(h.BackingFile) {
			return nil, fmt.Errorf("%s names its backing file as %q, which is not a file path", host, h.BackingFile)
		}
		next := BackingPath(host, h.BackingFile)
		if seen[next] {
			return nil, fmt.Errorf("the backing chain of %s loops back to %s", host, next)
		}
		seen[next] = true
		file, ok := local(next)
		if !ok {
			return nil, fmt.Errorf("%s reads through to %s, which lies outside the Host Data mount", host, next)
		}
		format := h.BackingFormat
		if format == "" {
			format, err = probe(file)
		} else {
			_, err = os.Stat(file)
		}
		if errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("%s reads through to %s, which is missing", host, next)
		}
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", next, err)
		}
		links = append(links, Link{Image: img, File: file, Format: format})
		switch format {
		case "qcow2":
			host, img = next, file
		case "raw":
			return links, nil
		default:
			return nil, fmt.Errorf("%s reads through to %s, a %s image, which BombVault cannot follow", host, next, format)
		}
	}
}

// BackingPath is the host path of the backing file the image at host names
// as name.
func BackingPath(host, name string) string {
	if strings.HasPrefix(name, "/") {
		return path.Clean(name)
	}
	return path.Join(path.Dir(host), name)
}

// Repoint makes the qcow2 image at file, which the VM host knows as host,
// read through to backing, a host path, unless its header leads there
// already. A relative name stays relative.
func Repoint(file, host, backing string) error {
	h, err := ReadHeader(file)
	if err != nil {
		return err
	}
	if h.BackingFile == "" {
		return errors.New("the image has no backing file")
	}
	backing = path.Clean(backing)
	if BackingPath(host, h.BackingFile) == backing {
		return nil
	}
	name := backing
	if !strings.HasPrefix(h.BackingFile, "/") {
		rel, err := filepath.Rel(filepath.FromSlash(path.Dir(host)), filepath.FromSlash(backing))
		if err != nil {
			return err
		}
		name = filepath.ToSlash(rel)
	}
	return SetBackingFile(file, name)
}

// hasProtocol reports whether qemu reads name as a protocol such as nbd: or
// json: rather than a file: a colon before the first slash.
func hasProtocol(name string) bool {
	i := strings.IndexAny(name, ":/")
	return i >= 0 && name[i] == ':'
}

// probe tells a qcow2 file from a raw one the way qemu does for a backing
// file whose format the image does not record.
func probe(file string) (string, error) {
	f, err := os.Open(file) //nolint:gosec // G304: a backing file below the host mount
	if err != nil {
		return "", err
	}
	defer f.Close() //nolint:errcheck // read only
	var b [4]byte
	if n, err := f.ReadAt(b[:], 0); n == 4 && binary.BigEndian.Uint32(b[:]) == magic {
		return "qcow2", nil
	} else if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	return "raw", nil
}
