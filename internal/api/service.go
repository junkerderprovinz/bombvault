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
	"io/fs"
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
	"github.com/junkerderprovinz/bombvault/internal/store"
	"github.com/junkerderprovinz/bombvault/internal/template"
)

// ResticEngine is the subset of *restic.Restic the service depends on. Defining
// it here (with the real restic.Mode/Summary/Snapshot types) lets the service be
// unit-tested with a fake engine without a real restic binary, while *restic.Restic
// satisfies it directly in production.
type ResticEngine interface {
	Init(ctx context.Context, repo string, mode restic.Mode) error
	Backup(ctx context.Context, repo string, paths, tags []string, mode restic.Mode) (restic.Summary, error)
	Restore(ctx context.Context, repo, snapshotID, target string, mode restic.Mode) error
	Snapshots(ctx context.Context, repo string, mode restic.Mode) ([]restic.Snapshot, error)
}

// compile-time check: the real adapter satisfies the seam.
var _ ResticEngine = (*restic.Restic)(nil)

// Service bridges the real adapters to the backup orchestrator's interfaces.
type Service struct {
	cfg    config.Config
	store  *store.Repo
	docker dockercli.Docker
	engine ResticEngine
}

// NewService constructs the backup service.
func NewService(cfg config.Config, st *store.Repo, d dockercli.Docker, eng ResticEngine) *Service {
	return &Service{cfg: cfg, store: st, docker: d, engine: eng}
}

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

// resolveAppdataPaths returns the host paths to back up for a container. It uses
// the inspect's bind mounts whose source lives under <HostMountRoot>/appdata
// (the Unraid convention). If none match, it falls back to the conventional
// per-container appdata directory <HostMountRoot>/appdata/<name>.
//
// ASSUMPTION (documented): appdata lives at <HostMountRoot>/appdata. The broad
// host mount (default host /mnt/user → /host/user) makes the container's own
// appdata reachable at that path for the backup.
func (s *Service) resolveAppdataPaths(name string, in model.Inspect) []string {
	appdataRoot := path.Join(s.cfg.HostMountRoot, "appdata")
	prefix := appdataRoot + "/"

	var out []string
	seen := map[string]bool{}
	for _, m := range in.Mounts {
		src := m.Source
		if src == "" {
			continue
		}
		clean := path.Clean(src)
		if clean == appdataRoot || strings.HasPrefix(clean, prefix) {
			if !seen[clean] {
				out = append(out, clean)
				seen[clean] = true
			}
		}
	}
	if len(out) == 0 {
		out = append(out, path.Join(appdataRoot, name))
	}
	return out
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

	tg, err := s.store.UpsertTarget(store.Target{ContainerName: name, AppdataPaths: appdata})
	if err != nil {
		return backup.Summary{}, fmt.Errorf("upsert target: %w", err)
	}

	return backup.BackupContainer(ctx, backup.BackupDeps{
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
}

// Restore runs a full container restore. The recreate profile is taken from the
// live container inspect (Phase 1 same-host disaster-recovery assumption: the
// container definition still exists on the host) and the captured Unraid
// template is read back from the flash templates dir.
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
		return fmt.Errorf("unknown target %q (back it up first): %w", name, err)
	}

	in, err := s.docker.Inspect(ctx, name)
	if err != nil {
		return fmt.Errorf("inspect container: %w", err)
	}

	// Read back the live Unraid template (if any) to flash on recreate.
	xml, _, _ := template.Read(s.cfg.FlashTemplatesDir, name)

	return backup.RestoreContainer(ctx, backup.RestoreDeps{
		Confirmed:         confirm,
		ContainerRef:      name,
		ContainerName:     name,
		RepoPath:          repo,
		SnapshotID:        snapshotID,
		RestoreTargetDir:  "/", // ALWAYS "/" (SEC-102)
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
	if _, statErr := os.Stat(filepath.Join(repo, "config")); errors.Is(statErr, fs.ErrNotExist) {
		return nil, nil
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

func (a *resticAdapter) Restore(ctx context.Context, repo, snapshotID, target string) error {
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
