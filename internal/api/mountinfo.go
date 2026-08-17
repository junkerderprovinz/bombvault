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

// mountinfoPath is the source of mount records. It is a package var so tests can
// point it at a fixture; production reads the live kernel table. The container
// runs with rslave propagation, so a mount that appears on the host (e.g. an
// Unassigned Devices disk that mounts after Docker starts) becomes visible here.
var mountinfoPath = "/proc/self/mountinfo"

// destinationMounted reports whether the LOCAL repo path sits on a genuinely
// PRESENT per-share/per-disk mount, by consulting the kernel mount table
// (mountinfoPath). A bare os.Stat / temp-write probe is NOT enough, because an
// unmounted mountpoint directory is usually still writable, which is exactly the
// late-mount case (#55) we must keep protecting.
//
// The discriminator: the repo counts as mounted only when some mount point M is
// the repo itself OR an ancestor directory of it, AND M is a STRICT/PROPER
// DESCENDANT of HostMountRoot (path.Clean(s.cfg.HostMountRoot), e.g. /host/user).
// In a real /proc/self/mountinfo "/" is ALWAYS a mount point, and the broad host
// bind (HostMountRoot itself, from HostSourceRoot) is too; neither may satisfy
// the check, or every local repo would look mounted and the #55 guard would be
// defeated. #55 protects paths served by a DISTINCT per-disk/per-share mount BELOW
// the broad host bind, and that mount only appears in mountinfo when actually
// mounted. So: EXCLUDE "/", EXCLUDE HostMountRoot itself, and exclude any mount at
// or above HostMountRoot. Examples (HostMountRoot=/host/user):
//   - UD disk mounted at /host/user/disks/X → repo under it is mounted → self-heal.
//   - genuinely-unmounted share: only "/" and /host/user present → not mounted → #55.
//   - array-default repo /host/user/bombvault/…: nearest mount is the broad bind
//     itself → not mounted → stays protected (safe/over-protective, acceptable).
//
// Remote repos have no local backing store and are never gated on a mount.
//
// This discriminator keys off s.cfg.HostMountRoot ALONE — it never reads
// HostSourceRoot, and makes no assumption about how the two relate. That
// makes it correct under both Unraid's split-root default (HostSourceRoot=/mnt
// translated to HostMountRoot=/host/user) and the generic/TrueNAS identity-root
// default (HostSourceRoot == HostMountRoot at a real path, e.g. both /data —
// no path translation): either way it excludes "/" and HostMountRoot itself
// exactly as documented above. This is scoped to an identity root at a real,
// non-root path; HostMountRoot=="/" itself is a distinct, more extreme case
// (it would collide with the exclude-root rule) and is not handled here.
//
// On ANY error reading or parsing the mount table it is CONSERVATIVE and returns
// false (destination treated as NOT mounted), so the #55 protection still holds:
// a marker is never cleared on uncertainty.
func (s *Service) destinationMounted(repo string) bool {
	if restic.IsRemoteRepo(repo) {
		return false
	}
	f, err := os.Open(mountinfoPath) //nolint:gosec // G304: mountinfoPath is a fixed package var (/proc/self/mountinfo), overridden only by tests
	if err != nil {
		return false // conservative: cannot prove the mount is present
	}
	defer f.Close() //nolint:errcheck // read-only handle
	mounted := parseMountedDirs(f)

	// Walk repo and each ancestor; a match counts only when the ancestor is a
	// listed mount point AND a proper descendant of HostMountRoot. mount records
	// are kernel paths (forward slashes), so normalise both the same way.
	root := path.Clean(filepath.ToSlash(s.cfg.HostMountRoot))
	p := path.Clean(filepath.ToSlash(repo))
	for {
		if mounted[p] && isStrictSubpath(root, p) {
			return true
		}
		parent := path.Dir(p)
		if parent == p {
			return false // reached the filesystem root without a qualifying mount
		}
		p = parent
	}
}

// isStrictSubpath reports whether p is a proper (strict) descendant of root: root
// is a path-prefix of p and p != root. Both must already be path.Clean'd. This is
// what excludes "/" and HostMountRoot itself from counting as the backing mount.
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

// parseMountedDirs reads /proc/self/mountinfo-format records from r and returns
// the set of mount-point directories (field 5 of each record), octal-unescaped.
// Malformed lines are skipped. Kept as an inner function taking an io.Reader so
// tests can feed a fixture without touching the filesystem.
func parseMountedDirs(r io.Reader) map[string]bool {
	out := make(map[string]bool)
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		// mountinfo: id, parent, major:minor, root, MOUNT-POINT, opts, ...
		if len(fields) < 5 {
			continue
		}
		out[path.Clean(unescapeOctal(fields[4]))] = true
	}
	return out
}

// unescapeOctal decodes the \NNN octal escapes the kernel uses in mountinfo for
// space (\040), tab (\011), newline (\012) and backslash (\134) within paths.
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
