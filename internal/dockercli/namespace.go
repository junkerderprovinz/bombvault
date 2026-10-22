package dockercli

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/docker/docker/api/types/container"
)

// rejoinStopTimeout is the grace a container sharing a namespace gets when it
// is recreated, the same a backup gives.
const rejoinStopTimeout = 30 * time.Second

// ContainerID returns the ID of the container named name, or "" when no
// container has exactly that name. Docker would also match an ID prefix.
func (c *Client) ContainerID(ctx context.Context, name string) (string, error) {
	resp, err := c.api.ContainerInspect(ctx, name)
	if err != nil {
		if isNoSuchContainer(err) {
			return "", nil
		}
		return "", fmt.Errorf("dockercli: inspect %q: %w", name, err)
	}
	if resp.ContainerJSONBase == nil || normalizeName(resp.Name) != normalizeName(name) {
		return "", nil
	}
	return resp.ID, nil
}

// RejoinNamespace recreates the containers that share the network, PID or IPC
// namespace of the container oldID, which parent has replaced. Docker binds
// such a container to the other one's ID, so once that is removed it can
// never start again; recreated, it joins parent. Each keeps its run state,
// and one that ran starts only when parent runs. It returns the containers
// it could not bring over, with an error that says why.
func (c *Client) RejoinNamespace(ctx context.Context, oldID, parent string) ([]string, error) {
	if oldID == "" {
		return nil, nil
	}
	summaries, err := c.api.ContainerList(ctx, container.ListOptions{All: true})
	if err != nil {
		return nil, fmt.Errorf("dockercli: list containers: %w", err)
	}
	parentRunning := false
	if p, err := c.api.ContainerInspect(ctx, parent); err == nil && p.State != nil {
		parentRunning = p.State.Running
	}
	old := "container:" + oldID
	var failed []string
	var errs []error
	for _, s := range summaries {
		resp, err := c.api.ContainerInspect(ctx, s.ID)
		if err != nil || resp.ContainerJSONBase == nil || resp.HostConfig == nil {
			continue
		}
		hc := resp.HostConfig
		if string(hc.NetworkMode) != old && string(hc.PidMode) != old && string(hc.IpcMode) != old {
			continue
		}
		name := normalizeName(resp.Name)
		if err := c.rejoin(ctx, resp.ID, name, old, "container:"+parent, parentRunning); err != nil {
			failed = append(failed, name)
			errs = append(errs, fmt.Errorf("%s: %w", name, err))
			continue
		}
		log.Printf("dockercli: recreated %q, which shares a namespace with %q", name, parent)
	}
	return failed, errors.Join(errs...)
}

// rejoin recreates one container with its namespaces moved from old to joined.
// The old container is renamed aside rather than removed until the new one
// exists, so a refused create leaves it where it was.
func (c *Client) rejoin(ctx context.Context, id, name, old, joined string, parentRunning bool) error {
	in, err := c.Inspect(ctx, id)
	if err != nil {
		return err
	}
	for _, mode := range []*string{&in.HostConfig.NetworkMode, &in.HostConfig.PidMode, &in.HostConfig.IpcMode} {
		if *mode == old {
			*mode = joined
		}
	}
	if in.Running {
		if err := c.Stop(ctx, id, rejoinStopTimeout); err != nil {
			return err
		}
	}
	aside := name + "-" + shortID(id)
	if err := c.api.ContainerRename(ctx, id, aside); err != nil {
		return fmt.Errorf("dockercli: rename aside: %w", err)
	}
	if err := c.CreateAndStart(ctx, in, false); err != nil {
		if rErr := c.api.ContainerRename(ctx, id, name); rErr != nil {
			return errors.Join(err, fmt.Errorf("dockercli: it stays as %s: %w", aside, rErr))
		}
		return err
	}
	if err := c.Remove(ctx, id); err != nil {
		return err
	}
	if in.Running && parentRunning {
		return c.Start(ctx, name)
	}
	return nil
}

func shortID(id string) string {
	if len(id) > 12 {
		return id[:12]
	}
	return id
}
