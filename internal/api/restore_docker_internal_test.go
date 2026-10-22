package api

import (
	"context"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/backup"
	"github.com/junkerderprovinz/bombvault/internal/dockercli"
)

type rejoinRecorder struct {
	dockercli.Docker
	asked string
}

func (r *rejoinRecorder) ContainerID(_ context.Context, name string) (string, error) {
	r.asked = name
	return "id-" + name, nil
}

func (r *rejoinRecorder) RejoinNamespace(context.Context, string, string) ([]string, error) {
	return nil, nil
}

// A restore of a container others share a network with has to bring them
// along, which it can only when the Docker it is handed says it can.
func TestTheRestoreDockerBringsNamespaceSharersAlong(t *testing.T) {
	host := &rejoinRecorder{}
	svc := &Service{docker: host}
	r, ok := svc.restoreDocker().(backup.NamespaceRejoiner)
	if !ok {
		t.Fatal("the Docker a restore gets cannot bring a recreated container's sharers along")
	}
	if id, err := r.ContainerID(context.Background(), "gluetun"); err != nil || id != "id-gluetun" || host.asked != "gluetun" {
		t.Fatalf("ContainerID = %q, %v; want the host's answer", id, err)
	}

	plain := &Service{docker: struct{ dockercli.Docker }{}}
	if _, ok := plain.restoreDocker().(backup.NamespaceRejoiner); ok {
		t.Fatal("a Docker without the rejoin claims it")
	}
}
