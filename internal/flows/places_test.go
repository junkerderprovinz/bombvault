//go:build flows

package flows

import (
	"encoding/json"
	"os"
	"slices"
	"strings"
	"testing"
)

const (
	secondName   = "Second place"
	secondFolder = "user/second"
)

func TestNamedLocationOnAFolderHoldingAnOwnFolderIsRefused(t *testing.T) {
	t.Parallel()
	b := newBombvault(t)
	b.start(currentBinary)
	typeOwnFolder(b, "files", "user/named/files")
	_, raw := b.send("POST", "/api/offsite/destinations", map[string]any{"provider": "path", "name": "Named", "settings": map[string]string{"path": "user/named"}})
	if !strings.Contains(string(raw), `"ok":false`) {
		t.Fatalf("a location on the folder of an own folder was taken: %s", raw)
	}
}

// moveStep is one section's part of a planned move.
type moveStep struct {
	Pending   int  `json:"pending"`
	KeepThere keep `json:"keepThere"`
}

func TestMovedOwnFolderBringsItsBackupsAndAgesByTheNewCard(t *testing.T) {
	setsCell(t, ownFolderPlaces, func(f *flow, sets folderSets) {
		f.own.card(keep{Last: 9})
		f.nextRound(sets, 1)
		before := ids(f.b.snapshotsIn(f.own.repo, ""))

		id := f.b.namedLocation(secondName, secondFolder)
		rule := keep{Last: 1}
		f.b.setLocationCard(id, secondName, rule)
		req := map[string]any{"domains": []string{"files"}, "location": "destination:" + id, "copyOver": true}

		var plan struct {
			Steps []moveStep `json:"steps"`
		}
		f.b.call("POST", "/api/storage/own-folders/preview", req, &plan)
		if len(plan.Steps) != 1 || plan.Steps[0].Pending != len(before) || plan.Steps[0].KeepThere != rule {
			f.errorf("the plan of the move is %+v, want %d backups to copy and the rule %+v", plan.Steps, len(before), rule)
		}

		f.b.call("POST", "/api/storage/own-folders", req, nil)
		f.b.settle()

		moved := secondFolder + "/files"
		if got := ids(f.b.snapshotsIn(moved, "")); len(got) != len(before) {
			f.errorf("the new place holds %d snapshots, the old one held %d", len(got), len(before))
		}
		settings, _ := f.b.settings()
		if settings["filesPath"] != moved {
			f.errorf("the section's folder is %v after the move, want %s", settings["filesPath"], moved)
		}
		f.agrees("files", rule, false)

		f.nextRound(sets, 2)
		if got := ids(f.b.snapshotsIn(f.own.repo, "")); len(got) != len(before) {
			f.errorf("the old place changed after the move: %d snapshots, had %d", len(got), len(before))
		}
		f.wantSnapshots(moved, photosTag, 1)
		f.wantSnapshots(moved, docsTag, 1)
		f.b.restic(f.b.hostPath(moved), "check")
		f.agrees("files", rule, false)
		f.wantNoFailedRuns()
	})
}

func TestMovedOwnFolderCanHoldOnToItsRule(t *testing.T) {
	setsCell(t, []place{typedPath, namedOwnFolder}, func(f *flow, sets folderSets) {
		rule := keep{Last: 1}
		f.own.card(rule)
		f.nextRound(sets, 1)

		id := f.b.namedLocation(secondName, secondFolder)
		f.b.setLocationCard(id, secondName, keep{Last: 7})
		f.b.call("POST", "/api/storage/own-folders",
			map[string]any{"domains": []string{"files"}, "location": "destination:" + id, "copyOver": true, "keepRules": []string{"files"}}, nil)
		f.b.settle()
		f.agrees("files", rule, true)

		f.nextRound(sets, 2)
		f.wantSnapshots(secondFolder+"/files", photosTag, 1)
	})
}

func TestUseAsOwnFolderGivesATypedPathTheCardOfItsLocation(t *testing.T) {
	setsCell(t, []place{namedThenTyped}, func(f *flow, sets folderSets) {
		rule := keep{Last: 1}
		f.own.card(rule)
		f.nextRound(sets, 1)

		f.b.call("POST", "/api/storage/own-folders",
			map[string]any{"domains": []string{"files"}, "location": "destination:" + f.own.location, "copyOver": true}, nil)
		f.b.settle()
		f.agrees("files", rule, false)

		f.nextRound(sets, 2)
		f.wantSnapshots(f.own.repo, photosTag, 1)
		f.wantSnapshots(f.own.repo, docsTag, 1)
		f.wantNoFailedRuns()
	})
}

func TestTypingAnotherPathLeavesTheCardOfTheLocation(t *testing.T) {
	setsCell(t, []place{namedOwnFolder}, func(f *flow, sets folderSets) {
		rule := keep{Last: 1}
		f.own.card(rule)
		f.nextRound(sets, 1)
		f.agrees("files", rule, false)

		const typed = "user/typed/files"
		typeOwnFolder(f.b, "files", typed)
		f.agrees("files", keep{}, false)
		f.nextRound(sets, 2)
		f.nextRound(sets, 3)
		f.wantSnapshots(typed, photosTag, 2)

		typeOwnFolder(f.b, "files", f.own.repo)
		f.agrees("files", rule, false)
		f.nextRound(sets, 4)
		f.wantSnapshots(f.own.repo, photosTag, 1)
	})
}

// copyTo sets up a named location that holds no own folder and makes it a place
// the section is copied to.
func (f *flow) copyTo(rule keep) (id, source string) {
	f.t.Helper()
	id = f.b.namedLocation(secondName, secondFolder)
	f.b.setLocationCard(id, secondName, rule)
	var answer struct {
		Target struct {
			ID string `json:"id"`
		} `json:"target"`
	}
	f.b.call("POST", "/api/offsite/destinations/"+id+"/domains/files", nil, &answer)
	if answer.Target.ID == "" {
		f.t.Fatalf("no target in the answer for the copy")
	}
	// A section with placement starts a new place unticked, and the page checks
	// what the change would do before it saves it.
	var planned struct {
		Impact json.RawMessage `json:"impact"`
	}
	change := map[string]any{"skip": []string{}}
	f.b.call("POST", "/api/placement/default/files/preview", change, &planned)
	change["expect"] = planned.Impact
	f.b.call("PUT", "/api/placement/default/files", change, nil)
	return id, "offsite:" + answer.Target.ID
}

// copySection finds the section of a copy on the location.
func copySection(l location) (section, bool) {
	i := slices.IndexFunc(l.Sections, func(s section) bool { return s.Use == "copy" })
	if i < 0 {
		return section{}, false
	}
	return l.Sections[i], true
}

func (b *bombvault) locationByID(id string) location {
	b.t.Helper()
	all := b.locations()
	i := slices.IndexFunc(all, func(l location) bool { return l.ID == id })
	if i < 0 {
		b.t.Fatalf("no location %s", id)
	}
	return all[i]
}

func TestCopyToASecondPlaceAgesByItsCard(t *testing.T) {
	setsCell(t, ownFolderPlaces, func(f *flow, sets folderSets) {
		f.own.card(keep{Last: 4})
		rule := keep{Last: 1}
		id, source := f.copyTo(rule)
		copies := secondFolder + "/files"

		f.nextRound(sets, 1)
		f.nextRound(sets, 2)

		f.wantSnapshots(copies, photosTag, 1)
		f.wantSnapshots(copies, docsTag, 1)

		loc := f.b.locationByID("destination:" + id)
		sec, ok := copySection(loc)
		if !ok {
			f.t.Fatalf("location %q lists no copy: %+v", loc.Name, loc.Sections)
		}
		if sec.Retention != rule {
			f.errorf("the location lists its copy with the rule %+v, want %+v", sec.Retention, rule)
		}
		if got := whoPrunesCopies(loc); got != prunesCard {
			f.errorf("Who prunes on the copies reads %q", got)
		}
		p, read := f.b.previewOf("files", source)
		if !read {
			f.t.Fatalf("no Preview of the copy after three copies")
		}
		if !p.Policy.On || p.Policy.keep != rule {
			f.errorf("the Preview of the copy says %+v, want %+v", p.Policy, rule)
		}
		if got := p.removals(); len(got) != 0 {
			f.errorf("the copy was just aged, and its Preview still names %v", got)
		}
		f.b.restic(f.b.hostPath(copies), "check")
		f.wantNoFailedRuns()
	})
}

func TestProtectedCopyIsNeverPruned(t *testing.T) {
	setsCell(t, []place{typedPath, namedOwnFolder}, func(f *flow, sets folderSets) {
		rule := keep{Last: 1}
		id, source := f.copyTo(rule)
		f.b.call("PUT", "/api/offsite/destinations/"+id, map[string]any{"name": secondName, "storageClass": "", "immutable": true, "retention": rule}, nil)
		copies := secondFolder + "/files"
		f.nextRound(sets, 1)
		held := len(f.b.snapshotsIn(copies, photosTag))
		f.nextRound(sets, 2)

		if got := len(f.b.snapshotsIn(copies, photosTag)); got != held+1 {
			f.errorf("a protected copy holds %d snapshots of photos after one more, had %d", got, held)
		}
		if got := whoPrunesCopies(f.b.locationByID("destination:" + id)); got != prunesFar {
			f.errorf("Who prunes on the protected copies reads %q", got)
		}
		p, read := f.b.previewOf("files", source)
		if read && (len(p.Repos) == 0 || !p.Repos[0].AppendOnly || len(p.removals()) != 0) {
			f.errorf("the Preview of a protected copy says %+v", p)
		}
	})
}

func TestDeleteOfABackupFreesItsSpace(t *testing.T) {
	setsCell(t, ownFolderPlaces, func(f *flow, sets folderSets) {
		big := f.bigFile()
		f.nextRound(sets, 1)
		first := f.newestOf(f.own.repo, photosTag)
		if err := os.Remove(big); err != nil {
			f.t.Fatal(err)
		}
		f.nextRound(sets, 2)
		withBig := f.b.repoBytes(f.own.repo)
		docsBefore := ids(f.b.snapshotsIn(f.own.repo, docsTag))

		f.b.signIn()
		f.b.call("DELETE", "/api/snapshots/files/"+first.ID, nil, nil)
		f.b.settle()

		if slices.Contains(ids(f.b.snapshotsIn(f.own.repo, photosTag)), first.ID) {
			f.errorf("snapshot %s is still there after its delete", first.ID)
		}
		var listed struct {
			Snapshots []resticSnapshot `json:"snapshots"`
		}
		f.b.call("GET", "/api/files/sets/"+sets.photos+"/snapshots", nil, &listed)
		if slices.Contains(ids(listed.Snapshots), first.ID) {
			f.errorf("the API still lists snapshot %s after its delete", first.ID)
		}

		// One backup is forgotten at once and its space comes back with the next
		// prune, so that several deletes pay for one.
		prunes := len(f.b.runsOf("prune"))
		f.b.call("POST", "/api/prune/files", nil, nil)
		f.b.settle()
		if pruned := f.b.awaitRun("prune", prunes); pruned.Status != "success" {
			f.errorf("the prune after the delete is recorded as %+v", pruned)
		}
		if got := f.b.repoBytes(f.own.repo); got > withBig-bigFileBytes/2 {
			f.errorf("the repository holds %d bytes after the delete and the prune, %d before", got, withBig)
		}
		if got := ids(f.b.snapshotsIn(f.own.repo, docsTag)); !slices.Equal(got, docsBefore) {
			f.errorf("the delete of a photos backup changed the docs backups: %v, had %v", got, docsBefore)
		}
		f.b.restic(f.b.hostPath(f.own.repo), "check")

		// Every backup of a set in one go, which also forgets the set and starts
		// a prune of its own.
		prunes = len(f.b.runsOf("prune"))
		f.b.call("DELETE", "/api/files/sets/"+sets.photos+"/backups", nil, nil)
		if pruned := f.b.awaitRun("prune", prunes); pruned.Status != "success" {
			f.errorf("the prune after deleting a set's backups is recorded as %+v", pruned)
		}
		f.b.settle()
		if got := f.b.snapshotsIn(f.own.repo, photosTag); len(got) != 0 {
			f.errorf("%d snapshots of photos left after deleting all of them", len(got))
		}
		if got := ids(f.b.snapshotsIn(f.own.repo, docsTag)); !slices.Equal(got, docsBefore) {
			f.errorf("deleting the photos backups changed the docs backups: %v, had %v", got, docsBefore)
		}
		f.wantNoFailedRuns()
	})
}

func TestRuleSetBeforeTheUpdateStillDecides(t *testing.T) {
	t.Parallel()
	for _, pl := range ownFolderPlaces {
		t.Run(pl.name, func(t *testing.T) {
			t.Parallel()
			f := newFlow(t)
			rule := keep{Last: 2}
			var sets folderSets
			starts[1].boot(f, func() {
				f.own = pl.prepare(f.b, "files")
				f.b.write(photosPath, "a.txt", "first\n", 0o640)
				f.b.write(docsPath, "c.txt", "first\n", 0o644)
				sets = folderSets{f.b.createSet("photos", photosPath), f.b.createSet("docs", docsPath)}
				f.own.card(rule)
				f.backupAll(sets)
			})
			f.agrees("files", rule, false)
			f.nextRound(sets, 1)
			f.nextRound(sets, 2)
			f.wantSnapshots(f.own.repo, photosTag, 2)
			f.agrees("files", rule, false)
			f.finish()
		})
	}
}
