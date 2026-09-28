package dockercli

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
	"time"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/go-connections/nat"

	"github.com/junkerderprovinz/bombvault/internal/model"
)

// StartTestLabel marks every container and network a start test creates. The
// cleanup removes nothing that lacks it.
const StartTestLabel = "bombvault.starttest"

// StartTestPrefix starts the name of everything a start test creates.
const StartTestPrefix = "bombvault-test-"

// IsolatedSpec describes the copy a start test runs: From is the recipe a
// restore would create the container from, Binds are the bind mounts already
// pointed at the restored copy of its data.
type IsolatedSpec struct {
	Name        string
	Network     string
	From        model.Inspect
	Binds       []string
	NanoCPUs    int64
	MemoryBytes int64
	PidsLimit   int64
	Labels      map[string]string
}

// IsolatedState is what a start test reads off its running copy.
type IsolatedState struct {
	Running   bool
	ExitCode  int
	OOMKilled bool
	// Health is empty without a healthcheck, else starting, healthy or
	// unhealthy.
	Health string
	// ExposedPorts are the ports the image and the recipe expose, such as
	// "8080/tcp".
	ExposedPorts []string
}

// ProberSpec is a short-lived container that runs one command inside a start
// test's network and reports its exit code.
type ProberSpec struct {
	Name       string
	Image      string
	Network    string
	Entrypoint []string
	Cmd        []string
	Timeout    time.Duration
}

// StartTestHost is what a start test needs from Docker beyond the Docker
// interface. Everything it creates carries StartTestLabel and a name starting
// with StartTestPrefix, and it removes nothing else.
type StartTestHost interface {
	CreateTestNetwork(ctx context.Context, name string) error
	RemoveTestNetwork(ctx context.Context, name string) error
	StartIsolated(ctx context.Context, spec IsolatedSpec) error
	IsolatedState(ctx context.Context, name string) (IsolatedState, error)
	RemoveTestContainer(ctx context.Context, name string) error
	RunProber(ctx context.Context, spec ProberSpec) (int, error)
	StartTestLeftovers(ctx context.Context) (containers, networks []string, err error)
}

var _ StartTestHost = (*Client)(nil)

// errNotStartTest refuses to remove something a start test did not create.
var errNotStartTest = errors.New("not created by a start test")

// CreateTestNetwork creates an internal bridge network: its containers reach
// each other and nothing else, and nothing outside reaches them.
func (c *Client) CreateTestNetwork(ctx context.Context, name string) error {
	if !strings.HasPrefix(name, StartTestPrefix) {
		return fmt.Errorf("dockercli: test network %q: %w", name, errNotStartTest)
	}
	_, err := c.api.NetworkCreate(ctx, name, network.CreateOptions{
		Driver:   "bridge",
		Internal: true,
		Labels:   map[string]string{StartTestLabel: "1"},
	})
	if err != nil {
		return fmt.Errorf("dockercli: create test network: %w", err)
	}
	return nil
}

// RemoveTestNetwork removes a network a start test created. One that is gone
// already is fine.
func (c *Client) RemoveTestNetwork(ctx context.Context, name string) error {
	n, err := c.api.NetworkInspect(ctx, name, network.InspectOptions{})
	if err != nil {
		if isNotFoundErr(err) {
			return nil
		}
		return fmt.Errorf("dockercli: inspect test network: %w", err)
	}
	if n.Labels[StartTestLabel] != "1" || !strings.HasPrefix(n.Name, StartTestPrefix) {
		return fmt.Errorf("dockercli: network %q: %w", name, errNotStartTest)
	}
	if err := c.api.NetworkRemove(ctx, n.ID); err != nil && !isNotFoundErr(err) {
		return fmt.Errorf("dockercli: remove test network: %w", err)
	}
	return nil
}

// StartIsolated creates and starts the isolated copy. An image that is not on
// the host any more is pulled once.
func (c *Client) StartIsolated(ctx context.Context, spec IsolatedSpec) error {
	if !strings.HasPrefix(spec.Name, StartTestPrefix) || !strings.HasPrefix(spec.Network, StartTestPrefix) {
		return fmt.Errorf("dockercli: isolated copy %q: %w", spec.Name, errNotStartTest)
	}
	cfg, hostCfg := isolatedConfig(spec)
	_, err := c.api.ContainerCreate(ctx, cfg, hostCfg, nil, nil, spec.Name)
	if err != nil && isNotFoundErr(err) && cfg.Image != "" {
		if pErr := c.Pull(ctx, cfg.Image); pErr != nil {
			return fmt.Errorf("dockercli: the image %s is not on this host and could not be pulled: %w", cfg.Image, pErr)
		}
		_, err = c.api.ContainerCreate(ctx, cfg, hostCfg, nil, nil, spec.Name)
	}
	if err != nil {
		return fmt.Errorf("dockercli: create isolated copy: %w", err)
	}
	if err := c.api.ContainerStart(ctx, spec.Name, container.StartOptions{}); err != nil {
		return fmt.Errorf("dockercli: start isolated copy: %w", err)
	}
	return nil
}

// isolatedConfig turns a restore recipe into the copy a start test may run.
// It builds on what a restore would create and then takes away every way out:
// no published ports, no other network, no host namespaces or devices, no
// restart, and fixed limits. The caller checks the recipe before, and this
// holds even if it did not.
func isolatedConfig(spec IsolatedSpec) (*container.Config, *container.HostConfig) {
	cfg, hostCfg := buildCreateConfig(spec.From)

	// None of the original's labels: Compose would count the copy as a member
	// of the project, Unraid as a managed app, and a reverse proxy or an
	// updater would route to it or act on it.
	labels := maps.Clone(spec.Labels)
	if labels == nil {
		labels = map[string]string{}
	}
	labels[StartTestLabel] = "1"
	cfg.Labels = labels

	if len(spec.From.HostConfig.PortBindings) > 0 {
		cfg.ExposedPorts = nat.PortSet{}
		for port := range spec.From.HostConfig.PortBindings {
			cfg.ExposedPorts[nat.Port(port)] = struct{}{}
		}
	}

	hostCfg.Binds = spec.Binds
	hostCfg.PortBindings = nil
	hostCfg.PublishAllPorts = false
	hostCfg.NetworkMode = container.NetworkMode(spec.Network)
	hostCfg.RestartPolicy = container.RestartPolicy{Name: container.RestartPolicyDisabled}
	hostCfg.Privileged = false
	hostCfg.Devices = nil
	hostCfg.PidMode = ""
	hostCfg.IpcMode = ""
	hostCfg.UsernsMode = ""
	// Added capabilities, unconfined profiles, sysctls and another cgroup
	// parent would give the copy what the original was granted on the host. A
	// copy that cannot start without them fails its test.
	hostCfg.CapAdd = nil
	hostCfg.SecurityOpt = restrictingSecurityOpts(spec.From.HostConfig.SecurityOpt)
	hostCfg.Sysctls = nil
	hostCfg.CgroupParent = ""
	hostCfg.NanoCPUs = spec.NanoCPUs
	hostCfg.Memory = spec.MemoryBytes
	hostCfg.MemorySwap = spec.MemoryBytes
	pids := spec.PidsLimit
	hostCfg.PidsLimit = &pids
	return cfg, hostCfg
}

// restrictingSecurityOpts keeps only no-new-privileges, the one security
// option that takes something away.
func restrictingSecurityOpts(opts []string) []string {
	var out []string
	for _, o := range opts {
		if strings.HasPrefix(o, "no-new-privileges") {
			out = append(out, o)
		}
	}
	return out
}

// GrantedPrivileges names what a recipe adds to a container beyond Docker's
// defaults, which a start test's copy runs without.
func GrantedPrivileges(in model.Inspect) []string {
	hc := in.HostConfig
	var out []string
	if len(hc.CapAdd) > 0 {
		out = append(out, "capabilities "+strings.Join(hc.CapAdd, ", "))
	}
	for _, o := range hc.SecurityOpt {
		if !strings.HasPrefix(o, "no-new-privileges") {
			out = append(out, "security option "+o)
		}
	}
	if len(hc.Sysctls) > 0 {
		keys := slices.Sorted(maps.Keys(hc.Sysctls))
		out = append(out, "sysctls "+strings.Join(keys, ", "))
	}
	if hc.CgroupParent != "" {
		out = append(out, "cgroup parent "+hc.CgroupParent)
	}
	return out
}

// IsolatedState reads the state of a start test's copy.
func (c *Client) IsolatedState(ctx context.Context, name string) (IsolatedState, error) {
	resp, err := c.api.ContainerInspect(ctx, name)
	if err != nil {
		return IsolatedState{}, fmt.Errorf("dockercli: inspect isolated copy: %w", err)
	}
	var st IsolatedState
	if resp.State != nil {
		st.Running = resp.State.Running
		st.ExitCode = resp.State.ExitCode
		st.OOMKilled = resp.State.OOMKilled
		if resp.State.Health != nil {
			st.Health = string(resp.State.Health.Status)
		}
	}
	if resp.Config != nil {
		for port := range resp.Config.ExposedPorts {
			st.ExposedPorts = append(st.ExposedPorts, string(port))
		}
	}
	return st, nil
}

// RemoveTestContainer removes a container a start test created, with its
// anonymous volumes. One that is gone already is fine.
func (c *Client) RemoveTestContainer(ctx context.Context, name string) error {
	resp, err := c.api.ContainerInspect(ctx, name)
	if err != nil {
		if isNoSuchContainer(err) {
			return nil
		}
		return fmt.Errorf("dockercli: inspect test container: %w", err)
	}
	if resp.Config == nil || resp.Config.Labels[StartTestLabel] != "1" || !strings.HasPrefix(normalizeName(resp.Name), StartTestPrefix) {
		return fmt.Errorf("dockercli: container %q: %w", name, errNotStartTest)
	}
	err = c.api.ContainerRemove(ctx, resp.ID, container.RemoveOptions{Force: true, RemoveVolumes: true})
	if err != nil && !isNoSuchContainer(err) {
		return fmt.Errorf("dockercli: remove test container: %w", err)
	}
	return nil
}

// RunProber runs one command in a small container on the test network and
// returns its exit code. The container is removed whatever happens.
func (c *Client) RunProber(ctx context.Context, spec ProberSpec) (int, error) {
	if !strings.HasPrefix(spec.Name, StartTestPrefix) || !strings.HasPrefix(spec.Network, StartTestPrefix) {
		return -1, fmt.Errorf("dockercli: prober %q: %w", spec.Name, errNotStartTest)
	}
	pids := int64(64)
	hostCfg := &container.HostConfig{
		NetworkMode:   container.NetworkMode(spec.Network),
		RestartPolicy: container.RestartPolicy{Name: container.RestartPolicyDisabled},
	}
	hostCfg.NanoCPUs = 250_000_000
	hostCfg.Memory = 64 << 20
	hostCfg.MemorySwap = 64 << 20
	hostCfg.PidsLimit = &pids
	cfg := &container.Config{
		Image:      spec.Image,
		Entrypoint: spec.Entrypoint,
		Cmd:        spec.Cmd,
		Labels:     map[string]string{StartTestLabel: "1"},
	}
	if _, err := c.api.ContainerCreate(ctx, cfg, hostCfg, nil, nil, spec.Name); err != nil {
		return -1, fmt.Errorf("dockercli: create prober: %w", err)
	}
	defer func() {
		rctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
		defer cancel()
		_ = c.RemoveTestContainer(rctx, spec.Name)
	}()
	wctx, cancel := context.WithTimeout(ctx, spec.Timeout)
	defer cancel()
	waitC, errC := c.api.ContainerWait(wctx, spec.Name, container.WaitConditionNextExit)
	if err := c.api.ContainerStart(ctx, spec.Name, container.StartOptions{}); err != nil {
		return -1, fmt.Errorf("dockercli: start prober: %w", err)
	}
	select {
	case res := <-waitC:
		return int(res.StatusCode), nil
	case err := <-errC:
		return -1, fmt.Errorf("dockercli: wait for prober: %w", err)
	}
}

// StartTestLeftovers lists the containers and networks earlier start tests
// left behind, by label and name.
func (c *Client) StartTestLeftovers(ctx context.Context) ([]string, []string, error) {
	f := filters.NewArgs(filters.Arg("label", StartTestLabel+"=1"))
	cs, err := c.api.ContainerList(ctx, container.ListOptions{All: true, Filters: f})
	if err != nil {
		return nil, nil, fmt.Errorf("dockercli: list test containers: %w", err)
	}
	var containers []string
	for _, s := range cs {
		if len(s.Names) > 0 && strings.HasPrefix(normalizeName(s.Names[0]), StartTestPrefix) {
			containers = append(containers, normalizeName(s.Names[0]))
		}
	}
	ns, err := c.api.NetworkList(ctx, network.ListOptions{Filters: f})
	if err != nil {
		return containers, nil, fmt.Errorf("dockercli: list test networks: %w", err)
	}
	var networks []string
	for _, n := range ns {
		if strings.HasPrefix(n.Name, StartTestPrefix) {
			networks = append(networks, n.Name)
		}
	}
	return containers, networks, nil
}

// isNotFoundErr reports the daemon's "no such" answer for any kind of object.
func isNotFoundErr(err error) bool { return cerrdefs.IsNotFound(err) }
