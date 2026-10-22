package api

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/dockercli"
	"github.com/junkerderprovinz/bombvault/internal/model"
)

// removalDocker knows the containers in live and creates the others.
type removalDocker struct {
	dockercli.Docker
	live      map[string]bool
	createErr error
	created   []string
	rejoined  []string
}

func (d *removalDocker) InspectName(_ context.Context, name string) (string, error) {
	if d.live[name] {
		return name, nil
	}
	return "", nil
}

func (d *removalDocker) CreateAndStart(_ context.Context, in model.Inspect, start bool) error {
	if d.createErr != nil {
		return d.createErr
	}
	if start {
		return errors.New("a container put back after a cut-off restore must stay stopped")
	}
	d.created = append(d.created, in.Name)
	return nil
}

func (d *removalDocker) ContainerID(context.Context, string) (string, error) { return "", nil }

func (d *removalDocker) RejoinNamespace(_ context.Context, oldID, parent string) ([]string, error) {
	d.rejoined = append(d.rejoined, oldID+"->"+parent)
	return nil, nil
}

func TestABootAfterACutOffRestoreCreatesTheRemovedContainerAgain(t *testing.T) {
	dock := &removalDocker{live: map[string]bool{"sonarr": true}}
	s, st := restartFixture(t, newZFSFakeDocker())
	s.docker = dock
	messages := zfsCaptureNotifications(t, s)
	marks := restoreRemovals{st: st}
	if err := marks.Mark("gluetun", model.Inspect{Name: "/gluetun"}, "old-gluetun"); err != nil {
		t.Fatal(err)
	}
	if err := marks.Mark("sonarr", model.Inspect{Name: "/sonarr"}, ""); err != nil {
		t.Fatal(err)
	}

	s.RecoverInterruptedRestores(context.Background())

	if !slices.Equal(dock.created, []string{"/gluetun"}) || !slices.Equal(dock.rejoined, []string{"old-gluetun->gluetun"}) {
		t.Fatalf("created %v, rejoined %v; want gluetun back with its sharers and sonarr left as it is", dock.created, dock.rejoined)
	}
	if left, _ := st.ListRestoreRemovals(); len(left) != 0 {
		t.Fatalf("still recorded: %v", left)
	}
	if !zfsAnyMessageContains(messages(), "restore them once more: gluetun") {
		t.Fatalf("notifications = %v, want gluetun named", messages())
	}
}

func TestABootTriesOnceToCreateAContainerACutOffRestoreRemoved(t *testing.T) {
	dock := &removalDocker{createErr: errors.New("port is already allocated")}
	s, st := restartFixture(t, newZFSFakeDocker())
	s.docker = dock
	messages := zfsCaptureNotifications(t, s)
	if err := (restoreRemovals{st: st}).Mark("plex", model.Inspect{Name: "/plex"}, ""); err != nil {
		t.Fatal(err)
	}

	s.RecoverInterruptedRestores(context.Background())

	if left, _ := st.ListRestoreRemovals(); len(left) != 0 {
		t.Fatalf("recorded %v, want nothing kept for the next boot", left)
	}
	if !zfsAnyMessageContains(messages(), "could not be created again: plex") {
		t.Fatalf("notifications = %v, want plex named", messages())
	}
}
