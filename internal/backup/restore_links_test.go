package backup_test

import (
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/backup"
	"github.com/junkerderprovinz/bombvault/internal/model"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

func linkedDeps(d *fakeDocker, runs *fakeRuns) backup.RestoreDeps {
	deps := restoreDeps(d, &fakeRestic{}, &fakeTemplates{}, runs)
	deps.Inspect.Running = true
	deps.Inspect.HostConfig.Links = []string{"/db:/plex/db", "/cache:/plex/cache"}
	return deps
}

func TestRestoreKeepsALinkToARunningContainer(t *testing.T) {
	d := &fakeDocker{liveName: "/plex"}
	runs := &fakeRuns{}
	if err := backup.RestoreContainer(t.Context(), linkedDeps(d, runs)); err != nil {
		t.Fatal(err)
	}
	if got := d.createdInspect.HostConfig.Links; len(got) != 2 {
		t.Fatalf("links = %v, want both", got)
	}
	if runs.finishCalls[0].note != "" {
		t.Fatalf("note = %q, want none", runs.finishCalls[0].note)
	}
}

// Docker refuses to create a container linked to one that does not exist, and
// to start one linked to a stopped container. The restore goes through
// without those links and says so.
func TestRestoreDropsLinksDockerWouldRefuse(t *testing.T) {
	d := &fakeDocker{
		liveName:  "/plex",
		absent:    map[string]bool{"db": true},
		healthSeq: map[string][]model.Health{"cache": {{Running: false}}},
	}
	runs := &fakeRuns{}
	deps := linkedDeps(d, runs)
	if err := backup.RestoreContainer(t.Context(), deps); err != nil {
		t.Fatal(err)
	}
	if got := d.createdInspect.HostConfig.Links; len(got) != 0 {
		t.Fatalf("links = %v, want none", got)
	}
	want := store.NoteRestoredWithoutLinks + ": db, cache"
	if runs.finishCalls[0].status != "success" || runs.finishCalls[0].note != want {
		t.Fatalf("run = %+v, want a success noting %q", runs.finishCalls[0], want)
	}
	if len(deps.Inspect.HostConfig.Links) != 2 {
		t.Fatal("the stored recipe itself must stay untouched")
	}
}

// A container recreated stopped can keep a link to a stopped container: the
// link only has to resolve when both are started.
func TestRestoreLeftStoppedKeepsALinkToAStoppedContainer(t *testing.T) {
	d := &fakeDocker{liveName: "/plex", healthSeq: map[string][]model.Health{"db": {{Running: false}}, "cache": {{Running: false}}}}
	runs := &fakeRuns{}
	deps := linkedDeps(d, runs)
	deps.LeaveStopped = true
	if err := backup.RestoreContainer(t.Context(), deps); err != nil {
		t.Fatal(err)
	}
	if got := d.createdInspect.HostConfig.Links; len(got) != 2 {
		t.Fatalf("links = %v, want both", got)
	}
}

func TestRestoreNotesBothTheGPUAndTheLinks(t *testing.T) {
	d := &fakeDocker{liveName: "/plex", absent: map[string]bool{"db": true}}
	runs := &fakeRuns{}
	deps := linkedDeps(d, runs)
	deps.Inspect.HostConfig.Links = []string{"db"}
	deps.Inspect.HostConfig.Runtime = "nvidia"
	deps.WithoutRuntime = true
	if err := backup.RestoreContainer(t.Context(), deps); err != nil {
		t.Fatal(err)
	}
	want := store.NoteRestoredWithoutRuntime + "; " + store.NoteRestoredWithoutLinks + ": db"
	if runs.finishCalls[0].note != want {
		t.Fatalf("note = %q, want %q", runs.finishCalls[0].note, want)
	}
}
