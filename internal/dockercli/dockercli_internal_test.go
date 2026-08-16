package dockercli

import (
	"testing"

	"github.com/docker/docker/client"
)

// TestDockerClientOptsHonorsDockerHost guards the bug where New() called
// client.FromEnv (which applies DOCKER_HOST) and then unconditionally called
// client.WithHost("unix:///var/run/docker.sock"), silently overriding any
// DOCKER_HOST the operator set — making rootless Docker
// ($XDG_RUNTIME_DIR/docker.sock) and Podman (/run/podman/podman.sock)
// unreachable even though the operator configured everything correctly.
//
// client.Opt values are unexported closures: there is no way to assert "this
// opt was/was not included" by inspecting the []client.Opt slice itself. The
// honest testable mechanism is to apply the real opts through the real SDK
// constructor — client.NewClientWithOpts only builds the HTTP client, it never
// dials the daemon — and read back the result through *client.Client's own
// exported DaemonHost() accessor (client.go), which reports exactly the host
// string the opts configured. That is the SDK's supported introspection point,
// not an SDK internal this test would otherwise have to reach into.
func TestDockerClientOptsHonorsDockerHost(t *testing.T) {
	t.Run("DOCKER_HOST set is honored, not overridden", func(t *testing.T) {
		t.Setenv("DOCKER_HOST", "unix:///run/podman/podman.sock")

		c, err := client.NewClientWithOpts(dockerClientOpts()...)
		if err != nil {
			t.Fatalf("NewClientWithOpts: %v", err)
		}
		defer func() { _ = c.Close() }()

		if got, want := c.DaemonHost(), "unix:///run/podman/podman.sock"; got != want {
			t.Errorf("DaemonHost() = %q, want %q (DOCKER_HOST was overridden)", got, want)
		}
	})

	t.Run("DOCKER_HOST empty (the unset case) defaults to the standard docker.sock", func(t *testing.T) {
		// t.Setenv(..., "") stands in for "unset": os.Getenv cannot tell an
		// empty value apart from a missing one, and neither can the SDK's own
		// WithHostFromEnv (envvars.go: EnvOverrideHost is read "when set to a
		// non-empty value") — so this exercises the exact code path an
		// operator with no DOCKER_HOST in their environment hits.
		t.Setenv("DOCKER_HOST", "")

		c, err := client.NewClientWithOpts(dockerClientOpts()...)
		if err != nil {
			t.Fatalf("NewClientWithOpts: %v", err)
		}
		defer func() { _ = c.Close() }()

		if got, want := c.DaemonHost(), "unix:///var/run/docker.sock"; got != want {
			t.Errorf("DaemonHost() = %q, want %q (Unraid's prior default must be unchanged)", got, want)
		}
	})
}
