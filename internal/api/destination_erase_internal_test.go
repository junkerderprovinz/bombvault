package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

const boxRepo = "rest:https://box:8000/bv"

func (f *placementFixture) erasurePreview(d store.OffsiteTarget) destinationErasure {
	f.t.Helper()
	res := f.do("GET", "/api/offsite/destinations/"+d.ID+"/erasure", nil)
	if res["ok"] != true {
		f.t.Fatalf("preview: %v", res)
	}
	return erasureOf(f.t, res)
}

func erasureOf(t *testing.T, res map[string]any) destinationErasure {
	t.Helper()
	b, err := json.Marshal(res["erasure"])
	if err != nil {
		t.Fatal(err)
	}
	var out destinationErasure
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func (f *placementFixture) erase(d store.OffsiteTarget, expect destinationRemoval, lastCopies ...string) map[string]any {
	f.t.Helper()
	return f.do("POST", "/api/offsite/destinations/"+d.ID+"/erasure", map[string]any{"expect": expect, "lastCopies": lastCopies})
}

// waitForErasure returns once the run that holds the domain has let go of it.
func (f *placementFixture) waitForErasure(domain string) {
	f.t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if unlock, ok := f.svc.tryLockDomainFor(domain, "test"); ok {
			unlock()
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	f.t.Fatalf("the deletion still holds %s", domain)
}

func (f *placementFixture) held(location string) []string {
	f.eng.mu.Lock()
	defer f.eng.mu.Unlock()
	return snapshotIDs(f.eng.snaps[location])
}

// boxWithVMs is a destination whose vms section holds the copies of the two
// backups of vm:win that are at home, three backups of vm:old that are nowhere
// else, and a snapshot somebody wrote by hand.
func boxWithVMs(f *placementFixture) (box, vms store.OffsiteTarget) {
	box = f.destination("Box", boxRepo)
	vms = f.tickDestination(box, "vms")
	f.hold(f.domainPath("vms"), snap("a1", 100, "vm:win"), snap("a2", 200, "vm:win"))
	f.hold(boxRepo+"/vms",
		copied("c1", "a1", 100, "vm:win"), copied("c2", "a2", 200, "vm:win"),
		snap("o1", 50, "vm:old"), snap("o2", 60, "vm:old"), snap("o3", 70, "vm:old"),
		snap("hand", 80))
	return box, vms
}

func TestTheErasurePreviewCountsWhatLosesACopyAndNamesTheLastCopies(t *testing.T) {
	f := newPlacementFixture(t)
	f.signIn()
	box, vms := boxWithVMs(f)
	files := f.tickDestination(box, "files")
	f.appendOnly(files.ID)
	f.hold(boxRepo+"/files", snap("f1", 10, "fileset:docs"), snap("f2", 20, "fileset:docs"))

	got := f.erasurePreview(box)

	want := []erasureSection{
		{Domain: "files", TargetID: files.ID, Repo: boxRepo + "/files", Backups: 2, Entries: 1, Kept: erasureKeptAppendOnly},
		{Domain: "vms", TargetID: vms.ID, Repo: boxRepo + "/vms", Backups: 5, Entries: 2},
	}
	if !slices.Equal(got.Sections, want) {
		t.Fatalf("sections = %+v, want %+v", got.Sections, want)
	}
	if got.Backups != 5 || got.Entries != 2 || got.BackupsOnlyThere != 3 {
		t.Fatalf("totals = %d backups, %d entries, %d only there; want 5, 2 and 3", got.Backups, got.Entries, got.BackupsOnlyThere)
	}
	if last := []erasureEntry{{Domain: "vms", Identity: "vm:old", Backups: 3}}; !slices.Equal(got.LastCopies, last) {
		t.Fatalf("last copies = %+v, want %+v", got.LastCopies, last)
	}
	if got.Key != "erase:"+box.ID || len(got.Removal.Sections) != 2 {
		t.Fatalf("key %q and removal %+v", got.Key, got.Removal)
	}
	if len(f.eng.deletes) != 0 {
		t.Fatalf("the preview deleted %+v", f.eng.deletes)
	}
}

func TestAnErasureThatDoesNotNameTheLastCopiesDeletesNothing(t *testing.T) {
	f := newPlacementFixture(t)
	f.signIn()
	box, _ := boxWithVMs(f)
	preview := f.erasurePreview(box)

	res := f.erase(box, preview.Removal)

	if res["ok"] != false || res["code"] != "last-copies" {
		t.Fatalf("answer = %v, want the refusal last-copies", res)
	}
	if now := erasureOf(t, res); len(now.LastCopies) != 1 || now.LastCopies[0].Identity != "vm:old" {
		t.Fatalf("the refusal carries %+v, want the entry to name", now.LastCopies)
	}
	f.waitForErasure("vms")
	if len(f.eng.deletes) != 0 || len(f.held(boxRepo+"/vms")) != 6 {
		t.Fatalf("a refused deletion forgot %+v", f.eng.deletes)
	}
	if _, ok, _ := f.st.GetDestination(box.ID); !ok {
		t.Fatal("a refused deletion removed the destination")
	}
}

func TestAnErasureEmptiesTheSectionsAndThenRemovesTheDestination(t *testing.T) {
	f := newPlacementFixture(t)
	f.signIn()
	box, vms := boxWithVMs(f)
	files := f.tickDestination(box, "files")
	f.appendOnly(files.ID)
	f.hold(boxRepo+"/files", snap("f1", 10, "fileset:docs"))
	preview := f.erasurePreview(box)

	res := f.erase(box, preview.Removal, "vm:old")
	if res["ok"] != true || res["started"] != true {
		t.Fatalf("answer = %v, want a started run", res)
	}
	f.waitForErasure("vms")

	if left := f.held(boxRepo + "/vms"); !slices.Equal(left, []string{"hand"}) {
		t.Fatalf("the vms repository still holds %v, want the snapshot written by hand alone", left)
	}
	if left := f.held(boxRepo + "/files"); !slices.Equal(left, []string{"f1"}) {
		t.Fatalf("the append-only files repository holds %v, want it untouched", left)
	}
	if !slices.Equal(f.eng.prunes, []string{boxRepo + "/vms"}) {
		t.Fatalf("pruned %v, want the vms repository once", f.eng.prunes)
	}
	if _, ok, _ := f.st.GetDestination(box.ID); ok {
		t.Fatal("the destination is still there")
	}
	if f.hasTarget(vms.ID) || f.hasTarget(files.ID) {
		t.Fatal("a section still copies to the removed destination")
	}
	run, err := f.st.LastRunOfKind("vms", "prune")
	if err != nil || run == nil || run.Status != "success" {
		t.Fatalf("the run of the vms section = %+v, %v; want a successful prune", run, err)
	}
	if home := f.held(f.domainPath("vms")); len(home) != 2 {
		t.Fatalf("the backups at home are %v, want both left alone", home)
	}
}

func TestAFailedErasureKeepsTheDestination(t *testing.T) {
	f := newPlacementFixture(t)
	f.signIn()
	box, vms := boxWithVMs(f)
	f.eng.forgetErr = map[string]error{boxRepo + "/vms": errors.New("connection reset")}
	preview := f.erasurePreview(box)

	if res := f.erase(box, preview.Removal, "vm:old"); res["ok"] != true {
		t.Fatalf("answer = %v, want a started run", res)
	}
	f.waitForErasure("vms")

	if _, ok, _ := f.st.GetDestination(box.ID); !ok || !f.hasTarget(vms.ID) {
		t.Fatal("a deletion that failed removed the destination")
	}
	run, err := f.st.LastRunOfKind("vms", "prune")
	if err != nil || run == nil || run.Status != "failed" {
		t.Fatalf("the run of the vms section = %+v, %v; want a failed one", run, err)
	}
}

func TestAnErasureWaitsForNoBackupAndRefusesWhatItCannotEmpty(t *testing.T) {
	f := newPlacementFixture(t)
	f.signIn()
	box, _ := boxWithVMs(f)
	preview := f.erasurePreview(box)

	unlock, ok := f.svc.tryLockDomainFor("vms", "backup")
	if !ok {
		t.Fatal("the vms domain is busy before the test took it")
	}
	if res := f.erase(box, preview.Removal, "vm:old"); res["code"] != "domain-busy" {
		t.Fatalf("answer while a backup runs = %v, want domain-busy", res)
	}
	unlock()

	f.eng.listErr[boxRepo+"/vms"] = errors.New("dial tcp: no route to host")
	unreadable := f.erasurePreview(box)
	if got := unreadable.Sections[0]; got.Kept != erasureKeptUnreadable || got.Backups != 0 {
		t.Fatalf("an unreachable section reads as %+v", got)
	}
	if res := f.erase(box, unreadable.Removal); res["code"] != "nothing-to-erase" {
		t.Fatalf("answer with nothing that can be emptied = %v, want nothing-to-erase", res)
	}
	if _, ok, _ := f.st.GetDestination(box.ID); !ok {
		t.Fatal("the destination went although nothing could be deleted")
	}
}

func TestAnErasureLeavesADirectRepositoryInUseAndEmptiesAnIdleOne(t *testing.T) {
	f := newPlacementFixture(t)
	f.signIn()
	box := f.destination("Box", boxRepo)
	containers := f.tickDestination(box, "containers")
	files := f.tickDestination(box, "files")
	used := f.direct(containers)
	idle := f.direct(files)
	f.container("nginx", used.ID)
	f.hold(used.Repo, snap("n1", 10, "container:nginx"))
	f.hold(idle.Repo, snap("d1", 10, "fileset:docs"), snap("d2", 20, "fileset:docs"))

	preview := f.erasurePreview(box)
	bySection := map[string]erasureSection{}
	for _, sec := range preview.Sections {
		bySection[sec.Domain+"/"+sec.RepoID] = sec
	}
	if got := bySection["containers/"+used.ID]; got.Kept != erasureKeptInUse || got.Backups != 1 {
		t.Fatalf("the direct repository nginx backs up to reads as %+v", got)
	}
	if got := bySection["files/"+idle.ID]; got.Kept != "" || got.Backups != 2 {
		t.Fatalf("the direct repository nothing backs up to reads as %+v", got)
	}
	if last := []erasureEntry{{Domain: "files", Identity: "fileset:docs", Backups: 2}}; !slices.Equal(preview.LastCopies, last) {
		t.Fatalf("last copies = %+v, want %+v", preview.LastCopies, last)
	}

	if res := f.erase(box, preview.Removal, "fileset:docs"); res["ok"] != true {
		t.Fatalf("answer = %v, want a started run", res)
	}
	f.waitForErasure("files")
	f.waitForErasure("containers")
	if left := f.held(idle.Repo); len(left) != 0 {
		t.Fatalf("the idle direct repository still holds %v", left)
	}
	if left := f.held(used.Repo); !slices.Equal(left, []string{"n1"}) {
		t.Fatalf("the direct repository in use holds %v, want its backup left alone", left)
	}
	if _, err := f.st.GetNamedRepo(used.ID); err != nil {
		t.Fatalf("the direct repository in use did not outlive the destination: %v", err)
	}
}

// aliasedHome gives the vms section its own repository under user/ and a
// destination under disk1/, whose vms folder is the same Unraid share: both
// hold the same two backups of vm:old.
func aliasedHome(f *placementFixture) (disk store.OffsiteTarget, home, there string) {
	f.settings(func(s *store.Settings) { s.VMsPath = "user/backups/vms" })
	home = f.localRepo("user/backups/vms")
	disk = f.destination("Disk", "disk1/backups")
	f.tickDestination(disk, "vms")
	there = f.localRepo("disk1/backups/vms")
	for _, loc := range []string{home, there} {
		f.hold(loc, snap("o1", 50, "vm:old"), snap("o2", 60, "vm:old"))
	}
	return disk, home, there
}

func TestAHomeUnderAnotherSpellingOfTheDestinationIsNoSecondCopy(t *testing.T) {
	f := newPlacementFixture(t)
	f.signIn()
	disk, home, _ := aliasedHome(f)

	preview := f.erasurePreview(disk)
	for _, sec := range preview.Sections {
		if sec.Kept == "" && len(preview.LastCopies) == 0 {
			t.Fatalf("the preview empties %+v and calls vm:old held elsewhere, though its home is the same folder", sec)
		}
	}
	res := f.erase(disk, preview.Removal)
	if res["ok"] != false {
		t.Fatalf("answer = %v, want a refusal", res)
	}
	f.waitForErasure("vms")
	if len(f.eng.deletes) != 0 || len(f.held(home)) != 2 {
		t.Fatalf("the erasure forgot %+v", f.eng.deletes)
	}
	if _, ok, _ := f.st.GetDestination(disk.ID); !ok {
		t.Fatal("a refused erasure removed the destination")
	}
}

func TestAPlaceTheErasureEmptiesHoldsNothingUnderAnotherSpelling(t *testing.T) {
	f := newPlacementFixture(t)
	f.signIn()
	_, _, there := aliasedHome(f)
	settings, err := f.st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	oc, err := f.svc.ownerContextFor("vms")
	if err != nil {
		t.Fatal(err)
	}

	held := f.svc.heldOutside(context.Background(), settings, "vms", "", oc, []string{there})

	if len(held.entries) != 0 || len(held.backups) != 0 {
		t.Fatalf("the home counts as holding %v elsewhere, though it is the folder being emptied", held.backups)
	}
}

func TestATargetTheErasureEmptiesHoldsNothingElse(t *testing.T) {
	f := newPlacementFixture(t)
	box, _ := boxWithVMs(f)
	f.rcloneConf("[vault]\ntype = drive\nroot_folder_id = 0Bvault\n")
	other := f.tickDestination(f.destination("Vault", vaultRepo), "vms")
	f.hold(other.Repo, copied("v1", "o1", 50, "vm:old"))
	settings, err := f.st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	oc, err := f.svc.ownerContextFor("vms")
	if err != nil {
		t.Fatal(err)
	}

	if held := f.svc.heldOutside(context.Background(), settings, "vms", box.ID, oc, nil); !held.entries["vm:old"] {
		t.Fatalf("the target at Vault holds %v, want vm:old among it", held.entries)
	}
	if held := f.svc.heldOutside(context.Background(), settings, "vms", box.ID, oc, []string{other.Repo}); held.entries["vm:old"] {
		t.Fatalf("the target the erasure empties counts as holding %v", held.entries)
	}
}

func TestAnErasureRefusesADestinationWhoseRemovalChanged(t *testing.T) {
	f := newPlacementFixture(t)
	f.signIn()
	box := f.destination("Box", boxRepo)
	f.tickDestination(box, "files")
	direct := f.direct(f.tickDestination(box, "containers"))
	f.container("nginx", direct.ID)
	f.hold(direct.Repo, snap("n1", 10, "container:nginx"))
	preview := f.erasurePreview(box)

	f.container("redis", direct.ID)
	res := f.erase(box, preview.Removal)

	if res["ok"] != false || res["code"] != "stale" || res["removal"] == nil {
		t.Fatalf("answer = %v, want the refusal stale with the removal as it stands", res)
	}
	if _, ok, _ := f.st.GetDestination(box.ID); !ok {
		t.Fatal("a refused erasure removed the destination")
	}
}

func TestAPlaceNothingCanNameHoldsNoSecondCopy(t *testing.T) {
	f := newPlacementFixture(t)
	f.signIn()
	box, _ := boxWithVMs(f)
	nowhere := f.destination("Nowhere", "rclone:nowhere:bv")
	other := f.tickDestination(nowhere, "vms")
	f.hold(other.Repo, copied("n1", "o1", 50, "vm:old"), copied("n2", "o2", 60, "vm:old"), copied("n3", "o3", 70, "vm:old"))

	preview := f.erasurePreview(box)

	if last := []erasureEntry{{Domain: "vms", Identity: "vm:old", Backups: 3}}; !slices.Equal(preview.LastCopies, last) {
		t.Fatalf("last copies = %+v, want %+v: no rclone remote is called nowhere", preview.LastCopies, last)
	}
}

func TestAnErasureLeavesTheRepositoriesEntriesBackUpToAlone(t *testing.T) {
	f := newPlacementFixture(t)
	f.signIn()
	box := f.destination("Box", boxRepo)
	files := f.tickDestination(box, "files")
	containers := f.tickDestination(box, "containers")
	f.container("nginx", f.namedRepo("Box containers", boxRepo+"/containers").ID)
	f.vm("win", f.namedRepo("Box vms", boxRepo+"/vms").ID)
	f.hold(boxRepo+"/containers", snap("n1", 10, "container:nginx"))
	f.hold(boxRepo+"/vms", snap("w1", 10, "vm:win"))

	preview := f.erasurePreview(box)
	want := []erasureSection{
		{Domain: "containers", TargetID: containers.ID, Repo: boxRepo + "/containers", Backups: 1, Entries: 1, Kept: erasureKeptInUse},
		{Domain: "files", TargetID: files.ID, Repo: boxRepo + "/files"},
	}
	if !slices.Equal(preview.Sections, want) {
		t.Fatalf("sections = %+v, want %+v", preview.Sections, want)
	}

	var named []string
	for _, entry := range preview.LastCopies {
		named = append(named, entry.Identity)
	}
	if res := f.erase(box, preview.Removal, named...); res["ok"] != true {
		t.Fatalf("answer = %v, want a started run", res)
	}
	for _, domain := range []string{"containers", "files", "vms"} {
		f.waitForErasure(domain)
	}
	if got := f.held(boxRepo + "/containers"); !slices.Equal(got, []string{"n1"}) {
		t.Fatalf("the repository nginx backs up to holds %v", got)
	}
	if got := f.held(boxRepo + "/vms"); !slices.Equal(got, []string{"w1"}) {
		t.Fatalf("the repository win backs up to holds %v", got)
	}
}

func TestASectionAnotherSectionCopiesToStaysAsShared(t *testing.T) {
	f := newPlacementFixture(t)
	f.signIn()
	box, _ := boxWithVMs(f)
	f.target("vms", "By hand", boxRepo+"/vms")

	preview := f.erasurePreview(box)

	if got := preview.Sections[0]; got.Kept != erasureKeptShared {
		t.Fatalf("the section another target copies to reads as %+v, want it kept as shared", got)
	}
}

func TestAnErasureRefusesASectionLeftBehindThatWasNotShown(t *testing.T) {
	f := newPlacementFixture(t)
	f.signIn()
	box := f.destination("Box", boxRepo)
	f.tickDestination(box, "files")
	f.hold(f.domainPath("vms"), snap("a1", 100, "vm:win"))
	preview := f.erasurePreview(box)

	f.hold(boxRepo+"/vms", copied("c1", "a1", 100, "vm:win"))
	res := f.erase(box, preview.Removal)

	if res["ok"] != false || res["code"] != "stale" {
		t.Fatalf("answer = %v, want the refusal stale", res)
	}
	if now := erasureOf(t, res); len(now.Sections) != 2 {
		t.Fatalf("the refusal carries %+v, want the section left behind in it", now.Sections)
	}
	f.waitForErasure("vms")
	if len(f.eng.deletes) != 0 {
		t.Fatalf("the erasure forgot %+v", f.eng.deletes)
	}
}

func TestAnErasureWaitsForABackupOfASectionLeftBehind(t *testing.T) {
	f := newPlacementFixture(t)
	f.signIn()
	box := f.destination("Box", boxRepo)
	f.tickDestination(box, "files")
	f.hold(f.domainPath("vms"), snap("a1", 100, "vm:win"))
	f.hold(boxRepo+"/vms", copied("c1", "a1", 100, "vm:win"))
	preview := f.erasurePreview(box)

	unlock, ok := f.svc.tryLockDomainFor("vms", "backup")
	if !ok {
		t.Fatal("the vms domain is busy before the test took it")
	}
	defer unlock()
	if res := f.erase(box, preview.Removal); res["code"] != "domain-busy" {
		t.Fatalf("answer while vms is backed up = %v, want domain-busy", res)
	}
	if len(f.eng.deletes) != 0 {
		t.Fatalf("the erasure forgot %+v", f.eng.deletes)
	}
}

func TestAnErasureRefusesBackupsThatArrivedAfterThePreview(t *testing.T) {
	f := newPlacementFixture(t)
	f.signIn()
	box, _ := boxWithVMs(f)
	preview := f.erasurePreview(box)

	f.hold(boxRepo+"/vms", append(f.snapsAt(boxRepo+"/vms"), snap("o4", 80, "vm:old"))...)
	res := f.erase(box, preview.Removal, "vm:old")

	if res["ok"] != false || res["code"] != "stale" {
		t.Fatalf("answer = %v, want the refusal stale", res)
	}
	if now := erasureOf(t, res); now.Backups != 6 {
		t.Fatalf("the refusal counts %d backups, want the 6 there are now", now.Backups)
	}
	f.waitForErasure("vms")
	if len(f.eng.deletes) != 0 {
		t.Fatalf("the erasure forgot %+v", f.eng.deletes)
	}
}

func TestAnErasureWaitsForAMoveOfOneOfItsSections(t *testing.T) {
	f := newPlacementFixture(t)
	f.signIn()
	box, vms := boxWithVMs(f)
	vault := f.destination("Vault", vaultRepo)
	waiting, done := make(chan struct{}), make(chan struct{})
	f.eng.onCopy = func() {
		select {
		case <-waiting:
		default:
			close(waiting)
		}
		<-done
	}
	moveOf(t, f.do("POST", "/api/offsite/targets/"+vms.ID+"/move", map[string]any{"to": vault.ID}))
	<-waiting
	preview := f.erasurePreview(box)

	res := f.erase(box, preview.Removal, "vm:old")
	close(done)

	if res["ok"] != false || res["code"] != "move-running" {
		t.Fatalf("answer while vms moves away = %v, want move-running", res)
	}
	f.waitForErasure("vms")
	if len(f.eng.deletes) != 0 {
		t.Fatalf("the erasure forgot %+v", f.eng.deletes)
	}
}

func TestAMoveWaitsForAnErasureAtEitherPlace(t *testing.T) {
	f := newPlacementFixture(t)
	f.signIn()
	box, vms := boxWithVMs(f)
	vault := f.destination("Vault", vaultRepo)
	f.tickDestination(vault, "files")
	f.hold(f.domainPath("files"), snap("d1", 10, "fileset:docs"))
	f.hold(vaultRepo+"/files", copied("e1", "d1", 10, "fileset:docs"))
	waiting, done := make(chan struct{}), make(chan struct{})
	f.eng.onForget = func() {
		select {
		case <-waiting:
		default:
			close(waiting)
		}
		<-done
	}
	if res := f.erase(vault, f.erasurePreview(vault).Removal); res["ok"] != true {
		t.Fatalf("answer = %v, want a started run", res)
	}
	<-waiting

	res := f.movePreview(vms, vault)
	started := f.do("POST", "/api/offsite/targets/"+vms.ID+"/move", map[string]any{"to": vault.ID})
	close(done)
	f.waitForErasure("files")

	if res["ok"] != true {
		t.Fatalf("preview = %v, want it to answer", res)
	}
	if started["ok"] != false || started["code"] != "erase-running" {
		t.Fatalf("a move onto a place being emptied = %v, want erase-running", started)
	}
	if got := f.stored(vms.ID); got.DestinationID != box.ID || len(f.eng.copies) != 0 {
		t.Fatalf("the refused move copied %+v and left the target at %+v", f.eng.copies, got)
	}
}

func TestDeletingTheBackupsAtADestinationTakesALoginPassword(t *testing.T) {
	f := newPlacementFixture(t)
	box, _ := boxWithVMs(f)

	for _, req := range []struct {
		method string
		body   any
	}{
		{"GET", nil},
		{"POST", map[string]any{"expect": destinationRemoval{}, "lastCopies": []string{"vm:old"}}},
	} {
		code, res := f.call(req.method, "/api/offsite/destinations/"+box.ID+"/erasure", req.body)
		if code != http.StatusForbidden || res["code"] != codeNoPassword {
			t.Fatalf("%s without a login password = %d %v, want %s", req.method, code, res, codeNoPassword)
		}
	}
	f.waitForErasure("vms")
	if len(f.eng.deletes) != 0 {
		t.Fatalf("the erasure forgot %+v", f.eng.deletes)
	}
	if res := f.do("POST", "/api/offsite/destinations/"+box.ID+"/removal", map[string]any{"expect": f.removalPreview(box)}); res["ok"] != true {
		t.Fatalf("removing the destination and keeping its backups = %v, want it done", res)
	}
}

func TestAnErasureKeepsASectionWhoseDeleteTestWasRefused(t *testing.T) {
	f := newPlacementFixture(t)
	f.signIn()
	box, vms := boxWithVMs(f)
	if err := f.st.RecordTamperTestForTarget("vms", vms.ID, true, ""); err != nil {
		t.Fatal(err)
	}

	preview := f.erasurePreview(box)

	if got := preview.Sections[0]; got.Kept != erasureKeptAppendOnly {
		t.Fatalf("the section behind an object lock reads as %+v, want it kept as append-only", got)
	}
	if res := f.erase(box, preview.Removal, "vm:old"); res["code"] != "nothing-to-erase" {
		t.Fatalf("answer = %v, want nothing-to-erase", res)
	}
	if len(f.eng.deletes) != 0 {
		t.Fatalf("the erasure forgot %+v", f.eng.deletes)
	}
}

func TestAnErasureThatFailsHalfwayNamesTheSectionsItEmptied(t *testing.T) {
	f := newPlacementFixture(t)
	f.signIn()
	box, _ := boxWithVMs(f)
	f.tickDestination(box, "files")
	f.hold(f.domainPath("files"), snap("d1", 10, "fileset:docs"))
	f.hold(boxRepo+"/files", copied("e1", "d1", 10, "fileset:docs"))
	f.eng.forgetErr = map[string]error{boxRepo + "/vms": errors.New("connection reset")}

	if res := f.erase(box, f.erasurePreview(box).Removal, "vm:old"); res["ok"] != true {
		t.Fatalf("answer = %v, want a started run", res)
	}
	f.waitForErasure("vms")
	f.waitForErasure("files")

	if left := f.held(boxRepo + "/files"); len(left) != 0 {
		t.Fatalf("the files section still holds %v, want it emptied first", left)
	}
	run, err := f.st.LastRunOfKind("vms", "prune")
	if err != nil || run == nil || run.Status != "failed" || !strings.Contains(run.Error, "files at "+boxRepo+"/files") {
		t.Fatalf("the failed run = %+v, %v; want it to name the files section it emptied before", run, err)
	}
}

func TestADestinationOverAHomeOrAnotherDestinationIsRefused(t *testing.T) {
	f := newPlacementFixture(t)
	f.signIn()
	ctx := context.Background()
	if _, err := f.svc.SaveNewDestination(ctx, draftRequest{Provider: "path", Settings: map[string]string{"path": "backups"}}); !errors.Is(err, errNestedLocation) {
		t.Fatalf("a destination around the repositories at home = %v, want errNestedLocation", err)
	}
	rest := draftRequest{Provider: "rest", Name: "Box", Settings: map[string]string{"url": "https://box:8000/bv", "user": "bv", "pass": "one"}}
	if _, err := f.svc.SaveNewDestination(ctx, rest); err != nil {
		t.Fatal(err)
	}
	rest.Settings["url"] = "https://box:8000/bv/inner"
	if _, err := f.svc.SaveNewDestination(ctx, rest); !errors.Is(err, errNestedLocation) {
		t.Fatalf("a destination inside another = %v, want errNestedLocation", err)
	}
	if sets := f.storedCredSets(); len(sets) != 1 {
		t.Fatalf("the refused destination left its sign-in behind: %+v", sets)
	}
}

func TestAnErasureTakesWhatAMovedSectionLeftBehind(t *testing.T) {
	f := newPlacementFixture(t)
	f.signIn()
	box, vms := boxWithVMs(f)
	files := f.tickDestination(box, "files")
	f.rcloneConf("[vault]\ntype = drive\nroot_folder_id = 0Bvault\n")
	vault := f.destination("Vault", vaultRepo)
	f.move(vms, vault)

	preview := f.erasurePreview(box)
	want := []erasureSection{
		{Domain: "files", TargetID: files.ID, Repo: boxRepo + "/files"},
		{Domain: "vms", Repo: boxRepo + "/vms", Backups: 5, Entries: 2},
	}
	if !slices.Equal(preview.Sections, want) {
		t.Fatalf("sections = %+v, want %+v", preview.Sections, want)
	}
	if len(preview.LastCopies) != 0 {
		t.Fatalf("last copies = %+v, want none: the section holds them at Vault", preview.LastCopies)
	}

	if res := f.erase(box, preview.Removal); res["ok"] != true {
		t.Fatalf("answer = %v, want a started run", res)
	}
	f.waitForErasure("files")
	if left := f.held(boxRepo + "/vms"); !slices.Equal(left, []string{"hand"}) {
		t.Fatalf("the repository the section left behind still holds %v", left)
	}
	if got := f.held(vaultRepo + "/vms"); len(got) != 5 {
		t.Fatalf("the moved section holds %v at Vault, want its 5 backups", got)
	}
	if _, ok, _ := f.st.GetDestination(box.ID); ok {
		t.Fatal("the destination is still there")
	}
}
