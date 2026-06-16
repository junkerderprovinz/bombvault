// Package api wires the real adapters (dockercli, restic, store, template,
// paths) into the dependency-injected backup orchestrator and exposes the
// JSON HTTP API plus the embedded SPA server.
//
// The DI seam is preserved: internal/backup imports only its own interfaces.
// All concrete-adapter wiring lives here in the service layer.
package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/backup"
	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/dockercli"
	"github.com/junkerderprovinz/bombvault/internal/model"
	"github.com/junkerderprovinz/bombvault/internal/paths"
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
	Backup(ctx context.Context, repo string, paths, tags []string, mode restic.Mode) (restic.Summary, error)
	RestorePath(ctx context.Context, repo, snapshotID, path string, mode restic.Mode) error
	// Restore extracts a whole snapshot to target (used by flash restore, which
	// never restores in-place — it writes to a folder the user then copies to a
	// fresh USB).
	Restore(ctx context.Context, repo, snapshotID, target string, mode restic.Mode) error
	Snapshots(ctx context.Context, repo string, mode restic.Mode) ([]restic.Snapshot, error)
	Forget(ctx context.Context, repo string, snapshotIDs []string, prune bool, mode restic.Mode) error
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
	// EnsureKnownHost pins the host key (raw ssh accept-new) before libvirt's
	// qemu+ssh transport verifies it, so virsh doesn't fail on an empty
	// known_hosts. Also confirms key auth.
	EnsureKnownHost(ctx context.Context) error
}

// Service bridges the real adapters to the backup orchestrator's interfaces.
type Service struct {
	cfg    config.Config
	store  *store.Repo
	docker dockercli.Docker
	virsh  virshcli.Virsh
	engine ResticEngine
	ssh    HostSSH // optional; nil = no SSH (VM NVRAM transfer skipped)
}

// NewService constructs the backup service.
func NewService(cfg config.Config, st *store.Repo, d dockercli.Docker, v virshcli.Virsh, eng ResticEngine) *Service {
	return &Service{cfg: cfg, store: st, docker: d, virsh: v, engine: eng}
}

// SetHostSSH wires the SSH connection used for VM NVRAM transfer + the UI's
// key/test endpoints. Called from main after the key is ensured.
func (s *Service) SetHostSSH(ssh HostSSH) { s.ssh = ssh }

// ModeFor builds the restic Mode from the encryption setting. Encryption ON
// derives the password from APP_KEY; OFF uses a password-less repo.
func (s *Service) ModeFor(settings store.Settings) restic.Mode {
	if settings.EncryptionEnabled {
		return restic.Mode{Encrypted: true, Password: restickey.Derive(s.cfg.AppKey)}
	}
	return restic.Mode{Encrypted: false}
}

// containersRepoPath resolves the absolute restic repo path for the containers
// domain under the host mount root, rejecting traversal.
func (s *Service) containersRepoPath(settings store.Settings) (string, error) {
	repo, err := paths.Resolve(s.cfg.HostMountRoot, settings.ContainersPath)
	if err != nil {
		return "", fmt.Errorf("resolve containers path: %w", err)
	}
	return repo, nil
}

// vmsRepoPath resolves the absolute restic repo path for the vms domain under
// the host mount root, rejecting traversal.
func (s *Service) vmsRepoPath(settings store.Settings) (string, error) {
	repo, err := paths.Resolve(s.cfg.HostMountRoot, settings.VMsPath)
	if err != nil {
		return "", fmt.Errorf("resolve vms path: %w", err)
	}
	return repo, nil
}

// flashRepoPath resolves the restic repo for the flash domain under the host
// mount root, rejecting traversal.
func (s *Service) flashRepoPath(settings store.Settings) (string, error) {
	repo, err := paths.Resolve(s.cfg.HostMountRoot, settings.FlashPath)
	if err != nil {
		return "", fmt.Errorf("resolve flash path: %w", err)
	}
	return repo, nil
}

// flashRestoreTarget resolves the folder a flash snapshot is extracted into — a
// "<flash path>-restore" sibling under the host mount. Flash restore NEVER
// touches the live /boot; the user copies this folder onto a fresh USB.
func (s *Service) flashRestoreTarget(settings store.Settings) (string, error) {
	target, err := paths.Resolve(s.cfg.HostMountRoot, settings.FlashPath+"-restore")
	if err != nil {
		return "", fmt.Errorf("resolve flash restore target: %w", err)
	}
	return target, nil
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

// EnsureRepo creates the repo directory and initialises the restic repo on first
// use. It is idempotent: an already-initialised repo (a `config` marker file
// present) skips Init, and an Init that reports an already-existing repo is
// tolerated.
func (s *Service) EnsureRepo(ctx context.Context, repo string, mode restic.Mode) error {
	if err := paths.EnsureDir(repo); err != nil {
		return fmt.Errorf("ensure repo dir: %w", err)
	}
	// A restic repository always has a top-level `config` file; its presence is
	// the cheap, binary-free idempotency check.
	if _, err := os.Stat(filepath.Join(repo, "config")); err == nil {
		return nil // already initialised
	} else if !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("stat repo: %w", err)
	}
	if err := s.engine.Init(ctx, repo, mode); err != nil {
		// Tolerate a race / pre-existing repo: the scrubbed adapter error may not
		// name the cause, so re-check the marker before failing.
		if _, statErr := os.Stat(filepath.Join(repo, "config")); statErr == nil {
			return nil
		}
		return fmt.Errorf("init repo: %w", err)
	}
	return nil
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
		// Last resort: the conventional appdata dir for this container.
		if c, ok := s.toContainerPath(path.Join("/mnt/user/appdata", name)); ok {
			out = append(out, c)
		} else {
			out = append(out, path.Join(mountRoot, "appdata", name))
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

// Backup runs a full container backup: resolve repo + mode, ensure the repo,
// inspect the container, find-or-create its target, and drive the orchestrator.
func (s *Service) Backup(ctx context.Context, name string) (backup.Summary, error) {
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

	in, err := s.docker.Inspect(ctx, name)
	if err != nil {
		return backup.Summary{}, fmt.Errorf("inspect container: %w", err)
	}
	appdata := s.resolveAppdataPaths(name, in)

	// Persist the recreate recipe (self-contained: inspect + template + appdata
	// paths) so restore works even after the container has been deleted.
	xml, _, _ := template.Read(s.cfg.FlashTemplatesDir, name)
	defBytes, _ := json.Marshal(containerDefinition{Inspect: in, TemplateXML: xml, AppdataPaths: appdata})
	defJSON := string(defBytes)

	tg, err := s.store.UpsertTarget(store.Target{ContainerName: name, AppdataPaths: appdata, Definition: defJSON})
	if err != nil {
		return backup.Summary{}, fmt.Errorf("upsert target: %w", err)
	}

	sum, err := backup.BackupContainer(ctx, backup.BackupDeps{
		ContainerRef:         name,
		ContainerName:        name,
		RepoPath:             repo,
		AppdataPaths:         appdata,
		StopTimeout:          30 * time.Second,
		TargetID:             tg.ID,
		SnapshotTemplatesDir: filepath.Join(s.cfg.DataDir, "templates"),
		FlashTemplatesDir:    s.cfg.FlashTemplatesDir,
		Docker:               s.docker,
		Restic:               &resticAdapter{engine: s.engine, mode: mode},
		Templates:            templatesAdapter{},
		Runs:                 runsAdapter{s.store},
	})
	if err != nil {
		return backup.Summary{}, err
	}

	// Mirror the definition (encrypted) onto the backup storage so a freshly
	// installed BombVault can rebuild its state via Discover after losing
	// /config. Best-effort: a write failure must never fail a good backup.
	if wErr := s.writeDefToStorage(settings, name, defBytes); wErr != nil {
		log.Printf("api: backup: WARN could not persist definition for %q to storage: %v", name, wErr) //nolint:gosec // G706: name is %q-quoted
	}
	return sum, nil
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
	// No repo yet → nothing to discover (not an error).
	if _, statErr := os.Stat(filepath.Join(repo, "config")); errors.Is(statErr, fs.ErrNotExist) {
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

// Restore runs a full container restore. The recreate profile is taken from the
// persisted definition (stored at backup time) so restore works even after the
// container has been deleted. For old targets without a stored definition the
// live inspect is used as a fallback; if that also fails a clear error is
// returned prompting the user to run one backup first.
func (s *Service) Restore(ctx context.Context, name, snapshotID string, confirm bool) error {
	// Guard confirmation before touching the store/docker so an unconfirmed
	// restore surfaces the sentinel (and never errors on a missing target first).
	if !confirm {
		return backup.ErrNotConfirmed
	}
	settings, err := s.store.GetSettings()
	if err != nil {
		return fmt.Errorf("read settings: %w", err)
	}
	repo, err := s.containersRepoPath(settings)
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
	if snapshotID == "latest" || snapshotID == "" {
		snaps, snapErr := s.Snapshots(ctx, name)
		if snapErr != nil {
			return snapErr
		}
		if len(snaps) == 0 {
			return errors.New("no backups found for this container")
		}
		snapshotID = snaps[len(snaps)-1].ID
	}

	// Re-validate the stored appdata paths stay within the host mount root before
	// restoring (defense-in-depth in case the DB was tampered with).
	if len(tg.AppdataPaths) == 0 {
		return errors.New("no backup paths recorded for this container — run a backup once, then restore")
	}
	for _, p := range tg.AppdataPaths {
		if !paths.Within(s.cfg.HostMountRoot, p) {
			log.Printf("api: restore: appdata path %q escapes mount root", p) //nolint:gosec // G706: %q-quoted
			return errors.New("a stored backup path is outside the host mount — refusing to restore")
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

	return backup.RestoreContainer(ctx, backup.RestoreDeps{
		Confirmed:         confirm,
		ContainerRef:      name,
		ContainerName:     name,
		RepoPath:          repo,
		SnapshotID:        snapshotID,
		AppdataPaths:      tg.AppdataPaths, // restored per-path back to origin
		TemplateXML:       xml,
		FlashTemplatesDir: s.cfg.FlashTemplatesDir,
		Inspect:           in,
		TargetID:          tg.ID,
		Docker:            s.docker,
		Restic:            &resticAdapter{engine: s.engine, mode: mode},
		Templates:         templatesAdapter{},
		Runs:              runsAdapter{s.store},
	})
}

// Snapshots lists the snapshots for a single container. The containers repo is
// shared across all containers, so snapshots are filtered by the
// `container:<name>` tag the backup writes — otherwise the restore UI for one
// container would list (and could restore) another container's snapshots.
func (s *Service) Snapshots(ctx context.Context, name string) ([]restic.Snapshot, error) {
	settings, err := s.store.GetSettings()
	if err != nil {
		return nil, fmt.Errorf("read settings: %w", err)
	}
	repo, err := s.containersRepoPath(settings)
	if err != nil {
		return nil, err
	}
	mode := s.ModeFor(settings)
	// A listing before any backup has run (repo not yet initialised) is "no
	// snapshots yet", not an error — the SPA shows an empty list, not a failure.
	// A non-ErrNotExist stat error (e.g. permission denied on the repo dir) is
	// logged as a warning but does not block the engine call: restic will surface
	// the real failure with better context.
	if _, statErr := os.Stat(filepath.Join(repo, "config")); errors.Is(statErr, fs.ErrNotExist) {
		return nil, nil
	} else if statErr != nil {
		log.Printf("api: snapshots: WARN could not stat repo config for %q: %v", name, statErr) //nolint:gosec // G706: name is %q-quoted; no raw user bytes reach the log formatter
	}
	all, err := s.engine.Snapshots(ctx, repo, mode)
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
	snaps, err := s.Snapshots(ctx, name)
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

// RestoreTo extracts a whole snapshot under target (flash restore — never
// in-place). Satisfies backup.FlashRestic.
func (a *resticAdapter) RestoreTo(ctx context.Context, repo, snapshotID, target string) error {
	return a.engine.Restore(ctx, repo, snapshotID, target, a.mode)
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
func (s *Service) BackupVM(ctx context.Context, name string) (backup.Summary, error) {
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
		return backup.Summary{}, fmt.Errorf("backup vm: dumpxml: %w", err)
	}
	domain, err := virshcli.ParseDomain(xmlStr)
	if err != nil {
		return backup.Summary{}, fmt.Errorf("backup vm: parse domain: %w", err)
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

	// NVRAM (UEFI var store) lives under /etc/libvirt on the host. Read it over
	// SSH and keep it IN the definition (no mount, no restic staging). On restore
	// it is written back over SSH; if it is missing, EnsureNVRAMTemplate
	// regenerates it from the OVMF master. A read failure is non-fatal.
	var nvramBytes []byte
	if domain.NVRAMPath != "" && s.ssh != nil {
		if b, rerr := s.ssh.ReadFile(ctx, domain.NVRAMPath); rerr == nil {
			nvramBytes = b
		} else {
			log.Printf("api: BackupVM: NVRAM read over SSH failed (%v); UEFI restore will regenerate it", rerr)
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

	def := vmDefinition{
		DomainXML:     xmlStr,
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

	deps := backup.VMBackupDeps{
		Name:       name,
		DiskPaths:  diskPaths,
		DiskDevice: domain.DiskDevice,
		RepoPath:   repo,
		TargetID:   tg.ID,
		DataDir:    s.cfg.DataDir,
		VM:         s.virsh,
		Restic:     &resticAdapter{engine: s.engine, mode: mode},
		Runs:       runsAdapter{s.store},
	}
	if method == "live" {
		// Live snapshot only works on a RUNNING VM (blockcommit --active --pivot
		// needs an active domain). For a shut-off VM, fall back to graceful — which
		// for an already-off VM just backs up the disks and leaves it off. This
		// avoids creating an overlay we then cannot commit.
		if running, _ := s.virsh.IsActive(ctx, name); running {
			return backup.BackupVMLive(ctx, deps)
		}
		log.Printf("api: BackupVM: %q is not running; using graceful backup instead of live", name) //nolint:gosec // G706: %q-quoted
	}
	return backup.BackupVMGraceful(ctx, deps)
}

// RestoreVM orchestrates a VM restore from a stored definition.
func (s *Service) RestoreVM(ctx context.Context, name, snapshotID string, confirm bool) error {
	if !confirm {
		return backup.ErrNotConfirmed
	}
	settings, err := s.store.GetSettings()
	if err != nil {
		return fmt.Errorf("read settings: %w", err)
	}
	repo, err := s.vmsRepoPath(settings)
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
		snaps, snapErr := s.SnapshotsVM(ctx, name)
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
				log.Printf("api: RestoreVM: NVRAM write over SSH failed (%v); libvirt will regenerate it from the firmware template", err)
			}
			return nil // never block the restore on NVRAM — the template fallback covers it
		}
	}

	// Pin the host key before the orchestrator's virsh-over-SSH calls.
	if s.ssh != nil {
		if err := s.ssh.EnsureKnownHost(ctx); err != nil {
			return fmt.Errorf("restore vm: ssh: %w", err)
		}
	}

	return backup.RestoreVM(ctx, backup.VMRestoreDeps{
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
func (s *Service) SnapshotsVM(ctx context.Context, name string) ([]restic.Snapshot, error) {
	settings, err := s.store.GetSettings()
	if err != nil {
		return nil, fmt.Errorf("read settings: %w", err)
	}
	repo, err := s.vmsRepoPath(settings)
	if err != nil {
		return nil, err
	}
	mode := s.ModeFor(settings)
	// A listing before any backup has run is "no snapshots yet", not an error.
	if _, statErr := os.Stat(filepath.Join(repo, "config")); errors.Is(statErr, fs.ErrNotExist) {
		return nil, nil
	}
	all, err := s.engine.Snapshots(ctx, repo, mode)
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

// resticAdapter also satisfies the flash domain's to-target restore surface.
var _ backup.FlashRestic = (*resticAdapter)(nil)

// BackupFlash backs up the whole Unraid USB flash (the mounted /boot) to the
// flash repo via restic. Fails with a clear message if the flash directory is
// not mounted (the /boot → /host/boot mount is required for this domain).
func (s *Service) BackupFlash(ctx context.Context) (backup.Summary, error) {
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
	return backup.BackupFlash(ctx, backup.FlashBackupDeps{
		SourceDir: s.cfg.FlashDir,
		Repo:      repo,
		TargetID:  store.FlashTargetID,
		Restic:    &resticAdapter{engine: s.engine, mode: mode},
		Runs:      runsAdapter{s.store},
	})
}

// RestoreFlash extracts a flash snapshot to the restore-target folder (safe —
// the live /boot is never overwritten). "latest"/"" resolves to the newest
// snapshot. Returns the absolute target folder so the caller can show the user
// where the recovered flash contents landed.
func (s *Service) RestoreFlash(ctx context.Context, snapshotID string, confirm bool) (string, error) {
	if !confirm {
		return "", backup.ErrNotConfirmed
	}
	settings, err := s.store.GetSettings()
	if err != nil {
		return "", fmt.Errorf("read settings: %w", err)
	}
	repo, err := s.flashRepoPath(settings)
	if err != nil {
		return "", err
	}
	target, err := s.flashRestoreTarget(settings)
	if err != nil {
		return "", err
	}
	mode := s.ModeFor(settings)

	if snapshotID == "latest" || snapshotID == "" {
		snaps, sErr := s.engine.Snapshots(ctx, repo, mode)
		if sErr != nil {
			return "", sErr
		}
		if len(snaps) == 0 {
			return "", errors.New("flash has not been backed up yet")
		}
		snapshotID = snaps[len(snaps)-1].ID
	}
	if err := paths.EnsureDir(target); err != nil {
		return "", fmt.Errorf("create flash restore folder: %w", err)
	}
	if err := backup.RestoreFlash(ctx, backup.FlashRestoreDeps{
		Confirmed:  confirm,
		SnapshotID: snapshotID,
		Repo:       repo,
		Target:     target,
		TargetID:   store.FlashTargetID,
		Restic:     &resticAdapter{engine: s.engine, mode: mode},
		Runs:       runsAdapter{s.store},
	}); err != nil {
		return "", err
	}
	return target, nil
}

// SnapshotsFlash lists restic snapshots in the flash repo (the repo is dedicated
// to flash, so all of its snapshots are flash backups).
func (s *Service) SnapshotsFlash(ctx context.Context) ([]restic.Snapshot, error) {
	settings, err := s.store.GetSettings()
	if err != nil {
		return nil, fmt.Errorf("read settings: %w", err)
	}
	repo, err := s.flashRepoPath(settings)
	if err != nil {
		return nil, err
	}
	mode := s.ModeFor(settings)
	if _, statErr := os.Stat(filepath.Join(repo, "config")); errors.Is(statErr, fs.ErrNotExist) {
		return nil, nil // no backups yet
	}
	return s.engine.Snapshots(ctx, repo, mode)
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
