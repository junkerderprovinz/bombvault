//go:build flows

package flows

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

const (
	photosPath = "user/data/photos"
	docsPath   = "user/data/docs"
	photosTag  = "fileset:photos"
	docsTag    = "fileset:docs"

	// bigFileBytes is random, so restic can neither compress nor dedup it, and
	// a repository that lets go of it shrinks by this much.
	bigFileBytes = 2 << 20
)

// knownFailure is a cell that fails today for a reason on record. The cell still
// runs in full, and shows is a part of the failure it has to report.
type knownFailure struct {
	cell, why, shows string
}

// knownFailures names the cells that fail today, as go test names them, with * for
// one level of the name.
var knownFailures = []knownFailure{
	{"TestBulkRunPrunesByTheCardRule/*/typed-on-named", typedOnNamed, "want 2"},
	{"TestKeepDaysFromTheCardReachTheJob/*/typed-on-named", typedOnNamed, "the Preview says the rule is on: false"},
	{"TestBulkRunPrunesByTheRuleOfTheSection/*/typed-on-named", typedOnNamed, "want 1"},
	{"TestPreviewNamesWhatThePruneRemoves/*/typed-on-named", typedOnNamed, "names nothing to remove"},
	{"TestRuleSetBeforeTheUpdateStillDecides/typed-on-named", typedOnNamed, "want 2"},
	{"TestFlashAndSelfBackupPruneByTheirRule/*/*/typed-on-named", typedOnNamed, "want 1"},
	{"TestUseAsOwnFolderGivesATypedPathTheCardOfItsLocation/*/typed-on-named", "Use as own folder takes a typed path on the location's folder as there already and links nothing", "want 1"},

	{"TestEmptiedSelectionKeepsTheGoodBackups", "an emptied selected folder gives green empty backups that age out the good ones", "no backup holds"},
	{"TestMissingSelectedFolderKeepsTheCompleteBackup", "a selected folder that is gone gives a green backup that ages out the complete one", "no backup holds"},
	{"TestEmptyFlashKeepsTheGoodZipAndBackups", "an emptied flash folder replaces the ZIP and ages out the good backups", "flash-latest.zip holds no files"},
	{"TestZipExportWithoutARecipientSaysSo", "with export encryption on and no recipient the ZIP stops without a word", "no export run"},
	{"TestSwitchedOffLocationTakesNoBackups", "a location switched off still takes own-folder backups and prunes them", "while it is switched off"},
	{"TestPreviewOfADirectRepositoryKnowsItsRule", "the Preview of a section that writes straight to a location finds no backups", "no backups"},
	{"TestStatusKnowsACopyPrunes", "the status says no prune rule is set while a copy prunes by its card", "pruneStrategySet"},
	{"TestCopyThatNeverRanDoesNotReadOk", "an off-site copy that never ran reads ok", "reads ok"},
	{"TestOwnPlanSetWhileSwitchedOffIsScheduled", "an own plan set while the entry was off gets no cron entry", "not on the schedule"},
	{"TestSwitchedOffSectionSchedulesNothing", "a switched-off section keeps running entries with an own plan", "still on the schedule"},
	{"TestNowhereToBackUpIsNoSuccess", "a full backup of a section with nowhere to go reports success", "recorded as success"},
	{"TestOwnWeeklyPlanIsNotOverdueByTheSectionPlan", "the section's plan makes an entry on its own weekly plan read overdue", "overdue"},
	{"TestNewPlanIsNotOverdueAtOnce", "a plan set on an older database reads overdue at once", "overdue"},
	{"TestSettingsRestoreThatRestoredNothingFails", "a self-backup from another data folder restores nothing and reports success", "recorded as success"},
}

const typedOnNamed = "a location's card does not reach an own folder typed on its folder"

// folderSets are the two sets every flow backs up.
type folderSets struct {
	photos, docs string
}

func (s folderSets) ids() []string { return []string{s.photos, s.docs} }

func (b *bombvault) createSet(name, path string) string {
	b.t.Helper()
	var created struct {
		ID string `json:"id"`
	}
	b.call("POST", "/api/files/sets", map[string]any{"name": name, "path": path}, &created)
	return created.ID
}

// setsCell runs body in every cell with the sets a files flow backs up: small
// files with modes worth checking, and on an updated install one round of backups
// by the old release.
func setsCell(t *testing.T, places []place, body func(f *flow, sets folderSets)) {
	t.Helper()
	runCells(t, places, false, body)
}

// mountedSetsCell is setsCell for a restore that writes into a new folder, one
// cell at a time on the one mount there is.
func mountedSetsCell(t *testing.T, places []place, body func(f *flow, sets folderSets)) {
	t.Helper()
	runCells(t, places, true, body)
}

func runCells(t *testing.T, places []place, mounted bool, body func(f *flow, sets folderSets)) {
	t.Helper()
	// Every level in parallel, or each flow waits for the slowest of its cells
	// with cores to spare.
	parallel := func(t *testing.T) {
		if !mounted {
			t.Parallel()
		}
	}
	parallel(t)
	for _, st := range starts {
		t.Run(st.name, func(t *testing.T) {
			parallel(t)
			for _, pl := range places {
				t.Run(pl.name, func(t *testing.T) {
					parallel(t)
					f := newFlow(t)
					if mounted {
						f.b.mountOnStorage()
					}
					var sets folderSets
					st.boot(f, func() {
						f.own = pl.prepare(f.b, "files")
						sets = f.newSets()
						if st.name == "updated" {
							f.backupAll(sets)
						}
					})
					body(f, sets)
					f.finish()
				})
			}
		})
	}
}

func (f *flow) newSets() folderSets {
	f.t.Helper()
	f.b.write(photosPath, "a.txt", "first version\n", 0o640)
	f.b.write(photosPath, "sub/b.txt", "stays the same\n", 0o600)
	f.b.write(docsPath, "c.txt", "a document\n", 0o644)
	return folderSets{f.b.createSet("photos", photosPath), f.b.createSet("docs", docsPath)}
}

func (f *flow) backupAll(sets folderSets) {
	f.t.Helper()
	f.b.call("POST", "/api/files/backup-all", map[string]any{"ids": sets.ids()}, nil)
	f.b.settle()
}

func (f *flow) backupSet(id string) {
	f.t.Helper()
	f.b.call("POST", "/api/files/sets/"+id+"/backup", nil, nil)
	f.b.settle()
}

// nextRound changes both sets, so each backup holds something the one before
// did not.
func (f *flow) nextRound(sets folderSets, round int) {
	f.t.Helper()
	f.b.write(photosPath, "a.txt", fmt.Sprintf("version %d\n", round), 0o640)
	f.b.write(docsPath, "c.txt", fmt.Sprintf("document %d\n", round), 0o644)
	f.backupAll(sets)
}

// bigFile puts a file into the photos set that only the next backup will hold.
func (f *flow) bigFile() string {
	f.t.Helper()
	data := make([]byte, bigFileBytes)
	if _, err := rand.Read(data); err != nil {
		f.t.Fatal(err)
	}
	p := filepath.Join(f.b.hostPath(photosPath), "big.bin")
	if err := os.WriteFile(p, data, 0o644); err != nil {
		f.t.Fatal(err)
	}
	return p
}

func ids(snaps []resticSnapshot) []string {
	out := make([]string, 0, len(snaps))
	for _, sn := range snaps {
		out = append(out, sn.ID)
	}
	slices.Sort(out)
	return out
}

func (f *flow) wantSnapshots(repo, tag string, want int) {
	f.t.Helper()
	if got := f.b.snapshotsIn(repo, tag); len(got) != want {
		f.errorf("%s holds %d snapshots of %s, want %d", repo, len(got), tag, want)
	}
}

func (f *flow) wantNoFailedRuns() {
	f.t.Helper()
	for _, r := range f.b.runs() {
		if r.Status == "failed" {
			f.errorf("run %s of %s failed: %s", r.Kind, r.TargetID, r.Error)
		}
	}
}

func TestBackupWritesWhatTheAPIReports(t *testing.T) {
	setsCell(t, ownFolderPlaces, func(f *flow, sets folderSets) {
		before := f.b.snapshotsIfAny(f.own.repo, photosTag)
		f.b.write(photosPath, "a.txt", "as backed up\n", 0o640)
		f.backupSet(sets.photos)

		snaps := f.b.snapshotsIn(f.own.repo, photosTag)
		if len(snaps) != len(before)+1 {
			f.t.Fatalf("%d snapshots after the backup, had %d", len(snaps), len(before))
		}
		newest := slices.MaxFunc(snaps, func(a, b resticSnapshot) int { return a.Time.Compare(b.Time) })

		runs := f.b.runsOf("backup")
		if len(runs) == 0 || runs[0].Status != "success" || runs[0].SnapshotID != newest.ID {
			f.errorf("the newest backup run is %+v, want a success with the snapshot %s", runs, newest.ID)
		}
		var listed struct {
			Snapshots []resticSnapshot `json:"snapshots"`
		}
		f.b.call("GET", "/api/files/sets/"+sets.photos+"/snapshots", nil, &listed)
		if !slices.Equal(ids(listed.Snapshots), ids(snaps)) {
			f.errorf("the API lists %v, restic %v", ids(listed.Snapshots), ids(snaps))
		}

		if got := string(f.b.restic(f.b.hostPath(f.own.repo), "dump", newest.ID, resticPath(newest, "a.txt"))); got != "as backed up\n" {
			f.errorf("a.txt in the snapshot reads %q", got)
		}
		if runtime.GOOS != "windows" {
			if got := f.nodeMode(newest, "sub/b.txt"); got != 0o600 {
				f.errorf("sub/b.txt is stored with mode %o, want 600", got)
			}
		}
		f.b.restic(f.b.hostPath(f.own.repo), "check")
		f.wantDomainStatus("files", func(d domainStatus) string {
			if d.LastSuccess == 0 {
				return "the status has no last success after a backup"
			}
			return ""
		})
		f.wantNoFailedRuns()
	})
}

// resticPath is a file of the snapshot as restic spells it: below the one path
// the snapshot was taken of, with a Windows drive as a first folder.
func resticPath(sn resticSnapshot, name string) string {
	root := filepath.ToSlash(sn.Paths[0])
	if len(root) > 1 && root[1] == ':' {
		root = "/" + root[:1] + root[2:]
	}
	return strings.TrimRight(root, "/") + "/" + name
}

// nodeMode reads the permission bits restic stored for a file of a snapshot.
func (f *flow) nodeMode(sn resticSnapshot, name string) os.FileMode {
	f.t.Helper()
	want := resticPath(sn, name)
	for line := range strings.Lines(string(f.b.restic(f.b.hostPath(f.own.repo), "ls", "--json", sn.ID))) {
		var node struct {
			Path string      `json:"path"`
			Mode os.FileMode `json:"mode"`
		}
		if json.Unmarshal([]byte(line), &node) == nil && node.Path == want {
			return node.Mode.Perm()
		}
	}
	f.t.Fatalf("%s is not in snapshot %s", want, sn.ID)
	return 0
}

func TestBulkRunPrunesByTheCardRule(t *testing.T) {
	setsCell(t, ownFolderPlaces, func(f *flow, sets folderSets) {
		rule := keep{Last: 2}
		f.own.card(rule)
		f.agrees("files", rule, false)

		big := f.bigFile()
		f.nextRound(sets, 1)
		withBig := f.b.repoBytes(f.own.repo)
		if err := os.Remove(big); err != nil {
			f.t.Fatal(err)
		}
		f.nextRound(sets, 2)
		f.nextRound(sets, 3)

		f.wantSnapshots(f.own.repo, photosTag, 2)
		f.wantSnapshots(f.own.repo, docsTag, 2)
		if prunes := f.b.runsOf("prune"); len(prunes) == 0 || prunes[0].Status != "success" {
			f.errorf("no successful prune recorded after the batches: %+v", prunes)
		}
		if got := f.b.repoBytes(f.own.repo); got > withBig-bigFileBytes/2 {
			f.errorf("the repository holds %d bytes after the prune, %d with the big file", got, withBig)
		}
		f.b.restic(f.b.hostPath(f.own.repo), "check")
		f.agrees("files", rule, false)
		f.wantNoFailedRuns()
	})
}

// Keeping by days is what the card offers first. With fewer days of backups than
// the rule covers, restic keeps the newest of each day and the oldest of all.
func TestKeepDaysFromTheCardReachTheJob(t *testing.T) {
	setsCell(t, ownFolderPlaces, func(f *flow, sets folderSets) {
		rule := keep{Daily: 14}
		f.own.card(rule)
		f.nextRound(sets, 1)
		f.agrees("files", rule, false)
		f.nextRound(sets, 2)
		f.nextRound(sets, 3)
		if got := f.b.snapshotsIn(f.own.repo, photosTag); len(got) > 3 {
			f.errorf("%d snapshots of photos left under %+v", len(got), rule)
		}
	})
}

func TestBulkRunPrunesByTheRuleOfTheSection(t *testing.T) {
	setsCell(t, ownFolderPlaces, func(f *flow, sets folderSets) {
		// The card keeps more than the section's rule, and the section decides.
		card, sectionRule := keep{Last: 4}, keep{Last: 1}
		f.own.card(card)
		f.b.changeSettings(func(s map[string]any) {
			s["ownRetention"] = map[string]keep{"files": sectionRule}
		})
		f.agrees("files", sectionRule, true)
		f.nextRound(sets, 1)
		f.nextRound(sets, 2)
		f.wantSnapshots(f.own.repo, photosTag, 1)
		f.wantSnapshots(f.own.repo, docsTag, 1)
		f.agrees("files", sectionRule, true)

		// A section's rule of all zeros keeps everything, whatever the card says.
		f.b.changeSettings(func(s map[string]any) {
			s["ownRetention"] = map[string]keep{"files": {}}
		})
		f.agrees("files", keep{}, true)
		f.nextRound(sets, 3)
		f.wantSnapshots(f.own.repo, photosTag, 2)

		// Without a rule of its own the section goes back to the card.
		f.b.changeSettings(func(s map[string]any) { s["ownRetention"] = map[string]keep{} })
		f.agrees("files", card, false)
		f.own.card(keep{Last: 1})
		f.nextRound(sets, 4)
		f.wantSnapshots(f.own.repo, photosTag, 1)

		f.b.restic(f.b.hostPath(f.own.repo), "check")
		f.wantNoFailedRuns()
	})
}

func TestBackupEverythingCoversEverySectionAndPrunes(t *testing.T) {
	setsCell(t, []place{typedPath, namedOwnFolder}, func(f *flow, sets folderSets) {
		rule := keep{Last: 1}
		f.own.card(rule)
		typeOwnFolder(f.b, "flash", "user/backups/flash")
		typeOwnFolder(f.b, "config", "user/backups/config")
		// A host without Docker cannot list containers, and that section is on by
		// default.
		f.b.changeSettings(func(s map[string]any) {
			s["containersEnabled"] = false
			s["ownRetention"] = map[string]keep{"flash": rule, "config": rule}
		})

		for round := range 2 {
			f.b.write(photosPath, "a.txt", fmt.Sprintf("everything %d\n", round), 0o640)
			f.b.write(docsPath, "c.txt", fmt.Sprintf("everything %d\n", round), 0o644)
			f.b.writeFlash(round)
			f.b.call("POST", "/api/backup-everything", nil, nil)
			f.b.settle()
		}

		f.wantSnapshots(f.own.repo, photosTag, 1)
		f.wantSnapshots(f.own.repo, docsTag, 1)
		f.wantSnapshots("user/backups/flash", "", 1)
		f.wantSnapshots("user/backups/config", "", 1)
		f.agrees("files", rule, false)
		f.agrees("flash", rule, true)
		f.agrees("config", rule, true)
		f.wantNoFailedRuns()
	})
}

func TestPreviewNamesWhatThePruneRemoves(t *testing.T) {
	setsCell(t, ownFolderPlaces, func(f *flow, sets folderSets) {
		f.nextRound(sets, 1)
		f.nextRound(sets, 2)
		f.agrees("files", keep{}, false)
		if off, _ := f.b.previewOf("files", ""); len(off.removals()) != 0 {
			f.errorf("with every keep value at 0 the Preview would remove %v", off.removals())
		}

		rule := keep{Last: 1}
		f.own.card(rule)
		before := ids(f.b.snapshotsIn(f.own.repo, ""))
		planned, _ := f.b.previewOf("files", "")
		named := planned.removals()
		if len(named) == 0 {
			f.errorf("with %+v and %d snapshots the Preview names nothing to remove", rule, len(before))
		}

		f.b.signIn()
		f.b.call("POST", "/api/prune/files", nil, nil)
		f.b.settle()

		after := ids(f.b.snapshotsIn(f.own.repo, ""))
		removed := slices.DeleteFunc(slices.Clone(before), func(id string) bool { return slices.Contains(after, id) })
		if !slices.Equal(named, removed) {
			f.errorf("the Preview named %v, the prune removed %v", named, removed)
		}
		if again, _ := f.b.previewOf("files", ""); len(again.removals()) != 0 {
			f.errorf("after the prune the Preview still names %v", again.removals())
		}
		f.b.restic(f.b.hostPath(f.own.repo), "check")
	})
}

// snapshotsIfAny is snapshotsIn for a repository the first backup has yet to make.
func (b *bombvault) snapshotsIfAny(sub, tag string) []resticSnapshot {
	b.t.Helper()
	if _, err := os.Stat(filepath.Join(b.hostPath(sub), "config")); err != nil {
		return nil
	}
	return b.snapshotsIn(sub, tag)
}

type domainStatus struct {
	Domain           string `json:"domain"`
	LastSuccess      int64  `json:"lastSuccess"`
	ReplicationState string `json:"replicationState"`
	PruneStrategySet bool   `json:"pruneStrategySet"`
	Status           string `json:"status"`
}

func (b *bombvault) statusOf(domain string) domainStatus {
	b.t.Helper()
	var got struct {
		Domains []domainStatus `json:"domains"`
	}
	b.call("GET", "/api/status", nil, &got)
	i := slices.IndexFunc(got.Domains, func(d domainStatus) bool { return d.Domain == domain })
	if i < 0 {
		b.t.Fatalf("the status lists no %s", domain)
	}
	return got.Domains[i]
}

func (f *flow) wantDomainStatus(domain string, problem func(domainStatus) string) {
	f.t.Helper()
	if msg := problem(f.b.statusOf(domain)); msg != "" {
		f.errorf("%s: %s", domain, msg)
	}
}
