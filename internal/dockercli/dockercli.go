// Package dockercli wraps the official Docker SDK behind the Docker interface
// (consumed by the backup orchestrator) so host control is mockable in tests.
// The orchestrator depends only on the interface in types.go; the concrete
// Client here is wired exclusively in cmd/bombvault.
package dockercli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"maps"
	"net"
	"net/netip"
	"os"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"time"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/moby/moby/api/pkg/authconfig"
	"github.com/moby/moby/api/pkg/stdcopy"
	"github.com/moby/moby/api/types/blkiodev"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/mount"
	"github.com/moby/moby/api/types/network"
	"github.com/moby/moby/api/types/registry"
	"github.com/moby/moby/client"
	"github.com/moby/moby/client/pkg/versions"

	"github.com/junkerderprovinz/bombvault/internal/model"
	"github.com/junkerderprovinz/bombvault/internal/traffic"
)

// Client is the real Docker adapter over the official SDK, talking to the
// mounted docker.sock.
type Client struct {
	api *client.Client
}

// compile-time interface check.
var _ Docker = (*Client)(nil)

// New constructs a Client connected to the Docker daemon. The SDK negotiates
// the API version on the first request, so it works across Unraid Docker
// versions; DOCKER_API_VERSION pins one instead.
//
// DOCKER_HOST is honored when set (rootless Docker, Podman); otherwise it
// defaults to the standard /var/run/docker.sock, matching every prior
// BombVault release.
func New() (*Client, error) {
	api, err := client.New(dockerClientOpts()...)
	if err != nil {
		return nil, fmt.Errorf("dockercli: new client: %w", err)
	}
	return &Client{api: api}, nil
}

// dockerClientOpts builds the SDK options New connects with. The socket is
// added only when DOCKER_HOST is unset, since a WithHost after FromEnv would
// override the host rootless Docker and Podman are reached through.
func dockerClientOpts() []client.Opt {
	opts := []client.Opt{client.FromEnv}
	if os.Getenv(client.EnvOverrideHost) == "" {
		opts = append(opts, client.WithHost("unix:///var/run/docker.sock"))
	}
	return opts
}

// Close releases the underlying SDK client.
func (c *Client) Close() error { return c.api.Close() }

// List returns all containers (running and stopped).
func (c *Client) List(ctx context.Context) ([]ContainerInfo, error) {
	list, err := c.api.ContainerList(ctx, client.ContainerListOptions{All: true})
	if err != nil {
		return nil, fmt.Errorf("dockercli: list: %w", err)
	}
	out := make([]ContainerInfo, 0, len(list.Items))
	for _, s := range list.Items {
		out = append(out, mapContainerSummary(s))
	}
	return out, nil
}

// Allocations reports each container's current IPv4 and published host ports,
// used by the restore pre-flight conflict check. It reads the list summary only
// (no per-container inspect): a running container reports its assigned IP and
// active published ports there; a stopped container reports neither, so it
// never produces a false conflict (it holds no live resource).
func (c *Client) Allocations(ctx context.Context) ([]model.Allocation, error) {
	list, err := c.api.ContainerList(ctx, client.ContainerListOptions{All: true})
	if err != nil {
		return nil, fmt.Errorf("dockercli: allocations: %w", err)
	}
	out := make([]model.Allocation, 0, len(list.Items))
	for _, s := range list.Items {
		name := ""
		if len(s.Names) > 0 {
			name = normalizeName(s.Names[0])
		}
		ip := firstIP(s.NetworkSettings)
		var ports []string
		seen := map[string]bool{}
		for _, p := range s.Ports {
			if p.PublicPort == 0 {
				continue // not published to the host
			}
			proto := p.Type
			if proto == "" {
				proto = "tcp"
			}
			key := fmt.Sprintf("%d/%s", p.PublicPort, proto)
			if !seen[key] {
				seen[key] = true
				ports = append(ports, key)
			}
		}
		out = append(out, model.Allocation{Name: name, IPv4: ip, HostPorts: ports})
	}
	return out, nil
}

// Networks returns the names of the networks on this host, sorted.
func (c *Client) Networks(ctx context.Context) ([]string, error) {
	list, err := c.api.NetworkList(ctx, client.NetworkListOptions{})
	if err != nil {
		return nil, fmt.Errorf("dockercli: list networks: %w", err)
	}
	names := make([]string, 0, len(list.Items))
	for _, n := range list.Items {
		names = append(names, n.Name)
	}
	slices.Sort(names)
	return names, nil
}

// NetworkDrivers names the driver of a network, which tells a Docker bridge,
// whose traffic stays on the host, from macvlan or ipvlan, which reach the LAN.
type NetworkDrivers interface {
	NetworkDriver(ctx context.Context, name string) (string, error)
}

// NetworkDriver returns the driver of the network name, "bridge" for example.
func (c *Client) NetworkDriver(ctx context.Context, name string) (string, error) {
	n, err := c.api.NetworkInspect(ctx, name, client.NetworkInspectOptions{})
	if err != nil {
		return "", fmt.Errorf("dockercli: inspect network: %w", err)
	}
	return n.Network.Driver, nil
}

// inspect is ContainerInspect with the answer alone.
func (c *Client) inspect(ctx context.Context, ref string) (container.InspectResponse, error) {
	res, err := c.api.ContainerInspect(ctx, ref, client.ContainerInspectOptions{})
	return res.Container, err
}

// Inspect returns the captured inspect subset for a container by name or ID.
func (c *Client) Inspect(ctx context.Context, name string) (model.Inspect, error) {
	resp, err := c.inspect(ctx, name)
	if err != nil {
		return model.Inspect{}, fmt.Errorf("dockercli: inspect: %w", err)
	}
	if !answersFor(name, resp) {
		return model.Inspect{}, fmt.Errorf("dockercli: inspect: %w", noSuchContainer{name})
	}
	c.fillVolumeMountSources(ctx, name, resp.Mounts)
	out := mapInspect(resp)
	// Without the image the values stay as they are, which recreates the
	// container as it ran.
	if img, err := c.api.ImageInspect(ctx, out.Image); err == nil && img.Config != nil {
		withoutImageDefaults(&out.Config, imageDefaults{
			Entrypoint: img.Config.Entrypoint, Cmd: img.Config.Cmd,
			WorkingDir: img.Config.WorkingDir, StopSignal: img.Config.StopSignal,
		})
	}
	out.HostConfig.NetworkMode = c.namespaceByName(ctx, out.HostConfig.NetworkMode)
	out.HostConfig.PidMode = c.namespaceByName(ctx, out.HostConfig.PidMode)
	out.HostConfig.IpcMode = c.namespaceByName(ctx, out.HostConfig.IpcMode)
	return out, nil
}

// FromNewImage returns the definition a container is recreated from when its
// tag has moved to a newer image: only what the user set carries over, and what
// came from the image it runs now follows the new one. Without that image the
// definition stays as it is, which recreates the container as it ran.
func (c *Client) FromNewImage(ctx context.Context, in model.Inspect) model.Inspect {
	img, err := c.api.ImageInspect(ctx, in.Image)
	if err != nil || img.Config == nil {
		return in
	}
	own := imageDefaults{Env: img.Config.Env, Labels: img.Config.Labels, Cmd: img.Config.Cmd, User: img.Config.User}
	if hc := img.Config.Healthcheck; hc != nil && len(hc.Test) > 0 {
		own.Healthcheck = &model.Healthcheck{
			Test: hc.Test, Interval: hc.Interval, Timeout: hc.Timeout, StartPeriod: hc.StartPeriod, Retries: hc.Retries,
		}
	}
	withoutInherited(&in.Config, own)
	return in
}

// namespaceByName turns "container:<id>" into "container:<name>". Docker
// stores the other container's ID, which a recreate of that container
// replaces, while its name stays. A mode whose container is gone keeps the
// ID.
func (c *Client) namespaceByName(ctx context.Context, mode string) string {
	ref, ok := strings.CutPrefix(mode, "container:")
	if !ok || ref == "" {
		return mode
	}
	resp, err := c.inspect(ctx, ref)
	if err != nil || resp.Name == "" {
		return mode
	}
	return "container:" + normalizeName(resp.Name)
}

// needsVolumeMountpoint reports whether a Mounts entry is a named-volume mount
// the daemon reported without a host-side Source. Normally the daemon already
// populates Source for a volume mount with its real storage location (e.g.
// /var/lib/docker/volumes/<name>/_data), so this is the rare fallback case —
// but resolveAppdataPaths (internal/api/service.go) needs a resolved Source to
// back up a named volume at all, so it is worth the one extra VolumeInspect
// call when the daemon left it empty.
func needsVolumeMountpoint(m container.MountPoint) bool {
	return m.Type == mount.TypeVolume && m.Source == "" && m.Name != ""
}

// fillVolumeMountSources resolves the host-side Source for any volume mount
// needsVolumeMountpoint flags, via VolumeInspect, mutating mounts in place.
// Best-effort, matching the same pattern as CreateAndStart's secondary-network
// reconnect: a volume lookup failure just leaves Source empty rather than
// failing the whole container inspect over one volume, but it is still logged
// (not silently dropped) — mapInspect then carries the empty Source through
// like any other unresolvable mount, and resolveAppdataPaths skips it.
func (c *Client) fillVolumeMountSources(ctx context.Context, name string, mounts []container.MountPoint) {
	for i := range mounts {
		if !needsVolumeMountpoint(mounts[i]) {
			continue
		}
		mp, err := c.VolumeInspect(ctx, mounts[i].Name)
		if err != nil {
			log.Printf("dockercli: inspect %q: resolve volume %q mountpoint failed (continuing): %v", name, mounts[i].Name, err)
			continue
		}
		mounts[i].Source = mp
	}
}

// VolumeInspect returns a named volume's real host-side storage location (its
// Mountpoint, e.g. /var/lib/docker/volumes/<name>/_data), or "" if the volume
// does not exist. Mirrors ImageID's not-found-returns-empty-string style so a
// caller can treat "gone" and "never existed" the same way without special-
// casing an error type.
func (c *Client) VolumeInspect(ctx context.Context, name string) (string, error) {
	res, err := c.api.VolumeInspect(ctx, name, client.VolumeInspectOptions{})
	if err != nil {
		if cerrdefs.IsNotFound(err) {
			return "", nil
		}
		return "", fmt.Errorf("dockercli: volume inspect: %w", err)
	}
	return res.Volume.Mountpoint, nil
}

// Stop stops a container, sending SIGKILL after timeout if it has not exited.
// A container that set a longer stop timeout of its own gets that one, since
// a database or game server given one needs it to shut down cleanly. The
// Docker API expresses the grace period in whole seconds; a positive
// sub-second timeout is rounded up to 1s so it never collapses to an immediate
// SIGKILL.
func (c *Client) Stop(ctx context.Context, name string, timeout time.Duration) error {
	secs := int(timeout.Seconds())
	if secs == 0 && timeout > 0 {
		secs = 1
	}
	if resp, err := c.inspect(ctx, name); err == nil {
		if !answersFor(name, resp) {
			return fmt.Errorf("dockercli: stop: %w", noSuchContainer{name})
		}
		if resp.Config != nil {
			secs = stopGrace(secs, resp.Config.StopTimeout)
		}
	}
	if _, err := c.api.ContainerStop(ctx, name, client.ContainerStopOptions{Timeout: &secs}); err != nil {
		return fmt.Errorf("dockercli: stop: %w", err)
	}
	return nil
}

// stopGrace is the grace period in seconds a stop gives: the caller's, or the
// container's own when that is longer. An own timeout without limit is not
// taken over, so a backup cannot wait forever on a container that never
// exits.
func stopGrace(secs int, own *int) int {
	if own != nil && *own > secs {
		return *own
	}
	return secs
}

// Start starts a container by name or ID.
func (c *Client) Start(ctx context.Context, name string) error {
	if err := c.refuseIDPrefixMatch(ctx, name); err != nil {
		return fmt.Errorf("dockercli: start: %w", err)
	}
	if _, err := c.api.ContainerStart(ctx, name, client.ContainerStartOptions{}); err != nil {
		return fmt.Errorf("dockercli: start: %w", err)
	}
	return nil
}

// Restart asks the daemon to restart the named container (stop then start). Used
// for BombVault's own restart-to-apply after a config restore: the daemon does
// both halves even though THIS process is killed during the stop, so it is the
// robust way to relaunch ourselves (no dependency on a --restart policy). The
// grace period is expressed in whole seconds, mirroring Stop.
func (c *Client) Restart(ctx context.Context, name string, timeout time.Duration) error {
	secs := int(timeout.Seconds())
	if err := c.refuseIDPrefixMatch(ctx, name); err != nil {
		return fmt.Errorf("dockercli: restart %q: %w", name, err)
	}
	if _, err := c.api.ContainerRestart(ctx, name, client.ContainerRestartOptions{Timeout: &secs}); err != nil {
		return fmt.Errorf("dockercli: restart %q: %w", name, err)
	}
	return nil
}

// WaitRunning polls until the named container reports Running, or until timeout.
// ContainerStart returns as soon as the daemon accepts the request, before the
// container is actually up; a dependent that shares this container's network
// namespace (network_mode: container:<name>) can't start until it is Running.
// Used after restarting a backed-up target so its netns dependents come back.
func (c *Client) WaitRunning(ctx context.Context, name string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	const poll = 200 * time.Millisecond
	for {
		resp, err := c.inspect(ctx, name)
		if err == nil && resp.State != nil && resp.State.Running {
			return nil
		}
		if time.Now().After(deadline) {
			if err != nil {
				return fmt.Errorf("dockercli: wait running %q: %w", name, err)
			}
			return fmt.Errorf("dockercli: wait running %q: not running after %s", name, timeout)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(poll):
		}
	}
}

// Health returns the readiness snapshot of a container (Running, whether it
// defines a Docker healthcheck, and — if it does — whether that healthcheck is
// currently "healthy"). Used by the health-gated ordered restart after a backup
// to wait for a dependency to be ready before starting the containers that
// depend on it. A container with no healthcheck reports HasHealthcheck=false, so
// the caller falls back to Running plus a short grace.
func (c *Client) Health(ctx context.Context, name string) (model.Health, error) {
	resp, err := c.inspect(ctx, name)
	if err != nil {
		return model.Health{}, fmt.Errorf("dockercli: health: %w", err)
	}
	var h model.Health
	if resp.State != nil {
		h.Running = resp.State.Running
		// State.Health is nil when no healthcheck is configured; some daemons
		// instead report it present with Status "none". Treat both as "no
		// healthcheck" so the caller uses the running-plus-grace fallback.
		if resp.State.Health != nil && resp.State.Health.Status != container.NoHealthcheck {
			h.HasHealthcheck = true
			h.Healthy = resp.State.Health.Status == container.Healthy
		}
	}
	return h, nil
}

// ExitError is a command run in a container that ended with a non-zero exit
// code. Reason is the tail of what it wrote, stderr first.
type ExitError struct {
	Code   int
	Reason string
}

func (e *ExitError) Error() string {
	if e.Reason == "" {
		return fmt.Sprintf("hook exited %d", e.Code)
	}
	return fmt.Sprintf("hook exited %d: %s", e.Code, e.Reason)
}

// ExitCode is the code the command ended with.
func (e *ExitError) ExitCode() int { return e.Code }

// Output is what the command wrote, "" when it wrote nothing.
func (e *ExitError) Output() string { return e.Reason }

// Exec runs cmd inside a running container and returns an *ExitError when it
// exits non-zero (used for pre/post-backup hooks). Output is captured only to
// surface a short failure reason; it is demuxed via stdcopy and drained so the
// exec completes before we read the exit code.
func (c *Client) Exec(ctx context.Context, name string, cmd []string) error {
	created, err := c.api.ExecCreate(ctx, name, client.ExecCreateOptions{
		Cmd:          cmd,
		AttachStdout: true,
		AttachStderr: true,
	})
	if err != nil {
		return fmt.Errorf("dockercli: exec create: %w", err)
	}
	att, err := c.api.ExecAttach(ctx, created.ID, client.ExecAttachOptions{})
	if err != nil {
		return fmt.Errorf("dockercli: exec attach: %w", err)
	}
	defer att.Close()
	outBuf, errBuf, err := drainExecOutput(ctx, att.Reader, att.Close)
	if err != nil {
		return fmt.Errorf("dockercli: exec: %w", err)
	}

	exitCode, err := c.execExit(ctx, created.ID)
	if err != nil {
		return err
	}
	if exitCode != 0 {
		reason := strings.TrimSpace(errBuf.String())
		if reason == "" {
			reason = strings.TrimSpace(outBuf.String())
		}
		if len(reason) > 200 {
			reason = reason[len(reason)-200:]
		}
		return &ExitError{Code: exitCode, Reason: reason}
	}
	return nil
}

// How long Exec keeps asking whether a finished stream also means a finished
// process. The daemon records the exit code from its own event handler, apart
// from closing the output, so the two are milliseconds apart in practice; the
// wait only has to outlast that gap, not the hook.
var (
	execExitPoll = 50 * time.Millisecond
	execExitWait = 10 * time.Second
)

// drainExecOutput reads an exec attach to the end and keeps a capped copy of
// stdout and stderr for the failure reason. The cap stops a hook that floods its
// output from ballooning memory; everything past it is read and discarded so the
// process can still finish.
//
// The attach is a hijacked raw connection whose Read never looks at the context,
// so a hook that hangs inside the container used to hold the backup until the
// hook itself gave up: the cancel button, BACKUP_MAX_HOURS and shutdown all end
// in a cancelled context that never reached this read. Closing the attach when
// the context ends is what unblocks it. The process inside the container keeps
// running (the daemon starts it detached); the container stop that follows a
// pre-hook, or the container's own lifetime, ends it.
func drainExecOutput(ctx context.Context, r io.Reader, closeAttach func()) (stdout, stderr bytes.Buffer, err error) {
	stop := context.AfterFunc(ctx, closeAttach)
	defer stop()

	_, _ = stdcopy.StdCopy(&stdout, &stderr, io.LimitReader(r, 64<<10))
	_, _ = io.Copy(io.Discard, r)

	if ctxErr := ctx.Err(); ctxErr != nil {
		return stdout, stderr, ctxErr
	}
	return stdout, stderr, nil
}

// waitExecExit asks inspect until the process reports it is no longer running
// and returns its exit code. Reading the code once, straight after the output
// ended, could see the zero of a process that had not finished yet, and a
// failing hook would then count as a success.
func waitExecExit(ctx context.Context, inspect func(context.Context) (running bool, exitCode int, err error), poll, maxWait time.Duration) (int, error) {
	deadline := time.Now().Add(maxWait)
	for {
		if err := ctx.Err(); err != nil {
			return 0, err
		}
		running, code, err := inspect(ctx)
		if err != nil {
			return 0, err
		}
		if !running {
			return code, nil
		}
		if time.Now().After(deadline) {
			return 0, fmt.Errorf("process still reported running %s after its output ended", maxWait)
		}
		select {
		case <-ctx.Done():
			return 0, ctx.Err()
		case <-time.After(poll):
		}
	}
}

// Stats reads a container's cumulative CPU and network counters once, without
// the second sample docker stats waits a second for. A stopped container comes
// back with Running false.
func (c *Client) Stats(ctx context.Context, name string) (traffic.Sample, error) {
	resp, err := c.api.ContainerStats(ctx, name, client.ContainerStatsOptions{})
	if err != nil {
		return traffic.Sample{}, fmt.Errorf("dockercli: stats %s: %w", name, err)
	}
	defer func() { _ = resp.Body.Close() }()
	var st container.StatsResponse
	if err := json.NewDecoder(resp.Body).Decode(&st); err != nil {
		return traffic.Sample{}, fmt.Errorf("dockercli: stats %s: decode: %w", name, err)
	}
	return statsSample(st, time.Now()), nil
}

func statsSample(st container.StatsResponse, at time.Time) traffic.Sample {
	s := traffic.Sample{
		At:       at,
		Running:  !st.Read.IsZero() && st.PidsStats.Current > 0,
		CPUNanos: st.CPUStats.CPUUsage.TotalUsage,
		HasNet:   len(st.Networks) > 0,
	}
	for _, n := range st.Networks {
		s.RxBytes += n.RxBytes
		s.TxBytes += n.TxBytes
	}
	return s
}

// Remove removes a container by name or ID.
func (c *Client) Remove(ctx context.Context, name string) error {
	if err := c.refuseIDPrefixMatch(ctx, name); err != nil {
		return nil // already gone, like the answer below
	}
	if _, err := c.api.ContainerRemove(ctx, name, client.ContainerRemoveOptions{}); err != nil {
		if isNoSuchContainer(err) {
			return nil // already gone — a restore/recreate can proceed
		}
		return fmt.Errorf("dockercli: remove: %w", err)
	}
	return nil
}

// Signal sends signal, such as "SIGHUP", to the main process of a running
// container.
func (c *Client) Signal(ctx context.Context, name, signal string) error {
	if err := c.refuseIDPrefixMatch(ctx, name); err != nil {
		return fmt.Errorf("dockercli: signal: %w", err)
	}
	if _, err := c.api.ContainerKill(ctx, name, client.ContainerKillOptions{Signal: signal}); err != nil {
		return fmt.Errorf("dockercli: signal: %w", err)
	}
	return nil
}

// Pull pulls an image anonymously, draining the progress stream to completion
// so the image is guaranteed present when Pull returns.
func (c *Client) Pull(ctx context.Context, img string) error {
	return c.PullWithAuth(ctx, img, "")
}

// PullWithAuth is Pull with an optional registry credential (#106): registryAuth
// is the base64url JSON blob the Docker Engine API expects (EncodeRegistryAuth),
// "" pulls anonymously exactly like Pull.
func (c *Client) PullWithAuth(ctx context.Context, img, registryAuth string) error {
	rc, err := c.api.ImagePull(ctx, img, client.ImagePullOptions{RegistryAuth: registryAuth})
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

// EncodeRegistryAuth renders a registry credential into the base64url JSON blob
// the Docker Engine API expects in client.ImagePullOptions.RegistryAuth (the
// X-Registry-Auth header). serverAddress is the registry host the credential is
// for (e.g. "ghcr.io"). Kept here so the wire format stays a dockercli concern.
func EncodeRegistryAuth(username, password, serverAddress string) (string, error) {
	enc, err := authconfig.Encode(registry.AuthConfig{
		Username:      username,
		Password:      password,
		ServerAddress: serverAddress,
	})
	if err != nil {
		return "", fmt.Errorf("dockercli: encode registry auth: %w", err)
	}
	return enc, nil
}

// ImageID returns the local image ID (sha256:…) resolved for a registry
// reference, or "" when the reference is not present locally. Used after a Pull
// to compare against a running container's image and skip a needless recreate
// when the image did not actually change (#52).
func (c *Client) ImageID(ctx context.Context, ref string) (string, error) {
	insp, err := c.api.ImageInspect(ctx, ref)
	if err != nil {
		if cerrdefs.IsNotFound(err) {
			return "", nil
		}
		return "", fmt.Errorf("dockercli: image inspect: %w", err)
	}
	return insp.ID, nil
}

// ImageTag points the reference ref at the local image id, as docker tag does.
func (c *Client) ImageTag(ctx context.Context, id, ref string) error {
	if _, err := c.api.ImageTag(ctx, client.ImageTagOptions{Source: id, Target: ref}); err != nil {
		return fmt.Errorf("dockercli: image tag: %w", err)
	}
	return nil
}

// ImageRemove removes a local image by ID (sha256:…). force=false, so the daemon
// refuses if any container still references it — a shared base image is never
// deleted. Used to prune the superseded image after an update-after-backup (#56).
// A not-found image counts as already-gone (nil).
func (c *Client) ImageRemove(ctx context.Context, id string) error {
	if id == "" {
		return nil
	}
	if _, err := c.api.ImageRemove(ctx, id, client.ImageRemoveOptions{Force: false, PruneChildren: false}); err != nil {
		if cerrdefs.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("dockercli: image remove: %w", err)
	}
	return nil
}

// CreateAndStart recreates a container from a captured inspect and starts it
// when start is true. Security-relevant fields (User, Cap*, Privileged,
// SecurityOpt, ReadonlyRootfs, NetworkMode, Devices) plus Binds/PortBindings/
// RestartPolicy/Env/Cmd/Image, the limits, the log driver, DNS and the GPU
// request are preserved so the recreated container never gains privilege or
// headroom over the original. A host that lacks the container's runtime or
// GPU driver answers with a model.MissingRuntimeError.
func (c *Client) CreateAndStart(ctx context.Context, in model.Inspect, start bool) error {
	cfg, hostCfg, err := buildCreateConfig(in)
	if err != nil {
		return fmt.Errorf("dockercli: create: %w", err)
	}
	netCfg, err := buildNetworkingConfig(in)
	if err != nil {
		return fmt.Errorf("dockercli: create: %w", err)
	}
	name := normalizeName(in.Name)
	resp, err := c.createContainer(ctx, client.ContainerCreateOptions{Config: cfg, HostConfig: hostCfg, NetworkingConfig: netCfg, Name: name})
	if err != nil {
		return fmt.Errorf("dockercli: create: %w", missingRuntime(err))
	}
	// Reconnect any SECONDARY networks (the primary is already attached, via netCfg
	// or NetworkMode). Best-effort: a network that no longer exists must not block
	// the restore, so each failure is logged and skipped.
	for _, n := range in.Networks {
		if n.Name == "" || n.Name == in.Network.Name {
			continue
		}
		ep, cErr := endpointSettings(n)
		if cErr == nil {
			_, cErr = c.api.NetworkConnect(ctx, n.Name, client.NetworkConnectOptions{Container: resp.ID, EndpointConfig: ep})
		}
		if cErr != nil {
			log.Printf("dockercli: restore %q: reconnect network %q failed (continuing): %v", name, n.Name, cErr)
		}
	}
	// Start only when the caller asked for it (it decides from the captured
	// run-state and the "leave stopped" restore option) — restore recreates the
	// container in its captured state rather than always starting it.
	if !start {
		return nil
	}
	if _, err := c.api.ContainerStart(ctx, resp.ID, client.ContainerStartOptions{}); err != nil {
		return fmt.Errorf("dockercli: create-start: %w", missingRuntime(err))
	}
	return nil
}

// createContainer refuses what a daemon before API 1.44 would drop without a
// word: a MAC set per network, which a DHCP reservation is tied to, and a
// recursive read-only bind. Only such a create asks the daemon its version
// first.
func (c *Client) createContainer(ctx context.Context, opts client.ContainerCreateOptions) (client.ContainerCreateResult, error) {
	if feature := api144Feature(opts); feature != "" {
		if _, err := c.api.Ping(ctx, client.PingOptions{NegotiateAPIVersion: true}); err != nil {
			return client.ContainerCreateResult{}, err
		}
		if v := c.api.ClientVersion(); versions.LessThan(v, "1.44") {
			return client.ContainerCreateResult{}, fmt.Errorf("%q requires API version 1.44, but the Docker daemon API version is %s", feature, v)
		}
	}
	return c.api.ContainerCreate(ctx, opts)
}

// api144Feature names what opts asks of API 1.44, "" when it asks nothing.
func api144Feature(opts client.ContainerCreateOptions) string {
	if opts.NetworkingConfig != nil {
		for _, ep := range opts.NetworkingConfig.EndpointsConfig {
			if ep != nil && len(ep.MacAddress) > 0 {
				return "specify mac-address per network"
			}
		}
	}
	if opts.HostConfig != nil {
		for _, m := range opts.HostConfig.Mounts {
			if m.BindOptions != nil && m.BindOptions.ReadOnlyForceRecursive {
				return "bind-recursive=readonly"
			}
		}
	}
	return ""
}

// InspectName returns the live container's normalized name, or "" if no such
// container exists. Any other inspect error is propagated.
func (c *Client) InspectName(ctx context.Context, name string) (string, error) {
	resp, err := c.inspect(ctx, name)
	if err != nil {
		if isNoSuchContainer(err) {
			return "", nil
		}
		return "", fmt.Errorf("dockercli: inspect-name: %w", err)
	}
	if !answersFor(name, resp) {
		return "", nil
	}
	return normalizeName(resp.Name), nil
}

// shortIDLen is the shortest id reference BombVault passes: what docker ps shows.
const shortIDLen = 12

// answersFor reports whether Docker's answer is about the container ref names.
// Docker falls back to an id prefix for a name no container has, so a removed
// "db" would otherwise resolve to whichever container's id starts with db, and
// that container would be backed up, stopped or removed in its place. A real id
// reference is never as short as such a name.
func answersFor(ref string, resp container.InspectResponse) bool {
	return normalizeName(resp.Name) == normalizeName(ref) || (len(ref) >= shortIDLen && strings.HasPrefix(resp.ID, ref))
}

// noSuchContainer is what answersFor turns an id prefix match into, the error
// Docker gives for a name it does not know.
type noSuchContainer struct{ name string }

func (e noSuchContainer) Error() string { return "no such container: " + e.name }

// refuseIDPrefixMatch fails with noSuchContainer when name only reaches a
// container through its id. Any other answer is left to the call that follows.
func (c *Client) refuseIDPrefixMatch(ctx context.Context, name string) error {
	resp, err := c.inspect(ctx, name)
	if err == nil && !answersFor(name, resp) {
		return noSuchContainer{name}
	}
	return nil
}

// Self returns the name of the container this process runs in, or "" when it
// cannot be found.
func (c *Client) Self(ctx context.Context) (string, error) {
	ref := ownContainerRef()
	if ref == "" {
		return "", nil
	}
	resp, err := c.inspect(ctx, ref)
	if err != nil {
		if isNoSuchContainer(err) {
			return "", nil
		}
		return "", fmt.Errorf("dockercli: self-inspect: %w", err)
	}
	return normalizeName(resp.Name), nil
}

var (
	mountinfoPath = "/proc/self/mountinfo"
	containerID   = regexp.MustCompile(`containers/([0-9a-f]{64})/`)
)

// ownContainerRef reads the container ID from the file Docker mounts over
// /etc/hostname. The hostname itself is only a fallback: a platform or template
// may set it to something other than the short ID.
func ownContainerRef() string {
	data, _ := os.ReadFile(mountinfoPath)
	for line := range strings.Lines(string(data)) {
		fields := strings.Fields(line)
		if len(fields) < 5 || fields[4] != "/etc/hostname" {
			continue
		}
		if m := containerID.FindStringSubmatch(fields[3]); m != nil {
			return m[1]
		}
	}
	host, _ := os.Hostname()
	return host
}

// isNoSuchContainer reports whether err is the SDK's "no such container" error.
func isNoSuchContainer(err error) bool {
	if cerrdefs.IsNotFound(err) {
		return true
	}
	return strings.Contains(strings.ToLower(err.Error()), "no such container")
}

// IsNotFound reports whether err is the daemon's "no such container" error,
// unwrapping through the package's inspect wrappers (the containerd typed error
// survives the fmt.Errorf("%w") wraps). Exported so the API layer can treat a
// scheduled backup of a removed container as a skip rather than a failure, the
// same way virshcli.IsNotFound serves the VM path.
func IsNotFound(err error) bool { return err != nil && isNoSuchContainer(err) }

// mapContainerSummary converts a list summary into a ContainerInfo. Named
// volumes are left out of Mounts: rename matching compares host paths, and a
// volume's source is Docker's own storage rather than the container's appdata.
func mapContainerSummary(s container.Summary) ContainerInfo {
	name := ""
	if len(s.Names) > 0 {
		name = normalizeName(s.Names[0])
	}
	ip := firstIP(s.NetworkSettings)
	mounts := make([]MountPoint, 0, len(s.Mounts))
	for _, m := range s.Mounts {
		if m.Type == mount.TypeBind && m.Source != "" && m.Destination != "" {
			mounts = append(mounts, MountPoint{
				Source:      m.Source,
				Destination: m.Destination,
			})
		}
	}
	return ContainerInfo{
		ID:      s.ID,
		Name:    name,
		Image:   s.Image,
		State:   string(s.State),
		Status:  s.Status,
		IP:      ip,
		Stack:   s.Labels["com.docker.compose.project"],
		ImageID: s.ImageID,
		Created: s.Created,
		Labels:  s.Labels,
		Mounts:  mounts,

		NetworkMode: s.HostConfig.NetworkMode,
	}
}

// firstIP is the first address a list summary reports for the container, ""
// when it has none.
func firstIP(ns *container.NetworkSettingsSummary) string {
	for _, n := range ns.Networks {
		if n != nil && n.IPAddress.IsValid() {
			return n.IPAddress.String()
		}
	}
	return ""
}

// addrString renders addr the way the daemon writes it, "" when it is unset.
func addrString(addr netip.Addr) string {
	if !addr.IsValid() {
		return ""
	}
	return addr.String()
}

// parseAddr reads an address back, "" as unset.
func parseAddr(s string) (netip.Addr, error) {
	if s == "" {
		return netip.Addr{}, nil
	}
	return netip.ParseAddr(s)
}

// mapInspect converts the SDK inspect response into our captured subset.
func mapInspect(resp container.InspectResponse) model.Inspect {
	out := model.Inspect{ID: resp.ID, Name: resp.Name, Image: resp.Image}
	if resp.State != nil {
		out.Running = resp.State.Running
	}
	if resp.HostConfig != nil {
		out.HostConfig = mapHostConfig(resp.HostConfig)
	}
	if resp.Config != nil {
		out.Config = model.Config{
			Image:  resp.Config.Image,
			Env:    resp.Config.Env,
			Cmd:    []string(resp.Config.Cmd),
			User:   resp.Config.User,
			Labels: resp.Config.Labels, // SEC/Unraid: keep net.unraid.docker.* labels
		}
		if hc := resp.Config.Healthcheck; hc != nil && len(hc.Test) > 0 {
			out.Config.Healthcheck = &model.Healthcheck{
				Test: hc.Test, Interval: hc.Interval, Timeout: hc.Timeout, StartPeriod: hc.StartPeriod, Retries: hc.Retries,
			}
		}
		out.Config.Entrypoint = []string(resp.Config.Entrypoint)
		out.Config.WorkingDir = resp.Config.WorkingDir
		out.Config.StopSignal = resp.Config.StopSignal
		out.Config.StopTimeout = resp.Config.StopTimeout
		out.Config.Tty = resp.Config.Tty
		out.Config.OpenStdin = resp.Config.OpenStdin
		out.Config.Hostname, out.Config.Domainname = ownHostname(resp)
	}
	for _, m := range resp.Mounts {
		out.Mounts = append(out.Mounts, model.Mount{
			Type:        string(m.Type),
			Source:      m.Source,
			Destination: m.Destination,
			Name:        m.Name,
		})
	}
	out.Network = mapPrimaryNetwork(resp)
	out.Networks = mapAllNetworks(resp)
	return out
}

// ownHostname returns the hostname and domain name the container was given.
// Docker fills in the short ID, the host's name on host networking and the
// other container's on a shared network or UTS namespace, and it refuses a
// hostname next to the last two, so none of those is kept.
func ownHostname(resp container.InspectResponse) (string, string) {
	if resp.Config == nil {
		return "", ""
	}
	host := resp.Config.Hostname
	if hc := resp.HostConfig; hc != nil && (strings.HasPrefix(string(hc.NetworkMode), "container:") || hc.NetworkMode == "host" || hc.UTSMode == "host") {
		return "", ""
	}
	if len(resp.ID) >= 12 && host == resp.ID[:12] {
		host = ""
	}
	return host, resp.Config.Domainname
}

// imageDefaults is what an image gives a container that sets none of its own.
type imageDefaults struct {
	Entrypoint, Cmd        []string
	WorkingDir, StopSignal string
	Env                    []string
	Labels                 map[string]string
	User                   string
	Healthcheck            *model.Healthcheck
}

// withoutImageDefaults clears what the container took over from its image
// unchanged. A recreate from a newer image then runs that image's entrypoint,
// working directory and stop signal instead of the old one's. An entrypoint
// is kept when the container has no command while the image has one: setting
// the entrypoint is what cleared the command, and leaving it out would bring
// the image's command back.
func withoutImageDefaults(cfg *model.Config, img imageDefaults) {
	if slices.Equal(cfg.Entrypoint, img.Entrypoint) && (len(cfg.Cmd) > 0 || len(img.Cmd) == 0) {
		cfg.Entrypoint = nil
	}
	if cfg.WorkingDir == img.WorkingDir {
		cfg.WorkingDir = ""
	}
	if cfg.StopSignal == img.StopSignal {
		cfg.StopSignal = ""
	}
}

// withoutInherited clears what the container took over from the image it runs,
// which withoutImageDefaults keeps because the rest of BombVault reads it, such
// as a database's variables. Docker copies the image's variables and labels
// into the container, so one that still reads as in the image counts as the
// image's. The command goes only when no entrypoint stays: next to one, Docker
// would not fill it back in.
func withoutInherited(cfg *model.Config, img imageDefaults) {
	cfg.Env = slices.DeleteFunc(slices.Clone(cfg.Env), func(e string) bool { return slices.Contains(img.Env, e) })
	labels := maps.Clone(cfg.Labels)
	maps.DeleteFunc(labels, func(k, v string) bool {
		own, ok := img.Labels[k]
		return ok && own == v
	})
	cfg.Labels = labels
	if len(cfg.Entrypoint) == 0 && slices.Equal(cfg.Cmd, img.Cmd) {
		cfg.Cmd = nil
	}
	if cfg.User == img.User {
		cfg.User = ""
	}
	if cfg.Healthcheck != nil && reflect.DeepEqual(cfg.Healthcheck, img.Healthcheck) {
		cfg.Healthcheck = nil
	}
}

// mapAllNetworks captures every network the container is attached to, so a
// multi-network container can be fully reconnected on restore.
func mapAllNetworks(resp container.InspectResponse) []model.NetworkEndpoint {
	if resp.NetworkSettings == nil {
		return nil
	}
	out := make([]model.NetworkEndpoint, 0, len(resp.NetworkSettings.Networks))
	for name, e := range resp.NetworkSettings.Networks {
		if e == nil {
			continue
		}
		out = append(out, endpointOf(name, e))
	}
	return out
}

// mapPrimaryNetwork extracts the primary network attachment (the one matching
// NetworkMode, else the first) so a recreated container keeps its original
// static IP / MAC.
func mapPrimaryNetwork(resp container.InspectResponse) model.NetworkEndpoint {
	if resp.NetworkSettings == nil || len(resp.NetworkSettings.Networks) == 0 {
		return model.NetworkEndpoint{}
	}
	netMode := ""
	if resp.HostConfig != nil {
		netMode = string(resp.HostConfig.NetworkMode)
	}
	name, ep := "", (*network.EndpointSettings)(nil)
	if e, ok := resp.NetworkSettings.Networks[netMode]; ok && e != nil {
		name, ep = netMode, e
	} else {
		for n, e := range resp.NetworkSettings.Networks {
			if e != nil {
				name, ep = n, e
				break
			}
		}
	}
	if ep == nil {
		return model.NetworkEndpoint{}
	}
	return endpointOf(name, ep)
}

// endpointOf captures what a recreate needs of an attachment: the requested
// static IPv4, the MAC and the aliases.
func endpointOf(name string, e *network.EndpointSettings) model.NetworkEndpoint {
	ep := model.NetworkEndpoint{Name: name, MACAddress: e.MacAddress.String(), Aliases: e.Aliases}
	if e.IPAMConfig != nil {
		ep.IPv4Address = addrString(e.IPAMConfig.IPv4Address)
	}
	return ep
}

// mapHostConfig maps the SDK HostConfig into our preserved subset.
func mapHostConfig(hc *container.HostConfig) model.HostConfig {
	out := model.HostConfig{
		Binds:          hc.Binds,
		VolumesFrom:    hc.VolumesFrom,
		CapAdd:         []string(hc.CapAdd),
		CapDrop:        []string(hc.CapDrop),
		Privileged:     hc.Privileged,
		SecurityOpt:    hc.SecurityOpt,
		ReadonlyRootfs: hc.ReadonlyRootfs,
		NetworkMode:    string(hc.NetworkMode),
		PidMode:        string(hc.PidMode),
		IpcMode:        string(hc.IpcMode),
		UsernsMode:     string(hc.UsernsMode),
		GroupAdd:       hc.GroupAdd,
		Sysctls:        hc.Sysctls,
		Tmpfs:          hc.Tmpfs,
		ExtraHosts:     hc.ExtraHosts,
		CgroupParent:   hc.CgroupParent,
		RestartPolicy: model.RestartPolicy{
			Name:              string(hc.RestartPolicy.Name),
			MaximumRetryCount: hc.RestartPolicy.MaximumRetryCount,
		},
		Memory:            hc.Memory,
		MemoryReservation: hc.MemoryReservation,
		MemorySwap:        hc.MemorySwap,
		MemorySwappiness:  hc.MemorySwappiness,
		NanoCPUs:          hc.NanoCPUs,
		CPUShares:         hc.CPUShares,
		CPUPeriod:         hc.CPUPeriod,
		CPUQuota:          hc.CPUQuota,
		CpusetCpus:        hc.CpusetCpus,
		CpusetMems:        hc.CpusetMems,
		PidsLimit:         hc.PidsLimit,
		OomKillDisable:    hc.OomKillDisable,
		OomScoreAdj:       hc.OomScoreAdj,
		ShmSize:           hc.ShmSize,
		Init:              hc.Init,
		DNSSearch:         hc.DNSSearch,
		DNSOptions:        hc.DNSOptions,
		UTSMode:           string(hc.UTSMode),
		CgroupnsMode:      string(hc.CgroupnsMode),
		Links:             hc.Links,
		StorageOpt:        hc.StorageOpt,
		BlkioWeight:       hc.BlkioWeight,
		Runtime:           hc.Runtime,
	}
	for _, a := range hc.DNS {
		out.DNS = append(out.DNS, addrString(a))
	}
	for _, r := range hc.DeviceRequests {
		out.DeviceRequests = append(out.DeviceRequests, model.DeviceRequest{
			Driver: r.Driver, Count: r.Count, DeviceIDs: r.DeviceIDs, Capabilities: r.Capabilities, Options: r.Options,
		})
	}
	for _, m := range hc.Mounts {
		out.Mounts = append(out.Mounts, mountSpecOf(m))
	}
	for _, w := range hc.BlkioWeightDevice {
		if w != nil {
			out.BlkioWeightDevice = append(out.BlkioWeightDevice, model.WeightDevice{Path: w.Path, Weight: w.Weight})
		}
	}
	out.BlkioDeviceReadBps = throttlesOf(hc.BlkioDeviceReadBps)
	out.BlkioDeviceWriteBps = throttlesOf(hc.BlkioDeviceWriteBps)
	out.BlkioDeviceReadIOps = throttlesOf(hc.BlkioDeviceReadIOps)
	out.BlkioDeviceWriteIOps = throttlesOf(hc.BlkioDeviceWriteIOps)
	if hc.LogConfig.Type != "" {
		out.LogConfig = &model.LogConfig{Type: hc.LogConfig.Type, Config: hc.LogConfig.Config}
	}
	for _, u := range hc.Ulimits {
		if u != nil {
			out.Ulimits = append(out.Ulimits, model.Ulimit{Name: u.Name, Soft: u.Soft, Hard: u.Hard})
		}
	}
	if len(hc.PortBindings) > 0 {
		out.PortBindings = make(map[string][]model.PortBinding, len(hc.PortBindings))
		for port, bindings := range hc.PortBindings {
			pb := make([]model.PortBinding, 0, len(bindings))
			for _, b := range bindings {
				pb = append(pb, model.PortBinding{HostIP: addrString(b.HostIP), HostPort: b.HostPort})
			}
			out.PortBindings[port.String()] = pb
		}
	}
	for _, d := range hc.Devices {
		out.Devices = append(out.Devices, model.DeviceMapping{
			PathOnHost:        d.PathOnHost,
			PathInContainer:   d.PathInContainer,
			CgroupPermissions: d.CgroupPermissions,
		})
	}
	return out
}

func mountSpecOf(m mount.Mount) model.MountSpec {
	out := model.MountSpec{
		Type: string(m.Type), Source: m.Source, Target: m.Target, ReadOnly: m.ReadOnly, Consistency: string(m.Consistency),
	}
	if o := m.BindOptions; o != nil {
		out.BindOptions = &model.BindOptions{
			Propagation: string(o.Propagation), NonRecursive: o.NonRecursive, CreateMountpoint: o.CreateMountpoint,
			ReadOnlyNonRecursive: o.ReadOnlyNonRecursive, ReadOnlyForceRecursive: o.ReadOnlyForceRecursive,
		}
	}
	if o := m.VolumeOptions; o != nil {
		out.VolumeOptions = &model.VolumeOptions{NoCopy: o.NoCopy, Labels: o.Labels, Subpath: o.Subpath}
		if o.DriverConfig != nil {
			out.VolumeOptions.DriverName, out.VolumeOptions.DriverOptions = o.DriverConfig.Name, o.DriverConfig.Options
		}
	}
	if o := m.TmpfsOptions; o != nil {
		out.TmpfsOptions = &model.TmpfsOptions{SizeBytes: o.SizeBytes, Mode: uint32(o.Mode), Options: o.Options}
	}
	if m.ImageOptions != nil {
		out.ImageSubpath = m.ImageOptions.Subpath
	}
	return out
}

func mountOf(m model.MountSpec) mount.Mount {
	out := mount.Mount{
		Type: mount.Type(m.Type), Source: m.Source, Target: m.Target, ReadOnly: m.ReadOnly, Consistency: mount.Consistency(m.Consistency),
	}
	if o := m.BindOptions; o != nil {
		out.BindOptions = &mount.BindOptions{
			Propagation: mount.Propagation(o.Propagation), NonRecursive: o.NonRecursive, CreateMountpoint: o.CreateMountpoint,
			ReadOnlyNonRecursive: o.ReadOnlyNonRecursive, ReadOnlyForceRecursive: o.ReadOnlyForceRecursive,
		}
	}
	if o := m.VolumeOptions; o != nil {
		out.VolumeOptions = &mount.VolumeOptions{NoCopy: o.NoCopy, Labels: o.Labels, Subpath: o.Subpath}
		if o.DriverName != "" || len(o.DriverOptions) > 0 {
			out.VolumeOptions.DriverConfig = &mount.Driver{Name: o.DriverName, Options: o.DriverOptions}
		}
	}
	if o := m.TmpfsOptions; o != nil {
		out.TmpfsOptions = &mount.TmpfsOptions{SizeBytes: o.SizeBytes, Mode: os.FileMode(o.Mode), Options: o.Options}
	}
	if m.ImageSubpath != "" {
		out.ImageOptions = &mount.ImageOptions{Subpath: m.ImageSubpath}
	}
	return out
}

func throttlesOf(in []*blkiodev.ThrottleDevice) []model.ThrottleDevice {
	var out []model.ThrottleDevice
	for _, d := range in {
		if d != nil {
			out = append(out, model.ThrottleDevice{Path: d.Path, Rate: d.Rate})
		}
	}
	return out
}

func throttlesFrom(in []model.ThrottleDevice) []*blkiodev.ThrottleDevice {
	var out []*blkiodev.ThrottleDevice
	for _, d := range in {
		out = append(out, &blkiodev.ThrottleDevice{Path: d.Path, Rate: d.Rate})
	}
	return out
}

// buildCreateConfig builds the SDK create config/hostconfig from our captured
// inspect, preserving the security-relevant fields (SEC parity). It fails on
// an address or port the SDK cannot carry, which the daemon would refuse.
func buildCreateConfig(in model.Inspect) (*container.Config, *container.HostConfig, error) {
	cfg := &container.Config{
		Image:       in.Config.Image,
		Env:         in.Config.Env,
		Cmd:         in.Config.Cmd,
		User:        in.Config.User,
		Labels:      in.Config.Labels,
		Entrypoint:  in.Config.Entrypoint,
		WorkingDir:  in.Config.WorkingDir,
		StopSignal:  in.Config.StopSignal,
		StopTimeout: in.Config.StopTimeout,
		Hostname:    in.Config.Hostname,
		Domainname:  in.Config.Domainname,
		Tty:         in.Config.Tty,
		OpenStdin:   in.Config.OpenStdin,
	}
	if hc := in.Config.Healthcheck; hc != nil {
		cfg.Healthcheck = &container.HealthConfig{
			Test: hc.Test, Interval: hc.Interval, Timeout: hc.Timeout, StartPeriod: hc.StartPeriod, Retries: hc.Retries,
		}
	}

	hc := in.HostConfig
	hostCfg := &container.HostConfig{
		Binds:          hc.Binds,
		VolumesFrom:    hc.VolumesFrom,
		Privileged:     hc.Privileged,
		ReadonlyRootfs: hc.ReadonlyRootfs,
		SecurityOpt:    hc.SecurityOpt,
		CapAdd:         hc.CapAdd,
		CapDrop:        hc.CapDrop,
		NetworkMode:    container.NetworkMode(hc.NetworkMode),
		PidMode:        container.PidMode(hc.PidMode),
		IpcMode:        container.IpcMode(hc.IpcMode),
		UsernsMode:     container.UsernsMode(hc.UsernsMode),
		GroupAdd:       hc.GroupAdd,
		Sysctls:        hc.Sysctls,
		Tmpfs:          hc.Tmpfs,
		ExtraHosts:     hc.ExtraHosts,
		RestartPolicy: container.RestartPolicy{
			Name:              container.RestartPolicyMode(hc.RestartPolicy.Name),
			MaximumRetryCount: hc.RestartPolicy.MaximumRetryCount,
		},
		OomScoreAdj:  hc.OomScoreAdj,
		ShmSize:      hc.ShmSize,
		Init:         hc.Init,
		DNSSearch:    hc.DNSSearch,
		DNSOptions:   hc.DNSOptions,
		UTSMode:      container.UTSMode(hc.UTSMode),
		CgroupnsMode: container.CgroupnsMode(hc.CgroupnsMode),
		Links:        hc.Links,
		StorageOpt:   hc.StorageOpt,
		Runtime:      hc.Runtime,
	}
	if hc.LogConfig != nil {
		hostCfg.LogConfig = container.LogConfig{Type: hc.LogConfig.Type, Config: hc.LogConfig.Config}
	}
	for _, d := range hc.DNS {
		a, err := netip.ParseAddr(d)
		if err != nil {
			return nil, nil, fmt.Errorf("dns server: %w", err)
		}
		hostCfg.DNS = append(hostCfg.DNS, a)
	}
	// The limits are promoted from the embedded Resources struct, so they are
	// set by assignment (a struct literal can't address promoted fields).
	hostCfg.CgroupParent = hc.CgroupParent
	hostCfg.Memory = hc.Memory
	hostCfg.MemoryReservation = hc.MemoryReservation
	hostCfg.MemorySwap = hc.MemorySwap
	hostCfg.MemorySwappiness = hc.MemorySwappiness
	hostCfg.NanoCPUs = hc.NanoCPUs
	hostCfg.CPUShares = hc.CPUShares
	hostCfg.CPUPeriod = hc.CPUPeriod
	hostCfg.CPUQuota = hc.CPUQuota
	hostCfg.CpusetCpus = hc.CpusetCpus
	hostCfg.CpusetMems = hc.CpusetMems
	hostCfg.PidsLimit = hc.PidsLimit
	hostCfg.OomKillDisable = hc.OomKillDisable
	hostCfg.BlkioWeight = hc.BlkioWeight
	for _, w := range hc.BlkioWeightDevice {
		hostCfg.BlkioWeightDevice = append(hostCfg.BlkioWeightDevice, &blkiodev.WeightDevice{Path: w.Path, Weight: w.Weight})
	}
	hostCfg.BlkioDeviceReadBps = throttlesFrom(hc.BlkioDeviceReadBps)
	hostCfg.BlkioDeviceWriteBps = throttlesFrom(hc.BlkioDeviceWriteBps)
	hostCfg.BlkioDeviceReadIOps = throttlesFrom(hc.BlkioDeviceReadIOps)
	hostCfg.BlkioDeviceWriteIOps = throttlesFrom(hc.BlkioDeviceWriteIOps)
	for _, r := range hc.DeviceRequests {
		hostCfg.DeviceRequests = append(hostCfg.DeviceRequests, container.DeviceRequest{
			Driver: r.Driver, Count: r.Count, DeviceIDs: r.DeviceIDs, Capabilities: r.Capabilities, Options: r.Options,
		})
	}
	for _, u := range hc.Ulimits {
		hostCfg.Ulimits = append(hostCfg.Ulimits, &container.Ulimit{Name: u.Name, Soft: u.Soft, Hard: u.Hard})
	}
	for _, m := range hc.Mounts {
		hostCfg.Mounts = append(hostCfg.Mounts, mountOf(m))
	}
	if len(hc.PortBindings) > 0 {
		hostCfg.PortBindings = make(network.PortMap, len(hc.PortBindings))
		for port, bindings := range hc.PortBindings {
			p, err := network.ParsePort(port)
			if err != nil {
				return nil, nil, err
			}
			nb := make([]network.PortBinding, 0, len(bindings))
			for _, b := range bindings {
				ip, err := parseAddr(b.HostIP)
				if err != nil {
					return nil, nil, fmt.Errorf("port %s: host ip: %w", port, err)
				}
				nb = append(nb, network.PortBinding{HostIP: ip, HostPort: b.HostPort})
			}
			hostCfg.PortBindings[p] = nb
		}
	}
	for _, d := range hc.Devices {
		hostCfg.Devices = append(hostCfg.Devices, container.DeviceMapping{
			PathOnHost:        d.PathOnHost,
			PathInContainer:   d.PathInContainer,
			CgroupPermissions: d.CgroupPermissions,
		})
	}
	return cfg, hostCfg, nil
}

// buildNetworkingConfig recreates the primary network attachment with its
// static IP, MAC and aliases, so a container on an Unraid br0.x network comes
// back with the address it had. It is nil when there is neither an IP nor a
// MAC to keep, and Docker attaches the container as usual.
func buildNetworkingConfig(in model.Inspect) (*network.NetworkingConfig, error) {
	n := in.Network
	if n.Name == "" || (n.IPv4Address == "" && n.MACAddress == "") {
		return nil, nil
	}
	ep, err := endpointSettings(n)
	if err != nil {
		return nil, err
	}
	return &network.NetworkingConfig{
		EndpointsConfig: map[string]*network.EndpointSettings{n.Name: ep},
	}, nil
}

// endpointSettings builds the SDK endpoint settings for a captured attachment,
// preserving the static IP (if any), MAC (if any) and aliases.
func endpointSettings(n model.NetworkEndpoint) (*network.EndpointSettings, error) {
	ep := &network.EndpointSettings{Aliases: n.Aliases}
	if n.IPv4Address != "" {
		ip, err := netip.ParseAddr(n.IPv4Address)
		if err != nil {
			return nil, fmt.Errorf("network %s: %w", n.Name, err)
		}
		ep.IPAMConfig = &network.EndpointIPAMConfig{IPv4Address: ip}
	}
	if n.MACAddress != "" {
		mac, err := net.ParseMAC(n.MACAddress)
		if err != nil {
			return nil, fmt.Errorf("network %s: %w", n.Name, err)
		}
		ep.MacAddress = network.HardwareAddr(mac)
	}
	return ep, nil
}

// Docker's refusals of a runtime it does not know (the second from daemons
// before 23.0) and of a device request no driver serves, as for --gpus on a
// host without the NVIDIA container toolkit. The runtime refusals arrive on
// create, the device one on start.
var missingRuntimeRefusals = []string{
	"unknown or invalid runtime name: ",
	"Unknown runtime specified ",
	"could not select device driver ",
}

// missingRuntime marks err as a model.MissingRuntimeError when it is one of
// Docker's refusals of a runtime or device driver this host does not have.
func missingRuntime(err error) error {
	msg := err.Error()
	for _, r := range missingRuntimeRefusals {
		if strings.Contains(msg, r) {
			return &model.MissingRuntimeError{Err: err}
		}
	}
	return err
}
