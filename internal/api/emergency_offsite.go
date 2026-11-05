package api

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// offsiteListTimeout bounds the listing of one off-site copy. restic keeps
// retrying a dead address for many minutes, and the copies that answer should
// not wait for it.
const offsiteListTimeout = time.Minute

// OffsiteEntries is the newest snapshot, as unix seconds, each entry has in its
// section's off-site copies: containers and file sets by name, VMs by libvirt
// name. From says which copy holds that newest snapshot, as the source a restore
// takes. Unread names a section with a copy that could not be listed.
type OffsiteEntries struct {
	Containers map[string]int64  `json:"containers"`
	VMs        map[string]int64  `json:"vms"`
	FileSets   map[string]int64  `json:"fileSets"`
	From       OffsiteSources    `json:"from"`
	Unread     map[string]string `json:"unread,omitempty"`
}

// OffsiteSources names, per entry, the source that reads its newest snapshot.
type OffsiteSources struct {
	Containers map[string]string `json:"containers"`
	VMs        map[string]string `json:"vms"`
	FileSets   map[string]string `json:"fileSets"`
}

// offsiteCopy is what one off-site copy of a section lists.
type offsiteCopy struct {
	source string
	newest map[string]int64
	err    error
}

// EmergencyOffsiteEntries lists what the off-site copies hold, for an
// emergency restore after the local backups went down with the old server.
// The lists of the other pages date entries from the local repositories
// alone, so they would offer none of these. A copy that cannot be read does not
// hide the others.
func (s *Service) EmergencyOffsiteEntries(ctx context.Context) (OffsiteEntries, error) {
	settings, err := s.store.GetSettings()
	if err != nil {
		return OffsiteEntries{}, fmt.Errorf("read settings: %w", err)
	}
	out := OffsiteEntries{
		Containers: map[string]int64{}, VMs: map[string]int64{}, FileSets: map[string]int64{},
		From: OffsiteSources{Containers: map[string]string{}, VMs: map[string]string{}, FileSets: map[string]string{}},
	}
	for _, sec := range []struct {
		domain, prefix string
		into           map[string]int64
		from           map[string]string
	}{
		{"containers", "container:", out.Containers, out.From.Containers},
		{"vms", "vm:", out.VMs, out.From.VMs},
		{"files", "fileset:", out.FileSets, out.From.FileSets},
	} {
		var unread []string
		targets := s.offsiteReplicationTargets(sec.domain, settings)
		for i, list := range s.listOffsiteCopies(ctx, settings, targets, sec.prefix) {
			if list.err != nil {
				unread = append(unread, targets[i].Name+": "+scrubError(list.err))
				continue
			}
			for name, ts := range list.newest {
				if ts > sec.into[name] {
					sec.into[name], sec.from[name] = ts, list.source
				}
			}
		}
		if len(unread) > 0 {
			if out.Unread == nil {
				out.Unread = map[string]string{}
			}
			out.Unread[sec.domain] = strings.Join(unread, " ")
		}
	}
	return out, nil
}

// listOffsiteCopies lists every target at once, so a dead one costs its timeout
// and no more. The answers come in the order of targets.
func (s *Service) listOffsiteCopies(ctx context.Context, settings store.Settings, targets []store.OffsiteTarget, prefix string) []offsiteCopy {
	out := make([]offsiteCopy, len(targets))
	var wg sync.WaitGroup
	for i, target := range targets {
		wg.Go(func() {
			lctx, cancel := context.WithTimeout(ctx, offsiteListTimeout)
			defer cancel()
			out[i] = s.listOffsiteCopy(lctx, settings, target, prefix)
		})
	}
	wg.Wait()
	return out
}

func (s *Service) listOffsiteCopy(ctx context.Context, settings store.Settings, target store.OffsiteTarget, prefix string) offsiteCopy {
	list := offsiteCopy{source: "offsite", newest: map[string]int64{}}
	if target.ID != "" {
		list.source = offsiteSourcePrefix + target.ID
	}
	repo, err := s.resolveRepo(target.Repo)
	if err != nil {
		list.err = err
		return list
	}
	if localRepoMissing(repo) {
		return list
	}
	list.err = s.newestPerTag(ctx, repo, s.offsiteModeForTarget(settings, target), prefix, list.newest)
	return list
}

// newestPerTag puts the newest snapshot time of every name tagged prefix+name
// in repo into into.
func (s *Service) newestPerTag(ctx context.Context, repo string, mode restic.Mode, prefix string, into map[string]int64) error {
	all, err := s.listSnapshots(ctx, repo, mode)
	if err != nil {
		return err
	}
	for _, snap := range all {
		ts, err := time.Parse(time.RFC3339Nano, snap.Time)
		if err != nil {
			continue
		}
		for _, tag := range snap.Tags {
			if name, ok := strings.CutPrefix(tag, prefix); ok && name != "" && ts.Unix() > into[name] {
				into[name] = ts.Unix()
			}
		}
	}
	return nil
}

// AdoptOffsiteFileSets gives every folder set an off-site copy holds a row of
// its own, disabled and path-less, as DiscoverFileSets does for a local
// repository. A restore goes by that row.
func (s *Service) AdoptOffsiteFileSets(entries OffsiteEntries) error {
	var failed []error
	for name := range entries.FileSets {
		if !validResourceName(name) {
			log.Printf("api: emergency: skipping unsafe file set name %q", name) //nolint:gosec // G706: %q-quoted
			continue
		}
		if _, err := s.store.GetFileSetByName(name); err == nil {
			continue
		}
		if _, err := s.store.CreateFileSet(store.FileSet{Name: name, Enabled: false}); err != nil {
			failed = append(failed, fmt.Errorf("adopt file set %q: %w", name, err))
		}
	}
	return errors.Join(failed...)
}

// handleEmergencyOffsiteEntries serves GET /api/emergency/offsite-entries.
func (h *Handler) handleEmergencyOffsiteEntries(w http.ResponseWriter, r *http.Request) {
	entries, err := h.svc.EmergencyOffsiteEntries(r.Context())
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"entries": entries}))
}

// handleEmergencyOffsiteAdopt serves POST /api/emergency/offsite-entries: the
// same list, with a row for each folder set in it.
func (h *Handler) handleEmergencyOffsiteAdopt(w http.ResponseWriter, r *http.Request) {
	if !crossOriginGuard(w, r) {
		return
	}
	entries, err := h.svc.EmergencyOffsiteEntries(r.Context())
	if err == nil {
		err = h.svc.AdoptOffsiteFileSets(entries)
	}
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"entries": entries}))
}

// handleEmergencyOffsiteOpen serves POST /api/emergency/offsite-open with the
// body {domain, source}. It opens one of this server's off-site copies the way
// the Restore page opens an old folder, for what cannot be recreated from it.
func (h *Handler) handleEmergencyOffsiteOpen(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Domain string `json:"domain"`
		Source string `json:"source"`
	}
	if !decodeBody(w, r, &body) {
		return
	}
	session, err := h.svc.OpenOffsiteCopy(r.Context(), body.Domain, body.Source)
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"session": session}))
}

// OpenOffsiteCopy opens the off-site copy a source names as a read-only session
// under this server's key and sign-ins. The address and the sign-in are this
// server's own, unlike the ones an old folder is opened with.
func (s *Service) OpenOffsiteCopy(_ context.Context, domain, source string) (string, error) {
	if !isOffsiteSource(source) || !validOffsiteDomain(domain) {
		return "", errNoOffsiteRepo
	}
	settings, err := s.store.GetSettings()
	if err != nil {
		return "", fmt.Errorf("read settings: %w", err)
	}
	target, err := s.offsiteTargetForSource(settings, domain, source)
	if err != nil {
		return "", err
	}
	repo, err := s.resolveRepo(target.Repo)
	if err != nil {
		return "", err
	}
	key, err := s.backupKey()
	if err != nil {
		return "", err
	}
	mode := s.offsiteModeForTarget(settings, target)
	mode.NoLock = true
	return s.openOwnRepoSession(repo, key, mode)
}

// openOwnRepoSession opens a session on a repository of this server. It is for
// reading only, so a lock in the repository does not stand in its way.
func (s *Service) openOwnRepoSession(repo, key string, mode restic.Mode) (string, error) {
	id, err := newForeignSessionID()
	if err != nil {
		return "", err
	}
	now := time.Now()
	s.foreignMu.Lock()
	defer s.foreignMu.Unlock()
	if s.foreignSessions == nil {
		s.foreignSessions = map[string]foreignSession{}
	}
	s.sweepForeignLocked(now)
	s.foreignSessions[id] = foreignSession{id: id, repo: repo, key: key, mode: mode, expires: now.Add(foreignSessionTTL)}
	s.startForeignJanitorLocked()
	return id, nil
}
