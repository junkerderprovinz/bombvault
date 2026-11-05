//go:build flows

package flows

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

// age moves everything an install remembers about time into the past, with the
// server stopped: there is no clock to turn, and an overdue state takes days.
func (b *bombvault) age(by time.Duration) {
	b.t.Helper()
	b.stop()
	db, err := store.Open(filepath.Join(b.data, "bombvault.sqlite"))
	if err != nil {
		b.t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	secs := int64(by / time.Second)
	stmts := []string{
		"UPDATE runs SET started_at = started_at - $1, finished_at = finished_at - $1",
		"UPDATE file_sets SET created_at = created_at - $1",
		"UPDATE schema_migrations SET applied_at = applied_at - $1",
		"UPDATE offsite_targets SET created_at = created_at - $1",
	}
	// A database the release before wrote has no such table yet.
	var plans int
	if err := db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE name = 'section_plans'").Scan(&plans); err != nil {
		b.t.Fatal(err)
	}
	if plans > 0 {
		stmts = append(stmts, "UPDATE section_plans SET changed_at = MAX(changed_at - $1, 0)")
	}
	for _, stmt := range stmts {
		if _, err := db.Exec(stmt, secs); err != nil {
			b.t.Fatalf("%s: %v", stmt, err)
		}
	}
	b.start(currentBinary)
}

func (b *bombvault) wantStatus(domain, want, what string) {
	b.t.Helper()
	if got := b.statusOf(domain); got.Status != want {
		b.t.Errorf("%s: %s reads %q, want %q (%+v)", domain, what, got.Status, want, got)
	}
}

func TestFirstRunStatusFollowsTheSection(t *testing.T) {
	t.Parallel()
	for _, st := range starts {
		t.Run(st.name, func(t *testing.T) {
			t.Parallel()
			for _, pl := range []place{typedPath, namedOwnFolder} {
				t.Run(pl.name, func(t *testing.T) {
					t.Parallel()
					f := newFlow(t)
					b := f.b
					st.boot(f, func() {
						pl.prepare(b, "files")
						b.write(photosPath, "a.txt", "first\n", 0o640)
					})
					b.wantStatus("files", "off", "a section with no schedule, Not scheduled,")

					id := b.createSet("photos", photosPath)
					b.wantStatus("files", "off", "a section with a set and no schedule, Not scheduled,")

					b.changeSettings(func(s map[string]any) { s["filesSchedule"] = "daily 03:00" })
					b.wantStatus("files", "never", "a scheduled section before its first backup, No backup yet,")

					b.age(25 * time.Hour)
					b.wantStatus("files", "overdue", "a section whose first scheduled run never came")

					b.call("POST", "/api/files/sets/"+id+"/backup", nil, nil)
					b.settle()
					b.wantStatus("files", "ok", "a section just backed up")

					b.age(25 * time.Hour)
					b.wantStatus("files", "warn", "a day-old backup past its period")

					b.age(48 * time.Hour)
					b.wantStatus("files", "overdue", "a backup more than two periods old")

					b.changeSettings(func(s map[string]any) { s["filesSchedule"] = "off" })
					b.wantStatus("files", "off", "the schedule taken away, Not scheduled,")
				})
			}
		})
	}
}

func TestFailedBackupIsRecordedAndCountsForNothing(t *testing.T) {
	setsCell(t, restorePlaces, func(f *flow, sets folderSets) {
		f.nextRound(sets, 1)
		before := f.b.statusOf("files").LastSuccess
		held := len(f.b.snapshotsIn(f.own.repo, photosTag))
		runs := len(f.b.runsOf("backup"))

		if err := os.RemoveAll(f.b.hostPath(photosPath)); err != nil {
			f.t.Fatal(err)
		}
		f.b.call("POST", "/api/files/sets/"+sets.photos+"/backup", nil, nil)
		f.b.settle()

		list := f.b.runsOf("backup")
		if len(list) != runs+1 || list[0].Status != "failed" || list[0].Error == "" {
			f.errorf("the backup of a folder that is gone is recorded as %+v", list[:min(len(list), 2)])
		}
		if got := len(f.b.snapshotsIn(f.own.repo, photosTag)); got != held {
			f.errorf("%d snapshots of photos after a failed backup, had %d", got, held)
		}
		if got := f.b.statusOf("files").LastSuccess; got != before {
			f.errorf("the last success moved from %d to %d with a failed backup", before, got)
		}
	})
}

// A switched-on section with nothing in it to back up is no reason for a red
// overview.
func TestEmptySectionsDoNotCountAsOverdue(t *testing.T) {
	t.Parallel()
	for _, domain := range []string{"zfs", "files"} {
		t.Run(domain, func(t *testing.T) {
			t.Parallel()
			b := newBombvault(t)
			b.start(currentBinary)
			b.changeSettings(func(s map[string]any) {
				s[domain+"Enabled"] = true
				s[domain+"Schedule"] = "daily 03:00"
			})
			b.age(72 * time.Hour)
			got := b.statusOf(domain)
			if got.Status == "overdue" || got.Status == "never" {
				b.t.Errorf("%s holds nothing to back up and reads %q: %+v", domain, got.Status, got)
			}
		})
	}
}

func TestUpdateKeepsTheStatusOfBackupsAlreadyMade(t *testing.T) {
	t.Parallel()
	f := newFlow(t)
	var sets folderSets
	var made int64
	starts[1].boot(f, func() {
		f.own = typedPath.prepare(f.b, "files")
		f.b.write(photosPath, "a.txt", "first\n", 0o640)
		f.b.write(docsPath, "c.txt", "first\n", 0o644)
		sets = folderSets{f.b.createSet("photos", photosPath), f.b.createSet("docs", docsPath)}
		f.b.changeSettings(func(s map[string]any) { s["filesSchedule"] = "daily 03:00" })
		f.backupAll(sets)
		made = f.b.statusOf("files").LastSuccess
	})
	got := f.b.statusOf("files")
	if got.Status != "ok" || got.LastSuccess != made {
		t.Errorf("after the update the status is %+v, before it the last success was %d", got, made)
	}
	if list := f.b.runsOf("backup"); len(list) != 2 {
		t.Errorf("the update changed the run history to %d backups, want 2", len(list))
	}
	var listed struct {
		Snapshots []resticSnapshot `json:"snapshots"`
	}
	f.b.call("GET", "/api/files/sets/"+sets.photos+"/snapshots", nil, &listed)
	if len(listed.Snapshots) != 1 {
		t.Errorf("the update lists %d snapshots of photos, want the 1 from before it", len(listed.Snapshots))
	}
}

// A real scheduled round is the path a night takes, with its own bulk flags and
// batched prune. It waits for the clock, so it runs on two places only.
func TestScheduledRoundBacksUpAndPrunes(t *testing.T) {
	setsCell(t, []place{typedPath, namedOwnFolder}, func(f *flow, sets folderSets) {
		rule := keep{Last: 1}
		f.own.card(rule)
		f.nextRound(sets, 1)
		f.nextRound(sets, 2)
		f.b.write(photosPath, "a.txt", "scheduled\n", 0o640)
		f.b.write(docsPath, "c.txt", "scheduled\n", 0o644)
		backups := len(f.b.runsOf("backup"))
		prunes := len(f.b.runsOf("prune"))

		fire := time.Now().Add(70 * time.Second)
		f.b.changeSettings(func(s map[string]any) {
			s["filesSchedule"] = fmt.Sprintf("daily %02d:%02d", fire.Hour(), fire.Minute())
		})

		f.b.awaitRun("prune", prunes)
		f.b.settle()
		if got := len(f.b.runsOf("backup")); got != backups+2 {
			f.errorf("the scheduled round recorded %d backups, want 2", got-backups)
		}
		f.wantSnapshots(f.own.repo, photosTag, 1)
		f.wantSnapshots(f.own.repo, docsTag, 1)
		f.b.restic(f.b.hostPath(f.own.repo), "check")
		f.agrees("files", rule, false)
		f.wantNoFailedRuns()
	})
}
