//go:build flows

package flows

import (
	"archive/zip"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
)

// typedFlow is one install with a typed own folder for domain, for a cell that
// does not vary by start or place.
func typedFlow(t *testing.T, domain string) *flow {
	t.Helper()
	t.Parallel()
	f := newFlow(t)
	f.b.start(currentBinary)
	f.own = typedPath.prepare(f.b, domain)
	return f
}

// holdsFile reports whether any backup of a tag in the repository holds a file.
func (f *flow) holdsFile(repo, tag, name string) bool {
	f.t.Helper()
	for _, sn := range f.b.snapshotsIn(repo, tag) {
		if strings.Contains(string(f.b.restic(f.b.hostPath(repo), "ls", sn.ID)), name) {
			return true
		}
	}
	return false
}

// selectFolders limits a set to folders below it, as the tree editor does.
func (f *flow) selectFolders(id, setPath string, folders ...string) {
	f.t.Helper()
	selected := make([]string, 0, len(folders))
	for _, folder := range folders {
		selected = append(selected, f.b.mount+"/"+setPath+"/"+folder)
	}
	f.b.call("PATCH", "/api/files/sets/"+id, map[string]any{"selectedPaths": selected}, nil)
}

func (f *flow) lastRun(kind string) run {
	f.t.Helper()
	list := f.b.runsOf(kind)
	if len(list) == 0 {
		f.t.Fatalf("no %s run recorded", kind)
	}
	return list[0]
}

func TestOwnFolderMovedOntoACopyFollowsTheCardAfterAnUpdate(t *testing.T) {
	t.Parallel()
	f := newFlow(t)
	rule := keep{Last: 1}
	// A move on 10.4.1 is what left the rule behind, so this cell stays on that
	// release whatever the last one is.
	f.b.start(releaseBinary(t, "v10.4.1"))
	f.b.setSharedCard(keep{Daily: 14})
	f.b.changeSettings(func(s map[string]any) { s["filesEnabled"] = true })
	id, _ := f.copyTo(keep{})
	f.b.moveOwnFolder("files", "destination:"+id)
	f.b.setLocationCard(id, secondName, rule)
	f.own = ownFolder{repo: secondFolder + "/files"}
	sets := f.newSets()
	f.backupAll(sets)
	f.b.stop()
	f.b.start(currentBinary)

	f.agrees("files", rule, false)
	f.nextRound(sets, 1)
	f.nextRound(sets, 2)
	f.wantSnapshots(f.own.repo, photosTag, 1)
	f.finish()
}

func TestNamedRepositoryKeepsWhatItsPageSays(t *testing.T) {
	t.Parallel()
	f := newFlow(t)
	f.b.start(currentBinary)
	f.own = namedOwnFolder.prepare(f.b, "files")
	const nas = "user/nas"
	var saved struct {
		Repo struct {
			ID string `json:"id"`
		} `json:"repo"`
	}
	f.b.call("POST", "/api/repos", map[string]any{"name": "NAS repo", "repo": nas}, &saved)
	f.b.write(photosPath, "a.txt", "first\n", 0o640)
	var created struct {
		ID string `json:"id"`
	}
	f.b.call("POST", "/api/files/sets", map[string]any{"name": "photos", "path": photosPath, "repo": saved.Repo.ID}, &created)
	f.own.card(keep{Last: 1})

	for _, version := range []string{"second\n", "third\n", "fourth\n"} {
		f.b.write(photosPath, "a.txt", version, 0o640)
		f.backupSet(created.ID)
	}

	f.wantSnapshots(nas, photosTag, 3)
	for _, l := range f.b.locations() {
		for _, s := range l.Sections {
			if s.RepoID == saved.Repo.ID && s.Retention != (keep{}) {
				f.errorf("the page of the named repository lists its rule as %+v, its card as %+v", s.Retention, l.Retention)
			}
		}
	}
	f.finish()
}

func TestEmptiedSelectionKeepsTheGoodBackups(t *testing.T) {
	f := typedFlow(t, "files")
	f.b.setSharedCard(keep{Last: 2})
	f.b.write(photosPath, "docs/a.txt", "one\n", 0o640)
	f.b.write(photosPath, "docs/b.txt", "two\n", 0o640)
	f.b.write(photosPath, "other.txt", "stays outside the selection\n", 0o640)
	id := f.b.createSet("photos", photosPath)
	f.selectFolders(id, photosPath, "docs")
	f.backupSet(id)
	f.b.write(photosPath, "docs/a.txt", "changed\n", 0o640)
	f.backupSet(id)

	for _, name := range []string{"a.txt", "b.txt"} {
		if err := os.Remove(filepath.Join(f.b.hostPath(photosPath), "docs", name)); err != nil {
			f.t.Fatal(err)
		}
	}
	f.backupSet(id)
	f.backupSet(id)

	if !f.holdsFile(f.own.repo, photosTag, "docs/b.txt") {
		f.errorf("no backup holds docs/b.txt any more, and the last run is %+v", f.lastRun("backup"))
	}
	f.wantWarning(f.lastRun("backup"), olderKept)
	f.finish()
}

// olderKept is what the run of a backup that saw less than it should ends on.
const olderKept = "older backups were kept"

// wantWarning asserts a run that went through and says so in its note.
func (f *flow) wantWarning(r run, says string) {
	f.t.Helper()
	if r.Status != "success" || !strings.Contains(r.Error, says) {
		f.errorf("the run reads %s %q, want a success with a note that says %q", r.Status, r.Error, says)
	}
}

func TestACompleteBackupAfterAGapAgesAgain(t *testing.T) {
	f := typedFlow(t, "files")
	f.b.setSharedCard(keep{Last: 1})
	f.b.write(photosPath, "docs/a.txt", "one\n", 0o640)
	f.b.write(photosPath, "pics/p.txt", "two\n", 0o640)
	id := f.b.createSet("photos", photosPath)
	f.selectFolders(id, photosPath, "docs", "pics")
	f.backupSet(id)

	pics := filepath.Join(f.b.hostPath(photosPath), "pics")
	if err := os.RemoveAll(pics); err != nil {
		f.t.Fatal(err)
	}
	f.backupSet(id)
	f.backupSet(id)
	if got := len(f.b.snapshotsIn(f.own.repo, photosTag)); got != 3 {
		f.errorf("%d backups after two that missed a folder, want all 3 kept", got)
	}

	f.b.write(photosPath, "pics/p.txt", "back\n", 0o640)
	f.backupSet(id)
	if last := f.lastRun("backup"); last.Status != "success" || last.Error != "" {
		f.errorf("the complete backup reads %s %q", last.Status, last.Error)
	}
	if got := len(f.b.snapshotsIn(f.own.repo, photosTag)); got != 1 {
		f.errorf("%d backups after a complete one under keep-last 1", got)
	}
	f.finish()
}

func TestSmallFolderAndAFirstBackupStayPlain(t *testing.T) {
	f := typedFlow(t, "files")
	f.b.setSharedCard(keep{Last: 1})
	f.b.write(photosPath, "a.txt", "small\n", 0o640)
	id := f.b.createSet("photos", photosPath)
	f.backupSet(id)
	if first := f.lastRun("backup"); first.Status != "success" || first.Error != "" {
		f.errorf("a first backup reads %s %q", first.Status, first.Error)
	}
	f.b.write(photosPath, "a.txt", "still small\n", 0o640)
	f.backupSet(id)
	if got := len(f.b.snapshotsIn(f.own.repo, photosTag)); got != 1 {
		f.errorf("a small folder under keep-last 1 holds %d backups", got)
	}
	f.finish()
}

func TestAlwaysEmptySelectedFolderIsNoWarning(t *testing.T) {
	f := typedFlow(t, "files")
	f.b.setSharedCard(keep{Last: 1})
	f.b.write(photosPath, "docs/a.txt", "one\n", 0o640)
	if err := os.MkdirAll(filepath.Join(f.b.hostPath(photosPath), "inbox"), 0o755); err != nil {
		f.t.Fatal(err)
	}
	id := f.b.createSet("photos", photosPath)
	f.selectFolders(id, photosPath, "docs", "inbox")
	f.backupSet(id)
	f.backupSet(id)
	if last := f.lastRun("backup"); last.Status != "success" || last.Error != "" {
		f.errorf("a folder that was always empty gives a run that reads %s %q", last.Status, last.Error)
	}
	if got := len(f.b.snapshotsIn(f.own.repo, photosTag)); got != 1 {
		f.errorf("%d backups under keep-last 1, with nothing missing", got)
	}
	f.finish()
}

func TestABackupThatMissedAFolderHoldsItsCopyAndAPruneByHand(t *testing.T) {
	f := typedFlow(t, "files")
	f.b.setSharedCard(keep{Last: 1})
	f.b.write(photosPath, "docs/a.txt", "one\n", 0o640)
	f.b.write(photosPath, "pics/p.txt", "two\n", 0o640)
	id := f.b.createSet("photos", photosPath)
	f.selectFolders(id, photosPath, "docs", "pics")
	f.copyTo(keep{Last: 1})
	f.backupSet(id)
	copies := secondFolder + "/files"
	if !f.holdsFile(copies, photosTag, "pics/p.txt") {
		f.t.Fatalf("the copy never held pics/p.txt")
	}

	if err := os.RemoveAll(filepath.Join(f.b.hostPath(photosPath), "pics")); err != nil {
		f.t.Fatal(err)
	}
	f.backupSet(id)
	f.backupSet(id)
	if !f.holdsFile(copies, photosTag, "pics/p.txt") {
		f.errorf("the copy aged out the last backup that holds pics/p.txt")
	}

	f.b.signIn()
	f.b.call("POST", "/api/prune/files", nil, nil)
	f.b.settle()
	if !f.holdsFile(f.own.repo, photosTag, "pics/p.txt") {
		f.errorf("a prune by hand aged out the last backup that holds pics/p.txt")
	}
	f.finish()
}

func TestMissingSelectedFolderKeepsTheCompleteBackup(t *testing.T) {
	f := typedFlow(t, "files")
	f.b.setSharedCard(keep{Last: 1})
	f.b.write(photosPath, "docs/a.txt", "one\n", 0o640)
	f.b.write(photosPath, "pics/p.txt", "two\n", 0o640)
	id := f.b.createSet("photos", photosPath)
	f.selectFolders(id, photosPath, "docs", "pics")
	f.backupSet(id)

	if err := os.RemoveAll(filepath.Join(f.b.hostPath(photosPath), "pics")); err != nil {
		f.t.Fatal(err)
	}
	f.backupSet(id)

	if !f.holdsFile(f.own.repo, photosTag, "pics/p.txt") {
		f.errorf("no backup holds pics/p.txt any more, and the last run is %+v", f.lastRun("backup"))
	}
	last := f.lastRun("backup")
	f.wantWarning(last, olderKept)
	if !strings.Contains(last.Error, "pics") {
		f.errorf("the run does not name the folder it missed: %q", last.Error)
	}
	f.finish()
}

// zipFiles counts the files in a ZIP, -1 when it cannot be read.
func zipFiles(p string) int {
	r, err := zip.OpenReader(p)
	if err != nil {
		return -1
	}
	defer func() { _ = r.Close() }()
	return len(r.File)
}

func TestEmptyFlashKeepsTheGoodZipAndBackups(t *testing.T) {
	f := typedFlow(t, "flash")
	f.b.setSharedCard(keep{Last: 2})
	const zips = "user/zips"
	f.b.changeSettings(func(s map[string]any) {
		s["flashZipExportEnabled"], s["flashZipExportPath"] = true, zips
	})
	f.b.writeFlash(0)
	f.b.call("POST", "/api/flash/backup", nil, nil)
	f.b.settle()
	f.b.writeFlash(1)
	f.b.call("POST", "/api/flash/backup", nil, nil)
	f.b.settle()
	latest := filepath.Join(f.b.hostPath(zips), "flash-latest.zip")
	if zipFiles(latest) < 1 {
		f.t.Fatalf("no ZIP with files from a good flash: %d", zipFiles(latest))
	}

	entries, err := os.ReadDir(f.b.flash)
	if err != nil {
		f.t.Fatal(err)
	}
	for _, e := range entries {
		if err := os.RemoveAll(filepath.Join(f.b.flash, e.Name())); err != nil {
			f.t.Fatal(err)
		}
	}
	f.b.call("POST", "/api/flash/backup", nil, nil)
	f.b.settle()
	f.b.call("POST", "/api/flash/backup", nil, nil)
	f.b.settle()

	if zipFiles(latest) < 1 {
		f.errorf("flash-latest.zip holds no files after a backup of an empty flash folder")
	}
	if !f.holdsFile(f.own.repo, "", "config/go") {
		f.errorf("no backup holds config/go any more")
	}
	f.wantWarning(f.lastRun("backup"), olderKept)
	if export := f.lastRun("export"); export.Status != "failed" || !strings.Contains(export.Error, "last ZIP was kept") {
		f.errorf("the export of an empty flash reads %s %q", export.Status, export.Error)
	}
	f.finish()
}

func TestZipExportWithoutARecipientSaysSo(t *testing.T) {
	f := typedFlow(t, "flash")
	f.b.writeFlash(0)
	f.b.changeSettings(func(s map[string]any) {
		s["flashZipExportEnabled"], s["flashZipExportPath"] = true, "user/zips"
		s["exportEncryptEnabled"], s["exportAgeRecipients"] = true, ""
	})
	f.b.call("POST", "/api/flash/backup", nil, nil)
	f.b.settle()

	if !slices.ContainsFunc(f.b.runs(), func(r run) bool { return r.Kind == "export" && r.Status == "failed" }) {
		f.errorf("no export run says the ZIP was not written, the runs are %+v", f.b.runs())
	}
	f.finish()
}

func TestSwitchedOffLocationTakesNoBackups(t *testing.T) {
	t.Parallel()
	f := newFlow(t)
	f.b.start(currentBinary)
	f.own = namedOwnFolder.prepare(f.b, "files")
	sets := f.newSets()
	f.own.card(keep{Last: 1})
	f.nextRound(sets, 1)
	before := ids(f.b.snapshotsIn(f.own.repo, photosTag))

	f.b.call("PUT", "/api/offsite/destinations/"+f.own.location, map[string]any{"name": "Named place", "storageClass": "", "immutable": false, "enabled": false}, nil)
	f.b.write(photosPath, "a.txt", "written with the location off\n", 0o640)
	if _, raw := f.b.send("POST", "/api/files/sets/"+sets.photos+"/backup", nil); strings.Contains(string(raw), `"ok":true`) {
		f.b.settle()
	}

	if after := ids(f.b.snapshotsIn(f.own.repo, photosTag)); !slices.Equal(after, before) {
		f.errorf("the location holds %v while it is switched off, it held %v", after, before)
	}
	if last := f.lastRun("backup"); last.Status != "failed" || !strings.Contains(last.Error, "switched off") {
		f.errorf("a backup into a switched-off location reads %s %q", last.Status, last.Error)
	}

	f.b.signIn()
	if _, raw := f.b.send("POST", "/api/prune/files", nil); !strings.Contains(string(raw), "switched off") {
		f.errorf("a prune of a switched-off location answers %s", raw)
	}
	f.b.settle()
	if after := ids(f.b.snapshotsIn(f.own.repo, photosTag)); !slices.Equal(after, before) {
		f.errorf("the prune changed a switched-off location: %v, it held %v", after, before)
	}
	f.finish()
}

func TestCardOfOneTypedFolderLeavesTheOther(t *testing.T) {
	f := typedFlow(t, "files")
	typeOwnFolder(f.b, "flash", "user/slow/flash")
	typeOwnFolder(f.b, "files", "user/fast/files")
	f.own.repo = "user/fast/files"
	sets := f.newSets()
	var fast, slow location
	for _, l := range f.b.locations() {
		switch l.Where {
		case "user/fast":
			fast = l
		case "user/slow":
			slow = l
		}
	}
	if fast.ID == "" || slow.ID == "" {
		f.t.Fatalf("no location for each typed folder: %+v", f.b.locations())
	}
	rule := keep{Last: 1}
	f.b.call("PUT", "/api/storage/locations/"+fast.ID+"/keep", rule, nil)

	if got := f.b.locationByID(fast.ID).Retention; *got != rule {
		f.errorf("the card of the folder reads %+v after saving %+v", *got, rule)
	}
	if got := f.b.locationByID(slow.ID).Retention; *got != (keep{}) {
		f.errorf("saving one card changed the other to %+v", *got)
	}
	f.nextRound(sets, 1)
	f.nextRound(sets, 2)
	f.wantSnapshots(f.own.repo, photosTag, 1)
	f.agrees("files", rule, false)
	f.finish()
}

func TestPreviewOfADirectRepositoryKnowsItsRule(t *testing.T) {
	f := typedFlow(t, "flash")
	rule := keep{Last: 1}
	id := f.b.namedLocation(secondName, secondFolder)
	f.b.setLocationCard(id, secondName, rule)
	f.b.call("POST", "/api/offsite/destinations/"+id+"/domains/flash", nil, nil)
	f.b.call("PUT", "/api/storage/sections/flash/local", map[string]any{"on": false}, nil)
	for round := range 3 {
		f.b.writeFlash(round)
		f.b.call("POST", "/api/flash/backup", nil, nil)
		f.b.settle()
	}
	f.wantSnapshots(secondFolder+"/flash", "", 1)

	if p, read := f.b.previewOf("flash", ""); !read {
		f.errorf("the Preview says no backups yet, with one at the place the rule %+v ages", rule)
	} else if !p.Policy.On {
		f.errorf("the Preview says the rule is off, and the place aged by %+v", rule)
	}
	f.finish()
}

func TestStatusKnowsACopyPrunes(t *testing.T) {
	f := typedFlow(t, "files")
	sets := f.newSets()
	f.copyTo(keep{Last: 1})
	f.nextRound(sets, 1)
	f.nextRound(sets, 2)
	f.wantSnapshots(secondFolder+"/files", photosTag, 1)

	if !f.b.statusOf("files").PruneStrategySet {
		f.errorf("pruneStrategySet is false, and the copy just aged by its card")
	}
	f.finish()
}

func TestCopyThatNeverRanDoesNotReadOk(t *testing.T) {
	f := typedFlow(t, "files")
	sets := f.newSets()
	f.b.changeSettings(func(s map[string]any) { s["filesSchedule"] = "daily 03:00" })
	f.nextRound(sets, 1)
	f.copyTo(keep{})

	if got := f.b.statusOf("files").ReplicationState; got == "ok" {
		f.errorf("the copy reads ok with nothing ever copied to it")
	}
	f.finish()
}

// onSchedule reports whether the cron table holds the backup job of a domain.
func (b *bombvault) onSchedule(domain string) bool {
	b.t.Helper()
	var next struct {
		Runs []struct {
			Job    string `json:"job"`
			Domain string `json:"domain"`
		} `json:"runs"`
	}
	b.call("GET", "/api/schedule/next", nil, &next)
	return slices.ContainsFunc(next.Runs, func(r struct {
		Job    string `json:"job"`
		Domain string `json:"domain"`
	}) bool {
		return r.Job == "backup" && r.Domain == domain
	})
}

func TestCopyThatNeverCameReadsOverdueOnceItsGraceIsOver(t *testing.T) {
	f := typedFlow(t, "files")
	sets := f.newSets()
	f.b.changeSettings(func(s map[string]any) { s["filesSchedule"] = "daily 03:00" })
	f.nextRound(sets, 1)
	f.copyTo(keep{})
	f.b.age(72 * time.Hour)

	if got := f.b.statusOf("files").ReplicationState; got != "overdue" {
		f.errorf("a copy set up three days ago and never made reads %q", got)
	}
	f.finish()
}

// pingLog is a health check as BombVault sees it: it notes every ping in order.
func pingLog(t *testing.T) (string, func() []string) {
	t.Helper()
	var mu sync.Mutex
	var pings []string
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		pings = append(pings, r.URL.Path)
	}))
	t.Cleanup(srv.Close)
	return srv.URL, func() []string {
		mu.Lock()
		defer mu.Unlock()
		return slices.Clone(pings)
	}
}

// checkOnly is a notification config with one Healthchecks channel and
// nothing else.
func checkOnly(url string) map[string]any {
	return map[string]any{"channels": []map[string]any{{
		"id": "checks", "kind": "healthchecks", "enabled": true, "on": "always",
		"healthchecks": map[string]any{"url": url},
	}}}
}

func TestFreshInstallPingsAWholeBackup(t *testing.T) {
	f := typedFlow(t, "files")
	sets := f.newSets()
	url, pings := pingLog(t)
	f.b.call("POST", "/api/notify", checkOnly(url), nil)
	f.backupSet(sets.photos)

	if got := strings.Join(pings(), " "); got != "/start /" {
		f.errorf("a good backup on a fresh install pinged %q", got)
	}
	f.finish()
}

func TestAGoodBackupDoesNotGreenAFailureOnTheSameCheck(t *testing.T) {
	f := typedFlow(t, "files")
	sets := f.newSets()
	url, pings := pingLog(t)
	f.b.call("POST", "/api/notify", checkOnly(url), nil)
	f.backupSet(sets.photos)

	if err := os.RemoveAll(f.b.hostPath(photosPath)); err != nil {
		f.t.Fatal(err)
	}
	if err := os.MkdirAll(f.b.hostPath(photosPath), 0o750); err != nil {
		f.t.Fatal(err)
	}
	f.backupSet(sets.photos)
	if f.lastRun("backup").Status != "failed" {
		f.t.Fatalf("the emptied set did not fail: %+v", f.lastRun("backup"))
	}
	f.backupSet(sets.docs)
	all := pings()
	if len(all) == 0 || all[len(all)-1] != "/fail" {
		f.errorf("the good backup beside a failed one left the check at %q", all)
	}

	f.b.write(photosPath, "a.txt", "back\n", 0o640)
	f.backupSet(sets.photos)
	all = pings()
	if all[len(all)-1] != "/" {
		f.errorf("the check stayed at %q after the failed set was good again", all)
	}
	f.finish()
}

func TestOwnPlanSetWhileSwitchedOffIsScheduled(t *testing.T) {
	f := typedFlow(t, "files")
	sets := f.newSets()
	f.b.call("PATCH", "/api/files/sets/"+sets.photos, map[string]any{"enabled": false}, nil)
	f.b.call("PATCH", "/api/files/sets/"+sets.docs, map[string]any{"enabled": false}, nil)
	f.b.call("PATCH", "/api/files/sets/"+sets.photos, map[string]any{"scheduleCadence": "daily 03:00"}, nil)
	f.b.call("PATCH", "/api/files/sets/"+sets.photos, map[string]any{"enabled": true}, nil)

	if !f.b.onSchedule("files") {
		f.errorf("a set switched on with its own plan is not on the schedule")
	}
	f.finish()
}

func TestSwitchedOffSectionSchedulesNothing(t *testing.T) {
	f := typedFlow(t, "files")
	sets := f.newSets()
	f.b.call("PATCH", "/api/files/sets/"+sets.photos, map[string]any{"scheduleCadence": "daily 03:00"}, nil)
	f.b.changeSettings(func(s map[string]any) { s["filesEnabled"] = false })

	if f.b.onSchedule("files") {
		f.errorf("a set of a switched-off section is still on the schedule")
	}
	f.finish()
}

func TestNowhereToBackUpIsNoSuccess(t *testing.T) {
	f := typedFlow(t, "files")
	f.newSets()
	var planned struct {
		Impact any `json:"impact"`
	}
	change := map[string]any{"home": "none"}
	f.b.call("POST", "/api/placement/default/files/preview", change, &planned)
	change["expect"] = planned.Impact
	f.b.call("PUT", "/api/placement/default/files", change, nil)
	f.b.changeSettings(func(s map[string]any) { s["containersEnabled"] = false })

	f.b.call("POST", "/api/backup-everything", nil, nil)
	f.b.settle()

	all := f.b.runs()
	i := slices.IndexFunc(all, func(r run) bool { return r.TargetID == "everything" })
	if i < 0 {
		f.t.Fatalf("no run of the full backup in %+v", all)
	}
	if all[i].Status == "success" {
		f.errorf("a full backup with nowhere to put anything is recorded as success: %+v", all[i])
	}
	f.finish()
}

func TestOwnWeeklyPlanIsNotOverdueByTheSectionPlan(t *testing.T) {
	f := typedFlow(t, "files")
	sets := f.newSets()
	f.b.changeSettings(func(s map[string]any) { s["filesSchedule"] = "daily 03:00" })
	for _, id := range sets.ids() {
		f.b.call("PATCH", "/api/files/sets/"+id, map[string]any{"scheduleCadence": "weekly Sun 02:00"}, nil)
	}
	f.backupAll(sets)
	f.b.age(72 * time.Hour)

	if got := f.b.statusOf("files").Status; got == "overdue" {
		f.errorf("sets on a weekly plan backed up three days ago read overdue")
	}
	f.finish()
}

func TestNewPlanIsNotOverdueAtOnce(t *testing.T) {
	f := typedFlow(t, "flash")
	f.b.age(72 * time.Hour)
	f.b.changeSettings(func(s map[string]any) { s["flashSchedule"] = "daily 03:00" })

	if got := f.b.statusOf("flash").Status; got == "overdue" {
		f.errorf("a plan saved this moment reads overdue on a database three days old")
	}
	f.finish()
}

func TestSettingsRestoreThatRestoredNothingFails(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("BombVault ships for Linux, and restoring its own database does not complete on Windows")
	}
	f := typedFlow(t, "config")
	f.backupSingleton(singletons[1], 0)
	sn := f.newestOf(f.own.repo, "")

	// The self-backup holds the folder it was read from, so one made under another
	// data folder has nothing where the restore looks.
	f.b.stop()
	moved := f.b.data + "-moved"
	if err := os.Rename(f.b.data, moved); err != nil {
		f.t.Fatal(err)
	}
	f.b.data = moved
	f.b.start(currentBinary)
	f.b.changeSettings(func(s map[string]any) { s["instanceName"] = "changed since" })
	f.b.call("POST", "/api/config/restore", map[string]any{"snapshot": sn.ID}, nil)
	f.b.settle()
	last := f.lastRun("restore")
	f.b.stop()
	f.b.start(currentBinary)

	settings, _ := f.b.settings()
	if settings["instanceName"] != instanceName(0) && last.Status == "success" {
		f.errorf("the instance is called %v after a restore recorded as success: %+v", settings["instanceName"], last)
	}
	f.finish()
}
