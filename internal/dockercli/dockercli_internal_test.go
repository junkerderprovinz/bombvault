package dockercli

import (
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/moby/moby/client"

	"github.com/junkerderprovinz/bombvault/internal/model"
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

// daemonAt is a Docker daemon that speaks API version and answers a list with
// no containers and a create with a new one. It returns the paths it was asked
// for.
func daemonAt(t *testing.T, version string) (*Client, *[]string) {
	t.Helper()
	var (
		mu    sync.Mutex
		paths []string
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		paths = append(paths, r.URL.Path)
		mu.Unlock()
		w.Header().Set("Api-Version", version)
		switch {
		case r.URL.Path == "/_ping":
			_, _ = w.Write([]byte("OK"))
		case strings.HasSuffix(r.URL.Path, "/containers/create"):
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"Id":"abc","Warnings":[]}`))
		default:
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte("[]"))
		}
	}))
	t.Cleanup(srv.Close)
	t.Setenv("DOCKER_HOST", "tcp://"+strings.TrimPrefix(srv.URL, "http://"))
	t.Setenv("DOCKER_API_VERSION", "")

	api, err := client.New(dockerClientOpts()...)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = api.Close() })
	return &Client{api: api}, &paths
}

// An Unraid or TrueNAS host can run a Docker well behind the SDK, and it
// answers only requests in an API version it speaks.
func TestTheClientSpeaksTheAPIVersionOfAnOlderDaemon(t *testing.T) {
	c, paths := daemonAt(t, "1.41")
	if _, err := c.List(t.Context()); err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(*paths, "/v1.41/containers/json") {
		t.Fatalf("requests = %v, want the list in API 1.41", *paths)
	}
}

// A daemon before API 1.44 drops a MAC set per network and a recursive
// read-only bind without a word, which would bring a container back with a new
// MAC its DHCP reservation does not know, or with a writable submount.
func TestARecreateNeedingAPI144IsRefusedByAnOlderDaemon(t *testing.T) {
	pinnedMAC := model.Inspect{
		Name: "/plex", Config: model.Config{Image: "plexinc/pms-docker"},
		Network: model.NetworkEndpoint{Name: "br0.20", MACAddress: "02:42:ac:11:00:02"},
	}
	readOnlyBind := model.Inspect{
		Name: "/app", Config: model.Config{Image: "alpine"},
		HostConfig: model.HostConfig{Mounts: []model.MountSpec{{
			Type: "bind", Source: "/mnt/user/media", Target: "/media", ReadOnly: true,
			BindOptions: &model.BindOptions{ReadOnlyForceRecursive: true},
		}}},
	}
	cases := []struct {
		name string
		in   model.Inspect
		want string
	}{
		{"MAC per network", pinnedMAC, `"specify mac-address per network" requires API version 1.44, but the Docker daemon API version is 1.43`},
		{"recursive read-only bind", readOnlyBind, `"bind-recursive=readonly" requires API version 1.44, but the Docker daemon API version is 1.43`},
	}
	for _, tc := range cases {
		t.Run(tc.name+" at 1.43", func(t *testing.T) {
			c, paths := daemonAt(t, "1.43")
			err := c.CreateAndStart(t.Context(), tc.in, false)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want %q", err, tc.want)
			}
			if slices.Contains(*paths, "/v1.43/containers/create") {
				t.Fatalf("requests = %v, want no create", *paths)
			}
		})
		t.Run(tc.name+" at 1.44", func(t *testing.T) {
			c, paths := daemonAt(t, "1.44")
			if err := c.CreateAndStart(t.Context(), tc.in, false); err != nil {
				t.Fatal(err)
			}
			if !slices.Contains(*paths, "/v1.44/containers/create") {
				t.Fatalf("requests = %v, want the create", *paths)
			}
		})
	}
}
