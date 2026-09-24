package api

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/model"
	"github.com/junkerderprovinz/bombvault/internal/paths"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/secret"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// containerDefinition is the recreate recipe persisted at backup time, so a
// restore works after the container is gone from the host and, once written
// (encrypted) to the backup storage, after BombVault's own /config is lost
// (disaster recovery via Discover). It holds the inspect data, the Unraid
// template and the backed-up appdata paths.
type containerDefinition struct {
	Inspect      model.Inspect `json:"inspect"`
	TemplateXML  string        `json:"template_xml"`
	AppdataPaths []string      `json:"appdata_paths"`
	// Aliases are the links the mirror on the backup storage records, the
	// only ones Discover rebuilds. The alias rows are what everything else
	// reads.
	Aliases []definitionAlias `json:"aliases,omitempty"`
}

// defsDir returns the directory inside the containers repo (repo/def) where
// the encrypted container definitions are mirrored for disaster recovery.
// Inside the repo, a copy of the repo folder is self-contained, with the DR
// definitions travelling along, and the backup root stays uncluttered.
// "def" never collides with restic's own repo entries (config, data, index,
// keys, locks, snapshots), and restic ignores unknown subdirectories.
func (s *Service) defsDir(settings store.Settings) (string, error) {
	repo, err := s.containersRepoPath(settings)
	if err != nil {
		return "", err
	}
	return defsDirFor(repo), nil
}

// itemDefsDir picks where to look for a rediscovered item's definition mirror:
// beside its own snapshots when it was found in a named repository (#204), the
// domain's mirror otherwise. Empty when the named repository is remote or gone,
// which leaves the caller on the domain's mirror.
func (s *Service) itemDefsDir(repoID string, forVM bool) string {
	if strings.TrimSpace(repoID) == "" {
		return ""
	}
	named, err := s.store.GetNamedRepo(repoID)
	if err != nil {
		return ""
	}
	loc, rErr := s.resolveRepo(named.Repo)
	if rErr != nil || restic.IsRemoteRepo(loc) {
		return ""
	}
	if forVM {
		return vmDefsDirFor(loc)
	}
	return defsDirFor(loc)
}

// defsDirFor is defsDir for one repository, so an item on a named
// repository (#204) has its definition mirrored beside its own snapshots
// and a copy of that repository folder is self-contained too; otherwise
// discovery would find the item's name there with nothing to rebuild it
// from. A remote repository has no folder to write into and keeps the
// domain's location.
func defsDirFor(repo string) string { return filepath.Join(repo, "def") }

// vmDefsDirFor is the same for VMs.
func vmDefsDirFor(repo string) string { return filepath.Join(repo, "vm-def") }

// legacyDefsDir is the older container defs location, a sibling of the
// repo. It is still read as a fallback and migrated away by
// migrateLegacyDefs.
func (s *Service) legacyDefsDir(settings store.Settings) (string, error) {
	repo, err := s.containersRepoPath(settings)
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(repo), "bombvault-defs"), nil
}

// ensureDefsDir creates the disaster-recovery defs directory and makes sure
// it is world-traversable (0755). It lives on the operator's backup
// storage, typically a network share also copied off-box, so a non-root SMB
// user must be able to read it; the .def files inside are always
// APP_KEY-encrypted, so the looser mode exposes nothing (the restic repo
// beside it is readable too). Chmod, not just MkdirAll, also heals a
// directory created at 0700, which would lock SMB users out of the whole
// backup folder.
func ensureDefsDir(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil { //nolint:gosec // G301: backup share must be readable by the off-server sync tool; .def contents are encrypted
		return err
	}
	if err := os.Chmod(dir, 0o755); err != nil { //nolint:gosec // G302: see above; must be sync-readable, contents are encrypted
		return err
	}
	return nil
}

// writeDef writes an encrypted definition into the defs dir, readable (0644)
// by the off-server sync tool that copies the backup share. os.WriteFile
// keeps the mode of an existing file, so an explicit Chmod heals a .def
// left at 0600 and defeats a strict process umask; the contents are always
// APP_KEY-encrypted, so 0644 exposes nothing.
func writeDef(dir, fn string, enc []byte) error {
	final := filepath.Join(dir, fn)
	tmp := final + ".tmp"
	if err := os.WriteFile(tmp, enc, 0o644); err != nil { //nolint:gosec // G306: encrypted contents; backup share must be sync-readable. fn validated by defFileName; dir is operator-configured
		return err
	}
	// os.WriteFile keeps an existing file's mode, so force 0644 (and defeat a
	// strict umask) to heal a leftover tmp and let the sync tool read it.
	if cErr := os.Chmod(tmp, 0o644); cErr != nil { //nolint:gosec // G302: see above
		_ = os.Remove(tmp) //nolint:gosec // G703: tmp = final+".tmp"; final = Join(dir, fn); fn validated by defFileName, dir operator-configured
		return cErr
	}
	// Atomic swap, so a reader, or migrateLegacyDefs deleting the legacy source
	// once the destination exists, never sees a half-written def as complete
	// (both paths sit on the same backup storage, so os.Rename is atomic).
	if rErr := os.Rename(tmp, final); rErr != nil { //nolint:gosec // G703: tmp/final derived from Join(dir, fn); fn validated by defFileName, dir operator-configured
		_ = os.Remove(tmp) //nolint:gosec // G703: see above
		return rErr
	}
	return nil
}

// makeRepoReadable relaxes a local restic repo tree so the operator can copy
// it off-box (e.g. to a second drive over SMB) as a non-root user. restic,
// run as root, writes the repo 0700/0600, which locks a non-root sync tool
// out of the whole folder; the repo is encrypted, so adding group/other
// read (and dir traverse) exposes nothing. It runs after every container
// backup, the single choke point for every code path, rather than once per
// batch. Best-effort: a walk or chmod error must never fail a good backup,
// and a non-local repo path (an off-site rclone remote) simply fails the
// walk and is skipped.
//
// A full walk costs one lstat per entry, and on /mnt/user shfs multiplies
// that about 37 times: 592 ms for a 123 GB repository of 8119 entries,
// against a median backup of 2 s. The walk also grows with the repository
// rather than with the run. Almost none of those entries need a chmod; the
// ones that do are files restic just wrote. Adding a file updates its
// directory's mtime, so a directory untouched since the last clean pass
// cannot hold an entry that pass did not already relax, and its files need
// no stat: 263 directory stats instead of 8119 file stats.
//
// Mtime does not propagate upwards: writing data/bb/newpack updates bb's
// mtime and leaves data's as it was. Skipping a whole subtree because its
// root looks old would therefore skip every new pack, so directories are
// always descended into; only the per-file lstat inside an unchanged
// directory is saved.
//
// Two things keep that from becoming a coverage hole. The stamp only
// advances after a pass that saw no errors, so a pass that failed halfway
// repeats in full. And a stamp older than fullSweepAfter is ignored, which
// repairs the one case the mtime shortcut cannot see: something outside
// BombVault making a file restrictive without touching its directory.
func makeRepoReadable(repo, stampDir string) {
	stamp := permStampPath(stampDir, repo)
	var cutoff time.Time
	if fi, err := os.Stat(stamp); err == nil && time.Since(fi.ModTime()) < fullSweepAfter {
		cutoff = fi.ModTime()
	}

	// Stamped with the time the pass started, not finished: a file written
	// while the pass was already past its directory leaves that directory's
	// mtime at or after this instant, so the next pass still sees it.
	started := time.Now()
	clean := true
	relaxTree(repo, cutoff, &clean)

	if clean {
		writePermStamp(stamp, started.Add(-stampBackdate))
	}
}

// stampBackdate is how far the stamp is set behind the moment the pass
// began.
//
// A directory's mtime is less precise than time.Now(). tmpfs and several
// other filesystems stamp a directory at the timer tick, and two writes
// 1.5 ms apart can leave a directory's mtime identical to the nanosecond.
// Without a margin, a file written just after the pass started could carry
// a directory mtime on a tick just before it, and the next pass would read
// that directory as unchanged and skip the file. Backdating costs a rescan
// of whatever changed in the last few seconds before a pass, which is
// nothing, and closes the window.
const stampBackdate = 5 * time.Second

// relaxTree relaxes one directory and recurses. `cutoff` is the start of the
// last clean pass, or the zero time to stat everything.
func relaxTree(dir string, cutoff time.Time, clean *bool) {
	di, err := os.Stat(dir)
	if err != nil || !di.IsDir() {
		*clean = false
		return
	}
	relaxPerm(dir, di, true)

	entries, err := os.ReadDir(dir)
	if err != nil {
		*clean = false
		return
	}
	// An unchanged directory cannot hold a file the last clean pass did not
	// already relax: creating one would have moved this mtime.
	statFiles := cutoff.IsZero() || !di.ModTime().Before(cutoff)

	for _, e := range entries {
		p := filepath.Join(dir, e.Name())
		if e.IsDir() {
			relaxTree(p, cutoff, clean)
			continue
		}
		if !statFiles {
			continue
		}
		info, ierr := e.Info()
		if ierr != nil {
			*clean = false
			continue
		}
		relaxPerm(p, info, false)
	}
}

// fullSweepAfter is how long a stamp is trusted. Beyond it the next pass walks
// everything again, so the one case a directory's mtime cannot reveal is still
// repaired within a day instead of never.
const fullSweepAfter = 24 * time.Hour

// relaxPerm adds group+other read (and traverse, on a directory) if they are
// missing, and nothing otherwise.
func relaxPerm(p string, info fs.FileInfo, isDir bool) {
	perm := info.Mode().Perm()
	want := perm | 0o044 // group+other read
	if isDir {
		want |= 0o011 // group+other traverse
	}
	if want == perm {
		return
	}
	// Perm() drops setuid/setgid/sticky; re-add them so a group-inheritance
	// (setgid) dir on a shared NAS keeps its special bit through the chmod.
	special := info.Mode() & (os.ModeSetuid | os.ModeSetgid | os.ModeSticky)
	_ = os.Chmod(p, want|special) //nolint:gosec // G302: encrypted repo; must be readable by the operator's off-box sync tool
}

// permStampPath is where the last clean pass over `repo` is recorded. Keyed by a
// hash of the path so two repositories never share a stamp, and kept in
// BombVault's own data directory rather than inside the repository: a restic
// repository is restic's to own, and an unexpected file at its root is something
// `restic check` would have to explain away.
func permStampPath(stampDir, repo string) string {
	sum := sha256.Sum256([]byte(filepath.Clean(repo)))
	return filepath.Join(stampDir, "perms", hex.EncodeToString(sum[:16])+".stamp")
}

// writePermStamp records `at` as the moment of the last clean pass.
// Best-effort throughout: a stamp that cannot be written costs a full walk
// next time, never a wrong result.
func writePermStamp(path string, at time.Time) {
	if err := paths.EnsureDir(filepath.Dir(path)); err != nil {
		return
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0o600) //nolint:gosec // G304: path is derived from a hash under our own data dir
	if err != nil {
		return
	}
	_ = f.Close()
	_ = os.Chtimes(path, at, at)
}

// makeOffsiteRepoReadable is makeRepoReadable for an off-site destination
// repo: the relax pass every backup runs on the primary repo, applied to the
// replica so the far side of a mounted-share destination is readable by the
// share's non-root clients too.
//
// restic derives its modes from the repo's existing `data` directory and
// otherwise defaults to 0700 dirs and 0400 files, so an off-site repo
// BombVault creates (EnsureRepo → paths.EnsureDir, 0700) stays root-only
// forever, every pack, index and snapshot file included. On a local repo
// that never shows, because the operator reads it through the same root
// process; on a share it decides whether any other client can open the
// replica. With restic 0.17.3, `init` into a 0755 directory still creates
// data/ and keys/ at 0700 and files at 0400, so relaxing the repo root
// alone is not enough and the tree has to be walked. After one walk restic
// derives group-readable modes for later writes, but still not
// other-readable ones, so this runs after every replication, like the
// primary repo's pass.
//
// Remote backends have no local tree to chmod and are skipped outright (a
// WalkDir over "rest:http://…" would merely fail, but the guard says so).
// The repo is encrypted, so group/other read exposes nothing. Best-effort
// throughout.
func makeOffsiteRepoReadable(dest, stampDir string) {
	if restic.IsRemoteRepo(dest) {
		return
	}
	makeRepoReadable(dest, stampDir)
}

// readStoredDef reads an encrypted definition, preferring the in-repo
// location (repo/def) and falling back to the legacy sibling location, so a
// restore from an older backup still finds its definitions.
func readStoredDef(newDir, legacyDir, fn string) ([]byte, error) {
	enc, err := os.ReadFile(filepath.Join(newDir, fn)) //nolint:gosec // G304: fn validated by defFileName; dirs are operator-configured
	if err == nil {
		return enc, nil
	}
	return os.ReadFile(filepath.Join(legacyDir, fn)) //nolint:gosec // G304: fn validated by defFileName; dirs are operator-configured
}

// migrateLegacyDefs moves any def files from the legacy sibling dir into the
// in-repo dir, best-effort, then removes the legacy dir once empty, so the
// backup root cleans itself up on the next backup. It never fails a backup;
// both dirs sit on the same backup storage, so os.Rename works.
func migrateLegacyDefs(newDir, legacyDir string) {
	entries, err := os.ReadDir(legacyDir)
	if err != nil {
		return // no legacy dir (fresh install or already migrated)
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".def") {
			continue
		}
		src := filepath.Join(legacyDir, e.Name())
		dst := filepath.Join(newDir, e.Name())
		if _, statErr := os.Stat(dst); statErr == nil {
			_ = os.Remove(src) // already present in the new location → drop the stale copy
			continue
		}
		if renErr := os.Rename(src, dst); renErr != nil {
			// cross-device or race: copy + remove as a fallback, never lose the def.
			if b, rErr := os.ReadFile(src); rErr == nil { //nolint:gosec // G304: legacy .def under an operator-configured dir
				if wErr := writeDef(newDir, e.Name(), b); wErr == nil {
					_ = os.Remove(src)
				}
			}
			continue
		}
		_ = os.Chmod(dst, 0o644) //nolint:gosec // G302: encrypted def; must be sync-readable (rename kept the old 0600)
	}
	_ = os.Remove(legacyDir) // succeeds only when the dir is now empty
}

// migrateLegacyDefsIfDomain runs the legacy defs migration only when the
// write that triggered it went to the domain's own defs folder.
//
// The legacy folder holds the whole domain's definitions. Migrating it into
// an item's named repository (#204) because that one item backed up first
// would move every other item's definition somewhere Discover does not
// look, and then delete the source folder.
//
// A named function rather than an `if` at each call site, so the rule has
// one home and a test can reach it.
func migrateLegacyDefsIfDomain(dir, domainDir, legacyDir string) {
	if dir != domainDir {
		return
	}
	migrateLegacyDefs(dir, legacyDir)
}

// writeDefToStorage encrypts the definition with the APP_KEY-derived key and
// writes it to <defsDir>/<name>.def (0644, readable by the off-server sync
// tool that copies the backup share; the contents are always encrypted).
// The env vars inside the definition are sensitive, so the file is always
// encrypted regardless of the restic encryption setting. aliases go in as
// the definition's link records.
func (s *Service) writeDefToStorage(settings store.Settings, name, itemRepo string, defJSON []byte, aliases []store.Alias) error {
	fn, err := defFileName(name)
	if err != nil {
		return err
	}
	defJSON, err = withAliasRecords(defJSON, aliases)
	if err != nil {
		return err
	}
	// Beside the snapshots, whichever repository those went to. A remote one has
	// no local folder, so the domain's mirror stays the fallback.
	domainDir, err := s.defsDir(settings)
	if err != nil {
		return err
	}
	dir := domainDir
	switch {
	case itemRepo != "" && !restic.IsRemoteRepo(itemRepo):
		dir = defsDirFor(itemRepo)
	case restic.IsRemoteRepo(itemRepo):
		// Logged once per backup. A filesystem sidecar has nowhere to live beside a
		// bucket, so this item's recreate definition exists only on this box, and
		// someone treating a cloud repository as their whole disaster-recovery
		// story would lose it with the box. The /config backup carries every
		// definition and remains the real answer.
		log.Printf("api: %q is on a remote repository, so its recreate definition is mirrored only on this box; the /config backup is what carries it off site", name) //nolint:gosec // G706: name is %q-quoted
	}
	if err := ensureDefsDir(dir); err != nil {
		return fmt.Errorf("ensure defs dir: %w", err)
	}
	enc, err := secret.Encrypt(s.cfg.AppKey, defJSON)
	if err != nil {
		return fmt.Errorf("encrypt definition: %w", err)
	}
	if err := writeDef(dir, fn, enc); err != nil {
		return fmt.Errorf("write definition: %w", err)
	}
	// Move any legacy defs from the old sibling dir into the repo and remove
	// the old dir once empty (best-effort; a good backup never fails over
	// this), only for the domain's own defs dir; see migrateLegacyDefsIfDomain.
	if legacy, lErr := s.legacyDefsDir(settings); lErr == nil {
		migrateLegacyDefsIfDomain(dir, domainDir, legacy)
	}
	return nil
}

// defFileName returns the filesystem-safe definition filename for a container,
// rejecting any name with a path separator or "" so it can never escape the
// defs dir (defense-in-depth; docker names never contain a separator anyway).
func defFileName(name string) (string, error) {
	if name == "" || strings.ContainsAny(name, `/\`) || strings.Contains(name, "..") {
		return "", fmt.Errorf("unsafe container name %q", name)
	}
	return name + ".def", nil
}

// vmDefsDir returns the directory inside the vms repo (repo/vm-def) where
// the encrypted VM definitions are mirrored for disaster recovery, so the
// backup root stays clean and a repo-folder copy is self-contained. It is
// "vm-def", not "def", so that even when the operator points the containers
// and vms repos at the same folder, a same-named container and VM never
// collide.
func (s *Service) vmDefsDir(settings store.Settings) (string, error) {
	repo, err := s.vmsRepoPath(settings)
	if err != nil {
		return "", err
	}
	return vmDefsDirFor(repo), nil
}

// legacyVMDefsDir is the older VM defs location, a sibling of the vms repo.
func (s *Service) legacyVMDefsDir(settings store.Settings) (string, error) {
	repo, err := s.vmsRepoPath(settings)
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(repo), "bombvault-vm-defs"), nil
}

// writeVMDefToStorage mirrors a VM's definition (encrypted) to the backup storage
// so a freshly installed BombVault can rebuild it via DiscoverVMs after losing
// its database. The definition holds the domain XML + NVRAM, so it is always
// encrypted regardless of the restic encryption setting. aliases go in as the
// definition's link records.
func (s *Service) writeVMDefToStorage(settings store.Settings, name, itemRepo string, defJSON []byte, aliases []store.Alias) error {
	fn, err := defFileName(name)
	if err != nil {
		return err
	}
	defJSON, err = withAliasRecords(defJSON, aliases)
	if err != nil {
		return err
	}
	// Beside the snapshots; see writeDefToStorage.
	domainDir, err := s.vmDefsDir(settings)
	if err != nil {
		return err
	}
	dir := domainDir
	switch {
	case itemRepo != "" && !restic.IsRemoteRepo(itemRepo):
		dir = vmDefsDirFor(itemRepo)
	case restic.IsRemoteRepo(itemRepo):
		// See writeDefToStorage: a bucket has no folder to put the sidecar in, and
		// for a VM the definition is the domain XML and the NVRAM.
		log.Printf("api: vm %q is on a remote repository, so its recreate definition is mirrored only on this box; the /config backup is what carries it off site", name) //nolint:gosec // G706: name is %q-quoted
	}
	if err := ensureDefsDir(dir); err != nil {
		return fmt.Errorf("ensure vm defs dir: %w", err)
	}
	enc, err := secret.Encrypt(s.cfg.AppKey, defJSON)
	if err != nil {
		return fmt.Errorf("encrypt vm definition: %w", err)
	}
	if err := writeDef(dir, fn, enc); err != nil {
		return fmt.Errorf("write vm definition: %w", err)
	}
	// Only from the domain's own dir; see migrateLegacyDefsIfDomain.
	if legacy, lErr := s.legacyVMDefsDir(settings); lErr == nil {
		migrateLegacyDefsIfDomain(dir, domainDir, legacy)
	}
	return nil
}
