package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

func (f *placementFixture) removalPreview(d store.OffsiteTarget) locationRemoval {
	f.t.Helper()
	res := f.do("GET", "/api/offsite/destinations/"+d.ID+"/removal", nil)
	if res["ok"] != true {
		f.t.Fatalf("preview: %v", res)
	}
	b, err := json.Marshal(res["removal"])
	if err != nil {
		f.t.Fatal(err)
	}
	var out locationRemoval
	if err := json.Unmarshal(b, &out); err != nil {
		f.t.Fatal(err)
	}
	return out
}

func (f *placementFixture) remove(d store.OffsiteTarget, expect locationRemoval) map[string]any {
	f.t.Helper()
	return f.do("POST", "/api/offsite/destinations/"+d.ID+"/removal", map[string]any{"expect": expect})
}

func (f *placementFixture) hasTarget(id string) bool {
	f.t.Helper()
	_, ok, err := f.st.GetOffsiteTarget(id)
	if err != nil {
		f.t.Fatal(err)
	}
	return ok
}

func TestTheRemovalPreviewCountsWhatEachSectionKeepsAtTheDestination(t *testing.T) {
	f := newPlacementFixture(t)
	box := f.destination("Box", "rest:https://box:8000/bv")
	vms := f.tickDestination(box, "vms")
	files := f.tickDestination(box, "files")
	f.appendOnly(files.ID)
	f.listing("vms", vms.ID, 1700000000,
		store.ItemCopies{Domain: "vms", Identity: "vm:win", TargetID: vms.ID, SnapshotCount: 7},
		store.ItemCopies{Domain: "vms", Identity: "vm:hass", TargetID: vms.ID, SnapshotCount: 3})

	got := f.removalPreview(box)

	if len(got.Sections) != 2 {
		t.Fatalf("sections = %+v, want files and vms", got.Sections)
	}
	want := removalSection{Domain: "vms", Use: removalCopy, TargetID: vms.ID, Repo: "rest:https://box:8000/bv/vms", Entries: 2, Backups: 10, ListedAt: 1700000000}
	if got.Sections[1] != want {
		t.Fatalf("vms = %+v, want %+v", got.Sections[1], want)
	}
	if s := got.Sections[0]; s.Domain != "files" || !s.AppendOnly || s.Entries != 0 || s.ListedAt != 0 {
		t.Fatalf("files = %+v, want an append-only section that was never listed", s)
	}
}

func TestRemovingADestinationDetachesItsSectionsAndLeavesNothingBehind(t *testing.T) {
	f := newPlacementFixture(t)
	d, err := f.svc.SaveNewDestination(context.Background(), draftRequest{
		Provider: "rest", Name: "Box", Settings: map[string]string{"url": "https://box:8000/bv", "user": "bv", "pass": "secret"},
	})
	if err != nil {
		t.Fatal(err)
	}
	vms := f.tickDestination(d, "vms")
	files := f.tickDestination(d, "files")
	unused := f.direct(files)
	f.listing("vms", vms.ID, 1700000000, store.ItemCopies{Domain: "vms", Identity: "vm:win", TargetID: vms.ID, SnapshotCount: 7})

	if res := f.remove(d, f.removalPreview(d)); res["ok"] != true {
		t.Fatalf("removal: %v", res)
	}

	if _, ok, _ := f.st.GetDestination(d.ID); ok {
		t.Fatal("the destination is still there")
	}
	if f.hasTarget(vms.ID) || f.hasTarget(files.ID) {
		t.Fatal("a section still copies to the removed destination")
	}
	if _, err := f.st.GetNamedRepo(unused.ID); err == nil {
		t.Fatal("a direct repository nothing backs up to outlived its target")
	}
	if copies, _ := f.st.ItemCopiesForDomain("vms"); len(copies) != 0 {
		t.Fatalf("observed copies left behind: %+v", copies)
	}
	if sets := f.storedCredSets(); len(sets) != 0 {
		t.Fatalf("the sign-in the destination brought along is still stored: %+v", sets)
	}
}

func TestADirectRepositoryEntriesBackUpToOutlivesItsDestination(t *testing.T) {
	f := newPlacementFixture(t)
	d, err := f.svc.SaveNewDestination(context.Background(), draftRequest{
		Provider: "rest", Name: "Box", Settings: map[string]string{"url": "https://box:8000/bv", "user": "bv", "pass": "secret"},
	})
	if err != nil {
		t.Fatal(err)
	}
	target := f.tickDestination(d, "containers")
	direct := f.direct(target)
	f.container("nginx", direct.ID)

	preview := f.removalPreview(d)
	if got := preview.Sections[0].Direct; got == nil || !got.Kept || got.Items != 1 || got.RepoID != direct.ID {
		t.Fatalf("direct = %+v, want it kept for its one entry", got)
	}
	if res := f.remove(d, preview); res["ok"] != true {
		t.Fatalf("removal: %v", res)
	}

	kept, err := f.st.GetNamedRepo(direct.ID)
	if err != nil {
		t.Fatalf("the repository nginx backs up to is gone: %v", err)
	}
	if kept.CompanionOf != "" || kept.Repo != direct.Repo || kept.CredsRef != d.CredsRef {
		t.Fatalf("kept repository = %+v, want it on its own with the location and sign-in it had", kept)
	}
	if home := f.home(store.ItemRef{Domain: "containers", Key: "nginx"}); home.Repo != direct.ID {
		t.Fatalf("nginx backs up to %q, want its repository as before", home.Repo)
	}
	if sets := f.storedCredSets(); len(sets) != 1 || sets[0].ID != d.CredsRef {
		t.Fatalf("credential sets = %+v, want the one the kept repository signs in with", sets)
	}
}

func TestRemovingADestinationClearsTheOffsiteFieldOfAPrimaryItHeld(t *testing.T) {
	f := newPlacementFixture(t)
	box := f.destination("Box", "rest:https://box:8000/bv")
	if _, _, err := f.svc.PrimaryFromDestination(context.Background(), box, "flash"); err != nil {
		t.Fatal(err)
	}
	if s := storedSettings(t, f.st); s.FlashOffsite != "rest:https://box:8000/bv/flash" {
		t.Fatalf("flash off-site field = %q before the removal", s.FlashOffsite)
	}

	preview := f.removalPreview(box)
	if len(preview.Sections) != 1 || !preview.Sections[0].Primary {
		t.Fatalf("sections = %+v, want the primary of flash", preview.Sections)
	}
	if res := f.remove(box, preview); res["ok"] != true {
		t.Fatalf("removal: %v", res)
	}

	if s := storedSettings(t, f.st); s.FlashOffsite != "" {
		t.Fatalf("flash off-site field = %q, want it cleared so a settings save does not bring the copy back", s.FlashOffsite)
	}
	if err := f.svc.syncPrimaryOffsiteTarget("flash", storedSettings(t, f.st)); err != nil {
		t.Fatal(err)
	}
	if targets, _ := f.st.OffsiteTargetsForDomain("flash"); len(targets) != 0 {
		t.Fatalf("flash copies to %+v after the removal", targets)
	}
}

// briefRemovalWait lets a removal give up on a busy section at once.
func briefRemovalWait(t *testing.T) {
	t.Helper()
	wait := removalLockWait
	removalLockWait = 0
	t.Cleanup(func() { removalLockWait = wait })
}

func TestALocationIsNotRemovedWhileACopyIntoItRuns(t *testing.T) {
	briefRemovalWait(t)
	f := newPlacementFixture(t)
	box, vms := boxWithVMs(f)
	preview := f.removalPreview(box)
	unlock, ok := f.svc.tryLockDomainFor("vms", "replicate")
	if !ok {
		t.Fatal("vms is locked already")
	}

	if res := f.remove(box, preview); res["ok"] != false || res["code"] != "domain-busy" {
		t.Fatalf("removal during the copy = %v, want it refused as busy", res)
	}
	if res := f.removeLocation("destination:"+box.ID, preview, ""); res["ok"] != false || res["code"] != "domain-busy" || res["domain"] != "vms" {
		t.Fatalf("removal from the storage page during the copy = %v, want it refused as busy with the section named", res)
	}
	if !f.hasTarget(vms.ID) {
		t.Fatal("the copy lost its target while it ran")
	}

	unlock()
	if res := f.remove(box, preview); res["ok"] != true {
		t.Fatalf("removal after the copy = %v", res)
	}
}

func TestARemovalStraightAfterARunWaitsForTheCheckThatFollowsIt(t *testing.T) {
	f := newPlacementFixture(t)
	box, vms := boxWithVMs(f)
	preview := f.removalPreview(box)
	unlock, ok := f.svc.tryLockDomainFor("vms", "verify")
	if !ok {
		t.Fatal("vms is locked already")
	}
	time.AfterFunc(100*time.Millisecond, unlock)

	if res := f.removeLocation("destination:"+box.ID, preview, ""); res["ok"] != true {
		t.Fatalf("removal while the check after the run ends = %v, want it to go through", res)
	}
	if f.hasTarget(vms.ID) {
		t.Fatal("the copy kept its target after the removal")
	}
}

func TestAQueuedProbeLetsAWaitingRemovalGoFirst(t *testing.T) {
	fastLockPolls(t)
	s := &Service{repoMu: map[string]*sync.Mutex{"vms": {}}}
	unlockRun, _ := s.tryLockDomainFor("vms", "backup")
	removal := make(chan error, 1)
	go func() {
		unlock, err := s.lockForRemoval(context.Background(), "vms")
		if err == nil {
			defer unlock()
			time.Sleep(50 * time.Millisecond)
		}
		removal <- err
	}()
	for !s.domainIsWanted("vms") {
		time.Sleep(time.Millisecond)
	}
	probe := make(chan func(), 1)
	go func() {
		if unlock, ok := s.waitProbeLock("vms"); ok {
			probe <- unlock
		}
	}()
	unlockRun()

	if err := <-removal; err != nil {
		t.Fatalf("removal = %v, want the section once the run let go", err)
	}
	select {
	case unlock := <-probe:
		unlock()
	case <-time.After(5 * time.Second):
		t.Fatal("the probe never got its turn after the removal")
	}
}

func TestARemovalRefusedAsBusyNamesTheSection(t *testing.T) {
	briefRemovalWait(t)
	f := newPlacementFixture(t)
	box, _ := boxWithVMs(f)
	preview := f.removalPreview(box)
	unlock, ok := f.svc.tryLockDomainFor("vms", "prune")
	if !ok {
		t.Fatal("vms is locked already")
	}
	defer unlock()

	res := f.remove(box, preview)
	if res["ok"] != false || res["code"] != "domain-busy" || res["domain"] != "vms" {
		t.Fatalf("answer = %v, want domain-busy naming vms", res)
	}
}

func TestARemovalIsRefusedWhenTheSectionsChangedSinceThePreview(t *testing.T) {
	f := newPlacementFixture(t)
	box := f.destination("Box", "rest:https://box:8000/bv")
	f.tickDestination(box, "vms")
	shown := f.removalPreview(box)
	added := f.tickDestination(box, "files")

	res := f.remove(box, shown)

	if res["ok"] != false || res["code"] != "stale" {
		t.Fatalf("answer = %v, want the refusal of a stale preview", res)
	}
	if now := res["removal"].(map[string]any)["sections"].([]any); len(now) != 2 {
		t.Fatalf("the refusal carries %v, want both sections as they are", now)
	}
	if !f.hasTarget(added.ID) {
		t.Fatal("a refused removal detached a section")
	}
}

func TestTheRemovalOfAnUnknownDestinationIsNotFound(t *testing.T) {
	f := newPlacementFixture(t)
	rec := httptest.NewRecorder()
	f.h.Router().ServeHTTP(rec, jsonReq(http.MethodGet, "/api/offsite/destinations/nope/removal", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestRemovingADestinationLeavesNothingThatBlocksAddingItAgain(t *testing.T) {
	f := newPlacementFixture(t)
	const mani = "s3:http://mail.mani.pt:9000/bombvault/dxp480t"
	m, err := f.st.SaveDestination(store.OffsiteTarget{Name: "Mani", Repo: mani, Provider: "minio"})
	if err != nil {
		t.Fatal(err)
	}
	if _, out := f.call(http.MethodPost, "/api/offsite/destinations/"+m.ID+"/primary/vms", nil); out["ok"] != true {
		t.Fatalf("use Mani: %v", out)
	}
	primary, _, _ := f.st.FieldOffsiteTarget("vms")
	f.vm("win", f.direct(primary).ID)
	_, got := f.call(http.MethodGet, "/api/settings", nil)
	s := got["settings"].(map[string]any)
	s["vmsOffsite"] = ""
	if _, out := f.call(http.MethodPut, "/api/settings", s); out["ok"] != true {
		t.Fatalf("clear field: %v", out)
	}
	if out := f.remove(m, f.removalPreview(m)); out["ok"] != true {
		t.Fatalf("remove: %v", out)
	}
	if err := f.svc.destinationClash(mani); err != nil {
		t.Fatalf("adding Mani again = %v, want it free", err)
	}
}

func TestClearingTheOffsiteFieldLeavesTheCopySwitchedOffOnItsDestination(t *testing.T) {
	f := newPlacementFixture(t)
	box := f.destination("Box", "rest:https://box:8000/bv")
	if _, _, err := f.svc.PrimaryFromDestination(context.Background(), box, "vms"); err != nil {
		t.Fatal(err)
	}

	cleared := f.fieldTarget("vms", "")

	if cleared.DestinationID != box.ID || cleared.Enabled {
		t.Fatalf("primary = %+v, want it switched off and still on Box", cleared)
	}
	loc, err := f.svc.StorageLocation(locationDestination + ":" + box.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(loc.Sections) != 1 || loc.Sections[0].TargetID != cleared.ID || loc.Sections[0].Enabled {
		t.Fatalf("Box lists %+v, want the switched-off copy of vms", loc.Sections)
	}
}

func TestRemovingADestinationTakesTheRowAClearedFieldLeftOnIt(t *testing.T) {
	f := newPlacementFixture(t)
	box := f.destination("Box", "rest:https://box:8000/bv")
	if _, _, err := f.svc.PrimaryFromDestination(context.Background(), box, "vms"); err != nil {
		t.Fatal(err)
	}
	left := f.fieldTarget("vms", "")
	unused := f.direct(left)
	if err := f.st.DetachFromDestination(left.ID); err != nil {
		t.Fatal(err)
	}
	f.fieldTarget("files", "rest:https://other:8000/files")
	elsewhere := f.fieldTarget("files", "")

	if res := f.remove(box, f.removalPreview(box)); res["ok"] != true {
		t.Fatalf("removal: %v", res)
	}

	if f.hasTarget(left.ID) {
		t.Fatal("the switched-off copy of vms outlived the destination it lay on")
	}
	if _, err := f.st.GetNamedRepo(unused.ID); err == nil {
		t.Fatal("its direct repository nothing backs up to outlived it")
	}
	if !f.hasTarget(elsewhere.ID) {
		t.Fatal("the removal took a switched-off copy at another place")
	}
}

func TestRowsTheStoragePageDoesNotListLeaveTheirPlaceFree(t *testing.T) {
	f := newPlacementFixture(t)
	f.fieldTarget("vms", "rest:https://box:8000/bv/vms")
	f.fieldTarget("vms", "")
	orphan := f.target("files", "Old", "rest:https://box:8000/bv/files")
	if _, err := f.db.Exec(`UPDATE offsite_targets SET destination_id = 'gone' WHERE id = ?`, orphan.ID); err != nil {
		t.Fatal(err)
	}
	settings := storedSettings(t, f.st)

	locs, err := f.svc.StorageLocations()
	if err != nil {
		t.Fatal(err)
	}
	for _, l := range locs {
		if l.Object == locationTarget {
			t.Fatalf("the storage page lists %+v, want neither row", l)
		}
	}
	for _, loc := range []string{"rest:https://box:8000/bv/vms", "rest:https://box:8000/bv/files"} {
		if err := f.svc.locationClash(settings, loc, locationSelf{}); err != nil {
			t.Errorf("a repository at %s = %v, want the place free", loc, err)
		}
	}
	live, err := f.svc.liveRepos(settings)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range live {
		if r.kind == liveTarget {
			t.Errorf("live repositories hold the target %+v", r)
		}
	}
}

func TestADestinationMayHoldADirectRepositoryButNoOtherOne(t *testing.T) {
	f := newPlacementFixture(t)
	box := f.destination("Box", "rest:https://box:8000/bv")
	direct := f.direct(f.tickDestination(box, "containers"))
	f.container("nginx", direct.ID)
	if res := f.remove(box, f.removalPreview(box)); res["ok"] != true {
		t.Fatalf("removal: %v", res)
	}
	if err := f.svc.destinationClash(box.Repo); err != nil {
		t.Fatalf("Box again over the direct repository it kept = %v, want it free", err)
	}

	f.namedRepo("Keep", "rest:https://box:8000/bv/keep")
	if err := f.svc.destinationClash(box.Repo); !errors.Is(err, errNestedLocation) {
		t.Fatalf("Box again over a repository of its own = %v, want errNestedLocation", err)
	}
}

func TestADestinationThatHoldsAnOwnFolderStaysUntilTheOwnFolderMoves(t *testing.T) {
	f := newPlacementFixture(t)
	f.signIn()
	vault := f.destination("Vault", vaultRepo)
	if err := f.svc.SetRcloneConf("[vault]\ntype = drive\n"); err != nil {
		t.Fatal(err)
	}
	ownSteps(t, f.do("POST", "/api/storage/own-folders", map[string]any{
		"domains": []string{"containers"}, "location": "destination:" + vault.ID,
	}))

	preview := f.removalPreview(vault)
	want := []removalSection{{Domain: "containers", Use: removalHome, Repo: vaultRepo + "/containers", Folder: "containers"}}
	if !slices.Equal(preview.Sections, want) {
		t.Fatalf("sections = %+v, want the own folder of containers", preview.Sections)
	}
	if res := f.remove(vault, preview); res["ok"] != false || res["code"] != "own-folder-there" {
		t.Fatalf("removal = %v, want it refused", res)
	}
	if res := f.do("GET", "/api/storage/locations/destination:"+vault.ID+"/erasure", nil); res["ok"] != false || res["code"] != "own-folder-there" {
		t.Fatalf("erasure preview = %v, want it refused", res)
	}
	if _, ok, _ := f.st.GetDestination(vault.ID); !ok {
		t.Fatal("Vault was removed")
	}
	settings, _ := f.st.GetSettings()
	if conf, _ := f.svc.decodeRcloneConf(settings); !strings.Contains(conf, "[vault]") {
		t.Fatalf("rclone.conf = %q, want the remote containers backs up through", conf)
	}
}

func TestAnRcloneRemoteAnOwnFolderUsesStays(t *testing.T) {
	f := newPlacementFixture(t)
	if err := f.svc.SetRcloneConf("[vault]\ntype = drive\n"); err != nil {
		t.Fatal(err)
	}
	settings, _ := f.st.GetSettings()
	settings.ContainersPath = vaultRepo + "/containers"
	if err := f.st.UpdateSettings(settings); err != nil {
		t.Fatal(err)
	}

	f.svc.removeRcloneRemote("vault")

	settings, _ = f.st.GetSettings()
	if conf, _ := f.svc.decodeRcloneConf(settings); !strings.Contains(conf, "[vault]") {
		t.Fatalf("rclone.conf = %q, want the remote kept", conf)
	}
}
