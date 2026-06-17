package dockercli

import (
	"context"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/model"
)

// ContainerInfo is a summary of a container as returned by List.
type ContainerInfo struct {
	ID     string
	Name   string // normalized (no leading slash)
	Image  string
	State  string
	Status string
	// IP is the first non-empty IP address found in the container's network
	// settings. Empty when the container has no network (e.g. host networking
	// configured without an explicit IP, or a stopped container whose network
	// state is not stored in the list summary).
	IP string
}

// Docker is the host-control surface consumed by the backup orchestrator.
// It is deliberately small and interface-shaped so the orchestrator can be
// unit-tested with fakes (the DI seam) without a real docker.sock.
//
// The rich container profile flows through this surface as model.Inspect so the
// security-relevant fields (User, Cap*, Privileged, SecurityOpt, ReadonlyRootfs,
// NetworkMode, Devices, …) are preserved end to end on restore (SEC §8).
type Docker interface {
	List(ctx context.Context) ([]ContainerInfo, error)
	Inspect(ctx context.Context, name string) (model.Inspect, error)
	// Allocations reports the static IP / published host ports every container
	// currently holds, for the restore pre-flight conflict check.
	Allocations(ctx context.Context) ([]model.Allocation, error)
	Stop(ctx context.Context, name string, timeout time.Duration) error
	Start(ctx context.Context, name string) error
	Remove(ctx context.Context, name string) error
	Pull(ctx context.Context, image string) error
	CreateAndStart(ctx context.Context, in model.Inspect) error
	// InspectName returns the live container's name (normalized, no leading
	// slash) or "" when no such container exists. Used as the restore
	// wrong-target guard.
	InspectName(ctx context.Context, name string) (string, error)
	// Exec runs cmd inside the (running) container and returns an error when the
	// command exits non-zero. Used for pre/post-backup hooks.
	Exec(ctx context.Context, name string, cmd []string) error
}

// normalizeName strips a single leading slash from a docker container name.
func normalizeName(name string) string {
	if len(name) > 0 && name[0] == '/' {
		return name[1:]
	}
	return name
}
