package api

import (
	"bufio"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/junkerderprovinz/bombvault/internal/restic"
)

// mountinfoPath is a var so tests can point it at a fixture. Host Data is
// mounted with slave propagation, so a disk the host mounts after Docker starts
// still shows up in this table.
var mountinfoPath = "/proc/self/mountinfo"

// destinationMounted reports whether a local repo sits on a mount of its own
// below HostMountRoot. Stat or a test write cannot answer that, because an
// unmounted mountpoint directory is usually still writable.
//
// A mount point counts when it is the repo or one of its ancestors and lies
// strictly below HostMountRoot. "/" and the host bind at HostMountRoot are
// always in the table and would make every repo look mounted. With
// HostMountRoot=/host/user:
//   - a disk mounted at /host/user/disks/X: a repo below it is mounted
//   - an unmounted share: only "/" and /host/user are listed, so not mounted
//   - a repo at /host/user/bombvault: the nearest mount is the host bind, so
//     it is treated as not mounted, which errs on the safe side
//
// Only HostMountRoot is consulted, so this works both when HostSourceRoot is
// translated (Unraid: /mnt becomes /host/user) and when the two are the same
// path, such as /data. HostMountRoot "/" is not supported.
//
// Remote repos report false. So does any error reading the mount table, which
// keeps a not-mounted marker from being cleared on uncertainty.
func (s *Service) destinationMounted(repo string) bool {
	if restic.IsRemoteRepo(repo) {
		return false
	}
	f, err := os.Open(mountinfoPath) //nolint:gosec // G304: mountinfoPath is a fixed package var (/proc/self/mountinfo), overridden only by tests
	if err != nil {
		return false
	}
	defer f.Close() //nolint:errcheck // read-only handle
	mounted := parseMountedDirs(f)

	// Mount records use forward slashes, so both paths are normalised the same
	// way.
	root := path.Clean(filepath.ToSlash(s.cfg.HostMountRoot))
	p := path.Clean(filepath.ToSlash(repo))
	for {
		if mounted[p] && isStrictSubpath(root, p) {
			return true
		}
		parent := path.Dir(p)
		if parent == p {
			return false
		}
		p = parent
	}
}

// isStrictSubpath reports whether p lies below root and is not root itself.
// Both paths must already be cleaned.
func isStrictSubpath(root, p string) bool {
	if p == root {
		return false
	}
	prefix := root
	if prefix != "/" {
		prefix += "/"
	}
	return strings.HasPrefix(p, prefix)
}

// parseMountedDirs returns the set of mount points in mountinfo records read
// from r. Malformed lines are skipped.
func parseMountedDirs(r io.Reader) map[string]bool {
	out := make(map[string]bool)
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		// id, parent, major:minor, root, mount point, options, ...
		if len(fields) < 5 {
			continue
		}
		out[path.Clean(unescapeOctal(fields[4]))] = true
	}
	return out
}

// unescapeOctal decodes the \NNN escapes the kernel writes into mountinfo
// paths for space, tab, newline and backslash.
func unescapeOctal(s string) string {
	if !strings.ContainsRune(s, '\\') {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+3 < len(s) &&
			s[i+1] >= '0' && s[i+1] <= '7' &&
			s[i+2] >= '0' && s[i+2] <= '7' &&
			s[i+3] >= '0' && s[i+3] <= '7' {
			b.WriteByte((s[i+1]-'0')<<6 | (s[i+2]-'0')<<3 | (s[i+3] - '0'))
			i += 3
			continue
		}
		b.WriteByte(s[i])
	}
	return b.String()
}
