package dockercli

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"

	"github.com/junkerderprovinz/bombvault/internal/model"
)

func gpuHostConfig() *container.HostConfig {
	hc := &container.HostConfig{Runtime: "nvidia"}
	hc.DeviceRequests = []container.DeviceRequest{{
		Count:        -1,
		DeviceIDs:    []string{"GPU-1"},
		Capabilities: [][]string{{"gpu", "utility"}},
		Options:      map[string]string{"mode": "x"},
	}}
	return hc
}

func TestRestoreKeepsTheRuntimeAndGPURequest(t *testing.T) {
	hc := restoredHostConfig(t, container.InspectResponse{
		ContainerJSONBase: &container.ContainerJSONBase{Name: "/plex", HostConfig: gpuHostConfig()},
		Config:            &container.Config{Image: "plexinc/pms-docker"},
	})
	if hc.Runtime != "nvidia" || len(hc.DeviceRequests) != 1 {
		t.Fatalf("runtime = %q, devices = %+v", hc.Runtime, hc.DeviceRequests)
	}
	r := hc.DeviceRequests[0]
	if r.Count != -1 || r.DeviceIDs[0] != "GPU-1" || r.Capabilities[0][1] != "utility" || r.Options["mode"] != "x" {
		t.Fatalf("device request = %+v", r)
	}
}

func TestIsolatedConfigAsksForNoGPUOrRuntime(t *testing.T) {
	from := mapInspect(container.InspectResponse{
		ContainerJSONBase: &container.ContainerJSONBase{Name: "/plex", HostConfig: gpuHostConfig()},
		Config:            &container.Config{Image: "plexinc/pms-docker"},
	})
	_, hc := isolatedConfig(IsolatedSpec{Name: StartTestPrefix + "plex-1", Network: StartTestPrefix + "net-1", From: from, NanoCPUs: 1, MemoryBytes: 1, PidsLimit: 1})
	if hc.Runtime != "" || len(hc.DeviceRequests) != 0 {
		t.Fatalf("runtime = %q, devices = %+v, want the daemon's default and no GPU", hc.Runtime, hc.DeviceRequests)
	}
	if granted := strings.Join(GrantedPrivileges(from), "; "); !strings.Contains(granted, "runtime nvidia") || !strings.Contains(granted, "GPU") {
		t.Fatalf("granted = %q, want the runtime and the GPU named", granted)
	}
}

// fakeDaemon answers a container create and start the way a Docker daemon
// refuses them.
func fakeDaemon(t *testing.T, createStatus int, createMsg string, startStatus int, startMsg string) *Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "/containers/create"):
			if createStatus != 0 {
				w.WriteHeader(createStatus)
				_, _ = fmt.Fprintf(w, "{%q:%q}", "message", createMsg)
				return
			}
			w.WriteHeader(http.StatusCreated)
			_, _ = fmt.Fprint(w, `{"Id":"abc","Warnings":[]}`)
		case strings.HasSuffix(r.URL.Path, "/start"):
			if startStatus != 0 {
				w.WriteHeader(startStatus)
				_, _ = fmt.Fprintf(w, "{%q:%q}", "message", startMsg)
				return
			}
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)
	api, err := client.NewClientWithOpts(client.WithHost("tcp://"+strings.TrimPrefix(srv.URL, "http://")), client.WithVersion("1.47"))
	if err != nil {
		t.Fatal(err)
	}
	return &Client{api: api}
}

func TestCreateAndStartRecognisesAHostWithoutTheRuntime(t *testing.T) {
	in := model.Inspect{Name: "/plex", Config: model.Config{Image: "plexinc/pms-docker"}}
	cases := []struct {
		name                      string
		createStatus, startStatus int
		createMsg, startMsg       string
		missing                   bool
	}{
		{name: "unknown runtime", createStatus: 400, createMsg: "unknown or invalid runtime name: nvidia", missing: true},
		{name: "unknown runtime, older daemon", createStatus: 400, createMsg: "Unknown runtime specified nvidia", missing: true},
		{name: "no GPU driver", startStatus: 500, startMsg: `could not select device driver "" with capabilities: [[gpu]]`, missing: true},
		{name: "name taken", createStatus: 409, createMsg: `Conflict. The container name "/plex" is already in use`},
		{name: "port taken", startStatus: 500, startMsg: "driver failed programming external connectivity: Bind for 0.0.0.0:32400 failed: port is already allocated"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := fakeDaemon(t, tc.createStatus, tc.createMsg, tc.startStatus, tc.startMsg)
			err := c.CreateAndStart(t.Context(), in, true)
			if err == nil {
				t.Fatal("want the daemon's refusal")
			}
			var missing *model.MissingRuntimeError
			if errors.As(err, &missing) != tc.missing {
				t.Fatalf("missing runtime = %v for %v", !tc.missing, err)
			}
			if tc.missing && !strings.Contains(missing.Error(), strings.TrimSpace(tc.createMsg+tc.startMsg)) {
				t.Fatalf("detail = %q, want the daemon's own words", missing.Error())
			}
		})
	}
}
