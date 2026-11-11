package dockercli

import (
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/moby/moby/client"
)

// TestDockerClientOptsHonorsDockerHost checks that the default socket does not
// override DOCKER_HOST, which rootless Docker and Podman need. client.Opt
// values are opaque, so the test builds a client, which does not dial the
// daemon, and reads DaemonHost back.
func TestDockerClientOptsHonorsDockerHost(t *testing.T) {
	t.Run("DOCKER_HOST set is honored, not overridden", func(t *testing.T) {
		t.Setenv("DOCKER_HOST", "unix:///run/podman/podman.sock")

		c, err := client.New(dockerClientOpts()...)
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		defer func() { _ = c.Close() }()

		if got, want := c.DaemonHost(), "unix:///run/podman/podman.sock"; got != want {
			t.Errorf("DaemonHost() = %q, want %q (DOCKER_HOST was overridden)", got, want)
		}
	})

	t.Run("DOCKER_HOST empty (the unset case) defaults to the standard docker.sock", func(t *testing.T) {
		// The SDK ignores an empty DOCKER_HOST, so this is the unset case.
		t.Setenv("DOCKER_HOST", "")

		c, err := client.New(dockerClientOpts()...)
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		defer func() { _ = c.Close() }()

		if got, want := c.DaemonHost(), "unix:///var/run/docker.sock"; got != want {
			t.Errorf("DaemonHost() = %q, want %q (the default Unraid relies on)", got, want)
		}
	})
}

// An Unraid or TrueNAS host can run a Docker well behind the SDK, and it
// answers only requests in an API version it speaks.
func TestTheClientSpeaksTheAPIVersionOfAnOlderDaemon(t *testing.T) {
	var paths []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Api-Version", "1.41")
		paths = append(paths, r.URL.Path)
		if r.URL.Path == "/_ping" {
			_, _ = w.Write([]byte("OK"))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("[]"))
	}))
	t.Cleanup(srv.Close)
	t.Setenv("DOCKER_HOST", "tcp://"+strings.TrimPrefix(srv.URL, "http://"))
	t.Setenv("DOCKER_API_VERSION", "")

	api, err := client.New(dockerClientOpts()...)
	if err != nil {
		t.Fatal(err)
	}
	c := &Client{api: api}
	defer func() { _ = c.Close() }()
	if _, err := c.List(t.Context()); err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(paths, "/v1.41/containers/json") {
		t.Fatalf("requests = %v, want the list in API 1.41", paths)
	}
}
