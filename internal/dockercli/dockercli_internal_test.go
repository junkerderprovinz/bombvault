package dockercli

import (
	"testing"

	"github.com/docker/docker/client"
)

// TestDockerClientOptsHonorsDockerHost checks that the default socket does not
// override DOCKER_HOST, which rootless Docker and Podman need. client.Opt
// values are opaque, so the test builds a client, which does not dial the
// daemon, and reads DaemonHost back.
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
		// The SDK ignores an empty DOCKER_HOST, so this is the unset case.
		t.Setenv("DOCKER_HOST", "")

		c, err := client.NewClientWithOpts(dockerClientOpts()...)
		if err != nil {
			t.Fatalf("NewClientWithOpts: %v", err)
		}
		defer func() { _ = c.Close() }()

		if got, want := c.DaemonHost(), "unix:///var/run/docker.sock"; got != want {
			t.Errorf("DaemonHost() = %q, want %q (the default Unraid relies on)", got, want)
		}
	})
}
