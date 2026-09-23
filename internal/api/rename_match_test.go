package api

import (
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/dockercli"
	"github.com/junkerderprovinz/bombvault/internal/model"
)

// A `docker rename` on the CLI keeps the container's ID, so an exact ID match
// is enough on its own.
func TestMatchRenamesSameDockerID(t *testing.T) {
	live := []dockercli.ContainerInfo{
		{Name: "plex2", ID: "abc123"},
	}
	orphans := []orphanEntry{
		{Name: "plex", Inspect: model.Inspect{ID: "abc123"}},
	}

	got := matchRenames(live, orphans, templateLineage{}, []string{"appdata"})

	want := RenameCandidate{OldName: "plex", Reason: reasonDockerID}
	if got["plex2"] != want {
		t.Fatalf("matchRenames()[%q] = %+v, want %+v", "plex2", got["plex2"], want)
	}
}

// An identical appdata bind (source -> destination), bound by this one live
// container and this one entry alone, is enough on its own.
func TestMatchRenamesSameAppdataBind(t *testing.T) {
	live := []dockercli.ContainerInfo{
		{
			Name: "radarr2",
			ID:   "live-1",
			Mounts: []dockercli.MountPoint{
				{Source: "/mnt/user/appdata/radarr", Destination: "/config"},
			},
		},
	}
	orphans := []orphanEntry{
		{
			Name: "radarr",
			Inspect: model.Inspect{
				ID: "orphan-1",
				Mounts: []model.Mount{
					{Type: "bind", Source: "/mnt/user/appdata/radarr", Destination: "/config"},
				},
			},
		},
	}

	got := matchRenames(live, orphans, templateLineage{}, []string{"appdata"})

	want := RenameCandidate{OldName: "radarr", Reason: reasonAppdataBind}
	if got["radarr2"] != want {
		t.Fatalf("matchRenames()[%q] = %+v, want %+v", "radarr2", got["radarr2"], want)
	}
}

// A folder bound by two live containers is evidence for neither, even though
// the bind set would otherwise line up with the entry.
func TestMatchRenamesSharedAppdataFolderNoMatch(t *testing.T) {
	sharedMount := []dockercli.MountPoint{
		{Source: "/mnt/user/appdata/shared", Destination: "/config"},
	}
	live := []dockercli.ContainerInfo{
		{Name: "app-a", ID: "live-a", Mounts: sharedMount},
		{Name: "app-b", ID: "live-b", Mounts: sharedMount},
	}
	orphans := []orphanEntry{
		{
			Name: "app-old",
			Inspect: model.Inspect{
				ID: "orphan-1",
				Mounts: []model.Mount{
					{Type: "bind", Source: "/mnt/user/appdata/shared", Destination: "/config"},
				},
			},
		},
	}

	got := matchRenames(live, orphans, templateLineage{}, []string{"appdata"})

	if _, ok := got["app-a"]; ok {
		t.Errorf("matchRenames()[%q] = %+v, want no match (shared folder)", "app-a", got["app-a"])
	}
	if _, ok := got["app-b"]; ok {
		t.Errorf("matchRenames()[%q] = %+v, want no match (shared folder)", "app-b", got["app-b"])
	}
}

// Container A's bind set equals the entry's. B carries an extra bind, so it
// can never match on set equality and only makes the shared source
// asymmetric. The shared folder is bound by two live containers, so it is not
// exclusive and A must not match on set equality alone.
func TestMatchRenamesAppdataBindExclusivityAsymmetric(t *testing.T) {
	live := []dockercli.ContainerInfo{
		{
			Name: "app-a",
			ID:   "live-a",
			Mounts: []dockercli.MountPoint{
				{Source: "/mnt/user/appdata/shared", Destination: "/config"},
			},
		},
		{
			Name: "app-b",
			ID:   "live-b",
			Mounts: []dockercli.MountPoint{
				{Source: "/mnt/user/appdata/shared", Destination: "/config"},
				{Source: "/mnt/user/appdata/only-b", Destination: "/other"},
			},
		},
	}
	orphans := []orphanEntry{
		{
			Name: "app-old",
			Inspect: model.Inspect{
				ID: "orphan-1",
				Mounts: []model.Mount{
					{Type: "bind", Source: "/mnt/user/appdata/shared", Destination: "/config"},
				},
			},
		},
	}

	got := matchRenames(live, orphans, templateLineage{}, []string{"appdata"})

	if _, ok := got["app-a"]; ok {
		t.Errorf("matchRenames()[%q] = %+v, want no match: its bind set equals the entry's, but /mnt/user/appdata/shared is also bound by app-b, so the folder is not exclusive", "app-a", got["app-a"])
	}
	if _, ok := got["app-b"]; ok {
		t.Errorf("matchRenames()[%q] = %+v, want no match (its bind set does not equal the entry's)", "app-b", got["app-b"])
	}
}

// Docker allows one host path at two destinations. A bind set keyed by source
// alone would keep only the last mount on each side, so it is keyed by the
// (source, destination) pair and the full identical set still matches.
func TestMatchRenamesAppdataBindSameSourceTwoDestinations(t *testing.T) {
	live := []dockercli.ContainerInfo{
		{
			Name: "radarr2",
			ID:   "live-1",
			Mounts: []dockercli.MountPoint{
				{Source: "/mnt/user/appdata/radarr", Destination: "/config"},
				{Source: "/mnt/user/appdata/radarr", Destination: "/config2"},
			},
		},
	}
	orphans := []orphanEntry{
		{
			Name: "radarr",
			Inspect: model.Inspect{
				ID: "orphan-1",
				Mounts: []model.Mount{
					{Type: "bind", Source: "/mnt/user/appdata/radarr", Destination: "/config"},
					{Type: "bind", Source: "/mnt/user/appdata/radarr", Destination: "/config2"},
				},
			},
		},
	}

	got := matchRenames(live, orphans, templateLineage{}, []string{"appdata"})

	want := RenameCandidate{OldName: "radarr", Reason: reasonAppdataBind}
	if got["radarr2"] != want {
		t.Fatalf("matchRenames()[%q] = %+v, want %+v", "radarr2", got["radarr2"], want)
	}
}

// The live container mounts one host path at two destinations and the entry
// stored only one of them. A source-keyed set could collapse the live side
// onto the entry's destination and report an identical set. Keyed by pair the
// sets differ in size, so this must not match.
func TestMatchRenamesAppdataBindSameSourceExtraDestinationNotCollapsed(t *testing.T) {
	live := []dockercli.ContainerInfo{
		{
			Name: "radarr2",
			ID:   "live-1",
			Mounts: []dockercli.MountPoint{
				{Source: "/mnt/user/appdata/radarr", Destination: "/config"},
				{Source: "/mnt/user/appdata/radarr", Destination: "/config2"},
			},
		},
	}
	orphans := []orphanEntry{
		{
			Name: "radarr",
			Inspect: model.Inspect{
				ID: "orphan-1",
				Mounts: []model.Mount{
					// The destination a source-keyed map would keep (the last
					// one above).
					{Type: "bind", Source: "/mnt/user/appdata/radarr", Destination: "/config2"},
				},
			},
		},
	}

	got := matchRenames(live, orphans, templateLineage{}, []string{"appdata"})

	if _, ok := got["radarr2"]; ok {
		t.Errorf("matchRenames()[%q] = %+v, want no match: the entry is missing the /config destination the live container actually has", "radarr2", got["radarr2"])
	}
}

// An image match only confirms: two live containers off the same image, and
// an entry that used it, never match on the image alone.
func TestMatchRenamesTwoInstancesSameImageNoMatch(t *testing.T) {
	live := []dockercli.ContainerInfo{
		{Name: "radarr-a", ID: "live-a", Image: "linuxserver/radarr:latest"},
		{Name: "radarr-b", ID: "live-b", Image: "linuxserver/radarr:latest"},
	}
	orphans := []orphanEntry{
		{
			Name: "radarr-old",
			Inspect: model.Inspect{
				ID: "orphan-1",
				Config: model.Config{
					Image: "linuxserver/radarr:latest",
				},
			},
		},
	}

	got := matchRenames(live, orphans, templateLineage{}, []string{"appdata"})

	if len(got) != 0 {
		t.Fatalf("matchRenames() = %+v, want empty (image alone never matches)", got)
	}
}

// Matching compose project and service labels are enough on their own.
func TestMatchRenamesSameComposeProjectAndService(t *testing.T) {
	labels := map[string]string{
		"com.docker.compose.project": "myapp",
		"com.docker.compose.service": "web",
	}
	live := []dockercli.ContainerInfo{
		{Name: "web2", ID: "live-1", Labels: labels},
	}
	orphans := []orphanEntry{
		{
			Name: "web",
			Inspect: model.Inspect{
				ID:     "orphan-1",
				Config: model.Config{Labels: labels},
			},
		},
	}

	got := matchRenames(live, orphans, templateLineage{}, []string{"appdata"})

	want := RenameCandidate{OldName: "web", Reason: reasonComposeService}
	if got["web2"] != want {
		t.Fatalf("matchRenames()[%q] = %+v, want %+v", "web2", got["web2"], want)
	}
}

// The old template is gone, the new one is present, and its content matches
// the entry's stored template apart from <Name> and <DateInstalled>.
func TestMatchRenamesTemplateLineage(t *testing.T) {
	oldXML := `<Container version="2"><Name>radarr</Name><DateInstalled>1690000000</DateInstalled><Repository>linuxserver/radarr</Repository></Container>`
	newXML := `<Container version="2"><Name>radarr2</Name><DateInstalled>1699999999</DateInstalled><Repository>linuxserver/radarr</Repository></Container>`

	live := []dockercli.ContainerInfo{
		{Name: "radarr2", ID: "live-1"},
	}
	orphans := []orphanEntry{
		{Name: "radarr", Inspect: model.Inspect{ID: "orphan-1"}, TemplateXML: oldXML},
	}
	tmpl := templateLineage{
		Present:   map[string]bool{"radarr": false, "radarr2": true},
		XMLByName: map[string]string{"radarr2": newXML},
	}

	got := matchRenames(live, orphans, tmpl, []string{"appdata"})

	want := RenameCandidate{OldName: "radarr", Reason: reasonTemplateLineage}
	if got["radarr2"] != want {
		t.Fatalf("matchRenames()[%q] = %+v, want %+v", "radarr2", got["radarr2"], want)
	}
}

// The one-to-one rule holds in both directions: an entry with two live
// candidates, and a live container with two entry candidates, both fall back
// to the manual picker instead of guessing.
func TestMatchRenamesTwoCandidatesNoMatch(t *testing.T) {
	// One entry, two live candidates (both compose-match it).
	oneEntryLabels := map[string]string{
		"com.docker.compose.project": "fanout",
		"com.docker.compose.service": "svc",
	}
	// One live container, two entry candidates (both compose-match it).
	oneLiveLabels := map[string]string{
		"com.docker.compose.project": "converge",
		"com.docker.compose.service": "svc",
	}

	live := []dockercli.ContainerInfo{
		{Name: "fanout-a", ID: "live-fa", Labels: oneEntryLabels},
		{Name: "fanout-b", ID: "live-fb", Labels: oneEntryLabels},
		{Name: "converge", ID: "live-c", Labels: oneLiveLabels},
	}
	orphans := []orphanEntry{
		{Name: "fanout-old", Inspect: model.Inspect{ID: "orphan-f", Config: model.Config{Labels: oneEntryLabels}}},
		{Name: "converge-old-1", Inspect: model.Inspect{ID: "orphan-c1", Config: model.Config{Labels: oneLiveLabels}}},
		{Name: "converge-old-2", Inspect: model.Inspect{ID: "orphan-c2", Config: model.Config{Labels: oneLiveLabels}}},
	}

	got := matchRenames(live, orphans, templateLineage{}, []string{"appdata"})

	for _, name := range []string{"fanout-a", "fanout-b", "converge"} {
		if _, ok := got[name]; ok {
			t.Errorf("matchRenames()[%q] = %+v, want no match (not one-to-one)", name, got[name])
		}
	}
}

// When the appdata folder was renamed along with the container, none of the
// hard signals fire and the case is left to the manual picker. Snapshots are
// not compared file by file.
func TestMatchRenamesRenamedAppdataFolderNoMatch(t *testing.T) {
	live := []dockercli.ContainerInfo{
		{
			Name: "sonarr2",
			ID:   "live-new",
			Mounts: []dockercli.MountPoint{
				{Source: "/mnt/user/appdata/sonarr2", Destination: "/config"},
			},
		},
	}
	orphans := []orphanEntry{
		{
			Name: "sonarr",
			Inspect: model.Inspect{
				ID: "orphan-old",
				Mounts: []model.Mount{
					{Type: "bind", Source: "/mnt/user/appdata/sonarr", Destination: "/config"},
				},
			},
			TemplateXML: `<Container><Name>sonarr</Name></Container>`,
		},
	}
	// Neither template exists on the flash under either name: no lineage signal.
	tmpl := templateLineage{Present: map[string]bool{}}

	got := matchRenames(live, orphans, tmpl, []string{"appdata"})

	if _, ok := got["sonarr2"]; ok {
		t.Errorf("matchRenames()[%q] = %+v, want no match (appdata folder renamed too, no hard signal)", "sonarr2", got["sonarr2"])
	}
}

// A bind under an operator-configured data-root segment (DATA_ROOT_SEGMENTS)
// other than "appdata" still gets the bind match. The match disappears again
// once only the default segment is passed in its place, so the signal really
// follows the caller's list instead of a hardcoded "appdata".
func TestMatchRenamesConfiguredSegmentMatches(t *testing.T) {
	live := []dockercli.ContainerInfo{
		{
			Name: "plex2",
			ID:   "live-1",
			Mounts: []dockercli.MountPoint{
				{Source: "/mnt/user/config/plex", Destination: "/config"},
			},
		},
	}
	orphans := []orphanEntry{
		{
			Name: "plex",
			Inspect: model.Inspect{
				ID: "orphan-1",
				Mounts: []model.Mount{
					{Type: "bind", Source: "/mnt/user/config/plex", Destination: "/config"},
				},
			},
		},
	}

	got := matchRenames(live, orphans, templateLineage{}, []string{"config"})
	want := RenameCandidate{OldName: "plex", Reason: reasonAppdataBind}
	if got["plex2"] != want {
		t.Fatalf("matchRenames()[%q] = %+v, want %+v", "plex2", got["plex2"], want)
	}

	got = matchRenames(live, orphans, templateLineage{}, []string{"appdata"})
	if _, ok := got["plex2"]; ok {
		t.Errorf("matchRenames()[%q] = %+v, want no match: the bind is under \"config\", not the default \"appdata\"", "plex2", got["plex2"])
	}
}

// The default segment list, exactly what an unset DATA_ROOT_SEGMENTS resolves
// to, still matches a plain /appdata/ bind.
func TestMatchRenamesDefaultSegmentStillMatches(t *testing.T) {
	live := []dockercli.ContainerInfo{
		{
			Name: "plex2",
			ID:   "live-1",
			Mounts: []dockercli.MountPoint{
				{Source: "/mnt/user/appdata/plex", Destination: "/config"},
			},
		},
	}
	orphans := []orphanEntry{
		{
			Name: "plex",
			Inspect: model.Inspect{
				ID: "orphan-1",
				Mounts: []model.Mount{
					{Type: "bind", Source: "/mnt/user/appdata/plex", Destination: "/config"},
				},
			},
		},
	}

	got := matchRenames(live, orphans, templateLineage{}, []string{"appdata"})

	want := RenameCandidate{OldName: "plex", Reason: reasonAppdataBind}
	if got["plex2"] != want {
		t.Fatalf("matchRenames()[%q] = %+v, want %+v", "plex2", got["plex2"], want)
	}
}
