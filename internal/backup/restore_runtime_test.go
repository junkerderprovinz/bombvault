package backup_test

import (
	"errors"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/backup"
	"github.com/junkerderprovinz/bombvault/internal/model"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

func gpuInspect() model.Inspect {
	in := sampleInspect()
	in.HostConfig.Runtime = "nvidia"
	in.HostConfig.DeviceRequests = []model.DeviceRequest{{Count: -1, Capabilities: [][]string{{"gpu"}}}}
	in.HostConfig.Memory = 4 << 30
	return in
}

func TestRestoreSaysTheHostLacksTheRuntime(t *testing.T) {
	refusal := errors.New("Error response from daemon: unknown or invalid runtime name: nvidia")
	d := &fakeDocker{liveName: "/plex", createErr: &model.MissingRuntimeError{Err: refusal}}
	runs := &fakeRuns{}
	deps := restoreDeps(d, &fakeRestic{}, &fakeTemplates{}, runs)
	deps.Inspect = gpuInspect()

	err := backup.RestoreContainer(t.Context(), deps)
	var missing *model.MissingRuntimeError
	if !errors.As(err, &missing) {
		t.Fatalf("err = %v, want the missing runtime", err)
	}
	want := store.ReasonRestoreNoRuntime + ": " + refusal.Error()
	if len(runs.finishCalls) != 1 || runs.finishCalls[0].status != "failed" || runs.finishCalls[0].note != want {
		t.Fatalf("run = %+v, want failed with %q", runs.finishCalls, want)
	}
}

func TestRestoreKeepsTheRuntimeUnlessAskedNotTo(t *testing.T) {
	d := &fakeDocker{liveName: "/plex"}
	runs := &fakeRuns{}
	deps := restoreDeps(d, &fakeRestic{}, &fakeTemplates{}, runs)
	deps.Inspect = gpuInspect()
	if err := backup.RestoreContainer(t.Context(), deps); err != nil {
		t.Fatal(err)
	}
	if d.createdInspect.HostConfig.Runtime != "nvidia" || len(d.createdInspect.HostConfig.DeviceRequests) != 1 {
		t.Fatalf("host config = %+v", d.createdInspect.HostConfig)
	}
	if runs.finishCalls[0].note != "" {
		t.Fatalf("note = %q, want none", runs.finishCalls[0].note)
	}
}

func TestRestoreWithoutRuntimeDropsTheGPUAndKeepsTheRest(t *testing.T) {
	d := &fakeDocker{liveName: "/plex"}
	runs := &fakeRuns{}
	deps := restoreDeps(d, &fakeRestic{}, &fakeTemplates{}, runs)
	deps.Inspect = gpuInspect()
	deps.WithoutRuntime = true
	if err := backup.RestoreContainer(t.Context(), deps); err != nil {
		t.Fatal(err)
	}
	hc := d.createdInspect.HostConfig
	if hc.Runtime != "" || hc.DeviceRequests != nil {
		t.Fatalf("runtime = %q, devices = %v, want neither", hc.Runtime, hc.DeviceRequests)
	}
	if hc.Memory != 4<<30 || len(hc.CapAdd) != 1 || !hc.ReadonlyRootfs {
		t.Fatalf("the rest of the recipe changed: %+v", hc)
	}
	if deps.Inspect.HostConfig.Runtime != "nvidia" {
		t.Fatal("the stored recipe itself must stay untouched")
	}
	if runs.finishCalls[0].status != "success" || runs.finishCalls[0].note != store.NoteRestoredWithoutRuntime {
		t.Fatalf("run = %+v, want a success that says so", runs.finishCalls[0])
	}
}

// A container that asked for no GPU and only the default runtime lost nothing,
// so its run carries no note.
func TestRestoreWithoutRuntimeOfAPlainContainerSaysNothing(t *testing.T) {
	d := &fakeDocker{liveName: "/plex"}
	runs := &fakeRuns{}
	deps := restoreDeps(d, &fakeRestic{}, &fakeTemplates{}, runs)
	deps.Inspect.HostConfig.Runtime = "runc"
	deps.WithoutRuntime = true
	if err := backup.RestoreContainer(t.Context(), deps); err != nil {
		t.Fatal(err)
	}
	if runs.finishCalls[0].note != "" {
		t.Fatalf("note = %q", runs.finishCalls[0].note)
	}
}
