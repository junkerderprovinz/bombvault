package api

import (
	"context"
	"path"
	"strings"

	"github.com/junkerderprovinz/bombvault/internal/restic"
)

// pickedPath is the restic call that restores one picked path: it starts at
// subtree, or at the snapshot root when subtree is empty, and includes only
// the pick below it.
type pickedPath struct {
	subtree string
	include string
}

// pickedSteps is one restic call per pick, in the order of picks. In place
// every pick goes back to its own place. Into a folder the restore starts at
// the deepest directory holding all of them, so a lone file lands as
// <target>/<name>, a lone folder as <target>/<folder> with its content, and
// picks from different folders keep only their path below the folder they
// share.
func pickedSteps(picks []string, inPlace bool) []pickedPath {
	subtree := ""
	if !inPlace && len(picks) > 0 {
		subtree = sharedDir(picks)
	}
	out := make([]pickedPath, len(picks))
	for i, p := range picks {
		// The picks are paths someone ticked, not patterns, and --include is a
		// glob: a raw "/Inception (2010) [1080p]" restored no files and restic
		// still exited 0. The subtree travels as a selector and stays raw.
		out[i] = pickedPath{subtree: subtree, include: restic.EscapeGlob(strings.TrimPrefix(p, subtree))}
	}
	return out
}

// sharedDir is the deepest directory that holds every one of picks, or "" at
// the top of the snapshot, where a plain include already lands each pick
// below the target as it is. It is always a directory, which restic needs for
// the root of a subtree restore.
func sharedDir(picks []string) string {
	shared := strings.Split(path.Dir(picks[0]), "/")
	for _, p := range picks[1:] {
		dir := strings.Split(path.Dir(p), "/")
		n := 0
		for n < len(shared) && n < len(dir) && shared[n] == dir[n] {
			n++
		}
		shared = shared[:n]
	}
	d := strings.Join(shared, "/")
	if d == "/" || d == "." {
		return ""
	}
	return d
}

// previewStep is the dry run of restoring p into target.
func (p pickedPath) previewStep(snapshotID, target string) restic.PreviewStep {
	return restic.PreviewStep{SnapshotID: snapshotID, Subtree: p.subtree, Target: target, Include: p.include}
}

func (s *Service) restorePicked(ctx context.Context, repo, snapshotID string, p pickedPath, target string, m restic.Mode) error {
	if p.subtree == "" {
		return s.engine.RestoreInclude(ctx, repo, snapshotID, p.include, target, m)
	}
	return s.engine.RestoreSubtreeInclude(ctx, repo, snapshotID, p.subtree, p.include, target, m)
}
