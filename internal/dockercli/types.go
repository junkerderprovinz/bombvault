package dockercli

import (
	"context"
	"time"
)

// PortBinding maps a published host endpoint for a container port.
type PortBinding struct {
	HostIP   string
	HostPort string
}

// DeviceMapping is a single HostConfig.Devices entry (host device → container device).
type DeviceMapping struct {
	PathOnHost        string
	PathInContainer   string
	CgroupPermissions string
}

// RestartPolicy mirrors the Docker container restart policy.
type RestartPolicy struct {
	Name              string
	MaximumRetryCount int
}

// Mount describes a single bind/volume mount as reported by inspect.
type Mount struct {
	Type        string
	Source      string
	Destination string
}

// ContainerInfo is a summary of a container as returned by List.
type ContainerInfo struct {
	ID     string
	Name   string // normalized (no leading slash)
	Image  string
	State  string
	Status string
}

// ContainerConfig holds the portable container configuration we preserve.
type ContainerConfig struct {
	Image string
	Env   []string
	Cmd   []string
	// User is the process user (e.g. "1000:1000"). SEC: preserved on recreate.
	User string
}

// HostConfig holds the host-side configuration we preserve on recreate.
// The Cap/Privileged/SecurityOpt/ReadonlyRootfs/NetworkMode/Devices fields are
// security-relevant: a recreated container must never gain privilege over the
// original (SEC parity with the TypeScript implementation).
type HostConfig struct {
	Binds         []string
	PortBindings  map[string][]PortBinding
	RestartPolicy RestartPolicy
	// SEC: security-relevant fields preserved on recreate.
	CapAdd         []string
	CapDrop        []string
	Privileged     bool
	SecurityOpt    []string
	ReadonlyRootfs bool
	NetworkMode    string
	Devices        []DeviceMapping
}

// ContainerInspect is the subset of a container's inspect data that BombVault
// captures at backup time and uses to recreate the container on restore.
type ContainerInspect struct {
	ID         string
	Name       string // dockerode-style, may carry a leading slash (e.g. "/plex")
	Image      string
	Config     ContainerConfig
	HostConfig HostConfig
	Mounts     []Mount
}

// Docker is the host-control surface consumed by the backup orchestrator.
// It is deliberately small and interface-shaped so the orchestrator can be
// unit-tested with fakes (the DI seam) without a real docker.sock.
type Docker interface {
	List(ctx context.Context) ([]ContainerInfo, error)
	Inspect(ctx context.Context, name string) (ContainerInspect, error)
	Stop(ctx context.Context, name string, timeout time.Duration) error
	Start(ctx context.Context, name string) error
	Remove(ctx context.Context, name string) error
	Pull(ctx context.Context, image string) error
	CreateAndStart(ctx context.Context, in ContainerInspect) error
	// InspectName returns the live container's name (normalized, no leading
	// slash) or "" when no such container exists. Used as the restore
	// wrong-target guard.
	InspectName(ctx context.Context, name string) (string, error)
}

// normalizeName strips a single leading slash from a docker container name.
func normalizeName(name string) string {
	if len(name) > 0 && name[0] == '/' {
		return name[1:]
	}
	return name
}
