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
	// Stack is the compose project (com.docker.compose.project label), "" if none.
	Stack string
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
	// WaitRunning blocks until the named container reports Running, or until
	// timeout. Used after restarting a backed-up container so its network-namespace
	// dependents (network_mode: container:<ref>) can attach to a live target.
	WaitRunning(ctx context.Context, name string, timeout time.Duration) error
	Remove(ctx context.Context, name string) error
	Pull(ctx context.Context, image string) error
	// CreateAndStart recreates the container from the captured inspect and starts
	// it only when start is true (the caller decides, e.g. from the captured
	// run-state and a "leave stopped" restore option).
	CreateAndStart(ctx context.Context, in model.Inspect, start bool) error
	// InspectName returns the live container's name (normalized, no leading
	// slash) or "" when no such container exists. Used as the restore
	// wrong-target guard.
	InspectName(ctx context.Context, name string) (string, error)
	// Self returns the name (normalized) of the container this process runs in,
	// resolved from our hostname (Docker defaults it to the short container ID),
	// or "" when it can't be determined (not in a container / not found). Used so
	// a backup never stops its own container.
	Self(ctx context.Context) (string, error)
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
