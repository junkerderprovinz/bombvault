package api

import (
	"os"
	"path"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/model"
)

// TestResolveAppdataPathsIncludesNamedVolumeUnconditionally: a named volume
// has no throwaway counterpart the way a bind mount does, so it counts as data
// even without an "appdata" segment in its path.
func TestResolveAppdataPathsIncludesNamedVolumeUnconditionally(t *testing.T) {
	s := svcWithMount()
	in := model.Inspect{Mounts: []model.Mount{
		{Type: "volume", Source: "/mnt/docker/volumes/myapp_data/_data", Destination: "/data"},
	}}
	got := s.resolveAppdataPaths("myapp", in)
	want := []string{"/host/user/docker/volumes/myapp_data/_data"}
	if len(got) != 1 || got[0] != want[0] {
		t.Fatalf("resolveAppdataPaths = %v, want %v", got, want)
	}
}

func TestResolveAppdataPathsBindMounts(t *testing.T) {
	s := svcWithMount()

	t.Run("appdata bind is translated and kept", func(t *testing.T) {
		in := model.Inspect{Mounts: []model.Mount{
			{Type: "bind", Source: "/mnt/user/appdata/myapp", Destination: "/config"},
		}}
		got := s.resolveAppdataPaths("myapp", in)
		want := "/host/user/user/appdata/myapp"
		if len(got) != 1 || got[0] != want {
			t.Fatalf("resolveAppdataPaths = %v, want [%q]", got, want)
		}
	})

	t.Run("non-appdata bind (media share) is dropped, no fallback folder exists", func(t *testing.T) {
		in := model.Inspect{Mounts: []model.Mount{
			{Type: "bind", Source: "/mnt/data/media", Destination: "/media"},
		}}
		got := s.resolveAppdataPaths("myapp", in)
		if len(got) != 0 {
			t.Fatalf("resolveAppdataPaths = %v, want empty (no appdata segment, no fallback folder)", got)
		}
	})
}

func TestResolveAppdataPathsVolumeAndBindBothIncluded(t *testing.T) {
	s := svcWithMount()
	in := model.Inspect{Mounts: []model.Mount{
		{Type: "bind", Source: "/mnt/user/appdata/myapp", Destination: "/config"},
		{Type: "volume", Source: "/mnt/docker/volumes/myapp_data/_data", Destination: "/data"},
	}}
	got := s.resolveAppdataPaths("myapp", in)
	want := []string{
		"/host/user/user/appdata/myapp",
		"/host/user/docker/volumes/myapp_data/_data",
	}
	if len(got) != len(want) {
		t.Fatalf("resolveAppdataPaths = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("resolveAppdataPaths = %v, want %v", got, want)
		}
	}
}

func TestResolveAppdataPathsVolumeDedupedAgainstBind(t *testing.T) {
	s := svcWithMount()
	in := model.Inspect{Mounts: []model.Mount{
		{Type: "bind", Source: "/mnt/user/appdata/myapp", Destination: "/config"},
		{Type: "volume", Source: "/mnt/user/appdata/myapp", Destination: "/config2"},
	}}
	got := s.resolveAppdataPaths("myapp", in)
	if len(got) != 1 || got[0] != "/host/user/user/appdata/myapp" {
		t.Fatalf("resolveAppdataPaths = %v, want exactly one deduped entry", got)
	}
}

// TestResolveAppdataPathsVolumeSkippedWhenUnresolved: an empty Source means
// neither the daemon nor dockercli's VolumeInspect could resolve the volume.
func TestResolveAppdataPathsVolumeSkippedWhenUnresolved(t *testing.T) {
	s := svcWithMount()
	in := model.Inspect{Mounts: []model.Mount{
		{Type: "volume", Source: "", Destination: "/data"},
	}}
	got := s.resolveAppdataPaths("myapp", in)
	if len(got) != 0 {
		t.Fatalf("resolveAppdataPaths = %v, want empty (unresolved volume must not produce a phantom path)", got)
	}
}

// TestResolveAppdataPathsRealFallbackFolderIncluded: when no mount matches,
// the platform's conventional appdata folder is used if it exists. The other
// tests only reach this fallback with the folder missing.
func TestResolveAppdataPathsRealFallbackFolderIncluded(t *testing.T) {
	root := t.TempDir()
	s := &Service{cfg: config.Config{
		HostSourceRoot:   "/mnt", // must match Unraid{}'s fixed "/mnt/user/appdata/<name>" literal
		HostMountRoot:    root,
		DataRootSegments: []string{"appdata"},
	}}
	fallback := path.Join(root, "user/appdata/myapp")
	if err := os.MkdirAll(fallback, 0o750); err != nil {
		t.Fatalf("seed fallback dir: %v", err)
	}

	in := model.Inspect{Mounts: []model.Mount{
		{Type: "bind", Source: "/mnt/data/media", Destination: "/media"}, // non-matching, dropped
	}}
	got := s.resolveAppdataPaths("myapp", in)
	if len(got) != 1 || got[0] != fallback {
		t.Fatalf("resolveAppdataPaths = %v, want [%q] (real fallback folder must be included when it actually exists)", got, fallback)
	}
}

// TestResolveAppdataPathsVolumeUnreachableHostPathSkipped: a volume skips the
// segment filter but not the containment check, so one outside the host mount
// is skipped like a bind.
func TestResolveAppdataPathsVolumeUnreachableHostPathSkipped(t *testing.T) {
	s := svcWithMount()
	in := model.Inspect{Mounts: []model.Mount{
		{Type: "volume", Source: "/var/lib/docker/volumes/myapp_data/_data", Destination: "/data"},
	}}
	got := s.resolveAppdataPaths("myapp", in)
	if len(got) != 0 {
		t.Fatalf("resolveAppdataPaths = %v, want empty (host path unreachable through the configured mount)", got)
	}
}

func TestResolveAppdataPathsConfigurableSegmentMatches(t *testing.T) {
	s := svcWithMount()
	s.cfg.DataRootSegments = []string{"appdata", "config"}
	in := model.Inspect{Mounts: []model.Mount{
		{Type: "bind", Source: "/mnt/srv/plex/config", Destination: "/config"},
	}}
	got := s.resolveAppdataPaths("plex", in)
	want := "/host/user/srv/plex/config"
	if len(got) != 1 || got[0] != want {
		t.Fatalf("resolveAppdataPaths = %v, want [%q] (bind matching the configured \"config\" segment must be included)", got, want)
	}
}

// TestResolveAppdataPathsComposeWorkingDirBelongsToTheStack: the compose
// project directory is backed up once per stack (stack_backup.go). Adding it
// to every member would re-read and re-hash it once per service. stackDirFor
// has to find it instead.
func TestResolveAppdataPathsComposeWorkingDirBelongsToTheStack(t *testing.T) {
	s := svcWithMount()
	in := model.Inspect{
		Config: model.Config{Labels: map[string]string{
			"com.docker.compose.project":             "myapp",
			"com.docker.compose.project.working_dir": "/mnt/opt/stacks/myapp",
		}},
		Mounts: []model.Mount{
			// No configured segment, so excluded.
			{Type: "bind", Source: "/mnt/data/media", Destination: "/media"},
		},
	}
	if got := s.resolveAppdataPaths("myapp", in); len(got) != 0 {
		t.Errorf("resolveAppdataPaths = %v, want none: the project directory is the stack's, not the member's", got)
	}

	project, dir, ok := s.stackDirFor(in)
	if !ok {
		t.Fatal("stackDirFor found nothing: the directory moved out of the member and nowhere else")
	}
	if project != "myapp" {
		t.Errorf("project = %q, want %q", project, "myapp")
	}
	if want := "/host/user/opt/stacks/myapp"; dir != want {
		t.Errorf("dir = %q, want %q", dir, want)
	}
}

func TestStackDirForIgnoresNonComposeAndUnreachable(t *testing.T) {
	s := svcWithMount()

	// No compose labels at all.
	if _, _, ok := s.stackDirFor(model.Inspect{}); ok {
		t.Error("a container with no compose labels must have no stack directory")
	}

	// A project, but the working dir is outside the host mount.
	outside := model.Inspect{Config: model.Config{Labels: map[string]string{
		"com.docker.compose.project":             "myapp",
		"com.docker.compose.project.working_dir": "/somewhere/else/myapp",
	}}}
	if _, _, ok := s.stackDirFor(outside); ok {
		t.Error("a project directory outside the host mount is unreachable and must be skipped, not guessed")
	}

	// A project label without a working_dir label: nothing to back up.
	noDir := model.Inspect{Config: model.Config{Labels: map[string]string{
		"com.docker.compose.project": "myapp",
	}}}
	if _, _, ok := s.stackDirFor(noDir); ok {
		t.Error("no working_dir label means no stack directory")
	}
}

// TestResolveAppdataPathsBombvaultDataLabelOverridesSegmentFilter: the
// bombvault.data label is the documented way to include every bind of a
// container whose layout the segment filter and the compose label both miss.
func TestResolveAppdataPathsBombvaultDataLabelOverridesSegmentFilter(t *testing.T) {
	in := func(labelVal string, present bool) model.Inspect {
		labels := map[string]string{}
		if present {
			labels["bombvault.data"] = labelVal
		}
		return model.Inspect{
			Config: model.Config{Labels: labels},
			Mounts: []model.Mount{
				{Type: "bind", Source: "/mnt/srv/plex/config", Destination: "/config"},
			},
		}
	}

	t.Run("truthy value includes the otherwise-non-matching bind", func(t *testing.T) {
		s := svcWithMount()
		got := s.resolveAppdataPaths("plex", in("true", true))
		want := "/host/user/srv/plex/config"
		if len(got) != 1 || got[0] != want {
			t.Fatalf("resolveAppdataPaths = %v, want [%q]", got, want)
		}
	})

	t.Run(`label value "false" does not override`, func(t *testing.T) {
		s := svcWithMount()
		got := s.resolveAppdataPaths("plex", in("false", true))
		if len(got) != 0 {
			t.Fatalf("resolveAppdataPaths = %v, want empty (bombvault.data=false must not override the segment filter)", got)
		}
	})

	t.Run("label absent does not override", func(t *testing.T) {
		s := svcWithMount()
		got := s.resolveAppdataPaths("plex", in("", false))
		if len(got) != 0 {
			t.Fatalf("resolveAppdataPaths = %v, want empty (no bombvault.data label at all)", got)
		}
	})
}

// TestResolveAppdataPathsDefaultSegments runs on config.Load's defaults rather
// than the svcWithMount fixture: with DATA_ROOT_SEGMENTS unset, only binds with
// an appdata segment are kept.
func TestResolveAppdataPathsDefaultSegments(t *testing.T) {
	cfg, err := config.Load(map[string]string{"APP_KEY": strings.Repeat("a", 64)})
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	if len(cfg.DataRootSegments) != 1 || cfg.DataRootSegments[0] != "appdata" {
		t.Fatalf("default DataRootSegments = %v, want [\"appdata\"] (unset DATA_ROOT_SEGMENTS regression guard)", cfg.DataRootSegments)
	}
	s := &Service{cfg: cfg}

	in := model.Inspect{Mounts: []model.Mount{
		{Type: "bind", Source: "/mnt/user/appdata/myapp", Destination: "/config"},
		{Type: "bind", Source: "/mnt/data/media", Destination: "/media"},
	}}
	got := s.resolveAppdataPaths("myapp", in)
	want := "/host/user/user/appdata/myapp"
	if len(got) != 1 || got[0] != want {
		t.Fatalf("resolveAppdataPaths = %v, want [%q] (unset DATA_ROOT_SEGMENTS must reproduce pre-Task-3 behavior byte-for-byte)", got, want)
	}
}
