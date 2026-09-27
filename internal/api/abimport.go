package api

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/appdatabackup"
	"github.com/junkerderprovinz/bombvault/internal/paths"
	"github.com/junkerderprovinz/bombvault/internal/progress"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
	"github.com/junkerderprovinz/bombvault/internal/template"
)

// appdataImportTagPrefix marks a snapshot made from an Appdata.Backup archive.
// The rest of the tag is the plugin's folder name, which is what tells an
// archive that was already imported apart from a new one.
const appdataImportTagPrefix = "import:"

// appdataImportKey is the progress key of an import, one per instance.
const appdataImportKey = "import:containers"

// The states an archive can be in when the folder is looked at.
const (
	abStatusNew         = "new"
	abStatusImported    = "imported"
	abStatusNoContainer = "no-container"
	// abStatusNotBackedUp is a container BombVault knows but never backed up,
	// so a restore would have no recreate recipe and no paths to put back.
	abStatusNotBackedUp = "not-backed-up"
)

// ABImportArchive is one container archive found in the chosen folder.
type ABImportArchive struct {
	Folder    string `json:"folder"`
	Container string `json:"container"`
	File      string `json:"file"`
	Size      int64  `json:"size"`
	Time      int64  `json:"time"`
	Status    string `json:"status"`
}

func isImportedSnapshot(sn restic.Snapshot) bool {
	for _, tag := range sn.Tags {
		if strings.HasPrefix(tag, appdataImportTagPrefix) {
			return true
		}
	}
	return false
}

// restoreRootsOf is what a container restore maps its stored paths onto. An
// imported snapshot recorded its staging folder as its path, while its tree
// holds the container paths themselves, so the host mount stands in for it and
// every stored path is looked up in the tree.
func (s *Service) restoreRootsOf(sn restic.Snapshot) []string {
	if isImportedSnapshot(sn) {
		return []string{path.Clean(s.cfg.HostMountRoot)}
	}
	return sn.Paths
}

type abPlanned struct {
	archive ABImportArchive
	backup  appdatabackup.Backup
	target  store.Target
	repo    string
	mode    restic.Mode
}

// ScanAppdataBackup lists what an Appdata.Backup folder holds and what an
// import would do with each archive. It reads the folder and the repositories
// and writes nothing.
func (s *Service) ScanAppdataBackup(ctx context.Context, sub string) ([]ABImportArchive, error) {
	plan, err := s.planAppdataImport(ctx, sub)
	if err != nil {
		return nil, err
	}
	out := make([]ABImportArchive, 0, len(plan))
	for _, p := range plan {
		out = append(out, p.archive)
	}
	return out, nil
}

func (s *Service) planAppdataImport(ctx context.Context, sub string) ([]abPlanned, error) {
	dir, err := paths.Resolve(s.cfg.HostMountRoot, sub)
	if err != nil {
		return nil, errors.New("choose a folder below the host mount")
	}
	backups, err := appdatabackup.Scan(dir, time.Local)
	if err != nil {
		return nil, fmt.Errorf("read the folder: %w", err)
	}
	settings, err := s.store.GetSettings()
	if err != nil {
		return nil, fmt.Errorf("read settings: %w", err)
	}
	imported := map[string]map[string]bool{}
	var out []abPlanned
	for _, b := range backups {
		for _, a := range b.Archives {
			p := abPlanned{backup: b, archive: ABImportArchive{
				Folder: b.Folder, Container: a.Container, File: a.File, Size: a.Size, Time: b.Time.Unix(),
			}}
			tg, gErr := s.store.GetTargetByContainer(a.Container)
			switch {
			case errors.Is(gErr, sql.ErrNoRows):
				p.archive.Status = abStatusNoContainer
				out = append(out, p)
				continue
			case gErr != nil:
				return nil, fmt.Errorf("read %s: %w", a.Container, gErr)
			case tg.Definition == "" || len(tg.AppdataPaths) == 0:
				p.archive.Status = abStatusNotBackedUp
				out = append(out, p)
				continue
			}
			repo, rErr := s.containerRepoPath(settings, tg)
			if rErr != nil {
				return nil, rErr
			}
			p.target, p.repo, p.mode = tg, repo, s.primaryModeFor(settings, "containers", repo)
			if imported[repo] == nil {
				imported[repo], err = s.importedArchives(ctx, repo, p.mode)
				if err != nil {
					return nil, err
				}
			}
			p.archive.Status = abStatusNew
			if imported[repo][a.Container+"|"+b.Folder] {
				p.archive.Status = abStatusImported
			}
			out = append(out, p)
		}
	}
	return out, nil
}

// importedArchives names the container and folder of every snapshot in repo
// that came from an import.
func (s *Service) importedArchives(ctx context.Context, repo string, mode restic.Mode) (map[string]bool, error) {
	done := map[string]bool{}
	if localRepoMissing(repo) {
		return done, nil
	}
	snaps, err := s.engine.Snapshots(ctx, repo, mode)
	if err != nil {
		return nil, fmt.Errorf("list the backups: %w", err)
	}
	for _, sn := range snaps {
		var container, folder string
		for _, tag := range sn.Tags {
			if c, ok := strings.CutPrefix(tag, "container:"); ok {
				container = c
			}
			if f, ok := strings.CutPrefix(tag, appdataImportTagPrefix); ok {
				folder = f
			}
		}
		if container != "" && folder != "" {
			done[container+"|"+folder] = true
		}
	}
	return done, nil
}

// StartImportAppdataBackup turns every new archive of the folder into a restore
// point of its container, dated when the plugin made it. The archives are only
// read. It reports how many archives it is going to import.
func (s *Service) StartImportAppdataBackup(ctx context.Context, sub string) (int, error) {
	plan, err := s.planAppdataImport(ctx, sub)
	if err != nil {
		return 0, err
	}
	var todo []abPlanned
	var total int64
	for _, p := range plan {
		if p.archive.Status == abStatusNew {
			todo = append(todo, p)
			total += p.archive.Size
		}
	}
	if len(todo) == 0 {
		return 0, errors.New("nothing new to import")
	}
	if !s.batchActive.CompareAndSwap(false, true) {
		return 0, errors.New("a backup or restore is already running")
	}
	bctx := context.WithoutCancel(ctx)
	go func() {
		defer s.recoverOperation("import appdata backup", nil, nil)
		defer s.batchActive.Store(false)
		startedAt := time.Now().Unix()
		publish := func(done int64, active bool) {
			if s.progress == nil {
				return
			}
			pct := 100.0
			if total > 0 {
				pct = float64(done) * 100 / float64(total)
			}
			s.progress.Publish(progress.Event{Key: appdataImportKey, Phase: "maintenance", Percent: pct, Active: active, StartedAt: startedAt})
		}
		publish(0, true)
		var done int64
		for _, p := range todo {
			base := done
			err := s.importAppdataArchive(bctx, p, func(read int64) { publish(base+read/2, true) })
			if err != nil {
				log.Printf("api: import: %s from %s failed: %v", p.archive.Container, p.archive.Folder, err) //nolint:gosec // G706: container and folder names come from the folder listing
			}
			done += p.archive.Size
			publish(done, true)
		}
		publish(done, false)
	}()
	return len(todo), nil
}

// importAppdataArchive unpacks one archive into a staging folder laid out the
// way the container's paths look to BombVault, backs that up as a snapshot of
// the container and removes the staging folder again.
func (s *Service) importAppdataArchive(ctx context.Context, p abPlanned, read func(int64)) (retErr error) {
	runID, err := s.startRun(ctx, p.target.ID, "import")
	if err != nil {
		return err
	}
	var snapID string
	var bytes int64
	defer func() {
		status, msg := "success", ""
		if retErr != nil {
			status, msg = "failed", truncateRunErr(retErr)
		}
		if fErr := s.store.FinishRun(runID, status, snapID, bytes, msg); fErr != nil {
			log.Printf("api: import: finishing the run failed: %v", fErr)
		}
	}()
	unlock := s.lockDomainFor("containers", "import")
	defer unlock()

	if err := s.EnsureRepo(ctx, p.repo, p.mode); err != nil {
		return err
	}
	stagingRoot := s.appdataImportStaging(p.repo)
	if err := os.MkdirAll(stagingRoot, 0o700); err != nil {
		return fmt.Errorf("staging folder: %w", err)
	}
	staging, err := os.MkdirTemp(stagingRoot, "import-")
	if err != nil {
		return fmt.Errorf("staging folder: %w", err)
	}
	defer func() { _ = os.RemoveAll(staging) }()

	archive := filepath.Join(p.backup.Path, p.archive.File)
	if _, err := appdatabackup.Extract(ctx, archive, staging, s.toContainerPath, read); err != nil {
		return fmt.Errorf("unpack %s: %w", p.archive.File, err)
	}
	tags := []string{"container:" + p.archive.Container, appdataImportTagPrefix + p.archive.Folder}
	sum, err := s.engine.ImportDir(ctx, p.repo, staging, tags, p.backup.Time, p.mode)
	if err != nil {
		return err
	}
	snapID, bytes = sum.SnapshotID, int64(sum.BytesAdded)
	if xml, ok := p.backup.Template(p.archive.Container); ok && snapID != "" {
		if err := template.Write(filepath.Join(s.cfg.DataDir, "templates"), snapID+"-"+p.archive.Container, xml); err != nil {
			log.Printf("api: import: keeping the template of %s failed: %v", p.archive.Container, err) //nolint:gosec // G706: a container name from the folder listing
		}
	}
	makeRepoReadable(p.repo, s.cfg.DataDir)
	return nil
}

// appdataImportStaging is where archives are unpacked: next to a local
// repository, which has room for the data by design, or in the app's own
// folder when the repository is remote.
func (s *Service) appdataImportStaging(repo string) string {
	if restic.IsRemoteRepo(repo) {
		return filepath.Join(s.cfg.DataDir, "import-staging")
	}
	return filepath.Join(filepath.Dir(repo), ".bombvault-import")
}
