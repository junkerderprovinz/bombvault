package api

// Foreign-repo read sessions (#61): open ANOTHER BombVault instance's restic
// repository READ-ONLY with the OTHER instance's APP_KEY, inventory what it
// contains, and restore single items from it (StartForeignRestore). Two hard
// guarantees distinguish this from the Recovery "attach" flow:
//
//  1. Nothing is persisted. A session lives in memory only (Service.
//     foreignSessions) with a 30-minute TTL — the foreign location and key are
//     NEVER written to Settings (the attach flow's putSettings/UpdateSettings
//     path is deliberately not used here).
//  2. The foreign repo is never written to. The open probe is RepoOpens
//     (`restic cat config`) — NOT EnsureRepo, which would INITIALIZE a missing
//     repo, i.e. create an empty repository on someone else's storage.

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/backup"
	"github.com/junkerderprovinz/bombvault/internal/paths"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/restickey"
	"github.com/junkerderprovinz/bombvault/internal/secret"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// ForeignItem is one restorable item (container, VM or file set) found in a
// foreign repository, with all of its snapshots (oldest-first, as restic
// reports them).
type ForeignItem struct {
	Name      string            `json:"name"`
	Snapshots []restic.Snapshot `json:"snapshots"`
}

// ForeignInventory groups a foreign repository's snapshots by the same tag
// prefixes Discover cuts (container:/vm:/fileset:), so the Recovery UI can
// offer a browse-and-restore tree without any local state.
type ForeignInventory struct {
	Containers []ForeignItem `json:"containers"`
	VMs        []ForeignItem `json:"vms"`
	FileSets   []ForeignItem `json:"fileSets"`
}

// foreignSession is one open read-only session onto a foreign repository. It
// carries everything a later restore needs (resolved repo, detected mode, and
// the FOREIGN APP_KEY for def decryption) so nothing has to be re-entered or —
// crucially — persisted. In memory only; expired sessions are swept on access.
type foreignSession struct {
	id      string
	repo    string // resolved repo location (paths.Resolve'd local path or remote URL verbatim)
	key     string // the OTHER instance's APP_KEY (64 hex) — decrypts foreign defs, never ours
	mode    restic.Mode
	expires time.Time
}

// foreignSessionTTL is how long an open foreign session stays usable. Long
// enough to browse and run several restores, short enough that the foreign key
// does not linger in memory indefinitely.
const foreignSessionTTL = 30 * time.Minute

// foreignKeyRe validates the foreign APP_KEY shape: exactly 64 lowercase hex
// characters (the same shape config validates for our own APP_KEY; that regexp
// is unexported, so the shape is mirrored here). Validated BEFORE any use —
// restickey.Derive panics on non-hex input by design.
var foreignKeyRe = regexp.MustCompile(`^[0-9a-f]{64}$`)

// errForeignSession is returned for an unknown or expired session id — the UI
// answer is the same either way: connect again.
var errForeignSession = errors.New("foreign session expired or unknown — connect to the repository again")

// newForeignSessionID returns a URL-safe 24-character random session id
// (18 bytes of crypto/rand, base64url — same recipe as randomDeployPassword).
func newForeignSessionID() (string, error) {
	buf := make([]byte, 18)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate session id: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// OpenForeign opens a foreign BombVault repository read-only and returns a new
// session id plus the full inventory. location is either a relative subpath
// under the host mount root (a mounted share) or a restic remote URL used
// verbatim; foreignKey is the OTHER instance's APP_KEY. Remote backends reuse
// the LOCAL instance's already-stored cloud credentials (read-only settings
// access) — no new credentials are persisted, and Settings is never written.
//
// Mode detection is a pure read: probe RepoOpens with the key-derived
// encrypted mode first, then the plain (unencrypted) mode. EnsureRepo is
// deliberately NOT used — it would initialize a missing repo, i.e. write into
// the foreign location.
func (s *Service) OpenForeign(ctx context.Context, location, foreignKey string) (string, ForeignInventory, error) {
	if strings.TrimSpace(location) == "" {
		return "", ForeignInventory{}, errors.New("missing repository location")
	}
	if !foreignKeyRe.MatchString(foreignKey) {
		return "", ForeignInventory{}, errors.New("the APP_KEY must be exactly 64 lowercase hex characters")
	}
	repo, err := s.resolveRepo(location)
	if err != nil {
		return "", ForeignInventory{}, err
	}
	settings, err := s.store.GetSettings() // READ-only: cloud creds for remote backends; never written back
	if err != nil {
		return "", ForeignInventory{}, fmt.Errorf("read settings: %w", err)
	}
	cloudEnv := s.cloudEnvFor(settings)

	encMode := restic.Mode{Encrypted: true, Password: restickey.Derive(foreignKey), Env: cloudEnv}
	plainMode := restic.Mode{Env: cloudEnv}
	var mode restic.Mode
	switch {
	case s.engine.RepoOpens(ctx, repo, encMode):
		mode = encMode
	case s.engine.RepoOpens(ctx, repo, plainMode):
		mode = plainMode
	default:
		return "", ForeignInventory{}, errors.New("could not open the repository — wrong APP_KEY, or the location is not a BombVault/restic repository")
	}

	inv, err := s.foreignInventory(ctx, repo, mode)
	if err != nil {
		return "", ForeignInventory{}, err
	}

	id, err := newForeignSessionID()
	if err != nil {
		return "", ForeignInventory{}, err
	}
	now := time.Now()
	s.foreignMu.Lock()
	if s.foreignSessions == nil {
		s.foreignSessions = map[string]foreignSession{}
	}
	s.sweepForeignLocked(now)
	s.foreignSessions[id] = foreignSession{
		id:      id,
		repo:    repo,
		key:     foreignKey,
		mode:    mode,
		expires: now.Add(foreignSessionTTL),
	}
	s.foreignMu.Unlock()
	return id, inv, nil
}

// CloseForeign drops a session immediately (the UI calls it on leave/unmount).
// Closing an unknown or already-expired id is a harmless no-op.
func (s *Service) CloseForeign(id string) {
	s.foreignMu.Lock()
	delete(s.foreignSessions, id)
	s.foreignMu.Unlock()
}

// foreignSession returns the live session for id, sweeping expired sessions
// first so an expired id is indistinguishable from an unknown one.
func (s *Service) foreignSession(id string) (foreignSession, error) {
	s.foreignMu.Lock()
	defer s.foreignMu.Unlock()
	s.sweepForeignLocked(time.Now())
	sess, ok := s.foreignSessions[id]
	if !ok {
		return foreignSession{}, errForeignSession
	}
	return sess, nil
}

// sweepForeignLocked removes expired sessions. Caller holds foreignMu.
func (s *Service) sweepForeignLocked(now time.Time) {
	for id, sess := range s.foreignSessions {
		if now.After(sess.expires) {
			delete(s.foreignSessions, id)
		}
	}
}

// foreignInventory lists the repo ONCE and groups the snapshots by the
// container:/vm:/fileset: tag prefixes (the same prefixes Discover cuts).
// Items are sorted by name; slices are non-nil so the JSON is always [].
func (s *Service) foreignInventory(ctx context.Context, repo string, mode restic.Mode) (ForeignInventory, error) {
	snaps, err := s.listSnapshots(ctx, repo, mode)
	if err != nil {
		return ForeignInventory{}, err
	}
	containers := map[string][]restic.Snapshot{}
	vms := map[string][]restic.Snapshot{}
	fileSets := map[string][]restic.Snapshot{}
	for _, snap := range snaps {
		for _, tag := range snap.Tags {
			if rest, ok := strings.CutPrefix(tag, "container:"); ok && rest != "" {
				containers[rest] = append(containers[rest], snap)
			}
			if rest, ok := strings.CutPrefix(tag, "vm:"); ok && rest != "" {
				vms[rest] = append(vms[rest], snap)
			}
			if rest, ok := strings.CutPrefix(tag, "fileset:"); ok && rest != "" {
				fileSets[rest] = append(fileSets[rest], snap)
			}
		}
	}
	return ForeignInventory{
		Containers: foreignItems(containers),
		VMs:        foreignItems(vms),
		FileSets:   foreignItems(fileSets),
	}, nil
}

// foreignItems flattens a name→snapshots map into a name-sorted item list.
func foreignItems(m map[string][]restic.Snapshot) []ForeignItem {
	out := make([]ForeignItem, 0, len(m))
	for name, snaps := range m {
		out = append(out, ForeignItem{Name: name, Snapshots: snaps})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// ---------------------------------------------------------------------------
// Foreign restore (#61): restore one item from an open session's repository
// ---------------------------------------------------------------------------

// StartForeignRestore restores ONE item snapshot from an open foreign-repo
// session through the EXISTING restore machinery (async, progress over SSE,
// recorded runs), so the restored object becomes a normal local container /
// VM / file set afterwards. domain selects the item kind ("containers", "vms"
// or "files"); snapshotID "latest"/"" resolves to the item's newest snapshot
// in the SESSION repo. targetSubPath names the destination folder for file
// sets (required — a foreign file set has no trusted local source path) and is
// ignored for the def-based domains.
//
// The foreign repo is only ever READ here — snapshot listings, def files and
// the restic restore itself. EnsureRepo, applyRetention, Forget*, Prune,
// TagAdd and makeRepoReadable all belong to OUR OWN repos' lifecycle and are
// never called against sess.repo. Container/VM definitions decrypt with the
// SESSION's foreign APP_KEY (never s.cfg.AppKey) and are upserted LOCALLY, so
// prepareRestoreIn/executeRestore and the run bookkeeping work unchanged.
//
// Every foreign restore is confirm-gated (a same-named local container/VM
// would be overwritten — never silently). ALL validation runs synchronously,
// so a bad request fails immediately and no goroutine starts. Shares
// batchActive with backups and the other restores; returns (false, nil) when
// one is already running.
func (s *Service) StartForeignRestore(ctx context.Context, sessionID, domain, item, snapshotID string, confirm bool, targetSubPath string) (bool, error) {
	if !s.batchActive.CompareAndSwap(false, true) {
		return false, nil
	}
	key, run, err := s.prepareForeignRestore(ctx, sessionID, domain, item, snapshotID, confirm, targetSubPath)
	if err != nil {
		s.batchActive.Store(false)
		return false, err
	}
	// Detach so the run is independent of the request that started it, capped
	// by restoreTimeout — the exact StartRestore pattern (progress key + cancel
	// registration; the run outcome lands in the run history).
	bctx := context.WithoutCancel(ctx)
	go func() {
		defer s.batchActive.Store(false)
		tctx, tcancel := context.WithTimeout(bctx, restoreTimeout)
		defer tcancel()
		rctx, cancel := context.WithCancel(tctx)
		defer cancel()
		s.registerCancel(key, cancel)
		defer s.unregisterCancel(key)
		if rerr := run(rctx); rerr != nil {
			log.Printf("api: foreign restore: %s %q failed: %v", domain, item, rerr) //nolint:gosec // G706: item is %q-quoted; domain passed the fixed switch below
		}
	}()
	return true, nil
}

// prepareForeignRestore runs ALL of a foreign restore's validation and
// resolution synchronously and returns the progress key plus the detached
// work for the domain. The confirm guard fires FIRST (the familiar sentinel,
// same discipline as prepareRestore); the item name is boundary-checked here
// because it feeds restic tags, def filenames and progress keys.
func (s *Service) prepareForeignRestore(ctx context.Context, sessionID, domain, item, snapshotID string, confirm bool, targetSubPath string) (string, func(context.Context) error, error) {
	if !confirm {
		return "", nil, backup.ErrNotConfirmed
	}
	if !validResourceName(item) {
		return "", nil, errors.New("invalid item name")
	}
	sess, err := s.foreignSession(sessionID)
	if err != nil {
		return "", nil, err
	}
	ref := repoRef{repo: sess.repo, mode: sess.mode}
	switch domain {
	case "containers":
		if err := s.adoptForeignContainerDef(sess, item); err != nil {
			return "", nil, err
		}
		plan, err := s.prepareRestoreIn(ctx, ref, item, snapshotID, true)
		if err != nil {
			return "", nil, err
		}
		return "container:" + item, func(rctx context.Context) error {
			return s.executeRestore(rctx, item, plan, false)
		}, nil
	case "vms":
		if err := s.adoptForeignVMDef(sess, item); err != nil {
			return "", nil, err
		}
		plan, err := s.prepareRestoreVMIn(ctx, ref, item, snapshotID, true)
		if err != nil {
			return "", nil, err
		}
		return "vm:" + item, func(rctx context.Context) error {
			return s.executeRestoreVM(rctx, item, plan, false)
		}, nil
	case "files":
		plan, err := s.prepareForeignFileSetRestore(ctx, sess, item, snapshotID, targetSubPath)
		if err != nil {
			return "", nil, err
		}
		rkey := "files:" + plan.setName // the exact progBegin key this restore publishes under
		return rkey, func(rctx context.Context) error {
			runID := s.beginRestoreRunForTarget(plan.setID)
			pctx := s.progBegin(rctx, rkey, "restore")
			rerr := s.runRestoreFileSet(pctx, plan)
			s.progEnd(rkey, "restore", rerr == nil)
			s.finishRestoreRun(runID, plan.snapshotID, rerr)
			return rerr
		}, nil
	default:
		return "", nil, errors.New("unknown domain (must be containers, vms or files)")
	}
}

// adoptForeignContainerDef reads the item's encrypted definition from the
// FOREIGN repo's defs dir (repo/def, with the pre-v5.4.1 sibling fallback),
// decrypts it with the SESSION's foreign APP_KEY — never s.cfg.AppKey — and
// upserts a LOCAL target row (exactly what Discover does against our own
// repo), so the existing restore preparation finds the recreate recipe and
// the restored container is a normal local target afterwards. Only the LOCAL
// store is written; the foreign repo is only read.
func (s *Service) adoptForeignContainerDef(sess foreignSession, name string) error {
	fn, err := defFileName(name)
	if err != nil {
		return err
	}
	enc, err := readStoredDef(filepath.Join(sess.repo, "def"), filepath.Join(filepath.Dir(sess.repo), "bombvault-defs"), fn)
	if err != nil {
		return fmt.Errorf("the foreign repository holds no readable definition for container %q — it cannot be recreated here", name)
	}
	plain, err := secret.Decrypt(sess.key, enc)
	if err != nil {
		return fmt.Errorf("the stored definition for %q does not decrypt with this session's APP_KEY", name)
	}
	var def containerDefinition
	if err := json.Unmarshal(plain, &def); err != nil {
		return fmt.Errorf("foreign definition for %q is corrupt: %w", name, err)
	}
	if _, err := s.store.UpsertTarget(store.Target{
		ContainerName: name,
		AppdataPaths:  def.AppdataPaths,
		Definition:    string(plain),
	}); err != nil {
		return fmt.Errorf("adopt container %q: %w", name, err)
	}
	return nil
}

// adoptForeignVMDef is adoptForeignContainerDef for the vms domain: read the
// encrypted definition from the foreign repo's vm-def dir (legacy sibling
// fallback), decrypt with the SESSION key, upsert a LOCAL VM target (what
// DiscoverVMs does against our own repo, including the graceful default).
func (s *Service) adoptForeignVMDef(sess foreignSession, name string) error {
	fn, err := defFileName(name)
	if err != nil {
		return err
	}
	enc, err := readStoredDef(filepath.Join(sess.repo, "vm-def"), filepath.Join(filepath.Dir(sess.repo), "bombvault-vm-defs"), fn)
	if err != nil {
		return fmt.Errorf("the foreign repository holds no readable definition for vm %q — it cannot be recreated here", name)
	}
	plain, err := secret.Decrypt(sess.key, enc)
	if err != nil {
		return fmt.Errorf("the stored definition for %q does not decrypt with this session's APP_KEY", name)
	}
	var def vmDefinition
	if err := json.Unmarshal(plain, &def); err != nil {
		return fmt.Errorf("foreign definition for %q is corrupt: %w", name, err)
	}
	method := def.Method
	if method == "" {
		method = "graceful"
	}
	if _, err := s.store.UpsertVMTarget(store.VMTarget{
		Name:       name,
		Method:     method,
		Definition: string(plain),
	}); err != nil {
		return fmt.Errorf("adopt vm %q: %w", name, err)
	}
	return nil
}

// prepareForeignFileSetRestore validates a foreign file-set restore and builds
// the same fileSetRestorePlan the settings-driven path executes, pointed at
// the SESSION repo. Foreign file sets ALWAYS restore into a chosen folder
// under the host mount (never in place — a foreign item has no trusted local
// source path), and the name is adopted as a LOCAL, disabled, path-less set —
// like DiscoverFileSets — so the recorded run is attributable in the history.
func (s *Service) prepareForeignFileSetRestore(ctx context.Context, sess foreignSession, item, snapshotID, targetSubPath string) (fileSetRestorePlan, error) {
	sub := strings.TrimSpace(targetSubPath)
	if sub == "" {
		return fileSetRestorePlan{}, errors.New("a target folder is required to restore a file set from a foreign repository")
	}
	target, err := paths.Resolve(s.cfg.HostMountRoot, sub)
	if err != nil {
		return fileSetRestorePlan{}, errors.New("invalid target folder: must be a relative subpath under the host mount")
	}

	// Snapshot ownership: an explicit id must be well-formed hex AND belong to
	// THIS item's fileset:<Name> tag in the SESSION repo; "latest"/"" resolves
	// to the newest matching snapshot (restic lists oldest-first).
	explicitID := snapshotID != "latest" && snapshotID != ""
	if explicitID && !backup.ValidSnapshotID(snapshotID) {
		return fileSetRestorePlan{}, backup.ErrInvalidSnapshotID
	}
	snaps, err := s.snapshotsForTag(ctx, sess.repo, sess.mode, "fileset:"+item)
	if err != nil {
		return fileSetRestorePlan{}, err
	}
	if explicitID {
		if !snapshotBelongs(snaps, snapshotID) {
			return fileSetRestorePlan{}, fmt.Errorf("snapshot %s does not belong to this file set", snapshotID)
		}
	} else {
		if len(snaps) == 0 {
			return fileSetRestorePlan{}, errors.New("no backups found for this file set")
		}
		snapshotID = snaps[len(snaps)-1].ID
	}

	// Adopt the name locally when unknown (disabled and path-less, exactly like
	// DiscoverFileSets — the UI flags "set path before backup") so the restore
	// run records against a stable file_sets.id. An existing local set of the
	// same name is reused untouched (its path/excludes/enabled state is user
	// configuration).
	setID := ""
	if set, gErr := s.store.GetFileSetByName(item); gErr == nil {
		setID = set.ID
	} else {
		created, cErr := s.store.CreateFileSet(store.FileSet{Name: item, Enabled: false})
		if cErr != nil {
			return fileSetRestorePlan{}, fmt.Errorf("adopt file set %q: %w", item, cErr)
		}
		setID = created.ID
	}

	// Create the target dir ONLY after every validation passed.
	if err := paths.EnsureDir(target); err != nil {
		return fileSetRestorePlan{}, fmt.Errorf("create target folder: %w", err)
	}
	return fileSetRestorePlan{
		repo:       sess.repo,
		mode:       sess.mode,
		snapshotID: snapshotID,
		setID:      setID,
		setName:    item,
		target:     target,
	}, nil
}
