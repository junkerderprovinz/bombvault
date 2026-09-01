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
	"path"
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
	repo    string // resolved repo location (paths.Resolve'd local mounted path; remote backends are rejected)
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
var errForeignSession = errors.New("foreign session expired or unknown. Connect to the repository again")

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
// session id plus the full inventory. location MUST be a LOCAL, mounted-share
// path (a relative subpath under the host mount root) — the real use case is
// mounting server A's backup share on server B and pointing at it. A restic
// REMOTE backend (rest:/s3:/sftp:/rclone:/b2:/gs:/azure:, or an unprefixed rclone
// remote name) is REJECTED: resolving one would hand a third-party URL to restic
// together with THIS instance's own off-site credentials — a confused-deputy
// credential disclosure. Foreign sessions therefore never attach cloudEnv and
// never touch a remote backend. foreignKey is the OTHER instance's APP_KEY;
// nothing is persisted (Settings is never written).
//
// Mode detection is a pure read: probe RepoOpens with the key-derived encrypted
// mode first, then the plain (unencrypted) mode. Every probe — and every later
// operation on the session — is lock-free (Mode.NoLock), so opening someone
// else's repo read-only never writes a lock file into it. EnsureRepo is
// deliberately NOT used — it would initialize a missing repo, i.e. write into the
// foreign location.
func (s *Service) OpenForeign(ctx context.Context, location, foreignKey string) (string, ForeignInventory, error) {
	if strings.TrimSpace(location) == "" {
		return "", ForeignInventory{}, errors.New("missing repository location")
	}
	if !foreignKeyRe.MatchString(foreignKey) {
		return "", ForeignInventory{}, errors.New("the APP_KEY must be exactly 64 lowercase hex characters")
	}
	// Only a locally mounted repository path is allowed. resolveRepo would pass a
	// remote-backend location (rest:/s3:/… — see restic.IsRemoteRepo) straight to
	// restic, which the foreign session would then probe with the local instance's
	// off-site credentials against a location the user controls. Reject it — and
	// the unprefixed-remote typo ("BackBlaze:bucket") — before any engine call.
	if restic.IsRemoteRepo(location) || restic.LooksLikeUnprefixedRemote(location) {
		return "", ForeignInventory{}, errors.New("only a locally mounted repository path is supported here; mount the other server's backup share and point at it")
	}
	repo, err := s.resolveRepo(location)
	if err != nil {
		return "", ForeignInventory{}, err
	}

	// No cloudEnv is attached: a foreign session is local-only by construction
	// (remote locations were rejected above), so the local instance's backend
	// credentials never travel with a foreign probe or restore. NoLock keeps the
	// read-only session from writing a lock file into the foreign repo.
	encMode := restic.Mode{Encrypted: true, Password: restickey.Derive(foreignKey), NoLock: true}
	plainMode := restic.Mode{NoLock: true}
	var mode restic.Mode
	switch {
	case s.engine.RepoOpens(ctx, repo, encMode):
		mode = encMode
	case s.engine.RepoOpens(ctx, repo, plainMode):
		mode = plainMode
	default:
		// Deliberately "BombVault or restic", not "BombVault/restic" ([584]): the
		// scrubber that runs over every error on its way out redacts absolute paths
		// with absPathRe, which cannot tell a filesystem path from a slash inside a
		// word. It turned this sentence into "not a BombVault[path] repository" for
		// every user who ever saw it, which is how it came back from the forum. The
		// redaction is right; the slash was ours to remove.
		return "", ForeignInventory{}, errors.New("could not open the repository: wrong APP_KEY, or the location is not a BombVault or restic repository")
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
	s.startForeignJanitorLocked() // periodic sweep so an abandoned key can't linger
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

// sweepForeignLocked removes expired sessions, dropping each expired session's
// foreign APP_KEY from memory (deleting the map entry makes the key string
// unreferenced and GC-eligible). Caller holds foreignMu. Runs on access
// (foreignSession / OpenForeign) AND on the background janitor's tick, so an
// abandoned session's key is cleared even when no further API call ever arrives.
func (s *Service) sweepForeignLocked(now time.Time) {
	for id, sess := range s.foreignSessions {
		if now.After(sess.expires) {
			delete(s.foreignSessions, id)
		}
	}
}

// foreignSweepInterval is how often the background janitor sweeps expired foreign
// sessions when no API call comes in. Short relative to the 30-minute TTL so an
// abandoned key is cleared promptly, long enough to be negligible overhead.
const foreignSweepInterval = 5 * time.Minute

// startForeignJanitorLocked starts the single background sweeper on first use and
// is a no-op afterwards (idempotent). Caller holds foreignMu. The goroutine ticks
// every foreignSweepInterval (or s.foreignSweepEvery when a test injects a faster
// one) and drops expired sessions — and the foreign key they hold — WITHOUT
// waiting for another foreign API call, closing the "expired key lingers in memory
// forever" gap. It exits when stopForeignJanitor closes the stop channel; a later
// OpenForeign restarts it.
func (s *Service) startForeignJanitorLocked() {
	if s.foreignJanitor != nil {
		return
	}
	stop := make(chan struct{})
	s.foreignJanitor = stop
	every := s.foreignSweepEvery
	if every <= 0 {
		every = foreignSweepInterval
	}
	go func() {
		t := time.NewTicker(every)
		defer t.Stop()
		for {
			select {
			case <-stop:
				return
			case <-t.C:
				s.sweepForeign()
			}
		}
	}()
}

// stopForeignJanitor stops the background sweeper if it is running (idempotent).
// Production leaves the janitor for the process lifetime; tests call this (via the
// Service's test cleanup) so the goroutine never outlives the test.
func (s *Service) stopForeignJanitor() {
	s.foreignMu.Lock()
	if s.foreignJanitor != nil {
		close(s.foreignJanitor)
		s.foreignJanitor = nil
	}
	s.foreignMu.Unlock()
}

// sweepForeign drops every expired session under the lock — the janitor's tick.
func (s *Service) sweepForeign() {
	s.foreignMu.Lock()
	s.sweepForeignLocked(time.Now())
	s.foreignMu.Unlock()
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
// SESSION's foreign APP_KEY (never s.cfg.AppKey); the restore is validated
// against the decrypted recipe FIRST and the recipe is adopted LOCALLY
// (UpsertTarget) only once every check passes — so a validation failure never
// clobbers a same-named local target — after which executeRestore and the run
// bookkeeping work unchanged.
//
// Every foreign restore is confirm-gated (a same-named local container/VM
// would be overwritten — never silently). ALL validation runs synchronously,
// so a bad request fails immediately and no goroutine starts. Shares
// batchActive with backups and the other restores; returns (false, nil) when
// one is already running.
//
// zvolPool is a VMS-DOMAIN-ONLY, OPTIONAL destination ZFS pool name for a VM
// whose domain XML carries TrueNAS zvol-backed (block-device) disks —
// destBase (derived from targetSubPath, see foreignVMDestBase) is a
// filesystem path and carries no ZFS pool information, so a zvol disk's `zfs
// receive` target cannot be rebased from it the way a file-backed disk's path
// is. See prepareRestoreVMForTarget's destZvolPool doc comment for the full
// rebase/refusal behavior. Ignored for every other domain and for a VM with
// no zvol disks.
func (s *Service) StartForeignRestore(ctx context.Context, sessionID, domain, item, snapshotID string, confirm bool, targetSubPath string, filePaths []string, overwrite bool, zvolPool string) (bool, error) {
	if !s.batchActive.CompareAndSwap(false, true) {
		return false, nil
	}
	key, run, onPanic, err := s.prepareForeignRestore(ctx, sessionID, domain, item, snapshotID, confirm, targetSubPath, filePaths, overwrite, zvolPool)
	if err != nil {
		s.batchActive.Store(false)
		return false, err
	}
	// Detach so the run is independent of the request that started it, capped
	// by restoreTimeout — the exact StartRestore pattern (progress key + cancel
	// registration; the run outcome lands in the run history).
	bctx := context.WithoutCancel(ctx)
	go func() {
		// onPanic is built PER DOMAIN by prepareForeignRestore, because which of
		// recoverOperation's two run-closing strategies is correct depends on which
		// branch of the switch below ran: the containers/vms branches dispatch into
		// s.executeRestore/executeRestoreVM, whose deep orchestrator (like
		// StartRestore/StartRestoreVM's own goroutine) calls store.StartRun itself —
		// so only the TARGET id is known here, and failStuckRun is correct. The files
		// branch instead drives its own beginRestoreRunForTarget/finishRestoreRun
		// bookkeeping inline (like StartRestoreFileSet/StartRestoreFileSetFiles) — so
		// there IS a local runID, and finishRestoreRun is correct instead. Each
		// branch already builds the right closure when it builds run itself; this
		// defer just wires it into the same outermost position every other
		// backup/restore goroutine in this package uses (see recoverOperation's own
		// doc comment for why outermost matters).
		defer s.recoverOperation("foreign restore: "+domain+":"+item, nil, onPanic)
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
// resolution synchronously and returns the progress key, the detached work
// for the domain, and an onPanic closure for recoverOperation to call if that
// detached work panics (see StartForeignRestore's goroutine for why this is
// domain-specific rather than one shared strategy — the containers/vms
// branches return a failStuckRun(plan.targetID, ...) closure, the files
// branch a finishRestoreRun(runID, ...) closure that closes over a runID
// declared alongside its run closure, exactly as StartRestoreFileSet does).
// The confirm guard fires FIRST (the familiar sentinel, same discipline as
// prepareRestore); the item name is boundary-checked here because it feeds
// restic tags, def filenames and progress keys. zvolPool is StartForeignRestore's
// own parameter, passed straight through — see its doc comment.
func (s *Service) prepareForeignRestore(ctx context.Context, sessionID, domain, item, snapshotID string, confirm bool, targetSubPath string, filePaths []string, overwrite bool, zvolPool string) (string, func(context.Context) error, func(string), error) {
	if !confirm {
		return "", nil, nil, backup.ErrNotConfirmed
	}
	// Domain-aware name check: libvirt VM names legitimately contain spaces
	// (e.g. "Windows Server 2022"), so VMs use the libvirt-aware validVMName
	// (still blocks empty/over-long/path-separators/".."/leading "-"/control
	// chars) while containers and file sets keep the strict validResourceName.
	nameOK := validResourceName(item)
	if domain == "vms" {
		nameOK = validVMName(item)
	}
	if !nameOK {
		return "", nil, nil, errors.New("invalid item name")
	}
	sess, err := s.foreignSession(sessionID)
	if err != nil {
		return "", nil, nil, err
	}
	ref := repoRef{repo: sess.repo, mode: sess.mode}
	switch domain {
	case "containers":
		// Read + decrypt the foreign recipe into an in-memory target, VALIDATE the
		// restore against it (snapshot ownership, appdata containment) WITHOUT
		// persisting, and only then adopt it locally. A validation failure returns
		// before any UpsertTarget, so a same-named local target keeps its own
		// definition + appdata_paths byte-for-byte.
		tg, err := s.foreignContainerTarget(sess, item)
		if err != nil {
			return "", nil, nil, err
		}
		if snapshotID != "latest" && snapshotID != "" && !backup.ValidSnapshotID(snapshotID) {
			return "", nil, nil, backup.ErrInvalidSnapshotID
		}
		// A cross-instance container restore ALWAYS remaps appdata onto a destination
		// on THIS host (#125): the request Target chooses it, else the configured
		// restore folder, else the conventional local appdata share. A standard
		// container whose appdata already lives there remaps to the same path (no-op);
		// a container from a pool this host lacks (e.g. /mnt/zfs) lands correctly
		// instead of writing to the wrong pool. The guard + overwrite check run in
		// prepareRestoreForTarget before any destructive Stop/Remove.
		destBase, err := s.foreignContainerDestBase(targetSubPath)
		if err != nil {
			return "", nil, nil, err
		}
		plan, err := s.prepareRestoreForTarget(ctx, ref, item, snapshotID, tg, destBase, overwrite)
		if err != nil {
			return "", nil, nil, err
		}
		adopted, err := s.store.UpsertTarget(tg)
		if err != nil {
			return "", nil, nil, fmt.Errorf("adopt container %q: %w", item, err)
		}
		plan.targetID = adopted.ID // attribute the run to the persisted row
		run := func(rctx context.Context) error {
			return s.executeRestore(rctx, item, plan, false)
		}
		// executeRestore's deep orchestrator (backup.RestoreContainer) calls
		// store.StartRun(plan.targetID, ...) itself — a plain sequential call, not
		// deferred — so a panic during the restore leaves that run stuck "running"
		// without this, exactly like StartRestore's own goroutine.
		onPanic := func(msg string) {
			s.failStuckRun(plan.targetID, msg)
		}
		return "container:" + item, run, onPanic, nil
	case "vms":
		// Same validate-before-adopt discipline as the containers case.
		tg, err := s.foreignVMTarget(sess, item)
		if err != nil {
			return "", nil, nil, err
		}
		if snapshotID != "latest" && snapshotID != "" && !backup.ValidSnapshotID(snapshotID) {
			return "", nil, nil, backup.ErrInvalidSnapshotID
		}
		// A cross-instance VM restore MUST remap the source server's disk paths onto
		// a destination on THIS host (the source pool is not mounted here — restoring
		// there would fill the host's RAM and brick it, #122). targetSubPath (the
		// request Target) chooses the destination; empty falls back to the local VM
		// domains path. prepareRestoreVMForTarget remaps disks + XML and guards the
		// destination before any restic write. zvolPool is the SEPARATE destination
		// ZFS pool a TrueNAS zvol-backed disk needs (destBase carries no ZFS pool
		// information — see prepareRestoreVMForTarget's destZvolPool doc comment);
		// prepareRestoreVMForTarget refuses cleanly if the VM has zvol disks and
		// zvolPool was left empty, rather than attempting `zfs receive` against the
		// source box's pool name on this host.
		destBase, err := s.foreignVMDestBase(targetSubPath)
		if err != nil {
			return "", nil, nil, err
		}
		plan, err := s.prepareRestoreVMForTarget(ctx, ref, item, snapshotID, tg, destBase, zvolPool)
		if err != nil {
			return "", nil, nil, err
		}
		adopted, err := s.store.UpsertVMTarget(tg)
		if err != nil {
			return "", nil, nil, fmt.Errorf("adopt vm %q: %w", item, err)
		}
		plan.targetID = adopted.ID
		// A foreign-restored VM is defined but NEVER autostarted: it carries the
		// SOURCE host's domain XML (host-specific devices/paths), so booting it — now
		// or on host boot — could wedge libvirt. Force autostart off and leave it
		// stopped; the operator starts it once they have vetted the definition.
		plan.wasAutostart = false
		run := func(rctx context.Context) error {
			return s.executeRestoreVM(rctx, item, plan, true)
		}
		// Same reasoning as the containers branch above, for executeRestoreVM's
		// backup.RestoreVM orchestrator.
		onPanic := func(msg string) {
			s.failStuckRun(plan.targetID, msg)
		}
		return "vm:" + item, run, onPanic, nil
	case "files":
		// A non-empty selection restores only those paths/subfolders (the manilx
		// #123 case: pull one stack out of a whole-appdata set); empty restores the
		// whole set. Both extract into the required target folder.
		if len(filePaths) > 0 {
			plan, err := s.prepareForeignFileSetFilesRestore(ctx, sess, item, snapshotID, targetSubPath, filePaths)
			if err != nil {
				return "", nil, nil, err
			}
			rkey := "files:" + plan.setName
			// runID is declared here (before the closures below) rather than with :=
			// at its assignment inside run, so onPanic — which can only run AFTER
			// that assignment, since a panic before it means beginRestoreRunForTarget
			// never got called and no run needs closing — sees whatever value it
			// holds at panic time (exactly StartRestoreFileSet's own goroutine).
			var runID string
			run := func(rctx context.Context) error {
				runID = s.beginRestoreRunForTarget(plan.setID)
				pctx, startedAt := s.progBegin(rctx, rkey, "restore")
				rerr := s.runRestoreFileSetFiles(pctx, plan)
				return s.concludeFileSetRestore(runID, rkey, plan.snapshotID, rerr, startedAt)
			}
			onPanic := func(msg string) {
				s.finishRestoreRun(runID, "", errors.New(msg)) // see StartRestoreFileSet for why not concludeFileSetRestore
			}
			return rkey, run, onPanic, nil
		}
		plan, err := s.prepareForeignFileSetRestore(ctx, sess, item, snapshotID, targetSubPath)
		if err != nil {
			return "", nil, nil, err
		}
		rkey := "files:" + plan.setName // the exact progBegin key this restore publishes under
		var runID string                // see the selective branch above for why this is declared here
		run := func(rctx context.Context) error {
			runID = s.beginRestoreRunForTarget(plan.setID)
			pctx, startedAt := s.progBegin(rctx, rkey, "restore")
			rerr := s.runRestoreFileSet(pctx, plan)
			return s.concludeFileSetRestore(runID, rkey, plan.snapshotID, rerr, startedAt)
		}
		onPanic := func(msg string) {
			s.finishRestoreRun(runID, "", errors.New(msg)) // see StartRestoreFileSet for why not concludeFileSetRestore
		}
		return rkey, run, onPanic, nil
	default:
		return "", nil, nil, errors.New("unknown domain (must be containers, vms or files)")
	}
}

// foreignContainerTarget reads the item's encrypted definition from the FOREIGN
// repo's defs dir (repo/def, with the pre-v5.4.1 sibling fallback), decrypts it
// with the SESSION's foreign APP_KEY — never s.cfg.AppKey — and returns the LOCAL
// target row it WOULD become (exactly the shape Discover upserts), WITHOUT writing
// anything. prepareForeignRestore validates the restore against this in-memory
// target and persists it (UpsertTarget) only once every check passes, so a failed
// foreign restore never clobbers a same-named local target's recipe. Only the
// foreign repo is read here.
func (s *Service) foreignContainerTarget(sess foreignSession, name string) (store.Target, error) {
	fn, err := defFileName(name)
	if err != nil {
		return store.Target{}, err
	}
	enc, err := readStoredDef(filepath.Join(sess.repo, "def"), filepath.Join(filepath.Dir(sess.repo), "bombvault-defs"), fn)
	if err != nil {
		return store.Target{}, fmt.Errorf("the foreign repository holds no readable definition for container %q, so it cannot be recreated here", name)
	}
	plain, err := secret.Decrypt(sess.key, enc)
	if err != nil {
		return store.Target{}, fmt.Errorf("the stored definition for %q does not decrypt with this session's APP_KEY", name)
	}
	var def containerDefinition
	if err := json.Unmarshal(plain, &def); err != nil {
		return store.Target{}, fmt.Errorf("foreign definition for %q is corrupt: %w", name, err)
	}
	return store.Target{
		ContainerName: name,
		AppdataPaths:  def.AppdataPaths,
		Definition:    string(plain),
	}, nil
}

// foreignVMTarget is foreignContainerTarget for the vms domain: read the encrypted
// definition from the foreign repo's vm-def dir (legacy sibling fallback), decrypt
// with the SESSION key, and return the LOCAL VM target it WOULD become (including
// the graceful-method default) WITHOUT persisting. prepareForeignRestore adopts it
// only after the restore validates.
func (s *Service) foreignVMTarget(sess foreignSession, name string) (store.VMTarget, error) {
	fn, err := defFileName(name)
	if err != nil {
		return store.VMTarget{}, err
	}
	enc, err := readStoredDef(filepath.Join(sess.repo, "vm-def"), filepath.Join(filepath.Dir(sess.repo), "bombvault-vm-defs"), fn)
	if err != nil {
		return store.VMTarget{}, fmt.Errorf("the foreign repository holds no readable definition for vm %q, so it cannot be recreated here", name)
	}
	plain, err := secret.Decrypt(sess.key, enc)
	if err != nil {
		return store.VMTarget{}, fmt.Errorf("the stored definition for %q does not decrypt with this session's APP_KEY", name)
	}
	var def vmDefinition
	if err := json.Unmarshal(plain, &def); err != nil {
		return store.VMTarget{}, fmt.Errorf("foreign definition for %q is corrupt: %w", name, err)
	}
	method := def.Method
	if method == "" {
		method = "graceful"
	}
	return store.VMTarget{
		Name:       name,
		Method:     method,
		Definition: string(plain),
	}, nil
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

	// Create the target dir ONLY after every validation passed. Readable (0o755)
	// so the operator's non-root SMB user can read what root restored onto the
	// user-visible share (see EnsureDirReadable).
	if err := paths.EnsureDirReadable(target); err != nil {
		return fileSetRestorePlan{}, fmt.Errorf("create target folder: %w", err)
	}
	return fileSetRestorePlan{
		repo:       sess.repo,
		mode:       sess.mode,
		snapshotID: snapshotID,
		setID:      setID,
		setName:    item,
		target:     target,
		// Subtree from the SNAPSHOT itself (Paths[0]) so the to-folder restore drops
		// the set's contents directly into target instead of nesting the absolute
		// path (issue #62); "" (path-less snapshot) → whole-tree fallback.
		subtree: snapshotSubtree(snaps, snapshotID),
	}, nil
}

// prepareForeignFileSetFilesRestore is the selective (pick-some-files/subfolder)
// counterpart of prepareForeignFileSetRestore: it restores only the chosen paths
// of a foreign file set into a target folder — the manilx #123 case, pulling one
// stack subfolder out of a whole-appdata set. Cross-instance is ALWAYS to a folder
// (never in place: writing the source host's absolute paths onto this host is the
// #122 class of bug), so a target is required. Snapshot ownership + local-set
// adoption mirror the whole-set path; the selection's containment + target guards
// live in the shared buildFileSetFilesPlan.
func (s *Service) prepareForeignFileSetFilesRestore(ctx context.Context, sess foreignSession, item, snapshotID, targetSubPath string, filePaths []string) (fileSetFilesRestorePlan, error) {
	if strings.TrimSpace(targetSubPath) == "" {
		return fileSetFilesRestorePlan{}, errors.New("a target folder is required to restore files from a foreign repository")
	}
	// Snapshot ownership: an explicit id must be well-formed hex AND belong to
	// THIS item's fileset:<Name> tag in the SESSION repo; "latest"/"" resolves to
	// the newest matching snapshot (restic lists oldest-first).
	explicitID := snapshotID != "latest" && snapshotID != ""
	if explicitID && !backup.ValidSnapshotID(snapshotID) {
		return fileSetFilesRestorePlan{}, backup.ErrInvalidSnapshotID
	}
	snaps, err := s.snapshotsForTag(ctx, sess.repo, sess.mode, "fileset:"+item)
	if err != nil {
		return fileSetFilesRestorePlan{}, err
	}
	if explicitID {
		if !snapshotBelongs(snaps, snapshotID) {
			return fileSetFilesRestorePlan{}, fmt.Errorf("snapshot %s does not belong to this file set", snapshotID)
		}
	} else {
		if len(snaps) == 0 {
			return fileSetFilesRestorePlan{}, errors.New("no backups found for this file set")
		}
		snapshotID = snaps[len(snaps)-1].ID
	}

	// Adopt the name locally when unknown (disabled + path-less, like DiscoverFileSets)
	// so the restore run records against a stable file_sets.id; an existing local set
	// of the same name is reused untouched. Mirrors the whole-set foreign path.
	setID := ""
	if set, gErr := s.store.GetFileSetByName(item); gErr == nil {
		setID = set.ID
	} else {
		created, cErr := s.store.CreateFileSet(store.FileSet{Name: item, Enabled: false})
		if cErr != nil {
			return fileSetFilesRestorePlan{}, fmt.Errorf("adopt file set %q: %w", item, cErr)
		}
		setID = created.ID
	}

	// The shared builder does the strict snapshot/selection validation, the
	// subtree-containment traversal guard, target resolution + EnsureDirReadable.
	return s.buildFileSetFilesPlan(snaps, snapshotID, setID, item, sess.repo, sess.mode, filePaths, targetSubPath)
}

// ListForeignFiles lists the files of one file set's snapshot in an open foreign
// session, so the recovery UI can offer a subfolder/file picker before a selective
// restore. It is the foreign, session-scoped twin of ListSnapshotFilesFileSet:
// read-only (sess.mode carries NoLock), tag-scoped to the item so one set's tree
// can't be listed through another's id, and it never touches local repos.
func (s *Service) ListForeignFiles(ctx context.Context, sessionID, domain, item, snapshotID string) ([]restic.FileEntry, error) {
	if domain != "files" {
		return nil, errors.New("file listing is only available for the files domain")
	}
	if !validResourceName(item) {
		return nil, errors.New("invalid item name")
	}
	sess, err := s.foreignSession(sessionID)
	if err != nil {
		return nil, err
	}
	explicitID := snapshotID != "latest" && snapshotID != ""
	if explicitID && !backup.ValidSnapshotID(snapshotID) {
		return nil, backup.ErrInvalidSnapshotID
	}
	snaps, err := s.snapshotsForTag(ctx, sess.repo, sess.mode, "fileset:"+item)
	if err != nil {
		return nil, err
	}
	if explicitID {
		if !snapshotBelongs(snaps, snapshotID) {
			return nil, fmt.Errorf("snapshot %s does not belong to this file set", snapshotID)
		}
	} else {
		if len(snaps) == 0 {
			return nil, errors.New("no backups found for this file set")
		}
		snapshotID = snaps[len(snaps)-1].ID
	}
	return s.engine.Ls(ctx, sess.repo, snapshotID, sess.mode)
}

// ForeignBindWarning is one NON-appdata bind of a foreign container that points at
// a pool/share this host does not have mounted — so after a cross-pool restore the
// recreated container's bind would land on empty/missing storage. The operator
// fixes these in the Unraid template; appdata binds are remapped automatically and
// never listed here (#125, Q1: "appdata only + warning").
type ForeignBindWarning struct {
	Host      string `json:"host"`
	Container string `json:"container"`
}

// ForeignContainerBindWarnings inspects a foreign container's recipe and returns
// the non-appdata binds whose source pool is not mounted on THIS host. Host
// devices/sockets (docker.sock, /etc/localtime, /dev/dri — anything not under the
// source mount root) are not pool binds and are skipped; appdata binds are
// remapped by the restore and skipped. Read-only, session-scoped.
func (s *Service) ForeignContainerBindWarnings(_ context.Context, sessionID, item string) ([]ForeignBindWarning, error) {
	if !validResourceName(item) {
		return nil, errors.New("invalid item name")
	}
	sess, err := s.foreignSession(sessionID)
	if err != nil {
		return nil, err
	}
	tg, err := s.foreignContainerTarget(sess, item)
	if err != nil {
		return nil, err
	}
	var def containerDefinition
	if err := json.Unmarshal([]byte(tg.Definition), &def); err != nil {
		return nil, fmt.Errorf("foreign definition for %q is corrupt: %w", item, err)
	}
	return s.foreignBindWarnings(def.Inspect.HostConfig.Binds, tg.AppdataPaths), nil
}

// foreignBindWarnings is the pure classification behind ForeignContainerBindWarnings
// (unit-tested in isolation): a bind is warned only when its host source is a
// pool/share path (reachable through the mount) that is NOT one of the container's
// appdata paths (those are remapped) AND is NOT mounted on this host. Host
// devices/sockets (docker.sock, /etc/localtime, /dev/dri — not under the source
// mount root) are skipped.
func (s *Service) foreignBindWarnings(binds, appdataPaths []string) []ForeignBindWarning {
	appdata := make(map[string]bool, len(appdataPaths))
	for _, p := range appdataPaths {
		appdata[path.Clean(p)] = true
	}
	var out []ForeignBindWarning
	for _, b := range binds {
		host, container, found := strings.Cut(b, ":")
		if !found {
			continue
		}
		cp, ok := s.toContainerPath(host)
		if !ok {
			continue // host device/socket, not a pool bind
		}
		if appdata[path.Clean(cp)] {
			continue // appdata bind — remapped automatically
		}
		if !s.destinationMounted(cp) {
			out = append(out, ForeignBindWarning{Host: host, Container: container})
		}
	}
	return out
}
