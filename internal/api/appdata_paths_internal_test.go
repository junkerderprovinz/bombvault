package api

import (
	"testing"

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
