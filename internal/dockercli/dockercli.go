// Package dockercli wraps the official Docker SDK behind the Docker interface
// (consumed by the backup orchestrator) so host control is mockable in tests.
// The orchestrator depends only on the interface in types.go; the concrete
// Client here is wired exclusively in cmd/bombvault.
package dockercli

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
)

// Client is the real Docker adapter over the official SDK, talking to the
// mounted docker.sock.
type Client struct {
	api *client.Client
}

// compile-time interface check.
var _ Docker = (*Client)(nil)

// New constructs a Client connected to the host docker.sock with API-version
// negotiation (so it works across Unraid Docker versions).
func New() (*Client, error) {
	api, err := client.NewClientWithOpts(
		client.FromEnv,
		client.WithHost("unix:///var/run/docker.sock"),
		client.WithAPIVersionNegotiation(),
	)
	if err != nil {
		return nil, fmt.Errorf("dockercli: new client: %w", err)
	}
	return &Client{api: api}, nil
}

// Close releases the underlying SDK client.
func (c *Client) Close() error { return c.api.Close() }

// List returns all containers (running and stopped).
func (c *Client) List(ctx context.Context) ([]ContainerInfo, error) {
	summaries, err := c.api.ContainerList(ctx, container.ListOptions{All: true})
	if err != nil {
		return nil, fmt.Errorf("dockercli: list: %w", err)
	}
	out := make([]ContainerInfo, 0, len(summaries))
	for _, s := range summaries {
		name := ""
		if len(s.Names) > 0 {
			name = normalizeName(s.Names[0])
		}
		out = append(out, ContainerInfo{
			ID:     s.ID,
			Name:   name,
			Image:  s.Image,
			State:  string(s.State),
			Status: s.Status,
		})
	}
	return out, nil
}

// Inspect returns the captured inspect subset for a container by name or ID.
func (c *Client) Inspect(ctx context.Context, name string) (ContainerInspect, error) {
	resp, err := c.api.ContainerInspect(ctx, name)
	if err != nil {
		return ContainerInspect{}, fmt.Errorf("dockercli: inspect: %w", err)
	}
	return mapInspect(resp), nil
}

// Stop stops a container, sending SIGKILL after timeout if it has not exited.
// The Docker API expresses the grace period in whole seconds; a positive
// sub-second timeout is rounded up to 1s so it never collapses to an immediate
// SIGKILL.
func (c *Client) Stop(ctx context.Context, name string, timeout time.Duration) error {
	secs := int(timeout.Seconds())
	if secs == 0 && timeout > 0 {
		secs = 1
	}
	if err := c.api.ContainerStop(ctx, name, container.StopOptions{Timeout: &secs}); err != nil {
		return fmt.Errorf("dockercli: stop: %w", err)
	}
	return nil
}

// Start starts a container by name or ID.
func (c *Client) Start(ctx context.Context, name string) error {
	if err := c.api.ContainerStart(ctx, name, container.StartOptions{}); err != nil {
		return fmt.Errorf("dockercli: start: %w", err)
	}
	return nil
}

// Remove removes a container by name or ID.
func (c *Client) Remove(ctx context.Context, name string) error {
	if err := c.api.ContainerRemove(ctx, name, container.RemoveOptions{}); err != nil {
		return fmt.Errorf("dockercli: remove: %w", err)
	}
	return nil
}

// Pull pulls an image, draining the progress stream to completion so the image
// is guaranteed present when Pull returns.
func (c *Client) Pull(ctx context.Context, img string) error {
	rc, err := c.api.ImagePull(ctx, img, image.PullOptions{})
	if err != nil {
		return fmt.Errorf("dockercli: pull: %w", err)
	}
	defer func() { _ = rc.Close() }()
	// Drain to completion; the body is progress JSON we do not need to parse.
	if _, err := io.Copy(io.Discard, rc); err != nil {
		return fmt.Errorf("dockercli: pull drain: %w", err)
	}
	return nil
}

// CreateAndStart recreates a container from a captured inspect and starts it.
// Security-relevant fields (User, Cap*, Privileged, SecurityOpt,
// ReadonlyRootfs, NetworkMode, Devices) plus Binds/PortBindings/RestartPolicy/
// Env/Cmd/Image are preserved so the recreated container never gains privilege
// over the original.
func (c *Client) CreateAndStart(ctx context.Context, in ContainerInspect) error {
	cfg, hostCfg := buildCreateConfig(in)
	name := normalizeName(in.Name)
	resp, err := c.api.ContainerCreate(ctx, cfg, hostCfg, nil, nil, name)
	if err != nil {
		return fmt.Errorf("dockercli: create: %w", err)
	}
	if err := c.api.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		return fmt.Errorf("dockercli: create-start: %w", err)
	}
	return nil
}

// InspectName returns the live container's normalized name, or "" if no such
// container exists. Any other inspect error is propagated.
func (c *Client) InspectName(ctx context.Context, name string) (string, error) {
	resp, err := c.api.ContainerInspect(ctx, name)
	if err != nil {
		if isNoSuchContainer(err) {
			return "", nil
		}
		return "", fmt.Errorf("dockercli: inspect-name: %w", err)
	}
	return normalizeName(resp.Name), nil
}

// isNoSuchContainer reports whether err is the SDK's "no such container" error.
func isNoSuchContainer(err error) bool {
	if cerrdefs.IsNotFound(err) {
		return true
	}
	return strings.Contains(strings.ToLower(err.Error()), "no such container")
}

// ---- mapping helpers -------------------------------------------------------

// mapInspect converts the SDK inspect response into our captured subset.
func mapInspect(resp container.InspectResponse) ContainerInspect {
	out := ContainerInspect{}
	if resp.ContainerJSONBase != nil {
		out.ID = resp.ID
		out.Name = resp.Name
		out.Image = resp.Image
		if resp.HostConfig != nil {
			out.HostConfig = mapHostConfig(resp.HostConfig)
		}
	}
	if resp.Config != nil {
		out.Config = ContainerConfig{
			Image: resp.Config.Image,
			Env:   resp.Config.Env,
			Cmd:   []string(resp.Config.Cmd),
			User:  resp.Config.User,
		}
	}
	for _, m := range resp.Mounts {
		out.Mounts = append(out.Mounts, Mount{
			Type:        string(m.Type),
			Source:      m.Source,
			Destination: m.Destination,
		})
	}
	return out
}

// mapHostConfig maps the SDK HostConfig into our preserved subset.
func mapHostConfig(hc *container.HostConfig) HostConfig {
	out := HostConfig{
		Binds:          hc.Binds,
		CapAdd:         []string(hc.CapAdd),
		CapDrop:        []string(hc.CapDrop),
		Privileged:     hc.Privileged,
		SecurityOpt:    hc.SecurityOpt,
		ReadonlyRootfs: hc.ReadonlyRootfs,
		NetworkMode:    string(hc.NetworkMode),
		RestartPolicy: RestartPolicy{
			Name:              string(hc.RestartPolicy.Name),
			MaximumRetryCount: hc.RestartPolicy.MaximumRetryCount,
		},
	}
	if len(hc.PortBindings) > 0 {
		out.PortBindings = make(map[string][]PortBinding, len(hc.PortBindings))
		for port, bindings := range hc.PortBindings {
			pb := make([]PortBinding, 0, len(bindings))
			for _, b := range bindings {
				pb = append(pb, PortBinding{HostIP: b.HostIP, HostPort: b.HostPort})
			}
			out.PortBindings[string(port)] = pb
		}
	}
	for _, d := range hc.Devices {
		out.Devices = append(out.Devices, DeviceMapping{
			PathOnHost:        d.PathOnHost,
			PathInContainer:   d.PathInContainer,
			CgroupPermissions: d.CgroupPermissions,
		})
	}
	return out
}

// buildCreateConfig builds the SDK create config/hostconfig from our captured
// inspect, preserving the security-relevant fields (SEC parity).
func buildCreateConfig(in ContainerInspect) (*container.Config, *container.HostConfig) {
	cfg := &container.Config{
		Image: in.Config.Image,
		Env:   in.Config.Env,
		Cmd:   in.Config.Cmd,
		User:  in.Config.User,
	}

	hc := in.HostConfig
	hostCfg := &container.HostConfig{
		Binds:          hc.Binds,
		Privileged:     hc.Privileged,
		ReadonlyRootfs: hc.ReadonlyRootfs,
		SecurityOpt:    hc.SecurityOpt,
		CapAdd:         hc.CapAdd,
		CapDrop:        hc.CapDrop,
		NetworkMode:    container.NetworkMode(hc.NetworkMode),
		RestartPolicy: container.RestartPolicy{
			Name:              container.RestartPolicyMode(hc.RestartPolicy.Name),
			MaximumRetryCount: hc.RestartPolicy.MaximumRetryCount,
		},
	}
	if len(hc.PortBindings) > 0 {
		hostCfg.PortBindings = make(nat.PortMap, len(hc.PortBindings))
		for port, bindings := range hc.PortBindings {
			nb := make([]nat.PortBinding, 0, len(bindings))
			for _, b := range bindings {
				nb = append(nb, nat.PortBinding{HostIP: b.HostIP, HostPort: b.HostPort})
			}
			hostCfg.PortBindings[nat.Port(port)] = nb
		}
	}
	for _, d := range hc.Devices {
		hostCfg.Devices = append(hostCfg.Devices, container.DeviceMapping{
			PathOnHost:        d.PathOnHost,
			PathInContainer:   d.PathInContainer,
			CgroupPermissions: d.CgroupPermissions,
		})
	}
	return cfg, hostCfg
}
