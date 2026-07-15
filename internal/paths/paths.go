// Package paths provides in-app path containment under the host mount root.
package paths

import (
	"errors"
	"os"
	"path"
	"strings"
)

// ErrTraversal is returned when a sub path would escape the root.
var ErrTraversal = errors.New("paths: sub path escapes the root (traversal)")

// ErrAbsoluteSub is returned when sub is an absolute path.
var ErrAbsoluteSub = errors.New("paths: sub must be a relative path")

// Resolve joins root and sub using slash semantics, cleans the result, and
// verifies the result stays strictly within root. It rejects any sub that is
// absolute or that after cleaning resolves outside root
// (e.g. "../etc" or "a/../../etc").
//
// Paths here are always Linux paths (container-internal), so the path package
// (always slash-separated) is correct regardless of the build OS.
func Resolve(root, sub string) (string, error) {
	// Reject absolute sub paths (start with "/").
	if strings.HasPrefix(sub, "/") {
		return "", ErrAbsoluteSub
	}

	cleanRoot := path.Clean(root)
	joined := cleanRoot + "/" + sub
	cleaned := path.Clean(joined)

	// The cleaned result must be a strict child of cleanRoot (not equal, not a sibling).
	// Append "/" to cleanRoot so /host/user never matches /host/user2/foo.
	prefix := cleanRoot + "/"
	if !strings.HasPrefix(cleaned, prefix) {
		return "", ErrTraversal
	}

	return cleaned, nil
}

// Within reports whether absPath is an absolute path that lies strictly inside
// root (after slash-clean). Used to re-validate stored absolute appdata paths
// before a restore writes to them (defense-in-depth).
func Within(root, absPath string) bool {
	if !strings.HasPrefix(absPath, "/") {
		return false
	}
	cleanRoot := path.Clean(root)
	cleaned := path.Clean(absPath)
	return strings.HasPrefix(cleaned, cleanRoot+"/")
}

// EnsureDir creates path and all parents with mode 0o700.
func EnsureDir(path string) error {
	return os.MkdirAll(path, 0o700)
}

// EnsureDirReadable creates path and all parents, then forces path itself to
// 0o755 so a restore TARGET on a user-visible / synced share (Unraid /mnt/user)
// is readable by the operator's non-root SMB user — root created it, and a 0o700
// dir (EnsureDir's mode) or a strict process umask would otherwise lock them out.
// The explicit Chmod (not just MkdirAll's mode, which a umask can strip) also
// heals a target an earlier version created at 0o700, mirroring how
// ensureDefsDir/makeRepoReadable heal perms on the backup share.
func EnsureDirReadable(path string) error {
	if err := os.MkdirAll(path, 0o755); err != nil { //nolint:gosec // G301: restore target on a user-visible share must be operator-readable
		return err
	}
	return os.Chmod(path, 0o755) //nolint:gosec // G302: see above — must be readable by the non-root share user
}
