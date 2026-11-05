package backup_test

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/backup"
	"github.com/junkerderprovinz/bombvault/internal/model"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

func TestRestartStartsANamespaceOwnerBeforeThoseJoiningIt(t *testing.T) {
	d := &fakeDocker{}
	failed := backup.RestartInOrder(t.Context(), d, []backup.StopContainer{
		{Name: "aaa-app", ID: "aaa-app", Joins: []string{"vpn"}},
		{Name: "bbb-app", ID: "bbb-app", Joins: []string{"aaa-app"}},
		{Name: "vpn", ID: "vpn", Service: "gluetun"},
	}, false, 0)
	if len(failed) != 0 {
		t.Fatalf("failed = %v", failed)
	}
	vpn, aaa, bbb := idxOf(d.log, "start:vpn"), idxOf(d.log, "start:aaa-app"), idxOf(d.log, "start:bbb-app")
	if vpn < 0 || aaa < vpn || bbb < aaa {
		t.Fatalf("want each owner before the container joining it: %v", d.log)
	}
}

func TestStopLevelsStopThoseJoiningANamespaceFirst(t *testing.T) {
	levels := backup.StopLevels([]backup.StopContainer{{Name: "vpn"}, {Name: "app", Joins: []string{"vpn"}}})
	if len(levels) != 2 || levels[0][0] != 1 || levels[1][0] != 0 {
		t.Fatalf("levels = %v, want the joining container first", levels)
	}
}

func backupDepsFor(d backup.Docker, runs *fakeRuns, r backup.Restic) backup.BackupDeps {
	return backup.BackupDeps{
		ContainerRef: "app", ContainerName: "app", RepoPath: "/repo", TargetID: "t", WasRunning: true,
		AppdataPaths: []string{"/host/user/appdata/app"},
		Docker:       d, Restic: r, Templates: &fakeTemplates{}, Runs: runs,
	}
}

func lastNote(t *testing.T, runs *fakeRuns) (status, note string) {
	t.Helper()
	if len(runs.finishCalls) == 0 {
		t.Fatal("no run was finished")
	}
	f := runs.finishCalls[len(runs.finishCalls)-1]
	return f.status, f.note
}

func TestABackupThatFailedStillNamesTheContainersNotBack(t *testing.T) {
	d := failingStart{fakeDocker: &fakeDocker{}, name: "db"}
	runs := &fakeRuns{}
	deps := backupDepsFor(d, runs, &fakeRestic{backupErr: errors.New("Fatal: repo locked")})
	deps.StopContainers = []backup.StopContainer{{Name: "mariadb", ID: "db", WasRunning: true}}
	var told []string
	deps.OnNotBack = func(names []string) { told = names }

	if _, err := backup.BackupContainer(t.Context(), deps); err == nil {
		t.Fatal("want the backup error")
	}
	status, note := lastNote(t, runs)
	if want := "Fatal: repo locked; " + store.NoteContainersNotBack + ": mariadb"; status != "failed" || note != want {
		t.Fatalf("finish = %q %q, want failed %q", status, note, want)
	}
	if !slices.Equal(told, []string{"mariadb"}) {
		t.Fatalf("told = %v", told)
	}
}

func TestATargetThatDoesNotComeBackIsNamedOnTheFailedRun(t *testing.T) {
	d := failingStart{fakeDocker: &fakeDocker{healthSeq: map[string][]model.Health{"app": {{Running: false}}}}, name: "app"}
	runs := &fakeRuns{}
	deps := backupDepsFor(d, runs, &fakeRestic{})
	var told []string
	deps.OnNotBack = func(names []string) { told = names }

	if _, err := backup.BackupContainer(t.Context(), deps); err == nil {
		t.Fatal("want the restart error")
	}
	if !slices.Equal(told, []string{"app"}) {
		t.Fatalf("told = %v, want the target once", told)
	}
}

func TestAContainerThatStartedAndDiedAtOnceCountsAsNotBack(t *testing.T) {
	d := &fakeDocker{healthSeq: map[string][]model.Health{"db": {{Running: false}}}}
	runs := &fakeRuns{}
	deps := backupDepsFor(d, runs, &fakeRestic{})
	deps.StopContainers = []backup.StopContainer{{Name: "mariadb", ID: "db", WasRunning: true}}

	if _, err := backup.BackupContainer(t.Context(), deps); err != nil {
		t.Fatal(err)
	}
	if status, note := lastNote(t, runs); status != "success" || note != store.NoteContainersNotBack+": mariadb" {
		t.Fatalf("finish = %q %q", status, note)
	}
}

func TestAFailedSnapshotReadStillNamesTheContainersNotBack(t *testing.T) {
	r := runSnapshotBackup(t.Context(), t, func(_ *backup.BackupDeps, snap *fakeSnapshot) {
		snap.readErr = errors.New("Fatal: repo locked")
		snap.d.healthSeq = map[string][]model.Health{"app": {{Running: false}}}
	})
	if r.err == nil {
		t.Fatal("want the read error")
	}
	if status, note := lastNote(t, r.runs); status != "failed" || note != "Fatal: repo locked; "+store.NoteContainersNotBack+": app" {
		t.Fatalf("finish = %q %q", status, note)
	}
}

func TestAContainerLeftOnADeadNetworkIsNamed(t *testing.T) {
	runs := &fakeRuns{}
	deps := backupDepsFor(&fakeDocker{}, runs, &fakeRestic{})
	deps.CutOff = []string{"bombvault"}
	var told []string
	deps.OnNotBack = func(names []string) { told = names }

	if _, err := backup.BackupContainer(t.Context(), deps); err != nil {
		t.Fatal(err)
	}
	if status, note := lastNote(t, runs); status != "success" || note != store.NoteNoNetworkUntilRestart+": bombvault" {
		t.Fatalf("finish = %q %q", status, note)
	}
	if !slices.Equal(told, []string{"bombvault"}) {
		t.Fatalf("told = %v", told)
	}
}

type stopFailsFor struct {
	*fakeDocker
	name string
}

func (d stopFailsFor) Stop(ctx context.Context, name string, timeout time.Duration) error {
	if name == d.name {
		return errors.New("cannot stop")
	}
	return d.fakeDocker.Stop(ctx, name, timeout)
}

func TestASharerThatCouldNotBeStoppedIsNamedAsCutOff(t *testing.T) {
	runs := &fakeRuns{}
	deps := backupDepsFor(stopFailsFor{&fakeDocker{}, "peer"}, runs, &fakeRestic{})
	deps.StopContainers = []backup.StopContainer{
		{Name: "peer", ID: "peer", WasRunning: true, Sharer: true},
		{Name: "db", ID: "db", WasRunning: true},
	}
	if _, err := backup.BackupContainer(t.Context(), deps); err != nil {
		t.Fatal(err)
	}
	if status, note := lastNote(t, runs); status != "success" || note != store.NoteNoNetworkUntilRestart+": peer" {
		t.Fatalf("finish = %q %q", status, note)
	}
}

func TestAnUpdateThatLeftTheTargetDownDoesNotKeepTheRunWaiting(t *testing.T) {
	d := &fakeDocker{healthSeq: map[string][]model.Health{"app": {{Running: true}}}}
	runs := &fakeRuns{}
	deps := backupDepsFor(d, runs, &fakeRestic{})
	deps.UpdateAfterBackup = func() func() []string {
		return func() []string {
			d.healthSeq["app"] = []model.Health{{Running: false}}
			return []string{"app"}
		}
	}
	if _, err := backup.BackupContainer(t.Context(), deps); err != nil {
		t.Fatal(err)
	}
	if n := countOf(d.log, "waitRunning:app"); n != 1 {
		t.Fatalf("waited %d times, want only the wait before the update: %v", n, d.log)
	}
	if status, note := lastNote(t, runs); status != "success" || note != store.NoteContainersNotBack+": app" {
		t.Fatalf("finish = %q %q", status, note)
	}
}

func TestAnUpdateAfterASnapshotReadNamesWhatItLeftDown(t *testing.T) {
	r := runSnapshotBackup(t.Context(), t, func(deps *backup.BackupDeps, _ *fakeSnapshot) {
		deps.UpdateAfterBackup = func() func() []string {
			return func() []string { return []string{"app", "peer"} }
		}
	})
	if r.err != nil {
		t.Fatal(r.err)
	}
	if n := countOf(r.docker.log, "waitRunning:app"); n != 1 {
		t.Fatalf("waited %d times, want only the wait before the update: %v", n, r.docker.log)
	}
	if status, note := lastNote(t, r.runs); status != "success" || note != store.NoteContainersNotBack+": app, peer" {
		t.Fatalf("finish = %q %q", status, note)
	}
}

// waitFailsAfter answers the first wants waits and then reports a container
// that never runs.
type waitFailsAfter struct {
	*fakeDocker
	waits *int
	wants int
}

func (d waitFailsAfter) WaitRunning(ctx context.Context, name string, timeout time.Duration) error {
	*d.waits++
	if *d.waits > d.wants {
		return errors.New("not running")
	}
	return d.fakeDocker.WaitRunning(ctx, name, timeout)
}

func TestATargetThatDiesAfterAnUpdateIsNamed(t *testing.T) {
	waits := 0
	runs := &fakeRuns{}
	deps := backupDepsFor(waitFailsAfter{&fakeDocker{}, &waits, 1}, runs, &fakeRestic{})
	deps.UpdateAfterBackup = func() func() []string { return func() []string { return nil } }
	if _, err := backup.BackupContainer(t.Context(), deps); err != nil {
		t.Fatal(err)
	}
	if status, note := lastNote(t, runs); status != "success" || note != store.NoteContainersNotBack+": app" {
		t.Fatalf("finish = %q %q", status, note)
	}
}

func TestRestoreCancelledDuringTheStopStartsTheContainerAgain(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	d := &rejoinDocker{fakeDocker: &fakeDocker{liveName: "plex", onStop: cancel}, ids: map[string]string{"plex": "old"}}
	r := &removals{d: d.fakeDocker}
	runs := &fakeRuns{}
	deps := restoreDeps(d.fakeDocker, &fakeRestic{}, &fakeTemplates{}, runs)
	deps.Docker, deps.Removals = d, r

	err := backup.RestoreContainer(ctx, deps)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want a cancel", err)
	}
	if contains(d.log, "remove:plex") || !contains(d.log, "start:plex") || !contains(d.log, "clear:plex") {
		t.Fatalf("want no removal, the container started again and its mark gone: %v", d.log)
	}
	if !contains(d.log, "rejoin:old->plex") {
		t.Fatalf("the containers sharing its network need its new one: %v", d.log)
	}
	if d.startCtxErrs[0] != nil {
		t.Fatalf("the start ran on the cancelled context: %v", d.startCtxErrs)
	}
	if status, _ := lastNote(t, runs); status != "cancelled" {
		t.Fatalf("status = %q", status)
	}
}

func TestRestoreWhoseRemoveFailsStartsTheContainerAgain(t *testing.T) {
	d := &fakeDocker{liveName: "plex", removeErr: errors.New("removal of container plex is already in progress")}
	r := &removals{d: d}
	runs := &fakeRuns{}
	deps := restoreDeps(d, &fakeRestic{}, &fakeTemplates{}, runs)
	deps.Removals = r

	if err := backup.RestoreContainer(t.Context(), deps); err == nil {
		t.Fatal("want the remove error")
	}
	if !contains(d.log, "start:plex") {
		t.Fatalf("want it started again: %v", d.log)
	}
	if status, _ := lastNote(t, runs); status != "failed" {
		t.Fatalf("status = %q", status)
	}
}

func TestRestoreLeavesAContainerThatWasStoppedAloneWhenItCannotGoOn(t *testing.T) {
	d := &fakeDocker{
		liveName:  "plex",
		removeErr: errors.New("nope"),
		healthSeq: map[string][]model.Health{"plex": {{Running: false}}},
	}
	deps := restoreDeps(d, &fakeRestic{}, &fakeTemplates{}, &fakeRuns{})
	if err := backup.RestoreContainer(t.Context(), deps); err == nil {
		t.Fatal("want the remove error")
	}
	if contains(d.log, "start:plex") {
		t.Fatalf("it was stopped before the restore and stays so: %v", d.log)
	}
}

func TestRestoreThatCouldNotStartTheContainerAgainSaysSo(t *testing.T) {
	d := &fakeDocker{liveName: "plex", removeErr: errors.New("nope"), startErr: errors.New("port taken")}
	runs := &fakeRuns{}
	deps := restoreDeps(d, &fakeRestic{}, &fakeTemplates{}, runs)
	var told []string
	deps.OnNotBack = func(names []string) { told = names }
	if err := backup.RestoreContainer(t.Context(), deps); err == nil {
		t.Fatal("want the remove error")
	}
	if !slices.Equal(told, []string{"plex"}) {
		t.Fatalf("told = %v", told)
	}
	if _, note := lastNote(t, runs); note != "restore: remove existing container: nope; "+store.NoteContainersNotBack+": plex" {
		t.Fatalf("note = %q", note)
	}
}

func TestRestoreKeepsVolumesFromInTheRecreate(t *testing.T) {
	d := &fakeDocker{liveName: "plex"}
	deps := restoreDeps(d, &fakeRestic{}, &fakeTemplates{}, &fakeRuns{})
	deps.Inspect.HostConfig.VolumesFrom = []string{"vfsrc:ro"}
	if err := backup.RestoreContainer(t.Context(), deps); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(d.createdInspect.HostConfig.VolumesFrom, []string{"vfsrc:ro"}) {
		t.Fatalf("VolumesFrom = %v", d.createdInspect.HostConfig.VolumesFrom)
	}
}

func TestRestoreRefusesVolumesFromAContainerThatIsGone(t *testing.T) {
	d := &fakeDocker{liveName: "plex", absent: map[string]bool{"vfsrc": true}}
	deps := restoreDeps(d, &fakeRestic{}, &fakeTemplates{}, &fakeRuns{})
	deps.Inspect.HostConfig.VolumesFrom = []string{"vfsrc:ro"}
	if err := backup.RestoreContainer(t.Context(), deps); err == nil {
		t.Fatal("want a refusal")
	}
	if contains(d.log, "stop:plex") || contains(d.log, "remove:plex") {
		t.Fatalf("the refusal comes before anything is touched: %v", d.log)
	}
}

func TestACopyBesideLeavesOutVolumesFromTheOriginals(t *testing.T) {
	d := &fakeDocker{}
	deps := besideDeps(d, &fakeRestic{}, &fakeRuns{})
	deps.Copy.HostConfig.VolumesFrom = []string{"vfsrc"}
	if err := backup.RestoreContainerBeside(t.Context(), deps); err != nil {
		t.Fatal(err)
	}
	if len(d.createdInspect.HostConfig.VolumesFrom) != 0 {
		t.Fatalf("the copy must not reach the live data: %v", d.createdInspect.HostConfig.VolumesFrom)
	}
}
