package api

import (
	"context"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

func (c *countingEngine) BackupDir(_ context.Context, repo, _ string, _ []string, _ restic.Mode, _ ...string) (restic.Summary, error) {
	if err := c.count(repo, "backup"); err != nil {
		return restic.Summary{}, err
	}
	return restic.Summary{SnapshotID: "z1"}, nil
}

// flashScene is a flash drive with backups in its own folder and two targets
// it copies to.
func flashScene(t *testing.T) (f *placementFixture, local string, b2, box store.OffsiteTarget) {
	t.Helper()
	f = newPlacementFixture(t)
	f.svc.cfg.FlashDir = t.TempDir()
	if err := os.WriteFile(filepath.Join(f.svc.cfg.FlashDir, "config"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	local = f.singletonRepo("flash")
	f.hold(local, snap("f1", 100, "flash"), snap("f2", 200, "flash"))
	b2 = keepLast(t, f, f.target("flash", "B2", "b2:bucket:flash"), 3)
	box = f.target("flash", "Box", "rclone:box:flash")
	f.replicated("flash")
	return f, local, b2, box
}

func switchLocal(t *testing.T, f *placementFixture, domain string, on bool) {
	t.Helper()
	if code, out := f.call(http.MethodPut, "/api/storage/sections/"+domain+"/local", map[string]bool{"on": on}); code != http.StatusOK || out["ok"] != true {
		t.Fatalf("switch %s local to %v = %d %v", domain, on, code, out)
	}
}

func TestAFlashDriveWithItsFolderOffBacksUpStraightToEachTarget(t *testing.T) {
	f, local, b2, box := flashScene(t)
	switchLocal(t, f, "flash", false)
	c := countCalls(f)

	if _, err := f.svc.BackupFlash(context.Background()); err != nil {
		t.Fatal(err)
	}

	for _, target := range []store.OffsiteTarget{b2, box} {
		if got := c.at(target.Repo); got["backup"] != 1 || got["copy"] != 0 {
			t.Errorf("calls at %s = %v, want one backup and no copy", target.Name, got)
		}
	}
	if got := c.at(b2.Repo); got["forget"] != 1 {
		t.Errorf("B2 aged %d times, want once by its own keep-last", got["forget"])
	}
	if got := c.at(local); got["backup"] != 0 {
		t.Errorf("the own folder was backed up to %d times, want none", got["backup"])
	}
}

// limitsEngine remembers the bandwidth caps each backup ran with, by repository.
type limitsEngine struct {
	ResticEngine
	limits map[string]restic.Limits
}

func (e *limitsEngine) Backup(ctx context.Context, repo string, paths, tags []string, mode restic.Mode, excludes ...string) (restic.Summary, error) {
	e.limits[repo] = mode.Limits
	return e.ResticEngine.Backup(ctx, repo, paths, tags, mode, excludes...)
}

func TestABackupStraightToATargetKeepsToItsBandwidthCaps(t *testing.T) {
	f, _, b2, _ := flashScene(t)
	b2.LimitUpload, b2.LimitDownload = 512, 2048
	if _, err := f.st.UpsertOffsiteTarget(b2); err != nil {
		t.Fatal(err)
	}
	switchLocal(t, f, "flash", false)
	e := &limitsEngine{ResticEngine: countCalls(f), limits: map[string]restic.Limits{}}
	f.svc.engine = e

	if _, err := f.svc.BackupFlash(context.Background()); err != nil {
		t.Fatal(err)
	}

	want := restic.Limits{UploadKBps: 512, DownloadKBps: 2048}
	if got, ok := e.limits[b2.Repo]; !ok || got != want {
		t.Fatalf("the backup to B2 ran with %+v (ran: %v), want its caps %+v", got, ok, want)
	}
}

func TestSwitchingTheFolderOffLeavesWhatItHoldsWhereItIs(t *testing.T) {
	f, local, _, _ := flashScene(t)
	switchLocal(t, f, "flash", false)
	c := countCalls(f)

	if _, err := f.svc.BackupFlash(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.ReplicateOffsite(context.Background(), "flash"); err != nil {
		t.Fatal(err)
	}

	if got := c.at(local); got["forget"] != 0 || got["prune"] != 0 || got["copy"] != 0 {
		t.Fatalf("calls at the own folder = %v, want it left alone", got)
	}
	if len(f.eng.copies) != 0 {
		t.Fatalf("copies = %+v, want none while the backups go to the targets directly", f.eng.copies)
	}
	if held := f.eng.snaps[local]; len(held) != 2 {
		t.Fatalf("the own folder holds %d snapshots, want both it had", len(held))
	}
}

func TestSwitchingTheFolderOnAgainBacksUpThereAndCopiesFromIt(t *testing.T) {
	f, local, b2, _ := flashScene(t)
	switchLocal(t, f, "flash", false)
	switchLocal(t, f, "flash", true)
	c := countCalls(f)

	if _, err := f.svc.BackupFlash(context.Background()); err != nil {
		t.Fatal(err)
	}

	if got := c.at(local); got["backup"] != 1 {
		t.Fatalf("calls at the own folder = %v, want the backup there", got)
	}
	if got := c.at(b2.Repo); got["backup"] != 0 || got["copy"] != 1 {
		t.Fatalf("calls at B2 = %v, want no backup straight to it and the copy from the folder", got)
	}
}

func TestASectionWithEveryPlaceOffIsNotBackedUpAndCountsAsUnprotected(t *testing.T) {
	f, _, b2, box := flashScene(t)
	switchLocal(t, f, "flash", false)
	for _, target := range []store.OffsiteTarget{b2, box} {
		target.Enabled = false
		if _, err := f.st.UpsertOffsiteTarget(target); err != nil {
			t.Fatal(err)
		}
	}

	if _, err := f.svc.BackupFlash(context.Background()); !errors.Is(err, errNoPlace) {
		t.Fatalf("BackupFlash = %v, want errNoPlace", err)
	}
	settings, err := f.st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	settings.FlashEnabled = true
	if err := f.st.UpdateSettings(settings); err != nil {
		t.Fatal(err)
	}
	if row := itemRow(t, f, "flash"); row["noPlace"] != true {
		t.Fatalf("flash row = %v, want it marked without a place", row)
	}
}

func TestOnlySectionsWithoutPlacementHaveAFolderSwitch(t *testing.T) {
	f := newPlacementFixture(t)
	if code, _ := f.call(http.MethodPut, "/api/storage/sections/containers/local", map[string]bool{"on": false}); code != http.StatusBadRequest {
		t.Fatalf("switching the containers' folder = %d, want 400", code)
	}
}

func TestTheStorageLocationsSayWhichOwnFolderIsOff(t *testing.T) {
	f, _, _, _ := flashScene(t)
	switchLocal(t, f, "flash", false)
	locations, err := f.svc.StorageLocations()
	if err != nil {
		t.Fatal(err)
	}
	for _, loc := range locations {
		for _, sec := range loc.Sections {
			if loc.Object == locationPath && sec.Off != (sec.Domain == "flash") {
				t.Errorf("%s at %s: off = %v", sec.Domain, loc.Name, sec.Off)
			}
		}
	}
}

func TestEverySectionWithoutPlacementSwitchesToItsTargets(t *testing.T) {
	f := newPlacementFixture(t)
	settings, err := f.st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	for _, domain := range store.LocalSwitchDomains {
		target := f.target(domain, "B2", "b2:bucket:"+domain)
		own := f.root + "/backups/" + domain
		places, err := f.svc.sectionPlaces(settings, domain, own)
		if err != nil || len(places) != 1 || !places[0].own() || places[0].repo != own {
			t.Fatalf("%s with its folder on: places %+v, err %v", domain, places, err)
		}
		switchLocal(t, f, domain, false)
		places, err = f.svc.sectionPlaces(settings, domain, own)
		if err != nil || len(places) != 1 || places[0].own() || places[0].repo != target.Repo {
			t.Fatalf("%s with its folder off: places %+v, err %v", domain, places, err)
		}
	}
}

func TestATreeIsWrittenToEveryPlaceMemberByMember(t *testing.T) {
	f := newPlacementFixture(t)
	c := countCalls(f)
	settings, err := f.st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	b2 := f.target(zfsDomain, "B2", "b2:bucket:zfs")
	box := f.target(zfsDomain, "Box", "rclone:box:zfs")
	places := []sectionPlace{{repo: b2.Repo, target: &b2}, {repo: box.Repo, target: &box}}
	adapter := f.svc.placesAdapter(settings, zfsDomain, places, "")
	for _, dir := range []string{"/mnt/tank/a", "/mnt/tank/b"} {
		if _, err := adapter.BackupDir(context.Background(), b2.Repo, dir, []string{"zfs:tank"}); err != nil {
			t.Fatal(err)
		}
	}
	if c.at(b2.Repo)["backup"] != 2 || c.at(box.Repo)["backup"] != 2 {
		t.Fatalf("backups at B2 %v and Box %v, want both members at each", c.at(b2.Repo), c.at(box.Repo))
	}
}

// openFileSet is a folder set that takes its section's place at its first
// backup.
func openFileSet(t *testing.T, f *placementFixture, name string) store.FileSet {
	t.Helper()
	path := "files/" + name
	if err := os.MkdirAll(filepath.FromSlash(f.root+"/"+path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.FromSlash(f.root+"/"+path+"/a.txt"), []byte("a"), 0o600); err != nil {
		t.Fatal(err)
	}
	set, err := f.st.CreateFileSet(store.FileSet{Name: name, Path: path, Enabled: true, RepoChosen: store.RepoOpen})
	if err != nil {
		t.Fatal(err)
	}
	return set
}

func putDefault(t *testing.T, f *placementFixture, domain string, change map[string]any) {
	t.Helper()
	if out := f.do(http.MethodPut, "/api/placement/default/"+domain, change); out["ok"] != true {
		t.Fatalf("PUT default %s %v = %v", domain, change, out)
	}
}

func TestADefaultOnATargetsDirectRepositoryBacksUpThereAlone(t *testing.T) {
	f := newPlacementFixture(t)
	d := f.direct(f.target("files", "B2", "b2:bucket:files"))
	putDefault(t, f, "files", map[string]any{"home": d.ID, "skip": []string{"*"}})
	set := openFileSet(t, f, "docs")
	c := countCalls(f)

	if _, err := f.svc.BackupFileSet(context.Background(), set.ID); err != nil {
		t.Fatal(err)
	}

	if got := c.at(d.Repo); got["backup"] != 1 {
		t.Fatalf("calls at the direct repository = %v, want the backup there", got)
	}
	if got := c.at(f.domainPath("files")); got["backup"] != 0 {
		t.Fatalf("calls at the domain path = %v, want no backup there", got)
	}
}

func TestADefaultWithNoPlaceLeavesANewEntryUnbackedUpAndUnprotected(t *testing.T) {
	f := newPlacementFixture(t)
	settings, err := f.st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	settings.FilesEnabled, settings.FilesSchedule = true, "0 3 * * *"
	if err := f.st.UpdateSettings(settings); err != nil {
		t.Fatal(err)
	}
	putDefault(t, f, "files", map[string]any{"home": store.HomeNowhere})
	set := openFileSet(t, f, "docs")
	settled := f.fileSet("photos", "")

	if _, err := f.svc.BackupFileSet(context.Background(), set.ID); !errors.Is(err, errNoPlace) {
		t.Fatalf("BackupFileSet = %v, want errNoPlace", err)
	}
	if row := itemRow(t, f, set.ID); row["noPlace"] != true {
		t.Fatalf("docs = %v, want it marked without a place", row)
	}
	if row := itemRow(t, f, settled.ID); row["noPlace"] == true {
		t.Fatalf("photos = %v, want it to keep the place of its first backup", row)
	}
	report, err := f.svc.Coverage(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range report.Domains {
		if d.Domain != "files" {
			continue
		}
		if len(d.Unprotected) != 1 || d.Unprotected[0].Name != "docs" || d.Unprotected[0].Reason != CoverageNoPlace {
			t.Fatalf("files coverage = %+v, want docs unprotected for want of a place", d)
		}
	}
}

func TestAPrimarySwitchedOffByHandStaysOffUntilItsFieldNamesAnotherPlace(t *testing.T) {
	f := newPlacementFixture(t)
	primary := f.fieldTarget("flash", "b2:bucket:flash")
	body := offsiteTargetToView(primary)
	body.Enabled = false
	if out := f.do(http.MethodPut, "/api/offsite/targets/"+primary.ID, body); out["ok"] != true {
		t.Fatalf("switching the primary off = %v", out)
	}
	settings, err := f.st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}

	f.svc.syncAllPrimaryOffsiteTargets(settings)
	if now := fieldPrimary(t, f, "flash"); now.Enabled {
		t.Fatal("a settings save switched the primary back on")
	}

	if now := f.fieldTarget("flash", "b2:bucket:flash-new"); !now.Enabled || now.ID != primary.ID {
		t.Fatalf("after the field names another place: %+v, want the same target on", now)
	}
}

func TestClearingAndRefillingTheFieldBringsAHandSwitchedPrimaryBack(t *testing.T) {
	f := newPlacementFixture(t)
	primary := f.fieldTarget("flash", "b2:bucket:flash")
	if err := f.st.SetPrimaryOff("flash", primary.Repo, true); err != nil {
		t.Fatal(err)
	}
	settings, err := f.st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	settings.FlashOffsite = ""
	if err := f.st.UpdateSettings(settings); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.syncPrimaryOffsiteTarget("flash", settings); err != nil {
		t.Fatal(err)
	}

	if now := f.fieldTarget("flash", "b2:bucket:flash"); !now.Enabled {
		t.Fatal("filling the field again left the primary off")
	}
}

func fieldPrimary(t *testing.T, f *placementFixture, domain string) store.OffsiteTarget {
	t.Helper()
	target, ok, err := f.st.FieldOffsiteTarget(domain)
	if err != nil || !ok {
		t.Fatalf("FieldOffsiteTarget %s: ok=%v err=%v", domain, ok, err)
	}
	return target
}

// itemRow is the row GET /api/items has for the key.
func itemRow(t *testing.T, f *placementFixture, key string) map[string]any {
	t.Helper()
	items, _ := f.do(http.MethodGet, "/api/items", nil)["items"].([]any)
	for _, it := range items {
		if row, _ := it.(map[string]any); row["key"] == key {
			return row
		}
	}
	t.Fatalf("no item %s in %v", key, items)
	return nil
}
