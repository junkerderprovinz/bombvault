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
	Stop(ctx context.Context, name string, timeout time.Duration) error
	Start(ctx context.Context, name string) error
	Remove(ctx context.Context, name string) error
	Pull(ctx context.Context, image string) error
	CreateAndStart(ctx context.Context, in model.Inspect) error
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
