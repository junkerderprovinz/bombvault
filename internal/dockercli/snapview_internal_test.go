package dockercli

import (
	"slices"
	"testing"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
)

func TestSnapViewSeesWhatThisContainerSeesWithTheSnapshotsOnTop(t *testing.T) {
	self := container.InspectResponse{
		ContainerJSONBase: &container.ContainerJSONBase{
			ID: "abc123", Image: "sha256:img",
			HostConfig: &container.HostConfig{Tmpfs: map[string]string{"/run": ""}},
		},
		Mounts: []container.MountPoint{
			{Type: mount.TypeBind, Source: "/mnt", Destination: "/host/user", RW: true, Propagation: mount.PropagationRSlave},
			{Type: mount.TypeBind, Source: "/mnt/user/appdata/bombvault", Destination: "/config", RW: true},
			{Type: mount.TypeVolume, Name: "bv-cache", Destination: "/cache", RW: false},
		},
	}
	binds := []SnapViewBind{{
		Source: "/mnt/cache/appdata/.zfs/snapshot/bombvault-20261015120000/plex",
		Target: "/host/user/user/appdata/plex",
	}}
	cfg, host := snapViewConfig(self, SnapViewPrefix+"run1", "inst", binds)

	if cfg.Image != "sha256:img" {
		t.Errorf("image = %q, want this container's own", cfg.Image)
	}
	if !slices.Contains(cfg.Entrypoint, SnapViewPrefix+"run1") {
		t.Errorf("entrypoint %q does not carry the name the pid is found by", cfg.Entrypoint)
	}
	if cfg.Labels[SnapViewLabel] != "1" || cfg.Labels[SnapViewInstanceLabel] != "inst" {
		t.Errorf("labels = %v", cfg.Labels)
	}
	if host.PidMode != "container:abc123" || host.NetworkMode != "container:abc123" {
		t.Errorf("pid %q, network %q: the view has to share this container's namespaces", host.PidMode, host.NetworkMode)
	}
	if !slices.Equal(host.CapDrop, []string{"ALL"}) {
		t.Errorf("cap drop = %v; a view with capabilities this container lacks cannot be entered", host.CapDrop)
	}
	if host.Tmpfs["/run"] != "" || len(host.Tmpfs) != 1 {
		t.Errorf("tmpfs = %v", host.Tmpfs)
	}

	want := []mount.Mount{
		{Type: mount.TypeBind, Source: "/mnt", Target: "/host/user", BindOptions: &mount.BindOptions{Propagation: mount.PropagationRSlave}},
		{Type: mount.TypeBind, Source: "/mnt/user/appdata/bombvault", Target: "/config", BindOptions: &mount.BindOptions{}},
		{Type: mount.TypeVolume, Source: "bv-cache", Target: "/cache", ReadOnly: true},
		{Type: mount.TypeBind, Source: binds[0].Source, Target: binds[0].Target, ReadOnly: true,
			BindOptions: &mount.BindOptions{Propagation: mount.PropagationRPrivate}},
	}
	if len(host.Mounts) != len(want) {
		t.Fatalf("mounts = %+v", host.Mounts)
	}
	for i := range want {
		got := host.Mounts[i]
		if got.Type != want[i].Type || got.Source != want[i].Source || got.Target != want[i].Target || got.ReadOnly != want[i].ReadOnly {
			t.Errorf("mount %d = %+v, want %+v", i, got, want[i])
		}
		if want[i].BindOptions != nil && (got.BindOptions == nil || got.BindOptions.Propagation != want[i].BindOptions.Propagation) {
			t.Errorf("mount %d propagation = %+v, want %+v", i, got.BindOptions, want[i].BindOptions)
		}
	}
}
