// Package api wires the real adapters (dockercli, restic, store, template,
// paths) into the dependency-injected backup orchestrator and exposes the
// JSON HTTP API plus the embedded SPA server.
//
// The DI seam is preserved: internal/backup imports only its own interfaces.
// All concrete-adapter wiring lives here in the service layer.
package api

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/dockercli"
	"github.com/junkerderprovinz/bombvault/internal/platform"
	"github.com/junkerderprovinz/bombvault/internal/progress"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
	"github.com/junkerderprovinz/bombvault/internal/virshcli"
)

// ResticEngine is the subset of *restic.Restic the service depends on. Defining
// it here (with the real restic.Mode/Summary/Snapshot types) lets the service be
// unit-tested with a fake engine without a real restic binary, while *restic.Restic
// satisfies it directly in production.
type ResticEngine interface {
	Init(ctx context.Context, repo string, mode restic.Mode) error
	// RepoOpens reports whether the repo opens (and decrypts) with mode, a cheap
	// probe via `restic cat config`. EnsureRepo uses it to reconcile the
	// configured mode with the repo's actual one.
	RepoOpens(ctx context.Context, repo string, mode restic.Mode) bool
	// RepoOpensErr is RepoOpens but returns the probe's actual failure instead of
	// discarding it, for TestOffsite, which needs to explain why a repo didn't open.
	RepoOpensErr(ctx context.Context, repo string, mode restic.Mode) error
	Backup(ctx context.Context, repo string, paths, tags []string, mode restic.Mode, excludes ...string) (restic.Summary, error)
	// BackupStdin backs up all of rd as a single synthetic file recorded under
	// path. The zvol VM disk backup pipes a `zfs send` stream through it straight
	// into `restic backup --stdin`, with no local staging file; see
	// backup.ZvolRestic.
	BackupStdin(ctx context.Context, repo string, rd io.Reader, path string, tags []string, mode restic.Mode) (restic.Summary, error)
	RestorePath(ctx context.Context, repo, snapshotID, path string, mode restic.Mode) error
	// DumpRaw streams the synthetic file at path from the given snapshot into w.
	// It is the restore-side counterpart of BackupStdin and feeds a `zfs receive`
	// over SSH.
	DumpRaw(ctx context.Context, repo, snapshotID, path string, w io.Writer, mode restic.Mode) error
	// DumpZip streams a snapshot subtree (rooted at subfolder) as a zip into w.
	// The flash restore offers it as a download: the live /boot is never touched
	// and no filesystem metadata is restored.
	DumpZip(ctx context.Context, repo, snapshotID, subfolder string, w io.Writer, mode restic.Mode) error
	Snapshots(ctx context.Context, repo string, mode restic.Mode) ([]restic.Snapshot, error)
	Forget(ctx context.Context, repo string, snapshotIDs []string, prune bool, mode restic.Mode) error
	// ForgetPolicy applies a keep-policy (retention). Inert when the policy has
	// no dimension set. tags scopes the policy to one item's snapshots as a
	// single group, so a change of the item's paths or a rename does not leave
	// its older snapshots in a group that never ages out; empty tags fall back
	// to the repo-wide paths-grouped pass. prune reclaims freed space in the
	// same run; batch callers pass false and Prune once at the end.
	ForgetPolicy(ctx context.Context, repo string, p restic.RetentionPolicy, mode restic.Mode, tags []string, prune bool) error
	// Ls lists the files in a snapshot (for file-level restore).
	Ls(ctx context.Context, repo, snapshotID string, mode restic.Mode) ([]restic.FileEntry, error)
	// LsStream lists a snapshot's nodes like Ls but hands each entry to onEntry
	// as it is read, retaining none of them. The exclusion assistant scans whole
	// appdata trees this way; Ls would buffer the entire listing (1.36 GiB on a
	// 672k-node snapshot).
	LsStream(ctx context.Context, repo, snapshotID string, mode restic.Mode, onEntry func(restic.FileEntry)) error
	// LsPath lists one directory's own node plus its direct children, scoped to
	// dirPath (a subtree root). A remapped restore reads the directory's original
	// owner and mode back from it, because restic's restorer never re-applies
	// that metadata to the subtree root it creates.
	LsPath(ctx context.Context, repo, snapshotID, dirPath string, mode restic.Mode) ([]restic.FileEntry, error)
	// RestoreInclude restores a single path from a snapshot to target (file-level
	// restore; target "/" = in-place to its original location).
	RestoreInclude(ctx context.Context, repo, snapshotID, includePath, target string, mode restic.Mode) error
	// RestoreSubtreeTo restores the subtree at subtreePath from a snapshot into
	// target, which receives the subtree's contents directly with no absolute-path
	// nesting (#62). The files to-folder restore uses it; subtreePath is the
	// snapshot's own backed-up path.
	RestoreSubtreeTo(ctx context.Context, repo, snapshotID, subtreePath, target string, mode restic.Mode) error
	// RestoreSubtreeInclude is RestoreSubtreeTo with an --include filter: only
	// includePath (relative to subtreePath) is written under target. It powers
	// the selective files restore into a folder.
	RestoreSubtreeInclude(ctx context.Context, repo, snapshotID, subtreePath, includePath, target string, mode restic.Mode) error
	// Check verifies repository structure + metadata integrity (restic check).
	Check(ctx context.Context, repo string, mode restic.Mode) error
	// CheckData runs a restore-readiness drill: `restic check
	// --read-data-subset=<pct>%`, which reads back and re-verifies a random subset
	// of the real pack data (not just metadata), proving the backup is restorable.
	CheckData(ctx context.Context, repo string, subsetPercent int, mode restic.Mode) error
	// Unlock removes locks from the repo (restic unlock). removeAll clears every
	// lock, not just stale ones.
	Unlock(ctx context.Context, repo string, removeAll bool, mode restic.Mode) error
	// Prune reclaims space freed by forgotten snapshots (restic prune).
	Prune(ctx context.Context, repo string, mode restic.Mode) error
	// CacheCleanup removes old per-repo cache directories (`restic cache
	// --cleanup`). It opens no repository and works on the local cache base dir
	// (RESTIC_CACHE_DIR), as part of the persistent-cache size trim.
	CacheCleanup(ctx context.Context) error
	// Copy replicates snapshots from srcRepo into destRepo (restic copy) for
	// off-site backup. Empty ids copy everything not already in dest. lim caps the
	// transfer bandwidth (zero = unlimited) so replication doesn't saturate the WAN.
	Copy(ctx context.Context, destRepo, srcRepo string, snapshotIDs []string, lim restic.Limits, mode restic.Mode) error
	// Stats returns repository statistics for the chosen --mode ("raw-data" for
	// the physical/deduplicated size + blob count; "restore-size" for the logical
	// size + file count). Used to sample the repo-size trend.
	Stats(ctx context.Context, repo, mode string, m restic.Mode) (restic.StatsResult, error)
	// StatsRestoreSize returns the logical restore size (bytes) and file count of
	// one snapshot (`restic stats --mode restore-size <snap>`). The DR drill
	// compares it against an on-disk walk of the restored sandbox.
	StatsRestoreSize(ctx context.Context, repo, snapshotID string, m restic.Mode) (files int, bytes int64, err error)
	// Diff compares two snapshots (restic diff --json) and returns the summary
	// counts + byte totals (what changed between two backups).
	Diff(ctx context.Context, repo, snap1, snap2 string, m restic.Mode) (restic.DiffResult, error)
	// TagAdd adds tags to a snapshot (restic tag --add). Tags must be
	// pre-sanitised by the caller (restic tags are comma-separated).
	TagAdd(ctx context.Context, repo, snapID string, tags []string, m restic.Mode) error
}

// compile-time check: the real adapter satisfies the seam.
var _ ResticEngine = (*restic.Restic)(nil)

// HostSSH is the subset of sshconn the service uses: NVRAM/TPM transfer for VM
// backup/restore plus the public key and reachability test for the UI. A nil
// HostSSH means VM-over-SSH features degrade gracefully (NVRAM/TPM capture is
// skipped; the UEFI restore falls back to EnsureNVRAMTemplate).
type HostSSH interface {
	ReadFile(ctx context.Context, path string) ([]byte, error)
	WriteFile(ctx context.Context, path string, data []byte) error
	PublicKey() (string, error)
	Test(ctx context.Context) error
	// Run executes a command on the host over SSH (args are shell-quoted). Used to
	// trigger Unraid's native notification script.
	Run(ctx context.Context, args ...string) (string, error)
	// EnsureKnownHost pins the host key (raw ssh accept-new) before libvirt's
	// qemu+ssh transport verifies it, so virsh doesn't fail on an empty
	// known_hosts. Also confirms key auth.
	EnsureKnownHost(ctx context.Context) error
	// StreamCommand starts a command on the host over SSH and streams its stdout
	// without buffering; the zvol VM disk backup reads `zfs send` through it.
	StreamCommand(ctx context.Context, args ...string) (io.ReadCloser, func() error, error)
	// RunWithStdin runs a command on the host over SSH with rd streamed to its
	// stdin; the zvol VM disk restore feeds `zfs receive` through it.
	RunWithStdin(ctx context.Context, rd io.Reader, args ...string) error
}

// Service bridges the real adapters to the backup orchestrator's interfaces.
type Service struct {
	cfg      config.Config
	store    *store.Repo
	docker   dockercli.Docker
	virsh    virshcli.Virsh
	engine   ResticEngine
	ssh      HostSSH         // optional; nil = no SSH (VM NVRAM transfer skipped)
	progress *progress.Store // optional; nil = progress reporting disabled
	// hostShell runs the "Backup Everything" global pre/post hook commands in
	// BombVault's own container (see hostshell.go). NewService sets the real
	// execHostShell, so it is never nil in production; tests override it with
	// SetHostShell.
	hostShell HostShell
	// platform is the detected or injected Platform adapter (Unraid, generic, ...)
	// behind the appdata fallback, the cross-instance restore-destination defaults
	// and the Unraid update-status reconcile. nil means platform.Unraid{} (see
	// platformFn), so a Service built as a bare &Service{...} literal, as most
	// tests here do, behaves as on Unraid instead of panicking on a nil interface.
	platform platform.Platform
	// platformMismatchOnce guards warnUnraidPlatformMismatch: an operator whose
	// notify.Config.Unraid is on but whose platform detection did not resolve to
	// Unraid gets one diagnostic log line per process, not one per notification
	// (a busy day with many failed backups would otherwise drown the log in
	// copies of the same explanation). The zero value is ready to use.
	platformMismatchOnce sync.Once
	// resticCacheDir is where restic keeps its persistent cache, the path main.go
	// exports as RESTIC_CACHE_DIR (via SetResticCacheDir). Empty means restic's
	// unmanaged default location, and the size-based trim is skipped. See
	// TrimResticCache.
	resticCacheDir string
	// diskFree is the storage forecast's free-space probe seam: nil (the normal
	// case) uses the platform statfs implementation (diskFreeBytes); tests
	// inject a fake. Accessed via diskFreeFn.
	diskFree func(path string) (uint64, error)
	// dirNonEmptyProbe is the container-restore overwrite guard's "does this
	// destination already hold data" seam: nil uses the real filesystem
	// (dirNonEmpty); tests inject a fake. Accessed via dirNonEmptyFn.
	dirNonEmptyProbe func(path string) bool
	// repoMu serialises operations per domain repo. A backup holds its domain's
	// lock for the whole run; maintenance (unlock/prune/delete) TryLocks and
	// reports "busy" instead, so a destructive `restic unlock --remove-all` /
	// prune can never run against a repo a backup is actively writing.
	repoMu map[string]*sync.Mutex

	// domainActivity names the operation currently holding each domain's repoMu
	// ("backup"|"restore"|"prune"|"verify"|"replicate"|"delete"|"unlock"|
	// "maintenance"), so backup starters can return a clear busy error instead of
	// launching a goroutine that then blocks silently on the mutex. Guarded by
	// activityMu; set when a lock is acquired, cleared (defer) on release.
	activityMu     sync.Mutex
	domainActivity map[string]string

	// runCancels maps a running restore's progress key ("container:<name>" /
	// "vm:<name>" / "to:<path>" / "stack:<project>") to the CancelFunc of its
	// detached context, so POST /api/restore/cancel can stop an in-flight restore
	// by key. Registered on launch, deleted (defer) when the run finishes. Guarded
	// by cancelMu. Cancelling an unknown/finished key is a harmless no-op.
	cancelMu   sync.Mutex
	runCancels map[string]context.CancelFunc

	// backupCancels does for backups what runCancels does for restores. It is a
	// separate map because shutdown must stop one kind and never the other:
	// interrupting a backup is safe, since restic writes the snapshot last and an
	// aborted run leaves only unreferenced data for the next prune, while
	// interrupting a restore is destructive, because the container is gone and
	// its appdata half-written (see restoreTimeout). On SIGTERM backups are
	// cancelled and restores left alone. A shared map with a key prefix would
	// cancel a restore the first time a caller forgot the prefix.
	backupCancels map[string]context.CancelFunc

	// cancelledBackups marks the keys a user cancelled (#200), so the run about to
	// fail with a context error is recorded as "cancelled" instead. It shares
	// backupCancels' guard and lifetime (set by CancelBackupRun, cleared by
	// unregisterBackupCancel) and is a separate map because shutdown must still be
	// able to call the cancel func after a user cancellation raced ahead of it.
	cancelledBackups map[string]bool

	// shuttingDown is set once by BeginShutdown and never cleared.
	// runsAdapter.Finish reads it to record a run cut short by shutdown as
	// aborted rather than failed, so the history says the server went down
	// instead of showing an unexplained error.
	shuttingDown atomic.Bool

	// selfName is BombVault's own container name, resolved once and cached, so a
	// backup never stops the process doing the backing up.
	selfMu       sync.Mutex
	selfName     string
	selfResolved bool

	// batchActive is the single-flight guard shared by every server-side backup
	// and restore starter (single, batch, VM, flash, restore in place, restore
	// files, restore to folder). A second request is answered "already running"
	// instead of overlapping, since they contend on repo locks and container
	// stop/start.
	batchActive atomic.Bool

	// everythingActive is the single-flight guard for a "Backup Everything" pass
	// (everything.go), scheduled or started via StartBackupEverything. It is
	// separate from batchActive because a pass drives the same per-domain
	// starters batchActive guards; sharing it would make a routine
	// single-container backup refuse to start during an unrelated pass. Each
	// domain's own lock (lockDomain) still governs contention at the repo level;
	// this guard only stops a second pass from overlapping the first.
	everythingActive atomic.Bool

	// suggestMu guards suggestCache, the exclusion assistant's snapshot-aggregate
	// cache: one entry per container, keyed inside on (repo, snapshot id,
	// resolved excludes), so a rescan is instant until the next backup writes a
	// newer snapshot. One entry per container bounds the map without an eviction
	// policy.
	// suggestFlights is the singleflight for that cache: one in-flight snapshot
	// aggregate per key, so a refresh, a second browser tab or two viewers do not
	// each spawn their own `restic ls` against the same repo inside a
	// memory-capped container. Guarded by suggestMu as well.
	suggestMu      sync.Mutex
	suggestCache   map[string]suggestCacheEntry
	suggestFlights map[string]*suggestFlight

	// budgetMu guards offsiteOverBudget, the per-domain latch for an off-site repo
	// over its growth budget. The alarm fires once when the latch goes from false
	// to true, not on every replication while over budget; the latch clears when
	// growth drops back under budget so a later breach alarms again.
	budgetMu          sync.Mutex
	offsiteOverBudget map[string]bool

	// statsMu guards statsRunning, the in-flight set for repo-size sampling, keyed
	// "<domain>/<source>". The 20-hour throttle in front of every sampling path
	// reads the newest repo_stats row, which is only written when a sample
	// finishes (CollectStats' AddRepoStat, after `snapshots` and two `stats`
	// runs). While one sample walks a repo, the throttle still sees the old
	// timestamp and waves every other caller through. Without this set a
	// container round starts one detached three-command probe per container
	// against the repo it writes to, all with --no-lock, and pins the CPU with
	// nine concurrent restic processes (#189).
	//
	// It is an in-flight set rather than a singleflight because no caller reads a
	// sample's return value (they read the row later), so there are no followers
	// to hand a result to. A loser simply skips. A finished sample writes its row
	// and the throttle takes over; a failed one writes nothing and releases, so
	// the next backup retries immediately.
	statsMu      sync.Mutex
	statsRunning map[string]bool

	// receiverCheckMu guards receiverChecking, the in-flight set of received-repo
	// integrity checks keyed by repo id. It works like statsRunning with more at
	// stake: the gated work is `restic check`, optionally with
	// --read-data-subset, which re-reads pack data.
	//
	// The scheduled sweep gates on ReceivedRepo.LastCheckAt, which is stamped when
	// the previous check finished, and received repos sit outside repoMu (their
	// location is none of the five domains). The sweep runs sequentially, but a
	// manual POST /api/receiver/repos/{id}/check, a second tab, or a manual check
	// landing on the nightly sweep would otherwise read the same repository's
	// pack data twice at once.
	receiverCheckMu  sync.Mutex
	receiverChecking map[string]bool

	// cacheTrimming is TrimResticCache's own one-at-a-time flag. See its comment:
	// the after-bulk hook it rides fires once per per-item cron entry, not once
	// per night.
	cacheTrimming atomic.Bool

	// tamperMu serialises RunTamperTest per domain so reading the previous
	// verdict, recording the new one and notifying happen atomically: two
	// concurrent tests can't both see the old verdict and double-fire (or
	// interleave and drop) the protection-loss alert. It is separate from repoMu
	// because a tamper test touches only the tamper history, not repo state, and
	// it is created lazily (tamperMuGuard) so it works however the Service was
	// constructed.
	tamperMuGuard sync.Mutex
	tamperMu      map[string]*sync.Mutex

	// foreignMu guards foreignSessions: short-lived, in-memory, read-only
	// sessions on another BombVault instance's repository (Recovery, "restore from
	// another repo", #61). They are never persisted: closing or expiring a session
	// forgets the foreign location and key, and Settings stays untouched (see
	// foreign.go). Created lazily so it works however the Service was
	// constructed.
	//
	// foreignJanitor is the stop channel of the background sweeper started by the
	// first OpenForeign (nil when not running); closing it stops the goroutine,
	// and the next open re-creates it. The sweeper drops expired sessions and
	// their foreign APP_KEY without waiting for another API call.
	// foreignSweepEvery overrides the sweep interval in tests (0 means the
	// production default).
	foreignMu         sync.Mutex
	foreignSessions   map[string]foreignSession
	foreignJanitor    chan struct{}
	foreignSweepEvery time.Duration

	// detectMu guards detectFlight, the single in-flight encryption-detection
	// pass (encryption_detect.go). The Recovery page fires POST
	// /api/encryption/detect on mount, and a pass can take minutes when a
	// configured off-site host is dead, so a second tab or a reload joins the
	// running pass instead of forking a second set of restic probes against the
	// same repositories. nil means no pass is running; the zero value is ready to
	// use.
	detectMu     sync.Mutex
	detectFlight *encryptionDetectFlight

	// placementMu guards a domain's placement state across a read-then-write: two
	// callers pausing the same domain at once (a replication pass and a
	// background listing, say) do not both try to insert its row and only one
	// notification fires. confirmDefault, moveFileSetRule and an item PATCH's
	// writeItemPlacement each hold it for one read-then-write of their own, so a
	// PATCH's home and copies land as a single atomic step and another PATCH to
	// the same item never lands in between.
	placementMu sync.Mutex

	// listingMu guards listing, the (domain, target) pairs being listed in the
	// background, so a second request for the same pair does not list it twice.
	listingMu sync.Mutex
	listing   map[string]bool
}

// lockTamper blocks until it holds domain's tamper lock and returns the unlock
// func, lazily creating the per-domain mutex. Serialises RunTamperTest so the
// read-prev → record → notify sequence is atomic (see the tamperMu field).
func (s *Service) lockTamper(domain string) func() {
	s.tamperMuGuard.Lock()
	if s.tamperMu == nil {
		s.tamperMu = map[string]*sync.Mutex{}
	}
	mu := s.tamperMu[domain]
	if mu == nil {
		mu = &sync.Mutex{}
		s.tamperMu[domain] = mu
	}
	s.tamperMuGuard.Unlock()
	mu.Lock()
	return mu.Unlock
}

// NewService constructs the backup service.
func NewService(cfg config.Config, st *store.Repo, d dockercli.Docker, v virshcli.Virsh, eng ResticEngine) *Service {
	return &Service{
		cfg: cfg, store: st, docker: d, virsh: v, engine: eng,
		hostShell: execHostShell{},
		repoMu: map[string]*sync.Mutex{
			"containers": {},
			"vms":        {},
			"flash":      {},
			"config":     {},
			"files":      {},
		},
		domainActivity:    map[string]string{},
		runCancels:        map[string]context.CancelFunc{},
		offsiteOverBudget: map[string]bool{},
		suggestCache:      map[string]suggestCacheEntry{},
		suggestFlights:    map[string]*suggestFlight{},
	}
}

// errDomainBusy is returned by a maintenance op when a backup is holding the
// domain's lock (so it never disturbs an in-progress backup's repo).
var errDomainBusy = errors.New("a backup is currently running for this domain; try again when it finishes")

// setDomainActivity records the reason label for a currently-held domain lock.
func (s *Service) setDomainActivity(domain, reason string) {
	s.activityMu.Lock()
	if s.domainActivity == nil {
		s.domainActivity = map[string]string{}
	}
	s.domainActivity[domain] = reason
	s.activityMu.Unlock()
}

// clearDomainActivity drops the reason label when a domain lock is released.
func (s *Service) clearDomainActivity(domain string) {
	s.activityMu.Lock()
	delete(s.domainActivity, domain)
	s.activityMu.Unlock()
}

// domainBusy reports the activity label of a domain whose repo lock is held,
// and whether it is held at all. A backup starter uses it to refuse a busy
// domain up front instead of launching a goroutine that blocks silently on
// the mutex. A scheduler can still grab the lock right after the check; that
// rare window is acceptable.
func (s *Service) domainBusy(domain string) (string, bool) {
	s.activityMu.Lock()
	defer s.activityMu.Unlock()
	r, ok := s.domainActivity[domain]
	return r, ok
}

// lockDomainFor is lockDomain plus an activity label recorded for the hold, so
// domainBusy can report what is running. The returned closure clears the label
// and unlocks. A nil/absent mutex (unknown domain) is a no-op.
func (s *Service) lockDomainFor(domain, reason string) func() {
	mu := s.repoMu[domain]
	if mu == nil {
		return func() {}
	}
	mu.Lock()
	s.setDomainActivity(domain, reason)
	return func() {
		s.clearDomainActivity(domain)
		mu.Unlock()
	}
}

// lockDomain blocks until it holds the domain's repo lock and returns the unlock
// func (used by backups). A nil/absent mutex (unknown domain) is a no-op. The
// hold is labelled "backup"; non-backup holders call lockDomainFor with their own
// label so domainBusy can name what is running.
func (s *Service) lockDomain(domain string) func() { return s.lockDomainFor(domain, "backup") }

// tryLockDomainFor acquires the domain's repo lock without blocking, recording
// the reason label on success. It returns the unlock func and true, or
// (nil, false) when another op holds it.
func (s *Service) tryLockDomainFor(domain, reason string) (func(), bool) {
	mu := s.repoMu[domain]
	if mu == nil {
		return func() {}, true
	}
	if !mu.TryLock() {
		return nil, false
	}
	s.setDomainActivity(domain, reason)
	return func() {
		s.clearDomainActivity(domain)
		mu.Unlock()
	}, true
}

// tryLockDomain acquires the domain's repo lock without blocking. It returns the
// unlock func and true on success, or (nil, false) when a backup holds it (used
// by maintenance ops, which must not run against a repo being backed up). The
// hold is labelled "maintenance"; callers that want a precise label
// (prune/verify/delete/unlock) call tryLockDomainFor.
func (s *Service) tryLockDomain(domain string) (func(), bool) {
	return s.tryLockDomainFor(domain, "maintenance")
}

// backupHardCap returns the maximum wall-clock time a single backup run may
// hold its domain lock before it is force-cancelled, so a wedged run cannot
// hold the lock forever. BACKUP_MAX_HOURS configures it:
//
//	unset/empty          -> 48h (large or slow cloud backups of more than
//	                        1 TB routinely run longer than 12h).
//	N (positive integer) -> N hours.
//	0                    -> no hard cap (returns 0; callers keep
//	                        context.WithoutCancel with no deadline, and a
//	                        wedged run is still bounded by the scheduler
//	                        overlap guard).
//	invalid              -> a warning is logged and the 48h default is used.
func backupHardCap() time.Duration {
	const def = 48 * time.Hour
	raw := strings.TrimSpace(os.Getenv("BACKUP_MAX_HOURS"))
	if raw == "" {
		return def
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 0 {
		log.Printf("api: invalid BACKUP_MAX_HOURS=%q (want a non-negative integer number of hours), using default %v", raw, def) //nolint:gosec // G706: %q-quoted; no raw user bytes reach the log formatter
		return def
	}
	if n == 0 {
		return 0 // unlimited
	}
	return time.Duration(n) * time.Hour
}

// backupHoldCtx detaches ctx from the caller's cancellation (keeping its values)
// so a backup survives the triggering client disconnecting, and applies the
// configurable hard cap (backupHardCap). With an unlimited cap
// (BACKUP_MAX_HOURS=0) no deadline is set; the returned context is still
// cancelled by the deferred cancel func when the run returns.
func backupHoldCtx(ctx context.Context) (context.Context, context.CancelFunc) {
	base := context.WithoutCancel(ctx)
	if cap := backupHardCap(); cap > 0 {
		return context.WithTimeout(base, cap)
	}
	return context.WithCancel(base)
}

// drillLockWait is the most a scheduled drill waits for the per-domain lock to
// free (matching the backup cap); drillLockPoll is how often it retries the
// lock while waiting. They are vars so tests can shrink them to sub-second
// values.
var (
	drillLockWait = drillWaitCap()   // max a scheduled drill waits for the domain to free (matches the backup cap)
	drillLockPoll = 15 * time.Second // how often it re-tries the domain lock while waiting
)

// drillWaitCap derives the scheduled-drill lock wait from the backup cap: a drill
// waiting for a domain to free should never give up sooner than a backup can run.
// When the backup cap is unlimited (BACKUP_MAX_HOURS=0) the drill wait is bounded
// at an effectively unbounded 100 years (safe for time.Time.Add, unlike a
// max-int sentinel which would overflow into the past).
func drillWaitCap() time.Duration {
	if cap := backupHardCap(); cap > 0 {
		return cap
	}
	return 100 * 365 * 24 * time.Hour
}

// waitLockDomainFor acquires the per-domain lock, waiting up to drillLockWait by
// polling tryLock (so a wedged lock-holder can't block a scheduled drill forever
// or pile up goroutines). Returns (unlock, true) on acquire, (nil, false) on timeout.
func (s *Service) waitLockDomainFor(domain, reason string) (func(), bool) {
	deadline := time.Now().Add(drillLockWait)
	for {
		if unlock, ok := s.tryLockDomainFor(domain, reason); ok {
			return unlock, true
		}
		if time.Now().After(deadline) {
			return nil, false
		}
		time.Sleep(drillLockPoll)
	}
}

// SetHostSSH wires the SSH connection used for VM NVRAM transfer + the UI's
// key/test endpoints. Called from main after the key is ensured.
func (s *Service) SetHostSSH(ssh HostSSH) { s.ssh = ssh }

// SetProgress wires the live-progress store that backup/restore operations
// publish to (and the SSE endpoint subscribes to). Called from main.
func (s *Service) SetProgress(p *progress.Store) { s.progress = p }

// SetHostShell overrides the "Backup Everything" global hook's shell-exec
// adapter (see hostshell.go). NewService already defaults it to the real
// execHostShell adapter, so production callers never need this; it exists for
// test injection of a fake HostShell.
func (s *Service) SetHostShell(h HostShell) { s.hostShell = h }

// SetPlatform wires the detected Platform adapter (platform.Detect plus main's
// Kind->Platform mapping) behind the appdata fallback, the cross-instance
// restore-destination defaults and the Unraid update-status reconcile. Unset,
// platformFn falls back to platform.Unraid{}.
func (s *Service) SetPlatform(p platform.Platform) { s.platform = p }

// platformFn returns the configured Platform adapter, or platform.Unraid{}
// when none is set, so a Service built without SetPlatform (such as a bare
// &Service{...} in tests) behaves as on Unraid.
func (s *Service) platformFn() platform.Platform {
	if s.platform != nil {
		return s.platform
	}
	return platform.Unraid{}
}

// unraidGate reports whether an Unraid-only, best-effort host-SSH step (the
// webGUI notification mirror, every sendUnraidNotify call site) should run:
// the caller wants it (unraidWanted, notify.Config.Unraid), SSH is
// configured, and the detected or overridden platform is Unraid.
//
// The platform check stays a hard requirement because notify.Config.Unraid
// can be stale, for example in a settings.json copied from an old Unraid box
// onto a TrueNAS or generic Docker host. An Unraid-only command there (the
// webGUI notify script, the #116 PHP-over-SSH reconcile, the `plugin` CLI)
// can only fail and adds log noise on every run. The failIfCalledSSH fakes in
// platform_gate_internal_test.go fail the test the moment such a step is
// attempted on another platform.
//
// A mismatch is not silently treated as "feature off" either:
// platform.Detect's only Unraid signal is one marker (config/plugins/dockerMan)
// under the container's /host/boot mount, so an Unraid host whose mount is
// missing (an old template, a hand-edited container config) resolves to
// KindGeneric with SSH fully configured. warnUnraidPlatformMismatch logs an
// actionable diagnostic, so "the user turned this off" and "detection is
// wrong" can be told apart.
func (s *Service) unraidGate(unraidWanted bool) bool {
	if !unraidWanted || s.ssh == nil {
		return false
	}
	if s.platformFn().Kind() == platform.KindUnraid {
		return true
	}
	s.warnUnraidPlatformMismatch()
	return false
}

// warnUnraidPlatformMismatch logs, once per process, that notify.Config.Unraid
// is enabled but platform detection did not resolve to Unraid. It names the
// detected Kind and the likely fix, so an operator can tell "I turned this
// off" from "BombVault misdetected my host" without reading source.
func (s *Service) warnUnraidPlatformMismatch() {
	s.platformMismatchOnce.Do(func() {
		log.Printf("platform: notify.Config.Unraid is enabled but BombVault detected platform=%q (not %q). "+
			"Unraid-only host features (webGUI notifications, the update-status reconcile, the dashboard-tile "+
			"plugin) stay disabled. If this is an Unraid host, verify the host's /boot is bind-mounted to "+
			"/host/boot inside the container (see the BombVault Unraid template) and restart the container. "+
			"Detection looks for %s.",
			s.platformFn().Kind(), platform.KindUnraid, filepath.Join(s.cfg.FlashDir, "config/plugins/dockerMan"))
	})
}

// unraidPlatformMismatchError builds the user-facing refusal for a
// request-scoped, Unraid-only action (TestNotify's Unraid channel, the
// dashboard-tile plugin's install and remove in runDashPluginCmd) attempted
// while platform detection did not resolve to Unraid. feature names what is
// refused, such as "the Unraid notification channel".
//
// Unlike unraidGate's background paths, which only log because nobody waits
// on them, these are synchronous user actions, so the reason and the
// /host/boot hint go straight into the response.
//
// The *platformMismatchErr lets the message bypass handlers.go's scrubError,
// which strips every absolute path from an error before it reaches the client
// and would reduce the hint to "[path] is bind-mounted to [path]". /boot and
// /host/boot are BombVault's fixed, documented mount points, never a repo path
// or secret; errRestoreDestination and errRepoPathGuidance bypass the
// scrubber for the same reason.
func (s *Service) unraidPlatformMismatchError(feature string) error {
	return &platformMismatchErr{msg: fmt.Sprintf(
		"%s is only available on Unraid hosts (BombVault detected platform=%q; "+
			"if this is an Unraid host, verify the host's /boot is bind-mounted to /host/boot "+
			"inside the container, see the BombVault Unraid template, and restart the container)",
		feature, s.platformFn().Kind())}
}

// errUnraidPlatformMismatch is the scrubber-bypass sentinel unraidPlatformMismatchError's
// *platformMismatchErr satisfies via Is, matched by handlers.go's scrubError.
var errUnraidPlatformMismatch = errors.New("unraid platform mismatch")

// platformMismatchErr carries unraidPlatformMismatchError's ready-to-show
// message (see its doc comment) while still satisfying
// errors.Is(err, errUnraidPlatformMismatch) for the scrubber bypass.
type platformMismatchErr struct{ msg string }

func (e *platformMismatchErr) Error() string { return e.msg }

func (e *platformMismatchErr) Is(target error) bool { return target == errUnraidPlatformMismatch }

// progBegin marks a backup, restore or replicate as started for key/phase and
// returns a context carrying a restic sink that republishes each percentage,
// plus the StartedAt (Unix seconds) stamped on every event it publishes.
// Percent updates are throttled to whole-percent steps. Without a progress
// store the context is unchanged, but the timestamp is still real, so a
// caller such as copyToOffsite can use it without a nil-store special case.
//
// Every event, including the terminal progEnd one (which the caller must pass
// the same startedAt), carries it, so a client can render a live elapsed
// duration for the whole run (#159). It is returned rather than taken by each
// caller from time.Now().Unix() because two captures of what is meant to be
// one instant can straddle a second boundary, and copyToOffsite's heartbeat
// would then disagree with progBegin's timestamp.
func (s *Service) progBegin(ctx context.Context, key, phase string) (context.Context, int64) {
	startedAt := time.Now().Unix()
	if s.progress == nil {
		return ctx, startedAt
	}
	s.progress.Publish(progress.Event{Key: key, Phase: phase, Percent: 0, Active: true, StartedAt: startedAt})
	last := -1.0
	return progress.WithSink(ctx, func(pct float64) {
		// A multi-path restore runs one restic process per path, and each restarts
		// at ~0. A drop below the last value means a new process began, so reset the
		// throttle and let paths 2..N report live progress too.
		if pct < last {
			last = -1
		}
		if pct < 100 && pct-last < 1 {
			return // throttle: only forward ≥1% steps (always forward the final 100)
		}
		last = pct
		s.progress.Publish(progress.Event{Key: key, Phase: phase, Percent: pct, Active: true, StartedAt: startedAt})
	}), startedAt
}

// offsiteLastCopy holds the most recently published restic-copy percentage
// for one "offsite:<domain>" replication, across however many sequential
// targets a multiTarget loop runs. copyToOffsite's heartbeat goroutine
// republishes it, so its periodic "still alive" event shows the current state
// instead of a blank placeholder that overwrites what progBeginCopySink's
// sink last reported.
//
// progBeginCopySink's sink writes it on copyToOffsiteTarget's goroutine
// (restic.Copy calls the sink inline while scanning stdout) and the heartbeat
// reads it from its own ticker goroutine, hence the mutex. The zero value
// means no real update yet (valid=false), which the heartbeat must tell apart
// from an actual Percent:0 update.
type offsiteLastCopy struct {
	mu    sync.Mutex
	valid bool
	cp    progress.CopyProgress
	total int
}

// set records the latest live update. A nil receiver is a no-op so callers
// that don't care about heartbeat continuity (direct copyToOffsiteTarget unit
// tests) can pass a nil *offsiteLastCopy.
func (l *offsiteLastCopy) set(cp progress.CopyProgress, total int) {
	if l == nil {
		return
	}
	l.mu.Lock()
	l.valid, l.cp, l.total = true, cp, total
	l.mu.Unlock()
}

// get returns the latest recorded update, or ok=false if none was ever set
// (including when l is nil).
func (l *offsiteLastCopy) get() (cp progress.CopyProgress, total int, ok bool) {
	if l == nil {
		return progress.CopyProgress{}, 0, false
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.cp, l.total, l.valid
}

// progBeginCopySink installs a progress.CopySink on ctx for a `restic copy`
// call, so its live per-snapshot pack-copy progress (see restic.Copy, #159)
// reaches the same "offsite:<domain>" key and StartedAt as every other event
// of this replication. estimatedTotal is the caller's best-effort N for a
// "snapshot k of N" display (restic.PendingCopyIDs explains why it is only an
// estimate); when the live SnapshotIndex exceeds a real estimate, the
// published total is widened to match rather than claiming fewer snapshots
// than are visibly running.
//
// estimatedTotal 0 means the caller could not estimate at all (both snapshot
// listings failed, or restic's stricter dedup found work PendingCopyIDs did
// not) and is published unchanged as SnapshotTotal 0, the documented
// "unknown" on the wire (see progress.Event). Widening it to SnapshotIndex
// would fake a "snapshot 7 of 7" that a consumer cannot tell from a genuine
// final snapshot, and the run-level percentage the frontend derives from k/N
// (offsiteRunProgress in web/src/lib/progress.ts) would read ~99% for the
// whole run. With 0 the frontend falls back to its duration-only text.
// Percent updates are throttled like progBegin's (whole-percent steps), but a
// SnapshotIndex change always forwards immediately so "k of N" advances
// without waiting on the new snapshot's first percentage. Without a progress
// store ctx is returned unchanged.
//
// last, when non-nil, receives every value published here, so a heartbeat
// tick in copyToOffsite republishes the current percentage instead of a blank
// one (see offsiteLastCopy).
func (s *Service) progBeginCopySink(ctx context.Context, domain string, startedAt int64, estimatedTotal int, last *offsiteLastCopy) context.Context {
	if s.progress == nil {
		return ctx
	}
	key := "offsite:" + domain
	lastIndex := -1
	lastPct := -1.0
	return progress.WithCopySink(ctx, func(cp progress.CopyProgress) {
		if cp.SnapshotIndex == lastIndex && cp.Percent < 100 && cp.Percent-lastPct < 1 {
			return // throttle: only forward ≥1% steps within the same snapshot
		}
		lastIndex, lastPct = cp.SnapshotIndex, cp.Percent
		// Widen an undercounting estimate but never invent one: estimatedTotal 0
		// stays 0 ("unknown").
		total := estimatedTotal
		if total > 0 && cp.SnapshotIndex > total {
			total = cp.SnapshotIndex
		}
		last.set(cp, total)
		s.progress.Publish(progress.Event{
			Key: key, Phase: "replicate", Active: true, StartedAt: startedAt,
			Percent: cp.Percent, SnapshotIndex: cp.SnapshotIndex, SnapshotTotal: total,
		})
	})
}

// progEnd emits the terminal event for key/phase: 100% on success, 0% on
// failure (the UI hides the bar either way). startedAt must be the value the
// matching progBegin returned; without it a client-rendered elapsed duration
// vanishes during the ~0.8-2.5s the terminal event lingers in the frontend's
// progress map (see COMPLETE_LINGER_MS in web/src/lib/progress.ts and
// OffsiteIndicator's MIN_VISIBLE_MS). No-op without a progress store.
func (s *Service) progEnd(key, phase string, ok bool, startedAt int64) {
	if s.progress == nil {
		return
	}
	pct := 100.0
	if !ok {
		pct = 0
	}
	s.progress.Publish(progress.Event{Key: key, Phase: phase, Percent: pct, Active: false, StartedAt: startedAt})
}

// recoverOperation is deferred first in every backup, restore and
// replication goroutine below; defers run LIFO, so it runs last, after the
// other cleanup defers (releasing batchActive, unregistering a cancel key)
// have fired normally. It contains a panic to the one operation that raised
// it, logged here with a stack trace, instead of letting it reach the top of
// the goroutine and crash the whole process: the HTTP server, the SSE
// progress stream and every other domain's in-flight operation with it.
// internal/schedule.Scheduler's cron.Recover gives the cron-triggered path
// (the same svc.Backup, BackupVM etc., wired in cmd/bombvault/main.go) the
// same protection; see Scheduler.New.
//
// onPanic, when non-nil, is called only on a recovered panic, with a short
// message describing it. Callers use it to close out whatever run record
// this operation would otherwise leave "running" forever: without a process
// restart, store.Repo.ReapInterruptedRuns never runs, so nothing else will.
// It never runs on the normal path, so it may do a store lookup a
// happy-path caller would not want to pay for.
//
// errOut, when non-nil, receives the panic as an error, for a caller with
// its own error result (a per-item batch helper whose loop must keep going
// past one bad item) to propagate like any other failure. It is an out
// parameter rather than a return value because recover() only has an
// effect when called directly by a deferred function; called from a plain
// function that a defer merely invokes, it silently recovers nothing. So
// recoverOperation must always be the direct target of `defer`, never
// wrapped in a `defer func(){ ... }()` closure, and a return value would be
// unreachable from there. A caller with nothing to propagate to (every
// single-target Start* goroutine below) passes nil.
func (s *Service) recoverOperation(op string, errOut *error, onPanic func(msg string)) {
	r := recover()
	if r == nil {
		return
	}
	const stackSize = 64 << 10 // matches robfig/cron's own Recover chain wrapper
	buf := make([]byte, stackSize)
	buf = buf[:runtime.Stack(buf, false)]
	log.Printf("api: %s: panic recovered (operation aborted, process unaffected): %v\n%s", op, r, buf) //nolint:gosec // G706: op is a fixed literal, sometimes suffixed with a target name/id/domain already boundary-validated (validResourceName, validVMName, or a fixed switch of domain literals) before this goroutine started, never raw request input
	msg := fmt.Sprintf("internal error (recovered panic): %v", r)
	if onPanic != nil {
		onPanic(msg)
	}
	if errOut != nil {
		*errOut = errors.New(msg)
	}
}

// failStuckRun marks targetID's still-"running" run row as failed after a
// recovered panic (store.Repo.FailRunningRun), for recoverOperation callers
// that know only the run's target, not its run id: the orchestrator that
// called store.StartRun panicked before it reached store.FinishRun. Scoped
// to targetID, so it never disturbs another target's in-flight run (see
// FailRunningRun). msg is bounded by truncateRunErr, the cap every other
// run-error path applies. Best-effort: a store error is logged, never
// returned, since the caller has already logged the panic. A blank targetID
// (nothing resolved, or no target row yet) is a silent no-op: nothing was
// started, so nothing is stuck.
func (s *Service) failStuckRun(targetID, msg string) {
	if targetID == "" {
		return
	}
	if _, err := s.store.FailRunningRun(targetID, truncateRunErr(errors.New(msg))); err != nil {
		log.Printf("api: mark stuck run failed for target %q: %v", targetID, err) //nolint:gosec // G706: targetID is a store-generated id / fixed literal, %q-quoted
	}
}

// truncateRunErr scrubs and bounds an error message so it fits the
// runs.error column (mirrors the orchestrator's truncateErr).
//
// It applies scrubSecrets to every error except the sentinel types
// scrubBypassMessage (handlers.go) carves out for scrubError, which pass
// through unscrubbed for the same reason scrubError bypasses them: their
// path-shaped content (a host:port conflict list, a ZFS dataset name,
// /boot vs /host/boot) is the actionable content, not a leak. scrubSecrets'
// path regex matches any slash-containing token, so scrubbing those would
// turn "host port 8080/tcp is already used by container ..." into "host
// port 8080[path] ..." and eat a zvol rebase failure's dataset name.
//
// Every other error is scrubbed, not just restic-originated ones. restic's
// lastReason is already clean, and scrubbing it again is a no-op, but not
// every caller goes through restic first: tamper.go's tamperProbe and
// primary_remote.go's RunPrimaryTamperTest can surface a raw url.Parse
// error, whose Error() embeds the full unparsed input URL, credentials and
// all. runs.error reaches the UI (handleRuns embeds store.Run), the weekly
// digest (digest.go forwards run.Error to every notification channel) and
// the widget feed (widget.go's truncateWidgetError only limits length), so
// scrubbing here, at the one function that writes runs.error, protects all
// of them without depending on every non-restic call site.
func truncateRunErr(err error) string {
	if err == nil {
		return ""
	}
	msg := err.Error()
	if bypass, ok := scrubBypassMessage(err); ok {
		msg = bypass
	} else {
		msg = scrubSecrets(msg)
	}
	const max = 500
	if len(msg) > max {
		return msg[:max]
	}
	return msg
}

// domainRunTargetID maps a domain to the runs.target_id used for PruneDomain
// and CheckDomain's run records. Flash and config are singleton domains with
// no per-item table, so their maintenance runs reuse the same reserved ids
// their backup rows already use (store.FlashTargetID / store.ConfigTargetID).
// Containers/vms/files have no single target to attribute a whole-repo
// prune/verify to, so the domain name itself is used as a literal target_id.
// This is safe even though it is never a real hex/UUID target id: every query
// that derives a domain FROM target_id (LastSuccessful*, RunCounts, the everyN
// due-gate) filters `kind = 'backup'` first, so a 'prune'/'verify' row can
// never be picked up there and pollute another domain's numbers.
func domainRunTargetID(domain string) string {
	switch domain {
	case "flash":
		return store.FlashTargetID
	case "config":
		return store.ConfigTargetID
	default:
		return domain
	}
}

// shortID truncates a restic snapshot id to its short (8-char) form.
func shortID(id string) string {
	if len(id) > 8 {
		return id[:8]
	}
	return id
}

// humanBytes formats a byte count as a compact human-readable size.
func humanBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(b)/float64(div), "KMGTPE"[exp])
}
