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

// ExcludesARoot reports whether restic would leave one of roots out whole
// for pattern, so its part of the backup would hold nothing: the pattern
// matches the root or a folder above it, or does so once its trailing "*"
// and "**" segments, which take every entry of a folder, are dropped. It
// matches the way restic does: a pattern that starts with "/" from the top,
// any other at any depth, with "**" standing for any number of folders.
func ExcludesARoot(pattern string, roots []string) bool {
	if ExcludesEverything(pattern) {
		return len(roots) > 0
	}
	pattern = strings.TrimSpace(pattern)
	if pattern == "" {
		return false
	}
	anchored := strings.HasPrefix(pattern, "/")
	parts := segments(pattern)
	for len(parts) > 0 && (parts[len(parts)-1] == "*" || parts[len(parts)-1] == "**") {
		parts = parts[:len(parts)-1]
	}
	for _, root := range roots {
		segs := segments(root)
		for start := 0; start <= len(segs); start++ {
			if anchored && start > 0 {
				break
			}
			for end := start + 1; end <= len(segs); end++ {
				if matchSegments(parts, segs[start:end]) {
					return true
				}
			}
		}
	}
	return false
}

func segments(p string) []string {
	var out []string
	for _, seg := range strings.Split(path.Clean(p), "/") {
		if seg != "" {
			out = append(out, seg)
		}
	}
	return out
}

// matchSegments reports whether the glob segments in pattern match all of
// segs, with "**" matching any number of them.
func matchSegments(pattern, segs []string) bool {
	if len(pattern) == 0 {
		return len(segs) == 0
	}
	if pattern[0] == "**" {
		for i := 0; i <= len(segs); i++ {
			if matchSegments(pattern[1:], segs[i:]) {
				return true
			}
		}
		return false
	}
	if len(segs) == 0 {
		return false
	}
	ok, _ := path.Match(pattern[0], segs[0])
	return ok && matchSegments(pattern[1:], segs[1:])
}

// DropRootExcludes returns excludes without the patterns that would leave one
// of roots out whole.
func DropRootExcludes(excludes, roots []string) []string {
	var out []string
	for _, p := range excludes {
		if !ExcludesARoot(p, roots) {
			out = append(out, p)
		}
	}
	return out
}
