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
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/backup"
	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/dockercli"
	"github.com/junkerderprovinz/bombvault/internal/model"
	"github.com/junkerderprovinz/bombvault/internal/notify"
	"github.com/junkerderprovinz/bombvault/internal/paths"
	"github.com/junkerderprovinz/bombvault/internal/progress"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/restickey"
	"github.com/junkerderprovinz/bombvault/internal/schedule"
	"github.com/junkerderprovinz/bombvault/internal/secret"
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
	Backup(ctx context.Context, repo string, paths, tags []string, mode restic.Mode) (restic.Summary, error)
	RestorePath(ctx context.Context, repo, snapshotID, path string, mode restic.Mode) error
	// DumpZip streams a snapshot subtree (rooted at subfolder) as a zip into w
	// (flash restore — a non-destructive zip download; the live /boot is never
	// touched and no filesystem metadata is restored).
	DumpZip(ctx context.Context, repo, snapshotID, subfolder string, w io.Writer, mode restic.Mode) error
	Snapshots(ctx context.Context, repo string, mode restic.Mode) ([]restic.Snapshot, error)
	Forget(ctx context.Context, repo string, snapshotIDs []string, prune bool, mode restic.Mode) error
	// ForgetPolicy applies a keep-policy + prune (retention). Inert when the
	// policy has no dimension set.
	ForgetPolicy(ctx context.Context, repo string, p restic.RetentionPolicy, mode restic.Mode) error
	// Ls lists the files in a snapshot (for file-level restore).
	Ls(ctx context.Context, repo, snapshotID string, mode restic.Mode) ([]restic.FileEntry, error)
	// RestoreInclude restores a single path from a snapshot to target (file-level
	// restore; target "/" = in-place to its original location).
	RestoreInclude(ctx context.Context, repo, snapshotID, includePath, target string, mode restic.Mode) error
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
	// Copy replicates snapshots from srcRepo into destRepo (restic copy) for
	// off-site backup. Empty ids copy everything not already in dest. lim caps the
	// transfer bandwidth (zero = unlimited) so replication doesn't saturate the WAN.
	Copy(ctx context.Context, destRepo, srcRepo string, snapshotIDs []string, lim restic.Limits, mode restic.Mode) error
	// Stats returns repository statistics for the chosen --mode ("raw-data" for
	// the physical/deduplicated size + blob count; "restore-size" for the logical
	// size + file count). Used to sample the repo-size trend.
	Stats(ctx context.Context, repo, mode string, m restic.Mode) (restic.StatsResult, error)
	// Diff compares two snapshots (restic diff --json) and returns the summary
	// counts + byte totals (what changed between two backups).
	Diff(ctx context.Context, repo, snap1, snap2 string, m restic.Mode) (restic.DiffResult, error)
	// TagAdd adds tags to a snapshot (restic tag --add). Tags must be
	// pre-sanitised by the caller (restic tags are comma-separated).
	TagAdd(ctx context.Context, repo, snapID string, tags []string, m restic.Mode) error
}

// compile-time check: the real adapter satisfies the seam.
var _ ResticEngine = (*restic.Restic)(nil)

// HostSSH is the subset of sshconn the service uses: NVRAM transfer for VM
// backup/restore plus the public key and reachability test for the UI. A nil
// HostSSH means VM-over-SSH features degrade gracefully (NVRAM is skipped; the
// UEFI restore falls back to EnsureNVRAMTemplate).
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
	// repoMu serialises operations per domain repo. A backup holds its domain's
	// lock for the whole run; maintenance (unlock/prune/delete) TryLocks and
	// reports "busy" instead, so a destructive `restic unlock --remove-all` /
	// prune can never run against a repo a backup is actively writing.
	repoMu map[string]*sync.Mutex

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
}

// NewService constructs the backup service.
func NewService(cfg config.Config, st *store.Repo, d dockercli.Docker, v virshcli.Virsh, eng ResticEngine) *Service {
	return &Service{
		cfg: cfg, store: st, docker: d, virsh: v, engine: eng,
		repoMu: map[string]*sync.Mutex{
			"containers": {},
			"vms":        {},
			"flash":      {},
		},
	}
}

// errDomainBusy is returned by a maintenance op when a backup is holding the
// domain's lock (so it never disturbs an in-progress backup's repo).
var errDomainBusy = errors.New("a backup is currently running for this domain; try again when it finishes")

// lockDomain blocks until it holds the domain's repo lock and returns the unlock
// func (used by backups). A nil/absent mutex (unknown domain) is a no-op.
func (s *Service) lockDomain(domain string) func() {
	mu := s.repoMu[domain]
	if mu == nil {
		return func() {}
	}
	mu.Lock()
	return mu.Unlock
}

// tryLockDomain acquires the domain's repo lock without blocking. It returns the
// unlock func and true on success, or (nil, false) when a backup holds it (used
// by maintenance ops, which must not run against a repo being backed up).
func (s *Service) tryLockDomain(domain string) (func(), bool) {
	mu := s.repoMu[domain]
	if mu == nil {
		return func() {}, true
	}
	if !mu.TryLock() {
		return nil, false
	}
	return mu.Unlock, true
}

// SetHostSSH wires the SSH connection used for VM NVRAM transfer + the UI's
// key/test endpoints. Called from main after the key is ensured.
func (s *Service) SetHostSSH(ssh HostSSH) { s.ssh = ssh }

// SetProgress wires the live-progress store that backup/restore operations
// publish to (and the SSE endpoint subscribes to). Called from main.
func (s *Service) SetProgress(p *progress.Store) { s.progress = p }

// progBegin marks a backup/restore as started for key/phase and returns a
// context carrying a restic sink that republishes each percentage. Percent
// updates are throttled to whole-percent steps to avoid flooding subscribers.
// When no progress store is wired it is a no-op returning ctx unchanged.
func (s *Service) progBegin(ctx context.Context, key, phase string) context.Context {
	if s.progress == nil {
		return ctx
	}
	s.progress.Publish(progress.Event{Key: key, Phase: phase, Percent: 0, Active: true})
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
		s.progress.Publish(progress.Event{Key: key, Phase: phase, Percent: pct, Active: true})
	})
}

// progEnd emits the terminal event for key/phase: 100% on success, 0% on
// failure (the UI hides the bar either way). No-op without a progress store.
func (s *Service) progEnd(key, phase string, ok bool) {
	if s.progress == nil {
		return
	}
	pct := 100.0
	if !ok {
		pct = 0
	}
	s.progress.Publish(progress.Event{Key: key, Phase: phase, Percent: pct, Active: false})
}

// ModeFor builds the restic Mode from the encryption setting. Encryption ON
// derives the password from APP_KEY; OFF uses a password-less repo.
func (s *Service) ModeFor(settings store.Settings) restic.Mode {
	m := restic.Mode{Env: s.cloudEnvFor(settings)}
	if settings.EncryptionEnabled {
		m.Encrypted = true
		m.Password = restickey.Derive(s.cfg.AppKey)
	}
	return m
}

// cloudEnvFor returns the backend-credential env vars for off-site repos, decoded
// from the stored (encrypted) cloud config. Best-effort: a decode failure logs
// and yields no env (the restic op then fails clearly on auth, not on a panic).
func (s *Service) cloudEnvFor(settings store.Settings) []string {
	c, err := s.decodeCloud(settings)
	if err != nil {
		log.Printf("api: cloud creds decode failed (ignoring): %v", err)
		return nil
	}
	return cloudEnv(c)
}

// resolveRepo turns a configured repo location into the value passed to restic
// -r. A restic remote backend (rclone:…, s3:…, sftp:… — off-site) is used
// verbatim; a local location is resolved as a relative subpath under the host
// mount root, rejecting traversal.
func (s *Service) resolveRepo(loc string) (string, error) {
	if restic.IsRemoteRepo(loc) {
		return loc, nil
	}
	repo, err := paths.Resolve(s.cfg.HostMountRoot, loc)
	if err != nil {
		return "", fmt.Errorf("resolve repo path: %w", err)
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

// offsiteLimits maps the stored bandwidth caps to restic transfer limits (KiB/s)
// for off-site replication. All-zero (the default) means unlimited, so the WAN is
// never throttled until the user opts in.
func (s *Service) offsiteLimits(settings store.Settings) restic.Limits {
	return restic.Limits{
		UploadKBps:   settings.OffsiteLimitUpload,
		DownloadKBps: settings.OffsiteLimitDownload,
	}
}

// retentionPolicyForSource returns the keep-policy to apply for a given repo
// source: the off-site policy for "offsite", the local policy otherwise.
func (s *Service) retentionPolicyForSource(settings store.Settings, source string) restic.RetentionPolicy {
	if source == "offsite" {
		return s.offsiteRetentionPolicy(settings)
	}
	return s.retentionPolicy(settings)
}

// applyRetention prunes repo to the configured keep-policy after a successful
// backup. Best-effort: a prune failure is logged but never fails the backup that
// just succeeded — the new snapshot is safe and pruning retries on the next run.
func (s *Service) applyRetention(ctx context.Context, repo string, settings store.Settings, mode restic.Mode) {
	p := s.retentionPolicy(settings)
	if !p.Any() {
		return
	}
	if err := s.engine.ForgetPolicy(ctx, repo, p, mode); err != nil {
		log.Printf("api: retention prune failed (backup is safe): %v", err)
	}
}

// offsiteRepoFor returns the configured off-site repo location for a domain, or
// "" when none is set.
func (s *Service) offsiteRepoFor(domain string, settings store.Settings) string {
	switch domain {
	case "containers":
		return settings.ContainersOffsite
	case "vms":
		return settings.VMsOffsite
	case "flash":
		return settings.FlashOffsite
	}
	return ""
}

// offsiteScheduleFor returns the per-domain off-site replication schedule. Empty
// means "replicate after every local backup"; a non-empty cadence means
// replication is driven by the scheduler instead (decoupled from backups).
func (s *Service) offsiteScheduleFor(domain string, settings store.Settings) string {
	switch domain {
	case "containers":
		return settings.ContainersOffsiteSchedule
	case "vms":
		return settings.VMsOffsiteSchedule
	case "flash":
		return settings.FlashOffsiteSchedule
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
	Domain        string `json:"domain"`        // "containers" | "vms" | "flash"
	Enabled       bool   `json:"enabled"`       // domain switched on in Settings
	Schedule      string `json:"schedule"`      // the cadence string (e.g. "daily 02:30")
	LastSuccess   int64  `json:"lastSuccess"`   // unix time of the last successful backup, 0 = none
	PeriodSeconds int64  `json:"periodSeconds"` // expected RPO window in seconds, 0 = no expectation
	Status        string `json:"status"`        // "off" | "never" | "overdue" | "warn" | "ok"
	// LastVerified is the unix time of the last LOCAL restore-verification drill
	// (`restic check --read-data-subset`), 0 = never verified. LastVerifiedOK is
	// its outcome. These drive the dashboard's "last verified restorable" badge
	// without an extra round-trip.
	LastVerified   int64 `json:"lastVerified"`
	LastVerifiedOK bool  `json:"lastVerifiedOK"`
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

// DomainStatus returns the RPO (protection) status of each domain (containers,
// vms, flash): whether its backups are current relative to its schedule. The
// enabled flag + cadence come from Settings; the last successful backup time
// comes from the store's per-domain helpers.
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

		// A period is only meaningful for an enabled domain with a parseable,
		// non-"off" cadence. An unparseable cadence (defensive — the settings PUT
		// validates) collapses to period 0 → "off".
		var period int64
		cad, cErr := schedule.ParseCadence(d.schedule)
		if cErr == nil {
			period = cad.PeriodSeconds()
		}
		scheduled := d.enabled && period > 0

		// The latest LOCAL restore-verification drill drives the "last verified
		// restorable" badge. Best-effort: a read error leaves the badge at "never"
		// (0 / false) rather than failing the whole status query.
		var lastVerified int64
		var lastVerifiedOK bool
		if drill, found, dErr := s.store.LatestRestoreDrill(d.name, "local"); dErr == nil && found {
			lastVerified = drill.At
			lastVerifiedOK = drill.OK
		}

		out = append(out, DomainStatusEntry{
			Domain:         d.name,
			Enabled:        d.enabled,
			Schedule:       d.schedule,
			LastSuccess:    lastUnix,
			PeriodSeconds:  period,
			Status:         rpoStatus(now, lastUnix, period, scheduled),
			LastVerified:   lastVerified,
			LastVerifiedOK: lastVerifiedOK,
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
}

// runDomains is the target_id → domain map ("container" | "vm" | "flash") used
// to attribute each run to its domain. It mirrors the same mapping handleRuns
// uses: container targets, VM targets, and the singleton flash id. Best-effort —
// an unknown id (e.g. a deleted target) maps to "" and is ignored by the
// bucketer.
func (s *Service) runDomains() map[string]string {
	domain := map[string]string{store.FlashTargetID: "flash"}
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
	if source != "offsite" {
		source = "local"
	}
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
	} {
		if d.enabled {
			s.CollectStatsAsync(d.name, "local")
		}
	}
}

// copyToOffsite replicates a domain's local repo to its off-site repo with
// `restic copy` (the local repo stays primary). It creates the off-site repo on
// first use and copies everything not already there (restic skips dupes, so the
// first run seeds history and later runs ship just the new snapshot). Returns the
// (scrubbed) error so on-demand/scheduled callers can surface it; it never logs
// the off-site location, which can embed credentials. Lock-free — the caller
// holds the domain lock.
func (s *Service) copyToOffsite(ctx context.Context, domain string, settings store.Settings, mode restic.Mode, localRepo string) (err error) {
	loc := s.offsiteRepoFor(domain, settings)
	if loc == "" {
		return errors.New("no off-site repo configured for this domain")
	}
	dest, rerr := s.resolveRepo(loc)
	if rerr != nil {
		return fmt.Errorf("resolve off-site repo: %w", rerr)
	}
	// Publish an active "off-site replication running" indicator for this domain so
	// the UI shows WHICH domain is replicating. restic copy has no machine-readable
	// progress, so this is active/indeterminate (no percent), not a filling bar.
	s.progBegin(ctx, "offsite:"+domain, "replicate")
	defer func() { s.progEnd("offsite:"+domain, "replicate", err == nil) }()
	if err = s.EnsureRepo(ctx, dest, mode); err != nil {
		return fmt.Errorf("ensure off-site repo: %w", err)
	}
	// Cap the transfer rate so off-site replication doesn't saturate the WAN
	// (zero limits = unlimited, the default).
	if err = s.engine.Copy(ctx, dest, localRepo, nil, s.offsiteLimits(settings), mode); err != nil {
		return err
	}
	// Apply the off-site retention policy (separate from local) after a successful
	// copy — only when one is set, so an off-site repo defaults to keep-everything
	// (archive) and existing setups are unchanged. Best-effort: a prune failure
	// must not fail the replication that already succeeded. An IMMUTABLE
	// (append-only) off-site repo is never pruned from here: the far side would
	// refuse the delete anyway, and retention is enforced far-side by design.
	if offsiteImmutableFor(domain, settings) {
		log.Printf("api: offsite %s: retention is enforced far-side (append-only)", domain) //nolint:gosec // G706: domain is a fixed literal
	} else if op := s.offsiteRetentionPolicy(settings); op.Any() {
		if perr := s.engine.ForgetPolicy(ctx, dest, op, mode); perr != nil {
			log.Printf("api: offsite %s: retention prune failed (replica is safe): %v", domain, perr) //nolint:gosec // G706: domain is a fixed literal
		}
	}
	return nil
}

// replicateOffsite runs right after a successful local backup (caller holds the
// domain lock). It replicates ONLY when the domain has no separate off-site
// schedule — a blank schedule couples replication to each backup; a set schedule
// hands it to the scheduler instead. Best-effort: the local backup has already
// succeeded, so an off-site failure is logged, never propagated.
func (s *Service) replicateOffsite(ctx context.Context, domain string, settings store.Settings, mode restic.Mode, localRepo string) {
	if s.offsiteRepoFor(domain, settings) == "" {
		return
	}
	if strings.TrimSpace(s.offsiteScheduleFor(domain, settings)) != "" {
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
	defer s.lockDomain(domain)()
	return s.copyToOffsite(ctx, domain, settings, s.ModeFor(settings), localRepo)
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
		return nil
	}
	// It did not open. Probe the OPPOSITE encryption mode (same backend creds): if
	// that opens it, the repo exists but was created under the other mode — the
	// user toggled the Encryption setting. Fail fast with an actionable message
	// rather than running Init (which would log "config already exists") and then
	// failing every subsequent backup against the now-mismatched repo.
	if s.engine.RepoOpens(ctx, repo, s.oppositeMode(mode)) {
		return fmt.Errorf("this backup repository was created %s, but the Encryption setting is now %s — "+
			"restic cannot open it after the change. Set Encryption back to %s, or point this backup at a "+
			"new, empty repository location",
			encryptionWord(!mode.Encrypted), enabledWord(mode.Encrypted), enabledWord(!mode.Encrypted))
	}
	// Opens with neither mode → treat as not initialised (or a brand-new location)
	// and create it. Local repos need their directory created first; remote
	// backends do not.
	if !restic.IsRemoteRepo(repo) {
		if err := paths.EnsureDir(repo); err != nil {
			return fmt.Errorf("ensure repo dir: %w", err)
		}
	}
	if err := s.engine.Init(ctx, repo, mode); err != nil {
		// Tolerate a race / pre-existing repo: the scrubbed adapter error may not
		// name the cause, so re-probe with the configured mode before failing.
		if s.engine.RepoOpens(ctx, repo, mode) {
			return nil
		}
		if !strings.Contains(strings.ToLower(err.Error()), "already") {
			return fmt.Errorf("init repo: %w", err)
		}
	}
	return nil
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
// We TRANSLATE every appdata bind source from the host root to the container mount
// root and back up the real, correctly cased path — not a guess. Only binds with
// an "appdata" path segment are kept (container config); media libraries, the
// flash, /etc/localtime and other shares are skipped.
//
// Fallback (no appdata bind found): the conventional /mnt/user/appdata/<name>,
// translated if reachable.
func (s *Service) resolveAppdataPaths(name string, in model.Inspect) []string {
	mountRoot := path.Clean(s.cfg.HostMountRoot) // its container path, e.g. /host/user

	var out []string
	seen := map[string]bool{}
	for _, m := range in.Mounts {
		if m.Source == "" || !hasSegment(path.Clean(m.Source), "appdata") {
			continue // only appdata binds (container config), not media/other shares
		}
		if container, ok := s.toContainerPath(m.Source); ok && !seen[container] {
			out = append(out, container)
			seen[container] = true
		}
	}
	if len(out) == 0 {
		// Last resort: the conventional appdata dir for this container — but ONLY
		// if it actually exists. A container with no appdata mount and no such
		// folder is stateless: default to an empty selection (config-only backup)
		// rather than a phantom folder that shows as selected yet backs up nothing.
		cand, ok := s.toContainerPath(path.Join("/mnt/user/appdata", name))
		if !ok {
			cand = path.Join(mountRoot, "appdata", name)
		}
		if _, err := os.Stat(cand); err == nil { //nolint:gosec // G703: cand is HostMountRoot + "appdata" + a validated container name, not raw user input
			out = append(out, cand)
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

// ContainerMounts returns the container's bind mounts annotated for the folder
// selector, plus any selected custom paths (in host form) that do not match a
// current mount. The selection is the stored explicit choice, or the automatic
// appdata default when none is configured.
func (s *Service) ContainerMounts(ctx context.Context, name string) ([]MountInfo, []string, error) {
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
	var custom []string
	for _, cp := range effective {
		if !matched[cp] {
			custom = append(custom, s.toHostPath(cp))
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

// effectiveBackupPaths returns the paths a container backup/export actually uses:
// the explicit folder selection if set, otherwise the automatic appdata
// detection, filtered to those that exist on disk (a stateless container ends up
// with an empty list).
func (s *Service) effectiveBackupPaths(name string, in model.Inspect) []string {
	chosen := s.resolveAppdataPaths(name, in)
	if existing, gErr := s.store.GetTargetByContainer(name); gErr == nil && len(existing.SelectedPaths) > 0 {
		chosen = existing.SelectedPaths
	}
	return onlyExistingPaths(chosen)
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

// Backup runs a full container backup: resolve repo + mode, ensure the repo,
// inspect the container, find-or-create its target, and drive the orchestrator.
func (s *Service) Backup(ctx context.Context, name string) (backup.Summary, error) {
	// A backup must survive the client that triggered it disconnecting — closing
	// the browser tab, or stopping the very container the BombVault UI runs in.
	// Detach from the request's cancellation (keeping its values) with a generous
	// hard cap so a wedged run can't hold the domain lock forever.
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 12*time.Hour)
	defer cancel()
	// Never back up our own container: stopping it mid-run is suicide.
	if self := s.selfContainerName(ctx); self != "" && name == self {
		return backup.Summary{}, ErrSelfBackup
	}
	defer s.lockDomain("containers")() // serialise per repo; blocks maintenance ops meanwhile
	settings, err := s.store.GetSettings()
	if err != nil {
		return backup.Summary{}, fmt.Errorf("read settings: %w", err)
	}
	repo, err := s.containersRepoPath(settings)
	if err != nil {
		return backup.Summary{}, err
	}
	mode := s.ModeFor(settings)
	if err := s.EnsureRepo(ctx, repo, mode); err != nil {
		return backup.Summary{}, err
	}
	// Clear any stale lock left by a previously interrupted run so it can't block
	// this backup (BombVault is the sole writer; an active lock is never stale).
	s.unlockStale(ctx, repo, mode)

	in, err := s.docker.Inspect(ctx, name)
	if err != nil {
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
		err := fmt.Errorf("backup %q: its backup folders are not reachable right now (is the appdata share mounted?) — refusing an empty backup that would look successful", name)
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

	pkey := "container:" + name
	bctx := s.progBegin(ctx, pkey, "backup")
	sum, err := backup.BackupContainer(bctx, backup.BackupDeps{
		ContainerRef:         name,
		ContainerName:        name,
		RepoPath:             repo,
		AppdataPaths:         effective,
		StopTimeout:          30 * time.Second,
		TargetID:             tg.ID,
		SnapshotTemplatesDir: filepath.Join(s.cfg.DataDir, "templates"),
		FlashTemplatesDir:    s.cfg.FlashTemplatesDir,
		WasRunning:           in.Running,
		PreHook:              tg.PreHook,
		PostHook:             tg.PostHook,
		StopContainers:       tg.StopContainers,
		Docker:               s.docker,
		Restic:               &resticAdapter{engine: s.engine, mode: mode},
		Templates:            templatesAdapter{},
		Runs:                 runsAdapter{s.store},
	})
	s.progEnd(pkey, "backup", err == nil)
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
	s.applyRetention(ctx, repo, settings, mode)
	s.replicateOffsite(ctx, "containers", settings, mode, repo)
	s.maybeCollectStats(ctx, "containers")
	return sum, nil
}

// StartBackupAll launches a server-side batch backup of the named containers,
// running them sequentially in a background goroutine. This is the robust path
// for "back up all selected": it runs ON THE SERVER, so it survives the browser
// that started it going away (closing the tab, or — the case that bit a user —
// stopping the very container the BombVault UI is open in). Self and blank names
// are skipped, and a single container failing is logged and the batch continues.
//
// It returns false if a batch is already running (the caller answers 409).
// Progress is published under "batch:containers" for an overall indicator, while
// each container still publishes its own "container:<name>" bar as it runs.
func (s *Service) StartBackupAll(ctx context.Context, names []string) bool {
	if !s.batchActive.CompareAndSwap(false, true) {
		return false
	}
	// Detach immediately so the run — and the self-detection it depends on — is
	// independent of the request that started it (which is canceled the moment the
	// handler returns). Each per-container Backup applies its own hard timeout, so
	// the batch needs no deadline of its own; WithoutCancel keeps request values
	// without a cancel func to leak.
	bctx := context.WithoutCancel(ctx)
	go func() {
		defer s.batchActive.Store(false)

		self := s.selfContainerName(bctx)
		queue := make([]string, 0, len(names))
		for _, n := range names {
			if n != "" && n != self {
				queue = append(queue, n)
			}
		}
		total := len(queue)
		const key = "batch:containers"
		s.publishBatch(key, 0, true)
		ok, fail := 0, 0
		for i, n := range queue {
			if _, err := s.Backup(bctx, n); err != nil {
				fail++
				log.Printf("api: backup-all: %q failed (continuing): %v", n, err) //nolint:gosec // G706: n is %q-quoted
			} else {
				ok++
			}
			s.publishBatch(key, float64(i+1)/float64(total)*100, true)
		}
		s.publishBatch(key, 100, false)
		log.Printf("api: backup-all done: %d ok, %d failed (of %d requested %d)", ok, fail, total, len(names))
	}()
	return true
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
// Returns false if a backup/batch is already running (the caller answers busy).
func (s *Service) StartBackup(ctx context.Context, name string) bool {
	if !s.batchActive.CompareAndSwap(false, true) {
		return false
	}
	// Detach so the run is independent of the request that started it (canceled
	// the moment the handler returns); Backup applies its own hard timeout.
	bctx := context.WithoutCancel(ctx)
	go func() {
		defer s.batchActive.Store(false)
		if _, err := s.Backup(bctx, name); err != nil {
			log.Printf("api: backup: %q failed: %v", name, err) //nolint:gosec // G706: name is %q-quoted
		}
	}()
	return true
}

// StartBackupVM launches a single VM backup in a background goroutine and
// returns immediately, mirroring StartBackup for the VM domain. Progress is
// published under "vm:<name>". Shares batchActive (no overlap with any other
// backup); returns false if one is already running.
func (s *Service) StartBackupVM(ctx context.Context, name string) bool {
	if !s.batchActive.CompareAndSwap(false, true) {
		return false
	}
	bctx := context.WithoutCancel(ctx)
	go func() {
		defer s.batchActive.Store(false)
		if _, err := s.BackupVM(bctx, name); err != nil {
			log.Printf("api: backup vm: %q failed: %v", name, err) //nolint:gosec // G706: name is %q-quoted
		}
	}()
	return true
}

// StartBackupFlash launches the singleton flash backup in a background goroutine
// and returns immediately, mirroring StartBackup. Progress is published under
// "flash". Shares batchActive; returns false if a backup is already running.
func (s *Service) StartBackupFlash(ctx context.Context) bool {
	if !s.batchActive.CompareAndSwap(false, true) {
		return false
	}
	bctx := context.WithoutCancel(ctx)
	go func() {
		defer s.batchActive.Store(false)
		if _, err := s.BackupFlash(bctx); err != nil {
			log.Printf("api: backup flash failed: %v", err)
		}
	}()
	return true
}

// BackupInProgress reports whether a single backup, a batch, or a restore is
// currently running (they share the same single-flight guard). It lets callers
// — and tests — observe when the detached goroutine has fully finished.
func (s *Service) BackupInProgress() bool { return s.batchActive.Load() }

// publishBatch emits an overall batch-progress event (no-op without a store).
func (s *Service) publishBatch(key string, percent float64, active bool) {
	if s.progress == nil {
		return
	}
	s.progress.Publish(progress.Event{Key: key, Phase: "backup", Percent: percent, Active: active})
}

// defsDir returns the directory (a sibling of the containers repo, on the same
// backup storage) where encrypted container definitions are mirrored for
// disaster recovery.
func (s *Service) defsDir(settings store.Settings) (string, error) {
	repo, err := s.containersRepoPath(settings)
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(repo), "bombvault-defs"), nil
}

// writeDefToStorage encrypts the definition with the APP_KEY-derived key and
// writes it to <defsDir>/<name>.def (0600). The env vars inside the definition
// are sensitive, so the file is always encrypted regardless of the restic
// encryption setting.
func (s *Service) writeDefToStorage(settings store.Settings, name string, defJSON []byte) error {
	fn, err := defFileName(name)
	if err != nil {
		return err
	}
	dir, err := s.defsDir(settings)
	if err != nil {
		return err
	}
	if err := paths.EnsureDir(dir); err != nil {
		return fmt.Errorf("ensure defs dir: %w", err)
	}
	enc, err := secret.Encrypt(s.cfg.AppKey, defJSON)
	if err != nil {
		return fmt.Errorf("encrypt definition: %w", err)
	}
	if err := os.WriteFile(filepath.Join(dir, fn), enc, 0o600); err != nil { //nolint:gosec // G703: fn validated by defFileName (no separators/..); dir is operator-configured
		return fmt.Errorf("write definition: %w", err)
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
func (s *Service) Discover(ctx context.Context) (int, error) {
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
	discovered := 0
	for name := range names {
		fn, fnErr := defFileName(name)
		if fnErr != nil {
			log.Printf("api: discover: skipping unsafe container name %q: %v", name, fnErr) //nolint:gosec // G706: %q-quoted
			continue
		}
		enc, rErr := os.ReadFile(filepath.Join(dir, fn)) //nolint:gosec // G304: fn validated by defFileName; dir is operator-configured
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
		if _, uErr := s.store.UpsertTarget(store.Target{
			ContainerName: name,
			AppdataPaths:  def.AppdataPaths,
			Definition:    string(plain),
		}); uErr != nil {
			log.Printf("api: discover: could not upsert target %q: %v", name, uErr) //nolint:gosec // G706: %q-quoted
			continue
		}
		discovered++
	}
	return discovered, nil
}

// vmDefsDir returns the directory (a sibling of the vms repo, on the same backup
// storage) where encrypted VM definitions are mirrored for disaster recovery.
func (s *Service) vmDefsDir(settings store.Settings) (string, error) {
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
	if err := paths.EnsureDir(dir); err != nil {
		return fmt.Errorf("ensure vm defs dir: %w", err)
	}
	enc, err := secret.Encrypt(s.cfg.AppKey, defJSON)
	if err != nil {
		return fmt.Errorf("encrypt vm definition: %w", err)
	}
	if err := os.WriteFile(filepath.Join(dir, fn), enc, 0o600); err != nil { //nolint:gosec // G703: fn validated by defFileName; dir is operator-configured
		return fmt.Errorf("write vm definition: %w", err)
	}
	return nil
}

// DiscoverVMs rebuilds the VM target list from backup storage — the VM
// counterpart of Discover, used after a fresh install / database loss so a VM
// that was deleted from the host (or whose target is gone) becomes restorable
// again. It lists the vms repo's snapshots (tagged vm:<name>), reads + decrypts
// each VM's mirrored definition, and upserts a target. VMs whose definition is
// missing (backed up before mirroring existed) or undecryptable are skipped.
// Returns the number of VMs discovered.
func (s *Service) DiscoverVMs(ctx context.Context) (int, error) {
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
	discovered := 0
	for name := range names {
		fn, fnErr := defFileName(name)
		if fnErr != nil {
			log.Printf("api: discover vms: skipping unsafe name %q: %v", name, fnErr) //nolint:gosec // G706: %q-quoted
			continue
		}
		enc, rErr := os.ReadFile(filepath.Join(dir, fn)) //nolint:gosec // G304: fn validated by defFileName; dir is operator-configured
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
		if _, uErr := s.store.UpsertVMTarget(store.VMTarget{
			Name:       name,
			Method:     method,
			Definition: string(plain),
		}); uErr != nil {
			log.Printf("api: discover vms: could not upsert target %q: %v", name, uErr) //nolint:gosec // G706: %q-quoted
			continue
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
	appdataPaths []string // restored per-path back to origin (nil = recreate-only)
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

// prepareRestore performs ALL of a container restore's validation and
// resolution synchronously — confirmation, name/snapshot-id guards, snapshot
// ownership, path containment and the recreate-recipe lookup — so a bad request
// fails immediately with a clear error, BEFORE anything long-running (or
// destructive) starts. The returned plan is everything executeRestore needs.
func (s *Service) prepareRestore(ctx context.Context, name, snapshotID string, confirm bool, source string) (containerRestorePlan, error) {
	// Guard confirmation before touching the store/docker so an unconfirmed
	// restore surfaces the sentinel (and never errors on a missing target first).
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
	settings, err := s.store.GetSettings()
	if err != nil {
		return containerRestorePlan{}, fmt.Errorf("read settings: %w", err)
	}
	repo, err := s.repoFor(settings, "containers", source)
	if err != nil {
		return containerRestorePlan{}, err
	}
	mode := s.ModeFor(settings)

	tg, err := s.store.GetTargetByContainer(name)
	if err != nil {
		log.Printf("api: restore: unknown target %q: %v", name, err) //nolint:gosec // G706: name is %q-quoted; no raw user bytes reach the log formatter
		return containerRestorePlan{}, errors.New("container has not been backed up yet")
	}

	// "latest" (or empty) resolves to the container's newest snapshot — used by
	// the bulk "restore selected" action. restic returns snapshots oldest-first,
	// so the last tag-matching one is the newest.
	// A definition-only backup (stateless container with no restic snapshot) has
	// no snapshot to resolve — recreate it from the stored definition instead.
	// An explicit id must belong to THIS container (tag-scoped via Snapshots, the
	// same access-control check the file/to-path restores use).
	recreateOnly := false
	snaps, snapErr := s.Snapshots(ctx, name, source)
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
	if recreateOnly {
		appdataForRestore = nil
	} else {
		if len(tg.AppdataPaths) == 0 {
			return containerRestorePlan{}, errors.New("no backup paths recorded for this container — run a backup once, then restore")
		}
		for _, p := range tg.AppdataPaths {
			if !paths.Within(s.cfg.HostMountRoot, p) {
				log.Printf("api: restore: appdata path %q escapes mount root", p) //nolint:gosec // G706: %q-quoted
				return containerRestorePlan{}, errors.New("a stored backup path is outside the host mount — refusing to restore")
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
			return containerRestorePlan{}, errors.New("no stored definition for this container — run a backup once after upgrading, then restore is possible even after deletion")
		}
		in = liveIn
		xml, _, _ = template.Read(s.cfg.FlashTemplatesDir, name)
	}

	return containerRestorePlan{
		repo:         repo,
		mode:         mode,
		targetID:     tg.ID,
		snapshotID:   snapshotID,
		recreateOnly: recreateOnly,
		appdataPaths: appdataForRestore,
		inspect:      in,
		templateXML:  xml,
	}, nil
}

// executeRestore drives the long-running (destructive) part of a container
// restore described by an already-validated plan, publishing "container:<name>"
// progress. The orchestrator records the run (kindRestore) itself.
func (s *Service) executeRestore(ctx context.Context, name string, plan containerRestorePlan, leaveStopped bool) error {
	// Hold the domain repo lock for the whole restic/docker phase. The scheduler
	// calls Backup/BackupVM directly and bypasses the batchActive single-flight
	// guard BY DESIGN — the domain lock is the one layer scheduled jobs do
	// respect — so without it a detached multi-hour restore could overlap a
	// scheduled backup of the same domain in both directions.
	unlock := s.lockDomain("containers")
	defer unlock()
	rkey := "container:" + name
	rctx := s.progBegin(ctx, rkey, "restore")
	rerr := backup.RestoreContainer(rctx, backup.RestoreDeps{
		Confirmed:         true, // prepareRestore rejected unconfirmed requests
		RecreateOnly:      plan.recreateOnly,
		ContainerRef:      name,
		ContainerName:     name,
		RepoPath:          plan.repo,
		SnapshotID:        plan.snapshotID,
		AppdataPaths:      plan.appdataPaths, // restored per-path back to origin (nil = recreate-only)
		TemplateXML:       plan.templateXML,
		FlashTemplatesDir: s.cfg.FlashTemplatesDir,
		Inspect:           plan.inspect,
		LeaveStopped:      leaveStopped,
		TargetID:          plan.targetID,
		Docker:            s.docker,
		Restic:            &resticAdapter{engine: s.engine, mode: plan.mode},
		Templates:         templatesAdapter{},
		Runs:              runsAdapter{s.store},
	})
	s.progEnd(rkey, "restore", rerr == nil)
	return rerr
}

// restoreTimeout is the hard cap on every detached restore goroutine
// (StartRestore/StartRestoreVM/StartRestoreFiles/StartRestoreToPath/
// StartRestoreStack). Aborting a restore mid-flight is DESTRUCTIVE — the
// container has already been removed and the appdata is partially written — so
// unlike the 12h backup cap this one is deliberately generous: it exists only
// so a truly wedged restic can't hold the single-flight guard (and the domain
// lock) forever, never to bound a legitimate huge restore.
const restoreTimeout = 48 * time.Hour

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
	go func() {
		defer s.batchActive.Store(false)
		rctx, cancel := context.WithTimeout(bctx, restoreTimeout)
		defer cancel()
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
func (s *Service) Snapshots(ctx context.Context, name, source string) ([]restic.Snapshot, error) {
	settings, err := s.store.GetSettings()
	if err != nil {
		return nil, fmt.Errorf("read settings: %w", err)
	}
	repo, err := s.repoFor(settings, "containers", source)
	if err != nil {
		return nil, err
	}
	mode := s.ModeFor(settings)
	// A listing before any local backup has run (repo not yet initialised) is "no
	// snapshots yet", not an error — the SPA shows an empty list, not a failure.
	// Remote repos skip this local check and are listed directly (see
	// localRepoMissing), so an off-site view is never wrongly shown as empty.
	if localRepoMissing(repo) {
		return nil, nil
	}
	all, err := s.listSnapshots(ctx, repo, mode)
	if err != nil {
		return nil, err
	}
	tag := "container:" + name
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
	return s.engine.Ls(ctx, repo, snapshotID, s.ModeFor(settings))
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
	if source != "local" && source != "offsite" {
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
	unlock := s.lockDomain("containers")
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
	go func() {
		defer s.batchActive.Store(false)
		rctx, cancel := context.WithTimeout(bctx, restoreTimeout)
		defer cancel()
		runID := s.beginRestoreRun(name)
		rkey := "container:" + name
		pctx := s.progBegin(rctx, rkey, "restore")
		rerr := s.runRestoreFiles(pctx, plan)
		s.progEnd(rkey, "restore", rerr == nil)
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
	runID, err := runsAdapter{s.store}.Start(tg.ID, "restore")
	if err != nil {
		log.Printf("api: restore: record run start for %q failed: %v", name, err) //nolint:gosec // G706: name is %q-quoted
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
	var err error
	if rerr != nil {
		err = runsAdapter{s.store}.Finish(runID, "failed", "", 0, truncateRunErr(rerr))
	} else {
		err = runsAdapter{s.store}.Finish(runID, "success", snapshotID, 0, "")
	}
	if err != nil {
		log.Printf("api: restore: record run finish failed: %v", err)
	}
}

// truncateRunErr bounds an error message so it fits the runs.error_message
// column (mirrors the orchestrator's truncateErr; the restic adapter already
// scrubs secrets/paths from its own errors).
func truncateRunErr(err error) string {
	if err == nil {
		return ""
	}
	msg := err.Error()
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
	if source != "local" && source != "offsite" {
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
	unlock := s.lockDomain("containers")
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
	go func() {
		defer s.batchActive.Store(false)
		rctx, cancel := context.WithTimeout(bctx, restoreTimeout)
		defer cancel()
		runID := s.beginRestoreRun(name)
		rkey := "container:" + name
		pctx := s.progBegin(rctx, rkey, "restore")
		rerr := s.runRestoreToPath(pctx, plan)
		s.progEnd(rkey, "restore", rerr == nil)
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
	if source != "local" && source != "offsite" {
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
	if source != "local" && source != "offsite" {
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
	return s.engine.TagAdd(ctx, repo, snapID, tags, s.ModeFor(settings))
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
	mode := s.ModeFor(settings)

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
	if source == "offsite" && offsiteImmutableFor("vms", settings) {
		return errOffsiteAppendOnly
	}
	if err := s.requireExistingRepo(repo, "no backups to delete yet"); err != nil {
		return err
	}
	unlock, ok := s.tryLockDomain("vms")
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
	// keeps the VM restorable from off-site, so purging only the off-site replica
	// must not strand it.
	if source != "offsite" {
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
		appdata := []string{path.Join(s.cfg.HostMountRoot, "appdata", name)}
		if in, inspErr := s.docker.Inspect(ctx, name); inspErr == nil {
			appdata = s.resolveAppdataPaths(name, in)
		} else {
			log.Printf("api: SetInclude: inspect %q failed (using fallback path): %v", name, inspErr) //nolint:gosec // G706: name is %q-quoted; no raw user bytes reach the log formatter
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

func (a *resticAdapter) Backup(ctx context.Context, repo string, paths, tags []string) (backup.Summary, error) {
	sum, err := a.engine.Backup(ctx, repo, paths, tags, a.mode)
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
type runsAdapter struct{ st *store.Repo }

var _ backup.Runs = runsAdapter{}

func (r runsAdapter) Start(targetID, kind string) (string, error) {
	return r.st.StartRun(targetID, kind)
}

func (r runsAdapter) Finish(runID, status, snapshotID string, bytes int64, errMsg string) error {
	return r.st.FinishRun(runID, status, snapshotID, bytes, errMsg)
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
	Method        string `json:"method"`
	WasAutostart  bool   `json:"was_autostart"`
	// WasRunning is the VM's run state at backup time. A pointer so an OLD backup
	// (taken before this field existed) reads as nil = unknown, and restore then
	// falls back to booting the VM (the historical behaviour). A non-nil value is
	// honoured so restore mirrors the captured state, like containers do.
	WasRunning *bool `json:"was_running,omitempty"`
}

// VMView is the per-VM row returned by ListVMs.
type VMView struct {
	Name              string `json:"name"`
	State             string `json:"state"`
	Method            string `json:"method"`
	IncludeInSchedule bool   `json:"includeInSchedule"`
	LastBackup        *int64 `json:"lastBackup"`
}

// ListVMs returns all known VMs (from virsh) merged with the DB targets.
// VMs with no virsh entry but with backup history appear as state="not-installed".
func (s *Service) ListVMs(ctx context.Context) ([]VMView, error) {
	infos, err := s.virsh.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("list vms: virsh: %w", err)
	}
	targets, _ := s.store.ListVMTargets()
	byName := make(map[string]store.VMTarget, len(targets))
	for _, t := range targets {
		byName[t.Name] = t
	}

	live := make(map[string]bool, len(infos))
	views := make([]VMView, 0, len(infos)+len(targets))
	for _, vm := range infos {
		live[vm.Name] = true
		v := VMView{Name: vm.Name, State: vm.State, Method: "graceful"}
		if t, ok := byName[vm.Name]; ok {
			v.Method = t.Method
			v.IncludeInSchedule = t.IncludeInSchedule
			if run, _ := s.store.LastSuccessfulBackup(t.ID); run != nil {
				v.LastBackup = run.FinishedAt
			}
		}
		views = append(views, v)
	}
	// Orphans: targets whose VM is no longer defined on the host.
	for _, t := range targets {
		if live[t.Name] {
			continue
		}
		v := VMView{Name: t.Name, State: "not-installed", Method: t.Method, IncludeInSchedule: t.IncludeInSchedule}
		if run, _ := s.store.LastSuccessfulBackup(t.ID); run != nil {
			v.LastBackup = run.FinishedAt
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
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 12*time.Hour)
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
			return backup.Summary{}, fmt.Errorf("backup vm: disk %q is not under the host mount and can't be reached for backup — the VM disk must live under your Host Data mount (/mnt)", hp)
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
		Runs:             runsAdapter{s.store},
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
	vkey := "vm:" + name
	bctx := s.progBegin(ctx, vkey, "backup")
	var sum backup.Summary
	if live {
		sum, err = backup.BackupVMLive(bctx, deps)
	} else {
		sum, err = backup.BackupVMGraceful(bctx, deps)
	}
	s.progEnd(vkey, "backup", err == nil)
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
	s.applyRetention(ctx, repo, settings, mode)
	s.replicateOffsite(ctx, "vms", settings, mode, repo)
	s.maybeCollectStats(ctx, "vms")
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
	// wasRunning is the captured run state (nil = old backup with no recorded
	// state → boot after restore, the historical behaviour).
	wasRunning *bool
	preDefine  func(context.Context) error
}

// prepareRestoreVM performs ALL of a VM restore's validation and resolution
// synchronously — confirmation, snapshot-id guard + ownership, definition
// lookup, disk-path containment and the SSH host-key pin — so a bad request
// fails immediately with a clear error, BEFORE anything long-running starts.
func (s *Service) prepareRestoreVM(ctx context.Context, name, snapshotID string, confirm bool, source string) (vmRestorePlan, error) {
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
	settings, err := s.store.GetSettings()
	if err != nil {
		return vmRestorePlan{}, fmt.Errorf("read settings: %w", err)
	}
	repo, err := s.repoFor(settings, "vms", source)
	if err != nil {
		return vmRestorePlan{}, err
	}
	mode := s.ModeFor(settings)

	tg, err := s.store.GetVMTargetByName(name)
	if err != nil {
		return vmRestorePlan{}, errors.New("vm has not been backed up yet")
	}

	// "latest" (or empty) resolves to the VM's newest snapshot. An explicit id
	// must belong to THIS VM (tag-scoped via SnapshotsVM), mirroring the container
	// restores' access-control check.
	snaps, snapErr := s.SnapshotsVM(ctx, name, source)
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

	if tg.Definition == "" {
		return vmRestorePlan{}, errors.New("no stored definition for this vm — run a backup once first")
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

	// Make UEFI domains bootable even if the captured NVRAM is absent: add a
	// template= to <nvram> so libvirt regenerates it from the OVMF master. When
	// NVRAM bytes were captured, PreDefine writes them back over SSH first, so
	// libvirt uses the real var store (boot entries preserved).
	domainXML := virshcli.EnsureNVRAMTemplate(def.DomainXML)

	// preDefine writes the captured NVRAM back to the host over SSH AFTER the old
	// domain is undefined (which removes its nvram) and BEFORE `virsh define`, so
	// the restored VM boots with its original UEFI variables. No-op when there is
	// nothing to write or SSH is unavailable.
	var preDefine func(context.Context) error
	if len(def.NVRAMBytes) > 0 && def.NVRAMHostPath != "" && s.ssh != nil {
		preDefine = func(ctx context.Context) error {
			if err := s.ssh.WriteFile(ctx, def.NVRAMHostPath, def.NVRAMBytes); err != nil {
				log.Printf("api: RestoreVM: WARN NVRAM write over SSH failed for %q (%v) — the VM is restored and will boot, but libvirt regenerates the UEFI variables from the firmware template, so boot entries may need to be re-added", name, err) //nolint:gosec // G706: name is %q-quoted
			}
			return nil // never block the restore on NVRAM — the firmware-template fallback keeps the VM bootable
		}
	}

	// Pin the host key before the orchestrator's virsh-over-SSH calls.
	if s.ssh != nil {
		if err := s.ssh.EnsureKnownHost(ctx); err != nil {
			return vmRestorePlan{}, fmt.Errorf("restore vm: ssh: %w", err)
		}
	}

	return vmRestorePlan{
		repo:         repo,
		mode:         mode,
		targetID:     tg.ID,
		snapshotID:   snapshotID,
		diskPaths:    diskPaths,
		domainXML:    domainXML,
		wasAutostart: def.WasAutostart,
		wasRunning:   def.WasRunning,
		preDefine:    preDefine,
	}, nil
}

// executeRestoreVM drives the long-running (destructive) part of a VM restore
// described by an already-validated plan, publishing "vm:<name>" progress. The
// orchestrator records the run (kindRestore) itself.
func (s *Service) executeRestoreVM(ctx context.Context, name string, plan vmRestorePlan, leaveStopped bool) error {
	// Hold the domain repo lock for the whole restic/libvirt phase: the scheduler
	// calls BackupVM directly (bypassing batchActive by design) and the domain
	// lock is the layer scheduled jobs DO respect (see executeRestore).
	unlock := s.lockDomain("vms")
	defer unlock()
	rkey := "vm:" + name
	rctx := s.progBegin(ctx, rkey, "restore")
	rerr := backup.RestoreVM(rctx, backup.VMRestoreDeps{
		Confirmed:    true, // prepareRestoreVM rejected unconfirmed requests
		Name:         name,
		SnapshotID:   plan.snapshotID,
		DiskPaths:    plan.diskPaths,
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
		Runs:       runsAdapter{s.store},
	})
	s.progEnd(rkey, "restore", rerr == nil)
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
	go func() {
		defer s.batchActive.Store(false)
		rctx, cancel := context.WithTimeout(bctx, restoreTimeout)
		defer cancel()
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
	return s.ssh.Test(ctx)
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
	mode := s.ModeFor(settings)
	// A listing before any backup has run is "no snapshots yet", not an error.
	if localRepoMissing(repo) {
		return nil, nil
	}
	all, err := s.listSnapshots(ctx, repo, mode)
	if err != nil {
		return nil, err
	}
	tag := "vm:" + name
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

// resticAdapter also satisfies the flash domain's backup surface.
var _ backup.FlashRestic = (*resticAdapter)(nil)

// BackupFlash backs up the whole Unraid USB flash (the mounted /boot) to the
// flash repo via restic. Fails with a clear message if the flash directory is
// not mounted (the /boot → /host/boot mount is required for this domain).
func (s *Service) BackupFlash(ctx context.Context) (backup.Summary, error) {
	// Survive the client that triggered it disconnecting (see Backup): detach from
	// the request's cancellation with a generous hard cap.
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 12*time.Hour)
	defer cancel()
	defer s.lockDomain("flash")() // serialise per repo; blocks maintenance ops meanwhile
	settings, err := s.store.GetSettings()
	if err != nil {
		return backup.Summary{}, fmt.Errorf("read settings: %w", err)
	}
	if _, statErr := os.Stat(s.cfg.FlashDir); errors.Is(statErr, fs.ErrNotExist) {
		return backup.Summary{}, fmt.Errorf("flash backup: the Unraid flash is not mounted — add the /boot → %s mount to the container template", s.cfg.FlashDir)
	}
	repo, err := s.flashRepoPath(settings)
	if err != nil {
		return backup.Summary{}, err
	}
	mode := s.ModeFor(settings)
	if err := s.EnsureRepo(ctx, repo, mode); err != nil {
		return backup.Summary{}, err
	}
	// Clear any stale lock left by a previously interrupted run so it can't block
	// this backup (BombVault is the sole writer; an active lock is never stale).
	s.unlockStale(ctx, repo, mode)
	fctx := s.progBegin(ctx, "flash", "backup")
	sum, err := backup.BackupFlash(fctx, backup.FlashBackupDeps{
		SourceDir: s.cfg.FlashDir,
		Repo:      repo,
		TargetID:  store.FlashTargetID,
		Restic:    &resticAdapter{engine: s.engine, mode: mode},
		Runs:      runsAdapter{s.store},
	})
	s.progEnd("flash", "backup", err == nil)
	s.notifyBackup(ctx, "flash", "", err == nil, sum, err)
	if err != nil {
		return backup.Summary{}, err
	}
	s.applyRetention(ctx, repo, settings, mode)
	s.replicateOffsite(ctx, "flash", settings, mode, repo)
	s.maybeCollectStats(ctx, "flash")
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
// the per-file permission errors a to-disk restore caused on /mnt/user), and the
// file drops straight into the Unraid USB creator.
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
	if derr := s.engine.DumpZip(ctx, repo, id, s.cfg.FlashDir, w, mode); derr != nil {
		_ = s.store.FinishRun(runID, "failed", "", 0, derr.Error())
		return derr
	}
	_ = s.store.FinishRun(runID, "success", id, 0, "")
	return nil
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
		return nil, nil // no backups yet
	}
	return s.listSnapshots(ctx, repo, mode)
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

// CheckDomain verifies the integrity of a domain's restic repo (restic check).
// domain is "containers" | "vms" | "flash". Returns a friendly error when the
// repo has not been created yet. Bounded by a timeout so a huge repo can't hang
// the request forever.
func (s *Service) CheckDomain(ctx context.Context, domain, source string) error {
	settings, repo, err := s.domainRepoSource(domain, source)
	if err != nil {
		return err
	}
	if err := s.requireExistingRepo(repo, "no backups to verify yet"); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Minute)
	defer cancel()
	return s.engine.Check(ctx, repo, s.ModeFor(settings))
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

// RunRestoreDrill proves a domain's backup is actually restorable by running
// `restic check --read-data-subset` (it reads back + re-verifies a random subset
// of the real pack data, not just metadata — no scratch disk needed) and records
// the result so the UI can show a "last verified restorable" badge. domain is
// {containers,vms,flash}; source is {local,offsite}.
//
// It takes the per-domain busy-guard like Prune/Unlock: if a backup is running it
// returns errDomainBusy and records nothing. A missing/empty repo returns a clear
// "no backups to verify" error and records nothing (no misleading failure). Both
// a passing and a failing drill ARE recorded; a failure also fires a notification.
func (s *Service) RunRestoreDrill(ctx context.Context, domain, source string) (store.RestoreDrill, error) {
	switch domain {
	case "containers", "vms", "flash":
	default:
		return store.RestoreDrill{}, fmt.Errorf("unknown domain %q", domain)
	}
	switch source {
	case "local", "offsite":
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

	// Serialise with backups (and other maintenance) so a drill never reads a repo
	// a backup is actively writing. Busy → report it WITHOUT recording a drill.
	unlock, ok := s.tryLockDomain(domain)
	if !ok {
		return store.RestoreDrill{}, errDomainBusy
	}
	defer unlock()

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

	// Reading back a subset of real pack data can be slow on a large repo; bound it.
	dctx, cancel := context.WithTimeout(ctx, 2*time.Hour)
	defer cancel()
	checkErr := s.engine.CheckData(dctx, repo, drillSubsetPct(settings.DrillsSubsetPct), mode)

	drill := store.RestoreDrill{
		Domain: domain,
		Source: source,
		At:     time.Now().Unix(),
		OK:     checkErr == nil,
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
	// A failed restorability check is important — notify on failure (best-effort).
	if checkErr != nil {
		s.notifyDrillFailure(ctx, domain, source, drill.Detail)
	}
	return drill, checkErr
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
	notify.Send(ctx, c, notify.Event{Title: "BombVault", Message: msg, OK: false})
	if c.Unraid && s.ssh != nil {
		if e := s.sendUnraidNotify(ctx, "BombVault: restore verification FAILED", msg, "warning"); e != nil {
			log.Printf("notify: unraid: %v", e)
		}
	}
}

// repoFor resolves the restic repo path for a domain ("containers"|"vms"|"flash")
// and source. source "offsite" selects the configured off-site repo (erroring if
// none is set); anything else ("" / "local") selects the primary local repo. This
// lets browse/restore/maintenance operate on either copy.
func (s *Service) repoFor(settings store.Settings, domain, source string) (string, error) {
	if source == "offsite" {
		loc := s.offsiteRepoFor(domain, settings)
		if loc == "" {
			return "", errors.New("no off-site repo configured for this domain")
		}
		return s.resolveRepo(loc)
	}
	switch domain {
	case "containers":
		return s.containersRepoPath(settings)
	case "vms":
		return s.vmsRepoPath(settings)
	case "flash":
		return s.flashRepoPath(settings)
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
	return snaps, err
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
	unlock, ok := s.tryLockDomain(domain)
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
func (s *Service) PruneDomain(ctx context.Context, domain, source string) error {
	settings, repo, err := s.domainRepoSource(domain, source)
	if err != nil {
		return err
	}
	// An immutable off-site repo is never pruned from this box (append-only is
	// the point). Only the offsite+immutable combination is gated — the local
	// repo stays fully maintainable.
	if source == "offsite" && offsiteImmutableFor(domain, settings) {
		return errOffsiteAppendOnly
	}
	if err := s.requireExistingRepo(repo, "no backups to prune yet"); err != nil {
		return err
	}
	unlock, ok := s.tryLockDomain(domain)
	if !ok {
		return errDomainBusy
	}
	defer unlock()
	ctx, cancel := context.WithTimeout(ctx, 30*time.Minute)
	defer cancel()
	mode := s.ModeFor(settings)
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
	if p := s.retentionPolicyForSource(settings, source); p.Any() {
		return s.engine.ForgetPolicy(ctx, repo, p, mode)
	}
	return s.engine.Prune(ctx, repo, mode)
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
	// off-site history. The local repo is unaffected.
	if source == "offsite" && offsiteImmutableFor(domain, settings) {
		return errOffsiteAppendOnly
	}
	if err := s.requireExistingRepo(repo, "no backups to delete yet"); err != nil {
		return err
	}
	unlock, ok := s.tryLockDomain(domain)
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
	settings, err := s.store.GetSettings()
	if err != nil {
		return err
	}
	if strings.TrimSpace(conf) == "" {
		settings.RcloneConf = ""
	} else {
		enc, encErr := secret.Encrypt(s.cfg.AppKey, []byte(conf))
		if encErr != nil {
			return fmt.Errorf("encrypt rclone conf: %w", encErr)
		}
		settings.RcloneConf = base64.StdEncoding.EncodeToString(enc)
	}
	if err := s.store.UpdateSettings(settings); err != nil {
		return err
	}
	return s.writeRcloneFile(settings.RcloneConf)
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
	settings, err := s.store.GetSettings()
	if err != nil {
		return err
	}
	if !c.Configured() && (c.On == "" || c.On == "never") {
		settings.NotifyConf = ""
	} else {
		blob, mErr := json.Marshal(c)
		if mErr != nil {
			return fmt.Errorf("marshal notify conf: %w", mErr)
		}
		enc, eErr := secret.Encrypt(s.cfg.AppKey, blob)
		if eErr != nil {
			return fmt.Errorf("encrypt notify conf: %w", eErr)
		}
		settings.NotifyConf = base64.StdEncoding.EncodeToString(enc)
	}
	return s.store.UpdateSettings(settings)
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
	settings, err := s.store.GetSettings()
	if err != nil {
		return err
	}
	// A fully-blank request means "clear" — check BEFORE the keep-prior merge,
	// otherwise the merge would re-fill the secrets and clearing would be
	// impossible once a secret had been stored.
	if (CloudCreds{}) == c {
		settings.CloudConf = ""
		return s.store.UpdateSettings(settings)
	}
	// Otherwise keep a previously stored secret when its field is left blank, so
	// the non-secret fields can be edited without re-entering keys.
	prev, _ := s.decodeCloud(settings)
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
	return s.store.UpdateSettings(settings)
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
	repos := make([]recoveryRepo, 0, 3)
	for _, d := range []string{"containers", "vms", "flash"} {
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
	target := "Unraid flash"
	if domain != "flash" {
		target = fmt.Sprintf("%s %q", domain, name)
	}
	var msg string
	if ok {
		msg = fmt.Sprintf("Backup of %s succeeded (snapshot %s, %s).", target, shortID(sum.SnapshotID), humanBytes(sum.Bytes))
	} else {
		msg = fmt.Sprintf("Backup of %s FAILED: %s", target, scrubError(backupErr))
	}
	notify.Send(ctx, c, notify.Event{Title: "BombVault", Message: msg, OK: ok})

	// Unraid native notification (delivered over SSH; notify.Send is HTTP-only).
	// Honour the same policy: notifyBackup already returned for "never", so send
	// on "always" or on any failure.
	if c.Unraid && s.ssh != nil && (c.On == "always" || !ok) {
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
