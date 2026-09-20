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

func TestThePlacementFixtureBuildsWhatItNames(t *testing.T) {
	f := newPlacementFixture(t)
	b2 := f.target("containers", "B2", "b2:bucket:containers")
	field := f.fieldTarget("vms", "b2:bucket:vms")
	nas := f.namedRepo("NAS", "nas")
	nginx := f.container("nginx", nas.ID)
	win := f.vm("win11", "")
	docs := f.fileSet("Docs", "")
	f.rule("containers", "container:nginx", store.SkipAll)
	f.setDefault("files", "", b2.ID)
	f.listing("containers", b2.ID, 300, copiesRow("container:nginx", 2, 290))
	f.backupRun(win.ID, 1_700_000_000)
	f.replicated("containers")
	f.hold(f.domainPath("containers"), snap("a1", 100, "container:nginx"), copied("c1", "x1", 90, "container:plex"))

	if nginx.Repo != nas.ID || field.SortOrder != 0 || !field.Enabled || docs.Name != "Docs" {
		t.Fatalf("rows: nginx %+v, field %+v, docs %+v", nginx, field, docs)
	}
	if _, err := os.Stat(filepath.Join(filepath.FromSlash(f.root), "nas", "config")); err != nil {
		t.Fatalf("the local named repository has no config marker: %v", err)
	}
	if _, err := os.Stat(filepath.Join(filepath.FromSlash(f.root), "files", "Docs")); err != nil {
		t.Fatalf("the file set has no source folder: %v", err)
	}
	if last, err := f.st.LastSuccessfulBackupAmong([]string{win.ID}); err != nil || last.Unix() != 1_700_000_000 {
		t.Fatalf("backupRun left %v, %v, want a success at 1700000000", last, err)
	}
	if _, offsite, err := f.st.DomainHasHistory("containers"); err != nil || !offsite {
		t.Fatalf("replicated left offsite=%v, %v", offsite, err)
	}
	snaps, err := f.eng.Snapshots(context.Background(), f.domainPath("containers"), restic.Mode{})
	if err != nil || len(snaps) != 2 || snaps[1].Original != "x1" || f.eng.lists[f.domainPath("containers")] != 1 {
		t.Fatalf("Snapshots = %+v, %v, lists %v", snaps, err, f.eng.lists)
	}
	if res := f.do(http.MethodGet, "/api/health", nil); res["ok"] != true {
		t.Fatalf("GET /api/health = %v", res)
	}
}

func TestThePlacementEngineCopiesWhatTheTargetLacks(t *testing.T) {
	f := newPlacementFixture(t)
	ctx := context.Background()
	src, dst := f.domainPath("containers"), "b2:bucket:containers"
	f.hold(src, snap("a1", 100, "container:nginx"), snap("a2", 200, "container:nginx"))
	f.hold(dst, copied("b1", "a1", 100, "container:nginx"))

	if err := f.eng.Copy(ctx, dst, src, nil, restic.Limits{}, restic.Mode{}); err != nil {
		t.Fatal(err)
	}
	held, _ := f.eng.Snapshots(ctx, dst, restic.Mode{})
	if len(held) != 2 || restic.Identity(held[1]) != "a2" || held[1].ID != copyID(dst, "a2") {
		t.Fatalf("after Copy the target holds %+v", held)
	}
	if err := f.eng.ForgetPolicy(ctx, dst, restic.RetentionPolicy{KeepLast: 1}, restic.Mode{}, "container:nginx", false); err != nil {
		t.Fatal(err)
	}
	if held, _ = f.eng.Snapshots(ctx, dst, restic.Mode{}); len(held) != 1 || restic.Identity(held[0]) != "a2" {
		t.Fatalf("after keep-last 1 the target holds %+v, want the newest", held)
	}
	if err := f.eng.Forget(ctx, dst, []string{held[0].ID}, false, restic.Mode{}); err != nil {
		t.Fatal(err)
	}
	if held, _ = f.eng.Snapshots(ctx, dst, restic.Mode{}); len(held) != 0 {
		t.Fatalf("after Forget the target holds %+v", held)
	}
	if len(f.eng.copies) != 1 || f.eng.copies[0].IDs != nil || len(f.eng.forgets) != 1 || len(f.eng.deletes) != 1 {
		t.Fatalf("records: copies %+v forgets %+v deletes %+v", f.eng.copies, f.eng.forgets, f.eng.deletes)
	}
	f.eng.opens[dst] = false
	if f.eng.RepoOpens(ctx, dst, restic.Mode{}) || f.eng.RepoOpensErr(ctx, dst, restic.Mode{}) == nil {
		t.Fatal("a location marked closed opened")
	}
	f.eng.listErr[src] = errors.New("share not mounted")
	if _, err := f.eng.Snapshots(ctx, src, restic.Mode{}); err == nil {
		t.Fatal("a location with a listing error listed")
	}
	if err := f.eng.Prune(ctx, dst, restic.Mode{}); err != nil || len(f.eng.prunes) != 1 {
		t.Fatalf("Prune = %v, prunes %v", err, f.eng.prunes)
	}
	if err := f.eng.Init(ctx, dst, restic.Mode{}); err != nil || len(f.eng.ensured) != 1 {
		t.Fatalf("Init = %v, ensured %v", err, f.eng.ensured)
	}
	if st, err := f.eng.Stats(ctx, dst, "raw-data", restic.Mode{}); err != nil || st.TotalSize != 0 {
		t.Fatalf("Stats = %+v, %v", st, err)
	}
	if err := f.eng.Unlock(ctx, dst, false, restic.Mode{}); err != nil {
		t.Fatal(err)
	}
	f.dock.installed["nginx"] = true
	if list, err := f.dock.List(ctx); err != nil || len(list) != 1 || list[0].Name != "nginx" {
		t.Fatalf("List = %+v, %v", list, err)
	}
}
