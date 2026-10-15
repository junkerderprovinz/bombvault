package backup_test

import (
	"slices"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/backup"
	"github.com/junkerderprovinz/bombvault/internal/model"
)

// fakeMarker keeps the record in the Docker fake's log, so a test sees it in
// order with the stops and starts.
type fakeMarker struct {
	d      *fakeDocker
	marked map[string]bool
}

func (m *fakeMarker) Mark(names []string) error {
	for _, n := range names {
		m.d.log = append(m.d.log, "mark:"+n)
		m.marked[n] = true
	}
	return nil
}

func (m *fakeMarker) Clear(names []string) error {
	for _, n := range names {
		m.d.log = append(m.d.log, "clear:"+n)
		delete(m.marked, n)
	}
	return nil
}

func markerDeps(d *fakeDocker, m *fakeMarker) backup.BackupDeps {
	return backup.BackupDeps{
		ContainerRef:         "app",
		ContainerName:        "app",
		RepoPath:             "/repo",
		AppdataPaths:         []string{"/host/user/user/appdata/app"},
		TargetID:             "target-1",
		WasRunning:           true,
		StopContainers:       []backup.StopContainer{{Name: "db", WasRunning: true}, {Name: "cache"}},
		SnapshotTemplatesDir: "/data/templates",
		FlashTemplatesDir:    "/boot/templates",
		Restarts:             m,
		Docker:               d,
		Restic:               &fakeRestic{summary: backup.Summary{SnapshotID: "deadbeef12345678"}},
		Templates:            &fakeTemplates{},
		Runs:                 &fakeRuns{},
	}
}

func TestBackupRecordsTheStoppedContainersUntilTheyRunAgain(t *testing.T) {
	for _, mode := range []string{"live", "snapshot"} {
		t.Run(mode, func(t *testing.T) {
			d := &fakeDocker{}
			m := &fakeMarker{d: d, marked: map[string]bool{}}
			deps := markerDeps(d, m)
			if mode == "snapshot" {
				deps.Snapshot = &fakeSnapshot{d: d}
			}
			if _, err := backup.BackupContainer(t.Context(), deps); err != nil {
				t.Fatal(err)
			}
			for _, pair := range [][2]string{{"mark:app", "stop:app"}, {"mark:db", "stop:db"}, {"start:db", "clear:db"}, {"start:app", "clear:app"}} {
				if !before(d.log, pair[0], pair[1]) {
					t.Errorf("%s has to come before %s: %v", pair[0], pair[1], d.log)
				}
			}
			if slices.Contains(d.log, "mark:cache") {
				t.Errorf("a container that was already stopped was recorded: %v", d.log)
			}
			if len(m.marked) != 0 {
				t.Errorf("still recorded after the backup: %v", m.marked)
			}
		})
	}
}

func TestBackupKeepsTheRecordOfAContainerThatDidNotComeBack(t *testing.T) {
	for _, mode := range []string{"live", "snapshot"} {
		t.Run(mode, func(t *testing.T) {
			d := &fakeDocker{healthSeq: map[string][]model.Health{"db": {{Running: false}}}}
			m := &fakeMarker{d: d, marked: map[string]bool{}}
			deps := markerDeps(d, m)
			if mode == "snapshot" {
				deps.Snapshot = &fakeSnapshot{d: d}
			}
			_, _ = backup.BackupContainer(t.Context(), deps)
			if !m.marked["db"] || m.marked["app"] {
				t.Errorf("recorded %v, want db alone, which never ran again", m.marked)
			}
		})
	}
}

func TestSnapshotUpdateRecordsTheDependentsItStopsAgain(t *testing.T) {
	d := &fakeDocker{}
	m := &fakeMarker{d: d, marked: map[string]bool{}}
	deps := markerDeps(d, m)
	deps.Snapshot = &fakeSnapshot{d: d}
	deps.UpdateAfterBackup = func() func() { return func() {} }
	deps.Clock = func() time.Time { return time.Unix(0, 0) }
	if _, err := backup.BackupContainer(t.Context(), deps); err != nil {
		t.Fatal(err)
	}
	if countOf(d.log, "mark:db") != 2 || countOf(d.log, "clear:db") != 2 || len(m.marked) != 0 {
		t.Errorf("db has to be recorded for both stops and cleared after both: %v", d.log)
	}
}
