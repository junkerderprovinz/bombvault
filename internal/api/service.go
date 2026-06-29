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
	// Unlock removes locks from the repo (restic unlock). removeAll clears ALL
	// locks, not just stale ones.
	Unlock(ctx context.Context, repo string, removeAll bool, mode restic.Mode) error
	// Prune reclaims space freed by forgotten snapshots (restic prune).
	Prune(ctx context.Context, repo string, mode restic.Mode) error
	// Copy replicates snapshots from srcRepo into destRepo (restic copy) for
	// off-site backup. Empty ids copy everything not already in dest.
	Copy(ctx context.Context, destRepo, srcRepo string, snapshotIDs []string, mode restic.Mode) error
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

	// batchActive guards the server-side "back up all" run so only one can be in
	// flight at a time (a second request gets a 409 instead of overlapping).
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
	if err = s.engine.Copy(ctx, dest, localRepo, nil, mode); err != nil {
		return err
	}
	// Apply the off-site retention policy (separate from local) after a successful
	// copy — only when one is set, so an off-site repo defaults to keep-everything
	// (archive) and existing setups are unchanged. Best-effort: a prune failure
	// must not fail the replication that already succeeded.
	if op := s.offsiteRetentionPolicy(settings); op.Any() {
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

// Restore runs a full container restore. The recreate profile is taken from the
// persisted definition (stored at backup time) so restore works even after the
// container has been deleted. For old targets without a stored definition the
// live inspect is used as a fallback; if that also fails a clear error is
// returned prompting the user to run one backup first.
func (s *Service) Restore(ctx context.Context, name, snapshotID string, confirm bool, source string) error {
	// Guard confirmation before touching the store/docker so an unconfirmed
	// restore surfaces the sentinel (and never errors on a missing target first).
	if !confirm {
		return backup.ErrNotConfirmed
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

	tg, err := s.store.GetTargetByContainer(name)
	if err != nil {
		log.Printf("api: restore: unknown target %q: %v", name, err) //nolint:gosec // G706: name is %q-quoted; no raw user bytes reach the log formatter
		return errors.New("container has not been backed up yet")
	}

	// "latest" (or empty) resolves to the container's newest snapshot — used by
	// the bulk "restore selected" action. restic returns snapshots oldest-first,
	// so the last tag-matching one is the newest.
	// A definition-only backup (stateless container with no restic snapshot) has
	// no snapshot to resolve — recreate it from the stored definition instead.
	recreateOnly := false
	if snapshotID == "latest" || snapshotID == "" {
		snaps, snapErr := s.Snapshots(ctx, name, source)
		if snapErr != nil {
			return snapErr
		}
		switch {
		case len(snaps) > 0:
			snapshotID = snaps[len(snaps)-1].ID
		case tg.Definition != "":
			recreateOnly = true
		default:
			return errors.New("no backups found for this container")
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
			return errors.New("no backup paths recorded for this container — run a backup once, then restore")
		}
		for _, p := range tg.AppdataPaths {
			if !paths.Within(s.cfg.HostMountRoot, p) {
				log.Printf("api: restore: appdata path %q escapes mount root", p) //nolint:gosec // G706: %q-quoted
				return errors.New("a stored backup path is outside the host mount — refusing to restore")
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
			return fmt.Errorf("restore: unmarshal stored definition: %w", jsonErr)
		}
		in = def.Inspect
		xml = def.TemplateXML
	} else {
		// Fallback: target was backed up before this feature; try live inspect.
		liveIn, liveErr := s.docker.Inspect(ctx, name)
		if liveErr != nil {
			return errors.New("no stored definition for this container — run a backup once after upgrading, then restore is possible even after deletion")
		}
		in = liveIn
		xml, _, _ = template.Read(s.cfg.FlashTemplatesDir, name)
	}

	rkey := "container:" + name
	rctx := s.progBegin(ctx, rkey, "restore")
	rerr := backup.RestoreContainer(rctx, backup.RestoreDeps{
		Confirmed:         confirm,
		RecreateOnly:      recreateOnly,
		ContainerRef:      name,
		ContainerName:     name,
		RepoPath:          repo,
		SnapshotID:        snapshotID,
		AppdataPaths:      appdataForRestore, // restored per-path back to origin (nil = recreate-only)
		TemplateXML:       xml,
		FlashTemplatesDir: s.cfg.FlashTemplatesDir,
		Inspect:           in,
		TargetID:          tg.ID,
		Docker:            s.docker,
		Restic:            &resticAdapter{engine: s.engine, mode: mode},
		Templates:         templatesAdapter{},
		Runs:              runsAdapter{s.store},
	})
	s.progEnd(rkey, "restore", rerr == nil)
	return rerr
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

// RestoreContainerFile restores a single file/dir from a container snapshot back
// to its original location (in-place). filePath must be an absolute path within
// the host mount — defense-in-depth so a restore can never write outside it.
func (s *Service) RestoreContainerFile(ctx context.Context, snapshotID, filePath string, confirm bool, source string) error {
	if !confirm {
		return backup.ErrNotConfirmed
	}
	if !backup.ValidSnapshotID(snapshotID) {
		return backup.ErrInvalidSnapshotID
	}
	// Clean once so the path we validate is exactly the path we execute.
	clean := path.Clean(filePath)
	if !paths.Within(s.cfg.HostMountRoot, clean) {
		return errors.New("restore file: path is outside the backup mount")
	}
	settings, err := s.store.GetSettings()
	if err != nil {
		return fmt.Errorf("read settings: %w", err)
	}
	repo, err := s.repoFor(settings, "containers", source)
	if err != nil {
		return err
	}
	// target "/" → restic writes the included path back to its absolute location.
	return s.engine.RestoreInclude(ctx, repo, snapshotID, clean, "/", s.ModeFor(settings))
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
	def := vmDefinition{
		DomainXML:     defXML,
		DiskPaths:     diskPaths,
		NVRAMHostPath: domain.NVRAMPath,
		NVRAMBytes:    nvramBytes,
		Method:        method,
		WasAutostart:  wasAutostart,
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
	return sum, nil
}

// RestoreVM orchestrates a VM restore from a stored definition.
func (s *Service) RestoreVM(ctx context.Context, name, snapshotID string, confirm bool, source string) error {
	if !confirm {
		return backup.ErrNotConfirmed
	}
	settings, err := s.store.GetSettings()
	if err != nil {
		return fmt.Errorf("read settings: %w", err)
	}
	repo, err := s.repoFor(settings, "vms", source)
	if err != nil {
		return err
	}
	mode := s.ModeFor(settings)

	tg, err := s.store.GetVMTargetByName(name)
	if err != nil {
		return errors.New("vm has not been backed up yet")
	}

	// "latest" (or empty) resolves to the VM's newest snapshot.
	if snapshotID == "latest" || snapshotID == "" {
		snaps, snapErr := s.SnapshotsVM(ctx, name, source)
		if snapErr != nil {
			return snapErr
		}
		if len(snaps) == 0 {
			return errors.New("no backups found for this vm")
		}
		snapshotID = snaps[len(snaps)-1].ID
	}

	if tg.Definition == "" {
		return errors.New("no stored definition for this vm — run a backup once first")
	}
	var def vmDefinition
	if err := json.Unmarshal([]byte(tg.Definition), &def); err != nil {
		return fmt.Errorf("restore vm: unmarshal definition: %w", err)
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
		return errors.New("no restorable disk paths found in this backup")
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
			return fmt.Errorf("restore vm: ssh: %w", err)
		}
	}

	rkey := "vm:" + name
	rctx := s.progBegin(ctx, rkey, "restore")
	rerr := backup.RestoreVM(rctx, backup.VMRestoreDeps{
		Confirmed:    confirm,
		Name:         name,
		SnapshotID:   snapshotID,
		DiskPaths:    diskPaths,
		DomainXML:    domainXML,
		WasAutostart: def.WasAutostart,
		StartAfter:   true,
		PreDefine:    preDefine,
		RepoPath:     repo,
		TargetID:     tg.ID,
		DataDir:      s.cfg.DataDir,
		VM:           s.virsh,
		Restic:       &resticAdapter{engine: s.engine, mode: mode},
		Runs:         runsAdapter{s.store},
	})
	s.progEnd(rkey, "restore", rerr == nil)
	return rerr
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
