package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/restic"
)

// OffsiteEntries is the newest snapshot, as unix seconds, each entry has in its
// section's off-site copy: containers and file sets by name, VMs by libvirt
// name. Unread names a section whose off-site copy could not be listed.
type OffsiteEntries struct {
	Containers map[string]int64  `json:"containers"`
	VMs        map[string]int64  `json:"vms"`
	FileSets   map[string]int64  `json:"fileSets"`
	Unread     map[string]string `json:"unread,omitempty"`
}

// EmergencyOffsiteEntries lists what the off-site copies hold, for an
// emergency restore after the local backups went down with the old server.
// The lists of the other pages date entries from the local repositories
// alone, so they would offer none of these.
func (s *Service) EmergencyOffsiteEntries(ctx context.Context) (OffsiteEntries, error) {
	settings, err := s.store.GetSettings()
	if err != nil {
		return OffsiteEntries{}, fmt.Errorf("read settings: %w", err)
	}
	out := OffsiteEntries{Containers: map[string]int64{}, VMs: map[string]int64{}, FileSets: map[string]int64{}}
	for _, sec := range []struct {
		domain, prefix string
		into           map[string]int64
	}{
		{"containers", "container:", out.Containers},
		{"vms", "vm:", out.VMs},
		{"files", "fileset:", out.FileSets},
	} {
		target, err := s.offsiteTargetForSource(settings, sec.domain, "offsite")
		if errors.Is(err, errNoOffsiteRepo) || errors.Is(err, errOffsiteTargetsOff) {
			continue
		}
		var repo string
		if err == nil {
			repo, err = s.resolveRepo(target.Repo)
		}
		if err == nil && localRepoMissing(repo) {
			continue
		}
		if err == nil {
			err = s.newestPerTag(ctx, repo, s.offsiteModeForTarget(settings, target), sec.prefix, sec.into)
		}
		if err != nil {
			if out.Unread == nil {
				out.Unread = map[string]string{}
			}
			out.Unread[sec.domain] = scrubError(err)
		}
	}
	return out, nil
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

// handleEmergencyOffsiteEntries serves GET /api/emergency/offsite-entries.
func (h *Handler) handleEmergencyOffsiteEntries(w http.ResponseWriter, r *http.Request) {
	entries, err := h.svc.EmergencyOffsiteEntries(r.Context())
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"entries": entries}))
}
