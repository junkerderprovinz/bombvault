// Package api wires the real adapters (dockercli, restic, store, template,
// paths) into the dependency-injected backup orchestrator and exposes the
// JSON HTTP API plus the embedded SPA server.
//
// The DI seam is preserved: internal/backup imports only its own interfaces.
// All concrete-adapter wiring lives here in the service layer.
package api

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/ageseal"
	"github.com/junkerderprovinz/bombvault/internal/backup"
	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/dockercli"
	"github.com/junkerderprovinz/bombvault/internal/model"
	"github.com/junkerderprovinz/bombvault/internal/notify"
	"github.com/junkerderprovinz/bombvault/internal/paths"
	"github.com/junkerderprovinz/bombvault/internal/platform"
	"github.com/junkerderprovinz/bombvault/internal/progress"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/restickey"
	"github.com/junkerderprovinz/bombvault/internal/schedule"
	"github.com/junkerderprovinz/bombvault/internal/secret"
	"github.com/junkerderprovinz/bombvault/internal/selfrestore"
	"github.com/junkerderprovinz/bombvault/internal/store"
	"github.com/junkerderprovinz/bombvault/internal/template"
	"github.com/junkerderprovinz/bombvault/internal/virshcli"
)

// containerDefinition is the recreate recipe persisted at backup time so that
// restore works even after the container has been deleted from the host — and,
// when written (encrypted) to the backup storage, after BombVault's own /config
// is lost (full disaster recovery via Discover). It is self-contained: Inspect +
// the Unraid template + the appdata paths that were backed up.
type containerDefinition struct {
	Inspect      model.Inspect `json:"inspect"`
	TemplateXML  string        `json:"template_xml"`
	AppdataPaths []string      `json:"appdata_paths"`
}

// ResticEngine is the subset of *restic.Restic the service depends on. Defining
// it here (with the real restic.Mode/Summary/Snapshot types) lets the service be
// unit-tested with a fake engine without a real restic binary, while *restic.Restic
// satisfies it directly in production.
type ResticEngine interface {
	Init(ctx context.Context, repo string, mode restic.Mode) error
	// RepoOpens reports whether the repo can be opened (and decrypted) with mode —
	// a cheap existence + encryption-mode probe (`restic cat config`). Used by
	// EnsureRepo to reconcile the configured mode against the repo's actual mode.
	RepoOpens(ctx context.Context, repo string, mode restic.Mode) bool
	// RepoOpensErr is RepoOpens but returns the probe's actual failure instead of
	// discarding it, for TestOffsite, which needs to explain why a repo didn't open.
	RepoOpensErr(ctx context.Context, repo string, mode restic.Mode) error
	Backup(ctx context.Context, repo string, paths, tags []string, mode restic.Mode, excludes ...string) (restic.Summary, error)
	// BackupStdin backs up the ENTIRE content of rd as a single synthetic file
	// recorded under path, tagged with tags — the zvol VM disk backup path
	// (v8.0.0 VM service-layer integration, Task 2): a `zfs send` stream piped
	// straight into `restic backup --stdin`, no local staging file. See
	// backup.ZvolRestic's doc comment (resticZvolAdapter below satisfies it).
	BackupStdin(ctx context.Context, repo string, rd io.Reader, path string, tags []string, mode restic.Mode) (restic.Summary, error)
	RestorePath(ctx context.Context, repo, snapshotID, path string, mode restic.Mode) error
	// DumpRaw streams the synthetic file at path, from the given snapshot, into
	// w — the restore-side counterpart of BackupStdin, feeding a `zfs receive`
	// over SSH (see backup.ZvolRestic's doc comment).
	DumpRaw(ctx context.Context, repo, snapshotID, path string, w io.Writer, mode restic.Mode) error
	// DumpZip streams a snapshot subtree (rooted at subfolder) as a zip into w
	// (flash restore — a non-destructive zip download; the live /boot is never
	// touched and no filesystem metadata is restored).
	DumpZip(ctx context.Context, repo, snapshotID, subfolder string, w io.Writer, mode restic.Mode) error
	Snapshots(ctx context.Context, repo string, mode restic.Mode) ([]restic.Snapshot, error)
	Forget(ctx context.Context, repo string, snapshotIDs []string, prune bool, mode restic.Mode) error
	// ForgetPolicy applies a keep-policy (retention). Inert when the policy has
	// no dimension set. tag scopes the policy to one item's snapshots as a
	// single group (identity-stable retention, issue #91: path-grouped retention
	// froze an item's old snapshots forever once its backed-up path set
	// changed); tag=="" falls back to the repo-wide paths-grouped pass. prune
	// reclaims freed space in the same run — batch callers pass false and Prune
	// once at the end.
	ForgetPolicy(ctx context.Context, repo string, p restic.RetentionPolicy, mode restic.Mode, tag string, prune bool) error
	// Ls lists the files in a snapshot (for file-level restore).
	Ls(ctx context.Context, repo, snapshotID string, mode restic.Mode) ([]restic.FileEntry, error)
	// LsStream lists a snapshot's nodes like Ls but hands each entry to onEntry
	// as it is read, retaining none of them. The exclusion assistant's
	// snapshot-sized scan aggregates a whole appdata tree this way; Ls would
	// buffer the entire listing (measured 1.36 GiB on a 672k-node snapshot) —
	// see restic.LsStream's own doc comment.
	LsStream(ctx context.Context, repo, snapshotID string, mode restic.Mode, onEntry func(restic.FileEntry)) error
	// LsPath lists one directory's own node plus its direct children, scoped to
	// dirPath (a subtree root) — used to read back a restored directory's
	// original owner/mode after a remapped restore, since restic's restorer
	// never re-applies that metadata to the subtree root it creates.
	LsPath(ctx context.Context, repo, snapshotID, dirPath string, mode restic.Mode) ([]restic.FileEntry, error)
	// RestoreInclude restores a single path from a snapshot to target (file-level
	// restore; target "/" = in-place to its original location).
	RestoreInclude(ctx context.Context, repo, snapshotID, includePath, target string, mode restic.Mode) error
	// RestoreSubtreeTo restores the subtree at subtreePath from a snapshot INTO
	// target — target receives that subtree's contents directly, with NO
	// absolute-path nesting (issue #62). Used by the files to-folder restore;
	// subtreePath is the snapshot's own backed-up path.
	RestoreSubtreeTo(ctx context.Context, repo, snapshotID, subtreePath, target string, mode restic.Mode) error
	// RestoreSubtreeInclude restores ONLY includePath (relative to subtreePath)
	// from a snapshot's subtree INTO target — same subtree-rooting as
	// RestoreSubtreeTo (no /host/user/… nesting, issue #62) but with an --include
	// filter, so only the selected path is written under target. Powers the
	// files-domain selective ("restore these files into a folder") restore.
	RestoreSubtreeInclude(ctx context.Context, repo, snapshotID, subtreePath, includePath, target string, mode restic.Mode) error
	// Check verifies repository structure + metadata integrity (restic check).
	Check(ctx context.Context, repo string, mode restic.Mode) error
	// CheckData runs a restore-readiness drill: `restic check
	// --read-data-subset=<pct>%`, which reads back and re-verifies a random subset
	// of the real pack data (not just metadata), proving the backup is restorable.
	CheckData(ctx context.Context, repo string, subsetPercent int, mode restic.Mode) error
	// Unlock removes locks from the repo (restic unlock). removeAll clears ALL
	// locks, not just stale ones.
	Unlock(ctx context.Context, repo string, removeAll bool, mode restic.Mode) error
	// Prune reclaims space freed by forgotten snapshots (restic prune).
	Prune(ctx context.Context, repo string, mode restic.Mode) error
	// CacheCleanup removes old per-repo cache directories (`restic cache
	// --cleanup`). Opens no repository — it operates on the local cache base dir
	// (RESTIC_CACHE_DIR). Part of the persistent-cache size trim.
	CacheCleanup(ctx context.Context) error
	// Copy replicates snapshots from srcRepo into destRepo (restic copy) for
	// off-site backup. Empty ids copy everything not already in dest. lim caps the
	// transfer bandwidth (zero = unlimited) so replication doesn't saturate the WAN.
	Copy(ctx context.Context, destRepo, srcRepo string, snapshotIDs []string, lim restic.Limits, mode restic.Mode) error
	// Stats returns repository statistics for the chosen --mode ("raw-data" for
	// the physical/deduplicated size + blob count; "restore-size" for the logical
	// size + file count). Used to sample the repo-size trend.
	Stats(ctx context.Context, repo, mode string, m restic.Mode) (restic.StatsResult, error)
	// StatsRestoreSize returns the logical restore size (bytes) + file count of ONE
	// snapshot (`restic stats --mode restore-size <snap>`). The DR drill compares it
	// against an on-disk walk of the restored sandbox.
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
	// StreamCommand starts a command on the host over SSH and streams its
	// stdout (never buffered) — the zvol VM disk backup path's `zfs send`
	// (v8.0.0 VM service-layer integration, Task 2; see sshZFSHost below).
	StreamCommand(ctx context.Context, args ...string) (io.ReadCloser, func() error, error)
	// RunWithStdin runs a command on the host over SSH with its stdin fed from
	// rd, streamed — the zvol VM disk restore path's `zfs receive`.
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
	// BombVault's OWN container (see hostshell.go). Defaulted to the real
	// execHostShell adapter in NewService, so it is never nil in production;
	// SetHostShell overrides it for tests.
	hostShell HostShell
	// platform is the detected/injected Platform adapter (Unraid/generic/…)
	// for the appdata-fallback convention, cross-instance restore-destination
	// defaults, and the Unraid update-status reconcile step. Optional; nil
	// defaults to platform.Unraid{} via platformFn() — this is what every
	// Service built as a bare &Service{...} literal (the vast majority of
	// this package's existing tests) gets, reproducing bombvault's original
	// Unraid-only behavior exactly rather than panicking on a nil interface.
	platform platform.Platform
	// platformMismatchOnce guards warnUnraidPlatformMismatch: an operator whose
	// notify.Config.Unraid is on but whose platform detection did not resolve
	// to Unraid gets exactly ONE diagnostic log line per process, not one per
	// notification (a busy day with many failed backups would otherwise drown
	// the log in copies of the same explanation). Zero value is ready to use.
	platformMismatchOnce sync.Once
	// resticCacheDir is the on-disk location of restic's persistent cache — the
	// same path main.go exports to the engine as RESTIC_CACHE_DIR (set via
	// SetResticCacheDir). Empty means the cache lives at restic's default
	// (unmanaged) location, so the size-based trim is skipped. See TrimResticCache.
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

	// self-container detection (resolved once, cached): the name of BombVault's
	// OWN container, so a backup never stops the process doing the backing up.
	selfMu       sync.Mutex
	selfName     string
	selfResolved bool

	// batchActive is the shared single-flight guard for every server-side
	// backup AND restore starter (single, batch, VM, flash, restore-in-place,
	// restore-files, restore-to-folder): only one can be in flight at a time (a
	// second request is answered "already running" instead of overlapping —
	// they contend on repo locks and container stop/start).
	batchActive atomic.Bool

	// everythingActive is the single-flight guard for a "Backup Everything" pass
	// (internal/api/everything.go): only one pass — scheduled or manually
	// triggered via StartBackupEverything — can be in flight at a time. Separate
	// from batchActive on purpose: a pass drives the very same per-domain
	// starters batchActive already guards (s.Backup/s.BackupVM/…), so reusing
	// batchActive here would make a routine single-container backup refuse to
	// start merely because an unrelated "Backup Everything" pass is mid-flight,
	// and vice versa. Each domain step's own existing lock (s.lockDomain) still
	// governs contention between the two at the repo level — this guard only
	// stops a SECOND "Backup Everything" pass from overlapping the first
	// (design spec, decision 7).
	everythingActive atomic.Bool

	// suggestMu guards suggestCache, the exclusion assistant's snapshot-aggregate
	// cache: ONE entry per container, keyed inside on (repo, snapshot id, resolved
	// excludes), so a rescan is instant until the next backup writes a newer
	// snapshot. One entry per container means the map is bounded by the container
	// count — no eviction policy, no unbounded growth.
	// suggestFlights is the singleflight for that cache: one in-flight snapshot
	// aggregate per key, so a refresh, a second browser tab or two viewers do not
	// each spawn their own `restic ls` against the same repo inside a
	// memory-capped container. Guarded by suggestMu as well.
	suggestMu      sync.Mutex
	suggestCache   map[string]suggestCacheEntry
	suggestFlights map[string]*suggestFlight

	// budgetMu guards offsiteOverBudget, the per-domain "off-site repo is over its
	// growth budget" latch. The alarm fires ONCE per false→true crossing (not on
	// every replication while over budget); the latch clears when growth drops
	// back under budget so a later breach re-alarms.
	budgetMu          sync.Mutex
	offsiteOverBudget map[string]bool

	// tamperMu serialises RunTamperTest per domain so the read-prev → record →
	// notify sequence is atomic: two concurrent tamper tests can't both observe the
	// old verdict and double-fire (or interleave and drop) the protection-loss
	// alert. It is distinct from repoMu — a tamper test touches no repo state, only
	// the tamper history — and is created lazily (tamperMuGuard) so it works
	// regardless of how the Service was constructed.
	tamperMuGuard sync.Mutex
	tamperMu      map[string]*sync.Mutex

	// foreignMu guards foreignSessions: short-lived, in-memory READ-ONLY
	// sessions onto ANOTHER BombVault instance's repository (Recovery →
	// "restore from another repo", #61). Deliberately NEVER persisted — closing
	// or expiring a session forgets the foreign location and key entirely, and
	// Settings stays untouched (see foreign.go). Created lazily so it works
	// regardless of how the Service was constructed.
	//
	// foreignJanitor is the stop channel of the background sweeper started on the
	// first OpenForeign (nil = not running). Closing it stops the goroutine; it is
	// re-created on the next open. The sweeper drops expired sessions — and their
	// foreign APP_KEY — WITHOUT waiting for another API call (#61). foreignSweepEvery
	// overrides the sweep interval in tests (0 = the production default).
	foreignMu         sync.Mutex
	foreignSessions   map[string]foreignSession
	foreignJanitor    chan struct{}
	foreignSweepEvery time.Duration

	// detectMu guards detectFlight, the single in-flight encryption-detection
	// pass (internal/api/encryption_detect.go). The Recovery page fires
	// POST /api/encryption/detect on mount, and one pass can take minutes when a
	// configured off-site host is dead — so a second tab, or a reload, joins the
	// pass already running and gets its answer instead of forking a second set of
	// restic probes against the same repositories. nil = no pass running. Zero
	// value is ready to use, so it works regardless of how the Service was
	// constructed.
	detectMu     sync.Mutex
	detectFlight *encryptionDetectFlight
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

// domainBusy reports the activity label of a domain whose repo lock is currently
// held, and whether it is held at all. It lets a backup starter refuse a busy
// domain up front instead of launching a goroutine that then blocks silently on
// the mutex (there is an inherent tiny race — a scheduler can grab the lock right
// after this check — that is acceptable UX; it shrinks the silent stall to a rare
// window).
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

// backupHardCap returns the maximum wall-clock time a single backup run may hold
// its domain lock before being force-cancelled — a guard so a wedged run cannot
// hold the lock forever. It is configurable via the BACKUP_MAX_HOURS env var:
//
//	unset/empty          -> 48h (generous default; very large or slow cloud
//	                        backups routinely need more than the original 12h,
//	                        which killed >1 TB runs at ~11h59m).
//	N (positive integer) -> N hours.
//	0                    -> no hard cap (returns 0; callers keep
//	                        context.WithoutCancel with no deadline — a wedged run
//	                        is still bounded by the scheduler overlap guard).
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

// drillLockWait is the most a SCHEDULED drill waits for the per-domain lock to
// free (matches the backup cap); drillLockPoll is how often it re-tries the lock
// while waiting. They are package-level vars (not consts) purely so tests can
// shrink them to sub-second values via a hook — production behaviour is fixed.
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

// SetPlatform wires the detected Platform adapter (platform.Detect + main's
// Kind->Platform mapping): the appdata-fallback convention, cross-instance
// restore-destination defaults, and the Unraid update-status reconcile step.
// Called from main after detection. Unset (nil) defaults to platform.Unraid{}
// via platformFn — bombvault's original, pre-Platform-seam behavior.
func (s *Service) SetPlatform(p platform.Platform) { s.platform = p }

// platformFn returns the configured Platform adapter, defaulting to
// platform.Unraid{} when unset (nil) — matching bombvault's historical
// Unraid-only behavior so every Service built without an explicit
// SetPlatform (including the many existing tests that construct a bare
// &Service{...}) keeps exercising exactly the literals it always has.
func (s *Service) platformFn() platform.Platform {
	if s.platform != nil {
		return s.platform
	}
	return platform.Unraid{}
}

// unraidGate reports whether an Unraid-only, best-effort host-SSH step (the
// webGUI notification mirror — every sendUnraidNotify call site) should run:
// the caller wants it (unraidWanted, always notify.Config.Unraid today), SSH
// is configured, AND the detected/overridden platform genuinely is Unraid.
//
// Why the platform check stays a hard requirement rather than trusting
// unraidWanted on its own (Platform expansion PR #149, Task 6 — see
// platform_gate_internal_test.go's failIfCalledSSH fakes, which fail the
// test the instant one of these steps is attempted on a non-Unraid
// platform): notify.Config.Unraid can be stale — e.g. a settings.json copied
// from an old Unraid box onto a genuinely different TrueNAS or
// generic-Docker host — and an Unraid-only command (the webGUI notify
// script, the #116 PHP-over-SSH reconcile, the `plugin` CLI) attempted
// against a host where it has no meaning would only fail predictably,
// adding noisy log spam on every run rather than accomplishing anything.
// Task 6's whole point was to skip that attempt entirely instead of letting
// it fail loudly every time; trusting the toggle blindly would reintroduce
// exactly that noise.
//
// Why a platform mismatch is NOT just silently treated as "feature off"
// either: internal/platform.Detect's only Unraid signal is one filesystem
// marker (config/plugins/dockerMan) under the container's /host/boot mount —
// a mount BombVault's own shipped template did not wire until months after
// this gate shipped. A genuinely Unraid host whose mount is missing (an old
// template, or a hand-edited container config) resolves to KindGeneric
// despite being real Unraid with SSH fully configured, and every gate here
// used to go dark with no way to tell "the user turned this off" apart from
// "detection is wrong". warnUnraidPlatformMismatch logs the specific,
// actionable diagnostic instead.
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

// warnUnraidPlatformMismatch logs, once per process (platformMismatchOnce),
// that notify.Config.Unraid is enabled but platform detection did not
// resolve to Unraid — naming the detected Kind and the most likely fix so an
// operator can tell "I turned this off" apart from "BombVault misdetected my
// host" without reading source. See unraidGate's doc comment for why the
// underlying gate does not just trust the toggle instead.
func (s *Service) warnUnraidPlatformMismatch() {
	s.platformMismatchOnce.Do(func() {
		log.Printf("platform: notify.Config.Unraid is enabled but BombVault detected platform=%q (not %q) — "+
			"Unraid-only host features (webGUI notifications, the update-status reconcile, the dashboard-tile "+
			"plugin) stay disabled. If this IS an Unraid host, verify the host's /boot is bind-mounted to "+
			"/host/boot inside the container (see the BombVault Unraid template) and restart the container — "+
			"detection looks for %s.",
			s.platformFn().Kind(), platform.KindUnraid, filepath.Join(s.cfg.FlashDir, "config/plugins/dockerMan"))
	})
}

// unraidPlatformMismatchError builds the user-facing refusal for a
// request-scoped, Unraid-only action (TestNotify's Unraid channel, the
// dashboard-tile plugin's install/remove — see runDashPluginCmd) attempted
// while platform detection did not resolve to Unraid. feature names the
// specific thing being refused (e.g. "the Unraid notification channel").
//
// Unlike unraidGate's best-effort background paths (which log
// warnUnraidPlatformMismatch instead — nobody is waiting on those), these
// are explicit, synchronous user actions: the caller must see the reason
// immediately in the response, including the same actionable /host/boot
// hint, not go dig through the container log for it.
//
// Returns a *platformMismatchErr (not a plain fmt.Errorf) so the message
// bypasses handlers.go's scrubError: that scrubber strips every absolute
// path from an error before it reaches the client (defense-in-depth against
// leaking a repo path or secret), which would otherwise reduce this whole
// hint to "the ... is only available on Unraid hosts (BombVault detected
// platform=\"generic\" — if this IS an Unraid host, verify the host's
// [path] is bind-mounted to [path] inside the container ...)" — useless.
// /boot and /host/boot are BombVault's own fixed, publicly-documented mount
// points (see the Unraid template), never a repo path or secret, so this is
// the same bypass errRestoreDestination/errRepoPathGuidance (handlers.go,
// repo_path.go) already use for the identical reason.
func (s *Service) unraidPlatformMismatchError(feature string) error {
	return &platformMismatchErr{msg: fmt.Sprintf(
		"%s is only available on Unraid hosts (BombVault detected platform=%q — "+
			"if this IS an Unraid host, verify the host's /boot is bind-mounted to /host/boot "+
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

// errZvolRebaseFailed is the scrubber-bypass sentinel a rebase-failure error
// from prepareRestoreVMForTarget's cross-instance zvol rebase loop satisfies
// via Is, matched by handlers.go's scrubError. The dataset/pool names ARE
// the message — RebaseZvolDatasetPool's own doc comment names them as the
// whole point of this error path — and a ZFS dataset name is "<pool>/<rest>",
// so it necessarily contains "/" characters handlers.go's absPathRe would
// otherwise mistake for a filesystem path and mangle (e.g. turning
// `dataset "tank/vm-disk1"` into `dataset "tank[path]"`, destroying exactly
// the information the message exists to convey). Same bypass pattern as
// errRestoreDestination/errUnraidPlatformMismatch, for the identical reason.
var errZvolRebaseFailed = errors.New("zvol dataset rebase failed")

// zvolRebaseErr carries a zvol-rebase failure's ready-to-show message (see
// errZvolRebaseFailed) while still satisfying
// errors.Is(err, errZvolRebaseFailed) for the scrubber bypass.
type zvolRebaseErr struct{ msg string }

func (e *zvolRebaseErr) Error() string { return e.msg }

func (e *zvolRebaseErr) Is(target error) bool { return target == errZvolRebaseFailed }

// progBegin marks a backup/restore/replicate as started for key/phase and
// returns a context carrying a restic sink that republishes each percentage,
// plus the StartedAt (Unix seconds) it stamped on every event it publishes.
// Percent updates are throttled to whole-percent steps to avoid flooding
// subscribers. When no progress store is wired it is still a real timestamp
// (just unpublished) — the returned ctx is unchanged, but the second return
// value is always valid so a caller (e.g. copyToOffsite) can rely on it
// without a nil-store special case.
//
// Every published event — including the eventual progEnd terminal one, which
// the caller must pass this SAME startedAt to — is stamped with it, so a
// client can render a live elapsed duration for the whole run (issue #159).
// Returning it (rather than each caller capturing its own time.Now().Unix())
// is deliberate: two independent captures of "now" for what is meant to be
// ONE instant can straddle a second boundary, which is exactly what made
// copyToOffsite's heartbeat briefly disagree with progBegin's own timestamp
// before this fix.
func (s *Service) progBegin(ctx context.Context, key, phase string) (context.Context, int64) {
	startedAt := time.Now().Unix()
	if s.progress == nil {
		return ctx, startedAt
	}
	s.progress.Publish(progress.Event{Key: key, Phase: phase, Percent: 0, Active: true, StartedAt: startedAt})
	last := -1.0
	return progress.WithSink(ctx, func(pct float64) {
		// A multi-path restore runs one restic process per path; each restarts at
		// ~0. A drop below the last value means a new process began — reset the
		// throttle so paths 2..N also report live progress.
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

// offsiteLastCopy holds the most recently PUBLISHED live restic-copy
// percentage for one "offsite:<domain>" replication (across however many
// sequential targets a multiTarget loop runs) — the one real signal
// copyToOffsite's heartbeat goroutine needs so its periodic "still alive"
// republish reflects the CURRENT state instead of a blank placeholder that
// stomps whatever progBeginCopySink's sink most recently reported.
//
// Two independent goroutines touch this: progBeginCopySink's sink callback
// runs synchronously on copyToOffsiteTarget's own goroutine (restic.Copy
// calls it inline as it scans stdout), while the heartbeat reads it from its
// own ticker goroutine — hence the mutex. A zero value is "no real update
// yet" (valid=false), which the heartbeat must tell apart from an actual
// Percent:0 update.
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
// call, so its live per-snapshot pack-copy progress (see restic.Copy — issue
// #159's real percentage, not the fabrication the first cut of this feature
// assumed was unavoidable) reaches the SAME "offsite:<domain>" key and
// StartedAt every other event for this replication uses. estimatedTotal is
// the caller's best-effort "N" for a "snapshot k of N" display (see
// restic.PendingCopyIDs's doc comment for why it is only ever a display
// estimate); if the live SnapshotIndex ever exceeds a REAL estimate — the
// estimate undercounted — the published total is widened to match rather than
// claiming fewer snapshots than are visibly running.
//
// estimatedTotal 0 means "the caller could not estimate at all" (both snapshot
// listings failed, or restic's own stricter dedup found work PendingCopyIDs
// did not) and is published UNCHANGED as SnapshotTotal 0 — the documented
// "unknown" on the wire (see progress.Event). It is deliberately NOT widened
// to SnapshotIndex: that produced a fabricated "snapshot 7 of 7" that a
// consumer cannot tell apart from a genuine final snapshot, and the run-level
// percentage the frontend now derives from k/N (issue #159's follow-up — see
// web/src/lib/progress.ts's offsiteRunProgress) would have read a confident
// ~99% for the whole run on nothing but that fabrication. An honest 0 makes
// the frontend fall back to its duration-only text instead. Percent updates are
// throttled like progBegin's plain Sink (whole-percent steps), but a
// SnapshotIndex change always forwards immediately so "k of N" advances
// without waiting on the new snapshot's first live percentage. No-op
// returning ctx unchanged when no progress store is wired.
//
// last, when non-nil, is updated with every value actually published here —
// copyToOffsite's heartbeat goroutine reads it back so a heartbeat tick
// republishes the real, current percentage instead of overwriting it with a
// blank one (see offsiteLastCopy's doc comment).
func (s *Service) progBeginCopySink(ctx context.Context, domain string, startedAt int64, estimatedTotal int, last *offsiteLastCopy) context.Context {
	if s.progress == nil {
		return ctx
	}
	key := "offsite:" + domain
	lastIndex := -1
	lastPct := -1.0
	return progress.WithCopySink(ctx, func(cp progress.CopyProgress) {
		if cp.SnapshotIndex == lastIndex && cp.Percent < 100 && cp.Percent-lastPct < 1 {
			return // throttle: only forward ≥1% steps within the SAME snapshot
		}
		lastIndex, lastPct = cp.SnapshotIndex, cp.Percent
		// Widen an UNDERCOUNTING estimate, never invent one from nothing:
		// estimatedTotal 0 stays 0 ("unknown") — see the doc comment above.
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
// failure (the UI hides the bar either way). startedAt must be the SAME value
// the matching progBegin returned, so the terminal event carries it too —
// before this fix it was omitted (zero), which made a client-rendered live
// elapsed duration visibly vanish during the ~0.8-2.5s the terminal event
// lingers in the frontend's progress map before the entry is dropped (see
// web/src/lib/progress.ts's COMPLETE_LINGER_MS / OffsiteIndicator's
// MIN_VISIBLE_MS). No-op without a progress store.
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

// ModeFor builds the restic Mode from the encryption setting. Encryption ON
// derives the password from APP_KEY; OFF uses a password-less repo.
func (s *Service) ModeFor(settings store.Settings) restic.Mode {
	// Decode the cloud creds once for BOTH the backend-credential env vars and the
	// off-site S3 storage class (they ride the same encrypted blob). Best-effort: a
	// decode failure logs and yields a zero CloudCreds, so the restic op fails
	// clearly on auth rather than panicking.
	c, err := s.decodeCloud(settings)
	if err != nil {
		log.Printf("api: cloud creds decode failed (ignoring): %v", err)
	}
	m := restic.Mode{Env: cloudEnv(c), StorageClass: c.S3StorageClass}
	if settings.EncryptionEnabled {
		m.Encrypted = true
		m.Password = restickey.Derive(s.cfg.AppKey)
	}
	return m
}

// resolveRepo turns a configured repo location into the value passed to restic
// -r. A restic remote backend (rclone:…, s3:…, sftp:… — off-site) is used
// verbatim; a local location is resolved as a relative subpath under the host
// mount root, rejecting traversal. A rejection comes back as operator-facing
// guidance naming the relative-path convention and the value to enter instead
// (see repoPathError) rather than the raw paths sentinel (issue #138).
func (s *Service) resolveRepo(loc string) (string, error) {
	if restic.IsRemoteRepo(loc) {
		return loc, nil
	}
	repo, err := paths.Resolve(s.cfg.HostMountRoot, loc)
	if err != nil {
		return "", s.repoPathError(loc, err)
	}
	return repo, nil
}

// containersRepoPath resolves the restic repo for the containers domain.
func (s *Service) containersRepoPath(settings store.Settings) (string, error) {
	return s.resolveRepo(settings.ContainersPath)
}

// vmsRepoPath resolves the restic repo for the vms domain.
func (s *Service) vmsRepoPath(settings store.Settings) (string, error) {
	return s.resolveRepo(settings.VMsPath)
}

// flashRepoPath resolves the restic repo for the flash domain.
func (s *Service) flashRepoPath(settings store.Settings) (string, error) {
	return s.resolveRepo(settings.FlashPath)
}

// configRepoPath resolves the restic repo for the config self-backup domain.
func (s *Service) configRepoPath(settings store.Settings) (string, error) {
	return s.resolveRepo(settings.ConfigPath)
}

// filesRepoPath resolves the restic repo for the files domain.
func (s *Service) filesRepoPath(settings store.Settings) (string, error) {
	return s.resolveRepo(settings.FilesPath)
}

// flashZipExportDir resolves the operator-configured output folder for the
// scheduled flash zip export. Unlike flashRepoPath (which, via resolveRepo, may
// hand a remote-backend string like "s3:…" straight to restic), this is always a
// plain LOCAL folder, so it applies only the containment half of resolveRepo:
// paths.Resolve(HostMountRoot, …), which rejects absolute paths and traversal.
func (s *Service) flashZipExportDir(settings store.Settings) (string, error) {
	dir, err := paths.Resolve(s.cfg.HostMountRoot, settings.FlashZipExportPath)
	if err != nil {
		return "", fmt.Errorf("flash zip export: resolve path: %w", err)
	}
	return dir, nil
}

// configSnapshotDir is the staging directory for the config self-backup — a
// consistent, restic-ready copy of BombVault's own /config state, rebuilt fresh
// before each config backup and removed afterwards. It lives under DataDir so it
// travels with the /config mount but is excluded from being its own live state.
func (s *Service) configSnapshotDir() string { return filepath.Join(s.cfg.DataDir, ".snapshot") }

// stageConfigSnapshot builds a consistent, restic-ready copy of BombVault's own
// /config state in a staging dir: a VACUUM-INTO snapshot of the live DB plus the
// rclone.conf and ssh/ keypair (copied as-is; they are static files). The live
// DB is never handed to restic directly (WAL mode can tear a raw file copy).
// Returns the staging dir; the caller removes it after the backup.
func (s *Service) stageConfigSnapshot() (string, error) {
	dir := s.configSnapshotDir()
	if err := os.RemoveAll(dir); err != nil {
		return "", fmt.Errorf("config snapshot: clear staging: %w", err)
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("config snapshot: mkdir staging: %w", err)
	}
	// A partial staging holds sensitive plaintext (the settings DB, rclone.conf
	// creds, the ssh private key). On ANY error path below we return before the
	// caller (BackupConfig) has registered its `defer os.RemoveAll(stagingDir)`, so
	// clean up ourselves unless we reach the end with ok=true.
	ok := false
	defer func() {
		if !ok {
			_ = os.RemoveAll(dir) // never leave a partial snapshot (DB + creds + ssh key) on disk
		}
	}()
	stagedDB := filepath.Join(dir, "bombvault.sqlite")
	if err := s.store.VacuumInto(stagedDB); err != nil {
		return "", err
	}
	// The SQLite driver creates the VACUUM'd DB at its default mode (~0o644); tighten
	// it to 0o600 so the staged settings DB is never group/other-readable, matching
	// the rclone.conf + ssh copies below. Defense-in-depth: the staging dir is already
	// 0o700, but the DB should not rely on the dir mode alone.
	if err := os.Chmod(stagedDB, 0o600); err != nil {
		return "", fmt.Errorf("config snapshot: chmod db: %w", err)
	}
	// rclone.conf + ssh/ are static on disk; copy verbatim if present.
	if src := filepath.Join(s.cfg.DataDir, "rclone.conf"); fileExists(src) {
		if err := copyFile(src, filepath.Join(dir, "rclone.conf"), 0o600); err != nil {
			return "", fmt.Errorf("config snapshot: copy rclone.conf: %w", err)
		}
	}
	if src := filepath.Join(s.cfg.DataDir, "ssh"); dirExists(src) {
		if err := copyTree(src, filepath.Join(dir, "ssh")); err != nil {
			return "", fmt.Errorf("config snapshot: copy ssh: %w", err)
		}
	}
	ok = true
	return dir, nil
}

// dirExists reports whether p exists and is a directory.
func dirExists(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.IsDir()
}

// copyFile copies src to dst with the given mode, truncating dst if it exists.
func copyFile(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src) //nolint:gosec // G304: src is an internal DataDir path (rclone.conf / ssh key), not user-supplied
	if err != nil {
		return err
	}
	// in is read-only; a close error is not actionable.
	defer func() { _ = in.Close() }()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode) //nolint:gosec // G304: dst is under our staging dir
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close() //nolint:errcheck,gosec // cleanup on error path; original error takes priority
		return err
	}
	return out.Close()
}

// copyTree recursively copies the directory src to dst, preserving file modes.
func copyTree(src, dst string) error {
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dst, 0o700); err != nil {
		return err
	}
	for _, e := range entries {
		sp := filepath.Join(src, e.Name())
		dp := filepath.Join(dst, e.Name())
		if e.IsDir() {
			if err := copyTree(sp, dp); err != nil {
				return err
			}
			continue
		}
		info, err := e.Info()
		if err != nil {
			return err
		}
		// Cap at 0o600: these are private keys/config; never widen perms on copy.
		mode := info.Mode().Perm()
		if mode > 0o600 {
			mode = 0o600
		}
		if err := copyFile(sp, dp, mode); err != nil {
			return err
		}
	}
	return nil
}

// toContainerPath translates a HOST path under HostSourceRoot to its
// container-visible equivalent under HostMountRoot (the broad Host Data mount,
// e.g. /mnt → /host/user). Returns ("", false) when the host path is not
// reachable through the mount. Used for appdata (containers) and VM disk paths;
// NVRAM is NOT translated here — it travels over SSH (see BackupVM/RestoreVM).
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

// retentionPolicy maps the stored settings to a restic keep-policy.
func (s *Service) retentionPolicy(settings store.Settings) restic.RetentionPolicy {
	return restic.RetentionPolicy{
		KeepLast:    settings.RetentionKeepLast,
		KeepDaily:   settings.RetentionKeepDaily,
		KeepWeekly:  settings.RetentionKeepWeekly,
		KeepMonthly: settings.RetentionKeepMonthly,
	}
}

// offsiteRetentionPolicy is the SEPARATE keep-policy for the off-site repo, so it
// can be kept longer (archive) than the local copy. All-zero (the default) means
// no off-site pruning — the off-site repo keeps everything, so existing setups
// are never silently trimmed and an off-site repo only gets pruned once the user
// explicitly sets this policy.
func (s *Service) offsiteRetentionPolicy(settings store.Settings) restic.RetentionPolicy {
	return restic.RetentionPolicy{
		KeepLast:    settings.OffsiteRetentionKeepLast,
		KeepDaily:   settings.OffsiteRetentionKeepDaily,
		KeepWeekly:  settings.OffsiteRetentionKeepWeekly,
		KeepMonthly: settings.OffsiteRetentionKeepMonthly,
	}
}

// targetOffsiteRetentionPolicy is the per-DESTINATION off-site keep-policy (the
// plural successor to offsiteRetentionPolicy, which reads the single global
// columns). All-zero means keep-everything, exactly as the global default. For a
// backfilled N=1 target these fields equal the global policy, so behavior is
// unchanged.
func targetOffsiteRetentionPolicy(t store.OffsiteTarget) restic.RetentionPolicy {
	return restic.RetentionPolicy{
		KeepLast:    t.RetentionKeepLast,
		KeepDaily:   t.RetentionKeepDaily,
		KeepWeekly:  t.RetentionKeepWeekly,
		KeepMonthly: t.RetentionKeepMonthly,
	}
}

// targetOffsiteLimits is the per-DESTINATION bandwidth cap (KiB/s). All-zero means
// unlimited. For a backfilled N=1 target these equal the global caps.
func targetOffsiteLimits(t store.OffsiteTarget) restic.Limits {
	return restic.Limits{
		UploadKBps:   t.LimitUpload,
		DownloadKBps: t.LimitDownload,
	}
}

// offsiteModeForTarget builds the restic mode for one off-site DESTINATION: it
// starts from the global mode (ModeFor already sets Env/StorageClass from the
// shared CloudCreds), then — stage 2 — resolves this target's OWN credential
// set via CredsRef and overrides Env (and StorageClass, unless the target sets
// its own) when it names one. A target with an empty CredsRef therefore keeps
// using the shared credentials exactly as before (every existing install is
// unaffected); a target with an empty StorageClass — e.g. the stage-1
// backfilled N=1 target, which the pure-SQL migration could not populate —
// PRESERVES whichever class its resolved credential set carries. The class is
// never unconditionally overwritten, which would wipe it to empty for existing
// single-off-site installs.
func (s *Service) offsiteModeForTarget(settings store.Settings, target store.OffsiteTarget) restic.Mode {
	mode := s.ModeFor(settings)
	if strings.TrimSpace(target.CredsRef) != "" {
		if c, err := s.decodeCloudFor(settings, target.CredsRef); err != nil {
			log.Printf("api: offsite target %s: cloud creds decode failed (ignoring, falling back to shared): %v", target.ID, err) //nolint:gosec // G706: target.ID is an opaque store-generated id
		} else {
			mode.Env = cloudEnv(c)
			if c.S3StorageClass != "" {
				mode.StorageClass = c.S3StorageClass
			}
		}
	}
	if target.StorageClass != "" {
		mode.StorageClass = target.StorageClass
	}
	return mode
}

// retentionPolicyForSource returns the keep-policy to apply for a given repo
// source: the off-site policy for any off-site source, the local policy
// otherwise. (The off-site policy is the settings-level one; per-target
// retention is a later stage — bare "offsite" is unchanged.)
func (s *Service) retentionPolicyForSource(settings store.Settings, source string) restic.RetentionPolicy {
	if isOffsiteSource(source) {
		return s.offsiteRetentionPolicy(settings)
	}
	return s.retentionPolicy(settings)
}

// applyRetention prunes the just-backed-up item to the configured keep-policy.
// tag is the item's identity tag (container:<name>, vm:<name>, fileset:<name>,
// flash, config): the policy is applied to that item's WHOLE history as one
// group, immune to path/host changes (issue #91 — the previous paths-grouped
// pass froze an item's old snapshots forever once its path set changed).
// Best-effort: a failure never fails the backup that just succeeded — but it is
// now NOTIFIED (not just logged), because silently skipped retention lets the
// repo grow unseen for weeks.
//
// During a bulk run (the #95 bulk-suppress flag on ctx: scheduled multi-item
// loops and the manual "back up all" batches) the expensive --prune is DEFERRED:
// each item's forget runs without prune and PruneAfterBulk reclaims the space
// ONCE after the whole loop — a 44-container night used to pay 44 full local
// prunes. Single/manual backups (and flash/config, which never set the flag)
// keep the immediate inline prune, byte-identical to before.
//
// domain identifies which domain repo belongs to, so this can check whether
// repo is a remote PRIMARY flagged append-only in its saved safety settings
// (issue #152, primaryIsImmutable) — when it is, retention is skipped
// entirely, exactly like copyToOffsiteTarget skips its off-site retention pass
// for an immutable off-site destination: an immutable primary has no separate
// off-site copy standing behind it, so this box's own credentials must not be
// able to prune its sole backup. A local primary, or a remote one with no
// saved safety settings (or saved but not flagged immutable), is unaffected.
func (s *Service) applyRetention(ctx context.Context, repo string, settings store.Settings, mode restic.Mode, tag, domain string) {
	p := s.retentionPolicy(settings)
	if !p.Any() {
		return
	}
	if s.primaryIsImmutable(domain, repo) {
		log.Printf("api: %s: retention skipped — primary repo is remote and flagged append-only", domain) //nolint:gosec // G706: domain is a fixed literal
		return
	}
	prune := !bulkReplicateSuppressed(ctx) // bulk run: one batched prune after the loop
	if err := s.forgetWithLockHeal(ctx, repo, p, mode, tag, prune); err != nil {
		log.Printf("api: retention prune failed (backup is safe): %v", err)
		s.notifyRetentionFailed(ctx, tag, truncateRunErr(err))
	}
}

// forgetWithLockHeal runs a ForgetPolicy pass, clearing a genuine stale orphan
// lock first with plain `restic unlock` (removeAll=false) — restic's own stale
// detection: a dead PID on THIS host, or any lock past restic's ~30-min age
// threshold. forget needs an EXCLUSIVE lock, so even a stale NON-exclusive lock —
// which lets backups keep succeeding — blocks every retention pass: one orphan
// used to fail a whole night's retentions across all items. A live lock is NOT
// force-removed (see the body); it carries the same bounded prior-incarnation
// hostname gap noted on CheckDomain.
func (s *Service) forgetWithLockHeal(ctx context.Context, repo string, p restic.RetentionPolicy, mode restic.Mode, tag string, prune bool) error {
	// Clear a genuine stale orphan (a dead-PID lock from a crashed run on this
	// host) before forget, which needs an exclusive lock. A live/concurrent lock
	// is NOT force-removed: reads run --no-lock, writes are serialized under the
	// domain lock, and forget itself passes --retry-lock to wait out a transient
	// cross-process lock (see the repo-lock-serialization plan). Force-removing a
	// live lock (the old #94 heal) could not fix a live holder and endangered a
	// running op, so it was removed.
	s.unlockStale(ctx, repo, mode)
	return s.engine.ForgetPolicy(ctx, repo, p, mode, tag, prune)
}

// identityTags returns the distinct item-identity tags present in snaps:
// container:<name>, vm:<name>, fileset:<name>, and the fixed flash/config tags.
// Profile/marker tags (p1, p2, live) are not identities.
func identityTags(snaps []restic.Snapshot) []string {
	seen := map[string]bool{}
	var out []string
	for _, sn := range snaps {
		for _, t := range sn.Tags {
			isIdentity := t == "flash" || t == "config" ||
				strings.HasPrefix(t, "container:") || strings.HasPrefix(t, "vm:") || strings.HasPrefix(t, "fileset:")
			if isIdentity && !seen[t] {
				seen[t] = true
				out = append(out, t)
			}
		}
	}
	return out
}

// applyRetentionPerIdentity applies policy per identity tag — one ungrouped,
// tag-scoped forget per item — then prunes once. Used where no single item is
// in scope (manual prune, off-site retention). Falls back to the legacy
// repo-wide paths-grouped pass when the snapshot listing fails or yields no
// identity tags, so retention never silently does nothing.
func (s *Service) applyRetentionPerIdentity(ctx context.Context, repo string, p restic.RetentionPolicy, mode restic.Mode) error {
	if !p.Any() {
		return nil
	}
	snaps, err := s.engine.Snapshots(ctx, repo, mode)
	tags := identityTags(snaps)
	if err != nil || len(tags) == 0 {
		return s.forgetWithLockHeal(ctx, repo, p, mode, "", true)
	}
	var errs []error
	for _, tag := range tags {
		if fErr := s.forgetWithLockHeal(ctx, repo, p, mode, tag, false); fErr != nil {
			errs = append(errs, fmt.Errorf("%s: %w", tag, fErr))
		}
	}
	if pErr := s.engine.Prune(ctx, repo, mode); pErr != nil {
		errs = append(errs, pErr)
	}
	return errors.Join(errs...)
}

// notifyRetentionFailed sends a best-effort alert when the post-backup
// retention prune fails. Mirrors notifyReplicationFailed's policy gate; a no-op
// when notifications are off.
func (s *Service) notifyRetentionFailed(ctx context.Context, tag, detail string) {
	c, err := s.NotifyConfig()
	if err != nil || c.On == "" || c.On == "never" {
		return
	}
	subject := "Retention prune FAILED for " + tag
	msg := fmt.Sprintf("Applying the retention policy for %s failed — old snapshots are not being pruned (the new backup itself is safe): %s", tag, detail)
	notify.Send(ctx, c, tag, notify.Event{Title: "BombVault", Message: subject + " — " + msg, OK: false})
	if s.unraidGate(c.Unraid) {
		if e := s.sendUnraidNotify(ctx, "BombVault: "+subject, msg, "warning"); e != nil {
			log.Printf("notify: unraid: %v", e)
		}
	}
}

// offsiteTargetsFor returns a domain's ENABLED off-site destinations from the
// store, in stable per-domain order (sort_order, then created_at). It is the
// plural successor to the single-repo Settings.*Offsite* columns: a backfilled
// single-off-site install yields a one-element slice, an unconfigured domain an
// empty one. A missing store (used by pure-settings unit tests) or a query error
// falls back to empty so callers apply their Settings fallback.
func (s *Service) offsiteTargetsFor(domain string) []store.OffsiteTarget {
	if s.store == nil {
		return nil
	}
	targets, err := s.store.OffsiteTargetsForDomain(domain)
	if err != nil {
		log.Printf("api: offsite %s: list targets failed (falling back to settings): %v", domain, err) //nolint:gosec // G706: domain is a fixed literal
		return nil
	}
	var out []store.OffsiteTarget
	for _, t := range targets {
		if t.Enabled {
			out = append(out, t)
		}
	}
	return out
}

// offsiteReplicationTargets is the destination set copyToOffsite replicates a
// domain to: the domain's enabled off-site targets, or — when no target row
// exists (a fresh install configured only through the legacy Settings columns, or
// one configured after the stage-1 backfill ran) — a single target synthesized
// from those Settings columns. This keeps N=1 byte-identical whether or not the
// row was backfilled.
func (s *Service) offsiteReplicationTargets(domain string, settings store.Settings) []store.OffsiteTarget {
	if ts := s.offsiteTargetsFor(domain); len(ts) > 0 {
		return ts
	}
	loc := offsiteRepoFromSettings(domain, settings)
	if loc == "" {
		return nil
	}
	return []store.OffsiteTarget{settingsOffsiteTarget(domain, settings, loc)}
}

// settingsOffsiteTarget synthesizes the one off-site target the stage-1 backfill
// would have produced from the legacy Settings.*Offsite* columns, so the target
// loop reproduces today's single-destination behavior for an un-backfilled
// install. StorageClass stays "" on purpose: the global S3 class lives in
// CloudCreds and is supplied by ModeFor, and the "" here preserves it (see the
// per-target mode in copyToOffsiteTarget).
func settingsOffsiteTarget(domain string, settings store.Settings, loc string) store.OffsiteTarget {
	return store.OffsiteTarget{
		Domain:               domain,
		Name:                 "Primary",
		Repo:                 loc,
		StorageClass:         "",
		Immutable:            offsiteImmutableFor(domain, settings),
		Schedule:             offsiteScheduleFromSettings(domain, settings),
		RetentionKeepLast:    settings.OffsiteRetentionKeepLast,
		RetentionKeepDaily:   settings.OffsiteRetentionKeepDaily,
		RetentionKeepWeekly:  settings.OffsiteRetentionKeepWeekly,
		RetentionKeepMonthly: settings.OffsiteRetentionKeepMonthly,
		LimitUpload:          settings.OffsiteLimitUpload,
		LimitDownload:        settings.OffsiteLimitDownload,
		GrowthBudgetGB:       settings.OffsiteGrowthBudgetGB,
		Enabled:              true,
	}
}

// aggregateTamper folds a domain's off-site tamper verdicts worst-of across its
// destinations for the ransomware scorecard: had is true only when EVERY
// destination has a recorded verdict, protected only when EVERY one refused the
// delete, and at is the OLDEST verdict timestamp (so the least-recently-checked
// destination drives the currency/overdue judgement). For a single-destination
// (N=1) domain — the only shape possible before stage 5 — it is EXACTLY
// LatestTamperTest(domain), byte-identical to the pre-aggregation read.
func (s *Service) aggregateTamper(domain string) (had, protected bool, at int64) {
	targets := s.offsiteTargetsFor(domain)
	if len(targets) <= 1 {
		tt, found, err := s.store.LatestTamperTest(domain)
		if err != nil || !found {
			return false, false, 0
		}
		return true, tt.Protected, tt.At
	}
	protected = true
	for _, t := range targets {
		tt, found, err := s.store.LatestTamperTestForTarget(domain, t.ID)
		if err != nil || !found {
			return false, false, 0 // an untested destination → no protected claim
		}
		had = true
		if !tt.Protected {
			protected = false
		}
		if at == 0 || tt.At < at {
			at = tt.At
		}
	}
	return had, protected, at
}

// aggregateReplicationCurrency folds a domain's last-successful-replication
// currency worst-of across its off-site destinations: ok only when EVERY
// destination has landed a successful copy, and at is the OLDEST of those (the
// least-recently-replicated destination sets the domain's freshness). For a
// single-destination (N=1) domain it is EXACTLY LatestSuccessfulOffsiteRun(domain),
// byte-identical to the pre-aggregation read.
func (s *Service) aggregateReplicationCurrency(domain string) (at int64, ok bool) {
	targets := s.offsiteTargetsFor(domain)
	if len(targets) <= 1 {
		run, found, err := s.store.LatestSuccessfulOffsiteRun(domain)
		if err != nil || !found {
			return 0, false
		}
		return run.StartedAt, true
	}
	for _, t := range targets {
		run, found, err := s.store.LatestSuccessfulOffsiteRunForTarget(domain, t.ID)
		if err != nil || !found {
			return 0, false // a never-replicated destination → domain is not current
		}
		if at == 0 || run.StartedAt < at {
			at = run.StartedAt
		}
	}
	return at, true
}

// offsiteRepoFor returns the configured off-site repo location for a domain, or
// "" when none is set. It is a thin wrapper over the first enabled off-site
// target, falling back to the legacy Settings column when no target row exists so
// nothing regresses (N=1: the backfilled target's Repo equals that column).
func (s *Service) offsiteRepoFor(domain string, settings store.Settings) string {
	if ts := s.offsiteTargetsFor(domain); len(ts) > 0 {
		return ts[0].Repo
	}
	return offsiteRepoFromSettings(domain, settings)
}

// offsiteRepoFromSettings reads the legacy single-repo off-site location straight
// off the Settings columns (the fallback source for offsiteRepoFor).
func offsiteRepoFromSettings(domain string, settings store.Settings) string {
	switch domain {
	case "containers":
		return settings.ContainersOffsite
	case "vms":
		return settings.VMsOffsite
	case "flash":
		return settings.FlashOffsite
	case "config":
		return settings.ConfigOffsite
	case "files":
		return settings.FilesOffsite
	}
	return ""
}

// offsiteScheduleFor returns the per-domain off-site replication schedule. Empty
// means "replicate after every local backup"; a non-empty cadence means
// replication is driven by the scheduler instead (decoupled from backups).
//
// The cadence is read from the per-domain Settings column and NOTHING else. That
// column is the single source of truth: Settings › Schedules edits it, and the
// scheduler registers each "<domain>-offsite" cron entry from it (see the offsite()
// block in internal/schedule/schedule.go). Reading the cadence from anywhere else
// lets the two disagree, and one direction of that disagreement loses the off-site
// copy outright (issue #150): when some other source says "decoupled" while the Settings
// column is blank, the coupled after-backup copy stands down AND no cron entry was
// ever registered to replace it, so the domain silently stops replicating — no
// error, no log line, no run row, for as long as it takes someone to notice.
//
// Concretely: an off-site TARGET ROW can carry a schedule value (the CRUD API
// accepts the field, and a settings import restores whatever was exported), and
// this used to be preferred over the Settings column. Per-target schedules are
// deliberately not a feature — every target of a domain replicates on that domain's
// off-site schedule (see OffsiteTargetsSection, which exposes no such control) — so
// a stray row value is ignored here rather than allowed to silence a domain.
func (s *Service) offsiteScheduleFor(domain string, settings store.Settings) string {
	return offsiteScheduleFromSettings(domain, settings)
}

// offsiteScheduleFromSettings reads the legacy per-domain off-site schedule
// straight off the Settings columns (the fallback source for offsiteScheduleFor).
func offsiteScheduleFromSettings(domain string, settings store.Settings) string {
	switch domain {
	case "containers":
		return settings.ContainersOffsiteSchedule
	case "vms":
		return settings.VMsOffsiteSchedule
	case "flash":
		return settings.FlashOffsiteSchedule
	case "config":
		return settings.ConfigOffsiteSchedule
	case "files":
		return settings.FilesOffsiteSchedule
	}
	return ""
}

// offsiteImmutableFor reports whether a domain's off-site repo is flagged
// append-only (immutable). The far side (e.g. rest-server --append-only)
// enforces it; the flag changes BombVault's OWN behaviour: replication skips
// the off-site retention prune, and off-site delete/prune are refused. Unlock
// stays allowed — rest-server permits lock removal in append-only mode, and
// clearing a stale lock is operationally required.
func offsiteImmutableFor(domain string, s store.Settings) bool {
	switch domain {
	case "containers":
		return s.ContainersOffsiteImmutable
	case "vms":
		return s.VMsOffsiteImmutable
	case "flash":
		return s.FlashOffsiteImmutable
	case "config":
		return s.ConfigOffsiteImmutable
	case "files":
		return s.FilesOffsiteImmutable
	}
	return false
}

// errOffsiteAppendOnly refuses a destructive operation against an off-site repo
// flagged immutable: the whole point of append-only is that credentials on this
// box cannot delete history, so BombVault does not even try.
var errOffsiteAppendOnly = errors.New("repo is append-only; prune far-side or use a maintenance window")

// DomainStatusEntry is the per-domain RPO (protection) status: whether a
// domain's backups are current relative to its schedule. It drives the
// dashboard's green/amber/red "are my backups current?" indicator.
type DomainStatusEntry struct {
	Domain   string `json:"domain"`   // "containers" | "vms" | "flash" | "config" | "files"
	Enabled  bool   `json:"enabled"`  // domain switched on in Settings
	Schedule string `json:"schedule"` // the domain's OWN cadence string (e.g. "daily 02:30")
	// CoveredBy carries the "Backup Everything" cadence when that pass is the
	// ONLY thing backing this domain up — its own schedule is off, but the pass
	// includes it. Empty whenever the domain has a schedule of its own, so a
	// client can render "the domain's cadence" and "covered by the whole-server
	// pass" as the different statements they are. Without it the status read
	// "Not scheduled" for a domain backed up nightly (#177).
	CoveredBy     string `json:"coveredBy"`
	LastSuccess   int64  `json:"lastSuccess"`   // unix time of the last successful backup, 0 = none
	PeriodSeconds int64  `json:"periodSeconds"` // expected RPO window in seconds, 0 = no expectation
	Status        string `json:"status"`        // "off" | "never" | "overdue" | "warn" | "ok"
	// LastVerified is the unix time of the last LOCAL restore-verification drill
	// (`restic check --read-data-subset`), 0 = never verified. LastVerifiedOK is
	// its outcome. These drive the dashboard's "last verified restorable" badge
	// without an extra round-trip.
	LastVerified   int64 `json:"lastVerified"`
	LastVerifiedOK bool  `json:"lastVerifiedOK"`
	// VerifiedDetail / DrillDetail carry the scrubbed failure reason of the last
	// LOCAL subset drill and the last OFF-SITE DR drill so the dashboard can show
	// WHY + WHICH check failed (#30). Both are "" on success (Detail is empty then),
	// so carrying them unconditionally is safe.
	VerifiedDetail string `json:"verifiedDetail"`
	DrillDetail    string `json:"drillDetail"`

	// Ransomware-protection scorecard facts (Task 8): whether the domain has an
	// off-site copy, whether it is flagged append-only (immutable), and the
	// age-stamped outcomes of the three protection checks — the active tamper
	// test, the off-site replication, and the off-site DR drill. Protection is the
	// red/amber/green aggregate (see protectionLevel); it is "" for a disabled
	// domain (the dashboard then shows nothing for it). These extend /api/status so
	// the dashboard card needs no second round-trip.
	OffsiteConfigured bool  `json:"offsiteConfigured"`
	OffsiteImmutable  bool  `json:"offsiteImmutable"`
	LastTamperAt      int64 `json:"lastTamperAt"`
	LastTamperOK      bool  `json:"lastTamperOK"`
	LastReplicationAt int64 `json:"lastReplicationAt"`
	LastReplicationOK bool  `json:"lastReplicationOK"`
	LastDRDrillAt     int64 `json:"lastDrDrillAt"`
	LastDRDrillOK     bool  `json:"lastDrDrillOK"`
	// LastOffsiteSubsetAt / LastOffsiteSubsetOK stamp the latest OFF-SITE SUBSET
	// drill (`restic check --read-data-subset` against the off-site repo) — the
	// cheaper integrity check every domain (including VMs, since v8.0.0) can run
	// alongside the real DR sandbox-restore drill. They drive the dashboard's
	// "off-site verified" badge (#63), independent of the DR fields above.
	LastOffsiteSubsetAt int64 `json:"lastOffsiteSubsetAt"`
	LastOffsiteSubsetOK bool  `json:"lastOffsiteSubsetOK"`
	// OffsiteDrillScheduled is true only when the scheduler actually runs an
	// off-site DR drill for this domain (DrillsEnabled AND OffsiteDrillsEnabled AND
	// an off-site repo configured). When false but the domain has an off-site repo,
	// the dashboard shows a muted "manual only" pill instead of a red drFailed (#37).
	OffsiteDrillScheduled bool   `json:"offsiteDrillScheduled"`
	Protection            string `json:"protection"` // "" (disabled) | "red" | "amber" | "green"

	// Per-check states derived from the SAME inputs Protection aggregates (see
	// protectionChecks), so the dashboard card can render each checklist row as a
	// pure function of the backend and never contradict the chip. EncryptionOn and
	// PruneStrategySet are the two config-level facts the card also renders, moved
	// server-side so the card needs no separate /api/settings round-trip.
	TamperState      string `json:"tamperState"`      // "" | "never" | "failed" | "stale" | "ok"
	ReplicationState string `json:"replicationState"` // "" | "never" | "overdue" | "ok"
	DrillState       string `json:"drillState"`       // "" | "never" | "failed" | "overdue" | "ok"
	EncryptionOn     bool   `json:"encryptionOn"`     // repo encryption is enabled
	PruneStrategySet bool   `json:"pruneStrategySet"` // an off-site retention strategy is configured
}

// rpoStatus is the pure status decision from the inputs, so it can be unit-tested
// exhaustively without a store. scheduled is true when the domain is enabled AND
// has an RPO expectation (periodSeconds > 0):
//
//   - "off"     scheduled is false (disabled / no schedule / unparseable period)
//   - "never"   scheduled but no successful backup yet (lastSuccess == 0)
//   - "overdue" age > period*2
//   - "warn"    age > period   (and <= period*2)
//   - "ok"      otherwise
func rpoStatus(nowUnix, lastSuccess, periodSeconds int64, scheduled bool) string {
	if !scheduled || periodSeconds <= 0 {
		return "off"
	}
	if lastSuccess <= 0 {
		return "never"
	}
	age := nowUnix - lastSuccess
	switch {
	case age > periodSeconds*2:
		return "overdue"
	case age > periodSeconds:
		return "warn"
	default:
		return "ok"
	}
}

// cadencePeriodSeconds parses a cadence string to its expected period in seconds,
// returning 0 for an empty or unparseable cadence (i.e. "no expectation").
func cadencePeriodSeconds(cadence string) int64 {
	if strings.TrimSpace(cadence) == "" {
		return 0
	}
	cad, err := schedule.ParseCadence(cadence)
	if err != nil {
		return 0
	}
	return cad.PeriodSeconds()
}

// domainCoverage answers "how often does this domain actually get backed up, and
// by what" for ONE domain, given its own cadence and the "Backup Everything"
// cadence. It returns the RPO window in seconds and, when the pass is the only
// thing covering the domain, that pass's cadence string.
//
// Backup Everything runs the domains as a sixth, independent pseudo-domain, so a
// user can (and #177's reporter does) leave every per-domain schedule off and let
// the pass do the work. Reading the per-domain cadence alone then answers zero,
// which the protection status renders as "Not scheduled" for a domain that is in
// fact backed up nightly, and which makes the overdue watchdog fall silent for
// that domain entirely — the far more expensive half, since its whole job is to
// notice when backups stop.
//
// When both are scheduled the window is the SHORTER of the two: whichever fires
// more often is what bounds how stale a backup can get. The pass only touches
// domains that are switched on, so a switched-off domain gets no coverage from
// it either — the caller's own enabled check still governs.
func domainCoverage(ownCadence, everythingCadence string) (period int64, coveredBy string) {
	own := cadencePeriodSeconds(ownCadence)
	every := cadencePeriodSeconds(everythingCadence)
	switch {
	case own == 0 && every == 0:
		return 0, ""
	case own == 0:
		return every, strings.TrimSpace(everythingCadence)
	case every == 0 || own <= every:
		return own, ""
	default:
		return every, ""
	}
}

// protInputs carries the facts protectionLevel aggregates, so the decision is a
// pure function of its inputs (unit-testable without a store) and mirrors
// rpoStatus's shape.
type protInputs struct {
	enabled           bool
	offsiteConfigured bool
	offsiteImmutable  bool
	hadTamper         bool
	lastTamperOK      bool
	lastTamperAt      int64
	tamperPeriod      int64 // seconds; 0 = no/invalid tamper schedule
	lastReplicationAt int64 // last SUCCESSFUL replication (currency source)
	offsitePeriod     int64 // seconds; 0 = replication coupled to each backup (no own schedule)
	lastBackupAt      int64 // last SUCCESSFUL backup (coupled-replication currency basis)
	backupPeriod      int64 // seconds; the domain's backup RPO period (coupled-grace basis)
	lastDRDrillAt     int64
	lastDRDrillOK     bool  // outcome of the latest DR drill (only meaningful when lastDRDrillAt != 0)
	drillPeriod       int64 // seconds; 0 = no drill schedule
}

// replicationState decides the off-site replication currency (""/never/overdue/ok)
// from the SAME inputs protectionLevel and protectionChecks share, so the chip and
// the checklist row can never disagree.
//
//   - No off-site configured → "" (there is no replication to be current; the
//     missing-off-site case is handled as red by protectionLevel).
//   - Decoupled (offsitePeriod>0): the standard rpoStatus against the off-site's
//     own schedule, using the last SUCCESSFUL replication.
//   - Coupled (offsitePeriod==0, the default): replication rides each backup, so
//     the claim is "the last successful backup has a corresponding successful
//     off-site copy". It goes overdue only once the gap between the last backup and
//     the last successful replication exceeds a grace of 2× the backup period
//     (conservative: a backup replicating shortly after is fine; a never-replicated
//     backup is flagged only once it has sat unreplicated beyond the grace). Amber,
//     never red.
func replicationState(now int64, in protInputs) string {
	if !in.offsiteConfigured {
		return ""
	}
	if in.offsitePeriod > 0 {
		switch rpoStatus(now, in.lastReplicationAt, in.offsitePeriod, true) {
		case "overdue":
			return "overdue"
		case "never":
			return "never"
		default:
			return "ok"
		}
	}
	// Coupled path: only meaningful once a backup exists and there is an RPO basis.
	if in.lastBackupAt == 0 || in.backupPeriod <= 0 {
		return ""
	}
	grace := in.backupPeriod * 2
	if in.lastReplicationAt == 0 {
		// Never replicated: overdue only once the backup has sat unreplicated > grace
		// (a just-made first backup replicating shortly after must not instantly flag).
		if now-in.lastBackupAt > grace {
			return "overdue"
		}
		return "ok"
	}
	if in.lastReplicationAt < in.lastBackupAt && in.lastBackupAt-in.lastReplicationAt > grace {
		return "overdue"
	}
	return "ok"
}

// protectionLevel aggregates a domain's ransomware-protection posture into a
// red/amber/green chip. The far side enforces immutability, so this NEVER goes
// green on configuration claims alone:
//
//   - ""    the domain is disabled — the dashboard shows nothing for it.
//   - red   the domain is enabled but has no off-site copy at all; OR the off-site
//     is flagged immutable yet the append-only guarantee is unproven — the
//     tamper test is missing, last failed, or is stale (older than 2× its
//     schedule period). A non-immutable off-site makes no append-only claim,
//     so a missing tamper test does not make it red.
//   - amber protection exists but a scheduled time-check is overdue by the SAME
//     period-doubling rule backups use (rpoStatus "overdue"): the off-site
//     replication (only when a decoupled off-site schedule is set) or the
//     off-site DR drill (only when a drill schedule is set); OR the latest
//     scheduled DR drill FAILED — a failed restorability proof is a real posture
//     concern, so the chip can't read green over the red "failed" drill row, but
//     it stays amber (not full red) because other protections may still be fine.
//   - green otherwise.
func protectionLevel(now int64, in protInputs) string {
	if !in.enabled {
		return "" // disabled domains carry no protection posture
	}
	if !in.offsiteConfigured {
		return "red" // enabled but no off-site copy — unprotected by design
	}
	if in.offsiteImmutable {
		tamperStale := in.tamperPeriod > 0 && now-in.lastTamperAt > in.tamperPeriod*2
		if !in.hadTamper || !in.lastTamperOK || tamperStale {
			return "red" // an append-only claim we cannot currently PROVE
		}
	}
	// Replication currency: overdue is amber. Decoupled off-sites use their own
	// schedule; coupled (default) off-sites are checked against the last backup with
	// a conservative grace (see replicationState) so off-site health is no longer
	// invisible in the config most users run.
	if replicationState(now, in) == "overdue" {
		return "amber"
	}
	// A recorded DR drill that FAILED downgrades the chip to amber (never green over
	// a red row). This mirrors protectionChecks' "failed" branch EXACTLY (same guard:
	// a drill schedule is set AND the latest recorded drill failed), so the chip and
	// the scorecard row can never disagree on a failed drill.
	if in.drillPeriod > 0 && in.lastDRDrillAt != 0 && !in.lastDRDrillOK {
		return "amber"
	}
	if rpoStatus(now, in.lastDRDrillAt, in.drillPeriod, in.drillPeriod > 0) == "overdue" {
		return "amber"
	}
	return "green"
}

// protChecks is the per-check state the ransomware scorecard renders. Tamper and
// Replication are derived from the SAME protInputs protectionLevel aggregates, so
// those rows can never contradict the chip. Drill additionally honors the latest
// DR drill's OUTCOME (a failed drill reads "failed"/red) to agree with the
// dedicated off-site "proven restorable" pill; protectionLevel downgrades that
// same failed-drill case to amber, so a red "failed" Drill row coincides with an
// (at least) amber chip — never a green one. An empty state ("") means the check
// makes no claim (and so is rendered muted, not as a failure).
type protChecks struct {
	Tamper      string // "" | "never" | "failed" | "stale" | "ok"
	Replication string // "" | "never" | "overdue" | "ok"
	Drill       string // "" | "never" | "failed" | "overdue" | "ok"
}

// protectionChecks mirrors protectionLevel for Tamper and Replication, and layers
// the DR-drill OUTCOME on top of currency for Drill:
//
//   - Tamper ∈ {never,failed,stale} is precisely the immutable branch that turns
//     the chip red; a non-immutable off-site makes no append-only claim → "".
//   - Replication "overdue" is precisely the amber branch (rpoStatus "overdue",
//     the same period-doubling rule). "never"/"ok" stay non-amber so they match a
//     green chip.
//   - Drill mirrors that currency (never/overdue/ok) for a PASSED drill, but a
//     recorded DR drill that FAILED reads "failed" (red) regardless of recency, so
//     the row agrees with the off-site "proven restorable" pill (lastDRDrillOK).
//     protectionLevel downgrades this same failed-drill case to amber, so the red
//     row coincides with an (at least) amber chip — never a green one.
//
// Replication still does NOT surface a red "replication failed": nothing else
// consumes lastReplicationOK, so only its currency is mirrored.
func protectionChecks(now int64, in protInputs) protChecks {
	var c protChecks

	// Tamper (append-only) — only an immutable off-site makes an append-only claim.
	switch {
	case !in.offsiteImmutable:
		c.Tamper = ""
	case !in.hadTamper:
		c.Tamper = "never"
	case !in.lastTamperOK:
		c.Tamper = "failed"
	case in.tamperPeriod > 0 && now-in.lastTamperAt > in.tamperPeriod*2:
		c.Tamper = "stale"
	default:
		c.Tamper = "ok"
	}

	// Replication currency — decoupled off-sites use their own schedule; coupled
	// (default) off-sites are checked against the last backup with a grace (see
	// replicationState). "" when there is nothing to claim yet.
	c.Replication = replicationState(now, in)

	// DR drill outcome + currency — only when a drill schedule is set. A recorded
	// DR drill that FAILED reads "failed" (a red row) regardless of recency, so the
	// row can't go green-by-currency while the off-site "proven restorable" pill
	// (lastDRDrillOK) reads red. "never" stays for no drill yet; a PASSED drill keeps
	// the overdue/ok currency logic.
	if in.drillPeriod > 0 {
		switch {
		case in.lastDRDrillAt != 0 && !in.lastDRDrillOK:
			c.Drill = "failed"
		default:
			switch rpoStatus(now, in.lastDRDrillAt, in.drillPeriod, true) {
			case "overdue":
				c.Drill = "overdue"
			case "never":
				c.Drill = "never"
			default:
				c.Drill = "ok"
			}
		}
	}

	return c
}

// DomainStatus returns the RPO (protection) status of each domain (containers,
// vms, flash, config, files): whether its backups are current relative to its
// schedule. The enabled flag + cadence come from Settings; the last successful
// backup time comes from the store's per-domain helpers.
func (s *Service) DomainStatus() ([]DomainStatusEntry, error) {
	settings, err := s.store.GetSettings()
	if err != nil {
		return nil, fmt.Errorf("read settings: %w", err)
	}
	now := time.Now().Unix()

	domains := []struct {
		name     string
		enabled  bool
		schedule string
		lastFn   func() (time.Time, error)
	}{
		{"containers", settings.ContainersEnabled, settings.ContainersSchedule, s.store.LastSuccessfulContainerBackup},
		{"vms", settings.VMsEnabled, settings.VMsSchedule, s.store.LastSuccessfulVMBackup},
		{"flash", settings.FlashEnabled, settings.FlashSchedule, s.store.LastSuccessfulFlashBackup},
		{"config", settings.ConfigEnabled, settings.ConfigSchedule, s.store.LastSuccessfulConfigBackup},
		{"files", settings.FilesEnabled, settings.FilesSchedule, s.store.LastSuccessfulFilesBackup},
	}

	out := make([]DomainStatusEntry, 0, len(domains))
	for _, d := range domains {
		last, lErr := d.lastFn()
		if lErr != nil {
			return nil, fmt.Errorf("domain %s last-success: %w", d.name, lErr)
		}
		var lastUnix int64
		if !last.IsZero() {
			lastUnix = last.Unix()
		}

		// A period is only meaningful for an enabled domain that something
		// actually backs up on a cadence — its own, or the "Backup Everything"
		// pass (see domainCoverage). An unparseable cadence (defensive — the
		// settings PUT validates) collapses to period 0 → "off".
		period, coveredBy := domainCoverage(d.schedule, settings.EverythingSchedule)
		scheduled := d.enabled && period > 0

		// The latest LOCAL restore-verification drill drives the "last verified
		// restorable" badge. Best-effort: a read error leaves the badge at "never"
		// (0 / false) rather than failing the whole status query.
		var lastVerified int64
		var lastVerifiedOK bool
		var verifiedDetail string
		if drill, found, dErr := s.store.LatestRestoreDrill(d.name, "local"); dErr == nil && found {
			lastVerified = drill.At
			lastVerifiedOK = drill.OK
			verifiedDetail = drill.Detail
		}

		// Ransomware-protection scorecard facts. All reads are best-effort: a store
		// error leaves the relevant fact at its zero value (a missing check), which
		// the aggregate then treats conservatively rather than failing the query.
		offsiteConfigured := s.offsiteRepoFor(d.name, settings) != ""
		offsiteImmutable := offsiteImmutableFor(d.name, settings)

		// Tamper facts are aggregated worst-of across the domain's off-site
		// destinations (protected only when EVERY destination is, currency = the
		// oldest). For a single-destination (N=1) domain this is exactly
		// LatestTamperTest(domain) — byte-identical.
		hadTamper, lastTamperOK, lastTamperAt := s.aggregateTamper(d.name)
		// Currency uses the last SUCCESSFUL replication (mirrors backups' last-SUCCESS):
		// a perpetually-failing replication then reads as stale → overdue → amber,
		// rather than staying fresh off a failed attempt's timestamp. Aggregated
		// worst-of (the OLDEST successful copy across destinations); byte-identical to
		// LatestSuccessfulOffsiteRun(domain) for N=1.
		lastReplicationAt, lastReplicationOK := s.aggregateReplicationCurrency(d.name)
		var lastDRDrillAt int64
		var lastDRDrillOK bool
		var drDetail string
		if dr, found, drErr := s.store.LatestRestoreDrillKind(d.name, "offsite", "dr"); drErr == nil && found {
			lastDRDrillAt = dr.At
			lastDRDrillOK = dr.OK
			drDetail = dr.Detail
		}
		// The latest OFF-SITE SUBSET drill (integrity check against the off-site
		// repo) drives the dashboard's "off-site verified" badge (#63). It is the
		// only off-site drill available for VMs (DR restores are refused for them),
		// so it is read for every domain. Best-effort like the reads above.
		var lastOffsiteSubsetAt int64
		var lastOffsiteSubsetOK bool
		if sub, found, subErr := s.store.LatestRestoreDrillKind(d.name, "offsite", "subset"); subErr == nil && found {
			lastOffsiteSubsetAt = sub.At
			lastOffsiteSubsetOK = sub.OK
		}

		// The DR-drill currency only has a claim when the scheduler actually runs
		// off-site DR drills (DrillsEnabled AND OffsiteDrillsEnabled); otherwise a
		// stale lastDRDrillAt must not read overdue. Opting out of the scheduled
		// off-site DR drill (#37) thus reuses the proven global-drills-off neutral
		// path: drillPeriod stays 0, so DrillState is "" (muted) and protectionLevel
		// ignores DR (both its DR branches are drillPeriod>0-guarded).
		var drillPeriod int64
		if settings.DrillsEnabled && settings.OffsiteDrillsEnabled {
			drillPeriod = cadencePeriodSeconds(settings.DrillsSchedule)
		}

		in := protInputs{
			enabled:           d.enabled,
			offsiteConfigured: offsiteConfigured,
			offsiteImmutable:  offsiteImmutable,
			hadTamper:         hadTamper,
			lastTamperOK:      lastTamperOK,
			lastTamperAt:      lastTamperAt,
			tamperPeriod:      cadencePeriodSeconds(settings.TamperTestSchedule),
			lastReplicationAt: lastReplicationAt,
			offsitePeriod:     cadencePeriodSeconds(s.offsiteScheduleFor(d.name, settings)),
			lastBackupAt:      lastUnix,
			backupPeriod:      period,
			lastDRDrillAt:     lastDRDrillAt,
			lastDRDrillOK:     lastDRDrillOK,
			drillPeriod:       drillPeriod,
		}
		// protection (the chip) and checks (each row) are derived from the SAME
		// protInputs. Tamper/Replication rows mirror the chip's red/amber branches
		// exactly. The Drill row additionally honors the latest drill's OUTCOME (a
		// failed drill reads a red "failed"); to keep the chip from reading green over
		// that red row, protectionLevel downgrades a failed drill to amber under the
		// same guard — so no row can contradict the chip.
		protection := protectionLevel(now, in)
		checks := protectionChecks(now, in)

		// An off-site retention strategy is "configured" when the far side prunes
		// (immutable), a growth budget is set, or an off-site keep policy is set.
		pruneStrategySet := offsiteImmutable ||
			settings.OffsiteGrowthBudgetGB > 0 ||
			settings.OffsiteRetentionKeepLast > 0 ||
			settings.OffsiteRetentionKeepDaily > 0 ||
			settings.OffsiteRetentionKeepWeekly > 0 ||
			settings.OffsiteRetentionKeepMonthly > 0

		out = append(out, DomainStatusEntry{
			Domain:                d.name,
			Enabled:               d.enabled,
			Schedule:              d.schedule,
			CoveredBy:             coveredBy,
			LastSuccess:           lastUnix,
			PeriodSeconds:         period,
			Status:                rpoStatus(now, lastUnix, period, scheduled),
			LastVerified:          lastVerified,
			LastVerifiedOK:        lastVerifiedOK,
			VerifiedDetail:        verifiedDetail,
			OffsiteConfigured:     offsiteConfigured,
			OffsiteImmutable:      offsiteImmutable,
			LastTamperAt:          lastTamperAt,
			LastTamperOK:          lastTamperOK,
			LastReplicationAt:     lastReplicationAt,
			LastReplicationOK:     lastReplicationOK,
			LastDRDrillAt:         lastDRDrillAt,
			LastDRDrillOK:         lastDRDrillOK,
			LastOffsiteSubsetAt:   lastOffsiteSubsetAt,
			LastOffsiteSubsetOK:   lastOffsiteSubsetOK,
			OffsiteDrillScheduled: settings.DrillsEnabled && settings.OffsiteDrillsEnabled && offsiteConfigured,
			DrillDetail:           drDetail,
			Protection:            protection,
			TamperState:           checks.Tamper,
			ReplicationState:      checks.Replication,
			DrillState:            checks.Drill,
			EncryptionOn:          settings.EncryptionEnabled,
			PruneStrategySet:      pruneStrategySet,
		})
	}
	return out, nil
}

// DayStat is the per-domain backup outcome count for a single calendar day.
type DayStat struct {
	OK     int `json:"ok"`
	Failed int `json:"failed"`
}

// HistoryDay is one calendar day's backup outcomes split by domain, for the
// dashboard's GitHub-contributions-style backup-health heatmap.
type HistoryDay struct {
	Date       string  `json:"date"` // local YYYY-MM-DD
	Containers DayStat `json:"containers"`
	VMs        DayStat `json:"vms"`
	Flash      DayStat `json:"flash"`
	Config     DayStat `json:"config"`
	Files      DayStat `json:"files"`
}

// runDomains is the target_id → domain map ("container" | "vm" | "flash" |
// "config" | "files") used to attribute each run to its domain. It mirrors the
// same mapping handleRuns uses: container targets, VM targets, file sets, and
// the singleton flash/config ids. Best-effort — an unknown id (e.g. a deleted
// target) maps to "" and is ignored by the bucketer.
func (s *Service) runDomains() map[string]string {
	domain := map[string]string{store.FlashTargetID: "flash", store.ConfigTargetID: "config"}
	if cts, err := s.store.ListTargets(); err == nil {
		for _, t := range cts {
			domain[t.ID] = "container"
		}
	}
	if vts, err := s.store.ListVMTargets(); err == nil {
		for _, t := range vts {
			domain[t.ID] = "vm"
		}
	}
	if fss, err := s.store.ListFileSets(); err == nil {
		for _, fs := range fss {
			domain[fs.ID] = "files"
		}
	}
	return domain
}

// bucketRunsByDay is the pure heatmap-bucketing core: it produces one HistoryDay
// for EVERY local calendar day in [startUnix, endUnix] (ascending), tallying
// each backup run's success/failed outcome into its domain via the target_id →
// domain map. Days with no runs come back with zeros so the frontend gets a
// contiguous grid. Non-backup kinds and "running" runs are ignored, as are runs
// whose target maps to no known domain. Kept free of the store/clock so it can
// be unit-tested directly.
func bucketRunsByDay(runs []store.Run, domain map[string]string, startUnix, endUnix int64) []HistoryDay {
	// Map each local day to its index in the output grid. Indices stay valid even
	// as the slice grows (unlike pointers into a slice that append may reallocate).
	idx := map[string]int{}
	start := time.Unix(startUnix, 0).Local()
	end := time.Unix(endUnix, 0).Local()
	startDay := time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, start.Location())
	endDay := time.Date(end.Year(), end.Month(), end.Day(), 0, 0, 0, 0, end.Location())

	out := make([]HistoryDay, 0)
	for d := startDay; !d.After(endDay); d = d.AddDate(0, 0, 1) {
		date := d.Format("2006-01-02")
		idx[date] = len(out)
		out = append(out, HistoryDay{Date: date})
	}

	for _, run := range runs {
		if run.Kind != "backup" {
			continue
		}
		dom := domain[run.TargetID]
		if dom == "" {
			continue // unknown / deleted target
		}
		date := time.Unix(run.StartedAt, 0).Local().Format("2006-01-02")
		i, ok := idx[date]
		if !ok {
			continue // outside the window (defensive — query already bounds it)
		}
		var stat *DayStat
		switch dom {
		case "container":
			stat = &out[i].Containers
		case "vm":
			stat = &out[i].VMs
		case "flash":
			stat = &out[i].Flash
		case "config":
			stat = &out[i].Config
		case "files":
			stat = &out[i].Files
		default:
			continue
		}
		switch run.Status {
		case "success":
			stat.OK++
		case "failed":
			stat.Failed++
		}
	}
	return out
}

// BackupHistory returns one HistoryDay per calendar day in the last `days` days
// (ascending, including empty days with zeros) for the dashboard heatmap. days
// is capped at 366. Runs are bucketed by local calendar day and by domain.
func (s *Service) BackupHistory(days int) ([]HistoryDay, error) {
	if days < 1 {
		days = 1
	}
	if days > 366 {
		days = 366
	}
	now := time.Now()
	since := now.AddDate(0, 0, -(days - 1))
	// Widen the store query to the start of the earliest day so a run early on the
	// first day isn't missed by an intra-day cutoff; the bucketer bounds the grid.
	startUnix := time.Date(since.Year(), since.Month(), since.Day(), 0, 0, 0, 0, since.Location()).Unix()
	runs, err := s.store.RunsSince(startUnix)
	if err != nil {
		return nil, fmt.Errorf("read runs: %w", err)
	}
	return bucketRunsByDay(runs, s.runDomains(), startUnix, now.Unix()), nil
}

// repoStatsMinInterval is the minimum age of the latest sample before a backup
// re-collects repo stats. Stats (two restic stats passes over the whole repo)
// are expensive, so once a day is plenty for a size/dedup trend — a domain
// backed up many times an hour samples only once.
const repoStatsMinInterval = 20 * time.Hour

// CollectStats samples a domain's repository size for source ("local"/"offsite")
// and records it for the size/dedup trend. It is best-effort and idempotent: a
// missing or empty (zero-snapshot) repo records nothing and returns nil, so it
// never turns an otherwise-good backup into a failure. Any restic error IS
// returned so the (throttled) caller can log it.
func (s *Service) CollectStats(ctx context.Context, domain, source string) error {
	settings, repo, err := s.domainRepoSource(domain, source)
	if err != nil {
		return err
	}
	// No repo yet (local not initialised) → nothing to measure, not an error.
	if localRepoMissing(repo) {
		return nil
	}
	mode := s.ModeFor(settings)
	snaps, err := s.engine.Snapshots(ctx, repo, mode)
	if err != nil {
		return err
	}
	if len(snaps) == 0 {
		return nil // empty repo — nothing to measure
	}
	raw, err := s.engine.Stats(ctx, repo, "raw-data", mode)
	if err != nil {
		return err
	}
	restoreSize, err := s.engine.Stats(ctx, repo, "restore-size", mode)
	if err != nil {
		return err
	}
	return s.store.AddRepoStat(store.RepoStat{
		Domain:      domain,
		Source:      source,
		At:          time.Now().Unix(),
		RawSize:     raw.TotalSize,
		RestoreSize: restoreSize.TotalSize,
		Snapshots:   int64(len(snaps)),
	})
}

// RepoStats returns the recorded repo-size samples for a domain + source
// (ascending by time), a thin passthrough to the store.
func (s *Service) RepoStats(domain, source string, limit int) ([]store.RepoStat, error) {
	return s.store.ListRepoStats(domain, source, limit)
}

// maybeCollectStats samples a domain's LOCAL repo size after a successful backup,
// throttled to repoStatsMinInterval so frequent backups don't re-scan the repo
// each time. It NEVER blocks or fails the backup: the work runs in a detached
// goroutine (request values kept, cancellation dropped, with its own timeout)
// and any error is only logged. Call this on each domain's success path.
func (s *Service) maybeCollectStats(ctx context.Context, domain string) {
	if latest, found, err := s.store.LatestRepoStat(domain, "local"); err != nil {
		log.Printf("api: stats: %s: could not read latest sample (skipping): %v", domain, err) //nolint:gosec // G706: domain is a fixed literal
		return
	} else if found && time.Since(time.Unix(latest.At, 0)) < repoStatsMinInterval {
		return // sampled recently enough
	}
	// Detach from the request (keep its values) so the sampling survives the
	// handler returning, with a hard cap so a wedged restic can't leak a goroutine.
	bg := context.WithoutCancel(ctx)
	go func() {
		cctx, cancel := context.WithTimeout(bg, 5*time.Minute)
		defer cancel()
		if err := s.CollectStats(cctx, domain, "local"); err != nil {
			log.Printf("api: stats: %s: collect failed (backup is safe): %v", domain, err) //nolint:gosec // G706: domain is a fixed literal
		}
	}()
}

// CollectStatsAsync samples a domain+source repo size in the background (detached,
// throttled to repoStatsMinInterval). Used to populate the Storage card for repos
// that already have backups but no sample yet (e.g. on upgrade, or before the next
// scheduled backup). Best-effort; errors are only logged. domain/source are always
// from a fixed whitelist (handler-validated or literal).
func (s *Service) CollectStatsAsync(domain, source string) {
	source = collectStatsSource(source)
	if latest, found, err := s.store.LatestRepoStat(domain, source); err == nil && found &&
		time.Since(time.Unix(latest.At, 0)) < repoStatsMinInterval {
		return // sampled recently enough
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		if err := s.CollectStats(ctx, domain, source); err != nil {
			log.Printf("api: stats: %s/%s: async collect failed: %v", domain, source, err) //nolint:gosec // G706: domain/source are fixed-whitelist values
		}
	}()
}

// collectStatsSource normalises a stats source: any off-site source — bare
// "offsite" (primary target) OR the per-target "offsite:<id>" form — samples the
// off-site repo and passes through unchanged; everything else collapses to
// "local". Using isOffsiteSource (not a literal "offsite" compare) lets a
// per-target source thread through to repoFor's off-site resolution instead of
// being clobbered to "local" (stage-3 carry-over).
func collectStatsSource(source string) string {
	if isOffsiteSource(source) {
		return source
	}
	return "local"
}

// CollectStatsOnStartup samples each enabled domain's LOCAL repo shortly after boot
// so the Storage card shows data for repos that already have backups, instead of
// "no data" until the next backup runs. Best-effort + throttled.
func (s *Service) CollectStatsOnStartup() {
	settings, err := s.store.GetSettings()
	if err != nil {
		return
	}
	for _, d := range []struct {
		name    string
		enabled bool
	}{
		{"containers", settings.ContainersEnabled},
		{"vms", settings.VMsEnabled},
		{"flash", settings.FlashEnabled},
		{"config", settings.ConfigEnabled},
		{"files", settings.FilesEnabled},
	} {
		if d.enabled {
			s.CollectStatsAsync(d.name, "local")
		}
	}
}

// copyToOffsite replicates a domain's local repo to its off-site DESTINATIONS with
// `restic copy` (the local repo stays primary). It resolves the domain's off-site
// targets (one per domain for a single-off-site install) and replicates to each in
// turn. It creates each off-site repo on first use and copies everything not
// already there (restic skips dupes, so the first run seeds history and later runs
// ship just the new snapshot). Returns the (scrubbed) error so on-demand/scheduled
// callers can surface it; it never logs an off-site location, which can embed
// credentials. Lock-free — the caller holds the domain lock.
//
// The passed-in mode is superseded by a per-target mode built inside the loop (so
// each destination carries its own S3 storage class); it is kept in the signature
// only to keep call-sites compiling.
//
// offsiteProgressHeartbeat is how often copyToOffsite re-publishes the
// indeterminate "replicating" progress event while a copy is in flight (#134).
// A package var (not a const) so tests can shrink it.
var offsiteProgressHeartbeat = 5 * time.Second

func (s *Service) copyToOffsite(ctx context.Context, domain string, settings store.Settings, _ restic.Mode, localRepo string) (err error) {
	targets := s.offsiteReplicationTargets(domain, settings)
	if len(targets) == 0 {
		return errors.New("no off-site repo configured for this domain")
	}
	// Additive kind="offsite" row in the SHARED runs table (StartRun/FinishRun on
	// the reserved domain target id, like prune/verify) so the replication shows
	// up in the dashboard Activity Log/Run History as persisted history. This stays
	// per-DOMAIN (one row per domain per call) — per-target activity/progress is a
	// later stage, and with a single target it is identical to before. Best-effort.
	activityRunID, aErr := s.store.StartRun(domainRunTargetID(domain), "offsite")
	if aErr != nil {
		log.Printf("api: offsite %s: could not start activity run (continuing): %v", domain, aErr) //nolint:gosec // G706: domain is a fixed literal
		activityRunID = ""
	}
	// ok is set true ONLY when every target succeeded, so an unwinding panic
	// (named-return err still nil) can't stamp a phantom successful run — the
	// deferred finish then records a failure, not a false success.
	var ok bool
	defer func() {
		if activityRunID == "" {
			return
		}
		status := "failed"
		if ok {
			status = "success"
		}
		if fErr := s.store.FinishRun(activityRunID, status, "", 0, truncateRunErr(err)); fErr != nil {
			log.Printf("api: offsite %s: could not finish activity run: %v", domain, fErr) //nolint:gosec // G706: domain is a fixed literal
		}
	}()
	// Publish an active "off-site replication running" indicator for this domain so
	// the UI shows WHICH domain is replicating, alongside a REAL live percentage
	// once one becomes available (issue #159 — see restic.Copy's and
	// progBeginCopySink's doc comments for the whole story: restic copy DOES print
	// genuine, parseable progress once RESTIC_PROGRESS_FPS is wired up the same way
	// backup/restore already get it; a first cut of this feature concluded
	// otherwise and shipped a duration-only readout instead, which is corrected
	// here). Kept per-DOMAIN so the OffsiteIndicator + dashboard stay unchanged for
	// N=1; a target's own live percentage is threaded in from
	// copyToOffsiteTarget's progBeginCopySink call, keyed to this SAME
	// "offsite:"+domain event so multiple sequential targets (multiTarget) don't
	// need their own indicator.
	//
	// startedAt is progBegin's own single time.Now().Unix() capture, returned so
	// this function's heartbeat below publishes the EXACT same instant rather than
	// a second, independent capture that could straddle a second boundary.
	_, startedAt := s.progBegin(ctx, "offsite:"+domain, "replicate")
	defer func() { s.progEnd("offsite:"+domain, "replicate", err == nil, startedAt) }()
	// #134: between a snapshot's own live percentage updates (and especially
	// during the tree-walk restic does before it starts copying packs, which
	// prints no percentage at all), lastSeen would otherwise only ever be touched
	// at progBegin. The frontend's STALE_MS (15s, web/src/lib/progress.ts) then
	// hides the "running" dashboard line as soon as a quiet stretch passes that —
	// which a real off-site copy easily can — making the operation look like it
	// silently vanished, even though it is still actively running (confirmed by
	// the line reappearing correctly on a manual refresh, since the backend state
	// was fine all along). Re-publish an active event periodically so lastSeen
	// keeps advancing through any such gap.
	//
	// lastCopy carries the most recently PUBLISHED real per-snapshot percentage
	// (see offsiteLastCopy's doc comment): a heartbeat tick republishes THAT
	// (percent/snapshotIndex/snapshotTotal all included) rather than a blank
	// Percent:0 placeholder. Before this, every 5s tick unconditionally
	// overwrote whatever real percentage progBeginCopySink's sink had just
	// reported — on a slow transfer (whole-percent steps taking many seconds
	// each), the heartbeat "won" almost every render, making the new live
	// percentage feature (issue #159) nearly invisible on exactly the slow
	// connections it was built for. When no real update has landed yet (before
	// the first "copy started" line, or a genuinely quiet stretch with no
	// per-snapshot signal at all) lastCopy stays unset and the heartbeat falls
	// back to the original bare "still alive" event — the duration-only
	// keep-alive #134 introduced this heartbeat for is unchanged.
	//
	// Shutdown is a real done-channel HANDSHAKE, not a bare close: closing hbDone
	// only asks the goroutine to stop, but a `select` with both hbDone and the
	// ticker ready at once can still pick the ticker and Publish one more tick —
	// waiting on hbStopped blocks until that goroutine has actually returned
	// (any in-flight Publish included), so it can never land AFTER progEnd's
	// terminal Publish below and resurrect a stale Active:true (pre-existing
	// since #134's heartbeat was introduced; tightened as part of this review).
	// Stopped via defer registered AFTER progEnd's (so it unwinds FIRST) so a
	// heartbeat can never race past the terminal event.
	lastCopy := &offsiteLastCopy{}
	if s.progress != nil {
		hbDone := make(chan struct{})
		hbStopped := make(chan struct{})
		defer func() {
			close(hbDone)
			<-hbStopped
		}()
		go func() {
			defer close(hbStopped)
			t := time.NewTicker(offsiteProgressHeartbeat)
			defer t.Stop()
			for {
				select {
				case <-hbDone:
					return
				case <-t.C:
					e := progress.Event{Key: "offsite:" + domain, Phase: "replicate", Active: true, StartedAt: startedAt}
					if cp, total, ok := lastCopy.get(); ok {
						e.Percent, e.SnapshotIndex, e.SnapshotTotal = cp.Percent, cp.SnapshotIndex, total
					}
					s.progress.Publish(e)
				}
			}
		}()
	}

	// Replicate to each destination best-effort: one target's failure is recorded
	// and logged but must NOT abort the others (moot for N=1). The joined error
	// surfaces to on-demand/scheduled callers so a failure still reports/notifies.
	// multiTarget is false for a single-destination (N=1) domain: the budget then
	// stays on the byte-identical DOMAIN path (source "offsite", the global
	// OffsiteGrowthBudgetGB, latch keyed by domain). Only with 2+ destinations does
	// each carry its OWN growth budget + per-target size sample + latch — dormant
	// until the stage-5 UI can add a second destination.
	multiTarget := len(targets) > 1
	var errs []error
	for _, t := range targets {
		if cerr := s.copyToOffsiteTarget(ctx, domain, settings, t, localRepo, multiTarget, startedAt, lastCopy); cerr != nil {
			log.Printf("api: offsite %s: copy to a destination failed (continuing): %v", domain, cerr) //nolint:gosec // G706: domain is a fixed literal
			errs = append(errs, cerr)
		}
	}
	if len(errs) > 0 {
		err = errors.Join(errs...)
		return err
	}
	ok = true
	return nil
}

// copyToOffsiteTarget replicates a domain's local repo to a SINGLE off-site
// destination and records that destination's own offsite_runs row (stamped with
// offsite_target_id). Per-target: destination repo, restic mode (S3 storage
// class), retention, bandwidth limits and append-only flag. Best-effort
// bookkeeping like the rest; returns the (scrubbed) copy error. startedAt is
// copyToOffsite's single progBegin capture, threaded through so this
// target's live copy-progress events (see progBeginCopySink) carry the SAME
// StartedAt as the domain-level begin/heartbeat/terminal events. lastCopy is
// copyToOffsite's shared heartbeat state (see offsiteLastCopy) that
// progBeginCopySink updates with every real percentage this target reports;
// a nil lastCopy (as a direct unit test of this function may pass) simply
// means no heartbeat is watching, which offsiteLastCopy's nil-safe methods
// handle without a special case here.
func (s *Service) copyToOffsiteTarget(ctx context.Context, domain string, settings store.Settings, target store.OffsiteTarget, localRepo string, multiTarget bool, startedAt int64, lastCopy *offsiteLastCopy) (err error) {
	// Persist this destination's replication attempt to the off-site run history
	// (begin now, close on the way out via defer with outcome + scrubbed error).
	// The offsite_runs row itself stays duration + outcome only (no percentage
	// column) — the live per-snapshot percentage this function now also feeds
	// into progBeginCopySink (issue #159) is a real-time SSE signal, not
	// persisted history; a completed run's own duration is exactly as
	// informative after the fact. Bookkeeping is best-effort: a store error is
	// logged, never fatal. offsite_target_id attributes the run to this
	// destination (empty for a settings-synthesized N=1 target, exactly as
	// before the backfill).
	runID, recErr := s.store.RecordOffsiteRunForTarget(domain, target.ID, time.Now().Unix())
	if recErr != nil {
		log.Printf("api: offsite %s: could not record replication run (continuing): %v", domain, recErr) //nolint:gosec // G706: domain is a fixed literal
		runID = 0
	}
	var ok bool
	defer func() {
		if runID == 0 {
			return
		}
		if ferr := s.store.FinishOffsiteRun(runID, ok, truncateRunErr(err)); ferr != nil {
			log.Printf("api: offsite %s: could not finish replication run: %v", domain, ferr) //nolint:gosec // G706: domain is a fixed literal
		}
	}()
	dest, rerr := s.resolveRepo(target.Repo)
	if rerr != nil {
		return fmt.Errorf("resolve off-site repo: %w", rerr)
	}
	// Relax the DESTINATION repo tree the same way every local backup already
	// relaxes the primary repo (makeRepoReadable). restic, run as root, writes a
	// local repo 0700/0400, so an off-site copy that lands on a mounted share is
	// root-only on the FAR side too: the share's other clients see the repo
	// folders but nothing inside them. That is exactly what an NFS→SMB switch
	// exposes — an Unassigned-Devices NFS mount reads the share as uid 0 and sails
	// through the 0700 dirs, while a CIFS mount authenticates as an ordinary SMB
	// user whose access the far side denies (issue #138 follow-up). Deferred, so a
	// copy that fails half-way, or a retention prune that writes fresh index/pack
	// files after it, is covered too; makeOffsiteRepoReadable skips remote
	// destinations and a path that is not there yet.
	defer makeOffsiteRepoReadable(dest)
	// Per-target restic mode carrying this destination's S3 storage class (see
	// offsiteModeForTarget: the global class is preserved for a backfilled N=1
	// target whose class is "").
	mode := s.offsiteModeForTarget(settings, target)
	if err = s.EnsureRepo(ctx, dest, mode); err != nil {
		return fmt.Errorf("ensure off-site repo: %w", err)
	}
	// Clear any stale lock a previously interrupted off-site op (replication copy /
	// integrity check) left on the destination repo, so restic copy can take its
	// lock instead of failing with "repository is already locked". BombVault is the
	// sole writer, so an existing off-site lock is always stale — this self-heals the
	// off-site repo on the next run (defence-in-depth for bug #29).
	s.unlockStale(ctx, dest, mode)
	// Best-effort upfront candidate count ("N") for the "snapshot k of N" live
	// progress display (issue #159 — see restic.PendingCopyIDs's doc comment for
	// the full reasoning). DISPLAY ONLY: the actual Copy call below still passes
	// nil for snapshotIDs, so restic's own (stricter — it also compares full
	// snapshot metadata) dedup remains the sole authority on what really gets
	// copied. A stale/wrong estimate here can only make "of N" briefly off, never
	// skip a real snapshot. listSnapshots (not a raw engine.Snapshots call) reuses
	// the existing stale-lock self-heal and "repo not initialized yet = no
	// snapshots" handling every other snapshot listing in this file already gets.
	pendingTotal := 0
	if srcSnaps, sErr := s.listSnapshots(ctx, localRepo, mode); sErr != nil {
		log.Printf("api: offsite %s: could not estimate pending snapshot count (continuing without it): %v", domain, sErr) //nolint:gosec // G706: domain is a fixed literal
	} else if dstSnaps, dErr := s.listSnapshots(ctx, dest, mode); dErr != nil {
		log.Printf("api: offsite %s: could not estimate pending snapshot count (continuing without it): %v", domain, dErr) //nolint:gosec // G706: domain is a fixed literal
	} else {
		pendingTotal = len(restic.PendingCopyIDs(srcSnaps, dstSnaps))
	}
	// Cap the transfer rate so off-site replication doesn't saturate the WAN
	// (zero limits = unlimited, the default). progBeginCopySink installs the
	// live per-snapshot percentage sink restic.Copy now reports through (issue
	// #159's real percentage — see its doc comment), publishing under the SAME
	// "offsite:"+domain key/StartedAt copyToOffsite's begin/heartbeat/terminal
	// events use, so it's one continuous indicator across a multiTarget loop.
	copyCtx := s.progBeginCopySink(ctx, domain, startedAt, pendingTotal, lastCopy)
	if err = s.engine.Copy(copyCtx, dest, localRepo, nil, targetOffsiteLimits(target), mode); err != nil {
		return err
	}
	// Apply the off-site retention policy (separate from local) after a successful
	// copy — only when one is set, so an off-site repo defaults to keep-everything
	// (archive) and existing setups are unchanged. Best-effort: a prune failure
	// must not fail the replication that already succeeded. An IMMUTABLE
	// (append-only) off-site repo is never pruned from here: the far side would
	// refuse the delete anyway, and retention is enforced far-side by design.
	if target.Immutable {
		log.Printf("api: offsite %s: retention is enforced far-side (append-only)", domain) //nolint:gosec // G706: domain is a fixed literal
	} else if op := targetOffsiteRetentionPolicy(target); op.Any() {
		// Per-identity: one tag-scoped, ungrouped forget per item, one prune —
		// identity-stable like the local retention (issue #91).
		if perr := s.applyRetentionPerIdentity(ctx, dest, op, mode); perr != nil {
			log.Printf("api: offsite %s: retention prune failed (replica is safe): %v", domain, perr) //nolint:gosec // G706: domain is a fixed literal
		}
	}
	// Sample the off-site repo size into the repo_stats time series and evaluate the
	// growth budget. When a budget is set we sample SYNCHRONOUSLY first so the check
	// sees THIS replication's fresh size — including the very first replication,
	// which has no prior sample — rather than a stale one; for an immutable repo
	// (no far-side prune) the budget is the only growth backstop, so it must not lag
	// or miss the seed. Without a budget we sample in the background (throttled) just
	// for the Storage card. The REST protocol can't see the far side's free space —
	// only BombVault's own growth — so the budget is a detection aid, not a hard cap.
	//
	// N=1 (single destination): the budget stays on the DOMAIN path — the size is
	// sampled under source "offsite", the global OffsiteGrowthBudgetGB drives it, and
	// the latch is keyed by domain. This is byte-identical to before. Only with 2+
	// destinations does each carry its OWN budget: the size is sampled under this
	// destination's per-target source ("offsite:<id>"), the threshold is this
	// target's GrowthBudgetGB, and the latch is keyed by (domain,targetID).
	if !multiTarget {
		if settings.OffsiteGrowthBudgetGB > 0 {
			if serr := s.CollectStats(ctx, domain, "offsite"); serr != nil {
				log.Printf("api: offsite %s: budget size sample failed (replica is safe): %v", domain, serr) //nolint:gosec // G706: domain is a fixed literal
			}
		} else {
			s.CollectStatsAsync(domain, "offsite")
		}
		s.checkOffsiteBudget(ctx, domain, settings)
		ok = true
		return nil
	}
	statSource := offsiteStatSource(target.ID)
	if target.GrowthBudgetGB > 0 {
		if serr := s.CollectStats(ctx, domain, statSource); serr != nil {
			log.Printf("api: offsite %s: budget size sample failed (replica is safe): %v", domain, serr) //nolint:gosec // G706: domain is a fixed literal
		}
	} else {
		s.CollectStatsAsync(domain, statSource)
	}
	s.checkOffsiteBudgetForTarget(ctx, domain, target)
	ok = true
	return nil
}

// offsiteStatSource maps an off-site destination id to the repo_stats source
// string used to sample THAT destination's size: bare "offsite" for a
// settings-synthesized target (empty id) so an un-backfilled N=1 install keeps
// sampling under "offsite", or "offsite:<id>" for a real per-target destination.
func offsiteStatSource(targetID string) string {
	if targetID == "" {
		return "offsite"
	}
	return offsiteSourcePrefix + targetID
}

// offsiteBudgetLatchKey keys the over-budget latch per (domain,targetID) so each
// destination alarms independently. The NUL separator keeps it unambiguous.
func offsiteBudgetLatchKey(domain, targetID string) string {
	return domain + "\x00" + targetID
}

// checkOffsiteBudgetForTarget is checkOffsiteBudget for ONE off-site destination:
// it compares that destination's latest sampled size (repo_stats source
// "offsite:<id>") against the target's OWN GrowthBudgetGB and fires a
// notification ONCE on each false→true crossing, latched per (domain,targetID).
// Used only for multi-destination domains; a single-destination domain stays on
// the byte-identical checkOffsiteBudget path.
func (s *Service) checkOffsiteBudgetForTarget(ctx context.Context, domain string, target store.OffsiteTarget) {
	s.checkGrowthBudget(ctx, domain, offsiteStatSource(target.ID), offsiteBudgetLatchKey(domain, target.ID), target.GrowthBudgetGB, "off-site")
}

// checkOffsiteBudget compares the latest sampled off-site repo size for a domain
// against the configured growth budget (OffsiteGrowthBudgetGB, 0 = off) and fires
// a notification ONCE on each false→true crossing. The latch (offsiteOverBudget)
// clears when growth drops back under budget so a later breach re-alarms. It reads
// the newest repo_stats row for domain+source="offsite"; if none exists yet (the
// async sample hasn't landed on the very first replication) it simply skips.
func (s *Service) checkOffsiteBudget(ctx context.Context, domain string, settings store.Settings) {
	s.checkGrowthBudget(ctx, domain, "offsite", domain, settings.OffsiteGrowthBudgetGB, "off-site")
}

// checkPrimaryRemoteBudget is checkOffsiteBudget's counterpart for a domain's
// remote PRIMARY (issue #152): when repo is remote and the domain has a saved
// primary-remote safety config (primaryRemoteTarget) with a growth budget set,
// it samples the LOCAL repo_stats source fresh (repo IS the local/primary
// path — there is no separate off-site copy to sample, unlike the off-site
// budget checks above, so "local" is the only meaningful source) and compares
// against that config's OWN GrowthBudgetGB, latched independently of the
// off-site budget (a "primary:"-prefixed key, so a domain that ALSO has an
// off-site budget alarms for each independently). A local primary, or a remote
// one with no saved safety config, is a silent no-op — exactly like every
// other primary-remote safety feature when unconfigured.
func (s *Service) checkPrimaryRemoteBudget(ctx context.Context, domain, repo string, settings store.Settings) {
	if !restic.IsRemoteRepo(repo) {
		return
	}
	t, ok := s.primaryRemoteTarget(domain)
	if !ok || !t.Enabled || t.GrowthBudgetGB <= 0 {
		return
	}
	// Sample synchronously (mirroring copyToOffsiteTarget's budget-set path) so
	// the very first remote-primary backup after the safety settings are saved
	// is judged against a fresh size, not a stale/missing sample.
	if serr := s.CollectStats(ctx, domain, "local"); serr != nil {
		log.Printf("api: primary-remote %s: budget size sample failed (backup is safe): %v", domain, serr) //nolint:gosec // G706: domain is a fixed literal
	}
	s.checkGrowthBudget(ctx, domain, "local", "primary:"+domain, t.GrowthBudgetGB, "primary")
}

// checkGrowthBudget is the shared compare-latch-notify core behind
// checkOffsiteBudget, checkOffsiteBudgetForTarget and checkPrimaryRemoteBudget:
// it reads the newest repo_stats sample for domain+source, compares it against
// budgetGB (a <=0 budget is "off", a no-op), and fires notifyOverBudget exactly
// once per false→true crossing of the per-latchKey latch (offsiteOverBudget;
// the map is shared across all three callers, so each passes its own
// collision-free key — offsiteBudgetLatchKey for a multi-target off-site
// destination, the bare domain for the single-off-site-budget path, and a
// "primary:"-prefixed key for a remote primary — so a domain's off-site and
// primary budgets alarm independently). kind ("off-site" | "primary") only
// selects notifyOverBudget's wording; a missing sample (async collection has
// not landed yet, or the domain has never been backed up) is silently skipped.
func (s *Service) checkGrowthBudget(ctx context.Context, domain, source, latchKey string, budgetGB int, kind string) {
	if budgetGB <= 0 {
		return // budget disabled
	}
	stat, found, err := s.store.LatestRepoStat(domain, source)
	if err != nil {
		log.Printf("api: %s %s: budget check could not read latest sample: %v", kind, domain, err) //nolint:gosec // G706: kind/domain are fixed-whitelist values
		return
	}
	if !found {
		return // no sample yet — nothing to compare
	}
	budgetBytes := int64(budgetGB) * 1024 * 1024 * 1024
	over := stat.RawSize > budgetBytes

	// Latch the state under the mutex and detect the false→true crossing so the
	// alarm fires exactly once per breach (not on every backup/replication while
	// over).
	s.budgetMu.Lock()
	if s.offsiteOverBudget == nil {
		s.offsiteOverBudget = map[string]bool{}
	}
	prev := s.offsiteOverBudget[latchKey]
	s.offsiteOverBudget[latchKey] = over
	s.budgetMu.Unlock()

	if over && !prev {
		s.notifyOverBudget(ctx, domain, stat.RawSize, budgetBytes, kind)
	}
}

// notifyOverBudget sends a best-effort alert when a domain's off-site repo OR
// remote primary (kind: "off-site" | "primary") first crosses its growth
// budget. It mirrors notifyProtectionLost/notifyDrillFailure's policy gate +
// Unraid fan-out; a no-op when notifications are off.
func (s *Service) notifyOverBudget(ctx context.Context, domain string, size, budget int64, kind string) {
	c, err := s.NotifyConfig()
	if err != nil || c.On == "" || c.On == "never" {
		return
	}
	subject := strings.ToUpper(kind[:1]) + kind[1:] + " backup over budget for " + domain
	action := "Prune the far side or raise the budget."
	if kind == "primary" {
		action = "Adjust the retention policy, prune it, or raise the budget."
	}
	msg := fmt.Sprintf("The %s repository for %s has grown to %s, over the configured growth budget of %s. %s", kind, domain, humanBytes(size), humanBytes(budget), action)
	notify.Send(ctx, c, domain, notify.Event{Title: "BombVault", Message: subject + " — " + msg, OK: false})
	if s.unraidGate(c.Unraid) {
		if e := s.sendUnraidNotify(ctx, "BombVault: "+subject, msg, "warning"); e != nil {
			log.Printf("notify: unraid: %v", e)
		}
	}
}

// offsiteReplicatesOnOwnSchedule reports whether a domain's off-site copy is driven
// by its OWN cron entry (a real, ENABLED cadence) rather than coupled to the backup
// run. Both a blank schedule and the literal "off" parse to a disabled cadence and
// therefore mean "coupled" — the backup path owns replication. Using ParseCadence
// makes this decision IDENTICAL to the scheduler's registration logic (the separate
// off-site cron entry registers only for an enabled cadence, schedule.go), so a
// domain can never fall between the two gates and be silently never replicated: a
// user who sets the off-site schedule to "off" now gets coupled replication instead
// of a silently rotting off-site copy (#95 review). A cadence that fails to parse
// defaults to coupled (replicate) — the safe direction.
func (s *Service) offsiteReplicatesOnOwnSchedule(domain string, settings store.Settings) bool {
	cad, err := schedule.ParseCadence(s.offsiteScheduleFor(domain, settings))
	return err == nil && cad.Enabled
}

// bulkReplicateSuppressKey marks a context whose post-backup inline off-site
// replication is DEFERRED to a single batched pass after the whole scheduled
// domain loop (issue #95). A blank off-site schedule normally couples replication
// to each backup; during a scheduled multi-item run that couples a full off-site
// repo open + index reload to EVERY item (44 containers → 44 high-latency B2
// round-trips, turning a ~seconds job into ~hours). The scheduled multi-item
// closures (containers/VMs/files) set this flag so each item's inline replication
// is skipped and the scheduler runs ONE ReplicateOffsiteAfterBulk per domain
// instead. Manual "Back up now" does NOT set it, so a single-item backup still
// replicates immediately.
type bulkReplicateSuppressKey struct{}

// WithBulkReplicateSuppressed defers a context's inline off-site replication to a
// batched post-loop pass (see bulkReplicateSuppressKey). Set by the scheduled
// multi-item backup closures in main.go; read by replicateOffsite.
func WithBulkReplicateSuppressed(ctx context.Context) context.Context {
	return context.WithValue(ctx, bulkReplicateSuppressKey{}, true)
}

// bulkReplicateSuppressed reports whether inline off-site replication is deferred
// to a batched post-loop pass for this context.
func bulkReplicateSuppressed(ctx context.Context) bool {
	v, _ := ctx.Value(bulkReplicateSuppressKey{}).(bool)
	return v
}

// runGroupKey marks a context whose backup call is one CHILD step of a
// "Backup Everything" pass (a sequential run over every domain — containers,
// vms, flash, files, config — triggered as one unit, e.g. so a dead-man's-
// switch ping can fire only once everything is done). The value is the
// PARENT run's id; runsAdapter/startedRunsAdapter read it via
// runGroupFromContext and stamp it onto the CHILD run they just started
// (store.SetRunGroup), so that run is durably traceable back to the pass
// that produced it. Unset by every caller today — a pure no-op until
// BackupEverything (internal/api/everything.go) starts setting it.
type runGroupKey struct{}

// WithRunGroup marks ctx as belonging to the "Backup Everything" pass whose
// parent run id is groupID (see runGroupKey). Set by BackupEverything around
// each domain's own backup entry point (s.Backup/s.BackupVM/s.BackupFlash/
// s.BackupFileSet/s.BackupConfig); read by runsAdapter/startedRunsAdapter
// when they record that call's child run.
func WithRunGroup(ctx context.Context, groupID string) context.Context {
	return context.WithValue(ctx, runGroupKey{}, groupID)
}

// runGroupFromContext reports the parent run id this context's backup call
// belongs to, or "" when it isn't part of a "Backup Everything" pass — true
// for every context in the codebase today, and for every restore/other-kind
// runsAdapter construction site that isn't part of such a pass. A nil ctx
// (e.g. a zero-value runsAdapter/startedRunsAdapter built without one) is
// treated the same as "no group", never a panic.
func runGroupFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	v, _ := ctx.Value(runGroupKey{}).(string)
	return v
}

// ReplicateOffsiteAfterBulk runs ONE off-site replication for a domain after a
// scheduled multi-item backup loop, replacing the per-item inline replication that
// the scheduled run suppressed (issue #95). It is a no-op when the domain has no
// off-site repo, or when the domain replicates on its OWN off-site schedule (that
// path fires from its own cron entry). Like ScheduledReplicateOffsite it takes the
// domain lock (so it serialises after the last item's backup releases it) and
// NOTIFIES on failure — a scheduled replication that silently failed would let the
// off-site copy rot unseen.
func (s *Service) ReplicateOffsiteAfterBulk(ctx context.Context, domain string) {
	settings, err := s.store.GetSettings()
	if err != nil {
		log.Printf("api: offsite %s: batched replicate: read settings: %v", domain, err) //nolint:gosec // G706: domain is a fixed literal
		return
	}
	if s.offsiteRepoFor(domain, settings) == "" {
		return // no off-site configured for this domain
	}
	if cadence := s.offsiteScheduleFor(domain, settings); s.offsiteReplicatesOnOwnSchedule(domain, settings) {
		// Replicated by the domain's OWN "<domain>-offsite" cron entry instead, which
		// the scheduler registers from this very cadence. Say so: a scheduled run that
		// backs up and prunes but never replicates looks identical to a broken one from
		// the activity log, and the whole point of this skip is that something ELSE is
		// doing the copy (issue #150).
		log.Printf("api: offsite %s: batched replicate skipped — this domain replicates on its own off-site schedule (%q)", domain, cadence) //nolint:gosec // G706: domain is a fixed literal, cadence is %q-quoted
		return
	}
	if err := s.ScheduledReplicateOffsite(ctx, domain); err != nil {
		log.Printf("api: offsite %s: batched replicate failed: %v", domain, err) //nolint:gosec // G706: domain is a fixed literal
	}
}

// replicateOffsite runs right after a successful local backup (caller holds the
// domain lock). It replicates ONLY when the domain has no separate off-site
// schedule — a blank schedule couples replication to each backup; a set schedule
// hands it to the scheduler instead. Best-effort: the local backup has already
// succeeded, so an off-site failure is logged, never propagated.
func (s *Service) replicateOffsite(ctx context.Context, domain string, settings store.Settings, mode restic.Mode, localRepo string) {
	if bulkReplicateSuppressed(ctx) {
		return // scheduled multi-item run: replicated once after the whole loop (#95)
	}
	if s.offsiteRepoFor(domain, settings) == "" {
		return
	}
	if s.offsiteReplicatesOnOwnSchedule(domain, settings) {
		return // replicated on its own schedule, not after every backup
	}
	if err := s.copyToOffsite(ctx, domain, settings, mode, localRepo); err != nil {
		// domain is a fixed literal; the error is already path-scrubbed by restic.
		log.Printf("api: offsite %s: copy failed (local backup is safe): %v", domain, err)
	}
}

// ReplicateOffsite replicates a domain's local repo to its off-site repo on
// demand — the "Replicate now" button and the scheduled off-site job. Unlike the
// post-backup hook it surfaces the error (so the UI can report it) and takes the
// domain lock to serialise with backups.
func (s *Service) ReplicateOffsite(ctx context.Context, domain string) error {
	settings, localRepo, err := s.domainRepoSource(domain, "local")
	if err != nil {
		return err
	}
	if s.offsiteRepoFor(domain, settings) == "" {
		return errors.New("no off-site repo configured for this domain")
	}
	defer s.lockDomainFor(domain, "replicate")()
	return s.copyToOffsite(ctx, domain, settings, s.ModeFor(settings), localRepo)
}

// StartReplicateOffsite kicks off an on-demand off-site replication in the
// BACKGROUND and returns immediately (#93 follow-up). The "Replicate now"
// handler used to run the copy synchronously on the HTTP request context: a
// first replication of real data easily outlives browser/proxy timeouts, and
// when the client gave up (504) the request context was cancelled — killing the
// running restic copy after a gigabyte or two. The copy now runs detached with
// its own generous ceiling; the existing off-site indicator shows it running,
// the run is recorded like any other, and a failure notifies exactly like a
// scheduled replication. Configuration errors and a busy domain still surface
// synchronously so the UI can show them.
func (s *Service) StartReplicateOffsite(domain string) error {
	settings, _, err := s.domainRepoSource(domain, "local")
	if err != nil {
		return err
	}
	if s.offsiteRepoFor(domain, settings) == "" {
		return errors.New("no off-site repo configured for this domain")
	}
	unlock, ok := s.tryLockDomainFor(domain, "replicate")
	if !ok {
		return errDomainBusy
	}
	go func() {
		defer s.recoverOperation("offsite replicate: "+domain, nil, func(msg string) {
			s.failStuckRun(domainRunTargetID(domain), msg)
		})
		defer unlock()
		// Detached from the HTTP request on purpose; the ceiling only guards
		// against a copy that hangs forever on a dead link.
		ctx, cancel := context.WithTimeout(context.Background(), 24*time.Hour)
		defer cancel()
		settings, localRepo, err := s.domainRepoSource(domain, "local")
		if err != nil {
			log.Printf("api: offsite %s: replicate start: %v", domain, err) //nolint:gosec // G706: domain is a fixed literal
			return
		}
		if err := s.copyToOffsite(ctx, domain, settings, s.ModeFor(settings), localRepo); err != nil {
			log.Printf("api: offsite %s: manual replication failed: %v", domain, err) //nolint:gosec // G706: domain is a fixed literal
			s.notifyReplicationFailed(ctx, domain, truncateRunErr(err))
		}
	}()
	return nil
}

// ScheduledReplicateOffsite runs a scheduled off-site replication and, unlike the
// interactive ReplicateOffsite (whose error the UI surfaces directly), NOTIFIES on
// failure. A scheduled replication that silently failed would let the off-site
// copy rot unseen — the scorecard's currency check would only catch it later — so
// a scheduled failure alerts immediately. Best-effort notify; the error is still
// returned for the scheduler log.
func (s *Service) ScheduledReplicateOffsite(ctx context.Context, domain string) error {
	// Detached upper bound so a copy wedged on a dead link can't hold the domain
	// lock forever and block subsequent backups (mirrors the manual
	// StartReplicateOffsite ceiling). SkipIfStillRunning stops the NEXT run from
	// piling on; this stops the current one from hanging indefinitely.
	ctx, cancel := context.WithTimeout(ctx, 24*time.Hour)
	defer cancel()
	err := s.ReplicateOffsite(ctx, domain)
	if err != nil {
		s.notifyReplicationFailed(ctx, domain, truncateRunErr(err))
	}
	return err
}

// notifyReplicationFailed sends a best-effort alert when a SCHEDULED off-site
// replication fails (the off-site copy is not current). Mirrors
// notifyOverBudget/notifyProtectionLost's policy gate + Unraid fan-out; a no-op
// when notifications are off.
func (s *Service) notifyReplicationFailed(ctx context.Context, domain, detail string) {
	c, err := s.NotifyConfig()
	if err != nil || c.On == "" || c.On == "never" {
		return
	}
	subject := "Off-site replication FAILED for " + domain
	msg := fmt.Sprintf("The scheduled off-site replication for %s failed — the off-site copy is not current: %s", domain, detail)
	notify.Send(ctx, c, domain, notify.Event{Title: "BombVault", Message: subject + " — " + msg, OK: false})
	if s.unraidGate(c.Unraid) {
		if e := s.sendUnraidNotify(ctx, "BombVault: "+subject, msg, "warning"); e != nil {
			log.Printf("notify: unraid: %v", e)
		}
	}
}

// TestOffsite probes a domain's off-site repo without modifying it, so the UI can
// tell the user whether the configured location is a reachable, initialised restic
// repository BEFORE relying on it. It uses the SAME probe EnsureRepo uses to detect
// an existing repo — `restic cat config` (ResticEngine.RepoOpensErr) — trying both
// encryption modes, so a repo created under the opposite Encryption setting still
// counts as initialised (that mode mismatch is reported by EnsureRepo, not here).
//
// reachable reports the repo could be opened at all; initialized that it is a real
// restic repository. A REMOTE repo that is reachable but has never been replicated
// to yet fails `cat config` with restic's "repository does not exist" — the SAME
// signal isRepoUninitialized/listSnapshots already treat as "reachable, just empty"
// (issue #117) rather than a genuine failure, so TestOffsite applies the same check
// and reports reachable=true, initialized=false, err=nil for it (the UI's existing
// "uninitialized" warning state — BombVault creates the repo on the first
// replication, this is expected, not an error; issue #130). Any OTHER probe failure
// (a bad rclone remote type, wrong credentials, a backend that refuses the
// connection, ...) is a genuine reachability failure: reported as neither reachable
// nor initialised, with err carrying the primary mode's probe failure instead of a
// bare false/false — the handler scrubs it before it reaches the client. An
// unconfigured off-site repo for the domain is also an error.
func (s *Service) TestOffsite(ctx context.Context, domain string) (reachable, initialized bool, err error) {
	settings, err := s.store.GetSettings()
	if err != nil {
		return false, false, fmt.Errorf("read settings: %w", err)
	}
	loc := s.offsiteRepoFor(domain, settings)
	if loc == "" {
		return false, false, errors.New("no off-site repo configured for this domain")
	}
	repo, err := s.resolveRepo(loc)
	if err != nil {
		return false, false, err
	}
	return s.probeOffsiteRepo(ctx, repo, s.ModeFor(settings))
}

// TestOffsiteTarget runs the SAME probe as TestOffsite against ONE off-site
// destination addressed by id, so every additional target can be verified on its
// own. TestOffsite only ever probes a domain's PRIMARY target, which let an
// extra destination stay silently broken behind a green "Test connection"
// (issue #138). The target's own S3 storage class is applied
// (offsiteModeForTarget) exactly as replication does.
//
// An unknown id is an error rather than a fallback to the primary: a test that
// silently probed something ELSE is precisely the failure mode being fixed here.
func (s *Service) TestOffsiteTarget(ctx context.Context, id string) (reachable, initialized bool, err error) {
	settings, err := s.store.GetSettings()
	if err != nil {
		return false, false, fmt.Errorf("read settings: %w", err)
	}
	target, ok, err := s.store.GetOffsiteTarget(id)
	if err != nil {
		return false, false, fmt.Errorf("read off-site target: %w", err)
	}
	if !ok {
		return false, false, errors.New("no such off-site target")
	}
	if target.Repo == "" {
		return false, false, errors.New("this off-site target has no repository configured")
	}
	repo, err := s.resolveRepo(target.Repo)
	if err != nil {
		return false, false, err
	}
	return s.probeOffsiteRepo(ctx, repo, s.offsiteModeForTarget(settings, target))
}

// probeOffsiteRepo is the shared reachable/initialised probe behind TestOffsite
// and TestOffsiteTarget: it opens an ALREADY-RESOLVED repo read-only in both
// encryption modes and classifies the outcome. See TestOffsite for the full
// contract of the three results.
func (s *Service) probeOffsiteRepo(ctx context.Context, repo string, mode restic.Mode) (reachable, initialized bool, err error) {
	// Bound each probe so a dead backend fails fast instead of hanging the
	// request (cat config over an unreachable REST server can otherwise stall).
	// PER attempt, not shared: a cold sftp connection over a VPN (Tailscale
	// tunnel + host-key pinning) can eat a shared budget on the first try and
	// leave the second probe zero time — reporting a reachable repo as
	// unreachable (#93).
	probe := func(m restic.Mode) error {
		pctx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		return s.engine.RepoOpensErr(pctx, repo, m)
	}
	primaryErr := probe(mode)
	if primaryErr == nil {
		return true, true, nil
	}
	// Opens under the opposite encryption mode → the repo exists and is reachable,
	// just created under the other Encryption setting; still reachable + initialised
	// for this probe (EnsureRepo surfaces the mismatch on the next backup).
	if probe(s.oppositeMode(mode)) == nil {
		return true, true, nil
	}
	// A reachable REMOTE destination that simply has no repository yet (no
	// replication has run) is not a failure — same signal listSnapshots already
	// treats as "empty, not fatal" (#117). Report it as the UI's uninitialized
	// state instead of a fatal error (#130).
	if restic.IsRemoteRepo(repo) && isRepoUninitialized(primaryErr) {
		return true, false, nil
	}
	// Neither mode opened and it's not the uninitialized case — surface the primary
	// mode's failure (the user's actual configured encryption setting) instead of a
	// silent false/false, so the UI can show the real reason (issue: roachman,
	// off-site "not reachable" with no detail).
	return false, false, primaryErr
}

// EnsureRepo makes sure the restic repo at repo is ready to use with the
// configured encryption mode. It is idempotent AND reconciles the mode:
//
//   - opens with mode                  → exists and consistent; nothing to do
//   - opens only with the opposite mode → the Encryption setting was toggled
//     against an existing repo; return a clear, actionable error instead of
//     letting every later restic call fail cryptically
//   - opens with neither mode          → not initialised yet, so create it
//
// The probe is `restic cat config` (cheap, needs no lock). Telling a real mode
// mismatch apart from a not-yet-created repo is what stops a flipped Encryption
// setting from silently breaking backups (issue #14).
func (s *Service) EnsureRepo(ctx context.Context, repo string, mode restic.Mode) error {
	// Fast path: the repo opens with the configured mode → it exists and its
	// encryption mode matches. This is the common case on every backup after the
	// first, and it replaces the old `config`-marker stat (which never checked the
	// mode) with one that does.
	if s.engine.RepoOpens(ctx, repo, mode) {
		s.markRepoEstablished(repo) // remember it for the not-mounted guard (#55)
		return nil
	}
	// It did not open. Probe the OPPOSITE encryption mode (same backend creds): if
	// that opens it, the repo exists but was created under the other mode — the
	// user toggled the Encryption setting. Fail fast with an actionable message
	// rather than running Init (which would log "config already exists") and then
	// failing every subsequent backup against the now-mismatched repo.
	if s.engine.RepoOpens(ctx, repo, s.oppositeMode(mode)) {
		return fmt.Errorf("this backup repository was created %s, but the Encryption setting is now %s, so "+
			"restic cannot open it after the change. Set Encryption back to %s, or point this backup at a "+
			"new, empty repository location",
			encryptionWord(!mode.Encrypted), enabledWord(mode.Encrypted), enabledWord(!mode.Encrypted))
	}
	// Opens with neither mode → not initialised (a brand-new location) OR its
	// backing store vanished. Local repos need their directory; remote backends do not.
	if !restic.IsRemoteRepo(repo) {
		// #55/#120: a repo was established here before but its `config` is now
		// missing. Two very different causes:
		//   - The backing store genuinely vanished (typically a remote share that
		//     mounts AFTER the container started, so it is invisible right now).
		//     RE-INIT would write an empty repo that shadows the real backups once
		//     the share reappears, so we REFUSE and surface "not mounted" (#55).
		//   - The destination is present, writable, and mounted, but this repo is
		//     legitimately not there yet — e.g. a phantom marker from a pre-mount
		//     init on a UD disk that mounted over it later (#120). The established
		//     marker is permanent (nothing deletes it, so a restart cannot clear
		//     it), so we must recognise the healthy disk and re-establish on it.
		// destinationMounted (kernel mount table, not a stat/write probe) tells the
		// two apart: an unmounted mountpoint dir is often still writable, which is
		// exactly the #55 case we keep protecting.
		if localRepoMissing(repo) && s.repoEstablished(repo) {
			if !s.destinationMounted(repo) {
				return ErrBackupPathNotMounted // #55: backing store not mounted
			}
			// #120: stale/phantom marker on a live disk — drop it and fall through
			// to EnsureDir+Init so the repo is re-established on the mounted disk.
			s.clearRepoEstablished(repo)
		}
		// Genuine first run (marker unset): create the repo dir chain as before.
		// The marker guard above is what prevents re-initialising over an
		// established-but-now-unmounted repo, so MkdirAll here is safe.
		if err := paths.EnsureDir(repo); err != nil {
			return fmt.Errorf("ensure repo dir: %w", err)
		}
	}
	if err := s.engine.Init(ctx, repo, mode); err != nil {
		// Tolerate a race / pre-existing repo: the scrubbed adapter error may not
		// name the cause, so re-probe with the configured mode before failing.
		if s.engine.RepoOpens(ctx, repo, mode) {
			s.markRepoEstablished(repo)
			return nil
		}
		if !strings.Contains(strings.ToLower(err.Error()), "already") {
			return fmt.Errorf("init repo: %w", err)
		}
	}
	// Mark established only when the repo VERIFIABLY opens now (a real config was
	// written), never on a no-op init, so the not-mounted guard can only trip on a
	// location that genuinely held a repo.
	if s.engine.RepoOpens(ctx, repo, mode) {
		s.markRepoEstablished(repo)
	}
	return nil
}

// ErrBackupPathNotMounted is returned when a LOCAL backup repo BombVault
// previously established is now unreadable — typically a remote share (e.g. under
// /mnt/remotes) that mounts AFTER the container started, so it stays invisible to
// the running container until the mount is restored. BombVault refuses to
// re-initialise an empty repo over it (which would shadow the real backups) and
// surfaces this instead of a misleading "no backups" (#55).
var ErrBackupPathNotMounted = errors.New("backup path is not mounted yet: a remote backup share may mount late at boot; it recovers once the mount is available (restart BombVault if it persists)")

// markRepoEstablished records a successfully created/opened LOCAL repo so a later
// open-failure can be told apart from a fresh location (#55). Remote repos have
// no local backing store to vanish, so they are not tracked. Best-effort.
func (s *Service) markRepoEstablished(repo string) {
	if restic.IsRemoteRepo(repo) {
		return
	}
	if err := s.store.MarkRepoEstablished(repo); err != nil {
		log.Printf("api: mark repo established: %v", err)
	}
}

// clearRepoEstablished removes the established marker for a LOCAL repo, used when
// the destination is confirmed mounted but the repo legitimately is not there yet
// (a stale/phantom pre-mount marker, #120). Remote repos have no marker. Best-effort.
func (s *Service) clearRepoEstablished(repo string) {
	if restic.IsRemoteRepo(repo) {
		return
	}
	if err := s.store.ClearRepoEstablished(repo); err != nil {
		log.Printf("api: clear repo established: %v", err)
	}
}

// repoEstablished reports whether a LOCAL repo destination was previously
// established. False for remote repos and on any store error (never blocks).
func (s *Service) repoEstablished(repo string) bool {
	if restic.IsRemoteRepo(repo) {
		return false
	}
	ok, err := s.store.IsRepoEstablished(repo)
	if err != nil {
		log.Printf("api: is repo established: %v", err)
		return false
	}
	return ok
}

// oppositeMode returns mode with its encryption flag flipped, preserving backend
// credentials (Env). The encrypted variant carries the APP_KEY-derived repo
// password so a probe can actually open an encrypted repo; the unencrypted
// variant clears it.
func (s *Service) oppositeMode(mode restic.Mode) restic.Mode {
	o := mode
	o.Encrypted = !mode.Encrypted
	if o.Encrypted {
		o.Password = restickey.Derive(s.cfg.AppKey)
	} else {
		o.Password = ""
	}
	return o
}

// enabledWord renders an Encryption setting state in the UI's wording.
func enabledWord(encrypted bool) string {
	if encrypted {
		return "enabled"
	}
	return "disabled"
}

// encryptionWord renders a repository's actual encryption mode.
func encryptionWord(encrypted bool) string {
	if encrypted {
		return "encrypted"
	}
	return "unencrypted"
}

// resolveAppdataPaths returns the CONTAINER-VISIBLE paths to back up for a
// container. Docker reports bind-mount sources as HOST paths (e.g.
// /mnt/user/appdata/<x>/data); BombVault reaches them only through the broad host
// mount (HostSourceRoot mounted at HostMountRoot — e.g. host /mnt → container
// /host/user, so host /mnt/user/appdata/x is reachable at /host/user/user/appdata/x).
// We TRANSLATE every matched bind source from the host root to the container
// mount root and back up the real, correctly cased path — not a guess. A bind is
// kept when its host source contains ANY of the configured data-root segments
// (s.cfg.DataRootSegments, default ["appdata"] — see config.DataRootSegments) as
// a full path segment, OR the container carries a truthy "bombvault.data" label
// (see bombvaultDataLabelTruthy), which forces EVERY bind mount in regardless of
// segment match — the documented escape hatch for a layout neither the segment
// filter nor the compose convention below catches (e.g. "/srv/plex/config" with
// no compose project). Media libraries, the flash, /etc/localtime and other
// non-matching shares are skipped.
//
// A named-volume mount (Type=="volume") is kept UNCONDITIONALLY, no segment
// filter — a named volume is persistent-by-construction, with no equivalent of
// a throwaway bind mount. Its resolved host path (Source, filled in by
// dockercli's Inspect from the daemon's own report or, failing that, a
// VolumeInspect fallback) goes through the SAME translate-and-check as a bind
// source, so an unreachable volume mountpoint is skipped exactly like an
// unreachable bind, and a volume that resolves to the same container path as an
// already-recorded bind is deduped. This fixes the majority case on non-Unraid
// hosts: a container using only Docker Compose named volumes previously
// produced zero discovered data paths.
//
// The standard Docker Compose "com.docker.compose.project.working_dir" label
// (see composeProjectDataDir), present on every container Compose creates, is
// added as an ADDITIONAL candidate independent of whether any bind matched —
// always-on, not behind a config flag, since it costs nothing when absent.
//
// Every candidate (segment-matched binds, the compose working dir, label-
// override binds, and volumes) is deduplicated against every other by cleaned
// absolute container path.
//
// Fallback (nothing matched above): the platform's conventional appdata path
// for <name> (Unraid: /mnt/user/appdata/<name>; generic: none), translated if
// reachable — see platform.Platform.AppdataFallback.
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

	// Docker Compose project working directory: an always-on additional
	// candidate, independent of whether any bind matched above.
	if dir, ok := composeProjectDataDir(in.Config.Labels); ok {
		if container, ok := s.toContainerPath(dir); ok && !seen[container] {
			out = append(out, container)
			seen[container] = true
		}
	}

	if len(out) == 0 {
		// Last resort: the platform's conventional appdata dir for this
		// container — but ONLY if it actually exists. A container with no
		// appdata mount, no such folder, and no platform convention (empty
		// AppdataFallback, e.g. on generic) is stateless: default to an empty
		// selection (config-only backup) rather than a phantom folder that
		// shows as selected yet backs up nothing.
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

// matchesAnyDataRootSegment reports whether path p contains ANY of the given
// data-root segments as a full path segment (hasSegment applied once per
// configured candidate — see config.DataRootSegments).
func matchesAnyDataRootSegment(p string, segments []string) bool {
	for _, seg := range segments {
		if hasSegment(p, seg) {
			return true
		}
	}
	return false
}

// bombvaultDataLabelTruthy reports whether a container opted ALL of its bind
// mounts into resolveAppdataPaths via the "bombvault.data" label — the
// documented per-container escape hatch for a data layout neither the
// configured segment filter nor composeProjectDataDir catches (e.g.
// "/srv/plex/config" with no compose project). Truthy means the label is
// PRESENT and its trimmed value is neither empty nor "false"
// (case-insensitive) — so "true", "1", "yes", or any other non-empty,
// non-"false" value all opt in; an absent label, or an explicit "false"/""
// value, opts out. A container that never sets the label is unaffected either
// way, which is what keeps the Unraid-default behavior unchanged.
func bombvaultDataLabelTruthy(labels map[string]string) bool {
	v, ok := labels["bombvault.data"]
	if !ok {
		return false
	}
	v = strings.TrimSpace(v)
	return v != "" && !strings.EqualFold(v, "false")
}

// composeProjectDataDir reads the standard Docker Compose
// "com.docker.compose.project.working_dir" label — present on every container
// Compose creates — off a container's Config.Labels and returns it as an
// additional host-path candidate for resolveAppdataPaths (translated and
// containment-checked the same way as a bind source), independent of whether
// any bind mount matched the configured segments. Follows the discovery
// pattern used by Nautical Backup. Returns ("", false) when the label is
// absent or its value is empty after trimming.
func composeProjectDataDir(labels map[string]string) (string, bool) {
	dir := strings.TrimSpace(labels["com.docker.compose.project.working_dir"])
	if dir == "" {
		return "", false
	}
	return dir, true
}

// toHostPath is the inverse of toContainerPath: it maps a container-visible path
// under HostMountRoot back to its HOST path under HostSourceRoot (e.g.
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

// ContainerMounts returns the container's bind mounts annotated for the folder
// selector, plus any selected custom paths (in host form) that do not match a
// current mount, each flagged with whether it still exists. The selection is the
// stored explicit choice, or the automatic appdata default when none is
// configured.
func (s *Service) ContainerMounts(ctx context.Context, name string) ([]MountInfo, []CustomPath, error) {
	in, err := s.docker.Inspect(ctx, name)
	if err != nil {
		return nil, nil, fmt.Errorf("inspect container: %w", err)
	}

	auto := s.resolveAppdataPaths(name, in)
	tg, _ := s.store.GetTargetByContainer(name) // absent target → zero value, no selection
	effective := tg.SelectedPaths
	if len(effective) == 0 {
		effective = auto
	}
	selSet := sliceSet(effective)
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

	// Custom = selected paths with no matching current mount, shown in host form.
	// Flag each with whether it still exists under the host mount so the UI can
	// distinguish a real selected folder from a stale/phantom one (issue #115).
	var custom []CustomPath
	for _, cp := range effective {
		if !matched[cp] {
			_, statErr := os.Stat(cp) //nolint:gosec // G703: cp is a stored container path already validated under the mount root on save, not raw user input
			custom = append(custom, CustomPath{Path: s.toHostPath(cp), Exists: statErr == nil})
		}
	}
	return mounts, custom, nil
}

// SetBackupPaths stores the user's explicit backup-folder selection for a
// container. The input paths are HOST paths (what the UI shows); each is
// translated to its container path and must be reachable under the host mount,
// otherwise the whole update is rejected. An empty list clears the selection so
// backups fall back to automatic appdata detection.
func (s *Service) SetBackupPaths(_ context.Context, name string, hostPaths []string) error {
	var cps []string
	seen := map[string]bool{}
	for _, hp := range hostPaths {
		hp = strings.TrimSpace(hp)
		if hp == "" {
			continue
		}
		// toContainerPath path.Cleans the input first (resolving any ".."), then
		// requires the host-source-root prefix, so its result is guaranteed to sit
		// under the mount root — no separate containment check needed.
		cp, ok := s.toContainerPath(hp)
		if !ok {
			return fmt.Errorf("path %q is not under the host mount and can't be backed up", hp)
		}
		if !seen[cp] {
			cps = append(cps, cp)
			seen[cp] = true
		}
	}
	return s.store.SetBackupPaths(name, cps)
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

// configuredBackupPaths returns the paths a container backup is CONFIGURED to
// use — the explicit folder selection if set, otherwise the automatic appdata
// detection — WITHOUT the on-disk filter effectiveBackupPaths applies.
//
// The distinction matters to any reader that can answer from the backup rather
// than from the filesystem (the exclusion assistant's snapshot feeder): an
// unmounted array makes every configured path fail its stat, and treating that
// as "this container has no folders" turns a temporarily unreachable share into
// a confident empty answer (#175 at root granularity). An empty list here means
// genuinely nothing is configured, which is the only case that IS "nothing".
func (s *Service) configuredBackupPaths(name string, in model.Inspect) []string {
	chosen := s.resolveAppdataPaths(name, in)
	if existing, gErr := s.store.GetTargetByContainer(name); gErr == nil && len(existing.SelectedPaths) > 0 {
		chosen = existing.SelectedPaths
	}
	return chosen
}

// effectiveBackupPaths returns the paths a container backup/export actually uses:
// the explicit folder selection if set, otherwise the automatic appdata
// detection, filtered to those that exist on disk (a stateless container ends up
// with an empty list).
func (s *Service) effectiveBackupPaths(name string, in model.Inspect) []string {
	return onlyExistingPaths(s.configuredBackupPaths(name, in))
}

// expectsData reports whether a container ought to have backup data: it has an
// appdata-style bind mount, or the user explicitly selected folders. Used to
// distinguish a genuinely stateless container (empty backup is correct) from one
// whose paths transiently resolved to nothing (appdata not mounted / misconfig),
// so the latter is refused rather than recorded as a successful empty backup.
func (s *Service) expectsData(name string) bool {
	existing, err := s.store.GetTargetByContainer(name)
	if err != nil {
		return false // no prior target — a first backup of a new/stateless container
	}
	// Only when a PREVIOUS backup actually captured data (or the user selected
	// folders) is an empty result suspicious. This avoids refusing the first
	// backup of a brand-new container whose appdata folder doesn't exist yet.
	return len(existing.AppdataPaths) > 0 || len(existing.SelectedPaths) > 0
}

// ErrSelfBackup is returned when a backup targets BombVault's own container.
// Backing it up stops the container mid-run (stop → backup → start), which kills
// the very process doing the backup and takes the app down. Its configuration is
// recovered separately via the encrypted definition mirror (Discover), so there
// is nothing to gain and a crash to lose.
var ErrSelfBackup = errors.New("BombVault won't back up its own container (it would stop itself mid-backup); its configuration is recovered via Discover")

// selfContainerName returns the name of BombVault's OWN container, resolved once
// and cached. The BOMBVAULT_SELF_CONTAINER env (set by the Unraid template) wins;
// otherwise we Inspect our hostname, which Docker defaults to the short container
// ID, and take that container's Name. Returns "" when undetectable (Docker not
// reachable yet) and leaves the cache unset so a later call can retry.
func (s *Service) selfContainerName(ctx context.Context) string {
	s.selfMu.Lock()
	defer s.selfMu.Unlock()
	if s.selfResolved {
		return s.selfName
	}
	if v := strings.TrimSpace(os.Getenv("BOMBVAULT_SELF_CONTAINER")); v != "" {
		s.selfName, s.selfResolved = v, true
		return s.selfName
	}
	name, err := s.docker.Self(ctx)
	if err != nil || name == "" {
		return "" // Docker not reachable / not in a container yet — retry next time
	}
	s.selfName, s.selfResolved = name, true
	return s.selfName
}

// SelfContainerName exposes the detected own-container name to the HTTP layer so
// the container list can flag it (the UI hides its backup action / excludes it
// from "select all").
func (s *Service) SelfContainerName(ctx context.Context) string {
	return s.selfContainerName(ctx)
}

// selfRestartDelay is the brief pause before BombVault restarts its own container,
// so the HTTP response for the triggering request flushes to the client first.
// A package var (not a const) so tests can shrink it.
var selfRestartDelay = 1500 * time.Millisecond

// ScheduleSelfRestart restarts BombVault's own container over the Docker socket
// shortly after returning, so a staged config restore is applied on the reboot
// (the daemon completes the stop+start even though this process dies mid-stop).
// It returns whether an auto-restart was scheduled: false when the self container
// can't be resolved (Docker unreachable / not in a container), in which case the
// caller instructs the user to restart the container manually.
func (s *Service) ScheduleSelfRestart() bool {
	name := s.selfContainerName(context.Background())
	if name == "" {
		return false
	}
	go func() {
		time.Sleep(selfRestartDelay)
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := s.docker.Restart(ctx, name, 10*time.Second); err != nil {
			log.Printf("api: self-restart of %q failed: %v (restart the container manually to apply)", name, err)
			s.batchActive.Store(false) // release the guard so operations can resume; user restarts manually
		}
	}()
	return true
}

// Backup runs a full container backup: resolve repo + mode, ensure the repo,
// inspect the container, find-or-create its target, and drive the orchestrator.
func (s *Service) Backup(ctx context.Context, name string) (_ backup.Summary, retErr error) {
	// A backup must survive the client that triggered it disconnecting — closing
	// the browser tab, or stopping the very container the BombVault UI runs in.
	// Detach from the request's cancellation (keeping its values) with a generous
	// hard cap so a wedged run can't hold the domain lock forever.
	ctx, cancel := backupHoldCtx(ctx)
	defer cancel()
	// Never back up our own container: stopping it mid-run is suicide.
	if self := s.selfContainerName(ctx); self != "" && name == self {
		return backup.Summary{}, ErrSelfBackup
	}
	defer s.lockDomain("containers")() // serialise per repo; blocks maintenance ops meanwhile

	// #64: a domain-wide fault (repo mount lost, disk full, restic repo error) that
	// begins mid-batch trips one of the pre-flight early-returns below — settings,
	// repo path, EnsureRepo, inspect, empty-paths guard, upsert — for EVERY remaining
	// container. None of those reach backup.BackupContainer, which is the ONLY place a
	// failed RUN was recorded, so those failures were INVISIBLE: nothing on the
	// dashboard heatmap/history and only a bare "N failed" count in the notification
	// (the exact #64 report — 10 recorded, 35 missing). Resolve the target up front and,
	// via the named-return finisher below, record a FAILED run carrying the real reason
	// for any such early-return. Once BackupContainer takes over run bookkeeping we set
	// orchestrated=true so this never double-records; a NotFound target is likewise
	// excluded — it records its own "skipped" run (recordAndNotifyContainerSkip).
	var targetID string
	if tg, tErr := s.store.GetTargetByContainer(name); tErr == nil {
		targetID = tg.ID
	}
	orchestrated := false
	defer func() {
		if retErr != nil && !orchestrated && !errors.Is(retErr, backup.ErrContainerNotInstalled) {
			s.recordContainerFailure(name, targetID, retErr)
		}
	}()

	settings, err := s.store.GetSettings()
	if err != nil {
		return backup.Summary{}, fmt.Errorf("read settings: %w", err)
	}
	repo, err := s.containersRepoPath(settings)
	if err != nil {
		return backup.Summary{}, err
	}
	mode := s.ModeFor(settings)
	mode.Limits = s.primaryLimitsFor("containers", repo) // issue #152: a remote primary's saved bandwidth caps, else zero (unlimited)
	if err := s.EnsureRepo(ctx, repo, mode); err != nil {
		return backup.Summary{}, err
	}
	// Clear any stale lock left by a previously interrupted run so it can't block
	// this backup (BombVault is the sole writer; an active lock is never stale).
	s.unlockStale(ctx, repo, mode)

	in, err := s.docker.Inspect(ctx, name)
	if err != nil {
		// The container is gone (removed) but is still a scheduled target. A
		// scheduled target can outlive its container, so skip it with a sentinel
		// instead of failing — the scheduler treats ErrContainerNotInstalled as a
		// skip, so the nightly job no longer errors or false-alarms Healthchecks
		// (#57). Record a "skipped" run (so the dashboard shows it and agrees with
		// the green aggregate ping) and warn the user once. Mirrors the VM path
		// (BackupVM → ErrVMNotInstalled). Returns before any real run is recorded.
		if dockercli.IsNotFound(err) {
			log.Printf("api: Backup: skipping %q — not present on host (not installed; backups only)", name) //nolint:gosec // G706: name is %q-quoted
			s.recordAndNotifyContainerSkip(ctx, name)
			return backup.Summary{}, backup.ErrContainerNotInstalled
		}
		return backup.Summary{}, fmt.Errorf("inspect container: %w", err)
	}
	// The paths actually backed up: the explicit folder selection if set, else the
	// automatic appdata detection, filtered to those that exist. A stateless
	// container ends up with an empty list → a definition-only backup (its
	// template/inspect is still captured so it can be recreated on restore).
	effective := s.effectiveBackupPaths(name, in)

	// Guard against a SILENT no-op: if a PREVIOUS backup captured data (or the user
	// selected folders) but every path now resolves away — e.g. the appdata share
	// isn't mounted right now, or HOST_SOURCE_ROOT is misconfigured — refuse instead
	// of recording an empty backup that looks successful and overwrites the stored
	// path list. A first backup of a new/stateless container is unaffected.
	if len(effective) == 0 && s.expectsData(name) {
		err := fmt.Errorf("backup %q: its backup folders are not reachable right now (is the appdata share mounted?). Refusing an empty backup that would look successful", name)
		s.notifyBackup(ctx, "container", name, false, backup.Summary{}, err)
		return backup.Summary{}, err
	}

	// Persist the recreate recipe (self-contained: inspect + template + backup
	// paths) so restore works even after the container has been deleted.
	xml, _, _ := template.Read(s.cfg.FlashTemplatesDir, name)
	defBytes, _ := json.Marshal(containerDefinition{Inspect: in, TemplateXML: xml, AppdataPaths: effective})
	defJSON := string(defBytes)

	tg, err := s.store.UpsertTarget(store.Target{ContainerName: name, AppdataPaths: effective, Definition: defJSON})
	if err != nil {
		return backup.Summary{}, fmt.Errorf("upsert target: %w", err)
	}

	// Give each dependency its own run-state so the backup never starts a
	// container the user had already stopped (#33): inspect each by name and
	// carry WasRunning (mirroring the target's in.Running). A dependency we
	// cannot inspect (e.g. removed) is logged and left untouched.
	var deps []backup.StopContainer
	for _, dep := range tg.StopContainers {
		di, dErr := s.docker.Inspect(ctx, dep)
		if dErr != nil {
			log.Printf("api: backup: inspect dependency %q: %v (leaving as-is)", dep, dErr) //nolint:gosec // G706: dep is %q-quoted
			continue
		}
		// Carry the dependency's compose identity so the restart-after-backup phase
		// can bring the stopped set back up in depends_on order (dependencies first)
		// and, when enabled, wait for each to be healthy before its dependents (#119).
		deps = append(deps, backup.StopContainer{
			Name:       dep,
			WasRunning: di.Running,
			Service:    composeService(di.Config.Labels),
			DependsOn:  parseDependsOn(di.Config.Labels),
		})
	}

	// #119 follow-up: "update after successful backup" RECREATES the container
	// (stop/remove/create) after the backup. If the stopped dependents were already
	// brought back (Part B) by then, they break against the target while it is torn
	// down for the recreate (bostafari: a dependent fires against a down target,
	// connection refused). Hand the update to the orchestrator as its
	// WhileDependentsStopped hook so it runs INSIDE the stop window — the dependents
	// are restarted only after the recreate. Only for a running, opted-in container;
	// otherwise nil, so the dependents restart right after the backup (unchanged).
	var whileDepsStopped func()
	if tg.UpdateAfterBackup && in.Running {
		whileDepsStopped = func() { s.updateContainerAfterBackup(ctx, name, in, tg.ID) }
	}

	pkey := "container:" + name
	// Healthchecks /start ping: deferred to here, past every pre-flight early-return,
	// so the paired done/fail notifyBackup below always follows (no dangling /start).
	s.notifyBackupStart(ctx, "container")
	bctx, startedAt := s.progBegin(ctx, pkey, "backup")
	// BackupContainer now owns run bookkeeping (it records its own failed/success run),
	// so the pre-flight failure finisher above must stand down to avoid a double record.
	orchestrated = true
	sum, err := backup.BackupContainer(bctx, backup.BackupDeps{
		ContainerRef:           name,
		ContainerName:          name,
		RepoPath:               repo,
		AppdataPaths:           effective,
		StopTimeout:            30 * time.Second,
		TargetID:               tg.ID,
		SnapshotTemplatesDir:   filepath.Join(s.cfg.DataDir, "templates"),
		FlashTemplatesDir:      s.cfg.FlashTemplatesDir,
		WasRunning:             in.Running,
		PreHook:                tg.PreHook,
		PostHook:               tg.PostHook,
		StopContainers:         deps,
		HealthWait:             settings.RestartHealthWait,
		HealthTimeout:          time.Duration(settings.RestartHealthTimeoutSec) * time.Second,
		WhileDependentsStopped: whileDepsStopped,
		Excludes:               s.resolveExcludePatterns(tg.Excludes, in),
		Docker:                 s.docker,
		Restic:                 &resticAdapter{engine: s.engine, mode: mode},
		Templates:              templatesAdapter{},
		Runs:                   runsAdapter{st: s.store, ctx: ctx},
	})
	s.progEnd(pkey, "backup", err == nil, startedAt)
	s.notifyBackup(ctx, "container", name, err == nil, sum, err)
	if err != nil {
		return backup.Summary{}, err
	}

	// Mirror the definition (encrypted) onto the backup storage so a freshly
	// installed BombVault can rebuild its state via Discover after losing
	// /config. Best-effort: a write failure must never fail a good backup.
	if wErr := s.writeDefToStorage(settings, name, defBytes); wErr != nil {
		log.Printf("api: backup: WARN could not persist definition for %q to storage: %v", name, wErr) //nolint:gosec // G706: name is %q-quoted
	}
	// #52: the optional post-backup image update already ran, if enabled, as the
	// orchestrator's WhileDependentsStopped hook (above) — inside the stop window,
	// so a recreate of the target completes BEFORE its dependents are restarted
	// (#119). The backup + fresh snapshot are its safety net; any failure there is
	// logged + recorded as a failed "update" run, but never fails the backup.
	s.applyRetention(ctx, repo, settings, mode, "container:"+name, "containers")
	makeRepoReadable(repo) // keep the local repo copyable off-box by a non-root user
	s.replicateOffsite(ctx, "containers", settings, mode, repo)
	s.maybeCollectStats(ctx, "containers")
	s.checkPrimaryRemoteBudget(ctx, "containers", repo, settings)
	return sum, nil
}

// updateContainerAfterBackup implements the per-container "update after backup"
// opt-in (#52): pull the container's image and, only if a newer image actually
// arrived, recreate the container from its live inspect (which then resolves to
// the new image). Recorded as its own "update" run so the recreate is visible
// in Run History. Best-effort: a failure here never fails the backup — the fresh
// snapshot lets the user roll back a bad update.
func (s *Service) updateContainerAfterBackup(ctx context.Context, name string, in model.Inspect, targetID string) {
	ref := in.Config.Image
	if ref == "" {
		ref = in.Image
	}
	// #106: images in a private/sponsor-gated registry need credentials — resolve
	// the ref's registry host against the stored registry credentials and pass
	// the match along ("" = no credential = the previous anonymous behavior).
	if err := s.docker.PullWithAuth(ctx, ref, s.registryAuthFor(ref)); err != nil {
		// Reached, but couldn't even CHECK for an update (registry rate-limit, auth,
		// network). Record a failed "update" run so "why wasn't this updated?" is
		// answerable from Run History / the Activity Log instead of vanishing into
		// the server log (#95). The backup itself already succeeded.
		log.Printf("api: update-after-backup: pull %q failed (backup is safe): %v", name, err) //nolint:gosec // G706: name is %q-quoted
		s.recordUpdateFailure(name, targetID, fmt.Errorf("pull image: %w", err))
		return
	}
	newID, err := s.docker.ImageID(ctx, ref)
	if err != nil {
		log.Printf("api: update-after-backup: resolve image id for %q failed: %v", name, err) //nolint:gosec // G706: name is %q-quoted
		s.recordUpdateFailure(name, targetID, fmt.Errorf("resolve image id: %w", err))
		return
	}
	// Nothing newer arrived → no recreate, and deliberately still NO run record
	// (44 "nothing happened" rows a night would drown Run History). The check DID
	// complete though — stamp it on the target so the UI can show "checked, up to
	// date" instead of leaving it indistinguishable from "never reached".
	if newID == "" || newID == in.Image {
		s.setUpdateCheck(name, "up-to-date")
		return
	}
	// context.Background(), not ctx: an "update" run is a side effect of the
	// container backup, not one of the five domain steps a "Backup Everything"
	// pass groups (see runGroupKey's doc comment) — deliberately excluded here,
	// same as recordUpdateFailure's failure path, so whether an "update" run
	// gets grouped never depends on which of the two paths happens to fire.
	// Applies to all three runsAdapter sites below; ctx itself is still the live
	// context and stays in use above for the Docker calls.
	runID, rErr := runsAdapter{st: s.store, ctx: context.Background()}.Start(targetID, "update")
	if rErr != nil {
		log.Printf("api: update-after-backup: start run for %q: %v", name, rErr) //nolint:gosec // G706: name is %q-quoted
		return
	}
	if err := s.recreateForUpdate(ctx, name, in); err != nil {
		_ = runsAdapter{st: s.store, ctx: context.Background()}.Finish(runID, "failed", "", 0, truncateRunErr(err))
		s.setUpdateCheck(name, "failed")
		log.Printf("api: update-after-backup: recreate %q failed (backup is safe): %v", name, err) //nolint:gosec // G706: name is %q-quoted
		return
	}
	_ = runsAdapter{st: s.store, ctx: context.Background()}.Finish(runID, "success", "", 0, "")
	s.setUpdateCheck(name, "updated")

	// #116: BombVault just recreated the container, so its image tag moved to the
	// new digest — but Unraid did not perform the update and still shows a stale
	// "update available" banner on the Docker tab, because its cached status file
	// was never refreshed. Ask Unraid to run its OWN update-status recheck over the
	// existing host SSH link so it rewrites that file itself. Only fires here (an
	// update actually happened), opt-out via the toggle, best-effort and non-fatal.
	if st, sErr := s.store.GetSettings(); sErr == nil && st.ReconcileUnraidUpdateStatus {
		s.reconcileUnraidUpdateStatus(ctx, ref)
	}

	// #56: optionally remove the now-superseded old image. Opt-in (default off) — the
	// old image is what makes a fresh-snapshot rollback cheap, so we never prune by
	// default. Best-effort with force=false, so the daemon refuses if another
	// container still references the image (a shared base image is never deleted).
	if st, sErr := s.store.GetSettings(); sErr == nil && st.PruneImageAfterUpdate && in.Image != "" {
		if rErr := s.docker.ImageRemove(ctx, in.Image); rErr != nil {
			log.Printf("api: update-after-backup: prune old image %s for %q: %v (kept)", shortID(in.Image), name, rErr) //nolint:gosec // G706: name is %q-quoted
		}
	}

	// #56: notify per updated container (opt-in), so the user can verify it still
	// works. Fires per container (updates are rare), NOT folded into the scheduled
	// summary. Healthchecks is suppressed so an update can't flip the domain monitor;
	// the message ctx carries no message-suppress flag, so it delivers in summary mode.
	if c, cErr := s.NotifyConfig(); cErr == nil && c.NotifyOnUpdate {
		msg := fmt.Sprintf("Updated container %q to a newer image. Please verify it still works.", name)
		notify.Send(notify.WithHealthchecksSuppressed(context.Background()), c, "containers",
			notify.Event{Title: "BombVault", Message: msg, OK: true})
		if s.unraidGate(c.Unraid) && c.On == "always" {
			if e := s.sendUnraidNotify(ctx, "BombVault: container updated", msg, "normal"); e != nil {
				log.Printf("notify: unraid: %v", e)
			}
		}
	}
}

// recreateForUpdate stops, removes and recreates+starts the container from its
// captured inspect — which, after the preceding Pull, resolves to the newer
// image. Mirrors the restore recreate path minus the appdata restore (the data
// is already current). Note: containers that share this one's network namespace
// (network_mode: container:<name>) may need a manual restart afterwards.
func (s *Service) recreateForUpdate(ctx context.Context, name string, in model.Inspect) error {
	if err := s.docker.Stop(ctx, name, 30*time.Second); err != nil {
		_ = err // absent/already-stopped is fine; a real problem surfaces at Remove
	}
	if err := s.docker.Remove(ctx, name); err != nil {
		return fmt.Errorf("remove container: %w", err)
	}
	if err := s.docker.CreateAndStart(ctx, in, true); err != nil {
		return fmt.Errorf("recreate container: %w", err)
	}
	return nil
}

// reconcileUnraidUpdateStatus asks the platform to refresh its own cached
// update status for a container BombVault just recreated (#116), so (on
// Unraid) the Docker tab's stale "update available" banner clears. The actual
// host-side step lives behind s.platformFn() (see
// platform.Platform.ReconcileContainerUpdateStatus — Unraid's PHP-over-SSH
// recheck; a no-op on any platform without an equivalent). This wrapper is
// the non-fatal/best-effort boundary, unchanged from before the Platform
// seam: a nil SSH link, a non-Unraid platform, or a command error is only
// logged and never affects the backup or update outcome.
func (s *Service) reconcileUnraidUpdateStatus(ctx context.Context, ref string) {
	if err := s.platformFn().ReconcileContainerUpdateStatus(ctx, s.ssh, ref); err != nil {
		log.Printf("api: update-after-backup: unraid update-status reconcile for %q failed (harmless): %v", ref, err) //nolint:gosec // G706: ref is %q-quoted
	}
}

// setUpdateCheck stamps the outcome of a completed post-backup update check on
// the container's target row (last_update_check/last_update_result), so the
// Containers page can show "checked, up to date / updated / check failed"
// without a per-night run row. Best-effort: a store error is only logged.
func (s *Service) setUpdateCheck(name, result string) {
	if err := s.store.SetUpdateCheck(name, time.Now().Unix(), result); err != nil {
		log.Printf("api: update-after-backup: %q could not record update check: %v", name, err) //nolint:gosec // G706: name is %q-quoted
	}
}

// recordUpdateFailure records a FAILED "update" run so a reached-but-failed
// post-backup update — a registry check that failed before we could tell whether a
// newer image exists — is visible in Run History and the Activity Log rather than
// only the server log (#95). It also stamps the target's update-check outcome as
// 'failed'. Best-effort: the backup already succeeded, so a bookkeeping error
// here is only logged. (An update that WAS available but failed to apply is
// recorded separately by the recreate path.)
func (s *Service) recordUpdateFailure(name, targetID string, cause error) {
	s.setUpdateCheck(name, "failed")
	// No ctx reaches this function (recordUpdateFailure takes none) — this "update"
	// kind run is never part of a "Backup Everything" pass's group-stamped children
	// (see runGroupKey's doc comment), so context.Background() is a genuine no-op,
	// not a stand-in for a real context that was dropped.
	runID, rErr := runsAdapter{st: s.store, ctx: context.Background()}.Start(targetID, "update")
	if rErr != nil {
		log.Printf("api: update-after-backup: %q could not record update failure: %v (cause: %v)", name, rErr, cause) //nolint:gosec // G706: name is %q-quoted
		return
	}
	_ = runsAdapter{st: s.store, ctx: context.Background()}.Finish(runID, "failed", "", 0, truncateRunErr(cause))
}

// StartBackupAll launches a server-side batch backup of the named containers,
// running them sequentially in a background goroutine. This is the robust path
// for "back up all selected": it runs ON THE SERVER, so it survives the browser
// that started it going away (closing the tab, or — the case that bit a user —
// stopping the very container the BombVault UI is open in). Self and blank names
// are skipped, and a single container failing is logged and the batch continues.
//
// It returns (false, nil) if a batch is already running (the caller answers 409),
// or (false, err) if the containers domain is already busy with another op
// (scheduled backup/restore/prune/…) — a clear busy error the handler maps to a
// 409, instead of launching a goroutine that then blocks silently on the lock.
// Progress is published under "batch:containers" for an overall indicator, while
// each container still publishes its own "container:<name>" bar as it runs.
//
// Unlike a SCHEDULED domain run (which aggregates its Healthchecks pings into one
// per run, #49), this manual multi-select keeps the per-item Healthchecks ping: it
// backs up an arbitrary user-chosen SUBSET, not the whole domain, so pinging the
// domain check "success" here would reset the scheduled-cadence monitor and could
// mask a genuinely overdue scheduled backup. Each item therefore pings as normal.
func (s *Service) StartBackupAll(ctx context.Context, names []string) (bool, error) {
	if !s.batchActive.CompareAndSwap(false, true) {
		return false, nil
	}
	if op, busy := s.domainBusy("containers"); busy {
		s.batchActive.Store(false)
		return false, fmt.Errorf("%s is running on containers", op)
	}
	// Detach immediately so the run — and the self-detection it depends on — is
	// independent of the request that started it (which is canceled the moment the
	// handler returns). Each per-container Backup applies its own hard timeout, so
	// the batch needs no deadline of its own; WithoutCancel keeps request values
	// without a cancel func to leak.
	// #95: suppress each container's inline off-site replication; the whole batch is
	// replicated ONCE after the loop (ReplicateOffsiteAfterBulk below), mirroring the
	// scheduled path instead of re-opening the off-site repo per container.
	bctx := WithBulkReplicateSuppressed(context.WithoutCancel(ctx))
	go func() {
		// Batch-level safety net: a panic OUTSIDE the per-item loop (ordering,
		// publishBatch, the post-loop prune/replicate) has no single container to
		// blame, so there is nothing for onPanic to close out — just contain it.
		// Each ITEM inside the loop gets its own, more precise recovery below
		// (backupOneForBatch), so one bad container can't abort the rest of the
		// batch — matching how a normal per-item error already only counts
		// against that one item.
		defer s.recoverOperation("backup-all", nil, nil)
		defer s.batchActive.Store(false)

		self := s.selfContainerName(bctx)
		queue := make([]string, 0, len(names))
		for _, n := range names {
			if n != "" && n != self {
				queue = append(queue, n)
			}
		}
		// #119: sequence the batch by the user's explicit manual backup order first,
		// then most-overdue-first as the tiebreak — the same ordering a scheduled run
		// uses. A store error here must never abort the batch (do not regress), so the
		// original selection order is kept as the fallback.
		if ordered, err := s.store.OrderContainerNamesForRun(queue); err != nil {
			log.Printf("api: backup-all: order containers: %v (using selection order)", err)
		} else {
			queue = ordered
		}
		total := len(queue)
		const key = "batch:containers"
		s.publishBatch(key, 0, true)
		ok, fail, skipped := 0, 0, 0
		for i, n := range queue {
			if err := s.backupOneForBatch(bctx, n); err != nil {
				if errors.Is(err, backup.ErrContainerNotInstalled) {
					skipped++                                                                   // removed container: a skip (already recorded), not a batch failure (#57)
					log.Printf("api: backup-all: %q skipped — not installed (backups only)", n) //nolint:gosec // G706: n is %q-quoted
				} else {
					fail++
					log.Printf("api: backup-all: %q failed (continuing): %v", n, err) //nolint:gosec // G706: n is %q-quoted
				}
			} else {
				ok++
			}
			s.publishBatch(key, float64(i+1)/float64(total)*100, true)
		}
		s.publishBatch(key, 100, false)
		// Retention first: ONE local prune for the whole batch (each container's
		// forget ran WITHOUT --prune under the bulk flag), BEFORE the batched
		// off-site replication — fewer snapshots left to copy.
		s.PruneAfterBulk(bctx, "containers")
		// #95: one batched off-site replication after the whole manual batch (no-op
		// unless containers replicate on a blank/coupled schedule with an off-site
		// repo). The per-container inline copy was suppressed via the bulk flag on bctx.
		s.ReplicateOffsiteAfterBulk(bctx, "containers")
		log.Printf("api: backup-all done: %d ok, %d skipped, %d failed (of %d requested %d)", ok, skipped, fail, total, len(names))
	}()
	return true, nil
}

// backupOneForBatch backs up a single queued container on behalf of
// StartBackupAll, containing any panic to just this item (recoverOperation)
// so one container's crash counts as that one item failing — exactly like a
// normal error already does — instead of aborting every container still
// queued behind it. On a recovered panic it also closes out the run
// StartRun'd deep inside backup.BackupContainer as failed (failStuckRun):
// that call is a plain sequential call, not deferred, so it would otherwise
// never be reached and the run would sit "running" forever.
func (s *Service) backupOneForBatch(ctx context.Context, name string) (err error) {
	// recoverOperation must be deferred DIRECTLY (not wrapped in a `defer
	// func(){...}()` closure) — see its doc comment: recover() only works when
	// called directly by a deferred function, so &err is how it hands the
	// panic back to this function's named return instead of a normal return
	// value.
	defer s.recoverOperation("backup-all: "+name, &err, func(msg string) {
		if tg, tErr := s.store.GetTargetByContainer(name); tErr == nil {
			s.failStuckRun(tg.ID, msg)
		}
	})
	_, err = s.Backup(ctx, name)
	return err
}

// StartBackup launches a single container backup in a background goroutine and
// returns immediately. Like StartBackupAll, this is the robust path: the work
// runs ON THE SERVER, so it survives the browser that started it going away —
// including the case that bit a user, where backing up the reverse-proxy
// container BombVault's UI runs through severs the request connection while the
// backup is still in flight. The per-container "container:<name>" progress bar
// keeps reporting over SSE so the SPA can watch completion.
//
// It shares batchActive with StartBackupAll so a single backup and a batch can
// never overlap (the same repo lock would otherwise serialise them anyway).
// Returns (false, nil) if a backup/batch is already running (the caller answers
// busy), or (false, err) if the containers domain is already busy with another op.
func (s *Service) StartBackup(ctx context.Context, name string) (bool, error) {
	if !s.batchActive.CompareAndSwap(false, true) {
		return false, nil
	}
	if op, busy := s.domainBusy("containers"); busy {
		s.batchActive.Store(false)
		return false, fmt.Errorf("%s is running on containers", op)
	}
	// Detach so the run is independent of the request that started it (canceled
	// the moment the handler returns); Backup applies its own hard timeout.
	bctx := context.WithoutCancel(ctx)
	go func() {
		defer s.recoverOperation("backup: "+name, nil, func(msg string) {
			if tg, tErr := s.store.GetTargetByContainer(name); tErr == nil {
				s.failStuckRun(tg.ID, msg)
			}
		})
		defer s.batchActive.Store(false)
		if _, err := s.Backup(bctx, name); err != nil {
			if errors.Is(err, backup.ErrContainerNotInstalled) {
				log.Printf("api: backup: %q skipped — not installed (backups only)", name) //nolint:gosec // G706: name is %q-quoted
			} else {
				log.Printf("api: backup: %q failed: %v", name, err) //nolint:gosec // G706: name is %q-quoted
			}
		}
	}()
	return true, nil
}

// StartBackupVM launches a single VM backup in a background goroutine and
// returns immediately, mirroring StartBackup for the VM domain. Progress is
// published under "vm:<name>". Shares batchActive (no overlap with any other
// backup); returns (false, nil) if one is already running, or (false, err) if the
// vms domain is already busy with another op.
func (s *Service) StartBackupVM(ctx context.Context, name string) (bool, error) {
	if !s.batchActive.CompareAndSwap(false, true) {
		return false, nil
	}
	if op, busy := s.domainBusy("vms"); busy {
		s.batchActive.Store(false)
		return false, fmt.Errorf("%s is running on vms", op)
	}
	bctx := context.WithoutCancel(ctx)
	go func() {
		defer s.recoverOperation("backup vm: "+name, nil, func(msg string) {
			if tg, tErr := s.store.GetVMTargetByName(name); tErr == nil {
				s.failStuckRun(tg.ID, msg)
			}
		})
		defer s.batchActive.Store(false)
		if _, err := s.BackupVM(bctx, name); err != nil {
			log.Printf("api: backup vm: %q failed: %v", name, err) //nolint:gosec // G706: name is %q-quoted
		}
	}()
	return true, nil
}

// StartBackupFlash launches the singleton flash backup in a background goroutine
// and returns immediately, mirroring StartBackup. Progress is published under
// "flash". Shares batchActive; returns (false, nil) if a backup is already
// running, or (false, err) if the flash domain is already busy with another op.
func (s *Service) StartBackupFlash(ctx context.Context) (bool, error) {
	if !s.batchActive.CompareAndSwap(false, true) {
		return false, nil
	}
	if op, busy := s.domainBusy("flash"); busy {
		s.batchActive.Store(false)
		return false, fmt.Errorf("%s is running on flash", op)
	}
	bctx := context.WithoutCancel(ctx)
	go func() {
		defer s.recoverOperation("backup flash: "+store.FlashTargetID, nil, func(msg string) {
			s.failStuckRun(store.FlashTargetID, msg)
		})
		defer s.batchActive.Store(false)
		if _, err := s.BackupFlash(bctx); err != nil {
			log.Printf("api: backup flash failed: %v", err)
		}
	}()
	return true, nil
}

// StartBackupConfig launches the singleton config self-backup in a background
// goroutine and returns immediately, mirroring StartBackupFlash. Progress is
// published under "config". Shares batchActive; returns (false, nil) if a backup
// is already running, or (false, err) if the config domain is already busy with
// another op.
func (s *Service) StartBackupConfig(ctx context.Context) (bool, error) {
	if !s.batchActive.CompareAndSwap(false, true) {
		return false, nil
	}
	if op, busy := s.domainBusy("config"); busy {
		s.batchActive.Store(false)
		return false, fmt.Errorf("%s is running on config", op)
	}
	bctx := context.WithoutCancel(ctx)
	go func() {
		defer s.recoverOperation("backup config: "+store.ConfigTargetID, nil, func(msg string) {
			s.failStuckRun(store.ConfigTargetID, msg)
		})
		defer s.batchActive.Store(false)
		if _, err := s.BackupConfig(bctx); err != nil {
			log.Printf("api: backup config failed: %v", err)
		}
	}()
	return true, nil
}

// StartBackupFileSet launches a single file-set backup in a background
// goroutine and returns immediately, mirroring StartBackupVM for the files
// domain. id is the set's stable store id; progress is published under
// "files:<name>". Shares batchActive (no overlap with any other backup);
// returns (false, nil) if one is already running, or (false, err) if the files
// domain is already busy with another op.
func (s *Service) StartBackupFileSet(ctx context.Context, id string) (bool, error) {
	if !s.batchActive.CompareAndSwap(false, true) {
		return false, nil
	}
	if op, busy := s.domainBusy("files"); busy {
		s.batchActive.Store(false)
		return false, fmt.Errorf("%s is running on files", op)
	}
	bctx := context.WithoutCancel(ctx)
	go func() {
		defer s.recoverOperation("backup file set: "+id, nil, func(msg string) {
			s.failStuckRun(id, msg) // id IS the runs.target_id for a file set — no lookup needed
		})
		defer s.batchActive.Store(false)
		if _, err := s.BackupFileSet(bctx, id); err != nil {
			log.Printf("api: backup file set: %q failed: %v", id, err) //nolint:gosec // G706: id is %q-quoted
		}
	}()
	return true, nil
}

// StartBackupFilesAll launches sequential backups for the given file-set ids in
// one background batch and returns immediately, mirroring StartBackupAll for
// the files domain. Unlike the containers batch there is no self-container to
// skip — every non-empty id is attempted, failures logged and counted without
// aborting the rest. Overall progress is published under "batch:files" while
// each set still publishes its own "files:<name>" bar as it runs. Shares
// batchActive; returns (false, nil) if a backup/batch is already running, or
// (false, err) if the files domain is already busy with another op.
func (s *Service) StartBackupFilesAll(ctx context.Context, ids []string) (bool, error) {
	if !s.batchActive.CompareAndSwap(false, true) {
		return false, nil
	}
	if op, busy := s.domainBusy("files"); busy {
		s.batchActive.Store(false)
		return false, fmt.Errorf("%s is running on files", op)
	}
	// Detach immediately so the batch is independent of the request that started
	// it (canceled the moment the handler returns). Each per-set BackupFileSet
	// applies its own hard timeout, so the batch needs no deadline of its own.
	// #95: suppress each set's inline off-site replication; the whole batch is
	// replicated ONCE after the loop (ReplicateOffsiteAfterBulk below), mirroring
	// the containers batch instead of re-opening the off-site repo per set.
	bctx := WithBulkReplicateSuppressed(context.WithoutCancel(ctx))
	go func() {
		// See StartBackupAll's identical pair of defers for why: this one contains
		// a panic OUTSIDE the per-item loop, while each item inside the loop gets
		// its own more precise recovery (backupFileSetOneForBatch) so one bad set
		// can't abort the rest of the batch.
		defer s.recoverOperation("backup-files-all", nil, nil)
		defer s.batchActive.Store(false)

		queue := make([]string, 0, len(ids))
		for _, id := range ids {
			if id != "" {
				queue = append(queue, id)
			}
		}
		total := len(queue)
		const key = "batch:files"
		s.publishBatch(key, 0, true)
		ok, fail := 0, 0
		for i, id := range queue {
			if err := s.backupFileSetOneForBatch(bctx, id); err != nil {
				fail++
				log.Printf("api: backup-files-all: %q failed (continuing): %v", id, err) //nolint:gosec // G706: id is %q-quoted
			} else {
				ok++
			}
			s.publishBatch(key, float64(i+1)/float64(total)*100, true)
		}
		s.publishBatch(key, 100, false)
		// Retention first: ONE local prune for the whole batch (each set's forget
		// ran WITHOUT --prune under the bulk flag), BEFORE the batched off-site
		// replication — fewer snapshots left to copy.
		s.PruneAfterBulk(bctx, "files")
		// #95: one batched off-site replication after the whole manual batch (no-op
		// unless files replicate on a blank/coupled schedule with an off-site
		// repo). The per-set inline copy was suppressed via the bulk flag on bctx.
		s.ReplicateOffsiteAfterBulk(bctx, "files")
		log.Printf("api: backup-files-all done: %d ok, %d failed (of %d requested %d)", ok, fail, total, len(ids))
	}()
	return true, nil
}

// backupFileSetOneForBatch backs up a single queued file set on behalf of
// StartBackupFilesAll — see backupOneForBatch (the containers-batch
// counterpart) for why each item gets its own recovery instead of one shared
// at the batch level. id is already the runs.target_id (no lookup needed).
func (s *Service) backupFileSetOneForBatch(ctx context.Context, id string) (err error) {
	// See backupOneForBatch for why this must be a direct defer, not wrapped.
	defer s.recoverOperation("backup-files-all: "+id, &err, func(msg string) {
		s.failStuckRun(id, msg)
	})
	_, err = s.BackupFileSet(ctx, id)
	return err
}

// BackupInProgress reports whether a single backup, a batch, or a restore is
// currently running (they share the same single-flight guard). It lets callers
// — and tests — observe when the detached goroutine has fully finished.
func (s *Service) BackupInProgress() bool { return s.batchActive.Load() }

// recoverOperation is deferred FIRST — i.e. before every other defer — in
// every backup/restore/replication goroutine below (defers run LIFO, so being
// registered first means it is the LAST one to run: every other cleanup defer
// in the same goroutine, like releasing batchActive or unregistering a cancel
// key, already fired normally before this one ever sees the panic). It
// contains a panic to the ONE operation that raised it — logged here with a
// stack trace — instead of letting it reach the top of the goroutine
// unrecovered, which crashes the ENTIRE process: the HTTP server, the SSE
// progress stream, and every OTHER domain's concurrently in-flight
// backup/restore/replication along with it. This mirrors
// internal/schedule.Scheduler's cron.Recover, which already gives the
// byte-identical CRON-triggered path (the same svc.Backup/BackupVM/etc, wired
// in cmd/bombvault/main.go) this exact protection — see Scheduler.New's doc
// comment for why that exists.
//
// onPanic, when non-nil, is called ONLY on a recovered panic, with a short
// message describing it. Callers use it to close out whatever run record this
// operation would otherwise leave stuck "running" forever: unlike a full
// process crash, there is no restart here to trigger
// store.Repo.ReapInterruptedRuns, so nothing else will ever do it. It is
// never called on the overwhelmingly common non-panic path, so it is safe for
// it to do a store lookup that a happy-path caller wouldn't want to pay for.
//
// errOut, when non-nil, receives the panic converted to an error — for a
// caller with its own error result (e.g. a per-item batch helper that must
// keep the surrounding loop's existing "one bad item doesn't abort the rest"
// behaviour) to propagate like any other failure. This is an OUT PARAMETER,
// not a return value, and that is deliberate: recover() only has any effect
// when called directly by a deferred function (a nested call to it from
// inside a plain function that a defer merely INVOKES does not count, and
// silently fails to recover anything — a genuine Go gotcha, not a style
// preference). recoverOperation must therefore always be the direct target of
// `defer` — `defer s.recoverOperation(...)`, never wrapped in a `defer
// func(){ ... s.recoverOperation(...) ... }()` closure — and a plain return
// value would be unreachable from such a call. A caller with nothing to
// propagate to (every single-target Start* goroutine below) passes nil.
func (s *Service) recoverOperation(op string, errOut *error, onPanic func(msg string)) {
	r := recover()
	if r == nil {
		return
	}
	const stackSize = 64 << 10 // matches robfig/cron's own Recover chain wrapper
	buf := make([]byte, stackSize)
	buf = buf[:runtime.Stack(buf, false)]
	log.Printf("api: %s: panic recovered (operation aborted, process unaffected): %v\n%s", op, r, buf) //nolint:gosec // G706: op is a fixed literal, sometimes suffixed with a target name/id/domain already boundary-validated (validResourceName, validVMName, or a fixed switch of domain literals) before this goroutine ever started — never raw, unvalidated request input
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
// that only know the run's TARGET, not its specific run id — the deep
// orchestrator that called store.StartRun for it panicked before it could
// reach the matching store.FinishRun. Scoped to targetID so it can never
// disturb a different target's genuinely in-flight run (see
// FailRunningRun's own doc comment). msg is bounded through truncateRunErr —
// the same cap every other run-error path (finishRestoreRun, FinishRun
// callers) already applies — before being stored. Best-effort: a store error
// here is logged, never returned — the panic itself is already logged by the
// caller either way. A blank targetID (nothing resolved, or no target row
// exists yet) is a silent no-op — nothing was started, so nothing is stuck.
func (s *Service) failStuckRun(targetID, msg string) {
	if targetID == "" {
		return
	}
	if _, err := s.store.FailRunningRun(targetID, truncateRunErr(errors.New(msg))); err != nil {
		log.Printf("api: mark stuck run failed for target %q: %v", targetID, err) //nolint:gosec // G706: targetID is a store-generated id / fixed literal, %q-quoted
	}
}

// publishBatch emits an overall batch-progress event (no-op without a store).
func (s *Service) publishBatch(key string, percent float64, active bool) {
	if s.progress == nil {
		return
	}
	s.progress.Publish(progress.Event{Key: key, Phase: "backup", Percent: percent, Active: active})
}

// defsDir returns the directory INSIDE the containers repo (repo/def) where the
// encrypted container definitions are mirrored for disaster recovery. Keeping them
// inside the repo makes a copy of the repo folder self-contained — the DR
// definitions travel with it — and leaves the backup root uncluttered. "def" never
// collides with restic's own repo entries (config, data, index, keys, locks,
// snapshots), and restic ignores unknown subdirectories.
func (s *Service) defsDir(settings store.Settings) (string, error) {
	repo, err := s.containersRepoPath(settings)
	if err != nil {
		return "", err
	}
	return filepath.Join(repo, "def"), nil
}

// legacyDefsDir is the pre-v5.4.1 container defs location (a sibling of the repo).
// Still read as a fallback and migrated away by migrateLegacyDefs.
func (s *Service) legacyDefsDir(settings store.Settings) (string, error) {
	repo, err := s.containersRepoPath(settings)
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(repo), "bombvault-defs"), nil
}

// ensureDefsDir creates the disaster-recovery defs directory and makes sure it is
// world-traversable (0755). It lives on the operator's backup storage — typically
// a network share the operator also copies off-box for a second copy — so a
// non-root SMB user must be able to read it; the .def files inside are ALWAYS
// APP_KEY-encrypted, so the looser mode exposes nothing (the restic repo beside it
// is likewise readable). Chmod (not just MkdirAll) heals a directory an older
// version created at 0700, which locked SMB users out of the whole backup folder.
func ensureDefsDir(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil { //nolint:gosec // G301: backup share must be readable by the off-server sync tool; .def contents are encrypted
		return err
	}
	if err := os.Chmod(dir, 0o755); err != nil { //nolint:gosec // G302: see above — must be sync-readable; contents are encrypted
		return err
	}
	return nil
}

// writeDef writes an encrypted definition into the defs dir readable (0644) by the
// off-server sync tool that copies the backup share. os.WriteFile keeps the mode of
// an EXISTING file, so an explicit Chmod is required to heal a .def an older version
// wrote at 0600 (and to defeat a strict process umask); the contents are always
// APP_KEY-encrypted, so 0644 exposes nothing.
func writeDef(dir, fn string, enc []byte) error {
	final := filepath.Join(dir, fn)
	tmp := final + ".tmp"
	if err := os.WriteFile(tmp, enc, 0o644); err != nil { //nolint:gosec // G306: encrypted contents; backup share must be sync-readable. fn validated by defFileName; dir is operator-configured
		return err
	}
	// os.WriteFile keeps an EXISTING file's mode, so force 0644 (and defeat a strict
	// umask) to heal a leftover tmp and guarantee the sync tool can read it.
	if cErr := os.Chmod(tmp, 0o644); cErr != nil { //nolint:gosec // G302: see above
		_ = os.Remove(tmp) //nolint:gosec // G703: tmp = final+".tmp"; final = Join(dir, fn); fn validated by defFileName, dir operator-configured
		return cErr
	}
	// Atomic swap so a reader — or migrateLegacyDefs, which deletes the legacy source
	// once the destination exists — can never observe a half-written def as complete
	// (both paths sit on the same backup storage, so os.Rename is atomic).
	if rErr := os.Rename(tmp, final); rErr != nil { //nolint:gosec // G703: tmp/final derived from Join(dir, fn); fn validated by defFileName, dir operator-configured
		_ = os.Remove(tmp) //nolint:gosec // G703: see above
		return rErr
	}
	return nil
}

// makeRepoReadable relaxes a LOCAL restic repo tree so the operator can copy it
// off-box (e.g. a second drive over SMB) as a non-root user. restic, run as root,
// writes the repo 0700/0600, which locks a non-root sync tool out of the whole
// folder; the repo is encrypted, so adding group/other READ (and dir traverse)
// exposes nothing. Only entries actually missing the bits are chmod'd, so a repeat
// walk issues almost no chmod syscalls; the directory walk itself still runs after
// every backup, but its cost is negligible next to the backup it follows. It runs
// per-container backup (the single choke point for every code path) rather than
// once per batch, trading a redundant walk for simplicity and total coverage.
// Best-effort: a walk/chmod error must never fail a good backup, and a non-local
// repo path (an off-site rclone remote) simply yields a walk error and is skipped.
func makeRepoReadable(repo string) {
	_ = filepath.WalkDir(repo, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil //nolint:nilerr // best-effort; a walk error must not fail the backup
		}
		info, ierr := d.Info()
		if ierr != nil {
			return nil
		}
		perm := info.Mode().Perm()
		want := perm | 0o044 // group+other read
		if d.IsDir() {
			want |= 0o011 // group+other traverse
		}
		if want != perm {
			// Perm() drops setuid/setgid/sticky; re-add them so a group-inheritance
			// (setgid) dir on a shared NAS keeps its special bit through the chmod.
			special := info.Mode() & (os.ModeSetuid | os.ModeSetgid | os.ModeSticky)
			_ = os.Chmod(p, want|special) //nolint:gosec // G302: encrypted repo; must be readable by the operator's off-box sync tool
		}
		return nil
	})
}

// makeOffsiteRepoReadable is makeRepoReadable for an off-site DESTINATION repo:
// the same relax pass every backup already runs on the primary repo, applied to
// the replica so the far side of a mounted-share destination is readable by the
// share's non-root clients too.
//
// Why the destination needs it at all: restic derives its modes from the repo's
// existing `data` directory and otherwise defaults to 0700 dirs / 0400 files, so
// an off-site repo BombVault creates (EnsureRepo → paths.EnsureDir, 0700) stays
// root-only forever — every pack, index and snapshot file included. On a local
// repo that never showed, because the operator reads it through the same root
// process; on a share it decides whether ANY other client can open the replica.
// Verified against restic 0.17.3: `init` into a 0755 directory still creates
// data/ and keys/ at 0700 and files at 0400, so relaxing the repo root alone is
// not enough — the tree has to be walked. Once the walk has run, restic derives
// group-readable modes for later writes, but still not other-readable ones, so
// this must run after EVERY replication, exactly like the primary repo's pass.
//
// Remote backends have no local tree to chmod and are skipped outright (a bare
// WalkDir over "rest:http://…" would merely fail, but the guard says so). The
// repo is encrypted, so group/other READ exposes nothing — the same reasoning
// makeRepoReadable already documents. Best-effort throughout.
func makeOffsiteRepoReadable(dest string) {
	if restic.IsRemoteRepo(dest) {
		return
	}
	makeRepoReadable(dest)
}

// readStoredDef reads an encrypted definition, preferring the new in-repo location
// (repo/def) and falling back to the pre-v5.4.1 sibling location so a restore from
// a backup made before the move still finds its definitions.
func readStoredDef(newDir, legacyDir, fn string) ([]byte, error) {
	enc, err := os.ReadFile(filepath.Join(newDir, fn)) //nolint:gosec // G304: fn validated by defFileName; dirs are operator-configured
	if err == nil {
		return enc, nil
	}
	return os.ReadFile(filepath.Join(legacyDir, fn)) //nolint:gosec // G304: fn validated by defFileName; dirs are operator-configured
}

// migrateLegacyDefs best-effort moves any pre-v5.4.1 def files from the legacy
// sibling dir into the new in-repo dir, then removes the legacy dir once empty, so
// the backup root cleans itself up after the first backup following the upgrade.
// Never fails a backup; both dirs sit on the same backup storage so os.Rename works.
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

// writeDefToStorage encrypts the definition with the APP_KEY-derived key and
// writes it to <defsDir>/<name>.def (0644 — readable by the off-server sync tool
// that copies the backup share; the contents are always encrypted). The env vars
// inside the definition are sensitive, so the file is always encrypted regardless
// of the restic encryption setting.
func (s *Service) writeDefToStorage(settings store.Settings, name string, defJSON []byte) error {
	fn, err := defFileName(name)
	if err != nil {
		return err
	}
	dir, err := s.defsDir(settings)
	if err != nil {
		return err
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
	// Move any pre-v5.4.1 defs from the old sibling dir into the repo and remove
	// the old dir once empty (best-effort; a good backup never fails over this).
	if legacy, lErr := s.legacyDefsDir(settings); lErr == nil {
		migrateLegacyDefs(dir, legacy)
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

// Discover rebuilds BombVault's target list from the backup storage — used after
// a fresh install / loss of /config. It lists the containers repo's snapshots
// (tagged container:<name>), reads + decrypts each container's mirrored
// definition, and upserts a target so the container can be restored. Returns the
// number of containers discovered. Containers whose definition is missing or
// undecryptable are skipped (logged).
//
// dryRun makes it READ-ONLY: it opens the repo and decrypts the definitions
// (proving the repo is reachable and the APP_KEY is correct) and returns the
// same count, but writes NO targets. The Recovery tab's readability probe uses
// this so merely checking "is my backup readable?" never resurrects orphan
// entries; only the explicit "Discover backups" action rebuilds targets (#44).
func (s *Service) Discover(ctx context.Context, dryRun bool) (int, error) {
	settings, err := s.store.GetSettings()
	if err != nil {
		return 0, fmt.Errorf("read settings: %w", err)
	}
	repo, err := s.containersRepoPath(settings)
	if err != nil {
		return 0, err
	}
	mode := s.ModeFor(settings)
	// No local repo yet → nothing to discover (not an error). Discover always
	// targets the primary (local) repo, so the local config check is correct here;
	// keeping it preserves the quiet "0 discovered" for a not-yet-created repo.
	if _, statErr := os.Stat(filepath.Join(repo, "config")); errors.Is(statErr, fs.ErrNotExist) { //nolint:gosec // G703: repo is the operator-configured local domain path, validated under the mount root on save
		return 0, nil
	}
	snaps, err := s.engine.Snapshots(ctx, repo, mode)
	if err != nil {
		return 0, err
	}

	// Collect the distinct container names from the container:<name> tags.
	names := map[string]bool{}
	for _, snap := range snaps {
		for _, tag := range snap.Tags {
			if rest, ok := strings.CutPrefix(tag, "container:"); ok && rest != "" {
				names[rest] = true
			}
		}
	}

	dir, err := s.defsDir(settings)
	if err != nil {
		return 0, err
	}
	legacyDir, err := s.legacyDefsDir(settings)
	if err != nil {
		return 0, err
	}
	discovered := 0
	for name := range names {
		fn, fnErr := defFileName(name)
		if fnErr != nil {
			log.Printf("api: discover: skipping unsafe container name %q: %v", name, fnErr) //nolint:gosec // G706: %q-quoted
			continue
		}
		enc, rErr := readStoredDef(dir, legacyDir, fn)
		if rErr != nil {
			log.Printf("api: discover: no stored definition for %q — skipping (cannot recreate): %v", name, rErr) //nolint:gosec // G706: %q-quoted
			continue
		}
		plain, dErr := secret.Decrypt(s.cfg.AppKey, enc)
		if dErr != nil {
			log.Printf("api: discover: definition for %q is undecryptable (wrong APP_KEY?) — skipping: %v", name, dErr) //nolint:gosec // G706: %q-quoted
			continue
		}
		var def containerDefinition
		if jErr := json.Unmarshal(plain, &def); jErr != nil {
			log.Printf("api: discover: definition for %q is corrupt — skipping: %v", name, jErr) //nolint:gosec // G706: %q-quoted
			continue
		}
		if !dryRun {
			if _, uErr := s.store.UpsertTarget(store.Target{
				ContainerName: name,
				AppdataPaths:  def.AppdataPaths,
				Definition:    string(plain),
			}); uErr != nil {
				log.Printf("api: discover: could not upsert target %q: %v", name, uErr) //nolint:gosec // G706: %q-quoted
				continue
			}
		}
		discovered++
	}
	return discovered, nil
}

// vmDefsDir returns the directory INSIDE the vms repo (repo/vm-def) where the
// encrypted VM definitions are mirrored for disaster recovery — a sibling-free
// layout so the backup root stays clean and a repo-folder copy is self-contained.
// The dir is "vm-def" (not "def") so that even if the operator points the
// containers and vms repos at the SAME folder, a same-named container and VM never
// collide (mirrors the old bombvault-defs / bombvault-vm-defs distinction).
func (s *Service) vmDefsDir(settings store.Settings) (string, error) {
	repo, err := s.vmsRepoPath(settings)
	if err != nil {
		return "", err
	}
	return filepath.Join(repo, "vm-def"), nil
}

// legacyVMDefsDir is the pre-v5.4.1 VM defs location (a sibling of the vms repo).
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
// encrypted regardless of the restic encryption setting.
func (s *Service) writeVMDefToStorage(settings store.Settings, name string, defJSON []byte) error {
	fn, err := defFileName(name)
	if err != nil {
		return err
	}
	dir, err := s.vmDefsDir(settings)
	if err != nil {
		return err
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
	if legacy, lErr := s.legacyVMDefsDir(settings); lErr == nil {
		migrateLegacyDefs(dir, legacy)
	}
	return nil
}

// DiscoverVMs rebuilds the VM target list from backup storage — the VM
// counterpart of Discover, used after a fresh install / database loss so a VM
// that was deleted from the host (or whose target is gone) becomes restorable
// again. It lists the vms repo's snapshots (tagged vm:<name>), reads + decrypts
// each VM's mirrored definition, and upserts a target. VMs whose definition is
// missing (backed up before mirroring existed) or undecryptable are skipped.
// Returns the number of VMs discovered. dryRun makes it READ-ONLY (open + decrypt
// to prove readability + APP_KEY, return the count, but write no targets) — used
// by the Recovery readability probe so it never resurrects orphan VM entries (#44).
func (s *Service) DiscoverVMs(ctx context.Context, dryRun bool) (int, error) {
	settings, repo, err := s.domainRepo("vms")
	if err != nil {
		return 0, err
	}
	// Discover targets the primary (local) repo; the local config check is correct
	// here and preserves the quiet "0 discovered" for a not-yet-created repo.
	if _, statErr := os.Stat(filepath.Join(repo, "config")); errors.Is(statErr, fs.ErrNotExist) { //nolint:gosec // G703: repo is the operator-configured local domain path, validated under the mount root on save
		return 0, nil // no repo yet → nothing to discover
	}
	mode := s.ModeFor(settings)
	snaps, err := s.listSnapshots(ctx, repo, mode)
	if err != nil {
		return 0, err
	}

	names := map[string]bool{}
	for _, snap := range snaps {
		for _, tag := range snap.Tags {
			if rest, ok := strings.CutPrefix(tag, "vm:"); ok && rest != "" {
				names[rest] = true
			}
		}
	}

	dir, err := s.vmDefsDir(settings)
	if err != nil {
		return 0, err
	}
	legacyDir, err := s.legacyVMDefsDir(settings)
	if err != nil {
		return 0, err
	}
	discovered := 0
	for name := range names {
		fn, fnErr := defFileName(name)
		if fnErr != nil {
			log.Printf("api: discover vms: skipping unsafe name %q: %v", name, fnErr) //nolint:gosec // G706: %q-quoted
			continue
		}
		enc, rErr := readStoredDef(dir, legacyDir, fn)
		if rErr != nil {
			log.Printf("api: discover vms: no stored definition for %q — skipping (cannot recreate): %v", name, rErr) //nolint:gosec // G706: %q-quoted
			continue
		}
		plain, dErr := secret.Decrypt(s.cfg.AppKey, enc)
		if dErr != nil {
			log.Printf("api: discover vms: definition for %q is undecryptable (wrong APP_KEY?) — skipping: %v", name, dErr) //nolint:gosec // G706: %q-quoted
			continue
		}
		var def vmDefinition
		if jErr := json.Unmarshal(plain, &def); jErr != nil {
			log.Printf("api: discover vms: definition for %q is corrupt — skipping: %v", name, jErr) //nolint:gosec // G706: %q-quoted
			continue
		}
		method := def.Method
		if method == "" {
			method = "graceful"
		}
		if !dryRun {
			if _, uErr := s.store.UpsertVMTarget(store.VMTarget{
				Name:       name,
				Method:     method,
				Definition: string(plain),
			}); uErr != nil {
				log.Printf("api: discover vms: could not upsert target %q: %v", name, uErr) //nolint:gosec // G706: %q-quoted
				continue
			}
		}
		discovered++
	}
	return discovered, nil
}

// containerRestorePlan carries everything prepareRestore validated and resolved
// so the long-running execution can run detached from the request that asked
// for it (StartRestore) while the sync Restore path keeps identical behaviour.
type containerRestorePlan struct {
	repo         string
	mode         restic.Mode
	targetID     string
	snapshotID   string
	recreateOnly bool
	appdataPaths []string            // restored per-path back to origin (nil = recreate-only)
	restoreDirs  []backup.RestoreDir // cross-pool remap: Subtree->Target; empty = in-place via appdataPaths
	inspect      model.Inspect
	templateXML  string
}

// Restore runs a full container restore. The recreate profile is taken from the
// persisted definition (stored at backup time) so restore works even after the
// container has been deleted. For old targets without a stored definition the
// live inspect is used as a fallback; if that also fails a clear error is
// returned prompting the user to run one backup first.
func (s *Service) Restore(ctx context.Context, name, snapshotID string, confirm bool, source string, leaveStopped bool) error {
	plan, err := s.prepareRestore(ctx, name, snapshotID, confirm, source)
	if err != nil {
		return err
	}
	return s.executeRestore(ctx, name, plan, leaveStopped)
}

// repoRef identifies an already-resolved restic repository together with the
// mode (encryption + backend credentials) needed to open it. The
// settings-driven paths build one via repoFor/ModeFor; a caller restoring from
// a repo that is NOT in Settings (e.g. another BombVault instance's repo) can
// build its own ref and reuse the same preparation logic.
type repoRef struct {
	repo string
	mode restic.Mode
}

// prepareRestore resolves the settings-configured containers repo (local or
// off-site) and delegates to prepareRestoreIn. The request guards run here
// FIRST — before the settings/repo resolution — so an unconfirmed or malformed
// request keeps failing with its own error (the sentinel, the name error, the
// snapshot-id error) rather than a resolution error; prepareRestoreIn
// re-validates them (defense-in-depth for non-settings callers).
func (s *Service) prepareRestore(ctx context.Context, name, snapshotID string, confirm bool, source string) (containerRestorePlan, error) {
	// Guard confirmation before touching the store/docker so an unconfirmed
	// restore surfaces the sentinel (and never errors on a missing target first).
	if !confirm {
		return containerRestorePlan{}, backup.ErrNotConfirmed
	}
	if !validResourceName(name) {
		return containerRestorePlan{}, errors.New("invalid container name")
	}
	if snapshotID != "latest" && snapshotID != "" && !backup.ValidSnapshotID(snapshotID) {
		return containerRestorePlan{}, backup.ErrInvalidSnapshotID
	}
	settings, err := s.store.GetSettings()
	if err != nil {
		return containerRestorePlan{}, fmt.Errorf("read settings: %w", err)
	}
	repo, err := s.repoFor(settings, "containers", source)
	if err != nil {
		return containerRestorePlan{}, err
	}
	return s.prepareRestoreIn(ctx, repoRef{repo: repo, mode: s.ModeFor(settings)}, name, snapshotID, confirm)
}

// prepareRestoreIn performs ALL of a container restore's validation and
// resolution synchronously against an explicit repository — confirmation,
// name/snapshot-id guards, snapshot ownership, path containment and the
// recreate-recipe lookup — so a bad request fails immediately with a clear
// error, BEFORE anything long-running (or destructive) starts. The returned
// plan is everything executeRestore needs.
func (s *Service) prepareRestoreIn(ctx context.Context, ref repoRef, name, snapshotID string, confirm bool) (containerRestorePlan, error) {
	if !confirm {
		return containerRestorePlan{}, backup.ErrNotConfirmed
	}
	// Re-validate the name at the service layer (defense-in-depth): the HTTP route
	// guards it via nameParam, but RestoreStack enumerates names from the store, so
	// the name-as-template-filename sink must be guarded here too, in case a
	// stored/imported name ever bypassed the boundary.
	if !validResourceName(name) {
		return containerRestorePlan{}, errors.New("invalid container name")
	}
	// An explicit snapshot id must be well-formed hex. The orchestrator re-checks
	// this, but guarding here makes a bad id fail synchronously (fail-fast for the
	// async StartRestore path). "latest"/"" resolve below.
	explicitID := snapshotID != "latest" && snapshotID != ""
	if explicitID && !backup.ValidSnapshotID(snapshotID) {
		return containerRestorePlan{}, backup.ErrInvalidSnapshotID
	}

	tg, err := s.store.GetTargetByContainer(name)
	if err != nil {
		log.Printf("api: restore: unknown target %q: %v", name, err) //nolint:gosec // G706: name is %q-quoted; no raw user bytes reach the log formatter
		return containerRestorePlan{}, errors.New("container has not been backed up yet")
	}
	// Same-instance restore: no destBase (in-place), no overwrite prompt — the
	// cross-pool remap path is foreign-only (#125).
	return s.prepareRestoreForTarget(ctx, ref, name, snapshotID, tg, "", false)
}

// prepareRestoreForTarget builds a container restore plan for an ALREADY-RESOLVED
// target tg against an explicit repo ref, WITHOUT reading or writing the store.
// prepareRestoreIn passes the stored target; the foreign restore passes a target
// built from the decrypted foreign definition, so snapshot ownership and appdata
// containment are validated BEFORE that foreign recipe is ever persisted locally
// (prepareForeignRestore adopts it only once this returns a plan — never on a
// validation failure, which would otherwise clobber a same-named local target).
// The caller runs the confirm / name / explicit-snapshot-id-shape guards first.
func (s *Service) prepareRestoreForTarget(ctx context.Context, ref repoRef, name, snapshotID string, tg store.Target, destBase string, overwrite bool) (containerRestorePlan, error) {
	explicitID := snapshotID != "latest" && snapshotID != ""

	// "latest" (or empty) resolves to the container's newest snapshot — used by
	// the bulk "restore selected" action. restic returns snapshots oldest-first,
	// so the last tag-matching one is the newest.
	// A definition-only backup (stateless container with no restic snapshot) has
	// no snapshot to resolve — recreate it from the stored definition instead.
	// An explicit id must belong to THIS container (tag-scoped, the same
	// access-control check the file/to-path restores use) — listed against the
	// caller's repo ref, NOT the settings repo.
	recreateOnly := false
	snaps, snapErr := s.snapshotsForTag(ctx, ref.repo, ref.mode, "container:"+name)
	if snapErr != nil {
		return containerRestorePlan{}, snapErr
	}
	if explicitID {
		if !snapshotBelongs(snaps, snapshotID) {
			return containerRestorePlan{}, fmt.Errorf("snapshot %s does not belong to this container", snapshotID)
		}
	} else {
		switch {
		case len(snaps) > 0:
			snapshotID = snaps[len(snaps)-1].ID
		case tg.Definition != "":
			recreateOnly = true
		default:
			return containerRestorePlan{}, errors.New("no backups found for this container")
		}
	}

	// Re-validate the stored appdata paths stay within the host mount root before
	// restoring (defense-in-depth in case the DB was tampered with). Skipped for a
	// recreate-only restore, which has no paths.
	appdataForRestore := tg.AppdataPaths
	var restoreDirs []backup.RestoreDir
	var bindRemap map[string]string
	if recreateOnly {
		appdataForRestore = nil
	} else {
		if len(tg.AppdataPaths) == 0 {
			return containerRestorePlan{}, errors.New("no backup paths recorded for this container: run a backup once, then restore")
		}
		for _, p := range tg.AppdataPaths {
			if !paths.Within(s.cfg.HostMountRoot, p) {
				log.Printf("api: restore: appdata path %q escapes mount root", p) //nolint:gosec // G706: %q-quoted
				return containerRestorePlan{}, errors.New("a stored backup path is outside the host mount, so refusing to restore")
			}
		}
		// Cross-instance / cross-pool remap (destBase set: foreign restore, #123/#125).
		// A foreign recipe carries the SOURCE host's absolute appdata paths; if this
		// host lacks that pool the in-place write would land in an unmounted dir under
		// the host mount (the array/RAM rootfs), silently writing appdata to the wrong
		// place or bricking the host (#122 class for containers). Instead restore every
		// appdata path's CONTENTS into its container-relative place under destBase on
		// THIS host (containerAppdataRemap — a multi-bind container keeps the folder its
		// binds share), GUARD those destination dirs, and point the recreated binds +
		// template there. A standard container whose appdata already lives under
		// destBase remaps to the same path (a no-op). destBase == "" is the
		// same-instance in-place restore, unchanged.
		if destBase != "" {
			if !paths.Within(s.cfg.HostMountRoot, destBase) {
				return containerRestorePlan{}, errors.New("restore destination is outside the host mount")
			}
			restoreDirs, bindRemap = s.containerAppdataRemap(destBase, appdataForRestore)
			destDirs := make([]string, len(restoreDirs))
			for i, d := range restoreDirs {
				destDirs[i] = d.Target
			}
			// Host-brick guard on the DESTINATION dirs (not the source), BEFORE
			// executeRestore's destructive Stop/Remove: prove each target is on a real
			// mounted pool so a cross-pool restore can never fill the RAM rootfs.
			if err := s.guardContainerRestoreDestination(ctx, ref, snapshotID, destDirs); err != nil {
				return containerRestorePlan{}, err
			}
			// Overwrite guard: a target that is NOT this container's own source path and
			// already holds data is likely a DIFFERENT container's appdata — refuse
			// unless the caller explicitly confirmed the overwrite.
			if !overwrite {
				for _, d := range restoreDirs {
					if d.Target != d.Subtree && s.dirNonEmptyFn()(d.Target) {
						return containerRestorePlan{}, destinationRefusal("restore destination %q already contains data — it may belong to a different container; confirm overwrite to proceed", s.toHostPath(d.Target))
					}
				}
			}
		}
	}

	// Resolve recreate recipe: prefer the stored definition (works for deleted
	// containers), fall back to live inspect (for old targets without a stored
	// definition), fail with a clear message if both are unavailable.
	var in model.Inspect
	var xml string
	if tg.Definition != "" {
		var def containerDefinition
		if jsonErr := json.Unmarshal([]byte(tg.Definition), &def); jsonErr != nil {
			return containerRestorePlan{}, fmt.Errorf("restore: unmarshal stored definition: %w", jsonErr)
		}
		in = def.Inspect
		xml = def.TemplateXML
	} else {
		// Fallback: target was backed up before this feature; try live inspect.
		liveIn, liveErr := s.docker.Inspect(ctx, name)
		if liveErr != nil {
			return containerRestorePlan{}, errors.New("no stored definition for this container: run a backup once after upgrading, then restore is possible even after deletion")
		}
		in = liveIn
		xml, _, _ = template.Read(s.cfg.FlashTemplatesDir, name)
	}

	// Cross-pool remap: point the recreated container's binds + flashed template at
	// the appdata's new location. ONLY appdata binds are rewritten (exact host-path
	// match); docker.sock, /etc/localtime, /dev/dri and every non-appdata bind are
	// left verbatim (they carry no backed-up data). See #125.
	if len(bindRemap) > 0 {
		in.HostConfig.Binds = rewriteBinds(in.HostConfig.Binds, bindRemap)
		in.Mounts = rewriteMountSources(in.Mounts, bindRemap)
		xml = template.RewriteHostPaths(xml, bindRemap)
	}

	return containerRestorePlan{
		repo:         ref.repo,
		mode:         ref.mode,
		targetID:     tg.ID,
		snapshotID:   snapshotID,
		recreateOnly: recreateOnly,
		appdataPaths: appdataForRestore,
		restoreDirs:  restoreDirs,
		inspect:      in,
		templateXML:  xml,
	}, nil
}

// executeRestore drives the long-running (destructive) part of a container
// restore described by an already-validated plan, publishing "container:<name>"
// progress. The orchestrator records the run (kindRestore) itself.
func (s *Service) executeRestore(ctx context.Context, name string, plan containerRestorePlan, leaveStopped bool) error {
	// Hold the domain repo lock for the whole restic/docker phase, INCLUDING the
	// destination pre-create below. The scheduler calls Backup/BackupVM directly
	// and bypasses the batchActive single-flight guard BY DESIGN — the domain
	// lock is the one layer scheduled jobs do respect — so without it a
	// detached multi-hour restore could overlap a scheduled backup of the same
	// domain in both directions.
	unlock := s.lockDomainFor("containers", "restore")
	defer unlock()
	// Pre-create every remapped destination readable (0o755), same convention as
	// the file-set/to-path restores (see EnsureDirReadable). restic's own
	// restorer creates a fresh subtree TARGET at 0o700 and never revisits that
	// root directory's own metadata (only its restored CONTENTS get the
	// snapshot's ownership/mode) — on the cross-pool remap's brand-new
	// destination dirs that leaves them root:root/0700, unreadable to whatever
	// UID the container actually runs as. Pre-creating here means restic's own
	// MkdirAll finds the dir already present and leaves the mode alone. In-place
	// restores (empty RestoreDirs, or a Target that already existed before this
	// restore) are an inexpensive no-op: EnsureDirReadable only heals mode, never
	// touches ownership; the numeric owner:group is restored separately below,
	// after a successful restore, by healRestoreDirOwnership (see #125).
	for _, rd := range plan.restoreDirs {
		if err := paths.EnsureDirReadable(rd.Target); err != nil {
			return fmt.Errorf("restore: prepare destination %q: %w", s.toHostPath(rd.Target), err)
		}
	}
	rkey := "container:" + name
	rctx, startedAt := s.progBegin(ctx, rkey, "restore")
	rerr := backup.RestoreContainer(rctx, backup.RestoreDeps{
		Confirmed:         true, // prepareRestore rejected unconfirmed requests
		RecreateOnly:      plan.recreateOnly,
		ContainerRef:      name,
		ContainerName:     name,
		RepoPath:          plan.repo,
		SnapshotID:        plan.snapshotID,
		AppdataPaths:      plan.appdataPaths, // restored per-path back to origin (nil = recreate-only)
		RestoreDirs:       plan.restoreDirs,  // cross-pool remap (foreign restore); empty = in-place
		TemplateXML:       plan.templateXML,
		FlashTemplatesDir: s.cfg.FlashTemplatesDir,
		Inspect:           plan.inspect,
		LeaveStopped:      leaveStopped,
		TargetID:          plan.targetID,
		Docker:            s.docker,
		Restic:            &resticAdapter{engine: s.engine, mode: plan.mode},
		Templates:         templatesAdapter{},
		Runs:              runsAdapter{st: s.store, ctx: ctx},
	})
	if rerr == nil {
		s.healRestoreDirOwnership(rctx, plan.repo, plan.snapshotID, plan.mode, plan.restoreDirs)
	}
	s.progEnd(rkey, "restore", rerr == nil, startedAt)
	return rerr
}

// healRestoreDirOwnership best-effort restores each remapped restore
// directory's own root to the owner and mode it had in the snapshot. restic's
// restorer creates a fresh subtree TARGET itself but never re-applies that
// root directory's own metadata (only its restored CONTENTS get the
// snapshot's ownership/mode — see the EnsureDirReadable call above, which only
// fixes readability, not ownership). Read-then-heal, never fatal: the restore
// already succeeded by the time this runs, so a failure here (repo busy, a
// stale snapshot layout, an unexpected filesystem) must not turn a completed
// restore into a reported failure — it is logged and the next dir is tried.
func (s *Service) healRestoreDirOwnership(ctx context.Context, repo, snapshotID string, mode restic.Mode, dirs []backup.RestoreDir) {
	for _, rd := range dirs {
		entries, err := s.engine.LsPath(ctx, repo, snapshotID, rd.Subtree, mode)
		if err != nil {
			log.Printf("api: restore: reading original owner for %q: %v", s.toHostPath(rd.Target), err)
			continue
		}
		var found *restic.FileEntry
		for i := range entries {
			if entries[i].Path == rd.Subtree {
				found = &entries[i]
				break
			}
		}
		if found == nil {
			log.Printf("api: restore: no snapshot entry for %q, leaving owner as restored", s.toHostPath(rd.Target))
			continue
		}
		// Owner before mode: on some systems chown() clears setuid/setgid as a
		// security measure, so applying it first means a later chmod restores
		// the snapshot's real bits rather than having them silently stripped.
		if err := os.Lchown(rd.Target, found.Uid, found.Gid); err != nil {
			log.Printf("api: restore: restoring owner on %q: %v", s.toHostPath(rd.Target), err)
			continue
		}
		if err := os.Chmod(rd.Target, os.FileMode(found.Mode).Perm()); err != nil {
			log.Printf("api: restore: restoring mode on %q: %v", s.toHostPath(rd.Target), err)
		}
	}
}

// restoreTimeout is the hard cap on every detached restore goroutine
// (StartRestore/StartRestoreVM/StartRestoreFiles/StartRestoreToPath/
// StartRestoreStack). Aborting a restore mid-flight is DESTRUCTIVE — the
// container has already been removed and the appdata is partially written — so
// unlike the configurable backup cap (backupHardCap, default 48h) this one is
// deliberately generous: it exists only so a truly wedged restic can't hold the
// single-flight guard (and the domain lock) forever, never to bound a
// legitimate huge restore.
const restoreTimeout = 48 * time.Hour

// registerCancel records the CancelFunc of a running restore under its progress
// key so POST /api/restore/cancel can stop it. Called on launch; paired with a
// deferred unregisterCancel.
func (s *Service) registerCancel(key string, cancel context.CancelFunc) {
	s.cancelMu.Lock()
	if s.runCancels == nil {
		s.runCancels = map[string]context.CancelFunc{}
	}
	s.runCancels[key] = cancel
	s.cancelMu.Unlock()
}

// unregisterCancel drops a restore's cancel entry once it has finished, so a
// later cancel of the same key is a harmless no-op.
func (s *Service) unregisterCancel(key string) {
	s.cancelMu.Lock()
	delete(s.runCancels, key)
	s.cancelMu.Unlock()
}

// CancelRun cancels a running restore by its progress key and reports whether one
// was registered. Cancelling an unknown/already-finished key returns false and is
// a no-op (idempotent), so the endpoint can be called safely at any time.
func (s *Service) CancelRun(key string) bool {
	s.cancelMu.Lock()
	cancel, ok := s.runCancels[key]
	s.cancelMu.Unlock()
	if ok {
		cancel()
	}
	return ok
}

// StartRestore launches an in-place container restore in a background goroutine
// and returns immediately, mirroring StartBackup. This is the robust path for
// long restores: the work runs ON THE SERVER, detached from the request, so a
// multi-hour restore can't be killed by the browser/proxy dropping the idle
// HTTP connection (which cancels the request context and aborted restic
// mid-restore). ALL validation runs synchronously first, so a bad request still
// fails immediately with a clear error and no goroutine is started.
//
// It shares batchActive with the backup starters so a restore can never run
// concurrently with a backup or another restore (they contend on repo locks and
// container stop/start). Returns (false, nil) when one is already running.
func (s *Service) StartRestore(ctx context.Context, name, snapshotID, source string, leaveStopped bool) (bool, error) {
	if !s.batchActive.CompareAndSwap(false, true) {
		return false, nil
	}
	plan, err := s.prepareRestore(ctx, name, snapshotID, true, source)
	if err != nil {
		s.batchActive.Store(false)
		return false, err
	}
	// Detach so the run is independent of the request that started it (canceled
	// the moment the handler returns), capped by restoreTimeout (see its comment
	// for why the restore cap is far more generous than the backup one).
	bctx := context.WithoutCancel(ctx)
	key := "container:" + name // the exact progBegin key executeRestore publishes under
	go func() {
		defer s.recoverOperation("restore: "+name, nil, func(msg string) {
			s.failStuckRun(plan.targetID, msg)
		})
		defer s.batchActive.Store(false)
		tctx, tcancel := context.WithTimeout(bctx, restoreTimeout)
		defer tcancel()
		rctx, cancel := context.WithCancel(tctx)
		defer cancel()
		s.registerCancel(key, cancel)
		defer s.unregisterCancel(key)
		if rerr := s.executeRestore(rctx, name, plan, leaveStopped); rerr != nil {
			log.Printf("api: restore: %q failed: %v", name, rerr) //nolint:gosec // G706: name is %q-quoted
		}
	}()
	return true, nil
}

// Snapshots lists the snapshots for a single container. The containers repo is
// shared across all containers, so snapshots are filtered by the
// `container:<name>` tag the backup writes — otherwise the restore UI for one
// container would list (and could restore) another container's snapshots.
// LatestContainerBackupTimes returns, per container name, the unix time of its
// NEWEST local snapshot (read from the container:<name> tag). It gives an orphan
// row a real "last backup" date when its target was rebuilt by Discover and so
// has NO run record — which would otherwise read "Never" even though the
// container clearly still has backups in the repo (#44). One snapshot listing.
func (s *Service) LatestContainerBackupTimes(ctx context.Context) (map[string]int64, error) {
	settings, err := s.store.GetSettings()
	if err != nil {
		return nil, fmt.Errorf("read settings: %w", err)
	}
	repo, err := s.repoFor(settings, "containers", "local")
	if err != nil {
		return nil, err
	}
	if localRepoMissing(repo) {
		return nil, nil
	}
	all, err := s.listSnapshots(ctx, repo, s.ModeFor(settings))
	if err != nil {
		return nil, err
	}
	out := make(map[string]int64, len(all))
	for _, snap := range all {
		ts, perr := time.Parse(time.RFC3339Nano, snap.Time)
		if perr != nil {
			continue
		}
		unix := ts.Unix()
		for _, tag := range snap.Tags {
			if name, ok := strings.CutPrefix(tag, "container:"); ok && name != "" && unix > out[name] {
				out[name] = unix
			}
		}
	}
	return out, nil
}

func (s *Service) Snapshots(ctx context.Context, name, source string) ([]restic.Snapshot, error) {
	settings, err := s.store.GetSettings()
	if err != nil {
		return nil, fmt.Errorf("read settings: %w", err)
	}
	repo, err := s.repoFor(settings, "containers", source)
	if err != nil {
		return nil, err
	}
	return s.snapshotsForTag(ctx, repo, s.ModeFor(settings), "container:"+name)
}

// snapshotsForTag lists an EXPLICIT repo (no settings resolution) and returns
// only the snapshots carrying the given tag, oldest-first as restic reports
// them. A missing local repo is "no snapshots yet", not an error — the SPA
// shows an empty list, not a failure — unless the repo was established before
// (share not mounted, #55). Remote repos skip that local check (see
// localRepoMissing). The settings-driven Snapshots* wrappers delegate here;
// non-settings callers (the foreign-repo session) pass their own repoRef parts.
func (s *Service) snapshotsForTag(ctx context.Context, repo string, mode restic.Mode, tag string) ([]restic.Snapshot, error) {
	if localRepoMissing(repo) {
		// #55 vs #120: only surface "not mounted" when the backing store is truly
		// absent. If the destination IS mounted, this is a fresh/phantom repo on a
		// healthy disk, so report an empty list (EnsureRepo re-establishes on write).
		if s.repoEstablished(repo) && !s.destinationMounted(repo) {
			return nil, ErrBackupPathNotMounted // #55: backing store not mounted
		}
		return nil, nil
	}
	all, err := s.listSnapshots(ctx, repo, mode)
	if err != nil {
		return nil, err
	}
	out := make([]restic.Snapshot, 0, len(all))
	for _, snap := range all {
		for _, t := range snap.Tags {
			if t == tag {
				out = append(out, snap)
				break
			}
		}
	}
	return out, nil
}

// ListSnapshotFiles lists the files in a container snapshot, for file-level
// restore. snapshotID must be valid hex.
func (s *Service) ListSnapshotFiles(ctx context.Context, name, snapshotID, source string) ([]restic.FileEntry, error) {
	if !backup.ValidSnapshotID(snapshotID) {
		return nil, backup.ErrInvalidSnapshotID
	}
	// Scope to the named container: the snapshot must be one of ITS snapshots, so
	// one container's file tree can't be listed through another's route.
	snaps, err := s.Snapshots(ctx, name, source)
	if err != nil {
		return nil, err
	}
	found := false
	for _, sn := range snaps {
		if sn.ID == snapshotID || strings.HasPrefix(sn.ID, snapshotID) {
			found = true
			break
		}
	}
	if !found {
		return nil, fmt.Errorf("snapshot %s does not belong to this container", snapshotID)
	}
	settings, err := s.store.GetSettings()
	if err != nil {
		return nil, fmt.Errorf("read settings: %w", err)
	}
	repo, err := s.repoFor(settings, "containers", source)
	if err != nil {
		return nil, err
	}
	return s.lsSelfHeal(ctx, repo, snapshotID, s.ModeFor(settings))
}

// RestoreContainerFiles restores one or more files/dirs from a container
// snapshot. With targetSubPath empty the selected paths are written back to their
// ORIGINAL locations (in-place, restic target "/"); with a non-empty
// targetSubPath the selection is extracted into an ALTERNATE folder under the host
// mount (non-destructive, same containment as RestoreContainerToPath). It returns
// the resolved absolute target folder for the alternate-folder case, or "" for an
// in-place restore.
//
// SEC: confirm-gated; the snapshot id passes the strict hex guard
// (backup.ValidSnapshotID) and must belong to the named container (tag-scoped via
// Snapshots, like RestoreContainerToPath) so one container's data can't be
// extracted through another's route; every selected path is path.Cleaned and must
// sit within the host mount (paths.Within) — defense-in-depth so a restore can
// never read/write outside the backup mount; and the alternate target is resolved
// with paths.Resolve and created (EnsureDir) only after containment passes.
func (s *Service) RestoreContainerFiles(ctx context.Context, name, source, snapshotID string, filePaths []string, targetSubPath string, confirm bool) (string, error) {
	plan, err := s.prepareRestoreFiles(ctx, name, source, snapshotID, filePaths, targetSubPath, confirm)
	if err != nil {
		return "", err
	}
	if err := s.runRestoreFiles(ctx, plan); err != nil {
		return "", err
	}
	return plan.resolved, nil
}

// filesRestorePlan carries everything prepareRestoreFiles validated and
// resolved so the restic loop can run detached from the request that asked for
// it (StartRestoreFiles) while the sync path keeps identical behaviour.
type filesRestorePlan struct {
	repo       string
	mode       restic.Mode
	snapshotID string
	paths      []string // cleaned selection, containment-validated for in-place
	target     string   // restic --target: "/" = in place, else the resolved folder
	resolved   string   // the resolved alternate folder ("" = in-place)
}

// prepareRestoreFiles performs ALL of a file-level restore's validation and
// resolution synchronously (see the SEC notes on RestoreContainerFiles) — so a
// bad request fails immediately with a clear error — and creates the alternate
// target folder once containment passes.
func (s *Service) prepareRestoreFiles(ctx context.Context, name, source, snapshotID string, filePaths []string, targetSubPath string, confirm bool) (filesRestorePlan, error) {
	if !confirm {
		return filesRestorePlan{}, backup.ErrNotConfirmed
	}
	if !validResourceName(name) {
		return filesRestorePlan{}, errors.New("invalid container name")
	}
	if source != "local" && !isOffsiteSource(source) {
		return filesRestorePlan{}, errors.New("invalid source (must be local or offsite)")
	}
	if !backup.ValidSnapshotID(snapshotID) {
		return filesRestorePlan{}, backup.ErrInvalidSnapshotID
	}
	if len(filePaths) == 0 {
		return filesRestorePlan{}, errors.New("no files selected")
	}

	// Clean each selected path once, so the path we validate is the path we run.
	cleaned := make([]string, 0, len(filePaths))
	for _, p := range filePaths {
		cleaned = append(cleaned, path.Clean(p))
	}

	// Scope to the named container: the snapshot must be one of ITS snapshots
	// (same access-control check as RestoreContainerToPath).
	snaps, err := s.Snapshots(ctx, name, source)
	if err != nil {
		return filesRestorePlan{}, err
	}
	if !snapshotBelongs(snaps, snapshotID) {
		return filesRestorePlan{}, fmt.Errorf("snapshot %s does not belong to this container", snapshotID)
	}

	// Resolve the destination. Empty targetSubPath → in-place (restic target "/",
	// which writes each included path back to its absolute location). Otherwise
	// resolve the alternate folder under the host mount (shared containment helper)
	// and create it only after containment passes.
	target := "/"
	resolved := ""
	if sub := strings.TrimSpace(targetSubPath); sub != "" {
		t, err := paths.Resolve(s.cfg.HostMountRoot, sub)
		if err != nil {
			return filesRestorePlan{}, errors.New("invalid target folder: must be a relative subpath under the host mount")
		}
		if err := paths.EnsureDir(t); err != nil {
			return filesRestorePlan{}, fmt.Errorf("create target folder: %w", err)
		}
		target = t
		resolved = t
	} else {
		// In place writes each path back to its absolute location, so every path
		// must sit within the host mount (defense-in-depth). Validate all up front
		// so one bad entry fails the whole batch before anything is written. For an
		// alternate folder this is unnecessary: restic writes under --target, which
		// paths.Resolve already contained above.
		for _, c := range cleaned {
			if !paths.Within(s.cfg.HostMountRoot, c) {
				return filesRestorePlan{}, errors.New("restore file: path is outside the backup mount")
			}
		}
	}

	settings, err := s.store.GetSettings()
	if err != nil {
		return filesRestorePlan{}, fmt.Errorf("read settings: %w", err)
	}
	repo, err := s.repoFor(settings, "containers", source)
	if err != nil {
		return filesRestorePlan{}, err
	}
	return filesRestorePlan{
		repo:       repo,
		mode:       s.ModeFor(settings),
		snapshotID: snapshotID,
		paths:      cleaned,
		target:     target,
		resolved:   resolved,
	}, nil
}

// runRestoreFiles restores each selected path of an already-validated plan.
// This is intentionally not atomic — restic writes per path — so if one fails
// mid-batch, the error names how many already went through and which path
// stopped it, instead of a bare failure that hides that earlier paths were
// already restored.
func (s *Service) runRestoreFiles(ctx context.Context, plan filesRestorePlan) error {
	// Hold the domain repo lock for the restic work: scheduled backups bypass
	// batchActive by design and the domain lock is the layer they DO respect
	// (see executeRestore).
	unlock := s.lockDomainFor("containers", "restore")
	defer unlock()
	for i, c := range plan.paths {
		if err := s.engine.RestoreInclude(ctx, plan.repo, plan.snapshotID, c, plan.target, plan.mode); err != nil {
			if len(plan.paths) > 1 {
				return fmt.Errorf("restored %d of %d files, then failed on %q: %w", i, len(plan.paths), c, err)
			}
			return err
		}
	}
	return nil
}

// StartRestoreFiles launches a file-level restore in a background goroutine and
// returns immediately (see StartRestore for why). ALL validation runs
// synchronously (a bad request fails right away, no goroutine); the resolved
// alternate target folder ("" for in-place) is returned in the ack so the UI
// can show it. The detached run publishes "container:<name>" progress (phase
// "restore") and records a run (kind "restore") so the outcome — including the
// real restic error text — lands in the run history.
//
// Shares batchActive with backups and the other restores; returns
// ("", false, nil) when one is already running.
func (s *Service) StartRestoreFiles(ctx context.Context, name, source, snapshotID string, filePaths []string, targetSubPath string, confirm bool) (string, bool, error) {
	if !s.batchActive.CompareAndSwap(false, true) {
		return "", false, nil
	}
	plan, err := s.prepareRestoreFiles(ctx, name, source, snapshotID, filePaths, targetSubPath, confirm)
	if err != nil {
		s.batchActive.Store(false)
		return "", false, err
	}
	bctx := context.WithoutCancel(ctx)
	rkey := "container:" + name // the exact progBegin key this restore publishes under
	go func() {
		// runID is declared here (before recoverOperation is deferred) rather than
		// with := at its assignment below, so the onPanic closure — which can only
		// run AFTER that assignment, since a panic before it means beginRestoreRun
		// never got called and no run needs closing — sees whatever value it holds
		// at panic time instead of failing to compile (runID would otherwise not
		// exist yet at this point in the function).
		var runID string
		defer s.recoverOperation("restore files: "+name, nil, func(msg string) {
			s.finishRestoreRun(runID, "", errors.New(msg))
		})
		defer s.batchActive.Store(false)
		tctx, tcancel := context.WithTimeout(bctx, restoreTimeout)
		defer tcancel()
		rctx, cancel := context.WithCancel(tctx)
		defer cancel()
		s.registerCancel(rkey, cancel)
		defer s.unregisterCancel(rkey)
		runID = s.beginRestoreRun(name)
		pctx, startedAt := s.progBegin(rctx, rkey, "restore")
		rerr := s.runRestoreFiles(pctx, plan)
		s.progEnd(rkey, "restore", rerr == nil, startedAt)
		s.finishRestoreRun(runID, plan.snapshotID, rerr)
		if rerr != nil {
			log.Printf("api: restore files: %q failed: %v", name, rerr) //nolint:gosec // G706: name is %q-quoted
		}
	}()
	return plan.resolved, true, nil
}

// beginRestoreRun best-effort records the start of a service-layer restore run
// (kind "restore") against the container's target row, so the outcome shows up
// in the run history like the orchestrated in-place restore does. Returns ""
// when recording is impossible (no target row / store error) — the restore
// itself must never be blocked by bookkeeping.
func (s *Service) beginRestoreRun(name string) string {
	tg, err := s.store.GetTargetByContainer(name)
	if err != nil {
		log.Printf("api: restore: no target row for %q — outcome won't appear in the run history: %v", name, err) //nolint:gosec // G706: name is %q-quoted
		return ""
	}
	return s.beginRestoreRunForTarget(tg.ID)
}

// beginRestoreRunForTarget is beginRestoreRun for an already-resolved
// runs.target_id (a container target's ID, or a file set's stable id — the
// files domain records its restores against file_sets.id directly), so every
// per-item domain shares one restore-bookkeeping path. Returns "" when
// recording fails (store error) — the restore itself must never be blocked by
// bookkeeping.
func (s *Service) beginRestoreRunForTarget(targetID string) string {
	// No ctx reaches this function — a restore run is never part of a
	// "Backup Everything" pass's group-stamped children (see runGroupKey's
	// doc comment), so context.Background() is a genuine no-op here.
	runID, err := runsAdapter{st: s.store, ctx: context.Background()}.Start(targetID, "restore")
	if err != nil {
		log.Printf("api: restore: record run start for target %q failed: %v", targetID, err) //nolint:gosec // G706: targetID is %q-quoted
		return ""
	}
	return runID
}

// finishRestoreRun closes a run opened by beginRestoreRun with the terminal
// status + the (truncated) error text. A "" runID (recording was skipped) is a
// no-op; a finish failure is logged, never surfaced (best-effort bookkeeping).
func (s *Service) finishRestoreRun(runID, snapshotID string, rerr error) {
	if runID == "" {
		return
	}
	// No ctx reaches this function — same as beginRestoreRunForTarget above, a
	// restore run is never part of a "Backup Everything" pass's group-stamped
	// children (see runGroupKey's doc comment), so context.Background() is a
	// genuine no-op in every branch below.
	var err error
	switch {
	case rerr == nil:
		err = runsAdapter{st: s.store, ctx: context.Background()}.Finish(runID, "success", snapshotID, 0, "")
	case errors.Is(rerr, context.Canceled):
		// A user cancel is an intentional, recorded outcome — NOT a failure: record
		// it as "cancelled" and fire no failure alert (restores have no failure
		// notifier today; the terminal progEnd already fired to clear the bar).
		err = runsAdapter{st: s.store, ctx: context.Background()}.Finish(runID, "cancelled", "", 0, "cancelled by user")
	default:
		err = runsAdapter{st: s.store, ctx: context.Background()}.Finish(runID, "failed", "", 0, truncateRunErr(rerr))
	}
	if err != nil {
		log.Printf("api: restore: record run finish failed: %v", err)
	}
}

// finishRestoreRunWarn closes a restore run as SUCCESS but records warn in the
// run's error column — used when restic extracted all data yet could not set
// ownership/metadata on the target (see restic.ErrRestoreMetadataOnly). The run
// counts as a success everywhere (health, retention gates); the message just tells
// the operator the original ownership could not be reproduced on the share.
func (s *Service) finishRestoreRunWarn(runID, snapshotID, warn string) {
	if runID == "" {
		return
	}
	// No ctx reaches this function — same as beginRestoreRunForTarget above, a
	// restore run is never part of a "Backup Everything" pass's group-stamped
	// children (see runGroupKey's doc comment), so context.Background() is a
	// genuine no-op here.
	err := runsAdapter{st: s.store, ctx: context.Background()}.Finish(runID, "success", snapshotID, 0, warn)
	if err != nil {
		log.Printf("api: restore: record run finish (warning) failed: %v", err)
	}
}

// concludeFileSetRestore ends a file-set restore (settings-driven or foreign): it
// drives the terminal progress event AND the run record, DOWNGRADING a
// metadata-only restic failure (all data present, only ownership/metadata could
// not be set on a /mnt/user FUSE share — restic.ErrRestoreMetadataOnly) to a
// success-with-warning instead of a hard failure. Genuine failures (missing
// snapshot, no space, unreachable repo) and user cancels are recorded as-is. It
// returns the EFFECTIVE error (nil for the metadata-only case) so the caller can
// log/propagate only a real failure. startedAt must be the value the matching
// progBegin returned, so the terminal event it emits carries it too (see
// progEnd's doc comment).
func (s *Service) concludeFileSetRestore(runID, rkey, snapshotID string, rerr error, startedAt int64) error {
	if errors.Is(rerr, restic.ErrRestoreMetadataOnly) {
		s.progEnd(rkey, "restore", true, startedAt)
		s.finishRestoreRunWarn(runID, snapshotID, restic.RestoreMetadataWarning)
		return nil
	}
	s.progEnd(rkey, "restore", rerr == nil, startedAt)
	s.finishRestoreRun(runID, snapshotID, rerr)
	return rerr
}

// truncateRunErr scrubs and bounds an error message so it fits the
// runs.error column (mirrors the orchestrator's truncateErr).
//
// This applies scrubSecrets to every error EXCEPT the sentinel types
// scrubBypassMessage (handlers.go) already carves out for scrubError — those
// pass through completely unscrubbed instead, for the identical reason
// scrubError itself bypasses them: the path-shaped content in their message
// (a host:port conflict list, a ZFS dataset name, /boot vs /host/boot, …) IS
// the actionable content, not a leak. Every other error is scrubbed
// unconditionally, not just restic-originated ones. The restic adapter's own
// lastReason already scrubs before this ever sees the message, so for those
// callers scrubSecrets runs a harmless second time on already-clean text
// (scrubbing an already-scrubbed string is a no-op). But not every caller of
// this function goes through restic first: tamper.go's tamperProbe can
// surface a raw url.Parse error — url.Error's Error() embeds the full,
// UNPARSED input URL verbatim, credentials and all — when a repo's URL fails
// to parse into an *http.Request, and primary_remote.go's
// RunPrimaryTamperTest hits the exact same code path for a domain's remote
// PRIMARY. Truncating without scrubbing FIRST let that raw URL (with any
// embedded "user:pass@") reach runs.error verbatim, and from there the UI
// (handleRuns embeds store.Run directly), the weekly digest (digest.go reads
// run.Error and forwards it to every notification channel), and the widget
// feed (widget.go's truncateWidgetError only limits length) all displayed or
// forwarded it unscrubbed. Scrubbing HERE, at the one function that writes
// runs.error, protects all three downstream readers at once and doesn't rely
// on finding every current AND FUTURE non-restic call site individually.
//
// An earlier version of this function scrubbed EVERYTHING unconditionally,
// including the sentinel-tagged errors above, on the theory that running the
// scrub regexes over already-clean text is a harmless no-op. That was false
// for exactly these sentinels: scrubSecrets' path regex matches ANY
// slash-containing token, not just a filesystem path, so routing them through
// unconditionally silently mangled the very content scrubError's bypasses
// exist to protect — e.g. turning "host port 8080/tcp is already used by
// container ..." into "host port 8080[path] is already used ..." and eating
// a zvol rebase failure's ZFS dataset name the same way — even though the
// SAME underlying error survives intact when it goes through scrubError
// instead. Checking scrubBypassMessage first closes that gap without
// reimplementing scrubError's bypass logic a second, independently-drifting
// time.
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

// RestoreContainerToPath extracts a whole container snapshot into an ALTERNATE
// folder under the host mount (non-destructive — the live container is never
// touched). Unlike Restore, it stops/removes/recreates nothing: it is for
// inspecting, cloning or migrating a snapshot's data. It returns the resolved
// absolute target path (container-visible, under the host mount root); the
// handler scrubs it for the UI.
//
// SEC: the snapshot id is the strict hex guard (backup.ValidSnapshotID, the same
// guard the file/in-place restores use), the snapshot must belong to the named
// container (tag-scoped via Snapshots, like ListSnapshotFiles), and the target is
// resolved with paths.Resolve(HostMountRoot, targetSubPath) — the SAME
// containment helper SetBackupPaths/handleBrowse use, which path.Cleans and
// rejects absolute/`..` escapes. The directory is created (MkdirAll) only AFTER
// containment passes.
func (s *Service) RestoreContainerToPath(ctx context.Context, name, source, snapshotID, targetSubPath string) (string, error) {
	plan, err := s.prepareRestoreToPath(ctx, name, source, snapshotID, targetSubPath)
	if err != nil {
		return "", err
	}
	if err := s.runRestoreToPath(ctx, plan); err != nil {
		return "", err
	}
	return plan.target, nil
}

// toPathRestorePlan carries everything prepareRestoreToPath validated and
// resolved so the restic extraction can run detached from the request that
// asked for it (StartRestoreToPath) while the sync path keeps identical
// behaviour.
type toPathRestorePlan struct {
	repo       string
	mode       restic.Mode
	snapshotID string
	target     string // resolved absolute target folder (under the host mount)
}

// prepareRestoreToPath performs ALL of a to-folder restore's validation and
// resolution synchronously (see the SEC notes on RestoreContainerToPath) — so a
// bad request fails immediately with a clear error — and creates the target
// folder once containment passes.
func (s *Service) prepareRestoreToPath(ctx context.Context, name, source, snapshotID, targetSubPath string) (toPathRestorePlan, error) {
	if !validResourceName(name) {
		return toPathRestorePlan{}, errors.New("invalid container name")
	}
	if source != "local" && !isOffsiteSource(source) {
		return toPathRestorePlan{}, errors.New("invalid source (must be local or offsite)")
	}
	if !backup.ValidSnapshotID(snapshotID) {
		return toPathRestorePlan{}, backup.ErrInvalidSnapshotID
	}

	// Resolve the target against the host mount root with the shared containment
	// helper: it path.Cleans the input and rejects an absolute path or any "../"
	// that would escape the mount. The result is guaranteed to sit under the mount.
	target, err := paths.Resolve(s.cfg.HostMountRoot, targetSubPath)
	if err != nil {
		// paths.Resolve returns ErrTraversal/ErrAbsoluteSub — neither leaks a host
		// path; keep the message generic (defense-in-depth, mirrors handleBrowse).
		return toPathRestorePlan{}, errors.New("invalid target folder: must be a relative subpath under the host mount")
	}

	// Scope to the named container: the snapshot must be one of ITS snapshots, so
	// one container's data can't be extracted through another's route (same
	// access-control check as ListSnapshotFiles).
	snaps, err := s.Snapshots(ctx, name, source)
	if err != nil {
		return toPathRestorePlan{}, err
	}
	if !snapshotBelongs(snaps, snapshotID) {
		return toPathRestorePlan{}, fmt.Errorf("snapshot %s does not belong to this container", snapshotID)
	}

	settings, err := s.store.GetSettings()
	if err != nil {
		return toPathRestorePlan{}, fmt.Errorf("read settings: %w", err)
	}
	repo, err := s.repoFor(settings, "containers", source)
	if err != nil {
		return toPathRestorePlan{}, err
	}

	// Create the target dir ONLY after containment passed.
	if err := paths.EnsureDir(target); err != nil {
		return toPathRestorePlan{}, fmt.Errorf("create target folder: %w", err)
	}
	return toPathRestorePlan{
		repo:       repo,
		mode:       s.ModeFor(settings),
		snapshotID: snapshotID,
		target:     target,
	}, nil
}

// runRestoreToPath restores the WHOLE snapshot tree of an already-validated
// plan into the target dir: restic restore --target <dir> --include /
// (everything). Reuses the existing restore-to-target engine method; "/"
// includes all paths in the snapshot.
func (s *Service) runRestoreToPath(ctx context.Context, plan toPathRestorePlan) error {
	// Hold the domain repo lock for the restic work: scheduled backups bypass
	// batchActive by design and the domain lock is the layer they DO respect
	// (see executeRestore).
	unlock := s.lockDomainFor("containers", "restore")
	defer unlock()
	return s.engine.RestoreInclude(ctx, plan.repo, plan.snapshotID, "/", plan.target, plan.mode)
}

// StartRestoreToPath launches a whole-snapshot extraction into an alternate
// folder in a background goroutine and returns immediately (see StartRestore
// for why — this is THE flow that died on multi-hour restores, issue #24). ALL
// validation runs synchronously (a bad request fails right away, no goroutine);
// the resolved target folder is returned in the ack so the UI can show it. The
// detached run publishes "container:<name>" progress (phase "restore") and
// records a run (kind "restore") so the outcome — including the real restic
// error text — lands in the run history.
//
// Shares batchActive with backups and the other restores; returns
// ("", false, nil) when one is already running.
func (s *Service) StartRestoreToPath(ctx context.Context, name, source, snapshotID, targetSubPath string) (string, bool, error) {
	if !s.batchActive.CompareAndSwap(false, true) {
		return "", false, nil
	}
	plan, err := s.prepareRestoreToPath(ctx, name, source, snapshotID, targetSubPath)
	if err != nil {
		s.batchActive.Store(false)
		return "", false, err
	}
	bctx := context.WithoutCancel(ctx)
	rkey := "container:" + name // the exact progBegin key this restore publishes under
	go func() {
		var runID string // see StartRestoreFiles's identical goroutine for why this is declared here
		defer s.recoverOperation("restore to path: "+name, nil, func(msg string) {
			s.finishRestoreRun(runID, "", errors.New(msg))
		})
		defer s.batchActive.Store(false)
		tctx, tcancel := context.WithTimeout(bctx, restoreTimeout)
		defer tcancel()
		rctx, cancel := context.WithCancel(tctx)
		defer cancel()
		s.registerCancel(rkey, cancel)
		defer s.unregisterCancel(rkey)
		runID = s.beginRestoreRun(name)
		pctx, startedAt := s.progBegin(rctx, rkey, "restore")
		rerr := s.runRestoreToPath(pctx, plan)
		s.progEnd(rkey, "restore", rerr == nil, startedAt)
		s.finishRestoreRun(runID, plan.snapshotID, rerr)
		if rerr != nil {
			log.Printf("api: restore to folder: %q failed: %v", name, rerr) //nolint:gosec // G706: name is %q-quoted
		}
	}()
	return plan.target, true, nil
}

// DiffSnapshots compares two of a container's snapshots (restic diff) and
// returns the summary of what changed between them (files added/removed/changed,
// bytes added/removed).
//
// SEC: both snapshot ids pass the strict hex guard (backup.ValidSnapshotID), and
// BOTH must belong to the named container (tag-scoped via Snapshots, like
// RestoreContainerToPath/ListSnapshotFiles), so one container's snapshots can't
// be diffed through another's route. The repo+mode are resolved for the source.
func (s *Service) DiffSnapshots(ctx context.Context, name, source, snap1, snap2 string) (restic.DiffResult, error) {
	if !validResourceName(name) {
		return restic.DiffResult{}, errors.New("invalid container name")
	}
	if source != "local" && !isOffsiteSource(source) {
		return restic.DiffResult{}, errors.New("invalid source (must be local or offsite)")
	}
	if !backup.ValidSnapshotID(snap1) || !backup.ValidSnapshotID(snap2) {
		return restic.DiffResult{}, backup.ErrInvalidSnapshotID
	}

	// Scope to the named container: BOTH snapshots must be among ITS snapshots.
	snaps, err := s.Snapshots(ctx, name, source)
	if err != nil {
		return restic.DiffResult{}, err
	}
	if !snapshotBelongs(snaps, snap1) {
		return restic.DiffResult{}, fmt.Errorf("snapshot %s does not belong to this container", snap1)
	}
	if !snapshotBelongs(snaps, snap2) {
		return restic.DiffResult{}, fmt.Errorf("snapshot %s does not belong to this container", snap2)
	}

	settings, err := s.store.GetSettings()
	if err != nil {
		return restic.DiffResult{}, fmt.Errorf("read settings: %w", err)
	}
	repo, err := s.repoFor(settings, "containers", source)
	if err != nil {
		return restic.DiffResult{}, err
	}
	return s.engine.Diff(ctx, repo, snap1, snap2, s.ModeFor(settings))
}

// TagSnapshot adds tags to one of a container's snapshots (restic tag --add).
//
// SEC: the snapshot id passes the strict hex guard and must belong to the named
// container (tag-scoped via Snapshots). Tags are sanitised — trimmed, empties
// dropped, and any tag with a comma or control character rejected (restic tags
// are comma-separated, so a comma would silently split into two tags). An empty
// resulting tag set is a no-op.
func (s *Service) TagSnapshot(ctx context.Context, name, source, snapID string, addTags []string) error {
	if !validResourceName(name) {
		return errors.New("invalid container name")
	}
	if source != "local" && !isOffsiteSource(source) {
		return errors.New("invalid source (must be local or offsite)")
	}
	if !backup.ValidSnapshotID(snapID) {
		return backup.ErrInvalidSnapshotID
	}
	tags, err := sanitizeTags(addTags)
	if err != nil {
		return err
	}
	if len(tags) == 0 {
		return nil // nothing to add
	}

	// Scope to the named container: the snapshot must be among ITS snapshots.
	snaps, err := s.Snapshots(ctx, name, source)
	if err != nil {
		return err
	}
	if !snapshotBelongs(snaps, snapID) {
		return fmt.Errorf("snapshot %s does not belong to this container", snapID)
	}

	settings, err := s.store.GetSettings()
	if err != nil {
		return fmt.Errorf("read settings: %w", err)
	}
	repo, err := s.repoFor(settings, "containers", source)
	if err != nil {
		return err
	}
	mode := s.ModeFor(settings)
	// Serialize against a live backup/prune on this repo: restic tag takes an
	// exclusive lock, so run it under the domain lock like the other maintenance
	// ops, and report a clean busy instead of colliding on restic's repo lock.
	unlock, ok := s.tryLockDomainFor("containers", "tag")
	if !ok {
		return errDomainBusy
	}
	defer unlock()
	// unlockStale clears a genuine stale orphan (dead PID on this host) left by a
	// crashed run before taking restic's exclusive tag lock — the off-site repo can
	// carry a lock left by an interrupted off-site op (replication copy / integrity
	// check), and `restic tag` would otherwise fail with "repository is already
	// locked". Every other repo-mutating path (backups, PruneDomain, DeleteSnapshot)
	// does this; TagSnapshot was missing it, which made adding a tag on the off-site
	// repo fail (bug #29).
	s.unlockStale(ctx, repo, mode)
	return s.engine.TagAdd(ctx, repo, snapID, tags, mode)
}

// snapshotBelongs reports whether id (exact or unique prefix) is present in the
// already-tag-scoped snapshot list — the access-control check shared by the
// diff/tag/restore-to-path routes.
func snapshotBelongs(snaps []restic.Snapshot, id string) bool {
	for _, sn := range snaps {
		if sn.ID == id || strings.HasPrefix(sn.ID, id) {
			return true
		}
	}
	return false
}

// snapshotSubtree returns the first backed-up path (Paths[0]) of the snapshot in
// snaps matching id (exact or unambiguous prefix, like snapshotBelongs), or "" if
// there is no match or the snapshot recorded no path. It is the subtree a to-folder
// restore extracts (<id>:<subtree>) — read from the SNAPSHOT so it stays valid even
// when HostMountRoot changed since the backup (a recompute from the set's path
// would then miss).
func snapshotSubtree(snaps []restic.Snapshot, id string) string {
	for _, sn := range snaps {
		if sn.ID == id || strings.HasPrefix(sn.ID, id) {
			if len(sn.Paths) > 0 {
				return sn.Paths[0]
			}
			return ""
		}
	}
	return ""
}

// vmRunTag returns the "vmrun:<runID>" correlation tag (v8.0.0 VM
// service-layer integration, Task 1/2 — internal/backup/vm_orchestrator.go's
// VMBackupDeps.RunTag) carried by the snapshot in snaps matching id (exact or
// unambiguous prefix, like snapshotBelongs/snapshotSubtree above), or "" when
// there is no match, or the matching snapshot carries no such tag.
//
// A "" result is the PERMANENT restore fallback, not a transitional one:
// BackupVM only ever sets RunTag when the VM has zvol disks (a file-only VM's
// single snapshot is already unambiguously identified by its plain
// "vm:<name>" tag alone, so tagging it would just be permanent, purposeless
// noise — see BackupVM's own RunTag-setting comment, internal/api/service.go).
// So "" covers both a Run predating this tag's existence AND, forever
// afterwards, every file-only VM's backup — which is most VMs in production.
// Callers treat "" as "resolve via id alone", exactly how VM restore worked
// before Task 3 (v8.0.0 VM service-layer integration — restore resolution via
// the "vmrun:" tag group).
func vmRunTag(snaps []restic.Snapshot, id string) string {
	for _, sn := range snaps {
		if sn.ID != id && !strings.HasPrefix(sn.ID, id) {
			continue
		}
		for _, t := range sn.Tags {
			if strings.HasPrefix(t, "vmrun:") {
				return t
			}
		}
		return ""
	}
	return ""
}

// vmrunGroupSnapshot returns the snapshot in group (a vmRunTag-keyed
// snapshotsForTag listing) carrying tag EXACTLY — a zvol disk's own
// "vm:<name>:zvol:<dev>" identity tag (see VMBlockDisk.Dev's doc comment,
// internal/backup/vm_orchestrator.go) — or false when no member does.
func vmrunGroupSnapshot(group []restic.Snapshot, tag string) (restic.Snapshot, bool) {
	for _, sn := range group {
		for _, t := range sn.Tags {
			if t == tag {
				return sn, true
			}
		}
	}
	return restic.Snapshot{}, false
}

// sanitizeTags trims each tag, drops empties, and rejects any tag containing a
// comma or a control character. restic stores tags as a comma-separated list, so
// a comma would split one tag into two; control characters could corrupt argv or
// the snapshot metadata. Returns an error naming the offending tag.
func sanitizeTags(in []string) ([]string, error) {
	out := make([]string, 0, len(in))
	for _, raw := range in {
		tag := strings.TrimSpace(raw)
		if tag == "" {
			continue
		}
		if strings.ContainsRune(tag, ',') {
			return nil, fmt.Errorf("invalid tag %q: tags cannot contain a comma", tag)
		}
		for _, r := range tag {
			if r < 0x20 || r == 0x7f {
				return nil, fmt.Errorf("invalid tag %q: tags cannot contain control characters", tag)
			}
		}
		out = append(out, tag)
	}
	return out, nil
}

// DeleteBackups removes ALL backups of a container — every restic snapshot
// tagged container:<name>, pruning the freed data — and forgets the container
// from the store (target + run history). Used to clean up containers that are no
// longer installed. The repo is shared, so only this container's snapshots
// (filtered by tag in Snapshots) are forgotten; prune never touches data still
// referenced by other containers' snapshots.
func (s *Service) DeleteBackups(ctx context.Context, name string) error {
	settings, err := s.store.GetSettings()
	if err != nil {
		return fmt.Errorf("read settings: %w", err)
	}
	repo, err := s.containersRepoPath(settings)
	if err != nil {
		return err
	}
	// Issue #152: refused when this repo IS a remote primary flagged append-only
	// in its saved safety settings (same gate as pruneDomain/DeleteSnapshot/
	// DeleteBackupsVM) — this function has no source parameter, so it always
	// targets the primary/local repo and only the primary half of the gate
	// applies (there is no separate off-site source to check here). This path
	// runs Forget with prune=true, so skipping it here would have let a
	// compromised on-box credential irreversibly reclaim space on an immutable
	// primary.
	if s.primaryIsImmutable("containers", repo) {
		return errOffsiteAppendOnly
	}
	mode := s.ModeFor(settings)

	// Serialize against a live backup on this repo: a container Backup holds the
	// domain lock for its whole run (potentially hours), so without this an
	// unlocked bulk delete could race a concurrent `restic forget --prune`
	// against the same repo files. Same guard DeleteBackupsVM/DeleteBackupsFileSet
	// already use. (No requireExistingRepo here, unlike those two: this must still
	// let a never-backed-up container's target row be cleaned up below.)
	unlock, ok := s.tryLockDomainFor("containers", "delete")
	if !ok {
		return errDomainBusy
	}
	defer unlock()
	s.unlockStale(ctx, repo, mode)

	// Collect this container's snapshot IDs (tag-filtered) and forget them.
	snaps, err := s.Snapshots(ctx, name, "")
	if err != nil {
		return err
	}
	ids := make([]string, 0, len(snaps))
	for _, snap := range snaps {
		ids = append(ids, snap.ID)
	}
	if len(ids) > 0 {
		if err := s.engine.Forget(ctx, repo, ids, true, mode); err != nil {
			return fmt.Errorf("forget snapshots: %w", err)
		}
	}

	// Remove the target row + its run history so the container disappears from
	// the "not installed" list once its backups are gone.
	if err := s.store.DeleteTarget(name); err != nil {
		return fmt.Errorf("delete target: %w", err)
	}
	return nil
}

// DeleteBackupsVM removes ALL backups of a VM in one go — every restic snapshot
// tagged vm:<name>, pruning the freed data — from the selected source (local or
// off-site). It is the VM counterpart to DeleteBackups, but source-aware: on the
// LOCAL source it also forgets the VM from the store (target + run history) so it
// disappears from the "not installed (backups only)" list; on the OFF-SITE source
// the target is kept so the VM stays restorable from local. The repo is shared,
// so only this VM's tagged snapshots are forgotten; prune never touches data
// still referenced by other VMs' snapshots. Serialised against VM backups via the
// domain lock, and stale locks are cleared first (so it can't fail on a leftover
// lock — the same reason PruneDomain needs it).
func (s *Service) DeleteBackupsVM(ctx context.Context, name, source string) error {
	settings, repo, err := s.domainRepoSource("vms", source)
	if err != nil {
		return err
	}
	// Bulk-deleting from an immutable off-site repo is refused, same gate as
	// DeleteSnapshot/PruneDomain: this path runs Forget with prune=true, exactly
	// the destructive op append-only exists to block. The local repo is unaffected.
	// The gate is per-target: bare "offsite" checks the primary target's flag (==
	// today), "offsite:<id>" checks that specific target's.
	if isOffsiteSource(source) && s.offsiteSourceImmutable(settings, "vms", source) {
		return errOffsiteAppendOnly
	}
	// Issue #152: the SAME refusal applies when the "local" source IS actually a
	// remote primary flagged append-only in its saved safety settings (same gate
	// as pruneDomain) — there is no separate off-site copy in that shape, so
	// refusing here is the only thing standing between an on-box credential and
	// deleting the sole backup. This path also runs Forget with prune=true, so
	// skipping it here (unlike PruneDomain/DeleteSnapshot) would have let a
	// compromised on-box credential irreversibly reclaim space on an immutable
	// primary.
	if !isOffsiteSource(source) && s.primaryIsImmutable("vms", repo) {
		return errOffsiteAppendOnly
	}
	if err := s.requireExistingRepo(repo, "no backups to delete yet"); err != nil {
		return err
	}
	unlock, ok := s.tryLockDomainFor("vms", "delete")
	if !ok {
		return errDomainBusy
	}
	defer unlock()
	mode := s.ModeFor(settings)
	s.unlockStale(ctx, repo, mode)

	// Collect this VM's snapshot IDs (tag-filtered vm:<name>) and forget+prune them
	// in one restic call (Forget with prune=true).
	snaps, err := s.SnapshotsVM(ctx, name, source)
	if err != nil {
		return err
	}
	ids := make([]string, 0, len(snaps))
	for _, snap := range snaps {
		ids = append(ids, snap.ID)
	}
	if len(ids) > 0 {
		if err := s.engine.Forget(ctx, repo, ids, true, mode); err != nil {
			return fmt.Errorf("forget snapshots: %w", err)
		}
	}

	// Only drop the store target when clearing the PRIMARY (local) copy: the target
	// keeps the VM restorable from off-site, so purging any off-site replica
	// must not strand it.
	if !isOffsiteSource(source) {
		if err := s.store.DeleteVMTarget(name); err != nil {
			return fmt.Errorf("delete vm target: %w", err)
		}
	}
	return nil
}

// ForgetVMTarget removes a VM's target row + run history WITHOUT touching any
// repo — for clearing a stale "Not installed" entry that has no backups (which
// also stops the scheduler from retrying a deleted VM). Deleting actual backups
// is DeleteBackupsVM; this is just the bookkeeping cleanup.
func (s *Service) ForgetVMTarget(name string) error {
	if err := s.store.DeleteVMTarget(name); err != nil {
		return fmt.Errorf("forget vm target: %w", err)
	}
	return nil
}

// SetInclude sets the include_in_schedule flag for a container, creating the
// target row first if it does not exist yet (the first backup has not run).
// It inspects the container to resolve appdata paths exactly like Backup does,
// so the target is fully populated from the start. If docker inspect fails the
// operation is still completed: a placeholder target is upserted with a
// conventional appdata path so the toggle is never silently lost.
func (s *Service) SetInclude(ctx context.Context, name string, include bool) error {
	if _, err := s.store.GetTargetByContainer(name); err != nil {
		// Target does not exist yet — find-or-create it before calling SetInclude.
		var appdata []string
		if in, inspErr := s.docker.Inspect(ctx, name); inspErr == nil {
			appdata = s.resolveAppdataPaths(name, in)
		} else {
			log.Printf("api: SetInclude: inspect %q failed (checking fallback path): %v", name, inspErr) //nolint:gosec // G706: name is %q-quoted; no raw user bytes reach the log formatter
			// Fall back to the conventional appdata dir, but ONLY if it actually
			// exists on disk (same os.Stat guard as resolveAppdataPaths). Persisting
			// a phantom placeholder would show as a selected folder that backs up
			// nothing (issue #115); leave AppdataPaths empty (definition-only) until
			// the user points it at a real folder.
			cand := path.Join(s.cfg.HostMountRoot, "appdata", name)
			if _, statErr := os.Stat(cand); statErr == nil { //nolint:gosec // G703: cand is HostMountRoot + "appdata" + a validated container name, not raw user input
				appdata = []string{cand}
			}
		}
		if _, upsertErr := s.store.UpsertTarget(store.Target{
			ContainerName: name,
			AppdataPaths:  appdata,
		}); upsertErr != nil {
			return fmt.Errorf("ensure target: %w", upsertErr)
		}
	}
	return s.store.SetInclude(name, include)
}

// SetScheduleCadence sets a container's per-item schedule override (#121). It
// find-or-creates the target row (exactly like SetInclude) so an override can be
// set before the first backup. The cadence is validated with the same grammar the
// domain schedules use; an empty string clears the override (back to the domain
// default). everyN is rejected here because a per-item entry has no per-item
// last-run gate to enforce the interval — the same restriction the off-site/drills
// schedules carry.
func (s *Service) SetScheduleCadence(ctx context.Context, name, cadence string) error {
	cadence = strings.TrimSpace(cadence)
	if cadence != "" {
		cad, err := schedule.ParseCadence(cadence)
		if err != nil {
			return fmt.Errorf("invalid schedule: %w", err)
		}
		if cad.IntervalDays > 0 {
			return fmt.Errorf("per-item schedules do not support 'everyN': use 'off', 'daily HH:MM', 'weekly DOW HH:MM', or a cron expression")
		}
	}
	if _, err := s.store.GetTargetByContainer(name); err != nil {
		// Target does not exist yet — find-or-create it (same path as SetInclude).
		var appdata []string
		if in, inspErr := s.docker.Inspect(ctx, name); inspErr == nil {
			appdata = s.resolveAppdataPaths(name, in)
		}
		if _, upsertErr := s.store.UpsertTarget(store.Target{
			ContainerName: name,
			AppdataPaths:  appdata,
		}); upsertErr != nil {
			return fmt.Errorf("ensure target: %w", upsertErr)
		}
	}
	return s.store.SetScheduleCadence(name, cadence)
}

// SetIncludeAll sets the include_in_schedule flag for EVERY installed container
// in one call — the one-click "include all in schedule" / "exclude all" action.
// It iterates the same installed-container source the containers list uses
// (docker.List) and ensures a target row exists for each (exactly as SetInclude
// does, find-or-create) so the flag is never silently lost on a container that
// has not been backed up yet. BombVault's own container is skipped — it can
// never be backed up (ErrSelfBackup), so scheduling it would only add a failing
// job and make it show up as a schedule member. A single container's
// inspect/upsert failure aborts the batch with that error rather than leaving a
// partial, ambiguous result.
func (s *Service) SetIncludeAll(ctx context.Context, include bool) error {
	infos, err := s.docker.List(ctx)
	if err != nil {
		return fmt.Errorf("list containers: %w", err)
	}
	self := s.selfContainerName(ctx)
	for _, c := range infos {
		if self != "" && c.Name == self {
			continue // never schedule BombVault's own container
		}
		if err := s.SetInclude(ctx, c.Name, include); err != nil {
			return err
		}
	}
	return nil
}

// ContainerPath returns the resolved absolute containers backup path, used by
// the spike's path-writable probe. Returns "" if it cannot be resolved.
func (s *Service) ContainerPath() string {
	settings, err := s.store.GetSettings()
	if err != nil {
		return ""
	}
	repo, err := s.containersRepoPath(settings)
	if err != nil {
		return ""
	}
	return repo
}

// ---------------------------------------------------------------------------
// adapters across the DI seam
// ---------------------------------------------------------------------------

// resticAdapter wraps a ResticEngine + Mode to satisfy backup.Restic, converting
// the engine's float64 BytesAdded to the orchestrator's int64 Bytes.
type resticAdapter struct {
	engine ResticEngine
	mode   restic.Mode
}

var _ backup.Restic = (*resticAdapter)(nil)

func (a *resticAdapter) Backup(ctx context.Context, repo string, paths, tags []string, excludes ...string) (backup.Summary, error) {
	sum, err := a.engine.Backup(ctx, repo, paths, tags, a.mode, excludes...)
	if err != nil {
		return backup.Summary{}, err
	}
	return backup.Summary{SnapshotID: sum.SnapshotID, Bytes: int64(sum.BytesAdded)}, nil
}

func (a *resticAdapter) RestorePaths(ctx context.Context, repo, snapshotID string, paths []string) error {
	for _, p := range paths {
		if err := a.engine.RestorePath(ctx, repo, snapshotID, p, a.mode); err != nil {
			return err
		}
	}
	return nil
}

func (a *resticAdapter) RestoreSubtreeTo(ctx context.Context, repo, snapshotID, subtreePath, target string) error {
	return a.engine.RestoreSubtreeTo(ctx, repo, snapshotID, subtreePath, target, a.mode)
}

// VerifySnapshot lists the repo (which also proves it is reachable and the key
// is right) and confirms snapshotID is present, so a restore aborts before any
// destructive teardown if the snapshot is missing or the repo is unreadable.
func (a *resticAdapter) VerifySnapshot(ctx context.Context, repo, snapshotID string) error {
	snaps, err := a.engine.Snapshots(ctx, repo, a.mode)
	if err != nil {
		return fmt.Errorf("read repo: %w", err)
	}
	prefixMatches := 0
	for _, s := range snaps {
		if s.ID == snapshotID {
			return nil // exact id is unambiguous
		}
		if strings.HasPrefix(s.ID, snapshotID) {
			prefixMatches++
		}
	}
	switch prefixMatches {
	case 0:
		return fmt.Errorf("snapshot %s not found", snapshotID)
	case 1:
		return nil
	default:
		// An ambiguous short id would fail in restic AFTER the destructive teardown
		// — reject it now, before anything is stopped/destroyed.
		return fmt.Errorf("snapshot id %s is ambiguous (matches %d snapshots)", snapshotID, prefixMatches)
	}
}

// templatesAdapter satisfies backup.Templates over the template package funcs.
type templatesAdapter struct{}

var _ backup.Templates = templatesAdapter{}

func (templatesAdapter) Read(dir, name string) (string, bool, error) { return template.Read(dir, name) }
func (templatesAdapter) Write(dir, name, xml string) error           { return template.Write(dir, name, xml) }

// runsAdapter satisfies backup.Runs over *store.Repo (StartRun/FinishRun).
// ctx is captured at construction solely so Start can read
// runGroupFromContext and stamp a "Backup Everything" pass's parent run id
// onto the child run it just created (see runGroupKey's doc comment) — every
// caller whose ctx carries no group (everyone today) sees no behaviour
// change at all.
type runsAdapter struct {
	st  *store.Repo
	ctx context.Context
}

var _ backup.Runs = runsAdapter{}

func (r runsAdapter) Start(targetID, kind string) (string, error) {
	id, err := r.st.StartRun(targetID, kind)
	if err != nil {
		return "", err
	}
	if gid := runGroupFromContext(r.ctx); gid != "" {
		// Best-effort, like every other post-Start bookkeeping call in this
		// file: the run already started successfully, so a stamp failure is
		// logged, never returned (see store.SetRunGroup's doc comment).
		if serr := r.st.SetRunGroup(id, gid); serr != nil {
			log.Printf("api: run %s: stamp group %s failed: %v", id, gid, serr) //nolint:gosec // G706: id/gid are internal ids, not user input
		}
	}
	return id, nil
}

func (r runsAdapter) Finish(runID, status, snapshotID string, bytes int64, errMsg string) error {
	return r.st.FinishRun(runID, status, snapshotID, bytes, errMsg)
}

// startedRunsAdapter satisfies backup.Runs exactly like runsAdapter, except
// Start returns an ALREADY-obtained run id instead of starting a fresh run.
// Used by BackupVM (v8.0.0 VM service-layer integration, Task 2): the run id
// must be known BEFORE VMBackupDeps is built, to set RunTag =
// "vmrun:<runID>" (RunTag drives every restic tag the orchestrator builds —
// see VMBackupDeps.RunTag's doc comment — so it cannot be filled in only
// AFTER the orchestrator's own Runs.Start call, which is where the run id was
// generated before this adapter existed). BackupVM calls store.StartRun
// itself up front and wraps the result in this adapter so the orchestrator's
// internal Start call is a no-op read rather than a second, orphaned run row;
// Finish still delegates to the real store, exactly like runsAdapter.
type startedRunsAdapter struct {
	st    *store.Repo
	runID string
}

var _ backup.Runs = startedRunsAdapter{}

func (r startedRunsAdapter) Start(string, string) (string, error) { return r.runID, nil }

func (r startedRunsAdapter) Finish(runID, status, snapshotID string, bytes int64, errMsg string) error {
	return r.st.FinishRun(runID, status, snapshotID, bytes, errMsg)
}

// sshZFSHost adapts HostSSH's semantic Run/StreamCommand/RunWithStdin SSH
// surface into backup.ZFSHost by shelling the zfs command lines
// virshcli.ZFSSnapshotArgs/ZFSSnapshotDestroyArgs/ZFSSendArgs/ZFSReceiveArgs
// build — mirrors how virshcli itself shells virsh commands over the same
// transport rather than requiring a local ZFS toolchain in the container
// (v8.0.0 VM service-layer integration, Task 2).
type sshZFSHost struct{ ssh HostSSH }

var _ backup.ZFSHost = sshZFSHost{}

func (h sshZFSHost) SnapshotCreate(ctx context.Context, dataset, snapName string) error {
	_, err := h.ssh.Run(ctx, virshcli.ZFSSnapshotArgs(dataset, snapName)...)
	return err
}

func (h sshZFSHost) SnapshotDestroy(ctx context.Context, dataset, snapName string) error {
	_, err := h.ssh.Run(ctx, virshcli.ZFSSnapshotDestroyArgs(dataset, snapName)...)
	return err
}

func (h sshZFSHost) StreamSend(ctx context.Context, dataset, snapName string) (io.ReadCloser, func() error, error) {
	return h.ssh.StreamCommand(ctx, virshcli.ZFSSendArgs(dataset, snapName)...)
}

func (h sshZFSHost) StreamReceive(ctx context.Context, rd io.Reader, targetDataset string) error {
	return h.ssh.RunWithStdin(ctx, rd, virshcli.ZFSReceiveArgs(targetDataset)...)
}

// resticZvolAdapter wraps a ResticEngine + Mode to satisfy backup.ZvolRestic
// for the zvol stdin backup/restore path, mirroring resticAdapter's exact
// float64-BytesAdded -> int64-Bytes conversion. Kept as its own small type
// since backup.ZvolRestic is deliberately separate from backup.Restic (see
// its doc comment: file-backed disk backup/restore never needs stdin
// streaming) — v8.0.0 VM service-layer integration, Task 2.
type resticZvolAdapter struct {
	engine ResticEngine
	mode   restic.Mode
}

var _ backup.ZvolRestic = (*resticZvolAdapter)(nil)

func (a *resticZvolAdapter) BackupStdin(ctx context.Context, repo string, rd io.Reader, path string, tags []string) (backup.Summary, error) {
	sum, err := a.engine.BackupStdin(ctx, repo, rd, path, tags, a.mode)
	if err != nil {
		return backup.Summary{}, err
	}
	return backup.Summary{SnapshotID: sum.SnapshotID, Bytes: int64(sum.BytesAdded)}, nil
}

func (a *resticZvolAdapter) DumpTo(ctx context.Context, repo, snapshotID, path string, w io.Writer) error {
	return a.engine.DumpRaw(ctx, repo, snapshotID, path, w, a.mode)
}

// ---------------------------------------------------------------------------
// VM service methods
// ---------------------------------------------------------------------------

// vmDefinition is the recreate recipe persisted at VM backup time so restore
// works even after the VM has been deleted or BombVault's /config is lost
// (full DR). It carries container-visible paths so the restore orchestrator
// can pass them directly to restic.
type vmDefinition struct {
	DomainXML string   `json:"domain_xml"`
	DiskPaths []string `json:"disk_paths"` // container-visible absolute paths (under the Host Data mount)
	// NVRAM travels in the definition (read/written over SSH), NOT via a libvirt
	// mount. NVRAMHostPath is the host path from the domain XML; NVRAMBytes is the
	// captured var store (base64 in JSON). Empty for BIOS VMs or when SSH capture
	// failed — EnsureNVRAMTemplate then regenerates on restore.
	NVRAMHostPath string `json:"nvram_host_path"`
	NVRAMBytes    []byte `json:"nvram_bytes,omitempty"`
	// TPMBytes is the captured vTPM state, read/written over SSH the SAME way
	// as NVRAMBytes above (v8.0.0 VM service-layer integration, Task 2) — see
	// BackupVM's inline TPM read and prepareRestoreVMForTarget's PreDefine
	// write-back. Unlike NVRAM, the TPM's host path is NOT persisted here — it
	// is re-derived from DomainXML (virshcli.ParseDomain's TPMPath) at both
	// backup and restore time, since it is a stable property of the domain
	// definition itself, not something restore ever remaps (destBase's
	// cross-instance remap only rewrites file-disk sources and NVRAM — see
	// virshcli.RewriteDiskSources/RewriteNVRAM). Empty for a VM with no vTPM
	// (DomainInfo.TPMPath == "") or when SSH capture failed — a read failure
	// is non-fatal, exactly like NVRAM's.
	TPMBytes     []byte `json:"tpm_bytes,omitempty"`
	Method       string `json:"method"`
	WasAutostart bool   `json:"was_autostart"`
	// WasRunning is the VM's run state at backup time. A pointer so an OLD backup
	// (taken before this field existed) reads as nil = unknown, and restore then
	// falls back to booting the VM (the historical behaviour). A non-nil value is
	// honoured so restore mirrors the captured state, like containers do.
	WasRunning *bool `json:"was_running,omitempty"`
}

// VMView is the per-VM row returned by ListVMs.
type VMView struct {
	Name string `json:"name"`
	// LibvirtName is the raw libvirt domain name — vm.Name, NEVER
	// vm.FriendlyName — on every platform. It equals Name everywhere except
	// TrueNAS, where Name is instead the presentation-only friendly name (see
	// the isTrueNAS block in ListVMs). This is the ONLY field the frontend may
	// send back on a VM action call (backup/restore/snapshots/forget/method/
	// include/scheduleCadence/backup-order/DR-drill-target) — every such route
	// (see vmNameParam, internal/api/handlers.go) takes the path segment
	// literally and hands it straight to virsh, with zero resolution. Name is
	// display-only; sending it as an identifier is exactly the bug this field
	// fixes (a TrueNAS user's Backup Now / Restore / Forget / etc. silently
	// targeting a domain name virsh has never heard of).
	LibvirtName       string `json:"libvirtName"`
	State             string `json:"state"`
	Method            string `json:"method"`
	IncludeInSchedule bool   `json:"includeInSchedule"`
	LastBackup        *int64 `json:"lastBackup"`
	LastBackupStarted *int64 `json:"lastBackupStarted"`
	// ScheduleCadence is the VM's optional per-item schedule override (#121); ""
	// means it follows the VMs domain schedule. Only takes effect when the
	// perItemSchedules setting is on.
	ScheduleCadence string `json:"scheduleCadence"`
}

// ListVMs returns all known VMs (from virsh) merged with the DB targets.
// VMs with no virsh entry but with backup history appear as state="not-installed".
func (s *Service) ListVMs(ctx context.Context) ([]VMView, error) {
	// Only reach libvirt over SSH when the VMs domain is enabled. The dashboard
	// calls this on every GUI load; for users who don't back up VMs at all, an
	// unconditional virsh-over-SSH connect spams the container log with
	// "could not resolve hostname / connection reset" errors (forum: BJZwart).
	// Stored VM targets are still listed (as orphans) — only the live enumeration
	// is skipped.
	settings, err := s.store.GetSettings()
	if err != nil {
		return nil, fmt.Errorf("read settings: %w", err)
	}
	var infos []virshcli.VMInfo
	if settings.VMsEnabled {
		infos, err = s.virsh.List(ctx)
		if err != nil {
			return nil, fmt.Errorf("list vms: virsh: %w", err)
		}
	}
	targets, _ := s.store.ListVMTargets()
	byName := make(map[string]store.VMTarget, len(targets))
	for _, t := range targets {
		byName[t.Name] = t
	}

	// displayName is the raw libvirt name for every platform except TrueNAS,
	// where it is the presentation-only virshcli.VMInfo.FriendlyName
	// (resolves the bare UUID libvirt uses for domain names on TrueNAS 26 to
	// something readable). Gated explicitly on Kind() per the gotcha
	// documented on FriendlyName (internal/virshcli/types.go): the
	// classifier that populates it is shape-based, not platform-gated, so a
	// non-TrueNAS host must never trust it even when it happens to differ
	// from Name. vm.Name — never FriendlyName — is still used below for the
	// byName lookup and stays the identifier everywhere else in this file.
	isTrueNAS := s.platformFn().Kind() == platform.KindTrueNAS

	live := make(map[string]bool, len(infos))
	views := make([]VMView, 0, len(infos)+len(targets))
	for _, vm := range infos {
		live[vm.Name] = true
		displayName := vm.Name
		if isTrueNAS {
			displayName = vm.FriendlyName
		}
		v := VMView{Name: displayName, LibvirtName: vm.Name, State: vm.State, Method: "graceful"}
		if t, ok := byName[vm.Name]; ok {
			v.Method = t.Method
			v.IncludeInSchedule = t.IncludeInSchedule
			v.ScheduleCadence = t.ScheduleCadence
			if run, _ := s.store.LastSuccessfulBackup(t.ID); run != nil {
				v.LastBackup = run.FinishedAt
				v.LastBackupStarted = &run.StartedAt
			}
		}
		views = append(views, v)
	}
	// Orphans: targets whose VM is no longer defined on the host.
	for _, t := range targets {
		if live[t.Name] {
			continue
		}
		v := VMView{Name: t.Name, LibvirtName: t.Name, State: "not-installed", Method: t.Method, IncludeInSchedule: t.IncludeInSchedule, ScheduleCadence: t.ScheduleCadence}
		if run, _ := s.store.LastSuccessfulBackup(t.ID); run != nil {
			v.LastBackup = run.FinishedAt
			v.LastBackupStarted = &run.StartedAt
		}
		views = append(views, v)
	}
	return views, nil
}

// BackupVM orchestrates a full VM backup: resolve repo + mode, ensure repo,
// dump XML, parse domain, translate paths, upsert VM target, run orchestrator.
// leftoverOverlayDevices returns the target devices of any writable disk whose
// source is a leftover BombVault live-snapshot overlay (a "*.bombvault-tmp" file)
// left by a previously interrupted live backup. Such an overlay blocks the next
// snapshot ("…already exists…") and, if left in place, would make a backup
// capture only the overlay and not its base disk. Matching on BombVault's own
// snapshot name is unambiguous — never a cdrom or a user's manual snapshot.
func leftoverOverlayDevices(d virshcli.DomainInfo) []string {
	// libvirt names a snapshot-create-as overlay "<base>.<snapname>", so our
	// leftover is exactly a "*.bombvault-tmp" file. Match the suffix (not a bare
	// substring) so a legit disk whose PATH merely contains the name is not hit.
	suffix := "." + backup.LiveSnapshotName
	var devs []string
	for _, disk := range d.Disks {
		if strings.HasSuffix(disk.Source, suffix) {
			devs = append(devs, disk.Dev)
		}
	}
	return devs
}

// recoverLeftoverOverlay commits a leftover BombVault snapshot overlay back into
// its base BEFORE a backup, so the VM is on a clean disk chain (live snapshots
// work again and the backup captures the real base, not just the overlay). It is
// safe: it only ever commits a disk whose source is our own "*.bombvault-tmp".
// Returns the refreshed domain XML + parsed info. A no-leftover domain is
// returned unchanged. The VM must be running to active-commit; a shut-off VM
// with a leftover is an error the user must resolve (we won't silently start it).
func (s *Service) recoverLeftoverOverlay(ctx context.Context, name, xmlStr string, domain virshcli.DomainInfo) (string, virshcli.DomainInfo, error) {
	devs := leftoverOverlayDevices(domain)
	if len(devs) == 0 {
		return xmlStr, domain, nil
	}
	// Must be running to active-commit. Do NOT swallow the check error: a flaky
	// host must not be misread as "shut off" (which would send a confusing message
	// and could otherwise mask a real fault).
	running, aerr := s.virsh.IsActive(ctx, name)
	if aerr != nil {
		return xmlStr, domain, fmt.Errorf("backup vm: check running state for overlay recovery: %w", aerr)
	}
	if !running {
		return xmlStr, domain, fmt.Errorf("backup vm: %q is shut off but left on a BombVault snapshot overlay from an interrupted live backup; start it briefly so the overlay can be merged, then retry", name)
	}
	log.Printf("api: BackupVM: %q is on a leftover BombVault snapshot overlay (%v); committing it back before backup", name, devs) //nolint:gosec // G706: %q-quoted name
	for _, dev := range devs {
		if cErr := s.virsh.BlockCommitActivePivot(ctx, name, dev); cErr != nil {
			return xmlStr, domain, fmt.Errorf("backup vm: recover leftover snapshot overlay (%s): %w", dev, cErr)
		}
	}
	// Re-read the now-clean domain so we back up the real base disk, not the overlay.
	fresh, err := s.virsh.DumpXML(ctx, name)
	if err != nil {
		return xmlStr, domain, fmt.Errorf("backup vm: re-dumpxml after overlay recovery: %w", err)
	}
	freshDomain, err := virshcli.ParseDomain(fresh)
	if err != nil {
		return xmlStr, domain, fmt.Errorf("backup vm: parse domain after overlay recovery: %w", err)
	}
	// Verify the commit actually cleared the overlay; if libvirt reported success
	// but the chain is still dirty, fail with a precise message rather than letting
	// the next snapshot fail with an opaque "already exists".
	if still := leftoverOverlayDevices(freshDomain); len(still) > 0 {
		return xmlStr, domain, fmt.Errorf("backup vm: overlay recovery did not clear the snapshot overlay on %v for %q; resolve it manually", still, name)
	}
	return fresh, freshDomain, nil
}

// removeStrayOverlays deletes leftover BombVault live-snapshot overlay files
// ("*.bombvault-tmp") next to the VM's base disks. blockcommit --active --pivot
// merges an overlay back into its base and switches the VM onto the base, but
// does NOT delete the now-orphaned overlay file — so without this, EVERY
// successful live backup leaves one behind and the NEXT snapshot fails with
// "external snapshot file ... already exists". The caller MUST ensure the VM is
// on its base disks first (post-recovery / post-commit) so these files are never
// in use. Best-effort: failures are logged, never fatal.
func (s *Service) removeStrayOverlays(diskPaths []string) {
	suffix := "." + backup.LiveSnapshotName
	seen := map[string]bool{}
	for _, dp := range diskPaths {
		dir := filepath.Dir(dp)
		if dir == "" || dir == "." || seen[dir] {
			continue
		}
		seen[dir] = true
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), suffix) {
				continue
			}
			p := filepath.Join(dir, e.Name())
			if rmErr := os.Remove(p); rmErr != nil { //nolint:gosec // G304: dir derived from a translated VM disk path; name has our fixed suffix
				log.Printf("api: BackupVM: could not remove stray overlay %q: %v", e.Name(), rmErr) //nolint:gosec // G706: %q-quoted
			} else {
				log.Printf("api: BackupVM: removed stray live-snapshot overlay %q", e.Name()) //nolint:gosec // G706: %q-quoted
			}
		}
	}
}

// failVMBackup makes a pre-orchestrator VM backup failure visible: it records a
// failed run against the VM's existing target (so it shows in the dashboard run
// history) and fires a notification. Used for failures that happen BEFORE the
// orchestrator starts its own run (overlay recovery, the running-state check) so
// a destructive/aborted attempt is never silent — especially for scheduled
// backups where the HTTP error is not seen. Best-effort: any bookkeeping error
// is ignored (the real cause is already being returned to the caller).
func (s *Service) failVMBackup(ctx context.Context, name string, cause error) {
	if tg, err := s.store.GetVMTargetByName(name); err == nil {
		if runID, sErr := s.store.StartRun(tg.ID, "backup"); sErr == nil {
			msg := cause.Error()
			if len(msg) > 500 {
				msg = msg[:500]
			}
			_ = s.store.FinishRun(runID, "failed", "", 0, msg)
		}
	}
	s.notifyBackup(ctx, "VM", name, false, backup.Summary{}, cause)
}

func (s *Service) BackupVM(ctx context.Context, name string) (backup.Summary, error) {
	// Survive the client that triggered it disconnecting (see Backup): detach from
	// the request's cancellation with a generous hard cap.
	ctx, cancel := backupHoldCtx(ctx)
	defer cancel()
	defer s.lockDomain("vms")() // serialise per repo; blocks maintenance ops meanwhile
	settings, err := s.store.GetSettings()
	if err != nil {
		return backup.Summary{}, fmt.Errorf("read settings: %w", err)
	}
	repo, err := s.vmsRepoPath(settings)
	if err != nil {
		return backup.Summary{}, err
	}
	mode := s.ModeFor(settings)
	mode.Limits = s.primaryLimitsFor("vms", repo) // issue #152: a remote primary's saved bandwidth caps, else zero (unlimited)
	if err := s.EnsureRepo(ctx, repo, mode); err != nil {
		return backup.Summary{}, err
	}
	// Clear any stale lock left by a previously interrupted run so it can't block
	// this backup (BombVault is the sole writer; an active lock is never stale).
	s.unlockStale(ctx, repo, mode)

	// Pin the host key before any virsh-over-SSH call (libvirt's qemu+ssh won't
	// self-populate known_hosts). Best-effort: a failure here surfaces again on
	// the virsh call below with full context.
	if s.ssh != nil {
		if err := s.ssh.EnsureKnownHost(ctx); err != nil {
			return backup.Summary{}, fmt.Errorf("backup vm: ssh: %w", err)
		}
	}

	// Capture the domain XML and parse disk/NVRAM paths.
	xmlStr, err := s.virsh.DumpXML(ctx, name)
	if err != nil {
		// The host no longer defines this domain (deleted, or an undefined
		// template). A scheduled target can outlive the VM, so skip it with an
		// info log and a sentinel instead of failing — the scheduler treats
		// ErrVMNotInstalled as a skip, so the nightly job no longer errors/spams.
		// Returns before any run is recorded or failure notification is sent.
		if virshcli.IsNotFound(err) {
			log.Printf("api: BackupVM: skipping %q — not defined on the host (not installed; backups only)", name) //nolint:gosec // G706: name is %q-quoted
			return backup.Summary{}, backup.ErrVMNotInstalled
		}
		return backup.Summary{}, fmt.Errorf("backup vm: dumpxml: %w", err)
	}
	domain, err := virshcli.ParseDomain(xmlStr)
	if err != nil {
		return backup.Summary{}, fmt.Errorf("backup vm: parse domain: %w", err)
	}

	// If the VM is still on a leftover BombVault snapshot overlay from a previously
	// interrupted live backup, commit it back first so live snapshots work again
	// and we back up the real base disk (not just the overlay). No-op otherwise.
	xmlStr, domain, err = s.recoverLeftoverOverlay(ctx, name, xmlStr, domain)
	if err != nil {
		s.failVMBackup(ctx, name, err) // attempted/needed a destructive commit — don't fail silently
		return backup.Summary{}, err
	}

	// Guard: refuse to back up a VM with no disk images (would produce an
	// empty restic snapshot that restores nothing useful).
	if len(domain.DiskPaths) == 0 {
		return backup.Summary{}, fmt.Errorf("backup vm: no disk paths found in domain XML for %q", name)
	}

	// Disks are read by restic through the broad Host Data mount (/mnt →
	// /host/user). A disk MUST be reachable there — fail clearly otherwise rather
	// than store an un-restorable path.
	var diskPaths []string
	for _, hp := range domain.DiskPaths {
		cp, ok := s.toContainerPath(hp)
		if !ok {
			return backup.Summary{}, fmt.Errorf("backup vm: disk %q is not under the host mount and can't be reached for backup. The VM disk must live under your Host Data mount (/mnt)", hp)
		}
		diskPaths = append(diskPaths, cp)
	}

	// The VM is now guaranteed on its base disks (recoverLeftoverOverlay committed
	// any overlay). Delete stray "*.bombvault-tmp" overlay files left behind by a
	// previous live backup, otherwise the next snapshot-create fails "already
	// exists". This recovers a VM already stuck in that state.
	s.removeStrayOverlays(diskPaths)

	// NVRAM (UEFI var store) lives under /etc/libvirt on the host. Read it over
	// SSH and keep it IN the definition (no mount, no restic staging). On restore
	// it is written back over SSH; if it is missing, EnsureNVRAMTemplate
	// regenerates it from the OVMF master. A read failure is non-fatal.
	var nvramBytes []byte
	if domain.NVRAMPath != "" && s.ssh != nil {
		if b, rerr := s.ssh.ReadFile(ctx, domain.NVRAMPath); rerr == nil {
			nvramBytes = b
		} else {
			log.Printf("api: BackupVM: WARN NVRAM read over SSH failed for %q (%v) — the disks are backed up, but on restore the UEFI variables (boot entries) will be regenerated from the firmware template, not restored", name, rerr) //nolint:gosec // G706: name is %q-quoted
		}
	}

	// TPM (vTPM) state is captured the SAME way as NVRAM above — read over SSH
	// from domain.TPMPath (already parsed by ParseDomain) and kept IN the
	// definition. Empty-safe: skipped entirely when TPMPath is "" (no vTPM on
	// this domain, or an unrecognized <tpm> backend shape — see
	// virshcli.DomainInfo.TPMPath's doc comment), exactly like NVRAM. A read
	// failure is non-fatal (v8.0.0 VM service-layer integration, Task 2).
	var tpmBytes []byte
	if domain.TPMPath != "" && s.ssh != nil {
		if b, rerr := s.ssh.ReadFile(ctx, domain.TPMPath); rerr == nil {
			tpmBytes = b
		} else {
			log.Printf("api: BackupVM: WARN TPM state read over SSH failed for %q (%v) — the disks are backed up, but on restore the vTPM will start fresh/empty, not restore its captured state", name, rerr) //nolint:gosec // G706: name is %q-quoted
		}
	}

	// Block-device-backed (zvol) disks go through a SEPARATE backup mechanism
	// (BackupZvolDisk, wired via deps.BlockDisks/ZFSHost/ZvolRestic below)
	// since restic cannot back up a raw block device by path. Each entry's ZFS
	// dataset is resolved from its /dev/zvol/<pool>/<dataset> source path —
	// exactly like the disk-outside-the-host-mount check above, a block
	// device this mechanism cannot reach fails the whole backup rather than
	// silently producing an incomplete one (v8.0.0 VM service-layer
	// integration, Task 2).
	var vmBlockDisks []backup.VMBlockDisk
	for _, bd := range domain.BlockDisks {
		dataset, ok := virshcli.ZvolDatasetFromDevPath(bd.Source)
		if !ok {
			return backup.Summary{}, fmt.Errorf("backup vm: block-device disk %q (%s) is not a recognizable ZFS zvol and can't be reached for backup", bd.Dev, bd.Source)
		}
		vmBlockDisks = append(vmBlockDisks, backup.VMBlockDisk{Dataset: dataset, Dev: bd.Dev})
	}
	if len(vmBlockDisks) > 0 && s.ssh == nil {
		return backup.Summary{}, fmt.Errorf("backup vm: %q has block-device (zvol) disks but no SSH host connection is configured, and zvol backup requires SSH", name)
	}

	// Default autostart to true (safe: most Unraid-managed VMs have autostart on).
	// TODO: parse virsh dominfo output to capture the real flag in a future wave.
	wasAutostart := true

	// Get method from existing target (default graceful).
	method := "graceful"
	if existing, tErr := s.store.GetVMTargetByName(name); tErr == nil {
		method = existing.Method
	}

	// Store the PERSISTENT (inactive) definition for restore so a live-snapshot
	// restore does not re-pin transient/hot-plugged devices (e.g. a guest USB
	// manager's serial stick) that the guest re-adds itself on boot. Fall back to
	// the live XML if --inactive is unavailable.
	defXML := xmlStr
	if inactive, ierr := s.virsh.DumpXMLInactive(ctx, name); ierr == nil && strings.TrimSpace(inactive) != "" {
		defXML = inactive
	}
	// Capture the run-state so restore can mirror it (like containers). Best-effort:
	// a probe failure just leaves it unrecorded (nil) and restore falls back to
	// booting. The VM is still in its original state here (the backup stops/snapshots
	// it later, in the orchestrator).
	var wasRunning *bool
	if running, aerr := s.virsh.IsActive(ctx, name); aerr == nil {
		wasRunning = &running
	}
	def := vmDefinition{
		DomainXML:     defXML,
		DiskPaths:     diskPaths,
		NVRAMHostPath: domain.NVRAMPath,
		NVRAMBytes:    nvramBytes,
		TPMBytes:      tpmBytes,
		Method:        method,
		WasAutostart:  wasAutostart,
		WasRunning:    wasRunning,
	}
	defBytes, _ := json.Marshal(def)

	tg, err := s.store.UpsertVMTarget(store.VMTarget{
		Name: name, Method: method, Definition: string(defBytes),
	})
	if err != nil {
		return backup.Summary{}, fmt.Errorf("upsert vm target: %w", err)
	}

	// Every writable disk gets an overlay in a live snapshot, so every one must be
	// committed back afterwards (not just the first).
	var commitDevs []string
	for _, disk := range domain.Disks {
		commitDevs = append(commitDevs, disk.Dev)
	}

	deps := backup.VMBackupDeps{
		Name:             name,
		DiskPaths:        diskPaths,
		DiskDevice:       domain.DiskDevice,
		CommitDevs:       commitDevs,
		SkipSnapshotDevs: domain.SkipSnapshotDevs,
		RepoPath:         repo,
		TargetID:         tg.ID,
		DataDir:          s.cfg.DataDir,
		VM:               s.virsh,
		Restic:           &resticAdapter{engine: s.engine, mode: mode},
		BlockDisks:       vmBlockDisks,
		ZFSHost:          sshZFSHost{ssh: s.ssh},
		ZvolRestic:       &resticZvolAdapter{engine: s.engine, mode: mode},
	}
	live := false
	if method == "live" {
		// Live snapshot only works on a RUNNING VM (blockcommit --active --pivot
		// needs an active domain). A shut-off VM is backed up by graceful — which for
		// an already-off VM just backs up the disks and leaves it off (no shutdown).
		// Do NOT swallow the check error: a flaky host must never be misread as
		// "not running" and silently downgrade a live VM to a shutdown backup.
		running, aerr := s.virsh.IsActive(ctx, name)
		if aerr != nil {
			e := fmt.Errorf("backup vm: check running state: %w", aerr)
			s.failVMBackup(ctx, name, e)
			return backup.Summary{}, e
		}
		if running {
			live = true
		} else {
			log.Printf("api: BackupVM: %q is not running; using graceful backup instead of live", name) //nolint:gosec // G706: %q-quoted
		}
	}

	// Start the run HERE (rather than let the orchestrator's own Runs.Start
	// call do it, as before) so the run id is known BEFORE the orchestrator
	// runs — RunTag must be set on deps before it is passed in (v8.0.0 VM
	// service-layer integration, Task 2; see startedRunsAdapter's doc
	// comment). Wrapping it in startedRunsAdapter means the orchestrator's own
	// Runs.Start call is a no-op read of this same id, not a second run row.
	runID, err := s.store.StartRun(tg.ID, "backup")
	if err != nil {
		return backup.Summary{}, fmt.Errorf("backup vm: record run start: %w", err)
	}
	if gid := runGroupFromContext(ctx); gid != "" {
		// Same "Backup Everything" group-stamp runsAdapter.Start does — inline
		// here (rather than inside startedRunsAdapter) because the run id, and
		// therefore the stamp, is produced ONCE right here, not inside a later
		// Start() call (see startedRunsAdapter's doc comment above). Best-effort:
		// a stamp failure must never fail a backup that already started.
		if serr := s.store.SetRunGroup(runID, gid); serr != nil {
			log.Printf("api: BackupVM: run %s: stamp group %s failed: %v", runID, gid, serr) //nolint:gosec // G706: runID/gid are internal ids, not user input
		}
	}
	deps.Runs = startedRunsAdapter{st: s.store, runID: runID}
	// RunTag correlates every snapshot ONE backup invocation produces — only
	// meaningful (and only set) when this backup will actually produce MORE
	// than one restic snapshot (a file-only VM's single snapshot is already
	// unambiguously identified by its "vm:<name>" tag; adding a "vmrun:" tag
	// to it would just be permanent, purposeless noise on every VM snapshot
	// in the repo). Leaving it empty for a file-only VM also keeps that VM's
	// restic Backup call BYTE-IDENTICAL to before this task.
	if len(vmBlockDisks) > 0 {
		deps.RunTag = "vmrun:" + runID
	}

	vkey := "vm:" + name
	// Healthchecks /start ping: deferred to here, past every pre-flight early-return
	// (incl. the ErrVMNotInstalled skip), so the paired done/fail notifyBackup below
	// always follows (no dangling /start).
	s.notifyBackupStart(ctx, "VM")
	bctx, startedAt := s.progBegin(ctx, vkey, "backup")
	var sum backup.Summary
	if live {
		sum, err = backup.BackupVMLive(bctx, deps)
	} else {
		sum, err = backup.BackupVMGraceful(bctx, deps)
	}
	s.progEnd(vkey, "backup", err == nil, startedAt)
	s.notifyBackup(ctx, "VM", name, err == nil, sum, err)
	if err != nil {
		return backup.Summary{}, err
	}
	// A successful live backup commits its overlay back into the base and pivots
	// the VM onto it, but leaves the orphaned overlay file behind — delete it so
	// the next snapshot doesn't fail "already exists". No-op after graceful.
	if live {
		s.removeStrayOverlays(diskPaths)
	}
	// Mirror the definition (encrypted) onto the backup storage so a freshly
	// installed BombVault can rebuild this VM via DiscoverVMs after a database
	// loss — and so a VM deleted from the host stays restorable. Best-effort.
	if wErr := s.writeVMDefToStorage(settings, name, defBytes); wErr != nil {
		log.Printf("api: backup vm: WARN could not persist definition for %q to storage: %v", name, wErr) //nolint:gosec // G706: name is %q-quoted
	}
	// Apply retention once per identity tag this backup actually produced: the
	// main file-backed "vm:<name>" always, plus one call per distinct
	// "vm:<name>:zvol:<dev>" tag (one per BlockDisks entry — see
	// VMBlockDisk.Dev's doc comment) — reusing restic's own native per-tag
	// forget so each disk's history is retained/pruned as its own group
	// instead of being lumped in with the file-backed snapshot's (v8.0.0 VM
	// service-layer integration, Task 2). A file-only VM (vmBlockDisks empty)
	// makes exactly the one call it always has.
	s.applyRetention(ctx, repo, settings, mode, "vm:"+name, "vms")
	for _, bd := range vmBlockDisks {
		if bd.Dev == "" {
			continue // no distinct identity tag without a target dev — lumped into "vm:<name>" above
		}
		s.applyRetention(ctx, repo, settings, mode, "vm:"+name+":zvol:"+bd.Dev, "vms")
	}
	makeRepoReadable(repo) // keep the local repo copyable off-box by a non-root user
	s.replicateOffsite(ctx, "vms", settings, mode, repo)
	s.maybeCollectStats(ctx, "vms")
	s.checkPrimaryRemoteBudget(ctx, "vms", repo, settings)
	return sum, nil
}

// RestoreVM orchestrates a VM restore from a stored definition.
func (s *Service) RestoreVM(ctx context.Context, name, snapshotID string, confirm bool, source string, leaveStopped bool) error {
	plan, err := s.prepareRestoreVM(ctx, name, snapshotID, confirm, source)
	if err != nil {
		return err
	}
	return s.executeRestoreVM(ctx, name, plan, leaveStopped)
}

// vmRestorePlan carries everything prepareRestoreVM validated and resolved so
// the long-running execution can run detached from the request that asked for
// it (StartRestoreVM) while the sync RestoreVM path keeps identical behaviour.
type vmRestorePlan struct {
	repo         string
	mode         restic.Mode
	targetID     string
	snapshotID   string
	diskPaths    []string
	domainXML    string
	wasAutostart bool
	// restoreDirs drives a REMAPPED restore (cross-instance): each entry restores a
	// snapshot subtree into a chosen destination dir. Empty = same-instance restore
	// (each disk goes back to its own path), byte-for-byte the historical behaviour.
	restoreDirs []backup.VMRestoreDir
	// wasRunning is the captured run state (nil = old backup with no recorded
	// state → boot after restore, the historical behaviour).
	wasRunning *bool
	preDefine  func(context.Context) error
	// blockDisks are the domain's block-device-backed (zvol) disks to restore,
	// each entry's SourceDataset resolved from the domain XML — SnapshotID/
	// StdinPath are left unset here; see prepareRestoreVMForTarget's own
	// comment for why (Task 3's job). Empty for a VM with only file-backed
	// disks (v8.0.0 VM service-layer integration, Task 2).
	blockDisks []backup.VMRestoreBlockDisk
}

// prepareRestoreVM performs ALL of a VM restore's validation and resolution
// synchronously — confirmation, snapshot-id guard + ownership, definition
// lookup, disk-path containment and the SSH host-key pin — so a bad request
// fails immediately with a clear error, BEFORE anything long-running starts.
// prepareRestoreVM resolves the settings-configured vms repo (local or
// off-site) and delegates to prepareRestoreVMIn. Request guards run here first
// so a bad request keeps failing with its own error before any resolution;
// prepareRestoreVMIn re-validates them (defense-in-depth for non-settings
// callers such as the foreign-repo session).
func (s *Service) prepareRestoreVM(ctx context.Context, name, snapshotID string, confirm bool, source string) (vmRestorePlan, error) {
	if !confirm {
		return vmRestorePlan{}, backup.ErrNotConfirmed
	}
	if snapshotID != "latest" && snapshotID != "" && !backup.ValidSnapshotID(snapshotID) {
		return vmRestorePlan{}, backup.ErrInvalidSnapshotID
	}
	settings, err := s.store.GetSettings()
	if err != nil {
		return vmRestorePlan{}, fmt.Errorf("read settings: %w", err)
	}
	repo, err := s.repoFor(settings, "vms", source)
	if err != nil {
		return vmRestorePlan{}, err
	}
	return s.prepareRestoreVMIn(ctx, repoRef{repo: repo, mode: s.ModeFor(settings)}, name, snapshotID, confirm)
}

// prepareRestoreVMIn performs ALL of a VM restore's validation and resolution
// synchronously against an explicit repository, mirroring prepareRestoreIn.
func (s *Service) prepareRestoreVMIn(ctx context.Context, ref repoRef, name, snapshotID string, confirm bool) (vmRestorePlan, error) {
	if !confirm {
		return vmRestorePlan{}, backup.ErrNotConfirmed
	}
	// An explicit snapshot id must be well-formed hex. The orchestrator re-checks
	// this, but guarding here makes a bad id fail synchronously (fail-fast for the
	// async StartRestoreVM path). "latest"/"" resolve below.
	explicitID := snapshotID != "latest" && snapshotID != ""
	if explicitID && !backup.ValidSnapshotID(snapshotID) {
		return vmRestorePlan{}, backup.ErrInvalidSnapshotID
	}

	tg, err := s.store.GetVMTargetByName(name)
	if err != nil {
		return vmRestorePlan{}, errors.New("vm has not been backed up yet")
	}
	// Same-instance restore: no destination base and no destination zvol pool,
	// so disks go back to their own paths and the domain XML is used verbatim
	// (byte-for-byte historical behaviour).
	return s.prepareRestoreVMForTarget(ctx, ref, name, snapshotID, tg, "", "")
}

// prepareRestoreVMForTarget builds a VM restore plan for an ALREADY-RESOLVED VM
// target tg against an explicit repo ref, WITHOUT reading or writing the store —
// the VM counterpart of prepareRestoreForTarget. The foreign restore passes a
// target built from the decrypted foreign definition so its disk-path containment
// is validated BEFORE that recipe is persisted locally (prepareForeignRestore
// adopts it only once this returns a plan, never on a validation failure). The
// caller runs the confirm / explicit-snapshot-id-shape guards first.
//
// destBase, when non-empty, REMAPS every FILE-backed disk (and the NVRAM) to
// <destBase>/<name>/<basename>: the restic restore target, the domain XML
// <disk><source file> / <nvram> paths, and the SSH NVRAM write all point at the
// destination FOLDER instead of the source server's paths. A remap ALSO gates
// the restore behind guardVMRestoreDestination, so it can never write a
// multi-GB disk onto an unmounted path (the RAM rootfs) and brick the host
// (#122). An empty destBase leaves the same-instance restore byte-for-byte
// unchanged.
//
// destZvolPool is the SEPARATE remap a BLOCK-DEVICE (zvol) disk needs: destBase
// is a filesystem path under the host mount and carries no ZFS pool
// information at all, so it cannot rebase a zvol's `zfs receive` target the
// way it rebases a file-backed disk's path. When destBase is non-empty (a
// cross-instance restore) and the domain has recognizable zvol disks,
// destZvolPool MUST be supplied — every such disk's dataset is rebased onto it
// via virshcli.RebaseZvolDatasetPool, so `zfs receive` lands on the
// DESTINATION box's pool rather than the source box's. An empty destZvolPool
// on a cross-instance restore with zvol disks REFUSES here, before any
// restic/SSH work starts (a destination pool cannot be guessed from destBase),
// rather than silently attempting `zfs receive` against the source pool's name
// and failing deep inside that call once the restore is already underway.
// Ignored (never validated) for a same-instance restore or a VM with no zvol
// disks — same-instance zvol restore keeps deriving its target from
// SourceDataset exactly as before this parameter existed.
func (s *Service) prepareRestoreVMForTarget(ctx context.Context, ref repoRef, name, snapshotID string, tg store.VMTarget, destBase, destZvolPool string) (vmRestorePlan, error) {
	explicitID := snapshotID != "latest" && snapshotID != ""

	// "latest" (or empty) resolves to the VM's newest snapshot. An explicit id
	// must belong to THIS VM (tag-scoped, mirroring the container restores'
	// access-control check) — listed against the caller's repo ref.
	snaps, snapErr := s.snapshotsForTag(ctx, ref.repo, ref.mode, "vm:"+name)
	if snapErr != nil {
		return vmRestorePlan{}, snapErr
	}
	if explicitID {
		if !snapshotBelongs(snaps, snapshotID) {
			return vmRestorePlan{}, fmt.Errorf("snapshot %s does not belong to this vm", snapshotID)
		}
	} else {
		if len(snaps) == 0 {
			return vmRestorePlan{}, errors.New("no backups found for this vm")
		}
		snapshotID = snaps[len(snaps)-1].ID
	}

	// Resolve this run's "vmrun:<runID>" correlation group (v8.0.0 VM
	// service-layer integration, Task 3): every restic snapshot ONE backup
	// invocation produced — the main file-backed snapshot at snapshotID
	// above plus one per zvol disk (see VMBackupDeps.RunTag's doc comment,
	// internal/backup/vm_orchestrator.go). The tag is read straight off the
	// already-resolved snapshot (found in snaps, the exact "vm:"+name-tagged
	// list snapshotID above came from — explicit or latest, so no extra
	// restic listing is needed just to find it), then ONE more
	// snapshotsForTag call, over the SAME repo ref, keyed on that tag.
	//
	// vmrunGroup stays nil — the PERMANENT fallback (see vmRunTag's doc
	// comment) — when the resolved snapshot carries no "vmrun:" tag at all.
	// In that case vmRestoreBlockDisks below leaves every entry's
	// SnapshotID/StdinPath at zero value, EXACTLY the pre-Task-3 behavior.
	var vmrunGroup []restic.Snapshot
	if runTag := vmRunTag(snaps, snapshotID); runTag != "" {
		group, gErr := s.snapshotsForTag(ctx, ref.repo, ref.mode, runTag)
		if gErr != nil {
			return vmRestorePlan{}, gErr
		}
		vmrunGroup = group
	}

	if tg.Definition == "" {
		return vmRestorePlan{}, errors.New("no stored definition for this vm: run a backup once first")
	}
	var def vmDefinition
	if err := json.Unmarshal([]byte(tg.Definition), &def); err != nil {
		return vmRestorePlan{}, fmt.Errorf("restore vm: unmarshal definition: %w", err)
	}

	// Disks must live within the Host Data mount (that is how restic reaches
	// them). SKIP any that don't rather than refusing the whole VM.
	var diskPaths []string
	for _, p := range def.DiskPaths {
		if paths.Within(s.cfg.HostMountRoot, p) {
			diskPaths = append(diskPaths, p)
		} else {
			log.Printf("api: RestoreVM: skipping disk path %q outside mount root", p) //nolint:gosec // G706: %q-quoted
		}
	}
	if len(diskPaths) == 0 {
		return vmRestorePlan{}, errors.New("no restorable disk paths found in this backup")
	}

	domainXML := def.DomainXML
	nvramHostPath := def.NVRAMHostPath
	var restoreDirs []backup.VMRestoreDir

	// REMAP (cross-instance restore): place every disk under <destBase>/<name>/ on
	// the DESTINATION pool, rewrite the domain XML disk/nvram sources to match, and
	// GUARD the destination so the restore can never fill an unmounted path (the RAM
	// rootfs) and brick the host (#122). An empty destBase is the same-instance
	// restore and skips all of this (disks return to their own paths, XML verbatim).
	if destBase != "" {
		destDir := path.Join(path.Clean(destBase), name) // container path
		destHostDir := s.toHostPath(destDir)             // host path for the domain XML
		diskRemap := make(map[string]string, len(diskPaths))
		seenDir := map[string]bool{}
		remapped := make([]string, 0, len(diskPaths))
		for _, cp := range diskPaths {
			base := path.Base(cp)
			remapped = append(remapped, destDir+"/"+base)
			diskRemap[s.toHostPath(cp)] = destHostDir + "/" + base
			if src := path.Dir(cp); !seenDir[src] {
				seenDir[src] = true
				restoreDirs = append(restoreDirs, backup.VMRestoreDir{Subtree: src, Target: destDir})
			}
		}
		diskPaths = remapped
		domainXML = virshcli.RewriteDiskSources(domainXML, diskRemap)
		if nvramHostPath != "" {
			newNVRAM := destHostDir + "/" + path.Base(nvramHostPath)
			domainXML = virshcli.RewriteNVRAM(domainXML, newNVRAM)
			nvramHostPath = newNVRAM
		}
		// HOST-BRICK GUARD: prove the destination is on a real mounted pool with room
		// BEFORE any restic write. On failure nothing is written.
		if err := s.guardVMRestoreDestination(ctx, ref, snapshotID, destDir); err != nil {
			return vmRestorePlan{}, err
		}
	}

	// Make UEFI domains bootable even if the captured NVRAM is absent: add a
	// template= to <nvram> so libvirt regenerates it from the OVMF master. When
	// NVRAM bytes were captured, PreDefine writes them back over SSH first, so
	// libvirt uses the real var store (boot entries preserved).
	domainXML = virshcli.EnsureNVRAMTemplate(domainXML)

	// Re-derive the TPM path and block-device (zvol) disk list from the
	// (possibly remapped) domain XML — NOT persisted separately in
	// vmDefinition (see its TPMBytes field's doc comment) — mirroring how
	// BackupVM derives them at backup time via the same virshcli.ParseDomain
	// call. Neither is affected by the destBase remap above: RewriteDiskSources
	// only rewrites <source file=...> (file-backed disks); a <tpm> element and
	// a <source dev=...> (block-device disk) are left untouched either way
	// (v8.0.0 VM service-layer integration, Task 2). A zvol disk's DATASET
	// (as opposed to the XML's device path, which stays the source box's for
	// the operator's own reference) is rebased separately, below — see the
	// "CROSS-INSTANCE zvol rebase" block after the SSH guard.
	var tpmPath string
	var vmRestoreBlockDisks []backup.VMRestoreBlockDisk
	if parsed, perr := virshcli.ParseDomain(domainXML); perr == nil {
		tpmPath = parsed.TPMPath
		for _, bd := range parsed.BlockDisks {
			dataset, ok := virshcli.ZvolDatasetFromDevPath(bd.Source)
			if !ok {
				log.Printf("api: RestoreVM: WARN block-device disk %q (%s) is not a recognizable ZFS zvol for %q — it will not be restored", bd.Dev, bd.Source, name) //nolint:gosec // G706: name is %q-quoted
				continue
			}
			rbd := backup.VMRestoreBlockDisk{SourceDataset: dataset}
			// Resolve THIS disk's own restic snapshot from the vmrun: group
			// above (v8.0.0 VM service-layer integration, Task 3): its
			// identity tag is "vm:"+name+":zvol:"+bd.Dev (see
			// VMBlockDisk.Dev's doc comment, internal/backup/
			// vm_orchestrator.go) — find the group member carrying that
			// EXACT tag and take its snapshot id plus the one path it
			// recorded, which IS the exact StdinPath BackupZvolDisk gave
			// restic (BackupStdin backs up exactly one synthetic file per
			// invocation — see ZvolStdinPath — so Paths always has exactly
			// one entry when set at all).
			//
			// Left at zero value — the PERMANENT fallback (see vmRunTag's
			// doc comment above), not just "old backups" — when there is no
			// group at all, or no member carries this disk's tag (bd.Dev
			// empty never happens from the real BackupVM caller, see
			// VMBlockDisk.Dev's doc comment): RestoreZvolDisk then fails
			// loudly on the empty snapshot id rather than silently skipping
			// the disk, EXACTLY as it did before this task.
			if bd.Dev != "" {
				if gs, ok := vmrunGroupSnapshot(vmrunGroup, "vm:"+name+":zvol:"+bd.Dev); ok && len(gs.Paths) > 0 {
					rbd.SnapshotID = gs.ID
					rbd.StdinPath = gs.Paths[0]
				}
			}
			vmRestoreBlockDisks = append(vmRestoreBlockDisks, rbd)
		}
	} else {
		log.Printf("api: RestoreVM: WARN could not re-parse domain xml for %q (%v) — TPM state and any zvol disks will not be restored", name, perr) //nolint:gosec // G706: name is %q-quoted
	}

	// Mirrors BackupVM's own "zvol backup requires SSH" guard: RestoreZvolDisk
	// calls ZFSHost.StreamReceive, which sshZFSHost forwards straight onto
	// s.ssh — a nil HostSSH there is a nil interface method call, i.e. an
	// unrecovered PANIC (not a clean error) deep inside the async restore
	// goroutine (StartRestoreVM), which crashes the whole process rather than
	// just failing this one restore. Caught here, before a plan is ever
	// returned, exactly like the backup-side check in BackupVM.
	if len(vmRestoreBlockDisks) > 0 && s.ssh == nil {
		return vmRestorePlan{}, fmt.Errorf("restore vm: %q has block-device (zvol) disks but no SSH host connection is configured, and zvol restore requires SSH", name)
	}

	// CROSS-INSTANCE zvol rebase: destBase remaps a FILE-backed disk onto the
	// destination's FILESYSTEM PATH, but that path carries no ZFS pool
	// information — RestoreZvolDisk would otherwise always derive its `zfs
	// receive` target from SourceDataset (the SOURCE box's pool, re-derived
	// unchanged from the domain XML above), which almost certainly does not
	// exist on the destination box under that name, and fail deep inside a
	// `zfs receive` call once the restore is already underway. REFUSE here,
	// before any restic/SSH work starts, when destZvolPool wasn't supplied;
	// when it was, rebase every zvol disk's dataset onto it now so the plan's
	// blockDisks already carry the correct destination target.
	//
	// destZvolPool is trimmed before the emptiness check so a whitespace-only
	// value ("   ", e.g. a stray space pasted into a direct API call) hits
	// THIS clear refusal rather than slipping through to
	// RebaseZvolDatasetPool's own less-actionable error below (it trims
	// internally too, so a whitespace-only pool reads there as empty and
	// fails the "no segment past its own pool" check instead of naming the
	// real problem).
	if destBase != "" && len(vmRestoreBlockDisks) > 0 {
		if strings.TrimSpace(destZvolPool) == "" {
			// The web UI has NO field for zvolPool yet (Recovery page — see
			// this repo's tracked follow-up); an operator hitting this
			// through the UI has no in-app way to act on this error, so the
			// message spells out the only path that currently exists: a
			// direct API call. Keep this in sync with
			// docs/vm-backup-ssh-setup.md's TrueNAS section, which carries
			// the same guidance for someone reading ahead of time.
			return vmRestorePlan{}, fmt.Errorf("restore vm: %q has %d TrueNAS zvol-backed disk(s) and this is a cross-instance restore, but no destination ZFS pool was specified, and the destination pool cannot be inferred from the chosen destination folder. There is no web UI field for this yet: call POST /api/foreign/restore directly with its zvolPool parameter set to the destination pool name (see docs/vm-backup-ssh-setup.md's TrueNAS section), or restore this VM on the instance it was backed up from", name, len(vmRestoreBlockDisks))
		}
		for i := range vmRestoreBlockDisks {
			rebased, ok := virshcli.RebaseZvolDatasetPool(vmRestoreBlockDisks[i].SourceDataset, destZvolPool)
			if !ok {
				return vmRestorePlan{}, &zvolRebaseErr{msg: fmt.Sprintf("restore vm: %q: cannot rebase zvol dataset %q onto destination pool %q", name, vmRestoreBlockDisks[i].SourceDataset, destZvolPool)}
			}
			vmRestoreBlockDisks[i].RestoreBaseDataset = rebased
		}
	}

	// preDefine writes the captured NVRAM/TPM state back to the host over SSH
	// AFTER the old domain is undefined (which removes its nvram) and BEFORE
	// `virsh define`, so the restored VM boots with its original UEFI
	// variables and vTPM state. It writes to the (possibly remapped)
	// destination nvram path plus the re-derived TPM path. No-op for either
	// when there is nothing to write or SSH is unavailable.
	var preDefine func(context.Context) error
	if s.ssh != nil && ((len(def.NVRAMBytes) > 0 && nvramHostPath != "") || (len(def.TPMBytes) > 0 && tpmPath != "")) {
		writeNVRAMPath := nvramHostPath
		writeTPMPath := tpmPath
		preDefine = func(ctx context.Context) error {
			if len(def.NVRAMBytes) > 0 && writeNVRAMPath != "" {
				if err := s.ssh.WriteFile(ctx, writeNVRAMPath, def.NVRAMBytes); err != nil {
					log.Printf("api: RestoreVM: WARN NVRAM write over SSH failed for %q (%v) — the VM is restored and will boot, but libvirt regenerates the UEFI variables from the firmware template, so boot entries may need to be re-added", name, err) //nolint:gosec // G706: name is %q-quoted
				}
			}
			if len(def.TPMBytes) > 0 && writeTPMPath != "" {
				if err := s.ssh.WriteFile(ctx, writeTPMPath, def.TPMBytes); err != nil {
					log.Printf("api: RestoreVM: WARN TPM state write over SSH failed for %q (%v) — the VM is restored and will boot, but the vTPM starts fresh/empty instead of restoring its captured state", name, err) //nolint:gosec // G706: name is %q-quoted
				}
			}
			return nil // never block the restore on NVRAM/TPM — the firmware-template fallback keeps the VM bootable
		}
	}

	// Pin the host key before the orchestrator's virsh-over-SSH calls.
	if s.ssh != nil {
		if err := s.ssh.EnsureKnownHost(ctx); err != nil {
			return vmRestorePlan{}, fmt.Errorf("restore vm: ssh: %w", err)
		}
	}

	return vmRestorePlan{
		repo:         ref.repo,
		mode:         ref.mode,
		targetID:     tg.ID,
		snapshotID:   snapshotID,
		diskPaths:    diskPaths,
		domainXML:    domainXML,
		wasAutostart: def.WasAutostart,
		restoreDirs:  restoreDirs,
		wasRunning:   def.WasRunning,
		preDefine:    preDefine,
		blockDisks:   vmRestoreBlockDisks,
	}, nil
}

// guardVMRestoreDestination is the HOST-BRICK GUARD for a remapped (cross-instance)
// VM restore (#122): it proves the destination directory is safe to write a
// multi-GB disk image into BEFORE restic writes anything. Two checks:
//
//  1. destDir must sit on a REAL mounted pool/share — a mount point that is a
//     proper descendant of HostMountRoot (destinationMounted, shared with #120).
//     The failure this prevents: a source path like /mnt/zfs/domains maps to an
//     UNMOUNTED dir on the destination, so /mnt lives on the RAM rootfs and restic
//     writes the image into tmpfs → OOM kills emhttpd/nginx → the host is bricked.
//  2. There must be enough free space for the restore. The size comes from the
//     SOURCE snapshot's restore-size (a read of the source repo); the free-space
//     probe runs on the nearest existing ancestor of destDir. A probe error is
//     treated as "cannot prove insufficient" and does not block (the mount check
//     is the primary defence); only a proven shortfall aborts.
func (s *Service) guardVMRestoreDestination(ctx context.Context, ref repoRef, snapshotID, destDir string) error {
	if !s.destinationMounted(destDir) {
		return destinationRefusal("restore destination %q is not on a mounted pool or share; a VM disk restored there would be written into the host's RAM and crash it. Choose a destination folder on real storage and retry", s.toHostPath(destDir))
	}
	_, wantBytes, err := s.engine.StatsRestoreSize(ctx, ref.repo, snapshotID, ref.mode)
	if err != nil {
		return fmt.Errorf("restore preflight: measure restore size: %w", err)
	}
	if wantBytes > 0 {
		if free, ferr := s.diskFreeFn()(nearestExistingDir(destDir)); ferr == nil && free < uint64(wantBytes) {
			return destinationRefusal("not enough free space to restore this VM: it needs %d bytes but the destination %q has only %d free. Free up space or choose another destination", wantBytes, s.toHostPath(destDir), free)
		}
	}
	return nil
}

// guardContainerRestoreDestination is the container counterpart of
// guardVMRestoreDestination, run ONLY for a cross-instance (foreign) container
// restore (issue #123 / the #122 class for containers). A foreign recipe carries
// the SOURCE host's absolute appdata paths; if the destination host lacks that
// pool (e.g. the source used /mnt/zfs but this box has no zfs share) restic would
// write appdata into an UNMOUNTED dir under the host mount — the array/RAM rootfs
// — silently landing the data in the wrong place (or bricking the host on a large
// restore). This proves every appdata target sits on a genuinely mounted
// pool/share BEFORE the caller reaches executeRestore's destructive Stop/Remove,
// turning that silent wrong-write into a clear, actionable refusal. The standard
// /mnt/user/appdata case always passes (shfs is a live mount below the host bind),
// so a normal cross-Unraid restore never regresses.
func (s *Service) guardContainerRestoreDestination(ctx context.Context, ref repoRef, snapshotID string, appdataPaths []string) error {
	for _, p := range appdataPaths {
		if !s.destinationMounted(p) {
			return destinationRefusal("appdata destination %q is not on a mounted pool or share on this system — the source backed it up from a pool this host does not have, so restoring would write it to the wrong place. Create or mount that share here, then retry", s.toHostPath(p))
		}
	}
	// Free-space preflight ONLY when the whole restore lands in a SINGLE appdata
	// path: with several paths they may sit on different pools and the snapshot's
	// total restore-size can't be attributed per pool, which would falsely refuse a
	// legitimate split restore. Non-blocking either way — a probe error is "cannot
	// prove insufficient" and never blocks; only a PROVEN shortfall aborts (exactly
	// like guardVMRestoreDestination).
	if len(appdataPaths) == 1 {
		if _, wantBytes, err := s.engine.StatsRestoreSize(ctx, ref.repo, snapshotID, ref.mode); err == nil && wantBytes > 0 {
			if free, ferr := s.diskFreeFn()(nearestExistingDir(appdataPaths[0])); ferr == nil && free < uint64(wantBytes) {
				return destinationRefusal("not enough free space to restore this container's appdata: it needs %d bytes but %q has only %d free. Free up space and retry", wantBytes, s.toHostPath(appdataPaths[0]), free)
			}
		}
	}
	return nil
}

// appdataRelPath splits a backed-up container path into the appdata ROOT it lives
// in and the container-relative remainder below it, e.g.
// /host/user/user/appdata/SnapOtter/conf → ("/host/user/user/appdata",
// "SnapOtter/conf") and /host/user/zfs/appdata/nexterm → ("/host/user/zfs/appdata",
// "nexterm"). The split is on the FIRST "appdata" segment — that is the share/pool
// root every recorded path is selected by (resolveAppdataPaths keeps a bind only
// when hasSegment(src,"appdata")), and taking the first occurrence keeps a
// container that happens to own a nested folder called "appdata"
// (…/appdata/foo/appdata) anchored at the real root.
//
// rel is "" when the path has no "appdata" segment at all or IS an appdata root
// (a container binding /mnt/user/appdata itself); callers then fall back to the
// plain basename, which is exactly the pre-#125-fix behaviour for those shapes.
func appdataRelPath(src string) (root, rel string) {
	segs := strings.Split(src, "/")
	for i, seg := range segs {
		if seg == "appdata" && i+1 < len(segs) {
			return strings.Join(segs[:i+1], "/"), strings.Join(segs[i+1:], "/")
		}
	}
	return "", ""
}

// containerAppdataRemap builds the per-appdata-path remap for a cross-pool
// (foreign) container restore: each backed-up appdata path's CONTENTS are
// restored into <destBase>/<container-relative path> on the DESTINATION pool, and
// the recreated container's binds + template are pointed there. destBase is a
// container path under HostMountRoot. It returns (1) the restic restore dirs
// (Subtree = the source appdata path, i.e. one of the snapshot's own backed-up
// paths; Target = the dest path), and (2) a bindRemap of HOST source path -> HOST
// dest path for rewriting HostConfig.Binds + the flashed template.
//
// The destination keeps the path structure BELOW the source's appdata root, not
// just the final segment. resolveAppdataPaths records every appdata bind as its
// OWN entry without merging binds that share a folder, so a container with two
// binds — /mnt/user/appdata/SnapOtter/conf and …/SnapOtter/data — arrives here as
// two paths sharing the "SnapOtter" ancestor. Mapping each one by its BASENAME
// alone dropped that ancestor and scattered the container's data as "conf" and
// "data" directly in the destination appdata root, next to (and able to overwrite)
// other containers' folders. Keeping the relative path nests them back under
// <destBase>/SnapOtter/… The single-bind case is unchanged: the relative path of
// /mnt/user/appdata/nexterm IS "nexterm", so it still lands in <destBase>/nexterm.
//
// SAFETY: RestoreSubtreeTo dumps the subtree's CONTENTS directly into Target
// (no path nesting, issue #62), so two source paths mapping to the SAME Target
// would silently MERGE data. Uniqueness is therefore enforced on the destination
// ROOT folder: a residual name collision (the same container folder name coming
// from two different pools) gets a "-2"/"-3" suffix. The suffix is decided ONCE
// per SOURCE root directory and reused by every path below it, so a multi-bind
// container is never split across <destBase>/SnapOtter and <destBase>/SnapOtter-2.
// Distinct source roots always get distinct destination roots, and two paths under
// the same source root differ in their remainder, so every Target stays unique.
func (s *Service) containerAppdataRemap(destBase string, appdataPaths []string) ([]backup.RestoreDir, map[string]string) {
	base := path.Clean(destBase)
	dirs := make([]backup.RestoreDir, 0, len(appdataPaths))
	remap := make(map[string]string, len(appdataPaths))
	destRootFor := map[string]string{} // SOURCE root dir -> its assigned dest root name
	takenRoot := map[string]bool{}     // dest root names already handed out
	for _, cp := range appdataPaths {
		src := path.Clean(cp)
		root, rel := appdataRelPath(src)
		if rel == "" {
			root, rel = path.Dir(src), path.Base(src)
		}
		rootName, sub, _ := strings.Cut(rel, "/")
		srcRoot := path.Join(root, rootName)
		destRoot, ok := destRootFor[srcRoot]
		if !ok {
			destRoot = rootName
			for n := 2; takenRoot[destRoot]; n++ {
				destRoot = rootName + "-" + strconv.Itoa(n)
			}
			takenRoot[destRoot] = true
			destRootFor[srcRoot] = destRoot
		}
		dest := base + "/" + destRoot
		if sub != "" {
			dest += "/" + sub
		}
		dirs = append(dirs, backup.RestoreDir{Subtree: src, Target: dest})
		remap[s.toHostPath(src)] = s.toHostPath(dest)
	}
	return dirs, remap
}

// rewriteBinds points a recreated container's docker binds at the remapped appdata
// locations. Each bind is "HOSTPATH:CONTAINERPATH[:opts]"; only the HOST part is
// rewritten and ONLY on an EXACT match against a remap key, so appdata binds move
// to the destination while docker.sock, /etc/localtime, /dev/dri and every other
// non-appdata bind (which carry no backed-up data) are left byte-for-byte. The
// container path and option suffix (:ro,:z,…) are preserved.
func rewriteBinds(binds []string, remap map[string]string) []string {
	if len(binds) == 0 || len(remap) == 0 {
		return binds
	}
	out := make([]string, len(binds))
	for i, b := range binds {
		if host, rest, found := strings.Cut(b, ":"); found {
			// Canonicalize the bind host before the lookup: the remap keys are
			// path.Clean'd host paths, so a non-canonical bind (trailing/doubled
			// slash) must be cleaned the same way or it would be left pointing at the
			// source pool while the warning classifier (foreignBindWarnings, which
			// also cleans) silently treats it as remapped appdata.
			if nw, ok := remap[path.Clean(host)]; ok {
				out[i] = nw + ":" + rest
				continue
			}
		}
		out[i] = b
	}
	return out
}

// rewriteMountSources rewrites the Source of each captured Mount whose source is a
// remap key, for display fidelity in the recreated definition (the actual recreate
// binds come from HostConfig.Binds via rewriteBinds). Exact match; empty remap is a
// no-op.
func rewriteMountSources(mounts []model.Mount, remap map[string]string) []model.Mount {
	if len(mounts) == 0 || len(remap) == 0 {
		return mounts
	}
	out := make([]model.Mount, len(mounts))
	for i, m := range mounts {
		if nw, ok := remap[path.Clean(m.Source)]; ok {
			m.Source = nw
		}
		out[i] = m
	}
	return out
}

// dirNonEmpty reports whether p exists and contains at least one entry — the
// overwrite guard's "there is already data here" check.
func dirNonEmpty(p string) bool {
	entries, err := os.ReadDir(p)
	return err == nil && len(entries) > 0
}

// dirNonEmptyFn returns the overwrite guard's dir probe: the injected test seam
// when set, else the real filesystem check.
func (s *Service) dirNonEmptyFn() func(string) bool {
	if s.dirNonEmptyProbe != nil {
		return s.dirNonEmptyProbe
	}
	return dirNonEmpty
}

// foreignVMDestBase resolves the destination base directory (a container path)
// for a cross-instance VM restore's disks. Resolution order, NEVER the source
// pool: an explicit target subpath (the request Target) wins; else the configured
// RestoreFolder (the settings default restore location); else the platform's
// conventional local VM domains location under the host mount (see
// platform.Platform.ForeignVMDestBase — Unraid's "user/domains" share;
// generic's identity default). Target/RestoreFolder are relative subpaths
// validated by paths.Resolve (no absolute path, no traversal), exactly like
// the file-set to-folder restore.
func (s *Service) foreignVMDestBase(target string) (string, error) {
	if sub := strings.TrimSpace(target); sub != "" {
		return paths.Resolve(s.cfg.HostMountRoot, sub)
	}
	settings, err := s.store.GetSettings()
	if err != nil {
		return "", fmt.Errorf("read settings: %w", err)
	}
	if sub := strings.TrimSpace(settings.RestoreFolder); sub != "" {
		return paths.Resolve(s.cfg.HostMountRoot, sub)
	}
	return s.platformFn().ForeignVMDestBase(path.Clean(s.cfg.HostMountRoot)), nil
}

// foreignContainerDestBase resolves the destination base directory (a container
// path) a cross-instance container restore remaps appdata INTO — the container
// counterpart of foreignVMDestBase (#125). Resolution order, NEVER the source
// pool: an explicit request Target wins; else the configured RestoreFolder; else
// the platform's conventional local appdata location under the host mount (see
// platform.Platform.ForeignContainerDestBase — Unraid's "user/appdata" share;
// generic's identity default). Target / RestoreFolder are relative subpaths
// validated by paths.Resolve (no absolute path, no traversal).
func (s *Service) foreignContainerDestBase(target string) (string, error) {
	if sub := strings.TrimSpace(target); sub != "" {
		return paths.Resolve(s.cfg.HostMountRoot, sub)
	}
	settings, err := s.store.GetSettings()
	if err != nil {
		return "", fmt.Errorf("read settings: %w", err)
	}
	if sub := strings.TrimSpace(settings.RestoreFolder); sub != "" {
		return paths.Resolve(s.cfg.HostMountRoot, sub)
	}
	return s.platformFn().ForeignContainerDestBase(path.Clean(s.cfg.HostMountRoot)), nil
}

// nearestExistingDir walks up from p until it finds a directory that exists, so
// a free-space probe (statfs needs a real path) can run against the filesystem
// the restore will write into even before the leaf destination dir is created.
func nearestExistingDir(p string) string {
	for {
		if _, err := os.Stat(p); err == nil {
			return p
		}
		parent := path.Dir(p)
		if parent == p {
			return p
		}
		p = parent
	}
}

// executeRestoreVM drives the long-running (destructive) part of a VM restore
// described by an already-validated plan, publishing "vm:<name>" progress. The
// orchestrator records the run (kindRestore) itself.
func (s *Service) executeRestoreVM(ctx context.Context, name string, plan vmRestorePlan, leaveStopped bool) error {
	// Hold the domain repo lock for the whole restic/libvirt phase, INCLUDING
	// the destination pre-create below: the scheduler calls BackupVM directly
	// (bypassing batchActive by design) and the domain lock is the layer
	// scheduled jobs DO respect (see executeRestore).
	unlock := s.lockDomainFor("vms", "restore")
	defer unlock()
	// Same remap-destination handling as executeRestore (container path), and
	// for the same reason: restic.RestoreSubtreeTo (vm_orchestrator.go) is the
	// identical call the container path uses, so a cross-instance VM restore's
	// freshly created destination directory is just as root:root/0700 as a
	// container's — see #125. Pre-create readable now; the numeric owner:group
	// is restored below, after a successful restore, by healRestoreDirOwnership.
	for _, rd := range plan.restoreDirs {
		if err := paths.EnsureDirReadable(rd.Target); err != nil {
			return fmt.Errorf("restore: prepare destination %q: %w", s.toHostPath(rd.Target), err)
		}
	}
	rkey := "vm:" + name
	rctx, startedAt := s.progBegin(ctx, rkey, "restore")
	rerr := backup.RestoreVM(rctx, backup.VMRestoreDeps{
		Confirmed:    true, // prepareRestoreVM rejected unconfirmed requests
		Name:         name,
		SnapshotID:   plan.snapshotID,
		DiskPaths:    plan.diskPaths,
		RestoreDirs:  plan.restoreDirs,
		DomainXML:    plan.domainXML,
		WasAutostart: plan.wasAutostart,
		// Boot after restore iff the VM was running when backed up (nil = old backup
		// with no recorded state → boot, the historical behaviour) AND the restore
		// didn't ask to leave it stopped.
		StartAfter: (plan.wasRunning == nil || *plan.wasRunning) && !leaveStopped,
		PreDefine:  plan.preDefine,
		RepoPath:   plan.repo,
		TargetID:   plan.targetID,
		DataDir:    s.cfg.DataDir,
		VM:         s.virsh,
		Restic:     &resticAdapter{engine: s.engine, mode: plan.mode},
		Runs:       runsAdapter{st: s.store, ctx: ctx},
		BlockDisks: plan.blockDisks,
		ZFSHost:    sshZFSHost{ssh: s.ssh},
		ZvolRestic: &resticZvolAdapter{engine: s.engine, mode: plan.mode},
	})
	if rerr == nil {
		s.healRestoreDirOwnership(rctx, plan.repo, plan.snapshotID, plan.mode, plan.restoreDirs)
	}
	s.progEnd(rkey, "restore", rerr == nil, startedAt)
	return rerr
}

// StartRestoreVM launches a VM restore in a background goroutine and returns
// immediately, mirroring StartRestore for the VM domain (a VM disk restore can
// run for hours — far past any browser/proxy idle timeout). ALL validation runs
// synchronously (a bad request fails right away, no goroutine); progress is
// published under "vm:<name>" and the orchestrator records the run.
//
// Shares batchActive with backups and the other restores; returns (false, nil)
// when one is already running.
func (s *Service) StartRestoreVM(ctx context.Context, name, snapshotID, source string, leaveStopped bool) (bool, error) {
	if !s.batchActive.CompareAndSwap(false, true) {
		return false, nil
	}
	plan, err := s.prepareRestoreVM(ctx, name, snapshotID, true, source)
	if err != nil {
		s.batchActive.Store(false)
		return false, err
	}
	bctx := context.WithoutCancel(ctx)
	key := "vm:" + name // the exact progBegin key executeRestoreVM publishes under
	go func() {
		defer s.recoverOperation("restore vm: "+name, nil, func(msg string) {
			s.failStuckRun(plan.targetID, msg)
		})
		defer s.batchActive.Store(false)
		tctx, tcancel := context.WithTimeout(bctx, restoreTimeout)
		defer tcancel()
		rctx, cancel := context.WithCancel(tctx)
		defer cancel()
		s.registerCancel(key, cancel)
		defer s.unregisterCancel(key)
		if rerr := s.executeRestoreVM(rctx, name, plan, leaveStopped); rerr != nil {
			log.Printf("api: restore vm: %q failed: %v", name, rerr) //nolint:gosec // G706: name is %q-quoted
		}
	}()
	return true, nil
}

// VMSSHInfo returns the libvirt SSH host and BombVault's public key for the user
// to authorize on the Unraid host (Settings → VM Backup). Errors when SSH is not
// wired (no key yet).
func (s *Service) VMSSHInfo() (host, publicKey string, err error) {
	if s.ssh == nil {
		return "", "", errors.New("vm backup over SSH is not configured")
	}
	pub, err := s.ssh.PublicKey()
	if err != nil {
		return "", "", err
	}
	return s.cfg.LibvirtHost, pub, nil
}

// VMSSHTest checks that libvirt is reachable over SSH (used by the Settings
// "Test connection" button). Bounded by a timeout so an unreachable host
// (e.g. a macvlan container with no route) fails fast instead of hanging.
func (s *Service) VMSSHTest(ctx context.Context) error {
	if s.ssh == nil {
		return errors.New("vm backup over SSH is not configured")
	}
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	if err := s.ssh.EnsureKnownHost(ctx); err != nil {
		return err // SSH/auth/reachability problem — clearer than libvirt's error
	}
	if err := s.ssh.Test(ctx); err != nil {
		// EnsureKnownHost passed, so SSH auth + reachability are fine — only
		// libvirt is missing. Say so, so a notifications-only user (who needs the
		// SSH connection but not libvirt) isn't misled into thinking their SSH is
		// broken (#53).
		return fmt.Errorf("%w. The SSH connection itself is working, and libvirt is only needed for VM backups, not for Unraid notifications", err)
	}
	return nil
}

// LibvirtReachable reports whether libvirt is reachable over SSH, for the
// host-integration spike's (best-effort) libvirt probe. Bounded by a timeout so
// a hung SSH attempt can't stall the spike.
func (s *Service) LibvirtReachable() error {
	if s.ssh == nil {
		return errors.New("vm backup over SSH is not configured")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()
	if err := s.ssh.EnsureKnownHost(ctx); err != nil {
		return err
	}
	return s.ssh.Test(ctx)
}

// SnapshotsVM lists restic snapshots for a single VM, filtered by the
// "vm:<name>" tag the backup writes.
func (s *Service) SnapshotsVM(ctx context.Context, name, source string) ([]restic.Snapshot, error) {
	settings, err := s.store.GetSettings()
	if err != nil {
		return nil, fmt.Errorf("read settings: %w", err)
	}
	repo, err := s.repoFor(settings, "vms", source)
	if err != nil {
		return nil, err
	}
	return s.snapshotsForTag(ctx, repo, s.ModeFor(settings), "vm:"+name)
}

// resticAdapter also satisfies the flash domain's backup surface.
var _ backup.FlashRestic = (*resticAdapter)(nil)

// BackupFlash backs up the whole Unraid USB flash (the mounted /boot) to the
// flash repo via restic. Fails with a clear message if the flash directory is
// not mounted (the /boot → /host/boot mount is required for this domain).
func (s *Service) BackupFlash(ctx context.Context) (backup.Summary, error) {
	// Survive the client that triggered it disconnecting (see Backup): detach from
	// the request's cancellation with a generous hard cap.
	ctx, cancel := backupHoldCtx(ctx)
	defer cancel()
	defer s.lockDomain("flash")() // serialise per repo; blocks maintenance ops meanwhile
	settings, err := s.store.GetSettings()
	if err != nil {
		return backup.Summary{}, fmt.Errorf("read settings: %w", err)
	}
	if _, statErr := os.Stat(s.cfg.FlashDir); errors.Is(statErr, fs.ErrNotExist) {
		return backup.Summary{}, fmt.Errorf("flash backup: the Unraid flash is not mounted. Add the /boot → %s mount to the container template", s.cfg.FlashDir)
	}
	repo, err := s.flashRepoPath(settings)
	if err != nil {
		return backup.Summary{}, err
	}
	mode := s.ModeFor(settings)
	mode.Limits = s.primaryLimitsFor("flash", repo) // issue #152: a remote primary's saved bandwidth caps, else zero (unlimited)
	if err := s.EnsureRepo(ctx, repo, mode); err != nil {
		return backup.Summary{}, err
	}
	// Clear any stale lock left by a previously interrupted run so it can't block
	// this backup (BombVault is the sole writer; an active lock is never stale).
	s.unlockStale(ctx, repo, mode)
	// Healthchecks /start ping: deferred to here, past the /boot-mounted + EnsureRepo
	// guards, so the paired done/fail notifyBackup below always follows (no dangling /start).
	s.notifyBackupStart(ctx, "flash")
	fctx, startedAt := s.progBegin(ctx, "flash", "backup")
	sum, err := backup.BackupFlash(fctx, backup.FlashBackupDeps{
		SourceDir: s.cfg.FlashDir,
		Repo:      repo,
		TargetID:  store.FlashTargetID,
		Restic:    &resticAdapter{engine: s.engine, mode: mode},
		Runs:      runsAdapter{st: s.store, ctx: ctx},
	})
	s.progEnd("flash", "backup", err == nil, startedAt)
	s.notifyBackup(ctx, "flash", "", err == nil, sum, err)
	if err != nil {
		return backup.Summary{}, err
	}
	s.applyRetention(ctx, repo, settings, mode, "flash", "flash")
	makeRepoReadable(repo) // keep the local repo copyable off-box by a non-root user
	s.replicateOffsite(ctx, "flash", settings, mode, repo)
	s.maybeCollectStats(ctx, "flash")
	s.checkPrimaryRemoteBudget(ctx, "flash", repo, settings)
	if err := s.exportFlashZip(ctx, settings, sum.SnapshotID, mode, repo); err != nil {
		log.Printf("flash zip export failed (backup is still valid): %v", err)
	}
	return sum, nil
}

// flashZipRe matches ONLY the timestamped export filenames pruneFlashZips is
// allowed to delete (flash-<YYYYMMDD>-<HHMMSS>.zip, or the .age-sealed variant
// when export encryption is on). flash-latest.zip(.age) and any unrelated file the
// operator drops in the folder never match, so they survive.
var flashZipRe = regexp.MustCompile(`^flash-\d{8}-\d{6}\.zip(\.age)?$`)

// exportFlashZip writes the just-backed-up flash snapshot to the configured
// folder as a plain .zip, for off-server sync (Syncthing etc.). It is non-fatal:
// any failure is returned to the caller (BackupFlash logs it) and never fails the
// backup itself. The write is atomic — a temp file is renamed into place — so a
// sync tool never sees a half-written zip. Each attempt is recorded as a
// kind="export" run on the flash target (bytes = the written zip size) so it
// shows in the dashboard Activity Log/Run History; a disabled export records
// nothing (nothing ran).
func (s *Service) exportFlashZip(ctx context.Context, settings store.Settings, snapshotID string, mode restic.Mode, repo string) (err error) {
	if !settings.FlashZipExportEnabled || settings.FlashZipExportPath == "" {
		return nil
	}
	// Resolve age recipients up front: with export encryption on but no valid
	// recipient this fails BEFORE the temp zip is created, so no plaintext artifact
	// is ever produced when the user asked for encryption.
	recipients, _, err := s.exportRecipients(settings)
	if err != nil {
		return err
	}
	// Publish a live "maintenance" progress pair keyed "export:flash" (mirroring
	// prune:/verify:/drill:) so a long zip export of a big flash shows on the
	// dashboard activity log WHILE it writes, not only after (#109). The terminal
	// event is deferred so any failure path above clears the live line. A disabled
	// export publishes nothing (nothing ran) — hence below the guard.
	_, startedAt := s.progBegin(ctx, "export:flash", "maintenance")
	defer func() { s.progEnd("export:flash", "maintenance", err == nil, startedAt) }()
	runID, rErr := s.store.StartRun(store.FlashTargetID, "export")
	if rErr != nil {
		log.Printf("api: flash zip export: could not start run record (continuing): %v", rErr)
		runID = ""
	}
	var zipBytes int64
	defer func() {
		if runID == "" {
			return
		}
		status := "success"
		if err != nil {
			status = "failed"
		}
		if fErr := s.store.FinishRun(runID, status, "", zipBytes, truncateRunErr(err)); fErr != nil {
			log.Printf("api: flash zip export: could not finish run record: %v", fErr)
		}
	}()
	dir, err := s.flashZipExportDir(settings)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil { //nolint:gosec // G301: operator-configured sync folder must be readable by the off-server sync tool
		return fmt.Errorf("flash zip export: mkdir: %w", err)
	}
	tmp := filepath.Join(dir, ".flash-export.tmp.zip")
	f, err := os.Create(tmp) //nolint:gosec // G304: dir is an operator-configured path under the host mount root
	if err != nil {
		return fmt.Errorf("flash zip export: create temp: %w", err)
	}
	dumpErr := s.engine.DumpZip(ctx, repo, snapshotID, s.cfg.FlashDir, f, mode)
	closeErr := f.Close()
	if dumpErr != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("flash zip export: dump: %w", dumpErr)
	}
	if closeErr != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("flash zip export: close temp: %w", closeErr)
	}
	name := "flash-latest.zip"
	if settings.FlashZipExportKeep > 0 {
		name = "flash-" + time.Now().UTC().Format("20060102-150405") + ".zip"
	}
	// Publish the temp zip: plain rename, or age-encrypted to <name>.zip.age when
	// export encryption is on (the plaintext temp is removed by sealOrRename).
	final, err := sealOrRename(tmp, filepath.Join(dir, name), recipients)
	if err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("flash zip export: finalize: %w", err)
	}
	// Cheap size sample for the run record (bytes column) — best-effort only.
	if fi, statErr := os.Stat(final); statErr == nil {
		zipBytes = fi.Size()
	}
	// Prune in BOTH modes: in latest mode (Keep==0) the user opted out of history,
	// so this deletes any stale flash-<ts>.zip left over from a previous history
	// run; in history mode (Keep>0) it trims to the newest N. flash-latest.zip never
	// matches flashZipRe, so it is never touched.
	s.pruneFlashZips(dir, settings.FlashZipExportKeep)
	return nil
}

// pruneFlashZips keeps the newest `keep` timestamped flash-*.zip files, deleting
// older ones. Best-effort; only files matching the exact flash-<ts>.zip pattern
// are ever touched (flash-latest.zip and unrelated files are left alone).
func (s *Service) pruneFlashZips(dir string, keep int) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		log.Printf("flash zip export: prune: read dir: %v", err)
		return
	}
	var zips []string
	for _, e := range entries {
		if !e.IsDir() && flashZipRe.MatchString(e.Name()) {
			zips = append(zips, e.Name())
		}
	}
	sort.Sort(sort.Reverse(sort.StringSlice(zips))) // timestamp names sort chronologically → newest first
	if keep >= len(zips) {
		return
	}
	for _, name := range zips[keep:] {
		if err := os.Remove(filepath.Join(dir, name)); err != nil {
			log.Printf("flash zip export: prune: remove %q: %v", name, err)
		}
	}
}

// resticAdapter also satisfies the files domain's backup surface.
var _ backup.FilesRestic = (*resticAdapter)(nil)

// BackupFileSet backs up one file set (a named host folder under the /mnt
// mount, resolved like settings.ContainersPath) to the files repo via restic,
// tagged fileset:<Name>. It mirrors BackupFlash: no lifecycle, no defs — with
// retention, off-site replication, and stats identical to the other domains.
// A source folder that does not exist under the host mount fails with a clear
// error BEFORE any restic call, recording a failed run against the set's id so
// a scheduled backup of a vanished folder surfaces in Run History.
func (s *Service) BackupFileSet(ctx context.Context, id string) (backup.Summary, error) {
	// Survive the client that triggered it disconnecting (see Backup): detach from
	// the request's cancellation with a generous hard cap.
	ctx, cancel := backupHoldCtx(ctx)
	defer cancel()
	defer s.lockDomain("files")() // serialise per repo; blocks maintenance ops meanwhile
	settings, err := s.store.GetSettings()
	if err != nil {
		return backup.Summary{}, fmt.Errorf("read settings: %w", err)
	}
	set, err := s.store.GetFileSet(id)
	if err != nil {
		return backup.Summary{}, fmt.Errorf("files backup: load file set: %w", err)
	}
	// A set without a path cannot be backed up (Discover creates path-less,
	// disabled sets from fileset: tags alone) — say so instead of letting
	// paths.Resolve report a misleading traversal error for "".
	if strings.TrimSpace(set.Path) == "" {
		return backup.Summary{}, fmt.Errorf("files backup: file set %q has no source path configured. Set a path before backing up", set.Name)
	}
	src, err := paths.Resolve(s.cfg.HostMountRoot, set.Path)
	if err != nil {
		return backup.Summary{}, fmt.Errorf("files backup: resolve source path for %q: %w", set.Name, err)
	}
	if _, statErr := os.Stat(src); errors.Is(statErr, fs.ErrNotExist) {
		// Record the miss as a failed run so a scheduled backup of a renamed or
		// deleted folder shows up in Run History instead of failing invisibly.
		err := fmt.Errorf("files backup: source path not found for %q (%s does not exist under the host mount)", set.Name, src)
		if runID, sErr := s.store.StartRun(set.ID, "backup"); sErr != nil {
			log.Printf("api: files backup: %q: record missing-path run: %v", set.Name, sErr) //nolint:gosec // G706: name is %q-quoted
			// truncateRunErr, like every other FinishRun in this file: it is "the one
			// function that writes runs.error", and this was the only call site
			// bypassing it. Both halves matter here — the message embeds the resolved
			// host path (scrub), and a file set's name and path are never
			// length-validated at creation, so an arbitrarily long string could reach
			// runs.error uncapped and travel on into the weekly digest.
		} else if fErr := s.store.FinishRun(runID, "failed", "", 0, truncateRunErr(err)); fErr != nil {
			log.Printf("api: files backup: %q: finish missing-path run: %v", set.Name, fErr) //nolint:gosec // G706: name is %q-quoted
		}
		return backup.Summary{}, err
	}
	repo, err := s.filesRepoPath(settings)
	if err != nil {
		return backup.Summary{}, err
	}
	mode := s.ModeFor(settings)
	mode.Limits = s.primaryLimitsFor("files", repo) // issue #152: a remote primary's saved bandwidth caps, else zero (unlimited)
	if err := s.EnsureRepo(ctx, repo, mode); err != nil {
		return backup.Summary{}, err
	}
	// Clear any stale lock left by a previously interrupted run so it can't block
	// this backup (BombVault is the sole writer; an active lock is never stale).
	s.unlockStale(ctx, repo, mode)
	// Healthchecks /start ping: deferred to here, past the source-exists + EnsureRepo
	// guards, so the paired done/fail notifyBackup below always follows (no dangling /start).
	s.notifyBackupStart(ctx, "files")
	key := "files:" + set.Name
	fctx, startedAt := s.progBegin(ctx, key, "backup")
	sum, err := backup.BackupFileSetDir(fctx, backup.FileSetBackupDeps{
		SourceDir: src,
		Repo:      repo,
		TargetID:  set.ID,
		SetName:   set.Name,
		Excludes:  set.Excludes,
		Restic:    &resticAdapter{engine: s.engine, mode: mode},
		Runs:      runsAdapter{st: s.store, ctx: ctx},
	})
	s.progEnd(key, "backup", err == nil, startedAt)
	s.notifyBackup(ctx, "files", set.Name, err == nil, sum, err)
	if err != nil {
		return backup.Summary{}, err
	}
	s.applyRetention(ctx, repo, settings, mode, "fileset:"+set.Name, "files")
	makeRepoReadable(repo) // keep the local repo copyable off-box by a non-root user
	s.replicateOffsite(ctx, "files", settings, mode, repo)
	s.maybeCollectStats(ctx, "files")
	s.checkPrimaryRemoteBudget(ctx, "files", repo, settings)
	return sum, nil
}

// errFileSetNotFound is the user-safe error for an unknown file-set id (the
// raw store error would leak SQL wording through the API surface).
var errFileSetNotFound = errors.New("file set not found")

// FileSetView is the per-set row returned by ListFileSetViews — the files
// domain's counterpart of VMView. LastBackup is the unix time of the last
// successful backup run (0 = never; runs-based, so listing never spawns a
// restic process per row). PathExists reports whether the set's resolved
// source folder currently exists under the host mount — false for a vanished
// folder AND for the path-less sets DiscoverFileSets creates, so the UI can
// flag "set path before backup".
type FileSetView struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	Path       string   `json:"path"`
	Excludes   []string `json:"excludes"`
	Enabled    bool     `json:"enabled"`
	LastBackup int64    `json:"lastBackup"`
	PathExists bool     `json:"pathExists"`
}

// ListFileSetViews returns all configured file sets with their last-backup
// time and source-path existence.
func (s *Service) ListFileSetViews(_ context.Context) ([]FileSetView, error) {
	sets, err := s.store.ListFileSets()
	if err != nil {
		return nil, fmt.Errorf("list file sets: %w", err)
	}
	views := make([]FileSetView, 0, len(sets))
	for _, set := range sets {
		v := FileSetView{ID: set.ID, Name: set.Name, Path: set.Path, Excludes: set.Excludes, Enabled: set.Enabled}
		if v.Excludes == nil {
			v.Excludes = []string{}
		}
		if run, _ := s.store.LastSuccessfulBackup(set.ID); run != nil && run.FinishedAt != nil {
			v.LastBackup = *run.FinishedAt
		}
		if resolved, rErr := paths.Resolve(s.cfg.HostMountRoot, set.Path); rErr == nil {
			if _, statErr := os.Stat(resolved); statErr == nil { //nolint:gosec // G703: resolved is containment-validated under the host mount root
				v.PathExists = true
			}
		}
		views = append(views, v)
	}
	return views, nil
}

// validateFileSet guards everything a file set feeds into: the name becomes a
// restic tag ("fileset:<Name>") and a progress key, so it passes the same
// strict charset as container names (validResourceName); the path must be a
// relative subpath under the host mount (paths.Resolve containment) AND exist
// on disk at save time, so a typo fails at configuration instead of on the
// next scheduled backup. The one exception: a PATH-LESS set is valid while it
// stays DISABLED — DiscoverFileSets rebuilds sets from fileset: tags alone
// (the files domain has no mirrored definitions), where the original path is
// unknowable; such a set must remain storable/patchable, but can never be
// enabled until a real path is set.
func (s *Service) validateFileSet(fs store.FileSet) error {
	if !validResourceName(fs.Name) {
		return errors.New("invalid file set name (letters, digits, . _ - only; must start with a letter or digit)")
	}
	if strings.TrimSpace(fs.Path) == "" {
		if fs.Enabled {
			return errors.New("file set has no source path: set a path before enabling it")
		}
		return nil
	}
	resolved, err := paths.Resolve(s.cfg.HostMountRoot, fs.Path)
	if err != nil {
		return errors.New("invalid path: must be a relative subpath under the host mount")
	}
	if _, statErr := os.Stat(resolved); statErr != nil { //nolint:gosec // G703: resolved is containment-validated under the host mount root
		return errors.New("source path not found under the host mount")
	}
	return nil
}

// fileSetHasBackups reports whether the file set id already has at least one
// recorded successful backup run — i.e. fileset:<Name>-tagged snapshots exist in
// the repo. Cheap: a single indexed runs lookup, no restic call. Renaming such a
// set would silently orphan those snapshots (they stay tagged with the OLD name
// and are never re-tagged), so handlePatchFileSet refuses a name change when this
// is true (change path/excludes/enabled freely; create a new set to rename).
func (s *Service) fileSetHasBackups(ctx context.Context, id string) (bool, error) {
	run, err := s.store.LastSuccessfulBackup(id)
	if err != nil {
		return false, err
	}
	if run != nil {
		return true, nil
	}
	// The runs table alone misses a Discover-rebuilt set: it has real
	// fileset:<Name> snapshots in the repo but a fresh id with NO run rows.
	// Confirm against the repo tags so such a set can't be renamed (which would
	// strand those snapshots). A brand-new set with no repo yet lists empty
	// (localRepoMissing -> nil); a set whose share is established but unmounted
	// errors, and we then refuse the rename conservatively rather than risk
	// stranding snapshots we cannot see.
	snaps, err := s.SnapshotsFileSet(ctx, id, "local")
	if err != nil {
		if errors.Is(err, errFileSetNotFound) {
			return false, err
		}
		return true, nil
	}
	return len(snaps) > 0, nil
}

// SnapshotsFileSet lists restic snapshots for a single file set, filtered by
// the "fileset:<Name>" tag its backups write — the files counterpart of
// SnapshotsVM. id is the set's stable store id; source selects the local or
// off-site repo.
func (s *Service) SnapshotsFileSet(ctx context.Context, id, source string) ([]restic.Snapshot, error) {
	set, err := s.store.GetFileSet(id)
	if err != nil {
		return nil, errFileSetNotFound
	}
	settings, err := s.store.GetSettings()
	if err != nil {
		return nil, fmt.Errorf("read settings: %w", err)
	}
	repo, err := s.repoFor(settings, "files", source)
	if err != nil {
		return nil, err
	}
	mode := s.ModeFor(settings)
	// A listing before any backup has run is "no snapshots yet", not an error.
	if localRepoMissing(repo) {
		// #55 vs #120: only surface "not mounted" when the backing store is truly
		// absent. If the destination IS mounted, this is a fresh/phantom repo on a
		// healthy disk, so report an empty list (EnsureRepo re-establishes on write).
		if s.repoEstablished(repo) && !s.destinationMounted(repo) {
			return nil, ErrBackupPathNotMounted // #55: backing store not mounted
		}
		return nil, nil
	}
	all, err := s.listSnapshots(ctx, repo, mode)
	if err != nil {
		return nil, err
	}
	tag := "fileset:" + set.Name
	out := make([]restic.Snapshot, 0, len(all))
	for _, snap := range all {
		for _, t := range snap.Tags {
			if t == tag {
				out = append(out, snap)
				break
			}
		}
	}
	return out, nil
}

// ListSnapshotFilesFileSet lists the files in a file-set snapshot, for the
// selective (pick-some-files) restore — the files counterpart of
// ListSnapshotFiles. snapshotID must be valid hex and must belong to THIS set
// (tag-scoped via SnapshotsFileSet + snapshotBelongs), so one set's file tree
// can't be listed through another's route.
func (s *Service) ListSnapshotFilesFileSet(ctx context.Context, id, snapshotID, source string) ([]restic.FileEntry, error) {
	if !backup.ValidSnapshotID(snapshotID) {
		return nil, backup.ErrInvalidSnapshotID
	}
	snaps, err := s.SnapshotsFileSet(ctx, id, source)
	if err != nil {
		return nil, err
	}
	if !snapshotBelongs(snaps, snapshotID) {
		return nil, fmt.Errorf("snapshot %s does not belong to this file set", snapshotID)
	}
	settings, err := s.store.GetSettings()
	if err != nil {
		return nil, fmt.Errorf("read settings: %w", err)
	}
	repo, err := s.repoFor(settings, "files", source)
	if err != nil {
		return nil, err
	}
	return s.lsSelfHeal(ctx, repo, snapshotID, s.ModeFor(settings))
}

// fileSetRestorePlan carries everything prepareRestoreFileSet validated and
// resolved so the restic work can run detached from the request that asked for
// it (StartRestoreFileSet), mirroring toPathRestorePlan.
type fileSetRestorePlan struct {
	repo       string
	mode       restic.Mode
	snapshotID string
	setID      string // runs.target_id the detached run is recorded against
	setName    string // progress key suffix ("files:<name>")
	inPlace    string // in-place: the set's resolved source path (engine.RestorePath); "" = to-folder
	target     string // to-folder: the resolved alternate folder under the host mount ("" = in-place)
	subtree    string // to-folder: the SNAPSHOT's own backed-up path (Paths[0]) — the <id>:<subtree> restore root ("" = path-less snapshot → whole-tree fallback)
}

// prepareRestoreFileSet performs ALL of a file-set restore's validation and
// resolution synchronously — so a bad request fails immediately with a clear
// error — and creates the alternate target folder once containment passes.
//
// SEC: the snapshot id passes the strict hex guard (backup.ValidSnapshotID)
// and must belong to THIS set (tag-scoped via SnapshotsFileSet +
// snapshotBelongs, like prepareRestoreToPath), so one set's data can't be
// extracted through another's route. An empty targetSubPath restores IN PLACE
// over the set's source folder — destructive, therefore confirm-gated (never
// silent); a non-empty targetSubPath is resolved with paths.Resolve under the
// host mount (rejects absolute/`..` escapes) and created only AFTER
// containment passes — non-destructive, so no confirm needed (same as
// RestoreContainerToPath).
func (s *Service) prepareRestoreFileSet(ctx context.Context, id, snapshotID, source, targetSubPath string, confirm bool) (fileSetRestorePlan, error) {
	set, err := s.store.GetFileSet(id)
	if err != nil {
		return fileSetRestorePlan{}, errFileSetNotFound
	}
	if source != "local" && !isOffsiteSource(source) {
		return fileSetRestorePlan{}, errors.New("invalid source (must be local or offsite)")
	}
	if !backup.ValidSnapshotID(snapshotID) {
		return fileSetRestorePlan{}, backup.ErrInvalidSnapshotID
	}

	plan := fileSetRestorePlan{snapshotID: snapshotID, setID: set.ID, setName: set.Name}
	if sub := strings.TrimSpace(targetSubPath); sub != "" {
		// Alternate folder: shared containment helper (path.Cleans the input and
		// rejects an absolute path or any "../" that would escape the mount).
		t, rErr := paths.Resolve(s.cfg.HostMountRoot, sub)
		if rErr != nil {
			return fileSetRestorePlan{}, errors.New("invalid target folder: must be a relative subpath under the host mount")
		}
		plan.target = t
	} else {
		// In-place: writes over the set's source folder — require the explicit
		// confirmation (same sentinel discipline as prepareRestore).
		if !confirm {
			return fileSetRestorePlan{}, backup.ErrNotConfirmed
		}
		// A discovered, path-less set has no original location to restore to —
		// say so instead of letting paths.Resolve report a misleading traversal
		// error for "" (restore-to-folder works without a path).
		if strings.TrimSpace(set.Path) == "" {
			return fileSetRestorePlan{}, fmt.Errorf("file set %q has no source path configured: restore to a folder instead, or set a path first", set.Name)
		}
		src, rErr := paths.Resolve(s.cfg.HostMountRoot, set.Path)
		if rErr != nil {
			return fileSetRestorePlan{}, errors.New("invalid file set path: must be a relative subpath under the host mount")
		}
		plan.inPlace = src
	}

	// Scope to THIS set: the snapshot must be one of ITS snapshots.
	snaps, err := s.SnapshotsFileSet(ctx, id, source)
	if err != nil {
		return fileSetRestorePlan{}, err
	}
	if !snapshotBelongs(snaps, snapshotID) {
		return fileSetRestorePlan{}, fmt.Errorf("snapshot %s does not belong to this file set", snapshotID)
	}
	// Take the to-folder restore subtree from the SNAPSHOT itself (its first
	// backed-up path), NOT a recompute of set.Path: HostMountRoot may have changed
	// since the backup, and a recomputed <id>:<path> selector would then miss and
	// fail. Empty (a path-less snapshot) falls back to a whole-tree restore in
	// runRestoreFileSet; it is unused for an in-place restore.
	plan.subtree = snapshotSubtree(snaps, snapshotID)

	settings, err := s.store.GetSettings()
	if err != nil {
		return fileSetRestorePlan{}, fmt.Errorf("read settings: %w", err)
	}
	repo, err := s.repoFor(settings, "files", source)
	if err != nil {
		return fileSetRestorePlan{}, err
	}
	plan.repo = repo
	plan.mode = s.ModeFor(settings)

	// Create the alternate target dir ONLY after every validation passed. Use the
	// readable (0o755) variant: the restore target lives on a user-visible / synced
	// share, so the operator's non-root SMB user must be able to read what root
	// restored there (see EnsureDirReadable).
	if plan.target != "" {
		if err := paths.EnsureDirReadable(plan.target); err != nil {
			return fileSetRestorePlan{}, fmt.Errorf("create target folder: %w", err)
		}
	}
	return plan, nil
}

// runRestoreFileSet restores an already-validated file-set plan: in place
// (restic restores the set's source path back to its own location) or the
// whole snapshot tree into the alternate target folder.
func (s *Service) runRestoreFileSet(ctx context.Context, plan fileSetRestorePlan) error {
	// Hold the domain repo lock for the restic work: scheduled backups bypass
	// batchActive by design and the domain lock is the layer they DO respect
	// (see executeRestore).
	unlock := s.lockDomainFor("files", "restore")
	defer unlock()
	if plan.inPlace != "" {
		return s.engine.RestorePath(ctx, plan.repo, plan.snapshotID, plan.inPlace, plan.mode)
	}
	if plan.subtree != "" {
		// Restore the snapshot's OWN subtree directly INTO the chosen folder, so its
		// contents land at <target>/… — not <target>/host/user/… (issue #62's nested
		// restore, which a bare RestoreInclude("/") produces).
		return s.engine.RestoreSubtreeTo(ctx, plan.repo, plan.snapshotID, plan.subtree, plan.target, plan.mode)
	}
	// Degenerate fallback: a snapshot with no recorded path — restore the whole tree
	// rather than emit an invalid "<id>:" selector.
	return s.engine.RestoreInclude(ctx, plan.repo, plan.snapshotID, "/", plan.target, plan.mode)
}

// StartRestoreFileSet launches a file-set restore in a background goroutine
// and returns immediately (see StartRestoreToPath for why). ALL validation
// runs synchronously (a bad request fails right away, no goroutine); the
// resolved alternate target folder ("" for an in-place restore) is returned in
// the ack so the UI can show it. The detached run publishes "files:<name>"
// progress (phase "restore"), registers a cancel key, and records a run (kind
// "restore") against the set's stable id so the outcome — including the real
// restic error text — lands in the run history.
//
// Shares batchActive with backups and the other restores; returns
// ("", false, nil) when one is already running.
func (s *Service) StartRestoreFileSet(ctx context.Context, id, snapshotID, source, targetSubPath string, confirm bool) (string, bool, error) {
	if !s.batchActive.CompareAndSwap(false, true) {
		return "", false, nil
	}
	plan, err := s.prepareRestoreFileSet(ctx, id, snapshotID, source, targetSubPath, confirm)
	if err != nil {
		s.batchActive.Store(false)
		return "", false, err
	}
	bctx := context.WithoutCancel(ctx)
	rkey := "files:" + plan.setName // the exact progBegin key this restore publishes under
	go func() {
		var runID string // see StartRestoreFiles's identical goroutine for why this is declared here
		defer s.recoverOperation("restore file set: "+id, nil, func(msg string) {
			// A panic is a genuine failure, never restic.ErrRestoreMetadataOnly, so
			// finishRestoreRun directly (bypassing concludeFileSetRestore's
			// metadata-only downgrade) is the correct, simpler call here.
			s.finishRestoreRun(runID, "", errors.New(msg))
		})
		defer s.batchActive.Store(false)
		tctx, tcancel := context.WithTimeout(bctx, restoreTimeout)
		defer tcancel()
		rctx, cancel := context.WithCancel(tctx)
		defer cancel()
		s.registerCancel(rkey, cancel)
		defer s.unregisterCancel(rkey)
		runID = s.beginRestoreRunForTarget(plan.setID)
		pctx, startedAt := s.progBegin(rctx, rkey, "restore")
		rerr := s.runRestoreFileSet(pctx, plan)
		if err := s.concludeFileSetRestore(runID, rkey, plan.snapshotID, rerr, startedAt); err != nil {
			log.Printf("api: restore file set: %q failed: %v", plan.setName, err) //nolint:gosec // G706: name is %q-quoted
		}
	}()
	return plan.target, true, nil
}

// fileSetFilesRestorePlan carries everything prepareRestoreFileSetFiles validated
// and resolved so the restic work can run detached from the request that asked
// for it (StartRestoreFileSetFiles) — the selective (pick-some-files) counterpart
// of fileSetRestorePlan.
type fileSetFilesRestorePlan struct {
	repo       string
	mode       restic.Mode
	snapshotID string
	setID      string   // runs.target_id the detached run is recorded against
	setName    string   // progress key suffix ("files:<name>")
	paths      []string // cleaned selection, containment-validated
	subtree    string   // the SNAPSHOT's own backed-up root (Paths[0]); "" = path-less snapshot
	target     string   // to-folder: the resolved alternate folder under the host mount ("" = in-place)
}

// prepareRestoreFileSetFiles performs ALL of a selective file-set restore's
// validation and resolution synchronously — so a bad request fails immediately
// with a clear error — and creates the alternate target folder once containment
// passes. It is the files-domain counterpart of prepareRestoreFiles (containers).
//
// SEC: confirm-gated (like the container file-level restore); the snapshot id
// passes the strict hex guard (backup.ValidSnapshotID) and must belong to THIS
// set (tag-scoped via SnapshotsFileSet + snapshotBelongs), so one set's data
// can't be extracted through another's route. Empty targetSubPath restores IN
// PLACE (each selected path back to its absolute location, restic target "/"),
// so every path is re-validated inside the host mount (paths.Within,
// defense-in-depth). A non-empty targetSubPath is resolved with paths.Resolve
// under the host mount (rejects absolute/`..` escapes) and, when the snapshot has
// a backed-up root, every selected path must sit within that subtree (a traversal
// guard: the selection feeds --include patterns). The target dir is created
// (EnsureDirReadable, 0o755) only AFTER all containment passes.
func (s *Service) prepareRestoreFileSetFiles(ctx context.Context, id, source, snapshotID string, filePaths []string, targetSubPath string, confirm bool) (fileSetFilesRestorePlan, error) {
	if !confirm {
		return fileSetFilesRestorePlan{}, backup.ErrNotConfirmed
	}
	set, err := s.store.GetFileSet(id)
	if err != nil {
		return fileSetFilesRestorePlan{}, errFileSetNotFound
	}
	if source != "local" && !isOffsiteSource(source) {
		return fileSetFilesRestorePlan{}, errors.New("invalid source (must be local or offsite)")
	}
	// Cheap guards BEFORE the snapshot listing: a malformed id or an empty
	// selection must fail fast without a (possibly remote, slow) restic snapshots
	// call — the pre-refactor order. buildFileSetFilesPlan re-checks both, so the
	// shared path stays safe on its own.
	if !backup.ValidSnapshotID(snapshotID) {
		return fileSetFilesRestorePlan{}, backup.ErrInvalidSnapshotID
	}
	if len(filePaths) == 0 {
		return fileSetFilesRestorePlan{}, errors.New("no files selected")
	}
	// Scope to THIS set: the snapshot must be one of ITS snapshots.
	snaps, err := s.SnapshotsFileSet(ctx, id, source)
	if err != nil {
		return fileSetFilesRestorePlan{}, err
	}
	settings, err := s.store.GetSettings()
	if err != nil {
		return fileSetFilesRestorePlan{}, fmt.Errorf("read settings: %w", err)
	}
	repo, err := s.repoFor(settings, "files", source)
	if err != nil {
		return fileSetFilesRestorePlan{}, err
	}
	return s.buildFileSetFilesPlan(snaps, snapshotID, set.ID, set.Name, repo, s.ModeFor(settings), filePaths, targetSubPath)
}

// buildFileSetFilesPlan builds a validated selective plan from ALREADY resolved
// snaps + repo/mode + the set identity. It is the shared core of both the local
// (settings-driven) prepareRestoreFileSetFiles and the foreign (session-driven)
// prepareForeignFileSetFilesRestore, so the containment/target guards are written
// once and both paths inherit them.
//
// SEC: the snapshot id passes the strict hex guard (backup.ValidSnapshotID) and
// must belong to the passed snaps (tag-scoped by the caller via SnapshotsFileSet
// or snapshotsForTag), so one set's data can't be extracted through another's
// route. Empty targetSubPath restores IN PLACE (each selected path back to its
// absolute location, restic target "/"), so every path is re-validated inside the
// host mount (paths.Within, defense-in-depth). A non-empty targetSubPath is
// resolved with paths.Resolve under the host mount (rejects absolute/`..` escapes)
// and, when the snapshot has a backed-up root, every selected path must sit within
// that subtree (a traversal guard: the selection feeds --include patterns). The
// target dir is created (EnsureDirReadable, 0o755) only AFTER all containment passes.
func (s *Service) buildFileSetFilesPlan(snaps []restic.Snapshot, snapshotID, setID, setName, repo string, mode restic.Mode, filePaths []string, targetSubPath string) (fileSetFilesRestorePlan, error) {
	if !backup.ValidSnapshotID(snapshotID) {
		return fileSetFilesRestorePlan{}, backup.ErrInvalidSnapshotID
	}
	if len(filePaths) == 0 {
		return fileSetFilesRestorePlan{}, errors.New("no files selected")
	}

	// Clean each selected path once, so the path we validate is the path we run.
	cleaned := make([]string, 0, len(filePaths))
	for _, p := range filePaths {
		cleaned = append(cleaned, path.Clean(p))
	}

	if !snapshotBelongs(snaps, snapshotID) {
		return fileSetFilesRestorePlan{}, fmt.Errorf("snapshot %s does not belong to this file set", snapshotID)
	}
	// The subtree comes from the SNAPSHOT itself (its first backed-up path), NOT a
	// recompute of set.Path — HostMountRoot may have changed since the backup. A
	// "." or "/" clean means a path-less snapshot (degenerate discovered set).
	subtree := path.Clean(snapshotSubtree(snaps, snapshotID))
	if subtree == "." || subtree == "/" {
		subtree = ""
	}

	plan := fileSetFilesRestorePlan{
		repo:       repo,
		mode:       mode,
		snapshotID: snapshotID,
		setID:      setID,
		setName:    setName,
		paths:      cleaned,
		subtree:    subtree,
	}

	if sub := strings.TrimSpace(targetSubPath); sub != "" {
		t, rErr := paths.Resolve(s.cfg.HostMountRoot, sub)
		if rErr != nil {
			return fileSetFilesRestorePlan{}, errors.New("invalid target folder: must be a relative subpath under the host mount")
		}
		// When the snapshot has a backed-up root, every selection must sit within it
		// — the selection becomes an --include relative to that subtree, so a path
		// outside it would be a client trying to reach beyond the set (traversal
		// guard). A path-less snapshot has no root to scope against; the whole-path
		// include fallback in runRestoreFileSetFiles is contained by --target alone.
		if subtree != "" {
			for _, c := range cleaned {
				if c != subtree && !strings.HasPrefix(c, subtree+"/") {
					return fileSetFilesRestorePlan{}, errors.New("restore file: selected path is outside the file set snapshot")
				}
			}
		}
		plan.target = t
	} else {
		// In place writes each path back to its absolute location, so every path must
		// sit within the host mount (defense-in-depth), exactly like the container
		// in-place file restore.
		for _, c := range cleaned {
			if !paths.Within(s.cfg.HostMountRoot, c) {
				return fileSetFilesRestorePlan{}, errors.New("restore file: path is outside the backup mount")
			}
		}
	}

	// Create the alternate target dir ONLY after every validation passed — readable
	// (0o755) variant, so the operator's non-root SMB user can read what root
	// restored to the synced share (see EnsureDirReadable / the v6.0.0 files fix).
	if plan.target != "" {
		if err := paths.EnsureDirReadable(plan.target); err != nil {
			return fileSetFilesRestorePlan{}, fmt.Errorf("create target folder: %w", err)
		}
	}
	return plan, nil
}

// runRestoreFileSetFiles restores each selected path of an already-validated
// selective plan. Like runRestoreFiles (containers) it is intentionally not
// atomic — restic writes per path — so a mid-batch failure names how many
// already went through and which path stopped it.
func (s *Service) runRestoreFileSetFiles(ctx context.Context, plan fileSetFilesRestorePlan) error {
	// Hold the domain repo lock for the restic work (see runRestoreFileSet).
	unlock := s.lockDomainFor("files", "restore")
	defer unlock()
	for i, c := range plan.paths {
		if err := s.restoreOneFileSetFile(ctx, plan, c); err != nil {
			if len(plan.paths) > 1 {
				return fmt.Errorf("restored %d of %d files, then failed on %q: %w", i, len(plan.paths), c, err)
			}
			return err
		}
	}
	return nil
}

// restoreOneFileSetFile restores a single selected path of an already-validated
// plan. In place (empty target) writes it back to its absolute location
// (RestoreInclude to "/"). To a folder roots at the snapshot's subtree and
// includes the path RELATIVE to that subtree, so its contents land at
// <target>/<rel> — NOT <target>/host/user/… (issue #62's nesting). A path-less
// snapshot (no backed-up root) falls back to a plain absolute include into the
// target (the degenerate discovered-set case).
func (s *Service) restoreOneFileSetFile(ctx context.Context, plan fileSetFilesRestorePlan, sel string) error {
	if plan.target == "" {
		// In place: restore each selected path back to its own absolute location.
		return s.engine.RestoreInclude(ctx, plan.repo, plan.snapshotID, sel, "/", plan.mode)
	}
	// To a folder: root the restore at the selection's IMMEDIATE PARENT and include
	// only its basename, so the picked file or folder lands directly as
	// <target>/<name> with NO intermediate tree. Rooting at the file-set's backed-up
	// root instead (the pre-fix behaviour) recreated the whole path BETWEEN that root
	// and a deeply nested selection under the target — e.g. selecting
	// dockhand/stacks/DXP480T/xo produced <target>/stacks/DXP480T/xo instead of
	// <target>/xo (manilx, #123, 2026-08-07). restic includes ancestor path nodes,
	// so rooting at the parent is valid for files and folders at any depth.
	parent := path.Dir(sel)
	base := path.Base(sel)
	if parent == "." || parent == "/" || base == "." || base == "/" || base == "" {
		// Degenerate (no usable parent/name) — drop the selection's contents in.
		return s.engine.RestoreSubtreeTo(ctx, plan.repo, plan.snapshotID, sel, plan.target, plan.mode)
	}
	return s.engine.RestoreSubtreeInclude(ctx, plan.repo, plan.snapshotID, parent, "/"+base, plan.target, plan.mode)
}

// StartRestoreFileSetFiles launches a selective file-set restore in a background
// goroutine and returns immediately (see StartRestoreFileSet). ALL validation
// runs synchronously (a bad request fails right away, no goroutine); the resolved
// alternate target folder ("" for an in-place restore) is returned in the ack so
// the UI can show it. The detached run publishes "files:<name>" progress (phase
// "restore"), registers a cancel key, and records a run (kind "restore") against
// the set's stable id — with the same metadata-error tolerance as the whole-set
// restore (concludeFileSetRestore).
//
// Shares batchActive with backups and the other restores; returns
// ("", false, nil) when one is already running.
func (s *Service) StartRestoreFileSetFiles(ctx context.Context, id, source, snapshotID string, filePaths []string, targetSubPath string, confirm bool) (string, bool, error) {
	if !s.batchActive.CompareAndSwap(false, true) {
		return "", false, nil
	}
	plan, err := s.prepareRestoreFileSetFiles(ctx, id, source, snapshotID, filePaths, targetSubPath, confirm)
	if err != nil {
		s.batchActive.Store(false)
		return "", false, err
	}
	bctx := context.WithoutCancel(ctx)
	rkey := "files:" + plan.setName // the exact progBegin key this restore publishes under
	go func() {
		var runID string // see StartRestoreFiles's identical goroutine for why this is declared here
		defer s.recoverOperation("restore file set files: "+id, nil, func(msg string) {
			s.finishRestoreRun(runID, "", errors.New(msg)) // see StartRestoreFileSet for why not concludeFileSetRestore
		})
		defer s.batchActive.Store(false)
		tctx, tcancel := context.WithTimeout(bctx, restoreTimeout)
		defer tcancel()
		rctx, cancel := context.WithCancel(tctx)
		defer cancel()
		s.registerCancel(rkey, cancel)
		defer s.unregisterCancel(rkey)
		runID = s.beginRestoreRunForTarget(plan.setID)
		pctx, startedAt := s.progBegin(rctx, rkey, "restore")
		rerr := s.runRestoreFileSetFiles(pctx, plan)
		if err := s.concludeFileSetRestore(runID, rkey, plan.snapshotID, rerr, startedAt); err != nil {
			log.Printf("api: restore file set files: %q failed: %v", plan.setName, err) //nolint:gosec // G706: name is %q-quoted
		}
	}()
	return plan.target, true, nil
}

// DeleteBackupsFileSet removes ALL backups of a file set in one go — every
// restic snapshot tagged fileset:<Name>, pruning the freed data — and forgets
// the set from the store (row + run history), mirroring DeleteBackupsVM's
// local-source behaviour. The repo is shared by all sets, so only this set's
// tagged snapshots are forgotten; prune never touches data still referenced by
// other sets' snapshots. Serialised against files backups via the domain lock,
// and stale locks are cleared first (so it can't fail on a leftover lock).
func (s *Service) DeleteBackupsFileSet(ctx context.Context, id string) error {
	settings, repo, err := s.domainRepo("files")
	if err != nil {
		return err
	}
	// Issue #152: refused when this repo IS a remote primary flagged append-only
	// in its saved safety settings (same gate as pruneDomain/DeleteSnapshot/
	// DeleteBackupsVM) — this function has no source parameter, so it always
	// targets the primary/local repo and only the primary half of the gate
	// applies (there is no separate off-site source to check here). This path
	// runs Forget with prune=true, so skipping it here would have let a
	// compromised on-box credential irreversibly reclaim space on an immutable
	// primary.
	if s.primaryIsImmutable("files", repo) {
		return errOffsiteAppendOnly
	}
	if err := s.requireExistingRepo(repo, "no backups to delete yet"); err != nil {
		return err
	}
	unlock, ok := s.tryLockDomainFor("files", "delete")
	if !ok {
		return errDomainBusy
	}
	defer unlock()
	mode := s.ModeFor(settings)
	s.unlockStale(ctx, repo, mode)

	// Collect this set's snapshot IDs (tag-filtered fileset:<Name>) and
	// forget+prune them in one restic call (Forget with prune=true).
	snaps, err := s.SnapshotsFileSet(ctx, id, "local")
	if err != nil {
		return err
	}
	ids := make([]string, 0, len(snaps))
	for _, snap := range snaps {
		ids = append(ids, snap.ID)
	}
	if len(ids) > 0 {
		if err := s.engine.Forget(ctx, repo, ids, true, mode); err != nil {
			return fmt.Errorf("forget snapshots: %w", err)
		}
	}

	// Drop the set row + its run history so the set disappears from the list
	// once its backups are gone.
	if err := s.store.DeleteFileSet(id); err != nil {
		return fmt.Errorf("delete file set: %w", err)
	}
	return nil
}

// DiscoverFileSets rebuilds the file-set list from backup storage — the files
// counterpart of Discover/DiscoverVMs, used after a fresh install / database
// loss. Unlike containers/VMs the files domain mirrors NO definitions to the
// repo (there is nothing to recreate beyond the folder's content), so
// discovery works from the fileset:<Name> snapshot tags alone: every unknown
// name is stored as a DISABLED, PATH-LESS set — the original source path is
// unknowable from tags, so the UI flags "set path before backup" while
// restore-to-folder already works. Existing sets are never touched (their
// path/excludes/enabled state is user configuration). Returns the number of
// file sets found in the repo. dryRun makes it READ-ONLY (list + count, write
// nothing) — used by the Recovery readability probe so it never resurrects
// orphan entries (#44).
func (s *Service) DiscoverFileSets(ctx context.Context, dryRun bool) (int, error) {
	settings, repo, err := s.domainRepo("files")
	if err != nil {
		return 0, err
	}
	// Discover targets the primary (local) repo; the local config check is correct
	// here and preserves the quiet "0 discovered" for a not-yet-created repo.
	if _, statErr := os.Stat(filepath.Join(repo, "config")); errors.Is(statErr, fs.ErrNotExist) { //nolint:gosec // G703: repo is the operator-configured local domain path, validated under the mount root on save
		return 0, nil // no repo yet → nothing to discover
	}
	mode := s.ModeFor(settings)
	snaps, err := s.listSnapshots(ctx, repo, mode)
	if err != nil {
		return 0, err
	}

	names := map[string]bool{}
	for _, snap := range snaps {
		for _, tag := range snap.Tags {
			if rest, ok := strings.CutPrefix(tag, "fileset:"); ok && rest != "" {
				names[rest] = true
			}
		}
	}

	discovered := 0
	for name := range names {
		// Defense-in-depth: only our own backups write fileset: tags, but a name
		// that fails the boundary charset (it feeds tags + progress keys) is
		// skipped rather than stored.
		if !validResourceName(name) {
			log.Printf("api: discover files: skipping unsafe file set name %q", name) //nolint:gosec // G706: %q-quoted
			continue
		}
		if dryRun {
			discovered++ // probe: count what a real discover would surface, write nothing
			continue
		}
		if _, gErr := s.store.GetFileSetByName(name); gErr == nil {
			discovered++ // already configured — never clobber user configuration
			continue
		}
		if _, cErr := s.store.CreateFileSet(store.FileSet{Name: name, Path: "", Enabled: false}); cErr != nil {
			log.Printf("api: discover files: could not create set %q: %v", name, cErr) //nolint:gosec // G706: %q-quoted
			continue
		}
		discovered++
	}
	return discovered, nil
}

// resticAdapter also satisfies the config domain's backup surface.
var _ backup.ConfigRestic = (*resticAdapter)(nil)

// BackupConfig backs up BombVault's own /config folder (the settings DB +
// rclone.conf + ssh/ keypair) to the config repo via restic. Unlike flash it
// never hands restic the live folder: it first stages a consistent, restic-ready
// snapshot (VACUUM-INTO of the WAL-mode DB + verbatim static files) and always
// removes that snapshot afterwards, so a rebuilt Unraid box can recover BombVault
// itself with no container stop.
func (s *Service) BackupConfig(ctx context.Context) (backup.Summary, error) {
	// Survive the client that triggered it disconnecting (see Backup): detach from
	// the request's cancellation with a generous hard cap.
	ctx, cancel := backupHoldCtx(ctx)
	defer cancel()
	defer s.lockDomain("config")() // serialise per repo; blocks maintenance ops meanwhile
	settings, err := s.store.GetSettings()
	if err != nil {
		return backup.Summary{}, fmt.Errorf("read settings: %w", err)
	}
	// Build the consistent staging snapshot of /config; restic backs THIS up, never
	// the live WAL-mode DB. Always removed afterwards so the snapshot never lingers.
	stagingDir, err := s.stageConfigSnapshot()
	if err != nil {
		return backup.Summary{}, err
	}
	defer func() { _ = os.RemoveAll(stagingDir) }()
	repo, err := s.configRepoPath(settings)
	if err != nil {
		return backup.Summary{}, err
	}
	mode := s.ModeFor(settings)
	mode.Limits = s.primaryLimitsFor("config", repo) // issue #152: a remote primary's saved bandwidth caps, else zero (unlimited)
	if err := s.EnsureRepo(ctx, repo, mode); err != nil {
		return backup.Summary{}, err
	}
	// Clear any stale lock left by a previously interrupted run so it can't block
	// this backup (BombVault is the sole writer; an active lock is never stale).
	s.unlockStale(ctx, repo, mode)
	// Healthchecks /start ping: deferred to here, past staging + EnsureRepo guards,
	// so the paired done/fail notifyBackup below always follows (no dangling /start).
	s.notifyBackupStart(ctx, "config")
	fctx, startedAt := s.progBegin(ctx, "config", "backup")
	sum, err := backup.BackupConfig(fctx, backup.ConfigBackupDeps{
		SourceDir: stagingDir,
		Repo:      repo,
		TargetID:  store.ConfigTargetID,
		Restic:    &resticAdapter{engine: s.engine, mode: mode},
		Runs:      runsAdapter{st: s.store, ctx: ctx},
	})
	s.progEnd("config", "backup", err == nil, startedAt)
	s.notifyBackup(ctx, "config", "", err == nil, sum, err)
	if err != nil {
		return backup.Summary{}, err
	}
	s.applyRetention(ctx, repo, settings, mode, "config", "config")
	s.replicateOffsite(ctx, "config", settings, mode, repo)
	s.maybeCollectStats(ctx, "config")
	s.checkPrimaryRemoteBudget(ctx, "config", repo, settings)
	return sum, nil
}

// FlashDownloadName is the suggested filename for a flash zip download.
func FlashDownloadName(id string) string { return "flash-" + id + ".zip" }

// resolveFlashSnapshot maps a user-supplied selector ("" / "latest", a full id,
// or a short prefix) to the single matching full snapshot id. It errors when the
// selector matches none OR is an ambiguous prefix of more than one — so the
// caller rejects it BEFORE any download bytes/headers are committed, and restic
// always receives an unambiguous full id.
func resolveFlashSnapshot(snaps []restic.Snapshot, selector string) (string, error) {
	if len(snaps) == 0 {
		return "", errors.New("flash has not been backed up yet")
	}
	if selector == "" || selector == "latest" {
		return snaps[len(snaps)-1].ID, nil
	}
	var match string
	for _, s := range snaps {
		if s.ID == selector {
			return s.ID, nil // exact id wins outright
		}
		if strings.HasPrefix(s.ID, selector) {
			if match != "" {
				return "", errors.New("ambiguous snapshot id")
			}
			match = s.ID
		}
	}
	if match == "" {
		return "", errors.New("snapshot not found")
	}
	return match, nil
}

// DownloadFlashZip streams a flash snapshot to w as a zip (restic dump), the
// non-destructive replacement for the old extract-to-folder restore: the live
// /boot is never touched, no filesystem metadata is restored (so it can't hit
// the per-file permission errors a to-disk restore caused on /mnt/user), and —
// via dumpFlashZipCompat — the file drops straight into the Unraid USB creator
// (restic's own zip dump does not: bombvault#136).
//
// "latest"/"" resolves to the newest snapshot; an explicit id is validated
// against the repo. onResolved (optional) is called with the concrete id once it
// is known-good and BEFORE streaming begins, so the HTTP handler can set the
// download headers only on the happy path. A restore run is recorded for history.
func (s *Service) DownloadFlashZip(ctx context.Context, snapshotID, source string, onResolved func(id string), w io.Writer) error {
	settings, err := s.store.GetSettings()
	if err != nil {
		return fmt.Errorf("read settings: %w", err)
	}
	// Resolve age recipients up front: with export encryption on but no valid
	// recipient the download fails BEFORE onResolved fires (no headers) and before
	// any bytes are streamed, so a plaintext zip is never sent.
	recipients, encOn, err := s.exportRecipients(settings)
	if err != nil {
		return err
	}
	repo, err := s.repoFor(settings, "flash", source)
	if err != nil {
		return err
	}
	mode := s.ModeFor(settings)
	snaps, err := s.engine.Snapshots(ctx, repo, mode)
	if err != nil {
		return err
	}
	id, err := resolveFlashSnapshot(snaps, snapshotID)
	if err != nil {
		return err
	}
	if onResolved != nil {
		onResolved(id)
	}
	runID, err := s.store.StartRun(store.FlashTargetID, "restore")
	if err != nil {
		return fmt.Errorf("flash download: start run: %w", err)
	}
	// When encryption is on, wrap the response writer so the streamed zip is
	// age-sealed on the fly. The age writer MUST be closed to finalize the stream.
	dst := w
	var ageW io.WriteCloser
	if encOn {
		ageW, err = ageseal.WrapWriter(w, recipients)
		if err != nil {
			_ = s.store.FinishRun(runID, "failed", "", 0, err.Error())
			return err
		}
		dst = ageW
	}
	if derr := s.dumpFlashZipCompat(ctx, repo, id, s.cfg.FlashDir, dst, mode); derr != nil {
		// A client disconnect / user cancel of the download is context.Canceled —
		// record it as "cancelled", not a failure.
		status, msg := "failed", derr.Error()
		if errors.Is(derr, context.Canceled) {
			status, msg = "cancelled", "cancelled by user"
		}
		_ = s.store.FinishRun(runID, status, "", 0, msg)
		return derr
	}
	if ageW != nil {
		if cerr := ageW.Close(); cerr != nil { // flush + finalize the age stream
			_ = s.store.FinishRun(runID, "failed", "", 0, cerr.Error())
			return cerr
		}
	}
	_ = s.store.FinishRun(runID, "success", id, 0, "")
	return nil
}

// ExportEncryptionOn reports whether the plain-export age encryption is enabled
// (best-effort; a settings read error reports false). The flash-download handler
// uses it to append the ".age" suffix to the Content-Disposition filename before
// streaming begins.
func (s *Service) ExportEncryptionOn() bool {
	settings, err := s.store.GetSettings()
	if err != nil {
		return false
	}
	return settings.ExportEncryptEnabled
}

// SnapshotsFlash lists restic snapshots in the flash repo (the repo is dedicated
// to flash, so all of its snapshots are flash backups).
func (s *Service) SnapshotsFlash(ctx context.Context, source string) ([]restic.Snapshot, error) {
	settings, err := s.store.GetSettings()
	if err != nil {
		return nil, fmt.Errorf("read settings: %w", err)
	}
	repo, err := s.repoFor(settings, "flash", source)
	if err != nil {
		return nil, err
	}
	mode := s.ModeFor(settings)
	if localRepoMissing(repo) {
		// #55 vs #120: only surface "not mounted" when the backing store is truly
		// absent. If the destination IS mounted, this is a fresh/phantom repo on a
		// healthy disk, so report an empty list (EnsureRepo re-establishes on write).
		if s.repoEstablished(repo) && !s.destinationMounted(repo) {
			return nil, ErrBackupPathNotMounted // #55: backing store not mounted
		}
		return nil, nil // no backups yet
	}
	return s.listSnapshots(ctx, repo, mode)
}

// resolveConfigSnapshot maps a user-supplied selector ("" / "latest", a full id,
// or a short prefix) to the single matching full snapshot id in the config repo.
// It is resolveFlashSnapshot with a config-worded empty message: the config repo
// is dedicated to BombVault's own /config snapshots, so an empty repo means the
// app has never backed itself up yet.
func resolveConfigSnapshot(snaps []restic.Snapshot, selector string) (string, error) {
	if len(snaps) == 0 {
		return "", errors.New("BombVault's configuration has not been backed up yet")
	}
	if selector == "" || selector == "latest" {
		return snaps[len(snaps)-1].ID, nil
	}
	var match string
	for _, s := range snaps {
		if s.ID == selector {
			return s.ID, nil // exact id wins outright
		}
		if strings.HasPrefix(s.ID, selector) {
			if match != "" {
				return "", errors.New("ambiguous snapshot id")
			}
			match = s.ID
		}
	}
	if match == "" {
		return "", errors.New("snapshot not found")
	}
	return match, nil
}

// SnapshotsConfig lists restic snapshots in the config repo (the repo is
// dedicated to the config self-backup, so all of its snapshots are config
// backups). Mirrors SnapshotsFlash.
func (s *Service) SnapshotsConfig(ctx context.Context, source string) ([]restic.Snapshot, error) {
	settings, err := s.store.GetSettings()
	if err != nil {
		return nil, fmt.Errorf("read settings: %w", err)
	}
	repo, err := s.repoFor(settings, "config", source)
	if err != nil {
		return nil, err
	}
	mode := s.ModeFor(settings)
	if localRepoMissing(repo) {
		// #55 vs #120: only surface "not mounted" when the backing store is truly
		// absent. If the destination IS mounted, this is a fresh/phantom repo on a
		// healthy disk, so report an empty list (EnsureRepo re-establishes on write).
		if s.repoEstablished(repo) && !s.destinationMounted(repo) {
			return nil, ErrBackupPathNotMounted // #55: backing store not mounted
		}
		return nil, nil // no backups yet
	}
	return s.listSnapshots(ctx, repo, mode)
}

// RestoreConfig STAGES a restore of BombVault's own /config: it cannot overwrite
// the live SQLite settings DB in place while this process holds it open (WAL), so
// it restic-restores the chosen snapshot into a staging root and writes a marker.
// The boot-time selfrestore.ApplyPending (called from main BEFORE store.Open on
// the next restart) performs the file-level staging→live swap. The restart is
// triggered separately (docker self-restart / manual), so this call only stages.
func (s *Service) RestoreConfig(ctx context.Context, snapshotID, source string) error {
	settings, err := s.store.GetSettings()
	if err != nil {
		return fmt.Errorf("read settings: %w", err)
	}
	repo, err := s.repoFor(settings, "config", source)
	if err != nil {
		return err
	}
	mode := s.ModeFor(settings)
	snaps, err := s.engine.Snapshots(ctx, repo, mode)
	if err != nil {
		return err
	}
	id, err := resolveConfigSnapshot(snaps, snapshotID)
	if err != nil {
		return err
	}
	root := selfrestore.StagingRoot(s.cfg.DataDir)
	if err := os.RemoveAll(root); err != nil {
		return fmt.Errorf("config restore: clear staging: %w", err)
	}
	// Also clear any stale <root>.bad left by a failed restore on a prior boot: it
	// contains a plaintext rclone.conf + ssh private key, so a fresh attempt should
	// not let it linger. Best-effort — a leftover .bad must never block a restore.
	_ = os.RemoveAll(root + ".bad")
	runID, err := s.store.StartRun(store.ConfigTargetID, "restore")
	if err != nil {
		return fmt.Errorf("config restore: start run: %w", err)
	}
	// Restore ONLY the config snapshot's subtree (<DataDir>/.snapshot) into the
	// staging root; restic recreates that absolute path beneath --target, landing
	// it at selfrestore.RestoredSnapshotDir(DataDir) — the exact path the boot swap
	// reads. The swap applies it on the next restart.
	if rerr := s.engine.RestoreInclude(ctx, repo, id, s.configSnapshotDir(), root, mode); rerr != nil {
		_ = s.store.FinishRun(runID, "failed", "", 0, rerr.Error())
		return rerr
	}
	if merr := selfrestore.WriteMarker(s.cfg.DataDir); merr != nil {
		_ = s.store.FinishRun(runID, "failed", "", 0, merr.Error())
		return merr
	}
	_ = s.store.FinishRun(runID, "success", id, 0, "")
	return nil
}

// StartRestoreConfig stages a restore of BombVault's own /config and, on success,
// triggers the self-restart that applies it on the next boot. It takes the shared
// single-flight guard (batchActive) so it can never overlap another backup/restore
// — a config self-restart would otherwise kill the container mid-write of an
// in-flight data restore. Returns (started, autoRestart, err): started=false with a
// nil err means another operation is already running; autoRestart=false means the
// caller must ask the user to restart the container manually. When an auto-restart
// IS scheduled the guard is held until the container goes down (so nothing new can
// start in the restart window); if the restart later fails, ScheduleSelfRestart
// releases the guard so operations can resume.
//
// Unlike the other Start* restores in this file, this one does NOT hand the actual
// restore off to a detached background goroutine and return immediately: the
// caller (Recovery.tsx's restore-own-config step) needs the real staged/autoRestart
// outcome synchronously to decide whether to poll for the self-restart or show the
// manual-restart instructions, and a fire-and-forget response here would make the
// SPA wait on a restart that may never come if the (now invisible) background
// restore fails. Instead it borrows RunRestoreDrill's approach: RestoreConfig still
// runs on THIS goroutine, but against a context detached from ctx
// (context.WithoutCancel) and capped by restoreTimeout — so a browser tab close or
// reverse-proxy idle timeout on the HTTP request can no longer reach into restic
// and kill a still-in-flight restore mid-write. This does NOT mean a truncated
// restore corrupts the live config on the next boot's swap: selfrestore.ApplyPending
// is marker-gated and runs validSQLite (PRAGMA quick_check) on the staged DB BEFORE
// touching the live one, moving a bad/truncated staged DB to <root>.bad and leaving
// the live DB untouched — the swap was never blind. The real, narrower residual risk
// (pre-existing, not introduced by this fix) is that RestoreConfig clears the
// staging dir on entry but never clears a stale marker: a successful restore #1
// (autoRestart=false, so the marker sits pending until a manual restart) followed
// by a FAILED restore #2 can leave that old marker pointing at restore #2's partial
// staging. validSQLite still catches a truncated bombvault.sqlite, but not a
// complete DB sitting beside a truncated rclone.conf or a partial ssh/ directory —
// ApplyPending only checks fileExists/dirExists for those, not their content, so
// that combination would still get swapped in. This fix reduces how often that
// window is hit (fewer restores now get killed mid-write) but doesn't close it;
// closing it fully is out of scope here. Separately: on a client disconnect during
// a config restore, the restore now runs to completion and self-restarts
// server-side, but the SPA's fetch rejects into its error branch and reports a
// failure to the user anyway — a real but minor reporting gap (not a correctness
// bug), also out of scope: closing it would mean moving outcome reporting off the
// synchronous HTTP response (e.g. polling or SSE), a bigger refactor than detaching
// the context. A panic anywhere in this call graph is now recovered like every
// other manual op on this branch: since RestoreConfig's own run-record id is local
// to it (never returned here), the fallback is FailRunningRun keyed by
// store.ConfigTargetID — the same fallback StartBackupConfig's sibling goroutine
// already uses for this domain.
func (s *Service) StartRestoreConfig(ctx context.Context, snapshotID, source string) (started bool, autoRestart bool, err error) {
	if !s.batchActive.CompareAndSwap(false, true) {
		return false, false, nil
	}
	if op, busy := s.domainBusy("config"); busy {
		s.batchActive.Store(false)
		return false, false, fmt.Errorf("%s is running on config", op)
	}
	// On a recovered panic the guard must ALSO be released here: unlike the
	// success path below, nothing else is left running that would release it, and
	// every future backup/restore would otherwise refuse forever with "already
	// running".
	defer s.recoverOperation("restore config: "+store.ConfigTargetID, &err, func(msg string) {
		s.batchActive.Store(false)
		s.failStuckRun(store.ConfigTargetID, msg)
	})
	bctx := context.WithoutCancel(ctx)
	rctx, cancel := context.WithTimeout(bctx, restoreTimeout)
	defer cancel()
	if rerr := s.RestoreConfig(rctx, snapshotID, source); rerr != nil {
		s.batchActive.Store(false)
		return false, false, rerr
	}
	autoRestart = s.ScheduleSelfRestart()
	if !autoRestart {
		// No auto-restart scheduled (Docker self unreachable): let normal operations
		// resume. The staged restore applies on the next manual boot and does not
		// affect anything running now.
		s.batchActive.Store(false)
	}
	// autoRestart: keep the guard held; ScheduleSelfRestart's goroutine releases it
	// if the restart call fails.
	return true, autoRestart, nil
}

// SetVMMethod updates the backup method for a VM, creating the target if absent.
func (s *Service) SetVMMethod(_ context.Context, name, method string) error {
	if _, err := s.store.GetVMTargetByName(name); err != nil {
		if _, uErr := s.store.UpsertVMTarget(store.VMTarget{Name: name, Method: method}); uErr != nil {
			return fmt.Errorf("ensure vm target: %w", uErr)
		}
		return nil
	}
	return s.store.SetVMMethod(name, method)
}

// SetVMInclude updates the include_in_schedule flag for a VM, creating the
// target if absent.
func (s *Service) SetVMInclude(_ context.Context, name string, include bool) error {
	if _, err := s.store.GetVMTargetByName(name); err != nil {
		if _, uErr := s.store.UpsertVMTarget(store.VMTarget{Name: name, Method: "graceful"}); uErr != nil {
			return fmt.Errorf("ensure vm target: %w", uErr)
		}
	}
	return s.store.SetVMInclude(name, include)
}

// SetVMScheduleCadence sets a VM's per-item schedule override (#121), creating the
// target if absent. The cadence is validated with the domain-schedule grammar; an
// empty string clears the override. everyN is rejected (no per-item last-run gate),
// exactly like SetScheduleCadence.
func (s *Service) SetVMScheduleCadence(_ context.Context, name, cadence string) error {
	cadence = strings.TrimSpace(cadence)
	if cadence != "" {
		cad, err := schedule.ParseCadence(cadence)
		if err != nil {
			return fmt.Errorf("invalid schedule: %w", err)
		}
		if cad.IntervalDays > 0 {
			return fmt.Errorf("per-item schedules do not support 'everyN': use 'off', 'daily HH:MM', 'weekly DOW HH:MM', or a cron expression")
		}
	}
	if _, err := s.store.GetVMTargetByName(name); err != nil {
		if _, uErr := s.store.UpsertVMTarget(store.VMTarget{Name: name, Method: "graceful"}); uErr != nil {
			return fmt.Errorf("ensure vm target: %w", uErr)
		}
	}
	return s.store.SetVMScheduleCadence(name, cadence)
}

// SetVMIncludeAll sets the include_in_schedule flag for EVERY known VM in one
// call — the VM counterpart to SetIncludeAll. It iterates the live VMs reported
// by virsh and ensures a target row exists for each (find-or-create, exactly as
// SetVMInclude does), then applies the same flag to every already-known VM
// target (so an orphan VM that still has backups is toggled too). De-duplicated
// so a VM that is both live and a known target is only set once.
func (s *Service) SetVMIncludeAll(ctx context.Context, include bool) error {
	infos, err := s.virsh.List(ctx)
	if err != nil {
		return fmt.Errorf("list vms: %w", err)
	}
	seen := make(map[string]bool, len(infos))
	for _, vm := range infos {
		if err := s.SetVMInclude(ctx, vm.Name, include); err != nil {
			return err
		}
		seen[vm.Name] = true
	}
	// Known targets whose VM is no longer defined on the host (orphans with
	// backups) — the find-or-create in SetVMInclude already handles existing
	// rows, so a plain store update is enough here.
	targets, err := s.store.ListVMTargets()
	if err != nil {
		return fmt.Errorf("list vm targets: %w", err)
	}
	for _, t := range targets {
		if seen[t.Name] {
			continue
		}
		if err := s.store.SetVMInclude(t.Name, include); err != nil {
			return err
		}
	}
	return nil
}

// SetContainerHooks stores the pre/post-backup hook commands for a container.
func (s *Service) SetContainerHooks(_ context.Context, name, preHook, postHook string) error {
	return s.store.SetHooks(name, preHook, postHook)
}

// SetUpdateAfterBackup toggles the post-backup image update for a container (#52).
func (s *Service) SetUpdateAfterBackup(_ context.Context, name string, updateAfterBackup bool) error {
	return s.store.SetUpdateAfterBackup(name, updateAfterBackup)
}

// SetStopContainers stores the other container names to stop during this
// container's backup. Names are trimmed + de-duplicated; blanks are dropped.
func (s *Service) SetStopContainers(_ context.Context, name string, stop []string) error {
	var clean []string
	seen := map[string]bool{}
	for _, c := range stop {
		c = strings.TrimSpace(c)
		if c == "" || c == name || seen[c] {
			continue // skip blanks, self, and duplicates
		}
		seen[c] = true
		clean = append(clean, c)
	}
	return s.store.SetStopContainers(name, clean)
}

// BackupOrders returns the current explicit manual backup ordering (#119): the
// containers with a positive order, sorted by order ascending.
func (s *Service) BackupOrders(_ context.Context) ([]store.ContainerOrder, error) {
	return s.store.BackupOrders()
}

// SetBackupOrders authoritatively replaces the manual backup ordering (#119) from
// an ordered list of container names: the first name gets order 1, the next 2, and
// so on, and every container not in the list is returned to unordered. Blanks and
// duplicates (first occurrence wins) are dropped so the positions stay dense.
func (s *Service) SetBackupOrders(_ context.Context, names []string) error {
	orders := make([]store.ContainerOrder, 0, len(names))
	seen := map[string]bool{}
	for _, n := range names {
		n = strings.TrimSpace(n)
		if n == "" || seen[n] {
			continue // skip blanks and duplicates
		}
		seen[n] = true
		orders = append(orders, store.ContainerOrder{Container: n, Order: len(orders) + 1})
	}
	return s.store.SetBackupOrders(orders)
}

// VMBackupOrders returns the current explicit VM backup ordering (#119, VMs): the
// VMs with a positive order, sorted by order ascending.
func (s *Service) VMBackupOrders(_ context.Context) ([]store.VMOrder, error) {
	return s.store.VMBackupOrders()
}

// SetVMBackupOrders authoritatively replaces the VM backup ordering (#119, VMs)
// from an ordered list of VM names: the first name gets order 1, the next 2, and so
// on, and every VM not in the list is returned to unordered. Blanks and duplicates
// (first occurrence wins) are dropped so the positions stay dense.
func (s *Service) SetVMBackupOrders(_ context.Context, names []string) error {
	orders := make([]store.VMOrder, 0, len(names))
	seen := map[string]bool{}
	for _, n := range names {
		n = strings.TrimSpace(n)
		if n == "" || seen[n] {
			continue // skip blanks and duplicates
		}
		seen[n] = true
		orders = append(orders, store.VMOrder{VM: n, Order: len(orders) + 1})
	}
	return s.store.SetVMBackupOrders(orders)
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

// CheckDomain verifies the integrity of a domain's restic repo (restic check).
// domain is "containers" | "vms" | "flash" | "files". Returns a friendly error
// when the repo has not been created yet. Bounded by a timeout so a huge repo
// can't hang the request forever.
func (s *Service) CheckDomain(ctx context.Context, domain, source string) (err error) {
	settings, repo, err := s.domainRepoSource(domain, source)
	if err != nil {
		return err
	}
	if err := s.requireExistingRepo(repo, "no backups to verify yet"); err != nil {
		return err
	}
	// Hold the in-process domain lock for the whole verify so no other BombVault op
	// (backup / prune / replicate) runs against this repo while we check it. If one
	// already holds it, report a clean "busy" instead of colliding on restic's repo
	// lock. This rules out BombVault itself as the source of a lock check hits, but
	// NOT a genuinely live restic process (e.g. a manual/external invocation) — that
	// case is a real, live lock, not an orphan, and is deliberately left alone below
	// (waited out by --retry-lock in the engine, never force-removed).
	unlock, ok := s.tryLockDomainFor(domain, "verify")
	if !ok {
		return errDomainBusy
	}
	defer unlock()
	ctx, cancel := context.WithTimeout(ctx, 15*time.Minute)
	defer cancel()
	mode := s.ModeFor(settings)

	// Publish a "maintenance" progress pair (begin/terminal, indeterminate — restic
	// check streams no percentage) and record a "verify" run, so a manual/scheduled
	// verify shows up on the dashboard activity log/run history instead of running
	// invisibly.
	vkey := "verify:" + domain
	_, startedAt := s.progBegin(ctx, vkey, "maintenance")
	defer func() { s.progEnd(vkey, "maintenance", err == nil, startedAt) }()
	runID, rErr := s.store.StartRun(domainRunTargetID(domain), "verify")
	if rErr != nil {
		log.Printf("api: verify %s: could not start run record (continuing): %v", domain, rErr) //nolint:gosec // G706: domain is a fixed literal
		runID = ""
	}
	defer func() {
		if runID == "" {
			return
		}
		status := "success"
		if err != nil {
			status = "failed"
		}
		if fErr := s.store.FinishRun(runID, status, "", 0, truncateRunErr(err)); fErr != nil {
			log.Printf("api: verify %s: could not finish run record: %v", domain, fErr) //nolint:gosec // G706: domain is a fixed literal
		}
	}()

	// Clear a GENUINE stale orphan before `restic check` takes its lock: unlockStale
	// runs plain `restic unlock`, which removes only locks restic itself deems stale
	// (a dead PID on THIS host, or any lock past restic's ~30-min age threshold). A
	// live/concurrent lock is never force-removed: we hold the domain lock for the
	// whole verify, so no other BombVault op can collide, and `restic check` passes
	// --retry-lock to wait out a transient cross-process lock instead of racing it.
	// KNOWN BOUNDED GAP: an orphan from a PRIOR container incarnation carries that
	// container's random hostname, so restic can't PID-probe it and won't call it
	// stale until it is ~30 min old; until then check fails "already locked" (it
	// self-heals, or a manual Unlock clears it). A stable container hostname closes
	// this — see the repo-lock-serialization plan.
	s.unlockStale(ctx, repo, mode)
	err = s.engine.Check(ctx, repo, mode)
	return err
}

// drillSubsetPct clamps the configured drill subset percentage into restic's
// valid 1..100 range, defaulting an unset/zero value to 5.
func drillSubsetPct(pct int) int {
	if pct <= 0 {
		return 5
	}
	if pct > 100 {
		return 100
	}
	return pct
}

// RunRestoreDrill runs a restore-verification drill of the requested kind and
// records the result so the UI can show a "last verified restorable" badge.
// kind "subset" (or "") is the classic in-place integrity check; kind "dr" is a
// real off-site sandbox restore (see runDRDrill). domain is
// {containers,vms,flash,config,files}; source is {local,offsite} (ignored for
// kind "dr", which is always off-site).
// wait selects the lock discipline for the underlying drill: a SCHEDULED drill
// passes wait=true so it BLOCKS for the per-domain lock and always records a
// result (a nightly backup/replication co-fire must not make it silently vanish
// → dashboard "never"); a MANUAL drill passes wait=false for immediate
// errDomainBusy feedback, recording nothing (#30).
func (s *Service) RunRestoreDrill(ctx context.Context, domain, source, kind string, wait bool) (store.RestoreDrill, error) {
	switch kind {
	case "", "subset":
		return s.runSubsetDrill(ctx, domain, source, wait)
	case "dr":
		return s.runDRDrill(ctx, domain, source, wait)
	default:
		return store.RestoreDrill{}, fmt.Errorf("unknown drill kind %q", kind)
	}
}

// runSubsetDrill proves a domain's backup is actually restorable by running
// `restic check --read-data-subset` (it reads back + re-verifies a random subset
// of the real pack data, not just metadata — no scratch disk needed) and records
// the result. domain is {containers,vms,flash,config,files}; source is
// {local,offsite}.
//
// It takes the per-domain busy-guard like Prune/Unlock: if a backup is running it
// returns errDomainBusy and records nothing. A missing/empty repo returns a clear
// "no backups to verify" error and records nothing (no misleading failure). Both
// a passing and a failing drill ARE recorded; a failure also fires a notification.
func (s *Service) runSubsetDrill(ctx context.Context, domain, source string, wait bool) (drill store.RestoreDrill, err error) {
	switch domain {
	case "containers", "vms", "flash", "config", "files":
	default:
		return store.RestoreDrill{}, fmt.Errorf("unknown domain %q", domain)
	}
	switch {
	case source == "local", isOffsiteSource(source):
	default:
		return store.RestoreDrill{}, fmt.Errorf("unknown source %q", source)
	}

	settings, repo, err := s.domainRepoSource(domain, source)
	if err != nil {
		return store.RestoreDrill{}, err
	}
	if err := s.requireExistingRepo(repo, "no backups to verify yet"); err != nil {
		return store.RestoreDrill{}, err
	}

	// Serialise with backups (and other maintenance) so a drill never reads a repo a
	// backup is actively writing. A SCHEDULED drill (wait) BLOCKS for the domain so it
	// always records a result even when a nightly backup/replication co-fires; a
	// MANUAL drill fails fast with immediate busy feedback WITHOUT recording (#30).
	var unlock func()
	if wait {
		// Bounded wait: poll the lock up to drillLockWait, then LOG + skip (record
		// nothing, like the manual-busy path) so a wedged lock-holder can't block a
		// scheduled drill forever or pile up a goroutine each night.
		u, ok := s.waitLockDomainFor(domain, "verify")
		if !ok {
			log.Printf("api: drill: %q busy longer than %v, skipping this scheduled run", domain, drillLockWait) //nolint:gosec // G706: domain is %q-quoted and validated to a fixed allow-list above
			// Record the skip as a dated failed row (instead of silently returning) so
			// the dashboard shows WHY the check did not run rather than freezing the
			// previous red with no reason (#30).
			skip := store.RestoreDrill{
				Domain: domain,
				Source: source,
				Kind:   "subset",
				At:     time.Now().Unix(),
				OK:     false,
				Detail: "skipped: repository busy longer than " + drillLockWait.String() + " (a backup or off-site copy held it)",
			}
			if aErr := s.store.AddRestoreDrill(skip); aErr != nil {
				log.Printf("api: drill: record busy-skip for %q: %v", domain, aErr) //nolint:gosec // G706: domain is %q-quoted and validated above
			}
			s.recordDomainRun(domain, "drill", false, skip.Detail)
			s.notifyDrillFailure(ctx, domain, source, skip.Detail)
			return skip, errDomainBusy
		}
		unlock = u
	} else {
		u, ok := s.tryLockDomainFor(domain, "verify")
		if !ok {
			return store.RestoreDrill{}, errDomainBusy
		}
		unlock = u
	}
	defer unlock()

	// Publish a live "maintenance" progress pair keyed "drill:<domain>" (mirroring
	// prune:/verify:) so a running restore-verification drill shows on the
	// dashboard activity log WHILE it reads back pack data, not only after it
	// finished (#109). The terminal event is deferred so an error/panic can never
	// leave a stuck live line.
	dkey := "drill:" + domain
	_, startedAt := s.progBegin(ctx, dkey, "maintenance")
	defer func() { s.progEnd(dkey, "maintenance", err == nil, startedAt) }()

	mode := s.ModeFor(settings)
	// An initialised-but-empty repo (no snapshots) has nothing to verify. Treat it
	// like a missing repo: a clear error, no misleading failure recorded.
	snaps, err := s.listSnapshots(ctx, repo, mode)
	if err != nil {
		return store.RestoreDrill{}, err
	}
	if len(snaps) == 0 {
		return store.RestoreDrill{}, errors.New("no backups to verify yet")
	}

	// Clear any stale lock a previously interrupted off-site op (replication copy /
	// integrity check) left behind before `restic check --read-data-subset` takes its
	// lock, so a drill can't fail "repository is already locked" — BombVault is the
	// sole writer, so an existing lock is always stale (mirrors CheckDomain; #29).
	s.unlockStale(ctx, repo, mode)
	// Reading back a subset of real pack data can be slow on a large repo; bound it.
	dctx, cancel := context.WithTimeout(ctx, 2*time.Hour)
	defer cancel()
	checkErr := s.engine.CheckData(dctx, repo, drillSubsetPct(settings.DrillsSubsetPct), mode)

	drill = store.RestoreDrill{
		Domain: domain,
		Source: source,
		At:     time.Now().Unix(),
		OK:     checkErr == nil,
		Kind:   "subset",
	}
	if checkErr != nil {
		drill.Detail = scrubError(checkErr)
		if len(drill.Detail) > 200 {
			drill.Detail = drill.Detail[:200]
		}
	}
	if recErr := s.store.AddRestoreDrill(drill); recErr != nil {
		// Recording is the whole point of a drill; surface a record failure.
		return store.RestoreDrill{}, fmt.Errorf("record drill: %w", recErr)
	}
	// Mirror the drill outcome into the shared runs table so it shows in the
	// dashboard Activity Log/Run History (the restore_drills row above stays the
	// badge/scorecard source of truth).
	s.recordDomainRun(domain, "drill", drill.OK, drill.Detail)
	// A failed restorability check is important — notify on failure (best-effort).
	if checkErr != nil {
		s.notifyDrillFailure(ctx, domain, source, drill.Detail)
	}
	return drill, checkErr
}

// drillMarkerName is the sentinel file written into a DR-drill sandbox at
// creation. Cleanup deletes a sandbox ONLY when this marker is present in that
// exact directory — a safety interlock so a drill can never os.RemoveAll a path
// that is not a marked drill sandbox we created.
const drillMarkerName = ".bombvault-drill"

// drillByteToleranceFloor is the ONLY slack allowed between restic's reported
// restore-size and the on-disk restore of a DR drill: a tight few-KB absolute
// floor for filesystem metadata rounding — NOT a percentage of the total.
// `restic restore` is content-addressed, so a completed restore reproduces the
// exact logical bytes; a percentage band (e.g. 5% of the whole snapshot) would
// wave through a large file restored truncated by less than that fraction, with
// the file count unchanged. The count must match exactly and the bytes to within
// this floor.
const drillByteToleranceFloor = 4096

// drillSnapshotTimeout bounds the DR-drill's off-site snapshot listing so a
// black-holed off-site (a `restic snapshots` that hangs on a dead network) can't
// hold the domain lock indefinitely and starve a concurrent scheduled backup. The
// restore itself is bounded separately (restoreTimeout), matching a real restore.
const drillSnapshotTimeout = 15 * time.Minute

// errNothingToDrill signals that the newest off-site snapshot has no restorable
// file data (0 files / 0 bytes — e.g. a definition-only / stateless container). A
// DR drill then records NOTHING: a green would be a false "verified restorable"
// and a red a false failure, so the scorecard must green/red neither.
var errNothingToDrill = errors.New("no restorable file data in the newest off-site snapshot: nothing to drill")

// runDRDrill performs a real off-site disaster-recovery drill for a domain: it
// restores the newest off-site snapshot of the drill target into a marker-guarded
// sandbox under the restore folder, verifies the restored file count + bytes
// against restic's own accounting, deletes the sandbox (marker-guarded), and
// records a restore_drills(kind='dr', source='offsite') row. It takes the domain
// repo lock exactly like a real restore, so a scheduled backup can never fire
// mid-drill and vice-versa; busy → errDomainBusy, recording nothing. A failure
// records kind='dr' ok=false AND fires the drill-failure notification.
func (s *Service) runDRDrill(ctx context.Context, domain, source string, wait bool) (drill store.RestoreDrill, err error) {
	switch domain {
	case "containers", "vms", "flash", "files":
	default:
		return store.RestoreDrill{}, fmt.Errorf("unknown domain %q", domain)
	}
	// A DR drill only ever restores from an off-site repo. A non-offsite source
	// (e.g. the scheduler's legacy call, or "local") normalises to the bare
	// "offsite" PRIMARY so behaviour is byte-identical to before; an "offsite:<id>"
	// source drills — and records under — that SPECIFIC destination.
	if !isOffsiteSource(source) {
		source = "offsite"
	}

	settings, err := s.store.GetSettings()
	if err != nil {
		return store.RestoreDrill{}, fmt.Errorf("read settings: %w", err)
	}
	target, ok := s.offsiteTargetForSource(settings, domain, source)
	if !ok {
		return store.RestoreDrill{}, errors.New("no off-site repo configured for this domain")
	}
	repo, err := s.resolveRepo(target.Repo)
	if err != nil {
		return store.RestoreDrill{}, err
	}

	// Serialise with backups/restores on the domain repo — a scheduled backup must
	// never fire mid-drill and vice-versa. A SCHEDULED drill (wait) BLOCKS for the
	// domain so it always records a result even when a nightly op co-fires; a MANUAL
	// drill fails fast with immediate busy feedback WITHOUT recording (#30).
	var unlock func()
	if wait {
		// Bounded wait: poll the lock up to drillLockWait, then LOG + skip (record
		// nothing, like the manual-busy path) so a wedged lock-holder can't block a
		// scheduled drill forever or pile up a goroutine each night.
		u, ok := s.waitLockDomainFor(domain, "verify")
		if !ok {
			log.Printf("api: drill: %q busy longer than %v, skipping this scheduled run", domain, drillLockWait) //nolint:gosec // G706: domain is %q-quoted and validated to a fixed allow-list above
			// Record the skip as a dated failed row (instead of silently returning) so
			// the dashboard shows WHY the off-site DR check did not run rather than
			// freezing the red with no reason (#30).
			skip := store.RestoreDrill{
				Domain: domain,
				Source: source,
				Kind:   "dr",
				At:     time.Now().Unix(),
				OK:     false,
				Detail: "skipped: repository busy longer than " + drillLockWait.String() + " (a backup or off-site copy held it)",
			}
			if aErr := s.store.AddRestoreDrill(skip); aErr != nil {
				log.Printf("api: drill: record busy-skip for %q: %v", domain, aErr) //nolint:gosec // G706: domain is %q-quoted and validated above
			}
			s.recordDomainRun(domain, "drdrill", false, skip.Detail)
			s.notifyDrillFailure(ctx, domain, source, skip.Detail)
			return skip, errDomainBusy
		}
		unlock = u
	} else {
		u, ok := s.tryLockDomainFor(domain, "verify")
		if !ok {
			return store.RestoreDrill{}, errDomainBusy
		}
		unlock = u
	}
	defer unlock()

	// Publish a live "maintenance" progress pair keyed "drdrill:<domain>"
	// (mirroring prune:/verify:) so a running DR drill shows on the dashboard
	// activity log WHILE it sandbox-restores, not only after it finished (#109).
	// The "drdrill" key/kind is deliberately distinct from the local subset
	// drill's "drill", so the log tells the off-site DR restore check apart from
	// the local read-back check. The terminal event is deferred so an error/panic
	// can never leave a stuck live line.
	dkey := "drdrill:" + domain
	_, startedAt := s.progBegin(ctx, dkey, "maintenance")
	defer func() { s.progEnd(dkey, "maintenance", err == nil, startedAt) }()

	// Detach from the request/scheduler ctx for the whole drill: a real DR restore
	// can take hours over a slow off-site link, and a browser tab close (request
	// ctx) or a context.Background scheduler parent must not abort it. The snapshot
	// listing is then bounded (drillSnapshotTimeout) so a black-holed off-site can't
	// hold the domain lock forever; the restore is bounded by restoreTimeout inside
	// sandboxRestoreVerify.
	drillCtx := context.WithoutCancel(ctx)
	mode := s.ModeFor(settings)
	listCtx, listCancel := context.WithTimeout(drillCtx, drillSnapshotTimeout)
	snapID, err := s.pickDRSnapshot(listCtx, domain, settings, repo, mode)
	listCancel()
	if err != nil {
		return store.RestoreDrill{}, err
	}

	// Clear any stale lock a previously interrupted off-site op left behind before the
	// sandbox restore takes its lock, so a drill can't fail "repository is already
	// locked" — BombVault is the sole writer, so an existing lock is always stale
	// (mirrors CheckDomain; #29). drillCtx (detached) matches the restore below.
	s.unlockStale(drillCtx, repo, mode)
	// Restore into the sandbox, verify, and clean up (marker-guarded). The outcome
	// is recorded either way; a failure also notifies. An empty (0-file/0-byte)
	// snapshot records NOTHING — neither a false green nor a false red.
	drillErr := s.sandboxRestoreVerify(drillCtx, domain, settings, repo, snapID, mode)
	if errors.Is(drillErr, errNothingToDrill) {
		return store.RestoreDrill{}, drillErr
	}
	drill = store.RestoreDrill{
		Domain: domain,
		Source: source,
		At:     time.Now().Unix(),
		OK:     drillErr == nil,
		Kind:   "dr",
	}
	if drillErr != nil {
		drill.Detail = scrubError(drillErr)
		if len(drill.Detail) > 200 {
			drill.Detail = drill.Detail[:200]
		}
	}
	if recErr := s.store.AddRestoreDrill(drill); recErr != nil {
		return store.RestoreDrill{}, fmt.Errorf("record drill: %w", recErr)
	}
	// Mirror the drill outcome into the shared runs table so it shows in the
	// dashboard Activity Log/Run History (the restore_drills row above stays the
	// badge/scorecard source of truth). Kind "drdrill" — distinct from the local
	// subset drill's "drill" — so the log names the off-site DR restore check.
	s.recordDomainRun(domain, "drdrill", drill.OK, drill.Detail)
	if drillErr != nil {
		s.notifyDrillFailure(ctx, domain, source, drill.Detail)
	}
	return drill, drillErr
}

// pickDRSnapshot resolves the newest off-site snapshot to drill for a domain.
// containers: the DRDrillTarget container (or, when unset, the most recently
// backed-up container), scoped to its container:<name> tag. vms: the
// DRDrillTargetVM VM (or, when unset, the most recently backed-up VM), scoped to
// its vm:<name> tag — same pattern as containers. flash: the newest snapshot
// outright (flash is a single whole-USB image, no per-item scoping). files
// follows the flash path: the newest snapshot in the files repo outright — a
// file-set restore is sandbox-cheap and any set proves the repo restorable. An
// empty repo or a target with no off-site snapshot yields a clear error.
func (s *Service) pickDRSnapshot(ctx context.Context, domain string, settings store.Settings, repo string, mode restic.Mode) (string, error) {
	all, err := s.listSnapshots(ctx, repo, mode)
	if err != nil {
		return "", err
	}
	if len(all) == 0 {
		return "", errors.New("no off-site backups to drill yet")
	}
	switch domain {
	case "flash", "files":
		return newestSnapshot(all).ID, nil
	case "containers", "vms":
		var (
			target string
			tagPfx string
		)
		if domain == "containers" {
			target, tagPfx = settings.DRDrillTarget, "container:"
			if target == "" {
				target, err = s.newestBackedUpContainer()
			}
		} else {
			target, tagPfx = settings.DRDrillTargetVM, "vm:"
			if target == "" {
				target, err = s.newestBackedUpVM()
			}
		}
		if err != nil {
			return "", err
		}
		tag := tagPfx + target
		var scoped []restic.Snapshot
		for _, snap := range all {
			for _, t := range snap.Tags {
				if t == tag {
					scoped = append(scoped, snap)
					break
				}
			}
		}
		if len(scoped) == 0 {
			return "", fmt.Errorf("no off-site snapshot for drill target %q", target)
		}
		return newestSnapshot(scoped).ID, nil
	default:
		return "", fmt.Errorf("unknown domain %q", domain)
	}
}

// newestSnapshot returns the snapshot with the latest Time (RFC3339 sorts
// chronologically as a string). snaps must be non-empty.
func newestSnapshot(snaps []restic.Snapshot) restic.Snapshot {
	best := snaps[0]
	for _, sn := range snaps[1:] {
		if sn.Time > best.Time {
			best = sn
		}
	}
	return best
}

// newestBackedUpContainer returns the container name with the most recent
// successful backup run — the default DR-drill target when none is pinned.
func (s *Service) newestBackedUpContainer() (string, error) {
	targets, err := s.store.ListTargets()
	if err != nil {
		return "", fmt.Errorf("list targets: %w", err)
	}
	best := ""
	var bestAt int64
	for _, t := range targets {
		run, rErr := s.store.LastSuccessfulBackup(t.ID)
		if rErr != nil {
			return "", rErr
		}
		if run == nil || run.FinishedAt == nil {
			continue
		}
		if *run.FinishedAt >= bestAt {
			bestAt = *run.FinishedAt
			best = t.ContainerName
		}
	}
	if best == "" {
		return "", errors.New("no backed-up container to drill")
	}
	return best, nil
}

// newestBackedUpVM returns the VM name with the most recent successful backup
// run — the default DR-drill target when none is pinned. Mirrors
// newestBackedUpContainer exactly (VM runs share the same runs table/column,
// see migration 5 runs_relax_fk).
func (s *Service) newestBackedUpVM() (string, error) {
	targets, err := s.store.ListVMTargets()
	if err != nil {
		return "", fmt.Errorf("list VM targets: %w", err)
	}
	best := ""
	var bestAt int64
	for _, t := range targets {
		run, rErr := s.store.LastSuccessfulBackup(t.ID)
		if rErr != nil {
			return "", rErr
		}
		if run == nil || run.FinishedAt == nil {
			continue
		}
		if *run.FinishedAt >= bestAt {
			bestAt = *run.FinishedAt
			best = t.Name
		}
	}
	if best == "" {
		return "", errors.New("no backed-up VM to drill")
	}
	return best, nil
}

// sandboxRestoreVerify restores the whole snapshot tree into a fresh
// marker-guarded sandbox under the restore folder, verifies the restored files +
// bytes against restic's own accounting, and (always) attempts marker-guarded
// cleanup. A mismatch or restore/verify error is returned; the sandbox is still
// removed. Reuses the SAME RestoreInclude machinery + paths.Resolve containment as
// a real restore-to-folder.
func (s *Service) sandboxRestoreVerify(ctx context.Context, domain string, settings store.Settings, repo, snapID string, mode restic.Mode) error {
	sub := path.Join(settings.RestoreFolder, fmt.Sprintf("bombvault-drill-%s-%d", domain, time.Now().UnixNano()))
	sandbox, err := paths.Resolve(s.cfg.HostMountRoot, sub)
	if err != nil {
		return errors.New("invalid restore folder: must be a relative subpath under the host mount")
	}
	// Create the parent (restore folder) then the sandbox LEAF with os.Mkdir, which
	// FAILS if it already exists — a positive assertion that this is a fresh dir of
	// ours before it becomes a marker-guarded RemoveAll target (MkdirAll would
	// silently adopt a pre-existing directory).
	if err := paths.EnsureDir(filepath.Dir(sandbox)); err != nil {
		return fmt.Errorf("create drill sandbox parent: %w", err)
	}
	if err := os.Mkdir(sandbox, 0o700); err != nil { //nolint:gosec // G703: sandbox is resolved strictly under the host mount root by paths.Resolve
		return fmt.Errorf("create drill sandbox: %w", err)
	}
	// Marker FIRST — before any restore — so the cleanup interlock can always
	// confirm this is a sandbox we created, even if the restore fails midway. If the
	// marker write itself fails the (still empty) dir would leak, so remove it
	// explicitly on that path before the cleanup defer is even registered.
	markerPath := filepath.Join(sandbox, drillMarkerName)
	if err := os.WriteFile(markerPath, []byte("bombvault dr drill\n"), 0o600); err != nil { //nolint:gosec // G306: marker is a non-secret sentinel; 0600 is already restrictive
		if rmErr := os.Remove(sandbox); rmErr != nil { //nolint:gosec // G703: sandbox is resolved strictly under the host mount root by paths.Resolve (rejects absolute/traversal); it was just created empty by os.Mkdir above
			log.Printf("api: dr-drill: could not remove sandbox after marker-write failure: %v", rmErr)
		}
		return fmt.Errorf("write drill marker: %w", err)
	}
	defer func() {
		if cErr := cleanupDrillSandbox(sandbox); cErr != nil {
			log.Printf("api: dr-drill: cleanup: %v", cErr)
		}
	}()

	// Bound the restore at restoreTimeout, matching a real restore — reading a whole
	// snapshot back over a slow off-site link can take many hours, far more than a
	// short drill-only deadline. ctx is already detached from the request by the
	// caller (runDRDrill), so a browser tab close can't abort a legitimate drill.
	dctx, cancel := context.WithTimeout(ctx, restoreTimeout)
	defer cancel()

	// A VM disk image (or any large snapshot) can be hundreds of GB; fail fast with a
	// clear error if the sandbox filesystem doesn't have room, rather than letting the
	// restore run the host out of disk mid-way. Same preflight idiom as
	// guardVMRestoreDestination/guardContainerRestoreDestination: a stats/probe error
	// (e.g. the unsupported-platform stub in diskfree_other.go) is "cannot prove
	// insufficient" and never blocks the drill — only a PROVEN shortfall aborts.
	if _, wantBytes, statErr := s.engine.StatsRestoreSize(dctx, repo, snapID, mode); statErr == nil && wantBytes > 0 {
		if free, fErr := s.diskFreeFn()(sandbox); fErr == nil && free < uint64(wantBytes) {
			return fmt.Errorf("not enough free space to sandbox-restore: it needs %d bytes but %q has only %d free. Free up space and retry", wantBytes, settings.RestoreFolder, free)
		}
	}

	if err := s.engine.RestoreInclude(dctx, repo, snapID, "/", sandbox, mode); err != nil {
		return fmt.Errorf("restore into sandbox: %w", err)
	}

	// Verify: restic's own file count (ls) + restore-size bytes+files (stats) vs an
	// on-disk walk of the sandbox (the marker is excluded from the walk).
	lsEntries, err := s.engine.Ls(dctx, repo, snapID, mode)
	if err != nil {
		return fmt.Errorf("list snapshot: %w", err)
	}
	lsFiles := 0
	for _, e := range lsEntries {
		if e.Type == "file" {
			lsFiles++
		}
	}
	statsFiles, wantBytes, err := s.engine.StatsRestoreSize(dctx, repo, snapID, mode)
	if err != nil {
		return fmt.Errorf("snapshot stats: %w", err)
	}
	// A snapshot with no restorable file data exercises no real restore — recording
	// it green would be a false "verified restorable". Signal "nothing to drill" so
	// the caller records neither green nor red.
	if lsFiles == 0 && wantBytes == 0 {
		return errNothingToDrill
	}
	gotFiles, gotBytes, err := walkDrillSandbox(sandbox)
	if err != nil {
		return fmt.Errorf("walk sandbox: %w", err)
	}
	if !drillVerifyOK(lsFiles, gotFiles, wantBytes, gotBytes) {
		return fmt.Errorf("verification mismatch: restic ls %d files, stats %d files / %d bytes; restored sandbox %d files / %d bytes", lsFiles, statsFiles, wantBytes, gotFiles, gotBytes)
	}
	return nil
}

// walkDrillSandbox counts the regular files and their total bytes under a drill
// sandbox, excluding the .bombvault-drill marker at its root. Only regular files
// count toward restore-size (dirs/symlinks/devices are ignored, matching restic).
func walkDrillSandbox(sandbox string) (files int, bytes int64, err error) {
	marker := filepath.Join(sandbox, drillMarkerName)
	err = filepath.WalkDir(sandbox, func(p string, d fs.DirEntry, walkErr error) error { //nolint:gosec // G703: sandbox is resolved strictly under the host mount root by paths.Resolve (rejects absolute/traversal); read-only walk
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() || p == marker {
			return nil
		}
		info, iErr := d.Info()
		if iErr != nil {
			return iErr
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		files++
		bytes += info.Size()
		return nil
	})
	return files, bytes, err
}

// drillVerifyOK reports whether a restored sandbox matches restic's own
// accounting. The completeness proof is the ON-DISK restore versus restic:
//
//   - the on-disk file count == restic ls file count (a completed restore
//     materialises every file node restic recorded);
//   - the on-disk bytes == restic's restore-size bytes to within only
//     drillByteToleranceFloor (a few KB for fs metadata), NOT a percentage — a
//     content-addressed restore reproduces the exact logical bytes.
//
// NOTE: restic's `stats --mode restore-size` file COUNT is deliberately NOT
// compared against `ls` here. The two restic counters legitimately differ on
// real snapshots (e.g. hardlinks, and how restore-size tallies files vs how ls
// enumerates nodes), so requiring statsFiles == lsFiles produced false
// "verification mismatch" failures on perfectly restorable backups — e.g. the
// Unraid flash restoring the exact file count and bytes yet being flagged (#30).
// A truncated restore is already caught by gotFiles != lsFiles and the byte
// check, both of which measure the ACTUAL restored sandbox; statsFiles measures
// neither, so it can only add false negatives.
func drillVerifyOK(lsFiles, gotFiles int, statsBytes, gotBytes int64) bool {
	if gotFiles != lsFiles {
		return false
	}
	diff := statsBytes - gotBytes
	if diff < 0 {
		diff = -diff
	}
	return diff <= drillByteToleranceFloor
}

// cleanupDrillSandbox removes a DR-drill sandbox, but ONLY after confirming the
// .bombvault-drill marker written at creation is present in that exact directory.
// This is a safety-critical interlock: os.RemoveAll is destructive, so a drill
// must never delete a path that is not a marked sandbox (e.g. a mis-resolved or
// operator-configured folder). A missing marker removes nothing and returns an
// error.
func cleanupDrillSandbox(sandbox string) error {
	if _, err := os.Stat(filepath.Join(sandbox, drillMarkerName)); err != nil { //nolint:gosec // G703: sandbox is resolved strictly under the host mount root by paths.Resolve; this stat is the marker interlock itself
		return fmt.Errorf("drill sandbox %q lacks the %s marker; refusing to delete", filepath.Base(sandbox), drillMarkerName)
	}
	return os.RemoveAll(sandbox) //nolint:gosec // G703: sandbox is under the host mount root (paths.Resolve) AND guarded above by the .bombvault-drill marker — never removes a non-drill path
}

// LatestDrill returns the most recent restore-verification drill for a domain +
// source (a thin passthrough to the store). found is false when none ran yet.
func (s *Service) LatestDrill(domain, source string) (store.RestoreDrill, bool, error) {
	return s.store.LatestRestoreDrill(domain, source)
}

// Drills returns the recorded restore-verification drills for a domain + source
// (newest first), a thin passthrough to the store.
func (s *Service) Drills(domain, source string, limit int) ([]store.RestoreDrill, error) {
	return s.store.ListRestoreDrills(domain, source, limit)
}

// notifyDrillFailure sends a best-effort notification when a restore-verification
// drill fails (the backup is NOT provably restorable). Mirrors notifyBackup's
// policy + Unraid fan-out; a no-op when notifications are off.
func (s *Service) notifyDrillFailure(ctx context.Context, domain, source, detail string) {
	c, err := s.NotifyConfig()
	if err != nil || c.On == "" || c.On == "never" {
		return
	}
	target := "Unraid flash"
	if domain != "flash" {
		target = domain
	}
	msg := fmt.Sprintf("Restore verification of %s (%s) FAILED — the backup may not be restorable: %s", target, source, detail)
	notify.Send(ctx, c, domain, notify.Event{Title: "BombVault", Message: msg, OK: false})
	if s.unraidGate(c.Unraid) {
		if e := s.sendUnraidNotify(ctx, "BombVault: restore verification FAILED", msg, "warning"); e != nil {
			log.Printf("notify: unraid: %v", e)
		}
	}
}

// repoFor resolves the restic repo path for a domain ("containers"|"vms"|
// "flash"|"config"|"files") and source. An off-site source selects the
// configured off-site repo (erroring if none is set); anything else ("" /
// "local") selects the primary local repo. This lets browse/restore/maintenance
// operate on either copy. The off-site source is either the bare "offsite" (the
// domain's PRIMARY target — the same repo as today) or "offsite:<id>" (a specific
// target); offsiteTargetForSource does the parsing so no caller pattern-matches
// the literal.
func (s *Service) repoFor(settings store.Settings, domain, source string) (string, error) {
	if isOffsiteSource(source) {
		target, ok := s.offsiteTargetForSource(settings, domain, source)
		if !ok {
			return "", errors.New("no off-site repo configured for this domain")
		}
		return s.resolveRepo(target.Repo)
	}
	switch domain {
	case "containers":
		return s.containersRepoPath(settings)
	case "vms":
		return s.vmsRepoPath(settings)
	case "flash":
		return s.flashRepoPath(settings)
	case "config":
		return s.configRepoPath(settings)
	case "files":
		return s.filesRepoPath(settings)
	default:
		return "", fmt.Errorf("unknown domain %q", domain)
	}
}

// domainRepo resolves the primary (local) restic repo path for a domain.
func (s *Service) domainRepo(domain string) (store.Settings, string, error) {
	return s.domainRepoSource(domain, "local")
}

// domainRepoSource is domainRepo with an explicit source ("local"|"offsite"),
// returning the settings alongside the resolved repo so callers don't re-read.
func (s *Service) domainRepoSource(domain, source string) (store.Settings, string, error) {
	settings, err := s.store.GetSettings()
	if err != nil {
		return store.Settings{}, "", fmt.Errorf("read settings: %w", err)
	}
	repo, err := s.repoFor(settings, domain, source)
	return settings, repo, err
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

// recordDomainRun persists an ALREADY-COMPLETED domain-scoped check (restore
// drill / tamper test) as a runs row on the reserved domain target id, so it
// shows up in the dashboard Activity Log/Run History like prune/verify runs do.
// The operation has finished by the time this is called, so the row is opened
// and closed back-to-back (its own history table carries the full timing).
// detail is bounded to the same cap as truncateRunErr. Best-effort: a store
// error is logged and never fails the check that already ran.
func (s *Service) recordDomainRun(domain, kind string, ok bool, detail string) {
	runID, err := s.store.StartRun(domainRunTargetID(domain), kind)
	if err != nil {
		log.Printf("api: %s %s: could not start run record (continuing): %v", kind, domain, err) //nolint:gosec // G706: kind and domain are fixed literals
		return
	}
	status := "success"
	if !ok {
		status = "failed"
	}
	const maxDetail = 500 // mirror truncateRunErr's cap for the runs.error column
	if len(detail) > maxDetail {
		detail = detail[:maxDetail]
	}
	if err := s.store.FinishRun(runID, status, "", 0, detail); err != nil {
		log.Printf("api: %s %s: could not finish run record: %v", kind, domain, err) //nolint:gosec // G706: kind and domain are fixed literals
	}
}

// localRepoMissing reports whether a LOCAL repo has not been initialised yet (no
// `config` marker). It is ALWAYS false for a remote repo (rest:/s3:/b2:/…),
// which has no local marker to stat — its emptiness is decided by actually
// listing it. This is why the off-site view (often a remote repo) must not use a
// local config check, or it would always look empty even when snapshots exist.
func localRepoMissing(repo string) bool {
	if restic.IsRemoteRepo(repo) {
		return false
	}
	_, statErr := os.Stat(filepath.Join(repo, "config")) //nolint:gosec // G703: repo is an operator-configured location validated under the mount root on save; source only selects which configured location
	return errors.Is(statErr, fs.ErrNotExist)
}

// requireExistingRepo returns a friendly error (notYet) when a local repo has not
// been initialised yet. Remote repos are assumed to exist (no cheap local check).
func (s *Service) requireExistingRepo(repo, notYet string) error {
	if restic.IsRemoteRepo(repo) {
		return nil
	}
	if _, statErr := os.Stat(filepath.Join(repo, "config")); errors.Is(statErr, fs.ErrNotExist) { //nolint:gosec // G703: repo is an operator-configured location (settings path or its off-site sibling), validated under the mount root on save; source only selects which configured location, never a raw path
		return errors.New(notYet)
	}
	return nil
}

// isLockErr reports whether a restic error is a repository-lock conflict. It
// matches restic's specific lock-conflict phrasing ("unable to create lock" /
// "already locked") rather than the bare word "locked", so an unrelated error
// that merely mentions a lock doesn't trigger a needless unlock + retry.
func isLockErr(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "unable to create lock") || strings.Contains(msg, "already locked")
}

// isRepoUninitialized reports whether a restic error is the "repository not
// initialised yet" signal (as opposed to a genuine auth/connectivity failure).
// restic phrases it as "repository does not exist" or, when it cannot read the
// config marker, "unable to open config file". Used to treat a not-yet-replicated
// REMOTE off-site repo as simply empty rather than surfacing restic's raw fatal
// (issue #117). Scoped to remote repos by the caller — local repos are guarded
// upstream by localRepoMissing.
func isRepoUninitialized(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "repository does not exist") || strings.Contains(msg, "unable to open config file")
}

// unlockStale best-effort clears stale locks (plain restic unlock: only locks
// from dead processes or old enough — never an active concurrent lock). Logged,
// never fatal.
func (s *Service) unlockStale(ctx context.Context, repo string, mode restic.Mode) {
	if err := s.engine.Unlock(ctx, repo, false, mode); err != nil {
		log.Printf("api: stale-unlock failed (continuing): %v", err)
	}
}

// listSnapshots lists snapshots, self-healing a stale-lock conflict: on a lock
// error it clears stale locks and retries once. This fixes "Failed to load
// backups" when an interrupted run left a lock behind.
func (s *Service) listSnapshots(ctx context.Context, repo string, mode restic.Mode) ([]restic.Snapshot, error) {
	snaps, err := s.engine.Snapshots(ctx, repo, mode)
	if isLockErr(err) {
		s.unlockStale(ctx, repo, mode)
		snaps, err = s.engine.Snapshots(ctx, repo, mode)
	}
	// A REMOTE off-site repo that has not been replicated/initialised yet has no
	// snapshots — restic reports "repository does not exist", which must read as
	// "no backups yet", not a fatal (issue #117). Local repos are already
	// short-circuited upstream by localRepoMissing, so this only affects remotes;
	// genuine auth/connectivity errors do NOT match isRepoUninitialized and still
	// propagate.
	if err != nil && restic.IsRemoteRepo(repo) && isRepoUninitialized(err) {
		return nil, nil
	}
	return snaps, err
}

// lsSelfHeal lists a snapshot's files (restic ls), self-healing a stale-lock
// conflict exactly like listSnapshots: on a lock error it clears stale locks
// and retries once. `ls` only takes a SHARED lock, but a stale EXCLUSIVE lock
// left behind by an interrupted write elsewhere in the repo (an off-site
// replication or check that didn't finish cleanly, a killed backup, …) blocks
// it all the same — restic itself never notices staleness on its own, and
// nothing else touches this repo until the next scheduled backup runs its own
// unlockStale as a side effect (see #29). Until then, every "Select files"
// attempt failed with a bare "Failed to load files" (#129) even though the
// backups list right above it kept working fine, because THAT already had
// this self-heal and this sibling call never got it.
func (s *Service) lsSelfHeal(ctx context.Context, repo, snapshotID string, mode restic.Mode) ([]restic.FileEntry, error) {
	entries, err := s.engine.Ls(ctx, repo, snapshotID, mode)
	if isLockErr(err) {
		s.unlockStale(ctx, repo, mode)
		entries, err = s.engine.Ls(ctx, repo, snapshotID, mode)
	}
	return entries, err
}

// UnlockDomain removes locks from a domain's repo (restic unlock --remove-all).
// BombVault is the sole writer and serialises its operations, so a leftover lock
// is always safe to clear — this is the manual counterpart to the automatic
// stale-lock cleanup done before each backup.
func (s *Service) UnlockDomain(ctx context.Context, domain, source string) error {
	settings, repo, err := s.domainRepoSource(domain, source)
	if err != nil {
		return err
	}
	if err := s.requireExistingRepo(repo, "no repository to unlock yet"); err != nil {
		return err
	}
	unlock, ok := s.tryLockDomainFor(domain, "unlock")
	if !ok {
		return errDomainBusy
	}
	defer unlock()
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	return s.engine.Unlock(ctx, repo, true, s.ModeFor(settings))
}

// PruneDomain reclaims repository space freed by forgotten snapshots
// (restic prune). Bounded by a generous timeout — pruning a large repo is slow.
// Once the domain lock is held it publishes a "maintenance" progress pair
// (begin/terminal, indeterminate — restic prune/forget streams no percentage)
// and records a "prune" run, so a manual/scheduled prune shows up on the
// dashboard activity log/run history instead of running invisibly.
func (s *Service) PruneDomain(ctx context.Context, domain, source string) error {
	return s.pruneDomain(ctx, domain, source, true)
}

// PruneAfterBulk runs ONE local prune for a domain after a bulk backup loop,
// replacing the per-item inline prune that the bulk run deferred: under the #95
// bulk flag applyRetention runs each item's forget WITHOUT --prune, so the
// expensive space-reclaim happens here exactly once per run. It reuses the
// PruneDomain core, so the batched prune takes the domain lock itself (the bulk
// loop has released all locks by now), publishes maintenance progress and
// records a kind="prune" run — visible in Run History/Activity Log exactly like
// a manual prune. LOCAL repo only: off-site retention stays inside
// copyToOffsite, and an immutable off-site repo is never pruned from this box.
// Skipped silently when no local retention policy is configured (mirroring
// applyRetention's own gate — nothing was forgotten, so there is nothing to
// reclaim). Best-effort: failures are logged, never propagated.
func (s *Service) PruneAfterBulk(ctx context.Context, domain string) {
	settings, err := s.store.GetSettings()
	if err != nil {
		log.Printf("api: prune %s: batched prune: read settings: %v", domain, err) //nolint:gosec // G706: domain is a fixed literal
		return
	}
	if !s.retentionPolicy(settings).Any() {
		return // no retention policy → the per-item passes forgot nothing (applyRetention's gate)
	}
	// applyPolicy=false: the per-item tag-scoped forgets already ran inline during
	// the loop (without --prune), so this pass is a plain space-reclaim.
	if err := s.pruneDomain(ctx, domain, "local", false); err != nil {
		log.Printf("api: prune %s: batched prune failed: %v", domain, err) //nolint:gosec // G706: domain is a fixed literal
	}
}

// pruneDomain is the shared core of PruneDomain and PruneAfterBulk. applyPolicy
// selects the manual-prune semantics (a configured retention policy is APPLIED:
// per-identity forget --keep-* then one prune — "apply retention now") versus a
// plain space-reclaim (`restic prune` only — the batched post-bulk pass, whose
// per-item forgets already ran inline without --prune).
func (s *Service) pruneDomain(ctx context.Context, domain, source string, applyPolicy bool) (err error) {
	settings, repo, err := s.domainRepoSource(domain, source)
	if err != nil {
		return err
	}
	// An immutable off-site repo is never pruned from this box (append-only is
	// the point). Only the offsite+immutable combination is gated — the local
	// repo stays fully maintainable. Per-target: bare "offsite" uses the primary
	// target's flag (== today), "offsite:<id>" that specific target's.
	if isOffsiteSource(source) && s.offsiteSourceImmutable(settings, domain, source) {
		return errOffsiteAppendOnly
	}
	// Issue #152: the SAME refusal applies when the "local" source IS actually a
	// remote primary flagged append-only in its saved safety settings — there is
	// no separate off-site copy in that shape, so refusing here is the only thing
	// standing between an on-box credential and deleting the sole backup.
	if !isOffsiteSource(source) && s.primaryIsImmutable(domain, repo) {
		return errOffsiteAppendOnly
	}
	if err := s.requireExistingRepo(repo, "no backups to prune yet"); err != nil {
		return err
	}
	unlock, ok := s.tryLockDomainFor(domain, "prune")
	if !ok {
		return errDomainBusy
	}
	defer unlock()
	ctx, cancel := context.WithTimeout(ctx, 30*time.Minute)
	defer cancel()
	mode := s.ModeFor(settings)

	pkey := "prune:" + domain
	_, startedAt := s.progBegin(ctx, pkey, "maintenance")
	defer func() { s.progEnd(pkey, "maintenance", err == nil, startedAt) }()
	runID, rErr := s.store.StartRun(domainRunTargetID(domain), "prune")
	if rErr != nil {
		log.Printf("api: prune %s: could not start run record (continuing): %v", domain, rErr) //nolint:gosec // G706: domain is a fixed literal
		runID = ""
	}
	defer func() {
		if runID == "" {
			return
		}
		status := "success"
		if err != nil {
			status = "failed"
		}
		if fErr := s.store.FinishRun(runID, status, "", 0, truncateRunErr(err)); fErr != nil {
			log.Printf("api: prune %s: could not finish run record: %v", domain, fErr) //nolint:gosec // G706: domain is a fixed literal
		}
	}()

	// Clear any stale lock left by a previously interrupted run so it can't block
	// this prune — a manual prune (and forget --prune) takes restic's exclusive
	// lock, and an interrupted backup/prune leaves one behind. BombVault is the
	// sole writer, so an existing lock is always stale. Every other repo-mutating
	// path (backups, DeleteSnapshot) does this; PruneDomain was missing it, which
	// made a manual Prune fail with "repository is already locked".
	s.unlockStale(ctx, repo, mode)
	// When a retention policy is configured, Prune APPLIES it (forget --keep-*
	// --prune): it collapses snapshots per the policy AND reclaims space — i.e. an
	// "apply retention now", which is what users expect from a manual prune.
	// Without a policy it stays a plain space-reclaim; running forget with no
	// keep-flags would delete every snapshot, so that path is guarded by p.Any().
	// The policy is per-source: pruning the off-site repo uses the off-site policy
	// (not the local one), so an archive off-site isn't trimmed to the local rules.
	// The batched post-bulk pass skips this (applyPolicy=false): its per-item
	// forgets already ran inline, so re-running them would only cost 44 more
	// exclusive-lock round-trips for nothing.
	if applyPolicy {
		if p := s.retentionPolicyForSource(settings, source); p.Any() {
			// Per-identity: tag-scoped, ungrouped forget per item + one prune —
			// also drains frozen path-groups left by the old grouping (issue #91).
			err = s.applyRetentionPerIdentity(ctx, repo, p, mode)
			return err
		}
	}
	err = s.engine.Prune(ctx, repo, mode)
	return err
}

// DeleteSnapshot forgets a single snapshot by id from a domain's repo (restic
// forget, no prune — fast). The space is reclaimed later by PruneDomain, so
// deleting several snapshots then pruning once is far cheaper than pruning per
// delete. The snapshot id is validated (arg-injection guard) and stale locks are
// cleared first.
func (s *Service) DeleteSnapshot(ctx context.Context, domain, snapshotID, source string) error {
	if !backup.ValidSnapshotID(snapshotID) {
		return backup.ErrInvalidSnapshotID
	}
	settings, repo, err := s.domainRepoSource(domain, source)
	if err != nil {
		return err
	}
	// Deleting snapshots from an immutable off-site repo is refused (same gate
	// as PruneDomain): append-only means credentials on this box cannot erase
	// off-site history. The local repo is unaffected. Per-target: bare "offsite"
	// uses the primary target's flag (== today), "offsite:<id>" that target's.
	if isOffsiteSource(source) && s.offsiteSourceImmutable(settings, domain, source) {
		return errOffsiteAppendOnly
	}
	// Issue #152: the SAME refusal applies when the "local" source IS actually a
	// remote primary flagged append-only in its saved safety settings (same gate
	// as pruneDomain) — there is no separate off-site copy in that shape, so
	// refusing here is the only thing standing between an on-box credential and
	// deleting backup history.
	if !isOffsiteSource(source) && s.primaryIsImmutable(domain, repo) {
		return errOffsiteAppendOnly
	}
	if err := s.requireExistingRepo(repo, "no backups to delete yet"); err != nil {
		return err
	}
	unlock, ok := s.tryLockDomainFor(domain, "delete")
	if !ok {
		return errDomainBusy
	}
	defer unlock()
	mode := s.ModeFor(settings)
	s.unlockStale(ctx, repo, mode)
	return s.engine.Forget(ctx, repo, []string{snapshotID}, false, mode)
}

// ---------------------------------------------------------------------------
// Off-site (rclone) config
// ---------------------------------------------------------------------------

// rcloneConfPath is where the decrypted rclone config is written for restic→rclone.
func (s *Service) rcloneConfPath() string { return filepath.Join(s.cfg.DataDir, "rclone.conf") }

// WriteRcloneConfFile (re)writes the on-disk rclone config from the encrypted
// value in settings, or removes it when empty. Called at startup so off-site
// repos work immediately after a restart.
func (s *Service) WriteRcloneConfFile() error {
	settings, err := s.store.GetSettings()
	if err != nil {
		return err
	}
	return s.writeRcloneFile(settings.RcloneConf)
}

// writeRcloneFile writes the decrypted rclone config (from its base64+AES-GCM
// stored form) to a 0600 file, or removes the file when the stored value is empty.
func (s *Service) writeRcloneFile(encB64 string) error {
	p := s.rcloneConfPath()
	if strings.TrimSpace(encB64) == "" {
		if err := os.Remove(p); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("remove rclone conf: %w", err)
		}
		return nil
	}
	enc, err := base64.StdEncoding.DecodeString(encB64)
	if err != nil {
		return fmt.Errorf("decode rclone conf: %w", err)
	}
	plain, err := secret.Decrypt(s.cfg.AppKey, enc)
	if err != nil {
		return fmt.Errorf("decrypt rclone conf: %w", err)
	}
	if err := os.WriteFile(p, plain, 0o600); err != nil {
		return fmt.Errorf("write rclone conf: %w", err)
	}
	// Guarantee 0600 even if the file pre-existed with looser perms (WriteFile
	// only applies the mode on creation) — it holds cleartext cloud credentials.
	if err := os.Chmod(p, 0o600); err != nil {
		return fmt.Errorf("chmod rclone conf: %w", err)
	}
	return nil
}

// SetRcloneConf encrypts + stores the rclone config and rewrites the on-disk
// file restic→rclone reads. An empty conf clears both. The stored DB value is
// AES-256-GCM-encrypted (APP_KEY); the on-disk file is 0600 in /config.
func (s *Service) SetRcloneConf(conf string) error {
	stored := ""
	if strings.TrimSpace(conf) != "" {
		enc, encErr := secret.Encrypt(s.cfg.AppKey, []byte(conf))
		if encErr != nil {
			return fmt.Errorf("encrypt rclone conf: %w", encErr)
		}
		stored = base64.StdEncoding.EncodeToString(enc)
	}
	if _, err := s.store.MutateSettings(func(settings *store.Settings) error {
		settings.RcloneConf = stored
		return nil
	}); err != nil {
		return err
	}
	return s.writeRcloneFile(stored)
}

// RcloneRemotes returns the configured rclone remote names (the [name] sections)
// for display — never the secrets themselves.
func (s *Service) RcloneRemotes() ([]string, error) {
	settings, err := s.store.GetSettings()
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(settings.RcloneConf) == "" {
		return nil, nil
	}
	enc, err := base64.StdEncoding.DecodeString(settings.RcloneConf)
	if err != nil {
		return nil, err
	}
	plain, err := secret.Decrypt(s.cfg.AppKey, enc)
	if err != nil {
		return nil, err
	}
	return parseRcloneRemotes(string(plain)), nil
}

// decodeRcloneConf returns the decrypted rclone config text stored in settings
// (an empty/blank rclone_conf yields "", no error). Unlike RcloneRemotes it keeps
// the full contents — used by the recovery kit, which needs the remote secrets.
func (s *Service) decodeRcloneConf(settings store.Settings) (string, error) {
	if strings.TrimSpace(settings.RcloneConf) == "" {
		return "", nil
	}
	enc, err := base64.StdEncoding.DecodeString(settings.RcloneConf)
	if err != nil {
		return "", err
	}
	plain, err := secret.Decrypt(s.cfg.AppKey, enc)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}

// parseRcloneRemotes extracts the [name] section headers from an rclone config.
func parseRcloneRemotes(conf string) []string {
	var out []string
	for _, line := range strings.Split(conf, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			if name := strings.TrimSpace(line[1 : len(line)-1]); name != "" {
				out = append(out, name)
			}
		}
	}
	return out
}

// ---------------------------------------------------------------------------
// Notifications
// ---------------------------------------------------------------------------

// NotifyConfig returns the decrypted notification config (an empty Config when
// none is set).
func (s *Service) NotifyConfig() (notify.Config, error) {
	settings, err := s.store.GetSettings()
	if err != nil {
		return notify.Config{}, err
	}
	var c notify.Config
	if strings.TrimSpace(settings.NotifyConf) == "" {
		return c, nil
	}
	enc, err := base64.StdEncoding.DecodeString(settings.NotifyConf)
	if err != nil {
		return c, err
	}
	plain, err := secret.Decrypt(s.cfg.AppKey, enc)
	if err != nil {
		return c, err
	}
	if err := json.Unmarshal(plain, &c); err != nil {
		return c, err
	}
	return c, nil
}

// SetNotifyConfig encrypts + stores the notification config. A config with no
// channel and no policy clears it.
func (s *Service) SetNotifyConfig(c notify.Config) error {
	stored := ""
	if c.Configured() || (c.On != "" && c.On != "never") {
		blob, mErr := json.Marshal(c)
		if mErr != nil {
			return fmt.Errorf("marshal notify conf: %w", mErr)
		}
		enc, eErr := secret.Encrypt(s.cfg.AppKey, blob)
		if eErr != nil {
			return fmt.Errorf("encrypt notify conf: %w", eErr)
		}
		stored = base64.StdEncoding.EncodeToString(enc)
	}
	_, err := s.store.MutateSettings(func(settings *store.Settings) error {
		settings.NotifyConf = stored
		return nil
	})
	return err
}

// ---------------------------------------------------------------------------
// Cloud-backend credentials (S3 / restic-REST) for off-site repos
// ---------------------------------------------------------------------------

// CloudCreds holds the backend credentials restic reads from the environment for
// off-site repos. Stored AES-256-GCM-encrypted in settings.cloud_conf. The two
// secret fields (S3Secret, RESTPassword) are write-only over the API.
type CloudCreds struct {
	S3KeyID      string `json:"s3KeyId"`
	S3Secret     string `json:"s3Secret"`
	S3Region     string `json:"s3Region"`
	RESTUser     string `json:"restUser"`
	RESTPassword string `json:"restPassword"`
	// S3StorageClass is the S3 storage class for restic writes to a native s3:
	// off-site backend (empty = the provider default). Unlike the credential
	// fields it is NOT a secret (a class name), so handleGetCloud returns it. It
	// rides this same AES-256-GCM-encrypted cloud_conf blob, so it needs no schema
	// migration. It is validated against restic.AllowedStorageClasses on save
	// (SetCloudCreds), so only a restore-readable tier is ever stored/emitted.
	S3StorageClass string `json:"s3StorageClass"`
}

// cloudEnv renders the credentials into the env vars restic expects (only the set
// ones), so they reach the restic process via Mode.Env and never via argv/logs.
func cloudEnv(c CloudCreds) []string {
	var env []string
	add := func(k, v string) {
		if v != "" {
			env = append(env, k+"="+v)
		}
	}
	add("AWS_ACCESS_KEY_ID", c.S3KeyID)
	add("AWS_SECRET_ACCESS_KEY", c.S3Secret)
	add("AWS_DEFAULT_REGION", c.S3Region)
	add("RESTIC_REST_USERNAME", c.RESTUser)
	add("RESTIC_REST_PASSWORD", c.RESTPassword)
	return env
}

// decodeCloud decrypts the stored cloud credentials from the given settings (an
// empty/blank cloud_conf yields a zero CloudCreds, no error).
func (s *Service) decodeCloud(settings store.Settings) (CloudCreds, error) {
	var c CloudCreds
	if strings.TrimSpace(settings.CloudConf) == "" {
		return c, nil
	}
	enc, err := base64.StdEncoding.DecodeString(settings.CloudConf)
	if err != nil {
		return c, err
	}
	plain, err := secret.Decrypt(s.cfg.AppKey, enc)
	if err != nil {
		return c, err
	}
	if err := json.Unmarshal(plain, &c); err != nil {
		return c, err
	}
	return c, nil
}

// CloudConfig returns the stored credentials. (Callers that serve it to the UI
// must blank the secret fields — see handleGetCloud.)
func (s *Service) CloudConfig() (CloudCreds, error) {
	settings, err := s.store.GetSettings()
	if err != nil {
		return CloudCreds{}, err
	}
	return s.decodeCloud(settings)
}

// SetCloudCreds stores the credentials encrypted. A blank secret field KEEPS the
// previously stored secret (so the UI can edit non-secret fields without
// re-entering keys). A config with nothing set clears it.
func (s *Service) SetCloudCreds(c CloudCreds) error {
	// Normalize + validate the S3 storage class BEFORE anything is stored: uppercase
	// it, and reject a non-empty value that is not a whitelisted, restore-readable
	// tier (see restic.AllowedStorageClasses) so an archival class that would break
	// restic restore is never silently persisted. Empty = the provider default.
	c.S3StorageClass = strings.ToUpper(strings.TrimSpace(c.S3StorageClass))
	if c.S3StorageClass != "" && !restic.StorageClassAllowed(c.S3StorageClass) {
		return fmt.Errorf("unsupported S3 storage class %q (allowed: %s)", c.S3StorageClass, strings.Join(restic.AllowedStorageClasses, ", "))
	}
	// The keep-prior merge below reads the CURRENTLY stored secrets, so it has to
	// happen in the same transaction as the write. Doing it against a snapshot
	// taken before the write would re-encrypt a secret that a save landing in
	// between had already replaced — the blank field would then "keep" a value
	// that is no longer the stored one.
	_, err := s.store.MutateSettings(func(settings *store.Settings) error {
		c := c
		// A fully-blank request means "clear" — check BEFORE the keep-prior merge,
		// otherwise the merge would re-fill the secrets and clearing would be
		// impossible once a secret had been stored.
		if (CloudCreds{}) == c {
			settings.CloudConf = ""
			return nil
		}
		// Otherwise keep a previously stored secret when its field is left blank, so
		// the non-secret fields can be edited without re-entering keys.
		prev, _ := s.decodeCloud(*settings)
		if c.S3Secret == "" {
			c.S3Secret = prev.S3Secret
		}
		if c.RESTPassword == "" {
			c.RESTPassword = prev.RESTPassword
		}
		blob, mErr := json.Marshal(c)
		if mErr != nil {
			return fmt.Errorf("marshal cloud conf: %w", mErr)
		}
		enc, eErr := secret.Encrypt(s.cfg.AppKey, blob)
		if eErr != nil {
			return fmt.Errorf("encrypt cloud conf: %w", eErr)
		}
		settings.CloudConf = base64.StdEncoding.EncodeToString(enc)
		return nil
	})
	return err
}

// ---------------------------------------------------------------------------
// Named credential sets (#141 stage 2): additional S3/restic-REST credentials
// an off-site target can opt into via OffsiteTarget.CredsRef, instead of every
// target sharing the one CloudCreds set above. An empty CredsRef keeps using
// CloudCreds exactly as before, so this is purely additive.
// ---------------------------------------------------------------------------

// CloudCredSet is one named, additional credential set. Embeds CloudCreds for
// the actual key/secret/region/storage-class fields so cloudEnv and the
// storage-class validation in SetCloudCredSets stay shared with the legacy
// single-set path.
type CloudCredSet struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	CloudCreds
}

// decodeCloudCredSets decrypts the stored additional credential sets from the
// given settings (an empty/blank cloud_cred_sets yields nil, no error).
func (s *Service) decodeCloudCredSets(settings store.Settings) ([]CloudCredSet, error) {
	if strings.TrimSpace(settings.CloudCredSets) == "" {
		return nil, nil
	}
	enc, err := base64.StdEncoding.DecodeString(settings.CloudCredSets)
	if err != nil {
		return nil, err
	}
	plain, err := secret.Decrypt(s.cfg.AppKey, enc)
	if err != nil {
		return nil, err
	}
	var sets []CloudCredSet
	if err := json.Unmarshal(plain, &sets); err != nil {
		return nil, err
	}
	return sets, nil
}

// CloudCredSets returns the additional named credential sets, with every
// secret field blanked (callers that need the real secrets, e.g. restic env
// building, must go through decodeCloudFor — this is for serving the list to
// the UI, same blank-secrets contract as handleGetCloud).
func (s *Service) CloudCredSets() ([]CloudCredSet, error) {
	settings, err := s.store.GetSettings()
	if err != nil {
		return nil, err
	}
	sets, err := s.decodeCloudCredSets(settings)
	if err != nil {
		return nil, err
	}
	out := make([]CloudCredSet, len(sets))
	for i, set := range sets {
		out[i] = set
		out[i].S3Secret = ""
		out[i].RESTPassword = ""
	}
	return out, nil
}

// SetCloudCredSets replaces the whole list of additional named credential
// sets. Each set's secret fields follow the same keep-prior-if-blank rule as
// SetCloudCreds (matched by ID against the previously stored set), so the UI
// can rename a set or edit its non-secret fields without re-entering keys. A
// set with a blank Name or a duplicate ID is rejected — both would make
// CredsRef resolution ambiguous or the set unreachable from the UI.
func (s *Service) SetCloudCredSets(sets []CloudCredSet) error {
	// Same reasoning as SetCloudCreds: the keep-prior-if-blank merge reads the
	// CURRENTLY stored sets, so it belongs in the same transaction as the write.
	// The incoming slice is copied rather than normalized in place — the caller's
	// value is theirs, and the merged one carries real secrets.
	_, err := s.store.MutateSettings(func(settings *store.Settings) error {
		next := make([]CloudCredSet, len(sets))
		copy(next, sets)

		prev, _ := s.decodeCloudCredSets(*settings)
		prevByID := make(map[string]CloudCredSet, len(prev))
		for _, p := range prev {
			prevByID[p.ID] = p
		}
		seen := make(map[string]bool, len(next))
		for i := range next {
			next[i].Name = strings.TrimSpace(next[i].Name)
			if next[i].Name == "" {
				return fmt.Errorf("credential set name must not be empty")
			}
			if next[i].ID == "" {
				return fmt.Errorf("credential set %q: missing id", next[i].Name)
			}
			if seen[next[i].ID] {
				return fmt.Errorf("duplicate credential set id %q", next[i].ID)
			}
			seen[next[i].ID] = true
			next[i].S3StorageClass = strings.ToUpper(strings.TrimSpace(next[i].S3StorageClass))
			if next[i].S3StorageClass != "" && !restic.StorageClassAllowed(next[i].S3StorageClass) {
				return fmt.Errorf("credential set %q: unsupported S3 storage class %q (allowed: %s)", next[i].Name, next[i].S3StorageClass, strings.Join(restic.AllowedStorageClasses, ", "))
			}
			if old, ok := prevByID[next[i].ID]; ok {
				if next[i].S3Secret == "" {
					next[i].S3Secret = old.S3Secret
				}
				if next[i].RESTPassword == "" {
					next[i].RESTPassword = old.RESTPassword
				}
			}
		}
		if len(next) == 0 {
			settings.CloudCredSets = ""
			return nil
		}
		blob, mErr := json.Marshal(next)
		if mErr != nil {
			return fmt.Errorf("marshal cloud cred sets: %w", mErr)
		}
		enc, eErr := secret.Encrypt(s.cfg.AppKey, blob)
		if eErr != nil {
			return fmt.Errorf("encrypt cloud cred sets: %w", eErr)
		}
		settings.CloudCredSets = base64.StdEncoding.EncodeToString(enc)
		return nil
	})
	return err
}

// decodeCloudFor resolves the credentials an off-site target should use: the
// shared/legacy CloudCreds when credsRef is empty (every existing install,
// unchanged), or the matching named CloudCredSet otherwise. A credsRef that no
// longer resolves (the set was deleted, or storage drifted) falls back to the
// shared creds rather than failing the caller outright — restic then fails
// loudly on auth if that fallback genuinely has no usable credentials for this
// target's endpoint, which is a clearer signal than an opaque config error.
func (s *Service) decodeCloudFor(settings store.Settings, credsRef string) (CloudCreds, error) {
	if strings.TrimSpace(credsRef) == "" {
		return s.decodeCloud(settings)
	}
	sets, err := s.decodeCloudCredSets(settings)
	if err != nil {
		return CloudCreds{}, err
	}
	for _, set := range sets {
		if set.ID == credsRef {
			return set.CloudCreds, nil
		}
	}
	log.Printf("api: off-site target references unknown credential set %q, falling back to shared credentials", credsRef)
	return s.decodeCloud(settings)
}

// ---------------------------------------------------------------------------
// Encryption-key recovery kit (disaster recovery without a running BombVault)
// ---------------------------------------------------------------------------

// recoveryRepo is one domain's resolved repo locations for the recovery kit.
type recoveryRepo struct {
	Domain  string
	Local   string
	Offsite string // "" when none configured
}

// RecoveryKit builds the plain-text/markdown recovery document the authenticated
// owner downloads to survive a loss of BombVault itself. With encryption ON it
// contains the master APP_KEY and the SAME APP_KEY-derived restic repository
// password the engine uses (restickey.Derive), the per-domain repo locations, and
// step-by-step manual `restic restore` instructions that need no BombVault
// container. With encryption OFF the repos use `--insecure-no-password`, so the
// kit's value is mainly the repo locations + the instructions.
//
// The document contains the master key, so it must never be logged and must be
// stored offline by the user (the handler streams it as an attachment only to the
// session-authenticated owner).
func (s *Service) RecoveryKit() (string, error) {
	settings, err := s.store.GetSettings()
	if err != nil {
		return "", fmt.Errorf("read settings: %w", err)
	}

	// Resolve each domain's local + off-site repo locations from the configured
	// settings (the same resolution the engine uses), so the kit names the real
	// places the data lives. A resolution failure for one domain leaves that line
	// blank rather than failing the whole kit.
	repos := make([]recoveryRepo, 0, 4)
	for _, d := range []string{"containers", "vms", "flash", "files"} {
		rr := recoveryRepo{Domain: d}
		if loc, rErr := s.repoFor(settings, d, "local"); rErr == nil {
			rr.Local = loc
		}
		if off := s.offsiteRepoFor(d, settings); off != "" {
			if loc, rErr := s.resolveRepo(off); rErr == nil {
				rr.Offsite = loc
			} else {
				rr.Offsite = off
			}
		}
		repos = append(repos, rr)
	}

	var b strings.Builder
	w := func(format string, a ...any) { fmt.Fprintf(&b, format, a...) }

	w("# BombVault encryption-key recovery kit\n\n")
	w("Generated: %s\n\n", time.Now().Format(time.RFC1123))
	w("> WARNING: this file is the master secret for your encrypted backups.\n")
	w("> It contains your APP_KEY and the derived restic repository password.\n")
	w("> Store it OFFLINE and securely (a password manager or printed copy in a safe).\n")
	w("> Anyone with this file can read and restore your backups.\n\n")

	w("## Encryption\n\n")
	if settings.EncryptionEnabled {
		password := restickey.Derive(s.cfg.AppKey)
		w("Status: ENABLED\n\n")
		w("APP_KEY (the master key — recreate the BombVault container with this exact value):\n\n")
		w("    %s\n\n", s.cfg.AppKey)
		w("restic repository password (derived from APP_KEY; use this with plain restic):\n\n")
		w("    %s\n\n", password)
	} else {
		w("Status: DISABLED\n\n")
		w("The repositories are created WITHOUT a password (restic --insecure-no-password).\n")
		w("There is no key to lose; the value of this kit is the repository locations and\n")
		w("the restore instructions below.\n\n")
	}

	w("## Repository locations\n\n")
	w("Paths are inside the BombVault container, under the host data mount (%s).\n", s.cfg.HostMountRoot)
	w("On the host they live under your backup share; remote backends (rclone:/s3:/rest:/sftp:) are used as shown.\n\n")
	for _, rr := range repos {
		w("- %s (local): %s\n", rr.Domain, orNone(rr.Local))
		if rr.Offsite != "" {
			w("- %s (off-site): %s\n", rr.Domain, rr.Offsite)
		}
	}
	w("\n")
	w("Each line above is a SEPARATE restic repository. Point restic (or a tool like\n")
	w("backrest) at the specific per-domain path — the parent folder that holds them is\n")
	w("NOT itself a repository, and the off-site repo only has snapshots once off-site\n")
	w("replication has actually run. Add each domain repo on its own.\n\n")

	// BombVault's own settings backup (the "config" self-backup domain). This repo is
	// the single bootstrap seed a rebuilt box needs: restore it FIRST to bring
	// BombVault's own configuration back, then the data domains above follow. It uses
	// the SAME APP_KEY-derived restic password already documented above, so no new
	// secret appears here. A resolution failure leaves the local line blank rather
	// than failing the kit; the off-site line prints only when one is configured.
	w("## BombVault settings backup (config domain)\n\n")
	w("This repository holds BombVault's OWN settings. On a rebuilt box, restore it\n")
	w("FIRST to bring BombVault's configuration back, then use the data repositories\n")
	w("above. It is the one location to write down so a fresh install can find itself.\n\n")
	configLocal := ""
	if loc, cErr := s.configRepoPath(settings); cErr == nil {
		configLocal = loc
	}
	w("- config (local): %s\n", orNone(configLocal))
	if settings.ConfigOffsite != "" {
		w("- config (off-site): %s\n", settings.ConfigOffsite)
	}
	w("\n")

	// Off-site/cloud credentials — the stored rest-server / S3 keys and rclone
	// config a user needs to reach a remote repository after losing BombVault.
	// These are secrets too, covered by the master-secret WARNING above; like the
	// APP_KEY they go ONLY into this downloaded kit and are never logged. Only the
	// fields that are actually set are printed (mirrors cloudEnv), so the section
	// never shows an empty label.
	creds, _ := s.decodeCloud(settings)
	rcloneConf, _ := s.decodeRcloneConf(settings)
	hasREST := creds.RESTUser != "" || creds.RESTPassword != ""
	hasS3 := creds.S3KeyID != "" || creds.S3Secret != "" || creds.S3Region != ""
	hasRclone := strings.TrimSpace(rcloneConf) != ""

	w("## Repository credentials\n\n")
	if !hasREST && !hasS3 && !hasRclone {
		w("No off-site/cloud credentials are stored in BombVault.\n\n")
	} else {
		w("These are the stored off-site backend credentials — the same secrets restic\n")
		w("reads from its environment (or the rclone config) to reach a remote repository.\n")
		w("They are as sensitive as the APP_KEY above; keep them just as safe.\n\n")

		if hasREST {
			w("rest-server (restic REST backend) — restic reads these from the environment:\n\n")
			if creds.RESTUser != "" {
				w("    RESTIC_REST_USERNAME=%s\n", creds.RESTUser)
			}
			if creds.RESTPassword != "" {
				w("    RESTIC_REST_PASSWORD=%s\n", creds.RESTPassword)
			}
			w("\n")
			w("Export these before running restic against a rest: repository. They can also\n")
			w("live inside the URL, e.g. rest:https://user:pass@host:8000/path.\n\n")
		}

		if hasS3 {
			w("S3-compatible backend — restic reads these from the environment:\n\n")
			if creds.S3KeyID != "" {
				w("    AWS_ACCESS_KEY_ID=%s\n", creds.S3KeyID)
			}
			if creds.S3Secret != "" {
				w("    AWS_SECRET_ACCESS_KEY=%s\n", creds.S3Secret)
			}
			if creds.S3Region != "" {
				w("    AWS_DEFAULT_REGION=%s\n", creds.S3Region)
			}
			w("\n")
			w("Export these before running restic against an s3: repository.\n\n")
		}

		if hasRclone {
			w("rclone config — holds each remote's own secrets. Save it verbatim as\n")
			w("~/.config/rclone/rclone.conf, then use the repo as rclone:<remote>:<path>:\n\n")
			w("```\n%s\n```\n\n", strings.TrimRight(rcloneConf, "\n"))
		}
	}

	w("## Manual restore without BombVault\n\n")
	w("You can restore directly with the restic CLI, no BombVault container required.\n\n")
	w("1. Install restic (https://restic.net) on any machine that can reach the repository.\n")
	if settings.EncryptionEnabled {
		w("2. Set the repository password from this kit:\n\n")
		w("       export RESTIC_PASSWORD='%s'\n\n", restickey.Derive(s.cfg.AppKey))
	} else {
		w("2. The repositories have no password — pass --insecure-no-password to every\n")
		w("   restic command below (e.g. `restic -r <repo> --insecure-no-password snapshots`).\n\n")
	}
	w("3. List the snapshots in a repository (use a path or remote from the list above):\n\n")
	w("       restic -r <repo> snapshots\n\n")
	w("4. Restore a snapshot into a target directory (`restic restore`):\n\n")
	w("       restic -r <repo> restore <snapshot-id> --target <restore-dir>\n\n")
	w("Notes:\n")
	w("- For a LOCAL repo, point <repo> at the backup folder on disk (the path above is the\n")
	w("  container view; on the host it is your backup share, e.g. /mnt/user/<...>).\n")
	w("- For an rclone remote, configure rclone (~/.config/rclone/rclone.conf) and use the\n")
	w("  repo verbatim, e.g. `restic -r rclone:remote:bucket/path snapshots`.\n")
	w("- For an S3/B2/REST/SFTP remote, export the backend credentials restic expects\n")
	w("  (AWS_ACCESS_KEY_ID / AWS_SECRET_ACCESS_KEY for S3, RESTIC_REST_USERNAME /\n")
	w("  RESTIC_REST_PASSWORD for a REST server) and use the repo verbatim.\n")

	return b.String(), nil
}

// orNone returns s, or "(not resolved)" when s is empty, so a blank repo line in
// the recovery kit reads clearly instead of trailing off.
func orNone(s string) string {
	if s == "" {
		return "(not resolved)"
	}
	return s
}

// notifyBackup sends a best-effort notification for a completed backup. It reads
// the stored config each call (cheap; backups are infrequent) and is a no-op when
// notifications are off.
func (s *Service) notifyBackup(ctx context.Context, domain, name string, ok bool, sum backup.Summary, backupErr error) {
	c, err := s.NotifyConfig()
	if err != nil || c.On == "" || c.On == "never" {
		return
	}
	// Singleton domains have no per-item name, so a "%s %q" label would render an
	// empty quote (e.g. `config ""`). Give each a clean human label.
	var target string
	switch domain {
	case "flash":
		target = "Unraid flash"
	case "config":
		target = "BombVault configuration"
	default:
		target = fmt.Sprintf("%s %q", domain, name)
	}
	var msg string
	if ok {
		msg = fmt.Sprintf("Backup of %s succeeded (snapshot %s, %s).", target, shortID(sum.SnapshotID), humanBytes(sum.Bytes))
	} else {
		msg = fmt.Sprintf("Backup of %s FAILED: %s", target, scrubError(backupErr))
	}
	notify.Send(ctx, c, domain, notify.Event{Title: "BombVault", Message: msg, OK: ok})

	// Unraid native notification (delivered over SSH; notify.Send is HTTP-only).
	// Honour the same policy: notifyBackup already returned for "never", so send
	// on "always" or on any failure. In scheduled summary mode, drop the per-item
	// Unraid push too — ScheduledNotifyResult sends the one aggregate (#56).
	if s.unraidGate(c.Unraid) && (c.On == "always" || !ok) &&
		(!notify.MessagesSuppressed(ctx) || !c.ScheduledSummary) {
		level := "normal"
		subject := "BombVault: backup OK"
		if !ok {
			level = "warning"
			subject = "BombVault: backup FAILED"
		}
		if e := s.sendUnraidNotify(ctx, subject, msg, level); e != nil {
			log.Printf("notify: unraid: %v", e)
		}
	}
}

// statusSkipped marks a run BombVault intentionally did NOT perform because the
// target's container no longer exists on the host (#57). runs.status is free-text,
// so this needs no schema migration (cf. the orchestrator's "cancelled" status).
const statusSkipped = "skipped"

// recordAndNotifyContainerSkip handles a scheduled target whose container is gone
// (#57): it records a "skipped" run so the dashboard reflects it — and agrees with
// the green aggregate Healthchecks ping instead of showing nothing — and warns the
// user, debounced to the FIRST miss so a permanently-removed container doesn't spam
// a notification every night. The warning never pings Healthchecks (a skip is not a
// backup failure), so it can never turn a green monitor red.
func (s *Service) recordAndNotifyContainerSkip(ctx context.Context, name string) {
	tg, err := s.store.GetTargetByContainer(name)
	if err != nil {
		log.Printf("api: Backup: skip %q: load target: %v", name, err) //nolint:gosec // G706: name is %q-quoted
		return
	}
	// Debounce: warn only when the previous run for this target wasn't already a skip.
	firstMiss := true
	if last, lErr := s.store.LastRunForTarget(tg.ID); lErr == nil && last != nil && last.Status == statusSkipped {
		firstMiss = false
	}
	// Always record the skip so Run History shows it every run (a cheap, honest audit
	// trail) rather than the removed target silently vanishing from the dashboard.
	if runID, sErr := s.store.StartRun(tg.ID, "backup"); sErr != nil {
		log.Printf("api: Backup: skip %q: start skipped run: %v", name, sErr) //nolint:gosec // G706: name is %q-quoted
	} else if fErr := s.store.FinishRun(runID, statusSkipped, "", 0, "container no longer exists on the host"); fErr != nil {
		log.Printf("api: Backup: skip %q: finish skipped run: %v", name, fErr) //nolint:gosec // G706: name is %q-quoted
	}
	if !firstMiss {
		return
	}
	c, err := s.NotifyConfig()
	// Honour the notify policy on BOTH channels (message + Unraid): when muted
	// ("never"/unset) the skipped run row and the dashboard chip already surface it,
	// so no push should fire — otherwise a benign skip would be noisier than a real
	// backup failure (which notifyBackup stays silent about under the same policy).
	if err != nil || (c.On != "always" && c.On != "failure") {
		return
	}
	// #111: the old wording ("…is still a scheduled backup target. It was skipped…")
	// read as "BombVault is still trying to back it up". Say the opposite loudly:
	// nothing is backed up anymore, the existing backups stay restorable, and how
	// to stop the reminder.
	msg := fmt.Sprintf("Container %q was removed from this host. BombVault is not backing it up anymore; the scheduled backup now skips it. Its existing backups are kept and remain restorable. To stop this reminder, exclude the container from the backup schedule or delete its backups in BombVault.", name)
	// Suppress the per-call Healthchecks ping unconditionally: a skip must never flip
	// the monitor to fail, and the scheduled run's aggregate ping already speaks for
	// the domain. Base the ctx on Background (not the scheduled ctx) so the message is
	// NOT swept up by scheduled-summary suppression — a "target no longer exists"
	// warning must reach the user even in summary mode (like the update notice).
	notify.Send(notify.WithHealthchecksSuppressed(context.Background()), c, "containers",
		notify.Event{Title: "BombVault: backup target skipped", Message: msg, OK: false})
	if s.unraidGate(c.Unraid) {
		if e := s.sendUnraidNotify(ctx, "BombVault: backup target skipped", msg, "warning"); e != nil {
			log.Printf("notify: unraid: %v", e)
		}
	}
}

// recordContainerFailure records a FAILED backup run for a container that failed
// BEFORE backup.BackupContainer could take over run bookkeeping — i.e. at one of
// Backup's pre-flight early-returns (settings / repo path / EnsureRepo / inspect /
// empty-paths / upsert). It mirrors recordAndNotifyContainerSkip: given the resolved
// target, StartRun then FinishRun with the (truncated) reason, so a domain-wide fault
// that trips these for every remaining container shows up as reds — each carrying its
// cause — on the dashboard heatmap/history instead of vanishing (#64). Best-effort: a
// bookkeeping error is logged, never returned (the caller is already returning the real
// error). targetID is the caller's pre-resolved id; when empty (a brand-new container
// with no target row yet) the failure can't be keyed to a run, so it is only logged
// here — the scheduled-summary notification still names it from the returned error.
func (s *Service) recordContainerFailure(name, targetID string, cause error) {
	if targetID == "" {
		log.Printf("api: Backup: %q failed before a run could be recorded (no target row yet): %v", name, cause) //nolint:gosec // G706: name is %q-quoted
		return
	}
	runID, err := s.store.StartRun(targetID, "backup")
	if err != nil {
		log.Printf("api: Backup: %q: start failed run: %v", name, err) //nolint:gosec // G706: name is %q-quoted
		return
	}
	if err := s.store.FinishRun(runID, "failed", "", 0, truncateRunErr(cause)); err != nil {
		log.Printf("api: Backup: %q: finish failed run: %v", name, err) //nolint:gosec // G706: name is %q-quoted
	}
}

// notifyBackupStart pings the Healthchecks /start endpoint at the beginning of a
// backup (best-effort; never affects the backup). The message channels have no
// "start" concept, so this is Healthchecks-only.
func (s *Service) notifyBackupStart(ctx context.Context, domain string) {
	c, err := s.NotifyConfig()
	if err != nil {
		return
	}
	notify.SendStart(ctx, c, domain)
}

// ScheduledHealthchecksStart pings the domain's Healthchecks check /start once at the
// beginning of a SCHEDULED per-domain run (containers/VMs). The scheduler runs each
// item with its own per-item /start suppressed (see main.go), so this single ping
// represents the whole domain job instead of one ping per container/VM (#49). It is
// best-effort and a no-op when the domain has no check configured or notifications are
// off. domain is the scheduler's spelling ("containers"|"vms"); notify normalises it.
func (s *Service) ScheduledHealthchecksStart(ctx context.Context, domain string) {
	c, err := s.NotifyConfig()
	if err != nil {
		return
	}
	notify.PingDomainStart(ctx, c, domain)
}

// ScheduledHealthchecksResult pings the domain's Healthchecks check once at the end of
// a SCHEDULED per-domain run: success when every item succeeded (failed == 0), else
// /fail with a short aggregate summary ("N of M items failed"). It is the aggregate
// counterpart to the per-item success/fail ping, which the run suppresses, so the check
// reflects the whole domain job (#49). Best-effort; a no-op when the domain has no
// check configured or notifications are off.
func (s *Service) ScheduledHealthchecksResult(ctx context.Context, domain string, attempted, failed int) {
	c, err := s.NotifyConfig()
	if err != nil {
		return
	}
	ok := failed == 0
	var summary string
	if ok {
		summary = fmt.Sprintf("%d of %d items succeeded", attempted, attempted)
	} else {
		summary = fmt.Sprintf("%d of %d items failed", failed, attempted)
	}
	notify.PingDomainResult(ctx, c, domain, ok, summary)
}

// ScheduledNotifyResult sends ONE summary message per SCHEDULED per-domain run on the
// message channels (webhook/Matrix/SMTP + Unraid) instead of one per item — the
// message-channel counterpart to ScheduledHealthchecksResult (#56). No-op unless
// Config.ScheduledSummary is on (in which case the per-item messages were suppressed);
// the On policy still governs whether an all-success run notifies at all. domain is the
// scheduler spelling ("containers"|"vms"). failures names the items that failed (name +
// reason) so a run that fails enumerates WHICH items broke and WHY, instead of a bare
// count that leaves the user with no idea what to fix (#64).
func (s *Service) ScheduledNotifyResult(ctx context.Context, domain string, attempted, failed int, failures []schedule.ItemFailure) {
	c, err := s.NotifyConfig()
	// Honour the notify policy on ALL channels (message + Unraid): muted ("never"/
	// unset) sends nothing, so the Unraid push below can't leak past a muted policy.
	if err != nil || !c.ScheduledSummary || attempted == 0 ||
		(c.On != "always" && c.On != "failure") {
		return
	}
	ok := failed == 0
	// "no failures" rather than "all succeeded": attempted counts every scheduled item
	// including any that were SKIPPED because their container is gone (#57) — a skip is
	// not a failure, but it isn't a success either, so don't overstate it. The skipped
	// target still gets its own per-item warning (recordAndNotifyContainerSkip).
	var summary string
	if ok {
		summary = fmt.Sprintf("Scheduled %s backup: %d items, no failures.", domain, attempted)
	} else {
		summary = fmt.Sprintf("Scheduled %s backup: %d of %d items failed.\n%s",
			domain, failed, attempted, formatItemFailures(failures))
	}
	// Reuse Send for the message channels with the Healthchecks ping suppressed
	// (ScheduledHealthchecksResult already sent the one aggregate HC ping). The summary
	// ctx carries no message-suppress flag, so Send delivers it (shouldSend still gates
	// an all-success summary out under On="failure").
	notify.Send(notify.WithHealthchecksSuppressed(ctx), c, domain,
		notify.Event{Title: "BombVault", Message: summary, OK: ok})
	if s.unraidGate(c.Unraid) && (c.On == "always" || !ok) {
		level := "normal"
		if !ok {
			level = "warning"
		}
		if e := s.sendUnraidNotify(ctx, "BombVault: scheduled "+domain+" backup", summary, level); e != nil {
			log.Printf("notify: unraid: %v", e)
		}
	}
}

// maxListedFailures caps how many failed items the scheduled summary enumerates
// individually before collapsing the rest into a "+N more" tail, so a night where
// 35 of 45 containers failed (the #64 report) stays readable in a chat/email.
const maxListedFailures = 10

// formatItemFailures renders a scheduled run's per-item failures as "- name: reason"
// lines for the summary notification, capping the list at maxListedFailures with a
// "+N more" tail (#64). Each reason is scrubbed of absolute host paths — the same
// treatment the per-item notifyBackup gives its error text — so the aggregated
// summary leaks nothing the suppressed per-item messages would not have.
func formatItemFailures(failures []schedule.ItemFailure) string {
	lines := make([]string, 0, len(failures))
	for i, f := range failures {
		if i == maxListedFailures {
			lines = append(lines, fmt.Sprintf("+%d more", len(failures)-maxListedFailures))
			break
		}
		lines = append(lines, fmt.Sprintf("- %s: %s", f.Name, scrubError(errors.New(f.Reason))))
	}
	return strings.Join(lines, "\n")
}

// sendUnraidNotify triggers Unraid's native notification system by running the
// host's notify script over SSH. level is "normal" | "warning" | "alert".
func (s *Service) sendUnraidNotify(ctx context.Context, subject, desc, level string) error {
	if s.ssh == nil {
		return errors.New("no SSH connection for Unraid notifications (set it up in Settings → VM Backup over SSH)")
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	_, err := s.ssh.Run(ctx, "/usr/local/emhttp/webGui/scripts/notify",
		"-e", "BombVault", "-s", subject, "-d", desc, "-i", level)
	return err
}

// TestNotify sends a test to every channel the (unsaved) config enables: the HTTP
// channels via notify.SendTest, plus the Unraid channel over SSH. It errors when
// nothing is configured or a configured channel fails, so the UI's Test button
// reflects the real result.
func (s *Service) TestNotify(ctx context.Context, c notify.Config) error {
	if !c.Configured() && !c.Unraid {
		return errors.New("no notification channel configured")
	}
	if c.Configured() {
		if err := notify.SendTest(ctx, c); err != nil {
			return err
		}
	}
	if c.Unraid {
		// TestNotify is a request-scoped, user-initiated action (the Settings
		// "Test" button), not a background best-effort job: silently reporting
		// success without attempting anything would be dishonest, so a
		// non-Unraid platform gets a clear, immediate refusal instead of an SSH
		// attempt that could only fail on the far end.
		if s.platformFn().Kind() != platform.KindUnraid {
			return fmt.Errorf("unraid: %w", s.unraidPlatformMismatchError("the Unraid notification channel"))
		}
		if err := s.sendUnraidNotify(ctx, "BombVault test notification",
			"If you see this in Unraid, BombVault notifications are working.", "normal"); err != nil {
			return fmt.Errorf("unraid: %w", err)
		}
	}
	return nil
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
