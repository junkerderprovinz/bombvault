package api

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/dockercli"
	"github.com/junkerderprovinz/bombvault/internal/model"
	"github.com/junkerderprovinz/bombvault/internal/store"
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

// startingRemovalDocker takes a start, which a container an update removed
// gets at boot.
type startingRemovalDocker struct {
	removalDocker
	createdStarted []string
	started        []string
}

func (d *startingRemovalDocker) CreateAndStart(_ context.Context, in model.Inspect, start bool) error {
	if d.createErr != nil {
		return d.createErr
	}
	if start {
		d.createdStarted = append(d.createdStarted, in.Name)
	}
	return nil
}

func (d *startingRemovalDocker) Start(_ context.Context, name string) error {
	d.started = append(d.started, name)
	return nil
}

func TestABootAfterACutOffUpdateCreatesTheContainerAndStartsIt(t *testing.T) {
	dock := &startingRemovalDocker{}
	s, st := restartFixture(t, newZFSFakeDocker())
	s.docker = dock
	messages := zfsCaptureNotifications(t, s)
	if err := st.MarkRestoreRemoval(store.RestoreRemoval{
		Name: "plex", Definition: `{"Name":"/plex"}`, OldID: "old-plex", At: 1, Update: true,
	}); err != nil {
		t.Fatal(err)
	}

	s.RecoverInterruptedRestores(context.Background())

	if !slices.Equal(dock.createdStarted, []string{"/plex"}) || !slices.Equal(dock.rejoined, []string{"old-plex->plex"}) {
		t.Fatalf("started %v, rejoined %v", dock.createdStarted, dock.rejoined)
	}
	if !zfsAnyMessageContains(messages(), "update") || !zfsAnyMessageContains(messages(), "plex") {
		t.Fatalf("notifications = %v, want the update and plex named", messages())
	}
	if zfsAnyMessageContains(messages(), "restore them") {
		t.Fatalf("notifications = %v, an update leaves the data alone", messages())
	}
	if left, _ := st.ListRestoreRemovals(); len(left) != 0 {
		t.Fatalf("still recorded: %v", left)
	}
}

// Cut off before its remove, the update left the container there but stopped.
func TestABootAfterAnUpdateCutOffBeforeItsRemoveStartsTheContainer(t *testing.T) {
	dock := &startingRemovalDocker{removalDocker: removalDocker{live: map[string]bool{"plex": true}}}
	s, st := restartFixture(t, newZFSFakeDocker())
	s.docker = dock
	if err := st.MarkRestoreRemoval(store.RestoreRemoval{
		Name: "plex", Definition: `{"Name":"/plex"}`, OldID: "old-plex", At: 1, Update: true,
	}); err != nil {
		t.Fatal(err)
	}

	s.RecoverInterruptedRestores(context.Background())

	if !slices.Equal(dock.started, []string{"plex"}) || len(dock.createdStarted) != 0 {
		t.Fatalf("started %v, created %v", dock.started, dock.createdStarted)
	}
}

func TestContainersNotBackAreNotifiedByName(t *testing.T) {
	s, _ := restartFixture(t, newZFSFakeDocker())
	messages := zfsCaptureNotifications(t, s)

	s.notifyContainersNotBack(context.Background(), `the backup of "app"`, []string{"db", "peer"})

	if !zfsAnyMessageContains(messages(), `these containers did not come back: db, peer`) {
		t.Fatalf("notifications = %v", messages())
	}
}
