// Package api wires the real adapters (dockercli, restic, store, template,
// paths) into the dependency-injected backup orchestrator and exposes the
// JSON HTTP API plus the embedded SPA server.
//
// The DI seam is preserved: internal/backup imports only its own interfaces.
// All concrete-adapter wiring lives here in the service layer.
package api

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"maps"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
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

// templateNameRe matches the template's <Name> element, the container's
// display name. A <Config Name="..."> attribute is a setting's label and does
// not match.
var templateNameRe = regexp.MustCompile(`<Name>[^<]*</Name>`)

// rewriteTemplateXMLName sets the template's <Name> element to newName. A
// template without one, including an empty template, is returned unchanged.
func rewriteTemplateXMLName(xml, newName string) string {
	if xml == "" {
		return xml
	}
	return templateNameRe.ReplaceAllString(xml, "<Name>"+newName+"</Name>")
}

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

// errZvolRebaseFailed is the scrubber-bypass sentinel for a rebase failure in
// prepareRestoreVMForTarget's cross-instance zvol rebase loop, matched by
// handlers.go's scrubError. The dataset and pool names are the message, and a
// ZFS dataset name "<pool>/<rest>" contains "/", which handlers.go's absPathRe
// would mistake for a filesystem path and mangle (`dataset "tank/vm-disk1"`
// becomes `dataset "tank[path]"`). Same bypass as errRestoreDestination and
// errUnraidPlatformMismatch.
var errZvolRebaseFailed = errors.New("zvol dataset rebase failed")

// zvolRebaseErr carries a zvol-rebase failure's ready-to-show message (see
// errZvolRebaseFailed) while still satisfying
// errors.Is(err, errZvolRebaseFailed) for the scrubber bypass.
type zvolRebaseErr struct{ msg string }

func (e *zvolRebaseErr) Error() string { return e.msg }

func (e *zvolRebaseErr) Is(target error) bool { return target == errZvolRebaseFailed }

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

// ModeFor builds the restic Mode from the encryption setting: with encryption
// on the password is derived from APP_KEY, otherwise the repo has none.
func (s *Service) ModeFor(settings store.Settings) restic.Mode {
	// Decode the cloud creds once for both the backend-credential env vars and
	// the off-site S3 storage class, which share the same encrypted blob. A decode
	// failure logs and yields a zero CloudCreds, so the restic op fails clearly on
	// auth rather than panicking.
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

// resolveRepo turns a configured repo location into the value passed to
// restic -r. A restic remote backend (rclone:…, s3:…, sftp:…, used off-site)
// is passed verbatim; a local location is resolved as a relative subpath
// under the host mount root, rejecting traversal. A rejection comes back as
// operator guidance naming the relative-path convention and the value to
// enter instead (see repoPathError) rather than the raw paths sentinel (#138).
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

// DiscoverSource reports which repository the discovery pass for a domain
// reads, so the Recovery wizard can name it instead of leaving the user to
// guess (#196). The wizard asks for an off-site repository in one step and
// then reads the domain's primary path in the next, and on a fresh install
// with nothing local left those are rarely the same place. Naming the folder
// does not make the wizard read the off-site copy, but it turns "my backups
// are gone" into "it looked in the wrong place".
func (s *Service) DiscoverSource(domain string) string {
	settings, err := s.store.GetSettings()
	if err != nil {
		return ""
	}
	var repo string
	switch domain {
	case "vms":
		repo, err = s.vmsRepoPath(settings)
	case "files":
		repo, err = s.filesRepoPath(settings)
	default:
		repo, err = s.containersRepoPath(settings)
	}
	if err != nil {
		return ""
	}
	return repo
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

// fileSetRepoPath resolves the restic repo for one file set (#204): its own
// repository if it has one, otherwise the Folders domain repository. A domain
// path could already be a restic remote, but that moved every folder set at
// once; this moves one, for large static folders that only need a copy on a
// B2 or NAS share.
//
// The override goes through the same resolveRepo as the domain path, so the
// same string shapes work and the same containment rules apply: a relative
// subpath is resolved under the host mount root, a raw remote ("b2:…",
// "s3:…", "sftp:…", "rest:…", "rclone:…") is handed to restic verbatim.
//
// PruneDomain and CheckDomain still operate on the domain repository. A set
// in its own repository is backed up and restored there and its retention
// runs with it (applyRetention takes the repo it is handed), but a
// whole-domain prune or integrity check does not reach into it. The UI
// discloses that gap; closing it means teaching those two to iterate
// repositories.
func (s *Service) fileSetRepoPath(settings store.Settings, set store.FileSet) (string, error) {
	return s.itemRepoPath(set.Repo, func() (string, error) { return s.filesRepoPath(settings) })
}

// itemRepoPath is the one place a per-item repository override (#204) turns
// into a location, shared by containers, VMs and folder sets.
//
// The stored value is a named repository's id, not a location. Locations are
// written down once in Settings and picked per item, so ten containers do not
// mean typing the same bucket path ten times, and a location can be corrected
// in one place instead of in every item that copied it.
//
// An id that no longer resolves is an error, never a quiet fallback to the
// domain repository. Falling back would send the next backup somewhere else
// and look exactly like a working backup until somebody went looking for a
// snapshot that is in the other repo.
func (s *Service) itemRepoPath(repoID string, domainFallback func() (string, error)) (string, error) {
	id := strings.TrimSpace(repoID)
	if id == "" {
		return domainFallback()
	}
	named, err := s.store.GetNamedRepo(id)
	if err != nil {
		return "", fmt.Errorf("this item points at a repository that no longer exists; pick one again in Settings or clear the field")
	}
	if !named.Enabled {
		return "", fmt.Errorf("the repository %q is switched off, so nothing can be backed up to it", named.Name)
	}
	return s.resolveRepo(named.Repo)
}

// containerRepoPath resolves a container's per-item repository override (#204),
// falling back to the Containers domain repository.
func (s *Service) containerRepoPath(settings store.Settings, tg store.Target) (string, error) {
	return s.itemRepoPath(tg.Repo, func() (string, error) { return s.containersRepoPath(settings) })
}

// validateItemRepoID refuses a per-item repository choice (#204) at the HTTP
// boundary instead of at the next backup, where it would surface as a restic
// error inside a run record nobody is watching. Empty is always valid and
// means "use the domain repository".
//
// It checks what itemRepoPath checks at use time (the repository exists and
// is switched on), so a choice that passes here still works when the backup
// runs. DeleteNamedRepoIfUnused counts users and deletes in one transaction,
// so a repository cannot disappear in between.
//
// A direct repository serves only the domain of its target.
func (s *Service) validateItemRepoID(domain, id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil
	}
	named, err := s.store.GetNamedRepo(id)
	if err != nil {
		return errors.New("no such repository; pick one from the list in Settings")
	}
	if !named.Enabled {
		return fmt.Errorf("the repository %q is switched off", named.Name)
	}
	if named.CompanionOf != "" {
		target, _, err := s.store.GetOffsiteTarget(named.CompanionOf)
		if err != nil {
			return fmt.Errorf("read the target of %q: %w", named.Name, err)
		}
		if target.Domain != domain {
			return errForeignDomain
		}
	}
	if _, err := s.resolveRepo(named.Repo); err != nil {
		return fmt.Errorf("the repository %q does not resolve to a usable location: %w", named.Name, err)
	}
	return nil
}

// containerRepoForName is repoFor for one container, with the signature the
// call sites already have (a name and a source), so a per-item repository
// reaches them without each one reading the store itself.
//
// An off-site source still resolves to the domain's off-site target, as in
// fileSetRepoFor: off-site copies are configured per domain, and where an
// item keeps its primary says nothing about where its replica lives.
//
// A container without a stored row has no override and takes the domain
// repository; that is the first backup, not an error. Every other store error
// is returned: treating a locked database as "no row" would send the backup
// to the domain repository while the item's snapshots sit in its own, a green
// run with a history split across two places. itemRepoPath refuses that
// case, and this function sits in front of it.
func (s *Service) containerRepoForName(settings store.Settings, name, source string) (string, error) {
	if isOffsiteSource(source) {
		return s.repoFor(settings, "containers", source)
	}
	tg, err := s.store.GetTargetByContainer(name)
	if errors.Is(err, sql.ErrNoRows) {
		return s.containersRepoPath(settings)
	}
	if err != nil {
		return "", fmt.Errorf("read this container's repository: %w", err)
	}
	return s.containerRepoPath(settings, tg)
}

// vmRepoForName is containerRepoForName's twin for the VMs domain, including the
// no-row-is-not-an-error rule.
func (s *Service) vmRepoForName(settings store.Settings, name, source string) (string, error) {
	if isOffsiteSource(source) {
		return s.repoFor(settings, "vms", source)
	}
	vm, err := s.store.GetVMTargetByName(name)
	if errors.Is(err, sql.ErrNoRows) {
		return s.vmsRepoPath(settings)
	}
	if err != nil {
		return "", fmt.Errorf("read this VM's repository: %w", err)
	}
	return s.vmRepoPath(settings, vm)
}

// vmRepoPath resolves a VM's per-item repository override (#204), falling back
// to the VMs domain repository.
func (s *Service) vmRepoPath(settings store.Settings, vm store.VMTarget) (string, error) {
	return s.itemRepoPath(vm.Repo, func() (string, error) { return s.vmsRepoPath(settings) })
}

// fileSetRepoFor is fileSetRepoPath with a source, mirroring repoFor: an
// off-site source still resolves to the domain's off-site target, because an
// off-site copy is configured per domain and a per-set override says nothing
// about where its replica lives. Only the primary is per set.
func (s *Service) fileSetRepoFor(settings store.Settings, set store.FileSet, source string) (string, error) {
	if isOffsiteSource(source) {
		return s.repoFor(settings, "files", source)
	}
	return s.fileSetRepoPath(settings, set)
}

// flashZipExportDir resolves the operator-configured output folder for the
// scheduled flash zip export. Unlike flashRepoPath, which may hand a remote
// backend like "s3:…" straight to restic, this is always a local folder, so
// it applies only the containment half of resolveRepo: paths.Resolve rejects
// absolute paths and traversal.
func (s *Service) flashZipExportDir(settings store.Settings) (string, error) {
	dir, err := paths.Resolve(s.cfg.HostMountRoot, settings.FlashZipExportPath)
	if err != nil {
		return "", fmt.Errorf("flash zip export: resolve path: %w", err)
	}
	return dir, nil
}

// configSnapshotDir is the staging directory for the config self-backup: a
// consistent, restic-ready copy of BombVault's own /config state, rebuilt
// before each config backup and removed afterwards. It lives under DataDir so
// it travels with the /config mount.
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
	// creds, the ssh private key). Error paths below return before BackupConfig
	// has registered its `defer os.RemoveAll(stagingDir)`, so clean up here
	// unless the end is reached with ok=true.
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

// retentionPolicy maps the stored settings to a restic keep-policy.
func (s *Service) retentionPolicy(settings store.Settings) restic.RetentionPolicy {
	return restic.RetentionPolicy{
		KeepLast:    settings.RetentionKeepLast,
		KeepDaily:   settings.RetentionKeepDaily,
		KeepWeekly:  settings.RetentionKeepWeekly,
		KeepMonthly: settings.RetentionKeepMonthly,
	}
}

// offsiteRetentionPolicy is the separate keep-policy for the off-site repo,
// so it can be kept longer (archive) than the local copy. All-zero (the
// default) means no off-site pruning: the off-site repo keeps everything
// until the user sets this policy.
func (s *Service) offsiteRetentionPolicy(settings store.Settings) restic.RetentionPolicy {
	return restic.RetentionPolicy{
		KeepLast:    settings.OffsiteRetentionKeepLast,
		KeepDaily:   settings.OffsiteRetentionKeepDaily,
		KeepWeekly:  settings.OffsiteRetentionKeepWeekly,
		KeepMonthly: settings.OffsiteRetentionKeepMonthly,
	}
}

// targetOffsiteRetentionPolicy is the per-destination off-site keep-policy,
// the plural successor to offsiteRetentionPolicy, which reads the single
// global columns. All-zero means keep everything, as the global default does.
// A backfilled N=1 target carries the global policy.
func targetOffsiteRetentionPolicy(t store.OffsiteTarget) restic.RetentionPolicy {
	return restic.RetentionPolicy{
		KeepLast:    t.RetentionKeepLast,
		KeepDaily:   t.RetentionKeepDaily,
		KeepWeekly:  t.RetentionKeepWeekly,
		KeepMonthly: t.RetentionKeepMonthly,
	}
}

// targetOffsiteLimits is the per-destination bandwidth cap (KiB/s). All-zero
// means unlimited. A backfilled N=1 target carries the global caps.
func targetOffsiteLimits(t store.OffsiteTarget) restic.Limits {
	return restic.Limits{
		UploadKBps:   t.LimitUpload,
		DownloadKBps: t.LimitDownload,
	}
}

// offsiteModeForTarget builds the restic mode for one off-site destination.
// It starts from the global mode (ModeFor sets Env and StorageClass from the
// shared CloudCreds), then applies the credential set the target names via
// CredsRef, overriding Env and, unless the target sets its own, StorageClass.
// A target with an empty CredsRef keeps the shared credentials. A target
// with an empty StorageClass, such as a backfilled N=1 target the SQL
// migration could not populate, keeps whichever class its credential set
// carries; overwriting it unconditionally would wipe the class for single
// off-site installs.
func (s *Service) offsiteModeForTarget(settings store.Settings, target store.OffsiteTarget) restic.Mode {
	mode := s.applyTargetCreds(s.ModeFor(settings), settings, target)
	if target.StorageClass != "" {
		mode.StorageClass = target.StorageClass
	}
	return mode
}

// applyTargetCreds overrides mode's Env (and storage class) with the
// credential set a target names; a target with an empty CredsRef is returned
// untouched.
//
// The off-site path (offsiteModeForTarget) and the primary path
// (primaryModeFor) share it because both ask the same row type the same
// question, and a second copy would let the two answers drift apart.
func (s *Service) applyTargetCreds(mode restic.Mode, settings store.Settings, target store.OffsiteTarget) restic.Mode {
	if strings.TrimSpace(target.CredsRef) == "" {
		return mode
	}
	c, err := s.decodeCloudFor(settings, target.CredsRef)
	if err != nil {
		log.Printf("api: target %s: cloud creds decode failed (ignoring, falling back to shared): %v", target.ID, err) //nolint:gosec // G706: target.ID is an opaque store-generated id
		return mode
	}
	mode.Env = cloudEnv(c)
	if c.S3StorageClass != "" {
		mode.StorageClass = c.S3StorageClass
	}
	return mode
}

// retentionPolicyForSource returns the keep-policy for a repo source: the
// off-site policy for any off-site source, the local policy otherwise. The
// off-site policy here is the settings-level one.
func (s *Service) retentionPolicyForSource(settings store.Settings, source string) restic.RetentionPolicy {
	if isOffsiteSource(source) {
		return s.offsiteRetentionPolicy(settings)
	}
	return s.retentionPolicy(settings)
}

// applyRetention prunes the just-backed-up item to the configured keep-policy.
// id is the item's identity: container:<name>, vm:<name>, fileset:<name> or
// the fixed flash/config tag, plus a renamed container's or VM's aliases. The
// tags retentionTagsFor allows are forgotten as one group, so neither a path
// change nor a rename leaves older snapshots in a group that never ages out.
// Best-effort: a failure never fails the backup that just succeeded, but it is
// notified rather than only logged, because silently skipped retention lets
// the repo grow unseen for weeks.
//
// During a bulk run (the #95 bulk-suppress flag on ctx: scheduled multi-item
// loops and the manual "back up all" batches) the expensive --prune is
// deferred: each item's forget runs without prune, and PruneAfterBulk reclaims
// the space once after the whole loop instead of once per item, since the
// prune pass is the costly part and a 44-container night would otherwise pay
// for it 44 times over. Single and manual backups (and flash/config, which
// never set the flag) keep the immediate inline prune.
//
// domain identifies which domain repo belongs to, so repo can be checked for
// a remote primary flagged append-only in its safety settings (#152,
// primaryIsImmutable). Retention is then skipped entirely, as
// copyToOffsiteTarget skips its pass for an immutable off-site destination:
// an immutable repository has no separate off-site copy behind it, so this
// box's own credentials must not be able to prune its only backup.
//
// "Immutable" is asked of the repository, not of the domain. A named
// repository (#204) carries its own flag and may be a plain folder on a
// share, so this applies to a local path as readily as to a cloud bucket.
// Anything with no append-only flag anywhere is unaffected.
func (s *Service) applyRetention(ctx context.Context, repo string, settings store.Settings, mode restic.Mode, id entryIdentity, domain string) {
	p := s.retentionPolicyForRef(settings, s.refFor(settings, domain, repo))
	if !p.Any() {
		return
	}
	if s.primaryIsImmutable(domain, repo) {
		// Name the repository, so the operator can tell which one was spared while
		// the rest of the domain pruned normally.
		log.Printf("api: %s: retention skipped: %s is flagged append-only", domain, shortRepoName(repo)) //nolint:gosec // G706: domain is a fixed literal and the name is shortened
		return
	}
	// restic forget without a tag would select the whole repository.
	if id.tag == "" {
		log.Printf("api: %s: retention skipped: the item has no identity tag", domain) //nolint:gosec // G706: domain is a fixed literal
		return
	}
	prune := !bulkReplicateSuppressed(ctx) // bulk run: one batched prune after the loop
	tags, ok := s.retentionTagsFor(ctx, repo, mode, id)
	if !ok {
		return // paused, and said so: nothing is forgotten
	}
	if err := s.forgetWithLockHeal(ctx, repo, p, mode, tags, prune); err != nil {
		log.Printf("api: retention prune failed (backup is safe): %v", err)
		// The current name, the one an operator recognises after a rename.
		s.notifyRetentionFailed(ctx, id.tag, truncateRunErr(err))
	}
}

// retentionTagsFor returns the tags restic forget may take for id, and false
// when the pass must not run at all. forget selects by tag and has no time
// bound, so:
//   - an alias tag joins only while every snapshot under it predates the
//     link. One that does not, or whose listing fails, is left out and its
//     pre-link snapshots are kept.
//   - an entry whose current name is another entry's alias pauses while that
//     entry's pre-link snapshots sit under the name, or when the listing
//     fails, because its forget would age them under its own policy.
//
// Either way the cost is storage, never another entry's snapshots.
func (s *Service) retentionTagsFor(ctx context.Context, repo string, mode restic.Mode, id entryIdentity) ([]string, bool) {
	if len(id.aliases) == 0 && id.ceded == nil {
		return id.listTags(), true
	}
	listed, err := s.snapshotsForTags(ctx, repo, mode, id.listTags())
	if err != nil {
		if id.ceded != nil {
			log.Printf("api: retention of %s paused: listing it failed, and it may hold another entry's snapshots: %v", id.tag, err) //nolint:gosec // G706: tags are validated names
			return nil, false
		}
		for _, a := range id.aliases {
			log.Printf("api: retention: listing %s failed, so %s is left out of its retention and its snapshots are kept: %v", id.tag, a.tag, err) //nolint:gosec // G706: tags are validated names
		}
		return []string{id.tag}, true
	}
	tags, withheld, paused := id.retentionTags(listed)
	if paused {
		owner := id.ceded.owner
		if owner == "" {
			owner = "another entry"
		}
		log.Printf("api: retention of %s paused: it still holds snapshots of %s from before that entry was renamed away from the name, and forget cannot tell them apart; they are kept", id.tag, owner) //nolint:gosec // G706: tags are validated names
		return nil, false
	}
	for _, a := range withheld {
		log.Printf("api: retention: %s has snapshots from after it was linked to %s (the name is in use again), so it is left out of that retention and its older snapshots are kept", a.tag, id.tag) //nolint:gosec // G706: tags are validated names
	}
	return tags, true
}

// forgetWithLockHeal runs a ForgetPolicy pass after clearing a genuinely
// stale lock with plain `restic unlock` (removeAll=false), which applies
// restic's own staleness rules: a dead PID on this host, or any lock past
// restic's ~30-minute age threshold. forget needs an exclusive lock, so even
// a stale non-exclusive lock, which lets backups keep succeeding, would block
// every retention pass, and one orphan would fail a whole night's retention
// across all items. A live lock is not force-removed (see the body); it has
// the same bounded prior-incarnation hostname gap noted on CheckDomain.
func (s *Service) forgetWithLockHeal(ctx context.Context, repo string, p restic.RetentionPolicy, mode restic.Mode, tags []string, prune bool) error {
	// A live or concurrent lock is not force-removed: reads run --no-lock,
	// writes are serialized under the domain lock, and forget passes
	// --retry-lock to wait out a transient cross-process lock. Force-removing a
	// live lock cannot fix a live holder and endangers a running operation.
	s.unlockStale(ctx, repo, mode)
	return s.engine.ForgetPolicy(ctx, repo, p, mode, tags, prune)
}

// identityTags returns the distinct identity tags in snaps: container:, vm:,
// fileset: and stack: names and the fixed flash and config tags. Marker tags
// such as p1, p2 and live are not identities.
func identityTags(snaps []restic.Snapshot) []string {
	seen := map[string]bool{}
	var out []string
	for _, sn := range snaps {
		for _, t := range sn.Tags {
			isIdentity := t == "flash" || t == "config" ||
				strings.HasPrefix(t, "container:") || strings.HasPrefix(t, "vm:") ||
				strings.HasPrefix(t, "fileset:") || strings.HasPrefix(t, "stack:")
			if isIdentity && !seen[t] {
				seen[t] = true
				out = append(out, t)
			}
		}
	}
	return out
}

// applyRetentionPerIdentity applies policy per identity, one tag-scoped forget
// per item, then prunes once. Used where no single item is in scope (manual
// prune, off-site retention).
//
// A failed listing forgets nothing: the repo-wide paths-grouped pass ignores
// identities and aliases, so it would age a renamed entry's pre-link snapshots
// together with a later machine's under the same name. That pass runs only for
// a repository with no identity tag at all, one written before identity tags
// existed, so its retention does not silently stop.
func (s *Service) applyRetentionPerIdentity(ctx context.Context, repo string, p restic.RetentionPolicy, mode restic.Mode) error {
	if !p.Any() {
		return nil
	}
	snaps, err := s.engine.Snapshots(ctx, repo, mode)
	if err != nil {
		return fmt.Errorf("list snapshots for retention: %w", err)
	}
	tags := identityTags(snaps)
	if len(tags) == 0 {
		return s.forgetWithLockHeal(ctx, repo, p, mode, nil, true)
	}
	return s.applyRetentionToTags(ctx, repo, p, mode, tags, snaps)
}

// applyRetentionToTags forgets per identity and prunes once, for a caller that
// already knows what the repository holds. snaps is what the tags were read
// from, which is what decides how an alias's old name folds.
func (s *Service) applyRetentionToTags(ctx context.Context, repo string, p restic.RetentionPolicy, mode restic.Mode, tags []string, snaps []restic.Snapshot) error {
	groups, _ := s.foldAliasedIdentityTags(tags, snaps) // skipped tags are logged there and kept
	var errs []error
	for _, group := range groups {
		if fErr := s.forgetWithLockHeal(ctx, repo, p, mode, group, false); fErr != nil {
			errs = append(errs, fmt.Errorf("%s: %w", strings.Join(group, ","), fErr))
		}
	}
	if pErr := s.engine.Prune(ctx, repo, mode); pErr != nil {
		errs = append(errs, pErr)
	}
	return errors.Join(errs...)
}

// foldAliasedIdentityTags groups the identity tags that belong to one entry,
// so applyRetentionPerIdentity forgets them together. A rename leaves the old
// tag on the snapshots already written; forgotten on its own, that tag never
// receives another snapshot and its "keep last N" set never shrinks.
//
// An old name can be taken up again by a different machine, and restic forget
// selects by tag, not by time. So for a tag that is an alias's old name, the
// alias's link time decides (aliasClaim):
//   - the tag holds snapshots the alias claims next to ones it does not: two
//     machines' history under one tag. It gets no group and is returned in
//     skipped, because its own pass would age the owner's pre-link snapshots
//     out. An alias lookup that fails skips the tag the same way.
//   - every snapshot under it is the alias's: it folds into the owner's
//     group, unless a row holds the name and may back up under it at any
//     time. Then it stays a group of its own, keyed apart from that row's.
//   - none is: the later machine's own group.
//
// A domain whose rows cannot be read does not fold at all, since an empty
// liveNames would switch off the reused-name check. A missed fold costs one
// extra pass; a wrong one merges two entries' retention for good.
func (s *Service) foldAliasedIdentityTags(tags []string, snaps []restic.Snapshot) (groups [][]string, skipped []string) {
	domains := s.aliasFoldDomains()
	byCanon := map[string][]string{}
	var order []string
	for _, tag := range tags {
		canon, skip := s.foldTag(tag, snaps, domains)
		if skip {
			skipped = append(skipped, tag)
			continue
		}
		if _, seen := byCanon[canon]; !seen {
			order = append(order, canon)
		}
		byCanon[canon] = append(byCanon[canon], tag)
	}
	groups = make([][]string, 0, len(order))
	for _, canon := range order {
		groups = append(groups, byCanon[canon])
	}
	return groups, skipped
}

// aliasFoldDomain is one domain's rows as foldAliasedIdentityTags needs them.
type aliasFoldDomain struct {
	domain, prefix string
	idToName       map[string]string
	liveNames      map[string]bool
	readable       bool
}

func (s *Service) aliasFoldDomains() []aliasFoldDomain {
	c := aliasFoldDomain{domain: "container", prefix: "container:", idToName: map[string]string{}, liveNames: map[string]bool{}}
	if targets, err := s.store.ListTargets(); err != nil {
		log.Printf("api: retention: listing targets for alias fold: %v; leaving every container tag as its own identity", err)
	} else {
		c.readable = true
		for _, t := range targets {
			c.idToName[t.ID] = t.ContainerName
			c.liveNames[t.ContainerName] = true
		}
	}
	v := aliasFoldDomain{domain: "vm", prefix: "vm:", idToName: map[string]string{}, liveNames: map[string]bool{}}
	if vms, err := s.store.ListVMTargets(); err != nil {
		log.Printf("api: retention: listing VMs for alias fold: %v; leaving every VM tag as its own identity", err)
	} else {
		v.readable = true
		for _, t := range vms {
			v.idToName[t.ID] = t.Name
			v.liveNames[t.Name] = true
		}
	}
	return []aliasFoldDomain{c, v}
}

// foldTag is foldAliasedIdentityTags's decision for one tag: the group it
// joins, or skip.
func (s *Service) foldTag(tag string, snaps []restic.Snapshot, domains []aliasFoldDomain) (canon string, skip bool) {
	for _, d := range domains {
		name, ok := strings.CutPrefix(tag, d.prefix)
		if !ok {
			continue
		}
		a, err := s.store.AliasByOldName(d.domain, name)
		if errors.Is(err, sql.ErrNoRows) {
			return tag, false
		}
		if err != nil {
			log.Printf("api: retention: %s left out: could not read whether it is a renamed entry's old name: %v", tag, err) //nolint:gosec // G706: tags are validated names
			return tag, true
		}
		claim := newAliasClaim(d.prefix, a)
		if claim.mixed(snaps) {
			log.Printf("api: retention: %s left out: it holds a renamed entry's snapshots from before the rename next to a later machine's, and a forget by that tag cannot keep them apart; they are kept", tag) //nolint:gosec // G706: tags are validated names
			return tag, true
		}
		if !claim.claimsEvery(snaps) {
			return tag, false
		}
		if cur := d.idToName[a.TargetID]; d.readable && !d.liveNames[name] && cur != "" {
			return d.prefix + cur, false
		}
		// Only the alias owner's snapshots, under a name a row may hold as its
		// current one. That row folds its own aliases under this tag, so this
		// group needs a key none of them can share.
		return "former:" + tag, false
	}
	return tag, false
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
	msg := fmt.Sprintf("Applying the retention policy for %s failed, so old snapshots are not being pruned (the new backup itself is safe): %s", tag, detail)
	notify.Send(ctx, c, tag, notify.Event{Title: "BombVault", Message: subject + ": " + msg, OK: false})
	if s.unraidGate(c.Unraid) {
		if e := s.sendUnraidNotify(ctx, "BombVault: "+subject, msg, "warning"); e != nil {
			log.Printf("notify: unraid: %v", e)
		}
	}
}

// offsiteTargetsFor returns a domain's enabled off-site destinations from the
// store, in stable per-domain order (sort_order, then created_at). It is the
// plural successor to the single-repo Settings.*Offsite* columns: a backfilled
// single-off-site install yields a one-element slice, an unconfigured domain an
// empty one. A missing store (used by pure-settings unit tests) or a query error
// falls back to empty so callers apply their Settings fallback.
func (s *Service) offsiteTargetsFor(domain string) []store.OffsiteTarget {
	out, err := s.enabledOffsiteTargets(domain)
	if err != nil {
		log.Printf("api: offsite %s: list targets failed (falling back to settings): %v", domain, err) //nolint:gosec // G706: domain is a fixed literal
		return nil
	}
	return out
}

// enabledOffsiteTargets is offsiteTargetsFor with the store error returned,
// for a check that has to refuse when it cannot see every copy.
func (s *Service) enabledOffsiteTargets(domain string) ([]store.OffsiteTarget, error) {
	if s.store == nil {
		return nil, nil
	}
	targets, err := s.store.OffsiteTargetsForDomain(domain)
	if err != nil {
		return nil, err
	}
	var out []store.OffsiteTarget
	for _, t := range targets {
		if t.Enabled {
			out = append(out, t)
		}
	}
	return out, nil
}

// offsiteReplicationTargets is the destination set copyToOffsite replicates
// a domain to: the domain's enabled off-site targets or, when no target row
// exists (an install configured only through the legacy Settings columns, or
// configured after the backfill ran), a single target synthesized from those
// columns, so N=1 behaves the same whether or not the row was backfilled.
func (s *Service) offsiteReplicationTargets(domain string, settings store.Settings) []store.OffsiteTarget {
	return orSettingsOffsiteTarget(s.offsiteTargetsFor(domain), domain, settings)
}

// orSettingsOffsiteTarget is targets, or when there are none, the one target
// the legacy Settings columns configure, if any.
func orSettingsOffsiteTarget(targets []store.OffsiteTarget, domain string, settings store.Settings) []store.OffsiteTarget {
	if len(targets) > 0 {
		return targets
	}
	loc := offsiteRepoFromSettings(domain, settings)
	if loc == "" {
		return nil
	}
	return []store.OffsiteTarget{settingsOffsiteTarget(domain, settings, loc)}
}

// settingsOffsiteTarget synthesizes the off-site target the backfill would
// have produced from the legacy Settings.*Offsite* columns, so the target
// loop keeps single-destination behavior for an install that was never
// backfilled. StorageClass stays "" because the global S3 class lives in
// CloudCreds and is supplied by ModeFor, and "" preserves it (see the
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

// aggregateTamper folds a domain's off-site tamper verdicts worst-of across
// its destinations for the ransomware scorecard: had is true only when every
// destination has a recorded verdict, protected only when every one refused
// the delete, and at is the oldest verdict timestamp, so the least recently
// checked destination drives the overdue judgement. A single-destination
// domain reads exactly LatestTamperTest(domain).
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
// currency worst-of across its off-site destinations: ok only when every
// destination has landed a successful copy, and at is the oldest of those,
// since the least recently replicated destination sets the domain's
// freshness. A single-destination domain reads exactly
// LatestSuccessfulOffsiteRun(domain).
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

// offsiteRepoFor returns the configured off-site repo location for a domain,
// or "" when none is set: the first enabled off-site target's, falling back
// to the legacy Settings column when no target row exists (for N=1 the
// backfilled target's Repo equals that column).
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

// offsiteScheduleFor returns the per-domain off-site replication schedule.
// Empty means "replicate after every local backup"; a non-empty cadence
// means the scheduler drives replication, decoupled from backups.
//
// The cadence comes from the per-domain Settings column and nothing else.
// Settings › Schedules edits that column and the scheduler registers each
// "<domain>-offsite" cron entry from it (see offsite() in
// internal/schedule/schedule.go). If another source said "decoupled" while
// the column is blank, the coupled after-backup copy would stand down with
// no cron entry to replace it, and the domain would silently stop
// replicating (#150). An off-site target row can carry a schedule value (the
// CRUD API accepts it and a settings import restores it), but per-target
// schedules are not a feature: every target replicates on its domain's
// off-site schedule and OffsiteTargetsSection exposes no such control, so
// the row value is ignored here.
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
// enforces it; the flag changes BombVault's own behaviour: replication skips
// the off-site retention prune, and off-site delete and prune are refused.
// Unlock stays allowed, because rest-server permits lock removal in
// append-only mode and clearing a stale lock is operationally required.
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

// The refusals for a destructive operation against a repository flagged
// append-only: credentials on this box must not be able to delete history,
// so BombVault does not even try.
//
// There are three toggles on three different cards plus the case where the
// flag cannot be read, so each refusal names the way out for its own card.
// None says "far side": a named repository (#204) can be a plain folder on a
// share, with no far side and no maintenance window to wait for.
//
// None contains a slash: every error leaving the API goes through
// scrubError, whose absolute-path regex redacts any slash-led token.
var (
	// A named repository (#204). Its toggle is on the Repositories card.
	errOffsiteAppendOnly = errors.New("this repository is append-only, so nothing here may delete from it. Turn Append-only off for it under Settings, Repositories, delete what you meant to delete, and switch it back on")
	// An off-site destination. Its toggle is on the off-site destinations card.
	errAppendOnlyOffsiteTarget = errors.New("this off-site destination is append-only, so nothing here may delete from it. Turn Append-only off for it under Settings, Off-site, delete what you meant to delete, and switch it back on")
	// A domain's own remote primary. Its toggle is in the Remote safety dialog
	// beside that domain's backup path.
	errAppendOnlyPrimaryRemote = errors.New("this repository is append-only, so nothing here may delete from it. Turn Append-only off in the Remote safety settings beside this domain's backup path, delete what you meant to delete, and switch it back on")
	// Nobody's toggle: the store could not be read. primaryIsImmutable answers
	// yes then, and sending the operator to a card to switch off a flag that may
	// not exist anywhere wastes a diagnosis on a transient failure.
	errAppendOnlyUnknown = errors.New("whether this repository is append-only could not be read, so nothing here may delete from it. That is the safe answer rather than a flag anybody set. Try again in a moment, and check the BombVault log if it keeps happening")
)

// DomainStatusEntry is the per-domain RPO (protection) status: whether a
// domain's backups are current relative to its schedule. It drives the
// dashboard's green/amber/red "are my backups current?" indicator.
type DomainStatusEntry struct {
	Domain   string `json:"domain"`   // "containers" | "vms" | "flash" | "config" | "files"
	Enabled  bool   `json:"enabled"`  // domain switched on in Settings
	Schedule string `json:"schedule"` // the domain's own cadence string (e.g. "daily 02:30")
	// CoveredBy carries the "Backup Everything" cadence when that pass is the
	// only thing backing this domain up: its own schedule is off, but the pass
	// includes it. It is empty whenever the domain has a schedule of its own, so
	// a client can tell "the domain's cadence" from "covered by the whole-server
	// pass". Without it a domain the pass backs up nightly would read "Not
	// scheduled" (#177).
	CoveredBy     string `json:"coveredBy"`
	LastSuccess   int64  `json:"lastSuccess"`   // unix time of the last successful backup, 0 = none
	PeriodSeconds int64  `json:"periodSeconds"` // expected RPO window in seconds, 0 = no expectation
	Status        string `json:"status"`        // "off" | "never" | "overdue" | "warn" | "ok"
	// LastVerified is the unix time of the last local restore-verification drill
	// (`restic check --read-data-subset`), 0 = never verified. LastVerifiedOK is
	// its outcome. These drive the dashboard's "last verified restorable" badge
	// without an extra round-trip.
	LastVerified   int64 `json:"lastVerified"`
	LastVerifiedOK bool  `json:"lastVerifiedOK"`
	// VerifiedDetail and DrillDetail carry the scrubbed failure reason of the
	// last local subset drill and the last off-site DR drill, so the dashboard
	// can show why and which check failed (#30). Both are "" on success.
	VerifiedDetail string `json:"verifiedDetail"`
	DrillDetail    string `json:"drillDetail"`

	// Ransomware-protection scorecard facts: whether the domain has an off-site
	// copy, whether it is flagged append-only (immutable), and the age-stamped
	// outcomes of the three protection checks (the active tamper test, the
	// off-site replication and the off-site DR drill). Protection is the
	// red/amber/green aggregate (see protectionLevel); it is "" for a disabled
	// domain, which the dashboard then leaves blank. These extend /api/status so
	// the dashboard card needs no second round-trip.
	OffsiteConfigured bool `json:"offsiteConfigured"`
	// OffPremisesCovered: every item of the domain lives somewhere that is a site
	// of its own, so the dashboard does not claim there is no copy off the premises.
	OffPremisesCovered bool  `json:"offPremisesCovered"`
	OffsiteImmutable   bool  `json:"offsiteImmutable"`
	LastTamperAt       int64 `json:"lastTamperAt"`
	LastTamperOK       bool  `json:"lastTamperOK"`
	LastReplicationAt  int64 `json:"lastReplicationAt"`
	LastReplicationOK  bool  `json:"lastReplicationOK"`
	LastDRDrillAt      int64 `json:"lastDrDrillAt"`
	LastDRDrillOK      bool  `json:"lastDrDrillOK"`
	// LastOffsiteSubsetAt and LastOffsiteSubsetOK stamp the latest off-site
	// subset drill (`restic check --read-data-subset` against the off-site repo),
	// the cheaper integrity check every domain, VMs included, can run alongside
	// the DR sandbox-restore drill. They drive the dashboard's "off-site
	// verified" badge (#63), independent of the DR fields above.
	LastOffsiteSubsetAt int64 `json:"lastOffsiteSubsetAt"`
	LastOffsiteSubsetOK bool  `json:"lastOffsiteSubsetOK"`
	// OffsiteDrillScheduled is true only when the scheduler runs an off-site DR
	// drill for this domain (DrillsEnabled and OffsiteDrillsEnabled set, and an
	// off-site repo configured). When it is false but the domain has an off-site
	// repo, the dashboard shows a muted "manual only" pill instead of a red
	// drFailed (#37).
	OffsiteDrillScheduled bool   `json:"offsiteDrillScheduled"`
	Protection            string `json:"protection"` // "" (disabled) | "red" | "amber" | "green"

	// Per-check states derived from the same inputs Protection aggregates (see
	// protectionChecks), so the dashboard card renders each checklist row as a
	// pure function of the backend and never contradicts the chip. EncryptionOn
	// and PruneStrategySet are the two config-level facts the card also renders,
	// served here so it needs no separate /api/settings round-trip.
	TamperState      string `json:"tamperState"`      // "" | "never" | "failed" | "stale" | "ok"
	ReplicationState string `json:"replicationState"` // "" | "never" | "overdue" | "ok" | "paused"
	DrillState       string `json:"drillState"`       // "" | "never" | "failed" | "overdue" | "ok"
	EncryptionOn     bool   `json:"encryptionOn"`     // repo encryption is enabled
	PruneStrategySet bool   `json:"pruneStrategySet"` // an off-site retention strategy is configured
}

// rpoStatus is the pure status decision from the inputs, so it can be unit-tested
// exhaustively without a store. scheduled is true when the domain is enabled and
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

// domainCoverage answers how often one domain is backed up, and by what,
// given its own cadence and the "Backup Everything" cadence. It returns the
// RPO window in seconds and, when the pass is the only thing covering the
// domain, that pass's cadence string.
//
// Backup Everything runs the domains as a sixth, independent pseudo-domain,
// so a user can leave every per-domain schedule off and let the pass do the
// work (#177). The per-domain cadence alone then answers zero: the status
// shows "Not scheduled" for a domain backed up nightly, and the overdue
// watchdog, whose job is to notice when backups stop, falls silent for it.
//
// When both are scheduled the window is the shorter of the two, since
// whichever fires more often bounds how stale a backup can get. The pass
// only touches enabled domains, so the caller's own enabled check still
// governs.
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
	lastReplicationAt int64 // last successful replication (currency source)
	offsitePeriod     int64 // seconds; 0 = replication coupled to each backup (no own schedule)
	lastBackupAt      int64 // last successful backup (coupled-replication currency basis)
	backupPeriod      int64 // seconds; the domain's backup RPO period (coupled-grace basis)
	lastDRDrillAt     int64
	lastDRDrillOK     bool             // outcome of the latest DR drill (only meaningful when lastDRDrillAt != 0)
	drillPeriod       int64            // seconds; 0 = no drill schedule
	paused            bool             // replication waits for the placement default to be confirmed
	byTarget          bool             // copy rules decide per target; targets replaces the domain-wide pair above
	targets           []targetCurrency // the enabled targets items are copied to
}

// replicationState decides the off-site replication currency (""/never/overdue/ok)
// from the same inputs protectionLevel and protectionChecks share, so the chip and
// the checklist row can never disagree.
//
//   - No off-site configured → "" (there is no replication to be current; the
//     missing-off-site case is handled as red by protectionLevel).
//   - Decoupled (offsitePeriod>0): the standard rpoStatus against the off-site's
//     own schedule, using the last successful replication.
//   - Coupled (offsitePeriod==0, the default): replication rides each backup, so
//     the claim is "the last successful backup has a corresponding successful
//     off-site copy". It goes overdue only once the gap between the last backup and
//     the last successful replication exceeds a grace of 2× the backup period
//     (conservative: a backup replicating shortly after is fine; a never-replicated
//     backup is flagged only once it has sat unreplicated beyond the grace). Amber,
//     never red.
//
// A paused domain reports "paused". With copy rules each enabled target is judged
// by the items copied there, and the worst of them counts; a target no item is
// copied to has no claim to make.
func replicationState(now int64, in protInputs) string {
	if !in.offsiteConfigured {
		return ""
	}
	if in.paused {
		return "paused"
	}
	if !in.byTarget {
		return targetReplicationState(now, in, in.lastBackupAt, in.lastReplicationAt)
	}
	worst := ""
	for _, t := range in.targets {
		if st := targetReplicationState(now, in, t.lastBackupAt, t.lastReplicationAt); replicationRank(st) > replicationRank(worst) {
			worst = st
		}
	}
	return worst
}

// targetReplicationState is the currency of one target, or of the domain as a
// whole, from the last successful backup and the last successful copy.
func targetReplicationState(now int64, in protInputs, lastBackupAt, lastReplicationAt int64) string {
	if in.offsitePeriod > 0 {
		switch rpoStatus(now, lastReplicationAt, in.offsitePeriod, true) {
		case "overdue":
			return "overdue"
		case "never":
			return "never"
		default:
			return "ok"
		}
	}
	// Coupled path: only meaningful once a backup exists and there is an RPO basis.
	if lastBackupAt == 0 || in.backupPeriod <= 0 {
		return ""
	}
	grace := in.backupPeriod * 2
	if lastReplicationAt == 0 {
		// Never replicated: overdue only once the backup has sat unreplicated > grace
		// (a just-made first backup replicating shortly after must not instantly flag).
		if now-lastBackupAt > grace {
			return "overdue"
		}
		return "ok"
	}
	if lastReplicationAt < lastBackupAt && lastBackupAt-lastReplicationAt > grace {
		return "overdue"
	}
	return "ok"
}

// replicationRank orders the states from nothing to claim up to overdue.
func replicationRank(state string) int {
	switch state {
	case "overdue":
		return 3
	case "never":
		return 2
	case "ok":
		return 1
	}
	return 0
}

// protectionLevel aggregates a domain's ransomware-protection posture into a
// red/amber/green chip. The far side enforces immutability, so this never
// goes green on configuration claims alone:
//
//   - ""    the domain is disabled; the dashboard shows nothing for it.
//   - red   the domain is enabled but has no off-site copy at all, or the
//     off-site is flagged immutable yet the append-only guarantee is
//     unproven: the tamper test is missing, last failed, or is stale (older
//     than 2× its schedule period). A non-immutable off-site makes no
//     append-only claim, so a missing tamper test does not make it red.
//   - amber protection exists but a scheduled time-check is overdue by the
//     same period-doubling rule backups use (rpoStatus "overdue"): the
//     off-site replication (only with a decoupled off-site schedule) or the
//     off-site DR drill (only with a drill schedule). Also amber when the
//     latest scheduled DR drill failed: the chip can't read green over the
//     red "failed" drill row, but other protections may still be fine.
//   - green otherwise.
//
// An off-site copy that is simply not flagged immutable stays green here: it
// is a real copy, and whether the far side can enforce append-only is a
// property of the destination the user picked, not a failed check. The
// dashboard's scorecard still marks that row amber rather than grey, so the
// gap is visible without the chip nagging about a chosen setup; it is the
// one place a row's colour and this chip differ (see appendOnlyRow in
// Dashboard.tsx).
func protectionLevel(now int64, in protInputs) string {
	if !in.enabled {
		return "" // disabled domains carry no protection posture
	}
	if !in.offsiteConfigured {
		return "red" // enabled but no off-site copy, so unprotected
	}
	if in.offsiteImmutable {
		tamperStale := in.tamperPeriod > 0 && now-in.lastTamperAt > in.tamperPeriod*2
		if !in.hadTamper || !in.lastTamperOK || tamperStale {
			return "red" // an append-only claim that cannot currently be proven
		}
	}
	// Replication currency: overdue is amber. Decoupled off-sites use their own
	// schedule; coupled (default) off-sites are checked against the last backup
	// with a conservative grace (see replicationState), so off-site health shows
	// in the configuration most users run. A paused replication copies nothing
	// at all, which is amber as well.
	if st := replicationState(now, in); st == "overdue" || st == "paused" {
		return "amber"
	}
	// A recorded DR drill that failed downgrades the chip to amber, never green
	// over a red row. The guard matches protectionChecks' "failed" branch (a
	// drill schedule is set and the latest recorded drill failed), so the chip
	// and the scorecard row never disagree on a failed drill.
	if in.drillPeriod > 0 && in.lastDRDrillAt != 0 && !in.lastDRDrillOK {
		return "amber"
	}
	if rpoStatus(now, in.lastDRDrillAt, in.drillPeriod, in.drillPeriod > 0) == "overdue" {
		return "amber"
	}
	return "green"
}

// protChecks is the per-check state the ransomware scorecard renders. Tamper
// and Replication derive from the same protInputs protectionLevel
// aggregates, so those rows never contradict the chip. Drill also honors the
// latest DR drill's outcome (a failed drill reads "failed", red) to agree
// with the off-site "proven restorable" pill; protectionLevel downgrades
// that case to amber, so a red Drill row never sits next to a green chip. An
// empty state means the check makes no claim and is rendered muted, not as
// a failure.
type protChecks struct {
	Tamper      string // "" | "never" | "failed" | "stale" | "ok"
	Replication string // "" | "never" | "overdue" | "ok" | "paused"
	Drill       string // "" | "never" | "failed" | "overdue" | "ok"
}

// protectionChecks mirrors protectionLevel for Tamper and Replication and
// layers the DR-drill outcome on top of currency for Drill:
//
//   - Tamper ∈ {never,failed,stale} is precisely the immutable branch that
//     turns the chip red; a non-immutable off-site makes no append-only
//     claim, so "".
//   - Replication "overdue" is precisely the amber branch (rpoStatus
//     "overdue", the same period-doubling rule). "never" and "ok" stay
//     non-amber so they match a green chip.
//   - Drill mirrors that currency (never/overdue/ok) for a passed drill, but
//     a recorded DR drill that failed reads "failed" (red) regardless of
//     recency, so the row agrees with the off-site "proven restorable" pill
//     (lastDRDrillOK). protectionLevel downgrades this case to amber, so the
//     red row never sits next to a green chip.
//
// Replication does not surface a red "replication failed": nothing else
// consumes lastReplicationOK, so only its currency is mirrored.
func protectionChecks(now int64, in protInputs) protChecks {
	var c protChecks

	// Tamper (append-only): only an immutable off-site makes an append-only claim.
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

	// Replication currency: decoupled off-sites use their own schedule; coupled
	// (default) off-sites are checked against the last backup with a grace (see
	// replicationState). "" when there is nothing to claim yet.
	c.Replication = replicationState(now, in)

	// DR drill outcome and currency, only when a drill schedule is set. A
	// recorded DR drill that failed reads "failed" (a red row) regardless of
	// recency, so the row can't go green by currency while the off-site "proven
	// restorable" pill (lastDRDrillOK) reads red. "never" stays for no drill
	// yet; a passed drill keeps the overdue/ok currency logic.
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
	return s.domainStatusFrom(settings)
}

// domainStatusFrom is DomainStatus for a caller that already holds the settings
// row. Both status endpoints do: /api/status also serves the "Backup Everything"
// cadence from it, and /api/fleet/status has it from the token gate. Settings is
// an ~80-column row and nothing caches it, so handing it in saves each of them a
// second full read of the same row.
func (s *Service) domainStatusFrom(settings store.Settings) ([]DomainStatusEntry, error) {
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

		// A period is only meaningful for an enabled domain that something backs up
		// on a cadence, its own or the "Backup Everything" pass (see
		// domainCoverage). An unparseable cadence (the settings PUT validates)
		// collapses to period 0, "off".
		period, coveredBy := domainCoverage(d.schedule, settings.EverythingSchedule)
		scheduled := d.enabled && period > 0

		// The latest local restore-verification drill drives the "last verified
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
		var offPremisesCovered bool
		if validPlacementDomain(d.name) {
			copied, covered, cErr := s.placementCoverage(settings, d.name)
			if cErr != nil {
				log.Printf("api: status %s: placement could not be read, off-site stays as configured: %v", d.name, cErr) //nolint:gosec // G706: domain is a fixed literal
			} else {
				offsiteConfigured = offsiteConfigured && copied
				offPremisesCovered = covered
			}
		}
		offsiteImmutable := offsiteImmutableFor(d.name, settings)

		// Tamper facts are aggregated worst-of across the domain's off-site
		// destinations (protected only when every destination is, currency from the
		// oldest). A single-destination domain reads exactly LatestTamperTest(domain).
		hadTamper, lastTamperOK, lastTamperAt := s.aggregateTamper(d.name)
		// Currency uses the last successful replication, as backups use their last
		// success: a replication that keeps failing then reads stale, overdue and
		// amber instead of staying fresh off a failed attempt's timestamp.
		// Aggregated worst-of (the oldest successful copy across destinations);
		// LatestSuccessfulOffsiteRun(domain) for a single destination.
		lastReplicationAt, lastReplicationOK := s.aggregateReplicationCurrency(d.name)
		var lastDRDrillAt int64
		var lastDRDrillOK bool
		var drDetail string
		if dr, found, drErr := s.store.LatestRestoreDrillKind(d.name, "offsite", "dr"); drErr == nil && found {
			lastDRDrillAt = dr.At
			lastDRDrillOK = dr.OK
			drDetail = dr.Detail
		}
		// The latest off-site subset drill (an integrity check against the off-site
		// repo) drives the dashboard's "off-site verified" badge (#63). It is the
		// only off-site drill available for VMs (DR restores are refused for them),
		// so it is read for every domain. Best-effort like the reads above.
		var lastOffsiteSubsetAt int64
		var lastOffsiteSubsetOK bool
		if sub, found, subErr := s.store.LatestRestoreDrillKind(d.name, "offsite", "subset"); subErr == nil && found {
			lastOffsiteSubsetAt = sub.At
			lastOffsiteSubsetOK = sub.OK
		}

		// The DR-drill currency only makes a claim when the scheduler runs off-site
		// DR drills (DrillsEnabled and OffsiteDrillsEnabled); otherwise a stale
		// lastDRDrillAt must not read overdue. Opting out of the scheduled off-site
		// DR drill (#37) leaves drillPeriod at 0, so DrillState is "" (muted) and
		// protectionLevel ignores DR, whose branches both require drillPeriod>0.
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
		in.paused, in.byTarget, in.targets = s.placementCurrency(settings, d.name)
		// The chip (protection) and each row (checks) derive from the same
		// protInputs. The Tamper and Replication rows mirror the chip's red and
		// amber branches exactly. The Drill row also honors the latest drill's
		// outcome (a failed drill reads a red "failed"), and protectionLevel
		// downgrades a failed drill to amber under the same guard, so no row can
		// contradict the chip.
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
			OffPremisesCovered:    offPremisesCovered,
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
// "config" | "files") used to attribute each run to its domain, the same
// mapping handleRuns uses: container targets, VM targets, file sets, and the
// singleton flash/config ids. Best-effort: an unknown id (e.g. a deleted
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
// for every local calendar day in [startUnix, endUnix] (ascending), tallying
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
			continue // outside the window; the query already bounds it
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

// repoStatsMinInterval is the minimum age of the latest sample before a
// backup re-collects repo stats. Stats (two restic stats passes over the
// whole repo) are expensive, so once a day is plenty for a size/dedup trend,
// and a domain backed up many times an hour samples only once.
const repoStatsMinInterval = 20 * time.Hour

// CollectStats samples a domain's repository size for source ("local"/"offsite")
// and records it for the size/dedup trend. It is best-effort and idempotent: a
// missing or empty (zero-snapshot) repo records nothing and returns nil, so it
// never turns an otherwise-good backup into a failure. Any restic error is
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
	mode := s.repoModeFor(settings, domain, source, repo)
	snaps, err := s.engine.Snapshots(ctx, repo, mode)
	if err != nil {
		return err
	}
	if len(snaps) == 0 {
		return nil // empty repo, nothing to measure
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

// claimStatsRun takes the in-flight slot for domain+source, reporting false when
// someone else already holds it. See statsMu's own comment for why this is a set
// rather than a singleflight.
func (s *Service) claimStatsRun(key string) bool {
	s.statsMu.Lock()
	defer s.statsMu.Unlock()
	if s.statsRunning[key] {
		return false
	}
	if s.statsRunning == nil {
		s.statsRunning = map[string]bool{}
	}
	s.statsRunning[key] = true
	return true
}

// releaseStatsRun frees the slot. Deferred by every claimant, so a panic in a
// sampling goroutine cannot wedge sampling for the life of the process.
func (s *Service) releaseStatsRun(key string) {
	s.statsMu.Lock()
	delete(s.statsRunning, key)
	s.statsMu.Unlock()
}

// collectStatsGuarded runs one sample unless one is already running for this
// domain+source. The throttle is re-read after the slot is taken: a caller
// that missed the throttle and then waited while the holder finished and
// wrote its row would otherwise re-measure a repo that was just measured.
func (s *Service) collectStatsGuarded(ctx context.Context, domain, source string) error {
	key := domain + "/" + source
	if !s.claimStatsRun(key) {
		return nil
	}
	defer s.releaseStatsRun(key)
	if s.statsSampledRecently(domain, source) {
		return nil
	}
	return s.CollectStats(ctx, domain, source)
}

// statsSampledRecently reports whether domain+source already has a sample
// younger than repoStatsMinInterval. A read error counts as not recently
// sampled: sampling is best-effort and a broken read should not silently
// stop it forever.
func (s *Service) statsSampledRecently(domain, source string) bool {
	latest, found, err := s.store.LatestRepoStat(domain, source)
	if err != nil || !found {
		return false
	}
	return time.Since(time.Unix(latest.At, 0)) < repoStatsMinInterval
}

// RepoStats returns the recorded repo-size samples for a domain + source
// (ascending by time), a thin passthrough to the store.
func (s *Service) RepoStats(domain, source string, limit int) ([]store.RepoStat, error) {
	return s.store.ListRepoStats(domain, source, limit)
}

// maybeCollectStats samples a domain's local repo size after a successful
// backup, throttled to repoStatsMinInterval so frequent backups don't re-scan
// the repo each time. It never blocks or fails the backup: the work runs in a
// detached goroutine (request values kept, cancellation dropped, with its own
// timeout) and any error is only logged. Call this on each domain's success
// path.
func (s *Service) maybeCollectStats(ctx context.Context, domain string) {
	if s.statsSampledRecently(domain, "local") {
		return // sampled recently enough
	}
	// Detach from the request (keep its values) so the sampling survives the
	// handler returning, with a hard cap so a wedged restic can't leak a goroutine.
	bg := context.WithoutCancel(ctx)
	go func() {
		cctx, cancel := context.WithTimeout(bg, 5*time.Minute)
		defer cancel()
		// Guarded, not bare: the check above reads a row this work only writes at
		// the end, so it is the throttle that cannot see a sample in progress.
		if err := s.collectStatsGuarded(cctx, domain, "local"); err != nil {
			log.Printf("api: stats: %s: collect failed (backup is safe): %v", domain, err) //nolint:gosec // G706: domain is a fixed literal
		}
	}()
}

// collectStatsAfterItem is the per-item success hook: it samples after a single
// backup, and does nothing when that backup is one item of a round.
//
// A round measures once, at the end, next to the batched prune and the
// batched off-site copy, keyed on the same "part of a round" flag those two
// use. Measuring per item piles up concurrent restic processes (see statsMu)
// and measures the wrong thing: a repo halfway through being written to,
// recorded N times a night into a series the Storage card plots as one point
// per day.
func (s *Service) collectStatsAfterItem(ctx context.Context, domain string) {
	if bulkReplicateSuppressed(ctx) {
		return
	}
	s.maybeCollectStats(ctx, domain)
}

// MaybeCollectStatsAfterBulk is the round's own sampling point, for the
// scheduler to call once a domain's loop is done. It is exported, like
// PruneAfterBulk and ReplicateOffsiteAfterBulk, because the after-bulk hooks
// are wired from main. It stays throttled and guarded, so a domain whose
// items run on their own per-item cadences cannot turn it into per-item
// sampling.
func (s *Service) MaybeCollectStatsAfterBulk(ctx context.Context, domain string) {
	s.maybeCollectStats(ctx, domain)
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
		// Guarded: this is the one sampling path a browser can drive without bound.
		// The Storage card asks for four domains on every mount, and the throttle
		// above cannot fire while a repo has no sample yet (found=false), so every
		// tab and remount would start another fan-out.
		if err := s.collectStatsGuarded(ctx, domain, source); err != nil {
			log.Printf("api: stats: %s/%s: async collect failed: %v", domain, source, err) //nolint:gosec // G706: domain/source are fixed-whitelist values
		}
	}()
}

// collectStatsSource normalises a stats source: any off-site source, bare
// "offsite" (primary target) or the per-target "offsite:<id>" form, samples
// the off-site repo and passes through unchanged; everything else collapses
// to "local". isOffsiteSource rather than a literal "offsite" compare lets a
// per-target source reach repoFor's off-site resolution instead of being
// clobbered to "local".
func collectStatsSource(source string) string {
	if isOffsiteSource(source) {
		return source
	}
	return "local"
}

// CollectStatsOnStartup samples each enabled domain's local repo shortly
// after boot so the Storage card shows data for repos that already have
// backups, instead of "no data" until the next backup runs. Best-effort and
// throttled.
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

// offsiteProgressHeartbeat is how often copyToOffsite re-publishes the
// indeterminate "replicating" progress event while a copy is in flight
// (#134). It is a var so tests can shrink it.
var offsiteProgressHeartbeat = 5 * time.Second

// copyToOffsite replicates a domain's local repo to its off-site destinations
// with `restic copy` (the local repo stays primary), one target after
// another. It creates each off-site repo on first use and copies everything
// not already there (restic skips dupes, so the first run seeds history and
// later runs ship just the new snapshot). It returns the scrubbed error so
// on-demand and scheduled callers can surface it, and never logs an off-site
// location, which can embed credentials. The caller holds the domain lock.
//
// item is the snapshot name a post-backup hook replicates for; the pass then
// visits only that item's targets. "" is the whole domain.
func (s *Service) copyToOffsite(ctx context.Context, domain string, settings store.Settings, item string, localRepos []domainRepoRef, skipped []repoSkip) (err error) {
	// One read of the rules, the default and the targets, before any restic call.
	p, perr := s.readPlacement(settings, domain)
	if perr == nil && p.State.Paused() {
		log.Printf("api: offsite %s: replication is paused until the placement default is confirmed", domain) //nolint:gosec // G706: domain is a fixed literal
		return nil
	}
	targets := p.enabledTargets()
	if perr == nil && item != "" {
		targets = p.effectiveTargets(item)
	}
	if len(targets) == 0 {
		if item != "" {
			return nil
		}
		return errNoOffsiteRepo
	}
	if perr == nil && p.TargetsUncertain && p.State.HasRules() {
		perr = errTargetsUncertain
	}
	var pass replicationPass
	if perr == nil {
		pass, perr = s.newPass(settings, p)
	}
	if perr == nil {
		localRepos, skipped = pass.placeSources(domain, localRepos, skipped)
	}
	if perr == nil && len(localRepos) == 0 {
		// nothingCoveredError, not skippedError: there are no sources at all, so
		// "covered only part of this domain" would be false.
		if sErr := nothingCoveredError(skipped); sErr != nil {
			return sErr
		}
		if len(skipped) > 0 {
			log.Printf("api: offsite %s: nothing to replicate: %s", domain, strings.Join(skipNames(skipped), ", ")) //nolint:gosec // G706: domain is a fixed literal and the names are the rows' own
			return nil
		}
		return errors.New("no repository to replicate")
	}
	if perr == nil {
		var paused bool
		var pending []string
		if paused, pending, perr = s.pauseOnFirstListing(ctx, settings, pass, targets, localRepos); paused && perr == nil {
			return nil
		}
		if perr == nil && len(pending) > 0 {
			targets = slices.DeleteFunc(slices.Clone(targets), func(t store.OffsiteTarget) bool {
				return slices.Contains(pending, t.ID)
			})
			if len(targets) == 0 {
				perr = errNoTargetVisited
			}
		}
	}
	// Additive kind="offsite" row in the shared runs table (StartRun/FinishRun
	// on the reserved domain target id, like prune/verify), so the replication
	// shows up in the dashboard Activity Log and Run History. It is one row per
	// domain per call. Best-effort.
	activityRunID, aErr := s.store.StartRun(domainRunTargetID(domain), "offsite")
	if aErr != nil {
		log.Printf("api: offsite %s: could not start activity run (continuing): %v", domain, aErr) //nolint:gosec // G706: domain is a fixed literal
		activityRunID = ""
	}
	// ok is set only when every target succeeded, so an unwinding panic (with
	// the named err still nil) can't stamp a phantom successful run; the
	// deferred finish then records a failure.
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
	if perr != nil {
		err = s.failPass(domain, targets, perr)
		return err
	}
	// Publish an active "off-site replication running" indicator for this
	// domain, so the UI shows which domain is replicating, with a live
	// percentage once one is available (#159; see restic.Copy and
	// progBeginCopySink). It stays per domain so the OffsiteIndicator and the
	// dashboard need no per-target handling; each target's live percentage comes
	// from copyToOffsiteTarget's progBeginCopySink call, keyed to this same
	// "offsite:"+domain event, so sequential targets (multiTarget) share one
	// indicator.
	//
	// startedAt is progBegin's single time.Now().Unix() capture, so the
	// heartbeat below publishes the exact same instant rather than a second
	// capture that could straddle a second boundary.
	_, startedAt := s.progBegin(ctx, "offsite:"+domain, "replicate")
	defer func() { s.progEnd("offsite:"+domain, "replicate", err == nil, startedAt) }()
	// Between a snapshot's live percentage updates, and during the tree walk
	// restic does before it copies packs (which prints no percentage at all),
	// lastSeen would only be touched at progBegin. The frontend's STALE_MS (15s,
	// web/src/lib/progress.ts) then hides the "running" dashboard line once a
	// quiet stretch passes that, which a real off-site copy easily does, and the
	// operation looks as if it vanished while it is still running (#134).
	// Re-publishing an active event periodically keeps lastSeen advancing.
	//
	// lastCopy carries the most recently published per-snapshot percentage (see
	// offsiteLastCopy), and a heartbeat tick republishes that, with
	// percent/snapshotIndex/snapshotTotal, rather than a blank Percent:0
	// placeholder that would overwrite the sink's last report; on a slow
	// transfer the heartbeat would otherwise win almost every render and hide
	// the live percentage on exactly the connections it is for. Until a real
	// update lands (before the first "copy started" line, or in a quiet stretch
	// with no per-snapshot signal) the heartbeat sends the bare "still alive"
	// event.
	//
	// Shutdown is a done-channel handshake, not a bare close: closing hbDone
	// only asks the goroutine to stop, and a `select` with both hbDone and the
	// ticker ready can still pick the ticker and publish one more tick. Waiting
	// on hbStopped blocks until the goroutine has returned (any in-flight
	// Publish included), so no tick can land after progEnd's terminal Publish
	// and resurrect a stale Active:true. The stop is deferred after progEnd's,
	// so it unwinds first.
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

	// Replicate to each destination best-effort: one target's failure is
	// recorded and logged but does not abort the others. The joined error
	// reaches on-demand and scheduled callers so a failure still reports and
	// notifies. multiTarget is false for a single-destination domain, whose
	// budget stays on the domain path (source "offsite", the global
	// OffsiteGrowthBudgetGB, latch keyed by domain); only with 2+ destinations
	// does each carry its own growth budget, size sample and latch.
	//
	// It counts the domain's enabled targets, not the targets this pass visits
	// (which a hook may narrow to one): a narrowed pass on a multi-destination
	// domain still has to sample and budget-check under that target's own
	// source, not the domain's bare "offsite" one, which resolves to the first
	// enabled target regardless of which one this pass reached.
	multiTarget := len(p.enabledTargets()) > 1
	var errs []error
	for _, t := range targets {
		visit, open := pass.visit(t)
		if !open {
			log.Printf("api: offsite %s: %s held nothing of this domain and gets nothing; not opened", domain, placementTargetName(t)) //nolint:gosec // G706: domain is a fixed literal, the name is the row's own
			continue
		}
		// The skip list travels with the sources. It names repositories this domain
		// uses that no source in localRepos could speak for, and the retention
		// decision at the far end needs it as much as the error does: aging a
		// destination under the off-site keep-policy while a repository feeding it
		// was never opened prunes the copy of exactly the items whose other copy is
		// the unreachable one. Reporting it only at the end (skippedError, below)
		// reaches the run row after every target's retention has already run.
		if cerr := s.copyToOffsiteTarget(ctx, domain, settings, t, localRepos, skipped, multiTarget, startedAt, lastCopy, visit); cerr != nil {
			log.Printf("api: offsite %s: copy to a destination failed (continuing): %v", domain, cerr) //nolint:gosec // G706: domain is a fixed literal
			errs = append(errs, cerr)
		}
	}
	if sErr := skippedError("this replication", skipped); sErr != nil {
		errs = append(errs, sErr)
	}
	if len(errs) > 0 {
		// Joined before the deferred bookkeeping reads it, so the run row does not
		// say "success" while the same pass tells the operator it covered only part
		// of the domain.
		err = errors.Join(errs...)
		return err
	}
	ok = true
	return nil
}

// copyToOffsiteTarget replicates a domain's local repo to a single off-site
// destination and records that destination's own offsite_runs row (stamped
// with offsite_target_id). Destination repo, restic mode (S3 storage class),
// retention, bandwidth limits and append-only flag are all per target.
// Bookkeeping is best-effort; it returns the scrubbed copy error. startedAt
// is copyToOffsite's single progBegin capture, so this target's live
// copy-progress events (see progBeginCopySink) carry the same StartedAt as
// the domain-level begin, heartbeat and terminal events. lastCopy is
// copyToOffsite's shared heartbeat state (see offsiteLastCopy), updated with
// every real percentage this target reports; nil means no heartbeat is
// watching, which offsiteLastCopy's nil-safe methods handle.
//
// skipped lists the repositories this domain uses that never made it into
// localRepos. The caller folds it into the returned error once for the
// whole pass, but the retention decision below has to see it, because a
// source dropped before the loop leaves no copyErr behind and would
// otherwise look like a destination that is fully in sync.
//
// visit is what the pass decided for this target: whether the rules choose
// the ids, and whether what the target holds is recorded.
func (s *Service) copyToOffsiteTarget(ctx context.Context, domain string, settings store.Settings, target store.OffsiteTarget, localRepos []domainRepoRef, skipped []repoSkip, multiTarget bool, startedAt int64, lastCopy *offsiteLastCopy, visit targetVisit) (err error) {
	// Persist this destination's replication attempt to the off-site run
	// history (begin now, close via defer with the outcome and scrubbed error).
	// The row holds duration and outcome only: the live per-snapshot percentage
	// fed to progBeginCopySink (#159) is a real-time SSE signal, and a finished
	// run's duration says as much after the fact. Bookkeeping is best-effort: a
	// store error is logged, never fatal. offsite_target_id attributes the run
	// to this destination (empty for a settings-synthesized N=1 target).
	runStarted := time.Now().Unix()
	runID, recErr := s.store.RecordOffsiteRunForTarget(domain, target.ID, runStarted)
	if recErr != nil {
		log.Printf("api: offsite %s: could not record replication run (continuing): %v", domain, recErr) //nolint:gosec // G706: domain is a fixed literal
		runID = 0
	}
	var ok, agingOnly bool
	defer func() {
		if runID == 0 {
			return
		}
		// Marked before the row turns green, so no reader sees a success that
		// counts for the currency and is none.
		if agingOnly {
			if mErr := s.store.MarkOffsiteRunAgingOnly(domain, target.ID, runStarted); mErr != nil {
				log.Printf("api: offsite %s: could not mark the run as aging only: %v", domain, mErr) //nolint:gosec // G706: domain is a fixed literal
			}
		}
		if ferr := s.store.FinishOffsiteRun(runID, ok, truncateRunErr(err)); ferr != nil {
			log.Printf("api: offsite %s: could not finish replication run: %v", domain, ferr) //nolint:gosec // G706: domain is a fixed literal
		}
	}()
	dest, rerr := s.resolveRepo(target.Repo)
	if rerr != nil {
		return fmt.Errorf("resolve off-site repo: %w", rerr)
	}
	// Relax the destination repo tree the same way every local backup relaxes
	// the primary repo (makeRepoReadable). restic, run as root, writes a local
	// repo 0700/0400, so an off-site copy on a mounted share is root-only on the
	// far side too: the share's other clients see the repo folders but nothing
	// inside them. An NFS to SMB switch exposes it: an Unassigned Devices NFS
	// mount reads the share as uid 0 and passes the 0700 dirs, while a CIFS
	// mount authenticates as an ordinary SMB user the far side denies (#138).
	// Deferred, so a copy that fails half-way, or a retention prune that writes
	// fresh index or pack files after it, is covered too;
	// makeOffsiteRepoReadable skips remote destinations and a path that is not
	// there yet.
	defer makeOffsiteRepoReadable(dest, s.cfg.DataDir)
	// Per-target restic mode carrying this destination's S3 storage class (see
	// offsiteModeForTarget: the global class is preserved for a backfilled N=1
	// target whose class is "").
	mode := s.offsiteModeForTarget(settings, target)
	if err = s.EnsureRepo(ctx, dest, mode); err != nil {
		return fmt.Errorf("ensure off-site repo: %w", err)
	}
	// Clear any stale lock a previously interrupted off-site op (replication
	// copy or integrity check) left on the destination repo, so restic copy can
	// take its lock instead of failing with "repository is already locked".
	// BombVault is the sole writer, so an existing off-site lock is always stale
	// (#29).
	s.unlockStale(ctx, dest, mode)
	// One listing of the destination serves the copy, the record of what it holds
	// and the names its keep-policy ages.
	dstSnaps, dstErr := s.listSnapshots(ctx, dest, mode)
	if dstErr != nil {
		log.Printf("api: offsite %s: could not list the destination before copying: %v", domain, scrubError(dstErr)) //nolint:gosec // G706: domain is a fixed literal, the error scrubbed here
	}
	out := s.copySources(ctx, domain, dest, mode, target, visit, localRepos, dstSnaps, dstErr, startedAt, lastCopy)
	copied, accounted, destIsASource := out.copied, out.accounted, out.destIsASource
	// Carried past the maintenance below: whatever did arrive is aged, sampled and
	// measured against the budget, and the joined error still reaches the run row.
	copyErr := errors.Join(out.errs...)
	// Gated on copyErr too: a pass that errored out before the keep-policy ran
	// must not stamp the run aging-only, or its history claims a maintenance
	// pass that never happened.
	agingOnly = visit.agingOnly && copied == 0 && copyErr == nil
	if visit.observe && dstErr == nil && !out.uncertain {
		s.recordListing(domain, target, visit.owners, dstSnaps, out.landed)
	}
	// Nothing arrived at all: the maintenance below is about what did arrive,
	// so running a forget and a prune over the destination after a completely
	// failed pass would age a replica no fresh snapshot reached.
	if copied == 0 {
		if copyErr != nil {
			err = copyErr
			return err
		}
		// No error and nothing copied means nothing was pending anywhere. The
		// destination is still a real replica that has to be sampled, so fall
		// through; the retention below decides for itself, on whether the pass was
		// error-free and whether any source answered for this domain at all.
		// `copied` alone cannot tell those apart.
		log.Printf("api: offsite %s: nothing was pending, nothing copied", domain) //nolint:gosec // G706: domain is a fixed literal
	}
	// Apply the off-site retention policy (separate from local) after a
	// successful copy, only when one is set, so an off-site repo defaults to
	// keep-everything (archive). Best-effort: a prune failure must not fail the
	// replication that already succeeded. An immutable (append-only) off-site
	// repo is never pruned from here: the far side would refuse the delete
	// anyway, and it enforces retention itself.
	settled := false
	switch {
	case destIsASource:
		// The destination holds a repository this domain backs up to. Whatever the
		// other sources managed, a forget plus prune here deletes snapshots whose
		// only copy is the thing being pruned. Refused, and logged, because the run
		// is otherwise reported as a partial success.
		log.Printf("api: offsite %s: not applying retention: this destination is itself one of the sources, so its snapshots have no second copy", domain) //nolint:gosec // G706: domain is a fixed literal
	case copyErr != nil:
		// Any failed source stops the retention, not only a pass where every source
		// failed. The destination is then missing exactly what the failed source
		// was carrying, and aging it under the keep-policy deletes history against
		// a replica nobody refreshed.
		//
		// The test is "the pass completed without error", not "something moved".
		// Gating on movement would never apply the policy on an all-named domain
		// after an in-sync pass, the strongest evidence that the far side is
		// current; gating on "every source failed" would let one failure out of two
		// read as a partial success and age the destination.
		log.Printf("api: offsite %s: not applying retention: a source could not be copied this pass", domain) //nolint:gosec // G706: domain is a fixed literal
	case len(unreachableSkips(skipped)) > 0:
		// A source that never reached the loop. offsiteReplicationSources puts a
		// repository that was established and is now unreachable into the skip list
		// rather than localRepos, so it produces no copyErr and the case above
		// cannot see it, yet the destination is missing exactly what that source
		// was carrying.
		//
		// Unreachable, not "actionable": the question is whether the destination
		// might be the last copy of something, and only a repository this box could
		// not open raises it. A repository the operator switched off still holds
		// its data; refusing on it would stop the domain's off-site retention for
		// good on installs where the post-backup hook is the only replication.
		log.Printf("api: offsite %s: not applying retention: %s could not be reached this pass", domain, strings.Join(skipNames(unreachableSkips(skipped)), ", ")) //nolint:gosec // G706: domain is a fixed literal and the names are the rows' own
	case accounted == 0:
		// Nothing answered for this domain. Every source either holds none of it or
		// was dropped, so there is no evidence that what the destination holds
		// still exists anywhere else, and a tag-scoped forget plus prune would run
		// against what may be the last copy.
		log.Printf("api: offsite %s: not applying retention: no source could account for this domain's snapshots this pass", domain) //nolint:gosec // G706: domain is a fixed literal
	case target.Immutable:
		log.Printf("api: offsite %s: retention is enforced far-side (append-only)", domain) //nolint:gosec // G706: domain is a fixed literal
	case agingOnly && visit.aged:
		log.Printf("api: offsite %s: %s was aged under these rules already; listed only", domain, placementTargetName(target)) //nolint:gosec // G706: domain is a fixed literal, the name is the row's own
	default:
		settled = s.ageTarget(ctx, domain, dest, mode, target, visit, dstSnaps, dstErr, out.landed)
	}
	s.noteAged(domain, target, visit, agingOnly, settled, len(out.landed) > 0)
	// Sample the off-site repo size into the repo_stats series and evaluate the
	// growth budget. With a budget set, sample synchronously first so the check
	// sees this replication's fresh size, including on the very first
	// replication, which has no prior sample; for an immutable repo (no
	// far-side prune) the budget is the only growth backstop, so it must not lag
	// or miss the seed. Without a budget, sample in the background (throttled)
	// for the Storage card. The REST protocol can't see the far side's free
	// space, only BombVault's own growth, so the budget is a detection aid, not
	// a hard cap.
	//
	// With a single destination the budget stays on the domain path: the size
	// is sampled under source "offsite", the global OffsiteGrowthBudgetGB
	// drives it, and the latch is keyed by domain. With 2+ destinations each
	// carries its own budget: the size is sampled under the per-target source
	// ("offsite:<id>"), the threshold is the target's GrowthBudgetGB, and the
	// latch is keyed by (domain,targetID).
	if !multiTarget {
		if settings.OffsiteGrowthBudgetGB > 0 {
			if serr := s.CollectStats(ctx, domain, "offsite"); serr != nil {
				log.Printf("api: offsite %s: budget size sample failed (replica is safe): %v", domain, serr) //nolint:gosec // G706: domain is a fixed literal
			}
		} else {
			s.CollectStatsAsync(domain, "offsite")
		}
		s.checkOffsiteBudget(ctx, domain, settings)
		if copyErr != nil {
			err = copyErr
			return err
		}
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
	if copyErr != nil {
		err = copyErr
		return err
	}
	ok = true
	return nil
}

// offsiteStatSource maps an off-site destination id to the repo_stats source
// that samples its size: bare "offsite" for a settings-synthesized target
// (empty id), so an un-backfilled N=1 install keeps sampling under
// "offsite", or "offsite:<id>" for a real per-target destination.
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

// checkOffsiteBudgetForTarget is checkOffsiteBudget for one off-site
// destination: it compares that destination's latest sampled size
// (repo_stats source "offsite:<id>") against the target's own
// GrowthBudgetGB and notifies once on each false→true crossing, latched per
// (domain,targetID). Only multi-destination domains use it; a
// single-destination domain stays on checkOffsiteBudget.
func (s *Service) checkOffsiteBudgetForTarget(ctx context.Context, domain string, target store.OffsiteTarget) {
	s.checkGrowthBudget(ctx, domain, offsiteStatSource(target.ID), offsiteBudgetLatchKey(domain, target.ID), target.GrowthBudgetGB, "off-site")
}

// checkOffsiteBudget compares the latest sampled off-site repo size for a domain
// against the configured growth budget (OffsiteGrowthBudgetGB, 0 = off) and fires
// a notification once on each false→true crossing. The latch (offsiteOverBudget)
// clears when growth drops back under budget so a later breach re-alarms. It reads
// the newest repo_stats row for domain+source="offsite"; if none exists yet (the
// async sample hasn't landed on the very first replication) it simply skips.
func (s *Service) checkOffsiteBudget(ctx context.Context, domain string, settings store.Settings) {
	s.checkGrowthBudget(ctx, domain, "offsite", domain, settings.OffsiteGrowthBudgetGB, "off-site")
}

// checkPrimaryRemoteBudget is checkOffsiteBudget's counterpart for a
// domain's remote primary (#152): when repo is remote and the domain has a
// saved primary-remote safety config (primaryRemoteTarget) with a growth
// budget, it measures the repo, which is the primary itself with no
// separate off-site copy, and compares against that config's own
// GrowthBudgetGB. The latch key is "primary:"-prefixed, so a domain that
// also has an off-site budget alarms for each independently. A local
// primary, or a remote one with no saved safety config, is a silent no-op
// like every other primary-remote safety feature when unconfigured.
func (s *Service) checkPrimaryRemoteBudget(ctx context.Context, domain, repo string, settings store.Settings) {
	if !restic.IsRemoteRepo(repo) {
		return
	}
	// …and it has to be the primary. Every call site passes the repository the
	// item it just backed up uses, which can be a named repository (#204),
	// while everything below is about the domain's own: the budget comes from
	// the primary-remote row, the alarm names the primary and the latch is
	// keyed "primary:"+domain. Charging a named repository's size to that
	// budget would alarm about a repository that is not the primary, and with
	// several items on different named repositories the latch would flap
	// between their sizes.
	own, oErr := s.repoFor(settings, domain, "local")
	if oErr != nil || !sameRepoLocation(own, repo) {
		return
	}
	t, ok := s.primaryRemoteTarget(domain)
	if !ok || !t.Enabled || t.GrowthBudgetGB <= 0 {
		return
	}
	// Measure synchronously, so the very first remote-primary backup after the
	// safety settings are saved is judged against a fresh size rather than a
	// stale or missing sample.
	//
	// One restic run, `stats --mode raw-data`, because the check reads exactly
	// one number. A full CollectStats sample also runs `snapshots` and the
	// expensive `stats --mode restore-size` (which walks every snapshot tree)
	// against the remote repo for every container of every round, and writes a
	// repo_stats row: N rows a night in a series the Storage card plots as one
	// point per day, closing the 20-hour throttle on the real sample (#189).
	// The primary repository is described by primaryModeFor: a named row at
	// that location carries its own credentials, storage class and caps, and a
	// size probe opened with the shared set answers about the wrong account.
	settingsMode := s.primaryModeFor(settings, domain, repo)
	raw, serr := s.engine.Stats(ctx, repo, "raw-data", settingsMode)
	if serr != nil {
		log.Printf("api: primary-remote %s: budget size measurement failed (backup is safe): %v", domain, serr) //nolint:gosec // G706: domain is a fixed literal
		return
	}
	// Compared directly rather than round-tripped via the store, which also
	// makes the first-backup case work: there is no "no sample yet".
	s.applyGrowthBudget(ctx, domain, "primary:"+domain, t.GrowthBudgetGB, "primary", raw.TotalSize)
}

// checkGrowthBudget is the shared compare-latch-notify core behind
// checkOffsiteBudget and checkOffsiteBudgetForTarget: it reads the newest
// repo_stats sample for domain+source, compares it against budgetGB (<=0 is
// off), and fires notifyOverBudget once per false→true crossing of the
// latchKey latch in offsiteOverBudget. The map is shared, so each caller
// passes its own collision-free key (offsiteBudgetLatchKey for a
// multi-target destination, the bare domain for the single off-site budget,
// a "primary:"-prefixed key for a remote primary) and a domain's off-site
// and primary budgets alarm independently. kind ("off-site" | "primary")
// only selects the wording; a missing sample (async collection has not
// landed, or the domain was never backed up) is skipped.
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
		return // no sample yet, nothing to compare
	}
	s.applyGrowthBudget(ctx, domain, latchKey, budgetGB, kind, stat.RawSize)
}

// applyGrowthBudget is checkGrowthBudget's compare-latch-notify half, for a
// caller that has just measured a size and should not write it to
// repo_stats and read it back (checkPrimaryRemoteBudget). The row is a
// user-visible series, not a scratch pad, and the budget check needs a
// number now rather than a sample.
func (s *Service) applyGrowthBudget(ctx context.Context, domain, latchKey string, budgetGB int, kind string, rawSize int64) {
	if budgetGB <= 0 {
		return // budget disabled
	}
	budgetBytes := int64(budgetGB) * 1024 * 1024 * 1024
	over := rawSize > budgetBytes

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
		s.notifyOverBudget(ctx, domain, rawSize, budgetBytes, kind)
	}
}

// notifyOverBudget sends a best-effort alert when a domain's off-site repo
// or remote primary (kind: "off-site" | "primary") first crosses its growth
// budget. It mirrors notifyProtectionLost/notifyDrillFailure's policy gate
// and Unraid fan-out; a no-op when notifications are off.
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
	notify.Send(ctx, c, domain, notify.Event{Title: "BombVault", Message: subject + ": " + msg, OK: false})
	if s.unraidGate(c.Unraid) {
		if e := s.sendUnraidNotify(ctx, "BombVault: "+subject, msg, "warning"); e != nil {
			log.Printf("notify: unraid: %v", e)
		}
	}
}

// offsiteReplicatesOnOwnSchedule reports whether a domain's off-site copy is
// driven by its own cron entry (a real, enabled cadence) rather than coupled
// to the backup run. A blank schedule and the literal "off" both parse to a
// disabled cadence and mean "coupled": the backup path owns replication.
// ParseCadence makes this decision match the scheduler's registration (the
// off-site cron entry registers only for an enabled cadence, schedule.go),
// so a domain can never fall between the two gates and silently never
// replicate (#95). A cadence that fails to parse defaults to coupled, the
// safe direction.
func (s *Service) offsiteReplicatesOnOwnSchedule(domain string, settings store.Settings) bool {
	cad, err := schedule.ParseCadence(s.offsiteScheduleFor(domain, settings))
	return err == nil && cad.Enabled
}

// bulkReplicateSuppressKey marks a context whose post-backup inline off-site
// replication is deferred to a single batched pass after the whole scheduled
// domain loop (#95). A blank off-site schedule couples replication to each
// backup, and in a scheduled multi-item run that means a full off-site repo
// open and index reload per item (44 containers, 44 high-latency B2
// round-trips, turning seconds into hours). The scheduled multi-item
// closures (containers, VMs, files) set this flag so each item's inline
// replication is skipped and the scheduler runs one
// ReplicateOffsiteAfterBulk per domain instead. A manual "Back up now" does
// not set it, so a single-item backup still replicates immediately.
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

// runGroupKey marks a context whose backup call is one child step of a
// "Backup Everything" pass, a sequential run over every domain (containers,
// vms, flash, files, config) triggered as one unit, so that for example a
// dead-man's-switch ping fires only once everything is done. The value is
// the parent run's id; runsAdapter and startedRunsAdapter read it via
// runGroupFromContext and stamp it onto the child run they just started
// (store.SetRunGroup), so that run can be traced back to its pass.
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
// belongs to, or "" when it is not part of a "Backup Everything" pass. A nil
// ctx (such as a zero-value runsAdapter built without one) counts as no
// group rather than panicking.
func runGroupFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	v, _ := ctx.Value(runGroupKey{}).(string)
	return v
}

// ReplicateOffsiteAfterBulk runs one off-site replication for a domain after
// a scheduled multi-item backup loop, replacing the per-item inline
// replications that run suppressed (#95). It is a no-op when the domain has
// no off-site repo, or when it replicates on its own off-site schedule (that
// path fires from its own cron entry). Like ScheduledReplicateOffsite it
// takes the domain lock, so it runs after the last item's backup releases
// it, and notifies on failure: a scheduled replication that failed silently
// would let the off-site copy rot unseen.
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
		// The domain's own "<domain>-offsite" cron entry replicates instead; the
		// scheduler registers it from this cadence. Log it, because a scheduled run
		// that backs up and prunes but never replicates otherwise looks like a
		// broken one in the activity log (#150).
		log.Printf("api: offsite %s: batched replicate skipped: this domain replicates on its own off-site schedule (%q)", domain, cadence) //nolint:gosec // G706: domain is a fixed literal, cadence is %q-quoted
		return
	}
	if err := s.ScheduledReplicateOffsite(ctx, domain); err != nil {
		log.Printf("api: offsite %s: batched replicate failed: %v", domain, err) //nolint:gosec // G706: domain is a fixed literal
	}
}

// replicateOffsite runs right after a successful local backup (the caller
// holds the domain lock). It replicates only when the domain has no
// separate off-site schedule: a blank schedule couples replication to each
// backup, a set one hands it to the scheduler. Best-effort: the local
// backup has already succeeded, so an off-site failure is logged, never
// propagated.
//
// localRepo is the repository the backup just wrote, which with named
// repositories (#204) need not be the domain's own, and copying exactly
// that one is the point: the hook exists to get the fresh snapshot off the
// box, and only that repository has it. The manual and scheduled paths
// cover the whole domain (offsiteReplicationSources), so the two agree.
//
// A remote named repository is skipped for the same reason it is skipped
// there: it is already off site, and one restic process cannot hold two
// clouds' credentials at once.
//
// item is the snapshot name of what the backup just wrote; the hook copies
// only to that item's targets. Flash and config pass "".
func (s *Service) replicateOffsite(ctx context.Context, domain string, settings store.Settings, localRepo, item string) {
	if bulkReplicateSuppressed(ctx) {
		return // scheduled multi-item run: replicated once after the whole loop (#95)
	}
	if s.offsiteRepoFor(domain, settings) == "" {
		return
	}
	if s.offsiteReplicatesOnOwnSchedule(domain, settings) {
		return // replicated on its own schedule, not after every backup
	}
	// The reference, not the bare string. This hook gets whatever repository
	// the item it just backed up uses, so it is the caller most likely to hold
	// a named repository, and the copy has to know that to narrow it to this
	// domain's snapshots. Deciding by position would never narrow here: a
	// one-element slice is always "index 0".
	ref := s.refFor(settings, domain, localRepo)
	if alreadyOffSite(ref) {
		log.Printf("api: offsite %s: this item's repository %s; not copied again", domain, offSiteReason(ref)) //nolint:gosec // G706: domain is a fixed literal
		return
	}
	// The skip list of the whole domain, even though this hook copies exactly
	// one repository. It is not a report (the hook only logs); the
	// destination's retention needs it.
	//
	// The retention at the far end is tag-scoped per identity over whatever the
	// destination holds, so a pass that opened one source ages the off-site
	// copies of every item in the domain. That is harmless while the other
	// sources are merely not copied this pass: they still exist. It is not
	// harmless when one of them is gone, because then the only copies of that
	// item's history are trimmed under the off-site keep-policy with nothing
	// left to restore them from. On an install with no separate off-site
	// schedule this hook is the replication, so it needs the same protection
	// as the whole-domain pass. Suppressing retention here outright would be
	// the other mistake: those installs would never age the destination at all.
	//
	// Narrowed to the unreachable ones, because the list also becomes the
	// pass's error, and a pass that copies one repository on purpose has no
	// business reporting that it did not cover the others. The whole list
	// would mark the domain's off-site run red after every backup on an install
	// with one switched-off repository.
	_, allSkips := s.offsiteReplicationSources(settings, domain)
	skipped := unreachableSkips(allSkips)
	if err := s.copyToOffsite(ctx, domain, settings, item, []domainRepoRef{ref}, skipped); err != nil {
		// domain is a fixed literal; the error is already path-scrubbed by restic.
		log.Printf("api: offsite %s: copy failed (local backup is safe): %v", domain, err)
		// The hook reports nothing else, so a replication that stopped over the
		// rules would otherwise go unnoticed.
		if errors.Is(err, errPlacementUnreadable) || errors.Is(err, errTargetsUncertain) {
			s.notifyReplicationFailed(ctx, domain, truncateRunErr(err))
		}
	}
}

// ReplicateOffsite replicates a domain's local repo to its off-site repo on
// demand, for the "Replicate now" button and the scheduled off-site job.
// Unlike the post-backup hook it surfaces the error (so the UI can report
// it) and takes the domain lock to serialise with backups.
//
// It copies every repository the domain's items write to (#204), not just
// the domain's own: an item pointed at a local named repository is part of
// this domain, and a "Replicate now" that quietly left it out would report
// success over a gap. See offsiteReplicationSources for which repositories
// qualify and why a remote named one does not.
func (s *Service) ReplicateOffsite(ctx context.Context, domain string) error {
	settings, err := s.store.GetSettings()
	if err != nil {
		return fmt.Errorf("read settings: %w", err)
	}
	if s.offsiteRepoFor(domain, settings) == "" {
		return errNoOffsiteRepo
	}
	defer s.lockDomainFor(domain, "replicate")()
	// The skip list goes in, so the run row this opens records it too rather
	// than being stamped a success the caller then contradicts.
	sources, skipped := s.offsiteReplicationSources(settings, domain)
	return s.copyToOffsite(ctx, domain, settings, "", sources, skipped)
}

// StartReplicateOffsite starts an on-demand off-site replication in the
// background and returns immediately (#93). A first replication of real
// data easily outlives browser and proxy timeouts, and a copy tied to the
// HTTP request context would be killed when the client gave up (504). The
// copy runs detached with its own generous ceiling; the off-site indicator
// shows it running, the run is recorded like any other, and a failure
// notifies like a scheduled replication. Configuration errors and a busy
// domain still surface synchronously so the UI can show them.
func (s *Service) StartReplicateOffsite(domain string) error {
	settings, _, err := s.domainRepoSource(domain, "local")
	if err != nil {
		return err
	}
	if s.offsiteRepoFor(domain, settings) == "" {
		return errNoOffsiteRepo
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
		// Detached from the HTTP request; the ceiling only guards against a copy
		// that hangs forever on a dead link.
		ctx, cancel := context.WithTimeout(context.Background(), 24*time.Hour)
		defer cancel()
		settings, err := s.store.GetSettings()
		if err != nil {
			log.Printf("api: offsite %s: replicate start: %v", domain, err) //nolint:gosec // G706: domain is a fixed literal
			return
		}
		sources, skipped := s.offsiteReplicationSources(settings, domain)
		err = s.copyToOffsite(ctx, domain, settings, "", sources, skipped)
		if err != nil {
			log.Printf("api: offsite %s: manual replication failed: %v", domain, err) //nolint:gosec // G706: domain is a fixed literal
			s.notifyReplicationFailed(ctx, domain, truncateRunErr(err))
		}
	}()
	return nil
}

// ScheduledReplicateOffsite runs a scheduled off-site replication and,
// unlike the interactive ReplicateOffsite (whose error the UI shows
// directly), notifies on failure. A scheduled replication that failed
// silently would let the off-site copy rot until the scorecard's currency
// check caught it. Best-effort notify; the error is still returned for the
// scheduler log.
func (s *Service) ScheduledReplicateOffsite(ctx context.Context, domain string) error {
	// Detached upper bound so a copy wedged on a dead link can't hold the
	// domain lock forever and block later backups (like the manual
	// StartReplicateOffsite ceiling). SkipIfStillRunning stops the next run
	// from piling on; this stops the current one from hanging indefinitely.
	ctx, cancel := context.WithTimeout(ctx, 24*time.Hour)
	defer cancel()
	err := s.ReplicateOffsite(ctx, domain)
	if err != nil {
		s.notifyReplicationFailed(ctx, domain, truncateRunErr(err))
	}
	return err
}

// notifyReplicationFailed sends a best-effort alert when a scheduled off-site
// replication fails (the off-site copy is not current). Mirrors
// notifyOverBudget/notifyProtectionLost's policy gate + Unraid fan-out; a no-op
// when notifications are off.
func (s *Service) notifyReplicationFailed(ctx context.Context, domain, detail string) {
	c, err := s.NotifyConfig()
	if err != nil || c.On == "" || c.On == "never" {
		return
	}
	subject := "Off-site replication FAILED for " + domain
	msg := fmt.Sprintf("The scheduled off-site replication for %s failed, so the off-site copy is not current: %s", domain, detail)
	notify.Send(ctx, c, domain, notify.Event{Title: "BombVault", Message: subject + ": " + msg, OK: false})
	if s.unraidGate(c.Unraid) {
		if e := s.sendUnraidNotify(ctx, "BombVault: "+subject, msg, "warning"); e != nil {
			log.Printf("notify: unraid: %v", e)
		}
	}
}

// TestOffsite probes a domain's off-site repo without modifying it, so the
// UI can tell the user whether the configured location is a reachable,
// initialised restic repository before relying on it. It uses the probe
// EnsureRepo uses to detect an existing repo, `restic cat config`
// (ResticEngine.RepoOpensErr), in both encryption modes, so a repo created
// under the opposite Encryption setting still counts as initialised
// (EnsureRepo reports that mismatch, not this).
//
// reachable reports whether the repo could be opened at all; initialized
// whether it is a real restic repository. A remote repo that is reachable
// but has never been replicated to fails `cat config` with restic's
// "repository does not exist", the signal isRepoUninitialized and
// listSnapshots treat as "reachable, just empty" (#117), so TestOffsite
// reports reachable=true, initialized=false, err=nil: the UI's
// "uninitialized" warning state, since BombVault creates the repo on the
// first replication (#130). Any other probe failure (a bad rclone remote
// type, wrong credentials, a backend refusing the connection) is a genuine
// reachability failure: neither reachable nor initialised, with err
// carrying the primary mode's probe failure, which the handler scrubs
// before it reaches the client. An unconfigured off-site repo for the
// domain is also an error.
func (s *Service) TestOffsite(ctx context.Context, domain string) (reachable, initialized bool, err error) {
	settings, err := s.store.GetSettings()
	if err != nil {
		return false, false, fmt.Errorf("read settings: %w", err)
	}
	loc := s.offsiteRepoFor(domain, settings)
	if loc == "" {
		return false, false, errNoOffsiteRepo
	}
	repo, err := s.resolveRepo(loc)
	if err != nil {
		return false, false, err
	}
	return s.probeOffsiteRepo(ctx, repo, s.ModeFor(settings))
}

// TestOffsiteTarget runs TestOffsite's probe against one off-site
// destination addressed by id, so every additional target can be verified
// on its own; TestOffsite only probes a domain's primary target, and an
// extra destination could otherwise stay broken behind a green "Test
// connection" (#138). The target's own S3 storage class is applied
// (offsiteModeForTarget) as replication does.
//
// An unknown id is an error rather than a fallback to the primary: a test
// that silently probed something else would hide exactly that failure.
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
// and TestOffsiteTarget: it opens an already resolved repo read-only in both
// encryption modes and classifies the outcome. See TestOffsite for the full
// contract of the three results.
func (s *Service) probeOffsiteRepo(ctx context.Context, repo string, mode restic.Mode) (reachable, initialized bool, err error) {
	// Bound each probe so a dead backend fails fast instead of hanging the
	// request (cat config over an unreachable REST server can otherwise stall).
	// Per attempt, not shared: a cold sftp connection over a VPN (Tailscale
	// tunnel plus host-key pinning) can eat a shared budget on the first try
	// and leave the second probe no time, reporting a reachable repo as
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
	// A reachable remote destination that simply has no repository yet (no
	// replication has run) is not a failure, the same signal listSnapshots
	// treats as "empty, not fatal" (#117). Report it as the UI's uninitialized
	// state instead (#130).
	if restic.IsRemoteRepo(repo) && isRepoUninitialized(primaryErr) {
		return true, false, nil
	}
	// Neither mode opened and it is not the uninitialized case: surface the
	// primary mode's failure (the user's configured encryption setting) instead
	// of a silent false/false, so the UI can show the real reason.
	//
	// This is the one place that holds the repository URL and the credentials it
	// was tried with at the same time, so a 401 whose cause is the two disagreeing
	// gets named here rather than guessed at downstream (#194).
	if named := restPathUserMismatch(primaryErr, repo, mode.Env); named != nil {
		return false, false, named
	}
	return false, false, primaryErr
}

// EnsureRepo makes sure the restic repo at repo is ready to use with the
// configured encryption mode. It is idempotent and reconciles the mode:
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
	// Fast path: the repo opens with the configured mode, so it exists and its
	// encryption mode matches. This is the common case on every backup after
	// the first.
	if s.engine.RepoOpens(ctx, repo, mode) {
		s.markRepoEstablished(repo) // remember it for the not-mounted guard (#55)
		return nil
	}
	// It did not open. Probe the opposite encryption mode (same backend creds):
	// if that opens it, the repo exists but was created under the other mode
	// because the user toggled the Encryption setting. Fail fast with an
	// actionable message rather than running Init (which would log "config
	// already exists") and failing every later backup against the mismatched
	// repo.
	if s.engine.RepoOpens(ctx, repo, s.oppositeMode(mode)) {
		return fmt.Errorf("this backup repository was created %s, but the Encryption setting is now %s, so "+
			"restic cannot open it after the change. Set Encryption back to %s, or point this backup at a "+
			"new, empty repository location",
			encryptionWord(!mode.Encrypted), enabledWord(mode.Encrypted), enabledWord(!mode.Encrypted))
	}
	// Opens with neither mode: not initialised (a brand-new location) or its
	// backing store vanished. Local repos need their directory; remote backends
	// do not.
	if !restic.IsRemoteRepo(repo) {
		// #55/#120: a repo was established here before but its `config` is now
		// missing. Two very different causes:
		//   - The backing store vanished, typically a remote share that mounts
		//     after the container started and is invisible right now.
		//     Re-initialising would write an empty repo that shadows the real
		//     backups once the share reappears, so refuse and surface "not
		//     mounted" (#55).
		//   - The destination is present, writable and mounted, but this repo is
		//     legitimately not there yet, e.g. a phantom marker from a pre-mount
		//     init on a UD disk that mounted over it later (#120). The established
		//     marker is permanent (nothing deletes it, so a restart cannot clear
		//     it), so the healthy disk has to be recognised and the repo
		//     re-established on it.
		// destinationMounted (kernel mount table, not a stat or write probe) tells
		// the two apart: an unmounted mountpoint dir is often still writable, which
		// is exactly the #55 case.
		if localRepoMissing(repo) && s.repoEstablished(repo) {
			if !s.destinationMounted(repo) {
				return ErrBackupPathNotMounted // #55: backing store not mounted
			}
			// #120: stale or phantom marker on a live disk. Drop it and fall through
			// to EnsureDir+Init so the repo is re-established on the mounted disk.
			s.clearRepoEstablished(repo)
		}
		// Genuine first run (marker unset): create the repo dir chain. The marker
		// guard above prevents re-initialising over an established but now
		// unmounted repo, so MkdirAll here is safe.
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
	// Mark established only when the repo verifiably opens now (a real config
	// was written), never on a no-op init, so the not-mounted guard can only
	// trip on a location that held a repo.
	if s.engine.RepoOpens(ctx, repo, mode) {
		s.markRepoEstablished(repo)
	}
	return nil
}

// ErrBackupPathNotMounted is returned when a local backup repo BombVault
// previously established is now unreadable, typically a remote share (e.g.
// under /mnt/remotes) that mounts after the container started and stays
// invisible to it until the mount is restored. BombVault refuses to
// re-initialise an empty repo over it, which would shadow the real backups,
// and surfaces this instead of a misleading "no backups" (#55).
var ErrBackupPathNotMounted = errors.New("backup path is not mounted yet: a remote backup share may mount late at boot; it recovers once the mount is available (restart BombVault if it persists)")

// markRepoEstablished records a successfully created/opened local repo so a later
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

// clearRepoEstablished removes the established marker for a local repo, used when
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

// repoEstablishment is the three-way answer to "was this local repository
// ever established?". reposThatExist, offsiteReplicationSources and
// discoverNamesAcrossRepos decide whether to report a missing repository
// and need to tell "it was never created, so nothing is missing" from "the
// store could not say". A bool cannot carry that: repoEstablished's
// false-on-error is the right default for EnsureRepo, where false means "go
// ahead and init", and the wrong one here, where it means "stay silent
// about a repository that may hold every backup".
type repoEstablishment int

const (
	repoNeverEstablished repoEstablishment = iota
	repoWasEstablished
	repoEstablishmentUnknown
)

// repoEstablishmentOf answers the question with its uncertainty intact. Remote
// repositories have no local backing store to vanish, so they are never tracked
// and answer "never".
func (s *Service) repoEstablishmentOf(repo string) repoEstablishment {
	if restic.IsRemoteRepo(repo) {
		return repoNeverEstablished
	}
	ok, err := s.store.IsRepoEstablished(repo)
	if err != nil {
		// One retry before giving up. The usual failure is a local SQLite read
		// losing a race with a concurrent write, which the next attempt almost
		// always wins; without the retry a nightly verify, prune, unlock and drill
		// would all report failure over a repository that never held anything,
		// because "could not tell" is reported rather than swallowed.
		if ok, err = s.store.IsRepoEstablished(repo); err != nil {
			log.Printf("api: is repo established (twice): %v", err)
			return repoEstablishmentUnknown
		}
	}
	if ok {
		return repoWasEstablished
	}
	return repoNeverEstablished
}

// repoEstablished reports whether a local repo destination was previously
// established. It is false for remote repos and on any store error, the
// permissive default EnsureRepo wants. A caller deciding whether to report
// a missing repository wants repoEstablishmentOf instead.
func (s *Service) repoEstablished(repo string) bool {
	return s.repoEstablishmentOf(repo) == repoWasEstablished
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

// ErrSelfBackup is returned when a backup targets BombVault's own container.
// Backing it up stops the container mid-run (stop → backup → start), which kills
// the very process doing the backup and takes the app down. Its configuration is
// recovered separately via the encrypted definition mirror (Discover), so there
// is nothing to gain and a crash to lose.
var ErrSelfBackup = errors.New("BombVault won't back up its own container (it would stop itself mid-backup); its configuration is recovered via Discover")

// selfContainerName returns BombVault's own container name, cached after the
// first success. BOMBVAULT_SELF_CONTAINER overrides the lookup. An empty result
// is not cached, so a call made before Docker is reachable is retried later.
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
		return "" // Docker not reachable or not in a container yet; retry next time
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
	// A backup must survive the client that triggered it disconnecting, whether
	// by closing the browser tab or by stopping the very container the
	// BombVault UI runs in. Detach from the request's cancellation (keeping its
	// values) with a generous hard cap so a wedged run can't hold the domain
	// lock forever.
	ctx, cancel := backupHoldCtx(ctx)
	defer cancel()
	// Detached from the caller, but reachable by shutdown: a closing browser
	// tab must not stop a backup, while the process itself leaving must, or the
	// run is killed anyway without anyone recording that it happened.
	s.registerBackupCancel("container:"+name, cancel)
	defer s.unregisterBackupCancel("container:" + name)
	// Never back up BombVault's own container: stopping it mid-run kills this process.
	if self := s.selfContainerName(ctx); self != "" && name == self {
		return backup.Summary{}, ErrSelfBackup
	}
	defer s.lockDomain("containers")() // serialise per repo; blocks maintenance ops meanwhile

	// #64: a domain-wide fault (repo mount lost, disk full, restic repo error)
	// that begins mid-batch trips one of the pre-flight early returns below
	// (settings, repo path, EnsureRepo, inspect, empty-paths guard, upsert) for
	// every remaining container. None of those reach backup.BackupContainer,
	// the only other place a failed run is recorded, so they would leave
	// nothing in the dashboard heatmap or history and only a bare "N failed"
	// count in the notification. Resolve the target up front and, via the
	// named-return finisher below, record a failed run carrying the real
	// reason for any such early return. Once BackupContainer takes over run
	// bookkeeping, orchestrated=true keeps this from double-recording; a
	// NotFound target is excluded too, since it records its own "skipped" run
	// (recordAndNotifyContainerSkip).
	var targetID string
	if tg, tErr := s.store.GetTargetByContainer(name); tErr == nil {
		targetID = tg.ID
	}
	orchestrated := false
	defer func() {
		if retErr != nil && !orchestrated && !errors.Is(retErr, backup.ErrContainerNotInstalled) {
			s.recordPreflightFailure("Backup", name, targetID, retErr)
		}
	}()

	settings, err := s.store.GetSettings()
	if err != nil {
		return backup.Summary{}, fmt.Errorf("read settings: %w", err)
	}
	item := store.ItemRef{Domain: "containers", Key: name}
	step, err := s.prepareHome(ctx, settings, item)
	if err != nil {
		return backup.Summary{}, err
	}

	in, err := s.docker.Inspect(ctx, name)
	if err != nil {
		// The container is gone but is still a scheduled target. A scheduled target
		// can outlive its container, so skip it with a sentinel instead of failing:
		// the scheduler treats ErrContainerNotInstalled as a skip, so the nightly
		// job neither errors nor false-alarms Healthchecks (#57). Record a
		// "skipped" run (so the dashboard shows it and agrees with the green
		// aggregate ping) and warn the user once. Mirrors the VM path (BackupVM →
		// ErrVMNotInstalled). Returns before any real run is recorded.
		if dockercli.IsNotFound(err) {
			log.Printf("api: Backup: skipping %q: not present on host (not installed; backups only)", name) //nolint:gosec // G706: name is %q-quoted
			s.recordAndNotifyContainerSkip(ctx, name)
			return backup.Summary{}, backup.ErrContainerNotInstalled
		}
		return backup.Summary{}, fmt.Errorf("inspect container: %w", err)
	}
	// The paths actually backed up: the explicit folder selection if set, else
	// the automatic appdata detection, filtered to those that exist. A
	// stateless container ends up with an empty list and a definition-only
	// backup (its template and inspect are still captured so it can be
	// recreated on restore). Both halves of the selection come out of one
	// read: the includes become the positionals, and selection feeds the
	// --exclude tail further down (see effectiveBackupPathsWithSelection).
	effective, selection := s.effectiveBackupPathsWithSelection(name, in)

	// Guard against a silent no-op: if a previous backup captured data (or the
	// user selected folders) but every path now resolves away, e.g. because
	// the appdata share isn't mounted or HOST_SOURCE_ROOT is misconfigured,
	// refuse instead of recording an empty backup that looks successful and
	// overwrites the stored path list. A first backup of a new or stateless
	// container is unaffected.
	//
	// Gone, not merely absent (#181): an empty result also happens when the
	// user has deselected every folder on purpose, which leaves the container
	// correctly stateless and makes a definition-only backup the right
	// outcome. The guard measures what its message claims: whether the data a
	// previous backup captured has disappeared from disk.
	if s.emptyBackupIsUnreachable(name, effective) {
		err := fmt.Errorf("backup %q: its backup folders are not reachable right now (is the appdata share mounted?). Refusing an empty backup that would look successful", name)
		s.notifyBackup(ctx, "container", name, false, backup.Summary{}, err)
		return backup.Summary{}, err
	}

	if step, err = s.recordHome(ctx, settings, item, step); err != nil {
		return backup.Summary{}, err
	}
	repo, mode := step.repo, step.mode

	// Persist the recreate recipe (self-contained: inspect + template + backup
	// paths) so restore works even after the container has been deleted.
	xml, _, _ := template.Read(s.cfg.FlashTemplatesDir, name)
	defBytes, _ := json.Marshal(containerDefinition{Inspect: in, TemplateXML: xml, AppdataPaths: effective})
	defJSON := string(defBytes)

	tg, err := s.store.UpsertTarget(store.Target{ContainerName: name, AppdataPaths: effective, Definition: defJSON})
	if err != nil {
		return backup.Summary{}, fmt.Errorf("upsert target: %w", err)
	}

	// Compile the per-root CACHEDIR.TAG toggles into the item-level union the
	// argv flag needs. tg comes from UpsertTarget's authoritative re-read (the
	// ON CONFLICT clause never touches exclude_caches), so this is a fresh read
	// of the stored map on every backup: a toggle saved between two backups
	// affects exactly the later one. The union runs over the stored map whether
	// or not that root is currently included in the selection, because
	// restic's --exclude-caches applies to every positional source and per-root
	// gating would be a lie either way.
	mode.ExcludeCaches = anyRootExcludeCaches(tg.ExcludeCaches)

	// The formerly: tags take the bare former names, and the mirrored definition
	// records each with its link time. A failed read costs this one backup
	// those tags and the mirror update, not the backup itself.
	aliases, aliasErr := s.store.TargetAliases("container", tg.ID)
	if aliasErr != nil {
		log.Printf("api: backup: aliases of %q: %v", name, aliasErr) //nolint:gosec // G706: %q-quoted
	}

	// Give each dependency its own run-state so the backup never starts a
	// container the user had already stopped (#33): inspect each by name and
	// carry WasRunning (mirroring the target's in.Running). A dependency that
	// cannot be inspected (e.g. removed) is logged and left untouched.
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

	// "Update after successful backup" recreates the container (stop, remove,
	// create) after the backup. Dependents already brought back by then would
	// break against the target while it is torn down (a dependent firing
	// against a down target gets connection refused, #119). Hand the update to
	// the orchestrator as its WhileDependentsStopped hook so it runs inside the
	// stop window and the dependents restart only after the recreate. Only for
	// a running, opted-in container; otherwise nil, and the dependents restart
	// right after the backup.
	var whileDepsStopped func()
	if tg.UpdateAfterBackup && in.Running {
		whileDepsStopped = func() { s.updateContainerAfterBackup(ctx, name, in, tg.ID) }
	}

	pkey := "container:" + name
	// Healthchecks /start ping: deferred to here, past every pre-flight early-return,
	// so the paired done/fail notifyBackup below always follows (no dangling /start).
	s.notifyBackupStart(ctx, "container")
	bctx, startedAt := s.progBegin(ctx, pkey, "backup")
	// BackupContainer owns run bookkeeping from here (it records its own failed
	// or success run), so the pre-flight failure finisher above stands down to
	// avoid a double record.
	orchestrated = true
	sum, err := backup.BackupContainer(bctx, backup.BackupDeps{
		ContainerRef:           name,
		FormerNames:            aliasOldNames(aliases),
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
		// User-owned exclude patterns stay first; the selection-derived tail
		// enforces the stored exclusion branches strictly below an included root,
		// so the snapshot content matches what the stored selection advertises.
		// It comes from the same read as the positionals above, never from
		// UpsertTarget's later re-read: those two reads could disagree, and a
		// --exclude from one selection biting a positional from another is a
		// snapshot nobody asked for. Patterns travel as typed builder arguments
		// into BackupArgs (excludes before --, positionals after), never through a
		// shell.
		Excludes:  append(s.resolveExcludePatterns(tg.Excludes, in), excludedBranches(selection)...),
		Docker:    s.docker,
		Restic:    &resticAdapter{engine: s.engine, mode: mode, extraTags: s.directTags(settings, "containers", repo)},
		Templates: templatesAdapter{},
		Runs:      runsAdapter{st: s.store, ctx: ctx, svc: s, cancelKey: "container:" + name},
	})
	s.progEnd(pkey, "backup", err == nil, startedAt)
	s.notifyBackup(ctx, "container", name, err == nil, sum, err)
	if err != nil {
		return backup.Summary{}, err
	}

	// Mirror the definition (encrypted) onto the backup storage so a freshly
	// installed BombVault can rebuild its state via Discover after losing
	// /config. Best-effort: a write failure must never fail a good backup.
	// Without its aliases the mirror would lose its link records, so it keeps
	// what the last write left.
	if aliasErr != nil {
		log.Printf("api: backup: WARN the stored definition of %q stays as it was, since its aliases could not be read", name) //nolint:gosec // G706: name is %q-quoted
	} else if wErr := s.writeDefToStorage(settings, name, repo, defBytes, aliases); wErr != nil {
		log.Printf("api: backup: WARN could not persist definition for %q to storage: %v", name, wErr) //nolint:gosec // G706: name is %q-quoted
	}
	// #52: the optional post-backup image update already ran, if enabled, as
	// the orchestrator's WhileDependentsStopped hook (above), inside the stop
	// window, so a recreate of the target completes before its dependents are
	// restarted (#119). The backup and fresh snapshot are its safety net; a
	// failure there is logged and recorded as a failed "update" run, but never
	// fails the backup.
	//
	// This container's compose stack, if it has one and this is a single
	// backup rather than one item of a round. The project directory does not
	// travel inside a member's own snapshot (see resolveAppdataPaths), so
	// backing up one member by hand would otherwise leave the shared folder
	// unprotected until the next full round. A round suppresses this and does
	// it once at the end, so the folder is walked once per project, not once
	// per service. It keys on the same flag as the batched off-site
	// replication, so there is one notion of "part of a round".
	if !bulkReplicateSuppressed(ctx) {
		// The stack's own retention forgets without pruning: the container's
		// applyRetention right below prunes once for both, the same repo either
		// way, instead of two separate prune passes for one manual backup.
		if err := s.BackupStacks(WithBulkReplicateSuppressed(ctx), []string{name}); err != nil {
			// Logged, never fatal: the container's own data is already safe.
			log.Printf("api: backup: %v", err)
		}
	}
	// A renamed container's "keep last N" counts across the rename, as long as
	// no other machine has used the old name since.
	s.applyRetention(ctx, repo, settings, mode, s.containerIdentity(name), "containers")
	makeRepoReadable(repo, s.cfg.DataDir) // keep the local repo copyable off-box by a non-root user
	s.replicateOffsite(ctx, "containers", settings, repo, "container:"+name)
	s.collectStatsAfterItem(ctx, "containers")
	s.checkPrimaryRemoteBudget(ctx, "containers", repo, settings)
	return sum, nil
}

// updateContainerAfterBackup implements the per-container "update after
// backup" opt-in (#52): pull the container's image and, only if a newer
// image arrived, recreate the container from its live inspect (which then
// resolves to the new image). It is recorded as its own "update" run so the
// recreate shows in Run History. Best-effort: a failure here never fails
// the backup, and the fresh snapshot lets the user roll back a bad update.
func (s *Service) updateContainerAfterBackup(ctx context.Context, name string, in model.Inspect, targetID string) {
	ref := in.Config.Image
	if ref == "" {
		ref = in.Image
	}
	// #106: images in a private or sponsor-gated registry need credentials:
	// resolve the ref's registry host against the stored registry credentials
	// and pass the match along ("" means no credential, an anonymous pull).
	if err := s.docker.PullWithAuth(ctx, ref, s.registryAuthFor(ref)); err != nil {
		// Reached, but couldn't even check for an update (registry rate limit,
		// auth, network). Record a failed "update" run so "why wasn't this
		// updated?" is answerable from Run History and the Activity Log instead of
		// only the server log (#95). The backup itself already succeeded.
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
	// Nothing newer arrived: no recreate, and no run record either (44
	// "nothing happened" rows a night would drown Run History). The check did
	// complete, so stamp it on the target and let the UI show "checked, up to
	// date" instead of something indistinguishable from "never reached".
	if newID == "" || newID == in.Image {
		s.setUpdateCheck(name, "up-to-date")
		return
	}
	// context.Background(), not ctx: an "update" run is a side effect of the
	// container backup, not one of the five domain steps a "Backup Everything"
	// pass groups (see runGroupKey), and recordUpdateFailure's failure path
	// leaves it ungrouped too, so grouping never depends on which path fires.
	// This applies to all three runsAdapter sites below; ctx stays in use for
	// the Docker calls.
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

	// #116: BombVault just recreated the container, so its image tag moved to
	// the new digest, but Unraid did not perform the update and still shows a
	// stale "update available" banner on the Docker tab because its cached
	// status file was never refreshed. Ask Unraid to run its own update-status
	// recheck over the existing host SSH link so it rewrites that file itself.
	// Only fires when an update happened; opt-out via the toggle, best-effort
	// and non-fatal.
	if st, sErr := s.store.GetSettings(); sErr == nil && st.ReconcileUnraidUpdateStatus {
		s.reconcileUnraidUpdateStatus(ctx, ref)
	}

	// #56: optionally remove the superseded old image. Opt-in (default off),
	// because the old image is what makes a fresh-snapshot rollback cheap.
	// Best-effort with force=false, so the daemon refuses while another
	// container still references the image (a shared base image is never
	// deleted).
	if st, sErr := s.store.GetSettings(); sErr == nil && st.PruneImageAfterUpdate && in.Image != "" {
		if rErr := s.docker.ImageRemove(ctx, in.Image); rErr != nil {
			log.Printf("api: update-after-backup: prune old image %s for %q: %v (kept)", shortID(in.Image), name, rErr) //nolint:gosec // G706: name is %q-quoted
		}
	}

	// #56: notify per updated container (opt-in), so the user can verify it
	// still works. It fires per container (updates are rare) rather than in the
	// scheduled summary. Healthchecks is suppressed so an update can't flip the
	// domain monitor; the message ctx carries no message-suppress flag, so it
	// delivers in summary mode.
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

// recreateForUpdate stops, removes and recreates+starts the container from
// its captured inspect, which after the preceding Pull resolves to the
// newer image. It mirrors the restore recreate path minus the appdata
// restore (the data is already current). Containers that share this one's
// network namespace (network_mode: container:<name>) may need a manual
// restart afterwards.
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
// update status for a container BombVault just recreated (#116), so on
// Unraid the Docker tab's stale "update available" banner clears. The
// host-side step lives behind s.platformFn() (see
// platform.Platform.ReconcileContainerUpdateStatus: Unraid's PHP-over-SSH
// recheck, a no-op on platforms without an equivalent). This is the
// best-effort boundary: a nil SSH link, a non-Unraid platform or a command
// error is only logged and never affects the backup or update outcome.
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

// recordUpdateFailure records a failed "update" run so a reached but failed
// post-backup update (a registry check that failed before it could tell
// whether a newer image exists) shows in Run History and the Activity Log
// rather than only the server log (#95). It also stamps the target's
// update-check outcome as 'failed'. Best-effort: the backup already
// succeeded, so a bookkeeping error here is only logged. An update that was
// available but failed to apply is recorded by the recreate path.
func (s *Service) recordUpdateFailure(name, targetID string, cause error) {
	s.setUpdateCheck(name, "failed")
	// This "update" run is never part of a "Backup Everything" pass's grouped
	// children (see runGroupKey), so context.Background() loses nothing.
	runID, rErr := runsAdapter{st: s.store, ctx: context.Background()}.Start(targetID, "update")
	if rErr != nil {
		log.Printf("api: update-after-backup: %q could not record update failure: %v (cause: %v)", name, rErr, cause) //nolint:gosec // G706: name is %q-quoted
		return
	}
	_ = runsAdapter{st: s.store, ctx: context.Background()}.Finish(runID, "failed", "", 0, truncateRunErr(cause))
}

// StartBackupAll launches a server-side batch backup of the named
// containers, running them sequentially in a background goroutine. Running
// on the server, it survives the browser that started it going away,
// including when the batch stops the very container the BombVault UI is
// open in. Self and blank names are skipped, and a single container
// failing is logged while the batch continues.
//
// It returns (false, nil) if a batch is already running (the caller answers
// 409), or (false, err) if the containers domain is already busy with
// another op (scheduled backup, restore, prune, ...), which the handler
// maps to a 409 instead of launching a goroutine that blocks silently on
// the lock. Progress is published under "batch:containers" for an overall
// indicator, while each container still publishes its own
// "container:<name>" bar.
//
// Unlike a scheduled domain run, which aggregates its Healthchecks pings
// into one per run (#49), this manual multi-select keeps the per-item ping:
// it backs up an arbitrary subset, not the whole domain, so pinging the
// domain check "success" here would reset the scheduled-cadence monitor and
// could mask an overdue scheduled backup.
func (s *Service) StartBackupAll(ctx context.Context, names []string) (bool, error) {
	if !s.batchActive.CompareAndSwap(false, true) {
		return false, nil
	}
	if op, busy := s.domainBusy("containers"); busy {
		s.batchActive.Store(false)
		return false, fmt.Errorf("%s is running on containers", op)
	}
	// Detach immediately so the run, and the self-detection it depends on, is
	// independent of the request that started it (canceled the moment the
	// handler returns). Each per-container Backup applies its own hard timeout,
	// so the batch needs no deadline of its own; WithoutCancel keeps request
	// values without a cancel func to leak. #95: each container's inline
	// off-site replication is suppressed and the whole batch is replicated once
	// after the loop (ReplicateOffsiteAfterBulk below), as on the scheduled
	// path.
	bctx := WithBulkReplicateSuppressed(context.WithoutCancel(ctx))
	go func() {
		// Batch-level safety net: a panic outside the per-item loop (ordering,
		// publishBatch, the post-loop prune and replicate) has no single container
		// to blame, so it is only contained. Each item gets its own, more precise
		// recovery (backupOneForBatch), so one bad container can't abort the rest
		// of the batch, just as a per-item error only counts against that item.
		defer s.recoverOperation("backup-all", nil, nil)
		defer s.batchActive.Store(false)

		self := s.selfContainerName(bctx)
		queue := make([]string, 0, len(names))
		for _, n := range names {
			if n != "" && n != self {
				queue = append(queue, n)
			}
		}
		// #119: order the batch by the user's explicit manual backup order first,
		// then most overdue first, as a scheduled run does. A store error here must
		// never abort the batch, so the selection order is the fallback.
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
					skipped++                                                                  // removed container: a skip (already recorded), not a batch failure (#57)
					log.Printf("api: backup-all: %q skipped: not installed (backups only)", n) //nolint:gosec // G706: n is %q-quoted
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
		// Each compose stack's project directory, once for the whole batch rather
		// than once per member. Under bctx its retention forgets without --prune,
		// like every member's, so the one prune below reclaims the space for both.
		if err := s.BackupStacks(bctx, queue); err != nil {
			// Never fails the batch: the members are already safely backed up,
			// and a project folder that could not be read is its own problem to
			// report rather than a reason to call the whole round failed.
			log.Printf("api: backup-all: %v", err)
		}
		// Retention first: one local prune for the whole batch (each container's
		// forget ran without --prune under the bulk flag), before the batched
		// off-site replication, so fewer snapshots are left to copy.
		s.PruneAfterBulk(bctx, "containers")
		// #95: one batched off-site replication after the whole manual batch (no-op
		// unless containers replicate on a blank/coupled schedule with an off-site
		// repo). The per-container inline copy was suppressed via the bulk flag on bctx.
		s.ReplicateOffsiteAfterBulk(bctx, "containers")
		// One repo-size sample for the whole round, here rather than after every
		// container (see collectStatsAfterItem). Last, so it measures the repo the
		// round actually left behind: after the prune reclaimed space and after the
		// off-site copy, not somewhere in the middle of both.
		s.maybeCollectStats(bctx, "containers")
		log.Printf("api: backup-all done: %d ok, %d skipped, %d failed (of %d requested %d)", ok, skipped, fail, total, len(names))
	}()
	return true, nil
}

// backupOneForBatch backs up a single queued container on behalf of
// StartBackupAll, containing any panic to this item (recoverOperation) so
// one container's crash counts as that item failing, as a normal error
// does, instead of aborting every container queued behind it. On a
// recovered panic it also closes out the run StartRun'd inside
// backup.BackupContainer as failed (failStuckRun): that call is a plain
// sequential call, not deferred, so it would otherwise never be reached and
// the run would sit "running" forever.
func (s *Service) backupOneForBatch(ctx context.Context, name string) (err error) {
	// recoverOperation must be deferred directly, not wrapped in a
	// `defer func(){...}()` closure: recover() only works when called directly
	// by a deferred function, and &err is how it hands the panic back to this
	// function's named return.
	defer s.recoverOperation("backup-all: "+name, &err, func(msg string) {
		if tg, tErr := s.store.GetTargetByContainer(name); tErr == nil {
			s.failStuckRun(tg.ID, msg)
		}
	})
	_, err = s.Backup(ctx, name)
	return err
}

// StartBackup launches a single container backup in a background goroutine
// and returns immediately. Like StartBackupAll, it runs on the server, so
// it survives the browser that started it going away, including when
// backing up the reverse-proxy container BombVault's UI runs through
// severs the request connection mid-backup. The per-container
// "container:<name>" progress bar keeps reporting over SSE so the SPA can
// watch completion.
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
				log.Printf("api: backup: %q skipped: not installed (backups only)", name) //nolint:gosec // G706: name is %q-quoted
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
			s.failStuckRun(id, msg) // id is the runs.target_id for a file set, no lookup needed
		})
		defer s.batchActive.Store(false)
		if _, err := s.BackupFileSet(bctx, id); err != nil {
			log.Printf("api: backup file set: %q failed: %v", id, err) //nolint:gosec // G706: id is %q-quoted
		}
	}()
	return true, nil
}

// StartBackupFilesAll launches sequential backups for the given file-set ids
// in one background batch and returns immediately, mirroring StartBackupAll
// for the files domain. There is no self-container to skip: every non-empty
// id is attempted, and failures are logged and counted without aborting the
// rest. Overall progress is published under "batch:files" while each set
// still publishes its own "files:<name>" bar. Shares batchActive; returns
// (false, nil) if a backup or batch is already running, or (false, err) if
// the files domain is already busy with another op.
func (s *Service) StartBackupFilesAll(ctx context.Context, ids []string) (bool, error) {
	if !s.batchActive.CompareAndSwap(false, true) {
		return false, nil
	}
	if op, busy := s.domainBusy("files"); busy {
		s.batchActive.Store(false)
		return false, fmt.Errorf("%s is running on files", op)
	}
	// Detach immediately so the batch is independent of the request that
	// started it (canceled the moment the handler returns). Each per-set
	// BackupFileSet applies its own hard timeout, so the batch needs no
	// deadline of its own. #95: each set's inline off-site replication is
	// suppressed and the whole batch is replicated once after the loop
	// (ReplicateOffsiteAfterBulk below), like the containers batch.
	bctx := WithBulkReplicateSuppressed(context.WithoutCancel(ctx))
	go func() {
		// See StartBackupAll's pair of defers: this one contains a panic outside
		// the per-item loop, while each item gets its own more precise recovery
		// (backupFileSetOneForBatch) so one bad set can't abort the rest.
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
		// Retention first: one local prune for the whole batch (each set's forget
		// ran without --prune under the bulk flag), before the batched off-site
		// replication, so fewer snapshots are left to copy.
		s.PruneAfterBulk(bctx, "files")
		// #95: one batched off-site replication after the whole manual batch (no-op
		// unless files replicate on a blank/coupled schedule with an off-site
		// repo). The per-set inline copy was suppressed via the bulk flag on bctx.
		s.ReplicateOffsiteAfterBulk(bctx, "files")
		// One sample for the whole round, exactly as the container batch does.
		s.maybeCollectStats(bctx, "files")
		log.Printf("api: backup-files-all done: %d ok, %d failed (of %d requested %d)", ok, fail, total, len(ids))
	}()
	return true, nil
}

// backupFileSetOneForBatch backs up a single queued file set on behalf of
// StartBackupFilesAll; see backupOneForBatch for why each item gets its own
// recovery. id is already the runs.target_id, so no lookup is needed.
func (s *Service) backupFileSetOneForBatch(ctx context.Context, id string) (err error) {
	// See backupOneForBatch for why this must be a direct defer, not wrapped.
	defer s.recoverOperation("backup-files-all: "+id, &err, func(msg string) {
		s.failStuckRun(id, msg)
	})
	_, err = s.BackupFileSet(ctx, id)
	return err
}

// BackupInProgress reports whether a single backup, a batch, or a restore is
// running (they share the same single-flight guard), so callers and tests
// can observe when the detached goroutine has finished.
func (s *Service) BackupInProgress() bool { return s.batchActive.Load() }

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

// publishBatch emits an overall batch-progress event (no-op without a store).
func (s *Service) publishBatch(key string, percent float64, active bool) {
	if s.progress == nil {
		return
	}
	s.progress.Publish(progress.Event{Key: key, Phase: "backup", Percent: percent, Active: active})
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

// Discover rebuilds BombVault's target list from the backup storage after a
// fresh install or loss of /config. It lists the containers repo's
// snapshots (tagged container:<name>), reads and decrypts each container's
// mirrored definition, and upserts a target so the container can be
// restored. The result counts the containers discovered. Containers whose
// definition is missing or undecryptable are skipped and logged.
//
// dryRun makes it read-only: it opens the repo and decrypts the definitions
// (proving the repo is reachable and the APP_KEY is correct) and returns
// the same count, but writes no targets. The Recovery tab's readability
// probe uses this, so checking "is my backup readable?" never resurrects
// orphan entries; only the explicit "Discover backups" action rebuilds
// targets (#44).
func (s *Service) Discover(ctx context.Context, dryRun bool) (DiscoverResult, error) {
	settings, err := s.store.GetSettings()
	if err != nil {
		return DiscoverResult{}, fmt.Errorf("read settings: %w", err)
	}
	// The distinct container names from the container:<name> tags, across every
	// repository this domain writes to, each with the named repository (#204) it
	// was found in. A not-yet-created repo yields nothing. A read failure comes
	// back as readErr together with whatever the named repositories yielded, so
	// an install whose domain repository is unreadable is still rebuilt as far
	// as it can be; the Recovery wizard classifies on that error.
	names, formerNames, skipped, directRows, readErr := s.discoverNamesAcrossRepos(ctx, settings, "containers", "container:")
	findings, fErr := s.directFindings("containers", directRows)
	if fErr != nil {
		log.Printf("api: discover containers: could not match direct repositories to targets: %v", fErr)
	}

	dir, err := s.defsDir(settings)
	if err != nil {
		return DiscoverResult{Direct: findings}, err
	}
	legacyDir, err := s.legacyDefsDir(settings)
	if err != nil {
		return DiscoverResult{Direct: findings}, err
	}
	res := DiscoverResult{Skipped: skipped, LeftOpen: []string{}, Direct: findings}
	unlock, locked := s.discoverLock("containers", dryRun)
	defer unlock()
	// storedDef reads name's mirrored definition. The item's own mirror comes
	// first: a container on a named repository has its definition beside its
	// snapshots, and the domain's mirror never had it.
	storedDef := func(name, repoID string) ([]byte, containerDefinition, error) {
		fn, err := defFileName(name)
		if err != nil {
			return nil, containerDefinition{}, err
		}
		lookIn := dir
		if own := s.itemDefsDir(repoID, false); own != "" {
			lookIn = own
		}
		enc, err := readStoredDef(lookIn, legacyDir, fn)
		if err != nil && lookIn != dir {
			enc, err = readStoredDef(dir, legacyDir, fn) // older backups mirrored to the domain
		}
		if err != nil {
			return nil, containerDefinition{}, fmt.Errorf("no stored definition: %w", err)
		}
		plain, err := secret.Decrypt(s.cfg.AppKey, enc)
		if err != nil {
			return nil, containerDefinition{}, fmt.Errorf("the definition does not decrypt (wrong APP_KEY?): %w", err)
		}
		var def containerDefinition
		if err := json.Unmarshal(plain, &def); err != nil {
			return nil, containerDefinition{}, fmt.Errorf("the definition is corrupt: %w", err)
		}
		return plain, def, nil
	}
	// rebuildOne reports whether name's stored definition is present,
	// decryptable and parseable, whatever dryRun says; dryRun only gates the
	// write.
	rebuildOne := func(name, repoID string) bool {
		plain, def, err := storedDef(name, repoID)
		if err != nil {
			log.Printf("api: discover: skipping %q, which cannot be recreated: %v", name, err) //nolint:gosec // G706: %q-quoted
			return false
		}
		if !dryRun {
			// A row this call is about to create takes its home from the pass only
			// while the domain lock was taken; otherwise it starts open, the same
			// as an existing row the lock refused, so discoverHome below reports it
			// in LeftOpen instead of the insert setting it straight through.
			write := store.HomeWrite{Choice: store.RepoOpen}
			if locked {
				write = s.discoverWrite("containers", name, repoID, readErr)
			}
			if _, uErr := s.store.UpsertTarget(store.Target{
				ContainerName: name,
				AppdataPaths:  def.AppdataPaths,
				Definition:    string(plain),
				Repo:          write.Repo,
				RepoChosen:    write.Choice,
			}); uErr != nil {
				log.Printf("api: discover: could not upsert target %q: %v", name, uErr) //nolint:gosec // G706: %q-quoted
				return false
			}
			left, hErr := s.discoverHome(ctx, store.ItemRef{Domain: "containers", Key: name}, repoID, readErr, locked)
			if hErr != nil {
				log.Printf("api: discover: could not restore the repository of %q: %v", name, hErr) //nolint:gosec // G706: %q-quoted
			}
			if left {
				res.LeftOpen = append(res.LeftOpen, name)
			}
		}
		return true
	}
	// A name that a stored definition records as a former name is left to
	// foldFormerNames, which runs once every other name has been rebuilt.
	claims := recordedFormerNames(names, func(name, repoID string) []definitionAlias {
		_, def, _ := storedDef(name, repoID) // rebuildOne logs why one does not read
		return def.Aliases
	})
	for name, repoID := range names {
		if len(claims[name]) > 0 {
			continue
		}
		if rebuildOne(name, repoID) {
			res.Found++
		}
	}
	// The fold runs once the loop above has rebuilt what it could, and never on
	// a dry run, which writes nothing.
	if !dryRun {
		logUnrecordedFormerNames("containers", formerNames, claims)
		res.Found += s.foldFormerNames(ctx, settings, "container", names, claims, rebuildOne,
			func(old, targetID string, record definitionAlias) error {
				_, err := s.store.AddAliasAt("container", old, targetID, record.LinkedAt)
				return err
			})
	}
	if !dryRun && res.Found > 0 {
		if err := s.pauseAfterDiscover(ctx, "containers", &res); err != nil {
			readErr = errors.Join(readErr, err)
		}
	}
	// The result first, the read failure last: a caller that branches on err
	// still sees it, and one that shows a partial rebuild has something to show.
	return res, readErr
}

// foldFormerNames links each name a rebuilt name's stored definition records
// as a former name to the first claimant with a row, with that claimant's
// record, and returns how many of those names it rebuilt as rows of their own:
// a name no claimant could be rebuilt for, and a name another machine has been
// backed up under since the link, which keeps its own row beside the link.
// A copy rule on a name it links moves to the claimant, as a takeover moves
// it, unless a container or VM installed under that name follows it, or the
// installed ones cannot be listed. Names and claimants are taken in sorted
// order, so a repository always rebuilds the same way.
func (s *Service) foldFormerNames(ctx context.Context, settings store.Settings, domain string, names map[string]string, claims map[string][]formerNameClaim,
	rebuild func(name, repoID string) bool, link func(old, targetID string, record definitionAlias) error) int {
	settingsDomain, _, _ := aliasDomain(domain)
	if len(claims) == 0 {
		return 0
	}
	var installed map[string]bool
	var listErr error
	if domain == "vm" {
		installed, listErr = s.definedVMs(ctx)
	} else {
		installed, listErr = s.installedContainers(ctx)
	}
	if listErr != nil {
		log.Printf("api: discover %s: the installed %s could not be listed, so a copy rule on a former name stays there: %v", settingsDomain, settingsDomain, listErr)
	}
	rebuilt := 0
	for _, old := range slices.Sorted(maps.Keys(claims)) {
		byOwner := claims[old]
		slices.SortFunc(byOwner, func(a, b formerNameClaim) int { return strings.Compare(a.owner, b.owner) })
		var owner, targetID string
		var record definitionAlias
		for _, c := range byOwner {
			if id, err := s.entryIDByName(domain, c.owner); err == nil {
				owner, targetID, record = c.owner, id, c.record
				break
			}
		}
		if owner == "" {
			curs := make([]string, 0, len(byOwner))
			for _, c := range byOwner {
				curs = append(curs, c.owner)
			}
			log.Printf("api: discover %s: none of %q, which record %q as a former name, could be rebuilt, so it is rebuilt on its own", settingsDomain, curs, old) //nolint:gosec // G706: names %q-quoted, the domain a fixed literal
			if rebuild(old, names[old]) {
				rebuilt++
			}
			continue
		}
		linked := false
		if a, err := s.store.AliasByOldName(domain, old); err == nil {
			if a.TargetID != targetID {
				log.Printf("api: discover %s: %q is already linked to another entry; leaving it alone", settingsDomain, old) //nolint:gosec // G706: name %q-quoted, the domain a fixed literal
				continue
			}
		} else {
			if _, err := s.entryIDByName(domain, old); err == nil {
				log.Printf("api: discover %s: %q already has its own row, so it is not linked to %q", settingsDomain, old, owner) //nolint:gosec // G706: names %q-quoted, the domain a fixed literal
				continue
			}
			if !validFormerName(domain, old) {
				log.Printf("api: discover %s: %q is not linked to %q: it cannot be written as a backup tag", settingsDomain, old, owner) //nolint:gosec // G706: names %q-quoted, the domain a fixed literal
				continue
			}
			if err := link(old, targetID, record); err != nil {
				log.Printf("api: discover %s: could not link %q to %q: %v", settingsDomain, old, owner, err) //nolint:gosec // G706: names %q-quoted, the domain a fixed literal
				continue
			}
			linked = true
		}
		if s.formerNameReusedSince(ctx, settings, domain, old, names[old], record.LinkedAt) && rebuild(old, names[old]) {
			rebuilt++
		}
		// After the rebuild, so a reused name that got its row back keeps its rule.
		if linked && listErr == nil {
			s.placementMu.Lock()
			err := s.store.CarryFormerNameRule(domain, old, installed)
			s.placementMu.Unlock()
			if err != nil {
				log.Printf("api: discover %s: the copy rule of %q could not move to %q, so it stays on the former name: %v", settingsDomain, old, owner, err) //nolint:gosec // G706: names %q-quoted, the domain a fixed literal
			}
		}
	}
	return rebuilt
}

// formerNameReusedSince reports whether old's backups, in the repository
// Discover found them in, hold one that a link at linkedAt does not claim:
// another machine has been backed up under the name since. A listing that
// fails counts too, so such a machine's backups never lose their entry.
func (s *Service) formerNameReusedSince(ctx context.Context, settings store.Settings, domain, old, repoID string, linkedAt int64) bool {
	settingsDomain, prefix, _ := aliasDomain(domain)
	claim := newAliasClaim(prefix, store.Alias{OldName: old, LinkedAt: linkedAt})
	repo, err := s.itemRepoPath(repoID, func() (string, error) { return s.repoFor(settings, settingsDomain, "local") })
	var snaps []restic.Snapshot
	if err == nil {
		snaps, err = s.snapshotsForTag(ctx, repo, s.primaryModeFor(settings, settingsDomain, repo), claim.tag)
	}
	if err != nil {
		log.Printf("api: discover %s: the backups of %q could not be listed, so it is rebuilt as its own entry beside the link: %v", settingsDomain, old, scrubError(err)) //nolint:gosec // G706: name %q-quoted, the domain a fixed literal and the error scrubbed
		return true
	}
	return !claim.claimsEvery(snaps)
}

// validFormerName reports whether name may become an alias in domain. Every
// later backup writes it into a formerly: tag, and restic splits a tag at a
// comma.
func validFormerName(domain, name string) bool {
	if domain == "vm" {
		return validVMName(name) && !strings.Contains(name, ",")
	}
	return validResourceName(name)
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

// DiscoverVMs rebuilds the VM target list from backup storage, the VM
// counterpart of Discover, after a fresh install or database loss, so a VM
// that was deleted from the host (or whose target is gone) becomes
// restorable again. It lists the vms repo's snapshots (tagged vm:<name>),
// reads and decrypts each VM's mirrored definition, and upserts a target.
// VMs whose definition is missing or undecryptable are skipped. The result
// counts the VMs discovered. dryRun makes it read-only: it opens the repo
// and decrypts the definitions to prove readability and the APP_KEY, and
// returns the same count, but writes no targets. The Recovery readability
// probe uses this so it never resurrects orphan VM entries (#44). A name
// that another VM's stored definition records as a former name is linked to
// that VM, and rebuilt beside the link only when a later VM has been backed
// up under it; see foldFormerNames.
func (s *Service) DiscoverVMs(ctx context.Context, dryRun bool) (DiscoverResult, error) {
	settings, err := s.store.GetSettings()
	if err != nil {
		return DiscoverResult{}, fmt.Errorf("read settings: %w", err)
	}
	// Every repository this domain writes to (#204), with the one each name was
	// found in; a not-yet-created repo yields nothing. A read failure comes back
	// as readErr together with whatever the named repositories yielded, so an
	// install whose domain repository is unreadable is still rebuilt as far as
	// it can be; the Recovery wizard classifies on that error.
	names, formerNames, skipped, directRows, readErr := s.discoverNamesAcrossRepos(ctx, settings, "vms", "vm:")
	findings, fErr := s.directFindings("vms", directRows)
	if fErr != nil {
		log.Printf("api: discover vms: could not match direct repositories to targets: %v", fErr)
	}

	dir, err := s.vmDefsDir(settings)
	if err != nil {
		return DiscoverResult{Direct: findings}, err
	}
	legacyDir, err := s.legacyVMDefsDir(settings)
	if err != nil {
		return DiscoverResult{Direct: findings}, err
	}
	res := DiscoverResult{Skipped: skipped, LeftOpen: []string{}, Direct: findings}
	unlock, locked := s.discoverLock("vms", dryRun)
	defer unlock()
	// storedDef reads name's definition file, from beside its own snapshots
	// first as in Discover.
	storedDef := func(name, repoID string) ([]byte, vmDefinition, error) {
		fn, err := defFileName(name)
		if err != nil {
			return nil, vmDefinition{}, err
		}
		lookIn := dir
		if own := s.itemDefsDir(repoID, true); own != "" {
			lookIn = own
		}
		enc, err := readStoredDef(lookIn, legacyDir, fn)
		if err != nil && lookIn != dir {
			enc, err = readStoredDef(dir, legacyDir, fn)
		}
		if err != nil {
			return nil, vmDefinition{}, fmt.Errorf("no stored definition: %w", err)
		}
		plain, err := secret.Decrypt(s.cfg.AppKey, enc)
		if err != nil {
			return nil, vmDefinition{}, fmt.Errorf("the definition does not decrypt (wrong APP_KEY?): %w", err)
		}
		var def vmDefinition
		if err := json.Unmarshal(plain, &def); err != nil {
			return nil, vmDefinition{}, fmt.Errorf("the definition is corrupt: %w", err)
		}
		return plain, def, nil
	}
	rebuildOne := func(name, repoID string) bool {
		plain, def, err := storedDef(name, repoID)
		if err != nil {
			log.Printf("api: discover vms: skipping %q, which cannot be recreated: %v", name, err) //nolint:gosec // G706: %q-quoted
			return false
		}
		method := def.Method
		if method == "" {
			method = "graceful"
		}
		if !dryRun {
			// See Discover: a row this call is about to create starts open unless
			// the domain lock was taken, so discoverHome reports it in LeftOpen
			// instead of the insert setting its home straight through.
			write := store.HomeWrite{Choice: store.RepoOpen}
			if locked {
				write = s.discoverWrite("vms", name, repoID, readErr)
			}
			if _, uErr := s.store.UpsertVMTarget(store.VMTarget{
				Name:       name,
				Method:     method,
				Definition: string(plain),
				Repo:       write.Repo,
				RepoChosen: write.Choice,
			}); uErr != nil {
				log.Printf("api: discover vms: could not upsert target %q: %v", name, uErr) //nolint:gosec // G706: %q-quoted
				return false
			}
			left, hErr := s.discoverHome(ctx, store.ItemRef{Domain: "vms", Key: name}, repoID, readErr, locked)
			if hErr != nil {
				log.Printf("api: discover vms: could not restore the repository of %q: %v", name, hErr) //nolint:gosec // G706: %q-quoted
			}
			if left {
				res.LeftOpen = append(res.LeftOpen, name)
			}
		}
		return true
	}
	// As in Discover, a recorded former name waits for the fold.
	claims := recordedFormerNames(names, func(name, repoID string) []definitionAlias {
		_, def, _ := storedDef(name, repoID) // rebuildOne logs why one does not read
		return def.Aliases
	})
	for name, repoID := range names {
		if len(claims[name]) > 0 {
			continue
		}
		if rebuildOne(name, repoID) {
			res.Found++
		}
	}
	// As in Discover, the fold runs after the loop and never on a dry run.
	if !dryRun {
		logUnrecordedFormerNames("vms", formerNames, claims)
		res.Found += s.foldFormerNames(ctx, settings, "vm", names, claims, rebuildOne,
			func(old, targetID string, record definitionAlias) error {
				if record.PrevDefinition == "" {
					log.Printf("api: discover vms: the link record of %q carries no definition, so it is linked without one and cannot be unlinked", old) //nolint:gosec // G706: %q-quoted
				}
				_, err := s.store.AddVMAliasAt(old, targetID, record.LinkedAt, record.PrevDefinition)
				return err
			})
	}
	if !dryRun && res.Found > 0 {
		if err := s.pauseAfterDiscover(ctx, "vms", &res); err != nil {
			readErr = errors.Join(readErr, err)
		}
	}
	// The result first, the read failure last: a caller that branches on err
	// still sees it, and one that shows a partial rebuild has something to show.
	return res, readErr
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
	appdataPaths []string            // restored per-path back to origin, in snapshot-path form (nil = recreate-only)
	restoreDirs  []backup.RestoreDir // cross-pool remap: Subtree->Target; empty = in-place via appdataPaths
	// skippedPaths carries stored paths that had no mapping in the chosen
	// snapshot. They are skipped one by one (scrubbed log plus a note in the
	// orchestrator's run record), never a global abort.
	skippedPaths []string
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
// a repo that is not in Settings (e.g. another BombVault instance's repo) can
// build its own ref and reuse the same preparation logic.
type repoRef struct {
	repo string
	mode restic.Mode
}

// prepareRestore resolves the settings-configured containers repo (local or
// off-site) and delegates to prepareRestoreIn. The request guards run
// first, before the settings and repo resolution, so an unconfirmed or
// malformed request fails with its own error (the sentinel, the name error,
// the snapshot-id error) rather than a resolution error; prepareRestoreIn
// re-validates them for non-settings callers.
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
	repo, err := s.containerRepoForName(settings, name, source)
	if err != nil {
		return containerRestorePlan{}, err
	}
	return s.prepareRestoreIn(ctx, repoRef{repo: repo, mode: s.repoModeFor(settings, "containers", source, repo)}, name, snapshotID, confirm)
}

// prepareRestoreIn performs all of a container restore's validation and
// resolution synchronously against an explicit repository (confirmation,
// name and snapshot-id guards, snapshot ownership, path containment and the
// recreate-recipe lookup), so a bad request fails immediately with a clear
// error before anything long-running or destructive starts. The returned
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
	// Same-instance restore: no destBase (in-place) and no overwrite prompt,
	// since the cross-pool remap is foreign-only. ref is this instance's own
	// repo, so the local alias history applies.
	return s.prepareRestoreForTarget(ctx, ref, name, snapshotID, tg, s.containerIdentity(name), "", false)
}

// prepareRestoreForTarget builds a container restore plan for an already
// resolved target tg against an explicit repo ref, without reading or writing
// the store. prepareRestoreIn passes the stored target; the foreign restore
// passes a target built from the decrypted foreign definition, so snapshot
// ownership and appdata containment are validated before that foreign recipe
// is persisted locally (prepareForeignRestore adopts it only once this returns
// a plan, never on a validation failure, which would clobber a same-named
// local target). The caller runs the confirm / name / explicit-snapshot-id-shape
// guards first.
//
// id is the identity the ownership check accepts. The caller decides it
// because it depends on where ref points: prepareRestoreIn passes the local
// containerIdentity, prepareForeignRestore only the literal container:<name>
// tag, since this instance's rename history says nothing about a foreign
// repository.
func (s *Service) prepareRestoreForTarget(ctx context.Context, ref repoRef, name, snapshotID string, tg store.Target, id entryIdentity, destBase string, overwrite bool) (containerRestorePlan, error) {
	explicitID := snapshotID != "latest" && snapshotID != ""

	// "latest" (or empty) resolves to the container's newest snapshot, as the
	// bulk "restore selected" action needs; restic returns snapshots oldest
	// first. A definition-only backup (stateless container with no restic
	// snapshot) is recreated from the stored definition instead. An explicit id
	// must be one id owns, the same access check the file and to-path restores
	// make, listed against the caller's repo ref rather than the settings repo.
	recreateOnly := false
	snaps, snapErr := s.snapshotsOwnedBy(ctx, ref.repo, ref.mode, id)
	if snapErr != nil {
		return containerRestorePlan{}, snapErr
	}
	if explicitID {
		if !snapshotBelongs(snaps, snapshotID) {
			return containerRestorePlan{}, notInListing{snapshotID, "container"}
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
	var appdataForRestore []string
	var restoreDirs []backup.RestoreDir
	var bindRemap map[string]string
	var planSkipped []string
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
		// Map the stored selection onto this snapshot's recorded Paths
		// (longest-prefix) before anything destructive. The stored list is the
		// selection as of the latest backup, and the chosen snapshot may be an
		// older one whose Paths have a different shape. Replaying the stored list
		// verbatim would fail the restore mid-loop at the adapter, after the caller
		// had already stopped and removed the container. All failure resolution
		// happens here, in the synchronous prepare phase: the orchestrator only
		// receives a list it has checked.
		chosen := chosenSnapshot(snaps, snapshotID)
		mapped, skipped, narrowed := mapRestorePaths(tg.AppdataPaths, chosen.Paths)
		// A pass-2 result is a stored path for which only an ancestor was
		// recorded. That proves it lies under a backed-up root, not that it is in
		// the snapshot: the branch may have been carved out by an --exclude when
		// the backup ran, or the folder may not have existed yet. Either way the
		// selector would miss, and the miss would land after the container was
		// stopped and removed, which is what this mapping exists to prevent. So
		// they are checked against the snapshot's real tree here, before anything
		// destructive, and a path not in it becomes an ordinary per-path skip,
		// never a widening back to the ancestor, which could lose data.
		if len(narrowed) > 0 {
			present, lsErr := s.pathsPresentInSnapshot(ctx, ref.repo, snapshotID, ref.mode, narrowed)
			if lsErr != nil {
				return containerRestorePlan{}, fmt.Errorf("read snapshot contents: %w", lsErr)
			}
			kept := mapped[:0]
			for _, q := range mapped {
				if present[q] {
					kept = append(kept, q)
					continue
				}
				skipped = append(skipped, q)
			}
			mapped = kept
		}
		if len(tg.AppdataPaths) > 0 && len(mapped) == 0 {
			return containerRestorePlan{}, errors.New("nothing to restore for this item from this snapshot")
		}
		if len(skipped) > 0 {
			// Per-path skip: an orphan stored path is never fatal. The reason is
			// scrubbed (paths become [path] first) and the skips reach the run record
			// via RestoreDeps.SkippedPaths below.
			log.Printf("api: restore: %d stored path(s) absent from snapshot %s, skipping: %s", len(skipped), snapshotID, scrubSecrets(strings.Join(skipped, ", "))) //nolint:gosec // G706: paths scrubbed to [path] before the formatter sees them
		}
		// The same containment check as the stored list above, over the mapped
		// selectors. Pass 1 takes them from the snapshot's recorded Paths (repo
		// metadata), a different and lower-trust source than the DB row the loop
		// above validated. Within cleans both sides, so a raw uncleaned metadata
		// string (or a crafted one with "..") is judged on its cleaned form. Pass 2
		// maps a stored path to itself and cannot produce a selector above the
		// stored list, so this guards pass 1 and crafted metadata.
		for _, q := range mapped {
			if !paths.Within(s.cfg.HostMountRoot, q) {
				log.Printf("api: restore: mapped path %q escapes mount root", q) //nolint:gosec // G706: %q-quoted
				return containerRestorePlan{}, errors.New("a mapped restore path is outside the host mount, so refusing to restore")
			}
		}
		appdataForRestore = mapped
		planSkipped = skipped
		// Cross-instance, cross-pool remap (destBase set: foreign restore,
		// #123/#125). A foreign recipe carries the source host's absolute appdata
		// paths; if this host lacks that pool, the in-place write would land in an
		// unmounted dir under the host mount (the array or RAM rootfs), writing
		// appdata to the wrong place or bricking the host (#122). Instead restore
		// every appdata path's contents into its container-relative place under
		// destBase on this host (containerAppdataRemap; a multi-bind container
		// keeps the folder its binds share), guard those destination dirs, and
		// point the recreated binds and template there. A standard container whose
		// appdata already lives under destBase remaps to the same path. destBase ==
		// "" is the same-instance in-place restore.
		if destBase != "" {
			if !paths.Within(s.cfg.HostMountRoot, destBase) {
				return containerRestorePlan{}, errors.New("restore destination is outside the host mount")
			}
			restoreDirs, bindRemap = s.containerAppdataRemap(destBase, appdataForRestore)
			destDirs := make([]string, len(restoreDirs))
			for i, d := range restoreDirs {
				destDirs[i] = d.Target
			}
			// Host-brick guard on the destination dirs (not the source), before
			// executeRestore's destructive Stop and Remove: prove each target is on a
			// real mounted pool so a cross-pool restore can never fill the RAM rootfs.
			if err := s.guardContainerRestoreDestination(ctx, ref, snapshotID, destDirs); err != nil {
				return containerRestorePlan{}, err
			}
			// Overwrite guard: a target that is not this container's own source path
			// and already holds data is likely a different container's appdata, so
			// refuse unless the caller explicitly confirmed the overwrite.
			if !overwrite {
				for _, d := range restoreDirs {
					if d.Target != d.Subtree && s.dirNonEmptyFn()(d.Target) {
						return containerRestorePlan{}, destinationRefusal("restore destination %q already contains data and may belong to a different container; confirm overwrite to proceed", s.toHostPath(d.Target))
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
		// Fallback: a target with no stored definition; try live inspect.
		liveIn, liveErr := s.docker.Inspect(ctx, name)
		if liveErr != nil {
			return containerRestorePlan{}, errors.New("no stored definition for this container: run a backup once after upgrading, then restore is possible even after deletion")
		}
		in = liveIn
		xml, _, _ = template.Read(s.cfg.FlashTemplatesDir, name)
	}

	// Cross-pool remap: point the recreated container's binds and flashed
	// template at the appdata's new location. Only appdata binds are rewritten
	// (exact host-path match); docker.sock, /etc/localtime, /dev/dri and every
	// non-appdata bind stay verbatim, since they carry no backed-up data (#125).
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
		skippedPaths: planSkipped,
		inspect:      in,
		templateXML:  xml,
	}, nil
}

// executeRestore drives the long-running (destructive) part of a container
// restore described by an already-validated plan, publishing "container:<name>"
// progress. The orchestrator records the run (kindRestore) itself.
func (s *Service) executeRestore(ctx context.Context, name string, plan containerRestorePlan, leaveStopped bool) error {
	// Hold the domain repo lock for the whole restic and docker phase,
	// including the destination pre-create below. The scheduler calls
	// Backup/BackupVM directly and bypasses the batchActive single-flight
	// guard, so the domain lock is the one layer scheduled jobs respect;
	// without it a detached multi-hour restore could overlap a scheduled backup
	// of the same domain in both directions.
	unlock := s.lockDomainFor("containers", "restore")
	defer unlock()
	// Pre-create every remapped destination readable (0o755), as the file-set
	// and to-path restores do (see EnsureDirReadable). restic's restorer
	// creates a fresh subtree target at 0o700 and never revisits that root's
	// own metadata (only its restored contents get the snapshot's ownership and
	// mode), which leaves the cross-pool remap's new destination dirs
	// root:root/0700, unreadable to whatever UID the container runs as.
	// Pre-created, restic's MkdirAll finds the dir present and leaves the mode
	// alone. In-place restores (empty RestoreDirs, or a Target that already
	// existed) are a cheap no-op: EnsureDirReadable only heals mode, never
	// ownership; the numeric owner:group is restored after a successful restore
	// by healRestoreDirOwnership (#125).
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
		AppdataPaths:      plan.appdataPaths, // restored per-path back to origin, snapshot-path form (nil = recreate-only)
		RestoreDirs:       plan.restoreDirs,  // cross-pool remap (foreign restore); empty = in-place
		SkippedPaths:      plan.skippedPaths, // unmapped stored paths → run-record note, never an abort
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

// healRestoreDirOwnership restores, best-effort, each remapped restore
// directory's own root to the owner and mode it had in the snapshot.
// restic's restorer creates a fresh subtree target itself but never
// re-applies that root's own metadata (only its restored contents get the
// snapshot's ownership and mode; the EnsureDirReadable call above fixes
// readability only). Read-then-heal, never fatal: the restore already
// succeeded when this runs, so a failure here (repo busy, a stale snapshot
// layout, an unexpected filesystem) must not turn a completed restore into
// a reported failure; it is logged and the next dir is tried.
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
// (StartRestore, StartRestoreVM, StartRestoreFiles, StartRestoreToPath,
// StartRestoreStack). Aborting a restore mid-flight is destructive, because
// the container has already been removed and the appdata is partly
// written, so unlike the configurable backup cap (backupHardCap, default
// 48h) this one is generous: it exists only so a truly wedged restic can't
// hold the single-flight guard and the domain lock forever, never to bound
// a legitimate huge restore.
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

// StartRestore launches an in-place container restore in a background
// goroutine and returns immediately, mirroring StartBackup. Long restores
// need this: the work runs on the server, detached from the request, so a
// multi-hour restore can't be killed by the browser or a proxy dropping the
// idle HTTP connection, which would cancel the request context and abort
// restic mid-restore. All validation runs synchronously first, so a bad
// request still fails immediately with a clear error and no goroutine is
// started.
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

// LatestContainerBackupTimes returns, per container name, the unix time of the
// newest backup that name owns. A card's date is read from here rather than
// from the run history, so it agrees with the list of backups under it: an
// entry rebuilt by Discover has no run at all (#44), and a run stays with the
// entry while a backup stays with the name it was written under.
func (s *Service) LatestContainerBackupTimes(ctx context.Context) (map[string]int64, error) {
	// When the targets cannot be read nothing folds, and each old name keeps
	// its own date.
	idToName := map[string]string{}
	if targets, tErr := s.store.ListTargets(); tErr != nil {
		log.Printf("api: last-backup times: listing targets for alias fold: %v; leaving every tag as its own identity", tErr)
	} else {
		for _, t := range targets {
			idToName[t.ID] = t.ContainerName
		}
	}
	return s.latestBackupTimes(ctx, "containers", "container", idToName)
}

// LatestFileSetBackupTimes is LatestContainerBackupTimes for the folder sets.
// A set's name is fixed once it has backups, so no former name folds into it;
// backups left under a name no set carries any more come back as their own set
// through Discover.
func (s *Service) LatestFileSetBackupTimes(ctx context.Context) (map[string]int64, error) {
	return s.latestBackupTimes(ctx, "files", "fileset", nil)
}

// LatestVMBackupTimes is LatestContainerBackupTimes for the VMs domain.
func (s *Service) LatestVMBackupTimes(ctx context.Context) (map[string]int64, error) {
	idToName := map[string]string{}
	if targets, tErr := s.store.ListVMTargets(); tErr != nil {
		log.Printf("api: last-backup times: listing VM targets for alias fold: %v; leaving every tag as its own identity", tErr)
	} else {
		for _, t := range targets {
			idToName[t.ID] = t.Name
		}
	}
	return s.latestBackupTimes(ctx, "vms", "vm", idToName)
}

// latestBackupTimes reads one snapshot listing per repository of domain and
// keeps, per name, the newest time under that name's tag. idToName carries the
// current name of every entry, for the alias fold.
func (s *Service) latestBackupTimes(ctx context.Context, domain, aliasDomain string, idToName map[string]string) (map[string]int64, error) {
	settings, err := s.store.GetSettings()
	if err != nil {
		return nil, fmt.Errorf("read settings: %w", err)
	}
	// Every repository this domain's items write to, not just the domain's own
	// (#204). A container pointed at a named repository keeps its snapshots
	// there, and reading only the domain repo would tell the dashboard it has
	// never been backed up.
	// The skip list is ignored here: this is a read-only overview, a repository
	// it cannot open has no times to contribute, and the loop below logs one it
	// cannot read. Every caller that changes something reports its skips; the
	// two that do not are this one and repoSharedWithAnotherDomain, which asks
	// a yes/no question and answers "shared" when it cannot tell.
	repos, _, err := s.domainReposInUse(settings, domain)
	if err != nil {
		return nil, err
	}
	prefix := aliasDomain + ":"
	out := make(map[string]int64)
	for _, repo := range repos {
		if localRepoMissing(repo.Loc) {
			continue
		}
		// Per repository, like every other reader: with the shared mode, a
		// container on a remote named repository with its own credentials could
		// not be listed, and the dashboard would say it had never been backed up.
		all, lErr := s.listSnapshots(ctx, repo.Loc, s.primaryModeFor(settings, domain, repo.Loc))
		if lErr != nil {
			// One unreachable repository must not blank the whole column: the
			// other entries' times are still true. The item whose repo this
			// is shows as never backed up, which is all that can be said while
			// its repository cannot be read.
			log.Printf("api: last-backup times: repository unreadable, skipping: %v", lErr)
			continue
		}
		for _, snap := range all {
			ts, perr := time.Parse(time.RFC3339Nano, snap.Time)
			if perr != nil {
				continue
			}
			unix := ts.Unix()
			for _, tag := range snap.Tags {
				name, ok := strings.CutPrefix(tag, prefix)
				if !ok || name == "" {
					continue
				}
				// A rename leaves the old tag on snapshots already written. One
				// from before the link is the renamed entry's and counts for its
				// current name, whoever holds the old name today; a later one
				// stays under the old name.
				if a, aErr := s.store.AliasByOldName(aliasDomain, name); aErr == nil {
					if cur := idToName[a.TargetID]; cur != "" && newAliasClaim(prefix, a).claims(snap) {
						name = cur
					}
				}
				if unix > out[name] {
					out[name] = unix
				}
			}
		}
	}
	return out, nil
}

// domainReposForOp is domainRepoSource for an operation that has to reach
// all of a domain's data rather than one repository: check, unlock, prune,
// and finding the repository a given snapshot lives in. With named
// repositories (#204), resolving the domain's own repository alone would
// give a green integrity check over a repository the item's data is not in,
// an unlock that leaves the stuck lock in place, a prune that never
// reclaims the space retention freed, and a delete button that reports "no
// matching ID" for a snapshot the list beside it shows.
//
// An off-site source still answers with exactly one repository: an
// off-site copy is configured per domain, and a per-item override says
// nothing about it.
//
// The third return value lists the repositories that belong to this domain
// and could not be included. A caller must report them (skippedError);
// covering three repositories out of four and calling it success is the
// failure these callers exist to avoid.
func (s *Service) domainReposForOp(domain, source string) (store.Settings, []domainRepoRef, []repoSkip, error) {
	settings, err := s.store.GetSettings()
	if err != nil {
		return store.Settings{}, nil, nil, fmt.Errorf("read settings: %w", err)
	}
	if isOffsiteSource(source) {
		repo, rErr := s.repoFor(settings, domain, source)
		if rErr != nil {
			return settings, nil, nil, rErr
		}
		// An off-site destination is neither the domain's own repository nor a
		// named one; the zero Own/Named is exactly right.
		return settings, []domainRepoRef{{Loc: repo}}, nil, nil
	}
	repos, skipped, err := s.domainReposInUse(settings, domain)
	if err != nil {
		return settings, nil, nil, err
	}
	return settings, repos, skipped, nil
}

// domainReposInUse returns every repository a domain's items write to: the
// domain's own, plus each distinct named repository (#204) one of its
// items points at. The order is deterministic (the domain repo first, then
// the named ones in the order Settings lists them), so a caller that stops
// at the first hit prefers the domain repo.
//
// A named repository that is switched off or no longer resolves is skipped
// rather than failing the call, but it is reported: every skip comes back
// in the second return value, and an operation that covers less of a
// domain than the domain has must say so rather than report success over
// the remainder. Callers such as verify, prune, unlock, the restorability
// drill, snapshot delete, discovery and off-site replication never reach
// itemRepoPath, so this is where a broken override gets noticed. A
// switched-off repository is the ordinary case: the interface invites it
// when a share dies.
func (s *Service) domainReposInUse(settings store.Settings, domain string) ([]domainRepoRef, []repoSkip, error) {
	own, err := s.repoFor(settings, domain, "local")
	if err != nil {
		return nil, nil, err
	}
	out := []domainRepoRef{ownRef(own)}
	var skipped []repoSkip
	ids := map[string]bool{}
	// A store read that fails is itself a skip: the answer is no longer "the
	// domain has one repository", it is "I could not find out", and those two
	// must not look the same to the caller.
	switch domain {
	case "containers":
		tgs, tErr := s.store.ListTargets()
		if tErr != nil {
			return out, []repoSkip{{Name: "this domain's items", Reason: "their list could not be read", Unreachable: true}}, nil //nolint:nilerr // reported as a skip instead
		}
		for _, t := range tgs {
			if id := strings.TrimSpace(t.Repo); id != "" {
				ids[id] = true
			}
		}
	case "vms":
		vms, vErr := s.store.ListVMTargets()
		if vErr != nil {
			return out, []repoSkip{{Name: "this domain's items", Reason: "their list could not be read", Unreachable: true}}, nil //nolint:nilerr // see above
		}
		for _, v := range vms {
			if id := strings.TrimSpace(v.Repo); id != "" {
				ids[id] = true
			}
		}
	case "files":
		sets, fErr := s.store.ListFileSets()
		if fErr != nil {
			return out, []repoSkip{{Name: "this domain's items", Reason: "their list could not be read", Unreachable: true}}, nil //nolint:nilerr // see above
		}
		for _, f := range sets {
			if id := strings.TrimSpace(f.Repo); id != "" {
				ids[id] = true
			}
		}
	}
	named, err := s.store.ListNamedRepos()
	if err != nil {
		return out, []repoSkip{{Name: "the named repositories", Reason: "their list could not be read", Unreachable: true}}, nil //nolint:nilerr // see above
	}
	for _, n := range named {
		if !ids[n.ID] {
			continue // nothing in this domain points at it
		}
		if !n.Enabled {
			skipped = append(skipped, repoSkip{Name: scrubSafeName(n.Name), Reason: "switched off"})
			continue
		}
		loc, rErr := s.resolveRepo(n.Repo)
		if rErr != nil {
			skipped = append(skipped, repoSkip{Name: scrubSafeName(n.Name), Reason: "its location does not resolve", Unreachable: true})
			continue
		}
		if sameRepoLocation(loc, own) {
			continue // the same place as the domain's own, already in the list
		}
		out = append(out, namedRef(loc, n))
	}
	return out, skipped, nil
}

// domainTagPrefix is the identity-tag prefix every snapshot of a domain carries
// ("container:<name>", "vm:<name>", "fileset:<name>"). Empty for flash and
// config, which have no per-item repositories and therefore cannot share one.
func domainTagPrefix(domain string) string {
	switch domain {
	case "containers":
		return "container:"
	case "vms":
		return "vm:"
	case "files":
		return "fileset:"
	}
	return ""
}

// domainRepoRef is one repository of a domain together with what it is.
// Deciding that by slice position ("index 0 is the domain's own
// repository") breaks as soon as a caller reshapes the slice: a filter
// that drops a never-created repository from the head would promote a
// named repository into the place that means "the domain's own", and the
// off-site copy would stop narrowing to this domain's snapshots. With the
// identity carried along, reshaping a list cannot change what an element
// means.
type domainRepoRef struct {
	// Loc is the resolved location, the string restic is handed.
	Loc string
	// Own marks the domain's own repository (Settings.<Domain>Path).
	Own bool
	// Named is the named repository's row (#204). Zero value when Own.
	Named store.OffsiteTarget
	// CountOnly marks a source no item is copied from. It is listed, so a
	// target's keep-policy knows its items still exist here, and never copied.
	CountOnly bool
}

// ownRef and namedRef are the two ways a reference is created, so nobody has to
// remember which fields go together.
func ownRef(loc string) domainRepoRef { return domainRepoRef{Loc: loc, Own: true} }
func namedRef(loc string, row store.OffsiteTarget) domainRepoRef {
	return domainRepoRef{Loc: loc, Named: row}
}

// refFor builds a reference for a location whose identity the caller does
// not already know: the post-backup replication hook, which gets whatever
// repository the item it just backed up uses.
func (s *Service) refFor(settings store.Settings, domain, loc string) domainRepoRef {
	if named, ok := s.namedRepoForLocation(loc); ok {
		return namedRef(loc, named)
	}
	// A domain repository that does not resolve leaves the location unidentified,
	// and an unidentified direct repository loses its tag and its exclusion from
	// the copies, so the failure is said rather than swallowed by the comparison.
	if own, err := s.repoFor(settings, domain, "local"); err != nil {
		log.Printf("api: %s: the domain repository does not resolve, so %s could not be identified: %v", domain, shortRepoName(loc), err) //nolint:gosec // G706: the domain is a fixed literal and the location is shortened
	} else if sameRepoLocation(own, loc) {
		return ownRef(loc)
	}
	// Neither the domain's own nor a known named row: an off-site destination
	// or a location resolved from a source string. Not Own and no named row,
	// which is what the consumers need to know.
	return domainRepoRef{Loc: loc}
}

// repoModeFor builds the restic mode for one repository, choosing the
// builder by source. Every operation that has already resolved which
// repository it is about to open uses it: the maintenance ones (verify,
// unlock, prune, drill, snapshot delete) and the readers (snapshot lists,
// file listings, diffs, restore plans) alike.
//
// An off-site location is described by its own target row (its credential
// set, storage class and caps), which offsiteModeForTarget reads.
// primaryModeFor describes a domain's primary repository; applied to an
// off-site copy it would authenticate somebody else's bucket with the
// primary's keys and impose the primary's bandwidth caps on a link they
// were never set for. Readers of an item's named repository likewise need
// that row's credentials, storage class and caps, or an archive the writer
// could write could not be opened again from the interface.
func (s *Service) repoModeFor(settings store.Settings, domain, source, repo string) restic.Mode {
	if isOffsiteSource(source) {
		if t, err := s.offsiteTargetForSource(settings, domain, source); err == nil {
			return s.offsiteModeForTarget(settings, t)
		}
		return s.ModeFor(settings)
	}
	return s.primaryModeFor(settings, domain, repo)
}

// repoSkip names one repository an operation could not cover, and why, so
// the operation can say it covered less than the whole domain instead of
// reporting success over what was left.
type repoSkip struct {
	Name   string
	Reason string
	// Note marks a skip the operator cannot act on: a permanent consequence of
	// how the instance is configured rather than something that went wrong. It
	// is still reported (skipNames carries it into the answer), but
	// skippedError leaves it out, so it never becomes the operation's error.
	//
	// The error is the failure channel: it stamps the run row red and fires a
	// "FAILED" notification. A shared named repository that can only ever get
	// the stale-lock clear is true, worth saying and unchangeable; delivered as
	// an error, a supported configuration would report failure every night and
	// teach the operator to ignore the one message that matters.
	Note bool
	// Unreachable marks a skip where this box could not open the repository at
	// all, so nothing here can say whether its contents still exist.
	//
	// It is separate from Note because "should this fail the operation" and
	// "could this repository's data be gone" are different questions. A
	// repository the operator switched off is a failure for an operation asked
	// to cover the whole domain (its items are not being backed up), and it is
	// not unreachable: it sits there with its data intact.
	//
	// The off-site retention gate is the one consumer: it must not age a
	// destination that may be the last copy of something. Keyed on "did
	// anything fail" instead, a domain with one switched-off repository would
	// never have its off-site destination aged again on an install where the
	// post-backup hook is the only replication.
	Unreachable bool
	// Ref is the repository the skip is about, when there is one, so a pass that
	// reads the placement later can tell whether anything is copied from it.
	Ref domainRepoRef
}

// skipNames renders a skip list for a JSON response: the repository's own
// name and why it could not be covered, one readable line each. Never a
// location: a path would be redacted on the way out and means nothing to a
// reader anyway.
func skipNames(skipped []repoSkip) []string {
	if len(skipped) == 0 {
		return nil
	}
	out := make([]string, 0, len(skipped))
	for _, s := range skipped {
		out = append(out, fmt.Sprintf("%s (%s)", s.Name, s.Reason))
	}
	return out
}

// actionableSkips drops the Notes and keeps the skips that say something
// went wrong: the ones an operator can act on, and the only ones that may
// fail an operation, colour a pill or hold back a destructive maintenance
// step. Every consumer of a skip list that decides rather than displays
// goes through here, so "permanent and correct" means the same thing in
// all of them.
func actionableSkips(skipped []repoSkip) []repoSkip {
	out := make([]repoSkip, 0, len(skipped))
	for _, s := range skipped {
		if s.Note {
			continue // reported, but not a failure; see repoSkip.Note
		}
		out = append(out, s)
	}
	return out
}

// unreachableSkips keeps the skips where this box could not open the
// repository, so nothing it saw can say whether that repository's contents
// still exist.
//
// The off-site retention gate is the one consumer, and it needs this
// question rather than actionableSkips'. The two differ on the case that
// matters: a repository the operator switched off is a failure for an
// operation asked to cover the whole domain, but it is not unreachable; it
// sits there with its data, so aging the off-site destination cannot leave
// its items without a copy.
func unreachableSkips(skipped []repoSkip) []repoSkip {
	out := make([]repoSkip, 0, len(skipped))
	for _, s := range skipped {
		if s.Unreachable {
			out = append(out, s)
		}
	}
	return out
}

// skippedError turns a skip list into the error an operation returns after
// doing what it could. The work is not abandoned (checking three
// repositories out of four beats checking none), but the outcome is not a
// success either: a run record must not say "success" about a domain half
// of whose data was never opened.
//
// `what` is a noun phrase naming the operation ("this prune", "this
// unlock"), because it is the subject of the sentence below.
func skippedError(what string, skipped []repoSkip) error {
	real := actionableSkips(skipped)
	if len(real) == 0 {
		return nil
	}
	parts := make([]string, 0, len(real))
	for _, s := range real {
		parts = append(parts, fmt.Sprintf("%s (%s)", s.Name, s.Reason))
	}
	return fmt.Errorf("%s covered only part of this domain: %s", what, strings.Join(parts, ", "))
}

// nothingCoveredError is skippedError's counterpart for an operation that
// covered nothing. It is a separate sentence, because "covered only part"
// is false there, and it is the message a single-repository domain whose
// repository went away is most likely to see.
func nothingCoveredError(skipped []repoSkip) error {
	real := actionableSkips(skipped)
	if len(real) == 0 {
		return nil
	}
	parts := make([]string, 0, len(real))
	for _, s := range real {
		parts = append(parts, fmt.Sprintf("%s (%s)", s.Name, s.Reason))
	}
	return fmt.Errorf("nothing in this domain could be opened: %s", strings.Join(parts, ", "))
}

// discoverNamesAcrossRepos collects the item names a domain's snapshots
// carry (from the tagPrefix tag, e.g. "container:") across every repository
// the domain writes to, and remembers which repository each name was found
// in: the value is the named repository's id (#204), or "" for the domain's
// own.
//
// Discover is the path back from a lost /config: it rebuilds items out of
// the snapshots that still exist. An item pointed at a named repository
// must be found there, or its intact backups would never be looked at
// again, and it must be put back on that repository, or its next backup
// would go somewhere else and show an empty history.
//
// A named repository that cannot be listed is skipped rather than failing
// the whole discovery. The domain's own listing failure is returned as an
// error: a wrong APP_KEY, an unmounted share or a corrupt repository all
// surface there, and the Recovery wizard classifies on exactly that error
// to tell "nothing found" from "this box cannot open its own repository".
// As a skip it would show a yellow "not reachable" pill with no message on
// the one screen an operator reaches on their worst day.
//
// The repository list is built here rather than taken from
// domainReposInUse, which derives it from the item rows discovery exists
// to rebuild. After a /config loss those tables are empty, so every named
// repository would drop out of the pass meant to find them; the same
// happens without any loss once the last item using a repository is
// removed. So every enabled named repository is searched, in use or not.
//
// A name is attributed to the repository holding its newest snapshot, not
// to "a named repository wins". If a retired containers folder is
// registered as a named repository so one item can still read it, that
// rule would re-home every container with old snapshots there onto it, and
// its next backup's retention would prune the archive under the domain
// keep-policy while its real history in the domain repository was
// orphaned. The newest snapshot is the only evidence available here of
// where an item is currently sent.
//
// The named repositories are searched before the domain's own: a failure
// on the domain's own ends the pass, so it has to come after everything
// else has been searched, and the error still reaches the caller that
// classifies on it.
//
// The second result maps a discovered name to the formerly: names on its
// own snapshots, so Discover can name each one no stored definition records
// as a link. Only container and VM backups write that tag. The fourth is
// every plain named repository holding bv:direct snapshots of the domain: a
// direct repository that lost its link.
func (s *Service) discoverNamesAcrossRepos(ctx context.Context, settings store.Settings, domain, tagPrefix string) (map[string]string, map[string][]string, []repoSkip, []store.OffsiteTarget, error) {
	own, err := s.repoFor(settings, domain, "local")
	if err != nil {
		return nil, nil, nil, nil, err
	}
	// Every enabled named repository that resolves somewhere else first, the
	// domain's own last: its listing failure ends the pass, so by then the
	// named repositories have already been searched.
	var refs []domainRepoRef
	var skipped []repoSkip
	var direct []store.OffsiteTarget
	named, nErr := s.store.ListNamedRepos()
	if nErr != nil {
		skipped = append(skipped, repoSkip{Name: "the named repositories", Reason: "their list could not be read", Unreachable: true})
		log.Printf("api: discover %s: could not list the named repositories (searching the domain repository only): %v", domain, nErr) //nolint:gosec // G706: domain is a fixed literal
	}
	for _, n := range named {
		if !n.Enabled {
			// Marked Note: switching a repository off is a first-class state, not something
			// that went wrong. It is still said, because after a /config loss the
			// operator needs to know which repositories were left out of the search,
			// but it must not colour the readability pill or fail the pass, or retiring
			// one share would leave Recovery permanently amber and swallow the
			// confirmation toast behind it.
			skipped = append(skipped, repoSkip{Name: scrubSafeName(n.Name), Reason: "switched off", Note: true})
			continue
		}
		loc, rErr := s.resolveRepo(n.Repo)
		if rErr != nil {
			skipped = append(skipped, repoSkip{Name: scrubSafeName(n.Name), Reason: "its location does not resolve", Unreachable: true})
			continue
		}
		if sameRepoLocation(loc, own) {
			continue
		}
		refs = append(refs, namedRef(loc, n))
	}
	refs = append(refs, ownRef(own))
	// best[name] is the newest snapshot seen for that name so far, and the
	// repository id it was in.
	type candidate struct {
		id   string
		when int64
	}
	best := map[string]candidate{}
	// formerly[name] holds the former names on name's snapshots in every
	// repository, not only the one best settles on: a formerly: tag holds
	// wherever it was written.
	formerly := map[string]map[string]bool{}
	// id → display name, so the duplicate-name line below can actually name both
	// sides instead of only saying that two exist. "" is the domain's own.
	refNames := map[string]string{}
	for _, ref := range refs {
		id := ""
		if !ref.Own {
			id = ref.Named.ID
		}
		refNames[id] = s.refName(ref)
	}
	for _, ref := range refs {
		if localRepoMissing(ref.Loc) {
			// The same three-way split the other two repository loops make: a location
			// that was never a repository holds nothing and is dropped silently, one
			// that was a working repository and is now unreachable is the most
			// important thing this pass can report, and "could not tell" goes to the
			// reported side.
			//
			// For the domain's own repository the two reported cases end the pass,
			// like the listing failure below. This branch fires before listSnapshots,
			// and Discover's attribution gate keys on the returned error: an unmounted
			// share takes this path, so without the error the newest-wins comparison
			// would be decided without the repository most items are in, and the
			// result would stick.
			est := s.repoEstablishmentOf(ref.Loc)
			if ref.Own && (est == repoWasEstablished || est == repoEstablishmentUnknown) {
				reason := "it was there before and is not reachable now"
				if est == repoEstablishmentUnknown {
					reason = "it is not reachable now, and whether it ever held backups could not be read"
				}
				log.Printf("api: discover %s: the domain's own repository is not there (%s)", domain, reason) //nolint:gosec // G706: domain is a fixed literal and the reason a fixed string
				skipped = append(skipped, repoSkip{Name: s.refName(ref), Reason: reason, Unreachable: true})
				out := make(map[string]string, len(best))
				for name, c := range best {
					out[name] = c.id
				}
				return out, formerlyOut(formerly), skipped, direct, errors.New("the " + domain + " repository is not reachable: " + reason)
			}
			switch est {
			case repoWasEstablished:
				skipped = append(skipped, repoSkip{Name: s.refName(ref), Reason: "it was there before and is not reachable now", Unreachable: true})
			case repoEstablishmentUnknown:
				skipped = append(skipped, repoSkip{Name: s.refName(ref), Reason: "it is not reachable now, and whether it ever held backups could not be read", Unreachable: true})
			case repoNeverEstablished:
				// Never created. For a named repository that is silence.
				//
				// For the domain's own it is a Note, not an error. An install with every
				// item on a named repository (#204) never creates the domain repository
				// at all, so ending the pass here would break exactly that configuration.
				//
				// The limit: after a /config loss the established marker is gone with the
				// database, so a repository that really exists on an unmounted share also
				// answers "never" here. That combination (configuration lost, share
				// unmounted, and a named repository holding older snapshots of the same
				// item) is what this branch cannot separate. It is named rather than
				// silent so the wizard shows it before anybody trusts the result, and it
				// is a Note rather than a skip because an all-named install would
				// otherwise report a permanent fault.
				if ref.Own {
					skipped = append(skipped, repoSkip{
						Name:   s.refName(ref),
						Reason: "it has not been created yet; if its share is simply not mounted, mount it and search again before trusting this result",
						Note:   true,
					})
				}
			}
			continue
		}
		mode := s.primaryModeFor(settings, domain, ref.Loc)
		snaps, sErr := s.listSnapshots(ctx, ref.Loc, mode)
		if sErr != nil {
			// The domain's own failure ends the pass. This is the error the Recovery
			// wizard classifies on (a wrong APP_KEY, an unmounted share, a corrupt
			// repository); as a skip nobody reads it would turn a red "the APP_KEY
			// differs from when this repo was first created" panel into a silent "0
			// found".
			//
			// It ends the pass with what the pass already has. The named repositories
			// are searched first and are already in `best`, and throwing them away
			// would turn "rebuilt three of five items, and here is why the rest are
			// missing" into "rebuilt nothing". A caller that only wants the error
			// still gets it.
			if ref.Own {
				log.Printf("api: discover %s: could not read the domain's own repository: %v", domain, scrubError(sErr)) //nolint:gosec // G706: domain is a fixed literal and the error scrubbed
				skipped = append(skipped, repoSkip{Name: s.refName(ref), Reason: scrubError(sErr), Unreachable: true})
				out := make(map[string]string, len(best))
				for name, c := range best {
					out[name] = c.id
				}
				return out, formerlyOut(formerly), skipped, direct, sErr
			}
			skipped = append(skipped, repoSkip{Name: s.refName(ref), Reason: scrubError(sErr), Unreachable: true})
			log.Printf("api: discover %s: could not read %s (continuing): %v", domain, s.refName(ref), scrubError(sErr)) //nolint:gosec // G706: domain is a fixed literal, the name is the row's own and the error scrubbed
			continue
		}
		if !ref.Own && ref.Named.CompanionOf == "" && holdsDirect(snaps, tagPrefix) {
			direct = append(direct, ref.Named)
		}
		id := ""
		if !ref.Own {
			id = ref.Named.ID
		}
		for _, snap := range snaps {
			when := int64(0)
			if ts, pErr := time.Parse(time.RFC3339Nano, snap.Time); pErr == nil {
				when = ts.Unix()
			}
			// A backup after a rename carries its identity tag and one formerly:
			// tag per former name, so each former name here belongs to the
			// identity tags on the same snapshot.
			var formerHere []string
			for _, tag := range snap.Tags {
				if old, ok := strings.CutPrefix(tag, "formerly:"); ok && old != "" {
					formerHere = append(formerHere, old)
				}
			}
			for _, tag := range snap.Tags {
				rest, ok := strings.CutPrefix(tag, tagPrefix)
				if !ok || rest == "" {
					continue
				}
				if len(formerHere) > 0 {
					set, ok := formerly[rest]
					if !ok {
						set = map[string]bool{}
						formerly[rest] = set
					}
					for _, old := range formerHere {
						set[old] = true
					}
				}
				prev, seen := best[rest]
				if seen && prev.id != id && prev.when != 0 && when != 0 {
					// Logged at the decision, naming both repositories: "it is in two places"
					// without saying which two is not something anybody can act on. It fires
					// once per pair of differing snapshots rather than once per name, because
					// the decision is re-made whenever a newer snapshot turns up in the other
					// place.
					log.Printf("api: discover %s: %q has backups in both %s and %s; taking the one with the newest snapshot", domain, rest, refNames[prev.id], refNames[id]) //nolint:gosec // G706: domain is a fixed literal, the name is %q-quoted and the repository names are shortened
				}
				// Strictly newer wins, and on a tie the domain's own repository keeps the
				// name. The tie is real: a repository duplicated by `restic copy` or a
				// plain folder copy carries identical snapshot times, and the paths differ
				// so sameRepoLocation does not catch it. The domain's own is the safe
				// answer there, and since the named repositories are listed first, the
				// first-seen rule alone would hand the name to whichever of them came
				// first.
				ownWinsTie := seen && when == prev.when && ref.Own && prev.id != ""
				if !seen || when > prev.when || ownWinsTie {
					best[rest] = candidate{id: id, when: when}
				}
			}
		}
	}
	out := make(map[string]string, len(best))
	for name, c := range best {
		out[name] = c.id
	}
	return out, formerlyOut(formerly), skipped, direct, nil
}

// formerlyOut flattens formerly into sorted lists, so Discover writes the same
// aliases on every run.
func formerlyOut(formerly map[string]map[string]bool) map[string][]string {
	if len(formerly) == 0 {
		return nil
	}
	out := make(map[string][]string, len(formerly))
	for name, set := range formerly {
		olds := make([]string, 0, len(set))
		for old := range set {
			olds = append(olds, old)
		}
		sort.Strings(olds)
		out[name] = olds
	}
	return out
}

// offsiteReplicationSources returns the repositories a domain's off-site
// replication copies from: the domain's own, whether local or remote, plus
// every local named repository (#204) its items point at.
//
// A remote named repository is left out, for two reasons that agree:
//
//   - It is already off site. Pointing a VM straight at b2:bucket/cold is
//     what #204 asked for; copying that bucket into a second cloud is a
//     transfer bill nobody asked for.
//   - restic copy often cannot express it. One process carries one set of
//     backend credentials (restic.Mode.Env), used for the destination; only
//     the repository password has a --from- counterpart. A named repository
//     can carry its own credential set, possibly for a different account or
//     provider, and then one of the two would be authenticated wrongly.
//
// The domain's own remote repository stays in. Its credentials are not
// guaranteed to match either: primaryModeFor applies the primary-remote
// row's own CredsRef (#182) and offsiteModeForTarget the destination
// row's, and those are independent store fields. But it is the setup
// docs/offsite-recovery.md describes, an s3 primary replicated into a
// second bucket of the same account, and the only way such an install gets
// a second copy at all. When the two rows name different credential sets,
// the copy fails loudly with an authentication error every night and
// nothing is written or deleted. Excluding it would empty the source list
// for every domain with a remote primary and no local named repository
// (flash and config always, since they have no named repositories), and
// the nightly pass would report a failure without attempting anything.
//
// A local repository is the easy case: a share on the box is exactly as
// exposed as anything else on it, needs the off-site copy just as much,
// and needs no credentials to read.
//
// A source that does not exist is dropped rather than attempted. With
// every item on a named repository, the domain's own repository leading
// the list is never created (each backup only EnsureRepo's the item's own
// location), and `restic copy` has to open its source to enumerate
// snapshots, so leading with it would fail the whole pass before the named
// repositories were attempted.
func (s *Service) offsiteReplicationSources(settings store.Settings, domain string) ([]domainRepoRef, []repoSkip) {
	repos, skipped, err := s.domainReposInUse(settings, domain)
	if err != nil || len(repos) == 0 {
		own, oErr := s.repoFor(settings, domain, "local")
		if oErr != nil {
			// The domain's path does not resolve. Reporting that as a skip keeps
			// the actionable sentence ("invalid backup path: …") instead of the
			// caller's generic "no repository to replicate", which named nothing
			// anybody could act on.
			return nil, append(skipped, repoSkip{Name: "the " + domain + " repository", Reason: scrubError(oErr), Unreachable: true})
		}
		repos = []domainRepoRef{ownRef(own)}
	}
	// Whether a named repository was among the candidates at all, before any of
	// them is checked for presence: it decides what an all-missing pass means
	// below.
	namedCandidate := slices.ContainsFunc(repos, func(r domainRepoRef) bool { return !r.Own })
	out := make([]domainRepoRef, 0, len(repos))
	for _, r := range repos {
		// A remote named repository is left out; the domain's own is not, however
		// remote. The reason is the credentials, and they follow ownership: one
		// restic process carries one set of backend credentials and copy spends
		// them on the destination. A named repository can carry a credential set
		// of its own (CredsRef) and is offered to every domain, and leaving it out
		// costs nothing, since it is already off site. See the function comment
		// for why the domain's own remote primary stays in; the post-backup hook
		// shares alreadyOffSite, so both halves answer the same way.
		if alreadyOffSite(r) {
			// Logged, not skipped. A skip becomes the operation's error, a red run row
			// and a "replication FAILED" notification, and this exclusion is permanent
			// and correct: nothing the operator can do makes b2: stop being remote, so
			// reporting it as an incomplete pass would make a supported configuration
			// fail forever over a repository that was never meant to be copied.
			log.Printf("api: offsite %s: named repository %s %s; not copied again", domain, scrubRepoLocation(r.Loc), offSiteReason(r)) //nolint:gosec // G706: domain is a fixed literal, the location has any embedded credential redacted
			continue
		}
		out = append(out, r)
	}
	// A repository that was never created holds nothing and is dropped
	// silently. A repository that was created and is now gone is reported. In
	// an all-named install the domain's own repository does not exist at all
	// (each backup only creates the item's own location), so reporting it
	// would fail the nightly run forever; and an unmounted share holding real
	// backups dropped without a word would let the run report success while
	// those items had no off-site copy.
	//
	// markRepoEstablished/repoEstablished is the record of "this location was
	// a working repository once", the same distinction #55 needed for the
	// not-mounted guard.
	present := make([]domainRepoRef, 0, len(out))
	for _, r := range out {
		if !localRepoMissing(r.Loc) {
			present = append(present, r)
			continue
		}
		// Unknown is reported, not swallowed: a transient store error here would
		// otherwise stamp the run "off-site copy is current" for a repository that
		// got no copy at all. Same rule as reposThatExist.
		switch s.repoEstablishmentOf(r.Loc) {
		case repoWasEstablished:
			skipped = append(skipped, repoSkip{Name: s.refName(r), Reason: "it was there before and is not reachable now", Unreachable: true, Ref: r})
		case repoEstablishmentUnknown:
			skipped = append(skipped, repoSkip{Name: s.refName(r), Reason: "it is not reachable now, and whether it ever held backups could not be read", Unreachable: true, Ref: r})
		case repoNeverEstablished:
		}
	}
	if len(present) == 0 {
		if !namedCandidate {
			// A domain with only its own repository, never created, keeps it, so the
			// caller still gets restic's own error rather than a silent no-op.
			return out, skipped
		}
		// A named repository was among the candidates and none of them is
		// present: that is a domain whose items are not copied, not a failure.
		return nil, append(skipped, nothingCopiedNote(domain))
	}
	return present, skipped
}

// refAppendOnly reports whether this repository is flagged append-only,
// and under which toggle, asked of the reference.
//
// It must answer exactly as primaryAppendOnly does; the reference already
// carries the named row, which saves the store read. If the two differed,
// the toggle would be honoured by some of the six gates that read it and
// not by others, and those would repack a repository the screen promises
// is protected.
//
// The flag rather than a bool, because the refusal has to name the card
// the toggle actually lives on, and there are three different cards.
func (s *Service) refAppendOnly(domain string, r domainRepoRef) appendOnlyFlag {
	if !r.Own && r.Named.ID != "" {
		if r.Named.Enabled && r.Named.Immutable {
			return appendOnlyNamedRepo
		}
		return appendOnlyNone
	}
	return s.primaryAppendOnly(domain, r.Loc)
}

// repoSharedWithAnotherDomain reports whether this repository is also written to
// by a domain other than the given one. Only a named repository can be: a
// domain's own belongs to it alone.
func (s *Service) repoSharedWithAnotherDomain(settings store.Settings, domain string, r domainRepoRef) bool {
	if r.Own || r.Named.ID == "" {
		return false
	}
	for _, d := range []string{"containers", "vms", "files"} {
		if d == domain {
			continue
		}
		others, _, err := s.domainReposInUse(settings, d)
		if err != nil {
			// Unknown counts as shared: the question is whether it is safe to force a
			// live lock away, and "I could not find out" is not a yes.
			return true
		}
		for _, o := range others {
			if !o.Own && o.Named.ID == r.Named.ID {
				return true
			}
		}
	}
	return false
}

// alreadyOffSite reports whether a source is one the off-site copy leaves
// out: a direct repository, which lies at its target already, or a
// repository that is not the domain's own and is remote.
//
// The post-backup hook and the whole-domain pass share this one predicate
// so they cannot answer differently. A non-empty Named.ID is not required:
// refFor's fallback yields Own=false with no named row when
// namedRepoForLocation's store read fails, and such a source still has to
// be left out.
//
// The reason is the credentials, not the location, and credentials follow
// ownership: see offsiteReplicationSources for why a domain's own remote
// primary stays in.
func alreadyOffSite(r domainRepoRef) bool {
	return r.Named.CompanionOf != "" || (!r.Own && restic.IsRemoteRepo(r.Loc))
}

// offSiteReason says in a log line why alreadyOffSite left a source out.
func offSiteReason(r domainRepoRef) string {
	if r.Named.CompanionOf != "" {
		return "is the direct repository of an off-site target"
	}
	return "is remote and is already off site"
}

// refName names a repository for a message: a named repository by its name, the
// domain's own by a shortened location. A name is what the operator recognises,
// and unlike a path it survives the error scrubber.
func (s *Service) refName(r domainRepoRef) string {
	if !r.Own && strings.TrimSpace(r.Named.Name) != "" {
		return scrubSafeName(r.Named.Name)
	}
	return shortRepoName(r.Loc)
}

// scrubSafeName makes an operator's free-text repository name survive the
// error scrubber. The name goes into skip lists, verify wrappers and run
// rows, all of which leave through scrubError, whose absolute-path regex
// redacts any slash-led token, so a repository called "NAS/cold" would
// reach the screen as "NAS[path]", matching nothing in the picker.
//
// Slashes become a middle dot rather than being dropped, because the name
// has to stay recognisable: "NAS · cold" still reads as what was typed.
// shortRepoName returns a single path segment for the same reason.
func scrubSafeName(name string) string {
	name = strings.TrimSpace(name)
	if !strings.ContainsAny(name, `/\`) {
		return name
	}
	r := strings.NewReplacer("/", " · ", `\`, " · ")
	return strings.Join(strings.Fields(r.Replace(name)), " ")
}

// Snapshots lists the snapshots of a single container. The containers
// repository is shared, so the listing is filtered by the container:<name> tag
// the backup writes: otherwise the restore panel of one container would list,
// and could restore, another's snapshots.
func (s *Service) Snapshots(ctx context.Context, name, source string) ([]restic.Snapshot, error) {
	return s.containerSnapshotsOf(ctx, name, source, s.containerIdentity(name))
}

// containerSnapshotsOf is Snapshots for an identity the caller has already
// built, so a gate lists with the one it checked.
func (s *Service) containerSnapshotsOf(ctx context.Context, name, source string, id entryIdentity) ([]restic.Snapshot, error) {
	settings, err := s.store.GetSettings()
	if err != nil {
		return nil, fmt.Errorf("read settings: %w", err)
	}
	repo, err := s.containerRepoForName(settings, name, source)
	if err != nil {
		return nil, err
	}
	return s.snapshotsOwnedBy(ctx, repo, s.repoModeFor(settings, "containers", source, repo), id)
}

// containerIdentity is the container entry that answers to name, with every
// name it had before. A rename records an alias but never rewrites the
// snapshots under the old tag, so readers list the old tags too; which of
// those snapshots the entry owns is entryIdentity's rule. A name with no row
// has no aliases, and a name that is another entry's old name cedes that
// entry's pre-link snapshots (cededClaimOn).
func (s *Service) containerIdentity(name string) entryIdentity {
	ownID := ""
	tg, err := s.store.GetTargetByContainer(name)
	if err == nil {
		ownID = tg.ID
	}
	return withRowReadErr(s.aliasedIdentity("container", "container:", name, ownID), err)
}

// vmIdentity is containerIdentity for VMs. name is the libvirt name: on
// TrueNAS the display name is one virsh does not know, so it never appears in
// a tag.
func (s *Service) vmIdentity(name string) entryIdentity {
	ownID := ""
	tg, err := s.store.GetVMTargetByName(name)
	if err == nil {
		ownID = tg.ID
	}
	return withRowReadErr(s.aliasedIdentity("vm", "vm:", name, ownID), err)
}

// withRowReadErr marks id partial when reading its row failed for any reason
// other than there being no row, since the aliases were then never read.
func withRowReadErr(id entryIdentity, err error) entryIdentity {
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		id.readErr = errors.Join(id.readErr, err)
	}
	return id
}

// aliasedIdentity reads targetID's aliases with their link times ("" for a
// name with no row: no aliases). A failed read leaves the entry with its own
// tag alone: its pre-rename history is hidden until a read succeeds, nothing
// is claimed that is not provably its own, and readErr records the failure.
func (s *Service) aliasedIdentity(domain, prefix, name, targetID string) entryIdentity {
	id := tagIdentity(prefix + name)
	id.ceded, id.readErr = s.cededClaimOn(domain, prefix, name, targetID)
	if targetID == "" {
		return id
	}
	aliases, err := s.store.TargetAliases(domain, targetID)
	if err != nil {
		log.Printf("api: %s aliases for %q: %v", domain, name, err) //nolint:gosec // G706: domain is a fixed literal, name %q-quoted
		id.readErr = errors.Join(id.readErr, err)
		return id
	}
	for _, a := range aliases {
		id.aliases = append(id.aliases, newAliasClaim(prefix, a))
	}
	return id
}

// cededClaimOn returns the alias another entry holds on name, if any: a
// machine that took an old name up again must not own the renamed entry's
// snapshots from before the link. An alias on the entry's own current name
// cedes nothing. A failed read cannot rule another entry out, so it cedes the
// whole name until a read succeeds, and the error comes back with it.
func (s *Service) cededClaimOn(domain, prefix, name, targetID string) (*cededClaim, error) {
	a, err := s.store.AliasByOldName(domain, name)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return nil, nil
	case err != nil:
		log.Printf("api: %s alias on %q: %v; none of its snapshots count as its own until this reads", domain, name, err) //nolint:gosec // G706: domain is a fixed literal, name %q-quoted
		return &cededClaim{aliasClaim: aliasClaim{tag: prefix + name}, unknown: true}, err
	case a.TargetID == targetID:
		return nil, nil
	}
	c := &cededClaim{aliasClaim: newAliasClaim(prefix, a)}
	if owner, oErr := s.entryNameByID(domain, a.TargetID); oErr == nil {
		c.owner = prefix + owner
	}
	return c, nil
}

// entryNameByID is the current name of the container or VM row id.
func (s *Service) entryNameByID(domain, id string) (string, error) {
	if domain == "vm" {
		tg, err := s.store.GetVMTargetByID(id)
		return tg.Name, err
	}
	tg, err := s.store.GetTargetByID(id)
	return tg.ContainerName, err
}

// entryIDByName is the id of the container or VM row on name.
func (s *Service) entryIDByName(domain, name string) (string, error) {
	if domain == "vm" {
		tg, err := s.store.GetVMTargetByName(name)
		return tg.ID, err
	}
	tg, err := s.store.GetTargetByContainer(name)
	return tg.ID, err
}

// snapshotsForTag lists an explicit repo (no settings resolution) and returns
// the snapshots carrying tag, oldest first. It reads one literal tag, not an
// entry's history: for the files and config domains, the zvol per-disk tags,
// and the checks that ask about one name's own snapshots. An entry's history
// is read through snapshotsOwnedBy.
func (s *Service) snapshotsForTag(ctx context.Context, repo string, mode restic.Mode, tag string) ([]restic.Snapshot, error) {
	return s.snapshotsForTags(ctx, repo, mode, []string{tag})
}

// snapshotsOwnedBy lists repo and keeps the snapshots id owns: those under its
// current tag that no other entry's alias claims, plus each alias's from
// before that alias was linked. Every reader of an entry's history goes
// through here, so neither a renamed entry nor a machine that took its old
// name up again can reach the other's snapshots.
func (s *Service) snapshotsOwnedBy(ctx context.Context, repo string, mode restic.Mode, id entryIdentity) ([]restic.Snapshot, error) {
	listed, err := s.snapshotsForTags(ctx, repo, mode, id.listTags())
	if err != nil {
		return nil, err
	}
	return id.owned(listed), nil
}

// snapshotsForTags is snapshotsForTag for several tags: a snapshot is included
// if it carries any of them. It applies no ownership rule; snapshotsOwnedBy
// narrows it to what an entry may claim, and retention reads it unfiltered. A
// missing local repo is "no snapshots yet", not an error, unless the repo was
// established before (share not mounted, #55). Remote repos skip that local
// check (see localRepoMissing).
func (s *Service) snapshotsForTags(ctx context.Context, repo string, mode restic.Mode, tags []string) ([]restic.Snapshot, error) {
	if localRepoMissing(repo) {
		// #55 vs #120: only surface "not mounted" when the backing store is truly
		// absent. If the destination is mounted, this is a fresh or phantom repo
		// on a healthy disk, so report an empty list (EnsureRepo re-establishes on
		// write).
		if s.repoEstablished(repo) && !s.destinationMounted(repo) {
			return nil, ErrBackupPathNotMounted // #55: backing store not mounted
		}
		return nil, nil
	}
	all, err := s.listSnapshots(ctx, repo, mode)
	if err != nil {
		return nil, err
	}
	wanted := make(map[string]bool, len(tags))
	for _, t := range tags {
		wanted[t] = true
	}
	out := make([]restic.Snapshot, 0, len(all))
	for _, snap := range all {
		for _, t := range snap.Tags {
			if wanted[t] {
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
	// Scope to the named container: the snapshot must be one of its snapshots,
	// so one container's file tree can't be listed through another's route.
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
		return nil, notInListing{snapshotID, "container"}
	}
	settings, err := s.store.GetSettings()
	if err != nil {
		return nil, fmt.Errorf("read settings: %w", err)
	}
	repo, err := s.containerRepoForName(settings, name, source)
	if err != nil {
		return nil, err
	}
	return s.lsSelfHeal(ctx, repo, snapshotID, s.repoModeFor(settings, "containers", source, repo))
}

// RestoreContainerFiles restores one or more files or dirs from a container
// snapshot. With targetSubPath empty the selected paths are written back to
// their original locations (in place, restic target "/"); with a non-empty
// targetSubPath the selection is extracted into an alternate folder under
// the host mount (non-destructive, same containment as
// RestoreContainerToPath). It returns the resolved absolute target folder
// for the alternate-folder case, or "" for an in-place restore.
//
// Security: confirm-gated; the snapshot id passes the strict hex guard
// (backup.ValidSnapshotID) and must belong to the named container
// (tag-scoped via Snapshots, like RestoreContainerToPath) so one
// container's data can't be extracted through another's route; every
// selected path is path.Cleaned and must sit within the host mount
// (paths.Within), so a restore can never read or write outside the backup
// mount; and the alternate target is resolved with paths.Resolve and
// created (EnsureDir) only after containment passes.
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

// prepareRestoreFiles performs all of a file-level restore's validation and
// resolution synchronously (see the security notes on
// RestoreContainerFiles), so a bad request fails immediately with a clear
// error, and creates the alternate target folder once containment passes.
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

	// Clean each selected path once, so the validated path is the path that runs.
	cleaned := make([]string, 0, len(filePaths))
	for _, p := range filePaths {
		cleaned = append(cleaned, path.Clean(p))
	}

	// Scope to the named container: the snapshot must be one of its snapshots
	// (the same access check as RestoreContainerToPath).
	snaps, err := s.Snapshots(ctx, name, source)
	if err != nil {
		return filesRestorePlan{}, err
	}
	if !snapshotBelongs(snaps, snapshotID) {
		return filesRestorePlan{}, notInListing{snapshotID, "container"}
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
	repo, err := s.containerRepoForName(settings, name, source)
	if err != nil {
		return filesRestorePlan{}, err
	}
	return filesRestorePlan{
		repo:       repo,
		mode:       s.repoModeFor(settings, "containers", source, repo),
		snapshotID: snapshotID,
		paths:      cleaned,
		target:     target,
		resolved:   resolved,
	}, nil
}

// runRestoreFiles restores each selected path of an already-validated plan.
// It is not atomic, since restic writes per path, so if one fails mid-batch
// the error says how many already went through and which path stopped it,
// instead of a bare failure that hides the paths already restored.
func (s *Service) runRestoreFiles(ctx context.Context, plan filesRestorePlan) error {
	// Hold the domain repo lock for the restic work: scheduled backups bypass
	// batchActive and the domain lock is the layer they respect (see
	// executeRestore).
	unlock := s.lockDomainFor("containers", "restore")
	defer unlock()
	for i, c := range plan.paths {
		// escapeGlobLiteral, because --include is a glob like --exclude and c is a
		// path the user ticked in the file picker, not a pattern anyone wrote. A
		// raw "/Inception (2010) [1080p]" restores zero files and restic still
		// exits 0, so the run would be recorded a success with an empty target
		// folder.
		if err := s.engine.RestoreInclude(ctx, plan.repo, plan.snapshotID, escapeGlobLiteral(c), plan.target, plan.mode); err != nil {
			if len(plan.paths) > 1 {
				return fmt.Errorf("restored %d of %d files, then failed on %q: %w", i, len(plan.paths), c, err)
			}
			return err
		}
	}
	return nil
}

// StartRestoreFiles launches a file-level restore in a background goroutine
// and returns immediately (see StartRestore for why). All validation runs
// synchronously (a bad request fails right away, no goroutine); the
// resolved alternate target folder ("" for in-place) is returned in the ack
// so the UI can show it. The detached run publishes "container:<name>"
// progress (phase "restore") and records a run (kind "restore"), so the
// outcome, including the real restic error text, lands in the run history.
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
		// runID is declared before recoverOperation is deferred, so the onPanic
		// closure sees whatever value it holds at panic time. A panic before the
		// assignment means beginRestoreRun never ran and there is no run to close.
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

// beginRestoreRun records, best-effort, the start of a service-layer
// restore run (kind "restore") against the container's target row, so the
// outcome shows up in the run history like the orchestrated in-place
// restore does. It returns "" when recording is impossible (no target row,
// store error); bookkeeping must never block the restore itself.
func (s *Service) beginRestoreRun(name string) string {
	tg, err := s.store.GetTargetByContainer(name)
	if err != nil {
		log.Printf("api: restore: no target row for %q, so the outcome won't appear in the run history: %v", name, err) //nolint:gosec // G706: name is %q-quoted
		return ""
	}
	return s.beginRestoreRunForTarget(tg.ID)
}

// beginRestoreRunForTarget is beginRestoreRun for an already resolved
// runs.target_id (a container target's ID, or a file set's stable id,
// since the files domain records its restores against file_sets.id
// directly), so every per-item domain shares one restore-bookkeeping path.
// It returns "" when recording fails; bookkeeping must never block the
// restore itself.
func (s *Service) beginRestoreRunForTarget(targetID string) string {
	// A restore run is never part of a "Backup Everything" pass's grouped
	// children (see runGroupKey), so context.Background() loses nothing.
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
	// As in beginRestoreRunForTarget, a restore run is never part of a grouped
	// pass, so context.Background() loses nothing in any branch below.
	var err error
	switch {
	case rerr == nil:
		err = runsAdapter{st: s.store, ctx: context.Background()}.Finish(runID, "success", snapshotID, 0, "")
	case errors.Is(rerr, context.Canceled):
		// A user cancel is an intentional, recorded outcome, not a failure: record
		// it as "cancelled" and fire no failure alert (the terminal progEnd already
		// fired to clear the bar).
		err = runsAdapter{st: s.store, ctx: context.Background()}.Finish(runID, "cancelled", "", 0, "cancelled by user")
	default:
		err = runsAdapter{st: s.store, ctx: context.Background()}.Finish(runID, "failed", "", 0, truncateRunErr(rerr))
	}
	if err != nil {
		log.Printf("api: restore: record run finish failed: %v", err)
	}
}

// finishRestoreRunWarn closes a restore run as success but records warn in
// the run's error column, for when restic extracted all data yet could not
// set ownership or metadata on the target (see
// restic.ErrRestoreMetadataOnly). The run counts as a success everywhere
// (health, retention gates); the message tells the operator the original
// ownership could not be reproduced on the share.
func (s *Service) finishRestoreRunWarn(runID, snapshotID, warn string) {
	if runID == "" {
		return
	}
	// As in beginRestoreRunForTarget, a restore run is never part of a grouped
	// pass, so context.Background() loses nothing.
	err := runsAdapter{st: s.store, ctx: context.Background()}.Finish(runID, "success", snapshotID, 0, warn)
	if err != nil {
		log.Printf("api: restore: record run finish (warning) failed: %v", err)
	}
}

// concludeFileSetRestore ends a file-set restore (settings-driven or
// foreign): it drives the terminal progress event and the run record, and
// downgrades a metadata-only restic failure (all data present, only
// ownership or metadata could not be set on a /mnt/user FUSE share,
// restic.ErrRestoreMetadataOnly) to a success with a warning. Genuine
// failures (missing snapshot, no space, unreachable repo) and user cancels
// are recorded as they are. It returns the effective error (nil for the
// metadata-only case), so the caller logs or propagates only a real
// failure. startedAt must be the value the matching progBegin returned
// (see progEnd).
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

// RestoreContainerToPath extracts a whole container snapshot into an
// alternate folder under the host mount; it is non-destructive, and the
// live container is never touched. Unlike Restore, it stops, removes and
// recreates nothing: it is for inspecting, cloning or migrating a
// snapshot's data. It returns the resolved absolute target path
// (container-visible, under the host mount root); the handler scrubs it for
// the UI.
//
// Security: the snapshot id passes the strict hex guard
// (backup.ValidSnapshotID, as for the file and in-place restores), the
// snapshot must belong to the named container (tag-scoped via Snapshots,
// like ListSnapshotFiles), and the target is resolved with
// paths.Resolve(HostMountRoot, targetSubPath), the containment helper
// SetBackupPaths and handleBrowse use, which path.Cleans and rejects
// absolute and `..` escapes. The directory is created (MkdirAll) only
// after containment passes.
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

// prepareRestoreToPath performs all of a to-folder restore's validation and
// resolution synchronously (see the security notes on
// RestoreContainerToPath), so a bad request fails immediately with a clear
// error, and creates the target folder once containment passes.
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
		// paths.Resolve returns ErrTraversal or ErrAbsoluteSub, neither of which
		// leaks a host path; keep the message generic, as handleBrowse does.
		return toPathRestorePlan{}, errors.New("invalid target folder: must be a relative subpath under the host mount")
	}

	// Scope to the named container: the snapshot must be one of its snapshots,
	// so one container's data can't be extracted through another's route (the
	// same access check as ListSnapshotFiles).
	snaps, err := s.Snapshots(ctx, name, source)
	if err != nil {
		return toPathRestorePlan{}, err
	}
	if !snapshotBelongs(snaps, snapshotID) {
		return toPathRestorePlan{}, notInListing{snapshotID, "container"}
	}

	settings, err := s.store.GetSettings()
	if err != nil {
		return toPathRestorePlan{}, fmt.Errorf("read settings: %w", err)
	}
	repo, err := s.containerRepoForName(settings, name, source)
	if err != nil {
		return toPathRestorePlan{}, err
	}

	// Create the target dir only after containment passed.
	if err := paths.EnsureDir(target); err != nil {
		return toPathRestorePlan{}, fmt.Errorf("create target folder: %w", err)
	}
	return toPathRestorePlan{
		repo:       repo,
		mode:       s.repoModeFor(settings, "containers", source, repo),
		snapshotID: snapshotID,
		target:     target,
	}, nil
}

// runRestoreToPath restores the whole snapshot tree of an already-validated
// plan into the target dir: restic restore --target <dir> --include /
// (everything). It reuses the restore-to-target engine method; "/"
// includes all paths in the snapshot.
func (s *Service) runRestoreToPath(ctx context.Context, plan toPathRestorePlan) error {
	// Hold the domain repo lock for the restic work: scheduled backups bypass
	// batchActive and the domain lock is the layer they respect (see
	// executeRestore).
	unlock := s.lockDomainFor("containers", "restore")
	defer unlock()
	return s.engine.RestoreInclude(ctx, plan.repo, plan.snapshotID, "/", plan.target, plan.mode)
}

// StartRestoreToPath launches a whole-snapshot extraction into an
// alternate folder in a background goroutine and returns immediately (see
// StartRestore for why; multi-hour extractions are the usual case, #24).
// All validation runs synchronously (a bad request fails right away, no
// goroutine); the resolved target folder is returned in the ack so the UI
// can show it. The detached run publishes "container:<name>" progress
// (phase "restore") and records a run (kind "restore"), so the outcome,
// including the real restic error text, lands in the run history.
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
// Security: both snapshot ids pass the strict hex guard
// (backup.ValidSnapshotID), and both must belong to the named container
// (tag-scoped via Snapshots, like RestoreContainerToPath and
// ListSnapshotFiles), so one container's snapshots can't be diffed through
// another's route. The repo and mode are resolved for the source.
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

	// Scope to the named container: both snapshots must be among its snapshots.
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
	repo, err := s.containerRepoForName(settings, name, source)
	if err != nil {
		return restic.DiffResult{}, err
	}
	return s.engine.Diff(ctx, repo, snap1, snap2, s.repoModeFor(settings, "containers", source, repo))
}

// TagSnapshot adds tags to one of a container's snapshots (restic tag --add).
//
// Security: the snapshot id passes the strict hex guard and must belong to
// the named container (tag-scoped via Snapshots). Tags are sanitised:
// trimmed, empties dropped, and any tag with a comma or control character
// rejected (restic tags are comma-separated, so a comma would silently
// split into two tags). An empty resulting tag set is a no-op.
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

	// Scope to the named container: the snapshot must be among its snapshots.
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
	repo, err := s.containerRepoForName(settings, name, source)
	if err != nil {
		return err
	}
	mode := s.repoModeFor(settings, "containers", source, repo)
	// Serialize against a live backup/prune on this repo: restic tag takes an
	// exclusive lock, so run it under the domain lock like the other maintenance
	// ops, and report a clean busy instead of colliding on restic's repo lock.
	unlock, ok := s.tryLockDomainFor("containers", "tag")
	if !ok {
		return errDomainBusy
	}
	defer unlock()
	// unlockStale clears a genuine stale orphan (dead PID on this host) left by
	// a crashed run before restic takes its exclusive tag lock: the off-site
	// repo can carry a lock left by an interrupted off-site op (replication
	// copy or integrity check), and `restic tag` would otherwise fail with
	// "repository is already locked" (#29). Every other repo-mutating path
	// (backups, PruneDomain, DeleteSnapshot) does the same.
	s.unlockStale(ctx, repo, mode)
	return s.engine.TagAdd(ctx, repo, snapID, tags, mode)
}

// snapshotBelongs reports whether id (exact or unique prefix) is present in
// the already tag-scoped snapshot list, the access check shared by the
// diff, tag and restore-to-path routes.
func snapshotBelongs(snaps []restic.Snapshot, id string) bool {
	for _, sn := range snaps {
		if sn.ID == id || strings.HasPrefix(sn.ID, id) {
			return true
		}
	}
	return false
}

// chosenSnapshot returns the snapshot in snaps matching id (exact or
// unambiguous prefix, like snapshotBelongs/snapshotSubtree), or nil when
// there is no match. The restore path mapping reads the chosen snapshot's
// full recorded Paths from it, the source of truth for which selectors are
// valid in this snapshot (a recompute from the stored list would miss
// after the selection changed).
func chosenSnapshot(snaps []restic.Snapshot, id string) *restic.Snapshot {
	for i := range snaps {
		if snaps[i].ID == id || strings.HasPrefix(snaps[i].ID, id) {
			return &snaps[i]
		}
	}
	return nil
}

// snapshotRestoreRoot returns the single tree node that covers every path
// the snapshot in snaps (matched by id, exact or unambiguous prefix, like
// snapshotBelongs) recorded: their deepest common ancestor. It is the
// subtree a to-folder restore extracts (<id>:<subtree>), read from the
// snapshot so it stays valid even when HostMountRoot changed since the
// backup (a recompute from the set's path would then miss). "" means there
// is no match, the snapshot recorded no path, or the paths share no
// ancestor below "/"; all three fall back to a whole-tree restore at the
// call site.
//
// A multi-root snapshot is the normal shape, since the file-set backup
// compiles one positional per selected root (fileSetPositionals ->
// FileSetBackupDeps.SourcePaths), so returning only the first recorded
// root would drop the others and report success with data missing.
// Restoring the common ancestor widens nothing: a snapshot tree contains
// only what was backed up, so the ancestor node holds exactly the recorded
// roots. With restic 0.17, a snapshot recording docs/keep-a and docs/keep-b
// restored as <id>:docs yields keep-a and keep-b and not the
// never-backed-up sibling; TestRestoreCommonAncestorOfRecordedRoots in
// internal/restic pins it.
func snapshotRestoreRoot(snaps []restic.Snapshot, id string) string {
	for _, sn := range snaps {
		if sn.ID == id || strings.HasPrefix(sn.ID, id) {
			return commonAncestor(sn.Paths)
		}
	}
	return ""
}

// commonAncestor returns the deepest path that is at or above every one of
// paths, or "" when there is none below the filesystem root (an empty
// list, or roots on different top-level branches). Segment-aligned, so /a
// never counts as an ancestor of /ab, the same rule as isStrictDescendant
// and internal/paths.Resolve.
// It trims the first path down rather than rebuilding one from segments: a
// recorded path is a selector that has to reach restic unchanged, and a
// rebuild would normalise whatever the original looked like (snapshots
// taken on Windows carry a drive letter and mixed separators, and a
// leading "/" would make every containment check miss).
func commonAncestor(paths []string) string {
	if len(paths) == 0 {
		return ""
	}
	if len(paths) == 1 {
		// A single root is handed back exactly as recorded, byte for byte.
		return paths[0]
	}
	covers := func(a string) bool {
		for _, p := range paths {
			if q := path.Clean(p); q != a && !isStrictDescendant(q, a) {
				return false
			}
		}
		return true
	}
	cand := path.Clean(paths[0])
	for {
		if covers(cand) {
			return cand
		}
		parent := path.Dir(cand)
		// Reaching the filesystem root means the roots live on different
		// branches: there is no single node to extract, so the caller falls back
		// to a whole-tree restore rather than emitting a "<id>:/" selector.
		if parent == cand || parent == "." || parent == "/" {
			return ""
		}
		cand = parent
	}
}

// vmRunTag returns the "vmrun:<runID>" correlation tag
// (VMBackupDeps.RunTag in internal/backup/vm_orchestrator.go) carried by
// the snapshot in snaps matching id (exact or unambiguous prefix, like
// snapshotBelongs/snapshotSubtree above), or "" when there is no match or
// the matching snapshot carries no such tag.
//
// "" is a permanent restore fallback: BackupVM sets RunTag only when the
// VM has zvol disks, since a file-only VM's single snapshot is already
// identified by its plain "vm:<name>" tag (see BackupVM's RunTag comment).
// So "" covers both runs older than the tag and every file-only VM's
// backup, which is most VMs. Callers treat "" as "resolve via id alone".
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
// snapshotsForTag listing) carrying tag exactly, a zvol disk's own
// "vm:<name>:zvol:<dev>" identity tag (see VMBlockDisk.Dev in
// internal/backup/vm_orchestrator.go), or false when no member does.
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

// DeleteBackups removes every backup of a container. From the local source it
// also forgets the container's entry; from an off-site source it deletes at that
// target only and the entry stays.
//
// Forgetting the entry takes its aliases with it, so the pre-link part of an
// old name falls to whichever entry uses that name next. The user asked for
// these backups to go, so that is the answer they want.
func (s *Service) DeleteBackups(ctx context.Context, name, source string) error {
	if isOffsiteSource(source) {
		if err := refuseDeleteWithPartialIdentity(name, s.containerIdentity(name)); err != nil {
			return err
		}
		_, err := s.forgetAtTarget(ctx, "containers", "container:"+name, source, taggedForItem, nil)
		return err
	}
	settings, err := s.store.GetSettings()
	if err != nil {
		return fmt.Errorf("read settings: %w", err)
	}
	repo, err := s.containerRepoForName(settings, name, "local")
	if err != nil {
		return err
	}
	mode := s.repoModeFor(settings, "containers", "local", repo)

	// Serialize against a live backup on this repo: a container Backup holds
	// the domain lock for its whole run (potentially hours), so without this
	// an unlocked bulk delete could race a concurrent `restic forget --prune`
	// against the same repo files. DeleteBackupsVM and DeleteBackupsFileSet
	// use the same guard. (No requireExistingRepo here, unlike those two: a
	// never-backed-up container's target row must still be cleaned up below.)
	//
	// The append-only question is asked before the domain lock and before
	// anything is written. It cannot refuse outright, because a row with no
	// backups must stay clearable, so the refusal itself sits after the
	// listing; but nothing on the way there may touch a repository the
	// interface promises nothing on this box may delete from: not the domain
	// lock (a scheduled containers backup would get errDomainBusy from a call
	// that can only be refused), and not a stale unlock, which removes lock
	// files and on a local append-only folder succeeds.
	protection := s.primaryAppendOnly("containers", repo)
	if protection != appendOnlyNone {
		// A read, and it answers the only question left: is there anything in here
		// for the flag to protect?
		snaps, sErr := s.Snapshots(ctx, name, "")
		if sErr != nil {
			return sErr
		}
		if len(snaps) > 0 {
			return appendOnlyRefusal(protection)
		}
	}

	unlock, ok := s.tryLockDomainFor("containers", "delete")
	if !ok {
		return errDomainBusy
	}
	defer unlock()
	id := s.containerIdentity(name)
	if err := refuseDeleteWithPartialIdentity(name, id); err != nil {
		return err
	}
	// The entry's former names keep its copy rule unless a container installed
	// under one follows its own.
	held, err := s.heldContainerNames(ctx)
	if err != nil {
		return fmt.Errorf("nothing was deleted: %w", err)
	}
	if protection == appendOnlyNone {
		s.unlockStale(ctx, repo, mode)
	}

	snaps, err := s.containerSnapshotsOf(ctx, name, "", id)
	if err != nil {
		return err
	}
	ids := make([]string, 0, len(snaps))
	for _, snap := range snaps {
		ids = append(ids, snap.ID)
	}
	if len(ids) > 0 {
		// The append-only refusal is asked here rather than at the top because the
		// flag protects snapshots and this function does two things: it forgets
		// them, and it removes the target row. With nothing to forget there is
		// nothing to protect, and refusing anyway would leave a container with no
		// backups stuck in the "not installed (backups only)" list, with no way
		// out but switching the whole repository's protection off, which drops it
		// for every other item sharing that repository.
		//
		// While an entry has backups, this is the only removal the not-installed
		// card offers (#232 added a row-only route, handleForgetContainer, for an
		// entry without any), which is what makes this the difference between an
		// inconvenience and a dead end.
		//
		// It is asked again rather than replayed. The check above runs outside the
		// domain lock and answers "is there anything to protect"; this one runs
		// inside it and answers "may this delete happen", and between the two an
		// operator can have turned the protection on, which is exactly when they
		// most mean it. A flag honoured only if it was already set when the button
		// was pressed is not a protection. Turning it off mid-flight costs one
		// refused delete and a second press.
		if f := s.primaryAppendOnly("containers", repo); f != appendOnlyNone {
			return appendOnlyRefusal(f)
		}
		if err := s.engine.Forget(ctx, repo, ids, true, mode); err != nil {
			return fmt.Errorf("forget snapshots: %w", err)
		}
	}

	// Remove the target row + its run history so the container disappears from
	// the "not installed" list once its backups are gone.
	if err := s.store.DeleteTarget(name, held); err != nil {
		return fmt.Errorf("delete target: %w", err)
	}
	return nil
}

// DeleteBackupsVM removes every backup of a VM. From the local source it also
// forgets the VM's entry; from an off-site source it deletes the snapshots the
// VM's backup list shows at that target and the entry stays. Its disk images
// live under their own tags and go through the window that shows them first.
// It is serialised against VM backups by the domain lock and clears stale locks
// first, so a leftover lock cannot fail it.
func (s *Service) DeleteBackupsVM(ctx context.Context, name, source string) error {
	if isOffsiteSource(source) {
		if err := refuseDeleteWithPartialIdentity(name, s.vmIdentity(name)); err != nil {
			return err
		}
		_, err := s.forgetAtTarget(ctx, "vms", "vm:"+name, source, taggedForItem, nil)
		return err
	}
	settings, err := s.store.GetSettings()
	if err != nil {
		return fmt.Errorf("read settings: %w", err)
	}
	// The VM's own repository (#204), not the domain's: deleting a VM's
	// backups has to reach the repository they were written to, or the call
	// succeeds against the domain repo and leaves the real snapshots behind.
	repo, err := s.vmRepoForName(settings, name, source)
	if err != nil {
		return err
	}
	// The same refusal applies when the local source is a remote primary flagged
	// append-only in its safety settings (#152). There is no separate off-site
	// copy in that shape, so this is the only thing between an on-box credential
	// and a Forget with prune against the sole backup.
	if f := s.primaryAppendOnly("vms", repo); f != appendOnlyNone {
		return appendOnlyRefusal(f)
	}
	if err := s.requireExistingRepo(repo, "no backups to delete yet"); err != nil {
		// A repository that was never created holds no backups, and nothing is
		// left to delete but the entry (#232); refusing here would leave a
		// not-installed card whose one removal button can never succeed.
		// snapshotsForTag tells that case apart from an established repository
		// whose share is not mounted (#55), where every snapshot still exists and
		// the entry is the only thing pointing at them: that one is refused, as
		// DeleteBackups does for a container. As this only removes the entry, it
		// keeps the entry of a VM still defined, like ForgetVMTarget.
		//
		// snapshotsForTag reads "could not tell whether it was established" as
		// "never", which is right for a list and wrong for a removal: the entry
		// could be all that points at a repository on an unmounted share. So that
		// answer is asked first and refused, the way reposThatExist words it.
		if s.repoEstablishmentOf(repo) == repoEstablishmentUnknown {
			return errors.New("this VM's repository is not reachable now, and whether it ever held backups could not be read, so its entry stays")
		}
		if _, sErr := s.snapshotsForTag(ctx, repo, s.repoModeFor(settings, "vms", source, repo), "vm:"+name); sErr != nil {
			return sErr
		}
		installed, lErr := s.installedVMs(ctx, settings)
		if lErr != nil {
			return fmt.Errorf("%q keeps its entry until the VMs on the host can be listed: %w", name, lErr)
		}
		if dErr := refuseDefinedVM(installed, name); dErr != nil {
			return dErr
		}
		unlock, ok := s.tryLockDomainFor("vms", "delete")
		if !ok {
			return errDomainBusy
		}
		defer unlock()
		if err := refuseDeleteWithPartialIdentity(name, s.vmIdentity(name)); err != nil {
			return err
		}
		held, err := s.heldVMNames(ctx)
		if err != nil {
			return fmt.Errorf("%q keeps its entry: %w", name, err)
		}
		if err := s.store.DeleteVMTarget(name, held); err != nil {
			return fmt.Errorf("delete vm target: %w", err)
		}
		return nil
	}
	unlock, ok := s.tryLockDomainFor("vms", "delete")
	if !ok {
		return errDomainBusy
	}
	defer unlock()
	mode := s.repoModeFor(settings, "vms", source, repo)
	s.unlockStale(ctx, repo, mode)

	id := s.vmIdentity(name)
	if err := refuseDeleteWithPartialIdentity(name, id); err != nil {
		return err
	}
	// The entry's former names keep its copy rule unless a VM defined under
	// one follows its own.
	held, err := s.heldVMNames(ctx)
	if err != nil {
		return fmt.Errorf("nothing was deleted: %w", err)
	}
	snaps, err := s.vmSnapshotsOf(ctx, name, source, id)
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

	if err := s.store.DeleteVMTarget(name, held); err != nil {
		return fmt.Errorf("delete vm target: %w", err)
	}
	return nil
}

// ForgetVMTarget removes a VM's target row and run history without
// touching any repo, to clear a stale "Not installed" entry that has no
// backups (which also stops the scheduler from retrying a deleted VM).
// Deleting actual backups is DeleteBackupsVM; this is only the bookkeeping.
//
// Refused for a VM that is defined on the host, asked the way ListVMs asks
// (libvirt only while VMs are enabled), so a card left open while the VM came
// back cannot wipe a live VM's settings and history. Serialised against VM
// backups and restores like DeleteBackupsVM: a restore of this entry writes
// the run row this would delete. Refused too while the entry owns a backup
// anywhere it replicates to (refuseRowRemovalWithBackups).
func (s *Service) ForgetVMTarget(ctx context.Context, name string) error {
	settings, err := s.store.GetSettings()
	if err != nil {
		return fmt.Errorf("read settings: %w", err)
	}
	installed, err := s.installedVMs(ctx, settings)
	if err != nil {
		return fmt.Errorf("%q keeps its entry until the VMs on the host can be listed: %w", name, err)
	}
	if err := refuseDefinedVM(installed, name); err != nil {
		return err
	}
	unlock, ok := s.tryLockDomainFor("vms", "delete")
	if !ok {
		return errDomainBusy
	}
	defer unlock()
	repo, err := s.vmRepoForName(settings, name, "")
	if err != nil {
		return fmt.Errorf("%q keeps its entry: its repository could not be resolved to check for backups: %w", name, err)
	}
	if err := s.refuseRowRemovalWithBackups(ctx, settings, "vms", name, repo, s.vmIdentity(name)); err != nil {
		return err
	}
	held, err := s.heldVMNames(ctx)
	if err != nil {
		return fmt.Errorf("%q keeps its entry: %w", name, err)
	}
	if err := s.store.DeleteVMTarget(name, held); err != nil {
		return fmt.Errorf("forget vm target: %w", err)
	}
	return nil
}

// refuseRowRemovalWithBackups refuses to remove the row of name while its
// identity id owns a snapshot in repo or any off-site target of domain, or
// while one of them cannot be read. The row carries the aliases that make its
// older backups its own; without them those backups would fall to whichever
// entry takes the name next.
func (s *Service) refuseRowRemovalWithBackups(ctx context.Context, settings store.Settings, domain, name, repo string, id entryIdentity) error {
	if id.readErr != nil {
		return fmt.Errorf("%q keeps its entry: its backups could not be checked: %w", name, id.readErr)
	}
	places, err := s.backupPlaces(settings, domain, []string{repo})
	if err != nil {
		return fmt.Errorf("%q keeps its entry until its backups can be ruled out: %w", name, err)
	}
	for _, p := range places {
		owned, err := s.snapshotsOwnedBy(ctx, p.repo, p.mode, id)
		if err != nil {
			return fmt.Errorf("%q keeps its entry until its backups can be ruled out: %s could not be read: %w", name, p.name, err)
		}
		if len(owned) > 0 {
			return fmt.Errorf("%q still has backups in %s; delete its backups instead", name, p.name)
		}
	}
	return nil
}

// refuseDeleteWithPartialIdentity refuses delete-all while id is partial,
// because it would forget only part of the entry's backups and then drop the
// aliases that make the rest its own.
func refuseDeleteWithPartialIdentity(name string, id entryIdentity) error {
	if id.readErr == nil {
		return nil
	}
	return fmt.Errorf("nothing was deleted: the backups of %q could not be checked: %w", name, id.readErr)
}

// refuseDefinedVM answers an error when name is among installed, for the routes
// that remove only a VM's entry, so none of them can drop the settings and
// history of a live VM.
func refuseDefinedVM(installed map[string]bool, name string) error {
	if installed[name] {
		return fmt.Errorf("VM %q is defined on the host, so its entry stays", name)
	}
	return nil
}

// installedContainers is the set of containers Docker lists, the ones the
// container list shows as installed.
func (s *Service) installedContainers(ctx context.Context) (map[string]bool, error) {
	infos, err := s.docker.List(ctx)
	if err != nil {
		return nil, err
	}
	installed := make(map[string]bool, len(infos))
	for _, c := range infos {
		installed[c.Name] = true
	}
	return installed, nil
}

// heldContainerNames is what the store is told Docker lists when it decides on
// the copy rule of a former container name. Docker is asked only while some
// container has a former name, so nothing else waits for it.
func (s *Service) heldContainerNames(ctx context.Context) (map[string]bool, error) {
	aliases, err := s.store.ListAliases("container")
	if err != nil {
		return nil, err
	}
	if len(aliases) == 0 {
		return nil, nil
	}
	installed, err := s.installedContainers(ctx)
	if err != nil {
		return nil, fmt.Errorf("the installed containers could not be listed: %w", err)
	}
	return installed, nil
}

// installedVMs is the set of VMs the VM list shows as installed: the ones
// libvirt defines while VMs are enabled, and none while they are off, when
// every entry is listed as not installed.
func (s *Service) installedVMs(ctx context.Context, settings store.Settings) (map[string]bool, error) {
	if !settings.VMsEnabled {
		return map[string]bool{}, nil
	}
	return s.definedVMs(ctx)
}

// definedVMs is the set of VMs libvirt defines, whatever the VMs setting says.
func (s *Service) definedVMs(ctx context.Context) (map[string]bool, error) {
	infos, err := s.virsh.List(ctx)
	if err != nil {
		return nil, err
	}
	defined := make(map[string]bool, len(infos))
	for _, vm := range infos {
		defined[vm.Name] = true
	}
	return defined, nil
}

// heldVMNames is what the store is told libvirt defines when it decides on the
// copy rule of a former VM name. libvirt is asked whatever the VMs setting says,
// since a rule written while VMs are off applies once they are on again, and
// when it cannot answer every former name counts as held.
func (s *Service) heldVMNames(ctx context.Context) (map[string]bool, error) {
	aliases, err := s.store.ListAliases("vm")
	if err != nil {
		return nil, err
	}
	if len(aliases) == 0 {
		return nil, nil
	}
	defined, err := s.definedVMs(ctx)
	if err == nil {
		return defined, nil
	}
	log.Printf("api: the VMs on the host could not be listed, so every former VM name keeps the copy rule it has: %v", err)
	held := make(map[string]bool, len(aliases))
	for _, a := range aliases {
		held[a.OldName] = true
	}
	return held, nil
}

// ForgetTarget removes a container's target row and run history without
// touching any repo, the container twin of ForgetVMTarget (#232). It
// clears a "Not installed" entry that has no backups, which also takes it
// off the schedule. Deleting actual backups is DeleteBackups; this is only
// the bookkeeping. Refused for an installed container and serialised like
// ForgetVMTarget, for the same reasons. Refused too while the entry owns a
// backup anywhere it replicates to (refuseRowRemovalWithBackups).
func (s *Service) ForgetTarget(ctx context.Context, name string) error {
	installed, err := s.installedContainers(ctx)
	if err != nil {
		return fmt.Errorf("list containers: %w", err)
	}
	if installed[name] {
		return fmt.Errorf("container %q is installed, so its entry stays", name)
	}
	unlock, ok := s.tryLockDomainFor("containers", "delete")
	if !ok {
		return errDomainBusy
	}
	defer unlock()
	settings, err := s.store.GetSettings()
	if err != nil {
		return fmt.Errorf("read settings: %w", err)
	}
	repo, err := s.containerRepoForName(settings, name, "")
	if err != nil {
		return fmt.Errorf("%q keeps its entry: its repository could not be resolved to check for backups: %w", name, err)
	}
	if err := s.refuseRowRemovalWithBackups(ctx, settings, "containers", name, repo, s.containerIdentity(name)); err != nil {
		return err
	}
	if err := s.store.DeleteTarget(name, installed); err != nil {
		return fmt.Errorf("forget target: %w", err)
	}
	return nil
}

// rewriteDefinitionJSON returns definition, a target's stored recreate recipe,
// with Inspect.Name and the template's <Name> set to name, so a restore before
// the next backup recreates the container under its new name. It does no
// store I/O because the takeover and the unlink write its result in the same
// transaction as the rename; a restore can then never pair the new name with
// the old definition. An empty definition (never backed up) is returned
// unchanged.
func rewriteDefinitionJSON(definition, name string) (string, error) {
	if definition == "" {
		return "", nil
	}
	var def containerDefinition
	if err := json.Unmarshal([]byte(definition), &def); err != nil {
		return "", fmt.Errorf("unmarshal: %w", err)
	}
	if strings.HasPrefix(def.Inspect.Name, "/") {
		def.Inspect.Name = "/" + name
	} else {
		def.Inspect.Name = name
	}
	def.TemplateXML = rewriteTemplateXMLName(def.TemplateXML, name)
	defBytes, err := json.Marshal(def)
	if err != nil {
		return "", fmt.Errorf("marshal: %w", err)
	}
	return string(defBytes), nil
}

// rewriteStopLists points every other entry's stop list from oldName to
// newName, so a container that stops the renamed one during its own backup
// keeps stopping it.
func (s *Service) rewriteStopLists(oldName, newName string) error {
	targets, err := s.store.ListTargets()
	if err != nil {
		return fmt.Errorf("rewrite stop lists: list targets: %w", err)
	}
	for _, t := range targets {
		if t.ContainerName == newName {
			continue
		}
		changed := false
		stop := make([]string, len(t.StopContainers))
		for i, n := range t.StopContainers {
			if n == oldName {
				n = newName
				changed = true
			}
			stop[i] = n
		}
		if !changed {
			continue
		}
		if err := s.store.SetStopContainers(t.ContainerName, stop); err != nil {
			return fmt.Errorf("rewrite stop lists: %q: %w", t.ContainerName, err)
		}
	}
	return nil
}

// configuredStateLabels lists the operator-set fields t carries, the ones a
// takeover may not discard just because the row has no backups yet.
func configuredStateLabels(t store.Target) []string {
	var labels []string
	if t.IncludeInSchedule {
		labels = append(labels, "scheduled")
	}
	if t.PreHook != "" || t.PostHook != "" {
		labels = append(labels, "hooks")
	}
	if len(t.Excludes) > 0 {
		labels = append(labels, "excludes")
	}
	if len(t.ExcludeCaches) > 0 {
		labels = append(labels, "exclude-caches")
	}
	if len(t.StopContainers) > 0 {
		labels = append(labels, "stop list")
	}
	if len(t.SelectedPaths) > 0 {
		labels = append(labels, "selected paths")
	}
	if t.Repo != "" {
		labels = append(labels, "repository override")
	}
	if t.ScheduleCadence != "" {
		labels = append(labels, "schedule cadence")
	}
	if t.BackupOrder != 0 {
		labels = append(labels, "backup order")
	}
	if t.UpdateAfterBackup {
		labels = append(labels, "update-after-backup")
	}
	return labels
}

// targetOccupyingNewNameIsEmpty reports whether t, the row already on a
// takeover's new name, may be deleted to make way for the entry on from: it has
// no backups, no operator-set configuration and no copy rule other than the
// entry's own. labels names what it found, so the refusal can say what would
// be lost.
func (s *Service) targetOccupyingNewNameIsEmpty(ctx context.Context, t store.Target, from string) (empty bool, labels []string, err error) {
	labels, err = s.withCopyRule(configuredStateLabels(t), "containers", "container:"+from, "container:"+t.ContainerName)
	if err != nil {
		return false, nil, err
	}
	if len(labels) > 0 {
		return false, labels, nil
	}
	hasBackups, err := s.containerHasBackups(ctx, t.ContainerName)
	if err != nil {
		return false, nil, err
	}
	return !hasBackups, nil, nil
}

// ownNamesFor returns the target's current name and every alias recorded for
// it.
func (s *Service) ownNamesFor(domain, targetID, currentName string) (map[string]bool, error) {
	own := map[string]bool{currentName: true}
	names, err := s.store.AliasNames(domain, targetID)
	if err != nil {
		return nil, fmt.Errorf("read alias names for %q: %w", currentName, err)
	}
	for _, n := range names {
		own[n] = true
	}
	return own, nil
}

// namesAFormerName reports whether snap carries a formerly: tag naming one of
// ownNames, which marks it as a backup of that entry made after a takeover.
func namesAFormerName(snap restic.Snapshot, ownNames map[string]bool) bool {
	for _, t := range snap.Tags {
		if n, ok := strings.CutPrefix(t, "formerly:"); ok && ownNames[n] {
			return true
		}
	}
	return false
}

// reposToCheckForTakeover returns the repositories an entry of domain
// ("container" or "vm") could read newName's snapshots from after a takeover:
// the one newName resolves to, and ownRepo, the entry's own, which the rename
// keeps. A stranger's history in either would become the entry's.
func (s *Service) reposToCheckForTakeover(settings store.Settings, domain, newName, ownRepo string) ([]string, error) {
	repoForName := s.containerRepoForName
	if domain == "vm" {
		repoForName = s.vmRepoForName
	}
	newRepo, err := repoForName(settings, newName, "")
	if err != nil {
		return nil, err
	}
	if ownRepo == newRepo {
		return []string{newRepo}, nil
	}
	return []string{newRepo, ownRepo}, nil
}

// refuseForeignBackups refuses to move entry targetID of domain from
// currentName onto newName while a backup under newName in repos or an
// off-site target is not the entry's, or while one of them cannot be read,
// since retention would then age a stranger's snapshots with the entry's. A
// backup naming one of the entry's names as a former name is its own, made
// before an unlink.
func (s *Service) refuseForeignBackups(ctx context.Context, settings store.Settings, domain, targetID, currentName, newName string, repos []string) error {
	settingsDomain, prefix, _ := aliasDomain(domain)
	own, err := s.ownNamesFor(domain, targetID, currentName)
	if err != nil {
		return fmt.Errorf("%q cannot be checked for backups: %w", newName, err)
	}
	places, err := s.backupPlaces(settings, settingsDomain, repos)
	if err != nil {
		return fmt.Errorf("%q cannot be checked for backups: %w", newName, err)
	}
	for _, p := range places {
		// The literal tag: an identity widened by aliases would pull in the
		// history of whatever row holds newName.
		snaps, err := s.snapshotsForTag(ctx, p.repo, p.mode, prefix+newName)
		if err != nil {
			return fmt.Errorf("%q cannot be checked for backups: %s could not be read: %w", newName, p.name, err)
		}
		for _, snap := range snaps {
			if namesAFormerName(snap, own) {
				continue
			}
			// Nothing on screen explains an orphaned backup, so the refusal says
			// whether an entry still owns the name.
			owner := "a different, unrelated entry"
			if _, err := s.entryIDByName(domain, newName); err == nil {
				owner = "its own entry"
			}
			return fmt.Errorf("%q already has backups in %s that belong to %s; delete them, then take over again", newName, p.name, owner)
		}
	}
	return nil
}

// moveDRDrillTargetTo repoints the DR drill from oldName to newName when it
// names oldName. The drill verifies by the literal container:<name> tag, so
// left alone it would keep checking an ever older snapshot under a name
// nothing answers to.
func (s *Service) moveDRDrillTargetTo(oldName, newName string) error {
	_, err := s.store.MutateSettings(func(cur *store.Settings) error {
		if cur.DRDrillTarget == oldName {
			cur.DRDrillTarget = newName
		}
		return nil
	})
	return err
}

// TakeOverContainer moves a not-installed entry onto the name its container was
// renamed to. Nothing in the repository is touched: the row keeps its id, so
// history and settings follow, and the old name becomes an alias the readers
// include. It is refused while the new name has backups that are not this
// entry's own, since adopting a stranger's history cannot be undone.
//
// Another entry's former name is refused on either side (takeoverAliasCheck).
// Onto the entry's own former name the takeover is a rename back: the store
// drops that alias and links the name the entry leaves, once the former name
// is shown to hold nothing from after its link (refuseTakeBackWhileNameReused).
//
// Both names are validated before anything is written, because the store does
// not validate them and the old name ends up in a formerly: tag on every later
// backup, which restic would split at a comma. The rewritten definition goes
// into the same transaction as the rename, so a corrupt stored definition
// fails the takeover before anything changes.
func (s *Service) TakeOverContainer(ctx context.Context, oldName, newName string) error {
	if oldName == newName {
		return errors.New("an entry cannot take over itself")
	}
	if !validResourceName(oldName) || !validResourceName(newName) {
		return errors.New("invalid container name")
	}
	// Locked before anything is checked: a backup or a restore finishing
	// between a lock-free check and the rename would slip past it.
	unlock, ok := s.tryLockDomainFor("containers", "takeover")
	if !ok {
		return errDomainBusy
	}
	defer unlock()
	live, err := s.installedContainers(ctx)
	if err != nil {
		return fmt.Errorf("list containers: %w", err)
	}
	if !live[newName] {
		return fmt.Errorf("container %q is not installed", newName)
	}
	if live[oldName] {
		return fmt.Errorf("container %q is installed again, so nothing was taken over", oldName)
	}
	// A restore from the entry would stop BombVault halfway.
	if self := s.selfContainerName(ctx); self != "" && newName == self {
		return fmt.Errorf("%q is BombVault's own container, so no entry can move onto it", newName)
	}
	oldTg, err := s.store.GetTargetByContainer(oldName)
	if err != nil {
		return fmt.Errorf("%q has no entry to take over", oldName)
	}
	settings, err := s.store.GetSettings()
	if err != nil {
		return fmt.Errorf("read settings: %w", err)
	}
	back, err := s.takeoverAliasCheck("container", oldTg.ID, oldName, newName)
	if err != nil {
		return err
	}
	ownRepo, err := s.containerRepoPath(settings, oldTg)
	if err != nil {
		return fmt.Errorf("resolve the entry's repository: %w", err)
	}
	repos, err := s.reposToCheckForTakeover(settings, "container", newName, ownRepo)
	if err != nil {
		return fmt.Errorf("%q cannot be checked for backups: %w", newName, err)
	}
	if back != nil {
		err = s.refuseTakeBackWhileNameReused(ctx, settings, *back, repos)
	} else {
		err = s.refuseForeignBackups(ctx, settings, "container", oldTg.ID, oldName, newName, repos)
	}
	if err != nil {
		return err
	}
	newDefinition, err := rewriteDefinitionJSON(oldTg.Definition, newName)
	if err != nil {
		return fmt.Errorf("rewrite definition for %q: %w", newName, err)
	}
	if existing, err := s.store.GetTargetByContainer(newName); err == nil {
		// RenameTargetWithAlias refuses an occupied name, so an empty row on it
		// is cleared first; one with backups or settings is kept.
		empty, labels, err := s.targetOccupyingNewNameIsEmpty(ctx, existing, oldName)
		if err != nil {
			return fmt.Errorf("check the existing entry of %q: %w", newName, err)
		}
		if !empty {
			if onlyACopyRule(labels) {
				return fmt.Errorf("%q: %w", newName, store.ErrCopyRuleTaken)
			}
			return fmt.Errorf("%q already has its own configured entry (%s), refusing to delete it; unlink or remove it yourself first", newName, strings.Join(labels, ", "))
		}
		if err := s.store.DeleteTarget(newName, live); err != nil {
			return fmt.Errorf("remove the empty entry of %q: %w", newName, err)
		}
	}
	// The store refuses such a rule too, but only after the mirrors below
	// have dropped their link records.
	if err := s.store.CheckCopyRuleMove("containers", "container:"+oldName, "container:"+newName); err != nil {
		return err
	}
	// Discover rebuilds a link only from the definition mirrors, so the name the
	// entry leaves stops recording links before the rename and the new name
	// records them after it.
	if err := s.dropLinkRecords("container", settings, oldName, ownRepo, oldTg.Definition); err != nil {
		return err
	}
	if err := s.store.RenameTargetWithAlias(oldName, newName, newDefinition); err != nil {
		return err
	}
	s.recordLinks("container", settings, oldTg.ID, newName, ownRepo, newDefinition)
	// Best-effort from here on: the rename, alias and definition are committed,
	// and other rows' stop lists and the DR drill in Settings cannot share that
	// transaction. A stop list still naming the old name stops nothing, which
	// is harmless.
	if err := s.rewriteStopLists(oldName, newName); err != nil {
		log.Printf("api: takeover %q -> %q succeeded, but rewriting sibling stop lists failed; some entries may still try to stop %q by its old name until fixed: %v", oldName, newName, oldName, err) //nolint:gosec // G706: both %q-quoted
	}
	if err := s.moveDRDrillTargetTo(oldName, newName); err != nil {
		log.Printf("api: takeover %q -> %q succeeded, but moving the DR-drill target failed; a drill may keep verifying the stale name %q until fixed: %v", oldName, newName, oldName, err) //nolint:gosec // G706: both %q-quoted
	}
	return nil
}

// takeoverAliasCheck refuses moving entry targetID from oldName to newName in
// domain ("container" or "vm") while either name is another entry's former
// name, because a former name and its older backups belong to one entry. When
// newName is the entry's own former name the move is a rename back, and that
// alias is returned.
func (s *Service) takeoverAliasCheck(domain, targetID, oldName, newName string) (*store.Alias, error) {
	_, _, kind := aliasDomain(domain)
	a, err := s.store.AliasByOldName(domain, oldName)
	switch {
	case err == nil:
		return nil, fmt.Errorf("%q is also a former name of %s, so its entry cannot move to %q; back up %q as a new entry instead", oldName, s.formerNameOwner(domain, a), newName, newName)
	case !errors.Is(err, sql.ErrNoRows):
		return nil, fmt.Errorf("read the former names: %w", err)
	}
	a, err = s.store.AliasByOldName(domain, newName)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return nil, nil
	case err != nil:
		return nil, fmt.Errorf("read the former names: %w", err)
	case a.TargetID == targetID:
		return &a, nil
	}
	return nil, fmt.Errorf("%q is a former name of %s and still holds its older backups; give the %s a name of its own, then take over", newName, s.formerNameOwner(domain, a), kind)
}

// formerNameOwner is how a message names the entry that alias a belongs to.
func (s *Service) formerNameOwner(domain string, a store.Alias) string {
	if name, err := s.entryNameByID(domain, a.TargetID); err == nil {
		return fmt.Sprintf("%q", name)
	}
	return "another entry"
}

// refuseTakeBackWhileNameReused refuses to rename an entry back onto one of its
// former names, back.OldName, while that name holds a backup from the link on
// in repos or an off-site target. Dropping the alias lifts the name's time
// bound, so this is the check unlink makes.
func (s *Service) refuseTakeBackWhileNameReused(ctx context.Context, settings store.Settings, back store.Alias, repos []string) error {
	_, _, kind := aliasDomain(back.Domain)
	reused, err := s.oldNameReused(ctx, settings, back, repos)
	if err != nil {
		return fmt.Errorf("%q cannot be taken back until newer backups under it can be ruled out: %w", back.OldName, err)
	}
	if reused {
		return fmt.Errorf("%q cannot be taken back: another %s has been backed up under that name since this entry left it. Delete those backups, then take over again", back.OldName, kind)
	}
	return nil
}

// UnlinkContainerAlias reverses a takeover: the entry goes back to its old
// name, the alias is removed and the stored definition is rewritten back, all
// in one transaction as in TakeOverContainer. It is refused when oldName is no
// alias, so a stale or mistyped name changes nothing, while another container
// is installed under oldName next to the entry's own, since the entry would
// move onto it, and while oldName holds another machine's backups from after
// the link (refuseUnlinkWhileOldNameReused).
func (s *Service) UnlinkContainerAlias(ctx context.Context, oldName string) error {
	if !validResourceName(oldName) {
		return errors.New("invalid container name")
	}
	alias, err := s.store.AliasByOldName("container", oldName)
	if err != nil {
		return fmt.Errorf("%q is not a taken-over name", oldName)
	}
	tg, err := s.store.GetTargetByID(alias.TargetID)
	if err != nil {
		return fmt.Errorf("read the linked entry: %w", err)
	}
	currentName := tg.ContainerName
	newDefinition, err := rewriteDefinitionJSON(tg.Definition, oldName)
	if err != nil {
		return fmt.Errorf("rewrite definition for %q: %w", oldName, err)
	}
	unlock, ok := s.tryLockDomainFor("containers", "unlink")
	if !ok {
		return errDomainBusy
	}
	defer unlock()
	// Under the lock, like the takeover gate: a backup of a machine under
	// oldName landing between these checks and the unlink would slip past them.
	live, err := s.installedContainers(ctx)
	if err != nil {
		return fmt.Errorf("%q stays linked until the installed containers can be listed: %w", oldName, err)
	}
	if live[oldName] && live[currentName] {
		return fmt.Errorf("%q stays linked: another container is installed under that name; rename that container first", oldName)
	}
	settings, err := s.store.GetSettings()
	if err != nil {
		return fmt.Errorf("read settings: %w", err)
	}
	ownRepo, err := s.containerRepoPath(settings, tg)
	if err != nil {
		return fmt.Errorf("resolve the linked entry's repository: %w", err)
	}
	if err := s.refuseUnlinkWhileOldNameReused(ctx, settings, alias, ownRepo); err != nil {
		return err
	}
	// The rule check and the definition mirrors follow as in TakeOverContainer.
	if err := s.store.CheckCopyRuleMove("containers", "container:"+currentName, "container:"+oldName); err != nil {
		return err
	}
	if err := s.dropLinkRecords("container", settings, currentName, ownRepo, tg.Definition); err != nil {
		return err
	}
	if err := s.store.UnlinkAlias(oldName, newDefinition, live); err != nil {
		return err
	}
	s.relistAfterUnlink("containers", "container:"+currentName)
	s.recordLinks("container", settings, tg.ID, oldName, ownRepo, newDefinition)
	// Best-effort, as in TakeOverContainer.
	if err := s.rewriteStopLists(currentName, oldName); err != nil {
		log.Printf("api: unlink %q succeeded, but rewriting sibling stop lists failed; some entries may still try to stop %q by its former name until fixed: %v", oldName, currentName, err) //nolint:gosec // G706: both %q-quoted
	}
	if err := s.moveDRDrillTargetTo(currentName, oldName); err != nil {
		log.Printf("api: unlink %q succeeded, but moving the DR-drill target back failed; a drill may keep verifying the stale name %q until fixed: %v", oldName, currentName, err) //nolint:gosec // G706: both %q-quoted
	}
	return nil
}

// relistAfterUnlink lists again, in the background, each target that counted
// copies under identity, the name an entry has just left. Some of them are the
// entry's again and some stay with the name, and only a listing tells which.
func (s *Service) relistAfterUnlink(domain, identity string) {
	held, err := s.store.ItemCopiesFor(domain, identity)
	if err != nil {
		log.Printf("api: unlink: the copies counted under %q stay there until the next replication: %v", identity, err) //nolint:gosec // G706: identity is %q-quoted
		return
	}
	for _, c := range held {
		s.listTargetInBackground(domain, c.TargetID)
	}
}

// refuseUnlinkWhileOldNameReused refuses to unlink a while its old name holds
// a snapshot a does not claim in ownRepo or any off-site target, or while one
// of them cannot be read. Unlinking lifts the name's time bound, so a
// later machine's backups under it would become the entry's own; the way out
// is renaming that machine or deleting its backups.
func (s *Service) refuseUnlinkWhileOldNameReused(ctx context.Context, settings store.Settings, a store.Alias, ownRepo string) error {
	_, _, kind := aliasDomain(a.Domain)
	reused, err := s.oldNameReused(ctx, settings, a, []string{ownRepo})
	if err != nil {
		return fmt.Errorf("%q stays linked until newer backups under that name can be ruled out: %w", a.OldName, err)
	}
	if reused {
		return fmt.Errorf("%q stays linked: another %s has been backed up under that name since it was linked here, and unlinking would make those backups this entry's. Rename that %s, or delete its backups, then unlink", a.OldName, kind, kind)
	}
	return nil
}

// aliasDomain maps an alias domain ("container" or "vm") to its settings
// domain, its identity tag prefix and the word a message uses for it.
func aliasDomain(domain string) (settingsDomain, prefix, kind string) {
	if domain == "vm" {
		return "vms", "vm:", "VM"
	}
	return "containers", "container:", "container"
}

// oldNameReused reports whether a's old name holds a snapshot a does not
// claim, one taken at or after the link or at a time that does not parse, in
// repos or in any off-site target of the alias's domain. The error names the
// place that could not be read.
func (s *Service) oldNameReused(ctx context.Context, settings store.Settings, a store.Alias, repos []string) (bool, error) {
	domain, prefix, _ := aliasDomain(a.Domain)
	places, err := s.backupPlaces(settings, domain, repos)
	if err != nil {
		return false, err
	}
	claim := newAliasClaim(prefix, a)
	for _, p := range places {
		snaps, err := s.snapshotsForTag(ctx, p.repo, p.mode, claim.tag)
		if err != nil {
			return false, fmt.Errorf("%s could not be read: %w", p.name, err)
		}
		if !claim.claimsEvery(snaps) {
			return true, nil
		}
	}
	return false, nil
}

// backupPlace is a repository an entry's snapshots may sit in, with the name
// a message gives it.
type backupPlace struct {
	name string
	repo string
	mode restic.Mode
}

// backupPlaces is repos plus every off-site target domain replicates to. An
// off-site target list that cannot be read is an error, because a check that
// skips a copy it cannot see passes on nothing.
func (s *Service) backupPlaces(settings store.Settings, domain string, repos []string) ([]backupPlace, error) {
	places := make([]backupPlace, 0, len(repos)+1)
	for _, repo := range repos {
		places = append(places, backupPlace{"the repository (" + shortRepoName(repo) + ")", repo, s.repoModeFor(settings, domain, "", repo)})
	}
	targets, err := s.enabledOffsiteTargets(domain)
	if err != nil {
		return nil, fmt.Errorf("the off-site target list could not be read: %w", err)
	}
	for _, t := range orSettingsOffsiteTarget(targets, domain, settings) {
		repo, err := s.resolveRepo(t.Repo)
		if err != nil {
			return nil, fmt.Errorf("off-site target %q could not be resolved: %w", t.Name, err)
		}
		places = append(places, backupPlace{fmt.Sprintf("off-site target %q", t.Name), repo, s.offsiteModeForTarget(settings, t)})
	}
	return places, nil
}

// SetInclude sets the include_in_schedule flag for a container, creating the
// target row first if it does not exist yet (the first backup has not run).
// It inspects the container to resolve appdata paths exactly like Backup does,
// so the target is fully populated from the start. If docker inspect fails the
// operation is still completed: a placeholder target is upserted with a
// conventional appdata path so the toggle is never silently lost.
func (s *Service) SetInclude(ctx context.Context, name string, include bool) error {
	if _, err := s.store.GetTargetByContainer(name); err != nil {
		// Target does not exist yet: find or create it before calling SetInclude.
		var appdata []string
		if in, inspErr := s.docker.Inspect(ctx, name); inspErr == nil {
			appdata = s.resolveAppdataPaths(name, in)
		} else {
			log.Printf("api: SetInclude: inspect %q failed (checking fallback path): %v", name, inspErr) //nolint:gosec // G706: name is %q-quoted; no raw user bytes reach the log formatter
			// Fall back to the conventional appdata dir, but only if it exists on
			// disk (the same os.Stat guard as resolveAppdataPaths). A phantom
			// placeholder would show as a selected folder that backs up nothing
			// (#115); leave AppdataPaths empty (definition-only) until the user points
			// it at a real folder.
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

// SetScheduleCadence sets a container's per-item schedule override (#121).
// It finds or creates the target row (like SetInclude) so an override can
// be set before the first backup. The cadence is validated with the
// grammar the domain schedules use; an empty string clears the override
// (back to the domain default). everyN is rejected because a per-item entry
// has no per-item last-run gate to enforce the interval, the same
// restriction the off-site and drill schedules carry.
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
		// Target does not exist yet: find or create it (same path as SetInclude).
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

// SetIncludeAll sets the include_in_schedule flag for every installed
// container in one call, the one-click "include all in schedule" and
// "exclude all" action. It iterates the installed-container source the
// containers list uses (docker.List) and ensures a target row exists for
// each (find or create, as SetInclude does), so the flag is never silently
// lost on a container that has not been backed up yet. BombVault's own
// container is skipped: it can never be backed up (ErrSelfBackup), so
// scheduling it would only add a failing job. A single container's
// inspect or upsert failure aborts the batch with that error rather than
// leaving a partial, ambiguous result.
//
// Excluding also reaches every target whose container is gone from the
// host (#232). Such a row stays scheduled until somebody switches it off,
// and every run records a skip for it, so "Exclude all" has to end that
// too. Including never reaches them: it would put every removed container
// straight back into the skip loop.
func (s *Service) SetIncludeAll(ctx context.Context, include bool) error {
	infos, err := s.docker.List(ctx)
	if err != nil {
		return fmt.Errorf("list containers: %w", err)
	}
	self := s.selfContainerName(ctx)
	live := make(map[string]bool, len(infos))
	for _, c := range infos {
		live[c.Name] = true
		if self != "" && c.Name == self {
			continue // never schedule BombVault's own container
		}
		if err := s.SetInclude(ctx, c.Name, include); err != nil {
			return err
		}
	}
	if include {
		return nil
	}
	targets, err := s.store.ListTargets()
	if err != nil {
		return fmt.Errorf("list targets: %w", err)
	}
	for _, t := range targets {
		if live[t.ContainerName] || !t.IncludeInSchedule {
			continue
		}
		if err := s.store.SetInclude(t.ContainerName, false); err != nil {
			return err
		}
	}
	return nil
}

// ContainerPath returns the resolved absolute containers backup path, used
// by the path-writable probe. Returns "" if it cannot be resolved.
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

// resticAdapter wraps a ResticEngine + Mode to satisfy backup.Restic, converting
// the engine's float64 BytesAdded to the orchestrator's int64 Bytes.
type resticAdapter struct {
	engine ResticEngine
	mode   restic.Mode
	// extraTags go on every snapshot the adapter writes, after the
	// orchestrator's own.
	extraTags []string
}

var _ backup.Restic = (*resticAdapter)(nil)

func (a *resticAdapter) Backup(ctx context.Context, repo string, paths, tags []string, excludes ...string) (backup.Summary, error) {
	sum, err := a.engine.Backup(ctx, repo, paths, withTags(tags, a.extraTags), a.mode, excludes...)
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
		// An ambiguous short id would fail in restic after the destructive
		// teardown, so reject it now, before anything is stopped or destroyed.
		return fmt.Errorf("snapshot id %s is ambiguous (matches %d snapshots)", snapshotID, prefixMatches)
	}
}

// templatesAdapter satisfies backup.Templates over the template package funcs.
type templatesAdapter struct{}

var _ backup.Templates = templatesAdapter{}

func (templatesAdapter) Read(dir, name string) (string, bool, error) { return template.Read(dir, name) }
func (templatesAdapter) Write(dir, name, xml string) error           { return template.Write(dir, name, xml) }

// runsAdapter satisfies backup.Runs over *store.Repo (StartRun/FinishRun).
// ctx is captured at construction so Start can read runGroupFromContext
// and stamp a "Backup Everything" pass's parent run id onto the child run
// it just created (see runGroupKey); a ctx without a group changes nothing.
type runsAdapter struct {
	st  *store.Repo
	ctx context.Context
	// svc is read only to ask whether the process is shutting down and whether
	// a user cancelled this particular backup (#200). It is optional: the
	// bookkeeping-only call sites below pass nil, which costs only the
	// shutdown relabel, and those sites are not the run a backup finishes on.
	svc *Service
	// cancelKey is this run's progress key ("files:<id>", "container:<name>",
	// "vm:<name>", "flash"), set only by the call sites that also register a
	// backup cancel func under it. Empty everywhere else, which is what makes
	// the user-cancellation relabel below reach exactly the runs a user can
	// actually cancel and no others.
	cancelKey string
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

// shutdownStatus rewrites a failure that is really a shutdown.
//
// The orchestrator has no idea why its context died; it sees a cancelled
// context and reports "failed", which is right for every other cause. On
// the way out of the process it is wrong where it matters most: a red row
// with a context error looks like a backup that genuinely broke, the
// dashboard counts it, the notification fires for it, and somebody goes
// looking for a fault that was a `docker stop`.
//
// This is the one place with both facts in hand, that the run failed and
// that the process is leaving, so it is the one place that can tell the
// difference. The "cancelled" status already exists and fires no failure
// alert.
//
// It is narrow: it only downgrades a failed run, and only while shutting
// down, so a real failure that lands in the same second keeps its status.
// Errors are not inspected for cancellation: once BeginShutdown has
// cancelled the context, every failure from that run is downstream of it,
// and matching on error text would be a weaker second guess at something
// the flag already knows.
func (s *Service) shutdownStatus(status string) (string, string, bool) {
	if status != "failed" || !s.shuttingDown.Load() {
		return status, "", false
	}
	return "cancelled", store.ReasonShutdown, true
}

func (r runsAdapter) Finish(runID, status, snapshotID string, bytes int64, errMsg string) error {
	if r.svc != nil {
		if newStatus, newMsg, changed := r.svc.shutdownStatus(status); changed {
			status, errMsg = newStatus, newMsg
		} else if status == "failed" && r.svc.backupWasCancelled(r.cancelKey) {
			// A user cancelled this backup (#200). The error in hand is the context
			// cancellation that followed, and recording it as a failure would put a
			// red row in Run History, count it on the dashboard and fire an alert for
			// something somebody asked for.
			//
			// As narrow as the shutdown relabel above, for the same reason: it only
			// downgrades a failed run, and only one whose key was marked. A real
			// failure in a backup nobody cancelled keeps its status, and the error
			// text is not consulted.
			status, errMsg = "cancelled", store.ReasonCancelled
		}
	}
	return r.st.FinishRun(runID, status, snapshotID, bytes, errMsg)
}

// startedRunsAdapter satisfies backup.Runs like runsAdapter, except Start
// returns a run id obtained earlier instead of starting a fresh run.
// BackupVM needs the run id before VMBackupDeps is built, to set RunTag =
// "vmrun:<runID>"; RunTag drives every restic tag the orchestrator builds
// (see VMBackupDeps.RunTag), so it cannot wait for the orchestrator's own
// Runs.Start call. BackupVM calls store.StartRun itself and wraps the
// result here, so the orchestrator's Start is a read rather than a second,
// orphaned run row; Finish delegates to the real store like runsAdapter.
type startedRunsAdapter struct {
	st    *store.Repo
	runID string
	// svc plays the same role as in runsAdapter: a VM backup is a backup, so a
	// shutdown mid-run must read as an abort here too; the two adapters differ
	// only in where the run id comes from.
	svc *Service
	// cancelKey: same role as runsAdapter.cancelKey (#200), and it has to be
	// here for the same reason the line above gives. A VM backup registers
	// "vm:<name>" like every other domain registers its own key, so a user who
	// cancels one must get the same "cancelled" row a cancelled folder backup
	// gets, not a red failure.
	cancelKey string
}

var _ backup.Runs = startedRunsAdapter{}

func (r startedRunsAdapter) Start(string, string) (string, error) { return r.runID, nil }

func (r startedRunsAdapter) Finish(runID, status, snapshotID string, bytes int64, errMsg string) error {
	if r.svc != nil {
		if newStatus, newMsg, changed := r.svc.shutdownStatus(status); changed {
			status, errMsg = newStatus, newMsg
		} else if status == "failed" && r.svc.backupWasCancelled(r.cancelKey) {
			// See runsAdapter.Finish (#200). Repeated rather than shared because the
			// two adapters are separate types, and a helper taking (svc, status, key)
			// would be indirection over three lines of condition.
			status, errMsg = "cancelled", store.ReasonCancelled
		}
	}
	return r.st.FinishRun(runID, status, snapshotID, bytes, errMsg)
}

// sshZFSHost adapts HostSSH's Run/StreamCommand/RunWithStdin surface into
// backup.ZFSHost by running the zfs command lines
// virshcli.ZFSSnapshotArgs/ZFSSnapshotDestroyArgs/ZFSSendArgs/ZFSReceiveArgs
// build over SSH, the way virshcli runs virsh commands over the same
// transport, so the container needs no local ZFS toolchain.
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

// resticZvolAdapter wraps a ResticEngine and Mode to satisfy
// backup.ZvolRestic for the zvol stdin backup and restore path, with the
// same float64 BytesAdded to int64 Bytes conversion as resticAdapter. It is
// its own small type because backup.ZvolRestic is separate from
// backup.Restic (file-backed disk backup and restore never need stdin
// streaming).
type resticZvolAdapter struct {
	engine ResticEngine
	mode   restic.Mode
	// extraTags go on every snapshot the adapter writes, after the
	// orchestrator's own.
	extraTags []string
}

var _ backup.ZvolRestic = (*resticZvolAdapter)(nil)

func (a *resticZvolAdapter) BackupStdin(ctx context.Context, repo string, rd io.Reader, path string, tags []string) (backup.Summary, error) {
	sum, err := a.engine.BackupStdin(ctx, repo, rd, path, withTags(tags, a.extraTags), a.mode)
	if err != nil {
		return backup.Summary{}, err
	}
	return backup.Summary{SnapshotID: sum.SnapshotID, Bytes: int64(sum.BytesAdded)}, nil
}

func (a *resticZvolAdapter) DumpTo(ctx context.Context, repo, snapshotID, path string, w io.Writer) error {
	return a.engine.DumpRaw(ctx, repo, snapshotID, path, w, a.mode)
}

// vmDefinition is the recreate recipe persisted at VM backup time so restore
// works even after the VM has been deleted or BombVault's /config is lost
// (full DR). It carries container-visible paths so the restore orchestrator
// can pass them directly to restic.
type vmDefinition struct {
	DomainXML string   `json:"domain_xml"`
	DiskPaths []string `json:"disk_paths"` // container-visible absolute paths (under the Host Data mount)
	// NVRAM travels in the definition (read and written over SSH), not via a
	// libvirt mount. NVRAMHostPath is the host path from the domain XML;
	// NVRAMBytes is the captured var store (base64 in JSON). Empty for BIOS VMs
	// or when SSH capture failed; EnsureNVRAMTemplate then regenerates it on
	// restore.
	NVRAMHostPath string `json:"nvram_host_path"`
	NVRAMBytes    []byte `json:"nvram_bytes,omitempty"`
	// TPMBytes is the captured vTPM state, read and written over SSH like
	// NVRAMBytes (see BackupVM's TPM read and prepareRestoreVMForTarget's
	// PreDefine write-back). Unlike NVRAM, the TPM's host path is not stored:
	// it is re-derived from DomainXML (virshcli.ParseDomain's TPMPath) at
	// backup and restore time, since it is a stable property of the domain
	// definition that restore never remaps (destBase's cross-instance remap
	// only rewrites file-disk sources and NVRAM; see
	// virshcli.RewriteDiskSources/RewriteNVRAM). Empty for a VM without vTPM
	// (DomainInfo.TPMPath == "") or when SSH capture failed; a read failure is
	// non-fatal, like NVRAM's.
	TPMBytes     []byte `json:"tpm_bytes,omitempty"`
	Method       string `json:"method"`
	WasAutostart bool   `json:"was_autostart"`
	// WasRunning is the VM's run state at backup time. A pointer, so a backup
	// without the field reads as nil (unknown) and restore boots the VM. A
	// non-nil value is honoured so restore mirrors the captured state, as for
	// containers.
	WasRunning *bool `json:"was_running,omitempty"`
	// Aliases are the links the mirror records, as in containerDefinition.
	Aliases []definitionAlias `json:"aliases,omitempty"`
}

// VMView is the per-VM row returned by ListVMs.
type VMView struct {
	Name string `json:"name"`
	// LibvirtName is the raw libvirt domain name (vm.Name, never
	// vm.FriendlyName) on every platform. It equals Name everywhere except
	// TrueNAS, where Name is the presentation-only friendly name (see the
	// isTrueNAS block in ListVMs). It is the only field the frontend may send
	// back on a VM action call (backup, restore, snapshots, forget, method,
	// include, scheduleCadence, backup-order, DR-drill-target): every such
	// route (see vmNameParam in handlers.go) hands the path segment straight
	// to virsh without resolution. Name is display-only; sent as an identifier
	// on TrueNAS it would target a domain name virsh has never heard of.
	LibvirtName       string `json:"libvirtName"`
	State             string `json:"state"`
	Method            string `json:"method"`
	IncludeInSchedule bool   `json:"includeInSchedule"`
	LastBackup        *int64 `json:"lastBackup"`
	LastBackupStarted *int64 `json:"lastBackupStarted"`
	// ScheduleCadence is the VM's optional per-item schedule override (#121); ""
	// means it follows the VMs domain schedule. Only takes effect when the
	// perItemSchedules setting is on.
	ScheduleCadence string        `json:"scheduleCadence"`
	Placement       placementView `json:"placement"`
	// RenameFrom and RenameReason suggest a rename: they are set on a live VM
	// with no backups of its own whose libvirt UUID matches a not-installed
	// entry's. They match containerView's fields so the frontend treats both
	// alike.
	RenameFrom   string `json:"renameFrom"`
	RenameReason string `json:"renameReason"`
	// AliasConflicts are the former names of this entry that are live domains
	// again, alphabetically, as in containerView.
	AliasConflicts []string `json:"aliasConflicts"`
	// Aliases are the libvirt names this entry had before, oldest link first.
	Aliases []string `json:"aliases"`
}

// vmUUID returns tg's libvirt UUID. An empty column is filled from the saved
// domain XML and stored, because the migration that added the column cannot
// parse XML. A definition that yields no UUID returns "" and writes nothing.
func (s *Service) vmUUID(tg store.VMTarget) string {
	if tg.UUID != "" {
		return tg.UUID
	}
	uuid := definitionUUID(tg.Definition)
	if uuid == "" {
		return ""
	}
	if err := s.store.SetVMUUID(tg.Name, uuid); err != nil {
		log.Printf("api: vmUUID: backfill %q: %v", tg.Name, err) //nolint:gosec // G706: %q-quoted
	}
	return uuid
}

// definitionUUID is the libvirt UUID in the domain XML of a stored VM
// definition, "" when it names none or does not parse.
func definitionUUID(definition string) string {
	var def vmDefinition
	if err := json.Unmarshal([]byte(definition), &def); err != nil {
		return ""
	}
	info, err := virshcli.ParseDomain(def.DomainXML)
	if err != nil {
		return ""
	}
	return info.UUID
}

// ListVMs returns all known VMs (from virsh) merged with the DB targets.
// VMs with no virsh entry but with backup history appear as state="not-installed".
func (s *Service) ListVMs(ctx context.Context) ([]VMView, error) {
	// Only reach libvirt over SSH when the VMs domain is enabled. The dashboard
	// calls this on every GUI load, and for users who don't back up VMs at all
	// an unconditional virsh-over-SSH connect would spam the container log
	// with "could not resolve hostname / connection reset" errors. Stored VM
	// targets are still listed (as orphans); only the live enumeration is
	// skipped.
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

	// displayName is the raw libvirt name on every platform except TrueNAS,
	// where it is the presentation-only virshcli.VMInfo.FriendlyName (the bare
	// UUID libvirt uses for domain names on TrueNAS 26, resolved to something
	// readable). Gated explicitly on Kind() because of the gotcha documented
	// on FriendlyName (internal/virshcli/types.go): the classifier that
	// populates it is shape-based, not platform-gated, so a non-TrueNAS host
	// must never trust it even when it differs from Name. vm.Name, never
	// FriendlyName, is still used below for the byName lookup and stays the
	// identifier everywhere else in this file.
	isTrueNAS := s.platformFn().Kind() == platform.KindTrueNAS

	live := make(map[string]bool, len(infos))
	for _, vm := range infos {
		live[vm.Name] = true
	}

	// A failed alias read only drops the conflict warnings and aliases: a
	// missing warning does less harm than a VM list that does not load.
	var formerNames, aliasConflicts aliasIndex
	if aliases, aErr := s.store.ListAliases("vm"); aErr != nil {
		log.Printf("api: list vms: alias conflict check: %v", aErr)
	} else {
		formerNames = newAliasIndex(aliases)
		aliasConflicts = liveFormerNames(aliases, live)
	}

	var orphanTargets []store.VMTarget
	for _, t := range targets {
		if !live[t.Name] {
			orphanTargets = append(orphanTargets, t)
		}
	}

	// One listing dates every row and tells the rename pass whether a live VM
	// has backups under its own name; a failed read keeps that pass from
	// guessing, as on the container list. With no VM and no entry there is
	// nothing to date.
	var snapTimes map[string]int64
	snapTimesFailed := false
	if len(infos) > 0 || len(targets) > 0 {
		if m, sErr := s.LatestVMBackupTimes(ctx); sErr != nil {
			log.Printf("api: list vms: latest backup times: %v", sErr)
			snapTimesFailed = true
		} else {
			snapTimes = m
		}
	}

	views := make([]VMView, 0, len(infos)+len(targets))
	viewIndex := make(map[string]int, len(infos)) // live rows only
	hasOwnBackup := make(map[string]bool, len(infos))
	needsRenameSuggestion := false
	for _, vm := range infos {
		displayName := vm.Name
		if isTrueNAS {
			displayName = vm.FriendlyName
		}
		v := VMView{Name: displayName, LibvirtName: vm.Name, State: vm.State, Method: "graceful", AliasConflicts: []string{}, Aliases: []string{}}
		var run *store.Run
		if t, ok := byName[vm.Name]; ok {
			v.AliasConflicts = aliasConflicts.of(t.ID)
			v.Aliases = formerNames.of(t.ID)
			v.Method = t.Method
			v.IncludeInSchedule = t.IncludeInSchedule
			v.ScheduleCadence = t.ScheduleCadence
			run, _ = s.store.LastSuccessfulBackup(t.ID)
		}
		v.LastBackup, v.LastBackupStarted = lastBackupDate(vm.Name, run, snapTimes, snapTimesFailed)
		own := v.LastBackup != nil
		hasOwnBackup[vm.Name] = own
		if !own {
			needsRenameSuggestion = true
		}
		viewIndex[vm.Name] = len(views)
		views = append(views, v)
	}

	// The match runs over every live domain to keep it one-to-one, so a VM
	// with backups of its own can still come back matched and is skipped here.
	for liveName, cand := range s.suggestVMRenames(ctx, infos, orphanTargets, needsRenameSuggestion && !snapTimesFailed) {
		if hasOwnBackup[liveName] {
			continue
		}
		if idx, ok := viewIndex[liveName]; ok {
			views[idx].RenameFrom = cand.OldName
			views[idx].RenameReason = cand.Reason
		}
	}

	// Orphans: targets whose VM is not defined on the host.
	for _, t := range orphanTargets {
		v := VMView{Name: t.Name, LibvirtName: t.Name, State: "not-installed", Method: t.Method, IncludeInSchedule: t.IncludeInSchedule, ScheduleCadence: t.ScheduleCadence, AliasConflicts: aliasConflicts.of(t.ID), Aliases: formerNames.of(t.ID)}
		run, _ := s.store.LastSuccessfulBackup(t.ID)
		v.LastBackup, v.LastBackupStarted = lastBackupDate(t.Name, run, snapTimes, snapTimesFailed)
		views = append(views, v)
	}
	return views, nil
}

// leftoverOverlayDevices returns the target devices of any writable disk
// whose source is a leftover BombVault live-snapshot overlay (a
// "*.bombvault-tmp" file) from an interrupted live backup. Such an overlay
// blocks the next snapshot ("…already exists…") and, left in place, would
// make a backup capture only the overlay and not its base disk. Matching
// on BombVault's own snapshot name is unambiguous: never a cdrom or a
// user's manual snapshot.
func leftoverOverlayDevices(d virshcli.DomainInfo) []string {
	// libvirt names a snapshot-create-as overlay "<base>.<snapname>", so a
	// BombVault leftover is exactly a "*.bombvault-tmp" file. Match the suffix,
	// not a bare substring, so a disk whose path merely contains the name is
	// not hit.
	suffix := "." + backup.LiveSnapshotName
	var devs []string
	for _, disk := range d.Disks {
		if strings.HasSuffix(disk.Source, suffix) {
			devs = append(devs, disk.Dev)
		}
	}
	return devs
}

// recoverLeftoverOverlay commits a leftover BombVault snapshot overlay back
// into its base before a backup, so the VM is on a clean disk chain (live
// snapshots work again and the backup captures the real base, not just the
// overlay). It only ever commits a disk whose source is BombVault's own
// "*.bombvault-tmp". It returns the refreshed domain XML and parsed info; a
// domain without a leftover is returned unchanged. The VM must be running
// to active-commit; a shut-off VM with a leftover is an error the user must
// resolve, since it is never started silently.
func (s *Service) recoverLeftoverOverlay(ctx context.Context, name, xmlStr string, domain virshcli.DomainInfo) (string, virshcli.DomainInfo, error) {
	devs := leftoverOverlayDevices(domain)
	if len(devs) == 0 {
		return xmlStr, domain, nil
	}
	// Must be running to active-commit. The check error is not swallowed: a
	// flaky host must not be misread as "shut off", which would send a
	// confusing message and could mask a real fault.
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
	// Re-read the now-clean domain so the backup reads the real base disk, not the overlay.
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

// removeStrayOverlays deletes leftover BombVault live-snapshot overlay
// files ("*.bombvault-tmp") next to the VM's base disks. blockcommit
// --active --pivot merges an overlay back into its base and switches the VM
// onto the base, but does not delete the orphaned overlay file, so without
// this every successful live backup would leave one behind and the next
// snapshot would fail with "external snapshot file ... already exists".
// The caller must make sure the VM is on its base disks first (after
// recovery or commit), so these files are never in use. Best-effort:
// failures are logged, never fatal.
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

// failVMBackup makes a pre-orchestrator VM backup failure visible: it
// records a failed run against the VM's existing target (so it shows in the
// dashboard run history) and fires a notification. It covers failures
// before the orchestrator starts its own run (overlay recovery, the
// running-state check), so a destructive or aborted attempt is never
// silent, least of all for scheduled backups, where nobody sees the HTTP
// error. Best-effort: bookkeeping errors are ignored, since the real cause
// is already returned to the caller.
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

// vmDiskContainerPaths is where restic reads the file disks of domain through
// the Host Data mount, the paths a VM definition stores. A domain with no file
// disk, or with one outside the mount, is an error: a snapshot of it would
// restore nothing.
func (s *Service) vmDiskContainerPaths(name string, domain virshcli.DomainInfo) ([]string, error) {
	if len(domain.DiskPaths) == 0 {
		return nil, fmt.Errorf("no disk paths found in domain XML for %q", name)
	}
	diskPaths := make([]string, 0, len(domain.DiskPaths))
	for _, hp := range domain.DiskPaths {
		cp, ok := s.toContainerPath(hp)
		if !ok {
			return nil, fmt.Errorf("disk %q is not under the host mount and can't be reached for backup. The VM disk must live under your Host Data mount (/mnt)", hp)
		}
		diskPaths = append(diskPaths, cp)
	}
	return diskPaths, nil
}

// BackupVM orchestrates a full VM backup: resolve repo and mode, ensure the
// repo, dump the XML, parse the domain, translate paths, upsert the VM
// target and run the orchestrator.
func (s *Service) BackupVM(ctx context.Context, name string) (_ backup.Summary, retErr error) {
	// Survive the client that triggered it disconnecting (see Backup): detach from
	// the request's cancellation with a generous hard cap.
	ctx, cancel := backupHoldCtx(ctx)
	defer cancel()
	s.registerBackupCancel("vm:"+name, cancel) // reachable by shutdown
	defer s.unregisterBackupCancel("vm:" + name)
	defer s.lockDomain("vms")() // serialise per repo; blocks maintenance ops meanwhile

	// Everything down to the orchestrator returns before any run is recorded,
	// so a failure there would leave the card that started this backup waiting
	// for one. Record it as Backup does; a VM that is not defined on the host
	// is a skip, not a failure.
	var targetID string
	if tg, tErr := s.store.GetVMTargetByName(name); tErr == nil {
		targetID = tg.ID
	}
	orchestrated := false
	defer func() {
		if retErr != nil && !orchestrated && !errors.Is(retErr, backup.ErrVMNotInstalled) {
			s.recordPreflightFailure("BackupVM", name, targetID, retErr)
		}
	}()
	settings, err := s.store.GetSettings()
	if err != nil {
		return backup.Summary{}, fmt.Errorf("read settings: %w", err)
	}
	item := store.ItemRef{Domain: "vms", Key: name}
	step, err := s.prepareHome(ctx, settings, item)
	if err != nil {
		return backup.Summary{}, err
	}

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
		// info log and a sentinel instead of failing: the scheduler treats
		// ErrVMNotInstalled as a skip, so the nightly job neither errors nor
		// spams. Returns before any run is recorded or failure notification is
		// sent.
		if virshcli.IsNotFound(err) {
			log.Printf("api: BackupVM: skipping %q: not defined on the host (not installed; backups only)", name) //nolint:gosec // G706: name is %q-quoted
			return backup.Summary{}, backup.ErrVMNotInstalled
		}
		return backup.Summary{}, fmt.Errorf("backup vm: dumpxml: %w", err)
	}
	domain, err := virshcli.ParseDomain(xmlStr)
	if err != nil {
		return backup.Summary{}, fmt.Errorf("backup vm: parse domain: %w", err)
	}

	// If the VM is still on a leftover BombVault snapshot overlay from an
	// interrupted live backup, commit it back first so live snapshots work
	// again and the backup reads the real base disk, not just the overlay.
	// No-op otherwise.
	xmlStr, domain, err = s.recoverLeftoverOverlay(ctx, name, xmlStr, domain)
	if err != nil {
		s.failVMBackup(ctx, name, err) // attempted or needed a destructive commit; don't fail silently
		return backup.Summary{}, err
	}

	diskPaths, err := s.vmDiskContainerPaths(name, domain)
	if err != nil {
		return backup.Summary{}, fmt.Errorf("backup vm: %w", err)
	}

	// The VM is now guaranteed on its base disks (recoverLeftoverOverlay committed
	// any overlay). Delete stray "*.bombvault-tmp" overlay files left behind by a
	// previous live backup, otherwise the next snapshot-create fails "already
	// exists". This recovers a VM already stuck in that state.
	s.removeStrayOverlays(diskPaths)

	// NVRAM (the UEFI var store) lives under /etc/libvirt on the host. Read it
	// over SSH and keep it in the definition (no mount, no restic staging). On
	// restore it is written back over SSH; if it is missing,
	// EnsureNVRAMTemplate regenerates it from the OVMF master. A read failure
	// is non-fatal.
	var nvramBytes []byte
	if domain.NVRAMPath != "" && s.ssh != nil {
		if b, rerr := s.ssh.ReadFile(ctx, domain.NVRAMPath); rerr == nil {
			nvramBytes = b
		} else {
			log.Printf("api: BackupVM: WARN NVRAM read over SSH failed for %q (%v); the disks are backed up, but on restore the UEFI variables (boot entries) will be regenerated from the firmware template, not restored", name, rerr) //nolint:gosec // G706: name is %q-quoted
		}
	}

	// vTPM state is captured like NVRAM above: read over SSH from
	// domain.TPMPath (parsed by ParseDomain) and kept in the definition.
	// Skipped when TPMPath is "" (no vTPM on this domain, or an unrecognized
	// <tpm> backend shape; see virshcli.DomainInfo.TPMPath). A read failure is
	// non-fatal.
	var tpmBytes []byte
	if domain.TPMPath != "" && s.ssh != nil {
		if b, rerr := s.ssh.ReadFile(ctx, domain.TPMPath); rerr == nil {
			tpmBytes = b
		} else {
			log.Printf("api: BackupVM: WARN TPM state read over SSH failed for %q (%v); the disks are backed up, but on restore the vTPM will start fresh/empty, not restore its captured state", name, rerr) //nolint:gosec // G706: name is %q-quoted
		}
	}

	// Block-device (zvol) disks go through a separate backup mechanism
	// (BackupZvolDisk, wired via deps.BlockDisks, ZFSHost and ZvolRestic
	// below), since restic cannot back up a raw block device by path. Each
	// disk's ZFS dataset is resolved from its /dev/zvol/<pool>/<dataset> source
	// path. As with a disk outside the host mount above, a block device this
	// mechanism cannot reach fails the whole backup rather than silently
	// producing an incomplete one.
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

	// Default autostart to true (safe: most Unraid-managed VMs have autostart
	// on).
	// TODO: read the real flag from virsh dominfo.
	wasAutostart := true

	// Get method from existing target (default graceful).
	method := "graceful"
	if existing, tErr := s.store.GetVMTargetByName(name); tErr == nil {
		method = existing.Method
	}

	// Store the persistent (inactive) definition for restore so a live-snapshot
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

	if step, err = s.recordHome(ctx, settings, item, step); err != nil {
		return backup.Summary{}, err
	}
	repo, mode := step.repo, step.mode

	tg, err := s.store.UpsertVMTarget(store.VMTarget{
		Name: name, Method: method, Definition: string(defBytes), UUID: domain.UUID,
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

	// As in Backup, an alias read failure costs this one backup its formerly:
	// tags and the mirror update, not the backup itself.
	aliases, aliasErr := s.store.TargetAliasesWithDefinitions("vm", tg.ID)
	if aliasErr != nil {
		log.Printf("api: backup vm: aliases of %q: %v", name, aliasErr) //nolint:gosec // G706: %q-quoted
	}

	deps := backup.VMBackupDeps{
		Name:             name,
		FormerNames:      aliasOldNames(aliases),
		DiskPaths:        diskPaths,
		DiskDevice:       domain.DiskDevice,
		CommitDevs:       commitDevs,
		SkipSnapshotDevs: domain.SkipSnapshotDevs,
		RepoPath:         repo,
		TargetID:         tg.ID,
		DataDir:          s.cfg.DataDir,
		VM:               s.virsh,
		Restic:           &resticAdapter{engine: s.engine, mode: mode, extraTags: s.directTags(settings, "vms", repo)},
		BlockDisks:       vmBlockDisks,
		ZFSHost:          sshZFSHost{ssh: s.ssh},
		ZvolRestic:       &resticZvolAdapter{engine: s.engine, mode: mode, extraTags: s.directTags(settings, "vms", repo)},
	}
	live := false
	if method == "live" {
		// A live snapshot only works on a running VM (blockcommit --active --pivot
		// needs an active domain). A shut-off VM is backed up gracefully, which for
		// an already-off VM just backs up the disks and leaves it off. The check
		// error is not swallowed: a flaky host must never be misread as "not
		// running" and silently downgrade a live VM to a shutdown backup.
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

	// Start the run here rather than in the orchestrator's own Runs.Start, so
	// the run id is known before the orchestrator runs: RunTag must be set on
	// deps before they are passed in (see startedRunsAdapter). Wrapped in
	// startedRunsAdapter, the orchestrator's Runs.Start is a read of this same
	// id, not a second run row.
	runID, err := s.store.StartRun(tg.ID, "backup")
	if err != nil {
		return backup.Summary{}, fmt.Errorf("backup vm: record run start: %w", err)
	}
	if gid := runGroupFromContext(ctx); gid != "" {
		// The "Backup Everything" group stamp runsAdapter.Start does, inline here
		// because the run id is produced once, right here, rather than in a later
		// Start() call. Best-effort: a stamp failure must never fail a backup that
		// already started.
		if serr := s.store.SetRunGroup(runID, gid); serr != nil {
			log.Printf("api: BackupVM: run %s: stamp group %s failed: %v", runID, gid, serr) //nolint:gosec // G706: runID/gid are internal ids, not user input
		}
	}
	deps.Runs = startedRunsAdapter{st: s.store, runID: runID, svc: s, cancelKey: "vm:" + name}
	// RunTag correlates every snapshot one backup invocation produces, and is
	// only set when this backup produces more than one restic snapshot. A
	// file-only VM's single snapshot is already identified by its "vm:<name>"
	// tag, and a "vmrun:" tag on it would be permanent noise on every VM
	// snapshot in the repo.
	if len(vmBlockDisks) > 0 {
		deps.RunTag = "vmrun:" + runID
	}

	vkey := "vm:" + name
	// Healthchecks /start ping: deferred to here, past every pre-flight early-return
	// (incl. the ErrVMNotInstalled skip), so the paired done/fail notifyBackup below
	// always follows (no dangling /start).
	s.notifyBackupStart(ctx, "VM")
	// The orchestrator records its own run from here on, so the finisher above
	// stands down.
	orchestrated = true
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
	// A successful live backup commits its overlay back into the base and
	// pivots the VM onto it, but leaves the orphaned overlay file behind;
	// delete it so the next snapshot doesn't fail "already exists". No-op
	// after graceful.
	if live {
		s.removeStrayOverlays(diskPaths)
	}
	// Mirror the definition (encrypted) onto the backup storage so a freshly
	// installed BombVault can rebuild this VM via DiscoverVMs after a database
	// loss, and so a VM deleted from the host stays restorable. Best-effort. A
	// failed alias read leaves it as it was, as in Backup.
	if aliasErr != nil {
		log.Printf("api: backup vm: WARN the stored definition of %q stays as it was, since its aliases could not be read", name) //nolint:gosec // G706: name is %q-quoted
	} else if wErr := s.writeVMDefToStorage(settings, name, repo, defBytes, aliases); wErr != nil {
		log.Printf("api: backup vm: WARN could not persist definition for %q to storage: %v", name, wErr) //nolint:gosec // G706: name is %q-quoted
	}
	// Retention runs once for the VM's identity, aliases included, and once per
	// zvol disk tag, so each disk's history ages as its own group. zvol tags
	// have no aliases: a VM with block disks is never offered a takeover.
	s.applyRetention(ctx, repo, settings, mode, s.vmIdentity(name), "vms")
	for _, bd := range vmBlockDisks {
		if bd.Dev == "" {
			continue // without a target dev it has no tag of its own and ages with "vm:<name>"
		}
		s.applyRetention(ctx, repo, settings, mode, tagIdentity("vm:"+name+":zvol:"+bd.Dev), "vms")
	}
	makeRepoReadable(repo, s.cfg.DataDir) // keep the local repo copyable off-box by a non-root user
	s.replicateOffsite(ctx, "vms", settings, repo, "vm:"+name)
	s.collectStatsAfterItem(ctx, "vms")
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
	// restoreDirs restores each snapshot subtree into a destination dir, for a
	// cross-instance restore or a snapshot from before a rename moved the disks.
	// Empty means each disk goes back to its own path.
	restoreDirs []backup.VMRestoreDir
	// wasRunning is the captured run state (nil = a backup without it, so the
	// VM boots after restore).
	wasRunning *bool
	preDefine  func(context.Context) error
	// blockDisks are the domain's block-device (zvol) disks to restore, each
	// entry's SourceDataset resolved from the domain XML and its SnapshotID
	// and StdinPath from the "vmrun:" group (see prepareRestoreVMForTarget).
	// Empty for a VM with only file-backed disks.
	blockDisks []backup.VMRestoreBlockDisk
}

// prepareRestoreVM resolves the settings-configured vms repo (local or
// off-site) and delegates to prepareRestoreVMIn, which performs all of a
// VM restore's validation and resolution synchronously (confirmation,
// snapshot-id guard and ownership, definition lookup, disk-path
// containment and the SSH host-key pin), so a bad request fails
// immediately with a clear error, before anything long-running starts.
// Request guards run here first so a bad request fails with its own error
// before any resolution; prepareRestoreVMIn re-validates them for
// non-settings callers such as the foreign-repo session.
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
	repo, err := s.vmRepoForName(settings, name, source)
	if err != nil {
		return vmRestorePlan{}, err
	}
	return s.prepareRestoreVMIn(ctx, repoRef{repo: repo, mode: s.repoModeFor(settings, "vms", source, repo)}, name, snapshotID, confirm)
}

// prepareRestoreVMIn performs all of a VM restore's validation and resolution
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
	// so each disk returns to the definition's path, from the old folder when a
	// rename moved it, and the domain XML is used verbatim. ref is this
	// instance's own repo, so the local alias history applies.
	return s.prepareRestoreVMForTarget(ctx, ref, name, snapshotID, tg, s.vmIdentity(name), "", "")
}

// prepareRestoreVMForTarget builds a VM restore plan for an already resolved VM
// target tg against an explicit repo ref, without reading or writing the store:
// the VM counterpart of prepareRestoreForTarget. The foreign restore passes a
// target built from the decrypted foreign definition so its disk-path
// containment is validated before that recipe is persisted locally
// (prepareForeignRestore adopts it only once this returns a plan, never on a
// validation failure). The caller runs the confirm / explicit-snapshot-id-shape
// guards first.
//
// id is the identity the ownership check accepts, decided by the caller as in
// prepareRestoreForTarget: the local vmIdentity for a same-instance restore,
// only the literal vm:<name> tag for a foreign one, so a local rename cannot
// widen what a foreign restore accepts.
//
// destBase, when non-empty, remaps every file-backed disk (and the NVRAM) to
// <destBase>/<name>/<basename>: the restic restore target, the domain XML
// <disk><source file> / <nvram> paths, and the SSH NVRAM write all point at the
// destination folder instead of the source server's paths. A remap also gates
// the restore behind guardVMRestoreDestination, so it can never write a
// multi-GB disk onto an unmounted path (the RAM rootfs) and brick the host
// (#122). An empty destBase restores each disk to the definition's path, from
// wherever the snapshot holds it.
//
// destZvolPool is the separate remap a block-device (zvol) disk needs: destBase
// is a filesystem path under the host mount and carries no ZFS pool
// information at all, so it cannot rebase a zvol's `zfs receive` target the
// way it rebases a file-backed disk's path. When destBase is non-empty (a
// cross-instance restore) and the domain has recognizable zvol disks,
// destZvolPool must be supplied: every such disk's dataset is rebased onto it
// via virshcli.RebaseZvolDatasetPool, so `zfs receive` lands on the
// destination box's pool rather than the source box's. An empty destZvolPool
// on a cross-instance restore with zvol disks refuses here, before any
// restic/SSH work starts (a destination pool cannot be guessed from destBase),
// rather than silently attempting `zfs receive` against the source pool's name
// and failing deep inside that call once the restore is already underway.
// Ignored (never validated) for a same-instance restore or a VM with no zvol
// disks; a same-instance zvol restore derives its target from SourceDataset.
func (s *Service) prepareRestoreVMForTarget(ctx context.Context, ref repoRef, name, snapshotID string, tg store.VMTarget, id entryIdentity, destBase, destZvolPool string) (vmRestorePlan, error) {
	explicitID := snapshotID != "latest" && snapshotID != ""

	// "latest" (or empty) resolves to the VM's newest snapshot. An explicit id
	// must be one id owns, listed against the caller's repo ref, the same
	// access check the container restores make.
	snaps, snapErr := s.snapshotsOwnedBy(ctx, ref.repo, ref.mode, id)
	if snapErr != nil {
		return vmRestorePlan{}, snapErr
	}
	var snap *restic.Snapshot
	if explicitID {
		if snap = chosenSnapshot(snaps, snapshotID); snap == nil {
			return vmRestorePlan{}, notInListing{snapshotID, "vm"}
		}
	} else {
		if len(snaps) == 0 {
			return vmRestorePlan{}, errors.New("no backups found for this vm")
		}
		snap = &snaps[len(snaps)-1]
		snapshotID = snap.ID
	}

	// Resolve this run's "vmrun:<runID>" group: every snapshot one backup
	// produced, the main file-backed one plus one per zvol disk (see
	// VMBackupDeps.RunTag). The tag is read off the snapshot resolved above,
	// so one more listing over the same repo ref finds the group.
	//
	// vmrunGroup stays nil when that snapshot carries no "vmrun:" tag (see
	// vmRunTag), and vmRestoreBlockDisks then leaves every entry's
	// SnapshotID/StdinPath at their zero values.
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

	// Disks must live within the Host Data mount, which is how restic reaches
	// them. The backup refuses a VM with a disk outside it and refuses it whole,
	// so the restore refuses too: leaving the disk out and defining the VM from
	// an XML that still lists it hands back a machine with a missing or stale
	// disk and says nothing.
	for _, p := range def.DiskPaths {
		if !paths.Within(s.cfg.HostMountRoot, p) {
			return vmRestorePlan{}, destinationRefusal("disk %s in this backup is not under your Host Data mount (%s) and cannot be restored. Move it under that mount, or restore this VM to another instance with a destination folder", p, s.cfg.HostSourceRoot)
		}
	}
	diskPaths := def.DiskPaths
	if len(diskPaths) == 0 {
		return vmRestorePlan{}, errors.New("no restorable disk paths found in this backup")
	}
	sources, err := snapshotDiskSources(diskPaths, snap.Paths)
	if err != nil {
		return vmRestorePlan{}, err
	}

	domainXML := def.DomainXML
	nvramHostPath := def.NVRAMHostPath
	var restoreDirs []backup.VMRestoreDir

	// A snapshot from before a rename holds the disks in the old folder. Any
	// folder list replaces the in-place restore, so it names every disk's
	// folder, an unmoved one onto itself. A snapshot folder restored into two
	// folders would copy all of its disks into both, over any file of the same
	// name there.
	if destBase == "" && !slices.Equal(sources, diskPaths) {
		targets := make(map[string]string, len(diskPaths))
		for i, cp := range diskPaths {
			src, dst := path.Dir(sources[i]), path.Dir(cp)
			switch t, seen := targets[src]; {
			case !seen:
				targets[src] = dst
				restoreDirs = append(restoreDirs, backup.VMRestoreDir{Subtree: src, Target: dst})
			case t != dst:
				return vmRestorePlan{}, destinationRefusal("the snapshot folder %s holds disks that now sit in %s and in %s, and restoring it into both would copy all of its disks into each; move those disks into one folder or pick another snapshot", s.toHostPath(src), s.toHostPath(t), s.toHostPath(dst))
			}
		}
	}

	// Remap for a cross-instance restore: place every disk under
	// <destBase>/<name>/ on the destination pool, rewrite the domain XML disk
	// and nvram sources to match, and guard the destination so the restore
	// can never fill an unmounted path (the RAM rootfs) and brick the host
	// (#122). An empty destBase is the same-instance restore and skips all of
	// this (disks return to their own paths, XML verbatim).
	if destBase != "" {
		destDir := path.Join(path.Clean(destBase), name) // container path
		destHostDir := s.toHostPath(destDir)             // host path for the domain XML
		diskRemap := make(map[string]string, len(diskPaths))
		seenDir := map[string]bool{}
		remapped := make([]string, 0, len(diskPaths))
		// Every disk lands in one folder here, so two disks that share a file
		// name would land on each other. The same-instance path refuses the
		// mirror image of this a few lines up.
		takenBy := make(map[string]string, len(diskPaths))
		for i, cp := range diskPaths {
			base := path.Base(cp)
			if other, taken := takenBy[base]; taken {
				return vmRestorePlan{}, destinationRefusal("this VM has two disks called %s, %s and %s, and both would be restored into %s. Rename one of them in the VM, or restore to a destination that keeps their folders apart", base, s.toHostPath(other), s.toHostPath(cp), destHostDir)
			}
			takenBy[base] = cp
			remapped = append(remapped, destDir+"/"+base)
			diskRemap[s.toHostPath(cp)] = destHostDir + "/" + base
			if src := path.Dir(sources[i]); !seenDir[src] {
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
		// Host-brick guard: prove the destination is on a real mounted pool with
		// room before any restic write. On failure nothing is written.
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
	// (possibly remapped) domain XML, since vmDefinition does not store them
	// (see its TPMBytes field), as BackupVM derives them at backup time via
	// the same virshcli.ParseDomain call. The destBase remap above does not
	// affect either: RewriteDiskSources only rewrites <source file=...>
	// (file-backed disks); a <tpm> element and a <source dev=...>
	// (block-device disk) are left untouched. A zvol disk's dataset, as
	// opposed to the XML's device path (which keeps the source box's value
	// for the operator's reference), is rebased separately below, in the
	// cross-instance zvol rebase after the SSH guard.
	var tpmPath string
	var vmRestoreBlockDisks []backup.VMRestoreBlockDisk
	if parsed, perr := virshcli.ParseDomain(domainXML); perr == nil {
		tpmPath = parsed.TPMPath
		for _, bd := range parsed.BlockDisks {
			dataset, ok := virshcli.ZvolDatasetFromDevPath(bd.Source)
			if !ok {
				log.Printf("api: RestoreVM: WARN block-device disk %q (%s) is not a recognizable ZFS zvol for %q, so it will not be restored", bd.Dev, bd.Source, name) //nolint:gosec // G706: name is %q-quoted
				continue
			}
			rbd := backup.VMRestoreBlockDisk{SourceDataset: dataset}
			// Resolve this disk's own restic snapshot from the vmrun: group above.
			// Its identity tag is "vm:"+name+":zvol:"+bd.Dev (see VMBlockDisk.Dev in
			// internal/backup/vm_orchestrator.go): find the group member carrying
			// exactly that tag and take its snapshot id plus the one path it
			// recorded, which is the StdinPath BackupZvolDisk gave restic
			// (BackupStdin backs up one synthetic file per invocation, see
			// ZvolStdinPath, so Paths has exactly one entry when set at all).
			//
			// Left at zero value, the permanent fallback (see vmRunTag), when there
			// is no group or no member carries this disk's tag (an empty bd.Dev
			// never comes from the real BackupVM caller): RestoreZvolDisk then fails
			// loudly on the empty snapshot id rather than silently skipping the disk.
			if bd.Dev != "" {
				if gs, ok := vmrunGroupSnapshot(vmrunGroup, "vm:"+name+":zvol:"+bd.Dev); ok && len(gs.Paths) > 0 {
					rbd.SnapshotID = gs.ID
					rbd.StdinPath = gs.Paths[0]
				}
			}
			vmRestoreBlockDisks = append(vmRestoreBlockDisks, rbd)
		}
	} else {
		log.Printf("api: RestoreVM: WARN could not re-parse domain xml for %q (%v), so TPM state and any zvol disks will not be restored", name, perr) //nolint:gosec // G706: name is %q-quoted
	}

	// Mirrors BackupVM's "zvol backup requires SSH" guard: RestoreZvolDisk
	// calls ZFSHost.StreamReceive, which sshZFSHost forwards to s.ssh, and a
	// nil HostSSH there is a nil interface method call, an unrecovered panic
	// deep inside the async restore goroutine (StartRestoreVM) that would
	// crash the whole process rather than fail this one restore. Caught here,
	// before a plan is returned.
	if len(vmRestoreBlockDisks) > 0 && s.ssh == nil {
		return vmRestorePlan{}, fmt.Errorf("restore vm: %q has block-device (zvol) disks but no SSH host connection is configured, and zvol restore requires SSH", name)
	}

	// Cross-instance zvol rebase: destBase remaps a file-backed disk onto the
	// destination's filesystem path, but that path carries no ZFS pool
	// information, and RestoreZvolDisk would otherwise derive its `zfs
	// receive` target from SourceDataset (the source box's pool, re-derived
	// unchanged from the domain XML above), which almost certainly does not
	// exist on the destination box, and fail deep inside `zfs receive` once
	// the restore is underway. Refuse here, before any restic or SSH work,
	// when destZvolPool was not supplied; when it was, rebase every zvol
	// disk's dataset onto it now so the plan's blockDisks carry the
	// destination target.
	//
	// destZvolPool is trimmed before the emptiness check so a whitespace-only
	// value (a stray space in a direct API call) gets this clear refusal
	// rather than RebaseZvolDatasetPool's less actionable error below, which
	// would read it as empty and fail the "no segment past its own pool"
	// check instead of naming the real problem.
	if destBase != "" && len(vmRestoreBlockDisks) > 0 {
		if strings.TrimSpace(destZvolPool) == "" {
			// The web UI has no field for zvolPool (Recovery page), so an operator
			// hitting this through the UI cannot act on it in the app, and the
			// message names the only path there is: a direct API call. Keep it in
			// sync with docs/vm-backup-ssh-setup.md's TrueNAS section, which carries
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

	// preDefine writes the captured NVRAM and TPM state back to the host over
	// SSH after the old domain is undefined (which removes its nvram) and
	// before `virsh define`, so the restored VM boots with its original UEFI
	// variables and vTPM state. It writes to the (possibly remapped)
	// destination nvram path and the re-derived TPM path. No-op for either
	// when there is nothing to write or SSH is unavailable.
	var preDefine func(context.Context) error
	if s.ssh != nil && ((len(def.NVRAMBytes) > 0 && nvramHostPath != "") || (len(def.TPMBytes) > 0 && tpmPath != "")) {
		writeNVRAMPath := nvramHostPath
		writeTPMPath := tpmPath
		preDefine = func(ctx context.Context) error {
			if len(def.NVRAMBytes) > 0 && writeNVRAMPath != "" {
				if err := s.ssh.WriteFile(ctx, writeNVRAMPath, def.NVRAMBytes); err != nil {
					log.Printf("api: RestoreVM: WARN NVRAM write over SSH failed for %q (%v); the VM is restored and will boot, but libvirt regenerates the UEFI variables from the firmware template, so boot entries may need to be re-added", name, err) //nolint:gosec // G706: name is %q-quoted
				}
			}
			if len(def.TPMBytes) > 0 && writeTPMPath != "" {
				if err := s.ssh.WriteFile(ctx, writeTPMPath, def.TPMBytes); err != nil {
					log.Printf("api: RestoreVM: WARN TPM state write over SSH failed for %q (%v); the VM is restored and will boot, but the vTPM starts fresh/empty instead of restoring its captured state", name, err) //nolint:gosec // G706: name is %q-quoted
				}
			}
			return nil // never block the restore on NVRAM/TPM; the firmware-template fallback keeps the VM bootable
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

// snapshotDiskSources returns the path the snapshot holds each disk under: the
// disk's own path, or else the one file of the same name, which is where a
// snapshot from before a rename that moved the disk folder keeps it. A disk the
// snapshot lacks keeps its own path when another disk of its folder matched
// there, because the snapshot then predates the disk rather than a move.
func snapshotDiskSources(disks, snapshotPaths []string) ([]string, error) {
	byName := make(map[string][]string, len(snapshotPaths))
	for _, p := range snapshotPaths {
		byName[path.Base(p)] = append(byName[path.Base(p)], p)
	}
	diskNames := make(map[string]int, len(disks))
	inPlace := make(map[string]bool, len(disks))
	for _, d := range disks {
		diskNames[path.Base(d)]++
		if slices.Contains(snapshotPaths, d) {
			inPlace[path.Dir(d)] = true
		}
	}
	sources := make([]string, len(disks))
	for i, d := range disks {
		name := path.Base(d)
		held := byName[name]
		switch {
		case slices.Contains(held, d), len(held) == 0 && inPlace[path.Dir(d)]:
			sources[i] = d
		case len(held) == 0:
			return nil, fmt.Errorf("this snapshot has no disk named %s; pick a snapshot that includes it", name)
		case len(held) > 1 || diskNames[name] > 1:
			return nil, fmt.Errorf("the disk name %s is not unique, so this snapshot's copy of it cannot be matched; pick another snapshot or give the disks distinct names", name)
		default:
			sources[i] = held[0]
		}
	}
	return sources, nil
}

// guardVMRestoreDestination is the host-brick guard for a remapped
// (cross-instance) VM restore (#122): it proves the destination directory
// is safe to write a multi-GB disk image into before restic writes
// anything. Two checks:
//
//  1. destDir must sit on a real mounted pool or share, a mount point that
//     is a proper descendant of HostMountRoot (destinationMounted, shared
//     with #120). Otherwise a source path like /mnt/zfs/domains maps to an
//     unmounted dir on the destination, /mnt lives on the RAM rootfs, and
//     restic writes the image into tmpfs; OOM then kills emhttpd and nginx
//     and the host is bricked.
//  2. There must be enough free space for the restore. The size comes from
//     the source snapshot's restore-size (a read of the source repo); the
//     free-space probe runs on the nearest existing ancestor of destDir. A
//     probe error counts as "cannot prove insufficient" and does not block
//     (the mount check is the primary defence); only a proven shortfall
//     aborts.
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
// guardVMRestoreDestination, run only for a cross-instance (foreign)
// container restore (#123, the #122 class for containers). A foreign
// recipe carries the source host's absolute appdata paths; if the
// destination host lacks that pool (the source used /mnt/zfs but this box
// has no zfs share), restic would write appdata into an unmounted dir
// under the host mount, the array or RAM rootfs, landing the data in the
// wrong place or bricking the host on a large restore. This proves every
// appdata target sits on a mounted pool or share before the caller
// reaches executeRestore's destructive Stop and Remove, turning that
// silent wrong write into a clear refusal. The standard /mnt/user/appdata
// case always passes (shfs is a live mount below the host bind), so a
// normal cross-Unraid restore is unaffected.
func (s *Service) guardContainerRestoreDestination(ctx context.Context, ref repoRef, snapshotID string, appdataPaths []string) error {
	for _, p := range appdataPaths {
		if !s.destinationMounted(p) {
			return destinationRefusal("appdata destination %q is not on a mounted pool or share on this system: the source backed it up from a pool this host does not have, so restoring would write it to the wrong place. Create or mount that share here, then retry", s.toHostPath(p))
		}
	}
	// Free-space preflight only when the whole restore lands in a single
	// appdata path: several paths may sit on different pools, and the
	// snapshot's total restore-size can't be attributed per pool, which would
	// falsely refuse a legitimate split restore. A probe error is "cannot
	// prove insufficient" and never blocks; only a proven shortfall aborts,
	// as in guardVMRestoreDestination.
	if len(appdataPaths) == 1 {
		if _, wantBytes, err := s.engine.StatsRestoreSize(ctx, ref.repo, snapshotID, ref.mode); err == nil && wantBytes > 0 {
			if free, ferr := s.diskFreeFn()(nearestExistingDir(appdataPaths[0])); ferr == nil && free < uint64(wantBytes) {
				return destinationRefusal("not enough free space to restore this container's appdata: it needs %d bytes but %q has only %d free. Free up space and retry", wantBytes, s.toHostPath(appdataPaths[0]), free)
			}
		}
	}
	return nil
}

// appdataRelPath splits a backed-up container path into the appdata root
// it lives in and the container-relative remainder below it, e.g.
// /host/user/user/appdata/SnapOtter/conf → ("/host/user/user/appdata",
// "SnapOtter/conf") and /host/user/zfs/appdata/nexterm →
// ("/host/user/zfs/appdata", "nexterm"). The split is on the first
// "appdata" segment, the share or pool root every recorded path is
// selected by (resolveAppdataPaths keeps a bind only when
// hasSegment(src,"appdata")); taking the first occurrence keeps a
// container that owns a nested folder called "appdata"
// (…/appdata/foo/appdata) anchored at the real root.
//
// rel is "" when the path has no "appdata" segment or is an appdata root
// (a container binding /mnt/user/appdata itself); callers then fall back
// to the plain basename.
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
// (foreign) container restore: each backed-up appdata path's contents are
// restored into <destBase>/<container-relative path> on the destination
// pool, and the recreated container's binds and template are pointed
// there. destBase is a container path under HostMountRoot. It returns (1)
// the restic restore dirs (Subtree = the source appdata path, one of the
// snapshot's own backed-up paths; Target = the dest path), and (2) a
// bindRemap of host source path to host dest path for rewriting
// HostConfig.Binds and the flashed template.
//
// The destination keeps the path structure below the source's appdata
// root, not just the final segment. resolveAppdataPaths records every
// appdata bind as its own entry without merging binds that share a folder,
// so a container with two binds, /mnt/user/appdata/SnapOtter/conf and
// …/SnapOtter/data, arrives here as two paths sharing the "SnapOtter"
// ancestor. Mapped by basename alone they would land as "conf" and "data"
// directly in the destination appdata root, next to (and able to
// overwrite) other containers' folders; the relative path nests them under
// <destBase>/SnapOtter/…. For a single bind the relative path of
// /mnt/user/appdata/nexterm is "nexterm", so it lands in
// <destBase>/nexterm.
//
// RestoreSubtreeTo dumps the subtree's contents directly into Target (no
// path nesting, #62), so two source paths mapping to the same Target would
// silently merge data. Uniqueness is therefore enforced on the destination
// root folder: a residual name collision (the same container folder name
// from two different pools) gets a "-2"/"-3" suffix. The suffix is decided
// once per source root directory and reused by every path below it, so a
// multi-bind container is never split across <destBase>/SnapOtter and
// <destBase>/SnapOtter-2. Distinct source roots always get distinct
// destination roots, and two paths under the same source root differ in
// their remainder, so every Target stays unique.
func (s *Service) containerAppdataRemap(destBase string, appdataPaths []string) ([]backup.RestoreDir, map[string]string) {
	base := path.Clean(destBase)
	dirs := make([]backup.RestoreDir, 0, len(appdataPaths))
	remap := make(map[string]string, len(appdataPaths))
	destRootFor := map[string]string{} // source root dir -> its assigned dest root name
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
		// The container's bind points at the mount, not at the folder inside it
		// that the selection narrowed to, and rewriteBinds matches exactly. So map
		// the root level too, or a narrowed selection leaves the bind pointing at
		// the source host's path on this host: the restored data sits under the
		// destination and the recreated container starts against something else
		// entirely, with no warning (foreignBindWarnings stays quiet because
		// /mnt/user/appdata is mounted on every Unraid).
		//
		// Never overwrite a more specific entry: a selection that is the root
		// already wrote the identical value.
		if rootKey := s.toHostPath(srcRoot); remap[rootKey] == "" {
			remap[rootKey] = s.toHostPath(base + "/" + destRoot)
		}
	}
	return dirs, remap
}

// rewriteBinds points a recreated container's docker binds at the remapped
// appdata locations. Each bind is "HOSTPATH:CONTAINERPATH[:opts]"; only the
// host part is rewritten, and only on an exact match against a remap key,
// so appdata binds move to the destination while docker.sock,
// /etc/localtime, /dev/dri and every other non-appdata bind (which carry no
// backed-up data) stay byte for byte. The container path and option suffix
// (:ro,:z,…) are preserved.
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

// dirNonEmpty reports whether p exists and contains at least one entry,
// the overwrite guard's "there is already data here" check.
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

// foreignVMDestBase resolves the destination base directory (a container
// path) for a cross-instance VM restore's disks. Resolution order, never
// the source pool: an explicit target subpath (the request Target) wins;
// else the configured RestoreFolder (the settings default restore
// location); else the platform's conventional local VM domains location
// under the host mount (see platform.Platform.ForeignVMDestBase: Unraid's
// "user/domains" share, generic's identity default). Target and
// RestoreFolder are relative subpaths validated by paths.Resolve (no
// absolute path, no traversal), as in the file-set to-folder restore.
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

// foreignContainerDestBase resolves the destination base directory (a
// container path) a cross-instance container restore remaps appdata into,
// the container counterpart of foreignVMDestBase (#125). Resolution order,
// never the source pool: an explicit request Target wins; else the
// configured RestoreFolder; else the platform's conventional local appdata
// location under the host mount (see
// platform.Platform.ForeignContainerDestBase: Unraid's "user/appdata"
// share, generic's identity default). Target and RestoreFolder are
// relative subpaths validated by paths.Resolve (no absolute path, no
// traversal).
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
	// Hold the domain repo lock for the whole restic and libvirt phase,
	// including the destination pre-create below: the scheduler calls BackupVM
	// directly, bypassing batchActive, and the domain lock is the layer
	// scheduled jobs respect (see executeRestore).
	unlock := s.lockDomainFor("vms", "restore")
	defer unlock()
	// restic leaves a subtree target it creates root:root/0700, as in a container
	// restore (#125), whether the target is another pool or the folder a rename
	// moved the disks to. Pre-create each one readable; healRestoreDirOwnership
	// restores owner and mode after a successful restore.
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
		// Boot after restore iff the VM was running when backed up (nil = a
		// backup without recorded state, so boot) and the restore didn't ask to
		// leave it stopped.
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

// StartRestoreVM launches a VM restore in a background goroutine and
// returns immediately, mirroring StartRestore for the VM domain (a VM disk
// restore can run for hours, far past any browser or proxy idle timeout).
// All validation runs synchronously (a bad request fails right away, no
// goroutine); progress is published under "vm:<name>" and the orchestrator
// records the run.
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
		return err // SSH, auth or reachability problem; clearer than libvirt's error
	}
	if err := s.ssh.Test(ctx); err != nil {
		// EnsureKnownHost passed, so SSH auth and reachability are fine and only
		// libvirt is missing. Say so, so a notifications-only user (who needs the
		// SSH connection but not libvirt) isn't misled into thinking their SSH is
		// broken (#53).
		return fmt.Errorf("%w. The SSH connection itself is working, and libvirt is only needed for VM backups, not for Unraid notifications", err)
	}
	return nil
}

// LibvirtReachable reports whether libvirt is reachable over SSH, for the
// host-integration check's best-effort libvirt probe. Bounded by a timeout
// so a hung SSH attempt can't stall the check.
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

// SnapshotsVM lists the restic snapshots a VM's identity owns: those under its
// "vm:<name>" tag plus each alias's from before that alias was linked.
func (s *Service) SnapshotsVM(ctx context.Context, name, source string) ([]restic.Snapshot, error) {
	return s.vmSnapshotsOf(ctx, name, source, s.vmIdentity(name))
}

// vmSnapshotsOf is containerSnapshotsOf for VMs.
func (s *Service) vmSnapshotsOf(ctx context.Context, name, source string, id entryIdentity) ([]restic.Snapshot, error) {
	settings, err := s.store.GetSettings()
	if err != nil {
		return nil, fmt.Errorf("read settings: %w", err)
	}
	repo, err := s.vmRepoForName(settings, name, source)
	if err != nil {
		return nil, err
	}
	return s.snapshotsOwnedBy(ctx, repo, s.repoModeFor(settings, "vms", source, repo), id)
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
	// Reachable by shutdown, like every other backup.
	s.registerBackupCancel("flash", cancel)
	defer s.unregisterBackupCancel("flash")
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
	// issue #152 (bandwidth caps) and #182 (this domain's own credential set)
	mode := s.primaryModeFor(settings, "flash", repo)
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
		Runs:      runsAdapter{st: s.store, ctx: ctx, svc: s, cancelKey: "flash"},
	})
	s.progEnd("flash", "backup", err == nil, startedAt)
	s.notifyBackup(ctx, "flash", "", err == nil, sum, err)
	if err != nil {
		return backup.Summary{}, err
	}
	s.applyRetention(ctx, repo, settings, mode, tagIdentity("flash"), "flash")
	makeRepoReadable(repo, s.cfg.DataDir) // keep the local repo copyable off-box by a non-root user
	s.replicateOffsite(ctx, "flash", settings, repo, "")
	s.collectStatsAfterItem(ctx, "flash")
	s.checkPrimaryRemoteBudget(ctx, "flash", repo, settings)
	if err := s.exportFlashZip(ctx, settings, sum.SnapshotID, mode, repo); err != nil {
		log.Printf("flash zip export failed (backup is still valid): %v", err)
	}
	return sum, nil
}

// flashZipRe matches only the timestamped export filenames pruneFlashZips is
// allowed to delete (flash-<YYYYMMDD>-<HHMMSS>.zip, or the .age-sealed variant
// when export encryption is on). flash-latest.zip(.age) and any unrelated file the
// operator drops in the folder never match, so they survive.
var flashZipRe = regexp.MustCompile(`^flash-\d{8}-\d{6}\.zip(\.age)?$`)

// exportFlashZip writes the just-backed-up flash snapshot to the configured
// folder as a plain .zip, for off-server sync (Syncthing etc.). It is
// non-fatal: any failure is returned to the caller (BackupFlash logs it)
// and never fails the backup itself. The write is atomic (a temp file is
// renamed into place), so a sync tool never sees a half-written zip. Each
// attempt is recorded as a kind="export" run on the flash target (bytes =
// the written zip size) so it shows in the dashboard Activity Log and Run
// History; a disabled export records nothing.
func (s *Service) exportFlashZip(ctx context.Context, settings store.Settings, snapshotID string, mode restic.Mode, repo string) (err error) {
	if !settings.FlashZipExportEnabled || settings.FlashZipExportPath == "" {
		return nil
	}
	// Resolve age recipients up front: with export encryption on but no valid
	// recipient this fails before the temp zip is created, so no plaintext
	// artifact is produced when the user asked for encryption.
	recipients, _, err := s.exportRecipients(settings)
	if err != nil {
		return err
	}
	// Publish a live "maintenance" progress pair keyed "export:flash" (like
	// prune:, verify:, drill:) so a long zip export of a big flash shows on the
	// dashboard activity log while it writes, not only after (#109). The
	// terminal event is deferred so any failure path above clears the live
	// line. A disabled export publishes nothing, hence below the guard.
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
	// Cheap size sample for the run record (bytes column), best-effort only.
	if fi, statErr := os.Stat(final); statErr == nil {
		zipBytes = fi.Size()
	}
	// Prune in both modes: in latest mode (Keep==0) the user opted out of history,
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
// mount, resolved like settings.ContainersPath) to the files repo via
// restic, tagged fileset:<Name>. It mirrors BackupFlash: no lifecycle, no
// defs, with retention, off-site replication and stats as in the other
// domains. A source folder that does not exist under the host mount fails
// with a clear error before any restic call, recording a failed run against
// the set's id so a scheduled backup of a vanished folder surfaces in Run
// History.
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
	// Registered under the set's name, not its id, and here rather than at the
	// top of the function for that reason (#200). It is the key the progress
	// stream publishes under ("files:"+set.Name, a few lines below), the key
	// containers and VMs use for both purposes. The interface cancels with the
	// one key it has, the one from the progress stream, and a mismatch would
	// make the Cancel button answer "cancelled: false" and do nothing, with no
	// error anywhere to explain it. What runs before this is a settings read
	// and a row lookup, neither of which can hang or is worth cancelling.
	s.registerBackupCancel("files:"+set.Name, cancel)
	defer s.unregisterBackupCancel("files:" + set.Name)
	// A set without a path cannot be backed up (Discover creates path-less,
	// disabled sets from fileset: tags alone); say so instead of letting
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
			// truncateRunErr, like every other FinishRun in this file: the message
			// embeds the resolved host path (scrub), and a file set's name and path
			// are never length-validated at creation, so an arbitrarily long string
			// could otherwise reach runs.error uncapped and travel on into the weekly
			// digest.
		} else if fErr := s.store.FinishRun(runID, "failed", "", 0, truncateRunErr(err)); fErr != nil {
			log.Printf("api: files backup: %q: finish missing-path run: %v", set.Name, fErr) //nolint:gosec // G706: name is %q-quoted
		}
		return backup.Summary{}, err
	}
	// The set's own repository if it has one (#204), otherwise the domain's.
	// Everything below takes `repo` as a parameter (EnsureRepo, the run
	// record, applyRetention, makeRepoReadable, checkPrimaryRemoteBudget), so a
	// per-set repository carries through the whole backup without a second
	// decision point. replicateOffsite is the exception and stays per domain:
	// an off-site copy is configured for the Folders domain, and a set's own
	// primary says nothing about where its replica should live.
	item := store.ItemRef{Domain: "files", Key: set.ID}
	step, err := s.prepareHome(ctx, settings, item)
	if err != nil {
		return backup.Summary{}, err
	}
	if step, err = s.recordHome(ctx, settings, item, step); err != nil {
		return backup.Summary{}, err
	}
	repo, mode := step.repo, step.mode
	// An empty or unreadable source still makes a successful restic snapshot,
	// and keep-N retention groups by tag alone, so each such snapshot would
	// age a real one of a set with history out. The walk stops at the first
	// file, so a populated folder never pays for the listing.
	if empty, eErr := fileSetSourceEmpty(src); eErr != nil || empty {
		hasHistory := true
		if snaps, hErr := s.snapshotsForTag(ctx, repo, mode, "fileset:"+set.Name); hErr == nil {
			hasHistory = len(snaps) > 0
		} else {
			log.Printf("api: files backup: %q: could not check existing history: %v", set.Name, hErr) //nolint:gosec // G706: name is %q-quoted
		}
		if hasHistory {
			reason := fmt.Sprintf("source folder for %q has no files (%s); check whether it moved", set.Name, src)
			if eErr != nil {
				reason = fmt.Sprintf("source folder for %q could not be read (%s): %v", set.Name, src, eErr)
			}
			err := fmt.Errorf("files backup: %s", reason)
			if runID, sErr := s.store.StartRun(set.ID, "backup"); sErr != nil {
				log.Printf("api: files backup: %q: record failed run: %v", set.Name, sErr) //nolint:gosec // G706: name is %q-quoted
			} else if fErr := s.store.FinishRun(runID, "failed", "", 0, truncateRunErr(err)); fErr != nil {
				log.Printf("api: files backup: %q: finish failed run: %v", set.Name, fErr) //nolint:gosec // G706: name is %q-quoted
			}
			return backup.Summary{}, err
		}
	}
	// Healthchecks /start ping: deferred to here, past the source-exists + EnsureRepo
	// guards, so the paired done/fail notifyBackup below always follows (no dangling /start).
	s.notifyBackupStart(ctx, "files")
	key := "files:" + set.Name
	fctx, startedAt := s.progBegin(ctx, key, "backup")
	// The single compile site for the files domain. The set was read fresh
	// above (a selection saved between two backups affects exactly the later
	// one, and a concurrent edit can never tear a mid-run argv), and
	// fileSetPositionals re-anchors the stored entries against this run's
	// resolved root, so a Path edit can never hand restic a positional outside
	// the set's current scope. set.SelectedPaths == nil (the NULL column)
	// compiles to []string{src}. User-owned exclude patterns stay first; the
	// selection-derived tail enforces the stored exclusion branches on the
	// argv, like the container compile line in the BackupDeps literal
	// (excludedBranches(nil) is empty, so the append is always safe). Patterns
	// travel as typed builder arguments (excludes before --, positionals
	// after), never through a shell.
	positionals := fileSetPositionals(set.SelectedPaths, src)
	// The exclusion tail is derived against the same anchored view, never the
	// raw stored list. With the exclusions read straight off storage, a Path
	// edit could make the includes fall out of scope and the positional fall
	// back to src, and an exclusion above the new root would then be emitted
	// as a strict descendant of the old one, asking restic to filter its own
	// source root. selection.go never emits that pair and relies on the tree
	// UI being unable to construct it; the fallback would construct it around
	// the UI. Pairing the positionals with the stored exclusions closes that:
	// an exclusion equal to a positional is not a strict descendant, so it is
	// not emitted.
	var anchored []string
	anchored = append(anchored, positionals...)
	for _, e := range set.SelectedPaths {
		if bare, excluded := SplitExclusion(e); excluded && bare != "" {
			anchored = append(anchored, e)
		}
	}
	sum, err := backup.BackupFileSetDir(fctx, backup.FileSetBackupDeps{
		SourceDir:   src,
		SourcePaths: positionals,
		Repo:        repo,
		TargetID:    set.ID,
		SetName:     set.Name,
		Excludes: append(append([]string{}, set.Excludes...),
			excludedBranches(anchored)...),
		Restic: &resticAdapter{engine: s.engine, mode: mode, extraTags: s.directTags(settings, "files", repo)},
		Runs:   runsAdapter{st: s.store, ctx: ctx, svc: s, cancelKey: "files:" + set.Name},
	})
	s.progEnd(key, "backup", err == nil, startedAt)
	s.notifyBackup(ctx, "files", set.Name, err == nil, sum, err)
	if err != nil {
		return backup.Summary{}, err
	}
	s.applyRetention(ctx, repo, settings, mode, tagIdentity("fileset:"+set.Name), "files")
	makeRepoReadable(repo, s.cfg.DataDir) // keep the local repo copyable off-box by a non-root user
	s.replicateOffsite(ctx, "files", settings, repo, "fileset:"+set.Name)
	s.collectStatsAfterItem(ctx, "files")
	s.checkPrimaryRemoteBudget(ctx, "files", repo, settings)
	return sum, nil
}

// walkFileSetSource is fileSetSourceEmpty's directory walk, replaceable in
// tests so a read failure can be forced.
var walkFileSetSource = filepath.WalkDir

// fileSetSourceEmpty reports whether src holds nothing but directories.
// Excludes play no part: a fully excluded folder is still the real folder.
func fileSetSourceEmpty(src string) (bool, error) {
	empty := true
	err := walkFileSetSource(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if p == src || d.IsDir() {
			return nil
		}
		empty = false
		return filepath.SkipAll
	})
	if err != nil {
		return false, err
	}
	return empty, nil
}

// errFileSetNotFound is the user-safe error for an unknown file-set id (the
// raw store error would leak SQL wording through the API surface).
var errFileSetNotFound = errors.New("file set not found")

// errFileSetEmptySelection is the file-set boundary's refusal; the PATCH
// handler maps it (errors.Is) to the machine-routable "empty-selection"
// envelope code, as the containers' boundary does for errEmptySelection. A
// file set cannot mean "back up nothing": a set with zero included folders
// is either a mistake or a set the user no longer wants, so the message
// points to removing the set rather than "select something".
var errFileSetEmptySelection = errors.New("a file set needs at least one folder selected; use Remove set if you no longer want this set")

// FileSetView is the per-set row returned by ListFileSetViews, the files
// domain's counterpart of VMView. LastBackup is the unix time of the last
// successful backup run (0 = never; runs-based, so listing never spawns a
// restic process per row). PathExists reports whether the set's resolved
// source folder exists under the host mount; it is false for a vanished
// folder and for the path-less sets DiscoverFileSets creates, so the UI
// can flag "set path before backup".
type FileSetView struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	Path       string   `json:"path"`
	Excludes   []string `json:"excludes"`
	Enabled    bool     `json:"enabled"`
	LastBackup int64    `json:"lastBackup"`
	PathExists bool     `json:"pathExists"`
	// SelectedPaths is the set's tree selection, served back for the editor in
	// the flat encoding the containers' backupPaths uses (bare entries are
	// included roots, "!"-prefixed ones deselected branches), in mount-root
	// absolute space. omitempty keeps the NULL column ("never touched by the
	// tree") distinguishable on the wire from a written selection: a
	// never-edited set must not render a tree state it never had.
	SelectedPaths []string `json:"selectedPaths,omitempty"`
	// ScheduleCadence is the set's per-item schedule override (#199); empty means
	// it follows the Folders domain schedule. Always sent, so the interface can
	// show the cadence without a second request, and only acted on while the
	// per-item-schedules toggle is on.
	ScheduleCadence string `json:"scheduleCadence"`
	// Repo is the set's own repository (#204); empty means it follows the
	// Folders domain repository. Always sent, so the card can show where a set
	// backs up without a second request, and a set with an override says so
	// instead of looking like every other set while its snapshots live
	// somewhere else.
	Repo string `json:"repo"`
	// RepoEffective is where this set's backups actually land, already resolved:
	// the override if there is one, otherwise the domain path. Computed here so
	// the sentence on the card comes from the same resolution the backup runs
	// through, not from a second copy in the interface that can drift.
	RepoEffective string `json:"repoEffective"`
	// EffectiveSchedule is what actually happens to this set: which schedule
	// backs it up, whether two of them do, or whether none does. Computed here
	// rather than in the interface so the sentence on the Folders card comes from
	// the same rules the scheduler runs, not from a second copy that can drift
	// (#199: three controls that all read like scheduling, and the combination
	// that looks most sensible silently protects nothing).
	EffectiveSchedule schedule.EffectiveSchedule `json:"effectiveSchedule"`
	Placement         placementView              `json:"placement"`
}

// ListFileSetViews returns all configured file sets with their last-backup
// time and source-path existence.
func (s *Service) ListFileSetViews(ctx context.Context) ([]FileSetView, error) {
	sets, err := s.store.ListFileSets()
	if err != nil {
		return nil, fmt.Errorf("list file sets: %w", err)
	}
	// Read once for the whole list: the effective schedule depends on four
	// settings fields, and asking per set would issue N identical reads.
	settings, err := s.store.GetSettings()
	if err != nil {
		return nil, fmt.Errorf("read settings: %w", err)
	}
	// Dated from the backups a set owns, as on the container and VM lists.
	var snapTimes map[string]int64
	snapTimesFailed := false
	if len(sets) > 0 {
		if m, sErr := s.LatestFileSetBackupTimes(ctx); sErr != nil {
			log.Printf("api: list file sets: latest backup times: %v", sErr)
			snapTimesFailed = true
		} else {
			snapTimes = m
		}
	}
	views := make([]FileSetView, 0, len(sets))
	for _, set := range sets {
		v := FileSetView{
			ID:                set.ID,
			Name:              set.Name,
			Path:              set.Path,
			Excludes:          set.Excludes,
			Enabled:           set.Enabled,
			ScheduleCadence:   set.ScheduleCadence,
			Repo:              set.Repo,
			EffectiveSchedule: schedule.EffectiveFileSetSchedule(set, settings),
		}
		// Resolved through the same helper the backup uses, so the card and the
		// run can never disagree about where this set goes. An unresolvable
		// override is shown as the raw stored value rather than swallowed: a
		// location that cannot resolve is exactly what the user has to see.
		if eff, rErr := s.fileSetRepoPath(settings, set); rErr == nil {
			v.RepoEffective = eff
		} else {
			v.RepoEffective = set.Repo
		}
		if v.Excludes == nil {
			v.Excludes = []string{}
		}
		// The stored form is served back verbatim, never re-normalized here: the
		// compile (fileSetPositionals) re-runs the shared NormalizeSelection, and
		// the view mirrors the column. nil stays nil so omitempty drops the key
		// (the NULL column).
		v.SelectedPaths = set.SelectedPaths
		run, _ := s.store.LastSuccessfulBackup(set.ID)
		if finished, _ := lastBackupDate(set.Name, run, snapTimes, snapTimesFailed); finished != nil {
			v.LastBackup = *finished
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
// restic tag and a progress key, so it passes the same strict charset as
// container names; the path must be a relative subpath under the host mount,
// and, when checkPathExists is true, must also exist on disk. A patch checks
// the path only when it changes the path or enables the set, so a dead path
// does not block the rest of the set's settings. A path-less set is valid
// while it stays disabled, the shape DiscoverFileSets creates from fileset:
// tags alone.
func (s *Service) validateFileSet(fs store.FileSet, checkPathExists bool) error {
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
	if checkPathExists {
		if _, statErr := os.Stat(resolved); statErr != nil { //nolint:gosec // G703: resolved is containment-validated under the host mount root
			return errors.New("source path not found under the host mount")
		}
	}
	return nil
}

// maxFileSetSelectedPaths caps one file-set selection: a tree save
// overwrites the whole column, so the cap bounds both the JSON blob and the
// normalize and compile work per save. It matches the ceiling
// SetExcludeCaches puts on the per-root exclusion map, so no editor surface
// can differ.
const maxFileSetSelectedPaths = 64

// SetFileSetSelectedPaths validates and stores a file set's tree
// selection. It is the file-set twin of SetBackupPaths (the containers'
// flat-set setter) and the only writer the tree editor reaches (the PATCH
// handler's selectedPaths field lands here). Unlike SetBackupPaths, the
// anchor is the set's own resolved root (its Path under the host mount),
// not a mount translation, and entries live in mount-root absolute space;
// they are never container-translated.
//
// Per-entry validation runs before any store write, so the whole save is
// rejected atomically: a list whose 40th entry escapes the root must leave
// the prior selection untouched, not half-apply. Each entry is trimmed,
// split into its class (SplitExclusion; the "!" prefix marks an excluded
// branch), cleaned, and must equal the set's resolved root or lie strictly
// below it (isStrictDescendant, segment-aligned, so /data/doc can never
// pass for a root /data/docs; fileSetPositionals re-anchors with the same
// primitive at compile time as a second layer). The raw entry text goes
// back into the error so the log identifies the offender; the envelope
// scrubs it to [path] on the way out.
//
// The validated list is then normalized through the shared
// NormalizeSelection (dedupe, per-class maximal-root prune, canonical
// order; never a second pruning site), and only a list that still has at
// least one include survives: zero includes is refused with
// errFileSetEmptySelection before any write, so a refused deselect leaves
// the prior selection untouched. The read-side stale-root re-anchor in
// fileSetPositionals remains the defense for anything already stored.
func (s *Service) SetFileSetSelectedPaths(_ context.Context, id string, entries []string) error {
	if len(entries) > maxFileSetSelectedPaths {
		return fmt.Errorf("too many selected paths (%d, max %d)", len(entries), maxFileSetSelectedPaths)
	}
	set, err := s.store.GetFileSet(id)
	if err != nil {
		return errFileSetNotFound
	}
	root, err := paths.Resolve(s.cfg.HostMountRoot, set.Path)
	if err != nil {
		return fmt.Errorf("file set %q has no valid source path to select folders under", set.Name)
	}
	cleaned := make([]string, 0, len(entries))
	for _, e := range entries {
		e = strings.TrimSpace(e)
		if e == "" {
			return errors.New("empty selected path")
		}
		bare, excluded := SplitExclusion(e)
		if bare == "" {
			return fmt.Errorf("empty excluded path %q", e)
		}
		bare = path.Clean(bare)
		if bare != root && !isStrictDescendant(bare, root) {
			return fmt.Errorf("selected path %q is not under the set's source folder", e)
		}
		if excluded {
			bare = ExclusionPrefix + bare
		}
		cleaned = append(cleaned, bare)
	}
	normalized := NormalizeSelection(cleaned)
	if len(includesOnly(normalized)) == 0 {
		return errFileSetEmptySelection
	}
	if err := s.store.SetFileSetSelectedPaths(id, normalized); err != nil {
		return fmt.Errorf("store file set selection: %w", err)
	}
	return nil
}

// errFileSetRepoUnreachable means fileSetHasBackups could not ask the set's
// repository: it is established but not mounted, or its established marker
// could not be read. The set then counts as having backups.
var errFileSetRepoUnreachable = errors.New("its repository could not be checked right now (not reachable)")

// fileSetHasBackups reports whether the file set id already has at least one
// recorded successful backup run, i.e. fileset:<Name>-tagged snapshots exist
// in the repo. handlePatchFileSet refuses a name change when this is true,
// since those snapshots stay tagged with the old name and are never
// re-tagged.
func (s *Service) fileSetHasBackups(ctx context.Context, id string) (bool, error) {
	run, err := s.store.LastSuccessfulBackup(id)
	if err != nil {
		return false, err
	}
	if run != nil {
		return true, nil
	}
	// The listing below reads a missing local repo directory as never
	// established, even when the marker could not be read, so that case is
	// caught here first.
	set, sErr := s.store.GetFileSet(id)
	if sErr != nil {
		return false, errFileSetNotFound
	}
	settings, gErr := s.store.GetSettings()
	if gErr != nil {
		return false, fmt.Errorf("read settings: %w", gErr)
	}
	if repo, rErr := s.fileSetRepoPath(settings, set); rErr == nil {
		if s.repoEstablishmentOf(repo) == repoEstablishmentUnknown {
			return true, errFileSetRepoUnreachable
		}
	}
	// A Discover-rebuilt set has real fileset:<Name> snapshots in the repo
	// but a fresh id with no run rows, so the runs table alone would miss it.
	snaps, err := s.SnapshotsFileSet(ctx, id, "local")
	if err != nil {
		if errors.Is(err, errFileSetNotFound) {
			return false, err
		}
		if errors.Is(err, ErrBackupPathNotMounted) {
			return true, errFileSetRepoUnreachable
		}
		return true, fmt.Errorf("%w: %v", errFileSetRepoUnreachable, err)
	}
	return len(snaps) > 0, nil
}

// fileSetNameAdoptable decides whether a create, or a repository or
// pre-backup name change, may land a file set on name: it refuses when
// fileset:<name> snapshots already exist in the repository this set will
// use, unless every one of them recorded a path at or below the resolved
// source, the same folder coming back. A listing failure refuses too, as in
// fileSetHasBackups.
func (s *Service) fileSetNameAdoptable(ctx context.Context, name, repoOverride, path string) error {
	settings, err := s.store.GetSettings()
	if err != nil {
		return fmt.Errorf("read settings: %w", err)
	}
	repo, err := s.fileSetRepoPath(settings, store.FileSet{Name: name, Repo: repoOverride})
	if err != nil {
		return err
	}
	mode := s.repoModeFor(settings, "files", "local", repo)
	snaps, err := s.snapshotsForTag(ctx, repo, mode, "fileset:"+name)
	if err != nil {
		return fmt.Errorf("%q: leftover backups could not be checked: %w", name, err)
	}
	if len(snaps) == 0 {
		return nil
	}
	resolved, rErr := paths.Resolve(s.cfg.HostMountRoot, path)
	if rErr != nil {
		return fmt.Errorf("%q already has backups; set a valid path so its source can be compared, or bring the old set back with Discover, or delete its old snapshots first", name)
	}
	for _, snap := range snaps {
		if !snapshotPathsMatchRoot(snap.Paths, resolved) {
			return fmt.Errorf("%q already has backups from a different source folder; bring the old set back with Discover, or delete its old snapshots first", name)
		}
	}
	return nil
}

// snapshotPathsMatchRoot reports whether every one of a snapshot's recorded
// paths is the resolved root itself or lies strictly below it. A set with a
// tree selection records the selected subpaths, never the root, so this is
// what tells "the same folder coming back" from a recorded parent or a
// disjoint folder wearing the same name.
func snapshotPathsMatchRoot(recorded []string, resolved string) bool {
	if len(recorded) == 0 {
		return false
	}
	for _, p := range recorded {
		if p != resolved && !isStrictDescendant(p, resolved) {
			return false
		}
	}
	return true
}

// containerHasBackups / vmHasBackups are fileSetHasBackups for the other two
// domains. An item's snapshots stay in the repository they were written to,
// so re-pointing an item that already has some splits its history, and the
// old half stays invisible, never pruned and reachable only through restic by
// hand.
//
// The runs table alone is not enough: an item rebuilt by Discover after a
// /config loss has real snapshots but a fresh id with no run rows. The
// interface's lastBackup lock misses that case, so the server refuses.
func (s *Service) containerHasBackups(ctx context.Context, name string) (bool, error) {
	tg, err := s.store.GetTargetByContainer(name)
	if err == nil {
		if run, rErr := s.store.LastSuccessfulBackup(tg.ID); rErr == nil && run != nil {
			return true, nil
		}
	}
	id := s.containerIdentity(name)
	if id.readErr != nil {
		return false, fmt.Errorf("its backups could not be checked: %w", id.readErr)
	}
	snaps, err := s.containerSnapshotsOf(ctx, name, "local", id)
	if err != nil {
		// Unreadable is not "empty": refusing conservatively is the safe way
		// round, because the cost of being wrong the other way is a split
		// history nobody can see.
		return true, nil //nolint:nilerr // unknown counts as "has backups"
	}
	return len(snaps) > 0, nil
}

func (s *Service) vmHasBackups(ctx context.Context, name string) (bool, error) {
	vm, err := s.store.GetVMTargetByName(name)
	if err == nil {
		if run, rErr := s.store.LastSuccessfulBackup(vm.ID); rErr == nil && run != nil {
			return true, nil
		}
	}
	id := s.vmIdentity(name)
	if id.readErr != nil {
		return false, fmt.Errorf("its backups could not be checked: %w", id.readErr)
	}
	snaps, err := s.vmSnapshotsOf(ctx, name, "local", id)
	if err != nil {
		return true, nil //nolint:nilerr // see containerHasBackups
	}
	return len(snaps) > 0, nil
}

// SnapshotsFileSet lists restic snapshots for a single file set, filtered
// by the "fileset:<Name>" tag its backups write; it is the files
// counterpart of SnapshotsVM. id is the set's stable store id; source
// selects the local or off-site repo.
func (s *Service) SnapshotsFileSet(ctx context.Context, id, source string) ([]restic.Snapshot, error) {
	set, err := s.store.GetFileSet(id)
	if err != nil {
		return nil, errFileSetNotFound
	}
	settings, err := s.store.GetSettings()
	if err != nil {
		return nil, fmt.Errorf("read settings: %w", err)
	}
	repo, err := s.fileSetRepoFor(settings, set, source)
	if err != nil {
		return nil, err
	}
	mode := s.repoModeFor(settings, "files", source, repo)
	// A listing before any backup has run is "no snapshots yet", not an error.
	if localRepoMissing(repo) {
		// #55 vs #120: only surface "not mounted" when the backing store is truly
		// absent. If the destination is mounted, this is a fresh or phantom repo
		// on a healthy disk, so report an empty list (EnsureRepo re-establishes on
		// write).
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

// ListSnapshotFilesFileSet lists the files in a file-set snapshot for the
// selective (pick-some-files) restore, the files counterpart of
// ListSnapshotFiles. snapshotID must be valid hex and must belong to this
// set (tag-scoped via SnapshotsFileSet and snapshotBelongs), so one set's
// file tree can't be listed through another's route.
func (s *Service) ListSnapshotFilesFileSet(ctx context.Context, id, snapshotID, source string) ([]restic.FileEntry, error) {
	if !backup.ValidSnapshotID(snapshotID) {
		return nil, backup.ErrInvalidSnapshotID
	}
	snaps, err := s.SnapshotsFileSet(ctx, id, source)
	if err != nil {
		return nil, err
	}
	if !snapshotBelongs(snaps, snapshotID) {
		return nil, notInListing{snapshotID, "file set"}
	}
	// Loaded for its repository override (#204): a set with its own repo is
	// listed from that repo, and reading the domain's would report "snapshot
	// not found" for a snapshot that exists.
	set, err := s.store.GetFileSet(id)
	if err != nil {
		return nil, errFileSetNotFound
	}
	settings, err := s.store.GetSettings()
	if err != nil {
		return nil, fmt.Errorf("read settings: %w", err)
	}
	repo, err := s.fileSetRepoFor(settings, set, source)
	if err != nil {
		return nil, err
	}
	return s.lsSelfHeal(ctx, repo, snapshotID, s.repoModeFor(settings, "files", source, repo))
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
	subtree    string // to-folder: the snapshot's own tree node covering all its recorded paths, the <id>:<subtree> restore root ("" = path-less snapshot or roots with no shared ancestor -> whole-tree fallback)
}

// prepareRestoreFileSet performs all of a file-set restore's validation and
// resolution synchronously, so a bad request fails immediately with a
// clear error, and creates the alternate target folder once containment
// passes.
//
// Security: the snapshot id passes the strict hex guard
// (backup.ValidSnapshotID) and must belong to this set (tag-scoped via
// SnapshotsFileSet and snapshotBelongs, like prepareRestoreToPath), so one
// set's data can't be extracted through another's route. An empty
// targetSubPath restores in place over the set's source folder, which is
// destructive and therefore confirm-gated; a non-empty targetSubPath is
// resolved with paths.Resolve under the host mount (rejecting absolute and
// `..` escapes) and created only after containment passes, which is
// non-destructive and needs no confirm (as in RestoreContainerToPath).
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
		// In place writes over the set's source folder, so require the explicit
		// confirmation (the same sentinel discipline as prepareRestore).
		if !confirm {
			return fileSetRestorePlan{}, backup.ErrNotConfirmed
		}
		// A discovered, path-less set has no original location to restore to; say
		// so instead of letting paths.Resolve report a misleading traversal error
		// for "" (restore to a folder works without a path).
		if strings.TrimSpace(set.Path) == "" {
			return fileSetRestorePlan{}, fmt.Errorf("file set %q has no source path configured: restore to a folder instead, or set a path first", set.Name)
		}
		src, rErr := paths.Resolve(s.cfg.HostMountRoot, set.Path)
		if rErr != nil {
			return fileSetRestorePlan{}, errors.New("invalid file set path: must be a relative subpath under the host mount")
		}
		plan.inPlace = src
	}

	// Scope to this set: the snapshot must be one of its snapshots.
	snaps, err := s.SnapshotsFileSet(ctx, id, source)
	if err != nil {
		return fileSetRestorePlan{}, err
	}
	if !snapshotBelongs(snaps, snapshotID) {
		return fileSetRestorePlan{}, notInListing{snapshotID, "file set"}
	}
	// Take the to-folder restore subtree from the snapshot itself, not a
	// recompute of set.Path: HostMountRoot may have changed since the backup,
	// and a recomputed <id>:<path> selector would then miss and fail. It is
	// the node covering every path the snapshot recorded; a set whose
	// selection is two sub-folders records two roots, and taking only the
	// first would restore half the set while reporting success. Empty (a
	// path-less snapshot) falls back to a whole-tree restore in
	// runRestoreFileSet; it is unused for an in-place restore.
	plan.subtree = snapshotRestoreRoot(snaps, snapshotID)

	// An in-place restore writes back over the set's source folder, so before
	// anything destructive the set's compiled selection (the
	// fileSetPositionals the backup ran, so guard and backup never disagree
	// about what the set selects) is mapped against the chosen snapshot's
	// recorded Paths; selectors come from the snapshot, never replayed from
	// storage. An empty intersection is refused here, synchronously: the
	// failure mode the container route's mapRestorePaths guard prevents, a
	// restore that would miss mid-loop after the folder was torn down.
	//
	// Only the in-place route does this: a to-folder restore is
	// non-destructive and its subtree comes from the snapshot, so a selection
	// that no longer matches it must not abort (TestRestoreFileSetToFolder's
	// cross-root snapshot keeps restoring). Snapshots with no recorded Paths
	// have nothing to map against and keep restoring whole.
	if plan.inPlace != "" {
		if chosen := chosenSnapshot(snaps, snapshotID); chosen != nil && len(chosen.Paths) > 0 {
			compiled := fileSetPositionals(set.SelectedPaths, plan.inPlace)
			// Neither skipped nor the third return is used: this guard only answers
			// "is the intersection empty", and the restore it guards hands restic the
			// set's own root (plan.inPlace), never one of these mapped paths, so no
			// unchecked selector can reach the engine and there are no per-path skips
			// to report.
			mapped, _, _ := mapRestorePaths(compiled, chosen.Paths)
			if len(mapped) == 0 {
				return fileSetRestorePlan{}, errors.New("nothing to restore for this set from this snapshot")
			}
		}
	}

	settings, err := s.store.GetSettings()
	if err != nil {
		return fileSetRestorePlan{}, fmt.Errorf("read settings: %w", err)
	}
	repo, err := s.fileSetRepoFor(settings, set, source)
	if err != nil {
		return fileSetRestorePlan{}, err
	}
	plan.repo = repo
	plan.mode = s.repoModeFor(settings, "files", source, repo)

	// Create the alternate target dir only after every validation passed,
	// with the readable (0o755) variant: the restore target lives on a
	// user-visible or synced share, so the operator's non-root SMB user must be
	// able to read what root restored there (see EnsureDirReadable).
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
	// batchActive and the domain lock is the layer they respect (see
	// executeRestore).
	unlock := s.lockDomainFor("files", "restore")
	defer unlock()
	if plan.inPlace != "" {
		return s.engine.RestorePath(ctx, plan.repo, plan.snapshotID, plan.inPlace, plan.mode)
	}
	if plan.subtree != "" {
		// Restore the snapshot's own subtree directly into the chosen folder, so
		// its contents land at <target>/… and not <target>/host/user/… (#62's
		// nested restore, which a bare RestoreInclude("/") produces).
		return s.engine.RestoreSubtreeTo(ctx, plan.repo, plan.snapshotID, plan.subtree, plan.target, plan.mode)
	}
	// Degenerate fallback: a snapshot with no recorded path. Restore the whole
	// tree rather than emit an invalid "<id>:" selector.
	return s.engine.RestoreInclude(ctx, plan.repo, plan.snapshotID, "/", plan.target, plan.mode)
}

// StartRestoreFileSet launches a file-set restore in a background
// goroutine and returns immediately (see StartRestoreToPath for why). All
// validation runs synchronously (a bad request fails right away, no
// goroutine); the resolved alternate target folder ("" for an in-place
// restore) is returned in the ack so the UI can show it. The detached run
// publishes "files:<name>" progress (phase "restore"), registers a cancel
// key, and records a run (kind "restore") against the set's stable id, so
// the outcome, including the real restic error text, lands in the run
// history.
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

// fileSetFilesRestorePlan carries everything prepareRestoreFileSetFiles
// validated and resolved so the restic work can run detached from the
// request that asked for it (StartRestoreFileSetFiles); it is the
// selective (pick-some-files) counterpart of fileSetRestorePlan.
type fileSetFilesRestorePlan struct {
	repo       string
	mode       restic.Mode
	snapshotID string
	setID      string   // runs.target_id the detached run is recorded against
	setName    string   // progress key suffix ("files:<name>")
	paths      []string // cleaned selection, containment-validated
	subtree    string   // the snapshot's own backed-up root (snapshotRestoreRoot); "" = path-less snapshot
	target     string   // to-folder: the resolved alternate folder under the host mount ("" = in-place)
}

// prepareRestoreFileSetFiles performs all of a selective file-set
// restore's validation and resolution synchronously, so a bad request
// fails immediately with a clear error, and creates the alternate target
// folder once containment passes. It is the files-domain counterpart of
// prepareRestoreFiles (containers).
//
// Security: confirm-gated like the container file-level restore; the
// snapshot must belong to this set (tag-scoped via SnapshotsFileSet), and
// buildFileSetFilesPlan applies the hex guard and the containment rules.
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
	// Cheap guards before the snapshot listing: a malformed id or an empty
	// selection must fail fast without a (possibly remote, slow) restic
	// snapshots call. buildFileSetFilesPlan re-checks both, so the shared path
	// stays safe on its own.
	if !backup.ValidSnapshotID(snapshotID) {
		return fileSetFilesRestorePlan{}, backup.ErrInvalidSnapshotID
	}
	if len(filePaths) == 0 {
		return fileSetFilesRestorePlan{}, errors.New("no files selected")
	}
	// Scope to this set: the snapshot must be one of its snapshots.
	snaps, err := s.SnapshotsFileSet(ctx, id, source)
	if err != nil {
		return fileSetFilesRestorePlan{}, err
	}
	settings, err := s.store.GetSettings()
	if err != nil {
		return fileSetFilesRestorePlan{}, fmt.Errorf("read settings: %w", err)
	}
	repo, err := s.fileSetRepoFor(settings, set, source)
	if err != nil {
		return fileSetFilesRestorePlan{}, err
	}
	return s.buildFileSetFilesPlan(snaps, snapshotID, set.ID, set.Name, repo, s.repoModeFor(settings, "files", source, repo), filePaths, targetSubPath)
}

// buildFileSetFilesPlan builds a validated selective plan from already
// resolved snaps, repo, mode and set identity. It is the shared core of
// the local (settings-driven) prepareRestoreFileSetFiles and the foreign
// (session-driven) prepareForeignFileSetFilesRestore, so the containment
// and target guards are written once.
//
// Security: the snapshot id passes the strict hex guard
// (backup.ValidSnapshotID) and must belong to the passed snaps (tag-scoped
// by the caller via SnapshotsFileSet or snapshotsForTag), so one set's
// data can't be extracted through another's route. An empty targetSubPath
// restores in place (each selected path back to its absolute location,
// restic target "/"), so every path is re-validated inside the host mount
// (paths.Within). A non-empty targetSubPath is resolved with paths.Resolve
// under the host mount (rejecting absolute and `..` escapes) and, when the
// snapshot has a backed-up root, every selected path must sit within that
// subtree, since the selection feeds --include patterns. The target dir is
// created (EnsureDirReadable, 0o755) only after all containment passes.
func (s *Service) buildFileSetFilesPlan(snaps []restic.Snapshot, snapshotID, setID, setName, repo string, mode restic.Mode, filePaths []string, targetSubPath string) (fileSetFilesRestorePlan, error) {
	if !backup.ValidSnapshotID(snapshotID) {
		return fileSetFilesRestorePlan{}, backup.ErrInvalidSnapshotID
	}
	if len(filePaths) == 0 {
		return fileSetFilesRestorePlan{}, errors.New("no files selected")
	}

	// Clean each selected path once, so the validated path is the path that runs.
	cleaned := make([]string, 0, len(filePaths))
	for _, p := range filePaths {
		cleaned = append(cleaned, path.Clean(p))
	}

	if !snapshotBelongs(snaps, snapshotID) {
		return fileSetFilesRestorePlan{}, notInListing{snapshotID, "file set"}
	}
	// The subtree comes from the snapshot itself (the node covering all of its
	// recorded paths, so a file under a second recorded root still passes this
	// guard), not a recompute of set.Path, since HostMountRoot may have changed
	// since the backup. A "." or "/" clean means a path-less snapshot (a
	// degenerate discovered set).
	subtree := path.Clean(snapshotRestoreRoot(snaps, snapshotID))
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
		// When the snapshot has a backed-up root, every selection must sit within
		// it: the selection becomes an --include relative to that subtree, so a
		// path outside it would be a client trying to reach beyond the set. A
		// path-less snapshot has no root to scope against; the whole-path include
		// fallback in runRestoreFileSetFiles is contained by --target alone.
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

	// Create the alternate target dir only after every validation passed, with
	// the readable (0o755) variant, so the operator's non-root SMB user can
	// read what root restored to the synced share (see EnsureDirReadable).
	if plan.target != "" {
		if err := paths.EnsureDirReadable(plan.target); err != nil {
			return fileSetFilesRestorePlan{}, fmt.Errorf("create target folder: %w", err)
		}
	}
	return plan, nil
}

// runRestoreFileSetFiles restores each selected path of an
// already-validated selective plan. Like runRestoreFiles (containers) it is
// not atomic, since restic writes per path, so a mid-batch failure says how
// many already went through and which path stopped it.
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

// restoreOneFileSetFile restores a single selected path of an
// already-validated plan. In place (empty target) writes it back to its
// absolute location (RestoreInclude to "/"). To a folder, the restore is
// rooted at the selection's parent and includes only its basename, so it
// lands at <target>/<name> and not <target>/host/user/… (#62's nesting). A
// selection without a usable parent restores its contents straight into
// the target.
func (s *Service) restoreOneFileSetFile(ctx context.Context, plan fileSetFilesRestorePlan, sel string) error {
	if plan.target == "" {
		// In place: restore each selected path back to its own absolute location.
		// Same glob rule as the to-folder branch above: sel is a picked path.
		return s.engine.RestoreInclude(ctx, plan.repo, plan.snapshotID, escapeGlobLiteral(sel), "/", plan.mode)
	}
	// To a folder: root the restore at the selection's immediate parent and
	// include only its basename, so the picked file or folder lands directly
	// as <target>/<name> with no intermediate tree. Rooted at the file set's
	// backed-up root, the whole path between that root and a deeply nested
	// selection would be recreated under the target (selecting
	// dockhand/stacks/DXP480T/xo would give <target>/stacks/DXP480T/xo instead
	// of <target>/xo, #123). restic includes ancestor path nodes, so rooting
	// at the parent is valid for files and folders at any depth.
	parent := path.Dir(sel)
	base := path.Base(sel)
	if parent == "." || parent == "/" || base == "." || base == "/" || base == "" {
		// Degenerate (no usable parent or name): drop the selection's contents in.
		return s.engine.RestoreSubtreeTo(ctx, plan.repo, plan.snapshotID, sel, plan.target, plan.mode)
	}
	// The subtree root travels as a selector (not a pattern) and stays raw;
	// the include is a glob and is escaped.
	return s.engine.RestoreSubtreeInclude(ctx, plan.repo, plan.snapshotID, parent, escapeGlobLiteral("/"+base), plan.target, plan.mode)
}

// StartRestoreFileSetFiles launches a selective file-set restore in a
// background goroutine and returns immediately (see StartRestoreFileSet).
// All validation runs synchronously (a bad request fails right away, no
// goroutine); the resolved alternate target folder ("" for an in-place
// restore) is returned in the ack so the UI can show it. The detached run
// publishes "files:<name>" progress (phase "restore"), registers a cancel
// key, and records a run (kind "restore") against the set's stable id,
// with the same metadata-error tolerance as the whole-set restore
// (concludeFileSetRestore).
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

// DeleteBackupsFileSet removes every backup of a file set. From the local source
// it also forgets the set; from an off-site source it deletes at that target only
// and the set stays.
func (s *Service) DeleteBackupsFileSet(ctx context.Context, id, source string) error {
	// Loaded first for its repository override (#204): this deletes the
	// snapshots of one set, and they live wherever that set backs up. Reading
	// the domain repository would report "no backups to delete yet" for a set
	// whose snapshots sit in its own repo, a refusal that looks like success.
	set, err := s.store.GetFileSet(id)
	if err != nil {
		return errFileSetNotFound
	}
	if isOffsiteSource(source) {
		_, err := s.forgetAtTarget(ctx, "files", "fileset:"+set.Name, source, taggedForItem, nil)
		return err
	}
	settings, err := s.store.GetSettings()
	if err != nil {
		return fmt.Errorf("read settings: %w", err)
	}
	repo, err := s.fileSetRepoPath(settings, set)
	if err != nil {
		return err
	}
	// Refused when this repo is a remote primary flagged append-only in its
	// safety settings (#152), the same gate pruneDomain and DeleteSnapshot use:
	// this path runs Forget with prune, which reclaims space irreversibly.
	if f := s.primaryAppendOnly("files", repo); f != appendOnlyNone {
		return appendOnlyRefusal(f)
	}
	if err := s.requireExistingRepo(repo, "no backups to delete yet"); err != nil {
		return err
	}
	unlock, ok := s.tryLockDomainFor("files", "delete")
	if !ok {
		return errDomainBusy
	}
	defer unlock()
	mode := s.repoModeFor(settings, "files", "local", repo)
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

// DiscoverFileSets rebuilds the file-set list from backup storage, the
// files counterpart of Discover and DiscoverVMs, after a fresh install or
// database loss. Unlike containers and VMs the files domain mirrors no
// definitions to the repo (there is nothing to recreate beyond the
// folder's content), so discovery works from the fileset:<Name> snapshot
// tags alone: every unknown name is stored as a disabled, path-less set,
// because the original source path cannot be known from tags; the UI flags
// "set path before backup" while restore to a folder already works.
//
// An existing set keeps its path, excludes and enabled state: those are
// the operator's own configuration. Its repository is the one exception,
// and only while it is still open or left unread by an earlier pass: such
// a set with no backups where it is pointed now is put back on the
// repository its snapshots were found in, the same repair Discover and
// DiscoverVMs make. The result counts the file sets found. dryRun makes it
// read-only: it lists and counts but writes nothing. The Recovery
// readability probe uses this so it never resurrects orphan entries (#44).
func (s *Service) DiscoverFileSets(ctx context.Context, dryRun bool) (DiscoverResult, error) {
	settings, err := s.store.GetSettings()
	if err != nil {
		return DiscoverResult{}, fmt.Errorf("read settings: %w", err)
	}
	// Every repository this domain writes to (#204), with the one each name was
	// found in; a not-yet-created repo yields nothing. A read failure comes back
	// as readErr together with whatever the named repositories yielded, so an
	// install whose domain repository is unreadable is still rebuilt as far as
	// it can be; the Recovery wizard classifies on that error.
	names, _, skipped, directRows, readErr := s.discoverNamesAcrossRepos(ctx, settings, "files", "fileset:")
	findings, fErr := s.directFindings("files", directRows)
	if fErr != nil {
		log.Printf("api: discover files: could not match direct repositories to targets: %v", fErr)
	}

	res := DiscoverResult{Skipped: skipped, LeftOpen: []string{}, Direct: findings}
	unlock, locked := s.discoverLock("files", dryRun)
	defer unlock()
	for name, repoID := range names {
		// Defense-in-depth: only BombVault's own backups write fileset: tags, but
		// a name that fails the boundary charset (it feeds tags and progress keys)
		// is skipped rather than stored.
		if !validResourceName(name) {
			log.Printf("api: discover files: skipping unsafe file set name %q", name) //nolint:gosec // G706: %q-quoted
			continue
		}
		if dryRun {
			res.Found++ // probe: count what a real discover would surface, write nothing
			continue
		}
		// A set this pass is about to create starts open unless the domain lock
		// was taken, the same rule the existing-row branch below gets from
		// discoverHome, so a create under a running backup is reported in
		// LeftOpen instead of getting its home straight through the insert.
		write := store.HomeWrite{Choice: store.RepoOpen}
		if locked {
			write = s.discoverWrite("files", name, repoID, readErr)
		}
		existing, gErr := s.store.GetFileSetByName(name)
		switch {
		case errors.Is(gErr, sql.ErrNoRows):
			if _, cErr := s.store.CreateFileSet(store.FileSet{Name: name, Enabled: false, Repo: write.Repo, RepoChosen: write.Choice}); cErr != nil {
				log.Printf("api: discover files: could not create set %q: %v", name, cErr) //nolint:gosec // G706: %q-quoted
				continue
			}
			if !locked {
				res.LeftOpen = append(res.LeftOpen, name)
			}
		case gErr != nil:
			log.Printf("api: discover files: could not read set %q: %v", name, gErr) //nolint:gosec // G706: %q-quoted
			continue
		default:
			left, hErr := s.discoverHome(ctx, store.ItemRef{Domain: "files", Key: existing.ID}, repoID, readErr, locked)
			if hErr != nil {
				log.Printf("api: discover files: could not restore the repository of %q: %v", name, hErr) //nolint:gosec // G706: %q-quoted
			}
			if left {
				res.LeftOpen = append(res.LeftOpen, name)
			}
		}
		res.Found++
	}
	if !dryRun && res.Found > 0 {
		if err := s.pauseAfterDiscover(ctx, "files", &res); err != nil {
			readErr = errors.Join(readErr, err)
		}
	}
	return res, readErr
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
	s.registerBackupCancel("config", cancel) // reachable by shutdown
	defer s.unregisterBackupCancel("config")
	defer s.lockDomain("config")() // serialise per repo; blocks maintenance ops meanwhile
	settings, err := s.store.GetSettings()
	if err != nil {
		return backup.Summary{}, fmt.Errorf("read settings: %w", err)
	}
	// Build the consistent staging snapshot of /config; restic backs this up,
	// never the live WAL-mode DB. It is always removed afterwards.
	stagingDir, err := s.stageConfigSnapshot()
	if err != nil {
		return backup.Summary{}, err
	}
	defer func() { _ = os.RemoveAll(stagingDir) }()
	repo, err := s.configRepoPath(settings)
	if err != nil {
		return backup.Summary{}, err
	}
	// issue #152 (bandwidth caps) and #182 (this domain's own credential set)
	mode := s.primaryModeFor(settings, "config", repo)
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
		Runs:      runsAdapter{st: s.store, ctx: ctx, svc: s, cancelKey: "config"},
	})
	s.progEnd("config", "backup", err == nil, startedAt)
	s.notifyBackup(ctx, "config", "", err == nil, sum, err)
	if err != nil {
		return backup.Summary{}, err
	}
	s.applyRetention(ctx, repo, settings, mode, tagIdentity("config"), "config")
	s.replicateOffsite(ctx, "config", settings, repo, "")
	s.collectStatsAfterItem(ctx, "config")
	s.checkPrimaryRemoteBudget(ctx, "config", repo, settings)
	return sum, nil
}

// FlashDownloadName is the suggested filename for a flash zip download.
func FlashDownloadName(id string) string { return "flash-" + id + ".zip" }

// resolveFlashSnapshot maps a user-supplied selector ("" / "latest", a
// full id, or a short prefix) to the single matching full snapshot id. It
// errors when the selector matches none or is an ambiguous prefix of more
// than one, so the caller rejects it before any download bytes or headers
// are committed, and restic always receives an unambiguous full id.
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
		return "", notInListing{id: selector}
	}
	return match, nil
}

// DownloadFlashZip streams a flash snapshot to w as a zip (restic dump),
// the non-destructive flash restore: the live /boot is never touched and no
// filesystem metadata is restored (so it can't hit the per-file permission
// errors a to-disk restore gets on /mnt/user), and via dumpFlashZipCompat
// the file drops straight into the Unraid USB creator, which restic's own
// zip dump does not (#136).
//
// "latest"/"" resolves to the newest snapshot; an explicit id is validated
// against the repo. onResolved (optional) is called with the concrete id
// once it is known-good and before streaming begins, so the HTTP handler
// sets the download headers only on the happy path. A restore run is
// recorded for history.
func (s *Service) DownloadFlashZip(ctx context.Context, snapshotID, source string, onResolved func(id string), w io.Writer) error {
	settings, err := s.store.GetSettings()
	if err != nil {
		return fmt.Errorf("read settings: %w", err)
	}
	// Resolve age recipients up front: with export encryption on but no valid
	// recipient the download fails before onResolved fires (no headers) and
	// before any bytes are streamed, so a plaintext zip is never sent.
	recipients, encOn, err := s.exportRecipients(settings)
	if err != nil {
		return err
	}
	repo, err := s.repoFor(settings, "flash", source)
	if err != nil {
		return err
	}
	mode := s.repoModeFor(settings, "flash", source, repo)
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
	// With encryption on, wrap the response writer so the streamed zip is
	// age-sealed on the fly. The age writer has to be closed to finalize the
	// stream.
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
		// A client disconnect or user cancel of the download is context.Canceled;
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
	mode := s.repoModeFor(settings, "flash", source, repo)
	if localRepoMissing(repo) {
		// #55 vs #120: only surface "not mounted" when the backing store is truly
		// absent. If the destination is mounted, this is a fresh or phantom repo
		// on a healthy disk, so report an empty list (EnsureRepo re-establishes on
		// write).
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
		return "", notInListing{id: selector}
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
	mode := s.repoModeFor(settings, "config", source, repo)
	if localRepoMissing(repo) {
		// #55 vs #120: only surface "not mounted" when the backing store is truly
		// absent. If the destination is mounted, this is a fresh or phantom repo
		// on a healthy disk, so report an empty list (EnsureRepo re-establishes on
		// write).
		if s.repoEstablished(repo) && !s.destinationMounted(repo) {
			return nil, ErrBackupPathNotMounted // #55: backing store not mounted
		}
		return nil, nil // no backups yet
	}
	return s.listSnapshots(ctx, repo, mode)
}

// RestoreConfig stages a restore of BombVault's own /config: it cannot
// overwrite the live SQLite settings DB in place while this process holds
// it open (WAL), so it restic-restores the chosen snapshot into a staging
// root and writes a marker. The boot-time selfrestore.ApplyPending (called
// from main before store.Open on the next restart) performs the file-level
// staging→live swap. The restart is triggered separately (docker
// self-restart or manual), so this call only stages.
func (s *Service) RestoreConfig(ctx context.Context, snapshotID, source string) error {
	settings, err := s.store.GetSettings()
	if err != nil {
		return fmt.Errorf("read settings: %w", err)
	}
	repo, err := s.repoFor(settings, "config", source)
	if err != nil {
		return err
	}
	mode := s.repoModeFor(settings, "config", source, repo)
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
	// Also clear any stale <root>.bad left by a failed restore on a prior
	// boot: it contains a plaintext rclone.conf and ssh private key, so a fresh
	// attempt should not let it linger. Best-effort; a leftover .bad must
	// never block a restore.
	_ = os.RemoveAll(root + ".bad")
	runID, err := s.store.StartRun(store.ConfigTargetID, "restore")
	if err != nil {
		return fmt.Errorf("config restore: start run: %w", err)
	}
	// Restore only the config snapshot's subtree (<DataDir>/.snapshot) into
	// the staging root; restic recreates that absolute path beneath --target,
	// landing it at selfrestore.RestoredSnapshotDir(DataDir), the exact path
	// the boot swap reads. The swap applies it on the next restart.
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

// StartRestoreConfig stages a restore of BombVault's own /config and, on
// success, triggers the self-restart that applies it on the next boot. It
// takes the shared single-flight guard (batchActive), so it never overlaps
// another backup or restore; a config self-restart would otherwise kill
// the container mid-write of an in-flight data restore. It returns
// (started, autoRestart, err): started=false with a nil err means another
// operation is already running; autoRestart=false means the caller must
// ask the user to restart the container manually. When an auto-restart is
// scheduled the guard stays held until the container goes down, so nothing
// new starts in the restart window; if the restart fails,
// ScheduleSelfRestart releases it.
//
// Unlike the other Start* restores, this one does not hand the restore to
// a background goroutine: the caller (the restore-own-config step in
// Recovery.tsx) needs the staged/autoRestart outcome synchronously to
// decide between polling for the self-restart and showing manual-restart
// instructions. Like RunRestoreDrill, RestoreConfig runs on this goroutine
// against a context detached from ctx (context.WithoutCancel) and capped
// by restoreTimeout, so a closed tab or a proxy idle timeout cannot kill a
// restore mid-write.
//
// A truncated restore does not corrupt the live config on the next boot:
// selfrestore.ApplyPending is marker-gated and runs validSQLite (PRAGMA
// quick_check) on the staged DB before touching the live one, moving a bad
// staged DB to <root>.bad. The remaining risk is narrower: RestoreConfig
// clears the staging dir but not a stale marker, so a successful restore
// (autoRestart=false, marker pending until a manual restart) followed by a
// failed one can leave the marker pointing at partial staging, and
// ApplyPending checks only the existence of rclone.conf and ssh/, not
// their content. On a client disconnect during a config restore, the
// restore still completes and self-restarts, but the SPA's fetch reports a
// failure; fixing that would mean reporting the outcome by polling or SSE
// instead of the HTTP response. A panic is recovered like every other
// manual op: RestoreConfig's run id is local to it, so the fallback is
// FailRunningRun keyed by store.ConfigTargetID.
func (s *Service) StartRestoreConfig(ctx context.Context, snapshotID, source string) (started bool, autoRestart bool, err error) {
	if !s.batchActive.CompareAndSwap(false, true) {
		return false, false, nil
	}
	if op, busy := s.domainBusy("config"); busy {
		s.batchActive.Store(false)
		return false, false, fmt.Errorf("%s is running on config", op)
	}
	// On a recovered panic the guard has to be released here as well: unlike
	// the success path below, nothing else left running would release it, and
	// every later backup or restore would refuse forever with "already
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

// SetFileSetScheduleCadence writes a folder set's per-item schedule
// override (#199), validating it the same way the container and VM
// setters do.
//
// "everyN" is refused for the same reason it is refused there:
// classifyItemOverride maps an interval cadence back to the domain default
// rather than giving the item its own entry, so accepting one here would
// store a value that silently does nothing. Better to say so at the point
// of entry than to have somebody discover it from a backup that never ran.
func (s *Service) SetFileSetScheduleCadence(_ context.Context, id, cadence string) error {
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
	return s.store.SetFileSetScheduleCadence(id, cadence)
}

// SetVMScheduleCadence sets a VM's per-item schedule override (#121),
// creating the target if absent. The cadence is validated with the
// domain-schedule grammar; an empty string clears the override. everyN is
// rejected (no per-item last-run gate), like SetScheduleCadence.
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

// SetVMIncludeAll sets the include_in_schedule flag for every VM on the
// host in one call, the VM counterpart to SetIncludeAll. It iterates the
// live VMs reported by virsh and ensures a target row exists for each
// (find or create, as SetVMInclude does). Excluding then applies to every
// already-known VM target too, so an orphan VM that still has backups
// comes off the schedule. De-duplicated so a VM that is both live and a
// known target is only set once.
//
// Including stops at the live VMs (#232): putting every deleted VM back on
// the schedule would make each run try it again and log a skip. Same rule
// as SetIncludeAll on containers.
func (s *Service) SetVMIncludeAll(ctx context.Context, include bool) error {
	infos, err := s.virsh.List(ctx)
	if err != nil {
		return fmt.Errorf("list vms: %w", err)
	}
	live := make(map[string]bool, len(infos))
	for _, vm := range infos {
		live[vm.Name] = true
		if err := s.SetVMInclude(ctx, vm.Name, include); err != nil {
			return err
		}
	}
	if include {
		return nil
	}
	// Known targets whose VM is no longer defined on the host (orphans with
	// backups). The find-or-create in SetVMInclude already handles existing
	// rows, so a plain store update is enough here.
	targets, err := s.store.ListVMTargets()
	if err != nil {
		return fmt.Errorf("list vm targets: %w", err)
	}
	for _, t := range targets {
		if live[t.Name] || !t.IncludeInSchedule {
			continue
		}
		if err := s.store.SetVMInclude(t.Name, false); err != nil {
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

// CheckDomain verifies the integrity of a domain's restic repo (restic check).
// domain is "containers" | "vms" | "flash" | "files". Returns a friendly error
// when the repo has not been created yet. Bounded by a timeout so a huge repo
// can't hang the request forever.
func (s *Service) CheckDomain(ctx context.Context, domain, source string) (err error) {
	// Every repository this domain's items write to (#204), not just its own.
	// Verifying the domain repository while an item's data sits in a named one
	// would give a green tick about a repository the data is not in.
	settings, repos, skipped, err := s.domainReposForOp(domain, source)
	if err != nil {
		return err
	}
	repos, missing, err := s.reposThatExist(repos, "no backups to verify yet")
	if err != nil {
		return err
	}
	skipped = append(skipped, missing...)
	// Hold the in-process domain lock for the whole verify so no other
	// BombVault op (backup, prune, replicate) runs against this repo during
	// the check. If one already holds it, report a clean "busy" instead of
	// colliding on restic's repo lock. This rules out BombVault itself as the
	// source of a lock the check hits, but not a live restic process (a manual
	// or external invocation); that is a real, live lock, not an orphan, and
	// it is left alone below (waited out by --retry-lock in the engine, never
	// force-removed).
	unlock, ok := s.tryLockDomainFor(domain, "verify")
	if !ok {
		return errDomainBusy
	}
	defer unlock()
	// The ceiling is per repository, multiplied up so the whole pass fits; a
	// single ceiling would cut a two-repository domain off half way through
	// the second one with a timeout naming neither. The multiplication is
	// bounded by the number of repositories the domain has, which is the
	// number of rows an operator created.
	ctx, cancel := context.WithTimeout(ctx, time.Duration(len(repos))*15*time.Minute)
	defer cancel()
	// Publish a "maintenance" progress pair (begin and terminal, indeterminate,
	// since restic check streams no percentage) and record a "verify" run, so
	// a manual or scheduled verify shows up on the dashboard activity log and
	// run history instead of running invisibly.
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

	// Clear a genuinely stale orphan before `restic check` takes its lock:
	// unlockStale runs plain `restic unlock`, which removes only locks restic
	// itself deems stale (a dead PID on this host, or any lock past restic's
	// ~30-minute age threshold). A live or concurrent lock is never
	// force-removed: the domain lock is held for the whole verify, so no other
	// BombVault op can collide, and `restic check` passes --retry-lock to wait
	// out a transient cross-process lock. A known, bounded gap: an orphan from
	// a prior container incarnation carries that container's random hostname,
	// so restic can't PID-probe it and calls it stale only at ~30 minutes old;
	// until then check fails "already locked" (it heals itself, or a manual
	// Unlock clears it). A stable container hostname would close this.
	//
	// Each repository in turn, under the one domain lock. The first failure is
	// the answer: a domain whose data is spread over a domain repository and
	// named ones is only verified when every one of them is. The mode is built
	// per repository, because a named repository carries its own credentials,
	// storage class and bandwidth caps.
	for _, r := range repos {
		rMode := s.repoModeFor(settings, domain, source, r.Loc)
		s.unlockStale(ctx, r.Loc, rMode)
		if err = s.engine.Check(ctx, r.Loc, rMode); err != nil {
			err = fmt.Errorf("verifying %s: %w", s.refName(r), err)
			return err
		}
	}
	// Everything that was reachable verified clean. That is not a success
	// while part of the domain was never opened, so the run records what was
	// missed instead of a green tick over the remainder.
	err = skippedError("this verify", skipped)
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
// wait selects the lock discipline for the underlying drill: a scheduled
// drill passes wait=true so it blocks for the per-domain lock and always
// records a result (a nightly backup or replication firing at the same
// time must not make it vanish and leave the dashboard at "never"); a
// manual drill passes wait=false for immediate errDomainBusy feedback and
// records nothing (#30).
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

// runSubsetDrill proves a domain's backup is restorable by running `restic
// check --read-data-subset` (it reads back and re-verifies a random subset
// of the real pack data, not just metadata, with no scratch disk needed)
// and records the result. domain is {containers,vms,flash,config,files};
// source is {local,offsite}.
//
// It takes the per-domain busy guard like Prune and Unlock: if a backup is
// running it returns errDomainBusy and records nothing.
//
// A repository that was never created returns a clear "no backups to
// verify" error and records nothing: there is nothing to be red about. A
// repository that was there and is gone records a failed row, or the
// restorability badge would keep last week's green tick over a share that
// stopped mounting.
//
// Passing and failing drills are both recorded. The notification is
// confined to the scheduled pass (wait), at all four sites: a manual press
// puts the answer on the screen of whoever pressed it, and
// notifyDrillFailure has no throttle, so repeated presses while somebody
// diagnoses an unmounted share would send a mail each time.
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

	// Every repository this domain's items write to (#204). A drill reads real
	// pack data back; reading it out of the domain repository while an item's
	// data sits in a named one would verify the wrong bytes and hand back a
	// green badge for it.
	settings, repos, skipped, err := s.domainReposForOp(domain, source)
	if err != nil {
		return store.RestoreDrill{}, err
	}
	repos, missing, err := s.reposThatExist(repos, "no backups to verify yet")
	if err != nil {
		// The all-missing case gets the same treatment as the read-nothing case
		// below, for the same reason: returning bare records no row, and the
		// restorability badge keeps last week's green tick while the share it
		// verified stopped mounting.
		//
		// "Never created yet" is not that case: reposThatExist reports it with an
		// empty skip list, and a domain that has never been backed up has nothing
		// to be red about.
		if len(missing) > 0 {
			rec := store.RestoreDrill{
				Domain: domain, Source: source, Kind: "subset",
				At: time.Now().Unix(), OK: false, Detail: truncateRunErr(err),
			}
			if aErr := s.store.AddRestoreDrill(rec); aErr != nil {
				log.Printf("api: drill: record unreachable-domain result for %q: %v", domain, aErr) //nolint:gosec // G706: domain is %q-quoted
			}
			s.recordDomainRun(domain, "drill", false, rec.Detail)
			// Notify only on the scheduled pass, like every other notification in
			// this function (see the doc comment).
			if wait {
				s.notifyDrillFailure(ctx, domain, source, rec.Detail)
			}
			return rec, err
		}
		return store.RestoreDrill{}, err
	}
	skipped = append(skipped, missing...)

	// Serialise with backups (and other maintenance) so a drill never reads a
	// repo a backup is writing. A scheduled drill (wait) blocks for the domain
	// so it always records a result even when a nightly backup or replication
	// fires at the same time; a manual drill fails fast with immediate busy
	// feedback without recording (#30).
	var unlock func()
	if wait {
		// Bounded wait: poll the lock up to drillLockWait, then log and skip, so a
		// wedged lock-holder can't block a scheduled drill forever or pile up a
		// goroutine each night.
		u, ok := s.waitLockDomainFor(domain, "verify")
		if !ok {
			log.Printf("api: drill: %q busy longer than %v, skipping this scheduled run", domain, drillLockWait) //nolint:gosec // G706: domain is %q-quoted and validated to a fixed allow-list above
			// Record the skip as a dated failed row, so the dashboard shows why the
			// check did not run rather than freezing the previous red with no reason
			// (#30).
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

	// Publish a live "maintenance" progress pair keyed "drill:<domain>" (like
	// prune: and verify:) so a running restore-verification drill shows on the
	// dashboard activity log while it reads back pack data, not only after it
	// finished (#109). The terminal event is deferred so an error or panic can
	// never leave a stuck live line.
	dkey := "drill:" + domain
	_, startedAt := s.progBegin(ctx, dkey, "maintenance")
	defer func() { s.progEnd(dkey, "maintenance", err == nil, startedAt) }()

	// Reading back a subset of real pack data can be slow on a large repo, so
	// the whole pass over the domain's repositories is bounded.
	//
	// ctx is rebound rather than given a second name, as in CheckDomain,
	// UnlockDomain and pruneDomain, so the snapshot listing and the stale-lock
	// clear inside the loop are under the ceiling too. A scheduled drill
	// enters on context.Background() holding the domain lock, so without the
	// ceiling one unreachable remote repository could park the listing forever
	// and leave every other operation on the domain answering "busy".
	//
	// outer is kept because the failure notification must not travel on the
	// drill's own deadline: the failure a drill most needs to announce is the
	// one where it ran out of time.
	outer := ctx
	ctx, cancel := context.WithTimeout(ctx, time.Duration(len(repos))*2*time.Hour)
	defer cancel()
	pct := drillSubsetPct(settings.DrillsSubsetPct)
	var checkErr error
	drilled := 0
	for _, r := range repos {
		rMode := s.repoModeFor(settings, domain, source, r.Loc)
		// An initialised but empty repo (no snapshots) has nothing to verify.
		// Treat it like a missing repo: skipped here, and a clear error below when
		// none of them had anything, with no misleading failure recorded either
		// way.
		snaps, sErr := s.listSnapshots(ctx, r.Loc, rMode)
		if sErr != nil {
			// One unreadable repository is a skip, not the end of the pass: returning
			// would record nothing and leave the badge frozen on its previous verdict
			// for every other repository of the domain too. restic's own sentence goes
			// into the reason, since "could not be listed" alone does not say whether
			// the share is gone or the password wrong.
			skipped = append(skipped, repoSkip{Name: s.refName(r), Reason: scrubError(sErr), Unreachable: true})
			continue
		}
		if len(snaps) == 0 {
			continue
		}
		drilled++
		// Clear any stale lock a previously interrupted off-site op (replication
		// copy or integrity check) left behind before `restic check
		// --read-data-subset` takes its lock, so a drill can't fail "repository is
		// already locked"; BombVault is the sole writer, so an existing lock is
		// always stale (as in CheckDomain, #29).
		s.unlockStale(ctx, r.Loc, rMode)
		if checkErr = s.engine.CheckData(ctx, r.Loc, pct, rMode); checkErr != nil {
			checkErr = fmt.Errorf("reading back %s: %w", s.refName(r), checkErr)
			break
		}
	}
	if drilled == 0 {
		// Nothing could be read back. If that is because something was
		// unreachable rather than empty, it is a failed drill and has to be
		// recorded as one: returning without a row would leave the restorability
		// badge frozen on its previous verdict, and a domain nobody can read any
		// more would keep last week's green tick. The busy-skip branch above
		// records its reason for the same purpose.
		if sErr := skippedError("this restorability check", skipped); sErr != nil {
			rec := store.RestoreDrill{
				Domain: domain, Source: source, Kind: "subset",
				At: time.Now().Unix(), OK: false, Detail: truncateRunErr(sErr),
			}
			if aErr := s.store.AddRestoreDrill(rec); aErr != nil {
				log.Printf("api: drill: record unreadable-domain result for %q: %v", domain, aErr) //nolint:gosec // G706: domain is %q-quoted
			}
			s.recordDomainRun(domain, "drill", false, rec.Detail)
			// Scheduled pass only, like the other notifications in this function. It
			// matters most here: the unreadable-share branch is exactly the state
			// somebody presses the button at repeatedly while diagnosing it.
			if wait {
				s.notifyDrillFailure(outer, domain, source, rec.Detail)
			}
			return rec, sErr
		}
		return store.RestoreDrill{}, errors.New("no backups to verify yet")
	}
	// A pass that read back everything it could reach, but could not reach all of
	// the domain, is not a passing drill: the badge would be green about data
	// nobody opened.
	if checkErr == nil {
		checkErr = skippedError("this restorability check", skipped)
	}

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
		// Recording is what a drill is for; surface a record failure.
		return store.RestoreDrill{}, fmt.Errorf("record drill: %w", recErr)
	}
	// Mirror the drill outcome into the shared runs table so it shows in the
	// dashboard Activity Log/Run History (the restore_drills row above stays the
	// badge/scorecard source of truth).
	s.recordDomainRun(domain, "drill", drill.OK, drill.Detail)
	// A failed restorability check is important, so notify on failure
	// (best-effort), on the scheduled pass only like the other three in this
	// function. A manual press puts the same answer on the screen of whoever
	// pressed it.
	if checkErr != nil && wait {
		s.notifyDrillFailure(outer, domain, source, drill.Detail)
	}
	return drill, checkErr
}

// drillMarkerName is the sentinel file written into a DR-drill sandbox at
// creation. Cleanup deletes a sandbox only when this marker is present in
// that exact directory, a safety interlock so a drill can never
// os.RemoveAll a path that is not a marked drill sandbox of its own.
const drillMarkerName = ".bombvault-drill"

// drillByteToleranceFloor is the only slack allowed between restic's
// reported restore-size and the on-disk restore of a DR drill: a tight
// few-KB absolute floor for filesystem metadata rounding, not a percentage
// of the total. `restic restore` is content-addressed, so a completed
// restore reproduces the exact logical bytes; a percentage band (e.g. 5%
// of the whole snapshot) would wave through a large file restored
// truncated by less than that fraction, with the file count unchanged. The
// count must match exactly and the bytes to within this floor.
const drillByteToleranceFloor = 4096

// drillSnapshotTimeout bounds the DR-drill's off-site snapshot listing so a
// black-holed off-site (a `restic snapshots` that hangs on a dead network) can't
// hold the domain lock indefinitely and starve a concurrent scheduled backup. The
// restore itself is bounded separately (restoreTimeout), matching a real restore.
const drillSnapshotTimeout = 15 * time.Minute

// errNothingToDrill signals that the newest off-site snapshot has no
// restorable file data (0 files, 0 bytes, e.g. a definition-only or
// stateless container). A DR drill then records nothing: a green would be
// a false "verified restorable" and a red a false failure.
var errNothingToDrill = errors.New("no restorable file data in the newest off-site snapshot: nothing to drill")

// runDRDrill performs a real off-site disaster-recovery drill for a domain: it
// restores the newest off-site snapshot of the drill target into a marker-guarded
// sandbox under the restore folder, verifies the restored file count + bytes
// against restic's own accounting, deletes the sandbox (marker-guarded), and
// records a restore_drills(kind='dr', source='offsite') row. It takes the domain
// repo lock exactly like a real restore, so a scheduled backup can never fire
// mid-drill and vice-versa; busy → errDomainBusy, recording nothing. A failure
// records kind='dr' ok=false and fires the drill-failure notification.
func (s *Service) runDRDrill(ctx context.Context, domain, source string, wait bool) (drill store.RestoreDrill, err error) {
	switch domain {
	case "containers", "vms", "flash", "files":
	default:
		return store.RestoreDrill{}, fmt.Errorf("unknown domain %q", domain)
	}
	// A DR drill only ever restores from an off-site repo. A non-offsite
	// source (the scheduler's legacy call, or "local") normalises to the bare
	// "offsite" primary; an "offsite:<id>" source drills, and records under,
	// that specific destination.
	if !isOffsiteSource(source) {
		source = "offsite"
	}

	settings, err := s.store.GetSettings()
	if err != nil {
		return store.RestoreDrill{}, fmt.Errorf("read settings: %w", err)
	}
	target, err := s.offsiteTargetForSource(settings, domain, source)
	if err != nil {
		return store.RestoreDrill{}, err
	}
	repo, err := s.resolveRepo(target.Repo)
	if err != nil {
		return store.RestoreDrill{}, err
	}

	// Serialise with backups and restores on the domain repo: a scheduled
	// backup must never fire mid-drill and vice versa. A scheduled drill
	// (wait) blocks for the domain so it always records a result even when a
	// nightly op fires at the same time; a manual drill fails fast with
	// immediate busy feedback without recording (#30).
	var unlock func()
	if wait {
		// Bounded wait: poll the lock up to drillLockWait, then log and skip, so a
		// wedged lock-holder can't block a scheduled drill forever or pile up a
		// goroutine each night.
		u, ok := s.waitLockDomainFor(domain, "verify")
		if !ok {
			log.Printf("api: drill: %q busy longer than %v, skipping this scheduled run", domain, drillLockWait) //nolint:gosec // G706: domain is %q-quoted and validated to a fixed allow-list above
			// Record the skip as a dated failed row, so the dashboard shows why the
			// off-site DR check did not run rather than freezing the red with no
			// reason (#30).
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
	// (like prune: and verify:) so a running DR drill shows on the dashboard
	// activity log while it sandbox-restores, not only after it finished
	// (#109). The "drdrill" key and kind are distinct from the local subset
	// drill's "drill", so the log tells the off-site DR restore check apart
	// from the local read-back check. The terminal event is deferred so an
	// error or panic can never leave a stuck live line.
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
	// The destination is described by the target row already in hand above.
	// offsiteModeForTarget is the builder copyToOffsiteTarget uses to write
	// this same repository; opened with the shared mode, a destination with
	// its own credentials could be written and never read back, and the drill
	// could not pass.
	mode := s.offsiteModeForTarget(settings, target)
	listCtx, listCancel := context.WithTimeout(drillCtx, drillSnapshotTimeout)
	snapID, err := s.pickDRSnapshot(listCtx, domain, settings, repo, mode)
	listCancel()
	if err != nil {
		return store.RestoreDrill{}, err
	}

	// Clear any stale lock a previously interrupted off-site op left behind
	// before the sandbox restore takes its lock, so a drill can't fail
	// "repository is already locked"; BombVault is the sole writer, so an
	// existing lock is always stale (as in CheckDomain, #29). drillCtx
	// (detached) matches the restore below.
	s.unlockStale(drillCtx, repo, mode)
	// Restore into the sandbox, verify, and clean up (marker-guarded). The
	// outcome is recorded either way; a failure also notifies. An empty
	// (0-file, 0-byte) snapshot records nothing, neither a false green nor a
	// false red.
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
	// dashboard Activity Log and Run History (the restore_drills row above
	// stays the badge and scorecard source of truth). Kind "drdrill", distinct
	// from the local subset drill's "drill", so the log names the off-site DR
	// restore check.
	s.recordDomainRun(domain, "drdrill", drill.OK, drill.Detail)
	if drillErr != nil {
		s.notifyDrillFailure(ctx, domain, source, drill.Detail)
	}
	return drill, drillErr
}

// pickDRSnapshot resolves the newest off-site snapshot to drill for a
// domain. containers: the DRDrillTarget container (or, when unset, the
// most recently backed-up container), scoped to its container:<name> tag.
// vms: the DRDrillTargetVM VM (or, when unset, the most recently
// backed-up VM), scoped to its vm:<name> tag, the same pattern. flash: the
// newest snapshot outright (flash is a single whole-USB image, no per-item
// scoping). files follows flash, the newest snapshot in the files repo
// outright, since a file-set restore is sandbox-cheap and any set proves
// the repo restorable. An empty repo or a target with no off-site snapshot
// yields a clear error.
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
// successful backup run, the default DR-drill target when none is pinned.
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

// newestBackedUpVM returns the VM name with the most recent successful
// backup run, the default DR-drill target when none is pinned. It mirrors
// newestBackedUpContainer (VM runs share the same runs table and column,
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
// marker-guarded sandbox under the restore folder, verifies the restored
// files and bytes against restic's own accounting, and always attempts
// marker-guarded cleanup. A mismatch or restore/verify error is returned;
// the sandbox is still removed. It reuses the RestoreInclude machinery and
// paths.Resolve containment of a real restore to a folder.
func (s *Service) sandboxRestoreVerify(ctx context.Context, domain string, settings store.Settings, repo, snapID string, mode restic.Mode) error {
	sub := path.Join(settings.RestoreFolder, fmt.Sprintf("bombvault-drill-%s-%d", domain, time.Now().UnixNano()))
	sandbox, err := paths.Resolve(s.cfg.HostMountRoot, sub)
	if err != nil {
		return errors.New("invalid restore folder: must be a relative subpath under the host mount")
	}
	// Create the parent (restore folder), then the sandbox leaf with os.Mkdir,
	// which fails if it already exists: a positive assertion that this is a
	// fresh directory before it becomes a marker-guarded RemoveAll target
	// (MkdirAll would silently adopt a pre-existing directory).
	if err := paths.EnsureDir(filepath.Dir(sandbox)); err != nil {
		return fmt.Errorf("create drill sandbox parent: %w", err)
	}
	if err := os.Mkdir(sandbox, 0o700); err != nil { //nolint:gosec // G703: sandbox is resolved strictly under the host mount root by paths.Resolve
		return fmt.Errorf("create drill sandbox: %w", err)
	}
	// Marker first, before any restore, so the cleanup interlock can always
	// confirm this is a sandbox of its own, even if the restore fails midway.
	// If the marker write itself fails the (still empty) dir would leak, so it
	// is removed explicitly on that path before the cleanup defer is
	// registered.
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

	// Bound the restore at restoreTimeout, like a real restore: reading a
	// whole snapshot back over a slow off-site link can take many hours, far
	// more than a short drill-only deadline. ctx is already detached from the
	// request by the caller (runDRDrill), so a browser tab close can't abort a
	// legitimate drill.
	dctx, cancel := context.WithTimeout(ctx, restoreTimeout)
	defer cancel()

	// A VM disk image (or any large snapshot) can be hundreds of GB; fail fast
	// with a clear error if the sandbox filesystem doesn't have room, rather
	// than letting the restore run the host out of disk midway. The same
	// preflight idiom as guardVMRestoreDestination and
	// guardContainerRestoreDestination: a stats or probe error (e.g. the
	// unsupported-platform stub in diskfree_other.go) is "cannot prove
	// insufficient" and never blocks the drill; only a proven shortfall
	// aborts.
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
	// A snapshot with no restorable file data exercises no real restore, and
	// recording it green would be a false "verified restorable". Signal
	// "nothing to drill" so the caller records neither green nor red.
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
// accounting. The completeness proof is the on-disk restore versus restic:
//
//   - the on-disk file count == restic ls file count (a completed restore
//     materialises every file node restic recorded);
//   - the on-disk bytes == restic's restore-size bytes to within
//     drillByteToleranceFloor (a few KB for fs metadata), not a
//     percentage, since a content-addressed restore reproduces the exact
//     logical bytes.
//
// restic's `stats --mode restore-size` file count is not compared against
// `ls`: the two counters legitimately differ on real snapshots (hardlinks,
// and how restore-size tallies files versus how ls enumerates nodes), so
// requiring statsFiles == lsFiles would flag perfectly restorable backups,
// such as an Unraid flash restoring the exact file count and bytes (#30).
// A truncated restore is already caught by gotFiles != lsFiles and the
// byte check, which both measure the restored sandbox; statsFiles measures
// neither and could only add false negatives.
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

// cleanupDrillSandbox removes a DR-drill sandbox, but only after
// confirming the .bombvault-drill marker written at creation is present in
// that exact directory. This is a safety-critical interlock: os.RemoveAll
// is destructive, so a drill must never delete a path that is not a marked
// sandbox (e.g. a mis-resolved or operator-configured folder). A missing
// marker removes nothing and returns an error.
func cleanupDrillSandbox(sandbox string) error {
	if _, err := os.Stat(filepath.Join(sandbox, drillMarkerName)); err != nil { //nolint:gosec // G703: sandbox is resolved strictly under the host mount root by paths.Resolve; this stat is the marker interlock itself
		return fmt.Errorf("drill sandbox %q lacks the %s marker; refusing to delete", filepath.Base(sandbox), drillMarkerName)
	}
	return os.RemoveAll(sandbox) //nolint:gosec // G703: sandbox is under the host mount root (paths.Resolve) and guarded above by the .bombvault-drill marker, so it never removes a non-drill path
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
// drill fails (the backup is not provably restorable). Mirrors notifyBackup's
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
	msg := fmt.Sprintf("Restore verification of %s (%s) FAILED, so the backup may not be restorable: %s", target, source, detail)
	notify.Send(ctx, c, domain, notify.Event{Title: "BombVault", Message: msg, OK: false})
	if s.unraidGate(c.Unraid) {
		if e := s.sendUnraidNotify(ctx, "BombVault: restore verification FAILED", msg, "warning"); e != nil {
			log.Printf("notify: unraid: %v", e)
		}
	}
}

// repoFor resolves the restic repo of a domain ("containers", "vms", "flash",
// "config" or "files") and source. An off-site source is the target
// offsiteTargetForSource resolves and fails where that refuses; anything else
// ("" or "local") is the domain's own repo.
func (s *Service) repoFor(settings store.Settings, domain, source string) (string, error) {
	if isOffsiteSource(source) {
		target, err := s.offsiteTargetForSource(settings, domain, source)
		if err != nil {
			return "", err
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

// domainRepoSource resolves one repository for a domain and source
// ("local"|"offsite"), returning the settings alongside it so callers
// don't re-read them.
//
// An operation that has to reach all of a domain's data wants
// domainReposForOp instead; this one is for the paths that address a
// single repository.
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

// recordDomainRun persists an already completed domain-scoped check (restore
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

// localRepoMissing reports whether a local repo has not been initialised
// yet (no `config` marker). It is always false for a remote repo
// (rest:/s3:/b2:/…), which has no local marker to stat; its emptiness is
// decided by listing it. The off-site view (often a remote repo) therefore
// must not use a local config check, or it would always look empty even
// when snapshots exist.
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

// reposThatExist narrows a domain's repositories (domainReposForOp) to the
// ones that are there, and fails with the caller's "not yet" sentence only
// when none of them is.
//
// With named repositories (#204), "the repository is missing" and "there
// is nothing to work on" are different sentences: a domain whose items
// all sit on named repositories may have no domain repository on disk at
// all, and refusing the whole operation over that would leave those
// repositories unverified, unpruned and locked.
// A repository that is missing while others are present is reported as a
// skip, for the same reason a switched-off one is: "the share was not
// mounted when the nightly verify ran" and "this repository was never
// created" are different facts, and only the second one is harmless.
func (s *Service) reposThatExist(repos []domainRepoRef, notYet string) ([]domainRepoRef, []repoSkip, error) {
	out := make([]domainRepoRef, 0, len(repos))
	var skipped []repoSkip
	var first error
	for _, r := range repos {
		if err := s.requireExistingRepo(r.Loc, notYet); err != nil {
			if first == nil {
				first = err
			}
			// Never created is silent; was there and is gone is reported, and so is
			// "don't know".
			//
			// An empty folder holds no backups, so a repository that was never created
			// is not a skip: pointing one container at a named repository before the
			// domain was ever backed up leaves the domain's own repository permanently
			// "not present", and verify, prune, unlock and the drill would report an
			// incomplete pass every night over repositories that are all fine.
			//
			// An unknown answer goes to the reported side, matching the other two
			// unknown rules in this file (an unreadable in-use count counts as in use;
			// an unreadable domain list counts as shared). Silence is the answer that
			// loses information, so it is not the one an error gets.
			switch s.repoEstablishmentOf(r.Loc) {
			case repoWasEstablished:
				skipped = append(skipped, repoSkip{Name: s.refName(r), Reason: "it was there before and is not reachable now", Unreachable: true})
			case repoEstablishmentUnknown:
				skipped = append(skipped, repoSkip{Name: s.refName(r), Reason: "it is not reachable now, and whether it ever held backups could not be read", Unreachable: true})
			case repoNeverEstablished:
			}
			continue
		}
		out = append(out, r)
	}
	if len(out) == 0 {
		if first == nil {
			first = errors.New(notYet)
		}
		// The skip list travels with the error: "nothing to verify yet" and "the
		// one repository this domain has went away" are different messages, and
		// only the second one asks somebody to act; without it an all-named domain
		// would be told it has no backups at all.
		//
		// nothingCoveredError, not skippedError: nothing was covered here, and
		// `notYet` is a whole sentence rather than a noun phrase, so the other
		// template would read "no backups to verify yet covered only part of this
		// domain: …".
		if sErr := nothingCoveredError(skipped); sErr != nil {
			return nil, skipped, sErr
		}
		return nil, skipped, first
	}
	return out, skipped, nil
}

// shortRepoName names a repository in a message without printing a full
// host path: the last segment, or the remote's scheme, is what a reader
// recognises, and the error scrubber would redact the whole path anyway.
//
// It is slash-free because scrubError's absolute-path regex redacts any
// slash-led token, so "folder backups/cold" would arrive as "folder
// backups[path]"; that is why this returns one segment and not two.
func shortRepoName(repo string) string {
	if restic.IsRemoteRepo(repo) {
		if i := strings.IndexByte(repo, ':'); i > 0 {
			return repo[:i] + " remote"
		}
		return "a remote repository"
	}
	parts := strings.Split(strings.Trim(filepath.ToSlash(repo), "/"), "/")
	if n := len(parts); n > 0 && parts[n-1] != "" {
		return "folder " + parts[n-1]
	}
	return "a repository"
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

// isRepoUninitialized reports whether a restic error means the backend was
// reached and holds no repository yet, which a remote off-site repo is until its
// first replication. restic says so as "repository does not exist". It prefixes
// "unable to open config file" onto every failure to reach the backend as well,
// a name that does not resolve or a 401 included, so that phrase alone counts
// only when the backend named a missing object, such as a bucket restic init
// will create, and no transport failure is named with it.
func isRepoUninitialized(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "repository does not exist") {
		return true
	}
	return strings.Contains(msg, "unable to open config file") &&
		containsAny(msg, repoAbsenceMarkers) && !containsAny(msg, transportFailureMarkers)
}

// unlockStale clears stale locks, best-effort (plain restic unlock: only
// locks from dead processes or old enough, never an active concurrent
// lock). Logged, never fatal.
func (s *Service) unlockStale(ctx context.Context, repo string, mode restic.Mode) {
	if err := s.engine.Unlock(ctx, repo, false, mode); err != nil {
		log.Printf("api: stale-unlock failed (continuing): %v", err)
	}
}

// listSnapshots lists snapshots, self-healing a stale-lock conflict: on a
// lock error it clears stale locks and retries once, so an interrupted run
// that left a lock behind does not make the backups list fail to load.
//
// The self-heal is skipped for a read-only caller. `restic unlock` writes:
// it deletes lock files in the repository. Mode.NoLock is how a caller
// declares "I never write to this repository", and the two surfaces that
// set it mean somebody else's repository: the foreign restore session and
// the receiver dashboard, both of which say so on screen. Retrying through
// an unlock would break that promise against a box whose owner never
// agreed to it, and only occasionally, since lock errors are rare and the
// repair looks like the read succeeding.
//
// A lock error on a foreign repository is also not BombVault's to repair.
// It usually means the far instance is running a backup right now, and
// the right answer is to report that rather than clear the marker it set.
func (s *Service) listSnapshots(ctx context.Context, repo string, mode restic.Mode) ([]restic.Snapshot, error) {
	snaps, err := s.engine.Snapshots(ctx, repo, mode)
	if isLockErr(err) && !mode.NoLock {
		s.unlockStale(ctx, repo, mode)
		snaps, err = s.engine.Snapshots(ctx, repo, mode)
	}
	// A remote off-site repo that has not been replicated or initialised yet
	// has no snapshots: restic reports "repository does not exist", which must
	// read as "no backups yet", not a fatal error (#117). Local repos are
	// short-circuited upstream by localRepoMissing, so this only affects
	// remotes; genuine auth or connectivity errors do not match
	// isRepoUninitialized and still propagate.
	if err != nil && restic.IsRemoteRepo(repo) && isRepoUninitialized(err) {
		return nil, nil
	}
	return snaps, err
}

// lsSelfHeal lists a snapshot's files (restic ls), self-healing a
// stale-lock conflict like listSnapshots: on a lock error it clears stale
// locks and retries once. `ls` only takes a shared lock, but a stale
// exclusive lock left by an interrupted write elsewhere in the repo (an
// off-site replication or check that didn't finish cleanly, a killed
// backup) blocks it all the same. restic never notices staleness on its
// own, and nothing else touches this repo until the next scheduled backup
// runs its own unlockStale (see #29), so "Select files" would fail with a
// bare "Failed to load files" (#129) while the backups list right above it
// kept working.
func (s *Service) lsSelfHeal(ctx context.Context, repo, snapshotID string, mode restic.Mode) ([]restic.FileEntry, error) {
	entries, err := s.engine.Ls(ctx, repo, snapshotID, mode)
	if isLockErr(err) {
		s.unlockStale(ctx, repo, mode)
		entries, err = s.engine.Ls(ctx, repo, snapshotID, mode)
	}
	return entries, err
}

// pathsPresentInSnapshot answers, for each candidate, whether that exact path is
// a node in the snapshot's tree. One `restic ls` for the whole set.
//
// It exists for mapRestorePaths' pass 2, which produces a selector out of
// a stored path whose ancestor was recorded. An ancestor proves the path
// lies under a backed-up root and nothing more: a --exclude at backup time
// (derived from the selection's own exclusion branches) leaves a hole in
// that root, and a folder created after the snapshot was taken was never
// in it at all. Handing restic such a selector fails the restore, and on
// the container route that failure lands after the container has been
// stopped and removed.
//
// Called only when pass 2 actually narrowed something, which is the uncommon
// case: a selection that still matches the chosen snapshot resolves entirely in
// pass 1 and never reaches here. That keeps a full listing off the normal path.
func (s *Service) pathsPresentInSnapshot(ctx context.Context, repo, snapshotID string, mode restic.Mode, candidates []string) (map[string]bool, error) {
	present := make(map[string]bool, len(candidates))
	if len(candidates) == 0 {
		return present, nil
	}
	entries, err := s.lsSelfHeal(ctx, repo, snapshotID, mode)
	if err != nil {
		return nil, err
	}
	want := make(map[string]bool, len(candidates))
	for _, c := range candidates {
		want[path.Clean(c)] = true
	}
	for _, e := range entries {
		if p := path.Clean(e.Path); want[p] {
			present[p] = true
		}
	}
	// Answer under the caller's own spelling too, so a candidate that was not
	// path.Clean to begin with still looks itself up.
	for _, c := range candidates {
		if present[path.Clean(c)] {
			present[c] = true
		}
	}
	return present, nil
}

// UnlockDomain removes locks from a domain's repo (restic unlock
// --remove-all). BombVault is the sole writer and serialises its
// operations, so a leftover lock is always safe to clear; this is the
// manual counterpart to the automatic stale-lock cleanup before each
// backup.
//
// It covers every repository this domain's items write to (#204). The
// button exists for the case where something is stuck, and a lock on an
// item's named repository is as stuck as one on the domain's own;
// unlocking only the latter would report success and change nothing. Every
// repository is attempted even if one fails, so a single unreachable
// location cannot leave the others locked; the first failure is what the
// caller hears about.
//
// The skip list is returned, not only folded into the error. A shared
// repository can only get the stale-lock clear, a permanent and correct
// limitation (a Note, in repoSkip's terms, so it must not stamp the run
// red), but the operator still has to be told, because the one repository
// they pressed the button for may be exactly the one that kept its lock.
//
// Rendered lines rather than the skips themselves: the caller shows them,
// the same lines the three discovery endpoints send, and a repoSkip is
// this package's own bookkeeping.
func (s *Service) UnlockDomain(ctx context.Context, domain, source string) ([]string, error) {
	settings, repos, skipped, err := s.domainReposForOp(domain, source)
	if err != nil {
		return nil, err
	}
	repos, missing, err := s.reposThatExist(repos, "no repository to unlock yet")
	if err != nil {
		return nil, err
	}
	skipped = append(skipped, missing...)
	unlock, ok := s.tryLockDomainFor(domain, "unlock")
	if !ok {
		return nil, errDomainBusy
	}
	defer unlock()
	// Per repository, like the other three (see CheckDomain).
	ctx, cancel := context.WithTimeout(ctx, time.Duration(len(repos))*2*time.Minute)
	defer cancel()
	// removeAll (a forced removal, live locks included) is only safe where
	// this process is provably the only writer, and the in-process
	// serialisation is keyed by domain. A named repository can be shared
	// between domains (nothing scopes one to a single domain, and the same
	// picker offers it to all three), so forcing on a shared repository would
	// yank the lock out from under another domain's running backup. There a
	// stale clear is the right tool: it removes a lock restic itself deems
	// dead and leaves a live one alone, as every other site in this file does.
	var firstErr error
	for _, r := range repos {
		rMode := s.repoModeFor(settings, domain, source, r.Loc)
		if s.repoSharedWithAnotherDomain(settings, domain, r) {
			log.Printf("api: unlock %s: %s is shared with another domain; clearing only stale locks there", domain, s.refName(r)) //nolint:gosec // G706: domain is a fixed literal, the name is the row's own
			// Reported, not silently downgraded. `restic unlock` without --remove-all
			// removes only what restic calls stale, and a lock left by a previous
			// container incarnation is not stale until it is old enough, so the
			// button that exists for exactly that case can come back green having
			// changed nothing. restic cannot be asked afterwards whether the lock is
			// gone, so the skip list says which repository got the weaker
			// treatment and why.
			if uErr := s.engine.Unlock(ctx, r.Loc, false, rMode); uErr != nil && firstErr == nil {
				firstErr = fmt.Errorf("clearing stale locks on %s: %w", s.refName(r), uErr)
			}
			skipped = append(skipped, repoSkip{Name: s.refName(r), Reason: "another domain writes to it too, so only stale locks were cleared there", Note: true})
			continue
		}
		if uErr := s.engine.Unlock(ctx, r.Loc, true, rMode); uErr != nil && firstErr == nil {
			firstErr = fmt.Errorf("unlocking %s: %w", s.refName(r), uErr)
		}
	}
	if firstErr == nil {
		firstErr = skippedError("this unlock", skipped)
	}
	return skipNames(skipped), firstErr
}

// PruneDomain reclaims repository space freed by forgotten snapshots
// (restic prune), bounded by a generous timeout since pruning a large repo
// is slow. Once the domain lock is held it publishes a "maintenance"
// progress pair (begin and terminal, indeterminate, since restic prune and
// forget stream no percentage) and records a "prune" run, so a manual or
// scheduled prune shows up on the dashboard activity log and run history
// instead of running invisibly.
func (s *Service) PruneDomain(ctx context.Context, domain, source string) error {
	return s.pruneDomain(ctx, domain, source, true)
}

// PruneAfterBulk runs one local prune for a domain after a bulk backup
// loop, replacing the per-item inline prune the bulk run deferred: under
// the #95 bulk flag applyRetention runs each item's forget without
// --prune, so the expensive space reclaim happens here once per run. It
// reuses the PruneDomain core, so the batched prune takes the domain lock
// itself (the bulk loop has released all locks by now), publishes
// maintenance progress and records a kind="prune" run, visible in Run
// History and the Activity Log like a manual prune. Local repo only:
// off-site retention stays inside copyToOffsite, and an immutable off-site
// repo is never pruned from this box. Skipped silently when no repository
// of the domain ages by a keep-policy (its own local repository, a named
// one, or a direct repository under its target's mirrored rules): nothing
// was forgotten, so there is nothing to reclaim. Best-effort: failures are
// logged, never propagated.
func (s *Service) PruneAfterBulk(ctx context.Context, domain string) {
	settings, err := s.store.GetSettings()
	if err != nil {
		log.Printf("api: prune %s: batched prune: read settings: %v", domain, err) //nolint:gosec // G706: domain is a fixed literal
		return
	}
	if !s.domainHasRetention(settings, domain) {
		return // no repository of this domain ages by a policy, so nothing was forgotten
	}
	// applyPolicy=false: the per-item tag-scoped forgets already ran inline during
	// the loop (without --prune), so this pass is a plain space-reclaim.
	if err := s.pruneDomain(ctx, domain, "local", false); err != nil {
		log.Printf("api: prune %s: batched prune failed: %v", domain, err) //nolint:gosec // G706: domain is a fixed literal
	}
}

// pruneDomain is the shared core of PruneDomain and PruneAfterBulk.
// applyPolicy selects the manual-prune semantics (a configured retention
// policy is applied: per-identity forget --keep-* then one prune, "apply
// retention now") versus a plain space reclaim (`restic prune` only, the
// batched post-bulk pass, whose per-item forgets already ran inline
// without --prune).
func (s *Service) pruneDomain(ctx context.Context, domain, source string, applyPolicy bool) (err error) {
	// Every repository this domain's items write to (#204). Prune is what
	// turns a forgotten snapshot back into free space, so pruning only the
	// domain repository would never reclaim the space retention freed on a
	// named one, and nothing would say so.
	settings, repos, skipped, err := s.domainReposForOp(domain, source)
	if err != nil {
		return err
	}
	// An immutable repo is never pruned from this box (append-only is the
	// point), and that is decided per repository:
	//   - off-site: the flag of the target the source names.
	//   - #152: the same refusal when the "local" source is a remote primary
	//     flagged append-only in its saved safety settings. There is no
	//     separate off-site copy in that shape, so refusing is the only thing
	//     standing between an on-box credential and deleting the only backup.
	//     A named repository carries its own flag, so one append-only archive
	//     among an item's repositories is skipped instead of blocking the
	//     whole domain.
	prunable := make([]domainRepoRef, 0, len(repos))
	// …and why each one was left out, so the refusal below can name the card the
	// toggle lives on. With a mixed set the first reason is the one reported: a
	// sentence per repository would be worse than one that names a place to go,
	// and the operator who clears that one comes straight back here for the next.
	refusal := error(nil)
	for _, r := range repos {
		if isOffsiteSource(source) {
			immutable, iErr := s.offsiteSourceImmutable(settings, domain, source)
			if iErr != nil {
				return iErr
			}
			if immutable {
				if refusal == nil {
					refusal = errAppendOnlyOffsiteTarget
				}
				continue
			}
		} else if f := s.refAppendOnly(domain, r); f != appendOnlyNone {
			if refusal == nil {
				refusal = appendOnlyRefusal(f)
			}
			continue
		}
		prunable = append(prunable, r)
	}
	if len(prunable) == 0 {
		if refusal == nil {
			refusal = errOffsiteAppendOnly
		}
		// The skip list travels with this refusal too. Without it an operator
		// whose repositories are all append-only and one of which was switched off
		// or unresolvable would hear only "append-only", never that a repository
		// was not considered at all.
		if sErr := skippedError("this prune", skipped); sErr != nil {
			return errors.Join(refusal, sErr)
		}
		return refusal
	}
	repos, missing, err := s.reposThatExist(prunable, "no backups to prune yet")
	if err != nil {
		return err
	}
	skipped = append(skipped, missing...)
	unlock, ok := s.tryLockDomainFor(domain, "prune")
	if !ok {
		return errDomainBusy
	}
	defer unlock()
	// Per repository, like the other three (see CheckDomain).
	ctx, cancel := context.WithTimeout(ctx, time.Duration(len(repos))*30*time.Minute)
	defer cancel()

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

	// Clear any stale lock left by a previously interrupted run so it can't
	// block this prune: a manual prune (and forget --prune) takes restic's
	// exclusive lock, and an interrupted backup or prune leaves one behind.
	// BombVault is the sole writer, so an existing lock is always stale. Every
	// other repo-mutating path (backups, DeleteSnapshot) does the same.
	//
	// When a retention policy is configured, Prune applies it (forget
	// --keep-* --prune): it collapses snapshots per the policy and reclaims
	// space, the "apply retention now" users expect from a manual prune.
	// Without a policy it stays a plain space reclaim; forget with no
	// keep-flags would delete every snapshot, so that path is guarded by
	// p.Any(). The policy is per source: pruning the off-site repo uses the
	// off-site policy, not the local one, so an archive off-site isn't trimmed
	// to the local rules. The batched post-bulk pass skips this
	// (applyPolicy=false): its per-item forgets already ran inline, and
	// re-running them would cost 44 more exclusive-lock round-trips for
	// nothing.
	for _, r := range repos {
		rMode := s.repoModeFor(settings, domain, source, r.Loc)
		s.unlockStale(ctx, r.Loc, rMode)
		policy := restic.RetentionPolicy{}
		switch {
		case !applyPolicy:
		case isOffsiteSource(source):
			policy = s.retentionPolicyForSource(settings, source)
		default:
			policy = s.retentionPolicyForRef(settings, r)
		}
		if policy.Any() {
			// Per identity: a tag-scoped, ungrouped forget per item and one prune,
			// which also drains frozen path-groups (#91).
			if err = s.applyRetentionPerIdentity(ctx, r.Loc, policy, rMode); err != nil {
				err = fmt.Errorf("pruning %s: %w", s.refName(r), err)
				return err
			}
			continue
		}
		if err = s.engine.Prune(ctx, r.Loc, rMode); err != nil {
			err = fmt.Errorf("pruning %s: %w", s.refName(r), err)
			return err
		}
	}
	err = skippedError("this prune", skipped)
	return err
}

// DeleteSnapshot forgets a single snapshot by id from a domain's repo
// (restic forget without prune, so it is fast). The space is reclaimed
// later by PruneDomain, so deleting several snapshots then pruning once is
// far cheaper than pruning per delete. The snapshot id is validated
// (argument-injection guard) and stale locks are cleared first.
func (s *Service) DeleteSnapshot(ctx context.Context, domain, snapshotID, source string) error {
	if !backup.ValidSnapshotID(snapshotID) {
		return backup.ErrInvalidSnapshotID
	}
	// The snapshot is deleted from the repository it is in, which with named
	// repositories (#204) need not be the domain's own. The id comes from a
	// list the interface built out of every one of them, so resolving the
	// domain repository alone would answer "no matching ID" for a snapshot
	// shown right beside the button, and the snapshot would stay.
	settings, repos, skipped, err := s.domainReposForOp(domain, source)
	if err != nil {
		return err
	}
	repos, missing, err := s.reposThatExist(repos, "no backups to delete yet")
	if err != nil {
		return err
	}
	skipped = append(skipped, missing...)
	unlock, ok := s.tryLockDomainFor(domain, "delete")
	if !ok {
		return errDomainBusy
	}
	defer unlock()
	repo, mode, err := s.repoHoldingSnapshot(ctx, settings, domain, source, repos, snapshotID)
	if err != nil {
		// A snapshot not found while part of the domain was unreachable is "I
		// could not look everywhere", not "it is not there", and the difference
		// decides whether somebody goes looking for it by hand.
		if sErr := skippedError("this delete", skipped); sErr != nil {
			return fmt.Errorf("%w; %w", err, sErr)
		}
		return err
	}
	// Deleting snapshots from an immutable repo is refused (same gate as
	// pruneDomain): append-only means credentials on this box cannot erase that
	// history. Asked of the repository the snapshot is actually in, so a named
	// append-only archive protects itself even though the domain's own repo is
	// maintainable. An off-site source asks the target it names.
	if isOffsiteSource(source) {
		immutable, err := s.offsiteSourceImmutable(settings, domain, source)
		if err != nil {
			return err
		}
		if immutable {
			return errAppendOnlyOffsiteTarget
		}
	}
	// The same refusal applies when the "local" source is a remote primary
	// flagged append-only in its saved safety settings (#152, same gate as
	// pruneDomain): there is no separate off-site copy in that shape, so
	// refusing here is the only thing standing between an on-box credential
	// and deleting backup history.
	if f := s.primaryAppendOnly(domain, repo); !isOffsiteSource(source) && f != appendOnlyNone {
		return appendOnlyRefusal(f)
	}
	s.unlockStale(ctx, repo, mode)
	return s.engine.Forget(ctx, repo, []string{snapshotID}, false, mode)
}

// repoHoldingSnapshot finds which of a domain's repositories a snapshot id
// lives in, and returns it with the mode that repository needs.
//
// A short id is matched by prefix, because that is how restic prints ids
// and therefore what the interface passes back. A repository that cannot
// be listed is skipped rather than failing the search: one unreachable
// location must not stop a deletion from a reachable one.
//
// Every repository is searched even after a hit, and an id that matches in
// two of them is refused rather than guessed. A short id is eight hex
// characters; two repositories of one domain hold snapshots written by the
// same BombVault, so a collision is likelier here than restic's own odds
// within one repository, and a wrong guess deletes the wrong backup.
//
// One repository short-circuits without listing: there is nothing to be
// ambiguous with, restic's own "no matching ID" is the better message for
// an id that is not there, and the caller's append-only refusal has to be
// reachable without first reading a repository that may be remote,
// unreachable or write-protected.
func (s *Service) repoHoldingSnapshot(ctx context.Context, settings store.Settings, domain, source string, repos []domainRepoRef, snapshotID string) (string, restic.Mode, error) {
	if len(repos) == 1 {
		return repos[0].Loc, s.repoModeFor(settings, domain, source, repos[0].Loc), nil
	}
	var (
		found     []domainRepoRef
		foundMode restic.Mode
		unread    []repoSkip
	)
	for _, r := range repos {
		mode := s.repoModeFor(settings, domain, source, r.Loc)
		snaps, err := s.listSnapshots(ctx, r.Loc, mode)
		if err != nil {
			// Remembered, not swallowed. A repository that could not be read is not a
			// repository that does not hold the snapshot, and answering "no repository
			// of this domain holds it" for a share that is merely unmounted sends
			// somebody looking for a backup that is right there.
			unread = append(unread, repoSkip{Name: s.refName(r), Reason: scrubError(err), Unreachable: true})
			continue
		}
		for _, sn := range snaps {
			if strings.HasPrefix(sn.ID, snapshotID) {
				found = append(found, r)
				foundMode = mode
				break
			}
		}
	}
	switch len(found) {
	case 0:
		if sErr := skippedError("the search for this backup", unread); sErr != nil {
			return "", restic.Mode{}, fmt.Errorf("no repository of this domain that could be read holds the backup %s; %w", snapshotID, sErr)
		}
		return "", restic.Mode{}, fmt.Errorf("no repository of this domain holds the backup %s", snapshotID)
	case 1:
		return found[0].Loc, foundMode, nil
	default:
		names := make([]string, 0, len(found))
		for _, r := range found {
			names = append(names, s.refName(r))
		}
		return "", restic.Mode{}, fmt.Errorf("the backup id %s matches a snapshot in more than one repository of this domain (%s); use the full id",
			snapshotID, strings.Join(names, ", "))
	}
}

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
	// Guarantee 0600 even if the file existed with looser perms (WriteFile
	// only applies the mode on creation): it holds cleartext cloud credentials.
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

// RcloneRemotes returns the configured rclone remote names (the [name]
// sections) for display, never the secrets themselves.
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

// decodeRcloneConf returns the decrypted rclone config text stored in
// settings (an empty or blank rclone_conf yields "", no error). Unlike
// RcloneRemotes it keeps the full contents, for the recovery kit, which
// needs the remote secrets.
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
	// fields it is not a secret (a class name), so handleGetCloud returns it.
	// It rides this same AES-256-GCM-encrypted cloud_conf blob, so it needs no
	// schema migration. It is validated against restic.AllowedStorageClasses
	// on save (SetCloudCreds), so only a restore-readable tier is ever stored
	// or emitted.
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

// CloudConfig returns the stored credentials. Callers that serve it to the
// UI must blank the secret fields (see handleGetCloud).
func (s *Service) CloudConfig() (CloudCreds, error) {
	settings, err := s.store.GetSettings()
	if err != nil {
		return CloudCreds{}, err
	}
	return s.decodeCloud(settings)
}

// SetCloudCreds stores the credentials encrypted. A blank secret field keeps the
// previously stored secret (so the UI can edit non-secret fields without
// re-entering keys). A config with nothing set clears it.
func (s *Service) SetCloudCreds(c CloudCreds) error {
	// Normalize and validate the S3 storage class before anything is stored:
	// uppercase it, and reject a non-empty value that is not a whitelisted,
	// restore-readable tier (see restic.AllowedStorageClasses), so an archival
	// class that would break restic restore is never persisted. Empty = the
	// provider default.
	c.S3StorageClass = strings.ToUpper(strings.TrimSpace(c.S3StorageClass))
	if c.S3StorageClass != "" && !restic.StorageClassAllowed(c.S3StorageClass) {
		return fmt.Errorf("unsupported S3 storage class %q (allowed: %s)", c.S3StorageClass, strings.Join(restic.AllowedStorageClasses, ", "))
	}
	// The keep-prior merge below reads the currently stored secrets, so it has
	// to happen in the same transaction as the write. Against a snapshot taken
	// before the write it would re-encrypt a secret that a save landing in
	// between had already replaced, and the blank field would "keep" a value
	// that is no longer the stored one.
	_, err := s.store.MutateSettings(func(settings *store.Settings) error {
		c := c
		// A fully blank request means "clear". Check it before the keep-prior
		// merge, otherwise the merge would re-fill the secrets and clearing would
		// be impossible once a secret had been stored.
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

// CloudCredSet is one named, additional credential set an off-site target
// can opt into via OffsiteTarget.CredsRef (#141) instead of sharing the one
// CloudCreds set; an empty CredsRef keeps using CloudCreds. It embeds
// CloudCreds for the key, secret, region and storage-class fields, so
// cloudEnv and the storage-class validation in SetCloudCredSets stay shared
// with the single-set path.
type CloudCredSet struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	// KeptFor is the id of the direct repository this set holds the values
	// of, from before a save changed them to ones that do not open it.
	KeptFor string `json:"keptFor,omitempty"`
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

// CloudCredSets returns the additional named credential sets with every
// secret field blanked, for serving the list to the UI (the same
// blank-secrets contract as handleGetCloud). Callers that need the real
// secrets, such as restic env building, go through decodeCloudFor.
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
// can rename a set or edit its non-secret fields without re-entering keys.
// KeptFor is carried over the same way, since the Settings page never sends
// it. A set with a blank Name or a duplicate ID is rejected: both would make
// CredsRef resolution ambiguous or the set unreachable from the UI.
func (s *Service) SetCloudCredSets(sets []CloudCredSet) error {
	// Same reasoning as SetCloudCreds: the keep-prior-if-blank merge reads the
	// sets stored right now, so it belongs in the same transaction as the write.
	// The incoming slice is copied rather than normalized in place, because the
	// caller's value is theirs and the merged one carries real secrets.
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
				if next[i].KeptFor == "" {
					next[i].KeptFor = old.KeptFor
				}
			}
		}
		enc, err := s.encodeCloudCredSets(next)
		if err != nil {
			return err
		}
		settings.CloudCredSets = enc
		return nil
	})
	return err
}

func (s *Service) encodeCloudCredSets(sets []CloudCredSet) (string, error) {
	if len(sets) == 0 {
		return "", nil
	}
	blob, err := json.Marshal(sets)
	if err != nil {
		return "", fmt.Errorf("marshal cloud cred sets: %w", err)
	}
	enc, err := secret.Encrypt(s.cfg.AppKey, blob)
	if err != nil {
		return "", fmt.Errorf("encrypt cloud cred sets: %w", err)
	}
	return base64.StdEncoding.EncodeToString(enc), nil
}

// editCloudCredSets applies edit to the stored credential sets, secrets
// included, inside one settings mutation. A list that comes back unchanged is
// not written, since encrypting it again would still change the row.
func (s *Service) editCloudCredSets(edit func([]CloudCredSet) []CloudCredSet) error {
	_, err := s.store.MutateSettings(func(settings *store.Settings) error {
		sets, err := s.decodeCloudCredSets(*settings)
		if err != nil {
			return fmt.Errorf("read the credential sets: %w", err)
		}
		next := edit(slices.Clone(sets))
		if slices.Equal(next, sets) {
			return nil
		}
		enc, err := s.encodeCloudCredSets(next)
		if err != nil {
			return err
		}
		settings.CloudCredSets = enc
		return nil
	})
	return err
}

// decodeCloudFor resolves the credentials an off-site target should use:
// the shared CloudCreds when credsRef is empty, or the matching named
// CloudCredSet otherwise. A credsRef that no longer resolves (the set was
// deleted, or storage drifted) falls back to the shared creds rather than
// failing the caller outright; restic then fails loudly on auth if that
// fallback has no usable credentials for this target's endpoint, which is
// a clearer signal than an opaque config error.
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

// namedRepoItemNames lists the containers, VMs and folder sets pointed at one
// named repository (#204), as a single comma-separated line for the recovery
// kit. Built from the three item lists rather than a new store query, and
// tolerant of a read failure: the location is the part that must not be lost,
// the inventory is the help.
func (s *Service) namedRepoItemNames(id string) string {
	var out []string
	if tgs, err := s.store.ListTargets(); err == nil {
		for _, t := range tgs {
			if strings.TrimSpace(t.Repo) == id {
				out = append(out, "container "+t.ContainerName)
			}
		}
	}
	if vms, err := s.store.ListVMTargets(); err == nil {
		for _, v := range vms {
			if strings.TrimSpace(v.Repo) == id {
				out = append(out, "VM "+v.Name)
			}
		}
	}
	if sets, err := s.store.ListFileSets(); err == nil {
		for _, f := range sets {
			if strings.TrimSpace(f.Repo) == id {
				out = append(out, "folder set "+f.Name)
			}
		}
	}
	return strings.Join(out, ", ")
}

// recoveryRepo is one domain's resolved repo locations for the recovery kit.
type recoveryRepo struct {
	Domain  string
	Local   string
	Offsite string // "" when none configured
}

// RecoveryKit builds the plain-text/markdown recovery document the
// authenticated owner downloads to survive a loss of BombVault itself.
// With encryption on it contains the master APP_KEY and the APP_KEY-derived
// restic repository password the engine uses (restickey.Derive), the
// per-domain repo locations, and step-by-step manual `restic restore`
// instructions that need no BombVault container. With encryption off the
// repos use `--insecure-no-password`, so the kit's value is mainly the repo
// locations and the instructions.
//
// The document contains the master key, so it must never be logged and
// must be stored offline by the user (the handler streams it as an
// attachment only to the session-authenticated owner).
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
	w("> Store it offline and securely (a password manager or printed copy in a safe).\n")
	w("> Anyone with this file can read and restore your backups.\n\n")

	w("## Encryption\n\n")
	if settings.EncryptionEnabled {
		password := restickey.Derive(s.cfg.AppKey)
		w("Status: ENABLED\n\n")
		w("APP_KEY (the master key; recreate the BombVault container with this exact value):\n\n")
		w("    %s\n\n", s.cfg.AppKey)
		w("restic repository password (derived from APP_KEY; use this with plain restic):\n\n")
		w("    %s\n\n", password)
	} else {
		w("Status: DISABLED\n\n")
		w("The repositories are created without a password (restic --insecure-no-password).\n")
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
	w("Each line above is a separate restic repository. Point restic (or a tool like\n")
	w("backrest) at the specific per-domain path. The parent folder that holds them is\n")
	w("not itself a repository, and the off-site repo only has snapshots once off-site\n")
	w("replication has actually run. Add each domain repo on its own.\n\n")

	// Named repositories (#204): the locations individual containers, VMs and
	// folder sets were pointed at instead of their domain's own. Their
	// locations exist only in the database, so without this section a lost
	// /config would leave their intact data unfindable, while the domain
	// repositories above come from settings the user configured and can
	// re-derive. Each line names the items that were pointed at it, so the kit
	// answers both "where is it" and "what is in there".
	if named, nErr := s.store.ListNamedRepos(); nErr == nil && len(named) > 0 {
		w("## Named repositories (per-item)\n\n")
		w("These repositories hold the backups of individual containers, VMs or folder\n")
		w("sets that were pointed at them instead of their domain repository above. They\n")
		w("are ordinary restic repositories and use the same password as the rest.\n\n")
		for _, n := range named {
			loc := n.Repo
			if resolved, rErr := s.resolveRepo(n.Repo); rErr == nil {
				loc = resolved
			}
			w("- %s: %s\n", n.Name, loc)
			if items := s.namedRepoItemNames(n.ID); items != "" {
				w("  holds: %s\n", items)
			}
			if !n.Enabled {
				w("  (switched off at the time this kit was written)\n")
			}
		}
		w("\n")
	}

	// BombVault's own settings backup (the "config" self-backup domain). This
	// repo is the bootstrap seed a rebuilt box needs: restore it first to bring
	// BombVault's configuration back, then the data domains follow. It uses
	// the APP_KEY-derived restic password documented above, so no new secret
	// appears here. A resolution failure leaves the local line blank rather
	// than failing the kit; the off-site line prints only when one is
	// configured.
	w("## BombVault settings backup (config domain)\n\n")
	w("This repository holds BombVault's own settings. On a rebuilt box, restore it\n")
	w("first to bring BombVault's configuration back, then use the data repositories\n")
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

	// Off-site and cloud credentials: the stored rest-server and S3 keys and
	// rclone config a user needs to reach a remote repository after losing
	// BombVault. These are secrets too, covered by the master-secret warning
	// above; like the APP_KEY they go only into this downloaded kit and are
	// never logged. Only the fields that are set are printed (as in cloudEnv),
	// so the section never shows an empty label.
	creds, _ := s.decodeCloud(settings)
	rcloneConf, _ := s.decodeRcloneConf(settings)
	hasREST := creds.RESTUser != "" || creds.RESTPassword != ""
	hasS3 := creds.S3KeyID != "" || creds.S3Secret != "" || creds.S3Region != ""
	hasRclone := strings.TrimSpace(rcloneConf) != ""

	w("## Repository credentials\n\n")
	if !hasREST && !hasS3 && !hasRclone {
		w("No off-site/cloud credentials are stored in BombVault.\n\n")
	} else {
		w("These are the stored off-site backend credentials: the same secrets restic\n")
		w("reads from its environment (or the rclone config) to reach a remote repository.\n")
		w("They are as sensitive as the APP_KEY above; keep them just as safe.\n\n")

		if hasREST {
			w("rest-server (restic REST backend). restic reads these from the environment:\n\n")
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
			w("S3-compatible backend. restic reads these from the environment:\n\n")
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
			w("rclone config, which holds each remote's own secrets. Save it verbatim as\n")
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
		w("2. The repositories have no password; pass --insecure-no-password to every\n")
		w("   restic command below (e.g. `restic -r <repo> --insecure-no-password snapshots`).\n\n")
	}
	w("3. List the snapshots in a repository (use a path or remote from the list above):\n\n")
	w("       restic -r <repo> snapshots\n\n")
	w("4. Restore a snapshot into a target directory (`restic restore`):\n\n")
	w("       restic -r <repo> restore <snapshot-id> --target <restore-dir>\n\n")
	w("Notes:\n")
	w("- For a local repo, point <repo> at the backup folder on disk (the path above is the\n")
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

	// Unraid native notification (delivered over SSH; notify.Send is
	// HTTP-only). Honour the same policy: notifyBackup already returned for
	// "never", so send on "always" or on any failure. In scheduled summary mode
	// drop the per-item Unraid push too; ScheduledNotifyResult sends the one
	// aggregate (#56).
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

// statusSkipped marks a run BombVault intentionally did not perform because the
// target's container no longer exists on the host (#57). runs.status is free-text,
// so this needs no schema migration (cf. the orchestrator's "cancelled" status).
const statusSkipped = "skipped"

// recordAndNotifyContainerSkip handles a scheduled target whose container
// is gone (#57): it records a "skipped" run so the dashboard reflects it,
// agreeing with the green aggregate Healthchecks ping instead of showing
// nothing, and warns the user, debounced to the first miss so a
// permanently removed container doesn't send a notification every night.
// The warning never pings Healthchecks (a skip is not a backup failure), so
// it can never turn a green monitor red.
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
	// Always record the skip so Run History shows it every run (a cheap audit trail)
	// rather than the removed target silently vanishing from the dashboard.
	if runID, sErr := s.store.StartRun(tg.ID, "backup"); sErr != nil {
		log.Printf("api: Backup: skip %q: start skipped run: %v", name, sErr) //nolint:gosec // G706: name is %q-quoted
	} else if fErr := s.store.FinishRun(runID, statusSkipped, "", 0, store.ReasonContainerGone); fErr != nil {
		log.Printf("api: Backup: skip %q: finish skipped run: %v", name, fErr) //nolint:gosec // G706: name is %q-quoted
	}
	if !firstMiss {
		return
	}
	c, err := s.NotifyConfig()
	// Honour the notify policy on both channels (message and Unraid): when
	// muted ("never" or unset) the skipped run row and the dashboard chip
	// already surface it, so no push should fire; otherwise a benign skip
	// would be noisier than a real backup failure, which notifyBackup keeps
	// quiet about under the same policy.
	if err != nil || (c.On != "always" && c.On != "failure") {
		return
	}
	// #111: say plainly that nothing is backed up any more, that the existing
	// backups stay restorable, and how to stop the reminder.
	msg := fmt.Sprintf("Container %q was removed from this host. BombVault is not backing it up anymore; the scheduled backup now skips it. Its existing backups are kept and remain restorable. To stop this reminder, exclude the container from the backup schedule or delete its backups in BombVault.", name)
	// Suppress the per-call Healthchecks ping unconditionally: a skip must
	// never flip the monitor to fail, and the scheduled run's aggregate ping
	// already speaks for the domain. The ctx is based on Background, not the
	// scheduled ctx, so the message is not swept up by scheduled-summary
	// suppression: a "target no longer exists" warning must reach the user
	// even in summary mode (like the update notice).
	notify.Send(notify.WithHealthchecksSuppressed(context.Background()), c, "containers",
		notify.Event{Title: "BombVault: backup target skipped", Message: msg, OK: false})
	if s.unraidGate(c.Unraid) {
		if e := s.sendUnraidNotify(ctx, "BombVault: backup target skipped", msg, "warning"); e != nil {
			log.Printf("notify: unraid: %v", e)
		}
	}
}

// recordPreflightFailure records a failed backup run for an entry that failed
// before the orchestrator could take over run bookkeeping, at one of the
// pre-flight early returns (settings, repo path, EnsureRepo, inspect, upsert).
// Without it a domain-wide fault that trips those for every entry leaves no red
// anywhere (#64), and the card that started a single backup waits for a run that
// never comes (#251). Best-effort: a bookkeeping error is logged, never returned,
// since the caller is already returning the real one. An entry with no target row
// yet has nothing to key a run to, so there the reason is only logged, and the
// scheduled summary still names it from the returned error.
func (s *Service) recordPreflightFailure(kind, name, targetID string, cause error) {
	if targetID == "" {
		log.Printf("api: %s: %q failed before a run could be recorded (no target row yet): %v", kind, name, cause) //nolint:gosec // G706: kind is a fixed literal, name is %q-quoted
		return
	}
	runID, err := s.store.StartRun(targetID, "backup")
	if err != nil {
		log.Printf("api: %s: %q: start failed run: %v", kind, name, err) //nolint:gosec // G706: see above
		return
	}
	if err := s.store.FinishRun(runID, "failed", "", 0, truncateRunErr(cause)); err != nil {
		log.Printf("api: %s: %q: finish failed run: %v", kind, name, err) //nolint:gosec // G706: see above
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
// beginning of a scheduled per-domain run (containers/VMs). The scheduler runs each
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
// a scheduled per-domain run: success when every item succeeded (failed == 0), else
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

// ScheduledNotifyResult sends one summary message per scheduled per-domain
// run on the message channels (webhook, Matrix, SMTP and Unraid) instead of
// one per item, the message-channel counterpart to
// ScheduledHealthchecksResult (#56). It is a no-op unless
// Config.ScheduledSummary is on (in which case the per-item messages were
// suppressed); the On policy still governs whether an all-success run
// notifies at all. domain is the scheduler spelling ("containers"|"vms").
// failures names the items that failed (name and reason), so a failing run
// lists which items broke and why instead of a bare count (#64).
func (s *Service) ScheduledNotifyResult(ctx context.Context, domain string, attempted, failed int, failures []schedule.ItemFailure) {
	c, err := s.NotifyConfig()
	// Honour the notify policy on all channels (message and Unraid): muted
	// ("never" or unset) sends nothing, so the Unraid push below can't leak
	// past a muted policy.
	if err != nil || !c.ScheduledSummary || attempted == 0 ||
		(c.On != "always" && c.On != "failure") {
		return
	}
	ok := failed == 0
	// "no failures" rather than "all succeeded": attempted counts every
	// scheduled item, including any skipped because their container is gone
	// (#57). A skip is not a failure, but it isn't a success either, so don't
	// overstate it. The skipped target still gets its own per-item warning
	// (recordAndNotifyContainerSkip).
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

// maxListedFailures caps how many failed items the scheduled summary lists
// individually before collapsing the rest into a "+N more" tail, so a
// night where 35 of 45 containers failed stays readable in a chat or email.
const maxListedFailures = 10

// formatItemFailures renders a scheduled run's per-item failures as "- name:
// reason" lines for the summary notification, capping the list at
// maxListedFailures with a "+N more" tail (#64). Each reason is scrubbed
// of absolute host paths, as the per-item notifyBackup does with its error
// text, so the aggregated summary leaks nothing the suppressed per-item
// messages would not have.
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
