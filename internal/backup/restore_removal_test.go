package backup_test

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/backup"
	"github.com/junkerderprovinz/bombvault/internal/model"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// removals records what a restore marks as removed, in the Docker log it
// shares, so a test sees the mark next to the stop and the create.
type removals struct {
	d      *fakeDocker
	marked map[string]string
}

func (r *removals) Mark(name string, _ model.Inspect, oldID string) error {
	r.d.log = append(r.d.log, "mark:"+name)
	if r.marked == nil {
		r.marked = map[string]string{}
	}
	r.marked[name] = oldID
	return nil
}

func (r *removals) Clear(name string) error {
	r.d.log = append(r.d.log, "clear:"+name)
	delete(r.marked, name)
	return nil
}

// createFailsOnce refuses the first create, like a cancel that lands in it.
type createFailsOnce struct {
	*rejoinDocker
	failed bool
}

func (d *createFailsOnce) CreateAndStart(ctx context.Context, in model.Inspect, start bool) error {
	if !d.failed {
		d.failed = true
		d.log = append(d.log, "createFailed:"+in.Name)
		return context.Canceled
	}
	return d.fakeDocker.CreateAndStart(ctx, in, start)
}

func TestARestoreRemembersTheContainerItRemovedUntilItIsBack(t *testing.T) {
	d := &fakeDocker{liveName: "/plex"}
	deps := restoreDeps(d, &fakeRestic{}, &fakeTemplates{}, &fakeRuns{})
	marks := &removals{d: d}
	deps.Removals = marks
	if err := backup.RestoreContainer(t.Context(), deps); err != nil {
		t.Fatal(err)
	}
	mark, stop := slices.Index(d.log, "mark:plex"), slices.Index(d.log, "stop:plex")
	create, clear := slices.Index(d.log, "createAndStart:/plex"), slices.Index(d.log, "clear:plex")
	if mark < 0 || mark > stop || clear < create {
		t.Fatalf("the removal is recorded before the stop and forgotten once the container is back: %v", d.log)
	}
	if len(marks.marked) != 0 {
		t.Fatalf("still recorded: %v", marks.marked)
	}
}

func TestARestoreThatCannotPutTheContainerBackKeepsTheRecord(t *testing.T) {
	d := &fakeDocker{liveName: "/plex", createErr: errors.New("no space left on device")}
	deps := restoreDeps(d, &fakeRestic{restoreErr: errors.New("connection reset by peer")}, &fakeTemplates{}, &fakeRuns{})
	marks := &removals{d: d}
	deps.Removals = marks
	if err := backup.RestoreContainer(t.Context(), deps); err == nil {
		t.Fatal("the restore reports success")
	}
	if _, ok := marks.marked["plex"]; !ok {
		t.Fatalf("the container is gone and nothing remembers it: %v", d.log)
	}
}

func TestARestoreThatStopsPartwayBringsTheContainersSharingItsNetworkAlong(t *testing.T) {
	d := &rejoinDocker{fakeDocker: &fakeDocker{liveName: "plex"}, ids: map[string]string{"plex": "old-plex-id"}}
	deps := restoreDeps(d.fakeDocker, &fakeRestic{restoreErr: errors.New("connection reset by peer")}, &fakeTemplates{}, &fakeRuns{})
	deps.Docker = d
	err := backup.RestoreContainer(t.Context(), deps)
	if err == nil || !strings.HasPrefix(err.Error(), store.ReasonRestoreHalfDone+": ") {
		t.Fatalf("err = %v, want a half-done restore", err)
	}
	create, rejoin := slices.Index(d.log, "createAndStart:/plex"), slices.Index(d.log, "rejoin:old-plex-id->plex")
	if create < 0 || rejoin < create {
		t.Fatalf("the sharers have to follow the container created again: %v", d.log)
	}
}

func TestARestoreWhoseLastCreateIsCutOffPutsTheContainerBack(t *testing.T) {
	d := &createFailsOnce{rejoinDocker: &rejoinDocker{fakeDocker: &fakeDocker{}, ids: map[string]string{"plex": "old-plex-id"}}}
	deps := restoreDeps(d.fakeDocker, &fakeRestic{}, &fakeTemplates{}, &fakeRuns{})
	deps.Docker = d
	marks := &removals{d: d.fakeDocker}
	deps.Removals = marks
	err := backup.RestoreContainer(t.Context(), deps)
	if err == nil || !strings.HasPrefix(err.Error(), store.ReasonRestoreHalfDone+": ") {
		t.Fatalf("err = %v, want the container put back", err)
	}
	if !contains(d.log, "createAndStart:/plex") || d.createdStart || !contains(d.log, "rejoin:old-plex-id->plex") {
		t.Fatalf("the container must be created again, stopped, with its sharers: %v", d.log)
	}
	if len(marks.marked) != 0 {
		t.Fatalf("the container is back and still recorded as removed: %v", marks.marked)
	}
}
