package dockercli

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
)

// engine is a Docker daemon with a few containers in memory, enough for the
// calls a recreate makes.
type engine struct {
	mu         sync.Mutex
	containers map[string]*container.InspectResponse // by ID
	calls      []string
	refuse     string // the name a create is refused for
}

func newEngine(t *testing.T, cs ...*container.InspectResponse) (*engine, *Client) {
	t.Helper()
	e := &engine{containers: map[string]*container.InspectResponse{}}
	for _, c := range cs {
		e.containers[c.ID] = c
	}
	srv := httptest.NewServer(http.HandlerFunc(e.serve))
	t.Cleanup(srv.Close)
	api, err := client.NewClientWithOpts(client.WithHost("tcp://"+strings.TrimPrefix(srv.URL, "http://")), client.WithVersion("1.47"))
	if err != nil {
		t.Fatal(err)
	}
	return e, &Client{api: api}
}

func running(id, name string, hc container.HostConfig) *container.InspectResponse {
	return &container.InspectResponse{
		ContainerJSONBase: &container.ContainerJSONBase{ID: id, Name: "/" + name, Image: "sha256:img", State: &container.State{Running: true}, HostConfig: &hc},
		Config:            &container.Config{Image: name},
	}
}

// find resolves a reference the way Docker does: ID, then name, then ID prefix.
func (e *engine) find(ref string) *container.InspectResponse {
	if c, ok := e.containers[ref]; ok {
		return c
	}
	for _, c := range e.containers {
		if c.Name == "/"+ref {
			return c
		}
	}
	for id, c := range e.containers {
		if strings.HasPrefix(id, ref) {
			return c
		}
	}
	return nil
}

func (e *engine) serve(w http.ResponseWriter, r *http.Request) {
	e.mu.Lock()
	defer e.mu.Unlock()
	path := strings.TrimPrefix(r.URL.Path, "/v1.47")
	e.calls = append(e.calls, r.Method+" "+path+"?"+r.URL.RawQuery)
	w.Header().Set("Content-Type", "application/json")
	notFound := func() {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"No such container"}`))
	}
	switch {
	case r.Method == http.MethodGet && path == "/containers/json":
		var list []container.Summary
		for _, c := range e.containers {
			list = append(list, container.Summary{ID: c.ID, Names: []string{c.Name}})
		}
		_ = json.NewEncoder(w).Encode(list)
	case r.Method == http.MethodGet && strings.HasPrefix(path, "/images/"):
		_, _ = w.Write([]byte(`{"Id":"sha256:img","Config":{}}`))
	case r.Method == http.MethodPost && path == "/containers/create":
		var req container.CreateRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		name := r.URL.Query().Get("name")
		if name == e.refuse {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"message":"refused"}`))
			return
		}
		id := "new-" + name + strings.Repeat("0", 60)
		e.containers[id] = &container.InspectResponse{
			ContainerJSONBase: &container.ContainerJSONBase{ID: id, Name: "/" + name, State: &container.State{}, HostConfig: req.HostConfig},
			Config:            req.Config,
		}
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(container.CreateResponse{ID: id})
	default:
		parts := strings.Split(strings.TrimPrefix(path, "/containers/"), "/")
		c := e.find(parts[0])
		if c == nil {
			notFound()
			return
		}
		switch {
		case r.Method == http.MethodGet && len(parts) == 2 && parts[1] == "json":
			_ = json.NewEncoder(w).Encode(c)
		case r.Method == http.MethodDelete:
			delete(e.containers, c.ID)
			w.WriteHeader(http.StatusNoContent)
		case len(parts) == 2 && parts[1] == "rename":
			c.Name = "/" + r.URL.Query().Get("name")
			w.WriteHeader(http.StatusNoContent)
		case len(parts) == 2 && parts[1] == "stop":
			c.State.Running = false
			w.WriteHeader(http.StatusNoContent)
		case len(parts) == 2 && parts[1] == "start":
			c.State.Running = true
			w.WriteHeader(http.StatusNoContent)
		default:
			notFound()
		}
	}
}

func (e *engine) named(name string) *container.InspectResponse {
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, c := range e.containers {
		if c.Name == "/"+name {
			return c
		}
	}
	return nil
}

var (
	oldGluetun = strings.Repeat("a", 64)
	newGluetun = strings.Repeat("b", 64)
	qbitID     = strings.Repeat("c", 64)
)

// Docker stores a shared network as the other container's ID. The definition
// keeps its name, which a recreate of that container does not change.
func TestInspectNamesTheContainerWhoseNetworkItShares(t *testing.T) {
	_, c := newEngine(t,
		running(newGluetun, "gluetun", container.HostConfig{NetworkMode: "bridge"}),
		running(qbitID, "qbittorrent", container.HostConfig{NetworkMode: container.NetworkMode("container:" + newGluetun), IpcMode: container.IpcMode("container:" + oldGluetun)}),
	)
	in, err := c.Inspect(t.Context(), "qbittorrent")
	if err != nil {
		t.Fatal(err)
	}
	if in.HostConfig.NetworkMode != "container:gluetun" {
		t.Fatalf("network mode = %q, want the name", in.HostConfig.NetworkMode)
	}
	if in.HostConfig.IpcMode != "container:"+oldGluetun {
		t.Fatalf("a mode whose container is gone keeps its ID, got %q", in.HostConfig.IpcMode)
	}
}

// After gluetun was recreated, qbittorrent still names its old ID and can
// never start. It is recreated onto the new gluetun and started again.
func TestRejoinNamespaceRecreatesTheSharersOntoTheNewContainer(t *testing.T) {
	e, c := newEngine(t,
		running(newGluetun, "gluetun", container.HostConfig{NetworkMode: "bridge"}),
		running(qbitID, "qbittorrent", container.HostConfig{NetworkMode: container.NetworkMode("container:" + oldGluetun)}),
		running(strings.Repeat("e", 64), "plex", container.HostConfig{NetworkMode: "bridge"}),
	)
	failed, err := c.RejoinNamespace(t.Context(), oldGluetun, "gluetun")
	if err != nil || len(failed) != 0 {
		t.Fatalf("failed = %v, err = %v", failed, err)
	}
	q := e.named("qbittorrent")
	if q == nil || q.ID == qbitID || q.HostConfig.NetworkMode != "container:gluetun" || !q.State.Running {
		t.Fatalf("qbittorrent = %+v, want it recreated on gluetun's network and running", q)
	}
	if _, left := e.containers[qbitID]; left {
		t.Fatal("the old qbittorrent must be removed")
	}
	if e.named("plex").ID != strings.Repeat("e", 64) {
		t.Fatal("a container that shares nothing is left alone")
	}
}

func TestRejoinNamespaceKeepsTheOldContainerWhenTheCreateIsRefused(t *testing.T) {
	e, c := newEngine(t,
		running(newGluetun, "gluetun", container.HostConfig{NetworkMode: "bridge"}),
		running(qbitID, "qbittorrent", container.HostConfig{NetworkMode: container.NetworkMode("container:" + oldGluetun)}),
	)
	e.refuse = "qbittorrent"
	failed, err := c.RejoinNamespace(t.Context(), oldGluetun, "gluetun")
	if err == nil || !slices.Equal(failed, []string{"qbittorrent"}) {
		t.Fatalf("failed = %v, err = %v", failed, err)
	}
	if q := e.named("qbittorrent"); q == nil || q.ID != qbitID {
		t.Fatalf("the old container must be back under its name, got %+v", q)
	}
}

func TestContainerIDTakesTheNameOnly(t *testing.T) {
	_, c := newEngine(t, running(strings.Repeat("ca", 32), "plex", container.HostConfig{}))
	if id, err := c.ContainerID(t.Context(), "plex"); err != nil || id != strings.Repeat("ca", 32) {
		t.Fatalf("id = %q, err = %v", id, err)
	}
	if id, err := c.ContainerID(t.Context(), "cafe"); err != nil || id != "" {
		t.Fatalf("an ID prefix is not a name: id = %q, err = %v", id, err)
	}
}

func TestStopGivesTheContainersOwnLongerTimeout(t *testing.T) {
	own := 120
	db := running(strings.Repeat("d", 64), "mariadb", container.HostConfig{})
	db.Config.StopTimeout = &own
	e, c := newEngine(t, db)
	if err := c.Stop(t.Context(), "mariadb", rejoinStopTimeout); err != nil {
		t.Fatal(err)
	}
	if !slices.ContainsFunc(e.calls, func(call string) bool { return strings.Contains(call, "/stop?") && strings.Contains(call, "t=120") }) {
		t.Fatalf("the stop must wait the container's own 120 s: %v", e.calls)
	}
}
