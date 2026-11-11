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
	"unicode"

	"github.com/junkerderprovinz/parleyport/kit/fortress"
	"github.com/junkerderprovinz/parleyport/kit/status"

	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// appExportWait bounds fetching an app's own backup, which the kit streams
// without a deadline of its own.
const appExportWait = 10 * time.Minute

// errExportUnencrypted keeps an app's own backup, which can hold its secrets
// in the clear, out of a repository without a password.
var errExportUnencrypted = errors.New("the repository is not encrypted, so the app's own backup stays out of it")

// The tags an app's own backup is stored under: the container it came from,
// the app's id and, where it fits in a tag, the name the app goes by.
const (
	appExportTagPrefix = "appexport:"
	appTagPrefix       = "app:"
	appNameTagPrefix   = "appname:"
)

func appExportPath(name, app string) string { return "/appexport/" + name + "/" + app }

// tagSafe reports whether text survives as one restic tag, which restic
// splits at a comma.
func tagSafe(text string) bool {
	return text != "" && len(text) <= 64 && !strings.ContainsFunc(text, func(r rune) bool { return r == ',' || unicode.IsControl(r) })
}

// containerAppExportIdentity is the container entry name under the tag the
// own backups of its apps carry, with every name it had before.
func (s *Service) containerAppExportIdentity(name string) entryIdentity {
	ownID := ""
	tg, err := s.store.GetTargetByContainer(name)
	if err == nil {
		ownID = tg.ID
	}
	return withRowReadErr(s.aliasedIdentity("container", appExportTagPrefix, name, ownID), err)
}

// forgetAppExportSeries ages the own backups of the apps in a container by the
// container's keep rule, as a series of their own, so a pass keeps as many of
// them as of the container's backups. It does not prune: the container's own
// pass right after it reclaims the space of both. A finding that holds the
// container's backups holds these too.
func (s *Service) forgetAppExportSeries(ctx context.Context, repo string, settings store.Settings, mode restic.Mode, name, targetID string) {
	p := s.retentionPolicyForRef(settings, "containers", s.refFor(settings, "containers", repo))
	if !p.Any() || s.primaryIsImmutable("containers", repo) || isBeforeRestore(ctx) {
		return
	}
	held, why, err := s.anomalies.RetentionHeld(ctx, anomalyScope{Kind: anomalyScopeItem, ID: targetID})
	if err != nil {
		log.Printf("api: retention of the app backups of %q skipped: the anomaly check failed: %v", name, err) //nolint:gosec // G706: name is %q-quoted
		s.notifyRetentionFailed(ctx, "containers", appExportTagPrefix+name,
			"the anomaly check could not run, so nothing was deleted: "+scrubError(err))
		return
	}
	if held {
		log.Printf("api: retention of the app backups of %q paused: %s", name, why) //nolint:gosec // G706: name is %q-quoted
		return
	}
	tags, ok := s.retentionTagsFor(ctx, repo, mode, s.containerAppExportIdentity(name))
	if !ok {
		return
	}
	if err := s.forgetWithLockHeal(ctx, repo, p, mode, tags, false); err != nil {
		log.Printf("api: retention of the app backups of %q failed (the backups are safe): %v", name, err) //nolint:gosec // G706: name is %q-quoted
	}
}

// memberLister is the part of a fortress that tells its members and where
// each runs.
type memberLister interface {
	Members() []fortress.Member
	Status(ctx context.Context, id, lang string) (status.Report, error)
}

// appExporter is the part of a fortress that hands over the backups of the
// apps in it.
type appExporter interface {
	memberLister
	Export(ctx context.Context, id string) (io.ReadCloser, error)
}

// memberOffering finds the member that runs in container id and offers
// action, whether it is on this network or not.
func memberOffering(ctx context.Context, ml memberLister, id, action string) (fortress.Member, bool) {
	for _, m := range ml.Members() {
		if !slices.Contains(m.Actions, action) {
			continue
		}
		rep, err := ml.Status(ctx, m.ID, "en")
		if err == nil && sameContainer(rep.Instance.Container, id) {
			return m, true
		}
	}
	return fortress.Member{}, false
}

// exportingMember finds the member that runs in container id and offers a
// backup of its own. Only one on this network can hand it over.
func exportingMember(ctx context.Context, ex appExporter, id string) (fortress.Member, bool) {
	m, ok := memberOffering(ctx, ex, id, fortress.ExportAction)
	return m, ok && m.Direct
}

// sameContainer compares a container id as an app reads it, often the short
// one its hostname carries, with the full one Docker reports.
func sameContainer(reported, id string) bool {
	const short = 12
	return len(reported) >= short && len(id) >= short && (strings.HasPrefix(id, reported) || strings.HasPrefix(reported, id))
}

// storeAppExport puts the own backup of the app that runs in a container into
// the container's repository before the container is backed up, so the app's
// state comes back consistent even while it kept running. It reports whether
// one was stored.
func (s *Service) storeAppExport(ctx context.Context, name, containerID, repo string, mode restic.Mode) bool {
	f := s.fortressNow()
	if f == nil {
		return false
	}
	stored, err := s.storeAppExportFrom(ctx, f, name, containerID, repo, mode)
	if err != nil {
		log.Printf("fortress: the own backup of the app in %s: %v", name, err) //nolint:gosec // G706: a container name from Docker, %s on a log line
	}
	return stored
}

func (s *Service) storeAppExportFrom(ctx context.Context, ex appExporter, name, containerID, repo string, mode restic.Mode) (bool, error) {
	m, ok := exportingMember(ctx, ex, containerID)
	if !ok {
		return false, nil
	}
	if !mode.Encrypted {
		return false, errExportUnencrypted
	}
	ctx, cancel := context.WithTimeout(ctx, appExportWait)
	defer cancel()
	rd, err := ex.Export(ctx, m.ID)
	if err != nil {
		return false, fmt.Errorf("fetch it from %s: %w", m.App.ID, err)
	}
	defer func() { _ = rd.Close() }()
	tags := []string{appExportTagPrefix + name, appTagPrefix + m.App.ID}
	if tagSafe(m.App.Name) {
		tags = append(tags, appNameTagPrefix+m.App.Name)
	}
	tags = append(tags, s.formerlyTags(name)...)
	if _, err := s.engine.BackupStdin(ctx, repo, rd, appExportPath(name, m.App.ID), tags, mode); err != nil {
		return false, fmt.Errorf("store it: %w", err)
	}
	log.Printf("fortress: stored the own backup of %s from %s", m.App.ID, name) //nolint:gosec // G706: names from the fortress and Docker, %s on a log line
	return true, nil
}

// formerlyTags names the container's former names as its backups do, so the
// link survives in the repository when the alias table is lost. A failed read
// costs this one backup the tags, not the backup.
func (s *Service) formerlyTags(name string) []string {
	tg, err := s.store.GetTargetByContainer(name)
	if err != nil {
		return nil
	}
	aliases, err := s.store.TargetAliases("container", tg.ID)
	if err != nil {
		log.Printf("fortress: the former names of %q stay off its app backup: %v", name, err) //nolint:gosec // G706: name is %q-quoted
		return nil
	}
	tags := make([]string, 0, len(aliases))
	for _, a := range aliases {
		tags = append(tags, "formerly:"+a.OldName)
	}
	return tags
}
