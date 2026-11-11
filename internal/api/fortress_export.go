package api

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"slices"
	"strings"
	"time"

	"github.com/junkerderprovinz/parleyport/kit/fortress"
	"github.com/junkerderprovinz/parleyport/kit/status"

	"github.com/junkerderprovinz/bombvault/internal/restic"
)

// appExportWait bounds fetching an app's own backup, which the kit streams
// without a deadline of its own.
const appExportWait = 10 * time.Minute

// errExportUnencrypted keeps an app's own backup, which can hold its secrets
// in the clear, out of a repository without a password.
var errExportUnencrypted = errors.New("the repository is not encrypted, so the app's own backup stays out of it")

// appExporter is the part of a fortress that hands over the backups of the
// apps in it.
type appExporter interface {
	Members() []fortress.Member
	Status(ctx context.Context, id, lang string) (status.Report, error)
	Export(ctx context.Context, id string) (io.ReadCloser, error)
}

// exportingMember finds the member that runs in container id and offers a
// backup of its own. Only one on this network can hand it over.
func exportingMember(ctx context.Context, ex appExporter, id string) (fortress.Member, bool) {
	for _, m := range ex.Members() {
		if !m.Direct || !slices.Contains(m.Actions, fortress.ExportAction) {
			continue
		}
		rep, err := ex.Status(ctx, m.ID, "en")
		if err == nil && sameContainer(rep.Instance.Container, id) {
			return m, true
		}
	}
	return fortress.Member{}, false
}

// sameContainer compares a container id as an app reads it, often the short
// one its hostname carries, with the full one Docker reports.
func sameContainer(reported, id string) bool {
	const short = 12
	return len(reported) >= short && len(id) >= short && (strings.HasPrefix(id, reported) || strings.HasPrefix(reported, id))
}

// storeAppExport puts the own backup of the app that runs in a container into
// the container's repository before the container is backed up, so the app's
// state comes back consistent even while it kept running.
func (s *Service) storeAppExport(ctx context.Context, name, containerID, repo string, mode restic.Mode) {
	f := s.fortressNow()
	if f == nil {
		return
	}
	if err := s.storeAppExportFrom(ctx, f, name, containerID, repo, mode); err != nil {
		log.Printf("fortress: the own backup of the app in %s: %v", name, err) //nolint:gosec // G706: a container name from Docker, %s on a log line
	}
}

func (s *Service) storeAppExportFrom(ctx context.Context, ex appExporter, name, containerID, repo string, mode restic.Mode) error {
	m, ok := exportingMember(ctx, ex, containerID)
	if !ok {
		return nil
	}
	if !mode.Encrypted {
		return errExportUnencrypted
	}
	ctx, cancel := context.WithTimeout(ctx, appExportWait)
	defer cancel()
	rd, err := ex.Export(ctx, m.ID)
	if err != nil {
		return fmt.Errorf("fetch it from %s: %w", m.App.ID, err)
	}
	defer func() { _ = rd.Close() }()
	tags := []string{"appexport:" + name, "app:" + m.App.ID}
	if _, err := s.engine.BackupStdin(ctx, repo, rd, "/appexport/"+name+"/"+m.App.ID, tags, mode); err != nil {
		return fmt.Errorf("store it: %w", err)
	}
	log.Printf("fortress: stored the own backup of %s from %s", m.App.ID, name) //nolint:gosec // G706: names from the fortress and Docker, %s on a log line
	return nil
}
