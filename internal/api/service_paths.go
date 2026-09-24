package api

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/junkerderprovinz/bombvault/internal/model"
)

// toContainerPath translates a host path under HostSourceRoot to its
// container-visible equivalent under HostMountRoot (the broad Host Data
// mount, e.g. /mnt → /host/user). It returns ("", false) when the host path
// is not reachable through the mount. Appdata and VM disk paths go through
// it; NVRAM travels over SSH instead (see BackupVM/RestoreVM).
func (s *Service) toContainerPath(host string) (string, bool) {
	srcRoot := path.Clean(s.cfg.HostSourceRoot)
	mountRoot := path.Clean(s.cfg.HostMountRoot)
	p := path.Clean(host)
	if p == srcRoot {
		return mountRoot, true
	}
	if rest := strings.TrimPrefix(p, srcRoot+"/"); rest != p {
		return mountRoot + "/" + rest, true
	}
	return "", false // not reachable through the mount
}

// ExcludePreview is one exclude line resolved against a container's live mounts:
// Resolved is the restic --exclude pattern that will actually be used, Status is
// how it was derived, Matches reports whether it would exclude anything in this
// container's backup (so the UI can warn on a line that matches nothing).
type ExcludePreview struct {
	Raw      string `json:"raw"`
	Resolved string `json:"resolved"`
	Status   string `json:"status"` // "basename" | "translated" | "passthrough"
	Matches  bool   `json:"matches"`
}

// resolveExcludeLine turns one raw user line into a restic --exclude pattern.
// No slash → verbatim (restic matches a bare name at any depth). A line under a
// container mount Destination → translated through that mount's Source +
// toContainerPath into the exact anchored path restic stored. Anything else →
// verbatim (advanced host/glob patterns), never silently dropped.
func (s *Service) resolveExcludeLine(line string, in model.Inspect) (pattern, status string) {
	line = strings.TrimSpace(line)
	if line == "" {
		return "", ""
	}
	if !strings.Contains(line, "/") {
		return line, "basename"
	}
	clean := path.Clean(line)
	var bestSrc, bestDest string
	for _, m := range in.Mounts {
		d := path.Clean(m.Destination)
		if d == "" || d == "/" || m.Source == "" {
			continue
		}
		if clean == d || strings.HasPrefix(clean, d+"/") {
			if len(d) > len(bestDest) {
				bestDest, bestSrc = d, m.Source
			}
		}
	}
	if bestDest != "" {
		host := path.Clean(bestSrc + strings.TrimPrefix(clean, bestDest))
		if cp, ok := s.toContainerPath(host); ok {
			// Escape the mount-derived head and leave the user's tail as the glob they
			// meant. The user wrote "/config/Cache"; everything in front of it comes from
			// the container's bind source, a folder name nobody wrote as a pattern. Left
			// unescaped, a mount at ".../Plex [Media]" gives an --exclude that cannot
			// match its own folder, so the branch the editor previews as excluded is
			// backed up on every run, and an unmatched bracket in a mount name fails the
			// whole backup. Escaping the whole line would take the glob away from the
			// half the user owns, where "*" and "?" are the point.
			if cpHead, headOK := s.toContainerPath(path.Clean(bestSrc)); headOK {
				return escapeGlobLiteral(cpHead) + strings.TrimPrefix(clean, bestDest), "translated"
			}
			return cp, "translated"
		}
	}
	return line, "passthrough"
}

// resolveExcludePatterns maps each raw user line through resolveExcludeLine and
// returns the resolved restic --exclude patterns (empty lines dropped). This is
// what feeds BackupDeps.Excludes for a container backup.
func (s *Service) resolveExcludePatterns(raw []string, in model.Inspect) []string {
	var out []string
	for _, line := range raw {
		pattern, status := s.resolveExcludeLine(line, in)
		if status == "" || pattern == "" {
			continue // blank line
		}
		out = append(out, pattern)
	}
	return out
}

// isUnderAny reports whether path p equals, or lives under, one of roots.
func isUnderAny(p string, roots []string) bool {
	for _, root := range roots {
		if p == root || strings.HasPrefix(p, root+"/") {
			return true
		}
	}
	return false
}

// previewExcludes resolves each non-empty raw line against the live inspect and
// reports, per line, the resolved --exclude pattern and whether it would match
// anything in this container's backup (effective = the volumes actually backed
// up). A basename matches at any depth; a translated path matches only when it
// is under a backed-up volume; a passthrough is reported as matching nothing.
// The user's original text round-trips in Raw.
func (s *Service) previewExcludes(raw []string, in model.Inspect, effective []string) []ExcludePreview {
	var out []ExcludePreview
	for _, line := range raw {
		if strings.TrimSpace(line) == "" {
			continue
		}
		pattern, status := s.resolveExcludeLine(line, in)
		matches := status == "basename" || (status == "translated" && isUnderAny(pattern, effective))
		out = append(out, ExcludePreview{
			Raw:      line,
			Resolved: pattern,
			Status:   status,
			Matches:  matches,
		})
	}
	return out
}

// resolveAppdataPaths returns the container-visible paths to back up for a
// container. Docker reports bind-mount sources as host paths (e.g.
// /mnt/user/appdata/<x>/data), and BombVault reaches them only through the
// broad host mount (HostSourceRoot mounted at HostMountRoot, e.g. host /mnt
// at container /host/user, so host /mnt/user/appdata/x is reachable at
// /host/user/user/appdata/x). Every matched bind source is translated from
// the host root to the container mount root, so the real, correctly cased
// path is backed up rather than a guess. A bind is kept when its host
// source contains any of the configured data-root segments
// (s.cfg.DataRootSegments, default ["appdata"], see config.DataRootSegments)
// as a full path segment, or when the container carries a truthy
// "bombvault.data" label (see bombvaultDataLabelTruthy), which takes in
// every bind mount regardless of segment: the documented escape hatch for a
// layout the segment filter does not catch (e.g. "/srv/plex/config").
// Media libraries, the flash, /etc/localtime and other non-matching shares
// are skipped.
//
// A named-volume mount (Type=="volume") is always kept, without the segment
// filter, because a named volume is persistent by construction. Its
// resolved host path (Source, filled in by dockercli's Inspect from the
// daemon's report or a VolumeInspect fallback) goes through the same
// translate-and-check as a bind source, so an unreachable volume mountpoint
// is skipped like an unreachable bind, and a volume that resolves to the
// same container path as a bind already recorded is deduped. On non-Unraid
// hosts this is the common case: a container using only Docker Compose
// named volumes.
//
// Every candidate is deduplicated against every other by cleaned absolute
// container path.
//
// Fallback (nothing matched above): the platform's conventional appdata
// path for <name> (Unraid: /mnt/user/appdata/<name>; generic: none),
// translated if reachable; see platform.Platform.AppdataFallback.
func (s *Service) resolveAppdataPaths(name string, in model.Inspect) []string {
	mountRoot := path.Clean(s.cfg.HostMountRoot) // its container path, e.g. /host/user

	segments := s.cfg.DataRootSegments
	labelOverride := bombvaultDataLabelTruthy(in.Config.Labels)

	var out []string
	seen := map[string]bool{}
	for _, m := range in.Mounts {
		if m.Type == "volume" {
			if m.Source == "" {
				continue // daemon (and the VolumeInspect fallback) couldn't resolve it
			}
			if container, ok := s.toContainerPath(m.Source); ok && !seen[container] {
				out = append(out, container)
				seen[container] = true
			}
			continue
		}
		if m.Source == "" {
			continue
		}
		if !matchesAnyDataRootSegment(path.Clean(m.Source), segments) && !labelOverride {
			continue // no configured data-root segment, and no per-container override
		}
		if container, ok := s.toContainerPath(m.Source); ok && !seen[container] {
			out = append(out, container)
			seen[container] = true
		}
	}

	// The Docker Compose project working directory belongs to the stack and is
	// backed up once per project by backupStackDir, not once per member. restic
	// deduplicates the stored bytes, so a five-service stack costs one copy on
	// disk, but not the reading: every member would walk, chunk and hash the
	// whole project directory on every run, and the CPU cost would scale with
	// the number of services (#189).
	//
	// A member whose only data is the project directory ends up with no paths
	// of its own. The data is in the stack's snapshot, and stackDirFor lets the
	// UI and the restore path say so.

	if len(out) == 0 {
		// Last resort: the platform's conventional appdata dir for this container,
		// but only if it exists. A container with no appdata mount, no such folder,
		// and no platform convention (empty AppdataFallback, e.g. on generic) is
		// stateless: default to an empty selection (config-only backup) rather
		// than a phantom folder that shows as selected yet backs up nothing.
		if hostCand := s.platformFn().AppdataFallback(mountRoot, name); hostCand != "" {
			cand, ok := s.toContainerPath(hostCand)
			if !ok {
				cand = path.Join(mountRoot, "appdata", name)
			}
			if _, err := os.Stat(cand); err == nil { //nolint:gosec // G703: cand is HostMountRoot + "appdata" + a validated container name, not raw user input
				out = append(out, cand)
			}
		}
	}
	return out
}

// hasSegment reports whether slash-separated path p contains seg as a full path
// segment (so "/mnt/user/appdata/x" matches "appdata" but "/mnt/appdataX" does not).
func hasSegment(p, seg string) bool {
	for _, s := range strings.Split(p, "/") {
		if s == seg {
			return true
		}
	}
	return false
}

// matchesAnyDataRootSegment reports whether path p contains any of the given
// data-root segments as a full path segment (see config.DataRootSegments).
func matchesAnyDataRootSegment(p string, segments []string) bool {
	for _, seg := range segments {
		if hasSegment(p, seg) {
			return true
		}
	}
	return false
}

// bombvaultDataLabelTruthy reports whether a container opted all of its bind
// mounts into resolveAppdataPaths via the "bombvault.data" label, the
// documented per-container escape hatch for a data layout the configured
// segment filter does not catch (e.g. "/srv/plex/config"). Truthy means the
// label is present and its trimmed value is neither empty nor "false"
// (case-insensitive), so "true", "1", "yes" or any other such value opts
// in. A container without the label keeps the Unraid-default behaviour.
func bombvaultDataLabelTruthy(labels map[string]string) bool {
	v, ok := labels["bombvault.data"]
	if !ok {
		return false
	}
	v = strings.TrimSpace(v)
	return v != "" && !strings.EqualFold(v, "false")
}

// composeProjectDataDir reads the standard Docker Compose
// "com.docker.compose.project.working_dir" label, present on every
// container Compose creates, off a container's Config.Labels. It returns
// ("", false) when the label is absent or empty after trimming.
func composeProjectDataDir(labels map[string]string) (string, bool) {
	dir := strings.TrimSpace(labels["com.docker.compose.project.working_dir"])
	if dir == "" {
		return "", false
	}
	return dir, true
}

// toHostPath is the inverse of toContainerPath: it maps a container-visible path
// under HostMountRoot back to its host path under HostSourceRoot (e.g.
// /host/user/appdata/x → /mnt/appdata/x). Returns the input unchanged when it is
// not under the mount root.
func (s *Service) toHostPath(cp string) string {
	mountRoot := path.Clean(s.cfg.HostMountRoot)
	srcRoot := path.Clean(s.cfg.HostSourceRoot)
	p := path.Clean(cp)
	if p == mountRoot {
		return srcRoot
	}
	if rest := strings.TrimPrefix(p, mountRoot+"/"); rest != p {
		return srcRoot + "/" + rest
	}
	return cp
}

// MountInfo describes one of a container's bind mounts for the backup-folder
// selector in the UI.
type MountInfo struct {
	Source    string `json:"source"`    // host path (shown to the user)
	Dest      string `json:"dest"`      // in-container mount point
	Selected  bool   `json:"selected"`  // currently included in the backup
	IsAppdata bool   `json:"isAppdata"` // auto-detected appdata default
	Reachable bool   `json:"reachable"` // reachable under the host mount (backable)
}

// CustomPath is a selected backup folder that does not correspond to a current
// bind mount (a manually added path, or an appdata folder for a container whose
// mount is gone). Exists reports whether it is still present under the host mount,
// so the UI can flag a stored-but-missing path ("no data folder detected")
// instead of showing it as a selected folder that backs up nothing (issue #115).
type CustomPath struct {
	Path   string `json:"path"`   // host path (shown to the user)
	Exists bool   `json:"exists"` // still present under the host mount
}

// ContainerMounts returns the container's bind mounts annotated for the
// folder selector, plus any selected custom paths (in host form) that do
// not match a current mount, each flagged with whether it still exists,
// plus the stored exclusions in host form, plus the stored per-root
// CACHEDIR.TAG toggles (host-form keys, nil when never set; the handler
// normalizes that to {} on the wire). The selection is the stored explicit
// choice, or the automatic appdata default when none is configured.
func (s *Service) ContainerMounts(ctx context.Context, name string) ([]MountInfo, []CustomPath, []string, map[string]bool, error) {
	in, err := s.docker.Inspect(ctx, name)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("inspect container: %w", err)
	}

	auto := s.resolveAppdataPaths(name, in)
	tg, _ := s.store.GetTargetByContainer(name) // absent target → zero value, no selection
	effective := tg.SelectedPaths
	if len(effective) == 0 {
		effective = auto
	}
	// Split the entry classes before anything consumes the list: includes
	// drive Selected, the auto fallback above and the custom loop below. A raw
	// "!"-prefixed entry fed to any of them would render as a phantom custom
	// path with Exists:false, because toHostPath passes the prefixed container
	// path through unchanged. SplitExclusion keeps the prefix semantics in
	// selection.go. The auto fallback above stays keyed on the raw list: an
	// exclusions-only selection is the explicit-none state, not a missing
	// selection, so it must never fall back to auto.
	var includes, exclCPs []string
	for _, e := range effective {
		if bare, excluded := SplitExclusion(e); excluded {
			exclCPs = append(exclCPs, bare)
		} else {
			includes = append(includes, bare)
		}
	}
	selSet := sliceSet(includes)
	autoSet := sliceSet(auto)

	matched := map[string]bool{}
	var mounts []MountInfo
	for _, m := range in.Mounts {
		if m.Type != "bind" || m.Source == "" {
			continue
		}
		cp, reachable := s.toContainerPath(m.Source)
		mi := MountInfo{Source: m.Source, Dest: m.Destination, Reachable: reachable}
		if reachable {
			mi.Selected = selSet[cp]
			mi.IsAppdata = autoSet[cp]
			matched[cp] = true
		}
		mounts = append(mounts, mi)
	}

	// Custom = selected paths with no matching current mount, shown in host
	// form, each flagged with whether it still exists under the host mount so
	// the UI can tell a real selected folder from a stale one (#115). Includes
	// only: exclusions are chosen state, not stale entries, and are returned
	// separately below.
	var custom []CustomPath
	for _, cp := range includes {
		if !matched[cp] {
			_, statErr := os.Stat(cp) //nolint:gosec // G703: cp is a stored container path already validated under the mount root on save, not raw user input
			custom = append(custom, CustomPath{Path: s.toHostPath(cp), Exists: statErr == nil})
		}
	}

	// Exclusions are reviewable state: surface them separately, in host form
	// (the form the caller submitted), so the UI can show what was deselected.
	excluded := make([]string, 0, len(exclCPs))
	for _, cp := range exclCPs {
		excluded = append(excluded, s.toHostPath(cp))
	}
	return mounts, custom, excluded, tg.ExcludeCaches, nil
}

// errEmptySelection refuses a tree-sourced save that would deselect
// everything: an explicit empty selection silently re-enables automatic
// appdata detection, which the tree's deselect-all must never do over a
// previously non-empty selection. It carries its message and is
// errors.Is-able, so the PATCH boundary routes it to the coded
// {code:"empty-selection"} envelope instead of the plain failure one.
var errEmptySelection = errors.New("an explicit empty selection would re-enable automatic appdata detection")

// SetBackupPaths stores the user's explicit backup-folder selection for a
// container. The input paths are host paths (what the UI shows); each is
// translated to its container path and must be reachable under the host
// mount, otherwise the whole update is rejected. An entry prefixed with "!"
// (the tree selector's excluded branch, see selection.go) is translated and
// contained on its bare path, then stored prefixed. An empty list clears
// the selection so backups fall back to automatic appdata detection, except
// when selectionSource is the literal "tree" and a non-empty selection is
// stored: that is a deselect-everything, refused with errEmptySelection
// (the coded envelope is the handler's job). selectionSource is a transient
// intent signal, never persisted; any other value, including "" from older
// clients, is treated as absent, so those clients keep the clear-to-auto
// behaviour. The guard looks only at the source, never at the payload.
func (s *Service) SetBackupPaths(_ context.Context, name string, hostPaths []string, selectionSource string) error {
	var cps []string
	seen := map[string]bool{}
	for _, hp := range hostPaths {
		hp = strings.TrimSpace(hp)
		if hp == "" {
			continue
		}
		// The exclusion prefix is parsed before translation: toContainerPath does a
		// strict TrimPrefix against the host source root, so a raw "!/mnt/..."
		// would fail it and reject the whole save. Split, translate the bare path,
		// re-attach.
		bare, excluded := SplitExclusion(hp)
		if bare == "" && excluded {
			return fmt.Errorf("empty excluded path %q", hp)
		}
		// toContainerPath path.Cleans the input first (resolving any ".."), then
		// requires the host-source-root prefix, so its result is guaranteed to sit
		// under the mount root and needs no separate containment check. Both
		// classes get the same check on their bare path, so no unvalidated string
		// reaches the store.
		cp, ok := s.toContainerPath(bare)
		if !ok {
			return fmt.Errorf("path %q is not under the host mount and can't be backed up", hp)
		}
		if excluded {
			cp = ExclusionPrefix + cp
		}
		if !seen[cp] {
			cps = append(cps, cp)
			seen[cp] = true
		}
	}
	// Normalize before persisting: per-class maximal-root pruning and canonical
	// order, so equal selections store identical sets whatever order the
	// client sent. This is not stale-path repair: dropping entries whose folder
	// vanished stays the engine's job at run time (onlyExistingPaths);
	// normalization only removes redundant entries and orders what the user
	// chose.
	normalized := NormalizeSelection(cps)
	// Empty-selection guard, gated on the source alone. All three conditions
	// must hold:
	//   - the save came from the tree source (only the literal "tree"; older
	//     and unknown sources keep clearing to auto),
	//   - the normalized result is empty (an exclusions-only result is
	//     non-empty and is the explicitly deselected state, so it passes),
	//   - a non-empty selection is stored to protect (a fresh container has
	//     nothing to lose, so the save is a no-op, not a destructive deselect).
	// The refusal returns before any store write, so the prior selection stays
	// untouched.
	if selectionSource == "tree" && len(normalized) == 0 {
		if prior, gErr := s.store.GetTargetByContainer(name); gErr == nil && len(prior.SelectedPaths) > 0 {
			return errEmptySelection
		}
	}
	return s.store.SetBackupPaths(name, normalized)
}

// sliceSet builds a set from a string slice.
func sliceSet(xs []string) map[string]bool {
	m := make(map[string]bool, len(xs))
	for _, x := range xs {
		m[x] = true
	}
	return m
}

// onlyExistingPaths returns the subset of paths that exist on disk. BombVault
// reaches every backup source through the host mount, so a missing path means
// there is genuinely nothing to back up there.
func onlyExistingPaths(paths []string) []string {
	out := make([]string, 0, len(paths))
	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			out = append(out, p)
		}
	}
	return out
}

// configuredBackupPaths returns the paths a container backup is configured
// to use (the explicit folder selection if set, otherwise automatic appdata
// detection) without the on-disk filter effectiveBackupPaths applies.
//
// The distinction matters to a reader that can answer from the backup
// rather than from the filesystem (the exclusion assistant's snapshot
// feeder): an unmounted array makes every configured path fail its stat,
// and treating that as "this container has no folders" turns a temporarily
// unreachable share into a confident empty answer (#175). An empty list
// here means nothing is configured, the only case that really is
// "nothing".
//
// The explicit-vs-auto test runs on the raw stored list, but only the
// includes are returned (SplitExclusion, selection.go): the raw test makes
// an exclusions-only selection count as explicit rather than auto-detect,
// and the includes-only return keeps deselected branches out of everything
// downstream (effectiveBackupPaths' positionals, SuggestExcludes' roots).
// Exclusions never become positionals and never derive --exclude flags.
func (s *Service) configuredBackupPaths(name string, in model.Inspect) []string {
	chosen := s.resolveAppdataPaths(name, in)
	if existing, gErr := s.store.GetTargetByContainer(name); gErr == nil && len(existing.SelectedPaths) > 0 {
		chosen = includesOnly(existing.SelectedPaths)
	}
	return chosen
}

// effectiveBackupPaths returns the paths a container backup/export actually uses:
// the explicit folder selection if set, otherwise the automatic appdata
// detection, filtered to those that exist on disk (a stateless container ends up
// with an empty list).
func (s *Service) effectiveBackupPaths(name string, in model.Inspect) []string {
	paths, _ := s.effectiveBackupPathsWithSelection(name, in)
	return paths
}

// effectiveBackupPathsWithSelection returns the same paths and the stored
// selection they were derived from, out of one read of the target row.
//
// A backup needs both: the includes become the restic positionals, the
// exclusion branches the --exclude tail. With two reads, a PATCH landing in
// between could pair old positionals with new exclusions, a shape the user
// never chose: a derived --exclude biting a positional from the previous
// selection. The snapshot would record a path whose content was filtered
// out of it, the run would be recorded as success, retention would count
// it as a full backup, and a later restore would resolve that path to an
// empty directory and report success. Nothing serialises the writer: the
// PATCH route takes no domain lock and is not gated on a running backup.
//
// One read cannot tear. It can still be overtaken by a save that lands just
// before it, which is ordinary staleness: the whole selection is then the
// new one, the next run uses it, and no snapshot is internally
// inconsistent.
func (s *Service) effectiveBackupPathsWithSelection(name string, in model.Inspect) (paths, selection []string) {
	chosen := s.resolveAppdataPaths(name, in)
	if existing, gErr := s.store.GetTargetByContainer(name); gErr == nil && len(existing.SelectedPaths) > 0 {
		selection = existing.SelectedPaths
		chosen = includesOnly(selection)
	}
	return onlyExistingPaths(chosen), selection
}

// emptyBackupIsUnreachable decides what an empty effective path list means,
// and so whether a container backup should be refused (#181).
//
// Several states produce an empty list and only one of them is a fault:
//
//   - Nothing configured, nothing ever stored: a stateless container. Back
//     up its definition only.
//   - Nothing configured, but a previous backup did capture data: the user
//     has since deselected every folder, and the container is stateless
//     from now on. Refusing would leave no way forward except re-selecting
//     the folder that was just removed on purpose.
//   - A selection whose every entry is an exclusion ("!"-prefixed, see
//     selection.go): explicit-none and never a fault, even though the
//     deselected folder is still on disk.
//   - The data a previous backup captured is gone from disk: the share is
//     not mounted or HOST_SOURCE_ROOT is wrong. This is the fault worth
//     refusing, because recording it would look successful and overwrite
//     the stored path list with nothing.
//
// Only storedDataIsGone separates the second case from the last. The store
// cannot: an emptied selection is indistinguishable there from one that
// was never made (both are an empty SelectedPaths, meaning "use automatic
// detection"), and the configured list cannot either, because the appdata
// fallback in resolveAppdataPaths is itself stat-gated and disappears
// along with the folder.
func (s *Service) emptyBackupIsUnreachable(name string, effective []string) bool {
	return len(effective) == 0 && s.storedDataIsGone(name)
}

// storedDataIsGone reports whether the paths a previous backup captured
// have all disappeared from disk. That is what the guard's message claims
// ("not reachable"), so it is what the guard measures.
//
// A container with no stored target, or one whose last run captured
// nothing, is a first or stateless backup and is never refused. Neither is
// an exclusions-only selection: its raw list is non-empty but holds no
// includes, so measuring it would stat nothing and report the deselect as
// a vanished share. "Gone" means the data is gone, not that the user said
// no.
func (s *Service) storedDataIsGone(name string) bool {
	existing, err := s.store.GetTargetByContainer(name)
	if err != nil {
		return false // no prior target: a first backup of a new or stateless container
	}
	// Explicit-none: a non-empty stored list with zero includes is a deselect,
	// not a disappearance. Checked before any stat, so the classification comes
	// from the selection's shape (the user said no) rather than from the disk.
	if len(existing.SelectedPaths) > 0 && len(includesOnly(existing.SelectedPaths)) == 0 {
		return false
	}
	// SelectedPaths first: while a selection stands it is what a backup uses,
	// so its disappearance means the share went away. Stat the includes only,
	// like every other reader of the stored list; the "!"-prefixed entries are
	// classes, not paths, and statted as literals (against cwd) they would
	// virtually never exist and could only vote "gone". Once the user clears
	// the selection, what the last run captured (AppdataPaths) is the only
	// record of where the data was, and it still tells whether that data is
	// there.
	stored := includesOnly(existing.SelectedPaths)
	if len(existing.SelectedPaths) == 0 {
		stored = existing.AppdataPaths
	}
	return len(stored) > 0 && len(onlyExistingPaths(stored)) == 0
}

// SetExcludes stores the restic --exclude patterns for a container's backup.
// Lines are trimmed; blanks and exact duplicates are dropped (order preserved).
func (s *Service) SetExcludes(_ context.Context, name string, excludes []string) error {
	var clean []string
	seen := map[string]bool{}
	for _, e := range excludes {
		e = strings.TrimSpace(e)
		if e == "" || seen[e] {
			continue // skip blanks and duplicates
		}
		seen[e] = true
		clean = append(clean, e)
	}
	return s.store.SetExcludes(name, clean)
}

// maxExcludeCachesEntries caps the per-root CACHEDIR.TAG toggle map on top
// of decodeBody's 1 MiB cap and the map[string]bool decode (which already
// fails non-boolean values): a mount list is bounded by the container's
// real bind mounts, so anything near this size is abuse, not intent.
const maxExcludeCachesEntries = 64

// SetExcludeCaches stores the per-root CACHEDIR.TAG toggles for a
// container's backup. The map is a whole-map replace keyed by host path
// (what the UI shows); every key must translate under the host mount like
// a SetBackupPaths entry, otherwise the whole save is rejected before any
// store write, so a bad key can never leave a partially written map. An
// empty (or nil) map clears every toggle. The keys are validated UI state
// only: nothing but the boolean union of the values ever reaches restic.
func (s *Service) SetExcludeCaches(_ context.Context, name string, m map[string]bool) error {
	if len(m) > maxExcludeCachesEntries {
		return fmt.Errorf("too many exclude-caches entries (%d, max %d)", len(m), maxExcludeCachesEntries)
	}
	for hp := range m {
		// toContainerPath path.Cleans the key first (resolving any "..") and then
		// requires the host-source-root prefix, so its ok result guarantees
		// containment, the same discipline SetBackupPaths applies to every user
		// path.
		if _, ok := s.toContainerPath(hp); !ok {
			return fmt.Errorf("path %q is not under the host mount and can't be backed up", hp)
		}
	}
	return s.store.SetExcludeCaches(name, m)
}

// anyRootExcludeCaches reports whether any per-root CACHEDIR.TAG toggle is
// on: the item-level union restic's --exclude-caches flag expresses, since
// it applies to every positional source. It ignores whether a root is in
// the selection: the stored map is the user's remembered intent per root,
// and restic cannot scope the flag per positional anyway.
func anyRootExcludeCaches(m map[string]bool) bool {
	for _, on := range m {
		if on {
			return true
		}
	}
	return false
}

// PreviewExcludes resolves candidate exclude lines against the container's live
// mounts and returns, per line, the resolved --exclude pattern and whether it
// would exclude anything in this container's backup, so the UI can warn on a
// line that matches nothing. Stateless: nothing is persisted.
func (s *Service) PreviewExcludes(ctx context.Context, name string, candidate []string) ([]ExcludePreview, error) {
	in, err := s.docker.Inspect(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("inspect container: %w", err)
	}
	effective := s.effectiveBackupPaths(name, in)
	return s.previewExcludes(candidate, in, effective), nil
}
