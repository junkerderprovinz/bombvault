package api

import (
	"context"
	"errors"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/restic"
)

type perRepoEngine struct {
	ResticEngine
	snaps map[string][]restic.Snapshot
	errs  map[string]error
}

func (e perRepoEngine) Snapshots(_ context.Context, repo string, _ restic.Mode) ([]restic.Snapshot, error) {
	return e.snaps[repo], e.errs[repo]
}

// After a disaster the local repositories may be gone with the old server,
// so the emergency lists what each section's off-site copy holds.
func TestEmergencyOffsiteEntriesListWhatTheOffsiteCopiesHold(t *testing.T) {
	const containers, vms, files = "rest:http://off/containers", "rest:http://off/vms", "rest:http://off/files"
	st := newAliasLinkStore(t)
	settings, err := st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	settings.ContainersOffsite, settings.VMsOffsite, settings.FilesOffsite = containers, vms, files
	if err := st.UpdateSettings(settings); err != nil {
		t.Fatal(err)
	}
	eng := perRepoEngine{
		snaps: map[string][]restic.Snapshot{
			containers: {
				{ID: "a1", Time: "2026-09-01T03:00:00Z", Tags: []string{"container:plex"}},
				{ID: "a2", Time: "2026-09-02T03:00:00Z", Tags: []string{"container:plex"}},
				{ID: "a3", Time: "2026-09-01T03:00:00Z", Tags: []string{"container:sonarr"}},
			},
			vms: {{ID: "b1", Time: "2026-09-03T03:00:00Z", Tags: []string{"vm:win11"}}},
		},
		errs: map[string]error{files: errors.New("Fatal: unable to open config file: 403 Forbidden")},
	}
	s := &Service{store: st, engine: eng}

	got, err := s.EmergencyOffsiteEntries(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got.Containers["plex"] != 1788318000 || got.Containers["sonarr"] == 0 || len(got.Containers) != 2 {
		t.Errorf("containers = %v", got.Containers)
	}
	if got.VMs["win11"] == 0 || len(got.VMs) != 1 {
		t.Errorf("vms = %v", got.VMs)
	}
	if len(got.FileSets) != 0 || got.Unread["files"] == "" {
		t.Errorf("file sets = %v, unread = %v; want the unreadable copy named", got.FileSets, got.Unread)
	}
}
