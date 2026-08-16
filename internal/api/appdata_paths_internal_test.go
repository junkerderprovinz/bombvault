package api

import (
	"os"
	"path"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/model"
)

// resolveAppdataPaths must treat a Docker named-volume mount (Type=="volume")
// as persistent data unconditionally — a named volume has no equivalent of a
// throwaway bind mount, so unlike a bind source it is never filtered by an
// "appdata" path segment. This is the fix for the majority case on non-Unraid
// hosts: a container using Docker Compose named volumes previously produced
// ZERO discovered data paths (#platform-expansion, direction 4/4, Task 2).

// TestResolveAppdataPathsIncludesNamedVolumeUnconditionally is the failing
// test that pins the fix: a volume mount whose resolved host path has NO
// "appdata" segment (unlike every bind case below) must still be included,
// proving the volume branch is genuinely unconditional and not just a second
// segment to match against.
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

// TestResolveAppdataPathsBindMountBehaviorUnchanged pins the EXISTING Unraid
// bind-mount behavior byte-for-byte before/after the volume branch is added:
// an appdata-segment bind is translated and kept, a non-appdata bind is
// dropped. This is the regression guard for "do not touch bind-mount logic".
func TestResolveAppdataPathsBindMountBehaviorUnchanged(t *testing.T) {
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

// TestResolveAppdataPathsVolumeAndBindBothIncluded: a container with one
// appdata bind AND one named volume gets BOTH as backup paths — the volume
// branch is additive, not a replacement for bind discovery.
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

// TestResolveAppdataPathsVolumeDedupedAgainstBind: a volume mount resolving to
// the SAME container path as an already-recorded bind must not be listed
// twice — the volume branch shares the bind branch's dedup ("seen") set.
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

// TestResolveAppdataPathsVolumeSkippedWhenUnresolved: a volume mount the
// daemon reported with no Source (and dockercli's VolumeInspect fallback also
// could not resolve it) must be skipped, not turned into a phantom empty-path
// entry — mirrors how an empty-Source bind is already skipped.
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

// TestResolveAppdataPathsRealFallbackFolderIncluded exercises the actual
// last-resort branch end to end: no bind/volume matches anything, but a REAL
// directory sits at the platform's conventional appdata path (Unraid's fixed
// "/mnt/user/appdata/<name>" literal, translated through HostSourceRoot/
// HostMountRoot) and os.Stat finds it. Every other test in this file that
// reaches the fallback (e.g. "non-appdata bind ... no fallback folder
// exists" above) only exercises the os.Stat-FAILS side, because their
// translated candidate never exists on the machine running the test. This is
// the one that actually creates the folder and confirms the OK side, which
// is also the branch platform.Platform.AppdataFallback's return value feeds
// into — the fallback this whole package now goes through.
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

// TestResolveAppdataPathsVolumeUnreachableHostPathSkipped: a volume mount whose
// resolved host path is not reachable through the configured host mount (e.g.
// Docker's default /var/lib/docker/volumes/... when HostSourceRoot is /mnt)
// goes through the SAME containment check as a bind source and is silently
// skipped, not force-included.
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

// --- Task 3: configurable data-root segments + compose-label discovery + ---
// --- per-container override (#platform-expansion, direction 4/4, Task 3) ---

// TestResolveAppdataPathsConfigurableSegmentMatches: a bind whose host source
// matches a NON-default configured segment (e.g. "config", for a
// /srv/plex/config-style layout) must be included even though it has no
// "appdata" segment at all — proving the single hardcoded literal was
// replaced by a loop over s.cfg.DataRootSegments, not just widened in place.
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

// TestResolveAppdataPathsComposeWorkingDirLabelAddsCandidate: a container
// carrying the standard com.docker.compose.project.working_dir label gets that
// directory added as a data path even when NONE of its bind mounts match any
// configured segment — the compose-label discovery source is always-on and
// additive, not gated by the segment filter.
func TestResolveAppdataPathsComposeWorkingDirLabelAddsCandidate(t *testing.T) {
	s := svcWithMount()
	in := model.Inspect{
		Config: model.Config{Labels: map[string]string{
			"com.docker.compose.project.working_dir": "/mnt/opt/stacks/myapp",
		}},
		Mounts: []model.Mount{
			// Non-matching bind (no configured segment) — must stay excluded.
			{Type: "bind", Source: "/mnt/data/media", Destination: "/media"},
		},
	}
	got := s.resolveAppdataPaths("myapp", in)
	want := "/host/user/opt/stacks/myapp"
	if len(got) != 1 || got[0] != want {
		t.Fatalf("resolveAppdataPaths = %v, want [%q] (compose working_dir label must be added regardless of bind matches)", got, want)
	}
}

// TestResolveAppdataPathsBombvaultDataLabelOverridesSegmentFilter: a
// container carrying a truthy "bombvault.data" label gets ALL of its bind
// mounts included, regardless of segment match — the documented escape hatch
// for a layout neither the segment filter nor the compose convention catches.
// An explicit "false" value must NOT trigger the override (falsy pin).
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

// TestResolveAppdataPathsDefaultSegmentsUnchanged is the regression pin: with
// DATA_ROOT_SEGMENTS UNSET, config.Load's default DataRootSegments (["appdata"])
// must reproduce the exact pre-Task-3 Unraid-only-appdata-segment behavior
// byte-for-byte — an appdata bind is kept, a non-appdata bind is dropped, same
// as TestResolveAppdataPathsBindMountBehaviorUnchanged pins directly against
// the (now removed) hardcoded literal.
func TestResolveAppdataPathsDefaultSegmentsUnchanged(t *testing.T) {
	cfg, err := config.Load(map[string]string{"APP_KEY": strings.Repeat("a", 64)})
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	if len(cfg.DataRootSegments) != 1 || cfg.DataRootSegments[0] != "appdata" {
		t.Fatalf("default DataRootSegments = %v, want [\"appdata\"] (unset DATA_ROOT_SEGMENTS regression guard)", cfg.DataRootSegments)
	}
	// config.Load's real defaults for HostSourceRoot/HostMountRoot happen to
	// equal the Unraid-shaped svcWithMount fixture, so this exercises the
	// production default end to end, not just the test fixture's shortcut.
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
