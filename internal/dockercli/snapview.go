package dockercli

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/mount"
)

// SnapViewLabel marks the containers that show a ZFS snapshot at the paths a
// container backup reads. Nothing without it is ever removed as one.
const SnapViewLabel = "bombvault.snapview"

// SnapViewInstanceLabel names the BombVault instance a view belongs to, so
// instances sharing one Docker host clear only their own.
const SnapViewInstanceLabel = "bombvault.snapview.instance"

// SnapViewPrefix starts the name of every view.
const SnapViewPrefix = "bombvault-snapview-"

// SnapViewBind puts the snapshot directory Source, a host path, read-only at
// Target, a path of this container.
type SnapViewBind struct {
	Source string
	Target string
}

// SnapViewHost is what a container backup that reads a snapshot needs from
// Docker beyond the Docker interface.
type SnapViewHost interface {
	StartSnapView(ctx context.Context, name, instance string, binds []SnapViewBind) error
	RemoveSnapView(ctx context.Context, name string) error
	SnapViewLeftovers(ctx context.Context, instance string) ([]string, error)
}

var _ SnapViewHost = (*Client)(nil)

var errNotSnapView = errors.New("not a snapshot view")

// StartSnapView starts a view: a container of this container's own image and
// mounts, with binds laid over them. It shares this container's process and
// network namespaces, so a process here finds its root at /proc/<pid>/root and
// sees the snapshots exactly where it would see the live data. Its command line
// carries name, which is how that pid is found. A view whose start fails is
// removed again.
func (c *Client) StartSnapView(ctx context.Context, name, instance string, binds []SnapViewBind) error {
	if !strings.HasPrefix(name, SnapViewPrefix) {
		return fmt.Errorf("dockercli: snapshot view %q: %w", name, errNotSnapView)
	}
	ref := ownContainerRef()
	if ref == "" {
		return errors.New("dockercli: snapshot view: this process does not run in a container")
	}
	self, err := c.api.ContainerInspect(ctx, ref)
	if err != nil {
		return fmt.Errorf("dockercli: snapshot view: inspect this container: %w", err)
	}
	cfg, hostCfg := snapViewConfig(self, name, instance, binds)
	if _, err := c.api.ContainerCreate(ctx, cfg, hostCfg, nil, nil, name); err != nil {
		return fmt.Errorf("dockercli: create snapshot view: %w", err)
	}
	if err := c.api.ContainerStart(ctx, name, container.StartOptions{}); err != nil {
		rctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
		defer cancel()
		_ = c.RemoveSnapView(rctx, name)
		return fmt.Errorf("dockercli: start snapshot view: %w", err)
	}
	return nil
}

// snapViewConfig builds a view from the inspect of this container. It keeps
// every bind, volume and tmpfs this container has, so restic finds the
// repositories, its cache and its credentials where it always does, and lays
// the snapshot binds over them. It holds nothing and may do nothing: no
// capabilities, no restart, a few processes.
func snapViewConfig(self container.InspectResponse, name, instance string, binds []SnapViewBind) (*container.Config, *container.HostConfig) {
	var mounts []mount.Mount
	for _, m := range self.Mounts {
		switch m.Type {
		case mount.TypeBind:
			mounts = append(mounts, mount.Mount{
				Type: mount.TypeBind, Source: m.Source, Target: m.Destination, ReadOnly: !m.RW,
				BindOptions: &mount.BindOptions{Propagation: m.Propagation},
			})
		case mount.TypeVolume:
			mounts = append(mounts, mount.Mount{Type: mount.TypeVolume, Source: m.Name, Target: m.Destination, ReadOnly: !m.RW})
		case mount.TypeTmpfs:
			mounts = append(mounts, mount.Mount{Type: mount.TypeTmpfs, Target: m.Destination})
		}
	}
	for _, b := range binds {
		mounts = append(mounts, mount.Mount{
			Type: mount.TypeBind, Source: b.Source, Target: b.Target, ReadOnly: true,
			BindOptions: &mount.BindOptions{Propagation: mount.PropagationRPrivate},
		})
	}

	labels := map[string]string{SnapViewLabel: "1"}
	if instance != "" {
		labels[SnapViewInstanceLabel] = instance
	}
	cfg := &container.Config{
		Image:      self.Image,
		Entrypoint: []string{"sh", "-c", "while :; do sleep 3600; done", name},
		Labels:     labels,
	}
	pids := int64(16)
	hostCfg := &container.HostConfig{
		Mounts:        mounts,
		NetworkMode:   container.NetworkMode("container:" + self.ID),
		PidMode:       container.PidMode("container:" + self.ID),
		RestartPolicy: container.RestartPolicy{Name: container.RestartPolicyDisabled},
		CapDrop:       []string{"ALL"},
		SecurityOpt:   []string{"no-new-privileges"},
	}
	if self.HostConfig != nil {
		hostCfg.Tmpfs = self.HostConfig.Tmpfs
	}
	hostCfg.NanoCPUs = 250_000_000
	hostCfg.Memory = 32 << 20
	hostCfg.PidsLimit = &pids
	return cfg, hostCfg
}

// RemoveSnapView removes a view. One that is already gone counts as removed;
// a container that is not a view is refused.
func (c *Client) RemoveSnapView(ctx context.Context, name string) error {
	resp, err := c.api.ContainerInspect(ctx, name)
	if err != nil {
		if isNoSuchContainer(err) {
			return nil
		}
		return fmt.Errorf("dockercli: inspect snapshot view: %w", err)
	}
	if resp.Config == nil || resp.Config.Labels[SnapViewLabel] != "1" || !strings.HasPrefix(normalizeName(resp.Name), SnapViewPrefix) {
		return fmt.Errorf("dockercli: container %q: %w", name, errNotSnapView)
	}
	err = c.api.ContainerRemove(ctx, resp.ID, container.RemoveOptions{Force: true, RemoveVolumes: true})
	if err != nil && !isNoSuchContainer(err) {
		return fmt.Errorf("dockercli: remove snapshot view: %w", err)
	}
	return nil
}

// SnapViewLeftovers names the views this instance left behind.
func (c *Client) SnapViewLeftovers(ctx context.Context, instance string) ([]string, error) {
	f := filters.NewArgs(filters.Arg("label", SnapViewLabel+"=1"))
	if instance != "" {
		f.Add("label", SnapViewInstanceLabel+"="+instance)
	}
	cs, err := c.api.ContainerList(ctx, container.ListOptions{All: true, Filters: f})
	if err != nil {
		return nil, fmt.Errorf("dockercli: list snapshot views: %w", err)
	}
	var out []string
	for _, s := range cs {
		if len(s.Names) > 0 && strings.HasPrefix(normalizeName(s.Names[0]), SnapViewPrefix) {
			out = append(out, normalizeName(s.Names[0]))
		}
	}
	return out, nil
}
