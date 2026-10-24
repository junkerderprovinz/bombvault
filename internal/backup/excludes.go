package backup

import (
	"path"
	"strings"
)

// ExcludesEverything reports whether restic would leave the whole backup out
// for pattern: "/" in any spelling, or a glob of only "*" and "**" segments,
// which matches every path. restic still reports such a run as a success,
// with a snapshot of empty folders.
func ExcludesEverything(pattern string) bool {
	pattern = strings.TrimSpace(pattern)
	if pattern == "" {
		return false
	}
	for _, seg := range strings.Split(path.Clean(pattern), "/") {
		if seg != "" && seg != "*" && seg != "**" {
			return false
		}
	}
	return true
}
