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

// Resolve joins root and sub, cleans the result and checks that it lies
// strictly inside root. It rejects an absolute sub and one that escapes root
// after cleaning, such as "../etc" or "a/../../etc".
//
// The paths are container-internal Linux paths, so the path package is right
// on any build OS.
func Resolve(root, sub string) (string, error) {
	if strings.HasPrefix(sub, "/") {
		return "", ErrAbsoluteSub
	}

	cleanRoot := path.Clean(root)
	joined := cleanRoot + "/" + sub
	cleaned := path.Clean(joined)

	// The trailing slash keeps /host/user from matching /host/user2/foo.
	prefix := cleanRoot + "/"
	if !strings.HasPrefix(cleaned, prefix) {
		return "", ErrTraversal
	}

	return cleaned, nil
}

// Within reports whether absPath is absolute and lies strictly inside root.
// Restore uses it to re-check stored appdata paths before writing to them.
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

// EnsureDirReadable creates path and sets it to 0o755, so a restore target on
// a user share such as /mnt/user stays readable for the operator's non-root
// SMB user. The Chmod covers what a umask strips from MkdirAll's mode, and an
// existing 0o700 directory.
func EnsureDirReadable(path string) error {
	if err := os.MkdirAll(path, 0o755); err != nil { //nolint:gosec // G301: restore target on a user-visible share must be operator-readable
		return err
	}
	return os.Chmod(path, 0o755) //nolint:gosec // G302: must be readable by the non-root share user
}
