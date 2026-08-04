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
// PRESENT mount, by consulting the kernel mount table (mountinfoPath). It returns
// true only when the repo itself or one of its ancestor directories is an actual
// mount point — a bare os.Stat / temp-write probe is NOT enough, because an
// unmounted mountpoint directory is usually still writable, which is exactly the
// late-mount case (#55) we must keep protecting.
//
// Remote repos have no local backing store and are never gated on a mount.
//
// On ANY error reading or parsing the mount table it is CONSERVATIVE and returns
// false (destination treated as NOT mounted), so the #55 protection still holds:
// a marker is never cleared on uncertainty.
func destinationMounted(repo string) bool {
	if restic.IsRemoteRepo(repo) {
		return false
	}
	f, err := os.Open(mountinfoPath) //nolint:gosec // G304: mountinfoPath is a fixed package var (/proc/self/mountinfo), overridden only by tests
	if err != nil {
		return false // conservative: cannot prove the mount is present
	}
	defer f.Close() //nolint:errcheck // read-only handle
	mounted := parseMountedDirs(f)

	// Walk repo and each ancestor; if any is a listed mount point, the backing
	// store is present. mount records are kernel paths (forward slashes), so
	// normalise the repo the same way before comparing.
	p := path.Clean(filepath.ToSlash(repo))
	for {
		if mounted[p] {
			return true
		}
		parent := path.Dir(p)
		if parent == p {
			return false // reached the root without a match
		}
		p = parent
	}
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
