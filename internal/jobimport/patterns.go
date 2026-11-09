package jobimport

import "strings"

// IsAnchored reports whether an exclude names one path rather than a name
// that can turn up anywhere.
func IsAnchored(p string) bool {
	return strings.HasPrefix(p, "/") || hasDrive(p)
}

func hasDrive(p string) bool {
	return len(p) >= 2 && p[1] == ':' && (p[0]|0x20 >= 'a' && p[0]|0x20 <= 'z')
}

func hasWildcard(p string) bool {
	return strings.ContainsAny(p, "*?[")
}

// namePattern translates a pattern of Duplicati or Duplicacy, whose star also
// matches slashes, into a restic pattern without a slash, which restic tries
// against every part of a path. Only the shapes that mean the same thing
// there are taken: a star followed by a name ("*.tmp", "*cache*") and a name
// between "*/" and the end ("*/node_modules"). A pattern that would match
// everything is never taken, since an include in front of it is what made it
// safe in the other tool.
func namePattern(p string) (string, bool) {
	p = strings.TrimSuffix(p, "/")
	if rest, ok := strings.CutPrefix(p, "*/"); ok {
		if rest != "" && !strings.Contains(rest, "/") && !hasWildcard(rest) {
			return rest, true
		}
		return "", false
	}
	if !strings.HasPrefix(p, "*") || strings.Contains(p, "/") || strings.Trim(p, "*") == "" {
		return "", false
	}
	return p, true
}
