package backup_test

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/backup"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// rejoinDocker is a Docker that recreates the containers sharing a namespace,
// the way dockercli does. ids answers ContainerID by name.
type rejoinDocker struct {
	*fakeDocker
	ids      map[string]string
	notBack  []string
	rejoined []string
	// onRejoin runs when a rejoin is asked for, standing in for its recreates.
	onRejoin func()
}

func (d *rejoinDocker) ContainerID(_ context.Context, name string) (string, error) {
	d.log = append(d.log, "containerID:"+name)
	return d.ids[name], nil
}

func (d *rejoinDocker) RejoinNamespace(_ context.Context, oldID, parent string) ([]string, error) {
	d.log = append(d.log, "rejoin:"+oldID+"->"+parent)
	d.rejoined = append(d.rejoined, oldID)
	if d.onRejoin != nil {
		d.onRejoin()
	}
	return d.notBack, nil
}

// failingStart fails the start of one container only.
type failingStart struct {
	*fakeDocker
	name string
}

func (d failingStart) Start(ctx context.Context, name string) error {
	if name == d.name {
		d.log = append(d.log, "start:"+name)
		return errors.New("joining network namespace of container: No such container")
	}
	return d.fakeDocker.Start(ctx, name)
}

func TestRestoreRefusesASharedNetworkWhoseContainerIsGone(t *testing.T) {
	gone := strings.Repeat("ab", 32)
	d := &fakeDocker{liveName: "plex", absent: map[string]bool{gone: true}}
	deps := restoreDeps(d, &fakeRestic{}, &fakeTemplates{}, &fakeRuns{})
	deps.Inspect.HostConfig.NetworkMode = "container:" + gone

	err := backup.RestoreContainer(t.Context(), deps)
	var refusal *backup.NamespaceGoneError
	if !errors.As(err, &refusal) || refusal.Container != gone {
		t.Fatalf("err = %v, want a refusal naming the missing container", err)
	}
	if contains(d.log, "stop:") || contains(d.log, "remove:") {
		t.Fatalf("nothing may be stopped or removed: %v", d.log)
	}
}

func TestRestoreBringsTheContainersSharingItsNetworkOntoTheNewOne(t *testing.T) {
	d := &rejoinDocker{fakeDocker: &fakeDocker{liveName: "plex"}, ids: map[string]string{"plex": "old-plex-id"}, notBack: []string{"qbittorrent"}}
	runs := &fakeRuns{}
	if err := backup.RestoreContainer(t.Context(), restoreDeps(d.fakeDocker, &fakeRestic{}, &fakeTemplates{}, runs)); err != nil {
		t.Fatal(err)
	}
	if len(d.rejoined) != 0 {
		t.Fatal("a Docker that cannot rejoin is not asked to")
	}

	d.log = nil
	deps := restoreDeps(d.fakeDocker, &fakeRestic{}, &fakeTemplates{}, runs)
	deps.Docker = d
	if err := backup.RestoreContainer(t.Context(), deps); err != nil {
		t.Fatal(err)
	}
	id, stop := slices.Index(d.log, "containerID:plex"), slices.Index(d.log, "stop:plex")
	create, rejoin := slices.Index(d.log, "createAndStart:/plex"), slices.Index(d.log, "rejoin:old-plex-id->plex")
	if id < 0 || id > stop || create < 0 || rejoin < create {
		t.Fatalf("the old ID is read before the stop and the sharers rejoin after the recreate: %v", d.log)
	}
	note := runs.finishCalls[len(runs.finishCalls)-1].note
	if note != store.NoteContainersNotBack+": qbittorrent" {
		t.Fatalf("note = %q, want the container that did not come back", note)
	}
}

// After the update recreated the target, and with it the containers sharing
// its network, those are started under their new IDs.
func TestUpdateAfterBackupStartsTheRecreatedSharersByTheirNewIDs(t *testing.T) {
	defer backup.SetHealthTimingForTest(time.Millisecond, time.Millisecond)()
	d := &rejoinDocker{fakeDocker: &fakeDocker{}, ids: map[string]string{}}
	recreate := func() {
		_ = d.CreateAndStart(t.Context(), sampleInspect(), true)
		d.ids["qbittorrent"] = "new-qbit"
	}
	deps := []backup.StopContainer{{Name: "qbittorrent", ID: "old-qbit", WasRunning: true}}
	runBackupWithHook(t, d.fakeDocker, deps, recreate, false, time.Second)
	if !contains(d.log, "start:old-qbit") {
		t.Fatalf("without a rejoining Docker the old ID is all there is: %v", d.log)
	}

	d.log = nil
	runs := &fakeRuns{}
	_, err := backup.BackupContainer(t.Context(), backup.BackupDeps{
		ContainerRef: "gluetun", ContainerName: "gluetun", RepoPath: "/repo", TargetID: "t",
		AppdataPaths: []string{"/host/user/appdata/gluetun"}, WasRunning: true,
		StopContainers:    []backup.StopContainer{{Name: "qbittorrent", ID: "old-qbit", WasRunning: true}},
		UpdateAfterBackup: updateFound(recreate),
		Docker:            d, Restic: &fakeRestic{}, Templates: &fakeTemplates{}, Runs: runs,
	})
	if err != nil {
		t.Fatal(err)
	}
	if contains(d.log, "start:old-qbit") || !contains(d.log, "start:new-qbit") {
		t.Fatalf("the sharer must be started by its new ID: %v", d.log)
	}
}

// A container the target's compose file depends on, its database, starts and
// becomes ready before the target; the others still start after it.
func TestADependencyOfTheTargetStartsBeforeIt(t *testing.T) {
	defer backup.SetHealthTimingForTest(time.Millisecond, time.Millisecond)()
	d := &fakeDocker{}
	deps := []backup.StopContainer{
		{Name: "nextcloud-cron", ID: "cron", WasRunning: true},
		{Name: "mariadb", ID: "db", WasRunning: true, BeforeTarget: true},
	}
	runBackupWithHook(t, d, deps, nil, true, time.Second)

	db, health, target, cron := idxOf(d.log, "start:db"), idxOf(d.log, "health:db"), idxOf(d.log, "start:backuptarget"), idxOf(d.log, "start:cron")
	if db < 0 || health < db || target < health || cron < target {
		t.Fatalf("want the database started and ready before the target, the rest after it: %v", d.log)
	}
}

func TestAContainerThatDoesNotStartAgainIsNotedOnTheBackup(t *testing.T) {
	d := failingStart{fakeDocker: &fakeDocker{}, name: "qbit"}
	runs := &fakeRuns{}
	_, err := backup.BackupContainer(t.Context(), backup.BackupDeps{
		ContainerRef: "gluetun", ContainerName: "gluetun", RepoPath: "/repo", TargetID: "t",
		AppdataPaths: []string{"/host/user/appdata/gluetun"}, WasRunning: true,
		StopContainers: []backup.StopContainer{{Name: "qbittorrent", ID: "qbit", WasRunning: true}},
		Docker:         d, Restic: &fakeRestic{}, Templates: &fakeTemplates{}, Runs: runs,
	})
	if err != nil {
		t.Fatal(err)
	}
	f := runs.finishCalls[len(runs.finishCalls)-1]
	if f.status != "success" || f.note != store.NoteContainersNotBack+": qbittorrent" {
		t.Fatalf("finish = %+v, want a success that names the container still down", f)
	}
}
